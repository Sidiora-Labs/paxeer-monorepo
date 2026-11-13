#!/usr/bin/env bash
# Shared deployment compiler and packager, also used for historical source trees.
PAXAI_GUEST_OPT_LEVEL=z
PAXAI_GUEST_CRATE_TYPE=cdylib
PAXAI_GUEST_LLVM_PASSES='enable-merge-functions ir-outliner inline-threshold=40'
PAXAI_GUEST_OPTIMIZER_VERSION='wasm-opt version 133 (version_133)'
PAXAI_GUEST_RUSTFLAGS=(
    -C target-feature=-reference-types
    -C link-arg=--compress-relocations
    -C llvm-args=--enable-merge-functions
    -C llvm-args=--ir-outliner
    -C llvm-args=--inline-threshold=40
)
PAXAI_GUEST_PASSES=(
    -Oz --flatten --rereloop -Oz --converge --merge-similar-functions
    -Oz --converge --strip-debug --strip-producers
)

paxai_guest_require_optimizer() {
    command -v wasm-opt >/dev/null 2>&1 || {
        echo 'paxai-host-boundary: wasm-opt is required' >&2
        return 1
    }
    if [[ "$(wasm-opt --version)" != "$PAXAI_GUEST_OPTIMIZER_VERSION" ]]; then
        echo 'AI market guest requires Binaryen 133' >&2
        return 1
    fi
}

paxai_guest_compile() (
    set -euo pipefail
    paxai_guest_require_optimizer
    cd "$1"
    # An encoded flag variable would silently override the shared arguments.
    unset CARGO_ENCODED_RUSTFLAGS
    export CARGO_PROFILE_RELEASE_OPT_LEVEL="$PAXAI_GUEST_OPT_LEVEL"
    export CARGO_PROFILE_RELEASE_LTO=true
    export CARGO_PROFILE_RELEASE_CODEGEN_UNITS=1
    export CARGO_PROFILE_RELEASE_PANIC=abort
    export CARGO_PROFILE_RELEASE_STRIP=true
    export RUSTFLAGS="${PAXAI_GUEST_RUSTFLAGS[*]}"
    "${PROGRAMS_CARGO:-cargo}" rustc --locked --release --target wasm32-unknown-unknown \
        -p layerx-programs-ai-market --lib --crate-type "$PAXAI_GUEST_CRATE_TYPE"
)

paxai_guest_package() (
    set -euo pipefail
    paxai_guest_require_optimizer
    wasm-opt "${PAXAI_GUEST_PASSES[@]}" "$1" -o "$2"
)

paxai_guest_build() (
    set -euo pipefail
    local source_dir output target_dir
    source_dir=$(cd "$1" && pwd)
    output=$(realpath -m "$2")
    paxai_guest_compile "$source_dir"
    target_dir=${CARGO_TARGET_DIR:-target}
    if [[ "$target_dir" != /* ]]; then
        target_dir="$source_dir/$target_dir"
    fi
    paxai_guest_package "$target_dir/wasm32-unknown-unknown/release/layerx_programs_ai_market.wasm" "$output"
)

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    set -euo pipefail
    case "${1:-}" in
        check) [[ $# == 1 ]]; paxai_guest_require_optimizer ;;
        compile) [[ $# == 2 ]]; paxai_guest_compile "$2" ;;
        package) [[ $# == 3 ]]; paxai_guest_package "$2" "$3" ;;
        build) [[ $# == 3 ]]; paxai_guest_build "$2" "$3" ;;
        *) echo 'usage: guest-recipe.sh check | compile SOURCE_DIR | package RAW OUTPUT | build SOURCE_DIR OUTPUT' >&2; exit 1 ;;
    esac
fi
