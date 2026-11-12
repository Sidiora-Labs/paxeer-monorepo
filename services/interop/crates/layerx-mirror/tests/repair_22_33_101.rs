mod test_network {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/../test_network.rs"));
}

// Task 22.33.101: finding/test mapping and out-of-scope wiring are recorded in
// src/repair_22_33_101.kvx. The peer below is the committed Solana loopback
// transcript used by repair_22_31_166; the component under repair is the real
// publisher, journal, remote signer client, source reader and verifier.

use layerx_mirror_signer as mirror_signer_service;

use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{OpenOptionsExt, PermissionsExt};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc as Shared, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine as _;
use ed25519_dalek::SigningKey as EdKey;
use k256::ecdsa::SigningKey as EthKey;
use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::rpc::{RpcEndpointConfig, RpcQuorumConfig};
use layerx_mirror::signer::{
    ChainSignature, RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SignerError,
    SigningAlgorithm,
};
use layerx_mirror::solana::{SolanaArchiveClient, SolanaMirrorReadConfig, SolanaProductionConfig};
use layerx_mirror::source::{
    MirrorLocator, MirrorReadPolicy, MirrorSource, MirrorSourceId, MirrorSources,
};
use layerx_mirror::store::{
    ArchiveSpool, FinalityPosition, MirrorChain, PublicationJournal, PublicationPhase,
    PublicationRecord, PublicationStage, StoreError, TransactionIdentity,
};
use layerx_mirror::{Archive, MirrorEvidenceLevel, MirrorVerifier, MirrorVerifyError, NodeHead};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::ReceiptCheck;
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

static NEXT: AtomicU64 = AtomicU64::new(0);
const PROGRAM: [u8; 32] = [0x22; 32];
const LOADER: [u8; 32] = [0x23; 32];
const PROGRAM_DATA: [u8; 32] = [0x24; 32];
const CODE: &[u8] = b"immutable recorded archive program";
const GENESIS: [u8; 32] = [0x25; 32];
const BLOCK: [u8; 32] = [0x26; 32];

fn archive() -> Archive {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
        .or_panic("expected Ok value");
    let header =
        CommittedHeader::from_block_json(&fixture["committedBlock"]).or_panic("expected Ok value");
    let receipts = fixture["receipts"]
        .as_array()
        .or_panic("expected Some value")
        .iter()
        .map(|value| unhex(value["receipt"].as_str().or_panic("expected Some value")))
        .collect();
    let height = header.height;
    Archive::from_committed(
        &CommittedBlock { header, receipts },
        77,
        NodeHead {
            latest_sealed_batch: height,
            latest_committed: None,
        },
    )
    .or_panic("expected Ok value")
}

fn hex(bytes: &[u8]) -> String {
    let mut text = String::from("0x");
    for byte in bytes {
        use std::fmt::Write as _;
        write!(text, "{byte:02x}").or_panic("expected Ok value");
    }
    text
}
fn unhex(text: &str) -> Vec<u8> {
    let digits = text.strip_prefix("0x").or_panic("expected Some value");
    (0..digits.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&digits[i..i + 2], 16).or_panic("expected Ok value"))
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
    let mut result = String::from_utf8(vec![b'1'; zeros]).or_panic("expected Ok value");
    for digit in digits.iter().rev() {
        result.push(char::from(ALPHABET[usize::from(*digit)]));
    }
    result
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
struct RpcServer {
    config: RpcQuorumConfig,
    running: Shared<AtomicBool>,
    addresses: Vec<SocketAddr>,
    threads: Vec<JoinHandle<()>>,
}
fn create_test_ca(
    key: &PKey<openssl::pkey::Private>,
) -> (X509, openssl::x509::X509Name, Asn1Time, Asn1Time) {
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
    ca.set_pubkey(key)
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
    ca.sign(key, MessageDigest::sha256())
        .unwrap_or_else(|e| panic!("CA signature: {e}"));
    let ca = ca.build();
    (ca, name, before, after)
}

fn make_tls_acceptor(directory: &Path) -> (Shared<SslAcceptor>, PathBuf) {
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap_or_else(|e| panic!("RSA: {e}")))
        .unwrap_or_else(|e| panic!("key: {e}"));
    let (ca, name, before, after) = create_test_ca(&key);
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
    let mut acceptor =
        SslAcceptor::mozilla_intermediate(SslMethod::tls()).unwrap_or_else(|e| panic!("TLS: {e}"));
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
    (acceptor, ca_path)
}

impl RpcServer {
    fn new(directory: &Path, network: &Shared<Mutex<Network>>) -> Self {
        let token_path = directory.join("bearer");
        let mut token = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&token_path)
            .or_panic("expected Ok value");
        token
            .write_all(b"repair-loopback-test")
            .or_panic("expected Ok value");
        let (acceptor, ca_path) = make_tls_acceptor(directory);
        let running = Shared::new(AtomicBool::new(true));
        let mut addresses = Vec::new();
        let mut endpoints = Vec::new();
        let mut threads = Vec::new();
        for index in 0..2 {
            let listener = TcpListener::bind("127.0.0.1:0").or_panic("expected Ok value");
            let address = listener.local_addr().or_panic("expected Ok value");
            addresses.push(address);
            endpoints.push(RpcEndpointConfig {
                url: format!("https://{address}/rpc"),
                ca_certificate_der: ca_path.clone(),
                bearer_token_file: token_path.clone(),
                independent_backend: format!("repair-chain-{index}"),
            });
            let acceptor = Shared::clone(&acceptor);
            let running = Shared::clone(&running);
            let network = Shared::clone(network);
            threads.push(thread::spawn(move || {
                while running.load(Ordering::Acquire) {
                    let (tcp,_) = listener.accept().or_panic("expected Ok value");
                    if !running.load(Ordering::Acquire) { break; }
                    tcp.set_read_timeout(Some(Duration::from_secs(3))).or_panic("expected Ok value");
                    tcp.set_write_timeout(Some(Duration::from_secs(3))).or_panic("expected Ok value");
                    let mut stream = acceptor.accept(tcp).or_panic("expected Ok value");
                    let mut head=Vec::new();
                    while !head.ends_with(b"\r\n\r\n") { let mut byte=[0]; stream.read_exact(&mut byte).or_panic("expected Ok value"); head.push(byte[0]); assert!(head.len() < 32768); }
                    let head=String::from_utf8(head).or_panic("expected Ok value");
                    assert!(head.starts_with("POST /rpc HTTP/1.1\r\n"));
                    assert!(head.lines().any(|line| line == "Authorization: Bearer repair-loopback-test"));
                    let length:usize=head.lines().find_map(|line| line.strip_prefix("Content-Length: ")).or_panic("expected Some value").parse().or_panic("expected Ok value");
                    assert!(length < 512*1024); let mut body=vec![0;length]; stream.read_exact(&mut body).or_panic("expected Ok value");
                    let request: Value=serde_json::from_slice(&body).or_panic("expected Ok value");
                    let result=network.lock().or_panic("expected Ok value").respond(&request);
                    let response=serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":request["id"], "result":result})).or_panic("expected Ok value");
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).or_panic("expected Ok value");
                    stream.write_all(&response).or_panic("expected Ok value"); stream.flush().or_panic("expected Ok value"); stream.shutdown().or_panic("expected Ok value");
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

#[derive(Clone)]
struct Transaction {
    raw: Vec<u8>,
    data: Vec<u8>,
    stage: PublicationStage,
}
struct Network {
    sol_payer: [u8; 32],
    transactions: BTreeMap<String, Transaction>,
    broadcasts: Vec<Vec<u8>>,
    blockhash_valid: bool,
    creation_failure: bool,
    recent_blockhash: [u8; 32],
    chunks: BTreeMap<u32, Vec<u8>>,
    accounts: BTreeMap<String, Vec<u8>>,
}
impl Network {
    fn respond(&mut self, request: &Value) -> Value {
        let params = &request["params"];
        match request["method"].as_str().or_panic("expected Some value") {
            "getGenesisHash" => json!(base58(&GENESIS)),
            "getAccountInfo" => {
                let address = params[0].as_str().or_panic("expected Some value");
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
                    return json!({"context":{"slot":11},"value":null});
                };
                json!({"context":{"slot":11},"value":{"owner":base58(&owner),"executable":executable,"data":[BASE64.encode(data),"base64"]}})
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
            "sendTransaction" => self.send_transaction(params),
            "getSignatureStatuses" => {
                assert_eq!(params[1]["searchTransactionHistory"], true);
                let status = json!({"err":null,"confirmationStatus":"finalized"});
                json!({"value":[status]})
            }
            "getTransaction" => {
                let id = params[0].as_str().or_panic("expected Some value");
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
    fn send_transaction(&mut self, params: &Value) -> Value {
        let raw = BASE64
            .decode(params[0].as_str().or_panic("expected Some value"))
            .or_panic("expected Ok value");
        assert_eq!(raw[0], 1);
        let signature =
            ed25519_dalek::Signature::from_slice(&raw[1..65]).or_panic("expected Ok value");
        ed25519_dalek::VerifyingKey::from_bytes(&self.sol_payer)
            .or_panic("expected Ok value")
            .verify_strict(&raw[65..], &signature)
            .or_panic("expected Ok value");
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
            2 => PublicationStage::Chunk(
                u32::try_from(number(&data[39..43])).unwrap_or_else(|_| panic!("value fits u32")),
            ),
            3 => PublicationStage::Finalize,
            _ => panic!("instruction"),
        };
        self.broadcasts.push(raw.clone());
        if !self.transactions.contains_key(&id) {
            self.record_solana(&id, raw, data, stage, &keys);
        }
        json!(id)
    }

    fn record_solana(
        &mut self,
        id: &str,
        raw: Vec<u8>,
        data: Vec<u8>,
        stage: PublicationStage,
        keys: &[Vec<u8>],
    ) {
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
                bytes.extend_from_slice(
                    &(u32::try_from(length).unwrap_or_else(|_| panic!("length fits u32")))
                        .to_be_bytes(),
                );
                bytes.extend_from_slice(chunk);
                self.accounts.insert(base58(&keys[2]), bytes);
                self.chunks.insert(index, chunk.to_vec());
                let manifest = self
                    .accounts
                    .get_mut(&manifest_address)
                    .or_panic("expected Some value");
                assert_eq!(number(&manifest[232..236]), index as usize);
                let mut hash = Sha256::new();
                hash.update(&manifest[192..224]);
                hash.update(index.to_be_bytes());
                hash.update(Sha256::digest(chunk));
                hash.update(
                    (u32::try_from(length).unwrap_or_else(|_| panic!("length fits u32")))
                        .to_be_bytes(),
                );
                manifest[192..224].copy_from_slice(&hash.finalize());
                let received = number(&manifest[224..232]) + length;
                manifest[224..232].copy_from_slice(&(received as u64).to_be_bytes());
                manifest[232..236].copy_from_slice(&(index + 1).to_be_bytes());
            }
            PublicationStage::Finalize => {
                let manifest = self
                    .accounts
                    .get_mut(&manifest_address)
                    .or_panic("expected Some value");
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
                manifest[333] = u8::try_from(tail).unwrap_or_else(|_| panic!("value fits u8"));
            }
        }
        self.transactions
            .insert(id.to_owned(), Transaction { raw, data, stage });
    }
}

fn number(bytes: &[u8]) -> usize {
    bytes
        .iter()
        .fold(0, |value, byte| value * 256 + usize::from(*byte))
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        test_network::isolate().or_panic("private test network");
        let directory = std::env::temp_dir().join(format!(
            "lx-2233101-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&directory).or_panic("expected Ok value");
        fs::set_permissions(&directory, fs::Permissions::from_mode(0o700))
            .or_panic("expected Ok value");
        Self(directory)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).or_panic("expected Ok value");
    }
}

struct ActualSigner {
    socket: PathBuf,
    running: Shared<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}
impl ActualSigner {
    fn new(directory: &Path) -> Self {
        let ethereum_key_file = directory.join("ethereum.key");
        let solana_keypair_file = directory.join("solana.json");
        let ethereum_key = hex(&[0x31; 32]);
        fs::write(&ethereum_key_file, &ethereum_key[2..]).or_panic("expected Ok value");
        let solana_key = EdKey::from_bytes(&[0x32; 32]);
        fs::write(
            &solana_keypair_file,
            serde_json::to_vec(&solana_key.to_keypair_bytes().to_vec())
                .or_panic("expected Ok value"),
        )
        .or_panic("expected Ok value");
        for path in [&ethereum_key_file, &solana_keypair_file] {
            fs::set_permissions(path, fs::Permissions::from_mode(0o600))
                .or_panic("expected Ok value");
        }
        let socket = directory.join("signer.sock");
        let listener =
            mirror_signer_service::SignerListener::bind(&mirror_signer_service::Options {
                socket: socket.clone(),
                ethereum_key_file,
                solana_keypair_file,
                ethereum_key_handle: "repair/ethereum".into(),
                solana_key_handle: "repair/solana".into(),
            })
            .or_panic("expected Ok value");
        let running = Shared::new(AtomicBool::new(true));
        let active = Shared::clone(&running);
        let thread = thread::spawn(move || {
            while active.load(Ordering::Acquire) {
                let result = listener.serve_once();
                if active.load(Ordering::Acquire) {
                    result.or_panic("expected Ok value");
                }
            }
        });
        Self {
            socket,
            running,
            thread: Some(thread),
        }
    }
    fn config(&self, algorithm: SigningAlgorithm) -> RemoteSignerConfig {
        let (key_handle, public_key) = match algorithm {
            SigningAlgorithm::Secp256k1Recoverable => (
                "repair/ethereum",
                EthKey::from_bytes((&[0x31; 32]).into())
                    .or_panic("expected Ok value")
                    .verifying_key()
                    .to_encoded_point(true)
                    .as_bytes()
                    .to_vec(),
            ),
            SigningAlgorithm::Ed25519 => (
                "repair/solana",
                EdKey::from_bytes(&[0x32; 32])
                    .verifying_key()
                    .to_bytes()
                    .to_vec(),
            ),
        };
        RemoteSignerConfig {
            endpoint: SignerEndpoint::Uds {
                socket: self.socket.clone(),
            },
            algorithm,
            key_handle: key_handle.into(),
            public_key,
            timeout: Duration::from_secs(3),
        }
    }
    fn client(&self, algorithm: SigningAlgorithm) -> RemoteChainSigner {
        RemoteChainSigner::new(self.config(algorithm)).or_panic("expected Ok value")
    }
}
impl Drop for ActualSigner {
    fn drop(&mut self) {
        self.running.store(false, Ordering::Release);
        // Wake the real blocking listener; its incomplete request is never signed.
        drop(UnixStream::connect(&self.socket));
        let result = self.thread.take().or_panic("expected Some value").join();
        if !thread::panicking() {
            assert!(result.is_ok(), "real signer failed");
        }
    }
}

#[test]
fn finding_14_signer_methods_match_the_actual_decoder() {
    let directory = Directory::new();
    let server = ActualSigner::new(&directory.0);
    let ethereum = server.client(SigningAlgorithm::Secp256k1Recoverable);
    let solana = server.client(SigningAlgorithm::Ed25519);
    let digest: [u8; 32] = Sha256::digest(b"committed digest").into();
    assert!(matches!(
        ethereum.sign_digest(mirror_signer_service::ETHEREUM_POLICY_DOMAIN, digest),
        Ok(ChainSignature::Secp256k1(_))
    ));
    let message = b"complete canonical Solana message";
    assert!(matches!(
        solana.sign_message(mirror_signer_service::SOLANA_POLICY_DOMAIN, message),
        Ok(ChainSignature::Ed25519(_))
    ));
    assert_eq!(
        solana.sign_digest(mirror_signer_service::SOLANA_POLICY_DOMAIN, digest),
        Err(SignerError::Configuration)
    );
    assert_eq!(
        ethereum.sign_message(mirror_signer_service::ETHEREUM_POLICY_DOMAIN, message),
        Err(SignerError::Configuration)
    );
    assert_eq!(
        solana.sign_message(mirror_signer_service::SOLANA_POLICY_DOMAIN, b""),
        Err(SignerError::Configuration)
    );
    assert_eq!(
        solana.sign_message(mirror_signer_service::SOLANA_POLICY_DOMAIN, &[1; 4097]),
        Err(SignerError::Configuration)
    );
    assert!(matches!(
        solana.sign_message(mirror_signer_service::SOLANA_POLICY_DOMAIN, &[1; 4096]),
        Ok(ChainSignature::Ed25519(_))
    ));
    // These reach the real request decoder and policy service; status 1 must
    // remain Refused on both methods, rather than a transport or decode error.
    assert_eq!(
        ethereum.sign_digest(mirror_signer_service::SOLANA_POLICY_DOMAIN, digest),
        Err(SignerError::Refused)
    );
    assert_eq!(
        solana.sign_message(mirror_signer_service::ETHEREUM_POLICY_DOMAIN, message),
        Err(SignerError::Refused)
    );
    let mut config = server.config(SigningAlgorithm::Ed25519);
    config.key_handle = "unknown/handle".into();
    assert_eq!(
        RemoteChainSigner::new(config)
            .or_panic("expected Ok value")
            .sign_message(mirror_signer_service::SOLANA_POLICY_DOMAIN, message),
        Err(SignerError::Refused)
    );
}

struct Fixture {
    rpc: RpcServer,
    signer: ActualSigner,
    network: Shared<Mutex<Network>>,
    directory: Directory,
}
impl Fixture {
    fn new() -> Self {
        let directory = Directory::new();
        let signer = ActualSigner::new(&directory.0);
        let network = Shared::new(Mutex::new(Network {
            sol_payer: EdKey::from_bytes(&[0x32; 32]).verifying_key().to_bytes(),
            transactions: BTreeMap::new(),
            broadcasts: Vec::new(),
            blockhash_valid: true,
            creation_failure: false,
            recent_blockhash: [0x41; 32],
            chunks: BTreeMap::new(),
            accounts: BTreeMap::new(),
        }));
        let rpc = RpcServer::new(&directory.0, &network);
        Self {
            rpc,
            signer,
            network,
            directory,
        }
    }
    fn open(&self) -> SolanaArchiveClient {
        SolanaArchiveClient::open(
            SolanaProductionConfig {
                rpc: self.rpc.config.clone(),
                genesis_hash: GENESIS,
                archive_program: PROGRAM,
                upgradeable_loader: LOADER,
                program_data_account: PROGRAM_DATA,
                program_code_hash: Sha256::digest(CODE).into(),
                first_batch_number: 1000,
                required_rooted_slots: 1,
                maximum_ancestry: 10,
                chunk_bytes: 720,
                journal_directory: self.directory.0.join("journal"),
            },
            self.signer.client(SigningAlgorithm::Ed25519),
        )
        .or_panic("expected Ok value")
    }
    fn publish(&self, archive: &Archive) {
        let mut client = self.open();
        let stage_count = archive.bytes().len().div_ceil(720) + 2;
        let mut complete = false;
        for _ in 0..stage_count * 2 {
            let progress = client.advance(archive).or_panic("expected Ok value");
            if progress.phase == PublicationPhase::RetrievedVerified {
                complete = true;
                break;
            }
        }
        assert!(
            complete,
            "publication must finish in two advances per stage"
        );
        assert_eq!(
            client
                .retrieve(archive.commitment())
                .or_panic("expected Ok value")
                .or_panic("expected Some value"),
            archive.bytes()
        );
        assert_eq!(client.cursor().latest_batch, Some(1000));
        drop(client);
    }
    fn source(&self) -> MirrorSource {
        MirrorSource::solana(
            MirrorSourceId::new("recorded-solana").or_panic("expected Ok value"),
            SolanaMirrorReadConfig {
                rpc: self.rpc.config.clone(),
                genesis_hash: GENESIS,
                archive_program: PROGRAM,
                upgradeable_loader: LOADER,
                program_data_account: PROGRAM_DATA,
                program_code_hash: Sha256::digest(CODE).into(),
                publisher: EdKey::from_bytes(&[0x32; 32]).verifying_key().to_bytes(),
            },
        )
        .or_panic("expected Ok value")
    }
}

fn receipt_keys() -> Vec<SequencerAuthorization> {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
        .or_panic("expected Ok value");
    let key = &fixture["receiptKey"];
    vec![SequencerAuthorization::new(
        unhex(key["keyId"].as_str().or_panic("expected Some value"))
            .try_into()
            .or_panic("expected Ok value"),
        unhex(key["publicKey"].as_str().or_panic("expected Some value"))
            .try_into()
            .or_panic("expected Ok value"),
        key["firstHeight"].as_u64().or_panic("expected Some value"),
        key["lastHeight"].as_u64().or_panic("expected Some value"),
    )]
}

fn assert_v4_reopen_and_v3_refusal(fixture: &Fixture, archive: &Archive) {
    let reopened = fixture.open();
    assert_eq!(
        reopened.cursor().latest_committed,
        archive.data().committed_coordinate()
    );
    assert_eq!(
        reopened
            .retrieve(archive.commitment())
            .or_panic("expected Ok value")
            .or_panic("expected Some value"),
        archive.bytes()
    );
    drop(reopened);
    let mut network = fixture.network.lock().or_panic("expected Ok value");
    let manifest = network
        .accounts
        .values_mut()
        .find(|bytes| bytes.starts_with(b"LXMMAN04"))
        .or_panic("expected Some value");
    manifest[..8].copy_from_slice(b"LXMMAN03");
    drop(network);
    assert!(fixture.open().retrieve(archive.commitment()).is_err());
}

#[test]
fn finding_28_v4_instructions_and_accounts_round_trip() {
    let archive = archive();
    let fixture = Fixture::new();
    fixture.publish(&archive);
    let network = fixture.network.lock().or_panic("expected Ok value");
    let manifest = network
        .accounts
        .values()
        .find(|bytes| bytes.starts_with(b"LXMMAN04"))
        .or_panic("expected Some value");
    assert_eq!(manifest.len(), 334);
    assert_eq!(&manifest[8..40], archive.commitment().as_bytes());
    assert_eq!(&manifest[40..72], &network.sol_payer);
    assert_eq!(&manifest[72..76], &77u32.to_be_bytes());
    assert_eq!(&manifest[76..84], &1000u64.to_be_bytes());
    assert_eq!(&manifest[84..116], &archive.data().committed.block_hash);
    assert_eq!(
        &manifest[116..124],
        &(archive.bytes().len() as u64).to_be_bytes()
    );
    assert_eq!(&manifest[128..160], &Sha256::digest(archive.bytes())[..]);
    assert_eq!(manifest[160..192], manifest[192..224]);
    assert_eq!(manifest[236], 1);
    let count = archive.bytes().len().div_ceil(720);
    assert_eq!(
        &manifest[124..128],
        &(u32::try_from(count).unwrap_or_else(|_| panic!("count fits u32"))).to_be_bytes()
    );
    assert_eq!(&manifest[224..232], &manifest[116..124]);
    assert_eq!(&manifest[232..236], &manifest[124..128]);
    let mut chunks = BTreeMap::new();
    let mut operations = Vec::new();
    for transaction in network.transactions.values() {
        let data = &transaction.data;
        assert_eq!(&data[..6], b"LXMA\0\x04");
        assert_eq!(&data[7..39], archive.commitment().as_bytes());
        match transaction.stage {
            PublicationStage::Manifest => {
                assert_eq!(data.len(), 159);
                assert_eq!(&data[39..], &manifest[72..192]);
                operations.push(1);
            }
            PublicationStage::Chunk(index) => {
                assert_eq!(&data[39..43], &index.to_be_bytes());
                let length = number(&data[43..45]);
                assert_eq!(data.len(), 45 + length + 32);
                let account = network
                    .accounts
                    .values()
                    .find(|bytes| {
                        bytes.starts_with(b"LXMCHK04") && bytes[40..44] == index.to_be_bytes()
                    })
                    .or_panic("expected Some value");
                assert_eq!(&account[8..40], archive.commitment().as_bytes());
                assert_eq!(&account[44..76], &data[45 + length..]);
                assert_eq!(
                    &account[76..80],
                    &(u32::try_from(length).unwrap_or_else(|_| panic!("length fits u32")))
                        .to_be_bytes()
                );
                assert_eq!(&account[80..], &data[45..45 + length]);
                chunks.insert(index, account[80..].to_vec());
                operations.push(2);
            }
            PublicationStage::Finalize => {
                assert_eq!(data.len(), 39);
                operations.push(3);
            }
        }
    }
    assert_eq!(operations.iter().filter(|&&op| op == 1).count(), 1);
    assert_eq!(operations.iter().filter(|&&op| op == 2).count(), count);
    assert_eq!(operations.iter().filter(|&&op| op == 3).count(), 1);
    assert_eq!(
        chunks.values().flatten().copied().collect::<Vec<_>>(),
        archive.bytes()
    );
    drop(network);
    assert_v4_reopen_and_v3_refusal(&fixture, &archive);
}

#[test]
fn finding_24_observed_committed_source_uses_index_and_key_windows() {
    let archive = archive();
    let fixture = Fixture::new();
    fixture.publish(&archive);
    let sources = MirrorSources::new(77, vec![fixture.source()]).or_panic("expected Ok value");
    let observed = sources
        .read(
            1000,
            &MirrorReadPolicy::Exact(MirrorLocator {
                source_index: 0,
                commitment: archive.commitment(),
            }),
        )
        .or_panic("expected Ok value");
    let observation = observed.observation().clone();
    assert_eq!(observed.archive(), archive.bytes());
    let verifier = MirrorVerifier::from_committed_source(observed).or_panic("expected Ok value");
    assert_eq!(
        verifier.committed().or_panic("expected Ok value"),
        archive
            .data()
            .committed_coordinate()
            .or_panic("expected Some value")
    );
    let keys = receipt_keys();
    for (index, canonical) in archive.data().committed.receipts.iter().enumerate() {
        let receipt = verifier
            .committed_receipt(index, &keys)
            .or_panic("expected Ok value");
        assert_eq!(
            layerx_wire::receipt::encode(receipt.value()).or_panic("expected Ok value"),
            *canonical
        );
        assert_eq!(receipt.level(), MirrorEvidenceLevel::Committed);
        assert_eq!(receipt.observation(), Some(&observation));
    }
    assert_eq!(
        verifier.committed_receipt(archive.data().committed.receipts.len(), &keys),
        Err(MirrorVerifyError::ReceiptMissing)
    );
    let key = &keys[0];
    for (first, last) in [(1, 999), (1001, 2000)] {
        let outside = [SequencerAuthorization::new(
            key.sequencer_id(),
            key.public_key(),
            first,
            last,
        )];
        assert_eq!(
            verifier.committed_receipt(0, &outside),
            Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
        );
    }
    let wrong = [SequencerAuthorization::new(
        key.sequencer_id(),
        [9; 32],
        1000,
        1000,
    )];
    assert_eq!(
        verifier.committed_receipt(0, &wrong),
        Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
    );
    assert_eq!(
        verifier.committed_receipt(0, &[]),
        Err(MirrorVerifyError::Receipt(ReceiptCheck::SequencerSignature))
    );
    let exact = [SequencerAuthorization::new(
        key.sequencer_id(),
        key.public_key(),
        1000,
        1000,
    )];
    assert_eq!(
        verifier
            .committed_receipt(0, &exact)
            .or_panic("expected Ok value")
            .level(),
        MirrorEvidenceLevel::Committed
    );
}

struct JournalHistory {
    records: Vec<PublicationRecord>,
    logs: Vec<Vec<u8>>,
    heads: Vec<Vec<u8>>,
    archive: Archive,
}
impl JournalHistory {
    fn new() -> Self {
        // Obtain an exact signed transaction from the production publisher and
        // real signer, then persist its transitions through the public store.
        let archive = archive();
        let fixture = Fixture::new();
        fixture.publish(&archive);
        let source = PublicationJournal::open(
            fixture.directory.0.join("journal"),
            MirrorChain::Solana,
            1000,
        )
        .or_panic("expected Ok value");
        let finalized = source
            .record(archive.commitment(), PublicationStage::Finalize)
            .or_panic("expected Some value")
            .clone();
        assert_eq!(finalized.phase, PublicationPhase::RetrievedVerified);
        assert!(!finalized.signed_payload.is_empty());
        drop(source);
        let directory = Directory::new();
        let mut journal = PublicationJournal::open(&directory.0, MirrorChain::Solana, 1000)
            .or_panic("expected Ok value");
        let mut logs =
            vec![fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value")];
        let mut heads =
            vec![fs::read(directory.0.join("solana.head")).or_panic("expected Ok value")];
        let mut records = Vec::new();
        for phase in [
            PublicationPhase::Prepared,
            PublicationPhase::Signed,
            PublicationPhase::Pending,
            PublicationPhase::Finalized,
            PublicationPhase::RetrievedVerified,
        ] {
            let mut record = finalized.clone();
            record.phase = phase;
            if phase == PublicationPhase::Prepared {
                record.signed_payload.clear();
                record.transaction = TransactionIdentity::None;
            }
            if matches!(
                phase,
                PublicationPhase::Prepared | PublicationPhase::Signed | PublicationPhase::Pending
            ) {
                record.position = FinalityPosition::None;
            }
            journal.append(record.clone()).or_panic("expected Ok value");
            records.push(record);
            logs.push(fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"));
            heads.push(fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"));
        }
        Self {
            records,
            logs,
            heads,
            archive,
        }
    }
    fn install(log: &[u8], head: &[u8]) -> Directory {
        let directory = Directory::new();
        durable_write(&directory.0.join("solana.journal"), log);
        durable_write(&directory.0.join("solana.head"), head);
        directory
    }
}
fn durable_write(path: &Path, bytes: &[u8]) {
    let mut file = fs::File::create(path).or_panic("expected Ok value");
    file.write_all(bytes).or_panic("expected Ok value");
    file.sync_all().or_panic("expected Ok value");
}
fn open_journal(directory: &Path) -> Result<PublicationJournal, StoreError> {
    PublicationJournal::open(directory, MirrorChain::Solana, 1000)
}
fn recover_journal(directory: &Path) -> Result<PublicationJournal, StoreError> {
    PublicationJournal::recover_unsealed_suffix(directory, MirrorChain::Solana, 1000)
}
fn assert_record(journal: &PublicationJournal, record: &PublicationRecord) {
    assert_eq!(
        journal.record(record.commitment, record.stage),
        Some(record)
    );
}

#[test]
fn finding_12_every_incomplete_frame_suffix_requires_explicit_durable_recovery() {
    let history = JournalHistory::new();
    // Both the first frame after an empty sealed head and a signed transaction
    // after Prepared are cut at every byte of sequence, length, body and digest.
    for boundary in [0, 1] {
        let prefix = &history.logs[boundary];
        let complete = &history.logs[boundary + 1];
        for cut in prefix.len() + 1..complete.len() {
            let crashed = &complete[..cut];
            let directory = JournalHistory::install(crashed, &history.heads[boundary]);
            assert_eq!(
                open_journal(&directory.0).err(),
                Some(StoreError::RecoveryRequired {
                    valid_bytes: prefix.len() as u64,
                    trailing_bytes: (cut - prefix.len()) as u64,
                }),
                "ordinary open must not silently discard a suffix at {cut}"
            );
            assert_eq!(
                fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
                crashed
            );
            assert_eq!(
                fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
                history.heads[boundary]
            );
            let recovered = recover_journal(&directory.0).or_panic("expected Ok value");
            if boundary == 0 {
                assert_eq!(recovered.records().count(), 0);
            } else {
                assert_record(&recovered, &history.records[boundary - 1]);
            }
            assert_eq!(recovered.cursor().latest_batch, None);
            drop(recovered);
            assert_eq!(
                fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
                *prefix
            );
            assert_eq!(
                fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
                history.heads[boundary]
            );
            let mut reopened = open_journal(&directory.0).or_panic("expected Ok value");
            reopened
                .append(history.records[boundary].clone())
                .or_panic("expected Ok value");
            assert_record(&reopened, &history.records[boundary]);
            drop(reopened);
            let reopened = open_journal(&directory.0).or_panic("expected Ok value");
            assert_record(&reopened, &history.records[boundary]);
            assert_eq!(
                fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
                *complete
            );
        }
    }
    // A complete unsealed signed transaction must survive even if the following
    // transition was interrupted. Recovery seals it before another append.
    let cut = history.logs[2].len() + 13;
    let directory = JournalHistory::install(&history.logs[3][..cut], &history.heads[1]);
    assert_eq!(
        open_journal(&directory.0).err(),
        Some(StoreError::RecoveryRequired {
            valid_bytes: history.logs[2].len() as u64,
            trailing_bytes: 13,
        })
    );
    let recovered = recover_journal(&directory.0).or_panic("expected Ok value");
    assert_record(&recovered, &history.records[1]);
    drop(recovered);
    assert_eq!(
        fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
        history.heads[2]
    );
    assert_eq!(
        fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
        history.logs[2]
    );
    let mut reopened = open_journal(&directory.0).or_panic("expected Ok value");
    for record in &history.records[2..] {
        reopened
            .append(record.clone())
            .or_panic("expected Ok value");
    }
    assert_eq!(reopened.cursor().latest_batch, Some(1000));
    assert_eq!(
        reopened.cursor().latest_committed,
        history.archive.data().committed_coordinate()
    );
}

#[test]
fn finding_12_interrupted_head_writes_retain_complete_signed_frames() {
    let history = JournalHistory::new();
    for cut in 0..=history.heads[2].len() {
        let directory = JournalHistory::install(&history.logs[2], &history.heads[1]);
        // write_head writes this temporary file, syncs it, and renames only when
        // complete. A crash during any temporary write leaves the old seal valid.
        durable_write(
            &directory.0.join(".solana.head.tmp"),
            &history.heads[2][..cut],
        );
        let journal = open_journal(&directory.0).or_panic("expected Ok value");
        assert_record(&journal, &history.records[1]);
        drop(journal);
        assert_eq!(
            fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
            history.logs[2]
        );
        assert_eq!(
            fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
            history.heads[2]
        );
        let reopened = open_journal(&directory.0).or_panic("expected Ok value");
        assert_record(&reopened, &history.records[1]);
    }
    // A complete renamed head after a crash remains valid and does not lose data.
    let directory = JournalHistory::install(&history.logs[2], &history.heads[2]);
    let recovered = recover_journal(&directory.0).or_panic("expected Ok value");
    assert_record(&recovered, &history.records[1]);
    drop(recovered);
    assert_eq!(
        fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
        history.heads[2]
    );
}

#[test]
fn finding_12_authenticated_damage_and_rollback_are_never_truncated() {
    let history = JournalHistory::new();
    let mut damaged = Vec::new();
    // Damage each sealed field, including body, link and independent head.
    for offset in [0, 8, 12, history.logs[1].len() - 1] {
        let mut log = history.logs[2].clone();
        log[offset] ^= 1;
        damaged.push((log, history.heads[1].clone()));
    }
    // A complete unsealed frame with a bad digest is corruption, not a crash suffix.
    let mut log = history.logs[2].clone();
    *log.last_mut().or_panic("expected Some value") ^= 1;
    damaged.push((log, history.heads[1].clone()));
    // Prefix rollback and damage in the independent head must also be refused.
    damaged.push((history.logs[1].clone(), history.heads[2].clone()));
    for offset in [0, 8, 16, 24, 55] {
        let mut head = history.heads[2].clone();
        head[offset] ^= 1;
        damaged.push((history.logs[2].clone(), head));
    }
    for cut in 0..history.heads[2].len() {
        damaged.push((history.logs[2].clone(), history.heads[2][..cut].to_vec()));
    }
    // The independent seal cannot authorize a partial frame inside its prefix.
    for cut in [1, 8, 12, history.logs[2].len() - 1] {
        damaged.push((history.logs[2][..cut].to_vec(), history.heads[2].clone()));
    }
    for (log, head) in damaged {
        let directory = JournalHistory::install(&log, &head);
        assert!(open_journal(&directory.0).is_err());
        assert!(recover_journal(&directory.0).is_err());
        assert_eq!(
            fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
            log
        );
        assert_eq!(
            fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
            head
        );
    }
    // Without a head no incomplete log has independent recovery authority.
    let directory = JournalHistory::install(
        &history.logs[2][..history.logs[2].len() - 1],
        &history.heads[1],
    );
    fs::remove_file(directory.0.join("solana.head")).or_panic("expected Ok value");
    let before = fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value");
    assert!(recover_journal(&directory.0).is_err());
    assert_eq!(
        fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
        before
    );
    assert!(!directory.0.join("solana.head").exists());
}

#[test]
fn finding_26_v3_retained_state_round_trips_and_incompatible_state_is_preserved() {
    let history = JournalHistory::new();
    let directory = JournalHistory::install(
        history.logs.last().or_panic("expected Some value"),
        history.heads.last().or_panic("expected Some value"),
    );
    let journal = open_journal(&directory.0).or_panic("expected Ok value");
    assert_record(
        &journal,
        history.records.last().or_panic("expected Some value"),
    );
    assert_eq!(journal.cursor().latest_batch, Some(1000));
    assert_eq!(
        journal.cursor().latest_committed,
        history.archive.data().committed_coordinate()
    );
    drop(journal);
    let head = history.heads.last().or_panic("expected Some value");
    let log = history.logs.last().or_panic("expected Some value");
    assert_eq!(head.len(), 56);
    assert_eq!(&head[..8], b"LXMHED03");
    assert_eq!(&head[8..16], &5u64.to_be_bytes());
    assert_eq!(&head[16..24], &(log.len() as u64).to_be_bytes());
    assert_eq!(&head[24..], &log[log.len() - 32..]);
    assert_eq!(&log[12..20], b"LXMJNL03");
    assert_eq!(&log[20..22], &3u16.to_be_bytes());
    assert_eq!(log[22], 2); // Solana lane
    assert_eq!(&log[23..31], &1000u64.to_be_bytes());
    assert_eq!(&log[31..63], history.archive.commitment().as_bytes());
    assert_eq!(log[63], 1);
    assert_eq!(&log[64..72], &1000u64.to_be_bytes());
    assert_eq!(&log[72..104], &history.archive.data().committed.block_hash);
    let spool_path = directory.0.join("spool");
    let spool = ArchiveSpool::open(&spool_path).or_panic("expected Ok value");
    let node_head = NodeHead {
        latest_sealed_batch: 1000,
        latest_committed: history.archive.data().committed_coordinate(),
    };
    spool
        .put(
            history.archive.commitment(),
            history.archive.bytes(),
            node_head,
        )
        .or_panic("expected Ok value");
    drop(spool);
    let reopened = ArchiveSpool::open(&spool_path).or_panic("expected Ok value");
    let saved = reopened
        .get(history.archive.commitment())
        .or_panic("expected Ok value");
    assert_eq!(saved.bytes, history.archive.bytes());
    assert_eq!(saved.node_head, node_head);
    drop(reopened);
    let spool_file = fs::read_dir(&spool_path)
        .or_panic("expected Ok value")
        .map(|entry| entry.or_panic("expected Ok value").path())
        .find(|path| path.extension().and_then(|extension| extension.to_str()) == Some("archive"))
        .or_panic("expected Some value");
    let spool_bytes = fs::read(&spool_file).or_panic("expected Ok value");
    assert_eq!(&spool_bytes[..8], b"LXMSPL03");
    assert_eq!(&spool_bytes[8..16], &1000u64.to_be_bytes());
    assert_eq!(spool_bytes[16], 1);
    assert_eq!(&spool_bytes[17..25], &1000u64.to_be_bytes());
    assert_eq!(
        &spool_bytes[25..57],
        &history.archive.data().committed.block_hash
    );

    assert_incompatible_retained_state(
        &history,
        &directory,
        head,
        log,
        &spool_path,
        &spool_file,
        spool_bytes,
    );
}

fn assert_incompatible_retained_state(
    history: &JournalHistory,
    directory: &Directory,
    head: &[u8],
    log: &[u8],
    spool_path: &Path,
    spool_file: &Path,
    spool_bytes: Vec<u8>,
) {
    // Incompatible retained bytes are never implicitly migrated or deleted.
    let mut old_head = head.to_vec();
    old_head[..8].copy_from_slice(b"LXMHED02");
    durable_write(&directory.0.join("solana.head"), &old_head);
    assert!(open_journal(&directory.0).is_err());
    assert!(recover_journal(&directory.0).is_err());
    assert_eq!(
        fs::read(directory.0.join("solana.head")).or_panic("expected Ok value"),
        old_head
    );
    assert_eq!(
        fs::read(directory.0.join("solana.journal")).or_panic("expected Ok value"),
        log
    );
    // Give an old journal a valid hash link to isolate version refusal from
    // checksum refusal, and leave its independent seal at the empty origin.
    let mut old_log = history.logs[1].clone();
    old_log[12..20].copy_from_slice(b"LXMJNL02");
    old_log[20..22].copy_from_slice(&2u16.to_be_bytes());
    let end = old_log.len() - 32;
    let mut hash = Sha256::new();
    hash.update(b"LayerX/mirror/journal-record/v2\0");
    hash.update([0; 32]);
    hash.update(&old_log[..12]);
    hash.update(&old_log[12..end]);
    old_log[end..].copy_from_slice(&hash.finalize());
    let old_directory = JournalHistory::install(&old_log, &history.heads[0]);
    assert!(open_journal(&old_directory.0).is_err());
    assert!(recover_journal(&old_directory.0).is_err());
    assert_eq!(
        fs::read(old_directory.0.join("solana.journal")).or_panic("expected Ok value"),
        old_log
    );
    assert_eq!(
        fs::read(old_directory.0.join("solana.head")).or_panic("expected Ok value"),
        history.heads[0]
    );
    let mut old_spool = spool_bytes;
    old_spool[..8].copy_from_slice(b"LXMSPL02");
    let end = old_spool.len() - 32;
    let digest = Sha256::digest(&old_spool[..end]);
    old_spool[end..].copy_from_slice(&digest);
    durable_write(spool_file, &old_spool);
    assert!(ArchiveSpool::open(spool_path).is_err());
    assert_eq!(
        fs::read(spool_file).or_panic("expected Ok value"),
        old_spool
    );
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
