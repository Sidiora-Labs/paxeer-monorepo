#[path = "support/chain.rs"]
mod chain;
mod support;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::ai_market::OperationRecord;
use layerx_human_security_provider::{
    ingest_recovery_receipt, serve, Config, RecoveryReceipt, RecoveryTransactionEvidence, Store,
};
use layerx_human_service::{
    security::RecoveryEvidenceProvider,
    server::{
        agent_runtime::TransactionType2,
        production_auth::{RemoteSecurityProvider, SecurityProviderConfig},
    },
    store::PrincipalId,
};
use layerx_intents::canonical::{decode_receipt, receipt_bytes, receipt_digest};
use layerx_programs::chain_proof::{
    header::ChainHeader,
    merkle::{hash_from_byte_slices, SimpleProof},
    results::{verify_tx_result, TxResult},
    trust_store::TrustRoot,
    vote::vote_sign_bytes,
    Commit,
};
use layerx_proof::{inclusion::SequencerAuthorization, receipt::verify_sequencer_signature};
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
    time::{Duration, Instant},
};

static NEXT: AtomicU64 = AtomicU64::new(0);
const DID: [u8; 32] = [9; 32];
const ROOT: [u8; 32] = [7; 32];
const VALIDATOR: [u8; 32] = [38; 32];
const RECEIPTER: [u8; 32] = [37; 32];
const HEIGHT: u64 = 37;

#[derive(Clone, Copy, Debug)]
enum Case {
    Recovery,
    ReceiptFailure,
    CommittedFailure,
    OtherOperation,
    OtherModule,
    OtherLifecycle,
    OtherTarget,
    WrongDid,
    WrongRoot,
    WrongThreshold,
    EmptyDid,
    EmptyRoot,
    EmptyThreshold,
}
const REFUSALS: &[Case] = &[
    Case::ReceiptFailure,
    Case::CommittedFailure,
    Case::OtherOperation,
    Case::OtherModule,
    Case::OtherLifecycle,
    Case::OtherTarget,
    Case::WrongDid,
    Case::WrongRoot,
    Case::WrongThreshold,
    Case::EmptyDid,
    Case::EmptyRoot,
    Case::EmptyThreshold,
];

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
fn field(out: &mut Vec<u8>, value: &[u8]) {
    out.extend_from_slice(&(value.len() as u32).to_be_bytes());
    out.extend_from_slice(value);
}
fn varint(out: &mut Vec<u8>, mut value: u64) {
    while value >= 128 {
        out.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    out.push(value as u8);
}
fn pb_bytes(out: &mut Vec<u8>, tag: u8, bytes: &[u8]) {
    out.push(tag << 3 | 2);
    varint(out, bytes.len() as u64);
    out.extend_from_slice(bytes);
}
fn proof(index: i64, leaf: &[u8], aunt: &[u8]) -> SimpleProof {
    SimpleProof {
        total: 2,
        index,
        leaf_hash: hash_from_byte_slices(&[leaf]),
        aunts: vec![hash_from_byte_slices(&[aunt])],
    }
}
fn proof_bytes(proof: &SimpleProof) -> String {
    let mut out = proof.total.to_be_bytes().to_vec();
    out.extend_from_slice(&proof.index.to_be_bytes());
    out.extend_from_slice(&proof.leaf_hash);
    out.push(proof.aunts.len() as u8);
    for aunt in &proof.aunts {
        out.extend_from_slice(aunt);
    }
    STANDARD.encode(out)
}
fn sign_header(header: &ChainHeader, template: &Commit) -> Commit {
    let mut commit = template.clone();
    commit.height = header.height;
    commit.block_id.hash = header.hash().unwrap().to_vec();
    for signature in &mut commit.signatures {
        signature.timestamp.seconds = header.time.seconds;
        signature.timestamp.nanos = header.time.nanos;
        let message = vote_sign_bytes(
            &header.chain_id,
            commit.height,
            commit.round,
            Some(&commit.block_id),
            signature.timestamp,
        )
        .unwrap();
        signature.signature = SigningKey::from_bytes(&VALIDATOR)
            .sign(&message)
            .to_bytes()
            .to_vec();
    }
    commit
}

struct Transcript {
    receipt: RecoveryReceipt,
    trust: Vec<u8>,
}

// In-process peers use the committed native-policy vector, production EIP-1559
// signer, canonical receipt codec, consensus verifier and authenticated IAVL fixture.
// Every negative case is independently signed and committed, rather than corrupting a proof.
fn transcript(case: Case) -> Transcript {
    let mut payload = layerx_intents::vectors::native_recovery_policy(false, DID).unwrap();
    match case {
        Case::OtherLifecycle => payload[1] = 2,
        Case::EmptyDid => payload[4..36].fill(0),
        Case::EmptyRoot => payload[36..68].fill(0),
        Case::EmptyThreshold => payload[68..70].fill(0),
        _ => {}
    }
    let mut call = vec![0x87, 0xd3, 0xa0, 0x67];
    let mut word = [0; 32];
    word[31] = 64;
    call.extend_from_slice(&word);
    call.extend_from_slice(&[0x52; 32]);
    word[31] = payload.len() as u8;
    call.extend_from_slice(&word);
    call.extend_from_slice(&payload);
    call.resize(196, 0);
    let mut target = [0; 20];
    target[18..].copy_from_slice(&[0x10, 0x1c]);
    if matches!(case, Case::OtherTarget) {
        target[19] = 0x1b;
    }
    let raw = TransactionType2 {
        chain_id: 125,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: target,
        data: call,
    }
    .sign(&[1; 32])
    .unwrap();
    let identity = OperationRecord::from_signed_transaction(raw.clone(), [0; 32], 0).unwrap();
    let activity = identity.activity_id();
    let previous = [0x11; 32];
    let mut sequence = Sha256::new();
    sequence.update(b"PAXEER/v1/sequence\0");
    sequence.update(previous);
    sequence.update(1u64.to_be_bytes());
    sequence.update(activity);
    let resulting: [u8; 32] = sequence.finalize().into();
    let context = [55; 32];
    let mut batch = b"PAXEER/v1/sequence-batch\0".to_vec();
    for number in [HEIGHT, 1, 1] {
        batch.extend_from_slice(&number.to_be_bytes());
    }
    batch.extend_from_slice(&previous);
    batch.extend_from_slice(&resulting);
    batch.extend_from_slice(&Sha256::digest(activity));
    batch.extend_from_slice(&context);
    batch.resize(b"PAXEER/v1/sequence-batch\0".len() + 216, 0);

    let mut output = [0; 32];
    output[31] = 1;
    let mut response = vec![8, 1];
    pb_bytes(&mut response, 3, &output);
    let hash = format!(
        "0x{}",
        activity
            .iter()
            .map(|b| format!("{b:02x}"))
            .collect::<String>()
    );
    pb_bytes(&mut response, 4, hash.as_bytes());
    let mut msg = Vec::new();
    pb_bytes(&mut msg, 1, b"/paxprotocol.paxchain.evm.MsgEVMTransaction");
    pb_bytes(&mut msg, 2, &response);
    let mut result_data = Vec::new();
    pb_bytes(&mut result_data, 1, &msg);
    let result = TxResult {
        code: u32::from(matches!(case, Case::CommittedFailure)),
        data: result_data,
        gas_wanted: 100_000,
        gas_used: 1,
    };
    let mut encoded_result = Vec::new();
    if result.code != 0 {
        encoded_result.extend_from_slice(&[8, result.code as u8]);
    }
    pb_bytes(&mut encoded_result, 2, &result.data);
    encoded_result.push(40);
    varint(&mut encoded_result, result.gas_wanted as u64);
    encoded_result.push(48);
    varint(&mut encoded_result, result.gas_used as u64);

    let module: u16 = if matches!(case, Case::OtherModule) {
        13
    } else {
        12
    };
    let mut summary = Vec::new();
    for number in [1u64, 1, 2] {
        summary.extend_from_slice(&number.to_be_bytes());
    }
    summary.extend_from_slice(&1u32.to_be_bytes());
    summary.extend_from_slice(&[0; 40]);
    summary.extend_from_slice(&Sha256::digest([]));
    summary.extend_from_slice(&32u32.to_be_bytes());
    summary.extend_from_slice(&0u32.to_be_bytes());
    summary.push(4);
    summary.extend_from_slice(b"uhpx");
    let mut effect = module.to_be_bytes().to_vec();
    effect.extend_from_slice(&0u16.to_be_bytes());
    effect.extend_from_slice(&0u16.to_be_bytes());
    effect.extend_from_slice(&[1, 0]);
    field(&mut effect, &[0; 32]);
    field(&mut effect, &summary);
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut bytes = layerx_intents::vectors::security_program_receipt(
        activity,
        batch_id,
        1,
        1_700_000_000_000,
        resulting,
        Sha256::digest(output).into(),
        None,
    );
    bytes[158..162].copy_from_slice(&i32::from(matches!(case, Case::ReceiptFailure)).to_be_bytes());
    bytes[162..166].copy_from_slice(&1u32.to_be_bytes());
    bytes[218..220].copy_from_slice(&module.to_be_bytes());
    bytes[220..224].copy_from_slice(&1u32.to_be_bytes());
    bytes[228] = if matches!(case, Case::OtherOperation) {
        1
    } else {
        2
    };
    bytes[285..317].copy_from_slice(&identity.signer_public_key());
    bytes[361..373].fill(0);
    bytes[373..393].copy_from_slice(&target);
    bytes[501..533].copy_from_slice(&context);
    bytes.splice(166..166, effect);
    let unsigned = receipt_bytes(&decode_receipt(&bytes).unwrap()).unwrap();
    assert_eq!(unsigned, bytes);
    let signature = SigningKey::from_bytes(&RECEIPTER)
        .sign(&receipt_digest(&unsigned).unwrap())
        .to_bytes();
    *bytes.last_mut().unwrap() = 1;
    field(&mut bytes, &signature);
    let canonical = receipt_bytes(&decode_receipt(&bytes).unwrap()).unwrap();
    assert_eq!(canonical, bytes);

    let mut key = vec![55; 32];
    key.extend_from_slice(
        &SigningKey::from_bytes(&RECEIPTER)
            .verifying_key()
            .to_bytes(),
    );
    key.extend_from_slice(&1u64.to_be_bytes());
    key.extend_from_slice(&u64::MAX.to_be_bytes());
    let mut chain = chain::ChainFixture::new_in_store(
        "sequencer",
        "pax-test",
        &[VALIDATOR],
        vec![(vec![3], key)],
    )
    .unwrap();
    let mut execution = chain.header.clone();
    execution.height = HEIGHT as i64;
    let batch_hash = Sha256::digest(&batch);
    let raw_hash = Sha256::digest(&raw);
    execution.data_hash = hash_from_byte_slices(&[&batch_hash, &raw_hash]).to_vec();
    let execution_commit = sign_header(&execution, &chain.commit);
    chain.header.last_block_id.hash = execution.hash().unwrap().to_vec();
    chain.header.last_results_hash = hash_from_byte_slices(&[&[], &encoded_result]).to_vec();
    chain.commit = sign_header(&chain.header, &chain.commit);
    let head = chain
        .verifier
        .verify_head(&chain.header, &chain.commit)
        .unwrap();
    let execution_head = chain
        .verifier
        .verify_head(&execution, &execution_commit)
        .unwrap();
    let tx_proof = proof(0, &batch_hash, &raw_hash);
    layerx_programs::verify_tx(&execution_head, &batch, &tx_proof).unwrap();
    let result_proof = proof(1, &encoded_result, &[]);
    verify_tx_result(&head, &result, &result_proof).unwrap();
    verify_sequencer_signature(
        &canonical,
        &[SequencerAuthorization::new(
            [55; 32],
            SigningKey::from_bytes(&RECEIPTER)
                .verifying_key()
                .to_bytes(),
            1,
            u64::MAX,
        )][..],
    )
    .unwrap();
    let inclusion = RecoveryTransactionEvidence {
        containing_transaction: STANDARD.encode(batch),
        transaction_proof: proof_bytes(&tx_proof),
        execution_header: STANDARD.encode(chain::encode_header(&execution)),
        execution_commit: STANDARD.encode(chain::encode_commit(&execution_commit)),
        activity_ids: vec![activity],
        receipt_index: 0,
        raw_transaction: STANDARD.encode(raw),
        receipt_keys_proof: STANDARD.encode(chain.encode_key(&[3]).unwrap()),
        result_code: result.code,
        result_data: STANDARD.encode(result.data),
        gas_wanted: result.gas_wanted,
        gas_used: result.gas_used,
        result_proof: proof_bytes(&result_proof),
        recovery_did: match case {
            Case::WrongDid => [10; 32],
            Case::EmptyDid => [0; 32],
            _ => DID,
        },
        recovery_root: match case {
            Case::WrongRoot => [8; 32],
            Case::EmptyRoot => [0; 32],
            _ => ROOT,
        },
        recovery_threshold: match case {
            Case::WrongThreshold => 3,
            Case::EmptyThreshold => 0,
            _ => 2,
        },
    };
    let receipt = RecoveryReceipt {
        version: 1,
        principal: "administrator-selected-principal".into(),
        evidence_id: "recovery-1".into(),
        canonical_receipt: STANDARD.encode(canonical),
        header: STANDARD.encode(chain::encode_header(&chain.header)),
        commit: STANDARD.encode(chain::encode_commit(&chain.commit)),
        transaction_proof: String::new(),
    }
    .with_transaction_evidence(&inclusion)
    .unwrap();
    Transcript {
        receipt,
        trust: TrustRoot::from_head(&head, chain.verifier.validators.clone())
            .unwrap()
            .canonical_bytes()
            .unwrap(),
    }
}

struct Fixture {
    root: PathBuf,
}
impl Fixture {
    fn new(trust: &[u8]) -> Self {
        let root = std::env::temp_dir().join(format!(
            "lx-22-33-46-{}-{}",
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

#[test]
fn repair_22_33_46_success_preserves_administrator_principal_through_reveal_and_restart() {
    let transcript = transcript(Case::Recovery);
    let fixture = Fixture::new(&transcript.trust);
    fixture.ingest(&transcript.receipt).unwrap();
    fixture.ingest(&transcript.receipt).unwrap();
    let authority = support::ClockAuthority::start(&fixture.root);
    let principal = PrincipalId::new(transcript.receipt.principal.clone()).unwrap();
    for now in [100, 101] {
        let server = fixture.start(&authority);
        let revealed = fixture
            .client()
            .reveal_verified_receipt(&principal, "recovery-1", now)
            .unwrap();
        assert_eq!(revealed.expose(), transcript.receipt.canonical_receipt);
        assert_eq!(revealed.remask_at(), now + 60);
        assert!(revealed.copyable());
        for other in ["alice", "bob"] {
            assert!(fixture
                .client()
                .reveal_verified_receipt(&PrincipalId::new(other).unwrap(), "recovery-1", now)
                .is_err());
        }
        drop(server);
        assert!(!fixture.root.join("socket").exists());
        drop(Store::open(&fixture.state(), &fixture.trust()).unwrap());
    }
}

#[test]
fn repair_22_33_46_authenticated_failures_non_recovery_and_wrong_policy_are_refused_at_ingestion() {
    for &case in REFUSALS {
        let transcript = transcript(case);
        let fixture = Fixture::new(&transcript.trust);
        assert!(
            fixture.ingest(&transcript.receipt).is_err(),
            "accepted {case:?}"
        );
        assert!(!fixture.state().exists(), "{case:?} initialized state");
    }
}

#[test]
fn repair_22_33_46_authenticated_invalid_receipts_are_refused_at_reveal_and_replay() {
    let valid = transcript(Case::Recovery);
    for &case in REFUSALS {
        let invalid = transcript(case);
        // The fixture's validator identity is stable; each case has its own
        // genuinely signed headers, transaction, receipt and result commitment.
        let fixture = Fixture::new(&valid.trust);
        fixture.ingest(&valid.receipt).unwrap();
        let authority = support::ClockAuthority::start(&fixture.root);
        let server = fixture.start(&authority);
        let principal = PrincipalId::new(valid.receipt.principal.clone()).unwrap();
        assert!(fixture
            .client()
            .reveal_verified_receipt(&principal, "recovery-1", 100)
            .is_ok());
        fixture.replace_journal_receipt(&invalid.receipt);
        assert!(
            fixture
                .client()
                .reveal_verified_receipt(&principal, "recovery-1", 101)
                .is_err(),
            "revealed {case:?}"
        );
        drop(server);
        assert!(
            Store::open(&fixture.state(), &fixture.trust()).is_err(),
            "replayed {case:?}"
        );
    }
}
