//! Task 22.33.1551 repair record.
//! web_hold_not_call: restart_hold_approval_and_exact_retry_through_served_route,
//! restart_refuses_changed_resource_offer_and_sender, changed_held_preparation_is_defective,
//! invalid_transaction_preparation_never_creates_a_hold.
//! Durable corruption: restart_refuses_changed_durable_transaction_and_sender.
//! Interrupted hold: restart_recovers_preparation_before_hold_publication.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd-host/src/payer.rs must
//! implement WebPayer::{approval_resources,prepare_transaction,submit_transaction} using
//! the owner's shared durable approval store/expiry/limits and exact EIP-1559 signing path.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd-host/src/payer.rs must
//! obtain receiver-owned draw approval for above-threshold existing metered grants;
//! the payer's unsigned call cannot substitute for the sidecar's actual draw.
//! Exact-payment codec: restart_hold_approval_and_exact_retry_through_served_route
//! drives the web producer through EVM preparation, disclosure binding, durable hold,
//! reopen, approval and exact retry; pay_exact_codec_checks also proves strict ABI refusal.
//! Qualification: ed5358cce66b288c77c861bf58cf80d354663ecd;
//! cg spec done 22.33.1551 -f layerx-merge --root /work/paxeer-monorepo; exit 1;
//! /out/logs/verify-1.log: invalid_transaction_preparation_never_creates_a_hold passed;
//! restart_hold_approval_and_exact_retry_through_served_route,
//! restart_refuses_changed_resource_offer_and_sender, changed_held_preparation_is_defective,
//! restart_refuses_changed_durable_transaction_and_sender and
//! restart_recovers_preparation_before_hold_publication failed at the missing payExact codec.
//! Baseline: no earlier gate ledger or repair test existed; the explicit implementation-first,
//! wrapper-only gate order excludes the contradictory unchanged-tree verification run.
//! Continuation qualification: 36c6e1febc17409a21de554ff48341aadcc07607;
//! cg spec done 22.33.1551 -f layerx-merge --root /work/paxeer-monorepo; exit 1;
//! /out/logs/verify-1.log: three passed; the exact-retry recording omitted the JSON
//! Content-Type required by the real RPC client, and two substring checks did not
//! match the served ApprovalChanged refusal. Exact refusal equality is asserted below.
//! Qualification revision and private evidence are emitted by hf-build/hf-done.

use std::collections::BTreeSet;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_agent_api::prepare::Prepared;
use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
    DecisionRequest,
};
use layerx_agentd::budget::{
    self, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::policy::approval::{ApprovalRegistry, ApprovalState, ApproverId};
use layerx_agentd::prepare::evm::{self, PrecompileCall};
use layerx_agentd::prepare::PreparationLifecycle;
use layerx_agentd::session::{open, OpenRequest, SessionCredential, SessionId, SessionRegistry};
use layerx_agentd::session_control::SessionControl;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::decode_unsigned;
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::boundary::{AgentSurface, ProgramReads, ToolBoundary};
use layerx_mcp::server::{Server, WebBoundary, WebRoute};
use layerx_mcp::stdio::{Bound, Session};
use layerx_mcp::tools::web::{
    ExactPayment, WebApprovalResources, WebConfig, WebPayer, WebPayerError,
};
use layerx_proof::sequencer_logs::SequencerLog;
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const HEIGHT: u64 = 20;
static NEXT: AtomicU64 = AtomicU64::new(0);

fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    serde_json::from_slice(&fs::read(path).expect("committed producer recording"))
        .expect("recording JSON")
}
fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn unhex(value: &str) -> Vec<u8> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    (0..value.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i + 2], 16).expect("hex"))
        .collect()
}
fn word(value: &Value) -> [u8; 32] {
    unhex(value.as_str().expect("word"))
        .try_into()
        .expect("word width")
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let root = fs::canonicalize(std::env::temp_dir())
            .expect("temporary root")
            .join(format!(
                "repair-22-33-1551-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
        fs::create_dir(&root).expect("directory");
        fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).expect("directory mode");
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove state");
    }
}

// Identity provisioning is outside the repaired server; all sessions and capabilities below
// are minted, persisted and restored by the daemon's actual registration/open paths.
struct IdentityLedger(CoreIdentity);
impl IdentityResolver for IdentityLedger {
    fn resolve(&mut self, _: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

struct Enrolled {
    directory: Directory,
    control: SessionControl,
    credential: SessionCredential,
}
impl Enrolled {
    fn new(scopes: &[&str], agent: &str) -> Self {
        let directory = Directory::new();
        let tenant = TenantId::new("tenant-a").expect("tenant");
        let mut store = Store::open(directory.0.join("store")).expect("store");
        let capability = Capability::new(
            CapabilityId([9; 32]),
            tenant.clone(),
            CapabilityDimensions {
                activity_types: BTreeSet::from([5, 7]),
                counterparties: BTreeSet::from([[2; 32]]),
                assets: BTreeSet::from([[3; 32]]),
                amount_ceiling: 100,
                rate_ceiling: RateCeiling {
                    maximum_uses: 100,
                    window_sequences: 10,
                },
                purposes: BTreeSet::from(["service-payment".to_owned()]),
                expiry_sequence: 400,
            },
        )
        .expect("capability");
        capability.persist(&mut store).expect("persist capability");
        let mut ledger = IdentityLedger(CoreIdentity {
            canonical_bytes: agent.as_bytes().to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(capability.id.0)],
        });
        let identity = register(
            &mut store,
            tenant.clone(),
            Did::new(agent.as_bytes()).expect("DID"),
            &mut ledger,
        )
        .expect("register");
        let mut sessions = SessionRegistry::default();
        let credential = open(
            &mut store,
            &mut sessions,
            &identity,
            OpenRequest {
                session_id: SessionId([7; 32]),
                token_id: [8; 32],
                tenant,
                agent: identity.did().clone(),
                authority: ProtocolAuthority::CapabilityGrant(capability.id.0),
                permitted_activity_types: BTreeSet::from([5, 7]),
                scopes: scopes.iter().map(|scope| (*scope).to_owned()).collect(),
                expiry_seconds: None,
                expiry_sequence: 300,
                opening_client: "mcp".to_owned(),
                policy_version: "policy-v1".to_owned(),
            },
            HEIGHT,
        )
        .expect("open authenticated session")
        .credential();
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(BudgetLimiter::new(vec![limit()]).expect("limiter")),
        );
        Self {
            directory,
            control,
            credential,
        }
    }
}
fn limit() -> LimitConfig {
    LimitConfig {
        id: LimitId([10; 16]),
        name: "mcp".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1000,
        consumed: 0,
    }
}
fn rpc<B: ToolBoundary>(session: &mut Session<B>, method: &str, params: Value) -> Value {
    let input = json!({"jsonrpc":"2.0", "id":1, "method":method, "params": params});
    let mut source = std::io::Cursor::new(format!("{input}\n"));
    let mut output = Vec::new();
    session
        .serve(&mut source, &mut output)
        .expect("MCP line transport");
    serde_json::from_slice(&output).expect("MCP response")
}

fn read_http(stream: &mut TcpStream) -> (String, Vec<(String, String)>, Vec<u8>) {
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .expect("read deadline");
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    reader.read_line(&mut line).expect("request line");
    let target = line
        .split_whitespace()
        .nth(1)
        .expect("request target")
        .to_owned();
    let mut headers = Vec::new();
    loop {
        line.clear();
        assert_ne!(reader.read_line(&mut line).expect("header"), 0);
        if line == "\r\n" {
            break;
        }
        let (name, value) = line.split_once(':').expect("header delimiter");
        headers.push((name.to_ascii_lowercase(), value.trim().to_owned()));
    }
    let length = headers
        .iter()
        .find(|(name, _)| name == "content-length")
        .map_or(0, |(_, value)| value.parse().expect("body length"));
    let mut body = vec![0; length];
    reader.read_exact(&mut body).expect("request body");
    (target, headers, body)
}

fn respond(stream: &mut TcpStream, status: u16, headers: &[(&str, String)], body: &[u8]) {
    write!(
        stream,
        "HTTP/1.1 {status} Recorded\r\nContent-Length: {}\r\nConnection: close\r\n",
        body.len()
    )
    .expect("response line");
    for (name, value) in headers {
        write!(stream, "{name}: {value}\r\n").expect("response header");
    }
    stream.write_all(b"\r\n").expect("header end");
    stream.write_all(body).expect("response body");
}

/// Committed seller, ledger and signed receipt transcripts are the web component's peers.
struct Recording {
    address: String,
    challenge: Arc<Mutex<Value>>,
    signatures: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Recording {
    fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("recording listener");
        let address = listener
            .local_addr()
            .expect("recording address")
            .to_string();
        let exchange = fixture("client-exchange.json");
        let mut required =
            exchange["exchange"][6]["response"]["headers"]["PAYMENT-REQUIRED"].clone();
        required["resource"]["url"] = json!(format!(
            "http://{address}{}",
            exchange["exchange"][6]["request"]["target"]
                .as_str()
                .expect("target")
        ));
        let challenge = Arc::new(Mutex::new(required));
        let shared_challenge = Arc::clone(&challenge);
        let signatures = Arc::new(Mutex::new(Vec::new()));
        let paid_signatures = Arc::clone(&signatures);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let ledger = fixture("gateway/metered.json");
            let exact = fixture("gateway/exact.json");
            for stream in listener.incoming() {
                let mut stream = stream.expect("recording connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (target, headers, body) = read_http(&mut stream);
                if target.starts_with("/v1/programs/") {
                    let body = json!({"program":"cc".repeat(32),"accounts":[],"freshness":{"observed_sequence":HEIGHT,"valid_through":400}});
                    respond(&mut stream, 200, &[], body.to_string().as_bytes());
                } else if target == "/" {
                    let request: Value = serde_json::from_slice(&body).expect("node request");
                    let result = if request["method"] == "eth_call" {
                        let data = unhex(request["params"][0]["data"].as_str().expect("ABI bytes"));
                        assert_eq!(
                            request["params"][0]["to"],
                            "0x000000000000000000000000000000000000101a"
                        );
                        assert_eq!(request["params"][1], "pending");
                        assert_eq!(
                            &data[..4],
                            &layerx_crypto::disclosure::call::ledger_selector(
                                "accountId(bytes32,bytes32)"
                            )
                        );
                        let asset = hex(&data[36..68]);
                        let record = ledger["ledger"]["grants"]
                            .as_array()
                            .expect("ledger grants")
                            .iter()
                            .find(|record| {
                                record["canonical"]
                                    .as_str()
                                    .expect("grant bytes")
                                    .get(192..256)
                                    == Some(asset.as_str())
                            })
                            .expect("recorded account for asset");
                        json!(format!(
                            "0x{}",
                            &record["canonical"].as_str().expect("grant bytes")[64..128]
                        ))
                    } else {
                        assert_eq!(request["method"], "pax_sequencerReceipt");
                        exact["endpoints"]["gateway"]["*"]
                            .as_array()
                            .expect("receipt transcript")
                            .iter()
                            .find(|record| {
                                record["method"] == request["method"]
                                    && record["params"] == request["params"]
                            })
                            .expect("recorded receipt request")["result"]
                            .clone()
                    };
                    let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result});
                    respond(
                        &mut stream,
                        200,
                        &[("Content-Type", "application/json".to_owned())],
                        body.to_string().as_bytes(),
                    );
                } else {
                    assert!(target.starts_with("/fetch?url="), "{target}");
                    if let Some((_, signature)) =
                        headers.iter().find(|(name, _)| name == "payment-signature")
                    {
                        let signature: Value = serde_json::from_slice(
                            &STANDARD.decode(signature).expect("signature base64"),
                        )
                        .expect("signature JSON");
                        let expected =
                            &exchange["exchange"][7]["request"]["headers"]["PAYMENT-SIGNATURE"];
                        assert_eq!(signature["payload"], expected["payload"]);
                        assert_eq!(signature["accepted"], expected["accepted"]);
                        paid_signatures
                            .lock()
                            .expect("signature capture")
                            .push(signature);
                        let reply = &exchange["exchange"][7]["response"];
                        let response = STANDARD.encode(
                            serde_json::to_vec(&reply["headers"]["PAYMENT-RESPONSE"])
                                .expect("settlement bytes"),
                        );
                        respond(
                            &mut stream,
                            200,
                            &[("PAYMENT-RESPONSE", response)],
                            &serde_json::to_vec(&reply["body"]).expect("content bytes"),
                        );
                    } else {
                        let required = shared_challenge.lock().expect("challenge");
                        let header = STANDARD
                            .encode(serde_json::to_vec(&*required).expect("challenge bytes"));
                        respond(&mut stream, 402, &[("PAYMENT-REQUIRED", header)], b"");
                    }
                }
            }
        });
        Self {
            address,
            challenge,
            signatures,
            stop,
            worker: Some(worker),
        }
    }
    fn endpoint(&self) -> String {
        format!("http://{}", self.address)
    }
}
impl Drop for Recording {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        TcpStream::connect(&self.address).expect("wake recording");
        self.worker
            .take()
            .expect("recording worker")
            .join()
            .expect("recording assertions");
    }
}

#[derive(Default)]
struct Traffic {
    preparations: usize,
    submitted: Vec<Prepared>,
}

struct RecordedNodePayer {
    resources: WebApprovalResources,
    node: SequencerEndpoint,
    receipt: layerx_sdk::rpc::RpcClient,
    traffic: Arc<Mutex<Traffic>>,
    nonce: u64,
    sender: [u8; 20],
    corrupt_hash: bool,
}
impl WebPayer for RecordedNodePayer {
    fn approval_resources(&self) -> Option<WebApprovalResources> {
        Some(self.resources.clone())
    }
    fn view(&mut self, call: &PrecompileCall) -> Result<Vec<u8>, WebPayerError> {
        self.node
            .call_pending(
                &call.target.address(),
                &call.encode().map_err(|_| WebPayerError::Refused)?,
            )
            .map_err(|_| WebPayerError::Unavailable)
    }
    fn prepare_transaction(
        &mut self,
        call: &PrecompileCall,
    ) -> Result<evm::Prepared, WebPayerError> {
        self.traffic.lock().expect("traffic").preparations += 1;
        let mut prepared = evm::prepare_bound(
            evm::EvmPreparationState {
                chain_id: 125,
                pending_nonce: self.nonce,
                base_fee_per_gas: 1,
                head_height: HEIGHT,
            },
            evm::PreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                maximum_payload_bytes: 65536,
            },
            evm::PrepareRequest {
                from: self.sender,
                call: call.clone(),
                expected_nonce: Some(self.nonce),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 65536,
            },
            150_000,
        )
        .map_err(|_| WebPayerError::Refused)?;
        if self.corrupt_hash {
            prepared.signing_hash[0] ^= 1;
        }
        Ok(prepared)
    }
    fn submit(&mut self, _: &PrecompileCall) -> Result<ExactPayment, WebPayerError> {
        panic!("an approved transaction must never use the re-preparing submit path")
    }
    fn submit_transaction(&mut self, prepared: &Prepared) -> Result<ExactPayment, WebPayerError> {
        let call = decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes())
            .expect("consumer decodes actual held transaction");
        assert_eq!(
            call.signing_hash().as_slice(),
            prepared.signing_preimage.as_bytes()
        );
        assert_eq!(
            call.nonce, 7,
            "retry must retain the originally observed nonce"
        );
        assert_eq!(prepared.disclosure.actor.as_str(), hex(&[0xab; 20]));
        self.traffic
            .lock()
            .expect("traffic")
            .submitted
            .push(prepared.clone());
        let buyer = fixture("gateway/buyer.json");
        let hash = word(&buyer["exact"]["USDC"]["txHash"]);
        let evidence = self
            .receipt
            .sequencer_receipt(hash)
            .map_err(|_| WebPayerError::Unavailable)?;
        let logs = evidence["logs"]
            .as_array()
            .expect("recorded logs")
            .iter()
            .map(|log| {
                SequencerLog::new(
                    unhex(log["address"].as_str().expect("log address"))
                        .try_into()
                        .expect("address width"),
                    log["topics"]
                        .as_array()
                        .expect("log topics")
                        .iter()
                        .map(word)
                        .collect(),
                    unhex(log["data"].as_str().expect("log data")),
                    u64::from_str_radix(
                        log["logIndex"]
                            .as_str()
                            .expect("log index")
                            .trim_start_matches("0x"),
                        16,
                    )
                    .expect("index quantity"),
                )
            })
            .collect();
        Ok(ExactPayment {
            tx_hash: hash,
            receipt: unhex(evidence["receipt"].as_str().expect("signed receipt")),
            logs,
        })
    }
}

fn resources(root: &std::path::Path) -> WebApprovalResources {
    let store = Arc::new(Mutex::new(
        Store::open(root).expect("durable approval store"),
    ));
    WebApprovalResources {
        expiry: Arc::new(ApprovalExpiry::from_shared_store(Arc::clone(&store))),
        store,
        limiter: Arc::new(
            BudgetLimiter::new(vec![LimitConfig {
                id: LimitId([12; 16]),
                name: "web spend".to_owned(),
                scope: LimitScope::Tenant([1; 32]),
                ceiling: 1_000_000,
                consumed: 0,
            }])
            .expect("approval budgets"),
        ),
        limits: vec![LimitId([12; 16])],
    }
}
fn key() -> [u8; 32] {
    word(&fixture("gateway/buyer.json")["exact"]["USDC"]["idempotencyKey"])
}
fn tenant() -> TenantId {
    TenantId::new("tenant-a").expect("tenant")
}

fn session(
    enrolled: &Enrolled,
    recording: &Recording,
    registry: Arc<ApprovalRegistry>,
    resources: WebApprovalResources,
    traffic: Arc<Mutex<Traffic>>,
    nonce: u64,
    configured_sender: [u8; 20],
    prepared_sender: [u8; 20],
    corrupt_hash: bool,
) -> Session<WebBoundary<ProgramReads>> {
    let server = Server::bind(
        enrolled.control.clone(),
        enrolled.credential.clone(),
        CapabilityId([9; 32]),
        HEIGHT,
        enrolled
            .directory
            .0
            .join(format!("audit-{}", NEXT.fetch_add(1, Ordering::Relaxed))),
    )
    .expect("authenticated MCP server");
    let buyer = fixture("gateway/buyer.json");
    let receiver = unhex(
        fixture("gateway/metered.json")["ledger"]["receiverAddress"]
            .as_str()
            .expect("receiver"),
    )
    .try_into()
    .expect("receiver address width");
    let config = WebConfig::new(
        &recording.endpoint(),
        buyer["payerDid"].as_str().expect("payer DID"),
        &format!("0x{}", hex(&configured_sender)),
        receiver,
        "eip155:125",
        word(&buyer["sequencerPublicKey"]),
        word(&buyer["sequencerId"]),
        buyer["firstBatchNumber"].as_u64().expect("first height"),
        buyer["lastBatchNumber"].as_u64().expect("last height"),
        Duration::from_secs(5),
        1,
    )
    .expect("web config");
    let payer = RecordedNodePayer {
        resources,
        traffic,
        nonce,
        sender: prepared_sender,
        corrupt_hash,
        node: SequencerEndpoint::new(&recording.endpoint(), &[]).expect("real node client"),
        receipt: layerx_sdk::rpc::RpcClient::connect_node(&recording.endpoint(), None)
            .expect("real receipt codec"),
    };
    let route = WebRoute::new(
        config,
        registry,
        ApprovalPolicy {
            amount_threshold: 0,
        },
        Box::new(payer),
    );
    let surface = AgentSurface::new(
        &recording.address,
        "b".repeat(48),
        &"cc".repeat(32),
        Duration::from_secs(5),
    )
    .expect("daemon peer");
    let boundary = server
        .route_web(ProgramReads::new(surface), route)
        .expect("real served web route");
    Session::new(Bound::Full(Box::new(server)), boundary)
}
fn invoke(session: &mut Session<WebBoundary<ProgramReads>>) -> Value {
    rpc(
        session,
        "tools/call",
        json!({"name":"web.fetch","arguments":{"url":"https://paxeer.app/data.json","currency":"USDC","scheme":"exact","idempotency_key":hex(&key())}}),
    )
}

fn approve(
    registry: &ApprovalRegistry,
    resources: &WebApprovalResources,
    prepared: &Prepared,
) -> layerx_agentd::approval::ApprovalDecision {
    let queue = ApprovalSubmissionQueue::default();
    let result = ApprovalService::new(registry, &resources.limiter, &resources.expiry)
        .approve(
            DecisionRequest {
                tenant: &tenant(),
                approval_id: key(),
                idempotency_key: &DecisionKey::new("approve-web").expect("decision key"),
                approver: ApproverId::new("operator").expect("approver"),
                current_sequence: HEIGHT,
            },
            prepared,
            &queue,
        )
        .expect("durable approval service");
    if result.outcome == ApprovalOutcome::Granted {
        assert_eq!(
            queue.prepared(result.submission_ref.expect("released reference")),
            Some(prepared.clone()),
            "approval queues the byte-exact original preparation"
        );
    }
    result
}

fn pay_exact_codec_checks(call: &layerx_crypto::evm_transaction::Eip1559Call, held: &Prepared) {
    use layerx_crypto::disclosure::call::{bind_call, decode_pay_exact, reencode, validate_call};
    use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};

    let sender = [0xab; 20];
    let payment = decode_pay_exact(call).expect("decode real web producer's exact call");
    let disclosed = bind_call(call, sender).expect("bind exact call to sender");
    assert_eq!(
        reencode(&disclosed).expect("reencode"),
        call.encode_unsigned()
    );
    assert_eq!(
        disclosed.signing_digest().expect("signing digest"),
        call.signing_hash()
    );
    assert_eq!(disclosed.actor, sender);
    assert_eq!(disclosed.authority, sender);
    assert_eq!(disclosed.activity_type.ordinal(), 5);
    assert_eq!(disclosed.counterparties.len(), 2);
    assert_eq!(disclosed.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(disclosed.counterparties[0].account, payment.payer);
    assert_eq!(
        disclosed.counterparties[1].role,
        CounterpartyRole::Recipient
    );
    assert_eq!(disclosed.counterparties[1].account, payment.recipient);
    assert_eq!(disclosed.amounts.len(), 1);
    assert_eq!(disclosed.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosed.amounts[0].value, payment.amount);
    assert_eq!(payment.amount, held.disclosure.amounts.values()[0].amount.0);
    assert_eq!(disclosed.asset, payment.asset);
    assert_eq!(disclosed.expiry.payload_expires_at, payment.expires_at_ms);
    assert_eq!(payment.expires_at_ms, held.expiry.0 * 1000);
    assert_eq!(disclosed.idempotency_key, key());
    assert_eq!(payment.idempotency_key, key());
    assert_eq!(disclosed.fee_limit, held.disclosure.fee_limit.0);
    assert_eq!(
        disclosed.payload_sequence().expect("no payload sequence"),
        None
    );
    validate_call(call, &disclosed, sender).expect("consumer validates the original sender");
    assert!(validate_call(call, &disclosed, [0xac; 20]).is_err());

    for field in ["amount", "expiry", "asset", "counterparty", "key", "actor"] {
        let mut changed = disclosed.clone();
        match field {
            "amount" => changed.amounts[0].value += 1,
            "expiry" => changed.expiry.payload_expires_at += 1,
            "asset" => changed.asset[0] ^= 1,
            "counterparty" => changed.counterparties[0].account[0] ^= 1,
            "key" => changed.idempotency_key[0] ^= 1,
            _ => changed.actor[0] ^= 1,
        }
        assert!(reencode(&changed).is_err(), "altered {field} disclosure");
    }
    for mutation in [
        "amount_width",
        "expiry_width",
        "truncated",
        "trailing",
        "target",
        "selector",
        "zero_amount",
        "zero_key",
        "fee_overflow",
    ] {
        let mut changed = call.clone();
        match mutation {
            "amount_width" => changed.data[4 + 3 * 32] = 1,
            "expiry_width" => changed.data[4 + 4 * 32] = 1,
            "truncated" => {
                changed.data.pop();
            }
            "trailing" => changed.data.push(0),
            "target" => changed.to[19] ^= 1,
            "selector" => changed.data[0] ^= 1,
            "zero_amount" => changed.data[4 + 3 * 32..4 + 4 * 32].fill(0),
            "zero_key" => changed.data[4 + 5 * 32..].fill(0),
            _ => changed.max_fee_per_gas = u128::MAX,
        }
        assert!(bind_call(&changed, sender).is_err(), "malformed {mutation}");
    }
}

#[test]
fn restart_hold_approval_and_exact_retry_through_served_route() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer DID"),
    );
    let root = enrolled.directory.0.join("approvals");
    let original = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&original.store)));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        original.clone(),
        Arc::clone(&traffic),
        7,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    let response = invoke(&mut served);
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert!(
        response.to_string().contains(&hex(&key())),
        "served hold has the actual request identity: {response}"
    );
    let ticket = registry
        .ticket(key())
        .expect("ticket lookup")
        .expect("durable hold");
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    let prepared = original
        .preparation(&tenant(), key())
        .expect("retained preparation")
        .expect("prepared");
    let call = decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes())
        .expect("held bytes are unsigned EIP-1559");
    pay_exact_codec_checks(&call, &prepared);
    assert_eq!(prepared.disclosure.actor.as_str(), hex(&[0xab; 20]));
    assert_eq!(
        prepared.disclosure.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(call.encode_unsigned()))
    );
    assert_eq!(prepared.signing_preimage.as_bytes(), call.signing_hash());
    assert_eq!(prepared.disclosure, ticket.disclosure);
    assert_eq!(prepared.disclosure.fee_limit.0, 450_000);
    assert!(original
        .limiter
        .has_reservation(key())
        .expect("reservation"));
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
    assert!(recording.signatures.lock().expect("signatures").is_empty());
    registry
        .validate_registry(&tenant(), &ModuleRegistry::new(&[]).expect("call registry"))
        .expect("production registry validates sender-bound transaction");
    drop(served);
    drop(registry);
    drop(original);

    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        registry
            .replay_tenant(&tenant(), &recovered.limiter)
            .expect("reopen durable holds and budget reservations"),
        1
    );
    registry
        .validate_registry(&tenant(), &ModuleRegistry::new(&[]).expect("call registry"))
        .expect("recovered bytes and sender remain bound");
    assert_eq!(
        recovered
            .preparation(&tenant(), key())
            .expect("recovered preparation"),
        Some(prepared.clone())
    );
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        recovered.clone(),
        Arc::clone(&traffic),
        999,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    let pending = invoke(&mut served);
    assert_eq!(pending["result"]["isError"], true, "{pending}");
    assert!(
        pending.to_string().contains("awaiting approval"),
        "{pending}"
    );
    assert_eq!(
        traffic.lock().expect("traffic").preparations,
        1,
        "recovery cannot prepare a different nonce"
    );
    let excess = budget::reserve(
        &recovered.limiter,
        &ReservationRequest {
            id: [88; 32],
            amount: 1_000_000,
            expiry_sequence: 1000,
            current_sequence: HEIGHT,
            applicable_limits: recovered.limits.clone(),
        },
    );
    assert!(
        excess.is_err(),
        "the recovered payment reservation still counts against the ceiling"
    );
    assert_eq!(
        approve(&registry, &recovered, &prepared).outcome,
        ApprovalOutcome::Granted
    );
    for _ in 0..2 {
        let response = invoke(&mut served);
        assert_eq!(response["result"]["isError"], false, "{response}");
        assert_eq!(
            response["result"]["structuredContent"]["result"]["settlement"]["verificationLevel"],
            "sequencer-signed",
            "{response}"
        );
    }
    let traffic = traffic.lock().expect("traffic");
    assert_eq!(traffic.preparations, 1);
    assert_eq!(traffic.submitted, vec![prepared.clone(), prepared]);
    let signatures = recording.signatures.lock().expect("signatures");
    assert_eq!(signatures.len(), 2);
    assert_eq!(
        signatures[0], signatures[1],
        "retry preserves the complete payment signature envelope"
    );
}

#[test]
fn restart_refuses_changed_resource_offer_and_sender() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer"),
    );
    let root = enrolled.directory.0.join("approvals");
    let initial = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&initial.store)));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        initial.clone(),
        Arc::clone(&traffic),
        7,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    assert_eq!(invoke(&mut served)["result"]["isError"], true);
    let prepared = initial
        .preparation(&tenant(), key())
        .expect("preparation")
        .expect("held");
    drop(served);
    drop(registry);
    drop(initial);
    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        registry
            .replay_tenant(&tenant(), &recovered.limiter)
            .expect("replay"),
        1
    );
    assert_eq!(
        approve(&registry, &recovered, &prepared).outcome,
        ApprovalOutcome::Granted
    );
    let original_challenge = recording.challenge.lock().expect("challenge").clone();
    for mutation in ["resource", "offer", "sender"] {
        *recording.challenge.lock().expect("challenge") = original_challenge.clone();
        let sender = if mutation == "sender" {
            [0xac; 20]
        } else {
            [0xab; 20]
        };
        if mutation == "resource" {
            recording.challenge.lock().expect("challenge")["resource"]["description"] =
                json!("changed resource commitment");
        }
        if mutation == "offer" {
            for offer in recording.challenge.lock().expect("challenge")["accepts"]
                .as_array_mut()
                .expect("offers")
            {
                offer["amount"] = json!("1001");
            }
        }
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            recovered.clone(),
            Arc::clone(&traffic),
            999,
            sender,
            sender,
            false,
        );
        let response = invoke(&mut served);
        assert_eq!(
            response["result"]["isError"], true,
            "changed {mutation}: {response}"
        );
        assert_eq!(
            response["result"]["structuredContent"]["refusal"],
            "the daemon response is unusable: the web spend differs from the disclosure under approval",
            "changed {mutation}: {response}"
        );
    }
    assert_eq!(traffic.lock().expect("traffic").preparations, 1);
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
    assert!(recording.signatures.lock().expect("signatures").is_empty());
}

#[test]
fn changed_held_preparation_is_defective() {
    for mutation in ["bytes", "sender", "signing_hash"] {
        let recording = Recording::start();
        let enrolled = Enrolled::new(
            &["write:web:fetch"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer"),
        );
        let state = resources(&enrolled.directory.0.join("approvals"));
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&state.store)));
        let traffic = Arc::new(Mutex::new(Traffic::default()));
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            state.clone(),
            Arc::clone(&traffic),
            7,
            [0xab; 20],
            [0xab; 20],
            false,
        );
        assert_eq!(invoke(&mut served)["result"]["isError"], true);
        let mut prepared = state
            .preparation(&tenant(), key())
            .expect("retained preparation")
            .expect("prepared");
        match mutation {
            "bytes" => {
                let mut call = decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes())
                    .expect("producer call");
                call.nonce += 1;
                prepared.unsigned_canonical_bytes =
                    layerx_agent_api::prepare::CanonicalBytes::new(call.encode_unsigned())
                        .expect("changed bytes");
            }
            "sender" => {
                prepared.disclosure.actor =
                    layerx_agent_api::identity::AgentDid::new(hex(&[0xac; 20]))
                        .expect("changed actor")
            }
            _ => {
                prepared.signing_preimage =
                    layerx_agent_api::prepare::SigningPreimage::new(vec![9; 32])
                        .expect("changed signing hash")
            }
        }
        assert_eq!(
            approve(&registry, &state, &prepared).outcome,
            ApprovalOutcome::Defective,
            "{mutation}"
        );
        assert!(!state
            .limiter
            .has_reservation(key())
            .expect("defective reservation released"));
        assert_eq!(invoke(&mut served)["result"]["isError"], true);
        assert!(traffic.lock().expect("traffic").submitted.is_empty());
        assert!(recording.signatures.lock().expect("signatures").is_empty());
    }
}

#[test]
fn invalid_transaction_preparation_never_creates_a_hold() {
    for (sender, corrupt_hash) in [([0xac; 20], false), ([0xab; 20], true)] {
        let recording = Recording::start();
        let enrolled = Enrolled::new(
            &["write:web:fetch"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer"),
        );
        let state = resources(&enrolled.directory.0.join("approvals"));
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&state.store)));
        let traffic = Arc::new(Mutex::new(Traffic::default()));
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            state.clone(),
            Arc::clone(&traffic),
            7,
            [0xab; 20],
            sender,
            corrupt_hash,
        );
        let response = invoke(&mut served);
        assert_eq!(response["result"]["isError"], true, "{response}");
        assert!(registry.hold_ids().expect("hold IDs").is_empty());
        assert!(state
            .preparation(&tenant(), key())
            .expect("journal")
            .is_none());
        assert!(!state.limiter.has_reservation(key()).expect("budget"));
        assert!(traffic.lock().expect("traffic").submitted.is_empty());
        assert!(recording.signatures.lock().expect("signatures").is_empty());
    }
}

#[test]
fn restart_refuses_changed_durable_transaction_and_sender() {
    for mutation in ["bytes", "sender", "signing_hash"] {
        let recording = Recording::start();
        let enrolled = Enrolled::new(
            &["write:web:fetch"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer"),
        );
        let root = enrolled.directory.0.join("approvals");
        let original = resources(&root);
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&original.store)));
        let traffic = Arc::new(Mutex::new(Traffic::default()));
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            original.clone(),
            Arc::clone(&traffic),
            7,
            [0xab; 20],
            [0xab; 20],
            false,
        );
        assert_eq!(invoke(&mut served)["result"]["isError"], true);
        drop(served);
        drop(registry);
        drop(original);
        let mut store = Store::open(&root).expect("stopped component's durable store");
        let mut id = b"web-spend-v1:".to_vec();
        id.extend_from_slice(&key());
        let storage_key =
            TenantKey::new(tenant(), ObjectKind::PreparedActivity, id).expect("journal key");
        let mut record: Value =
            serde_json::from_slice(store.get(&storage_key).expect("producer's journal").bytes())
                .expect("journal codec");
        let byte = record[mutation][0].as_u64().expect("retained byte");
        record[mutation][0] = json!(byte ^ 1);
        store
            .put_local(
                storage_key,
                serde_json::to_vec(&record).expect("corrupted journal"),
            )
            .expect("persist changed state");
        drop(store);
        let recovered = resources(&root);
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
        assert_eq!(
            registry
                .replay_tenant(&tenant(), &recovered.limiter)
                .expect("real approval recovery"),
            1
        );
        let mut served = session(
            &enrolled,
            &recording,
            registry,
            recovered,
            Arc::clone(&traffic),
            999,
            [0xab; 20],
            [0xab; 20],
            false,
        );
        let response = invoke(&mut served);
        assert_eq!(
            response["result"]["isError"], true,
            "{mutation}: {response}"
        );
        assert_eq!(
            response["result"]["structuredContent"]["refusal"],
            "the daemon response is unusable: the web spend differs from the disclosure under approval",
            "{mutation}: {response}"
        );
        assert_eq!(traffic.lock().expect("traffic").preparations, 1);
        assert!(traffic.lock().expect("traffic").submitted.is_empty());
        assert!(recording.signatures.lock().expect("signatures").is_empty());
    }
}

#[test]
fn restart_recovers_preparation_before_hold_publication() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer"),
    );
    let root = enrolled.directory.0.join("approvals");
    let original = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&original.store)));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        original.clone(),
        Arc::clone(&traffic),
        7,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    assert_eq!(invoke(&mut served)["result"]["isError"], true);
    let prepared = original
        .preparation(&tenant(), key())
        .expect("journal")
        .expect("prepared");
    drop(served);
    drop(registry);
    drop(original);
    let mut store = Store::open(&root).expect("stopped durable store");
    let mut id = b"approval-hold-v1:".to_vec();
    id.extend_from_slice(&key());
    let hold_key = TenantKey::new(tenant(), ObjectKind::PreparedActivity, id).expect("hold key");
    // The journal write precedes publication of the approval hold. Retain exactly the
    // producer's journal to reproduce a stop between those two durable writes.
    assert!(store
        .remove_local(&hold_key)
        .expect("interrupted publication"));
    drop(store);
    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        registry
            .replay_tenant(&tenant(), &recovered.limiter)
            .expect("interrupted hold recovery"),
        0
    );
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        recovered.clone(),
        Arc::clone(&traffic),
        999,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    assert_eq!(invoke(&mut served)["result"]["isError"], true);
    assert_eq!(
        registry
            .ticket(key())
            .expect("ticket")
            .expect("recovered hold")
            .disclosure,
        prepared.disclosure
    );
    assert!(recovered
        .limiter
        .has_reservation(key())
        .expect("recovered reservation"));
    assert_eq!(
        traffic.lock().expect("traffic").preparations,
        1,
        "the interrupted hold must retain the original transaction"
    );
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
}
