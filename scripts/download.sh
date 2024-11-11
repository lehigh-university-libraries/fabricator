#!/usr/bin/env bash

set -eou pipefail

# extract sheet ID from https://docs.google.com/spreadsheets/d/foo/edit?gid=0#gid=0
SHEET_ID=$(echo "$URL" | sed -n 's|.*/d/\(.*\)/.*|\1|p')

response=$(curl -s \
    "https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID/values/$RANGE" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

echo $response > ok.txt

if echo "$response" | jq -e .values >/dev/null; then
  # save as a CSV
  echo "$response" | jq .values | jq -r '(map(keys) | add | unique) as $cols | map(. as $row | $cols | map($row[.])) as $rows | $cols, $rows[] | @csv' | tail -n +2 > source.csv
  # and also as JSON in a format check my work expects
  echo "$response" | jq -r '.values | map(map(tostring))' > csv.json
else
  echo "Failed to fetch data: $(echo "$response" | jq -r '.error.message')"
  exit 1
fi
