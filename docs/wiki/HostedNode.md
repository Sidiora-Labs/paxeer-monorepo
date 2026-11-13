# Hosted node

The hosted node runs `paxd` with the LayerX kernel embedded in the network.
Paxeer mainnet-beta is one BFT chain, with EVM chain ID **125** (`0x7d`) and
consensus chain ID `hyperpax_125-1`. The kernel participates in block proposal;
a receipt is final when its block is committed.

The [node manifest](../../services/platform/hosted/node/deployment.yaml) launches
[`kernel_runtime.py`](../../docker/kernel/kernel_runtime.py). The runtime owns
the node process and checks live committed-chain readiness. Its native RPC and
process helpers are in
[`native_runtime.py`](../../services/platform/hosted/node/native_runtime.py).
The [kernel Dockerfile](../../docker/kernel/Dockerfile) packages `paxd` at
`/usr/local/bin/paxd` and the runtime at `/opt/layerx/kernel_runtime.py`.

## Chain access

Send Ethereum JSON-RPC requests to the node's root path. Select a public node
from [Public JSON-RPC](PublicRpc.md); use its root URL as the shell variable
`PAXEER_RPC_URL` in this example:

```sh
curl --fail-with-body "$PAXEER_RPC_URL" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected result is `"0x7d"`. Compare `eth_blockNumber` across nodes when
choosing a current head. The hosted gateway serves its application contract
at `POST /` and its schema at `GET /openrpc.json`; see
[Hosted gateway](HostedGateway.md). Sub-chains are tenant domains in the
database and share the network.

## Kubernetes layout

The manifest declares StatefulSet `layerx-node` in namespace `layerx-testnet`,
with one replica and labels `app: layerx-node` and `layerx-plane: native-chain`.
It runs as UID/GID/fsGroup `4020`, disables service-account token mounting,
uses a read-only root filesystem and sets `shareProcessNamespace: false`.
The termination grace period is 60 seconds.

Init container `native-home` prepares the persistent home. The sole application
container, `paxd`, runs:

```text
python3 /opt/layerx/kernel_runtime.py --home /data/layerx/node
```

Both containers use image `ghcr.io/sidiora-labs/kernel:0.1.0` in the template.
The data PVC requests `40Gi`, is `ReadWriteOnce`, and mounts at `/data`.
A separate `emptyDir` mounts at `/tmp`.

| Service | Port | Target |
| --- | --- | --- |
| `layerx-node` | `26656` | `p2p`, headless |
| `layerx-native-chain` | `8545` | `rpc` |
| `layerx-native-chain` | `26657` | `consensus` |
| `layerx-native-chain` | `9460` | `health` |

The native-chain Service is ClusterIP. NetworkPolicy admits RPC TCP 8545 from
gateway, control and registry pods and the developer namespace. Node pods may
exchange P2P traffic on TCP 26656. Egress permits node P2P and cluster DNS.

## Provisioned material

Secret `layerx-native-inputs` supplies `manifest.json` and nine files:

- `genesis.json`, `config.toml` and `app.toml`;
- `priv_validator_key.json`, `node_key.json` and `receipt.key`;
- `priv_validator_state.json`, `treasury-policy.json` and `receipt-keys.json`.

The manifest schema is `layerx.beta.native-material.v1`. It binds the consensus
chain ID, EVM chain ID and SHA-256 digest of each file. The init container checks
those bindings before copying files into a new home, using mode `0700` for
its directories and `0600` for files. It syncs files and directories and retains
the manifest as `.native-material.json`.

On restart, the retained manifest must match the supplied manifest. Every file
must remain regular and nonsymlink; digests are checked again except for the
mutable validator signing-state file. An existing partial home is refused.
The running node preserves validator signing state rather than recreating it.

The runtime requires a canonical owner-controlled home and private `config/`
and `data/` directories. Genesis must contain the configured chain identity,
validators, bank state and `app_state.lxphost`. Private validator, node and
signing-state JSON files must have mode `0600`. The receipt seed is nonzero
lowercase 64-character hex. `paxd validate-genesis --home HOME` runs before
node startup.

ConfigMap `layerx-node-config` sets `network-id: "125"`,
`chain-id: "hyperpax_125-1"` and `paxeer-chain-id: "125"`.
The pod exports `LAYERX_NODE_NETWORK_ID` from `network-id` and sets
`LAYERX_KERNEL_PROFILE=native`. The runtime fixes the native chain identities;
`LAYERX_NODE_EVM_CHAIN_ID` and `LAYERX_NODE_PAXEER_CHAIN_ID`, if supplied,
must both equal `125`.

## Native process

`node_command` starts `paxd` with the provisioned home and consensus chain ID.
Consensus RPC binds to loopback port `26657`; HTTP JSON-RPC binds to all
interfaces on `8545`. WebSocket RPC is enabled on `8546`. The test API and unsafe
keyring RPC are disabled. `--sequencer.enable true` enables execution inside
`paxd`; the WAL lives at `HOME/data/sequencer-wal`, and the receipt key at
`HOME/config/receipt.key`.

The runtime takes an exclusive `.kernel-owner.lock`, starts one child and
serves readiness on port `9460`. SIGTERM and SIGINT stop the child and the
health server. An unsolicited node exit causes the runtime to exit with a
nonzero status so the container owner can restart it.

## Readiness and restart

`GET /readyz` returns HTTP 200 with `ready: true`, `chain_id: 125`, `height`
and `block_hash` only after a live check of the node process and chain:

1. Consensus status identifies the configured chain and a positive height,
   with `catching_up: false`.
2. The canonical signed commit and EVM block agree at that height, and
   `eth_chainId` matches the configured EVM chain ID.
3. A second status read confirms stable height and hash, with
   `last_committed_block_height` covering the observed height.
4. The latest block timestamp is no more than 60 seconds old and is not
   in the future.

A private `.committed-readiness.json` binds the observed height and hash to
the genesis digest. It provides a recovery floor: a lower height or a changed
hash at the same height is refused. Each request still reads live chain state.
Unavailability returns HTTP 503 with `ready: false` and `chain_id: 125`.
Other GET paths return 404.

The manifest probes readiness every five seconds with a ten-second timeout.
The startup probe uses the same route and allows 60 failed probes. Preserve
the complete home, validator signing state, WAL and retained manifests across
restarts. The runtime opens existing state and does not initialize a new chain.

The beta cluster helper's native path is
[`beta-native.sh`](../../services/platform/hosted/tests/beta-native.sh).
It applies the native node manifest, forwards native RPC and consensus ports,
and verifies a committed native transaction receipt. Retained mode uses
`LAYERX_BETA_RETAIN_MATERIAL=1` to reopen the saved native material and the same
readiness transaction. See [Hosted Human material](HostedHuman.md) for the
separate Human policy and evidence assembler.

[Home](Home.md)
