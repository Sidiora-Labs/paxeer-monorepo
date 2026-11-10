//! Task 22.31.219: production codecs, socket transport, durable owners and native proofs.
//! Finding/test mapping and cross-domain wiring are in `src/repair_22_31_219.kvx`.
fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use ed25519_dalek::{Signer, SigningKey};
use layerx_agentd::{
    human::{
        self, HumanAgentJourneyKind, HumanFinalizationEvidence, HumanOperationError, HumanPeer,
        HumanRefusal, HumanRequest, HumanWireResponse,
    },
    identity::{self, ProtocolAuthority},
    managed_agent::{self, ManagedAgent, ManagedAgentCheck, SessionObservationV1},
    outbox::Outbox,
    prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget},
    read::{
        self, BlockCommit, BlockId, CheckpointHeader, CheckpointRequest, CommitTimestamp,
        CommitValidator, CommitVote, NativeResultSource, ServedCheckpoint,
    },
    receipt,
    session::{self, OpenRequest, SessionId, SessionRegistry},
    store::{Store, TenantId},
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::{ids::Did, rung::Rung, verify::VerificationLevel};
use layerx_wire::{encode::Encoder, hash::receipt_digest};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::{BTreeMap, BTreeSet},
    io::{BufRead, BufReader, Read, Write},
    net::TcpListener,
    ops::ControlFlow,
    os::unix::{fs::PermissionsExt, net::UnixStream},
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, AtomicU64, Ordering},
        Arc, Mutex,
    },
    thread,
    time::Duration,
};
const HEIGHT: u64 = 1000;
const NOW: u64 = 1_750_000_400_000;
static NEXT: AtomicU64 = AtomicU64::new(1);
#[track_caller]
fn must<T, E: std::fmt::Debug>(r: Result<T, E>) -> T {
    match r {
        Ok(value) => value,
        Err(error) => panic!("{error:?}"),
    }
}
fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn quantity(v: u64) -> String {
    format!("{v:#x}")
}
fn decode_hex(v: &Value) -> Vec<u8> {
    must(hex::decode(require_some(
        v.as_str()
            .unwrap_or_else(|| panic!("{}", "required test value"))
            .strip_prefix("0x"),
        "required test value",
    )))
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let p = must(std::fs::canonicalize(std::env::temp_dir())).join(format!(
            "repair219-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::SeqCst)
        ));
        must(std::fs::create_dir(&p));
        must(std::fs::set_permissions(
            &p,
            std::fs::Permissions::from_mode(0o700),
        ));
        Self(p)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        must(std::fs::remove_dir_all(&self.0));
    }
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
        time: CommitTimestamp { seconds, nanos: 0 },
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
    let time = if h.time.seconds == require_ok(i64::try_from(NOW), "test value fits i64") / 1000 - 1
    {
        "2025-06-15T15:13:19Z"
    } else {
        "2025-06-15T15:13:20Z"
    };
    let id = |b: BlockId| json!({"hash":hex::encode(b.hash),"parts":{"total":b.part_set_total,"hash":hex::encode(b.part_set_hash)}});
    json!({"header":{"version":{"block":"11","app":"1"},"chain_id":"125","height":h.height.to_string(),"time":time,
        "last_block_id":id(require_some(h.last_block_id, "required test value")),"last_commit_hash":hex::encode(require_some(h.last_commit_hash, "required test value")),"data_hash":hex::encode(require_some(h.data_hash, "required test value")),"validators_hash":hex::encode(h.validators_hash),"next_validators_hash":hex::encode(h.next_validators_hash),"consensus_hash":hex::encode(h.consensus_hash),"app_hash":hex::encode(h.app_hash),"last_results_hash":hex::encode(require_some(h.last_results_hash, "required test value")),"evidence_hash":hex::encode(require_some(h.evidence_hash, "required test value")),"proposer_address":hex::encode(h.proposer_address)},
        "commit":{"height":c.height.to_string(),"round":c.commit.round,"block_id":id(c.commit.block_id),"signatures":c.commit.votes.iter().map(|v|json!({"block_id_flag":2,"validator_address":hex::encode(v.validator_address),"timestamp":time,"signature":openssl::base64::encode_block(&v.signature)})).collect::<Vec<_>>()}})
}
fn block_json(c: &ServedCheckpoint) -> Value {
    json!({"number":quantity(c.height),"hash":rpc_hex(&c.block_hash),"parentHash":rpc_hex(&require_some(c.header.last_block_id, "required test value").hash),"timestamp":quantity(require_ok(u64::try_from(c.header.time.seconds), "test value fits u64")),"transactionsRoot":rpc_hex(&require_some(c.header.data_hash, "required test value")),"receiptsRoot":rpc_hex(&[9;32]),"stateRoot":rpc_hex(&c.app_hash),"transactions":[]})
}
fn protobuf_varint(mut value: u64, bytes: &mut Vec<u8>) {
    while value >= 128 {
        bytes.push(value.to_le_bytes()[0] | 128);
        value >>= 7;
    }
    bytes.push(value.to_le_bytes()[0]);
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
        require_ok(
            u64::from_str_radix(
                require_some(receipt[name].as_str(), "required test value")
                    .trim_start_matches("0x"),
                16,
            ),
            "required test value",
        )
    };
    let mut response = Vec::new();
    protobuf_integer(1, q("gasUsed"), &mut response);
    if receipt["vmError"] != "" {
        protobuf_bytes(
            2,
            require_some(receipt["vmError"].as_str(), "required test value").as_bytes(),
            &mut response,
        );
    }
    protobuf_bytes(3, &decode_hex(&receipt["returnData"]), &mut response);
    protobuf_bytes(
        4,
        require_some(receipt["txHash"].as_str(), "required test value").as_bytes(),
        &mut response,
    );
    for log in require_some(receipt["logs"].as_array(), "required test value") {
        let mut bytes = Vec::new();
        protobuf_bytes(
            1,
            require_some(log["address"].as_str(), "required test value").as_bytes(),
            &mut bytes,
        );
        for topic in require_some(log["topics"].as_array(), "required test value") {
            protobuf_bytes(
                2,
                require_some(topic.as_str(), "required test value").as_bytes(),
                &mut bytes,
            );
        }
        protobuf_bytes(3, &decode_hex(&log["data"]), &mut bytes);
        protobuf_integer(
            4,
            require_ok(
                u64::from_str_radix(
                    require_some(log["logIndex"].as_str(), "required test value")
                        .trim_start_matches("0x"),
                    16,
                ),
                "required test value",
            ),
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
}
struct Fixture {
    blocks: BTreeMap<u64, ServedCheckpoint>,
    receipts: Vec<Value>,
    results: Value,
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
        let receipts = require_some(recording["cases"].as_array(), "required test value")[..2]
            .iter()
            .map(|case| case["receiptJson"].clone())
            .collect::<Vec<_>>();
        let pinned = &recording["cases"][0]["pinnedKey"];
        let key = ReceiptKeyEntry {
            key_id: require_ok(
                decode_hex(&pinned["keyId"]).try_into(),
                "required test value",
            ),
            public_key: require_ok(
                decode_hex(&pinned["publicKey"]).try_into(),
                "required test value",
            ),
            first_height: 1000,
            last_height: 1999,
        };
        let results = json!([{}, {"data":openssl::base64::encode_block(&abci_response(&receipts[0])),"gas_wanted":"60000","gas_used":"21000"},
            {"data":openssl::base64::encode_block(&abci_response(&receipts[1])),"gas_wanted":"60000","gas_used":"46220"}]);
        let leaves = (0..3)
            .map(|i| must(NativeResultSource::from_results(&results, i)).result)
            .collect::<Vec<_>>();
        let empty = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[]);
        let older = committed(
            HEIGHT - 1,
            require_ok(i64::try_from(NOW), "test value fits i64") / 1000 - 1,
            [1; 32],
            empty,
            4,
        );
        let current = committed(
            HEIGHT,
            require_ok(i64::try_from(NOW), "test value fits i64") / 1000
                - i64::from(attack == Attack::Timestamp),
            older.block_hash,
            empty,
            if attack == Attack::Quorum { 2 } else { 4 },
        );
        let next = committed(
            HEIGHT + 1,
            require_ok(i64::try_from(NOW), "test value fits i64") / 1000,
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
            key,
            attack,
            calls: Vec::new(),
        }
    }
    fn answer(&mut self, request: &Value) -> Value {
        let method = require_some(request["method"].as_str(), "required test value");
        self.calls.push(method.into());
        let params = &request["params"];
        let block = &self.blocks[&HEIGHT];
        let height = || {
            require_ok(
                params["height"]
                    .as_str()
                    .unwrap_or_else(|| panic!("{}", "required test value"))
                    .parse::<u64>(),
                "required test value",
            )
        };
        match method {
            "eth_chainId" => json!(if self.attack == Attack::Chain {
                "0x7e"
            } else {
                "0x7d"
            }),
            "eth_blockNumber" => json!(quantity(HEIGHT)),
            "eth_getBlockByNumber" => {
                let h = require_ok(
                    u64::from_str_radix(
                        require_some(
                            params[0]
                                .as_str()
                                .unwrap_or_else(|| panic!("{}", "required test value"))
                                .strip_prefix("0x"),
                            "required test value",
                        ),
                        16,
                    ),
                    "required test value",
                );
                block_json(&self.blocks[&h])
            }
            "px_getStateRoot" => json!(rpc_hex(&block.app_hash)),
            "px_getCommit" => {
                let h = require_ok(
                    params[0]
                        .as_str()
                        .unwrap_or_else(|| panic!("{}", "required test value"))
                        .parse::<u64>(),
                    "required test value",
                );
                json!({"height":h.to_string(),"signedHeader":signed_header(&self.blocks[&h])})
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
            "pax_sequencerReceipt" => require_some(
                self.receipts.iter().find(|r| r["txHash"] == params[0]),
                "required test value",
            )
            .clone(),
            "eth_getTransactionReceipt" => {
                let r = require_some(
                    self.receipts.iter().find(|r| r["txHash"] == params[0]),
                    "required test value",
                );
                json!({"transactionHash":r["txHash"],"status":"0x1","blockNumber":quantity(HEIGHT),"blockHash":rpc_hex(&if self.attack==Attack::ForeignBlock {[0x55;32]}else{block.block_hash}),"transactionIndex":if self.attack==Attack::LandingIndex {json!("0x0")}else{r["index"].clone()},"gasUsed":r["gasUsed"],"logs":r["logs"]})
            }
            "px_getHistory" => {
                let recorded: Value = must(serde_json::from_str(include_str!(
                    "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
                )));
                self.complete_answer_1_repair_22_31_219(params, &recorded)
            }
            other => panic!("unexpected direct node method {other}: {params}"),
        }
    }

    fn complete_answer_1_repair_22_31_219(&mut self, params: &Value, recorded: &Value) -> Value {
        let mut rows=self.receipts.iter().enumerate().map(|(i,r)|json!({"blockHeight":HEIGHT,"txIndex":i+1,"logIndex":0,"tx_hash":r["txHash"]})).collect::<Vec<_>>();
        match self.attack {
            Attack::Omitted | Attack::PrematureEnd => {
                rows.pop();
            }
            Attack::EmptyPage => rows.clear(),
            Attack::EmptyNextPage => {
                if params[1].is_null()
                    || require_ok(
                        read::Cursor::from_hex(require_some(
                            params[1].as_str(),
                            "required test value",
                        )),
                        "required test value",
                    )
                    .block_height
                        < HEIGHT
                {
                    rows.truncate(1);
                } else {
                    rows.clear();
                }
            }
            _ => {}
        }
        let next = if self.attack == Attack::EmptyNextPage
            && (params[1].is_null()
                || require_ok(
                    read::Cursor::from_hex(require_some(params[1].as_str(), "required test value")),
                    "required test value",
                )
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
        json!({"account":params[0],"items":rows,"next_cursor":next,"indexedHeight":if self.attack==Attack::StaleIndex {HEIGHT-1}else{HEIGHT},"chainHeight":HEIGHT,"freshness":recorded["result"]["freshness"]})
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
    fn endpoint(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
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
        must(require_some(self.worker.take(), "required test value").join());
    }
}

#[test]
fn startup_gateway_node_uses_direct_node_commit_and_rejects_chain_mismatch() {
    let node = Node::start(Attack::None);
    let cp = must(layerx_agentd::human_node::verify_direct_node_readiness(
        &node.endpoint(),
        125,
    ));
    assert_eq!(cp.height, HEIGHT);
    assert_eq!(cp.rung, Rung::Committed);
    assert!(!must(node.state.lock())
        .calls
        .iter()
        .any(|m| m == "px_getNetwork"));
    let other = Node::start(Attack::Chain);
    assert!(
        layerx_agentd::human_node::verify_direct_node_readiness(&other.endpoint(), 125).is_err()
    );
    assert_eq!(must(other.state.lock()).calls, vec!["eth_chainId"]);
}
#[test]
fn checkpoint_request_shape_routes_selector_and_refuses_bad_selector_and_quorum() {
    let node = Node::start(Attack::None);
    let request: CheckpointRequest = must(serde_json::from_value(
        json!({"checkpoint":HEIGHT.to_string(),"requested_verification_level":"committed"}),
    ));
    let encoded = must(serde_json::to_vec(&request));
    let restored: CheckpointRequest = must(serde_json::from_slice(&encoded));
    let value = must(restored.acquire(&node.endpoint(), "125", node.trust().validator_set()));
    assert_eq!(value["value"]["height"], HEIGHT.to_string());
    assert_eq!(value["achieved_verification_level"], "committed");
    assert!(!require_some(value["value"]["commit"].as_str(), "required test value").is_empty());
    assert!(!require_some(
        value["value"]["validatorSet"].as_str(),
        "required test value"
    )
    .is_empty());
    for selector in ["0", "01000", "-1", "1000 ", "18446744073709551616"] {
        let mut bad = request.clone();
        bad.checkpoint = selector.into();
        assert!(bad
            .acquire(&node.endpoint(), "125", node.trust().validator_set())
            .is_err());
    }
    assert!(serde_json::from_value::<CheckpointRequest>(
        json!({"checkpoint":"1000","requested_verification_level":"committed","extra":true})
    )
    .is_err());
    let minority = Node::start(Attack::Quorum);
    assert!(request
        .acquire(
            &minority.endpoint(),
            "125",
            minority.trust().validator_set()
        )
        .is_err());
}
fn actor() -> Did {
    must(Did::new(
        format!("did:layerx:{}", hex::encode([9; 32])).as_bytes(),
    ))
}
#[test]
fn history_commit_inclusion_binds_signed_outcomes_landing_and_timestamp() {
    let node = Node::start(Attack::None);
    let source = must(layerx_agentd::protocol_evidence::PolicyHistorySource::new(
        &node.config(),
        &node.url,
    ));
    let (canonical, checkpoint, key) = {
        let f = must(node.state.lock());
        (
            decode_hex(&f.receipts[0]["receipt"]),
            f.blocks[&(HEIGHT + 1)].clone(),
            f.key,
        )
    };
    let position = read::Cursor {
        block_height: HEIGHT,
        tx_index: 1,
        log_index: 0,
    };
    must(source.prove(&canonical, position, &checkpoint, &[key], &mut 0));
    for attack in [
        Attack::ForeignBlock,
        Attack::LandingIndex,
        Attack::Uncommitted,
    ] {
        must(node.state.lock()).attack = attack;
        assert!(
            source
                .prove(&canonical, position, &checkpoint, &[key], &mut 0)
                .is_err(),
            "{attack:?}"
        );
    }
    let altered = Node::start(Attack::Timestamp);
    let altered_source = must(layerx_agentd::protocol_evidence::PolicyHistorySource::new(
        &altered.config(),
        &altered.url,
    ));
    let altered_checkpoint = must(altered.state.lock()).blocks[&(HEIGHT + 1)].clone();
    assert!(altered_source
        .prove(&canonical, position, &altered_checkpoint, &[key], &mut 0)
        .is_err());
    must(node.state.lock()).attack = Attack::None;
    let mut wrong = position;
    wrong.tx_index = 2;
    assert!(source
        .prove(&canonical, wrong, &checkpoint, &[key], &mut 0)
        .is_err());
}
#[test]
fn history_window_completeness_refuses_omission_end_empty_and_stale_after_restart() {
    use layerx_agentd::policy_history::{read_policy_history_from_sources, PolicyHistoryWindow};
    let actor = actor();
    let node = Node::start(Attack::None);
    let history = must(read_policy_history_from_sources(
        &node.config(),
        &node.url,
        &actor,
        &node.trust(),
        PolicyHistoryWindow::Sequence {
            core: HEIGHT,
            width: 1,
        },
    ));
    let usage = must(history.cumulative(&actor, HEIGHT, HEIGHT));
    assert_eq!(usage.count(), 2);
    assert_eq!(usage.amount(), 1_000_000_000_000_000_000);
    assert!(history.time_window(&actor, 1).is_err());
    drop(history);
    drop(node);
    // Stop and reopen the real producer against the same committed recording.
    let node = Node::start(Attack::None);
    let history = must(read_policy_history_from_sources(
        &node.config(),
        &node.url,
        &actor,
        &node.trust(),
        PolicyHistoryWindow::Time {
            head: HEIGHT,
            seconds: 1,
        },
    ));
    let use_window = must(history.time_window(&actor, 1));
    assert_eq!(use_window.count(), 2);
    assert_eq!(use_window.successful_count(), 2);
    assert!(history.time_window(&actor, 2).is_err());
    assert!(history.cumulative(&actor, HEIGHT, HEIGHT).is_err());
    for attack in [
        Attack::Omitted,
        Attack::PrematureEnd,
        Attack::EmptyPage,
        Attack::EmptyNextPage,
        Attack::StaleIndex,
    ] {
        must(node.state.lock()).attack = attack;
        assert!(
            read_policy_history_from_sources(
                &node.config(),
                &node.url,
                &actor,
                &node.trust(),
                PolicyHistoryWindow::Sequence {
                    core: HEIGHT,
                    width: 1
                }
            )
            .is_err(),
            "{attack:?}"
        );
    }
}

// A committed peer identity observation. Identity resolution is outside the
// repaired component; registration, session persistence and restoration are real.
struct IdentityRecording;
impl identity::IdentityResolver for IdentityRecording {
    fn resolve(
        &mut self,
        _: &Did,
    ) -> Result<Option<identity::CoreIdentity>, identity::IdentityError> {
        Ok(Some(identity::CoreIdentity {
            canonical_bytes: b"recorded-primary-key-identity-v1".to_vec(),
            head_sequence: HEIGHT,
            revocation_sequence: 1,
            verification_level: VerificationLevel::COMMITTED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey([3; 32])],
        }))
    }
}
fn open_session(store: &mut Store, sessions: &mut SessionRegistry, tenant: &TenantId, n: u8) {
    let did = actor();
    let identity = must(identity::register(
        store,
        tenant.clone(),
        did.clone(),
        &mut IdentityRecording,
    ));
    let token = must(session::open(
        store,
        sessions,
        &identity,
        OpenRequest {
            session_id: SessionId([n; 32]),
            token_id: [n + 1; 32],
            tenant: tenant.clone(),
            agent: did,
            authority: ProtocolAuthority::PrimaryKey([3; 32]),
            permitted_activity_types: BTreeSet::from([7]),
            scopes: BTreeSet::from(["prepare".into()]),
            expiry_sequence: HEIGHT + 10000,
            expiry_seconds: Some(NOW / 1000 + 10000),
            opening_client: "repair219".into(),
            policy_version: "v1".into(),
        },
        HEIGHT,
    ));
    assert_eq!(token.credential().generation(), 1);
}
fn owner_word() -> [u8; 32] {
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [0; 20],
        data: vec![],
    };
    let (call, sig) = must(decode_signed(&must(call.sign(&[1; 32]))));
    let mut word = [0; 32];
    word[12..].copy_from_slice(&must(recover_sender(&call, &sig)));
    word
}
fn session_grant() -> layerx_crypto::session::IssuedSessionKey {
    use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
    must(issue_session_key(&SessionKeyRequest {
        grantor: [8; 32],
        session_public_key: [12; 32],
        not_before: HEIGHT,
        expires_at: Some(HEIGHT + 10000),
        permitted_activity_types: vec![must(layerx_types::payload::ActivityType::new(
            layerx_types::payload::ModuleId::Governance,
            6,
        ))],
        revocation_sequence: Some(1),
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    }))
}
fn seed() -> human::HumanAgentLifecycleSeed {
    human::HumanAgentLifecycleSeed {
        agent_id: format!("agt_{}", hex::encode([8; 32])),
        name: "agent".into(),
        purpose: "repair".into(),
        currency: "PAX".into(),
        monthly_limit: 100,
        period_start: 1,
        period_end: NOW / 1000 + 10000,
        created_at: 1,
        updated_at: 1,
        verified_evidence: vec![[1; 32]],
        actor: require_ok(
            String::from_utf8(actor().as_bytes().to_vec()),
            "required test value",
        ),
        primary_authority: hex::encode(owner_word()),
        custody_key: "owner".into(),
        custody_public_key: [9; 32],
        owner_account: format!(
            "agent:{}:main",
            require_ok(
                std::str::from_utf8(actor().as_bytes()),
                "required test value"
            )
        ),
        budget_account: format!(
            "agent:{}:budget:managed",
            require_ok(
                std::str::from_utf8(actor().as_bytes()),
                "required test value"
            )
        ),
        budget_asset: [4; 32],
        purpose_hash: [5; 32],
        recovery_root: [6; 32],
        recovery_threshold: 1,
        capability_id: [7; 32],
        activity_types: vec![7],
        counterparties: vec![[4; 32]],
        assets: vec![[4; 32]],
        amount_ceiling: 100,
        rate_maximum_uses: 10,
        rate_window_sequences: 10,
        purposes: vec!["repair".into()],
        capability_expiry_sequence: HEIGHT + 10000,
        session_scopes: vec!["prepare".into()],
        session_expiry_unix_seconds: NOW / 1000 + 10000,
        protocol_grant_id: session_grant().grant_id,
        budget_period_seconds: 100,
        budget_expiry_seconds: NOW / 1000 + 10000,
        initial_funding: 100,
        network_id: 125,
        creation_receipt_roots: vec![[1; 32]],
    }
}
fn owner() -> (Directory, TenantId, String) {
    let directory = Directory::new();
    let tenant = must(TenantId::new("repair219"));
    let mut store = must(Store::open(&directory.0));
    let mut sessions = SessionRegistry::default();
    open_session(&mut store, &mut sessions, &tenant, 10);
    let seed = seed();
    let agent = must(ManagedAgent::from_creation(
        &seed,
        &seed.actor,
        [10; 32],
        [11; 32],
        1,
    ));
    must(managed_agent::publish_creation(&mut store, &tenant, &agent));
    (directory, tenant, seed.agent_id)
}
fn word(n: u128) -> [u8; 32] {
    let mut w = [0; 32];
    w[16..].copy_from_slice(&n.to_be_bytes());
    w
}
fn replacement(current: [u8; 32], action: [u8; 32]) -> [u8; 32] {
    Sha256::new()
        .chain_update(b"layerx-human/agent-limit-budget/v1")
        .chain_update(current)
        .chain_update(action)
        .finalize()
        .into()
}
fn budget_call(agent: &ManagedAgent, id: [u8; 32], amount: u128, nonce: u64) -> Eip1559Call {
    use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
    let account = must(layerx_types::account::AccountId::parse(
        &agent.context.budget_account,
    ));
    let budget_account = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
    let args = vec![
        id,
        budget_account,
        agent.context.budget_asset,
        agent.context.purpose_hash,
        word(amount),
        word(0),
        word(0),
        word(u128::from(agent.context.budget_period_seconds) * 1000),
        word(u128::from(NOW)),
        word(u128::from(NOW + 100_000_000)),
        word(1),
        word(1),
    ];
    let create=PrecompileCall { target:PrecompileTarget::Pay,
        selector:layerx_crypto::disclosure::call::ledger_selector("budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)"),
        arguments:args.into_iter().map(AbiArgument::Word).collect() };
    Eip1559Call {
        chain_id: 125,
        nonce,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: create.target.address(),
        data: must(create.encode()),
    }
}
fn event(
    address: [u8; 20],
    name: &str,
    id: [u8; 32],
    data: Vec<u8>,
) -> layerx_proof::sequencer_logs::SequencerLog {
    layerx_proof::sequencer_logs::SequencerLog::new(
        address,
        vec![transaction_hash(name.as_bytes()), id],
        data,
        0,
    )
}
fn protocol_receipt(
    raw: &[u8],
    logs: &[layerx_proof::sequencer_logs::SequencerLog],
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let tx = transaction_hash(raw);
    let height = HEIGHT;
    let (call, sig) = must(decode_signed(raw));
    let sender = must(recover_sender(&call, &sig));
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let mut summary = Encoder::new(256);
    must(summary.u64(1));
    must(summary.u64(1));
    must(summary.u64(1));
    must(summary.u32(0));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&hash(b"")));
    must(summary.u32(0));
    must(summary.u32(require_ok(u32::try_from(logs.len()), "test value fits u32")));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut e = Encoder::new(16384);
    must(e.structure_header_version(0x5201, 2));
    must(e.u16(2));
    must(e.bytes(&tx, 32));
    must(e.u64(height));
    must(e.bytes(&[2; 32], 32));
    let seq = hash(
        &[
            b"PAXEER/v1/sequence\0".as_slice(),
            &[2; 32],
            &height.to_be_bytes(),
            &tx,
        ]
        .concat(),
    );
    must(e.bytes(&seq, 32));
    must(e.bytes(&hash(b""), 32));
    must(e.i32(0));
    must(e.sequence_length(1 + logs.len(), 512));
    for (index, body) in std::iter::once(summary.finish())
        .chain(logs.iter().map(|log| {
            let mut body = log.address().to_vec();
            body.push(require_ok(
                u8::try_from(log.topics().len()),
                "test value fits u8",
            ));
            for topic in log.topics() {
                body.extend_from_slice(topic);
            }
            body.extend_from_slice(&hash(log.data()));
            body
        }))
        .enumerate()
    {
        must(e.u16(12));
        must(e.u16(require_ok(u16::try_from(index), "test value fits u16")));
        must(e.u16(0));
        must(e.tag(if index == 0 { 1 } else { 3 }, 3));
        must(e.u8(0));
        must(e.bytes(&[0; 32], 32));
        must(e.bytes(&body, 256));
    }
    must(e.u128(1));
    let mut block = [0; 32];
    block[24..].copy_from_slice(&height.to_be_bytes());
    must(e.bytes(&block, 32));
    must(e.u16(12));
    must(e.u32(1));
    must(e.u32(0));
    must(e.u8(2));
    must(e.bytes(&[0; 32], 32));
    must(e.u128(0));
    must(e.bytes(&from, 32));
    must(e.u128(1000));
    must(e.u128(999));
    must(e.u64(call.nonce));
    must(e.bytes(&to, 32));
    must(e.u128(0));
    must(e.u128(0));
    must(e.bytes(&[0; 32], 32));
    must(e.bytes(&[0; 32], 32));
    must(e.bytes(&[11; 32], 32));
    must(e.u64(NOW));
    must(e.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(e.bytes(&signature, 64));
    }
    e.finish()
}
fn retain_receipt(
    store: &mut Store,
    tenant: &TenantId,
    call: &Eip1559Call,
    action: [u8; 32],
    logs: &[layerx_proof::sequencer_logs::SequencerLog],
) -> HumanFinalizationEvidence {
    let height = HEIGHT;
    let raw = must(call.sign(&[1; 32]));
    let tx = must(Outbox::default().enqueue(store, tenant.clone(), raw.clone()));
    let signer = SigningKey::from_bytes(&[31; 32]);
    let digest = must(receipt_digest(&protocol_receipt(&raw, logs, None)));
    let canonical = protocol_receipt(&raw, logs, Some(signer.sign(&digest).to_bytes()));
    // Both signed log effects and their committed ABCI result are emitted by the peer transcript.
    let peer = json!({"gasUsed":"0x1","vmError":"","returnData":"0x","txHash":rpc_hex(&tx),"logs":logs.iter().map(|log|
        json!({"address":rpc_hex(&log.address()),"topics":log.topics().iter().map(|t|rpc_hex(t)).collect::<Vec<_>>(),
            "data":rpc_hex(log.data()),"logIndex":format!("{:#x}",log.log_index())})).collect::<Vec<_>>()});
    let results = json!([{"data":openssl::base64::encode_block(&abci_response(&peer)),"gas_wanted":"1","gas_used":"1"}]);
    let inclusion = must(NativeResultSource::from_results(&results, 0));
    let root = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[&inclusion.result]);
    let checkpoint = committed(
        height + 1,
        require_ok(i64::try_from(NOW), "test value fits i64") / 1000,
        [1; 32],
        root,
        4,
    );
    let key = ReceiptKeyEntry {
        key_id: [11; 32],
        public_key: signer.verifying_key().to_bytes(),
        first_height: height,
        last_height: height,
    };
    must(receipt::persist_committed_protocol_receipt(
        store,
        tenant.clone(),
        &canonical,
        &[key],
        height,
        0,
        &inclusion,
        &checkpoint,
        "125",
        &checkpoint.validators,
    ));
    HumanFinalizationEvidence {
        action_key: action,
        tx_hash: tx,
        receipt_hash: hash(&canonical),
        observed_height: height,
        rung: Rung::Committed.wire_rank(),
        finalized_at: NOW / 1000 + call.nonce,
    }
}
enum LifecycleCall {
    Suspend,
    Resume,
    Archive,
    KeyChange {
        ready_at: u64,
    },
    Limit {
        budget: [u8; 32],
        amount: u128,
    },
    Reclaim {
        budget: [u8; 32],
        amount: u128,
        limit: u128,
    },
}

// Produce the signed call and its exact sequencer log effects before retaining the
// committed receipt. The lifecycle consumer restores and verifies all of them.
fn evidence(
    store: &mut Store,
    tenant: &TenantId,
    action: u8,
    operation: LifecycleCall,
) -> HumanFinalizationEvidence {
    let agent = must(ManagedAgent::from_creation(
        &seed(),
        &seed().actor,
        [10; 32],
        [11; 32],
        1,
    ));
    let mut logs = Vec::new();
    let call = match operation {
        LifecycleCall::Limit { budget, amount } => {
            let call = budget_call(&agent, budget, amount, u64::from(action));
            logs.push(event(
                call.to,
                "BudgetCreated(bytes32,bytes32,uint128)",
                budget,
                [agent.context.budget_asset.to_vec(), word(0).to_vec()].concat(),
            ));
            call
        }
        operation => {
            let precompile = match operation {
                LifecycleCall::Suspend | LifecycleCall::Archive => {
                    let reason = if matches!(operation, LifecycleCall::Archive) {
                        2
                    } else {
                        1
                    };
                    finish_evidence_1_repair_22_31_219(&agent, &mut logs, reason)
                }
                LifecycleCall::Resume => {
                    let grant = session_grant();
                    finish_evidence_3_repair_22_31_219(action, &mut logs, grant)
                }
                LifecycleCall::KeyChange { ready_at } => {
                    finish_evidence_2_repair_22_31_219(action, &agent, ready_at)
                }
                LifecycleCall::Reclaim {
                    budget,
                    amount,
                    limit,
                } => {
                    let data = [word(amount).to_vec(), word(limit - amount).to_vec()].concat();
                    logs.push(event(
                        PrecompileTarget::Pay.address(),
                        "BudgetDefunded(bytes32,uint128,uint128)",
                        budget,
                        data,
                    ));
                    PrecompileCall {
                        target: PrecompileTarget::Pay,
                        selector: layerx_crypto::disclosure::call::ledger_selector(
                            "budgetDefund(bytes32,uint128)",
                        ),
                        arguments: vec![AbiArgument::Word(budget), AbiArgument::Word(word(amount))],
                    }
                }
                LifecycleCall::Limit { .. } => unreachable!(),
            };
            Eip1559Call {
                chain_id: 125,
                nonce: action.into(),
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                gas_limit: 100_000,
                to: precompile.target.address(),
                data: must(precompile.encode()),
            }
        }
    };
    let evidence = retain_receipt(store, tenant, &call, [action; 32], &logs);
    assert_ne!(evidence.action_key, evidence.tx_hash);
    evidence
}

fn finish_evidence_3_repair_22_31_219(
    action: u8,
    logs: &mut Vec<layerx_proof::sequencer_logs::SequencerLog>,
    grant: layerx_crypto::session::IssuedSessionKey,
) -> PrecompileCall {
    let body = must(
        PrecompileCall {
            target: PrecompileTarget::Auth,
            selector: [0; 4],
            arguments: vec![AbiArgument::Bytes(grant.registration_payload.clone())],
        }
        .encode(),
    );
    logs.push(event(
        PrecompileTarget::Auth.address(),
        "SessionGranted(bytes32,bytes)",
        grant.grant_id,
        body[4..].to_vec(),
    ));
    PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: layerx_crypto::disclosure::call::ledger_selector(
            "grantSession(bytes,uint64,bytes32)",
        ),
        arguments: vec![
            AbiArgument::Bytes(grant.registration_payload),
            AbiArgument::Word(word(1)),
            AbiArgument::Word([action; 32]),
        ],
    }
}

fn finish_evidence_2_repair_22_31_219(
    action: u8,
    agent: &ManagedAgent,
    ready_at: u64,
) -> PrecompileCall {
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

    let did = must(Did::new(agent.context.actor.as_bytes()));
    let intent = layerx_intents::Intent::v3(layerx_intents::IntentKind::KeyRotation(must(
        layerx_intents::KeyRotation::new(
            did,
            layerx_types::intent::PublicKey::new([53; 32]),
            must(layerx_types::activity::TimestampBound::new(
                NOW / 1000 + u64::from(action),
                ready_at,
            )),
            layerx_types::intent::Sequence::from_u64(HEIGHT),
        ),
    )));

    let activity = must(ActivityType::new(ModuleId::Governance, 2));
    let registration = must(ModuleRegistration::new(ModuleId::Governance, &[activity]));
    let registry = must(ModuleRegistry::new(&[registration]));
    let compiled = must(layerx_intents::compile(&intent, &registry));
    must(compiled.verify_disclosure(&intent));
    let payload = compiled.payload().as_bytes().to_vec();
    PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: layerx_crypto::disclosure::call::ledger_selector(
            "recordLifecycle(bytes,bytes32)",
        ),
        arguments: vec![AbiArgument::Bytes(payload), AbiArgument::Word([action; 32])],
    }
}

fn finish_evidence_1_repair_22_31_219(
    agent: &ManagedAgent,
    logs: &mut Vec<layerx_proof::sequencer_logs::SequencerLog>,
    reason: u8,
) -> PrecompileCall {
    let revoked = [
        agent.protocol_grant_id.to_vec(),
        vec![reason],
        HEIGHT.to_be_bytes().to_vec(),
    ]
    .concat();
    let body = must(
        PrecompileCall {
            target: PrecompileTarget::Auth,
            selector: [0; 4],
            arguments: vec![AbiArgument::Bytes(revoked)],
        }
        .encode(),
    );
    logs.push(event(
        PrecompileTarget::Auth.address(),
        "GrantRevoked(bytes32,bytes)",
        agent.protocol_grant_id,
        body[4..].to_vec(),
    ));
    PrecompileCall {
        target: PrecompileTarget::Auth,
        selector: layerx_crypto::disclosure::call::ledger_selector(
            "revokeGrant(bytes32,uint8,uint64)",
        ),
        arguments: vec![
            AbiArgument::Word(agent.protocol_grant_id),
            AbiArgument::Word(word(reason.into())),
            AbiArgument::Word(word(HEIGHT.into())),
        ],
    }
}
fn socket_exchange(
    request: &HumanRequest,
    handler: impl FnOnce(HumanRequest) -> Result<human::HumanResponse, HumanOperationError>
        + Send
        + 'static,
) -> HumanWireResponse {
    use human::frame::HumanFrameTransport;
    let bytes = must(human::encode_lifecycle_request(request));
    let (a, b) = must(UnixStream::pair());
    let worker = thread::spawn(move || {
        let mut transport = must(human::AcceptedTransport::new(
            a,
            1_048_576,
            Duration::from_secs(3),
        ));
        let peer = HumanPeer {
            uid: 1000,
            principal: "human".into(),
            tenant: "repair219".into(),
            subject: None,
        };
        must(human::serve_one_with(&mut transport, &peer, handler));
    });
    let mut client = must(human::AcceptedTransport::new(
        b,
        1_048_576,
        Duration::from_secs(3),
    ));
    must(client.send(&bytes));
    let result = must(human::decode_response(must(client.receive())));
    must(worker.join());
    result
}
fn lifecycle(
    directory: &Directory,
    tenant: &TenantId,
    request: &HumanRequest,
) -> HumanWireResponse {
    let path = directory.0.clone();
    let tenant = tenant.clone();
    socket_exchange(request, move |request| {
        let mut store = require_ok(Store::open(&path), "required test value");
        managed_agent::apply_lifecycle_request(&mut store, &tenant, request)
    })
}
fn agent_state(bytes: &[u8]) -> u8 {
    let mut offset = 0;
    for _ in 0..3 {
        let n = u32::from_be_bytes(require_ok(
            bytes[offset..offset + 4].try_into(),
            "required test value",
        )) as usize;
        offset += 4 + n;
    }
    bytes[offset]
}
fn success(result: HumanWireResponse) -> Vec<u8> {
    match result {
        HumanWireResponse::Success(value) => value.bytes().to_vec(),
        other => panic!("expected success: {other:?}"),
    }
}
fn refused(result: &HumanWireResponse) {
    assert!(
        matches!(result, HumanWireResponse::Refused(_)),
        "{result:?}"
    );
}
fn observe_budget(
    store: &mut Store,
    tenant: &TenantId,
    budget: [u8; 32],
    tag: u8,
    remaining: u128,
) -> [u8; 32] {
    let state = layerx_agentd::human_runtime::CoreBudgetState {
        revocation_sequence: 1,
        observed_head_sequence: HEIGHT,
        verification: VerificationLevel::COMMITTED.wire_rank(),
        evidence_digest: [tag; 32],
        receipt_digest: [2; 32],
        checkpoint_digest: [3; 32],
        age_sequences: 0,
        maximum_age_sequences: 10,
        remaining,
        asset: [4; 32],
    };
    must(managed_agent::record_budget_snapshot(
        store, tenant, budget, &state,
    ));
    state.evidence_digest
}
#[test]
fn finalization_rungs_and_session_observation_shape_survive_suspend_resume_archive_reopen() {
    let (directory, tenant, id) = owner();
    let mut store = must(Store::open(&directory.0));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant));
    let action = 21;
    let observation = must(managed_agent::suspend_session(
        &mut store,
        &mut sessions,
        &tenant,
        &id,
        [action; 32],
    ));
    let observation = must(SessionObservationV1::decode(observation.bytes()));
    assert_eq!(observation.generation, 2);
    assert!(!observation.open);
    let mut bad = observation.clone();
    bad.generation = 1;
    let (bytes, digest) = must(bad.encode());
    must(managed_agent::record_observation(
        &mut store,
        &tenant,
        3,
        digest,
        bytes.bytes().to_vec(),
    ));
    let evidence = evidence(&mut store, &tenant, action, LifecycleCall::Suspend);
    drop(store);
    let request = |obs, evidence| HumanRequest::AgentControl {
        agent_id: id.clone(),
        resume: false,
        session_observation: obs,
        evidence,
    };
    refused(&lifecycle(&directory, &tenant, &request(digest, evidence)));
    let mut below = evidence;
    below.rung = Rung::Receipted.wire_rank();
    refused(&lifecycle(
        &directory,
        &tenant,
        &request(observation.digest, below),
    ));
    let first = success(lifecycle(
        &directory,
        &tenant,
        &request(observation.digest, evidence),
    ));
    assert_eq!(agent_state(&first), 2);
    assert_eq!(
        first,
        success(lifecycle(
            &directory,
            &tenant,
            &request(observation.digest, evidence)
        ))
    );
    let store = must(Store::open(&directory.0));
    assert_eq!(
        must(receipt::serve_committed_evidence(
            &store,
            tenant.clone(),
            evidence.tx_hash
        ))
        .metadata
        .rung,
        Rung::Committed
    );
    let sessions = SessionRegistry::default();
    finish_finalization_rungs_and_session_observation_shape_survive_suspend_resume_archive_reopen_1_repair_22_31_219(&directory, &tenant, &id, store, sessions);
}

fn finish_finalization_rungs_and_session_observation_shape_survive_suspend_resume_archive_reopen_1_repair_22_31_219(
    directory: &Directory,
    tenant: &TenantId,
    id: &str,
    mut store: Store,
    mut sessions: SessionRegistry,
) {
    must(sessions.restore_tenant(&store, tenant));
    must(managed_agent::validate_session_coordinates(
        &store, &sessions, tenant,
    ));
    assert_eq!(sessions.generation(tenant, SessionId([10; 32])), Some(2));
    open_session(&mut store, &mut sessions, tenant, 12);
    let resume = must(managed_agent::bind_session(
        &mut store,
        tenant,
        id,
        ([12; 32], [13; 32], 1, session_grant().grant_id),
        [22; 32],
    ));
    let resume = must(SessionObservationV1::decode(resume.bytes()));
    let resume_evidence = self::evidence(&mut store, tenant, 22, LifecycleCall::Resume);
    drop(store);
    let resumed = success(lifecycle(
        directory,
        tenant,
        &HumanRequest::AgentControl {
            agent_id: id.to_owned(),
            resume: true,
            session_observation: resume.digest,
            evidence: resume_evidence,
        },
    ));
    assert_eq!(agent_state(&resumed), 1);
    let mut store = must(Store::open(&directory.0));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, tenant));
    let closed = must(managed_agent::suspend_session(
        &mut store,
        &mut sessions,
        tenant,
        id,
        [23; 32],
    ));
    let _closed = must(SessionObservationV1::decode(closed.bytes()));
    let pre = observe_budget(&mut store, tenant, [8; 32], 31, 100);
    let post = observe_budget(&mut store, tenant, [8; 32], 32, 0);
    let (prepared, key, bytes) = must(managed_agent::prepare_session_observation(
        &store, tenant, id, [24; 32], false,
    ));
    assert_eq!(prepared.bytes(), bytes);
    must(store.put_local(key, bytes));
    let closed = must(SessionObservationV1::decode(prepared.bytes()));
    let archive_evidence = self::evidence(&mut store, tenant, 24, LifecycleCall::Archive);
    drop(store);
    success(lifecycle(
        directory,
        tenant,
        &HumanRequest::AgentArchive {
            agent_id: id.to_owned(),
            confirm_name: "agent".into(),
            pre_observation: pre,
            post_observation: post,
            session_observation: closed.digest,
            evidence: archive_evidence,
        },
    ));
    let store = must(Store::open(&directory.0));
    assert_eq!(
        agent_state(must(managed_agent::get(&store, tenant, id)).bytes()),
        4
    );
}
#[test]
fn journey_wire_rotate_recover_bind_persisted_key_policy_over_socket() {
    assert_persisted_key_policy_rank_over_socket(VerificationLevel::COMMITTED.wire_rank());
}

pub(crate) fn assert_persisted_key_policy_rank_over_socket(policy_rank: u8) {
    for (recovery, action) in [(false, 41), (true, 42)] {
        let (directory, tenant, id) = owner();
        let mut store = must(Store::open(&directory.0));
        let did = seed().actor;
        let policy = layerx_agentd::human_runtime::CoreKeyPolicy {
            policy_revision: 1,
            required_delay_seconds: 10,
            maximum_delay_seconds: 20,
            effective_sequence: 1,
            observed_head_sequence: HEIGHT,
            verification: policy_rank,
            evidence_digest: [51; 32],
            checkpoint_digest: [52; 32],
            age_sequences: 0,
            maximum_age_sequences: 10,
        };
        must(managed_agent::record_key_policy_snapshot(
            &mut store, &tenant, &did, recovery, &policy,
        ));
        let evidence = evidence(
            &mut store,
            &tenant,
            action,
            LifecycleCall::KeyChange {
                ready_at: NOW / 1000 + u64::from(action) + 10,
            },
        );
        drop(store);
        let store = must(Store::open(&directory.0));
        let validation = managed_agent::validate_authority_revocation(
            &store,
            &tenant,
            &id,
            (if recovery { 2 } else { 1 }, 10, evidence.finalized_at + 10),
            policy.evidence_digest,
            [0; 32],
            evidence.into(),
        );
        if let ControlFlow::Break(()) =
            finish_assert_persisted_key_policy_rank_over_socket_1_repair_22_31_219(
                FinishAssertPersistedKeyPolicyRankOverSocket1Repair2231219Context {
                    policy_rank,
                    recovery,
                    directory,
                    tenant,
                    id,
                    policy,
                    evidence,
                    store,
                    validation,
                },
            )
        {}
    }
}

struct FinishAssertPersistedKeyPolicyRankOverSocket1Repair2231219Context {
    policy_rank: u8,
    recovery: bool,
    directory: Directory,
    tenant: TenantId,
    id: String,
    policy: layerx_agentd::human_runtime::CoreKeyPolicy,
    evidence: HumanFinalizationEvidence,
    store: Store,
    validation: Result<managed_agent::ValidatedAuthorityRevocation, HumanOperationError>,
}

fn finish_assert_persisted_key_policy_rank_over_socket_1_repair_22_31_219(
    context: FinishAssertPersistedKeyPolicyRankOverSocket1Repair2231219Context,
) -> ControlFlow<()> {
    let FinishAssertPersistedKeyPolicyRankOverSocket1Repair2231219Context {
        policy_rank,
        recovery,
        directory,
        tenant,
        id,
        policy,
        evidence,
        store,
        validation,
    } = context;

    if !matches!(policy_rank, 4..=6) {
        let check = if VerificationLevel::from_wire_rank(policy_rank).is_err() {
            ManagedAgentCheck::KeyPolicyVerification
        } else {
            ManagedAgentCheck::KeyPolicyObservationBinding
        };
        assert_eq!(
            validation.err(),
            Some(HumanOperationError::Typed(HumanRefusal::ManagedAgent(
                check
            ))),
            "rank {policy_rank}"
        );
        return ControlFlow::Break(());
    }
    must(validation);
    drop(store);
    let kind = if recovery {
        HumanAgentJourneyKind::Recover {
            challenge_delay_seconds: 10,
            ready_at: evidence.finalized_at + 10,
        }
    } else {
        HumanAgentJourneyKind::Rotate {
            challenge_delay_seconds: 10,
            ready_at: evidence.finalized_at + 10,
        }
    };
    let request = HumanRequest::AgentJourney {
        agent_id: id.clone(),
        kind,
        pre_observation: policy.evidence_digest,
        post_observation: [0; 32],
        evidence,
    };
    let encoded = must(human::encode_lifecycle_request(&request));
    assert_eq!(&encoded[9..11], b"J1");
    assert!(matches!(
        must(human::decode_request(encoded)),
        HumanRequest::AgentJourney { .. }
    ));
    let first = success(lifecycle(&directory, &tenant, &request));
    assert_eq!(first, success(lifecycle(&directory, &tenant, &request)));
    let mut absent = must(human::decode_request(must(
        human::encode_lifecycle_request(&request),
    )));
    if let HumanRequest::AgentJourney {
        pre_observation, ..
    } = &mut absent
    {
        *pre_observation = [53; 32];
    }
    assert!(!matches!(
        lifecycle(&directory, &tenant, &absent),
        HumanWireResponse::Success(_)
    ));
    let store = must(Store::open(&directory.0));
    must(receipt::serve_committed_evidence(
        &store,
        tenant.clone(),
        evidence.tx_hash,
    ));
    ControlFlow::Continue(())
}
#[test]
fn finalization_limit_and_reclaim_use_committed_receipts_and_independent_action_keys() {
    let (directory, tenant, id) = owner();
    let mut store = must(Store::open(&directory.0));
    let budget = replacement([8; 32], [61; 32]);
    let limit = evidence(
        &mut store,
        &tenant,
        61,
        LifecycleCall::Limit {
            budget,
            amount: 200,
        },
    );
    drop(store);
    success(lifecycle(
        &directory,
        &tenant,
        &HumanRequest::AgentLimit {
            agent_id: id.clone(),
            monthly_limit: 200,
            currency: "PAX".into(),
            replacement_budget_id: budget,
            evidence: limit,
        },
    ));
    let mut store = must(Store::open(&directory.0));
    let pre = observe_budget(&mut store, &tenant, budget, 63, 100);
    let post = observe_budget(&mut store, &tenant, budget, 64, 50);
    let reclaim = evidence(
        &mut store,
        &tenant,
        65,
        LifecycleCall::Reclaim {
            budget,
            amount: 50,
            limit: 200,
        },
    );
    drop(store);
    success(lifecycle(
        &directory,
        &tenant,
        &HumanRequest::AgentJourney {
            agent_id: id,
            kind: HumanAgentJourneyKind::Reclaim {
                amount: 50,
                currency: "PAX".into(),
            },
            pre_observation: pre,
            post_observation: post,
            evidence: reclaim,
        },
    ));
}
#[test]
fn human_refusal_wire_preserves_budget_session_capability_export_policy_and_nonce() {
    let request = HumanRequest::AgentLimit {
        agent_id: seed().agent_id,
        monthly_limit: 100,
        currency: "PAX".into(),
        replacement_budget_id: [1; 32],
        evidence: HumanFinalizationEvidence {
            action_key: [1; 32],
            tx_hash: [2; 32],
            receipt_hash: [3; 32],
            observed_height: HEIGHT,
            rung: 2,
            finalized_at: NOW / 1000,
        },
    };
    let reasons = [
        HumanOperationError::Typed(HumanRefusal::BudgetLimitExceeded),
        HumanOperationError::Typed(HumanRefusal::SessionExpired),
        HumanOperationError::Typed(HumanRefusal::ExportLevelUnattainable),
        HumanOperationError::Typed(HumanRefusal::PolicyContextMismatch(
            human::PolicyContextField::AgentDid,
        )),
        HumanOperationError::Typed(HumanRefusal::StaleNonce {
            expected: 7,
            pending: 9,
        }),
    ];
    for error in reasons {
        let HumanOperationError::Typed(typed) = error else {
            unreachable!()
        };
        let response = socket_exchange(&request, move |_| Err(error));
        let HumanWireResponse::Refused(refusal) = response else {
            panic!("{response:?}")
        };
        assert_eq!(refusal.reason, typed.reason());
        assert_eq!(
            refusal.nonce,
            if matches!(typed, HumanRefusal::StaleNonce { .. }) {
                Some((7, 9))
            } else {
                None
            }
        );
        let mut trailing = human::encode_response(Err(error));
        trailing.push(0);
        assert!(human::decode_response(trailing).is_err());
    }
    let dimension = layerx_agentd::capability::Dimension::Amount;
    let response = socket_exchange(&request, move |_| {
        Err(HumanOperationError::CapabilityRefused(dimension))
    });
    let HumanWireResponse::Refused(refusal) = response else {
        panic!("{response:?}")
    };
    assert_eq!(refusal.reason, human::capability_refusal_reason(dimension));
}
