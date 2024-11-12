#!/usr/bin/env bash

set -eou pipefail

regex='^(https:\/\/docs\.google\.com\/spreadsheets\/d\/[a-zA-Z0-9_-]+)'
if [[ "$URL" =~ $regex ]]; then
  URL="${BASH_REMATCH[1]}"
else
  echo "Invalid URL"
  exit 1
fi

# extract sheet ID from https://docs.google.com/spreadsheets/d/foo/edit?gid=0#gid=0
SHEET_ID=$(echo "$URL" | sed -n 's|.*/d/\(.*\)/.*|\1|p')

response=$(curl -s \
    "https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID/values/$RANGE" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

if echo "$response" | jq -e .values >/dev/null; then
  # save as a CSV
  echo "$response" | jq .values | jq -r '(map(keys) | add | unique) as $cols | map(. as $row | $cols | map($row[.])) as $rows | $cols, $rows[] | @csv' | tail -n +2 > source.csv
  # and also as JSON in a format check my work expects
  echo "$response" | jq -r '.values | map(map(tostring))' > csv.json
else
  echo "Failed to fetch data: $(echo "$response" | jq -r '.error.message')"
  exit 1
fi

nohup ./fabricator -server=1 &
echo $! > fabricator_pid.txt

# make sure the sheet passes the check my work
STATUS=$(curl -v \
  -w '%{http_code}' \
  -H "X-Secret: $SHARED_SECRET" \
  -XPOST \
  --upload-file csv.json \
  http://localhost:8080/workbench/check)
if [ "${STATUS}" -gt 299 ] || [ "${STATUS}" -lt 200 ]; then
  echo "Check my work failed"
  exit 1
fi
