# Validator Signing

Paxeer validators sign consensus messages inside the network run by `paxd`.
The kernel participates in that network and proposes blocks; a receipt is final
when its block is committed. This page distinguishes the classic proposal/vote
format from optional Autobahn signatures. The inherited classic rules remain
useful protocol references, but do not describe Autobahn certificates.

Use [proposer selection](proposer-selection.md) to identify the active leader
rule and [WAL recovery](wal.md) to identify the corresponding durable state.

## SignedMsgType

Classic messages use the Protobuf enum in
[`types.proto`](../../proto/tendermint/types/types.proto), whose generated Go
type is `SignedMsgType int32`:

| Message | Enum | Value |
| --- | --- | --- |
| Prevote | `PrevoteType` | `0x01` |
| Precommit | `PrecommitType` | `0x02` |
| Proposal | `ProposalType` | `0x20` |

The old description of this type as a Go byte alias does not match the current
Protobuf declarations. Autobahn uses typed message variants instead of this enum.

## Timestamp

Classic proposal and vote canonical bytes include a timestamp. Basic structural
validation does not enforce clock bounds, but the consensus state machine does
check proposal timeliness, timestamp equality with the block, and block-time
progression. See the [PBTS runtime checks](proposer-based-timestamp/v1/pbts_001_draft.md#classic-runtime-checks)
for the receive-time interval and its locked-state/reproposal conditions.

Classic votes use the validator's local signing time. Their timestamps are
still signed; they are not used to compute a weighted median timestamp for the
next block in the current block-building path. Do not infer a per-validator
strictly increasing timestamp check from the height/round/step signing rules.

## ChainID

The classic canonical message binds the consensus chain ID string passed to
`ProposalSignBytes` or `VoteSignBytes`. Genesis and header validation limit that
string to `MaxChainIDLen` (50 bytes), as defined in
[`genesis.go`](../../types/genesis.go). Signers must use the chain's configured
consensus ID. It is distinct from the public EVM chain ID, 125 (`0x7d`);
substituting the decimal EVM ID into consensus signing is not valid.

`FilePV` takes the chain ID from its caller. Its persisted last-sign state is
not a configuration mechanism for selecting a chain ID.

## BlockID

The current classic Go structure in [`block.go`](../../types/block.go) is:

```go
type BlockID struct {
    Hash          tmbytes.HexBytes
    PartSetHeader PartSetHeader
}
```

`BlockID.IsNil()` requires an empty hash and a zero part-set header (zero total
and empty part hash). Empty slices and nil slices both satisfy the empty-byte
checks. `BlockID.IsComplete()` requires a 32-byte block hash, a positive part
count and a 32-byte part-set hash. These predicates are separate from
`BlockID.ValidateBasic`, which checks the component encodings and can accept a
partial ID. Proposals require a complete ID; votes require either nil or
complete. The historical `PartsHeader` and `IsZero` spellings for `BlockID` are
not the current API.

## Proposals

[`Proposal.ValidateBasic`](../../types/proposal.go) requires:

- `Type == ProposalType`;
- `Height >= 0` and `Round >= 0`;
- `-1 <= POLRound < Round`;
- a structurally valid, complete `BlockID`.

The basic check accepts height zero; the live consensus state machine separately
requires the proposal to match its current height and round. A positive chain
height is not a stronger condition enforced by `ValidateBasic` itself.

The canonical signed proposal contains `Type`, `Height`, `Round`, `POLRound`,
`BlockID`, `Timestamp` and `ChainID`. The runtime proposal also carries headers,
transaction keys, evidence and commit information; do not substitute its JSON
encoding for canonical signing bytes. The receiving state machine verifies the
signature against the selected round leader and performs contextual validation.

## Votes

[`Vote.ValidateBasic`](../../types/vote.go) requires:

- a prevote or precommit type;
- `Height >= 0` and `Round >= 0`;
- a structurally valid `BlockID` that is either nil or complete;
- a validator address of `crypto.AddressSize` (20 bytes);
- `ValidatorIndex >= 0`.

Signature verification is separate: `Vote.Verify` checks the canonical bytes
with the supplied chain ID and validator public key. Basic structure alone
neither authenticates a vote nor proves that its sender belongs to the current
validator set.

The canonical signed vote contains `Type`, `Height`, `Round`, `BlockID`,
`Timestamp` and `ChainID`; it omits the validator address and index. The exact
schema is [`canonical.proto`](../../proto/tendermint/types/canonical.proto).
Both proposal and vote signing use varint-length-prefixed Protobuf via
`ProposalSignBytes` and `VoteSignBytes`, with canonicalization in
[`canonical.go`](../../types/canonical.go). Height and round use `sfixed64`;
proposal `POLRound` uses `int64`. The historical Go `binary` struct tags are not
the signing format.

## Invalid Votes and Proposals

Structural checks, signature checks and state-machine checks are distinct.
Applications and signers should not treat successful canonicalization or a
successful signing request as proof that a message satisfies all consensus
rules. Invalid peer messages can be rejected independently of evidence handling.
A protocol-valid signature does not make a wrong-height or wrong-leader proposal
acceptable.

## Double Signing

Conflicting signatures at the same consensus position are equivocation.
Classic [`FilePV`](../../privval/file.go) prevents regressions and conflicting
re-signing by persisting its last signed position and bytes. Evidence processing
and application consequences are separate from this local signer protection.

### State

`FilePVLastSignState` stores `Height`, `Round`, `Step`, `Signature` and
`SignBytes`. The internal steps are ordered as follows:

| Step | Value |
| --- | --- |
| No message | 0 |
| Proposal | 1 |
| Prevote | 2 |
| Precommit | 3 |

These are signer steps, not the `SignedMsgType` wire values. The signer rejects
a lower height, a lower round at the same height, or a lower step at the same
height and round. A higher height resets the round comparison; a higher round
resets the step comparison. Last-sign state is saved before a new signature is
returned. Preserve that state together with the validator key across restart.

### Proposals

At a new allowed position, the signer signs and persists the canonical proposal.
At the same height, round and proposal step, byte-identical requests reuse the
stored signature. Any change in those bytes, including a timestamp-only change,
is rejected as `conflicting data`. An identical retry is permitted; the old
blanket prohibition on signing a proposal twice at the same position was too
strong.

### Votes

The same position rules allow proposal → prevote → precommit, but never a
regression in that sequence. At the same vote position:

- identical canonical bytes reuse the stored signature;
- if only the timestamp differs, `FilePV` restores the saved timestamp and
  signature instead of signing again;
- other differences are rejected as `conflicting data`.

Nil votes count as signed votes. A nil vote cannot be replaced by a non-nil vote
at the same height, round and vote step. Restoring a stale or empty signer state
would defeat this protection; replaying the consensus WAL does not replace it.

### Other Rules

Classic locking and unlocking also depend on the observed prevote and precommit
quorums. The signer only remembers its last position and bytes; it cannot
independently establish that the consensus state machine had sufficient
justification to change a lock. The inherited discussion of “amnesia” concerns
this distinction. Use the [classic consensus paper](https://arxiv.org/abs/1807.04938)
for that research model, and [`state.go`](../../internal/consensus/state.go) for
this fork's actual transitions. Classic locking analysis is not an Autobahn
signer-state specification.

## Autobahn Signatures and Certificates

[`autobahn/types/msg.go`](../../autobahn/types/msg.go) defines typed lane
proposals/votes, application votes, road proposals, prepare votes, commit votes
and timeout votes. `Sign` requires a secret key bound with `ForCommittee`.
It signs a message hash with the `pax_giga_autobahn` Ed25519 tag and the preimage

```text
"pax/autobahn/signature/v1\x00" || committeeDomain || messageHash
```

`VerifySig` requires committee membership and verifies against the domain of
the committee supplied by the verifier. The domain binds the chain ID, initial
global block number, genesis timestamp, sorted keys and weights. It is not
provided by an untrusted signed message. A signature valid for another chain or
committee is not valid in this context.

[`Committee`](../../autobahn/types/committee.go) defines `Faulty = (W - 1) / 3`
for total weight `W`. Prepare, commit, timeout and application certificate
quorums require weight `W - Faulty`; lane availability certificates require
`Faulty + 1`. Certificate verification also checks signer uniqueness,
membership and agreement on the certified value. A lane certificate alone is
not a road commit certificate.

[`FullProposal.VerifyAt`](../../autobahn/types/proposal.go) verifies the road
leader, signature, view and global range, monotone timestamp, timeout
justification and fresh-proposal timeliness. A justified reproposal must match
the certified proposal. The [PBTS page](proposer-based-timestamp/v1/pbts_001_draft.md#autobahn-runtime-checks)
details this distinct timestamp path.

Autobahn node setup requires a local validator key and rejects a configured
remote signer (`priv-validator.laddr`), as enforced in
[`node.go`](../../node/node.go) and [`setup.go`](../../node/setup.go).
`FilePVLastSignState` is not Autobahn's road-vote recovery mechanism. The
consensus loop persists its votes and justifying certificates before publishing
vote outputs; its availability and data layers persist certified payloads and
prune anchors. Preserve all of the [Autobahn persistence](wal.md#autobahn-persistence)
state to maintain those protections across restart.
