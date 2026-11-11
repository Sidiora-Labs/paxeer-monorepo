#!/bin/sh
set -eu

inventory=${1:-tools/workspace/paxeer-manifest-roots.txt}
actual=$(mktemp)
declared=$(mktemp)
trap 'rm -f "$actual" "$declared"' EXIT HUP INT TERM

# The Paxeer chain sits at the repository root beside the LayerX trees, so the
# scan names every chain-owned root explicitly instead of walking the whole
# repository. A new chain top-level directory must be added here.
chain_roots="admin api tools/benchmark consensus custodyproof daemon docker engine tests/fixtures/example tools/hpx tests/integration interchain modules node tests/occ precompiles rpc sdk storage tests/testutil utils wasm wasm-runtime contracts tests/chain tools/chain tools/tx-scanner tools/utils"
for root in $chain_roots; do
    if test ! -d "$root"; then
        echo "Paxeer manifest scan root is not a directory: $root" >&2
        exit 1
    fi
done
{
    for manifest in go.mod foundry.paxeer.toml; do test -f "$manifest" && printf '%s\n' "$manifest"; done
    # shellcheck disable=SC2086
    find $chain_roots -path '*/node_modules' -prune -o -path '*/target' -prune -o \
        \( -name Cargo.toml -o -name go.mod -o -name package.json -o -name foundry.toml \) \
        -print
} | LC_ALL=C sort > "$actual"
awk -F '|' '!/^#/ && NF == 3 { print $2 }' "$inventory" | LC_ALL=C sort > "$declared"
cmp "$actual" "$declared"

while IFS='|' read -r kind path classification; do
    case "$kind" in \#*|'') continue ;; esac
    test -s "$path"
    case "$classification" in
        build_test_lint|build_static_live_test_blocked|build_static) ;;
        locked-rust-independent)
            test "$kind" = rust
            test -s "${path%/Cargo.toml}/Cargo.lock"
            ;;
        locked-pnpm-independent)
            test "$kind" = npm
            test -s "${path%/package.json}/pnpm-lock.yaml"
            ;;
        covered-by-package-build)
            test "$kind" = npm
            grep -q '"build"[[:space:]]*:' "$path"
            ;;
        covered-by-launchpad-lock)
            test "$kind" = npm
            case "$path" in sdk/launchpad/packages/*/package.json) ;; *) exit 1 ;; esac
            test -s apps/launchpad/pnpm-lock.yaml
            grep -Fq "../../${path%/package.json}:" apps/launchpad/pnpm-lock.yaml
            grep -Fq '../../sdk/launchpad/packages/*' apps/launchpad/pnpm-workspace.yaml
            ;;
        covered-by-cargo-workspace:*)
            test "$kind" = rust
            covering=${classification#covered-by-cargo-workspace:}
            case "$path" in "$covering"/*/Cargo.toml) ;; *) exit 1 ;; esac
            member=${path#"$covering"/}
            member=${member%/Cargo.toml}
            test -s "$covering/Cargo.lock"
            grep -q '^\[workspace\]' "$covering/Cargo.toml"
            grep -Fq "\"$member\"" "$covering/Cargo.toml"
            ;;
        covered-by-jupiter-lock)
            test "$kind" = rust
            test "$path" = tools/benchmark/loadtest/contracts/pax-cosmwasm/Cargo.toml
            test -s tools/benchmark/loadtest/contracts/jupiter/Cargo.lock
            grep -Fq 'path = "../pax-cosmwasm"' tools/benchmark/loadtest/contracts/jupiter/Cargo.toml
            ;;
        covered-by-wasmvm-vendor)
            test "$kind" = rust
            test "$path" = wasm-runtime/libwasmvm/vendor/cosmwasm-crypto/Cargo.toml
            test -s wasm-runtime/libwasmvm/Cargo.lock
            test -s wasm-runtime/libwasmvm/vendor/cosmwasm-crypto/NOTICE
            grep -Fq 'cosmwasm-crypto = { path = "vendor/cosmwasm-crypto" }' wasm-runtime/libwasmvm/Cargo.toml
            ;;
        covered-by-test:*)
            case "$kind" in go|rust) ;; *) exit 1 ;; esac
            covering=${classification#covered-by-test:}
            case "$covering" in *_test.go) ;; *) exit 1 ;; esac
            test -s "$covering"
            ;;
        vendored_source_checksum_bound)
            test "$path" = tools/benchmark/loadtest/contracts/evm/lib/openzeppelin-contracts/contracts/package.json
            test -s tools/benchmark/loadtest/contracts/evm/VENDORING.md
            test -x tools/benchmark/loadtest/contracts/evm/setup.sh
            ;;
        *) echo "unknown Paxeer manifest classification: $classification" >&2; exit 1 ;;
    esac
    if test "$kind" = rust; then
        case "$classification" in
            covered-by-cargo-workspace:*|covered-by-jupiter-lock|covered-by-wasmvm-vendor|covered-by-test:*) ;;
            *) test -s "${path%/Cargo.toml}/Cargo.lock" ;;
        esac
    fi
done < "$inventory"
