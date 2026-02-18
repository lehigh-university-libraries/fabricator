#!/usr/bin/env bash

set -eou pipefail

if [ ! -d logs ]; then
  mkdir -p logs
fi

if [ ! -d configs ]; then
  mv ../workbench-configs configs
fi

mv ../*.csv input_data/

export REQUESTS_CA_BUNDLE

if [ -f input_data/target.update.csv ]; then
  python3 workbench --config configs/update.yml
  grep ERROR logs/update.log | grep -Ev '"supplemental_file" in .* not created because CSV field is empty' && exit 1 || echo "No errors"
fi

if [ -f input_data/target.csv ]; then
  python3 workbench --config configs/create.yml
  grep ERROR logs/items.log | grep -Ev '"supplemental_file" in .* not created because CSV field is empty' && exit 1 || echo "No errors"
fi
