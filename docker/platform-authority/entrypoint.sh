#!/bin/sh
set -eu

port=${PORT:?PORT is required}
listen=${LAYERX_AUTHORITY_LISTEN:?LAYERX_AUTHORITY_LISTEN is required}
case "$port" in
'' | *[!0-9]*) echo "authority startup: PORT must be a decimal TCP port" >&2; exit 1 ;;
esac
if ! [ "$port" -ge 1 ] || ! [ "$port" -le 65535 ]; then
	echo "authority startup: PORT must be in 1..65535" >&2
	exit 1
fi
if [ "$listen" != "[::]:$port" ]; then
	echo "authority startup: LAYERX_AUTHORITY_LISTEN must bind PORT on all IPv6 interfaces" >&2
	exit 1
fi

if [ "${LAYERX_AUTHORITY_REPLICA_TOKEN_B64+x}" ]; then
    replica_file=/run/layerx-authority/replica-token
    replica_tmp=$(mktemp /run/layerx-authority/.replica-token.XXXXXX)
    trap 'rm -f "$replica_tmp"' EXIT HUP INT TERM
    if ! printf '%s' "$LAYERX_AUTHORITY_REPLICA_TOKEN_B64" | base64 -d >"$replica_tmp"; then
        echo "authority startup: LAYERX_AUTHORITY_REPLICA_TOKEN_B64 must be base64" >&2
        exit 1
    fi
    chmod 0600 "$replica_tmp"
    mv -f "$replica_tmp" "$replica_file"
    trap - EXIT HUP INT TERM
    LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE=$replica_file
    export LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE
    unset LAYERX_AUTHORITY_REPLICA_TOKEN_B64
fi

exec /usr/local/bin/layerx-env-files /usr/local/bin/layerx-runtime-clock \
	--runtime-dir /tmp -- /usr/local/bin/layerx-receipt-authority "$@"
