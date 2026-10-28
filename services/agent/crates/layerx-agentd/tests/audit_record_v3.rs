use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::audit::{
    read_entries, reconstruct_session, record, redact, Coverage, DataClass, Decision, Entry,
    EventClass, Log, OutputSurface, PayloadEvidence, RecordError, StoredReceiptEvidence,
};
use layerx_agentd::identity::ProtocolAuthority;
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_agentd::tenant::{Config, RedactionPolicy, Retention};
use layerx_types::ids::{ActivityId, Did, IdempotencyKey};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

const VERSION_TWO_ENTRY: &str = concat!(
    "4c5841520209",
    "0102030405060708",
    "000874656e616e742d61",
    "00126469643a6c61796572783a6167656e742d61",
    "010101010101010101010101010101010101010101010101010101010101010101",
    "010202020202020202020202020202020202020202020202020202020202020202",
    "0009706f6c6963792d7637",
    "0303030303030303030303030303030303030303030303030303030303030303",
    "010404040404040404040404040404040404040404040404040404040404040404",
    "06",
    "000e61756469742d65766964656e6365",
    "010505050505050505050505050505050505050505050505050505050505050505",
    "03",
    "030606060606060606060606060606060606060606060606060606060606060606",
    "010909090909090909090909090909090909090909090909090909090909090909",
    "010707070707070707070707070707070707070707070707070707070707070707",
);

struct Directory(PathBuf);

impl Directory {
    fn new(label: &str) -> Self {
        let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
        Self(std::env::temp_dir().join(format!(
            "layerx-audit-v3-{label}-{}-{sequence}",
            std::process::id()
        )))
    }

    fn log(&self) -> Log {
        Log::open(&self.0, &tenant()).unwrap_or_else(|error| panic!("open audit: {error}"))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn entry() -> Entry {
    let config = Config {
        tenant: tenant(),
        policy_version: "policy-v7".to_owned(),
        redaction: RedactionPolicy::Standard,
        retention: Retention {
            events: 100,
            audit: 100,
            receipts: 100,
        },
        verification_default: VerificationLevel::STATE_PROVEN,
        approval_required_for: BTreeSet::new(),
    };
    let reason = redact(
        &config,
        &tenant(),
        OutputSurface::Audit,
        DataClass::PublicText,
        b"audit-evidence",
        10,
    )
    .unwrap_or_else(|error| panic!("redact: {error}"))
    .value;
    Entry {
        class: EventClass::TerminalOutcome,
        observed_at_ms: 0x0102_0304_0506_0708,
        tenant: tenant(),
        agent: Did::new(b"did:layerx:agent-a").unwrap_or_else(|error| panic!("DID: {error:?}")),
        session: Some(SessionId([1; 32])),
        capability: Some([2; 32]),
        policy_version: "policy-v7".to_owned(),
        request_id: [3; 32],
        idempotency_key: Some(IdempotencyKey::new([4; 32])),
        decision: Decision::Executed,
        reason,
        resulting_activity_id: None,
        verification_level: VerificationLevel::UNVERIFIED,
        protocol_authority: Some(ProtocolAuthority::CapabilityGrant([6; 32])),
        submitted_bytes: Some(PayloadEvidence::Digest([9; 32])),
        receipt_id: Some([7; 32]),
        tx_hash: Some([8; 32]),
        nonce: Some(42),
        rung: Some(Rung::Committed),
    }
}

fn read_payload(payload: &[u8]) -> Result<Vec<Entry>, RecordError> {
    let directory = Directory::new("payload");
    let mut log = directory.log();
    log.before_operation(payload, || ())?;
    read_entries(log.path())
}

#[test]
fn audit_entry_v3_records_transaction_hash_and_rung() {
    let directory = Directory::new("round-trip");
    let mut log = directory.log();
    let path = log.path().to_path_buf();
    let mut coverage = Coverage::default();
    let mut expected = Vec::new();
    for (rung, nonce) in [
        (Rung::Submitted, 0),
        (Rung::Receipted, 42),
        (Rung::Committed, u64::MAX),
    ] {
        let mut item = entry();
        item.rung = Some(rung);
        item.nonce = Some(nonce);
        let (_, observed) = record(&mut log, &mut coverage, &item, || read_entries(&path))
            .unwrap_or_else(|error| panic!("record: {error}"));
        expected.push(item);
        assert_eq!(
            observed.unwrap_or_else(|error| panic!("durable entry: {error}")),
            expected
        );
    }
    assert_eq!(coverage.count(EventClass::TerminalOutcome), 3);
    let mut authentication = entry();
    authentication.class = EventClass::Authentication;
    authentication.decision = Decision::Allowed;
    authentication.tx_hash = None;
    authentication.nonce = None;
    authentication.rung = None;
    record(&mut log, &mut coverage, &authentication, || ())
        .unwrap_or_else(|error| panic!("record authentication: {error}"));
    expected.push(authentication);
    drop(log);
    let bytes = fs::read(&path).unwrap_or_else(|error| panic!("chain bytes: {error}"));
    assert_eq!(&bytes[88..93], b"LXAR\x03");
    let decoded = read_entries(&path).unwrap_or_else(|error| panic!("read: {error}"));
    assert_eq!(decoded, expected);
    let first_length = usize::try_from(u32::from_be_bytes(
        bytes[84..88]
            .try_into()
            .unwrap_or_else(|error| panic!("payload length: {error}")),
    ))
    .unwrap_or_else(|error| panic!("payload size: {error}"));
    let payload = &bytes[88..88 + first_length];
    let mut invalid_rung = payload.to_vec();
    let last = invalid_rung.len() - 1;
    invalid_rung[last] = 3;
    assert!(matches!(
        read_payload(&invalid_rung),
        Err(RecordError::Decode(_))
    ));
    assert!(matches!(
        read_payload(&payload[..last]),
        Err(RecordError::Decode(_))
    ));
    let mut trailing = payload.to_vec();
    trailing.push(0);
    assert!(matches!(
        read_payload(&trailing),
        Err(RecordError::Decode(_))
    ));

    let submitted_bytes = b"canonical-signed-transaction".to_vec();
    let mut submission = entry();
    submission.class = EventClass::Submission;
    submission.decision = Decision::Submitted;
    submission.rung = Some(Rung::Submitted);
    submission.receipt_id = None;
    submission.submitted_bytes = Some(PayloadEvidence::Digest(
        Sha256::digest(&submitted_bytes).into(),
    ));
    let terminal = entry();
    let mut other_transaction = terminal.clone();
    other_transaction.tx_hash = Some([10; 32]);
    let mut other_session = terminal.clone();
    other_session.session = Some(SessionId([11; 32]));
    let entries = vec![
        other_transaction,
        submission.clone(),
        other_session,
        terminal.clone(),
    ];
    let receipts = BTreeMap::from([(
        [7; 32],
        StoredReceiptEvidence {
            submitted_bytes: submitted_bytes.clone(),
            core_receipt_bytes: b"stored-core-receipt".to_vec(),
        },
    )]);
    let replay = reconstruct_session(&entries, &receipts, SessionId([1; 32]), [8; 32])
        .unwrap_or_else(|error| panic!("reconstruct: {error}"));
    assert_eq!(replay.tx_hash, [8; 32]);
    assert_eq!(replay.nonce, 42);
    assert_eq!(replay.rung, Rung::Committed);
    assert_eq!(replay.decisions.len(), 2);
    assert_eq!(replay.decisions[0].decision, Decision::Submitted);
    assert_eq!(replay.decisions[1].decision, Decision::Executed);
    assert_eq!(replay.submitted_bytes, submitted_bytes);
    assert_eq!(replay.core_receipt_bytes, b"stored-core-receipt");
    assert_eq!(
        replay.protocol_authority,
        ProtocolAuthority::CapabilityGrant([6; 32])
    );
    for invalid_terminal in [
        Entry {
            nonce: Some(43),
            ..terminal.clone()
        },
        Entry {
            rung: None,
            ..terminal.clone()
        },
    ] {
        assert!(matches!(
            reconstruct_session(
                &[submission.clone(), invalid_terminal],
                &receipts,
                SessionId([1; 32]),
                [8; 32],
            ),
            Err(RecordError::IncompleteReconstruction(_))
        ));
    }
    assert!(matches!(
        reconstruct_session(
            &[submission, terminal.clone(), terminal],
            &receipts,
            SessionId([1; 32]),
            [8; 32],
        ),
        Err(RecordError::IncompleteReconstruction(_))
    ));
}

#[test]
fn version_two_audit_entries_stay_readable() {
    let payload =
        hex::decode(VERSION_TWO_ENTRY).unwrap_or_else(|error| panic!("version 2 fixture: {error}"));
    let mut legacy = entry();
    legacy.resulting_activity_id = Some(ActivityId::new([5; 32]));
    legacy.verification_level = VerificationLevel::STATE_PROVEN;
    legacy.tx_hash = None;
    legacy.nonce = None;
    legacy.rung = None;
    assert_eq!(
        read_payload(&payload).unwrap_or_else(|error| panic!("read version 2: {error}")),
        vec![legacy.clone()]
    );
    let directory = Directory::new("mixed-versions");
    let mut log = directory.log();
    log.before_operation(&payload, || ())
        .unwrap_or_else(|error| panic!("append version 2: {error}"));
    let current = entry();
    record(&mut log, &mut Coverage::default(), &current, || ())
        .unwrap_or_else(|error| panic!("append version 3: {error}"));
    assert_eq!(
        read_entries(log.path()).unwrap_or_else(|error| panic!("mixed versions: {error}")),
        vec![legacy, current]
    );
    let mut version_one = payload;
    version_one[4] = 1;
    version_one.drain(6..14);
    log.before_operation(&version_one, || ())
        .unwrap_or_else(|error| panic!("append version 1: {error}"));
    assert!(matches!(
        read_entries(log.path()),
        Err(RecordError::Version)
    ));
    for version in [0, 1, 4, u8::MAX] {
        let mut unsupported = b"LXAR".to_vec();
        unsupported.push(version);
        assert!(matches!(
            read_payload(&unsupported),
            Err(RecordError::Version)
        ));
    }
}
