# LayerX Portable Verification Guide

This guide covers offline verification of signed network receipts and the typed
boundary for importing external evidence. Paxeer mainnet-beta is one BFT network
(chain ID 125, `0x7d`); the kernel is part of the network and proposes blocks.
A transaction is final when its block commits. A portable signature proof alone
does not establish that commit.

## Overview

The portable verification crate supports two directions:

1. **Export**: carry canonical signed `ProtocolReceipt` v2 bytes to an offline verifier.
2. **Import**: bind an adapter's typed verification result to exact external presentation bytes.

The current receipt API is `PortableReceiptV2`. `verify_portable_json` accepts
only `layerx-receipt-proof-v2`. Other portable format versions return
`PortableReceiptError::UnsupportedFormat`.

## Portable Receipt Export

### Purpose

Once receipt keys have been pinned independently, an external verifier needs no
running gateway, node, database, network connection or wall clock. The carried
key is evidence to compare with that trust policy, never its source of trust.

Online acquisition uses node JSON-RPC at the configured node's root HTTPS path
on chain ID 125. See the [public RPC guide](../site/docs/reference/public-rpc.md)
for connection details. `pax_sequencerReceipt` reports `submitted`, `receipted`
or `committed`; those are stages within the same network.

### Format: `layerx-receipt-proof-v2`

`PortableReceiptV2` serializes a UTF-8 JSON object with these camelCase members:

| Member | Type | Description |
| --- | --- | --- |
| `format` | string | Literal `layerx-receipt-proof-v2` |
| `verificationLevel` | string | Literal `receipted`, also when exported from a committed answer |
| `signedReceipt` | string | Complete canonical signed `ProtocolReceipt` v2, lowercase `0x` hex |
| `receiptDigest` | string | 32-byte receipt-signature digest, lowercase `0x` hex |
| `receiptKey` | object | Exactly the four members below |
| `receiptKey.keyId` | string | 32-byte signing key identifier, lowercase `0x` hex |
| `receiptKey.publicKey` | string | 32-byte Ed25519 public key, lowercase `0x` hex |
| `receiptKey.firstHeight` | string | Inclusive first signing height, `0x` quantity |
| `receiptKey.lastHeight` | string | Inclusive last signing height, `0x` quantity |
| `inclusionHeight` | string | Carried height at least the signed receipt height, `0x` quantity |
| `rawTransaction` | optional string | Raw signed transaction, lowercase `0x` hex, hashed against the activity ID |

JSON member order is insignificant. Quantities are unsigned 64-bit lowercase hex
without leading zeroes (`0x0` for zero). Unknown or duplicate members are refused.
A JSON document is at most 1,500,000 bytes; receipt and optional transaction bytes
are each nonempty and at most 1,048,576 bytes. See the full wire layout in
[FORMAT.md](../../services/interop/crates/layerx-portable/FORMAT.md).

### Verification Algorithm (v2)

1. Parse the bounded, closed JSON shape and canonical fields.
2. Decode and canonically re-encode the signed `ProtocolReceipt` v2. Require
   a nonzero activity ID, the supported receipt layout and an Ed25519 signature.
3. Select exactly one independently pinned `ReceiptKeys` entry by the signed
   context hash (the key ID). The signed height is in the last eight bytes of
   the receipt's batch ID, whose first 24 bytes must be zero. Require that height
   to lie within the entry's inclusive window and every carried key field to
   equal that entry. Do not select the window using `inclusionHeight`.
4. Replace the final signature framing and signature (`01 00000040` plus 64 bytes)
   with one absent-signature byte (`00`). Compute
   `SHA-256("LXP/v1/receipt\0" || unsigned_canonical_receipt)`; the domain ends in
   a NUL byte. Require equality with `receiptDigest` and strictly verify the
   Ed25519 signature over that digest.
5. Require `inclusionHeight` to be at least the signed height. If `rawTransaction`
   is present, require its Keccak-256 hash to equal the signed activity ID.
6. Inspect the signed result code: a nonzero result remains a rejected transaction.
   For a payment claim, additionally authenticate the supplied logs against the
   receipt's commitments and match the expected event, identifiers, asset and
   amount. `PaymentExact` must be emitted by layerxledger at
   `0x000000000000000000000000000000000000101a`. `verify_v2` alone does not match
   an application payment claim.

### What This Format Proves

Verification authenticates the exact receipt under the independently pinned
signing key and its height window. Optional transaction bytes are bound to that
receipt's activity ID. It does not prove block inclusion, BFT commit, absence of
equivocation, or settlement on an external chain. The carried inclusion height
is only a consistency check, and exporting a committed answer still produces
verification level `receipted`. Applications needing committed evidence must
verify the additional block/commit proof under their own consensus trust policy.

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
  "signedReceipt": "0x00025201000200000020a0d985d943521892401c623bd7b32a627147585cbe206badb4d214d844b1ccd1000000000000000300000020495f54c9be4c9531a0e3b2e3dc3c5919cadad7edeb4a35ec9bce39f98d1307ff000000206a6936594eb02b36903e48ec2710bf5bf7eacce656934407da363ff1d93b24ea00000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000002000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000ef9c000000000000ef9c00000000773594000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85500000000000000010475687078000c000100000300000000200000000000000000000000000000000000000000000000000000000000000000000000b5000000000000000000000000000000000000101a04c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b126fde3dedb14662268bff5bb3ab6560550a0dc3938b0c9d5ed717932c75b98391fb0082aa4aa4ce1eb67568a7d8f67798f3b008c254e0929dfe5eabb1dc0d769112b929c0d6cb01c57eaad307e6d0a094c6c6e58f2e0ce3c62856cf815c0fba46855168bb5fdfdfc33077297f77debd66ffd7b9410d4e6550606ca064e6c67ed000000000000000000006f93a9d230000000002000000000000000000000000000000000000000000000000000000000000003e9000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f730000000000000000000000000000000000000020000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd7300000000000000055de62d33fb3a700000000000000000055de5bda051684000000000000000000200000020000000000000000000000000000000000000000000000000000000000000101a0000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000206b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b000001977426f81001000000401370ce08ed0dee4b72e305f93f8ca9ef73f0394782d1489ceafdbb58d045e1186ff2e235b559712cd013368002611607b0776be1c166b43f54b5d953c68a9208",
  "receiptDigest": "0x3f63045f1bcd3b2eb0fd9de72de19e76de4e509b567212c8645c58a18d2b4bcb",
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

The public API in [receipt.rs](../../services/interop/crates/layerx-portable/src/receipt.rs) provides:

- `PortableReceiptV2::from_sequencer_receipt(answer, receipt_keys, raw_transaction)`:
  accepts receipted or committed node answers and rejects submitted answers.
- `PortableReceiptV2::export` and `export_with_transaction`: export signed bytes
  after checking the supplied key and height.
- `PortableReceiptV2::from_json`: parses fields without establishing trust.
- `verify_v2`, `verify_json` and `verify_portable_json`: authenticate against the
  caller's pinned key list; inspect `result_code()` on the verified value.

The following example is also committed as
[`examples/portable_v2.rs`](../../services/interop/examples/portable_v2.rs). It exports a node answer and verifies the portable representation against
independently pinned keys.

Pass the node's receipted or committed answer to `export_receipt`, then carry the
returned UTF-8 JSON bytes to `verify_receipt`. Supply `ReceiptKeyEntry::new(key_id,
public_key, first_height, last_height)` entries from your independent trust policy
on both sides; do not build that policy from the proof's `receiptKey`. The key ID
must be unique and its inclusive height window must cover the signed receipt
height, even when a committed answer carries a later block height. Supply raw
transaction bytes when the consumer needs transaction binding.

```rust
use layerx_portable::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    PortableVerifiedReceiptV2, ReceiptKeyEntry,
};

pub fn export_receipt(
    answer: &serde_json::Value,
    pinned_keys: &[ReceiptKeyEntry],
    raw_transaction: Option<&[u8]>,
) -> Result<Vec<u8>, PortableReceiptError> {
    PortableReceiptV2::from_sequencer_receipt(answer, pinned_keys, raw_transaction)?.to_json()
}

pub fn verify_receipt(
    json: &[u8],
    pinned_keys: &[ReceiptKeyEntry],
) -> Result<PortableVerifiedReceiptV2, PortableReceiptError> {
    let PortableVerified::V2(receipt) = verify_portable_json(json, pinned_keys)?;
    Ok(receipt)
}
```

After verification, inspect `result_code()` before treating execution as
successful: a nonzero result remains rejected. Neither function proves BFT
commit or matches an application payment claim. Parsing with `from_json` alone
also does not establish trust.

The Rust [independent verifier harness](../../services/interop/crates/layerx-portable/tests/independent_verifier.rs)
compares the crate with a [Python verifier](../../services/interop/crates/layerx-portable/tests/python_portable_verifier.py)
that implements the format without importing a LayerX SDK. Python uses
`cryptography` for Ed25519 and `pycryptodome` for Keccak-256.

## External Evidence Import

### Purpose

External presentations retain their protocol's wire format. The generic portable
boundary validates metadata, invokes the supplied adapter, and binds its typed
result to the bytes verified. It does not itself implement external signature,
delegation, time, nonce, audience or payment checks.

### Architecture

```text
ExternalPresentation (untrusted bytes and metadata)
    -> ExternalEvidenceVerifier (caller-selected, trusted implementation)
    -> protocol-specific verification with explicit context
    -> VerifiedExternalEvidence<T> (typed result and digest binding)
```

### External Presentation

`ExternalPresentation::new` takes adapter ID, protocol ID, exact spec version,
`ExternalEvidenceKind::Mandate` or `Receipt`, media type, and borrowed payload
bytes. Adapter/protocol identifiers and the pinned version are validated. The
media type must be nonempty, at most 128 bytes and contain no ASCII controls.
The payload must contain 1–2,097,152 bytes.

### Adapter Contract

`ExternalEvidenceVerifier<C>` declares an `AdapterDescriptor`, evidence kind and
exact accepted media type, and implements protocol-specific `verify(payload, context)`. Its associated types preserve the verified value and verification
error. `verify_external_evidence` refuses descriptor, kind or media-type
mismatches before calling it, then preserves adapter errors.

The caller selects and trusts the verifier implementation, upstream pin,
conformance declaration and context. A descriptor digest or vector count alone
is not proof that a conformance suite ran or that an implementation is trustworthy.

### Verified External Evidence

`VerifiedExternalEvidence<T>` has private fields and contains the typed result,
adapter/protocol/version, specification digest, conformance suite identity and
digest, evidence kind, media type and evidence digest. The digest is:

```text
SHA-256(
    "LayerX/interop/external-evidence/v1\0" || kind_tag ||
    len(adapter) || adapter || len(protocol) || protocol ||
    len(spec_version) || spec_version || spec_document_digest ||
    len(conformance_suite) || conformance_suite || conformance_vector_count ||
    conformance_suite_digest || len(media_type) || media_type ||
    len(payload) || payload
)
```

Lengths and the vector count are unsigned 64-bit big-endian integers. The kind
tag is one byte: mandate = 1, receipt = 2. Specification and suite digests are
32 raw bytes each.

This digest binds bytes and metadata for comparison. It is not a signature,
proof of verifier provenance, or non-repudiation. The verification context and
typed result are not included in it. An external consumer must authenticate a
verification report and trust its producer, or repeat verification. The wrapper
does not retain the borrowed payload, but the adapter controls what its `T`
contains. Digests can permit correlation or guessing of low-entropy inputs;
apply the relevant data-handling policy before storing or sharing them.

### Example: AP2 Mandate Pair

The implemented generic import is
[`Ap2ExternalMandateVerifier`](../../services/interop/crates/layerx-ap2/src/portable.rs).
`Ap2MandatePair::new(checkout, payment).to_json()` constructs an envelope with
exactly `checkoutMandate` and `paymentMandate`; each value is an unchanged
upstream SD-JWT presentation. The accepted media type is
`application/vnd.layerx.ap2-mandate-pair+json` and the kind is `Mandate`.

Construct the verifier with a trusted `KeyResolver` and a real `ConformanceSuite`.
Use its descriptor's adapter, protocol and version for the presentation and pass
the explicit AP2 `VerificationContext` to `verify_external_evidence`. The adapter
verifies the pair together and returns `VerifiedMandates` inside the binding.
Read `evidence_digest()` before consuming the wrapper with `into_verified()`.

### Supported Protocols

The generic trait permits additional implementations; it is not a list of
implemented or qualified upstream protocols. In this tree AP2 supplies the
concrete external mandate verifier above. Other interop adapters expose their
own verification and transport APIs. Consult their actual compatibility pages,
including [x402](../../services/interop/crates/layerx-x402/COMPATIBILITY.md), for declared coverage;
there is no blanket claim of upstream reference-vector conformance.

## Testing and Portability Proof

### Golden Vectors

The shared receipt corpus is
[`sequencer-receipt-v1.json`](../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json).
The fixture filename does not denote the portable JSON version. Receipt export
and refusal cases live in [receipt_vectors.rs](../../services/interop/crates/layerx-portable/tests/receipt_vectors.rs)
and [sequencer_vectors.rs](../../services/interop/crates/layerx-portable/tests/sequencer_vectors.rs).
[external_verification.rs](../../services/interop/crates/layerx-portable/tests/external_verification.rs)
covers the generic binding boundary; it does not qualify every external adapter.

### Running Tests

For development from the monorepo root:

```bash
make interop-test-portable
make interop-test-mandates
cargo test --locked --manifest-path services/interop/Cargo.toml -p layerx-portable --test receipt_vectors
cargo test --locked --manifest-path services/interop/Cargo.toml -p layerx-portable --test external_verification
cargo test --locked --manifest-path services/interop/Cargo.toml -p layerx-portable --test independent_verifier
```

`interop-test-portable` covers the portable crate; `interop-test-mandates` runs
the AP2 crate, including the recorded guide example check.

### Portability Proof

The independent harness exercises recorded successful and rejected receipts,
field substitutions, key-window boundaries and transaction binding against
independently supplied fixture keys. Its Python implementation checks the same
wire format using general cryptographic libraries. This is offline format and
signature coverage, not a live-network or consensus-finality qualification.

## Security Considerations

### Trust Boundaries

Keep receipt trust keys independent of the proof being checked. Treat parsing,
signature validity, payment matching and proof of commit as separate checks.
For external imports, trust the verifier and its explicit context independently
of its metadata; authenticate any report that crosses a process boundary.
Raw mandates and adapter results can contain sensitive payment information.

### What Verification Does NOT Guarantee

Portable signature verification does not establish upstream availability,
external-chain finality, protection against local storage rollback or absence of
equivocation. External evidence verification establishes only the checks actually
performed by the selected adapter under its supplied context.

## References

- [Format specification](../../services/interop/crates/layerx-portable/FORMAT.md)
- [Public API](../../services/interop/crates/layerx-portable/src/lib.rs)
- [Portable test sources](../../services/interop/crates/layerx-portable/tests/)
- [x402 compatibility](../../services/interop/crates/layerx-x402/COMPATIBILITY.md)
- [Migration operations](../../services/interop/crates/layerx-migrate/OPERATIONS.md)
- [Mirror archives](../../services/interop/crates/layerx-mirror/README.md)
- [Proof library](../../services/agent/crates/layerx-proof/)
