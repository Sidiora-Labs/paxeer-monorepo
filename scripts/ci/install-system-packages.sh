#!/usr/bin/env bash
set -euo pipefail

readonly APT_UPDATE_TIMEOUT_SECONDS=120
readonly APT_INSTALL_TIMEOUT_SECONDS=600

for attempt in {1..3}; do
  if [ "$attempt" -gt 1 ]; then
    sudo dpkg --configure -a || true
  fi
  if timeout --kill-after=10s "${APT_UPDATE_TIMEOUT_SECONDS}s" sudo apt-get update &&
    timeout --kill-after=10s "${APT_INSTALL_TIMEOUT_SECONDS}s" sudo apt-get install "$@"; then
    exit 0
  fi
  if [ "$attempt" -lt 3 ]; then
    sleep 5
  fi
done
exit 1
