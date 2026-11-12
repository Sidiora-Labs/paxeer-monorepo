use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_bridge_relayer::attestation::uint256_from_u64;
use layerx_bridge_relayer::journal::{Entry, Journal, Observation};
use layerx_bridge_relayer::relayer::RelayerError;
use layerx_bridge_relayer::rpc::{PaxeerRpc, RpcFault};
use layerx_bridge_relayer::solana::observe::{observe_deposits, Finding, SolanaSettings};
use layerx_bridge_relayer::solana::rpc::{Commitment, SolanaRpc};
use layerx_bridge_relayer::solana::{base58_fixed, SOLANA_CHAIN_ID};
use layerx_bridge_relayer::transport::PaxeerEndpoint;
use serde_json::{json, Value};

const TRANSCRIPT: &str = include_str!("../src/solana/fixtures/caught_cpi.json");
type Requests = Arc<Mutex<Vec<(String, String, Value)>>>;

struct FixtureServer {
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
    requests: Requests,
}

impl FixtureServer {
    fn fixture() -> Value {
        serde_json::from_str(TRANSCRIPT).or_panic("expected Ok value")
    }

    fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").or_panic("expected Ok value");
        let address = listener.local_addr().or_panic("expected Ok value");
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let requests: Requests = Arc::new(Mutex::new(Vec::new()));
        let received = Arc::clone(&requests);
        let worker = thread::spawn(move || {
            let fixture: Value = serde_json::from_str(TRANSCRIPT).or_panic("expected Ok value");
            for connection in listener.incoming() {
                let mut stream = connection.or_panic("expected Ok value");
                if stop.load(Ordering::Acquire) {
                    break;
                }
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .or_panic("expected Ok value");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .or_panic("expected Ok value");
                let mut reader = BufReader::new(stream.try_clone().or_panic("expected Ok value"));
                let mut first = String::new();
                reader.read_line(&mut first).or_panic("expected Ok value");
                let mode = first
                    .split_whitespace()
                    .nth(1)
                    .or_panic("expected Some value");
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).or_panic("expected Ok value"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((key, value)) = line.trim().split_once(':') {
                        if key.eq_ignore_ascii_case("content-length") {
                            assert!(length.is_none());
                            length =
                                Some(value.trim().parse::<usize>().unwrap_or_else(|error| {
                                    panic!("expected Ok value: {error:?}")
                                }));
                        }
                    }
                }
                let length = length.or_panic("expected Some value");
                assert!(length <= 16_384);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).or_panic("expected Ok value");
                let request: Value = serde_json::from_slice(&body).or_panic("expected Ok value");
                let method = request["method"].as_str().or_panic("expected Some value");
                received.lock().or_panic("expected Ok value").push((
                    mode.to_owned(),
                    method.to_owned(),
                    request["params"].clone(),
                ));
                let result = transcript_result(&fixture, mode, method, &request["params"]);
                let body = serde_json::to_vec(
                    &json!({"jsonrpc": "2.0", "id": request["id"], "result": result}),
                )
                .or_panic("expected Ok value");
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).or_panic("expected Ok value");
                stream.write_all(&body).or_panic("expected Ok value");
            }
        });
        Self {
            address,
            stopped,
            worker: Some(worker),
            requests,
        }
    }

    fn transport(&self, mode: &str) -> PaxeerRpc {
        PaxeerRpc::new(vec![PaxeerEndpoint::new(
            format!("http://{}{mode}", self.address),
            Duration::from_secs(5),
            None,
            SOLANA_CHAIN_ID,
            true,
        )
        .or_panic("expected Ok value")])
        .or_panic("expected Ok value")
    }

    fn rpc(&self, mode: &str) -> SolanaRpc {
        SolanaRpc::new(Box::new(self.transport(mode)))
    }
}

impl Drop for FixtureServer {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        TcpStream::connect_timeout(&self.address, Duration::from_secs(5))
            .or_panic("expected Ok value");
        self.worker
            .take()
            .or_panic("expected Some value")
            .join()
            .or_panic("expected Ok value");
    }
}

fn transcript_result(fixture: &Value, mode: &str, method: &str, params: &Value) -> Value {
    match method {
        "eth_chainId" => json!(format!("0x{SOLANA_CHAIN_ID:x}")),
        "getSignaturesForAddress" => {
            assert_eq!(
                params,
                &json!([fixture["program"], {"commitment": "finalized", "limit": 1000}])
            );
            let transactions = if mode == "/ancestor" {
                vec![&fixture["ancestor"]]
            } else {
                fixture["transactions"]
                    .as_array()
                    .or_panic("expected Some value")
                    .iter()
                    .rev()
                    .collect()
            };
            json!(transactions.iter().map(|tx| json!({
                "signature": tx["transaction"]["signatures"][0], "slot": tx["slot"], "err": null,
            })).collect::<Vec<_>>())
        }
        "getTransaction" => {
            assert_eq!(
                params[1],
                json!({"commitment": "finalized", "encoding": "json", "maxSupportedTransactionVersion": 0})
            );
            let tx = if mode == "/ancestor" {
                assert_eq!(
                    params[0],
                    fixture["ancestor"]["transaction"]["signatures"][0]
                );
                fixture["ancestor"].clone()
            } else {
                fixture["transactions"]
                    .as_array()
                    .or_panic("expected Some value")
                    .iter()
                    .find(|tx| tx["transaction"]["signatures"][0] == params[0])
                    .or_panic("expected Some value")
                    .clone()
            };
            mutate_transaction(fixture, mode, tx)
        }
        "getAccountInfo" => {
            assert_eq!(
                params[1],
                json!({"commitment": "finalized", "encoding": "base64"})
            );
            let address = params[0].as_str().or_panic("expected Some value");
            let mut account = fixture["accounts"]
                .get(address)
                .or_panic("account is in the transcript")
                .clone();
            if mode == "/missing-receipt"
                && address
                    == fixture["transactions"][0]["transaction"]["message"]["accountKeys"][5]
                        .as_str()
                        .or_panic("expected Some value")
            {
                account["value"] = Value::Null;
            }
            account
        }
        _ => panic!("unexpected transcript RPC method {method}"),
    }
}

fn mutate_transaction(fixture: &Value, mode: &str, mut tx: Value) -> Value {
    match mode {
        "/missing-log" => {
            tx["meta"]["logMessages"]
                .as_array_mut()
                .or_panic("expected Some value")
                .retain(|line| {
                    !line
                        .as_str()
                        .or_panic("expected Some value")
                        .starts_with("Program log: PXBR/deposit/v1")
                });
        }
        "/missing-outcome" => {
            let ending = format!(
                "Program {} success",
                fixture["program"].as_str().or_panic("expected Some value")
            );
            tx["meta"]["logMessages"]
                .as_array_mut()
                .or_panic("expected Some value")
                .retain(|line| line != &json!(ending));
        }
        "/borrowed-log" | "/spoofed-log" | "/duplicate-log" => {
            let logs = tx["meta"]["logMessages"]
                .as_array_mut()
                .or_panic("expected Some value");
            let index = logs
                .iter()
                .position(|line| {
                    line.as_str()
                        .or_panic("expected Some value")
                        .starts_with("Program log: PXBR/deposit/v1")
                })
                .or_panic("expected Some value");
            let record = if mode == "/duplicate-log" {
                logs[index].clone()
            } else {
                logs.remove(index)
            };
            // The first failed invocation and the caller cannot lend
            // evidence to the successful custody invocation.
            let at = match mode {
                "/borrowed-log" => 2,
                "/spoofed-log" => 1,
                _ => index,
            };
            logs.insert(at, record);
        }
        "/missing-invocations" => tx["meta"]["logMessages"] = json!([]),
        "/missing-parent-outcome" => {
            tx["meta"]["logMessages"]
                .as_array_mut()
                .or_panic("expected Some value")
                .pop()
                .or_panic("expected Some value");
        }
        "/missing-inner" => tx["meta"]["innerInstructions"] = json!([]),
        "/legacy-height" => {
            for ix in tx["meta"]["innerInstructions"][0]["instructions"]
                .as_array_mut()
                .or_panic("expected Some value")
            {
                ix.as_object_mut()
                    .or_panic("expected Some value")
                    .remove("stackHeight");
            }
        }
        "/native-prefix" => {
            let message = &mut tx["transaction"]["message"];
            let keys = message["accountKeys"]
                .as_array_mut()
                .or_panic("expected Some value");
            let index = keys.len();
            keys.push(json!("ComputeBudget111111111111111111111111111111"));
            message["instructions"]
                .as_array_mut()
                .or_panic("expected Some value")
                .insert(
                    0,
                    json!({"programIdIndex":index,"accounts":[],"data":"Fj2Eoy"}),
                );
            tx["meta"]["innerInstructions"][0]["index"] = json!(1);
        }
        "/wrong-depth" => {
            tx["meta"]["innerInstructions"][0]["instructions"][0]["stackHeight"] = json!(5);
        }
        "/truncated" => tx["meta"]["logMessages"]
            .as_array_mut()
            .or_panic("expected Some value")
            .push(json!("Log truncated")),
        _ => {}
    }
    tx
}

#[test]
fn solana_caught_cpi_progress_and_restart_recovery() {
    let runtime = FixtureServer::start();
    let info = FixtureServer::fixture();
    let settings = SolanaSettings {
        chain_id: SOLANA_CHAIN_ID,
        vault: layerx_bridge_relayer::hex::fixed("0x334121a65b47bd45c3f6381537d9180e98e445bc")
            .or_panic("expected Ok value"),
        program_id: base58_fixed(info["program"].as_str().or_panic("expected Some value"))
            .or_panic("expected Ok value"),
        finality_depth: 0,
        start_slot: 1050,
        max_slot_range: 100,
        commitment: Commitment::Finalized,
    };
    let rpc = runtime.rpc("/");
    assert_caught_invocation(&rpc, &settings);
    let first = observe_deposits(&rpc, &settings, 1050, 1050).or_panic("expected Ok value");
    assert_eq!(first.len(), 1, "the caught failure produces no deposit");
    let Finding::Deposit(observation) = &first[0] else {
        panic!("the valid CPI was refused");
    };
    assert!(
        matches!(observation, Observation::Inbound { amount, log_index: 7, .. } if *amount == uint256_from_u64(12_345_678))
    );
    assert_eq!(
        layerx_bridge_relayer::hex::prefixed(
            &observation
                .inbound_attestation()
                .or_panic("expected Some value")
                .digest()
        ),
        "0x3333b122e2a4e61ad324d5c875a8a9c72cf98a2724da26c211235e30a8d294ef"
    );
    for mode in ["/legacy-height", "/native-prefix"] {
        assert_eq!(
            observe_deposits(&runtime.rpc(mode), &settings, 1050, 1050)
                .or_panic("expected Ok value"),
            first
        );
    }
    let directory = std::env::temp_dir().join(format!("repair-22-31-161-{}", std::process::id()));
    std::fs::create_dir(&directory).or_panic("expected Ok value");
    let path = directory.join("journal.jsonl");
    let stream = format!("in:{SOLANA_CHAIN_ID}");
    let mut journal = Journal::open(&path).or_panic("expected Ok value");
    journal
        .append(&Entry::Observed {
            item: observation.key(),
            observation: *observation,
        })
        .or_panic("expected Ok value");
    journal
        .append(&Entry::Cursor {
            stream: stream.clone(),
            next_block: 1051,
        })
        .or_panic("expected Ok value");
    let recovered = journal.state().clone();
    drop(journal);
    drop(rpc);

    // Stop both consumer components and reopen them against the same fixture
    // server. The durable cursor gets beyond the formerly stuck scan window.
    let rpc = runtime.rpc("/");
    let mut journal = Journal::open(&path).or_panic("expected Ok value");
    assert_eq!(journal.state(), &recovered);
    let next = journal.state().cursors[&stream];
    let later = observe_deposits(&rpc, &settings, next, 1090).or_panic("expected Ok value");
    assert_eq!(later.len(), 1);
    let Finding::Deposit(later) = &later[0] else {
        panic!("the later deposit was refused");
    };
    assert!(
        matches!(later, Observation::Inbound { amount, log_index: 9, .. } if *amount == uint256_from_u64(4_200_000))
    );
    journal
        .append(&Entry::Observed {
            item: later.key(),
            observation: *later,
        })
        .or_panic("expected Ok value");
    journal
        .append(&Entry::Cursor {
            stream: stream.clone(),
            next_block: 1091,
        })
        .or_panic("expected Ok value");
    assert_eq!(journal.state().items.len(), 2);
    let saved = journal.state().clone();
    drop(journal);
    let journal = Journal::open(&path).or_panic("expected Ok value");
    assert_eq!(journal.state(), &saved);
    assert_eq!(journal.state().cursors[&stream], 1091);
    assert_eq!(
        observe_deposits(&rpc, &settings, 1050, 1050).or_panic("expected Ok value"),
        first
    );

    assert_invalid_custody_evidence(&runtime, &settings);
    drop(journal);
    std::fs::remove_dir_all(directory).or_panic("expected Ok value");
}

fn assert_caught_invocation(rpc: &SolanaRpc, settings: &SolanaSettings) {
    let signatures = rpc
        .get_signatures_for_address(&settings.program_id, None, 1000, settings.commitment)
        .or_panic("expected Ok value");
    let transaction = rpc
        .get_transaction(&signatures[1].signature, settings.commitment)
        .or_panic("expected Ok value")
        .or_panic("expected Some value");
    let custody: Vec<_> = transaction
        .invocations
        .iter()
        .filter(|invocation| invocation.program == settings.program_id)
        .collect();
    assert_eq!(custody.len(), 2);
    assert!(!custody[0].succeeded);
    assert!(!custody[0].committed);
    assert!(custody[1].succeeded && custody[1].committed);
    assert_ne!(custody[0].instruction_index, custody[1].instruction_index);
    assert_eq!(custody[0].parent, custody[1].parent);
}

fn assert_invalid_custody_evidence(runtime: &FixtureServer, settings: &SolanaSettings) {
    // A locally successful custody invocation is not committed if an
    // ancestor failed, even when the top-level caller caught that failure.
    let ancestor_rpc = runtime.rpc("/ancestor");
    let ancestor_signatures = ancestor_rpc
        .get_signatures_for_address(&settings.program_id, None, 1000, settings.commitment)
        .or_panic("expected Ok value");
    let ancestor = ancestor_rpc
        .get_transaction(&ancestor_signatures[0].signature, settings.commitment)
        .or_panic("expected Ok value")
        .or_panic("expected Some value");
    let custody = ancestor
        .invocations
        .iter()
        .find(|invocation| invocation.program == settings.program_id)
        .or_panic("expected Some value");
    assert!(custody.succeeded);
    assert!(!custody.committed);
    assert!(!ancestor.invocations[custody.parent.or_panic("expected Some value")].succeeded);
    assert!(observe_deposits(&ancestor_rpc, settings, 1050, 1050)
        .or_panic("expected Ok value")
        .is_empty());

    for mode in [
        "/missing-receipt",
        "/missing-log",
        "/missing-outcome",
        "/missing-parent-outcome",
        "/missing-invocations",
        "/missing-inner",
        "/borrowed-log",
        "/spoofed-log",
        "/duplicate-log",
        "/wrong-depth",
        "/truncated",
    ] {
        assert_eq!(
            observe_deposits(&runtime.rpc(mode), settings, 1050, 1050),
            Err(RelayerError::Rpc(RpcFault::Malformed)),
            "successful-deposit evidence must fail closed: {mode}"
        );
    }
    let requests = runtime.requests.lock().or_panic("expected Ok value");
    assert!(
        requests.iter().all(|(_, method, params)| {
            method != "getAccountInfo" || params[0] != FixtureServer::fixture()["missingReceipt"]
        }),
        "a caught failed CPI must never fetch a receipt"
    );
    assert!(
        requests
            .iter()
            .all(|(mode, method, _)| { mode != "/ancestor" || method != "getAccountInfo" }),
        "rolled-back calls must never fetch account evidence"
    );
    drop(requests);
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
