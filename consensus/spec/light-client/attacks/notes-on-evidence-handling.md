# Light client attacks

These notes retain the useful Tendermint trace and evidence model and describe
its implementation in this fork. A light-client attack involves conflicting
headers that can be verified from a trusted root under the model's assumptions.
The primary supplies one trace and a witness supplies another. Neither role
establishes which peer is honest, so evidence may need to be constructed in both
directions.

## Current-mode applicability

Paxeer mainnet-beta is one BFT network run by `paxd`, with EVM chain ID 125
(`0x7d`). Its kernel is part of the network and proposes blocks. A receipt is
final when its block is committed. This page describes the classic Tendermint
evidence path retained in that tree; it does not add a settlement step.

The `autobahn-config-file` node configuration key enables Autobahn when
non-empty ([configuration][config], [node assembly][node]). In that mode the
classic `commit` RPC returns `coretypes.ErrCommitNotAvailable`, and
`broadcast_evidence` returns `coretypes.ErrEvidenceNotAccepted`
([block handlers][blocks], [evidence handler][evidence-rpc], [errors][rpc-errors]).
Consequently the classic HTTP light-block provider and this evidence flow cannot
be treated as an Autobahn light-client protocol.

Autobahn's [`CommitQC` and `FullCommitQC`][commit-qc] are checked against an
Autobahn [`Committee`][committee]. Their signed-message and durable-state
contracts are distinct from Tendermint commits and the research monitor below.
See the [detector's applicability map][detection] for the implementation entry
points and boundaries.

## Evidence shape

In the research abstraction, the receiver already holds its own chain, so the
sender need only identify a common block by height and provide one conflicting
light block. This avoids transmitting both complete verification traces. It
does not make evidence a fixed number of bytes: validator sets and signatures
vary in size.

The actual Go type in [`types/evidence.go`][evidence-type] has additional
application metadata:

```go
type LightClientAttackEvidence struct {
    ConflictingBlock    *LightBlock
    CommonHeight        int64 `json:",string"`
    ByzantineValidators []*Validator
    TotalVotingPower    int64 `json:",string"`
    Timestamp           time.Time
}
```

`LightBlock` contains `*SignedHeader` and `*ValidatorSet`; the signed header
contains `*Header` and `*Commit` ([types][light-type]). `ValidateBasic` checks
internal consistency, including that the commit references the header and the
validator set matches its hash. It does not authenticate signatures.

The historical two-field type is therefore an explanation of evidence
construction, not a complete request schema. Use the type's existing protobuf
and tagged JSON conversions for transport. `CommonHeight` and
`TotalVotingPower` use decimal strings in the Go JSON representation.

## Light client attack creation

A trusted root is established outside the untrusted primary's assertions. In
the retained Tendermint model, normal verification assumes more than two thirds
correct voting power at the relevant heights, a non-expired root and timely
access to a correct full node. Detection considers a violation of the voting
assumption while correct full nodes still agree on their canonical chain.

### Primary trace

The primary trace starts with the trusted light block and ends at the requested
height. Each successive entry has passed a verification step from a preceding
verified block. Skipping verification can fetch intermediate blocks when a
direct jump lacks sufficient overlap. A downloaded block is not trusted merely
because it was fetched.

In [`light/client.go`][client], both `verifySequential` and
`verifySkippingAgainstPrimary` invoke `detectDivergence` before a successful
forward update is saved through `updateTrustedLightBlock`. The low-level
checks are in [`light/verifier.go`][verifier]. See the
[detection specification][detection] for the mathematical trace model.

### Witness with a conflicting header

The implemented functions in [`light/detector.go`][detector] replace the old
`DetectLightClientAttacks` and `DetectLightClientAttack` pseudocode:

1. `compareNewLightBlockWithWitness` compares the target header hash with each
   witness. It treats lagging witnesses separately from proven conflicts.
2. For a conflicting response, `examineConflictingHeaderAgainstTrace` checks
   that the witness agrees with the trusted root, then verifies its blocks at
   trace heights to locate a divergence. The check can also expose a primary
   claiming a future height inconsistent with the witness's block time.
3. `newLightClientAttackEvidence` constructs evidence against the primary using
   the witness's trace as the local reference. `sendEvidence` sends it to that
   witness.
4. `handleConflictingHeaders` reverses the roles and attempts to construct
   evidence against the witness for submission to the primary. After a verified
   conflict it returns `ErrLightClientAttack`, including when the reverse attempt
   fails.

Reporting is best effort: `sendEvidence` logs reporting errors. Returning an
attack error means that the update is refused, not that both providers accepted
evidence or that validators have been punished. Invalid or unavailable witness
data cannot by itself establish validator misbehavior.

For a lunatic header, the constructor uses the last common block's height,
time and total voting power. When deterministic header fields agree (the
equivocation or amnesia cases), it uses the trusted block at the conflicting
height for those fields. It then computes `ByzantineValidators` from the
relevant validator sets and commits. Thus `CommonHeight` can equal the
conflicting height; it is not always the preceding trace entry.

## Evidence handling

### Receiving-node validation

The classic HTTP provider's `ReportEvidence` calls the consensus client's
`BroadcastEvidence` method ([provider][provider]). This is an engine RPC, not
an `eth_` method. The public EVM JSON-RPC nodes are documented in the
[public RPC reference][public-rpc]; their EVM chain ID does not replace the
consensus header's string `ChainID` used to verify signatures.

On a classic node, [`BroadcastEvidence`][evidence-rpc] rejects missing evidence,
runs `ValidateBasic`, and calls the evidence pool's `AddEvidence`. A successful
response contains the evidence hash. That acknowledges submission to the pool,
not inclusion in a committed block.

The receiving full node validates against its own stored history in
[`internal/evidence/verify.go`][evidence-verifier]:

- It requires local block metadata and validators for the evidence height.
  Missing local history can mean that the node is behind; it is not automatically
  proof that the sender supplied invalid evidence.
- It rejects evidence as expired when **both** the elapsed block count exceeds
  `ConsensusParams.Evidence.MaxAgeNumBlocks` and elapsed time exceeds
  `ConsensusParams.Evidence.MaxAgeDuration`. Evidence time comes from the local
  block metadata, rather than the sender's timestamp.
- It loads the common signed header and the canonical signed header at the
  conflicting height. `getSignedHeader` reads `LoadBlockCommit(height)` directly.
  For a forward lunatic claim beyond local history it can use the latest local
  signed header and checks the time contradiction.
- `VerifyLightClientAttack` checks the required common-set overlap when heights
  differ and verifies all signatures contributing to the conflicting commit.
  The conflicting set must supply more than two thirds voting power. When
  common and conflicting heights coincide, deterministic header fields must
  agree. The header hash must conflict with local history, with the additional
  time check for forward lunatic evidence.
- `ValidateABCI` checks total voting power, timestamp, and the computed
  byzantine validator addresses and powers. If only this metadata is wrong, the
  pool regenerates it, attempts to save the corrected evidence, and returns the
  validation error to the caller.

The research uses an unbonding period to bound accountability. The concrete
pool rule above uses consensus evidence parameters; it should not be described
as simply substituting `UNBONDING_PERIOD` into the light-client verifier.

### Attribution and application notification

[`GetByzantineValidators`][evidence-type] uses the following classification;
[attacker isolation][isolation] explains the conditional correctness goal.

| Attack class | Implemented attribution |
| --- | --- |
| Lunatic | Commit signers who belong to the common validator set. Only `BlockIDFlagCommit` signatures count. |
| Equivocation | Validators signing both conflicting blocks in the same commit round, again excluding absent and nil votes. |
| Amnesia | No validators are returned when the deterministic fields agree but commit rounds differ. |

`ConflictingHeaderIsInvalid` compares `ValidatorsHash`, `NextValidatorsHash`,
`ConsensusHash` and `AppHash`. It also compares `LastResultsHash` unless
`types.SkipLastResultsHashValidation` is enabled. The accused validator list is
ordered by voting power and checked by the receiver.

Once evidence is included in a classic block, `LightClientAttackEvidence.ABCI`
produces `abci.MisbehaviorType_LIGHT_CLIENT_ATTACK` entries. The
[block executor][executor] passes them in
`RequestFinalizeBlock.ByzantineValidators` to the in-process
[`Application.FinalizeBlock`][application] call. The application decides the
consequences. These notes do not promise that the evidence pool directly slashes
validators, checks their current bonded status, or halts the entire network.

### Amnesia attack evidence handling

The upstream notes proposed a monitor to identify invalid Tendermint voting
transitions across rounds. This remains research material. In this tree the
different-round branch of `GetByzantineValidators` returns an empty list;
there is no implemented `HandleAmnesiaAttackEvidence` monitor in this path.

The proposal requires validators to retain all votes sent and received during
multi-round heights. A trusted monitor would request those vote sets, collect
them within a specified deadline, and merge authenticated votes observed by
other validators into each sender's history. It would then look for:

- Multiple prevotes or precommits by one validator in the same round.
- A precommit without the required quorum of supporting prevotes.
- A later conflicting prevote without the intervening quorum needed to justify
  leaving an earlier lock.

The proposed monitor also classified a validator that missed its vote-set
deadline as faulty. That conclusion depends on the model's reliable, bounded
communication assumption; a timeout on a deployed network alone does not prove
validator misconduct. Neither vote retention nor the monitor is established by
the existence of `LightClientAttackEvidence`.

This proposed Tendermint voting-history analysis is not an Autobahn recovery
protocol. Autobahn's [persisted consensus state][persisted-state] and
[persistence implementation][persistence] govern restart behavior; they must be
read with their own certificate and committee checks.

## References

- [Light client attack detector][detection]: trace definitions, assumptions and
  concrete detector outcomes.
- [Lightclient attackers isolation][isolation]: validator attribution and the
  limits of the upstream completeness argument.
- [Verification research][verification]: the retained Tendermint trust model.

[detection]: ../detection/detection_001_reviewed.md
[isolation]: isolate-attackers_002_reviewed.md
[verification]: ../verification/verification_002_draft.md
[config]: ../../../config/config.go
[node]: ../../../node/node.go
[client]: ../../../light/client.go
[verifier]: ../../../light/verifier.go
[detector]: ../../../light/detector.go
[provider]: ../../../light/provider/http/http.go
[light-type]: ../../../types/light.go
[evidence-type]: ../../../types/evidence.go
[evidence-verifier]: ../../../internal/evidence/verify.go
[evidence-rpc]: ../../../internal/rpc/core/evidence.go
[blocks]: ../../../internal/rpc/core/blocks.go
[rpc-errors]: ../../../rpc/coretypes/responses.go
[executor]: ../../../internal/state/execution.go
[application]: ../../../abci/types/application.go
[committee]: ../../../autobahn/types/committee.go
[commit-qc]: ../../../autobahn/types/commit_qc.go
[persisted-state]: ../../../internal/autobahn/consensus/persisted_inner.go
[persistence]: ../../../internal/autobahn/consensus/persist/persist.go
[public-rpc]: ../../../../docs/site/docs/reference/public-rpc.md
