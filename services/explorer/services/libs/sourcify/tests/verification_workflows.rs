#[path = "../src/test_service.rs"]
mod test_service;

use bytes::Bytes;
use serde_json::json;
use sourcify::{Client, ClientBuilder, Error, MatchType, SourcifyError, VerifyFromEtherscanError};
use std::{
    collections::BTreeMap,
    time::{Duration, Instant},
};
use test_service::{
    api_error, contract, contract_route, submit_route, Exchange, Service, ADDRESS, LIBRARY,
    PLACEHOLDER,
};

fn bytes(hex: &str) -> Bytes {
    blockscout_display_bytes::decode_hex(hex).unwrap().into()
}

fn client(service: &Service, timeout: Duration, retries: u32) -> Client {
    ClientBuilder::default()
        .try_base_url(&service.base_url())
        .unwrap()
        .operation_timeout(timeout)
        .max_retries(retries)
        .build()
}

#[test]
fn sourcify_base_url_rejects_invalid_endpoints() {
    for endpoint in [
        "mailto:a@example.com",
        "data:text/plain,sourcify",
        "urn:example:sourcify",
        "ftp://example.com/server",
        "file:///server",
        "http://",
        "https://?query",
    ] {
        assert!(
            ClientBuilder::default().try_base_url(endpoint).is_err(),
            "accepted {endpoint}"
        );
    }
    for endpoint in [
        "https://example.com/server",
        "http://localhost:1234/a/b/",
        "https://[::1]/",
    ] {
        assert!(
            ClientBuilder::default().try_base_url(endpoint).is_ok(),
            "rejected {endpoint}"
        );
    }
}

#[tokio::test]
async fn sourcify_successful_submission_job_sources_and_libraries_preserve_prefix() {
    // The first lookup is pending; the next poll must complete before retrieval.
    let mut exchanges = test_service::verification();
    exchanges.insert(
        1,
        Exchange::new(
            "GET",
            "v2/verify/job-521",
            200,
            json!({"isJobCompleted":false,"error":null}),
        ),
    );
    exchanges.push(Exchange::new("GET", contract_route(), 200, contract()));
    let service = Service::start(exchanges);
    let client = client(&service, Duration::from_secs(5), 0);
    let result = client
        .verify_from_etherscan("10", bytes(ADDRESS))
        .await
        .unwrap();
    assert_eq!(result.address, bytes(ADDRESS));
    assert_eq!(result.chain_id, "10");
    assert_eq!(result.status, MatchType::Partial);
    assert_eq!(
        result.library_map,
        BTreeMap::from([(PLACEHOLDER.to_string(), bytes(LIBRARY))])
    );
    assert_eq!(
        result.immutable_references,
        Some(test_service::immutables())
    );
    let sources = client
        .get_source_files_any("10", bytes(ADDRESS))
        .await
        .unwrap();
    assert_eq!(sources.status, MatchType::Partial);
    assert_eq!(
        sources.sources,
        BTreeMap::from([(
            "contracts/Example.sol".to_string(),
            test_service::SOURCE.to_string()
        )])
    );
    assert_eq!(sources.metadata, test_service::metadata());
    assert_eq!(sources.library_map, result.library_map);
    assert_eq!(sources.constructor_arguments, Some(bytes("0x1234")));
    assert_eq!(sources.immutable_references, result.immutable_references);
    let files = sources.raw_json["files"].as_array().unwrap();
    assert_eq!(files.len(), 6);
    assert!(files.iter().all(|file| file["path"]
        .as_str()
        .unwrap()
        .starts_with(&format!("contracts/partial_match/10/{ADDRESS}/"))));
    service.assert_finished();
}

#[tokio::test]
async fn sourcify_existing_verification_retrieves_full_match_without_polling() {
    let mut full = contract();
    full["runtimeMatch"] = json!("exact_match");
    let service = Service::start(vec![
        Exchange::new(
            "POST",
            submit_route(),
            409,
            api_error("already_verified", "Contract already verified"),
        ),
        Exchange::new("GET", contract_route(), 200, full),
    ]);
    let client = ClientBuilder::default()
        .try_base_url(&(service.base_url() + "/"))
        .unwrap()
        .max_retries(0)
        .build();
    let result = client
        .verify_from_etherscan("10", bytes(ADDRESS))
        .await
        .unwrap();
    assert_eq!(result.status, MatchType::Full);
    assert_eq!(
        result.library_map,
        BTreeMap::from([(PLACEHOLDER.to_string(), bytes(LIBRARY))])
    );
    assert_eq!(
        result.immutable_references,
        Some(test_service::immutables())
    );
    service.assert_finished();
}

async fn assert_verify_timeout(exchanges: Vec<Exchange>, retries: u32) {
    let service = Service::start(exchanges);
    let client = client(&service, Duration::from_millis(400), retries);
    let started = Instant::now();
    let error = client
        .verify_from_etherscan("10", bytes(ADDRESS))
        .await
        .unwrap_err();
    assert!(matches!(error, Error::VerificationTimeout), "{error:?}");
    assert_eq!(
        error.to_string(),
        "Sourcify operation exceeded its configured total deadline"
    );
    assert!(
        started.elapsed() < Duration::from_millis(800),
        "operation escaped its deadline: {:?}",
        started.elapsed()
    );
    service.assert_finished();
}

#[tokio::test]
async fn sourcify_operation_deadline_bounds_submission() {
    let mut exchanges = test_service::verification();
    exchanges.truncate(1);
    exchanges[0] = Exchange::new(
        "POST",
        submit_route(),
        202,
        json!({"verificationId":"job-521"}),
    )
    .delayed(Duration::from_secs(1));
    assert_verify_timeout(exchanges, 0).await;
}

#[tokio::test]
async fn sourcify_operation_deadline_bounds_poll_transport() {
    let mut exchanges = test_service::verification();
    exchanges.truncate(2);
    exchanges[1] = Exchange::new(
        "GET",
        "v2/verify/job-521",
        200,
        json!({"isJobCompleted":true}),
    )
    .delayed(Duration::from_secs(1));
    assert_verify_timeout(exchanges, 0).await;
}

#[tokio::test]
async fn sourcify_operation_deadline_bounds_pending_poll_sleep() {
    let mut exchanges = test_service::verification();
    exchanges.truncate(2);
    exchanges[1] = Exchange::new(
        "GET",
        "v2/verify/job-521",
        200,
        json!({"isJobCompleted":false}),
    );
    assert_verify_timeout(exchanges, 0).await;
}

#[tokio::test]
async fn sourcify_operation_deadline_bounds_retrieval_body() {
    let mut exchanges = test_service::verification();
    exchanges[2] = Exchange::new("GET", contract_route(), 200, contract())
        .delayed_body(Duration::from_secs(1));
    assert_verify_timeout(exchanges, 0).await;
}

#[tokio::test]
async fn sourcify_operation_deadline_is_shared_across_legs() {
    let mut exchanges = test_service::verification();
    // Every leg is faster than the budget; their sum exceeds it.
    exchanges[0] = Exchange::new(
        "POST",
        submit_route(),
        202,
        json!({"verificationId":"job-521"}),
    )
    .delayed(Duration::from_millis(160));
    exchanges[1] = Exchange::new(
        "GET",
        "v2/verify/job-521",
        200,
        json!({"isJobCompleted":true}),
    )
    .delayed(Duration::from_millis(160));
    exchanges[2] =
        Exchange::new("GET", contract_route(), 200, contract()).delayed(Duration::from_millis(160));
    assert_verify_timeout(exchanges, 0).await;
}

#[tokio::test]
async fn sourcify_operation_deadline_bounds_retry_backoff() {
    assert_verify_timeout(
        vec![Exchange::new(
            "POST",
            submit_route(),
            503,
            api_error("unavailable", "try later"),
        )],
        3,
    )
    .await;
}

#[tokio::test]
async fn sourcify_existing_verification_keeps_deadline_through_chains() {
    assert_verify_timeout(
        vec![
            Exchange::new(
                "POST",
                submit_route(),
                409,
                api_error("already_verified", "already verified"),
            ),
            Exchange::new("GET", contract_route(), 404, test_service::unverified()),
            Exchange::new(
                "GET",
                "chains",
                200,
                json!([{"chainId":10,"supported":true}]),
            )
            .delayed(Duration::from_secs(1)),
        ],
        0,
    )
    .await;
}

#[tokio::test]
async fn sourcify_source_lookup_has_total_deadline() {
    let service = Service::start(vec![
        Exchange::new("GET", contract_route(), 404, test_service::unverified())
            .delayed(Duration::from_millis(240)),
        Exchange::new(
            "GET",
            "chains",
            200,
            json!([{"chainId":10,"supported":true}]),
        )
        .delayed(Duration::from_millis(240)),
    ]);
    let started = Instant::now();
    let error = client(&service, Duration::from_millis(400), 0)
        .get_source_files_any("10", bytes(ADDRESS))
        .await
        .unwrap_err();
    assert!(matches!(error, Error::VerificationTimeout), "{error:?}");
    assert!(started.elapsed() < Duration::from_millis(800));
    service.assert_finished();
}

#[tokio::test]
async fn sourcify_typed_submission_and_completed_job_refusals() {
    for (code, message, kind) in [
        ("unsupported_chain", "unsupported Etherscan chain", 0),
        ("etherscan_limit", "Etherscan rate limit", 1),
        ("etherscan_request_failed", "upstream refused", 2),
        ("malformed_etherscan_response", "invalid compiler input", 3),
        ("compiler_error", "compilation failed", 4),
        ("no_match", "bytecode does not match", 4),
    ] {
        for completed_job in [false, true] {
            let error_body = api_error(code, message);
            let exchanges = if completed_job {
                vec![
                    Exchange::new(
                        "POST",
                        submit_route(),
                        202,
                        json!({"verificationId":"job-521"}),
                    ),
                    Exchange::new(
                        "GET",
                        "v2/verify/job-521",
                        200,
                        json!({"isJobCompleted":true,"error":error_body}),
                    ),
                ]
            } else {
                vec![Exchange::new("POST", submit_route(), 400, error_body)]
            };
            let service = Service::start(exchanges);
            let error = client(&service, Duration::from_secs(2), 0)
                .verify_from_etherscan("10", bytes(ADDRESS))
                .await
                .unwrap_err();
            let expected = match kind {
                0 => VerifyFromEtherscanError::ChainNotSupported(message.into()),
                1 => VerifyFromEtherscanError::TooManyRequests(message.into()),
                2 => VerifyFromEtherscanError::ApiResponseError(message.into()),
                3 => VerifyFromEtherscanError::CannotGenerateSolcJsonInput(message.into()),
                4 => VerifyFromEtherscanError::VerifiedWithErrors(message.into()),
                _ => unreachable!(),
            };
            assert!(
                matches!(error, Error::Sourcify(SourcifyError::Custom(actual)) if actual == expected)
            );
            service.assert_finished();
        }
    }
}

#[tokio::test]
async fn sourcify_source_lookup_distinguishes_not_found_and_unsupported_chain() {
    for supported in [true, false] {
        let service = Service::start(vec![
            Exchange::new("GET", contract_route(), 404, test_service::unverified()),
            Exchange::new(
                "GET",
                "chains",
                200,
                json!([{"chainId":10,"supported":supported}]),
            ),
        ]);
        let error = client(&service, Duration::from_secs(2), 0)
            .get_source_files_any("10", bytes(ADDRESS))
            .await
            .unwrap_err();
        if supported {
            assert!(
                matches!(error, Error::Sourcify(SourcifyError::NotFound(message)) if message == format!("Contract {ADDRESS} on chain 10 is not verified"))
            );
        } else {
            assert!(
                matches!(error, Error::Sourcify(SourcifyError::ChainNotSupported(message)) if message == "Chain 10 is not supported by Sourcify")
            );
        }
        service.assert_finished();
    }
}

#[tokio::test]
async fn sourcify_connection_refusal_is_reported() {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let endpoint = format!("http://{}/server", listener.local_addr().unwrap());
    drop(listener);
    let client = ClientBuilder::default()
        .try_base_url(&endpoint)
        .unwrap()
        .max_retries(0)
        .operation_timeout(Duration::from_secs(2))
        .build();
    let error = client
        .verify_from_etherscan("10", bytes(ADDRESS))
        .await
        .unwrap_err();
    assert!(matches!(error, Error::Reqwest(error) if error.is_connect()));
}
