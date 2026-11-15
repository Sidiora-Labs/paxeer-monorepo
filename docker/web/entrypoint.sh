#!/bin/sh
set -eu

umask 077
for directory in "${LAYERX_RUM_STORAGE_DIRECTORY:?Performance storage directory is required}" \
    "${LAYERX_SUPPORT_REPORT_DIR:?Support report storage directory is required}"; do
    case "$directory" in
        /*) [ "$directory" != / ] ;;
        *) exit 1 ;;
    esac
    [ ! -L "$directory" ] || exit 1
    mkdir -p "$directory"
    chmod 0700 "$directory"
    if [ "$(id -u)" = 0 ]; then
        chown 65532:65532 "$directory"
    fi
done

if [ "$(id -u)" = 0 ]; then
    exec gosu 65532:65532 "$@"
fi
exec "$@"
