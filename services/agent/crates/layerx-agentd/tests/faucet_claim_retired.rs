use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agent_api::Operation;
use layerx_agentd::agent_rpc::{decode, error_body, http_status};
use layerx_agentd::tenant::OperationClass;
use serde_json::{json, Value};

#[test]
fn faucet_claim_is_refused_as_retired_and_absent_from_the_catalogue() {
    assert!(!Operation::ALL.is_empty());
    for operation in Operation::ALL.iter().copied() {
        assert_ne!(operation.name(), "faucet.claim");
        assert!(!OperationClass::authorized_scopes(operation).contains(&"write:faucet:claim"));
    }

    for credential in [
        Value::Null,
        json!({
            "tenant": "faucet-retired",
            "session_id": "11".repeat(32),
            "token_id": "22".repeat(32),
            "generation": "1",
        }),
    ] {
        let body = json!({
            "version": 1,
            "request_id": "91",
            "operation": "faucet.claim",
            "request": {
                "did": "did:layerx:retired",
                "signer_public_key": "33".repeat(32),
            },
            "credential": credential,
        });
        let rejection = assert_some(
            decode(body.to_string().as_bytes()).err(),
            "retired faucet.claim must be refused before authorization",
        );
        assert_eq!(rejection.class, ErrorClass::UnavailableCapability);
        assert_eq!(rejection.reason, "unavailable_capability.faucet.claim");
        assert_eq!(rejection.retriability, Retriability::Terminal);
        assert_eq!(rejection.request_id, RequestId(91));
        assert_eq!(http_status(&rejection), 503);
        let response: Value = assert_ok(
            serde_json::from_slice(&error_body(&rejection)),
            "unexpected error",
        );
        assert_eq!(response["reason"], "unavailable_capability.faucet.claim");
        assert_eq!(response["request_id"], "91");
        let error = assert_some(rejection.into_api_error(), "typed retirement error");
        assert_eq!(error.class, ErrorClass::UnavailableCapability);
        assert_eq!(error.retriability, Retriability::Terminal);
    }
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

#[track_caller]
fn assert_some<T>(value: Option<T>, message: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{message}"),
    }
}
