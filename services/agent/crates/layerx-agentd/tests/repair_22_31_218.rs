fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::thread;
use std::time::Duration;

use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agentd::read::{
    AbciResultProof, CommitValidator, HistoryLimits, HistoryPage, NativeProofRequest,
    NativeReadRoute, NativeResultSource,
};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_programs::hex;
use layerx_types::rung::Rung;
use serde_json::{json, Value};

#[test]
fn proof_root_algorithm_uses_node_emitted_abci_bytes() {
    let vector: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../consensus/types/testdata/results_hash_v1.json"
        )),
        "required test value",
    );
    let root = require_ok(
        hex::decode_digest(require_some(
            vector["abci_results_hash"].as_str(),
            "required test value",
        )),
        "required test value",
    );
    // Values are the exact producer vector, converted only from its documented
    // hexadecimal representation into Comet RPC's base64 representation.
    let results = json!([
        {"code":0,"data":"AAH/","gas_wanted":"100","gas_used":"73"},
        {"code":7,"data":"ZGVuaWVk","gas_wanted":"200","gas_used":"150"},
        {"code":0,"data":"","gas_wanted":"0","gas_used":"0"}
    ]);
    for (index, emitted) in require_some(vector["results"].as_array(), "required test value")
        .iter()
        .enumerate()
    {
        let proof = require_ok(
            NativeResultSource::from_results(&results, index as u64),
            "required test value",
        );
        assert_eq!(
            proof.result,
            require_ok(
                hex::decode(require_some(
                    emitted["protobuf"].as_str(),
                    "required test value"
                )),
                "required test value"
            )
        );
        assert_eq!(
            proof.proof.leaf_hash,
            require_ok(
                hex::decode_digest(require_some(
                    emitted["proof"]["leaf_hash"].as_str(),
                    "required test value"
                )),
                "required test value"
            )
        );
        let aunts = require_some(emitted["proof"]["aunts"].as_array(), "required test value")
            .iter()
            .map(|aunt| {
                require_ok(
                    hex::decode_digest(require_some(aunt.as_str(), "required test value")),
                    "required test value",
                )
            })
            .collect::<Vec<_>>();
        assert_eq!(proof.proof.aunts, aunts);
        require_ok(proof.verify(root, index as u64), "required test value");
        let mut recovered = require_ok(
            AbciResultProof::decode(&require_ok(proof.encode(), "required test value")),
            "required test value",
        );
        assert_eq!(proof, recovered);
        recovered.result.push(0);
        assert!(recovered.verify(root, index as u64).is_err());
        assert!(proof.verify(root, (index + 1) as u64).is_err());
    }
}

#[test]
fn proof_root_commits_recorded_node_results_at_successor_height() {
    use std::io::{BufRead, BufReader, Read, Write};
    use std::net::TcpListener;
    let recording: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../daemon/committed-fixture/vectors/vector-v4-committed.json"
        )),
        "required test value",
    );
    let settlement = &recording["settlement"];
    let validators = require_some(settlement["validators"].as_array(), "required test value")
        .iter()
        .map(|v| CommitValidator {
            address: require_ok(
                hex::decode(require_some(v["address"].as_str(), "required test value"))
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                    .try_into(),
                "required test value",
            ),
            public_key: require_ok(
                openssl::base64::decode_block(require_some(
                    v["pub_key"]["value"].as_str(),
                    "required test value",
                ))
                .unwrap_or_else(|error| panic!("{}: {error:?}", "required test value"))
                .try_into(),
                "required test value",
            ),
            voting_power: require_ok(
                v["voting_power"]
                    .as_str()
                    .unwrap_or_else(|| panic!("{}", "required test value"))
                    .parse(),
                "required test value",
            ),
        })
        .collect::<Vec<_>>();
    let signed =
        json!({"signed_header":{"header":settlement["header"],"commit":settlement["commit"]}});
    let listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
    let source = require_ok(
        NativeResultSource::connect(
            &format!(
                "http://{}",
                require_ok(listener.local_addr(), "required test value")
            ),
            None,
        ),
        "required test value",
    );
    let server = thread::spawn(move || {
        let (mut stream, _) = require_ok(listener.accept(), "required test value");
        require_ok(
            stream.set_read_timeout(Some(Duration::from_secs(3))),
            "required test value",
        );
        let mut reader = BufReader::new(&stream);
        let mut line = String::new();
        require_ok(reader.read_line(&mut line), "required test value");
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = 0;
        loop {
            line.clear();
            assert_ne!(
                require_ok(reader.read_line(&mut line), "required test value"),
                0
            );
            if line == "\r\n" {
                break;
            }
            if let Some((name, value)) = line.split_once(':') {
                if name.eq_ignore_ascii_case("content-length") {
                    length = require_ok(value.trim().parse(), "required test value");
                }
            }
        }
        let mut body = vec![0; length];
        require_ok(reader.read_exact(&mut body), "required test value");
        let request: Value = require_ok(serde_json::from_slice(&body), "required test value");
        assert_eq!(request["method"], "commit");
        assert_eq!(request["params"], json!({"height":"11"}));
        let body = json!({"jsonrpc":"2.0","id":request["id"],"result":signed}).to_string();
        require_ok(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()), "required test value");
    });
    complete_proof_root_commits_recorded_node_results_at_successor_height_1_repair_22_31_218(
        settlement,
        &validators,
        &source,
        server,
    );
}

fn complete_proof_root_commits_recorded_node_results_at_successor_height_1_repair_22_31_218(
    settlement: &Value,
    validators: &[CommitValidator],
    source: &NativeResultSource,
    server: thread::JoinHandle<()>,
) {
    let checkpoint = require_ok(
        source.checkpoint(11, "committed-fixture", validators),
        "required test value",
    );
    require_ok(server.join(), "required test value");
    let proof = require_ok(
        NativeResultSource::from_results(&json!([{}, settlement["exec_tx_result"]]), 1),
        "required test value",
    );
    assert_eq!(
        proof.result,
        decode_hex(&settlement["exec_tx_result_bytes"])
    );
    assert_eq!(
        proof.proof.leaf_hash.as_slice(),
        require_ok(
            openssl::base64::decode_block(require_some(
                settlement["exec_tx_result_proof"]["leaf_hash"].as_str(),
                "required test value"
            )),
            "required test value"
        )
    );
    require_ok(
        proof.verify_committed(10, 1, &checkpoint, "committed-fixture", validators),
        "required test value",
    );
    assert!(proof
        .verify_committed(11, 1, &checkpoint, "committed-fixture", validators)
        .is_err());
    assert_ne!(
        settlement["execution_header"]["last_results_hash"],
        settlement["header"]["last_results_hash"]
    );
    let mut insufficient = checkpoint.commit.clone();
    insufficient.votes.clear();
    assert!(layerx_agentd::read::checkpoint(
        11,
        "committed-fixture",
        &checkpoint.header,
        &insufficient,
        validators
    )
    .is_err());
}

#[test]
fn history_policy_shape_consumes_actual_hosted_business_rows() {
    let document: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        )),
        "required test value",
    );
    let result = &document["result"];
    let hosted = layerx_client::sequencer::HistoryPage {
        account: require_some(
            result["account"]["evm_address"].as_str(),
            "required test value",
        )
        .to_owned(),
        freshness: result["freshness"].clone(),
        items: require_some(result["items"].as_array(), "required test value").clone(),
        next_cursor: result["next_cursor"].as_str().map(str::to_owned),
        indexed_height: result["indexedHeight"].as_u64(),
        chain_height: result["chainHeight"].as_u64(),
    };
    assert_eq!(hosted.account, result["items"][0]["account"]);
    assert_eq!(hosted.freshness, result["freshness"]);
    let limits = HistoryLimits {
        maximum_items: 100,
        maximum_response_bytes: 240 * 1024,
    };
    let page = require_ok(
        HistoryPage::from_hosted(hosted.clone(), 0, u64::MAX, None, limits),
        "required test value",
    );
    assert_eq!(page.items.len(), 4);
    assert_eq!(page.items[0].position.block_height, 10);
    assert_eq!(page.items[3].position.tx_index, 1);
    assert_eq!(
        require_ok(
            serde_json::from_slice::<Value>(&page.items[0].canonical),
            "required test value"
        ),
        hosted.items[0]
    );
    assert!(HistoryPage::from_hosted(
        hosted.clone(),
        0,
        u64::MAX,
        None,
        HistoryLimits {
            maximum_items: 1,
            ..limits
        }
    )
    .is_err());
    assert!(HistoryPage::from_hosted(
        hosted.clone(),
        0,
        u64::MAX,
        None,
        HistoryLimits {
            maximum_response_bytes: 1,
            ..limits
        }
    )
    .is_err());
    let mut reordered = hosted;
    reordered.items.reverse();
    assert!(HistoryPage::from_hosted(reordered, 0, u64::MAX, None, limits).is_err());
}

// The RPC peer is an in-process committed fixture. Production readers, codecs,
// signature checks, Merkle verification and storage verification run unchanged.
// Receipt bytes are the Go producer's conformance recording. Consensus test
// authorities and in-memory store proofs come from the Programs fixture store;
// they are deliberately not represented as a recording of a live deployment.
#[allow(
    dead_code,
    clippy::too_many_lines,
    clippy::manual_let_else,
    reason = "copied registry support reports dead_code, clippy::too_many_lines and clippy::manual_let_else"
)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;

const CHAIN: &str = "repair-22-31-218";
const HEIGHT: u64 = 1000;
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const NOW: u64 = 1_700_000_000_123;

fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn quantity(value: u64) -> String {
    format!("{value:#x}")
}
fn decode_hex(value: &Value) -> Vec<u8> {
    require_ok(
        hex::decode(require_some(
            value
                .as_str()
                .unwrap_or_else(|| panic!("{}", "required test value"))
                .strip_prefix("0x"),
            "required test value",
        )),
        "required test value",
    )
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

fn sign_fixture(fixture: &mut chain::ChainFixture, height: u64) {
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_programs::chain_proof::{validators::TrustedValidator, vote::vote_sign_bytes};
    fixture.header.height = require_ok(i64::try_from(height), "test value fits i64");
    fixture.commit.height = require_ok(i64::try_from(height), "test value fits i64");
    fixture.commit.block_id.hash =
        require_ok(fixture.header.hash(), "required test value").to_vec();
    let message = require_ok(
        vote_sign_bytes(
            CHAIN,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            fixture.commit.signatures[0].timestamp,
        ),
        "required test value",
    );
    for sig in &mut fixture.commit.signatures {
        let key = require_some(
            SEEDS.iter().map(SigningKey::from_bytes).find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == sig.validator_address
            }),
            "required test value",
        );
        sig.signature = key.sign(&message).to_bytes().to_vec();
    }
}
fn comet_commit(fixture: &chain::ChainFixture) -> Value {
    let header = &fixture.header;
    let commit = &fixture.commit;
    let time = "2023-11-14T22:13:20.123456789Z";
    json!({"signed_header": {
        "header": {
            "version":{"block":header.version.block.to_string(),"app":header.version.app.to_string()},
            "chain_id":CHAIN,"height":header.height.to_string(),"time":time,
            "last_block_id":{"hash":hex::encode(&header.last_block_id.hash),"parts":{"total":header.last_block_id.part_set_total,"hash":hex::encode(&header.last_block_id.part_set_hash)}},
            "last_commit_hash":hex::encode(&header.last_commit_hash),"data_hash":hex::encode(&header.data_hash),
            "validators_hash":hex::encode(&header.validators_hash),"next_validators_hash":hex::encode(&header.next_validators_hash),
            "consensus_hash":hex::encode(&header.consensus_hash),"app_hash":hex::encode(&header.app_hash),
            "last_results_hash":hex::encode(&header.last_results_hash),"evidence_hash":hex::encode(&header.evidence_hash),
            "proposer_address":hex::encode(&header.proposer_address)
        },
        "commit":{"height":commit.height.to_string(),"round":commit.round,
            "block_id":{"hash":hex::encode(&commit.block_id.hash),"parts":{"total":commit.block_id.part_set_header.total,"hash":hex::encode(&commit.block_id.part_set_header.hash)}},
            "signatures":commit.signatures.iter().map(|sig| json!({"block_id_flag":2,"validator_address":hex::encode(&sig.validator_address),"timestamp":time,"signature":openssl::base64::encode_block(&sig.signature)})).collect::<Vec<_>>()}
    }})
}

fn word(value: u64) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[24..].copy_from_slice(&value.to_be_bytes());
    bytes
}
fn fixture_did() -> layerx_types::ids::Did {
    require_ok(
        layerx_types::ids::Did::new(format!("did:layerx:{}", hex::encode(&[9; 32])).as_bytes()),
        "required test value",
    )
}

struct LocalNode {
    rpc: String,
    raw: Vec<u8>,
    receipt: Value,
    validators: Vec<CommitValidator>,
    stop: std::sync::Arc<std::sync::atomic::AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl LocalNode {
    fn start() -> Self {
        Self::with_account_state(false)
    }
    fn with_account_state(account_state: bool) -> Self {
        Self::fixture(account_state, 20, 4)
    }
    fn with_funding(balance: u64) -> Self {
        Self::fixture(false, balance, 4)
    }
    fn with_insufficient_quorum() -> Self {
        Self::fixture(false, 20, 2)
    }
    fn fixture(account_state: bool, funded_balance: u64, signed_votes: usize) -> Self {
        use std::net::TcpListener;
        use std::sync::{
            atomic::{AtomicBool, Ordering},
            Arc,
        };
        let recording = fixture_recording();
        let chain = committed_fixture(
            &recording.receipt,
            &recording.raw,
            &recording.account_vector,
            account_state,
            signed_votes,
        );
        let calls = fixture_calls();
        let listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
        let rpc = format!(
            "http://{}",
            require_ok(listener.local_addr(), "required test value")
        );
        require_ok(listener.set_nonblocking(true), "required test value");
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let raw = recording.raw.clone();
        let receipt = recording.receipt.clone();
        let validators = chain.validators.clone();
        let peer = FixturePeer {
            recording,
            chain,
            calls,
            account_state,
            funded_balance,
        };
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::SeqCst) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(e) => panic!("fixture listener: {e}"),
                };
                peer.serve(stream);
            }
        });
        Self {
            rpc,
            raw,
            receipt,
            validators,
            stop,
            worker: Some(worker),
        }
    }
    fn validators(&self) -> Vec<CommitValidator> {
        self.validators.clone()
    }
    fn endpoint(&self) -> SequencerEndpoint {
        require_ok(
            SequencerEndpoint::new(&self.rpc, &[]),
            "required test value",
        )
    }
    fn source(&self) -> NativeResultSource {
        require_ok(
            NativeResultSource::connect(&self.rpc, None),
            "required test value",
        )
    }
    fn route(&self) -> NativeReadRoute {
        require_ok(
            NativeReadRoute::new(self.endpoint(), 125)
                .with_checkpoint_trust(CHAIN.into(), self.validators()),
            "required test value",
        )
        .with_result_source(self.source())
    }
}

fn verify_fixture_2_repair_22_31_218(account_vector: &Value, params: &Value) -> Value {
    assert_eq!(
        params[0],
        format!(
            "0x{}",
            require_some(account_vector["address"].as_str(), "required test value")
        )
    );
    assert_eq!(
        params[1],
        json!([format!(
            "0x{}",
            require_some(account_vector["slot"].as_str(), "required test value")
        )])
    );
    assert_eq!(params[2], quantity(HEIGHT));
    account_vector["response"].clone()
}

struct VerifyFixture1Repair2231218Context<'a, F0> {
    funded_balance: u64,
    owner: [u8; 32],
    asset: [u8; 32],
    main_account: [u8; 32],
    address: [u8; 32],
    binding: &'a Vec<u8>,
    metadata: &'a Vec<u8>,
    budget: &'a Vec<u8>,
    selector: F0,
    params: &'a Value,
}

fn verify_fixture_1_repair_22_31_218(
    context: VerifyFixture1Repair2231218Context<'_, impl Fn(&[u8]) -> Vec<u8>>,
) -> Value {
    let VerifyFixture1Repair2231218Context {
        funded_balance,
        owner,
        asset,
        main_account,
        address,
        binding,
        metadata,
        budget,
        selector,
        params,
    } = context;

    let data = decode_hex(&params[0]["data"]);
    let head = &data[..4];
    let bytes = if head == selector(b"getEvmAddrByLayerX(bytes32)") {
        assert_eq!(&data[4..], &[9; 32]);
        address.to_vec()
    } else if head == selector(b"getUnifiedAccount(address)") {
        assert_eq!(&data[4..], &address);
        [address, word(128), [9; 32], main_account, word(0)].concat()
    } else if head == selector(b"getLayerXDid(address)") {
        assert_eq!(&data[4..], &address);
        binding.clone()
    } else if head == selector(b"getAsset(bytes32)") {
        assert_eq!(&data[4..], &asset);
        metadata.clone()
    } else if head == selector(b"accountId(bytes32,bytes32)") {
        assert_eq!(&data[4..], &[owner, asset].concat());
        main_account.to_vec()
    } else if head == selector(b"accountOwner(bytes32)") {
        assert_eq!(&data[4..], &main_account);
        owner.to_vec()
    } else if head == selector(b"balanceOf(bytes32,bytes32)") {
        assert_eq!(&data[36..], &asset);
        if data[4..36] == [3; 32] {
            word(funded_balance).to_vec()
        } else {
            assert_eq!(&data[4..36], &main_account);
            word(100).to_vec()
        }
    } else if head == selector(b"nextSequence(bytes32,bytes32)") {
        assert_eq!(&data[4..], &[main_account, asset].concat());
        word(1).to_vec()
    } else if head == selector(b"getBudget(bytes32)") {
        assert_eq!(&data[4..], &[1; 32]);
        budget.clone()
    } else if head == selector(b"budgetRemaining(bytes32)") {
        assert_eq!(&data[4..], &[1; 32]);
        word(funded_balance.min(77)).to_vec()
    } else {
        panic!("undeclared committed call {params}")
    };
    json!(rpc_hex(&bytes))
}

fn complete_fixture_1_repair_22_31_218(
    history_rows: &[Value],
    params: &Value,
    cursor: Option<layerx_agentd::read::Cursor>,
) -> Value {
    let position = |row: &Value| layerx_agentd::read::Cursor {
        block_height: require_some(row["blockHeight"].as_u64(), "required test value"),
        tx_index: require_ok(
            u32::try_from(require_some(row["txIndex"].as_u64(), "required test value")),
            "test value fits u32",
        ),
        log_index: require_ok(
            u32::try_from(require_some(
                row["logIndex"].as_u64(),
                "required test value",
            )),
            "test value fits u32",
        ),
    };
    let rows = history_rows
        .iter()
        .filter(|row| cursor.is_none_or(|c| position(row) > c))
        .collect::<Vec<_>>();
    let limit = require_ok(
        usize::try_from(require_some(params[2].as_u64(), "required test value")),
        "test value fits usize",
    );
    assert!(limit > 0 && limit <= 100);
    let mut count = limit.min(rows.len());
    while count < rows.len() && position(rows[count]) == position(rows[count - 1]) {
        count += 1;
    }
    let next = (count < rows.len()).then(|| position(rows[count - 1]).to_hex());
    let transcript: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        )),
        "required test value",
    );
    json!({
        "account": params[0],
        "items": rows[..count],
        "next_cursor": next,
        "indexedHeight": HEIGHT + 1,
        "chainHeight": HEIGHT + 1,
        "freshness": transcript["result"]["freshness"]
    })
}
impl Drop for LocalNode {
    fn drop(&mut self) {
        self.stop.store(true, std::sync::atomic::Ordering::SeqCst);
        let result = require_some(self.worker.take(), "required test value").join();
        if !thread::panicking() {
            require_ok(result, "required test value");
        }
    }
}

#[test]
fn native_checkpoint_proof_acquisition_and_restart_use_real_node() {
    let node = LocalNode::start();
    let endpoint = node.endpoint();
    let validators = node.validators();
    let height = HEIGHT;
    let path = format!("/v1/reads/checkpoint/{height}");
    let mut route = node.route();
    assert!(NativeReadRoute::new(node.endpoint(), 125)
        .read(&path)
        .is_err());
    let served = require_ok(
        route.read(&path),
        "native route must authenticate committed quorum",
    );
    assert_eq!(served["verified"], true);
    assert_eq!(served["rung"], "committed");
    assert!(
        LocalNode::with_insufficient_quorum()
            .route()
            .read(&path)
            .is_err(),
        "the served checkpoint route must reject half of trusted power"
    );
    let checkpoint = require_ok(
        node.source().checkpoint(height, CHAIN, &validators),
        "required test value",
    );
    let mut insufficient = checkpoint.commit.clone();
    insufficient.votes.truncate(2);
    assert!(layerx_agentd::read::checkpoint(
        height,
        CHAIN,
        &checkpoint.header,
        &insufficient,
        &validators
    )
    .is_err());
    let mut damaged = checkpoint.commit.clone();
    damaged.votes[0].signature[0] ^= 1;
    assert!(layerx_agentd::read::checkpoint(
        height,
        CHAIN,
        &checkpoint.header,
        &damaged,
        &validators
    )
    .is_err());
    assert!(
        node.source()
            .program_state_record(
                [1; 32],
                b"unregistered-program-state",
                "lxphost",
                &checkpoint
            )
            .is_err(),
        "a missing native snapshot producer must not become a successful program state"
    );
    verify_native_checkpoint_proof_acquisition_and_restart_use_real_node_2_repair_22_31_218(
        VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode2Repair2231218Context {
            node,
            endpoint,
            validators,
            height,
            path,
            route,
            served,
            checkpoint,
        },
    );
}

struct VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode2Repair2231218Context {
    node: LocalNode,
    endpoint: SequencerEndpoint,
    validators: Vec<CommitValidator>,
    height: u64,
    path: String,
    route: NativeReadRoute,
    served: Value,
    checkpoint: layerx_agentd::read::ServedCheckpoint,
}

fn verify_native_checkpoint_proof_acquisition_and_restart_use_real_node_2_repair_22_31_218(
    context: VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode2Repair2231218Context,
) {
    let VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode2Repair2231218Context {
        node,
        endpoint,
        validators,
        height,
        path,
        route,
        served,
        checkpoint,
    } = context;

    let commitment = require_ok(checkpoint.commitment_proof(), "required test value");
    let decoded = require_ok(
        layerx_programs::chain_proof::header::ChainHeader::decode(&commitment.signed_header),
        "required test value",
    );
    assert_eq!(
        require_ok(decoded.hash(), "required test value"),
        checkpoint.block_hash
    );
    let (decoded_header, decoded_commit, decoded_validators) = require_ok(
        layerx_programs::chain_evidence::commitment_trust_step(&commitment),
        "required test value",
    );
    require_ok(
        layerx_programs::chain_proof::ChainProofVerifier {
            chain_id: CHAIN.into(),
            validators: decoded_validators,
        }
        .verify_head(&decoded_header, &decoded_commit),
        "required test value",
    );
    let program_reader = || {
        require_ok(
            layerx_agentd::read::LayerxdProgramBalanceReader::connect_native(
                &node.rpc,
                "fixture-reader".into(),
                None,
                node.source(),
                require_ok(
                    layerx_programs::ProtocolDeploymentVerifier::from_chain_trust(
                        CHAIN,
                        validators
                            .iter()
                            .map(|v| {
                                layerx_programs::chain_proof::validators::TrustedValidator::new(
                                    v.public_key,
                                    v.voting_power,
                                )
                            })
                            .collect(),
                        60_000,
                    ),
                    "required test value",
                ),
                layerx_programs::Registry::new(),
            ),
            "required test value",
        )
    };
    let reader = program_reader();
    let historical = require_ok(
        reader.read_chain_head_at(height, NOW),
        "native historical chain evidence",
    );
    verify_native_checkpoint_proof_acquisition_and_restart_use_real_node_1_repair_22_31_218(
        VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context {
            node,
            endpoint,
            validators,
            height,
            path,
            route,
            served,
            reader,
            historical,
        },
    );
}

struct VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context {
    node: LocalNode,
    endpoint: SequencerEndpoint,
    validators: Vec<CommitValidator>,
    height: u64,
    path: String,
    route: NativeReadRoute,
    served: Value,
    reader: layerx_agentd::read::LayerxdProgramBalanceReader,
    historical: layerx_programs::VerifiedChainHead,
}

fn verify_native_checkpoint_proof_acquisition_and_restart_use_real_node_1_repair_22_31_218(
    context: VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context,
) {
    use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};

    let VerifyNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context {
        node,
        endpoint,
        validators,
        height,
        path,
        route,
        served,
        reader,
        historical,
    } = context;

    let program_reader = || {
        require_ok(
            layerx_agentd::read::LayerxdProgramBalanceReader::connect_native(
                &node.rpc,
                "fixture-reader".into(),
                None,
                node.source(),
                require_ok(
                    layerx_programs::ProtocolDeploymentVerifier::from_chain_trust(
                        CHAIN,
                        validators
                            .iter()
                            .map(|v| {
                                layerx_programs::chain_proof::validators::TrustedValidator::new(
                                    v.public_key,
                                    v.voting_power,
                                )
                            })
                            .collect(),
                        60_000,
                    ),
                    "required test value",
                ),
                layerx_programs::Registry::new(),
            ),
            "required test value",
        )
    };
    assert_eq!(historical.freshness().observed_sequence, height);
    let current = require_ok(reader.read_chain_head(NOW), "native current chain evidence");
    assert!(current.freshness().observed_sequence >= height);
    drop(reader);

    let directory = std::env::temp_dir().join(format!(
        "repair-22-31-218-{}-{}",
        std::process::id(),
        require_ok(
            std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH),
            "required test value"
        )
        .as_nanos()
    ));
    let journal_key = require_ok(
        TenantKey::new(
            require_ok(TenantId::new("native-reader"), "required test value"),
            ObjectKind::Configuration,
            b"historical-head".to_vec(),
        ),
        "required test value",
    );
    let mut store = require_ok(Store::open(&directory), "required test value");
    require_ok(
        store.put_local(journal_key.clone(), height.to_be_bytes().to_vec()),
        "required test value",
    );
    drop(store);
    let reopened_store = require_ok(Store::open(&directory), "required test value");
    let retained_height = u64::from_be_bytes(require_ok(
        reopened_store
            .get(&journal_key)
            .unwrap_or_else(|| panic!("{}", "required test value"))
            .bytes()
            .try_into(),
        "required test value",
    ));
    let recovered = require_ok(
        program_reader().read_chain_head_at(retained_height, NOW),
        "native historical evidence after reopening reader",
    );
    assert_eq!(historical.receipt_digest(), recovered.receipt_digest());
    finish_native_checkpoint_proof_acquisition_and_restart_use_real_node_1_repair_22_31_218(
        FinishNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context {
            node,
            endpoint,
            validators,
            path,
            route,
            served,
            historical,
            directory,
            reopened_store,
            recovered,
        },
    );
}

struct FinishNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context {
    node: LocalNode,
    endpoint: SequencerEndpoint,
    validators: Vec<CommitValidator>,
    path: String,
    route: NativeReadRoute,
    served: Value,
    historical: layerx_programs::VerifiedChainHead,
    directory: std::path::PathBuf,
    reopened_store: layerx_agentd::store::Store,
    recovered: layerx_programs::VerifiedChainHead,
}

fn finish_native_checkpoint_proof_acquisition_and_restart_use_real_node_1_repair_22_31_218(
    context: FinishNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context,
) {
    let FinishNativeCheckpointProofAcquisitionAndRestartUseRealNode1Repair2231218Context {
        node,
        endpoint,
        validators,
        path,
        route,
        served,
        historical,
        directory,
        reopened_store,
        recovered,
    } = context;

    assert_eq!(historical.state_root(), recovered.state_root());
    drop(reopened_store);
    require_ok(std::fs::remove_dir_all(directory), "required test value");
    drop(route);
    let mut reopened = node.route();
    assert_eq!(
        require_ok(reopened.read(&path), "required test value"),
        served,
        "reopening preserves authenticated historical coordinates"
    );
    let hash = transaction_hash(&node.raw);
    assert_eq!(rpc_hex(&hash), node.receipt["txHash"]);
    let keys = require_ok(endpoint.receipt_keys(), "required test value");
    let Lookup::Verified(receipt) = require_ok(
        lookup_authenticated(
            &endpoint,
            hash,
            LookupContext {
                receipt_keys: &keys,
            },
        ),
        "required test value",
    ) else {
        panic!("recorded receipt must authenticate")
    };
    let received = require_ok(
        layerx_agentd::read::acquire_native_receipt(
            &endpoint,
            ProofBundleTarget::Receipt(hash),
            Rung::Receipted,
            &node.raw,
            receipt.canonical_bytes(),
            1,
        ),
        "required test value",
    );
    assert_eq!(received.achieved_rung, Rung::Receipted);
    let proof = require_ok(
        reopened.read(&format!("/v1/reads/proof/{}", hex::encode(&hash))),
        "native proof route must return full authenticated inclusion bundle",
    );
    assert_eq!(proof["verificationLevel"], "committed");
    assert!(proof["receiptInclusion"]
        .as_str()
        .is_some_and(|bytes| !bytes.is_empty()));
    assert!(proof["commit"]
        .as_str()
        .is_some_and(|bytes| !bytes.is_empty()));
    let account_node = LocalNode::with_account_state(true);
    let mut account = [0; 32];
    account[12..].copy_from_slice(&require_ok(
        hex::decode("1122334455667788990011223344556677889900"),
        "required test value",
    ));
    let slot = require_ok(
        hex::decode_digest("0000000000000000000000000000000000000000000000000123456789abcdef"),
        "required test value",
    );
    let account_bundle = require_ok(
        NativeProofRequest {
            target: ProofBundleTarget::AccountState {
                activity_id: hash,
                account_id: account,
            },
            requested_rung: Rung::Committed,
            raw_transaction: &account_node.raw,
            served_receipt: receipt.canonical_bytes(),
            chain_id: CHAIN,
            validators: &validators,
            account_slots: &[slot],
        }
        .acquire(&account_node.endpoint(), &account_node.source()),
        "committed account acquisition must obtain authenticated storage material",
    );
    assert!(account_bundle.value.account_proof.is_some());
}

#[test]
fn native_history_keeps_position_groups_and_authenticates_reverse_windows() {
    use layerx_agentd::read::Cursor;
    let node = LocalNode::start();
    let mut route = node.route();
    let first = require_ok(
        route.read("/v1/reads/history/account?limit=1"),
        "required test value",
    );
    assert_eq!(
        require_some(first["items"].as_array(), "required test value").len(),
        2
    );
    assert_eq!(first["items"][0]["txIndex"], 1);
    assert_eq!(first["items"][1]["txIndex"], 1);
    let cursor = require_some(first["cursor"].as_str(), "required test value");
    let second = require_ok(
        route.read(&format!(
            "/v1/reads/history/account?limit=1&cursor={cursor}"
        )),
        "required test value",
    );
    assert_eq!(
        require_some(second["items"].as_array(), "required test value").len(),
        2
    );
    assert_eq!(second["items"][0]["txIndex"], 2);
    assert_eq!(second["complete"], true);
    assert!(second["cursor"].is_null());
    let limits = HistoryLimits {
        maximum_items: 1,
        maximum_response_bytes: 240 * 1024,
    };
    let newest = require_ok(
        HistoryPage::read_newest(&node.endpoint(), "account", None, limits),
        "required test value",
    );
    assert_eq!(newest.items.len(), 1);
    assert_eq!(newest.items[0].position.tx_index, 2);
    assert_eq!(
        newest.items[0].canonical,
        decode_hex(&node.receipt["receipt"])
    );
    let next = require_some(newest.next_cursor, "required test value");
    assert_eq!((next.block_height, next.tx_index), (HEIGHT, 2));
    let older = require_ok(
        HistoryPage::read_newest(&node.endpoint(), "account", Some(next), limits),
        "required test value",
    );
    assert_eq!(older.items.len(), 1);
    assert_eq!(older.items[0].position.tx_index, 1);
    assert!(older.next_cursor.is_none());
    // A forward page cannot overflow into a later position, even if its byte
    // size is within the response ceiling.
    let invalid = layerx_client::sequencer::HistoryPage {
        account: "account".to_owned(),
        freshness: require_ok(
            serde_json::from_str::<Value>(include_str!(
                "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
            )),
            "required test value",
        )["result"]["freshness"]
            .clone(),
        items: require_some(first["items"].as_array(), "required test value")
            .iter()
            .chain(require_some(
                second["items"].as_array(),
                "required test value",
            ))
            .cloned()
            .collect(),
        next_cursor: None,
        indexed_height: Some(HEIGHT + 1),
        chain_height: Some(HEIGHT + 1),
    };
    assert!(HistoryPage::from_hosted(invalid, 0, u64::MAX, None, limits).is_err());
    assert_eq!(
        require_ok(Cursor::from_hex(cursor), "required test value").tx_index,
        1
    );
}

#[test]
fn committed_inventory_selects_only_checkpoint_funded_budget() {
    use layerx_agentd::read::committed::CommittedReads;
    for balance in [20, 0] {
        let node = LocalNode::with_funding(balance);
        let endpoint = node.endpoint();
        let checkpoint = require_ok(
            node.source().checkpoint(HEIGHT, CHAIN, &node.validators()),
            "required test value",
        );
        let reads = require_ok(
            CommittedReads::at_checkpoint(&endpoint, &checkpoint),
            "required test value",
        );
        let mut caps = require_ok(
            reads.inventory(&fixture_did(), &[[4; 32]], [4; 32]),
            "required test value",
        );
        require_ok(
            reads.inventory_records(&mut caps, &[[1; 32]], &[]),
            "required test value",
        );
        let funding = require_ok(caps.budget_funding(&[1; 32]), "required test value");
        assert_eq!(funding.allowance(), 77);
        assert_eq!(funding.funded_balance(), u128::from(balance));
        assert_eq!(funding.remaining(), u128::from(balance));
        if balance > 0 {
            assert_eq!(
                require_ok(
                    caps.budget_source(&[1; 32], [4; 32], u128::from(balance)),
                    "required test value"
                )
                .remaining(),
                u128::from(balance)
            );
        }
        assert!(caps
            .budget_source(&[1; 32], [4; 32], u128::from(balance) + 1)
            .is_err());
        assert!(caps.budget_source(&[1; 32], [4; 32], 0).is_err());
    }
}

#[test]
fn committed_remaining_preserves_allowance_and_funded_spending_power() {
    use layerx_agentd::read::committed::CommittedBudgetFunding;
    use layerx_client::budget::ProtocolBudgetRecord;
    let vector: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../tests/vectors/native-budget-records.json"
        )),
        "required test value",
    );
    for version in ["v1", "v2"] {
        let bytes = require_ok(
            hex::decode(require_some(
                vector[version]
                    .as_str()
                    .unwrap_or_else(|| panic!("{}", "required test value"))
                    .strip_prefix("0x"),
                "required test value",
            )),
            "required test value",
        );
        let record = require_ok(ProtocolBudgetRecord::decode(&bytes), "required test value");
        assert_eq!(record.per_period_limit, 100);
        assert_eq!(record.spent_this_period, 23);
        let funded = require_ok(
            CommittedBudgetFunding::verify_values(&record, 20, 20),
            "required test value",
        );
        assert_eq!(funded.allowance(), 77);
        assert_eq!(funded.funded_balance(), 20);
        assert_eq!(funded.remaining(), 20);
        let defunded = require_ok(
            CommittedBudgetFunding::verify_values(&record, 0, 0),
            "required test value",
        );
        assert_eq!(defunded.allowance(), 77);
        assert_eq!(defunded.remaining(), 0);
        assert!(CommittedBudgetFunding::verify_values(&record, 20, 77).is_err());
        assert!(CommittedBudgetFunding::verify_values(&record, 0, 20).is_err());
    }
}

struct FixtureRecording {
    raw: Vec<u8>,
    receipt: Value,
    key: Value,
    older_receipt: Value,
    history_rows: Vec<Value>,
    account_vector: Value,
}
fn fixture_recording() -> FixtureRecording {
    let vectors: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        )),
        "required test value",
    );
    let vector = &vectors["cases"][1];
    let raw = decode_hex(&vector["rawTransaction"]);
    let receipt = vector["receiptJson"].clone();
    let key = vector["pinnedKey"].clone();
    let older_receipt = vectors["cases"][0]["receiptJson"].clone();
    let recorded_history: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
        )),
        "required test value",
    );
    let mut history_rows = require_some(
        recorded_history["result"]["items"].as_array(),
        "required test value",
    )
    .clone();
    for (index, row) in history_rows.iter_mut().enumerate() {
        row["blockHeight"] = json!(HEIGHT);
        row["txIndex"] = json!(if index < 2 { 1 } else { 2 });
        row["logIndex"] = json!(0);
        row["tx_hash"] = if index < 2 {
            older_receipt["txHash"].clone()
        } else {
            receipt["txHash"].clone()
        };
    }
    let account_vector: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../programs/tests/vectors/ics23-evm-storage-app-hash.json"
        )),
        "required test value",
    );
    let account_vector = account_vector["entries"][0].clone();
    FixtureRecording {
        raw,
        receipt,
        key,
        older_receipt,
        history_rows,
        account_vector,
    }
}
struct FixtureChain {
    execution_commit: Value,
    successor_commit: Value,
    execution_block: Value,
    successor_block: Value,
    execution_ops: Vec<Value>,
    successor_ops: Vec<Value>,
    published: Vec<Value>,
    batch: Vec<u8>,
    next_batch: Vec<u8>,
    response: Vec<u8>,
    validators: Vec<CommitValidator>,
}
fn committed_fixture(
    receipt: &Value,
    raw: &[u8],
    account_vector: &Value,
    account_state: bool,
    signed_votes: usize,
) -> FixtureChain {
    use layerx_programs::chain_proof::merkle::hash_from_byte_slices;
    use sha2::{Digest, Sha256};
    let mut execution = require_ok(
        chain::ChainFixture::new(CHAIN, &SEEDS, vec![(vec![2], 2_u64.to_be_bytes().to_vec())]),
        "required test value",
    );
    let mut successor = require_ok(
        chain::ChainFixture::new(CHAIN, &SEEDS, vec![(vec![2], 2_u64.to_be_bytes().to_vec())]),
        "required test value",
    );
    // The account peer uses the actual EVM store vector, including its
    // recorded multistore root. Its independent test authority commits it.
    if account_state {
        successor.header.app_hash = require_ok(
            hex::decode(require_some(
                account_vector["app_hash"].as_str(),
                "required test value",
            )),
            "required test value",
        );
    }
    let mut batch = b"PAXEER/v1/sequence-batch\0".to_vec();
    batch.extend_from_slice(&[0; 216]);
    let offset = b"PAXEER/v1/sequence-batch\0".len();
    batch[offset..offset + 8].copy_from_slice(&HEIGHT.to_be_bytes());
    batch[offset + 8..offset + 16].copy_from_slice(&1_u64.to_be_bytes());
    batch[offset + 16..offset + 24].copy_from_slice(&2_u64.to_be_bytes());
    execution.header.data_hash = hash_from_byte_slices(&[&Sha256::digest(&batch)]).to_vec();
    sign_fixture(&mut execution, HEIGHT);
    successor.header.last_block_id.hash =
        require_ok(execution.header.hash(), "required test value").to_vec();
    let response = abci_response(receipt);
    let mut result = Vec::new();
    protobuf_bytes(2, &response, &mut result);
    protobuf_integer(5, 60_000, &mut result);
    protobuf_integer(6, 46_220, &mut result);
    successor.header.last_results_hash = hash_from_byte_slices(&[&[], &[], &result]).to_vec();
    let mut next_batch = batch.clone();
    next_batch[offset..offset + 8].copy_from_slice(&(HEIGHT + 1).to_be_bytes());
    successor.header.data_hash = hash_from_byte_slices(&[&Sha256::digest(&next_batch)]).to_vec();
    sign_fixture(&mut successor, HEIGHT + 1);
    let validators = execution
        .verifier
        .validators
        .validators()
        .iter()
        .map(|v| CommitValidator {
            address: v.address,
            public_key: v.public_key,
            voting_power: v.voting_power,
        })
        .collect::<Vec<_>>();
    let mut execution_commit = comet_commit(&execution);
    for sig in &mut require_some(
        execution_commit["signed_header"]["commit"]["signatures"].as_array_mut(),
        "required test value",
    )[signed_votes..]
    {
        *sig = json!({"block_id_flag":1,"validator_address":"","timestamp":"0001-01-01T00:00:00Z","signature":null});
    }
    let successor_commit = comet_commit(&successor);
    let tx = fixture_transaction(&execution, raw, receipt);
    let execution_block = fixture_block(&execution, &successor.header.last_results_hash, &[tx]);
    let successor_block = fixture_block(&successor, &[6; 32], &[]);
    let host_ops = |fixture: &chain::ChainFixture| {
        require_ok(fixture.prove(&[2]), "required test value").iter().map(|op|json!({"type":op.op_type,"key":openssl::base64::encode_block(&op.key),"data":openssl::base64::encode_block(&op.data)})).collect::<Vec<_>>()
    };
    let execution_ops = host_ops(&execution);
    let successor_ops = host_ops(&successor);
    let published = validators.iter().map(|v| json!({"address":hex::encode(&v.address),"pub_key":{"type":"tendermint/PubKeyEd25519","value":openssl::base64::encode_block(&v.public_key)},"voting_power":v.voting_power.to_string()})).collect::<Vec<_>>();
    FixtureChain {
        execution_commit,
        successor_commit,
        execution_block,
        successor_block,
        execution_ops,
        successor_ops,
        published,
        batch,
        next_batch,
        response,
        validators,
    }
}
fn fixture_block(fixture: &chain::ChainFixture, results: &[u8], txs: &[Value]) -> Value {
    json!({
        "number":quantity(require_ok(u64::try_from(fixture.header.height), "test value fits u64")),"hash":rpc_hex(&require_ok(fixture.header.hash(), "required test value")),
        "parentHash":rpc_hex(&fixture.header.last_block_id.hash),"timestamp":quantity(require_ok(u64::try_from(fixture.header.time.seconds), "test value fits u64")),
        "transactionsRoot":rpc_hex(&fixture.header.data_hash),"receiptsRoot":rpc_hex(results),"stateRoot":rpc_hex(&fixture.header.app_hash),"transactions":txs
    })
}

fn fixture_transaction(execution: &chain::ChainFixture, raw: &[u8], receipt: &Value) -> Value {
    let (call, signature) = require_ok(
        layerx_crypto::evm_transaction::decode_signed(raw),
        "required test value",
    );
    let signature = signature.as_bytes();
    let integer_hex = |bytes: &[u8]| {
        let text = hex::encode(bytes);
        format!("0x{}", text.trim_start_matches('0'))
    };
    json!({
        "blockHash":rpc_hex(&require_ok(execution.header.hash(), "required test value")),"blockNumber":quantity(HEIGHT),"from":receipt["from"],
        "gas":quantity(call.gas_limit),"gasPrice":"0x77359400","hash":rpc_hex(&transaction_hash(raw)),
        "input":rpc_hex(&call.data),"nonce":quantity(call.nonce),"to":rpc_hex(&call.to),"transactionIndex":"0x0",
        "value":"0x0","type":"0x2","v":quantity(u64::from(signature[64]-27)),"r":integer_hex(&signature[..32]),"s":integer_hex(&signature[32..64]),
        "chainId":"0x7d","accessList":[],"yParity":quantity(u64::from(signature[64]-27)),"maxFeePerGas":format!("{:#x}",call.max_fee_per_gas),"maxPriorityFeePerGas":format!("{:#x}",call.max_priority_fee_per_gas)
    })
}
struct FixtureCalls {
    owner: [u8; 32],
    asset: [u8; 32],
    main_account: [u8; 32],
    address: [u8; 32],
    binding: Vec<u8>,
    metadata: Vec<u8>,
    budget: Vec<u8>,
}
fn fixture_calls() -> FixtureCalls {
    let did = fixture_did();
    let owner = require_ok(
        layerx_wire::hash::did_id_for_protocol(&did, 3),
        "required test value",
    );
    let asset = [4; 32];
    let account = require_ok(
        layerx_types::account::AccountId::for_asset(
            require_ok(std::str::from_utf8(did.as_bytes()), "required test value"),
            asset,
            asset,
        ),
        "required test value",
    );
    let main_account = require_ok(
        layerx_wire::hash::account_id_for_protocol(&account, 3),
        "required test value",
    );
    let mut address = [0; 32];
    address[12..].fill(5);
    let mut binding = [word(64), word(did.as_bytes().len() as u64)].concat();
    binding.splice(0..0, [9; 32]);
    binding.extend_from_slice(did.as_bytes());
    binding.resize(binding.len().div_ceil(32) * 32, 0);
    let mut metadata = vec![0; 12 * 32];
    metadata[..32].copy_from_slice(&asset);
    metadata[32..64].copy_from_slice(&word(384));
    metadata[64..96].copy_from_slice(&word(448));
    metadata[96..128].copy_from_slice(&word(18));
    metadata[160..192].copy_from_slice(&owner);
    metadata[9 * 32..10 * 32].copy_from_slice(&word(512));
    for name in [b"PAX", b"Pax"] {
        metadata.extend_from_slice(&word(3));
        metadata.extend_from_slice(name);
        metadata.resize(metadata.len().div_ceil(32) * 32, 0);
    }
    metadata.extend_from_slice(&word(0));
    let mut budget = vec![0; 20 * 32];
    for (index, value) in [
        (0, 32),
        (5, 100),
        (6, 100),
        (8, 23),
        (10, 1000),
        (11, 1),
        (12, u64::MAX),
        (14, 1),
        (17, 576),
    ] {
        budget[index * 32..(index + 1) * 32].copy_from_slice(&word(value));
    }
    budget[32..64].copy_from_slice(&owner);
    budget[64..96].fill(3);
    budget[96..128].copy_from_slice(&asset);
    budget[128..160].fill(5);
    FixtureCalls {
        owner,
        asset,
        main_account,
        address,
        binding,
        metadata,
        budget,
    }
}
struct FixturePeer {
    recording: FixtureRecording,
    chain: FixtureChain,
    calls: FixtureCalls,
    account_state: bool,
    funded_balance: u64,
}
impl FixturePeer {
    fn answer(&self, request: &Value) -> Value {
        let params = &request["params"];
        let method = require_some(request["method"].as_str(), "required test value");
        let selector = |name: &[u8]| transaction_hash(name)[..4].to_vec();
        let selected = params["height"]
            .as_str()
            .and_then(|v| v.parse::<u64>().ok());
        match method {
            "eth_chainId" => json!("0x7d"),
            "eth_blockNumber" => json!(quantity(HEIGHT + 1)),
            "px_getReceiptKeys" => json!({"keys":[self.recording.key]}),
            "pax_sequencerReceipt" => {
                if params[0] == self.recording.receipt["txHash"] {
                    self.recording.receipt.clone()
                } else {
                    assert_eq!(params[0], self.recording.older_receipt["txHash"]);
                    self.recording.older_receipt.clone()
                }
            }
            "px_getHistory" => {
                use layerx_agentd::read::Cursor;
                let cursor = params[1]
                    .as_str()
                    .map(|text| require_ok(Cursor::from_hex(text), "required test value"));
                complete_fixture_1_repair_22_31_218(&self.recording.history_rows, params, cursor)
            }
            "eth_getTransactionReceipt" => Value::Null,
            "eth_getBlockByNumber" => {
                match require_some(params[0].as_str(), "required test value") {
                    "0x3e8" => self.chain.execution_block.clone(),
                    "0x3e9" => self.chain.successor_block.clone(),
                    other => panic!("unexpected EVM height {other}"),
                }
            }
            "commit" => match selected {
                Some(HEIGHT) => self.chain.execution_commit.clone(),
                Some(h) if h == HEIGHT + 1 => self.chain.successor_commit.clone(),
                _ => panic!("unknown commit {params}"),
            },
            "validators" => {
                assert!(selected == Some(HEIGHT) || selected == Some(HEIGHT + 1));
                json!({"block_height":require_some(selected, "required test value").to_string(),"count":"4","total":"4","validators":self.chain.published})
            }
            "block" => {
                let h = require_some(selected, "required test value");
                assert!(h == HEIGHT || h == HEIGHT + 1);
                json!({"block":{"header":{"height":h.to_string()},"data":{"txs":[openssl::base64::encode_block(if h==HEIGHT {&self.chain.batch} else {&self.chain.next_batch})]}}})
            }
            "block_results" => {
                assert_eq!(selected, Some(HEIGHT));
                json!({"height":HEIGHT.to_string(),"txs_results":[{}, {}, {"data":openssl::base64::encode_block(&self.chain.response),"gas_wanted":"60000","gas_used":"46220"}]})
            }
            "abci_query" if params["path"] == "/store/lxphost/key" => {
                assert_eq!(params["data"], "02");
                let h = require_some(selected, "required test value");
                assert!(h == HEIGHT - 1 || h == HEIGHT);
                json!({"response":{"code":0,"height":h.to_string(),"key":openssl::base64::encode_block(&[2]),"value":openssl::base64::encode_block(&2_u64.to_be_bytes()),"proofOps":{"ops":if h==HEIGHT-1 {&self.chain.execution_ops}else{&self.chain.successor_ops}}}})
            }
            "abci_query" => {
                json!({"response":{"code":6,"height":require_some(selected, "required test value").to_string(),"log":"ProgramState producer is not registered"}})
            }
            "eth_call" => {
                assert_eq!(params[1], quantity(HEIGHT));
                verify_fixture_1_repair_22_31_218(VerifyFixture1Repair2231218Context {
                    funded_balance: self.funded_balance,
                    owner: self.calls.owner,
                    asset: self.calls.asset,
                    main_account: self.calls.main_account,
                    address: self.calls.address,
                    binding: &self.calls.binding,
                    metadata: &self.calls.metadata,
                    budget: &self.calls.budget,
                    selector,
                    params,
                })
            }
            "eth_getProof" => {
                assert!(self.account_state);
                verify_fixture_2_repair_22_31_218(&self.recording.account_vector, params)
            }
            _ => panic!("undeclared native RPC {request}"),
        }
    }
    fn serve(&self, mut stream: std::net::TcpStream) {
        use std::io::{BufRead, BufReader, Read, Write};
        require_ok(
            stream.set_read_timeout(Some(Duration::from_secs(3))),
            "required test value",
        );
        let mut reader = BufReader::new(&stream);
        let mut line = String::new();
        require_ok(reader.read_line(&mut line), "required test value");
        assert_eq!(line, "POST / HTTP/1.1\r\n");
        let mut length = 0;
        loop {
            line.clear();
            assert_ne!(
                require_ok(reader.read_line(&mut line), "required test value"),
                0
            );
            if line == "\r\n" {
                break;
            }
            if let Some((name, value)) = line.split_once(':') {
                if name.eq_ignore_ascii_case("content-length") {
                    length = require_ok(value.trim().parse(), "required test value");
                }
            }
        }
        let mut body = vec![0; length];
        require_ok(reader.read_exact(&mut body), "required test value");
        let request: Value = require_ok(serde_json::from_slice(&body), "required test value");
        let result = self.answer(&request);
        let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
        require_ok(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()), "required test value");
    }
}
