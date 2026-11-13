# Interop portable contract

The [portable evidence library](../../../services/interop/crates/layerx-portable/src/lib.rs) exports `PortableReceiptV2` documents and verifies them offline against independently pinned `ReceiptKeyEntry` values. `PORTABLE_RECEIPT_FORMAT` is `layerx-receipt-proof-v2`. Protocol adapters retain ownership of external-evidence cryptography and constraints.

## fmt-portable-format-probe

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-portable-format-probe</code> | <code>file_format</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>Rust model: struct FormatProbe { format: String, }</code> |

## fmt-portable-portable-receipt-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt-key</code> | <code>file_format</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>Rust model: #[serde(rename_all = "camelCase", deny_unknown_fields)] struct PortableReceiptKey { key_id: String, public_key: String, first_height: String, last_height: String, }</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## fmt-portable-portable-receipt-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt-v2</code> | <code>file_format</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>Rust model: #[serde(rename_all = "camelCase", deny_unknown_fields)] pub struct PortableReceiptV2 { format: String, verification_level: String, signed_receipt: String, receipt_digest: String, receipt_key: PortableReceiptKey, inclusion_height: String, #[serde(default, skip_serializing_if = "Option::is_none")] raw_transaction: Option&lt;String&gt;, }</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-external-evidence-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-evidence-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub enum ExternalEvidenceKind</code> |

## portable-external-evidence-verifier

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-evidence-verifier</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub trait ExternalEvidenceVerifier&lt;C: ?Sized&gt;</code> |

## portable-external-presentation

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub struct ExternalPresentation&lt;'a&gt;</code> |

## portable-external-presentation-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn adapter(&amp;self) -&gt; &amp;str</code> |

## portable-external-presentation-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub enum ExternalPresentationError</code> |

## portable-external-presentation-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn kind(&amp;self) -&gt; ExternalEvidenceKind</code> |

## portable-external-presentation-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-media-type</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn media_type(&amp;self) -&gt; &amp;str</code> |

## portable-external-presentation-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-new</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn new(adapter: &amp;'a str, protocol: &amp;'a str, spec_version: &amp;'a str, kind: ExternalEvidenceKind, media_type: &amp;'a str, payload: &amp;'a [u8]) -&gt; Result&lt;Self, ExternalPresentationError&gt;</code> |

## portable-external-presentation-payload

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-payload</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn payload(&amp;self) -&gt; &amp;[u8]</code> |

## portable-external-presentation-protocol

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-protocol</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn protocol(&amp;self) -&gt; &amp;str</code> |

## portable-external-presentation-spec-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-spec-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn spec_version(&amp;self) -&gt; &amp;str</code> |

## portable-external-verification-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-external-verification-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub enum ExternalVerificationError&lt;E&gt;</code> |

## portable-interop-portable-verification

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-interop-portable-verification</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/lib.rs](../../../services/interop/crates/layerx-portable/src/lib.rs) | <code>pub const fn interop_portable_verification() -&gt; &amp;'static str</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-error

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-error</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub enum PortableReceiptError</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-format

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-format</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const PORTABLE_RECEIPT_FORMAT: &amp;str</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub struct PortableReceiptV2</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-v2-export

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-export</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn export(signed_receipt: &amp;[u8], receipt_key: &amp;ReceiptKeyEntry, inclusion_height: u64) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-v2-format

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-format</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn format(&amp;self) -&gt; &amp;str</code> |

## portable-portable-receipt-v2-from-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-from-json</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn from_json(bytes: &amp;[u8]) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |

## portable-portable-receipt-v2-to-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-to-json</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn to_json(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, PortableReceiptError&gt;</code> |

## portable-portable-receipt-v2-verify-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-verify-json</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn verify_json(bytes: &amp;[u8], receipt_keys: &amp;[ReceiptKeyEntry]) -&gt; Result&lt;PortableVerifiedReceiptV2, PortableReceiptError&gt;</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-v2-verify-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-verify-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn verify_v2(&amp;self, receipt_keys: &amp;[ReceiptKeyEntry]) -&gt; Result&lt;PortableVerifiedReceiptV2, PortableReceiptError&gt;</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-verified

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub enum PortableVerified</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-verified-receipt-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub struct PortableVerifiedReceiptV2</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-verified-receipt-v2-activity-id

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-activity-id</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn activity_id(&amp;self) -&gt; [u8; 32]</code> |

## portable-portable-verified-receipt-v2-global-sequence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-global-sequence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn global_sequence(&amp;self) -&gt; u64</code> |

## portable-portable-verified-receipt-v2-height

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-height</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn height(&amp;self) -&gt; u64</code> |

## portable-portable-verified-receipt-v2-inclusion-height

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-inclusion-height</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn inclusion_height(&amp;self) -&gt; u64</code> |

## portable-portable-verified-receipt-v2-receipt-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-receipt-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn receipt_digest(&amp;self) -&gt; [u8; 32]</code> |

## portable-portable-verified-receipt-v2-receipt-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-receipt-key</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn receipt_key(&amp;self) -&gt; &amp;ReceiptKeyEntry</code> |

## portable-portable-verified-receipt-v2-result-code

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-result-code</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub const fn result_code(&amp;self) -&gt; i32</code> |

## portable-portable-verified-receipt-v2-signed-receipt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-signed-receipt</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn signed_receipt(&amp;self) -&gt; &amp;[u8]</code> |

## portable-receipt-key-entry

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-receipt-key-entry</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub type ReceiptKeyEntry = SequencerAuthorization</code> |

## portable-verified-external-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub struct VerifiedExternalEvidence&lt;T&gt;</code> |

## portable-verified-external-evidence-adapter

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-adapter</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn adapter(&amp;self) -&gt; &amp;str</code> |

## portable-verified-external-evidence-conformance-suite

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-suite</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn conformance_suite(&amp;self) -&gt; &amp;str</code> |

## portable-verified-external-evidence-conformance-suite-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-suite-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn conformance_suite_digest(&amp;self) -&gt; [u8; 32]</code> |

## portable-verified-external-evidence-conformance-vector-count

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-vector-count</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn conformance_vector_count(&amp;self) -&gt; u64</code> |

## portable-verified-external-evidence-evidence-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-evidence-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

## portable-verified-external-evidence-into-verified

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-into-verified</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn into_verified(self) -&gt; T</code> |

## portable-verified-external-evidence-kind

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-kind</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn kind(&amp;self) -&gt; ExternalEvidenceKind</code> |

## portable-verified-external-evidence-media-type

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-media-type</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn media_type(&amp;self) -&gt; &amp;str</code> |

## portable-verified-external-evidence-protocol

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-protocol</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn protocol(&amp;self) -&gt; &amp;str</code> |

## portable-verified-external-evidence-spec-document-digest

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-spec-document-digest</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn spec_document_digest(&amp;self) -&gt; [u8; 32]</code> |

## portable-verified-external-evidence-spec-version

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-spec-version</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn spec_version(&amp;self) -&gt; &amp;str</code> |

## portable-verified-external-evidence-verified

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-verified</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub const fn verified(&amp;self) -&gt; &amp;T</code> |

## portable-verify-external-evidence

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verify-external-evidence</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/external.rs](../../../services/interop/crates/layerx-portable/src/external.rs) | <code>pub fn verify_external_evidence&lt;C: ?Sized, V: ExternalEvidenceVerifier&lt;C&gt;&gt;(verifier: &amp;V, presentation: &amp;ExternalPresentation&lt;'_&gt;, context: &amp;C) -&gt; Result&lt;VerifiedExternalEvidence&lt;V::Verified&gt;, ExternalVerificationError&lt;V::Error&gt;&gt;</code> |

## portable-verify-portable-json

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.11.rs-portable-verify-portable-json</code> | <code>rust_pub_item</code> | [services/interop/crates/layerx-portable/src/receipt.rs](../../../services/interop/crates/layerx-portable/src/receipt.rs) | <code>pub fn verify_portable_json( bytes: &amp;[u8], receipt_keys: &amp;[ReceiptKeyEntry], ) -&gt; Result&lt;PortableVerified, PortableReceiptError&gt;</code> |

| Field | Rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |
