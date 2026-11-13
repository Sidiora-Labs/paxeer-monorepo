# Portable receipt verifier

`layerx-portable` exports and verifies `layerx-receipt-proof-v2` JSON proofs
of signed protocol receipts. Verification runs offline against a caller's
independently pinned receipt keys. The format identifier is
`PORTABLE_RECEIPT_FORMAT`; the only accepted `verificationLevel` is `receipted`.

Paxeer is one BFT network. A receipt is final when its block is committed.
This portable object verifies receipt authenticity and optional transaction
binding; establishing block commitment requires separate chain evidence. See
[SDK sequencer receipt verification](SdkTerminalVerification.md).

## JSON contract

`PortableReceiptV2` uses camelCase fields and rejects unknown members, including
unknown members of `receiptKey`. JSON member order is insignificant. Byte strings
use lowercase `0x`-prefixed hex with two digits per byte. Heights use canonical
lowercase `0x` quantities, with no leading zeros except `0x0`.

| Member | Meaning |
| --- | --- |
| `format` | Literal `layerx-receipt-proof-v2` |
| `verificationLevel` | Literal `receipted` |
| `signedReceipt` | Canonical signed receipt bytes, 1–1,048,576 bytes |
| `receiptDigest` | 32-byte receipt-signature digest |
| `receiptKey` | Object with `keyId`, `publicKey`, `firstHeight` and `lastHeight` |
| `inclusionHeight` | Carried height, at least the signed receipt height |
| `rawTransaction` | Optional transaction bytes, 1–1,048,576 bytes when present |

The key ID and Ed25519 public key each encode 32 bytes. The first and last
heights describe the key's inclusive authorization window. The carried key
must match an entry in the verifier's own pinned list; the object cannot
supply its own trust authority.

JSON input must contain 1–1,500,000 bytes. The total JSON bound also applies
to exports, even when individual byte fields fit their bounds.

## Export and verification APIs

`ReceiptKeyEntry` is an alias for `SequencerAuthorization`.
`PortableReceiptV2::export` takes signed receipt bytes, one trusted key entry
and an inclusion height. `export_with_transaction` also takes raw transaction
bytes. Both verify the resulting proof before returning it.

`from_sequencer_receipt` accepts a `receipted` or `committed` RPC answer,
a complete pinned key list and optional raw transaction bytes. It refuses a
`submitted` answer with `NotReceipted`. For a committed answer it carries
`blockNumber` as `inclusionHeight`; otherwise it uses the answer's `height`.
The exported verification level remains `receipted` in either case.

`from_json` parses and checks field encodings without establishing trust.
`to_json` encodes a validated object. Use `verify_v2` on an object or
`PortableReceiptV2::verify_json` on JSON bytes to obtain
`PortableVerifiedReceiptV2`. The top-level `verify_portable_json` returns
`PortableVerified::V2` and refuses other format identifiers.

## Verification order

After field decoding, `verify_v2` performs these checks:

1. Verify the canonical signed receipt with `verify_sequencer_signature`
   against the complete pinned key list, including key identity and authorized
   height range.
2. Select the pinned key identified by the receipt's `context_hash` and require
   the carried key, public key and height window to equal that entry.
3. Require `inclusionHeight` to be at least the signed receipt height, encoded
   in the last eight bytes of `batch_id` as a big-endian u64.
4. Recompute the receipt digest and compare it with `receiptDigest`.
5. When `rawTransaction` is present, require its Keccak-256 hash to equal the
   receipt's `activity_id`.

The receipt digest is SHA-256 over `LXP/v1/receipt\0` followed by the unsigned
canonical receipt. The signed receipt's signature tail is replaced with the
zero flag for that digest. Ed25519 receipt verification uses the independently
authorized public key.

The verified value exposes `signed_receipt`, `raw_transaction`, `receipt_digest`,
`receipt_key`, `activity_id`, `global_sequence`, `result_code`, `height` and
`inclusion_height` getters. A verified receipt can carry a rejected execution
result. Consumers must check `result_code` and their payment or program terms
before accepting an operation as successful.

The carried inclusion height is a lower-bound check, not a block inclusion
proof. Portable verification alone does not establish BFT commitment, external
settlement, a payment's intended receiver or amount, or a program's complete
terminal outcome. `verify_external_evidence` is a separate adapter-binding API
for upstream mandates and receipts.

## Typed refusals

`PortableReceiptError` reports the first refusal in the relevant API path.

| Variant | Meaning |
| --- | --- |
| `JsonBounds` | Empty or oversized JSON |
| `JsonShape` | Invalid JSON shape, missing or unknown members, or serialization failure |
| `ReceiptBounds` | Exported receipt bytes are empty or oversized |
| `UnsupportedFormat` | Format identifier is unsupported |
| `UnsupportedVerificationLevel` | Verification level is not `receipted` |
| `InvalidHex(field)` | Byte string is not canonical lowercase `0x` hex |
| `InvalidQuantity(field)` | Height is not a canonical u64 `0x` quantity |
| `InvalidLength(field)` | Decoded field length is outside its bounds |
| `Receipt(VerificationFailure)` | Signed receipt verification failed; `check` identifies the stage |
| `ReceiptKeyMismatch` | Carried key does not match the pinned signing entry |
| `InclusionHeight` | Carried inclusion height precedes the signed height |
| `ReceiptDigestMismatch` | Claimed digest differs from the recomputed digest |
| `RawTransactionMismatch` | Raw transaction hash differs from the receipt activity ID |
| `NotReceipted` | RPC answer is only `submitted` |

## Sources and coverage

The implementation and public exports are in
[`src/receipt.rs`](../../services/interop/crates/layerx-portable/src/receipt.rs)
and [`src/lib.rs`](../../services/interop/crates/layerx-portable/src/lib.rs).

[`tests/receipt_vectors.rs`](../../services/interop/crates/layerx-portable/tests/receipt_vectors.rs)
includes offline v2 verification, export, encoding and mutation cases.
[`tests/independent_verifier.rs`](../../services/interop/crates/layerx-portable/tests/independent_verifier.rs)
consumes the SDK's
[`sequencer-receipt-v1.json`](../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json)
fixture and exercises an independent Python verifier in
[`tests/python_portable_verifier.py`](../../services/interop/crates/layerx-portable/tests/python_portable_verifier.py).
The fixture filename and portable JSON format have separate versions.

[Home](Home.md)
