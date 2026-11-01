use std::collections::{BTreeMap, BTreeSet, VecDeque};
use std::fs::{File, OpenOptions};
use std::io::{Read as _, Write as _};
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::thread::{self, JoinHandle};

use layerx_client::lni::handshake::{encode_node_info, NodeInfo, NodeRole};
use layerx_client::lni::schema::{
    decode_envelope, encode_envelope, Capability, Envelope, SchemaError, Version,
};
use layerx_crypto::evm_transaction::{self, Eip1559Call, EvmTransactionError};
use layerx_crypto::signer::AgentSignature;
use layerx_human_service::frame::{read_frame, write_frame, FrameViolation, TransportError};
use layerx_intents::canonical::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use layerx_types::payload::ModuleRegistry;
use layerx_types::result::{KnownResult, ResultCode};
use sha2::{Digest as _, Sha256};

const NODE_INFO_REQUEST_TAG: u16 = 1;
const NODE_INFO_RESPONSE_TAG: u16 = 2;
const SUBMIT_REQUEST_TAG: u16 = 3;
const SUBMIT_RESPONSE_TAG: u16 = 4;
const ERROR_RESPONSE_TAG: u16 = 25;

const MALFORMED_REFUSAL_CLASS: u8 = 1;
const ADMISSION_REFUSAL_CLASS: u8 = 4;
const AUTHENTICATION_REFUSAL_CLASS: u8 = 6;

const JOURNAL_NAME: &str = "admission.journal";
const JOURNAL_MAGIC: &[u8; 4] = b"LXHA";
const JOURNAL_VERSION: u16 = 1;
const JOURNAL_SUPERBLOCK_BYTES: usize = 10;
const RECORD_MAGIC: &[u8; 4] = b"LXAR";
const RECORD_HEADER_BYTES: usize = 40;
const RECORD_DIGEST_BYTES: usize = 32;

pub const FRAME_BYTES: usize = 1_048_576;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Refusal {
    pub class: u8,
    pub result: ResultCode,
}

impl Refusal {
    const fn known(class: u8, result: KnownResult) -> Self {
        Self {
            class,
            result: ResultCode::from_raw(result.raw()),
        }
    }

    fn payload(self) -> [u8; 5] {
        let mut payload = [0_u8; 5];
        payload[0] = self.class;
        payload[1..].copy_from_slice(&self.result.raw().to_be_bytes());
        payload
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdmissionRecord {
    pub tx_hash: [u8; 32],
    pub raw_transaction: Vec<u8>,
}

#[derive(Debug)]
pub struct AdmissionJournal {
    directory: PathBuf,
    file: File,
    records: Vec<AdmissionRecord>,
    admitted: BTreeSet<[u8; 32]>,
}

impl AdmissionJournal {
    /// # Panics
    /// Panics if journal creation, permissions, persistence, or recovery fails.
    #[must_use]
    pub fn open(directory: &Path, network_id: u32) -> Self {
        std::fs::create_dir_all(directory)
            .unwrap_or_else(|error| panic!("admission directory: {error}"));
        std::fs::set_permissions(directory, std::fs::Permissions::from_mode(0o700))
            .unwrap_or_else(|error| panic!("admission directory permissions: {error}"));
        let path = directory.join(JOURNAL_NAME);
        let existed = path.exists();
        let mut file = OpenOptions::new()
            .read(true)
            .append(true)
            .create(true)
            .mode(0o600)
            .open(&path)
            .unwrap_or_else(|error| panic!("admission journal: {error}"));
        let records = if existed {
            Self::recover_from(&mut file, network_id)
        } else {
            file.write_all(&superblock(network_id))
                .unwrap_or_else(|error| panic!("admission superblock: {error}"));
            file.sync_data()
                .unwrap_or_else(|error| panic!("admission superblock fdatasync: {error}"));
            File::open(directory)
                .and_then(|parent| parent.sync_all())
                .unwrap_or_else(|error| panic!("admission directory fsync: {error}"));
            Vec::new()
        };
        let admitted = records.iter().map(|record| record.tx_hash).collect();
        Self {
            directory: directory.to_path_buf(),
            file,
            records,
            admitted,
        }
    }

    /// # Panics
    /// Panics if the journal cannot be read or its superblock does not match the network.
    #[must_use]
    pub fn recover(directory: &Path, network_id: u32) -> Vec<AdmissionRecord> {
        let mut file = File::open(directory.join(JOURNAL_NAME))
            .unwrap_or_else(|error| panic!("recover admission journal: {error}"));
        Self::recover_from(&mut file, network_id)
    }

    fn recover_from(file: &mut File, network_id: u32) -> Vec<AdmissionRecord> {
        let mut bytes = Vec::new();
        file.read_to_end(&mut bytes)
            .unwrap_or_else(|error| panic!("read admission journal: {error}"));
        assert!(
            bytes.len() >= JOURNAL_SUPERBLOCK_BYTES
                && bytes[..JOURNAL_SUPERBLOCK_BYTES] == superblock(network_id),
            "admission journal superblock mismatch"
        );
        let mut records = Vec::new();
        let mut cursor = JOURNAL_SUPERBLOCK_BYTES;
        while cursor + RECORD_HEADER_BYTES <= bytes.len() {
            let header = &bytes[cursor..cursor + RECORD_HEADER_BYTES];
            if &header[..4] != RECORD_MAGIC {
                break;
            }
            let length = usize::try_from(u32::from_be_bytes([
                header[4], header[5], header[6], header[7],
            ]))
            .unwrap_or_else(|_| panic!("admission record length overflow"));
            let mut tx_hash = [0_u8; 32];
            tx_hash.copy_from_slice(&header[8..40]);
            let body_end = cursor + RECORD_HEADER_BYTES + length;
            let record_end = body_end + RECORD_DIGEST_BYTES;
            if length == 0 || length > MAX_MESSAGE_BYTES || record_end > bytes.len() {
                break;
            }
            let raw_transaction = &bytes[cursor + RECORD_HEADER_BYTES..body_end];
            if bytes[body_end..record_end] != record_digest(&header[4..40], raw_transaction) {
                break;
            }
            records.push(AdmissionRecord {
                tx_hash,
                raw_transaction: raw_transaction.to_vec(),
            });
            cursor = record_end;
        }
        records
    }

    #[must_use]
    pub fn contains(&self, tx_hash: &[u8; 32]) -> bool {
        self.admitted.contains(tx_hash)
    }

    #[must_use]
    pub fn records(&self) -> &[AdmissionRecord] {
        &self.records
    }

    #[must_use]
    pub fn directory(&self) -> &Path {
        &self.directory
    }

    fn admit(&mut self, tx_hash: [u8; 32], raw_transaction: &[u8]) -> std::io::Result<()> {
        let mut record =
            Vec::with_capacity(RECORD_HEADER_BYTES + raw_transaction.len() + RECORD_DIGEST_BYTES);
        record.extend_from_slice(RECORD_MAGIC);
        let length = u32::try_from(raw_transaction.len())
            .map_err(|_| std::io::Error::other("admission record length overflow"))?;
        record.extend_from_slice(&length.to_be_bytes());
        record.extend_from_slice(&tx_hash);
        record.extend_from_slice(raw_transaction);
        let digest = record_digest(&record[4..RECORD_HEADER_BYTES], raw_transaction);
        record.extend_from_slice(&digest);
        self.file.write_all(&record)?;
        self.file.sync_data()?;
        self.records.push(AdmissionRecord {
            tx_hash,
            raw_transaction: raw_transaction.to_vec(),
        });
        self.admitted.insert(tx_hash);
        Ok(())
    }
}

fn superblock(network_id: u32) -> [u8; JOURNAL_SUPERBLOCK_BYTES] {
    let mut bytes = [0_u8; JOURNAL_SUPERBLOCK_BYTES];
    bytes[..4].copy_from_slice(JOURNAL_MAGIC);
    bytes[4..6].copy_from_slice(&JOURNAL_VERSION.to_be_bytes());
    bytes[6..10].copy_from_slice(&network_id.to_be_bytes());
    bytes
}

fn record_digest(header_tail: &[u8], raw_transaction: &[u8]) -> [u8; RECORD_DIGEST_BYTES] {
    let mut digest = Sha256::new();
    digest.update(b"LXH/v1/admission-record\0");
    digest.update(header_tail);
    digest.update(raw_transaction);
    digest.finalize().into()
}

#[derive(Debug)]
pub struct EvidenceNode {
    info: NodeInfo,
    identities: BTreeMap<Vec<u8>, [u8; 32]>,
    journal: AdmissionJournal,
    queue: VecDeque<[u8; 32]>,
    queue_capacity: usize,
    authentication_refusals: u64,
    fail_stopped: bool,
}

impl EvidenceNode {
    /// # Panics
    /// Panics if queue capacity is zero or opening the admission journal fails.
    #[must_use]
    pub fn new(
        authorised_sequencer_key: [u8; 32],
        network_id: u32,
        _registry: ModuleRegistry,
        admission_directory: &Path,
        queue_capacity: usize,
    ) -> Self {
        assert!(
            queue_capacity > 0,
            "admission queue capacity must be bounded above zero"
        );
        Self {
            info: NodeInfo {
                interface_version: Version::V1_3,
                protocol_version: PROTOCOL_VERSION,
                network_id,
                role: NodeRole::Sequencer,
                chain_head_sequence: 50,
                latest_sealed_batch: 7,
                latest_finalised_checkpoint: [0x91; 32],
                authorised_sequencer_key,
                advertised_capabilities: vec![
                    Capability::AuthenticatedDurableSubmit.name().to_owned(),
                    Capability::NodeInfo.name().to_owned(),
                    Capability::Submit.name().to_owned(),
                ],
            },
            identities: BTreeMap::new(),
            journal: AdmissionJournal::open(admission_directory, network_id),
            queue: VecDeque::new(),
            queue_capacity,
            authentication_refusals: 0,
            fail_stopped: false,
        }
    }

    pub fn register_identity(&mut self, actor_did: &[u8], owner_key: [u8; 32]) {
        self.identities.insert(actor_did.to_vec(), owner_key);
    }

    #[must_use]
    pub const fn journal(&self) -> &AdmissionJournal {
        &self.journal
    }

    #[must_use]
    pub fn queued(&self) -> Vec<[u8; 32]> {
        self.queue.iter().copied().collect()
    }

    #[must_use]
    pub const fn authentication_refusals(&self) -> u64 {
        self.authentication_refusals
    }

    #[must_use]
    pub const fn fail_stopped(&self) -> bool {
        self.fail_stopped
    }

    /// # Panics
    /// Panics if a worker cannot start; the worker panics on invalid handshakes or I/O failures.
    #[must_use]
    pub fn serve(mut self, listener: UnixListener) -> JoinHandle<Self> {
        thread::spawn(move || loop {
            let (mut stream, _) = listener
                .accept()
                .unwrap_or_else(|error| panic!("accept evidence node connection: {error}"));
            match self.serve_connection(&mut stream) {
                Ok(()) => return self,
                Err(TransportError::Frame(FrameViolation::Oversized { .. })) => continue,
                Err(error) => panic!("read evidence handshake: {error:?}"),
            }
        })
    }

    fn serve_connection(&mut self, stream: &mut UnixStream) -> Result<(), TransportError> {
        let request = read_frame(stream, FRAME_BYTES)?;
        let request = decode_envelope(&request)
            .unwrap_or_else(|error| panic!("decode evidence handshake: {error:?}"));
        assert_eq!(request.message_tag, NODE_INFO_REQUEST_TAG);
        assert_eq!(request.correlation_id, 0);
        assert!(request.canonical_payload.is_empty());
        assert!(request.proof_material.is_empty());
        let payload = encode_node_info(&self.info)
            .unwrap_or_else(|error| panic!("encode evidence node information: {error:?}"));
        self.respond(stream, NODE_INFO_RESPONSE_TAG, 0, &payload, &[]);
        while !self.fail_stopped {
            let frame = match read_frame(stream, FRAME_BYTES) {
                Ok(frame) => frame,
                Err(error @ TransportError::Frame(FrameViolation::Oversized { .. })) => {
                    return Err(error);
                }
                Err(_) => break,
            };
            let Ok(request) = decode_envelope(&frame) else {
                break;
            };
            let correlation_id = request.correlation_id;
            if request.message_tag != SUBMIT_REQUEST_TAG {
                let refusal =
                    Refusal::known(MALFORMED_REFUSAL_CLASS, KnownResult::MalformedEnvelope);
                self.respond(
                    stream,
                    ERROR_RESPONSE_TAG,
                    correlation_id,
                    &refusal.payload(),
                    &[],
                );
                continue;
            }
            match self.admit(&request) {
                Ok(tx_hash) => {
                    let echoed = request.canonical_payload.to_vec();
                    self.respond(
                        stream,
                        SUBMIT_RESPONSE_TAG,
                        correlation_id,
                        &echoed,
                        &tx_hash,
                    );
                }
                Err(Some(refusal)) => {
                    self.respond(
                        stream,
                        ERROR_RESPONSE_TAG,
                        correlation_id,
                        &refusal.payload(),
                        &[],
                    );
                }
                Err(None) => {
                    self.fail_stopped = true;
                }
            }
        }
        Ok(())
    }

    fn respond(
        &self,
        stream: &mut UnixStream,
        message_tag: u16,
        correlation_id: u64,
        canonical_payload: &[u8],
        proof_material: &[u8],
    ) {
        let response = encode_envelope(Envelope {
            version: self.info.interface_version,
            message_tag,
            correlation_id,
            canonical_payload,
            proof_material,
        })
        .unwrap_or_else(|error: SchemaError| panic!("encode evidence node response: {error:?}"));
        write_frame(stream, &response, FRAME_BYTES)
            .unwrap_or_else(|error| panic!("write evidence node response: {error:?}"));
    }

    fn admit(&mut self, request: &Envelope<'_>) -> Result<[u8; 32], Option<Refusal>> {
        let payload = request.canonical_payload;
        if !request.proof_material.is_empty()
            || payload.is_empty()
            || payload.len() > MAX_MESSAGE_BYTES
        {
            return Err(Some(Refusal::known(
                MALFORMED_REFUSAL_CLASS,
                KnownResult::MalformedEnvelope,
            )));
        }
        let (call, signature) = decode_signed(payload).map_err(|error| {
            Some(if error == EvmTransactionError::BadSignature {
                self.authentication_refusal(KnownResult::BadSignature)
            } else {
                Refusal::known(ADMISSION_REFUSAL_CLASS, KnownResult::MalformedEnvelope)
            })
        })?;
        if call.chain_id != crate::EVIDENCE_CHAIN_ID {
            return Err(Some(Refusal::known(
                ADMISSION_REFUSAL_CLASS,
                KnownResult::WrongNetwork,
            )));
        }
        evm_transaction::recover_sender(&call, &signature)
            .map_err(|_| Some(self.authentication_refusal(KnownResult::BadSignature)))?;
        let identifier = transaction_hash(payload);

        if self.journal.contains(&identifier) {
            return Ok(identifier);
        }
        if self.queue.len() >= self.queue_capacity {
            return Err(Some(Refusal::known(
                ADMISSION_REFUSAL_CLASS,
                KnownResult::LengthLimit,
            )));
        }
        self.journal.admit(identifier, payload).map_err(|_| None)?;
        self.queue.push_back(identifier);
        Ok(identifier)
    }

    fn authentication_refusal(&mut self, result: KnownResult) -> Refusal {
        self.authentication_refusal_code(ResultCode::from_raw(result.raw()))
    }

    fn authentication_refusal_code(&mut self, result: ResultCode) -> Refusal {
        self.authentication_refusals = self.authentication_refusals.saturating_add(1);
        Refusal {
            class: AUTHENTICATION_REFUSAL_CLASS,
            result,
        }
    }
}

fn decode_signed(
    raw_transaction: &[u8],
) -> Result<(Eip1559Call, AgentSignature), EvmTransactionError> {
    evm_transaction::decode_signed(raw_transaction)
}

fn transaction_hash(raw_transaction: &[u8]) -> [u8; 32] {
    evm_transaction::transaction_hash(raw_transaction)
}
