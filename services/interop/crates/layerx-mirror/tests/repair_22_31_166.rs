// Task 22.31.166 repair record (audit 22.30.111):
// Finding 5: ethereum_reopens_prepared_signed_and_broadcast_stages.
// Finding 6: solana_reopens_prepared_signed_and_broadcast_stages.
// Finding 7: pending_disappearance_preserves_ambiguity_on_both_chains,
// solana_pending_expiry_requires_history_before_replacement,
// ethereum_pending_canonical_replacement_reuses_signed_identity.
// Wiring requests: none; all three repairs are local to this crate.
// Baseline verify was not run: the task's ordered gate contract requires the
// complete implementation before hf-build and hf-done, and forbids hand gates.
// Crash fixtures copy the production journal and sealed head at the signer/RPC
// boundary, before either peer replies; no records are manufactured for recovery.

use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc as Shared, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey as EdKey};
use k256::ecdsa::SigningKey as EthKey;
use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::ethereum::{EthereumArchiveClient, EthereumError, EthereumProductionConfig};
use layerx_mirror::rpc::{RpcEndpointConfig, RpcQuorumConfig};
use layerx_mirror::signer::{
    RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm,
};
use layerx_mirror::solana::{SolanaArchiveClient, SolanaError, SolanaProductionConfig};
use layerx_mirror::store::{
    FinalityPosition, MirrorChain, PublicationJournal, PublicationPhase, PublicationRecord,
    PublicationStage, StoreError, TransactionIdentity,
};
use layerx_mirror::{Archive, NodeHead};
use openssl::asn1::Asn1Time;
use openssl::bn::BigNum;
use openssl::hash::MessageDigest;
use openssl::pkey::PKey;
use openssl::rsa::Rsa;
use openssl::ssl::{SslAcceptor, SslMethod};
use openssl::x509::extension::{
    BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
};
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use sha3::Keccak256;

static NEXT: AtomicU64 = AtomicU64::new(0);
const CONTRACT: [u8; 20] = [0x21; 20];
const PROGRAM: [u8; 32] = [0x22; 32];
const LOADER: [u8; 32] = [0x23; 32];
const PROGRAM_DATA: [u8; 32] = [0x24; 32];
const CODE: &[u8] = b"immutable recorded archive program";
const GENESIS: [u8; 32] = [0x25; 32];
const BLOCK: [u8; 32] = [0x26; 32];

fn archive() -> Archive {
    let fixture: Value =
        serde_json::from_str(include_str!("fixtures/committed_blocks.json")).unwrap();
    let mut header = CommittedHeader::from_block_json(&fixture["committedBlock"]).unwrap();
    header.height = 1;
    let receipts = fixture["receipts"]
        .as_array()
        .unwrap()
        .iter()
        .map(|value| unhex(value["receipt"].as_str().unwrap()))
        .collect();
    Archive::from_committed(
        &CommittedBlock { header, receipts },
        77,
        NodeHead {
            latest_sealed_batch: 1,
            latest_committed: None,
        },
    )
    .unwrap()
}

fn hex(bytes: &[u8]) -> String {
    let mut text = String::from("0x");
    for byte in bytes {
        use std::fmt::Write as _;
        write!(text, "{byte:02x}").unwrap();
    }
    text
}
fn unhex(text: &str) -> Vec<u8> {
    let digits = text.strip_prefix("0x").unwrap();
    (0..digits.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&digits[i..i + 2], 16).unwrap())
        .collect()
}
fn base58(bytes: &[u8]) -> String {
    const ALPHABET: &[u8] = b"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
    let zeros = bytes.iter().take_while(|&&b| b == 0).count();
    let mut digits = Vec::<u8>::new();
    for &byte in bytes {
        let mut carry = u32::from(byte);
        for digit in &mut digits {
            carry += u32::from(*digit) * 256;
            *digit = (carry % 58) as u8;
            carry /= 58;
        }
        while carry > 0 {
            digits.push((carry % 58) as u8);
            carry /= 58;
        }
    }
    let mut result = String::from_utf8(vec![b'1'; zeros]).unwrap();
    for digit in digits.iter().rev() {
        result.push(char::from(ALPHABET[usize::from(*digit)]));
    }
    result
}
fn word(n: usize) -> Vec<u8> {
    let mut out = vec![0; 32];
    out[24..].copy_from_slice(&(n as u64).to_be_bytes());
    out
}
fn number(bytes: &[u8]) -> usize {
    bytes
        .iter()
        .fold(0usize, |n, b| n.checked_mul(256).unwrap() + usize::from(*b))
}
fn short(bytes: &[u8], offset: &mut usize) -> usize {
    let mut result = 0;
    let mut shift = 0;
    loop {
        let byte = bytes[*offset];
        *offset += 1;
        result |= usize::from(byte & 127) << shift;
        if byte & 128 == 0 {
            return result;
        }
        shift += 7;
        assert!(shift < 21);
    }
}
fn rlp<'a>(bytes: &'a [u8], offset: &mut usize) -> &'a [u8] {
    let tag = bytes[*offset];
    *offset += 1;
    if tag < 128 {
        return &bytes[*offset - 1..*offset];
    }
    let length = if tag <= 183 {
        usize::from(tag - 128)
    } else if tag <= 191 {
        let n = usize::from(tag - 183);
        let len = number(&bytes[*offset..*offset + n]);
        *offset += n;
        len
    } else if tag <= 247 {
        usize::from(tag - 192)
    } else {
        let n = usize::from(tag - 247);
        let len = number(&bytes[*offset..*offset + n]);
        *offset += n;
        len
    };
    let value = &bytes[*offset..*offset + length];
    *offset += length;
    value
}

#[derive(Clone, Copy, Debug, PartialEq)]
enum Observation {
    Finalized,
    Pending,
    Missing,
    CanonicalReplacement,
}
#[derive(Clone)]
struct Transaction {
    raw: Vec<u8>,
    data: Vec<u8>,
    stage: PublicationStage,
}
#[derive(Clone)]
struct Network {
    eth_payer: [u8; 20],
    sol_payer: [u8; 32],
    transactions: BTreeMap<String, Transaction>,
    broadcasts: Vec<Vec<u8>>,
    signatures: usize,
    observation: Observation,
    blockhash_valid: bool,
    creation_failure: bool,
    recent_blockhash: [u8; 32],
    manifest: Vec<u8>,
    chunks: BTreeMap<u32, Vec<u8>>,
    accounts: BTreeMap<String, Vec<u8>>,
}

struct Snapshot {
    directory: PathBuf,
    network: Network,
    phase: PublicationPhase,
}
struct Capture {
    root: PathBuf,
    source: Option<PathBuf>,
    snapshots: Vec<Snapshot>,
}
impl Capture {
    fn save(&mut self, network: &Network, phase: PublicationPhase) {
        let Some(source) = &self.source else {
            return;
        };
        let directory = self.root.join(format!("snapshot-{}", self.snapshots.len()));
        copy_journal(source, &directory);
        self.snapshots.push(Snapshot {
            directory,
            network: network.clone(),
            phase,
        });
    }
}
fn copy_journal(source: &Path, destination: &Path) {
    fs::create_dir(destination).unwrap();
    for entry in fs::read_dir(source).unwrap() {
        let entry = entry.unwrap();
        if matches!(
            entry.path().extension().and_then(|s| s.to_str()),
            Some("journal" | "head")
        ) {
            fs::copy(entry.path(), destination.join(entry.file_name())).unwrap();
        }
    }
}

struct RpcServer {
    config: RpcQuorumConfig,
    running: Shared<AtomicBool>,
    addresses: Vec<SocketAddr>,
    threads: Vec<JoinHandle<()>>,
}
impl RpcServer {
    fn new(
        directory: &Path,
        network: Shared<Mutex<Network>>,
        capture: Shared<Mutex<Capture>>,
    ) -> Self {
        let token_path = directory.join("bearer");
        let mut token = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&token_path)
            .unwrap();
        token.write_all(b"repair-loopback-test").unwrap();
        let key = PKey::from_rsa(Rsa::generate(2048).unwrap_or_else(|e| panic!("RSA: {e}")))
            .unwrap_or_else(|e| panic!("key: {e}"));
        let mut name = X509NameBuilder::new().unwrap_or_else(|e| panic!("name: {e}"));
        name.append_entry_by_text("CN", "Committed source test CA")
            .unwrap_or_else(|e| panic!("name: {e}"));
        let name = name.build();
        let mut ca = X509::builder().unwrap_or_else(|e| panic!("CA: {e}"));
        ca.set_version(2).unwrap_or_else(|e| panic!("version: {e}"));
        let serial = BigNum::from_u32(1)
            .and_then(|n| n.to_asn1_integer())
            .unwrap_or_else(|e| panic!("serial: {e}"));
        ca.set_serial_number(&serial)
            .unwrap_or_else(|e| panic!("serial: {e}"));
        ca.set_subject_name(&name)
            .unwrap_or_else(|e| panic!("subject: {e}"));
        ca.set_issuer_name(&name)
            .unwrap_or_else(|e| panic!("issuer: {e}"));
        ca.set_pubkey(&key)
            .unwrap_or_else(|e| panic!("public key: {e}"));
        let before = Asn1Time::days_from_now(0).unwrap_or_else(|e| panic!("time: {e}"));
        let after = Asn1Time::days_from_now(1).unwrap_or_else(|e| panic!("time: {e}"));
        ca.set_not_before(&before)
            .unwrap_or_else(|e| panic!("time: {e}"));
        ca.set_not_after(&after)
            .unwrap_or_else(|e| panic!("time: {e}"));
        ca.append_extension(
            BasicConstraints::new()
                .critical()
                .ca()
                .build()
                .unwrap_or_else(|e| panic!("CA constraints: {e}")),
        )
        .unwrap_or_else(|e| panic!("extension: {e}"));
        ca.append_extension(
            KeyUsage::new()
                .key_cert_sign()
                .crl_sign()
                .build()
                .unwrap_or_else(|e| panic!("CA usage: {e}")),
        )
        .unwrap_or_else(|e| panic!("extension: {e}"));
        ca.sign(&key, MessageDigest::sha256())
            .unwrap_or_else(|e| panic!("CA signature: {e}"));
        let ca = ca.build();
        let ca_path = directory.join("ca.der");
        fs::write(&ca_path, ca.to_der().unwrap_or_else(|e| panic!("DER: {e}")))
            .unwrap_or_else(|e| panic!("CA file: {e}"));

        let mut certificate = X509::builder().unwrap_or_else(|e| panic!("certificate: {e}"));
        certificate
            .set_version(2)
            .unwrap_or_else(|e| panic!("version: {e}"));
        let serial = BigNum::from_u32(2)
            .and_then(|n| n.to_asn1_integer())
            .unwrap_or_else(|e| panic!("serial: {e}"));
        certificate
            .set_serial_number(&serial)
            .unwrap_or_else(|e| panic!("serial: {e}"));
        certificate
            .set_subject_name(&name)
            .unwrap_or_else(|e| panic!("subject: {e}"));
        certificate
            .set_issuer_name(ca.subject_name())
            .unwrap_or_else(|e| panic!("issuer: {e}"));
        certificate
            .set_pubkey(&key)
            .unwrap_or_else(|e| panic!("public key: {e}"));
        certificate
            .set_not_before(&before)
            .unwrap_or_else(|e| panic!("time: {e}"));
        certificate
            .set_not_after(&after)
            .unwrap_or_else(|e| panic!("time: {e}"));
        certificate
            .append_extension(
                BasicConstraints::new()
                    .critical()
                    .build()
                    .unwrap_or_else(|e| panic!("constraints: {e}")),
            )
            .unwrap_or_else(|e| panic!("extension: {e}"));
        certificate
            .append_extension(
                KeyUsage::new()
                    .digital_signature()
                    .key_encipherment()
                    .build()
                    .unwrap_or_else(|e| panic!("usage: {e}")),
            )
            .unwrap_or_else(|e| panic!("extension: {e}"));
        certificate
            .append_extension(
                ExtendedKeyUsage::new()
                    .server_auth()
                    .build()
                    .unwrap_or_else(|e| panic!("usage: {e}")),
            )
            .unwrap_or_else(|e| panic!("extension: {e}"));
        let san = SubjectAlternativeName::new()
            .ip("127.0.0.1")
            .build(&certificate.x509v3_context(Some(&ca), None))
            .unwrap_or_else(|e| panic!("SAN: {e}"));
        certificate
            .append_extension(san)
            .unwrap_or_else(|e| panic!("extension: {e}"));
        certificate
            .sign(&key, MessageDigest::sha256())
            .unwrap_or_else(|e| panic!("signature: {e}"));
        let certificate = certificate.build();
        let mut acceptor = SslAcceptor::mozilla_intermediate(SslMethod::tls())
            .unwrap_or_else(|e| panic!("TLS: {e}"));
        acceptor
            .set_certificate(&certificate)
            .unwrap_or_else(|e| panic!("TLS certificate: {e}"));
        acceptor
            .set_private_key(&key)
            .unwrap_or_else(|e| panic!("TLS key: {e}"));
        acceptor
            .check_private_key()
            .unwrap_or_else(|e| panic!("TLS key check: {e}"));
        let acceptor = Shared::new(acceptor.build());
        let running = Shared::new(AtomicBool::new(true));
        let mut addresses = Vec::new();
        let mut endpoints = Vec::new();
        let mut threads = Vec::new();
        for index in 0..2 {
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            addresses.push(address);
            endpoints.push(RpcEndpointConfig {
                url: format!("https://{address}/rpc"),
                ca_certificate_der: ca_path.clone(),
                bearer_token_file: token_path.clone(),
                independent_backend: format!("repair-chain-{index}"),
            });
            let acceptor = Shared::clone(&acceptor);
            let running = Shared::clone(&running);
            let network = Shared::clone(&network);
            let capture = Shared::clone(&capture);
            threads.push(thread::spawn(move || {
                while running.load(Ordering::Acquire) {
                    let (tcp,_) = listener.accept().unwrap();
                    if !running.load(Ordering::Acquire) { break; }
                    tcp.set_read_timeout(Some(Duration::from_secs(3))).unwrap();
                    tcp.set_write_timeout(Some(Duration::from_secs(3))).unwrap();
                    let mut stream = acceptor.accept(tcp).unwrap();
                    let mut head=Vec::new();
                    while !head.ends_with(b"\r\n\r\n") { let mut byte=[0]; stream.read_exact(&mut byte).unwrap(); head.push(byte[0]); assert!(head.len() < 32768); }
                    let head=String::from_utf8(head).unwrap();
                    assert!(head.starts_with("POST /rpc HTTP/1.1\r\n"));
                    assert!(head.lines().any(|line| line == "Authorization: Bearer repair-loopback-test"));
                    let length:usize=head.lines().find_map(|line| line.strip_prefix("Content-Length: ")).unwrap().parse().unwrap();
                    assert!(length < 512*1024); let mut body=vec![0;length]; stream.read_exact(&mut body).unwrap();
                    let request: Value=serde_json::from_slice(&body).unwrap();
                    let result=network.lock().unwrap().respond(&request, &capture);
                    let response=serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":request["id"], "result":result})).unwrap();
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).unwrap();
                    stream.write_all(&response).unwrap(); stream.flush().unwrap(); stream.shutdown().unwrap();
                }
            }));
        }
        Self {
            config: RpcQuorumConfig {
                endpoints,
                quorum: 2,
                connect_timeout_ms: 1000,
                request_timeout_ms: 3000,
                maximum_response_bytes: 1024 * 1024,
            },
            running,
            addresses,
            threads,
        }
    }
}
impl Drop for RpcServer {
    fn drop(&mut self) {
        self.running.store(false, Ordering::Release);
        for address in &self.addresses {
            let _ = TcpStream::connect(address);
        }
        for thread in self.threads.drain(..) {
            let result = thread.join();
            if !thread::panicking() {
                assert!(result.is_ok(), "RPC peer failed");
            }
        }
    }
}

impl Network {
    fn respond(&mut self, request: &Value, capture: &Mutex<Capture>) -> Value {
        let params = &request["params"];
        match request["method"].as_str().unwrap() {
            "eth_chainId" => json!("0x1"),
            "eth_getCode" => json!(hex(CODE)),
            "eth_getBlockByNumber" => {
                let hash = if params[0] == "0x0" {
                    GENESIS
                } else if self.observation == Observation::CanonicalReplacement {
                    [0x27; 32]
                } else {
                    BLOCK
                };
                json!({"number":"0xa", "hash":hex(&hash)})
            }
            "eth_blockNumber" => json!("0xb"),
            "eth_estimateGas" => json!(if self.creation_failure {
                "0x0"
            } else {
                "0x10000"
            }),
            "eth_getTransactionCount" => json!(format!("0x{:x}", self.transactions.len())),
            "eth_maxPriorityFeePerGas" => json!("0x1"),
            "eth_gasPrice" => json!("0x2"),
            "eth_call" => {
                let data = unhex(params[0]["data"].as_str().unwrap());
                let publisher = Keccak256::digest(b"publisher()");
                let manifest = Keccak256::digest(b"manifest(bytes32)");
                let chunk = Keccak256::digest(b"chunk(bytes32,uint32)");
                if data[..4] == publisher[..4] {
                    let mut out = vec![0; 12];
                    out.extend_from_slice(&self.eth_payer);
                    json!(hex(&out))
                } else if data[..4] == manifest[..4] {
                    json!(hex(&self.manifest))
                } else {
                    assert_eq!(data[..4], chunk[..4]);
                    let index = number(&data[36..68]) as u32;
                    let bytes = &self.chunks[&index];
                    let mut out = word(32);
                    out.extend(word(bytes.len()));
                    out.extend_from_slice(bytes);
                    out.resize(64 + bytes.len().div_ceil(32) * 32, 0);
                    json!(hex(&out))
                }
            }
            "eth_sendRawTransaction" => {
                capture.lock().unwrap().save(self, PublicationPhase::Signed);
                let raw = unhex(params[0].as_str().unwrap());
                assert_eq!(raw[0], 2);
                let hash = hex(&Keccak256::digest(&raw));
                let mut offset = 1;
                let fields = rlp(&raw, &mut offset);
                assert_eq!(offset, raw.len());
                let mut offset = 0;
                let fields = (0..12)
                    .map(|_| rlp(fields, &mut offset))
                    .collect::<Vec<_>>();
                assert_eq!(number(fields[0]), 1);
                assert_eq!(fields[5], CONTRACT);
                assert!(fields[6].is_empty());
                let data = fields[7].to_vec();
                let selector = &data[..4];
                let stage = if selector
                    == &Keccak256::digest(
                        b"begin(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32,bytes32)",
                    )[..4]
                {
                    PublicationStage::Manifest
                } else if selector == &Keccak256::digest(b"append(bytes32,uint32,bytes)")[..4] {
                    PublicationStage::Chunk(number(&data[36..68]) as u32)
                } else {
                    assert_eq!(selector, &Keccak256::digest(b"finalize(bytes32)")[..4]);
                    PublicationStage::Finalize
                };
                self.broadcasts.push(raw.clone());
                if !self.transactions.contains_key(&hash) {
                    match stage {
                        PublicationStage::Manifest => {
                            self.manifest = data[132..164].to_vec();
                            self.manifest.extend_from_slice(&data[164..196]);
                            self.manifest.extend_from_slice(&data[196..228]);
                            self.manifest.extend(word(0));
                        }
                        PublicationStage::Chunk(index) => {
                            assert_eq!(number(&data[68..100]), 96);
                            let length = number(&data[100..132]);
                            self.chunks.insert(index, data[132..132 + length].to_vec());
                        }
                        PublicationStage::Finalize => {
                            assert_eq!(self.manifest.len(), 128);
                            self.manifest[127] = 1;
                        }
                    }
                    self.transactions
                        .insert(hash.clone(), Transaction { raw, data, stage });
                }
                capture.lock().unwrap().save(self, PublicationPhase::Signed);
                json!(hash)
            }
            "eth_getTransactionReceipt" => {
                if matches!(
                    self.observation,
                    Observation::Missing | Observation::Pending
                ) {
                    return Value::Null;
                }
                let hash = params[0].as_str().unwrap();
                let tx = &self.transactions[hash];
                let event = match tx.stage {
                    PublicationStage::Manifest => {
                        "ManifestOpened(bytes32,uint32,uint64,bytes32,uint64,uint32,bytes32)"
                    }
                    PublicationStage::Chunk(_) => "ChunkStored(bytes32,uint32,bytes32,uint32)",
                    PublicationStage::Finalize => "ArchiveFinalized(bytes32,bytes32)",
                };
                json!({"status":"0x1", "transactionHash":hash,"to":hex(&CONTRACT),"blockNumber":"0xa","blockHash":hex(&BLOCK),
                    "logs":[{"address":hex(&CONTRACT),"topics":[hex(&Keccak256::digest(event.as_bytes())),hex(&tx.data[4..36])]}]})
            }
            "eth_getTransactionByHash" => {
                if self.observation == Observation::Missing {
                    return Value::Null;
                }
                let hash = params[0].as_str().unwrap();
                let tx = &self.transactions[hash];
                json!({"hash":hash,"from":hex(&self.eth_payer),"to":hex(&CONTRACT),"input":hex(&tx.data)})
            }
            "getGenesisHash" => json!(base58(&GENESIS)),
            "getAccountInfo" => {
                let address = params[0].as_str().unwrap();
                let (owner, executable, data) = if address == base58(&PROGRAM) {
                    let mut bytes = 2u32.to_le_bytes().to_vec();
                    bytes.extend_from_slice(&PROGRAM_DATA);
                    (LOADER, true, bytes)
                } else if address == base58(&PROGRAM_DATA) {
                    let mut bytes = 3u32.to_le_bytes().to_vec();
                    bytes.extend_from_slice(&1u64.to_le_bytes());
                    bytes.resize(45, 0);
                    bytes.extend_from_slice(CODE);
                    (LOADER, false, bytes)
                } else if let Some(bytes) = self.accounts.get(address) {
                    (PROGRAM, false, bytes.clone())
                } else {
                    return json!({"value":null});
                };
                json!({"value":{"owner":base58(&owner),"executable":executable,"data":[BASE64.encode(data),"base64"]}})
            }
            "getLatestBlockhash" => {
                if self.creation_failure {
                    json!({"value":{"blockhash":null}})
                } else {
                    json!({"value":{"blockhash":base58(&self.recent_blockhash),"lastValidBlockHeight":100}})
                }
            }
            "isBlockhashValid" => {
                assert!(params[1]["commitment"] == "finalized");
                json!({"value":self.blockhash_valid})
            }
            "sendTransaction" => {
                capture.lock().unwrap().save(self, PublicationPhase::Signed);
                let raw = BASE64.decode(params[0].as_str().unwrap()).unwrap();
                assert_eq!(raw[0], 1);
                let signature = ed25519_dalek::Signature::from_slice(&raw[1..65]).unwrap();
                ed25519_dalek::VerifyingKey::from_bytes(&self.sol_payer)
                    .unwrap()
                    .verify_strict(&raw[65..], &signature)
                    .unwrap();
                let id = base58(&raw[1..65]);
                let message = &raw[65..];
                let mut offset = 3;
                let count = short(message, &mut offset);
                let keys = (0..count)
                    .map(|_| {
                        let key = message[offset..offset + 32].to_vec();
                        offset += 32;
                        key
                    })
                    .collect::<Vec<_>>();
                assert_eq!(keys[0], self.sol_payer);
                assert_eq!(keys[count - 1], PROGRAM);
                offset += 32;
                assert_eq!(short(message, &mut offset), 1);
                assert_eq!(usize::from(message[offset]), count - 1);
                offset += 1;
                let metas = short(message, &mut offset);
                offset += metas;
                let length = short(message, &mut offset);
                let data = message[offset..offset + length].to_vec();
                assert_eq!(offset + length, message.len());
                assert_eq!(&data[..6], b"LXMA\0\x04");
                let stage = match data[6] {
                    1 => PublicationStage::Manifest,
                    2 => PublicationStage::Chunk(number(&data[39..43]) as u32),
                    3 => PublicationStage::Finalize,
                    _ => panic!("instruction"),
                };
                self.broadcasts.push(raw.clone());
                if !self.transactions.contains_key(&id) {
                    let manifest_address = base58(&keys[1]);
                    match stage {
                        PublicationStage::Manifest => {
                            let mut bytes = b"LXMMAN04".to_vec();
                            bytes.extend_from_slice(&data[7..39]);
                            bytes.extend_from_slice(&self.sol_payer);
                            bytes.extend_from_slice(&data[39..]);
                            assert_eq!(bytes.len(), 192);
                            bytes.resize(334, 0);
                            self.accounts.insert(manifest_address, bytes);
                        }
                        PublicationStage::Chunk(index) => {
                            let length = number(&data[43..45]);
                            let chunk = &data[45..45 + length];
                            assert_eq!(&data[45 + length..], &Sha256::digest(chunk)[..]);
                            let mut bytes = b"LXMCHK04".to_vec();
                            bytes.extend_from_slice(&data[7..43]);
                            bytes.extend_from_slice(&data[45 + length..]);
                            bytes.extend_from_slice(&(length as u32).to_be_bytes());
                            bytes.extend_from_slice(chunk);
                            self.accounts.insert(base58(&keys[2]), bytes);
                            self.chunks.insert(index, chunk.to_vec());
                            let manifest = self.accounts.get_mut(&manifest_address).unwrap();
                            assert_eq!(number(&manifest[232..236]), index as usize);
                            let mut hash = Sha256::new();
                            hash.update(&manifest[192..224]);
                            hash.update(index.to_be_bytes());
                            hash.update(Sha256::digest(chunk));
                            hash.update((length as u32).to_be_bytes());
                            manifest[192..224].copy_from_slice(&hash.finalize());
                            let received = number(&manifest[224..232]) + length;
                            manifest[224..232].copy_from_slice(&(received as u64).to_be_bytes());
                            manifest[232..236].copy_from_slice(&(index + 1).to_be_bytes());
                        }
                        PublicationStage::Finalize => {
                            let manifest = self.accounts.get_mut(&manifest_address).unwrap();
                            assert_eq!(manifest[160..192], manifest[192..224]);
                            assert_eq!(manifest[116..124], manifest[224..232]);
                            assert_eq!(manifest[124..128], manifest[232..236]);
                            manifest[236] = 1;
                            let received = number(&manifest[224..232]);
                            let all = self.chunks.values().flatten().copied().collect::<Vec<_>>();
                            assert_eq!(&Sha256::digest(&all)[..], &manifest[128..160]);
                            let tail = received % 64;
                            manifest[269..333].fill(0);
                            manifest[269..269 + tail].copy_from_slice(&all[received - tail..]);
                            manifest[333] = tail as u8;
                        }
                    }
                    self.transactions
                        .insert(id.clone(), Transaction { raw, data, stage });
                }
                capture.lock().unwrap().save(self, PublicationPhase::Signed);
                json!(id)
            }
            "getSignatureStatuses" => {
                assert_eq!(params[1]["searchTransactionHistory"], true);
                let status = match self.observation {
                    Observation::Missing => Value::Null,
                    Observation::Pending => json!({"err":null,"confirmationStatus":"confirmed"}),
                    _ => json!({"err":null,"confirmationStatus":"finalized"}),
                };
                json!({"value":[status]})
            }
            "getTransaction" => {
                let id = params[0].as_str().unwrap();
                let tx = &self.transactions[id];
                json!({"transaction":[BASE64.encode(&tx.raw),"base64"],"meta":{"err":null},"slot":10})
            }
            "getSlot" => json!(11),
            "getBlock" => {
                json!({"blockhash":base58(&if params[0] == 11 { [0x28;32] } else { BLOCK }),
                "parentSlot":10,"previousBlockhash":base58(&BLOCK)})
            }
            other => panic!("unexpected RPC method {other}"),
        }
    }
}

struct SigningServer {
    config: RemoteSignerConfig,
    running: Shared<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}
impl SigningServer {
    fn new(
        directory: &Path,
        chain: MirrorChain,
        network: Shared<Mutex<Network>>,
        capture: Shared<Mutex<Capture>>,
    ) -> Self {
        let socket = directory.join("signer.sock");
        let listener = UnixListener::bind(&socket).unwrap();
        let eth_key = EthKey::from_bytes((&[0x31; 32]).into()).unwrap();
        let sol_key = EdKey::from_bytes(&[0x32; 32]);
        let public_key = match chain {
            MirrorChain::Ethereum => eth_key
                .verifying_key()
                .to_encoded_point(true)
                .as_bytes()
                .to_vec(),
            MirrorChain::Solana => sol_key.verifying_key().to_bytes().to_vec(),
        };
        let running = Shared::new(AtomicBool::new(true));
        let active = Shared::clone(&running);
        let thread = thread::spawn(move || {
            while active.load(Ordering::Acquire) {
                let (mut stream, _) = listener.accept().unwrap();
                if !active.load(Ordering::Acquire) {
                    break;
                }
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                let mut length = [0; 4];
                stream.read_exact(&mut length).unwrap();
                let length = u32::from_be_bytes(length) as usize;
                assert!(length < 8192);
                let mut request = vec![0; length];
                stream.read_exact(&mut request).unwrap();
                assert_eq!(&request[..6], b"LXCS\0\x01");
                let handle_len = number(&request[7..9]);
                assert_eq!(&request[9..9 + handle_len], b"repair-signer");
                let mut offset = 9 + handle_len;
                let domain_len = number(&request[offset..offset + 2]);
                offset += 2;
                let domain = &request[offset..offset + domain_len];
                offset += domain_len;
                let digest = &request[offset..offset + 32];
                offset += 32;
                let message_len = number(&request[offset..offset + 4]);
                offset += 4;
                let message = &request[offset..];
                assert_eq!(message.len(), message_len);
                {
                    let mut net = network.lock().unwrap();
                    capture
                        .lock()
                        .unwrap()
                        .save(&net, PublicationPhase::Prepared);
                    net.signatures += 1;
                }
                let mut response = vec![0];
                match chain {
                    MirrorChain::Ethereum => {
                        assert_eq!(request[6], 1);
                        assert_eq!(domain, b"LayerX/mirror/ethereum-eip1559/v1");
                        assert!(message.is_empty());
                        let (signature, recovery) =
                            eth_key.sign_prehash_recoverable(digest).unwrap();
                        response.extend_from_slice(&signature.to_bytes());
                        response.push(recovery.to_byte());
                    }
                    MirrorChain::Solana => {
                        assert_eq!(request[6], 2);
                        assert_eq!(domain, b"LayerX/mirror/solana-transaction/v1");
                        assert_eq!(digest, &Sha256::digest(message)[..]);
                        response.extend_from_slice(&sol_key.sign(message).to_bytes());
                    }
                }
                stream
                    .write_all(&(response.len() as u32).to_be_bytes())
                    .unwrap();
                stream.write_all(&response).unwrap();
                stream.flush().unwrap();
            }
        });
        Self {
            config: RemoteSignerConfig {
                endpoint: SignerEndpoint::Uds { socket },
                algorithm: match chain {
                    MirrorChain::Ethereum => SigningAlgorithm::Secp256k1Recoverable,
                    MirrorChain::Solana => SigningAlgorithm::Ed25519,
                },
                key_handle: "repair-signer".into(),
                public_key,
                timeout: Duration::from_secs(3),
            },
            running,
            thread: Some(thread),
        }
    }
    fn client(&self) -> RemoteChainSigner {
        RemoteChainSigner::new(self.config.clone()).unwrap()
    }
}
impl Drop for SigningServer {
    fn drop(&mut self) {
        self.running.store(false, Ordering::Release);
        let SignerEndpoint::Uds { socket } = &self.config.endpoint else {
            unreachable!()
        };
        let _ = UnixStream::connect(socket);
        let result = self.thread.take().unwrap().join();
        if !thread::panicking() {
            assert!(result.is_ok(), "signer peer failed");
        }
    }
}

struct Directory(PathBuf);
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}
struct Fixture {
    rpc: RpcServer,
    signer: SigningServer,
    network: Shared<Mutex<Network>>,
    capture: Shared<Mutex<Capture>>,
    chain: MirrorChain,
    directory: Directory,
}
#[derive(Debug)]
struct Progress {
    stage: PublicationStage,
    phase: PublicationPhase,
}
enum Client {
    Ethereum(EthereumArchiveClient),
    Solana(SolanaArchiveClient),
}
impl Client {
    fn advance(&mut self, archive: &Archive) -> Progress {
        match self {
            Self::Ethereum(client) => {
                let p = client.advance(archive).unwrap();
                Progress {
                    stage: p.stage,
                    phase: p.phase,
                }
            }
            Self::Solana(client) => {
                let p = client.advance(archive).unwrap();
                Progress {
                    stage: p.stage,
                    phase: p.phase,
                }
            }
        }
    }
    fn retrieve(&self, archive: &Archive) -> Vec<u8> {
        match self {
            Self::Ethereum(c) => c.retrieve(archive.commitment()).unwrap().unwrap(),
            Self::Solana(c) => c.retrieve(archive.commitment()).unwrap().unwrap(),
        }
    }
}
impl Fixture {
    fn new(chain: MirrorChain) -> Self {
        let directory = std::env::temp_dir().join(format!(
            "lx-repair-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&directory).unwrap();
        fs::set_permissions(&directory, fs::Permissions::from_mode(0o700)).unwrap();
        let eth_key = EthKey::from_bytes((&[0x31; 32]).into()).unwrap();
        let eth_public = eth_key.verifying_key().to_encoded_point(false);
        let eth_payer = Keccak256::digest(&eth_public.as_bytes()[1..])[12..]
            .try_into()
            .unwrap();
        let sol_payer = EdKey::from_bytes(&[0x32; 32]).verifying_key().to_bytes();
        let network = Shared::new(Mutex::new(Network {
            eth_payer,
            sol_payer,
            transactions: BTreeMap::new(),
            broadcasts: Vec::new(),
            signatures: 0,
            observation: Observation::Finalized,
            blockhash_valid: true,
            creation_failure: false,
            recent_blockhash: [0x41; 32],
            manifest: Vec::new(),
            chunks: BTreeMap::new(),
            accounts: BTreeMap::new(),
        }));
        let capture = Shared::new(Mutex::new(Capture {
            root: directory.clone(),
            source: None,
            snapshots: Vec::new(),
        }));
        let rpc = RpcServer::new(&directory, Shared::clone(&network), Shared::clone(&capture));
        let signer = SigningServer::new(
            &directory,
            chain,
            Shared::clone(&network),
            Shared::clone(&capture),
        );
        Self {
            rpc,
            signer,
            network,
            capture,
            chain,
            directory: Directory(directory),
        }
    }
    fn open(&self, directory: &Path) -> Client {
        match self.chain {
            MirrorChain::Ethereum => Client::Ethereum(
                EthereumArchiveClient::open(
                    EthereumProductionConfig {
                        rpc: self.rpc.config.clone(),
                        chain_id: 1,
                        genesis_hash: GENESIS,
                        archive_contract: CONTRACT,
                        archive_code_hash: Keccak256::digest(CODE).into(),
                        first_batch_number: 1,
                        required_confirmations: 2,
                        maximum_reorg_depth: 10,
                        chunk_bytes: 1024,
                        transaction_gas_limit: 100000,
                        maximum_fee_per_gas: 100,
                        maximum_priority_fee_per_gas: 10,
                        journal_directory: directory.to_owned(),
                    },
                    self.signer.client(),
                )
                .unwrap(),
            ),
            MirrorChain::Solana => Client::Solana(
                SolanaArchiveClient::open(
                    SolanaProductionConfig {
                        rpc: self.rpc.config.clone(),
                        genesis_hash: GENESIS,
                        archive_program: PROGRAM,
                        upgradeable_loader: LOADER,
                        program_data_account: PROGRAM_DATA,
                        program_code_hash: Sha256::digest(CODE).into(),
                        first_batch_number: 1,
                        required_rooted_slots: 1,
                        maximum_ancestry: 10,
                        chunk_bytes: 720,
                        journal_directory: directory.to_owned(),
                    },
                    self.signer.client(),
                )
                .unwrap(),
            ),
        }
    }
    fn journal(&self, directory: &Path) -> PublicationJournal {
        PublicationJournal::open(directory, self.chain, 1).unwrap()
    }
    fn snapshots(&self, archive: &Archive) -> Vec<Snapshot> {
        let live = self.directory.0.join("live");
        self.capture.lock().unwrap().source = Some(live.clone());
        let mut client = self.open(&live);
        let stage_count = archive.bytes().len().div_ceil(match self.chain {
            MirrorChain::Ethereum => 1024,
            MirrorChain::Solana => 720,
        }) + 2;
        let mut complete = false;
        for _ in 0..stage_count * 2 {
            let progress = client.advance(archive);
            if progress.phase == PublicationPhase::Pending {
                let network = self.network.lock().unwrap();
                self.capture
                    .lock()
                    .unwrap()
                    .save(&network, PublicationPhase::Pending);
            }
            if progress.phase == PublicationPhase::RetrievedVerified {
                complete = true;
                break;
            }
        }
        assert!(
            complete,
            "production publication must finish within two advances per stage"
        );
        assert_eq!(client.retrieve(archive), archive.bytes());
        drop(client);
        let journal = self.journal(&live);
        assert_eq!(journal.cursor().latest_batch, Some(1));
        assert_eq!(
            journal.cursor().latest_committed,
            archive.data().committed_coordinate()
        );
        drop(journal);
        let mut capture = self.capture.lock().unwrap();
        capture.source = None;
        let snapshots = std::mem::take(&mut capture.snapshots);
        assert_eq!(snapshots.len(), stage_count * 4);
        snapshots
    }
    fn restore(&self, snapshot: &Snapshot) {
        *self.network.lock().unwrap() = snapshot.network.clone();
    }
    fn boundary_record(&self, snapshot: &Snapshot, archive: &Archive) -> PublicationRecord {
        let journal = self.journal(&snapshot.directory);
        let records = journal
            .records()
            .filter(|r| r.phase == snapshot.phase)
            .cloned()
            .collect::<Vec<_>>();
        assert_eq!(records.len(), 1);
        let record = records[0].clone();
        assert_eq!(record.commitment, archive.commitment());
        assert_eq!(record.committed, archive.data().committed_coordinate());
        record
    }
}

fn assert_reopen_boundaries(chain: MirrorChain) {
    let archive = archive();
    let fixture = Fixture::new(chain);
    let snapshots = fixture.snapshots(&archive);
    let mut prepared_stages = Vec::new();
    for snapshot in snapshots {
        let before = fixture.boundary_record(&snapshot, &archive);
        if snapshot.phase == PublicationPhase::Prepared {
            prepared_stages.push(before.stage);
        }
        fixture.restore(&snapshot);
        let resumed_blockhash = [0x43; 32];
        if chain == MirrorChain::Solana && before.phase == PublicationPhase::Prepared {
            fixture.network.lock().unwrap().recent_blockhash = resumed_blockhash;
        }
        let signatures = snapshot.network.signatures;
        let broadcasts = snapshot.network.broadcasts.len();
        let mut client = fixture.open(&snapshot.directory);
        let progress = client.advance(&archive);
        assert_eq!(progress.stage, before.stage);
        if before.phase == PublicationPhase::Prepared {
            assert_eq!(progress.phase, PublicationPhase::Pending);
            let confirmed = client.advance(&archive);
            assert_eq!(confirmed.stage, before.stage);
            assert_eq!(
                confirmed.phase,
                if before.stage == PublicationStage::Finalize {
                    PublicationPhase::RetrievedVerified
                } else {
                    PublicationPhase::Finalized
                }
            );
        } else {
            assert_eq!(
                progress.phase,
                if before.stage == PublicationStage::Finalize {
                    PublicationPhase::RetrievedVerified
                } else {
                    PublicationPhase::Finalized
                }
            );
        }
        {
            let network = fixture.network.lock().unwrap();
            assert_eq!(
                network.signatures,
                signatures + usize::from(before.phase == PublicationPhase::Prepared)
            );
            assert_eq!(
                network.broadcasts.len(),
                broadcasts + usize::from(before.phase != PublicationPhase::Pending)
            );
        }
        drop(client);
        let journal = fixture.journal(&snapshot.directory);
        let recovered = journal
            .record(archive.commitment(), before.stage)
            .unwrap()
            .clone();
        assert_eq!(recovered.stage_payload_digest, before.stage_payload_digest);
        if before.phase != PublicationPhase::Prepared {
            assert_eq!(recovered.signed_payload, before.signed_payload);
            assert_eq!(recovered.transaction, before.transaction);
        }
        let network = fixture.network.lock().unwrap();
        let transaction = network
            .transactions
            .values()
            .find(|tx| tx.raw == recovered.signed_payload)
            .unwrap();
        assert_eq!(
            network
                .transactions
                .values()
                .filter(|tx| tx.stage == before.stage)
                .count(),
            1
        );
        assert_eq!(transaction.stage, before.stage);
        assert_eq!(
            &Sha256::digest(&transaction.data)[..],
            recovered.stage_payload_digest
        );
        let commitment = match chain {
            MirrorChain::Ethereum => &transaction.data[4..36],
            MirrorChain::Solana => &transaction.data[7..39],
        };
        assert_eq!(commitment, archive.commitment().as_bytes());
        if chain == MirrorChain::Solana && before.phase == PublicationPhase::Prepared {
            let message = &recovered.signed_payload[65..];
            let mut offset = 3;
            let keys = short(message, &mut offset);
            assert_eq!(
                &message[offset + keys * 32..offset + keys * 32 + 32],
                resumed_blockhash
            );
        }
    }
    assert_eq!(prepared_stages.first(), Some(&PublicationStage::Manifest));
    assert_eq!(prepared_stages.last(), Some(&PublicationStage::Finalize));
    assert!(prepared_stages.contains(&PublicationStage::Chunk(0)));
}

#[test]
fn ethereum_reopens_prepared_signed_and_broadcast_stages() {
    assert_reopen_boundaries(MirrorChain::Ethereum);
}
#[test]
fn solana_reopens_prepared_signed_and_broadcast_stages() {
    assert_reopen_boundaries(MirrorChain::Solana);
}

#[test]
fn pending_disappearance_preserves_ambiguity_on_both_chains() {
    for chain in [MirrorChain::Ethereum, MirrorChain::Solana] {
        let archive = archive();
        let fixture = Fixture::new(chain);
        let snapshots = fixture.snapshots(&archive);
        let snapshot = snapshots
            .iter()
            .find(|s| s.phase == PublicationPhase::Pending)
            .unwrap();
        let before = fixture.boundary_record(snapshot, &archive);
        fixture.restore(snapshot);
        fixture.network.lock().unwrap().observation = Observation::Missing;
        for _ in 0..2 {
            let mut client = fixture.open(&snapshot.directory);
            let progress = client.advance(&archive);
            assert_eq!(progress.phase, PublicationPhase::BroadcastUnknown);
            drop(client);
            let mut journal = fixture.journal(&snapshot.directory);
            let recovered = journal
                .record(archive.commitment(), before.stage)
                .unwrap()
                .clone();
            assert_eq!(recovered.transaction, before.transaction);
            assert_eq!(recovered.signed_payload, before.signed_payload);
            let mut unverified_rebroadcast = recovered.clone();
            unverified_rebroadcast.phase = PublicationPhase::Signed;
            assert_eq!(
                journal.append(unverified_rebroadcast),
                Err(StoreError::InvalidTransition)
            );
            let mut replacement = recovered;
            replacement.signed_payload.push(0);
            assert_eq!(journal.append(replacement), Err(StoreError::Conflict));
        }
        let network = fixture.network.lock().unwrap();
        assert_eq!(network.broadcasts, snapshot.network.broadcasts);
        assert_eq!(network.signatures, snapshot.network.signatures);
        drop(network);
        fixture.network.lock().unwrap().observation = Observation::Finalized;
        let mut client = fixture.open(&snapshot.directory);
        assert_eq!(client.advance(&archive).phase, PublicationPhase::Finalized);
        drop(client);
        assert_eq!(
            fixture
                .journal(&snapshot.directory)
                .record(archive.commitment(), before.stage)
                .unwrap()
                .phase,
            PublicationPhase::Finalized
        );
    }
}

#[test]
fn solana_pending_expiry_requires_history_before_replacement() {
    let archive = archive();
    let fixture = Fixture::new(MirrorChain::Solana);
    let snapshots = fixture.snapshots(&archive);
    let snapshot = snapshots
        .iter()
        .find(|s| s.phase == PublicationPhase::Pending)
        .unwrap();
    let before = fixture.boundary_record(snapshot, &archive);
    fixture.restore(snapshot);
    {
        let mut network = fixture.network.lock().unwrap();
        network.observation = Observation::Missing;
        network.blockhash_valid = false;
        network.recent_blockhash = [0x42; 32];
    }
    let mut client = fixture.open(&snapshot.directory);
    assert_eq!(
        client.advance(&archive).phase,
        PublicationPhase::BroadcastExpired
    );
    drop(client);
    {
        let mut journal = fixture.journal(&snapshot.directory);
        let expired = journal
            .record(archive.commitment(), before.stage)
            .unwrap()
            .clone();
        assert_eq!(expired.transaction, before.transaction);
        assert_eq!(expired.signed_payload, before.signed_payload);
        let mut wrong_stage = expired;
        wrong_stage.phase = PublicationPhase::Signed;
        wrong_stage.stage_payload_digest[0] ^= 1;
        assert_eq!(journal.append(wrong_stage), Err(StoreError::Conflict));
    }
    assert_eq!(
        fixture.network.lock().unwrap().broadcasts,
        snapshot.network.broadcasts
    );
    assert_eq!(
        fixture.network.lock().unwrap().signatures,
        snapshot.network.signatures
    );
    fixture.network.lock().unwrap().observation = Observation::Pending;
    let mut client = fixture.open(&snapshot.directory);
    assert_eq!(client.advance(&archive).phase, PublicationPhase::Pending);
    drop(client);
    let replacement = fixture
        .journal(&snapshot.directory)
        .record(archive.commitment(), before.stage)
        .unwrap()
        .clone();
    assert_ne!(replacement.transaction, before.transaction);
    assert_ne!(replacement.signed_payload, before.signed_payload);
    assert_eq!(
        replacement.stage_payload_digest,
        before.stage_payload_digest
    );
    let message = &replacement.signed_payload[65..];
    let mut offset = 3;
    let keys = short(message, &mut offset);
    assert_eq!(
        &message[offset + keys * 32..offset + keys * 32 + 32],
        [0x42; 32]
    );
    assert_eq!(
        fixture.network.lock().unwrap().signatures,
        snapshot.network.signatures + 1
    );
    assert_eq!(
        fixture.network.lock().unwrap().broadcasts.len(),
        snapshot.network.broadcasts.len() + 1
    );
    fixture.network.lock().unwrap().observation = Observation::Finalized;
    let mut client = fixture.open(&snapshot.directory);
    assert_eq!(client.advance(&archive).phase, PublicationPhase::Finalized);
    drop(client);
    assert_eq!(
        fixture
            .journal(&snapshot.directory)
            .record(archive.commitment(), before.stage)
            .unwrap()
            .transaction,
        replacement.transaction
    );
}

#[test]
fn ethereum_pending_canonical_replacement_reuses_signed_identity() {
    let archive = archive();
    let fixture = Fixture::new(MirrorChain::Ethereum);
    let snapshots = fixture.snapshots(&archive);
    let snapshot = snapshots
        .iter()
        .find(|s| s.phase == PublicationPhase::Pending)
        .unwrap();
    let before = fixture.boundary_record(snapshot, &archive);
    fixture.restore(snapshot);
    fixture.network.lock().unwrap().observation = Observation::CanonicalReplacement;
    let mut client = fixture.open(&snapshot.directory);
    assert_eq!(client.advance(&archive).phase, PublicationPhase::Reorged);
    drop(client);
    {
        let mut journal = fixture.journal(&snapshot.directory);
        let reorged = journal
            .record(archive.commitment(), before.stage)
            .unwrap()
            .clone();
        assert_eq!(
            reorged.position,
            FinalityPosition::Ethereum {
                block_number: 10,
                block_hash: BLOCK
            }
        );
        let mut wrong_identity = reorged;
        wrong_identity.phase = PublicationPhase::Signed;
        wrong_identity.transaction = TransactionIdentity::Ethereum([0x99; 32]);
        assert_eq!(journal.append(wrong_identity), Err(StoreError::Conflict));
    }
    assert_eq!(
        fixture.network.lock().unwrap().broadcasts,
        snapshot.network.broadcasts
    );
    fixture.network.lock().unwrap().observation = Observation::Finalized;
    let mut client = fixture.open(&snapshot.directory);
    assert_eq!(client.advance(&archive).phase, PublicationPhase::Finalized);
    drop(client);
    let recovered = fixture
        .journal(&snapshot.directory)
        .record(archive.commitment(), before.stage)
        .unwrap()
        .clone();
    assert_eq!(recovered.transaction, before.transaction);
    assert_eq!(recovered.signed_payload, before.signed_payload);
    let network = fixture.network.lock().unwrap();
    assert_eq!(network.signatures, snapshot.network.signatures);
    assert_eq!(
        network.broadcasts.len(),
        snapshot.network.broadcasts.len() + 1
    );
    assert_eq!(network.broadcasts.last(), Some(&before.signed_payload));
}

#[test]
fn prepared_payload_mismatch_is_refused_before_signing() {
    for chain in [MirrorChain::Ethereum, MirrorChain::Solana] {
        let archive = archive();
        let fixture = Fixture::new(chain);
        let snapshots = fixture.snapshots(&archive);
        let snapshot = snapshots
            .iter()
            .find(|s| s.phase == PublicationPhase::Prepared)
            .unwrap();
        let mut before = fixture.boundary_record(snapshot, &archive);
        fixture.restore(snapshot);
        // An independently valid journal with the wrong payload commitment must
        // never authorize signing a different reconstructed stage.
        let corrupt = fixture.directory.0.join("wrong-payload");
        let mut journal = fixture.journal(&corrupt);
        before.stage_payload_digest[0] ^= 1;
        journal.append(before).unwrap();
        drop(journal);
        let mut client = fixture.open(&corrupt);
        match &mut client {
            Client::Ethereum(c) => assert_eq!(c.advance(&archive), Err(EthereumError::Transaction)),
            Client::Solana(c) => assert_eq!(c.advance(&archive), Err(SolanaError::Transaction)),
        }
        drop(client);
        let network = fixture.network.lock().unwrap();
        assert_eq!(network.signatures, snapshot.network.signatures);
        assert_eq!(network.broadcasts, snapshot.network.broadcasts);
    }
}

#[test]
fn pre_broadcast_failure_reopens_both_clients() {
    for chain in [MirrorChain::Ethereum, MirrorChain::Solana] {
        let archive = archive();
        let fixture = Fixture::new(chain);
        let path = fixture.directory.0.join("failed-creation");
        fixture.network.lock().unwrap().creation_failure = true;
        let mut client = fixture.open(&path);
        match &mut client {
            Client::Ethereum(c) => assert_eq!(c.advance(&archive), Err(EthereumError::Transaction)),
            Client::Solana(c) => assert_eq!(c.advance(&archive), Err(SolanaError::Blockhash)),
        }
        drop(client);
        let before = fixture
            .journal(&path)
            .record(archive.commitment(), PublicationStage::Manifest)
            .unwrap()
            .clone();
        assert_eq!(before.phase, PublicationPhase::PreBroadcastFailure);
        assert_eq!(before.transaction, TransactionIdentity::None);
        assert!(fixture.network.lock().unwrap().broadcasts.is_empty());
        fixture.network.lock().unwrap().creation_failure = false;
        let mut client = fixture.open(&path);
        assert_eq!(client.advance(&archive).phase, PublicationPhase::Pending);
        drop(client);
        let recovered = fixture
            .journal(&path)
            .record(archive.commitment(), before.stage)
            .unwrap()
            .clone();
        assert_eq!(recovered.stage_payload_digest, before.stage_payload_digest);
        assert_eq!(recovered.commitment, before.commitment);
        assert_eq!(recovered.committed, before.committed);
        assert_eq!(fixture.network.lock().unwrap().broadcasts.len(), 1);
    }
}
