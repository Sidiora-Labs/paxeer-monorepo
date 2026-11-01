use ed25519_dalek::{Signer as _, SigningKey};
use layerx_interop_gateway::adapter::{
    AdapterDescriptor, AdapterId, ConformanceSuite, PinnedSpec, SpecVersion,
};
use layerx_interop_gateway::audit::{verify_export, AuditEventKind};
use layerx_interop_gateway::error::{error_emission, GatewayError, Retriability};
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::gateway::{
    SequencerSettlementError, TranslationKind, TranslationRequest, TranslationStatus,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::{interop_gateway_core, GatewayCore};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use layerx_proof::sequencer_logs::{LogField, SequencerLog, SequencerLogRefusal};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest as wire_receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, MAX_SEQUENCER_EFFECTS, PROTOCOL_VERSION};
use layerx_wire::WireError;
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;

const ASSET: [u8; 32] = [0xc1; 32];

const FIXTURE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
));

fn fixture_case() -> &'static str {
    FIXTURE
        .split_once("\"id\": \"V1-layerxledger-pay-exact\"")
        .and_then(|(_, rest)| rest.split("\"id\":").next())
        .unwrap_or_else(|| panic!("missing recorded pay-exact case"))
}

fn field<'a>(case: &'a str, name: &str) -> &'a str {
    case.split_once(&format!("\"{name}\": \""))
        .and_then(|(_, rest)| rest.split_once('"'))
        .map(|(value, _)| value)
        .unwrap_or_else(|| panic!("missing fixture field {name}"))
}

fn hex(value: &str) -> Vec<u8> {
    let value = value
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("fixture value is not hex"));
    assert_eq!(value.len() % 2, 0);
    (0..value.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&value[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("invalid fixture hex: {error}"))
        })
        .collect()
}

fn array<const N: usize>(value: &str) -> [u8; N] {
    hex(value)
        .try_into()
        .unwrap_or_else(|_| panic!("fixture value must have {N} bytes"))
}

fn payment_exact_words(case: &str) -> [[u8; 32]; 3] {
    let transaction = hex(field(case, "rawTransaction"));
    let offsets: Vec<usize> = transaction
        .windows(4)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == [0x78, 0xea, 0x11, 0xdb]).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one recorded payExact selector");
    let calldata = &transaction[offsets[0] + 4..];
    [5, 0, 1].map(|word| {
        calldata[word * 32..(word + 1) * 32]
            .try_into()
            .unwrap_or_else(|_| panic!("recorded payExact word {word}"))
    })
}

fn signed_recorded_payment(
    case: &str,
    recorded_topics: &[[u8; 32]],
    topics: &[[u8; 32]],
) -> Vec<u8> {
    let signed = hex(field(case, "signedReceipt"));
    assert_eq!(
        canonical_receipt_digest(&signed),
        Ok(array(field(case, "receiptDigest")))
    );
    let mut unsigned = signed[..signed.len() - 69].to_vec();
    let before = recorded_topics.concat();
    let offsets: Vec<usize> = unsigned
        .windows(before.len())
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == before).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "one signed payment log");
    unsigned[offsets[0]..offsets[0] + before.len()].copy_from_slice(&topics.concat());
    let mut digest = Sha256::new();
    digest.update(b"LXP/v1/receipt\0");
    digest.update(&unsigned);
    digest.update([0]);
    let signer = SigningKey::from_bytes(&[0x5e; 32]);
    assert_eq!(
        signer.verifying_key().to_bytes(),
        array(field(case, "publicKey"))
    );
    let signature = signer.sign(&digest.finalize()).to_bytes();
    unsigned.extend_from_slice(&[1, 0, 0, 0, 64]);
    unsigned.extend_from_slice(&signature);
    unsigned
}

fn recorded_evidence() -> SequencerEvidence {
    let case = fixture_case();
    let recorded_topics: Vec<[u8; 32]> = case
        .split_once("\"topics\": [")
        .and_then(|(_, rest)| rest.split_once(']'))
        .map(|(topics, _)| topics)
        .unwrap_or_else(|| panic!("missing fixture topics"))
        .split('"')
        .filter(|value| value.starts_with("0x"))
        .map(array)
        .collect();
    let [reference, from, to] = payment_exact_words(case);
    let topics = vec![recorded_topics[0], reference, from, to];
    let signed_receipt = signed_recorded_payment(case, &recorded_topics, &topics);
    let log_index = u64::from_str_radix(
        field(case, "logIndex")
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("invalid log index")),
        16,
    )
    .unwrap_or_else(|error| panic!("invalid log index: {error}"));
    SequencerEvidence {
        transaction_hash: array(field(case, "txHash")),
        signed_receipt,
        logs: vec![SequencerLog::new(
            array(field(case, "address")),
            topics,
            hex(field(case, "data")),
            log_index,
        )],
        rung: SettlementRung::Receipted,
    }
}

fn recorded_keys() -> [SequencerAuthorization; 1] {
    let case = fixture_case();
    let bound = |name: &str| {
        case.split_once(&format!("\"{name}\": "))
            .and_then(|(_, rest)| rest.split(|c: char| !c.is_ascii_digit()).next())
            .and_then(|value| value.parse().ok())
            .unwrap_or_else(|| panic!("missing fixture bound {name}"))
    };
    [SequencerAuthorization::new(
        array(field(case, "keyId")),
        array(field(case, "publicKey")),
        bound("firstHeight"),
        bound("lastHeight"),
    )]
}

fn principal(name: &str) -> PrincipalId {
    PrincipalId::new(name).unwrap_or_else(|error| panic!("principal {name}: {error}"))
}

fn adapter_id(name: &str) -> AdapterId {
    AdapterId::new(name).unwrap_or_else(|error| panic!("adapter {name}: {error}"))
}

fn x402_descriptor() -> AdapterDescriptor {
    let version = SpecVersion::parse("2.0.0").unwrap_or_else(|error| panic!("version: {error}"));
    let spec = PinnedSpec::new(adapter_id("x402"), version, [0x51; 32])
        .unwrap_or_else(|error| panic!("spec: {error}"));
    let conformance = ConformanceSuite::new(adapter_id("x402-v2-vectors"), 96, [0x52; 32])
        .unwrap_or_else(|error| panic!("conformance: {error}"));
    AdapterDescriptor::new(adapter_id("x402"), spec, conformance)
}

fn registered_core() -> GatewayCore {
    let mut core = interop_gateway_core();
    core.register_adapter(x402_descriptor(), &TraceId::mint([0xee; 16]), 1)
        .unwrap_or_else(|error| panic!("register: {error}"));
    core
}

fn request(kind: TranslationKind, key_byte: u8) -> TranslationRequest {
    request_for_key(kind, [key_byte; 32])
}

fn request_for_key(kind: TranslationKind, key: [u8; 32]) -> TranslationRequest {
    TranslationRequest::new(adapter_id("x402"), kind, key, [0x42; 32])
        .unwrap_or_else(|error| panic!("request: {error}"))
}

#[test]
fn registration_pins_the_spec_and_conformance_suite_and_refuses_duplicates() {
    let mut core = registered_core();
    let declared = core
        .adapter(&adapter_id("x402"))
        .unwrap_or_else(|| panic!("registered adapter missing"));
    assert_eq!(declared.spec().version().as_str(), "2.0.0");
    assert_eq!(declared.spec().document_digest(), [0x51; 32]);
    assert_eq!(declared.conformance().vector_count(), 96);
    assert_eq!(declared.conformance().suite_digest(), [0x52; 32]);

    let trace = TraceId::mint([0xef; 16]);
    let refused = core
        .register_adapter(x402_descriptor(), &trace, 2)
        .map_err(|error| *error.error());
    assert_eq!(refused, Err(GatewayError::DuplicateAdapter));

    let entries = core.registry_audit().entries();
    assert_eq!(entries.len(), 1);
    assert_eq!(entries[0].kind(), AuditEventKind::AdapterRegistered);
    assert_eq!(entries[0].subject(), [0x51; 32]);
    assert_eq!(entries[0].adapter(), "x402");
}

#[test]
fn translations_against_unregistered_adapters_are_refused() {
    let mut core = interop_gateway_core();
    let trace = TraceId::mint([1; 16]);
    let refused = core
        .begin_translation(
            &principal("alice"),
            &request(TranslationKind::ReadOnly, 0x11),
            &trace,
            10,
        )
        .map_err(|error| *error.error());
    assert_eq!(refused, Err(GatewayError::UnknownAdapter));
}

#[test]
fn a_state_changing_translation_refuses_every_unproved_sequencer_termination() {
    let mut core = registered_core();
    let alice = principal("alice");
    let trace = TraceId::mint([2; 16]);
    let evidence = recorded_evidence();
    let keys = recorded_keys();
    let key = evidence.logs[0].topics()[1];
    let opened = core
        .begin_translation(
            &alice,
            &request_for_key(TranslationKind::StateChanging, key),
            &trace,
            10,
        )
        .unwrap_or_else(|error| panic!("begin: {error}"));
    assert_eq!(opened, TranslationStatus::Pending);

    let no_receipt = core
        .complete_read_only(&alice, key, &trace, 11)
        .map_err(|error| *error.error());
    assert_eq!(no_receipt, Err(GatewayError::ReceiptRequired));
    assert_nothing_recorded(&core, &alice, key);

    let signature_rejected =
        SequencerSettlementError::Gateway(GatewayError::ReceiptRejected(VerificationFailure {
            check: ReceiptCheck::SequencerSignature,
        }));
    let mut tampered = evidence.clone();
    let last = tampered.signed_receipt.len() - 1;
    tampered.signed_receipt[last] ^= 0x01;

    let foreign_signer = SigningKey::from_bytes(&[0x99; 32]);
    let foreign_public_key = foreign_signer.verifying_key().to_bytes();
    let case = fixture_case();
    assert_ne!(foreign_public_key, array::<32>(field(case, "publicKey")));
    let foreign_authority = vec![SequencerAuthorization::new(
        array(field(case, "keyId")),
        foreign_public_key,
        1000,
        1999,
    )];
    let unknown_authority = vec![SequencerAuthorization::new(
        [0x99; 32],
        array(field(case, "publicKey")),
        1000,
        1999,
    )];
    let expired_authority = vec![SequencerAuthorization::new(
        array(field(case, "keyId")),
        array(field(case, "publicKey")),
        1000,
        1000,
    )];
    let future_authority = vec![SequencerAuthorization::new(
        array(field(case, "keyId")),
        array(field(case, "publicKey")),
        1002,
        1999,
    )];
    let mut unsigned = evidence.clone();
    unsigned
        .signed_receipt
        .truncate(unsigned.signed_receipt.len() - 69);
    unsigned.signed_receipt.push(0);

    let mut altered_log = evidence.clone();
    let log = &evidence.logs[0];
    let mut altered_data = log.data().to_vec();
    let last = altered_data.len() - 1;
    altered_data[last] ^= 0x01;
    altered_log.logs[0] = SequencerLog::new(
        log.address(),
        log.topics().to_vec(),
        altered_data,
        log.log_index(),
    );
    let mut submitted = evidence.clone();
    submitted.rung = SettlementRung::Submitted;

    for (label, unproved, authorities, expected) in [
        (
            "tampered signature",
            tampered,
            keys.to_vec(),
            signature_rejected,
        ),
        (
            "foreign key",
            evidence.clone(),
            foreign_authority,
            signature_rejected,
        ),
        (
            "unknown key id",
            evidence.clone(),
            unknown_authority,
            signature_rejected,
        ),
        (
            "expired key",
            evidence.clone(),
            expired_authority,
            signature_rejected,
        ),
        (
            "future key",
            evidence.clone(),
            future_authority,
            signature_rejected,
        ),
        (
            "no pinned keys",
            evidence.clone(),
            vec![],
            signature_rejected,
        ),
        (
            "missing signature",
            unsigned,
            keys.to_vec(),
            SequencerSettlementError::Gateway(GatewayError::ReceiptRejected(VerificationFailure {
                check: ReceiptCheck::MissingSignature,
            })),
        ),
        (
            "altered log",
            altered_log,
            keys.to_vec(),
            SequencerSettlementError::Logs(SequencerLogRefusal::Log {
                index: 0,
                field: LogField::Data,
            }),
        ),
        (
            "submitted rung",
            submitted,
            keys.to_vec(),
            SequencerSettlementError::Gateway(GatewayError::ReceiptPending),
        ),
    ] {
        let before = format!("{core:?}");
        let rejected = core
            .settle_with_sequencer_evidence(&alice, key, &unproved, &authorities, &trace, 12)
            .unwrap_err();
        assert_eq!(*rejected.error(), expected, "{label}");
        assert_eq!(rejected.trace(), &trace, "{label}");
        assert_nothing_recorded(&core, &alice, key);
        assert_eq!(format!("{core:?}"), before, "{label} must write nothing");
    }

    let settled = core
        .settle_with_sequencer_evidence(&alice, key, &evidence, &keys, &trace, 13)
        .unwrap_or_else(|error| panic!("recorded receipt with pinned keys: {error}"));
    assert_eq!(
        settled,
        TranslationStatus::ReceiptVerified {
            receipt_digest: canonical_receipt_digest(&evidence.signed_receipt)
                .unwrap_or_else(|error| panic!("native receipt digest: {error:?}")),
        }
    );
}

#[test]
fn a_verified_receipt_terminates_the_translation_and_binds_the_audit_chain() {
    let mut core = registered_core();
    let alice = principal("alice");
    let trace = TraceId::mint([2; 16]);
    let material = recorded_evidence();
    let key = material.logs[0].topics()[1];
    core.begin_translation(
        &alice,
        &request_for_key(TranslationKind::StateChanging, key),
        &trace,
        10,
    )
    .unwrap_or_else(|error| panic!("begin: {error}"));

    let settled = core
        .settle_with_sequencer_evidence(&alice, key, &material, &recorded_keys(), &trace, 14)
        .unwrap_or_else(|error| panic!("settle: {error}"));
    let receipt_digest = canonical_receipt_digest(&material.signed_receipt)
        .unwrap_or_else(|error| panic!("native receipt digest: {error:?}"));
    assert_eq!(
        settled,
        TranslationStatus::ReceiptVerified { receipt_digest }
    );

    let replay = core
        .settle_with_sequencer_evidence(&alice, key, &material, &recorded_keys(), &trace, 15)
        .unwrap_or_else(|error| panic!("replayed settle: {error}"));
    assert_eq!(
        replay,
        TranslationStatus::ReceiptVerified { receipt_digest }
    );

    let logs = vec![transfer_log(0), payment_exact_log(1, key)];
    let other = SequencerEvidence {
        transaction_hash: SEQUENCER_ACTIVITY_ID,
        signed_receipt: signed_sequencer_receipt(&logs),
        logs,
        rung: SettlementRung::Receipted,
    };
    let conflict = core
        .settle_with_sequencer_evidence(&alice, key, &other, &in_range_keys(), &trace, 16)
        .map_err(|error| *error.error());
    assert_eq!(
        conflict,
        Err(SequencerSettlementError::Gateway(
            GatewayError::IdempotencyConflict
        ))
    );

    let chain = core
        .principal_audit(&alice)
        .unwrap_or_else(|| panic!("principal audit chain missing"));
    let completed = chain
        .entries()
        .iter()
        .find(|entry| entry.kind() == AuditEventKind::TranslationCompleted)
        .unwrap_or_else(|| panic!("completion was not audited"));
    assert_eq!(completed.subject(), receipt_digest);
    assert_eq!(completed.trace(), &trace);
    let head = chain.head().unwrap_or_else(|error| panic!("head: {error}"));
    verify_export("alice", &chain.export(), &head)
        .unwrap_or_else(|error| panic!("audit chain broken: {error}"));
}

#[test]
fn idempotency_keys_bind_content_and_replay_the_original() {
    let mut core = registered_core();
    let alice = principal("alice");
    let trace = TraceId::mint([3; 16]);
    let first = request(TranslationKind::StateChanging, 0x31);
    core.begin_translation(&alice, &first, &trace, 10)
        .unwrap_or_else(|error| panic!("begin: {error}"));
    let replay = core
        .begin_translation(&alice, &first, &trace, 11)
        .unwrap_or_else(|error| panic!("replay: {error}"));
    assert_eq!(replay, TranslationStatus::Pending);

    let conflicting = TranslationRequest::new(
        adapter_id("x402"),
        TranslationKind::StateChanging,
        [0x31; 32],
        [0x43; 32],
    )
    .unwrap_or_else(|error| panic!("conflicting request: {error}"));
    let conflict = core
        .begin_translation(&alice, &conflicting, &trace, 12)
        .map_err(|error| *error.error());
    assert_eq!(conflict, Err(GatewayError::IdempotencyConflict));

    let rekind = TranslationRequest::new(
        adapter_id("x402"),
        TranslationKind::ReadOnly,
        [0x31; 32],
        [0x42; 32],
    )
    .unwrap_or_else(|error| panic!("rekind request: {error}"));
    let kind_conflict = core
        .begin_translation(&alice, &rekind, &trace, 13)
        .map_err(|error| *error.error());
    assert_eq!(kind_conflict, Err(GatewayError::IdempotencyConflict));

    assert_eq!(
        TranslationRequest::new(
            adapter_id("x402"),
            TranslationKind::ReadOnly,
            [0; 32],
            [0x42; 32]
        ),
        Err(GatewayError::InvalidTranslation),
        "the reserved zero idempotency key must be refused"
    );
}

#[test]
fn principals_are_isolated_across_every_gateway_surface() {
    let mut core = registered_core();
    let alice = principal("alice");
    let mallory = principal("mallory");
    let trace = TraceId::mint([4; 16]);
    let material = recorded_evidence();
    let key = material.logs[0].topics()[1];
    core.begin_translation(
        &alice,
        &request_for_key(TranslationKind::StateChanging, key),
        &trace,
        10,
    )
    .unwrap_or_else(|error| panic!("begin: {error}"));

    assert_eq!(core.translation(&mallory, key), None);
    let stolen = core
        .settle_with_sequencer_evidence(&mallory, key, &material, &recorded_keys(), &trace, 11)
        .map_err(|error| *error.error());
    assert_eq!(
        stolen,
        Err(SequencerSettlementError::Gateway(
            GatewayError::UnknownTranslation
        )),
        "a principal must not reach another principal's translation"
    );
    assert_eq!(
        core.translation(&alice, key),
        Some(TranslationStatus::Pending)
    );

    let same_key = core
        .begin_translation(
            &mallory,
            &request_for_key(TranslationKind::StateChanging, key),
            &trace,
            12,
        )
        .unwrap_or_else(|error| panic!("independent begin: {error}"));
    assert_eq!(
        same_key,
        TranslationStatus::Pending,
        "the same key under another principal is an independent translation, not a replay"
    );

    let alice_chain = core
        .principal_audit(&alice)
        .unwrap_or_else(|| panic!("alice audit missing"));
    let mallory_chain = core
        .principal_audit(&mallory)
        .unwrap_or_else(|| panic!("mallory audit missing"));
    assert_eq!(alice_chain.entries().len(), 1);
    assert_eq!(mallory_chain.entries().len(), 1);
    let alice_head = alice_chain
        .head()
        .unwrap_or_else(|error| panic!("head: {error}"));
    assert!(
        verify_export("mallory", &alice_chain.export(), &alice_head).is_err(),
        "one principal's chain must not verify under another principal's genesis"
    );
}

#[test]
fn read_only_translations_complete_without_receipts_but_cannot_claim_settlement() {
    let mut core = registered_core();
    let alice = principal("alice");
    let trace = TraceId::mint([5; 16]);
    let material = recorded_evidence();
    let key = material.logs[0].topics()[1];
    core.begin_translation(
        &alice,
        &request_for_key(TranslationKind::ReadOnly, key),
        &trace,
        10,
    )
    .unwrap_or_else(|error| panic!("begin: {error}"));
    let misclaimed = core
        .settle_with_sequencer_evidence(&alice, key, &material, &recorded_keys(), &trace, 11)
        .map_err(|error| *error.error());
    assert_eq!(
        misclaimed,
        Err(SequencerSettlementError::Gateway(
            GatewayError::NotStateChanging
        ))
    );

    let completed = core
        .complete_read_only(&alice, key, &trace, 12)
        .unwrap_or_else(|error| panic!("complete: {error}"));
    assert_eq!(completed, TranslationStatus::Translated);
    let replay = core
        .complete_read_only(&alice, key, &trace, 13)
        .unwrap_or_else(|error| panic!("replay: {error}"));
    assert_eq!(replay, TranslationStatus::Translated);
}

#[test]
fn refusals_are_honest_terminal_states() {
    let mut core = registered_core();
    let alice = principal("alice");
    let trace = TraceId::mint([6; 16]);
    let material = recorded_evidence();
    let key = material.logs[0].topics()[1];
    core.begin_translation(
        &alice,
        &request_for_key(TranslationKind::StateChanging, key),
        &trace,
        10,
    )
    .unwrap_or_else(|error| panic!("begin: {error}"));
    let refused = core
        .refuse_translation(&alice, key, &trace, 11)
        .unwrap_or_else(|error| panic!("refuse: {error}"));
    assert_eq!(refused, TranslationStatus::Refused);

    let closed = core
        .settle_with_sequencer_evidence(&alice, key, &material, &recorded_keys(), &trace, 12)
        .map_err(|error| *error.error());
    assert_eq!(
        closed,
        Err(SequencerSettlementError::Gateway(
            GatewayError::TranslationClosed
        ))
    );
    assert_eq!(
        core.translation(&alice, key),
        Some(TranslationStatus::Refused)
    );
}

#[test]
fn traces_propagate_onto_audits_errors_and_boundary_headers() {
    let mut core = registered_core();
    let alice = principal("alice");
    let begin_trace = TraceId::from_inbound(Some("trc_00112233445566778899aabbccddeeff"), [7; 16]);
    assert_eq!(
        begin_trace.as_str(),
        "trc_00112233445566778899aabbccddeeff",
        "a well-formed inbound trace must cross the boundary unchanged"
    );
    let material = recorded_evidence();
    let key = material.logs[0].topics()[1];
    core.begin_translation(
        &alice,
        &request_for_key(TranslationKind::StateChanging, key),
        &begin_trace,
        10,
    )
    .unwrap_or_else(|error| panic!("begin: {error}"));

    let settle_trace = TraceId::mint([8; 16]);
    core.settle_with_sequencer_evidence(
        &alice,
        key,
        &material,
        &recorded_keys(),
        &settle_trace,
        11,
    )
    .unwrap_or_else(|error| panic!("settle: {error}"));

    let chain = core
        .principal_audit(&alice)
        .unwrap_or_else(|| panic!("audit missing"));
    assert_eq!(chain.entries()[0].trace(), &begin_trace);
    assert_eq!(chain.entries()[1].trace(), &settle_trace);

    let error_trace = TraceId::mint([9; 16]);
    let Err(failure) = core.settle_with_sequencer_evidence(
        &alice,
        [0x72; 32],
        &material,
        &recorded_keys(),
        &error_trace,
        12,
    ) else {
        panic!("an unknown translation must not settle");
    };
    assert_eq!(failure.trace(), &error_trace);
    let SequencerSettlementError::Gateway(error) = failure.error() else {
        panic!("unknown translation must return a gateway error");
    };
    assert_eq!(error, &GatewayError::UnknownTranslation);
    assert_eq!(error.retriability(), Retriability::Terminal);
    let rendered = error_emission(&failure.trace().wrap(*error))
        .unwrap_or_else(|error| panic!("error emission: {error}"));
    assert!(!rendered.is_empty());
}

const SEQUENCER_MODULE_ID: u16 = 12;
const SEQUENCER_KEY_ID: [u8; 32] = [0x33; 32];
const SEQUENCER_HEIGHT: u64 = 26_400_000;
const SEQUENCER_SEED: [u8; 32] = [7; 32];
const SEQUENCER_ACTIVITY_ID: [u8; 32] = [0xac; 32];
const SEQUENCER_PREVIOUS_HASH: [u8; 32] = [0x04; 32];
const SEQUENCER_SEQUENCE: u64 = 7;
const SEQUENCER_FEE_DENOM: &str = "upax";
const LEDGER_ADDRESS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];
const PAYER: [u8; 32] = [0xa1; 32];
const PAYEE: [u8; 32] = [0xb2; 32];
const GRANT: [u8; 32] = [0x6a; 32];
const CONTEXT: [u8; 32] = [0x7c; 32];
const PAID_AMOUNT: u128 = 1_000;
const EFFECT_KIND_STATE: u8 = 1;
const EFFECT_KIND_EVENT: u8 = 3;

fn keccak256(bytes: &[u8]) -> [u8; 32] {
    Keccak256::digest(bytes).into()
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn abi_amount(amount: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&amount.to_be_bytes());
    word
}

fn transfer_log(log_index: u64) -> SequencerLog {
    let mut reason = [0; 32];
    reason[31] = 1;
    SequencerLog::new(
        LEDGER_ADDRESS,
        vec![
            keccak256(b"Transfer(bytes32,bytes32,bytes32,uint128,uint8)"),
            PAYER,
            PAYEE,
            ASSET,
        ],
        [abi_amount(PAID_AMOUNT), reason].concat(),
        log_index,
    )
}

fn payment_exact_log(log_index: u64, payment_reference: [u8; 32]) -> SequencerLog {
    let abi: serde_json::Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../precompiles/layerxledger/abi.json"
    )))
    .unwrap();
    let event = abi
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| entry["name"] == "PaymentExact")
        .unwrap();
    let mut topics = vec![keccak256(
        b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)",
    )];
    for input in event["inputs"].as_array().unwrap() {
        if input["indexed"] == true {
            topics.push(match input["name"].as_str().unwrap() {
                "paymentReference" => payment_reference,
                "from" => PAYER,
                "to" => PAYEE,
                name => panic!("unexpected indexed PaymentExact field: {name}"),
            });
        }
    }
    SequencerLog::new(
        LEDGER_ADDRESS,
        topics,
        [ASSET, abi_amount(PAID_AMOUNT)].concat(),
        log_index,
    )
}

fn payment_drawn_log(log_index: u64, idempotency_key: [u8; 32]) -> SequencerLog {
    SequencerLog::new(
        LEDGER_ADDRESS,
        vec![
            keccak256(b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)"),
            GRANT,
            PAYER,
            PAYEE,
        ],
        [ASSET, abi_amount(PAID_AMOUNT), idempotency_key, CONTEXT].concat(),
        log_index,
    )
}

fn sequencer_summary_body(log_count: u32) -> Vec<u8> {
    let mut body = Vec::new();
    body.extend_from_slice(&61_340_u64.to_be_bytes());
    body.extend_from_slice(&61_340_u64.to_be_bytes());
    body.extend_from_slice(&1_000_000_000_u64.to_be_bytes());
    body.extend_from_slice(&0_u32.to_be_bytes());
    body.extend_from_slice(&[0; 20]);
    body.extend_from_slice(&[0; 20]);
    body.extend_from_slice(&sha256(b""));
    body.extend_from_slice(&0_u32.to_be_bytes());
    body.extend_from_slice(&log_count.to_be_bytes());
    body.push(u8::try_from(SEQUENCER_FEE_DENOM.len()).unwrap_or(u8::MAX));
    body.extend_from_slice(SEQUENCER_FEE_DENOM.as_bytes());
    body
}

fn sequencer_log_body(log: &SequencerLog) -> Vec<u8> {
    let mut body = log.address().to_vec();
    body.push(u8::try_from(log.topics().len()).unwrap_or(u8::MAX));
    for topic in log.topics() {
        body.extend_from_slice(topic);
    }
    body.extend_from_slice(&sha256(log.data()));
    body
}

fn encode_sequencer_receipt(
    logs: &[SequencerLog],
    signature: Option<[u8; 64]>,
) -> Result<Vec<u8>, WireError> {
    let log_count = u32::try_from(logs.len()).unwrap_or_else(|_| panic!("log count exceeds u32"));
    let mut effects = vec![(0_u16, EFFECT_KIND_STATE, sequencer_summary_body(log_count))];
    for (ordinal, log) in (1_u16..).zip(logs) {
        effects.push((ordinal, EFFECT_KIND_EVENT, sequencer_log_body(log)));
    }
    let mut sequence_preimage = b"PAXEER/v1/sequence\0".to_vec();
    sequence_preimage.extend_from_slice(&SEQUENCER_PREVIOUS_HASH);
    sequence_preimage.extend_from_slice(&SEQUENCER_SEQUENCE.to_be_bytes());
    sequence_preimage.extend_from_slice(&SEQUENCER_ACTIVITY_ID);
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&SEQUENCER_HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&[0xaa; 20]);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&LEDGER_ADDRESS);

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    encoder.structure_header_version(0x5201, PROTOCOL_VERSION)?;
    encoder.u16(PROTOCOL_VERSION)?;
    encoder.bytes(&SEQUENCER_ACTIVITY_ID, 32)?;
    encoder.u64(SEQUENCER_SEQUENCE)?;
    encoder.bytes(&SEQUENCER_PREVIOUS_HASH, 32)?;
    encoder.bytes(&sha256(&sequence_preimage), 32)?;
    encoder.bytes(&sha256(b""), 32)?;
    encoder.i32(0)?;
    encoder.sequence_length(effects.len(), MAX_SEQUENCER_EFFECTS)?;
    for (ordinal, kind, body) in &effects {
        encoder.u16(SEQUENCER_MODULE_ID)?;
        encoder.u16(*ordinal)?;
        encoder.u16(0)?;
        encoder.tag(*kind, 3)?;
        encoder.u8(0)?;
        encoder.bytes(&[0; 32], 32)?;
        encoder.bytes(body, 256)?;
    }
    encoder.u128(52_000_000_000_000)?;
    encoder.bytes(&batch_id, 32)?;
    encoder.u16(SEQUENCER_MODULE_ID)?;
    encoder.u32(1)?;
    encoder.u32(0)?;
    encoder.u8(2)?;
    encoder.bytes(&sha256(SEQUENCER_FEE_DENOM.as_bytes()), 32)?;
    encoder.u128(1_000_000_000_000_000_000)?;
    encoder.bytes(&from, 32)?;
    encoder.u128(5_000_000_000_000_000_000)?;
    encoder.u128(3_999_948_000_000_000_000)?;
    encoder.u64(41)?;
    encoder.bytes(&to, 32)?;
    encoder.u128(2_000_000_000_000_000_000)?;
    encoder.u128(3_000_000_000_000_000_000)?;
    encoder.bytes(&[0; 32], 32)?;
    encoder.bytes(&[0; 32], 32)?;
    encoder.bytes(&SEQUENCER_KEY_ID, 32)?;
    encoder.u64(1_750_000_000_250)?;
    encoder.u8(u8::from(signature.is_some()))?;
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64)?;
    }
    Ok(encoder.finish())
}

fn signed_sequencer_receipt(logs: &[SequencerLog]) -> Vec<u8> {
    let signer = SigningKey::from_bytes(&SEQUENCER_SEED);
    let unsigned = encode_sequencer_receipt(logs, None)
        .unwrap_or_else(|error| panic!("unsigned receipt: {error:?}"));
    let digest =
        wire_receipt_digest(&unsigned).unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    encode_sequencer_receipt(logs, Some(signer.sign(&digest).to_bytes()))
        .unwrap_or_else(|error| panic!("signed receipt: {error:?}"))
}

fn receipt_keys(first: u64, last: u64) -> [SequencerAuthorization; 1] {
    [SequencerAuthorization::new(
        SEQUENCER_KEY_ID,
        SigningKey::from_bytes(&SEQUENCER_SEED)
            .verifying_key()
            .to_bytes(),
        first,
        last,
    )]
}

fn in_range_keys() -> [SequencerAuthorization; 1] {
    receipt_keys(SEQUENCER_HEIGHT - 100, SEQUENCER_HEIGHT + 100)
}

fn pending_state_changing(core: &mut GatewayCore, alice: &PrincipalId, key_byte: u8) {
    let opened = core
        .begin_translation(
            alice,
            &request(TranslationKind::StateChanging, key_byte),
            &TraceId::mint([0x10; 16]),
            10,
        )
        .unwrap_or_else(|error| panic!("begin: {error}"));
    assert_eq!(opened, TranslationStatus::Pending);
}

fn assert_nothing_recorded(core: &GatewayCore, alice: &PrincipalId, key: [u8; 32]) {
    assert_eq!(
        core.translation(alice, key),
        Some(TranslationStatus::Pending),
        "a refused sequencer settlement must leave the translation pending"
    );
    let chain = core
        .principal_audit(alice)
        .unwrap_or_else(|| panic!("principal audit chain missing"));
    assert_eq!(chain.entries().len(), 1);
    assert!(
        chain
            .entries()
            .iter()
            .all(|entry| entry.kind() != AuditEventKind::TranslationCompleted),
        "a refused sequencer settlement must not audit a completion"
    );
}

fn assert_settled(core: &GatewayCore, alice: &PrincipalId, key: [u8; 32], receipt: &[u8]) {
    let receipt_digest = canonical_receipt_digest(receipt)
        .unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    assert_eq!(
        core.translation(alice, key),
        Some(TranslationStatus::ReceiptVerified { receipt_digest })
    );
    let chain = core
        .principal_audit(alice)
        .unwrap_or_else(|| panic!("principal audit chain missing"));
    let completed = chain
        .entries()
        .iter()
        .find(|entry| entry.kind() == AuditEventKind::TranslationCompleted)
        .unwrap_or_else(|| panic!("completion was not audited"));
    assert_eq!(completed.subject(), receipt_digest);
    let head = chain.head().unwrap_or_else(|error| panic!("head: {error}"));
    verify_export("alice", &chain.export(), &head)
        .unwrap_or_else(|error| panic!("audit chain broken: {error}"));
}

#[test]
fn a_payment_exact_receipt_matched_by_payment_reference_settles_the_translation() {
    let mut core = registered_core();
    let alice = principal("alice");
    let key = [0x81; 32];
    pending_state_changing(&mut core, &alice, 0x81);
    let logs = vec![transfer_log(0), payment_exact_log(1, key)];
    let receipt = signed_sequencer_receipt(&logs);
    let trace = TraceId::mint([0x81; 16]);

    let settled = core
        .settle_with_sequencer_receipt(
            &alice,
            key,
            &receipt,
            logs.clone(),
            &in_range_keys(),
            &trace,
            11,
        )
        .unwrap_or_else(|error| panic!("settle: {error}"));
    assert_eq!(
        settled,
        TranslationStatus::ReceiptVerified {
            receipt_digest: canonical_receipt_digest(&receipt)
                .unwrap_or_else(|error| panic!("receipt digest: {error:?}"))
        }
    );
    assert_settled(&core, &alice, key, &receipt);

    let replay = core
        .settle_with_sequencer_receipt(&alice, key, &receipt, logs, &in_range_keys(), &trace, 12)
        .unwrap_or_else(|error| panic!("replayed settle: {error}"));
    assert_eq!(replay, settled);
}

#[test]
fn a_payment_exact_receipt_with_legacy_topic_order_is_refused() {
    let mut core = registered_core();
    let alice = principal("alice");
    let [key, from, to] = payment_exact_words(fixture_case());
    assert_ne!(key, from);
    assert_ne!(key, to);
    assert_ne!(from, to);
    let opened = core
        .begin_translation(
            &alice,
            &request_for_key(TranslationKind::StateChanging, key),
            &TraceId::mint([0x81; 16]),
            10,
        )
        .unwrap_or_else(|error| panic!("begin: {error}"));
    assert_eq!(opened, TranslationStatus::Pending);
    let evidence = recorded_evidence();
    let native = &evidence.logs[0];
    let mut topics = native.topics().to_vec();
    assert_eq!(&topics[1..], &[key, from, to]);
    topics[1..].copy_from_slice(&[from, to, key]);
    let logs = vec![SequencerLog::new(
        native.address(),
        topics,
        native.data().to_vec(),
        0,
    )];
    let receipt = signed_sequencer_receipt(&logs);
    let result = core
        .settle_with_sequencer_receipt(
            &alice,
            key,
            &receipt,
            logs,
            &in_range_keys(),
            &TraceId::mint([0x81; 16]),
            11,
        )
        .map_err(|error| *error.error());
    assert_eq!(result, Err(SequencerSettlementError::PaymentUnmatched));
    assert_nothing_recorded(&core, &alice, key);
}

#[test]
fn a_payment_drawn_receipt_matched_by_its_idempotency_key_settles_the_translation() {
    let mut core = registered_core();
    let alice = principal("alice");
    let key = [0x82; 32];
    pending_state_changing(&mut core, &alice, 0x82);
    let logs = vec![transfer_log(0), payment_drawn_log(1, key)];
    let receipt = signed_sequencer_receipt(&logs);

    let settled = core
        .settle_with_sequencer_receipt(
            &alice,
            key,
            &receipt,
            logs,
            &in_range_keys(),
            &TraceId::mint([0x82; 16]),
            11,
        )
        .unwrap_or_else(|error| panic!("settle: {error}"));
    assert_eq!(
        settled,
        TranslationStatus::ReceiptVerified {
            receipt_digest: canonical_receipt_digest(&receipt)
                .unwrap_or_else(|error| panic!("receipt digest: {error:?}"))
        }
    );
    assert_settled(&core, &alice, key, &receipt);
}

#[test]
fn a_payment_for_a_differing_idempotency_key_is_refused_and_records_nothing() {
    let mut core = registered_core();
    let alice = principal("alice");
    let key = [0x83; 32];
    pending_state_changing(&mut core, &alice, 0x83);
    let trace = TraceId::mint([0x83; 16]);

    for logs in [
        vec![transfer_log(0), payment_exact_log(1, [0x84; 32])],
        vec![transfer_log(0), payment_drawn_log(1, [0x84; 32])],
    ] {
        let receipt = signed_sequencer_receipt(&logs);
        let refused = core
            .settle_with_sequencer_receipt(
                &alice,
                key,
                &receipt,
                logs,
                &in_range_keys(),
                &trace,
                11,
            )
            .map_err(|error| *error.error());
        assert_eq!(refused, Err(SequencerSettlementError::PaymentUnmatched));
        assert_nothing_recorded(&core, &alice, key);
    }
}

#[test]
fn a_receipt_key_outside_its_height_range_is_refused_and_records_nothing() {
    let mut core = registered_core();
    let alice = principal("alice");
    let key = [0x85; 32];
    pending_state_changing(&mut core, &alice, 0x85);
    let logs = vec![transfer_log(0), payment_exact_log(1, key)];
    let receipt = signed_sequencer_receipt(&logs);
    let trace = TraceId::mint([0x85; 16]);

    for keys in [
        receipt_keys(SEQUENCER_HEIGHT - 100, SEQUENCER_HEIGHT - 1),
        receipt_keys(SEQUENCER_HEIGHT + 1, SEQUENCER_HEIGHT + 100),
    ] {
        let refused = core
            .settle_with_sequencer_receipt(&alice, key, &receipt, logs.clone(), &keys, &trace, 11)
            .map_err(|error| *error.error());
        assert_eq!(
            refused,
            Err(SequencerSettlementError::Gateway(
                GatewayError::ReceiptRejected(VerificationFailure {
                    check: ReceiptCheck::SequencerSignature
                })
            ))
        );
        assert_nothing_recorded(&core, &alice, key);
    }
}

#[test]
fn an_altered_log_is_refused_and_records_nothing() {
    let mut core = registered_core();
    let alice = principal("alice");
    let key = [0x86; 32];
    pending_state_changing(&mut core, &alice, 0x86);
    let logs = vec![transfer_log(0), payment_exact_log(1, key)];
    let receipt = signed_sequencer_receipt(&logs);
    let altered_payment = payment_exact_log(1, key);
    let mut altered_data = altered_payment.data().to_vec();
    altered_data[63] ^= 0x01;
    let altered = vec![
        transfer_log(0),
        SequencerLog::new(
            altered_payment.address(),
            altered_payment.topics().to_vec(),
            altered_data,
            altered_payment.log_index(),
        ),
    ];

    let refused = core
        .settle_with_sequencer_receipt(
            &alice,
            key,
            &receipt,
            altered,
            &in_range_keys(),
            &TraceId::mint([0x86; 16]),
            11,
        )
        .map_err(|error| *error.error());
    assert_eq!(
        refused,
        Err(SequencerSettlementError::Logs(SequencerLogRefusal::Log {
            index: 1,
            field: LogField::Data
        }))
    );
    assert_nothing_recorded(&core, &alice, key);
}
