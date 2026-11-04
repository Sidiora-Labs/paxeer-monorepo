//! Task 22.33.428: production model serializers -> loopback HTTP -> ApiClient
//! -> both typed response consumers. Backend recordings supply the envelopes;
//! model-produced branch probes cover distinct heights and every action kind.
//! This gate does not execute the Elixir backend. See the in-scope repair record
//! in src/models/repair_22_33_428.kvx for provenance and schema wiring requests.

use blockscout_client::{
    apis::transactions_api::GetTxParams, models::*, Api, ApiClient, Configuration,
};
use serde_json::{json, Value};
use std::time::Duration;
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    task::JoinHandle,
    time::timeout,
};

fn recording(text: &str) -> (String, String) {
    let recording: Value = serde_json::from_str(text).unwrap();
    (
        recording["request"]["urlPath"].as_str().unwrap().into(),
        recording["response"]["body"].as_str().unwrap().into(),
    )
}

async fn replay(path: String, body: String) -> (ApiClient, JoinHandle<()>) {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let origin = format!("http://{}", listener.local_addr().unwrap());
    let server = tokio::spawn(async move {
        timeout(Duration::from_secs(5), async move {
            let (mut socket, _) = listener.accept().await.unwrap();
            let mut request = Vec::new();
            while !request.ends_with(b"\r\n\r\n") {
                let mut byte = [0];
                socket.read_exact(&mut byte).await.unwrap();
                request.push(byte[0]);
                assert!(request.len() <= 16384, "unexpected oversized request");
            }
            let request = std::str::from_utf8(&request).unwrap();
            assert_eq!(request.lines().next().unwrap(), format!("GET {path} HTTP/1.1"));
            let headers = format!(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                body.len()
            );
            socket.write_all(headers.as_bytes()).await.unwrap();
            socket.write_all(body.as_bytes()).await.unwrap();
            socket.shutdown().await.unwrap();
        })
        .await
        .expect("loopback replay must finish");
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(5))
        .build()
        .unwrap();
    let configuration = Configuration::builder()
        .base_path(origin)
        .client(reqwest_middleware::ClientBuilder::new(http).build())
        .build();
    (ApiClient::new(configuration), server)
}

#[tokio::test]
async fn health_cache_number() {
    // Unchanged committed HealthController recording, then a production-codec
    // branch probe. The recording itself has equal heights; never call the
    // deliberately distinct probe an independently recorded backend response.
    let (path, recorded) = recording(include_str!("recorded/eth_blockscout_com/_api_health.json"));
    let mut producer: HealthCheckResponse = serde_json::from_str(&recorded).unwrap();
    let latest = &mut producer.metadata.as_mut().unwrap().blocks.latest_block;
    latest.db.number = "23116931".into();
    latest.cache.number = "23116933".into();
    latest.node.number = "23116939".into();
    let distinct = serde_json::to_string(&producer).unwrap();

    for (body, heights) in [
        (recorded, ["23116935", "23116935", "23116935"]),
        (distinct, ["23116931", "23116933", "23116939"]),
    ] {
        let wire: Value = serde_json::from_str(&body).unwrap();
        let (client, server) = replay(path.clone(), body).await;
        let response = client.health_api().health().await.unwrap();
        server.await.unwrap();
        assert_eq!(response.status, reqwest::StatusCode::OK);
        let cached = response.try_as_success().expect("typed health response");
        let parsed = response.try_parse_as_success_model().unwrap();
        assert_eq!(cached, &parsed);
        let latest = &parsed.metadata.as_ref().unwrap().blocks.latest_block;
        assert_eq!(latest.db.number, heights[0]);
        assert_eq!(latest.cache.number, heights[1]);
        assert_eq!(latest.node.number, heights[2]);
        let encoded = serde_json::to_value(latest).unwrap();
        assert_eq!(encoded, wire["metadata"]["blocks"]["latest_block"]);
        assert!(encoded["cache"].get("mber").is_none());
    }
}

fn producer_actions() -> Vec<TransactionAction> {
    // Data contracts: Indexer.Transform.TransactionActions.aave_handle_event,
    // uniswap_handle_event and uniswap_mint_nft; source OpenAPI examples supply
    // token identities and amounts. Actual variant serializers produce the
    // wire actions. Expected classifications never use the decoder under test.
    let liquidation = json!({
        "debt_amount": "1.289548595490270429", "debt_symbol": "AAVE",
        "debt_address": "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9",
        "collateral_amount": "110.824768", "collateral_symbol": "USDC",
        "collateral_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        "block_number": 1
    });
    let token = json!({
        "symbol": "AAVE", "address": "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9",
        "block_number": 1
    });
    let mut amount = token.clone();
    amount["amount"] = json!("1.289548595490270429");
    let nft = json!({
        "name": "Uniswap V3: Positions NFT", "symbol": "UNI-V3-POS",
        "address": "0x1F98431c8aD98523631AE4a59f267346ea31F984",
        "to": "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9",
        "ids": ["1", "115792089237316195423570985008687907853269984665640564039457584007913129639935"],
        "block_number": 1
    });
    let liquidity = json!({
        "address0": "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9",
        "address1": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        "amount0": "1.289548595490270429", "amount1": "110.824768",
        "symbol0": "AAVE", "symbol1": "USDC", "block_number": 1
    });
    let mut actions = vec![TransactionAction::TransactionActionAaveV3LiquidationCall(
        TransactionActionAaveV3LiquidationCall::new(
            liquidation,
            "aave_v3".into(),
            "liquidation_call".into(),
        ),
    )];
    for kind in ["borrow", "supply", "withdraw", "repay", "flash_loan"] {
        actions.push(TransactionAction::TransactionActionAaveV3Bswrf(
            TransactionActionAaveV3Bswrf::new(amount.clone(), "aave_v3".into(), kind.into()),
        ));
    }
    for kind in ["enable_collateral", "disable_collateral"] {
        actions.push(
            TransactionAction::TransactionActionAaveV3EnableDisableCollateral(
                TransactionActionAaveV3EnableDisableCollateral::new(
                    token.clone(),
                    "aave_v3".into(),
                    kind.into(),
                ),
            ),
        );
    }
    actions.push(TransactionAction::TransactionActionUniswapV3MintNft(
        TransactionActionUniswapV3MintNft::new(nft, "uniswap_v3".into(), "mint_nft".into()),
    ));
    // The backend emits mint as well as the burn/collect/swap OpenAPI examples.
    for kind in ["mint", "burn", "collect", "swap"] {
        actions.push(TransactionAction::TransactionActionUniswapV3Bcs(
            TransactionActionUniswapV3Bcs::new(liquidity.clone(), "uniswap_v3".into(), kind.into()),
        ));
    }
    actions
}

#[tokio::test]
async fn action_variant_collapse() {
    let (path, body) = recording(include_str!(
        "recorded/eth_blockscout_com/_api_v2_transactions_0x4dd7e3f4.json"
    ));
    let mut envelope: Value = serde_json::from_str(&body).unwrap();
    let expected = producer_actions();
    assert_eq!(
        expected.len(),
        13,
        "exercise every supported producer action"
    );
    envelope["actions"] = serde_json::to_value(&expected).unwrap();
    let (client, server) = replay(path, serde_json::to_string(&envelope).unwrap()).await;
    let response = client
        .transactions_api()
        .get_tx(GetTxParams {
            transaction_hash: envelope["hash"].as_str().unwrap().into(),
        })
        .await
        .unwrap();
    server.await.unwrap();
    assert_eq!(response.status, reqwest::StatusCode::OK);
    let cached = response
        .try_as_success()
        .expect("typed transaction response");
    let parsed = response.try_parse_as_success_model().unwrap();
    for actions in [&cached.actions, &parsed.actions] {
        let actions = actions.as_ref().expect("producer actions must be present");
        // PartialEq includes the enum variant, the protocol/type and all raw data.
        assert_eq!(actions, &expected);
        assert_eq!(serde_json::to_value(actions).unwrap(), envelope["actions"]);
        for action in actions {
            let bytes = serde_json::to_vec(action).unwrap();
            let decoded: TransactionAction = serde_json::from_slice(&bytes).unwrap();
            assert_eq!(&decoded, action);
        }
    }
}

#[test]
fn action_variant_invalid_discriminators_are_rejected() {
    for (protocol, kind) in [
        ("uniswap_v3", "liquidation_call"),
        ("aave_v3", "mint_nft"),
        ("uniswap_v3", "borrow"),
        ("aave_v3", "swap"),
        ("other", "liquidation_call"),
        ("aave_v3", "other"),
        ("uniswap_v3", "other"),
        ("", ""),
    ] {
        let wire = json!({"protocol": protocol, "type": kind, "data": {}});
        let error = serde_json::from_value::<TransactionAction>(wire).unwrap_err();
        assert!(error
            .to_string()
            .contains("unsupported transaction action protocol/type"));
    }
    let valid = json!({"protocol": "aave_v3", "type": "borrow", "data": {}});
    for field in ["protocol", "type", "data"] {
        let mut wire = valid.clone();
        wire.as_object_mut().unwrap().remove(field);
        assert!(
            serde_json::from_value::<TransactionAction>(wire).is_err(),
            "missing {field}"
        );
    }
    for field in ["protocol", "type"] {
        for invalid in [Value::Null, json!(3), json!([]), json!({})] {
            let mut wire = valid.clone();
            wire[field] = invalid;
            assert!(
                serde_json::from_value::<TransactionAction>(wire).is_err(),
                "invalid {field}"
            );
        }
    }
}

#[test]
fn action_variant_raw_data_is_lossless() {
    let data = json!({
        "amount": "1.289548595490270429", "extension": {
            "values": [null, false, 42, {"nested": ["raw", "東京"]}],
            "large": "115792089237316195423570985008687907853269984665640564039457584007913129639935"
        }
    });
    for producer in producer_actions() {
        let mut wire = serde_json::to_value(&producer).unwrap();
        wire["data"] = data.clone();
        let decoded: TransactionAction = serde_json::from_value(wire.clone()).unwrap();
        assert_eq!(
            std::mem::discriminant(&decoded),
            std::mem::discriminant(&producer)
        );
        assert_eq!(serde_json::to_value(decoded).unwrap(), wire);
    }
}
