#!/bin/sh
set -eu

cd "$(dirname "$0")/../.."
script=scripts/workspace/check-platform-dependencies.sh
inventory=tools/workspace/platform-dependency-roots.txt
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM

sh "$script" "$inventory"

grep -v '^rust|services/platform/hosted/core/Cargo.toml|' "$inventory" > "$work/missing-root.txt"
if sh "$script" "$work/missing-root.txt" >/dev/null 2>&1; then
    echo "check-platform-dependencies accepted an inventory that omits a manifest" >&2
    exit 1
fi

cp "$inventory" "$work/extra-root.txt"
echo 'rust|services/platform/hosted/absent/Cargo.toml|covered-by-platform-cargo' >> "$work/extra-root.txt"
if sh "$script" "$work/extra-root.txt" >/dev/null 2>&1; then
    echo "check-platform-dependencies accepted an inventory naming an absent manifest" >&2
    exit 1
fi

sed 's#^npm|services/platform/middleware/agent/package.json|covered-by-root-npm$#npm|services/platform/middleware/agent/package.json|locked-npm-independent#' "$inventory" > "$work/unlocked-npm.txt"
if sh "$script" "$work/unlocked-npm.txt" >/dev/null 2>&1; then
    echo "check-platform-dependencies accepted a locked npm root without package-lock.json" >&2
    exit 1
fi


repository=$(pwd)
fixture=$work/checkout
mkdir -p "$fixture/services/platform"
cat > "$work/covered-inventory" <<'EOF'
python|services/platform/hosted/paxeer/requirements.txt|covered-by-python-source:services/platform/hosted/paxeer/evm.py
rust|services/platform/perf-db/Cargo.toml|locked-independent
rust|services/platform/perf-db/src/storage/repair_139/Cargo.toml|covered-by-test:services/platform/perf-db/src/storage/repair_139_test.go
EOF
for path in services/platform/hosted/paxeer/requirements.txt \
    services/platform/hosted/paxeer/evm.py \
    services/platform/perf-db/Cargo.toml services/platform/perf-db/Cargo.lock \
    services/platform/perf-db/src/storage/repair_139/Cargo.toml \
    services/platform/perf-db/src/storage/repair_139_test.go; do
    mkdir -p "$fixture/$(dirname "$path")"
    cp "$path" "$fixture/$path"
done

check_fixture() {
    (cd "$fixture" && sh "$repository/$script" "$work/covered-inventory")
}

reject_fixture() {
    if check_fixture > "$work/output" 2>&1; then
        echo "check-platform-dependencies accepted $1" >&2
        exit 1
    fi
}

check_fixture
for cover in services/platform/hosted/paxeer/evm.py \
    services/platform/perf-db/Cargo.lock \
    services/platform/perf-db/src/storage/repair_139_test.go; do
    mv "$fixture/$cover" "$work/cover"
    reject_fixture "a missing classification cover: $cover"
    mv "$work/cover" "$fixture/$cover"
done
path=services/platform/hosted/paxeer/evm.py
cp "$fixture/$path" "$work/original"
sed '/^from cryptography\./d' "$work/original" > "$fixture/$path"
reject_fixture 'Python dependency coverage without an import'
mv "$work/original" "$fixture/$path"
cp "$work/covered-inventory" "$work/original-inventory"
sed 's/^python|/rust|/' "$work/original-inventory" > "$work/covered-inventory"
reject_fixture 'Python source coverage for the wrong manifest kind'
sed 's#|covered-by-test:services/platform/perf-db/src/storage/repair_139_test.go$#|locked-independent#' "$work/original-inventory" > "$work/covered-inventory"
reject_fixture 'a test producer classified as independently locked'

echo "check-platform-dependencies regression: ok"
