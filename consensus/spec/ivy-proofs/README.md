# Ivy Proofs

```copyright
Copyright (c) 2020 Galois, Inc.
SPDX-License-Identifier: Apache-2.0
```

## Applicability

These are inherited **classic Tendermint** research models and proof artifacts.
They model rounds, prevotes, precommits and locking. They do not model the whole
Paxeer application, its in-process `FinalizeBlock`/`Commit` boundary, or Autobahn
certificates, availability, committee configuration and crash recovery.

Paxeer mainnet-beta is one BFT network run by `paxd`, with EVM chain ID 125. The
kernel participates in the network and proposes blocks; committed blocks give
transaction receipts finality. The proofs here are not a conformance claim for
that whole implementation. Autobahn's current types and persistence are in
[`autobahn/types`](../../autobahn/types/) and
[`internal/autobahn`](../../internal/autobahn/). See also
[State](../core/state.md), [fork accountability](../light-client/accountability/README.md),
and the [TLA+ synopsis](../light-client/accountability/Synopsis.md).

## Contents

| File | Model or property |
| --- | --- |
| [`tendermint.ivy`](tendermint.ivy) | Classic protocol following *[The latest gossip on BFT consensus](https://arxiv.org/abs/1807.04938)* by Buchman, Kwon and Milosevic. |
| [`abstract_tendermint.ivy`](abstract_tendermint.ivy) | Abstraction used to make verification tractable. |
| [`classic_safety.ivy`](classic_safety.ivy) | Agreement assuming every two quorums intersect in a well-behaved node. |
| [`accountable_safety_1.ivy`](accountable_safety_1.ivy) | Under the assumption that every quorum includes a well-behaved node, disagreement exposes misbehavior in a quorum intersection (the familiar `f+1` bound under the model's quorum interpretation). |
| [`accountable_safety_2.ivy`](accountable_safety_2.ivy) | Well-behaved nodes cannot be framed by the modeled evidence, without a quorum-honesty assumption. |
| [`network_shim.ivy`](network_shim.ivy) | Network model and shim for the protocol. |
| [`domain_model.ivy`](domain_model.ivy) | Rounds, values, node sets and quorums. |

All models use [Ivy](https://github.com/kenmcmil/ivy). The license above applies
to the files in this directory. Quorum assumptions and refinement relations in
the models are part of each result; they must not be replaced with assumptions
about a different consensus mode.

## Building and running

The retained [`Dockerfile`](Dockerfile) uses Debian Buster, Python 2.7 and Ivy
revision `271ee38980699115508eb90a0dd01deeb750a94b`.
[`docker-compose.yml`](docker-compose.yml) mounts the models read-only and
`output` writable. This is a legacy research environment: the operating-system
packages and Python dependencies are not fully pinned, and a successful image
build alone establishes no proof result.

The entry point is `check_proofs.sh`, invoked by the `tendermint-proof` Compose
service. It preserves each checker and `tee` exit status, captures stdout and
stderr in a unique private directory under `output`, and requires a successful
log read with a final line of `OK`. All three proofs run; any checker, logging
or result failure makes the script exit nonzero. Output-directory creation
failure stops the run before any checker starts.

For a research rerun in a provisioned environment, execute each checker directly,
capture stderr as well as stdout, and require both a successful exit and its
expected final marker. For example, from this directory with `ivy_check` installed:

```bash
set -euo pipefail
proof_log_dir=$(mktemp -d output/manual-XXXXXX)
proof_seed=12345
ivy_check seed="$proof_seed" classic_safety.ivy > "$proof_log_dir/classic_safety.txt" 2>&1
test "$(tail -n 1 "$proof_log_dir/classic_safety.txt")" = OK
ivy_check seed="$proof_seed" accountable_safety_1.ivy > "$proof_log_dir/accountable_safety_1.txt" 2>&1
test "$(tail -n 1 "$proof_log_dir/accountable_safety_1.txt")" = OK
ivy_check seed="$proof_seed" complete=fo accountable_safety_2.ivy > "$proof_log_dir/accountable_safety_2.txt" 2>&1
test "$(tail -n 1 "$proof_log_dir/accountable_safety_2.txt")" = OK
```

Create the writable `output` directory first if it does not exist. Record the
source revision, Ivy revision, dependency/image versions, seed, exact commands,
individual exit statuses and full private logs. Do not infer checker success
from an `OK` line alone or from archived output. No Ivy checker was run for this
documentation rewrite; the documentation gate does not check these proofs.
