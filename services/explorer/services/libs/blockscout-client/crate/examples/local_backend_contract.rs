use blockscout_client::{
    apis::{blocks_api::GetBlocksParams, health_api::HealthError, Error},
    models::{health_check_response_metadata::HealthBlocks, HealthCheckResponse},
    ApiClient, Configuration,
};
use rust_decimal::Decimal;
use serde::Deserialize;
use serde_json::{json, Value};
use std::{env, fs, path::Path};

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ExpectedBlock {
    height: i64,
    hash: String,
    parent_hash: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct BackendState {
    origin: String,
    source_revision: String,
    health: HealthBlocks,
    blocks: Vec<ExpectedBlock>,
}

fn read_state(path: &str, healthy: bool) -> BackendState {
    let state: BackendState = serde_json::from_slice(&fs::read(path).unwrap()).unwrap();
    assert_eq!(state.source_revision.len(), 40);
    assert!(state.source_revision.bytes().all(|b| b.is_ascii_hexdigit()));
    let origin = url::Url::parse(&state.origin).unwrap();
    assert!(matches!(origin.host_str(), Some("127.0.0.1" | "[::1]")));
    assert_eq!(origin.path(), "/");
    assert!(origin.query().is_none() && origin.fragment().is_none());
    assert!(origin.username().is_empty() && origin.password().is_none());
    assert_eq!(state.health.healthy, healthy);
    if healthy {
        assert!(!state.blocks.is_empty());
        assert!(state.health.error.is_none());
    } else {
        assert!(state.blocks.is_empty());
        let error = state.health.error.as_ref().unwrap();
        assert_eq!(error.code, 5002);
        assert_eq!(error.message, "There are no blocks in the DB.");
    }
    assert!(state.blocks.len() <= 50);
    state
}

async fn check_backend(state: &BackendState, evidence: &mut Vec<Value>) {
    let client = ApiClient::new(Configuration::builder().base_path(&state.origin).build());
    let (status, content, model) = match client.health_api().health().await {
        Ok(response) => {
            assert!(state.health.healthy);
            assert_eq!(response.status.as_u16(), 200);
            let model = response.try_parse_as_success_model().unwrap();
            assert_eq!(response.try_as_success().unwrap(), &model);
            (response.status, response.content, model)
        }
        Err(Error::ResponseError(response)) => {
            assert!(!state.health.healthy);
            assert_eq!(response.status.as_u16(), 500);
            let model: HealthCheckResponse = serde_json::from_str(&response.content).unwrap();
            match response.entity.as_ref().unwrap() {
                HealthError::Status500(typed) => assert_eq!(typed.as_ref(), &model),
                other => panic!("untyped health error: {other:?}"),
            }
            (response.status, response.content, model)
        }
        Err(error) => panic!("health transport/decoding failed: {error:?}"),
    };
    assert_eq!(model.healthy, Some(state.health.healthy));
    assert_eq!(model.error, state.health.error);
    let actual = model.metadata.as_ref().unwrap().blocks.as_ref().unwrap();
    assert_eq!(actual, &state.health);
    // Require all three independently exported height sources even on HTTP 500.
    assert!(actual.latest_block.db.as_ref().unwrap().number.is_some());
    assert!(actual.latest_block.cache.as_ref().unwrap().number.is_some());
    assert!(actual.latest_block.node.is_some());
    evidence.push(json!({
        "source_revision": state.source_revision, "origin": state.origin,
        "path": "/api/health", "status": status.as_u16(), "body": content,
        "typed": model,
    }));

    let response = client
        .blocks_api()
        .get_blocks(GetBlocksParams {
            r#type: Some("block".into()),
        })
        .await
        .unwrap();
    assert_eq!(response.status.as_u16(), 200);
    let page = response.try_parse_as_success_model().unwrap();
    assert_eq!(response.try_as_success().unwrap(), &page);
    assert!(page.next_page_params.is_null());
    assert_eq!(page.items.len(), state.blocks.len());
    for (actual, expected) in page.items.iter().zip(&state.blocks) {
        assert_eq!(actual.height, Decimal::from(expected.height));
        assert_eq!(actual.hash, expected.hash);
        assert_eq!(actual.parent_hash, expected.parent_hash);
    }
    evidence.push(json!({
        "source_revision": state.source_revision, "origin": state.origin,
        "path": "/api/v2/blocks?type=block", "status": response.status.as_u16(),
        "body": response.content, "typed": page,
    }));
}

#[tokio::main]
async fn main() {
    let args: Vec<String> = env::args().collect();
    assert_eq!(
        args.len(),
        4,
        "usage: local_backend_contract healthy.json empty.json evidence.json"
    );
    let healthy = read_state(&args[1], true);
    let empty = read_state(&args[2], false);
    assert_eq!(healthy.source_revision, empty.source_revision);
    assert_ne!(healthy.origin, empty.origin);
    assert!(!Path::new(&args[3]).exists(), "evidence output must be new");
    let mut evidence = Vec::new();
    check_backend(&healthy, &mut evidence).await;
    check_backend(&empty, &mut evidence).await;
    fs::write(&args[3], serde_json::to_vec_pretty(&evidence).unwrap()).unwrap();
}
