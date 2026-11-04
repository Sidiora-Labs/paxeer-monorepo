use blockscout_client::{Api, ApiClient, Configuration};
use pretty_assertions::assert_eq;
use rstest::*;
use std::time::Duration;

mod support;
use support::OwnedApiClient;

const DEFAULT_TOKEN_HASH: &str = "0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401";
const DEFAULT_CONTRACT_HASH: &str = "0x8FD4596d4E7788a71F82dAf4119D069a84E7d3f3";
const DEFAULT_TOKEN_INSTANCE_NUMBER: &str =
    "25625468407840116393736812939389551247551040926951238633020744494000165263268";
const DEFAULT_TX_HASH: &str = "0x4dd7e3f4522fcf2483ae422fd007492380051d87de6fdb17be71c7134e26857e";
const DEFAULT_ADDRESS_HASH: &str = "0xc0De20A37E2dAC848F81A93BD85FE4ACDdE7C0DE";
const BLOCK_VALIDATOR_ADDRESS_HASH: &str = "0x4838B106FCe9647Bdf1E7877BF73cE8B0BAD5f97";
const DEFAULT_SEARCH_QUERY: &str = "USDT";

#[fixture]
async fn api_client() -> OwnedApiClient {
    OwnedApiClient::start()
        .await
        .expect("start and await this test's owned recording server")
}

#[rstest]
#[tokio::test]
async fn health(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let health_api = api_client.health_api();
    let health = health_api.health().await.expect("Failed to get health");
    let success = health.try_as_success().cloned().unwrap_or_else(|| {
        panic!("failed to get health: {:?}", &health.entity);
    });
    assert_eq!(success.healthy, true);
    let metadata = serde_json::to_value(success.metadata.expect("metadata is None"))
        .expect("Failed to serialize typed health metadata");
    assert!(metadata.get("latest_block").is_none());
    assert_eq!(metadata["blocks"]["healthy"], true);
    for source in ["db", "cache", "node"] {
        let number = metadata["blocks"]["latest_block"][source]["number"]
            .as_str()
            .expect("block number is None");
        assert!(!number.is_empty());
        assert!(number.chars().all(|c| c.is_ascii_digit()));
        assert_eq!(number, "23116935");
    }

    assert!(matches!(
        health_api.health_v1().await,
        Err(blockscout_client::apis::Error::UnsupportedCapability(
            "upstream /api/v1/health is not mounted; use health() at /api/health"
        ))
    ));
}

#[test]
fn legacy_upstream_health_v1_2025_02_19() {
    let recording: serde_json::Value = serde_json::from_str(include_str!(
        "recorded/eth_blockscout_com/_api_v1_health.json"
    ))
    .expect("historical upstream recording");
    assert_eq!(
        recording["metadata"]["compatibility_version"],
        "upstream-v1-2025-02-19"
    );
    assert_eq!(recording["metadata"]["merged_explorer_supported"], false);
    assert_eq!(recording["request"]["method"], "GET");
    assert_eq!(recording["request"]["urlPath"], "/api/v1/health");
    assert_eq!(recording["response"]["status"], 200);
    let success: blockscout_client::models::V1HealthCheckResponse = serde_json::from_str(
        recording["response"]["body"]
            .as_str()
            .expect("historical body"),
    )
    .expect("historical v1 model");
    assert_eq!(success.healthy, Some(true));
    assert_eq!(
        success
            .data
            .expect("data is None")
            .latest_block_number
            .expect("latest_block_number is None"),
        "21879216"
    );
}

// Replay committed response bytes; assert every request at the HTTP boundary.
async fn recording_peer(
    requests: Vec<(String, &'static str)>,
) -> (ApiClient, tokio::task::JoinHandle<()>) {
    use tokio::{
        io::{AsyncReadExt, AsyncWriteExt},
        net::TcpListener,
    };

    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let server = tokio::spawn(async move {
        tokio::time::timeout(Duration::from_secs(10), async move {
            for (target, recording) in requests {
                let (mut stream, _) = listener.accept().await.unwrap();
                let mut headers = Vec::new();
                while !headers.ends_with(b"\r\n\r\n") {
                    assert!(headers.len() < 8192, "oversized HTTP request");
                    headers.push(stream.read_u8().await.unwrap());
                }
                let request = std::str::from_utf8(&headers).unwrap();
                assert_eq!(
                    request.lines().next().unwrap(),
                    format!("GET {target} HTTP/1.1")
                );
                let envelope: serde_json::Value = serde_json::from_str(recording).unwrap();
                let body = envelope["response"]["body"].as_str().unwrap();
                let status = envelope["response"]["status"].as_u64().unwrap();
                let content_type = envelope["response"]["headers"]["Content-Type"]
                    .as_str()
                    .unwrap();
                let response = format!(
                    "HTTP/1.1 {status} OK\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                );
                stream.write_all(response.as_bytes()).await.unwrap();
                stream.shutdown().await.unwrap();
            }
        })
        .await
        .expect("recording peer timed out");
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(10))
        .build()
        .unwrap();
    let client = ApiClient::new(
        Configuration::builder()
            .base_path(origin)
            .client(reqwest_middleware::ClientBuilder::new(http).build())
            .build(),
    );
    (client, server)
}

#[tokio::test]
async fn supported_health_http_and_legacy_refusal() {
    let (client, server) = recording_peer(vec![(
        "/api/health".to_owned(),
        include_str!("recorded/eth_blockscout_com/_api_health.json"),
    )])
    .await;
    // Refusal happens before transport: the peer's only request must be /api/health.
    assert!(matches!(
        client.health_api().health_v1().await,
        Err(blockscout_client::apis::Error::UnsupportedCapability(
            "upstream /api/v1/health is not mounted; use health() at /api/health"
        ))
    ));
    let response = client.health_api().health().await.unwrap();
    server.await.unwrap();
    assert_eq!(response.status.as_u16(), 200);
    let typed = response.try_parse_as_success_model().unwrap();
    assert_eq!(typed.healthy, true);
    let metadata = serde_json::to_value(typed.metadata.unwrap()).unwrap();
    assert_eq!(metadata["blocks"]["healthy"], true);
    for source in ["db", "cache", "node"] {
        assert_eq!(
            metadata["blocks"]["latest_block"][source]["number"],
            "23116935"
        );
    }
}

#[tokio::test]
async fn nft_full_width_cursor_survives_real_client_http() {
    use blockscout_client::apis::tokens_api::GetNftInstancesParams;

    const LAST_ID: &str =
        "115785665443821794889628784622978168165342723005226866880138161782244014770980";
    const RECORDING: &str = include_str!(
        "recorded/eth_blockscout_com/_api_v2_tokens_0xD4416b13d2b3a9aBae7AcD5D6C2BbDBE25686401_instances.json"
    );
    let path = format!("/api/v2/tokens/{DEFAULT_TOKEN_HASH}/instances");
    // The second replay checks cursor forwarding only; it is not a captured next page.
    let (client, server) = recording_peer(vec![
        (path.clone(), RECORDING),
        (format!("{path}?unique_token={LAST_ID}"), RECORDING),
    ])
    .await;
    let first = client
        .tokens_api()
        .get_nft_instances(
            GetNftInstancesParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .build(),
        )
        .await
        .unwrap();
    assert_eq!(first.status.as_u16(), 200);
    let typed = first.try_parse_as_success_model().unwrap();
    assert_eq!(typed.items.last().unwrap().id, LAST_ID);
    let cursor = first
        .next_page_cursor()
        .unwrap()
        .expect("nonterminal cursor");
    assert_eq!(
        cursor.query_pairs().collect::<Vec<_>>(),
        vec![("unique_token", LAST_ID)]
    );
    let forwarded = client
        .tokens_api()
        .get_nft_instances(
            GetNftInstancesParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .cursor(cursor)
                .build(),
        )
        .await
        .unwrap();
    server.await.unwrap();
    assert_eq!(forwarded.status.as_u16(), 200);
    assert_eq!(forwarded.content, first.content);
}

#[rstest]
#[tokio::test]
async fn blocks(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let blocks_api = api_client.blocks_api();
    let blocks = blocks_api
        .get_blocks(blockscout_client::apis::blocks_api::GetBlocksParams::builder().build())
        .await
        .expect("Failed to get blocks");
    let success_model = blocks.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn transactions(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let transactions_api = api_client.transactions_api();
    let transactions = transactions_api
        .get_txs(blockscout_client::apis::transactions_api::GetTxsParams::builder().build())
        .await
        .expect("Failed to get transactions");
    let success_model = transactions.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn internal_transactions(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let transactions_api = api_client.transactions_api();
    let internal_transactions = transactions_api
        .get_transaction_internal_txs(
            blockscout_client::apis::transactions_api::GetTransactionInternalTxsParams::builder()
                .transaction_hash(DEFAULT_TX_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get transactions");
    let success_model = internal_transactions.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn smart_contracts(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let smart_contracts_api = api_client.smart_contracts_api();
    let smart_contracts = smart_contracts_api
        .get_smart_contracts(
            blockscout_client::apis::smart_contracts_api::GetSmartContractsParams::builder()
                .build(),
        )
        .await
        .expect("Failed to get transactions");
    let success_model = smart_contracts.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
    let _smart_contract = smart_contracts_api
        .get_smart_contract(
            blockscout_client::apis::smart_contracts_api::GetSmartContractParams::builder()
                .address_hash(DEFAULT_CONTRACT_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get transactions");
}

#[rstest]
#[tokio::test]
async fn tokens(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let tokens_api = api_client.tokens_api();
    let tokens = tokens_api
        .get_tokens_list(
            blockscout_client::apis::tokens_api::GetTokensListParams::builder().build(),
        )
        .await
        .expect("Failed to get transactions");
    let success_model = tokens.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());

    let _token = tokens_api
        .get_token(
            blockscout_client::apis::tokens_api::GetTokenParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get transactions")
        .try_parse_as_success_model()
        .unwrap();
    let token_instances = tokens_api
        .get_nft_instances(
            blockscout_client::apis::tokens_api::GetNftInstancesParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get transactions");
    let success_model = token_instances.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());

    let params = blockscout_client::apis::tokens_api::GetNftInstanceParams::builder()
        .address_hash(DEFAULT_TOKEN_HASH.to_owned())
        .id(DEFAULT_TOKEN_INSTANCE_NUMBER.to_owned())
        .build();
    let _token_instance = tokens_api
        .get_nft_instance(params)
        .await
        .expect("Failed to get transactions")
        .try_parse_as_success_model()
        .unwrap();
}

#[rstest]
#[tokio::test]
async fn get_transaction(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let transactions_api = api_client.transactions_api();
    let response = transactions_api
        .get_tx(
            blockscout_client::apis::transactions_api::GetTxParams::builder()
                .transaction_hash(DEFAULT_TX_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get transactions");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert_eq!(success_model.hash, DEFAULT_TX_HASH);
}

#[rstest]
#[tokio::test]
async fn token_counters(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let tokens_api = api_client.tokens_api();
    let response = tokens_api
        .get_token_counters(
            blockscout_client::apis::tokens_api::GetTokenCountersParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get token counters");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(success_model.transfers_count.parse::<i64>().unwrap() > 0);
}

#[rstest]
#[tokio::test]
async fn token_holders(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let tokens_api = api_client.tokens_api();
    let response = tokens_api
        .get_token_holders(
            blockscout_client::apis::tokens_api::GetTokenHoldersParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get token holders");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn token_transfers(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let tokens_api = api_client.tokens_api();
    let response = tokens_api
        .get_token_token_transfers(
            blockscout_client::apis::tokens_api::GetTokenTokenTransfersParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get token transfers");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn token_instance_holders(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let tokens_api = api_client.tokens_api();
    let response = tokens_api
        .get_token_instance_holders(
            blockscout_client::apis::tokens_api::GetTokenInstanceHoldersParams::builder()
                .address_hash(DEFAULT_TOKEN_HASH.to_owned())
                .id(DEFAULT_TOKEN_INSTANCE_NUMBER.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get token instance holders");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn address_details(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address(
            blockscout_client::apis::addresses_api::GetAddressParams::builder()
                .address_hash(DEFAULT_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address details");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert_eq!(success_model.hash, DEFAULT_ADDRESS_HASH);
}

#[rstest]
#[tokio::test]
async fn address_blocks_validated(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_blocks_validated(
            blockscout_client::apis::addresses_api::GetAddressBlocksValidatedParams::builder()
                .address_hash(BLOCK_VALIDATOR_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address blocks validated");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn address_counters(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_counters(
            blockscout_client::apis::addresses_api::GetAddressCountersParams::builder()
                .address_hash(DEFAULT_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address counters");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(success_model.transactions_count.parse::<i64>().unwrap() > 0);
}

#[rstest]
#[tokio::test]
async fn address_logs(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_logs(
            blockscout_client::apis::addresses_api::GetAddressLogsParams::builder()
                .address_hash(DEFAULT_CONTRACT_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address logs");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn address_nft_collections(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_nft_collections(
            blockscout_client::apis::addresses_api::GetAddressNftCollectionsParams::builder()
                .address_hash(DEFAULT_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address NFT collections");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn address_token_balances(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_token_balances(
            blockscout_client::apis::addresses_api::GetAddressTokenBalancesParams::builder()
                .address_hash(DEFAULT_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address token balances");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.is_empty());
}

#[rstest]
#[tokio::test]
async fn address_token_transfers(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_token_transfers(
            blockscout_client::apis::addresses_api::GetAddressTokenTransfersParams::builder()
                .address_hash(DEFAULT_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address token transfers");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn address_tokens(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let addresses_api = api_client.addresses_api();
    let response = addresses_api
        .get_address_tokens(
            blockscout_client::apis::addresses_api::GetAddressTokensParams::builder()
                .address_hash(DEFAULT_ADDRESS_HASH.to_owned())
                .build(),
        )
        .await
        .expect("Failed to get address tokens");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

#[rstest]
#[tokio::test]
async fn stats(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let stats_api = api_client.stats_api();
    let response = stats_api.get_stats().await.expect("Failed to get stats");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(success_model.total_blocks.unwrap().parse::<i64>().unwrap() > 0);
}

// #[rstest]
// #[tokio::test]
// async fn config_json_rpc_url(api_client: &ApiClient) {
//     let config_api = api_client.config_api();
//     let response = config_api
//         .get_json_rpc_url()
//         .await
//         .expect("Failed to get JSON RPC URL");
//     let success_model = response.try_parse_as_success_model().unwrap();
//     assert!(!success_model.json_rpc_url.is_empty());
// }

#[rstest]
#[tokio::test]
async fn search(#[future] api_client: OwnedApiClient) {
    let api_client = api_client.await;
    let search_api = api_client.search_api();
    let params = blockscout_client::apis::search_api::SearchParams::builder()
        .q(DEFAULT_SEARCH_QUERY.to_owned())
        .build();
    let response = search_api
        .search(params)
        .await
        .expect("Failed to get search");
    let success_model = response.try_parse_as_success_model().unwrap();
    assert!(!success_model.items.is_empty());
}

use serde::{Deserialize, Serialize};

#[rstest]
#[tokio::test]
async fn deserialize_decimal() {
    #[derive(Debug, Serialize, Deserialize)]
    struct Message {
        maybe_number: Option<rust_decimal::Decimal>,
        number: rust_decimal::Decimal,
        numbers: Vec<rust_decimal::Decimal>,
    }

    let _: Message = serde_json::from_str(
        r#"{"maybe_number": 0, "number": "1.23", "numbers": ["1.23", "4.56", "0"]}"#,
    )
    .unwrap();
}
