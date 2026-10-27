use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::export::FactRef;
use layerx_proof::export_codec::{CompleteOfflineArtifact, EncodedOfflineArtifact, FactSelector};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::{build_proof, encode_proof};
use layerx_sdk::production::{verify_receipt, ReceiptKeys, ReceiptVerificationFailure};
use layerx_sdk::ReceiptFailureCode;
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::handover::sequencer_id;
use layerx_wire::hash::{batch_header_digest, receipt_digest};
use layerx_wire::limits::PROTOCOL_VERSION;

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("sdk offline export: {error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn id(value: u8) -> [u8; 32] {
    let mut out = [value; 32];
    out[0] = 0xa0 | (value & 0x0f);
    out
}

fn lp(out: &mut Vec<u8>, bytes: &[u8]) {
    out.extend_from_slice(&must(u32::try_from(bytes.len())).to_be_bytes());
    out.extend_from_slice(bytes);
}

fn receipt(activity: [u8; 32], signer: &SigningKey) -> (Vec<u8>, [u8; 32]) {
    let encode = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, PROTOCOL_VERSION),
            Ok(())
        );
        assert_eq!(encoder.u16(PROTOCOL_VERSION), Ok(()));
        assert_eq!(encoder.bytes(&activity, 32), Ok(()));
        assert_eq!(encoder.u64(9), Ok(()));
        assert_eq!(encoder.bytes(&[2; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[3; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
        assert_eq!(encoder.i32(0), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&batch_id(), 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&[5; 32], 32), Ok(()));
        assert_eq!(encoder.u128(25), Ok(()));
        assert_eq!(encoder.bytes(&[6; 32], 32), Ok(()));
        assert_eq!(encoder.u128(100), Ok(()));
        assert_eq!(encoder.u128(75), Ok(()));
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[7; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(encoder.u128(35), Ok(()));
        assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[10; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[11; 32], 32), Ok(()));
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(value) = signature {
            assert_eq!(encoder.bytes(&value, 64), Ok(()));
        }
        encoder.finish()
    };
    let digest = must(receipt_digest(&encode(None)));
    (encode(Some(signer.sign(&digest).to_bytes())), digest)
}

fn batch_id() -> [u8; 32] {
    let mut id = [0; 32];
    id[24..].copy_from_slice(&1_u64.to_be_bytes());
    id
}

fn decode_export(
    export: &EncodedOfflineArtifact,
) -> Result<CompleteOfflineArtifact, layerx_proof::export_codec::ArtifactDecodeError> {
    CompleteOfflineArtifact::decode(
        &export.facts,
        &export.receipts,
        &export.proofs,
        &export.certificates,
        &export.headers,
    )
}

struct Buckets {
    facts: Vec<String>,
    receipts: Vec<Vec<u8>>,
    proofs: Vec<Vec<u8>>,
    headers: Vec<Vec<u8>>,
}

fn header(receipt_root: [u8; 32], signer: &SigningKey) -> Vec<u8> {
    let mut encoder = Encoder::new(354);
    must(encoder.structure_header_version(0x1701, PROTOCOL_VERSION));
    must(encoder.u8(15));
    for field in 1..=15 {
        must(encoder.tag(field, 15));
        match field {
            1 => must(encoder.u16(PROTOCOL_VERSION)),
            2 => must(encoder.u32(42)),
            3 | 4 => must(encoder.u64(1)),
            5 | 6 => must(encoder.u64(9)),
            7 => must(encoder.bytes(&[2; 32], 32)),
            8 => must(encoder.bytes(&[3; 32], 32)),
            10 => must(encoder.bytes(&receipt_root, 32)),
            14 => must(encoder.u64(1_000)),
            15 => must(encoder.bytes(&must(sequencer_id(&signer.verifying_key().to_bytes())), 32)),
            _ => must(encoder.bytes(&[1; 32], 32)),
        }
    }
    encoder.finish()
}

fn buckets(signer: &SigningKey, activity: [u8; 32]) -> Buckets {
    let sequencer_key = signer.verifying_key().to_bytes();
    let (receipt_bytes, receipt_hash) = receipt(activity, signer);
    let reference = format!("receipt:{}", hex(&activity));
    let (proof, root) = must(build_proof(&[receipt_bytes.as_slice()], 0));
    let header = header(root, signer);
    let mut header_record = b"LXHD".to_vec();
    header_record.push(1);
    lp(&mut header_record, &header);
    header_record.extend_from_slice(&signer.sign(&must(batch_header_digest(&header))).to_bytes());
    header_record.extend_from_slice(&must(sequencer_id(&sequencer_key)));
    header_record.extend_from_slice(&sequencer_key);
    header_record.extend_from_slice(&1_u64.to_be_bytes());
    header_record.extend_from_slice(&13_u64.to_be_bytes());
    let mut receipt_record = b"LXRF".to_vec();
    receipt_record.push(1);
    lp(&mut receipt_record, reference.as_bytes());
    lp(&mut receipt_record, &receipt_bytes);
    for value in [batch_id(), [5; 32], [2; 32], [3; 32]] {
        receipt_record.extend_from_slice(&value);
    }
    receipt_record.extend_from_slice(&signer.verifying_key().to_bytes());
    receipt_record.extend_from_slice(&receipt_hash);
    let mut inclusion = b"LXIP".to_vec();
    inclusion.push(1);
    inclusion.push(2);
    lp(&mut inclusion, reference.as_bytes());
    lp(&mut inclusion, &receipt_bytes);
    lp(&mut inclusion, &encode_proof(&proof));
    inclusion.extend_from_slice(&must(batch_header_digest(&header)));
    Buckets {
        facts: vec![reference],
        receipts: vec![receipt_record],
        proofs: vec![inclusion],
        headers: vec![header_record],
    }
}

fn api(buckets: &Buckets) -> EncodedOfflineArtifact {
    EncodedOfflineArtifact {
        facts: buckets.facts.clone(),
        receipts: buckets.receipts.clone(),
        proofs: buckets.proofs.clone(),
        certificates: Vec::new(),
        headers: buckets.headers.clone(),
    }
}

#[test]
fn api_export_decodes_through_the_one_shared_codec() {
    let key = SigningKey::from_bytes(&[9; 32]);
    let activity = id(1);
    let parts = buckets(&key, activity);
    let no_certificates: [Vec<u8>; 0] = [];
    let Ok(decoded) = decode_export(&api(&parts)) else {
        panic!("canonical api export refused");
    };
    let direct = must(CompleteOfflineArtifact::decode(
        &parts.facts,
        &parts.receipts,
        &parts.proofs,
        &no_certificates,
        &parts.headers,
    ));
    assert_eq!(
        decoded.facts,
        vec![FactSelector::Receipt {
            activity_id: activity
        }]
    );
    assert_eq!(decoded.facts, direct.facts);
    let encoded = must(decoded.encode());
    assert_eq!(encoded.facts, parts.facts);
    assert_eq!(encoded.receipts, parts.receipts);
    assert_eq!(encoded.proofs, parts.proofs);
    assert_eq!(encoded.headers, parts.headers);
    assert!(encoded.certificates.is_empty());
}

#[test]
fn api_fact_refs_are_validated_at_the_export_boundary() {
    let key = SigningKey::from_bytes(&[9; 32]);
    let activity = id(1);
    let canonical = buckets(&key, activity);
    let good = hex(&activity);
    for fact in [
        format!("receipt:{}", good.to_uppercase()),
        format!("receipt:0x{}", &good[2..]),
        format!("receipt:{good} "),
        format!("receipt:{}", hex(&[0; 32])),
        format!("ledger:{good}"),
        format!("checkpoint:{good}:01"),
        format!("checkpoint:{good}:18446744073709551616"),
    ] {
        let mut parts = buckets(&key, activity);
        // The compatibility newtype still accepts any non-empty text; the SDK must not.
        assert!(FactRef::new(fact.clone()).is_ok());
        parts.facts = vec![fact.clone()];
        assert!(decode_export(&api(&parts)).is_err(), "accepted {fact:?}");
    }
    let mut repeated = buckets(&key, activity);
    repeated.facts = vec![canonical.facts[0].clone(), canonical.facts[0].clone()];
    assert!(decode_export(&api(&repeated)).is_err());
    let mut seventeen = buckets(&key, activity);
    seventeen.facts = (1..=17)
        .map(|value| format!("receipt:{}", hex(&id(value))))
        .collect();
    assert!(decode_export(&api(&seventeen)).is_err());
}

#[test]
fn api_buckets_refuse_trailing_truncated_or_misplaced_records() {
    let key = SigningKey::from_bytes(&[9; 32]);
    let activity = id(1);
    let mut trailing = buckets(&key, activity);
    trailing.headers[0].push(0);
    assert!(decode_export(&api(&trailing)).is_err());
    let mut truncated = buckets(&key, activity);
    truncated.receipts[0].pop();
    assert!(decode_export(&api(&truncated)).is_err());
    let mut version = buckets(&key, activity);
    version.proofs[0][4] = 2;
    assert!(decode_export(&api(&version)).is_err());
    let mut misplaced = buckets(&key, activity);
    std::mem::swap(&mut misplaced.receipts, &mut misplaced.headers);
    assert!(decode_export(&api(&misplaced)).is_err());
    let mut certificate = buckets(&key, activity);
    let header = certificate.headers[0].clone();
    let mut export = api(&certificate);
    export.certificates = vec![header];
    certificate.headers.clear();
    assert!(decode_export(&export).is_err());
}

#[test]
fn decoded_export_never_claims_an_unverified_rung_or_a_rogue_sequencer() {
    let key = SigningKey::from_bytes(&[9; 32]);
    let activity = id(1);
    let decoded = must(decode_export(&api(&buckets(&key, activity))));
    let keys = [SequencerAuthorization::new(
        [11; 32],
        key.verifying_key().to_bytes(),
        1,
        1,
    )];
    let record = &decoded.receipts[0];
    let verified = must(verify_receipt(
        &record.canonical_receipt,
        ReceiptKeys::HeightRanges(&keys),
        record.authorised_batch.asset(),
        record.authorised_batch.previous_state_root(),
        record.authorised_batch.resulting_state_root(),
    ));
    assert_eq!(verified.rung(), Rung::Receipted);
    assert!(verified.rung() < Rung::Committed);
    assert_eq!(
        verified.evidence().receipt_digest(),
        Some(record.expected_receipt_digest)
    );

    let layerx_proof::export_codec::ProofRecord::Inclusion(inclusion) = &decoded.proofs[0] else {
        panic!("receipt inclusion missing");
    };
    let header = &decoded.headers[0];
    let authorization = SequencerAuthorization::new(
        must(sequencer_id(&key.verifying_key().to_bytes())),
        key.verifying_key().to_bytes(),
        1,
        1,
    );
    assert_eq!(inclusion.canonical_leaf, record.canonical_receipt);
    assert_eq!(inclusion.header_digest, must(header.digest()));
    let included = must(layerx_proof::inclusion::verify_receipt(
        &inclusion.canonical_leaf,
        &inclusion.proof,
        &header.canonical_header,
        &header.signature,
        &authorization,
    ));
    assert_eq!(included.rung(), Rung::Receipted);
    let mut changed_leaf = inclusion.canonical_leaf.clone();
    changed_leaf[0] ^= 1;
    assert!(layerx_proof::inclusion::verify_receipt(
        &changed_leaf,
        &inclusion.proof,
        &header.canonical_header,
        &header.signature,
        &authorization,
    )
    .is_err());

    let rogue = SigningKey::from_bytes(&[0x77; 32]);
    let forged = must(decode_export(&api(&buckets(&rogue, activity))));
    assert_eq!(
        verify_receipt(
            &forged.receipts[0].canonical_receipt,
            ReceiptKeys::HeightRanges(&keys),
            record.authorised_batch.asset(),
            record.authorised_batch.previous_state_root(),
            record.authorised_batch.resulting_state_root(),
        ),
        Err(ReceiptVerificationFailure {
            check: ReceiptFailureCode::SequencerSignature
        })
    );
}
