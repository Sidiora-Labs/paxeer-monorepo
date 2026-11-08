#[path = "support/budget_native_capture.rs"]
mod native_capture;
use serde_json::json;
use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agentd::budget::native_binding::{
    validate_native_preparation, validate_retained_preparation, verify_native_budget_submission,
    NativeBindingError,
};
use layerx_agentd::budget::*;
use layerx_agentd::prepare::{
    prepare_bound, AbiArgument, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
    EvmPrepared, PrecompileCall, PrecompileTarget,
};
use layerx_agentd::protocol_evidence::{
    EvidenceAuthority, RawReceiptEvidence, VerifiedReceiptEvidence,
};
use layerx_agentd::sign::{verify_before_submit, SigningPreparation, VerifiedSubmission};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::sequencer::{SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{AuthorizedBatch, MaintainedOutcomeEvidence};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

fn number(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn construct(value: &Value) -> PrecompileCall {
    let value = &value["construct"];
    let target = match value["target"].as_str().expect("target") {
        "pay" => PrecompileTarget::Pay,
        "ledger" => PrecompileTarget::Ledger,
        "programs" => PrecompileTarget::Programs,
        other => panic!("unexpected native target {other}"),
    };
    let selector = transaction_hash(
        value["signature"]
            .as_str()
            .expect("ABI signature")
            .as_bytes(),
    );
    PrecompileCall {
        target,
        selector: selector[..4].try_into().expect("selector"),
        arguments: value["arguments"]
            .as_array()
            .expect("ABI arguments")
            .iter()
            .map(|value| {
                if let Some(word) = value.get("word") {
                    AbiArgument::Word(fixed(word))
                } else {
                    AbiArgument::Bytes(
                        hex::decode(value["bytes"].as_str().expect("bytes")).expect("ABI bytes"),
                    )
                }
            })
            .collect(),
    }
}

struct Replay {
    url: String,
    stopped: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<()>>,
    missing: Arc<Mutex<Vec<Value>>>,
    answers: Arc<Mutex<Vec<Value>>>,
}

impl Replay {
    fn new(answers: Vec<Value>) -> Self {
        let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("fixture listener");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let url = format!("http://{}", listener.local_addr().expect("fixture address"));
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let missing = Arc::new(Mutex::new(Vec::new()));
        let unrecorded = Arc::clone(&missing);
        let answers = Arc::new(Mutex::new(answers));
        let served = Arc::clone(&answers);
        let worker = std::thread::spawn(move || {
            while !stop.load(Ordering::SeqCst) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(std::time::Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("fixture accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                    .expect("read deadline");
                let mut headers = Vec::new();
                while !headers.ends_with(b"\r\n\r\n") {
                    let mut byte = [0];
                    stream.read_exact(&mut byte).expect("HTTP header");
                    headers.push(byte[0]);
                    assert!(headers.len() <= 8192, "bounded HTTP header");
                }
                let headers = String::from_utf8(headers).expect("HTTP headers");
                let length: usize = headers
                    .lines()
                    .find_map(|line| {
                        let (key, value) = line.split_once(':')?;
                        key.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse().expect("length"))
                    })
                    .expect("content length");
                assert!(length <= 4_194_304);
                let mut bytes = vec![0; length];
                stream.read_exact(&mut bytes).expect("RPC request");
                let request: Value = serde_json::from_slice(&bytes).expect("request JSON");
                let answers = served.lock().expect("peer answers");
                let answer = answers.iter().find(|answer| {
                    answer["method"] == request["method"] && answer["params"] == request["params"]
                });
                let response = if let Some(answer) = answer {
                    if let Some(error) = answer.get("error") {
                        json!({"jsonrpc":"2.0", "id":request["id"], "error":error})
                    } else {
                        json!({"jsonrpc":"2.0", "id":request["id"], "result":answer["result"]})
                    }
                } else {
                    unrecorded
                        .lock()
                        .expect("unrecorded calls")
                        .push(request.clone());
                    json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32602,"message":"request absent from committed peer transcript"}})
                };
                let body = serde_json::to_vec(&response).expect("response JSON");
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("HTTP response");
                stream.write_all(&body).expect("response body");
            }
        });
        Self {
            url,
            stopped,
            worker: Some(worker),
            missing,
            answers,
        }
    }

    fn endpoint(&self) -> SequencerEndpoint {
        SequencerEndpoint::new(&self.url, &[]).expect("loopback production client")
    }
}

impl Drop for Replay {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::SeqCst);
        let result = self.worker.take().expect("replay worker").join();
        if !std::thread::panicking() {
            result.expect("peer replay");
            assert!(
                self.missing.lock().expect("unrecorded calls").is_empty(),
                "every request must match the committed peer transcript"
            );
        }
    }
}

fn add_account_and_fee_answers(answers: &mut Vec<Value>, case: &Value) {
    let corpus: Value =
        serde_json::from_slice(include_bytes!("fixtures/repair_22_31_217/corpus.json"))
            .expect("corpus");
    let views = &corpus["peer_views"];
    let identity = &views["identity"];
    let public: [u8; 32] = fixed(&identity["public"]);
    let owner: [u8; 32] = fixed(&identity["owner"]);
    let main: [u8; 32] = fixed(&identity["main"]);
    let sender = |seed: u8| {
        let key = k256::ecdsa::SigningKey::from_bytes((&[seed; 32]).into()).expect("fixture key");
        layerx_crypto::secp256k1::evm_address(
            key.verifying_key().to_encoded_point(false).as_bytes(),
        )
        .expect("fixture sender")
    };
    let fee_payer = sender(fixed::<32>(&case["signing_seed"])[0]);
    let proof = case.get("export").map(|path| {
        let root =
            PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/repair_22_31_217");
        serde_json::from_slice::<Value>(
            &std::fs::read(root.join(path.as_str().expect("proof export path")))
                .expect("recorded proof export"),
        )
        .expect("proof export JSON")
    });
    let (bound_address, bound_account) = if let Some(proof) = &proof {
        let did = Did::new(case["did"].as_str().expect("case DID").as_bytes()).expect("DID");
        assert_eq!(
            did.as_bytes(),
            format!("did:layerx:{}", hex::encode(public)).as_bytes()
        );
        assert_eq!(
            layerx_wire::hash::did_id_for_protocol(&did, 3).expect("identity key"),
            owner
        );
        assert_eq!(fixed::<32>(&proof["identity_key"]), owner);
        let account = &proof["proofs"]["account_join"]["account"];
        assert_eq!(account["layerx_did"], case["did"]);
        assert_eq!(account["layerx_account"], proof["accounts"]["owner"]);
        (
            fixed::<20>(&account["evm_address"]),
            fixed::<32>(&proof["accounts"]["owner"]),
        )
    } else {
        (fee_payer, main)
    };
    let address = [vec![0; 12], bound_address.to_vec()].concat();
    let mut add = |target: [u8; 20], signature: &[u8], args: &[u8], result: Vec<u8>| {
        let mut data = transaction_hash(signature)[..4].to_vec();
        data.extend(args);
        let row = json!({"method":"eth_call", "params":[{"to":format!("0x{}",hex::encode(target)),"data":format!("0x{}",hex::encode(data))},"0x8"], "result":format!("0x{}",hex::encode(result))});
        assert!(!answers
            .iter()
            .any(|old| old["method"] == row["method"] && old["params"] == row["params"]));
        answers.push(row);
    };
    let addr = layerx_client::sequencer::ADDR_PRECOMPILE;
    add(
        addr,
        b"getEvmAddrByLayerX(bytes32)",
        &public,
        address.clone(),
    );
    add(
        addr,
        b"getUnifiedAccount(address)",
        &address,
        [
            address.clone(),
            number(128).to_vec(),
            public.to_vec(),
            bound_account.to_vec(),
            number(0).to_vec(),
        ]
        .concat(),
    );
    add(
        addr,
        b"getLayerXDid(address)",
        &address,
        hex::decode(identity["did_result"].as_str().expect("DID reply")).expect("ABI reply"),
    );
    for (seed, asset) in [(0xa5, [4; 32]), (0xa6, [5; 32])] {
        let view = &views["fee_assets"][hex::encode(asset)];
        let payer = [vec![0; 12], sender(seed).to_vec()].concat();
        let mut fee_token = [0; 20];
        fee_token[18] = 0x10;
        fee_token[19] = 0x18;
        add(
            fee_token,
            b"getFeeDenom(address)",
            &payer,
            hex::decode(view["denomination"].as_str().expect("denomination reply"))
                .expect("ABI reply"),
        );
        add(
            PrecompileTarget::Ledger.address(),
            b"getAsset(bytes32)",
            &asset,
            hex::decode(view["metadata"].as_str().expect("metadata reply")).expect("ABI reply"),
        );
    }
    let asset: [u8; 32] = fixed(&case["fee_asset"]);
    add(
        PrecompileTarget::Ledger.address(),
        b"accountId(bytes32,bytes32)",
        &[owner, asset].concat(),
        main.to_vec(),
    );
    add(
        PrecompileTarget::Ledger.address(),
        b"accountOwner(bytes32)",
        &main,
        owner.to_vec(),
    );
    if bound_account != main {
        add(
            PrecompileTarget::Ledger.address(),
            b"accountOwner(bytes32)",
            &bound_account,
            owner.to_vec(),
        );
    }
    add(
        PrecompileTarget::Ledger.address(),
        b"balanceOf(bytes32,bytes32)",
        &[main, asset].concat(),
        number(1_000_000).to_vec(),
    );
    add(
        PrecompileTarget::Ledger.address(),
        b"nextSequence(bytes32,bytes32)",
        &[main, asset].concat(),
        number(0).to_vec(),
    );
    add(
        PrecompileTarget::Ledger.address(),
        b"accountOwner(bytes32)",
        &[5; 32],
        owner.to_vec(),
    );
    add(
        PrecompileTarget::Ledger.address(),
        b"nextSequence(bytes32,bytes32)",
        &[[5; 32], [3; 32]].concat(),
        number(0).to_vec(),
    );
    add(
        PrecompileTarget::Ledger.address(),
        b"accountOwner(bytes32)",
        &[8; 32],
        owner.to_vec(),
    );
    let nonce = if case["construct"]["signature"].as_str()
        == Some("budgetFundNative(bytes32,uint128,uint64)")
    {
        let args = &case["construct"]["arguments"][2]["word"];
        fixed::<32>(args).to_vec()
    } else {
        number(0).to_vec()
    };
    add(
        PrecompileTarget::Ledger.address(),
        b"nextSequence(bytes32,bytes32)",
        &[[8; 32], [4; 32]].concat(),
        nonce,
    );
    add(
        PrecompileTarget::Programs.address(),
        b"getValueAccount(bytes32,bytes32)",
        &[[0x11; 32], [0x33; 32]].concat(),
        hex::decode(
            views["value_account"]
                .as_str()
                .expect("value account reply"),
        )
        .expect("ABI reply"),
    );
}

fn budget_answer(record: &ProtocolBudgetRecord) -> Vec<u8> {
    let mut words = vec![[0; 32]; 20];
    for (index, value) in [
        (0, 32),
        (5, record.per_period_limit),
        (6, record.configured_period_limit),
        (7, record.carry_cap),
        (8, record.spent_this_period),
        (9, record.carried),
        (10, record.period_length.into()),
        (11, record.period_start.into()),
        (12, record.expiry.into()),
        (13, record.revocation_sequence.into()),
        (14, record.rollover_policy.into()),
        (15, u128::from(record.closed)),
        (16, u128::from(record.revoked)),
        (17, 18 * 32),
        (19, record.delegates.len() as u128),
    ] {
        words[index] = number(value);
    }
    for (index, value) in [
        (1, record.owner),
        (2, record.budget_account),
        (3, record.asset_id),
        (4, record.purpose_hash),
        (18, record.source_account.unwrap_or([0; 32])),
    ] {
        words[index] = value;
    }
    words.extend_from_slice(&record.delegates);
    words.concat()
}

// Peer fixture encoder: the node's module-12 layout, including signed log effects.
// It does not execute the precompile or claim a live-chain outcome.
fn mutation_receipt(signed: &[u8], record: &ProtocolBudgetRecord, revoke: bool) -> Value {
    let topic = transaction_hash(if revoke {
        b"BudgetRevoked(bytes32,uint128,uint64)".as_slice()
    } else {
        b"BudgetFunded(bytes32,uint128)".as_slice()
    });
    let data = if revoke {
        [number(77), number(1_000)].concat()
    } else {
        number(10).to_vec()
    };
    let log = layerx_client::sequencer::SequencerLog {
        address: decode_signed(signed).unwrap().0.to,
        topics: vec![topic, record.budget_id],
        data,
        log_index: 0,
    };
    node_receipt(signed, &[log])
}

fn node_receipt(signed: &[u8], logs: &[layerx_client::sequencer::SequencerLog]) -> Value {
    node_receipt_at(signed, logs, 9, [6; 32], [7; 32], "uhpx")
}

fn node_receipt_at(
    signed: &[u8],
    logs: &[layerx_client::sequencer::SequencerLog],
    height: u64,
    previous: [u8; 32],
    state: [u8; 32],
    denom: &str,
) -> Value {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_wire::encode::Encoder;
    use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
    let (call, signature) = decode_signed(signed).expect("peer signed transaction");
    let sender = recover_sender(&call, &signature).expect("peer sender");
    let tx_hash = transaction_hash(signed);
    let mut summary = Encoder::new(512);
    summary.u64(100_000).unwrap();
    summary.u64(100_000).unwrap();
    summary.u64(1).unwrap();
    summary.u32(0).unwrap();
    summary.fixed(&[0; 20]).unwrap();
    summary.fixed(&[0; 20]).unwrap();
    summary.fixed(&Sha256::digest([])).unwrap();
    summary.u32(0).unwrap();
    summary.u32(logs.len().try_into().unwrap()).unwrap();
    summary.u8(denom.len().try_into().unwrap()).unwrap();
    summary.fixed(denom.as_bytes()).unwrap();
    let mut effects = vec![summary.finish()];
    for log in logs {
        let mut body = log.address.to_vec();
        body.push(log.topics.len().try_into().unwrap());
        body.extend(log.topics.concat());
        body.extend(Sha256::digest(&log.data));
        effects.push(body);
    }
    let encode = |signature: Option<[u8; 64]>| {
        let mut e = Encoder::new(MAX_MESSAGE_BYTES);
        e.structure_header_version(0x5201, PROTOCOL_VERSION)
            .unwrap();
        e.u16(PROTOCOL_VERSION).unwrap();
        e.bytes(&tx_hash, 32).unwrap();
        e.u64(height).unwrap();
        e.bytes(&previous, 32).unwrap();
        e.bytes(&state, 32).unwrap();
        e.bytes(&[8; 32], 32).unwrap();
        e.i32(0).unwrap();
        e.sequence_length(effects.len(), 512).unwrap();
        for (index, body) in effects.iter().enumerate() {
            e.u16(12).unwrap();
            e.u16(index as u16).unwrap();
            e.u16(0).unwrap();
            e.u8(if index == 0 { 1 } else { 3 }).unwrap();
            e.u8(0).unwrap();
            e.bytes(&[0; 32], 32).unwrap();
            e.bytes(body, 256).unwrap();
        }
        e.u128(100_000).unwrap();
        e.bytes(&number(height.into()), 32).unwrap();
        e.u16(12).unwrap();
        e.u32(1).unwrap();
        e.u32(0).unwrap();
        e.u8(2).unwrap();
        e.bytes(&Sha256::digest(denom.as_bytes()), 32).unwrap();
        e.u128(0).unwrap();
        e.bytes(&[vec![0; 12], sender.to_vec()].concat(), 32)
            .unwrap();
        e.u128(1_000_000).unwrap();
        e.u128(900_000).unwrap();
        e.u64(call.nonce).unwrap();
        e.bytes(&[vec![0; 12], call.to.to_vec()].concat(), 32)
            .unwrap();
        e.u128(0).unwrap();
        e.u128(0).unwrap();
        e.bytes(&[0; 32], 32).unwrap();
        e.bytes(&[0; 32], 32).unwrap();
        e.bytes(&[1; 32], 32).unwrap();
        e.u64(1_700_000_001_000).unwrap();
        e.u8(u8::from(signature.is_some())).unwrap();
        if let Some(signature) = signature {
            e.bytes(&signature, 64).unwrap();
        }
        e.finish()
    };
    let key = SigningKey::from_bytes(&[8; 32]);
    let signature = key
        .sign(&layerx_wire::hash::receipt_digest(&encode(None)).unwrap())
        .to_bytes();
    let canonical = encode(Some(signature));
    let authorization = [SequencerAuthorization::new(
        [1; 32],
        key.verifying_key().to_bytes(),
        1,
        100,
    )];
    let verified = layerx_proof::receipt::verify_sequencer_signature(&canonical, &authorization)
        .expect("peer receipt signature and exact canonical encoding");
    let protocol = verified.protocol().expect("native node receipt");
    assert_eq!(
        protocol.protocol_version(),
        2,
        "native sequencer receipt ABI"
    );
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), signed[0]);
    assert_eq!(
        protocol.amount(),
        0,
        "Eip1559Call encodes zero native value"
    );
    assert_eq!(protocol.debit_sequence(), call.nonce);
    assert_eq!(protocol.from()[12..], sender);
    assert_eq!(protocol.to()[12..], call.to);
    layerx_proof::sequencer_logs::verify_logs(
        &verified,
        logs.iter()
            .map(|log| {
                layerx_proof::sequencer_logs::SequencerLog::new(
                    log.address,
                    log.topics.clone(),
                    log.data.clone(),
                    log.log_index,
                )
            })
            .collect(),
    )
    .expect("peer log bytes match the signed log effects");
    let h = |bytes: &[u8]| format!("0x{}", hex::encode(bytes));
    json!({
        "rung":"committed", "txHash":h(&tx_hash), "nonce":"0x0", "seq":format!("0x{height:x}"), "height":format!("0x{height:x}"),
        "index":"0x0", "timestamp":"0x18bcfe56be8", "status":"0x1", "returnData":"0x",
        "logs":logs.iter().map(|log| json!({"address":h(&log.address), "topics":log.topics.iter().map(|t|h(t)).collect::<Vec<_>>(), "data":h(&log.data), "logIndex":format!("0x{:x}",log.log_index)})).collect::<Vec<_>>(),
        "gasUsed":"0x186a0", "cumulativeGasUsed":"0x186a0", "effectiveGasPrice":"0x1",
        "feeDenom":denom, "paymentDenom":denom, "feeAmount":"0x186a0", "sponsor":h(&[0;20]),
        "from":h(&sender), "to":h(&call.to), "contractAddress":h(&[0;20]), "vmError":"",
        "keyId":h(&[1;32]), "receipt":h(&canonical), "signingKey":h(&key.verifying_key().to_bytes()),
        "blockNumber":format!("0x{height:x}"), "blockHash":h(&[9;32])
    })
}

fn add_mutation_answers(answers: &mut Vec<Value>, case: &Value) {
    let Some(signature) = case["construct"]["signature"].as_str() else {
        return;
    };
    let revoke = signature == "budgetRevoke(bytes32,uint64)";
    if !revoke && signature != "budgetFundNative(bytes32,uint128,uint64)" {
        return;
    }
    let records: Value = serde_json::from_slice(include_bytes!(
        "fixtures/repair_22_31_217/native-budget-records.json"
    ))
    .unwrap();
    let prior = ProtocolBudgetRecord::decode(&fixed_bytes(&records["v2"])).unwrap();
    let mut post = prior.clone();
    if revoke {
        post.revoked = true;
        post.revocation_sequence = 1_000;
        post.per_period_limit = post.spent_this_period;
    }
    for (height, record) in [(8, &prior), (9, &post)] {
        let mut data = transaction_hash(b"getBudget(bytes32)")[..4].to_vec();
        data.extend(record.budget_id);
        answers.push(json!({"method":"eth_call", "params":[{"to":format!("0x{}",hex::encode(PrecompileTarget::Pay.address())),"data":format!("0x{}",hex::encode(data))},format!("0x{height:x}")], "result":format!("0x{}",hex::encode(budget_answer(record)))}));
    }
    let mut data = transaction_hash(b"balanceOf(bytes32,bytes32)")[..4].to_vec();
    data.extend(post.budget_account);
    data.extend(post.asset_id);
    answers.push(json!({"method":"eth_call", "params":[{"to":format!("0x{}",hex::encode(PrecompileTarget::Ledger.address())),"data":format!("0x{}",hex::encode(data))},"0x9"], "result":format!("0x{}",hex::encode(number(if revoke {0} else {87})))}));
    let construct = construct(case);
    let signed = Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: construct.target.address(),
        data: construct.encode().unwrap(),
    }
    .sign(&fixed(&case["signing_seed"]))
    .unwrap();
    answers.push(json!({"method":"pax_sequencerReceipt", "params":[format!("0x{}",hex::encode(transaction_hash(&signed)))], "result":mutation_receipt(&signed,&post,revoke)}));
    let public = ed25519_dalek::SigningKey::from_bytes(&[8; 32])
        .verifying_key()
        .to_bytes();
    answers.push(json!({"method":"px_getReceiptKeys", "params":[], "result":{"keys":[{"keyId":format!("0x{}",hex::encode([1;32])),"publicKey":format!("0x{}",hex::encode(public)),"firstHeight":1,"lastHeight":100}]}}));
}

fn fixed_bytes(value: &Value) -> Vec<u8> {
    hex::decode(value.as_str().unwrap().trim_start_matches("0x")).unwrap()
}

fn registry(module: ModuleId, ordinal: u16) -> ModuleRegistry {
    let kind = ActivityType::new(module, ordinal).expect("operation");
    ModuleRegistry::new(&[ModuleRegistration::new(module, &[kind]).expect("registration")])
        .expect("registry")
}

fn prepare(call: PrecompileCall) -> EvmPrepared {
    prepare_as(call, [0x11; 20])
}

fn prepare_as(call: PrecompileCall, from: [u8; 20]) -> EvmPrepared {
    prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: 0,
            base_fee_per_gas: 1,
            head_height: 7,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 3,
            maximum_payload_bytes: 1_048_576,
        },
        EvmPrepareRequest {
            from,
            call,
            expected_nonce: Some(0),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1_048_576,
        },
        100_000,
    )
    .unwrap_or_else(|error| {
        panic!("prepare::evm::finish_call -> prepare::disclose::bind -> disclosure::call::bind_call refused {error:?}; chain_id=125, gas_limit=100000, priority_fee=1, max_fee=3")
    })
}

fn assert_native_call_limits(call: &Eip1559Call) {
    assert_eq!(call.chain_id, 125, "prepare::disclose::bind chain ID");
    assert!(call.gas_limit > 0, "prepare::disclose::bind gas limit");
    assert!(
        call.max_priority_fee_per_gas <= call.max_fee_per_gas,
        "prepare::disclose::bind priority fee {} exceeds maximum {}",
        call.max_priority_fee_per_gas,
        call.max_fee_per_gas,
    );
}

fn assert_fee_snapshot(case: &Case, height: u64) {
    assert!(height > 0, "fee_policy::native_fee_policy_for held height");
    let head = case.endpoint.block_number().expect("fee snapshot head RPC");
    assert!(head >= height, "fee snapshot head {head} below {height}");
    let header = case
        .endpoint
        .block_by_number(height)
        .expect("fee snapshot header RPC")
        .expect("fee snapshot header exists");
    let root = case
        .endpoint
        .state_root(height)
        .expect("fee snapshot root RPC");
    assert_eq!(header.number, height, "fee snapshot header number");
    assert_ne!(header.hash, [0; 32], "fee snapshot header hash");
    assert_ne!(root, [0; 32], "fee snapshot state root");
    assert_eq!(
        case.endpoint
            .block_by_number(height)
            .expect("fee snapshot header reread"),
        Some(header),
        "fee snapshot header must remain stable",
    );
    assert_eq!(
        case.endpoint
            .state_root(height)
            .expect("fee snapshot root reread"),
        root,
        "fee snapshot root must remain stable",
    );
}

// Peers replay committed codec/checkpoint fixtures. The budget, preparation,
// signature verifier and durable store are the production implementations.
struct Case {
    root: PathBuf,
    value: Value,
    endpoint: SequencerEndpoint,
    replay: Replay,
    native_capture: Option<native_capture::NativeCapture>,
}

impl Case {
    fn open(name: &str) -> Self {
        Self::open_mode(name, false)
    }
    fn open_execution(name: &str) -> Self {
        Self::open_mode(name, true)
    }
    fn open_mode(name: &str, execution: bool) -> Self {
        let root =
            PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/repair_22_31_217");
        let raw = include_bytes!("fixtures/repair_22_31_217/corpus.json");
        assert!(raw.len() <= 4_194_304);
        let document: Value = serde_json::from_slice(raw).expect("committed repair corpus JSON");
        assert_eq!(document["schema"], "layerx.budget-repair.22.31.217.v1");
        let revision = document["producer_revision"]
            .as_str()
            .expect("producer revision");
        assert_eq!(revision.len(), 40);
        assert!(revision.bytes().all(|byte| byte.is_ascii_hexdigit()));
        let mut value = document["cases"][name].clone();
        if matches!(
            name,
            "fund_source_nonce_zero" | "fund_source_nonce_above_height" | "revoke_counter"
        ) {
            value["receipt_public_key"] = json!(hex::encode(
                ed25519_dalek::SigningKey::from_bytes(&[8; 32])
                    .verifying_key()
                    .to_bytes()
            ));
        }
        assert!(value.is_object(), "real producer case {name}");
        let answers: Value = serde_json::from_slice(
            &std::fs::read(root.join(value["rpc_answers"].as_str().expect("peer capture")))
                .expect("committed peer capture"),
        )
        .expect("peer JSON");
        let mut answers = answers.as_array().expect("RPC transcript").clone();
        add_account_and_fee_answers(&mut answers, &value);
        add_mutation_answers(&mut answers, &value);
        if matches!(
            name,
            "program_settlement" | "deploy" | "upgrade" | "wind_down" | "wind_down_exit"
        ) {
            let construct = construct(&value);
            let signed = Eip1559Call {
                chain_id: 125,
                nonce: 0,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                gas_limit: 100_000,
                to: construct.target.address(),
                data: construct.encode().unwrap(),
            }
            .sign(&fixed(&value["signing_seed"]))
            .unwrap();
            let observation = if name == "program_settlement" {
                Value::Null
            } else {
                let mut receipt = node_receipt(&signed, &[]);
                receipt["rung"] = json!("receipted");
                receipt.as_object_mut().unwrap().remove("blockNumber");
                receipt.as_object_mut().unwrap().remove("blockHash");
                receipt
            };
            answers.push(json!({"method":"pax_sequencerReceipt", "params":[format!("0x{}", hex::encode(transaction_hash(&signed)))], "result":observation}));
        }
        let native_capture = if execution {
            value["tenant"] = json!(layerx_identity_binding::subject_namespace(
                "native-budget-peer",
                "operator"
            )
            .unwrap());
            Some(native_capture::configure(name, &mut value, &mut answers))
        } else {
            None
        };
        let replay = Replay::new(answers);
        let endpoint = replay.endpoint();
        assert_eq!(
            endpoint.chain_id().expect("recorded chain"),
            value["chain_id"].as_u64().expect("chain ID")
        );
        Self {
            root,
            value,
            endpoint,
            replay,
            native_capture,
        }
    }

    fn node(&self) -> SequencerEndpoint {
        self.replay.endpoint()
    }

    fn construct(&self) -> PrecompileCall {
        construct(&self.value)
    }

    fn bytes(&self, name: &str) -> Vec<u8> {
        if name == "signed_transaction" {
            let call = Eip1559Call {
                chain_id: self.value["chain_id"].as_u64().expect("chain ID"),
                nonce: 0,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                gas_limit: 100_000,
                to: self.construct().target.address(),
                data: self.construct().encode().expect("production ABI encoder"),
            };
            return call
                .sign(&fixed(&self.value["signing_seed"]))
                .expect("production native signing peer");
        }
        let path = self.value[name].as_str().unwrap_or_else(|| {
            panic!("the production peer did not emit {name} for this refused construct; see corpus provenance and the repair wiring record")
        });
        if let Some((path, field)) = path.split_once('#') {
            let document: Value = serde_json::from_slice(
                &std::fs::read(self.root.join(path)).expect("committed codec vector"),
            )
            .expect("codec JSON");
            return hex::decode(
                document[field]
                    .as_str()
                    .expect("codec vector")
                    .trim_start_matches("0x"),
            )
            .expect("codec bytes");
        }
        std::fs::read(self.root.join(path)).expect(name)
    }

    fn id(&self, name: &str) -> [u8; 32] {
        fixed(&self.value[name])
    }
    fn did(&self) -> Did {
        Did::new(self.value["did"].as_str().expect("actor DID").as_bytes()).expect("DID")
    }
    fn tenant(&self) -> TenantId {
        TenantId::new(self.value["tenant"].as_str().expect("tenant")).expect("tenant ID")
    }

    fn transaction(&self) -> (EvmPrepared, VerifiedSubmission, ModuleRegistry) {
        let signed = self.bytes("signed_transaction");
        let (call, signature) = decode_signed(&signed).expect("real signed EIP-1559 transaction");
        assert_native_call_limits(&call);
        let from = recover_sender(&call, &signature).expect("authenticated sender");
        let construct = self.construct();
        if self.value.get("generated_payload").is_none() {
            if let Some(capture) = self.value["capture"].as_str() {
                let captured: Value = serde_json::from_slice(
                    &std::fs::read(self.root.join(capture)).expect("native producer capture"),
                )
                .expect("capture JSON");
                if let Some(expected) = captured.get("construct") {
                    let mut expected = expected.clone();
                    if let Some(denom) = captured["native_fee_denom"].as_str() {
                        expected["arguments"][0]["word"] =
                            json!(hex::encode(transaction_hash(denom.as_bytes())));
                    }
                    assert_eq!(&expected, &self.value["construct"]);
                } else {
                    assert_eq!(
                        construct.encode().expect("production ABI encoder"),
                        hex::decode(
                            captured["evm"]["calldata"]
                                .as_str()
                                .expect("capture call")
                                .trim_start_matches("0x")
                        )
                        .expect("captured bytes")
                    );
                }
            }
        }
        let prepared = prepare_bound(
            EvmPreparationState {
                chain_id: call.chain_id,
                pending_nonce: call.nonce,
                base_fee_per_gas: self.value["base_fee"]
                    .as_u64()
                    .expect("observed base fee")
                    .into(),
                head_height: self.value["preparation_height"]
                    .as_u64()
                    .expect("held height"),
            },
            EvmPreparationDefaults {
                max_priority_fee_per_gas: call.max_priority_fee_per_gas,
                max_fee_per_gas: call.max_fee_per_gas,
                maximum_payload_bytes: 4_194_304,
            },
            EvmPrepareRequest {
                from,
                call: construct,
                expected_nonce: Some(call.nonce),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 4_194_304,
            },
            call.gas_limit,
        )
        .unwrap_or_else(|error| {
            panic!("prepare::evm::finish_call -> disclosure::call::bind_call refused {error:?}; construct={}, chain_id={}, gas_limit={}, priority_fee={}, max_fee={}",
                self.value["construct"]["signature"], call.chain_id, call.gas_limit,
                call.max_priority_fee_per_gas, call.max_fee_per_gas)
        });
        assert_eq!(prepared.call, call);
        validate_native_preparation(&prepared).expect("native call and disclosure");
        let registry = registry(
            prepared.disclosure.activity_type.module(),
            prepared.disclosure.activity_type.ordinal(),
        );
        let submission = verify_before_submit(
            &signed,
            &SigningPreparation {
                actor_address: from,
                transaction: prepared.clone(),
            },
            &from,
            &registry,
        )
        .expect("real signature verification");
        assert_eq!(
            verify_native_budget_submission(&prepared, &submission).expect("budget/native binding"),
            call
        );
        (prepared, submission, registry)
    }

    fn authority(&self) -> EvidenceAuthority {
        let keys = self
            .endpoint
            .receipt_keys()
            .expect("native receipt key producer");
        let key = keys
            .iter()
            .find(|key| key.public_key == self.id("receipt_public_key"))
            .expect("configured receipt key is served by this node");
        EvidenceAuthority::pinned_to_handshake(
            3,
            self.value["chain_id"].as_u64().expect("network") as u32,
            key.public_key,
        )
        .expect("receipt authority")
    }

    fn caps(&self, height: u64) -> layerx_agentd::read::CommittedCaps {
        let assets = self.value["assets"]
            .as_array()
            .expect("assets")
            .iter()
            .map(fixed)
            .collect::<Vec<_>>();
        layerx_agentd::read::CommittedReads::open(&self.endpoint, height)
            .expect("authenticated checkpoint")
            .inventory(&self.did(), &assets, self.id("fee_asset"))
            .expect("real source inventory")
    }

    fn receipt(
        &self,
    ) -> (
        VerifiedReceiptEvidence,
        AuthorizedBatch,
        SequencerAuthorization,
    ) {
        let canonical = self.bytes("receipt");
        let header_bytes = self.bytes("header");
        let header =
            layerx_wire::receipt::decode_batch_header(&header_bytes).expect("producer header");
        let public = self.id("receipt_public_key");
        assert!(self
            .endpoint
            .receipt_keys()
            .expect("real key registry")
            .iter()
            .any(|key| key.public_key == public));
        let decoded = layerx_wire::receipt::decode(&canonical).expect("producer execution receipt");
        let protocol = decoded.protocol().expect("native execution outcome");
        let authority = AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            header.previous_state_root(),
            protocol.resulting_state_root(),
            public,
        );
        let raw = RawReceiptEvidence::new(
            canonical.clone(),
            layerx_proof::merkle::decode_proof(&self.bytes("receipt_proof"))
                .expect("producer inclusion proof"),
            header_bytes.clone(),
            self.bytes("header_signature")
                .try_into()
                .expect("header signature"),
        );
        let authorization = SequencerAuthorization::new(
            header.sequencer_id(),
            public,
            header.batch_number(),
            header.batch_number(),
        );
        let maintenance = self.bytes("maintenance");
        let maintenance_proof =
            layerx_proof::merkle::decode_proof(&self.bytes("maintenance_proof"))
                .expect("maintenance proof");
        let header_signature = raw.header_signature();
        let evidence = MaintainedOutcomeEvidence {
            header: raw.canonical_header(),
            header_signature: &header_signature,
            activity_proof: raw.proof(),
            maintenance: &maintenance,
            maintenance_proof: &maintenance_proof,
            authorization: &authorization,
        };
        let receipts = self.value["batch_receipts"]
            .as_array()
            .expect("complete producer activity batch")
            .iter()
            .map(|value| {
                std::fs::read(self.root.join(value.as_str().expect("receipt path")))
                    .expect("batch receipt")
            })
            .collect::<Vec<_>>();
        let verified = VerifiedReceiptEvidence::verify_authorized_maintained(
            &raw,
            &authority,
            &evidence,
            &receipts,
            header.protocol_version(),
            header.network_id(),
        )
        .expect("authenticated native execution receipt");
        let observed = self
            .endpoint
            .sequencer_receipt(&verified.activity_id())
            .expect("real native receipt transport")
            .expect("known transaction");
        let SequencerReceipt::Committed { receipt, .. } = observed else {
            panic!("committed native outcome required")
        };
        assert_eq!(receipt.receipt, canonical);
        (verified, authority, authorization)
    }

    fn reservation(&self, sources: &VerifiedResolvedProgramCharges) -> ProgramBudgetReservation {
        use std::os::unix::fs::PermissionsExt as _;
        let root = temporary("allocated-reservation");
        let mut store = Store::open(&root).expect("real reservation store");
        let declared = LimitConfig {
            id: LimitId(fixed(&self.value["limit_id"])),
            name: "native source allocation".to_owned(),
            scope: LimitScope::Tenant(sources.actor()),
            ceiling: 1_000_000,
            consumed: 0,
        };
        let limiter = BudgetLimiter::new(vec![]).expect("production reservation limiter");
        let installed = install_enrolment_limits(&mut store, &limiter, &self.tenant(), &[declared])
            .expect("durable production enrolment");
        let limit = installed[0].limit_id;
        let allocations = sources
            .budget_allocations(|_| Ok(vec![limit]))
            .expect("production source allocation producer");
        let first = allocations.charges().first().expect("nonempty charges");
        assert!(allocations
            .charges()
            .iter()
            .all(|charge| charge.asset == first.asset && charge.source == first.source));
        let denomination_path = root.join("denominations.json");
        let document = json!({"version":1,"tenant":self.tenant().as_str(),"limits":[{
            "stable_id":hex::encode(installed[0].stable_id),
            "asset":hex::encode(first.asset),"source":hex::encode(first.source)
        }]});
        std::fs::write(&denomination_path, serde_json::to_vec(&document).unwrap())
            .expect("protected denomination source");
        std::fs::set_permissions(&denomination_path, std::fs::Permissions::from_mode(0o600))
            .expect("protect denomination source");
        let verified = layerx_agentd::config::load_program_budget_denominations(
            &denomination_path,
            &self.tenant(),
        )
        .expect("enrolment verifies the source and asset denomination");
        assert_eq!(
            verified.bindings()[0].1,
            ProgramLimitDenomination {
                asset: first.asset,
                source: Some(first.source),
            }
        );
        install_program_enrolment_denominations(&mut store, &limiter, &verified)
            .expect("public production denomination installation");
        let request = ProgramReservationRequest {
            id: allocations.preparation_digest(),
            charges: allocations.charges().to_vec(),
            expiry_sequence: sources.global_sequence().checked_add(10).unwrap(),
            current_sequence: sources.global_sequence(),
            core_deadline: None,
        };
        let reservation = limiter
            .stage_program_allocation(
                &request,
                &allocations,
                CoreTimestampMs(self.value["now_ms"].as_u64().unwrap()),
            )
            .expect("production allocation and reservation consumer")
            .publish();
        assert_eq!(limiter.held_reservations().unwrap(), 1);
        let key = TenantKey::new(self.tenant(), ObjectKind::Budget, reservation.id.to_vec())
            .expect("reservation store key");
        store
            .put_local(
                key.clone(),
                reservation.encode().expect("production reservation codec"),
            )
            .expect("retain allocated reservation");
        drop(store);
        let reopened = Store::open(&root).expect("restart reservation consumer");
        let recovered = ProgramBudgetReservation::decode(
            reopened.get(&key).expect("retained allocation").bytes(),
        )
        .expect("production reservation recovery codec");
        assert_eq!(recovered, reservation);
        assert_eq!(recovered.allocation_actor(), Some(sources.actor()));
        assert_eq!(
            recovered.allocation_state_root(),
            Some(sources.state_root())
        );
        assert_eq!(
            recovered.allocation_sequence(),
            Some(sources.global_sequence())
        );
        drop(reopened);
        std::fs::remove_dir_all(root).expect("remove isolated reservation store");
        recovered
    }
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    hex::decode(
        value
            .as_str()
            .expect("hex bytes")
            .strip_prefix("0x")
            .unwrap_or(value.as_str().expect("hex bytes")),
    )
    .expect("hex encoding")
    .try_into()
    .unwrap_or_else(|_| panic!("expected {N} bytes"))
}

fn temporary(name: &str) -> PathBuf {
    let mut suffix = [0; 16];
    getrandom::fill(&mut suffix).expect("temporary identity");
    std::env::temp_dir().join(format!("repair-22-31-217-{name}-{}", hex::encode(suffix)))
}

fn create_request() -> (BudgetRequest, BudgetCreateIdentity) {
    let identity = BudgetCreateIdentity {
        encoding_version: 2,
        budget_id: [1; 32],
        sender: [0x11; 20],
        budget_account: [2; 32],
        asset: [3; 32],
        purpose: [4; 32],
        per_period_limit: 100,
        carry_cap: 0,
        initial_amount: 50,
        period_length_ms: 1_000,
        period_start_ms: 1_000,
        expiry_ms: 10_000,
        revocation_sequence: 1,
        rollover: 1,
        source_account: [5; 32],
        source_sequence: 0,
    };
    (
        BudgetRequest {
            tenant: TenantId::new("repair").expect("tenant"),
            request_id: identity.budget_id,
            kind: BudgetKind::ProtocolBudget,
            asset: identity.asset,
            ceiling: 100,
            expiry_ms: 10_000,
            core_time_ms: 1_000,
            canonical_activity: vec![],
            verified_submission: None,
        },
        identity,
    )
}

#[test]
fn repair_create_source_nonce_first_use_prepares_submits_and_confirms() {
    let (request, mut identity) = create_request();
    let call = request
        .create_protocol_budget(&identity)
        .expect("nonce zero is valid");
    let signed = signed_native_call(&call);
    let (native, signature) = decode_signed(&signed).expect("production signed create call");
    let sender = recover_sender(&native, &signature).expect("authenticated create sender");
    assert_eq!(native.data.len(), 4 + 14 * 32);
    assert_eq!(&native.data[4 + 13 * 32..], &number(0));
    assert_eq!(
        layerx_crypto::disclosure::call::bind_call(&native, sender),
        Err(layerx_crypto::disclosure::DisclosureError::MalformedPayload),
        "call.rs native-create predicate rejects source_sequence == 0 at word 13",
    );
    let mut control = native.clone();
    control.data[4 + 13 * 32..].copy_from_slice(&number(1));
    layerx_crypto::disclosure::call::bind_call(&control, sender)
        .expect("identical ABI layout with source nonce one binds");
    identity.source_sequence = u64::MAX;
    assert_eq!(
        request.create_protocol_budget(&identity),
        Err(BudgetCreationError::InvalidLimit)
    );
    let case = Case::open("create_source_nonce");
    let signed = case.bytes("signed_transaction");
    let (call, signature) = decode_signed(&signed).expect("signed corpus create");
    let sender = recover_sender(&call, &signature).expect("authenticated corpus sender");
    assert_eq!(
        layerx_crypto::disclosure::call::bind_call(&call, sender),
        Err(layerx_crypto::disclosure::DisclosureError::MalformedPayload),
        "producer-to-consumer defect at native source word 13; see attempt_9.create_probe",
    );
    let identity = budget_create_identity(signed.as_slice(), &registry(ModuleId::Budget, 1))
        .expect("budget consumer accepts the production zero source nonce");
    assert_eq!(identity.source_sequence, 0);
    validate_native_budget_source(
        &case.endpoint,
        8,
        sender,
        identity.source_account,
        identity.asset,
        identity.source_sequence,
    )
    .expect("zero source nonce matches authenticated owner and checkpoint");
}

fn assert_current_binder_result(case: &Case) {
    let signed = case.bytes("signed_transaction");
    let (call, signature) = decode_signed(&signed).expect("native signed call");
    assert_native_call_limits(&call);
    let sender = recover_sender(&call, &signature).expect("authenticated sender");
    let result = prepare_bound(
        EvmPreparationState {
            chain_id: call.chain_id,
            pending_nonce: call.nonce,
            base_fee_per_gas: 1,
            head_height: 8,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: call.max_priority_fee_per_gas,
            max_fee_per_gas: call.max_fee_per_gas,
            maximum_payload_bytes: 4_194_304,
        },
        EvmPrepareRequest {
            from: sender,
            call: case.construct(),
            expected_nonce: Some(call.nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 4_194_304,
        },
        call.gas_limit,
    );
    if case.construct().target == PrecompileTarget::Programs {
        let bound = result.expect("lifecycle preparation must bind after 22.31.585");
        validate_native_preparation(&bound).expect("bound lifecycle call");
    } else {
        assert_eq!(
            result,
            Err(layerx_agentd::prepare::evm::PrepareError::Disclosure(
                layerx_crypto::disclosure::DisclosureError::MalformedPayload,
            ))
        );
    }
}

fn confirm_live_mutation(name: &str, expected_nonce: Option<u64>) {
    let case = Case::open(name);
    assert_current_binder_result(&case);
    let signed = case.bytes("signed_transaction");
    let registry = registry(ModuleId::Budget, 2);
    let height = case.value["preparation_height"]
        .as_u64()
        .expect("held checkpoint");
    let raw = case.bytes("prior_budget_record");
    let record = ProtocolBudgetRecord::decode(&raw).expect("real prior budget producer");
    let prior = ProtocolBudgetState::read_at(&case.endpoint, record.budget_id, height)
        .expect("real precompile prestate");
    assert_eq!(prior.record(), &record);
    let context = budget_state_context(&record, &raw, height, 77).expect("bound prior context");
    let decoded =
        budget_mutation_identity(&signed, &registry, &context).expect("native mutation identity");
    assert_eq!(decoded.source_sequence(), expected_nonce);
    if let Some(nonce) = decoded.source_sequence() {
        let (call, signature) = decode_signed(&signed).expect("signed call");
        validate_native_budget_source(
            &case.endpoint,
            height,
            recover_sender(&call, &signature).expect("sender"),
            context.source_account,
            context.asset,
            nonce,
        )
        .expect("authenticate source ownership and exact nonce at preparation");
    }
    if let Some(counter) = decoded.revocation_sequence() {
        assert!(counter > height && counter > record.revocation_sequence);
    }
    let tx_hash = transaction_hash(&signed);
    let receipt = case
        .endpoint
        .sequencer_receipt(&tx_hash)
        .expect("native receipt transport")
        .expect("recorded committed mutation");
    let post = ProtocolBudgetState::read_at(&case.endpoint, record.budget_id, 9)
        .expect("real resulting precompile state");
    let mut balance_call = transaction_hash(b"balanceOf(bytes32,bytes32)")[..4].to_vec();
    balance_call.extend(record.budget_account);
    balance_call.extend(record.asset_id);
    let balance_bytes = case
        .endpoint
        .call_at(9, &PrecompileTarget::Ledger.address(), &balance_call)
        .expect("independent resulting balance transport");
    assert_eq!(balance_bytes.len(), 32);
    assert_eq!(&balance_bytes[..16], &[0; 16]);
    let balance = u128::from_be_bytes(balance_bytes[16..].try_into().unwrap());
    assert_eq!(balance, if expected_nonce.is_some() { 87 } else { 0 });
    let confirmed = verify_committed_budget_mutation(
        &signed,
        &registry,
        &context,
        &receipt,
        &post,
        balance,
        &case.authority(),
    )
    .expect("native precompile mutation confirmation");
    assert_eq!(confirmed.activity_id(), tx_hash);
    if let Some(counter) = decoded.revocation_sequence() {
        assert!(counter > confirmed.observed_head_sequence());
        assert_eq!(confirmed.record().revocation_sequence, counter);
    }
    let mut altered = context.clone();
    altered.revocation_sequence = decoded
        .revocation_sequence()
        .unwrap_or(record.revocation_sequence)
        + 1;
    assert!(
        verify_committed_budget_mutation(
            &signed,
            &registry,
            &altered,
            &receipt,
            &post,
            balance,
            &case.authority(),
        )
        .is_err(),
        "prior counter must remain bound"
    );
    assert!(
        verify_committed_budget_mutation(
            &signed,
            &registry,
            &context,
            &receipt,
            &post,
            balance + 1,
            &case.authority(),
        )
        .is_err(),
        "resulting balance must remain exact"
    );
}

#[test]
fn repair_fund_source_nonce_zero_and_above_height_confirm() {
    for nonce in [0_u64, 1_000] {
        let call = PrecompileCall {
            target: PrecompileTarget::Pay,
            selector: transaction_hash(b"budgetFundNative(bytes32,uint128,uint64)")[..4]
                .try_into()
                .expect("selector"),
            arguments: vec![
                AbiArgument::Word([1; 32]),
                AbiArgument::Word(number(10)),
                AbiArgument::Word(number(nonce.into())),
            ],
        };
        let signed = signed_native_call(&call);
        let (native, signature) = decode_signed(&signed).expect("real native signed producer");
        recover_sender(&native, &signature).expect("real signature");
        assert_eq!(&native.data[68..100], &number(nonce.into()));
    }
    confirm_live_mutation("fund_source_nonce_zero", Some(0));
    confirm_live_mutation("fund_source_nonce_above_height", Some(1_000));
}

#[test]
fn repair_revoke_counter_above_height_confirms() {
    confirm_live_mutation("revoke_counter", None);
}

#[test]
fn repair_proof_revocation_counter_exports_and_verifies() {
    let case = Case::open("proof_revocation_counter");
    let export = case.bytes("export");
    let (height, hash) = budget_proof_committed_head(&export).expect("real authority v2 export");
    let checkpoint = layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
        &case.endpoint,
        height,
        Some(hash),
        &case.value["chain_id"]
            .as_u64()
            .expect("network")
            .to_string(),
    )
    .expect("real committed checkpoint");
    let document: Value = serde_json::from_slice(&export).expect("export JSON");
    assert_eq!(
        document["budget_state"]["verification"],
        json!(layerx_types::verify::VerificationLevel::COMMITTED.wire_rank())
    );
    let mut retired = document.clone();
    retired["budget_state"]["verification"] = json!(4);
    assert!(
        verify_budget_proof(
            &serde_json::to_vec(&retired).unwrap(),
            case.id("budget_id"),
            &BudgetProofTrust {
                tenant: "tenant",
                principal: "operator",
                owner: &case.did(),
                protocol_version: 3,
                network_id: 125,
                checkpoint: &checkpoint,
                endpoint: &case.endpoint,
                current_height: height,
                now_ms: 1_700_000_000_000,
                maximum_age_seconds: 60,
            },
        )
        .is_err(),
        "the original rank-4 export hits budget_verification_level before counter checks"
    );
    assert!(
        document["budget_state"]["revocation_sequence"]
            .as_u64()
            .expect("counter")
            > height
    );
    let owner = case.did();
    let head = case.endpoint.block_number().expect("current head");
    let now = case
        .endpoint
        .block_by_number(head)
        .expect("current block")
        .expect("block")
        .timestamp
        .checked_mul(1_000)
        .expect("milliseconds");
    let trust = BudgetProofTrust {
        tenant: case.value["tenant"].as_str().expect("tenant"),
        principal: case.value["principal"].as_str().expect("principal"),
        owner: &owner,
        protocol_version: 3,
        network_id: case.value["chain_id"].as_u64().expect("network") as u32,
        checkpoint: &checkpoint,
        endpoint: &case.endpoint,
        current_height: head,
        now_ms: now,
        maximum_age_seconds: 60,
    };
    let verified = verify_budget_proof(&export, case.id("budget_id"), &trust)
        .expect("counter above height must verify");
    assert_eq!(verified.canonical_export_bytes(), export);
    let mut altered = document;
    altered["budget_state"]["revocation_sequence"] = serde_json::json!(height);
    assert!(verify_budget_proof(
        &serde_json::to_vec(&altered).expect("JSON"),
        case.id("budget_id"),
        &trust
    )
    .is_err());
}

fn native_carrier_payload(
    disclosure: &layerx_crypto::disclosure::Disclosure,
    actor: &layerx_types::ids::Did,
) -> Result<Vec<u8>, NativeBindingError> {
    use layerx_crypto::disclosure::DisclosedNativeOperation;
    Ok(match &disclosure.native_operation {
        Some(DisclosedNativeOperation::ProgramCall(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        Some(DisclosedNativeOperation::ProgramDeploy(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        Some(DisclosedNativeOperation::ProgramUpgrade(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        Some(DisclosedNativeOperation::ProgramWindDown(value)) => value
            .native()
            .encode()
            .map_err(|_| NativeBindingError::Preparation)?,
        None => disclosure
            .payment
            .as_ref()
            .ok_or(NativeBindingError::Preparation)?
            .encode(actor.as_bytes())
            .map_err(|_| NativeBindingError::Preparation)?,
        _ => return Err(NativeBindingError::Preparation),
    })
}

fn native_carrier_authority(
    actor: &layerx_types::ids::Did,
) -> Result<layerx_types::activity::Authority, NativeBindingError> {
    let actor_text =
        std::str::from_utf8(actor.as_bytes()).map_err(|_| NativeBindingError::Preparation)?;
    let public = actor_text
        .strip_prefix("did:layerx:")
        .and_then(|text| layerx_programs::hex::decode(text).ok())
        .filter(|bytes| bytes.len() == 32)
        .ok_or(NativeBindingError::Preparation)?;
    layerx_types::activity::Authority::owner(&public).map_err(|_| NativeBindingError::Preparation)
}

fn native_call_preparation(
    canonical_bytes: &[u8],
    sender: [u8; 20],
    observed_height: u64,
) -> Result<EvmPrepared, NativeBindingError> {
    let call = layerx_crypto::evm_transaction::decode_unsigned(canonical_bytes)
        .map_err(|_| NativeBindingError::Preparation)?;
    let disclosure = layerx_crypto::disclosure::call::bind_call(&call, sender)
        .map_err(|_| NativeBindingError::Preparation)?;
    let prepared = EvmPrepared {
        from: sender,
        signing_hash: call.signing_hash(),
        disclosure_digest: disclosure
            .signing_digest()
            .map_err(|_| NativeBindingError::Preparation)?,
        disclosure,
        call,
        observed_head_height: observed_height,
    };
    layerx_agentd::prepare::verify_call_binding(&prepared)
        .map_err(|_| NativeBindingError::Preparation)?;
    Ok(prepared)
}

fn native_call_carrier(
    canonical_bytes: &[u8],
    sender: [u8; 20],
    observed_height: u64,
    actor: layerx_types::ids::Did,
    registry: &layerx_types::payload::ModuleRegistry,
) -> Result<layerx_agentd::prepare::Prepared, NativeBindingError> {
    use layerx_crypto::disclosure::call::bind_call;
    use layerx_types::activity::{EnvelopeBuilder, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::IdempotencyKey;
    use layerx_types::payload::Payload;
    let call = layerx_crypto::evm_transaction::decode_unsigned(canonical_bytes)
        .map_err(|_| NativeBindingError::Preparation)?;
    let disclosure = bind_call(&call, sender).map_err(|_| NativeBindingError::Preparation)?;
    let payload = native_carrier_payload(&disclosure, &actor)?;
    let authority = native_carrier_authority(&actor)?;
    let payload = Payload::new(registry, disclosure.activity_type, &payload)
        .map_err(|_| NativeBindingError::Preparation)?;
    let payload_hash = layerx_wire::hash::payload_hash_for(&payload)
        .map_err(|_| NativeBindingError::Preparation)?;
    let bound = TimestampBound::new(disclosure.expiry.not_before, disclosure.expiry.not_after)
        .map_err(|_| NativeBindingError::Preparation)?;
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(3)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .network_id(u32::try_from(call.chain_id).map_err(|_| NativeBindingError::Preparation)?)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .activity_type(disclosure.activity_type)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .actor_did(actor)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .authority(authority)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .account_sequence(call.nonce)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .timestamp_bound(bound)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .idempotency_key(IdempotencyKey::new(disclosure.idempotency_key))
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .fee_limit(Amount::from_u128(disclosure.fee_limit))
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .payload_hash(payload_hash)
        .map_err(|_| NativeBindingError::Preparation)?;
    builder
        .payload(payload)
        .map_err(|_| NativeBindingError::Preparation)?;
    let digest = layerx_agentd::prepare::DisclosureDigest(
        disclosure
            .audit_digest()
            .map_err(|_| NativeBindingError::Preparation)?,
    );
    let result = layerx_agentd::prepare::Prepared {
        envelope: builder
            .build()
            .map_err(|_| NativeBindingError::Preparation)?,
        canonical_bytes: canonical_bytes.to_vec(),
        signing_preimage: call.signing_hash(),
        observed_head_sequence: observed_height,
        audit: layerx_agentd::prepare::PreparationAuditEntry {
            idempotency_key: disclosure.idempotency_key,
            observed_head_sequence: observed_height,
            disclosure_digest: digest,
        },
        disclosure,
        disclosure_digest: digest,
    };
    let signing = SigningPreparation {
        actor_address: sender,
        transaction: EvmPrepared {
            from: sender,
            signing_hash: call.signing_hash(),
            disclosure_digest: result
                .disclosure
                .signing_digest()
                .map_err(|_| NativeBindingError::Preparation)?,
            disclosure: result.disclosure.clone(),
            call,
            observed_head_height: observed_height,
        },
    };
    validate_retained_preparation(&result, Some(&signing))?;
    Ok(result)
}

fn signing_preparation(case: &Case) -> SigningPreparation {
    let (transaction, _, _) = case.transaction();
    SigningPreparation {
        actor_address: transaction.from,
        transaction,
    }
}

fn restored(
    case: &Case,
) -> (
    layerx_agentd::prepare::Prepared,
    VerifiedSubmission,
    ModuleRegistry,
) {
    let (native, submission, registry) = case.transaction();
    let prepared = native_call_carrier(
        &native.call.encode_unsigned(),
        native.from,
        native.observed_head_height,
        case.did(),
        &registry,
    )
    .expect("native bytes and disclosure restore into budget carrier");
    assert_eq!(
        validate_retained_preparation(&prepared, Some(&signing_preparation(case)))
            .expect("retained native binding"),
        native.call
    );
    (prepared, submission, registry)
}

fn account_proofs(
    case: &Case,
    authorization: &SequencerAuthorization,
) -> BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState> {
    case.value["account_proofs"]
        .as_array()
        .expect("real account-status proofs")
        .iter()
        .map(|value| {
            let read = |name: &str| {
                std::fs::read(case.root.join(value[name].as_str().expect(name))).expect(name)
            };
            let proof = layerx_proof::state::NestedAccountProof {
                account_id: fixed(&value["account_id"]),
                account_root: fixed(&value["account_root"]),
                universal_root: fixed(&value["universal_root"]),
                resulting_state_root: fixed(&value["state_root"]),
                account_proof: layerx_proof::merkle::decode_proof(&read("account_proof"))
                    .expect("account proof"),
                account_tree_proof: layerx_proof::merkle::decode_proof(&read("account_tree_proof"))
                    .expect("account tree proof"),
                universal_root_proof: layerx_proof::merkle::decode_proof(&read(
                    "universal_root_proof",
                ))
                .expect("universal proof"),
                receipt_bytes: read("receipt"),
                receipt_proof: layerx_proof::merkle::decode_proof(&read("receipt_proof"))
                    .expect("receipt proof"),
                header_bytes: read("header"),
                header_signature: read("header_signature").try_into().expect("signature"),
            };
            let account = layerx_proof::state::verify_nested_account(
                &read("account_value"),
                proof.account_id,
                Some(fixed(&value["asset"])),
                &proof,
                authorization,
            )
            .expect("authoritative account status must verify");
            (proof.account_id, account)
        })
        .collect()
}

fn lifecycle(name: &str) -> Vec<program_settlement::ProgramExecutedDebit> {
    use native_settlement::{
        committed_native_execution_state_with_account_proofs,
        read_retained_native_call_debit_settlement,
    };
    let case = Case::open_execution(name);
    let (native, _, _) = case.transaction();
    let (prepared, submission, registry) = restored(&case);
    let payload = if let Some(payload) = case.value.get("generated_payload") {
        fixed_bytes(payload)
    } else {
        let capture: Value = serde_json::from_slice(
            &std::fs::read(case.root.join(case.value["capture"].as_str().unwrap())).unwrap(),
        )
        .unwrap();
        fixed_bytes(&capture["payload_hex"])
    };
    verify_native_lifecycle_binding(
        submission.exact_bytes(),
        prepared.disclosure.activity_type.ordinal(),
        &payload,
    )
    .expect("native lifecycle call and disclosed values");
    let proofs = &case.native_capture.as_ref().unwrap().proofs;
    let caps = case.caps(8);
    let sources = read_native_lifecycle_budget_sources(
        &mut case.node(),
        &prepared,
        Some(&signing_preparation(&case)),
        &caps,
        proofs,
    )
    .expect("real native lifecycle preparation source selection");
    let path = native_capture::retain(&case, &sources, &native, &submission, &registry);
    native_capture::execute(&case);
    let accounts = case.value["accounts"]
        .as_array()
        .unwrap()
        .iter()
        .map(fixed)
        .collect::<Vec<_>>();
    if name == "wind_down_exit" {
        assert!(
            committed_native_execution_state_with_account_proofs(
                &mut case.node(),
                &case.endpoint,
                &prepared,
                &submission,
                &case.caps(9),
                case.id("program"),
                &accounts,
                &BTreeMap::new()
            )
            .is_err(),
            "value account existence alone must not establish frozen status"
        );
    }
    let state = committed_native_execution_state_with_account_proofs(
        &mut case.node(),
        &case.endpoint,
        &prepared,
        &submission,
        &case.caps(9),
        case.id("program"),
        &accounts,
        proofs,
    )
    .expect("real receipt-bound prestate and authoritative account status");
    let settle = |store: &Store| {
        read_retained_native_call_debit_settlement(
            &case.endpoint,
            &prepared,
            &submission,
            store,
            &native_capture::peer(&case),
            Some(&state),
        )
        .expect("native lifecycle settlement")
    };
    let store = Store::open_read_only(&path).expect("real retained daemon store");
    let (reservation, before) = settle(&store);
    assert_eq!(
        reservation.id,
        <[u8; 32]>::from(Sha256::digest(&prepared.canonical_bytes))
    );
    assert_eq!(before.activity_id(), submission.activity_id());
    assert_eq!(
        before.terminal_receipt(),
        <[u8; 32]>::from(Sha256::digest(
            &case.native_capture.as_ref().unwrap().receipt
        ))
    );
    let debits = before.debits().to_vec();
    assert!(debits
        .iter()
        .any(|row| row.kind == ProgramChargeKind::Fee && row.actual_amount == 100_000));
    drop(store);
    let reopened = Store::open_read_only(&path).expect("reopen consumer after restart");
    assert_eq!(settle(&reopened).1.debits(), debits);
    drop(reopened);
    std::fs::remove_dir_all(path).unwrap();
    debits
}

#[test]
fn repair_execution_account_exists_funded_unfrozen_wind_down() {
    let case = Case::open_execution("wind_down_exit");
    let mut data = transaction_hash(b"getValueAccount(bytes32,bytes32)")[..4].to_vec();
    data.extend(case.id("program"));
    data.extend(case.id("source_account"));
    let bytes = case
        .endpoint
        .call_at(
            case.value["prestate_height"]
                .as_u64()
                .expect("receipt prestate"),
            &PrecompileTarget::Programs.address(),
            &data,
        )
        .expect("real programs precompile");
    let observation = decode_program_value_observation(&bytes).expect("(balance, exists) ABI");
    assert!(observation.exists && observation.balance > 0);
    let debits = lifecycle("wind_down_exit");
    assert!(debits
        .iter()
        .any(|debit| debit.kind == ProgramChargeKind::ProgramSpend
            && debit.source == case.id("source_account")
            && debit.actual_amount == observation.balance));
}

#[test]
fn repair_fee_payer_denom_selects_two_real_preferences_and_reservations() {
    let mut assets = Vec::new();
    for name in ["fee_payer_a", "fee_payer_b"] {
        let case = Case::open(name);
        let (prepared, _, registry) = restored(&case);
        let payer: [u8; 20] = prepared
            .disclosure
            .actor
            .as_slice()
            .try_into()
            .expect("native payer");
        let fee = fee_policy::native_fee_policy_for(
            &case.endpoint,
            prepared.observed_head_sequence,
            payer,
        )
        .expect("real payer preference");
        assert_eq!(fee.value.asset.asset_id, case.id("fee_asset"));
        assets.push(fee.value.asset.asset_id);
        let caps = case.caps(prepared.observed_head_sequence);
        assert_fee_snapshot(&case, prepared.observed_head_sequence);
        let sources = read_native_effect_budget_sources(
            &mut case.node(),
            &prepared,
        Some(&signing_preparation(&case)),
            &caps,
            case.value["now_ms"].as_u64().expect("core time"),
            1,
            layerx_agentd::capability::NativeAdmissionProfile::Effect,
        )
        .unwrap_or_else(|error| {
            panic!("program_sources::read_native_effect_budget_sources refused {error:?}; fee snapshot passed, caps.level={:?}, required={:?}",
                caps.level(), layerx_types::verify::VerificationLevel::STATE_PROVEN)
        });
        assert!(sources
            .charges()
            .iter()
            .any(|charge| charge.source.kind() == ProgramChargeKind::Fee
                && charge.asset == fee.value.asset.asset_id));
        assert!(sources.matches_prepared(&signing_preparation(&case).transaction));
        let reservation = case.reservation(&sources);
        assert_eq!(
            reservation.id,
            <[u8; 32]>::from(Sha256::digest(&prepared.canonical_bytes))
        );
        assert!(reservation
            .charges
            .iter()
            .any(|charge| charge.kind == ProgramChargeKind::Fee
                && charge.asset == fee.value.asset.asset_id
                && charge.amount == prepared.disclosure.fee_limit));
        let _ = registry;
    }
    assert_ne!(assets[0], assets[1]);
}

#[test]
fn repair_lifecycle_native_envelope_deploy_upgrade_wind_down_and_restart() {
    for name in ["deploy", "upgrade", "wind_down", "wind_down_exit"] {
        lifecycle(name);
    }
}

#[test]
fn repair_settlement_native_envelope_program_receipt_and_restart() {
    let case = Case::open("program_settlement");
    let (prepared, submission, registry) = restored(&case);
    layerx_agentd::capability::derive_effects(
        &prepared.disclosure,
        &layerx_agentd::capability::VerifiedInputs::default(),
    )
    .expect("the production semantic planner admits the supported native call");
    assert!(
        case.value.get("receipt").is_none(),
        "the capture makes no execution claim"
    );
    assert_eq!(
        program_settlement::read_native_execution_observation(
            &case.endpoint,
            &prepared,
            &submission
        ),
        Err(program_settlement::ProgramSettlementError::Receipt),
        "an unknown native call must not settle or release its reservation",
    );
    let root = temporary("no-receipt");
    let key = TenantKey::new(
        case.tenant(),
        ObjectKind::PreparedActivity,
        submission.activity_id().to_vec(),
    )
    .expect("retained native transaction key");
    let mut store = Store::open(&root).expect("real store");
    store
        .put_local(key.clone(), submission.exact_bytes().to_vec())
        .expect("retain unknown transaction");
    drop(store);
    let reopened = Store::open(&root).expect("restart native execution consumer");
    let retained = reopened
        .get(&key)
        .expect("unknown transaction remains retained");
    assert_eq!(retained.bytes(), submission.exact_bytes());
    let native = native_call_carrier(
        &prepared.canonical_bytes,
        submission.recovered_sender(),
        prepared.observed_head_sequence,
        case.did(),
        &registry,
    )
    .expect("real native restoration after reopening store");
    assert_eq!(
        program_settlement::read_native_execution_observation(&case.endpoint, &native, &submission),
        Err(program_settlement::ProgramSettlementError::Receipt),
    );
    assert_eq!(reopened.get(&key).unwrap().bytes(), retained.bytes());
    drop(reopened);
    std::fs::remove_dir_all(root).expect("remove isolated store");
}

#[test]
fn repair_sources_native_envelope_allocates_and_restores_real_preparation() {
    let case = Case::open("native_effect_sources");
    let (prepared, _, registry) = restored(&case);
    let caps = case.caps(prepared.observed_head_sequence);
    assert_fee_snapshot(&case, prepared.observed_head_sequence);
    let sources = read_native_effect_budget_sources(
        &mut case.node(),
        &prepared,
        Some(&signing_preparation(&case)),
        &caps,
        case.value["now_ms"].as_u64().expect("core time"),
        1,
        layerx_agentd::capability::NativeAdmissionProfile::Effect,
    )
    .unwrap_or_else(|error| {
        panic!("program_sources::read_native_effect_budget_sources refused {error:?}; fee snapshot passed, caps.level={:?}, required={:?}",
            caps.level(), layerx_types::verify::VerificationLevel::STATE_PROVEN)
    });
    let limit = daemon_limit_id(enrolment_limit_id(
        &case.tenant(),
        &LimitScope::Tenant(sources.actor()),
        &LimitId(fixed(&case.value["limit_id"])),
    ));
    let allocations = sources
        .budget_allocations(|_| Ok(vec![limit]))
        .expect("real allocation producer");
    let burn_destination = layerx_wire::hash::asset_issuance_account_id(&case.id("fee_asset"))
        .expect("production burn returns supply to the issuance account");
    assert!(allocations
        .allocations()
        .iter()
        .any(|row| row.kind == ProgramChargeKind::Principal
            && row.maximum_amount == 7
            && row.destination == Some(burn_destination)));
    let reservation = case.reservation(&sources);
    assert_eq!(
        reservation.allocations().expect("retained allocation"),
        allocations.allocations()
    );
    assert_eq!(reservation.charges, allocations.charges());
    let root = temporary("sources");
    let key = TenantKey::new(
        case.tenant(),
        ObjectKind::PreparedActivity,
        reservation.id.to_vec(),
    )
    .expect("native preparation key");
    let mut store = Store::open(&root).expect("real store");
    store
        .put_local(key.clone(), prepared.canonical_bytes.clone())
        .expect("persist real producer bytes");
    drop(store);
    let reopened = Store::open(&root).expect("restart consumer");
    let sender = prepared
        .disclosure
        .actor
        .as_slice()
        .try_into()
        .expect("native sender");
    let recovered = native_call_carrier(
        reopened.get(&key).expect("retained native bytes").bytes(),
        sender,
        prepared.observed_head_sequence,
        case.did(),
        &registry,
    )
    .expect("native durable restoration");
    assert_eq!(recovered.canonical_bytes, prepared.canonical_bytes);
    assert_eq!(recovered.disclosure_digest, prepared.disclosure_digest);
    let recovered_call = native_call_preparation(
        &recovered.canonical_bytes,
        sender,
        recovered.observed_head_sequence,
    )
    .expect("rebind retained call after reopening store");
    assert!(sources.matches_prepared(&recovered_call));
    let mut substituted = recovered_call;
    substituted.call.data.push(0);
    assert!(!sources.matches_prepared(&substituted));
    drop(reopened);
    std::fs::remove_dir_all(root).expect("remove isolated store");
}

#[test]
fn repair_settlement_native_envelope_effect_receipt_and_restart() {
    let case = Case::open_execution("native_effect_settlement");
    let (native, _, _) = case.transaction();
    let (prepared, submission, registry) = restored(&case);
    let caps = case.caps(prepared.observed_head_sequence);
    assert_fee_snapshot(&case, prepared.observed_head_sequence);
    let sources = read_native_effect_budget_sources(
        &mut case.node(),
        &prepared,
        Some(&signing_preparation(&case)),
        &caps,
        case.value["now_ms"].as_u64().unwrap(),
        1,
        layerx_agentd::capability::NativeAdmissionProfile::Effect,
    )
    .expect("real burn preparation sources");
    let path = native_capture::retain(&case, &sources, &native, &submission, &registry);
    native_capture::execute(&case);
    let settle = |store: &Store| {
        native_settlement::read_retained_native_call_debit_settlement(
            &case.endpoint,
            &prepared,
            &submission,
            store,
            &native_capture::peer(&case),
            None,
        )
        .expect("real native-effect settlement")
    };
    let store = Store::open_read_only(&path).expect("real producer store");
    let (reservation, before) = settle(&store);
    assert_eq!(
        reservation.id,
        <[u8; 32]>::from(Sha256::digest(&prepared.canonical_bytes))
    );
    let debits = before.debits().to_vec();
    assert!(!debits.is_empty());
    assert!(debits
        .iter()
        .any(|debit| debit.kind == ProgramChargeKind::Principal && debit.actual_amount == 7));
    assert!(debits
        .iter()
        .any(|debit| debit.kind == ProgramChargeKind::Fee && debit.actual_amount == 100_000));
    drop(store);
    let reopened = Store::open_read_only(&path).expect("restart consumer");
    assert_eq!(settle(&reopened).1.debits(), debits);
    let mut altered = prepared.clone();
    altered.canonical_bytes.push(0);
    assert_eq!(
        native_settlement::read_retained_native_call_debit_settlement(
            &case.endpoint,
            &altered,
            &submission,
            &reopened,
            &native_capture::peer(&case),
            None
        ),
        Err(program_settlement::ProgramSettlementError::Preparation)
    );
    let original = {
        let mut answers = case.replay.answers.lock().unwrap();
        let row = answers
            .iter_mut()
            .find(|v| v["method"] == "pax_sequencerReceipt")
            .unwrap();
        let original = row.clone();
        row["result"]["logs"][0]["data"] = json!(format!(
            "0x{}",
            hex::encode([number(8), number(11)].concat())
        ));
        original
    };
    assert!(
        native_settlement::read_retained_native_call_debit_settlement(
            &case.endpoint,
            &prepared,
            &submission,
            &reopened,
            &native_capture::peer(&case),
            None
        )
        .is_err(),
        "the signed receipt must reject substituted precompile log bytes"
    );
    *case
        .replay
        .answers
        .lock()
        .unwrap()
        .iter_mut()
        .find(|v| v["method"] == "pax_sequencerReceipt")
        .unwrap() = original;
    drop(reopened);
    std::fs::remove_dir_all(path).unwrap();
}

fn signed_native_call(call: &PrecompileCall) -> Vec<u8> {
    Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: call.target.address(),
        data: call.encode().expect("production ABI encoder"),
    }
    .sign(&[0xa5; 32])
    .expect("production EIP-1559 signer")
}

#[test]
fn repair_create_native_source_nonce_round_trips_signed_codec_and_rejects_maximum() {
    let (request, mut identity) = create_request();
    let registry = registry(ModuleId::Budget, 1);
    for nonce in [0, 1_000, u64::MAX - 1] {
        identity.source_sequence = nonce;
        let call = request
            .create_protocol_budget(&identity)
            .expect("source nonce");
        let signed = signed_native_call(&call);
        let decoded =
            budget_create_identity(signed.as_slice(), &registry).expect("native consumer");
        assert_eq!(decoded.source_sequence, nonce);
        assert_eq!(decoded.source_account, identity.source_account);
        let (transaction, signature) = decode_signed(&signed).expect("signed codec");
        assert_eq!(
            decoded.sender,
            recover_sender(&transaction, &signature).expect("authenticated sender")
        );
        assert_eq!(transaction.nonce, 0);
        assert_eq!(transaction.data, call.encode().expect("producer bytes"));
    }
    identity.source_sequence = u64::MAX;
    assert_eq!(
        request.create_protocol_budget(&identity),
        Err(BudgetCreationError::InvalidLimit)
    );
    identity.source_sequence = 0;
    let mut reserved = request
        .create_protocol_budget(&identity)
        .expect("valid source");
    reserved.arguments[13] = AbiArgument::Word(number(u64::MAX.into()));
    assert_eq!(
        budget_create_identity(signed_native_call(&reserved).as_slice(), &registry),
        Err(BudgetCreationError::InvalidLimit)
    );
    for source in [[0; 32], identity.budget_account] {
        let mut invalid = request
            .create_protocol_budget(&identity)
            .expect("valid source");
        invalid.arguments[12] = AbiArgument::Word(source);
        assert_eq!(
            budget_create_identity(signed_native_call(&invalid).as_slice(), &registry),
            Err(BudgetCreationError::InvalidLimit)
        );
    }
}

#[test]
fn repair_fund_and_revoke_signed_codecs_preserve_nonce_and_counter_units() {
    let context = layerx_crypto::disclosure::BudgetStateContext {
        budget_id: [1; 32],
        owner: [2; 32],
        budget_account: [3; 32],
        asset: [4; 32],
        source_account: [5; 32],
        native_source: true,
        revocation_sequence: 900,
        balance: 50,
        state_digest: [6; 32],
        observed_head_sequence: 7,
        purpose_hash: [8; 32],
    };
    let registry = registry(ModuleId::Budget, 2);
    for nonce in [0, 1_000, u64::MAX - 1] {
        let call = PrecompileCall {
            target: PrecompileTarget::Pay,
            selector: transaction_hash(b"budgetFundNative(bytes32,uint128,uint64)")[..4]
                .try_into()
                .expect("selector"),
            arguments: vec![
                AbiArgument::Word(context.budget_id),
                AbiArgument::Word(number(10)),
                AbiArgument::Word(number(nonce.into())),
            ],
        };
        let signed = signed_native_call(&call);
        let mutation = budget_mutation_identity(&signed, &registry, &context)
            .expect("native mutation consumer");
        assert_eq!(mutation.source_sequence(), Some(nonce));
        assert_eq!(mutation.revocation_sequence(), None);
        let mut reserved = call;
        reserved.arguments[2] = AbiArgument::Word(number(u64::MAX.into()));
        assert_eq!(
            budget_mutation_identity(&signed_native_call(&reserved), &registry, &context),
            Err(BudgetCreationError::ContextMismatch)
        );
    }
    let mut revoke = PrecompileCall {
        target: PrecompileTarget::Pay,
        selector: transaction_hash(b"budgetRevoke(bytes32,uint64)")[..4]
            .try_into()
            .expect("selector"),
        arguments: vec![
            AbiArgument::Word(context.budget_id),
            AbiArgument::Word(number(1_000)),
        ],
    };
    let mutation = budget_mutation_identity(&signed_native_call(&revoke), &registry, &context)
        .expect("monotonic counter above height");
    assert_eq!(mutation.revocation_sequence(), Some(1_000));
    assert_eq!(mutation.source_sequence(), None);
    for stale in [0, 899, 900] {
        revoke.arguments[1] = AbiArgument::Word(number(stale));
        assert_eq!(
            budget_mutation_identity(&signed_native_call(&revoke), &registry, &context),
            Err(BudgetCreationError::StaleRevocation)
        );
    }
}

#[test]
fn repair_native_carrier_validates_real_preparation_and_restores_after_store_restart() {
    let construct = PrecompileCall {
        target: PrecompileTarget::Ledger,
        selector: transaction_hash(b"openAccount(bytes32)")[..4]
            .try_into()
            .expect("selector"),
        arguments: vec![AbiArgument::Word([3; 32])],
    };
    let signed = signed_native_call(&construct);
    let (call, signature) = decode_signed(&signed).expect("real signed producer bytes");
    let sender = recover_sender(&call, &signature).expect("native sender");
    let prepared = prepare_as(construct, sender);
    assert_eq!(prepared.call, call);
    let registry = registry(ModuleId::Asset, 4);
    let submission = verify_before_submit(
        &signed,
        &SigningPreparation {
            transaction: prepared.clone(),
            actor_address: sender,
        },
        &sender,
        &registry,
    )
    .expect("real signing gate");
    assert_eq!(
        verify_native_budget_submission(&prepared, &submission).expect("native budget binding"),
        call
    );
    let actor =
        Did::new(format!("did:layerx:{}", hex::encode([0x0a; 32])).as_bytes()).expect("DID");
    let signing = SigningPreparation {
        actor_address: sender,
        transaction: prepared.clone(),
    };
    let retained =
        native_call_carrier(&call.encode_unsigned(), sender, 7, actor.clone(), &registry)
            .expect("native restoration");
    assert_eq!(
        validate_retained_preparation(&retained, Some(&signing)),
        Ok(call.clone())
    );
    let root = temporary("native-codec-restart");
    let key = TenantKey::new(
        TenantId::new("native-codec").expect("tenant"),
        ObjectKind::PreparedActivity,
        Sha256::digest(&retained.canonical_bytes).to_vec(),
    )
    .expect("key");
    let mut store = Store::open(&root).expect("production store");
    store
        .put_local(key.clone(), retained.canonical_bytes.clone())
        .expect("persist producer bytes");
    drop(store);
    let reopened = Store::open(&root).expect("reopen component");
    let recovered = native_call_carrier(
        reopened.get(&key).expect("durable bytes").bytes(),
        sender,
        7,
        actor.clone(),
        &registry,
    )
    .expect("restore after restart");
    assert_eq!(recovered.canonical_bytes, retained.canonical_bytes);
    assert_eq!(recovered.disclosure, retained.disclosure);
    assert_eq!(recovered.disclosure_digest, retained.disclosure_digest);
    assert_eq!(
        validate_retained_preparation(&recovered, Some(&signing)),
        Ok(call.clone())
    );
    let mut other_call = call;
    other_call.data = PrecompileCall {
        target: PrecompileTarget::Ledger,
        selector: transaction_hash(b"openAccount(bytes32)")[..4]
            .try_into()
            .expect("selector"),
        arguments: vec![AbiArgument::Word([9; 32])],
    }
    .encode()
    .expect("second real ABI producer");
    let other = native_call_carrier(&other_call.encode_unsigned(), sender, 7, actor, &registry)
        .expect("second real carrier");
    let mut substituted_payload = recovered.clone();
    substituted_payload.envelope = other.envelope;
    assert!(
        validate_retained_preparation(&substituted_payload, Some(&signing)).is_err(),
        "reject a carrier payload from a different native call"
    );
    for field in 0..7 {
        let mut altered = recovered.clone();
        match field {
            0 => altered.canonical_bytes.push(0),
            1 => altered.signing_preimage[0] ^= 1,
            2 => altered.audit.idempotency_key[0] ^= 1,
            3 => altered.audit.disclosure_digest.0[0] ^= 1,
            4 => altered.disclosure_digest.0[0] ^= 1,
            5 => altered.observed_head_sequence += 1,
            6 => altered.disclosure.actor[0] ^= 1,
            _ => unreachable!(),
        }
        assert!(
            validate_retained_preparation(&altered, Some(&signing)).is_err(),
            "reject changed field {field}"
        );
    }
    drop(reopened);
    std::fs::remove_dir_all(root).expect("remove isolated store");
}

#[test]
fn repair_wire_carrier_binds_the_retained_signing_preparation_after_restart() {
    let case = Case::open("native_effect_sources");
    let (call_carrier, _, registry) = restored(&case);
    let canonical = layerx_wire::activity::encode_unsigned_envelope(&call_carrier.envelope)
        .expect("real unsigned wire envelope producer");
    let authority = call_carrier.envelope.authority();
    let prepared = layerx_agentd::prepare::restore_canonical(
        &canonical,
        call_carrier.observed_head_sequence,
        &registry,
        Some(authority),
    )
    .expect("approval-path wire restoration");
    assert_eq!(prepared.canonical_bytes, canonical);
    assert_ne!(prepared.canonical_bytes, call_carrier.canonical_bytes);
    let signing = signing_preparation(&case);
    let produced = layerx_agentd::budget::native_binding::restore_native_preparation(
        &signing.transaction.call.encode_unsigned(),
        signing.actor_address,
        signing.transaction.observed_head_height,
        case.did(),
        &registry,
    )
    .expect("native-to-wire approval carrier producer");
    assert_eq!(produced, prepared);
    let root = temporary("wire-signing-binding");
    let tenant = case.tenant();
    let id = Sha256::digest(&canonical).into();
    let mut store = Store::open(&root).expect("real preparation store");
    signing
        .retain(&mut store, tenant.clone(), id)
        .expect("retain signing preparation under carrier identity");
    drop(store);
    let reopened = Store::open(&root).expect("restart signing preparation consumer");
    let retained = SigningPreparation::restore(&reopened, tenant.clone(), id)
        .expect("restore same retained signing preparation");
    assert_eq!(
        validate_retained_preparation(&prepared, Some(&retained)),
        Ok(signing.transaction.call.clone())
    );
    assert_eq!(
        validate_retained_preparation(&prepared, None),
        Err(NativeBindingError::Preparation)
    );
    assert!(SigningPreparation::restore(&reopened, tenant, [0xff; 32]).is_err());
    for field in 0..5 {
        let mut disagrees = retained.clone();
        match field {
            0 => disagrees.transaction.disclosure_digest[0] ^= 1,
            1 => disagrees.transaction.signing_hash[0] ^= 1,
            2 => disagrees.actor_address[0] ^= 1,
            3 => disagrees.transaction.observed_head_height += 1,
            4 => disagrees.transaction.call.nonce += 1,
            _ => unreachable!(),
        }
        assert_eq!(
            validate_retained_preparation(&prepared, Some(&disagrees)),
            Err(NativeBindingError::Preparation),
            "refuse changed signing field {field}"
        );
    }
    let mut substituted = retained.clone();
    substituted.transaction.call.nonce += 1;
    substituted.transaction.signing_hash = substituted.transaction.call.signing_hash();
    let disclosed = layerx_agentd::prepare::disclose::bind(
        &substituted.transaction.call,
        substituted.actor_address,
    )
    .expect("valid disclosure of a different transaction");
    substituted.transaction.disclosure = disclosed.disclosure;
    substituted.transaction.disclosure_digest = disclosed.digest;
    validate_native_preparation(&substituted.transaction)
        .expect("independently valid substituted preparation");
    assert_eq!(
        validate_retained_preparation(&prepared, Some(&substituted)),
        Err(NativeBindingError::Preparation)
    );
    for field in 0..5 {
        let mut altered = prepared.clone();
        match field {
            0 => altered.disclosure_digest.0[0] ^= 1,
            1 => altered.signing_preimage[0] ^= 1,
            2 => altered.audit.disclosure_digest.0[0] ^= 1,
            3 => altered.canonical_bytes.push(0),
            4 => {
                let other = native_call_carrier(
                    &retained.transaction.call.encode_unsigned(),
                    retained.actor_address,
                    retained.transaction.observed_head_height,
                    Did::new(format!("did:layerx:{}", hex::encode([0x7b; 32])).as_bytes())
                        .expect("other owner DID"),
                    &registry,
                )
                .expect("other owner's envelope producer");
                altered.envelope = other.envelope;
            }
            _ => unreachable!(),
        }
        assert_eq!(
            validate_retained_preparation(&altered, Some(&retained)),
            Err(NativeBindingError::Preparation),
            "refuse changed wire carrier field {field}"
        );
    }
    let caps = case.caps(prepared.observed_head_sequence);
    let sources = read_native_effect_budget_sources(
        &mut case.node(),
        &prepared,
        Some(&retained),
        &caps,
        case.value["now_ms"].as_u64().expect("core time"),
        1,
        layerx_agentd::capability::NativeAdmissionProfile::Effect,
    )
    .expect("wire carrier reaches production source selection through retained signer");
    assert!(sources.matches_prepared(&retained.transaction));
    drop(reopened);
    std::fs::remove_dir_all(root).expect("remove isolated preparation store");
}

#[test]
fn repair_recorded_malformed_payload_refusal_is_explicit() {
    // This records the shared producer refusal without turning the mandatory
    // first-use success test into an expected failure.
    let case = Case::open("create_source_nonce");
    let signed = case.bytes("signed_transaction");
    let (call, signature) = decode_signed(&signed).expect("real native producer bytes");
    assert_native_call_limits(&call);
    let sender = recover_sender(&call, &signature).expect("authenticated sender");
    let identity = budget_create_identity(signed.as_slice(), &registry(ModuleId::Budget, 1))
        .expect("the repaired budget consumer accepts nonce zero");
    assert_eq!(identity.source_sequence, 0);
    assert_eq!(
        prepare_bound(
            EvmPreparationState {
                chain_id: 125,
                pending_nonce: 0,
                base_fee_per_gas: 1,
                head_height: 8,
            },
            EvmPreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                maximum_payload_bytes: 1_048_576,
            },
            EvmPrepareRequest {
                from: sender,
                call: case.construct(),
                expected_nonce: Some(0),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 1_048_576,
            },
            100_000,
        ),
        Err(layerx_agentd::prepare::evm::PrepareError::Disclosure(
            layerx_crypto::disclosure::DisclosureError::MalformedPayload,
        ))
    );
    let mut used_source = call.clone();
    used_source.data[4 + 13 * 32..].copy_from_slice(&number(1));
    let bound = layerx_agentd::prepare::disclose::bind(&used_source, sender)
        .expect("the same native create call with source nonce one must bind");
    assert!(matches!(
        bound.disclosure.native_operation,
        Some(layerx_crypto::disclosure::DisclosedNativeOperation::BudgetCreate(ref budget))
            if budget.source_sequence == 1 && budget.source_account == identity.source_account
    ));
    assert_eq!(
        bound.digest,
        bound
            .disclosure
            .signing_digest()
            .expect("bound disclosure digest"),
    );
    for field in ["chain_id", "gas_limit", "priority_fee"] {
        let mut malformed = used_source.clone();
        match field {
            "chain_id" => malformed.chain_id = 0,
            "gas_limit" => malformed.gas_limit = 0,
            "priority_fee" => malformed.max_priority_fee_per_gas = malformed.max_fee_per_gas + 1,
            _ => unreachable!(),
        }
        assert_eq!(
            layerx_agentd::prepare::disclose::bind(&malformed, sender),
            Err(layerx_crypto::disclosure::DisclosureError::MalformedPayload),
            "prepare::disclose::bind must refuse malformed {field}",
        );
    }
}

#[test]
fn repair_native_source_nonce_authenticates_owner_and_checkpoint() {
    for (name, source, asset, nonce) in [
        ("create_source_nonce", [5; 32], [3; 32], 0),
        ("fund_source_nonce_zero", [8; 32], [4; 32], 0),
        ("fund_source_nonce_above_height", [8; 32], [4; 32], 1_000),
    ] {
        let case = Case::open(name);
        let signed = case.bytes("signed_transaction");
        let (call, signature) = decode_signed(&signed).expect("native signed call");
        let sender = recover_sender(&call, &signature).expect("real signature recovery");
        assert_eq!(
            validate_native_budget_source(&case.endpoint, 8, sender, source, asset, nonce),
            Ok(())
        );
        assert_eq!(
            validate_native_budget_source(&case.endpoint, 8, sender, source, asset, nonce + 1),
            Err(BudgetCreationError::ContextMismatch)
        );
        assert_eq!(
            validate_native_budget_source(&case.endpoint, 8, sender, source, asset, u64::MAX),
            Err(BudgetCreationError::InvalidLimit)
        );
        assert_eq!(
            validate_native_budget_source(&case.endpoint, 8, [0; 20], source, asset, nonce),
            Err(BudgetCreationError::InvalidLimit)
        );
    }
}

#[test]
fn repair_fee_payer_preferences_bind_metadata_and_durable_fee_holds() {
    let mut selected = Vec::new();
    for name in ["fee_payer_a", "fee_payer_b"] {
        let case = Case::open(name);
        let (prepared, _, _) = case.transaction();
        let fee = fee_policy::native_fee_policy_for(&case.endpoint, 8, prepared.from)
            .expect("production fee selection for the authenticated sender");
        assert_eq!(fee.value.asset.asset_id, case.id("fee_asset"));
        assert_eq!(fee.observed_sequence, 8);
        assert_eq!(fee.value.asset.decimals, 18);
        let caps = case.caps(8);
        assert_eq!(fee.state_root, caps.state_root());
        let source = caps
            .all_accounts()
            .values()
            .find(|account| account.asset_id() == fee.value.asset.asset_id)
            .expect("authenticated fee asset account");
        let root = temporary(name);
        let mut store = Store::open(&root).expect("production store");
        let limiter = BudgetLimiter::new(vec![]).expect("limiter");
        let id: [u8; 32] = Sha256::digest(prepared.call.encode_unsigned()).into();
        let limit = daemon_limit_id(id);
        create_daemon_limit(
            &mut store,
            &limiter,
            DaemonLimitRecord {
                tenant: case.tenant(),
                budget_id: id,
                limit_id: limit,
                agent_digest: caps.did(),
                asset: fee.value.asset.asset_id,
                ceiling: 1_000_000,
                consumed: 0,
                expiry_ms: 1_700_000_060_000,
                revoked: false,
                mutation_key: id,
                body_digest: Sha256::digest(prepared.call.encode_unsigned()).into(),
                revoke_key: [0; 32],
            },
            CoreTimestampMs(1_700_000_000_000),
        )
        .expect("fee-denominated limit");
        let request = ProgramReservationRequest {
            id,
            charges: vec![ProgramBudgetCharge {
                kind: ProgramChargeKind::Fee,
                source: source.account_id,
                asset: fee.value.asset.asset_id,
                amount: prepared.disclosure.fee_limit,
                applicable_limits: vec![limit],
            }],
            expiry_sequence: 20,
            current_sequence: 8,
            core_deadline: None,
        };
        let reservation = limiter
            .stage_program_reservation(&request, CoreTimestampMs(1_700_000_000_000))
            .expect("production denomination validation and fee hold")
            .publish();
        assert_eq!(reservation.charges, request.charges);
        let key = TenantKey::new(case.tenant(), ObjectKind::Budget, id.to_vec()).expect("hold key");
        store
            .put_local(
                key.clone(),
                reservation.encode().expect("reservation producer codec"),
            )
            .expect("persist fee hold");
        drop(store);
        let reopened = Store::open(&root).expect("reopen fee hold store");
        let recovered = ProgramBudgetReservation::decode(
            reopened.get(&key).expect("retained fee hold").bytes(),
        )
        .expect("production reservation decoder");
        assert_eq!(recovered, reservation);
        assert_eq!(recovered.charges[0].asset, fee.value.asset.asset_id);
        selected.push(fee.value.asset.asset_id);
        drop(reopened);
        std::fs::remove_dir_all(root).expect("remove isolated store");
    }
    assert_ne!(selected[0], selected[1]);
}

#[test]
fn repair_unsupported_binder_methods_preserve_real_native_lifecycle_binding() {
    for name in [
        "fund_source_nonce_zero",
        "fund_source_nonce_above_height",
        "revoke_counter",
        "deploy",
        "upgrade",
        "wind_down",
        "wind_down_exit",
    ] {
        let case = Case::open(name);
        assert_current_binder_result(&case);
        if case.value.get("capture").is_none() {
            continue;
        }
        let capture: Value = serde_json::from_slice(
            &std::fs::read(case.root.join(case.value["capture"].as_str().unwrap())).unwrap(),
        )
        .unwrap();
        let signed = case.bytes("signed_transaction");
        let payload = fixed_bytes(&capture["payload_hex"]);
        let ordinal = capture["ordinal"].as_u64().unwrap() as u16;
        let observation =
            program_settlement::read_signed_native_execution_observation(&case.endpoint, &signed)
                .expect("supported read-side native lifecycle execution observation");
        let SequencerReceipt::Receipted(executed) = observation else {
            panic!("native lifecycle peer must serve the Receipted rung");
        };
        assert_eq!(executed.tx_hash, transaction_hash(&signed));

        let bound = verify_native_lifecycle_binding(&signed, ordinal, &payload)
            .expect("production lifecycle consumer binds exact producer ABI and native semantics");
        assert_eq!(bound.0, transaction_hash(&signed));
        assert_eq!(
            case.construct().encode().unwrap(),
            fixed_bytes(&capture["evm"]["calldata"])
        );
        let root = temporary(name);
        let key = TenantKey::new(
            case.tenant(),
            ObjectKind::PreparedActivity,
            bound.0.to_vec(),
        )
        .unwrap();
        let mut store = Store::open(&root).unwrap();
        store.put_local(key.clone(), signed.clone()).unwrap();
        drop(store);
        let reopened = Store::open(&root).unwrap();
        assert_eq!(
            verify_native_lifecycle_binding(reopened.get(&key).unwrap().bytes(), ordinal, &payload),
            Ok(bound)
        );
        let mut altered = payload.clone();
        altered[0] ^= 1;
        assert!(verify_native_lifecycle_binding(&signed, ordinal, &altered).is_err());
        let (mut call, _) = decode_signed(&signed).unwrap();
        call.chain_id = 0;
        let invalid = call.sign(&fixed(&case.value["signing_seed"])).unwrap();
        assert!(verify_native_lifecycle_binding(&invalid, ordinal, &payload).is_err());
        drop(reopened);
        std::fs::remove_dir_all(root).unwrap();
    }
}

#[test]
fn repair_source_checkpoint_identifies_retired_caps_level() {
    let case = Case::open("native_effect_sources");
    let (prepared, _, _) = restored(&case);
    let caps = case.caps(prepared.observed_head_sequence);
    let freshness = caps.freshness();
    let actor = layerx_wire::hash::did_id_for_protocol(prepared.envelope.actor_did(), 3).unwrap();
    assert!(case.value["now_ms"].as_u64().unwrap() > 0);
    assert_eq!(caps.did(), actor);
    assert_ne!(caps.state_root(), [0; 32]);
    assert_eq!(freshness.global_sequence, prepared.observed_head_sequence);
    assert_eq!(
        freshness.observed_head_sequence,
        prepared.observed_head_sequence
    );
    assert_eq!(freshness.batch_number, caps.observed_height());
    assert_eq!(
        case.endpoint.block_number().unwrap(),
        prepared.observed_head_sequence
    );
    assert_eq!(
        case.endpoint.block_number().unwrap(),
        caps.observed_height()
    );
    assert!(
        caps.level().is_current(),
        "decision 2694: committed capabilities carry a current achievement"
    );
    assert_eq!(
        caps.level(),
        layerx_types::verify::VerificationLevel::COMMITTED
    );
    assert!(caps.level() >= layerx_types::verify::VerificationLevel::STATE_PROVEN);
    let result = read_native_effect_budget_sources(
        &mut case.node(),
        &prepared,
        Some(&signing_preparation(&case)),
        &caps,
        case.value["now_ms"].as_u64().unwrap(),
        1,
        layerx_agentd::capability::NativeAdmissionProfile::Effect,
    );
    result.expect("current committed checkpoint admits native budget sources");
    let mut wrong_call = case.construct();
    wrong_call.arguments[1] = AbiArgument::Word([6; 32]);
    let (native, _, registry) = case.transaction();
    let wrong_native = Eip1559Call {
        data: wrong_call.encode().expect("production ABI encoder"),
        ..native.call.clone()
    };
    let wrong_disclosure = layerx_agentd::prepare::disclose::bind(&wrong_native, native.from)
        .expect("real substituted call disclosure");
    let wrong_signing = SigningPreparation {
        actor_address: native.from,
        transaction: EvmPrepared {
            from: native.from,
            signing_hash: wrong_native.signing_hash(),
            disclosure_digest: wrong_disclosure.digest,
            disclosure: wrong_disclosure.disclosure,
            call: wrong_native.clone(),
            observed_head_height: native.observed_head_height,
        },
    };
    let wrong = native_call_carrier(
        &wrong_native.encode_unsigned(),
        native.from,
        native.observed_head_height,
        case.did(),
        &registry,
    )
    .expect("bound disclosure for substituted source account");
    assert_eq!(
        read_native_effect_budget_sources(
            &mut case.node(),
            &wrong,
            Some(&wrong_signing),
            &caps,
            case.value["now_ms"].as_u64().unwrap(),
            1,
            layerx_agentd::capability::NativeAdmissionProfile::Effect,
        ),
        Err(ProgramSourceError::SourceOwnership),
        "program_sources.rs destruction source must equal the authenticated inventory account"
    );

    let no_time = read_native_effect_budget_sources(
        &mut case.node(),
        &prepared,
        Some(&signing_preparation(&case)),
        &caps,
        0,
        1,
        layerx_agentd::capability::NativeAdmissionProfile::Effect,
    );
    assert_eq!(no_time, Err(ProgramSourceError::Snapshot));
}
