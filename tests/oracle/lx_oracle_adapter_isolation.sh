#!/bin/bash
set -euo pipefail
shopt -s globstar nullglob dotglob

inspected_objects=0

for directory in modules ledger state protocol; do
    object_directory="build/obj/src/$directory"
    if [ ! -d "$object_directory" ]; then
        printf 'Missing built object directory: %s\n' "$object_directory" >&2
        exit 1
    fi
    for object in "$object_directory"/**/*.o; do
        if [ ! -f "$object" ]; then
            continue
        fi
        undefined_symbols=$(nm -u "$object")
        inspected_objects=$((inspected_objects + 1))
        if printf '%s\n' "$undefined_symbols" |
            awk '{print $NF}' |
            grep -E '^(socket|connect|getaddrinfo|gethostbyname|send|recv|curl_easy_)' >/dev/null; then
            exit 1
        fi
    done
done

if [ "$inspected_objects" -eq 0 ]; then
    printf 'No built objects inspected for oracle adapter isolation\n' >&2
    exit 1
fi
