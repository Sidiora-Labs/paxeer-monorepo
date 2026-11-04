#!/bin/bash

set -euo pipefail

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
RELAYER_HOME="$SCRIPTDIR/.relayer"

if ! command -v rly >/dev/null || ! command -v python3 >/dev/null; then
  echo "Error: rly and python3 are required." >&2
  exit 1
fi

python3 "$SCRIPTDIR/harness.py" homes "$SCRIPTDIR"
exec python3 "$SCRIPTDIR/harness.py" transfer "$RELAYER_HOME" "$SCRIPTDIR/data"
