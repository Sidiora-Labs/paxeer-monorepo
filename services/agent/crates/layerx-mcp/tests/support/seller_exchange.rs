use crate::option_assertion;
use crate::result_assertion;

use crate::test_clock;

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agentd::policy::approval::{ApprovalContext, ApprovalRegistry};
use layerx_agentd::prepare::evm::{PrecompileCall, PrecompileTarget};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::payments::Payment;
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::tools::web::{
    web_tool, ExactPayment, GrantTerms, WebApproval, WebCall, WebConfig, WebPayer, WebPayerError,
    WebResult,
};
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_types::payload::ModuleId;
use serde_json::{json, Value};

use crate::seller_server;

fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(crate::manifest_dir())
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    result_assertion::required(
        serde_json::from_slice(&result_assertion::required(
            std::fs::read(path),
            "producer recording",
        )),
        "recorded JSON",
    )
}

fn hex(bytes: &[u8]) -> String {
    layerx_programs::hex::encode(bytes)
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| result_assertion::required(u8::from_str_radix(&text[i..i + 2], 16), "hex byte"))
        .collect()
}

fn word(value: &Value) -> [u8; 32] {
    result_assertion::required(
        unhex(option_assertion::required(value.as_str(), "hex word")).try_into(),
        "32 bytes",
    )
}

fn read_http(stream: &mut TcpStream) -> (String, BTreeMap<String, String>, Vec<u8>) {
    result_assertion::required(
        stream.set_read_timeout(Some(Duration::from_secs(5))),
        "timeout",
    );
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    result_assertion::required(reader.read_line(&mut line), "HTTP line");
    let first = line.trim_end().to_owned();
    let mut headers = BTreeMap::new();
    loop {
        line.clear();
        assert_ne!(
            result_assertion::required(reader.read_line(&mut line), "HTTP header"),
            0
        );
        if line == "\r\n" {
            break;
        }
        let (name, value) = option_assertion::required(line.split_once(':'), "header delimiter");
        headers.insert(name.to_ascii_lowercase(), value.trim().to_owned());
    }
    let length = headers
        .get("content-length")
        .map_or(0, |v| result_assertion::required(v.parse(), "length"));
    let mut body = vec![0; length];
    result_assertion::required(reader.read_exact(&mut body), "HTTP body");
    (first, headers, body)
}

fn respond(stream: &mut TcpStream, value: &Value) {
    let body = result_assertion::required(serde_json::to_vec(value), "response JSON");
    result_assertion::required(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()), "response header");
    result_assertion::required(stream.write_all(&body), "response body");
}

/// Replays ledger and signed receipt observations emitted by the qualified node producer.
/// The real `SequencerEndpoint` owns all request encoding and response decoding.
struct NodeRecording {
    endpoint: String,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl NodeRecording {
    fn start(account_result: Option<Value>) -> Self {
        let listener =
            result_assertion::required(TcpListener::bind("127.0.0.1:0"), "node listener");
        let endpoint = format!(
            "http://{}",
            result_assertion::required(listener.local_addr(), "node address")
        );
        let calls = Arc::new(Mutex::new(Vec::new()));
        let requests = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let metered = fixture("gateway/metered.json");
            let exact = fixture("gateway/exact.json");
            for stream in listener.incoming() {
                let mut stream = result_assertion::required(stream, "node connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (line, _, body) = read_http(&mut stream);
                assert_eq!(line, "POST / HTTP/1.1", "node uses the merged root");
                let request: Value =
                    result_assertion::required(serde_json::from_slice(&body), "node request");
                result_assertion::required(requests.lock(), "calls").push(request.clone());
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
                        let grant = option_assertion::required(
                            option_assertion::required(
                                metered["ledger"]["grants"].as_array(),
                                "ledger grants",
                            )
                            .iter()
                            .find(|g| {
                                option_assertion::required(g["canonical"].as_str(), "grant")
                                    .get(192..256)
                                    == Some(asset)
                            }),
                            "producer's account for asset",
                        );
                        json!(format!(
                            "0x{}",
                            &option_assertion::required(grant["canonical"].as_str(), "grant")
                                [64..128]
                        ))
                    }
                } else {
                    option_assertion::required(
                        metered["endpoints"]["gateway"]["*"].as_array(),
                        "metered RPC observations",
                    )
                    .iter()
                    .chain(option_assertion::required(
                        exact["endpoints"]["gateway"]["*"].as_array(),
                        "exact RPC observations",
                    ))
                    .find(|r| r["method"] == request["method"] && r["params"] == request["params"])
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
        let _ = TcpStream::connect(option_assertion::required(
            self.endpoint.strip_prefix("http://"),
            "endpoint",
        ));
        result_assertion::required(
            option_assertion::required(self.worker.take(), "node worker").join(),
            "node assertions",
        );
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
        for record in
            option_assertion::required(ledger["ledger"]["grants"].as_array(), "ledger grants")
                .iter()
                .filter(|record| {
                    ["PAX", "SID", "USDC", "USDL"].contains(&option_assertion::required(
                        record["name"].as_str(),
                        "grant name",
                    ))
                })
        {
            let Payment::IssueGrant(grant) = Payment::decode(
                ModuleId::Asset,
                7,
                &unhex(option_assertion::required(
                    record["canonical"].as_str(),
                    "grant",
                )),
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

fn purchase(index: usize, account_result: Option<&Value>) {
    let exchange = option_assertion::required(
        fixture("client-exchange.json")["exchange"].as_array(),
        "exchange",
    )
    .clone();
    let challenge = exchange[index].clone();
    let paid = exchange[index + 1].clone();
    let expected_signature = paid["request"]["headers"]["PAYMENT-SIGNATURE"].clone();
    let target =
        option_assertion::required(challenge["request"]["target"].as_str(), "resource target")
            .to_owned();
    let (seller_endpoint, running, seller_requests) =
        start_seller(index, challenge, paid, expected_signature.clone(), target);
    let node = NodeRecording::start(account_result.cloned());
    let buyer = fixture("gateway/buyer.json");
    let ledger = fixture("gateway/metered.json");
    let config = purchase_config(&seller_endpoint, &buyer, &ledger);
    let metered = index == 0;
    let call = result_assertion::required(
        WebCall::from_arguments(
            if metered { "web.search" } else { "web.fetch" },
            &if metered {
                json!({"query":"paxeer", "currency":"SID", "scheme":"metered", "idempotency_key":expected_signature["payload"]["idempotencyKey"]})
            } else {
                json!({"url":"https://paxeer.app/data.json", "currency":"USDC", "scheme":"exact", "idempotency_key":expected_signature["payload"]["idempotencyKey"]})
            },
        ),
        "resource call",
    );
    let registry = ApprovalRegistry::default();
    let clock = test_clock::TestClock::new(50);
    let approval = purchase_approval(&registry, &buyer, &clock);
    let mut payer = NodePayer {
        node: result_assertion::required(
            SequencerEndpoint::new(&node.endpoint, &[]),
            "real node transport",
        ),
        receipt_client: result_assertion::required(
            layerx_sdk::rpc::RpcClient::connect_node(&node.endpoint, None),
            "real receipt evidence transport",
        ),
        authorization: layerx_proof::inclusion::SequencerAuthorization::new(
            word(&buyer["sequencerId"]),
            word(&buyer["sequencerPublicKey"]),
            option_assertion::required(buyer["firstBatchNumber"].as_u64(), "first height"),
            option_assertion::required(buyer["lastBatchNumber"].as_u64(), "last height"),
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
        let settlement = option_assertion::required(outcome.settlement, "verified settlement");
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
    let calls = result_assertion::required(node.calls.lock(), "RPC traffic");
    assert_eq!(
        calls
            .iter()
            .filter(|q| q["method"] == "eth_call"
                && option_assertion::required(q["params"][0]["data"].as_str(), "call data")
                    .starts_with("0x3f050cbc"))
            .count(),
        1,
        "resolve account once, through payer transport, and retain it for the purchase"
    );
    drop(calls);
    result_assertion::required(running.shutdown(), "stop real router");
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
        purchase(0, Some(&value));
        purchase(6, Some(&value));
    }
}

fn start_seller(
    index: usize,
    challenge: Value,
    paid: Value,
    expected_signature: Value,
    target: String,
) -> (String, seller_server::RunningServer, Arc<AtomicU64>) {
    let seller_requests = Arc::new(AtomicU64::new(0));
    let requests = Arc::clone(&seller_requests);
    let expected = expected_signature;
    let expected_target = target;
    let mut routes = seller_server::RouteTable::new();
    result_assertion::required(
        routes.set(
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
                    let received: Value = result_assertion::required(
                        serde_json::from_slice(&result_assertion::required(
                            STANDARD.decode(header),
                            "payment header",
                        )),
                        "payment JSON",
                    );
                    assert_eq!(received["payload"], expected["payload"]);
                    assert_eq!(received["accepted"], expected["accepted"]);
                    let response = &paid["response"];
                    seller_server::Response {
                        status: 200,
                        content_type: "application/json".into(),
                        headers: vec![(
                            "PAYMENT-RESPONSE".into(),
                            STANDARD.encode(result_assertion::required(
                                serde_json::to_vec(&response["headers"]["PAYMENT-RESPONSE"]),
                                "settlement",
                            )),
                        )],
                        body: result_assertion::required(
                            serde_json::to_vec(&response["body"]),
                            "paid resource",
                        ),
                    }
                } else {
                    let mut required = challenge["response"]["headers"]["PAYMENT-REQUIRED"].clone();
                    required["resource"]["url"] = json!(format!(
                        "http://{}{}",
                        option_assertion::required(request.header("host"), "seller authority"),
                        full_target
                    ));
                    seller_server::Response {
                        status: 402,
                        content_type: "application/json".into(),
                        headers: vec![(
                            "PAYMENT-REQUIRED".into(),
                            STANDARD.encode(result_assertion::required(
                                serde_json::to_vec(&required),
                                "challenge",
                            )),
                        )],
                        body: result_assertion::required(
                            serde_json::to_vec(&json!({"paymentRequired": required})),
                            "challenge body",
                        ),
                    }
                }
            },
        ),
        "real seller route",
    );
    let seller = result_assertion::required(
        seller_server::Server::bind(
            result_assertion::required("127.0.0.1:0".parse(), "bind address"),
            seller_server::Limits::default(),
            routes,
        ),
        "real x-websearch router",
    );
    let seller_endpoint = format!(
        "http://{}",
        result_assertion::required(seller.local_addr(), "seller address")
    );
    let running = result_assertion::required(seller.spawn(), "start seller");
    (seller_endpoint, running, seller_requests)
}

fn purchase_config(seller_endpoint: &str, buyer: &Value, ledger: &Value) -> WebConfig {
    result_assertion::required(
        WebConfig::new(
            seller_endpoint,
            option_assertion::required(buyer["payerDid"].as_str(), "payer DID"),
            "0xabababababababababababababababababababab",
            result_assertion::required(
                unhex(option_assertion::required(
                    ledger["ledger"]["receiverAddress"].as_str(),
                    "receiver",
                ))
                .try_into(),
                "address",
            ),
            "eip155:125",
            word(&buyer["sequencerPublicKey"]),
            word(&buyer["sequencerId"]),
            option_assertion::required(buyer["firstBatchNumber"].as_u64(), "first height"),
            option_assertion::required(buyer["lastBatchNumber"].as_u64(), "last height"),
            Duration::from_secs(5),
            1,
        ),
        "web config",
    )
}

fn purchase_approval<'a>(
    registry: &'a ApprovalRegistry,
    buyer: &Value,
    clock: &'a dyn layerx_types::clock::Clock,
) -> WebApproval<'a> {
    WebApproval {
        registry,
        policy: ApprovalPolicy {
            amount_threshold: u128::MAX,
        },
        context: ApprovalContext {
            request_id: [1; 32],
            tenant: result_assertion::required(
                layerx_agentd::store::TenantId::new("repair-native"),
                "tenant",
            ),
            agent: result_assertion::required(
                layerx_types::ids::Did::new(
                    option_assertion::required(buyer["payerDid"].as_str(), "DID").as_bytes(),
                ),
                "DID",
            ),
            actor_address: [0xab; 20],
            session: layerx_agentd::session::SessionId([3; 32]),
            capability: layerx_agentd::capability::CapabilityId([4; 32]),
            policy_version: "policy-v1".into(),
        },
        current_sequence: 50,
        clock,
    }
}
