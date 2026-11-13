//! parity-synthetic-owner: recorded node producer -> production owner/store ->
//! authenticated envelope transport -> parity comparison, including actual store reopen.
use std::collections::BTreeMap;
use std::fs;
use std::io::{BufRead, BufReader, Read};
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use agent_sdk_parity::{compare_evidence, digest, read_rpc_body, write_rpc_response, DaemonParity};
use layerx_agent_api::error::RequestId;
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::{ReadRequest, RelativeTo, VerifiedRead};
use layerx_agent_api::verify::{Rung, VerificationStatus};
use layerx_agentd::agent_rpc::{self, AgentRpcResponse};
use layerx_agentd::agent_rpc_dispatch::Dispatched;
use layerx_agentd::agent_rpc_tls::{AgentRpcTls, AgentRpcTlsPaths};
use layerx_agentd::finality::{augment_verified, restore_finality, VerifiedReceipt};
use layerx_agentd::human_runtime::{
    lookup_human_receipt_by_outbox_hash, proof_bundle_operation_value, receipt_proof_operation,
};
use layerx_agentd::idempotency::{EconomicResult, Outcome, RetentionPolicy, Store as Idempotency};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_sdk::agent_envelope::{
    decode_proof_bundle_request, encode_proof_bundle_request, AgentEnvelopeTransport,
    EnvelopeCredential, EnvelopeError,
};
use layerx_sdk::rpc::RpcClient;
use layerx_sdk::{Client, Operation};
use serde_json::{json, Value};
use ureq::tls::{Certificate, ClientCert, PrivateKey};

fn bytes(value: &Value) -> Vec<u8> {
    let text = value.as_str().unwrap();
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).unwrap(), 16).unwrap())
        .collect()
}
fn hex(value: &[u8]) -> String {
    value.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn tenant() -> TenantId {
    TenantId::new("parity-repair").unwrap()
}
fn fixture() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("src/fixture")
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "parity-1572-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap();
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

struct Peer {
    url: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Peer {
    fn start(
        tls: bool,
        mut handler: impl FnMut(Vec<u8>) -> AgentRpcResponse + Send + 'static,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let url = format!(
            "{}://localhost:{}",
            if tls { "https" } else { "http" },
            listener.local_addr().unwrap().port()
        );
        let tls = tls.then(|| {
            AgentRpcTls::from_paths(&AgentRpcTlsPaths {
                cert: fixture().join("server.pem"),
                key: fixture().join("server-key.pem"),
                client_ca: fixture().join("ca.pem"),
                peer: "parity-client".to_owned(),
            })
            .unwrap()
        });
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (mut socket, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                socket
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                if let Some(tls) = &tls {
                    let mut socket = tls.accept(socket).unwrap();
                    let response = handler(read_rpc_body(&mut socket).unwrap());
                    write_rpc_response(&mut socket, response.status, &response.body).unwrap();
                } else {
                    let mut reader = BufReader::new(&mut socket);
                    let mut line = String::new();
                    reader.read_line(&mut line).unwrap();
                    assert_eq!(line, "POST / HTTP/1.1\r\n");
                    let mut length = None;
                    loop {
                        line.clear();
                        reader.read_line(&mut line).unwrap();
                        if line == "\r\n" {
                            break;
                        }
                        let (name, value) = line.split_once(':').unwrap();
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().unwrap());
                        }
                    }
                    let length = length.unwrap();
                    assert!(length <= 1_048_576);
                    let mut body = vec![0; length];
                    reader.read_exact(&mut body).unwrap();
                    drop(reader);
                    let response = handler(body);
                    write_rpc_response(&mut socket, response.status, &response.body).unwrap();
                }
            }
        });
        Self {
            url,
            stop,
            worker: Some(worker),
        }
    }
    fn client(&self) -> DaemonParity {
        DaemonParity::connect(
            &self.url,
            &fs::read(fixture().join("client.pem")).unwrap(),
            &fs::read(fixture().join("client-key.pem")).unwrap(),
            &fixture().join("ca.pem"),
            EnvelopeCredential::new("parity-repair", [1; 32], [2; 32], 1).unwrap(),
        )
        .unwrap()
    }
    fn transport(&self) -> AgentEnvelopeTransport {
        let certificate = fs::read(fixture().join("client.pem")).unwrap();
        let key = fs::read(fixture().join("client-key.pem")).unwrap();
        let identity = ClientCert::new_with_certs(
            &[Certificate::from_pem(&certificate).unwrap()],
            PrivateKey::from_pem(&key).unwrap(),
        );
        AgentEnvelopeTransport::connect_daemon(&self.url, identity, &fixture().join("ca.pem"))
            .unwrap()
    }
}
impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker.take().unwrap().join().unwrap();
    }
}
fn recording() -> Value {
    let corpus: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap();
    corpus["cases"][1].clone()
}
fn node_peer(record: Value, sends: Arc<AtomicUsize>) -> Peer {
    Peer::start(false, move |body| {
        let request: Value = serde_json::from_slice(&body).unwrap();
        let result = match request["method"].as_str().unwrap() {
            "eth_chainId" => json!("0x7d"),
            "eth_sendRawTransaction" => {
                assert_eq!(request["params"], json!([record["rawTransaction"]]));
                sends.fetch_add(1, Ordering::SeqCst);
                record["receiptJson"]["txHash"].clone()
            }
            "px_getReceiptKeys" => json!({"keys":[record["pinnedKey"]]}),
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"], json!([record["receiptJson"]["txHash"]]));
                record["receiptJson"].clone()
            }
            "eth_getTransactionReceipt" => Value::Null,
            other => panic!("unrecorded node method {other}"),
        };
        AgentRpcResponse {
            status: 200,
            body: json!({"jsonrpc":"2.0","id":request["id"],"result":result})
                .to_string()
                .into_bytes(),
        }
    })
}
fn node(url: &str) -> NodeRpc {
    NodeRpc::connect(&NodeRpcConfig {
        url: url.to_owned(),
        chain_id: 125,
        deadline: Duration::from_secs(3),
    })
    .unwrap()
}
fn idempotency(root: &Path) -> Idempotency {
    Idempotency::open(root, tenant(), RetentionPolicy::new(100, 50).unwrap()).unwrap()
}

fn receipt_answer(
    store: &mut Store,
    outbox: &Outbox,
    url: &str,
    record: &Value,
    recovered: bool,
) -> Value {
    let hash: [u8; 32] = bytes(&record["receiptJson"]["txHash"]).try_into().unwrap();
    let node = node(url);
    let keys = node.receipt_keys().unwrap();
    let client = Client::direct_node(url, &[]).unwrap();
    // The production lookup authenticates the node receipt and its exact queued transaction.
    let (lookup, observed_keys) = lookup_human_receipt_by_outbox_hash(
        &node,
        client.direct_node_client().unwrap(),
        outbox,
        hash,
        hash,
    )
    .unwrap();
    assert_eq!(keys, observed_keys);
    assert!(format!("{lookup:?}").starts_with("Verified("));
    let response = RpcClient::connect_node(url, None)
        .unwrap()
        .sequencer_receipt(hash)
        .unwrap();
    let canonical = bytes(&response["receipt"]);
    assert_eq!(canonical, bytes(&record["signedReceipt"]));
    assert_eq!(bytes(&response["txHash"]), hash);
    let proof = receipt_proof_operation(
        ProofBundleTarget::Receipt(hash),
        Rung::Receipted,
        outbox.exact_signed_bytes(hash).unwrap(),
        &canonical,
        &keys,
        125,
        u64::from_str_radix(
            response["height"]
                .as_str()
                .unwrap()
                .trim_start_matches("0x"),
            16,
        )
        .unwrap(),
        [9; 32],
    )
    .unwrap();
    assert_eq!(proof.value.signed_receipt.as_bytes(), canonical);
    let key = TenantKey::new(tenant(), ObjectKind::Receipt, hash.to_vec()).unwrap();
    let encoded = serde_json::to_vec(&proof_bundle_operation_value(&proof).unwrap()).unwrap();
    if !recovered {
        store.put_core_cache(key.clone(), encoded.clone()).unwrap();
    }
    let persisted = store.get(&key).unwrap().bytes();
    assert_eq!(persisted, encoded);
    let served: Value = serde_json::from_slice(persisted).unwrap();
    assert_eq!(bytes(&served["value"]["signedReceipt"]), canonical);
    let key = keys[0];
    let evidence = if recovered {
        restore_finality(store, tenant(), hash, &key, "parity", &[]).unwrap()
    } else {
        let verified = VerifiedReceipt::verify(hash, &canonical, &keys).unwrap();
        augment_verified(store, tenant(), hash, &verified, None).unwrap()
    };
    assert_eq!(evidence.rung, Rung::Receipted);
    served
}
fn receipt_peer(answer: Value) -> Peer {
    let proof = VerifiedRead::<ProofBundle>::decode_json(&answer.to_string()).unwrap();
    let RelativeTo::Block { height, hash } = proof.freshness.relative_to;
    assert_eq!(height, proof.freshness.value_height);
    assert_ne!(hash, [0; 32]);
    assert!(height <= proof.freshness.chain_head);
    assert_eq!(proof_bundle_operation_value(&proof).unwrap(), answer);
    Peer::start(true, move |body| {
        let envelope = agent_rpc::decode(&body).unwrap();
        assert_eq!(envelope.operation, Operation::ReadProofBundle);
        let credential = envelope.credential.as_ref().unwrap();
        assert_eq!(credential.tenant(), &tenant());
        let request =
            decode_proof_bundle_request(&Value::Object(envelope.request.clone())).unwrap();
        let achieved = proof.achieved_rung;
        let verification = if achieved < request.requested_rung {
            VerificationStatus::Below {
                requested: request.requested_rung,
                achieved,
                reason: layerx_agent_api::error::ReasonCode::new("requested_rung_not_achieved")
                    .unwrap(),
            }
        } else {
            request.check_proof_response(&proof).unwrap();
            VerificationStatus::Achieved(achieved)
        };
        let dispatched = Dispatched {
            value: proof_bundle_operation_value(&proof).unwrap(),
            verification: Some(verification),
        };
        agent_rpc::success_body(
            envelope.request_id,
            &dispatched,
            Some(Some(request.requested_rung)),
        )
        .unwrap()
    })
}
fn query(peer: &Peer, record: &Value, rung: Rung) -> Value {
    let hash = bytes(&record["receiptJson"]["txHash"]).try_into().unwrap();
    let request = ReadRequest {
        selector: ProofBundleTarget::Receipt(hash).encode().unwrap(),
        requested_rung: rung,
    };
    peer.client()
        .observe(
            Operation::ReadProofBundle,
            RequestId(1572),
            &encode_proof_bundle_request(&request).unwrap(),
            None,
        )
        .unwrap()
}

#[test]
fn parity_synthetic_owner_compares_node_receipts_and_reopened_state() {
    let directory = Directory::new();
    let record = recording();
    let sends = Arc::new(AtomicUsize::new(0));
    let node_peer = node_peer(record.clone(), sends.clone());
    let node = node(&node_peer.url);
    let raw = bytes(&record["rawTransaction"]);
    let hash: [u8; 32] = bytes(&record["receiptJson"]["txHash"]).try_into().unwrap();
    let mut store = Store::open(directory.0.join("owner")).unwrap();
    let mut outbox = Outbox::default();
    assert_eq!(
        outbox.enqueue(&mut store, tenant(), raw.clone()).unwrap(),
        hash
    );
    let idempotency_root = directory.0.join("idempotency");
    let requests = idempotency(&idempotency_root);
    let submit = || {
        let sent = node
            .send_raw_transaction(&raw)
            .map_err(|error| error.to_string())?;
        Ok(EconomicResult {
            response_bytes: sent.to_vec(),
            receipt_ref: Some(sent),
        })
    };
    assert!(matches!(
        requests.execute([7; 32], &raw, 10, |_| submit()).unwrap(),
        Outcome::First(_)
    ));
    assert!(matches!(
        requests.execute([7; 32], &raw, 11, |_| submit()).unwrap(),
        Outcome::RepeatedOriginal(_)
    ));
    let before = receipt_answer(&mut store, &outbox, &node_peer.url, &record, false);
    let peer = receipt_peer(before.clone());
    let actual = query(&peer, &record, Rung::Receipted);
    assert_eq!(actual["status"], 200);
    assert_eq!(
        bytes(&actual["document"]["value"]["value"]["signedReceipt"]),
        bytes(&record["signedReceipt"])
    );
    assert_eq!(
        bytes(&actual["document"]["value"]["value"]["rawTransaction"]),
        raw
    );
    assert_eq!(
        actual["document"]["verification_status"],
        json!({"state":"achieved","rung":"receipted"})
    );
    // A below-rung envelope is observable evidence, but cannot be accepted as a proof.
    let request = ReadRequest {
        selector: ProofBundleTarget::Receipt(hash).encode().unwrap(),
        requested_rung: Rung::Committed,
    };
    let credential = EnvelopeCredential::new("parity-repair", [1; 32], [2; 32], 1).unwrap();
    let (received, result) = peer.transport().send_operation_recorded(
        Operation::ReadProofBundle,
        RequestId(1572),
        &encode_proof_bundle_request(&request).unwrap(),
        Some(&credential),
        None,
    );
    assert!(matches!(
        result,
        Err(EnvelopeError::Decode {
            operation: Operation::ReadProofBundle
        })
    ));
    let (status, document) = received.unwrap();
    assert_eq!(status, 200);
    let below = json!({"status": status, "document": document});
    assert_eq!(
        below["document"]["verification_status"]["state"],
        "unverified"
    );
    assert_eq!(
        below["document"]["verification_status"]["achieved"],
        "receipted"
    );
    assert_eq!(
        below["document"]["verification_status"]["requested"],
        "committed"
    );
    let receipts = store.list_object_ids(&tenant(), ObjectKind::Receipt).len();
    assert!(receipts > 0);
    drop(peer);
    drop(outbox);
    drop(store);
    drop(requests);
    // Reopen both actual durable components and start a new TLS listener.
    let mut store = Store::open(directory.0.join("owner")).unwrap();
    let mut outbox = Outbox::default();
    outbox.restore(&store, tenant(), hash).unwrap();
    assert_eq!(outbox.exact_signed_bytes(hash).unwrap(), raw);
    let requests = idempotency(&idempotency_root);
    assert!(matches!(
        requests.execute([7; 32], &raw, 12, |_| submit()).unwrap(),
        Outcome::RepeatedOriginal(_)
    ));
    assert_eq!(sends.load(Ordering::SeqCst), 1);
    let after = receipt_answer(&mut store, &outbox, &node_peer.url, &record, true);
    assert_eq!(after, before);
    assert_eq!(
        store.list_object_ids(&tenant(), ObjectKind::Receipt).len(),
        receipts
    );
    let reopened = receipt_peer(after);
    let recovered = query(&reopened, &record, Rung::Receipted);
    compare_evidence(
        "reopened",
        &BTreeMap::from([("receipt".to_owned(), actual)]),
        &BTreeMap::from([("receipt".to_owned(), recovered)]),
    )
    .unwrap();
}

#[test]
fn parity_synthetic_owner_rejects_invalid_node_evidence_and_mismatched_envelopes() {
    let record = recording();
    let mut damaged = record.clone();
    let mut canonical = bytes(&damaged["receiptJson"]["receipt"]);
    let last = canonical.last_mut().unwrap();
    *last ^= 1;
    damaged["receiptJson"]["receipt"] = json!(format!("0x{}", hex(&canonical)));
    let peer = node_peer(damaged, Arc::new(AtomicUsize::new(0)));
    let keys = node(&peer.url).receipt_keys().unwrap();
    let hash: [u8; 32] = bytes(&record["receiptJson"]["txHash"]).try_into().unwrap();
    assert!(VerifiedReceipt::verify(hash, &canonical, &keys).is_err());
    assert!(receipt_proof_operation(
        ProofBundleTarget::Receipt(hash),
        Rung::Receipted,
        &bytes(&record["rawTransaction"]),
        &canonical,
        &keys,
        125,
        123,
        [9; 32],
    )
    .is_err());
    let peer = Peer::start(true, |body| {
        let envelope = agent_rpc::decode(&body).unwrap();
        let dispatched = Dispatched {
            value: json!({"height":"123"}),
            verification: Some(VerificationStatus::Achieved(Rung::Submitted)),
        };
        // A correctly shaped owner response with the wrong request binding must fail.
        agent_rpc::success_body(RequestId(envelope.request_id.0 + 1), &dispatched, None).unwrap()
    });
    assert!(peer
        .client()
        .observe(
            Operation::TenantReadiness,
            RequestId(1572),
            &json!({}),
            None
        )
        .is_err());
}

#[test]
fn parity_synthetic_owner_preserves_refusals_and_detects_changed_or_missing_results() {
    let peer = Peer::start(true, |body| {
        let envelope = agent_rpc::decode(&body).unwrap();
        let error = agent_rpc::Rejection {
            class: layerx_agent_api::error::ErrorClass::CoreRejection,
            retriability: layerx_agent_api::error::Retriability::Terminal,
            request_id: envelope.request_id,
            reason: "node.rejected",
        };
        AgentRpcResponse {
            status: agent_rpc::http_status(&error),
            body: agent_rpc::error_body(&error),
        }
    });
    let actual = peer
        .client()
        .observe(
            Operation::TenantReadiness,
            RequestId(1572),
            &json!({}),
            None,
        )
        .unwrap();
    assert_eq!(actual["document"]["class"], "CoreRejection");
    assert_eq!(actual["document"]["request_id"], "1572");
    let reference = BTreeMap::from([("refusal".to_owned(), actual.clone())]);
    compare_evidence("same", &reference, &reference).unwrap();
    let mut changed = actual;
    changed["document"]["reason"] = json!("different");
    assert!(compare_evidence(
        "changed",
        &reference,
        &BTreeMap::from([("refusal".to_owned(), changed)])
    )
    .is_err());
    assert!(compare_evidence("missing", &reference, &BTreeMap::new()).is_err());
    assert!(digest(&json!("00")).is_err());
}
