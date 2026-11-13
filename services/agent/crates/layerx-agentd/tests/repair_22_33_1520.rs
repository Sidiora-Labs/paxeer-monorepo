//! Repair record 22.33.1520 (audit 22.30.92, requirement 22998.1).
//!
//! `missing_contracts` -> `proof_v2_contract_refuses_retired_versions`.
//! `proof_owner_join` -> `proof_owner_join_refuses_rehashed_identity_substitution`.
//! `daemon_atomic_creation` -> `daemon_creation_reopens_at_durable_admission_boundary`.
//! `execution_state_test` -> `execution_state_uses_receipt_prestate_and_authenticated_account_status`.
//! `fee_fixture` -> `fee_preferences_reach_preparation_and_reservation_at_historical_height`.
//! `enrolment_atomic_creation` -> `enrolment_creation_reopens_at_durable_admission_boundary`.
//! `reservation_zero_id` -> `scalar_zero_id_refusal_and_durable_recovery_agree`.
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

fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

#[path = "support/budget_native_capture.rs"]
mod native_capture;
use serde_json::json;
use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};

use layerx_agentd::budget::native_binding::restore_native_preparation;
use layerx_agentd::budget::*;
use layerx_agentd::prepare::{
    prepare_bound, AbiArgument, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
    EvmPrepared, PrecompileCall, PrecompileTarget,
};
use layerx_agentd::sign::{verify_before_submit, SigningPreparation, VerifiedSubmission};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_proof::inclusion::SequencerAuthorization;
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
    let target = match require_some(value["target"].as_str(), "target") {
        "pay" => PrecompileTarget::Pay,
        "ledger" => PrecompileTarget::Ledger,
        "programs" => PrecompileTarget::Programs,
        other => panic!("unexpected native target {other}"),
    };
    let selector =
        transaction_hash(require_some(value["signature"].as_str(), "ABI signature").as_bytes());
    PrecompileCall {
        target,
        selector: require_ok(selector[..4].try_into(), "selector"),
        arguments: require_some(value["arguments"].as_array(), "ABI arguments")
            .iter()
            .map(|value| {
                if let Some(word) = value.get("word") {
                    AbiArgument::Word(fixed(word))
                } else {
                    AbiArgument::Bytes(require_ok(
                        hex::decode(require_some(value["bytes"].as_str(), "bytes")),
                        "ABI bytes",
                    ))
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
        let listener = require_ok(
            std::net::TcpListener::bind("127.0.0.1:0"),
            "fixture listener",
        );
        require_ok(listener.set_nonblocking(true), "nonblocking listener");
        let url = format!(
            "http://{}",
            require_ok(listener.local_addr(), "fixture address")
        );
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
                require_ok(
                    stream.set_read_timeout(Some(std::time::Duration::from_secs(5))),
                    "read deadline",
                );
                let mut headers = Vec::new();
                while !headers.ends_with(b"\r\n\r\n") {
                    let mut byte = [0];
                    require_ok(stream.read_exact(&mut byte), "HTTP header");
                    headers.push(byte[0]);
                    assert!(headers.len() <= 8192, "bounded HTTP header");
                }
                let headers = require_ok(String::from_utf8(headers), "HTTP headers");
                let length: usize = require_some(
                    headers.lines().find_map(|line| {
                        let (key, value) = line.split_once(':')?;
                        key.eq_ignore_ascii_case("content-length")
                            .then(|| require_ok(value.trim().parse(), "length"))
                    }),
                    "content length",
                );
                assert!(length <= 4_194_304);
                let mut bytes = vec![0; length];
                require_ok(stream.read_exact(&mut bytes), "RPC request");
                let request: Value = require_ok(serde_json::from_slice(&bytes), "request JSON");
                require_ok(received.lock(), "required test value").push(request.clone());
                let answers = require_ok(served.lock(), "peer answers");
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
                    require_ok(unrecorded.lock(), "unrecorded calls").push(request.clone());
                    json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32602,"message":"request absent from committed peer transcript"}})
                };
                let body = require_ok(serde_json::to_vec(&response), "response JSON");
                require_ok(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()), "HTTP response");
                require_ok(stream.write_all(&body), "response body");
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
        require_ok(
            SequencerEndpoint::new(&self.url, &[]),
            "loopback production client",
        )
    }
}

impl Drop for Replay {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::SeqCst);
        let result = require_some(self.worker.take(), "replay worker").join();
        if !std::thread::panicking() {
            require_ok(result, "peer replay");
            assert!(
                require_ok(self.missing.lock(), "unrecorded calls").is_empty(),
                "every request must match the committed peer transcript"
            );
        }
    }
}

fn add_account_and_fee_answers(answers: &mut Vec<Value>, case: &Value) {
    let corpus: Value = require_ok(
        serde_json::from_slice(include_bytes!("fixtures/repair_22_31_217/corpus.json")),
        "corpus",
    );
    let views = &corpus["peer_views"];
    let identity = &views["identity"];
    let public: [u8; 32] = fixed(&identity["public"]);
    let owner: [u8; 32] = fixed(&identity["owner"]);
    let main: [u8; 32] = fixed(&identity["main"]);
    let sender = |seed: u8| {
        let key = require_ok(
            k256::ecdsa::SigningKey::from_bytes((&[seed; 32]).into()),
            "fixture key",
        );
        require_ok(
            layerx_crypto::secp256k1::evm_address(
                key.verifying_key().to_encoded_point(false).as_bytes(),
            ),
            "fixture sender",
        )
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
    verify_add_account_and_fee_answers_2_repair_22_33_1520(
        VerifyAddAccountAndFeeAnswers2Repair22331520Context {
            case,
            views,
            identity,
            public,
            owner,
            main,
            sender,
            address,
            add,
            addr,
        },
    );
}

struct VerifyAddAccountAndFeeAnswers2Repair22331520Context<'a, F0, F1> {
    case: &'a Value,
    views: &'a Value,
    identity: &'a Value,
    public: [u8; 32],
    owner: [u8; 32],
    main: [u8; 32],
    sender: F0,
    address: Vec<u8>,
    add: F1,
    addr: [u8; 20],
}

fn verify_add_account_and_fee_answers_2_repair_22_33_1520(
    context: VerifyAddAccountAndFeeAnswers2Repair22331520Context<
        '_,
        impl Fn(u8) -> [u8; 20],
        impl FnMut([u8; 20], &[u8], &[u8], Vec<u8>),
    >,
) {
    let VerifyAddAccountAndFeeAnswers2Repair22331520Context {
        case,
        views,
        identity,
        public,
        owner,
        main,
        sender,
        address,
        mut add,
        addr,
    } = context;

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
        require_ok(
            hex::decode(require_some(identity["did_result"].as_str(), "DID reply")),
            "ABI reply",
        ),
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
            require_ok(
                hex::decode(require_some(
                    view["denomination"].as_str(),
                    "denomination reply",
                )),
                "ABI reply",
            ),
        );
        add(
            PrecompileTarget::Ledger.address(),
            b"getAsset(bytes32)",
            &asset,
            require_ok(
                hex::decode(require_some(view["metadata"].as_str(), "metadata reply")),
                "ABI reply",
            ),
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
    verify_add_account_and_fee_answers_1_repair_22_33_1520(
        VerifyAddAccountAndFeeAnswers1Repair22331520Context {
            case,
            views,
            owner,
            main,
            add,
            asset,
        },
    );
}

struct VerifyAddAccountAndFeeAnswers1Repair22331520Context<'a, F0> {
    case: &'a Value,
    views: &'a Value,
    owner: [u8; 32],
    main: [u8; 32],
    add: F0,
    asset: [u8; 32],
}

fn verify_add_account_and_fee_answers_1_repair_22_33_1520(
    context: VerifyAddAccountAndFeeAnswers1Repair22331520Context<
        '_,
        impl FnMut([u8; 20], &[u8], &[u8], Vec<u8>),
    >,
) {
    let VerifyAddAccountAndFeeAnswers1Repair22331520Context {
        case,
        views,
        owner,
        main,
        mut add,
        asset,
    } = context;

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
        require_ok(
            hex::decode(require_some(
                views["value_account"].as_str(),
                "value account reply",
            )),
            "ABI reply",
        ),
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
    let (call, signature) = require_ok(decode_signed(signed), "peer signed transaction");
    let NodeReceipt {
        sender,
        tx_hash,
        key,
        canonical,
        verified,
    } = verify_node_receipt_at_2_repair_22_33_1520(VerifyNodeReceiptAt2Repair22331520Context {
        signed,
        logs,
        height,
        previous,
        state,
        denom,
        call: &call,
        signature,
    });
    let protocol = require_some(verified.protocol(), "native node receipt");
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
    require_ok(
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
        ),
        "peer log bytes match the signed log effects",
    );
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

#[derive(Clone, Copy)]
struct VerifyNodeReceiptAt2Repair22331520Context<'a> {
    signed: &'a [u8],
    logs: &'a [layerx_client::sequencer::SequencerLog],
    height: u64,
    previous: [u8; 32],
    state: [u8; 32],
    denom: &'a str,
    call: &'a Eip1559Call,
    signature: layerx_crypto::signer::AgentSignature,
}

fn verify_node_receipt_at_2_repair_22_33_1520(
    context: VerifyNodeReceiptAt2Repair22331520Context<'_>,
) -> NodeReceipt {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_wire::encode::Encoder;
    use layerx_wire::limits::MAX_MESSAGE_BYTES;

    let VerifyNodeReceiptAt2Repair22331520Context {
        signed,
        logs,
        height,
        previous,
        state,
        denom,
        call,
        signature,
    } = context;

    let sender = require_ok(recover_sender(call, &signature), "peer sender");
    let tx_hash = transaction_hash(signed);
    let mut summary = Encoder::new(512);
    require_ok(summary.u64(100_000), "required test value");
    require_ok(summary.u64(100_000), "required test value");
    require_ok(summary.u64(1), "required test value");
    require_ok(summary.u32(0), "required test value");
    require_ok(summary.fixed(&[0; 20]), "required test value");
    require_ok(summary.fixed(&[0; 20]), "required test value");
    require_ok(summary.fixed(&Sha256::digest([])), "required test value");
    require_ok(summary.u32(0), "required test value");
    require_ok(
        summary.u32(require_ok(logs.len().try_into(), "required test value")),
        "required test value",
    );
    require_ok(
        summary.u8(require_ok(denom.len().try_into(), "required test value")),
        "required test value",
    );
    require_ok(summary.fixed(denom.as_bytes()), "required test value");
    let mut effects = vec![summary.finish()];
    for log in logs {
        let mut body = log.address.to_vec();
        body.push(require_ok(
            log.topics.len().try_into(),
            "required test value",
        ));
        body.extend(log.topics.concat());
        body.extend(Sha256::digest(&log.data));
        effects.push(body);
    }
    let encode = |signature: Option<[u8; 64]>| {
        let e = Encoder::new(MAX_MESSAGE_BYTES);
        verify_node_receipt_at_1_repair_22_33_1520(VerifyNodeReceiptAt1Repair22331520Context {
            height,
            previous,
            state,
            denom,
            call,
            sender,
            tx_hash,
            effects: &effects,
            signature,
            e,
        })
    };
    let key = SigningKey::from_bytes(&[8; 32]);
    let signature = key
        .sign(&require_ok(
            layerx_wire::hash::receipt_digest(&encode(None)),
            "required test value",
        ))
        .to_bytes();
    let canonical = encode(Some(signature));
    let authorization = [SequencerAuthorization::new(
        [1; 32],
        key.verifying_key().to_bytes(),
        1,
        100,
    )];
    let verified = require_ok(
        layerx_proof::receipt::verify_sequencer_signature(&canonical, &authorization),
        "peer receipt signature and exact canonical encoding",
    );
    let protocol = require_some(verified.protocol(), "native node receipt");
    assert_eq!(
        protocol.protocol_version(),
        2,
        "native sequencer receipt ABI"
    );
    NodeReceipt {
        sender,
        tx_hash,
        key,
        canonical,
        verified,
    }
}

struct VerifyNodeReceiptAt1Repair22331520Context<'a> {
    height: u64,
    previous: [u8; 32],
    state: [u8; 32],
    denom: &'a str,
    call: &'a Eip1559Call,
    sender: [u8; 20],
    tx_hash: [u8; 32],
    effects: &'a [Vec<u8>],
    signature: Option<[u8; 64]>,
    e: layerx_wire::encode::Encoder,
}

fn verify_node_receipt_at_1_repair_22_33_1520(
    context: VerifyNodeReceiptAt1Repair22331520Context<'_>,
) -> Vec<u8> {
    use layerx_wire::limits::PROTOCOL_VERSION;

    let VerifyNodeReceiptAt1Repair22331520Context {
        height,
        previous,
        state,
        denom,
        call,
        sender,
        tx_hash,
        effects,
        signature,
        mut e,
    } = context;

    require_ok(
        e.structure_header_version(0x5201, PROTOCOL_VERSION),
        "required test value",
    );
    require_ok(e.u16(PROTOCOL_VERSION), "required test value");
    require_ok(e.bytes(&tx_hash, 32), "required test value");
    require_ok(e.u64(height), "required test value");
    require_ok(e.bytes(&previous, 32), "required test value");
    require_ok(e.bytes(&state, 32), "required test value");
    require_ok(e.bytes(&[8; 32], 32), "required test value");
    require_ok(e.i32(0), "required test value");
    require_ok(e.sequence_length(effects.len(), 512), "required test value");
    for (index, body) in effects.iter().enumerate() {
        require_ok(e.u16(12), "required test value");
        require_ok(
            e.u16(require_ok(u16::try_from(index), "test value fits u16")),
            "required test value",
        );
        require_ok(e.u16(0), "required test value");
        require_ok(e.u8(if index == 0 { 1 } else { 3 }), "required test value");
        require_ok(e.u8(0), "required test value");
        require_ok(e.bytes(&[0; 32], 32), "required test value");
        require_ok(e.bytes(body, 256), "required test value");
    }
    require_ok(e.u128(100_000), "required test value");
    require_ok(e.bytes(&number(height.into()), 32), "required test value");
    require_ok(e.u16(12), "required test value");
    require_ok(e.u32(1), "required test value");
    require_ok(e.u32(0), "required test value");
    require_ok(e.u8(2), "required test value");
    require_ok(
        e.bytes(&Sha256::digest(denom.as_bytes()), 32),
        "required test value",
    );
    require_ok(e.u128(0), "required test value");
    require_ok(
        e.bytes(&[vec![0; 12], sender.to_vec()].concat(), 32),
        "required test value",
    );
    require_ok(e.u128(1_000_000), "required test value");
    require_ok(e.u128(900_000), "required test value");
    require_ok(e.u64(call.nonce), "required test value");
    require_ok(
        e.bytes(&[vec![0; 12], call.to.to_vec()].concat(), 32),
        "required test value",
    );
    require_ok(e.u128(0), "required test value");
    require_ok(e.u128(0), "required test value");
    require_ok(e.bytes(&[0; 32], 32), "required test value");
    require_ok(e.bytes(&[0; 32], 32), "required test value");
    require_ok(e.bytes(&[1; 32], 32), "required test value");
    require_ok(e.u64(1_700_000_001_000), "required test value");
    require_ok(e.u8(u8::from(signature.is_some())), "required test value");
    if let Some(signature) = signature {
        require_ok(e.bytes(&signature, 64), "required test value");
    }
    e.finish()
}

fn fixed_bytes(value: &Value) -> Vec<u8> {
    require_ok(
        hex::decode(require_some(value.as_str(), "required test value").trim_start_matches("0x")),
        "required test value",
    )
}

fn registry(module: ModuleId, ordinal: u16) -> ModuleRegistry {
    let kind = require_ok(ActivityType::new(module, ordinal), "operation");
    require_ok(
        ModuleRegistry::new(&[require_ok(
            ModuleRegistration::new(module, &[kind]),
            "registration",
        )]),
        "registry",
    )
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    require_ok(
        hex::decode(
            require_some(value.as_str(), "hex bytes")
                .strip_prefix("0x")
                .unwrap_or(require_some(value.as_str(), "hex bytes")),
        ),
        "hex encoding",
    )
    .try_into()
    .unwrap_or_else(|_| panic!("expected {N} bytes"))
}

fn temporary(name: &str) -> PathBuf {
    let mut suffix = [0; 16];
    require_ok(getrandom::fill(&mut suffix), "temporary identity");
    std::env::temp_dir().join(format!("repair-22-33-1520-{name}-{}", hex::encode(suffix)))
}

struct NativeCapture {
    proofs: BTreeMap<[u8; 32], layerx_proof::state::VerifiedAccountState>,
}

struct Case {
    value: Value,
    endpoint: SequencerEndpoint,
    replay: Replay,
    native_capture: Option<NativeCapture>,
}

impl Case {
    fn open(name: &str, execution: bool) -> Self {
        let corpus: Value = require_ok(
            serde_json::from_slice(include_bytes!("fixtures/repair_22_31_217/corpus.json")),
            "required test value",
        );
        let mut value = corpus["cases"][name].clone();
        assert!(value.is_object());
        let root =
            PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures/repair_22_31_217");
        let mut answers: Vec<Value> = require_ok(
            serde_json::from_slice(&require_ok(
                std::fs::read(root.join(require_some(
                    value["rpc_answers"].as_str(),
                    "required test value",
                ))),
                "required test value",
            )),
            "required test value",
        );
        add_account_and_fee_answers(&mut answers, &value);
        let native_capture = execution.then(|| {
            let (proofs, _) = native_capture::configure(name, &mut value, &mut answers);
            NativeCapture { proofs }
        });
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
        require_ok(
            TenantId::new(require_some(
                self.value["tenant"].as_str(),
                "required test value",
            )),
            "required test value",
        )
    }
    fn did(&self) -> Did {
        require_ok(
            Did::new(require_some(self.value["did"].as_str(), "required test value").as_bytes()),
            "required test value",
        )
    }

    fn transaction(&self) -> (EvmPrepared, VerifiedSubmission, ModuleRegistry) {
        let call = construct(&self.value);
        let signed = require_ok(
            Eip1559Call {
                chain_id: 125,
                nonce: 0,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 3,
                gas_limit: 100_000,
                to: call.target.address(),
                data: require_ok(call.encode(), "required test value"),
            }
            .sign(&fixed(&self.value["signing_seed"])),
            "required test value",
        );
        let (native, signature) = require_ok(decode_signed(&signed), "required test value");
        let payer = require_ok(recover_sender(&native, &signature), "required test value");
        let prepared = require_ok(
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
            ),
            "required test value",
        );
        assert_eq!(prepared.call, native);
        let registry = registry(
            prepared.disclosure.activity_type.module(),
            prepared.disclosure.activity_type.ordinal(),
        );
        let submission = require_ok(
            verify_before_submit(
                &signed,
                &SigningPreparation {
                    actor_address: payer,
                    transaction: prepared.clone(),
                },
                &payer,
                &registry,
            ),
            "required test value",
        );
        (prepared, submission, registry)
    }

    fn caps(&self, height: u64) -> layerx_agentd::read::CommittedCaps {
        let assets = require_some(self.value["assets"].as_array(), "required test value")
            .iter()
            .map(fixed)
            .collect::<Vec<_>>();
        require_ok(
            layerx_agentd::read::CommittedReads::open(&self.endpoint, height)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .inventory(&self.did(), &assets, self.id("fee_asset")),
            "required test value",
        )
    }
}

#[test]
fn execution_state_uses_receipt_prestate_and_authenticated_account_status() {
    use native_settlement::committed_native_execution_state_with_account_proofs;
    for name in ["deploy", "wind_down_exit"] {
        let case = Case::open(name, true);
        let (native, submission, _registry) = case.transaction();
        let prepared = require_ok(
            restore_native_preparation(
                &native.call.encode_unsigned(),
                native.from,
                native.observed_head_height,
            ),
            "required test value",
        );
        native_capture::execute(&case);
        let caps = case.caps(9);
        let accounts = require_some(case.value["accounts"].as_array(), "required test value")
            .iter()
            .map(fixed)
            .collect::<Vec<_>>();
        let proofs = &require_some(case.native_capture.as_ref(), "required test value").proofs;
        require_ok(case.replay.requests.lock(), "required test value").clear();
        let state = require_ok(
            committed_native_execution_state_with_account_proofs(
                &mut case.replay.endpoint(),
                &case.endpoint,
                &prepared,
                &submission,
                &caps,
                case.id("program"),
                &accounts,
                proofs,
            ),
            "production receipt-bound execution prestate and witness consumer",
        );
        assert_eq!(state.prestate_height(), 8);
        assert_eq!(state.program_existed(), name != "deploy");
        let calls = require_ok(case.replay.requests.lock(), "required test value")
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
            let answer = require_ok(
                case.endpoint
                    .call_at(8, &PrecompileTarget::Programs.address(), &data),
                "required test value",
            );
            let observed = require_ok(
                decode_program_value_observation(&answer),
                "required test value",
            );
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
            let get_program = hex::encode(&transaction_hash(b"getProgram(bytes32)")[..4]);
            assert!(calls.iter().any(|r| require_some(
                r["params"][0]["data"].as_str(),
                "required test value"
            )
            .starts_with(&format!("0x{get_program}"))));
        }
    }
}

#[test]
fn fee_preferences_reach_preparation_and_reservation_at_historical_height() {
    let mut selected = Vec::new();
    for name in ["fee_payer_a", "fee_payer_b"] {
        let case = Case::open(name, false);
        let (native, _, _registry) = case.transaction();
        let prepared = require_ok(
            restore_native_preparation(
                &native.call.encode_unsigned(),
                native.from,
                native.observed_head_height,
            ),
            "required test value",
        );
        let signing = SigningPreparation {
            actor_address: native.from,
            transaction: native.clone(),
        };
        // The preference is read at the held preparation height even after the head advances.
        require_some(
            case.replay
                .answers
                .lock()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .iter_mut()
                .find(|r| r["method"] == "eth_blockNumber"),
            "required test value",
        )["result"] = json!("0x9");
        let fee = require_ok(
            fee_policy::native_fee_policy_for(&case.endpoint, 8, native.from),
            "required test value",
        );
        assert_eq!(fee.observed_sequence, 8);
        assert_eq!(
            fee.state_root,
            require_ok(case.endpoint.state_root(8), "required test value")
        );
        assert_eq!(fee.value.asset.asset_id, case.id("fee_asset"));
        let metadata = require_ok(
            layerx_agentd::read::CommittedReads::open(&case.endpoint, 8)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .effective_asset(case.id("fee_asset")),
            "required test value",
        );
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
                Some(&signing),
                &case.caps(8),
                1_700_000_000_000,
                1,
                layerx_agentd::capability::NativeAdmissionProfile::Effect
            )
            .is_err(),
            "stale preparation must refuse admission"
        );
        let case = Case::open(name, false);
        let sources = require_ok(
            read_native_effect_budget_sources(
                &mut case.replay.endpoint(),
                &prepared,
                Some(&signing),
                &case.caps(8),
                1_700_000_000_000,
                1,
                layerx_agentd::capability::NativeAdmissionProfile::Effect,
            ),
            "required test value",
        );
        complete_fee_preferences_reach_preparation_and_reservation_at_historical_height_2_repair_22_33_1520(name, prepared, fee, case, &sources);
    }
    assert_ne!(selected[0], selected[1]);
}

fn complete_fee_preferences_reach_preparation_and_reservation_at_historical_height_2_repair_22_33_1520(
    name: &'static str,
    prepared: layerx_agentd::prepare::Prepared,
    fee: layerx_client::rpc::payments::CommittedSnapshot<
        layerx_client::rpc::payments::NativeFeePolicy,
    >,
    case: Case,
    sources: &VerifiedResolvedProgramCharges,
) {
    let fee_charge = require_some(
        sources
            .charges()
            .iter()
            .find(|charge| charge.source.kind() == ProgramChargeKind::Fee),
        "required test value",
    );
    assert_eq!(fee_charge.asset, fee.value.asset.asset_id);
    let root = temporary(name);
    let mut store = require_ok(Store::open(&root), "required test value");
    let limiter = require_ok(BudgetLimiter::new(vec![]), "required test value");
    let limit = require_ok(
        install_enrolment_limits(
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
        ),
        "required test value",
    )
    .remove(0);
    let allocations = require_ok(
        sources.budget_allocations(|_| Ok(vec![limit.limit_id])),
        "required test value",
    );
    let first = require_some(allocations.charges().first(), "required test value");
    assert!(allocations
        .charges()
        .iter()
        .all(|row| row.asset == first.asset && row.source == first.source));
    let config_path = root.join("denominations.json");
    require_ok(
        std::fs::write(
            &config_path,
            require_ok(
                serde_json::to_vec(
                    &json!({"version":1,"tenant":case.tenant().as_str(),"limits":[{
                        "stable_id":hex::encode(limit.stable_id),"asset":hex::encode(first.asset),"source":hex::encode(first.source)
                    }]}),
                ),
                "required test value",
            ),
        ),
        "required test value",
    );

    complete_fee_preferences_reach_preparation_and_reservation_at_historical_height_1_repair_22_33_1520(CompleteFeePreferencesReachPreparationAndReservationAtHistoricalHeight1Repair22331520Context { prepared, fee, case, root, store, limiter, allocations, config_path });
}

struct CompleteFeePreferencesReachPreparationAndReservationAtHistoricalHeight1Repair22331520Context
{
    prepared: layerx_agentd::prepare::Prepared,
    fee: layerx_client::rpc::payments::CommittedSnapshot<
        layerx_client::rpc::payments::NativeFeePolicy,
    >,
    case: Case,
    root: PathBuf,
    store: Store,
    limiter: BudgetLimiter,
    allocations: VerifiedProgramBudgetAllocations,
    config_path: PathBuf,
}

fn complete_fee_preferences_reach_preparation_and_reservation_at_historical_height_1_repair_22_33_1520(
    context: CompleteFeePreferencesReachPreparationAndReservationAtHistoricalHeight1Repair22331520Context,
) {
    let CompleteFeePreferencesReachPreparationAndReservationAtHistoricalHeight1Repair22331520Context { prepared, fee, case, root, mut store, limiter, allocations, config_path } = context;

    require_ok(
        std::fs::set_permissions(
            &config_path,
            <std::fs::Permissions as std::os::unix::fs::PermissionsExt>::from_mode(0o600),
        ),
        "required test value",
    );
    let denominations = require_ok(
        layerx_agentd::config::load_program_budget_denominations(&config_path, &case.tenant()),
        "required test value",
    );
    require_ok(
        install_program_enrolment_denominations(&mut store, &limiter, &denominations),
        "required test value",
    );
    let reservation = require_ok(
        limiter.stage_program_allocation(
            &ProgramReservationRequest {
                id: allocations.preparation_digest(),
                charges: allocations.charges().to_vec(),
                expiry_sequence: 18,
                current_sequence: 8,
                core_deadline: None,
            },
            &allocations,
            CoreTimestampMs(1_700_000_000_000),
        ),
        "required test value",
    )
    .publish();
    assert!(reservation
        .charges
        .iter()
        .any(|row| row.kind == ProgramChargeKind::Fee
            && row.asset == fee.value.asset.asset_id
            && row.amount == prepared.disclosure.fee_limit));
    assert_eq!(
        require_ok(
            ProgramBudgetReservation::decode(&require_ok(
                reservation.encode(),
                "required test value"
            )),
            "required test value"
        ),
        reservation
    );
    drop(store);
    require_ok(std::fs::remove_dir_all(root), "required test value");
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
    let mut store = require_ok(Store::open(root), "required test value");
    require_ok(
        store.put_local(
            require_ok(
                TenantKey::new(tenant.clone(), ObjectKind::Configuration, b"seed".to_vec()),
                "required test value",
            ),
            vec![1],
        ),
        "required test value",
    );
}

fn configuration_key(tenant: &TenantId, prefix: &[u8], id: &[u8]) -> TenantKey {
    require_ok(
        TenantKey::new(
            tenant.clone(),
            ObjectKind::Configuration,
            [prefix, id].concat(),
        ),
        "required test value",
    )
}

#[test]
fn daemon_creation_reopens_at_durable_admission_boundary() {
    let root = temporary("daemon-atomic");
    let tenant = require_ok(TenantId::new("atomic-daemon"), "required test value");
    store_seed(&root, &tenant);
    let record = daemon_record(tenant.clone());
    let index = configuration_key(&tenant, b"budget/daemon/limit-id/", &record.limit_id.0);
    let key = configuration_key(&tenant, b"budget/daemon/limit/", &record.budget_id);
    let limiter = require_ok(BudgetLimiter::new(vec![]), "required test value");
    let before = require_ok(std::fs::read(root.join("store.bin")), "required test value");
    // An interrupted publication must leave neither companion in memory or on disk.
    let mut read_only = require_ok(Store::open_read_only(&root), "required test value");
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
    assert_eq!(
        require_ok(std::fs::read(root.join("store.bin")), "required test value"),
        before
    );
    let mut store = require_ok(Store::open(&root), "required test value");
    // Stop admission after the durable publish, before the new limiter can install it.
    let occupied = require_ok(
        BudgetLimiter::new(vec![LimitConfig {
            id: record.limit_id,
            name: "occupied admission stage".into(),
            scope: LimitScope::Agent(record.agent_digest),
            ceiling: 100,
            consumed: 0,
        }]),
        "required test value",
    );
    assert!(matches!(
        create_daemon_limit(&mut store, &occupied, record.clone(), CoreTimestampMs(1)),
        Err(DaemonLimitError::Limit(LimitRefusal::InvalidConfiguration))
    ));
    assert!(store.get(&index).is_some() && store.get(&key).is_some());
    drop(store);
    drop(occupied);
    let mut reopened = require_ok(Store::open(&root), "required test value");
    assert_eq!(
        require_ok(daemon_limits(&reopened, &tenant), "required test value"),
        vec![record.clone()]
    );
    let restored = require_ok(BudgetLimiter::new(vec![]), "required test value");
    assert_eq!(
        require_ok(
            load_daemon_limits(&reopened, &restored),
            "required test value"
        ),
        vec![record.clone()]
    );
    assert_eq!(
        require_ok(
            create_daemon_limit(&mut reopened, &restored, record.clone(), CoreTimestampMs(1)),
            "required test value"
        ),
        record
    );
    let held = require_ok(
        reserve(
            &restored,
            &ReservationRequest {
                id: [1; 32],
                amount: 100,
                expiry_sequence: 10,
                current_sequence: 1,
                applicable_limits: vec![record.limit_id],
            },
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(
            restored.held_exposure(record.limit_id),
            "required test value"
        ),
        100
    );
    require_ok(
        release(&restored, held.id, ReleaseKind::Failed, 1),
        "required test value",
    );
    assert_eq!(
        require_ok(
            restored.held_exposure(record.limit_id),
            "required test value"
        ),
        0
    );
    drop(reopened);
    require_ok(std::fs::remove_dir_all(root), "required test value");
}

#[test]
fn enrolment_creation_reopens_at_durable_admission_boundary() {
    let root = temporary("enrolment-atomic");
    let tenant = require_ok(TenantId::new("atomic-enrolment"), "required test value");
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
    let limiter = require_ok(BudgetLimiter::new(vec![]), "required test value");
    let before = require_ok(std::fs::read(root.join("store.bin")), "required test value");
    let mut read_only = require_ok(Store::open_read_only(&root), "required test value");
    assert!(matches!(
        install_enrolment_limits(
            &mut read_only,
            &limiter,
            &tenant,
            std::slice::from_ref(&config)
        ),
        Err(DaemonLimitError::Store(_))
    ));
    assert!(read_only.get(&index).is_none() && read_only.get(&key).is_none());
    assert!(matches!(
        limiter.consumed(id),
        Err(LimitRefusal::UnknownLimit(_))
    ));
    drop(read_only);
    assert_eq!(
        require_ok(std::fs::read(root.join("store.bin")), "required test value"),
        before
    );
    let poisoned = require_ok(
        BudgetLimiter::new(vec![config.clone()]),
        "required test value",
    );
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
    let store = require_ok(Store::open(&root), "required test value");
    complete_enrolment_creation_reopens_at_durable_admission_boundary_1_repair_22_33_1520(
        CompleteEnrolmentCreationReopensAtDurableAdmissionBoundary1Repair22331520Context {
            root,
            tenant,
            config,
            stable,
            id,
            index,
            key,
            poisoned,
            store,
        },
    );
}

struct CompleteEnrolmentCreationReopensAtDurableAdmissionBoundary1Repair22331520Context {
    root: PathBuf,
    tenant: TenantId,
    config: LimitConfig,
    stable: [u8; 32],
    id: LimitId,
    index: TenantKey,
    key: TenantKey,
    poisoned: BudgetLimiter,
    store: Store,
}

fn complete_enrolment_creation_reopens_at_durable_admission_boundary_1_repair_22_33_1520(
    context: CompleteEnrolmentCreationReopensAtDurableAdmissionBoundary1Repair22331520Context,
) {
    let CompleteEnrolmentCreationReopensAtDurableAdmissionBoundary1Repair22331520Context {
        root,
        tenant,
        config,
        stable,
        id,
        index,
        key,
        poisoned,
        mut store,
    } = context;

    assert!(matches!(
        install_enrolment_limits(
            &mut store,
            &poisoned,
            &tenant,
            std::slice::from_ref(&config)
        ),
        Err(DaemonLimitError::Limit(LimitRefusal::Poisoned))
    ));
    assert!(store.get(&index).is_some() && store.get(&key).is_some());
    drop(store);
    drop(poisoned);
    let mut reopened = require_ok(Store::open(&root), "required test value");
    let restored = require_ok(BudgetLimiter::new(vec![]), "required test value");
    let installed = require_ok(
        install_enrolment_limits(
            &mut reopened,
            &restored,
            &tenant,
            std::slice::from_ref(&config),
        ),
        "required test value",
    );
    assert_eq!(installed[0].stable_id, stable);
    assert_eq!(require_ok(restored.consumed(id), "required test value"), 7);
    assert_eq!(
        require_ok(
            install_enrolment_limits(&mut reopened, &restored, &tenant, &[config]),
            "required test value"
        ),
        installed
    );
    let held = require_ok(
        reserve(
            &restored,
            &ReservationRequest {
                id: [1; 32],
                amount: 93,
                expiry_sequence: 10,
                current_sequence: 1,
                applicable_limits: vec![id],
            },
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(restored.held_exposure(id), "required test value"),
        93
    );
    require_ok(
        release(&restored, held.id, ReleaseKind::Executed, 1),
        "required test value",
    );
    assert_eq!(
        require_ok(restored.consumed(id), "required test value"),
        100
    );
    drop(reopened);
    require_ok(std::fs::remove_dir_all(root), "required test value");
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
    let limiter = require_ok(BudgetLimiter::new(configs.clone()), "required test value");
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
    assert_eq!(
        require_ok(limiter.held_reservations(), "required test value"),
        0
    );
    for config in &configs {
        assert_eq!(
            require_ok(limiter.held_exposure(config.id), "required test value"),
            0
        );
        assert_eq!(
            require_ok(limiter.consumed(config.id), "required test value"),
            config.consumed
        );
    }
    request.id = [8; 32];
    let held = require_ok(reserve(&limiter, &request), "required test value");
    assert_eq!(held.durable.len(), 2);
    let root = temporary("scalar-reopen");
    let tenant = require_ok(TenantId::new("scalar"), "required test value");
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
    let key = require_ok(
        DurablePreparation::store_key(&tenant, held.id),
        "required test value",
    );
    complete_scalar_zero_id_refusal_and_durable_recovery_agree_1_repair_22_33_1520(
        CompleteScalarZeroIdRefusalAndDurableRecoveryAgree1Repair22331520Context {
            configs,
            limiter,
            request,
            held,
            root,
            tenant,
            preparation,
            key,
        },
    );
}

struct CompleteScalarZeroIdRefusalAndDurableRecoveryAgree1Repair22331520Context {
    configs: Vec<LimitConfig>,
    limiter: BudgetLimiter,
    request: ReservationRequest,
    held: BudgetReservation,
    root: PathBuf,
    tenant: TenantId,
    preparation: layerx_agentd::prepare::DurablePreparation,
    key: TenantKey,
}

fn complete_scalar_zero_id_refusal_and_durable_recovery_agree_1_repair_22_33_1520(
    context: CompleteScalarZeroIdRefusalAndDurableRecoveryAgree1Repair22331520Context,
) {
    use layerx_agentd::prepare::DurablePreparation;

    let CompleteScalarZeroIdRefusalAndDurableRecoveryAgree1Repair22331520Context {
        configs,
        limiter,
        request,
        held,
        root,
        tenant,
        preparation,
        key,
    } = context;

    let encoded = require_ok(preparation.encode(), "required test value");
    let mut store = require_ok(Store::open(&root), "required test value");
    require_ok(
        store.put_local(key.clone(), encoded.clone()),
        "required test value",
    );
    drop(store);
    drop(limiter);
    let store = require_ok(Store::open(&root), "required test value");
    let recovered = require_ok(
        DurablePreparation::decode(
            tenant,
            require_some(store.get(&key), "required test value").bytes(),
        ),
        "required test value",
    );
    assert_eq!(
        require_ok(recovered.encode(), "required test value"),
        encoded
    );
    assert_eq!(recovered.holds, preparation.holds);
    let restored = require_ok(BudgetLimiter::new(configs.clone()), "required test value");
    require_ok(
        restore_bounded(&restored, &recovered.holds),
        "required test value",
    );
    for config in &configs {
        assert_eq!(
            require_ok(restored.held_exposure(config.id), "required test value"),
            19
        );
        assert_eq!(
            require_ok(restored.consumed(config.id), "required test value"),
            config.consumed
        );
    }
    assert!(require_ok(
        restored.has_reservation(request.id),
        "required test value"
    ));
    require_ok(
        release(&restored, held.id, ReleaseKind::Executed, 1),
        "required test value",
    );
    for config in &configs {
        assert_eq!(
            require_ok(restored.held_exposure(config.id), "required test value"),
            0
        );
        assert_eq!(
            require_ok(restored.consumed(config.id), "required test value"),
            config.consumed + 19
        );
    }
    drop(store);
    require_ok(std::fs::remove_dir_all(root), "required test value");
}

fn rehash_export(document: &mut Value) {
    let did = require_some(
        document["proofs"]["account_join"]["account"]["layerx_did"].as_str(),
        "required test value",
    );
    let identity = [
        b"LXP/v1/did-id\0".as_slice(),
        &(require_ok(u16::try_from(did.len()), "test value fits u16")).to_be_bytes(),
        did.as_bytes(),
    ]
    .concat();
    document["identity_key"] = json!(hex::encode(Sha256::digest(identity)));
    let budget = fixed_bytes(&document["proofs"]["budget_call"]["result"]);
    let remaining = fixed_bytes(&document["proofs"]["remaining_call"]["result"]);
    let join = require_ok(
        serde_json::to_vec(&document["proofs"]["account_join"]),
        "required test value",
    );
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
    let mut document: Value = require_ok(
        serde_json::from_slice(include_bytes!(
            "../../../../../tests/vectors/budget-proof-v2/export.json"
        )),
        "required test value",
    );
    document["budget_state"]["verification"] =
        json!(layerx_types::verify::VerificationLevel::COMMITTED.wire_rank());
    let mut answers: Vec<Value> = require_ok(
        serde_json::from_slice(include_bytes!(
            "../../../../../tests/vectors/budget-proof-v2/node-answers.json"
        )),
        "required test value",
    );
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
    let bytes = require_ok(serde_json::to_vec(document), "required test value");
    let (height, hash) = require_ok(budget_proof_committed_head(&bytes), "required test value");
    let endpoint = replay.endpoint();
    let checkpoint = require_ok(
        layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
            &endpoint,
            height,
            Some(hash),
            "125",
        ),
        "required test value",
    );
    let did = require_ok(
        Did::new(
            require_some(
                document["proofs"]["account_join"]["account"]["layerx_did"].as_str(),
                "required test value",
            )
            .as_bytes(),
        ),
        "required test value",
    );
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
        let verified = require_ok(
            verify_budget_proof(
                &require_ok(serde_json::to_vec(&document), "required test value"),
                id,
                trust,
            ),
            "required test value",
        );
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
                verify_budget_proof(
                    &require_ok(serde_json::to_vec(&forged), "required test value"),
                    id,
                    trust
                ),
                Err(BudgetProofError::Refused)
            ),
            "a different authenticated DID cannot own the real budget by rehashing the export"
        );
    });
    let requests = require_ok(replay.requests.lock(), "required test value");
    let owner_selector = format!(
        "0x{}",
        hex::encode(&transaction_hash(b"accountOwner(bytes32)")[..4])
    );
    assert!(requests.iter().filter(|r| r["method"] == "eth_call"
        && require_some(r["params"][0]["data"].as_str(), "required test value").starts_with(&owner_selector)).count() >= 4,
        "both genuine and substituted identities reach the historical owner query and independent re-read");
}

#[test]
fn proof_v2_contract_refuses_retired_versions() {
    use layerx_agentd::budget::budget_proof::{BudgetProofError, BUDGET_PROOF_SCHEMA};
    let (document, replay) = proof_peer();
    assert_eq!(document["schema"], BUDGET_PROOF_SCHEMA);
    with_proof_trust(&document, &replay, |trust, id| {
        let bytes = require_ok(serde_json::to_vec(&document), "required test value");
        assert_eq!(
            require_ok(
                verify_budget_proof(&bytes, id, trust),
                "required test value"
            )
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
                verify_budget_proof(
                    &require_ok(serde_json::to_vec(&retired), "required test value"),
                    id,
                    trust
                ),
                Err(BudgetProofError::Refused)
            ));
        }
    });
}

struct NodeReceipt {
    sender: [u8; 20],
    tx_hash: [u8; 32],
    key: ed25519_dalek::SigningKey,
    canonical: Vec<u8>,
    verified: layerx_wire::receipt::Receipt,
}
