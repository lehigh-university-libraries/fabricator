#!/usr/bin/env bash

set -eou pipefail

# extract sheet ID from https://docs.google.com/spreadsheets/d/foo/edit?gid=0#gid=0
SHEET_ID=$(echo "$1" | sed -n 's|.*/d/\(.*\)/.*|\1|p')
RANGE=$2
ACCESS_TOKEN=$3

# Fetch data from the Google Sheet
response=$(curl -s \
    "https://sheets.googleapis.com/v4/spreadsheets/$SHEET_ID/values/$RANGE" \
    -H "Authorization: Bearer $ACCESS_TOKEN")

json_data=$(echo "$response" | jq -r '.values | map(map(tostring))')
echo "$json_data" > csv.json
