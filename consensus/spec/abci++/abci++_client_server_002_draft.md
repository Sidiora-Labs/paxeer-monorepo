---
order: 5
title: Client and Server
---

# Client and Server

The consensus engine and application run in the same Go process in this fork.
An integrator implements the
[`Application` interface](../../abci/types/application.go) and supplies it to
[`node.New`](../../node/public.go). The node wraps it with
[`internal/proxy.Proxy`](../../internal/proxy/proxy.go), which forwards calls
and records method timings. There is no application network listener to start.

Read [Overview and basic concepts](./abci++_basic_concepts_002_draft.md) for the
current methods, result codes, determinism requirements, and consensus-mode
boundaries. This page retains its upstream draft filename and front matter for
existing site links. The sibling [Methods](./abci++_methods_002_draft.md),
[Application Requirements](./abci++_app_requirements_002_draft.md), and
[Expected Behavior](./abci++_tmint_expected_behavior_002_draft.md) pages contain
inherited draft material; they do not add methods or transports to the Go API.

## Message Protocol and Synchrony

The current interface is a Go method contract, not a serialized stream of
request/response envelopes. Its declarations are split across:

- [`application.go`](../../abci/types/application.go): the complete method set.
- [`plain_types.go`](../../abci/types/plain_types.go): request structures,
  proposal status, snapshot metadata, and other plain Go types.
- [`types.go`](../../abci/types/types.go): extended transaction request and
  response types and deterministic result encoding.
- [`types.proto`](../../proto/tendermint/abci/types.proto): protobuf response,
  event, validator, and execution-result structures.

Generating code from `types.proto` alone does not implement the interface.
For example, `ResponseCheckTxV2` embeds `*ResponseCheckTx` and adds Go-only
fields, including EVM hashes, addresses, and `uint256.Int` balance information.

Calls return synchronously to their caller. The proxy does not turn the
consensus, mempool, query, and snapshot method groups into four connections,
nor does it place a global lock around the application. Separate callers can
execute concurrently; application state and speculative execution must be
synchronized appropriately. Respect the context supplied to methods that take
one, and keep committed, admission, and speculative state consistent with the
lifecycle below.

## Server Implementations

### In Process

Implement all methods declared by `Application`, including `GetValidators`,
`GetTxPriorityHint`, `EvmNonce`, and `EvmBalance` as well as block processing,
queries, transaction checks, and snapshots. `CheckTx` returns
`*ResponseCheckTxV2` directly; most request-based methods return a response and
a Go `error`. Use the declared signatures rather than the upstream draft's
method list.

[`BaseApplication`](../../abci/types/application.go) supplies default behavior
and demonstrates the interface shape. The
[`kvstore` example](../../abci/example/kvstore/kvstore.go) embeds it and implements
a small application. Defaults do not implement a production application's
validation, persistence, or snapshot policy. The network application uses
[`BaseApp`](../../../sdk/baseapp/abci.go) and
[node-specific handlers](../../../node/app.go).

### GRPC

The upstream gRPC ABCI transport is not implemented in this fork. Running a
gRPC server generated from the response protobuf definitions does not connect
an application to this engine. There is no supported `abci-cli --abci grpc`
integration recipe here.

### Socket

The upstream ABCI socket client and server are not implemented in this fork.
There is no supported `abci-cli --abci socket` integration recipe. The earlier
draft's conflicting length-prefix descriptions and byte examples do not define
a wire protocol for this runtime. TCP and Unix socket framing are therefore
not application integration steps; use the in-process interface above.

## Execution and recovery

The configured consensus mode determines which code calls the shared interface.
The full method inventory and response semantics are in
[Methods overview](./abci++_basic_concepts_002_draft.md#methods-overview).

### Classic consensus

The [handshaker](../../internal/consensus/replay.go) begins with `Info` and uses
`LastBlockHeight` and `LastBlockAppHash` to reconcile the application with stored
blocks and consensus state. It initializes genesis with `InitChain` for a fresh
chain and replays blocks as needed on recovery.

During consensus, [`BlockExecutor`](../../internal/state/execution.go) calls
`ProcessProposal` to obtain an accept/reject status. After a block is decided,
`ApplyBlock` calls `FinalizeBlock`, saves its response, validates updates, then
calls `Commit` under the mempool lock. It updates the mempool against the
committed state and saves the resulting consensus state. Return the state hash
in `ResponseFinalizeBlock.AppHash`; `ResponseCommit.RetainHeight` controls
pruning and is not a hash response.

`FinalizeBlock` returns one `ExecTxResult` per transaction, preserving the input
order. Failed transactions still need results. Local `CheckTx` admission does
not replace execution-time validation, and speculative proposal work must not
commit a proposal that is later abandoned.

### Autobahn

A non-empty `autobahn-config-file` setting selects Autobahn in
[`node/node.go`](../../node/node.go). The JSON configuration is defined by
[`AutobahnFileConfig`](../../config/autobahn.go); this is a consensus mode within
the embedded node.

[`GigaRouter.runExecute`](../../internal/p2p/giga_router.go) calls `Info`, invokes
`InitChain` when the committed application height is zero, and otherwise resumes
after the last committed height. It reads ordered global blocks. For each block,
`executeBlock` calls `FinalizeBlock`, persists the finalization response, calls
`Commit`, and records `ResponseFinalizeBlock.AppHash` in Autobahn's data layer.
It uses `ResponseCommit.RetainHeight` when pruning.

This loop does not call `ProcessProposal`, and supplies an empty
`DecidedLastCommit` and no `ByzantineValidators`. The request hash identifies the
Autobahn header. Do not impose classic Tendermint vote, evidence, header, WAL,
or state-sync assumptions on this path. Durable restart requires both accurate
application height/hash reporting and Autobahn persistence; the configuration
requires `persistent_state_dir` unless the explicit test-only persistence
opt-out is used. Consensus persistence alone does not persist application state.

## Client

The engine is the caller of `Application`. Queries and transaction submissions
from external users enter node APIs; users do not establish ABCI connections or
send `RequestFinalizeBlock` to an application server.

For Paxeer mainnet-beta, the public EVM JSON-RPC uses HTTPS at the root path of
`api1.mainnet-beta.paxeer.network` through
`api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`, with chain ID 125 (`0x7d`). For example, the
HTTP endpoint is `https://api1.mainnet-beta.paxeer.network`.
The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test`, and `txpool`.
The `px_` methods belong to the hosted gateway, whose contract is
[`openrpc.json`](../../../services/platform/hosted/gateway/openrpc.json).
These public APIs are separate from the internal Go application contract.
