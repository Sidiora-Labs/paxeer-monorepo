#!/bin/sh
set -eu
config_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec rosetta-fixture send --config "$config_dir" "$@"
