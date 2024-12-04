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
API_URL="https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID/values/Sheet1!D2:append"
PAYLOAD=$(tail +4 islandora_workbench/input_data/rollback.csv | jq -R 'tonumber? | [if . == null then empty else . end]' | jq -s '{"values": [.[] | . ]}')

STATUS=$(curl -s -X POST \
  -o /tmp/gsup.log \
  -w '%{http_code}' \
  --header "Authorization: Bearer $ACCESS_TOKEN" \
  --header "Content-Type: application/json" \
  --data @<(echo "$PAYLOAD") \
  "$API_URL?valueInputOption=RAW")

if [ "${STATUS}" != 200 ]; then
  echo "Failed to update spreadsheet with node IDs"
  cat /tmp/gsup.log
  exit 1
fi
