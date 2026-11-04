#[path = "../../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_human_security_provider::{RecoveryReceipt, RecoveryTransactionEvidence};
use layerx_programs::chain_proof::{
    header::ChainHeader,
    trust_store::TrustRoot,
    validators::{TrustedValidator, ValidatorSet},
};
use serde::Deserialize;
use std::path::Path;
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::Duration;

pub struct ClockAuthority {
    pub clock: Arc<RuntimeClock>,
    binding: ClockBinding,
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
            clock: RuntimeClock::connect(binding.clone()).unwrap(),
            binding,
            stop,
            thread: Some(thread),
        }
    }

    pub fn configure(&self, command: &mut Command) {
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.binding.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", self.binding.process.to_string())
            .env("LAYERX_RUNTIME_CLOCK_UID", self.binding.user.to_string());
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.thread.take().unwrap().join().unwrap();
    }
}

#[derive(Deserialize)]
struct RecordedValidator {
    public_key: String,
    power: u64,
}

#[derive(Deserialize)]
struct RecordedCase {
    validators: Vec<RecordedValidator>,
    receipt: RecoveryReceipt,
    inclusion: RecoveryTransactionEvidence,
}

#[derive(Deserialize)]
struct Recording {
    provenance: String,
    cases: Vec<RecordedCase>,
}

pub fn recorded_evidence(index: usize) -> (Vec<u8>, RecoveryReceipt) {
    let recording: Recording =
        serde_json::from_slice(include_bytes!("recording/recovery-22-33-47.json")).unwrap();
    assert!(recording
        .provenance
        .starts_with("paxd RPC via daemon/committed-fixture"));
    assert_eq!(recording.cases.len(), 3);
    let case = recording.cases.into_iter().nth(index).unwrap();
    let header = ChainHeader::decode(&STANDARD.decode(&case.receipt.header).unwrap()).unwrap();
    let validators = ValidatorSet::new(
        case.validators
            .into_iter()
            .map(|v| {
                TrustedValidator::new(
                    STANDARD.decode(v.public_key).unwrap().try_into().unwrap(),
                    v.power,
                )
            })
            .collect(),
    )
    .unwrap();
    assert_eq!(validators.hash().as_slice(), header.validators_hash);
    let trust = TrustRoot {
        chain_id: header.chain_id.clone(),
        height: u64::try_from(header.height).unwrap(),
        time_ms: u64::try_from(header.time.seconds).unwrap() * 1000
            + u64::try_from(header.time.nanos / 1_000_000).unwrap(),
        header_hash: header.hash().unwrap(),
        validators,
    }
    .canonical_bytes()
    .unwrap();
    (
        trust,
        case.receipt
            .with_transaction_evidence(&case.inclusion)
            .unwrap(),
    )
}

pub fn evidence() -> (Vec<u8>, RecoveryReceipt) {
    recorded_evidence(1)
}
