#!/bin/sh
set -eu
config_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# Restore the pinned fixture identity and derive all consumers from its bank store.
# An existing node home is refused, never silently destroyed.
exec rosetta-fixture restore --config "$config_dir" "$@"
