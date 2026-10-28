#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
metadata_file=$(mktemp)
license_file=$(mktemp)
trap 'rm -f -- "$metadata_file" "$license_file"' EXIT HUP INT TERM

case "$#" in
    0)
        cargo metadata --manifest-path "$repo_root/services/platform/Cargo.toml" \
            --locked --format-version 1 >"$metadata_file"
        cargo metadata --manifest-path "$repo_root/tools/platform/Cargo.toml" \
            --locked --format-version 1 >>"$metadata_file"
        cargo metadata --manifest-path "$repo_root/sdk/platform/generators/Cargo.toml" \
            --locked --format-version 1 >>"$metadata_file"
        ;;
    1)
        cat -- "$1" >"$metadata_file"
        ;;
    *)
        echo "usage: dependency-policy.sh [metadata.json]" >&2
        exit 2
        ;;
esac

banned='^(bindgen|libsqlite3-sys|rusqlite|sqlx-sqlite|layerx-core|layerx-protocol-core|layerx-paxeer-client|layerx-intents|ctor|inventory)$'
banned_packages=$(jq -r --arg banned "$banned" \
    '[.packages[].name | select(test($banned))] | join("\n")' "$metadata_file")
if [ -n "$banned_packages" ]; then
    printf 'platform dependency policy: forbidden boundary or global-initialisation crate\n%s\n' "$banned_packages" >&2
    exit 1
fi

allowlisted_license_token() {
    case "$1" in
        Apache-2.0|BSD-1-Clause|BSD-2-Clause|BSD-3-Clause|CC0-1.0|ISC|MIT|Unicode-3.0|Zlib|LLVM-exception)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

jq -r '.packages[] | select(.source != null) | [.name, (.license // "")] | @tsv' \
    "$metadata_file" >"$license_file"
while IFS="$(printf '\t')" read -r package license; do
    if [ -z "$license" ]; then
        echo "platform dependency policy: $package has no SPDX license" >&2
        exit 1
    fi
    case "$license" in
        *" AND "*)
            tokens=$(printf '%s\n' "$license" | sed 's/[()]/ /g; s|/| |g; s/ AND / /g; s/ OR / /g; s/ WITH / /g')
            for token in $tokens; do
                if ! allowlisted_license_token "$token"; then
                    echo "platform dependency policy: $package uses non-allowlisted license $license" >&2
                    exit 1
                fi
            done
            ;;
        *)
            alternatives=$(printf '%s\n' "$license" | sed 's/[()]//g; s|/|\n|g; s/ OR /\n/g')
            satisfied=0
            while IFS= read -r alternative; do
                [ -n "$alternative" ] || continue
                alternative_ok=1
                for token in $(printf '%s\n' "$alternative" | sed 's/ WITH / /g'); do
                    if ! allowlisted_license_token "$token"; then
                        alternative_ok=0
                    fi
                done
                if [ "$alternative_ok" -eq 1 ]; then
                    satisfied=1
                fi
            done <<LICENSE_ALTERNATIVES
$alternatives
LICENSE_ALTERNATIVES
            if [ "$satisfied" -ne 1 ]; then
                echo "platform dependency policy: $package uses non-allowlisted license $license" >&2
                exit 1
            fi
            ;;
    esac
done <"$license_file"

if command -v rg >/dev/null 2>&1; then
    unsafe_hits=$(rg -n --glob '*.rs' '\bunsafe\s+(fn|trait|impl|extern)|\bunsafe\s*\{' "$repo_root/tools/platform/cli" "$repo_root/sdk/platform" "$repo_root/tools/platform/release" "$repo_root/services/platform/tools" || true)
else
    unsafe_hits=$(grep -rEn --include='*.rs' '\bunsafe[[:space:]]+(fn|trait|impl|extern)|\bunsafe[[:space:]]*\{' "$repo_root/tools/platform/cli" "$repo_root/sdk/platform" "$repo_root/tools/platform/release" "$repo_root/services/platform/tools" || true)
fi
if [ -n "$unsafe_hits" ]; then
    printf '%s\n' "$unsafe_hits" >&2
    echo "platform unsafe policy: unsafe code is forbidden" >&2
    exit 1
fi

echo "platform dependency and unsafe policies passed"
