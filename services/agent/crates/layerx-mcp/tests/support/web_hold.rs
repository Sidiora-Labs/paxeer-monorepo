use crate::option_assertion;
use crate::result_assertion;

pub use crate::test_clock;

pub use std::collections::BTreeSet;
pub use std::fs;
pub use std::io::{BufRead, BufReader, Read, Write};
pub use std::net::{TcpListener, TcpStream};
pub use std::os::unix::fs::PermissionsExt;
pub use std::path::PathBuf;
pub use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
pub use std::sync::{Arc, Mutex};
pub use std::thread::{self, JoinHandle};
pub use std::time::Duration;

pub use base64::{engine::general_purpose::STANDARD, Engine as _};
pub use layerx_agent_api::prepare::Prepared;
pub use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, DecisionKey, DecisionRequest,
};
pub use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
pub use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
pub use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
pub use layerx_agentd::policy::approval::{ApprovalRegistry, ApprovalState, ApproverId};
pub use layerx_agentd::prepare::evm::{self, PrecompileCall};
pub use layerx_agentd::prepare::PreparationLifecycle;
pub use layerx_agentd::session::{
    open, OpenRequest, SessionCredential, SessionId, SessionRegistry,
};
pub use layerx_agentd::session_control::SessionControl;
pub use layerx_agentd::store::{Store, TenantId};
pub use layerx_client::sequencer::SequencerEndpoint;
pub use layerx_crypto::evm_transaction::decode_unsigned;
pub use layerx_mcp::tools::web::{
    ExactPayment, GrantTerms, WebApprovalResources, WebPayer, WebPayerError,
};
pub use layerx_proof::sequencer_logs::SequencerLog;
pub use layerx_types::ids::Did;
pub use layerx_types::verify::VerificationLevel;
pub use serde_json::{json, Value};
pub use sha2::{Digest as _, Sha256};

pub const HEIGHT: u64 = 20;
pub static NEXT: AtomicU64 = AtomicU64::new(0);

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn fixture(name: &str) -> Value {
    let path = std::path::Path::new(crate::manifest_dir())
        .join("../../../../services/interop/crates/x-websearch/tests/fixtures")
        .join(name);
    result_assertion::required(
        serde_json::from_slice(&result_assertion::required(
            fs::read(path),
            "committed producer recording",
        )),
        "recording JSON",
    )
}
#[must_use]
pub fn hex(bytes: &[u8]) -> String {
    layerx_programs::hex::encode(bytes)
}
/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn unhex(value: &str) -> Vec<u8> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    (0..value.len())
        .step_by(2)
        .map(|i| result_assertion::required(u8::from_str_radix(&value[i..i + 2], 16), "hex"))
        .collect()
}
/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn word(value: &Value) -> [u8; 32] {
    result_assertion::required(
        unhex(option_assertion::required(value.as_str(), "word")).try_into(),
        "word width",
    )
}
pub struct Directory(pub PathBuf);
impl Directory {
    /// # Panics
    /// Panics if the producer fixture, protocol assertion or required local resource is invalid.
    #[must_use]
    pub fn new() -> Self {
        let root =
            result_assertion::required(fs::canonicalize(std::env::temp_dir()), "temporary root")
                .join(format!(
                    "repair-22-33-1551-{}-{}",
                    std::process::id(),
                    NEXT.fetch_add(1, Ordering::Relaxed)
                ));
        result_assertion::required(fs::create_dir(&root), "directory");
        result_assertion::required(
            fs::set_permissions(&root, fs::Permissions::from_mode(0o700)),
            "directory mode",
        );
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        result_assertion::required(fs::remove_dir_all(&self.0), "remove state");
    }
}

// Identity provisioning is outside the repaired server; all sessions and capabilities below
// are minted, persisted and restored by the daemon's actual registration/open paths.
pub struct IdentityLedger(CoreIdentity);
impl IdentityResolver for IdentityLedger {
    fn resolve(&mut self, _: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

pub struct Enrolled {
    pub directory: Directory,
    pub control: SessionControl,
    pub credential: SessionCredential,
}
impl Enrolled {
    /// # Panics
    /// Panics if the producer fixture, protocol assertion or required local resource is invalid.
    #[must_use]
    pub fn new(scopes: &[&str], agent: &str) -> Self {
        let directory = Directory::new();
        let tenant = result_assertion::required(TenantId::new("tenant-a"), "tenant");
        let mut store = result_assertion::required(Store::open(directory.0.join("store")), "store");
        let capability = result_assertion::required(
            Capability::new(
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
            ),
            "capability",
        );
        result_assertion::required(capability.persist(&mut store), "persist capability");
        let mut ledger = IdentityLedger(CoreIdentity {
            canonical_bytes: agent.as_bytes().to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(capability.id.0)],
        });
        let identity = result_assertion::required(
            register(
                &mut store,
                tenant.clone(),
                result_assertion::required(Did::new(agent.as_bytes()), "DID"),
                &mut ledger,
            ),
            "register",
        );
        let mut sessions = SessionRegistry::default();
        let credential = result_assertion::required(
            open(
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
            ),
            "open authenticated session",
        )
        .credential();
        let control = SessionControl::new(
            Arc::new(Mutex::new(store)),
            sessions,
            Arc::new(PreparationLifecycle::default()),
            Arc::new(result_assertion::required(
                BudgetLimiter::new(vec![limit()]),
                "limiter",
            )),
        );
        Self {
            directory,
            control,
            credential,
        }
    }
}
#[must_use]
pub fn limit() -> LimitConfig {
    LimitConfig {
        id: LimitId([10; 16]),
        name: "mcp".to_owned(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 1000,
        consumed: 0,
    }
}
/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn read_http(stream: &mut TcpStream) -> (String, Vec<(String, String)>, Vec<u8>) {
    result_assertion::required(
        stream.set_read_timeout(Some(Duration::from_secs(5))),
        "read deadline",
    );
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    result_assertion::required(reader.read_line(&mut line), "request line");
    let target =
        option_assertion::required(line.split_whitespace().nth(1), "request target").to_owned();
    let mut headers = Vec::new();
    loop {
        line.clear();
        assert_ne!(
            result_assertion::required(reader.read_line(&mut line), "header"),
            0
        );
        if line == "\r\n" {
            break;
        }
        let (name, value) = option_assertion::required(line.split_once(':'), "header delimiter");
        headers.push((name.to_ascii_lowercase(), value.trim().to_owned()));
    }
    let length = headers
        .iter()
        .find(|(name, _)| name == "content-length")
        .map_or(0, |(_, value)| {
            result_assertion::required(value.parse(), "body length")
        });
    let mut body = vec![0; length];
    result_assertion::required(reader.read_exact(&mut body), "request body");
    (target, headers, body)
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
pub fn respond(stream: &mut TcpStream, status: u16, headers: &[(&str, String)], body: &[u8]) {
    result_assertion::required(
        write!(
            stream,
            "HTTP/1.1 {status} Recorded\r\nContent-Length: {}\r\nConnection: close\r\n",
            body.len()
        ),
        "response line",
    );
    for (name, value) in headers {
        result_assertion::required(write!(stream, "{name}: {value}\r\n"), "response header");
    }
    result_assertion::required(stream.write_all(b"\r\n"), "header end");
    result_assertion::required(stream.write_all(body), "response body");
}

/// Committed seller, ledger and signed receipt transcripts are the web component's peers.
pub struct Recording {
    pub address: String,
    pub challenge: Arc<Mutex<Value>>,
    pub signatures: Arc<Mutex<Vec<Value>>>,
    pub ledger: Arc<Mutex<Value>>,
    pub stop: Arc<AtomicBool>,
    pub worker: Option<JoinHandle<()>>,
}
impl Recording {
    /// # Panics
    /// Panics if the producer fixture, protocol assertion or required local resource is invalid.
    #[must_use]
    pub fn start() -> Self {
        Self::start_exchange(6)
    }

    /// # Panics
    /// Panics if the producer fixture, protocol assertion or required local resource is invalid.
    #[must_use]
    pub fn start_search() -> Self {
        Self::start_exchange(0)
    }

    fn start_exchange(exchange_index: usize) -> Self {
        let listener =
            result_assertion::required(TcpListener::bind("127.0.0.1:0"), "recording listener");
        let address =
            result_assertion::required(listener.local_addr(), "recording address").to_string();
        let exchange = fixture("client-exchange.json");
        let mut required =
            exchange["exchange"][exchange_index]["response"]["headers"]["PAYMENT-REQUIRED"].clone();
        required["resource"]["url"] = json!(format!(
            "http://{address}{}",
            option_assertion::required(
                exchange["exchange"][exchange_index]["request"]["target"].as_str(),
                "target"
            )
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
                let mut stream = result_assertion::required(stream, "recording connection");
                if stopped.load(Ordering::Acquire) {
                    break;
                }
                let (target, headers, body) = read_http(&mut stream);
                let ledger = result_assertion::required(shared_ledger.lock(), "ledger recording");
                if target.starts_with("/v1/programs/") {
                    let body = json!({"program":"cc".repeat(32),"accounts":[],"freshness":{"observed_sequence":HEIGHT,"valid_through":400}});
                    respond(&mut stream, 200, &[], body.to_string().as_bytes());
                } else if target == "/" {
                    let request: Value =
                        result_assertion::required(serde_json::from_slice(&body), "node request");
                    let result = recorded_node_result(&request, &ledger, &exact);
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
                        option_assertion::required(
                            exchange["exchange"][exchange_index]["request"]["target"].as_str(),
                            "recorded target"
                        )
                    );
                    if let Some((_, signature)) =
                        headers.iter().find(|(name, _)| name == "payment-signature")
                    {
                        answer_paid_fetch(
                            &mut stream,
                            signature,
                            &exchange,
                            exchange_index,
                            &paid_signatures,
                        );
                    } else {
                        let required =
                            result_assertion::required(shared_challenge.lock(), "challenge");
                        let header = STANDARD.encode(result_assertion::required(
                            serde_json::to_vec(&*required),
                            "challenge bytes",
                        ));
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
    #[must_use]
    pub fn endpoint(&self) -> String {
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
pub struct Traffic {
    pub preparations: usize,
    pub submitted: Vec<Prepared>,
    pub standing_grant: bool,
    pub grants: usize,
    pub submits: usize,
    pub approval_unavailable: bool,
}

pub struct RecordedNodePayer {
    pub resources: WebApprovalResources,
    pub node: SequencerEndpoint,
    pub receipt: layerx_sdk::rpc::RpcClient,
    pub traffic: Arc<Mutex<Traffic>>,
    pub nonce: u64,
    pub sender: [u8; 20],
    pub corrupt_hash: bool,
}
impl WebPayer for RecordedNodePayer {
    fn approval_resources(&self) -> Option<WebApprovalResources> {
        if result_assertion::required(self.traffic.lock(), "traffic").approval_unavailable {
            None
        } else {
            Some(self.resources.clone())
        }
    }
    fn standing_grant(&mut self, terms: &GrantTerms) -> Result<Option<[u8; 32]>, WebPayerError> {
        if !result_assertion::required(self.traffic.lock(), "traffic").standing_grant {
            return Ok(None);
        }
        let ledger = fixture("gateway/metered.json");
        let grant =
            option_assertion::required(ledger["ledger"]["grants"].as_array(), "recorded grants")
                .iter()
                .find(|grant| {
                    let canonical =
                        option_assertion::required(grant["canonical"].as_str(), "grant bytes");
                    canonical.get(192..256) == Some(hex(&terms.asset).as_str())
                })
                .ok_or(WebPayerError::Refused)?;
        Ok(Some(word(&grant["grantId"])))
    }
    fn grant(&mut self, terms: &GrantTerms) -> Result<PrecompileCall, WebPayerError> {
        use ed25519_dalek::{Signer as _, SigningKey};

        let mut traffic = result_assertion::required(self.traffic.lock(), "traffic");
        traffic.grants += 1;
        assert!(
            !traffic.standing_grant,
            "standing-grant approval must never sign or issue a new grant"
        );
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
        result_assertion::required(self.traffic.lock(), "traffic").preparations += 1;
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
                maximum_payload_bytes: 65_536,
            },
            evm::PrepareRequest {
                from: self.sender,
                call: call.clone(),
                expected_nonce: Some(self.nonce),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 65_536,
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
        result_assertion::required(self.traffic.lock(), "traffic").submits += 1;
        panic!("an approved transaction must never use the re-preparing submit path")
    }
    fn submit_transaction(&mut self, prepared: &Prepared) -> Result<ExactPayment, WebPayerError> {
        let call = result_assertion::required(
            decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes()),
            "consumer decodes actual held transaction",
        );
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
        result_assertion::required(self.traffic.lock(), "traffic")
            .submitted
            .push(prepared.clone());
        let buyer = fixture("gateway/buyer.json");
        let hash = word(&buyer["exact"]["USDC"]["txHash"]);
        let evidence = self
            .receipt
            .sequencer_receipt(hash)
            .map_err(|_| WebPayerError::Unavailable)?;
        let logs = option_assertion::required(evidence["logs"].as_array(), "recorded logs")
            .iter()
            .map(|log| {
                SequencerLog::new(
                    result_assertion::required(
                        unhex(option_assertion::required(
                            log["address"].as_str(),
                            "log address",
                        ))
                        .try_into(),
                        "address width",
                    ),
                    option_assertion::required(log["topics"].as_array(), "log topics")
                        .iter()
                        .map(word)
                        .collect(),
                    unhex(option_assertion::required(log["data"].as_str(), "log data")),
                    result_assertion::required(
                        u64::from_str_radix(
                            option_assertion::required(log["logIndex"].as_str(), "log index")
                                .trim_start_matches("0x"),
                            16,
                        ),
                        "index quantity",
                    ),
                )
            })
            .collect();
        Ok(ExactPayment {
            tx_hash: hash,
            receipt: unhex(option_assertion::required(
                evidence["receipt"].as_str(),
                "signed receipt",
            )),
            logs,
        })
    }
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn resources(root: &std::path::Path) -> WebApprovalResources {
    let store = Arc::new(Mutex::new(result_assertion::required(
        Store::open(root),
        "durable approval store",
    )));
    WebApprovalResources {
        expiry: Arc::new(ApprovalExpiry::from_shared_store(Arc::clone(&store))),
        store,
        limiter: Arc::new(result_assertion::required(
            BudgetLimiter::new(vec![LimitConfig {
                id: LimitId([12; 16]),
                name: "web spend".to_owned(),
                scope: LimitScope::Tenant([1; 32]),
                ceiling: 1_000_000,
                consumed: 0,
            }]),
            "approval budgets",
        )),
        limits: vec![LimitId([12; 16])],
    }
}
#[must_use]
pub fn key() -> [u8; 32] {
    word(&fixture("gateway/buyer.json")["exact"]["USDC"]["idempotencyKey"])
}
/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn tenant() -> TenantId {
    result_assertion::required(TenantId::new("tenant-a"), "tenant")
}

fn recorded_node_result(request: &Value, ledger: &Value, exact: &Value) -> Value {
    if request["method"] == "eth_call" {
        let data = unhex(option_assertion::required(
            request["params"][0]["data"].as_str(),
            "ABI bytes",
        ));
        assert_eq!(
            request["params"][0]["to"],
            "0x000000000000000000000000000000000000101a"
        );
        assert_eq!(request["params"][1], "pending");
        if data[..4]
            == layerx_crypto::disclosure::call::ledger_selector("accountId(bytes32,bytes32)")
        {
            let asset = hex(&data[36..68]);
            let record = option_assertion::required(
                option_assertion::required(ledger["ledger"]["grants"].as_array(), "ledger grants")
                    .iter()
                    .find(|record| {
                        option_assertion::required(record["canonical"].as_str(), "grant bytes")
                            .get(192..256)
                            == Some(asset.as_str())
                    }),
                "recorded account for asset",
            );
            json!(format!(
                "0x{}",
                &option_assertion::required(record["canonical"].as_str(), "grant bytes")[64..128]
            ))
        } else {
            if data[..4]
                == layerx_crypto::disclosure::call::ledger_selector("nextSequence(bytes32,bytes32)")
            {
                let asset = hex(&data[36..68]);
                let recipient = ledger["ledger"]["grants"]
                    .as_array()
                    .unwrap_or_else(|| panic!("ledger grants"))
                    .iter()
                    .find_map(|record| {
                        let canonical = record["canonical"].as_str()?;
                        (canonical.get(192..256) == Some(asset.as_str()))
                            .then(|| word(&json!(canonical[128..192].to_owned())))
                    })
                    .unwrap_or_else(|| panic!("canonical receiver account"));
                assert_eq!(&data[4..36], &recipient);
                assert_ne!(recipient, word(&ledger["ledger"]["receiverDidKey"]));
            }
            option_assertion::required(
                ledger["endpoints"]["gateway"]["*"].as_array(),
                "metered RPC observations",
            )
            .iter()
            .find(|record| {
                record["method"] == request["method"] && record["params"] == request["params"]
            })
            .unwrap_or_else(|| panic!("unrecorded ledger view: {request}"))["result"]
                .clone()
        }
    } else {
        assert_eq!(request["method"], "pax_sequencerReceipt");
        option_assertion::required(
            option_assertion::required(
                exact["endpoints"]["gateway"]["*"].as_array(),
                "receipt transcript",
            )
            .iter()
            .find(|record| {
                record["method"] == request["method"] && record["params"] == request["params"]
            }),
            "recorded receipt request",
        )["result"]
            .clone()
    }
}

fn answer_paid_fetch(
    stream: &mut TcpStream,
    signature: &str,
    exchange: &Value,
    exchange_index: usize,
    signatures: &Mutex<Vec<Value>>,
) {
    let signature: Value = result_assertion::required(
        serde_json::from_slice(&result_assertion::required(
            STANDARD.decode(signature),
            "signature base64",
        )),
        "signature JSON",
    );
    let expected =
        &exchange["exchange"][exchange_index + 1]["request"]["headers"]["PAYMENT-SIGNATURE"];
    assert_eq!(signature["payload"], expected["payload"]);
    assert_eq!(signature["accepted"], expected["accepted"]);
    result_assertion::required(signatures.lock(), "signature capture").push(signature);
    let reply = &exchange["exchange"][exchange_index + 1]["response"];
    let response = STANDARD.encode(result_assertion::required(
        serde_json::to_vec(&reply["headers"]["PAYMENT-RESPONSE"]),
        "settlement bytes",
    ));
    respond(
        stream,
        200,
        &[("PAYMENT-RESPONSE", response)],
        &result_assertion::required(serde_json::to_vec(&reply["body"]), "content bytes"),
    );
}

impl Default for Directory {
    fn default() -> Self {
        Self::new()
    }
}
