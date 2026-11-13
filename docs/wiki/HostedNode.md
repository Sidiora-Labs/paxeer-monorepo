# Hosted node

The hosted node runs `paxd` with the LayerX kernel embedded in the network.
Paxeer mainnet-beta is one BFT chain, with EVM chain ID **125** (`0x7d`) and
consensus chain ID `hyperpax_125-1`. The kernel participates in block proposal;
a signed execution receipt becomes final when its block is committed.
The pre-merge `layerxd` process, separate authority replica, LNI activity ingress
and guarantor finality are retired.

This page describes the native node lifecycle implemented by
[`native_runtime.py`](../../services/platform/hosted/node/native_runtime.py)
and the hosted Kubernetes layout in
[`deployment.yaml`](../../services/platform/hosted/node/deployment.yaml).
Hosted APIs remain application services around the chain. See
[Hosted core](HostedCore.md), [Hosted authority](HostedAuthority.md),
[Hosted agent boundary](HostedAgentBoundary.md), [Hosted Human](HostedHuman.md)
and [Hosted gateway](HostedGateway.md) for those services.

## Public access

Send Ethereum JSON-RPC requests to the **root path** of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, or
`https://api-mainnet-beta.paxeer.network`.

```sh
curl --fail-with-body https://api1.mainnet-beta.paxeer.network \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

The expected chain ID result is `"0x7d"`. Compare `eth_blockNumber` across
nodes when choosing a current head. The hosted gateway's `px_` API has its own
contract in
[`openrpc.json`](../../services/platform/hosted/gateway/openrpc.json);
`services/platform/hosted/gateway/src/main.rs` dispatches JSON-RPC on `POST /`.
Do not assume every gateway method is available on each public full node.
`node.hyperpaxeer.com` is only the hpx library mirror, not a chain endpoint.

Sub-chains are tenant domains in the database. Creating a tenant does not
require provisioning another chain or another node stack.

## Cluster shape

The manifest declares StatefulSet `layerx-node` in namespace `layerx-testnet`,
with one replica, labels `app: layerx-node` and
`layerx-plane: trusted-boundary`, and headless Service `layerx-node`.
The pod defaults to UID/GID `4020:4020`, disables service-account token mounting
and sets `shareProcessNamespace: false`.

The `paxd` container invokes `/opt/layerx/supervisor.sh`. That wrapper delegates
to `native_runtime.py supervise`, which starts `paxd` and the local generation
and supervisor socket servers. The process starts with these native flags:

```text
paxd start --home DATA_DIR
  --rpc.laddr tcp://BIND_ADDRESS:26657
  --evm.http_enabled true --evm.http_address BIND_ADDRESS --evm.http_port 8545
  --sequencer.enable true
  --sequencer.wal-dir DATA_DIR/sequencer-wal
  --sequencer.receipt-key-file RECEIPT_KEY_FILE
```

Here `--sequencer.*` configures the executor inside `paxd`. The hosted manifest
selects bind address `0.0.0.0`; the supervisor defaults to `127.0.0.1` when used
outside that manifest. Its health requests use loopback in either case.

The pod also declares the treasury signer, mirror signer and publisher,
core and receipt-authority boundaries, agent boundary and agentd TLS boundary,
and Human services and providers. These support signing, APIs and application
workflows; chain commit supplies receipt finality.

| Service | Port | Target |
| --- | --- | --- |
| `layerx-node` | `9443` | `core-tls`, headless |
| `layerx-native-chain` | `8545`, `26657` | `chain-rpc`, `consensus-rpc` |
| `layerx-pending-core` | `9443` | `core-tls` |
| `layerx-pending-core-admin` | `9444` | `core-admin-tls` |
| `layerx-receipt-authority` | `9443` | `authority-tls` |
| `layerx-agent-boundary` | `9443` | `agent-tls` |
| `layerx-agentd` | `9443` | `agentd-tls` |

The core, authority and agent Services are ClusterIP services. The node manifest
has no public Ingress object; public access is configured at the gateway edge.

### Image requirements

The manifest names `ghcr.io/sidiora-labs/layerx-kernel:0.1.0` and expects
`/usr/local/bin/paxd`, the native supervisor, Python helpers and signer provider.
The image must include all of those runtime components.
`docker/kernel/Dockerfile` supplies the kernel image but does not install
`paxd` or `native_runtime.py`.
`docker/paxeer/Dockerfile.paxd-node` builds the chain executable with its native
libraries at `/usr/bin/paxd`; it does not supply the hosted Python supervisor.
Use an image whose contents satisfy the native launch contract before deploying
this StatefulSet, and match `--paxd` to the actual executable path.

## Bootstrap inputs

`bootstrap.sh` delegates to `native_runtime.py bootstrap`. Supervision performs
bootstrap automatically when no prepared generation exists. Bootstrap requires
an empty, owner-controlled data directory and the following explicit inputs:

| Option | Meaning |
| --- | --- |
| `--data-dir` | Native chain home |
| `--native-inputs` | Directory containing authoritative native configuration |
| `--chain-id` | Consensus chain identity, `hyperpax_125-1` for mainnet-beta |
| `--network-id` | Hosted control-plane identifier |
| `--evm-chain-id` | EVM chain ID, `125` for mainnet-beta |
| `--asset` | Nonzero asset identifier, 64 lowercase hex characters |
| `--gateway-endpoint` | HTTPS root origin with an explicit port |
| `--receipt-key-file` | Private file containing a 64-character lowercase hex seed |
| `--treasury-signer-socket` | Protected absolute treasury signer socket path |
| `--funding-sender-key-file` | Protected absolute funding sender key path |

Supervision additionally requires `--run-dir` and `--state-dir`.
The defaults for `--chain-rpc-url` and `--consensus-rpc-url` are
`http://127.0.0.1:8545` and `http://127.0.0.1:26657`.
Both must be root origins with explicit ports. Bootstrap validates the gateway
origin as HTTPS and the core chain origin as loopback HTTP.

The native-input directory must supply `genesis.json`, `config.toml`,
`app.toml`, `priv_validator_key.json` and `rpc-ca.der`. Files read through
`read_regular` must be regular, owned by the running UID, singly linked,
within the size bound and not writable by group or others; symlinks are refused.
The validator key file must have mode `0600`.

Bootstrap runs `paxd init --mode validator`, copies the authoritative files into
`config/`, and runs `paxd validate-genesis`. Genesis must match `--chain-id` and
contain validators, bank state and `app_state.lxphost`. The receipt key's public
key and SHA-256 key ID must match an `app_state.sequencer.receipt_keys` entry
covering height 1. A missing or mismatched pin refuses bootstrap.
Joining an existing chain requires that chain's authoritative genesis and
validator configuration; initialization does not create a new mainnet validator
membership.

The current ConfigMap sets `network-id: "402"`, `chain-id: "hyperpax_125-1"`
and `paxeer-chain-id: "125"`. The first value is used for hosted lifecycle
requests; it does not change the EVM chain ID. The manifest maps these keys to
`LAYERX_NODE_NETWORK_ID`, `LAYERX_NODE_CHAIN_ID` and `LAYERX_NODE_EVM_CHAIN_ID`.
It maps `asset-id` to `LAYERX_NODE_ASSET_ID` and `gateway-endpoint` to
`LAYERX_CORE_SEQUENCER_ENDPOINT`.

## Storage, keys and generation publication

| Path in the hosted pod | Purpose |
| --- | --- |
| `/var/lib/layerx/node` | Persistent `paxd` home, including consensus state and `sequencer-wal` |
| `/var/lib/layerx/supervisor-state` | Persistent lifecycle journal and staged generations |
| `/run/layerx/node` | Runtime process identity and local sockets |
| `/run/native-private/owner` | Private copies of native configuration, validator key, receipt key and CA |
| `/run/layerx/node/treasury-signer.sock` | Treasury signer boundary |

The first two paths share the `data` PVC, requested at `50Gi`. The runtime
volume is a memory-backed `emptyDir` with a `16Mi` limit. Data and lifecycle
state directories must be owned by the supervisor UID with mode `0700`;
the run directory accepts `0700` or `0750` and is set to `0750`.
Staged generations must share the live data device, and the live data and
state directories must not overlap.

The `native-material` init container copies native inputs from ConfigMap
`layerx-node-native-genesis`, keys from Secret `layerx-node-native-keys` and
the DER CA from `layerx-internal-ca` into the private directory. It writes
files with mode `0600`. The receipt seed stays in the private key mount;
generation publication carries its public identity.
The separate treasury signer reads
`/run/treasury-private/owner/treasury.key`; the core uses its socket.
The funding key is staged separately at `/run/funding-private/owner/sender.key`.

Bootstrap publishes `node.env`, `core.env`, `rpc-ca.der`, a `.generation-proof`
copy and `.native-prepared.json` with SHA-256 bindings. Publication uses private
files, atomic rename and directory synchronization.

`node.env` contains the hosted network ID, consensus and EVM chain IDs,
receipt signer ID/public key, `LAYERX_NODE_RPC_URL` and
`LAYERX_NODE_CONSENSUS_RPC_URL`. `core.env` contains the signer ID, treasury
asset and signer socket, gateway endpoint, CA path, chain URL and funding key
path under the corresponding `LAYERX_CORE_*` names. In particular,
`LAYERX_CORE_SEQUENCER_ENDPOINT` names the gateway root origin and
`LAYERX_CORE_CHAIN_RPC_URL` names the local chain origin.

The local `generation.sock` broker and `generation_client.py` transfer a
validated generation to the core through file descriptors. The broker requires
the configured peer UID/GID, default `4021:4020`, and version 2 requests.
These sockets carry local lifecycle/configuration traffic, not an independent
execution or finality plane.

## Readiness and ordinary restart

Both startup and readiness probes invoke:

```sh
python3 /opt/layerx/native_runtime.py health \
  --data-dir /var/lib/layerx/node --run-dir /run/layerx/node
```

The health command checks the recorded process identity, generation binding
and native committed state. `committed_health` requires:

1. Consensus `status` reports the configured chain, a positive height and
   `catching_up: false`.
2. Consensus `commit` returns a canonical signed commit at that height with the
   matching block hash and chain ID, signatures and a valid app-hash encoding.
3. `eth_chainId` matches the configured EVM chain ID.
4. `eth_getBlockByNumber` at that height matches the consensus block hash.

The RPC helper posts to `/`, checks the JSON-RPC ID and result, and rejects
transport errors or error envelopes. A successful health command prints
`height` and `block_hash`; a refused health command exits `1` with
`native node refused: ...`. A running process or an execution receipt alone
does not satisfy these readiness checks.

For an ordinary restart, retain the chain home, `sequencer-wal`, validator
signing state and supervisor journal together. Restart the supervisor with the
same inputs and options. It checks the retained publication and network binding,
starts `paxd` against that home and waits for committed readiness. The startup
wait defaults to 120 seconds (`--ready-seconds`, accepted range 1–300).
If `paxd` exits before readiness or a supervised child exits later, supervision
fails instead of publishing a healthy generation. Preserve the committed data
and diagnose the refusal before attempting recovery; a reset is a separate
operation that replaces the generation.

## Controlled reset and recovery

`reset_state.py` implements the local `supervisor.sock` protocol and persistent
reset journal. A version 1 JSON request contains exactly `version`, `operation`,
`reset_id`, `network` and `request_digest`. The ID is 32 lowercase hex
characters; the digest is SHA-256 of canonical JSON containing `version: 1`,
`operation: "reset"`, that ID and the network. Status requests reuse the same
digest with `operation: "status"`. The network value must match the configured
hosted control-plane identifier.

The journal records `admitted`, `stopping`, `bootstrapping`, `prepared`,
`activating` and `completed` phases, with `ambiguous` for unresolved state.
The supervisor prepares and seals a generation under
`STATE_DIR/generations/RESET_ID/data`, stops the old process before activation,
activates the staged data and waits for native committed health. Completion
publishes the durable response with `state: "reset"`, `reset_id` and
`generation`.

On interruption, retain the same reset ID and inputs and query its status;
the supervisor reconciles the persisted phase on restart. Do not issue a new
reset to mask `reset_timeout` or `reset_ambiguous`, or delete the lifecycle
journal. The plain `status` compatibility request reports supervisor generation
state; use the native health probe to establish committed-chain readiness.
Validator recovery must preserve signing state and restore a coherent chain
home and journal. Rebuilding a test generation is not a mainnet recovery step.

## NetworkPolicy

`layerx-node-ingress` permits gateway pods to reach native ports `8545` and
`26657`, and `app=layerx-native-validator` peers to reach P2P port `26656`.
Additional rules admit the control, registry, faucet, developer and gateway
clients to their hosted API ports. `layerx-node-egress` includes native
validator P2P, cluster DNS, Paxeer boundary, receipt-authority and gateway
traffic. Separate policies add Human event delivery and mirror HTTPS egress.
Local Unix sockets do not cross these policies. Keep consensus and internal
service ports within the configured cluster access boundaries.

## Tests

The hosted-node lifecycle tests live in
`services/platform/hosted/node/tests/`, including
`test_data_directory.py`, `test_generation_transport.py`, `test_reset_state.py`,
`test_node_without_guarantor.py`, `test_retired_node_drivers.py` and
`test_repair_22_31_355.py`. They cover data ownership, generation transport,
reset persistence and the native lifecycle. Their presence does not certify a
published image or a running cluster.

[Home](Home.md)
