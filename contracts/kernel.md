# Kernel and retained archive formats

Paxeer mainnet-beta is one BFT network with EVM chain ID **125** (`0x7d`). The kernel is part of that network and proposes blocks. A transaction receipt is final once its block is committed. Sub-chains are tenant domains in the database; they do not require another execution/settlement stack.

Public JSON-RPC uses the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, and `api-mainnet-beta.paxeer.network`. Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test`, and `txpool`. Hosted `px_` methods are specified by the [gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json). `node.hyperpaxeer.com` is the build library mirror, not an RPC endpoint.

This page restores the retained batch-header format contract used to inspect historical records. Historical names such as `sequencer_id` and `lxp_sequencer_authorization` identify bytes and types in that codec. They do not describe a separate running service or an additional finality vote. The old archive-codec executable is removed; callers retaining historical formats must use the library declarations that still exist.

Related contracts: [explorer archive HTTP reads](explorer-http-api.md#api-v2-paxeer-x), [socket status and recovery](explorer-ws.md), and [CLI receipt handling](platform-cli.md).

## Format batch header

The declarations are in [lxp_batch.h](../include/layerx/lxp_batch.h); the implementation is [lxp_batch_header.c](../src/sequencer/lxp_batch_header.c), using [lxp_codec.c](../src/codec/lxp_codec.c). `LXP_BATCH_HEADER_ENCODED_SIZE` is **354 bytes**. This is a canonical byte encoding, not a memory dump of the C struct or JSON.

The first five bytes are:

| Offset | Width | Value |
| --- | --- | --- |
| 0 | 2 | Protocol version, unsigned big-endian |
| 2 | 2 | Structure tag `0x1701`, unsigned big-endian |
| 4 | 1 | Field count `15` |

Each field then begins with its one-byte field ID, in the exact order below. Scalar values are unsigned big-endian with the indicated width. Every 32-byte value is encoded as a four-byte big-endian length (`32`) followed by 32 raw bytes; the field tag is additional. There is no alignment padding and no textual hex prefix.

| ID | Name | Value encoding after tag |
| --- | --- | --- |
| 1 | `protocol_version` | u16; must match the envelope version |
| 2 | `network_id` | u32 |
| 3 | `epoch` | u64 |
| 4 | `batch_number` | u64 |
| 5 | `first_sequence` | u64 |
| 6 | `last_sequence` | u64 |
| 7 | `previous_state_root` | Length-prefixed 32 bytes |
| 8 | `resulting_state_root` | Length-prefixed 32 bytes |
| 9 | `activity_merkle_root` | Length-prefixed 32 bytes |
| 10 | `receipt_merkle_root` | Length-prefixed 32 bytes |
| 11 | `event_merkle_root` | Length-prefixed 32 bytes |
| 12 | `data_availability_root` | Length-prefixed 32 bytes |
| 13 | `oracle_root` | Length-prefixed 32 bytes |
| 14 | `timestamp_ms` | u64; milliseconds |
| 15 | `sequencer_id` | Length-prefixed 32 bytes |

The encoded size is `5 + 15 + 2 + 4 + (5 * 8) + (8 * 36) = 354`. [lxp_protocol_version_supported](../src/protocol/lxp_protocol.c) accepts versions 1, 2, and 3; version values and structure tags are checked by the codec. The archive `network_id` is a u32 in the record; decoding a historical value must not silently replace it with today's EVM chain ID. A format decode alone does not establish semantic validity of roots, timestamps, or sequence ranges.

### Encoding and decoding

`lxp_batch_header_encode(header, arena, encoded)` writes into the caller's arena and returns a byte span owned by that arena. Null required arguments return `LXP_ERR_NON_CANONICAL`. Writer allocation/capacity failures propagate. Unsupported versions fail the struct-header writer with `LXP_ERR_INVALID_TAG`. A final encoded-size disagreement returns `LXP_FATAL_INVARIANT`.

`lxp_batch_header_decode(bytes, length, header)` clears the destination struct before parsing, after its initial argument/oversize checks. Treat output as unusable unless the entire function returns `LXP_OK`; a failed decode can leave a partially filled struct. Error behavior includes:

| Condition | Result |
| --- | --- |
| Null destination or null bytes with nonzero length | `LXP_ERR_NON_CANONICAL` |
| Input longer than 354 bytes | `LXP_ERR_TRAILING_BYTES` |
| Incomplete scalar, length prefix, or bytes | `LXP_ERR_TRUNCATED` |
| Wrong structure tag, unsupported envelope version, or envelope/field version disagreement | `LXP_ERR_VERSION_UNSUPPORTED` |
| Field count other than 15 | `LXP_ERR_NON_CANONICAL` |
| Missing, reordered, repeated, zero, or out-of-range field ID when encountered instead of the expected ID | `LXP_ERR_UNKNOWN_FIELD` |
| Byte-field length greater than 32 | `LXP_ERR_LENGTH_LIMIT` |
| Byte-field length less than 32 after a successful read | `LXP_ERR_NON_CANONICAL` |
| Unconsumed bytes after all fields | `LXP_ERR_TRAILING_BYTES` |

The first failing check determines the returned error. No unknown-field skipping or extension area is defined by this format.

### Header digest

`lxp_batch_header_hash` first re-encodes the header canonically, then calls `lxp_hash_domain(LXP_DOMAIN_BATCH_HEADER, ...)`. [Domain tags](../src/protocol/lxp_protocol.c) and [hash implementation](../src/crypto/lxp_hash.c) define the digest as:

```text
SHA-256(ASCII("LXP/v1/batch-header") || 0x00 || canonical_354_byte_header)
```

The terminating zero is part of the domain tag. There is no added domain-length prefix. The historical domain name stays unchanged for supported codec versions. It is not an Ethereum transaction hash or the current consensus block hash.

### Signature authorization

`lxp_batch_verify_signature(header, signature, signature_length, authorization, arena)` verifies an Ed25519 signature over the **32-byte header digest**, not directly over the 354-byte header. Signature length must be exactly 64 bytes; the authorization public key is 32 bytes.

Before hashing or signature verification, `authorization_matches` requires:

1. A non-null authorization with `authorized == 1`.
2. `first_batch_number <= header.batch_number <= last_batch_number`, both endpoints inclusive.
3. A constant-time match of the authorization's 32-byte `sequencer_id` against the header's ID.

Null header/signature/arena or a signature length other than 64 returns `LXP_ERR_BAD_SIGNATURE`. Missing authorization, unauthorized status, an out-of-range batch, or a mismatched signer ID returns `LXP_ERR_AUTH_SCOPE`. Hash/encoding errors propagate; otherwise the Ed25519 verifier's result is returned. The temporary digest is securely cleared.

Authorization must come from the caller's independently trusted historical configuration. This helper does not establish network membership, prove a consensus commit, or validate archived Merkle proofs by itself.

## Format batch body archive

Older registry entries also describe a `0x1702` batch-body archive and an export executable. Their former implementation paths are absent from the current tree; they are retired interfaces, not supported export commands. The retained `lxp_batch_header` contract above remains useful for decoding headers already held by archive consumers. For explorer-accessible frozen records use `/api/v2/paxeer-x/archive/anchors` and `/api/v2/paxeer-x/archive/receipts`, with the [HTTP contract](explorer-http-api.md#api-v2-paxeer-x) defining pagination and availability.
