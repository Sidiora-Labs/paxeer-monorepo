use blockscout_client::{
    apis::{
        health_api::{HealthError, HealthSuccess},
        Error,
    },
    Api, ApiClient, Configuration,
};
use serde_json::Value;
use std::time::Duration;
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
};

fn assert_preserved(produced: &Value, consumed: &Value, path: &str) {
    if let Value::Object(fields) = produced {
        let actual = consumed
            .as_object()
            .unwrap_or_else(|| panic!("{path}: generated consumer dropped an object"));
        for (key, value) in fields {
            let field_path = format!("{path}.{key}");
            let actual = actual
                .get(key)
                .unwrap_or_else(|| panic!("{field_path}: generated consumer dropped a field"));
            assert_preserved(value, actual, &field_path);
        }
    } else {
        assert_eq!(
            consumed, produced,
            "{path}: generated consumer changed a value"
        );
    }
}

fn consume_health(status: u16, body: &str) -> Value {
    match status {
        200 => match serde_json::from_str::<HealthSuccess>(body).expect("health success codec") {
            HealthSuccess::Status200(model) => serde_json::to_value(model).unwrap(),
            HealthSuccess::UnknownValue(_) => panic!("healthy response must have a typed model"),
        },
        500 => match serde_json::from_str::<HealthError>(body).expect("health error codec") {
            HealthError::Status500(model) => serde_json::to_value(model).unwrap(),
            HealthError::UnknownValue(_) => {
                panic!("unhealthy response must have a typed HTTP 500 model")
            }
        },
        _ => panic!("unexpected controller status {status}"),
    }
}

#[tokio::test]
async fn health_response_shape_preserves_recorded_blocks() {
    let recording: Value =
        serde_json::from_str(include_str!("recorded/eth_blockscout_com/_api_health.json")).unwrap();
    let body = recording["response"]["body"].as_str().unwrap();
    let produced: Value = serde_json::from_str(body).unwrap();
    assert_eq!(recording["response"]["status"], 200);
    let blocks = &produced["metadata"]["blocks"];
    assert_eq!(blocks["healthy"], true);
    assert_eq!(blocks["latest_block"]["cache"]["number"], "23116935");
    assert_eq!(blocks["latest_block"]["node"]["number"], "23116935");

    // The current controller's blocks subtree and top-level error must survive.
    let response = request_health(200, body)
        .await
        .expect("recorded health request");
    assert_eq!(response.status.as_u16(), 200);
    assert_eq!(response.content, body);
    let model = response
        .try_as_success()
        .expect("typed recorded health response");
    let consumed = serde_json::to_value(model).unwrap();
    assert_preserved(blocks, &consumed["metadata"]["blocks"], "metadata.blocks");
    assert_eq!(consumed.get("error"), produced.get("error"));
    let consumed = consume_health(200, body);
    assert_preserved(blocks, &consumed["metadata"]["blocks"], "metadata.blocks");
    assert_eq!(consumed.get("error"), produced.get("error"));
}

async fn request_health(
    status: u16,
    body: &str,
) -> Result<blockscout_client::apis::ResponseContent<HealthSuccess>, Error<HealthError>> {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let response_body = body.to_owned();
    let server = tokio::spawn(async move {
        let (mut socket, _) = listener.accept().await.unwrap();
        let mut request = Vec::new();
        loop {
            let mut buffer = [0_u8; 1024];
            let read = socket.read(&mut buffer).await.unwrap();
            assert!(read > 0, "client closed before sending request headers");
            request.extend_from_slice(&buffer[..read]);
            if request.windows(4).any(|part| part == b"\r\n\r\n") {
                break;
            }
            assert!(request.len() < 16384, "unexpectedly large health request");
        }
        assert!(request.starts_with(b"GET /api/health HTTP/1.1\r\n"));
        let reason = match status {
            200 => "OK",
            500 => "Internal Server Error",
            _ => panic!("unexpected health fixture status {status}"),
        };
        let response = format!(
            "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response_body}",
            response_body.len()
        );
        socket.write_all(response.as_bytes()).await.unwrap();
        socket.shutdown().await.unwrap();
    });
    let http = reqwest::Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(5))
        .build()
        .unwrap();
    let client = ApiClient::new(
        Configuration::builder()
            .base_path(format!("http://{address}"))
            .client(reqwest_middleware::ClientBuilder::new(http).build())
            .build(),
    );
    let result = client.health_api().health().await;
    tokio::time::timeout(Duration::from_secs(5), server)
        .await
        .expect("loopback health server timed out")
        .expect("loopback health server failed");
    result
}

#[tokio::test]
async fn health_response_shape_preserves_controller_lifecycle() {
    let responses: Vec<Value> =
        serde_json::from_str(include_str!("health/repair_22_31_269.json")).unwrap();
    assert_eq!(responses.len(), 5, "every controller branch must execute");

    let mut cases = Vec::new();
    for (response, expected_case) in
        responses
            .into_iter()
            .zip(["healthy", "no_blocks", "stale", "rollup", "media_worker"])
    {
        cases.push((
            expected_case,
            tokio::spawn(assert_controller_case(response, expected_case)),
        ));
    }
    let mut failures = Vec::new();
    for (case, task) in cases {
        if let Err(error) = task.await {
            failures.push(format!("{case}: {error}"));
        }
    }
    assert!(
        failures.is_empty(),
        "controller lifecycle failures: {failures:#?}"
    );
}

async fn assert_controller_case(response: Value, expected_case: &str) {
    let case = response["case"].as_str().unwrap();
    assert_eq!(case, expected_case);
    let status = u16::try_from(response["status"].as_u64().unwrap()).unwrap();
    let produced = &response["body"];
    let body = serde_json::to_string(produced).unwrap();
    let healthy = matches!(case, "healthy" | "rollup" | "media_worker");
    assert_eq!(status, if healthy { 200 } else { 500 });
    assert_eq!(produced["healthy"], healthy);

    if case == "media_worker" {
        assert_eq!(produced["data"], serde_json::json!({}));
    } else {
        let blocks = &produced["metadata"]["blocks"];
        assert_eq!(blocks["healthy"], healthy);
        if case == "no_blocks" {
            assert_eq!(blocks["latest_block"]["cache"]["number"], "");
            assert_eq!(blocks["latest_block"]["db"]["timestamp"], "");
        } else {
            assert_eq!(blocks["latest_block"]["cache"]["number"], "101");
            assert_eq!(blocks["latest_block"]["node"]["number"], "150");
            assert!(!blocks["latest_block"]["db"]["timestamp"]
                .as_str()
                .unwrap()
                .is_empty());
        }
        if !healthy {
            assert_eq!(
                blocks["error"]["code"],
                if case == "no_blocks" { 5002 } else { 5001 }
            );
            assert!(!blocks["error"]["message"].as_str().unwrap().is_empty());
            assert_eq!(produced["error"], blocks["error"]);
        }
        if case == "rollup" {
            let batches = &produced["metadata"]["batches"];
            assert_eq!(batches["healthy"], true);
            assert_eq!(batches["latest_batch"]["db"]["number"], "20");
            assert_eq!(batches["latest_batch"]["db"]["average_batch_time"], "12");
        }
    }

    let consumed = match request_health(status, &body).await {
        Ok(response) => {
            assert_eq!(status, 200, "unhealthy response must use the error path");
            assert_eq!(response.status.as_u16(), status);
            assert_eq!(response.content, body);
            let model = response.try_as_success().expect("typed health success");
            assert_eq!(healthy, model.healthy);
            let parsed = response
                .try_parse_as_success_model()
                .expect("success model codec");
            assert_eq!(&parsed, model);
            serde_json::to_value(model).unwrap()
        }
        Err(Error::ResponseError(response)) => {
            assert_eq!(status, 500, "healthy response must use the success path");
            assert_eq!(response.status.as_u16(), status);
            assert_eq!(response.content, body);
            match response.entity.expect("typed health failure") {
                HealthError::Status500(model) => {
                    assert!(!model.healthy);
                    assert_eq!(model.metadata.blocks.healthy, model.healthy);
                    assert_eq!(model.error, model.metadata.blocks.error.clone().unwrap());
                    serde_json::to_value(model).unwrap()
                }
                HealthError::UnknownValue(_) => {
                    panic!("unhealthy response must have a typed HTTP 500 model")
                }
            }
        }
        Err(error) => panic!("health transport failed: {error:?}"),
    };
    assert_preserved(produced, &consumed, case);
    assert_preserved(produced, &consume_health(status, &body), case);
}

#[path = "../../../tests/repair_22_31_269.rs"]
mod completeness;
