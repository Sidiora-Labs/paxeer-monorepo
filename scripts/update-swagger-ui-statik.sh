#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$SCRIPT_DIR/.."
STAGING=$(mktemp -d ./docs/.swagger-statik.XXXXXXXX)
trap 'rm -rf -- "$STAGING"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

ignite generate openapi -y
statik -src=./docs/swagger-ui -dest="$STAGING" -f -p swagger
python3 "$SCRIPT_DIR/swagger_assets.py" "$STAGING/swagger/statik.go" docs/swagger/statik.go
