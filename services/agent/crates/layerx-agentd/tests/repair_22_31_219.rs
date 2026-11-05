//! Task 22.31.219: production codecs, socket transport, durable owners and native proofs.
//! Finding/test mapping and cross-domain wiring are in src/repair_22_31_219.kvx.
use ed25519_dalek::{Signer, SigningKey};
use layerx_agentd::{
    human::{
        self, HumanAgentJourneyKind, HumanFinalizationEvidence, HumanOperationError, HumanPeer,
        HumanRefusal, HumanRequest, HumanWireResponse,
    },
    identity::{self, ProtocolAuthority},
    managed_agent::{self, ManagedAgent, SessionObservationV1},
    read::{
        self, BlockCommit, BlockId, CheckpointHeader, CheckpointRequest, CommitTimestamp,
        CommitValidator, CommitVote, NativeResultSource, ServedCheckpoint,
    },
    receipt,
    session::{self, OpenRequest, SessionId, SessionRegistry},
    store::{Store, TenantId},
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_types::{ids::Did, rung::Rung, verify::VerificationLevel};
use layerx_wire::{encode::Encoder, hash::receipt_digest};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::{BTreeMap, BTreeSet},
    io::{BufRead, BufReader, Read, Write},
    net::TcpListener,
    os::unix::net::UnixStream,
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
    must(hex::decode(v.as_str().unwrap().strip_prefix("0x").unwrap()))
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let p = std::env::temp_dir().join(format!(
            "repair219-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::SeqCst)
        ));
        must(std::fs::create_dir(&p));
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
    let time = if h.time.seconds == NOW as i64 / 1000 - 1 {
        "2025-06-15T15:13:19Z"
    } else {
        "2025-06-15T15:13:20Z"
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
        let receipts = recording["cases"].as_array().unwrap()[..2]
            .iter()
            .map(|case| case["receiptJson"].clone())
            .collect::<Vec<_>>();
        let pinned = &recording["cases"][0]["pinnedKey"];
        let key = ReceiptKeyEntry {
            key_id: decode_hex(&pinned["keyId"]).try_into().unwrap(),
            public_key: decode_hex(&pinned["publicKey"]).try_into().unwrap(),
            first_height: 1000,
            last_height: 1999,
        };
        let results = json!([{}, {"data":openssl::base64::encode_block(&abci_response(&receipts[0])),"gas_wanted":"60000","gas_used":"21000"},
            {"data":openssl::base64::encode_block(&abci_response(&receipts[1])),"gas_wanted":"60000","gas_used":"46220"}]);
        let leaves = (0..3)
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
            "pax_sequencerReceipt" => self
                .receipts
                .iter()
                .find(|r| r["txHash"] == params[0])
                .unwrap()
                .clone(),
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
                match self.attack {
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
                            rows.truncate(1);
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
                json!({"items":rows,"next_cursor":next,"indexedHeight":if self.attack==Attack::StaleIndex {HEIGHT-1}else{HEIGHT}})
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
        must(self.worker.take().unwrap().join());
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
    assert!(!value["value"]["commit"].as_str().unwrap().is_empty());
    assert!(!value["value"]["validatorSet"].as_str().unwrap().is_empty());
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
            verification_level: VerificationLevel::CHECKPOINT_FINALISED,
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
        actor: String::from_utf8(actor().as_bytes().to_vec()).unwrap(),
        primary_authority: hex::encode([3; 32]),
        custody_key: "owner".into(),
        custody_public_key: [3; 32],
        owner_account: "owner".into(),
        budget_account: "budget".into(),
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
        protocol_grant_id: [7; 32],
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
// The peer encodes a real signed protocol receipt; the production store must
// verify its signature, native result inclusion and Comet quorum before promotion.
fn protocol_receipt(tx: [u8; 32], module: u16, op: u8, signature: Option<[u8; 64]>) -> Vec<u8> {
    let mut e = Encoder::new(4096);
    must(e.structure_header_version(0x5201, 2));
    must(e.u16(2));
    must(e.bytes(&tx, 32));
    must(e.u64(HEIGHT));
    for root in [[2; 32], [3; 32], [4; 32]] {
        must(e.bytes(&root, 32));
    }
    must(e.i32(0));
    must(e.sequence_length(0, 512));
    must(e.u128(1));
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    must(e.bytes(&height, 32));
    must(e.u16(module));
    must(e.u32(1));
    must(e.u32(1));
    must(e.u8(op));
    must(e.bytes(&[6; 32], 32));
    must(e.u128(1));
    must(e.bytes(&[7; 32], 32));
    must(e.u128(100));
    must(e.u128(99));
    must(e.u64(7));
    must(e.bytes(&[8; 32], 32));
    must(e.u128(0));
    must(e.u128(1));
    for root in [[9; 32], [10; 32], [11; 32]] {
        must(e.bytes(&root, 32));
    }
    must(e.u64(NOW));
    must(e.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(e.bytes(&signature, 64));
    }
    e.finish()
}
fn evidence(
    store: &mut Store,
    tenant: &TenantId,
    action: u8,
    module: u16,
    op: u8,
) -> HumanFinalizationEvidence {
    let tx = hash(&[action, 42]);
    let signer = SigningKey::from_bytes(&[31; 32]);
    let digest = must(receipt_digest(&protocol_receipt(tx, module, op, None)));
    let canonical = protocol_receipt(tx, module, op, Some(signer.sign(&digest).to_bytes()));
    let peer =
        json!({"gasUsed":"0x1","vmError":"","returnData":"0x","txHash":rpc_hex(&tx),"logs":[]});
    let results = json!([{"data":openssl::base64::encode_block(&abci_response(&peer)),"gas_wanted":"1","gas_used":"1"}]);
    let inclusion = must(NativeResultSource::from_results(&results, 0));
    let root =
        layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[inclusion.result.as_slice()]);
    let checkpoint = committed(HEIGHT + 1, NOW as i64 / 1000, [1; 32], root, 4);
    let key = ReceiptKeyEntry {
        key_id: [11; 32],
        public_key: signer.verifying_key().to_bytes(),
        first_height: HEIGHT,
        last_height: HEIGHT,
    };
    let record = must(receipt::persist_committed_protocol_receipt(
        store,
        tenant.clone(),
        &canonical,
        &[key],
        HEIGHT,
        0,
        &inclusion,
        &checkpoint,
        "125",
        &checkpoint.validators,
    ));
    assert_eq!(record.metadata.rung, Rung::Committed);
    assert_ne!([action; 32], tx);
    HumanFinalizationEvidence {
        action_key: [action; 32],
        tx_hash: tx,
        receipt_hash: hash(&canonical),
        observed_height: HEIGHT,
        rung: Rung::Committed.wire_rank(),
        finalized_at: NOW / 1000 + u64::from(action),
    }
}
fn socket_exchange(
    request: &HumanRequest,
    handler: impl FnOnce(HumanRequest) -> Result<human::HumanResponse, HumanOperationError>
        + Send
        + 'static,
) -> HumanWireResponse {
    use human::frame::HumanFrameTransport;
    let bytes = must(human::encode_lifecycle_request(&request));
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
        let mut store = Store::open(&path).unwrap();
        managed_agent::apply_lifecycle_request(&mut store, &tenant, request)
    })
}
fn agent_state(bytes: &[u8]) -> u8 {
    let mut offset = 0;
    for _ in 0..3 {
        let n = u32::from_be_bytes(bytes[offset..offset + 4].try_into().unwrap()) as usize;
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
fn refused(result: HumanWireResponse) {
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
        verification: 4,
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
    let evidence = evidence(&mut store, &tenant, action, 7, 6);
    drop(store);
    let request = |obs, evidence| HumanRequest::AgentControl {
        agent_id: id.clone(),
        resume: false,
        session_observation: obs,
        evidence,
    };
    refused(lifecycle(&directory, &tenant, &request(digest, evidence)));
    let mut below = evidence;
    below.rung = Rung::Receipted.wire_rank();
    refused(lifecycle(
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
    let mut store = must(Store::open(&directory.0));
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
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant));
    must(managed_agent::validate_session_coordinates(
        &store, &sessions, &tenant,
    ));
    assert_eq!(sessions.generation(&tenant, SessionId([10; 32])), Some(2));
    open_session(&mut store, &mut sessions, &tenant, 12);
    let resume = must(managed_agent::bind_session(
        &mut store,
        &tenant,
        &id,
        ([12; 32], [13; 32], 1, [7; 32]),
        [22; 32],
    ));
    let resume = must(SessionObservationV1::decode(resume.bytes()));
    let resume_evidence = self::evidence(&mut store, &tenant, 22, 7, 6);
    drop(store);
    let resumed = success(lifecycle(
        &directory,
        &tenant,
        &HumanRequest::AgentControl {
            agent_id: id.clone(),
            resume: true,
            session_observation: resume.digest,
            evidence: resume_evidence,
        },
    ));
    assert_eq!(agent_state(&resumed), 1);
    let mut store = must(Store::open(&directory.0));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant));
    let closed = must(managed_agent::suspend_session(
        &mut store,
        &mut sessions,
        &tenant,
        &id,
        [23; 32],
    ));
    let closed = must(SessionObservationV1::decode(closed.bytes()));
    let pre = observe_budget(&mut store, &tenant, [8; 32], 31, 100);
    let post = observe_budget(&mut store, &tenant, [8; 32], 32, 0);
    let (prepared, key, bytes) = must(managed_agent::prepare_session_observation(
        &store, &tenant, &id, [24; 32], false,
    ));
    assert_eq!(prepared.bytes(), bytes);
    must(store.put_local(key, bytes));
    let closed = must(SessionObservationV1::decode(prepared.bytes()));
    let archive_evidence = self::evidence(&mut store, &tenant, 24, 7, 6);
    drop(store);
    success(lifecycle(
        &directory,
        &tenant,
        &HumanRequest::AgentArchive {
            agent_id: id.clone(),
            confirm_name: "agent".into(),
            pre_observation: pre,
            post_observation: post,
            session_observation: closed.digest,
            evidence: archive_evidence,
        },
    ));
    let store = must(Store::open(&directory.0));
    assert_eq!(
        agent_state(must(managed_agent::get(&store, &tenant, &id)).bytes()),
        4
    );
}
#[test]
fn journey_wire_rotate_recover_bind_persisted_key_policy_over_socket() {
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
            verification: 4,
            evidence_digest: [51; 32],
            checkpoint_digest: [52; 32],
            age_sequences: 0,
            maximum_age_sequences: 10,
        };
        must(managed_agent::record_key_policy_snapshot(
            &mut store, &tenant, &did, recovery, &policy,
        ));
        let evidence = evidence(&mut store, &tenant, action, 7, 2);
        must(managed_agent::validate_authority_revocation(
            &store,
            &tenant,
            &id,
            (if recovery { 2 } else { 1 }, 10, evidence.finalized_at + 10),
            policy.evidence_digest,
            [0; 32],
            evidence.into(),
        ));
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
    }
}
#[test]
fn finalization_limit_and_reclaim_use_committed_receipts_and_independent_action_keys() {
    let (directory, tenant, id) = owner();
    let mut store = must(Store::open(&directory.0));
    let limit = evidence(&mut store, &tenant, 61, 3, 1);
    drop(store);
    success(lifecycle(
        &directory,
        &tenant,
        &HumanRequest::AgentLimit {
            agent_id: id.clone(),
            monthly_limit: 200,
            currency: "PAX".into(),
            replacement_budget_id: [62; 32],
            evidence: limit,
        },
    ));
    let mut store = must(Store::open(&directory.0));
    let pre = observe_budget(&mut store, &tenant, [62; 32], 63, 100);
    let post = observe_budget(&mut store, &tenant, [62; 32], 64, 50);
    let reclaim = evidence(&mut store, &tenant, 65, 3, 7);
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
