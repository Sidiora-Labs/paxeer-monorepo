#!/usr/bin/env bash
# Verify the reviewed source identity before the xweb Forge build.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)

fetch_missing_library() {
    local name="$1" url="$2" tag="$3" commit="$4"
    local dest="$root/contracts/lib/$name"
    if [ -e "$dest" ] || [ -L "$dest" ]; then
        return 0
    fi
    git init "$dest"
    git -C "$dest" remote add origin "$url"
    git -C "$dest" fetch --depth 1 origin "$commit"
    git -C "$dest" checkout --detach "$commit"
    git -C "$dest" tag "$tag" "$commit"
}

fetch_missing_library 'forge-std' 'https://github.com/foundry-rs/forge-std' 'v1.9.6' '3b20d60d14b343ee4f908cb8079495c07f5e8981' # v1.9.6
fetch_missing_library 'openzeppelin-contracts' 'https://github.com/OpenZeppelin/openzeppelin-contracts' 'v5.3.0' 'e4f70216d759d8e6a64144a9e1f7bbeed78e7079' # v5.3.0

exec python3 "$root/scripts/ci/xweb_dependency_identity.py"
