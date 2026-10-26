use ed25519_dalek::{Signer as _, SigningKey};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::level::achieved_rung;
use layerx_proof::receipt::{
    canonical_protocol_facts, verify, verify_outcome, AuthorizedBatch, ReceiptCheck, ReceiptKeys,
    VerificationFailure,
};
use layerx_types::{rung::Rung, verify::VerificationLevel};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{LEGACY_PROTOCOL_VERSION, PROTOCOL_VERSION};

#[derive(Clone)]
struct Fields {
    activity_id: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    from: [u8; 32],
    from_before: u128,
    from_after: u128,
    to: [u8; 32],
    to_before: u128,
    to_after: u128,
}

fn fields() -> Fields {
    Fields {
        activity_id: [1; 32],
        previous_state_root: [2; 32],
        resulting_state_root: [3; 32],
        batch_id: [4; 32],
        asset: [5; 32],
        amount: 25,
        from: [6; 32],
        from_before: 100,
        from_after: 75,
        to: [7; 32],
        to_before: 10,
        to_after: 35,
    }
}

fn encode_fields_version(
    fields: &Fields,
    signature: Option<[u8; 64]>,
    protocol_version: u16,
) -> Vec<u8> {
    let mut encoder = Encoder::new(4096);
    assert_eq!(
        encoder.structure_header_version(0x5201, protocol_version),
        Ok(())
    );
    assert_eq!(encoder.u16(protocol_version), Ok(()));
    assert_eq!(encoder.bytes(&fields.activity_id, 32), Ok(()));
    assert_eq!(encoder.u64(9), Ok(()));
    assert_eq!(encoder.bytes(&fields.previous_state_root, 32), Ok(()));
    assert_eq!(encoder.bytes(&fields.resulting_state_root, 32), Ok(()));
    assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
    assert_eq!(encoder.i32(0), Ok(()));
    assert_eq!(encoder.sequence_length(0, 512), Ok(()));
    assert_eq!(encoder.u128(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.batch_id, 32), Ok(()));
    assert_eq!(encoder.u16(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u32(1), Ok(()));
    assert_eq!(encoder.u8(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.asset, 32), Ok(()));
    assert_eq!(encoder.u128(fields.amount), Ok(()));
    assert_eq!(encoder.bytes(&fields.from, 32), Ok(()));
    assert_eq!(encoder.u128(fields.from_before), Ok(()));
    assert_eq!(encoder.u128(fields.from_after), Ok(()));
    assert_eq!(encoder.u64(1), Ok(()));
    assert_eq!(encoder.bytes(&fields.to, 32), Ok(()));
    assert_eq!(encoder.u128(fields.to_before), Ok(()));
    assert_eq!(encoder.u128(fields.to_after), Ok(()));
    assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[10; 32], 32), Ok(()));
    assert_eq!(encoder.bytes(&[11; 32], 32), Ok(()));
    assert_eq!(encoder.u64(1_000), Ok(()));
    assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
    if let Some(value) = signature {
        assert_eq!(encoder.bytes(&value, 64), Ok(()));
    }
    encoder.finish()
}

fn encode_fields(fields: &Fields, signature: Option<[u8; 64]>) -> Vec<u8> {
    encode_fields_version(fields, signature, PROTOCOL_VERSION)
}

fn sign(fields: &Fields, signing_key: &SigningKey) -> Vec<u8> {
    let unsigned = encode_fields(fields, None);
    let digest = receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("receipt hashing failed: {error:?}"));
    encode_fields(fields, Some(signing_key.sign(&digest).to_bytes()))
}

fn sign_version(fields: &Fields, signing_key: &SigningKey, protocol_version: u16) -> Vec<u8> {
    let unsigned = encode_fields_version(fields, None, protocol_version);
    let digest = receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("receipt hashing failed: {error:?}"));
    encode_fields_version(
        fields,
        Some(signing_key.sign(&digest).to_bytes()),
        protocol_version,
    )
}

fn authorised(fields: &Fields, signing_key: &SigningKey) -> AuthorizedBatch {
    AuthorizedBatch::new(
        fields.batch_id,
        fields.asset,
        fields.previous_state_root,
        fields.resulting_state_root,
        signing_key.verifying_key().to_bytes(),
    )
}

#[test]
fn verifies_from_bytes_with_no_ambient_capabilities() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields();
    let bytes = sign(&fields, &signing_key);
    let verified = verify(&bytes, &authorised(&fields, &signing_key))
        .unwrap_or_else(|error| panic!("valid core receipt rejected: {error:?}"));
    assert_eq!(verified.canonical_bytes(), bytes);
    assert_eq!(verified.level(), VerificationLevel::SEQUENCER_SIGNED);
    let facts = canonical_protocol_facts(verified.canonical_bytes())
        .unwrap_or_else(|error| panic!("verified receipt facts rejected: {error:?}"));
    assert_eq!(facts.result_code(), 0);
    assert_eq!(facts.asset(), fields.asset);
    assert_eq!(facts.amount(), fields.amount);
    assert_eq!(facts.fee_charged(), 1);
}

#[test]
fn verifies_current_occupancy_protocol_receipts() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields();
    let bytes = sign_version(&fields, &signing_key, PROTOCOL_VERSION);
    let verified = verify(&bytes, &authorised(&fields, &signing_key))
        .unwrap_or_else(|error| panic!("valid protocol v2 receipt rejected: {error:?}"));
    assert_eq!(verified.canonical_bytes(), bytes);
    assert_eq!(
        verified
            .receipt()
            .protocol()
            .map(layerx_wire::receipt::ProtocolReceipt::protocol_version),
        Some(PROTOCOL_VERSION)
    );
}

#[test]
fn legacy_receipts_decode_but_are_not_accepted_as_beta_evidence() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields();
    let bytes = sign_version(&fields, &signing_key, LEGACY_PROTOCOL_VERSION);
    assert_eq!(
        verify(&bytes, &authorised(&fields, &signing_key)),
        Err(VerificationFailure {
            check: ReceiptCheck::ProtocolVersion,
        })
    );
}

#[test]
fn rejects_altered_values_and_wrong_authority() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let wrong_key = SigningKey::from_bytes(&[4; 32]);
    let original = fields();
    let original_signature = {
        let unsigned = encode_fields(&original, None);
        let digest = receipt_digest(&unsigned)
            .unwrap_or_else(|error| panic!("receipt hashing failed: {error:?}"));
        signing_key.sign(&digest).to_bytes()
    };

    let mut altered_amount = original.clone();
    altered_amount.amount = 24;
    altered_amount.from_after = 76;
    altered_amount.to_after = 34;
    assert_eq!(
        verify(
            &encode_fields(&altered_amount, Some(original_signature)),
            &authorised(&original, &signing_key),
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::SequencerSignature,
        })
    );

    let mut altered_recipient = original.clone();
    altered_recipient.to[0] ^= 1;
    assert_eq!(
        verify(
            &encode_fields(&altered_recipient, Some(original_signature)),
            &authorised(&original, &signing_key),
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::SequencerSignature,
        })
    );

    assert_eq!(
        verify(
            &sign(&original, &wrong_key),
            &authorised(&original, &signing_key)
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::SequencerSignature,
        })
    );
}

#[test]
fn names_balance_asset_and_state_chain_failures() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let original = fields();

    let mut bad_debit = original.clone();
    bad_debit.from_after = 76;
    assert_eq!(
        verify(
            &sign(&bad_debit, &signing_key),
            &authorised(&bad_debit, &signing_key)
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::DebitBalance,
        })
    );

    let mut bad_credit = original.clone();
    bad_credit.to_after = 36;
    assert_eq!(
        verify(
            &sign(&bad_credit, &signing_key),
            &authorised(&bad_credit, &signing_key)
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::CreditBalance,
        })
    );

    let bytes = sign(&original, &signing_key);
    let wrong_asset = AuthorizedBatch::new(
        original.batch_id,
        [99; 32],
        original.previous_state_root,
        original.resulting_state_root,
        signing_key.verifying_key().to_bytes(),
    );
    assert_eq!(
        verify(&bytes, &wrong_asset),
        Err(VerificationFailure {
            check: ReceiptCheck::Asset,
        })
    );
    let wrong_roots = AuthorizedBatch::new(
        original.batch_id,
        original.asset,
        [98; 32],
        original.resulting_state_root,
        signing_key.verifying_key().to_bytes(),
    );
    assert_eq!(
        verify(&bytes, &wrong_roots),
        Err(VerificationFailure {
            check: ReceiptCheck::PreviousStateRoot,
        })
    );
}

#[test]
fn verifies_state_commitment_receipt_and_keeps_root_and_signature_checks() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields();
    let version = layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION;
    let bytes = sign_version(&fields, &signing_key, version);
    let Ok(verified) = verify(&bytes, &authorised(&fields, &signing_key)) else {
        panic!("version three signed receipt rejected");
    };
    assert_eq!(verified.canonical_bytes(), bytes);
    let mut wrong_root = fields.clone();
    wrong_root.resulting_state_root[0] ^= 1;
    assert_eq!(
        verify(&bytes, &authorised(&wrong_root, &signing_key)),
        Err(VerificationFailure {
            check: ReceiptCheck::ResultingStateRoot
        })
    );
    let mut altered = bytes;
    let last = altered.len() - 1;
    altered[last] ^= 1;
    assert_eq!(
        verify(&altered, &authorised(&fields, &signing_key)),
        Err(VerificationFailure {
            check: ReceiptCheck::SequencerSignature
        })
    );
}

#[test]
fn pinned_receipt_keys_authorize_the_batch_and_report_receipted() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let wrong_key = SigningKey::from_bytes(&[4; 32]);
    let public_key = signing_key.verifying_key().to_bytes();
    let pin = SequencerAuthorization::new([11; 32], public_key, 41, 43);
    let keys = [
        SequencerAuthorization::new([12; 32], wrong_key.verifying_key().to_bytes(), 41, 43),
        pin,
    ];
    let mut fields = fields();
    fields.batch_id = [0; 32];
    fields.batch_id[24..].copy_from_slice(&42_u64.to_be_bytes());
    let bytes = sign(&fields, &signing_key);
    let batch = AuthorizedBatch::from_receipt_keys(
        &bytes,
        ReceiptKeys::HeightRanges(&keys),
        fields.asset,
        fields.previous_state_root,
        fields.resulting_state_root,
    )
    .unwrap_or_else(|error| panic!("pinned batch rejected: {error:?}"));
    assert_eq!(batch.batch_id(), fields.batch_id);
    assert_eq!(batch.sequencer_public_key(), public_key);
    assert_eq!(batch.asset(), fields.asset);
    assert_eq!(batch.previous_state_root(), fields.previous_state_root);
    assert_eq!(batch.resulting_state_root(), fields.resulting_state_root);
    for verified in [verify_outcome(&bytes, &batch), verify(&bytes, &batch)] {
        let verified =
            verified.unwrap_or_else(|error| panic!("pinned receipt rejected: {error:?}"));
        assert_eq!(verified.rung(), Rung::Receipted);
        assert_eq!(verified.level(), VerificationLevel::SEQUENCER_SIGNED);
        assert_eq!(verified.evidence().rung(), Rung::Receipted);
        assert_eq!(achieved_rung(verified.evidence()), Rung::Receipted);
        assert_eq!(verified.canonical_bytes(), bytes);
        assert_eq!(
            verified.evidence().receipt_digest(),
            Some(
                receipt_digest(&encode_fields(&fields, None))
                    .unwrap_or_else(|error| panic!("receipt hashing failed: {error:?}"))
            )
        );
    }
    for height in [41_u64, 43] {
        let mut boundary = fields.clone();
        boundary.batch_id[24..].copy_from_slice(&height.to_be_bytes());
        let bytes = sign(&boundary, &signing_key);
        let batch = AuthorizedBatch::from_receipt_keys(
            &bytes,
            &keys,
            boundary.asset,
            boundary.previous_state_root,
            boundary.resulting_state_root,
        )
        .unwrap_or_else(|error| panic!("inclusive boundary rejected: {error:?}"));
        assert_eq!(
            verify_outcome(&bytes, &batch).map(|receipt| receipt.rung()),
            Ok(Rung::Receipted)
        );
    }
    let invalid_pins = [
        vec![],
        vec![SequencerAuthorization::new([12; 32], public_key, 41, 43)],
        vec![SequencerAuthorization::new(
            [11; 32],
            wrong_key.verifying_key().to_bytes(),
            41,
            43,
        )],
        vec![SequencerAuthorization::new([11; 32], public_key, 43, 44)],
        vec![SequencerAuthorization::new([11; 32], public_key, 40, 41)],
        vec![pin, pin],
    ];
    for invalid in invalid_pins {
        assert_eq!(
            AuthorizedBatch::from_receipt_keys(
                &bytes,
                invalid.as_slice(),
                fields.asset,
                fields.previous_state_root,
                fields.resulting_state_root,
            ),
            Err(VerificationFailure {
                check: ReceiptCheck::SequencerSignature
            })
        );
    }
    assert_eq!(
        AuthorizedBatch::from_receipt_keys(
            &bytes,
            ReceiptKeys::HistoricalKernel(public_key),
            fields.asset,
            fields.previous_state_root,
            fields.resulting_state_root,
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::SequencerSignature
        })
    );
    let mut malformed = fields.clone();
    malformed.batch_id[0] = 1;
    assert_eq!(
        AuthorizedBatch::from_receipt_keys(
            &sign(&malformed, &signing_key),
            &keys,
            fields.asset,
            fields.previous_state_root,
            fields.resulting_state_root,
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::BatchId
        })
    );
    for invalid in [sign(&fields, &wrong_key), {
        let mut tampered = bytes.clone();
        let last = tampered.len() - 1;
        tampered[last] ^= 1;
        tampered
    }] {
        assert_eq!(
            AuthorizedBatch::from_receipt_keys(
                &invalid,
                &keys,
                fields.asset,
                fields.previous_state_root,
                fields.resulting_state_root,
            ),
            Err(VerificationFailure {
                check: ReceiptCheck::SequencerSignature
            })
        );
        assert_eq!(
            verify_outcome(&invalid, &batch),
            Err(VerificationFailure {
                check: ReceiptCheck::SequencerSignature
            })
        );
    }
    assert_eq!(
        AuthorizedBatch::from_receipt_keys(
            &encode_fields(&fields, None),
            &keys,
            fields.asset,
            fields.previous_state_root,
            fields.resulting_state_root,
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::MissingSignature
        })
    );
    for (asset, previous, resulting, check) in [
        (
            [99; 32],
            fields.previous_state_root,
            fields.resulting_state_root,
            ReceiptCheck::Asset,
        ),
        (
            fields.asset,
            [99; 32],
            fields.resulting_state_root,
            ReceiptCheck::PreviousStateRoot,
        ),
        (
            fields.asset,
            fields.previous_state_root,
            [99; 32],
            ReceiptCheck::ResultingStateRoot,
        ),
    ] {
        let batch = AuthorizedBatch::from_receipt_keys(&bytes, &keys, asset, previous, resulting)
            .unwrap_or_else(|error| {
                panic!("signature should establish batch authority: {error:?}")
            });
        assert_eq!(
            verify_outcome(&bytes, &batch),
            Err(VerificationFailure { check })
        );
    }
}

#[test]
fn authority_supplied_key_reports_only_the_submitted_rung() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let fields = fields();
    let bytes = sign(&fields, &signing_key);
    let batch = authorised(&fields, &signing_key);
    for verified in [verify_outcome(&bytes, &batch), verify(&bytes, &batch)] {
        let verified = verified.unwrap_or_else(|error| panic!("valid receipt rejected: {error:?}"));
        assert_eq!(verified.rung(), Rung::Submitted);
        assert_eq!(verified.level(), VerificationLevel::SEQUENCER_SIGNED);
        assert_eq!(verified.evidence().rung(), Rung::Submitted);
        assert_eq!(achieved_rung(verified.evidence()), Rung::Submitted);
        assert_eq!(verified.canonical_bytes(), bytes);
    }
}
