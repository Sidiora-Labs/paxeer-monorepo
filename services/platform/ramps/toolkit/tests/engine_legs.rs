include!("support/required.rs");

use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_ramp_toolkit::journal::{Journal, TransitionEvidence, WorkflowStage};
use layerx_ramp_toolkit::{
    compile_payer_grant_draw, AuthenticatedPrincipal, CreateOrder, OperatorIdentity, QuoteTerms,
    RampDirection, RampError, RampOrder,
};
use layerx_types::rung::Rung;
use serde::Serialize;
use sha2::{Digest as _, Sha256};

#[path = "support/repair_22_33_1490.rs"]
pub mod repair_1490;
pub use repair_1490::*;

#[test]
fn engine_written_v2_journal_uses_production_settlement_recovery() {
    repair_1490::engine_restart();
}

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
const WORKER: &str = "journal-worker";

struct JournalDirectory(PathBuf);

impl JournalDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ramp-engine-legs-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).required("create journal directory");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt as _;
            fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
                .required("protect journal directory");
        }
        Self(path)
    }

    fn path(&self) -> PathBuf {
        self.0.join("journal.jsonl")
    }
}

impl Drop for JournalDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).required("remove journal directory");
    }
}

fn order(direction: RampDirection) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: "order-1".into(),
            quote_id: "quote-1".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([7; 32]),
            payer_grant_purpose: (direction == RampDirection::OffRamp).then_some([6; 32]),
        },
        QuoteTerms {
            quote_id: "quote-1".into(),
            direction,
            layerx_asset: [1; 32],
            layerx_amount: 1_000,
            external_currency: "EUR".into(),
            external_amount_minor: 100,
            rate_numerator: 10,
            rate_denominator: 1,
            fee_minor: 2,
            maximum_slippage_bps: 25,
            context: [8; 32],
            provider_token: "product-eur".into(),
            payout_token: "beneficiary-123".into(),
            expires_at: 2_000,
        },
        AuthenticatedPrincipal {
            principal_id: "customer-1".into(),
            account: "agent:did:layerx:customer-1:main".into(),
        },
        OperatorIdentity {
            principal_id: "operator-1".into(),
            account: "agent:did:layerx:operator-1:main".into(),
            signer_key_handle: "kms.operator-1.primary".into(),
        },
        1_000,
    )
    .required("bind order")
}

fn assert_record_hashes(path: &PathBuf) {
    let bytes = fs::read(path).required("journal bytes");
    let mut previous_hash = [0; 32];
    for line in std::str::from_utf8(&bytes)
        .required("journal UTF-8")
        .lines()
    {
        let record: serde_json::Value = serde_json::from_str(line).required("record JSON");
        assert_eq!(record["version"], 2);
        assert_eq!(record["previous_hash"], serde_json::json!(previous_hash));
        let (body, _) = line
            .rsplit_once(",\"record_hash\":")
            .required("record hash field");
        let body = format!("{body}}}");
        let mut hasher = Sha256::new();
        hasher.update(b"LXP/market-maker-ramp/journal/v2\0");
        hasher.update(body.as_bytes());
        previous_hash = hasher.finalize().into();
        assert_eq!(record["record_hash"], serde_json::json!(previous_hash));
    }
}

fn assert_replay_and_rejection(
    path: &PathBuf,
    digest: [u8; 32],
    expected: &layerx_ramp_toolkit::journal::OrderSnapshot,
    tx_hash: [u8; 32],
    raw_transaction: &[u8],
    receipt_digest: [u8; 32],
) {
    let mut replay = Journal::open(path).required("reopen journal");
    let snapshot = replay.order(&digest).required("replayed order");
    assert_eq!(snapshot, expected);
    assert_eq!(snapshot.evidence.tx_hash, Some(tx_hash));
    assert_eq!(
        snapshot.evidence.raw_transaction.as_deref(),
        Some(raw_transaction)
    );
    assert_eq!(snapshot.evidence.receipt_digest, Some(receipt_digest));
    assert_eq!(snapshot.evidence.receipt_height, Some(42));
    assert_eq!(snapshot.evidence.rung, Some(Rung::Committed));
    assert_eq!(snapshot.presentation().transaction_hash, Some(tx_hash));
    replay
        .recover_verified(|_| Ok(()))
        .required("recover v2 writer");
    assert!(replay.health().ready);
    let mut settled = TransitionEvidence::empty();
    settled.provider_operation_id = Some("provider-1".into());
    settled.provider_evidence_digest = Some([6; 32]);
    replay
        .transition(
            digest,
            WorkflowStage::LayerxVerified,
            WorkflowStage::ProviderSettled,
            settled,
            WORKER,
            1_006,
        )
        .required("settle provider");
    replay
        .transition(
            digest,
            WorkflowStage::ProviderSettled,
            WorkflowStage::Done,
            TransitionEvidence::empty(),
            WORKER,
            1_007,
        )
        .required("complete with transaction evidence");
    drop(replay);
    let completed = Journal::open(path).required("replay completed v2 journal");
    assert_eq!(
        completed.order(&digest).required("completed order").stage,
        WorkflowStage::Done
    );
    drop(completed);

    let text = fs::read_to_string(path).required("read completed journal");
    let tampered = text.replace("\"rung\":\"committed\"", "\"rung\":\"receipted\"");
    assert_ne!(text, tampered);
    fs::write(path, tampered).required("alter recorded rung");
    assert!(matches!(Journal::open(path), Err(RampError::Journal)));
}

fn assert_uncommitted_receipt_rejected(
    journal: &mut Journal,
    digest: [u8; 32],
    evidence: &TransitionEvidence,
) {
    assert!(matches!(
        journal.transition(
            digest,
            WorkflowStage::LayerxPending,
            WorkflowStage::LayerxVerified,
            evidence.clone(),
            WORKER,
            1_005,
        ),
        Err(RampError::IllegalTransition)
    ));
    assert_eq!(
        journal.order(&digest).required("pending order").stage,
        WorkflowStage::LayerxPending
    );
}

#[test]
fn journal_records_transaction_hash_and_rung() {
    let directory = JournalDirectory::new();
    let path = directory.path();
    let order = order(RampDirection::OffRamp);
    let digest = order.order_digest;
    let payment = compile_payer_grant_draw(&order, 9).required("compile ledger draw");
    let raw_transaction = Eip1559Call {
        chain_id: 125,
        nonce: 9,
        max_fee_per_gas: 10,
        max_priority_fee_per_gas: 1,
        gas_limit: 200_000,
        to: payment.to,
        data: payment.data,
    }
    .sign(&[1; 32])
    .required("sign ledger transaction");
    let tx_hash = transaction_hash(&raw_transaction);
    let receipt_digest: [u8; 32] = Sha256::digest(b"journal receipt evidence").into();
    let mut journal = Journal::open(&path).required("open journal");
    journal.create_order(order, 1_001).required("create order");
    journal
        .acquire_lease(digest, WORKER, 1_001, 30)
        .required("lease");
    journal
        .transition(
            digest,
            WorkflowStage::CompliancePending,
            WorkflowStage::AwaitingLayerxPayment,
            TransitionEvidence::empty(),
            WORKER,
            1_002,
        )
        .required("approve payment");
    let mut evidence = TransitionEvidence::empty();
    evidence.tx_hash = Some(tx_hash);
    evidence.raw_transaction = Some(raw_transaction.clone());
    evidence.rung = Some(Rung::Submitted);
    journal
        .transition(
            digest,
            WorkflowStage::AwaitingLayerxPayment,
            WorkflowStage::LayerxSubmissionPlanned,
            evidence.clone(),
            WORKER,
            1_003,
        )
        .required("plan transaction");
    journal
        .transition(
            digest,
            WorkflowStage::LayerxSubmissionPlanned,
            WorkflowStage::LayerxPending,
            TransitionEvidence::empty(),
            WORKER,
            1_004,
        )
        .required("submitted remains pending");
    assert_eq!(
        journal
            .order(&digest)
            .required("submitted order")
            .evidence
            .rung,
        Some(Rung::Submitted)
    );
    evidence.receipt_digest = Some(receipt_digest);
    evidence.receipt_height = Some(42);
    assert_uncommitted_receipt_rejected(&mut journal, digest, &evidence);
    evidence.rung = Some(Rung::Committed);
    journal
        .transition(
            digest,
            WorkflowStage::LayerxPending,
            WorkflowStage::LayerxVerified,
            evidence.clone(),
            WORKER,
            1_005,
        )
        .required("record committed receipt");
    let expected = journal.order(&digest).required("verified order").clone();
    drop(journal);

    assert_record_hashes(&path);
    assert_replay_and_rejection(
        &path,
        digest,
        &expected,
        tx_hash,
        &raw_transaction,
        receipt_digest,
    );
}

#[derive(Clone, Default, Serialize)]
struct VersionOneEvidence {
    provider_operation_id: Option<String>,
    provider_evidence_digest: Option<[u8; 32]>,
    activity_id: Option<[u8; 32]>,
    canonical_activity: Option<Vec<u8>>,
    receipt_digest: Option<[u8; 32]>,
    refusal_code: Option<String>,
    retry_at: Option<u64>,
}

#[derive(Serialize)]
#[serde(rename_all = "snake_case")]
enum VersionOneEvent {
    OrderCreated {
        order: Box<RampOrder>,
    },
    Transition {
        order_digest: [u8; 32],
        expected: WorkflowStage,
        next: WorkflowStage,
        evidence: Box<VersionOneEvidence>,
    },
}

#[derive(Serialize)]
struct VersionOneBody {
    sequence: u64,
    previous_hash: [u8; 32],
    recorded_at: u64,
    event: VersionOneEvent,
}

fn append_version_one(
    path: &PathBuf,
    sequence: u64,
    previous_hash: [u8; 32],
    event: VersionOneEvent,
) -> [u8; 32] {
    let body = VersionOneBody {
        sequence,
        previous_hash,
        recorded_at: 1_001 + sequence,
        event,
    };
    let mut bytes = serde_json::to_vec(&body).required("encode original v1 body");
    let mut hasher = Sha256::new();
    hasher.update(b"LXP/market-maker-ramp/journal/v1\0");
    hasher.update(&bytes);
    let hash: [u8; 32] = hasher.finalize().into();
    assert_eq!(bytes.pop(), Some(b'}'));
    bytes.extend_from_slice(b",\"record_hash\":");
    bytes.extend_from_slice(&serde_json::to_vec(&hash).required("encode v1 hash"));
    bytes.extend_from_slice(b"}\n");
    let mut options = OpenOptions::new();
    options.create(true).append(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt as _;
        options.mode(0o600);
    }
    let mut file = options.open(path).required("open v1 fixture");
    file.write_all(&bytes).required("write original v1 record");
    file.sync_all().required("persist v1 fixture");
    hash
}

fn append_and_replay_transitions(
    path: &PathBuf,
    digest: [u8; 32],
    mut hash: [u8; 32],
    transitions: [(WorkflowStage, WorkflowStage, VersionOneEvidence); 6],
) {
    for (index, (expected, next, evidence)) in transitions.into_iter().enumerate() {
        hash = append_version_one(
            path,
            index as u64 + 1,
            hash,
            VersionOneEvent::Transition {
                order_digest: digest,
                expected,
                next,
                evidence: Box::new(evidence),
            },
        );
        let journal = Journal::open(path).required("replay original v1 transition");
        assert_eq!(journal.order(&digest).required("v1 order").stage, next);
    }
}

#[test]
fn journal_replays_version_one_records() {
    let directory = JournalDirectory::new();
    let path = directory.path();
    let order = order(RampDirection::OnRamp);
    let digest = order.order_digest;
    let hash = append_version_one(
        &path,
        0,
        [0; 32],
        VersionOneEvent::OrderCreated {
            order: Box::new(order),
        },
    );
    let journal = Journal::open(&path).required("replay v1 created order");
    assert_eq!(
        journal.order(&digest).required("created order").stage,
        WorkflowStage::CompliancePending
    );
    drop(journal);
    let empty = VersionOneEvidence::default();
    let provider = VersionOneEvidence {
        provider_operation_id: Some("provider-1".into()),
        provider_evidence_digest: Some([6; 32]),
        ..VersionOneEvidence::default()
    };
    let payment = VersionOneEvidence {
        activity_id: Some([3; 32]),
        canonical_activity: Some(vec![1, 2, 3]),
        ..VersionOneEvidence::default()
    };
    let receipt = VersionOneEvidence {
        receipt_digest: Some([4; 32]),
        ..VersionOneEvidence::default()
    };
    let transitions = [
        (
            WorkflowStage::CompliancePending,
            WorkflowStage::AwaitingExternalCredit,
            empty.clone(),
        ),
        (
            WorkflowStage::AwaitingExternalCredit,
            WorkflowStage::ProviderSettled,
            provider,
        ),
        (
            WorkflowStage::ProviderSettled,
            WorkflowStage::LayerxSubmissionPlanned,
            payment,
        ),
        (
            WorkflowStage::LayerxSubmissionPlanned,
            WorkflowStage::LayerxPending,
            empty.clone(),
        ),
        (
            WorkflowStage::LayerxPending,
            WorkflowStage::LayerxVerified,
            receipt,
        ),
        (WorkflowStage::LayerxVerified, WorkflowStage::Done, empty),
    ];
    append_and_replay_transitions(&path, digest, hash, transitions);
    let original = fs::read(&path).required("original v1 bytes");
    let mut journal = Journal::open(&path).required("open completed v1 journal");
    let snapshot = journal.order(&digest).required("v1 completed order");
    assert_eq!(snapshot.evidence.activity_id, Some([3; 32]));
    assert_eq!(
        snapshot.evidence.canonical_activity.as_deref(),
        Some([1, 2, 3].as_slice())
    );
    assert_eq!(snapshot.evidence.receipt_digest, Some([4; 32]));
    assert_eq!(snapshot.evidence.tx_hash, None);
    assert_eq!(snapshot.evidence.raw_transaction, None);
    assert_eq!(snapshot.evidence.receipt_height, None);
    assert_eq!(snapshot.evidence.rung, None);
    assert!(!journal.health().ready);
    assert!(matches!(
        journal.recover_verified(|_| Ok(())),
        Err(RampError::Journal)
    ));
    assert!(matches!(
        journal.acquire_lease(digest, WORKER, 1_010, 30),
        Err(RampError::Journal)
    ));
    assert_eq!(
        fs::read(&path).required("v1 journal after write attempts"),
        original
    );
}
