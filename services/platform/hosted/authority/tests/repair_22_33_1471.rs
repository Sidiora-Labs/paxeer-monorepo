// Task record 22.33.1471 / audit 22.30.85 / acceptance 22998.1.
// budget-export-corpus-optional -> budget_export_corpus_optional
// human-tls-retired-anchor -> human_tls_retired_anchor
// human-budget-corpus-optional -> human_budget_corpus_optional
// retirement-guard-requires-anchor -> retirement_guard_requires_anchor
// Baseline: not run; implementation-first and hf-done-only instructions take
// precedence over the contradictory baseline step. No baseline failure claimed.
// Evidence: the private hf-build/hf-done ledger records the verified candidate.
// Wiring: services/platform/hosted/testnet/tests (platform/domain8) owns live-node
// capture qualification; this gate uses canonical in-memory peer stores and TLS.
include!("../src/main.rs");

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
                                        hex::encode(&[0x61; 32])
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
                    include_bytes!("../src/human/fixtures/server.der").to_vec(),
                )],
                fixture_must(PrivateKeyDer::try_from(
                    include_bytes!("../src/human/fixtures/server-key.der").to_vec(),
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
        include_bytes!("../src/human/fixtures/ca.der").to_vec(),
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

#[path = "../src/human/budget_state/record.rs"]
mod committed_budget_record;
#[path = "retired_client_modules.rs"]
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
        include_bytes!("fixtures/native-sessions/legacy.receipt").to_vec()
    } else {
        include_bytes!("../../../../../tests/fixtures/governance/owner-rotation/receipt").to_vec()
    };
    let captured_key = if capability {
        *include_bytes!("fixtures/native-sessions/legacy.sequencer-public")
    } else {
        *include_bytes!("../../../../../tests/fixtures/governance/owner-rotation/sequencer-public")
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

fn assert_no_retired_calls(node: &Node) {
    let calls = node.state.lock().unwrap().calls.clone();
    assert!(!calls.is_empty());
    assert!(calls.iter().any(|r| r["method"] == "px_getCommit"));
    assert!(calls.iter().any(|r| r["method"] == "abci_query"));
    assert!(!calls
        .iter()
        .any(|r| r["method"] == "eth_call"
            && r["params"][0]["to"].as_str().unwrap().ends_with("1014")));
    for signature in [b"latestFinalized()".as_slice(), b"checkpoint(uint64)"] {
        let selector =
            hex::encode(&layerx_crypto::evm_transaction::transaction_hash(signature)[..4]);
        assert!(!calls.iter().any(|r| r["method"] == "eth_call"
            && r["params"][0]["data"]
                .as_str()
                .unwrap()
                .starts_with(&format!("0x{selector}"))));
    }
}

pub(crate) struct ProducedCorpus {
    directory: Directory,
    pub(crate) path: PathBuf,
}

pub(crate) fn produced_export_corpus() -> ProducedCorpus {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let (status, export) = tls_request(
        config,
        &budget_path("budget-proof", &fixture),
        "repair-human-token-208",
    );
    assert_eq!(status, 200, "{export}");
    assert_no_retired_calls(&node);
    let response = directory.0.join("export.json");
    fixture_must(fs::write(
        &response,
        fixture_must(serde_json::to_vec(&export)),
    ));
    fixture_must(fs::set_permissions(
        &response,
        fs::Permissions::from_mode(0o600),
    ));
    let corpus = document!({"cases":[{"response":response,"rpc_exchanges":node.state.lock().unwrap().exchanges,
        "budget_id":hex::encode(&fixture.id),"owner_account":hex::encode(&fixture.owner),"owner_did":fixture.did,
        "asset_id":hex::encode(&fixture.asset),"committed_height":export["finality"]["height"],
        "state_height":export["finality"]["state_height"],"block_hash":export["finality"]["block_hash"],"app_hash":export["finality"]["app_hash"]}]});
    let path = directory.0.join("corpus.json");
    fixture_must(fs::write(&path, fixture_must(serde_json::to_vec(&corpus))));
    fixture_must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o600),
    ));
    ProducedCorpus { directory, path }
}

fn protected_export(path: &Path, maximum: u64) -> Vec<u8> {
    let metadata = fixture_must(fs::symlink_metadata(path));
    assert!(metadata.is_file() && metadata.len() > 0 && metadata.len() <= maximum);
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    fixture_must(fs::read(path))
}

fn export_bytes(value: &Value) -> Result<Vec<u8>, ()> {
    let text = value.as_str().ok_or(())?;
    let decoded = hex::decode(text).map_err(|_| ())?;
    if hex::encode(&decoded) != text {
        return Err(());
    }
    Ok(decoded)
}

fn verify_native_export_proof(
    proof: &Value,
    key: &[u8],
    state_height: u64,
    root: [u8; 32],
) -> Result<Vec<u8>, ()> {
    if proof["code"] != 0
        || proof["height"] != state_height.to_string()
        || committed::base64(proof["key"].as_str().ok_or(())?)? != key
    {
        return Err(());
    }
    let value = committed::base64(proof["value"].as_str().ok_or(())?)?;
    let ops = proof["proofOps"]["ops"].as_array().ok_or(())?;
    if ops.len() != 2
        || ops[0]["type"] != "ics23:iavl"
        || ops[1]["type"] != "ics23:simple"
        || committed::base64(ops[0]["key"].as_str().ok_or(())?)? != key
        || committed::base64(ops[1]["key"].as_str().ok_or(())?)? != b"lxphost"
    {
        return Err(());
    }
    committed::verify_membership(
        key,
        &value,
        &committed::base64(ops[0]["data"].as_str().ok_or(())?)?,
        &committed::base64(ops[1]["data"].as_str().ok_or(())?)?,
        root,
    )?;
    Ok(value)
}

fn verify_committed_export(document: &Value, expected_finality: &Value) -> Result<(), ()> {
    use committed_budget_record::{number, Record};
    if document["schema"] != "layerx.human.budget-proof.v2"
        || document["protocol_version"] != 3
        || document.get("lni_interface_version").is_some()
        || document.get("authorization").is_some()
        || document.get("maximum_bytes").is_some()
        || &document["finality"] != expected_finality
    {
        return Err(());
    }
    let state = &document["budget_state"];
    let proofs = &document["proofs"];
    if proofs.as_object().ok_or(())?.len() != 3 {
        return Err(());
    }
    let budget = export_bytes(&proofs["budget_call"]["result"])?;
    let remaining = export_bytes(&proofs["remaining_call"]["result"])?;
    let record = Record::from_call(
        export_bytes(&state["budget_id"])?
            .try_into()
            .map_err(|_| ())?,
        &budget,
    )?;
    if export_bytes(&state["canonical_core_bytes"])? != budget
        || export_bytes(&state["asset"])? != record.asset
        || state["revocation_sequence"] != record.revocation
        || state["closed"] != record.closed
        || state["revoked"] != record.revoked
        || remaining.len() != 32
        || state["remaining"] != number(&remaining, 0, 16)?.to_string()
    {
        return Err(());
    }
    let head = &document["head"];
    let finality = &document["finality"];
    if head.as_object().ok_or(())?.len() != 2 || finality.as_object().ok_or(())?.len() != 4 {
        return Err(());
    }
    let committed = head["committed_height"].as_u64().ok_or(())?;
    let block_hash: [u8; 32] = export_bytes(&head["block_hash"])?
        .try_into()
        .map_err(|_| ())?;
    let state_height = finality["state_height"].as_u64().ok_or(())?;
    let app_hash: [u8; 32] = export_bytes(&finality["app_hash"])?
        .try_into()
        .map_err(|_| ())?;
    if committed == 0
        || finality["height"] != committed
        || block_hash == [0; 32]
        || app_hash == [0; 32]
        || export_bytes(&finality["block_hash"])? != block_hash
        || export_bytes(&state["checkpoint_digest"])? != block_hash
        || state_height.checked_add(1) != Some(committed)
        || record.revocation == 0
        || record.revocation > state["observed_head_sequence"].as_u64().ok_or(())?
    {
        return Err(());
    }
    for (name, signature) in [
        ("budget_call", b"getBudget(bytes32)".as_slice()),
        ("remaining_call", b"budgetRemaining(bytes32)".as_slice()),
    ] {
        let call = &proofs[name];
        let mut data = layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec();
        data.extend_from_slice(&record.id);
        if call.as_object().ok_or(())?.len() != 4
            || call["height"] != state_height
            || export_bytes(&call["to"])?
                != hex::decode("000000000000000000000000000000000000101b").map_err(|_| ())?
            || export_bytes(&call["data"])? != data
        {
            return Err(());
        }
    }
    for (name, id) in [
        ("owner", record.owner),
        ("budget", record.account),
        ("source", record.source.unwrap_or(record.owner)),
    ] {
        if export_bytes(&document["accounts"][name])? != id {
            return Err(());
        }
    }
    let join = &proofs["account_join"];
    if join.as_object().ok_or(())?.len() != 6 || &join["finality"] != finality {
        return Err(());
    }
    let row = &join["account"];
    if row["bound"] != true
        || row["frozen"] != false
        || export_bytes(&row["layerx_account"])? != record.owner
        || export_bytes(&row["asset_id"])? != record.asset
    {
        return Err(());
    }
    let owner_key = [vec![6], record.owner.to_vec()].concat();
    let owner_value = verify_native_export_proof(
        &join["native_account_proof"],
        &owner_key,
        state_height,
        app_hash,
    )?;
    let sequence = verify_native_export_proof(
        &join["receipt_sequence_proof"],
        &[2],
        state_height,
        app_hash,
    )?;
    let sequence = u64::from_be_bytes(sequence.try_into().map_err(|_| ())?);
    if sequence == 0 || state["observed_head_sequence"] != sequence {
        return Err(());
    }
    let binding = &join["binding_call"];
    let data = export_bytes(&binding["data"])?;
    let result = export_bytes(&binding["result"])?;
    if binding.as_object().ok_or(())?.len() != 4
        || binding["height"] != state_height
        || export_bytes(&binding["to"])?
            != hex::decode("0000000000000000000000000000000000001004").map_err(|_| ())?
        || data.len() != 36
        || data[..4]
            != layerx_crypto::evm_transaction::transaction_hash(b"getEvmAddrByLayerX(bytes32)")[..4]
        || result.len() != 32
        || result[..12] != [0; 12]
        || result[12..] == [0; 20]
        || row["evm_address"] != format!("0x{}", hex::encode(&result[12..]))
    {
        return Err(());
    }
    let did = format!("did:layerx:{}", hex::encode(&data[4..]));
    let identity = layerx_wire::hash::did_id_for_protocol(
        &layerx_types::ids::Did::new(did.as_bytes()).map_err(|_| ())?,
        3,
    )
    .map_err(|_| ())?;
    if export_bytes(&document["identity_key"])? != identity {
        return Err(());
    }
    let balances = join["balances"].as_array().ok_or(())?;
    let mut ids = vec![record.owner, record.account];
    if let Some(source) = record.source {
        ids.push(source);
    }
    if balances.len() != ids.len() {
        return Err(());
    }
    for (balance, id) in balances.iter().zip(ids) {
        if export_bytes(&balance["account_id"])? != id
            || export_bytes(&balance["asset_id"])? != record.asset
        {
            return Err(());
        }
        let value = verify_native_export_proof(
            &balance["proof"],
            &[vec![6], id.to_vec()].concat(),
            state_height,
            app_hash,
        )?;
        if id == record.owner && value != owner_value {
            return Err(());
        }
        let mut reader = layerx_wire::decode::Decoder::new(&value, 2048);
        if reader.bytes(32).map_err(|_| ())? != id {
            return Err(());
        }
        let name = std::str::from_utf8(reader.bytes(512).map_err(|_| ())?).map_err(|_| ())?;
        if id == record.owner && name != format!("agent:{did}:main") {
            return Err(());
        }
        let kind = reader.u8().map_err(|_| ())?;
        if kind == 0 {
            return Err(());
        }
        if balance["amount"] != reader.u128().map_err(|_| ())?.to_string()
            || reader.bytes(32).map_err(|_| ())? != record.asset
        {
            return Err(());
        }
        reader.u8().map_err(|_| ())?;
        reader.u64().map_err(|_| ())?;
        reader.u64().map_err(|_| ())?;
        reader.u8().map_err(|_| ())?;
        if reader.u8().map_err(|_| ())? != 0 {
            return Err(());
        }
        if reader.bytes(32).map_err(|_| ())?.len() != 32 {
            return Err(());
        }
        reader.u8().map_err(|_| ())?;
        reader.finish().map_err(|_| ())?;
    }
    let mut evidence = Sha256::new();
    evidence.update(b"layerx-human/rpc-budget-evidence/v2\0");
    evidence.update(block_hash);
    let join = serde_json::to_vec(&proofs["account_join"]).map_err(|_| ())?;
    for material in [&budget, &remaining, &join] {
        evidence.update(u64::try_from(material.len()).map_err(|_| ())?.to_be_bytes());
        evidence.update(material);
    }
    let digest: [u8; 32] = evidence.finalize().into();
    if export_bytes(&state["evidence_digest"])? != digest
        || export_bytes(&state["receipt_digest"])? != Sha256::digest(&budget).as_slice()
    {
        return Err(());
    }
    Ok(())
}

pub(crate) fn assert_export_corpus(fixture: &Value) {
    let cases = fixture["cases"].as_array().expect("real export cases");
    assert!(!cases.is_empty());
    for case in cases {
        let document: Value = serde_json::from_slice(&protected_export(
            Path::new(case["response"].as_str().expect("response path")),
            1_048_576,
        ))
        .expect("real export JSON");
        let finality = serde_json::json!({
            "height":case["committed_height"], "block_hash":case["block_hash"], "app_hash":case["app_hash"], "state_height":case["state_height"]
        });
        assert!(verify_committed_export(&document, &finality).is_ok());
        assert_eq!(document["budget_state"]["budget_id"], case["budget_id"]);
        assert_eq!(document["budget_state"]["asset"], case["asset_id"]);
        assert_eq!(document["accounts"]["owner"], case["owner_account"]);
        assert_eq!(document["finality"]["block_hash"], case["block_hash"]);
        assert_eq!(document["finality"]["app_hash"], case["app_hash"]);
        for name in ["budget_call", "remaining_call"] {
            let mut corrupt = document.clone();
            let mut raw = export_bytes(&corrupt["proofs"][name]["result"]).expect("call bytes");
            *raw.last_mut().expect("nonempty call") ^= 1;
            corrupt["proofs"][name]["result"] = serde_json::json!(hex::encode(&raw));
            assert!(
                verify_committed_export(&corrupt, &finality).is_err(),
                "tampered {name}"
            );
        }
        for name in ["budget_call", "remaining_call"] {
            for field in ["to", "data", "height"] {
                let mut corrupt = document.clone();
                corrupt["proofs"][name][field] = serde_json::json!(0);
                assert!(
                    verify_committed_export(&corrupt, &finality).is_err(),
                    "tampered {name}.{field}"
                );
            }
        }
        let mut corrupt = document.clone();
        corrupt["finality"]["block_hash"] = serde_json::json!(hex::encode(&[0xaa; 32]));
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["finality"]["height"] = serde_json::json!(0);
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["finality"]["app_hash"] = serde_json::json!(hex::encode(&[0xaa; 32]));
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["proofs"]["account_join"]["layerx"]["accounts"] = serde_json::json!([]);
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        let mut corrupt = document.clone();
        corrupt["head"]["committed_height"] = serde_json::json!(0);
        assert!(verify_committed_export(&corrupt, &finality).is_err());
        for field in ["key", "value", "height", "proofOps"] {
            let mut corrupt = document.clone();
            corrupt["proofs"]["account_join"]["native_account_proof"][field] = document!(null);
            // Recompute the envelope digest: the authenticated membership check
            // must reject this independently of the export checksum.
            let mut digest = Sha256::new();
            digest.update(b"layerx-human/rpc-budget-evidence/v2\0");
            digest.update(export_bytes(&corrupt["finality"]["block_hash"]).unwrap());
            for material in [
                export_bytes(&corrupt["proofs"]["budget_call"]["result"]).unwrap(),
                export_bytes(&corrupt["proofs"]["remaining_call"]["result"]).unwrap(),
                serde_json::to_vec(&corrupt["proofs"]["account_join"]).unwrap(),
            ] {
                digest.update((material.len() as u64).to_be_bytes());
                digest.update(material);
            }
            corrupt["budget_state"]["evidence_digest"] = document!(hex::encode(&digest.finalize()));
            assert!(
                verify_committed_export(&corrupt, &finality).is_err(),
                "accepted native {field}"
            );
        }
    }
}

#[test]
pub(crate) fn budget_export_corpus_optional() {
    let corpus = produced_export_corpus();
    let fixture: Value = fixture_must(serde_json::from_slice(&protected_export(
        &corpus.path,
        65_536,
    )));
    assert!(corpus.directory.0.join("export.json").is_file());
    assert_export_corpus(&fixture);
}

#[test]
pub(crate) fn human_tls_retired_anchor() {
    for capability in [false, true] {
        let fixture = Arc::new(identity_fixture(capability));
        let node = Node::start(fixture.clone());
        let directory = Directory::new();
        let config = config_fixture(&node, &fixture, &directory.0);
        retain_records(&config, &fixture.records);
        assert_eq!(
            fixture_must(fs::read_dir(directory.0.join("state"))).count(),
            1
        );
        drop(config);
        // Reopen the real authority evidence store before driving any route.
        let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
        let policy = fixture.policy();
        let identity = &policy["identities"][0];
        let base = format!("{}&did={}", human_path("identity"), fixture.did);
        let mut paths = vec![base];
        if capability {
            let scope = &identity["capabilities"][0];
            paths.push(format!(
                "{}&did={}&authority={}&action_key={}&capability_id={}",
                human_path("capability-scope"),
                fixture.did,
                scope["authority"].as_str().unwrap(),
                scope["action_key"].as_str().unwrap(),
                scope["capability_id"].as_str().unwrap()
            ));
        } else {
            paths.push(format!(
                "{}&did={}&recovery=false",
                human_path("key-policy"),
                fixture.did
            ));
        }
        for path in &paths {
            assert_eq!(tls_request(config.clone(), path, "wrong-token").0, 401);
            let (status, answer) = tls_request(config.clone(), path, "repair-human-token-208");
            assert_eq!(status, 200, "{path}: {answer}");
            assert_eq!(answer["finality"]["height"], 9);
            assert_eq!(answer["finality"]["state_height"], 8);
            assert_eq!(answer["finality"]["block_hash"], hex::encode(&[9; 32]));
            assert_eq!(
                answer["finality"]["app_hash"],
                hex::encode(&fixture.store.app_hash)
            );
            if path.contains("/identity?") {
                assert_eq!(answer["head_sequence"], fixture.sequence);
                assert_eq!(
                    answer["authorities"].as_array().unwrap().len(),
                    if capability { 2 } else { 1 }
                );
            } else {
                assert_eq!(answer["checkpoint_digest"], hex::encode(&[9; 32]));
                assert_eq!(answer["verification"], 4);
                if capability {
                    assert_eq!(answer["activity_types"], document!([3]));
                    assert_eq!(
                        answer["capability_id"],
                        identity["capabilities"][0]["capability_id"]
                    );
                    assert_eq!(answer["observed_sequence"], fixture.sequence);
                } else {
                    assert_eq!(
                        answer["policy_revision"],
                        identity["rotation"]["policy_revision"]
                    );
                    assert_eq!(answer["observed_head_sequence"], fixture.sequence);
                }
            }
        }
        for fault in [
            "apphash",
            "proof",
            "value",
            "key",
            "height",
            "store",
            "landing",
            "uncommitted",
            "receipt-height",
            "key-id",
            "signing-key",
        ] {
            node.fault(fault);
            for path in &paths {
                let (status, answer) = tls_request(config.clone(), path, "repair-human-token-208");
                assert_eq!(status, 503, "accepted {fault}: {answer}");
                assert_eq!(
                    answer["error"]["code"],
                    match fault {
                        "apphash" => "committed_header_unavailable",
                        "proof" | "value" | "key" | "height" | "store" =>
                            "identity_state_proof_unavailable",
                        _ => "committed_receipt_evidence_unavailable",
                    }
                );
            }
        }
        assert_no_retired_calls(&node);
    }
}

#[test]
pub(crate) fn human_budget_corpus_optional() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let state_path = budget_path("budget-state", &fixture);
    let export_path = budget_path("budget-proof", &fixture);
    let (status, state) = tls_request(config.clone(), &state_path, "repair-human-token-208");
    assert_eq!(status, 200, "{state}");
    let (status, export) = tls_request(config.clone(), &export_path, "repair-human-token-208");
    assert_eq!(status, 200, "{export}");
    assert_eq!(export["budget_state"], state);
    assert_eq!(state["remaining"], "77");
    assert_eq!(state["observed_head_sequence"], 31);
    assert_eq!(export["schema"], "layerx.human.budget-proof.v2");
    assert_eq!(export["accounts"]["owner"], hex::encode(&fixture.owner));
    assert_eq!(export["accounts"]["budget"], hex::encode(&fixture.budget));
    assert!(verify_committed_export(&export, &export["finality"]).is_ok());
    assert!(export.get("lni_interface_version").is_none());
    assert!(!directory.0.join("absent.sock").exists());
    // A separate replay requires the corpus just produced through TLS. No
    // environment-variable branch can turn absence into a successful test.
    let replay_corpus = produced_export_corpus();
    let corpus: Value = fixture_must(serde_json::from_slice(&protected_export(
        &replay_corpus.path,
        65_536,
    )));
    let cases = corpus["cases"].as_array().expect("required replay cases");
    assert!(!cases.is_empty());
    for case in cases {
        let exchanges = case["rpc_exchanges"]
            .as_array()
            .expect("required captured RPC exchanges")
            .clone();
        assert!(!exchanges.is_empty());
        let replay = Node::with_transcript(fixture.clone(), Some(exchanges));
        let expected: Value = fixture_must(serde_json::from_slice(&protected_export(
            Path::new(case["response"].as_str().unwrap()),
            1_048_576,
        )));
        let replay_directory = Directory::new();
        let replay_config = Arc::new(config_fixture(&replay, &fixture, &replay_directory.0));
        let (status, state) =
            tls_request(replay_config.clone(), &state_path, "repair-human-token-208");
        assert_eq!(status, 200, "{state}");
        let (status, export) = tls_request(replay_config, &export_path, "repair-human-token-208");
        assert_eq!(status, 200, "{export}");
        assert_eq!(state, expected["budget_state"]);
        assert_eq!(export["budget_state"], state);
        assert_eq!(export["schema"], "layerx.human.budget-proof.v2");
        for field in ["head", "finality", "proofs", "accounts", "identity_key"] {
            assert_eq!(export[field], expected[field], "{field}");
        }
        assert!(export.get("lni_interface_version").is_none());
        assert!(!replay_directory.0.join("absent.sock").exists());
        assert_no_retired_calls(&replay);
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
        for path in [&state_path, &export_path] {
            let (status, answer) = tls_request(config.clone(), path, "repair-human-token-208");
            assert_eq!(status, 503, "accepted {fault}: {answer}");
        }
    }
    assert_no_retired_calls(&node);
}

#[test]
fn retirement_guard_requires_anchor() {
    let fixture = Arc::new(Fixture::new(false));
    let node = Node::start(fixture.clone());
    let directory = Directory::new();
    let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
    let (status, export) = tls_request(
        config,
        &budget_path("budget-proof", &fixture),
        "repair-human-token-208",
    );
    assert_eq!(status, 200, "{export}");
    assert!(verify_committed_export(&export, &export["finality"]).is_ok());
    assert_no_retired_calls(&node);
}
