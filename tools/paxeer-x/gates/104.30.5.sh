#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P)"
cd "$root"
exec python3 -B "$root/tools/paxeer-x/gates/profile2_native.py" "$@"
