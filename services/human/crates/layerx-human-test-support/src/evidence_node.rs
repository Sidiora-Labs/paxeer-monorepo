use std::collections::BTreeSet;
use std::fs::{File, OpenOptions};
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{TcpListener, TcpStream};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use serde_json::{json, Value};
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};

use layerx_intents::canonical::MAX_MESSAGE_BYTES;
use layerx_types::result::ResultCode;
use sha2::{Digest as _, Sha256};

const JOURNAL_NAME: &str = "admission.journal";
const JOURNAL_MAGIC: &[u8; 4] = b"LXHA";
const JOURNAL_VERSION: u16 = 1;
const JOURNAL_SUPERBLOCK_BYTES: usize = 10;
const RECORD_MAGIC: &[u8; 4] = b"LXAR";
const RECORD_HEADER_BYTES: usize = 40;
const RECORD_DIGEST_BYTES: usize = 32;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Refusal {
    pub class: u8,
    pub result: ResultCode,
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
            let (records, valid_bytes) = Self::recover_from(&mut file, network_id);
            file.set_len(valid_bytes as u64)
                .unwrap_or_else(|error| panic!("truncate admission tail: {error}"));
            file.sync_data()
                .unwrap_or_else(|error| panic!("sync recovered admission tail: {error}"));
            records
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
        Self::recover_from(&mut file, network_id).0
    }

    fn recover_from(file: &mut File, network_id: u32) -> (Vec<AdmissionRecord>, usize) {
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
        (records, cursor)
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

/// In-process JSON-RPC producer with authenticated, durable transaction admission.
/// The retired Unix LNI listener is replaced by the same HTTP boundary NodeRpc uses.
pub struct EvidenceNode {
    journal: AdmissionJournal,
    receipt_key: [u8; 32],
    network_id: u32,
    senders: BTreeSet<[u8; 20]>,
    requests: usize,
    served: Vec<String>,
}

impl EvidenceNode {
    #[must_use]
    pub fn new(directory: &Path, network_id: u32, signer: &SigningKey, requests: usize) -> Self {
        assert!(
            requests > 0,
            "evidence_node.rs: producer requires bounded requests"
        );
        Self {
            journal: AdmissionJournal::open(directory, network_id),
            receipt_key: signer.verifying_key().to_bytes(),
            network_id,
            senders: BTreeSet::new(),
            requests,
            served: Vec::new(),
        }
    }

    pub fn register_sender(&mut self, sender: [u8; 20]) {
        self.senders.insert(sender);
    }

    #[must_use]
    pub fn journal(&self) -> &AdmissionJournal {
        &self.journal
    }

    #[must_use]
    pub fn served(&self) -> &[String] {
        &self.served
    }

    fn submit(&mut self, raw: &[u8]) -> Result<[u8; 32], &'static str> {
        if raw.is_empty() || raw.len() > MAX_MESSAGE_BYTES {
            return Err("LXP_MALFORMED_ENVELOPE: bounded signed transaction required");
        }
        let (transaction, signature) =
            decode_signed(raw).map_err(|_| "LXP_MALFORMED_ENVELOPE: invalid signed transaction")?;
        let sender = recover_sender(&transaction, &signature)
            .map_err(|_| "LXP_BAD_SIGNATURE: sender recovery failed")?;
        if transaction.chain_id != 125 {
            return Err("LXP_WRONG_NETWORK: chain id differs");
        }
        if !self.senders.contains(&sender) {
            return Err("LXP_UNKNOWN_DID: sender is not registered");
        }
        let hash = transaction_hash(raw);
        if !self.journal.contains(&hash) {
            self.journal
                .admit(hash, raw)
                .unwrap_or_else(|error| panic!("durable admission before response: {error}"));
        }
        Ok(hash)
    }

    fn result(&mut self, method: &str, params: &Value) -> Value {
        match method {
            "eth_chainId" => {
                assert_eq!(params, &json!([]));
                json!({"result": "0x7d"})
            }
            "px_getReceiptKeys" => {
                assert_eq!(params, &json!([]));
                let key = crate::encode_hex(&self.receipt_key);
                json!({"result": {"keys": [{"keyId": format!("0x{key}"),
                    "publicKey": format!("0x{key}"), "firstHeight": 7, "lastHeight": 7}]}})
            }
            "eth_sendRawTransaction" => {
                let raw = params
                    .as_array()
                    .filter(|items| items.len() == 1)
                    .and_then(|items| items[0].as_str())
                    .and_then(decode_hex);
                match raw
                    .as_deref()
                    .ok_or("LXP_MALFORMED_ENVELOPE: invalid hex")
                    .and_then(|raw| self.submit(raw))
                {
                    Ok(hash) => json!({"result": format!("0x{}", crate::encode_hex(&hash))}),
                    Err(reason) => {
                        json!({"error": {"code": 3, "message": "execution reverted", "data": reason}})
                    }
                }
            }
            "pax_sequencerReceipt" => {
                let hash = params[0].as_str().and_then(decode_hex);
                let record = hash.as_deref().and_then(|hash| {
                    self.journal
                        .records()
                        .iter()
                        .find(|record| record.tx_hash == hash)
                });
                let result = record.map(|record| {
                    let (transaction, _) = decode_signed(&record.raw_transaction)
                        .unwrap_or_else(|error| panic!("admitted transaction decode: {error:?}"));
                    json!({"rung": "submitted", "txHash": format!("0x{}", crate::encode_hex(&record.tx_hash)),
                        "nonce": format!("{:#x}", transaction.nonce)})
                });
                json!({"result": result})
            }
            method => panic!(
                "evidence_node.rs: unexpected producer method {method} on network {}",
                self.network_id
            ),
        }
    }

    /// Serves a finite number of real JSON-RPC requests and returns the producer state.
    #[must_use]
    pub fn serve(mut self, listener: TcpListener) -> JoinHandle<Self> {
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("evidence listener: {error}"));
        thread::spawn(move || {
            for _ in 0..self.requests {
                let stream = accept_request(&listener, 5_000);
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .unwrap_or_else(|error| panic!("evidence read timeout: {error}"));
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .unwrap_or_else(|error| panic!("evidence write timeout: {error}"));
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(
                        reader
                            .read_line(&mut line)
                            .unwrap_or_else(|error| panic!("evidence header: {error}")),
                        0
                    );
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(
                            value
                                .trim()
                                .parse::<usize>()
                                .unwrap_or_else(|error| panic!("evidence length: {error}")),
                        );
                    }
                }
                let length = length.expect("evidence_node.rs: content length required");
                assert!(length <= 2 * MAX_MESSAGE_BYTES);
                let mut body = vec![0; length];
                reader
                    .read_exact(&mut body)
                    .unwrap_or_else(|error| panic!("evidence body: {error}"));
                let request: Value = serde_json::from_slice(&body)
                    .unwrap_or_else(|error| panic!("evidence JSON: {error}"));
                assert_eq!(request["jsonrpc"], "2.0");
                let method = request["method"]
                    .as_str()
                    .expect("evidence_node.rs: method required");
                self.served.push(method.to_owned());
                let mut response = self.result(method, &request["params"]);
                response["jsonrpc"] = json!("2.0");
                response["id"] = request["id"].clone();
                let answer = response.to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len())
                    .unwrap_or_else(|error| panic!("evidence response: {error}"));
                stream
                    .flush()
                    .unwrap_or_else(|error| panic!("evidence flush: {error}"));
            }
            self
        })
    }
}

fn accept_request(listener: &TcpListener, attempts: usize) -> TcpStream {
    for _ in 0..attempts {
        match listener.accept() {
            Ok((stream, _)) => return stream,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                thread::sleep(Duration::from_millis(1));
            }
            Err(error) => panic!("evidence accept: {error}"),
        }
    }
    panic!("evidence_node.rs: missing consumer request");
}

fn decode_hex(text: &str) -> Option<Vec<u8>> {
    let digits = text.strip_prefix("0x")?;
    if digits.len() % 2 != 0 {
        return None;
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).ok()?, 16).ok())
        .collect()
}
