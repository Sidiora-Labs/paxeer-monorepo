# Proposer-Based Timestamps (PBTS)

This directory retains the upstream Tendermint PBTS design and its versioned
research drafts. It also maps that design to the timestamp checks in the
embedded Paxeer consensus engine. The drafts are not a qualification result
for the running network or a specification of Autobahn.

Paxeer is one BFT network run by `paxd` (EVM chain ID 125). The kernel is part
of the network and proposes blocks. A receipt is final once its block is
committed; the clock assumptions discussed here do not add a finality stage.

## Current runtime

The node setting `autobahn-config-file` selects the implementation: empty uses
classic consensus, nonempty enables Autobahn. This describes the code's mode
selection, not an assertion about a particular deployed node. See
[node assembly](../../../node/node.go) and [configuration](../../../config/config.go).

| Implementation | Timestamp source and validation | Relationship to these drafts |
| --- | --- | --- |
| Classic consensus | `State.MakeBlock` uses the local clock. For kernel-ordered heights, `CreateProposalBlock` uses the sealed batch's `OpenTime` through `MakeBlockAt` and rejects a time that is not after `LastBlockTime`. `Proposal.IsTimely` checks the proposal reception time before an unlocked validator prevotes for a fresh proposal. | Implements the second draft's definitive block time and reception-window idea, with the runtime details below. |
| Autobahn | `NewProposal` uses the leader's clock, raises it to the minimum allowed by the previous certificate or genesis, and assigns successive global blocks one-microsecond offsets. `FullProposal.VerifyAt` checks monotonicity and a future-time bound for fresh proposals. | Uses different proposals, certificates and time admission rules. The classic PBTS predicate and proofs do not establish Autobahn time validity. |

For the classic path, the authoritative code is
[block construction](../../../internal/state/state.go),
[block execution](../../../internal/state/execution.go),
[consensus state](../../../internal/consensus/state.go),
[block validation](../../../internal/state/validation.go) and
[`Proposal.IsTimely`](../../../types/proposal.go).

For Autobahn, use [proposal types and verification](../../../autobahn/types/proposal.go),
[proposal admission before prepare voting](../../../internal/autobahn/consensus/inner.go),
[proposal creation](../../../internal/autobahn/consensus/state.go),
[global-block assembly](../../../internal/autobahn/data/state.go) and
[application execution](../../../internal/p2p/giga_router.go).

### Classic reception window

The consensus parameters are `SynchronyParams.Precision` and
`SynchronyParams.MessageDelay`, represented as `synchrony.precision` and
`synchrony.message_delay` in consensus-parameter JSON (string-encoded durations
in nanoseconds). [DefaultSynchronyParams](../../../types/params.go) supplies
505 milliseconds and 12 seconds respectively; zero values are filled from
those defaults by `SynchronyParamsOrDefaults`. Validated parameters require
both values to be positive. These are consensus parameters, not Autobahn
configuration keys.

For proposal timestamp `ts` and recorded reception time `t`,
`Proposal.IsTimely` accepts the inclusive window:

```text
ts - Precision <= t <= ts + effectiveMessageDelay + Precision
```

The message-delay allowance doubles every ten rounds, with the shift capped
before duration overflow. The second research draft uses a fixed `MSGDELAY`;
this round-based adaptation is an implementation detail beyond that model.

`defaultSetProposal` records the first accepted proposal's reception time for
the current height and round. `defaultDoPrevote` requires the proposal timestamp
to equal the block time, and rejects an untimely proposal with a nil prevote
when `POLRound == -1` and `LockedRound == -1`. A proposal backed by an earlier
proof of lock, or a validator already locked, does not take that fresh-proposal
window check. Other block, signature, locking and application checks still
apply. Block validation requires strictly increasing time after the initial
height; the first block may equal genesis time.

### Autobahn time admission

`ViewSpec.NextTimestamp` supplies genesis time for the initial proposal or the
previous committed proposal's next timestamp. `Proposal.BlockTimestamp` adds
one microsecond for each block's offset within the proposal's global range;
`NextTimestamp` advances past that range and reports offset overflow.

Fresh proposal creation and `FullProposal.VerifyAt` check that even the last
block's timestamp is no later than the local clock plus
`MaxFutureProposalDrift`, currently one second. An empty proposal checks its
base timestamp. Failure returns `ErrFutureProposalTime`. Verification occurs
before a prepare vote is signed.

A reproposal justified by a verified timeout certificate carrying a prepared
proposal must match the expected proposal rebuilt for the new view. The
[timestamp, lane ranges and application state](../../../autobahn/types/timeout.go)
are preserved; the leader signs the proposal for the new view. That path
bypasses the fresh time check; historical certificate verification is
independent of the current clock. There is no classic `MSGDELAY` reception
window here. The one-second bound assumes sufficiently synchronized consensus
clocks; it is not a proof that committed timestamps have a specified absolute
error from UTC.

## Context

Historically, Tendermint's [BFTTime][bfttime] design used a weighted median of
precommit timestamps selected from the previous height's commit. That is
background for PBTS, not the timestamp algorithm used by the current paths
above. Clock agreement and accuracy relative to real time are different
assumptions: clocks can agree with each other while all being wrong about UTC.
The [first design draft][main_v1] discusses this motivation.

Block time is application-visible and affects time-dependent policies,
including classic [evidence expiry](../evidence.md#verification). Deterministic
agreement on a timestamp alone does not prove its accuracy.

## Proposal

In the retained second PBTS design, the original proposer assigns a timestamp
to the block. Other validators evaluate its reception against two assumptions:

- **Synchronized clocks:** simultaneous reads at correct validators differ by
  at most `PRECISION`.
- **Bounded proposal delivery:** a proposal from a correct proposer reaches
  correct validators within `MSGDELAY`.

A fresh proposal timestamp `ts` received at local time `t` is timely when
`ts - PRECISION <= t <= ts + MSGDELAY + PRECISION`. The left bound limits
future timestamps; the right bound limits old timestamps relative to reception.
These are conditional research assumptions, not measured network guarantees.
The [system model][sysmodel] states the clock and delay properties precisely.

## Implementation

The retained [second algorithm draft][algorithm] modifies the Tendermint
[paper algorithm][arXiv] as follows:

- A newly created block includes its original proposer's time. Reproposing the
  same block preserves that time.
- A proposer waits if needed to keep time increasing over heights.
- A validator records proposal reception and uses the timeliness check before
  prevoting for a fresh value.
- Reproposals with an earlier proof of lock reuse acceptance of the original
  value and its definitive timestamp.

The [first algorithm draft][algorithm_v1] instead allowed the same value to be
reproposed with a different time and changed vote identities accordingly.
That superseded design is retained for comparison, not as an implementation
recipe. The [current runtime](#current-runtime) section identifies actual
branches and exceptions in this tree; the pseudocode does not replace them.

## Further details

- [System Model and Properties, second research draft][sysmodel]
- [Protocol Specification, second research draft][algorithm]
- [System Model and Properties, superseded first draft][sysmodel_v1]
- [Protocol Specification, superseded first draft][algorithm_v1]
- [First TLA+ model][proposertla] (historical first draft)
- [Second TLA+ model][proposertla_v2] (preliminary research specification)

### Open issues

These are the upstream discussions listed by the historical proposal, not a
current status list or a runtime qualification checklist for Paxeer:

- [PBTS: evidence #355][issue355]
- [PBTS: should synchrony parameters be adaptive? #371][issue371]
- [PBTS: Treat proposal and block parts explicitly in the spec #372][issue372]
- [PBTS: margins for proposal times assigned by Byzantine proposers #377][issue377]

### Closed issues

The upstream proposal recorded these discussions as closed:

- [Proposer time - fix message filter condition #353][issue353]
- [PBTS: association between timely predicate and timeout_commit #370][issue370]

[main_v1]: ./v1/pbts_001_draft.md
[algorithm]: ./pbts-algorithm_002_draft.md
[algorithm_v1]: ./v1/pbts-algorithm_001_draft.md
[sysmodel]: ./pbts-sysmodel_002_draft.md
[sysmodel_v1]: ./v1/pbts-sysmodel_001_draft.md
[proposertla]: ./tla/TendermintPBT_001_draft.tla
[proposertla_v2]: ./tla/TendermintPBT_002_draft.tla
[bfttime]: ../bft-time.md
[arXiv]: https://arxiv.org/pdf/1807.04938.pdf
[issue353]: https://github.com/tendermint/spec/issues/353
[issue355]: https://github.com/tendermint/spec/issues/355
[issue370]: https://github.com/tendermint/spec/issues/370
[issue371]: https://github.com/tendermint/spec/issues/371
[issue372]: https://github.com/tendermint/spec/issues/372
[issue377]: https://github.com/tendermint/spec/issues/377
