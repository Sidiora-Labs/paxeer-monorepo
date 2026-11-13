include!("../src/main.rs");

#[test]
fn capabilities_answer_has_no_custody_or_anchor() {
    let retained = serde_json::json!({
        "receipts": {"enabled": true, "versions": [1, 2]},
        "programs": false,
        "ledger": null,
        "network": "paxeer-x"
    });
    let mut upstream = retained.clone();
    upstream["custody"] = serde_json::json!({"enabled": true});
    upstream["anchor"] = serde_json::json!({"enabled": true});
    let mut answer = OutgoingResponse {
        status: 200,
        content_type: "application/json".to_owned(),
        headers: vec![("etag".to_owned(), "upstream-representation".to_owned()), ("x-content-sha256".to_owned(), "upstream-digest".to_owned()), ("vary".to_owned(), "Origin".to_owned())],
        body: serde_json::to_vec(&upstream).unwrap_or_else(|error| panic!("{}: {error:?}", "upstream capabilities encode")),
        retry_after: None,
    };
    let original_body = answer.body.clone();
    explorer_proxy::filter_capabilities_response("/api/v2/stats", &mut answer).unwrap_or_else(|error| panic!("{}: {error:?}", "other explorer responses pass through"));
    assert_eq!(answer.body, original_body);
    assert_eq!(answer.headers.len(), 3);

    explorer_proxy::filter_capabilities_response("/api/v2/paxeer-x/capabilities", &mut answer).unwrap_or_else(|error| panic!("{}: {error:?}", "capabilities response is filtered"));
    let body: serde_json::Value = serde_json::from_slice(&answer.body).unwrap_or_else(|error| panic!("{}: {error:?}", "filtered capabilities are JSON"));
    assert!(body.get("custody").is_none());
    assert!(body.get("anchor").is_none());
    assert_eq!(body, retained);
    assert_eq!(answer.status, 200);
    assert_eq!(answer.content_type, "application/json");
    assert_eq!(answer.headers, vec![("vary".to_owned(), "Origin".to_owned())]);

    let filtered_body = answer.body.clone();
    explorer_proxy::filter_capabilities_response("/api/v2/paxeer-x/capabilities", &mut answer).unwrap_or_else(|error| panic!("{}: {error:?}", "capabilities without retired keys remain valid"));
    assert_eq!(answer.body, filtered_body);

    for invalid in [b"not JSON".as_slice(), b"[]", b"null"] {
        answer.body = invalid.to_vec();
        assert!(explorer_proxy::filter_capabilities_response("/api/v2/paxeer-x/capabilities", &mut answer,).is_err());
    }
}
