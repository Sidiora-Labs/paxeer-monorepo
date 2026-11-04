//! Task 22.31.224 repair record.
//! seller_rpc_misroute: seller_routes_and_node_transport_complete_exact_and_metered,
//! node_account_failures_never_reach_settlement.
//! native_disclosure_legacy: native_unsigned_call_prepare_disclose_sign_and_submit_binding,
//! native_call_rejects_every_changed_response_and_intent_field.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd/src/human_runtime.rs must serve EVM native owner answers through native_call_preparation_answer/native_call_disclosure_answer.
//! Wiring request (domain 9): services/agent/crates/layerx-agent-api/src/identity.rs and services/agent/crates/layerx-agentd/src/agent_rpc_wire.rs need the EVM sender-bound native request replacing the explicitly refused legacy NativeSendPurposeV1.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd-host/src/payer.rs must implement WebPayer::view against the authenticated node root; old payer test adapters also need to handle accountId.
//! Baseline: no prior gate ledger or repair test existed; the explicit implementation-first,
//! wrapper-only gate order excludes the contradictory additional unchanged-tree test run.
//! Qualification attempt 1: revision 69fe40a07f08a49e888b2be15c03a4f0fda31b9a; hf-build passed;
//! hf-done exited 1 (/out/logs/verify-1.log): four passed, exact purchase used the wrong full-receipt decoder.
//! Repair: use the SDK receipt-evidence transport and explicitly verify the captured signature and log commitments.
//! Continuation repair: connect_node preserves the merged node root for receipt evidence;
//! connect appended /rpc and violated NodeRecording's POST / assertion before decoding.
//! Final gate revision and private evidence are emitted by hf-build/hf-done.

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agent_api::error::RequestId;
use layerx_agentd::policy::approval::{ApprovalContext, ApprovalRegistry};
use layerx_agentd::prepare::evm::{self, PrecompileCall, PrecompileTarget};
use layerx_agentd::tenant::Operation;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_crypto::payments::Payment;
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::tools::web::{
    web_tool, ExactPayment, GrantTerms, WebApproval, WebCall, WebConfig, WebPayer, WebPayerError,
    WebResult,
};
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_types::payload::{ModuleId, ModuleRegistry};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

// Compile the existing router itself, without adding a new workspace dependency.
#[allow(dead_code)]
#[path = "../../../../interop/crates/x-websearch/src/config.rs"]
mod config;
#[allow(dead_code)]
#[path = "../../../../interop/crates/x-websearch/src/server.rs"]
mod seller_server;

use layerx_mcp::boundary;
use layerx_mcp::tools::write as native_consumer;

fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    serde_json::from_slice(&std::fs::read(path).expect("producer recording"))
        .expect("recorded JSON")
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).expect("hex byte"))
        .collect()
}

fn word(value: &Value) -> [u8; 32] {
    unhex(value.as_str().expect("hex word"))
        .try_into()
        .expect("32 bytes")
}

fn read_http(stream: &mut TcpStream) -> (String, BTreeMap<String, String>, Vec<u8>) {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("timeout");
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader.read_line(&mut line).expect("HTTP line");
    let first = line.trim_end().to_owned();
    let mut headers = BTreeMap::new();
    loop {
        line.clear();
        assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
        if line == "\r\n" {
            break;
        }
        let (name, value) = line.split_once(':').expect("header delimiter");
        headers.insert(name.to_ascii_lowercase(), value.trim().to_owned());
    }
    let length = headers
        .get("content-length")
        .map_or(0, |v| v.parse().expect("length"));
    let mut body = vec![0; length];
    reader.read_exact(&mut body).expect("HTTP body");
    (first, headers, body)
}

fn respond(stream: &mut TcpStream, value: &Value) {
    let body = serde_json::to_vec(value).expect("response JSON");
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("response header");
    stream.write_all(&body).expect("response body");
}

/// Replays ledger and signed receipt observations emitted by the qualified node producer.
/// The real SequencerEndpoint owns all request encoding and response decoding.
struct NodeRecording {
    endpoint: String,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl NodeRecording {
    fn start(account_result: Option<Value>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("node listener");
        let endpoint = format!("http://{}", listener.local_addr().expect("node address"));
        let calls = Arc::new(Mutex::new(Vec::new()));
        let requests = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let metered = fixture("gateway/metered.json");
            let exact = fixture("gateway/exact.json");
            for stream in listener.incoming() {
                let mut stream = stream.expect("node connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (line, _, body) = read_http(&mut stream);
                assert_eq!(line, "POST / HTTP/1.1", "node uses the merged root");
                let request: Value = serde_json::from_slice(&body).expect("node request");
                requests.lock().expect("calls").push(request.clone());
                let data = request["params"][0]["data"].as_str().unwrap_or("");
                let result = if request["method"] == "eth_call" && data.starts_with("0x3f050cbc") {
                    assert_eq!(
                        request["params"][0]["to"],
                        "0x000000000000000000000000000000000000101a"
                    );
                    assert_eq!(request["params"][1], "pending");
                    assert_eq!(
                        &data[10..74],
                        format!("{}{}", "0".repeat(24), "ab".repeat(20))
                    );
                    if let Some(result) = &account_result {
                        result.clone()
                    } else {
                        let asset = &data[74..];
                        let grant = metered["ledger"]["grants"]
                            .as_array()
                            .expect("ledger grants")
                            .iter()
                            .find(|g| {
                                g["canonical"].as_str().expect("grant").get(192..256) == Some(asset)
                            })
                            .expect("producer's account for asset");
                        json!(format!(
                            "0x{}",
                            &grant["canonical"].as_str().expect("grant")[64..128]
                        ))
                    }
                } else {
                    metered["endpoints"]["gateway"]["*"]
                        .as_array()
                        .expect("metered RPC observations")
                        .iter()
                        .chain(
                            exact["endpoints"]["gateway"]["*"]
                                .as_array()
                                .expect("exact RPC observations"),
                        )
                        .find(|r| {
                            r["method"] == request["method"] && r["params"] == request["params"]
                        })
                        .unwrap_or_else(|| panic!("unrecorded node operation: {request}"))["result"]
                        .clone()
                };
                respond(
                    &mut stream,
                    &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
                );
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for NodeRecording {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.endpoint.strip_prefix("http://").expect("endpoint"));
        self.worker
            .take()
            .expect("node worker")
            .join()
            .expect("node assertions");
    }
}

struct NodePayer {
    node: SequencerEndpoint,
    receipt_client: layerx_sdk::rpc::RpcClient,
    authorization: layerx_proof::inclusion::SequencerAuthorization,
    submitted: usize,
    exact_hash: [u8; 32],
}
impl WebPayer for NodePayer {
    fn view(&mut self, call: &PrecompileCall) -> Result<Vec<u8>, WebPayerError> {
        let data = call.encode().map_err(|_| WebPayerError::Refused)?;
        self.node
            .call_pending(&call.target.address(), &data)
            .map_err(|_| WebPayerError::Unavailable)
    }
    fn standing_grant(&mut self, terms: &GrantTerms) -> Result<Option<[u8; 32]>, WebPayerError> {
        let ledger = fixture("gateway/metered.json");
        for record in ledger["ledger"]["grants"]
            .as_array()
            .expect("ledger grants")
            .iter()
            .filter(|record| {
                ["PAX", "SID", "USDC", "USDL"]
                    .contains(&record["name"].as_str().expect("grant name"))
            })
        {
            let Payment::IssueGrant(grant) = Payment::decode(
                ModuleId::Asset,
                7,
                &unhex(record["canonical"].as_str().expect("grant")),
                b"",
            )
            .map_err(|_| WebPayerError::Refused)?
            else {
                return Err(WebPayerError::Refused);
            };
            if grant.from == terms.payer_account
                && grant.recipient == terms.recipient
                && grant.asset == terms.asset
                && grant.purpose_hash == terms.purpose_hash
                && grant.per_draw_maximum >= terms.amount
            {
                return Ok(Some(grant.id));
            }
        }
        Ok(None)
    }
    fn submit(&mut self, call: &PrecompileCall) -> Result<ExactPayment, WebPayerError> {
        self.submitted += 1;
        assert_eq!(call.target, PrecompileTarget::Ledger);
        assert_eq!(
            call.selector,
            layerx_crypto::disclosure::call::ledger_selector(
                "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"
            )
        );
        let evidence = self
            .receipt_client
            .sequencer_receipt(self.exact_hash)
            .map_err(|_| WebPayerError::Unavailable)?;
        if evidence["txHash"] != format!("0x{}", hex(&self.exact_hash))
            || evidence["rung"] != "receipted"
            || evidence["status"] != "0x1"
        {
            return Err(WebPayerError::Refused);
        }
        let receipt = unhex(evidence["receipt"].as_str().ok_or(WebPayerError::Refused)?);
        let decoded =
            layerx_proof::receipt::verify_sequencer_signature(&receipt, &[self.authorization])
                .map_err(|_| WebPayerError::Refused)?;
        let logs = evidence["logs"]
            .as_array()
            .ok_or(WebPayerError::Refused)?
            .iter()
            .map(|log| {
                let address = unhex(log["address"].as_str().ok_or(WebPayerError::Refused)?)
                    .try_into()
                    .map_err(|_| WebPayerError::Refused)?;
                let topics = log["topics"]
                    .as_array()
                    .ok_or(WebPayerError::Refused)?
                    .iter()
                    .map(|topic| {
                        unhex(topic.as_str().ok_or(WebPayerError::Refused)?)
                            .try_into()
                            .map_err(|_| WebPayerError::Refused)
                    })
                    .collect::<Result<Vec<_>, _>>()?;
                let data = unhex(log["data"].as_str().ok_or(WebPayerError::Refused)?);
                let index = log["logIndex"]
                    .as_str()
                    .and_then(|i| i.strip_prefix("0x"))
                    .and_then(|i| u64::from_str_radix(i, 16).ok())
                    .ok_or(WebPayerError::Refused)?;
                Ok(SequencerLog::new(address, topics, data, index))
            })
            .collect::<Result<Vec<_>, WebPayerError>>()?;
        let verified = layerx_proof::sequencer_logs::verify_logs(&decoded, logs)
            .map_err(|_| WebPayerError::Refused)?;
        Ok(ExactPayment {
            tx_hash: self.exact_hash,
            receipt,
            logs: verified.into_logs(),
        })
    }
}

fn purchase(index: usize, account_result: Option<Value>) {
    let exchange = fixture("client-exchange.json")["exchange"]
        .as_array()
        .expect("exchange")
        .clone();
    let challenge = exchange[index].clone();
    let paid = exchange[index + 1].clone();
    let expected_signature = paid["request"]["headers"]["PAYMENT-SIGNATURE"].clone();
    let target = challenge["request"]["target"]
        .as_str()
        .expect("resource target")
        .to_owned();
    let seller_requests = Arc::new(AtomicU64::new(0));
    let requests = Arc::clone(&seller_requests);
    let expected = expected_signature.clone();
    let expected_target = target.clone();
    let mut routes = seller_server::RouteTable::new();
    routes
        .set(
            if index == 0 {
                seller_server::Route::Search
            } else {
                seller_server::Route::Fetch
            },
            move |request| {
                requests.fetch_add(1, Ordering::SeqCst);
                assert_eq!(request.method, "GET");
                let full_target = format!(
                    "{}{}",
                    request.path,
                    request
                        .query
                        .as_ref()
                        .map_or(String::new(), |q| format!("?{q}"))
                );
                assert_eq!(full_target, expected_target);
                if let Some(header) = request.header("PAYMENT-SIGNATURE") {
                    let received: Value =
                        serde_json::from_slice(&STANDARD.decode(header).expect("payment header"))
                            .expect("payment JSON");
                    assert_eq!(received["payload"], expected["payload"]);
                    assert_eq!(received["accepted"], expected["accepted"]);
                    let response = &paid["response"];
                    seller_server::Response {
                        status: 200,
                        content_type: "application/json".into(),
                        headers: vec![(
                            "PAYMENT-RESPONSE".into(),
                            STANDARD.encode(
                                serde_json::to_vec(&response["headers"]["PAYMENT-RESPONSE"])
                                    .expect("settlement"),
                            ),
                        )],
                        body: serde_json::to_vec(&response["body"]).expect("paid resource"),
                    }
                } else {
                    let mut required = challenge["response"]["headers"]["PAYMENT-REQUIRED"].clone();
                    required["resource"]["url"] = json!(format!(
                        "http://{}{}",
                        request.header("host").expect("seller authority"),
                        full_target
                    ));
                    seller_server::Response {
                        status: 402,
                        content_type: "application/json".into(),
                        headers: vec![(
                            "PAYMENT-REQUIRED".into(),
                            STANDARD.encode(serde_json::to_vec(&required).expect("challenge")),
                        )],
                        body: serde_json::to_vec(&json!({"paymentRequired": required}))
                            .expect("challenge body"),
                    }
                }
            },
        )
        .expect("real seller route");
    let seller = seller_server::Server::bind(
        "127.0.0.1:0".parse().expect("bind address"),
        seller_server::Limits::default(),
        routes,
    )
    .expect("real x-websearch router");
    let seller_endpoint = format!("http://{}", seller.local_addr().expect("seller address"));
    let running = seller.spawn().expect("start seller");
    let node = NodeRecording::start(account_result.clone());
    let buyer = fixture("gateway/buyer.json");
    let ledger = fixture("gateway/metered.json");
    let config = WebConfig::new(
        &seller_endpoint,
        buyer["payerDid"].as_str().expect("payer DID"),
        "0xabababababababababababababababababababab",
        unhex(
            ledger["ledger"]["receiverAddress"]
                .as_str()
                .expect("receiver"),
        )
        .try_into()
        .expect("address"),
        "eip155:125",
        word(&buyer["sequencerPublicKey"]),
        word(&buyer["sequencerId"]),
        buyer["firstBatchNumber"].as_u64().expect("first height"),
        buyer["lastBatchNumber"].as_u64().expect("last height"),
        Duration::from_secs(5),
        1,
    )
    .expect("web config");
    let metered = index == 0;
    let call = WebCall::from_arguments(if metered { "web.search" } else { "web.fetch" }, &if metered {
        json!({"query":"paxeer", "currency":"SID", "scheme":"metered", "idempotency_key":expected_signature["payload"]["idempotencyKey"]})
    } else {
        json!({"url":"https://paxeer.app/data.json", "currency":"USDC", "scheme":"exact", "idempotency_key":expected_signature["payload"]["idempotencyKey"]})
    }).expect("resource call");
    let registry = ApprovalRegistry::default();
    let approval = WebApproval {
        registry: &registry,
        policy: ApprovalPolicy {
            amount_threshold: u128::MAX,
        },
        context: ApprovalContext {
            request_id: [1; 32],
            tenant: layerx_agentd::store::TenantId::new("repair-native").expect("tenant"),
            agent: layerx_types::ids::Did::new(buyer["payerDid"].as_str().expect("DID").as_bytes())
                .expect("DID"),
            actor_address: [0xab; 20],
            session: layerx_agentd::session::SessionId([3; 32]),
            capability: layerx_agentd::capability::CapabilityId([4; 32]),
            policy_version: "policy-v1".into(),
        },
        current_sequence: 50,
    };
    let mut payer = NodePayer {
        node: SequencerEndpoint::new(&node.endpoint, &[]).expect("real node transport"),
        receipt_client: layerx_sdk::rpc::RpcClient::connect_node(&node.endpoint, None)
            .expect("real receipt evidence transport"),
        authorization: layerx_proof::inclusion::SequencerAuthorization::new(
            word(&buyer["sequencerId"]),
            word(&buyer["sequencerPublicKey"]),
            buyer["firstBatchNumber"].as_u64().expect("first height"),
            buyer["lastBatchNumber"].as_u64().expect("last height"),
        ),
        submitted: 0,
        exact_hash: if metered {
            [0; 32]
        } else {
            word(&expected_signature["payload"]["txHash"])
        },
    };
    let outcome = web_tool(&config, &call, &approval, &mut payer);
    if account_result.is_some() {
        assert!(
            outcome.is_err(),
            "invalid ledger facts must refuse: {outcome:?}"
        );
        assert_eq!(seller_requests.load(Ordering::SeqCst), 1);
        assert_eq!(payer.submitted, 0);
    } else {
        let outcome = outcome.unwrap_or_else(|error| {
            panic!("paid resource index {index} succeeds without seller RPC: {error:?}")
        });
        let settlement = outcome.settlement.expect("verified settlement");
        assert_eq!(
            settlement.transaction,
            exchange[index + 1]["response"]["headers"]["PAYMENT-RESPONSE"]["transaction"]
        );
        assert_eq!(payer.submitted, usize::from(!metered));
        assert_eq!(seller_requests.load(Ordering::SeqCst), 2);
        assert!(matches!(
            outcome.result,
            WebResult::Search { .. } | WebResult::Fetch { .. }
        ));
    }
    let calls = node.calls.lock().expect("RPC traffic");
    assert_eq!(
        calls
            .iter()
            .filter(|q| q["method"] == "eth_call"
                && q["params"][0]["data"]
                    .as_str()
                    .expect("call data")
                    .starts_with("0x3f050cbc"))
            .count(),
        1,
        "resolve account once, through payer transport, and retain it for the purchase"
    );
    drop(calls);
    running.shutdown().expect("stop real router");
}

#[test]
fn seller_routes_and_node_transport_complete_exact_and_metered() {
    purchase(0, None);
    purchase(6, None);
}

#[test]
fn node_account_failures_never_reach_settlement() {
    for value in [
        json!("0x"),
        json!(format!("0x{}", "00".repeat(32))),
        json!(format!("0x{}", "01".repeat(33))),
    ] {
        purchase(0, Some(value.clone()));
        purchase(6, Some(value));
    }
}

/// The actual body bytes cross a socket before the receiving crate decodes them.
fn body_transport(bytes: &[u8]) -> Vec<u8> {
    let (mut sender, mut receiver) =
        std::os::unix::net::UnixStream::pair().expect("daemon transport");
    receiver
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("socket deadline");
    let bytes = bytes.to_vec();
    let writer = thread::spawn(move || sender.write_all(&bytes).expect("send daemon body"));
    let mut received = Vec::new();
    receiver
        .read_to_end(&mut received)
        .expect("receive daemon body");
    writer.join().expect("transport writer");
    received
}

fn daemon_request(operation: Operation, request: &Value) -> Value {
    let credential =
        layerx_sdk::agent_envelope::EnvelopeCredential::new("repair-native", [7; 32], [8; 32], 1)
            .expect("session coordinates");
    let envelope = layerx_sdk::agent_envelope::encode_envelope(
        operation,
        RequestId(1),
        request,
        Some(&credential),
        operation
            .mutating()
            .then(|| layerx_agent_api::idempotency::Key::new([9; 32]).expect("request key")),
    )
    .expect("SDK request producer");
    let bytes = body_transport(&serde_json::to_vec(&envelope).expect("request encoding"));
    let decoded = layerx_agentd::agent_rpc::decode(&bytes).expect("daemon request consumer");
    assert_eq!(decoded.operation, operation);
    assert_eq!(
        decoded.credential.expect("credential").tenant().as_str(),
        "repair-native"
    );
    Value::Object(decoded.request)
}

/// Runs the production daemon response encoder and SDK consumer, with no test response schema.
fn daemon_transport(value: Value) -> Value {
    let response = layerx_agentd::agent_rpc::mcp_result_response(RequestId(1), value);
    let bytes = body_transport(&response.body);
    let wire: Value = serde_json::from_slice(&bytes).expect("daemon wire");
    layerx_sdk::agent_envelope::decode_response(response.status, &wire)
        .expect("SDK response codec")
        .expect("daemon success")
        .value
}

fn native_prepared() -> evm::Prepared {
    let payload =
        unhex(include_str!("../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim());
    let key = k256::ecdsa::SigningKey::from_slice(&[3; 32]).expect("EVM key");
    let from = layerx_crypto::secp256k1::evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    )
    .expect("sender");
    evm::prepare_bound(
        evm::EvmPreparationState {
            chain_id: 125,
            pending_nonce: 7,
            base_fee_per_gas: 1,
            head_height: 50,
        },
        evm::PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 5,
            maximum_payload_bytes: 65536,
        },
        evm::PrepareRequest {
            from,
            call: PrecompileCall::ledger_transfer(payload),
            expected_nonce: Some(7),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 65536,
        },
        100_000,
    )
    .expect("real daemon EVM preparation producer")
}

fn native_request(prepared: &evm::Prepared) -> Value {
    let d = &prepared.disclosure;
    let canonical = prepared.call.encode_unsigned();
    let id: [u8; 32] = Sha256::digest(&canonical).into();
    let raw = prepared.call.sign(&[3; 32]).expect("real owner signature");
    let (_, signature) = decode_signed(&raw).expect("typed signature");
    use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};
    let destination = d
        .counterparties
        .iter()
        .find(|p| p.role == CounterpartyRole::Recipient)
        .expect("recipient")
        .account;
    let amount = d
        .amounts
        .iter()
        .find(|a| a.role == AmountRole::Transfer)
        .expect("amount")
        .value;
    let payload_hash: [u8; 32] = Sha256::new()
        .chain_update(b"LXP/v1/payload-hash\0")
        .chain_update(&prepared.call.data)
        .finalize()
        .into();
    json!({"variant":"native_write_v1", "intent":{"destination":hex(&destination), "asset":hex(&d.asset), "amount":amount.to_string(), "idempotency_key":hex(&d.idempotency_key)},
        "canonical_bytes":hex(&canonical), "signer_address":hex(&prepared.from), "signature":hex(signature.as_bytes()),
        "preparation":{"variant":"native_send_v1", "activity":{"version":"1","module":"1","ordinal":"5"},
            "actor":"did:layerx:alice", "authority":hex(&prepared.from), "account_sequence":prepared.call.nonce.to_string(),
            "fee_limit":d.fee_limit.to_string(), "not_before":d.expiry.not_before.to_string(), "not_after":d.expiry.not_after.to_string(),
            "payload":hex(&prepared.call.data), "payload_hash":hex(&payload_hash), "idempotency_key":hex(&d.idempotency_key),
            "purpose":{"purpose":{"preparation_id":hex(&id), "canonical_digest":hex(&id), "agent_did":"did:layerx:alice"}}}})
}

fn consume_native(
    prepared: &evm::Prepared,
    arguments: &Value,
    changed: Option<(&str, Value, bool)>,
) -> Result<Value, boundary::BoundaryRefusal> {
    native_consumer::execute_native_alias(
        "wallet.send",
        arguments,
        &ModuleRegistry::new(&[]).expect("registry"),
        prepared.disclosure.idempotency_key,
        |operation, request| {
            assert_eq!(
                operation,
                Operation::Prepare,
                "held preparation must never sign or submit"
            );
            let request = daemon_request(operation, request);
            let disclose = request["variant"] == "native_disclosure_v1";
            let mut value = if disclose {
                native_consumer::native_call_disclosure_answer(prepared)?
            } else {
                native_consumer::native_call_preparation_answer(prepared, Some([7; 32]))?
            };
            if let Some((field, replacement, at_disclosure)) = &changed {
                if *at_disclosure == disclose {
                    value[*field] = replacement.clone();
                }
            }
            Ok(daemon_transport(value))
        },
    )
}

#[test]
fn native_unsigned_call_prepare_disclose_sign_and_submit_binding() {
    let prepared = native_prepared();
    let arguments = native_request(&prepared);
    let held = consume_native(&prepared, &arguments, None)
        .expect("unsigned EIP-1559 accepted by real consumer");
    assert_eq!(held["approval_required"], true);
    assert_eq!(held["canonical_bytes"], arguments["canonical_bytes"]);
    let signing = layerx_agentd::sign::SigningPreparation {
        transaction: prepared.clone(),
        actor_address: prepared.from,
    };
    let external = layerx_agentd::sign::external(&signing).expect("daemon signing package");
    assert_eq!(external.canonical_bytes, prepared.call.encode_unsigned());
    let signature = unhex(arguments["signature"].as_str().expect("signature"))
        .try_into()
        .expect("65 bytes");
    let signed = layerx_agentd::sign::attach_external_signature(&signing, signature)
        .expect("daemon sign consumer");
    let admitted = layerx_agentd::sign::verify_before_submit(
        &signed,
        &signing,
        &prepared.from,
        &ModuleRegistry::new(&[]).expect("registry"),
    )
    .expect("daemon submit admission");
    assert_eq!(admitted.exact_bytes(), signed);
    assert_eq!(admitted.activity_id(), transaction_hash(&signed));
    let mut other = prepared.from;
    other[0] ^= 1;
    assert!(layerx_agentd::sign::verify_before_submit(
        &signed,
        &signing,
        &other,
        &ModuleRegistry::new(&[]).expect("registry")
    )
    .is_err());

    // The node peer admits a real signed type-2 call and returns its real transaction id.
    // All durable submission transitions, bytes and tracking answers are owned by agentd.
    let listener = TcpListener::bind("127.0.0.1:0").expect("submission node");
    let node_url = format!("http://{}", listener.local_addr().expect("node address"));
    let expected_call = prepared.call.clone();
    let expected_sender = prepared.from;
    let sent = Arc::new(Mutex::new(Vec::new()));
    let received = Arc::clone(&sent);
    let peer = thread::spawn(move || {
        for expected_method in ["eth_chainId", "eth_sendRawTransaction"] {
            let (mut stream, _) = listener.accept().expect("node request");
            let (line, _, body) = read_http(&mut stream);
            assert_eq!(line, "POST / HTTP/1.1");
            let request: Value = serde_json::from_slice(&body).expect("node JSON");
            assert_eq!(request["method"], expected_method);
            let result = if expected_method == "eth_chainId" {
                json!("0x7d")
            } else {
                let raw = unhex(request["params"][0].as_str().expect("signed call"));
                let (call, signature) = decode_signed(&raw).expect("actual node call codec");
                assert_eq!(call, expected_call);
                assert_eq!(
                    layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                        .expect("sender"),
                    expected_sender
                );
                *received.lock().expect("submitted bytes") = raw.clone();
                json!(format!("0x{}", hex(&transaction_hash(&raw))))
            };
            respond(
                &mut stream,
                &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
            );
        }
    });
    let node = layerx_agentd::node_rpc::NodeRpc::connect(&layerx_agentd::node_rpc::NodeRpcConfig {
        url: node_url,
        chain_id: 125,
        deadline: Duration::from_secs(5),
    })
    .expect("actual daemon node transport");
    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
    let directory = std::env::temp_dir().join(format!(
        "repair-22-31-224-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    ));
    std::fs::create_dir(&directory).expect("durable test directory");
    let mut store =
        layerx_agentd::store::Store::open(directory.join("store")).expect("real daemon store");
    let tenant = layerx_agentd::store::TenantId::new("repair-native").expect("tenant");
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    let mut stages = Vec::new();
    let mut signed_bytes = None;
    let outcome = native_consumer::execute_native_alias(
        "wallet.send",
        &arguments,
        &ModuleRegistry::new(&[]).expect("registry"),
        prepared.disclosure.idempotency_key,
        |operation, request| {
            stages.push(operation);
            let request = daemon_request(operation, request);
            let value = match operation {
                Operation::Prepare if request["variant"] == "native_disclosure_v1" => {
                    native_consumer::native_call_disclosure_answer(&prepared)?
                }
                Operation::Prepare => {
                    native_consumer::native_call_preparation_answer(&prepared, None)?
                }
                Operation::Sign => {
                    assert_eq!(request["preparation_ref"], held["preparation_id"]);
                    let signature = unhex(request["signature"].as_str().expect("owner signature"))
                        .try_into()
                        .expect("65 bytes");
                    signed_bytes = Some(
                        layerx_agentd::sign::attach_external_signature(&signing, signature)
                            .expect("actual daemon signer"),
                    );
                    // The MCP consumer does not use the sign answer; return the verified preparation.
                    native_consumer::native_call_preparation_answer(&prepared, None)?
                }
                Operation::Submit => {
                    assert_eq!(request["preparation_ref"], held["preparation_id"]);
                    assert_eq!(request["variant"], "native_send_submit_v1");
                    let raw = signed_bytes.as_ref().expect("sign stage before submission");
                    let admitted = layerx_agentd::sign::verify_before_submit(
                        raw,
                        &signing,
                        &prepared.from,
                        &ModuleRegistry::new(&[]).expect("registry"),
                    )
                    .expect("actual daemon submission verification");
                    let response = layerx_agentd::human_runtime::submit_raw_write(
                        &mut outbox,
                        &mut store,
                        tenant.clone(),
                        admitted.into_exact_bytes(),
                        &node,
                    )
                    .expect("actual daemon submission");
                    let (tracked, _) = layerx_agentd::agent_rpc_dispatch::decode_write_answer(
                        RequestId(1),
                        response,
                    )
                    .expect("actual daemon tracked response");
                    json!({"activity_id":tracked["tx_hash"], "submission":tracked, "receipt":null})
                }
                Operation::Track => {
                    layerx_agentd::agent_rpc_dispatch::track_write_answer(
                        RequestId(1),
                        &outbox,
                        request["submission_ref"]
                            .as_str()
                            .expect("submission reference"),
                    )
                    .expect("actual daemon tracking")
                    .0
                }
                _ => panic!("unexpected owner operation"),
            };
            Ok(daemon_transport(value))
        },
    )
    .expect("prepare/disclose/sign/submit/track through the repaired consumer");
    peer.join().expect("node assertions");
    assert_eq!(
        stages,
        vec![
            Operation::Prepare,
            Operation::Prepare,
            Operation::Sign,
            Operation::Submit,
            Operation::Track
        ]
    );
    assert_eq!(outcome["state"], "Submitted");
    assert_eq!(outcome["submission_ref"], hex(&transaction_hash(&signed)));
    assert_eq!(*sent.lock().expect("actual bytes"), signed);
    assert_eq!(
        outbox
            .exact_signed_bytes(transaction_hash(&signed))
            .expect("durable bytes"),
        signed
    );
    drop(store);
    std::fs::remove_dir_all(directory).expect("test state cleanup");
}

#[test]
fn native_call_rejects_every_changed_response_and_intent_field() {
    let prepared = native_prepared();
    let arguments = native_request(&prepared);
    for (field, replacement) in [
        ("version", json!("2")),
        ("preparation_id", json!("00".repeat(32))),
        ("canonical_bytes", json!("00")),
        ("signing_preimage", json!("00".repeat(32))),
        (
            "activity",
            json!({"version":"1","module":"1","ordinal":"6"}),
        ),
        (
            "activity",
            json!({"version":"1","module":"2","ordinal":"5"}),
        ),
        (
            "activity",
            json!({"version":"2","module":"1","ordinal":"5"}),
        ),
        ("approval_id", Value::Null),
        ("approval_required", json!(false)),
    ] {
        assert!(
            consume_native(&prepared, &arguments, Some((field, replacement, false))).is_err(),
            "changed preparation {field}"
        );
    }
    for (field, replacement) in [
        ("version", json!("2")),
        ("preparation_id", json!("00".repeat(32))),
        ("canonical_bytes", json!("00")),
        ("disclosure_digest", json!("00".repeat(32))),
        ("activity_type", json!("65542")),
        ("actor", json!("00".repeat(20))),
        ("authority", json!("00".repeat(20))),
        ("asset", json!("00".repeat(32))),
        ("fee_limit", json!("1")),
        ("not_before", json!("1")),
        ("not_after", json!("1")),
        ("payload_expires_at", json!("1")),
        ("idempotency_key", json!("00".repeat(32))),
    ] {
        assert!(
            consume_native(&prepared, &arguments, Some((field, replacement, true))).is_err(),
            "changed disclosure {field}"
        );
    }
    for (field, replacement) in [
        ("destination", json!("00".repeat(32))),
        ("asset", json!("00".repeat(32))),
        ("amount", json!("1")),
        ("idempotency_key", json!("00".repeat(32))),
    ] {
        let mut changed = arguments.clone();
        changed["intent"][field] = replacement;
        assert!(
            consume_native(&prepared, &changed, None).is_err(),
            "changed intent {field}"
        );
    }
    for (field, replacement) in [
        ("actor", json!("did:layerx:other")),
        ("authority", json!("00".repeat(20))),
        ("account_sequence", json!("8")),
        ("fee_limit", json!("1")),
        ("not_before", json!("1")),
        ("not_after", json!("1")),
        ("payload", json!("00")),
        ("payload_hash", json!("00".repeat(32))),
        ("idempotency_key", json!("00".repeat(32))),
    ] {
        let mut changed = arguments.clone();
        changed["preparation"][field] = replacement;
        assert!(
            consume_native(&prepared, &changed, None).is_err(),
            "changed request {field}"
        );
    }
    for field in ["signature", "signer_address", "canonical_bytes"] {
        let mut changed = arguments.clone();
        let value = changed[field].as_str().expect("hex").to_owned();
        changed[field] = json!(format!(
            "{}{}",
            if value.starts_with('0') { "1" } else { "0" },
            &value[1..]
        ));
        assert!(
            consume_native(&prepared, &changed, None).is_err(),
            "changed authentication {field}"
        );
    }
    let mut damaged = prepared.clone();
    damaged.from[0] ^= 1;
    assert!(native_consumer::native_call_preparation_answer(&damaged, None).is_err());
    assert!(native_consumer::native_call_disclosure_answer(&damaged).is_err());
}

#[test]
fn native_legacy_owner_contract_remains_refused() {
    let prepared = native_prepared();
    let request = native_request(&prepared);
    assert!(layerx_agentd::agent_rpc::validate_mcp_native_request(
        Operation::Prepare, &request["preparation"],
    ).is_err(), "the retired native owner contract must remain refused until domain 9 wires its sender-bound replacement");
}
