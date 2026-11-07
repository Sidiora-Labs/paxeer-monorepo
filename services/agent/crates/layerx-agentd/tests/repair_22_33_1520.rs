//! Repair record 22.33.1520 (audit 22.30.92, requirement 22998.1).
//!
//! missing_contracts -> proof_v2_contract_refuses_retired_versions.
//! proof_owner_join -> proof_owner_join_refuses_rehashed_identity_substitution.
//! daemon_atomic_creation -> daemon_creation_reopens_at_durable_admission_boundary.
//! execution_state_test -> execution_state_uses_receipt_prestate_and_authenticated_account_status.
//! fee_fixture -> fee_preferences_reach_preparation_and_reservation_at_historical_height.
//! enrolment_atomic_creation -> enrolment_creation_reopens_at_durable_admission_boundary.
//! reservation_zero_id -> scalar_zero_id_refusal_and_durable_recovery_agree.
//!
//! Wiring request: domain 9 owns spec/layerx-merge/interfaces.kvx and
//! contracts/agent-{formats,api,rust-api,config}.md; publish proof v2, canonical
//! current read/budget routes, Rust types and configuration at existing docs
//! paths, with explicit refusal of retired proof versions.
//!
//! Baseline c23ceff9cd8dd87ba1ffcf9403803efb0dfdb741: no earlier gate ledger or
//! repair target existed. No pre-change verification ran: the explicit gate
//! order requires implementation first and hf-done as the sole verification.
//! No static finding is claimed as an executed baseline failure. The runner
//! records the exact candidate revision and private evidence emitted by hf-done.
//!
//! The loopback peers use committed native codec/precompile captures and the
//! shared signed receipt/account-witness producer. Consumers are production
//! preparation, proof verification, query, limiter, codec and file store paths.

#[path = "support/budget_native_capture.rs"]
mod native_capture;
use serde_json::json;
use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agentd::budget::native_binding::{
    restore_native_preparation, validate_native_preparation, validate_retained_preparation,
    verify_native_budget_submission,
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
    requests: Arc<Mutex<Vec<Value>>>,
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
        let requests = Arc::new(Mutex::new(Vec::new()));
        let received = Arc::clone(&requests);
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
                received.lock().unwrap().push(request.clone());
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
            requests,
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
    let address = [vec![0; 12], fee_payer.to_vec()].concat();
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
        [address.clone(), vec![0; 32], public.to_vec(), main.to_vec()].concat(),
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

fn fixed_bytes(value: &Value) -> Vec<u8> {
    hex::decode(value.as_str().unwrap().trim_start_matches("0x")).unwrap()
}

fn registry(module: ModuleId, ordinal: u16) -> ModuleRegistry {
    let kind = ActivityType::new(module, ordinal).expect("operation");
    ModuleRegistry::new(&[ModuleRegistration::new(module, &[kind]).expect("registration")])
        .expect("registry")
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
    std::env::temp_dir().join(format!("repair-22-33-1520-{name}-{}", hex::encode(suffix)))
}

struct Case {
    value: Value,
    endpoint: SequencerEndpoint,
    replay: Replay,
    native_capture: Option<native_capture::NativeCapture>,
}

impl Case {
    fn open(name: &str, execution: bool) -> Self {
        let corpus: Value =
            serde_json::from_slice(include_bytes!("fixtures/repair_22_31_217/corpus.json"))
                .unwrap();
        let mut value = corpus["cases"][name].clone();
        assert!(value.is_object());
        let root =
            PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/repair_22_31_217");
        let mut answers: Vec<Value> = serde_json::from_slice(
            &std::fs::read(root.join(value["rpc_answers"].as_str().unwrap())).unwrap(),
        )
        .unwrap();
        add_account_and_fee_answers(&mut answers, &value);
        let native_capture =
            execution.then(|| native_capture::configure(name, &mut value, &mut answers));
        let replay = Replay::new(answers);
        let endpoint = replay.endpoint();
        Self {
            value,
            endpoint,
            replay,
            native_capture,
        }
    }

    fn id(&self, name: &str) -> [u8; 32] {
        fixed(&self.value[name])
    }
    fn tenant(&self) -> TenantId {
        TenantId::new(self.value["tenant"].as_str().unwrap()).unwrap()
    }
    fn did(&self) -> Did {
        Did::new(self.value["did"].as_str().unwrap().as_bytes()).unwrap()
    }

    fn transaction(&self) -> (EvmPrepared, VerifiedSubmission, ModuleRegistry) {
        let call = construct(&self.value);
        let signed = Eip1559Call {
            chain_id: 125,
            nonce: 0,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 3,
            gas_limit: 100_000,
            to: call.target.address(),
            data: call.encode().unwrap(),
        }
        .sign(&fixed(&self.value["signing_seed"]))
        .unwrap();
        let (native, signature) = decode_signed(&signed).unwrap();
        let payer = recover_sender(&native, &signature).unwrap();
        let prepared = prepare_bound(
            EvmPreparationState {
                chain_id: 125,
                pending_nonce: 0,
                base_fee_per_gas: 1,
                head_height: 8,
            },
            EvmPreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                maximum_payload_bytes: 4_194_304,
            },
            EvmPrepareRequest {
                from: payer,
                call,
                expected_nonce: Some(0),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 4_194_304,
            },
            100_000,
        )
        .unwrap();
        assert_eq!(prepared.call, native);
        let registry = registry(
            prepared.disclosure.activity_type.module(),
            prepared.disclosure.activity_type.ordinal(),
        );
        let submission = verify_before_submit(
            &signed,
            &SigningPreparation {
                actor_address: payer,
                transaction: prepared.clone(),
            },
            &payer,
            &registry,
        )
        .unwrap();
        (prepared, submission, registry)
    }

    fn caps(&self, height: u64) -> layerx_agentd::read::CommittedCaps {
        let assets = self.value["assets"]
            .as_array()
            .unwrap()
            .iter()
            .map(fixed)
            .collect::<Vec<_>>();
        layerx_agentd::read::CommittedReads::open(&self.endpoint, height)
            .unwrap()
            .inventory(&self.did(), &assets, self.id("fee_asset"))
            .unwrap()
    }
}

#[test]
fn execution_state_uses_receipt_prestate_and_authenticated_account_status() {
    use native_settlement::committed_native_execution_state_with_account_proofs;
    for name in ["deploy", "wind_down_exit"] {
        let case = Case::open(name, true);
        let (native, submission, registry) = case.transaction();
        let prepared = restore_native_preparation(
            &native.call.encode_unsigned(),
            native.from,
            native.observed_head_height,
            case.did(),
            &registry,
        )
        .unwrap();
        native_capture::execute(&case);
        let caps = case.caps(9);
        let accounts = case.value["accounts"]
            .as_array()
            .unwrap()
            .iter()
            .map(fixed)
            .collect::<Vec<_>>();
        let proofs = &case.native_capture.as_ref().unwrap().proofs;
        case.replay.requests.lock().unwrap().clear();
        let state = committed_native_execution_state_with_account_proofs(
            &mut case.replay.endpoint(),
            &case.endpoint,
            &prepared,
            &submission,
            &caps,
            case.id("program"),
            &accounts,
            proofs,
        )
        .expect("production receipt-bound execution prestate and witness consumer");
        assert_eq!(state.prestate_height(), 8);
        assert_eq!(state.program_existed(), name != "deploy");
        let calls = case
            .replay
            .requests
            .lock()
            .unwrap()
            .iter()
            .filter(|r| r["method"] == "eth_call")
            .cloned()
            .collect::<Vec<_>>();
        assert!(!calls.is_empty());
        assert!(
            calls.iter().all(|r| r["params"][1] == "0x8"),
            "all prestate queries precede the receipt block"
        );
        if name == "wind_down_exit" {
            let account = case.id("source_account");
            assert_eq!(state.account_status(account), Some((77, false)));
            let witness = &proofs[&account];
            assert!(!witness.account().frozen);
            assert_eq!(witness.header().header().batch_number(), 8);
            let mut data = transaction_hash(b"getValueAccount(bytes32,bytes32)")[..4].to_vec();
            data.extend(case.id("program"));
            data.extend(account);
            let answer = case
                .endpoint
                .call_at(8, &PrecompileTarget::Programs.address(), &data)
                .unwrap();
            let observed = decode_program_value_observation(&answer).unwrap();
            assert_eq!((observed.balance, observed.exists), (77, true));
            assert!(
                committed_native_execution_state_with_account_proofs(
                    &mut case.replay.endpoint(),
                    &case.endpoint,
                    &prepared,
                    &submission,
                    &caps,
                    case.id("program"),
                    &accounts,
                    &BTreeMap::new(),
                )
                .is_err(),
                "ABI existence alone must not authenticate frozen status"
            );
        } else {
            assert!(accounts.is_empty());
            assert_eq!(state.account_status([1; 32]), None);
            let get_program = hex::encode(transaction_hash(b"getProgram(bytes32)")[..4].to_vec());
            assert!(calls.iter().any(|r| r["params"][0]["data"]
                .as_str()
                .unwrap()
                .starts_with(&format!("0x{get_program}"))));
        }
    }
}

#[test]
fn fee_preferences_reach_preparation_and_reservation_at_historical_height() {
    let mut selected = Vec::new();
    for name in ["fee_payer_a", "fee_payer_b"] {
        let case = Case::open(name, false);
        let (native, _, registry) = case.transaction();
        let prepared = restore_native_preparation(
            &native.call.encode_unsigned(),
            native.from,
            native.observed_head_height,
            case.did(),
            &registry,
        )
        .unwrap();
        // The preference is read at the held preparation height even after the head advances.
        case.replay
            .answers
            .lock()
            .unwrap()
            .iter_mut()
            .find(|r| r["method"] == "eth_blockNumber")
            .unwrap()["result"] = json!("0x9");
        let fee = fee_policy::native_fee_policy_for(&case.endpoint, 8, native.from).unwrap();
        assert_eq!(fee.observed_sequence, 8);
        assert_eq!(fee.state_root, case.endpoint.state_root(8).unwrap());
        assert_eq!(fee.value.asset.asset_id, case.id("fee_asset"));
        let metadata = layerx_agentd::read::CommittedReads::open(&case.endpoint, 8)
            .unwrap()
            .effective_asset(case.id("fee_asset"))
            .unwrap();
        assert_eq!(
            &fee.value.asset,
            metadata.metadata(),
            "preserve every asset metadata field"
        );
        selected.push(fee.value.asset.asset_id);
        assert!(
            read_native_effect_budget_sources(
                &mut case.replay.endpoint(),
                &prepared,
                &case.caps(8),
                1_700_000_000_000,
                1,
                layerx_agentd::capability::NativeAdmissionProfile::Effect
            )
            .is_err(),
            "stale preparation must refuse admission"
        );
        let case = Case::open(name, false);
        let sources = read_native_effect_budget_sources(
            &mut case.replay.endpoint(),
            &prepared,
            &case.caps(8),
            1_700_000_000_000,
            1,
            layerx_agentd::capability::NativeAdmissionProfile::Effect,
        )
        .unwrap();
        let fee_charge = sources
            .charges()
            .iter()
            .find(|charge| charge.source.kind() == ProgramChargeKind::Fee)
            .unwrap();
        assert_eq!(fee_charge.asset, fee.value.asset.asset_id);
        let root = temporary(name);
        let mut store = Store::open(&root).unwrap();
        let limiter = BudgetLimiter::new(vec![]).unwrap();
        let limit = install_enrolment_limits(
            &mut store,
            &limiter,
            &case.tenant(),
            &[LimitConfig {
                id: LimitId([1; 16]),
                name: "payer fee".into(),
                scope: LimitScope::Tenant(sources.actor()),
                ceiling: 1_000_000,
                consumed: 0,
            }],
        )
        .unwrap()
        .remove(0);
        let allocations = sources
            .budget_allocations(|_| Ok(vec![limit.limit_id]))
            .unwrap();
        let first = allocations.charges().first().unwrap();
        assert!(allocations
            .charges()
            .iter()
            .all(|row| row.asset == first.asset && row.source == first.source));
        let config_path = root.join("denominations.json");
        std::fs::write(&config_path, serde_json::to_vec(&json!({"version":1,"tenant":case.tenant().as_str(),"limits":[{
            "stable_id":hex::encode(limit.stable_id),"asset":hex::encode(first.asset),"source":hex::encode(first.source)
        }]})).unwrap()).unwrap();
        use std::os::unix::fs::PermissionsExt;
        std::fs::set_permissions(&config_path, std::fs::Permissions::from_mode(0o600)).unwrap();
        let denominations =
            layerx_agentd::config::load_program_budget_denominations(&config_path, &case.tenant())
                .unwrap();
        install_program_enrolment_denominations(&mut store, &limiter, &denominations).unwrap();
        let reservation = limiter
            .stage_program_allocation(
                &ProgramReservationRequest {
                    id: allocations.preparation_digest(),
                    charges: allocations.charges().to_vec(),
                    expiry_sequence: 18,
                    current_sequence: 8,
                    core_deadline: None,
                },
                &allocations,
                CoreTimestampMs(1_700_000_000_000),
            )
            .unwrap()
            .publish();
        assert!(reservation
            .charges
            .iter()
            .any(|row| row.kind == ProgramChargeKind::Fee
                && row.asset == fee.value.asset.asset_id
                && row.amount == prepared.disclosure.fee_limit));
        assert_eq!(
            ProgramBudgetReservation::decode(&reservation.encode().unwrap()).unwrap(),
            reservation
        );
        drop(store);
        std::fs::remove_dir_all(root).unwrap();
    }
    assert_ne!(selected[0], selected[1]);
}

fn daemon_record(tenant: TenantId) -> DaemonLimitRecord {
    DaemonLimitRecord {
        tenant,
        budget_id: [7; 32],
        limit_id: daemon_limit_id([7; 32]),
        agent_digest: [8; 32],
        asset: [4; 32],
        ceiling: 100,
        consumed: 0,
        expiry_ms: 10_000,
        revoked: false,
        mutation_key: [9; 32],
        body_digest: [10; 32],
        revoke_key: [0; 32],
    }
}

fn store_seed(root: &std::path::Path, tenant: &TenantId) {
    let mut store = Store::open(root).unwrap();
    store
        .put_local(
            TenantKey::new(tenant.clone(), ObjectKind::Configuration, b"seed".to_vec()).unwrap(),
            vec![1],
        )
        .unwrap();
}

fn configuration_key(tenant: &TenantId, prefix: &[u8], id: &[u8]) -> TenantKey {
    TenantKey::new(
        tenant.clone(),
        ObjectKind::Configuration,
        [prefix, id].concat(),
    )
    .unwrap()
}

#[test]
fn daemon_creation_reopens_at_durable_admission_boundary() {
    let root = temporary("daemon-atomic");
    let tenant = TenantId::new("atomic-daemon").unwrap();
    store_seed(&root, &tenant);
    let record = daemon_record(tenant.clone());
    let index = configuration_key(&tenant, b"budget/daemon/limit-id/", &record.limit_id.0);
    let key = configuration_key(&tenant, b"budget/daemon/limit/", &record.budget_id);
    let limiter = BudgetLimiter::new(vec![]).unwrap();
    let before = std::fs::read(root.join("store.bin")).unwrap();
    // An interrupted publication must leave neither companion in memory or on disk.
    let mut read_only = Store::open_read_only(&root).unwrap();
    assert!(matches!(
        create_daemon_limit(&mut read_only, &limiter, record.clone(), CoreTimestampMs(1)),
        Err(DaemonLimitError::Store(_))
    ));
    assert!(read_only.get(&index).is_none() && read_only.get(&key).is_none());
    assert!(matches!(
        limiter.consumed(record.limit_id),
        Err(LimitRefusal::UnknownLimit(_))
    ));
    drop(read_only);
    assert_eq!(std::fs::read(root.join("store.bin")).unwrap(), before);
    let mut store = Store::open(&root).unwrap();
    // Stop admission after the durable publish, before the new limiter can install it.
    let occupied = BudgetLimiter::new(vec![LimitConfig {
        id: record.limit_id,
        name: "occupied admission stage".into(),
        scope: LimitScope::Agent(record.agent_digest),
        ceiling: 100,
        consumed: 0,
    }])
    .unwrap();
    assert!(matches!(
        create_daemon_limit(&mut store, &occupied, record.clone(), CoreTimestampMs(1)),
        Err(DaemonLimitError::Limit(LimitRefusal::InvalidConfiguration))
    ));
    assert!(store.get(&index).is_some() && store.get(&key).is_some());
    drop(store);
    drop(occupied);
    let mut reopened = Store::open(&root).unwrap();
    assert_eq!(
        daemon_limits(&reopened, &tenant).unwrap(),
        vec![record.clone()]
    );
    let restored = BudgetLimiter::new(vec![]).unwrap();
    assert_eq!(
        load_daemon_limits(&reopened, &restored).unwrap(),
        vec![record.clone()]
    );
    assert_eq!(
        create_daemon_limit(&mut reopened, &restored, record.clone(), CoreTimestampMs(1)).unwrap(),
        record
    );
    let held = reserve(
        &restored,
        &ReservationRequest {
            id: [1; 32],
            amount: 100,
            expiry_sequence: 10,
            current_sequence: 1,
            applicable_limits: vec![record.limit_id],
        },
    )
    .unwrap();
    assert_eq!(restored.held_exposure(record.limit_id).unwrap(), 100);
    release(&restored, held.id, ReleaseKind::Failed, 1).unwrap();
    assert_eq!(restored.held_exposure(record.limit_id).unwrap(), 0);
    drop(reopened);
    std::fs::remove_dir_all(root).unwrap();
}

#[test]
fn enrolment_creation_reopens_at_durable_admission_boundary() {
    let root = temporary("enrolment-atomic");
    let tenant = TenantId::new("atomic-enrolment").unwrap();
    store_seed(&root, &tenant);
    let config = LimitConfig {
        id: LimitId([3; 16]),
        name: "enrolment".into(),
        scope: LimitScope::Tenant([8; 32]),
        ceiling: 100,
        consumed: 7,
    };
    let stable = enrolment_limit_id(&tenant, &config.scope, &config.id);
    let id = daemon_limit_id(stable);
    let index = configuration_key(&tenant, b"budget/daemon/limit-id/", &id.0);
    let key = configuration_key(&tenant, b"budget/enrolment/limit/", &stable);
    let limiter = BudgetLimiter::new(vec![]).unwrap();
    let before = std::fs::read(root.join("store.bin")).unwrap();
    let mut read_only = Store::open_read_only(&root).unwrap();
    assert!(matches!(
        install_enrolment_limits(&mut read_only, &limiter, &tenant, &[config.clone()]),
        Err(DaemonLimitError::Store(_))
    ));
    assert!(read_only.get(&index).is_none() && read_only.get(&key).is_none());
    assert!(matches!(
        limiter.consumed(id),
        Err(LimitRefusal::UnknownLimit(_))
    ));
    drop(read_only);
    assert_eq!(std::fs::read(root.join("store.bin")).unwrap(), before);
    let poisoned = BudgetLimiter::new(vec![config.clone()]).unwrap();
    let successor = LimitConfig {
        id: LimitId([4; 16]),
        ..config.clone()
    };
    assert!(std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        let _ = poisoned.renew_locked::<DaemonLimitError>(&config, &successor, |_| {
            panic!("interrupt admission before enrolment installation")
        });
    }))
    .is_err());
    let mut store = Store::open(&root).unwrap();
    assert!(matches!(
        install_enrolment_limits(&mut store, &poisoned, &tenant, &[config.clone()]),
        Err(DaemonLimitError::Limit(LimitRefusal::Poisoned))
    ));
    assert!(store.get(&index).is_some() && store.get(&key).is_some());
    drop(store);
    drop(poisoned);
    let mut reopened = Store::open(&root).unwrap();
    let restored = BudgetLimiter::new(vec![]).unwrap();
    let installed =
        install_enrolment_limits(&mut reopened, &restored, &tenant, &[config.clone()]).unwrap();
    assert_eq!(installed[0].stable_id, stable);
    assert_eq!(restored.consumed(id).unwrap(), 7);
    assert_eq!(
        install_enrolment_limits(&mut reopened, &restored, &tenant, &[config]).unwrap(),
        installed
    );
    let held = reserve(
        &restored,
        &ReservationRequest {
            id: [1; 32],
            amount: 93,
            expiry_sequence: 10,
            current_sequence: 1,
            applicable_limits: vec![id],
        },
    )
    .unwrap();
    assert_eq!(restored.held_exposure(id).unwrap(), 93);
    release(&restored, held.id, ReleaseKind::Executed, 1).unwrap();
    assert_eq!(restored.consumed(id).unwrap(), 100);
    drop(reopened);
    std::fs::remove_dir_all(root).unwrap();
}

#[test]
fn scalar_zero_id_refusal_and_durable_recovery_agree() {
    use layerx_agentd::prepare::{DurablePreparation, LifecycleState};
    let configs = vec![
        LimitConfig {
            id: LimitId([1; 16]),
            name: "tenant".into(),
            scope: LimitScope::Tenant([2; 32]),
            ceiling: 100,
            consumed: 7,
        },
        LimitConfig {
            id: LimitId([2; 16]),
            name: "agent".into(),
            scope: LimitScope::Agent([3; 32]),
            ceiling: 200,
            consumed: 11,
        },
    ];
    let limiter = BudgetLimiter::new(configs.clone()).unwrap();
    let mut request = ReservationRequest {
        id: [0; 32],
        amount: 19,
        expiry_sequence: 20,
        current_sequence: 8,
        applicable_limits: configs.iter().map(|c| c.id).collect(),
    };
    assert_eq!(
        reserve(&limiter, &request),
        Err(LimitRefusal::InvalidRequest)
    );
    assert_eq!(
        reserve_until_core_time(&limiter, &request, CoreTimestampMs(100), CoreTimestampMs(1)),
        Err(LimitRefusal::InvalidRequest)
    );
    assert_eq!(limiter.held_reservations().unwrap(), 0);
    for config in &configs {
        assert_eq!(limiter.held_exposure(config.id).unwrap(), 0);
        assert_eq!(limiter.consumed(config.id).unwrap(), config.consumed);
    }
    request.id = [8; 32];
    let held = reserve(&limiter, &request).unwrap();
    assert_eq!(held.durable.len(), 2);
    let root = temporary("scalar-reopen");
    let tenant = TenantId::new("scalar").unwrap();
    let preparation = DurablePreparation {
        tenant: tenant.clone(),
        preparation_id: held.id,
        session_id: [3; 32],
        generation: 1,
        not_after: 20,
        payload_hash: [4; 32],
        state: LifecycleState::Prepared,
        activity_id: None,
        holds: held.durable.iter().cloned().map(|h| (h, None)).collect(),
        extensions: BTreeMap::new(),
    };
    let key = DurablePreparation::store_key(&tenant, held.id).unwrap();
    let encoded = preparation.encode().unwrap();
    let mut store = Store::open(&root).unwrap();
    store.put_local(key.clone(), encoded.clone()).unwrap();
    drop(store);
    drop(limiter);
    let store = Store::open(&root).unwrap();
    let recovered = DurablePreparation::decode(tenant, store.get(&key).unwrap().bytes()).unwrap();
    assert_eq!(recovered.encode().unwrap(), encoded);
    assert_eq!(recovered.holds, preparation.holds);
    let restored = BudgetLimiter::new(configs.clone()).unwrap();
    restore_bounded(&restored, &recovered.holds).unwrap();
    for config in &configs {
        assert_eq!(restored.held_exposure(config.id).unwrap(), 19);
        assert_eq!(restored.consumed(config.id).unwrap(), config.consumed);
    }
    assert!(restored.has_reservation(request.id).unwrap());
    release(&restored, held.id, ReleaseKind::Executed, 1).unwrap();
    for config in &configs {
        assert_eq!(restored.held_exposure(config.id).unwrap(), 0);
        assert_eq!(restored.consumed(config.id).unwrap(), config.consumed + 19);
    }
    drop(store);
    std::fs::remove_dir_all(root).unwrap();
}

fn rehash_export(document: &mut Value) {
    let did = document["proofs"]["account_join"]["account"]["layerx_did"]
        .as_str()
        .unwrap();
    let identity = [
        b"LXP/v1/did-id\0".as_slice(),
        &(did.len() as u16).to_be_bytes(),
        did.as_bytes(),
    ]
    .concat();
    document["identity_key"] = json!(hex::encode(Sha256::digest(identity)));
    let budget = fixed_bytes(&document["proofs"]["budget_call"]["result"]);
    let remaining = fixed_bytes(&document["proofs"]["remaining_call"]["result"]);
    let join = serde_json::to_vec(&document["proofs"]["account_join"]).unwrap();
    let mut digest = Sha256::new();
    digest.update(b"layerx-human/rpc-budget-evidence/v2\0");
    digest.update(fixed::<32>(&document["head"]["block_hash"]));
    for bytes in [&budget, &remaining, &join] {
        digest.update((bytes.len() as u64).to_be_bytes());
        digest.update(bytes);
    }
    document["budget_state"]["evidence_digest"] = json!(hex::encode(digest.finalize()));
    document["budget_state"]["receipt_digest"] = json!(hex::encode(Sha256::digest(&budget)));
    document["budget_state"]["checkpoint_digest"] = document["head"]["block_hash"].clone();
}

fn proof_peer() -> (Value, Replay) {
    let mut document: Value = serde_json::from_slice(include_bytes!(
        "../../../../../tests/vectors/budget-proof-v2/export.json"
    ))
    .unwrap();
    document["budget_state"]["verification"] =
        json!(layerx_types::verify::VerificationLevel::COMMITTED.wire_rank());
    let mut answers: Vec<Value> = serde_json::from_slice(include_bytes!(
        "../../../../../tests/vectors/budget-proof-v2/node-answers.json"
    ))
    .unwrap();
    let owner_account = fixed::<32>(&document["accounts"]["owner"]);
    let identity = fixed::<32>(&document["identity_key"]);
    let mut add = |target: [u8; 20], signature: &[u8], args: &[u8], result: Vec<u8>| {
        let data = [transaction_hash(signature)[..4].to_vec(), args.to_vec()].concat();
        answers.push(json!({"method":"eth_call", "params":[{"to":format!("0x{}",hex::encode(target)),
            "data":format!("0x{}",hex::encode(data))},"0x8"], "result":format!("0x{}",hex::encode(result))}));
    };
    let addr = layerx_client::sequencer::ADDR_PRECOMPILE;
    for (public, evm) in [([0x0a; 32], [0x11; 20]), ([0x0b; 32], [0x22; 20])] {
        let address = [vec![0; 12], evm.to_vec()].concat();
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
                owner_account.to_vec(),
                number(0).to_vec(),
            ]
            .concat(),
        );
    }
    let mut ledger = addr;
    ledger[19] = 0x1a;
    add(
        ledger,
        b"accountOwner(bytes32)",
        &owner_account,
        identity.to_vec(),
    );
    (document, Replay::new(answers))
}

fn with_proof_trust(
    document: &Value,
    replay: &Replay,
    run: impl FnOnce(&BudgetProofTrust<'_>, [u8; 32]),
) {
    let bytes = serde_json::to_vec(document).unwrap();
    let (height, hash) = budget_proof_committed_head(&bytes).unwrap();
    let endpoint = replay.endpoint();
    let checkpoint = layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
        &endpoint,
        height,
        Some(hash),
        "125",
    )
    .unwrap();
    let did = Did::new(
        document["proofs"]["account_join"]["account"]["layerx_did"]
            .as_str()
            .unwrap()
            .as_bytes(),
    )
    .unwrap();
    run(
        &BudgetProofTrust {
            tenant: "tenant",
            principal: "operator",
            owner: &did,
            protocol_version: 3,
            network_id: 125,
            checkpoint: &checkpoint,
            endpoint: &endpoint,
            current_height: height,
            now_ms: 1_700_000_000_000,
            maximum_age_seconds: 60,
        },
        fixed(&document["budget_state"]["budget_id"]),
    );
}

#[test]
fn proof_owner_join_refuses_rehashed_identity_substitution() {
    use layerx_agentd::budget::budget_proof::BudgetProofError;
    let (document, replay) = proof_peer();
    with_proof_trust(&document, &replay, |trust, id| {
        let verified =
            verify_budget_proof(&serde_json::to_vec(&document).unwrap(), id, trust).unwrap();
        assert_eq!(verified.remaining(), 77);
        assert_eq!(verified.source_account(), [2; 32]);
    });
    let mut forged = document.clone();
    let foreign = format!("did:layerx:{}", hex::encode([0x0b; 32]));
    forged["proofs"]["account_join"]["account"]["layerx_did"] = json!(foreign);
    forged["proofs"]["account_join"]["account"]["evm_address"] =
        json!(format!("0x{}", hex::encode([0x22; 20])));
    forged["proofs"]["account_join"]["layerx"]["accounts"][0] =
        forged["proofs"]["account_join"]["account"].clone();
    rehash_export(&mut forged);
    assert_ne!(forged["identity_key"], document["identity_key"]);
    assert_ne!(
        forged["budget_state"]["evidence_digest"],
        document["budget_state"]["evidence_digest"]
    );
    assert_eq!(
        forged["proofs"]["budget_call"],
        document["proofs"]["budget_call"]
    );
    assert_eq!(
        forged["proofs"]["remaining_call"],
        document["proofs"]["remaining_call"]
    );
    with_proof_trust(&forged, &replay, |trust, id| {
        assert!(
            matches!(
                verify_budget_proof(&serde_json::to_vec(&forged).unwrap(), id, trust),
                Err(BudgetProofError::Refused)
            ),
            "a different authenticated DID cannot own the real budget by rehashing the export"
        );
    });
    let requests = replay.requests.lock().unwrap();
    let owner_selector = format!(
        "0x{}",
        hex::encode(&transaction_hash(b"accountOwner(bytes32)")[..4])
    );
    assert!(requests.iter().filter(|r| r["method"] == "eth_call"
        && r["params"][0]["data"].as_str().unwrap().starts_with(&owner_selector)).count() >= 4,
        "both genuine and substituted identities reach the historical owner query and independent re-read");
}

#[test]
fn proof_v2_contract_refuses_retired_versions() {
    use layerx_agentd::budget::budget_proof::{BudgetProofError, BUDGET_PROOF_SCHEMA};
    let (document, replay) = proof_peer();
    assert_eq!(document["schema"], BUDGET_PROOF_SCHEMA);
    with_proof_trust(&document, &replay, |trust, id| {
        let bytes = serde_json::to_vec(&document).unwrap();
        assert_eq!(
            verify_budget_proof(&bytes, id, trust)
                .unwrap()
                .canonical_export_bytes(),
            bytes
        );
        for schema in [
            "layerx.human.budget-proof.v1",
            "layerx.human.budget-proof.v3",
            "",
        ] {
            let mut retired = document.clone();
            retired["schema"] = json!(schema);
            assert!(matches!(
                verify_budget_proof(&serde_json::to_vec(&retired).unwrap(), id, trust),
                Err(BudgetProofError::Refused)
            ));
        }
    });
}
