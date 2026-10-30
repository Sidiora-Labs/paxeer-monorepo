use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::ops::program::{
    ProgramAccountBalance, ProgramOperationError, ProgramOperations,
};
use layerx_agentd::read::{LayerxdProgramBalanceReader, ProtocolProgramBalanceReader};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_programs::{
    chain_evidence::programs_state_key,
    chain_proof::{decode::ProofOp, state::verify_state, VerifiedChainHead},
    program_account_registration_commitment, CanonicalAccountLeaf, ProgramId, ProgramLifecycle,
    ProgramValueAccountBinding, VerifiedAccountSnapshot,
};

#[allow(dead_code)]
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("program balance acceptance: {error:?}"))
}

struct Fixture {
    height: u64,
    block_hash: String,
    timestamp: u64,
    program: String,
    lifecycle: String,
    accounts: Vec<Account>,
}

struct Account {
    account: String,
    asset: String,
    amount: String,
    frozen: bool,
}

struct CommittedPrograms {
    fixture: Fixture,
    block: Value,
    bindings: Vec<ProgramValueAccountBinding>,
    accounts: Vec<CommittedAccount>,
}

struct CommittedAccount {
    leaf: CanonicalAccountLeaf,
    proof: Vec<ProofOp>,
    head: VerifiedChainHead,
}

impl CommittedAccount {
    fn account(&self) -> CanonicalAccountLeaf {
        must(VerifiedAccountSnapshot::verify_chain(
            &self.head,
            &self.leaf,
            &self.proof,
        ))
        .account()
        .clone()
    }
}

impl CommittedPrograms {
    fn new() -> Self {
        let program = must(ProgramId::new([0x71; 32]));
        let mut bindings = Vec::new();
        let mut leaves = Vec::new();
        let mut pairs = Vec::new();
        for (seed, amount, frozen, asset) in [
            (b"vault\0reserve".to_vec(), u128::MAX, true, [0xaa; 32]),
            (vec![0x37; 128], (1_u128 << 96) + 42, false, [0xbb; 32]),
            (Vec::new(), 0, false, [0xcc; 32]),
        ] {
            let account = must(layerx_programs_runtime::derive_program_account(
                program, &seed,
            ));
            let mut binding = ProgramValueAccountBinding {
                record_version: 2,
                program,
                seed,
                account_id: account.bytes(),
                asset_id: asset,
                registered_sequence: 7,
                registration_event_digest: [0; 32],
            };
            binding.registration_event_digest = program_account_registration_commitment(&binding);
            pairs.push((
                programs_state_key(&binding.primary_key()),
                must(binding.primary_value()),
            ));
            let leaf = CanonicalAccountLeaf {
                account_id: account.bytes(),
                name: format!("module:programs:value:{}", hex::encode(account.bytes()))
                    .into_bytes(),
                kind: 13,
                balance: amount,
                asset_id: asset,
                has_asset: true.into(),
                next_sequence: 9,
                created_at_sequence: 7,
                frozen: frozen.into(),
                has_open_reference: false.into(),
                authority_key: [0; 32],
                has_authority_key: false.into(),
            };
            pairs.push((
                programs_state_key(&leaf.primary_key()),
                must(leaf.primary_value()),
            ));
            bindings.push(binding);
            leaves.push(leaf);
        }
        let chain = must(chain::ChainFixture::new(
            "program-balance-read",
            &[[1; 32], [2; 32], [3; 32], [4; 32]],
            pairs,
        ));
        let head = must(chain.verifier.verify_head(&chain.header, &chain.commit));
        let mut accounts = Vec::new();
        for (binding, leaf) in bindings.iter().zip(leaves) {
            let key = programs_state_key(&binding.primary_key());
            assert_eq!(
                must(verify_state(
                    &head,
                    chain::STORE_NAME,
                    &key,
                    &must(binding.primary_value()),
                    &must(chain.prove(&key))
                )),
                chain::STATE_HEIGHT,
            );
            let key = programs_state_key(&leaf.primary_key());
            let verified = must(VerifiedAccountSnapshot::verify_chain(
                &head,
                &leaf,
                &must(chain.prove(&key)),
            ));
            assert_eq!(verified.height(), chain::STATE_HEIGHT);
            let mut changed = leaf.clone();
            changed.balance ^= 1;
            assert!(VerifiedAccountSnapshot::verify_chain(
                &head,
                &changed,
                &must(chain.prove(&key))
            )
            .is_err());
            accounts.push(CommittedAccount {
                leaf: verified.account().clone(),
                proof: must(chain.prove(&key)),
                head: head.clone(),
            });
        }
        let fixture = Fixture {
            height: chain::STATE_HEIGHT.unsigned_abs(),
            // The signed successor authenticates the committed state's block ID.
            block_hash: hex::encode(&chain.header.last_block_id.hash),
            timestamp: 1_699_999_999,
            program: hex::encode(program.bytes()),
            lifecycle: "active".to_owned(),
            accounts: accounts
                .iter()
                .map(|snapshot| {
                    let leaf = snapshot.account();
                    Account {
                        account: hex::encode(leaf.account_id),
                        asset: hex::encode(leaf.asset_id),
                        amount: leaf.balance.to_string(),
                        frozen: bool::from(leaf.frozen),
                    }
                })
                .collect(),
        };
        let block = json!({
            "number": format!("{:#x}", fixture.height),
            "hash": format!("0x{}", fixture.block_hash),
            "parentHash": format!("0x{}", "02".repeat(32)),
            "timestamp": format!("{:#x}", fixture.timestamp),
            "transactionsRoot": format!("0x{}", "03".repeat(32)),
            "receiptsRoot": format!("0x{}", "04".repeat(32)),
            "transactions": [],
        });
        Self {
            fixture,
            block,
            bindings,
            accounts,
        }
    }

    fn answer(&self, request: &Value) -> Value {
        let height = format!("{:#x}", self.fixture.height);
        let result = match request["method"].as_str() {
            Some("eth_getBlockByNumber") => {
                assert_eq!(request["params"][1], true);
                if request["params"][0] == height {
                    self.block.clone()
                } else {
                    Value::Null
                }
            }
            Some("eth_call") => {
                assert_eq!(request["params"][1], height);
                assert_eq!(
                    request["params"][0]["to"],
                    "0x000000000000000000000000000000000000101d"
                );
                let input = must(hex::decode(
                    request["params"][0]["data"]
                        .as_str()
                        .unwrap()
                        .strip_prefix("0x")
                        .unwrap(),
                ));
                let selector = |signature: &[u8]| {
                    layerx_crypto::evm_transaction::transaction_hash(signature)[..4].to_vec()
                };
                let output = if input[..4] == selector(b"getWindDown(bytes32)") {
                    assert_eq!(input.len(), 36);
                    assert_eq!(input[4..], id(&self.fixture.program));
                    let mut output = word(1).to_vec();
                    output.extend_from_slice(&[0; 32]);
                    output.extend_from_slice(&word(0));
                    output.extend_from_slice(&word(7));
                    output.extend_from_slice(&word(self.accounts.len() as u128));
                    output.extend_from_slice(&word(
                        self.accounts
                            .iter()
                            .filter(|s| s.account().balance != 0)
                            .count() as u128,
                    ));
                    output
                } else if input[..4] == selector(b"getProgramAccountById(bytes32)") {
                    assert_eq!(input.len(), 36);
                    let binding = self
                        .bindings
                        .iter()
                        .find(|binding| binding.account_id == input[4..])
                        .unwrap();
                    let mut output = binding.program.bytes().to_vec();
                    output.extend_from_slice(&binding.asset_id);
                    output.extend_from_slice(&word(96));
                    output.extend_from_slice(&word(binding.seed.len() as u128));
                    output.extend_from_slice(&binding.seed);
                    output.resize(128 + binding.seed.len().div_ceil(32) * 32, 0);
                    output
                } else {
                    assert_eq!(input[..4], selector(b"getValueAccount(bytes32,bytes32)"));
                    assert_eq!(input.len(), 68);
                    assert_eq!(input[4..36], id(&self.fixture.program));
                    let snapshot = self
                        .accounts
                        .iter()
                        .find(|snapshot| snapshot.account().account_id == input[36..])
                        .unwrap();
                    let mut output = word(snapshot.account().balance).to_vec();
                    output.extend_from_slice(&word(u128::from(bool::from(
                        snapshot.account().frozen,
                    ))));
                    output
                };
                json!(format!("0x{}", hex::encode(output)))
            }
            other => panic!("unexpected Programs RPC method: {other:?}"),
        };
        json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
    }
}

fn word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

fn id(text: &str) -> [u8; 32] {
    must(must(hex::decode(text.strip_prefix("0x").unwrap_or(text))).try_into())
}

/// Serves the committed, proof-verified account fixture over the real RPC transport.
struct Relay {
    url: String,
    requests: Arc<Mutex<Vec<Value>>>,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Relay {
    fn start(state: CommittedPrograms) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let url = format!("http://{}/", must(listener.local_addr()));
        must(listener.set_nonblocking(true));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let captured = Arc::clone(&requests);
        let stopped = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stopped);
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("capture RPC connection: {error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(10))));
                must(stream.set_write_timeout(Some(Duration::from_secs(10))));
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                let length = length.unwrap_or_else(|| panic!("RPC content length required"));
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                must(captured.lock()).push(request.clone());
                let answer = state.answer(&request).to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                    answer.len()
                ));
                must(stream.flush());
            }
        });
        Self {
            url,
            requests,
            stopped,
            worker: Some(worker),
        }
    }

    fn endpoint(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
    }
}

impl Drop for Relay {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            if worker.join().is_err() && !thread::panicking() {
                panic!("RPC capture relay failed");
            }
        }
    }
}

#[test]
fn program_balance_is_read_by_eth_call_at_a_committed_height() {
    let state = CommittedPrograms::new();
    assert!(state.fixture.height > 0);
    assert!(!state.fixture.accounts.is_empty());
    assert!(state
        .fixture
        .accounts
        .iter()
        .any(|account| must(account.amount.parse::<u128>()) > 0));
    let fixture = Fixture {
        height: state.fixture.height,
        block_hash: state.fixture.block_hash.clone(),
        timestamp: state.fixture.timestamp,
        program: state.fixture.program.clone(),
        lifecycle: state.fixture.lifecycle.clone(),
        accounts: state
            .fixture
            .accounts
            .iter()
            .map(|account| Account {
                account: account.account.clone(),
                asset: account.asset.clone(),
                amount: account.amount.clone(),
                frozen: account.frozen,
            })
            .collect(),
    };
    let relay = Relay::start(state);
    let program = must(ProgramId::new(id(&fixture.program)));
    let accounts: Vec<_> = fixture
        .accounts
        .iter()
        .map(|account| id(&account.account))
        .collect();
    let reader = ProtocolProgramBalanceReader::bind(relay.endpoint());
    let read = must(reader.read(program, &accounts, fixture.height));
    assert_eq!(read.program, program.bytes());
    assert_eq!(read.freshness.height, fixture.height);
    assert_eq!(read.freshness.block_hash, id(&fixture.block_hash));
    assert_eq!(read.freshness.timestamp, fixture.timestamp);
    let lifecycle = match fixture.lifecycle.as_str() {
        "active" => ProgramLifecycle::Active,
        "deprecated" => ProgramLifecycle::Deprecated,
        "tombstoned" => ProgramLifecycle::Tombstoned,
        other => panic!("unknown fixture lifecycle: {other}"),
    };
    assert_eq!(read.lifecycle, lifecycle);
    assert_eq!(read.accounts.len(), fixture.accounts.len());
    for (actual, expected) in read.accounts.iter().zip(&fixture.accounts) {
        assert_eq!(actual.account, id(&expected.account));
        assert_eq!(actual.asset, id(&expected.asset));
        assert_eq!(actual.amount, must(expected.amount.parse::<u128>()));
        assert_eq!(actual.frozen, expected.frozen);
    }
    let requests = must(relay.requests.lock()).clone();
    assert_eq!(requests.len(), 3 + 2 * accounts.len());
    for request in [&requests[0], &requests[requests.len() - 1]] {
        assert_eq!(request["method"], "eth_getBlockByNumber");
        assert_eq!(
            request["params"],
            json!([format!("{:#x}", fixture.height), true])
        );
    }
    for request in &requests[1..requests.len() - 1] {
        assert_eq!(request["method"], "eth_call");
        assert_eq!(
            request["params"][0]["to"],
            "0x000000000000000000000000000000000000101d"
        );
        assert_eq!(request["params"][1], format!("{:#x}", fixture.height));
    }
    let wind_down = layerx_crypto::evm_transaction::transaction_hash(b"getWindDown(bytes32)");
    assert_eq!(
        requests[1]["params"][0]["data"],
        format!(
            "0x{}",
            hex::encode([wind_down[..4].to_vec(), program.bytes().to_vec()].concat())
        )
    );
    let identity =
        layerx_crypto::evm_transaction::transaction_hash(b"getProgramAccountById(bytes32)");
    let selector =
        layerx_crypto::evm_transaction::transaction_hash(b"getValueAccount(bytes32,bytes32)");
    for (index, account) in accounts.iter().enumerate() {
        assert_eq!(
            requests[2 + 2 * index]["params"][0]["data"],
            format!(
                "0x{}",
                hex::encode([identity[..4].to_vec(), account.to_vec()].concat())
            )
        );
        let expected = [
            selector[..4].to_vec(),
            program.bytes().to_vec(),
            account.to_vec(),
        ]
        .concat();
        assert_eq!(
            requests[3 + 2 * index]["params"][0]["data"],
            format!("0x{}", hex::encode(expected))
        );
    }
    let direct = relay.endpoint();
    let same = must(LayerxdProgramBalanceReader::read_at(
        &direct,
        program,
        &accounts,
        fixture.height,
    ));
    assert_eq!(same, read);
    let request_count = must(relay.requests.lock()).len();
    assert!(reader.read(program, &accounts, 0).is_err());
    let mut duplicates = accounts.clone();
    duplicates.push(accounts[0]);
    assert!(reader.read(program, &duplicates, fixture.height).is_err());
    assert!(reader.read(program, &[[0; 32]], fixture.height).is_err());
    let excessive = vec![accounts[0]; layerx_programs::MAX_PROGRAM_VALUE_ACCOUNTS + 1];
    assert!(reader.read(program, &excessive, fixture.height).is_err());
    assert_eq!(must(relay.requests.lock()).len(), request_count);
    assert!(ProgramId::new([0; 32]).is_err());
    assert!(reader.read(program, &accounts, fixture.height + 1).is_err());
    let requests = must(relay.requests.lock());
    assert_eq!(requests.len(), request_count + 1);
    assert_eq!(requests.last().unwrap()["method"], "eth_getBlockByNumber");
    assert_eq!(
        requests.last().unwrap()["params"],
        json!([format!("{:#x}", fixture.height + 1), true])
    );
}

fn serve(replies: Vec<Value>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        let mut requests = Vec::new();
        for mut reply in replies {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "expected RPC request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut content_length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    content_length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = content_length.unwrap_or_else(|| panic!("missing content length"));
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            reply["jsonrpc"] = json!("2.0");
            reply["id"] = request["id"].clone();
            let answer = reply.to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
            requests.push(request);
        }
        requests
    });
    (url, server)
}

fn encoded_balance(amount: u128, exists: bool) -> Vec<u8> {
    let mut result = vec![0; 64];
    result[16..32].copy_from_slice(&amount.to_be_bytes());
    result[63] = u8::from(exists);
    result
}

fn config(url: String) -> NodeRpcConfig {
    NodeRpcConfig {
        url,
        chain_id: 125,
        deadline: Duration::from_secs(2),
    }
}

fn assert_call(request: &Value, program: [u8; 32], account: [u8; 32], height: u64) {
    assert_eq!(request["method"], "eth_call");
    let selector =
        layerx_crypto::evm_transaction::transaction_hash(b"getValueAccount(bytes32,bytes32)");
    let mut calldata = selector[..4].to_vec();
    calldata.extend_from_slice(&program);
    calldata.extend_from_slice(&account);
    assert_eq!(
        request["params"],
        json!([
            {"to": "0x000000000000000000000000000000000000101d", "data": format!("0x{}", hex::encode(calldata))},
            format!("{height:#x}")
        ])
    );
}

#[test]
fn agentd_reads_program_balances_through_eth_call() {
    let program_bytes = [0x31; 32];
    let program = must(ProgramId::new(program_bytes));
    let accounts = [[0x41; 32], [0x42; 32], [0x43; 32], [0x44; 32]];
    let height = 0x1234;
    let expected = [(347_u128, true), (0, true), (0, false), (u128::MAX, true)];
    let mut replies = vec![json!({"result": "0x7d"}), json!({"result": "0x1234"})];
    replies.extend(expected.iter().map(|(amount, exists)| {
        json!({"result": format!("0x{}", hex::encode(encoded_balance(*amount, *exists)))})
    }));
    let (url, server) = serve(replies);
    let (observed_height, balances) = must(ProgramOperations::current_balances(
        &config(url),
        program,
        &accounts,
    ));
    assert_eq!(observed_height, height);
    let expected_balances: Vec<_> = accounts
        .iter()
        .zip(expected)
        .map(|(account, (balance, exists))| ProgramAccountBalance {
            account: *account,
            balance,
            exists,
        })
        .collect();
    assert_eq!(balances, expected_balances);
    let requests = must(server.join());
    assert_eq!(requests.len(), 6);
    assert_eq!(requests[0]["method"], "eth_chainId");
    assert_eq!(requests[1]["method"], "eth_blockNumber");
    assert_eq!(requests[1]["params"], json!([]));
    for (request, account) in requests[2..].iter().zip(accounts) {
        assert_call(request, program_bytes, account, height);
    }

    let mut overflow = encoded_balance(1, true);
    overflow[15] = 1;
    let mut invalid_bool = encoded_balance(1, true);
    invalid_bool[63] = 2;
    let mut bool_padding = encoded_balance(1, true);
    bool_padding[32] = 1;
    let mut trailing = encoded_balance(1, true);
    trailing.push(0);
    for invalid in [
        vec![],
        vec![0; 32],
        vec![0; 63],
        overflow,
        invalid_bool,
        bool_padding,
        trailing,
        encoded_balance(1, false),
    ] {
        let (url, server) = serve(vec![
            json!({"result": "0x7d"}),
            json!({"result": format!("0x{}", hex::encode(invalid))}),
        ]);
        let node = must(NodeRpc::connect(&config(url)));
        assert_eq!(
            ProgramOperations::value_accounts_at(&node, height, program, &accounts[..1]),
            Err(ProgramOperationError::UnverifiedReceipt)
        );
        let requests = must(server.join());
        assert_eq!(requests.len(), 2);
        assert_call(&requests[1], program_bytes, accounts[0], height);
    }

    let (url, server) = serve(vec![
        json!({"result": "0x7d"}),
        json!({"error": {"code": -32000, "message": "state unavailable"}}),
    ]);
    let node = must(NodeRpc::connect(&config(url)));
    assert_eq!(
        ProgramOperations::value_accounts_at(&node, height, program, &accounts[..1]),
        Err(ProgramOperationError::Unavailable)
    );
    let requests = must(server.join());
    assert_eq!(requests.len(), 2);
    assert_call(&requests[1], program_bytes, accounts[0], height);

    for dependency_file in [
        include_str!("../Cargo.toml"),
        include_str!("../../layerx-agentd-host/Cargo.toml"),
        include_str!("../../../Cargo.lock"),
    ] {
        assert!(!dependency_file.contains("layerx-programs-protocol-adapter"));
    }
}
