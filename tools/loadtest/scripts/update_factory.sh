#!/usr/bin/env bash
set -euo pipefail
exec "$(dirname "$0")/../build/paxload" scaffold factory generator/scenarios/factory.go "$@"
