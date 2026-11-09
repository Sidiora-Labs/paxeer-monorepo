use layerx_proof::export::{
    verify, DerivedAggregate, ExportVerificationError, InclusionFact, InclusionKind, OfflineExport,
    ReceiptFact,
};
use layerx_proof::export_codec::{FactSelector, ReceiptRecord};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::build_proof;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, decode_batch_header, encode_unsigned};

const RECEIPT: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt");
const ACTIVITY: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/activity.lxa");
const HEADER: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header");
const SIGNATURE: [u8; 64] =
    *include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header.signature");
const KEY: [u8; 32] =
    *include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/sequencer.public");

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("native offline export: {error:?}"))
}

#[test]
fn offline_export_verifies_receipts_and_inclusions_without_a_settlement_domain() {
    let receipt = must(decode(RECEIPT));
    let Some(protocol) = receipt.protocol() else {
        panic!("native withdrawal receipt");
    };
    let digest = must(receipt_digest(&must(encode_unsigned(&receipt))));
    let record = ReceiptRecord {
        reference: FactSelector::Receipt {
            activity_id: protocol.activity_id(),
        },
        canonical_receipt: RECEIPT.to_vec(),
        authorised_batch: AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            KEY,
        ),
        expected_receipt_digest: digest,
    };
    let encoded = must(record.encode());
    let restored = must(ReceiptRecord::decode(&encoded));
    assert_eq!(restored, record);
    let header = must(decode_batch_header(HEADER));
    let (proof, root) = must(build_proof(&[ACTIVITY], 0));
    assert_eq!(root, header.activity_merkle_root());
    let export = OfflineExport {
        receipts: vec![ReceiptFact {
            statement: restored.reference.canonical_text(),
            canonical_receipt_bytes: restored.canonical_receipt,
            authorised_batch: restored.authorised_batch,
            expected_receipt_digest: restored.expected_receipt_digest,
        }],
        inclusions: vec![InclusionFact {
            statement: "The native withdrawal activity is included in its signed batch".into(),
            kind: InclusionKind::Activity,
            canonical_leaf_bytes: ACTIVITY.to_vec(),
            proof,
            named_root: root,
            canonical_header_bytes: HEADER.to_vec(),
            header_signature: SIGNATURE,
            sequencer_authorization: SequencerAuthorization::new(
                header.sequencer_id(),
                KEY,
                header.batch_number(),
                header.batch_number(),
            ),
        }],
        derived_aggregates: vec![DerivedAggregate {
            label: "withdrawals".into(),
            rendered_value: "1".into(),
            contributing_receipt_digests: vec![digest],
        }],
    };
    let report = must(verify(&export));
    assert_eq!(report.verified_receipts, 1);
    assert_eq!(report.verified_inclusions, 1);
    assert_eq!(report.receipt_digests, vec![digest]);
    assert_eq!(
        report.achieved_levels,
        vec![
            VerificationLevel::SEQUENCER_SIGNED,
            VerificationLevel::BATCH_INCLUDED,
        ]
    );
    assert!(!report.derived_aggregates_are_protocol_facts);

    let mut altered = export.clone();
    altered.receipts[0].expected_receipt_digest[0] ^= 1;
    assert_eq!(
        verify(&altered),
        Err(ExportVerificationError::ReceiptDigest { index: 0 })
    );
    altered = export.clone();
    let last = altered.receipts[0].canonical_receipt_bytes.len() - 1;
    altered.receipts[0].canonical_receipt_bytes[last] ^= 1;
    assert!(matches!(
        verify(&altered),
        Err(ExportVerificationError::Receipt { index: 0, .. })
    ));
    altered = export.clone();
    altered.inclusions[0].canonical_leaf_bytes[0] ^= 1;
    assert!(matches!(
        verify(&altered),
        Err(ExportVerificationError::Inclusion { index: 0, .. })
    ));
    altered = export.clone();
    altered.inclusions[0].named_root[0] ^= 1;
    assert_eq!(
        verify(&altered),
        Err(ExportVerificationError::InclusionRoot { index: 0 })
    );
    altered = export;
    altered.derived_aggregates[0].contributing_receipt_digests = vec![[0x77; 32]];
    assert_eq!(
        verify(&altered),
        Err(ExportVerificationError::UnknownAggregateContributor {
            aggregate: 0,
            digest: [0x77; 32],
        })
    );
}
