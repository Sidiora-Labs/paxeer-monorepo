---
order: 1
title: Overview
parent:
  title: Spec
  order: 7
---

# Consensus Specifications

This directory contains inherited Tendermint protocol specifications and research
models beside the consensus engine embedded in `paxd`. Paxeer mainnet-beta is one
BFT network, with EVM chain ID **125** (`0x7d`). The merged LayerX kernel is part
of that network and proposes blocks. A transaction receipt is final when its
block is committed.

The inherited documents are references for the algorithms they describe, not a
conformance statement for every mode of this fork. The Go implementations linked
below define the current application interface, block identity and recovery
behavior. In particular, the classic Tendermint specification does not specify
Autobahn.

## Table of Contents

- [Overview](#overview)
- [Mode applicability](#mode-applicability)
- [Autobahn protocol and implementation map](#autobahn-protocol-and-implementation-map)
- [Application requirements](./abci++/abci++_app_requirements_002_draft.md#current-fork-requirements)
- [Inherited specifications](#inherited-specifications)
- [Contributing](#contributing)

## Overview

The application and consensus engine run in the same process. The authoritative
Go interface is [`Application`](../abci/types/application.go); the
[proxy](../internal/proxy/proxy.go) forwards calls directly and records timings.
There is no socket or gRPC ABCI application transport in this fork. The
[application requirements page](./abci++/abci++_app_requirements_002_draft.md)
distinguishes the implemented lifecycle from the retained upstream ABCI++ draft.

Public Ethereum JSON-RPC uses the root HTTPS path, for example
`https://api1.mainnet-beta.paxeer.network`. The public node names are
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`,
and `api-mainnet-beta.paxeer.network`. These endpoints serve chain ID 125;
`eth_chainId` returns `0x7d`. The consensus engine's internal RPC routes and peer
transport are different interfaces from the public Ethereum JSON-RPC surface.

## Mode applicability

[`Config.AutobahnConfigFile`](../config/config.go) is read from the top-level
`autobahn-config-file` configuration key. A non-empty value selects Autobahn and
points to a JSON file described by [`AutobahnFileConfig`](../config/autobahn.go).
An empty value selects the classic engine path. See
[node construction](../node/node.go) for the actual services installed in each
mode. The integrated `consensus.sequencer.enable` option cannot be combined with
`autobahn-config-file`; configuration validation rejects that combination.

| Subject | Classic engine path | Autobahn path |
| --- | --- | --- |
| Proposals and agreement | Height/round consensus in [`internal/consensus`](../internal/consensus/); weighted validator votes. Integrated proposer selection also uses the application's `LeaderSource` when configured. | Lane proposals, ordered lane ranges, views and certificates in [`autobahn/types`](../autobahn/types/) and [`internal/autobahn/consensus`](../internal/autobahn/consensus/). |
| Block identity | Tendermint block/header and block ID in [`types/block.go`](../types/block.go). | Lane `BlockHeader.Hash()` plus a distinct `GlobalBlockNumber`; see [identity below](#block-identity-and-time). |
| Application execution | [`BlockExecutor`](../internal/state/execution.go) calls `ProcessProposal` for validation, then `FinalizeBlock` and `Commit` for the decided block. | [`GigaRouter.executeBlock`](../internal/p2p/giga_router.go) calls `FinalizeBlock` and `Commit` for global blocks in order. It does not call `ProcessProposal`. |
| Application commitments | `ResponseFinalizeBlock.AppHash` and deterministic transaction results update classic state; see `ApplyBlock` and `TxResultsHash`. | `PushAppHash` reports execution to the Autobahn data layer; `AppQC` certifies application-state proposals. |
| Restart | Classic handshake reconciles application height with block/state stores and replays as necessary. | Autobahn restores its consensus/data persistence and resumes from application `Info`; the classic handshake is bypassed. |
| Synchronization | Classic block-sync and snapshot state-sync reactors are assembled in the non-Autobahn branch. | Classic snapshot state sync is not assembled in the Autobahn branch; its data/certificate streams and persistence govern recovery. |
| Verification models | Tendermint signing, light-client, WAL, PBTS, TLA+ and Ivy material applies only within its stated classic/research assumptions. | Use Autobahn certificate verifiers and persistence code. The classic models do not establish Autobahn safety, time or recovery properties. |

## Autobahn protocol and implementation map

### Block identity and time

[`BlockHeader`](../autobahn/types/block.go) commits to a lane, lane-local block
number, parent header hash and payload hash. A `GlobalBlock` associates that lane
header and payload with its position in the finalized global order and a
timestamp. Global height and lane-local height are distinct numbers.

The execution adapter passes the **lane-header hash** as
`RequestFinalizeBlock.Hash`, including as the block hash used by EVM execution.
A Tendermint-shaped RPC header is an adapter view: hashing that view does not
reconstruct the Autobahn block identity. An empty `DecidedLastCommit` in this
adapter is intentional; finality is represented by Autobahn certificates.

[`Proposal.BlockTimestamp`](../autobahn/types/proposal.go) derives block timestamps
from the consensus proposal and position in its global range, with microsecond
increments. Proposal construction and verification enforce the corresponding
ordering and time bounds. The inherited BFT-time and proposer-based timestamp
research pages do not define this clock contract.

### Committee, quorum and certificates

[`NewGigaRouter`](../internal/p2p/giga_router.go) constructs a round-robin committee
from the configured validator keys, genesis chain ID, initial height and genesis
time. [`NewRoundRobinElection`](../autobahn/types/committee.go) assigns equal
weight to these keys. The general committee type also supports weights.

For total committee weight `W`, the implementation defines
`F = floor((W - 1) / 3)`. Commit, prepare, timeout and application certificates
require weight `W - F`; lane certificates require `F + 1`. Certificate verification
checks distinct signers, membership, signatures and the relevant weight threshold.
Signatures are bound to the verifier's network and committee context in
[`msg.go`](../autobahn/types/msg.go).

- [`LaneQC`](../autobahn/types/lane_qc.go) certifies a lane proposal.
- [`PrepareQC`](../autobahn/types/prepare_qc.go) and
  [`CommitQC`](../autobahn/types/commit_qc.go) carry the agreement evidence;
  `CommitQC.GlobalRange` identifies the finalized range.
- [`FullCommitQC`](../autobahn/types/commit_qc.go) includes the headers for that
  range and verifies their agreement with the certificate.
- [`AppQC`](../autobahn/types/app_qc.go) certifies an application-state proposal;
  it is distinct from the certificate that orders blocks.

### Peer transport and limits

Autobahn uses the node's internal Giga peer transport. Its stream declarations,
message byte limits, windows and concurrency limits are in
[`giga/api.go`](../internal/p2p/giga/api.go); the wire contract is
[`giga/api.proto`](../internal/p2p/giga/api.proto). These are peer streams, not
public JSON-RPC methods.

The block types bound payloads to 2,000 transactions and 2,048,000 aggregate
transaction bytes (`MaxTxsPerBlock` and `MaxTxsBytesPerBlock`). The proposal and
block-fetch streams allow `MaxBlockProtoSize + 10 KiB` messages; full commit
certificate streams allow 100 KiB. These transport/type bounds are separate from
the producer's configured transaction and gas budgets. Consult the constants
and stream registrations when changing either side of the transport.

### Application state and durable restart

[`executeBlock`](../internal/p2p/giga_router.go) saves the `FinalizeBlock` response,
calls application `Commit`, then calls `PushAppHash` before publishing execution
events. [`data.State`](../internal/autobahn/data/state.go) makes `PushAppHash` wait
for the block and certificate to be durably persisted before application votes
can be issued. Results remain in transaction order.

The JSON `persistent_state_dir` setting supplies the shared root for consensus
and data persistence. [`config/autobahn.go`](../config/autobahn.go) requires a
nonblank directory setting. Omitting it is permitted only with the explicit
`unsafe_test_only_disable_persistence` option for a throwaway test network.
The two settings are mutually exclusive. A production restart requires
both the application's committed store and the Autobahn safety/data records.

[`consensus/inner.go`](../internal/autobahn/consensus/inner.go) describes the
alternating `inner_a.pb`/`inner_b.pb` safety-state files, saved votes and view
justifications. The indexed persistence code lives in
[`consensus/persist`](../internal/autobahn/consensus/persist/). The data layer
restores global blocks and full commit certificates from its WALs and checks
cursor consistency. Recovery does not use the classic consensus WAL algorithm.

At startup, `runExecute` reads `Info.LastBlockHeight` and `LastBlockAppHash`. At
height zero it calls `InitChain` and begins at the genesis initial height. On
restart after a commit it reports the last application hash and resumes at the
next global height. Application `Commit` must therefore persist state before
returning, and `Info` must describe that persisted state. Application
`RetainHeight` controls pruning of Autobahn data and saved execution responses;
pruned history is not an archive guarantee.

## Inherited specifications

These links retain their existing site locations. Treat them as classic protocol
references or historical design material, with the applicability limits above.

- [Application Blockchain Interface drafts](./abci++/README.md)
- [Application requirements and archived draft](./abci++/abci++_app_requirements_002_draft.md)
- [Encoding and Digests](./core/encoding.md)
- [Core Data Structures](./core/data_structures.md)
- [State](./core/state.md)
- [Consensus Algorithm](./consensus/consensus.md)
- [Creating a proposal](./consensus/creating-proposal.md)
- [Time research](./consensus/proposer-based-timestamp/README.md)
- [Classic Light Client](./consensus/light-client/README.md)
- [The Base P2P Layer](./p2p/node.md)
- [Peer Exchange (PEX)](./p2p/messages/pex.md)
- [Consensus Remote Procedure Calls](./rpc/README.md)
- [Classic Write-Ahead Log](./consensus/wal.md)
- [Ivy Proofs](./ivy-proofs/README.md)

## Contributing

Use Go for implementation examples and link claims to their implementing types
and call sites. State which engine mode a protocol change affects. Keep
historical models labeled with their assumptions rather than treating them as a
proof of the current fork.

Proposals can be discussed in the
[repository](https://github.com/Sidiora-Labs/paxeer-monorepo). Report
security-sensitive discrepancies through the repository
[security policy](../../SECURITY.md).
