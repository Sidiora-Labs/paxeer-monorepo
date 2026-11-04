# Evidence

Paxeer is one BFT network, run by `paxd`, with EVM chain ID 125. Its kernel
is part of the network and proposes blocks. A receipt is final when its block
is committed. Evidence records validator misbehavior; it does not add another
finality step.

This page describes the classic consensus evidence path in the embedded
Tendermint-derived engine. The protocol tolerates Byzantine voting power below
one third. A single validator can equivocate and produce evidence even when
that threshold is not exceeded. Evidence supports detection, gossip and
application policy; it does not itself slash validators or guarantee inclusion
when Byzantine power can censor blocks.

## Runtime applicability

The node setting `autobahn-config-file` selects the consensus implementation:
an empty value selects classic consensus; a nonempty value enables Autobahn.
See [node assembly](../../node/node.go) and
[configuration](../../config/config.go).

The detection, verification, gossip and block-inclusion rules below describe
classic `DuplicateVoteEvidence` and `LightClientAttackEvidence`. Autobahn uses
its own certificates. Its application execution path currently leaves
`RequestFinalizeBlock.ByzantineValidators` empty, and the consensus engine's
`broadcast_evidence` method rejects submissions with
`coretypes.ErrEvidenceNotAccepted` ("evidence is not accepted under autobahn
consensus"). See the [submission handler](../../internal/rpc/core/evidence.go)
and [Autobahn application execution](../../internal/p2p/giga_router.go).
These classic proofs are not an Autobahn evidence or slashing specification.

Evidence here is consensus evidence, distinct from application-defined proofs.
The [timestamp applicability map](./proposer-based-timestamp/README.md#current-runtime)
explains the time rules used by each implementation.

## Detection

### Equivocation

A validator equivocates by signing conflicting votes for the same height,
round and vote type. Classic consensus reports a conflicting pair through
`ReportConflictingVotes`; the evidence pool buffers the pair and constructs
`DuplicateVoteEvidence` during its update, when the corresponding block time
and validator set are available. The votes need not arrive from the same peer.
See [consensus vote handling](../../internal/consensus/state.go) and the
[evidence pool](../../internal/evidence/pool.go).

The proof contains `VoteA` and `VoteB` together with the validator power, total
voting power and block timestamp needed by the application. Its full Go type
is defined in [types/evidence.go](../../types/evidence.go).
[Verification](#duplicatevoteevidence) checks both signatures and their conflict.

### Light Client Attacks

Classic light clients can detect conflicting signed headers by comparing
responses from multiple providers. The retained upstream
[detection design](../light-client/detection/detection_003_reviewed.md) and
[verification design](../light-client/verification/verification_001_published.md)
explain that model and its trust assumptions.

`LightClientAttackEvidence` contains a `ConflictingBlock *LightBlock` and a
`CommonHeight`, together with the Byzantine validator list, total voting power
and timestamp for application delivery. A conflicting light block must still
pass [full-node verification](#lightclientattackevidence); comparison alone
is not sufficient proof. This classic light-client format does not establish
validity of Autobahn certificates.

## Verification

The [pool](../../internal/evidence/pool.go) verifies received evidence before
adding it to its persistent pending store. Inclusion in a proposed block is
checked by `CheckEvidence`, including rejection of already committed evidence
and duplicate hashes within the same block. Pending duplicate-vote proofs can
reuse earlier verification; light-client attack evidence is reverified even
when pending, because another conflicting block can share its evidence hash.

Evidence expires only when **both** age limits are exceeded:

```text
currentHeight - evidenceHeight > EvidenceParams.MaxAgeNumBlocks
AND
currentBlockTime - evidenceBlockTime > EvidenceParams.MaxAgeDuration
```

The evidence height is the vote height for duplicate votes and `CommonHeight`
for a light-client attack. Verification loads that height's block metadata to
obtain its time rather than trusting a supplied timestamp. Missing historical
blocks or validator sets can prevent verification. Applications that use
bonding and slashing must choose evidence retention and unbonding policies
that allow valid evidence to remain actionable. See
[verification](../../internal/evidence/verify.go) and
[consensus parameters](../../types/params.go).

### DuplicateVoteEvidence

`VerifyDuplicateVote` requires:

- Equal validator addresses, heights, rounds and vote types.
- Different block IDs; a vote for a nil block can be part of the conflict.
- Membership in the historical validator set at the evidence height.
- Two valid signatures under that validator's public key, using the chain's
  consensus `ChainID` in the vote sign bytes.

The consensus `ChainID` used for signing is distinct from the numeric EVM
chain ID 125.

### LightClientAttackEvidence

`VerifyLightClientAttack` checks the conflicting block against locally trusted
headers and historical validator sets:

- When the conflicting height differs from the common height, signatures must
  satisfy the common validator set's trusting verification threshold.
- When the heights match, the conflicting header must have the expected
  deterministic header fields.
- More than two thirds of the conflicting validator set's voting power must
  sign its commit; verification checks all supplied signatures.
- The conflicting and trusted header hashes must differ.
- For a conflicting block ahead of the local trusted height, its time must
  not be later than the trusted header's time, demonstrating a violation of
  monotonic time.

The pool separately validates the application metadata derived from these
proofs. If the proof is valid but that metadata is inconsistent, it regenerates
and stores corrected metadata and returns the validation error for the
submitted form; it does not accept arbitrary validator powers or times.

## Gossiping

The [evidence reactor](../../internal/evidence/reactor.go) sends pending proofs
one at a time on the classic evidence channel. Each peer's broadcast loop
restarts from the beginning of the pending list after the
`broadcastEvidenceInterval` of 60 seconds, or when its current list element is
removed. This is a list-rescan interval, not a delivery guarantee for every
proof. Committed or expired proofs are removed from the pending list.

<a id="commiting-on-chain"></a>

## Committing on Chain

`BlockExecutor.CreateProposalBlock` requests pending evidence up to
`EvidenceParams.MaxBytes` before calculating the remaining transaction space.
The [block executor](../../internal/state/execution.go) reserves that evidence
space when selecting transactions; the [block validation
rules](../../internal/state/validation.go) also enforce the evidence byte cap.

Classic validators validate block evidence before voting for the block.
Pending evidence is only a candidate for inclusion. Once consensus commits a
block containing it, application processing follows the lifecycle below.

## Sending Evidence to the Application

The current in-process [Application interface](../../abci/types/application.go)
uses `ProcessProposal`, `FinalizeBlock` and `Commit`. The old delivery through
`EndBlock` is retired. The request field is named `ByzantineValidators`, and
its element type is `abci.Misbehavior`; there is no request field named
`Misbehavior` in this tree. See the [request types](../../abci/types/plain_types.go).

1. Before a classic validator prevotes for a valid proposed block,
   `BlockExecutor.ProcessProposal` supplies
   `RequestProcessProposal.ByzantineValidators = block.Evidence.ToABCI()` for
   application validation. This is not committed inclusion.
2. When applying the consensus-committed block, `BlockExecutor.ApplyBlock`
   validates the block and calls `FinalizeBlock` with the same conversion in
   `RequestFinalizeBlock.ByzantineValidators`, alongside the transactions,
   decided last commit, block hash and header.
3. After processing and saving the finalization response and updating the
   consensus state, the executor calls application `Commit`.
4. After `Commit` succeeds, `evpool.Update` records the included evidence as
   committed and removes it from pending storage and gossip. The executor then
   saves the state with the application's committed hash.

Application code can apply its misbehavior policy during finalization; the
application's durable commit follows in step 3. Evidence gossip or proposal
inspection alone must not be treated as committed punishment.

`EvidenceList.ToABCI()` removes the full proof and emits one record for the
validator in a duplicate vote, or one per identified Byzantine validator in a
light-client attack:

```go
type Misbehavior struct {
  Type             MisbehaviorType
  Validator        Validator
  Height           int64
  Time             time.Time
  TotalVotingPower int64
}
```

`Validator` contains `Address []byte` and `Power int64`.
`MisbehaviorType` has `UNKNOWN = 0`, `DUPLICATE_VOTE = 1` and
`LIGHT_CLIENT_ATTACK = 2`. The conversion and metadata consistency rules are
implemented in [types/evidence.go](../../types/evidence.go).
