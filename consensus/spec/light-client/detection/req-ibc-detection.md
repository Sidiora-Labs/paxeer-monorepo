<!-- markdown-link-check-disable -->

# Requirements for Fork Detection in the IBC Context

## Status and applicability

This page retains the useful requirements from the upstream Tendermint IBC
fork-detection discussion and maps them to this tree. Its trust, timing and
validator assumptions concern classic Tendermint signed headers. They do not
establish verification of Autobahn certificates or recovery of an Autobahn node.
The [supervisor](../supervisor/supervisor_001_draft.md) describes that boundary;
[initialization](../supervisor/supervisor_002_draft.md) explains the actual Go API.

Paxeer mainnet-beta is one BFT network run by `paxd`, with the kernel participating
in the network and proposing blocks. A receipt is final when its block is
committed. The EVM chain ID is 125 (`0x7d`). Sub-chains are tenant domains in the
database; they do not imply a new consensus network or a new IBC light client.
IBC client state describes the counterparty chain being verified.

The public Ethereum JSON-RPC uses the root URL of
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network` and
`https://api-mainnet-beta.paxeer.network`. An Ethereum JSON-RPC response does not
supply a Tendermint signed header and validator set. The HTTP provider in
[`consensus/light`](../../../light/provider/http/http.go) instead needs the
consensus `Commit` and `Validators` methods on an appropriate consensus endpoint.
In Autobahn mode, [`Commit`](../../../internal/rpc/core/blocks.go) returns
`ErrCommitNotAvailable`; the provider treats that as `ErrUnreliableProvider`.
There is no certificate conversion in this classic client.

## What you need to know about IBC

### Components and their interface

#### Tendermint Blockchains

The classic light-client model starts from a subjectively trusted checkpoint.
During its trusting period, sufficient voting power in the trusted validator
set must remain honest. Forward verification checks time, chain identity,
validator-set hashes and commit signatures. Non-adjacent verification also needs
the configured trust-level overlap with the trusted set. Witnesses supply an
independent view for detecting a primary that presents a conflicting history.

These are assumptions of the retained Tendermint model. They are not inferred
from the EVM chain ID, a successful endpoint query or an Autobahn quorum.

#### An IBC/Tendermint correspondence

The concrete IBC types live in
[`interchain/modules/light-clients/07-tendermint/types`](../../../../interchain/modules/light-clients/07-tendermint/types/).
The table replaces the draft's proposed one-to-one correspondence with the
representations actually used here.

| IBC concept | Representation in this tree | Consequence |
| --- | --- | --- |
| Commitment root | `ConsensusState.Root`, derived from header `AppHash` | An application commitment, not an entire light block |
| Consensus state | `Timestamp`, `Root`, `NextValidatorsHash` | The next validator set is committed by hash, not stored in full here |
| Client state | `ClientState` with chain identity, trust and timing parameters, latest and frozen heights | Different from the off-chain client's trusted block store |
| Height | Revision number and revision height | Compare both components; it is not just a Go `int64` block height |
| Header | `Header` with signed header, validator set, `TrustedHeight` and `TrustedValidators` | The supplied trusted validators must match the stored consensus state's next-validator hash |
| Misbehaviour | `Misbehaviour` with `ClientId`, `Header1` and `Header2` | The handler verifies both headers against their respective trusted states |
| Frozen height | `FrozenHeight = NewHeight(0, 1)` | Used as a nonzero frozen marker, not the height where a fork began |

The [`header.go`](../../../../interchain/modules/light-clients/07-tendermint/types/header.go),
[`consensus_state.go`](../../../../interchain/modules/light-clients/07-tendermint/types/consensus_state.go)
and [`misbehaviour.go`](../../../../interchain/modules/light-clients/07-tendermint/types/misbehaviour.go)
implement these representations. The upstream
[ICS-002 client semantics](https://github.com/cosmos/ibc/tree/main/spec/core/ics-002-client-semantics)
provides background; the local types and handlers determine this fork's behavior.

#### Required Changes in ICS 007

The original draft proposed changes to upstream ICS-007. Those proposals are
historical design questions, not instructions to change this tree's wire types.
In particular, the local handler does not accept the draft
`LightNodeProofOfFork` or a generic list of light blocks as misbehaviour.

[`CheckHeaderAndUpdateState`](../../../../interchain/modules/light-clients/07-tendermint/types/update.go)
verifies an update against the consensus state at `TrustedHeight`. It can fill
an older height skipped by earlier updates, as well as advance the latest
height; the submitted header must be newer than its own trusted height and in
the same revision. It does not run an off-chain provider/bisection loop.
A valid conflicting consensus state at an existing height, or a violation of
time monotonicity with neighboring consensus states, freezes the client.

[`CheckMisbehaviourAndUpdateState`](../../../../interchain/modules/light-clients/07-tendermint/types/misbehaviour_handle.go)
checks two headers. Equal-height headers must have different block hashes;
unequal-height headers must violate monotonic time. Each needs a stored trusted
consensus state and valid trusted-validator overlap within the trusting period.
Success returns a client state with the nonzero frozen marker.

#### Handler

The handler verifies relayer input against its own stored client and consensus
states. It cannot adopt the relayer's off-chain trust decision. The local
methods are `CheckHeaderAndUpdateState` and `CheckMisbehaviourAndUpdateState`,
with SDK context, codec and client store arguments; the original draft's
`checkValidityAndUpdateState(Header)` and byte-only evidence functions were
conceptual interfaces, not Go API signatures.

Historical consensus states matter: the misbehaviour handler loads the trusted
height for each submitted header. A relayer must use states still available to
the handler and account for expiry and pruning rather than assume that a query
of the latest state is sufficient.

#### Relayer

In the retained model, a relayer runs a light client for each counterparty it
needs to follow, queries the receiving chain's client state and consensus
states, and submits updates or misbehaviour. Multiple relayers can submit to a
handler; none is trusted solely because it delivered a message. Detection
liveness assumes that a correct relayer can observe and report a conflict in
time. This page specifies requirements for that behavior, not a deployed
relayer service or an automatic reporting schedule.

## Informal Problem Statement: Fork detection in IBC

### Relayer requirement: Evidence for Handler

A relayer that detects conflicting histories should construct evidence the
receiving handler can verify from its own trusted consensus states. Its local
verification trace may start at a different checkpoint. It may therefore need
to submit intermediate header updates before submitting the two-header
misbehaviour object. A local trace is not itself the handler's evidence format.

### Relayer requirement: Fork detection

There are two useful observations to retain from the research draft:

1. The relayer's own light client can detect a conflict between its providers.
2. The relayer can discover that a handler's stored consensus state conflicts
   with the history it has independently verified.

These observations require different follow-up work. Reporting to a source
full node and reporting to an IBC handler are separate operations with separate
validation rules. A report to one does not freeze the other.

### Handler requirement

The handler must validate signatures, chain and revision identity, trusted
validator commitments and timing for the submitted evidence. It must reject an
unverifiable accusation. A valid conflicting update or verified misbehaviour
freezes this IBC client; it does not roll back committed Paxeer receipts or halt
the whole network.

### Challenges in the handler requirement

Relayer and handler stores need not share every height. Checkpoints expire and
stores can be pruned. The relayer must preserve enough history to construct
verifiable updates and report before the handler's relevant trusted states
expire. For classic Tendermint, the trusting period plus time for checking,
reporting and punishment must fit within the unbonding period; this page does
not prescribe a network-specific duration.

If a relayer changes its trusted checkpoint, it must reassess the handlers it
follows. The retained requirement is a timely comparison against the handler's
actual state, not a guarantee that merely polling a provider detects all forks.

## On the interconnectedness of things

### Fork detection

The [classic detector model](detection_003_reviewed.md) distinguishes a fork of
the chain from an attack on a light client using past validator power. The
implementation in [`light/detector.go`](../../../light/detector.go) cross-checks
the verified target against witnesses and examines divergent verification
traces. A differing hash alone is not a validated accusation of validator
misbehaviour.

### Evidence creation and submission

Keep the three interfaces distinct:

| Recipient | Material | Responsibility |
| --- | --- | --- |
| Off-chain light client | Provider light blocks and verification traces | Verify and cross-check the target before trusting it |
| IBC handler | The local `Header` and `Misbehaviour` types | Verify against stored counterparty consensus states and freeze that client when warranted |
| Classic full node | `types.LightClientAttackEvidence` | Validate the evidence against chain history and determine accountable validators |

[`LightClientAttackEvidence`](../../../types/evidence.go) includes a conflicting
block, common height, Byzantine validators, total voting power and timestamp.
The Go detector reports it through `provider.Provider.ReportEvidence` on a
best-effort basis. Reporting errors are logged; `ErrLightClientAttack` does not
prove that a recipient persisted evidence or applied punishment.

### Isolating misbehaving nodes

Attributing an attack to validators is separate from finding conflicting
histories. The classic evidence code uses validator membership, voting power
and commits to identify accountable validators. The upstream discussion of
lunatic, double-signing, amnesic and phantom validators remains background in
[the attack research](../attacks/isolate-attackers_002_reviewed.md). It is not an
Autobahn slashing or recovery specification.

## Next steps

An Autobahn-aware IBC client needs an explicit certificate format, committee
trust and transition rules, application-root binding, misbehaviour rules and
restart behavior before these classic requirements can be applied to that
mode. [`CommitQC` and `FullCommitQC`](../../../autobahn/types/commit_qc.go) verify
against a [`Committee`](../../../autobahn/types/committee.go); they are not
Tendermint `SignedHeader` commits. See the
[supervisor applicability map](../supervisor/supervisor_001_draft.md#status)
for the current source boundary. This page makes no claim that those missing
protocol mappings have been implemented.
