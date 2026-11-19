#!/usr/bin/env bash
set -euo pipefail

for attempt in {1..3}; do
  if sudo apt-get update && sudo apt-get install "$@"; then
    exit 0
  fi
  if [ "$attempt" -lt 3 ]; then
    sleep 5
  fi
done
exit 1
