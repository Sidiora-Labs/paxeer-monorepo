# Encoding

This page describes serialization and commitments in Paxeer's embedded
consensus engine. The [data structures](./data_structures.md) page documents
the corresponding fields and validation rules. The `tendermint` protobuf
package names are wire compatibility names in this tree.

Paxeer is one BFT network whose kernel participates in block proposal. The
classic consensus block-part and signing formats below describe the classic
engine. Autobahn uses the separate message schema in
[autobahn.proto](../../internal/autobahn/autobahn.proto) and converters in
[autobahn/types](../../autobahn/types/); its payload transport does not inherit
the classic part-size or part-count limits.

## Protocol Buffers

Consensus block, vote, proposal, validator and proof wire messages use
[Protocol Buffers proto3](https://protobuf.dev/programming-guides/proto3/).
The authoritative schemas are in [proto/tendermint](../../proto/tendermint/).
Go domain types are converted using their `ToProto`/`FromProto` functions;
a domain struct's JSON or memory representation is not its protobuf encoding.

Protobuf field keys encode the field number and wire type. Length-delimited
fields carry their byte length as an unsigned varint before their contents.
Omitted default-valued fields, nested messages and signing length prefixes
must be handled according to the relevant schema and producer.

## Byte Arrays

A protobuf `bytes` field is length-delimited: its field key is followed by the
unsigned-varint byte count and the raw bytes. With the field key omitted for
illustration, `[0x0A, 0x0B]` has length and contents `02 0A 0B`; a 300-byte
array beginning with those two bytes has `AC 02 0A 0B ...`.

For example, `Part.bytes` is field 2, so its field key is `12` in hexadecimal.
A two-byte payload in that field alone encodes as `12 02 0A 0B`. This example
is not a complete valid `Part`: a real part also carries its index and proof.
See the [protobuf encoding guide](https://protobuf.dev/programming-guides/encoding/).

## Hashing

Consensus uses SHA-256 and the [RFC 6962 Merkle tree](#merkle-trees), but the
bytes supplied to each hash are type-specific. Do not substitute a hash of an
entire protobuf or JSON object for a field-wise commitment.

| Commitment | Input and implementation |
| --- | --- |
| Transaction hash | SHA-256 of the raw transaction bytes, `Tx.Hash` in [tx.go](../../types/tx.go). |
| `DataHash` | Merkle root over transaction hashes in transaction order, `Txs.Hash`. |
| Header/block hash | Merkle root over the 14 individually encoded header fields, `Header.Hash` in [block.go](../../types/block.go). |
| Part-set root | Merkle root over the raw part payloads, `NewPartSetFromData` in [part_set.go](../../types/part_set.go). |
| Commit hash | Merkle root over encoded height, round, block ID and each protobuf `CommitSig`, `Commit.Hash`. Basic block validation also accepts the historical signatures-only `legacyHash`. |
| Validator-set hash | Merkle root of protobuf `SimpleValidator` encodings (public key and voting power) in set order, `Validator.Bytes` and `ValidatorSet.Hash`. |
| Consensus-parameter hash | SHA-256 of protobuf `HashedParams` containing `block_max_bytes` and `block_max_gas`, `HashConsensusParams` in [params.go](../../types/params.go). |
| Last-results hash | Merkle root over protobuf deterministic transaction results, `TxResultsHash` in [execution.go](../../internal/state/execution.go) using `abci.MarshalTxResults`. Only code, data, gas wanted and gas used are retained. |
| Evidence hash | Merkle root over each evidence item's encoded bytes, `EvidenceList.Hash` in [evidence.go](../../types/evidence.go). |

`Header.Hash` encodes version, timestamp and block ID as their protobuf
messages. It encodes the chain-ID string, height and byte-slice fields using
[cdcEncode](../../types/encoding_helper.go), which uses protobuf wrapper
messages and returns empty bytes for empty values. Field order is documented
under [Header](./data_structures.md#header) and fixed by the implementation.
These rules differ from signing canonicalization.

## Public Key Cryptography

The current consensus [crypto package](../../crypto/crypto.go) aliases public
keys, private keys and signatures to its Ed25519 implementation. The protobuf
[PublicKey](../../proto/tendermint/crypto/keys.proto) uses a `oneof sum` with
`bytes ed25519 = 1`. The old `secp256k1` and `sr25519` field numbers and names
are reserved; they are not accepted validator-key alternatives in this tree.
A signature field is raw bytes, not a public-key `oneof`.

### Key Types

#### Ed25519

A public key contains exactly 32 bytes, and a signature exactly 64 bytes.
`PublicKeyFromBytes` and `SignatureFromBytes` enforce those lengths in
[conv.go](../../crypto/ed25519/conv.go). A validator address is:

```text
SHA256(raw_32_byte_public_key)[0:20]
```

Verification uses ZIP-215 semantics in
[ed25519.go](../../crypto/ed25519/ed25519.go). This is current behavior.
A correct byte length alone does not prove a signature; the consumer must
verify it against the correct message, chain ID and validator key.

#### Secp256k1

Secp256k1 is not a supported consensus validator-key encoding here. Its reserved
protobuf field must not be used as an alternative to Ed25519. EVM account keys
and addresses belong to the application protocol and do not change the
consensus validator-key format.

## Other Common Types

### BitArray

Classic consensus uses a bit array to track votes or received parts. The
[protobuf representation](../../proto/tendermint/libs/bits/types.proto) is:

| Field | Type | Meaning |
| --- | --- | --- |
| bits | int64 | Logical number of bits. |
| elems | repeated uint64 | Packed bit words. |

`elems` contains integers, not base64 text. The Go
[BitArray](../../libs/bits/bit_array.go) has custom JSON encoding: a string
with `x` for a set bit and `_` for an unset bit, in increasing index order.
For example, bits `1, 0, 1, 1, 0` encode as `"x_xx_"`. A nil bit array encodes
as JSON `null`.

### Part

The classic [Part](./data_structures.md#part) wire message is defined in
[types.proto](../../proto/tendermint/types/types.proto):

```proto
message Part {
  uint32 index = 1;
  bytes bytes = 2;
  tendermint.crypto.Proof proof = 3;
}
```

The index is zero-based. `bytes` contains a slice of the serialized block,
and `proof` is a structured [Merkle proof](#merkle-proof) for that slice.
Neither the payload nor the whole proof is a fixed 32-byte hash.
`Part.ValidateBasic` bounds the payload size, checks the proof's basic shape,
and requires `int64(index) == proof.index`. `PartSet.AddPart` separately checks
the receiving set's index range and verifies the proof against its root and
the payload bytes.

### MakeParts

“MakeParts” denotes the block-splitting operation; the actual Go API is
`(*Block).MakePartSet(partSize uint32) (*PartSet, error)` in
[block.go](../../types/block.go). It converts the block to protobuf, marshals
it and calls `NewPartSetFromData`. A zero `partSize` is rejected.

Classic callers use `types.BlockPartSizeBytes`. The current constants in
[params.go](../../types/params.go) are:

| Constant | Value |
| --- | --- |
| `MaxBlockSizeBytes` | 104,857,600 bytes (100 MiB) |
| `BlockPartSizeBytes` | 1,048,576 bytes (1 MiB) |
| `MaxBlockPartsCount` | `(MaxBlockSizeBytes / BlockPartSizeBytes) + 1` = 101 |

For serialized byte length `n` and positive part size `s`, the producer creates
`ceil(n / s)` parts. Each contains consecutive bytes in block order; only the
last may be shorter than `s`. `ProofsFromByteSlices` builds the part-set root
and each part's proof from those raw slices.

`Part.ValidateBasic` rejects payloads over 1 MiB, and
`PartSetHeader.ValidateBasic` rejects a `total` over 101. That derived count is
a structural allocation bound, not permission to accept a 101 MiB block.
The configured consensus block byte limit is validated separately and cannot
exceed the 100 MiB hard cap. These constants describe classic block gossip;
Autobahn's payload and transport budgets are defined in
[autobahn/types/block.go](../../autobahn/types/block.go) and its own schema.

## Merkle Trees

[HashFromByteSlices](../../crypto/merkle/tree.go) implements the RFC 6962 tree
with SHA-256 and preserves item order. The rules are:

- Empty root: `SHA256(empty bytes)`.
- Leaf: `SHA256(0x00 || leaf_data)`.
- Inner node: `SHA256(0x01 || left_hash || right_hash)`.
- For more than one leaf, split at the largest power of two strictly less than
  the number of leaves, then hash the two subtrees.

The different leaf and inner prefixes provide domain separation. The tree
need not have a power-of-two number of leaves. Six and seven leaves have the
following shapes (`h0` through `h6` are leaf hashes):

```text
        six leaves                      seven leaves
             *                               *
           /   \                           /   \
          *     *                         *     *
         / \   / \                       / \   / \
        *   * h4 h5                     *   * *  h6
       / \ / \                         / \ / \ / \
      h0 h1 h2 h3                     h0 h1 h2 h3 h4 h5
```

### MerkleRoot

`MerkleRoot` is descriptive notation for `merkle.HashFromByteSlices`. In
pseudocode, where `||` means byte concatenation:

```text
MerkleRoot(items):
    if len(items) == 0:
        return SHA256(empty bytes)
    if len(items) == 1:
        return SHA256(0x00 || items[0])
    k = largest power of two strictly less than len(items)
    return SHA256(0x01 || MerkleRoot(items[:k]) || MerkleRoot(items[k:]))
```

Items are arbitrary byte arrays. Prehashing is specific to the producer:
transaction-tree items are transaction hashes, while part-tree items are raw
part payloads. There is no universal rule that every object is protobuf-encoded
or prehashed before becoming a Merkle leaf.

### Merkle Proof

The [proof message](../../proto/tendermint/crypto/proof.proto) is:

| Field | Type | Meaning |
| --- | --- | --- |
| total | int64 | Leaf count. |
| index | int64 | Zero-based index of the proved leaf. |
| leaf_hash | bytes | `SHA256(0x00 || leaf_data)`. |
| aunts | repeated bytes | Sibling hashes from the leaf upward, excluding the root. |

[Proof.ValidateBasic](../../crypto/merkle/proof.go) requires nonnegative total
and index, a 32-byte leaf hash, at most `MaxAunts` (100) siblings, and 32 bytes
per sibling. It does not prove membership or enforce `index < total`.

`Proof.Verify(rootHash, leaf)` returns an error on failure and nil on success.
It recomputes the prefixed leaf hash and reconstructs the root using the index,
total and aunts. Reconstruction requires `total > 0`, `0 <= index < total`,
and exactly the aunt path needed by the tree shape. A one-leaf proof has no
aunts. Every recursive split uses the same largest-power-of-two rule as the
tree, and the final computed hash must equal `rootHash`.

### IAVL+ Tree

These simple Merkle trees commit to consensus data. The application chooses
its own state commitment and returns `AppHash`; this page's part or transaction
proof format does not specify an IAVL or other application-state proof.

## JSON

Consensus JSON is separate from protobuf, canonical signing bytes and the
public EVM JSON-RPC encoding. Use the Go JSON tags and marshalers for the
specific type. `HexBytes` values such as validator addresses and block hashes
encode as hexadecimal strings; ordinary `[]byte` fields use base64. Many
64-bit numeric fields use decimal strings where the struct's JSON tag requests
`,string`. `BitArray` uses the custom string form described above.

The [jsontypes](../../internal/jsontypes/) machinery wraps registered values
with a type tag when invoked by the containing type's marshaler. For example,
a tagged Ed25519 public key has this shape:

```json
{
  "type": "tendermint/PubKeyEd25519",
  "value": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
}
```

The value above illustrates the base64 encoding of 32 bytes, not a validator
identity to configure. Direct `PublicKey.MarshalJSON` produces the base64
string; the type wrapper comes from `jsontypes`. See
[ed25519/json.go](../../crypto/ed25519/json.go).

### Signed Messages

Classic votes and proposals sign canonical protobuf messages with an unsigned
varint length prefix. The actual producers are `VoteSignBytes` and
`ProposalSignBytes`, using `protoio.MarshalDelimited` after canonicalization
in [canonical.go](../../types/canonical.go). Do not sign JSON or the complete
ordinary wire message.

A canonical vote is:

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

A canonical proposal, also defined in
[canonical.proto](../../proto/tendermint/types/canonical.proto), is:

```proto
message CanonicalProposal {
  SignedMsgType type = 1;
  sfixed64 height = 2;
  sfixed64 round = 3;
  int64 pol_round = 4;
  CanonicalBlockID block_id = 5;
  google.protobuf.Timestamp timestamp = 6;
  string chain_id = 7;
}
```

Height and round use signed fixed-width 64-bit encoding in canonical messages;
the message type remains a protobuf enum encoded as a varint. Proto3 default
omission and the outer length prefix mean implementations must parse protobuf
rather than assume every field has a fixed offset. The chain ID is the
consensus chain-ID string, not the EVM numeric ID.

Canonical block IDs contain the block hash and canonical part-set header;
a nil block ID canonicalizes to an absent message. Canonical votes omit the
validator address, index, signature and deprecated extension fields.
Canonical proposals omit the signature and block-reconstruction fields
(transaction keys, header, commit, evidence and proposer address). The
consumer still has to authenticate the signer and validate the full proposal.
See [Vote](./data_structures.md#vote), [Proposal](./data_structures.md#proposal)
and the [signing specification](../consensus/signing.md).
