#!/bin/sh
set -eu

index=${LAYERX_BRIDGE_ATTESTOR_INDEX:?attestor index is required}
case "$index" in
'' | *[!0-9]* | 0*)
	echo "bridge-relayer-entrypoint: attestor index must be a positive integer" >&2
	exit 1
	;;
esac
export LAYERX_ROLE="bridge-relayer-$index"
exec /usr/local/bin/layerx-env-files /usr/local/bin/bridge-relayer-entrypoint
