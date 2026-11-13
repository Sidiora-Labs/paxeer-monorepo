use layerx_human_test_support::StepClock;
use layerx_types::clock::{Clock, Deadline};
use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_agentd::human_receipt_frame::ReceiptLookupFrame;
use layerx_human_service::frame::{read_frame, write_frame, Limits};
use layerx_human_service::journeys::ReceiptMaterial;
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_types::payload::ModuleRegistry;

pub const TRACE: &str = "trc_0123456789abcdef0123456789abcdef";
const MAX_FRAME: usize = 4096;

pub fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("agent runtime peer: {error:?}"))
}

pub struct Peer {
    directory: PathBuf,
    server: Option<JoinHandle<()>>,
}

impl Peer {
    #[must_use]
    /// # Panics
    /// Panics if the peer socket cannot be opened.
    pub fn serve(exchanges: Vec<(Vec<u8>, Vec<u8>)>) -> (Self, AgentRuntime) {
        let mut entropy = [0; 16];
        checked(getrandom::fill(&mut entropy));
        let directory =
            std::env::temp_dir().join(format!("human-rungs-{:032x}", u128::from_be_bytes(entropy)));
        checked(std::fs::create_dir(&directory));
        let socket = directory.join("peer.sock");
        let listener = checked(UnixListener::bind(&socket));
        checked(listener.set_nonblocking(true));
        let clock: Arc<dyn Clock> = Arc::new(StepClock::new(100_000));
        let peer_clock = Arc::clone(&clock);
        let server = thread::spawn(move || {
            for (expected, response) in exchanges {
                let mut deadline =
                    checked(Deadline::start(peer_clock.as_ref(), Duration::from_secs(5)));
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(
                                !checked(deadline.remaining(peer_clock.as_ref())).is_zero(),
                                "peer request deadline"
                            );
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("peer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(5))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(5))));
                assert_eq!(checked(read_frame(&mut stream, MAX_FRAME)), expected);
                checked(write_frame(&mut stream, &response, MAX_FRAME));
            }
        });
        let runtime = checked(AgentRuntime::new_with_clock(
            &socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: Duration::from_secs(5),
            },
            checked(ModuleRegistry::new(&[])),
            clock,
        ));
        (
            Self {
                directory,
                server: Some(server),
            },
            runtime,
        )
    }

    pub fn finish(mut self) {
        if let Some(server) = self.server.take() {
            checked(server.join());
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.directory);
    }
}

pub fn bytes(target: &mut Vec<u8>, value: &[u8]) {
    target.extend_from_slice(&checked(u32::try_from(value.len())).to_be_bytes());
    target.extend_from_slice(value);
}

fn traced(inner: &[u8]) -> Vec<u8> {
    let mut frame = b"LXHAGT02".to_vec();
    frame.extend_from_slice(TRACE.as_bytes());
    bytes(&mut frame, inner);
    frame
}

#[must_use]
pub fn request(opcode: u8, body: &[u8]) -> Vec<u8> {
    let mut inner = b"LXHAGT01".to_vec();
    inner.push(opcode);
    inner.extend_from_slice(body);
    traced(&inner)
}

#[must_use]
pub fn answer(body: &[u8]) -> Vec<u8> {
    let mut inner = b"LXHAGT01\0".to_vec();
    inner.extend_from_slice(body);
    traced(&inner)
}

#[must_use]
pub fn frame(material: &ReceiptMaterial) -> ReceiptLookupFrame {
    ReceiptLookupFrame {
        canonical_bytes: material.canonical_bytes.clone(),
        authorised_batch: material.authorised_batch,
        verification_level: material.verification_level,
        tx_hash: material.tx_hash,
        receipt_hash: material.receipt_hash,
        block_height: material.block_height,
        rung: material.rung,
        signed_receipt: material.signed_receipt.clone(),
        receipt_key_id: material.receipt_key.key_id,
        public_key: material.receipt_key.public_key,
        first_height: material.receipt_key.first_height,
        last_height: material.receipt_key.last_height,
        inclusion_height: material.inclusion_height,
    }
}
