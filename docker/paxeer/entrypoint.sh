#!/bin/sh
set -eu

port=${PORT:?PORT is required}
listen=${LAYERX_PAXEER_BOUNDARY_LISTEN:?LAYERX_PAXEER_BOUNDARY_LISTEN is required}
case "$port" in
'' | *[!0-9]*) echo "paxeer startup: PORT must be a decimal TCP port" >&2; exit 1 ;;
esac
if ! [ "$port" -ge 1 ] || ! [ "$port" -le 65535 ]; then
	echo "paxeer startup: PORT must be in 1..65535" >&2
	exit 1
fi
if [ "$listen" != "0.0.0.0:$port" ]; then
	echo "paxeer startup: LAYERX_PAXEER_BOUNDARY_LISTEN must bind PORT on all IPv4 interfaces" >&2
	exit 1
fi

exec /usr/local/bin/layerx-env-files /usr/local/bin/layerx-paxeer-boundary "$@"
