#!/usr/bin/env bash
# Fetches the two pinned Solidity libraries the Foundry builds need into
# contracts/lib. Neither library is committed, and this script is the only place
# their pins are written: the Paxeer forge workflow and the continuous
# integration solidity job both populate lib by calling it.
# Running it again with both libraries already at their pinned commit and unmodified
# does nothing; a checkout whose contents were edited is replaced.
set -euo pipefail

FORGE_STD_TAG='v1.9.6'
FORGE_STD_COMMIT='3b20d60d14b343ee4f908cb8079495c07f5e8981' # v1.9.6
FORGE_STD_URL='https://github.com/foundry-rs/forge-std'
OPENZEPPELIN_TAG='v5.3.0'
OPENZEPPELIN_COMMIT='e4f70216d759d8e6a64144a9e1f7bbeed78e7079' # v5.3.0
OPENZEPPELIN_URL='https://github.com/OpenZeppelin/openzeppelin-contracts'

lib_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/lib"

# Prints the commit the checkout at $1 is on, or nothing for a non-checkout.
checked_out_commit() {
    local dir="$1"
    [ -d "$dir/.git" ] || return 0
    git -C "$dir" rev-parse --verify HEAD 2>/dev/null || true
}

# Succeeds when the checkout at $1 carries the commit's own contents: no modified
# file, no deleted file and no extra file. A checkout that is at the pinned commit
# but was edited afterwards is not the pinned dependency, so it is not reused.
# A checkout git cannot read the status of is not pristine either.
checkout_is_pristine() {
    local dir="$1" status
    status="$(git -C "$dir" status --porcelain)" || return 1
    [ -z "$status" ]
}

fetch_library() {
    local name="$1" url="$2" tag="$3" commit="$4" current
    local dest="$lib_dir/$name"

    current="$(checked_out_commit "$dest")"
    if [ "$current" = "$commit" ] && checkout_is_pristine "$dest"; then
        printf 'bootstrap-libs: %s already at %s (%s)\n' "$name" "$tag" "$commit"
        return 0
    fi
    if [ -e "$dest" ]; then
        if [ "$current" = "$commit" ]; then
            printf 'bootstrap-libs: %s is at %s but its contents are modified, replacing it\n' "$name" "$tag" >&2
        elif [ -n "$current" ]; then
            printf 'bootstrap-libs: %s is at %s, replacing it with %s\n' "$name" "$current" "$tag" >&2
        else
            printf 'bootstrap-libs: %s is present but not a checkout of %s, replacing it\n' "$name" "$tag" >&2
        fi
        rm -rf -- "$dest"
    fi
    printf 'bootstrap-libs: fetching %s %s (%s)\n' "$name" "$tag" "$commit"
    git init "$dest"
    git -C "$dest" remote add origin "$url"
    git -C "$dest" fetch --depth 1 origin "$commit"
    git -C "$dest" checkout --detach "$commit"
    git -C "$dest" tag "$tag" "$commit"
}

mkdir -p -- "$lib_dir"
fetch_library 'forge-std' "$FORGE_STD_URL" "$FORGE_STD_TAG" "$FORGE_STD_COMMIT"
fetch_library 'openzeppelin-contracts' "$OPENZEPPELIN_URL" "$OPENZEPPELIN_TAG" "$OPENZEPPELIN_COMMIT"
