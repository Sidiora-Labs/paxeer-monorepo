//! Timed approval proof through SDK envelopes, the daemon binding and the real web route.

use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::web_hold;
use web_hold::*;

use layerx_agent_api::error::RequestId;
use layerx_agent_api::TimestampSeconds;
use layerx_agentd::approval::ApprovalSubmissionQueue;
use layerx_agentd::session_control::McpInvocationIdentity;
use layerx_mcp::binding::{Binding, WebAuthority};
use layerx_mcp::boundary::BoundaryRefusal;
use layerx_sdk::agent_envelope::{encode_envelope, EnvelopeCredential};
use layerx_types::clock::Clock;

trait TestClockControl {
    fn zero_generation(seconds: u64) -> Self;
    fn advance(&self, seconds: u64);
}

impl TestClockControl for test_clock::TestClock {
    fn zero_generation(seconds: u64) -> Self {
        let mut clock = Self::new(seconds);
        clock.generation = [0; 16];
        clock
    }

    fn advance(&self, seconds: u64) {
        result_assertion::required(
            result_assertion::required(self.ticks.lock(), "test clock").advance(seconds),
            "clock advance",
        );
    }
}

fn binding(
    enrolled: &Enrolled,
    recording: &Recording,
    registry: Arc<ApprovalRegistry>,
    resources: WebApprovalResources,
    traffic: Arc<Mutex<Traffic>>,
    clock: Arc<dyn Clock>,
) -> Binding {
    let buyer = fixture("gateway/buyer.json");
    let bearer = enrolled.directory.0.join("bearer");
    result_assertion::required(fs::write(&bearer, "b".repeat(48)), "protected bearer");
    result_assertion::required(
        fs::set_permissions(&bearer, fs::Permissions::from_mode(0o600)),
        "bearer mode",
    );
    let document = json!({
        "mode":"full", "tenant":"tenant-a", "store":enrolled.directory.0.join("store"),
        "audit_root":enrolled.directory.0.join("audit"), "session_id":hex(&[7;32]),
        "session_token_file":enrolled.directory.0.join("token"),
        "session_generation":enrolled.credential.generation(), "capability_id":hex(&[9;32]),
        "core_height":HEIGHT, "deadline_ms":5000,
        "agent":{"endpoint":recording.address,"bearer_file":bearer,"probe_program":"cc".repeat(32)},
        "limit":{"id":hex(&[10;16]),"name":"mcp","scope":"tenant","scope_id":hex(&[1;32]),"ceiling":"1000","consumed":"0"},
        "web":{
            "endpoint":recording.endpoint(), "payer_address":format!("0x{}",hex(&[0xab;20])),
            "receiver_address":fixture("gateway/metered.json")["ledger"]["receiverAddress"],
            "network":"eip155:125",
            "receipt_key_index":[{"keyId":buyer["sequencerId"],"publicKey":buyer["sequencerPublicKey"],"firstHeight":buyer["firstBatchNumber"],"lastHeight":buyer["lastBatchNumber"]}],
            "sequencer_id":buyer["sequencerId"],"first_batch_number":buyer["firstBatchNumber"],"last_batch_number":buyer["lastBatchNumber"],
            "timeout_ms":5000,"pending_attempts":1,"approval_threshold":"0"
        }
    });
    let mut binding =
        result_assertion::required(Binding::parse(&document.to_string()), "operator binding");
    binding.attach_web(WebAuthority::new(
        clock,
        registry,
        Box::new(RecordedNodePayer {
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
        }),
    ));
    binding
}

fn served<F>(
    enrolled: &Enrolled,
    binding: &Binding,
    tool: &'static str,
    arguments: &Value,
    owner: F,
) -> Result<Value, BoundaryRefusal>
where
    F: FnMut(
        layerx_agentd::tenant::Operation,
        &SessionCredential,
        &Value,
        [u8; 32],
    ) -> Result<Value, layerx_agentd::agent_rpc::Rejection>,
{
    let sequence = NEXT.fetch_add(1, Ordering::Relaxed);
    let mut invocation_key = [1; 32];
    invocation_key[..8].copy_from_slice(&sequence.to_be_bytes());
    let credential = result_assertion::required(
        EnvelopeCredential::new(
            "tenant-a",
            [7; 32],
            enrolled.credential.token_id(),
            enrolled.credential.generation(),
        ),
        "SDK session credential",
    );
    let wire = result_assertion::required(
        encode_envelope(
            layerx_agent_api::Operation::McpInvoke,
            RequestId(sequence),
            &json!({"tool":tool,"arguments":arguments}),
            Some(&credential),
            Some(result_assertion::required(
                layerx_agent_api::idempotency::Key::new(invocation_key),
                "invocation key",
            )),
        ),
        "real SDK producer",
    );
    let bytes = result_assertion::required(serde_json::to_vec(&wire), "wire bytes");
    let decoded = option_assertion::required(
        layerx_agentd::agent_rpc::decode_mcp_invocation(&bytes)
            .unwrap_or_else(|_| panic!("daemon decoder")),
        "MCP envelope",
    );
    result_assertion::required(
        result_assertion::required(
            binding.daemon_invocation(
                enrolled.control.clone(),
                decoded.credential,
                HEIGHT,
                tool,
                &decoded.arguments,
                McpInvocationIdentity {
                    tool_name: tool,
                    idempotency_key: decoded.idempotency_key,
                    request_digest: Sha256::digest(&bytes).into(),
                },
                None,
                owner,
            ),
            "authenticated daemon invocation",
        )
        .release_current(HEIGHT, Ok),
        "authorized release",
    )
}

fn fetch(enrolled: &Enrolled, binding: &Binding) -> Result<Value, BoundaryRefusal> {
    served(
        enrolled,
        binding,
        "web.fetch",
        &json!({
            "url":"https://paxeer.app/data.json", "currency":"USDC", "scheme":"exact", "idempotency_key":hex(&key())
        }),
        |_, _, _, _| panic!("web calls must reach the real web route"),
    )
}

fn approve_served(
    enrolled: &Enrolled,
    binding: &Binding,
    registry: &ApprovalRegistry,
    resources: &WebApprovalResources,
    clock: &dyn Clock,
    prepared: &Prepared,
) {
    let ticket = option_assertion::required(
        result_assertion::required(registry.ticket(key()), "ticket lookup"),
        "held ticket",
    );
    let queue = ApprovalSubmissionQueue::default();
    let response = result_assertion::required(
        served(
            enrolled,
            binding,
            "approval.approve",
            &json!({
                "approval_id":hex(&ticket.hold_id), "held_digest":hex(&ticket.disclosure_digest),
                "current_sequence":HEIGHT.to_string()
            }),
            |operation, credential, arguments, _| {
                assert_eq!(operation, layerx_agentd::tenant::Operation::ApprovalApprove);
                assert_eq!(
                    arguments["held_digest"],
                    hex(&prepared.disclosure.canonical_digest)
                );
                let current_time = TimestampSeconds(
                    result_assertion::required(
                        clock.sample(Duration::from_secs(1)),
                        "runtime clock",
                    )
                    .unix_seconds(),
                );
                let decision = result_assertion::required(
                    ApprovalService::new(registry, &resources.limiter, &resources.expiry).approve(
                        DecisionRequest {
                            tenant: credential.tenant(),
                            approval_id: ticket.hold_id,
                            idempotency_key: &result_assertion::required(
                                DecisionKey::new("timed-web-approval"),
                                "decision key",
                            ),
                            approver: result_assertion::required(
                                ApproverId::new("operator"),
                                "approver",
                            ),
                            current_sequence: HEIGHT,
                            current_time,
                        },
                        prepared,
                        &queue,
                    ),
                    "real approval service",
                );
                assert_eq!(decision.outcome, ApprovalOutcome::Granted);
                assert_eq!(
                    queue.prepared(option_assertion::required(
                        decision.submission_ref,
                        "submission reference"
                    )),
                    Some(prepared.clone())
                );
                Ok(json!({"outcome":"granted"}))
            },
        ),
        "served approval",
    );
    assert_eq!(response["outcome"], "granted");
    assert_eq!(
        option_assertion::required(
            result_assertion::required(registry.ticket(key()), "approved ticket"),
            "ticket"
        )
        .state,
        ApprovalState::Approved
    );
}

fn timed_hold(restart: bool) {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch", "approve"],
        option_assertion::required(
            fixture("gateway/buyer.json")["payerDid"].as_str(),
            "payer DID",
        ),
    );
    let root = enrolled.directory.0.join("approvals");
    let resources_before = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(
        &resources_before.store,
    )));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let clock = Arc::new(test_clock::TestClock::new(1_800_000_000));
    let route = binding(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        resources_before.clone(),
        Arc::clone(&traffic),
        clock.clone(),
    );
    let (prepared, record) = assert_initial_timed_hold(
        &enrolled,
        &route,
        &resources_before,
        &registry,
        &traffic,
        clock.as_ref(),
    );
    drop(route);
    if restart {
        drop(registry);
        drop(resources_before);
        clock.advance(5);
        complete_recovered_timed_hold(
            &enrolled, &recording, &root, &traffic, &clock, &prepared, &record,
        );
    } else {
        let route = binding(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            resources_before.clone(),
            Arc::clone(&traffic),
            clock.clone(),
        );
        clock.advance(1);
        approve_served(
            &enrolled,
            &route,
            &registry,
            &resources_before,
            clock.as_ref(),
            &prepared,
        );
        assert_eq!(
            result_assertion::required(fetch(&enrolled, &route), "completed spend")["settlement"]
                ["verificationLevel"],
            "sequencer-signed"
        );
    }
    assert_eq!(
        result_assertion::required(traffic.lock(), "traffic").submitted,
        vec![prepared]
    );
    assert_eq!(
        result_assertion::required(traffic.lock(), "traffic").preparations,
        1
    );
    assert_eq!(
        result_assertion::required(recording.signatures.lock(), "payment signatures").len(),
        1
    );
}

#[test]
fn timed_hold_is_approved_through_the_binding_and_spend_completes() {
    timed_hold(false);
}

#[test]
fn restart_recovers_the_hold_and_advanced_runtime_clock_can_decide_it() {
    timed_hold(true);
}

#[test]
fn zero_generation_refuses_unavailable_without_a_hold() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        option_assertion::required(
            fixture("gateway/buyer.json")["payerDid"].as_str(),
            "payer DID",
        ),
    );
    let resources = resources(&enrolled.directory.0.join("approvals"));
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&resources.store)));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let route = binding(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        resources.clone(),
        Arc::clone(&traffic),
        Arc::new(test_clock::TestClock::zero_generation(1_800_000_000)),
    );
    let response = fetch(&enrolled, &route);
    assert!(
        matches!(response, Err(BoundaryRefusal::Unavailable(_))),
        "{response:?}"
    );
    assert!(result_assertion::required(registry.ticket(key()), "no held approval").is_none());
    assert!(result_assertion::required(
        resources.preparation(&tenant(), key()),
        "no durable spend"
    )
    .is_none());
    assert!(!result_assertion::required(
        resources.limiter.has_reservation(key()),
        "no reservation"
    ));
    assert_eq!(
        result_assertion::required(traffic.lock(), "traffic").preparations,
        0
    );
    assert!(result_assertion::required(traffic.lock(), "traffic")
        .submitted
        .is_empty());
    assert!(result_assertion::required(recording.signatures.lock(), "signatures").is_empty());
}

fn assert_initial_timed_hold(
    enrolled: &Enrolled,
    route: &Binding,
    resources_before: &WebApprovalResources,
    registry: &ApprovalRegistry,
    traffic: &Mutex<Traffic>,
    clock: &test_clock::TestClock,
) -> (
    layerx_agent_api::prepare::Prepared,
    layerx_agentd::approval::ApprovalRecord,
) {
    let response = fetch(enrolled, route);
    assert!(
        matches!(response, Err(BoundaryRefusal::Held { hold_id, awaiting: false }) if hold_id == key()),
        "{response:?}"
    );
    let prepared = option_assertion::required(
        result_assertion::required(
            resources_before.preparation(&tenant(), key()),
            "preparation lookup",
        ),
        "retained exact spend",
    );
    let record = result_assertion::required(
        ApprovalService::new(
            registry,
            &resources_before.limiter,
            &resources_before.expiry,
        )
        .get(&tenant(), key(), HEIGHT, TimestampSeconds(clock.now())),
        "timed hold record",
    );
    assert_eq!(
        option_assertion::required(record.presentation, "decision time").created_at_unix_seconds,
        clock.now()
    );
    assert_eq!(record.held_activity.expiry, prepared.expiry);
    assert!(result_assertion::required(traffic.lock(), "traffic")
        .submitted
        .is_empty());

    (prepared, record)
}

fn complete_recovered_timed_hold(
    enrolled: &Enrolled,
    recording: &Recording,
    root: &std::path::Path,
    traffic: &Arc<Mutex<Traffic>>,
    clock: &Arc<test_clock::TestClock>,
    prepared: &layerx_agent_api::prepare::Prepared,
    record: &layerx_agentd::approval::ApprovalRecord,
) {
    let recovered = resources(root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        result_assertion::required(
            registry.replay_tenant(&tenant(), &recovered.limiter),
            "replay real durable hold"
        ),
        1
    );
    let restored = result_assertion::required(
        ApprovalService::new(&registry, &recovered.limiter, &recovered.expiry).get(
            &tenant(),
            key(),
            HEIGHT,
            TimestampSeconds(clock.now()),
        ),
        "recovered record",
    );
    assert_eq!(restored.presentation, record.presentation);
    assert_eq!(
        result_assertion::required(
            recovered.preparation(&tenant(), key()),
            "durable preparation"
        ),
        Some(prepared.clone())
    );
    let route = binding(
        enrolled,
        recording,
        Arc::clone(&registry),
        recovered.clone(),
        Arc::clone(traffic),
        clock.clone(),
    );
    assert!(
        matches!(fetch(enrolled, &route), Err(BoundaryRefusal::Held { hold_id, awaiting: true }) if hold_id == key())
    );
    approve_served(
        enrolled,
        &route,
        &registry,
        &recovered,
        clock.as_ref(),
        prepared,
    );
    assert_eq!(
        result_assertion::required(fetch(enrolled, &route), "completed recovered spend")
            ["settlement"]["verificationLevel"],
        "sequencer-signed"
    );
}
