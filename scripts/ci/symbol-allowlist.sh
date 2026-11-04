#!/bin/sh
set -eu

build_dir=${1:-build}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
python3 "$script_dir/audit_object_checks.py" symbols "$build_dir"

if find src/crypto -type f -name '*.c' -print0 | \
    xargs -0 grep -nE '(^|[^[:alnum:]_])memcmp[[:space:]]*\('; then
    echo "crypto source calls plain memcmp instead of lxp_ct_memcmp" >&2
    exit 1
fi

if grep -nE 'ctx_(set_)?balance|balance[[:space:]]*=' \
    include/layerx/lxp_module.h; then
    echo "module ABI exposes a balance-write capability" >&2
    exit 1
fi
