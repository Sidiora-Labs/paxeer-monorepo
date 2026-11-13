# Hosted node

The hosted node runs `paxd`, the Paxeer BFT node with the kernel integrated
into the network. The kernel participates in block proposal; execution and
commit belong to one chain. A receipt is final once its block is committed.
An acknowledgement for an open block does not establish finality.

Paxeer mainnet-beta uses EVM chain ID **125** (`0x7d`) and consensus chain ID
`hyperpax_125-1`. Sub-chains are tenant domains in the database; creating a
domain does not require provisioning another consensus network.

The container lifecycle is implemented in `docker/kernel/kernel_runtime.py`.
[Hosted core](hosted-core.md), [Hosted authority](hosted-authority.md),
[Hosted agent boundary](agent-boundary.md) and [Hosted Human](../human/hosted.md)
are separate services. Check their availability separately from chain readiness.

## Deployment and ingress

`services/platform/hosted/node/deployment.yaml` declares StatefulSet
`layerx-node` in namespace `layerx-testnet`, with one replica. Its single
application container, `paxd`, selects `ghcr.io/sidiora-labs/kernel:0.1.0`
and runs:

```sh
python3 /opt/layerx/kernel_runtime.py --home /data/layerx/node
```

The pod uses UID/GID `4020:4020`, disables service-account token mounting
and has `shareProcessNamespace: false`. The manifest provides these Services:

| Service | Ports | Purpose |
| --- | --- | --- |
| `layerx-node` | `26656` | Headless BFT peer service |
| `layerx-native-chain` | `8545` | Root-path EVM JSON-RPC |
| `layerx-native-chain` | `26657` | Consensus RPC service port |
| `layerx-native-chain` | `9460` | Committed-chain readiness |

EVM HTTP binds all interfaces on port `8545`; consensus RPC binds loopback
on port `26657`. The NetworkPolicy admits TCP `8545` from gateway, testnet
control and registry pods and the developer namespace, and TCP `26656` from
node pods. Egress admits DNS and node peer traffic. Service declarations and
listener bindings both govern connectivity. There is no Ingress in this manifest.

Use an operator-provided HTTPS node root for public JSON-RPC. Set `RPC_URL`
to that origin before running:

```sh
curl --fail-with-body "${RPC_URL:?}" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected `result` is `"0x7d"`. Compare `eth_blockNumber` across nodes
when selecting one near the head. The hosted gateway's contract is in
`services/platform/hosted/gateway/openrpc.json`; node reads are registered
independently. See [Gateway RPC](gateway-rpc.md).

## Provisioned native material

The `native-home` init container reads Secret `layerx-native-inputs` at
`/run/native-inputs`. Its `manifest.json` uses schema
`layerx.beta.native-material.v1`, consensus chain ID `hyperpax_125-1`, EVM
chain ID `125` and a SHA-256 digest for each of these nine files:

| Input | Destination relative to the node home |
| --- | --- |
| `genesis.json` | `config/genesis.json` |
| `config.toml` | `config/config.toml` |
| `app.toml` | `config/app.toml` |
| `priv_validator_key.json` | `config/priv_validator_key.json` |
| `node_key.json` | `config/node_key.json` |
| `receipt.key` | `config/receipt.key` |
| `priv_validator_state.json` | `data/priv_validator_state.json` |
| `treasury-policy.json` | `treasury-policy.json` |
| `receipt-keys.json` | `receipt-keys.json` |

On initial provisioning, the home must be absent. The initializer creates
owner-only home, `config` and `data` directories with mode `0700`, writes
files with mode `0600`, and retains the manifest as `.native-material.json`.
Writes and directories are synchronized. On restart, it checks the supplied
manifest against the retained manifest and material hashes. Validator signing
state may evolve and is not compared to its initial digest.

The runtime validates canonical owner-controlled directories and private
material, validator-mode configuration and paths within the home. Genesis
must name the configured chain and include validators, `bank` and `lxphost`
state. The receipt seed is nonzero lowercase hexadecimal encoding 32 bytes.
`paxd validate-genesis --home <home>` runs before the child starts. Receipt
key validation in `node/sequencer/receiptkey.go` checks governed pins at the
next block height.

ConfigMap `layerx-node-config` declares `network-id: "125"`, `network-name`,
`chain-id`, `paxeer-chain-id: "125"`, `asset-id` and `rpc-origin`. The container
receives `LAYERX_NODE_NETWORK_ID` from `network-id`; EVM signing chain identity
is fixed at `125` by the runtime.

## Startup and committed readiness

The runtime holds an exclusive `.kernel-owner.lock` and starts
`/usr/local/bin/paxd start --home <home> --chain-id hyperpax_125-1`.
It enables EVM HTTP on port `8545` and WebSockets on `8546`, disables the
test API and unsafe keyring RPC, and enables the integrated proposal and
receipt path using:

```text
--sequencer.enable true
--sequencer.wal-dir <home>/data/sequencer-wal
--sequencer.receipt-key-file <home>/config/receipt.key
```

The readiness server listens on port `9460`. Each `GET /readyz` invokes
`committed_health` from `services/platform/hosted/node/native_runtime.py`
and checks:

1. Consensus status reports the configured chain, no catch-up, a positive
   committed height and block hash.
2. The canonical consensus commit matches the height, hash and chain identity.
3. EVM chain identity is `125`, and the EVM block at that height matches the
   committed hash.
4. A fresh status still names that height and hash, its last committed height
   covers the observation, and its block time is between zero and 60 seconds old.
5. The same child process remains live throughout the observation.

The runtime retains the genesis-bound height and hash in
`.committed-readiness.json`. A lower height or a different hash at the same
height is refused. This file is a recovery floor; each probe reads live chain
state. Successful readiness returns HTTP 200 with `ready: true`, `chain_id: 125`,
`height` and `block_hash`. Refusal returns HTTP 503 with `ready: false` and
`chain_id: 125`.

Both startup and readiness probes use HTTP `/readyz` every five seconds with
a ten-second timeout; the startup failure threshold is 60. The runtime stops
the child on SIGTERM or SIGINT. An unexpected child exit terminates the
container with a failure status, including an unexpected clean child exit.

## Storage and recovery

The `data` PVC requests `40Gi`, uses `ReadWriteOnce` and mounts at `/data`.
Retain the entire `/data/layerx/node` home, including validator signing state,
receipt seed, `data/sequencer-wal`, native material manifest and readiness
checkpoint. The container root filesystem is read-only, with a separate
writable `/tmp` volume.

`node/app.go` reloads committed application state and refreshes the kernel
ABI. `node/sequencer/node_start.go` checks the receipt key against committed
pins, compares the WAL's committed height, sequence and application hash
with the application store, restores receipts and replays the WAL. Receipt
bytes and sequence identity must agree. Preserve consistent application and
WAL state during recovery.

Native state-sync recovery uses the snapshot/chunk handlers in
`sdk/baseapp/abci.go`, with extensions registered by `node/app.go`. Retain
validator signing state to prevent double signing, and check live committed
readiness after recovery.

## Local lifecycle tooling

`services/platform/hosted/node/native_runtime.py` also provides local
`bootstrap`, `supervise` and `health` commands, used by the shell wrappers
in that directory. Its generation and reset tooling maintains local consumer
bindings and a durable reset journal. The Kubernetes container command above
uses `kernel_runtime.py`; provision its native home and Secret material as
described here.

[Home](../index.md)
