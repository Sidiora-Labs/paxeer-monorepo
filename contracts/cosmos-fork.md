# Cosmos fork contract

This page describes the Cosmos SDK fork under [`sdk/`](../sdk), its integration
with [`node/app.go`](../node/app.go), and the SDK contracts linked from the
interface registry. Go packages use the root module path
`github.com/Sidiora-Labs/Paxeer-X-Network/sdk/...`; nested client packages can
have their own manifests. The compiled declarations and registered application
services define the available API.

## Network boundary

Paxeer mainnet-beta is one BFT network, run by `paxd`, with EVM chain ID **125**
(`0x7d`). The kernel is part of the network and proposes blocks. A transaction
receipt is final when its block is committed. Sub-chains are tenant domains in
the database, launchable without separate chain infrastructure.

Public Ethereum JSON-RPC uses the HTTPS root of `api1.mainnet-beta.paxeer.network`
through `api14.mainnet-beta.paxeer.network`, or
`api-mainnet-beta.paxeer.network`; for example,
`https://api5.mainnet-beta.paxeer.network`. Node namespaces are `debug`, `echo`,
`eth`, `net`, `pax`, `test` and `txpool`. Hosted `px_` methods are declared in
the [gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json).
`node.hyperpaxeer.com` is only the build library mirror, not an RPC endpoint.

The SDK's consensus client, gRPC services and REST gateway use their own
protocols and configured listeners. See the [consensus contract](consensus-fork.md)
for consensus transport. The pre-merge public router, separate LayerX interface
and `layerxd` binary are retired.

## Go API

[`BaseApp`](../sdk/baseapp/baseapp.go) owns application contexts, transaction
execution and commit state. [`sdk.Context`](../sdk/types/context.go) carries the
multistore, header, gas meters and event manager. Keep these values from the same
execution branch when handing work between application components.

[`node/app.go`](../node/app.go) constructs module keepers and registers their
services through the module configurator, `MsgServiceRouter()` and
`GRPCQueryRouter()`. These are in-process dispatch mechanisms. A keeper method
or generated query type alone does not establish a public HTTP endpoint.

[`codec/types.InterfaceRegistry`](../sdk/codec/types/interface_registry.go)
registers concrete protobuf implementations and resolves `Any` values against
that registry. Its default unpack limits are 100 subcalls and recursion depth
10. Use the application's encoding configuration and registered type URLs;
upstream Cosmos types are not a substitute for this fork's declarations.
Module message/query schemas and generated bindings live beside their owners
under [`sdk/x/`](../sdk/x) and [`sdk/proto/`](../sdk/proto).

### Historical stores and recovery

[`SetupPaxDB`](../node/paxdb.go) installs
[`sdk/storev2/rootmulti`](../sdk/storev2/rootmulti/store.go) before other BaseApp
options. `state-commit.sc-enable` must be true; startup panics if it is disabled.
State commit (SC) and historical state store (SS) are storage roles within the
node. SS is configured with `state-store.ss-enable`, `state-store.ss-backend`
and `state-store.ss-db-directory`. A positive `state-sync.snapshot-interval`
requires SS when SC is enabled.

`CacheMultiStoreWithVersion(version)` uses SS for historical contexts; with SS
disabled it permits only the latest state and rejects other versions. Proof
queries use the SC tree and commit proof. Historical SC loads are bounded by
`state-commit.sc-historical-proof-max-inflight`,
`state-commit.sc-historical-proof-rate-limit` and
`state-commit.sc-historical-proof-burst`. Admission refusal returns `ErrConflict`
with `historical proof busy` or `historical proof rate limited`. A requested
proof without proof operations returns `ErrInvalidRequest`; a missing or pruned
version must not be interpreted as an empty account or successful absence proof.

The older [`sdk/store/rootmulti`](../sdk/store/rootmulti/store.go) mounts plain
DB adapters for `StoreTypeIAVL` and `StoreTypeDB`. It explicitly refuses
`CacheMultiStoreWithVersion` and versioned export with
`ErrVersionedStateUnsupported`; snapshot and restore return `ErrLogic` explaining
that legacy IAVL snapshots/restore are unsupported. Its unversioned
`CacheMultiStore()` branches working state. These legacy wrappers do not provide
PaxDB's historical-state guarantees. Preserve the configured persistent stores
and snapshot data across restarts; an in-memory cache is not recovery evidence.

[`prefix.Store`](../sdk/store/prefix/store.go) accepts keys relative to its
prefix. Iterators and `GetAllKeyStrsInRange` strip that prefix from returned
keys. `DeleteAll` and enumeration translate the range to the parent namespace;
a nil end uses the next prefix boundary. Callers must keep parent keys and
prefix-relative keys distinct.

### Genesis streams

[`node/genesis.go`](../node/genesis.go) reads `genesis.stream-import` and
`genesis.genesis-stream-file`; defaults are false and an empty path. Enabling
stream import requires a nonblank file path. This is the streamed export format,
not an arbitrary line-split ordinary genesis JSON document.

[`genesis.Stream`](../sdk/types/genesis/genesis.go) owns its source and exposes
`Lines`, `Wait() error`, `Close() error` and `Consume(func(string) error) error`.
Drain `Lines` and check `Wait`, or use `Consume`; channel closure alone does not
prove successful reading. Close the stream when stopping early. Open/read/close
failures and cancellation are reported through the completion API.

[`Manager.InitGenesisStream`](../sdk/types/module/module.go) accepts contiguous
module groups in `OrderInitGenesis`, followed by terminal genesis metadata.
Each module record has the shape
`{"app_state":{"module":"bank","data":{...}}}`. Unknown modules, repeated
groups, missing groups, invalid JSON, invalid metadata and records after terminal
metadata are refused. Import works in a cached context and writes it only after
the entire stream succeeds. Export and import must use matching module schemas
and ordering.

## BaseApp commit hooks

[`sdk/baseapp/abci.go`](../sdk/baseapp/abci.go) exposes:

| Method | Selected state |
| --- | --- |
| `SetDeliverStateToCommit()` | Deliver state |
| `SetProcessProposalStateToCommit()` | Process-proposal state |
| `SetSealedStateToCommit(ctx sdk.Context, ms sdk.CacheMultiStore) error` | Supplied sealed store and context |

The sealed setter binds the context to `ms`. A nil store returns
`cannot commit a nil sealed multistore` without changing the selected state.
[`App.FinalizeBlocker`](../node/app.go) uses it when the locally sealed block
matches the decided block. `Commit(context.Context)` requires selected state,
uses its header, runs the precommit handler, writes its cache and commits the
multistore. It then resets check state and clears other execution states.
Selecting sealed state is not itself a durable commit.

## CLI

[`daemon/paxd/cmd/root.go`](../daemon/paxd/cmd/root.go) registers `paxd` commands,
including `start`, `status`, `config`, `keys`, `query`, `tx`, genesis commands and
server maintenance commands. Module query and transaction commands are added
through the module basic manager. Command constructors retained in `sdk/` must
be registered by the application to become reachable.

[`client.toml`](../sdk/client/config/toml.go) lives under the selected home's
`config/` directory. `paxd config <key> [value]` reads or updates these keys:

| Key | SDK configuration default |
| --- | --- |
| `chain-id` | Empty string; configure the consensus genesis chain ID |
| `keyring-backend` | `os` |
| `output` | `text` |
| `node` | `tcp://localhost:26657`, the consensus listener |
| `broadcast-mode` | `sync` |

These defaults come from [`config.go`](../sdk/client/config/config.go). The root
command initially constructs a block-mode client context, then reads the client
configuration. The consensus chain-ID string is distinct from the EVM chain ID.
Do not point the SDK consensus client at an Ethereum JSON-RPC endpoint.

[`client.BroadcastTx`](../sdk/client/broadcast.go) supports `sync`, `async` and
`block`. It can return a nonzero-code `TxResponse` with a nil Go error for
mempool refusal. Check both the Go error and response code; successful sync or
async submission alone does not prove committed execution.

## Errors

See [error codes](#error-codes) for ABCI identities. Transport errors, gRPC
statuses, integration errors and ABCI codes have separate representations;
preserve the owning protocol's fields when reporting a failure.

## Error codes

[`sdk/types/errors`](../sdk/types/errors/errors.go) registers errors by codespace
and numeric code. [`ABCIInfo`](../sdk/types/errors/abci.go) returns codespace,
code and log; success has code 0 and empty codespace/log. Unclassified errors
use `undefined`/1. The current non-debug encoder returns `err.Error()`, so callers
must not assume it redacts arbitrary error text. Debug encoding uses `%+v`.

| Codespace | Code | Sentinel |
| --- | --- | --- |
| `sdk` | 2 | `ErrTxDecode` |
| `sdk` | 4 | `ErrUnauthorized` |
| `sdk` | 5 | `ErrInsufficientFunds` |
| `sdk` | 11 | `ErrOutOfGas` |
| `sdk` | 18 | `ErrInvalidRequest` |
| `sdk` | 19 / 20 / 21 | `ErrTxInMempoolCache` / `ErrMempoolIsFull` / `ErrTxTooLarge` |
| `store` | 2 | `ErrInvalidProof` |
| `bank` | 5 | `ErrSendDisabled` |
| `staking` | 3 | `ErrNoValidatorFound` |

Module-specific declarations remain in their owners' `errors.go`, including
[bank](../sdk/x/bank/types/errors.go), [staking](../sdk/x/staking/types/errors.go)
and [store](../sdk/store/types/errors.go). Compare both codespace and code;
numeric values alone do not identify an error across modules.

## Bank gateway

The generated [bank gateway bindings](../sdk/x/bank/types/query.pb.gw.go) register
`GET /cosmos/bank/v1beta1/supply/by_denom?denom=...` and
`GET /cosmos/bank/v1beta1/denoms_metadata_by_query_string?denom=...` alongside
path-parameter forms. Query-string denoms support names such as
`factory/pax1creator/sub`. These routes require the SDK REST gateway registration;
they are not Ethereum JSON-RPC methods.

[`BaseSendKeeper`](../sdk/x/bank/keeper/send.go) checks send-enabled parameters
and registered `SendEnabledChecker` callbacks. `node/app.go` registers the
tokenfactory checker for both bank keepers. A false checker refuses the denom;
recipient checks and send-enabled checks are separate hooks.

## Gas meter

[`GasMeter`](../sdk/store/types/gas.go) uses unsigned 64-bit gas values.
`ConsumeGas` panics with `ErrorOutOfGas` when consumed gas exceeds the limit,
or `ErrorGasOverflow` on overflow. Refunding more than consumed gas panics with
`ErrorNegativeGasConsumed`. These typed execution failures are handled by the
application's transaction path, not exposed as a second network protocol.

## OCC

[`tasks.NewScheduler`](../sdk/tasks/scheduler.go) schedules transaction execution
against [`multiversion`](../sdk/store/multiversion) stores. It tracks transaction
indices, incarnations and dependencies, validates execution and re-executes
conflicting work. This concurrency mechanism operates inside the application;
its speculative results must pass the normal finalization and commit path.

## Client SDKs

The registry also covers language clients and integrations under `sdk/`:

| Source | Public entry point or boundary |
| --- | --- |
| [Chainflow Go](../sdk/chainflow/go/client.go) | `NewClient(opts ...option.RequestOption)` |
| [Chainflow TypeScript](../sdk/chainflow/typescript/src/client.ts) | `ChainflowDataGateway` |
| [Chainflow Python](../sdk/chainflow/python/src/chainflow_data_gateway/_client.py) | `ChainflowDataGateway`, `AsyncChainflowDataGateway` |
| [Chainflow Rust](../sdk/chainflow/rust/src/client.rs) | `ChainflowDataGatewayClient`, `ChainflowDataGatewayClientBuilder` |
| [Framework integrations](../sdk/integrations) | Payment, receipt and webhook adapters |

Chainflow clients describe the data gateway's REST and WebSocket resources;
their API base URL is service configuration. They do not select a different
execution network. The Go client reads `SIDIORALABS_BASE_URL`, `REST_API_KEY`,
`WS_MINT_KEY`, `WS_TOKEN` and `SIDIORALABS_CUSTOM_HEADERS`; explicit request
options are applied after defaults. Use each language client's own declared
models and error types rather than assuming identical configuration behavior.
Launchpad-specific clients are covered by the [client contracts](clients.md).

## Integration receipts

[`express/src/index.ts`](../sdk/integrations/express/src/index.ts) reads
`LAYERX_RECEIPT_KEYS_JSON`. It requires 1–32 key objects with exactly `keyId`,
`publicKey`, `firstHeight` and `lastHeight`. IDs and public keys are 32-byte hex;
heights are unsigned 64-bit decimal strings with first height no greater than
last height. The returned keys use `first_height` and `last_height` BigInts.
The former `LAYERX_AUTHORIZED_BATCH_JSON` configuration is retired.

`ReceiptPayloadAuthority` receives the pinned receipt keys.
`assertReceiptBacked` checks the existing `sequencer-signed` verification level,
zero result code, payment asset/recipient/amount, settlement network and
transaction identity, and the canonical receipt digest before releasing a
resource. Mismatches produce `receipt-not-backed`. This signed evidence check
does not replace the network rule that finality requires a committed block.
Language adapters retain their own payload names and verification paths; use
the producer's wire shape rather than translating names by assumption.

## Telemetry

SDK OTel instruments are defined beside their owners in `metrics.go` and
recorded on the corresponding execution paths. Export configuration and
collection determine their visibility; declaring an instrument does not prove
that a particular node has emitted it.

## OTel instruments

The following names are OTel scope/instrument names, before exporter-specific
Prometheus naming transformations:

| Scope and source | Instruments and units |
| --- | --- |
| [`paxcosmos_baseapp`](../sdk/baseapp/metrics.go) | `commit_duration`, `mid_block_duration`, `end_block_duration`, `deliver_tx_duration`, `abci_query_duration`, `process_proposal_duration`, `finalize_block_duration`, `get_tx_priority_hint_duration`, `run_tx_duration`, `run_msgs_duration`, `run_msg_latency`: histograms in seconds |
| `paxcosmos_baseapp` | `tx`, `tx_result`: counters; `tx_gas_used`, `tx_gas_wanted`: gauges in gas |
| [`paxcosmos_store_types`](../sdk/store/types/metrics.go) | `gas_exceeded`: counter; `last_evicted_keys_bounded_cache`: gauge |
| [`paxcosmos_storev2_rootmulti`](../sdk/storev2/rootmulti/metrics.go) | `sc_commit_latency`: seconds histogram; `ss_version`: version gauge; `historical_abci_query`, `state_sync_keys_exported`: counters; `iavl_total_key_bytes`, `iavl_total_value_bytes`: byte gauges; `iavl_total_num_keys`: key-count gauge |
| [`paxcosmos_x_bank`](../sdk/x/bank/metrics.go) | `init_genesis_duration`: seconds histogram |
| [`paxcosmos_x_staking`](../sdk/x/staking/metrics.go) | `staking_begin_blocker_duration`, `staking_end_blocker_duration`: seconds histograms |
| [`paxcosmos_x_upgrade`](../sdk/x/upgrade/metrics.go) | `begin_blocker_duration`: seconds histogram; `plan_height`: block-height gauge |
| [`paxcosmos_x_upgrade_keeper`](../sdk/x/upgrade/keeper/metrics.go) | `upgrade_pending_plan_height`: block-height gauge |

Upgrade plan gauges attach `name`; the former `info` label is removed. Gas
exceeded counters attach `error` and `descriptor`. Other modules have their own
scopes and instrument definitions; use their source names when collecting data.

## Legacy telemetry

[`sdk/telemetry`](../sdk/telemetry/metrics.go) retains the legacy collection API.
Its existence does not imply that migrated SDK call sites still emit legacy
go-metrics names. OTel definitions above describe those migrated paths; old
registry rows and source comments naming former emissions do not establish a
second emission. Absence tests and OTel behavior tests live beside the owners.

## OTel test harness

[`oteltest.Install()`](../sdk/testutil/oteltest/oteltest.go) installs one global
meter provider with a manual reader and returns that same reader on subsequent
calls. `HistogramCount`, `CounterSum` and `GaugeInt` query by scope and instrument
name, aggregating matching data points. Tests should compare before/after values
for the real operation because collection is cumulative and the provider is
shared. `InstallLegacySink(t)` captures legacy emissions and `Keys()` returns
their sorted names; test cleanup replaces that sink with a blackhole sink.
