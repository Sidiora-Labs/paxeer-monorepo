# Portable receipt verifier

`layerx-portable` exports and verifies `layerx-receipt-proof-v2`. It carries
the exact signed receipt and checks it against independently pinned receipt
keys without contacting a node, gateway, database, clock or network.
The public API lives in
`services/interop/crates/layerx-portable/src/receipt.rs` and is re-exported
by that crate's `src/lib.rs`.

The only admitted `verificationLevel` is `receipted`, including proofs
exported from committed RPC answers. Network finality occurs when a receipt's
block is committed, but the portable object alone does not prove that
commitment. See [Receipt concepts](../concepts/receipts.md) and
[Mirrors](mirrors.md) for the distinct evidence boundaries.

## Inputs

`PortableReceiptV2::verify_json(bytes, receipt_keys)` parses and verifies
the wire object. `verify_portable_json(bytes, receipt_keys)` dispatches by
format and returns `PortableVerified::V2`. For an already parsed object,
call `verify_v2(receipt_keys)`. `from_json` alone establishes no trust.

`ReceiptKeyEntry` aliases `SequencerAuthorization`. Each independently
pinned entry binds a 32-byte key ID, a 32-byte Ed25519 public key and an
inclusive first/last signing-height window. The carried key entry must
match the verifier's own pin in full. Obtain those governed entries through
an authenticated trust policy; a key carried only in the proof or in the
RPC response is not authoritative.

JSON must be nonempty and at most `1_500_000` bytes. Signed receipt bytes
and optional raw transaction bytes must each have length
`1..=1_048_576`. Byte strings use lowercase, even-length `0x` hex.
Heights use lowercase unsigned 64-bit `0x` quantities without leading
zeroes, except `0x0`.

| Member | Value |
| --- | --- |
| `format` | Literal `layerx-receipt-proof-v2` (`PORTABLE_RECEIPT_FORMAT`). |
| `verificationLevel` | Literal `receipted`. |
| `signedReceipt` | Exact canonical signed receipt bytes. |
| `receiptDigest` | 32-byte receipt-signature digest. |
| `receiptKey` | Object containing exactly `keyId`, `publicKey`, `firstHeight`, `lastHeight`. IDs and public keys are 32-byte hex; heights are quantities. |
| `inclusionHeight` | Quantity no lower than the height encoded in the signed receipt. |
| `rawTransaction` | Optional exact signed transaction bytes. When present, their Keccak-256 hash must equal the receipt's activity ID. |

Member order is insignificant. The Rust structs deny unknown fields at both
object levels and reject duplicate declared fields; independent decoders
should reject duplicate and unknown members before verification.

## Export

The supported constructors are:

| API | Input and behavior |
| --- | --- |
| `PortableReceiptV2::export(signed_receipt, receipt_key, inclusion_height)` | Verifies the signed bytes under the supplied pinned entry and checks the height before producing the object. |
| `PortableReceiptV2::export_with_transaction(signed_receipt, receipt_key, inclusion_height, raw_transaction)` | Also binds the raw transaction to the signed activity ID. |
| `PortableReceiptV2::from_sequencer_receipt(answer, receipt_keys, raw_transaction)` | Extracts a `pax_sequencerReceipt` answer and checks it under the caller's complete key list. Refuses `submitted`; accepts `receipted` and `committed`. |

The RPC constructor uses `height` for a receipted answer and `blockNumber`
for a committed answer as the carried inclusion height. It reads `receipt`
and `keyId` from the answer. It does not authenticate every other RPC view
field or prove the reported block's commitment. Verify those separately
when consuming the full RPC response. `to_json` encodes the bounded object;
it is not a replacement for verification against the consumer's own pins.

## What it verifies

After decoding fields, `verify_v2` performs these checks in order:

1. Call `layerx_proof::receipt::verify_sequencer_signature` on the canonical
   signed bytes using the complete pinned key list. It decodes and
   re-encodes byte-for-byte, requires a supported full protocol receipt,
   a nonzero activity ID and a signature, and verifies the Ed25519 signature.
2. Bind the carried `receiptKey` to the independently pinned entry selected
   by the receipt's signed `context_hash` (the key ID).
3. Require `inclusionHeight` to be at least the signed height.
4. Recompute the receipt-signature digest and compare `receiptDigest`.
5. If `rawTransaction` is present, require its Keccak-256 hash to equal the
   signed activity ID.

For height-range keys, `verify_sequencer_signature` requires the first 24
bytes of signed `batch_id` to be zero; its last eight bytes encode the
big-endian signed height. Exactly one pinned entry must have the signed
key ID. Missing or duplicate key identities, or a signed height outside
that entry's inclusive window, are refusals. The window applies to the
**signed height**, not `inclusionHeight`. A later inclusion height may lie
outside the signing window.

The digest is:

```text
SHA-256("LXP/v1/receipt\0" || unsigned_canonical_receipt)
```

The domain ends in one NUL byte. For the canonical signed framing, replace
the trailing 69 bytes (`01 00000040` followed by the 64-byte signature)
with one zero signature-presence byte to obtain the unsigned preimage.
Ed25519 verifies the 32-byte digest under the selected pinned public key.
Transaction hashing uses Keccak-256, which differs from SHA3-256.

Successful verification returns `PortableVerifiedReceiptV2`, exposing
`signed_receipt`, optional `raw_transaction`, `receipt_digest`,
`receipt_key`, `activity_id`, `global_sequence`, `result_code`, `height`
and `inclusion_height` through accessors. It authenticates the outcome,
including a rejected outcome: `result_code != 0` remains a failed
transaction. Payment consumers must also check the intended accounts,
asset, amount and relevant authenticated execution logs.

## Independent Python verification

`services/interop/crates/layerx-portable/tests/python_portable_verifier.py`
implements the sequencer receipt v2 verification path without importing a
LayerX SDK. It uses `cryptography` for Ed25519 and `pycryptodome` for
Keccak-256; dependencies are listed in that directory's `requirements.txt`.
Its `verify(document, pinned_keys)` function takes independently supplied
pins, checks canonical fields and receipt layout, verifies digest and
signature, and returns the receipt digest and activity ID.

For this sequencer layout the decoder reads fixed-width big-endian integers
and length-prefixed byte fields. It requires envelope version 2, tag
`0x5201`, protocol version 2, a nonzero activity ID, module ID above 11 and
module version 1. It bounds effect count and body lengths and rejects
truncation, invalid effect kinds and flags, signature framing errors and
trailing bytes. This is a verifier for that layout, not every historical or
program-specific receipt shape.

The Rust harness `tests/independent_verifier.rs` exports actual signed
receipts from
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json` and passes
their v2 portable JSON to Python over stdin. The fixture filename describes
the source recording, not the portable format version. Its six accepted
receipt cases include five successful outcomes and one reverted outcome.
Python checks the same digest and transaction identity, rejects substituted
fields and untrusted key lists, and exercises signing-window boundaries.
The script's command-line harness reads pins from the fixture and runs
mutation assertions; production consumers must supply their own trust pins
rather than use fixture keys.

`tests/proof_portable_receipt.rs` covers both direct and RPC constructors
through the public JSON consumer, including repeated use, raw-transaction
presence, corrupted transport and ambiguous pins.

## Typed refusals

`PortableReceiptError` preserves wire, authority and evidence failures:

| Variant | Condition in the v2 path |
| --- | --- |
| `JsonBounds` | Empty or oversized JSON, including an oversized encoded export. |
| `JsonShape` | Invalid JSON shape, missing required members, unknown members or an invalid RPC rung. |
| `ReceiptBounds` | Empty or oversized receipt supplied to export, or insufficient signed framing for digest extraction. |
| `UnsupportedFormat` | Format other than `layerx-receipt-proof-v2`. |
| `UnsupportedVerificationLevel` | Level other than `receipted`. |
| `InvalidHex(field)` | Missing `0x`, odd hex length or a non-lowercase hex digit. |
| `InvalidLength(field)` | Decoded bytes outside the field's fixed size or bounds. |
| `InvalidQuantity(field)` | Noncanonical or overflowing height quantity. |
| `Receipt(VerificationFailure { check })` | Canonical receipt or signature verification failed; `check` names the failing stage. |
| `ReceiptKeyMismatch` | No export key matching the RPC key ID, or the carried key differs from the pinned signing entry. |
| `NotReceipted` | RPC source rung is `submitted`. |
| `InclusionHeight` | Carried inclusion height precedes signed height. |
| `ReceiptDigestMismatch` | Recomputed digest differs from `receiptDigest`. |
| `RawTransactionMismatch` | Raw transaction hash differs from signed activity ID. |

Receipt verification stages are `Decode`, `CanonicalEncoding`,
`ReceiptShape`, `ProtocolVersion`, `ActivityId`, `MissingSignature`,
`BatchId` and `SequencerSignature`. Unpinned or ambiguous signing IDs and
out-of-window signed heights fail at `SequencerSignature`. The error enum
also retains older variants such as `InvalidBase64` and
`BatchAuthorizationMismatch`; these are not the v2 verification contract.

## Go SDK bundles

`sdk/platform/go/portable_receipt.go` exposes a separate portable schema with
the same `layerx-receipt-proof-v2` format label. It requires `rawTransaction`
and `receiptJson` and has no `inclusionHeight`. A `committed` bundle also
requires `blockHeader`, `receiptInclusion`, `commit` and `validatorSet`;
the verifier requires independently supplied consensus trust as well as
receipt-key pins. The Go SDK's conformance bundles are described in
`sdk/platform/conformance/README.md`.

Select a verifier by the complete object schema and its trust requirements.
The Rust constructors and Python verifier documented above accept the
receipted schema in this page's member table. The shared format label alone
does not make the two object shapes interchangeable.

## Evidence limits and migration

The v1 base64url object, its `sequencer-signed` level and caller-supplied
`AuthorizedBatch` portable verification workflow are retired. Current
`verify_portable_json` accepts v2 only. To migrate, obtain the exact signed
receipt and authenticated receipt-key ranges, export v2 through the current
constructor, then verify against the consumer's own pinned list. Renaming
the old format field does not migrate its evidence.

A portable proof does not establish committed block inclusion, current
network head, arbitrary account-state validity, external settlement or
provider payout. `inclusionHeight` is a checked carried value, not a block
proof. It also does not validate the full RPC logs or reconstruct every
program outcome. [AP2](ap2.md) and [ramps](ramps.md) impose their own
mandate, payment and settlement checks in addition to receipt authenticity.

`verify_external_evidence` is a separate API for upstream mandates and
receipts. A version-pinned adapter owns protocol-specific cryptography and
constraints; the generic layer binds its typed verified result to the exact
presentation, media type, evidence kind, specification and conformance
descriptor. It is not the network receipt verifier.

[Interoperability](index.md) · [Home](../index.md)
