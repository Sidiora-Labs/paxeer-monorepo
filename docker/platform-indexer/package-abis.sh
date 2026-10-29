#!/bin/sh
set -eu

if [ "$#" -ne 2 ]; then
	echo "Usage: $0 <precompiles dir> <out dir>" >&2
	exit 1
fi

precompiles=$1
out=$2

for dir in "$precompiles"/*/; do
	[ -f "$dir/abi.json" ] || continue
	name=$(basename "$dir")
	mkdir -p "$out/$name"
	cp "$dir/abi.json" "$out/$name/abi.json"
done

for relative_path in \
	perps/abi/abi.json \
	pricefeed/abi/abi.json \
	spot/abi/pecor.json \
	spot/abi/orders.json \
	spot/abi/stoporders.json \
	spot/abi/vault.json; do
	if [ ! -f "$precompiles/$relative_path" ]; then
		echo "Missing required ABI: $precompiles/$relative_path" >&2
		exit 1
	fi
	mkdir -p "$out/$(dirname "$relative_path")"
	cp "$precompiles/$relative_path" "$out/$relative_path"
done
