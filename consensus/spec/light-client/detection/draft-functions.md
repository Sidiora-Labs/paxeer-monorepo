# Draft of Functions for Fork Detection and Proof of Fork Submission

## Status and applicability

This page retains the useful contracts from an upstream Tendermint/IBC design
draft: find a common trust root between two sparse stores, connect both
verification traces to it, and submit evidence the receiving client can verify.
The named functions below are research proposals, not exported Go APIs or
JSON-RPC methods. The [detector model](detection_003_reviewed.md) supplies their
Tendermint trust, validator and timing assumptions; the
[discussion decisions](discussions.md) explain why conflicting headers alone
are insufficient and why proof minimization must not require a faulty primary.

Paxeer is one BFT network with its kernel inside the network participating in
block proposal. Its EVM chain ID is 125 (`0x7d`), and a receipt is final once
its block is committed. These proposals concern an observer verifying a chain
and an IBC client verifying counterparty headers. They do not introduce another
execution domain or settlement stage.

Classic [`LightBlock`](../../../types/light.go) and
[`LightClientAttackEvidence`](../../../types/evidence.go) are distinct from
Autobahn's [`LaneQC`](../../../autobahn/types/lane_qc.go) and
[`CommitQC`](../../../autobahn/types/commit_qc.go). Setting `autobahn-config-file`
in [`config/config.go`](../../../config/config.go) selects that mode. The
consensus handlers under [`internal`](../../../internal/) refuse classic commit
retrieval (`rpc/core/blocks.go`, `ErrCommitNotAvailable`) and evidence submission
(`rpc/core/evidence.go`, `ErrEvidenceNotAccepted`) under Autobahn. Neither this
draft nor the classic IBC client defines certificate conversion or Autobahn
restart/recovery; consensus persistence lives under
[`internal/autobahn/consensus/persist`](../../../internal/autobahn/consensus/persist/).

- [On-chain IBC component](#on-chain-ibc-component)
- [Relayer](#relayer)

## On-chain IBC Component

### Current implemented interfaces

The Tendermint IBC client uses
[`Misbehaviour`](../../../../interchain/modules/light-clients/07-tendermint/types/misbehaviour.go),
which carries `ClientId`, `Header1` and `Header2`. Each header includes its own
trusted height and trusted validators. It does not accept a
`LightNodeProofOfFork` containing arbitrary-length traces.

[`ClientState.CheckMisbehaviourAndUpdateState`](../../../../interchain/modules/light-clients/07-tendermint/types/misbehaviour_handle.go)
checks both headers against their stored trusted consensus states. Equal-height
headers must have different block hashes. Unequal heights are handled as a
violation of monotonic BFT time, with `Header1` at the greater height. Successful
misbehaviour handling sets `FrozenHeight` to the sentinel height `{0, 1}`.
It does not compute the minimum of an existing frozen height and the root's
height. Invalid types, unavailable trusted states, expired trust and invalid
headers can return errors; the old draft's “error condition: none” is not an
implementation contract.

The client keeper's
[`grpc_query.go`](../../../../interchain/modules/core/02-client/keeper/grpc_query.go)
implements `ConsensusState`, `ConsensusStates` and `ConsensusStateHeights`.
The listing queries are paginated. They return stored consensus states or
heights, not all historical light blocks or a proof that any two headers
share a trust path.

#### [TAG-IBC-MISBEHAVIOR.1]

Historical proposal: `checkMisbehaviourAndUpdateState(cs, PoF)`.

The proposal assumes a proof object with a `TrustedBlock`, `PrimaryTrace` and
`SecondaryTrace`. The trusted root must match a consensus state already known
to the receiver. Both traces must verify from that root at the receiver's
current time, respecting the trusting period and validator rules, and end in
different headers at the same height. The intended outcome is to freeze the
client on verified misbehaviour.

This remains a trace-based research contract. Implementations must use the
actual `Misbehaviour` type and validation path above; neither the proposed
proof shape nor the old `min(cs.FrozenHeight, root.Height)` rule describes the
current receiver.

#### [TAG-IBC-HEIGHTS.1]

Historical proposal: `QueryHeightsRange(id, from, to) -> heights`.

The proposed postcondition is to return exactly the stored consensus-state
heights `h` for client `id` such that `from <= h <= to`. This is useful for a
late-joining relayer that has not observed prior header updates.

The keeper instead exposes the paginated `ConsensusStateHeights` query. A
consumer implementing the proposed range operation would need to follow
pagination and apply the range bounds using IBC revision heights. The proposal
does not establish a new endpoint. Events can help discover updates, but do not
replace checking what remains in the receiver's store.

## Relayer

### Auxiliary Functions to be implemented in the Light Client

#### [LCV-LS-FUNC-GET-PREV.1]

Historical proposal: `LightStore.GetPreviousVerified(height)`.

Return the locally verified light block with maximal height strictly smaller
than `height`, together with a found/not-found result. Absence must be explicit;
it cannot be replaced with an unverified block or an invented root.

The implemented [`store.Store`](../../../light/store/store.go) offers
`LightBlockBefore(height)` over stored light blocks and returns an error on
failure. It has neither `GetPreviousVerified` nor the research four-state
store API described in [the discussion](discussions.md#semantics-of-the-lightstore).
A caller must respect `LightBlockBefore`'s documented height bounds and the
client's trusted-store semantics.

### Relayer Submitting Proof of Fork to the IBC Component

A relayer may discover a conflict through its light client or by comparing
its verified view with consensus states installed in the IBC client. Because
both stores may be sparse, possession of a local proof does not imply the
receiving client already has the necessary trusted state.

#### [TAG-SUBMIT-POF-IBC.1]

Historical proposal: `SubmitIBCProofOfFork(lightStore, PoF, ibc)`.

The proposed algorithm is:

1. Query the receiving client's consensus state at the proposed root height
   and check that it matches the root needed by the proof.
2. If it matches, submit evidence supported by that state.
3. Otherwise, find a verifiable common root with `commonRoot`, and connect
   the local traces to it with `extendPoF`.
4. If no usable common root exists, report that evidence could not be
   constructed. A mismatch or query failure is not successful submission.

This describes the trust-path problem, not an implemented relayer entry point.
For the actual IBC client, evidence must be encoded as its two-header
`Misbehaviour` and pass its validation against stored trusted states. Reporting
success requires the receiving path's result; generating a local proof alone
cannot freeze an on-chain client.

### Auxiliary Functions at the Relayer

#### [TAG-COMMON-ROOT.1]

Historical proposal:
`commonRoot(lightStore, ibc, target) -> (receiverRoot, connector, auxiliaryStore, result)`.

Search receiver-known heights below the local target, starting with the
highest candidate in the usable local range. For each candidate, retrieve the
receiver's consensus state and try to connect it to a locally verified block
below the target. If the connection verifies, return the receiver root, the
local connector and the retrieved verification material. If candidates are
exhausted, return an explicit no-root result.

A common numeric height alone is insufficient: the header identity, validator
information and trust window must support the connection. The original
`QueryHeightsRange` call is the proposal above, not a method implemented by
the keeper. The receiver stores consensus states, not full `LightBlock`
objects; constructing a usable connector needs the corresponding authenticated
header and validator data as well.

#### [TAG-LS-FUNC-CONNECT.1]

Historical proposal: `Connector(lightStore, root, targetHeight)`.

Return a locally verified block below `targetHeight` that can be verified from
`root` in one step, or an explicit failure. The proposal requires a chain of
trust from this connector to the target. A store containing independently
verified blocks does not by itself prove such a chain; missing intermediate
verification material must be obtained and verified before constructing the
proof.

#### [TAG-EXTEND-POF.1]

Historical proposal: `extendPoF(root, connector, lightStore, PoF)`.

The original trace construction is retained as notation:

```text
prefix = connector
       + verified subtrace after connector and before PoF.TrustedBlock
       + PoF.TrustedBlock
newPoF.TrustedBlock = root
newPoF.PrimaryTrace = prefix + PoF.PrimaryTrace
newPoF.SecondaryTrace = prefix + PoF.SecondaryTrace
```

Here `+` means concatenation with increasing heights and without duplicating
shared endpoints. Every step must verify, including `root` to `connector` and
the junction with the original trusted block. Copying records into a list does
not establish a trust path. The resulting trace object remains a research
representation; it is not automatically accepted by the implemented two-header
IBC misbehaviour handler.

<a id="detection-a-fork-at-the-ibc-component"></a>

### Detection of a fork at the IBC component

#### [TAG-HANDLER-DETECT-FORK.1]

Historical proposal: `DetectIBCFork(ibc, lightStore) -> (proof, result)`.

The proposal polls the receiver's installed consensus state, or reacts to an
observed update, and compares it with a locally verified header at the same
height. If the local block is absent, verify it through the light client before
comparison. Failure to fetch or verify is an error, not a no-fork result.
If compatible authenticated data agrees, there is no conflict at that height.
If it differs, establish the root and both supporting paths before producing
evidence. Raw comparison of a client-state object with a light-block object is
not meaningful; compare the corresponding authenticated consensus fields.

This routine is not implemented by the draft. It must not assume a remote
receiver is correct merely because a query succeeded. Pruned states, expired
trust, incomplete pagination and a missing common root can prevent proof
construction. Such failures do not establish either a fork or its absence.
The current receiver's validation rules, rather than this outline, decide
whether submitted IBC misbehaviour is accepted.
