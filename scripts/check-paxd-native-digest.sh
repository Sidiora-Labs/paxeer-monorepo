#!/bin/sh
# Asserts build/paxd was linked against the native archives currently on disk:
# the digest chain.mk stamps at link time must equal their present sha256.
set -eu
cd "$(dirname "$0")/.."
missing=0
for archive in build/liblayerx.a programs/target/release/liblayerx_programs_sandbox.a; do
	if [ ! -f "$archive" ]; then
		echo "paxd-native-digest-check: missing native archive: $archive" >&2
		missing=1
	fi
done
if [ "$missing" -ne 0 ]; then
	exit 1
fi
want=$(cat build/liblayerx.a programs/target/release/liblayerx_programs_sandbox.a | sha256sum | cut -d' ' -f1)
got=$(./build/paxd version --long --output json | jq -er .native_archive_digest)
if [ "$got" != "$want" ]; then
	echo "paxd-native-digest-check: build/paxd carries native digest $got, archives on disk are $want" >&2
	exit 1
fi
echo "paxd-native-digest-check: $got"
