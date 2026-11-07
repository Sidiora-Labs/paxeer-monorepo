---
order: 1
---
# Byzantine Consensus Algorithm

Paxeer mainnet-beta is one BFT network run by `paxd`, with EVM chain ID 125
(`0x7d`). The kernel is part of the network and proposes blocks. A transaction
receipt is final once its block is committed. The consensus protocols below are
node implementation modes, not separate execution and settlement networks.

Public EVM JSON-RPC uses the root path of the numbered hosts
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`,
and `api-mainnet-beta.paxeer.network`. ABCI is an
[in-process application API](../abci/client-server.md); it is not that public
JSON-RPC interface.

## Consensus modes

The [node assembly](../../node/node.go) selects Autobahn when the TOML
`autobahn-config-file` setting is nonempty. Otherwise it assembles the classic
consensus state machine. Read the matching row before applying a block, quorum,
timestamp, or recovery rule:

| Contract | Classic path | Autobahn path |
| --- | --- | --- |
| Ordering | One block per height, proposal/prevote/precommit rounds | Lane blocks ordered globally by certified proposals at an index and view |
| Identity | `types.BlockID`, block hash and part-set header | Lane header hash, lane-local number, global block number and proposal hash |
| Votes | Voting-power supermajorities of prevotes and precommits | Committee-bound lane, prepare, commit, timeout and application certificates |
| Time | Proposer timestamp, monotonic block time and reception-time checks | Proposal base time plus microsecond offsets, with a fresh-proposal drift bound |
| Application | `ProcessProposal`, then decided `FinalizeBlock` and `Commit` | Finalized global blocks delivered to `FinalizeBlock` and `Commit` |
| Recovery | Consensus WAL, block/state stores and application handshake | Persisted views/votes, indexed blocks/certificates and application committed height |

The classic sections retain useful Tendermint concepts, corrected for this
fork's current state machine. They do not specify Autobahn. The inherited
[consensus paper](consensus-paper/README.md) and formal models remain protocol
background, not a proof that every extension in this tree satisfies those models.

## Terms

- A *node* communicates with directly connected *peers*. Validators sign votes;
  other nodes can relay and validate data without an active validator key.
- Classic height `H` identifies the next block. A round `R` contains proposal,
  prevote, and precommit steps; `(H,R,S)` denotes height, round, and step.
- A vote signs its height, round, vote type, and block identity in the chain's
  signing domain. See [signing](signing.md).
- “More than two-thirds” means a strict supermajority of voting power, not a
  count of nodes. A nil vote records no chosen block; it cannot commit a block.
- A *proof of lock change* (PoLC, also called a polka) is a supermajority of
  prevotes for one block or nil at a height and round. `POLRound` in a proposal
  refers to a prior round, or is `-1` for a fresh proposal.
- Autobahn uses *lanes*, `(Index, Number)` views, and quorum certificates (QCs).
  Its global block number is distinct from a lane-local block number and from
  the index of the ordering proposal.

## State Machine Overview

The classic implementation is
[`internal/consensus/state.go`](../../internal/consensus/state.go). At each
height it tries rounds until it decides a block. An offline leader, invalid
candidate, delayed data, or missing quorum can require further rounds. Timeout
values grow with rounds; they are liveness mechanisms, not evidence that a block
is committed.

## State Machine Diagram

```text
NewHeight -> Propose -> Prevote -> Precommit -> Commit -> NewHeight
                ^                      |
                +---- next round ------+
```

This diagram omits catch-up transitions and wait steps. `Commit` requires a
supermajority of matching precommits and the actual block before execution.

## Background Gossip

Classic peers exchange proposals, block parts or transaction keys, votes, and
height/round/step information. Nodes behind the head can obtain older blocks
and commits through synchronization. Key-only proposal reconstruction is
conditional on `GossipTransactionKeyOnly`; a node must still obtain the missing
transaction bytes before validating and executing a block.

The local classic protocol code is in
[`internal/consensus`](../../internal/consensus/) and
[`internal/blocksync`](../../internal/blocksync/). Autobahn instead uses the
[internal giga peer API](../../internal/p2p/giga/api.go) to exchange lane
proposals, availability votes, ordering votes, certificates, and block data.
Peer protocol streams are internal node communication, not public EVM methods.

## Proposals

Classic [`RoundState.Leader`](../../internal/consensus/types/round_state.go)
uses the committed leader schedule when its `LeaderPin` applies at the current
height: round `R` selects `leaders[R % len(leaders)]`. Below activation, or
without a pin, a deterministic hash of the height and round selects a validator
with probability proportional to voting power. The old upstream weighted
round-robin description is not this fork's leader algorithm.

A signed proposal identifies its height, round, block ID, timestamp, and
`POLRound`. The proposer can reuse a previously valid block with its known
valid round to let other validators safely advance from older locks. See
[Creating a proposal](creating-proposal.md) for block contents and limits, and
[BFT Time](bft-time.md) for timestamp checks.

## State Machine Spec

### Propose Step (height:H,round:R)

The designated leader constructs and signs a proposal. A validator proceeds to
prevote once the proposal is complete (including required prior-round prevotes
when it cites `POLRound`), or when its proposal timeout expires. Missing or
invalid data cannot produce a positive prevote.

### Prevote Step (height:H,round:R)

`defaultDoPrevote` requires a reconstructed block, matching proposal/block time,
applicable timeliness checks, consensus block validation, and application
acceptance through `ProcessProposal`.

For a fresh proposal (`POLRound == -1`), a validator prevotes the proposal only
if it is unlocked or the proposal matches its locked block. For a proposal with
`0 <= POLRound < R`, it also needs the corresponding supermajority prevotes for
the proposed block; it may prevote that block if its locked round is no later
than `POLRound`, or its locked block already matches. Otherwise it prevotes nil.

A matching prevote supermajority permits precommit. A supermajority spread over
different values starts the prevote-wait timer; expiry permits a nil precommit.
The implementation does not first clear its lock just because a proposal cites
a different PoLC.

### Precommit Step (height:H,round:R)

Without a matching block prevote supermajority, the validator precommits nil.
A supermajority of nil prevotes also causes a nil precommit and leaves the lock
unchanged. This corrects the inherited description that nil prevotes unlock the
validator.

With a supermajority for a known valid proposal block whose timestamp matches,
the validator locks or relocks that block at round `R` and precommits it. If the
block is unavailable, it precommits nil while obtaining the data. A nil precommit
alone says nothing about whether the validator retains an older lock.

A supermajority of nil precommits permits the next round. A supermajority of
precommits for mixed values starts a precommit-wait timer; its expiry also
permits the next round.

### Common exit conditions

A supermajority of precommits for one block enters commit for that block.
Sufficient votes from a later round can move a lagging validator to that round's
corresponding steps. Such catch-up transitions do not bypass block validation
or locking rules.

### Commit Step (height:H)

A node records the commit round and waits for the committed block if needed.
It validates and saves the block, then applies it through the block executor:
`FinalizeBlock`, saved application response, `Commit`, and saved consensus state.
An application error interrupts local execution; it is not permission to choose
a conflicting block. See [Applications](../abci/apps.md#consensus-connection).

### NewHeight Step (height:H)

After applying the block, the node retains its commit, advances height, resets
round state, and schedules round zero. `Timeout.Commit` determines the normal
wait; `BypassCommitTimeout` can allow earlier progress once all precommits are
available. The activated internal sequencing configuration has its own timing
settings. Consult `proposeTimeout`, `voteTimeout`, `commitTime`, and
`bypassCommitTimeout` in the state machine rather than upstream timeout names.

## Autobahn protocol

The live types are in [`autobahn/types`](../../autobahn/types/) and the state
machine is in [`internal/autobahn/consensus`](../../internal/autobahn/consensus/).

### Lanes and global ordering

Each producer builds lane-local blocks. A header binds the lane, lane-local
number, parent header hash, and payload hash. A `LaneQC` certifies availability
of a lane prefix; it is not a global commit. An ordering proposal selects lane
ranges extending the previous commit certificate and may carry an application
proposal backed by an `AppQC`.

The elected committee leader signs the proposal for `(Index, Number)`.
Validators verify the view, leader signature, previous committed range,
justifications, time, and transport budgets. Prepare votes can form a
`PrepareQC`; commit votes form a `CommitQC` for the proposal. Timeout votes and
`TimeoutQC` justify advancing views. When the timeout justification carries a
prepared reproposal, its certified value must be preserved.

A `FullCommitQC` supplies the ordered headers needed to map the proposal into
global blocks. The data layer fetches the corresponding payloads, verifies their
header linkage, and exposes them in global execution order. An `AppProposal`
and `AppQC` bind the execution root of an already finalized global block. These
application certificates do not describe settlement on another network.

### Committee and quorum rules

[`Committee`](../../autobahn/types/committee.go) binds signatures to the chain
ID, configured replicas and weights, first global block, and genesis timestamp.
Leader election is deterministic and weighted using that committee domain and
the view. A signature from another committee is not valid merely because its
public key is familiar.

For total weight `N`, `Faulty()` returns `f = floor((N - 1) / 3)`.
`PrepareQuorum`, `CommitQuorum`, `TimeoutQuorum`, and `AppQuorum` are `N - f`;
`LaneQuorum` is `f + 1`. Availability and ordering certificates thus have
different thresholds. The supported committee has at most 16 replicas. The
configuration requires a local validator key; remote validator signers are not
supported in Autobahn mode.

Payload, proposal, and certificate bounds are detailed in
[Creating a proposal](creating-proposal.md#autobahn-proposals). The
[timestamp contract](bft-time.md#autobahn-timestamps) includes microsecond offsets,
the one-second future bound for fresh proposals, and certified reproposals.
Classic `LastCommit`, PoLC, block sizing, and validator-update timing must not be
substituted for these contracts.

### Autobahn recovery

The JSON [Autobahn configuration](../../config/autobahn.go) requires a nonblank
`persistent_state_dir` for durable operation. The explicit
`unsafe_test_only_disable_persistence` exception is for throwaway tests and is
mutually exclusive with a persistence directory. The persistence directory must
already exist and be writable.

Consensus persists view justification and its prepare, commit, and timeout votes
before broadcasting those votes. The
[A/B persistence implementation](../../internal/autobahn/consensus/persist/persist.go)
loads the highest valid sequence, can recover from one corrupt copy, and fails
if neither copy is usable. The data layer separately retains indexed blocks
and certificates for restart and synchronization. Keep these records together
with the application database; an application hash alone cannot reconstruct
lost vote-safety state or unavailable payloads.

Execution resumes from the application's durable `Info` height and hash through
the [Autobahn application recovery path](../abci/apps.md#autobahn-recovery), not
the classic block/state-store handshaker. The internal data path acknowledges
an execution hash only after its block and certificate are durable.

## Proofs

### Proof of Safety

For the classic protocol, two strict two-thirds voting-power quorums intersect
in more than one-third of the total power. With less than one-third Byzantine
power, a conflicting commit must therefore involve an honest validator violating
its voting/locking rules. Locks and justified later-round proposals prevent
that under the protocol assumptions. Correct signature domains, validator sets,
and durable vote records are part of those assumptions.

This is the classic safety argument, not a substitute for analyzing Autobahn's
committee certificates, availability threshold, and persisted view transitions.

### Proof of Liveness

Classic liveness assumes eventual network synchrony and sufficient honest
voting power online. Growing round timeouts eventually accommodate propagation;
a leader aware of a sufficiently recent valid round can propose a value that
honest locked validators can accept. Application rejection of otherwise valid
proposals, unavailable data, or unavailable quorum can halt progress. Bounded
block size and consistent application validation are necessary to this argument.

### Proof of Fork Accountability

The historical justification-vote-set (JSet) argument collects each validator's
votes and the PoLCs justifying its lock changes. Across conflicting commits,
complete justification records expose at least one-third culpable voting power
through conflicting votes or unjustified lock changes, under the classic model.
A commit's signatures alone need not contain every justification from earlier
rounds. Preserve votes and evidence rather than treating the JSet argument as an
implemented automatic blame mechanism. See [evidence](evidence.md).

### Alternative algorithm

The inherited “full commit” proposal required every commit to carry its complete
JSet so fork accountability could be checked immediately. That is historical
research, not this fork's commit format or a promised next release. In
particular, Autobahn's `FullCommitQC` is a different type and does not mean this
historical full JSet.

### Censorship Attacks

Enough withheld voting power can prevent a supermajority, halt progress, or
reject proposals containing particular transactions. Network partitions can
obscure who caused the loss of progress. Consensus safety does not guarantee
transaction inclusion or diagnose every liveness failure.

### Overcoming Forks and Censorship Attacks

The inherited discussion of externally coordinated reorganization proposals was
an operational research idea. This node does not implement the described
wallet prompts or automatic fork selection based on such proposals. A safety
failure requires external incident coordination and evidence review; it is not
resolved by the normal voting state machine.

### Canonical vs subjective commit

A classic validator can decide after observing one valid supermajority of
precommits. Different validators may initially observe different subsets for the
same block. The next block carries a particular valid previous commit in
`LastCommit`, giving all validators the same recorded subset. This distinction
is about the recorded votes, not competing finality levels. It is specific to
the classic block format; Autobahn records its own commit certificates.
