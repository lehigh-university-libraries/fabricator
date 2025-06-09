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

# if we had linked agents with additional metadata
# run that job first
if [ -f input_data/target.agents.csv ]; then
  python3 workbench --config configs/terms.yml

  # fail the job if workbench logged any errors
  grep ERROR logs/agents.log && exit 1 || echo "No errors"
fi

if [ -f input_data/target.update.csv ]; then
  python3 workbench --config configs/update.yml
  grep ERROR logs/update.log | grep -Ev '"supplemental_file" in .* not created because CSV field is empty' && exit 1 || echo "No errors"
fi

if [ -f input_data/target.csv ]; then
  python3 workbench --config configs/create.yml
  grep ERROR logs/items.log | grep -Ev '"supplemental_file" in .* not created because CSV field is empty' && exit 1 || echo "No errors"
fi

