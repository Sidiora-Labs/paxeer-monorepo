#!/bin/sh
set -eu

unsafe_exceptions() {
    cat <<'UNSAFE_EXCEPTIONS'
# Foreign function declaration and export for migration ingress.
layerx-programs-runtime/src/ffi.rs
# Foreign function declarations and exports for CALL ingress.
layerx-programs-runtime/src/ffi_call.rs
# Foreign function declaration for interface ingress.
layerx-programs-runtime/src/ffi_interface.rs
# Foreign function declarations and exports for transfer ingress.
layerx-programs-runtime/src/ffi_transfer.rs
# Foreign function exports for the native contract.
layerx-programs-runtime/src/native_contract.rs
# Guest cell of the single threaded guest.
layerx-programs-ai-market/src/guest_cell.rs
# Foreign function declarations and exports for sandbox host ingress.
layerx-programs-sandbox/src/host_ffi.rs
# Foreign function declaration and export for sandbox terminal publication.
layerx-programs-sandbox/src/host_ffi/terminal.rs
# Foreign function declaration retaining sandbox host exports.
layerx-programs-sandbox/src/lib.rs
# Foreign function link of the sandbox archive test.
layerx-programs-sandbox/tests/repair_22_33_231.rs
# Foreign function link of the sandbox host export test.
layerx-programs-sandbox/tests/host_ffi_link.rs
# Foreign function link of the runtime native contract test.
layerx-programs-runtime/tests/repair_22_31_259.rs
# Counting allocator of the runtime receipt decoder test.
layerx-programs-runtime/tests/repair_22_33_226.rs
# Counting allocator of the runtime guest execution test.
layerx-programs-runtime/tests/repair_22_33_227.rs
# Counting allocator of the runtime host state decoder test.
layerx-programs-runtime/tests/repair_22_33_258.rs
# Foreign function link of the runtime interface test.
layerx-programs-runtime/tests/repair_22_33_259.rs
# Foreign function link of the arbiter signature test.
layerx-programs-arbiter/tests/proof_arbiter_crate.rs
UNSAFE_EXCEPTIONS
}

check_unsafe_code() (
    crates_root=$1
    search_tool=$2
    case "$search_tool" in
        rg)
            if unsafe_hits=$(rg -n --glob '*.rs' \
                '\bunsafe\s+(fn|trait|impl|extern)|\bunsafe\s*\{' "$crates_root"); then
                :
            else
                status=$?
                [ "$status" -eq 1 ] || return "$status"
            fi
            ;;
        grep)
            if unsafe_hits=$(grep -rEn --include='*.rs' \
                '\bunsafe[[:space:]]+(fn|trait|impl|extern)|\bunsafe[[:space:]]*\{' "$crates_root"); then
                :
            else
                status=$?
                [ "$status" -eq 1 ] || return "$status"
            fi
            ;;
        *) return 2 ;;
    esac
    unsafe_hits=$(printf '%s\n' "$unsafe_hits" | awk \
        -v root="$crates_root/" -v exceptions="$(unsafe_exceptions)" '
        BEGIN {
            count = split(exceptions, paths, "\n")
            for (i = 1; i <= count; i++) {
                if (paths[i] != "" && paths[i] !~ /^#/) excepted[root paths[i]] = 1
            }
        }
        NF && !(substr($0, 1, index($0, ":") - 1) in excepted)
    ')
    if [ -n "$unsafe_hits" ]; then
        printf '%s\n' "$unsafe_hits" >&2
        echo "programs unsafe policy: unsafe code is forbidden" >&2
        return 1
    fi
)

unsafe_policy_self_test() (
    fixture_root=$(mktemp -d)
    trap 'rm -rf -- "$fixture_root"' EXIT HUP INT TERM
    mkdir -p "$fixture_root/crates/layerx-programs-runtime/src"
    printf '%s\n' 'unsafe extern "C" {}' >"$fixture_root/crates/layerx-programs-runtime/src/ffi.rs"
    for search_tool in rg grep; do
        if [ "$search_tool" = rg ] && ! command -v rg >/dev/null 2>&1; then
            continue
        fi
        check_unsafe_code "$fixture_root/crates" "$search_tool"
        outside="$fixture_root/crates/layerx-programs-runtime/src/ffi_extra.rs"
        printf '%s\n' 'unsafe fn outside_boundary() {}' >"$outside"
        if check_unsafe_code "$fixture_root/crates" "$search_tool" >"$fixture_root/output" 2>&1; then
            echo "programs unsafe policy self-test: accepted a file outside the exception list" >&2
            return 1
        else
            status=$?
            [ "$status" -eq 1 ] || return "$status"
        fi
        grep -F "$outside:1:unsafe fn outside_boundary() {}" "$fixture_root/output" >/dev/null
        grep -Fx 'programs unsafe policy: unsafe code is forbidden' "$fixture_root/output" >/dev/null
        rm -- "$outside"
    done
)

unsafe_policy_self_test
if [ "${1:-}" = --self-test ]; then
    echo "programs unsafe policy self-test passed"
    exit 0
fi

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
metadata_file=$(mktemp)
license_file=$(mktemp)
vendor_file=$(mktemp)
trap 'rm -f -- "$metadata_file" "$license_file" "$vendor_file"' EXIT HUP INT TERM

cd "$repo_root/programs"
cargo metadata --manifest-path "$repo_root/programs/Cargo.toml" \
    --locked --format-version 1 --features layerx-programs-sandbox/host-ffi >"$metadata_file"

banned='^(bindgen|libsqlite3-sys|rusqlite|sqlx-sqlite|ctor|inventory|getrandom|rand|chrono|time|instant|tokio|mio|socket2|wasi|wasmtime)$'
if jq -r '.packages[].name' "$metadata_file" | grep -Eq "$banned"; then
    echo "programs dependency policy: forbidden boundary, clock, randomness or network crate" >&2
    exit 1
fi
if jq -e '
    . as $metadata
    | [$metadata.packages[] | select(.name == "rand_core") | .id] as $ids
    | any($metadata.resolve.nodes[];
        ((.id as $id | $ids | index($id)) != null)
        and any(.features[]; . == "getrandom" or . == "std"))
' "$metadata_file" >/dev/null; then
    echo "programs dependency policy: rand_core entropy features are forbidden" >&2
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
        echo "programs dependency policy: $package has no SPDX license" >&2
        exit 1
    fi
    case "$license" in
        *" AND "*)
            tokens=$(printf '%s\n' "$license" | sed 's/[()]/ /g; s|/| |g; s/ AND / /g; s/ OR / /g; s/ WITH / /g')
            for token in $tokens; do
                if ! allowlisted_license_token "$token"; then
                    echo "programs dependency policy: $package uses non-allowlisted license $license" >&2
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
                echo "programs dependency policy: $package uses non-allowlisted license $license" >&2
                exit 1
            fi
            ;;
    esac
done <"$license_file"

find "$repo_root/programs/vendor" -mindepth 2 -maxdepth 2 \
    -name Cargo.toml -print | while IFS= read -r manifest; do
    directory=$(dirname -- "$manifest")
    [ -f "$directory/.cargo-checksum.json" ] || continue
    awk '
        /^\[package\][[:space:]]*$/ { in_package = 1; next }
        /^\[/ && in_package { exit }
        in_package && /^[[:space:]]*name[[:space:]]*=/ && name == "" {
            line = $0
            sub(/^[^=]*=[[:space:]]*/, "", line)
            gsub(/^"|"[[:space:]]*$/, "", line)
            name = line
        }
        in_package && /^[[:space:]]*version[[:space:]]*=/ && version == "" {
            line = $0
            sub(/^[^=]*=[[:space:]]*/, "", line)
            gsub(/^"|"[[:space:]]*$/, "", line)
            version = line
        }
        END { if (name != "" && version != "") print name "\t" version }
    ' "$manifest"
done | sort -u >"$vendor_file"

jq -r '.packages[] | select(.source != null) | [.name, .version] | @tsv' \
    "$metadata_file" | while IFS="$(printf '\t')" read -r package version; do
    if ! awk -F '\t' -v package="$package" -v version="$version" \
        '$1 == package && $2 == version { found = 1 } END { exit !found }' \
        "$vendor_file"; then
        echo "programs vendoring policy: $package $version is not vendored with a checksum" >&2
        exit 1
    fi
done

wasmi_dependency=$(grep '^wasmi = ' "$repo_root/programs/Cargo.toml" || true)
case "$wasmi_dependency" in
    *'path = "vendor/wasmi-0.31.2"'*'version = "=0.31.2"'*|\
    *'version = "=0.31.2"'*'path = "vendor/wasmi-0.31.2"'*)
        ;;
    *)
    echo "programs vendoring policy: the WASM engine must stay pinned to an exact revision" >&2
    exit 1
        ;;
esac
test -r "$repo_root/programs/.cargo/config.toml"
if ! grep -q 'replace-with = "vendored-sources"' "$repo_root/programs/.cargo/config.toml"; then
    echo "programs vendoring policy: builds must resolve the engine from programs/vendor" >&2
    exit 1
fi

if command -v rg >/dev/null 2>&1; then
    check_unsafe_code "$repo_root/programs/crates" rg
else
    check_unsafe_code "$repo_root/programs/crates" grep
fi

if command -v rg >/dev/null 2>&1; then
    float_hits=$(rg -n --glob '*.rs' '\b(f32|f64)\b' "$repo_root/programs/crates" || true)
else
    float_hits=$(grep -rEn --include='*.rs' '\b(f32|f64)\b' "$repo_root/programs/crates" || true)
fi
if [ -n "$float_hits" ]; then
    printf '%s\n' "$float_hits" >&2
    echo "programs integer-only policy: floating-point types are forbidden in consensus-adjacent code" >&2
    exit 1
fi

test -r "$repo_root/programs/deny.toml"
echo "programs dependency, vendoring, unsafe and integer-only policies passed"
