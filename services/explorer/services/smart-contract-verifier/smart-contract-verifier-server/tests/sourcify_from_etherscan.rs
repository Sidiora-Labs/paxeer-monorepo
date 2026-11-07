#[path = "support/repair_22_33_596/mod.rs"]
mod support;

use actix_web::http::StatusCode;
use serde_json::json;
use support::*;

#[actix_web::test]
async fn should_return_200() {
    for full in [true, false] {
        let (status, body) = call(IMPORT, json!({"address":ADDRESS,"chain":CHAIN}), vec![
            chains(true, true),
            Exchange::new("POST", "/verify/etherscan", Some(json!({"address":ADDRESS,"chainId":CHAIN})), 200, json!({"result":[{"address":ADDRESS,"chainId":CHAIN,"status":if full {"perfect"} else {"partial"},"libraryMap":{}}]})),
            source_exchange(true, full),
        ]).await;
        assert_eq!(status, StatusCode::OK);
        assert_success(&body, true, full);
    }
}

#[actix_web::test]
async fn chain_not_supported_fail() {
    for (chain, supported, etherscan_api) in [
        ("2221", true, true),
        (CHAIN, false, true),
        (CHAIN, true, false),
    ] {
        let (status, body) = call(
            IMPORT,
            json!({"address":ADDRESS,"chain":chain}),
            vec![chains(supported, etherscan_api)],
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_failure(
            &body,
            &format!("Chain {chain} is not supported for importing from Etherscan"),
        );
    }
}

#[actix_web::test]
async fn contract_not_verified_fail() {
    let message = "contract is not verified on Etherscan";
    let mut exchanges = vec![chains(true, true)];
    // The pinned upstream library retries 502 once before decoding the typed refusal.
    exchanges.extend((0..2).map(|_| {
        Exchange::new(
            "POST",
            "/verify/etherscan",
            Some(json!({"address":ADDRESS,"chainId":CHAIN})),
            502,
            json!({"error":message}),
        )
    }));
    let (status, body) = call(IMPORT, json!({"address":ADDRESS,"chain":CHAIN}), exchanges).await;
    assert_eq!(status, StatusCode::OK);
    assert_failure(&body, message);
}

#[actix_web::test]
async fn invalid_address_never_reaches_upstream() {
    let (status, body) = call(IMPORT, json!({"address":"0xnothex","chain":CHAIN}), vec![]).await;
    assert_eq!(status, StatusCode::BAD_REQUEST);
    assert!(std::str::from_utf8(&body)
        .unwrap()
        .contains("address is not a valid"));
}

#[actix_web::test]
async fn upstream_import_failure() {
    let (status, body) = call(
        IMPORT,
        json!({"address":ADDRESS,"chain":CHAIN}),
        vec![
            chains(true, true),
            Exchange::new(
                "POST",
                "/verify/etherscan",
                Some(json!({"address":ADDRESS,"chainId":CHAIN})),
                500,
                json!({"error":"import service unavailable"}),
            ),
        ],
    )
    .await;
    assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
    assert!(std::str::from_utf8(&body)
        .unwrap()
        .contains("import service unavailable"));
}
