<!-- markdown-link-check-disable -->

# Light Client Attack Detector

This page retains the upstream Tendermint detector's trust and trace reasoning
and maps it to the classic light client in this tree. The model assumes a
canonical chain on which correct full nodes agree, while malicious validators
and providers may try to convince a light client to accept another header.
Detection checks a primary's verified trace against witnesses before a forward
light-client update becomes trusted.

## Context of this document

Paxeer mainnet-beta is one BFT network run by `paxd`, with EVM chain ID 125
(`0x7d`). The kernel is part of the network and proposes blocks. A receipt is
final once its block is committed; the detector does not define a second
execution or settlement stage.

The node's `autobahn-config-file` configuration key selects Autobahn when
non-empty and disables it when empty ([configuration][config],
[node assembly][node]). This distinction determines the applicability of the
retained Tendermint model:

| Subject | Authoritative implementation | Applicability |
| --- | --- | --- |
| Light blocks and signed headers | [`types/light.go`][light-type] | Classic Tendermint data model. Basic consistency is separate from signature verification. |
| Sequential and skipping verification | [`light/verifier.go`][verifier], [`light/client.go`][client] | Classic trust root, validator overlap, trusting period and clock drift checks. |
| Witness cross-check and trace examination | [`light/detector.go`][detector] | Classic detector described below. |
| Evidence validation and attribution | [`internal/evidence/verify.go`][evidence-verifier], [`types/evidence.go`][evidence-type] | Classic evidence; amnesia does not yield attributable validators. |
| Autobahn certificates | [`autobahn/types/commit_qc.go`][commit-qc], [`committee.go`][committee], [`msg.go`][messages] | `CommitQC` and `FullCommitQC` verification against the committee and its signing domain, not Tendermint skipping verification. |
| Autobahn recovery | [`persisted_inner.go`][persisted-state], [`persist/persist.go`][persistence] | Durable consensus state and certificate validation, not a light-client trace or an accountability monitor. |

Under Autobahn, the classic `commit` handler returns
`coretypes.ErrCommitNotAvailable` and `broadcast_evidence` returns
`coretypes.ErrEvidenceNotAccepted` ([block handlers][blocks],
[evidence handler][evidence-rpc], [error declarations][rpc-errors]). The HTTP
light-client provider obtains signed headers through `Commit` and validator sets
through `Validators` ([provider][provider]); a compatible-looking block response
does not supply a classic commit certificate. This page makes no claim that the
classic detector verifies Autobahn blocks or implements Autobahn recovery.

The public EVM JSON-RPC is served at the root path of
`https://api-mainnet-beta.paxeer.network` and the numbered
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`
nodes, with chain ID 125. See the [public RPC reference][public-rpc]. Those
`eth_` calls are distinct from the engine's classic light-block and evidence
methods. The consensus header's string `ChainID`, used in signature checks,
must come from the configured consensus chain; EVM chain ID 125 is not a
replacement trust root.

## Tendermint Consensus and Light Client Attacks

The definitions in this section are retained research abstractions. Ordinary
Tendermint verification assumes more than two thirds correct voting power at
the relevant heights. A detector is a second line of defense when this
assumption fails: it can expose conflicting verifiable headers if it still has
access to a correct peer. It cannot recover trust when every available source
colludes.

#### **[TMBC-GENESIS.1]**

`Genesis` denotes the agreed genesis state. A light client normally begins with
an externally trusted height and hash; an untrusted provider cannot establish
that trust merely by supplying a genesis file or matching another provider.

#### **[TMBC-FUNC-SIGN.1]**

For adjacent light blocks `b` and `c`, `signs(b,c)` abstracts a valid sequential
verification step. The next-validator hash in `b` must match the validator hash
in `c`, and more than two thirds of `c`'s validator voting power must sign its
commit. The implementation is `VerifyAdjacent`, including chain, height and
time checks. A header's `LastCommitHash` is not itself a set of signatures.

#### **[TMBC-FUNC-SUPPORT.1]**

`supports(b,c,t)` abstracts a valid skipping-verification step from `b` to a
later block `c` at time `t`: the root remains within its trusting period, the
target is well formed, and enough trusted voting power signed the target.
In the upstream model this overlap is expressed using `b.NextValidators` and
requires more than one third of that set's voting power.

The actual `VerifyNonAdjacent` takes an explicit `trustedVals` set.
`light/client.go` supplies the trusted light block's `ValidatorSet` to `Verify`.
The default `DefaultTrustLevel` is `1/3`; accepted trust-level settings are in
`[1/3, 1)`. In addition to the trusted-set overlap, the target's own set must
provide more than two thirds signing power. The implementation's supplied set
and checks govern; the abstract `NextValidators` notation is not a separate
runtime field on `LightBlock`.

#### **[TMBC-SEQ-ROOTED.1]**

`sequ-rooted(b)` means there is a sequence of valid adjacent steps from genesis
to `b`. It expresses membership in the canonical history under the model.

#### **[TMBC-SKIP-TRACE.1]**

`skip-trace(b,c,t)` means a sequence starts at `b`, ends at `c`, and each
successive pair passes verification at time `t`. Call this a **verification
trace**. Heights increase; a trace need not contain every intervening height.
Downloading an intermediate block alone does not verify it.

#### **[TMBC-SIGN-SKIP-MATCH.1]**

For a canonical root `a`, `sign-skip-match(a,b,c,t)` states that if `b` and `c`
have the same height and both have verification traces from `a`, their header
hashes agree. Commits need not have identical signature subsets to certify the
same header.

#### **[TMBC-ATTACK.1]**

`attack(a,b,c,t)` means this agreement implication is false: two verifiable
traces from a common canonical root lead to different header hashes at the
same target height. The conflicting height is `b.Height == c.Height`; the
common root's height is a separate value.

### Node-based characterization of attacks

#### **[TMBC-MC-FORK.1]**

A main-chain fork in this model means that two correct full nodes have decided
on conflicting blocks. It concerns consensus safety, rather than only a light
client's interaction with malicious data providers.

#### **[TMBC-LC-ATTACK.1]**

A light-client attack has verifiable conflicting headers while correct full
nodes still agree on their chain. One branch may exist only in an attacker's
responses. This no-main-chain-fork assumption is necessary for the evidence
receiver to use its own chain as the reference.

#### **[TMBC-LC-ATTACK-EVIDENCE.1]**

Let a receiver have a chain from a common root `a` to `b`, while another
provider supplied a verified trace from `a` to a conflicting `c`. There is a
first trace step whose starting block agrees with the receiver's chain and
whose ending header differs from the receiver's header at that height.
The receiver can reconstruct the starting block and verify that step.

The existence argument is by induction: the initial block agrees; if each
next header also agreed, the final header would agree, contradicting the
assumed conflict. This argument needs a genuinely verified trace and a common
trusted root, not just two unequal responses.

#### **[TMBC-LC-EVIDENCE-DATA.1]**

The research evidence is the common height and the conflicting light block
for such a step. The receiver already has its own reference history, so the
sender need not transmit both full traces. This is a bound on the number of
blocks in the abstract evidence, not on encoded byte size. The implemented
evidence type and its same-height cases are described [below](#evidence).

### Block-based characterization of attacks

#### **[TMBC-SIGN-UNIQUE.1]**

`sign-unique(b,c)` says that two sequentially rooted blocks at the same height
have the same header hash. Distinct commits for the same header do not by
themselves violate this property.

#### **[TMBC-BLOCKS-MCFORK.1]**

A violation of `sign-unique` is a fork in the block-based model, even if no
correct full node has accepted the attacker's branch.

#### **[TMBC-BLOCKS-LCFORK.1]**

A block-based light-client fork has a uniquely canonical block `b` and a
different block `c` at the same height that passes skipping verification from
a common root. This separates a forged light-client branch from two canonical
sequential histories.

#### **[TMBC-BOGUS.1]**

A bogus block is neither sequentially rooted nor reachable by a valid
verification trace from any canonical root. Rejecting such a block is not the
same as producing valid evidence of a conflicting verifiable branch.

### Informal Problem statement

The detector checks data from untrusted peers in a distributed setting. It
does not decide network consensus or instruct the entire chain to halt. Its
local task is to cross-check a proposed update, identify a verifiable conflict,
and attempt evidence delivery to the providers that can validate it against
their own history. [Evidence handling][handling] describes the subsequent pool
and application path.

#### **[LCD-IP-MODEL.1]**

Retain the Tendermint model in which the usual voting-power assumption may be
violated but correct full nodes have no main-chain fork. Full-node providers
and signing validators are separate roles; a provider's failure to respond
does not prove that a validator signed an invalid vote.

#### **[LCD-IP-STATEMENT.1]**

Given a trusted root and a primary verification trace, compare the target
with witnesses. A divergent witness must be checked against the trace before
constructing evidence. Unverifiable data or insufficient responses must not
silently turn into a trusted update. A successful cross-check is distinct from
an incomplete check that found no evidence.

## Assumptions/Incentives/Environment

### Assumptions

#### **[LCD-A-CorrFull.1]**

The research model assumes at least one correct full node among the primary
and witnesses throughout detection, an initially correct trusted root, and a
root that stays usable within its trusting period. If every available peer
colludes, agreement between peers alone does not prove canonical history.

#### **[LCD-A-RelComm.1]**

The research liveness argument assumes reliable communication with a known
end-to-end bound `Delta`. A request/response timeout of at least `2 * Delta`
is needed under that model. This is an assumption for the proof, not a
latency guarantee supplied by the network. In practice providers can lag,
requests can fail and clock estimates can be wrong.

The implementation uses request contexts, provider timeouts, `maxClockDrift`
and `maxBlockLag`. When a witness is behind, the detector waits
`2 * maxClockDrift + maxBlockLag` before checking again. A timeout can justify
trying another data source but is not signed evidence of validator misconduct.

## Definitions

### Evidence

#### **[LC-DATA-EVIDENCE.1]**

The actual type in [`types/evidence.go`][evidence-type] is:

```go
type LightClientAttackEvidence struct {
    ConflictingBlock    *LightBlock
    CommonHeight        int64 `json:",string"`
    ByzantineValidators []*Validator
    TotalVotingPower    int64 `json:",string"`
    Timestamp           time.Time
}
```

This extends the research common-height/conflicting-block pair with metadata
validated by the receiving full node. `CommonHeight` may equal the conflicting
height for equivocation or amnesia; see [construction details][handling].

#### **[LC-DATA-EVIDENCE-INT.1]**

The upstream `InternalEvidence` wrapper associated evidence with a peer.
The actual detector passes a `provider.Provider` receiver to `sendEvidence`;
there is no need to invent that wrapper as a public request type. Provider
identities identify destinations, not necessarily accused validator addresses.

#### **[LC-SUMBIT-EVIDENCE.1]**

The research `submitEvidence` operation corresponds to the detector's
`sendEvidence`, which calls `Provider.ReportEvidence`. Reporting is best effort
and failures are logged. Successful local detection does not guarantee delivery
to both providers, inclusion in a block, or punishment by the application.

### LightStore

The retained [verification research][verification] uses LightStore and trusted,
verified and untrusted states as abstract concepts. In the current Go client,
the verification trace is a `[]*types.LightBlock`, separate from the trusted
store. Forward verification and witness checks must succeed before
`updateTrustedLightBlock` saves the target. The old draft's proposed
`VerifyToTarget` signature and `LatestVerified` calls are not Go APIs to use.

## (Distributed) Problem statement

The following properties retain the intent of the upstream model, with their
assumptions explicit. They are not unconditional guarantees for a failed or
incomplete RPC operation.

#### **[LCD-DIST-INV-ATTACK.1]**

Returned evidence must concern a verified conflict. Different bytes, a missing
block or a timeout alone are insufficient. The receiving node independently
validates the evidence before using it.

#### **[LCD-DIST-INV-STORE.1]**

Under the common-root, correct-peer and timing assumptions, a successfully
cross-checked trace follows the canonical chain. Merely returning no evidence
on an error path does not establish this property and must not update trust.

#### **[LCD-DIST-LIVE.1]**

The research algorithm terminates with bounded peer operations and a finite
peer set or a trust-expiry failure. Deployment liveness also depends on actual
provider behavior and timeout configuration; the trusting period is not itself
a watchdog interrupting an arbitrary blocked request.

#### **[LCD-DIST-TERM-NORMAL.1]**

With a canonical primary trace, no attack, a usable root, and timely correct
peer responses, cross-checking produces no attack evidence and can succeed.

#### **[LCD-DIST-TERM-ATTACK.1]**

If a peer supplies a verifiable conflicting trace while the root and required
peers remain usable, the detector can construct evidence. An attack never
presented to the detector cannot be detected by this observation procedure.

#### **[LCD-DIST-SAFE-SECONDARY.1]**

The research claim that no correct secondary is replaced depends on reliable,
bounded communication and appropriate timeouts. It does not make provider
removal in a real deployment a proof of maliciousness.

#### **[LCD-DIST-SAFE-BOGUS.1]**

A witness supplying invalid data is removed according to the implementation's
provider rules. Failure to obtain any matching usable witness blocks success;
it does not establish that the primary's header is canonical.

# Protocol

## Functions and Data defined in other Specifications

The historical names `AttackDetector`, `CreateEvidenceForPeer`,
`VerifyToTarget` and `Replace_Secondary` describe the research decomposition.
The implementation uses methods on `light.Client`, a primary provider, a
witness list and a trusted store. The following names refer to actual methods
in [`light/client.go`][client] and [`light/detector.go`][detector].

## Outline

1. `verifySequential` or `verifySkippingAgainstPrimary` constructs a verified
   primary trace from a trusted root to the target.
2. `detectDivergence` asks witnesses to compare the target and examines
   conflicting responses.
3. A verified conflict triggers evidence construction and best-effort reports,
   then `ErrLightClientAttack`. Otherwise at least one matching witness is
   required for a successful cross-check.
4. The caller saves the target to the trusted store only after success.

## Details of the functions

#### **[LCD-FUNC-DETECTOR.1]**

`detectDivergence` requires at least two light blocks in the primary trace.
It returns `ErrNoWitnesses` when no witnesses are configured. It requests
witness comparisons concurrently and processes the results:

- A matching header counts toward cross-reference success. Since proposer
  priorities are not in the header hash, the detector also compares
  `ValidatorSet.ProposerPriorityHash()`; a difference returns
  `ErrProposerPrioritiesDiverge` and is not attributable attack evidence.
- `ErrConflictingHeaders` triggers `handleConflictingHeaders`, which verifies
  the witness's trace before constructing evidence. If that verification
  fails, the witness is scheduled for removal and blacklisting.
- Other bad-witness responses are also scheduled for removal. Context
  cancellation and deadline errors are returned. Lagging or unhelpful
  responses do not count as matches.

On the path that completes comparisons, the detector applies scheduled witness
removals and blacklisting. It returns success if at least one witness matched,
otherwise `ErrFailedHeaderCrossReferencing`, subject to errors from witness
removal. A verified attack returns early; it does not keep collecting evidence
from every witness as the old pseudocode proposed.

### Trace examination and evidence creation

`examineConflictingHeaderAgainstTrace` requires the source's starting block to
match the trusted root. It verifies source blocks at the primary trace's
intermediate heights using `verifySkipping`, advancing the common point when
headers agree. The first verified divergence yields the source trace and the
conflicting reference block. The method also handles a forward lunatic claim
whose height is beyond the witness head but whose time contradicts that head.
If a divergence cannot be verified, it returns an error rather than fabricated
evidence.

`handleConflictingHeaders` forms evidence against the primary and sends it to
the witness. It then reverses the roles and tries to form evidence against
the witness for the primary. Once the first conflict is verified, failure of
the reverse attempt still returns `ErrLightClientAttack`. `sendEvidence`
only logs reporting failures; it does not weaken that local rejection.

### Error conditions

The Go declarations are in [`light/errors.go`][light-errors]:

| Outcome | Meaning |
| --- | --- |
| `ErrOldHeaderExpired` | The verifier cannot extend trust from the expired header. |
| `ErrNewValSetCantBeTrusted` | A skipping step lacks the required trusted-set overlap; the client may need intermediate blocks. |
| `ErrInvalidHeader` | Header consistency, time, validator or commit verification failed. |
| `ErrVerificationFailed` | Wraps a failed verification step, including its source and target heights and reason. |
| `ErrNoWitnesses` | The detector has no configured witnesses. |
| `ErrFailedHeaderCrossReferencing` | No witness supplied the matching usable data needed for success. |
| `ErrProposerPrioritiesDiverge` | Header hashes agree but proposer-priority hashes differ; attribution is not established. |
| `ErrLightClientAttack` | A verified conflict prevents accepting the update; reporting remains best effort. |

The old draft's `ErrorTrustExpired`, `ErrorNoPeers`, `FoundEvidence` and
`FaultyPeer` are research labels, not the current error declarations.

## Correctness arguments

The key retained argument is the first-divergence argument: two traces with
the same trusted root cannot end in different headers without a verified
diverging step. That step supplies evidence a receiver can check against its
own history. Reversing the roles avoids assuming that the primary or witness
is inherently honest.

Successful storage depends on verification and a successful witness cross-check.
Termination and the claim that correct peers are never removed require the
research timing assumptions; production timeouts do not establish culpability.
The full node still validates evidence independently. Attacker classification
and its amnesia limitation are explained in [isolation][isolation] and
[evidence handling][handling]. None of these arguments proves equivalence to
Autobahn certificate verification or durable restart behavior.

# References

- [Notes on evidence handling][handling]: complete evidence shape, construction,
  receiving-node checks and application delivery.
- [Lightclient attackers isolation][isolation]: lunatic, equivocation and
  amnesia attribution and the limits of the completeness argument.
- [Verification research][verification] and [supervisor research][supervisor]:
  retained upstream Tendermint models, not alternative Go API declarations.

[handling]: ../attacks/notes-on-evidence-handling.md
[isolation]: ../attacks/isolate-attackers_002_reviewed.md
[verification]: ../verification/verification_002_draft.md
[supervisor]: ../supervisor/supervisor_001_draft.md
[config]: ../../../config/config.go
[node]: ../../../node/node.go
[light-type]: ../../../types/light.go
[client]: ../../../light/client.go
[verifier]: ../../../light/verifier.go
[detector]: ../../../light/detector.go
[light-errors]: ../../../light/errors.go
[provider]: ../../../light/provider/http/http.go
[evidence-type]: ../../../types/evidence.go
[evidence-verifier]: ../../../internal/evidence/verify.go
[blocks]: ../../../internal/rpc/core/blocks.go
[evidence-rpc]: ../../../internal/rpc/core/evidence.go
[rpc-errors]: ../../../rpc/coretypes/responses.go
[commit-qc]: ../../../autobahn/types/commit_qc.go
[committee]: ../../../autobahn/types/committee.go
[messages]: ../../../autobahn/types/msg.go
[persisted-state]: ../../../internal/autobahn/consensus/persisted_inner.go
[persistence]: ../../../internal/autobahn/consensus/persist/persist.go
[public-rpc]: ../../../../docs/site/docs/reference/public-rpc.md
