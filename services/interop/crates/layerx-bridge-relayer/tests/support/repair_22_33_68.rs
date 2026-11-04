use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex, RwLock};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::abi::{self, address_word};
use layerx_bridge_relayer::attestation::{
    ethereum_address, recover_signer, to_attestor_signature, uint256_from_u64, InboundAttestation,
    OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{
    deposit_answer, release_answer, serve, AttestationView,
};
use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{Entry, Journal, Observation, Position};
use layerx_bridge_relayer::relayer::{
    ChainLink, ChainSettings, GasPolicy, PaxeerLink, PaxeerSettings, Relayer, RelayerParts,
};
use layerx_bridge_relayer::rpc::{JsonRpc, PaxeerRpc, RpcFault};
use layerx_bridge_relayer::signer::{Attestor, Submitter};
use layerx_bridge_relayer::transport::{PaxeerEndpoint, PaxeerTransport, TransportFault};
use layerx_mirror::signer::{
    RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm,
};
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::{PKey, Private};
use openssl::rsa::Rsa;
use openssl::ssl::{SslAcceptor, SslMethod};
use openssl::x509::extension::{BasicConstraints, SubjectAlternativeName};
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};

const TIMEOUT: Duration = Duration::from_secs(5);
const VAULT: [u8; 20] = [0x11; 20];

pub struct Directory(pub PathBuf);
impl Directory {
    pub fn new() -> Self {
        let mut unique = [0; 16];
        openssl::rand::rand_bytes(&mut unique).unwrap();
        let path = std::env::temp_dir().join(format!("repair-22-33-68-{}", hex::encode(&unique)));
        fs::create_dir(&path).unwrap();
        Self(path)
    }
    fn journal(&self) -> Journal {
        Journal::open(&self.0.join("journal.jsonl")).unwrap()
    }
    fn cosign(&self) -> CosignDirectory {
        CosignDirectory::new(self.0.join("cosign"))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}
fn key(seed: u8) -> SigningKey {
    SigningKey::from_bytes((&[seed; 32]).into()).unwrap()
}
fn sign(digest: &[u8; 32], seed: u8) -> ([u8; 20], [u8; 65]) {
    let key = key(seed);
    let (signature, recovery) = key.sign_prehash_recoverable(digest).unwrap();
    let mut bytes = [0; 65];
    bytes[..64].copy_from_slice(&signature.to_bytes());
    bytes[64] = recovery.to_byte();
    (
        ethereum_address(key.verifying_key()),
        to_attestor_signature(bytes).unwrap(),
    )
}
fn members(seeds: &[u8]) -> Vec<[u8; 20]> {
    seeds
        .iter()
        .map(|seed| ethereum_address(key(*seed).verifying_key()))
        .collect()
}

#[derive(Clone)]
pub struct Policy {
    pub threshold: u64,
    pub members: Vec<[u8; 20]>,
}
impl Policy {
    fn new(threshold: u64, seeds: &[u8]) -> Self {
        Self {
            threshold,
            members: members(seeds),
        }
    }
    fn addresses(&self) -> Vec<u8> {
        let mut bytes = uint256_from_u64(self.members.len().try_into().unwrap()).to_vec();
        for member in &self.members {
            bytes.extend_from_slice(&address_word(member));
        }
        bytes
    }
    fn precompile(&self) -> Vec<u8> {
        let mut bytes = uint256_from_u64(96).to_vec();
        bytes.extend_from_slice(&uint256_from_u64(128 + 32 * self.members.len() as u64));
        bytes.extend_from_slice(&uint256_from_u64(self.threshold));
        bytes.extend_from_slice(&self.addresses());
        bytes.extend_from_slice(&uint256_from_u64(self.members.len() as u64));
        for _ in &self.members {
            bytes.extend_from_slice(&uint256_from_u64(1));
        }
        let decoded = abi::decode_get_attestors(&bytes).unwrap();
        assert_eq!(decoded.signers, self.members);
        assert_eq!(decoded.threshold, self.threshold);
        bytes
    }
    fn vault_members(&self) -> Vec<u8> {
        let mut bytes = uint256_from_u64(32).to_vec();
        bytes.extend_from_slice(&self.addresses());
        assert_eq!(abi::decode_address_list(&bytes).unwrap(), self.members);
        bytes
    }
}
fn certificate() -> (PKey<Private>, X509) {
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap()).unwrap();
    let mut name = X509NameBuilder::new().unwrap();
    name.append_entry_by_text("CN", "localhost").unwrap();
    let name = name.build();
    let mut cert = X509::builder().unwrap();
    cert.set_version(2).unwrap();
    cert.set_serial_number(
        &openssl::bn::BigNum::from_u32(1)
            .unwrap()
            .to_asn1_integer()
            .unwrap(),
    )
    .unwrap();
    cert.set_subject_name(&name).unwrap();
    cert.set_issuer_name(&name).unwrap();
    cert.set_pubkey(&key).unwrap();
    cert.set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    cert.set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    cert.append_extension(BasicConstraints::new().critical().ca().build().unwrap())
        .unwrap();
    let san = SubjectAlternativeName::new()
        .dns("localhost")
        .build(&cert.x509v3_context(None, None))
        .unwrap();
    cert.append_extension(san).unwrap();
    cert.sign(&key, MessageDigest::sha256()).unwrap();
    (key, cert.build())
}

// The peers are bounded loopback HTTP servers. All client, policy, journal and
// bundle processing is the production crate; no JsonRpc implementation is replaced.
pub struct Node {
    pub endpoint: PaxeerEndpoint,
    pub policy: Arc<Mutex<Policy>>,
    pub requests: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Node {
    pub fn new(chain: u64, tls: bool) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let port = listener.local_addr().unwrap().port();
        let (acceptor, anchor) = if tls {
            let (key, cert) = certificate();
            let mut acceptor = SslAcceptor::mozilla_intermediate(SslMethod::tls()).unwrap();
            acceptor.set_private_key(&key).unwrap();
            acceptor.set_certificate(&cert).unwrap();
            acceptor.check_private_key().unwrap();
            (Some(acceptor.build()), Some(cert.to_der().unwrap()))
        } else {
            (None, None)
        };
        let url = if tls {
            format!("https://localhost:{port}/")
        } else {
            format!("http://127.0.0.1:{port}/")
        };
        let endpoint = PaxeerEndpoint::new(url, TIMEOUT, anchor, chain, !tls).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let policy = Arc::new(Mutex::new(Policy::new(3, &[1, 2, 3])));
        let served_policy = Arc::clone(&policy);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let received = Arc::clone(&requests);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        stream.set_read_timeout(Some(TIMEOUT)).unwrap();
                        stream.set_write_timeout(Some(TIMEOUT)).unwrap();
                        if let Some(acceptor) = &acceptor {
                            // Refused trust anchors close the TLS connection before HTTP.
                            if let Ok(mut stream) = acceptor.accept(stream) {
                                exchange(&mut stream, chain, &served_policy, &received);
                                let _ = stream.shutdown();
                            }
                        } else {
                            exchange(&mut { stream }, chain, &served_policy, &received);
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2))
                    }
                    Err(error) => panic!("RPC listener: {error}"),
                }
            }
        });
        Self {
            endpoint,
            policy,
            requests,
            stop,
            worker: Some(worker),
        }
    }
    fn rpc(&self) -> PaxeerRpc {
        PaxeerRpc::new(vec![self.endpoint.clone()]).unwrap()
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker.take().unwrap().join().unwrap();
    }
}
fn exchange(
    stream: &mut (impl Read + Write),
    chain: u64,
    policy: &Mutex<Policy>,
    requests: &Mutex<Vec<Value>>,
) {
    let mut reader = BufReader::new(&mut *stream);
    let mut line = String::new();
    reader.read_line(&mut line).unwrap();
    assert_eq!(line, "POST / HTTP/1.1\r\n");
    let mut length = None;
    loop {
        line.clear();
        assert_ne!(reader.read_line(&mut line).unwrap(), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.strip_prefix("Content-Length: ") {
            length = Some(value.trim().parse::<usize>().unwrap());
        }
    }
    let length = length.unwrap();
    assert!(length <= 16_384);
    let mut body = vec![0; length];
    reader.read_exact(&mut body).unwrap();
    let request: Value = serde_json::from_slice(&body).unwrap();
    drop(reader);
    assert_eq!(request["jsonrpc"], "2.0");
    requests.lock().unwrap().push(request.clone());
    let result = match request["method"].as_str().unwrap() {
        "eth_chainId" => {
            assert_eq!(request["params"], json!([]));
            json!(format!("0x{chain:x}"))
        }
        "eth_blockNumber" => json!("0x10"),
        "eth_call" => {
            assert_eq!(request["params"][1], "latest");
            let data = hex::decode(request["params"][0]["data"].as_str().unwrap()).unwrap();
            let policy = policy.lock().unwrap();
            let (target, bytes) = if data == abi::encode_get_chain(1) {
                let mut bytes = uint256_from_u64(1).to_vec();
                bytes.extend_from_slice(&address_word(&VAULT));
                bytes.extend_from_slice(&uint256_from_u64(1));
                bytes.extend_from_slice(&uint256_from_u64(1));
                assert_eq!(abi::decode_get_chain(&bytes).unwrap().vault, VAULT);
                (abi::LAYERX_BRIDGE_PRECOMPILE, bytes)
            } else if data == abi::GET_ATTESTORS_SELECTOR {
                (abi::LAYERX_BRIDGE_PRECOMPILE, policy.precompile())
            } else if data == abi::THRESHOLD_SELECTOR {
                (VAULT, uint256_from_u64(policy.threshold).to_vec())
            } else if data == abi::ATTESTORS_SELECTOR {
                (VAULT, policy.vault_members())
            } else {
                panic!("unexpected policy calldata {}", hex::prefixed(&data));
            };
            assert_eq!(request["params"][0]["to"], hex::prefixed(&target));
            json!(hex::prefixed(&bytes))
        }
        method => panic!("unexpected RPC method {method}"),
    };
    let body = json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
    write!(
        stream,
        "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    )
    .unwrap();
    stream.flush().unwrap();
}

pub fn configured_root_transport() {
    let directory = Directory::new();
    let node = Node::new(125, true);
    let anchor = directory.0.join("ca.der");
    fs::write(&anchor, node.endpoint.trust_anchor_der.as_ref().unwrap()).unwrap();
    let mut value: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../services/bridge/deploy/relayer-solana.example.json"
    )))
    .unwrap();
    value["paxeer"]["receipt_keys"] = json!([{"sequencer_id": hex::prefixed(&[1;32]), "public_key": hex::prefixed(&[2;32]), "first_height": 0, "last_height": u64::MAX}]);
    value["paxeer"]["endpoints"] =
        json!([{"url": node.endpoint.url, "trust_anchor_der": anchor, "request_timeout_ms": 5000}]);
    let path = directory.0.join("relayer.json");
    fs::write(&path, serde_json::to_vec(&value).unwrap()).unwrap();
    let config = RelayerConfig::load(&path).unwrap();
    let endpoints = config.paxeer_endpoints().unwrap();
    assert_eq!(endpoints, vec![node.endpoint.clone()]);
    assert_eq!(
        PaxeerRpc::new(endpoints)
            .unwrap()
            .call("eth_blockNumber", json!([])),
        Ok(json!("0x10"))
    );
    assert_eq!(node.requests.lock().unwrap().len(), 2);
    let mut wrong = node.endpoint.clone();
    wrong.expected_chain_id = 126;
    assert_eq!(
        PaxeerTransport::call(&wrong, "eth_blockNumber", &json!([])),
        Err(TransportFault::ChainMismatch {
            expected: 126,
            actual: 125
        })
    );
    assert_eq!(node.requests.lock().unwrap().len(), 3);
    let mut untrusted = node.endpoint.clone();
    untrusted.trust_anchor_der = Some(certificate().1.to_der().unwrap());
    assert!(matches!(
        PaxeerTransport::call(&untrusted, "eth_blockNumber", &json!([])),
        Err(TransportFault::Authentication { .. })
    ));
    assert_eq!(node.requests.lock().unwrap().len(), 3);
}

pub fn tls_aliases_are_not_independent_votes() {
    let first = Node::new(125, true);
    let second = Node::new(125, true);
    let independent = vec![first.endpoint.clone(), second.endpoint.clone()];
    assert_eq!(
        PaxeerRpc::new(independent)
            .unwrap()
            .call("eth_blockNumber", json!([])),
        Ok(json!("0x10"))
    );
    for url in [
        "https://LOCALHOST/",
        "https://localhost.:443/",
        "https://127.0.0.1:0443/",
        "https://[::1]:443/",
        "https://[::ffff:127.0.0.1]:443/private-proxy",
    ] {
        let mut a = first.endpoint.clone();
        a.url = "https://localhost/".to_owned();
        let mut alias = a.clone();
        alias.url = url.to_owned();
        alias.trust_anchor_der = second.endpoint.trust_anchor_der.clone();
        assert!(matches!(
            PaxeerRpc::new(vec![a, alias]),
            Err(RpcFault::Configuration)
        ));
    }
    // Different DNS names and ports sharing a pinned public key are aliases too.
    let mut alias = first.endpoint.clone();
    alias.url = "https://another-backend.example.invalid:9443/".to_owned();
    assert!(matches!(
        PaxeerRpc::new(vec![first.endpoint.clone(), alias, second.endpoint.clone()]),
        Err(RpcFault::Configuration)
    ));
}

fn remote(directory: &Directory, seed: u8) -> RemoteChainSigner {
    RemoteChainSigner::new(RemoteSignerConfig {
        endpoint: SignerEndpoint::Uds {
            socket: directory.0.join("unused-signer.sock"),
        },
        algorithm: SigningAlgorithm::Secp256k1Recoverable,
        key_handle: format!("fixture/{seed}"),
        public_key: key(seed)
            .verifying_key()
            .to_encoded_point(true)
            .as_bytes()
            .to_vec(),
        timeout: TIMEOUT,
    })
    .unwrap()
}
fn relayer(directory: &Directory, paxeer: &Node, external: &Node) -> Relayer {
    let gas = GasPolicy {
        gas_limit: 1000000,
        max_fee_per_gas: 100000000000,
        max_priority_fee_per_gas: 2000000000,
    };
    Relayer::new(RelayerParts {
        attestor: Attestor::new(remote(directory, 1)).unwrap(),
        paxeer: PaxeerLink {
            settings: PaxeerSettings {
                chain_id: 125,
                finality_depth: 1,
                start_block: 1,
                max_block_range: 10,
                gas,
            },
            rpc: Box::new(paxeer.rpc()),
            receipt_keys: Vec::new(),
            submitter: Submitter::paxeer(remote(directory, 8)).unwrap(),
        },
        chains: vec![ChainLink {
            settings: ChainSettings {
                chain_id: 1,
                vault: VAULT,
                finality_depth: 1,
                start_block: 1,
                max_block_range: 10,
                gas,
            },
            rpc: Box::new(external.rpc()),
            submitter: Submitter::ethereum(remote(directory, 9)).unwrap(),
        }],
        journal: directory.journal(),
        cosign: Some(directory.cosign()),
        max_submissions: 3,
    })
    .unwrap()
}
fn get(address: SocketAddr, path: &str) -> Value {
    let mut stream = TcpStream::connect_timeout(&address, TIMEOUT).unwrap();
    stream.set_read_timeout(Some(TIMEOUT)).unwrap();
    write!(
        stream,
        "GET {path} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
    )
    .unwrap();
    let mut response = String::new();
    stream.read_to_string(&mut response).unwrap();
    let (head, body) = response.split_once("\r\n\r\n").unwrap();
    assert!(head.starts_with("HTTP/1.1 200 OK"), "{response}");
    assert!(head.contains("Cache-Control: no-store"));
    serde_json::from_str(body).unwrap()
}

pub fn policy_restart_and_rotation() {
    let directory = Directory::new();
    let paxeer = Node::new(125, false);
    let external = Node::new(1, false);
    *external.policy.lock().unwrap() = Policy::new(1, &[1]);
    let mut recipient = [0; 32];
    recipient[12..].fill(0x22);
    let deposit = InboundAttestation {
        chain_id: 1,
        vault: VAULT,
        tx_hash: [0x33; 32],
        log_index: 7,
        recipient,
        asset: [0x44; 20],
        amount: uint256_from_u64(123),
    };
    let release = OutboundAttestation {
        chain_id: 1,
        vault: VAULT,
        paxeer_tx_hash: [0x55; 32],
        paxeer_nonce: 9,
        recipient: [0x22; 20],
        asset: deposit.asset,
        amount: deposit.amount,
    };
    let position = Position {
        block_number: 10,
        block_hash: [0x66; 32],
    };
    let mut journal = directory.journal();
    for (observation, digest) in [
        (Observation::inbound(&deposit, position), deposit.digest()),
        (Observation::outbound(&release, position), release.digest()),
    ] {
        let item = observation.key();
        journal
            .append(&Entry::Observed {
                item: item.clone(),
                observation,
            })
            .unwrap();
        journal
            .append(&Entry::Signed {
                item,
                signature: sign(&digest, 1).1,
            })
            .unwrap();
    }
    let recovered = journal.state().clone();
    drop(journal);
    let mut relay = relayer(&directory, &paxeer, &external);
    assert_eq!(relay.journal().state(), &recovered);
    let shared = Arc::new(RwLock::new(relay.attestation_view()));
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let serving = Arc::clone(&shared);
    thread::spawn(move || serve(listener, serving, Vec::new()).unwrap());
    let deposit_path = format!("/v1/deposit/1/{}/7", hex::prefixed(&deposit.tx_hash));
    let release_path = format!("/v1/release/1/{}/9", hex::prefixed(&release.paxeer_tx_hash));
    assert_eq!(get(address, &deposit_path)["status"], "attesting");
    assert_eq!(get(address, &deposit_path)["signatures"], json!([]));
    for result in relay.refresh_attestation_policies() {
        result.unwrap();
    }
    *shared.write().unwrap() = relay.attestation_view();
    assert_eq!(get(address, &deposit_path)["status"], "attesting");
    assert_eq!(
        get(address, &deposit_path)["signatures"],
        json!([hex::prefixed(&sign(&deposit.digest(), 1).1)])
    );
    assert_eq!(get(address, &release_path)["status"], "ready");
    // Opposite-direction policy changes must not lower the inbound threshold.
    for threshold in [2, 1] {
        *external.policy.lock().unwrap() = Policy::new(threshold, &[1, 2, 3]);
        for result in relay.refresh_attestation_policies() {
            result.unwrap();
        }
        *shared.write().unwrap() = relay.attestation_view();
        assert_eq!(get(address, &deposit_path)["status"], "attesting");
        assert_eq!(
            get(address, &release_path)["status"],
            if threshold == 1 { "ready" } else { "attesting" }
        );
    }
    drop(relay);
    relay = relayer(&directory, &paxeer, &external);
    assert_eq!(relay.journal().state(), &recovered);
    *shared.write().unwrap() = relay.attestation_view();
    for path in [&deposit_path, &release_path] {
        assert_eq!(get(address, path)["status"], "attesting");
        assert_eq!(get(address, path)["signatures"], json!([]));
    }
    for result in relay.refresh_attestation_policies() {
        result.unwrap();
    }
    *shared.write().unwrap() = relay.attestation_view();
    assert_eq!(get(address, &deposit_path)["status"], "attesting");
    let cosign = directory.cosign();
    for digest in [deposit.digest(), release.digest()] {
        for seed in [1, 2, 3] {
            let (address, signature) = sign(&digest, seed);
            cosign.publish(&digest, &address, &signature).unwrap();
        }
    }
    *shared.write().unwrap() = relay.attestation_view();
    assert_eq!(get(address, &deposit_path)["status"], "ready");
    // The actual policy producer rotates both destinations; old local and peer
    // signatures remain durable but cannot appear in the served bundle.
    *paxeer.policy.lock().unwrap() = Policy::new(2, &[3, 4]);
    *external.policy.lock().unwrap() = Policy::new(2, &[3, 4]);
    drop(relay);
    relay = relayer(&directory, &paxeer, &external);
    for result in relay.refresh_attestation_policies() {
        result.unwrap();
    }
    *shared.write().unwrap() = relay.attestation_view();
    for (path, digest) in [
        (&deposit_path, deposit.digest()),
        (&release_path, release.digest()),
    ] {
        let bundle = get(address, path);
        assert_eq!(bundle["status"], "attesting");
        assert_eq!(
            bundle["signatures"],
            json!([hex::prefixed(&sign(&digest, 3).1)])
        );
        let (signer, signature) = sign(&digest, 4);
        cosign.publish(&digest, &signer, &signature).unwrap();
    }
    *shared.write().unwrap() = relay.attestation_view();
    for (path, digest) in [
        (&deposit_path, deposit.digest()),
        (&release_path, release.digest()),
    ] {
        let bundle = get(address, path);
        assert_eq!(bundle["status"], "ready");
        let signatures = bundle["signatures"].as_array().unwrap();
        assert_eq!(signatures.len(), 2);
        let signers: Vec<_> = signatures
            .iter()
            .map(|signature| {
                recover_signer(&digest, &hex::fixed(signature.as_str().unwrap()).unwrap()).unwrap()
            })
            .collect();
        assert!(signers.windows(2).all(|pair| pair[0] < pair[1]));
        assert!(signers
            .iter()
            .all(|signer| members(&[3, 4]).contains(signer)));
    }
    let view = relay.attestation_view();
    assert_eq!(
        deposit_answer(&view, 1, &deposit.tx_hash, 7).unwrap(),
        get(address, &deposit_path)
    );
    assert_eq!(
        release_answer(&view, 1, &release.paxeer_tx_hash, 9).unwrap(),
        get(address, &release_path)
    );
    for node in [&paxeer, &external] {
        assert!(node
            .requests
            .lock()
            .unwrap()
            .iter()
            .any(|request| request["method"] == "eth_call"));
    }
}
