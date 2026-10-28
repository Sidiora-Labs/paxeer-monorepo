# LayerX Portable Verification Guide

This guide explains how to verify LayerX receipts and external protocol evidence without LayerX infrastructure.

## Overview

The LayerX portable verification system supports two directions of evidence flow:

1. **Export**: LayerX receipts exported for verification by external parties
2. **Import**: External protocol receipts and mandates verified by LayerX through adapters

Both directions maintain cryptographic rigor, exact provenance binding, and independence from live infrastructure.

## Portable Receipt Export

### Purpose

External parties can verify LayerX receipts without:
- A LayerX gateway or node
- The layerxd daemon
- A database connection
- Network connectivity
- Wall-clock time

### Format: `layerx-receipt-proof-v1`

The portable receipt is a UTF-8 JSON object with these exact camelCase members (in any JSON member order):

| Member | Type | Description |
|--------|------|-------------|
| `format` | string | Literal `"layerx-receipt-proof-v1"` |
| `verificationLevel` | string | Literal `"sequencer-signed"` |
| `canonicalReceipt` | string | Complete canonical LayerX receipt (unpadded base64url) |
| `receiptDigest` | string | 32-byte receipt-signature digest (unpadded base64url) |
| `batchId` | string | 32-byte authorized batch ID (unpadded base64url) |
| `asset` | string | 32-byte batch asset (unpadded base64url) |
| `previousStateRoot` | string | 32-byte predecessor state root (unpadded base64url) |
| `resultingStateRoot` | string | 32-byte successor state root (unpadded base64url) |
| `sequencerPublicKey` | string | 32-byte Ed25519 public key (unpadded base64url) |

### Format: `layerx-receipt-proof-v2`

`PortableReceiptV2` serializes a UTF-8 JSON object with exactly these camelCase members:

| Member | Type | Description |
|--------|------|-------------|
| `format` | string | Literal `"layerx-receipt-proof-v2"` |
| `verificationLevel` | string | Literal `"receipted"`; the sequencer signed the receipt |
| `signedReceipt` | string | Complete canonical signed `ProtocolReceipt` v2 bytes (lowercase `0x` hex) |
| `receiptDigest` | string | 32-byte receipt-signature digest (lowercase `0x` hex) |
| `receiptKey` | object | The signing entry, with exactly the four members below |
| `receiptKey.keyId` | string | 32-byte receipt key identifier (lowercase `0x` hex) |
| `receiptKey.publicKey` | string | 32-byte Ed25519 public key (lowercase `0x` hex) |
| `receiptKey.firstHeight` | string | Inclusive first signing height (`0x` quantity) |
| `receiptKey.lastHeight` | string | Inclusive last signing height (`0x` quantity) |
| `inclusionHeight` | string | Carried inclusion height, at least the signed receipt height (`0x` quantity) |

Quantities use lowercase hex without leading zeroes (`0x0` for zero). JSON member order is insignificant. The carried key and inclusion height do not establish trust or prove block inclusion.

### Verification Algorithm (v2)

An independent verifier must:

1. **Parse and validate the JSON object**
   - Reject unknown members
   - Require the v2 format and `verificationLevel: "receipted"`
   - Reject non-canonical hex, quantities and wrong fixed-field lengths
   - Reject JSON above 1,500,000 bytes or signed receipts above 1,048,576 bytes

2. **Obtain the pinned `ReceiptKeys` list independently**
   - From a trusted snapshot, certificate or test vector manifest
   - Select the entry matching the receipt's signed key ID
   - Require the signed height to lie in its inclusive `firstHeight` to `lastHeight` window
   - Require every carried `receiptKey` member to equal that pinned entry
   - The JSON object itself is NOT the trust root

3. **Decode and re-encode the canonical receipt**
   - Must produce byte-identical output
   - `ProtocolReceipt` version 2 required
   - Non-zero operation and activity IDs required
   - Sequencer signature required

4. **Compute and verify the canonical receipt digest**
   - Encode the receipt without its signature: replace the trailing signature-presence flag, length and 64-byte signature with the single absent-signature byte `0x00`
   - Compute `SHA-256("LXP/v1/receipt\0" || unsigned_canonical_receipt)`
   - Require digest equals `receiptDigest`

5. **Verify the Ed25519 signature under the height-ranged receipt key**
   - Signature over the 32-byte digest
   - Public key comes from the independently pinned entry
   - Strict signature verification required
   - Require `inclusionHeight` to be at least the signed receipt height; this is a consistency check, not an inclusion proof

6. **Match the payment log before accepting a payment claim**
   - Require a successful signed outcome and authenticate the supplied log view against the signed receipt's log commitments
   - Require the `PaymentExact` emitter to be layerxledger at `0x000000000000000000000000000000000000101a` (`0x101A`)
   - Match the event signature, indexed payment identifiers and data (asset and exact amount) to the independently expected payment
   - Parsing the portable object or calling `verify_v2` alone does not perform this application-specific payment match

### What This Format Proves

A verified v2 portable receipt proves:
- The sequencer signed this exact receipt
- The signing key matches the pinned entry and covers the signed height
- The canonical receipt digest matches
- The receipt is cryptographically valid

It does **not** claim:
- Batch inclusion in a checkpoint
- Checkpoint finality
- External settlement
- Network consensus

Those properties require additional proofs beyond this format.

### Example (v1)

```json
{
  "format": "layerx-receipt-proof-v1",
  "verificationLevel": "sequencer-signed",
  "canonicalReceipt": "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgIBAAAAAAAACgAAAAAAAAAFAAAAAAAAAMgAAAAAAAABAAAAAAAAAAEAAAAAAAAAAQAAAAAAAAABAAAAAAAAAGQAAAAAAAAAZAAAAAAAAAABZAAAAAAAAAFkAAAAAAAAAQAAAAAAAABlAAAAAAAAAGQAAAAAAAAAAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE",
  "receiptDigest": "BAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQ",
  "batchId": "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE",
  "asset": "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI",
  "previousStateRoot": "AwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwM",
  "resultingStateRoot": "BAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQ",
  "sequencerPublicKey": "BQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQU"
}
```

### Example (v2)

This is the `PortableReceiptV2::export` result for the recorded
`V1-layerxledger-pay-exact` case in
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`, using its
`signedReceipt`, `pinnedKey` and signed height as `inclusionHeight`.
The AP2 test `portability_v2_example_parses` reproduces the export,
compares every JSON value and verifies the example against that pinned key.
All receipt, digest, key and height values below come from that recorded case.

```json
{
  "format": "layerx-receipt-proof-v2",
  "verificationLevel": "receipted",
  "signedReceipt": "0x000252010002000000209f6be3b6fda7dd664ddb83a901dad4458b78298b59e307151c217a2a1f13d2bb000000000000000300000020495f54c9be4c9531a0e3b2e3dc3c5919cadad7edeb4a35ec9bce39f98d1307ff00000020ee19fe28b96580c893539da18cb1011622992a17df97515c27ce57bdb80e110b00000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000002000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000ef9c000000000000ef9c00000000773594000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000010475687078000c000100000300000000200000000000000000000000000000000000000000000000000000000000000000000000b5000000000000000000000000000000000000101a04c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b11fb0082aa4aa4ce1eb67568a7d8f67798f3b008c254e0929dfe5eabb1dc0d76984ddb3e2af095e5f24fbe8a6d3e161eb9ebae5d2292861f3287f14e0db31798526fde3dedb14662268bff5bb3ab6560550a0dc3938b0c9d5ed717932c75b98396855168bb5fdfdfc33077297f77debd66ffd7b9410d4e6550606ca064e6c67ed000000000000000000006f93a9d230000000002000000000000000000000000000000000000000000000000000000000000003e9000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f730000000000000000000000000000000000000020000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd7300000000000000055de62d33fb3a700000000000000000055de5bda051684000000000000000000200000020000000000000000000000000000000000000000000000000000000000000101a0000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000206b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b000001977426f81001000000406ca73ab02f2784617246460ceef1861b2da793b3f80860560f2f57340c89a12b8d990f99a4b83a872d224734fb3ea5a9b687b11e04d25922eba5a3206d705a08",
  "receiptDigest": "0x07ca8892f51d79da84854829f737777341e03fe8bb156ce1784c88a4cc92b4ab",
  "receiptKey": {
    "keyId": "0x6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b",
    "publicKey": "0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
    "firstHeight": "0x3e8",
    "lastHeight": "0x7cf"
  },
  "inclusionHeight": "0x3e9"
}
```

### Implementation Example

See `services/interop/crates/layerx-portable/tests/independent_verifier.rs` for a complete standalone verifier that processes golden vectors with no LayerX infrastructure.

## External Evidence Import

### Purpose

LayerX verifies external protocol evidence (x402 payments, AP2 mandates, UCP orders, etc.) through version-pinned adapters that:
- Perform protocol-specific cryptographic verification
- Enforce exact spec version and conformance suite bindings
- Return typed verified values bound to exact input bytes
- Preserve verification provenance for audit

### Architecture

```
External Presentation (untrusted)
    ↓
ExternalEvidenceVerifier (adapter)
    ↓
Protocol-Specific Verification
    ↓
VerifiedExternalEvidence (typed + bound)
```

### External Presentation

The `ExternalPresentation` struct carries:
- `adapter`: Adapter ID (e.g., `"x402-v2"`)
- `protocol`: Protocol ID (e.g., `"x402"`)
- `spec_version`: Exact pinned version (e.g., `"2.0.3"`)
- `kind`: `ExternalEvidenceKind::Mandate` or `ExternalEvidenceKind::Receipt`
- `media_type`: IANA media type (e.g., `"application/vnd.x402.payment+jose"`)
- `payload`: Raw untrusted bytes (borrowed, never retained)

Validation rules:
- Adapter and protocol IDs must follow LayerX identifier rules
- Spec version must be exactly pinned (no wildcards or ranges)
- Media type must not contain control characters
- Payload bounds: 1 byte to 2,097,152 bytes
- Empty payloads rejected
- Oversized payloads rejected

### Adapter Contract

An `ExternalEvidenceVerifier` implementation must:

1. **Declare exact descriptor**
   - Adapter ID
   - Protocol ID and spec version
   - Spec document digest (SHA-256 of canonical spec text)
   - Conformance suite (name, vector count, suite digest)

2. **Declare evidence kind**
   - `Mandate` or `Receipt`
   - A verifier accepts exactly one kind

3. **Declare media type**
   - The exact IANA media type accepted
   - Must match presentation media type exactly

4. **Perform protocol-specific verification**
   - Signature verification
   - Delegation chain validation
   - Time bounds and nonce checks
   - Audience and constraint enforcement
   - Return typed verified value only after all checks pass

### Verified External Evidence

After successful verification, `VerifiedExternalEvidence<T>` binds:
- The adapter's protocol-specific verified value (type `T`)
- Adapter ID, protocol ID, spec version
- Spec document digest
- Conformance suite name, vector count, suite digest
- Evidence kind and media type
- **Evidence digest**: domain-separated SHA-256 hash of all inputs

The evidence digest is computed as:
```
SHA-256(
    "LayerX/interop/external-evidence/v1\0" ||
    kind_tag ||
    len(adapter) || adapter ||
    len(protocol) || protocol ||
    len(spec_version) || spec_version ||
    spec_document_digest ||
    len(conformance_suite) || conformance_suite ||
    conformance_vector_count ||
    conformance_suite_digest ||
    len(media_type) || media_type ||
    len(payload) || payload
)
```

This digest:
- Changes with any input byte or metadata field
- Is safe to store and share (no secret exposure)
- Provides non-repudiation for audit
- Avoids retaining possibly secret-bearing raw mandates

### Example: x402 Payment Receipt

```rust
use layerx_portable::{verify_external_evidence, ExternalPresentation, ExternalEvidenceKind};

// 1. Receive untrusted x402 payment receipt
let x402_payload = receive_payment_receipt();

// 2. Create presentation with exact metadata
let presentation = ExternalPresentation::new(
    "x402-v2",
    "x402",
    "2.0.3",
    ExternalEvidenceKind::Receipt,
    "application/vnd.x402.payment-response+jose",
    &x402_payload,
)?;

// 3. Verify through pinned x402 adapter
let x402_verifier = get_x402_verifier();
let verified = verify_external_evidence(
    &x402_verifier,
    &presentation,
    &payment_context,
)?;

// 4. Extract typed x402 receipt value
let x402_receipt = verified.into_verified();

// 5. Store only the evidence digest (not raw payload)
store_evidence_digest(verified.evidence_digest());
```

### Supported Protocols

Each adapter implements full upstream conformance:

| Protocol | Adapter | Evidence Types | Conformance |
|----------|---------|----------------|-------------|
| x402 v2 | `x402-v2` | Mandate, Receipt | x402 reference vectors |
| AP2 | `ap2` | Mandate pair | AP2 SD-JWT suite |
| UCP | `ucp` | Order, Receipt | UCP profile vectors |
| Visa Tap | `visa-tap` | Mandate | Visa conformance suite |

See each adapter's `COMPATIBILITY.md` for transport matrix and vector coverage.

## Testing and Portability Proof

### Golden Vectors

Test vectors are located in:
- `services/interop/crates/layerx-portable/tests/receipt_vectors.rs` — LayerX receipt export/verification
- `services/interop/crates/layerx-portable/tests/external_verification.rs` — External evidence import
- `services/interop/crates/layerx-portable/tests/independent_verifier.rs` — Standalone verifier harness

### Running Tests

```bash
# Run all portable verification tests
make interop-test-mandates

# Run only receipt vectors
cargo test --manifest-path services/interop/Cargo.toml -p layerx-portable --test receipt_vectors

# Run only external evidence tests  
cargo test --manifest-path services/interop/Cargo.toml -p layerx-portable --test external_verification

# Run independent verifier proof
cargo test --manifest-path services/interop/Cargo.toml -p layerx-portable --test independent_verifier
```

### Portability Proof

The `independent_verifier.rs` test proves portability by implementing a verifier that:
1. Has no LayerX infrastructure dependencies
2. Verifies golden vectors using only the format specification
3. Requires only a trusted `AuthorizedBatch` from an independent source
4. Processes receipts through `PortableReceipt` API alone

This proves external parties can verify LayerX receipts without running LayerX software.

## Security Considerations

### Trust Boundaries

**LayerX Receipt Export:**
- The JSON object itself is NOT a trust root
- For v1, the verifier MUST obtain `AuthorizedBatch` from an independent trusted source
- For v2, the verifier MUST obtain the pinned, height-ranged `ReceiptKeys` list independently
- A valid signature proves the sequencer signed this receipt
- It does NOT prove the batch was finalized or settled

**External Evidence Import:**
- Adapters trust only the pinned upstream specification
- Verification is cryptographic and deterministic
- The evidence digest is safe to store (no secret exposure)
- Raw mandate payloads may contain payment instruments — handle accordingly

### What Verification Does NOT Guarantee

Neither portable receipts nor external evidence verification prove:
- Liveness or availability of upstream systems
- Settlement finality on external networks
- Absence of equivocation (requires additional proofs)
- Freedom from rollback (requires checkpoint finality)

Always consult the specific verification level and proof scope.

## References

- **Format Specification**: `services/interop/crates/layerx-portable/FORMAT.md`
- **API Documentation**: `services/interop/crates/layerx-portable/src/lib.rs`
- **Golden Vectors**: `services/interop/crates/layerx-portable/tests/`
- **x402 Compatibility**: `services/interop/crates/layerx-x402/COMPATIBILITY.md`
- **Proof Library**: `services/agent/crates/layerx-proof/`

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026 | Initial `layerx-receipt-proof-v1` format and external evidence binding |
| 2.0 | 2026 | `layerx-receipt-proof-v2`: receipted v2 bytes, canonical digest and height-ranged receipt key; recorded example checked against `PortableReceiptV2::export`; v1 documents remain readable |
