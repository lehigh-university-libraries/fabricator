#!/usr/bin/env bash

set -eou pipefail

WORKING_DIR="$HOME/etds"
PROCESSED_DIR="$HOME/etds.processed"

cd "$WORKING_DIR"
mv ./*.zip "$PROCESSED_DIR/"

rm -rf ./*
