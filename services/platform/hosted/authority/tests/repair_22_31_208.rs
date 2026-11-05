include!("../src/main.rs");

use chain_readiness_tests::{base64, must};
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
                must(crate::committed::verify_membership(
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
    let id = must(layerx_wire::hash::account_id_for_protocol(
        &must(layerx_types::account::AccountId::parse(name)),
        3,
    ));
    let mut writer = Encoder::new(2048);
    must(writer.bytes(&id, 32));
    must(writer.bytes(name.as_bytes(), 512));
    must(writer.u8(kind));
    must(writer.u128(balance));
    must(writer.bytes(&asset, 32));
    must(writer.u8(1));
    must(writer.u64(2));
    must(writer.u64(1));
    must(writer.u8(0));
    must(writer.u8(0));
    must(writer.bytes(&[0x61; 32], 32));
    must(writer.u8(1));
    (id, writer.finish())
}

fn wire_proof(proof: &layerx_proof::merkle::Proof) -> String {
    let mut writer = layerx_wire::encode::Encoder::new(2048);
    must(writer.structure_header(0x4d50));
    must(writer.u32(proof.leaf_index()));
    must(writer.u32(proof.leaf_count()));
    must(writer.u8(u8::try_from(proof.siblings().len()).unwrap()));
    must(writer.bytes(&proof.siblings().concat(), 1024));
    hex::encode(&writer.finish())
}

fn seal_fixture(mut row: Value) -> Value {
    // Preserve the native receipt, including its zero context; only reseal the header's key identity.
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x22; 32]);
    let public = signer.verifying_key().to_bytes();
    assert_eq!(hex::encode(&public), row["sequencer_public_key"]);
    let key_id = hash(&public);
    let bytes = must(hex::decode(row["receipt"].as_str().unwrap()));
    let original = must(layerx_proof::receipt::verify_sequencer_signature(
        &bytes, public,
    ));
    let protocol = original.protocol().unwrap();
    assert_eq!(protocol.context_hash(), [0; 32]);
    let maintenance = must(hex::decode(row["maintenance"].as_str().unwrap()));
    let leaves = [&bytes[..], &maintenance[..]];
    let (proof, root) = must(layerx_proof::merkle::build_proof(&leaves, 0));
    let (maintenance_proof, _) = must(layerx_proof::merkle::build_proof(&leaves, 1));
    let mut header = must(hex::decode(row["header"].as_str().unwrap()));
    let original = must(layerx_wire::receipt::decode_batch_header(&header));
    for (old, new) in [
        (original.sequencer_id(), key_id),
        (original.receipt_merkle_root(), root),
    ] {
        let offset = header.windows(32).position(|value| value == old).unwrap();
        header[offset..offset + 32].copy_from_slice(&new);
    }
    row["header_signature"] = document!(hex::encode(
        &signer
            .sign(&must(layerx_wire::hash::batch_header_digest(&header)))
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
    let evidence = must(layerx_platform_authority::parse_replica_evidence(
        &must(serde_json::to_vec(&document)),
        [9; 32],
        public,
    ));
    let facts = must(layerx_platform_authority::authorized_batch_by_activity(
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
        let corpus: Value = must(serde_json::from_str(include_str!(
            "fixtures/native-module-outcomes.json"
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
        let header = must(hex::decode(receipts[1]["header"].as_str().unwrap()));
        let sequence = if receipt_head {
            must(layerx_wire::receipt::decode_batch_header(&header)).last_sequence()
        } else {
            31
        };
        let public = must(hex::decode32(
            receipts[0]["sequencer_public_key"].as_str().unwrap(),
        ));
        let key_id = hash(&public);
        let store = NativeStore::commit(vec![
            (vec![2], sequence.to_be_bytes().to_vec()),
            ([vec![6], owner.to_vec()].concat(), owner_value),
            ([vec![6], budget.to_vec()].concat(), budget_value),
        ]);
        // The committed precompile ABI transcript supplies the real budget layout.
        let answers: Value = must(serde_json::from_str(include_str!(
            "../../../../../tests/vectors/budget-proof-v2/node-answers.json"
        )));
        let original = answers
            .as_array()
            .unwrap()
            .iter()
            .find(|row| row["method"] == "eth_call")
            .unwrap()["result"]
            .as_str()
            .unwrap();
        let mut budget_call = must(hex::decode(original.strip_prefix("0x").unwrap()));
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
        }
    }

    fn policy(&self) -> Value {
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
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        must(listener.set_nonblocking(true));
        let state = Arc::new(Mutex::new(PeerState {
            height: 9,
            fault: "",
            calls: vec![],
            called: false,
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
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                must(reader.read_line(&mut line));
                let path = line.split_whitespace().nth(1).unwrap().to_owned();
                let mut length = 0;
                loop {
                    line.clear();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = must(value.trim().parse::<usize>());
                        }
                    }
                }
                assert!(length < 65536);
                let mut bytes = vec![0; length];
                must(reader.read_exact(&mut bytes));
                let request: Value = must(serde_json::from_slice(&bytes));
                let mut state = thread_state.lock().unwrap();
                state.calls.push(request.clone());
                let height = state.height;
                let result = match request["method"].as_str().unwrap() {
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
                                reply["proofOps"]["ops"][0]["data"] = document!(base64(&[1, 2, 3]))
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
                        assert_eq!(request["params"][1], format!("0x{:x}", height - 1));
                        state.called = true;
                        let to = request["params"][0]["to"].as_str().unwrap();
                        let data = request["params"][0]["data"].as_str().unwrap();
                        let answer = if to.ends_with("1004") {
                            assert_eq!(
                                data,
                                format!(
                                    "0x{}{}",
                                    hex::encode(&layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX),
                                    hex::encode(&[0x61; 32])
                                )
                            );
                            let mut answer = vec![0; 32];
                            if state.fault != "unbound" {
                                answer[12..].fill(0x11);
                            }
                            answer
                        } else if to.ends_with("101a") {
                            let data = must(hex::decode(data.strip_prefix("0x").unwrap()));
                            assert_eq!(&data[..4], &[0xb1, 0xff, 0xc0, 0x3e]);
                            assert_eq!(data.len(), 68);
                            assert_eq!(&data[36..], &fixture.asset);
                            let key = format!("06{}", hex::encode(&data[4..36]));
                            let row = fixture.store.queries.get(&key).expect("ledger account");
                            let bytes =
                                must(crate::committed::base64(row["value"].as_str().unwrap()));
                            let mut reader = layerx_wire::decode::Decoder::new(&bytes, 2048);
                            assert_eq!(must(reader.bytes(32)), &data[4..36]);
                            must(reader.bytes(512));
                            must(reader.u8());
                            let balance = must(reader.u128());
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
                };
                let answer =
                    document!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
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
        must(getrandom::fill(&mut nonce));
        let root =
            std::env::temp_dir().join(format!("authority-repair-208-{}", hex::encode(&nonce)));
        must(fs::create_dir(&root));
        must(fs::set_permissions(
            &root,
            fs::Permissions::from_mode(0o700),
        ));
        must(fs::create_dir(root.join("state")));
        must(fs::set_permissions(
            root.join("state"),
            fs::Permissions::from_mode(0o700),
        ));
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

fn config_fixture(node: &Node, fixture: &Fixture, directory: &Path) -> Config {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let tls = must(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(
                vec![CertificateDer::from(
                    include_bytes!("../src/human/fixtures/server.der").to_vec(),
                )],
                must(PrivateKeyDer::try_from(
                    include_bytes!("../src/human/fixtures/server-key.der").to_vec(),
                )),
            ),
    );
    let keys = directory.join("keys.json");
    must(fs::write(&keys,document!([{"keyId":base64(&fixture.key_id),"publicKey":base64(&fixture.public),"firstHeight":1,"lastHeight":1000}]).to_string()));
    must(fs::set_permissions(
        &keys,
        fs::Permissions::from_mode(0o600),
    ));
    let policy = fixture.policy();
    must(fs::write(
        directory.join("policy.json"),
        document!({"principals":[policy]}).to_string(),
    ));
    must(fs::set_permissions(
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
        chain_receipts: must(ChainReceipts::load(&node.url, &keys)),
        chain_rpc_url: node.url.clone(),
        protocol_network_id: 77,
        network_id: "repair-208".to_owned(),
        wire_version: "3".to_owned(),
        authorization: SequencerAuthorization::new(fixture.key_id, fixture.public, 1, 1000),
        sequencer_public_key: fixture.public,
    };
    config.fixture_human_policy(&fixture.policy(), directory);
    config
}

fn successful(response: Result<Response, Response>) -> Value {
    let response =
        response.unwrap_or_else(|r| panic!("{}: {}", r.status, String::from_utf8_lossy(&r.body)));
    assert_eq!(response.status, 200);
    must(serde_json::from_slice(&response.body))
}

fn retain_records(config: &Config, records: &[Value]) {
    let human = config.human.as_ref().unwrap();
    for record in records {
        must(human.retain(
            &must(hex::decode(record["receipt_hex"].as_str().unwrap())),
            &must(serde_json::to_vec(&record["replica_document"])),
            config,
        ));
    }
}

#[test]
fn mixed_height_and_sequence_authenticates_multiple_receipts_and_empty_blocks() {
    let fixture = Arc::new(Fixture::new(true));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = config_fixture(&node, &fixture, &directory.0);
    retain_records(&config, &fixture.records);
    assert_eq!(must(fs::read_dir(directory.0.join("state"))).count(), 2);
    drop(config);
    let config = config_fixture(&node, &fixture, &directory.0);
    assert_ne!(fixture.sequence, 9);
    let first = successful(config.fixture_subject_head(
        &fixture.policy(),
        &fixture.did,
        fixture.owner,
        fixture.asset,
    ));
    assert_eq!(first["committed_height"], 9);
    assert_eq!(first["state_height"], 8);
    assert_eq!(first["receipt_sequence"], fixture.sequence);
    assert_eq!(first["block_hash"], hex::encode(&[9; 32]));
    let calls = &node.state.lock().unwrap().calls.clone();
    assert_eq!(
        calls
            .iter()
            .filter(|r| r["method"] == "pax_sequencerReceipt")
            .count(),
        2
    );
    // Both genuine native receipts landed in block 3; blocks 4..10 have no relevant receipts.
    node.state.lock().unwrap().height = 10;
    let later = successful(config.fixture_subject_head(
        &fixture.policy(),
        &fixture.did,
        fixture.owner,
        fixture.asset,
    ));
    assert_eq!(later["committed_height"], 10);
    assert_eq!(later["state_height"], 9);
    assert_eq!(later["receipt_sequence"], first["receipt_sequence"]);
    assert_ne!(later["block_hash"], first["block_hash"]);
    let incomplete_directory = Directory::new();
    let incomplete = config_fixture(&node, &fixture, &incomplete_directory.0);
    retain_records(&incomplete, &fixture.records[..1]);
    assert!(incomplete
        .fixture_subject_head(
            &fixture.policy(),
            &fixture.did,
            fixture.owner,
            fixture.asset,
        )
        .is_err());
    for fault in [
        "proof",
        "value",
        "key",
        "height",
        "store",
        "apphash",
        "landing",
        "uncommitted",
        "reorg",
        "receipt-height",
        "key-id",
        "signing-key",
    ] {
        node.fault(fault);
        assert!(
            config
                .fixture_subject_head(
                    &fixture.policy(),
                    &fixture.did,
                    fixture.owner,
                    fixture.asset,
                )
                .is_err(),
            "accepted {fault}"
        );
    }
}

#[test]
fn native_receipt_authority_requires_pinned_header_key_signature_and_range() {
    let fixture = Arc::new(Fixture::new(true));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let mut config = config_fixture(&node, &fixture, &directory.0);
    let record = &fixture.records[0];
    let receipt = must(hex::decode(record["receipt_hex"].as_str().unwrap()));
    let decoded = must(layerx_wire::receipt::decode(&receipt));
    assert_eq!(decoded.protocol().unwrap().context_hash(), [0; 32]);
    let document = &record["replica_document"];
    let (_, authority) = must(trust::replica(
        &config,
        &receipt,
        &must(serde_json::to_vec(document)),
    ));
    assert_eq!(authority.sequencer_id(), fixture.key_id);
    assert_eq!(authority.public_key(), fixture.public);
    for fault in ["key", "header-key", "signature", "replica"] {
        let mut changed = document.clone();
        match fault {
            "key" => changed["sequencer_public_key"] = document!(hex::encode(&[0xff; 32])),
            "replica" => changed["authority_replica_id"] = document!(hex::encode(&[0xff; 32])),
            "header-key" => {
                let mut header = must(hex::decode(
                    changed["batch_evidence"]["header_hex"].as_str().unwrap(),
                ));
                let offset = header
                    .windows(32)
                    .position(|bytes| bytes == fixture.key_id)
                    .unwrap();
                header[offset] ^= 1;
                // Even a correctly signed header must identify the pinned key.
                let signer = ed25519_dalek::SigningKey::from_bytes(&[0x22; 32]);
                changed["batch_evidence"]["header_signature"] = document!(hex::encode(
                    &signer
                        .sign(&must(layerx_wire::hash::batch_header_digest(&header)))
                        .to_bytes()
                ));
                changed["batch_evidence"]["header_hex"] = document!(hex::encode(&header));
            }
            "signature" => {
                let mut signature = must(hex::decode(
                    changed["batch_evidence"]["header_signature"]
                        .as_str()
                        .unwrap(),
                ));
                signature[0] ^= 1;
                changed["batch_evidence"]["header_signature"] = document!(hex::encode(&signature));
            }
            _ => unreachable!(),
        }
        assert!(
            trust::replica(&config, &receipt, &must(serde_json::to_vec(&changed))).is_err(),
            "accepted {fault}"
        );
        assert!(
            config
                .human
                .as_ref()
                .unwrap()
                .retain(&receipt, &must(serde_json::to_vec(&changed)), &config)
                .is_err(),
            "writer accepted {fault}"
        );
    }
    let keys = directory.0.join("keys.json");
    must(fs::write(&keys, document!([{"keyId":base64(&fixture.key_id),"publicKey":base64(&fixture.public),"firstHeight":1000,"lastHeight":1000}]).to_string()));
    config.chain_receipts = must(ChainReceipts::load(&node.url, &keys));
    assert!(trust::replica(&config, &receipt, &must(serde_json::to_vec(document))).is_err());
}

fn tls_request(config: Arc<Config>, path: &str, token: &str) -> (u16, Value) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    let server = thread::spawn(move || {
        let (stream, _) = must(listener.accept());
        must(handle_connection(&config, stream, &AtomicBool::new(false)));
    });
    let mut roots = RootCertStore::empty();
    must(roots.add(CertificateDer::from(
        include_bytes!("../src/human/fixtures/ca.der").to_vec(),
    )));
    let client = rustls::ClientConfig::builder()
        .with_root_certificates(roots)
        .with_no_client_auth();
    let connection = must(rustls::ClientConnection::new(
        Arc::new(client),
        must(rustls::pki_types::ServerName::try_from("localhost")),
    ));
    let tcp = must(TcpStream::connect(address));
    must(tcp.set_read_timeout(Some(Duration::from_secs(10))));
    let mut stream = StreamOwned::new(connection, tcp);
    must(write!(stream,"GET {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {token}\r\nConnection: close\r\n\r\n"));
    let mut answer = Vec::new();
    must(stream.read_to_end(&mut answer));
    server.join().unwrap();
    let offset = answer.windows(4).position(|v| v == b"\r\n\r\n").unwrap();
    let header = std::str::from_utf8(&answer[..offset]).unwrap();
    let status = header.split_whitespace().nth(1).unwrap().parse().unwrap();
    (status, must(serde_json::from_slice(&answer[offset + 4..])))
}

#[test]
fn budget_gateway_wiring_reads_authenticated_native_state_through_authority_tls() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let path = format!(
        "/v1/agent/budget-proof?tenant=repair-tenant&principal=repair-principal&budget_id={}",
        hex::encode(&fixture.id)
    );
    assert_eq!(tls_request(config.clone(), &path, "wrong-token").0, 401);
    let (status, proof) = tls_request(config.clone(), &path, "repair-human-token-208");
    assert_eq!(status, 200, "{proof}");
    assert_eq!(proof["budget_state"]["remaining"], "77");
    assert_eq!(proof["budget_state"]["observed_head_sequence"], 31);
    assert_eq!(proof["head"]["committed_height"], 9);
    assert_eq!(proof["finality"]["state_height"], 8);
    assert_eq!(proof["proofs"]["budget_call"]["height"], 8);
    assert_eq!(proof["proofs"]["remaining_call"]["height"], 8);
    assert_eq!(proof["proofs"]["account_join"]["binding_call"]["height"], 8);
    assert_eq!(
        proof["proofs"]["account_join"]["native_account_proof"]["height"],
        "8"
    );
    assert_eq!(
        proof["proofs"]["account_join"]["account"]["layerx_account"],
        hex::encode(&fixture.owner)
    );
    assert_eq!(proof["accounts"]["budget"], hex::encode(&fixture.budget));
    assert_eq!(
        proof["proofs"]["account_join"]["balances"][0]["amount"],
        "500"
    );
    assert_eq!(
        proof["proofs"]["account_join"]["balances"][1]["amount"],
        "77"
    );
    assert_eq!(
        proof["proofs"]["account_join"]["receipt_sequence_proof"]["height"],
        "8"
    );
    let calls = node.state.lock().unwrap().calls.clone();
    assert!(!calls.iter().any(|row| matches!(
        row["method"].as_str(),
        Some("px_getAccount" | "px_getBalances")
    )));
    assert!(calls
        .iter()
        .filter(|r| r["method"] == "eth_call")
        .all(|r| r["params"][1] == "0x8"));
    let ledger_calls: Vec<_> = calls
        .iter()
        .filter(|row| {
            row["method"] == "eth_call"
                && row["params"][0]["to"]
                    .as_str()
                    .is_some_and(|to| to.ends_with("101a"))
        })
        .collect();
    assert_eq!(ledger_calls.len(), 2);
    for (call, id) in ledger_calls.iter().zip([fixture.owner, fixture.budget]) {
        assert_eq!(
            call["params"][0]["data"],
            format!(
                "0xb1ffc03e{}{}",
                hex::encode(&id),
                hex::encode(&fixture.asset)
            )
        );
    }
    let queried: Vec<_> = calls
        .iter()
        .filter(|r| r["method"] == "abci_query")
        .map(|r| r["params"]["data"].clone())
        .collect();
    for id in [fixture.owner, fixture.budget] {
        assert!(queried.contains(&document!(format!("0x06{}", hex::encode(&id)))));
    }
    for fault in [
        "proof",
        "value",
        "key",
        "height",
        "store",
        "apphash",
        "unbound",
        "balance-proof",
        "ledger-mismatch",
        "ledger-overflow",
        "ledger-truncated",
        "reorg",
    ] {
        node.fault(fault);
        let (status, answer) = tls_request(config.clone(), &path, "repair-human-token-208");
        assert_eq!(status, 503, "accepted {fault}: {answer}");
    }
}

#[test]
fn authenticated_native_accounts_still_require_canonical_identity_asset_and_flags() {
    for fault in ["frozen", "boolean", "identity", "asset", "trailing", "kind"] {
        let fixture = Arc::new(Fixture::with_account_fault(false, fault));
        let node = Node::start(fixture.clone());
        let directory = Directory::new();
        let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
        let path = format!(
            "/v1/agent/budget-proof?tenant=repair-tenant&principal=repair-principal&budget_id={}",
            hex::encode(&fixture.id)
        );
        let (status, answer) = tls_request(config, &path, "repair-human-token-208");
        assert_eq!(status, 503, "accepted committed account {fault}: {answer}");
    }
}
