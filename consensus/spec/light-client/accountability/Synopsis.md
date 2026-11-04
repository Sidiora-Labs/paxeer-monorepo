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

## Reproducing the full experiment

[`run.sh`](run.sh) accepts actual installation paths and pins
[informalsystems/apalache-tests](https://github.com/informalsystems/apalache-tests/tree/1b76b8e70549f6cd71a758b4014e7f7682d7fd44)
at commit `1b76b8e70549f6cd71a758b4014e7f7682d7fd44`. Provision that checkout
separately; the runner neither downloads nor builds dependencies. It calls that
revision's `scripts/mk-run.py` and `scripts/parse-logs.py` directly. The external
`Makefile.common` is intentionally avoided: at this revision it passes `--ini`,
whereas the generator accepts `--iniFile`.

Provision a built [Apalache](https://github.com/informalsystems/apalache)
checkout compatible with these historical TLA+ inputs and profiling/log formats.
Supply its **full lowercase 40-character commit** as `APALACHE_REVISION`;
`unstable`, branch names and abbreviated hashes are rejected. No known compatible
Apalache revision can be inferred from the historical `unstable` results. Tool
compatibility still requires an actual full experiment; the source gate does not
establish it. The executable `bin/apalache-mc` must be built from the supplied
revision. The runner checks checkout identity and cleanliness, but cannot prove
that ignored build artifacts correspond to their source; provisioning must do so.

Required prerequisites are Linux, a POSIX shell, Git, Python 3 (standard library),
Bash, Java appropriate to the selected Apalache build, GNU `/usr/bin/time`, GNU
`/usr/bin/timeout` and `mktemp`. Both external checkouts must be clean, including
untracked files (build outputs must be explicitly ignored). Commit the model and
runner first so that the recorded repository revision identifies their inputs.
The archive directory must already exist and be writable. Paths must use only
letters, digits, underscore, dot, slash, colon or hyphen: the pinned harness
generates unquoted shell paths. Neither environment credentials nor live
endpoints are needed. From any working directory, use, for example:

```sh
APALACHE_TESTS_DIR=/opt/research/apalache-tests \
APALACHE_DIR=/opt/research/apalache \
APALACHE_REVISION="$(git -C /opt/research/apalache rev-parse HEAD)" \
RESULTS_ROOT=/var/tmp/accountability-private \
sh /path/to/paxeer-monorepo/consensus/spec/light-client/accountability/run.sh
```

Select and build the intended Apalache commit before capturing its revision.
Every invocation uses all twelve rows of `001indinv-apalache.csv`, sequentially,
with unchanged constants, predicates, invariants, timeouts and lengths. The
[`experiment.py`](experiment.py) validator checks the exact benchmark set before
generation and requires exactly one `NoError` result for each case afterward;
batch-shell success alone does not establish successful experiments.

Each run creates a fresh private directory below `RESULTS_ROOT`, tagged with the
repository, harness and Apalache revisions. It retains `revisions.txt` (revisions,
paths and final exit code), the input CSV, `runner.log` (expanded invocations and
tool versions), generated scripts and all per-case raw logs under `runs/`, plus
the revision-tagged parsed result CSV. Failures retain the same evidence and
return nonzero. Keep the complete archive; historical files under `results/`
remain unchanged and are not relabeled as new results. No model checker was
executed for this repair, and its Python source gate does not reproduce the
historical results or certify the current node.
