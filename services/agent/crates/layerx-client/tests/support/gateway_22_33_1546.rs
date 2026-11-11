#![allow(dead_code)]
use layerx_client::px::{PxEndpoint, Uint256};
use layerx_client::rpc::{history, payments};
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use serde_json::{json, Value};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::time::Duration;

pub fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}
pub fn records() -> Value {
    must(serde_json::from_str(include_str!(
        "../fixtures/repair_22_33_1546_gateway.json"
    )))
}
fn text(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("text: {value}"))
}
pub struct Server {
    pub url: String,
    stop: Arc<AtomicBool>,
    join: Option<std::thread::JoinHandle<()>>,
    pub requests: Arc<Mutex<Vec<Value>>>,
}
impl Server {
    pub fn start(handler: impl Fn(&Value) -> Value + Send + 'static) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let seen = requests.clone();
        let join = std::thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                must(stream.set_write_timeout(Some(Duration::from_secs(5))));
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
                let mut body = vec![
                    0;
                    match length {
                        Some(required_value) => required_value,
                        None => panic!("HTTP body length"),
                    }
                ];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                must(seen.lock()).push(request.clone());
                let mut answer = handler(&request);
                answer["id"] = request["id"].clone();
                answer["jsonrpc"] = json!("2.0");
                let body = answer.to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()));
                must(stream.flush());
            }
        });
        Self {
            url,
            stop,
            join: Some(join),
            requests,
        }
    }
    pub fn sequencer(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
    }
    pub fn px(&self) -> PxEndpoint {
        PxEndpoint::from(self.sequencer())
    }
    pub fn count(&self) -> usize {
        must(self.requests.lock()).len()
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(join) = self.join.take() {
            if std::thread::panicking() {
                let _ = join.join();
            } else {
                must(join.join());
            }
        }
    }
}

fn assert_assets(table: &layerx_client::px::PxAssetTable, expected: &Value) {
    assert_eq!(
        table.height,
        match expected["height"].as_u64() {
            Some(required_value) => required_value,
            None => panic!("height"),
        }
    );
    assert_eq!(table.joined_limit, 16);
    let Some(rows) = expected["assets"].as_array() else {
        panic!("rows")
    };
    assert_eq!(table.assets.len(), rows.len());
    for (asset, row) in table.assets.iter().zip(rows) {
        assert_eq!(asset.asset_id, row["asset_id"]);
        assert_eq!(asset.kind, row["kind"]);
        assert_eq!(asset.denom.as_deref(), row["denom"].as_str());
        assert_eq!(asset.name.as_deref(), row["name"].as_str());
        assert_eq!(asset.symbol.as_deref(), row["symbol"].as_str());
        assert_eq!(asset.decimals.map(u64::from), row["decimals"].as_u64());
        assert_eq!(
            asset.supply.map(Uint256::decimal),
            row["supply"].as_str().map(str::to_owned)
        );
        assert_eq!(asset.paused, row.get("paused").and_then(Value::as_bool));
        assert_eq!(
            asset.verification.as_deref(),
            row.get("verification").and_then(Value::as_str)
        );
    }
}
pub fn assets() {
    let expected = records()["assets"].clone();
    let response_body = expected.clone();
    let server = Server::start(move |r| {
        assert_eq!(r["method"], "px_listAssets");
        assert_eq!(r["params"], json!([]));
        json!({"result":response_body})
    });
    let endpoint = server.px();
    let table = must(payments::list_assets(&endpoint));
    assert_assets(&table, &expected);
    for asset in &table.assets {
        assert_eq!(
            must(payments::get_asset(&endpoint, &asset.asset_id)),
            *asset
        );
    }
    assert_eq!(
        payments::get_asset(&endpoint, "missing"),
        Err(SequencerError::Unavailable)
    );
    assert_eq!(server.count(), 18);
    let rows = table.assets;
    assert_eq!(
        (match (match rows.iter().find(|v| v.asset_id == "denom:ubank") {
            Some(required_value) => required_value,
            None => panic!("bank"),
        })
        .supply
        {
            Some(required_value) => required_value,
            None => panic!("supply"),
        })
        .decimal(),
        "115792089237316195423570985008687907853269984665640564039457584007913129639935"
    );
    let Some(broken) = rows.iter().find(|v| v.asset_id == "denom:ubroken") else {
        panic!("missing metadata")
    };
    assert_eq!(
        (
            broken.name.as_ref(),
            broken.symbol.as_ref(),
            broken.decimals,
            broken.supply
        ),
        (None, None, None, None)
    );
    let mut cases = vec![];
    for (key, value) in [
        ("asset_id", json!("")),
        ("decimals", json!(256)),
        ("supply", json!("01")),
        ("paused", json!("false")),
        ("verification", json!("unverified")),
    ] {
        let mut bad = expected.clone();
        bad["assets"][0][key] = value;
        cases.push(bad);
    }
    for bad in cases {
        let server = Server::start(move |_| json!({"result":bad}));
        assert_eq!(
            payments::list_assets(&server.px()),
            Err(SequencerError::Malformed)
        );
        assert_eq!(
            payments::get_asset(&server.px(), "missing"),
            Err(SequencerError::Malformed)
        );
    }
    let server = Server::start(
        |_| json!({"error":{"code":-32_004,"message":"assets unavailable","data":{"reason":"unavailable"}}}),
    );
    assert!(
        matches!(payments::list_assets(&server.px()),Err(SequencerError::Rpc(e)) if e.code==-32_004 && e.data==Some(json!({"reason":"unavailable"})))
    );
}

pub fn balances() {
    let recorded = records();
    let account = text(&recorded["account"]["account"]["evm_address"]).to_owned();
    let selector = account.clone();
    let response_body = recorded["balances"].clone();
    let expected = response_body.clone();
    let server = Server::start(move |r| {
        assert_eq!(r["method"], "px_getBalances");
        assert_eq!(r["params"], json!([selector]));
        json!({"result":response_body})
    });
    for _ in 0..2 {
        let result = must(server.px().balances(&account));
        assert_eq!(result.height, 16);
        assert!(result.account.bound);
        assert_eq!(
            result.account.evm_address,
            Some(hex_bytes::<20>(&expected["account"]["evm_address"]))
        );
        assert_eq!(
            result.account.pax_address.as_deref(),
            expected["account"]["pax_address"].as_str()
        );
        assert_eq!(
            result.account.layerx_did.as_deref(),
            expected["account"]["layerx_did"].as_str()
        );
        assert_eq!(
            result.account.layerx_account,
            Some(hex_bytes::<32>(&expected["account"]["layerx_account"]))
        );
        let Some(rows) = expected["balances"].as_array() else {
            panic!("coins")
        };
        assert_eq!(result.balances.len(), rows.len());
        for (coin, row) in result.balances.iter().zip(rows) {
            assert_eq!(coin.denom, row["denom"]);
            assert_eq!(coin.amount.decimal(), row["amount"]);
            assert_eq!(
                Some(coin.amount),
                Uint256::from_decimal(text(&row["amount"]))
            );
        }
        assert_eq!(
            result.balances[1].amount.decimal(),
            "1606938044258990275541962092341162602522202993782792835301376"
        );
    }
    assert_eq!(server.count(), 2);
    for (key, value) in [
        ("amount", json!("01")),
        ("amount", json!("-1")),
        (
            "amount",
            json!("115792089237316195423570985008687907853269984665640564039457584007913129639936"),
        ),
        ("denom", Value::Null),
    ] {
        let mut bad = expected.clone();
        bad["balances"][0][key] = value;
        let server = Server::start(move |_| json!({"result":bad}));
        assert_eq!(
            server.px().balances(&account),
            Err(SequencerError::Malformed)
        );
    }
    for key in ["account", "height", "balances"] {
        let mut bad = expected.clone();
        (match bad.as_object_mut() {
            Some(required_value) => required_value,
            None => panic!("object"),
        })
        .remove(key);
        let server = Server::start(move |_| json!({"result":bad}));
        assert_eq!(
            server.px().balances(&account),
            Err(SequencerError::Malformed)
        );
    }
    let server = Server::start(
        |_| json!({"error":{"code":-32_001,"message":"Paxeer read unavailable","data":{"code":"unbound_did"}}}),
    );
    assert!(
        matches!(server.px().balances(&account),Err(SequencerError::Rpc(e)) if e.data==Some(json!({"code":"unbound_did"})))
    );
}
fn hex_bytes<const N: usize>(value: &Value) -> [u8; N] {
    let digits = text(value).strip_prefix("0x").unwrap_or(text(value));
    let bytes: Vec<_> = digits
        .as_bytes()
        .chunks_exact(2)
        .map(|p| must(u8::from_str_radix(must(std::str::from_utf8(p)), 16)))
        .collect();
    must(bytes.try_into())
}
pub fn history() {
    let response_body = records()["history"].clone();
    let expected = response_body.clone();
    let account = text(&response_body["account"]).to_owned();
    let selector = account.clone();
    let server = Server::start(move |r| {
        assert_eq!(r["method"], "px_getHistory");
        assert_eq!(r["params"], json!([selector, null, 50]));
        json!({"result":response_body})
    });
    let page = must(history::history(&server.sequencer(), &account, None, None));
    assert_eq!(page.account, account);
    assert_eq!(page.chain_height, Some(12));
    assert_eq!(page.indexed_height, Some(12));
    assert_eq!(page.next_cursor, None);
    assert_eq!(page.freshness, expected["freshness"]);
    assert_eq!(
        page.items,
        *match expected["items"].as_array() {
            Some(required_value) => required_value,
            None => panic!("items"),
        }
    );
    assert_eq!(page.items.len(), 4);
    // Producer metadata and nested row coordinates are retained, including null metadata.
    assert_eq!(page.items[1]["asset_metadata"]["decimals"], 6);
    assert_eq!(page.items[3]["asset_metadata"], Value::Null);
    assert_eq!(server.count(), 1);
    for cursor in [
        "a".repeat(112),
        "000000000000ABCD0000000100000000".to_owned(),
        "g".repeat(32),
        String::new(),
    ] {
        assert_eq!(
            history::history(&server.sequencer(), &account, Some(&cursor), None),
            Err(SequencerError::Endpoint)
        );
    }
    assert_eq!(server.count(), 1);
    let next = "000000000000000c0000000100000000";
    let mut paged = expected.clone();
    paged["next_cursor"] = json!(next);
    let selector = account.clone();
    let request_count = Arc::new(Mutex::new(0));
    let seen = request_count.clone();
    let server = Server::start(move |r| {
        let mut count = must(seen.lock());
        assert_eq!(r["method"], "px_getHistory");
        let mut result = paged.clone();
        if *count == 0 {
            assert_eq!(r["params"], json!([selector, null, 4]));
        } else {
            assert_eq!(r["params"], json!([selector, next, 4]));
            result["items"] = json!([]);
            result["next_cursor"] = Value::Null;
        }
        *count += 1;
        json!({"result":result})
    });
    let first = must(history::history(
        &server.sequencer(),
        &account,
        None,
        Some(4),
    ));
    assert_eq!(first.next_cursor.as_deref(), Some(next));
    let last = must(history::history(
        &server.sequencer(),
        &account,
        first.next_cursor.as_deref(),
        Some(4),
    ));
    assert_eq!(last.next_cursor, None);
    assert!(last.items.is_empty());
    assert_eq!(last.chain_height, first.chain_height);
    assert_eq!(last.indexed_height, first.indexed_height);
    assert_eq!(last.freshness, first.freshness);
    for (key, value) in [
        ("next_cursor", json!("a".repeat(112))),
        ("next_cursor", json!(next.to_uppercase())),
        ("freshness", json!({})),
        ("chainHeight", Value::Null),
    ] {
        let mut bad = expected.clone();
        bad[key] = value;
        let server = Server::start(move |_| json!({"result":bad}));
        assert_eq!(
            history::history(&server.sequencer(), &account, None, None),
            Err(SequencerError::Malformed)
        );
    }
    let server = Server::start(
        |_| json!({"error":{"code":-32_001,"message":"History unavailable","data":{"code":"indexer_not_configured"}}}),
    );
    assert!(
        matches!(history::history(&server.sequencer(),&account,None,None),Err(SequencerError::Rpc(e)) if e.code==-32_001 && e.data==Some(json!({"code":"indexer_not_configured"})))
    );
}

pub fn repeated_reads() {
    let recorded = records();
    let response_body = recorded.clone();
    let server = Server::start(move |r| {
        let field = match text(&r["method"]) {
            "px_getAccount" => "account",
            "px_getBalances" => "balances",
            "px_listAssets" => "assets",
            "px_getHistory" => "history",
            other => panic!("unexpected {other}"),
        };
        match field {
            "assets" => assert_eq!(r["params"], json!([])),
            "history" => assert_eq!(
                r["params"],
                json!([response_body["history"]["account"], null, 50])
            ),
            _ => assert_eq!(
                r["params"],
                json!([response_body["account"]["account"]["evm_address"]])
            ),
        }
        json!({"result":response_body[field]})
    });
    let account = text(&recorded["account"]["account"]["evm_address"]);
    for _ in 0..2 {
        let joined = must(server.px().account(account));
        assert_eq!(
            joined.account.evm_address,
            Some(hex_bytes(&recorded["account"]["account"]["evm_address"]))
        );
        assert!(joined.account.bound);
        assert_eq!(joined.layerx, None);
        let Some(pax) = joined.paxeer else {
            panic!("Paxeer")
        };
        assert_eq!(pax.height, 16);
        assert_eq!(pax.nonce, 42);
        assert_eq!(pax.balance.decimal(), "81985529216486895");
        let balances = must(server.px().balances(account));
        assert_eq!(balances.account, joined.account);
        assert_eq!(balances.height, pax.height);
        assert_assets(&must(server.px().list_assets()), &recorded["assets"]);
        let page = must(history::history(
            &server.sequencer(),
            text(&recorded["history"]["account"]),
            None,
            None,
        ));
        assert_eq!(
            page.items,
            *match recorded["history"]["items"].as_array() {
                Some(required_value) => required_value,
                None => panic!("items"),
            }
        );
        assert_eq!(page.freshness, recorded["history"]["freshness"]);
    }
    assert_eq!(server.count(), 8);
    for method in [
        "px_getAccount",
        "px_getBalances",
        "px_listAssets",
        "px_getHistory",
    ] {
        let server = Server::start(move |r| {
            assert_eq!(r["method"], method);
            json!({"result":{}})
        });
        let result = match method {
            "px_getAccount" => server.px().account(account).map(|_| ()),
            "px_getBalances" => server.px().balances(account).map(|_| ()),
            "px_listAssets" => server.px().list_assets().map(|_| ()),
            _ => history::history(&server.sequencer(), account, None, None).map(|_| ()),
        };
        assert_eq!(result, Err(SequencerError::Malformed));
    }
}
