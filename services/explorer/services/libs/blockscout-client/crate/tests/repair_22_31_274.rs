//! Offline integration coverage of the production generated client over real
//! loopback HTTP, using committed recordings and explicit producer-view examples.
//! No live explorer, environment configuration, credentials or subprocesses.

use blockscout_client::{
    apis::{
        addresses_api::*, blocks_api::*, feed::FeedItem, pagination::PageCursor, search_api::*,
        smart_contracts_api::*, tokens_api::*, transactions_api::*, ResponseContent,
    },
    Api, ApiClient, Configuration,
};
use serde::{de::DeserializeOwned, Serialize};
use serde_json::Value;
use std::collections::{BTreeMap, BTreeSet};

// The HTTP peer replays committed explorer recordings. Endpoints absent from
// those recordings have explicit examples of the producer views below; the
// component under test is always ApiClient and its production transport/codecs.
const ADDRESS: &str = "0xc0De20A37E2dAC848F81A93BD85FE4ACDdE7C0DE";
const TOKEN: &str = "0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401";
const TRANSACTION: &str = "0x4dd7e3f4522fcf2483ae422fd007492380051d87de6fdb17be71c7134e26857e";
const INSTANCE: &str =
    "25625468407840116393736812939389551247551040926951238633020744494000165263268";
const HUGE_TOKEN_ID: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639935";

fn address() -> String {
    ADDRESS.to_owned()
}
fn token() -> String {
    TOKEN.to_owned()
}
fn transaction() -> String {
    TRANSACTION.to_owned()
}
fn block() -> String {
    "23116935".to_owned()
}
fn instance() -> String {
    INSTANCE.to_owned()
}

macro_rules! recorded {
    ($name:literal) => {{
        let transcript: Value =
            serde_json::from_str(include_str!(concat!("recorded/eth_blockscout_com/", $name)))
                .unwrap();
        assert_eq!(transcript["response"]["status"], 200);
        serde_json::from_str::<Value>(transcript["response"]["body"].as_str().unwrap()).unwrap()
    }};
}

// Query vectors retain duplicate keys, so equality rejects duplication as well
// as dropped/truncated cursor values and lost original filters.
type RequestKey = (String, Vec<(String, String)>);

#[derive(Default)]
struct Replay {
    responses: BTreeMap<RequestKey, String>,
}

impl Replay {
    fn response(&mut self, path: &str, query: &[(&str, &str)], body: String) {
        let mut query: Vec<_> = query
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect();
        query.sort();
        assert!(self
            .responses
            .insert((path.to_owned(), query), body)
            .is_none());
    }

    fn pages(&mut self, path: &str, filters: &[(&str, &str)], body: Value, cursor: Value) {
        let cursor_query: Vec<_> = cursor
            .as_object()
            .unwrap()
            .iter()
            .map(|(k, v)| {
                let value = match v {
                    Value::String(value) => value.clone(),
                    Value::Null => String::new(),
                    Value::Bool(_) | Value::Number(_) => v.to_string(),
                    _ => panic!("fixture cursor must be scalar"),
                };
                (k.clone(), value)
            })
            .collect();
        self.pages_with_wire_cursor(path, filters, body, &cursor.to_string(), &cursor_query);
    }

    fn pages_with_wire_cursor(
        &mut self,
        path: &str,
        filters: &[(&str, &str)],
        body: Value,
        cursor_json: &str,
        cursor_query: &[(String, String)],
    ) {
        let items = body["items"].as_array().unwrap();
        assert!(
            items.len() >= 2,
            "replay requires two nonempty pages: {path}"
        );
        let split = items.len() / 2;
        let encode = |items: &[Value], cursor: &str| {
            // Encode the cursor separately to retain the exact numeric spelling
            // of uint256 token IDs on the wire, without a JSON Value conversion.
            let mut fields = vec![
                format!("\"items\":{}", serde_json::to_string(items).unwrap()),
                format!("\"next_page_params\":{cursor}"),
            ];
            for (name, value) in body.as_object().unwrap() {
                if name != "items" && name != "next_page_params" {
                    fields.push(format!(
                        "{}:{}",
                        serde_json::to_string(name).unwrap(),
                        value
                    ));
                }
            }
            format!("{{{}}}", fields.join(","))
        };
        self.response(path, filters, encode(&items[..split], cursor_json));
        let mut query: BTreeMap<String, String> = filters
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect();
        for (key, value) in cursor_query {
            if let Some(original) = query.insert(key.clone(), value.clone()) {
                assert_eq!(original, *value, "invalid fixture filter collision");
            }
        }
        let query: Vec<_> = query
            .iter()
            .map(|(k, v)| (k.as_str(), v.as_str()))
            .collect();
        self.response(path, &query, encode(&items[split..], "null"));
    }
}

struct RecordedClient {
    api: ApiClient,
    server: tokio::task::JoinHandle<()>,
}

impl std::ops::Deref for RecordedClient {
    type Target = ApiClient;
    fn deref(&self) -> &Self::Target {
        &self.api
    }
}

impl Drop for RecordedClient {
    fn drop(&mut self) {
        self.server.abort();
    }
}

fn client() -> RecordedClient {
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    let replay = explorer_responses();
    let listener = std::net::TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0)).unwrap();
    listener.set_nonblocking(true).unwrap();
    let base = format!("http://{}", listener.local_addr().unwrap());
    let listener = tokio::net::TcpListener::from_std(listener).unwrap();
    let server = tokio::spawn(async move {
        loop {
            let (mut socket, _) = listener.accept().await.unwrap();
            let mut bytes = Vec::new();
            let mut buffer = [0; 4096];
            while !bytes.windows(4).any(|w| w == b"\r\n\r\n") {
                let size = tokio::time::timeout(
                    std::time::Duration::from_secs(5),
                    socket.read(&mut buffer),
                )
                .await
                .unwrap()
                .unwrap();
                assert!(size > 0, "incomplete HTTP request");
                bytes.extend_from_slice(&buffer[..size]);
                assert!(bytes.len() <= 65536, "HTTP request exceeds fixture bound");
            }
            let request = std::str::from_utf8(&bytes).unwrap();
            let mut line = request.lines().next().unwrap().split_whitespace();
            assert_eq!(line.next(), Some("GET"));
            let url =
                url::Url::parse(&format!("http://localhost{}", line.next().unwrap())).unwrap();
            let mut query: Vec<_> = url
                .query_pairs()
                .map(|(k, v)| (k.into_owned(), v.into_owned()))
                .collect();
            query.sort();
            let key = (url.path().to_owned(), query);
            let (status, body) = match replay.responses.get(&key) {
                Some(body) => ("200 OK", body.clone()),
                None => (
                    "400 Bad Request",
                    serde_json::json!({"unexpected_request": format!("{key:?}")}).to_string(),
                ),
            };
            let response = format!("HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len());
            socket.write_all(response.as_bytes()).await.unwrap();
            socket.shutdown().await.unwrap();
        }
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(std::time::Duration::from_secs(10))
        .build()
        .unwrap();
    let mut config = Configuration::new();
    config.base_path = base;
    config.client = reqwest_middleware::ClientBuilder::new(http).build();
    RecordedClient {
        api: ApiClient::new(config),
        server,
    }
}

fn explorer_responses() -> Replay {
    use serde_json::json;
    let mut replay = Replay::default();
    let transactions = recorded!("_api_v2_transactions.json");
    let blocks = recorded!("_api_v2_blocks.json");
    let transfers = recorded!("_api_v2_token-transfers.json");
    let token_transfers =
        recorded!("_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_transfers.json");
    let logs = recorded!("_api_v2_addresses_0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3_logs.json");
    let holders =
        recorded!("_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_holders.json");
    let tokens = recorded!("_api_v2_tokens.json");
    let contracts = recorded!("_api_v2_smart-contracts.json");
    let search = recorded!("_api_v2_search_q=USDT.json");
    let collections = recorded!(
        "_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_nft_collections.json"
    );
    let balances =
        recorded!("_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_tokens.json");
    let instances =
        recorded!("_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_instances.json");
    let internal = recorded!("_api_v2_transactions_0x4dd7e3f4_internal-transactions.json");

    // The original recordings usually contain fifty items. Partition their
    // unchanged items into two HTTP pages, with a producer cursor then null.
    // Singleton/missing recordings use explicit producer-view scenarios. These
    // are HTTP peer fixtures, never alternate client implementations.
    // FeedView.prepare_update: successful native submission with one market.
    let native = json!({
        "hash":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        "block_number":23116936, "block_hash":"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "timestamp":"2025-08-11T09:33:11.000000Z", "index":3,
        "from":{"hash":ADDRESS,"is_contract":false}, "to":null, "value":"0",
        "fee":{"type":"actual","value":"0"}, "gas_used":"12500", "gas_limit":"20000",
        "status":"ok", "result":"success", "method":"submitPrices", "transaction_types":["price_update"],
        "type":127, "confirmations":3, "price_update":{
            "feeder_bech32":"paxeer1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqn2m7av", "observation_count":1,
            "observations":[{"market_id":7,"accepted":true,"symbol":"ETH/USD",
                "index_price":"4273590000000000000000", "mark_price":"4273550000000000000000",
                "source_timestamp_ms":1754904780000u64,"sequence":42}]
        }
    });
    let mixed = vec![native, transactions["items"][0].clone()];
    replay.response(
        "/api/v2/main-page/transactions",
        &[],
        serde_json::to_string(&mixed).unwrap(),
    );
    replay.response(
        "/api/v2/transactions",
        &[],
        json!({"items":mixed,"next_page_params":null}).to_string(),
    );
    // AddressView daily-history response, matching address_controller_test's
    // nonempty 2000/1000 example and configured ten-day window.
    replay.response(&format!("/api/v2/addresses/{ADDRESS}/coin-balance-history-by-day"), &[],
        json!({"items":[{"date":"2025-08-10","value":"2000"},{"date":"2025-08-11","value":"1000"}],"days":10}).to_string());

    let block_cursor = blocks["next_page_params"].clone();
    let tx_cursor = transactions["next_page_params"].clone();
    let transfer_cursor = transfers["next_page_params"].clone();
    let holder_cursor = holders["next_page_params"].clone();
    replay.pages(
        "/api/v2/blocks",
        &[("type", "block")],
        blocks.clone(),
        block_cursor.clone(),
    );
    replay.pages(
        &format!("/api/v2/blocks/{}/transactions", block()),
        &[],
        transactions.clone(),
        tx_cursor.clone(),
    );

    // WithdrawalView's loaded-block/address branch; no withdrawal recording is
    // present in tests/recorded. All three mounted clients consume this shape.
    let withdrawals = json!({"items":[
        {"index":1001,"validator_index":51,"amount":"1000000000","receiver":{"hash":ADDRESS,"is_contract":false,"private_tags":[],"public_tags":[],"watchlist_names":[]},"block_number":23116935,"timestamp":"2025-08-11T09:32:59.000000Z"},
        {"index":1000,"validator_index":50,"amount":"2000000000","receiver":{"hash":ADDRESS,"is_contract":false,"private_tags":[],"public_tags":[],"watchlist_names":[]},"block_number":23116934,"timestamp":"2025-08-11T09:32:47.000000Z"}
    ]});
    for path in [
        "/api/v2/withdrawals".to_owned(),
        format!("/api/v2/blocks/{}/withdrawals", block()),
        format!("/api/v2/addresses/{ADDRESS}/withdrawals"),
    ] {
        replay.pages(
            &path,
            &[],
            withdrawals.clone(),
            json!({"index":1001,"items_count":1}),
        );
    }
    let mut internal_second = internal["items"][0].clone();
    // A second call in the same transaction, using InternalTransactionView's
    // identical shape but a distinct intra-transaction/block index.
    internal_second["index"] = json!(2);
    internal_second["block_index"] = json!(751);
    let internal = json!({"items":[internal["items"][0],internal_second]});
    let internal_cursor =
        json!({"block_number":20077921,"transaction_index":134,"index":1,"items_count":1});
    replay.pages(
        "/api/v2/internal-transactions",
        &[],
        internal.clone(),
        internal_cursor.clone(),
    );
    replay.pages(
        &format!("/api/v2/addresses/{ADDRESS}/internal-transactions"),
        &[("filter", "to")],
        internal.clone(),
        internal_cursor.clone(),
    );
    replay.pages(
        &format!("/api/v2/transactions/{TRANSACTION}/internal-transactions"),
        &[],
        internal,
        internal_cursor,
    );
    replay.pages(
        "/api/v2/token-transfers",
        &[],
        transfers.clone(),
        transfer_cursor,
    );
    replay.pages(
        &format!("/api/v2/addresses/{ADDRESS}/blocks-validated"),
        &[],
        recorded!(
            "_api_v2_addresses_0x4838B106FCe9647Bdf1E7877BF73cE8B0BAD5f97_blocks-validated.json"
        ),
        block_cursor,
    );
    replay.pages(&format!("/api/v2/addresses/{ADDRESS}/coin-balance-history"), &[], json!({"items":[
        {"transaction_hash":TRANSACTION,"block_number":23116935,"block_timestamp":"2025-08-11T09:32:59.000000Z","delta":"-1000","value":"1000"},
        {"transaction_hash":null,"block_number":23116934,"block_timestamp":"2025-08-11T09:32:47.000000Z","delta":"2000","value":"2000"}
    ]}), json!({"block_number":23116935,"items_count":1}));
    let log_cursor = json!({"index":logs["items"][0]["index"],"block_number":logs["items"][0]["block_number"],"items_count":6});
    replay.pages(
        &format!("/api/v2/addresses/{ADDRESS}/logs"),
        &[],
        logs.clone(),
        log_cursor.clone(),
    );
    replay.pages(
        &format!("/api/v2/transactions/{TRANSACTION}/logs"),
        &[],
        logs,
        log_cursor,
    );

    // AddressView.prepare_nft projects the recorded collection instances with
    // their enclosing token. Keep metadata and uint256 IDs from the recording.
    let nft_items: Vec<_> = collections["items"][0]["token_instances"]
        .as_array()
        .unwrap()
        .iter()
        .map(|item| {
            let mut item = item.clone();
            item["token"] = collections["items"][0]["token"].clone();
            item
        })
        .collect();
    replay.pages(&format!("/api/v2/addresses/{ADDRESS}/nft"), &[("type","ERC-721")],
        json!({"items":nft_items}), json!({"token_name":"NameWrapper","token_type":"ERC-1155","token_inserted_at":"2025-08-11T09:32:59Z","unique_token":INSTANCE}));
    // A second collection scenario for the collection list boundary.
    let mut second_collection = collections["items"][0].clone();
    second_collection["token"]["address"] = json!("0x0000000000000000000000000000000000000721");
    second_collection["token"]["address_hash"] = second_collection["token"]["address"].clone();
    second_collection["token"]["type"] = json!("ERC-721");
    second_collection["token"]["name"] = json!("Example NFT");
    for item in second_collection["token_instances"].as_array_mut().unwrap() {
        item["token_type"] = json!("ERC-721");
    }
    replay.pages(&format!("/api/v2/addresses/{ADDRESS}/nft/collections"), &[("type","ERC-721")],
        json!({"items":[collections["items"][0],second_collection]}), json!({"token_name":"NameWrapper","token_type":"ERC-1155","token_inserted_at":"2025-08-11T09:32:59Z","items_count":1}));
    replay.pages(
        &format!("/api/v2/addresses/{ADDRESS}/token-transfers"),
        &[("filter", "to"), ("type", "ERC-1155"), ("token", TOKEN)],
        recorded!(
            "_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_token-transfers.json"
        ),
        json!({"block_number":21908651,"index":379,"items_count":6}),
    );
    replay.pages(
        &format!("/api/v2/addresses/{ADDRESS}/tokens"),
        &[("type", "ERC-721")],
        balances,
        json!({"fiat_value":null,"id":123,"value":"1","items_count":2}),
    );
    replay.pages(
        &format!("/api/v2/addresses/{ADDRESS}/transactions"),
        &[("filter", "to")],
        transactions.clone(),
        tx_cursor.clone(),
    );
    // AddressView.prepare_address_for_list on recorded transaction addresses.
    let addresses: Vec<_> = transactions["items"].as_array().unwrap()[..2]
        .iter()
        .map(|tx| {
            let mut account = tx["to"].clone();
            account["coin_balance"] = json!("1000");
            account["transactions_count"] = json!("7");
            account
        })
        .collect();
    replay.pages("/api/v2/addresses", &[], json!({"items":addresses,"exchange_rate":"4273.59","total_supply":"120000000"}),
        json!({"hash":addresses[0]["hash"],"fetched_coin_balance":"1000","transactions_count":7,"items_count":1}));
    replay.pages(
        "/api/v2/tokens",
        &[("q", "USDT"), ("type", "ERC-1155")],
        tokens.clone(),
        tokens["next_page_params"].clone(),
    );
    replay.pages(
        &format!("/api/v2/tokens/{TOKEN}/holders"),
        &[],
        holders.clone(),
        holder_cursor.clone(),
    );
    // HolderView is shared with instance holders. Replay real recorded holder
    // objects while checking the instance path (including its complete ID).
    replay.pages(
        &format!("/api/v2/tokens/{TOKEN}/instances/{INSTANCE}/holders"),
        &[],
        holders,
        holder_cursor,
    );
    replay.pages_with_wire_cursor(
        &format!("/api/v2/tokens/{TOKEN}/instances"),
        &[],
        instances,
        &format!("{{\"unique_token\":{HUGE_TOKEN_ID}}}"),
        &[("unique_token".to_owned(), HUGE_TOKEN_ID.to_owned())],
    );
    replay.pages(
        &format!("/api/v2/tokens/{TOKEN}/instances/{INSTANCE}/transfers"),
        &[],
        token_transfers.clone(),
        token_transfers["next_page_params"].clone(),
    );
    replay.pages(
        &format!("/api/v2/tokens/{TOKEN}/transfers"),
        &[],
        token_transfers.clone(),
        token_transfers["next_page_params"].clone(),
    );
    replay.pages(
        "/api/v2/transactions",
        &[("filter", "validated"), ("type", "coin_transfer")],
        transactions,
        tx_cursor,
    );
    replay.pages(
        &format!("/api/v2/transactions/{TRANSACTION}/token-transfers"),
        &[("type", "ERC-1155")],
        token_transfers.clone(),
        token_transfers["next_page_params"].clone(),
    );
    // TransactionView's state_changes envelope includes the legacy null marker
    // and the items_count offset, both of which must survive serialization.
    replay.pages(&format!("/api/v2/transactions/{TRANSACTION}/state-changes"), &[], json!({"items":[
        {"type":"coin","is_miner":false,"address":{"hash":ADDRESS,"is_contract":false},"balance_before":"2000","balance_after":"1000","change":"-1000"},
        {"type":"coin","is_miner":true,"address":{"hash":TOKEN,"is_contract":true},"balance_before":"0","balance_after":"1000","change":"1000"}
    ]}), json!({"state_changes":null,"items_count":1}));
    replay.pages(
        "/api/v2/smart-contracts",
        &[("q", "CryptoPunks"), ("filter", "solidity")],
        contracts.clone(),
        contracts["next_page_params"].clone(),
    );
    replay.pages(
        "/api/v2/search",
        &[("q", "USDT")],
        search.clone(),
        search["next_page_params"].clone(),
    );
    replay
}

fn wire_items<T>(response: &ResponseContent<T>) -> Vec<Value> {
    assert_eq!(
        response.status,
        reqwest::StatusCode::OK,
        "{}",
        response.content
    );
    let body: Value = serde_json::from_str(&response.content).unwrap();
    body["items"]
        .as_array()
        .expect("producer must return an items envelope")
        .clone()
}

fn assert_same_items<T: Serialize + DeserializeOwned>(typed: &[T], wire: &[Value]) {
    assert_eq!(typed.len(), wire.len(), "the typed consumer lost entries");
    // Compare every entry in order after the production codec's normalization
    // (for example, Decimal accepts numbers but serializes them as strings).
    for (item, source) in typed.iter().zip(wire) {
        let expected: T = serde_json::from_value(source.clone())
            .expect("each producer item must decode through its production type");
        let actual = serde_json::to_value(item).unwrap();
        assert!(actual.is_object());
        assert_eq!(
            actual,
            serde_json::to_value(expected).unwrap(),
            "the typed consumer changed or reordered a producer item"
        );
    }
}

fn finish_pages<T>(first: &ResponseContent<T>, second: &ResponseContent<T>) {
    let first_items = wire_items(first);
    let second_items = wire_items(second);
    assert!(
        !first_items.is_empty(),
        "first replay page must be nonempty"
    );
    assert!(
        !second_items.is_empty(),
        "second replay page must be nonempty"
    );
    let first_set: BTreeSet<_> = first_items.iter().map(Value::to_string).collect();
    let second_set: BTreeSet<_> = second_items.iter().map(Value::to_string).collect();
    assert_eq!(
        first_set.len(),
        first_items.len(),
        "first page duplicated an item"
    );
    assert_eq!(
        second_set.len(),
        second_items.len(),
        "second page duplicated an item"
    );
    assert!(
        first_set.is_disjoint(&second_set),
        "cursor repeated a previous page"
    );
    assert!(
        second.next_page_cursor().unwrap().is_none(),
        "second page cursor must be terminal null"
    );
}

// Every call below goes through ApiClient, the production reqwest transport, and
// both generated typed-success decoding paths. The returned cursor is taken
// from the original wire text, not from a potentially rounded JSON Value.
macro_rules! pages {
    ($api:expr, $method:ident $(, $params:expr)?) => {{
        let api = $api;
        let first = api.$method($(($params).clone())?).await.expect(stringify!($method));
        let first_wire = wire_items(&first);
        assert_same_items(&first.try_as_success().unwrap_or_else(|| panic!("{} first typed success: {:?}", stringify!($method), first.try_parse_as_success_model().err())).items, &first_wire);
        let parsed = first.try_parse_as_success_model().expect("first strict success codec");
        assert_same_items(&parsed.items, &first_wire);
        let cursor = first.next_page_cursor().expect("valid producer cursor").expect("first page must carry a cursor");
        let next = api.with_cursor(cursor);
        let second = next.$method($($params)?).await.expect(stringify!($method));
        let second_wire = wire_items(&second);
        assert_same_items(&second.try_as_success().unwrap_or_else(|| panic!("{} second typed success: {:?}", stringify!($method), second.try_parse_as_success_model().err())).items, &second_wire);
        let parsed = second.try_parse_as_success_model().expect("second strict success codec");
        assert_same_items(&parsed.items, &second_wire);
        finish_pages(&first, &second);
    }};
}

#[tokio::test]
async fn homepage_native_items() {
    let api = client();
    let response = api.main_page_api().get_main_page_txs().await.unwrap();
    assert_eq!(response.status, reqwest::StatusCode::OK);
    let wire: Vec<Value> = serde_json::from_str(&response.content).unwrap();
    let typed = response
        .try_as_success()
        .expect("mixed homepage typed success");
    let parsed = response.try_parse_as_success_model().unwrap();
    assert_eq!(typed.len(), wire.len());
    assert_eq!(parsed.len(), wire.len());
    assert_same_items(typed, &wire);
    assert_same_items(&parsed, &wire);
    assert!(
        typed.iter().any(|item| matches!(item, FeedItem::Evm(_))),
        "homepage must contain an EVM transaction"
    );
    assert!(
        typed.iter().any(|item| matches!(item, FeedItem::Native(_))),
        "homepage must contain a native submission"
    );
    for (item, source) in typed.iter().zip(&wire) {
        match item {
            FeedItem::Evm(tx) => {
                assert_ne!(source["type"], 127);
                assert_eq!(tx.hash, source["hash"]);
            }
            FeedItem::Native(update) => {
                assert_eq!(source["type"], 127);
                assert!(source["to"].is_null());
                assert_eq!(
                    serde_json::to_value(update).unwrap(),
                    *source,
                    "native codec must retain every producer field"
                );
                assert_eq!(
                    update.price_update.observation_count as usize,
                    update.price_update.observations.len()
                );
            }
        }
    }
    // The same discriminated item type also consumes the real transaction list.
    let list = api
        .transactions_api()
        .get_txs(GetTxsParams::builder().build())
        .await
        .unwrap();
    let list_typed = list
        .try_as_success()
        .expect("mixed transaction list typed success");
    assert!(list_typed
        .items
        .iter()
        .any(|item| matches!(item, FeedItem::Native(_))));
    assert!(list_typed
        .items
        .iter()
        .any(|item| matches!(item, FeedItem::Evm(_))));
    assert_eq!(list_typed.items.len(), wire_items(&list).len());
    assert_same_items(&list_typed.items, &wire_items(&list));
    let list_parsed = list.try_parse_as_success_model().unwrap();
    assert_same_items(&list_parsed.items, &wire_items(&list));
}

#[tokio::test]
async fn daily_balance_envelope() {
    let api = client();
    let response = api
        .addresses_api()
        .get_address_coin_balance_history_by_day(
            GetAddressCoinBalanceHistoryByDayParams::builder()
                .address_hash(address())
                .build(),
        )
        .await
        .unwrap();
    assert_eq!(response.status, reqwest::StatusCode::OK);
    let wire: Value = serde_json::from_str(&response.content).unwrap();
    let typed = response
        .try_as_success()
        .expect("AddressView items/days typed envelope");
    let parsed = response.try_parse_as_success_model().unwrap();
    assert_eq!(typed, &parsed);
    assert_eq!(typed.days, wire["days"].as_u64().unwrap());
    assert!(typed.days > 0);
    let entries = wire["items"].as_array().unwrap();
    assert!(!entries.is_empty(), "daily envelope must contain balances");
    assert_eq!(typed.items.len(), entries.len());
    for (item, source) in typed.items.iter().zip(entries) {
        assert_eq!(item.date, source["date"]);
        assert_eq!(item.value.to_string(), source["value"].as_str().unwrap());
    }
}

#[tokio::test]
async fn address_cursors() {
    let client = client();
    let api = client.addresses_api();
    pages!(
        api,
        get_address_blocks_validated,
        GetAddressBlocksValidatedParams::builder()
            .address_hash(address())
            .build()
    );
    pages!(
        api,
        get_address_coin_balance_history,
        GetAddressCoinBalanceHistoryParams::builder()
            .address_hash(address())
            .build()
    );
    pages!(
        api,
        get_address_internal_txs,
        GetAddressInternalTxsParams::builder()
            .address_hash(address())
            .filter("to".to_owned())
            .build()
    );
    pages!(
        api,
        get_address_logs,
        GetAddressLogsParams::builder()
            .address_hash(address())
            .build()
    );
    pages!(
        api,
        get_address_nft,
        GetAddressNftParams::builder()
            .address_hash(address())
            .r#type("ERC-721".to_owned())
            .build()
    );
    pages!(
        api,
        get_address_nft_collections,
        GetAddressNftCollectionsParams::builder()
            .address_hash(address())
            .r#type("ERC-721".to_owned())
            .build()
    );
    pages!(
        api,
        get_address_token_transfers,
        GetAddressTokenTransfersParams::builder()
            .address_hash(address())
            .filter("to".to_owned())
            .r#type("ERC-1155".to_owned())
            .token(token())
            .build()
    );
    pages!(
        api,
        get_address_tokens,
        GetAddressTokensParams::builder()
            .address_hash(address())
            .r#type("ERC-721".to_owned())
            .build()
    );
    pages!(
        api,
        get_address_txs,
        GetAddressTxsParams::builder()
            .address_hash(address())
            .filter("to".to_owned())
            .build()
    );
    pages!(
        api,
        get_address_withdrawals,
        GetAddressWithdrawalsParams::builder()
            .address_hash(address())
            .build()
    );
    pages!(api, get_addresses);
}

#[tokio::test]
async fn block_cursors() {
    let client = client();
    let api = client.blocks_api();
    pages!(
        api,
        get_blocks,
        GetBlocksParams::builder()
            .r#type("block".to_owned())
            .build()
    );
    pages!(
        api,
        get_block_txs,
        GetBlockTxsParams::builder()
            .block_number_or_hash(block())
            .build()
    );
    pages!(
        api,
        get_block_withdrawals,
        GetBlockWithdrawalsParams::builder()
            .block_number_or_hash(block())
            .build()
    );
}

#[tokio::test]
async fn internal_cursors() {
    let client = client();
    pages!(
        client.internal_transactions_api(),
        get_internal_transactions
    );
}

#[tokio::test]
async fn transfer_cursors() {
    let client = client();
    pages!(client.token_transfers_api(), get_token_transfers);
}

#[tokio::test]
async fn token_cursors() {
    let client = client();
    let api = client.tokens_api();
    pages!(
        api,
        get_tokens_list,
        GetTokensListParams::builder()
            .q("USDT".to_owned())
            .r#type("ERC-1155".to_owned())
            .build()
    );
    pages!(
        api,
        get_token_holders,
        GetTokenHoldersParams::builder()
            .address_hash(token())
            .build()
    );
    pages!(
        api,
        get_token_instance_holders,
        GetTokenInstanceHoldersParams::builder()
            .address_hash(token())
            .id(instance())
            .build()
    );
    pages!(
        api,
        get_nft_instances,
        GetNftInstancesParams::builder()
            .address_hash(token())
            .build()
    );
    pages!(
        api,
        get_nft_instance_transfers,
        GetNftInstanceTransfersParams::builder()
            .address_hash(token())
            .id(instance())
            .build()
    );
    pages!(
        api,
        get_token_token_transfers,
        GetTokenTokenTransfersParams::builder()
            .address_hash(token())
            .build()
    );
}

#[tokio::test]
async fn transaction_cursors() {
    let client = client();
    let api = client.transactions_api();
    pages!(
        api,
        get_txs,
        GetTxsParams::builder()
            .filter("validated".to_owned())
            .r#type("coin_transfer".to_owned())
            .build()
    );
    pages!(
        api,
        get_transaction_internal_txs,
        GetTransactionInternalTxsParams::builder()
            .transaction_hash(transaction())
            .build()
    );
    pages!(
        api,
        get_transaction_logs,
        GetTransactionLogsParams::builder()
            .transaction_hash(transaction())
            .build()
    );
    pages!(
        api,
        get_transaction_state_changes,
        GetTransactionStateChangesParams::builder()
            .transaction_hash(transaction())
            .build()
    );
    pages!(
        api,
        get_transaction_token_transfers,
        GetTransactionTokenTransfersParams::builder()
            .transaction_hash(transaction())
            .r#type("ERC-1155".to_owned())
            .build()
    );
}

#[tokio::test]
async fn contract_cursors() {
    let client = client();
    pages!(
        client.smart_contracts_api(),
        get_smart_contracts,
        GetSmartContractsParams::builder()
            .q("CryptoPunks".to_owned())
            .filter("solidity".to_owned())
            .build()
    );
}

#[tokio::test]
async fn withdrawal_cursors() {
    let client = client();
    pages!(client.withdrawals_api(), get_withdrawals);
}

#[tokio::test]
async fn search_cursors() {
    let client = client();
    pages!(
        client.search_api(),
        search,
        SearchParams::builder().q("USDT".to_owned()).build()
    );
}

#[test]
fn cursor_codec_preserves_numbers_and_validates_input() {
    // Boundary probes exercise the real response and query codecs. These are
    // deliberately adversarial inputs, not substitute backend responses.
    let huge = "115792089237316195423570985008687907853269984665640564039457584007913129639935";
    let content = format!(
        r#"{{"items":[],"next_page_params":{{"unique_token":{huge},"token[name]":"A+B & café/東京","token[fiat_value]":null,"is_name_null":false,"index":1000003}}}}"#
    );
    let response = ResponseContent::<Value> {
        status: reqwest::StatusCode::OK,
        content,
        entity: None,
    };
    let cursor = response.next_page_cursor().unwrap().unwrap();
    let params: BTreeMap<_, _> = cursor.query_pairs().collect();
    assert_eq!(params["unique_token"], huge);
    assert_eq!(params["token[name]"], "A+B & café/東京");
    assert_eq!(params["token[fiat_value]"], "");
    assert_eq!(params["is_name_null"], "false");
    assert_eq!(params["index"], "1000003");
    let encoded = url::form_urlencoded::Serializer::new(String::new())
        .extend_pairs(cursor.query_pairs())
        .finish();
    let decoded: BTreeMap<_, _> = url::form_urlencoded::parse(encoded.as_bytes())
        .map(|(k, v)| (k.into_owned(), v.into_owned()))
        .collect();
    assert_eq!(decoded["unique_token"], huge);
    assert_eq!(decoded["token[name]"], "A+B & café/東京");
    let mut request = reqwest::Client::new()
        .get("http://localhost/api/v2/tokens")
        .query(&[("type", "ERC-721"), ("q", "original & filter")])
        .build()
        .unwrap();
    cursor.apply(&mut request).unwrap();
    let sent: BTreeMap<_, _> = request
        .url()
        .query_pairs()
        .map(|(k, v)| (k.into_owned(), v.into_owned()))
        .collect();
    assert_eq!(sent["type"], "ERC-721");
    assert_eq!(sent["q"], "original & filter");
    assert_eq!(sent["unique_token"], huge);
    assert_eq!(sent["token[name]"], "A+B & café/東京");
    let count = request.url().query_pairs().count();
    cursor.apply(&mut request).unwrap();
    assert_eq!(
        request.url().query_pairs().count(),
        count,
        "reapplying identical cursor must not duplicate query keys"
    );
    let mut duplicate_request = reqwest::Client::new()
        .get("http://localhost/api/v2/tokens?type=ERC-721&type=ERC-20")
        .build()
        .unwrap();
    assert_eq!(
        cursor.apply(&mut duplicate_request).unwrap_err().kind(),
        std::io::ErrorKind::InvalidInput
    );
    for bad in [
        "{}",
        "[]",
        r#"{"x":[]}"#,
        r#"{"x":{}}"#,
        r#"{"x":1,"x":2}"#,
        r#"{"bad&key":1}"#,
        r#"{"x":1} trailing"#,
    ] {
        assert!(
            PageCursor::from_json(bad).is_err(),
            "accepted invalid cursor: {bad}"
        );
    }
    assert!(PageCursor::from_json("null").unwrap().is_none());
    let duplicate = ResponseContent::<Value> {
        status: reqwest::StatusCode::OK,
        content: r#"{"next_page_params":null,"next_page_params":{"index":1}}"#.into(),
        entity: None,
    };
    assert!(duplicate.next_page_cursor().is_err());
}

#[tokio::test]
async fn conflicting_filters_are_rejected_before_transport() {
    let config = Configuration::builder()
        .base_path("http://127.0.0.1:1")
        .build();
    let client = ApiClient::new(config);
    let cursor = PageCursor::from_json(r#"{"q":"different","index":1}"#)
        .unwrap()
        .unwrap();
    let api = client.search_api().with_cursor(cursor);
    let result = api
        .search(SearchParams::builder().q("original".to_owned()).build())
        .await;
    match result {
        Err(blockscout_client::apis::Error::Io(error)) => {
            assert_eq!(error.kind(), std::io::ErrorKind::InvalidInput);
            assert!(error
                .to_string()
                .contains("conflicts with original filter q"));
        }
        other => panic!("conflicting cursor must fail before any network call: {other:?}"),
    }
}
