#!/usr/bin/env bash

set -eou pipefail

GSHEET=$(cat gsheet.json)
WORKBENCH_BASE_URL="${WORKBENCH_BASE_URL:-https://islandora-test.lib.lehigh.edu}"

if echo "$GSHEET" | jq -e .values >/dev/null; then
  NORMALIZED_VALUES=$(echo "$GSHEET" | jq -c '
    .values as $rows
    | ($rows | map(length) | max // 0) as $width
    | $rows
    | map(. + ([range(0; $width - length)] | map("")))
    | map(map(
        tostring
        | rtrimstr("\r") | ltrimstr("\r")
        | rtrimstr("\n") | ltrimstr("\n")
        | rtrimstr(" ") | ltrimstr(" ")
      ))
  ')

  # Keep the same padded rectangular grid for both check and transform so the
  # CLI path matches the Apps Script path from Google Sheets.
  printf '%s\n' "$NORMALIZED_VALUES" > csv.json
  echo "$NORMALIZED_VALUES" | jq -r '.[] | @csv' > source.csv
else
  echo "Failed to fetch data: $(echo "$GSHEET" | jq -r '.error.message')"
  exit 1
fi

# make sure the sheet passes the check my work
STATUS=$(curl -s \
  -w '%{http_code}' \
  --cacert /etc/ssl/certs/isle.pem \
  -H "X-Secret: $SHARED_SECRET" \
  -o check.json \
  -XPOST \
  --upload-file csv.json \
  "$WORKBENCH_BASE_URL/workbench/check")
if [ "${STATUS}" != 200 ]; then
  echo "Check my work failed"
  exit 1
fi

if [[ "$(jq '. | length' check.json)" -gt 0 ]]; then
  echo "Check my work failed"
  jq . check.json
  exit 1
fi

# transform google sheet to a workbench CSV
STATUS=$(curl -s \
  -w '%{http_code}' \
  --cacert /etc/ssl/certs/isle.pem \
  -H "X-Secret: $SHARED_SECRET" \
  -XPOST \
  -o target.zip \
  --upload-file source.csv \
  "$WORKBENCH_BASE_URL/workbench/transform")
if [ "${STATUS}" -gt 299 ] || [ "${STATUS}" -lt 200 ]; then
  echo "CSV transform failed"
  exit 1
fi

unzip target.zip
rm target.zip

TARGET_FILE="target.csv"
if [ -f target.add_media.csv ]; then
  TARGET_FILE="target.add_media.csv"
elif [ -f target.update.csv ]; then
  TARGET_FILE="target.update.csv"
fi
TARGET=$(wc -l < "$TARGET_FILE")

# ensure we're uploading at least one item
if [ "$TARGET" -lt 2 ]; then
  echo "target CSV less than two lines long"
  exit 1
fi

# ensure some required headers exist
required_fields=("field_model" "title" "field_full_title" "id")
if [ -f target.add_media.csv ]; then
  required_fields=("node_id" "file")
elif [ -f target.update.csv ]; then
  required_fields=("node_id")
fi
header=$(head -1 "$TARGET_FILE")
missing_fields=()
for field in "${required_fields[@]}"; do
  if ! grep -q "$field" <<< "$header"; then
    missing_fields+=("$field")
  fi
done
if [ ${#missing_fields[@]} -eq 0 ]; then
  echo "All required fields are present in the header."
else
  echo "Missing fields: ${missing_fields[*]}"
  exit 1
fi
