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
SHEET_ID=$(echo "$URL" | sed -n 's|.*/d/\(.*\)|\1|p')

attempt=0
max_attempts=5
while [[ "$attempt" -lt "$max_attempts" ]]; do
  GSHEET_STATUS=$(curl -s \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -o gsheet.json \
    -w '%{http_code}' \
    "https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID/values/$RANGE")

  TITLE_STATUS=$(curl -s \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -o response.json \
    -w '%{http_code}' \
    "https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID?fields=properties(title)")
  
  if [ "$GSHEET_STATUS" -lt 500 ] && [ "$TITLE_STATUS" -lt 500 ]; then
    jq -r .properties.title response.json > title.txt
    break
  fi

  delay=$((attempt * 30))
  sleep "$delay"
  attempt=$(( attempt + 1))
done

files=("gsheet.json" "title.txt")
for file in "${files[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "$file does not exist."
    exit 1
  elif [[ ! -s "$file" ]]; then
    echo "$file is empty."
    exit 1
  fi
  contents=$(cat $file)
  if [ "$contents" = "null" ]; then
    echo "$file is null."
    exit 1
  fi
done
