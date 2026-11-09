#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR/.."
STAGING=$(mktemp -d ./docs/.swagger-statik.XXXXXXXX)
trap 'rm -rf -- "$STAGING"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

case "${1:-}" in
    '') ignite generate openapi -y ;;
    --assets-only) [ "$#" -eq 1 ] ;;
    *) echo "usage: $0 [--assets-only]" >&2; exit 2 ;;
esac
python3 "$SCRIPT_DIR/swagger_assets.py" prepare docs/swagger-ui "$STAGING/assets"
"${STATIK:-statik}" -src="$STAGING/assets" -dest="$STAGING" -f -m -p swagger
python3 "$SCRIPT_DIR/swagger_assets.py" "$STAGING/swagger/statik.go" docs/swagger/statik.go
