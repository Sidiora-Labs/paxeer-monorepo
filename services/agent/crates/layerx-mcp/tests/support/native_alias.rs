use crate::option_assertion;
use crate::result_assertion;

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use layerx_agent_api::error::RequestId;
use layerx_agentd::prepare::evm::{self, PrecompileCall};
use layerx_agentd::tenant::Operation;
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_types::payload::ModuleRegistry;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use layerx_mcp::boundary;
use layerx_mcp::tools::write as native_consumer;
fn body_transport(bytes: &[u8]) -> Vec<u8> {
    let (mut sender, mut receiver) =
        result_assertion::required(std::os::unix::net::UnixStream::pair(), "daemon transport");
    result_assertion::required(
        receiver.set_read_timeout(Some(Duration::from_secs(5))),
        "socket deadline",
    );
    let bytes = bytes.to_vec();
    let writer = thread::spawn(move || {
        result_assertion::required(sender.write_all(&bytes), "send daemon body");
    });
    let mut body = Vec::new();
    result_assertion::required(receiver.read_to_end(&mut body), "receive daemon body");
    result_assertion::required(writer.join(), "transport writer");
    body
}

fn daemon_request(operation: Operation, request: &Value) -> Value {
    let credential = result_assertion::required(
        layerx_sdk::agent_envelope::EnvelopeCredential::new("repair-native", [7; 32], [8; 32], 1),
        "session coordinates",
    );
    let envelope = result_assertion::required(
        layerx_sdk::agent_envelope::encode_envelope(
            operation,
            RequestId(1),
            request,
            Some(&credential),
            operation.mutating().then(|| {
                result_assertion::required(
                    layerx_agent_api::idempotency::Key::new([9; 32]),
                    "request key",
                )
            }),
        ),
        "SDK request producer",
    );
    let bytes = body_transport(&result_assertion::required(
        serde_json::to_vec(&envelope),
        "request encoding",
    ));
    let decoded = result_assertion::required(
        layerx_agentd::agent_rpc::decode(&bytes),
        "daemon request consumer",
    );
    assert_eq!(decoded.operation, operation);
    assert_eq!(
        option_assertion::required(decoded.credential, "credential")
            .tenant()
            .as_str(),
        "repair-native"
    );
    Value::Object(decoded.request)
}

/// Runs the production daemon response encoder and SDK consumer, with no test response schema.
fn daemon_transport(value: Value) -> Value {
    let response = layerx_agentd::agent_rpc::mcp_result_response(RequestId(1), value);
    let bytes = body_transport(&response.body);
    let wire: Value = result_assertion::required(serde_json::from_slice(&bytes), "daemon wire");
    result_assertion::required(
        option_assertion::required(
            layerx_sdk::agent_envelope::decode_response(response.status, &wire),
            "SDK response codec",
        ),
        "daemon success",
    )
    .value
}

fn native_prepared() -> evm::Prepared {
    let payload =
        unhex(include_str!("../../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim());
    let key = result_assertion::required(k256::ecdsa::SigningKey::from_slice(&[3; 32]), "EVM key");
    let from = result_assertion::required(
        layerx_crypto::secp256k1::evm_address(
            key.verifying_key().to_encoded_point(false).as_bytes(),
        ),
        "sender",
    );
    result_assertion::required(
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
                maximum_payload_bytes: 65_536,
            },
            evm::PrepareRequest {
                from,
                call: PrecompileCall::ledger_transfer(payload),
                expected_nonce: Some(7),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 65_536,
            },
            100_000,
        ),
        "real daemon EVM preparation producer",
    )
}

fn native_request(prepared: &evm::Prepared) -> Value {
    use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};

    let d = &prepared.disclosure;
    let canonical = prepared.call.encode_unsigned();
    let id: [u8; 32] = Sha256::digest(&canonical).into();
    let raw = result_assertion::required(prepared.call.sign(&[3; 32]), "real owner signature");
    let (_, signature) = result_assertion::required(decode_signed(&raw), "typed signature");
    let destination = option_assertion::required(
        d.counterparties
            .iter()
            .find(|p| p.role == CounterpartyRole::Recipient),
        "recipient",
    )
    .account;
    let amount = option_assertion::required(
        d.amounts.iter().find(|a| a.role == AmountRole::Transfer),
        "amount",
    )
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
    changed: Option<&(&str, Value, bool)>,
) -> Result<Value, boundary::BoundaryRefusal> {
    native_consumer::execute_native_alias(
        "wallet.send",
        arguments,
        &result_assertion::required(ModuleRegistry::new(&[]), "registry"),
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
            if let Some((field, replacement, at_disclosure)) = changed {
                if *at_disclosure == disclose {
                    value[*field] = replacement.clone();
                }
            }
            Ok(daemon_transport(value))
        },
    )
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
pub fn native_unsigned_call_prepare_disclose_sign_and_submit_binding() {
    let prepared = native_prepared();
    let arguments = native_request(&prepared);
    let (held, signing, signed) = signed_native_call(&prepared, &arguments);

    // The node peer admits a real signed type-2 call and returns its real transaction id.
    // All durable submission transitions, bytes and tracking answers are owned by agentd.
    let (node_url, sent, peer) = submission_peer(&prepared);
    let node = submission_client(node_url);
    let (directory, mut store, tenant) = submission_store();
    let mut outbox = layerx_agentd::outbox::Outbox::default();
    let mut stages = Vec::new();
    let mut signed_bytes = None;
    let outcome = result_assertion::required(
        native_consumer::execute_native_alias(
            "wallet.send",
            &arguments,
            &result_assertion::required(ModuleRegistry::new(&[]), "registry"),
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
                        let signature = result_assertion::required(
                            unhex(option_assertion::required(
                                request["signature"].as_str(),
                                "owner signature",
                            ))
                            .try_into(),
                            "65 bytes",
                        );
                        signed_bytes = Some(result_assertion::required(
                            layerx_agentd::sign::attach_external_signature(&signing, signature),
                            "actual daemon signer",
                        ));
                        // The MCP consumer does not use the sign answer; return the verified preparation.
                        native_consumer::native_call_preparation_answer(&prepared, None)?
                    }
                    Operation::Submit => {
                        assert_eq!(request["preparation_ref"], held["preparation_id"]);
                        assert_eq!(request["variant"], "native_send_submit_v1");
                        let raw = option_assertion::required(
                            signed_bytes.as_ref(),
                            "sign stage before submission",
                        );
                        let admitted = verify_native_submission(raw, &signing, &prepared.from);
                        let response = result_assertion::required(
                            layerx_agentd::human_runtime::submit_raw_write(
                                &mut outbox,
                                &mut store,
                                tenant.clone(),
                                admitted.into_exact_bytes(),
                                &node,
                            ),
                            "actual daemon submission",
                        );
                        let (tracked, _) = result_assertion::required(
                            layerx_agentd::agent_rpc_dispatch::decode_write_answer(
                                RequestId(1),
                                response,
                            ),
                            "actual daemon tracked response",
                        );
                        json!({"activity_id":tracked["tx_hash"], "submission":tracked, "receipt":null})
                    }
                    Operation::Track => {
                        result_assertion::required(
                            layerx_agentd::agent_rpc_dispatch::track_write_answer(
                                RequestId(1),
                                &outbox,
                                option_assertion::required(
                                    request["submission_ref"].as_str(),
                                    "submission reference",
                                ),
                            ),
                            "actual daemon tracking",
                        )
                        .0
                    }
                    _ => panic!("unexpected owner operation"),
                };
                Ok(daemon_transport(value))
            },
        ),
        "prepare/disclose/sign/submit/track through the repaired consumer",
    );
    result_assertion::required(peer.join(), "node assertions");
    assert_native_submission(&stages, &outcome, &sent, &signed, &outbox);
    drop(store);
    result_assertion::required(std::fs::remove_dir_all(directory), "test state cleanup");
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
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
            consume_native(&prepared, &arguments, Some(&(field, replacement, false))).is_err(),
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
            consume_native(&prepared, &arguments, Some(&(field, replacement, true))).is_err(),
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
        let value = option_assertion::required(changed[field].as_str(), "hex").to_owned();
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

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
pub fn native_legacy_owner_contract_remains_refused() {
    let prepared = native_prepared();
    let request = native_request(&prepared);
    assert!(layerx_agentd::agent_rpc::validate_mcp_native_request(
        Operation::Prepare, &request["preparation"],
    ).is_err(), "the retired native owner contract must remain refused until domain 9 wires its sender-bound replacement");
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

fn signed_native_call(
    prepared: &evm::Prepared,
    arguments: &Value,
) -> (Value, layerx_agentd::sign::SigningPreparation, Vec<u8>) {
    let held = result_assertion::required(
        consume_native(prepared, arguments, None),
        "unsigned EIP-1559 accepted by real consumer",
    );
    assert_eq!(held["approval_required"], true);
    assert_eq!(held["canonical_bytes"], arguments["canonical_bytes"]);
    let signing = layerx_agentd::sign::SigningPreparation {
        transaction: prepared.clone(),
        actor_address: prepared.from,
    };
    let external = result_assertion::required(
        layerx_agentd::sign::external(&signing),
        "daemon signing package",
    );
    assert_eq!(external.canonical_bytes, prepared.call.encode_unsigned());
    let signature = result_assertion::required(
        unhex(option_assertion::required(
            arguments["signature"].as_str(),
            "signature",
        ))
        .try_into(),
        "65 bytes",
    );
    let signed = result_assertion::required(
        layerx_agentd::sign::attach_external_signature(&signing, signature),
        "daemon sign consumer",
    );
    let admitted = result_assertion::required(
        layerx_agentd::sign::verify_before_submit(
            &signed,
            &signing,
            &prepared.from,
            &result_assertion::required(ModuleRegistry::new(&[]), "registry"),
        ),
        "daemon submit admission",
    );
    assert_eq!(admitted.exact_bytes(), signed);
    assert_eq!(admitted.activity_id(), transaction_hash(&signed));
    let mut other = prepared.from;
    other[0] ^= 1;
    assert!(layerx_agentd::sign::verify_before_submit(
        &signed,
        &signing,
        &other,
        &result_assertion::required(ModuleRegistry::new(&[]), "registry")
    )
    .is_err());

    (held, signing, signed)
}

fn submission_peer(
    prepared: &evm::Prepared,
) -> (String, Arc<Mutex<Vec<u8>>>, thread::JoinHandle<()>) {
    let listener = result_assertion::required(TcpListener::bind("127.0.0.1:0"), "submission node");
    let node_url = format!(
        "http://{}",
        result_assertion::required(listener.local_addr(), "node address")
    );
    let expected_call = prepared.call.clone();
    let expected_sender = prepared.from;
    let sent = Arc::new(Mutex::new(Vec::new()));
    let received = Arc::clone(&sent);
    let peer = thread::spawn(move || {
        for expected_method in ["eth_chainId", "eth_sendRawTransaction"] {
            let (mut stream, _) = result_assertion::required(listener.accept(), "node request");
            let (line, _, body) = read_http(&mut stream);
            assert_eq!(line, "POST / HTTP/1.1");
            let request: Value =
                result_assertion::required(serde_json::from_slice(&body), "node JSON");
            assert_eq!(request["method"], expected_method);
            let result = if expected_method == "eth_chainId" {
                json!("0x7d")
            } else {
                let raw = unhex(option_assertion::required(
                    request["params"][0].as_str(),
                    "signed call",
                ));
                let (call, signature) =
                    result_assertion::required(decode_signed(&raw), "actual node call codec");
                assert_eq!(call, expected_call);
                assert_eq!(
                    result_assertion::required(
                        layerx_crypto::evm_transaction::recover_sender(&call, &signature),
                        "sender"
                    ),
                    expected_sender
                );
                result_assertion::required(received.lock(), "submitted bytes").clone_from(&raw);
                json!(format!("0x{}", hex(&transaction_hash(&raw))))
            };
            respond(
                &mut stream,
                &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
            );
        }
    });
    (node_url, sent, peer)
}

fn submission_store() -> (
    std::path::PathBuf,
    layerx_agentd::store::Store,
    layerx_agentd::store::TenantId,
) {
    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
    let directory = std::env::temp_dir().join(format!(
        "repair-22-31-224-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    ));
    result_assertion::required(std::fs::create_dir(&directory), "durable test directory");
    let store = result_assertion::required(
        layerx_agentd::store::Store::open(directory.join("store")),
        "real daemon store",
    );
    let tenant = result_assertion::required(
        layerx_agentd::store::TenantId::new("repair-native"),
        "tenant",
    );
    (directory, store, tenant)
}

fn assert_native_submission(
    stages: &[Operation],
    outcome: &Value,
    sent: &Mutex<Vec<u8>>,
    signed: &[u8],
    outbox: &layerx_agentd::outbox::Outbox,
) {
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
    assert_eq!(outcome["submission_ref"], hex(&transaction_hash(signed)));
    assert_eq!(
        *result_assertion::required(sent.lock(), "actual bytes"),
        signed
    );
    assert_eq!(
        result_assertion::required(
            outbox.exact_signed_bytes(transaction_hash(signed)),
            "durable bytes"
        ),
        signed
    );
}

fn submission_client(node_url: String) -> layerx_agentd::node_rpc::NodeRpc {
    result_assertion::required(
        layerx_agentd::node_rpc::NodeRpc::connect(&layerx_agentd::node_rpc::NodeRpcConfig {
            url: node_url,
            chain_id: 125,
            deadline: Duration::from_secs(5),
        }),
        "actual daemon node transport",
    )
}

fn verify_native_submission(
    raw: &[u8],
    signing: &layerx_agentd::sign::SigningPreparation,
    signer: &[u8; 20],
) -> layerx_agentd::sign::VerifiedSubmission {
    result_assertion::required(
        layerx_agentd::sign::verify_before_submit(
            raw,
            signing,
            signer,
            &result_assertion::required(ModuleRegistry::new(&[]), "registry"),
        ),
        "actual daemon submission verification",
    )
}
