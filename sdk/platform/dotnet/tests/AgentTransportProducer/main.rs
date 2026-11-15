// Compile the production daemon modules exactly as the daemon's own boundary tests do.
#[path = "../../../../../services/agent/crates/layerx-agentd/src/lib.rs"]
mod daemon;
pub use daemon::*;
#[path = "../../../../../services/agent/crates/layerx-agentd/src/agent_rpc_wire.rs"]
mod agent_rpc_wire;
#[path = "../../../../../services/agent/crates/layerx-agentd/src/outbound_tls.rs"]
mod outbound_tls;

use layerx_agent_api::error::{ErrorClass, ReasonCode, Retriability};
use layerx_agent_api::verify::{Rung, RungStatus};
use std::io::{Read, Write};

fn main() {
    let state = std::env::args().nth(1).expect("verification case");
    let mut body = Vec::new();
    std::io::stdin()
        .read_to_end(&mut body)
        .expect("SDK request bytes");
    if state == "gateway" {
        forward(&body);
        return;
    }
    let response = match agent_rpc::decode(&body) {
        Err(rejection) => agent_rpc::AgentRpcResponse {
            status: agent_rpc::http_status(&rejection),
            body: agent_rpc::error_body(&rejection),
        },
        Ok(envelope) => {
            assert_eq!(
                agent_rpc::dispatch_target(envelope.operation, envelope.request_id),
                Ok(envelope.operation)
            );
            assert!(envelope.credential.is_some());
            if state == "authorize" {
                owner_authorization(&envelope)
            } else if state == "refusal" {
                let rejection = agent_rpc::Rejection {
                    class: ErrorClass::PolicyRefusal,
                    request_id: envelope.request_id,
                    retriability: Retriability::Terminal,
                    reason: "policy.refused",
                };
                agent_rpc::AgentRpcResponse {
                    status: agent_rpc::http_status(&rejection),
                    body: agent_rpc::error_body(&rejection),
                }
            } else {
                let (value, verification) = match state.as_str() {
                    "achieved" => (
                        serde_json::json!({"account":"ready"}),
                        Some(RungStatus::Achieved(Rung::Submitted)),
                    ),
                    "unverified" => (
                        serde_json::json!({"account":"ready"}),
                        Some(RungStatus::Below {
                            requested: Rung::Receipted,
                            achieved: Rung::Submitted,
                            reason: ReasonCode::new("verification.below_requested")
                                .expect("machine-readable reason"),
                        }),
                    ),
                    "pending" => (
                        serde_json::json!({"found":false,"submitted":{"nonce":"7","tx_hash":format!("0x{}", "ab".repeat(32))}}),
                        Some(RungStatus::Achieved(Rung::Submitted)),
                    ),
                    "absent" => (serde_json::json!({"found":false}), None),
                    _ => panic!("unknown state"),
                };
                agent_rpc::success_body(
                    envelope.request_id,
                    &agent_rpc_dispatch::Dispatched {
                        value,
                        verification,
                    },
                    Some(Some(Rung::Receipted)),
                )
                .expect("real daemon producer")
            }
        }
    };
    let body: serde_json::Value =
        serde_json::from_slice(&response.body).expect("daemon JSON response");
    serde_json::to_writer(
        std::io::stdout().lock(),
        &serde_json::json!({"status":response.status,"body":body}),
    )
    .expect("emit response to real HTTP host");
    std::io::stdout().flush().expect("flush producer output");
}

// Exercise the exact HTTP forwarding client used by routes::agent_rpc, without its
// Redis-backed gateway API-key store (the offline task gate prohibits Redis).
fn forward(body: &[u8]) {
    use layerx_platform_gateway::http::{Client, Endpoint, OutboundRequest};
    let port: u16 = std::env::args()
        .nth(2)
        .expect("daemon port")
        .parse()
        .expect("port");
    let pem = std::env::args().nth(3).expect("CA file");
    let ca = native_tls::Certificate::from_pem(&std::fs::read(pem).expect("CA PEM")).expect("CA");
    let client = Client::without_identity(ca);
    let response = client
        .request_forwarded(
            &Endpoint::plain("127.0.0.1".to_owned(), port),
            "",
            &OutboundRequest {
                method: "POST",
                path: "/rpc",
                idempotency: None,
                content_type: "application/json",
                body,
            },
            &[],
        )
        .expect("production gateway forwarding exchange");
    assert_eq!(response.content_type, "application/json");
    let body: serde_json::Value = serde_json::from_slice(&response.body).expect("daemon JSON");
    serde_json::to_writer(
        std::io::stdout().lock(),
        &serde_json::json!({"status":response.status,"body":body}),
    )
    .expect("forwarded response");
}

fn owner_authorization(envelope: &agent_rpc::Envelope) -> agent_rpc::AgentRpcResponse {
    use layerx_types::ids::Did;
    use session::{OpenRequest, SessionId, SessionRecord, SessionRef, SessionRegistry};
    use std::collections::BTreeSet;
    use std::sync::{Arc, Mutex};
    use store::{Store, TenantId};
    let tenant = TenantId::new("tenant-a").expect("tenant");
    let id = SessionId([0xcd; 32]);
    let scopes = BTreeSet::from(["prepare".to_owned(), "read".to_owned()]);
    let activities = BTreeSet::from([5]);
    let mut registry = SessionRegistry::default();
    registry.replace(
        &SessionRef::new(tenant.clone(), id),
        SessionRecord {
            request: OpenRequest {
                session_id: id,
                token_id: [0xac; 32],
                tenant: tenant.clone(),
                agent: Did::new(b"did:layerx:repair138").expect("DID"),
                authority: identity::ProtocolAuthority::SessionKey([3; 32]),
                permitted_activity_types: activities.clone(),
                scopes: scopes.clone(),
                expiry_sequence: 100,
                expiry_seconds: None,
                opening_client: "repair138".to_owned(),
                policy_version: "v1".to_owned(),
            },
            open: true,
            sequence: 0,
            budget_reserved: 0,
            subscription_cursor: 0,
            generation: 6,
            retired_token_ids: BTreeSet::new(),
        },
    );
    let directory = std::env::temp_dir().join(format!("repair138-session-{}", std::process::id()));
    let mut store = Store::open(&directory).expect("real durable session store");
    // The production session rotation emits the persisted record and credential;
    // reopen that store before using the real authorization authority.
    let token = session::restrict_scope(
        &mut store,
        &mut registry,
        &tenant,
        id,
        [0xab; 32],
        scopes,
        activities,
    )
    .expect("real session rotation");
    assert_eq!(token.generation(), 7);
    drop(store);
    drop(registry);
    let store = Store::open(&directory).expect("reopen session store");
    let mut registry = SessionRegistry::default();
    registry
        .restore_tenant(&store, &tenant)
        .expect("recover producer session");
    let control = session_control::SessionControl::new(
        Arc::new(Mutex::new(store)),
        registry,
        Arc::new(prepare::PreparationLifecycle::default()),
        Arc::new(budget::BudgetLimiter::new(vec![]).expect("budget limiter")),
    );
    let response = match agent_rpc::authorize(
        &control,
        envelope,
        tenant::surface_for(envelope.operation),
        10,
        None,
    ) {
        Err(rejection) => agent_rpc::AgentRpcResponse {
            status: agent_rpc::http_status(&rejection),
            body: agent_rpc::error_body(&rejection),
        },
        Ok(permit) => {
            assert!(
                envelope.idempotency_key.is_some(),
                "catalogue mutation needs idempotency"
            );
            assert_eq!(permit.principal().tenant, tenant);
            agent_rpc::success_body(
                envelope.request_id,
                &agent_rpc_dispatch::Dispatched {
                    value: serde_json::json!({"tenant":permit.principal().tenant.as_str(),
                    "agent":"did:layerx:repair138", "generation":token.generation().to_string()}),
                    verification: Some(RungStatus::Achieved(Rung::Submitted)),
                },
                None,
            )
            .expect("authorized daemon response")
        }
    };
    drop(control);
    std::fs::remove_dir_all(directory).expect("remove private session store");
    response
}
