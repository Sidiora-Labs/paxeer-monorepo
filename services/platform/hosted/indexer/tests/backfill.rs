use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::AbiRegistry;
use layerx_indexer::backfill::Backfill;
use layerx_indexer::blockscout::{rpc_blocks, BlockscoutRange};
use layerx_indexer::follow::{FollowPolicy, StepOutcome};
use layerx_indexer::paxeer::{AttributeEncoding, PaxeerIngester, TX_INDEX_DISABLED};
use layerx_indexer::paxscan::{resolve_transaction_domains, PaxscanDatabase, PaxscanTls};
use layerx_indexer::store::{AssetRow, Store};
use layerx_indexer::transport::{Endpoint, Security};
use layerx_indexer::IndexError;
use serde_json::{json, Value};

type Handler = dyn Fn(&[u8]) -> (u16, Value) + Send + Sync;

fn fixture_at(path: &Path) -> Value {
    let text =
        std::fs::read_to_string(path).unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    serde_json::from_str(&text).unwrap_or_else(|error| panic!("{}: {error}", path.display()))
}

fn fixture(name: &str) -> Value {
    fixture_at(
        &Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("fixtures")
            .join(name),
    )
}

fn blockscout() -> BlockscoutRange {
    let value = fixture_at(
        &Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("tests/fixtures")
            .join("backfill_blockscout.json"),
    );
    serde_json::from_value(value).unwrap_or_else(|error| panic!("blockscout fixture: {error}"))
}

fn within(range: &BlockscoutRange, from: u64, to: u64, absent: &[u64]) -> BlockscoutRange {
    let keep = |height: u64| (from..=to).contains(&height) && !absent.contains(&height);
    BlockscoutRange {
        blocks: range
            .blocks
            .iter()
            .filter(|row| keep(row.number))
            .cloned()
            .collect(),
        transactions: range
            .transactions
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        logs: range
            .logs
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        token_transfers: range
            .token_transfers
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        internal_transactions: range
            .internal_transactions
            .iter()
            .filter(|row| keep(row.block_number))
            .cloned()
            .collect(),
        tokens: range.tokens.clone(),
    }
}

fn precompiles() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../precompiles")
}

fn read_request(stream: &mut TcpStream) -> Option<Vec<u8>> {
    let mut bytes = Vec::new();
    let mut chunk = [0_u8; 4096];
    let header_end = loop {
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            break position + 4;
        }
    };
    let head = String::from_utf8(bytes[..header_end].to_vec()).ok()?;
    let length = head
        .split("\r\n")
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .and_then(|(_, value)| value.trim().parse::<usize>().ok())
        .unwrap_or(0);
    while bytes.len() < header_end + length {
        let count = stream.read(&mut chunk).ok()?;
        if count == 0 {
            return None;
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    Some(bytes[header_end..header_end + length].to_vec())
}

#[derive(Default)]
struct NodeCounts {
    connections: AtomicUsize,
    requests: Mutex<Vec<(bool, Value)>>,
    batch_sizes: Mutex<Vec<usize>>,
    failures: AtomicUsize,
    batches: Mutex<Vec<(String, usize)>>,
}

#[derive(Clone, Copy)]
enum BatchReply {
    Complete,
    MissingBlock,
    ErrorBlock,
    MissingReceipt,
    ErrorReceipt,
    ForeignId,
    MissingDomain,
    ErrorDomain,
    DomainPosition(&'static str),
    DomainWithoutChainId,
    DomainsByIndex,
    FailBatchOnce(&'static str),
    FailLargeBatches(&'static str),
    DecodeBatch(&'static str),
    SourceDomainOnce,
}

fn serve(handler: Arc<Handler>, counts: Arc<NodeCounts>) -> String {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("bind: {error}"));
    let address = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("address: {error}"));
    thread::spawn(move || {
        for incoming in listener.incoming() {
            let Ok(mut stream) = incoming else {
                continue;
            };
            counts.connections.fetch_add(1, Ordering::Relaxed);
            let handler = Arc::clone(&handler);
            thread::spawn(move || {
                let Some(body) = read_request(&mut stream) else {
                    return;
                };
                let (status, body) = handler(&body);
                let body = body.to_string();
                let _ = write!(
                    stream,
                    "HTTP/1.1 {status} Fixture\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                let _ = stream.flush();
            });
        }
    });
    format!("http://{address}")
}

fn endpoint(url: &str) -> Endpoint {
    Endpoint::parse(
        url,
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(5),
    )
    .unwrap_or_else(|error| panic!("{error}"))
}

fn tx_search_range(query: &str) -> (u64, u64) {
    let mut bounds = query.split(" AND ").map(|part| {
        part.split_once('=')
            .and_then(|(_, value)| value.parse::<u64>().ok())
            .unwrap_or_else(|| panic!("tx_search query {query}"))
    });
    let low = bounds.next().unwrap_or_else(|| panic!("low bound"));
    let high = bounds.next().unwrap_or_else(|| panic!("high bound"));
    (low, high)
}

/// How the recorded node answers `tx_search`.
#[derive(Clone, Copy)]
enum TxIndex {
    On,
    Off,
    Broken,
    Recovering,
}

/// A JSON-RPC node answering from the recorded `paxeer_blocks.json` and
/// `tx_search.json` fixtures, serving hash-only blocks for verification.
fn node(tx_index: TxIndex) -> String {
    counted_node(tx_index, BatchReply::Complete, &blockscout()).0
}

fn counted_node(
    tx_index: TxIndex,
    reply: BatchReply,
    rows: &BlockscoutRange,
) -> (String, Arc<NodeCounts>) {
    let mut chain = fixture("paxeer_blocks.json");
    for (height, block) in rpc_blocks(rows).unwrap_or_else(|error| panic!("{error}")) {
        if height > 3 {
            chain["canonical"]["blocks"][height.to_string()] = block.block;
        }
    }
    let search = fixture("tx_search.json");
    let counts = Arc::new(NodeCounts::default());
    let recorded = Arc::clone(&counts);
    let url = serve(
        Arc::new(move |body| {
            let request: Value =
                serde_json::from_slice(body).unwrap_or_else(|error| panic!("{error}"));
            let is_batch = request.is_array();
            let requests = request.as_array().cloned().unwrap_or_else(|| vec![request]);
            recorded
                .requests
                .lock()
                .unwrap_or_else(|error| panic!("{error}"))
                .extend(requests.iter().cloned().map(|request| (is_batch, request)));
            if is_batch {
                recorded
                    .batch_sizes
                    .lock()
                    .unwrap_or_else(|error| panic!("{error}"))
                    .push(requests.len());
            }
            if is_batch {
                recorded
                    .batches
                    .lock()
                    .unwrap_or_else(|error| panic!("{error}"))
                    .push((
                        requests[0]["method"]
                            .as_str()
                            .unwrap_or_else(|| panic!("method"))
                            .to_owned(),
                        requests.len(),
                    ));
            }
            if is_batch {
                match reply {
                    BatchReply::FailBatchOnce(method)
                        if requests.len() > 1
                            && requests[0]["method"] == method
                            && recorded
                                .failures
                                .compare_exchange(0, 1, Ordering::Relaxed, Ordering::Relaxed)
                                .is_ok() =>
                    {
                        return (503, Value::Null)
                    }
                    BatchReply::FailLargeBatches(method)
                        if requests.len() > 1 && requests[0]["method"] == method =>
                    {
                        recorded.failures.fetch_add(1, Ordering::Relaxed);
                        return (503, Value::Null);
                    }
                    BatchReply::DecodeBatch(method) if requests[0]["method"] == method => {
                        return (200, json!(false));
                    }
                    _ => {}
                }
            }
            let mut answers: Vec<Value> = requests
            .iter()
            .map(|request| {
                let params = &request["params"];
                let view = &chain["canonical"];
                let answer =
                    |result: Value| json!({"jsonrpc": "2.0", "id": request["id"], "result": result});
                match request["method"].as_str().unwrap_or_default() {
                    "eth_chainId" => answer(chain["chain_id"].clone()),
                    "eth_blockNumber" => answer(view["head"].clone()),
                    "eth_getBlockByNumber" => {
                        let quantity = params[0].as_str().unwrap_or_default();
                        let height = u64::from_str_radix(
                            quantity.trim_start_matches("0x"),
                            16,
                        )
                        .unwrap_or(u64::MAX);
                        let mut block = view["blocks"]
                            .get(height.to_string())
                            .cloned()
                            .unwrap_or(Value::Null);
                        if params[1] == Value::Bool(false) {
                            if let Some(transactions) =
                                block.get_mut("transactions").and_then(Value::as_array_mut)
                            {
                                for transaction in transactions.iter_mut() {
                                    *transaction = transaction["hash"].clone();
                                }
                            }
                        }
                        answer(block)
                    }
                    "eth_getTransactionByHash" => {
                        let mut transaction = chain["canonical"]["blocks"]
                            .as_object()
                            .unwrap_or_else(|| panic!("blocks"))
                            .values()
                            .flat_map(|block| {
                                block["transactions"]
                                    .as_array()
                                    .unwrap_or_else(|| panic!("transactions"))
                            })
                            .find(|transaction| transaction["hash"] == params[0])
                            .cloned()
                            .unwrap_or_else(|| panic!("transaction"));
                        if params[0] == chain["canonical"]["blocks"]["3"]["transactions"][0]["hash"]
                        {
                            match reply {
                                BatchReply::DomainPosition(field) => {
                                    transaction[field] = if field == "hash" {
                                        json!(format!("0x{:064x}", 99))
                                    } else {
                                        json!("0xff")
                                    };
                                }
                                BatchReply::DomainWithoutChainId => {
                                    transaction
                                        .as_object_mut()
                                        .unwrap_or_else(|| panic!("transaction"))
                                        .remove("chainId");
                                }
                                BatchReply::SourceDomainOnce
                                    if recorded.failures.load(Ordering::Relaxed) == 0 =>
                                {
                                    if !is_batch {
                                        recorded.failures.fetch_add(1, Ordering::Relaxed);
                                    }
                                    return json!({"jsonrpc":"2.0", "id":request["id"],
                                    "error":{"code":-32000,"message":"entry unavailable"}});
                                }
                                _ => {}
                            }
                        }
                        if matches!(reply, BatchReply::DomainsByIndex) {
                            let index = u64::from_str_radix(
                                transaction["transactionIndex"]
                                    .as_str()
                                    .unwrap_or_else(|| panic!("index"))
                                    .trim_start_matches("0x"),
                                16,
                            )
                            .unwrap_or_else(|error| panic!("{error}"));
                            transaction["chainId"] = json!(format!("0x{:x}", 229 + index));
                        }
                        answer(transaction)
                    }
                    "eth_getTransactionReceipt" => {
                        let hash = params[0].as_str().unwrap_or_default();
                        answer(chain["receipts"].get(hash).cloned().unwrap_or(Value::Null))
                    }
                    "tx_search" => match tx_index {
                        TxIndex::Off => json!({
                            "jsonrpc": "2.0",
                            "id": request["id"],
                            "error": {"code": -32603, "message": "Internal error", "data": TX_INDEX_DISABLED}
                        }),
                        TxIndex::Broken => json!({
                            "jsonrpc": "2.0",
                            "id": request["id"],
                            "error": {"code": -32603, "message": "Internal error", "data": "database is locked"}
                        }),
                        TxIndex::On | TxIndex::Recovering => {
                            if matches!(tx_index, TxIndex::Recovering) {
                                let calls = recorded
                                    .requests
                                    .lock()
                                    .unwrap_or_else(|error| panic!("{error}"));
                                let tries = calls
                                    .iter()
                                    .filter(|(_, call)| {
                                        call["method"] == "tx_search" && call["params"] == *params
                                    })
                                    .count();
                                if tries <= 5 {
                                    return json!({"jsonrpc":"2.0", "id":request["id"],
                                    "error":{"code":-32000,"message":"history unavailable"}});
                                }
                            }
                            let (low, high) = tx_search_range(params["query"].as_str().unwrap_or_default());
                            let matching: Vec<Value> = search["result"]["txs"]
                                .as_array()
                                .map_or(&[][..], Vec::as_slice)
                                .iter()
                                .filter(|tx| {
                                    let height: u64 = tx["height"]
                                        .as_str()
                                        .and_then(|text| text.parse().ok())
                                        .unwrap_or(0);
                                    (low..=high).contains(&height)
                                })
                                .cloned()
                                .collect();
                            let total = matching.len();
                            answer(json!({ "txs": matching, "total_count": total.to_string() }))
                        }
                    },
                    other => panic!("unexpected JSON-RPC method {other}"),
                }
            })
            .collect();
            if is_batch {
                let method = match reply {
                    BatchReply::MissingBlock | BatchReply::ErrorBlock => "eth_getBlockByNumber",
                    BatchReply::MissingReceipt | BatchReply::ErrorReceipt => {
                        "eth_getTransactionReceipt"
                    }
                    BatchReply::MissingDomain | BatchReply::ErrorDomain => "eth_getTransactionByHash",
                    _ => "",
                };
                if let Some(index) = requests.iter().position(|request| {
                    request["method"] == method
                        && match reply {
                            BatchReply::MissingBlock | BatchReply::ErrorBlock => {
                                request["params"][0] == "0x2"
                            }
                            _ => {
                                request["params"][0]
                                    == chain["canonical"]["blocks"]["2"]["transactions"][0]["hash"]
                            }
                        }
                }) {
                    match reply {
                        BatchReply::MissingBlock | BatchReply::MissingReceipt | BatchReply::MissingDomain => {
                            answers.remove(index);
                        }
                        BatchReply::ErrorBlock | BatchReply::ErrorReceipt | BatchReply::ErrorDomain => {
                            answers[index] = json!({"jsonrpc":"2.0", "id":requests[index]["id"],
                            "error":{"code":-32000,"message":"entry unavailable"}});
                        }
                        _ => {}
                    }
                }
                if matches!(reply, BatchReply::ForeignId) {
                    answers.push(json!({"jsonrpc":"2.0","id":0,"result":null}));
                }
                answers.reverse();
                (200, Value::Array(answers))
            } else {
                (200, answers.into_iter().next().unwrap_or(Value::Null))
            }
        }),
        Arc::clone(&counts),
    );
    (url, counts)
}

fn long_range() -> BlockscoutRange {
    let mut rows = blockscout();
    for height in 4..=120 {
        let mut block = rows
            .blocks
            .last()
            .cloned()
            .unwrap_or_else(|| panic!("block"));
        block.parent_hash = block.hash;
        block.hash = layerx_indexer::codec::unhex(&format!("0x{:064x}", height + 0x1000))
            .unwrap_or_else(|error| panic!("{error}"));
        block.number = height;
        block.timestamp += 1;
        block.gas_used = "0".to_owned();
        rows.blocks.push(block);
    }
    rows
}

struct RangeStore {
    store: Store,
    path: PathBuf,
    created_at: i64,
}

fn unix_seconds() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("{error}"))
        .as_secs()
        .try_into()
        .unwrap_or_else(|error| panic!("{error}"))
}

impl RangeStore {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "indexer-backfill-{}-{}.sqlite",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        let created_at = unix_seconds();
        let store = Store::open(&path).unwrap_or_else(|error| panic!("{error}"));
        Self {
            store,
            path,
            created_at,
        }
    }

    fn chain_rows(&self) -> Vec<(String, Vec<Vec<rusqlite::types::Value>>)> {
        let connection =
            rusqlite::Connection::open(&self.path).unwrap_or_else(|error| panic!("{error}"));
        let mut tables = connection
            .prepare("SELECT name FROM sqlite_schema WHERE type = 'table' ORDER BY name")
            .unwrap_or_else(|error| panic!("{error}"));
        let names: Vec<String> = tables
            .query_map([], |row| row.get(0))
            .unwrap_or_else(|error| panic!("{error}"))
            .map(|name| name.unwrap_or_else(|error| panic!("{error}")))
            .collect();
        names
            .into_iter()
            .map(|name| {
                let mut statement = connection
                    .prepare(&format!(
                        "SELECT * FROM \"{}\" ORDER BY rowid",
                        name.replace('"', "\"\"")
                    ))
                    .unwrap_or_else(|error| panic!("{error}"));
                let columns: Vec<bool> = statement
                    .column_names()
                    .map(|column| {
                        (name == "assets" && column == "first_seen")
                            || (name == "backfill_cursors" && column == "updated_at")
                    })
                    .collect();
                let observed_at = unix_seconds();
                let rows = statement
                    .query_map([], |row| {
                        let mut data = Vec::new();
                        for (column, ingestion_time) in columns.iter().enumerate() {
                            if *ingestion_time {
                                let stamp: i64 = row.get(column)?;
                                assert!((self.created_at..=observed_at).contains(&stamp));
                            } else {
                                data.push(row.get::<_, rusqlite::types::Value>(column)?);
                            }
                        }
                        Ok(data)
                    })
                    .unwrap_or_else(|error| panic!("{error}"))
                    .map(|row| row.unwrap_or_else(|error| panic!("{error}")))
                    .collect();
                (name, rows)
            })
            .collect()
    }
}

impl Drop for RangeStore {
    fn drop(&mut self) {
        std::fs::remove_file(&self.path).unwrap_or_else(|error| panic!("{error}"));
    }
}

fn store() -> Store {
    let chain = fixture("paxeer_blocks.json");
    let pointer = chain["pointer"]["address"]
        .as_str()
        .unwrap_or_default()
        .to_owned();
    let store = Store::open_in_memory().unwrap_or_else(|error| panic!("{error}"));
    store
        .register_assets(&[AssetRow {
            asset: format!("evm:{pointer}"),
            chain: "paxeer".to_owned(),
            kind: "pointer".to_owned(),
            address: Some(pointer),
            denom: Some("upax".to_owned()),
            metadata: json!({}),
        }])
        .unwrap_or_else(|error| panic!("{error}"));
    store
}

fn ingester(url: &str) -> PaxeerIngester {
    let registry = AbiRegistry::load_dir(&precompiles()).unwrap_or_else(|error| panic!("{error}"));
    PaxeerIngester::new(
        endpoint(url),
        Some(endpoint(url)),
        registry,
        FollowPolicy {
            finality_depth: 2,
            max_units_per_step: 16,
        },
        1,
        Some(0xe5),
        AttributeEncoding::Base64,
    )
}

fn assets(store: &Store) -> Vec<Value> {
    let page = store
        .assets(None, 500)
        .unwrap_or_else(|error| panic!("{error}"));
    page.items
        .into_iter()
        .map(|mut asset| {
            if let Some(object) = asset.as_object_mut() {
                object.remove("first_seen");
            }
            asset
        })
        .collect()
}

fn history(store: &Store) -> (Vec<Value>, Vec<Value>, Vec<Value>) {
    (
        store
            .transfers("paxeer")
            .unwrap_or_else(|error| panic!("{error}")),
        store
            .events("paxeer")
            .unwrap_or_else(|error| panic!("{error}")),
        assets(store),
    )
}

fn live_rows() -> (Vec<Value>, Vec<Value>, Vec<Value>) {
    let url = node(TxIndex::On);
    let live = ingester(&url);
    let store = store();
    assert_eq!(
        live.step(&store).unwrap_or_else(|error| panic!("{error}")),
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );
    history(&store)
}

fn kinds(rows: &[Value]) -> Vec<String> {
    let mut kinds: Vec<String> = rows
        .iter()
        .filter_map(|row| row["kind"].as_str().map(str::to_owned))
        .collect();
    kinds.sort();
    kinds.dedup();
    kinds
}

#[test]
fn backfill_blockscout_rows_convert_to_the_live_rpc_shapes() {
    let chain = fixture("paxeer_blocks.json");
    let converted = rpc_blocks(&blockscout()).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(converted.keys().copied().collect::<Vec<_>>(), vec![1, 2, 3]);
    for (height, block) in &converted {
        let live = &chain["canonical"]["blocks"][height.to_string()];
        for field in [
            "number",
            "hash",
            "parentHash",
            "timestamp",
            "miner",
            "gasUsed",
        ] {
            assert_eq!(block.block[field], live[field], "block {height} {field}");
        }
        let live_transactions = live["transactions"]
            .as_array()
            .map_or(&[][..], Vec::as_slice);
        let transactions = block.block["transactions"]
            .as_array()
            .map_or(&[][..], Vec::as_slice);
        assert_eq!(transactions.len(), live_transactions.len());
        for (ours, theirs) in transactions.iter().zip(live_transactions) {
            for field in [
                "hash",
                "blockHash",
                "blockNumber",
                "transactionIndex",
                "from",
                "to",
                "value",
                "input",
                "chainId",
            ] {
                assert_eq!(ours[field], theirs[field], "block {height} tx {field}");
            }
            let hash = theirs["hash"].as_str().unwrap_or_default();
            let receipt = &block.receipts[hash];
            let live_receipt = &chain["receipts"][hash];
            for field in [
                "transactionHash",
                "transactionIndex",
                "blockHash",
                "blockNumber",
                "from",
                "to",
                "gasUsed",
                "contractAddress",
                "status",
                "logs",
            ] {
                assert_eq!(
                    receipt[field], live_receipt[field],
                    "receipt {hash} {field}"
                );
            }
        }
    }

    let mut orphan = blockscout();
    orphan.internal_transactions[0].transaction_index = 9;
    assert!(matches!(rpc_blocks(&orphan), Err(IndexError::Integrity(_))));
    let mut disagreeing = blockscout();
    disagreeing.token_transfers[0].amount = Some("999".to_owned());
    assert!(matches!(
        rpc_blocks(&disagreeing),
        Err(IndexError::Integrity(_))
    ));
}

#[test]
fn backfill_from_the_blockscout_fixture_writes_the_live_rows() {
    let live = live_rows();
    let url = node(TxIndex::On);
    let node = ingester(&url);
    let store = store();
    let rows = blockscout();
    let plan = Backfill::new(&node, 3, 10);
    let cutover = plan
        .run(&store, |from, to| Ok(within(&rows, from, to, &[])))
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(cutover, 3);
    let backfilled = history(&store);
    assert_eq!(backfilled, live);
    assert!(kinds(&backfilled.0).contains(&"erc20_transfer".to_owned()));
    assert!(kinds(&backfilled.0).contains(&"native_transfer".to_owned()));
    assert!(kinds(&backfilled.0).contains(&"pointer_transfer".to_owned()));
}

#[test]
fn backfill_batch_readers_preserve_single_reader_rows() {
    let rows = blockscout();
    let batched_node_url = node(TxIndex::On);
    let batched_node = ingester(&batched_node_url);
    let batched_store = store();
    Backfill::with_options(&batched_node, 3, 3, 4, 25)
        .apply_range(&batched_store, 1, 3, &rows)
        .unwrap_or_else(|error| panic!("{error}"));

    let single_node_url = node(TxIndex::On);
    let single_node = ingester(&single_node_url);
    let single_store = store();
    Backfill::with_options(&single_node, 3, 3, 1, 1)
        .apply_range(&single_store, 1, 3, &rows)
        .unwrap_or_else(|error| panic!("{error}"));

    assert_eq!(history(&batched_store), history(&single_store));
}

#[test]
fn backfill_120_heights_use_bounded_batches_and_preserve_stored_chain_rows() {
    let rows = long_range();
    let (url, counts) = counted_node(TxIndex::On, BatchReply::Complete, &rows);
    let batched_node = ingester(&url);
    let batched = RangeStore::new();
    Backfill::with_options(&batched_node, 120, 120, 4, 25)
        .apply_range(&batched.store, 1, 120, &rows)
        .unwrap_or_else(|error| panic!("{error}"));

    let (url, single_counts) = counted_node(TxIndex::On, BatchReply::Complete, &rows);
    let single_node = ingester(&url);
    let single = RangeStore::new();
    Backfill::with_options(&single_node, 120, 120, 1, 1)
        .apply_range(&single.store, 1, 120, &rows)
        .unwrap_or_else(|error| panic!("{error}"));

    assert_eq!(batched.chain_rows(), single.chain_rows());
    assert_eq!(
        batched
            .store
            .backfill_cursor("paxeer")
            .unwrap_or_else(|error| panic!("{error}"))
            .unwrap_or_else(|| panic!("backfill cursor"))
            .position,
        120
    );
    assert!(counts.connections.load(Ordering::Relaxed) < 120);
    let batches = counts
        .batch_sizes
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(batches.iter().all(|size| (1..=25).contains(size)));
    assert_eq!(batches.iter().filter(|size| **size == 25).count(), 4);
    assert!(single_counts
        .batch_sizes
        .lock()
        .unwrap_or_else(|error| panic!("{error}"))
        .is_empty());
    let calls = counts
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    let singles = single_counts
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(calls.len(), singles.len());
    assert_eq!(
        calls
            .iter()
            .filter(|(_, call)| call["method"] == "eth_getBlockByNumber")
            .count(),
        120
    );
    assert!(singles.iter().all(|(batch, _)| !batch));
}

#[test]
fn backfill_missing_and_error_batch_entries_read_only_the_entry_again() {
    let rows = long_range();
    let (url, _) = counted_node(TxIndex::On, BatchReply::Complete, &rows);
    let baseline_node = ingester(&url);
    let baseline = RangeStore::new();
    Backfill::with_options(&baseline_node, 120, 120, 1, 1)
        .apply_range(&baseline.store, 1, 120, &rows)
        .unwrap_or_else(|error| panic!("{error}"));
    let expected = baseline.chain_rows();
    for reply in [
        BatchReply::MissingBlock,
        BatchReply::ErrorBlock,
        BatchReply::MissingReceipt,
        BatchReply::ErrorReceipt,
    ] {
        let (url, counts) = counted_node(TxIndex::On, reply, &rows);
        let node = ingester(&url);
        let store = RangeStore::new();
        Backfill::with_options(&node, 120, 120, 4, 25)
            .apply_range(&store.store, 1, 120, &rows)
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(store.chain_rows(), expected);
        let requests = counts
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        let singles: Vec<_> = requests
            .iter()
            .filter(|(batch, request)| !batch && request["method"] != "tx_search")
            .collect();
        assert_eq!(singles.len(), 1);
        let request = &singles[0].1;
        match reply {
            BatchReply::MissingBlock | BatchReply::ErrorBlock => {
                assert_eq!(request["method"], "eth_getBlockByNumber");
                assert_eq!(request["params"], json!(["0x2", true]));
            }
            BatchReply::MissingReceipt | BatchReply::ErrorReceipt => {
                let chain = fixture("paxeer_blocks.json");
                assert_eq!(request["method"], "eth_getTransactionReceipt");
                assert_eq!(
                    request["params"],
                    json!([chain["canonical"]["blocks"]["2"]["transactions"][0]["hash"]])
                );
            }
            _ => panic!("entry fallback"),
        }
    }
}

#[test]
fn backfill_batch_hash_mismatch_and_foreign_ids_leave_store_rows_unchanged() {
    let rows = long_range();
    for reply in [BatchReply::Complete, BatchReply::ForeignId] {
        let (url, _) = counted_node(TxIndex::On, reply, &rows);
        let node = ingester(&url);
        let store = RangeStore::new();
        let before = store.chain_rows();
        let mut explorer = rows.clone();
        if matches!(reply, BatchReply::Complete) {
            explorer.blocks[119].hash[31] ^= 1;
        }
        let error = Backfill::with_options(&node, 120, 120, 4, 25)
            .apply_range(&store.store, 1, 120, &explorer)
            .err()
            .unwrap_or_else(|| panic!("range must refuse invalid node answers"));
        match reply {
            BatchReply::Complete => assert!(
                matches!(&error, IndexError::Integrity(message)
                if message.contains("paxscan block 120 hash")),
                "{error}"
            ),
            BatchReply::ForeignId => assert!(
                matches!(&error, IndexError::Source(message)
                if message.contains("another request")),
                "{error}"
            ),
            _ => panic!("range integrity"),
        }
        assert_eq!(store.chain_rows(), before);
    }
}

fn domain_baseline() -> (BlockscoutRange, RangeStore) {
    let mut rows = blockscout();
    let (url, _) = counted_node(TxIndex::On, BatchReply::Complete, &rows);
    let node = ingester(&url);
    for row in &mut rows.transactions {
        row.chain_id = Some(
            node.transaction_domain(row)
                .unwrap_or_else(|error| panic!("{error}"))
                .to_string(),
        );
    }
    let store = RangeStore::new();
    Backfill::new(&node, 3, 3)
        .apply_range(&store.store, 1, 3, &rows)
        .unwrap_or_else(|error| panic!("{error}"));
    (rows, store)
}

#[test]
fn backfill_transaction_domains_use_bounded_batches_and_preserve_rows() {
    let (expected, baseline) = domain_baseline();
    let mut rows = blockscout();
    for row in &mut rows.transactions {
        row.chain_id = Some("125000999".to_owned());
    }
    assert!(
        rows.transactions
            .iter()
            .filter(|row| row.block_number == 2)
            .count()
            > 1
    );
    let (url, counts) = counted_node(TxIndex::On, BatchReply::Complete, &rows);
    let node = ingester(&url);
    resolve_transaction_domains(&mut rows, &node, 2, 2).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(rows, expected);
    let batches = counts
        .batches
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(batches.len(), 3);
    assert!(batches.len() < rows.transactions.len());
    assert!(batches
        .iter()
        .all(|(method, size)| method == "eth_getTransactionByHash" && (1..=2).contains(size)));
    assert_eq!(counts.connections.load(Ordering::Relaxed), batches.len());
    drop(batches);
    let store = RangeStore::new();
    Backfill::with_options(&node, 3, 3, 2, 2)
        .apply_range(&store.store, 1, 3, &rows)
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(store.chain_rows(), baseline.chain_rows());
    let mut empty = BlockscoutRange::default();
    let before = counts.connections.load(Ordering::Relaxed);
    resolve_transaction_domains(&mut empty, &node, 2, 2).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(counts.connections.load(Ordering::Relaxed), before);
}

#[test]
fn backfill_transaction_domains_keep_each_answer_in_row_order() {
    let mut rows = blockscout();
    rows.transactions.reverse();
    let (url, _) = counted_node(TxIndex::On, BatchReply::DomainsByIndex, &rows);
    let node = ingester(&url);
    let expected: Vec<_> = rows
        .transactions
        .iter()
        .map(|row| Some((229 + row.index).to_string()))
        .collect();
    resolve_transaction_domains(&mut rows, &node, 2, 2).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(
        rows.transactions
            .iter()
            .map(|row| row.chain_id.clone())
            .collect::<Vec<_>>(),
        expected
    );
}

#[test]
fn backfill_missing_and_error_domain_entries_read_only_that_entry_alone() {
    let (expected, baseline) = domain_baseline();
    let target = fixture("paxeer_blocks.json")["canonical"]["blocks"]["2"]["transactions"][0]
        ["hash"]
        .clone();
    for reply in [BatchReply::MissingDomain, BatchReply::ErrorDomain] {
        let mut rows = blockscout();
        let (url, counts) = counted_node(TxIndex::On, reply, &rows);
        let node = ingester(&url);
        resolve_transaction_domains(&mut rows, &node, 2, 2)
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(rows, expected);
        let calls = counts
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(calls.len(), rows.transactions.len() + 1);
        let singles: Vec<_> = calls.iter().filter(|(batch, _)| !batch).collect();
        assert_eq!(singles.len(), 1);
        assert_eq!(singles[0].1["method"], "eth_getTransactionByHash");
        assert_eq!(singles[0].1["params"], json!([target]));
        drop(calls);
        let store = RangeStore::new();
        Backfill::with_options(&node, 3, 3, 2, 2)
            .apply_range(&store.store, 1, 3, &rows)
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(store.chain_rows(), baseline.chain_rows());
    }
}

#[test]
fn backfill_invalid_domain_answers_leave_range_and_store_unchanged() {
    for reply in [
        BatchReply::DomainPosition("hash"),
        BatchReply::DomainPosition("blockNumber"),
        BatchReply::DomainPosition("transactionIndex"),
        BatchReply::DomainWithoutChainId,
    ] {
        let mut rows = blockscout();
        for row in &mut rows.transactions {
            row.chain_id = None;
        }
        let original = rows.clone();
        let (url, _) = counted_node(TxIndex::On, reply, &rows);
        let node = ingester(&url);
        let store = RangeStore::new();
        let before = store.chain_rows();
        let result = Backfill::with_options(&node, 3, 3, 1, 2).step(&store.store, |_, _| {
            resolve_transaction_domains(&mut rows, &node, 2, 1)?;
            Ok(rows.clone())
        });
        match reply {
            BatchReply::DomainWithoutChainId => {
                assert!(matches!(result, Err(IndexError::Decode(_))))
            }
            _ => assert!(matches!(result, Err(IndexError::Integrity(_)))),
        }
        assert_eq!(rows, original);
        assert_eq!(store.chain_rows(), before);
    }
}

#[test]
fn backfill_source_failed_batches_split_and_commit_the_range() {
    let (expected, baseline) = domain_baseline();
    for method in [
        "eth_getTransactionByHash",
        "eth_getBlockByNumber",
        "eth_getTransactionReceipt",
    ] {
        let mut rows = blockscout();
        let (url, counts) = counted_node(TxIndex::On, BatchReply::FailBatchOnce(method), &rows);
        let node = ingester(&url);
        let store = RangeStore::new();
        Backfill::with_options(&node, 3, 3, 1, 5)
            .step(&store.store, |_, _| {
                resolve_transaction_domains(&mut rows, &node, 5, 1)?;
                Ok(rows.clone())
            })
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(rows, expected);
        assert_eq!(store.chain_rows(), baseline.chain_rows());
        assert_eq!(counts.failures.load(Ordering::Relaxed), 1);
        let batches = counts
            .batches
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        let sizes: Vec<_> = batches
            .iter()
            .filter(|(name, _)| name == method)
            .map(|(_, size)| *size)
            .collect();
        assert_eq!(
            sizes,
            if method == "eth_getBlockByNumber" {
                vec![3, 1, 2]
            } else {
                vec![5, 2, 3]
            }
        );
    }
}

#[test]
fn backfill_source_failed_batches_split_down_to_single_entries() {
    let (_, baseline) = domain_baseline();
    for method in [
        "eth_getTransactionByHash",
        "eth_getBlockByNumber",
        "eth_getTransactionReceipt",
    ] {
        let mut rows = blockscout();
        let (url, counts) = counted_node(TxIndex::On, BatchReply::FailLargeBatches(method), &rows);
        let node = ingester(&url);
        let store = RangeStore::new();
        Backfill::with_options(&node, 3, 3, 1, 5)
            .step(&store.store, |_, _| {
                resolve_transaction_domains(&mut rows, &node, 5, 1)?;
                Ok(rows.clone())
            })
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(store.chain_rows(), baseline.chain_rows());
        let batches = counts
            .batches
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        let sizes: Vec<_> = batches
            .iter()
            .filter(|(name, _)| name == method)
            .map(|(_, size)| *size)
            .collect();
        assert_eq!(
            sizes,
            if method == "eth_getBlockByNumber" {
                vec![3, 1, 2, 1, 1]
            } else {
                vec![5, 2, 1, 1, 3, 1, 2, 1, 1]
            }
        );
        assert_eq!(
            counts.failures.load(Ordering::Relaxed),
            if method == "eth_getBlockByNumber" {
                2
            } else {
                4
            }
        );
    }
}

#[test]
fn backfill_decode_failed_batches_are_not_split() {
    for method in [
        "eth_getTransactionByHash",
        "eth_getBlockByNumber",
        "eth_getTransactionReceipt",
    ] {
        let mut rows = blockscout();
        let (url, counts) = counted_node(TxIndex::On, BatchReply::DecodeBatch(method), &rows);
        let node = ingester(&url);
        let store = RangeStore::new();
        let before = store.chain_rows();
        let result = Backfill::with_options(&node, 3, 3, 1, 5)
            .with_retry_delay(Duration::ZERO)
            .run(&store.store, |_, _| {
                resolve_transaction_domains(&mut rows, &node, 5, 1)?;
                Ok(rows.clone())
            });
        assert!(matches!(result, Err(IndexError::Decode(_))));
        assert_eq!(store.chain_rows(), before);
        let batches = counts
            .batches
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(batches.iter().filter(|(name, _)| name == method).count(), 1);
    }
}

#[test]
fn backfill_source_failed_step_repeats_the_same_range_and_commits() {
    let (expected, baseline) = domain_baseline();
    let rows = blockscout();
    let (url, counts) = counted_node(TxIndex::On, BatchReply::SourceDomainOnce, &rows);
    let node = ingester(&url);
    let store = RangeStore::new();
    let mut tries = Vec::new();
    let cutover = Backfill::with_options(&node, 3, 3, 2, 2)
        .with_retry_delay(Duration::from_millis(1))
        .run(&store.store, |from, to| {
            tries.push((from, to));
            let mut rows = rows.clone();
            resolve_transaction_domains(&mut rows, &node, 2, 2)?;
            assert_eq!(rows, expected);
            Ok(rows)
        })
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(cutover, 3);
    assert_eq!(tries, vec![(1, 3), (1, 3)]);
    assert_eq!(counts.failures.load(Ordering::Relaxed), 1);
    assert_eq!(
        store
            .store
            .cursor("paxeer")
            .unwrap_or_else(|error| panic!("{error}"))
            .unwrap_or_else(|| panic!("live cursor"))
            .position,
        3
    );
    baseline
        .store
        .finish_backfill("paxeer", 3, 2)
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(history(&store.store), history(&baseline.store));
}

#[test]
fn backfill_persistent_source_failure_stops_after_five_repeats() {
    let rows = blockscout();
    let (url, counts) = counted_node(TxIndex::Broken, BatchReply::Complete, &rows);
    let node = ingester(&url);
    let store = RangeStore::new();
    let before = store.chain_rows();
    let mut tries = Vec::new();
    let result = Backfill::new(&node, 3, 3)
        .with_retry_delay(Duration::ZERO)
        .run(&store.store, |from, to| {
            tries.push((from, to));
            Ok(rows.clone())
        });
    assert!(matches!(result, Err(IndexError::Source(_))));
    assert_eq!(tries, vec![(1, 3); 6]);
    let calls = counts
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(
        calls
            .iter()
            .filter(|(_, call)| call["method"] == "tx_search")
            .count(),
        6
    );
    assert_eq!(store.chain_rows(), before);
}

#[test]
fn backfill_success_resets_the_source_repeat_count() {
    let rows = blockscout();
    let (url, _) = counted_node(TxIndex::Recovering, BatchReply::Complete, &rows);
    let node = ingester(&url);
    let store = RangeStore::new();
    let mut tries = Vec::new();
    let cutover = Backfill::new(&node, 3, 1)
        .with_retry_delay(Duration::ZERO)
        .run(&store.store, |from, to| {
            tries.push((from, to));
            Ok(within(&rows, from, to, &[]))
        })
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(cutover, 3);
    assert_eq!(
        tries,
        (1..=3)
            .flat_map(|height| vec![(height, height); 6])
            .collect::<Vec<_>>()
    );
    let (_, baseline) = domain_baseline();
    assert_eq!(
        store
            .store
            .cursor("paxeer")
            .unwrap_or_else(|error| panic!("{error}"))
            .unwrap_or_else(|| panic!("live cursor"))
            .position,
        3
    );
    baseline
        .store
        .finish_backfill("paxeer", 3, 2)
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(history(&store.store), history(&baseline.store));
}

#[test]
fn backfill_integrity_failed_step_is_tried_once() {
    let mut rows = blockscout();
    let (url, _) = counted_node(TxIndex::On, BatchReply::Complete, &rows);
    let node = ingester(&url);
    rows.blocks[2].hash[31] ^= 1;
    let store = RangeStore::new();
    let before = store.chain_rows();
    let mut tries = 0;
    let result = Backfill::with_options(&node, 3, 3, 2, 2)
        .with_retry_delay(Duration::ZERO)
        .run(&store.store, |from, to| {
            tries += 1;
            assert_eq!((from, to), (1, 3));
            Ok(rows.clone())
        });
    assert!(matches!(result, Err(IndexError::Integrity(_))));
    assert_eq!(tries, 1);
    assert_eq!(store.chain_rows(), before);
}

#[test]
fn backfill_fills_a_height_absent_from_paxscan_from_the_node() {
    let live = live_rows();
    let url = node(TxIndex::On);
    let node = ingester(&url);
    let store = store();
    let rows = within(&blockscout(), 1, 3, &[2]);
    assert!(rows.blocks.iter().all(|block| block.number != 2));
    let plan = Backfill::new(&node, 3, 10);
    let filled = plan
        .apply_range(&store, 1, 3, &rows)
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(filled, 1);
    let cursor = store
        .backfill_cursor("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .unwrap_or_else(|| panic!("backfill cursor"));
    assert_eq!(cursor.position, 3);
    assert!(store
        .cursor("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .is_none());
    store
        .finish_backfill("paxeer", 3, 2)
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(history(&store), live);
}

#[test]
fn backfill_stops_on_a_paxscan_hash_the_node_disagrees_with() {
    let url = node(TxIndex::On);
    let node = ingester(&url);
    let store = store();
    let mut rows = blockscout();
    rows.blocks[2].hash[31] ^= 1;
    let error = Backfill::new(&node, 3, 10)
        .apply_range(&store, 1, 3, &rows)
        .err()
        .unwrap_or_else(|| panic!("a hash mismatch must stop the backfill"));
    assert!(
        matches!(&error, IndexError::Integrity(message) if message.contains("paxscan block 3 hash")),
        "{error}"
    );
    assert!(store
        .backfill_cursor("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .is_none());
    assert!(store
        .transfers("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .is_empty());
}

#[test]
fn backfill_cutover_hands_live_ingestion_the_next_height() {
    let live = live_rows();
    let url = node(TxIndex::On);
    let node = ingester(&url);
    let store = store();
    let rows = blockscout();
    let chain = fixture("paxeer_blocks.json");
    let plan = Backfill::new(&node, 2, 1);
    let mut asked = Vec::new();
    let cutover = plan
        .run(&store, |from, to| {
            asked.push((from, to));
            Ok(within(&rows, from, to, &[]))
        })
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(cutover, 2);
    assert_eq!(asked, vec![(1, 1), (2, 2)]);
    let cursor = store
        .cursor("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .unwrap_or_else(|| panic!("live cursor"));
    assert_eq!(cursor.position, 2);
    assert_eq!(
        cursor.hash,
        chain["canonical"]["blocks"]["2"]["hash"]
            .as_str()
            .unwrap_or_default()
    );
    assert!(plan
        .next_height(&store)
        .unwrap_or_else(|error| panic!("{error}"))
        .is_none());
    assert_eq!(
        node.step(&store).unwrap_or_else(|error| panic!("{error}")),
        StepOutcome::Advanced {
            units: 1,
            position: 3
        }
    );
    assert_eq!(history(&store), live);
    let refused = plan.next_height(&store);
    assert!(
        matches!(&refused, Err(IndexError::Config(message)) if message.contains("past the cutover")),
        "{refused:?}"
    );
    let late = Backfill::new(&node, 2, 1).run(&store, |_, _| {
        panic!("a backfill behind the live cursor must not read paxscan")
    });
    assert!(late.is_err());
}

#[test]
fn backfill_era_node_with_tx_index_off_steps_evm_only() {
    let url = node(TxIndex::Off);
    let live = ingester(&url);
    let store = store();
    for _ in 0..2 {
        let outcome = live.step(&store).unwrap_or_else(|error| panic!("{error}"));
        assert!(matches!(
            outcome,
            StepOutcome::Advanced { .. } | StepOutcome::Idle
        ));
    }
    let cursor = store
        .cursor("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .unwrap_or_else(|| panic!("live cursor"));
    assert_eq!(cursor.position, 3);
    let transfers = store
        .transfers("paxeer")
        .unwrap_or_else(|error| panic!("{error}"));
    let seen = kinds(&transfers);
    for evm in ["erc20_transfer", "native_transfer", "pointer_transfer"] {
        assert!(seen.contains(&evm.to_owned()), "{seen:?}");
    }
    for cosmos in ["bank_transfer", "tokenfactory_mint"] {
        assert!(!seen.contains(&cosmos.to_owned()), "{seen:?}");
    }

    let broken = ingester(&node(TxIndex::Broken));
    let error = broken.step(&self::store());
    assert!(
        matches!(&error, Err(IndexError::Source(message)) if message.contains("tx_search")),
        "{error:?}"
    );
}

#[test]
#[ignore = "reads the live paxscan database; needs PAXSCAN_DATABASE_PUBLIC_URL"]
fn backfill_reads_the_live_paxscan_database() {
    let Ok(url) = std::env::var("PAXSCAN_DATABASE_PUBLIC_URL") else {
        return;
    };
    let tls = std::env::var("LAYERX_INDEXER_PAXSCAN_CERT_SHA256").map_or(
        PaxscanTls::Unauthenticated,
        |pin| {
            let bytes: Vec<u8> = (0..pin.len())
                .step_by(2)
                .filter_map(|index| u8::from_str_radix(pin.get(index..index + 2)?, 16).ok())
                .collect();
            PaxscanTls::PinnedLeaf(
                bytes
                    .try_into()
                    .unwrap_or_else(|_| panic!("the certificate pin must be 32 bytes")),
            )
        },
    );
    let mut database =
        PaxscanDatabase::connect(&url, &tls).unwrap_or_else(|error| panic!("{error}"));
    let rows = database
        .range(1_000_000, 1_000_099)
        .unwrap_or_else(|error| panic!("{error}"));
    let converted = rpc_blocks(&rows).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(converted.len(), rows.blocks.len());
    assert!(converted
        .keys()
        .all(|height| (1_000_000..=1_000_099).contains(height)));
}

#[path = "support/ai_snapshot_fixture.rs"]
pub mod repair_paths;

#[test]
fn tenant_backfill_preserves_chain_id_history_events_and_summary(
) -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::tenant_backfill_preserves_chain_id_history_events_and_summary()?)
}

#[test]
fn daemon_observation_order_allows_disabled_index_evm_fallback(
) -> Result<(), Box<dyn std::error::Error>> {
    Ok(repair_paths::daemon_observation_order_allows_disabled_index_evm_fallback()?)
}
