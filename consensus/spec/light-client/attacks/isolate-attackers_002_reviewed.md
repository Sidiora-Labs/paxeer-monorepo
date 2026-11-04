<!-- markdown-link-check-disable -->

# Lightclient Attackers Isolation

This page retains the Tendermint attacker-isolation model and maps it to the
classic evidence implementation in this tree. Its trust, validator and timing
assumptions are essential. It is not a specification of Autobahn certificates,
validator punishment or recovery.

Paxeer mainnet-beta is one BFT network run by `paxd`, with EVM chain ID 125
(`0x7d`). The kernel is part of the network and proposes blocks; a receipt is
final when its block is committed. The research model below does not introduce
another finality stage.

## Current-mode applicability

The node configuration key `autobahn-config-file` selects Autobahn when non-empty
([configuration][config], [node assembly][node]). An empty value leaves Autobahn
disabled. Determine the node's consensus mode before applying this model.

| Concern | Implementation and limit |
| --- | --- |
| Classic light-client conflict | [The detector][detection] checks primary and witness traces; a disagreement alone is not proof against a validator. |
| Evidence validation | [`VerifyLightClientAttack`][evidence-verifier] checks signatures and a conflict against the receiving full node's history. |
| Validator attribution | [`LightClientAttackEvidence.GetByzantineValidators`][evidence-type] implements the lunatic and same-round equivocation cases. Different-round amnesia returns no validators. |
| Application notification | The classic [block executor][executor] passes committed evidence as `RequestFinalizeBlock.ByzantineValidators` to the in-process application. Evidence submission alone does not punish anyone. |
| Autobahn | [`BroadcastEvidence`][evidence-rpc] returns `coretypes.ErrEvidenceNotAccepted`; this classic isolation path is not an Autobahn evidence API. |

Autobahn uses committee-verified `CommitQC` and `FullCommitQC` objects in
[`autobahn/types/commit_qc.go`][commit-qc]. Its committee weights, signed messages
and [persisted consensus state][persisted-state] have their own contracts. A
classic commit, trusting period or isolation argument cannot be substituted for
those contracts.

# Outline

[Part I](#part-i---basics-and-definition-of-the-problem) states the retained
Tendermint problem. [Part II](#part-ii---protocol) explains the implemented
validation and attribution path. [Part III](#part-iii---completeness) states
the limits of the upstream completeness argument. [Evidence handling][handling]
covers construction, transport and the unimplemented amnesia monitor proposal.

# Part I - Basics and Definition of the Problem

Under the classic Tendermint model, more than two thirds of the voting power
at each relevant height follows the consensus protocol. Light-client trust also
requires a trusted starting block that has not expired and sufficient overlap
with its validator set. A conflicting header that passes verification can expose
a violation of these assumptions. Full nodes serving data and validators signing
votes are different roles: attribution concerns signed validator messages, not
the peer identity that relayed them.

The [verification research][verification] defines light blocks and trust
conditions. The actual [`types.LightBlock`][light-type] contains a signed header
and its validator set. Basic validation checks their internal consistency;
cryptographic verification is a separate step.

The detector's abstract evidence consists of a conflicting light block and a
common height. The actual [`LightClientAttackEvidence`][evidence-type] also carries
`ByzantineValidators`, `TotalVotingPower` and `Timestamp`. See the
[complete shape][handling-shape]; the two-field research abstraction is not the
wire type.

#### **[LCAI-INV-Output.1]**

The retained research goal is to isolate validators without accusing a correct
validator. Given validated conflicting evidence, a canonical chain prefix, and
the required unbonding and voting-power assumptions, the desired set represents
more than one third of the voting power of a relevant bonded validator set.
Invalid evidence must be rejected before attribution.

This goal is conditional, rather than a guarantee for every accepted evidence
object. In particular, the implementation cannot attribute amnesia from the
available evidence. The original research also assumed a synchronized full node
with the block after the conflicting height so that its `LastCommit` was
available. The current verifier loads the header and `LoadBlockCommit(height)`
directly in `getSignedHeader`; it does not require the next header as the API for
obtaining that commit.

The research unbonding assumption protects accountability while validators can
still be held responsible. The implementation uses the consensus evidence age
parameters described in [evidence handling][handling-validation]. These are not
an unconditional assertion that every returned validator is currently bonded;
application policy remains separate.

# Part II - Protocol

## Isolation

### Outline

Validation comes before attribution. The receiving full node supplies its own
canonical headers and validator sets. It does not trust the submitted list of
accused validators. The code first distinguishes an invalid deterministic header
(the research term is **lunatic**), then conflicting commits in the same round
(**equivocation**), then conflicting commits in different rounds (**amnesia**).

#### **[LCAI-FUNC-MAIN.1]**

The upstream name `isolateMisbehavingProcesses` denotes the research operation;
it is not an exported function in this tree. The implementation is split between
`Pool.verify`, `VerifyLightClientAttack`, and
`LightClientAttackEvidence.GetByzantineValidators`:

1. Check basic consistency, evidence age and the full node's required history.
2. Verify the conflicting commit, the required common-set overlap and the
   conflict with the local chain.
3. Compute the attributable validators using the rules below.
4. Validate the supplied application metadata against that computation and the
   local block time and voting power.

Failures in the first two steps are errors, not an empty successful result.
An empty attribution for amnesia does not mean that no conflicting headers were
detected. The [detector][detection] can still stop the light-client update with
`ErrLightClientAttack`.

### Details of the Functions

#### **[LCAI-FUNC-VVU.1]**

`ValidAndVerifiedUnbonding` is a historical helper: it replaces the light
client's trusting-period bound with an unbonding-period bound. The current
evidence path instead uses `Pool.verify` for evidence age and
`VerifyLightClientAttack` for signatures and conflict checks. Do not call the
research helper as though it were an available API, or replace the pool's age
checks with light-client verification alone.

#### **[LCAI-FUNC-NONVALID.1]**

The historical `violatesTMValidity` predicate is implemented by
`LightClientAttackEvidence.ConflictingHeaderIsInvalid`. It compares the
conflicting header with the trusted header supplied by the full node.

#### **[LCAI-NONVALID-OUTPUT.1]**

The method returns true for a difference in any of `ValidatorsHash`,
`NextValidatorsHash`, `ConsensusHash` or `AppHash`. It also compares
`LastResultsHash` when `types.SkipLastResultsHashValidation` is false. This
qualification matters in this fork; the upstream unconditional five-field
predicate does not describe all runtime settings.

For this lunatic case, `GetByzantineValidators` includes only signatures with
`BlockIDFlagCommit` whose validator addresses belong to the common validator
set. It returns those validators ordered by voting power. It does not accuse
every member of the conflicting validator set.

If the deterministic header fields agree and both commits have the same round,
the method selects validators with `BlockIDFlagCommit` signatures in both
commits. Verified matching validator sets make the signature indexing meaningful.
The result is again ordered by voting power.

If the deterministic fields agree but the commit rounds differ, the method
returns an empty validator list. The upstream `IsolateAmnesiaAttacker` helper
and its query/response accountability monitor are research proposals, not
implemented functions or an on-chain service in this path. The proposal and its
additional vote-retention assumptions are retained in
[amnesia evidence handling][amnesia].

The research helper `RoundOf` means the integer `Commit.Round`; it does not
return validator addresses. `Signers` means only validators that actually signed
the block, excluding absent and nil votes. `Addresses` projects validator
addresses from a validator set. These conceptual helpers do not replace signature
verification or validator-set membership checks.

# Part III - Completeness

The retained Tendermint analysis distinguishes:

- **Lunatic:** signing a block with invalid deterministic header fields.
- **Equivocation:** signing conflicting blocks in the same consensus round.
- **Amnesia:** signing conflicting blocks in different rounds without the
  voting history required to justify the transition.

Once the validity comparison fixes the validator set, the upstream analysis can
consider one Tendermint consensus instance with fixed membership and two
conflicting values. The [TLA+ accountability model][tendermint-accountability]
and the [Ivy material][ivy] study equivocation and amnesia under that model.

This is a classification argument with Tendermint assumptions. It does not
prove that this fork identifies every attacker: the implemented amnesia branch
has no validator attribution. It also establishes no equivalence to Autobahn
lane certificates, committee-bound signatures, application certificates or
restart recovery. Those require their own implementation-specific reasoning.

# References

- [Light client attack detector][detection]: trust and trace model, current
  detector flow and errors.
- [Notes on evidence handling][handling]: full evidence type, receiving-node
  validation, application notification and amnesia limits.
- [Light client verification research][verification] and
  [supervisor research][supervisor]: retained upstream assumptions and algorithms.

[detection]: ../detection/detection_001_reviewed.md
[handling]: notes-on-evidence-handling.md
[handling-shape]: notes-on-evidence-handling.md#evidence-shape
[handling-validation]: notes-on-evidence-handling.md#receiving-node-validation
[amnesia]: notes-on-evidence-handling.md#amnesia-attack-evidence-handling
[verification]: ../verification/verification_002_draft.md
[supervisor]: ../supervisor/supervisor_001_draft.md
[tendermint-accountability]: ../accountability/README.md
[ivy]: ../../ivy-proofs/README.md
[config]: ../../../config/config.go
[node]: ../../../node/node.go
[light-type]: ../../../types/light.go
[evidence-type]: ../../../types/evidence.go
[evidence-verifier]: ../../../internal/evidence/verify.go
[evidence-rpc]: ../../../internal/rpc/core/evidence.go
[executor]: ../../../internal/state/execution.go
[commit-qc]: ../../../autobahn/types/commit_qc.go
[persisted-state]: ../../../internal/autobahn/consensus/persisted_inner.go
