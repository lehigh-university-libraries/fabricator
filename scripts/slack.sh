#!/usr/bin/env bash

set -eou pipefail

escaped_message=$(echo "$MESSAGE" | jq -Rsa .)
curl -s -o /dev/null -XPOST "$SLACK_WEBHOOK_URL" -d '{
  "text": '"$escaped_message"'
}'
