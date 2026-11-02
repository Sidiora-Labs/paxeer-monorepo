# LayerX portable verification formats

`layerx-receipt-proof-v1` is a UTF-8 JSON object with exactly these
camel-cased members, in any JSON member order:

| Member | Value |
|---|---|
| `format` | Literal `layerx-receipt-proof-v1` |
| `verificationLevel` | Literal `sequencer-signed` |
| `canonicalReceipt` | The complete canonical LayerX receipt, unpadded base64url |
| `receiptDigest` | The 32-byte receipt-signature digest, unpadded base64url |
| `batchId` | The claimed 32-byte authorised batch id, unpadded base64url |
| `asset` | The claimed 32-byte batch asset, unpadded base64url |
| `previousStateRoot` | The claimed 32-byte predecessor root, unpadded base64url |
| `resultingStateRoot` | The claimed 32-byte successor root, unpadded base64url |
| `sequencerPublicKey` | The claimed 32-byte Ed25519 key, unpadded base64url |

An independent verifier must reject unknown members, padded or non-canonical
base64url, wrong fixed-field lengths, and canonical receipts above 1,048,576
bytes. The object is not its own trust root: before verification, the verifier
must obtain the authorised batch id, asset, root chain and sequencer key from
its own trusted snapshot, certificate or vector manifest. It then:

1. Decodes the canonical receipt and re-encodes it byte-for-byte under the
   LayerX receipt codec. A difference is a refusal.
2. Requires a full protocol receipt, protocol version 1, a non-zero operation
   and activity id, and a sequencer signature.
3. Requires every claimed batch field in the object and receipt to equal the
   independently trusted batch authorization.
4. For a successful result, checks the debit decreases and credit increases by
   exactly the receipt amount using checked unsigned integer arithmetic.
5. Encodes the same receipt without its sequencer signature and computes
   `SHA-256("LXP/v1/receipt\0" || unsigned_canonical_receipt)`.
6. Requires that digest to equal `receiptDigest`, then strictly verifies the
   Ed25519 signature over the 32-byte digest with `sequencerPublicKey`.

A verified rejected outcome remains rejected. This format proves a
sequencer-signed receipt; it does not claim batch inclusion, checkpoint
finality or external settlement.

External mandates and receipts retain their upstream wire formats. LayerX does
not wrap or reinterpret their claims. `ExternalPresentation` names the exact
adapter, protocol version, media type and claim class; the matching adapter
performs its protocol-specific signature, delegation, time, nonce, audience
and constraint checks. `verify_external_evidence` returns the adapter's typed
verified value bound to the exact presentation bytes and pinned upstream
specification and conformance-suite digests. It retains only that
domain-separated digest, never the possibly secret-bearing raw mandate.

AP2 Checkout and Payment Mandates must be verified as a bound pair. The AP2
adapter therefore accepts
`application/vnd.layerx.ap2-mandate-pair+json`, an object containing exactly
`checkoutMandate` and `paymentMandate`. Their string contents are the unchanged
upstream SD-JWT presentations; the JSON object is only a LayerX transport
envelope. `Ap2ExternalMandateVerifier` passes both exact strings into AP2's
signature, selective-disclosure, delegation and constraint verifier and
returns its private `VerifiedMandates` type through the generic binding.

## Verification Portability

Both LayerX receipts and external protocol evidence are designed for verification
by independent implementations without LayerX infrastructure:

### LayerX Receipt Portability

The current portable sequencer proof is `layerx-receipt-proof-v2`. An external
verifier needs this format and an independently pinned `ReceiptKeys` list; no
LayerX SDK, batch authorization, gateway, node, database, clock or network is
required. Python implementations can use `cryptography` for Ed25519 and
`pycryptodome` for Keccak-256.

A v2 proof is a UTF-8 JSON object with exactly these members (plus the optional
`rawTransaction`). Reject duplicate or unknown members at either object level.

| Member | Value |
|---|---|
| `format` | Literal `layerx-receipt-proof-v2` |
| `verificationLevel` | Literal `receipted`, including exports of committed answers |
| `signedReceipt` | Complete canonical signed ProtocolReceipt v2 bytes |
| `receiptDigest` | 32-byte receipt-signature digest |
| `receiptKey` | Object with exactly `keyId`, `publicKey`, `firstHeight`, `lastHeight` |
| `inclusionHeight` | Height no lower than the signed receipt height |
| `rawTransaction` | Optional raw signed transaction, bound to the activity id |

All byte strings use lowercase, even-length `0x` hex. `keyId` and `publicKey`
are exactly 32 bytes. Heights are unsigned 64-bit lowercase `0x` quantities,
without leading zeroes except `0x0`. Receipt and raw transaction bytes must each
be nonempty and at most 1,048,576 bytes; the JSON input is at most 1,500,000 bytes.

For the sequencer layout, decode the following fields in order. All integers
are fixed-width big-endian; `bytes32` means a u32 length equal to 32 followed
by exactly 32 bytes. Reject truncation, invalid lengths and trailing bytes.

| Fields in wire order | Encoding |
|---|---|
| Envelope version, tag, protocol version | u16 = 2, u16 = 0x5201, u16 = 2 |
| Activity id, global sequence | bytes32 (nonzero), u64 |
| Previous state root, resulting state root, activity root | three bytes32 |
| Result code, effect count | i32, u32 |
| Each effect | u16 module, u16 ordinal, u16 event type, u8 kind, u8 monetary, bytes32 transfer root, u32 body length, body |
| Fee charged, batch id | u128, bytes32 |
| Module id, module version, parameter version, operation | u16 > 11, u32 = 1, u32, u8 |
| Asset, amount, from | bytes32, u128, bytes32 |
| From balance before, after, sequence | u128, u128, u64 |
| To, to balance before, after | bytes32, u128, u128 |
| Transfer set root, authorization hash, context hash | three bytes32 |
| Timestamp, signature | u64, u8 = 1, u32 = 64, 64 signature bytes |

Effect kind is 1, 2 or 3; monetary is 0 or 1 and may be 1 only for kind 2.
Each effect body is at most 256 bytes and the effect count is at most
floor(1,048,576 / 48). This sequencer layout has no supply or program-outcome
extension. The first 24 bytes of batch id must be zero; its final eight bytes
are the signed height. Context hash is the signed key id.

Select exactly one entry from the verifier's pinned list by that signed key
id; reject missing or duplicate identities. Its inclusive `firstHeight` to
`lastHeight` window must cover the **signed height**, and the carried
`receiptKey` must equal that entire pinned entry. Never trust a key supplied
only by the proof, or select its window using `inclusionHeight`.
The signed height may equal either window endpoint; a later inclusion height
may be outside that window.

Replace the final 69 bytes (`01 00000040` and the 64-byte signature) with a
single zero signature-presence byte to obtain the unsigned preimage. Compute
`sha256("LXP/v1/receipt\0" || unsigned_preimage)` (the domain ends with a NUL),
require equality with `receiptDigest`, and verify the Ed25519 signature over
that 32-byte digest using the selected pinned public key. If `rawTransaction`
is present, require `keccak256(rawTransaction)` to equal the signed activity
id; Keccak-256 is distinct from SHA3-256.

Require `inclusionHeight` to be at least the signed height. This carried height
does not prove inclusion or finality: a v2 proof establishes a sequencer-signed
receipt only. A signed nonzero result code remains a rejected transaction.
The shared `sdk/platform/conformance/fixtures/sequencer-receipt-v1.json` vectors
exercise successful and reverted receipts in both independent harnesses.

### External Evidence Portability

External mandates and receipts (x402, AP2, UCP, etc.) are verified through
version-pinned adapters. Each adapter:
1. Declares its exact protocol, spec version, spec document digest, and conformance suite
2. Performs protocol-specific cryptographic and constraint verification
3. Returns a typed verified value bound to the exact presentation bytes

`verify_external_evidence` ensures the presentation matches the verifier's
declared adapter, protocol, version, evidence kind, and media type before
invoking adapter verification. The resulting `VerifiedExternalEvidence`
carries the adapter's typed output plus all binding metadata:
- Exact spec version and document digest
- Conformance suite name, vector count, and suite digest  
- Evidence digest (domain-separated hash of all inputs)

This binding means an external system can trust the verification provenance
without re-running the full verification or storing secret-bearing raw mandates.
The evidence digest is sufficient for audit and non-repudiation.

Test coverage in `tests/external_verification.rs` proves typed rigour,
descriptor matching, and digest stability across payload changes.
