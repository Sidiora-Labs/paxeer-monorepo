# LayerX portable verification formats

The current receipt format is `layerx-receipt-proof-v2`. The public API exports
`PortableReceiptV2`, `ReceiptKeyEntry`, `PortableVerifiedReceiptV2` and
`verify_portable_json`. The historical v1/base64 format and `AuthorizedBatch`
verification path are retired; `verify_portable_json` refuses v1 with
`PortableReceiptError::UnsupportedFormat`.

Paxeer mainnet-beta is one network, chain ID 125 (`0x7d`). The kernel participates
in that network and proposes blocks; block commit is finality. A portable proof
at level `receipted` authenticates a signature and does not prove block commit.
For acquisition and application guidance, see [PORTABILITY.md](../../PORTABILITY.md).

## Verification Portability

### LayerX Receipt Portability

An offline verifier needs the format and an independently pinned `ReceiptKeys`
list. It needs no gateway, node, database, clock or network connection. The
[Python implementation](tests/python_portable_verifier.py) uses `cryptography`
for Ed25519 and `pycryptodome` for Keccak-256.

A v2 proof is a UTF-8 JSON object with exactly these members (plus the optional
`rawTransaction`). Reject duplicate or unknown members at either object level.

| Member | Value |
| --- | --- |
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
| --- | --- |
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

External presentations keep their upstream wire formats. The generic
`ExternalEvidenceVerifier<C>` boundary leaves protocol-specific cryptography,
delegation, time, nonce, audience and constraint checks to the selected adapter.
`ExternalPresentation::new` validates identifiers, the exact spec version, a
nonempty media type of at most 128 bytes without ASCII control characters, and
a payload of 1–2,097,152 bytes.

`verify_external_evidence` compares adapter ID, protocol, exact version, evidence
kind and media type with the verifier's declaration before invoking it. Adapter
errors are preserved. On success, `VerifiedExternalEvidence<T>` carries the typed
result and binding metadata, without retaining the borrowed presentation bytes.
The adapter's output type can itself contain sensitive values.

The evidence digest is SHA-256 over the following concatenation:

```text
"LayerX/interop/external-evidence/v1\0" || kind_tag ||
len(adapter) || adapter || len(protocol) || protocol ||
len(spec_version) || spec_version || spec_document_digest ||
len(conformance_suite) || conformance_suite || conformance_vector_count ||
conformance_suite_digest || len(media_type) || media_type || len(payload) || payload
```

The domain ends in NUL. Kind tags are one byte (`Mandate` = 1, `Receipt` = 2).
Lengths and vector count are unsigned 64-bit big-endian; specification and suite
digests are 32 raw bytes. Context and typed output are not hashed.

This digest identifies bytes and metadata. It does not authenticate the verifier,
prove that a conformance suite ran, or provide non-repudiation. A downstream
consumer needs an authenticated verification report from a trusted producer or
must re-run verification with trusted keys and context. A digest alone also does
not guarantee privacy for predictable or linkable payloads.

The concrete generic import in this tree is AP2's
[`Ap2ExternalMandateVerifier`](../layerx-ap2/src/portable.rs). Its
`application/vnd.layerx.ap2-mandate-pair+json` envelope contains exactly
`checkoutMandate` and `paymentMandate`, holding unchanged upstream SD-JWT
presentations. It verifies both as a bound pair using the caller's `KeyResolver`,
conformance declaration and `VerificationContext`, and returns `VerifiedMandates`.
The generic trait does not establish support or upstream conformance for every
interop adapter; see [x402 compatibility](../layerx-x402/COMPATIBILITY.md) for that
adapter's coverage limits.

The [external binding tests](tests/external_verification.rs) cover descriptor and
kind matching, typed results and digest binding. The
[independent verifier harness](tests/independent_verifier.rs) covers receipt
portability with the Python implementation and independently supplied fixture
keys. These sources do not establish live-network settlement or verifier provenance.
