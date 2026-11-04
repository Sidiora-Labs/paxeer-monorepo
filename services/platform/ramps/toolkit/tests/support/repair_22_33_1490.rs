use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_client::sequencer::{ReceiptKeyEntry, SequencerReceipt};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_ramp_toolkit::clients::{
    decode_hex, decode_kms_signature_response, hex, verify_recovery_settlement, ComplianceClient,
    Endpoint, LayerxClient, LayerxConfig, MutualTlsClient, MutualTlsFiles, PaxeerCustodyClient,
    PaxeerSubmission, ProviderClient, ProviderResult, ProviderState, SecretFile,
};
use layerx_ramp_toolkit::engine::RampEngine;
use layerx_ramp_toolkit::evm::{
    encode_draw, encode_get_budget, verify_customer_budget, PAY_ADDRESS,
};
use layerx_ramp_toolkit::journal::{
    Journal, PaxeerObservation, TransitionEvidence, WorkflowStage as S,
};
use layerx_ramp_toolkit::{
    compile_customer_payment, verify_order_receipt, AuthenticatedPrincipal, CreateOrder,
    OperatorIdentity, QuoteTerms, RampDirection, RampError, RampOrder, ReceiptEvidence,
};
use layerx_types::{payload::ModuleRegistry, rung::Rung};
use native_tls::{Identity, TlsAcceptor};
use serde_json::{json, Value};

static NEXT: AtomicU64 = AtomicU64::new(0);
const WORKER: &str = "repair-1490";

struct Files(PathBuf);
impl Files {
    fn new() -> Self {
        let directory = std::env::temp_dir().join(format!(
            "ramp-1490-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&directory).unwrap();
        fs::set_permissions(&directory, fs::Permissions::from_mode(0o700)).unwrap();
        let files = Self(directory);
        files.write(
            "ca.pem",
            include_bytes!("../../src/repair_22_33_1486/ca.pem"),
        );
        files.write(
            "ca.der",
            include_bytes!("../../src/repair_22_33_1486/ca.der"),
        );
        files.write(
            "identity.p12",
            include_bytes!("../../src/repair_22_33_1486/identity.p12"),
        );
        files.write("password", b"ramp-config-proof");
        files.write("token", b"repair-1490-token");
        files
    }
    fn write(&self, name: &str, bytes: &[u8]) {
        let mut file = fs::OpenOptions::new()
            .create_new(true)
            .write(true)
            .mode(0o600)
            .open(self.0.join(name))
            .unwrap();
        file.write_all(bytes).unwrap();
    }
    fn http(&self) -> MutualTlsClient {
        MutualTlsClient::new(
            &MutualTlsFiles {
                ca_pem: self.0.join("ca.pem"),
                identity_pkcs12: SecretFile::new(self.0.join("identity.p12")).unwrap(),
                identity_password: SecretFile::new(self.0.join("password")).unwrap(),
            },
            Duration::from_secs(5),
        )
        .unwrap()
    }
    fn client(&self, endpoint: &str, address: [u8; 20], pin: ReceiptKeyEntry) -> LayerxClient {
        let config: LayerxConfig = serde_json::from_value(json!({
            "rpc_endpoint":endpoint, "rpc_trust_anchor_der":self.0.join("ca.der"),
            "chain_id":125, "operator_address":format!("0x{}",hex(&address)),
            "receipt_keys":[{"keyId":format!("0x{}",hex(&pin.key_id)), "publicKey":format!("0x{}",hex(&pin.public_key)), "firstHeight":pin.first_height, "lastHeight":pin.last_height}],
            "max_fee_per_gas":3000000000_u64, "max_priority_fee_per_gas":1000000000_u64,
            "gas_limit_cap":2000000, "signer_endpoint":"https://localhost:1",
            "signer_token_file":self.0.join("token"), "actor_did":"did:layerx:operator-1"
        })).unwrap();
        config.build(self.http()).unwrap()
    }
    fn provider(&self, endpoint: &str) -> ProviderClient {
        ProviderClient {
            http: self.http(),
            endpoint: Endpoint::parse(endpoint).unwrap(),
            credential: "repair-provider".into(),
            settlement_path: "/settlements".into(),
            status_path: "/status".into(),
        }
    }
    fn custody(&self, endpoint: &str) -> PaxeerCustodyClient {
        PaxeerCustodyClient {
            http: self.http(),
            endpoint: Endpoint::parse(endpoint).unwrap(),
            credential: "repair-custody".into(),
            broadcast_path: "/broadcast".into(),
            status_path: "/status".into(),
            operator_account: "operator".into(),
            wallet_address: "wallet".into(),
            vault_id: "vault".into(),
            signer_key_handle: "key".into(),
        }
    }
}
impl Drop for Files {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn read_request(stream: &mut impl Read) -> (String, Value) {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        stream.read_exact(&mut byte).unwrap();
        header.push(byte[0]);
        assert!(header.len() <= 32768);
    }
    let header = String::from_utf8(header).unwrap();
    let length: usize = header
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse().unwrap())
        })
        .unwrap();
    assert!(length <= 512 * 1024);
    let mut body = vec![0; length];
    stream.read_exact(&mut body).unwrap();
    (
        header,
        if body.is_empty() {
            Value::Null
        } else {
            serde_json::from_slice(&body).unwrap()
        },
    )
}
fn respond(stream: &mut impl Write, value: Value) {
    let bytes = serde_json::to_vec(&value).unwrap();
    write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",bytes.len()).unwrap();
    stream.write_all(&bytes).unwrap();
    stream.flush().unwrap();
}

// Peers speak the production contracts. The components being tested are the
// production ramp engine, durable journal, RPC client and authentication code.
struct Server {
    endpoint: String,
    requests: Arc<Mutex<Vec<(String, Value)>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Server {
    fn new(tls: bool, mut answer: impl FnMut(&str, &Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind(("localhost", 0)).unwrap();
        listener.set_nonblocking(true).unwrap();
        let endpoint = format!(
            "{}://localhost:{}",
            if tls { "https" } else { "http" },
            listener.local_addr().unwrap().port()
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let seen = requests.clone();
        let worker = thread::spawn(move || {
            let acceptor = tls.then(|| {
                TlsAcceptor::new(
                    Identity::from_pkcs12(
                        include_bytes!("../../src/repair_22_33_1486/identity.p12"),
                        "ramp-config-proof",
                    )
                    .unwrap(),
                )
                .unwrap()
            });
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                if let Some(acceptor) = &acceptor {
                    let mut stream = acceptor.accept(stream).unwrap();
                    let (header, body) = read_request(&mut stream);
                    seen.lock().unwrap().push((header.clone(), body.clone()));
                    respond(&mut stream, answer(&header, &body));
                    let _ = stream.shutdown();
                } else {
                    let mut stream = stream;
                    let (header, body) = read_request(&mut stream);
                    seen.lock().unwrap().push((header.clone(), body.clone()));
                    respond(&mut stream, answer(&header, &body));
                }
            }
        });
        Self {
            endpoint,
            requests,
            stop,
            worker: Some(worker),
        }
    }
    fn methods(&self) -> Vec<String> {
        self.requests
            .lock()
            .unwrap()
            .iter()
            .map(|(_, body)| body["method"].as_str().unwrap().into())
            .collect()
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap();
        }
    }
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    decode_hex(value.as_str().unwrap(), N)
        .unwrap()
        .try_into()
        .unwrap()
}
fn vector() -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap();
    assert_eq!(
        document["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = document["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap()
        .clone();
    assert_eq!(case["signedReceipt"], case["receiptJson"]["receipt"]);
    case
}
fn pin(case: &Value) -> ReceiptKeyEntry {
    ReceiptKeyEntry {
        key_id: fixed(&case["pinnedKey"]["keyId"]),
        public_key: fixed(&case["pinnedKey"]["publicKey"]),
        first_height: case["pinnedKey"]["firstHeight"].as_u64().unwrap(),
        last_height: case["pinnedKey"]["lastHeight"].as_u64().unwrap(),
    }
}
fn rpc_reply(request: &Value, result: Value) -> Value {
    json!({"jsonrpc":"2.0","id":request["id"],"result":result})
}
fn order(direction: RampDirection) -> RampOrder {
    RampOrder::bind(
        CreateOrder {
            order_id: match direction {
                RampDirection::OnRamp => "funding",
                RampDirection::OffRamp => "payment",
            }
            .into(),
            quote_id: "quote-1490".into(),
            payer_grant: (direction == RampDirection::OffRamp).then_some([7; 32]),
            payer_grant_purpose: (direction == RampDirection::OffRamp).then_some([6; 32]),
        },
        QuoteTerms {
            quote_id: "quote-1490".into(),
            direction,
            layerx_asset: [1; 32],
            layerx_amount: 1000,
            external_currency: "EUR".into(),
            external_amount_minor: 100,
            rate_numerator: 10,
            rate_denominator: 1,
            fee_minor: 2,
            maximum_slippage_bps: 25,
            context: [8; 32],
            provider_token: "product-eur".into(),
            payout_token: "beneficiary-1490".into(),
            expires_at: 2000,
        },
        AuthenticatedPrincipal {
            principal_id: "customer-1".into(),
            account: format!("agent:did:layerx:{}:main", hex(&[4; 32])),
        },
        OperatorIdentity {
            principal_id: "operator-1".into(),
            account: "agent:did:layerx:operator-1:main".into(),
            signer_key_handle: "kms.operator-1.primary".into(),
        },
        1000,
    )
    .unwrap()
}
fn payment_call(order: &RampOrder) -> Eip1559Call {
    let compiled = compile_customer_payment(order).unwrap();
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_fee_per_gas: 100,
        max_priority_fee_per_gas: 2,
        gas_limit: 100000,
        to: compiled.to,
        data: compiled.data,
    }
}
fn word(value: u64) -> [u8; 32] {
    let mut word = [0; 32];
    word[24..].copy_from_slice(&value.to_be_bytes());
    word
}
fn account(value: &str) -> [u8; 32] {
    layerx_wire::hash::account_id_for_protocol(
        &layerx_types::account::AccountId::parse(value).unwrap(),
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .unwrap()
}
// getBudget's ABI tuple is a peer fixture, not an execution transcript.
fn budget(order: &RampOrder) -> Vec<u8> {
    let mut words = [[0; 32]; 20];
    words[0] = word(32);
    words[1] = account(&order.customer.account);
    words[3] = order.quote.layerx_asset;
    words[4] = order.payer_grant_purpose.unwrap();
    words[12] = word(order.quote.expires_at * 1000);
    words[17] = word(18 * 32);
    words.concat()
}
fn provider_result(order: &RampOrder) -> ProviderResult {
    ProviderResult {
        operation_id: "provider-1490".into(),
        order_digest: order.order_digest,
        direction: order.direction(),
        order_id: order.order_id.clone(),
        quote_id: order.quote.quote_id.clone(),
        customer_principal: order.customer.principal_id.clone(),
        layerx_asset: order.quote.layerx_asset,
        layerx_amount: order.quote.layerx_amount,
        provider_token: order.quote.provider_token.clone(),
        beneficiary_token: order.quote.payout_token.clone(),
        amount_minor: order.quote.external_amount_minor,
        currency: order.quote.external_currency.clone(),
        state: ProviderState::Settled,
        evidence_digest: Some([9; 32]),
        refusal_code: None,
        retry_at: None,
    }
}
fn seed(journal: &mut Journal, order: &RampOrder, to: S) {
    journal.create_order(order.clone(), 1000).unwrap();
    journal
        .acquire_lease(order.order_digest, WORKER, 1000, 100)
        .unwrap();
    journal
        .transition(
            order.order_digest,
            S::CompliancePending,
            to,
            TransitionEvidence::empty(),
            WORKER,
            1001,
        )
        .unwrap();
}

pub fn engine_restart() {
    let files = Files::new();
    let funding = order(RampDirection::OnRamp);
    let payment = order(RampDirection::OffRamp);
    let call = payment_call(&payment);
    let raw = call.sign(&[1; 32]).unwrap();
    let (_, sig) = decode_signed(&raw).unwrap();
    let sender = recover_sender(&call, &sig).unwrap();
    let hash = transaction_hash(&raw);
    let expected_raw = raw.clone();
    let authority = budget(&payment);
    let node = Server::new(false, move |header, request| {
        assert!(header.starts_with("POST / HTTP/1.1"));
        let result = match request["method"].as_str().unwrap() {
            "eth_chainId" => json!("0x7d"),
            "eth_blockNumber" => json!("0x3e9"),
            "eth_call" => {
                let to = request["params"][0]["to"].as_str().unwrap();
                if to == format!("0x{}", hex(&PAY_ADDRESS)) {
                    assert_eq!(
                        request["params"][0]["data"],
                        format!("0x{}", hex(&encode_get_budget([7; 32])))
                    );
                    json!(format!("0x{}", hex(&authority)))
                } else {
                    assert_eq!(
                        to,
                        format!("0x{}", hex(&layerx_client::sequencer::ADDR_PRECOMPILE))
                    );
                    let mut encoded = vec![0; 12];
                    encoded.extend_from_slice(&sender);
                    json!(format!("0x{}", hex(&encoded)))
                }
            }
            "eth_getTransactionCount" => json!("0x7"),
            "eth_sendRawTransaction" => {
                assert_eq!(
                    request["params"],
                    json!([format!("0x{}", hex(&expected_raw))])
                );
                json!(format!("0x{}", hex(&hash)))
            }
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"], json!([format!("0x{}", hex(&hash))]));
                json!({"rung":"submitted","txHash":format!("0x{}",hex(&hash)),"nonce":"0x7"})
            }
            method => panic!("node must not serve a gateway namespace: {method}"),
        };
        rpc_reply(request, result)
    });
    let result = provider_result(&funding);
    let state = Arc::new(Mutex::new(result));
    let observed = state.clone();
    let funding_digest = funding.order_digest;
    let provider_peer = Server::new(true, move |header, body| {
        assert!(header.contains("Authorization: Bearer repair-provider"));
        if header.starts_with("POST /settlements HTTP/1.1") {
            assert_eq!(body["order_digest"], json!(funding_digest));
            assert!(header.contains(&format!("Idempotency-Key: {}", hex(&funding_digest))));
        } else {
            assert!(header.starts_with("GET /status/provider-1490 HTTP/1.1"));
            assert_eq!(*body, Value::Null);
        }
        serde_json::to_value(&*observed.lock().unwrap()).unwrap()
    });
    let client = files.client(&node.endpoint, sender, pin(&vector()));
    let provider = files.provider(&provider_peer.endpoint);
    let custody = files.custody("https://localhost:1");
    let compliance = ComplianceClient {
        http: files.http(),
        endpoint: Endpoint::parse("https://localhost:1").unwrap(),
        service_token: "unused".into(),
        verifying_key: [0; 32],
    };
    let registry = ModuleRegistry::new(&[]).unwrap();
    let path = files.0.join("engine.jsonl");
    let mut journal = Journal::open(&path).unwrap();
    seed(&mut journal, &funding, S::AwaitingExternalCredit);
    seed(&mut journal, &payment, S::AwaitingLayerxPayment);
    {
        let mut engine = RampEngine {
            journal: &mut journal,
            compliance: &compliance,
            provider: &provider,
            layerx: &client,
            registry: &registry,
            worker_id: WORKER,
            lease_seconds: 100,
        };
        engine.submit_provider(funding.order_digest, 1002).unwrap();
        engine
            .submit_customer_payment(payment.order_digest, &raw, 1003)
            .unwrap();
        assert_eq!(
            engine.submit_provider(payment.order_digest, 1004),
            Err(RampError::IllegalTransition)
        );
    }
    let expected = journal.order(&payment.order_digest).unwrap().clone();
    assert_eq!(expected.stage, S::LayerxPending);
    assert_eq!(expected.evidence.tx_hash, Some(hash));
    assert_eq!(
        expected.evidence.raw_transaction.as_deref(),
        Some(raw.as_slice())
    );
    assert_eq!(expected.evidence.rung, Some(Rung::Submitted));
    assert_eq!(expected.evidence.receipt_digest, None);
    let expected_funding = journal.order(&funding.order_digest).unwrap().clone();
    assert_eq!(expected_funding.stage, S::ProviderSettled);
    drop(journal);
    drop(client);
    let persisted = fs::read(&path).unwrap();
    let mut reopened = Journal::open(&path).unwrap();
    assert!(!reopened.health().ready);
    assert_eq!(reopened.order(&payment.order_digest), Some(&expected));
    assert_eq!(
        reopened.order(&funding.order_digest),
        Some(&expected_funding)
    );
    let restarted = files.client(&node.endpoint, sender, pin(&vector()));
    reopened
        .recover_verified(|projection| {
            verify_recovery_settlement(
                projection,
                &payment.operator,
                &provider,
                &restarted,
                &custody,
            )
        })
        .unwrap();
    assert!(reopened.health().ready);
    assert_eq!(reopened.order(&payment.order_digest), Some(&expected));
    assert_eq!(
        reopened.order(&funding.order_digest),
        Some(&expected_funding)
    );
    assert_eq!(fs::read(&path).unwrap(), persisted);
    drop(reopened);
    let mut reopened = Journal::open(&path).unwrap();
    state.lock().unwrap().evidence_digest = Some([10; 32]);
    assert_eq!(
        reopened.recover_verified(|projection| verify_recovery_settlement(
            projection,
            &payment.operator,
            &provider,
            &restarted,
            &custody
        )),
        Err(RampError::Provider)
    );
    assert!(!reopened.health().ready);
    assert_eq!(reopened.order(&payment.order_digest), Some(&expected));
    assert_eq!(
        reopened.order(&funding.order_digest),
        Some(&expected_funding)
    );
    assert_eq!(fs::read(&path).unwrap(), persisted);
    let methods = node.methods();
    assert_eq!(
        methods
            .iter()
            .filter(|method| *method == "eth_sendRawTransaction")
            .count(),
        1,
        "recovery must never rebroadcast"
    );
    assert!(methods.iter().all(|method| !method.starts_with("px_")));
    assert!(fs::read_to_string(path)
        .unwrap()
        .lines()
        .all(|line| serde_json::from_str::<Value>(line).unwrap()["version"] == 2));
}

pub fn recorded_receipt() {
    let files = Files::new();
    let case = vector();
    let transaction = fixed(&case["receiptJson"]["txHash"]);
    let response = Arc::new(Mutex::new(case["receiptJson"].clone()));
    let produced = response.clone();
    let node = Server::new(false, move |header, request| {
        assert!(header.starts_with("POST / HTTP/1.1"));
        assert_eq!(request["method"], "pax_sequencerReceipt");
        assert_eq!(
            request["params"],
            json!([format!("0x{}", hex(&transaction))])
        );
        rpc_reply(request, produced.lock().unwrap().clone())
    });
    let client = files.client(
        &node.endpoint,
        fixed(&case["receiptJson"]["from"]),
        pin(&case),
    );
    let receipt = client
        .sequencer
        .sequencer_receipt(&transaction)
        .unwrap()
        .unwrap();
    let (execution, inclusion) = client
        .authenticate_settlement(transaction, &receipt)
        .unwrap();
    let execution = execution.unwrap();
    assert_eq!(
        execution.receipt,
        decode_hex(case["signedReceipt"].as_str().unwrap(), 512 * 1024).unwrap()
    );
    assert!(inclusion.is_none());
    assert_eq!(execution.logs.len(), 1);
    assert!(layerx_ramp_toolkit::evm::decode_ledger_payment(&execution.logs[0]).is_some());
    // The node conformance fixture is not a native ramp execution. It must
    // never be relabelled as one, even though its payment event decodes.
    let evidence = ReceiptEvidence {
        raw_transaction: decode_hex(case["rawTransaction"].as_str().unwrap(), 512 * 1024).unwrap(),
        signed_receipt: execution.receipt.clone(),
        logs: execution.logs.clone(),
        receipt_keys: vec![pin(&case)],
    };
    assert!(verify_order_receipt(&order(RampDirection::OnRamp), &evidence).is_err());
    for change in 0..9 {
        let mut altered = case["receiptJson"].clone();
        match change {
            0 => {
                let mut bytes = execution.receipt.clone();
                *bytes.last_mut().unwrap() ^= 1;
                altered["receipt"] = json!(format!("0x{}", hex(&bytes)));
            }
            1 => altered["from"] = json!(format!("0x{}", hex(&[0; 20]))),
            2 => altered["logs"][0]["address"] = json!(format!("0x{}", hex(&PAY_ADDRESS))),
            3 | 4 => {
                let mut bytes = execution.logs[0].data.clone();
                bytes[if change == 3 { 0 } else { 63 }] ^= 1;
                altered["logs"][0]["data"] = json!(format!("0x{}", hex(&bytes)));
            }
            5 => altered["logs"][0]["topics"][2] = json!(format!("0x{}", hex(&[0; 32]))),
            6 => altered["status"] = json!("0x0"),
            7 => altered["nonce"] = json!("0x99"),
            _ => altered["gasUsed"] = json!("0x1"),
        }
        *response.lock().unwrap() = altered;
        let altered = client
            .sequencer
            .sequencer_receipt(&transaction)
            .unwrap()
            .unwrap();
        assert_eq!(
            client.authenticate_settlement(transaction, &altered),
            Err(RampError::ReceiptMismatch),
            "mutation {change}"
        );
    }
    *response.lock().unwrap() = case["receiptJson"].clone();
    let restored = client
        .sequencer
        .sequencer_receipt(&transaction)
        .unwrap()
        .unwrap();
    assert!(client
        .authenticate_settlement(transaction, &restored)
        .is_ok());
    assert_eq!(
        client.authenticate_settlement([0; 32], &restored),
        Err(RampError::ReceiptMismatch)
    );
}

fn base64(bytes: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in bytes.chunks(3) {
        let n = (u32::from(chunk[0]) << 16)
            | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
            | u32::from(*chunk.get(2).unwrap_or(&0));
        out.push(TABLE[((n >> 18) & 63) as usize] as char);
        out.push(TABLE[((n >> 12) & 63) as usize] as char);
        out.push(if chunk.len() > 1 {
            TABLE[((n >> 6) & 63) as usize] as char
        } else {
            '='
        });
        out.push(if chunk.len() > 2 {
            TABLE[(n & 63) as usize] as char
        } else {
            '='
        });
    }
    out
}
pub fn signer_contract() {
    let call = payment_call(&order(RampDirection::OffRamp));
    let raw = call.sign(&[1; 32]).unwrap();
    let (_, signature) = decode_signed(&raw).unwrap();
    let operator = recover_sender(&call, &signature).unwrap();
    let wire = json!({"signature":base64(&signature.as_bytes()[..64]),"recovery_id":signature.as_bytes()[64]-27});
    let decoded =
        decode_kms_signature_response(&call, &serde_json::to_vec(&wire).unwrap(), operator)
            .unwrap();
    assert_eq!(decoded.as_bytes(), signature.as_bytes());
    assert!(
        decode_kms_signature_response(&call, &serde_json::to_vec(&wire).unwrap(), [0; 20]).is_err()
    );
    for malformed in [
        json!({"signature":base64(signature.as_bytes())}),
        json!({"signature":base64(signature.as_bytes()),"recovery_id":0}),
        json!({"signature64":wire["signature"],"recovery_id":wire["recovery_id"]}),
        json!({"signature":wire["signature"],"recovery_id":27}),
        json!({"signature":wire["signature"],"recovery_id":null}),
        json!({"signature":wire["signature"],"recovery_id":wire["recovery_id"],"unknown":true}),
    ] {
        assert!(decode_kms_signature_response(
            &call,
            &serde_json::to_vec(&malformed).unwrap(),
            operator
        )
        .is_err());
    }
    let mut another_call = call.clone();
    another_call.nonce += 1;
    assert!(decode_kms_signature_response(
        &another_call,
        &serde_json::to_vec(&wire).unwrap(),
        operator
    )
    .is_err());
}

pub fn customer_authority() {
    let order = order(RampDirection::OffRamp);
    let compiled = compile_customer_payment(&order).unwrap();
    assert_eq!(compiled.to, PAY_ADDRESS);
    assert_eq!(&compiled.data[36..68], &account(&order.operator.account));
    let authority = budget(&order);
    verify_customer_budget(&order, &authority, true).unwrap();
    for index in [32, 96, 128, 12 * 32 + 31, 15 * 32 + 31, 16 * 32 + 31] {
        let mut altered = authority.clone();
        altered[index] ^= 1;
        assert_eq!(
            verify_customer_budget(&order, &altered, true),
            Err(RampError::ReceiptMismatch)
        );
    }
    let historical = encode_draw(
        order.payer_grant.unwrap(),
        account(&order.operator.account),
        order.quote.layerx_amount,
        order.order_digest,
        7,
        order.payer_grant_purpose.unwrap(),
        order.context,
    );
    assert_ne!(&compiled.data[..4], &historical[..4]);
    let files = Files::new();
    let call = payment_call(&order);
    let raw = call.sign(&[1; 32]).unwrap();
    let (_, signature) = decode_signed(&raw).unwrap();
    let address = recover_sender(&call, &signature).unwrap();
    let node = Server::new(false, move |_, request| {
        let result = match request["method"].as_str().unwrap() {
            "eth_call" => {
                if request["params"][0]["to"] == format!("0x{}", hex(&PAY_ADDRESS)) {
                    json!(format!("0x{}", hex(&authority)))
                } else {
                    let mut word = vec![0; 12];
                    word.extend_from_slice(&address);
                    json!(format!("0x{}", hex(&word)))
                }
            }
            "eth_getTransactionCount" => json!("0x7"),
            method => panic!("unexpected authority request {method}"),
        };
        rpc_reply(request, result)
    });
    let client = files.client(&node.endpoint, [9; 20], pin(&vector()));
    let prepared = client.prepare_customer_payment(&order, &raw, 1001).unwrap();
    assert_eq!(prepared.raw_transaction(), raw);
    assert_eq!(prepared.tx_hash(), transaction_hash(&raw));
    let mut retired = call.clone();
    retired.to = layerx_ramp_toolkit::evm::LEDGER_ADDRESS;
    retired.data = historical;
    assert_eq!(
        client.prepare_customer_payment(&order, &retired.sign(&[1; 32]).unwrap(), 1001),
        Err(RampError::ReceiptMismatch)
    );
    for change in 0..4 {
        let mut altered = call.clone();
        match change {
            0 => altered.data[36] ^= 1,
            1 => altered.data[99] ^= 1,
            2 => altered.chain_id += 1,
            _ => altered.to[19] ^= 1,
        };
        assert_eq!(
            client.prepare_customer_payment(&order, &altered.sign(&[1; 32]).unwrap(), 1001),
            Err(RampError::ReceiptMismatch)
        );
    }
    assert_eq!(
        client.prepare_customer_payment(&order, &call.sign(&[2; 32]).unwrap(), 1001),
        Err(RampError::ReceiptMismatch)
    );
    assert!(node
        .methods()
        .iter()
        .all(|method| method != "eth_sendRawTransaction"));
}

fn transaction_json(raw: &[u8]) -> Value {
    let (call, signature) = decode_signed(raw).unwrap();
    let sig = signature.as_bytes();
    json!({"blockHash":format!("0x{}",hex(&[8;32])),"blockNumber":"0x3e9","transactionIndex":"0x0",
        "hash":format!("0x{}",hex(&transaction_hash(raw))),"from":format!("0x{}",hex(&recover_sender(&call,&signature).unwrap())),
        "to":format!("0x{}",hex(&call.to)),"input":format!("0x{}",hex(&call.data)),"nonce":format!("{:#x}",call.nonce),
        "gas":format!("{:#x}",call.gas_limit),"gasPrice":"0x1","value":"0x0","type":"0x2","chainId":"0x7d",
        "maxFeePerGas":format!("{:#x}",call.max_fee_per_gas),"maxPriorityFeePerGas":format!("{:#x}",call.max_priority_fee_per_gas),
        "accessList":[],"v":format!("{:#x}",sig[64]-27),"yParity":format!("{:#x}",sig[64]-27),
        "r":format!("0x{}",hex(&sig[..32]).trim_start_matches('0')),"s":format!("0x{}",hex(&sig[32..64]).trim_start_matches('0'))})
}
pub fn custody_restart() {
    let files = Files::new();
    let case = vector();
    let transaction = fixed(&case["receiptJson"]["txHash"]);
    let raw = decode_hex(case["rawTransaction"].as_str().unwrap(), 512 * 1024).unwrap();
    let mut committed = case["receiptJson"].clone();
    committed["rung"] = json!("committed");
    committed["blockNumber"] = json!("0x3e9");
    committed["blockHash"] = json!(format!("0x{}", hex(&[8; 32])));
    let response = Arc::new(Mutex::new(committed.clone()));
    let produced = response.clone();
    let chain = Arc::new(AtomicU64::new(125));
    let observed_chain = chain.clone();
    let block = Arc::new(Mutex::new(
        json!({"number":"0x3e9","hash":format!("0x{}",hex(&[8;32])),"parentHash":format!("0x{}",hex(&[7;32])),"timestamp":"0x1","transactionsRoot":format!("0x{}",hex(&[6;32])),"receiptsRoot":format!("0x{}",hex(&[5;32])),"transactions":[transaction_json(&raw)]}),
    ));
    let served_block = block.clone();
    let node = Server::new(false, move |header, request| {
        assert!(header.starts_with("POST / HTTP/1.1"));
        let result = match request["method"].as_str().unwrap() {
            "eth_chainId" => json!(format!("{:#x}", observed_chain.load(Ordering::Acquire))),
            "eth_blockNumber" => json!("0x3e9"),
            "pax_sequencerReceipt" => {
                assert_eq!(
                    request["params"],
                    json!([format!("0x{}", hex(&transaction))])
                );
                produced.lock().unwrap().clone()
            }
            "eth_getBlockByNumber" => {
                assert_eq!(request["params"], json!(["0x3e9", true]));
                served_block.lock().unwrap().clone()
            }
            method => panic!("hosted method must never reach the node root: {method}"),
        };
        rpc_reply(request, result)
    });
    let key = [2; 32];
    let custody_peer = Server::new(true, move |header, body| {
        assert!(header.starts_with(&format!(
            "GET /status/by-idempotency/{} HTTP/1.1",
            hex(&key)
        )));
        assert!(header.contains("Authorization: Bearer repair-custody"));
        assert_eq!(*body, Value::Null);
        serde_json::to_value(PaxeerSubmission {
            operation_id: "custody-1490".into(),
            idempotency_key: key,
            operator_account: "operator".into(),
            wallet_address: "wallet".into(),
            vault_id: "vault".into(),
            asset: [1; 32],
            amount: 100,
            transaction_hash: format!("0x{}", hex(&transaction)),
        })
        .unwrap()
    });
    let custody = files.custody(&custody_peer.endpoint);
    let provider = files.provider("https://localhost:1");
    let client = files.client(
        &node.endpoint,
        fixed(&case["receiptJson"]["from"]),
        pin(&case),
    );
    let path = files.0.join("custody.jsonl");
    let mut journal = Journal::open(&path).unwrap();
    journal.plan_paxeer(key, [1; 32], 100, 1000).unwrap();
    journal
        .observe_paxeer(
            key,
            PaxeerObservation {
                operation_id: "custody-1490",
                transaction_hash: transaction,
                rung: Rung::Committed,
                block_hash: Some([8; 32]),
                block_height: Some(1001),
            },
            1001,
        )
        .unwrap();
    let expected = journal.paxeer(&key).unwrap().clone();
    drop(journal);
    let persisted = fs::read(&path).unwrap();
    assert!(std::str::from_utf8(&persisted)
        .unwrap()
        .lines()
        .all(|line| serde_json::from_str::<Value>(line).unwrap()["version"] == 2));
    let operator = order(RampDirection::OnRamp).operator;
    let mut journal = Journal::open(&path).unwrap();
    assert!(!journal.health().ready);
    journal
        .recover_verified(|projection| {
            verify_recovery_settlement(projection, &operator, &provider, &client, &custody)
        })
        .unwrap();
    assert!(journal.health().ready);
    assert_eq!(journal.paxeer(&key), Some(&expected));
    assert_eq!(fs::read(&path).unwrap(), persisted);
    drop(journal);
    // Each contradiction starts with the same persisted terminal v2 bytes.
    for change in 0..7 {
        let mut journal = Journal::open(&path).unwrap();
        let mut current = committed.clone();
        match change {
            0 => chain.store(126, Ordering::Release),
            1 => current = case["receiptJson"].clone(),
            2 => {
                current = json!({"rung":"submitted","txHash":format!("0x{}",hex(&transaction)),"nonce":case["receiptJson"]["nonce"]})
            }
            3 => current["status"] = json!("0x0"),
            4 => current["blockHash"] = json!(format!("0x{}", hex(&[9; 32]))),
            5 => {
                let mut signed =
                    decode_hex(case["signedReceipt"].as_str().unwrap(), 512 * 1024).unwrap();
                *signed.last_mut().unwrap() ^= 1;
                current["receipt"] = json!(format!("0x{}", hex(&signed)));
            }
            _ => block.lock().unwrap()["transactions"] = json!([]),
        }
        *response.lock().unwrap() = current;
        assert!(
            journal
                .recover_verified(|projection| verify_recovery_settlement(
                    projection, &operator, &provider, &client, &custody
                ))
                .is_err(),
            "terminal contradiction {change}"
        );
        assert!(!journal.health().ready);
        assert_eq!(journal.paxeer(&key), Some(&expected));
        assert_eq!(fs::read(&path).unwrap(), persisted);
        chain.store(125, Ordering::Release);
        block.lock().unwrap()["transactions"] = json!([transaction_json(&raw)]);
    }
    let methods = node.methods();
    assert!(methods.iter().all(|method| !method.starts_with("px_")));
    assert!(!methods
        .iter()
        .any(|method| method == "eth_sendRawTransaction"));
}
