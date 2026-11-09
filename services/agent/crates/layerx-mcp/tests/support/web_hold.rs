#[path = "clock.rs"]
pub(crate) mod test_clock;

pub(crate) use std::collections::BTreeSet;
pub(crate) use std::fs;
pub(crate) use std::io::{BufRead, BufReader, Read, Write};
pub(crate) use std::net::{TcpListener, TcpStream};
pub(crate) use std::os::unix::fs::PermissionsExt;
pub(crate) use std::path::PathBuf;
pub(crate) use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
pub(crate) use std::sync::{Arc, Mutex};
pub(crate) use std::thread::{self, JoinHandle};
pub(crate) use std::time::Duration;

pub(crate) use base64::{engine::general_purpose::STANDARD, Engine as _};
pub(crate) use layerx_agent_api::prepare::Prepared;
pub(crate) use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
    DecisionRequest,
};
pub(crate) use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
pub(crate) use layerx_agentd::capability::{
    Capability, CapabilityDimensions, CapabilityId, RateCeiling,
};
pub(crate) use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
pub(crate) use layerx_agentd::policy::approval::{ApprovalRegistry, ApprovalState, ApproverId};
pub(crate) use layerx_agentd::prepare::evm::{self, PrecompileCall};
pub(crate) use layerx_agentd::prepare::PreparationLifecycle;
pub(crate) use layerx_agentd::session::{
    open, OpenRequest, SessionCredential, SessionId, SessionRegistry,
};
pub(crate) use layerx_agentd::session_control::SessionControl;
pub(crate) use layerx_agentd::store::{Store, TenantId};
pub(crate) use layerx_client::sequencer::SequencerEndpoint;
pub(crate) use layerx_crypto::evm_transaction::decode_unsigned;
pub(crate) use layerx_mcp::tools::web::{
    ExactPayment, GrantTerms, WebApprovalResources, WebPayer, WebPayerError,
};
pub(crate) use layerx_proof::sequencer_logs::SequencerLog;
pub(crate) use layerx_types::ids::Did;
pub(crate) use layerx_types::verify::VerificationLevel;
pub(crate) use serde_json::{json, Value};
pub(crate) use sha2::{Digest as _, Sha256};

pub(crate) const HEIGHT: u64 = 20;
pub(crate) static NEXT: AtomicU64 = AtomicU64::new(0);

pub(crate) fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    serde_json::from_slice(&fs::read(path).expect("committed producer recording"))
        .expect("recording JSON")
}
pub(crate) fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
pub(crate) fn unhex(value: &str) -> Vec<u8> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    (0..value.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&value[i..i + 2], 16).expect("hex"))
        .collect()
}
pub(crate) fn word(value: &Value) -> [u8; 32] {
    unhex(value.as_str().expect("word"))
        .try_into()
        .expect("word width")
}
pub(crate) struct Directory(pub(crate) PathBuf);
impl Directory {
    pub(crate) fn new() -> Self {
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
pub(crate) struct IdentityLedger(CoreIdentity);
impl IdentityResolver for IdentityLedger {
    fn resolve(&mut self, _: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

pub(crate) struct Enrolled {
    pub(crate) directory: Directory,
    pub(crate) control: SessionControl,
    pub(crate) credential: SessionCredential,
}
impl Enrolled {
    pub(crate) fn new(scopes: &[&str], agent: &str) -> Self {
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
pub(crate) fn limit() -> LimitConfig {
    LimitConfig {
        id: LimitId([10; 16]),
        name: "mcp".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1000,
        consumed: 0,
    }
}
pub(crate) fn read_http(stream: &mut TcpStream) -> (String, Vec<(String, String)>, Vec<u8>) {
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

pub(crate) fn respond(
    stream: &mut TcpStream,
    status: u16,
    headers: &[(&str, String)],
    body: &[u8],
) {
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
pub(crate) struct Recording {
    pub(crate) address: String,
    pub(crate) challenge: Arc<Mutex<Value>>,
    pub(crate) signatures: Arc<Mutex<Vec<Value>>>,
    pub(crate) ledger: Arc<Mutex<Value>>,
    pub(crate) stop: Arc<AtomicBool>,
    pub(crate) worker: Option<JoinHandle<()>>,
}
impl Recording {
    pub(crate) fn start() -> Self {
        Self::start_exchange(6)
    }

    pub(crate) fn start_search() -> Self {
        Self::start_exchange(0)
    }

    fn start_exchange(exchange_index: usize) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("recording listener");
        let address = listener
            .local_addr()
            .expect("recording address")
            .to_string();
        let exchange = fixture("client-exchange.json");
        let mut required =
            exchange["exchange"][exchange_index]["response"]["headers"]["PAYMENT-REQUIRED"].clone();
        required["resource"]["url"] = json!(format!(
            "http://{address}{}",
            exchange["exchange"][exchange_index]["request"]["target"]
                .as_str()
                .expect("target")
        ));
        let challenge = Arc::new(Mutex::new(required));
        let shared_challenge = Arc::clone(&challenge);
        let signatures = Arc::new(Mutex::new(Vec::new()));
        let paid_signatures = Arc::clone(&signatures);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let ledger = Arc::new(Mutex::new(fixture("gateway/metered.json")));
        let shared_ledger = Arc::clone(&ledger);
        let worker = thread::spawn(move || {
            let exact = fixture("gateway/exact.json");
            for stream in listener.incoming() {
                let mut stream = stream.expect("recording connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (target, headers, body) = read_http(&mut stream);
                let ledger = shared_ledger.lock().expect("ledger recording");
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
                        if data[..4]
                            == layerx_crypto::disclosure::call::ledger_selector(
                                "accountId(bytes32,bytes32)",
                            )
                        {
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
                            if data[..4]
                                == layerx_crypto::disclosure::call::ledger_selector(
                                    "nextSequence(bytes32,bytes32)",
                                )
                            {
                                let asset = hex(&data[36..68]);
                                let recipient = ledger["ledger"]["grants"]
                                    .as_array()
                                    .expect("ledger grants")
                                    .iter()
                                    .find_map(|record| {
                                        let canonical = record["canonical"].as_str()?;
                                        (canonical.get(192..256) == Some(asset.as_str()))
                                            .then(|| word(&json!(canonical[128..192].to_owned())))
                                    })
                                    .expect("canonical receiver account");
                                assert_eq!(&data[4..36], &recipient);
                                assert_ne!(recipient, word(&ledger["ledger"]["receiverDidKey"]));
                            }
                            ledger["endpoints"]["gateway"]["*"]
                                .as_array()
                                .expect("metered RPC observations")
                                .iter()
                                .find(|record| {
                                    record["method"] == request["method"]
                                        && record["params"] == request["params"]
                                })
                                .unwrap_or_else(|| panic!("unrecorded ledger view: {request}"))
                                ["result"]
                                .clone()
                        }
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
                    assert_eq!(
                        target,
                        exchange["exchange"][exchange_index]["request"]["target"]
                            .as_str()
                            .expect("recorded target")
                    );
                    if let Some((_, signature)) =
                        headers.iter().find(|(name, _)| name == "payment-signature")
                    {
                        let signature: Value = serde_json::from_slice(
                            &STANDARD.decode(signature).expect("signature base64"),
                        )
                        .expect("signature JSON");
                        let expected = &exchange["exchange"][exchange_index + 1]["request"]
                            ["headers"]["PAYMENT-SIGNATURE"];
                        assert_eq!(signature["payload"], expected["payload"]);
                        assert_eq!(signature["accepted"], expected["accepted"]);
                        paid_signatures
                            .lock()
                            .expect("signature capture")
                            .push(signature);
                        let reply = &exchange["exchange"][exchange_index + 1]["response"];
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
            ledger,
            stop,
            worker: Some(worker),
        }
    }
    pub(crate) fn endpoint(&self) -> String {
        format!("http://{}", self.address)
    }
}
impl Drop for Recording {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Err(error) = TcpStream::connect(&self.address) {
            eprintln!("recording wake connection failed: {error}");
        }
        if let Some(worker) = self.worker.take() {
            if let Err(error) = worker.join() {
                eprintln!("recording assertions failed: {error:?}");
            }
        }
    }
}

#[derive(Default)]
pub(crate) struct Traffic {
    pub(crate) preparations: usize,
    pub(crate) submitted: Vec<Prepared>,
    pub(crate) standing_grant: bool,
    pub(crate) grants: usize,
    pub(crate) submits: usize,
    pub(crate) approval_unavailable: bool,
}

pub(crate) struct RecordedNodePayer {
    pub(crate) resources: WebApprovalResources,
    pub(crate) node: SequencerEndpoint,
    pub(crate) receipt: layerx_sdk::rpc::RpcClient,
    pub(crate) traffic: Arc<Mutex<Traffic>>,
    pub(crate) nonce: u64,
    pub(crate) sender: [u8; 20],
    pub(crate) corrupt_hash: bool,
}
impl WebPayer for RecordedNodePayer {
    fn approval_resources(&self) -> Option<WebApprovalResources> {
        if self.traffic.lock().expect("traffic").approval_unavailable {
            None
        } else {
            Some(self.resources.clone())
        }
    }
    fn standing_grant(&mut self, terms: &GrantTerms) -> Result<Option<[u8; 32]>, WebPayerError> {
        if !self.traffic.lock().expect("traffic").standing_grant {
            return Ok(None);
        }
        let ledger = fixture("gateway/metered.json");
        let grant = ledger["ledger"]["grants"]
            .as_array()
            .expect("recorded grants")
            .iter()
            .find(|grant| {
                let canonical = grant["canonical"].as_str().expect("grant bytes");
                canonical.get(192..256) == Some(hex(&terms.asset).as_str())
            })
            .ok_or(WebPayerError::Refused)?;
        Ok(Some(word(&grant["grantId"])))
    }
    fn grant(&mut self, terms: &GrantTerms) -> Result<PrecompileCall, WebPayerError> {
        use ed25519_dalek::{Signer as _, SigningKey};

        let mut traffic = self.traffic.lock().expect("traffic");
        traffic.grants += 1;
        if traffic.standing_grant {
            panic!("standing-grant approval must never sign or issue a new grant");
        }
        drop(traffic);
        let signer = SigningKey::from_bytes(&[0x53; 32]);
        let mut grant = terms.unsigned_grant(signer.verifying_key().to_bytes());
        grant.id = grant.signing_digest().map_err(|_| WebPayerError::Refused)?;
        grant.signature = signer.sign(&grant.id).to_bytes();
        terms.prepare_grant(&grant)
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
        self.traffic.lock().expect("traffic").submits += 1;
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
        assert_eq!(
            prepared.disclosure.actor.as_str(),
            hex(&[0xab; 20]),
            "held disclosure actor must retain the sender address"
        );
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

pub(crate) fn resources(root: &std::path::Path) -> WebApprovalResources {
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
pub(crate) fn key() -> [u8; 32] {
    word(&fixture("gateway/buyer.json")["exact"]["USDC"]["idempotencyKey"])
}
pub(crate) fn tenant() -> TenantId {
    TenantId::new("tenant-a").expect("tenant")
}
