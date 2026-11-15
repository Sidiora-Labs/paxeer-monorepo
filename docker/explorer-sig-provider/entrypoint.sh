#!/bin/sh
set -eu

PORT="${PORT-8050}"
case "$PORT" in
    ''|*[!0-9]*|0*|??????*)
        echo 'PORT must be a decimal integer between 1 and 65535' >&2
        exit 1
        ;;
esac
if [ "$PORT" -gt 65535 ]; then
    echo 'PORT must be a decimal integer between 1 and 65535' >&2
    exit 1
fi

export PORT
export SIG_PROVIDER__SERVER__HTTP__ADDR="${SIG_PROVIDER__SERVER__HTTP__ADDR:-[::]:${PORT}}"
exec "$@"
