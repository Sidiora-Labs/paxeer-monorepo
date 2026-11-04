#!/bin/bash
# Start the merged kernel in its single paxd process. Provision the native
# chain home separately; never initialise a new validator network on restart.
set -euo pipefail
umask 077

case "${LAYERX_KERNEL_PROFILE:-full}" in
    full|native) ;;
    *) printf 'kernel-init: LAYERX_KERNEL_PROFILE must be full or native\n' >&2; exit 1 ;;
esac
[ "${LAYERX_NODE_NETWORK_ID:-125}" = 125 ] || {
    printf 'kernel-init: the merged kernel requires EVM chain 125\n' >&2
    exit 1
}

node_home=${LAYERX_KERNEL_NODE_HOME:-/data/layerx/node}
case "$node_home" in
    /data/*) ;;
    *) printf 'kernel-init: node home must be on /data\n' >&2; exit 1 ;;
esac

# The runtime validates all provisioned paths before starting the node. It
# refuses an old daemon volume instead of deleting or migrating its state.
exec setpriv --reuid=4020 --regid=4020 --clear-groups --no-new-privs \
    python3 /opt/layerx/kernel_runtime.py --home "$node_home"
