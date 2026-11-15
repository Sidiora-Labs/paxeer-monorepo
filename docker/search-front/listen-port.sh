#!/bin/sh
set -eu

: "${PORT:?PORT is required}"
case "$PORT" in
'' | *[!0-9]* | 0*)
	echo "search-front: PORT must be an integer from 1 to 65535" >&2
	exit 2
	;;
esac
if [ "${#PORT}" -gt 5 ] || [ "$PORT" -gt 65535 ]; then
	echo "search-front: PORT must be an integer from 1 to 65535" >&2
	exit 2
fi

template=${1:-/etc/nginx/nginx.conf.template}
out=${2:-/etc/nginx/nginx.conf}
sed 's/listen \(\[::\]:\)[0-9][0-9]* ipv6only=off;/listen \1'"$PORT"' ipv6only=off;/' "$template" >"$out.new"
mv "$out.new" "$out"
