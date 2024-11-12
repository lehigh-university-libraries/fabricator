#!/usr/bin/env bash

set -eou pipefail

if [ ! -d logs ]; then
  mkdir -p logs
fi

if [ ! -d configs ]; then
  mv ../workbench-configs configs
fi

mv ../*.csv input_data/

# if we had linked agents with additional metadata
# run that job first
if [ -f target.agents.csv ]; then
  python3 workbench --config configs/terms.yml

  # fail the job if workbench logged any errors
  grep ERROR logs/agents.log && exit 1 || echo "No errors"
fi

# run the ingest
python3 workbench --config configs/create.yml

# fail the job if workbench logged any errors
grep ERROR logs/items.log && exit 1 || echo "No errors"
