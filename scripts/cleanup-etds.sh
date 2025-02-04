#!/usr/bin/env bash

set -eou pipefail

WORKING_DIR="$HOME/etds"
cd "$WORKING_DIR"

mv ./*.zip ./etd* processed/
rm ./*.xml ./*.pdf
