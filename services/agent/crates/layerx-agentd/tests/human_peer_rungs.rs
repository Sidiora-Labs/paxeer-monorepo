use layerx_agentd::human::{
    self, decode_request, encode_response, HumanOperationError, HumanProtocolError, HumanRefusal,
    HumanRequest, HumanResponse,
};
use layerx_types::rung::Rung;

fn field(bytes: &mut Vec<u8>, value: &[u8]) {
    bytes.extend_from_slice(
        &u32::try_from(value.len())
            .expect("bounded field")
            .to_be_bytes(),
    );
    bytes.extend_from_slice(value);
}

fn prepare_frame(nonce: u64) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(human::PREPARE);
    frame.extend_from_slice(&17u64.to_be_bytes());
    frame.extend_from_slice(&[0x11; 32]);
    frame.extend_from_slice(&[0x22; 32]);
    frame.extend_from_slice(&0x0001_0001u32.to_be_bytes());
    field(&mut frame, b"did:layerx:alice");
    field(&mut frame, b"owner");
    frame.extend_from_slice(&nonce.to_be_bytes());
    frame.extend_from_slice(&100u64.to_be_bytes());
    frame.extend_from_slice(&110u64.to_be_bytes());
    field(&mut frame, b"idempotency");
    frame.extend_from_slice(&1000u128.to_be_bytes());
    field(&mut frame, &[1, 2, 3]);
    frame.extend_from_slice(&[0x33; 32]);
    frame
}

#[test]
fn human_prepare_carries_the_pending_nonce() {
    for (nonce, pending) in [
        (2, 3),
        (4, 3),
        (3, 3),
        (0, 0),
        (0, 1),
        (1, 0),
        (u64::MAX - 1, u64::MAX),
        (u64::MAX, u64::MAX),
    ] {
        let frame = prepare_frame(nonce);
        let HumanRequest::Prepare(request) = decode_request(frame.clone()).expect("PREPARE codec")
        else {
            panic!("PREPARE request expected");
        };
        assert_eq!(request.request_id, 17);
        assert_eq!(request.key, [0x11; 32]);
        assert_eq!(request.body_digest, [0x22; 32]);
        assert_eq!(request.operation.nonce, nonce);
        assert_eq!(request.operation.not_before_height, 100);
        assert_eq!(request.operation.not_after_height, 110);
        let result = request.operation.validate_pending_nonce(pending);
        if nonce == pending {
            assert_eq!(result, Ok(()));
        } else {
            let refusal = HumanRefusal::StaleNonce {
                expected: nonce,
                pending,
            };
            assert_eq!(result, Err(refusal));
            assert_eq!(refusal.reason(), "prepare.stale_nonce");
            assert_eq!(
                refusal.class(),
                layerx_agent_api::error::ErrorClass::VerificationFailure
            );
            let response = encode_response(Err(HumanOperationError::Typed(refusal)));
            let mut expected = b"LXHAGT01\x01".to_vec();
            field(&mut expected, b"prepare.stale_nonce");
            expected.extend_from_slice(&nonce.to_be_bytes());
            expected.extend_from_slice(&pending.to_be_bytes());
            assert_eq!(response, expected);
        }
        let response = encode_response(Ok(HumanResponse::pending_nonce(pending)));
        assert_eq!(&response[..9], b"LXHAGT01\x00");
        assert_eq!(&response[9..], &pending.to_be_bytes());
        let mut trailing = frame.clone();
        trailing.push(0);
        assert!(matches!(
            decode_request(trailing),
            Err(HumanProtocolError::Malformed)
        ));
        assert!(matches!(
            decode_request(frame[..frame.len() - 1].to_vec()),
            Err(HumanProtocolError::Malformed)
        ));
    }
    let head = encode_response(Ok(HumanResponse::head(500_000)));
    assert_eq!(&head[..9], b"LXHAGT01\x00");
    assert_eq!(&head[9..], &500_000u64.to_be_bytes());
}

fn control_frame(rung: u8, receipt_hash: [u8; 32], height: u64) -> Vec<u8> {
    let mut frame = b"LXHAGT01".to_vec();
    frame.push(human::AGENT_CONTROL);
    field(&mut frame, b"agent");
    frame.push(1);
    frame.extend_from_slice(&[0x44; 32]);
    frame.extend_from_slice(&[0x55; 32]);
    frame.extend_from_slice(&[0x66; 32]);
    frame.extend_from_slice(&receipt_hash);
    frame.extend_from_slice(&height.to_be_bytes());
    frame.push(rung);
    frame.extend_from_slice(&1234u64.to_be_bytes());
    frame
}

#[test]
fn human_evidence_reports_rung_bytes() {
    let tx_hash = [0x66; 32];
    let receipt_hash = [0x77; 32];
    let height = 321u64;
    let mut request = b"LXHAGT01".to_vec();
    request.push(human::EVIDENCE);
    request.extend_from_slice(&[0x55; 32]);
    request.extend_from_slice(&tx_hash);
    let HumanRequest::Evidence {
        idempotency_key,
        tx_hash: decoded_hash,
    } = decode_request(request.clone()).expect("EVIDENCE codec")
    else {
        panic!("EVIDENCE request expected");
    };
    assert_eq!(idempotency_key, [0x55; 32]);
    assert_eq!(decoded_hash, tx_hash);
    let mut trailing = request.clone();
    trailing.push(0);
    assert!(matches!(
        decode_request(trailing),
        Err(HumanProtocolError::Malformed)
    ));
    assert!(matches!(
        decode_request(request[..request.len() - 1].to_vec()),
        Err(HumanProtocolError::Malformed)
    ));
    for (rung, byte) in [
        (Rung::Submitted, 0),
        (Rung::Receipted, 1),
        (Rung::Committed, 2),
    ] {
        let (receipt, observed) = if rung == Rung::Submitted {
            ([0; 32], 0)
        } else {
            (receipt_hash, height)
        };
        let response = encode_response(Ok(HumanResponse::evidence(
            tx_hash, receipt, observed, rung,
        )
        .expect("valid transaction evidence")));
        assert_eq!(&response[..9], b"LXHAGT01\x00");
        assert_eq!(&response[9..41], &tx_hash);
        assert_eq!(&response[41..73], &receipt);
        assert_eq!(&response[73..81], &observed.to_be_bytes());
        assert_eq!(&response[81..], &[byte]);
        if rung != Rung::Submitted {
            let HumanRequest::AgentControl { evidence, .. } =
                decode_request(control_frame(byte, receipt_hash, height))
                    .expect("finalization codec")
            else {
                panic!("agent control request expected");
            };
            assert_eq!(evidence.action_key, [0x55; 32]);
            assert_eq!(evidence.tx_hash, tx_hash);
            assert_eq!(evidence.receipt_hash, receipt_hash);
            assert_eq!(evidence.observed_height, height);
            assert_eq!(evidence.rung, byte);
            assert_eq!(evidence.finalized_at, 1234);
        }
    }
    for rung in [0, 3, 4, 5, 255] {
        assert!(matches!(
            decode_request(control_frame(rung, receipt_hash, height)),
            Err(HumanProtocolError::Malformed)
        ));
    }
    for (receipt, observed) in [([0; 32], height), (receipt_hash, 0)] {
        assert!(matches!(
            decode_request(control_frame(1, receipt, observed)),
            Err(HumanProtocolError::Malformed)
        ));
        assert_eq!(
            HumanResponse::evidence(tx_hash, receipt, observed, Rung::Receipted),
            Err(HumanProtocolError::Malformed)
        );
    }
    assert_eq!(
        HumanResponse::evidence([0; 32], receipt_hash, height, Rung::Receipted),
        Err(HumanProtocolError::Malformed)
    );
    assert_eq!(
        HumanResponse::evidence(tx_hash, receipt_hash, height, Rung::Submitted),
        Err(HumanProtocolError::Malformed)
    );
}

#[test]
fn human_opcode_numbers_are_unchanged() {
    let opcodes = [
        ("PREPARE", human::PREPARE, 1u8),
        ("SUBMIT", human::SUBMIT, 2u8),
        ("TRACK", human::TRACK, 3u8),
        ("RECEIPT_LOOKUP", human::RECEIPT_LOOKUP, 4u8),
        ("REGISTRY", human::REGISTRY, 5u8),
        ("APPROVAL_LIST", human::APPROVAL_LIST, 9u8),
        ("APPROVAL_GET", human::APPROVAL_GET, 10u8),
        ("APPROVAL_APPROVE", human::APPROVAL_APPROVE, 11u8),
        ("APPROVAL_REJECT", human::APPROVAL_REJECT, 12u8),
        ("IDENTITY_RESOLVE", human::IDENTITY_RESOLVE, 20u8),
        ("LEASE_MAP", human::LEASE_MAP, 21u8),
        ("OWNER_VALIDATE", human::OWNER_VALIDATE, 22u8),
        ("OWNER_INSTALL", human::OWNER_INSTALL, 23u8),
        ("AGENT_LIST", human::AGENT_LIST, 24u8),
        ("AGENT_GET", human::AGENT_GET, 25u8),
        ("AGENT_CONTROL", human::AGENT_CONTROL, 26u8),
        ("AGENT_LIMIT", human::AGENT_LIMIT, 27u8),
        ("AGENT_OWNER_ROTATED", human::AGENT_OWNER_ROTATED, 43u8),
        ("AGENT_JOURNEY", human::AGENT_JOURNEY, 28u8),
        ("AGENT_ARCHIVE", human::AGENT_ARCHIVE, 29u8),
        ("CAPABILITY_INSTALL", human::CAPABILITY_INSTALL, 30u8),
        ("AGENT_CONTEXT", human::AGENT_CONTEXT, 31u8),
        ("AGENT_BUDGET_STATE", human::AGENT_BUDGET_STATE, 32u8),
        ("AGENT_KEY_POLICY", human::AGENT_KEY_POLICY, 33u8),
        (
            "AGENT_SESSION_SNAPSHOT",
            human::AGENT_SESSION_SNAPSHOT,
            34u8,
        ),
        ("AGENT_SESSION_SUSPEND", human::AGENT_SESSION_SUSPEND, 35u8),
        ("AGENT_SESSION_BIND", human::AGENT_SESSION_BIND, 36u8),
        (
            "AGENT_LIFECYCLE_PUBLISH",
            human::AGENT_LIFECYCLE_PUBLISH,
            37u8,
        ),
        (
            "AGENT_SESSION_RESTRICT",
            human::AGENT_SESSION_RESTRICT,
            38u8,
        ),
        ("ACCOUNT_SEQUENCE", human::ACCOUNT_SEQUENCE, 13u8),
        ("BALANCE", human::BALANCE, 6u8),
        ("NATIVE_FEE_POLICY", human::NATIVE_FEE_POLICY, 39u8),
        ("SESSION_FEE_STATE", human::SESSION_FEE_STATE, 40u8),
        ("SESSION_SEED_PREPARE", human::SESSION_SEED_PREPARE, 41u8),
        ("ACCOUNT_STATE", human::ACCOUNT_STATE, 42u8),
        ("SUBJECT", human::SUBJECT, 44u8),
        ("OPERATOR", human::OPERATOR, 45u8),
        (
            "APPROVAL_LIST_FACTS_V2",
            human::APPROVAL_LIST_FACTS_V2,
            46u8,
        ),
        ("APPROVAL_GET_FACTS_V2", human::APPROVAL_GET_FACTS_V2, 47u8),
        (
            "APPROVAL_BUDGET_AFTER_V2",
            human::APPROVAL_BUDGET_AFTER_V2,
            48u8,
        ),
        (
            "MANAGED_EVIDENCE_BY_DIGEST_V2",
            human::MANAGED_EVIDENCE_BY_DIGEST_V2,
            49u8,
        ),
        (
            "NATIVE_APPROVAL_LIST_FACTS_V2",
            human::NATIVE_APPROVAL_LIST_FACTS_V2,
            50u8,
        ),
        (
            "NATIVE_APPROVAL_GET_FACTS_V2",
            human::NATIVE_APPROVAL_GET_FACTS_V2,
            51u8,
        ),
        (
            "NATIVE_PROGRAM_APPROVAL_LIST_V5",
            human::NATIVE_PROGRAM_APPROVAL_LIST_V5,
            58u8,
        ),
        (
            "NATIVE_PROGRAM_APPROVAL_GET_V5",
            human::NATIVE_PROGRAM_APPROVAL_GET_V5,
            59u8,
        ),
        (
            "NATIVE_PROGRAM_APPROVAL_MATERIAL_V5",
            human::NATIVE_PROGRAM_APPROVAL_MATERIAL_V5,
            60u8,
        ),
        (
            "NATIVE_PROGRAM_APPROVAL_BUDGET_V5",
            human::NATIVE_PROGRAM_APPROVAL_BUDGET_V5,
            61u8,
        ),
        (
            "NATIVE_PROGRAM_APPROVAL_DECIDE_V5",
            human::NATIVE_PROGRAM_APPROVAL_DECIDE_V5,
            62u8,
        ),
        (
            "NATIVE_EFFECT_APPROVAL_LIST_V3",
            human::NATIVE_EFFECT_APPROVAL_LIST_V3,
            52u8,
        ),
        (
            "NATIVE_EFFECT_APPROVAL_GET_V3",
            human::NATIVE_EFFECT_APPROVAL_GET_V3,
            53u8,
        ),
        (
            "NATIVE_EFFECT_APPROVAL_DECIDE_V3",
            human::NATIVE_EFFECT_APPROVAL_DECIDE_V3,
            54u8,
        ),
        (
            "NATIVE_EFFECT_APPROVAL_MATERIAL_V4",
            human::NATIVE_EFFECT_APPROVAL_MATERIAL_V4,
            55u8,
        ),
        (
            "NATIVE_EFFECT_APPROVAL_BUDGET_V4",
            human::NATIVE_EFFECT_APPROVAL_BUDGET_V4,
            56u8,
        ),
        ("AGENT_BUDGET_PROOF_V4", human::AGENT_BUDGET_PROOF_V4, 57u8),
        (
            "NATIVE_SEND_PREVIEW_V1",
            human::NATIVE_SEND_PREVIEW_V1,
            63u8,
        ),
        (
            "NATIVE_OWNER_CONTEXT_V1",
            human::NATIVE_OWNER_CONTEXT_V1,
            64u8,
        ),
        (
            "NATIVE_JOURNEY_ENVELOPE_V1",
            human::NATIVE_JOURNEY_ENVELOPE_V1,
            65u8,
        ),
        (
            "NATIVE_SEND_OWNER_PREVIEW_V1",
            human::NATIVE_SEND_OWNER_PREVIEW_V1,
            66u8,
        ),
        (
            "NATIVE_SEND_ENVELOPE_V1",
            human::NATIVE_SEND_ENVELOPE_V1,
            67u8,
        ),
        ("HEAD", human::HEAD, 7u8),
        ("EVIDENCE", human::EVIDENCE, 8u8),
    ];
    assert_eq!(opcodes.len(), 61);
    let mut seen = std::collections::BTreeSet::new();
    for (name, actual, expected) in opcodes {
        assert_eq!(actual, expected, "opcode {name} changed");
        assert!(
            seen.insert(actual),
            "opcode {name} duplicates another operation"
        );
    }
    assert_eq!(
        HumanRefusal::ExportSettlementAnchoringUnavailable.reason(),
        "export.settlement_anchoring_unavailable"
    );
    assert_eq!(
        HumanRefusal::ExportSettlementAnchoringUnavailable.class(),
        layerx_agent_api::error::ErrorClass::UnavailableCapability
    );
}
