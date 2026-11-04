#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 1 || ! $1 =~ ^[0-9a-f]{40}$ ]]; then
  echo 'usage: generate-from-remote.sh UPSTREAM_SCHEMA_REVISION' >&2
  exit 2
fi
yarn getV2
yarn merge
yarn generate "$1"
yarn format
