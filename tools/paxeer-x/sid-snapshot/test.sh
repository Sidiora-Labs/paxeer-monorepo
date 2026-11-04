#!/usr/bin/env bash
# Transfer real SID on a disposable loopback paxd chain, then export a fixed
# committed block. See README.md for funded, unlocked account prerequisites.
set -euo pipefail
here="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$here/contract_test.py" "$@"
