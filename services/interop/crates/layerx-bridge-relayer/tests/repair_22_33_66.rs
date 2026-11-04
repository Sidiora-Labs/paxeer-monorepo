use std::collections::BTreeMap;
use std::fs::{self, OpenOptions};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, RwLock};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    ethereum_address, to_attestor_signature, uint256_from_u64, InboundAttestation,
    OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{
    deposit_answer, release_answer, serve, AttestationSnapshot, AttestationView,
    AuthenticatedPolicy, PolicyDestination,
};
use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{
    Completion, Entry, Item, Journal, JournalError, Observation, Position, SubmissionStatus,
    CHECKPOINT_INTERVAL,
};
use layerx_bridge_relayer::rpc::{JsonRpc, PaxeerRpc, RpcFault, SendOutcome};
use layerx_bridge_relayer::transport::PaxeerEndpoint;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

const TIMEOUT: Duration = Duration::from_secs(3);
static DIRECTORY: AtomicUsize = AtomicUsize::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-66-{}-{}",
            std::process::id(),
            DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap();
        Self(path)
    }

    fn path(&self) -> PathBuf {
        self.0.join("relay.jsonl")
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn observation(index: u64, outbound: bool) -> Observation {
    let position = Position {
        block_number: index + 1,
        block_hash: [0xab; 32],
    };
    if outbound {
        Observation::outbound(
            &OutboundAttestation {
                chain_id: 1,
                vault: [0x11; 20],
                paxeer_tx_hash: uint256_from_u64(index),
                paxeer_nonce: index,
                recipient: [0x22; 20],
                asset: [0x33; 20],
                amount: uint256_from_u64(123),
            },
            position,
        )
    } else {
        Observation::inbound(
            &InboundAttestation {
                chain_id: 1,
                vault: [0x11; 20],
                tx_hash: uint256_from_u64(index),
                log_index: index,
                recipient: [0x22; 32],
                asset: [0x33; 20],
                amount: uint256_from_u64(123),
            },
            position,
        )
    }
}

fn signed(journal: &mut Journal, observation: Observation) -> String {
    let key = observation.key();
    journal
        .append(&Entry::Observed {
            item: key.clone(),
            observation,
        })
        .unwrap();
    let digest = match observation {
        Observation::Inbound { .. } => observation.inbound_attestation().unwrap().digest(),
        Observation::Outbound { .. } => observation.outbound_attestation().unwrap().digest(),
    };
    journal
        .append(&Entry::Signed {
            item: key.clone(),
            signature: sign(&digest, 1).1,
        })
        .unwrap();
    key
}

fn sign(digest: &[u8; 32], seed: u8) -> ([u8; 20], [u8; 65]) {
    let key = SigningKey::from_bytes((&[seed; 32]).into()).unwrap();
    let (signature, recovery) = key.sign_prehash_recoverable(digest).unwrap();
    let mut bytes = [0; 65];
    bytes[..64].copy_from_slice(&signature.to_bytes());
    bytes[64] = recovery.to_byte();
    (
        ethereum_address(key.verifying_key()),
        to_attestor_signature(bytes).unwrap(),
    )
}

fn policies() -> BTreeMap<PolicyDestination, AuthenticatedPolicy> {
    [
        PolicyDestination::Inbound {
            chain_id: 125,
            bridge: layerx_bridge_relayer::abi::LAYERX_BRIDGE_PRECOMPILE,
            source_chain_id: 1,
            source_vault: [0x11; 20],
        },
        PolicyDestination::EvmRelease {
            chain_id: 1,
            vault: [0x11; 20],
        },
    ]
    .into_iter()
    .map(|destination| {
        (
            destination,
            AuthenticatedPolicy {
                destination,
                threshold: 2,
                attestors: vec![sign(&[0; 32], 1).0, sign(&[0; 32], 2).0],
            },
        )
    })
    .collect()
}

fn archive_file(path: &Path, key: &str) -> PathBuf {
    path.with_extension("history")
        .join(hex::prefixed(&Sha256::digest(key.as_bytes())))
}

fn get(address: SocketAddr, path: &str) -> (u16, Value) {
    let mut stream = TcpStream::connect(address).unwrap();
    stream.set_read_timeout(Some(TIMEOUT)).unwrap();
    write!(
        stream,
        "GET {path} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
    )
    .unwrap();
    let mut response = String::new();
    stream.read_to_string(&mut response).unwrap();
    let (headers, body) = response.split_once("\r\n\r\n").unwrap();
    (
        headers.split_whitespace().nth(1).unwrap().parse().unwrap(),
        serde_json::from_str(body).unwrap(),
    )
}

#[test]
fn journal_growth_checkpoint_replays_pending_bytes_and_serves_indexed_history() {
    let directory = Directory::new();
    let path = directory.path();
    let mut journal = Journal::open(&path).unwrap();
    let cosign = CosignDirectory::new(directory.0.join("cosign"));
    let inbound = observation(1, false);
    let outbound = observation(2, true);
    let deposit = signed(&mut journal, inbound);
    let release = signed(&mut journal, outbound);
    for observation in [inbound, outbound] {
        let digest = match observation {
            Observation::Inbound { .. } => observation.inbound_attestation().unwrap().digest(),
            Observation::Outbound { .. } => observation.outbound_attestation().unwrap().digest(),
        };
        let (address, signature) = sign(&digest, 2);
        cosign.publish(&digest, &address, &signature).unwrap();
    }
    let raw = vec![2, 0, 0xff, 0x80, 0x01, 0xfe];
    journal
        .append(&Entry::Submitted {
            item: deposit.clone(),
            submitter: [0x44; 20],
            nonce: 7,
            tx_hash: [0x55; 32],
            raw: raw.clone(),
        })
        .unwrap();
    journal
        .append(&Entry::Receipted {
            item: deposit.clone(),
            tx_hash: [0x55; 32],
            sequence: 8,
            height: 9,
        })
        .unwrap();
    journal
        .append(&Entry::Completed {
            item: deposit.clone(),
            completion: Completion::Included {
                tx_hash: [0x55; 32],
                block_number: 10,
            },
        })
        .unwrap();
    journal
        .append(&Entry::Completed {
            item: release.clone(),
            completion: Completion::AlreadyBridged,
        })
        .unwrap();
    let pending = signed(&mut journal, observation(3, false));
    journal
        .append(&Entry::Submitted {
            item: pending.clone(),
            submitter: [0x44; 20],
            nonce: 8,
            tx_hash: [0x66; 32],
            raw: raw.clone(),
        })
        .unwrap();
    journal
        .append(&Entry::Receipted {
            item: pending.clone(),
            tx_hash: [0x66; 32],
            sequence: 11,
            height: 12,
        })
        .unwrap();
    let solana = signed(&mut journal, observation(4, true));
    journal
        .append(&Entry::RecipientResolved {
            item: solana.clone(),
            key: [0x77; 32],
        })
        .unwrap();
    journal
        .append(&Entry::ReleaseSubmitted {
            item: solana.clone(),
            fee_payer: [0x88; 32],
            signature: [0x99; 64],
            last_valid_block_height: 999,
            raw: vec![0, 1, 0xff, 0x80],
        })
        .unwrap();
    let refused = signed(&mut journal, observation(5, false));
    journal
        .append(&Entry::Refused {
            item: refused.clone(),
            reason: "unsupported asset".into(),
        })
        .unwrap();
    journal
        .append(&Entry::Cursor {
            stream: "in:1".into(),
            next_block: 100,
        })
        .unwrap();
    let before = journal.state().clone();
    let before_view = AttestationView::from_journal(&journal, Some(&cosign), &policies());
    let before_deposit = deposit_answer(&before_view, 1, &uint256_from_u64(1), 1).unwrap();
    let before_release = release_answer(&before_view, 1, &uint256_from_u64(2), 2).unwrap();
    assert_eq!(before_deposit["status"], "ready");
    assert_eq!(before_deposit["signatures"].as_array().unwrap().len(), 2);
    journal.checkpoint().unwrap();
    assert_eq!(journal.state().items.len(), 2);
    for (key, item) in &before.items {
        assert_eq!(journal.lookup(key).unwrap().as_ref(), Some(item));
    }
    drop(journal);
    let mut reopened = Journal::open(&path).unwrap();
    assert_eq!(reopened.state().cursors, before.cursors);
    assert_eq!(reopened.state().items.len(), 2);
    assert_eq!(reopened.state().items[&pending].pending().unwrap().raw, raw);
    assert_eq!(
        reopened.state().items[&pending].pending().unwrap().status,
        SubmissionStatus::Receipted {
            sequence: 11,
            height: 12
        }
    );
    assert_eq!(
        reopened.state().items[&solana]
            .pending_release()
            .unwrap()
            .raw,
        vec![0, 1, 0xff, 0x80]
    );
    for (key, item) in &before.items {
        assert_eq!(reopened.lookup(key).unwrap().as_ref(), Some(item));
    }
    reopened
        .append(&Entry::Observed {
            item: deposit.clone(),
            observation: inbound,
        })
        .unwrap();
    assert!(!reopened.state().items.contains_key(&deposit));
    let mut conflict = inbound;
    if let Observation::Inbound { ref mut amount, .. } = conflict {
        *amount = [0xff; 32];
    }
    assert!(matches!(
        reopened.append(&Entry::Observed {
            item: deposit.clone(),
            observation: conflict
        }),
        Err(JournalError::Conflict(_))
    ));
    assert!(matches!(
        reopened.append(&Entry::Submitted {
            item: deposit.clone(),
            submitter: [0x44; 20],
            nonce: 9,
            tx_hash: [0xaa; 32],
            raw: vec![1],
        }),
        Err(JournalError::Conflict(_))
    ));
    reopened
        .append(&Entry::Cursor {
            stream: "in:1".into(),
            next_block: 101,
        })
        .unwrap();
    drop(reopened);
    let reopened = Journal::open(&path).unwrap();
    assert_eq!(reopened.state().cursors["in:1"], 101);
    assert_eq!(
        reopened.lookup(&pending).unwrap().unwrap(),
        before.items[&pending]
    );
    let snapshot = AttestationSnapshot::from_journal(&reopened, Some(&cosign), &policies());
    assert_eq!(snapshot.items.len(), 2);
    assert_eq!(snapshot.signatures.len(), 2);
    assert_eq!(
        deposit_answer(&snapshot, 1, &uint256_from_u64(1), 1),
        Some(before_deposit.clone())
    );
    assert_eq!(
        release_answer(&snapshot, 1, &uint256_from_u64(2), 2),
        Some(before_release.clone())
    );
    let shared = Arc::new(RwLock::new(snapshot));
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let serving = Arc::clone(&shared);
    thread::spawn(move || serve(listener, serving, vec![]).unwrap());
    let deposit_path = format!("/v1/deposit/1/{}/1", hex::prefixed(&uint256_from_u64(1)));
    let release_path = format!("/v1/release/1/{}/2", hex::prefixed(&uint256_from_u64(2)));
    assert_eq!(get(address, &deposit_path), (200, before_deposit));
    assert_eq!(get(address, &release_path), (200, before_release));
    assert_eq!(
        get(
            address,
            &format!("/v1/deposit/1/{}/999", hex::prefixed(&[0; 32]))
        )
        .0,
        404
    );
    fs::write(
        archive_file(&path, &deposit),
        b"interrupted/corrupt archive",
    )
    .unwrap();
    assert_eq!(get(address, &deposit_path).0, 500);
}

fn complete(journal: &mut Journal, index: u64) -> String {
    let observation = observation(index, false);
    let key = observation.key();
    journal
        .append(&Entry::Observed {
            item: key.clone(),
            observation,
        })
        .unwrap();
    journal
        .append(&Entry::Completed {
            item: key.clone(),
            completion: Completion::AlreadyBridged,
        })
        .unwrap();
    key
}

#[test]
fn journal_growth_interrupted_archival_and_replacement_recover_equivalent_state() {
    for during_replacement in [false, true] {
        let directory = Directory::new();
        let path = directory.path();
        let mut journal = Journal::open(&path).unwrap();
        let first = complete(&mut journal, 1);
        let second = complete(&mut journal, 2);
        let pending = signed(&mut journal, observation(3, false));
        journal
            .append(&Entry::Submitted {
                item: pending.clone(),
                submitter: [0x22; 20],
                nonce: 42,
                tx_hash: [0xaa; 32],
                raw: vec![0xff, 0, 0x80],
            })
            .unwrap();
        let state = journal.state().clone();
        let log = fs::read(&path).unwrap();
        let blocked = if during_replacement {
            path.with_extension("checkpoint.tmp")
        } else {
            archive_file(&path, &second).with_extension("tmp")
        };
        fs::create_dir(&blocked).unwrap();
        assert!(matches!(journal.checkpoint(), Err(JournalError::Io(_))));
        assert!(
            archive_file(&path, &first).is_file(),
            "interruption occurs after actual archival began"
        );
        assert_eq!(
            fs::read(&path).unwrap(),
            log,
            "old authoritative log survives"
        );
        assert!(journal
            .append(&Entry::Cursor {
                stream: "in:1".into(),
                next_block: 10
            })
            .is_err());
        drop(journal);
        fs::remove_dir(&blocked).unwrap();
        let mut recovered = Journal::open(&path).unwrap();
        assert_eq!(recovered.state(), &state);
        recovered.checkpoint().unwrap();
        drop(recovered);
        let recovered = Journal::open(&path).unwrap();
        assert_eq!(recovered.state().items.len(), 1);
        for (key, item) in &state.items {
            assert_eq!(recovered.lookup(key).unwrap().as_ref(), Some(item));
        }
        assert_eq!(
            recovered.state().items[&pending].pending().unwrap().raw,
            vec![0xff, 0, 0x80]
        );
        assert_eq!(
            recovered.lookup(&first).unwrap().unwrap().completion,
            Some(Completion::AlreadyBridged)
        );
        assert_eq!(
            recovered.lookup(&second).unwrap().unwrap().completion,
            Some(Completion::AlreadyBridged)
        );
    }
}

#[test]
fn journal_growth_automatic_checkpoints_bound_active_state_and_replay() {
    let directory = Directory::new();
    let path = directory.path();
    let mut journal = Journal::open(&path).unwrap();
    let pending = signed(&mut journal, observation(999_999, false));
    let mut keys = Vec::new();
    for index in 0..u64::try_from(CHECKPOINT_INTERVAL * 3).unwrap() {
        keys.push(complete(&mut journal, index));
        assert!(journal.state().items.len() <= CHECKPOINT_INTERVAL + 1);
    }
    assert!(fs::metadata(&path).unwrap().len() < 20_000);
    let snapshot = AttestationSnapshot::from_journal(&journal, None, &BTreeMap::new());
    assert!(snapshot.items.len() <= CHECKPOINT_INTERVAL + 1);
    assert!(snapshot.signatures.len() <= CHECKPOINT_INTERVAL + 1);
    journal.checkpoint().unwrap();
    let bytes = fs::read(&path).unwrap();
    assert_eq!(bytes.iter().filter(|byte| **byte == b'\n').count(), 1);
    let state = journal.state().clone();
    drop(journal);
    // An incomplete tail is discarded without losing the committed checkpoint.
    OpenOptions::new()
        .append(true)
        .open(&path)
        .unwrap()
        .write_all(b"{\"kind\":\"cursor\"")
        .unwrap();
    let recovered = Journal::open(&path).unwrap();
    assert_eq!(recovered.state(), &state);
    assert_eq!(fs::read(&path).unwrap(), bytes);
    assert!(recovered.state().items.contains_key(&pending));
    for key in keys {
        assert_eq!(
            recovered.lookup(&key).unwrap().unwrap().completion,
            Some(Completion::AlreadyBridged)
        );
    }
}

struct RpcServer {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    requests: Arc<Mutex<Vec<Value>>>,
}

impl RpcServer {
    fn start(answer: Value) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let received = Arc::clone(&requests);
        let worker = thread::spawn(move || {
            for stream in listener.incoming() {
                let mut stream = stream.unwrap();
                if stop.load(Ordering::Acquire) {
                    break;
                }
                stream.set_read_timeout(Some(TIMEOUT)).unwrap();
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut first = String::new();
                reader.read_line(&mut first).unwrap();
                assert_eq!(first, "POST / HTTP/1.1\r\n");
                let mut length = None;
                loop {
                    let mut header = String::new();
                    assert_ne!(reader.read_line(&mut header).unwrap(), 0);
                    if header == "\r\n" {
                        break;
                    }
                    if let Some((key, value)) = header.trim().split_once(':') {
                        if key.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().unwrap());
                        }
                    }
                }
                let mut bytes = vec![0; length.unwrap()];
                reader.read_exact(&mut bytes).unwrap();
                let request: Value = serde_json::from_slice(&bytes).unwrap();
                received.lock().unwrap().push(request.clone());
                let value = if request["method"] == "eth_chainId" {
                    json!("0x7d")
                } else {
                    answer.clone()
                };
                let body =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": value}).to_string();
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                )
                .unwrap();
            }
        });
        Self {
            address,
            stopped,
            worker: Some(worker),
            requests,
        }
    }

    fn endpoint(&self) -> PaxeerEndpoint {
        PaxeerEndpoint::new(
            format!("http://{}/", self.address),
            TIMEOUT,
            None,
            125,
            true,
        )
        .unwrap()
    }
}

impl Drop for RpcServer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        TcpStream::connect(self.address).unwrap();
        self.worker.take().unwrap().join().unwrap();
    }
}

fn config_value(endpoints: Value) -> Value {
    json!({
        "journal_path": "/unused/journal", "poll_interval_ms": 10, "max_submissions": 3,
        "signer": {"endpoint": {"transport": "uds", "socket": "/unused/signer"}, "timeout_ms": 100},
        "attestor": {"handle": "attestor", "public_key": "0x02aa"},
        "paxeer": {
            "chain_id": 125, "endpoints": endpoints,
            "receipt_keys": [{"sequencer_id": hex::prefixed(&[1; 32]), "public_key": hex::prefixed(&[2; 32]), "first_height": 0, "last_height": u64::MAX}],
            "finality_depth": 1, "start_block": 0, "max_block_range": 100,
            "submitter": {"handle": "paxeer-fees", "public_key": "0x02bb"},
            "gas": {"gas_limit": 100000, "max_fee_per_gas": 100, "max_priority_fee_per_gas": 1}
        },
        "chains": [{
            "chain_id": 1, "vault": hex::prefixed(&[0x11; 20]), "finality_depth": 1, "start_block": 0, "max_block_range": 100,
            "rpc": {"endpoints": [], "quorum": 2, "connect_timeout_ms": 100, "request_timeout_ms": 100, "maximum_response_bytes": 1024},
            "submitter": {"handle": "external-fees", "public_key": "0x02cc"},
            "gas": {"gas_limit": 100000, "max_fee_per_gas": 100, "max_priority_fee_per_gas": 1}
        }]
    })
}

fn config(endpoints: Value) -> RelayerConfig {
    serde_json::from_value(config_value(endpoints)).unwrap()
}

fn endpoint_config(server: &RpcServer, identity: Option<&str>) -> Value {
    let mut value =
        json!({"url": server.endpoint().url, "local_emulator": true, "request_timeout_ms": 3000});
    if let Some(identity) = identity {
        value["independent_backend"] = json!(identity);
    }
    value
}

#[test]
fn duplicate_quorum_rejects_urls_authority_aliases_and_backend_aliases_before_transport() {
    let first = RpcServer::start(json!("0x10"));
    let second = RpcServer::start(json!("0x11"));
    let port = first.address.port();
    for url in [
        first.endpoint().url,
        format!("http://localhost:{port}/"),
        format!("http://LOCALHOST.:{port}/private"),
        format!("http://[::ffff:127.0.0.1]:{port}/"),
        format!("http://127.0.0.2:{port}/another"),
    ] {
        let mut alias = first.endpoint();
        alias.url = url.clone();
        assert!(matches!(
            PaxeerRpc::with_backends(vec![
                ("first".into(), first.endpoint()),
                ("alias".into(), alias),
                ("second".into(), second.endpoint()),
            ]),
            Err(RpcFault::Configuration)
        ));
        let mut alias_config = endpoint_config(&first, Some("alias"));
        alias_config["url"] = json!(url);
        let config = config(json!([
            endpoint_config(&first, Some("first")),
            alias_config,
            endpoint_config(&second, Some("second"))
        ]));
        assert!(config.validate().is_err());
        assert!(config.paxeer_rpc().is_err());
    }
    for identity in ["shared", "SHARED"] {
        assert!(matches!(
            PaxeerRpc::with_backends(vec![
                ("shared".into(), first.endpoint()),
                (identity.into(), second.endpoint()),
            ]),
            Err(RpcFault::Configuration)
        ));
        assert!(config(json!([
            endpoint_config(&first, Some("shared")),
            endpoint_config(&second, Some(identity))
        ]))
        .validate()
        .is_err());
    }
    for identity in ["", " backend", "backend ", "two backends"] {
        assert!(matches!(
            PaxeerRpc::with_backends(vec![
                (identity.into(), first.endpoint()),
                ("other".into(), second.endpoint())
            ]),
            Err(RpcFault::Configuration)
        ));
    }
    assert!(matches!(
        PaxeerRpc::new(vec![first.endpoint(), second.endpoint()]),
        Err(RpcFault::Configuration)
    ));
    assert!(config(json!([
        endpoint_config(&first, None),
        endpoint_config(&second, Some("second"))
    ]))
    .validate()
    .is_err());
    let tls_aliases = config(json!([
        {"url": "https://RPC.EXAMPLE.invalid/", "trust_anchor_der": "/unused/one.der", "request_timeout_ms": 3000, "independent_backend": "one"},
        {"url": "https://rpc.example.invalid.:443/alias", "trust_anchor_der": "/unused/two.der", "request_timeout_ms": 3000, "independent_backend": "two"}
    ]));
    assert!(
        tls_aliases.validate().is_err(),
        "case, trailing dots, default ports and paths do not add authorities"
    );
    let mut mixed_chain = second.endpoint();
    mixed_chain.expected_chain_id = 126;
    assert!(matches!(
        PaxeerRpc::with_backends(vec![
            ("first".into(), first.endpoint()),
            ("second".into(), mixed_chain)
        ]),
        Err(RpcFault::Configuration)
    ));
    assert!(matches!(
        PaxeerRpc::with_backends(vec![]),
        Err(RpcFault::Configuration)
    ));
    assert!(matches!(
        PaxeerRpc::with_backends(
            (0..9)
                .map(|index| (format!("backend-{index}"), first.endpoint()))
                .collect()
        ),
        Err(RpcFault::Configuration)
    ));
    assert!(first.requests.lock().unwrap().is_empty());
    assert!(second.requests.lock().unwrap().is_empty());
}

#[test]
fn duplicate_quorum_counts_independent_backends_and_retains_single_endpoint_operation() {
    let first = RpcServer::start(json!("0x10"));
    let second = RpcServer::start(json!("0x10"));
    let outlier = RpcServer::start(json!("0x11"));
    let directory = Directory::new();
    let value = config(json!([
        endpoint_config(&first, Some("operator-a")),
        endpoint_config(&second, Some("operator-b")),
        endpoint_config(&outlier, Some("operator-c")),
    ]));
    let config_path = directory.0.join("relayer.json");
    let mut endpoints = json!([]);
    for (server, identity) in [
        (&first, "operator-a"),
        (&second, "operator-b"),
        (&outlier, "operator-c"),
    ] {
        endpoints
            .as_array_mut()
            .unwrap()
            .push(endpoint_config(server, Some(identity)));
    }
    assert_eq!(value.validate(), Ok(()));
    assert_eq!(
        value
            .paxeer_rpc()
            .unwrap()
            .call("eth_blockNumber", json!([])),
        Ok(json!("0x10"))
    );
    let rpc = PaxeerRpc::with_backends(vec![
        ("operator-a".into(), first.endpoint()),
        ("operator-c".into(), outlier.endpoint()),
    ])
    .unwrap();
    assert_eq!(
        rpc.call("eth_blockNumber", json!([])),
        Err(RpcFault::Divergence)
    );
    let single = config(json!([endpoint_config(&outlier, None)]));
    assert_eq!(single.validate(), Ok(()));
    assert_eq!(
        single
            .paxeer_rpc()
            .unwrap()
            .call("eth_blockNumber", json!([])),
        Ok(json!("0x11"))
    );
    for server in [&first, &second, &outlier] {
        let requests = server.requests.lock().unwrap();
        for pair in requests.chunks_exact(2) {
            assert_eq!(pair[0]["method"], "eth_chainId");
            assert_eq!(pair[1]["method"], "eth_blockNumber");
        }
        assert!(!requests.is_empty());
    }
    fs::write(
        &config_path,
        serde_json::to_vec(&config_value(endpoints)).unwrap(),
    )
    .unwrap();
    let loaded = RelayerConfig::load(&config_path).unwrap();
    assert_eq!(loaded, value);
    assert_eq!(
        loaded
            .paxeer_rpc()
            .unwrap()
            .call("eth_blockNumber", json!([])),
        Ok(json!("0x10"))
    );
}

#[test]
fn journal_growth_reopened_pending_transaction_broadcasts_exact_checkpointed_bytes() {
    let directory = Directory::new();
    let path = directory.path();
    let raw = vec![2, 0xff, 0, 0x80, 7];
    let hash = [0xab; 32];
    let mut journal = Journal::open(&path).unwrap();
    let key = signed(&mut journal, observation(1, false));
    journal
        .append(&Entry::Submitted {
            item: key.clone(),
            submitter: [0x11; 20],
            nonce: 7,
            tx_hash: hash,
            raw: raw.clone(),
        })
        .unwrap();
    journal.checkpoint().unwrap();
    drop(journal);
    let recovered = Journal::open(&path).unwrap();
    let item: Item = recovered.lookup(&key).unwrap().unwrap();
    let pending = item.pending().unwrap();
    let server = RpcServer::start(json!(hex::prefixed(&hash)));
    let rpc = PaxeerRpc::new(vec![server.endpoint()]).unwrap();
    assert_eq!(
        rpc.send_raw_transaction(&pending.raw, &pending.tx_hash),
        Ok(SendOutcome::Accepted)
    );
    let requests = server.requests.lock().unwrap();
    assert_eq!(requests.len(), 2);
    assert_eq!(requests[0]["method"], "eth_chainId");
    assert_eq!(requests[1]["method"], "eth_sendRawTransaction");
    assert_eq!(requests[1]["params"], json!([hex::prefixed(&raw)]));
}
