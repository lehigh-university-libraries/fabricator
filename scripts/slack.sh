#!/usr/bin/env bash

set -eou pipefail

if [ -v URL ]; then
  regex='^(https:\/\/docs\.google\.com\/spreadsheets\/d\/[a-zA-Z0-9_-]+)'
  if [[ "$URL" =~ $regex ]]; then
    URL="${BASH_REMATCH[1]}"
  else
    echo "Invalid URL"
    exit 1
  fi

  # extract sheet ID from https://docs.google.com/spreadsheets/d/foo/edit?gid=0#gid=0
  SHEET_ID=$(echo "$URL" | sed -n 's|.*/d/\(.*\)|\1|p')
  echo $ACCESS_TOKEN > token
  TITLE=$(curl -s \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    "https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID?fields=properties(title)" | jq .properties.title
  )
  MESSAGE="${MESSAGE//__TITLE__/$TITLE}"
  MESSAGE="${MESSAGE//__URL__/$URL\/edit\?gid=0#gid=0}"
fi

escaped_message=$(echo "$MESSAGE" | jq -Rsa .)
echo $escaped_message
curl -s -o /dev/null -XPOST "$SLACK_WEBHOOK_URL" -d '{
  "text": '"$escaped_message"'
}'

exit 1
