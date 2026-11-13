<!-- Source: https://docs.paxeer.app/consensus/ -->

# Consensus

Consensus service wiring, application execution, trusted headers, and validator signing state.

## Overview

Paxeer includes a Tendermint-derived consensus stack and Autobahn components. Consensus orders and verifies blocks; the application executes transactions and commits state. Network safety depends on the protocol's validator and trust assumptions, while observed block time and throughput depend on the deployed configuration.

## Autobahn and Validator Committees

Autobahn uses application and lane proposals, votes, quorum certificates, committees, and timeouts. Their types are defined under `consensus/autobahn/types/`; the protocol state is coordinated under `consensus/internal/autobahn/`.

Use the deployment's Autobahn configuration to inspect its lane and committee settings. Track throughput and block intervals through node metrics.

**Source:** `consensus/autobahn/types/committee.go`, `consensus/autobahn/types/lane_proposal.go`, `consensus/autobahn/types/commit_qc.go`, `consensus/config/config.go`.

## Application Execution

The node application registers `ProcessProposalHandler` and `FinalizeBlocker`. A proposal may begin optimistic work before finalization; finalized execution and successful commit establish the application result. Mempool acceptance and a returned submission hash are earlier stages.

Transaction execution can use the standard scheduler or the configured Giga executor. The standard scheduler records multiversion accesses, validates dependencies, and retries conflicting tasks. After 10 iterations it reverts to sequential processing of remaining unvalidated work.

Consensus establishes transaction ordering, while the execution scheduler manages parallel work and dependency validation.

**Source:** `node/app.go` — `ProcessProposalHandler`, `FinalizeBlocker`, `ExecuteTxsConcurrently`; `sdk/tasks/scheduler.go` — `ProcessAll`.

## Node Services

The `consensus/node/` package assembles peer networking, block and state stores, mempool, evidence handling, RPC, indexing, and application connections. Three node modes enable different services: `full`, `validator`, and `seed`. Any other mode value is rejected; there is no archive mode.

Sequencer mode cannot run with an `autobahn-config-file`; the configuration rejects that combination.

See [Configuration](https://docs.paxeer.app/configuration) for node mode defaults and [Run a Node](https://docs.paxeer.app/run-node) for initialization.

## State and Synchronization

Consensus state records validator sets, block identity, consensus parameters, and application commitment. State sync uses a trusted checkpoint and application snapshots to bootstrap a node without replaying all earlier blocks.

Configure application snapshot production on provider nodes and state-sync consumption on joining nodes. Use the [storage configuration](https://docs.paxeer.app/storage) to retain the data and commitment history needed for queries and proofs.

**Source:** `consensus/config/config.go` — `StateSyncConfig`; `consensus/internal/statesync/`; `consensus/internal/state/`.

## Light Client

The light client checks new headers against a trusted header, validator set, trust period, and clock-drift rules. Non-adjacent verification checks the configured trusted-set overlap and requires more than two-thirds of the new validator voting power to sign the new header.

Maintain a trusted checkpoint within the configured trust period. For historical application proofs, connect to a node retaining the corresponding commitment state.

**Source:** `consensus/light/verifier.go` — `VerifyAdjacent`, `VerifyNonAdjacent`; `consensus/light/client.go`.

## Validator Signing

File-based validators persist a signing key and a mutable last-sign state containing height, round, step, and signature data. Recovery must preserve both. Never run two active signers using the same validator key.

Autobahn requires a local validator key and rejects remote signers. Do not configure a remote signer through the node's `[priv-validator]` configuration for an Autobahn validator.

Equivocation handling can permanently tombstone a validator. Query the connected network's slashing parameters for its configured monetary penalties.

**Source:** `consensus/privval/file.go`, `consensus/node/setup.go` — `createPrivval`; `sdk/x/evidence/keeper/infraction.go`.

## RPC and Metrics

```
curl -sS http://127.0.0.1:26657/status
curl -sS http://127.0.0.1:26657/net_info
curl -sS http://127.0.0.1:26657/consensus_params
```

Consensus RPC and EVM JSON-RPC are separate listeners. The consensus metrics include height, validator last-signed height, missed-block gauges, block/state-sync state, and block intervals. Use the series emitted by your configured namespace when constructing alerts.

**Source:** `consensus/internal/rpc/core/routes.go`; `consensus/internal/consensus/metrics.go`.

## Next Steps

- [EVM execution engine](https://docs.paxeer.app/engine)

- [Validator operations](https://docs.paxeer.app/operators)

- [Network parameters](https://docs.paxeer.app/network-parameters)
