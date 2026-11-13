use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::web_hold;
use web_hold::*;

use layerx_agent_api::TimestampSeconds;
use layerx_agentd::store::{ObjectKind, TenantKey};
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::boundary::{AgentSurface, ProgramReads};
use layerx_mcp::server::{Server, WebBoundary, WebRoute};
use layerx_mcp::stdio::{Bound, Session};
use layerx_mcp::tools::web::{WebConfig, WebToolError};
use layerx_types::payload::ModuleRegistry;

fn search_key() -> [u8; 32] {
    Sha256::digest(b"x-websearch-test/receive/metered-SID").into()
}

fn payer_did() -> String {
    option_assertion::required(
        fixture("gateway/buyer.json")["payerDid"].as_str(),
        "configured payer DID",
    )
    .to_owned()
}

fn stored_key(prefix: &[u8]) -> TenantKey {
    let mut id = prefix.to_vec();
    id.extend_from_slice(&search_key());
    result_assertion::required(
        TenantKey::new(tenant(), ObjectKind::PreparedActivity, id),
        "durable key",
    )
}

fn invoke(served: &mut Session<WebBoundary<ProgramReads>>) -> Value {
    let input = json!({"jsonrpc":"2.0", "id":1, "method":"tools/call", "params":{
        "name":"web.search", "arguments":{
            "query":"paxeer", "currency":"SID", "scheme":"metered", "idempotency_key":hex(&search_key())
        }
    }});
    let mut source = std::io::Cursor::new(format!("{input}\n"));
    let mut output = Vec::new();
    result_assertion::required(served.serve(&mut source, &mut output), "MCP line transport");
    result_assertion::required(serde_json::from_slice(&output), "served response")
}

fn session(
    enrolled: &Enrolled,
    recording: &Recording,
    registry: Arc<ApprovalRegistry>,
    resources: WebApprovalResources,
    traffic: Arc<Mutex<Traffic>>,
) -> Session<WebBoundary<ProgramReads>> {
    let server = result_assertion::required(
        Server::bind(
            enrolled.control.clone(),
            enrolled.credential.clone(),
            CapabilityId([9; 32]),
            HEIGHT,
            enrolled
                .directory
                .0
                .join(format!("audit-{}", NEXT.fetch_add(1, Ordering::Relaxed))),
        ),
        "authenticated server",
    );
    let buyer = fixture("gateway/buyer.json");
    let receiver = result_assertion::required(
        unhex(option_assertion::required(
            fixture("gateway/metered.json")["ledger"]["receiverAddress"].as_str(),
            "receiver address",
        ))
        .try_into(),
        "receiver width",
    );
    let config = result_assertion::required(
        WebConfig::new(
            &recording.endpoint(),
            &payer_did(),
            &format!("0x{}", hex(&[0xab; 20])),
            receiver,
            "eip155:125",
            word(&buyer["sequencerPublicKey"]),
            word(&buyer["sequencerId"]),
            option_assertion::required(buyer["firstBatchNumber"].as_u64(), "first height"),
            option_assertion::required(buyer["lastBatchNumber"].as_u64(), "last height"),
            Duration::from_secs(5),
            1,
        ),
        "web config",
    );
    let payer = RecordedNodePayer {
        resources,
        traffic,
        nonce: 7,
        sender: [0xab; 20],
        corrupt_hash: false,
        node: result_assertion::required(
            SequencerEndpoint::new(&recording.endpoint(), &[]),
            "real node client",
        ),
        receipt: result_assertion::required(
            layerx_sdk::rpc::RpcClient::connect_node(&recording.endpoint(), None),
            "real receipt client",
        ),
    };
    let route = WebRoute::new(
        config,
        registry,
        ApprovalPolicy {
            amount_threshold: 0,
        },
        Box::new(payer),
        Arc::new(test_clock::TestClock::new(HEIGHT)),
    );
    let surface = result_assertion::required(
        AgentSurface::new(
            &recording.address,
            "b".repeat(48),
            &"cc".repeat(32),
            Duration::from_secs(5),
        ),
        "daemon peer",
    );
    let boundary = result_assertion::required(
        server.route_web(ProgramReads::new(surface), route),
        "served route",
    );
    Session::new(Bound::Full(Box::new(server)), boundary)
}

fn assert_unpaid(traffic: &Mutex<Traffic>, recording: &Recording) {
    let traffic = result_assertion::required(traffic.lock(), "payer traffic");
    assert_eq!(traffic.preparations, 1);
    assert_eq!(traffic.grants, 1);
    assert_eq!(traffic.submits, 0);
    assert!(traffic.submitted.is_empty());
    assert!(
        result_assertion::required(recording.signatures.lock(), "payment signatures").is_empty()
    );
}

#[test]
fn metered_hold_uses_sender_actor_and_restart_remains_pending() {
    let recording = Recording::start_search();
    let enrolled = Enrolled::new(&["write:web:search"], &payer_did());
    let root = enrolled.directory.0.join("approvals");
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let (prepared, ticket) = publish_metered_hold(&enrolled, &recording, &root, &traffic);

    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        result_assertion::required(
            registry.replay_tenant(&tenant(), &recovered.limiter),
            "replay hold and budget"
        ),
        1
    );
    assert_eq!(
        result_assertion::required(
            recovered.preparation(&tenant(), search_key()),
            "re-derived preparation"
        ),
        Some(prepared)
    );
    assert_eq!(
        option_assertion::required(
            result_assertion::required(registry.ticket(search_key()), "recovered ticket"),
            "hold"
        )
        .disclosure,
        ticket.disclosure
    );
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        recovered.clone(),
        Arc::clone(&traffic),
    );
    let pending = invoke(&mut served);
    assert_eq!(pending["result"]["isError"], true, "{pending}");
    assert!(
        pending.to_string().contains("awaiting approval"),
        "{pending}"
    );
    assert!(
        pending.to_string().contains(&hex(&search_key())),
        "{pending}"
    );
    assert!(result_assertion::required(
        recovered.limiter.has_reservation(search_key()),
        "retained reservation"
    ));

    result_assertion::required(
        registry.validate_registry(
            &tenant(),
            &result_assertion::required(ModuleRegistry::new(&[]), "call registry"),
        ),
        "registry still accepts sender-bound hold after restart and pending retry",
    );
    assert_eq!(
        option_assertion::required(
            result_assertion::required(registry.ticket(search_key()), "pending ticket"),
            "hold"
        )
        .state,
        ApprovalState::AwaitingApproval
    );

    let decision = result_assertion::required(
        ApprovalService::new(&registry, &recovered.limiter, &recovered.expiry).reject(
            DecisionRequest {
                tenant: &tenant(),
                approval_id: search_key(),
                idempotency_key: &result_assertion::required(
                    DecisionKey::new("inspect-held-agent"),
                    "decision key",
                ),
                approver: result_assertion::required(ApproverId::new("operator"), "approver"),
                current_sequence: HEIGHT,
                current_time: TimestampSeconds(HEIGHT),
            },
        ),
        "real rejection exposes persisted hold context in audit",
    );
    assert_eq!(decision.outcome, ApprovalOutcome::Rejected);
    let audit = option_assertion::required(
        result_assertion::required(registry.audit_entry(search_key()), "hold audit"),
        "terminal audit",
    );
    assert_eq!(audit.agent.as_bytes(), hex(&[0xab; 20]).as_bytes());
    assert_eq!(
        audit.agent.as_bytes(),
        ticket.disclosure.actor.as_str().as_bytes()
    );
    assert_eq!(audit.request_id, search_key());
    assert_unpaid(&traffic, &recording);
}

#[test]
fn undecodable_stored_spend_is_refused_without_payment() {
    for mutation in ["missing_sender", "unexpected_actor", "malformed_json"] {
        for keep_ticket in [true, false] {
            let recording = Recording::start_search();
            let enrolled = Enrolled::new(&["write:web:search"], &payer_did());
            let root = enrolled.directory.0.join("approvals");
            let initial = resources(&root);
            let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&initial.store)));
            let traffic = Arc::new(Mutex::new(Traffic::default()));
            let mut served = session(
                &enrolled,
                &recording,
                Arc::clone(&registry),
                initial.clone(),
                Arc::clone(&traffic),
            );
            assert_eq!(invoke(&mut served)["result"]["isError"], true);
            assert!(result_assertion::required(registry.ticket(search_key()), "ticket").is_some());
            assert_unpaid(&traffic, &recording);
            drop(served);
            drop(registry);
            drop(initial);

            let mut store = result_assertion::required(Store::open(&root), "stopped store");
            let storage_key = stored_key(b"web-spend-v1:");
            let mut record: Value = result_assertion::required(
                serde_json::from_slice(
                    option_assertion::required(store.get(&storage_key), "producer's spend").bytes(),
                ),
                "spend JSON",
            );
            assert!(record.get("actor").is_none());
            let bytes = match mutation {
                "missing_sender" => {
                    assert!(
                        option_assertion::required(record.as_object_mut(), "spend object")
                            .remove("sender")
                            .is_some()
                    );
                    result_assertion::required(serde_json::to_vec(&record), "missing sender JSON")
                }
                "unexpected_actor" => {
                    record["actor"] = json!(payer_did());
                    result_assertion::required(serde_json::to_vec(&record), "unknown actor JSON")
                }
                "malformed_json" => b"{".to_vec(),
                _ => unreachable!("declared corruption cases"),
            };
            result_assertion::required(
                store.put_local(storage_key, bytes),
                "persist undecodable spend",
            );
            if !keep_ticket {
                assert!(result_assertion::required(
                    store.remove_local(&stored_key(b"approval-hold-v1:")),
                    "interrupted hold publication"
                ));
            }
            drop(store);

            let recovered = resources(&root);
            assert!(matches!(
                recovered.preparation(&tenant(), search_key()),
                Err(WebToolError::ApprovalChanged)
            ));
            let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
            assert_eq!(
                result_assertion::required(
                    registry.replay_tenant(&tenant(), &recovered.limiter),
                    "replay registry"
                ),
                usize::from(keep_ticket)
            );
            let mut served = session(
                &enrolled,
                &recording,
                Arc::clone(&registry),
                recovered,
                Arc::clone(&traffic),
            );
            let response = invoke(&mut served);
            assert_eq!(
                response["result"]["isError"], true,
                "{mutation}, ticket={keep_ticket}: {response}"
            );
            assert_eq!(response["result"]["structuredContent"]["refusal"],
                "the daemon response is unusable: the web spend differs from the disclosure under approval",
                "{mutation}, ticket={keep_ticket}: {response}");
            assert_eq!(
                result_assertion::required(registry.ticket(search_key()), "no replacement hold")
                    .is_some(),
                keep_ticket
            );
            assert_unpaid(&traffic, &recording);
        }
    }
}

fn publish_metered_hold(
    enrolled: &Enrolled,
    recording: &Recording,
    root: &std::path::Path,
    traffic: &Arc<Mutex<Traffic>>,
) -> (
    layerx_agent_api::prepare::Prepared,
    layerx_agentd::policy::approval::ApprovalTicket,
) {
    let initial = resources(root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&initial.store)));
    let mut served = session(
        enrolled,
        recording,
        Arc::clone(&registry),
        initial.clone(),
        Arc::clone(traffic),
    );
    let response = invoke(&mut served);
    assert_eq!(response["result"]["isError"], true, "{response}");
    let ticket = option_assertion::required(
        result_assertion::required(registry.ticket(search_key()), "ticket lookup"),
        "held search",
    );
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    assert_eq!(ticket.hold_id, search_key());
    assert!(
        response.to_string().contains(&hex(&ticket.hold_id)),
        "{response}"
    );
    assert_eq!(ticket.disclosure.amounts.values()[0].amount.0, 3114);
    assert_eq!(ticket.disclosure.actor.as_str(), hex(&[0xab; 20]));
    assert_ne!(ticket.disclosure.actor.as_str(), payer_did());
    result_assertion::required(
        registry.validate_registry(
            &tenant(),
            &result_assertion::required(ModuleRegistry::new(&[]), "call registry"),
        ),
        "registry accepts sender-bound held search",
    );
    {
        let store = result_assertion::required(initial.store.lock(), "approval store");
        let record: Value = result_assertion::required(
            serde_json::from_slice(
                option_assertion::required(
                    store.get(&stored_key(b"web-spend-v1:")),
                    "stored spend",
                )
                .bytes(),
            ),
            "stored JSON",
        );
        assert!(record.get("actor").is_none());
        assert_eq!(record["sender"], json!(vec![0xabu8; 20]));
    }
    let prepared = option_assertion::required(
        result_assertion::required(initial.preparation(&tenant(), search_key()), "preparation"),
        "held preparation",
    );
    assert_eq!(prepared.disclosure, ticket.disclosure);
    let call = result_assertion::required(
        decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes()),
        "metered grant is held as an unsigned EVM transaction",
    );
    assert_eq!(prepared.signing_preimage.as_bytes(), call.signing_hash());
    assert_unpaid(traffic, recording);
    drop(served);
    drop(registry);
    drop(initial);
    (prepared, ticket)
}
