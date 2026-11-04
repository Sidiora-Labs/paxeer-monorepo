# Synopsis

This directory retains a TLA+ model of **classic Tendermint** fork accountability.
It is a research abstraction of the protocol in
*[The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)*, rather
than an executable specification of the merged Paxeer node.

Paxeer mainnet-beta is one BFT network (`paxd`, EVM chain ID 125), with the kernel
participating in block proposals. Receipts are final when their blocks commit.
These TLA+ modules do not model the application's
[`FinalizeBlock`/`Commit` lifecycle](../../core/state.md#execution), nor Autobahn
certificates and persistent recovery. For the implementation boundary, see
[fork accountability](README.md#implementation-boundary) and
[attack isolation](../attacks/isolate-attackers_001_draft.md).

The model deliberately simplifies the protocol:

- One height is modeled: this is one-shot consensus with fixed membership.
- Safety is the focus; timeouts are nondeterministic and proposer selection has
  no fairness assumption. This is not a liveness proof.
- Faulty-process messages are injected into initial states.
- Every process has voting power one; cardinality arguments do not establish
  safety for an arbitrary weighted validator set.
- Hashes are modeled as identity; cryptography is abstracted.

The accountability goal is to expose at least `f+1` Byzantine processes under the
model's bounds when correct processes disagree, without accusing correct
processes. The modeled fault scenarios are equivocation within a round and
amnesia across rounds. This goal is distinct from what the current Go evidence
implementation can attribute: different-round attacks return no accused
validators.

# TLA+ modules

| Module | Purpose |
| --- | --- |
| [TendermintAcc_004_draft](TendermintAcc_004_draft.tla) | Protocol and state transitions. |
| [TendermintAccInv_004_draft](TendermintAccInv_004_draft.tla) | Inductive invariant and agreement/accountability theorem statements. |
| `MC_n<n>_f<f>`, such as [MC_n4_f1](MC_n4_f1.tla) | Finite instances with fixed correct/faulty sets, values and maximum round. `ConstInit` chooses the proposer nondeterministically. |
| [TendermintAccTrace_004_draft](TendermintAccTrace_004_draft.tla) | Restrictions to a chosen action sequence, useful for a counterexample trace. |
| [TendermintAccDebug_004_draft](TendermintAccDebug_004_draft.tla) | Definitions for inspecting executions with TLC and Apalache. |

The separate [Ivy models](../../ivy-proofs/README.md) study classic Tendermint
with their own abstractions and assumptions; they are not interchangeable with
these finite experiments.

# Reasoning about fork scenarios

The theorem names below occur in
[TendermintAccInv_004_draft.tla](TendermintAccInv_004_draft.tla).

1. `TypedInvIsInductive` combines initialization and preservation of `TypedInv`.
   Its disjunct `FaultyQuorum` makes no invariant claim when faulty processes
   alone reach `THRESHOLD2`.
2. `AgreementWhenLessThanThirdFaulty` states:

   ```tla
   LessThanThirdFaulty /\ TypedInv => Agreement
   ```

   Here `LessThanThirdFaulty` means `N > 3 * T` and
   `Cardinality(Faulty) <= T`.
3. `AgreementOrFork` states:

   ```tla
   ~FaultyQuorum /\ TypedInv => Accountability
   ```

   `Accountability` describes agreement or the modeled fork witnesses. It does
   not specify a deployed evidence-submission or punishment mechanism.

These are theorem statements in the source, not a record that a parameterized
proof was executed. The recorded Apalache experiments check initialization,
one-step induction and implications separately for finite instances. A
parameterized proof would require a separate argument or theorem prover.

# Model checking results

The [historical report](results/001indinv-apalache-report.md) records `NoError`
for twelve experiments. Their exact inputs are in
[`001indinv-apalache.csv`](001indinv-apalache.csv): four- and five-process
instances with one or two faulty processes, lengths zero or one, and
`--cinit=ConstInit`. Other instance files in the directory are not evidence
that those instances were included in that report.

The retained [`run.sh`](run.sh) is **not a portable reproduction command**. It
sets `SCRIPTS_DIR=~/devl/apalache-tests/scripts`, `BUILDS=unstable`,
`BENCHMARK=001indinv-apalache`, and `RUN_SCRIPT=./run-all.sh`, then loads
`~/devl/apalache-tests/Makefile.common` with `make -e`. That external harness and
a pinned Apalache build are not supplied here. An environment override cannot
replace a value that the script itself exports. There is no documented,
repository-provided configurable harness path in this version.

A reproduction must first provision and identify compatible Apalache and
experiment-harness revisions, then run the full CSV through that harness with
its actual paths. Preserve the finite constants, initialization predicates,
invariants and lengths. Record the repository revision, tool and harness
revisions, exact invocations, exit codes and complete private logs; do not treat
the historical `unstable` label as a version pin. No model checker was executed
for this documentation rewrite, and the documentation gate does not reproduce
these results.
