# Data Structures

This page describes the consensus types implemented in Paxeer's embedded
consensus engine. Paxeer mainnet-beta is one BFT network, with EVM chain ID 125
(`0x7d`); the kernel participates in the network and proposes blocks. A receipt
is final when its block is committed.

The types below are defined in [types](../../types/) and the
[protobuf schemas](../../proto/tendermint/types/). The `tendermint` protobuf
package names are retained on the wire. Classic consensus uses block parts,
prevotes and precommits. Autobahn has its own lane blocks, payloads, proposals
and quorum certificates in [autobahn/types](../../autobahn/types/) and
[autobahn.proto](../../internal/autobahn/autobahn.proto); the classic gossip
rules below do not specify that transport. See [Encoding](./encoding.md) for
serialization, hashing and signing.

Unless stated otherwise, field types are protobuf types. `ValidateBasic`
checks structure; it does not establish signature validity, quorum, membership
or agreement with chain state. Those checks happen at the relevant consumer.

- [Block](#block)
- [Execution](#execution)
- [Header](#header)
- [Version](#version)
- [BlockID](#blockid)
- [PartSetHeader](#partsetheader)
- [Part](#part)
- [Time](#time)
- [Data](#data)
- [Commit](#commit)
- [CommitSig](#commitsig)
- [BlockIDFlag](#blockidflag)
- [Vote](#vote)
- [CanonicalVote](#canonicalvote)
- [Proposal](#proposal)
- [SignedMsgType](#signedmsgtype)
- [Signature](#signature)
- [EvidenceList](#evidencelist)
- [Evidence](#evidence)
  - [DuplicateVoteEvidence](#duplicatevoteevidence)
  - [LightClientAttackEvidence](#lightclientattackevidence)
- [LightBlock](#lightblock)
- [SignedHeader](#signedheader)
- [ValidatorSet](#validatorset)
- [Validator](#validator)
- [Address](#address)
- [ConsensusParams](#consensusparams)
  - [BlockParams](#blockparams)
  - [EvidenceParams](#evidenceparams)
  - [ValidatorParams](#validatorparams)
  - [VersionParams](#versionparams)
  - [SynchronyParams](#synchronyparams)
  - [TimeoutParams](#timeoutparams)
  - [ABCIParams](#abciparams)
- [Proof](#proof)

## Block

[Block](../../proto/tendermint/types/block.proto) contains the following fields:

| Name | Type | Meaning and checks |
| --- | --- | --- |
| Header | [Header](#header) | Metadata and commitments; checked by `Header.ValidateBasic`. |
| Data | [Data](#data) | Ordered transactions. The application interprets their bytes. |
| Evidence | [EvidenceList](#evidencelist) | Validator misbehavior evidence; individual basic checks and the evidence hash are checked. |
| LastCommit | [Commit](#commit) | Commit for the preceding block; must be non-nil in the Go block. The initial block has no last-commit signatures. |

`Block.ValidateBasic` in [block.go](../../types/block.go) checks internal
consistency, including `LastCommitHash`, `DataHash` and `EvidenceHash`.
State validation additionally checks the previous block ID, versions, chain ID,
height, application and validator commitments, proposer membership, time and
evidence size. For a non-initial block, `LastValidators.VerifyCommit` verifies
the preceding commit against the expected height and block ID: one signature
slot per validator and strictly more than two-thirds of total voting power
signing that block. Nil votes do not count toward that quorum.

## Execution

The classic [BlockExecutor](../../internal/state/execution.go) validates a
block, calls the application's `FinalizeBlock`, saves the finalize response,
validates validator and consensus-parameter updates, and constructs the next
state. It then calls application `Commit` while holding the mempool lock,
updates the pools, assigns `ResponseFinalizeBlock.AppHash` to the next state,
and saves that state before publishing events. `Commit` is not the producer
of the new application hash.

After executing height `h`, the saved state records that block's ID and time,
its deterministic transaction-results hash, and its application hash. The
current validators become `LastValidators`, the previous `NextValidators`
become `Validators`, and validator updates affect `NextValidators` (height
`h + 2`). Consensus-parameter updates take effect at `h + 1`.

The corresponding [state validation](../../internal/state/validation.go)
requires the initial block height to equal `InitialHeight`, subsequent heights
to advance by one, and each header's chain ID and version to match local state.
The initial timestamp must be at or after genesis time; subsequent timestamps
must be strictly after the preceding block's timestamp. Proposal timeliness is
checked separately; these are not weighted-median or `TimeIotaMs` rules.
The Giga executor sets `SkipLastResultsHashValidation`, so the classic
results-hash comparison is conditional on that flag.

## Header

See [Header and its hash](../../types/block.go). Go uses `time.Time` for
`Time`, `version.Consensus` for `Version`, and byte slices for hashes.
`Header.ValidateBasic` accepts empty or 32-byte hashes through `ValidateHash`
for every hash below except `AppHash`, whose length is application-defined.
State validation checks their expected values; a length check alone does not
prove a commitment.

| Name | Type | Meaning |
| --- | --- | --- |
| Version | [Version](#version) | Block protocol and application versions. |
| ChainID | string | Consensus chain ID from genesis; at most `MaxChainIDLen` (50 bytes). This string is distinct from the EVM numeric chain ID. |
| Height | int64 | Positive block height, checked against state as described above. |
| Time | [Time](#time) | Block timestamp. |
| LastBlockID | [BlockID](#blockid) | Previous block ID; zero at the initial height. |
| LastCommitHash | bytes | Current `Commit.Hash()` commits to height, round, block ID and each commit signature. Basic block validation also accepts the legacy signatures-only hash for historical blocks. |
| DataHash | bytes | Merkle root over SHA-256 transaction hashes, in transaction order. |
| ValidatorsHash | bytes | Hash of the current validator set. |
| NextValidatorsHash | bytes | Hash of the next validator set. |
| ConsensusHash | bytes | SHA-256 of protobuf `HashedParams`, containing only `block_max_bytes` and `block_max_gas`. |
| AppHash | bytes | Application state after the preceding block; initially supplied by `InitChain`. |
| LastResultsHash | bytes | Merkle root over deterministic transaction results from the preceding block. `TxResultsHash` retains `Code`, `Data`, `GasWanted` and `GasUsed`; logs, info, codespace and events are excluded. |
| EvidenceHash | bytes | Merkle root of encoded evidence in this block. |
| ProposerAddress | bytes | Original block proposer's 20-byte address, preserved across re-proposals; must belong to the current validator set. |

`Header.Hash()` is a Merkle root over the individually encoded fields in the
order above, not SHA-256 of the entire header protobuf. It returns nil when
`ValidatorsHash` is missing. See [Hashing](./encoding.md#hashing) for the precise
encoding sources.

## Version

The [consensus version](../../proto/tendermint/version/types.proto) has two
`uint64` fields, `Block` and `App`. `Header.ValidateBasic` checks `Block` against
`version.BlockProtocol`; state validation checks both fields against
`state.Version.Consensus`. `App` comes from application version parameters.
These fields do not encode the P2P protocol version.

## BlockID

A `BlockID` contains two different commitments:

| Name | Type | Meaning |
| --- | --- | --- |
| Hash | bytes | Block header hash. |
| PartSetHeader | [PartSetHeader](#partsetheader) | Count and Merkle root of the serialized block's parts. |

`BlockID.ValidateBasic` permits empty or 32-byte hashes and validates the part
header. `IsNil()` requires an empty block hash and a zero part header.
`IsComplete()` requires a 32-byte block hash, positive part count and a
32-byte part root. Votes require either nil or complete IDs; proposals require
a complete ID. See [MakeParts](./encoding.md#makeparts).

## PartSetHeader

| Name | Type | Meaning and checks |
| --- | --- | --- |
| Total | uint32 | Number of parts. `ValidateBasic` requires at most `MaxBlockPartsCount` (101); zero is allowed for a zero header. A complete block ID requires a positive count. |
| Hash | bytes | Merkle root over part payloads. `ValidateBasic` accepts empty or 32 bytes; a complete block ID requires 32 bytes. |

The zero header has `Total = 0` and an empty `Hash`.

## Part

A classic block is serialized and divided into parts for gossip. The actual
[Part implementation](../../types/part_set.go) and protobuf agree on this shape:

| Name | Type | Meaning and checks |
| --- | --- | --- |
| index | uint32 | Zero-based part index; the first part has index 0. `Part.ValidateBasic` requires it to equal `proof.index`. `PartSet.AddPart` rejects an index outside the receiving set. |
| bytes | bytes | Variable-length serialized block payload, at most `BlockPartSizeBytes` (1,048,576 bytes). The last part may be shorter; this is not a hash. Basic validation imposes no positive minimum length. |
| proof | [Proof](#proof) | Structured Merkle inclusion proof for these payload bytes. Basic validation checks its shape; adding a part verifies the payload and proof against the receiving part-set root. |

`PartSet.AddPart` returns `ErrPartSetUnexpectedIndex` for an out-of-range index
and `ErrPartSetInvalidProof` for a failed proof. An already present index is
ignored without adding another part. See [Part](./encoding.md#part) and
[Merkle Proof](./encoding.md#merkle-proof) for the encoding.

## Time

The wire type is `google.protobuf.Timestamp`: `int64 seconds` and `int32 nanos`.
Generated consensus messages use Go `time.Time`. Signing canonicalizes the
message fields and uses protobuf timestamps; textual timestamp formatting is
not the signature payload. Timestamp validity and proposal timeliness are
contextual checks, not guarantees of `Vote.ValidateBasic` or
`Proposal.ValidateBasic`.

## Data

`Data` wraps `repeated bytes txs` (Go `Txs`, a slice of `Tx` byte slices).
The list can be empty. Consensus commits to its order and bytes; the
application validates and executes the transaction contents.
[Txs.Hash](../../types/tx.go) computes the Merkle root of the SHA-256 hashes of
the transactions, including the RFC 6962 empty root for an empty list.

## Commit

| Name | Type | Meaning and basic checks |
| --- | --- | --- |
| Height | int64 | Commit height; nonnegative. Height 0 represents the initial empty commit. |
| Round | int32 | Commit round; nonnegative, including round 0. |
| BlockID | [BlockID](#blockid) | Committed block ID. For height at least 1, `ValidateBasic` rejects a nil block ID. |
| Signatures | repeated [CommitSig](#commitsig) | For height at least 1, non-empty with at most `MaxVotesCount` (10,000) entries, each passing basic validation. |

These are the checks in `Commit.ValidateBasic`; they do not verify signatures
or establish a quorum. Full verification also checks the expected height and
block ID, validator-set size, addresses, signatures and voting power.

## CommitSig

A `CommitSig` represents an absent validator, a vote for the committed block,
or a nil vote.

| Name | Type | Meaning and basic checks |
| --- | --- | --- |
| BlockIDFlag | [BlockIDFlag](#blockidflag) | Must be absent, commit or nil; unknown is rejected. |
| ValidatorAddress | bytes | Empty for absent; exactly 20 bytes for commit or nil. |
| Timestamp | [Time](#time) | Go zero time for absent; vote timestamp otherwise. |
| Signature | bytes | Absent means no signature. Commit and nil require a signature; a present signature decodes to the fixed 64-byte Ed25519 type. |

The Go signature field is `utils.Option[crypto.Sig]`. Timestamp and
cryptographic validity are checked in their consuming contexts.

## BlockIDFlag

The wire enum is:

```proto
enum BlockIDFlag {
  BLOCK_ID_FLAG_UNKNOWN = 0;
  BLOCK_ID_FLAG_ABSENT = 1;
  BLOCK_ID_FLAG_COMMIT = 2;
  BLOCK_ID_FLAG_NIL = 3;
}
```

Unknown is a protobuf default, not a valid `CommitSig` participation flag.

## Vote

The [Go Vote](../../types/vote.go) describes a classic prevote or precommit.

| Name | Wire type | Meaning and basic checks |
| --- | --- | --- |
| Type | [SignedMsgType](#signedmsgtype) | Prevote (1) or precommit (2). |
| Height | int64 | Nonnegative in `ValidateBasic`; the consensus consumer checks the expected height. |
| Round | int32 | Nonnegative; round 0 is valid. |
| BlockID | [BlockID](#blockid) | Must be either nil or complete, after basic block-ID validation. |
| Timestamp | [Time](#time) | Signed vote time; not validated by `ValidateBasic`. |
| ValidatorAddress | bytes | Exactly 20 bytes. |
| ValidatorIndex | int32 | Nonnegative index into the validator set; index 0 is valid. Membership is checked by the consumer. |
| Signature | bytes | Optional in the Go representation; non-empty wire bytes must decode as a 64-byte Ed25519 signature. `Verify` requires a present, valid signature. |

The protobuf retains deprecated fields `extension` (9) and
`extension_signature` (10). The current Go `Vote` has neither field;
`VoteFromProto` does not retain them and `ToProto` does not emit them. Their
presence in the schema does not establish runtime vote-extension support.
`VoteFromProto` does not call `Vote.ValidateBasic`.

`Vote.Verify(chainID, pubKey)` compares the public key's address to
`ValidatorAddress`, then verifies `VoteSignBytes(chainID, vote.ToProto())`.
It returns `ErrVoteInvalidValidatorAddress` or `ErrVoteInvalidSignature` on
those respective failures; success is a nil error.

## CanonicalVote

The signing-only type is defined in
[canonical.proto](../../proto/tendermint/types/canonical.proto):

```proto
message CanonicalVote {
  SignedMsgType type = 1;
  sfixed64 height = 2;
  sfixed64 round = 3;
  CanonicalBlockID block_id = 4;
  google.protobuf.Timestamp timestamp = 5;
  string chain_id = 6;
}
```

`CanonicalizeVote` omits validator address, index, signature and deprecated
extension fields. `VoteSignBytes` protobuf-encodes this message with a varint
length prefix. A nil block ID canonicalizes to an absent `block_id`. See
[Signed Messages](./encoding.md#signed-messages) for proposal signing too.

## Proposal

A classic proposal identifies a block for a height and round. A nonnegative
proof-of-lock round (`POLRound`) refers to an earlier round; `-1` means no POL.
The [implementation](../../types/proposal.go) includes block-reconstruction
fields in addition to the signed proposal metadata:

| Name | Wire type | Meaning and checks |
| --- | --- | --- |
| Type | [SignedMsgType](#signedmsgtype) | Must be proposal (32). |
| Height | int64 | Nonnegative in `ValidateBasic`; checked against the current height by consensus. |
| Round | int32 | Nonnegative, including 0. |
| POLRound | int32 | Exactly `-1 <= POLRound < Round`. For round 0, only -1 is valid. |
| BlockID | [BlockID](#blockid) | Must pass basic validation and be complete and non-empty. |
| Timestamp | [Time](#time) | Proposal timestamp; basic validation does not check it. |
| Signature | bytes | Decoded to `crypto.Sig`, exactly 64 bytes. `ValidateBasic` does not verify the signature. |
| TxHashes (`tx_keys`) | repeated TxKey | Transaction hashes in order; each `TxKey` contains `bytes tx_key`. |
| Evidence | [EvidenceList](#evidencelist) | Evidence used to reconstruct the block. |
| LastCommit | [Commit](#commit) | Previous commit. |
| Header | [Header](#header) | Block header. |
| ProposerAddress | bytes | Proposer of this height/round, distinct from the original proposer recorded in the block header. |

`ProposalFromProto` decodes the signature, block ID, transaction hashes,
header, commit and evidence, then calls `Proposal.ValidateBasic`. That basic
check covers type, height, round, POL range and complete block ID; it does not
replace reconstruction, block validation, proposer or signature checks.
`Proposal.IsTimely` checks the timestamp against receive time and synchrony
parameters, with message delay scaled for later rounds.

## SignedMsgType

```proto
enum SignedMsgType {
  SIGNED_MSG_TYPE_UNKNOWN = 0;
  SIGNED_MSG_TYPE_PREVOTE = 1;
  SIGNED_MSG_TYPE_PRECOMMIT = 2;
  SIGNED_MSG_TYPE_PROPOSAL = 32;
}
```

A vote accepts only the two vote types; a proposal accepts only proposal type.

## Signature

The consensus `crypto.Sig` alias is a fixed-size Ed25519 signature. Its wire
representation is 64 raw bytes. An optional absent signature is encoded as
empty bytes where that message permits absence. See
[Key Types](./encoding.md#key-types) for public keys and verification semantics.

## EvidenceList

The wire wrapper contains `repeated Evidence evidence`; Go represents it as
`EvidenceList`, a slice. It may be empty. Each item has basic validation and a
serialized contribution to `EvidenceHash`; evidence-pool checks provide the
stateful verification. The serialized list must fit `EvidenceParams.MaxBytes`.

## Evidence

[Evidence](../../types/evidence.go) is a protobuf `oneof` of
`DuplicateVoteEvidence` and `LightClientAttackEvidence`. See the
[evidence protocol](../consensus/evidence.md) for verification against historical
state, age and duplicate checks.

### DuplicateVoteEvidence

| Name | Type | Meaning |
| --- | --- | --- |
| VoteA | [Vote](#vote) | First conflicting vote. |
| VoteB | [Vote](#vote) | Second conflicting vote. |
| TotalVotingPower | int64 | Validator-set power at the infraction height. |
| ValidatorPower | int64 | Offending validator's power at that height. |
| Timestamp | [Time](#time) | Time of the block at the infraction height. |

Basic validation requires both votes, validates each, and requires strict
lexicographic ordering by `BlockID.Key()`. Stateful verification establishes
that the votes conflict at the same height, round and type from the same
validator, verifies signatures, and checks the power and time metadata against
historical state.

### LightClientAttackEvidence

| Name | Type | Meaning |
| --- | --- | --- |
| ConflictingBlock | [LightBlock](#lightblock) | Conflicting signed header and validator set. |
| CommonHeight | int64 | Positive common height, no greater than the conflicting block's height. |
| ByzantineValidators | repeated [Validator](#validator) | Validators implicated by the attack. |
| TotalVotingPower | int64 | Positive total voting power. |
| Timestamp | [Time](#time) | Infraction time checked against historical state. |

Basic validation requires a conflicting block with a header and validates that
light block. It does not establish the attack or culpability by itself. See
[light-client accountability](../light-client/accountability/README.md).

## LightBlock

`LightBlock` contains non-nil `SignedHeader` and `ValidatorSet` fields.
[LightBlock.ValidateBasic](../../types/light.go) validates both and requires
`SignedHeader.ValidatorsHash == ValidatorSet.Hash()`. Signature and trust
verification are additional [light-client](../light-client/README.md) steps.

## SignedHeader

`SignedHeader` contains a non-nil `Header` and `Commit`. Its basic validation
checks both, the requested chain ID, equal header and commit heights, and
`Commit.BlockID.Hash == Header.Hash()`. These consistency checks do not verify
the commit's signatures or quorum.

## ValidatorSet

| Name | Wire type | Meaning |
| --- | --- | --- |
| Validators | repeated [Validator](#validator) | Non-empty active set. |
| Proposer | [Validator](#validator) | Non-nil proposer whose address belongs to the set. |
| TotalVotingPower | int64 | Serialized total; Go maintains a cached total derived from validators. |

[ValidatorSet.ValidateBasic](../../types/validator_set.go) validates every
validator and the proposer. Managed sets are sorted by voting power descending,
then address ascending. `Hash()` takes the Merkle root of `Validator.Bytes()`
in set order; those bytes encode only the public key and voting power, not
address or proposer priority.

## Validator

| Name | Wire type | Meaning and checks |
| --- | --- | --- |
| Address | bytes | 20-byte validator address. |
| PubKey | crypto.PublicKey | Protobuf Ed25519 public-key wrapper; decoded to the fixed 32-byte Go key type. |
| VotingPower | int64 | Nonnegative in `Validator.ValidateBasic`; zero in an update requests removal. |
| ProposerPriority | int64 | Signed proposer-selection priority; not constrained by basic validation. |

See [validator.go](../../types/validator.go) for conversion and validation.

## Address

`types.Address` aliases `crypto.Address`, a `HexBytes` slice encoded as hex in
JSON. An Ed25519 validator address is the first 20 bytes of SHA-256 of the raw
32-byte public key. This is a consensus validator identity; it is not an EVM
account-address derivation.

## ConsensusParams

The [parameter schema](../../proto/tendermint/types/params.proto) and
[Go implementation](../../types/params.go) define:

| Wire name | Type | Field number |
| --- | --- | --- |
| block | [BlockParams](#blockparams) | 1 |
| evidence | [EvidenceParams](#evidenceparams) | 2 |
| validator | [ValidatorParams](#validatorparams) | 3 |
| version | [VersionParams](#versionparams) | 4 |
| synchrony | [SynchronyParams](#synchronyparams) | 5 |
| timeout | [TimeoutParams](#timeoutparams) | 6 |
| abci | [ABCIParams](#abciparams) | 7 |

`ValidateConsensusParams` checks parameter bounds. Updates additionally pass
`ValidateUpdate`. The header's `ConsensusHash` commits only to the `HashedParams`
subset described under [Header](#header).

### BlockParams

| Wire name | Type | Field number | Meaning and bounds |
| --- | --- | --- | --- |
| max_bytes | int64 | 1 | Positive block byte limit, at most 104,857,600 (100 MiB). |
| max_gas | int64 | 2 | Block gas limit; at least -1, with -1 interpreted as unlimited. |
| min_txs_in_block | int64 | 3 | Deprecated minimum-transaction field; must be nonnegative. |
| max_gas_wanted | int64 | 4 | Gas-wanted limit; at least -1, with -1 interpreted as unlimited. |

`recheck_tx` is an ABCI parameter, not a block field. Part size is a code
constant, not this block's configured byte limit.

### EvidenceParams

| Wire name | Type | Field number | Meaning and bounds |
| --- | --- | --- | --- |
| max_age_num_blocks | int64 | 1 | Positive evidence age limit in blocks. |
| max_age_duration | google.protobuf.Duration | 2 | Positive evidence age limit in time. |
| max_bytes | int64 | 3 | Nonnegative evidence byte limit, no greater than block `max_bytes`. |

### ValidatorParams

`repeated string pub_key_types` is field 1. The list must be non-empty and
contain recognized ABCI key-type names. The current implementation recognizes
`ed25519`; these names differ from the JSON type tag `tendermint/PubKeyEd25519`.

### VersionParams

`uint64 app_version` is field 1 and supplies the application protocol version.

### SynchronyParams

| Wire name | Type | Field number | Meaning |
| --- | --- | --- | --- |
| message_delay | google.protobuf.Duration | 1 | Positive bound used for proposal delivery time. |
| precision | google.protobuf.Duration | 2 | Positive clock-skew allowance. |

Classic proposal timeliness uses these parameters; see the
[proposer-based timestamp specification](../consensus/proposer-based-timestamp/README.md).

### TimeoutParams

| Wire name | Type | Field number | Meaning |
| --- | --- | --- | --- |
| propose | google.protobuf.Duration | 1 | Initial proposal timeout. |
| propose_delta | google.protobuf.Duration | 2 | Proposal-timeout increase per round. |
| vote | google.protobuf.Duration | 3 | Initial vote timeout. |
| vote_delta | google.protobuf.Duration | 4 | Vote-timeout increase per round. |
| commit | google.protobuf.Duration | 5 | Wait after a commit quorum before the next height. |
| bypass_commit_timeout | bool | 6 | Advance when all precommits arrive, bypassing the remaining commit timeout. |

All five durations must be positive. These are classic consensus step timings.

### ABCIParams

| Wire name | Type | Field number | Meaning |
| --- | --- | --- | --- |
| vote_extensions_enable_height | int64 | 1 | Nonnegative schema parameter; zero denotes disabled. The Go `Vote` conversion does not carry extension data, as described under [Vote](#vote). |
| recheck_tx | bool | 2 | Whether to recheck transactions remaining in the mempool after execution. |

The existence of a parameter alone does not establish support in a running
consensus mode; use the actual message and execution paths.

## Proof

The [Merkle proof schema](../../proto/tendermint/crypto/proof.proto) contains:

| Wire name | Type | Field number | Meaning |
| --- | --- | --- | --- |
| total | int64 | 1 | Number of leaves. |
| index | int64 | 2 | Zero-based leaf index. |
| leaf_hash | bytes | 3 | SHA-256 of `0x00` followed by the leaf bytes. |
| aunts | repeated bytes | 4 | Sibling hashes ordered from the leaf toward the root. |

`Proof.ValidateBasic` requires nonnegative total and index, a 32-byte leaf hash,
at most 100 aunts and 32 bytes per aunt. Verification additionally requires a
positive total, `0 <= index < total`, the correct leaf hash, a consistent aunt
path and the expected root. See [Merkle Proof](./encoding.md#merkle-proof).
