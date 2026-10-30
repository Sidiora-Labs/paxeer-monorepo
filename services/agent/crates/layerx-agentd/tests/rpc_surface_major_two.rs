use std::collections::BTreeSet;

use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agent_api::{
    agent_api_schema_v1, ContractVersion, Operation, Sequence, VersionRequest, VersionResponse,
    AGENT_API_V1_SOURCE,
};
use layerx_agentd::agent_rpc::{
    decode, dispatch_target, error_body, http_status, mcp_result_response, version_exchange,
    Rejection, ENVELOPE_VERSION,
};
use layerx_agentd::session::{SessionCredential, SessionId};
use layerx_agentd::store::TenantId;
use serde_json::{json, Value};

fn envelope(operation: Operation) -> Value {
    let bootstrap = matches!(operation, Operation::AgentRegister | Operation::SessionOpen);
    let mut body = json!({
        "version": 1,
        "request_id": "7",
        "operation": operation.name(),
        "request": {},
        "credential": if bootstrap { Value::Null } else { json!({
            "tenant": "rpc-surface",
            "session_id": "11".repeat(32),
            "token_id": "22".repeat(32),
            "generation": "1",
        }) },
    });
    if operation.mutating() {
        body["idempotency_key"] = json!("33".repeat(32));
    }
    body
}

fn assert_refusal(error: &Rejection, class: ErrorClass, reason: &str) {
    assert_refusal_with_id(error, class, reason, RequestId(7));
}

fn assert_refusal_with_id(error: &Rejection, class: ErrorClass, reason: &str, id: RequestId) {
    assert_eq!(error.class, class);
    assert_eq!(error.reason, reason);
    assert_eq!(error.request_id, id);
    assert_eq!(error.retriability, Retriability::Terminal);
    let rendered: Value = serde_json::from_slice(&error_body(error))
        .unwrap_or_else(|error| panic!("error envelope: {error}"));
    assert_eq!(rendered["reason"], reason);
    assert_eq!(rendered["request_id"], id.0.to_string());
}

#[test]
fn every_schema_operation_is_dispatched_retired_or_bootstrap_gated() {
    let schema_operations: BTreeSet<_> = AGENT_API_V1_SOURCE
        .lines()
        .filter_map(|line| {
            line.strip_prefix("[transport.agent_http.operation.")?
                .strip_suffix(']')
        })
        .collect();
    let generated_operations: BTreeSet<_> = Operation::ALL.iter().map(|op| op.name()).collect();
    assert_eq!(generated_operations.len(), Operation::ALL.len());
    assert_eq!(generated_operations, schema_operations);

    for &operation in Operation::ALL {
        let body = envelope(operation).to_string();
        let decoded = decode(body.as_bytes());
        let target = dispatch_target(operation, RequestId(7));
        match operation {
            Operation::FaucetClaim => {
                let error = decoded
                    .err()
                    .unwrap_or_else(|| panic!("retired operation decoded"));
                assert_refusal(
                    &error,
                    ErrorClass::UnavailableCapability,
                    "unavailable_capability.faucet.claim",
                );
                assert_eq!(target, Err(error));
            }
            Operation::AgentRegister | Operation::SessionOpen => {
                let decoded = decoded.unwrap_or_else(|error| panic!("bootstrap decode: {error:?}"));
                assert_eq!(decoded.operation, operation);
                assert!(decoded.credential.is_none());
                let error = target
                    .err()
                    .unwrap_or_else(|| panic!("bootstrap reached adapter"));
                assert_refusal(
                    &error,
                    ErrorClass::PolicyRefusal,
                    "refused_pending_bootstrap_artifact",
                );
            }
            _ => {
                let decoded =
                    decoded.unwrap_or_else(|error| panic!("{}: {error:?}", operation.name()));
                assert_eq!(decoded.operation, operation);
                assert!(decoded.credential.is_some());
                assert_eq!(
                    target,
                    Ok(operation),
                    "{} must reach the owner dispatcher",
                    operation.name()
                );
                assert_eq!(
                    dispatch_target(decoded.operation, decoded.request_id),
                    target
                );
            }
        }
    }

    let mut unknown = envelope(Operation::TenantReadiness);
    unknown["operation"] = json!("unknown.operation");
    let error = decode(unknown.to_string().as_bytes())
        .err()
        .unwrap_or_else(|| panic!("unknown operation accepted"));
    assert_refusal(
        &error,
        ErrorClass::ProtocolIncompatibility,
        "envelope.unknown_operation",
    );
    let mut unknown_field = envelope(Operation::TenantReadiness);
    unknown_field["extra"] = json!(true);
    let error = decode(unknown_field.to_string().as_bytes())
        .err()
        .unwrap_or_else(|| panic!("unknown field accepted"));
    assert_refusal_with_id(
        &error,
        ErrorClass::ProtocolIncompatibility,
        "envelope.unknown_field",
        RequestId(0),
    );

    let statuses = [
        (ErrorClass::TransportFailure, 500),
        (ErrorClass::Deadline, 500),
        (ErrorClass::ProtocolIncompatibility, 400),
        (ErrorClass::UnavailableCapability, 503),
        (ErrorClass::CoreRejection, 500),
        (ErrorClass::VerificationFailure, 500),
        (ErrorClass::PolicyRefusal, 403),
        (ErrorClass::CapabilityRefusal, 403),
        (ErrorClass::BudgetRefusal, 403),
        (ErrorClass::RateLimit, 500),
        (ErrorClass::IdempotencyConflict, 409),
        (ErrorClass::InternalFault, 500),
    ];
    assert_eq!(statuses.len(), ErrorClass::ALL.len());
    for ((class, status), expected_class) in statuses.into_iter().zip(ErrorClass::ALL) {
        assert_eq!(class, *expected_class);
        assert_eq!(
            http_status(&Rejection {
                class,
                retriability: Retriability::Terminal,
                request_id: RequestId(7),
                reason: "surface.refusal",
            }),
            status
        );
    }
}

#[test]
fn version_exchange_reports_contract_major_two() {
    let schema = agent_api_schema_v1();
    assert_eq!(schema.version, ContractVersion { major: 2, minor: 0 });
    let request = VersionRequest {
        request_id: Sequence(7),
        supported: schema.version,
    }
    .encode();
    assert_eq!(
        hex::encode(request),
        "00020000000000000000000700000000020000"
    );
    let answer =
        version_exchange(&request).unwrap_or_else(|error| panic!("version exchange: {error:?}"));
    assert_eq!(
        hex::encode(answer),
        "0002000000000000000000070000000000000200000001"
    );
    assert_eq!(
        answer,
        VersionResponse {
            request_id: Sequence(7),
            contract: schema.version,
            node_interface_major: schema.node_interface_major,
        }
        .encode()
    );

    for minor in [0, 1, u16::MAX] {
        let request = VersionRequest {
            request_id: Sequence(u64::MAX),
            supported: ContractVersion { major: 2, minor },
        }
        .encode();
        let answer = version_exchange(&request)
            .unwrap_or_else(|error| panic!("compatible minor: {error:?}"));
        assert_eq!(
            answer,
            VersionResponse {
                request_id: Sequence(u64::MAX),
                contract: schema.version,
                node_interface_major: schema.node_interface_major,
            }
            .encode()
        );
    }
    for major in [0, 1, 3, u16::MAX] {
        let request = VersionRequest {
            request_id: Sequence(7),
            supported: ContractVersion { major, minor: 0 },
        }
        .encode();
        let error = version_exchange(&request).unwrap_err();
        assert_refusal(
            &error,
            ErrorClass::ProtocolIncompatibility,
            "envelope.version",
        );
        assert_eq!(http_status(&error), 400);
    }
    for length in 0..request.len() {
        let error = version_exchange(&request[..length]).unwrap_err();
        assert_eq!(error.class, ErrorClass::ProtocolIncompatibility);
        assert_eq!(error.reason, "envelope.malformed");
    }
    let mut trailing = request.to_vec();
    trailing.push(0);
    assert_refusal(
        &version_exchange(&trailing).unwrap_err(),
        ErrorClass::ProtocolIncompatibility,
        "envelope.malformed",
    );
    for index in [12, 13, 14, 18] {
        let mut malformed = request;
        malformed[index] = 1;
        assert_refusal(
            &version_exchange(&malformed).unwrap_err(),
            ErrorClass::ProtocolIncompatibility,
            "envelope.malformed",
        );
    }
    for index in [1, 16] {
        let mut incompatible = request;
        incompatible[index] = 1;
        assert_refusal(
            &version_exchange(&incompatible).unwrap_err(),
            ErrorClass::ProtocolIncompatibility,
            "envelope.version",
        );
    }
}

#[test]
fn envelope_version_other_than_one_is_refused() {
    assert_eq!(ENVELOPE_VERSION, 1);
    for operation in [
        Operation::TenantReadiness,
        Operation::FaucetClaim,
        Operation::SessionOpen,
    ] {
        for version in 0..=u8::MAX {
            if version == ENVELOPE_VERSION {
                continue;
            }
            let mut body = envelope(operation);
            body["version"] = json!(version);
            let error = decode(body.to_string().as_bytes())
                .err()
                .unwrap_or_else(|| panic!("version {version} accepted"));
            assert_refusal(
                &error,
                ErrorClass::ProtocolIncompatibility,
                "envelope.version",
            );
            assert_eq!(http_status(&error), 400);
        }
    }
}

#[test]
fn agent_http_envelope_goldens_round_trip() {
    let request = include_bytes!("../../../schema/agent-api/golden/agent-http-request.json");
    let success = include_bytes!("../../../schema/agent-api/golden/agent-http-success.json");
    let refusal = include_bytes!("../../../schema/agent-api/golden/agent-http-refusal.json");

    let decoded = decode(request).unwrap_or_else(|error| panic!("Go request golden: {error:?}"));
    assert_eq!(decoded.request_id, RequestId(7));
    assert_eq!(decoded.operation, Operation::TenantReadiness);
    assert_eq!(decoded.idempotency_key, None);
    assert!(decoded.request.is_empty());
    assert_eq!(
        decoded.credential,
        Some(SessionCredential::new(
            TenantId::new("tenant-a").expect("golden tenant"),
            SessionId([0xcd; 32]),
            [0xab; 32],
            7,
        ))
    );
    assert_eq!(
        dispatch_target(decoded.operation, decoded.request_id),
        Ok(Operation::TenantReadiness)
    );

    let mut unsupported: Value = serde_json::from_slice(request).expect("request JSON");
    unsupported["version"] = json!(2);
    let error = decode(unsupported.to_string().as_bytes())
        .err()
        .expect("version-2 request must be refused");
    assert_refusal(
        &error,
        ErrorClass::ProtocolIncompatibility,
        "envelope.version",
    );

    let response = mcp_result_response(decoded.request_id, json!({ "account": "ready" }));
    assert_eq!(response.status, 200);
    assert_eq!(response.body.as_slice(), success);
    let rejection = Rejection {
        class: ErrorClass::PolicyRefusal,
        retriability: Retriability::Terminal,
        request_id: decoded.request_id,
        reason: "policy.refused",
    };
    assert_eq!(http_status(&rejection), 403);
    assert_eq!(error_body(&rejection).as_slice(), refusal);
}
