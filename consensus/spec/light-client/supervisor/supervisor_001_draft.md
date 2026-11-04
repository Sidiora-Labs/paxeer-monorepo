<!-- markdown-link-check-disable -->

# Draft of Light Client Supervisor for discussion

## TODOs

The upstream draft combined a sequential research model with proposed Go-like
pseudocode. This revision retains the model's useful requirements and labels
their implementation boundary. An Autobahn light-client protocol and its IBC
mapping remain separate work; the classic model below does not supply them.

# Light Client Sequential Supervisor

A classic light client reads headers through a primary provider and independent
witnesses. It verifies a target against trusted state, cross-checks it, then
stores the trusted result. The application supplies the initial trust anchor;
agreement among untrusted providers cannot establish that anchor by itself.

Paxeer mainnet-beta is one BFT network run by `paxd`. The kernel is part of that
network and proposes blocks. A receipt is final once its block is committed;
light-client verification is how an observer checks a history, not another
finality stage. The EVM chain ID is 125 (`0x7d`). Sub-chains are tenant domains
in the database, not an instruction to run another consensus service.

# Status

This file is an inherited Tendermint research specification, revised to explain
the current Go implementation. Its safety and liveness claims are conditional
on the classic trust, validator and timing assumptions. It is not an Autobahn
certificate verifier, an Autobahn restart procedure or a qualification result.

| Topic | Authoritative code in this tree | Applicability |
| --- | --- | --- |
| Off-chain classic client | [`light/client.go`](../../../light/client.go), [`light/verifier.go`](../../../light/verifier.go), [`light/detector.go`](../../../light/detector.go) | Tendermint signed headers and validator sets |
| Initialization and persistence | [`light/trust_options.go`](../../../light/trust_options.go), [`light/store/store.go`](../../../light/store/store.go) | Trusted height/hash or an existing trusted store; see [initialization](supervisor_002_draft.md) |
| Consensus mode selection | [`config/config.go`](../../../config/config.go), [`node/node.go`](../../../node/node.go) | A nonempty `autobahn-config-file` selects Autobahn; empty disables it |
| Autobahn certificates | [`autobahn/types/commit_qc.go`](../../../autobahn/types/commit_qc.go), [`autobahn/types/committee.go`](../../../autobahn/types/committee.go) | Committee-bound certificates, lane ranges and finalized global block ranges |
| Autobahn recovery | [`internal/autobahn/consensus/state.go`](../../../internal/autobahn/consensus/state.go), [`persisted_inner.go`](../../../internal/autobahn/consensus/persisted_inner.go), [`persist/`](../../../internal/autobahn/consensus/persist/) | Persisted votes, certificates and indexed block data; not the classic trusted store |
| IBC counterparty client | [`interchain/modules/light-clients/07-tendermint/types`](../../../../interchain/modules/light-clients/07-tendermint/types/) | Separate on-chain client state; see [IBC fork detection](../detection/req-ibc-detection.md) |

The classic HTTP provider fetches `Commit` and `Validators` from a consensus
endpoint. In Autobahn mode, the consensus
[`Commit` handler](../../../internal/rpc/core/blocks.go) returns
`ErrCommitNotAvailable`, and the
[provider](../../../light/provider/http/http.go) maps that to
`ErrUnreliableProvider`. This is an explicit compatibility boundary. Matching
an RPC block height does not make an Autobahn certificate a Tendermint commit.

The public Ethereum JSON-RPC is at the root of the node URLs
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`, plus
`https://api-mainnet-beta.paxeer.network`. Those EVM responses are not a
replacement for the consensus provider contract. The Go client's `chainID`
argument is the consensus header's chain identity, not the hexadecimal value
returned by `eth_chainId`.

# Part I - Tendermint Blockchain

The [verification research](../verification/verification_001_published.md) and
[detector research](../detection/detection_003_reviewed.md) describe the classic
model. Validators may change between heights. The root of trust is subjective,
and enough trusted voting power must stay honest throughout the trusting period.
The trusting period must leave time for verification, reporting and punishment
before unbonding completes. At least one correct, reachable provider is needed
for the detector's security argument; providers must be sufficiently timely
for progress.

[`VerifyAdjacent`](../../../light/verifier.go) binds the next height's validator
hash to the trusted header's `NextValidatorsHash` and checks signatures from
more than two-thirds of the new validator set. `VerifyNonAdjacent` additionally
checks the configured trusted-set overlap, defaulting to one-third, as well as
more than two-thirds of the new set. Insufficient overlap triggers intermediate
verification rather than acceptance of an unsupported target.

Forward verification rejects a trusted header when
`trusted.Time + trustingPeriod <= now`. It requires increasing height and time,
and a new timestamp strictly before `now + maxClockDrift`. These are classic
verification rules, not claims about Autobahn certificate time or committee
transitions.

# Part II - Sequential Problem Definition

#### **[LC-SEQ-INIT-LIVE.1]**

The research goal is to establish an initial trusted header from user-supplied
trust. The actual Go constructors accept an existing trusted store or a trusted
height/hash with a trusting period. They do not construct a light block from a
genesis document. See [the initialization mapping](supervisor_002_draft.md).

#### **[LC-SEQ-LIVE.1]**

Given a requested target height, sufficient trusted history, timely providers
and no detected attack, the client should obtain and store the verified target.
The implementation returns errors for expired trust, missing history, provider
failures and failed cross-checking; it does not promise unconditional progress.

#### **[LC-SEQ-SAFE.1]**

The retained safety goal is to expose only headers justified by the trusted
checkpoint and verification rules. It depends on the model assumptions and is
not a proof for a different consensus mode.

# Part III - Light Client as Distributed System

## Computational Model

[`provider.Provider`](../../../light/provider/provider.go) supplies
`LightBlock(ctx, height)`, `ReportEvidence(ctx, evidence)` and `ID()`.
Provider height zero requests the latest block. A provider response is
untrusted input; the client validates and verifies it.

## Distributed Problem Statement

### Two Kinds of Liveness

#### **[LC-DIST-TERM.1]**

The draft's “terminate on attack” means that a caller must stop relying on the
unverified target after an attack is detected. The library returns an error;
it does not terminate `paxd`, halt the chain or run an infinite supervisor loop.

### Design choices

#### **[LC-DIST-STORE.1]**

The research `LightStore` corresponds only conceptually to the concrete
`store.Store`. The implementation stores trusted target light blocks. The
intermediate blocks used for forward verification remain in the verification
trace and are not all saved to the trusted database. There is no persisted
`StateVerified`/`StateTrusted` enum or verification-root field in this API.

#### **[LC-DIST-SAFE.1]**

A successful forward verification is followed by witness detection before
`updateTrustedLightBlock` saves the target. A failed verification or detector
result must not be treated as a trusted read. In particular,
`VerifyLightBlockAtHeight` can return a block together with an error; callers
must check the error before using the block.

#### **[LC-DIST-LIVE.1]**

The client may successfully verify a target or report a failure requiring caller
action. An attack elsewhere need not be observed by this client's providers.
Failure to obtain a witness match is an availability failure, not proof that
all providers supplied the correct chain or proof of a punishable attack.

### Solving the sequential specification

Verification and detection are complementary under the classic assumptions.
[`detectDivergence`](../../../light/detector.go) checks witnesses against the
primary trace. It can accept a matching witness, examine a conflicting trace,
remove bad witnesses, or return a cross-reference or attack error. This source
mapping does not extend the draft's safety argument to Autobahn.

# Part IV - Light Client Supervisor Protocol

The sequential model is: obtain a trusted root, fetch a target, verify it,
cross-check it and save the target only after success. The current Go API uses
method calls rather than the draft's input/output event loop. Forward
verification traces and backward hash verification have different mechanics.

## Definitions

### Peers

#### **[LC-DATA-PEERS.1]:**

`Client` has one `primary`, a `witnesses` slice and an in-memory blacklist with
`blacklistTTL`. It does not maintain the draft's additional `FullNodes` and
`FaultyNodes` sets. Both constructors require at least one witness, including
when sequential verification is selected.

#### **[LC-INV-NODES.1]:**

The research partition into primary, secondaries, reserves and faulty nodes
expresses the need for independent sources and safe replacement. It is not the
current data layout or a guarantee of a fixed witness count. `AddProvider`,
provider removal and primary replacement operate on the implementation's
provider list.

#### **[LC-FUNC-REPLACE-PRIMARY.1]:**

The draft's `Replace_Primary` maps conceptually to `findNewPrimary` in
[`client.go`](../../../light/client.go). Replacement uses witnesses and can
fail when providers are exhausted. Verification paths check that a replacement
provider's target matches the original target before continuing.

#### **[LC-FUNC-REPLACE-SECONDARY.1]:**

There is no exported `Replace_Secondary` API or automatic replenishment to a
fixed secondary count. The client removes bad witnesses and can blacklist them;
callers can supply additional providers through `AddProvider`.

### Data Types

#### **[LC-DATA-LIGHTBLOCK.1]**

The actual [`types.LightBlock`](../../../types/light.go) embeds a
`*SignedHeader` and contains a `*ValidatorSet`. `SignedHeader` embeds `*Header`
and contains `*Commit`. The header carries `NextValidatorsHash`; the draft's
explicit `NextValidators` and `Provider` fields are not part of `LightBlock`.
`ValidateBasic` checks internal consistency, including the validator hash and
commit/header binding, but does not verify cryptographic signatures.

#### **[LC-DATA-LIGHTSTORE.1]**

[`store.Store`](../../../light/store/store.go) exposes `SaveLightBlock`,
`DeleteLightBlock`, `LightBlock`, `LightBlockBefore`, first/last height queries,
`Prune` and `Size`. The database implementation is in
[`light/store/db`](../../../light/store/db/db.go). Missing blocks and pruning
must be handled by the caller; the store is not a complete chain archive.

### Inputs

#### **[LC-DATA-INIT.1]**

The historical `LCInitData` union is a research notation, not a declared Go
input type. `NewClient` takes `TrustOptions`, providers, a store and
`blacklistTTL`; `NewClientFromTrustedStore` takes a trusting period and the
existing store. Both accept functional options.

#### **[LC-DATA-GENESIS.1]**

A genesis document belongs to full-node initialization. It is not an accepted
fallback input for these light-client constructors.

#### **[LC-FUNC-MAKEBLOCK.1]**

The draft's `makeblock` proposal is not an implemented light-client function.
Constructing an unsigned synthetic genesis light block does not provide the
commit required by the concrete `LightBlock` validation and verification path.

### Configuration Parameters

The functional options are `SequentialVerification`, `SkippingVerification`,
`PruningSize`, `MaxClockDrift` and `MaxBlockLag`. Defaults in `client.go` are
skipping verification with `DefaultTrustLevel` (one-third), pruning size 1000,
clock drift 10 seconds and block lag 10 seconds. Trust levels satisfy
`1/3 <= level < 1`. `PruningSize(0)` disables automatic pruning on update;
`Cleanup()` explicitly removes the trusted blocks. The caller supplies the
trusting period and blacklist TTL; these are not public endpoint parameters.

#### **[LC-INV-ROOT-AGREED.1]**

Agreement by all providers on every latest trusted block was a draft invariant.
The implementation instead compares witnesses during fresh initialization and
forward verification. Restoring a store does not perform the initial provider
comparison, and ordinary forward detection requires a witness match after
processing responses, not unanimous availability of every provider.

### Assumptions

The operator must authenticate the initial consensus chain identity, height and
hash, select a suitable trusting period and choose independent providers. A
persisted trusted store is itself trusted input. Provider agreement is a
consistency check, not a substitute for these decisions.

### Invariants

#### **[LC-INV-PEERLIST.1]:**

At least one witness is required. `ErrNoWitnesses` reports exhaustion of that
requirement. It is a library error for the caller to handle, not an instruction
to terminate the network.

## Supervisor

### Outline

The actual method correspondence is:

| Research operation | Go implementation |
| --- | --- |
| `InitLightClient` | `NewClient` or `NewClientFromTrustedStore` |
| Request a target | `VerifyLightBlockAtHeight(ctx, height, now)`; height must be positive |
| Verify a supplied header | `VerifyHeader(ctx, header, now)` |
| Advance to provider head | `Update(ctx, now)` |
| Read stored trust | `TrustedLightBlock(height)`; zero means latest |
| Inspect stored height range | `FirstTrustedHeight()` and `LastTrustedHeight()` |

#### **[LC-FUNC-SUPERVISOR.1]:**

There is no exported `Sequential-Supervisor` function. The caller chooses when
to invoke these methods and what to do on failure. The library reports classic
attack evidence through its providers, independently of IBC misbehaviour
submission. See [the IBC requirements](../detection/req-ibc-detection.md).

### Details of the Functions

#### Initialization

#### **[LC-FUNC-INIT.1]:**

See [initialization and restart](supervisor_002_draft.md) for the constructor
order, checkpoint checks, witness comparison and expiry behavior. The draft's
choice between an initial light block and synthetic genesis block is historical.

#### Main verification and detection logic

#### **[LC-FUNC-MAIN-VERIF-DETECT.1]:**

For an already stored target, `VerifyLightBlockAtHeight` returns the trusted
block. `VerifyHeader` additionally checks that a supplied header has the same
hash. For a new target, the client fetches the light block and selects a trusted
root according to height:

1. At or above the latest trusted height, verify forward from the latest block.
2. Below the first trusted height, follow `LastBlockID` hashes backward from the
   earliest trusted block, with at most 10,000 backward steps per call.
3. Between stored heights, verify forward from the closest prior trusted block.

Forward verification uses sequential adjacent checks or skipping verification
with intermediate pivots when overlap is insufficient. Both forward modes run
the detector against the resulting trace. Backward verification checks adjacent
header hashes, identity and time order; it does not run the witness detector or
the forward trusting-period check. After success, the target is saved and the
configured pruning policy is applied.

[`light/errors.go`](../../../light/errors.go) defines `ErrVerificationFailed`
(with an unwrap-able reason), `ErrOldHeaderExpired`, `ErrInvalidHeader`,
`ErrFailedHeaderCrossReferencing`, `ErrNoWitnesses`, `ErrConflictingHeaders`,
`ErrProposerPrioritiesDiverge` and `ErrLightClientAttack`. These are Go errors,
not numeric EVM JSON-RPC error codes. A validated conflicting trace causes
best-effort `ReportEvidence` calls and an attack error. Failed evidence delivery
is logged, so the error alone is not proof of successful evidence persistence
or punishment. The caller must handle the result before exposing a target.
