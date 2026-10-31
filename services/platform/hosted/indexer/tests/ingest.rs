use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use layerx_indexer::abi::AbiRegistry;
use layerx_indexer::api;
use layerx_indexer::codec::hex0x;
use layerx_indexer::follow::{FollowPolicy, StepOutcome};
use layerx_indexer::paxeer::{AttributeEncoding, PaxeerIngester};
use layerx_indexer::store::{AssetRow, Position, Store};
use layerx_indexer::transport::{Endpoint, Security};
use layerx_indexer::IndexError;
use serde_json::{json, Value};

type Handler = dyn Fn(&str, &str, &[u8]) -> (u16, Value) + Send + Sync;

fn fixture(name: &str) -> Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("fixtures")
        .join(name);
    let text = std::fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("{}: {error}", path.display()));
    serde_json::from_str(&text).unwrap_or_else(|error| panic!("{}: {error}", path.display()))
}

fn precompiles() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../precompiles")
}

fn read_request(stream: &mut TcpStream) -> Option<(String, String, Vec<u8>)> {
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
    let mut lines = head.split("\r\n");
    let mut start = lines.next()?.split_whitespace();
    let method = start.next()?.to_owned();
    let target = start.next()?.to_owned();
    let length = lines
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
    Some((
        method,
        target,
        bytes[header_end..header_end + length].to_vec(),
    ))
}

fn serve_fixture(handler: Arc<Handler>) -> String {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("bind: {error}"));
    let address = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("address: {error}"));
    thread::spawn(move || {
        for incoming in listener.incoming() {
            let Ok(mut stream) = incoming else {
                continue;
            };
            let handler = Arc::clone(&handler);
            thread::spawn(move || {
                let Some((method, target, body)) = read_request(&mut stream) else {
                    return;
                };
                let (status, value) = handler(&method, &target, &body);
                let body = value.to_string();
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

fn paxeer_server(state: Arc<Mutex<&'static str>>) -> String {
    let chain = fixture("paxeer_blocks.json");
    let search = fixture("tx_search.json");
    serve_fixture(Arc::new(move |method, _, body| {
        assert_eq!(method, "POST");
        let request: Value = serde_json::from_slice(body).unwrap_or_else(|error| panic!("{error}"));
        let chosen = *state.lock().unwrap_or_else(|error| panic!("{error}"));
        let view = &chain[chosen];
        let params = &request["params"];
        let result = match request["method"].as_str().unwrap_or_default() {
            "eth_chainId" => chain["chain_id"].clone(),
            "eth_blockNumber" => view["head"].clone(),
            "eth_getBlockByNumber" => {
                let quantity = params[0].as_str().unwrap_or_default();
                let height =
                    u64::from_str_radix(quantity.trim_start_matches("0x"), 16).unwrap_or(u64::MAX);
                assert_eq!(params[1], Value::Bool(true));
                view["blocks"]
                    .get(height.to_string())
                    .cloned()
                    .unwrap_or(Value::Null)
            }
            "eth_getTransactionReceipt" => {
                let hash = params[0].as_str().unwrap_or_default();
                chain["receipts"].get(hash).cloned().unwrap_or(Value::Null)
            }
            "tx_search" => {
                let (low, high) = tx_search_range(params["query"].as_str().unwrap_or_default());
                let page: usize = params["page"]
                    .as_str()
                    .and_then(|page| page.parse().ok())
                    .unwrap_or(1);
                let per_page: usize = params["per_page"]
                    .as_str()
                    .and_then(|size| size.parse().ok())
                    .unwrap_or(30);
                let fork_heights: Vec<u64> = if chosen == "reorg" {
                    vec![3]
                } else {
                    Vec::new()
                };
                let matching: Vec<Value> = search["result"]["txs"]
                    .as_array()
                    .map_or(&[][..], Vec::as_slice)
                    .iter()
                    .filter(|tx| {
                        let height: u64 = tx["height"]
                            .as_str()
                            .and_then(|text| text.parse().ok())
                            .unwrap_or(0);
                        (low..=high).contains(&height) && !fork_heights.contains(&height)
                    })
                    .cloned()
                    .collect();
                let total = matching.len();
                let items: Vec<Value> = matching
                    .into_iter()
                    .skip((page - 1) * per_page)
                    .take(per_page)
                    .collect();
                json!({ "txs": items, "total_count": total.to_string() })
            }
            other => panic!("unexpected JSON-RPC method {other}"),
        };
        (
            200,
            json!({ "jsonrpc": "2.0", "id": request["id"], "result": result }),
        )
    }))
}

fn set(state: &Arc<Mutex<&'static str>>, view: &'static str) {
    *state.lock().unwrap_or_else(|error| panic!("{error}")) = view;
}

fn step_ok<F: Fn() -> Result<StepOutcome, IndexError>>(step: F) -> StepOutcome {
    step().unwrap_or_else(|error| panic!("{error}"))
}

fn by(rows: &[Value], field: &str, value: &str) -> Vec<Value> {
    rows.iter()
        .filter(|row| row[field] == value)
        .cloned()
        .collect()
}

fn address_of(label: &str) -> String {
    let chain = fixture("paxeer_blocks.json");
    let block = &chain["canonical"]["blocks"]["2"];
    match label {
        "alice" => block["transactions"][0]["from"]
            .as_str()
            .unwrap_or_default()
            .to_owned(),
        "bob" => block["transactions"][0]["to"]
            .as_str()
            .unwrap_or_default()
            .to_owned(),
        "carol" => block["transactions"][3]["to"]
            .as_str()
            .unwrap_or_default()
            .to_owned(),
        _ => panic!("unknown label"),
    }
}

#[test]
fn paxeer_blocks_logs_and_tx_search_decode_paginate_and_roll_back() {
    let alice = address_of("alice");
    let bob = address_of("bob");
    let carol = address_of("carol");
    let chain = fixture("paxeer_blocks.json");
    let pointer = chain["pointer"]["address"]
        .as_str()
        .unwrap_or_default()
        .to_owned();

    let state = Arc::new(Mutex::new("canonical"));
    let rpc = paxeer_server(Arc::clone(&state));
    let store = Store::open_in_memory().unwrap_or_else(|error| panic!("{error}"));
    store
        .register_assets(&[AssetRow {
            asset: format!("evm:{pointer}"),
            chain: "paxeer".to_owned(),
            kind: "pointer".to_owned(),
            address: Some(pointer.clone()),
            denom: Some("upax".to_owned()),
            metadata: json!({}),
        }])
        .unwrap_or_else(|error| panic!("{error}"));
    let registry = AbiRegistry::load_dir(&precompiles()).unwrap_or_else(|error| panic!("{error}"));
    let ingester = PaxeerIngester::new(
        endpoint(&rpc),
        Some(endpoint(&rpc)),
        registry,
        FollowPolicy {
            finality_depth: 2,
            max_units_per_step: 16,
        },
        1,
        Some(0xe5),
        AttributeEncoding::Base64,
    );

    assert_eq!(
        step_ok(|| ingester.step(&store)),
        StepOutcome::Advanced {
            units: 3,
            position: 3
        }
    );

    let transfers = store
        .transfers("paxeer")
        .unwrap_or_else(|error| panic!("{error}"));
    let native = by(&transfers, "kind", "native_transfer");
    assert_eq!(native.len(), 2);
    assert_eq!(
        by(&native, "direction", "out")[0]["amount"],
        "1000000000000000000"
    );
    assert!(by(&transfers, "account", &carol)
        .iter()
        .all(|row| row["kind"] != "native_transfer"));

    let erc20 = by(&transfers, "kind", "erc20_transfer");
    assert_eq!(erc20.len(), 2);
    let erc20_out = &by(&erc20, "direction", "out")[0];
    assert_eq!(erc20_out["account"], alice.as_str());
    assert_eq!(erc20_out["counterparty"], bob.as_str());
    assert_eq!(erc20_out["amount"], "1000000");
    assert_eq!(
        erc20_out["asset"],
        "evm:0x7b79995e5f793a07bc00c21412e50ecae098e7f9"
    );

    let pointer_legs = by(&transfers, "kind", "pointer_transfer");
    assert_eq!(pointer_legs.len(), 2);
    assert_eq!(
        by(&pointer_legs, "direction", "in")[0]["account"],
        carol.as_str()
    );
    assert_eq!(pointer_legs[0]["amount"], "42");

    assert!(by(&transfers, "kind", "custody_deposit").is_empty());

    let bank = by(&transfers, "kind", "bank_transfer");
    assert_eq!(bank.len(), 4);
    let bank_assets: Vec<&str> = bank
        .iter()
        .filter_map(|row| row["asset"].as_str())
        .collect();
    assert!(bank_assets.contains(&"denom:upax"));
    assert!(bank_assets.contains(&"denom:uatom"));
    assert!(bank
        .iter()
        .all(|row| row["amount"] != "99" && row["amount"] != "11"));
    let mint = by(&transfers, "kind", "tokenfactory_mint");
    assert_eq!(mint.len(), 1);
    assert_eq!(mint[0]["amount"], "5000");

    let events = store
        .events("paxeer")
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(by(&events, "name", "CustodyDeposit").is_empty());
    let checkpoint = by(&events, "name", "layerx_checkpoint_submitted");
    assert_eq!(checkpoint.len(), 1);
    assert_eq!(checkpoint[0]["decoded"]["attributes"]["batch_number"], "3");
    assert_eq!(by(&events, "name", "create_denom").len(), 1);

    let first = api::route(&store, "GET", &format!("/v1/history/{alice}?limit=1"));
    assert_eq!(first.status, 200);
    let first: Value = serde_json::from_str(&first.body).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(first["items"].as_array().map(Vec::len), Some(1));
    assert_eq!(first["indexedHeight"], 3);
    assert_eq!(first["chainHeight"], 3);
    assert_eq!(first["freshness"]["version"], 1);
    assert_eq!(first["freshness"]["database"], "ok");
    let next = first["next_cursor"]
        .as_str()
        .unwrap_or_else(|| panic!("second page cursor"))
        .to_owned();
    assert_eq!(next, "00000000000000020000000000000000");
    assert_eq!(
        Position::parse(&next),
        Some(Position {
            height: 2,
            tx_index: 0,
            log_index: 0
        })
    );
    let second = api::route(
        &store,
        "GET",
        &format!(
            "/v1/history/{}?limit=1&cursor={next}",
            alice.to_uppercase().replacen("0X", "0x", 1)
        ),
    );
    assert_eq!(second.status, 200);
    let second: Value =
        serde_json::from_str(&second.body).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(second["items"].as_array().map(Vec::len), Some(1));
    assert!(second["next_cursor"].is_null());
    assert_eq!(second["indexedHeight"], 3);
    assert_eq!(second["chainHeight"], 3);
    let walked: Vec<(String, (u64, u64, u64))> = first["items"]
        .as_array()
        .into_iter()
        .chain(second["items"].as_array())
        .flatten()
        .map(|item| {
            let index = |field: &str| {
                item[field]
                    .as_str()
                    .and_then(|text| text.parse::<u64>().ok())
                    .unwrap_or_else(|| panic!("{field} in {item}"))
            };
            (
                item["kind"].as_str().unwrap_or_default().to_owned(),
                (
                    index("height_or_seq"),
                    index("tx_index"),
                    index("log_index"),
                ),
            )
        })
        .collect();
    assert_eq!(
        walked,
        [
            ("native_transfer".to_owned(), (2, 0, 0)),
            ("erc20_transfer".to_owned(), (2, 1, 0)),
        ]
    );
    for cursor in [
        "1".to_owned(),
        "000000000000000A0000000100000000".to_owned(),
        "ab".repeat(56),
    ] {
        let refused = api::route(
            &store,
            "GET",
            &format!("/v1/history/{alice}?cursor={cursor}"),
        );
        assert_eq!(refused.status, 400, "{cursor}");
        let refused: Value =
            serde_json::from_str(&refused.body).unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(refused["error"]["code"], "invalid_cursor", "{cursor}");
    }
    let filtered = api::route(
        &store,
        "GET",
        &format!("/v1/history/{alice}?kind=erc20_transfer"),
    );
    let filtered: Value =
        serde_json::from_str(&filtered.body).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(filtered["items"].as_array().map(Vec::len), Some(1));
    assert_eq!(
        api::route(&store, "GET", "/v1/history/x?limit=0").status,
        400
    );
    assert_eq!(
        api::route(&store, "GET", "/v1/history/x?bogus=1").status,
        400
    );
    assert_eq!(api::route(&store, "POST", "/healthz").status, 405);

    let assets = api::route(&store, "GET", "/v1/assets?limit=100");
    let assets: Value =
        serde_json::from_str(&assets.body).unwrap_or_else(|error| panic!("{error}"));
    let listed: Vec<&str> = assets["items"]
        .as_array()
        .map_or(&[][..], Vec::as_slice)
        .iter()
        .filter_map(|asset| asset["asset"].as_str())
        .collect();
    let denom = "denom:factory/pax1qyqszqgpqyqszqgpqyqszqgpqyqszqgp8apuk5/ulx";
    assert!(listed.contains(&denom));
    assert!(listed.contains(&"evm:native"));
    let one = api::route(
        &store,
        "GET",
        &format!("/v1/assets/{}", denom.replace('/', "%2F")),
    );
    assert_eq!(one.status, 200);
    let one: Value = serde_json::from_str(&one.body).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(one["asset"]["kind"], "tokenfactory");
    assert_eq!(one["asset"]["transfer_legs"], "1");
    assert_eq!(
        api::route(&store, "GET", "/v1/assets/denom:none").status,
        404
    );

    let paged = api::route(&store, "GET", "/v1/assets?limit=1");
    let paged: Value = serde_json::from_str(&paged.body).unwrap_or_else(|error| panic!("{error}"));
    let cursor = paged["next_cursor"]
        .as_str()
        .unwrap_or_else(|| panic!("asset cursor"))
        .to_owned();
    let rest = api::route(
        &store,
        "GET",
        &format!("/v1/assets?limit=500&cursor={cursor}"),
    );
    let rest: Value = serde_json::from_str(&rest.body).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(
        rest["items"].as_array().map(Vec::len).unwrap_or_default() + 1,
        listed.len()
    );

    set(&state, "reorg");
    assert_eq!(
        step_ok(|| ingester.step(&store)),
        StepOutcome::RolledBack { fork: Some(2) }
    );
    let transfers = store
        .transfers("paxeer")
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(by(&transfers, "kind", "custody_deposit").is_empty());
    assert!(by(&transfers, "kind", "pointer_transfer").is_empty());
    assert!(by(&transfers, "kind", "tokenfactory_mint").is_empty());
    assert_eq!(by(&transfers, "kind", "erc20_transfer").len(), 2);
    let events = store
        .events("paxeer")
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(by(&events, "name", "CustodyDeposit").is_empty());
    assert!(by(&events, "name", "create_denom").is_empty());
    assert_eq!(
        step_ok(|| ingester.step(&store)),
        StepOutcome::Advanced {
            units: 1,
            position: 3
        }
    );
    let cursor = store
        .cursor("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .unwrap_or_else(|| panic!("cursor"));
    assert_eq!(
        cursor.hash,
        chain["reorg"]["blocks"]["3"]["hash"]
            .as_str()
            .unwrap_or_default()
    );
    let carol_history = store
        .history(&carol, None, 10, None)
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(carol_history.items.is_empty());
}

#[test]
fn a_dropped_in_abi_is_decoded_without_code_changes() {
    let directory = std::env::temp_dir().join(format!("layerx-indexer-abi-{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&directory);
    std::fs::create_dir_all(directory.join("launchpad")).unwrap_or_else(|error| panic!("{error}"));
    std::fs::write(
        directory.join("launchpad/abi.json"),
        json!([{
            "type": "event",
            "name": "TokenLaunched",
            "anonymous": false,
            "inputs": [
                {"name": "token", "type": "address", "indexed": true},
                {"name": "supply", "type": "uint256", "indexed": false}
            ]
        }])
        .to_string(),
    )
    .unwrap_or_else(|error| panic!("{error}"));
    let registry = AbiRegistry::load_dir(&directory).unwrap_or_else(|error| panic!("{error}"));
    let mut emitter = [0_u8; 20];
    emitter[18] = 0x10;
    emitter[19] = 0x17;
    let topic = layerx_indexer::abi::keccak(b"TokenLaunched(address,uint256)");
    let mut token = [0_u8; 32];
    token[31] = 0x42;
    let mut data = [0_u8; 32];
    data[31] = 9;
    let (_, decoded) = registry
        .decode_log(emitter, &[topic, token], &data)
        .unwrap_or_else(|error| panic!("{error}"))
        .unwrap_or_else(|| panic!("dropped-in event must decode"));
    assert_eq!(decoded.contract, "launchpad");
    assert_eq!(decoded.arg("supply"), Some(&json!("9")));
    let _ = std::fs::remove_dir_all(&directory);
}

fn word(value: u128) -> String {
    format!("{value:064x}")
}

fn address_word(address: &str) -> String {
    format!("{:0>64}", address.trim_start_matches("0x"))
}

const LAUNCHPAD: &str = "0x0000000000000000000000000000000000001017";
const POOL_ID: &str = "0x5f1c3a0e9d27b4c86a1e0f3d92b7c54e18a6d0f2c39b74e5a1d806f2c4b9e317";
const TOKEN: &str = "0x4b2f8a91c7e3d5f06a1b9c2e8d7f3a5b6c0e1d24";
const CREATOR: &str = "0xa7dcef9aef26202fce82a7c7d6672afb3a149db2";
const POOL: &str = "0x9c3e71d5a2b84f6e0d1c7a3b5e9f2d8c4a6b1e07";
const OPTICAL: &str = "0x2d6a9e4c1b7f3e8a5d0c6b2f9e4a7d1c3b8e5f60";
const BLOCK_HASH: &str = "0x3e8f1a2b4c5d6e7f8091a2b3c4d5e6f708192a3b4c5d6e7f8091a2b3c4d5e6f7";
const PARENT_HASH: &str = "0x0000000000000000000000000000000000000000000000000000000000000000";
const TX_HASH: &str = "0x7a1d4e9b2c6f3a8e5d0b7c4f1a9e6d3b8c5f2a7e4d1b9c6f3a8e5d2b7c4f1a96";

fn launchpad_v6120_block_and_receipt() -> (Value, Value) {
    let created = layerx_indexer::abi::keccak(
        b"MarketCreated(bytes32,address,address,address,address,uint256,uint256)",
    );
    let swapped = layerx_indexer::abi::keccak(
        b"Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)",
    );
    let log = |index: &str, topics: Vec<String>, data: String| {
        json!({
            "address": LAUNCHPAD,
            "topics": topics,
            "data": data,
            "blockNumber": "0x1",
            "blockHash": BLOCK_HASH,
            "transactionHash": TX_HASH,
            "transactionIndex": "0x0",
            "logIndex": index,
            "removed": false,
        })
    };
    let receipt = json!({
        "transactionHash": TX_HASH,
        "blockHash": BLOCK_HASH,
        "blockNumber": "0x1",
        "transactionIndex": "0x0",
        "from": CREATOR,
        "to": LAUNCHPAD,
        "status": "0x1",
        "gasUsed": "0x5208",
        "cumulativeGasUsed": "0x5208",
        "logs": [
            log(
                "0x0",
                vec![
                    hex0x(&created),
                    POOL_ID.to_owned(),
                    format!("0x{}", address_word(TOKEN)),
                    format!("0x{}", address_word(CREATOR)),
                ],
                format!(
                    "0x{}{}{}{}",
                    address_word(POOL),
                    address_word(OPTICAL),
                    word(1_760_000_000),
                    word(1)
                ),
            ),
            log(
                "0x1",
                vec![
                    hex0x(&swapped),
                    POOL_ID.to_owned(),
                    format!("0x{}", address_word(CREATOR)),
                ],
                format!(
                    "0x{}{}{}{}{}{}{}",
                    word(1),
                    word(5_000_000),
                    word(812_345_678_901_234_567_890),
                    word(50_000),
                    word(6_093),
                    word(1_760_000_000),
                    word(1)
                ),
            ),
        ],
        "type": "0x2",
        "contractAddress": null,
        "effectiveGasPrice": "0x3b9aca00",
    });
    let block = json!({
        "number": "0x1",
        "hash": BLOCK_HASH,
        "parentHash": PARENT_HASH,
        "timestamp": "0x68e8c800",
        "transactions": [{
            "hash": TX_HASH,
            "blockHash": BLOCK_HASH,
            "blockNumber": "0x1",
            "from": CREATOR,
            "to": LAUNCHPAD,
            "value": "0x0",
            "input": "0x",
            "nonce": "0x0",
            "transactionIndex": "0x0",
            "type": "0x2",
            "chainId": "0xe5",
        }],
    });
    (block, receipt)
}

fn launchpad_v6120_server() -> String {
    let (block, receipt) = launchpad_v6120_block_and_receipt();
    serve_fixture(Arc::new(move |method, _, body| {
        assert_eq!(method, "POST");
        let request: Value = serde_json::from_slice(body).unwrap_or_else(|error| panic!("{error}"));
        let params = &request["params"];
        let result = match request["method"].as_str().unwrap_or_default() {
            "eth_chainId" => json!("0xe5"),
            "eth_blockNumber" => json!("0x1"),
            "eth_getBlockByNumber" if params[0] == "0x1" => block.clone(),
            "eth_getTransactionReceipt" if params[0] == TX_HASH => receipt.clone(),
            "eth_getBlockByNumber" | "eth_getTransactionReceipt" => Value::Null,
            other => panic!("unexpected JSON-RPC method {other}"),
        };
        (
            200,
            json!({ "jsonrpc": "2.0", "id": request["id"], "result": result }),
        )
    }))
}

#[test]
fn launchpad_v6120_logs_ingest() {
    let rpc = launchpad_v6120_server();
    let store = Store::open_in_memory().unwrap_or_else(|error| panic!("{error}"));
    let registry = AbiRegistry::load_dir(&precompiles()).unwrap_or_else(|error| panic!("{error}"));
    let ingester = PaxeerIngester::new(
        endpoint(&rpc),
        None,
        registry,
        FollowPolicy {
            finality_depth: 1,
            max_units_per_step: 16,
        },
        1,
        Some(0xe5),
        AttributeEncoding::Base64,
    );

    assert_eq!(
        step_ok(|| ingester.step(&store)),
        StepOutcome::Advanced {
            units: 1,
            position: 1
        }
    );
    assert_eq!(step_ok(|| ingester.step(&store)), StepOutcome::Idle);

    let events = store
        .events("paxeer")
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(by(&events, "name", "undecodable").is_empty(), "{events:?}");
    let launchpad = by(&events, "contract", "launchpad");
    assert_eq!(launchpad.len(), 2);

    let markets = by(&launchpad, "name", "MarketCreated");
    assert_eq!(markets.len(), 1);
    let market = &markets[0];
    assert_eq!(market["source"], "evm-log");
    assert_eq!(market["height_or_seq"], "1");
    assert_eq!(market["tx_id"], TX_HASH);
    assert_eq!(market["ordinal"], "1");
    assert_eq!(market["account"], TOKEN);
    assert_eq!(market["decoded"]["address"], LAUNCHPAD);
    assert_eq!(market["decoded"]["log_index"], "0");
    assert_eq!(
        market["decoded"]["signature"],
        "MarketCreated(bytes32,address,address,address,address,uint256,uint256)"
    );
    assert_eq!(
        market["decoded"]["args"],
        json!({
            "poolId": POOL_ID,
            "token": TOKEN,
            "creator": CREATOR,
            "pool": POOL,
            "optical": OPTICAL,
            "timestamp": "1760000000",
            "blockNumber": "1",
        })
    );

    let swaps = by(&launchpad, "name", "Swap");
    assert_eq!(swaps.len(), 1);
    let swap = &swaps[0];
    assert_eq!(swap["source"], "evm-log");
    assert_eq!(swap["height_or_seq"], "1");
    assert_eq!(swap["tx_id"], TX_HASH);
    assert_eq!(swap["ordinal"], "2");
    assert_eq!(swap["account"], CREATOR);
    assert_eq!(swap["decoded"]["address"], LAUNCHPAD);
    assert_eq!(swap["decoded"]["log_index"], "1");
    assert_eq!(
        swap["decoded"]["signature"],
        "Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)"
    );
    assert_eq!(
        swap["decoded"]["args"],
        json!({
            "poolId": POOL_ID,
            "sender": CREATOR,
            "isBuy": true,
            "amountIn": "5000000",
            "amountOut": "812345678901234567890",
            "fee": "50000",
            "price": "6093",
            "timestamp": "1760000000",
            "blockNumber": "1",
        })
    );
    assert!(store
        .transfers("paxeer")
        .unwrap_or_else(|error| panic!("{error}"))
        .is_empty());
}

#[test]
fn the_api_answers_over_a_real_socket() {
    let store = Arc::new(Store::open_in_memory().unwrap_or_else(|error| panic!("{error}")));
    let listener = api::bind(
        "127.0.0.1:0"
            .parse()
            .unwrap_or_else(|error| panic!("{error}")),
        false,
    )
    .unwrap_or_else(|error| panic!("{error}"));
    let address = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("{error}"));
    let served = Arc::clone(&store);
    thread::spawn(move || api::serve(&listener, &served, None));
    let client = endpoint(&format!("http://{address}"));
    let (status, body) = client
        .get("/healthz")
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(status, 200);
    assert_eq!(
        serde_json::from_slice::<Value>(&body).unwrap_or_default()["status"],
        "ok"
    );
    let page = client
        .get_json("/v1/history/0xabc?limit=5")
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(
        page,
        json!({
            "version": 1, "items": [], "next_cursor": null,
            "indexedHeight": null, "chainHeight": null,
            "freshness": {
                "version": 1, "status": "unavailable", "database": "ok",
                "started_at": 0, "sources": [],
            },
        })
    );
    assert!(api::bind(
        "0.0.0.0:0"
            .parse()
            .unwrap_or_else(|error| panic!("{error}")),
        false
    )
    .is_err());
}
