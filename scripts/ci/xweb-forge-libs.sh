#!/usr/bin/env bash
# Verify the reviewed source identity before the xweb Forge build.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
exec python3 "$root/scripts/ci/xweb_dependency_identity.py"
