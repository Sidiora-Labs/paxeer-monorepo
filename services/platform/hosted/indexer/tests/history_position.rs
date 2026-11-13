use std::collections::{BTreeMap, HashSet};
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use layerx_indexer::abi::{keccak, precompile_address, AbiRegistry, LoadedAbi};
use layerx_indexer::api;
use layerx_indexer::codec::hex;
use layerx_indexer::paxeer::{self, AttributeEncoding, BlockInputs};
use layerx_indexer::store::{Position, Store, Unit};
use serde_json::{json, Value};

const A: &str = "0x00000000000000000000000000000000000000a1";
const B: &str = "0x00000000000000000000000000000000000000b2";
const C: &str = "0x00000000000000000000000000000000000000c3";
const TOKEN: &str = "0x00000000000000000000000000000000000000d4";
const LEDGER: &str = "0x000000000000000000000000000000000000101a";
const PAY: &str = "0x000000000000000000000000000000000000101b";
const AUTH: &str = "0x000000000000000000000000000000000000101c";

fn word(value: u64) -> String {
    format!("{value:064x}")
}

fn hash(tag: u64) -> String {
    format!("0x{tag:064x}")
}

fn padded(address: &str) -> String {
    format!("0x{:0>64}", &address[2..])
}

fn topic(signature: &str) -> String {
    format!("0x{}", hex(&keccak(signature.as_bytes())))
}

fn id(tag: u64) -> String {
    format!("0x{:064x}", 0xb0d6_0000_u64 + tag)
}

/// One log of transaction `tx_index` at `log_index` in its block.
fn log(address: &str, log_index: u64, topics: &[String], data: &str) -> Value {
    json!({
        "address": address,
        "logIndex": format!("0x{log_index:x}"),
        "topics": topics,
        "data": format!("0x{data}"),
        "removed": false,
    })
}

fn transfer_log(log_index: u64, from: &str, to: &str, amount: u64) -> Value {
    log(
        TOKEN,
        log_index,
        &[
            topic("Transfer(address,address,uint256)"),
            padded(from),
            padded(to),
        ],
        &word(amount),
    )
}

/// A transaction from `from` to `to` carrying `value` wei and `logs`.
struct Tx {
    from: &'static str,
    to: &'static str,
    value: u64,
    logs: Vec<Value>,
}

/// Decodes and commits one block of `transactions` through the Paxeer
/// block decoder.
fn commit_block(store: &Store, height: u64, transactions: Vec<Tx>) {
    let block_hash = hash(0xb10c_0000_0000 + height);
    let mut objects = Vec::new();
    let mut receipts = BTreeMap::new();
    for (index, tx) in transactions.into_iter().enumerate() {
        let index = index as u64;
        let tx_hash = hash((height << 16) + index);
        objects.push(json!({
            "hash": tx_hash,
            "from": tx.from,
            "to": tx.to,
            "value": format!("0x{:x}", tx.value),
            "transactionIndex": format!("0x{index:x}"),
        }));
        let logs: Vec<Value> = tx
            .logs
            .into_iter()
            .map(|mut entry| {
                entry["transactionIndex"] = json!(format!("0x{index:x}"));
                entry
            })
            .collect();
        receipts.insert(
            tx_hash,
            json!({ "blockHash": block_hash, "status": "0x1", "logs": logs }),
        );
    }
    let block = json!({
        "number": format!("0x{height:x}"),
        "hash": block_hash,
        "parentHash": hash(0xb10c_0000_0000 + height - 1),
        "transactions": objects,
    });
    let registry = AbiRegistry::default();
    let is_pointer = |_: &str| Ok(false);
    let inputs = BlockInputs {
        registry: &registry,
        receipts: &receipts,
        cosmos_txs: &[],
        encoding: AttributeEncoding::Base64,
        is_pointer: &is_pointer,
    };
    let unit = paxeer::decode_block(&block, &inputs).unwrap_or_else(|error| panic!("{error}"));
    store
        .commit(&unit, 2)
        .unwrap_or_else(|error| panic!("{error}"));
}

fn get(store: &Store, target: &str) -> (u16, Value) {
    let response = api::route(store, "GET", target);
    let body = serde_json::from_str(&response.body).unwrap_or_else(|error| panic!("{error}"));
    (response.status, body)
}

fn items(page: &Value) -> Vec<Value> {
    page["items"]
        .as_array()
        .unwrap_or_else(|| panic!("no items in {page}"))
        .clone()
}

fn triple(item: &Value) -> (u64, u32, u32) {
    let field = |name: &str| {
        item[name]
            .as_str()
            .and_then(|text| text.parse::<u64>().ok())
            .unwrap_or_else(|| panic!("{name} missing in {item}"))
    };
    (
        field("height_or_seq"),
        u32::try_from(field("tx_index")).unwrap_or(u32::MAX),
        u32::try_from(field("log_index")).unwrap_or(u32::MAX),
    )
}

fn store() -> Store {
    Store::open_in_memory().unwrap_or_else(|error| panic!("{error}"))
}

#[test]
fn history_orders_by_block_position_and_pages_strictly_after_the_cursor() {
    let store = store();
    commit_block(
        &store,
        10,
        vec![Tx {
            from: A,
            to: B,
            value: 7,
            logs: Vec::new(),
        }],
    );
    commit_block(
        &store,
        12,
        vec![Tx {
            from: C,
            to: TOKEN,
            value: 0,
            logs: vec![transfer_log(3, C, A, 6), transfer_log(0, C, A, 5)],
        }],
    );

    let (status, first) = get(&store, &format!("/v1/history/{A}?limit=2"));
    assert_eq!(status, 200);
    let page = items(&first);
    assert_eq!(page.len(), 2);
    assert_eq!(triple(&page[0]), (10, 0, 0));
    assert_eq!(page[0]["kind"], "native_transfer");
    assert_eq!(page[0]["direction"], "out");
    assert_eq!(page[0]["amount"], "7");
    assert_eq!(triple(&page[1]), (12, 0, 0));
    assert_eq!(page[1]["kind"], "erc20_transfer");
    assert_eq!(page[1]["amount"], "5");
    let cursor = first["next_cursor"]
        .as_str()
        .unwrap_or_else(|| panic!("no cursor"))
        .to_owned();
    assert_eq!(cursor, "000000000000000c0000000000000000");
    assert_eq!(
        Position::parse(&cursor),
        Some(Position {
            height: 12,
            tx_index: 0,
            log_index: 0
        })
    );

    let (status, second) = get(&store, &format!("/v1/history/{A}?limit=2&cursor={cursor}"));
    assert_eq!(status, 200);
    let page = items(&second);
    assert_eq!(page.len(), 1);
    assert_eq!(triple(&page[0]), (12, 0, 3));
    assert_eq!(page[0]["amount"], "6");
    assert_eq!(second["next_cursor"], Value::Null);
}

#[test]
fn rows_in_one_block_page_in_triple_order_without_repeat_or_gap() {
    let store = store();
    commit_block(
        &store,
        40,
        vec![
            Tx {
                from: A,
                to: TOKEN,
                value: 3,
                logs: vec![transfer_log(0, A, B, 11), transfer_log(1, C, A, 12)],
            },
            Tx {
                from: A,
                to: A,
                value: 9,
                logs: Vec::new(),
            },
            Tx {
                from: B,
                to: TOKEN,
                value: 0,
                logs: vec![
                    transfer_log(2, B, A, 13),
                    transfer_log(3, A, A, 14),
                    transfer_log(4, B, C, 15),
                ],
            },
            Tx {
                from: C,
                to: A,
                value: 4,
                logs: vec![transfer_log(5, C, A, 16)],
            },
        ],
    );

    let all = store
        .history(A, None, 500, None)
        .unwrap_or_else(|error| panic!("{error}"));
    let expected: Vec<((u64, u32, u32), String)> = all
        .items
        .iter()
        .map(|item| {
            (
                triple(item),
                item["direction"].as_str().unwrap_or_default().to_owned(),
            )
        })
        .collect();
    assert_eq!(
        expected,
        vec![
            ((40, 0, 0), "out".to_owned()),
            ((40, 0, 0), "out".to_owned()),
            ((40, 0, 1), "in".to_owned()),
            ((40, 1, 0), "out".to_owned()),
            ((40, 1, 0), "in".to_owned()),
            ((40, 2, 2), "in".to_owned()),
            ((40, 2, 3), "out".to_owned()),
            ((40, 2, 3), "in".to_owned()),
            ((40, 3, 0), "in".to_owned()),
            ((40, 3, 5), "in".to_owned()),
        ]
    );
    assert_eq!(all.items[0]["kind"], "native_transfer");
    assert_eq!(all.items[1]["kind"], "erc20_transfer");
    assert_eq!(all.next_cursor, None);

    for limit in 1..=4 {
        let mut seen = Vec::new();
        let mut cursor: Option<String> = None;
        let mut previous_last: Option<(u64, u32, u32)> = None;
        loop {
            let target = cursor.as_ref().map_or_else(
                || format!("/v1/history/{A}?limit={limit}"),
                |cursor| format!("/v1/history/{A}?limit={limit}&cursor={cursor}"),
            );
            let (status, page) = get(&store, &target);
            assert_eq!(status, 200);
            let rows = items(&page);
            assert!(!rows.is_empty(), "limit {limit} served an empty page");
            if let Some(last) = previous_last {
                assert!(triple(&rows[0]) > last, "limit {limit} repeated a position");
            }
            previous_last = rows.last().map(triple);
            seen.extend(rows.iter().cloned());
            match page["next_cursor"].as_str() {
                Some(next) => {
                    let position =
                        Position::parse(next).unwrap_or_else(|| panic!("bad cursor {next}"));
                    let last = previous_last.unwrap_or_default();
                    assert_eq!(
                        (position.height, position.tx_index, position.log_index),
                        last
                    );
                    cursor = Some(next.to_owned());
                }
                None => break,
            }
        }
        assert_eq!(seen, all.items, "limit {limit} paged differently");
    }
}

#[test]
fn a_page_of_fifty_reads_one_hundred_twenty_transactions_in_three_pages() {
    let store = store();
    commit_block(
        &store,
        50,
        (0..120)
            .map(|index| Tx {
                from: A,
                to: B,
                value: index + 1,
                logs: Vec::new(),
            })
            .collect(),
    );
    let mut cursor: Option<String> = None;
    let mut sizes = Vec::new();
    let mut transactions = HashSet::new();
    loop {
        let target = cursor.as_ref().map_or_else(
            || format!("/v1/history/{A}?limit=50"),
            |cursor| format!("/v1/history/{A}?limit=50&cursor={cursor}"),
        );
        let (status, page) = get(&store, &target);
        assert_eq!(status, 200);
        let rows = items(&page);
        sizes.push(rows.len());
        for row in &rows {
            assert!(transactions.insert(row["tx_id"].as_str().unwrap_or_default().to_owned()));
        }
        match page["next_cursor"].as_str() {
            Some(next) => cursor = Some(next.to_owned()),
            None => break,
        }
    }
    assert_eq!(sizes, vec![50, 50, 20]);
    assert_eq!(transactions.len(), 120);
}

#[test]
fn any_other_cursor_form_is_refused_with_invalid_cursor() {
    let store = store();
    commit_block(
        &store,
        10,
        vec![Tx {
            from: A,
            to: B,
            value: 7,
            logs: Vec::new(),
        }],
    );
    let major_one = "ab".repeat(56);
    assert_eq!(major_one.len(), 112);
    for cursor in [
        major_one.as_str(),
        "000000000000000C0000000000000000",
        "000000000000000c000000000000000",
        "000000000000000c00000000000000000",
        "0x0000000000000c0000000000000000",
        "8000000000000000000000000000000000",
        "80000000000000000000000000000000",
        "5",
        "",
    ] {
        for route in ["history", "holders"] {
            let (status, body) = get(&store, &format!("/v1/{route}/{A}?cursor={cursor}"));
            assert_eq!(status, 400, "{route} accepted cursor {cursor:?}");
            assert_eq!(
                body["error"]["code"], "invalid_cursor",
                "{route} {cursor:?}"
            );
        }
    }
    let (status, body) = get(
        &store,
        &format!("/v1/history/{A}?cursor=00000000000000090000000000000000"),
    );
    assert_eq!(status, 200);
    assert_eq!(items(&body).len(), 1);
    let (status, body) = get(
        &store,
        &format!("/v1/history/{A}?cursor=000000000000000a0000000000000000"),
    );
    assert_eq!(status, 200);
    assert!(items(&body).is_empty());
}

fn budget_created(log_index: u64, budget: u64) -> Value {
    log(
        PAY,
        log_index,
        &[topic("BudgetCreated(bytes32,bytes32,uint128)"), id(budget)],
        &format!("{}{}", word(0xa55e7), word(500)),
    )
}

fn budget_ended(event: &str, log_index: u64, budget: u64) -> Value {
    log(
        PAY,
        log_index,
        &[
            topic(&format!("{event}(bytes32,uint128,uint64)")),
            id(budget),
        ],
        &format!("{}{}", word(120), word(9)),
    )
}

fn grant_body(log_index: u64, grant: u64, chunk: u64) -> Value {
    log(
        AUTH,
        log_index,
        &[topic("GrantBody(bytes32,uint8,uint8,bytes)"), id(grant)],
        &format!(
            "{}{}{}{}{:0<64}",
            word(1),
            word(chunk),
            word(0x60),
            word(2),
            "beef"
        ),
    )
}

fn commit_holder_fixture(store: &Store) {
    commit_block(
        store,
        20,
        vec![
            Tx {
                from: A,
                to: PAY,
                value: 0,
                logs: vec![budget_created(0, 1), budget_created(1, 2)],
            },
            Tx {
                from: A,
                to: LEDGER,
                value: 0,
                logs: vec![log(
                    LEDGER,
                    2,
                    &[topic("PayerGrantIssued(bytes32)"), id(3)],
                    "",
                )],
            },
            Tx {
                from: A,
                to: AUTH,
                value: 0,
                logs: vec![grant_body(3, 4, 0), grant_body(4, 4, 1)],
            },
            Tx {
                from: B,
                to: PAY,
                value: 0,
                logs: vec![budget_created(5, 5)],
            },
        ],
    );
    commit_block(
        store,
        21,
        vec![Tx {
            from: A,
            to: PAY,
            value: 0,
            logs: vec![budget_ended("BudgetRevoked", 0, 1)],
        }],
    );
    commit_block(
        store,
        22,
        vec![Tx {
            from: A,
            to: AUTH,
            value: 0,
            logs: vec![log(
                AUTH,
                0,
                &[topic("GrantRevoked(bytes32,bytes)"), id(4)],
                &format!("{}{}{:0<64}", word(0x20), word(1), "01"),
            )],
        }],
    );
    store
        .record_source_head("paxeer", Some(30))
        .unwrap_or_else(|error| panic!("{error}"));
}

#[test]
fn holder_records_carry_their_latest_state_by_holder() {
    let store = store();
    commit_holder_fixture(&store);

    let (status, budgets) = get(&store, &format!("/v1/holders/{A}?kind=budget"));
    assert_eq!(status, 200);
    let records = items(&budgets);
    assert_eq!(records.len(), 2);
    let bare = |tag: u64| id(tag)[2..].to_owned();
    assert_eq!(records[0]["record_id"], bare(1));
    assert_eq!(records[0]["kind"], "budget");
    assert_eq!(records[0]["contract"], "layerxpay");
    assert_eq!(records[0]["holder"], A);
    assert_eq!(records[0]["state"], "revoked");
    assert_eq!(records[0]["opened"]["event"], "BudgetCreated");
    assert_eq!(records[0]["opened"]["height"], "20");
    assert_eq!(records[0]["opened"]["decoded"]["args"]["amount"], "500");
    assert_eq!(records[0]["latest"]["event"], "BudgetRevoked");
    assert_eq!(records[0]["latest"]["height"], "21");
    assert_eq!(records[0]["latest"]["decoded"]["args"]["drained"], "120");
    assert_eq!(records[1]["record_id"], bare(2));
    assert_eq!(records[1]["state"], "active");
    assert_eq!(budgets["next_cursor"], Value::Null);
    assert_eq!(budgets["indexedHeight"], 22);
    assert_eq!(budgets["chainHeight"], 30);

    let (status, first) = get(&store, &format!("/v1/holders/{A}?kind=budget&limit=1"));
    assert_eq!(status, 200);
    assert_eq!(items(&first).len(), 1);
    assert_eq!(first["items"][0]["record_id"], bare(1));
    let cursor = first["next_cursor"]
        .as_str()
        .unwrap_or_else(|| panic!("no cursor"))
        .to_owned();
    assert_eq!(cursor, "00000000000000140000000000000000");
    let (status, second) = get(
        &store,
        &format!("/v1/holders/{A}?kind=budget&limit=1&cursor={cursor}"),
    );
    assert_eq!(status, 200);
    assert_eq!(items(&second).len(), 1);
    assert_eq!(second["items"][0]["record_id"], bare(2));
    assert_eq!(second["next_cursor"], Value::Null);

    let (status, grants) = get(&store, &format!("/v1/holders/{A}?kind=grant"));
    assert_eq!(status, 200);
    let records = items(&grants);
    let summary: Vec<(String, String, String)> = records
        .iter()
        .map(|record| {
            (
                record["contract"].as_str().unwrap_or_default().to_owned(),
                record["record_id"].as_str().unwrap_or_default().to_owned(),
                record["state"].as_str().unwrap_or_default().to_owned(),
            )
        })
        .collect();
    assert_eq!(
        summary,
        vec![
            ("layerxledger".to_owned(), bare(3), "active".to_owned()),
            ("layerxauth".to_owned(), bare(4), "revoked".to_owned()),
        ]
    );
    assert_eq!(records[1]["opened"]["event"], "GrantBody");
    assert_eq!(records[1]["latest"]["event"], "GrantRevoked");

    let (status, all) = get(
        &store,
        &format!("/v1/holders/{}", A.to_uppercase().replacen("0X", "0x", 1)),
    );
    assert_eq!(status, 200);
    assert_eq!(items(&all).len(), 4);
    let (status, other) = get(&store, &format!("/v1/holders/{B}"));
    assert_eq!(status, 200);
    assert_eq!(items(&other).len(), 1);
    assert_eq!(other["items"][0]["record_id"], bare(5));

    let (status, body) = get(&store, &format!("/v1/holders/{A}?kind=native_transfer"));
    assert_eq!(status, 400);
    assert_eq!(body["error"]["code"], "invalid_kind");
}

#[test]
fn freshness_carries_the_indexed_and_chain_heights() {
    let store = store();
    commit_block(
        &store,
        990,
        vec![Tx {
            from: A,
            to: B,
            value: 1,
            logs: Vec::new(),
        }],
    );
    store
        .record_source_head("paxeer", Some(1000))
        .unwrap_or_else(|error| panic!("{error}"));
    for target in [format!("/v1/history/{A}"), format!("/v1/holders/{A}")] {
        let (status, page) = get(&store, &target);
        assert_eq!(status, 200, "{target}");
        assert_eq!(page["indexedHeight"], 990, "{target}");
        assert_eq!(page["chainHeight"], 1000, "{target}");
        assert_eq!(page["version"], 1, "{target}");
        assert!(page["freshness"].is_object(), "{target}");
    }
}

struct ProofScratch(PathBuf);
impl Drop for ProofScratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn load_auth_proof() -> (ProofScratch, Value) {
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"))
        .as_nanos();
    let scratch = ProofScratch(
        std::env::temp_dir().join(format!("layerxauth-proof-{}-{nonce}", std::process::id())),
    );
    std::fs::create_dir(&scratch.0)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    let input = if let Some(path) = std::env::var_os("LAYERX_AUTH_PROOF_INPUT") {
        PathBuf::from(path)
    } else {
        let path = scratch.0.join("evm-receipts.json");
        let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"));
        let output = Command::new("go")
            .current_dir(root)
            .args([
                "test",
                "-count=1",
                "./precompiles/layerxauth/",
                "-run",
                "^TestProofLayerxauth0x101c$",
            ])
            .env("LAYERX_AUTH_PROOF_INPUT", &path)
            .output()
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
        assert!(
            output.status.success(),
            "real EVM producer: {}{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        path
    };
    let proof: Value = serde_json::from_slice(
        &std::fs::read(input)
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    (scratch, proof)
}

fn decode_auth_block(
    proof: &Value,
    registry: &AbiRegistry,
    holder: &str,
    height: u64,
    receipt_key: &str,
    tx_key: &str,
) -> Unit {
    let tx_hash = proof[tx_key]
        .as_str()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
        .to_owned();
    let block_hash = hash(0xb10c_0000_0000 + height);
    let block = json!({
        "number": format!("0x{height:x}"), "hash": block_hash,
        "parentHash": hash(0xb10c_0000_0000 + height - 1),
        "transactions": [{"hash": tx_hash, "from": holder, "to": AUTH,
            "value": "0x0", "transactionIndex": "0x0"}],
    });
    let mut receipt = proof[receipt_key].clone();
    receipt["blockHash"] = json!(block_hash);
    let receipts = BTreeMap::from([(tx_hash, receipt)]);
    let is_pointer = |_: &str| Ok(false);
    paxeer::decode_block(
        &block,
        &BlockInputs {
            registry,
            receipts: &receipts,
            cosmos_txs: &[],
            encoding: AttributeEncoding::Base64,
            is_pointer: &is_pointer,
        },
    )
    .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"))
}

fn assert_auth_grant_body(proof: &Value, registry: &AbiRegistry) {
    let mut body = String::from("0x");
    for log in proof["granted"]["logs"]
        .as_array()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
    {
        if log["topics"][0] == topic("GrantBody(bytes32,uint8,uint8,bytes)") {
            let topics = log["topics"]
                .as_array()
                .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
                .iter()
                .map(|value| {
                    layerx_indexer::codec::unhex_fixed::<32>(
                        value
                            .as_str()
                            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value")),
                    )
                    .unwrap_or_else(|error| {
                        panic!("called Result::unwrap() on an Err value: {error:?}")
                    })
                })
                .collect::<Vec<_>>();
            let data = layerx_indexer::codec::unhex(
                log["data"]
                    .as_str()
                    .unwrap_or_else(|| panic!("called Option::unwrap() on a None value")),
            )
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
            let (_, event) = registry
                .decode_log(precompile_address(0x101c), &topics, &data)
                .unwrap_or_else(|error| {
                    panic!("called Result::unwrap() on an Err value: {error:?}")
                })
                .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"));
            body.push_str(
                event
                    .arg("body")
                    .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
                    .as_str()
                    .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
                    .trim_start_matches("0x"),
            );
        }
    }
    assert_eq!(body, proof["body"]);
}

/// Proves real App.DeliverTx receipts from 0x101C survive ABI decoding,
/// holder projection, `SQLite` restart and replay before the holder API reads them.
/// The Go producer is also callable when this consumer test is run on its own.
#[test]
fn proof_layerxauth_0x101c_receipts() {
    let (scratch, proof) = load_auth_proof();
    let holder = proof["holder"]
        .as_str()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
        .to_ascii_lowercase();
    let grant_id = proof["grantId"]
        .as_str()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
        .trim_start_matches("0x")
        .to_owned();
    let mut registry = AbiRegistry::default();
    registry.insert(LoadedAbi {
        contract: "layerxauth".to_owned(),
        address: Some(precompile_address(0x101c)),
        events: AbiRegistry::parse_events("layerxauth", &proof["abi"])
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}")),
    });
    let opened = decode_auth_block(&proof, &registry, &holder, 11, "granted", "grantTx");
    assert_eq!(
        opened.holders.len(),
        1,
        "body chunks must open exactly one holder record"
    );
    let target = format!("/v1/holders/{holder}?kind=grant");
    let path = scratch.0.join("index.sqlite");
    let store = Store::open(&path)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    store
        .commit(&opened, 2)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    let (status, active) = get(&store, &target);
    assert_eq!(status, 200);
    let rows = items(&active);
    assert_eq!(rows.len(), 1);
    assert_eq!(rows[0]["holder"], holder);
    assert_eq!(rows[0]["contract"], "layerxauth");
    assert_eq!(rows[0]["record_id"], grant_id);
    assert_eq!(rows[0]["state"], "active");
    assert_eq!(rows[0]["opened"]["event"], "GrantBody");
    assert_eq!(rows[0]["opened"]["decoded"]["args"]["sub"], "5");
    assert_eq!(rows[0]["opened"]["decoded"]["args"]["chunk"], "0");
    assert_auth_grant_body(&proof, &registry);
    drop(store);
    let store = Store::open(&path)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    assert_eq!(get(&store, &target), (status, active.clone()));
    assert!(
        store.commit(&opened, 2).is_err(),
        "duplicate opening unit must be refused"
    );
    assert_eq!(get(&store, &target), (status, active));

    let refused = decode_auth_block(&proof, &registry, &holder, 12, "refused", "refusedTx");
    assert!(
        refused.holders.is_empty(),
        "a refused signer must produce no holder row"
    );
    store
        .commit(&refused, 2)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    assert_eq!(items(&get(&store, &target).1), rows);
    let revoked = decode_auth_block(&proof, &registry, &holder, 13, "revoked", "revokeTx");
    assert_eq!(revoked.holders.len(), 1);
    store
        .commit(&revoked, 2)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    let (status, ended) = get(&store, &target);
    assert_eq!(status, 200);
    let records = items(&ended);
    assert_eq!(records.len(), 1);
    assert_eq!(records[0]["record_id"], grant_id);
    assert_eq!(records[0]["state"], "revoked");
    assert_eq!(records[0]["opened"], rows[0]["opened"]);
    assert_eq!(records[0]["latest"]["event"], "GrantRevoked");
    assert_eq!(
        records[0]["latest"]["decoded"]["args"]["record"],
        proof["record"]
    );
    drop(store);
    let store = Store::open(&path)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    assert_eq!(get(&store, &target), (status, ended.clone()));
    assert!(
        store.commit(&revoked, 2).is_err(),
        "duplicate revocation unit must be refused"
    );
    assert_eq!(get(&store, &target), (status, ended));
    let (status, bad_cursor) = get(&store, &format!("{target}&cursor=invalid"));
    assert_eq!(status, 400);
    assert_eq!(bad_cursor["error"]["code"], "invalid_cursor");
}

/// Consumes only logs emitted by the Go application proof; run via its declared gate.
#[test]
fn proof_layerxpay_0x101b() {
    let document: Value =
        serde_json::from_reader(std::io::stdin().lock()).unwrap_or_else(|error| {
            panic!("run TestProofLayerxpay0x101b with producer bytes on stdin: {error}")
        });
    let abi: Value = serde_json::from_slice(
        &std::fs::read(document["abi"].as_str().unwrap_or_default())
            .unwrap_or_else(|error| panic!("{error}")),
    )
    .unwrap_or_else(|error| panic!("{error}"));
    let mut registry = AbiRegistry::default();
    registry.insert(LoadedAbi {
        contract: "layerxpay".to_owned(),
        address: Some(precompile_address(0x101b)),
        events: AbiRegistry::parse_events("layerxpay", &abi)
            .unwrap_or_else(|error| panic!("{error}")),
    });
    let path = std::path::Path::new(document["store"].as_str().unwrap_or_default());
    let owner = document["owner"].as_str().unwrap_or_default();
    let target = format!("/v1/holders/{owner}?kind=budget");
    let is_pointer = |_: &str| Ok(false);
    let mut units = Vec::new();
    for envelope in document["blocks"]
        .as_array()
        .unwrap_or_else(|| panic!("blocks"))
    {
        let receipts: BTreeMap<String, Value> =
            serde_json::from_value(envelope["receipts"].clone())
                .unwrap_or_else(|error| panic!("{error}"));
        let inputs = BlockInputs {
            registry: &registry,
            receipts: &receipts,
            cosmos_txs: &[],
            encoding: AttributeEncoding::Base64,
            is_pointer: &is_pointer,
        };
        let unit = paxeer::decode_block(&envelope["block"], &inputs)
            .unwrap_or_else(|error| panic!("{error}"));
        let store = Store::open(path).unwrap_or_else(|error| panic!("{error}"));
        store
            .commit(&unit, 2)
            .unwrap_or_else(|error| panic!("{error}"));
        units.push(unit);
        drop(store);
        // Close every connection before querying the durable projection twice.
        for _ in 0..2 {
            let store = Store::open(path).unwrap_or_else(|error| panic!("{error}"));
            let (status, page) = get(&store, &target);
            assert_eq!(status, 200);
            let rows = items(&page);
            assert_eq!(rows.len(), 1);
            assert_eq!(rows[0]["record_id"], document["budget"]);
            assert_eq!(rows[0]["opened"]["event"], "BudgetCreated");
            assert_eq!(rows[0]["opened"]["decoded"]["args"]["amount"], "200");
            assert_eq!(
                rows[0]["opened"]["decoded"]["args"]["assetId"],
                document["asset"]
            );
            if units.len() == 1 {
                assert_eq!(rows[0]["state"], "active");
            } else {
                assert_eq!(rows[0]["state"], "revoked");
                assert_eq!(rows[0]["latest"]["event"], "BudgetRevoked");
                assert_eq!(rows[0]["latest"]["decoded"]["args"]["drained"], "200");
                assert_eq!(
                    rows[0]["latest"]["decoded"]["args"]["revocationSequence"],
                    "1"
                );
            }
        }
    }
    assert_eq!(units.len(), 2);
    let first = &document["blocks"][0];
    let mut receipts: BTreeMap<String, Value> =
        serde_json::from_value(first["receipts"].clone()).unwrap_or_else(|error| panic!("{error}"));
    let receipt = receipts
        .values_mut()
        .next()
        .unwrap_or_else(|| panic!("receipt"));
    let emitted = receipt["logs"][0]["data"].as_str().unwrap_or_default();
    receipt["logs"][0]["data"] = json!(&emitted[..emitted.len() - 2]);
    let inputs = BlockInputs {
        registry: &registry,
        receipts: &receipts,
        cosmos_txs: &[],
        encoding: AttributeEncoding::Base64,
        is_pointer: &is_pointer,
    };
    // paxeer::decode_log quarantines invalid producer bytes as undecodable.
    let malformed =
        paxeer::decode_block(&first["block"], &inputs).unwrap_or_else(|error| panic!("{error}"));
    assert!(malformed.holders.is_empty());
    assert!(malformed
        .events
        .iter()
        .any(|event| event.name == "undecodable"));
}

#[test]
fn layerxledger_producer_abi_preserves_the_history_decoder_inventory() {
    let document: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../precompiles/layerxledger/abi.json"
    )))
    .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    let events = AbiRegistry::parse_events("layerxledger", &document)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    for (name, signature, indexed) in [
        ("PayerGrantIssued", "PayerGrantIssued(bytes32)", 1),
        (
            "PaymentDrawn",
            "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
            3,
        ),
        (
            "PaymentExact",
            "PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)",
            3,
        ),
        (
            "Transfer",
            "Transfer(bytes32,bytes32,bytes32,uint128,uint8)",
            3,
        ),
    ] {
        let event = events
            .iter()
            .find(|event| event.name == name)
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"));
        assert_eq!(event.signature, signature);
        assert_eq!(event.topic, keccak(signature.as_bytes()));
        assert_eq!(
            event.params.iter().filter(|param| param.indexed).count(),
            indexed
        );
    }
    // Identifier-returning writes must retain their return type when this
    // published producer ABI is loaded by clients alongside the history ABI.
    for name in ["register", "openAccount", "issueGrant"] {
        let method = document
            .as_array()
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
            .iter()
            .find(|entry| entry["type"] == "function" && entry["name"] == name)
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"));
        assert_eq!(
            method["outputs"]
                .as_array()
                .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
                .len(),
            1
        );
        assert_eq!(method["outputs"][0]["type"], "bytes32");
    }
}

fn assert_payment_events(store: &Store, proof: &Value, target: &str) -> (u16, Value, Vec<Value>) {
    let events = store
        .events("paxeer")
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    assert_eq!(events.len(), 2);
    let exact = events
        .iter()
        .find(|event| event["name"] == "PaymentExact")
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"));
    assert_eq!(exact["decoded"]["args"]["amount"], "15");
    assert_eq!(
        exact["decoded"]["args"]["paymentReference"],
        proof["reference"]
    );
    assert_eq!(exact["decoded"]["args"]["from"], proof["from"]);
    assert_eq!(exact["decoded"]["args"]["to"], proof["to"]);
    assert_eq!(exact["decoded"]["args"]["assetId"], proof["asset"]);
    assert!(events.iter().any(|event| event["name"] == "Transfer"));
    let (status, page) = get(store, target);
    assert_eq!(status, 200);
    let rows = items(&page);
    // Retired issuance emitted no grant and cannot create an active holder.
    assert!(proof["grantLogs"]
        .as_array()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
        .is_empty());
    assert!(rows.is_empty());
    assert!(events
        .iter()
        .all(|event| event["name"] != "PayerGrantIssued"));
    (status, page, events)
}

fn commit_ledger_units(
    proof: &Value,
    registry: &AbiRegistry,
    owner: &str,
    path: &std::path::Path,
) -> Vec<Unit> {
    let mut units = Vec::new();
    for (offset, key) in ["paymentLogs", "grantLogs", "refusedLogs"]
        .iter()
        .enumerate()
    {
        let height = 11 + offset as u64;
        let tx_hash = hash(height);
        let mut logs = proof[*key]
            .as_array()
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
            .clone();
        for (index, log) in logs.iter_mut().enumerate() {
            log["logIndex"] = json!(format!("0x{index:x}"));
            log["transactionIndex"] = json!("0x0");
        }
        let receipts = BTreeMap::from([(
            tx_hash.clone(),
            json!({
                "status": if *key == "paymentLogs" { "0x1" } else { "0x0" },
                "logs": logs, "blockHash": hash(height + 100),
            }),
        )]);
        let block = json!({
            "number": format!("0x{height:x}"), "hash": hash(height + 100),
            "parentHash": hash(height + 99),
            "transactions": [{ "hash": tx_hash, "from": owner, "to": LEDGER,
                "value": "0x0", "transactionIndex": "0x0" }],
        });
        let is_pointer = |_: &str| Ok(false);
        let unit = paxeer::decode_block(
            &block,
            &BlockInputs {
                registry,
                receipts: &receipts,
                cosmos_txs: &[],
                encoding: AttributeEncoding::Base64,
                is_pointer: &is_pointer,
            },
        )
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
        if *key != "paymentLogs" {
            assert!(unit.events.is_empty());
            assert!(unit.holders.is_empty());
        }
        let store = Store::open(path)
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
        store
            .commit(&unit, 2)
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
        units.push(unit);
        drop(store);
    }
    units
}

/// Consumes application-produced logs over stdin, then restarts the real store.
#[test]
fn proof_layerxledger_0x101a() {
    let proof: Value = serde_json::from_reader(std::io::stdin().lock()).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "precompiles/layerxledger/proof_layerxledger_0x101a_test.go producer"
        )
    });
    let abi: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../precompiles/layerxledger/abi.json"
    )))
    .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    let mut registry = AbiRegistry::default();
    registry.insert(LoadedAbi {
        contract: "layerxledger".to_owned(),
        address: Some(precompile_address(0x101a)),
        events: AbiRegistry::parse_events("layerxledger", &abi)
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}")),
    });
    let owner = proof["owner"]
        .as_str()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
        .to_ascii_lowercase();
    let target = format!("/v1/holders/{owner}?kind=grant");
    let path = std::path::Path::new(
        proof["store"]
            .as_str()
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value")),
    );
    let units = commit_ledger_units(&proof, &registry, &owner, path);
    let store = Store::open(path)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    let (status, page, events) = assert_payment_events(&store, &proof, &target);
    drop(store);
    let store = Store::open(path)
        .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    assert_eq!(get(&store, &target), (status, page.clone()));
    assert_eq!(
        store
            .events("paxeer")
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}")),
        events
    );
    assert!(store.commit(&units[0], 2).is_err());
    assert_eq!(get(&store, &target), (status, page));
    assert_eq!(
        store
            .events("paxeer")
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}")),
        events
    );
    // Reject a truncated real payment body at the consumer's own ABI boundary.
    let payment = &proof["paymentLogs"][1];
    let topics = payment["topics"]
        .as_array()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"))
        .iter()
        .map(|topic| {
            layerx_indexer::codec::unhex_fixed::<32>(
                topic
                    .as_str()
                    .unwrap_or_else(|| panic!("called Option::unwrap() on a None value")),
            )
            .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"))
        })
        .collect::<Vec<_>>();
    let mut data = layerx_indexer::codec::unhex(
        payment["data"]
            .as_str()
            .unwrap_or_else(|| panic!("called Option::unwrap() on a None value")),
    )
    .unwrap_or_else(|error| panic!("called Result::unwrap() on an Err value: {error:?}"));
    data.pop()
        .unwrap_or_else(|| panic!("called Option::unwrap() on a None value"));
    assert!(registry
        .decode_log(precompile_address(0x101a), &topics, &data)
        .is_err());
}
