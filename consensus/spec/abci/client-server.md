---
order: 3
title: Client and Server
---

# Client and Server

ABCI integration in this fork is in-process Go integration. `paxd` embeds the
consensus engine and supplies an application object to
[`node.New`](../../node/public.go). This is an internal node API, separate
from the public JSON-RPC served by the network's node hosts at their root path.

Read [Applications](apps.md) for state isolation, execution results, recovery,
and snapshots. The local
[`Application`](../../abci/types/application.go) declaration is authoritative
when inherited [ABCI Methods and Types](abci.md) prose differs from this fork.

## Message Protocol

There is no ABCI wire framing on this integration path. A
[`Proxy`](../../internal/proxy/proxy.go) wraps the application and invokes its
methods directly with Go request and response types. Most methods take a
`context.Context` and return a response pointer and an error. `CheckTx` instead
returns `*ResponseCheckTxV2` directly; its node-facing wrapper is `CheckTxSafe`.
`GetValidators`, `EvmNonce`, and `EvmBalance` also have their own signatures.

[`plain_types.go`](../../abci/types/plain_types.go) declares the lifecycle request
types; other types are in [`abci/types`](../../abci/types/). Protobuf definitions
remain useful for shared data structures, persistence, and peer protocols, but
compiling the inherited protobuf file does not generate a working ABCI service
for this node.

## Server Implementations

### In Process

Implement `Application` and pass the object into the embedded node. The SDK
starts the node from [`sdk/server/start.go`](../../../sdk/server/start.go) and
provides [`BaseApp`](../../../sdk/baseapp/abci.go) as its application foundation.
The proxy adds metrics and call checks; it is not a remote client or a server
listener.

The method groups are:

| Role | Current methods |
| --- | --- |
| Information and queries | `Info`, `Query`, `GetValidators` |
| Transaction admission | `CheckTx`, `GetTxPriorityHint`, `EvmNonce`, `EvmBalance` |
| Consensus and execution | `InitChain`, `ProcessProposal`, `FinalizeBlock`, `Commit` |
| Snapshots | `ListSnapshots`, `OfferSnapshot`, `LoadSnapshotChunk`, `ApplySnapshotChunk` |

An implementation must satisfy this exact Go interface, including the EVM helper
methods. The historical `BeginBlock`/`DeliverTx`/`EndBlock` exchange is not the
application callback sequence used here.

### GRPC

The upstream ABCI gRPC server recipe is not implemented in this fork. There is
no supported `abci-cli --abci grpc` workflow for attaching an application to
`paxd`. Other gRPC services in the repository do not establish an ABCI transport.

### TSP

The upstream ABCI socket transport is also absent. The old page mixed two
incompatible historical encodings: a length-of-length big-endian prefix and a
protobuf varint prefix. Its `0x0104DEADBEEF` example described the former, not a
varint. That obsolete framing recipe is removed; this fork specifies neither
socket framing nor a TCP/Unix ABCI application listener.

### Async vs Sync

Proxy calls are ordinary synchronous function calls. There is no asynchronous
`DeliverTx` queue, connection flush, or transport response ordering guarantee.
Independent callers can still run concurrently: the proxy has no global mutex.
The application must protect shared state itself. In classic mode the executor
coordinates commit with its mempool lock; that does not serialize every query or
snapshot call. See [Connection State](apps.md#connection-state).

## Client

The engine is the application caller. In classic mode it consults
`ProcessProposal` before voting and, after deciding a block, calls `FinalizeBlock`,
saves the response, and calls `Commit`. `ResponseFinalizeBlock` carries
`AppHash`; `ResponseCommit` carries the retention height.

Autobahn executes finalized global blocks through `FinalizeBlock` followed by
`Commit` and publishes the resulting application hash to its own data layer.
It does not use classic proposal callbacks or classic startup replay in that
execution loop. The [consensus mode map](../consensus/consensus.md#consensus-modes)
and [crash recovery guide](apps.md#crash-recovery) explain the differences.

When diagnosing integration failures, distinguish a method's Go error from an
application result code. In particular, `CheckTxSafe` catches panics, rejects nil
responses, and requires EVM results to contain an EVM hash and Paxeer sender
address. A successful admission response is not a committed transaction receipt.
