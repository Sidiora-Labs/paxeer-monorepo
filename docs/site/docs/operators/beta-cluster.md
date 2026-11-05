# Disposable beta cluster

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
The kernel runs inside `paxd` and participates in block proposal and execution.
A receipt is final when its block is committed. The former standalone daemon,
separate settlement bootstrap, custody-credit provisioning and guarantor
finality model are retired.

This page covers the native hosted-node configuration and the disposable
cluster tooling in `services/platform/hosted/tests/beta-cluster.sh`. A local
cluster using chain ID `125` is still an independent disposable environment:
chain ID alone does not establish mainnet identity. Hosted services provide
application APIs around the node. Sub-chains are tenant domains in the database;
creating a tenant does not require another consensus network.

## Network and service boundaries

Public JSON-RPC uses the root of `https://api-mainnet-beta.paxeer.network` or
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. See [Public RPC endpoints](../reference/public-rpc.md).
The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and
`txpool`. Hosted `px_` methods belong to the gateway contract in
`services/platform/hosted/gateway/openrpc.json`; see
[Hosted gateway JSON-RPC](../platform/gateway-rpc.md).
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.

The native StatefulSet in `services/platform/hosted/node/deployment.yaml` is
`layerx-node`. Its `paxd` container runs `/opt/layerx/supervisor.sh`, which
executes `native_runtime.py supervise`. The manifest selects image
`ghcr.io/sidiora-labs/layerx-kernel:0.1.0` for that container. It exposes the
node through Service `layerx-native-chain`: JSON-RPC port `8545` and consensus
RPC port `26657`. These are private cluster addresses, not public DNS names.

Services `layerx-pending-core`, `layerx-pending-core-admin` and
`layerx-receipt-authority` select the same hosted-node pod. They are private
compatibility and application boundaries, not independent sources of chain
finality. [Hosted control](beta-control.md) reports their reachability.
Identity, Human, gateway, registry, webhooks and explorer services have their
own deployment manifests under `services/platform/hosted/`.

## Native bring-up prerequisites

Provision the native node's inputs before starting its StatefulSet. The
manifest's `native-material` init container copies projected files into a
private memory volume. `native_runtime.py` validates them and lets `paxd`
validate genesis; it does not manufacture a replacement genesis from the old
custody profile.

| Resource | Required material |
| --- | --- |
| ConfigMap `layerx-node-config` | `network-id`, `chain-id`, `paxeer-chain-id`, `asset-id` and `gateway-endpoint`; the manifest uses namespace ID `402`, Comet chain ID `hyperpax_125-1` and EVM chain ID `125` |
| ConfigMap `layerx-node-native-genesis` | `genesis.json`, `config.toml`, `app.toml` |
| Secret `layerx-node-native-keys` | `priv_validator_key.json`, `receipt.key` |
| Secret `layerx-node-native-funding` | `sender.key`, the native EVM funding sender key |
| Secret `layerx-node-keys` | `treasury.key`, used by the treasury signer |

The complete manifest also requires its TLS, trust, Human, publication and
service credential resources. The trust volume supplies the RPC CA as
`rpc-ca.der` to the native runtime. Keep private keys in Secrets and private
storage, with the file ownership and permissions established by the init
containers.

The native runtime requires an authoritative genesis with non-empty validators
and `app_state.bank` and `app_state.lxphost`. Its chain ID must match the
configured Comet chain ID. The validator key must be mode `0600`; `receipt.key`
is a hexadecimal 32-byte seed. Its public key and SHA256 key ID must match a
record in `app_state.sequencer.receipt_keys` whose inclusive height range
starts at `1` and includes that height. Fresh gateway material additionally
requires `LAYERX_BETA_RECEIPT_KEYS_FILE`, a JSON export of governed ReceiptKeys;
`sequencer-pins.py --gateway-check` checks that export. Receipt signer pins
must agree with the native chain, rather than an archived batch trust history.

Start the native node with its supplied material, wait for its native health
probe, then make the private application dependencies available before relying
on control journey admission. The control configuration and its exact probe
sets are on [Hosted control](beta-control.md#journeys-and-probes).

## Cluster tooling and current limits

The helper's work directory is `build/beta-cluster`. With
`LAYERX_BETA_KUBECONFIG` unset it selects a local kind cluster named
`LAYERX_BETA_CLUSTER_NAME` (default `layerx-beta`); a set kubeconfig selects an
owner cluster. `beta-images.sh` owns `IMAGE_NAMES` and `image_source`, the
shared image inventory and Dockerfile mappings. Check the workload manifest
as well as that table when selecting the native runtime image.

| Command | Purpose |
| --- | --- |
| `bash services/platform/hosted/tests/beta-cluster.sh images` | Build or pull the image inventory without creating a cluster |
| `bash services/platform/hosted/tests/beta-cluster.sh render` | Prepare material and render manifests under `build/beta-cluster/manifests`; applies no workloads |
| `make platform-beta-cluster-up` | Invoke the helper's `up` path, forwarding `PLATFORM_BETA_CLUSTER_FLAGS` |
| `make platform-hosted-smoke` | Consume `PLATFORM_BETA_CLUSTER_ENV` (default `build/beta-cluster/env`) and exercise hosted journeys with real credentials |
| `make platform-beta-cluster-down` | Invoke destructive disposable-cluster teardown |

These are command entrypoints, not a claim that the full helper is qualified
for the merged deployment. Its current `beta_cluster_up` still invokes
`human_custody_step bootstrap`, `wait_for_node_genesis` and
`settlement_publish`, and applies the older `paxeer.yaml` fixture before the
native hosted-node manifest. That fixture has its own chain home and observer.
Those compatibility provisioning steps do not establish the merged node's
native genesis or its finality. Do not use the helper unchanged as a mainnet
installation or interpret a rendered manifest as a running, recovered node.

The public node endpoint list is not an input for disposable resets or test
funding. Use the cluster's private origins and CA. The helper writes
`paxeer/rpc-origins.json` for its fixture origins, including the CA, backend
homes and genesis identity. Preserve that identity when using those fixtures;
matching `eth_chainId` alone is insufficient.

## Readiness

The native node's startup and readiness probes execute:

```sh
python3 /opt/layerx/native_runtime.py health \
  --data-dir /var/lib/layerx/node --run-dir /run/layerx/node
```

This is an in-container command. `committed_health` checks consensus `status`,
requires a positive height and `catching_up == false`, fetches the canonical
signed `commit` at that height, and checks its chain, height and block hash.
It then requires `eth_chainId == "0x7d"` and the EVM block at that height to
match the committed block hash. A listening socket or an open head alone does
not satisfy native readiness.

Control `GET /readyz` is a separate application health check. It requires all
eight dependencies, all three journeys and the pending release versions to
be ready. It is not a consensus certificate. The helper saves the response as
`build/beta-cluster/readyz.json`; its readiness timeout is
`LAYERX_BETA_READY_TIMEOUT` (default `900` seconds). Inspect named dependency
failures before retrying. Hosted smoke performs funding, payment and receipt
inspection after admission and needs its generated environment and credentials.

## Funding and explorer read identity

Fund public accounts by a native value transfer through the chosen node's
root JSON-RPC, using a funded signer and checking the transaction receipt.
Private faucet funding goes through control to core-admin, which uses the
native EVM transfer implementation in `core/src/chain_fund.rs` and the durable
intent/archive implementation in `core/src/funding_native.rs`. See
[Private funding](beta-control.md#private-funding-and-recovery) for identity
binding and retry behavior. The retired custody-credit path is not a
prerequisite for native transfers or explorer reads.

`explorer_read_principal_generate` still generates and retains an Ed25519
service key alongside other cluster credentials. `secrets_apply` publishes
`read-key` and `sequencer-public-key` in Secret `layerx-explorer-index`.
`registry/deployment.yaml` retains `LAYERX_EXPLORER_READ_KEY_FILE`,
`LAYERX_EXPLORER_READ_ENDPOINT`, `LAYERX_EXPLORER_READ_CA_DER`,
`LAYERX_EXPLORER_READ_SEQUENCER_PUBLIC_KEY_FILE`,
`LAYERX_EXPLORER_READ_NETWORK_ID` and `LAYERX_EXPLORER_READ_FEE_LIMIT`.
These are compatibility configuration, not proof of identity registration,
balance or a successful naming read. `human_native_provision` now produces
and validates owner registration; it does not append explorer identities or
fund them through custody. The old sequence-zero and fixed occupancy-credit
instructions are retired. Check the explorer's actual read response and native
identity/balance state before treating its configuration as ready.

## Retained state and recovery

The native runtime stores its chain generations under `/var/lib/layerx/node`
and supervisor records under `/var/lib/layerx/supervisor-state` on the node's
50Gi `data` PVC. Human state uses the separate `layerx-human-state` PVC. Core
funding journals persist under `/var/lib/layerx/core`. Retain these volumes,
the authoritative native inputs, validator state, receipt keys, funding keys
and trust material together.

For a process restart, retain the existing data and let the supervisor reopen
its generation and reset records. Recheck native committed health and the
application journeys. If a funding response was indeterminate, retry the same
request with the same idempotency key; inspect the saved intent and receipt
rather than sending a second transfer. Private reset creates a new disposable
generation through the supervisor; it is not restart recovery for mainnet.

`LAYERX_BETA_RETAIN_MATERIAL=1` makes the helper validate an existing material
inventory, file permissions, registry consistency and live KMS seal before
reapplying material. It refuses incomplete material. Its retained path still
expects compatibility artifacts, so this option alone does not prove native
recovery. Keep CA, Secrets and private inventories out of published logs.

## Teardown

`beta-cluster.sh down` stops port-forwards and deletes the recorded kind
cluster, or deletes namespaces `layerx-testnet`, `layerx-developer` and
`layerx-internal` in owner mode. It removes recorded/labeled images and
`build/beta-cluster`; pinned tools are also removed unless
`LAYERX_BETA_KEEP_TOOLS=1`. Cluster deletion removes access to its persistent
state; namespace/PVC deletion can also destroy storage according to the
storage reclaim policy. Retained-material mode does not preserve local
material across `down`. Use teardown only for the disposable environment
whose loss is intended.

[Hosted control](beta-control.md) · [Home](../index.md)
