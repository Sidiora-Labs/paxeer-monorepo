#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../../.."

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
make_bin=$(command -v make)

digest_target=$(cat <<'MAKE'
.PHONY: print-native-archive-digest
print-native-archive-digest:
	@printf '%s\n' '$(native_archive_digest)'
MAKE
)

native_archive_digest() {
    PATH="$1" "$make_bin" --no-print-directory -s -f chain.mk \
        --eval "$digest_target" LEDGER_ENABLED=false \
        NATIVE_ARCHIVES="$tmp/archive-a $tmp/archive-b" \
        print-native-archive-digest
}

native_archive_digest_fails_closed() {
    local expected actual tool mode output

    printf 'first archive\000\377\n' > "$tmp/archive-a"
    printf 'second archive\000\001\n' > "$tmp/archive-b"
    expected=$(cat "$tmp/archive-a" "$tmp/archive-b" | sha256sum)
    expected=${expected%% *}

    for mode in sha256sum shasum neither; do
        mkdir "$tmp/$mode"
        for tool in cat git sed sort tail go uname find printf; do
            ln -s "$(type -P "$tool")" "$tmp/$mode/$tool"
        done
    done
    ln -s "$(command -v sha256sum)" "$tmp/sha256sum/sha256sum"
    ln -s "$(command -v shasum)" "$tmp/sha256sum/shasum"
    ln -s "$(command -v shasum)" "$tmp/shasum/shasum"

    for mode in sha256sum shasum; do
        actual=$(native_archive_digest "$tmp/$mode")
        if [[ "$actual" != "$expected" ]]; then
            printf '%s digest: expected %s, got %s\n' "$mode" "$expected" "$actual" >&2
            return 1
        fi
    done

    if output=$(native_archive_digest "$tmp/neither" 2>&1); then
        printf 'make succeeded without a native archive hasher: %s\n' "$output" >&2
        return 1
    fi
    if [[ "$output" != *"cannot hash native archives $tmp/archive-a $tmp/archive-b"* ]]; then
        printf 'make did not report the hash failure: %s\n' "$output" >&2
        return 1
    fi

    rm "$tmp/archive-b"
    if output=$(native_archive_digest "$tmp/sha256sum" 2>&1); then
        printf 'make succeeded with a missing native archive: %s\n' "$output" >&2
        return 1
    fi
    if [[ "$output" != *"native archives missing: $tmp/archive-b; run paxd-native-libs"* ]]; then
        printf 'make did not report the missing archive: %s\n' "$output" >&2
        return 1
    fi

    printf 'PASS native_archive_digest_fails_closed\n'
}

native_archive_digest_fails_closed
