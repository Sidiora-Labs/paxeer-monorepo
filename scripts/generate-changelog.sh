#!/usr/bin/env bash
# Generate a changelog from resolved local, remote or explicitly fetched releases.
# Usage: CHANGELOG_REPO=owner/repository ./scripts/generate-changelog.sh BASE HEAD
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ "$#" -ne 2 ]; then
    echo "Usage: CHANGELOG_REPO=owner/repository $0 <base-branch-or-commit> <head-branch-or-commit>" >&2
    exit 1
fi
# Explicit release metadata avoids guessing the owner from a fork's remote URL.
exec bash "$SCRIPT_DIR/maintenance.sh" changelog "${CHANGELOG_REPO:-}" "$1" "$2"
