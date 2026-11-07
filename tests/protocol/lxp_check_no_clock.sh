#!/bin/sh
set -eu

# The shared checker requires rg, preflights every source input, and accepts
# only rg exit 1 (exhaustive no-match); matches and tool errors both refuse.
command -v rg >/dev/null 2>&1 || {
    echo "clock scan requires rg" >&2
    exit 1
}
exec go run ./tests/protocol/cmd/protocol-policy clock --root .
