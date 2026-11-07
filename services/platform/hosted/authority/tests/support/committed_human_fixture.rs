use chain_readiness_tests::{base64, must as fixture_must};
use ed25519_dalek::Signer as _;
use serde_json::{json as document, Value};
use sha2::{Digest, Sha256};
use std::io::{BufRead as _, BufReader};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::sync::Mutex;

fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn varint(mut value: u64) -> Vec<u8> {
    let mut bytes = Vec::new();
    while value >= 128 {
        bytes.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    bytes.push(value as u8);
    bytes
}

fn field(number: u8, value: &[u8]) -> Vec<u8> {
    [
        vec![(number << 3) | 2],
        varint(value.len() as u64),
        value.to_vec(),
    ]
    .concat()
}

fn leaf_hash(prefix: &[u8], key: &[u8], value: &[u8]) -> [u8; 32] {
    hash(
        &[
            prefix.to_vec(),
            varint(key.len() as u64),
            key.to_vec(),
            vec![32],
            hash(value).to_vec(),
        ]
        .concat(),
    )
}

fn leaf_proof(key: &[u8], value: &[u8], prefix: &[u8], path: &[(Vec<u8>, Vec<u8>)]) -> Vec<u8> {
    let leaf = [vec![8, 1, 24, 1, 32, 1], field(5, prefix)].concat();
    let mut existence = [field(1, key), field(2, value), field(3, &leaf)].concat();
    for (prefix, suffix) in path {
        let inner = [vec![8, 1], field(2, prefix), field(3, suffix)].concat();
        existence.extend(field(4, &inner));
    }
    field(1, &existence)
}

struct StoreNode {
    root: [u8; 32],
    height: u64,
    size: u64,
    rows: Vec<(Vec<u8>, Vec<u8>, Vec<(Vec<u8>, Vec<u8>)>)>,
}

impl StoreNode {
    fn build(rows: &[(Vec<u8>, Vec<u8>)]) -> Self {
        if rows.len() == 1 {
            let (key, value) = &rows[0];
            return Self {
                root: leaf_hash(&[0, 2, 2], key, value),
                height: 0,
                size: 1,
                rows: vec![(key.clone(), value.clone(), Vec::new())],
            };
        }
        let (left, right) = rows.split_at(rows.len() / 2);
        let mut left = Self::build(left);
        let mut right = Self::build(right);
        let height = 1 + left.height.max(right.height);
        let size = left.size + right.size;
        let prefix = [varint(height * 2), varint(size * 2), vec![2, 32]].concat();
        let suffix = [vec![32], right.root.to_vec()].concat();
        let root = hash(&[prefix.clone(), left.root.to_vec(), suffix.clone()].concat());
        for (_, _, path) in &mut left.rows {
            path.push((prefix.clone(), suffix.clone()));
        }
        let right_prefix = [prefix, left.root.to_vec(), vec![32]].concat();
        for (_, _, path) in &mut right.rows {
            path.push((right_prefix.clone(), vec![]));
        }
        left.rows.extend(right.rows);
        Self {
            root,
            height,
            size,
            rows: left.rows,
        }
    }
}

struct NativeStore {
    app_hash: [u8; 32],
    queries: BTreeMap<String, Value>,
}

impl NativeStore {
    fn commit(mut rows: Vec<(Vec<u8>, Vec<u8>)>) -> Self {
        rows.sort_by(|a, b| a.0.cmp(&b.0));
        assert!(rows.windows(2).all(|pair| pair[0].0 != pair[1].0));
        let tree = StoreNode::build(&rows);
        let outer = leaf_proof(b"lxphost", &tree.root, &[0], &[]);
        let app_hash = leaf_hash(&[0], b"lxphost", &tree.root);
        let queries = tree
            .rows
            .into_iter()
            .map(|(key, value, path)| {
                let proof = leaf_proof(&key, &value, &[0, 2, 2], &path);
                // The in-memory peer commits real ICS23 membership, verified by the production codec.
                fixture_must(crate::committed::verify_membership(
                    &key, &value, &proof, &outer, app_hash,
                ));
                (
                    hex::encode(&key),
                    document!({"code":0,"key":base64(&key),"value":base64(&value),
                "proofOps":{"ops":[{"type":"ics23:iavl","key":base64(&key),"data":base64(&proof)},
                {"type":"ics23:simple","key":base64(b"lxphost"),"data":base64(&outer)}]}}),
                )
            })
            .collect();
        Self { app_hash, queries }
    }
}

fn account(name: &str, kind: u8, asset: [u8; 32], balance: u128) -> ([u8; 32], Vec<u8>) {
    use layerx_wire::encode::Encoder;
    let id = fixture_must(layerx_wire::hash::account_id_for_protocol(
        &fixture_must(layerx_types::account::AccountId::parse(name)),
        3,
    ));
    let mut writer = Encoder::new(2048);
    fixture_must(writer.bytes(&id, 32));
    fixture_must(writer.bytes(name.as_bytes(), 512));
    fixture_must(writer.u8(kind));
    fixture_must(writer.u128(balance));
    fixture_must(writer.bytes(&asset, 32));
    fixture_must(writer.u8(1));
    fixture_must(writer.u64(2));
    fixture_must(writer.u64(1));
    fixture_must(writer.u8(0));
    fixture_must(writer.u8(0));
    fixture_must(writer.bytes(&[0x61; 32], 32));
    fixture_must(writer.u8(1));
    (id, writer.finish())
}

fn wire_proof(proof: &layerx_proof::merkle::Proof) -> String {
    let mut writer = layerx_wire::encode::Encoder::new(2048);
    fixture_must(writer.structure_header(0x4d50));
    fixture_must(writer.u32(proof.leaf_index()));
    fixture_must(writer.u32(proof.leaf_count()));
    fixture_must(writer.u8(u8::try_from(proof.siblings().len()).unwrap()));
    fixture_must(writer.bytes(&proof.siblings().concat(), 1024));
    hex::encode(&writer.finish())
}

fn seal_fixture(mut row: Value) -> Value {
    // Preserve the native receipt, including its zero context; only reseal the header's key identity.
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x22; 32]);
    let public = signer.verifying_key().to_bytes();
    assert_eq!(hex::encode(&public), row["sequencer_public_key"]);
    let key_id = hash(&public);
    let bytes = fixture_must(hex::decode(row["receipt"].as_str().unwrap()));
    let original = fixture_must(layerx_proof::receipt::verify_sequencer_signature(
        &bytes, public,
    ));
    let protocol = original.protocol().unwrap();
    assert_eq!(protocol.context_hash(), [0; 32]);
    let maintenance = fixture_must(hex::decode(row["maintenance"].as_str().unwrap()));
    let leaves = [&bytes[..], &maintenance[..]];
    let (proof, root) = fixture_must(layerx_proof::merkle::build_proof(&leaves, 0));
    let (maintenance_proof, _) = fixture_must(layerx_proof::merkle::build_proof(&leaves, 1));
    let mut header = fixture_must(hex::decode(row["header"].as_str().unwrap()));
    let original = fixture_must(layerx_wire::receipt::decode_batch_header(&header));
    for (old, new) in [
        (original.sequencer_id(), key_id),
        (original.receipt_merkle_root(), root),
    ] {
        let offset = header.windows(32).position(|value| value == old).unwrap();
        header[offset..offset + 32].copy_from_slice(&new);
    }
    row["header_signature"] = document!(hex::encode(
        &signer
            .sign(&fixture_must(layerx_wire::hash::batch_header_digest(
                &header
            )))
            .to_bytes()
    ));
    row["header"] = document!(hex::encode(&header));
    row["receipt"] = document!(hex::encode(&bytes));
    row["receipt_proof"] = document!(wire_proof(&proof));
    row["maintenance_proof"] = document!(wire_proof(&maintenance_proof));
    let document = document!({"authority_replica_id":hex::encode(&[9;32]),
        "sequencer_public_key":row["sequencer_public_key"],
        "batch_evidence":{"header_hex":row["header"],"header_signature":row["header_signature"],
            "receipt_proof_hex":row["receipt_proof"],"batch_identity":{"kind":"batch_maintenance_v1",
                "receipt_hex":row["maintenance"],"receipt_proof_hex":row["maintenance_proof"],
                "activity_receipts_hex":[row["receipt"]]}}});
    let evidence = fixture_must(layerx_platform_authority::parse_replica_evidence(
        &fixture_must(serde_json::to_vec(&document)),
        [9; 32],
        public,
    ));
    let facts = fixture_must(layerx_platform_authority::authorized_batch_by_activity(
        protocol.activity_id(),
        &bytes,
        &evidence,
        &SequencerAuthorization::new(key_id, public, 1, 1000),
    ));
    assert_eq!(facts.global_sequence, protocol.global_sequence());
    row
}

struct Fixture {
    did: String,
    owner: [u8; 32],
    budget: [u8; 32],
    asset: [u8; 32],
    id: [u8; 32],
    budget_call: Vec<u8>,
    store: NativeStore,
    receipts: Vec<Value>,
    records: Vec<Value>,
    sequence: u64,
    public: [u8; 32],
    key_id: [u8; 32],
    policy_override: Option<Value>,
}

impl Fixture {
    fn new(receipt_head: bool) -> Self {
        Self::with_account_fault(receipt_head, "")
    }

    fn with_account_fault(receipt_head: bool, account_fault: &str) -> Self {
        let did = format!("did:layerx:{}", hex::encode(&[0x61; 32]));
        let asset = [4; 32];
        let id = [7; 32];
        let owner_name = format!("agent:{did}:main");
        let (owner, mut owner_value) = account(&owner_name, 1, asset, 500);
        let kind = 40 + owner_name.len();
        match account_fault {
            "frozen" => owner_value[kind + 70] = 1,
            "boolean" => owner_value[kind + 53] = 2,
            "identity" => owner_value[4] ^= 1,
            "asset" => owner_value[kind + 21] ^= 1,
            "trailing" => owner_value.push(0),
            "kind" => owner_value[kind] = 2,
            "" => {}
            fault => panic!("unknown committed account defect {fault}"),
        }
        let (budget, budget_value) = account(
            &format!("agent:{did}:budget:{}", hex::encode(&id)),
            2,
            asset,
            77,
        );
        let corpus: Value = fixture_must(serde_json::from_str(include_str!(
            "../fixtures/native-module-outcomes.json"
        )));
        let receipts: Vec<_> = corpus["receipts"].as_array().unwrap()[..2]
            .iter()
            .cloned()
            .map(seal_fixture)
            .collect();
        let records = receipts.iter().map(|row| document!({"receipt_hex":row["receipt"],
            "replica_document":{"authority_replica_id":hex::encode(&[9;32]),"sequencer_public_key":row["sequencer_public_key"],
                "batch_evidence":{"header_hex":row["header"],"header_signature":row["header_signature"],
                    "receipt_proof_hex":row["receipt_proof"],"batch_identity":{"kind":"batch_maintenance_v1",
                        "receipt_hex":row["maintenance"],"receipt_proof_hex":row["maintenance_proof"],
                        "activity_receipts_hex":[row["receipt"]]}}}})).collect();
        let header = fixture_must(hex::decode(receipts[1]["header"].as_str().unwrap()));
        let sequence = if receipt_head {
            fixture_must(layerx_wire::receipt::decode_batch_header(&header)).last_sequence()
        } else {
            31
        };
        let public = fixture_must(hex::decode32(
            receipts[0]["sequencer_public_key"].as_str().unwrap(),
        ));
        let key_id = hash(&public);
        let store = NativeStore::commit(vec![
            (vec![2], sequence.to_be_bytes().to_vec()),
            ([vec![6], owner.to_vec()].concat(), owner_value),
            ([vec![6], budget.to_vec()].concat(), budget_value),
        ]);
        // The committed precompile ABI transcript supplies the real budget layout.
        let answers: Value = fixture_must(serde_json::from_str(include_str!(
            "../../../../../../tests/vectors/budget-proof-v2/node-answers.json"
        )));
        let original = answers
            .as_array()
            .unwrap()
            .iter()
            .find(|row| row["method"] == "eth_call")
            .unwrap()["result"]
            .as_str()
            .unwrap();
        let mut budget_call = fixture_must(hex::decode(original.strip_prefix("0x").unwrap()));
        budget_call[32..64].copy_from_slice(&owner);
        budget_call[64..96].copy_from_slice(&budget);
        budget_call[96..128].copy_from_slice(&asset);
        budget_call[13 * 32..14 * 32].fill(0);
        budget_call[14 * 32 - 1] = if receipt_head { 1 } else { 31 };
        Self {
            did,
            owner,
            budget,
            asset,
            id,
            budget_call,
            store,
            receipts,
            records,
            sequence,
            public,
            key_id,
            policy_override: None,
        }
    }

    fn policy(&self) -> Value {
        if let Some(policy) = &self.policy_override {
            return policy.clone();
        }
        let reference = document!({"activity_id":self.receipts[0]["activity_id"],"receipt_digest":hex::encode(&[1;32])});
        let key_policy = document!({"policy_revision":1,"required_delay_seconds":1,"maximum_delay_seconds":1,"effective_sequence":1,"evidence":reference});
        document!({"tenant":"repair-tenant","principal":"repair-principal","account_id":hex::encode(&self.owner),
            "asset_id":hex::encode(&self.asset),"activities":[self.receipts[0]["activity_id"]],"budgets":[hex::encode(&self.id)],
            "maximum_age_seconds":3600,"maximum_age_sequences":100,"identities":[{"did":self.did,
                "authorities":[{"kind":"primary_key","id":hex::encode(&[0x61;32])}],"revocation_sequence":1,"frozen":false,
                "evidence":reference,"capabilities":[],"rotation":key_policy,"recovery":key_policy}]})
    }
}

struct PeerState {
    height: u64,
    fault: &'static str,
    calls: Vec<Value>,
    called: bool,
    exchanges: Vec<Value>,
}

struct Node {
    url: String,
    address: SocketAddr,
    state: Arc<Mutex<PeerState>>,
    stopped: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

fn block(height: u64, app_hash: [u8; 32], changed: bool) -> Value {
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs();
    let mut hash = [height as u8; 32];
    if changed {
        hash[0] ^= 1;
    }
    document!({"number":format!("0x{height:x}"),"hash":format!("0x{}",hex::encode(&hash)),
        "parentHash":format!("0x{}",hex::encode(&[(height-1) as u8;32])),"timestamp":format!("0x{now:x}"),
        "transactionsRoot":format!("0x{}",hex::encode(&[3;32])),"receiptsRoot":format!("0x{}",hex::encode(&[4;32])),
        "stateRoot":format!("0x{}",hex::encode(&app_hash)),"transactions":[]})
}

impl Node {
    fn start(fixture: Arc<Fixture>) -> Self {
        Self::with_transcript(fixture, None)
    }

    fn with_transcript(fixture: Arc<Fixture>, transcript: Option<Vec<Value>>) -> Self {
        let listener = fixture_must(TcpListener::bind("127.0.0.1:0"));
        let address = fixture_must(listener.local_addr());
        fixture_must(listener.set_nonblocking(true));
        let state = Arc::new(Mutex::new(PeerState {
            height: 9,
            fault: "",
            calls: vec![],
            called: false,
            exchanges: vec![],
        }));
        let stopped = Arc::new(AtomicBool::new(false));
        let (thread_state, thread_stopped) = (state.clone(), stopped.clone());
        let worker = thread::spawn(move || {
            while !thread_stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(e) => panic!("node: {e}"),
                };
                fixture_must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                fixture_must(reader.read_line(&mut line));
                let path = line.split_whitespace().nth(1).unwrap().to_owned();
                let mut length = 0;
                loop {
                    line.clear();
                    assert_ne!(fixture_must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = fixture_must(value.trim().parse::<usize>());
                        }
                    }
                }
                assert!(length < 65536);
                let mut bytes = vec![0; length];
                fixture_must(reader.read_exact(&mut bytes));
                let request: Value = fixture_must(serde_json::from_slice(&bytes));
                let mut state = thread_state.lock().unwrap();
                state.calls.push(request.clone());
                let height = state.height;
                let result = if let Some(transcript) = &transcript {
                    transcript
                        .iter()
                        .find(|answer| {
                            answer["method"] == request["method"]
                                && answer["params"] == request["params"]
                        })
                        .unwrap_or_else(|| panic!("required corpus exchange missing: {request}"))
                        ["result"]
                        .clone()
                } else {
                    match request["method"].as_str().unwrap() {
                        "eth_chainId" => document!("0x7d"),
                        "eth_blockNumber" => document!(format!("0x{height:x}")),
                        "eth_getBlockByNumber" => {
                            let number = u64::from_str_radix(
                                request["params"][0]
                                    .as_str()
                                    .unwrap()
                                    .strip_prefix("0x")
                                    .unwrap(),
                                16,
                            )
                            .unwrap();
                            block(
                                number,
                                fixture.store.app_hash,
                                state.fault == "reorg" && state.called && number == height,
                            )
                        }
                        "px_getCommit" => {
                            assert_eq!(request["params"], document!([height.to_string()]));
                            let app = if state.fault == "apphash" {
                                [0xff; 32]
                            } else {
                                fixture.store.app_hash
                            };
                            document!({"height":height.to_string(),"signedHeader":{"header":{"height":height.to_string(),"chain_id":"125","app_hash":hex::encode(&app)},
                            "commit":{"height":height.to_string(),"signatures":[],"block_id":{"hash":hex::encode(&[height as u8;32])}}}})
                        }
                        "abci_query" => {
                            assert_eq!(path, "/comet");
                            assert_eq!(request["params"]["path"], "/store/lxphost/key");
                            assert_eq!(request["params"]["height"], (height - 1).to_string());
                            assert_eq!(request["params"]["prove"], true);
                            let key = request["params"]["data"]
                                .as_str()
                                .unwrap()
                                .strip_prefix("0x")
                                .unwrap();
                            let mut reply = fixture
                                .store
                                .queries
                                .get(key)
                                .expect("real native store key")
                                .clone();
                            reply["height"] = document!((height - 1).to_string());
                            if state.fault == "balance-proof"
                                && key == format!("06{}", hex::encode(&fixture.budget))
                            {
                                reply["proofOps"]["ops"][0]["data"] = document!(base64(&[1, 2, 3]));
                            }
                            match state.fault {
                                "proof" => {
                                    reply["proofOps"]["ops"][0]["data"] =
                                        document!(base64(&[1, 2, 3]))
                                }
                                "value" => reply["value"] = document!(base64(&[0xff; 8])),
                                "key" => reply["key"] = document!(base64(&[0xff; 33])),
                                "height" => reply["height"] = document!((height - 2).to_string()),
                                "store" => {
                                    reply["proofOps"]["ops"][1]["key"] = document!(base64(b"evm"))
                                }
                                _ => {}
                            }
                            document!({"response":reply})
                        }
                        "eth_call" => {
                            if request["params"][0]["to"]
                                .as_str()
                                .unwrap()
                                .ends_with("1014")
                            {
                                let answer = document!({"jsonrpc":"2.0","id":request["id"],
                                "error":{"code":-32000,"message":"ErrModuleRetired"}})
                                .to_string();
                                let mut stream = reader.into_inner();
                                fixture_must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
                                continue;
                            }
                            assert_eq!(request["params"][1], format!("0x{:x}", height - 1));
                            state.called = true;
                            let to = request["params"][0]["to"].as_str().unwrap();
                            let data = request["params"][0]["data"].as_str().unwrap();
                            let answer = if to.ends_with("1004") {
                                assert_eq!(
                                    data,
                                    format!(
                                        "0x{}{}",
                                        hex::encode(
                                            &layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX
                                        ),
                                        fixture.did.strip_prefix("did:layerx:").unwrap()
                                    )
                                );
                                let mut answer = vec![0; 32];
                                if state.fault != "unbound" {
                                    answer[12..].fill(0x11);
                                }
                                answer
                            } else if to.ends_with("101a") {
                                let data =
                                    fixture_must(hex::decode(data.strip_prefix("0x").unwrap()));
                                assert_eq!(&data[..4], &[0xb1, 0xff, 0xc0, 0x3e]);
                                assert_eq!(data.len(), 68);
                                assert_eq!(&data[36..], &fixture.asset);
                                let key = format!("06{}", hex::encode(&data[4..36]));
                                let row = fixture.store.queries.get(&key).expect("ledger account");
                                let bytes = fixture_must(crate::committed::base64(
                                    row["value"].as_str().unwrap(),
                                ));
                                let mut reader = layerx_wire::decode::Decoder::new(&bytes, 2048);
                                assert_eq!(fixture_must(reader.bytes(32)), &data[4..36]);
                                fixture_must(reader.bytes(512));
                                fixture_must(reader.u8());
                                let balance = fixture_must(reader.u128());
                                let mut answer = vec![0; 32];
                                answer[16..].copy_from_slice(&balance.to_be_bytes());
                                match state.fault {
                                    "ledger-mismatch" => answer[31] ^= 1,
                                    "ledger-overflow" => answer[0] = 1,
                                    "ledger-truncated" => {
                                        answer.pop();
                                    }
                                    _ => {}
                                }
                                answer
                            } else {
                                assert!(to.ends_with("101b"));
                                let get = hex::encode(
                                    &layerx_crypto::evm_transaction::transaction_hash(
                                        b"getBudget(bytes32)",
                                    )[..4],
                                );
                                let remaining = hex::encode(
                                    &layerx_crypto::evm_transaction::transaction_hash(
                                        b"budgetRemaining(bytes32)",
                                    )[..4],
                                );
                                assert!(data.ends_with(&hex::encode(&fixture.id)));
                                if data[2..].starts_with(&get) {
                                    fixture.budget_call.clone()
                                } else {
                                    assert!(data[2..].starts_with(&remaining));
                                    let mut amount = vec![0; 32];
                                    amount[31] = 77;
                                    amount
                                }
                            };
                            document!(format!("0x{}", hex::encode(&answer)))
                        }
                        "pax_sequencerReceipt" => {
                            let activity = request["params"][0]
                                .as_str()
                                .unwrap()
                                .strip_prefix("0x")
                                .unwrap();
                            let row = fixture
                                .receipts
                                .iter()
                                .find(|row| row["activity_id"] == activity)
                                .unwrap();
                            document!({"txHash":format!("0x{activity}"),"rung":if state.fault == "uncommitted" {"receipted"} else {"committed"},
                            "height":if state.fault == "receipt-height" {"0x3e9"} else {"0x3"},"blockNumber":"0x3","blockHash":format!("0x{}",hex::encode(&[if state.fault == "landing" {0xff} else {3};32])),
                            "receipt":format!("0x{}",row["receipt"].as_str().unwrap()),"keyId":format!("0x{}",hex::encode(&if state.fault == "key-id" {[0xff;32]} else {fixture.key_id})),
                            "signingKey":format!("0x{}",hex::encode(&if state.fault == "signing-key" {[0xff;32]} else {fixture.public}))})
                        }
                        method => panic!("node does not produce hosted gateway joins: {method}"),
                    }
                };
                state.exchanges.push(document!({"method":request["method"],"params":request["params"],"result":result}));
                let answer =
                    document!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                let mut stream = reader.into_inner();
                fixture_must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
            }
        });
        Self {
            url: format!("http://{address}/evm"),
            address,
            state,
            stopped,
            worker: Some(worker),
        }
    }

    fn fault(&self, fault: &'static str) {
        let mut state = self.state.lock().unwrap();
        state.fault = fault;
        state.called = false;
    }
}

impl Drop for Node {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        self.worker.take().unwrap().join().unwrap();
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let mut nonce = [0; 12];
        fixture_must(getrandom::fill(&mut nonce));
        let root =
            std::env::temp_dir().join(format!("authority-repair-1471-{}", hex::encode(&nonce)));
        fixture_must(fs::create_dir(&root));
        fixture_must(fs::set_permissions(
            &root,
            fs::Permissions::from_mode(0o700),
        ));
        fixture_must(fs::create_dir(root.join("state")));
        fixture_must(fs::set_permissions(
            root.join("state"),
            fs::Permissions::from_mode(0o700),
        ));
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fixture_must(fs::remove_dir_all(&self.0));
    }
}

fn config_fixture(node: &Node, fixture: &Fixture, directory: &Path) -> Config {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let tls = fixture_must(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(
                vec![CertificateDer::from(
                    include_bytes!("../../src/human/fixtures/server.der").to_vec(),
                )],
                fixture_must(PrivateKeyDer::try_from(
                    include_bytes!("../../src/human/fixtures/server-key.der").to_vec(),
                )),
            ),
    );
    let keys = directory.join("keys.json");
    fixture_must(fs::write(&keys,document!([{"keyId":base64(&fixture.key_id),"publicKey":base64(&fixture.public),"firstHeight":1,"lastHeight":1000}]).to_string()));
    fixture_must(fs::set_permissions(
        &keys,
        fs::Permissions::from_mode(0o600),
    ));
    let policy = fixture.policy();
    fixture_must(fs::write(
        directory.join("policy.json"),
        document!({"principals":[policy]}).to_string(),
    ));
    fixture_must(fs::set_permissions(
        directory.join("policy.json"),
        fs::Permissions::from_mode(0o600),
    ));
    let mut config = Config {
        listen: node.address,
        tls: Arc::new(tls),
        tokens: vec![],
        human: None,
        replica_address: node.address,
        replica_host: node.address.to_string(),
        replica_token: Zeroizing::new("replica-208".to_owned()),
        evidence_read_token: None,
        replica_id: [9; 32],
        chain_receipts: fixture_must(ChainReceipts::load(&node.url, &keys)),
        chain_rpc_url: node.url.clone(),
        protocol_network_id: 77,
        network_id: "repair-1471".to_owned(),
        wire_version: "3".to_owned(),
        authorization: SequencerAuthorization::new(fixture.key_id, fixture.public, 1, 1000),
        sequencer_public_key: fixture.public,
    };
    config.fixture_human_policy(&fixture.policy(), directory);
    config
}

fn retain_records(config: &Config, records: &[Value]) {
    let human = config.human.as_ref().unwrap();
    for record in records {
        fixture_must(human.retain(
            &fixture_must(hex::decode(record["receipt_hex"].as_str().unwrap())),
            &fixture_must(serde_json::to_vec(&record["replica_document"])),
            config,
        ));
    }
}

fn tls_request(config: Arc<Config>, path: &str, token: &str) -> (u16, Value) {
    let listener = fixture_must(TcpListener::bind("127.0.0.1:0"));
    let address = fixture_must(listener.local_addr());
    let server = thread::spawn(move || {
        let (stream, _) = fixture_must(listener.accept());
        fixture_must(handle_connection(&config, stream, &AtomicBool::new(false)));
    });
    let mut roots = RootCertStore::empty();
    fixture_must(roots.add(CertificateDer::from(
        include_bytes!("../../src/human/fixtures/ca.der").to_vec(),
    )));
    let client = rustls::ClientConfig::builder()
        .with_root_certificates(roots)
        .with_no_client_auth();
    let connection = fixture_must(rustls::ClientConnection::new(
        Arc::new(client),
        fixture_must(rustls::pki_types::ServerName::try_from("localhost")),
    ));
    let tcp = fixture_must(TcpStream::connect(address));
    fixture_must(tcp.set_read_timeout(Some(Duration::from_secs(10))));
    let mut stream = StreamOwned::new(connection, tcp);
    fixture_must(write!(stream,"GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nConnection: close\r\n\r\n"));
    let mut answer = Vec::new();
    fixture_must(stream.read_to_end(&mut answer));
    server.join().unwrap();
    let offset = answer.windows(4).position(|v| v == b"\r\n\r\n").unwrap();
    let header = std::str::from_utf8(&answer[..offset]).unwrap();
    let status = header.split_whitespace().nth(1).unwrap().parse().unwrap();
    (
        status,
        fixture_must(serde_json::from_slice(&answer[offset + 4..])),
    )
}

#[path = "../../src/human/budget_state/record.rs"]
mod committed_budget_record;
#[path = "../retired_client_modules.rs"]
mod retirement_guards;

fn replace_fixture_bytes(bytes: &mut [u8], original: &[u8], replacement: &[u8]) {
    assert_eq!(original.len(), replacement.len());
    let offsets: Vec<_> = bytes
        .windows(original.len())
        .enumerate()
        .filter_map(|(offset, value)| (value == original).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1, "unique canonical fixture field");
    bytes[offsets[0]..offsets[0] + original.len()].copy_from_slice(replacement);
}

// The peer's state is an in-memory committed store. Session payloads come from
// the production issuer; the captured native receipt supplies its canonical
// layout and identity state. Reseal the peer's envelope for this validity window
// rather than changing any consumer clock, lifetime bound or refusal check.
fn identity_fixture(capability: bool) -> Fixture {
    let mut fixture = Fixture::new(false);
    let mut bytes = if capability {
        include_bytes!("../fixtures/native-sessions/legacy.receipt").to_vec()
    } else {
        include_bytes!("../../../../../../tests/fixtures/governance/owner-rotation/receipt")
            .to_vec()
    };
    let captured_key = if capability {
        *include_bytes!("../fixtures/native-sessions/legacy.sequencer-public")
    } else {
        *include_bytes!(
            "../../../../../../tests/fixtures/governance/owner-rotation/sequencer-public"
        )
    };
    fixture_must(layerx_proof::receipt::verify_sequencer_signature(
        &bytes,
        captured_key,
    ));
    let original = fixture_must(layerx_wire::receipt::decode(&bytes));
    let original = original.protocol().unwrap();
    let state = original
        .effects()
        .iter()
        .find(|e| e.event_type() == 0x7110)
        .unwrap()
        .body()
        .to_vec();
    let did = if capability {
        format!("did:layerx:{}", hex::encode(&state[37..69]))
    } else {
        "did:layerx:042bb23f93d99cf87ee8cd5428a149217a94200afce832589629d75f309c833a".to_owned()
    };
    let did_id = fixture_must(layerx_wire::hash::did_id_for_protocol(
        &fixture_must(layerx_types::ids::Did::new(did.as_bytes())),
        3,
    ));
    assert_eq!(&state[5..37], &did_id);
    let number = |offset| u64::from_be_bytes(state[offset..offset + 8].try_into().unwrap());
    let mut rows = vec![([vec![1, 0, 7], did_id.to_vec()].concat(), state.clone())];
    let mut scope = None;
    if capability {
        use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_millis() as u64;
        let issued = fixture_must(issue_session_key(&SessionKeyRequest {
            grantor: did_id,
            session_public_key: ed25519_dalek::SigningKey::from_bytes(&[0x63; 32])
                .verifying_key()
                .to_bytes(),
            not_before: now - 1000,
            expires_at: Some(now + 3_600_000),
            permitted_activity_types: vec![fixture_must(layerx_types::payload::ActivityType::new(
                layerx_types::payload::ModuleId::Asset,
                3,
            ))],
            revocation_sequence: Some(number(69)),
            fee_budget: None,
            purpose: SessionPurpose::Activity,
        }));
        let original_summary = original
            .effects()
            .iter()
            .find(|e| e.event_type() == 0x7145)
            .unwrap()
            .body();
        let mut summary = original_summary.to_vec();
        summary[5..37].copy_from_slice(&issued.grant_id);
        summary[133..165].copy_from_slice(&issued.session_public_key);
        summary[173..181].copy_from_slice(&2_u64.to_be_bytes());
        summary[181..183].copy_from_slice(&3_u16.to_be_bytes());
        summary[183..185].copy_from_slice(&3_u16.to_be_bytes());
        summary[185..193].copy_from_slice(&issued.not_before.to_be_bytes());
        summary[193..201].copy_from_slice(&issued.expires_at.to_be_bytes());
        replace_fixture_bytes(&mut bytes, original_summary, &summary);
        let original_payload: Vec<_> = original
            .effects()
            .iter()
            .filter(|e| matches!(e.event_type(), 0x7105 | 0x7125))
            .flat_map(|e| e.body().iter().copied())
            .collect();
        assert_eq!(original_payload.len(), issued.registration_payload.len());
        let mut offset = 0;
        for effect in original
            .effects()
            .iter()
            .filter(|e| matches!(e.event_type(), 0x7105 | 0x7125))
        {
            let next = offset + effect.body().len();
            replace_fixture_bytes(
                &mut bytes,
                effect.body(),
                &issued.registration_payload[offset..next],
            );
            offset = next;
        }
        // Native fixtures have neither supply nor Programs outcomes.
        assert!(original.total_units().is_none() && original.program_outcome().is_none());
        let timestamp = bytes.len() - 1 - 4 - 64 - 8;
        bytes[timestamp..timestamp + 8].copy_from_slice(&now.to_be_bytes());
        rows.push((
            [vec![1, 0, 7, 0x15], issued.grant_id.to_vec()].concat(),
            summary.clone(),
        ));
        rows.push((
            [vec![1, 0, 7, 5], issued.grant_id.to_vec()].concat(),
            issued.registration_payload,
        ));
        scope = Some(summary);
    }
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x22; 32]);
    let public = signer.verifying_key().to_bytes();
    let key_id = hash(&public);
    let unsigned = fixture_must(layerx_wire::receipt::encode_unsigned(&fixture_must(
        layerx_wire::receipt::decode(&bytes),
    )));
    let end = bytes.len();
    bytes[end - 64..].copy_from_slice(
        &signer
            .sign(&fixture_must(layerx_wire::hash::receipt_digest(&unsigned)))
            .to_bytes(),
    );
    let decoded = fixture_must(layerx_proof::receipt::verify_sequencer_signature(
        &bytes, public,
    ));
    let receipt = decoded.protocol().unwrap();
    let batch = (1..=1000)
        .find(|batch| {
            fixture_must(layerx_wire::hash::program_execution_batch_id(
                receipt.previous_state_root(),
                receipt.activity_root(),
                receipt.global_sequence(),
                receipt.global_sequence(),
                *batch,
            )) == receipt.batch_id()
        })
        .expect("captured native execution batch");
    let mut encoder = layerx_wire::encode::Encoder::new(354);
    fixture_must(encoder.structure_header_version(0x1701, 3));
    fixture_must(encoder.u8(15));
    fixture_must(encoder.tag(1, 15));
    fixture_must(encoder.u16(3));
    fixture_must(encoder.tag(2, 15));
    fixture_must(encoder.u32(77));
    for (tag, value) in [
        (3, 1),
        (4, batch),
        (5, receipt.global_sequence()),
        (6, receipt.global_sequence()),
    ] {
        fixture_must(encoder.tag(tag, 15));
        fixture_must(encoder.u64(value));
    }
    for (tag, value) in [
        (7, receipt.previous_state_root()),
        (8, receipt.resulting_state_root()),
        (9, receipt.activity_root()),
        (10, fixture_must(layerx_wire::hash::merkle_leaf(&bytes))),
        (11, [0; 32]),
        (12, [0; 32]),
        (13, [0; 32]),
    ] {
        fixture_must(encoder.tag(tag, 15));
        fixture_must(encoder.bytes(&value, 32));
    }
    fixture_must(encoder.tag(14, 15));
    fixture_must(encoder.u64(receipt.timestamp()));
    fixture_must(encoder.tag(15, 15));
    fixture_must(encoder.bytes(&key_id, 32));
    let header = encoder.finish();
    assert_eq!(header.len(), 354);
    let (proof, _) = fixture_must(layerx_proof::merkle::build_proof(&[&bytes], 0));
    let document = document!({"authority_replica_id":hex::encode(&[9;32]),"sequencer_public_key":hex::encode(&public),
        "batch_evidence":{"header_hex":hex::encode(&header),"header_signature":hex::encode(&signer.sign(&fixture_must(layerx_wire::hash::batch_header_digest(&header))).to_bytes()),"receipt_proof_hex":wire_proof(&proof)}});
    let locator = fixture_must(layerx_platform_authority::receipt_locator(&bytes));
    let reference = document!({"activity_id":hex::encode(&locator.activity_id),"receipt_digest":hex::encode(&locator.receipt_digest)});
    let rotation = if capability {
        document!({"policy_revision":1,"required_delay_seconds":1,"maximum_delay_seconds":1,
            "effective_sequence":receipt.global_sequence(),"evidence":reference})
    } else {
        document!({"policy_revision":number(167),"required_delay_seconds":number(199).div_ceil(1000),
            "maximum_delay_seconds":number(207)/1000,"effective_sequence":receipt.global_sequence(),"evidence":reference})
    };
    let capabilities = scope.iter().map(|summary| document!({"authority":hex::encode(&summary[69..101]),"action_key":hex::encode(&summary[101..133]),
        "capability_id":hex::encode(&summary[5..37]),"activity_types":[3],"counterparties":[],"assets":[],"amount_ceiling":"0",
        "expiry_sequence":u64::from_be_bytes(summary[165..173].try_into().unwrap()),"enforceable_dimensions":[],"evidence":reference})).collect::<Vec<_>>();
    let mut policy = fixture.policy();
    policy["activities"] = document!([hex::encode(&locator.activity_id)]);
    // Keep the original identity TLS test's historical-receipt freshness bound.
    policy["maximum_age_seconds"] = document!(if capability { 3600 } else { 315_360_000 });
    policy["identities"] = document!([{"did":did,"authorities":[{"kind":"primary_key","id":hex::encode(&state[37..69])}],
        "revocation_sequence":number(69),"frozen":false,"evidence":reference,"rotation":rotation,"recovery":rotation,"capabilities":capabilities}]);
    fixture.did = did;
    fixture.sequence = receipt.global_sequence();
    fixture.public = public;
    fixture.key_id = key_id;
    fixture.store = NativeStore::commit(rows);
    fixture.receipts = vec![
        document!({"activity_id":hex::encode(&locator.activity_id),"receipt":hex::encode(&bytes)}),
    ];
    fixture.records =
        vec![document!({"receipt_hex":hex::encode(&bytes),"replica_document":document})];
    fixture.policy_override = Some(policy);
    fixture
}

fn human_path(name: &str) -> String {
    format!("/v1/agent/{name}?tenant=repair-tenant&principal=repair-principal")
}

fn budget_path(name: &str, fixture: &Fixture) -> String {
    format!(
        "{}&budget_id={}",
        human_path(name),
        hex::encode(&fixture.id)
    )
}
