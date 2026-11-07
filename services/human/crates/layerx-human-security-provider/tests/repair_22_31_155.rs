mod support;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_human_security_provider::{
    ingest_recovery_receipt, serve, Config, RecoveryReceipt, RecoveryTransactionEvidence, Store,
};
use layerx_human_service::{
    security::RecoveryEvidenceProvider,
    server::production_auth::{RemoteSecurityProvider, SecurityProviderConfig},
    store::PrincipalId,
};
use layerx_programs::chain_proof::{
    header::ChainHeader,
    trust_store::TrustRoot,
    validators::{TrustedValidator, ValidatorSet},
};
use serde_json::Value;
use sha2::{Digest as _, Sha256};
use std::{
    fs::{self, OpenOptions},
    io::Write as _,
    os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _},
    path::{Path, PathBuf},
    process::Command,
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc, OnceLock,
    },
    time::{Duration, Instant},
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

fn directory() -> PathBuf {
    let root = std::env::temp_dir().join(format!(
        "lx-recovery-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ));
    fs::DirBuilder::new().mode(0o700).create(&root).unwrap();
    root
}

fn node_evidence() -> &'static Value {
    static EVIDENCE: OnceLock<Value> = OnceLock::new();
    EVIDENCE.get_or_init(|| {
        let root = directory();
        let output = root.join("node-evidence.json");
        let repo = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
        // The real merged node links the kernel and sandbox through lxphost's CGO bridge.
        // Build those dependencies before asking its receipt producer to execute recovery.
        let kernel = Command::new("make")
            .current_dir(&repo)
            .args(["-j2", "build/liblayerx.a"])
            .output()
            .unwrap();
        assert!(
            kernel.status.success(),
            "native kernel build failed ({}):\n{}\n{}",
            kernel.status,
            String::from_utf8_lossy(&kernel.stdout),
            String::from_utf8_lossy(&kernel.stderr)
        );
        let sandbox = Command::new("cargo")
            .current_dir(repo.join("programs"))
            .env("CARGO_TARGET_DIR", repo.join("programs/target"))
            .args([
                "build",
                "--locked",
                "--package",
                "layerx-programs-sandbox",
                "--features",
                "host-ffi",
                "--lib",
            ])
            .output()
            .unwrap();
        assert!(
            sandbox.status.success(),
            "native sandbox build failed ({}):\n{}\n{}",
            sandbox.status,
            String::from_utf8_lossy(&sandbox.stdout),
            String::from_utf8_lossy(&sandbox.stderr)
        );
        let produced = Command::new("go")
            .current_dir(repo)
            .args([
                "test",
                "-mod=readonly",
                "./services/human/crates/layerx-human-security-provider/src",
                "-run",
                "^TestRecoveryEvidenceProducer$",
                "-count=1",
            ])
            .env("LAYERX_RECOVERY_EVIDENCE_OUTPUT", &output)
            .output()
            .unwrap();
        assert!(
            produced.status.success(),
            "real merged node evidence producer failed ({}):\n{}\n{}",
            produced.status,
            String::from_utf8_lossy(&produced.stdout),
            String::from_utf8_lossy(&produced.stderr)
        );
        let value = serde_json::from_slice(&fs::read(output).unwrap()).unwrap();
        fs::remove_dir_all(root).unwrap();
        value
    })
}

fn evidence(index: usize) -> (RecoveryReceipt, RecoveryTransactionEvidence) {
    let case = &node_evidence()["cases"][index];
    (
        serde_json::from_value(case["receipt"].clone()).unwrap(),
        serde_json::from_value(case["inclusion"].clone()).unwrap(),
    )
}

struct Fixture {
    root: PathBuf,
}
impl Fixture {
    fn new() -> Self {
        let root = directory();
        let (receipt, _) = evidence(1);
        let header = ChainHeader::decode(&STANDARD.decode(&receipt.header).unwrap()).unwrap();
        let public: [u8; 32] = STANDARD
            .decode(node_evidence()["public_key"].as_str().unwrap())
            .unwrap()
            .try_into()
            .unwrap();
        let trust = TrustRoot {
            chain_id: header.chain_id.clone(),
            height: header.height as u64,
            time_ms: (header.time.seconds * 1000 + i64::from(header.time.nanos / 1_000_000)) as u64,
            header_hash: header.hash().unwrap(),
            validators: ValidatorSet::new(vec![TrustedValidator::new(public, 100)]).unwrap(),
        };
        private(&root.join("trust"), &trust.canonical_bytes().unwrap());
        Self { root }
    }
    fn state(&self) -> PathBuf {
        self.root.join("state")
    }
    fn trust(&self) -> PathBuf {
        self.root.join("trust")
    }
    fn client(&self) -> RemoteSecurityProvider {
        RemoteSecurityProvider::new(SecurityProviderConfig {
            socket: self.root.join("socket"),
            deadline: Duration::from_secs(3),
            maximum_frame_bytes: 1_048_576,
        })
        .unwrap()
    }
    fn ingest(&self, receipt: &RecoveryReceipt) -> layerx_human_security_provider::Result<()> {
        let path = self
            .root
            .join(format!("receipt-{}", NEXT.fetch_add(1, Ordering::Relaxed)));
        private(&path, &serde_json::to_vec(receipt).unwrap());
        ingest_recovery_receipt(&self.state(), &self.trust(), &path)
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
        let deadline = Instant::now() + Duration::from_secs(5);
        while self.client().probe().is_err() {
            assert!(!thread.is_finished(), "provider stopped before ready");
            assert!(
                Instant::now() < deadline,
                "provider startup deadline exceeded"
            );
            std::thread::sleep(Duration::from_millis(10));
        }
        Running {
            stop,
            thread: Some(thread),
        }
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

#[test]
fn node_recovery_receipt_ingest_reveal_and_reopen() {
    let fixture = Fixture::new();
    let (receipt, inclusion) = evidence(1);
    let receipt = receipt.with_transaction_evidence(&inclusion).unwrap();
    assert!(STANDARD
        .decode(&inclusion.containing_transaction)
        .unwrap()
        .starts_with(b"PAXEER/v1/sequence-batch\0"));
    assert_ne!(
        STANDARD.decode(&inclusion.containing_transaction).unwrap(),
        STANDARD.decode(&receipt.canonical_receipt).unwrap()
    );
    assert_eq!(inclusion.receipt_index, 1);
    fixture.ingest(&receipt).unwrap();
    fixture.ingest(&receipt).unwrap();
    let authority = support::ClockAuthority::start(&fixture.root);
    let alice = PrincipalId::new("alice").unwrap();
    for now in [100, 101] {
        let server = fixture.start(&authority);
        let revealed = fixture
            .client()
            .reveal_verified_receipt(&alice, "recovery-1", now)
            .unwrap();
        assert_eq!(revealed.expose(), receipt.canonical_receipt);
        assert_eq!(revealed.remask_at(), now + 60);
        assert!(revealed.copyable());
        assert!(fixture
            .client()
            .reveal_verified_receipt(&PrincipalId::new("bob").unwrap(), "recovery-1", now)
            .is_err());
        drop(server);
        assert!(!fixture.root.join("socket").exists());
        drop(Store::open(&fixture.state(), &fixture.trust()).unwrap());
    }
}

#[test]
fn node_recovery_evidence_rejects_wrong_transactions_outcomes_and_receipts() {
    let fixture = Fixture::new();
    for index in [0, 2] {
        let (receipt, inclusion) = evidence(index);
        assert!(fixture
            .ingest(&receipt.with_transaction_evidence(&inclusion).unwrap())
            .is_err());
    }
    let (original, inclusion) = evidence(1);
    assert!(
        fixture.ingest(&original).is_err(),
        "receipt-as-transaction v1 is retired"
    );
    for change in 0..13 {
        let mut candidate = inclusion.clone();
        let mut receipt = original.clone();
        match change {
            0 => candidate.containing_transaction = receipt.canonical_receipt.clone(),
            1 => candidate.receipt_index = 0,
            2 => candidate.receipt_index = u32::MAX,
            3 => candidate.activity_ids[1][0] ^= 1,
            4 => candidate.activity_ids.swap(0, 1),
            5 => candidate.raw_transaction = evidence(0).1.raw_transaction,
            6 => candidate.result_proof = evidence(0).1.result_proof,
            7 => candidate.result_data = evidence(2).1.result_data,
            8 => candidate.recovery_did[0] ^= 1,
            9 => candidate.recovery_root[0] ^= 1,
            10 => candidate.recovery_threshold += 1,
            11 => receipt.canonical_receipt = evidence(0).0.canonical_receipt,
            _ => {
                let mut bytes = STANDARD.decode(&receipt.canonical_receipt).unwrap();
                *bytes.last_mut().unwrap() ^= 1;
                receipt.canonical_receipt = STANDARD.encode(bytes);
            }
        }
        assert!(
            fixture
                .ingest(&receipt.with_transaction_evidence(&candidate).unwrap())
                .is_err(),
            "mutation {change} accepted"
        );
    }
    assert!(
        !fixture.state().exists(),
        "refused evidence must not initialize the store"
    );
}

#[test]
fn stored_node_recovery_evidence_is_reverified_at_reveal_and_replay() {
    let fixture = Fixture::new();
    let (receipt, inclusion) = evidence(1);
    fixture
        .ingest(&receipt.with_transaction_evidence(&inclusion).unwrap())
        .unwrap();
    let authority = support::ClockAuthority::start(&fixture.root);
    let server = fixture.start(&authority);
    let path = fixture.state().join("00000000000000000001.json");
    let mut record: Value = serde_json::from_slice(&fs::read(&path).unwrap()).unwrap();
    let stored = &mut record["state"]["receipts"]["alice"]["recovery-1"];
    let mut forged: RecoveryReceipt = serde_json::from_value(stored.clone()).unwrap();
    let mut binding = forged.transaction_evidence().unwrap();
    binding.receipt_index = 0;
    forged = forged.with_transaction_evidence(&binding).unwrap();
    *stored = serde_json::to_value(forged).unwrap();
    // Re-encode through the real record's typed serialization order, as the store does.
    #[derive(serde::Serialize, serde::Deserialize)]
    struct Record {
        sequence: u64,
        previous: [u8; 32],
        state: State,
    }
    #[derive(serde::Serialize, serde::Deserialize)]
    struct State {
        accounts: std::collections::BTreeMap<String, Value>,
        receipts:
            std::collections::BTreeMap<String, std::collections::BTreeMap<String, RecoveryReceipt>>,
    }
    let record: Record = serde_json::from_value(record).unwrap();
    let bytes = serde_json::to_vec(&record).unwrap();
    fs::write(&path, &bytes).unwrap();
    let mut head = 1u64.to_be_bytes().to_vec();
    head.extend_from_slice(&Sha256::digest(bytes));
    fs::write(fixture.state().join("head"), head).unwrap();
    assert!(fixture
        .client()
        .reveal_verified_receipt(&PrincipalId::new("alice").unwrap(), "recovery-1", 100)
        .is_err());
    drop(server);
    assert!(Store::open(&fixture.state(), &fixture.trust()).is_err());
}
