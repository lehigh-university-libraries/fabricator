#!/usr/bin/env bash

set -eou pipefail


if [[ "$ETD" =~ ^(true|false)$ ]]; then
  echo "Valid ETD value: $ETD"
else
  echo "Invalid ETD value!" >&2
  exit 1
fi

if [ "$ETD" != "true" ]; then
  exit 0
fi

WORKING_DIR="$HOME/etds"
PROCESSED_DIR="$HOME/etds.processed"

if [ -z "$(ls -A "$WORKING_DIR" 2>/dev/null)" ]; then
  echo "$WORKING_DIR is empty"
  exit 0
fi

cd "$WORKING_DIR"
mv ./*.zip "$PROCESSED_DIR/"

rm -rf ./*
