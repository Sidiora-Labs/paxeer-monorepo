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
use sha2::{Digest as _, Sha256};

#[derive(Clone)]
struct Fields {
    activity_id: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    fee_charged: u128,
    effect_bodies: Vec<Vec<u8>>,
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
        fee_charged: 1,
        effect_bodies: vec![],
        from: [6; 32],
        from_before: 100,
        from_after: 75,
        to: [7; 32],
        to_before: 10,
        to_after: 35,
    }
}

#[derive(Clone, Copy, Debug)]
enum ProgramsReceiptKind {
    Call,
    Lifecycle,
    PreexecutionRejection,
}

#[derive(Clone, Copy, Debug)]
struct ProgramsVector {
    protocol: u16,
    module: u32,
    guest: u16,
    kind: ProgramsReceiptKind,
}

fn encode_programs_vector(
    vector: ProgramsVector,
    signature: Option<[u8; 64]>,
) -> Result<Vec<u8>, layerx_wire::WireError> {
    use layerx_types::result::KnownResult;

    let mut encoder = Encoder::new(4096);
    let result = if matches!(vector.kind, ProgramsReceiptKind::PreexecutionRejection) {
        KnownResult::VersionUnsupported.raw()
    } else {
        0
    };
    encoder.structure_header_version(0x5201, vector.protocol)?;
    encoder.u16(vector.protocol)?;
    encoder.bytes(&[1; 32], 32)?; // activity
    encoder.u64(9)?;
    encoder.bytes(&[2; 32], 32)?; // previous state root
    encoder.bytes(&[3; 32], 32)?; // resulting state root
    encoder.bytes(&[8; 32], 32)?; // activity root
    encoder.i32(result)?;
    encoder.sequence_length(0, 512)?;
    encoder.u128(0)?; // fee
    encoder.bytes(&[4; 32], 32)?; // batch
    encoder.u16(9)?; // Programs module
    encoder.u32(vector.module)?;
    encoder.u32(1)?; // parameter version
    encoder.u8(if matches!(vector.kind, ProgramsReceiptKind::Lifecycle) {
        0
    } else {
        3
    })?;
    encoder.bytes(&[0; 32], 32)?; // asset
    encoder.u128(0)?; // amount
    encoder.bytes(&[0; 32], 32)?; // from
    encoder.u128(0)?;
    encoder.u128(0)?;
    encoder.u64(0)?;
    encoder.bytes(&[0; 32], 32)?; // to
    encoder.u128(0)?;
    encoder.u128(0)?;
    encoder.bytes(&[0; 32], 32)?; // transfer root
    encoder.bytes(&[0; 32], 32)?; // authorization
    encoder.bytes(&[0; 32], 32)?; // context
    encoder.u64(1_000)?;
    if matches!(vector.kind, ProgramsReceiptKind::Call) {
        encoder.u32(0x5052_4733)?; // canonical outcome version three
        encoder.u8(1)?; // successful terminal
        encoder.i32(0)?;
        encoder.u16(layerx_programs_runtime::RUNTIME_VERSION)?;
        encoder.u16(vector.guest)?;
        encoder.u32(1)?; // fee schedule
        encoder.u32(1)?; // metering schedule
        encoder.u64(1)?; // CPU fuel
        encoder.u64(0)?; // memory
        encoder.u64(0)?; // storage read
        encoder.u64(0)?; // storage write
        encoder.u32(0)?; // output values
        encoder.u64(0)?; // output bytes
        encoder.u128(0)?; // occupancy byte batches
        encoder.u128(0)?; // occupancy fee
        for _ in 0..7 {
            encoder.u64(0)?; // schedule prices
        }
        let occupancy = vector.protocol != LEGACY_PROTOCOL_VERSION;
        encoder.bytes(&[if occupancy { 12 } else { 0 }; 32], 32)?;
        encoder.bytes(&[if occupancy { 13 } else { 0 }; 32], 32)?;
        encoder.bytes(&[0; 32], 32)?; // occupancy transfer root
        encoder.u128(0)?; // fee units
        encoder.bytes(&[14; 32], 32)?; // call graph
        encoder.bytes(&[15; 32], 32)?; // terminal payload
        encoder.bytes(&[0; 32], 32)?; // transfer root
    }
    encoder.u8(u8::from(signature.is_some()))?;
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64)?;
    }
    Ok(encoder.finish())
}

fn sign_programs_vector(vector: ProgramsVector, signing_key: &SigningKey) -> Vec<u8> {
    use layerx_wire::receipt::{decode, encode, encode_unsigned};

    let unsigned = encode_programs_vector(vector, None)
        .unwrap_or_else(|error| panic!("encode {vector:?}: {error:?}"));
    let receipt = decode(&unsigned).unwrap_or_else(|error| panic!("decode {vector:?}: {error:?}"));
    let canonical = encode_unsigned(&receipt)
        .unwrap_or_else(|error| panic!("canonical unsigned receipt: {error:?}"));
    assert_eq!(canonical, unsigned);
    let digest =
        receipt_digest(&canonical).unwrap_or_else(|error| panic!("hash {vector:?}: {error:?}"));
    let signed = encode_programs_vector(vector, Some(signing_key.sign(&digest).to_bytes()))
        .unwrap_or_else(|error| panic!("sign {vector:?}: {error:?}"));
    let decoded =
        decode(&signed).unwrap_or_else(|error| panic!("decode signed {vector:?}: {error:?}"));
    assert_eq!(
        encode(&decoded).unwrap_or_else(|error| panic!("canonical receipt: {error:?}")),
        signed
    );
    signed
}

fn verify_programs_vector(
    kind: ProgramsReceiptKind,
    bytes: &[u8],
    batch: &AuthorizedBatch,
) -> Result<layerx_proof::receipt::VerifiedReceipt, VerificationFailure> {
    use layerx_proof::receipt::{
        verify_program_outcome, verify_program_preexecution_rejection, verify_program_state,
    };
    match kind {
        ProgramsReceiptKind::Call => verify_program_outcome(bytes, batch),
        ProgramsReceiptKind::Lifecycle => verify_program_state(bytes, batch),
        ProgramsReceiptKind::PreexecutionRejection => {
            verify_program_preexecution_rejection(bytes, batch)
        }
    }
}

#[test]
fn programs_module_versions_verify_signed_receipts() {
    use layerx_proof::receipt::verify_historical_program_outcome_v1;
    use layerx_types::{guest_abi, programs_module_abi};
    use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION;

    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let batch = AuthorizedBatch::new(
        [4; 32],
        [0; 32],
        [2; 32],
        [3; 32],
        signing_key.verifying_key().to_bytes(),
    );
    let kinds = [
        ProgramsReceiptKind::Call,
        ProgramsReceiptKind::Lifecycle,
        ProgramsReceiptKind::PreexecutionRejection,
    ];
    for module in [
        programs_module_abi::SANDBOX_DESTROY,
        programs_module_abi::GUEST_ABI_V5,
    ] {
        for kind in kinds {
            for guest in 1..=guest_abi::MAX_VERSION {
                let vector = ProgramsVector {
                    protocol: STATE_COMMITMENT_PROTOCOL_VERSION,
                    module,
                    guest,
                    kind,
                };
                let bytes = sign_programs_vector(vector, &signing_key);
                let verified = verify_programs_vector(kind, &bytes, &batch)
                    .unwrap_or_else(|error| panic!("valid {vector:?}: {error:?}"));
                assert_eq!(verified.canonical_bytes(), bytes);
                let protocol = verified
                    .receipt()
                    .protocol()
                    .expect("Programs protocol receipt");
                assert_eq!(protocol.module_version(), module);
                match kind {
                    ProgramsReceiptKind::Call => assert_eq!(
                        protocol
                            .program_outcome()
                            .expect("call outcome")
                            .abi_version(),
                        guest
                    ),
                    ProgramsReceiptKind::Lifecycle => assert_eq!(protocol.operation(), 0),
                    ProgramsReceiptKind::PreexecutionRejection => {
                        assert!(protocol.program_outcome().is_none());
                        assert_eq!(
                            protocol.result_code(),
                            layerx_types::result::KnownResult::VersionUnsupported.raw()
                        );
                    }
                }
                if !matches!(kind, ProgramsReceiptKind::PreexecutionRejection) {
                    assert_eq!(
                        verify_outcome(&bytes, &batch)
                            .expect("dispatch Programs receipt")
                            .canonical_bytes(),
                        bytes
                    );
                }
                let mut tampered = bytes.clone();
                let last = tampered.len() - 1;
                tampered[last] ^= 1;
                assert_eq!(
                    verify_programs_vector(kind, &tampered, &batch),
                    Err(VerificationFailure {
                        check: ReceiptCheck::SequencerSignature
                    }),
                    "{vector:?}"
                );
                for (previous, resulting, check) in [
                    ([99; 32], [3; 32], ReceiptCheck::PreviousStateRoot),
                    ([2; 32], [99; 32], ReceiptCheck::ResultingStateRoot),
                ] {
                    let wrong = AuthorizedBatch::new(
                        batch.batch_id(),
                        batch.asset(),
                        previous,
                        resulting,
                        batch.sequencer_public_key(),
                    );
                    assert_eq!(
                        verify_programs_vector(kind, &bytes, &wrong),
                        Err(VerificationFailure { check }),
                        "{vector:?}"
                    );
                }
                let wrong_batch = AuthorizedBatch::new(
                    [99; 32],
                    batch.asset(),
                    batch.previous_state_root(),
                    batch.resulting_state_root(),
                    batch.sequencer_public_key(),
                );
                assert_eq!(
                    verify_programs_vector(kind, &bytes, &wrong_batch),
                    Err(VerificationFailure {
                        check: ReceiptCheck::BatchId
                    })
                );
            }
        }
        for guest in [guest_abi::MAX_VERSION + 1, u16::MAX] {
            let vector = ProgramsVector {
                protocol: STATE_COMMITMENT_PROTOCOL_VERSION,
                module,
                guest,
                kind: ProgramsReceiptKind::Call,
            };
            assert_eq!(
                verify_programs_vector(
                    vector.kind,
                    &sign_programs_vector(vector, &signing_key),
                    &batch
                ),
                Err(VerificationFailure {
                    check: ReceiptCheck::ProtocolVersion
                })
            );
        }
    }
    for module in [0, 1, 2, 3, 6, u32::MAX] {
        for kind in kinds {
            let vector = ProgramsVector {
                protocol: STATE_COMMITMENT_PROTOCOL_VERSION,
                module,
                guest: 2,
                kind,
            };
            assert_eq!(
                verify_programs_vector(kind, &sign_programs_vector(vector, &signing_key), &batch),
                Err(VerificationFailure {
                    check: ReceiptCheck::Module
                }),
                "{vector:?}"
            );
        }
    }
    for module in [0, 1, 2, 3, 4, 5, 6, u32::MAX] {
        for kind in kinds {
            let vector = ProgramsVector {
                protocol: PROTOCOL_VERSION,
                module,
                guest: 2,
                kind,
            };
            let bytes = sign_programs_vector(vector, &signing_key);
            let expected = match kind {
                ProgramsReceiptKind::Call if matches!(module, 1..=3) => None,
                ProgramsReceiptKind::Lifecycle if matches!(module, 2 | 3) => None,
                ProgramsReceiptKind::PreexecutionRejection => Some(ReceiptCheck::ProtocolVersion),
                _ => Some(ReceiptCheck::Module),
            };
            assert_eq!(
                verify_programs_vector(kind, &bytes, &batch)
                    .map(|_| ())
                    .map_err(|error| error.check),
                expected.map_or(Ok(()), Err),
                "{vector:?}"
            );
        }
        let vector = ProgramsVector {
            protocol: LEGACY_PROTOCOL_VERSION,
            module,
            guest: 2,
            kind: ProgramsReceiptKind::Call,
        };
        let bytes = sign_programs_vector(vector, &signing_key);
        assert_eq!(
            verify_historical_program_outcome_v1(&bytes, &batch)
                .map(|_| ())
                .map_err(|error| error.check),
            if module == programs_module_abi::INITIAL {
                Ok(())
            } else {
                Err(ReceiptCheck::Module)
            },
            "historical {vector:?}"
        );
        assert_eq!(
            verify_programs_vector(vector.kind, &bytes, &batch),
            Err(VerificationFailure {
                check: ReceiptCheck::ProtocolVersion
            })
        );
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
    assert_eq!(
        encoder.sequence_length(fields.effect_bodies.len(), 512),
        Ok(())
    );
    for (ordinal, body) in fields.effect_bodies.iter().enumerate() {
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(
            encoder.u16(
                u16::try_from(ordinal).unwrap_or_else(|error| panic!("effect ordinal: {error}"))
            ),
            Ok(())
        );
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.tag(1, 3), Ok(()));
        assert_eq!(encoder.u8(0), Ok(()));
        assert_eq!(encoder.bytes(&[0; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(body, 256), Ok(()));
    }
    assert_eq!(encoder.u128(fields.fee_charged), Ok(()));
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

fn execution_effect_body(denomination: &[u8]) -> Vec<u8> {
    let mut body = vec![0; 108];
    body.push(
        u8::try_from(denomination.len())
            .unwrap_or_else(|error| panic!("fee denomination length: {error}")),
    );
    body.extend_from_slice(denomination);
    body
}

#[test]
fn debit_identity_accounts_for_the_fee_in_the_receipt_asset() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let mut fields = fields();
    fields.asset = Sha256::digest(b"uhpx").into();
    fields.fee_charged = 7;
    fields.effect_bodies = vec![execution_effect_body(b"uhpx")];
    for amount in [25, 0] {
        fields.amount = amount;
        fields.to_after = fields.to_before + amount;
        fields.from_after = fields.from_before - amount - fields.fee_charged;
        let bytes = sign(&fields, &signing_key);
        let verified = verify(&bytes, &authorised(&fields, &signing_key))
            .unwrap_or_else(|error| panic!("same-asset fee rejected: {error:?}"));
        assert_eq!(verified.canonical_bytes(), bytes);
        fields.from_after = fields.from_before - amount;
        assert_eq!(
            verify(
                &sign(&fields, &signing_key),
                &authorised(&fields, &signing_key)
            ),
            Err(VerificationFailure {
                check: ReceiptCheck::DebitBalance
            })
        );
    }
    for (before, amount, fee) in [(24, 25, 7), (31, 25, 7), (u128::MAX, 1, u128::MAX)] {
        fields.from_before = before;
        fields.amount = amount;
        fields.fee_charged = fee;
        fields.from_after = 0;
        fields.to_after = fields.to_before + amount;
        assert_eq!(
            verify(
                &sign(&fields, &signing_key),
                &authorised(&fields, &signing_key)
            ),
            Err(VerificationFailure {
                check: ReceiptCheck::DebitBalance
            })
        );
    }
}

#[test]
fn debit_identity_ignores_a_fee_in_another_asset() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let mut fields = fields();
    fields.asset = Sha256::digest(b"uhpx").into();
    fields.fee_charged = 7;
    fields.effect_bodies = vec![execution_effect_body(b"other")];
    let bytes = sign(&fields, &signing_key);
    let verified = verify(&bytes, &authorised(&fields, &signing_key))
        .unwrap_or_else(|error| panic!("other-asset fee rejected: {error:?}"));
    assert_eq!(verified.canonical_bytes(), bytes);
    fields.from_after = fields.from_before - fields.amount - fields.fee_charged;
    assert_eq!(
        verify(
            &sign(&fields, &signing_key),
            &authorised(&fields, &signing_key)
        ),
        Err(VerificationFailure {
            check: ReceiptCheck::DebitBalance
        })
    );
}

#[test]
fn debit_identity_keeps_fee_free_identity_without_a_complete_first_denomination() {
    let signing_key = SigningKey::from_bytes(&[3; 32]);
    let mut fields = fields();
    fields.asset = Sha256::digest(b"uhpx").into();
    fields.fee_charged = 7;
    let mut truncated = execution_effect_body(b"uhpx");
    truncated.pop();
    let mut overrun = execution_effect_body(b"uhpx");
    overrun[108] = u8::MAX;
    for bodies in [
        vec![],
        vec![vec![]],
        vec![vec![0; 108]],
        vec![truncated],
        vec![overrun],
        vec![
            execution_effect_body(b"other"),
            execution_effect_body(b"uhpx"),
        ],
    ] {
        fields.effect_bodies = bodies;
        fields.from_after = fields.from_before - fields.amount;
        let bytes = sign(&fields, &signing_key);
        assert!(verify(&bytes, &authorised(&fields, &signing_key)).is_ok());
        fields.from_after -= fields.fee_charged;
        assert_eq!(
            verify(
                &sign(&fields, &signing_key),
                &authorised(&fields, &signing_key)
            ),
            Err(VerificationFailure {
                check: ReceiptCheck::DebitBalance
            })
        );
    }
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
