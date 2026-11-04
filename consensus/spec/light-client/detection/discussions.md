# Results of Discussions and Decisions

## Status and applicability

These are retained Tendermint light-client design decisions, with their
relationship to the current implementation made explicit. Read the
[attack-detector model](detection_003_reviewed.md) for the formal definitions,
trust and timing assumptions, and the [IBC function drafts](draft-functions.md)
for the proposed relayer operations. The supervisor and store states below
are research abstractions rather than public Go APIs.

Paxeer is one BFT network whose kernel runs inside the network and participates
in block proposal. Its EVM chain ID is 125 (`0x7d`); a receipt is final once
its block is committed. A light client's inability to verify a remote response
is a limitation of that observer's knowledge, not another network finality stage.

The classic implementation lives in [`light/client.go`](../../../light/client.go)
and [`light/detector.go`](../../../light/detector.go). It verifies signed headers
from a primary and cross-checks them with witness providers. These decisions
depend on Tendermint validator voting power, a trusted starting block, an
unexpired trusting period, bounded communication and at least one correct
peer among the primary and witnesses. Comparing several responses alone does
not establish those assumptions.

Autobahn is selected through `autobahn-config-file` in
[`config/config.go`](../../../config/config.go). Its
[`LaneQC`](../../../autobahn/types/lane_qc.go) and
[`CommitQC`](../../../autobahn/types/commit_qc.go) types are committee-verified
certificates, not the classic light blocks assumed here. Under Autobahn the
consensus handlers in [`internal`](../../../internal/) return
`ErrCommitNotAvailable` from `rpc/core/blocks.go`'s `Commit`, and
`ErrEvidenceNotAccepted` from `rpc/core/evidence.go`'s `BroadcastEvidence`.
These research decisions do not define an Autobahn light client, evidence
submission protocol or restart procedure. Autobahn consensus persistence is
implemented separately under
[`internal/autobahn/consensus/persist`](../../../internal/autobahn/consensus/persist/).

## Proof size and evidence generation

The historical [minimal-proof discussion](https://github.com/tendermint/tendermint/issues/5083)
identified a useful constraint: evidence generation should not depend on a
faulty primary cooperating to minimize a proof.

For example, start with a trusted block at height 1. The primary supplies a
block at height 10 that verifies directly from that root. A witness supplies
a conflicting block at height 10 whose verification requires an intermediate
block at height 5. Sending only the witness's block at 5 does not locate the
conflict; sending only its block at 10 omits the trust path. Asking the primary
for more blocks may reduce the proof, but a faulty primary can stop responding.

The retained design decisions are:

- A full research proof of fork contains two verification traces from the same
  trusted light block to conflicting headers at the same height.
- Keep enough trace data to establish the conflict without further cooperation
  from the suspect provider. Proof minimization is an optimization.
- Compare the primary against each witness. Comparing every pair of witnesses
  is not the detector's task.
- Two different headers at the same height alone are insufficient evidence.
  One may be an unsupported, bogus block; both branches need a valid trust path.

The current implementation examines divergent traces in
[`handleConflictingHeaders`](../../../light/detector.go) and constructs
[`LightClientAttackEvidence`](../../../types/evidence.go) containing a
conflicting block and common height, with application evidence fields. It
attempts to send evidence against the primary to the supporting witness, then
examines the reverse direction for evidence to send to the primary. It returns
`ErrLightClientAttack` once the attack is detected, including when the reverse
examination fails. `sendEvidence` logs reporting failures; detection is not a
guarantee of delivery, inclusion or punishment. The two-trace research object
is not the serialized evidence payload.

## Initial trust

The initial header needs an independently trusted height and hash. Walking
backward from arbitrary provider data cannot manufacture that trust: a faulty
peer can keep supplying unsupported blocks until the useful trust window is
lost.

In the implementation, `initializeWithTrustOptions` in
[`light/client.go`](../../../light/client.go) fetches the configured initial
block, checks its hash against `TrustOptions.Hash`, verifies its commit and
chain identity, compares it with witnesses, and saves it. The historical phrase
“trust the initial header as is” describes the external trust anchor; it does
not permit skipping these checks.

# Light Client Sequential Supervisor

#### **[LC-FUNC-SUPERVISOR.1]:**

The retained supervisor sketch alternates verification and detection:

```text
repeat for each requested target height:
  verify a primary trace from the trusted root to the target
  if verification fails, handle the error before advancing trust
  cross-check the verified trace with witnesses
  if an attack is established, attempt evidence reporting and stop
  if cross-checking succeeds, save the target as locally trusted
```

This is an outline, not a callable `Sequential-Supervisor` function or a
complete error-handling algorithm. The historical sketch's `Replace_Primary`,
`Forkdetector`, `SubmitProofOfFork` and `ErrorFork` names do not define the
current API. Use `VerifyLightBlockAtHeight` in
[`light/client.go`](../../../light/client.go) and the returned errors.
Forward sequential and skipping verification call `detectDivergence` before
`updateTrustedLightBlock` saves a newly trusted block. This description does
not replace the client's distinct paths for existing stored blocks or backward
verification.

The detector requires witnesses. It returns `ErrNoWitnesses` when none are
available and `ErrFailedHeaderCrossReferencing` when no witness matches and
no attack evidence has been established. Context errors and
`ErrProposerPrioritiesDiverge` are also failure outcomes; they must not be
interpreted as successful cross-checks. The error definitions are in
[`light/errors.go`](../../../light/errors.go).

The research liveness argument assumes a correct peer responds within the
stated timing bound. It does not promise that a real client under arbitrary
network delay runs forever, always replaces peers successfully, or always
finishes verification before the root expires.

# Semantics of the LightStore

The historical model distinguishes four local states:

| Research state | Meaning in the model |
| --- | --- |
| `StateUnverified` | Downloaded but not yet verified. |
| `StateVerified` | Verified against the primary, awaiting witness cross-checking. |
| `StateFailed` | Verification failed. |
| `StateTrusted` | Cross-checking succeeded under the model's correct-peer assumptions. |

These are not states exposed by the current
[`store.Store`](../../../light/store/store.go) interface. That interface saves,
loads, deletes and prunes light blocks; it also provides `LightBlockBefore`.
The client keeps a trusted store and uses temporary traces during verification.
Its `TrustedLightBlock` method retrieves locally trusted data. There is no
four-state enum to set through this store API.

The distinction between “verified against one provider” and “cross-checked”
remains useful: accepting a primary-only result too early may expose the
observer to an attack that detection later discovers. Conversely, witness
agreement is meaningful only with the trust, validator and timing assumptions
of the [detector model](detection_003_reviewed.md#assumptions). A local
`StateTrusted` label is not a substitute for consensus block commitment.

The original discussion also considered intermittent access to a correct peer.
During intervals with no correct peer, the observer may have no way to detect
an attack. Reconnection might reveal a conflict and require reconsidering its
local trust history. Replacing `StateTrusted` with a flag was a research
proposal for that weaker assumption, not an implemented API migration or a
current-network rollback guarantee. The invariant named in that discussion,
[`LCD-DIST-INV-STORE.1`](detection_003_reviewed.md#lcd-dist-inv-store1), depends
on the stronger correct-peer assumption retained by the detector model.
