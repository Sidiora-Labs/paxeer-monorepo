//! Current node ledger-payment time evidence, replayed by repair_22_33_1533.rs.
//! The node recording is rpc/sequencer_receipt_vectors_test.go's payExact case;
//! Comet headers, votes and ABCI results are committed deterministic peer fixtures.
//! Custody-credit fixtures retained elsewhere are migration-only decode coverage,
//! and are never a current time-window producer or funding path.

use ed25519_dalek::{Signer, SigningKey};
use layerx_agentd::policy_history::{read_policy_history_from_sources, PolicyHistoryWindow};
use layerx_agentd::protocol_evidence::EvidenceAuthority;
use layerx_agentd::read::{
    self, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp, CommitValidator, CommitVote,
    NativeResultSource, ServedCheckpoint,
};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_types::{ids::Did, rung::Rung};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    io::{BufRead, BufReader, Read, Write},
    net::TcpListener,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    thread,
    time::Duration,
};

const HEIGHT: u64 = 1001;
const NOW: u64 = 1_750_000_400_400;
#[track_caller]
fn must<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
    r.unwrap_or_else(|e| panic!("{e:?}"))
}
fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn quantity(v: u64) -> String {
    format!("{v:#x}")
}
fn decode_hex(v: &Value) -> Vec<u8> {
    must(hex::decode(v.as_str().unwrap().strip_prefix("0x").unwrap()))
}
fn actor() -> Did {
    must(Did::new(
        format!("did:layerx:{}", hex::encode([9; 32])).as_bytes(),
    ))
}
fn validators() -> (Vec<SigningKey>, Vec<CommitValidator>) {
    let keys = (1..=4)
        .map(|n| SigningKey::from_bytes(&[n; 32]))
        .collect::<Vec<_>>();
    let validators = keys
        .iter()
        .map(|key| {
            let public_key = key.verifying_key().to_bytes();
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power: 1,
            }
        })
        .collect();
    (keys, validators)
}
fn committed(
    height: u64,
    seconds: i64,
    parent: [u8; 32],
    result_root: [u8; 32],
    signers: usize,
) -> ServedCheckpoint {
    let (keys, validators) = validators();
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".into(),
        height,
        time: CommitTimestamp {
            seconds,
            nanos: if height >= HEIGHT { 400_000_000 } else { 0 },
        },
        last_block_id: Some(BlockId {
            hash: parent,
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: read::validator_set_hash(&validators),
        next_validators_hash: read::validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: Some(result_root),
        evidence_hash: Some([7; 32]),
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [8; 32],
        },
        votes: Vec::new(),
    };
    for (key, v) in keys.iter().zip(&validators).take(signers) {
        let timestamp = header.time;
        commit.votes.push(CommitVote {
            validator_address: v.address,
            voting_power: v.voting_power,
            timestamp,
            signature: key
                .sign(&commit.vote_sign_bytes("125", timestamp))
                .to_bytes(),
        });
    }
    // The negative quorum fixture is intentionally carried to the production verifier.
    ServedCheckpoint {
        height,
        block_hash: header.block_hash(),
        app_hash: header.app_hash,
        commit,
        rung: Rung::Committed,
        header,
        validators,
    }
}
fn signed_header(c: &ServedCheckpoint) -> Value {
    let h = &c.header;
    let time = if h.time.seconds == NOW as i64 / 1000 - 1 {
        "2025-06-15T15:13:19Z"
    } else {
        "2025-06-15T15:13:20.400Z"
    };
    let id = |b: BlockId| json!({"hash":hex::encode(b.hash),"parts":{"total":b.part_set_total,"hash":hex::encode(b.part_set_hash)}});
    json!({"header":{"version":{"block":"11","app":"1"},"chain_id":"125","height":h.height.to_string(),"time":time,
        "last_block_id":id(h.last_block_id.unwrap()),"last_commit_hash":hex::encode(h.last_commit_hash.unwrap()),"data_hash":hex::encode(h.data_hash.unwrap()),"validators_hash":hex::encode(h.validators_hash),"next_validators_hash":hex::encode(h.next_validators_hash),"consensus_hash":hex::encode(h.consensus_hash),"app_hash":hex::encode(h.app_hash),"last_results_hash":hex::encode(h.last_results_hash.unwrap()),"evidence_hash":hex::encode(h.evidence_hash.unwrap()),"proposer_address":hex::encode(h.proposer_address)},
        "commit":{"height":c.height.to_string(),"round":c.commit.round,"block_id":id(c.commit.block_id),"signatures":c.commit.votes.iter().map(|v|json!({"block_id_flag":2,"validator_address":hex::encode(v.validator_address),"timestamp":time,"signature":openssl::base64::encode_block(&v.signature)})).collect::<Vec<_>>()}})
}
fn block_json(c: &ServedCheckpoint) -> Value {
    json!({"number":quantity(c.height),"hash":rpc_hex(&c.block_hash),"parentHash":rpc_hex(&c.header.last_block_id.unwrap().hash),"timestamp":quantity(c.header.time.seconds as u64),"transactionsRoot":rpc_hex(&c.header.data_hash.unwrap()),"receiptsRoot":rpc_hex(&[9;32]),"stateRoot":rpc_hex(&c.app_hash),"transactions":[]})
}
fn protobuf_varint(mut value: u64, bytes: &mut Vec<u8>) {
    while value >= 128 {
        bytes.push(value as u8 | 128);
        value >>= 7;
    }
    bytes.push(value as u8);
}
fn protobuf_bytes(number: u64, value: &[u8], bytes: &mut Vec<u8>) {
    protobuf_varint(number << 3 | 2, bytes);
    protobuf_varint(value.len() as u64, bytes);
    bytes.extend_from_slice(value);
}
fn protobuf_integer(number: u64, value: u64, bytes: &mut Vec<u8>) {
    if value != 0 {
        protobuf_varint(number << 3, bytes);
        protobuf_varint(value, bytes);
    }
}

// api/evm/tx.proto and cosmos/base/abci/v1beta1/abci.proto, independently
// encoded by the peer rather than using NativeResultSource's result encoder.
fn abci_response(receipt: &Value) -> Vec<u8> {
    let q = |name: &str| {
        u64::from_str_radix(receipt[name].as_str().unwrap().trim_start_matches("0x"), 16).unwrap()
    };
    let mut response = Vec::new();
    protobuf_integer(1, q("gasUsed"), &mut response);
    if receipt["vmError"] != "" {
        protobuf_bytes(
            2,
            receipt["vmError"].as_str().unwrap().as_bytes(),
            &mut response,
        );
    }
    protobuf_bytes(3, &decode_hex(&receipt["returnData"]), &mut response);
    protobuf_bytes(
        4,
        receipt["txHash"].as_str().unwrap().as_bytes(),
        &mut response,
    );
    for log in receipt["logs"].as_array().unwrap() {
        let mut bytes = Vec::new();
        protobuf_bytes(1, log["address"].as_str().unwrap().as_bytes(), &mut bytes);
        for topic in log["topics"].as_array().unwrap() {
            protobuf_bytes(2, topic.as_str().unwrap().as_bytes(), &mut bytes);
        }
        protobuf_bytes(3, &decode_hex(&log["data"]), &mut bytes);
        protobuf_integer(
            4,
            u64::from_str_radix(
                log["logIndex"].as_str().unwrap().trim_start_matches("0x"),
                16,
            )
            .unwrap(),
            &mut bytes,
        );
        protobuf_bytes(5, &bytes, &mut response);
    }
    let mut msg = Vec::new();
    protobuf_bytes(1, b"/paxprotocol.paxchain.evm.MsgEVMTransaction", &mut msg);
    protobuf_bytes(2, &response, &mut msg);
    let mut tx = Vec::new();
    protobuf_bytes(1, &msg, &mut tx);
    tx
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum Attack {
    None,
    Omitted,
    PrematureEnd,
    EmptyPage,
    EmptyNextPage,
    StaleIndex,
    ForeignBlock,
    LandingIndex,
    Timestamp,
    Uncommitted,
    Quorum,
    Chain,
    Duplicate,
    ReceiptTamper,
    CanonicalTamper,
    HeaderTamper,
    SignatureTamper,
    DenominationTamper,
}
struct Fixture {
    blocks: BTreeMap<u64, ServedCheckpoint>,
    receipts: Vec<Value>,
    results: Value,
    freshness: Value,
    key: ReceiptKeyEntry,
    attack: Attack,
    calls: Vec<String>,
}
impl Fixture {
    fn new(attack: Attack) -> Self {
        // These exact signed receipts were emitted by the repository's node recorder.
        let recording: Value = must(serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        )));
        assert_eq!(
            recording["provenance"]["generator"],
            "rpc/sequencer_receipt_vectors_test.go"
        );
        let case = recording["cases"]
            .as_array()
            .unwrap()
            .iter()
            .find(|case| case["id"] == "V1-layerxledger-pay-exact")
            .unwrap();
        let raw = decode_hex(&case["rawTransaction"]);
        let (transaction, signature) = must(layerx_crypto::evm_transaction::decode_signed(&raw));
        must(layerx_crypto::evm_transaction::recover_sender(
            &transaction,
            &signature,
        ));
        assert_eq!(
            rpc_hex(&layerx_crypto::evm_transaction::transaction_hash(&raw)),
            case["receiptJson"]["txHash"]
        );
        assert_eq!(
            case["receiptJson"]["to"],
            "0x000000000000000000000000000000000000101a"
        );
        assert_eq!(case["receiptJson"]["receipt"], case["signedReceipt"]);
        let receipts = vec![case["receiptJson"].clone()];
        let history_recording: Value = must(serde_json::from_str(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        )));
        let pinned = &case["pinnedKey"];
        let key = ReceiptKeyEntry {
            key_id: decode_hex(&pinned["keyId"]).try_into().unwrap(),
            public_key: decode_hex(&pinned["publicKey"]).try_into().unwrap(),
            first_height: 1000,
            last_height: 1999,
        };
        let results = json!([{}, {"data":openssl::base64::encode_block(&abci_response(&receipts[0])),"gas_wanted":"90000","gas_used":"61340"}]);
        let leaves = (0..2)
            .map(|i| must(NativeResultSource::from_results(&results, i)).result)
            .collect::<Vec<_>>();
        let empty = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[]);
        let older = committed(HEIGHT - 1, NOW as i64 / 1000 - 1, [1; 32], empty, 4);
        let current = committed(
            HEIGHT,
            NOW as i64 / 1000 - if attack == Attack::Timestamp { 1 } else { 0 },
            older.block_hash,
            empty,
            if attack == Attack::Quorum { 2 } else { 4 },
        );
        let next = committed(
            HEIGHT + 1,
            NOW as i64 / 1000,
            current.block_hash,
            layerx_programs::chain_proof::merkle::hash_from_byte_slices(
                &leaves.iter().map(Vec::as_slice).collect::<Vec<_>>(),
            ),
            4,
        );
        Self {
            blocks: BTreeMap::from([(HEIGHT - 1, older), (HEIGHT, current), (HEIGHT + 1, next)]),
            receipts,
            results,
            freshness: history_recording["result"]["freshness"].clone(),
            key,
            attack,
            calls: Vec::new(),
        }
    }
    fn answer(&mut self, request: &Value) -> Value {
        let method = request["method"].as_str().unwrap();
        self.calls.push(method.into());
        let params = &request["params"];
        let block = &self.blocks[&HEIGHT];
        let height = || params["height"].as_str().unwrap().parse::<u64>().unwrap();
        match method {
            "eth_chainId" => json!(if self.attack == Attack::Chain {
                "0x7e"
            } else {
                "0x7d"
            }),
            "eth_blockNumber" => json!(quantity(HEIGHT)),
            "eth_getBlockByNumber" => {
                let h = u64::from_str_radix(
                    params[0].as_str().unwrap().strip_prefix("0x").unwrap(),
                    16,
                )
                .unwrap();
                block_json(&self.blocks[&h])
            }
            "px_getStateRoot" => json!(rpc_hex(&block.app_hash)),
            "px_getCommit" => {
                let h = params[0].as_str().unwrap().parse::<u64>().unwrap();
                let mut signed = signed_header(&self.blocks[&h]);
                if self.attack == Attack::HeaderTamper {
                    signed["header"]["app_hash"] = json!(hex::encode([99; 32]));
                }
                if self.attack == Attack::SignatureTamper {
                    signed["commit"]["signatures"][0]["signature"] =
                        json!(openssl::base64::encode_block(&[0; 64]));
                }
                json!({"height":h.to_string(),"signedHeader":signed})
            }
            "px_getValidators" => {
                json!({"height":HEIGHT.to_string(),"total":"4","validators":block.validators.iter().map(|v|json!({"address":hex::encode(v.address),"pubKey":{"type":"tendermint/PubKeyEd25519","value":openssl::base64::encode_block(&v.public_key)},"votingPower":v.voting_power.to_string()})).collect::<Vec<_>>()})
            }
            "commit" => json!({"signed_header":signed_header(&self.blocks[&height()])}),
            "block_results" => {
                json!({"height":height().to_string(),"txs_results":if height()==HEIGHT && self.attack!=Attack::Uncommitted {self.results.clone()}else{json!([])}})
            }
            "px_getReceiptKeys" => {
                json!({"keys":[{"keyId":rpc_hex(&self.key.key_id),"publicKey":rpc_hex(&self.key.public_key),"firstHeight":self.key.first_height,"lastHeight":self.key.last_height}]})
            }
            "eth_call" => {
                let mut address = [0; 32];
                address[12..].copy_from_slice(&decode_hex(&self.receipts[0]["from"]));
                json!(rpc_hex(&address))
            }
            "pax_sequencerReceipt" => {
                let mut receipt = self
                    .receipts
                    .iter()
                    .find(|r| r["txHash"] == params[0])
                    .unwrap()
                    .clone();
                if self.attack == Attack::ReceiptTamper {
                    receipt["timestamp"] = json!(quantity(NOW + 1));
                }
                if self.attack == Attack::CanonicalTamper {
                    let mut bytes = decode_hex(&receipt["receipt"]);
                    bytes[0] ^= 1;
                    receipt["receipt"] = json!(rpc_hex(&bytes));
                }
                if self.attack == Attack::DenominationTamper {
                    receipt["paymentDenom"] = json!("usid");
                }
                receipt
            }
            "eth_getTransactionReceipt" => {
                let r = self
                    .receipts
                    .iter()
                    .find(|r| r["txHash"] == params[0])
                    .unwrap();
                json!({"transactionHash":r["txHash"],"status":"0x1","blockNumber":quantity(HEIGHT),"blockHash":rpc_hex(&if self.attack==Attack::ForeignBlock {[0x55;32]}else{block.block_hash}),"transactionIndex":if self.attack==Attack::LandingIndex {json!("0x0")}else{r["index"].clone()},"gasUsed":r["gasUsed"],"logs":r["logs"]})
            }
            "px_getHistory" => {
                let mut rows=self.receipts.iter().enumerate().map(|(i,r)|json!({"blockHeight":HEIGHT,"txIndex":i+1,"logIndex":0,"tx_hash":r["txHash"]})).collect::<Vec<_>>();
                if let Some(cursor) = params[1].as_str() {
                    let cursor = must(read::Cursor::from_hex(cursor));
                    rows.retain(|row| {
                        read::Cursor {
                            block_height: row["blockHeight"].as_u64().unwrap(),
                            tx_index: row["txIndex"].as_u64().unwrap().try_into().unwrap(),
                            log_index: row["logIndex"].as_u64().unwrap().try_into().unwrap(),
                        } > cursor
                    });
                }
                match self.attack {
                    Attack::Duplicate if !rows.is_empty() => {
                        // Several business rows at one position legitimately share a receipt.
                        // Reusing it at a different transaction position must be refused.
                        let mut duplicate = rows[0].clone();
                        duplicate["txIndex"] = json!(2);
                        rows.push(duplicate);
                    }
                    Attack::Omitted | Attack::PrematureEnd => {
                        rows.pop();
                    }
                    Attack::EmptyPage => rows.clear(),
                    Attack::EmptyNextPage => {
                        if params[1].is_null()
                            || read::Cursor::from_hex(params[1].as_str().unwrap())
                                .unwrap()
                                .block_height
                                < HEIGHT
                        {
                            rows.clear();
                        } else {
                            rows.clear();
                        }
                    }
                    _ => {}
                }
                let next = if self.attack == Attack::EmptyNextPage
                    && (params[1].is_null()
                        || read::Cursor::from_hex(params[1].as_str().unwrap())
                            .unwrap()
                            .block_height
                            < HEIGHT)
                {
                    Some(
                        read::Cursor {
                            block_height: HEIGHT,
                            tx_index: 1,
                            log_index: 0,
                        }
                        .to_hex(),
                    )
                } else {
                    None
                };
                json!({"account":params[0],"items":rows,"next_cursor":next,"indexedHeight":if self.attack==Attack::StaleIndex {HEIGHT-1}else{HEIGHT},"chainHeight":HEIGHT,"freshness":self.freshness})
            }
            other => panic!("unexpected direct node method {other}: {params}"),
        }
    }
}
struct Node {
    url: String,
    state: Arc<Mutex<Fixture>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl Node {
    fn start(attack: Attack) -> Self {
        let state = Arc::new(Mutex::new(Fixture::new(attack)));
        let peer = state.clone();
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}", must(listener.local_addr()));
        must(listener.set_nonblocking(true));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::SeqCst) {
                let (mut stream, _) = match listener.accept() {
                    Ok(v) => v,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(e) => panic!("{e}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                let mut reader = BufReader::new(&stream);
                let mut line = String::new();
                must(reader.read_line(&mut line));
                let mut length = 0;
                loop {
                    line.clear();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, v)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = must(v.trim().parse::<usize>());
                        }
                    }
                }
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                let value = must(peer.lock()).answer(&request);
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":value}).to_string();
                must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()));
            }
        });
        Self {
            url,
            state,
            stop,
            worker: Some(worker),
        }
    }
    fn config(&self) -> layerx_agentd::node_rpc::NodeRpcConfig {
        layerx_agentd::node_rpc::NodeRpcConfig {
            url: self.url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        }
    }
    fn trust(&self) -> layerx_agentd::export::ExportTrustSource {
        let f = must(self.state.lock());
        must(layerx_agentd::export::ExportTrustSource::new(
            vec![f.key],
            f.blocks[&HEIGHT].validators.clone(),
        ))
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        must(self.worker.take().unwrap().join());
    }
}

fn time_history(
    node: &Node,
    actor: &Did,
    seconds: u64,
) -> Result<layerx_agentd::policy_history::PolicyUsageHistory, read::HistoryReadError> {
    read_policy_history_from_sources(
        &node.config(),
        &node.url,
        actor,
        &node.trust(),
        PolicyHistoryWindow::Time {
            head: HEIGHT,
            seconds,
        },
    )
}

#[test]
fn actual_native_batch_proves_time_count_and_exact_receipt_denomination() {
    let actor = actor();
    // Stop and reopen the peer and consumer over the same committed recording.
    let mut previous = None;
    for _ in 0..2 {
        let node = Node::start(Attack::None);
        let history = must(time_history(&node, &actor, 1));
        let facts = must(history.time_window(&actor, 1));
        assert_eq!(facts.actor(), &actor);
        assert_eq!(facts.count(), 1);
        assert_eq!(facts.successful_count(), 1);
        assert_eq!(facts.observed_core_ms(), NOW);
        assert_eq!(facts.first_covered_sequence(), HEIGHT - 1);
        assert_eq!(facts.through_sequence(), HEIGHT);
        assert_eq!(facts.window_seconds(), 1);
        let state = must(node.state.lock());
        assert_eq!(facts.observed_batch_id(), state.blocks[&HEIGHT].block_hash);
        let canonical = decode_hex(&state.receipts[0]["receipt"]);
        let decoded = must(layerx_wire::receipt::decode(&canonical));
        let receipt = decoded.protocol().unwrap();
        assert_eq!(receipt.timestamp(), NOW);
        assert_eq!(facts.activity_ids(), &[receipt.activity_id()]);
        assert_eq!(facts.receipt_amounts().len(), 1);
        assert_eq!(
            facts.receipt_amounts()[0].asset(),
            <[u8; 32]>::from(Sha256::digest(b"uhpx"))
        );
        assert_eq!(facts.receipt_amounts()[0].asset(), receipt.asset());
        assert_eq!(facts.receipt_amounts()[0].source_account(), receipt.from());
        // The signed transaction transfers zero native value. Its PaymentExact log
        // records a separate ledger amount; policy must not manufacture a native debit.
        assert_eq!(facts.receipt_amounts()[0].amount(), 0);
        assert_eq!(facts.receipt_amounts()[0].amount(), receipt.amount());
        let log = decode_hex(&state.receipts[0]["logs"][0]["data"]);
        assert_eq!(
            u128::from_be_bytes(log[48..64].try_into().unwrap()),
            1_000_000
        );
        if let Some(prior) = previous {
            assert_eq!(facts, prior);
        }
        previous = Some(facts);
        drop(state);
        let other = must(Did::new(
            format!("did:layerx:{}", hex::encode([10; 32])).as_bytes(),
        ));
        assert!(history.time_window(&other, 1).is_err());
        assert!(history.time_window(&actor, 0).is_err());
        assert!(history.time_window(&actor, u64::MAX).is_err());
        assert!(history.time_window(&actor, 2).is_err());
    }
}

#[test]
fn captured_native_time_evidence_refuses_missing_duplicate_and_altered_material() {
    for attack in [
        Attack::Omitted,
        Attack::PrematureEnd,
        Attack::EmptyPage,
        Attack::EmptyNextPage,
        Attack::Duplicate,
        Attack::StaleIndex,
        Attack::ForeignBlock,
        Attack::LandingIndex,
        Attack::Timestamp,
        Attack::Uncommitted,
        Attack::Quorum,
        Attack::Chain,
        Attack::ReceiptTamper,
        Attack::CanonicalTamper,
        Attack::HeaderTamper,
        Attack::SignatureTamper,
        Attack::DenominationTamper,
    ] {
        let node = Node::start(attack);
        assert!(time_history(&node, &actor(), 1).is_err(), "{attack:?}");
    }
}

#[test]
fn observed_native_clock_requires_exact_authority_and_head_coordinate() {
    let node = Node::start(Attack::None);
    let checkpoint = must(node.state.lock()).blocks[&HEIGHT].clone();
    let key = must(node.state.lock()).key.public_key;
    let authority = must(
        must(EvidenceAuthority::pinned_to_handshake(3, 125, key))
            .pin_committed_validators("125".to_owned(), checkpoint.validators.clone()),
    );
    let observed = must(authority.authenticate_committed_time(&checkpoint));
    assert_eq!(observed.observed_core_ms(), NOW);
    for changed in [
        "height",
        "hash",
        "signature",
        "time",
        "chain",
        "validators",
        "rung",
        "quorum",
    ] {
        let mut bad = checkpoint.clone();
        match changed {
            "height" => bad.height += 1,
            "hash" => bad.block_hash[0] ^= 1,
            "signature" => bad.commit.votes[0].signature[0] ^= 1,
            "time" => bad.header.time.nanos += 1,
            "chain" => bad.header.chain_id = "126".to_owned(),
            "validators" => bad.validators[0].public_key[0] ^= 1,
            "rung" => bad.rung = Rung::Receipted,
            "quorum" => bad.commit.votes.truncate(2),
            _ => unreachable!(),
        }
        assert!(
            authority.authenticate_committed_time(&bad).is_err(),
            "{changed}"
        );
    }
    let wrong_chain = must(
        must(EvidenceAuthority::pinned_to_handshake(3, 126, key))
            .pin_committed_validators("126".to_owned(), checkpoint.validators.clone()),
    );
    assert!(wrong_chain
        .authenticate_committed_time(&checkpoint)
        .is_err());
    let mut wrong_validators = checkpoint.validators.clone();
    let public = SigningKey::from_bytes(&[99; 32]).verifying_key().to_bytes();
    wrong_validators[0].public_key = public;
    wrong_validators[0].address = CommitValidator::address_for_key(&public);
    let wrong_authority = must(
        must(EvidenceAuthority::pinned_to_handshake(3, 125, key))
            .pin_committed_validators("125".to_owned(), wrong_validators),
    );
    assert!(wrong_authority
        .authenticate_committed_time(&checkpoint)
        .is_err());
    assert!(time_history(&node, &actor(), 0).is_err());
    assert!(time_history(&node, &actor(), u64::MAX).is_err());
}
