#!/bin/sh
set -eu

scanner=scripts/workspace/check-paxeer-manifests.sh
roots=$(sed -n 's/^chain_roots="\(.*\)"$/\1/p' "$scanner")
test -n "$roots"
for root in $roots; do
    if test -z "$(git ls-files -- "$root" | head -n 1)"; then
        echo "chain root $root has no tracked files in a clean checkout" >&2
        exit 1
    fi
done
sh "$scanner"

repository=$(pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
fixture=$work/checkout
mkdir -p "$fixture"
for root in $roots; do mkdir -p "$fixture/$root"; done

copy_file() {
    mkdir -p "$fixture/$(dirname "$1")"
    cp "$repository/$1" "$fixture/$1"
}

# Exercise each new classification with repository manifests and their real covers.
awk -F '|' '$3 ~ /^(locked-rust-independent|locked-pnpm-independent|covered-by-)/' \
    tools/workspace/paxeer-manifest-roots.txt > "$work/inventory"
cat >> "$work/inventory" <<'EOF'
rust|tools/benchmark/loadtest/contracts/jupiter/Cargo.toml|build_test_lint
rust|wasm-runtime/libwasmvm/Cargo.toml|build_test_lint
EOF
while IFS='|' read -r kind path classification; do
    copy_file "$path"
    case "$classification" in
        locked-rust-independent|build_test_lint) copy_file "${path%/Cargo.toml}/Cargo.lock" ;;
        locked-pnpm-independent) copy_file "${path%/package.json}/pnpm-lock.yaml" ;;
        covered-by-test:*) copy_file "${classification#covered-by-test:}" ;;
    esac
done < "$work/inventory"
for cover in contracts/bridge/solana/Cargo.lock \
    apps/launchpad/pnpm-lock.yaml apps/launchpad/pnpm-workspace.yaml \
    wasm-runtime/libwasmvm/vendor/cosmwasm-crypto/NOTICE; do
    copy_file "$cover"
done

check_fixture() {
    (cd "$fixture" && sh "$repository/$scanner" "$work/inventory")
}

reject_fixture() {
    if check_fixture > "$work/output" 2>&1; then
        echo "check-paxeer-manifests accepted $1" >&2
        exit 1
    fi
}

check_fixture
rmdir "$fixture/tools/hpx"
reject_fixture 'a missing scan root'
grep -Fxq 'Paxeer manifest scan root is not a directory: tools/hpx' "$work/output"
printf 'not a directory\n' > "$fixture/tools/hpx"
reject_fixture 'a regular file as a scan root'
grep -Fxq 'Paxeer manifest scan root is not a directory: tools/hpx' "$work/output"
rm "$fixture/tools/hpx"
mkdir "$fixture/tools/hpx"

for cover in sdk/chainflow/rust/Cargo.lock \
    sdk/names/pnpm-lock.yaml apps/launchpad/pnpm-lock.yaml \
    apps/launchpad/pnpm-workspace.yaml contracts/bridge/solana/Cargo.lock \
    tools/benchmark/loadtest/contracts/jupiter/Cargo.lock \
    wasm-runtime/libwasmvm/Cargo.lock \
    wasm-runtime/libwasmvm/vendor/cosmwasm-crypto/NOTICE \
    sdk/chainflow/go/repair_m1306_test.go \
    sdk/launchpad/protocol/bindings/repair_37_test.go \
    sdk/launchpad/protocol/coverage/repair_28_test.go \
    sdk/platform/dotnet/repair_138_test.go; do
    mv "$fixture/$cover" "$work/cover"
    reject_fixture "a missing classification cover: $cover"
    mv "$work/cover" "$fixture/$cover"
done

for path in sdk/chainflow/typescript/package.json \
    contracts/bridge/solana/Cargo.toml \
    tools/benchmark/loadtest/contracts/jupiter/Cargo.toml \
    wasm-runtime/libwasmvm/Cargo.toml; do
    cp "$fixture/$path" "$work/original"
    case "$path" in
        */package.json) sed '/"build"[[:space:]]*:/d' "$work/original" > "$fixture/$path" ;;
        contracts/bridge/solana/Cargo.toml) sed '/^members = /d' "$work/original" > "$fixture/$path" ;;
        */jupiter/Cargo.toml) sed '/path = "..\/pax-cosmwasm"/d' "$work/original" > "$fixture/$path" ;;
        */libwasmvm/Cargo.toml) sed '/cosmwasm-crypto = { path = /d' "$work/original" > "$fixture/$path" ;;
    esac
    reject_fixture "an absent covering declaration in $path"
    mv "$work/original" "$fixture/$path"
done

cp "$work/inventory" "$work/original-inventory"
sed 's/|covered-by-package-build$/|unknown-class/' "$work/original-inventory" > "$work/inventory"
reject_fixture 'an unknown classification'
sed 's/^npm|sdk\/chainflow\/typescript\//rust|sdk\/chainflow\/typescript\//' "$work/original-inventory" > "$work/inventory"
reject_fixture 'a classification with the wrong manifest kind'
sed 's#|covered-by-test:sdk/platform/dotnet/repair_138_test.go$#|build_test_lint#' "$work/original-inventory" > "$work/inventory"
reject_fixture 'an independent Rust root without its own lock'
sed '/^npm|sdk\/chainflow\/typescript\/package.json|/d' "$work/original-inventory" > "$work/inventory"
reject_fixture 'an omitted source manifest'
cp "$work/original-inventory" "$work/inventory"
echo 'npm|sdk/absent/package.json|build_static' >> "$work/inventory"
reject_fixture 'an absent declared manifest'

echo "check-paxeer-manifests regression: ok"
