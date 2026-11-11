fn require_err<T: std::fmt::Debug, E>(result: Result<T, E>, message: &str) -> E {
    result
        .err()
        .unwrap_or_else(|| panic!("{message}: expected an error"))
}

fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::collections::BTreeSet;

use layerx_agentd::{agent_rpc, agent_rpc_dispatch, session, store};

use agent_rpc::{
    decode, dispatch_target, error_body, http_status, mcp_result_response, success_body,
    version_exchange, Rejection, ENVELOPE_VERSION,
};
use agent_rpc_dispatch::Dispatched;
use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agent_api::verify::{Rung, RungStatus};
use layerx_agent_api::{
    agent_api_schema_v1, ContractVersion, Operation, Sequence, VersionRequest, VersionResponse,
    AGENT_API_V1_SOURCE,
};
use serde_json::{json, Value};
use session::{SessionCredential, SessionId};
use store::TenantId;

fn envelope(operation: Operation) -> Value {
    let mut body = json!({
        "version": 1,
        "request_id": "7",
        "operation": operation.name(),
        "request": {},
        "credential": {
            "tenant": "rpc-surface",
            "session_id": "11".repeat(32),
            "token_id": "22".repeat(32),
            "generation": "1",
        },
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
    let retired_operations: BTreeSet<_> = AGENT_API_V1_SOURCE
        .split("\n[")
        .filter_map(|section| {
            let (header, body) = section.split_once(']')?;
            let name = header.strip_prefix("transport.agent_http.operation.")?;
            body.lines()
                .any(|line| line.trim() == r#"disposition = "retired""#)
                .then_some(name)
        })
        .collect();
    assert_eq!(generated_operations.len(), Operation::ALL.len());
    assert!(generated_operations.is_disjoint(&retired_operations));
    assert_eq!(
        generated_operations
            .union(&retired_operations)
            .copied()
            .collect::<BTreeSet<_>>(),
        schema_operations
    );

    for operation in retired_operations {
        let mut body = envelope(Operation::TenantReadiness);
        body["operation"] = json!(operation);
        let error = decode(body.to_string().as_bytes())
            .err()
            .unwrap_or_else(|| panic!("retired operation {operation} accepted"));
        assert_refusal(
            &error,
            ErrorClass::UnavailableCapability,
            &format!("unavailable_capability.{operation}"),
        );
    }

    for &operation in Operation::ALL {
        let mut body = envelope(operation);
        let decoded = decode(body.to_string().as_bytes())
            .unwrap_or_else(|error| panic!("{}: {error:?}", operation.name()));
        let target = dispatch_target(operation, RequestId(7));
        assert_eq!(decoded.operation, operation);
        assert_eq!(
            decoded.credential,
            Some(SessionCredential::new(
                require_ok(TenantId::new("rpc-surface"), "surface tenant"),
                SessionId([0x11; 32]),
                [0x22; 32],
                1,
            ))
        );
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

        body["credential"] = Value::Null;
        let error = decode(body.to_string().as_bytes())
            .err()
            .unwrap_or_else(|| panic!("{} accepted without credentials", operation.name()));
        assert_refusal(
            &error,
            ErrorClass::ProtocolIncompatibility,
            "envelope.credential",
        );
    }

    finish_every_schema_operation_is_dispatched_retired_or_bootstrap_gated_1_rpc_surface_major_two(
    );
}

fn finish_every_schema_operation_is_dispatched_retired_or_bootstrap_gated_1_rpc_surface_major_two()
{
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
fn retired_faucet_claim_wire_name_is_refused() {
    let mut retired = envelope(Operation::TenantReadiness);
    retired["operation"] = json!("faucet.claim");
    let error = decode(retired.to_string().as_bytes())
        .err()
        .unwrap_or_else(|| panic!("retired operation accepted"));
    assert_refusal(
        &error,
        ErrorClass::UnavailableCapability,
        "unavailable_capability.faucet.claim",
    );
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
        answer,
        require_ok(
            layerx_agentd::test_support::version_exchange(&request),
            "required test value"
        )
    );
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
        let error = require_err(version_exchange(&request), "required test value");
        assert_refusal(
            &error,
            ErrorClass::ProtocolIncompatibility,
            "envelope.version",
        );
        assert_eq!(http_status(&error), 400);
    }
    for length in 0..request.len() {
        let error = require_err(version_exchange(&request[..length]), "required test value");
        assert_eq!(error.class, ErrorClass::ProtocolIncompatibility);
        assert_eq!(error.reason, "envelope.malformed");
    }
    let mut trailing = request.to_vec();
    trailing.push(0);
    assert_refusal(
        &require_err(version_exchange(&trailing), "required test value"),
        ErrorClass::ProtocolIncompatibility,
        "envelope.malformed",
    );
    for index in [12, 13, 14, 18] {
        let mut malformed = request;
        malformed[index] = 1;
        assert_refusal(
            &require_err(version_exchange(&malformed), "required test value"),
            ErrorClass::ProtocolIncompatibility,
            "envelope.malformed",
        );
    }
    for index in [1, 16] {
        let mut incompatible = request;
        incompatible[index] = 1;
        assert_refusal(
            &require_err(version_exchange(&incompatible), "required test value"),
            ErrorClass::ProtocolIncompatibility,
            "envelope.version",
        );
    }
}

#[test]
fn envelope_version_other_than_one_is_refused() {
    assert_eq!(ENVELOPE_VERSION, 1);
    for operation in [Operation::TenantReadiness, Operation::SessionOpen] {
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
            require_ok(TenantId::new("tenant-a"), "golden tenant"),
            SessionId([0xcd; 32]),
            [0xab; 32],
            7,
        ))
    );
    assert_eq!(
        dispatch_target(decoded.operation, decoded.request_id),
        Ok(Operation::TenantReadiness)
    );

    let mut unsupported: Value = require_ok(serde_json::from_slice(request), "request JSON");
    unsupported["version"] = json!(2);
    let error = require_some(
        decode(unsupported.to_string().as_bytes()).err(),
        "version-2 request must be refused",
    );
    assert_refusal(
        &error,
        ErrorClass::ProtocolIncompatibility,
        "envelope.version",
    );

    let response = mcp_result_response(decoded.request_id, json!({ "account": "ready" }));
    assert_eq!(response.status, 200);
    assert_eq!(response.body.as_slice(), success);
    for (answer, golden, state) in [
        (
            Dispatched {
                value: json!({
                    "found": false,
                    "submitted": {"tx_hash": format!("0x{}", "ab".repeat(32)), "nonce": "7"},
                }),
                verification: Some(RungStatus::Achieved(Rung::Submitted)),
            },
            include_bytes!("../../../schema/agent-api/golden/agent-http-success-pending.json")
                .as_slice(),
            "pending",
        ),
        (
            Dispatched {
                value: json!({"found": false}),
                verification: None,
            },
            include_bytes!("../../../schema/agent-api/golden/agent-http-success-absent.json")
                .as_slice(),
            "absent",
        ),
    ] {
        let response = require_ok(
            success_body(decoded.request_id, &answer, Some(Some(Rung::Receipted))),
            "not-ready program.receipt envelope",
        );
        assert_eq!(response.status, 200);
        assert_eq!(response.body.as_slice(), golden);
        let body: Value = require_ok(serde_json::from_slice(&response.body), "success JSON");
        assert_eq!(body["verification_status"]["state"], state);
        assert_eq!(body["verification_status"]["requested"], "receipted");
        assert!(body["verification_status"].get("rung").is_none());
    }
    let rejection = Rejection {
        class: ErrorClass::PolicyRefusal,
        retriability: Retriability::Terminal,
        request_id: decoded.request_id,
        reason: "policy.refused",
    };
    assert_eq!(http_status(&rejection), 403);
    assert_eq!(error_body(&rejection).as_slice(), refusal);
}
