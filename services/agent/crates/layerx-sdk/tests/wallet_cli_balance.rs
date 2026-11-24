//! wallet-cli-balance: real CLI framing -> production account mapping and ledger
//! reader -> HTTP -> funded in-memory ledger fixture -> exact CLI balance.
//! The fixture is a peer of the repaired CLI, not a replacement for its reader.

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_client::rpc::account::{balance, ReadContext, Requested};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::rung::Rung;
use serde_json::{json, Value};

const NATIVE: [u8; 32] = [3; 32];
const TOKEN: [u8; 32] = [4; 32];
const NATIVE_FUNDS: u128 = 100_000_000_000_000_000_001;
const TOKEN_FUNDS: u128 = u128::MAX;

fn hex(bytes: &[u8]) -> String {
    hex::encode(bytes)
}

fn did() -> String {
    derive_from_seed(&[0x42; 64], 0)
        .unwrap_or_else(|error| panic!("provisioned fixture wallet: {error:?}"))
        .did()
}

// The node provisions explicit account names, independently of wallet_account's
// native-asset branch. Both use the actual protocol account-ID codec.
fn provisioned_account(asset: [u8; 32]) -> [u8; 32] {
    let suffix = if asset == NATIVE {
        "main".to_owned()
    } else {
        format!("asset:{}", hex(&asset))
    };
    let name = AccountId::parse(&format!("agent:{}:{suffix}", did()))
        .unwrap_or_else(|error| panic!("account name: {error:?}"));
    layerx_wire::hash::account_id_for_protocol(&name, 3)
        .unwrap_or_else(|error| panic!("node account ID: {error:?}"))
}

enum LedgerReply {
    Funded,
    Refusal,
    Malformed(Value),
}

struct Node {
    endpoint: String,
    stop: Arc<AtomicBool>,
    seen: Arc<Mutex<Vec<(String, Value)>>>,
    worker: Option<JoinHandle<()>>,
}

impl Node {
    fn start(reply: LedgerReply) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("loopback node: {error:?}"));
        let endpoint = format!(
            "http://{}/",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("node address: {error:?}"))
        );
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("nonblocking accept: {error:?}"));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let seen = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&seen);
        let ledger = funded_ledger();
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let socket = match listener.accept() {
                    Ok((socket, _)) => socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                let (mut socket, path, request) = read_request(socket);
                assert_eq!(request["jsonrpc"], "2.0");
                let result = match request["method"]
                    .as_str()
                    .unwrap_or_else(|| panic!("RPC method"))
                {
                    "eth_blockNumber" => {
                        assert_eq!(request["params"], json!([]));
                        json!({"result":"0x2a"})
                    }
                    "eth_call" => {
                        let params = &request["params"];
                        assert_eq!(params[1], "0x2a", "read the exact selected height");
                        assert_eq!(
                            params[0]["to"],
                            "0x000000000000000000000000000000000000101a"
                        );
                        let call = params[0]["data"]
                            .as_str()
                            .unwrap_or_else(|| panic!("calldata"));
                        assert_eq!(call.len(), 2 + 8 + 64 + 64);
                        assert_eq!(&call[..10], "0xb1ffc03e", "balanceOf(bytes32,bytes32)");
                        let funds = ledger
                            .get(&(call[10..74].to_owned(), call[74..].to_owned()))
                            .unwrap_or_else(|| {
                                panic!("requested account and asset must be provisioned together")
                            });
                        match &reply {
                            LedgerReply::Funded => {
                                let mut word = [0; 32];
                                word[16..].copy_from_slice(&funds.to_be_bytes());
                                json!({"result":format!("0x{}", hex(&word))})
                            }
                            LedgerReply::Refusal => json!({"error":{
                                "code":3, "message":"execution reverted",
                                "data":{"code":"LXP_ASSET_MISMATCH", "asset":hex(&TOKEN)}
                            }}),
                            LedgerReply::Malformed(value) => json!({"result":value}),
                        }
                    }
                    method => {
                        panic!("unexpected method {method}; ledger IDs must not use eth_getBalance")
                    }
                };
                let mut response = result
                    .as_object()
                    .unwrap_or_else(|| panic!("response object"))
                    .clone();
                response.insert("jsonrpc".into(), json!("2.0"));
                response.insert("id".into(), request["id"].clone());
                let body = serde_json::to_vec(&response)
                    .unwrap_or_else(|error| panic!("encode node response: {error:?}"));
                recorded
                    .lock()
                    .unwrap_or_else(|error| panic!("record lock: {error:?}"))
                    .push((path, request));
                write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())
                    .unwrap_or_else(|error| panic!("HTTP response: {error:?}"));
                socket
                    .write_all(&body)
                    .unwrap_or_else(|error| panic!("node result: {error:?}"));
            }
        });
        Self {
            endpoint,
            stop,
            seen,
            worker: Some(worker),
        }
    }

    fn finish(mut self) -> Vec<(String, Value)> {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .unwrap_or_else(|| panic!("node worker"))
            .join()
            .unwrap_or_else(|error| panic!("node requests: {error:?}"));
        self.seen
            .lock()
            .unwrap_or_else(|error| panic!("requests lock: {error:?}"))
            .clone()
    }
}

impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn read_request(socket: TcpStream) -> (TcpStream, String, Value) {
    socket
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("read timeout: {error:?}"));
    socket
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("write timeout: {error:?}"));
    let mut reader = BufReader::new(socket);
    let mut line = String::new();
    reader
        .read_line(&mut line)
        .unwrap_or_else(|error| panic!("request line: {error:?}"));
    let path = line.trim_end().to_owned();
    let mut length = None;
    let mut total = line.len();
    loop {
        line.clear();
        assert_ne!(
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("header: {error:?}")),
            0
        );
        total += line.len();
        assert!(total < 8192, "bounded request headers");
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(
                value
                    .trim()
                    .parse::<usize>()
                    .unwrap_or_else(|error| panic!("content length: {error:?}")),
            );
        }
    }
    let length = length.unwrap_or_else(|| panic!("request content length"));
    assert!(length < 8192);
    let mut body = vec![0; length];
    reader
        .read_exact(&mut body)
        .unwrap_or_else(|error| panic!("RPC body: {error:?}"));
    let request =
        serde_json::from_slice(&body).unwrap_or_else(|error| panic!("RPC JSON: {error:?}"));
    (reader.into_inner(), path, request)
}

fn request(endpoint: &str, asset: [u8; 32], route: &str) -> Value {
    json!({"action":"balance", "did":did(), "asset":hex(&asset),
    "configuration":{
        "endpoint":endpoint, "rpc_route":route,
        "protocol_version":"3", "network_id":"125",
        "native_asset":hex(&NATIVE), "sequencer_id":"01".repeat(32),
        "sequencer_key":"02".repeat(32), "first_batch":"1", "last_batch":"100"
    }})
}

fn cli(input: &Value) -> (bool, Value) {
    let mut child = Command::new(env!("CARGO_BIN_EXE_layerx-wallet-rpc"))
        .env_remove("LAYERX_RUNTIME_CLOCK_SOCKET")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .unwrap_or_else(|error| panic!("built production CLI: {error:?}"));
    child
        .stdin
        .take()
        .unwrap_or_else(|| panic!("CLI stdin"))
        .write_all(
            &serde_json::to_vec(input).unwrap_or_else(|error| panic!("CLI input: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("write input: {error:?}"));
    let deadline = Instant::now() + Duration::from_secs(10);
    while child
        .try_wait()
        .unwrap_or_else(|error| panic!("CLI status: {error:?}"))
        .is_none()
    {
        if Instant::now() >= deadline {
            child
                .kill()
                .unwrap_or_else(|error| panic!("kill timed-out CLI: {error:?}"));
            let output = child
                .wait_with_output()
                .unwrap_or_else(|error| panic!("reap CLI: {error:?}"));
            panic!("CLI timeout: {}", String::from_utf8_lossy(&output.stderr));
        }
        thread::sleep(Duration::from_millis(1));
    }
    let output = child
        .wait_with_output()
        .unwrap_or_else(|error| panic!("CLI output: {error:?}"));
    assert!(
        output.stderr.is_empty(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let value = serde_json::from_slice(&output.stdout)
        .unwrap_or_else(|error| panic!("one JSON result: {error:?}"));
    (output.status.success(), value)
}

#[test]
fn wallet_cli_balance_native_and_token_match_production_reader() {
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxledger/abi.json"
    ))
    .unwrap_or_else(|error| panic!("production ledger ABI: {error:?}"));
    let method = abi
        .as_array()
        .unwrap_or_else(|| panic!("ABI methods"))
        .iter()
        .find(|method| method["name"] == "balanceOf")
        .unwrap_or_else(|| panic!("balanceOf ABI"));
    assert_eq!(method["inputs"][0]["type"], "bytes32");
    assert_eq!(method["inputs"][1]["type"], "bytes32");
    assert_eq!(method["outputs"][0]["type"], "uint128");

    let node = Node::start(LedgerReply::Funded);
    for route in ["node", "gateway"] {
        for (asset, expected) in [(NATIVE, NATIVE_FUNDS), (TOKEN, TOKEN_FUNDS)] {
            let endpoint = SequencerEndpoint::new(&node.endpoint, &[])
                .unwrap_or_else(|error| panic!("production reader: {error:?}"));
            let direct = balance(
                &endpoint,
                provisioned_account(asset),
                asset,
                ReadContext {
                    committed_height: endpoint
                        .block_number()
                        .unwrap_or_else(|error| panic!("node height: {error:?}")),
                    requested: Requested::new(Rung::Committed)
                        .unwrap_or_else(|error| panic!("committed read: {error:?}")),
                },
            )
            .unwrap_or_else(|error| panic!("SDK ledger balance: {error:?}"));
            assert_eq!(direct.value(), expected);
            let (success, output) = cli(&request(&node.endpoint, asset, route));
            assert!(success, "valid funded wallet must succeed: {output}");
            assert_eq!(
                output,
                json!({"ok":true,"result":{
                    "account_id":hex(&provisioned_account(asset)), "asset_id":hex(&asset),
                    "balance":direct.value().to_string()
                }})
            );
        }
    }
    let requests = node.finish();
    assert_eq!(
        requests.len(),
        16,
        "both sides must traverse the node for every read"
    );
    assert_eq!(
        requests
            .iter()
            .filter(|(path, _)| path == "POST /rpc HTTP/1.1")
            .count(),
        4
    );
    assert_eq!(
        requests
            .iter()
            .filter(|(path, _)| path == "POST / HTTP/1.1")
            .count(),
        12
    );
    assert_ne!(provisioned_account(NATIVE), provisioned_account(TOKEN));
}

#[test]
fn wallet_cli_balance_preserves_remote_refusal() {
    let node = Node::start(LedgerReply::Refusal);
    let (success, output) = cli(&request(&node.endpoint, TOKEN, "node"));
    assert!(!success);
    assert_eq!(
        output,
        json!({"ok":false,"error":{
            "code":3, "message":"execution reverted",
            "data":{"code":"LXP_ASSET_MISMATCH", "asset":hex(&TOKEN)}
        }})
    );
    assert_eq!(
        node.finish().len(),
        2,
        "refusal must come from the real transport"
    );
}

#[test]
fn wallet_cli_balance_preserves_configured_routes() {
    let node = Node::start(LedgerReply::Funded);
    for (suffix, route) in [
        ("node/evm", "node"),
        ("tenant", "gateway"),
        ("tenant/rpc/", "gateway"),
    ] {
        let endpoint = format!("{}{suffix}", node.endpoint);
        let (success, output) = cli(&request(&endpoint, TOKEN, route));
        assert!(success, "{output}");
        assert_eq!(output["result"]["balance"], TOKEN_FUNDS.to_string());
    }
    let paths: Vec<_> = node.finish().into_iter().map(|(path, _)| path).collect();
    assert_eq!(
        paths,
        [
            "POST /node/evm HTTP/1.1",
            "POST /node/evm HTTP/1.1",
            "POST /tenant/rpc HTTP/1.1",
            "POST /tenant/rpc HTTP/1.1",
            "POST /tenant/rpc HTTP/1.1",
            "POST /tenant/rpc HTTP/1.1"
        ]
    );
}

#[test]
fn wallet_cli_balance_rejects_malformed_abi_amounts() {
    for invalid in [
        json!("0x"),
        json!("0x01"),
        json!(format!("0x{}", "00".repeat(33))),
        json!(format!("0x{}", "01".repeat(32))),
        json!(format!("0x{}", "zz".repeat(32))),
        Value::Null,
    ] {
        let node = Node::start(LedgerReply::Malformed(invalid));
        let (success, output) = cli(&request(&node.endpoint, TOKEN, "node"));
        assert!(
            !success,
            "invalid ABI amount must never become a balance: {output}"
        );
        assert_eq!(
            output,
            json!({"ok":false,"error":{"kind":"wallet_request_refused_or_unavailable"}})
        );
        assert_eq!(node.finish().len(), 2);
    }
}

#[test]
fn wallet_cli_balance_rejects_invalid_selectors_before_transport() {
    let node = Node::start(LedgerReply::Funded);
    for (field, invalid) in [
        ("did", json!("bad::did")),
        ("asset", json!("04")),
        ("asset", json!("zz".repeat(32))),
    ] {
        let mut input = request(&node.endpoint, TOKEN, "node");
        input[field] = invalid;
        let (success, output) = cli(&input);
        assert!(!success);
        assert_eq!(output["ok"], false);
    }
    assert!(
        node.finish().is_empty(),
        "malformed selectors must not leave the CLI"
    );
}

fn funded_ledger() -> BTreeMap<(String, String), Amount> {
    [(NATIVE, NATIVE_FUNDS), (TOKEN, TOKEN_FUNDS)]
        .into_iter()
        .map(|(asset, funds)| {
            (
                (hex(&provisioned_account(asset)), hex(&asset)),
                Amount::from_u128(funds),
            )
        })
        .collect()
}
