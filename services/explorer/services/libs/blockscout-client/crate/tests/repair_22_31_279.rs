use blockscout_client::{
    apis::{addresses_api::*, smart_contracts_api::*, tokens_api::*, transactions_api::*},
    Api, ApiClient, Configuration,
};
use blockscout_client::{
    apis::{
        addresses_api::{GetAddressLogsSuccess, GetAddressTokenTransfersSuccess},
        smart_contracts_api::GetSmartContractSuccess,
        tokens_api::{
            GetTokenHoldersSuccess, GetTokenInstanceHoldersSuccess, GetTokenTokenTransfersSuccess,
        },
        ResponseContent,
    },
    models::{TokenTransfer, TokenTransferTotal},
};
use serde::de::DeserializeOwned;
use serde_json::json;
use serde_json::Value;
use std::{collections::BTreeSet, time::Duration};
use std::{fs, path::Path};

const LOGS: &str = "_api_v2_addresses_0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3_logs.json";
const HOLDERS: &str = "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_holders.json";
const INSTANCE_HOLDERS: &str = "_api_v2_tokens_0xD4416b13_instances_25625468_holders.json";
const TOKEN_TRANSFERS: &str =
    "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_transfers.json";
const ADDRESS_TRANSFERS: &str =
    "_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_token-transfers.json";
const CONTRACT: &str = "_api_v2_smart-contracts_0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3.json";

fn recorded_response<T: DeserializeOwned>(name: &str) -> (Value, ResponseContent<T>) {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("tests/recorded/eth_blockscout_com")
        .join(name);
    let recording: Value = serde_json::from_str(&fs::read_to_string(&path).unwrap()).unwrap();
    assert_eq!(recording["request"]["method"], "GET");
    assert_eq!(recording["response"]["status"], 200);
    let content = recording["response"]["body"].as_str().unwrap().to_owned();
    let body = serde_json::from_str(&content).unwrap();
    let entity = serde_json::from_str(&content).expect("recorded response must decode");
    (
        body,
        ResponseContent {
            status: reqwest::StatusCode::OK,
            content,
            entity: Some(entity),
        },
    )
}

#[test]
fn log_emitter_preserves_origin_distinct_from_transaction_destination() {
    let (body, response) = recorded_response::<GetAddressLogsSuccess>(LOGS);
    let page = response
        .try_as_success()
        .expect("address logs must be a typed Status200 response");
    assert_eq!(page.items.len(), body["items"].as_array().unwrap().len());
    assert_eq!(page.items.len(), 12);
    let mut distinct_origin_seen = false;
    for (log, raw) in page.items.iter().zip(body["items"].as_array().unwrap()) {
        let emitter = raw["address"]["hash"].as_str().unwrap();
        let destination = raw["smart_contract"]["hash"].as_str().unwrap();
        distinct_origin_seen |= emitter != destination;
        assert_eq!(
            log.address_hash
                .as_ref()
                .map(|address| address.hash.as_str()),
            Some(emitter),
            "the producer's address must survive decoding independently of smart_contract"
        );
        assert_eq!(log.transaction_hash, raw["transaction_hash"]);
        assert_eq!(log.data, raw["data"]);
        assert_eq!(serde_json::to_value(&log.topics).unwrap(), raw["topics"]);
    }
    assert!(distinct_origin_seen);
    assert_eq!(response.try_parse_as_success_model().unwrap(), *page);
}

fn assert_holders(holders: &[blockscout_client::models::Holder], raw: &Value) {
    let expected = raw["items"].as_array().unwrap();
    assert_eq!(holders.len(), expected.len());
    assert!(!holders.is_empty());
    for (holder, item) in holders.iter().zip(expected) {
        assert_eq!(
            holder
                .address_hash
                .as_ref()
                .map(|address| address.hash.as_str()),
            item["address"]["hash"].as_str(),
            "a balance must retain its checksum holder identity"
        );
        let token_id = item["token_id"].as_str().unwrap();
        assert!(token_id.len() > 64);
        assert_eq!(holder.token_id.as_deref(), Some(token_id));
        assert_eq!(holder.value, item["value"].as_str().unwrap());
        assert!(!holder.value.is_empty());
        assert!(holder.value.bytes().all(|byte| byte.is_ascii_digit()));
    }
}

#[test]
fn token_holders_preserve_checksum_address_token_id_and_base_units() {
    let (body, response) = recorded_response::<GetTokenHoldersSuccess>(HOLDERS);
    let page = response
        .try_as_success()
        .expect("token holders must be a typed Status200 response");
    assert_eq!(page.items.len(), 50);
    assert_holders(&page.items, &body);
    assert_eq!(response.try_parse_as_success_model().unwrap(), *page);
}

#[test]
fn instance_holders_preserve_checksum_address_token_id_and_base_units() {
    let (body, response) = recorded_response::<GetTokenInstanceHoldersSuccess>(INSTANCE_HOLDERS);
    let page = response
        .try_as_success()
        .expect("instance holders must be a typed Status200 response");
    assert_eq!(page.items.len(), 1);
    assert_holders(&page.items, &body);
    assert_eq!(
        page.items[0].token_id.as_deref(),
        Some("25625468407840116393736812939389551247551040926951238633020744494000165263268")
    );
    assert_eq!(response.try_parse_as_success_model().unwrap(), *page);
}

fn assert_transfer_total(transfer: &TokenTransfer, raw: &Value) {
    assert_eq!(transfer.transaction_hash, raw["transaction_hash"]);
    let total = &raw["total"];
    let decoded_total = transfer
        .total
        .as_ref()
        .expect("recorded transfer total must be present");
    match (raw["token"]["type"].as_str().unwrap(), decoded_total) {
        ("ERC-20", TokenTransferTotal::TotalErc20(decoded)) => {
            assert!(total.get("token_id").is_none());
            assert_eq!(decoded.value.as_deref(), total["value"].as_str());
            assert_eq!(decoded.decimals.as_deref(), total["decimals"].as_str());
        }
        ("ERC-721", TokenTransferTotal::TotalErc721(decoded)) => {
            assert_eq!(
                decoded.token_id.as_deref(),
                Some(total["token_id"].as_str().unwrap())
            );
            let instance = decoded
                .token_instance
                .as_ref()
                .expect("missing NFT instance");
            assert_eq!(instance.id, total["token_instance"]["id"].as_str().unwrap());
            assert_eq!(Some(instance.id.as_str()), decoded.token_id.as_deref());
        }
        ("ERC-1155", TokenTransferTotal::TotalErc1155(decoded)) => {
            assert_eq!(
                decoded.token_id.as_deref(),
                Some(total["token_id"].as_str().unwrap())
            );
            let instance = decoded
                .token_instance
                .as_ref()
                .expect("missing NFT instance");
            assert_eq!(instance.id, total["token_instance"]["id"].as_str().unwrap());
            assert_eq!(Some(instance.id.as_str()), decoded.token_id.as_deref());
            let quantity = total["value"].as_str().unwrap();
            assert!(!quantity.is_empty());
            assert!(quantity.bytes().all(|byte| byte.is_ascii_digit()));
            assert_eq!(decoded.value.as_deref(), Some(quantity));
            assert_eq!(decoded.decimals.as_deref(), total["decimals"].as_str());
        }
        (kind, decoded) => panic!("{kind} was decoded as the wrong total variant: {decoded:?}"),
    }
}

#[test]
fn token_transfers_preserve_erc1155_variant_instance_and_quantity() {
    let (body, response) = recorded_response::<GetTokenTokenTransfersSuccess>(TOKEN_TRANSFERS);
    let page = response
        .try_as_success()
        .expect("token transfers must be a typed Status200 response");
    assert_eq!(page.items.len(), 50);
    assert_eq!(page.items.len(), body["items"].as_array().unwrap().len());
    for (transfer, raw) in page.items.iter().zip(body["items"].as_array().unwrap()) {
        assert_eq!(raw["token"]["type"], "ERC-1155");
        assert_transfer_total(transfer, raw);
    }
    assert_eq!(response.try_parse_as_success_model().unwrap(), *page);
}

#[test]
fn address_transfers_preserve_fungible_and_erc1155_variants() {
    let (body, response) = recorded_response::<GetAddressTokenTransfersSuccess>(ADDRESS_TRANSFERS);
    let page = response
        .try_as_success()
        .expect("address transfers must be a typed Status200 response");
    assert_eq!(page.items.len(), body["items"].as_array().unwrap().len());
    let expected = body["items"].as_array().unwrap();
    assert_eq!(
        expected
            .iter()
            .filter(|item| item["token"]["type"] == "ERC-20")
            .count(),
        8
    );
    assert_eq!(
        expected
            .iter()
            .filter(|item| item["token"]["type"] == "ERC-1155")
            .count(),
        5
    );
    for (transfer, raw) in page.items.iter().zip(expected) {
        assert_transfer_total(transfer, raw);
    }
    assert_eq!(response.try_parse_as_success_model().unwrap(), *page);
}

#[test]
fn verified_contract_is_typed_with_structured_abi_and_optimization_runs() {
    let (body, response) = recorded_response::<GetSmartContractSuccess>(CONTRACT);
    let expected_abi = body["abi"].as_array().unwrap();
    assert_eq!(expected_abi.len(), 36);
    assert_eq!(body["optimization_runs"], 800);
    assert!(expected_abi
        .iter()
        .any(|entry| entry["type"] == "function" && entry["name"] == "burn"));
    let contract = response
        .try_as_success()
        .expect("verified contract must decode as Status200, never UnknownValue");
    let decoded = serde_json::to_value(contract).unwrap();
    assert_eq!(
        decoded["abi"]
            .as_array()
            .expect("ABI must remain structured"),
        expected_abi
    );
    assert_eq!(decoded["optimization_runs"], body["optimization_runs"]);
    assert_eq!(response.try_parse_as_success_model().unwrap(), *contract);
}

// The production component under repair is the Rust client. Its HTTP peer
// replays committed explorer recordings and producer-schema branch fixtures.
// No Elixir controller execution is claimed by this offline gate.
fn transcript(name: &str) -> Value {
    serde_json::from_str(
        &fs::read_to_string(
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("tests/recorded/eth_blockscout_com")
                .join(name),
        )
        .unwrap(),
    )
    .unwrap()
}

fn producer_cases() -> Value {
    serde_json::from_str(include_str!("repair_22_31_279_cases.json")).unwrap()
}

struct Exchange {
    path: String,
    query: Vec<(String, String)>,
    body: String,
}

impl Exchange {
    fn recorded(name: &str) -> Self {
        let recording = transcript(name);
        assert_eq!(recording["request"]["method"], "GET");
        assert_eq!(recording["response"]["status"], 200);
        Self {
            path: recording["request"]["urlPath"].as_str().unwrap().to_owned(),
            query: vec![],
            body: recording["response"]["body"].as_str().unwrap().to_owned(),
        }
    }

    fn with_body(mut self, body: &Value) -> Self {
        self.body = serde_json::to_string(body).unwrap();
        self
    }
}

struct Loopback {
    base_path: String,
    server: Option<tokio::task::JoinHandle<()>>,
}

impl Loopback {
    async fn start(exchanges: Vec<Exchange>) -> Self {
        use tokio::io::{AsyncReadExt, AsyncWriteExt};
        let listener = tokio::net::TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0))
            .await
            .unwrap();
        let base_path = format!("http://{}", listener.local_addr().unwrap());
        let server = tokio::spawn(async move {
            for mut expected in exchanges {
                let (mut socket, _) =
                    tokio::time::timeout(Duration::from_secs(10), listener.accept())
                        .await
                        .expect("the client must consume every fixture exchange")
                        .unwrap();
                let mut request = Vec::new();
                let mut buffer = [0; 4096];
                while !request.windows(4).any(|window| window == b"\r\n\r\n") {
                    let count =
                        tokio::time::timeout(Duration::from_secs(5), socket.read(&mut buffer))
                            .await
                            .unwrap()
                            .unwrap();
                    assert!(count > 0, "incomplete HTTP request");
                    request.extend_from_slice(&buffer[..count]);
                    assert!(request.len() <= 65536);
                }
                let request = std::str::from_utf8(&request).unwrap();
                let mut line = request.lines().next().unwrap().split_whitespace();
                assert_eq!(line.next(), Some("GET"));
                let url =
                    url::Url::parse(&format!("http://localhost{}", line.next().unwrap())).unwrap();
                assert_eq!(url.path(), expected.path);
                let mut query: Vec<_> = url
                    .query_pairs()
                    .map(|(key, value)| (key.into_owned(), value.into_owned()))
                    .collect();
                query.sort();
                expected.query.sort();
                // Comparing vectors also rejects duplicated cursor/filter keys.
                assert_eq!(
                    query, expected.query,
                    "wrong cursor or filter for {}",
                    expected.path
                );
                let body = expected.body;
                let response = format!(
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                socket.write_all(response.as_bytes()).await.unwrap();
                socket.shutdown().await.unwrap();
            }
        });
        Self {
            base_path,
            server: Some(server),
        }
    }

    fn client(&self) -> ApiClient {
        let http = reqwest::Client::builder()
            .no_proxy()
            .timeout(Duration::from_secs(10))
            .build()
            .unwrap();
        let config = Configuration::builder()
            .base_path(self.base_path.clone())
            .client(reqwest_middleware::ClientBuilder::new(http).build())
            .build();
        ApiClient::new(config)
    }

    async fn finish(mut self) {
        tokio::time::timeout(Duration::from_secs(10), self.server.take().unwrap())
            .await
            .unwrap()
            .expect("HTTP fixture assertions must pass");
    }
}

impl Drop for Loopback {
    fn drop(&mut self) {
        if let Some(server) = self.server.take() {
            server.abort();
        }
    }
}

fn route_identifier(name: &str, segment: usize) -> String {
    transcript(name)["request"]["urlPath"]
        .as_str()
        .unwrap()
        .split('/')
        .nth(segment)
        .unwrap()
        .to_owned()
}

#[tokio::test]
async fn recorded_routes_preserve_emitter_holders_totals_and_abi() {
    let peer = Loopback::start(
        [
            LOGS,
            HOLDERS,
            INSTANCE_HOLDERS,
            TOKEN_TRANSFERS,
            ADDRESS_TRANSFERS,
            CONTRACT,
        ]
        .into_iter()
        .map(Exchange::recorded)
        .collect(),
    )
    .await;
    let client = peer.client();
    let logs = client
        .addresses_api()
        .get_address_logs(
            GetAddressLogsParams::builder()
                .address_hash(route_identifier(LOGS, 4))
                .build(),
        )
        .await
        .unwrap();
    let page = logs.try_as_success().expect("typed logs over HTTP");
    let raw: Value = serde_json::from_str(&logs.content).unwrap();
    assert_eq!(page.items.len(), 12);
    let mut distinct = false;
    for (log, item) in page.items.iter().zip(raw["items"].as_array().unwrap()) {
        assert_eq!(
            log.address_hash.as_ref().unwrap().hash,
            item["address"]["hash"]
        );
        distinct |= item["address"]["hash"] != item["smart_contract"]["hash"];
        assert_eq!(log.transaction_hash, item["transaction_hash"]);
    }
    assert!(distinct);
    let holders = client
        .tokens_api()
        .get_token_holders(
            GetTokenHoldersParams::builder()
                .address_hash(route_identifier(HOLDERS, 4))
                .build(),
        )
        .await
        .unwrap();
    assert_holders(
        &holders.try_as_success().unwrap().items,
        &serde_json::from_str(&holders.content).unwrap(),
    );
    let holders = client
        .tokens_api()
        .get_token_instance_holders(
            GetTokenInstanceHoldersParams::builder()
                .address_hash(route_identifier(INSTANCE_HOLDERS, 4))
                .id(route_identifier(INSTANCE_HOLDERS, 6))
                .build(),
        )
        .await
        .unwrap();
    assert_holders(
        &holders.try_as_success().unwrap().items,
        &serde_json::from_str(&holders.content).unwrap(),
    );
    let transfers = client
        .tokens_api()
        .get_token_token_transfers(
            GetTokenTokenTransfersParams::builder()
                .address_hash(route_identifier(TOKEN_TRANSFERS, 4))
                .build(),
        )
        .await
        .unwrap();
    let body: Value = serde_json::from_str(&transfers.content).unwrap();
    assert_eq!(transfers.try_as_success().unwrap().items.len(), 50);
    for (transfer, raw) in transfers
        .try_as_success()
        .unwrap()
        .items
        .iter()
        .zip(body["items"].as_array().unwrap())
    {
        assert_transfer_total(transfer, raw);
    }
    let transfers = client
        .addresses_api()
        .get_address_token_transfers(
            GetAddressTokenTransfersParams::builder()
                .address_hash(route_identifier(ADDRESS_TRANSFERS, 4))
                .build(),
        )
        .await
        .unwrap();
    let body: Value = serde_json::from_str(&transfers.content).unwrap();
    assert_eq!(transfers.try_as_success().unwrap().items.len(), 13);
    for (transfer, raw) in transfers
        .try_as_success()
        .unwrap()
        .items
        .iter()
        .zip(body["items"].as_array().unwrap())
    {
        assert_transfer_total(transfer, raw);
    }
    let response = client
        .smart_contracts_api()
        .get_smart_contract(
            GetSmartContractParams::builder()
                .address_hash(route_identifier(CONTRACT, 4))
                .build(),
        )
        .await
        .unwrap();
    let contract = response
        .try_as_success()
        .expect("HTTP 200 must contain a typed contract");
    let abi = contract.abi.as_ref().unwrap();
    assert_eq!(abi.len(), 36);
    assert!(abi
        .iter()
        .any(|entry| entry["type"] == "function" && entry["name"] == "burn"));
    assert_eq!(
        contract.optimization_runs,
        Some(rust_decimal::Decimal::from(800))
    );
    let raw: Value = serde_json::from_str(&response.content).unwrap();
    assert_eq!(serde_json::to_value(contract).unwrap()["abi"], raw["abi"]);
    peer.finish().await;
}

#[tokio::test]
async fn nullable_logs_preserve_raw_identity_over_addresses_http() {
    let body = &producer_cases()["logs"];
    let peer = Loopback::start(vec![Exchange::recorded(LOGS).with_body(body)]).await;
    let response = peer
        .client()
        .addresses_api()
        .get_address_logs(
            GetAddressLogsParams::builder()
                .address_hash(route_identifier(LOGS, 4))
                .build(),
        )
        .await
        .unwrap();
    let page = response
        .try_as_success()
        .expect("raw event must not reject its page");
    assert_eq!(page.items.len(), 12);
    assert!(page.items[0].decoded.is_none());
    assert!(page.items[0].smart_contract.is_none());
    assert!(page.items[1].decoded.is_some());
    for (log, raw) in page.items.iter().zip(body["items"].as_array().unwrap()) {
        assert_eq!(
            log.address_hash.as_ref().unwrap().hash,
            raw["address"]["hash"]
        );
        assert_eq!(log.data, raw["data"]);
        assert_eq!(log.transaction_hash, raw["transaction_hash"]);
        assert_eq!(serde_json::to_value(&log.topics).unwrap(), raw["topics"]);
    }
    peer.finish().await;
}

#[tokio::test]
async fn all_transfer_standards_survive_both_http_routes() {
    let cases = producer_cases();
    let body = &cases["transfers"];
    let items = body["items"].as_array().unwrap();
    let mut exchanges = Vec::new();
    for item in items {
        let token = item["token"]["address_hash"].as_str().unwrap();
        exchanges.push(Exchange {
            path: format!("/api/v2/tokens/{token}/transfers"),
            query: vec![],
            body: json!({"items": [item], "next_page_params": null}).to_string(),
        });
    }
    exchanges.push(Exchange::recorded(ADDRESS_TRANSFERS).with_body(body));
    let peer = Loopback::start(exchanges).await;
    let client = peer.client();
    for (item, standard) in items.iter().zip(["ERC-20", "ERC-721", "ERC-1155"]) {
        assert_eq!(item["token"]["type"], standard);
        let response = client
            .tokens_api()
            .get_token_token_transfers(
                GetTokenTokenTransfersParams::builder()
                    .address_hash(item["token"]["address_hash"].as_str().unwrap().to_owned())
                    .build(),
            )
            .await
            .unwrap();
        let page = response
            .try_as_success()
            .expect("each standard must stay typed");
        assert_eq!(page.items.len(), 1);
        assert_transfer_total(&page.items[0], item);
    }
    let response = client
        .addresses_api()
        .get_address_token_transfers(
            GetAddressTokenTransfersParams::builder()
                .address_hash(route_identifier(ADDRESS_TRANSFERS, 4))
                .build(),
        )
        .await
        .unwrap();
    let page = response.try_as_success().unwrap();
    assert_eq!(page.items.len(), 3);
    for (decoded, item) in page.items.iter().zip(items) {
        assert_transfer_total(decoded, item);
    }
    peer.finish().await;
}

#[test]
fn nft_totals_reject_incomplete_producer_structures() {
    let cases = producer_cases();
    for item in &cases["transfers"]["items"].as_array().unwrap()[1..] {
        let total = &item["total"];
        assert!(serde_json::from_value::<TokenTransferTotal>(total.clone()).is_ok());
        for field in total.as_object().unwrap().keys() {
            let mut invalid = total.clone();
            invalid.as_object_mut().unwrap().remove(field);
            assert!(
                serde_json::from_value::<TokenTransferTotal>(invalid).is_err(),
                "accepted missing {field}"
            );
        }
        let mut malformed = total.clone();
        malformed["token_id"] = json!({"id": "wrong shape"});
        assert!(serde_json::from_value::<TokenTransferTotal>(malformed).is_err());
    }
    // Producer properties may be present with null values; this is distinct
    // from a missing property and must keep the NFT variant.
    let nullable: TokenTransferTotal = serde_json::from_value(
        json!({"token_id": null, "token_instance": null, "value": null, "decimals": null}),
    )
    .unwrap();
    assert!(matches!(nullable, TokenTransferTotal::TotalErc1155(_)));
}

const INTERNAL: &str = "_api_v2_transactions_0x4dd7e3f4_internal-transactions.json";
const TRANSACTION: &str = "_api_v2_transactions_0x4dd7e3f4.json";

#[tokio::test]
async fn internal_creation_and_failed_calls_survive_transactions_http() {
    let cases = producer_cases();
    let body = &cases["internal_transactions"];
    let peer = Loopback::start(vec![Exchange::recorded(INTERNAL).with_body(body)]).await;
    let response = peer
        .client()
        .transactions_api()
        .get_transaction_internal_txs(
            GetTransactionInternalTxsParams::builder()
                .transaction_hash(route_identifier(INTERNAL, 4))
                .build(),
        )
        .await
        .unwrap();
    let page = response
        .try_as_success()
        .expect("creation must not reject its trace page");
    assert_eq!(page.items.len(), 3);
    assert!(page.items[0].to.is_some());
    let creation = &page.items[1];
    assert_eq!(creation.r#type, "create");
    assert!(creation.to.is_none());
    assert_eq!(
        creation.created_contract.as_ref().unwrap().hash,
        body["items"][1]["created_contract"]["hash"]
    );
    assert!(creation.success);
    assert!(creation.error.is_none());
    let failed = &page.items[2];
    assert!(!failed.success);
    assert_eq!(failed.error.as_deref(), Some("out of gas"));
    assert!(failed.to.is_some());
    assert!(failed.created_contract.is_none());
    for (trace, raw) in page.items.iter().zip(body["items"].as_array().unwrap()) {
        assert_eq!(trace.transaction_hash, raw["transaction_hash"]);
        assert_eq!(trace.value, raw["value"]);
        assert_eq!(trace.gas_limit, raw["gas_limit"]);
        assert_eq!(trace.from.hash, raw["from"]["hash"]);
    }
    peer.finish().await;
}

fn assert_transaction_lifecycle(
    tx: &blockscout_client::models::Transaction,
    raw: &Value,
    committed: bool,
) {
    assert_eq!(tx.hash, raw["hash"]);
    assert_eq!(tx.from.hash, raw["from"]["hash"]);
    assert_eq!(
        tx.status.as_deref(),
        if committed { Some("ok") } else { None }
    );
    assert_eq!(tx.result, if committed { "success" } else { "pending" });
    assert_eq!(tx.block_number.is_some(), committed);
    assert_eq!(tx.timestamp.is_some(), committed);
    assert_eq!(tx.gas_used.is_some(), committed);
    assert_eq!(tx.position.is_some(), committed);
    assert_eq!(tx.base_fee_per_gas.is_some(), committed);
    assert_eq!(tx.gas_price.is_some(), committed);
    assert_eq!(tx.r#type.is_some(), committed);
    assert_eq!(tx.actions.is_some(), committed);
    assert!(tx.to.is_none());
    assert!(tx.exchange_rate.is_none());
    assert_eq!(tx.created_contract.is_some(), committed);
    if committed {
        assert_eq!(
            tx.created_contract.as_ref().unwrap().hash,
            raw["created_contract"]["hash"]
        );
        assert_eq!(
            tx.block_number.unwrap().to_string(),
            raw["block_number"].to_string()
        );
        assert_eq!(tx.gas_used.as_deref(), raw["gas_used"].as_str());
    }
}

#[tokio::test]
async fn transactions_pending_to_committed_creation_without_pricing() {
    use blockscout_client::apis::feed::FeedItem;
    let cases = producer_cases();
    let mut exchanges = Vec::new();
    for state in ["pending_transaction", "committed_transaction"] {
        exchanges.push(Exchange::recorded(TRANSACTION).with_body(&cases[state]));
        exchanges.push(Exchange {
            path: "/api/v2/transactions".to_owned(),
            query: vec![],
            body: json!({"items": [&cases[state]], "next_page_params": null}).to_string(),
        });
    }
    let peer = Loopback::start(exchanges).await;
    let mut pending_hash = None;
    for (state, committed) in [
        ("pending_transaction", false),
        ("committed_transaction", true),
    ] {
        // Recreate the consumer and its HTTP pool between lifecycle states.
        let client = peer.client();
        let response = client
            .transactions_api()
            .get_tx(
                GetTxParams::builder()
                    .transaction_hash(route_identifier(TRANSACTION, 4))
                    .build(),
            )
            .await
            .unwrap();
        let tx = response
            .try_as_success()
            .expect("lifecycle detail must remain typed");
        assert_transaction_lifecycle(tx, &cases[state], committed);
        if committed {
            assert_eq!(pending_hash.as_deref(), Some(tx.hash.as_str()));
        } else {
            pending_hash = Some(tx.hash.clone());
        }
        let list = client
            .transactions_api()
            .get_txs(GetTxsParams::builder().build())
            .await
            .unwrap();
        let page = list
            .try_as_success()
            .expect("lifecycle list must remain typed");
        assert_eq!(page.items.len(), 1);
        match &page.items[0] {
            FeedItem::Evm(list_tx) => {
                assert_transaction_lifecycle(list_tx, &cases[state], committed);
                assert_eq!(list_tx.as_ref(), tx);
            }
            other => panic!("creation was decoded as a native transaction: {other:?}"),
        }
    }
    peer.finish().await;
}

#[derive(Clone, Copy)]
enum PageIdentity {
    Block,
    Log,
    Holder,
    Instance,
    Nft,
    Collection,
    Transfer,
    Token,
    Internal,
}

fn scalar(value: &Value) -> String {
    match value {
        Value::String(value) => value.clone(),
        Value::Null => String::new(),
        Value::Number(_) | Value::Bool(_) => value.to_string(),
        other => panic!("expected scalar fixture identity, got {other}"),
    }
}

impl PageIdentity {
    fn key(self, item: &Value) -> String {
        let fields = match self {
            Self::Block => vec![&item["hash"]],
            Self::Log => vec![&item["transaction_hash"], &item["index"]],
            Self::Holder => vec![&item["address"]["hash"], &item["token_id"]],
            Self::Instance => vec![&item["id"]],
            Self::Nft => vec![&item["token"]["address_hash"], &item["id"]],
            Self::Collection => vec![&item["token"]["address_hash"]],
            Self::Token => vec![&item["token"]["address_hash"], &item["token_id"]],
            Self::Transfer => vec![
                &item["transaction_hash"],
                &item["log_index"],
                &item["total"]["token_id"],
            ],
            Self::Internal => vec![&item["transaction_hash"], &item["index"]],
        };
        let fields: Vec<_> = fields.into_iter().map(scalar).collect();
        assert!(
            !fields[0].is_empty(),
            "a decoded item must retain its identity"
        );
        fields.join(":")
    }
}

struct PagePlan {
    exchanges: Vec<Exchange>,
    expected: Vec<String>,
    identity: PageIdentity,
}

impl PagePlan {
    fn recorded(
        name: &str,
        cursor: Value,
        filters: &[(&str, &str)],
        identity: PageIdentity,
    ) -> Self {
        let exchange = Exchange::recorded(name);
        let body = serde_json::from_str(&exchange.body).unwrap();
        Self::new(
            exchange.path,
            body,
            cursor.to_string(),
            cursor
                .as_object()
                .unwrap()
                .iter()
                .map(|(key, value)| (key.clone(), scalar(value)))
                .collect(),
            filters,
            identity,
        )
    }

    fn new(
        path: String,
        body: Value,
        cursor: String,
        cursor_query: Vec<(String, String)>,
        filters: &[(&str, &str)],
        identity: PageIdentity,
    ) -> Self {
        let items = body["items"].as_array().unwrap();
        assert!(!items.is_empty());
        let expected = items.iter().map(|item| identity.key(item)).collect();
        let split = items.len().div_ceil(2);
        let encode = |items: &[Value], cursor: &str| {
            format!(
                "{{\"items\":{},\"next_page_params\":{cursor}}}",
                serde_json::to_string(items).unwrap()
            )
        };
        let query: Vec<_> = filters
            .iter()
            .map(|(key, value)| (key.to_string(), value.to_string()))
            .collect();
        let mut next_query = query.clone();
        for pair in cursor_query {
            assert!(
                !next_query.iter().any(|(key, _)| key == &pair.0),
                "fixture cursor must not duplicate a filter"
            );
            next_query.push(pair);
        }
        Self {
            exchanges: vec![
                Exchange {
                    path: path.clone(),
                    query,
                    body: encode(&items[..split], &cursor),
                },
                Exchange {
                    path,
                    query: next_query,
                    body: encode(&items[split..], "null"),
                },
            ],
            expected,
            identity,
        }
    }
}

macro_rules! read_all_pages {
    ($plan:expr, $api:ident, $operation:ident, $builder:expr) => {{
        let PagePlan {
            exchanges,
            expected,
            identity,
        } = $plan;
        let peer = Loopback::start(exchanges).await;
        let client = peer.client();
        let first = client.$api().$operation($builder.build()).await.unwrap();
        let page = first
            .try_as_success()
            .expect("first page must remain typed");
        let mut actual: Vec<_> = page
            .items
            .iter()
            .map(|item| identity.key(&serde_json::to_value(item).unwrap()))
            .collect();
        let cursor = first
            .next_page_cursor()
            .unwrap()
            .expect("fixture has a continuation");
        drop(client);
        // Both the generated client and its HTTP pool are recreated, so the
        // continuation must come entirely from the returned producer cursor.
        let client = peer.client();
        let next = client
            .$api()
            .$operation($builder.cursor(cursor).build())
            .await
            .unwrap();
        let page = next
            .try_as_success()
            .expect("second page must remain typed");
        actual.extend(
            page.items
                .iter()
                .map(|item| identity.key(&serde_json::to_value(item).unwrap())),
        );
        assert!(next.next_page_cursor().unwrap().is_none());
        assert_eq!(
            actual, expected,
            "every recorded item must arrive once and in order"
        );
        assert_eq!(
            actual.iter().collect::<BTreeSet<_>>().len(),
            actual.len(),
            "pagination duplicated an identity"
        );
        peer.finish().await;
    }};
}

const BLOCKS_VALIDATED: &str =
    "_api_v2_addresses_0x4838B106FCe9647Bdf1E7877BF73cE8B0BAD5f97_blocks-validated.json";
const INSTANCES: &str = "_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_instances.json";
const COLLECTIONS: &str =
    "_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_nft_collections.json";
const ADDRESS_TOKENS: &str =
    "_api_v2_addresses_0xc0de20a37e2dac848f81a93bd85fe4acdde7c0de_tokens.json";

#[tokio::test]
async fn every_declared_paginated_route_continues_after_client_recreation() {
    let body = |name: &str| {
        serde_json::from_str::<Value>(transcript(name)["response"]["body"].as_str().unwrap())
            .unwrap()
    };
    let block = &body(BLOCKS_VALIDATED)["items"][24];
    read_all_pages!(
        PagePlan::recorded(
            BLOCKS_VALIDATED,
            json!({"block_number":block["height"],"items_count":25}),
            &[],
            PageIdentity::Block
        ),
        addresses_api,
        get_address_blocks_validated,
        GetAddressBlocksValidatedParams::builder()
            .address_hash(route_identifier(BLOCKS_VALIDATED, 4))
    );
    let log = &body(LOGS)["items"][5];
    read_all_pages!(
        PagePlan::recorded(
            LOGS,
            json!({"block_number":log["block_number"],"index":log["index"],"items_count":6}),
            &[],
            PageIdentity::Log
        ),
        addresses_api,
        get_address_logs,
        GetAddressLogsParams::builder().address_hash(route_identifier(LOGS, 4))
    );
    let collections = body(COLLECTIONS);
    let collection = &collections["items"][0];
    let nft_items: Vec<_> = collection["token_instances"]
        .as_array()
        .unwrap()
        .iter()
        .map(|item| {
            // AddressView.prepare_nft adds the enclosing token to each instance.
            let mut item = item.clone();
            item["token"] = collection["token"].clone();
            item
        })
        .collect();
    let nft_cursor = json!({"token_contract_address_hash":collection["token"]["address_hash"],"token_type":collection["token"]["type"],"items_count":1,"token_id":nft_items[0]["id"]});
    let nft_path = format!("/api/v2/addresses/{}/nft", route_identifier(COLLECTIONS, 4));
    read_all_pages!(
        PagePlan::new(
            nft_path,
            json!({"items":nft_items}),
            nft_cursor.to_string(),
            nft_cursor
                .as_object()
                .unwrap()
                .iter()
                .map(|(key, value)| (key.clone(), scalar(value)))
                .collect(),
            &[("type", "ERC-1155")],
            PageIdentity::Nft
        ),
        addresses_api,
        get_address_nft,
        GetAddressNftParams::builder()
            .address_hash(route_identifier(COLLECTIONS, 4))
            .r#type("ERC-1155".to_owned())
    );
    read_all_pages!(
        PagePlan::recorded(
            COLLECTIONS,
            json!({"token_contract_address_hash":collection["token"]["address_hash"],"token_type":collection["token"]["type"],"items_count":1}),
            &[("type", "ERC-1155")],
            PageIdentity::Collection
        ),
        addresses_api,
        get_address_nft_collections,
        GetAddressNftCollectionsParams::builder()
            .address_hash(route_identifier(COLLECTIONS, 4))
            .r#type("ERC-1155".to_owned())
    );
    let transfer = &body(ADDRESS_TRANSFERS)["items"][6];
    // Address transfers have no block_number in the client model; the cursor
    // still transports the producer's block/index fields without dropping them.
    read_all_pages!(
        PagePlan::recorded(
            ADDRESS_TRANSFERS,
            json!({"block_number":transfer["block_number"],"index":transfer["log_index"],"items_count":7}),
            &[("filter", "to")],
            PageIdentity::Transfer
        ),
        addresses_api,
        get_address_token_transfers,
        GetAddressTokenTransfersParams::builder()
            .address_hash(route_identifier(ADDRESS_TRANSFERS, 4))
            .filter("to".to_owned())
    );
    read_all_pages!(
        PagePlan::recorded(
            ADDRESS_TOKENS,
            json!({"fiat_value":null,"id":123,"value":"1","items_count":3}),
            &[],
            PageIdentity::Token
        ),
        addresses_api,
        get_address_tokens,
        GetAddressTokensParams::builder().address_hash(route_identifier(ADDRESS_TOKENS, 4))
    );
    let holder = &body(HOLDERS)["items"][24];
    read_all_pages!(
        PagePlan::recorded(
            HOLDERS,
            json!({"address_hash":holder["address"]["hash"].as_str().unwrap().to_lowercase(),"value":holder["value"],"items_count":25}),
            &[],
            PageIdentity::Holder
        ),
        tokens_api,
        get_token_holders,
        GetTokenHoldersParams::builder().address_hash(route_identifier(HOLDERS, 4))
    );
    let holder = &body(INSTANCE_HOLDERS)["items"][0];
    read_all_pages!(
        PagePlan::recorded(
            INSTANCE_HOLDERS,
            json!({"address_hash":holder["address"]["hash"],"value":holder["value"],"items_count":1}),
            &[],
            PageIdentity::Holder
        ),
        tokens_api,
        get_token_instance_holders,
        GetTokenInstanceHoldersParams::builder()
            .address_hash(route_identifier(INSTANCE_HOLDERS, 4))
            .id(route_identifier(INSTANCE_HOLDERS, 6))
    );
    let instances = body(INSTANCES);
    let full_id = instances["items"][24]["id"].as_str().unwrap();
    assert!(full_id.len() > 64);
    // Preserve the full numeric uint256 spelling, before any JSON Value/f64
    // conversion can round it. The HTTP peer compares every query byte.
    read_all_pages!(
        PagePlan::new(
            Exchange::recorded(INSTANCES).path,
            instances.clone(),
            format!("{{\"unique_token\":{full_id}}}"),
            vec![("unique_token".to_owned(), full_id.to_owned())],
            &[],
            PageIdentity::Instance
        ),
        tokens_api,
        get_nft_instances,
        GetNftInstancesParams::builder().address_hash(route_identifier(INSTANCES, 4))
    );
    let transfer = &body(TOKEN_TRANSFERS)["items"][24];
    read_all_pages!(
        PagePlan::recorded(
            TOKEN_TRANSFERS,
            json!({"block_number":transfer["block_number"],"index":transfer["log_index"]}),
            &[],
            PageIdentity::Transfer
        ),
        tokens_api,
        get_token_token_transfers,
        GetTokenTokenTransfersParams::builder().address_hash(route_identifier(TOKEN_TRANSFERS, 4))
    );
    let traces = producer_cases()["internal_transactions"].clone();
    let cursor = json!({"block_number":traces["items"][1]["block_number"],"transaction_index":134,"index":traces["items"][1]["index"],"items_count":2});
    read_all_pages!(
        PagePlan::new(
            Exchange::recorded(INTERNAL).path,
            traces,
            cursor.to_string(),
            cursor
                .as_object()
                .unwrap()
                .iter()
                .map(|(key, value)| (key.clone(), scalar(value)))
                .collect(),
            &[],
            PageIdentity::Internal
        ),
        transactions_api,
        get_transaction_internal_txs,
        GetTransactionInternalTxsParams::builder().transaction_hash(route_identifier(INTERNAL, 4))
    );
}
