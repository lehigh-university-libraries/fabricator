#!/usr/bin/env bash

set -eou pipefail

regex='^(https:\/\/docs\.google\.com\/spreadsheets\/d\/[a-zA-Z0-9_-]+)'
if [[ "$URL" =~ $regex ]]; then
  URL="${BASH_REMATCH[1]}"
  echo "Valid Google Sheets URL: $URL"
else
  echo "Invalid URL: $URL"
  exit 1
fi

# Regular expression for valid Google Sheets ranges or sheet names
# Matches:
# - Sheet names (e.g., Sheet1)
# - Cell ranges (e.g., A1:B10)
regex='^([A-Za-z]+[0-9]*|[A-Za-z]+[0-9]+:[A-Za-z]+[0-9]+)$'
if [[ "$RANGE" =~ $regex ]]; then
  echo "Valid range or sheet name"
else
  echo "Invalid range"
  exit 1
fi
