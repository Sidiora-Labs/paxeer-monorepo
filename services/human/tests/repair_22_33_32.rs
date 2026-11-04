//! Task 22.33.32; audit 22.30.102; acceptance 22998.1.
//! human-layout-doc is proved by documented_layout_matches_retained_workspace,
//! documented_intent_authority_checks_produced_payload,
//! documented_bank_call_crosses_custody_and_attestor_codecs,
//! documented_sponsorship_uses_chain_125_quote, and
//! documented_commitment_requires_authenticated_consensus_evidence.
//! Baseline: no target or previous gate ledger existed at session start. The
//! explicit implementation-first, hf-build then hf-done order prohibits the
//! contradictory unchanged-tree verify; no pre-change test failure is claimed.
//! This is the task record: spec/ edits are outside the user-authorized scope.
//! No out-of-path wiring is required. The crate test shim maps this planned path
//! to the Cargo integration target. Gates retain the exact candidate revision,
//! command, exit and private log; no completion is asserted before hf-done passes.
//! Proof peers are the existing committed node/sequencer transcript and loopback
//! HTTP, with production custody, attestor, receipt and consensus codecs in-process.
//! No external clock, credentials, endpoint, device or subprocess is required.

use std::collections::BTreeSet;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_kms::attestor::{decode_unsigned_evm_transaction, unsigned_evm_transaction};
use layerx_human_service::custody::EvmTransaction;
use layerx_human_service::journeys::{BankCommitEvidence, BANK_ADDRESS};
use layerx_human_service::onboarding::gas_station::{
    Call, GasStationClient, QuoteRequest, CHAIN_ID,
};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const README: &str = include_str!("../README.md");
const MANIFEST: &str = include_str!("../Cargo.toml");

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("repair 22.33.32: {error:?}"))
}

#[test]
fn documented_layout_matches_retained_workspace() {
    let members: BTreeSet<_> = MANIFEST
        .split_once("members = [")
        .expect("workspace members")
        .1
        .split_once(']')
        .expect("members end")
        .0
        .lines()
        .map(str::trim)
        .filter_map(|line| {
            line.strip_prefix('"')
                .and_then(|line| line.split_once('"'))
                .map(|(path, _)| path)
        })
        .collect();
    let layout = README
        .split_once("The members in")
        .expect("layout")
        .1
        .split_once("Related applications")
        .expect("related applications")
        .0;
    let documented: BTreeSet<_> = layout
        .lines()
        .filter_map(|line| {
            line.strip_prefix("| [`")
                .and_then(|line| line.split_once('`'))
                .map(|(path, _)| path)
        })
        .collect();
    assert_eq!(documented, members);
    let human = Path::new(env!("CARGO_MANIFEST_DIR")).join("../..");
    for member in members {
        assert!(
            human.join(member).join("Cargo.toml").is_file(),
            "missing member {member}"
        );
    }
    // Check every local Markdown destination, including wallet and application links.
    for suffix in README.split("](").skip(1) {
        let target = suffix
            .split_once(')')
            .expect("closed Markdown destination")
            .0;
        if !target.contains("://") {
            assert!(human.join(target).exists(), "broken guide link: {target}");
        }
    }
    for retired in [
        "layerx-human-movement-provider",
        "layerx-paxeer-client",
        "layerx-explorer-index",
        "](wallet",
        "services/human/wallet",
    ] {
        assert!(!README.contains(retired), "retired guide path: {retired}");
    }
    assert!(README.contains("](../../apps/wallet/README.md)"));
    assert!(README.contains("](../wallet/README.md)"));
    assert!(README.contains("kernel executes inside the chain-125 node"));
}

#[test]
fn documented_intent_authority_checks_produced_payload() {
    use layerx_intents::{compile, DidRegistration, DisclosureCheck, Intent, IntentKind};
    use layerx_types::ids::Did;
    use layerx_types::intent::PublicKey;
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    let activity = checked(ActivityType::new(ModuleId::Governance, 1));
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Governance,
        &[activity],
    ))]));
    let intent = |key| {
        Intent::v1(IntentKind::DidRegistration(checked(DidRegistration::new(
            checked(Did::new(b"did:layerx:doc-regression")),
            PublicKey::new([key; 32]),
        ))))
    };
    let authorized = intent(1);
    let produced = checked(compile(&authorized, &registry));
    let disclosed = checked(DisclosureCheck::verify(&authorized, &produced));
    assert_eq!(disclosed.canonical_payload(), produced.payload().as_bytes());
    assert_eq!(produced.activity_type(), activity);
    assert!(DisclosureCheck::verify(&intent(2), &produced).is_err());
    assert!(README.contains("authority for typed protocol intent compilation"));
    assert!(README.contains("disclosure checks"));
}

#[test]
fn documented_bank_call_crosses_custody_and_attestor_codecs() {
    let sender = unhex(transcript()["sender"].as_str().expect("fixture sender"))
        .try_into()
        .expect("address");
    let recipient = [0x22; 20];
    let mut amount = [0; 32];
    amount[31] = 90;
    let template = EvmTransaction {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 1_000_000_000_000,
        gas_limit: 1_000_000,
        to: BANK_ADDRESS,
        value: [0; 32],
        calldata: Vec::new(),
    };
    let transaction = checked(
        template
            .clone()
            .with_bank_transfer(sender, recipient, "uhpx", amount),
    );
    let call = Eip1559Call {
        chain_id: transaction.chain_id,
        nonce: transaction.nonce,
        max_priority_fee_per_gas: transaction.max_priority_fee_per_gas.into(),
        max_fee_per_gas: transaction.max_fee_per_gas.into(),
        gas_limit: transaction.gas_limit,
        to: transaction.to,
        data: transaction.calldata,
    };
    // Production custody producer -> attestor wire codec -> disclosure consumer.
    let unsigned = unsigned_evm_transaction(&call);
    assert_eq!(unsigned, call.encode_unsigned());
    let decoded = checked(decode_unsigned_evm_transaction(&unsigned));
    assert_eq!(decoded, call);
    let disclosure = checked(bind_call(&decoded, sender));
    assert_eq!(
        checked(disclosure.construct_method()),
        Some(ConstructMethod::BankSend {
            from: sender,
            to: recipient,
            denom: "uhpx".into(),
            amount,
        })
    );
    // Fixture-only secret signs through the real shared codec; production uses attestors.
    let secret: [u8; 32] = unhex(transcript()["secret"].as_str().expect("fixture secret"))
        .try_into()
        .expect("key");
    let raw = checked(decoded.sign(&secret));
    let (signed_call, signature) = checked(decode_signed(&raw));
    assert_eq!(raw[0], 2);
    assert_eq!(signed_call, call);
    assert_eq!(checked(recover_sender(&signed_call, &signature)), sender);
    assert_ne!(transaction_hash(&raw), [0; 32]);
    let mut wrong_target = template.clone();
    wrong_target.to = [0x88; 20];
    assert!(wrong_target
        .with_bank_transfer(sender, recipient, "uhpx", amount)
        .is_err());
    let mut nonzero_value = template;
    nonzero_value.value[31] = 1;
    assert!(nonzero_value
        .with_bank_transfer(sender, recipient, "uhpx", amount)
        .is_err());
    for claim in [
        "intent authority is separate from",
        "EIP-1559 codecs",
        "0x0000000000000000000000000000000000001001",
        "send(address,address,string,uint256)",
        "configured attestor key identities",
        "private material is not returned",
    ] {
        assert!(
            README.contains(claim),
            "missing documented boundary: {claim}"
        );
    }
}

#[test]
fn documented_sponsorship_uses_chain_125_quote() {
    let request = QuoteRequest {
        account: [0x22; 20],
        nonce: [0; 32],
        calls: vec![Call {
            to: BANK_ADDRESS,
            value: [0; 32],
            data: Vec::new(),
        }],
        max_token_amount: 1_000,
        gas_cost: 90,
        token: [0x33; 20],
        decimals: 18,
    };
    let encoded = checked(serde_json::to_vec(&checked(GasStationClient::quote_body(
        &request,
    ))));
    let body: Value = checked(serde_json::from_slice(&encoded));
    assert_eq!(CHAIN_ID, 125);
    assert_eq!(body["chainId"], "125");
    assert_eq!(body["maxTokenAmount"], "1000");
    assert_eq!(body["gasCost"], "90");
    assert_eq!(body["calls"][0]["value"], "0");
    let mut invalid = request;
    invalid.max_token_amount = 0;
    assert!(GasStationClient::quote_body(&invalid).is_err());
    // Bind the prose to the actual sponsor consumer, without running its CLI or env loader.
    let sponsor = include_str!("../crates/layerx-human-service/src/server/onboarding_sponsor.rs");
    for claim in [
        "bindLayerX(bytes32,bytes)",
        "0x0000000000000000000000000000000000001004",
        "LayerXBound",
    ] {
        assert!(README.contains(claim));
        assert!(sponsor.contains(claim));
    }
    assert!(sponsor.contains("Some(\"receipted\" | \"committed\")"));
    assert!(sponsor.contains("receipt[\"status\"] != \"1\""));
    assert!(README.contains("A `submitted` reply remains incomplete"));
    assert!(README.contains("A `receipted` or `committed` reply must"));
    assert!(README.contains("does not itself establish block commitment"));
}

fn transcript() -> Value {
    checked(serde_json::from_str(include_str!(
        "../crates/layerx-human-service/src/journeys/repair_22_31_153_transcript.json"
    )))
}
fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").expect("hex prefix");
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| checked(u8::from_str_radix(checked(std::str::from_utf8(pair)), 16)))
        .collect()
}

struct RpcPeer {
    endpoint: SequencerEndpoint,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl RpcPeer {
    fn start(answer: impl Fn(&Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback RPC");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().expect("address")),
            &[],
        )
        .expect("production client");
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = stop.clone();
        let calls = Arc::new(Mutex::new(Vec::new()));
        let observed = calls.clone();
        let worker = thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(value) => value,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .expect("bounded socket");
                stream
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .expect("bounded response");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().expect("length"));
                    }
                }
                let length = length.expect("content length");
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).expect("RPC body");
                let request: Value = serde_json::from_slice(&body).expect("RPC JSON");
                let result = answer(&request);
                observed
                    .lock()
                    .expect("call inventory")
                    .push(request.clone());
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).expect("response");
                stream.flush().expect("flush");
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
    fn receipts() -> Self {
        let document = transcript();
        Self::start(move |request| {
            let record = &document["records"][0];
            match request["method"].as_str().expect("method") {
                "pax_sequencerReceipt" => {
                    assert_eq!(request["params"][0], record["view"]["txHash"]);
                    record["view"].clone()
                }
                "eth_getTransactionReceipt" => {
                    assert_eq!(request["params"][0], record["view"]["txHash"]);
                    record["landing"].clone()
                }
                "px_getCommit" => record["material"].clone(),
                "px_getValidators" => record["validators"].clone(),
                "eth_getBlockByNumber" if request["params"][1] == false => {
                    json!({"number":request["params"][0], "stateRoot":record["appHash"]})
                }
                "eth_getBlockByNumber" => record["block"].clone(),
                other => panic!("unrecorded producer method {other}"),
            }
        })
    }
}
impl Drop for RpcPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("worker")
            .join()
            .expect("RPC peer thread");
    }
}

fn commit_proof() -> BankCommitEvidence {
    let document = transcript();
    let record = &document["records"][0];
    let proof = &record["proof"];
    let batch = unhex(record["batch"].as_str().expect("signed batch"));
    let mut leaf = b"LXAR\x01".to_vec();
    leaf.extend(
        proof["total"]
            .as_str()
            .expect("total")
            .parse::<i64>()
            .expect("total integer")
            .to_be_bytes(),
    );
    leaf.extend(
        proof["index"]
            .as_str()
            .expect("index")
            .parse::<i64>()
            .expect("index integer")
            .to_be_bytes(),
    );
    leaf.extend(Sha256::digest(&batch));
    let mut fields = vec![leaf];
    fields.extend(
        proof["aunts"]
            .as_array()
            .expect("aunts")
            .iter()
            .map(|aunt| {
                base64::engine::general_purpose::STANDARD
                    .decode(aunt.as_str().expect("aunt"))
                    .expect("base64 proof")
            }),
    );
    BankCommitEvidence {
        chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .expect("chain")
            .to_owned(),
        trusted_validators: record["validators"].clone(),
        committed_header: record["material"]["signedHeader"].clone(),
        covering_height: record["height"].as_u64().expect("height"),
        batch_transaction: batch,
        transaction_proof: fields,
        activity_ids: vec![unhex(record["view"]["txHash"].as_str().expect("hash"))
            .try_into()
            .expect("hash width")],
    }
}

#[test]
fn documented_commitment_requires_authenticated_consensus_evidence() {
    let peer = RpcPeer::receipts();
    let document = transcript();
    let key = &document["key"];
    let keys = vec![ReceiptKeyEntry {
        key_id: unhex(key["keyId"].as_str().expect("key id"))
            .try_into()
            .expect("digest"),
        public_key: unhex(key["publicKey"].as_str().expect("public key"))
            .try_into()
            .expect("key"),
        first_height: key["firstHeight"].as_u64().expect("first height"),
        last_height: key["lastHeight"].as_u64().expect("last height"),
    }];
    let raw = unhex(
        document["records"][0]["raw"]
            .as_str()
            .expect("producer transaction"),
    );
    let hash = transaction_hash(&raw);
    let Lookup::Verified(execution) = checked(lookup_authenticated(
        &peer.endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )) else {
        panic!("signed execution receipt required")
    };
    assert_eq!(execution.activity_id(), hash);
    assert_eq!(
        execution.rung(),
        Rung::Receipted,
        "RPC landing is insufficient for commitment"
    );
    let proof = commit_proof();
    let committed = checked(proof.authenticate(&peer.endpoint, hash, &keys));
    assert_eq!(committed.rung(), Rung::Committed);
    assert_eq!(committed.receipt().activity_id(), hash);
    assert_eq!(committed.receipt().height(), execution.height());
    assert_eq!(committed.receipt().height(), 8);
    assert_eq!(proof.covering_height, 18);
    let mut wrong_key = keys.clone();
    wrong_key[0].public_key[0] ^= 1;
    assert!(proof
        .authenticate(&peer.endpoint, hash, &wrong_key)
        .is_err());
    let mut wrong_batch = proof.clone();
    wrong_batch.batch_transaction[0] ^= 1;
    assert!(wrong_batch
        .authenticate(&peer.endpoint, hash, &keys)
        .is_err());
    let mut missing_quorum = proof;
    missing_quorum.trusted_validators = json!({"validators": []});
    assert!(missing_quorum
        .authenticate(&peer.endpoint, hash, &keys)
        .is_err());
    let calls = peer.calls.lock().expect("transport inventory");
    for method in [
        "pax_sequencerReceipt",
        "eth_getTransactionReceipt",
        "px_getCommit",
        "px_getValidators",
        "eth_getBlockByNumber",
    ] {
        assert!(
            calls.iter().any(|request| request["method"] == method),
            "real transport did not serve {method}"
        );
    }
    for claim in [
        "signed sequence batch",
        "validator quorum",
        "transaction inclusion",
        "governed receipt key",
        "hash alone does not",
        "`gateway-reported`",
    ] {
        assert!(README.contains(claim), "missing evidence boundary: {claim}");
    }
}
