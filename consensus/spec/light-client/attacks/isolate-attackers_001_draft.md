# Lightclient Attackers Isolation

This page preserves the problem and attack classification of the upstream
**classic Tendermint** draft, while mapping them to the current Go evidence
implementation. It is not a complete isolation algorithm for every attack.

Paxeer mainnet-beta is one BFT network (`paxd`, EVM chain ID 125); the kernel
participates in the network and proposes blocks. A receipt is final when its
block commits. This classic light-client research does not establish the safety
or accountability of Autobahn certificates, availability, committee changes or
recovery. See [State](../../core/state.md#autobahn-state-and-recovery) for that
mode boundary and [fork accountability](../accountability/README.md) for the
retained attack scenarios.

# Outline

The sections below distinguish the historical goal, the implemented evidence
shape, validation prerequisites and the limits of attribution. Names retained
from the draft identify research concepts; they are not additional Go APIs.

# Part I - Basics

Classic light-client [verification](../verification/verification_002_draft.md)
authenticates headers using signed commits and validator sets. Safety assumes
less than one third Byzantine voting power in the relevant sets, as well as the
client's trusted-base, trusting-period and clock assumptions. Skipping
verification additionally checks the configured trusted-set overlap. A bad RPC
response alone is not proof that consensus safety was violated: a correct
verifier may reject it before accepting a header.

Once verified conflicting headers are available, an evidence mechanism seeks
to identify protocol violators without accusing correct validators. Collecting
evidence, attributing signers and imposing an application penalty are distinct
steps. Their assumptions must be checked independently.

# Part II - Definition of the Problem

The historical isolation goal was to output validator addresses representing
more than one third of the relevant bonded voting power, with no correct
validator included, when a verified attack is within the assumed unbonding
window. That is a research goal, not the return-value guarantee of the current
Go helper for every attack class.

The current type in [`types/evidence.go`](../../../types/evidence.go) is:

```go
type LightClientAttackEvidence struct {
    ConflictingBlock    *LightBlock
    CommonHeight        int64 `json:",string"`
    ByzantineValidators []*Validator
    TotalVotingPower    int64 `json:",string"`
    Timestamp           time.Time
}
```

`CommonHeight` identifies the trusted common reference. The additional fields
carry attribution and application-facing metadata, which the evidence pool
checks against local chain history. The output is not a set of p2p peer IDs:
validator addresses identify the accused consensus keys.

#### **[FN-INV-Output.1]**

The retained research requirement is that a nonempty accusation contain only
validators whose signed messages prove misconduct, with sufficient voting power
under the relevant reference set. A production caller must not interpret an
empty attribution as proof that no attack occurred: different-round amnesia is
the explicit unsupported-attribution case.

The current validation requirements include well-formed evidence, authenticated
validator sets and commits, a conflicting trusted header, and non-expired local
history. The pool uses consensus evidence-age limits; a historical requirement
phrased only in terms of application unbonding time is insufficient. See
[State's evidence rules](../../core/state.md#evidence).

# Part IV - Protocol

## Isolation

### Outline

The current path is implemented by
[`internal/evidence/verify.go`](../../../internal/evidence/verify.go) and
[`types/evidence.go`](../../../types/evidence.go). Verify before attribution:
`GetByzantineValidators` assumes authenticated inputs and does not independently
perform the full signature and history validation.

#### **[LCAI-FUNC-MAIN.1]**

The draft name `isolateMisbehavingProcesses` is not a Go entry point in the
current evidence implementation. Its classification corresponds to these
branches in `LightClientAttackEvidence.GetByzantineValidators`:

| Condition after validation | Implemented attribution |
| --- | --- |
| `ConflictingHeaderIsInvalid(trusted.Header)` | Commit signers for the conflicting block that are members of the common trusted validator set. |
| Deterministic fields match and commit rounds match | Validators with block-commit signatures in both conflicting commits. |
| Deterministic fields match but commit rounds differ | Empty validator set: the commits alone do not identify amnesia culprits. |

The helper returns `[]*Validator`, sorted by voting power for the attributed
branches. It uses block-commit signatures (`BlockIDFlagCommit`), excluding nil
and absent votes. Membership and signature validation must precede its indexed
same-round comparison.

The old pseudocode fetched the canonical commit from the following block's
`LastCommit`. The current helper instead accepts a trusted `SignedHeader` with
its commit; callers need the appropriate authenticated reference and retained
history. A universal requirement to wait for height `H + 1` is not the current
helper's API.

### Details of the Functions

#### **[LCAI-FUNC-VVU.1]**

The historical `ValidAndVerifiedUnbonding` concept is not a substitute for the
implemented checks. `VerifyLightClientAttack` receives the evidence, common and
trusted signed headers, the common validator set, current time and a duration.
Its documented prerequisites require basic validation and evidence-expiration
checks by its caller.

When common and conflicting heights differ, it checks trusted-set overlap with
`VerifyCommitLightTrustingAllSignatures`. It then verifies all signatures in the
conflicting commit with `VerifyCommitLightAllSignatures`, requiring more than
two thirds of the conflicting set's voting power. Same-height cases must have
the expected deterministic fields. The routine also rejects identical trusted
and conflicting header hashes and checks the time ordering of a forward attack.
The evidence pool provides the local-history context and validates metadata;
calling this routine in isolation does not perform all pool checks.

#### **[LCAI-FUNC-NONVALID.1]**

The draft's `violatesTMValidity` classification is implemented as
`ConflictingHeaderIsInvalid`. Its compared fields are:

- `ValidatorsHash`
- `NextValidatorsHash`
- `ConsensusHash`
- `AppHash`
- `LastResultsHash`, when `SkipLastResultsHashValidation` is false

These comparisons are against a trusted reference, not a general application
execution proof. The last-results exception describes existing behavior; it is
not a recommendation to bypass validation.

The draft's `IsolateAmnesiaAttacker` is not an implemented complete detector.
The different-round branch returns no accused validators, and `ValidateABCI`
expects empty attribution in that case. Research about locking histories in
[fork accountability](../accountability/README.md#flip-flopping-amnesia-based-attacks)
does not change that behavior.

`RoundOf`, `Signers` and `Addresses` were explanatory draft helpers rather than
current evidence API declarations. A round is `Commit.Round` (an integer), not a
list of validator addresses. Signer extraction must distinguish block votes
from nil/absent votes and use the authenticated validator set.

## Application handoff

`GenerateABCI` populates timestamp, total voting power and accused validators
from the verified reference data. `ValidateABCI` recomputes the expected
attribution and checks the supplied metadata. `ABCI()` emits one
`MisbehaviorType_LIGHT_CLIENT_ATTACK` entry per accused validator.

This conversion does not itself slash a validator or define an unbonding
policy. An empty amnesia attribution produces no accused-validator entries.
Keep evidence verification and application policy separate when assessing the
scope of these classic protocol mechanisms.

# Part V - Completeness

The retained classification distinguishes lunatic, same-round equivocation and
different-round amnesia attacks. Equal deterministic validator hashes allow the
same-round branch to compare the corresponding validator positions. That
observation does not prove completeness of attribution for all executions,
membership changes or alternative consensus protocols.

The [TLA+ synopsis](../accountability/Synopsis.md) documents a one-height,
equal-weight model with finite experiments. The [Ivy proofs](../../ivy-proofs/README.md)
have explicit quorum and refinement assumptions and a legacy harness whose
pipeline can lose checker failure status. Neither artifact proves that the
current Go node has a complete amnesia detector, and neither covers Autobahn
certificates or persistent recovery. No model checker was executed for this
documentation rewrite.

# References

- [Classic light-client verification draft](../verification/verification_002_draft.md).
- [Fork accountability and implementation boundary](../accountability/README.md).
- [TLA+ modules and historical experiment limitations](../accountability/Synopsis.md).
- [Ivy models and checker-status requirements](../../ivy-proofs/README.md).
- [Current consensus state and recovery](../../core/state.md).
- [Current evidence types and attribution](../../../types/evidence.go).
- [Current evidence verification](../../../internal/evidence/verify.go).
