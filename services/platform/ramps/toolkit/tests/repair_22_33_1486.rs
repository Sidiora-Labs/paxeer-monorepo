//! Repair record and cross-domain wiring: src/repair_22_33_1486/record.kvx.
//! The components under repair are the crate's descriptor reader, public codecs,
//! pinned client, journal and presentation producer. Network peers are loopback fixtures.
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{symlink, OpenOptionsExt as _, PermissionsExt as _};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_ramp_toolkit::clients::{
    hex, LayerxClient, LayerxConfig, LayerxSubmission, ReceiptKeyConfig, SecretFile,
};
use layerx_ramp_toolkit::evm::{decode_budget_spent, encode_draw, PAY_ADDRESS};
use layerx_ramp_toolkit::journal::{
    Journal, OrderSnapshot, TransitionEvidence, WorkflowStage as S,
};
use layerx_ramp_toolkit::{
    compile_customer_payment, verify_order_receipt, RampDirection, RampError, RampOrder,
    ReceiptEvidence,
};
use layerx_types::{account::AccountId, payload::ModuleRegistry, rung::Rung};
use native_tls::{Identity, TlsAcceptor};
use serde_json::{json, Value};
use sha3::{Digest as _, Keccak256};

#[path = "proof_ramp_layerx_config.rs"]
mod config_proof;
#[path = "evm_order.rs"]
mod evm_order;
use config_proof::ClientFiles;

fn write_secret(path: &std::path::Path, bytes: &[u8]) {
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(path)
        .unwrap();
    file.write_all(bytes).unwrap();
}

#[test]
fn secret_file_open_refuses_symlinks_replacement_and_unprotected_descriptors() {
    let files = ClientFiles::new();
    let path = files.0.join("secret");
    write_secret(&path, b"original credential");
    let secret = SecretFile::new(&path).unwrap();
    assert_eq!(secret.read().unwrap(), b"original credential");
    fs::rename(&path, files.0.join("original")).unwrap();
    write_secret(&path, b"private replacement");
    assert_eq!(secret.read(), Err(RampError::Configuration));
    let replacement = SecretFile::new(&path).unwrap();
    fs::set_permissions(&path, fs::Permissions::from_mode(0o644)).unwrap();
    assert_eq!(replacement.read(), Err(RampError::Configuration));
    assert!(SecretFile::new(&path).is_err());
    fs::remove_file(&path).unwrap();
    symlink(files.0.join("original"), &path).unwrap();
    assert!(SecretFile::new(&path).is_err());
    assert_eq!(secret.read(), Err(RampError::Configuration));
    fs::remove_file(&path).unwrap();
    fs::create_dir(&path).unwrap();
    assert!(SecretFile::new(&path).is_err());
    fs::remove_dir(&path).unwrap();
    write_secret(&path, &vec![1; 65_537]);
    assert!(SecretFile::new(&path).is_err());
    fs::remove_file(&path).unwrap();
    fs::hard_link(files.0.join("original"), &path).unwrap();
    assert!(SecretFile::new(&path).is_err());
    assert_eq!(secret.read(), Err(RampError::Configuration));
}

#[test]
fn anchor_registry_optionality_round_trips_required_pin_and_protected_credential() {
    let files = ClientFiles::new();
    let mut producer = files.producer(0);
    producer.max_fee_per_gas = 100;
    producer.max_priority_fee_per_gas = 2;
    let wire = serde_json::to_vec(&producer).unwrap();
    let decoded: LayerxConfig = serde_json::from_slice(&wire).unwrap();
    let client = decoded.build(files.http()).unwrap();
    assert_eq!(client.chain_id, producer.chain_id);
    assert_eq!(client.signer_token, "proof-credential-0");
    let document: Value = serde_json::from_slice(&wire).unwrap();
    for value in [Value::Null, json!(42)] {
        let mut malformed = document.clone();
        malformed["rpc_trust_anchor_der"] = value;
        assert!(serde_json::from_value::<LayerxConfig>(malformed).is_err());
    }
    let mut missing = document;
    missing
        .as_object_mut()
        .unwrap()
        .remove("rpc_trust_anchor_der");
    assert!(serde_json::from_value::<LayerxConfig>(missing).is_err());
    fs::remove_file(&producer.signer_token_file).unwrap();
    symlink(files.0.join("token-1"), &producer.signer_token_file).unwrap();
    assert_eq!(
        decoded.build(files.http()).err().unwrap(),
        "protected client credential unavailable"
    );
}

#[test]
fn draw_registry_signature_preserves_historical_authority_and_destination() {
    let grant = [1; 32];
    let destination = [2; 32];
    let purpose = [3; 32];
    let context = [4; 32];
    let idempotency = [5; 32];
    let encoded = encode_draw(
        grant,
        destination,
        u128::MAX,
        idempotency,
        u64::MAX,
        purpose,
        context,
    );
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 9,
        max_fee_per_gas: 100,
        max_priority_fee_per_gas: 1,
        gas_limit: 100_000,
        to: layerx_ramp_toolkit::evm::LEDGER_ADDRESS,
        data: encoded.clone(),
    };
    let raw = call.sign(&[3; 32]).unwrap();
    let (received, signature) = decode_signed(&raw).unwrap();
    assert_eq!(received, call);
    assert_ne!(recover_sender(&received, &signature).unwrap(), [0; 20]);
    assert_eq!(
        &encoded[..4],
        &Keccak256::digest(b"draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)")[..4]
    );
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&u128::MAX.to_be_bytes());
    let mut sequence = [0; 32];
    sequence[24..].copy_from_slice(&u64::MAX.to_be_bytes());
    for (index, expected) in [
        grant,
        destination,
        amount,
        idempotency,
        sequence,
        purpose,
        context,
    ]
    .iter()
    .enumerate()
    {
        assert_eq!(&received.data[4 + index * 32..36 + index * 32], expected);
    }
    let order = evm_order::order(RampDirection::OffRamp);
    let maintained = compile_customer_payment(&order).unwrap();
    assert_eq!(maintained.to, PAY_ADDRESS);
    assert_ne!(&maintained.data[..4], &encoded[..4]);
    let mut invalid = order.clone();
    invalid.payer_grant_purpose = None;
    invalid.order_digest = invalid.digest();
    assert_eq!(
        compile_customer_payment(&invalid),
        Err(RampError::PayerGrantRequired)
    );
}

#[test]
fn offramp_account_disagreement_binds_compiler_signed_transport_and_receipt_destination() {
    let order = evm_order::order(RampDirection::OffRamp);
    let accounts = order.payment_accounts().unwrap();
    let expected = layerx_wire::hash::account_id_for_protocol(
        &AccountId::parse(&order.operator.account).unwrap(),
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    )
    .unwrap();
    assert_eq!(accounts.to, expected);
    assert_ne!(accounts.from, accounts.to);
    let compiled = compile_customer_payment(&order).unwrap();
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    ))
    .unwrap();
    let native_method = abi
        .as_array()
        .unwrap()
        .iter()
        .find(|entry| entry["type"] == "function" && entry["name"] == "budgetSpend")
        .unwrap();
    let inputs = native_method["inputs"].as_array().unwrap();
    assert_eq!(
        inputs
            .iter()
            .map(|input| input["name"].as_str().unwrap())
            .collect::<Vec<_>>(),
        ["budgetId", "recipient", "amount"]
    );
    let signature = format!(
        "budgetSpend({})",
        inputs
            .iter()
            .map(|input| input["type"].as_str().unwrap())
            .collect::<Vec<_>>()
            .join(",")
    );
    assert_eq!(
        &compiled.data[..4],
        &Keccak256::digest(signature.as_bytes())[..4]
    );
    assert_eq!(&compiled.data[36..68], &expected);
    let log = evm_order::payment_log(&order);
    assert_eq!(
        decode_budget_spent(&log),
        Some((
            order.payer_grant.unwrap(),
            expected,
            order.quote.layerx_amount
        ))
    );
    let evidence = evm_order::signed_evidence(&order, vec![log.clone()]);
    let (consumed, _) = decode_signed(&evidence.raw_transaction).unwrap();
    assert_eq!(consumed.to, compiled.to);
    assert_eq!(consumed.data, compiled.data);
    let leg = verify_order_receipt(&order, &evidence).unwrap();
    assert_eq!(leg.tx_hash, transaction_hash(&evidence.raw_transaction));
    // A newly signed receipt cannot authorize another destination, budget or amount.
    for index in [0, 31, 63] {
        let mut changed = log.clone();
        changed.data[index] ^= 1;
        assert_eq!(
            verify_order_receipt(&order, &evm_order::signed_evidence(&order, vec![changed])),
            Err(RampError::ReceiptMismatch)
        );
    }
    let mut wrong_budget = log.clone();
    wrong_budget.topics[1][0] ^= 1;
    assert_eq!(
        verify_order_receipt(
            &order,
            &evm_order::signed_evidence(&order, vec![wrong_budget])
        ),
        Err(RampError::ReceiptMismatch)
    );
    let mut extra = log.clone();
    extra.topics.push([1; 32]);
    assert_eq!(
        verify_order_receipt(&order, &evm_order::signed_evidence(&order, vec![extra])),
        Err(RampError::ReceiptMismatch)
    );
    let mut wrong_emitter = log.clone();
    wrong_emitter.address[0] ^= 1;
    assert_eq!(
        verify_order_receipt(
            &order,
            &evm_order::signed_evidence(&order, vec![wrong_emitter])
        ),
        Err(RampError::ReceiptMismatch)
    );
    let mut changed_order = order.clone();
    changed_order.operator.account = "agent:did:layerx:other-operator:main".into();
    changed_order.order_digest = changed_order.digest();
    assert_eq!(
        verify_order_receipt(&changed_order, &evidence),
        Err(RampError::ReceiptMismatch)
    );
}

const WORKER: &str = "repair-worker";
fn transition(
    journal: &mut Journal,
    order: &RampOrder,
    from: S,
    to: S,
    evidence: TransitionEvidence,
) {
    journal
        .transition(order.order_digest, from, to, evidence, WORKER, 1_002)
        .unwrap();
}
fn ready_order(journal: &mut Journal, order: &RampOrder) {
    journal.create_order(order.clone(), 1_000).unwrap();
    journal
        .acquire_lease(order.order_digest, WORKER, 1_001, 100)
        .unwrap();
    if order.direction() == RampDirection::OffRamp {
        transition(
            journal,
            order,
            S::CompliancePending,
            S::AwaitingLayerxPayment,
            TransitionEvidence::empty(),
        );
    } else {
        transition(
            journal,
            order,
            S::CompliancePending,
            S::AwaitingExternalCredit,
            TransitionEvidence::empty(),
        );
        let mut evidence = TransitionEvidence::empty();
        evidence.provider_operation_id = Some(format!("provider-{}", order.order_id));
        evidence.provider_evidence_digest = Some([3; 32]);
        transition(
            journal,
            order,
            S::AwaitingExternalCredit,
            S::ProviderSettled,
            evidence,
        );
    }
}
fn planned(journal: &mut Journal, order: &RampOrder, raw: &[u8]) {
    let mut evidence = TransitionEvidence::empty();
    evidence.tx_hash = Some(transaction_hash(raw));
    evidence.raw_transaction = Some(raw.to_vec());
    evidence.rung = Some(Rung::Submitted);
    let from = if order.direction() == RampDirection::OnRamp {
        S::ProviderSettled
    } else {
        S::AwaitingLayerxPayment
    };
    transition(journal, order, from, S::LayerxSubmissionPlanned, evidence);
}

#[test]
fn presentation_legacy_identifier_versions_wire_and_recovers_transaction_hash() {
    let files = ClientFiles::new();
    let path = files.0.join("presentation.jsonl");
    let order = evm_order::order(RampDirection::OffRamp);
    let proof = evm_order::signed_evidence(&order, vec![evm_order::payment_log(&order)]);
    let verified = verify_order_receipt(&order, &proof).unwrap();
    let verifier_output = serde_json::to_value(&verified).unwrap();
    let mut journal = Journal::open(&path).unwrap();
    ready_order(&mut journal, &order);
    planned(&mut journal, &order, &proof.raw_transaction);
    let snapshot = journal.order(&order.order_digest).unwrap().clone();
    let response = snapshot.response();
    assert_eq!(response["version"], 2);
    assert_eq!(response["presentation"]["version"], 2);
    assert_eq!(
        response["evidence"]["transaction_hash"],
        verifier_output["transaction_hash"]
    );
    assert_eq!(
        response["presentation"]["transaction_hash"],
        verifier_output["transaction_hash"]
    );
    assert!(response["presentation"].get("activity_id").is_none());
    assert!(response["evidence"].get("tx_hash").is_none());
    assert!(verifier_output.get("tx_hash").is_none());
    drop(journal);
    let reopened = Journal::open(&path).unwrap();
    assert_eq!(
        reopened.order(&order.order_digest).unwrap().response(),
        response
    );
    let mut legacy: OrderSnapshot = snapshot;
    legacy.evidence.tx_hash = None;
    legacy.evidence.raw_transaction = None;
    legacy.evidence.activity_id = Some([9; 32]);
    assert_eq!(legacy.presentation().transaction_hash, None);
    let legacy_output = serde_json::to_value(legacy.presentation_v1()).unwrap();
    assert_eq!(legacy_output["version"], 1);
    assert_eq!(legacy_output["activity_id"], json!([9; 32].to_vec()));
    assert!(legacy_output.get("transaction_hash").is_none());
}

fn base64(bytes: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for chunk in bytes.chunks(3) {
        let n = (u32::from(chunk[0]) << 16)
            | (u32::from(*chunk.get(1).unwrap_or(&0)) << 8)
            | u32::from(*chunk.get(2).unwrap_or(&0));
        for shift in [18, 12] {
            out.push(TABLE[((n >> shift) & 63) as usize] as char);
        }
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
fn request(stream: &mut impl Read) -> (String, Value) {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        stream.read_exact(&mut byte).unwrap();
        header.push(byte[0]);
        assert!(header.len() <= 32_768);
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
    (header, serde_json::from_slice(&body).unwrap())
}
fn respond(stream: &mut impl Write, response: Value) {
    let body = serde_json::to_vec(&response).unwrap();
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    stream.write_all(&body).unwrap();
    stream.flush().unwrap();
}

struct Server {
    endpoint: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Server {
    fn new(tls: bool, mut answer: impl FnMut(String, Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind(("localhost", 0)).unwrap();
        listener.set_nonblocking(true).unwrap();
        let endpoint = format!(
            "{}://localhost:{}",
            if tls { "https" } else { "http" },
            listener.local_addr().unwrap().port()
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = thread::spawn(move || {
            let acceptor = tls.then(|| {
                TlsAcceptor::new(
                    Identity::from_pkcs12(
                        include_bytes!("../src/repair_22_33_1486/identity.p12"),
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
                    Err(error) => panic!("loopback accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                if let Some(acceptor) = &acceptor {
                    let mut stream = acceptor.accept(stream).unwrap();
                    let (header, body) = request(&mut stream);
                    respond(&mut stream, answer(header, body));
                    let _ = stream.shutdown();
                } else {
                    let mut stream = stream;
                    let (header, body) = request(&mut stream);
                    respond(&mut stream, answer(header, body));
                }
            }
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
        }
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

fn transaction_json(raw: &[u8]) -> Value {
    let (call, signature) = decode_signed(raw).unwrap();
    let sig = signature.as_bytes();
    json!({
        "blockHash": format!("0x{}",hex(&[8;32])), "blockNumber":"0x64", "from":format!("0x{}",hex(&recover_sender(&call,&signature).unwrap())),
        "gas":format!("{:#x}",call.gas_limit), "gasPrice": "0x1", "hash":format!("0x{}",hex(&transaction_hash(raw))),
        "input":format!("0x{}",hex(&call.data)), "nonce":format!("{:#x}",call.nonce), "to":format!("0x{}",hex(&call.to)),
        "transactionIndex":"0x0", "value":"0x0", "type":"0x2", "v":format!("{:#x}",sig[64]-27),
        "yParity":format!("{:#x}",sig[64]-27),
        "r":format!("0x{}",hex(&sig[..32]).trim_start_matches('0')), "s":format!("0x{}",hex(&sig[32..64]).trim_start_matches('0')),
        "chainId":format!("{:#x}",call.chain_id), "maxFeePerGas":format!("{:#x}",call.max_fee_per_gas),
        "maxPriorityFeePerGas":format!("{:#x}",call.max_priority_fee_per_gas), "accessList":[]
    })
}
fn receipt_json(evidence: &ReceiptEvidence, committed: bool) -> Value {
    let (call, signature) = decode_signed(&evidence.raw_transaction).unwrap();
    let key = &evidence.receipt_keys[0];
    let mut value = json!({
        "rung":"receipted", "txHash":format!("0x{}",hex(&transaction_hash(&evidence.raw_transaction))), "nonce":"0x9", "seq":"0x1",
        "height":"0x64", "index":"0x0", "timestamp":"0x3e9", "status":"0x1", "returnData":"0x",
        "logs":evidence.logs.iter().map(|log| json!({"address":format!("0x{}",hex(&log.address)), "topics":log.topics.iter().map(|topic|format!("0x{}",hex(topic))).collect::<Vec<_>>(), "data":format!("0x{}",hex(&log.data)), "logIndex":format!("{:#x}",log.log_index)})).collect::<Vec<_>>(),
        "gasUsed":"0x0", "cumulativeGasUsed":"0x0", "effectiveGasPrice":"0x0", "feeDenom":"upaxe", "paymentDenom":"upaxe", "feeAmount":"0x0",
        "sponsor":format!("0x{}",hex(&[0;20])), "from":format!("0x{}",hex(&recover_sender(&call,&signature).unwrap())), "to":format!("0x{}",hex(&call.to)),
        "contractAddress":format!("0x{}",hex(&[0;20])), "vmError":"", "keyId":format!("0x{}",hex(&key.key_id)),
        "receipt":format!("0x{}",hex(&evidence.signed_receipt)), "signingKey":format!("0x{}",hex(&key.public_key))
    });
    if committed {
        value["rung"] = json!("committed");
        value["blockNumber"] = json!("0x64");
        value["blockHash"] = json!(format!("0x{}", hex(&[8; 32])));
    }
    value
}

#[test]
fn transaction_cache_growth_uses_durable_ownership_across_terminal_orders_and_restart() {
    let files = ClientFiles::new();
    let mut orders = Vec::new();
    let mut proofs = BTreeMap::new();
    for index in 0..12 {
        let mut order = evm_order::order(RampDirection::OnRamp);
        order.order_id = format!("completed-{index}");
        order.order_digest = order.digest();
        let proof = evm_order::signed_evidence(&order, vec![evm_order::payment_log(&order)]);
        proofs.insert(
            format!("0x{}", hex(&transaction_hash(&proof.raw_transaction))),
            proof,
        );
        orders.push(order);
    }
    let signer_calls: Vec<_> = orders
        .iter()
        .map(|order| {
            let proof = &proofs[&format!(
                "0x{}",
                hex(&transaction_hash(
                    &evm_order::signed_evidence(order, vec![evm_order::payment_log(order)])
                        .raw_transaction
                ))
            )];
            decode_signed(&proof.raw_transaction).unwrap().0
        })
        .collect();
    let sign_count = Arc::new(Mutex::new(0_usize));
    let observed_signs = sign_count.clone();
    let signer = Server::new(true, move |header, body| {
        assert!(header.starts_with("POST /v1/signatures HTTP/1.1"));
        assert!(header.contains("Bearer proof-credential-0"));
        let mut index = observed_signs.lock().unwrap();
        let call = &signer_calls[*index];
        *index += 1;
        assert_eq!(body["algorithm"], "secp256k1");
        assert_eq!(body["key_handle"], "kms.operator-1.primary");
        assert_eq!(body["message"], base64(&call.signing_hash()));
        let raw = call.sign(&[3; 32]).unwrap();
        let (_, sig) = decode_signed(&raw).unwrap();
        json!({"signature":base64(&sig.as_bytes()[..64]), "recovery_id":sig.as_bytes()[64]-27})
    });
    let committed = Arc::new(Mutex::new(BTreeSet::<String>::new()));
    let observed = committed.clone();
    let selected = Arc::new(Mutex::new(String::new()));
    let current = selected.clone();
    let refused: BTreeSet<_> = orders
        .iter()
        .enumerate()
        .filter(|(index, _)| index % 2 == 1)
        .map(|(_, order)| {
            let raw = evm_order::signed_evidence(order, vec![evm_order::payment_log(order)])
                .raw_transaction;
            format!("0x{}", hex(&transaction_hash(&raw)))
        })
        .collect();
    let answers = proofs.clone();
    let rpc = Server::new(false, move |_, body| {
        let result = match body["method"].as_str().unwrap() {
            "eth_getTransactionCount" => json!("0x9"),
            "eth_estimateGas" => json!("0x186a0"),
            "eth_sendRawTransaction" => {
                let raw = layerx_ramp_toolkit::clients::decode_hex(
                    body["params"][0].as_str().unwrap(),
                    1_000_000,
                )
                .unwrap();
                let hash = format!("0x{}", hex(&transaction_hash(&raw)));
                assert!(answers.contains_key(&hash));
                if refused.contains(&hash) {
                    return json!({"jsonrpc":"2.0","id":body["id"],"error":{"code":-32000,"message":"LXP_TEST_REFUSAL","data":"LXP_TEST_REFUSAL"}});
                }
                json!(hash)
            }
            "pax_sequencerReceipt" => {
                let hash = body["params"][0].as_str().unwrap().to_owned();
                *current.lock().unwrap() = hash.clone();
                if observed.lock().unwrap().contains(&hash) {
                    receipt_json(&answers[&hash], true)
                } else {
                    json!({"rung":"submitted","txHash":hash,"nonce":"0x9"})
                }
            }
            "eth_blockNumber" => json!("0x64"),
            "eth_getBlockByNumber" => {
                let proof = &answers[&*current.lock().unwrap()];
                json!({"number":"0x64", "hash":format!("0x{}",hex(&[8;32])), "parentHash":format!("0x{}",hex(&[7;32])), "timestamp":"0x3e9", "transactionsRoot":format!("0x{}",hex(&[6;32])), "receiptsRoot":format!("0x{}",hex(&[5;32])), "transactions":[transaction_json(&proof.raw_transaction)]})
            }
            method => panic!("unexpected RPC method {method}"),
        };
        json!({"jsonrpc":"2.0","id":body["id"],"result":result})
    });
    let mut config = files.producer(0);
    config.rpc_endpoint = rpc.endpoint.clone();
    config.signer_endpoint = signer.endpoint.clone();
    config.chain_id = 1;
    config.actor_did = "did:layerx:operator-1".into();
    config.max_fee_per_gas = 67_000_000;
    config.max_priority_fee_per_gas = 1;
    config.gas_limit_cap = 100_000;
    let sample = proofs.values().next().unwrap();
    let (call, signature) = decode_signed(&sample.raw_transaction).unwrap();
    config.operator_address = format!("0x{}", hex(&recover_sender(&call, &signature).unwrap()));
    config.receipt_keys = vec![ReceiptKeyConfig {
        key_id: format!("0x{}", hex(&[6; 32])),
        public_key: format!(
            "0x{}",
            hex(&SigningKey::from_bytes(&[42; 32]).verifying_key().to_bytes())
        ),
        first_height: 100,
        last_height: 100,
    }];
    let client = config.build(files.http()).unwrap();
    let registry = ModuleRegistry::new(&[]).unwrap();
    let path = files.0.join("ownership.jsonl");
    let mut journal = Journal::open(&path).unwrap();
    let mut terminal = Vec::new();
    for (index, order) in orders.iter().enumerate() {
        ready_order(&mut journal, order);
        let prepared = client.prepare_payment(order, 9, 1_001, &registry).unwrap();
        let raw = prepared.raw_transaction().to_vec();
        let hash = prepared.tx_hash();
        assert_eq!(transaction_hash(&raw), hash);
        planned(&mut journal, order, &raw);
        // No client-owned bytes are returned by a receipt-only lookup, even immediately after preparation.
        let (answer, _, finality) = client.resolve_with_evidence(order, hash).unwrap();
        assert!(matches!(
            answer,
            LayerxSubmission::Pending {
                canonical_activity: None,
                ..
            }
        ));
        assert!(!finality);
        let submission = client.submit_prepared(order, prepared);
        assert_eq!(submission.raw_transaction(), Some(raw.as_slice()));
        if index % 2 == 1 {
            assert!(matches!(submission, LayerxSubmission::Refused { .. }));
            let mut evidence = TransitionEvidence::empty();
            evidence.refusal_code = Some("LXP_TEST_REFUSAL".into());
            transition(
                &mut journal,
                order,
                S::LayerxSubmissionPlanned,
                S::LayerxRefused,
                evidence,
            );
        } else {
            assert!(matches!(submission, LayerxSubmission::Pending { .. }));
            committed
                .lock()
                .unwrap()
                .insert(format!("0x{}", hex(&hash)));
            let (answer, proof, finality) = client
                .resolve_transaction(order, hash, Some(&raw), false)
                .unwrap();
            assert!(finality);
            let LayerxSubmission::Verified { leg, .. } = answer else {
                panic!("committed verified leg required")
            };
            assert_eq!(leg.rung, Rung::Committed);
            assert_eq!(
                verify_order_receipt(order, &proof.unwrap())
                    .unwrap()
                    .tx_hash,
                hash
            );
            let mut evidence = TransitionEvidence::empty();
            evidence.receipt_digest = Some(leg.receipt_digest);
            evidence.receipt_height = Some(leg.height);
            evidence.rung = Some(leg.rung);
            transition(
                &mut journal,
                order,
                S::LayerxSubmissionPlanned,
                S::LayerxVerified,
                evidence,
            );
            transition(
                &mut journal,
                order,
                S::LayerxVerified,
                S::Done,
                TransitionEvidence::empty(),
            );
        }
        let snapshot = journal.order(&order.order_digest).unwrap().clone();
        assert_eq!(
            snapshot.evidence.raw_transaction.as_deref(),
            Some(raw.as_slice())
        );
        terminal.push(snapshot);
    }
    assert_eq!(*sign_count.lock().unwrap(), 12);
    drop(client);
    drop(journal);
    let mut reopened = Journal::open(&path).unwrap();
    let restarted: LayerxClient = config.build(files.http()).unwrap();
    for snapshot in &terminal {
        assert_eq!(reopened.order(&snapshot.order.order_digest), Some(snapshot));
        if snapshot.stage == S::Done {
            let (answer, _, committed) = restarted
                .resolve_transaction(
                    &snapshot.order,
                    snapshot.evidence.tx_hash.unwrap(),
                    snapshot.evidence.raw_transaction.as_deref(),
                    false,
                )
                .unwrap();
            assert!(committed);
            assert!(matches!(answer, LayerxSubmission::Verified { .. }));
        } else {
            assert_eq!(snapshot.stage, S::LayerxRefused);
        }
    }
    reopened
        .recover_verified(|projection| {
            for expected in &terminal {
                let snapshot = projection
                    .order(&expected.order.order_digest)
                    .ok_or(RampError::Journal)?;
                let (answer, _, committed) = restarted.resolve_transaction(
                    &snapshot.order,
                    snapshot.evidence.tx_hash.ok_or(RampError::Layerx)?,
                    snapshot.evidence.raw_transaction.as_deref(),
                    false,
                )?;
                match (snapshot.stage, answer) {
                    (S::Done, LayerxSubmission::Verified { leg, .. })
                        if committed
                            && Some(leg.receipt_digest) == snapshot.evidence.receipt_digest
                            && Some(leg.height) == snapshot.evidence.receipt_height
                            && Some(leg.rung) == snapshot.evidence.rung => {}
                    (S::LayerxRefused, LayerxSubmission::Pending { .. })
                        if !committed
                            && snapshot.evidence.refusal_code.as_deref()
                                == Some("LXP_TEST_REFUSAL") => {}
                    _ => return Err(RampError::ReceiptMismatch),
                }
            }
            Ok(())
        })
        .unwrap();
    assert!(reopened.health().ready);
}
