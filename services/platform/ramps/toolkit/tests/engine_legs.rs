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
        fs::create_dir(&path).expect("create journal directory");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt as _;
            fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
                .expect("protect journal directory");
        }
        Self(path)
    }

    fn path(&self) -> PathBuf {
        self.0.join("journal.jsonl")
    }
}

impl Drop for JournalDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove journal directory");
    }
}

fn order(direction: RampDirection) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: "order-1".into(),
            quote_id: "quote-1".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([7; 32]),
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
    .expect("bind order")
}

#[test]
fn journal_records_transaction_hash_and_rung() {
    let directory = JournalDirectory::new();
    let path = directory.path();
    let order = order(RampDirection::OffRamp);
    let digest = order.order_digest;
    let payment = compile_payer_grant_draw(&order, 9).expect("compile ledger draw");
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
    .expect("sign ledger transaction");
    let tx_hash = transaction_hash(&raw_transaction);
    let receipt_digest: [u8; 32] = Sha256::digest(b"journal receipt evidence").into();
    let mut journal = Journal::open(&path).expect("open journal");
    journal.create_order(order, 1_001).expect("create order");
    journal
        .acquire_lease(digest, WORKER, 1_001, 30)
        .expect("lease");
    journal
        .transition(
            digest,
            WorkflowStage::CompliancePending,
            WorkflowStage::AwaitingLayerxPayment,
            TransitionEvidence::empty(),
            WORKER,
            1_002,
        )
        .expect("approve payment");
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
        .expect("plan transaction");
    journal
        .transition(
            digest,
            WorkflowStage::LayerxSubmissionPlanned,
            WorkflowStage::LayerxPending,
            TransitionEvidence::empty(),
            WORKER,
            1_004,
        )
        .expect("submitted remains pending");
    assert_eq!(
        journal
            .order(&digest)
            .expect("submitted order")
            .evidence
            .rung,
        Some(Rung::Submitted)
    );
    evidence.receipt_digest = Some(receipt_digest);
    evidence.receipt_height = Some(42);
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
        journal.order(&digest).expect("pending order").stage,
        WorkflowStage::LayerxPending
    );
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
        .expect("record committed receipt");
    let expected = journal.order(&digest).expect("verified order").clone();
    drop(journal);

    let bytes = fs::read(&path).expect("journal bytes");
    let mut previous_hash = [0; 32];
    for line in std::str::from_utf8(&bytes).expect("journal UTF-8").lines() {
        let record: serde_json::Value = serde_json::from_str(line).expect("record JSON");
        assert_eq!(record["version"], 2);
        assert_eq!(record["previous_hash"], serde_json::json!(previous_hash));
        let (body, _) = line
            .rsplit_once(",\"record_hash\":")
            .expect("record hash field");
        let body = format!("{body}}}");
        let mut hasher = Sha256::new();
        hasher.update(b"LXP/market-maker-ramp/journal/v2\0");
        hasher.update(body.as_bytes());
        previous_hash = hasher.finalize().into();
        assert_eq!(record["record_hash"], serde_json::json!(previous_hash));
    }
    let mut replay = Journal::open(&path).expect("reopen journal");
    let snapshot = replay.order(&digest).expect("replayed order");
    assert_eq!(snapshot, &expected);
    assert_eq!(snapshot.evidence.tx_hash, Some(tx_hash));
    assert_eq!(
        snapshot.evidence.raw_transaction.as_deref(),
        Some(raw_transaction.as_slice())
    );
    assert_eq!(snapshot.evidence.receipt_digest, Some(receipt_digest));
    assert_eq!(snapshot.evidence.receipt_height, Some(42));
    assert_eq!(snapshot.evidence.rung, Some(Rung::Committed));
    assert_eq!(snapshot.presentation().activity_id, Some(tx_hash));
    replay
        .recover_verified(|_| Ok(()))
        .expect("recover v2 writer");
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
        .expect("settle provider");
    replay
        .transition(
            digest,
            WorkflowStage::ProviderSettled,
            WorkflowStage::Done,
            TransitionEvidence::empty(),
            WORKER,
            1_007,
        )
        .expect("complete with transaction evidence");
    drop(replay);
    let completed = Journal::open(&path).expect("replay completed v2 journal");
    assert_eq!(
        completed.order(&digest).expect("completed order").stage,
        WorkflowStage::Done
    );
    drop(completed);

    let text = fs::read_to_string(&path).expect("read completed journal");
    let tampered = text.replace("\"rung\":\"committed\"", "\"rung\":\"receipted\"");
    assert_ne!(text, tampered);
    fs::write(&path, tampered).expect("alter recorded rung");
    assert!(matches!(Journal::open(&path), Err(RampError::Journal)));
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
        evidence: VersionOneEvidence,
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
    let mut bytes = serde_json::to_vec(&body).expect("encode original v1 body");
    let mut hasher = Sha256::new();
    hasher.update(b"LXP/market-maker-ramp/journal/v1\0");
    hasher.update(&bytes);
    let hash: [u8; 32] = hasher.finalize().into();
    assert_eq!(bytes.pop(), Some(b'}'));
    bytes.extend_from_slice(b",\"record_hash\":");
    bytes.extend_from_slice(&serde_json::to_vec(&hash).expect("encode v1 hash"));
    bytes.extend_from_slice(b"}\n");
    let mut options = OpenOptions::new();
    options.create(true).append(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt as _;
        options.mode(0o600);
    }
    let mut file = options.open(path).expect("open v1 fixture");
    file.write_all(&bytes).expect("write original v1 record");
    file.sync_all().expect("persist v1 fixture");
    hash
}

#[test]
fn journal_replays_version_one_records() {
    let directory = JournalDirectory::new();
    let path = directory.path();
    let order = order(RampDirection::OnRamp);
    let digest = order.order_digest;
    let mut hash = append_version_one(
        &path,
        0,
        [0; 32],
        VersionOneEvent::OrderCreated {
            order: Box::new(order),
        },
    );
    let journal = Journal::open(&path).expect("replay v1 created order");
    assert_eq!(
        journal.order(&digest).expect("created order").stage,
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
    for (index, (expected, next, evidence)) in transitions.into_iter().enumerate() {
        hash = append_version_one(
            &path,
            index as u64 + 1,
            hash,
            VersionOneEvent::Transition {
                order_digest: digest,
                expected,
                next,
                evidence,
            },
        );
        let journal = Journal::open(&path).expect("replay original v1 transition");
        assert_eq!(journal.order(&digest).expect("v1 order").stage, next);
    }
    let original = fs::read(&path).expect("original v1 bytes");
    let mut journal = Journal::open(&path).expect("open completed v1 journal");
    let snapshot = journal.order(&digest).expect("v1 completed order");
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
        fs::read(&path).expect("v1 journal after write attempts"),
        original
    );
}
