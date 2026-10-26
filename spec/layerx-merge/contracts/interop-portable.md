# Interop portable contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## fmt-portable-format-probe

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-portable-format-probe</code> | <code>file_format</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:598</code> | <code>FormatProbe { format: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-portable-format-probe</code> | notes | portable receipt JSON via serde Deserialize |
| <code>interface.11.fmt-portable-format-probe</code> | overridden_by | override.J48.44 |

## fmt-portable-portable-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt</code> | <code>file_format</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:34</code> | <code>PortableReceipt { format: String, verificationLevel: String, canonicalReceipt: String, receiptDigest: String, batchId: String, asset: String, previousStateRoot: String, resultingStateRoot: String, sequencerPublicKey: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt</code> | notes | portable receipt JSON via serde Clone, Debug, Eq, PartialEq, Deserialize, Serialize, deny_unknown_fields, rename_all camelCase |
| <code>interface.11.fmt-portable-portable-receipt</code> | overridden_by | override.J48.43 |

## fmt-portable-portable-receipt-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt-key</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:359</code> | <code>PortableReceiptKey { keyId: String, publicKey: String, firstHeight: String, lastHeight: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt-key</code> | notes | portable receipt JSON via serde Clone, Debug, Eq, PartialEq, Deserialize, Serialize, deny_unknown_fields, rename_all camelCase |
| <code>interface.11.fmt-portable-portable-receipt-key</code> | overridden_by | override.J48.30 |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt-v2</code> | <code>file_format</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:348</code> | <code>PortableReceiptV2 { format: String, verificationLevel: String, signedReceipt: String, receiptDigest: String, receiptKey: PortableReceiptKey, inclusionHeight: String }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-portable-portable-receipt-v2</code> | notes | portable receipt JSON via serde Clone, Debug, Eq, PartialEq, Deserialize, Serialize, deny_unknown_fields, rename_all camelCase |
| <code>interface.11.fmt-portable-portable-receipt-v2</code> | overridden_by | override.J48.29 |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-evidence-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:12</code> | <code>pub enum ExternalEvidenceKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-evidence-kind</code> | notes | enum layerx_portable::external::ExternalEvidenceKind; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17 |

## portable-external-evidence-verifier

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-evidence-verifier</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:117</code> | <code>pub trait ExternalEvidenceVerifier&lt;C: ?Sized&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-evidence-verifier</code> | notes | trait layerx_portable::external::ExternalEvidenceVerifier; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17 |

## portable-external-presentation

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:32</code> | <code>pub struct ExternalPresentation&lt;'a&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation</code> | notes | struct layerx_portable::external::ExternalPresentation; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17 |

## portable-external-presentation-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:81</code> | <code>pub const fn adapter(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-adapter</code> | notes | method layerx_portable::external::ExternalPresentation::adapter |

## portable-external-presentation-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:273</code> | <code>pub enum ExternalPresentationError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-error</code> | notes | enum layerx_portable::external::ExternalPresentationError; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17 |

## portable-external-presentation-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:96</code> | <code>pub const fn kind(&amp;self) -&gt; ExternalEvidenceKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-kind</code> | notes | method layerx_portable::external::ExternalPresentation::kind; no reference outside the defining file found by name match |

## portable-external-presentation-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-media-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:101</code> | <code>pub const fn media_type(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-media-type</code> | notes | method layerx_portable::external::ExternalPresentation::media_type |

## portable-external-presentation-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-new</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:50</code> | <code>pub fn new(adapter: &amp;'a str, protocol: &amp;'a str, spec_version: &amp;'a str, kind: ExternalEvidenceKind, media_type: &amp;'a str, payload: &amp;'a [u8]) -&gt; Result&lt;Self, ExternalPresentationError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-new</code> | notes | method layerx_portable::external::ExternalPresentation::new |

## portable-external-presentation-payload

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-payload</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:106</code> | <code>pub const fn payload(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-payload</code> | notes | method layerx_portable::external::ExternalPresentation::payload; no reference outside the defining file found by name match |

## portable-external-presentation-protocol

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-protocol</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:86</code> | <code>pub const fn protocol(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-protocol</code> | notes | method layerx_portable::external::ExternalPresentation::protocol |

## portable-external-presentation-spec-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-presentation-spec-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:91</code> | <code>pub const fn spec_version(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-presentation-spec-version</code> | notes | method layerx_portable::external::ExternalPresentation::spec_version |

## portable-external-verification-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-external-verification-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:301</code> | <code>pub enum ExternalVerificationError&lt;E&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-external-verification-error</code> | notes | enum layerx_portable::external::ExternalVerificationError; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17 |

## portable-interop-portable-verification

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-interop-portable-verification</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/lib.rs:29</code> | <code>pub const fn interop_portable_verification() -&gt; &amp;'static str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-interop-portable-verification</code> | notes | fn layerx_portable::interop_portable_verification |
| <code>interface.11.rs-portable-interop-portable-verification</code> | overridden_by | override.J48.42 |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:34</code> | <code>pub struct PortableReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt</code> | notes | struct layerx_portable::receipt::PortableReceipt; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21; spec/layerx-merge/spec.kvx task 10.5.43 changes export (services/interop/crates/layerx-portable/src/receipt.rs:45) and verify (services/interop/crates/layerx-portable/src/receipt.rs:120, checks R1-R10) of the v1 receipt |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-canonical-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-canonical-receipt</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:167</code> | <code>pub fn canonical_receipt(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-canonical-receipt</code> | notes | method layerx_portable::receipt::PortableReceipt::canonical_receipt |
| <code>interface.11.rs-portable-portable-receipt-canonical-receipt</code> | overridden_by | override.J48.36 |

## portable-portable-receipt-error

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-error</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:242</code> | <code>pub enum PortableReceiptError</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-error</code> | notes | enum layerx_portable::receipt::PortableReceiptError; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21 |
| <code>interface.11.rs-portable-portable-receipt-error</code> | overridden_by | override.J48.28 |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-export

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-export</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:55</code> | <code>pub fn export(canonical_receipt: &amp;[u8], authorised: &amp;AuthorizedBatch) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-export</code> | notes | method layerx_portable::receipt::PortableReceipt::export; spec/layerx-merge/spec.kvx task 10.5.43 changes export (services/interop/crates/layerx-portable/src/receipt.rs:45) and verify (services/interop/crates/layerx-portable/src/receipt.rs:120, checks R1-R10) of the v1 receipt |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-format</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:14</code> | <code>pub const PORTABLE_RECEIPT_FORMAT: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-format</code> | notes | const layerx_portable::receipt::PORTABLE_RECEIPT_FORMAT; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21; spec/layerx-merge/spec.kvx task 10.5.43 changes PORTABLE_RECEIPT_FORMAT to layerx-receipt-proof-v2; HEAD keeps it v1 and adds a separate PORTABLE_RECEIPT_FORMAT_V2 constant at services/interop/crates/layerx-portable/src/receipt.rs:17 |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-format-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-format-v2</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:17</code> | <code>pub const PORTABLE_RECEIPT_FORMAT_V2: &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-format-v2</code> | notes | const layerx_portable::receipt::PORTABLE_RECEIPT_FORMAT_V2; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21 |
| <code>interface.11.rs-portable-portable-receipt-format-v2</code> | overridden_by | override.J48.31 |

## portable-portable-receipt-from-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-from-json</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:87</code> | <code>pub fn from_json(bytes: &amp;[u8]) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-from-json</code> | notes | method layerx_portable::receipt::PortableReceipt::from_json |
| <code>interface.11.rs-portable-portable-receipt-from-json</code> | overridden_by | override.J48.32 |

## portable-portable-receipt-receipt-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-receipt-digest</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:173</code> | <code>pub fn receipt_digest(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-receipt-digest</code> | notes | method layerx_portable::receipt::PortableReceipt::receipt_digest |
| <code>interface.11.rs-portable-portable-receipt-receipt-digest</code> | overridden_by | override.J48.37 |

## portable-portable-receipt-to-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-to-json</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:117</code> | <code>pub fn to_json(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-to-json</code> | notes | method layerx_portable::receipt::PortableReceipt::to_json; no reference outside the defining file found by name match |
| <code>interface.11.rs-portable-portable-receipt-to-json</code> | overridden_by | override.J48.34 |

## portable-portable-receipt-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:348</code> | <code>pub struct PortableReceiptV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2</code> | notes | struct layerx_portable::receipt::PortableReceiptV2; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21; spec/layerx-merge/spec.kvx task 6.5.19 adds rawTransaction at receipted and blockHeader, receiptInclusion, commit, validatorSet at committed (layerx_committed_proof); HEAD PortableReceiptV2 has only format, verificationLevel, signedReceipt, receiptDigest, receiptKey, inclusionHeight |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-export</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:374</code> | <code>pub fn export(signed_receipt: &amp;[u8], receipt_key: &amp;ReceiptKeyEntry, inclusion_height: u64) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-export</code> | notes | method layerx_portable::receipt::PortableReceiptV2::export; spec/layerx-merge/spec.kvx task 6.5.19 adds rawTransaction at receipted and blockHeader, receiptInclusion, commit, validatorSet at committed (layerx_committed_proof); HEAD PortableReceiptV2 has only format, verificationLevel, signedReceipt, receiptDigest, receiptKey, inclusionHeight |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-format</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:488</code> | <code>pub fn format(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-format</code> | notes | method layerx_portable::receipt::PortableReceiptV2::format; no reference outside the defining file found by name match |

## portable-portable-receipt-v2-from-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-from-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:406</code> | <code>pub fn from_json(bytes: &amp;[u8]) -&gt; Result&lt;Self, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-from-json</code> | notes | method layerx_portable::receipt::PortableReceiptV2::from_json; no reference outside the defining file found by name match |

## portable-portable-receipt-v2-to-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-to-json</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:421</code> | <code>pub fn to_json(&amp;self) -&gt; Result&lt;Vec&lt;u8&gt;, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-to-json</code> | notes | method layerx_portable::receipt::PortableReceiptV2::to_json; no reference outside the defining file found by name match |

## portable-portable-receipt-v2-verify-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-verify-json</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:479</code> | <code>pub fn verify_json(bytes: &amp;[u8], receipt_keys: &amp;[ReceiptKeyEntry]) -&gt; Result&lt;PortableVerifiedReceiptV2, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-verify-json</code> | notes | method layerx_portable::receipt::PortableReceiptV2::verify_json; spec/layerx-merge/spec.kvx task 6.5.19 adds rawTransaction at receipted and blockHeader, receiptInclusion, commit, validatorSet at committed (layerx_committed_proof); HEAD PortableReceiptV2 has only format, verificationLevel, signedReceipt, receiptDigest, receiptKey, inclusionHeight |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-verify-v2</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:434</code> | <code>pub fn verify_v2(&amp;self, receipt_keys: &amp;[ReceiptKeyEntry]) -&gt; Result&lt;PortableVerifiedReceiptV2, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-v2-verify-v2</code> | notes | method layerx_portable::receipt::PortableReceiptV2::verify_v2; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 6.5.19 adds rawTransaction at receipted and blockHeader, receiptInclusion, commit, validatorSet at committed (layerx_committed_proof); HEAD PortableReceiptV2 has only format, verificationLevel, signedReceipt, receiptDigest, receiptKey, inclusionHeight |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-verify

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-verify</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:130</code> | <code>pub fn verify(&amp;self, trusted_batch: &amp;AuthorizedBatch) -&gt; Result&lt;PortableVerifiedReceipt, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-verify</code> | notes | method layerx_portable::receipt::PortableReceipt::verify; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 10.5.43 changes export (services/interop/crates/layerx-portable/src/receipt.rs:45) and verify (services/interop/crates/layerx-portable/src/receipt.rs:120, checks R1-R10) of the v1 receipt |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-receipt-verify-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-verify-json</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:103</code> | <code>pub fn verify_json(bytes: &amp;[u8], trusted_batch: &amp;AuthorizedBatch) -&gt; Result&lt;PortableVerifiedReceipt, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-receipt-verify-json</code> | notes | method layerx_portable::receipt::PortableReceipt::verify_json; no reference outside the defining file found by name match |
| <code>interface.11.rs-portable-portable-receipt-verify-json</code> | overridden_by | override.J48.33 |

## portable-portable-verified

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:592</code> | <code>pub enum PortableVerified</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified</code> | notes | enum layerx_portable::receipt::PortableVerified; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21 |
| <code>interface.11.rs-portable-portable-verified</code> | overridden_by | override.J48.41 |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |

## portable-portable-verified-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:214</code> | <code>pub struct PortableVerifiedReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt</code> | notes | struct layerx_portable::receipt::PortableVerifiedReceipt; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21; no reference outside the defining file found by name match |
| <code>interface.11.rs-portable-portable-verified-receipt</code> | overridden_by | override.J48.38 |

## portable-portable-verified-receipt-authorised-batch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-authorised-batch</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:235</code> | <code>pub const fn authorised_batch(&amp;self) -&gt; &amp;AuthorizedBatch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-authorised-batch</code> | notes | method layerx_portable::receipt::PortableVerifiedReceipt::authorised_batch; no reference outside the defining file found by name match; authorised batch evidence is replaced by sequencer receipts (spec/layerx-merge/spec.kvx task 7.5.17, 7.5.18, 7.5.21) |

## portable-portable-verified-receipt-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-receipt</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:223</code> | <code>pub const fn receipt(&amp;self) -&gt; &amp;VerifiedReceipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-receipt</code> | notes | method layerx_portable::receipt::PortableVerifiedReceipt::receipt; no reference outside the defining file found by name match |
| <code>interface.11.rs-portable-portable-verified-receipt-receipt</code> | overridden_by | override.J48.39 |

## portable-portable-verified-receipt-receipt-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-receipt-digest</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:229</code> | <code>pub const fn receipt_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-receipt-digest</code> | notes | method layerx_portable::receipt::PortableVerifiedReceipt::receipt_digest; no reference outside the defining file found by name match |
| <code>interface.11.rs-portable-portable-verified-receipt-receipt-digest</code> | overridden_by | override.J48.40 |

## portable-portable-verified-receipt-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:529</code> | <code>pub struct PortableVerifiedReceiptV2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2</code> | notes | struct layerx_portable::receipt::PortableVerifiedReceiptV2; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21; no reference outside the defining file found by name match; spec/layerx-merge/spec.kvx task 6.5.19 adds rawTransaction at receipted and blockHeader, receiptInclusion, commit, validatorSet at committed (layerx_committed_proof); HEAD PortableReceiptV2 has only format, verificationLevel, signedReceipt, receiptDigest, receiptKey, inclusionHeight |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
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

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-activity-id</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:561</code> | <code>pub const fn activity_id(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-activity-id</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::activity_id; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-global-sequence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-global-sequence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:567</code> | <code>pub const fn global_sequence(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-global-sequence</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::global_sequence; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-height

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-height</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:579</code> | <code>pub const fn height(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-height</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::height; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-inclusion-height

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-inclusion-height</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:585</code> | <code>pub const fn inclusion_height(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-inclusion-height</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::inclusion_height; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-receipt-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-receipt-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:549</code> | <code>pub const fn receipt_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-receipt-digest</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::receipt_digest; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-receipt-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-receipt-key</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:555</code> | <code>pub const fn receipt_key(&amp;self) -&gt; &amp;ReceiptKeyEntry</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-receipt-key</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::receipt_key; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-result-code

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-result-code</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:573</code> | <code>pub const fn result_code(&amp;self) -&gt; i32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-result-code</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::result_code; no reference outside the defining file found by name match |

## portable-portable-verified-receipt-v2-signed-receipt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-signed-receipt</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:543</code> | <code>pub fn signed_receipt(&amp;self) -&gt; &amp;[u8]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-portable-verified-receipt-v2-signed-receipt</code> | notes | method layerx_portable::receipt::PortableVerifiedReceiptV2::signed_receipt; no reference outside the defining file found by name match |

## portable-receipt-key-entry

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-receipt-key-entry</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:339</code> | <code>pub type ReceiptKeyEntry = SequencerAuthorization</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-receipt-key-entry</code> | notes | type alias layerx_portable::receipt::ReceiptKeyEntry; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21 |

## portable-receipt-portable-receipt-format

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-receipt-portable-receipt-format</code> | <code>rust_pub_item</code> | <code>retire</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:161</code> | <code>pub fn format(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-receipt-portable-receipt-format</code> | notes | method layerx_portable::receipt::PortableReceipt::format; no reference outside the defining file found by name match |
| <code>interface.11.rs-portable-receipt-portable-receipt-format</code> | overridden_by | override.J48.35 |

## portable-receipt-v2-document

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.fmt-portability-receipt-proof-v2-section-j47</code> | <code>file_format</code> | <code>new</code> | <code>services/interop/PORTABILITY.md</code> (planned) | <code>PORTABILITY.md section: portable receipt version 2 (signed receipt, receipt key, inclusion height)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.fmt-portability-receipt-proof-v2-section-j47</code> | notes | Documents PortableReceiptV2; task 11.10.1. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.11.4.1</code> | req | 11004.1 |
| <code>acceptance.11.4.1</code> | scenario | Given the fenced JSON block under the v2 example heading of PORTABILITY.md, when PortableReceiptV2::from_json parses it, then parsing succeeds and the format field is layerx-receipt-proof-v2. |
| <code>acceptance.11.4.1</code> | test | services/interop/crates/layerx-ap2/tests/portability_v2.rs::documented_v2_example_parses_and_verifies (planned) |
| <code>acceptance.11.4.1</code> | qualification | UNRUN |
| <code>req.11004</code> | ac_1 | The v2 example in PORTABILITY.md parses with PortableReceiptV2::from_json and carries format layerx-receipt-proof-v2 (services/interop/crates/layerx-portable/src/receipt.rs:17) (planned). |
| <code>acceptance.11.45.1</code> | req | 11045.1 |
| <code>acceptance.11.45.1</code> | scenario | Given the fenced v2 example of PORTABILITY.md, when from_json parses it, then the receipt digest equals the recorded receiptDigest. |
| <code>acceptance.11.45.1</code> | test | services/interop/crates/layerx-ap2/tests/portability_v2.rs::portability_v2_example_parses (planned) |
| <code>acceptance.11.45.1</code> | qualification | UNRUN |
| <code>req.11045</code> | ac_1 | The v2 example of PORTABILITY.md parses with PortableReceiptV2::from_json (planned) (services/interop/crates/layerx-portable/src/receipt.rs:406). |
| <code>capability.11.10.1</code> | title | Document the layerx-receipt-proof-v2 export in PORTABILITY.md with a parsed example |
| <code>capability.11.10.1</code> | reqs | 11004.1 |
| <code>capability.11.10.2</code> | title | Add a version 2 example document to PORTABILITY.md taken from the AP2 export |
| <code>capability.11.10.2</code> | reqs | 11045.1 |

## portable-verified-external-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:190</code> | <code>pub struct VerifiedExternalEvidence&lt;T&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence</code> | notes | struct layerx_portable::external::VerifiedExternalEvidence; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17; no reference outside the defining file found by name match |

## portable-verified-external-evidence-adapter

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-adapter</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:206</code> | <code>pub fn adapter(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-adapter</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::adapter; no reference outside the defining file found by name match |

## portable-verified-external-evidence-conformance-suite

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-suite</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:226</code> | <code>pub fn conformance_suite(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-suite</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::conformance_suite; no reference outside the defining file found by name match |

## portable-verified-external-evidence-conformance-suite-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-suite-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:236</code> | <code>pub const fn conformance_suite_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-suite-digest</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::conformance_suite_digest; no reference outside the defining file found by name match |

## portable-verified-external-evidence-conformance-vector-count

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-vector-count</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:231</code> | <code>pub const fn conformance_vector_count(&amp;self) -&gt; u64</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-conformance-vector-count</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::conformance_vector_count; no reference outside the defining file found by name match |

## portable-verified-external-evidence-evidence-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-evidence-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:254</code> | <code>pub const fn evidence_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-evidence-digest</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::evidence_digest; no reference outside the defining file found by name match |

## portable-verified-external-evidence-into-verified

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-into-verified</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:266</code> | <code>pub fn into_verified(self) -&gt; T</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-into-verified</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::into_verified; no reference outside the defining file found by name match |

## portable-verified-external-evidence-kind

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-kind</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:241</code> | <code>pub const fn kind(&amp;self) -&gt; ExternalEvidenceKind</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-kind</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::kind; no reference outside the defining file found by name match |

## portable-verified-external-evidence-media-type

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-media-type</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:246</code> | <code>pub fn media_type(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-media-type</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::media_type; no reference outside the defining file found by name match |

## portable-verified-external-evidence-protocol

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-protocol</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:211</code> | <code>pub fn protocol(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-protocol</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::protocol; no reference outside the defining file found by name match |

## portable-verified-external-evidence-spec-document-digest

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-spec-document-digest</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:221</code> | <code>pub const fn spec_document_digest(&amp;self) -&gt; [u8; 32]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-spec-document-digest</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::spec_document_digest; no reference outside the defining file found by name match |

## portable-verified-external-evidence-spec-version

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-spec-version</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:216</code> | <code>pub fn spec_version(&amp;self) -&gt; &amp;str</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-spec-version</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::spec_version; no reference outside the defining file found by name match |

## portable-verified-external-evidence-verified

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-verified</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:260</code> | <code>pub const fn verified(&amp;self) -&gt; &amp;T</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verified-external-evidence-verified</code> | notes | method layerx_portable::external::VerifiedExternalEvidence::verified; no reference outside the defining file found by name match |

## portable-verify-external-evidence

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verify-external-evidence</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/interop/crates/layerx-portable/src/external.rs:147</code> | <code>pub fn verify_external_evidence&lt;C: ?Sized, V: ExternalEvidenceVerifier&lt;C&gt;&gt;(verifier: &amp;V, presentation: &amp;ExternalPresentation&lt;'_&gt;, context: &amp;C) -&gt; Result&lt;VerifiedExternalEvidence&lt;V::Verified&gt;, ExternalVerificationError&lt;V::Error&gt;&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verify-external-evidence</code> | notes | fn layerx_portable::external::verify_external_evidence; re-exported at services/interop/crates/layerx-portable/src/lib.rs:17 |

## portable-verify-portable-json

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.11.rs-portable-verify-portable-json</code> | <code>rust_pub_item</code> | <code>change</code> | <code>services/interop/crates/layerx-portable/src/receipt.rs:609</code> | <code>pub fn verify_portable_json(bytes: &amp;[u8], trusted_batch: Option&lt;&amp;AuthorizedBatch&gt;, receipt_keys: &amp;[ReceiptKeyEntry]) -&gt; Result&lt;PortableVerified, PortableReceiptError&gt;</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.11.rs-portable-verify-portable-json</code> | notes | fn layerx_portable::receipt::verify_portable_json; re-exported at services/interop/crates/layerx-portable/src/lib.rs:21; spec/layerx-merge/spec.kvx task 6.5.19 adds rawTransaction at receipted and blockHeader, receiptInclusion, commit, validatorSet at committed (layerx_committed_proof); HEAD PortableReceiptV2 has only format, verificationLevel, signedReceipt, receiptDigest, receiptKey, inclusionHeight |

Contract context <code>contract_context.11.portable-receipt</code>:

| Field | Frozen rule |
|---|---|
| interface | layerx-portable exports and verifies v2 portable receipt proofs from pax_sequencerReceipt answers. |
| storage | none |
| errors | PortableReceiptError. |
| events | none |
| gas | none |
| activation | Format layerx-receipt-proof-v2. |
| invariants | A proof verifies only under a pinned key whose height window covers the receipt. |
| pause | none |
