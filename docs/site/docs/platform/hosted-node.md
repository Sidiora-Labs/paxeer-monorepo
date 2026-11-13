# Hosted node

The hosted node runs `paxd`, the Paxeer BFT node with the kernel integrated
into the network. The kernel participates in block proposal; execution and
commit belong to one chain. A receipt is final once its block is committed.
An acknowledgement for an open block does not establish finality.

Paxeer mainnet-beta uses EVM chain ID **125** (`0x7d`) and consensus chain ID
`hyperpax_125-1`. Sub-chains are tenant domains in the database; creating a
domain does not require provisioning another consensus network.

The pre-merge `layerxd` deployment, authority replica, LNI activity socket,
checkpoint settlement and guarantor finality model are retired. Their old
bootstrap files and reset instructions do not operate the merged node.

This page describes the native lifecycle implemented in
`services/platform/hosted/node/native_runtime.py`, its storage and the
hosted Kubernetes manifest. The associated application services are
[Hosted core](hosted-core.md), [Hosted authority](hosted-authority.md),
[Hosted agent boundary](agent-boundary.md) and [Hosted Human](../human/hosted.md).
Their availability must be checked separately from chain readiness.

## Deployment and ingress

`services/platform/hosted/node/deployment.yaml` declares StatefulSet
`layerx-node` in namespace `layerx-testnet`, with one replica. Its `paxd`
container selects `ghcr.io/sidiora-labs/layerx-kernel:0.1.0` and runs
`/opt/layerx/supervisor.sh`. The script delegates to
`native_runtime.py supervise`, which starts `paxd` and the local generation
and reset services. The pod uses UID/GID `4020:4020`, disables service-account
token mounting and has `shareProcessNamespace: false`.

The manifest describes these node-facing listeners and Services:

| Surface | Address or Service | Purpose |
| --- | --- | --- |
| Native EVM JSON-RPC | `layerx-native-chain:8545` | Root-path JSON-RPC for the hosted gateway |
| Consensus JSON-RPC | `layerx-native-chain:26657` | Consensus status and signed commits |
| Peer transport | Container port `26656` | BFT peer traffic |
| Core | `layerx-pending-core:9443` | Hosted application boundary |
| Core administration | `layerx-pending-core-admin:9444` | Protected funding and reset operations |
| Receipt authority | `layerx-receipt-authority:9443` to container port `9445` | Hosted receipt service |
| Agent boundary | `layerx-agent-boundary:9443` to container port `9446` | Hosted agent service |
| Agentd | `layerx-agentd:9443` to container port `9454` | Hosted agentd TLS boundary |

The headless `layerx-node` Service targets the core TLS port `9443`; it is
not the EVM port. There is no Ingress object in this manifest. The ingress
NetworkPolicy admits the gateway to `8545` and `26657`, native validator
pods to `26656`, and selected application clients to the hosted TLS ports.
These private ports are not the public API. Inspect the manifest's complete
NetworkPolicies when changing cluster connectivity.

Public JSON-RPC uses the root of the node origins
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, and
`https://api-mainnet-beta.paxeer.network`. For example:

```sh
curl --fail-with-body https://api1.mainnet-beta.paxeer.network/ \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected `result` is `"0x7d"`. Compare `eth_blockNumber` across nodes
when a node trails the head. The hosted gateway's `px_` contract is declared in
`services/platform/hosted/gateway/openrpc.json`; public nodes also expose their
registered `px_` reads.
See [Gateway RPC](gateway-rpc.md) for that service's authentication and API.
The host `node.hyperpaxeer.com` is a library mirror, not an RPC endpoint.

### Deployment prerequisites

The node image must package `paxd`, `native_runtime.py`, its generation/reset
helpers and signer dependencies. Bootstrap requires authoritative native genesis
and keys. The gateway connects to native chain RPC, and hosted consumers use
committed native reads. Check chain and application-service readiness separately.

## Bootstrap inputs

`bootstrap.sh` delegates to `native_runtime.py bootstrap`. Bootstrap requires
an empty, owner-controlled data directory. The supervisor performs this
preparation on first startup and reuses a prepared generation on restart;
do not run bootstrap over retained chain state.

The native input directory contains these owner-readable regular files:

| Input | Requirement |
| --- | --- |
| `genesis.json` | Authoritative consensus genesis with matching chain ID, validators and native `bank` and `lxphost` application state |
| `config.toml` | Consensus configuration, including the intended peer configuration |
| `app.toml` | Native application configuration |
| `priv_validator_key.json` | Validator consensus key, mode `0600` |
| `rpc-ca.der` | DER trust anchor for the hosted gateway TLS origin |

`--receipt-key-file` supplies a separate receipt signing seed: 64 lowercase
hex characters encoding 32 bytes, with owner-only permissions. Its public
key and SHA-256 key ID must be pinned from height 1 in
`genesis.json` under `app_state.sequencer.receipt_keys`. It must differ
from the consensus signing key. Runtime key validation in
`node/sequencer/receiptkey.go` also checks the committed pin range at the
next block height when restarting.

Input reads reject symlinks, multiple hard links, wrong ownership and
group/world-writable files. The manifest's init containers copy projected
ConfigMap and Secret inputs into private memory-backed files before these
checks; do not point the native reader at Kubernetes projection symlinks.

The supervisor's main configuration arguments are:

| Argument | Meaning |
| --- | --- |
| `--paxd` | Native executable; default `/usr/local/bin/paxd` |
| `--data-dir` | Persistent node home, deployed as `/var/lib/layerx/node` |
| `--state-dir` | Persistent reset journal and staging root, deployed as `/var/lib/layerx/supervisor-state` |
| `--run-dir` | Runtime sockets and process identity, deployed as `/run/layerx/node` |
| `--native-inputs` | Protected input directory, deployed as `/run/native-private/owner` |
| `--chain-id` | Consensus chain ID; `hyperpax_125-1` for mainnet-beta |
| `--evm-chain-id` | EVM chain ID; `125` for mainnet-beta |
| `--network-id` | Application network identity, bound into the reset journal; the hosted manifest currently supplies `402` |
| `--asset` | Authoritative, nonzero 32-byte asset ID in lowercase hex |
| `--gateway-endpoint` | HTTPS gateway root origin with an explicit port, backed by `rpc-ca.der` |
| `--chain-rpc-url` | Local EVM root origin; default `http://127.0.0.1:8545` |
| `--consensus-rpc-url` | Local consensus root origin; default `http://127.0.0.1:26657` |
| `--rpc-bind-address` | `127.0.0.1` by default; manifest selects `0.0.0.0` for private Service access |
| `--receipt-key-file` | Protected receipt seed file |
| `--treasury-signer-socket` | Absolute path to the treasury signing service |
| `--funding-sender-key-file` | Absolute path to the core's protected native funding sender key |
| `--consumer-uid`, `--consumer-gid` | Local generation consumer identity; defaults `4021:4020` |
| `--ready-seconds` | Committed readiness deadline; default `120`, accepted range `1`–`300` seconds |

The application network identity is not the EVM chain ID. Keep it consistent
with the authoritative genesis and consumers; do not substitute `402` for
`125` in wallet or JSON-RPC chain configuration. ConfigMap
`layerx-node-config` supplies `network-id`, `chain-id`, `paxeer-chain-id`,
`asset-id` and `gateway-endpoint` to the deployed supervisor arguments.

Bootstrap runs `paxd init` with `--mode validator`, installs the supplied
native configuration and validator key, then runs `paxd validate-genesis`.
It publishes `node.env`, `core.env`, `rpc-ca.der`, the immutable
`.generation-proof/` artifact copies and `.native-prepared.json` hashes
using synchronized file writes. The published environments use mode `0600`.
They are generated artifacts, not shell scripts to source or edit.

`node.env` publishes `LAYERX_NODE_NETWORK_ID`, `LAYERX_NODE_CHAIN_ID`,
`LAYERX_NODE_EVM_CHAIN_ID`, `LAYERX_NODE_RPC_URL` and
`LAYERX_NODE_CONSENSUS_RPC_URL`, plus the receipt key identity in
`LAYERX_NODE_SEQUENCER_ID` and `LAYERX_NODE_SEQUENCER_PUBLIC_KEY`.
The retained `SEQUENCER` identifiers name the native receipt role.
`core.env` binds the gateway origin and trust, local chain reads, asset,
treasury socket and funding key path for the core consumer.

## Startup and committed readiness

The supervisor launches `paxd start --home <data-dir>` with native consensus
RPC and EVM HTTP settings. It enables the integrated receipt/proposal path
using the existing `--sequencer.enable true`,
`--sequencer.wal-dir <data-dir>/sequencer-wal` and
`--sequencer.receipt-key-file <protected-key>` flags. These are node
configuration flags, not instructions to provision another process.

The native health entrypoint used by both deployment probes is:

```sh
python3 /opt/layerx/native_runtime.py health \
  --data-dir /var/lib/layerx/node \
  --run-dir /run/layerx/node
```

`committed_health` in `native_runtime.py` requires all of the following:

1. Consensus `status` reports the configured chain, `catching_up: false`,
   a positive committed height and a block hash.
2. Consensus `commit` at that height is canonical, has signatures and
   matches the status height/hash and chain ID.
3. `eth_chainId` matches the configured EVM ID, and
   `eth_getBlockByNumber` at that height returns the same committed hash.

Health also checks the retained process PID/start time, data-directory
identity and generation binding. An open port, a pending receipt or an
application TLS response alone does not satisfy readiness. A successful
probe prints the committed height and block hash; a refusal exits `1` with
`native node refused: ...`. Startup fails if the child exits or does not
reach committed readiness within the configured deadline.

## Persistent storage, keys and restart

The manifest mounts the `data` PVC at `/var/lib/layerx` and requests `50Gi`.
The node home and supervisor state share that persistent filesystem. Data
and state roots require owner ownership and mode `0700`; they must be
separate, non-nested directories on the same device for reset activation.
The runtime root permits mode `0700` or `0750` and is memory-backed in the
manifest. It contains `native-process.json`, `bindings.json`,
`generation.sock`, `supervisor.sock` and the treasury signer socket.

The consensus and receipt keys are copied from Secret
`layerx-node-native-keys` into `/run/native-private/owner`. Native
configuration comes from ConfigMap `layerx-node-native-genesis`.
The treasury seed comes from `layerx-node-keys/treasury.key`, is copied to
`/run/treasury-private/owner/treasury.key` and is mounted only into the
treasury signer after initialization. Core signs through
`/run/layerx/node/treasury-signer.sock`; it does not receive that seed.
The core's native funding sender is a different key, projected from
`layerx-node-native-funding/sender.key` into a private core mount.

For an ordinary restart, retain the node home, its validator signing state,
`sequencer-wal`, supervisor journal and the same protected inputs. The
supervisor holds an exclusive persistent ownership lock and rejects a
prepared generation whose published bindings have changed. Input or key
rotation requires coordinated provisioning, not an edit to `node.env`.

`node/app.go` reloads the latest committed application version and refreshes
the kernel ABI from that state. `node/sequencer/node_start.go` reloads the
receipt key against committed pins, compares the WAL's committed height,
sequence and application hash with the application store, restores committed
receipts and replays the WAL. Replay checks receipt bytes and sequence
identity. A mismatched committed tip is refused; do not erase the WAL to
bypass that refusal.

Native snapshot/state-sync recovery uses the application's snapshot manager
and ABCI snapshot/chunk handlers in `sdk/baseapp/abci.go`, with extensions
registered by `node/app.go`. Restore a consistent native chain state under
the network's validator recovery procedure, retaining the signing state
needed to prevent double signing. A restored database alone does not prove
that the local receipt WAL matches it. Run the same committed readiness
checks after recovery. The retired standalone snapshot and identity-sidecar
migration procedure does not restore a native `paxd` home.

## Reset and generation handoff

A reset bootstraps a replacement native generation; it is not routine
restart or snapshot catch-up. The supervisor binds reset work to the native
input hashes and all bootstrap options. `reset_state.py` persists its
journal and original response so retrying the same reset identity can
recover the recorded result.

The staged lifecycle is `admitted` → `stopping` → `bootstrapping` →
`prepared` → `activating` → `completed`. Replacement data is built under
`<state-dir>/generations/<reset-id>/data`, sealed with its artifact hashes,
then activated by the filesystem machinery in `data_directory.py`.
Completion requires the selected `.reset-generation.json` marker,
`.activation-durable.json` after directory synchronization, matching
process identity and committed native health. A directory exchange or
socket appearing alone cannot complete a reset. Interrupted work is
reconciled from the retained journal and generation markers on restart.

The local `supervisor.sock` accepts newline-delimited reset/status
requests from authorized peers. Version-1 structured requests carry
`operation`, a 32-character lowercase hex `reset_id`, the application
`network` and `request_digest`. The digest is SHA-256 of canonical compact,
key-sorted JSON containing `version: 1`, `operation: "reset"`, `reset_id`
and `network`; a status query retains the reset's digest. Preserve this
identity when retrying. Refusals include `bindings_mismatch`,
`digest_mismatch`, `reset_timeout` and `reset_ambiguous`; they are failures
to investigate, not successful resets. Never delete the journal to clear
an ambiguous operation.

`generation_transport.py` serves authorized local consumers on
`generation.sock`. Its version-2 `core` request transfers read-only artifact
file descriptors with the generation number and public node facts, rather
than granting consumers access to private node state. Consumers must reject
a different generation or changed artifact binding. This socket is a local
configuration handoff, not transaction ingress or a public endpoint.

[Home](../index.md)
