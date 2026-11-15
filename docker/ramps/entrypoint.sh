#!/bin/sh
set -eu

: "${PORT:?PORT is required}"
case "$PORT" in
	'' | *[!0-9]*)
		echo "ramp-entrypoint: PORT must be an integer from 1 to 65535" >&2
		exit 2
		;;
esac
if [ "${#PORT}" -gt 5 ] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
	echo "ramp-entrypoint: PORT must be an integer from 1 to 65535" >&2
	exit 2
fi
export RAMP_LISTEN="[::]:$PORT"
exec /usr/local/bin/layerx-env-files /usr/local/bin/ramp-env-init "$@"
