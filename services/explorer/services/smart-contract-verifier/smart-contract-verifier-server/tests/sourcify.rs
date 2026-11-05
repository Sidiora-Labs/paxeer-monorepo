#[path = "support/repair_22_33_596/mod.rs"]
mod support;

use actix_web::http::StatusCode;
use serde_json::json;
use support::*;

#[actix_web::test]
async fn should_return_200() {
    for full in [true, false] {
        let request = direct_request();
        let (status, body) = call(DIRECT, request.clone(), vec![
            Exchange::new("POST", "/", Some(request), 200, json!({"result":[{"address":ADDRESS,"status":if full {"perfect"} else {"partial"}}]})),
            source_exchange(false, full),
        ]).await;
        assert_eq!(status, StatusCode::OK);
        assert_success(&body, false, full);
    }
}

#[actix_web::test]
async fn invalid_contracts() {
    // The three original refusal assertions remain active and now also check
    // the error text, with a controlled upstream and no public chain lookup.
    for message in [
        "does not have a contract",
        "Metadata file not found",
        "deployed and recompiled bytecode don't match",
    ] {
        let mut request = direct_request();
        if message == "Metadata file not found" {
            request["files"] = json!({});
        }
        let (status, body) = call(
            DIRECT,
            request.clone(),
            vec![Exchange::new(
                "POST",
                "/",
                Some(request),
                200,
                json!({"error":message}),
            )],
        )
        .await;
        assert_eq!(status, StatusCode::OK);
        assert_failure(&body, message);
    }
}

#[actix_web::test]
async fn invalid_input() {
    let mut request = direct_request();
    request["address"] = json!("invalid-address");
    let (status, body) = call(DIRECT, request.clone(), vec![
        Exchange::new("POST", "/", Some(request), 400, json!({"message":"Validation Error","errors":[{"field":"address","message":"Invalid address"}]})),
    ]).await;
    assert_eq!(status, StatusCode::BAD_REQUEST);
    assert!(std::str::from_utf8(&body)
        .unwrap()
        .contains("Invalid address"));
}

#[actix_web::test]
async fn upstream_failure() {
    let request = direct_request();
    let exchanges = (0..2)
        .map(|_| {
            Exchange::new(
                "POST",
                "/",
                Some(request.clone()),
                500,
                json!({"upstream":"unavailable"}),
            )
        })
        .collect();
    let (status, body) = call(DIRECT, request, exchanges).await;
    assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
    assert!(std::str::from_utf8(&body)
        .unwrap()
        .contains("error while making verification request to Sourcify"));
}
