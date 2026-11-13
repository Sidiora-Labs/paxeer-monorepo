#[allow(dead_code)]
mod support;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_human_security_provider::{
    ingest_recovery_receipt, serve, Config, RecoveryReceipt, Store,
};
use layerx_human_service::{
    security::RecoveryEvidenceProvider,
    server::production_auth::{RemoteSecurityProvider, SecurityProviderConfig},
    store::PrincipalId,
};
use layerx_intents::canonical::decode_receipt;
use layerx_programs::chain_proof::header::ChainHeader;
use serde_json::Value;
use sha2::{Digest as _, Sha256};
use std::{
    collections::BTreeMap,
    fs::{self, OpenOptions},
    io::Write as _,
    os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _},
    path::{Path, PathBuf},
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc,
    },
    time::Duration,
};

static NEXT: AtomicU64 = AtomicU64::new(0);

fn private(path: &Path, bytes: &[u8]) {
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(path)
        .unwrap();
    file.write_all(bytes).unwrap();
    file.sync_all().unwrap();
}

struct Fixture {
    root: PathBuf,
}
impl Fixture {
    fn new(trust: &[u8]) -> Self {
        let root = std::env::temp_dir().join(format!(
            "lx-22-33-47-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&root).unwrap();
        private(&root.join("trust"), trust);
        Self { root }
    }
    fn state(&self) -> PathBuf {
        self.root.join("state")
    }
    fn trust(&self) -> PathBuf {
        self.root.join("trust")
    }
    fn ingest(&self, receipt: &RecoveryReceipt) -> layerx_human_security_provider::Result<()> {
        let path = self
            .root
            .join(format!("receipt-{}", NEXT.fetch_add(1, Ordering::Relaxed)));
        private(&path, &serde_json::to_vec(receipt).unwrap());
        ingest_recovery_receipt(&self.state(), &self.trust(), &path)
    }
    fn client(&self) -> RemoteSecurityProvider {
        RemoteSecurityProvider::new(SecurityProviderConfig {
            socket: self.root.join("socket"),
            deadline: Duration::from_secs(3),
            maximum_frame_bytes: 1_048_576,
        })
        .unwrap()
    }
    fn start(&self, authority: &support::ClockAuthority) -> Running {
        let config = Config {
            state_root: self.state(),
            trust_history: self.trust(),
            socket: self.root.join("socket"),
            allowed_uid: rustix::process::geteuid().as_raw(),
            deadline: Duration::from_secs(1),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = stop.clone();
        let clock = authority.clock.clone();
        let thread = std::thread::spawn(move || serve(config, shutdown, clock));
        let mut startup_deadline =
            layerx_types::clock::Deadline::start(authority.clock.as_ref(), Duration::from_secs(5))
                .unwrap_or_else(|error| panic!("startup clock: {error}"));
        while self.client().probe().is_err() {
            assert!(!thread.is_finished(), "provider stopped before ready");
            assert!(
                !startup_deadline
                    .remaining(authority.clock.as_ref())
                    .unwrap_or_else(|error| panic!("startup clock: {error}"))
                    .is_zero(),
                "provider startup deadline exceeded"
            );
            std::thread::sleep(Duration::from_millis(10));
        }
        Running {
            stop,
            thread: Some(thread),
        }
    }
    fn replace_journal_receipt(&self, replacement: &RecoveryReceipt) {
        // Preserve the real journal's canonical field order and head digest so
        // refusal proves semantic revalidation rather than JSON/hash corruption.
        #[derive(serde::Serialize, serde::Deserialize)]
        struct Record {
            sequence: u64,
            previous: [u8; 32],
            state: State,
        }
        #[derive(serde::Serialize, serde::Deserialize)]
        struct State {
            accounts: BTreeMap<String, Value>,
            receipts: BTreeMap<String, BTreeMap<String, RecoveryReceipt>>,
        }
        let path = self.state().join("00000000000000000001.json");
        let mut record: Record = serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();
        *record
            .state
            .receipts
            .get_mut(&replacement.principal)
            .unwrap()
            .get_mut(&replacement.evidence_id)
            .unwrap() = replacement.clone();
        let bytes = serde_json::to_vec(&record).unwrap();
        fs::write(path, &bytes).unwrap();
        let mut head = 1u64.to_be_bytes().to_vec();
        head.extend_from_slice(&Sha256::digest(&bytes));
        fs::write(self.state().join("head"), head).unwrap();
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap();
    }
}
struct Running {
    stop: Arc<AtomicBool>,
    thread: Option<std::thread::JoinHandle<layerx_human_security_provider::Result<()>>>,
}
impl Drop for Running {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Relaxed);
        self.thread.take().unwrap().join().unwrap().unwrap();
    }
}

fn refusals() -> Vec<RecoveryReceipt> {
    let (_, good) = support::evidence();
    let (_, failed) = support::recorded_evidence(2);
    let (_, other) = support::recorded_evidence(0);
    let mut altered = good.transaction_evidence().unwrap();
    let mut raw = STANDARD.decode(&altered.raw_transaction).unwrap();
    *raw.last_mut().unwrap() ^= 1;
    altered.raw_transaction = STANDARD.encode(raw);
    let altered = good.clone().with_transaction_evidence(&altered).unwrap();
    let mut altered_batch = good.transaction_evidence().unwrap();
    let mut batch = STANDARD
        .decode(&altered_batch.containing_transaction)
        .unwrap();
    *batch.last_mut().unwrap() ^= 1;
    altered_batch.containing_transaction = STANDARD.encode(batch);
    let altered_batch = good
        .clone()
        .with_transaction_evidence(&altered_batch)
        .unwrap();
    let mut mismatch = good.clone();
    mismatch.canonical_receipt = other.canonical_receipt.clone();
    let mut wrong_root = good.transaction_evidence().unwrap();
    wrong_root.recovery_root[0] ^= 1;
    let wrong_root = good.clone().with_transaction_evidence(&wrong_root).unwrap();
    let mut failed_result = good.transaction_evidence().unwrap();
    failed_result.result_code = 1;
    let failed_result = good
        .clone()
        .with_transaction_evidence(&failed_result)
        .unwrap();
    vec![
        failed,
        other,
        altered,
        altered_batch,
        mismatch,
        wrong_root,
        failed_result,
    ]
}

#[test]
fn repair_22_33_47_committed_recovery_ingest_reveal_restart_and_replay() {
    let (trust, receipt) = support::evidence();
    let inclusion = receipt.transaction_evidence().unwrap();
    let transaction = STANDARD.decode(&inclusion.containing_transaction).unwrap();
    let canonical = STANDARD.decode(&receipt.canonical_receipt).unwrap();
    assert!(transaction.starts_with(b"PAXEER/v1/sequence-batch\0"));
    assert_ne!(transaction, canonical);
    let decoded = decode_receipt(&canonical).unwrap();
    let protocol = decoded.protocol().unwrap();
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), 2);
    assert_eq!(inclusion.result_code, 0);
    let execution =
        ChainHeader::decode(&STANDARD.decode(&inclusion.execution_header).unwrap()).unwrap();
    let successor = ChainHeader::decode(&STANDARD.decode(&receipt.header).unwrap()).unwrap();
    assert_eq!(successor.height, execution.height + 1);
    assert_eq!(successor.last_block_id.hash, execution.hash().unwrap());
    let keys = layerx_programs::chain_proof::decode::decode_commitment_proof(
        &STANDARD.decode(&inclusion.receipt_keys_proof).unwrap(),
    )
    .unwrap();
    assert_eq!(
        keys.signed_header,
        STANDARD.decode(&receipt.header).unwrap()
    );
    assert_eq!(keys.store_name, "sequencer");
    assert_eq!(keys.key, [3]);
    let fixture = Fixture::new(&trust);
    // Exercise the actual administrative command, decoder and persistent store.
    private(
        &fixture.root.join("receipt.json"),
        &serde_json::to_vec(&receipt).unwrap(),
    );
    let output = std::process::Command::new(env!("CARGO_BIN_EXE_layerx-human-security-provider"))
        .arg("ingest-recovery-receipt")
        .arg(fixture.root.join("receipt.json"))
        .env("LAYERX_HUMAN_SECURITY_PROVIDER_STATE_ROOT", fixture.state())
        .env(
            "LAYERX_HUMAN_SECURITY_PROVIDER_TRUST_HISTORY",
            fixture.trust(),
        )
        .output()
        .unwrap();
    assert!(
        output.status.success(),
        "administration refused committed evidence: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    fixture.ingest(&receipt).unwrap();
    let authority = support::ClockAuthority::start(&fixture.root);
    let principal = PrincipalId::new(receipt.principal.clone()).unwrap();
    let journal = fs::read(fixture.state().join("00000000000000000001.json")).unwrap();
    for now in [100, 101] {
        let server = fixture.start(&authority);
        let revealed = fixture
            .client()
            .reveal_verified_receipt(&principal, &receipt.evidence_id, now)
            .unwrap();
        assert_eq!(revealed.expose(), receipt.canonical_receipt);
        assert_eq!(revealed.remask_at(), now + 60);
        assert!(revealed.copyable());
        assert!(fixture
            .client()
            .reveal_verified_receipt(&PrincipalId::new("bob").unwrap(), &receipt.evidence_id, now)
            .is_err());
        drop(server);
        assert!(!fixture.root.join("socket").exists());
        drop(Store::open(&fixture.state(), &fixture.trust()).unwrap());
        assert_eq!(
            fs::read(fixture.state().join("00000000000000000001.json")).unwrap(),
            journal
        );
    }
}

#[test]
fn repair_22_33_47_altered_transactions_failed_outcomes_and_mismatched_receipts_refuse_ingest() {
    let (trust, _) = support::evidence();
    for (index, receipt) in refusals().iter().enumerate() {
        let fixture = Fixture::new(&trust);
        assert!(
            fixture.ingest(receipt).is_err(),
            "accepted refusal case {index}"
        );
        assert!(!fixture.state().exists());
    }
}

#[test]
fn repair_22_33_47_invalid_recordings_refuse_reveal_and_reopen() {
    let (trust, good) = support::evidence();
    for (index, bad) in refusals().iter().enumerate() {
        let fixture = Fixture::new(&trust);
        fixture.ingest(&good).unwrap();
        let authority = support::ClockAuthority::start(&fixture.root);
        let server = fixture.start(&authority);
        let principal = PrincipalId::new(good.principal.clone()).unwrap();
        assert!(fixture
            .client()
            .reveal_verified_receipt(&principal, &good.evidence_id, 100)
            .is_ok());
        fixture.replace_journal_receipt(bad);
        assert!(
            fixture
                .client()
                .reveal_verified_receipt(&principal, &good.evidence_id, 101)
                .is_err(),
            "revealed refusal case {index}"
        );
        drop(server);
        assert!(
            Store::open(&fixture.state(), &fixture.trust()).is_err(),
            "replayed refusal case {index}"
        );
    }
}
