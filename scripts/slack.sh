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

  TITLE=$(cat title.txt)
  MESSAGE="${MESSAGE//__TITLE__/$TITLE}"
  MESSAGE="${MESSAGE//__URL__/$URL\/edit\?gid=0#gid=0}"

  LINE_COUNT=$(jq '.values | length' gsheet.json)
  LINE_COUNT=$(( LINE_COUNT - 1))
  MESSAGE="${MESSAGE//__LINE_COUNT__/$LINE_COUNT}"
fi

escaped_message=$(echo "$MESSAGE" | jq -Rsa .)
curl -s -o /dev/null -XPOST "$SLACK_WEBHOOK_URL" -d '{
  "text": '"$escaped_message"'
}'
