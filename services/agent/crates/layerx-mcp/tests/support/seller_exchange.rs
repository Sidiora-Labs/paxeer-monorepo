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
#[path = "../../../../../interop/crates/x-websearch/src/config.rs"]
pub mod config;
#[allow(dead_code)]
#[path = "../../../../../interop/crates/x-websearch/src/server.rs"]
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

pub fn seller_routes_and_node_transport_complete_exact_and_metered() {
    purchase(0, None);
    purchase(6, None);
}

pub fn node_account_failures_never_reach_settlement() {
    for value in [
        json!("0x"),
        json!(format!("0x{}", "00".repeat(32))),
        json!(format!("0x{}", "01".repeat(33))),
    ] {
        purchase(0, Some(value.clone()));
        purchase(6, Some(value));
    }
}
