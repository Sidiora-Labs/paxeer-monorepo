# Light Client Verification

This page replaces the inherited early-signature-verification fragment with a
map to the classic verifier in this tree. The old fragment was design pseudocode:
it omitted the source of `lookUpByIndex` and used obsolete key and lookup APIs.
It was not an executable Go test or the active Autobahn certificate consumer.
See the [verification model](./verification_002_draft.md) for the retained trust,
timing and validator-overlap assumptions.

## Applicability

Classic routines authenticate `types.Commit` using a `types.ValidatorSet`.
Autobahn uses different committee-bound vote and certificate types. Both are
consensus-engine paths inside the single `paxd` network. A transaction receipt is
final when its block is committed; this verification does not add a settlement
stage.

#### **[LCV-FUNC-VERIFYCOMMITLIGHT.1]**

The current package-level API in
[`types/validation.go`](../../../types/validation.go) is:

```go
func VerifyCommitLight(chainID string, vals *ValidatorSet, blockID BlockID,
    height int64, commit *Commit) error
```

This is an API signature, not a standalone program. The corresponding method
`(*ValidatorSet).VerifyCommitLight` delegates to it. Related APIs are:

| API | Threshold and purpose | Signature selection |
| --- | --- | --- |
| `VerifyCommitLight` | More than two thirds of candidate-set voting power | May return after reaching quorum |
| `VerifyCommitLightAllSignatures` | Same candidate quorum | Checks all signatures selected by the light-verification filter |
| `VerifyCommitLightTrusting` | More than the configured fraction of trusted-set voting power | Looks up validators by address; may return after overlap threshold |
| `VerifyCommitLightTrustingAllSignatures` | Same trusted overlap | Checks all selected matching signatures |

The trusting variants take `(chainID, vals, commit, trustLevel)`, where
`trustLevel` is a `math.Fraction`. They check overlap with a trusted set that may
differ from the candidate's validator set; they do not replace the candidate's
own commit-quorum check.

## Candidate Commit Verification

`VerifyCommitLight` first rejects a nil validator set or commit, mismatched
signature count, an unexpected height, or a different block ID. It calculates
`votingPowerNeeded = vals.TotalVotingPower() * 2 / 3` using integer arithmetic.
The required tally is **strictly greater** than that value.

Only signatures with `BlockIDFlagCommit` are selected. Absent and nil-block
votes do not contribute. Candidate-set lookup uses the signature's position and
requires its validator address to match that validator. Each counted signature
must verify the commit's chain-bound vote sign bytes with the validator's public
key. The current key API is `PubKey.Verify`; it is not the fragment's
`VerifySignature` call.

The implementation has batch and single-signature paths. The early-exit variant
can succeed once enough selected signatures have verified, without examining
the remainder. Its success proves sufficient signed voting power; it does not
certify every signature in the commit. A tally that does not exceed the threshold
returns `ErrNotEnoughVotingPowerSigned`.

The `AllSignatures` light variants still apply their selection filter. For the
full consensus commit check, `VerifyCommit` checks all non-absent signatures,
including nil-block votes, while counting only block-commit votes toward quorum.
Choose the routine according to the required guarantee.

## Trusted-Set Overlap

`VerifyCommitLightTrusting` derives the threshold from trusted total power times
`trustLevel.Numerator / trustLevel.Denominator`. Selected commit signatures are
looked up by validator address. Unknown addresses do not add trusted voting
power, and duplicate matching validators encountered during verification are
rejected. The tally must exceed the threshold. The classic forward verifier
validates the configured trust fraction in `[1/3, 1)`.

[`light/verifier.go`](../../../light/verifier.go) combines this with header,
time and validator-hash checks. For non-adjacent verification it checks trusted
overlap before the candidate set's greater-than-two-thirds commit quorum;
insufficient overlap becomes `ErrNewValSetCantBeTrusted`. Adjacent verification
instead requires the candidate validator hash to equal the trusted header's
next validator hash. These are separate paths, not a boolean lookup switch for
callers to set in copied pseudocode.

## Autobahn Certificates

The running Autobahn data consumer is
[`giga/data.go`](../../../internal/p2p/giga/data.go). It decodes a
`FullCommitQC`, calls `Verify` against the configured committee, checks progress
of its certified global block range and inserts it into the data state.

[`CommitQC.Verify` and `FullCommitQC.Verify`](../../../autobahn/types/commit_qc.go)
and the [shared signature verifier](../../../autobahn/types/msg.go) enforce
committee membership, committee-domain signatures, unique signers, quorum and
certified header consistency. Unlike the classic early-exit routine, QC
verification checks every supplied signature. The committee quorum is defined
by [`Committee.CommitQuorum`](../../../autobahn/types/committee.go), not by the
classic validator-set API above.

The [message index](../../p2p/messages/README.md#autobahn-streaming-rpcs) documents
stream registrations, and [block sync](../../p2p/messages/block-sync.md#autobahn-data-fetch)
distinguishes classic block responses from Autobahn payload fetches. Persisted
Autobahn votes and QCs govern restart safety; this historical signature fragment
provides no proof of equivalent recovery or light-client trust semantics.
