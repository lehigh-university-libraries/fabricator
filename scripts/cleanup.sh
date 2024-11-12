#!/usr/bin/env bash

set -eou pipefail

if [ -f "fabricator_pid.txt" ]; then
  PID="$(cat fabricator_pid.txt)"
  pgrep -f "fabricator" | grep "$PID" && kill "$PID" || echo "No fabricator process"
  rm fabricator_pid.txt
fi
