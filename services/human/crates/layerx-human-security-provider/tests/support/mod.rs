mod chain;
#[path = "../../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_security_provider::RecoveryReceipt;
use layerx_intents::canonical::{execution_batch_id, receipt_digest};
use layerx_intents::vectors::security_program_receipt;
use layerx_programs::chain_proof::{
    merkle::{hash_from_byte_slices, SimpleProof},
    trust_store::TrustRoot,
    validators::TrustedValidator,
    vote::vote_sign_bytes,
};
use sha2::{Digest as _, Sha256};
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::Duration;

pub struct ClockAuthority {
    pub clock: Arc<RuntimeClock>,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl ClockAuthority {
    pub fn start(root: &Path) -> Self {
        let mut server = clock_server::Server::bind(root).unwrap();
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = stop.clone();
        let thread = std::thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                server.poll().unwrap();
                std::thread::sleep(Duration::from_millis(1));
            }
        });
        Self {
            clock: RuntimeClock::connect(binding).unwrap(),
            stop,
            thread: Some(thread),
        }
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.thread.take().unwrap().join().unwrap();
    }
}

pub fn evidence() -> (Vec<u8>, RecoveryReceipt) {
    let seeds = [[37; 32], [38; 32], [39; 32]];
    let key = SigningKey::from_bytes(&seeds[0]);
    let activity = [42; 32];
    let batch = execution_batch_id([0x11; 32], activity, 1, 1).unwrap();
    let timestamp = 1_700_000_000_000;
    let unsigned =
        security_program_receipt(activity, batch, 1, timestamp, [23; 32], [24; 32], None);
    let signature = key.sign(&receipt_digest(&unsigned).unwrap()).to_bytes();
    let receipt = security_program_receipt(
        activity,
        batch,
        1,
        timestamp,
        [23; 32],
        [24; 32],
        Some(signature),
    );
    let mut fixture = chain::ChainFixture::new(
        "human",
        &seeds,
        vec![(b"recovery".to_vec(), receipt.clone())],
    )
    .unwrap();
    let transaction_hash: [u8; 32] = Sha256::digest(&receipt).into();
    let transaction_root = hash_from_byte_slices(&[&transaction_hash]);
    fixture.header.data_hash = transaction_root.to_vec();
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
    let keys: Vec<_> = seeds.iter().map(SigningKey::from_bytes).collect();
    for signature in &mut fixture.commit.signatures {
        let signer = keys
            .iter()
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap();
        let bytes = vote_sign_bytes(
            &fixture.header.chain_id,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )
        .unwrap();
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let trust = TrustRoot::from_head(&head, fixture.verifier.validators.clone())
        .unwrap()
        .canonical_bytes()
        .unwrap();
    let proof = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: transaction_root,
        aunts: Vec::new(),
    };
    let mut transaction_proof = Vec::new();
    transaction_proof.extend_from_slice(&proof.total.to_be_bytes());
    transaction_proof.extend_from_slice(&proof.index.to_be_bytes());
    transaction_proof.extend_from_slice(&proof.leaf_hash);
    transaction_proof.push(u8::try_from(proof.aunts.len()).unwrap());
    for aunt in &proof.aunts {
        transaction_proof.extend_from_slice(aunt);
    }
    (
        trust,
        RecoveryReceipt {
            version: 1,
            principal: "alice".into(),
            evidence_id: "recovery-1".into(),
            canonical_receipt: STANDARD.encode(receipt),
            header: STANDARD.encode(chain::encode_header(&fixture.header)),
            commit: STANDARD.encode(chain::encode_commit(&fixture.commit)),
            transaction_proof: STANDARD.encode(transaction_proof),
        },
    )
}
