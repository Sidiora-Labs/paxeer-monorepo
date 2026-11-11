use layerx_agentd_host::payer;

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::enrolment::{
    self, BindingMode, BindingPublisher, DaemonSurface, EnrolmentRequest, WebDeclaration,
};
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_agentd::session::{SessionId, SessionRegistry};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ExecutedReceipt, ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_mcp::binding::Binding;
use layerx_mcp::tools::web::{GrantTerms, WebPayerError};
use layerx_proof::sequencer_logs::decode_summary;
use layerx_types::ids::Did;
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

/// Returns a successful test operation.
///
/// # Panics
///
/// Panics if the test operation returns an error.
#[track_caller]
pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn unhex(value: &Value) -> Vec<u8> {
    let text = {
        let Some(expected_value) = value.as_str() else {
            panic!("recorded hex string")
        };
        expected_value
    };
    let digits = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    must(unhex(value).try_into())
}

fn hex(bytes: &[u8]) -> String {
    let encoded: String = bytes
        .iter()
        .flat_map(|byte| {
            let digits = b"0123456789abcdef";
            [
                char::from(digits[usize::from(byte >> 4)]),
                char::from(digits[usize::from(byte & 0xf)]),
            ]
        })
        .collect();
    format!("0x{encoded}")
}

fn exact_case() -> Value {
    let recordings: Value = must(serde_json::from_str(include_str!(
        "../../src/repair_22_31_363_receipts.json"
    )));
    assert_eq!(
        recordings["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    recordings["success"].clone()
}

fn pin(case: &Value) -> ReceiptKeyEntry {
    let pin = &case["pinnedKey"];
    ReceiptKeyEntry {
        key_id: fixed(&pin["keyId"]),
        public_key: fixed(&pin["publicKey"]),
        first_height: {
            let Some(expected_value) = pin["firstHeight"].as_u64() else {
                panic!("first height")
            };
            expected_value
        },
        last_height: {
            let Some(expected_value) = pin["lastHeight"].as_u64() else {
                panic!("last height")
            };
            expected_value
        },
    }
}

fn exact_call(case: &Value) -> PrecompileCall {
    let raw = unhex(&case["rawTransaction"]);
    let (transaction, signature) = must(decode_signed(&raw));
    assert_eq!(
        must(recover_sender(&transaction, &signature)),
        fixed::<20>(&case["receiptJson"]["from"])
    );
    assert_eq!(
        transaction_hash(&raw),
        fixed(&case["receiptJson"]["txHash"])
    );
    assert_eq!(transaction.to, PrecompileTarget::Ledger.address());
    assert_eq!(transaction.data.len(), 4 + 6 * 32);
    assert_eq!(&transaction.data[..4], &[0x78, 0xea, 0x11, 0xdb]);
    let call = PrecompileCall {
        target: PrecompileTarget::Ledger,
        selector: must(transaction.data[..4].try_into()),
        arguments: transaction.data[4..]
            .chunks_exact(32)
            .map(|word| AbiArgument::Word(must(word.try_into())))
            .collect(),
    };
    assert_eq!(must(call.encode()), transaction.data);
    call
}

// Only the peer is a transcript server. The client, signature/summary/log codecs,
// enrolment store, binding loader and payment receipt consumer are production code.
struct NodeTranscript {
    url: String,
    stop: Arc<AtomicBool>,
    requests: Arc<Mutex<Vec<Value>>>,
    worker: Option<JoinHandle<()>>,
}

impl NodeTranscript {
    fn open(cases: &[Value], sequence: Option<(Vec<u8>, Value)>) -> Self {
        let cases = cases.to_vec();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&requests);
        let worker = std::thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                must(stream.set_write_timeout(Some(Duration::from_secs(5))));
                let (mut reader, request) = transcript_request(stream);
                let method = {
                    let Some(expected_value) = request["method"].as_str() else {
                        panic!("RPC method")
                    };
                    expected_value
                };
                let result = match method {
                    "px_getReceiptKeys" => {
                        json!({"keys": cases.iter().map(|case| case["pinnedKey"].clone()).collect::<Vec<_>>()})
                    }
                    "pax_sequencerReceipt" | "eth_getTransactionReceipt" => {
                        let case = {
                            let Some(expected_value) = cases.iter().find(|case| {
                                request["params"] == json!([case["receiptJson"]["txHash"]])
                            }) else {
                                panic!("recorded transaction")
                            };
                            expected_value
                        };
                        if request["method"] == "pax_sequencerReceipt" {
                            case["receiptJson"].clone()
                        } else {
                            Value::Null
                        }
                    }
                    "eth_call" => {
                        let (data, result) = {
                            let Some(expected_value) = sequence.as_ref() else {
                                panic!("recorded receiver sequence")
                            };
                            expected_value
                        };
                        assert_eq!(
                            request["params"],
                            json!([{"to":hex(&PrecompileTarget::Ledger.address()), "data":hex(data)}, "pending"])
                        );
                        result.clone()
                    }
                    method => panic!("unexpected RPC method {method}"),
                };
                let answer =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result": result}).to_string();
                must(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
                must(reader.get_mut().flush());
                must(recorded.lock()).push(request);
            }
        });
        Self {
            url,
            stop,
            requests,
            worker: Some(worker),
        }
    }

    fn endpoint(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
    }

    fn execution(&self, case: &Value, keys: &[ReceiptKeyEntry]) -> ExecutedReceipt {
        let Lookup::Verified(receipt) = must(lookup_authenticated(
            &self.endpoint(),
            fixed(&case["receiptJson"]["txHash"]),
            LookupContext { receipt_keys: keys },
        )) else {
            panic!("authenticated node receipt");
        };
        assert_eq!(
            receipt.canonical_bytes(),
            unhex(&case["receiptJson"]["receipt"])
        );
        assert_eq!(
            must(layerx_wire::receipt::encode(receipt.receipt())),
            receipt.canonical_bytes()
        );
        receipt.execution().clone()
    }
}

impl Drop for NodeTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

/// Asserts that payer receipt verification binds separate payment and fee denominations.
///
/// # Panics
/// Panics if fixture setup fails or any payer invariant is violated.
pub fn payer_equal_denom_fixture() {
    let case = exact_case();
    let node = NodeTranscript::open(std::slice::from_ref(&case), None);
    let execution = node.execution(&case, &[pin(&case)]);
    assert_eq!(execution.payment_denom, "uhpx");
    assert_eq!(execution.fee_denom, "usid");
    assert_ne!(execution.payment_denom, execution.fee_denom);
    let call = exact_call(&case);
    let payment = must(payer::receipt(
        execution.clone(),
        execution.tx_hash,
        execution.from,
        &call,
        &[pin(&case)],
    ));
    assert_eq!(payment.receipt, unhex(&case["signedReceipt"]));
    assert_eq!(payment.logs.len(), 1);
    assert_eq!(payment.logs[0].data(), execution.logs[0].data);
    let decoded = must(layerx_wire::receipt::decode(&payment.receipt));
    assert_eq!(
        {
            let Some(expected_value) = decoded.protocol() else {
                panic!("protocol receipt")
            };
            expected_value
        }
        .asset(),
        <[u8; 32]>::from(Sha256::digest(b"uhpx"))
    );
    assert_eq!(must(decode_summary(&decoded)).fee_denom(), "usid");
    for field in ["payment_denom", "fee_denom"] {
        let mut altered = execution.clone();
        match field {
            "payment_denom" => altered.payment_denom.clone_from(&execution.fee_denom),
            "fee_denom" => altered.fee_denom.clone_from(&execution.payment_denom),
            _ => unreachable!(),
        }
        assert_eq!(
            payer::receipt(
                altered,
                execution.tx_hash,
                execution.from,
                &call,
                &[pin(&case)]
            ),
            Err(WebPayerError::Refused),
            "{field} must remain independently bound"
        );
    }
}

// The committed sidecar transcript preserves the signed draw bytes and raw logs.
// Its older JSON view omits redundant metadata; recover that metadata with the
// production receipt and summary codecs without changing or signing the receipt.
fn draw_case() -> (Value, GrantTerms, [u8; 32], Vec<u8>, Value) {
    let recorded: Value = must(serde_json::from_str(include_str!(
        "../../../../../interop/crates/x-websearch/tests/fixtures/gateway/metered.json"
    )));
    let authority: Value = must(serde_json::from_str(include_str!(
        "../../../../../interop/crates/x-websearch/src/repair_22_31_169_ledger.json"
    )));
    let exchanges = {
        let Some(expected_value) = recorded["endpoints"]["gateway"]["*"].as_array() else {
            panic!("recorded RPC exchanges")
        };
        expected_value
    };
    let mut view = exchanges[17]["result"].clone();
    assert_eq!(exchanges[17]["method"], "pax_sequencerReceipt");
    let receipt = must(layerx_wire::receipt::decode(&unhex(&view["receipt"])));
    let protocol = {
        let Some(expected_value) = receipt.protocol() else {
            panic!("recorded draw protocol")
        };
        expected_value
    };
    let summary = must(decode_summary(&receipt));
    assert_eq!(protocol.asset(), <[u8; 32]>::from(Sha256::digest(b"upax")));
    view["index"] = json!(format!("{:#x}", summary.tx_index()));
    view["timestamp"] = json!(format!("{:#x}", protocol.timestamp()));
    view["returnData"] = json!("0x");
    view["gasUsed"] = json!(format!("{:#x}", summary.gas_used()));
    view["cumulativeGasUsed"] = json!(format!("{:#x}", summary.cumulative_gas_used()));
    view["effectiveGasPrice"] = json!(format!("{:#x}", summary.effective_gas_price()));
    view["feeDenom"] = json!(summary.fee_denom());
    view["paymentDenom"] = json!("upax");
    view["feeAmount"] = json!(format!("{:#x}", protocol.fee_charged()));
    view["sponsor"] = json!(hex(&summary.sponsor()));
    view["from"] = json!(hex(&protocol.from()[12..]));
    view["to"] = json!(hex(&protocol.to()[12..]));
    view["contractAddress"] = json!(hex(&summary.contract_address()));
    view["vmError"] = json!("");
    view["signingKey"] = authority["receipt_key"]["public_key"].clone();
    view["signingKey"] = json!(hex(&unhex(&view["signingKey"])));
    let key = &authority["receipt_key"];
    // Height zero predates the current key-index contract. Narrow the public
    // conformance pin to committed heights; never widen its authority range.
    assert_eq!(key["first_height"], 0);
    assert!(
        {
            let Some(expected_value) = key["last_height"].as_u64() else {
                panic!("last height")
            };
            expected_value
        } >= 9
    );
    let pin = json!({"keyId": hex(&unhex(&key["key_id"])), "publicKey": hex(&unhex(&key["public_key"])), "firstHeight": 1, "lastHeight": key["last_height"]});
    let encoded = {
        let Some(expected_value) = exchanges[8]["params"][0].as_str() else {
            panic!("recorded signed draw")
        };
        expected_value
    };
    let offset = {
        let Some(expected_value) = encoded.find("2e89490f") else {
            panic!("draw selector")
        };
        expected_value
    };
    let data = unhex(&json!(&encoded[offset..offset + (4 + 7 * 32) * 2]));
    let word = |offset: usize| -> [u8; 32] { must(data[offset..offset + 32].try_into()) };
    let grant = word(4);
    let terms = GrantTerms {
        payer_did: "did:layerx:web-agent".to_owned(),
        payer_account: fixed(&view["logs"][0]["topics"][2]),
        recipient: word(36),
        receiver_did_key: fixed(&recorded["ledger"]["receiverDidKey"]),
        asset: must(unhex(&view["logs"][0]["data"])[..32].try_into()),
        expires_at_ms: 1_900_000_000_000,
        amount: u128::from_be_bytes(must(data[84..100].try_into())),
        purpose_hash: word(164),
        idempotency_key: word(100),
    };
    assert_eq!(exchanges[14]["method"], "eth_call");
    (
        json!({"receiptJson":view, "pinnedKey":pin}),
        terms,
        grant,
        data,
        exchanges[14]["result"].clone(),
    )
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let root = must(fs::canonicalize(std::env::temp_dir())).join(format!(
            "payer-audit-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&root));
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

struct IdentityRecords(BTreeMap<Did, CoreIdentity>);
impl IdentityResolver for IdentityRecords {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        Ok(self.0.get(did).cloned())
    }
}

/// Asserts that daemon bindings retain the authenticated payer identity and receipt keys.
///
/// # Panics
/// Panics if fixture setup fails or any payer invariant is violated.
pub fn payer_binding_bypass() {
    let exact = exact_case();
    let (draw, terms, grant, expected_draw, sequence) = draw_case();
    let node = NodeTranscript::open(
        &[exact.clone(), draw.clone()],
        Some((must(terms.next_sequence_call().encode()), sequence)),
    );
    let keys = must(node.endpoint().receipt_keys());
    assert_eq!(keys, vec![pin(&exact), pin(&draw)]);
    let directory = Directory::new();
    let tenant = must(TenantId::new("payer-audit"));
    let agent = must(Did::new(terms.payer_did.as_bytes()));
    let capability = CapabilityId([9; 32]);
    let mut store = must(Store::open(directory.0.join("store")));
    persist_capability(&mut store, capability, &tenant, &terms);
    let identity = register_identity(&mut store, &tenant, agent, capability);
    let publisher = audit_publisher(&directory, &node, &draw, &keys);
    let mut sessions = SessionRegistry::default();
    let request = audit_request(capability);
    let enrolled = must(enrolment::enrol(
        &mut store,
        &mut sessions,
        &identity,
        request.clone(),
        &publisher,
    ));
    let bytes = must(fs::read(&enrolled.binding));
    let document: Value = must(serde_json::from_slice(&bytes));
    let binding = must(Binding::open(&enrolled.binding));
    let daemon_credential = must(binding.daemon_credential());
    assert!(binding.matches_session(&daemon_credential));
    assert_eq!(daemon_credential.tenant(), &tenant);
    assert_eq!(daemon_credential.session_id(), enrolled.session_id);
    assert_eq!(binding.core_height(), 50);
    assert_eq!(document["web"]["network"], "eip155:125");
    assert_eq!(
        fixed::<32>(&document["web"]["sequencer_id"]),
        keys[1].key_id
    );
    let enrolled_keys = published_receipt_keys(&document, &keys);
    for (case, call) in [
        (&exact, exact_call(&exact)),
        (&draw, {
            let sequence_call = terms.next_sequence_call();
            let result = must(node.endpoint().call_pending(
                &sequence_call.target.address(),
                &must(sequence_call.encode()),
            ));
            let prepared = must(terms.draw_from_sequence_result(grant, &result));
            assert_eq!(must(prepared.encode()), expected_draw);
            prepared
        }),
    ] {
        assert_payment(&node, case, &call, &enrolled_keys);
    }
    let mut retired = document.clone();
    retired["web"]["network"] = json!("layerx:125");
    assert!(Binding::parse(&retired.to_string()).is_err());
    let mut ungoverned = document.clone();
    ungoverned["web"]["receipt_key_index"] = json!([]);
    assert!(Binding::parse(&ungoverned.to_string()).is_err());
    drop(binding);
    drop(sessions);
    drop(store);
    let store = must(Store::open(directory.0.join("store")));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant));
    must(publisher.validate_existing(&sessions, &identity, &request));
    let reopened = must(Binding::open(&enrolled.binding));
    assert_eq!(
        must(reopened.daemon_credential()).session_id(),
        daemon_credential.session_id()
    );
    assert_eq!(must(fs::read(&enrolled.binding)), bytes);
    let requests = must(node.requests.lock());
    assert_eq!(
        requests
            .iter()
            .filter(|request| request["method"] == "px_getReceiptKeys")
            .count(),
        1
    );
    assert_eq!(
        requests
            .iter()
            .filter(|request| request["method"] == "pax_sequencerReceipt")
            .count(),
        2
    );
}

fn persist_capability(
    store: &mut Store,
    capability: CapabilityId,
    tenant: &TenantId,
    terms: &GrantTerms,
) {
    must(
        must(Capability::new(
            capability,
            tenant.clone(),
            CapabilityDimensions {
                activity_types: BTreeSet::from([7]),
                counterparties: BTreeSet::from([terms.recipient]),
                assets: BTreeSet::from([terms.asset]),
                amount_ceiling: 1_000_000,
                rate_ceiling: RateCeiling {
                    maximum_uses: 10,
                    window_sequences: 10,
                },
                purposes: BTreeSet::from(["service-payment".to_owned()]),
                expiry_sequence: 400,
            },
        ))
        .persist(store),
    );
}

fn register_identity(
    store: &mut Store,
    tenant: &TenantId,
    agent: Did,
    capability: CapabilityId,
) -> identity::IdentityRecord {
    let mut identities = IdentityRecords(BTreeMap::from([(
        agent.clone(),
        CoreIdentity {
            canonical_bytes: b"recorded-web-identity".to_vec(),
            head_sequence: 10,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: vec![ProtocolAuthority::CapabilityGrant(capability.0)],
        },
    )]));
    must(identity::register(
        store,
        tenant.clone(),
        agent,
        &mut identities,
    ))
}

fn audit_publisher(
    directory: &Directory,
    node: &NodeTranscript,
    draw: &Value,
    keys: &[ReceiptKeyEntry],
) -> BindingPublisher {
    let web = must(WebDeclaration::new(
        node.url.clone(),
        "eip155:125".to_owned(),
        fixed(&draw["receiptJson"]["from"]),
        [0x31; 20],
        keys.to_vec(),
        1,
        5_000,
        3,
        1_000_000,
    ));
    must(BindingPublisher::new(
        directory.0.join("binding"),
        directory.0.join("store"),
        directory.0.join("audit"),
        must(DaemonSurface::new(
            "127.0.0.1:4001",
            "b".repeat(48),
            [8; 32],
        )),
        LimitConfig {
            id: LimitId([1; 16]),
            name: "mcp".to_owned(),
            scope: LimitScope::Tenant([1; 32]),
            ceiling: 1_000_000,
            consumed: 0,
        },
        Duration::from_secs(5),
        BindingMode::Full,
    ))
    .with_web(web)
}

fn audit_request(capability: CapabilityId) -> EnrolmentRequest {
    EnrolmentRequest {
        session_id: SessionId([7; 32]),
        capability_id: capability,
        permitted_activity_types: BTreeSet::from([7]),
        scopes: BTreeSet::from([
            "read:balance".to_owned(),
            "web.search".to_owned(),
            "web.fetch".to_owned(),
        ]),
        expiry_sequence: 300,
        opening_client: "mcp".to_owned(),
        policy_version: "policy-v1".to_owned(),
        core_sequence: 50,
    }
}

fn published_receipt_keys(document: &Value, keys: &[ReceiptKeyEntry]) -> Vec<ReceiptKeyEntry> {
    let mut rpc_keys = {
        let Some(expected_value) = document["web"]["receipt_key_index"].as_array() else {
            panic!("enrolled governed keys")
        };
        expected_value
    }
    .clone();
    for key in &mut rpc_keys {
        for field in ["keyId", "publicKey"] {
            key[field] = json!(format!("0x{}", {
                let Some(expected_value) = key[field].as_str() else {
                    panic!("binding receipt key digest")
                };
                expected_value
            }));
        }
    }
    let enrolled_keys = must(layerx_client::sequencer::decode_receipt_keys(&json!({
        "keys": rpc_keys,
    })));
    assert_eq!(enrolled_keys, keys);
    enrolled_keys
}

fn transcript_request(stream: TcpStream) -> (BufReader<TcpStream>, Value) {
    let mut reader = BufReader::new(stream);
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(must(value.trim().parse::<usize>()));
        }
    }
    let length = {
        let Some(expected_value) = length else {
            panic!("HTTP content length")
        };
        expected_value
    };
    assert!(length <= 65_536);
    let mut body = vec![0; length];
    must(reader.read_exact(&mut body));
    let request: Value = must(serde_json::from_slice(&body));
    assert_eq!(request["jsonrpc"], "2.0");
    (reader, request)
}

fn assert_payment(
    node: &NodeTranscript,
    case: &Value,
    call: &PrecompileCall,
    enrolled_keys: &[ReceiptKeyEntry],
) {
    let execution = node.execution(case, enrolled_keys);
    let payment = must(payer::receipt(
        execution.clone(),
        execution.tx_hash,
        execution.from,
        call,
        enrolled_keys,
    ));
    assert_eq!(payment.tx_hash, fixed(&case["receiptJson"]["txHash"]));
    assert_eq!(payment.receipt, unhex(&case["receiptJson"]["receipt"]));
    assert_eq!(payment.logs.len(), 1);
    assert_eq!(payment.logs[0].topics(), execution.logs[0].topics);
    assert_eq!(payment.logs[0].data(), execution.logs[0].data);
    let mut changed = call.clone();
    changed.arguments[1] = AbiArgument::Word([0x99; 32]);
    assert_eq!(
        payer::receipt(
            execution.clone(),
            execution.tx_hash,
            execution.from,
            &changed,
            enrolled_keys
        ),
        Err(WebPayerError::Refused)
    );
}
