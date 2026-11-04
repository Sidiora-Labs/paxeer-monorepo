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

use layerx_mcp::boundary;
use layerx_mcp::tools::write as native_consumer;
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
        unhex(include_str!("../../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim());
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

pub fn native_unsigned_call_prepare_disclose_sign_and_submit_binding() {
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

pub fn native_call_rejects_every_changed_response_and_intent_field() {
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

pub fn native_legacy_owner_contract_remains_refused() {
    let prepared = native_prepared();
    let request = native_request(&prepared);
    assert!(layerx_agentd::agent_rpc::validate_mcp_native_request(
        Operation::Prepare, &request["preparation"],
    ).is_err(), "the retired native owner contract must remain refused until domain 9 wires its sender-bound replacement");
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
