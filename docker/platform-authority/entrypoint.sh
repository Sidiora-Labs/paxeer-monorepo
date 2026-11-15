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

exec /usr/local/bin/layerx-env-files /usr/local/bin/layerx-runtime-clock \
	--runtime-dir /tmp -- /usr/local/bin/layerx-receipt-authority "$@"
