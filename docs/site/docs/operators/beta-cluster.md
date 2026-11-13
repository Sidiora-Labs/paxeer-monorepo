# Disposable beta cluster

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` and participates in execution and block proposal.
A receipt is final when its block is committed. A disposable cluster with the
same chain ID has its own genesis and validator identity.

The cluster entry point is `services/platform/hosted/tests/beta-cluster.sh`.
Its `up` and `render` commands delegate to `beta-native.sh` in the same
directory. The native helper prepares chain material, renders the node
manifest and checks a committed receipt. Hosted application services have
separate manifests and configuration.

## Native node deployment

`services/platform/hosted/node/deployment.yaml` declares StatefulSet
`layerx-node` with one replica and a 40Gi `ReadWriteOnce` data volume.
Its `paxd` container uses `ghcr.io/sidiora-labs/kernel:0.1.0` and runs:

```sh
python3 /opt/layerx/kernel_runtime.py --home /data/layerx/node
```

The implementation packaged into the image is `docker/kernel/kernel_runtime.py`.
The pod runs as UID/GID `4020`, drops capabilities and uses a read-only root
filesystem. Service `layerx-node` provides peer discovery on port `26656`.
Service `layerx-native-chain` exposes node JSON-RPC on `8545`, consensus RPC on
`26657` and health on `9460`. The manifest's NetworkPolicy controls which
workloads can reach those ports.

ConfigMap `layerx-node-config` declares `network-id: "125"`,
`chain-id: "hyperpax_125-1"`, `paxeer-chain-id: "125"`, `asset-id`,
`network-name` and `rpc-origin`. The container reads `network-id` through
`LAYERX_NODE_NETWORK_ID`; the native runtime fixes its EVM signing chain at 125.
Application namespace settings do not select a different signing chain.

## Native material

The `native-home` init container reads Secret `layerx-native-inputs`.
Its `manifest.json` has schema `layerx.beta.native-material.v1`, consensus
chain ID, EVM chain ID, treasury policy and SHA-256 digests of these files:

| Material | Installed path relative to `/data/layerx/node` |
| --- | --- |
| `genesis.json`, `config.toml`, `app.toml` | Corresponding files under `config/` |
| `priv_validator_key.json`, `node_key.json`, `receipt.key` | Corresponding files under `config/` |
| `priv_validator_state.json` | `data/priv_validator_state.json` |
| `treasury-policy.json`, `receipt-keys.json` | Home directory |

The init container checks every projected digest before installation. It
creates private directories with mode `0700` and files with mode `0600`,
syncs them, and retains the material manifest. On reopen, the manifest and
installed inputs must agree; validator signing state may advance. Partial
homes are refused. Preserve validator signing state with the matching keys
and genesis when restarting.

Fresh local preparation initializes a validator home, funds the configured
treasury in genesis, generates a receipt key and validates genesis. An
operator can instead set `LAYERX_BETA_NATIVE_HOME` and
`LAYERX_BETA_TREASURY_KEY_FILE` to import an existing disposable native home
and its treasury signer. The helper validates the chain and treasury bindings
before projecting material.

`LAYERX_BETA_RETAIN_MATERIAL=1` reopens the saved native material against the
same treasury policy and requires the saved readiness transaction. It does
not regenerate a validator or replace incomplete material. Material handling
is implemented in `services/platform/hosted/tests/beta_runtime.go`.

## Commands

The work directory is `build/beta-cluster`. With `LAYERX_BETA_KUBECONFIG`
unset, the helper selects a kind cluster named by `LAYERX_BETA_CLUSTER_NAME`
(default `layerx-beta`). A supplied kubeconfig selects an operator's cluster.
`beta-images.sh` owns the image inventory and Dockerfile mappings.

| Command | Behavior |
| --- | --- |
| `bash services/platform/hosted/tests/beta-cluster.sh images` | Build or pull the image inventory |
| `bash services/platform/hosted/tests/beta-cluster.sh render` | Prepare native material and render `build/beta-cluster/manifests/node.yaml` without applying workloads |
| `make platform-beta-cluster-up` | Start the native cluster; forwards `PLATFORM_BETA_CLUSTER_FLAGS` |
| `make platform-beta-cluster-down` | Tear down the selected disposable environment |

Native bring-up applies `node.yaml`, waits for the node pod, opens local
node and consensus port-forwards, and submits or reuses a zero-value treasury
transaction for receipt readiness. It exports `LAYERX_NODE_URL`,
`LAYERX_PAXEER_RPC_URL` and `LAYERX_CHAIN_ID=125` into
`build/beta-cluster/env` after readiness succeeds. The native path refuses
`--boundary-checks`.

`make platform-hosted-smoke` exercises the separately configured hosted
journeys. It needs their services and credentials; the native environment
export alone does not supply those application dependencies. See
[Hosted control](beta-control.md#journeys-and-probes) for its admission contract.

## Readiness and recovery

The pod probes `GET /readyz` on port `9460`. The runtime checks that the node
process is alive, consensus is caught up, the head has a canonical commit,
and the EVM block at that height matches the consensus block. It also checks
chain identity and a committed-head age of at most 60 seconds. A retained
checkpoint binds the genesis and prevents a lower or divergent committed
coordinate from being accepted after restart; each probe reads live state.

Cluster receipt readiness additionally checks `pax_sequencerReceipt` for the
saved transaction, authenticates its signed bytes under the saved governed
receipt keys and binds its committed height and hash. Its retained state
binds the chain, genesis, treasury policy and transaction. The implementation
is `services/platform/hosted/tests/beta_readiness.go`.

Retain the data volume, native input Secret, treasury signer and saved
material together. Reuse the original transaction identity after an uncertain
response. Native readiness and the control service's aggregate `/readyz`
are separate checks.

For public access, use the root endpoints in
[Public RPC endpoints](../reference/public-rpc.md), and check `eth_chainId`
returns `0x7d` before signing. Public endpoints are not inputs for disposable
resets or test funding. Tenant domains are namespaces within the same network.

## Teardown

`beta-cluster.sh down` stops port-forwards, deletes the recorded kind cluster
or the configured hosted namespaces, removes recorded images and deletes
`build/beta-cluster`. It also removes installed tools unless
`LAYERX_BETA_KEEP_TOOLS=1`. Volume deletion follows the cluster's storage reclaim
policy. Retained-material mode does not preserve local files across `down`.

[Hosted control](beta-control.md) · [Hosted Human](../human/hosted.md) · [Home](../index.md)
