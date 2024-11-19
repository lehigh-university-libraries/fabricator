#!/usr/bin/env bash

set -eou pipefail

GSHEET=$(cat gsheet.json)

if echo "$GSHEET" | jq -e .values >/dev/null; then
  # save as a CSV
  echo "$GSHEET" | jq .values | jq -r '(map(keys) | add | unique) as $cols | map(. as $row | $cols | map($row[.])) as $rows | $cols, $rows[] | @csv' | tail -n +2 > source.csv
  # and also as JSON in a format check my work expects
  echo "$GSHEET" | jq -r '.values | map(map(tostring))' > csv.json
else
  echo "Failed to fetch data: $(echo "$GSHEET" | jq -r '.error.message')"
  exit 1
fi

nohup ./fabricator &
echo $! > fabricator_pid.txt
while true; do
  STATUS=$(curl -s -w '%{http_code}' -o /dev/null http://localhost:8080/healthcheck)
  if [ "${STATUS}" -eq 200 ]; then
    break
  fi
  echo "Waiting for fabricator to come online"
  sleep 1
done

# make sure the sheet passes the check my work
STATUS=$(curl -v \
  -w '%{http_code}' \
  -H "X-Secret: $SHARED_SECRET" \
  -o check.json \
  -XPOST \
  --upload-file csv.json \
  http://localhost:8080/workbench/check)
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
STATUS=$(curl -v \
  -w '%{http_code}' \
  -H "X-Secret: $SHARED_SECRET" \
  -XPOST \
  -o target.zip \
  --upload-file source.csv \
  http://localhost:8080/workbench/transform)
if [ "${STATUS}" -gt 299 ] || [ "${STATUS}" -lt 200 ]; then
  echo "CSV transform failed"
  exit 1
fi

unzip target.zip
rm target.zip

# make sure source and target CSVs line count match
SOURCE=$(wc -l < source.csv)
TARGET=$(wc -l < target.csv)
if [ "$SOURCE" != "$TARGET" ]; then
  echo "source and target CSVs don't match ($SOURCE != $TARGET)"
  exit 1
fi

# and that we're uploading at least one item
if [ "$TARGET" -lt 2 ]; then
  echo "target CSV less than two lines long"
  exit 1
fi

# and some required headers exist
header=$(head -1 target.csv)
required_fields=("field_model" "title" "field_full_title" "id")
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
