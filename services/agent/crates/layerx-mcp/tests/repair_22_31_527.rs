//! Timed approval proof through SDK envelopes, the daemon binding and the real web route.

#[path = "support/web_hold.rs"]
mod web_hold;
use web_hold::*;

use layerx_agent_api::error::RequestId;
use layerx_agent_api::TimestampSeconds;
use layerx_agentd::session_control::McpInvocationIdentity;
use layerx_mcp::binding::{Binding, WebAuthority};
use layerx_mcp::boundary::BoundaryRefusal;
use layerx_sdk::agent_envelope::{encode_envelope, EnvelopeCredential};
use layerx_types::clock::Clock;

impl test_clock::TestClock {
    fn zero_generation(seconds: u64) -> Self {
        let mut clock = Self::new(seconds);
        clock.generation = [0; 16];
        clock
    }

    fn advance(&self, seconds: u64) {
        self.ticks
            .lock()
            .expect("test clock")
            .advance(seconds)
            .expect("clock advance");
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
    fs::write(&bearer, "b".repeat(48)).expect("protected bearer");
    fs::set_permissions(&bearer, fs::Permissions::from_mode(0o600)).expect("bearer mode");
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
    let mut binding = Binding::parse(&document.to_string()).expect("operator binding");
    binding.attach_web(WebAuthority::new(
        clock,
        registry,
        Box::new(RecordedNodePayer {
            resources,
            traffic,
            nonce: 7,
            sender: [0xab; 20],
            corrupt_hash: false,
            node: SequencerEndpoint::new(&recording.endpoint(), &[]).expect("real node client"),
            receipt: layerx_sdk::rpc::RpcClient::connect_node(&recording.endpoint(), None)
                .expect("real receipt client"),
        }),
    ));
    binding
}

fn served<F>(
    enrolled: &Enrolled,
    binding: &Binding,
    tool: &'static str,
    arguments: Value,
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
    let credential = EnvelopeCredential::new(
        "tenant-a",
        [7; 32],
        enrolled.credential.token_id(),
        enrolled.credential.generation(),
    )
    .expect("SDK session credential");
    let wire = encode_envelope(
        layerx_agent_api::Operation::McpInvoke,
        RequestId(sequence),
        &json!({"tool":tool,"arguments":arguments}),
        Some(&credential),
        Some(layerx_agent_api::idempotency::Key::new(invocation_key).expect("invocation key")),
    )
    .expect("real SDK producer");
    let bytes = serde_json::to_vec(&wire).expect("wire bytes");
    let decoded = layerx_agentd::agent_rpc::decode_mcp_invocation(&bytes)
        .unwrap_or_else(|_| panic!("daemon decoder"))
        .expect("MCP envelope");
    binding
        .daemon_invocation(
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
        )
        .expect("authenticated daemon invocation")
        .release_current(HEIGHT, |value| Ok(value))
        .expect("authorized release")
}

fn fetch(enrolled: &Enrolled, binding: &Binding) -> Result<Value, BoundaryRefusal> {
    served(
        enrolled,
        binding,
        "web.fetch",
        json!({
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
    let ticket = registry
        .ticket(key())
        .expect("ticket lookup")
        .expect("held ticket");
    let queue = ApprovalSubmissionQueue::default();
    let response = served(
        enrolled,
        binding,
        "approval.approve",
        json!({
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
                clock
                    .sample(Duration::from_secs(1))
                    .expect("runtime clock")
                    .unix_seconds(),
            );
            let decision = ApprovalService::new(registry, &resources.limiter, &resources.expiry)
                .approve(
                    DecisionRequest {
                        tenant: credential.tenant(),
                        approval_id: ticket.hold_id,
                        idempotency_key: &DecisionKey::new("timed-web-approval")
                            .expect("decision key"),
                        approver: ApproverId::new("operator").expect("approver"),
                        current_sequence: HEIGHT,
                        current_time,
                    },
                    prepared,
                    &queue,
                )
                .expect("real approval service");
            assert_eq!(decision.outcome, ApprovalOutcome::Granted);
            assert_eq!(
                queue.prepared(decision.submission_ref.expect("submission reference")),
                Some(prepared.clone())
            );
            Ok(json!({"outcome":"granted"}))
        },
    )
    .expect("served approval");
    assert_eq!(response["outcome"], "granted");
    assert_eq!(
        registry
            .ticket(key())
            .expect("approved ticket")
            .expect("ticket")
            .state,
        ApprovalState::Approved
    );
}

fn timed_hold(restart: bool) {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch", "approve"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer DID"),
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
    let response = fetch(&enrolled, &route);
    assert!(
        matches!(response, Err(BoundaryRefusal::Held { hold_id, awaiting: false }) if hold_id == key()),
        "{response:?}"
    );
    let prepared = resources_before
        .preparation(&tenant(), key())
        .expect("preparation lookup")
        .expect("retained exact spend");
    let record = ApprovalService::new(
        &registry,
        &resources_before.limiter,
        &resources_before.expiry,
    )
    .get(&tenant(), key(), HEIGHT, TimestampSeconds(clock.now()))
    .expect("timed hold record");
    assert_eq!(
        record
            .presentation
            .expect("decision time")
            .created_at_unix_seconds,
        clock.now()
    );
    assert_eq!(record.held_activity.expiry, prepared.expiry);
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
    drop(route);
    if restart {
        drop(registry);
        drop(resources_before);
        clock.advance(5);
        let recovered = resources(&root);
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
        assert_eq!(
            registry
                .replay_tenant(&tenant(), &recovered.limiter)
                .expect("replay real durable hold"),
            1
        );
        let restored = ApprovalService::new(&registry, &recovered.limiter, &recovered.expiry)
            .get(&tenant(), key(), HEIGHT, TimestampSeconds(clock.now()))
            .expect("recovered record");
        assert_eq!(restored.presentation, record.presentation);
        assert_eq!(
            recovered
                .preparation(&tenant(), key())
                .expect("durable preparation"),
            Some(prepared.clone())
        );
        let route = binding(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            recovered.clone(),
            Arc::clone(&traffic),
            clock.clone(),
        );
        assert!(
            matches!(fetch(&enrolled, &route), Err(BoundaryRefusal::Held { hold_id, awaiting: true }) if hold_id == key())
        );
        approve_served(
            &enrolled,
            &route,
            &registry,
            &recovered,
            clock.as_ref(),
            &prepared,
        );
        assert_eq!(
            fetch(&enrolled, &route).expect("completed recovered spend")["settlement"]
                ["verificationLevel"],
            "sequencer-signed"
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
            fetch(&enrolled, &route).expect("completed spend")["settlement"]["verificationLevel"],
            "sequencer-signed"
        );
    }
    assert_eq!(traffic.lock().expect("traffic").submitted, vec![prepared]);
    assert_eq!(traffic.lock().expect("traffic").preparations, 1);
    assert_eq!(
        recording
            .signatures
            .lock()
            .expect("payment signatures")
            .len(),
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
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer DID"),
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
    assert!(registry.ticket(key()).expect("no held approval").is_none());
    assert!(resources
        .preparation(&tenant(), key())
        .expect("no durable spend")
        .is_none());
    assert!(!resources
        .limiter
        .has_reservation(key())
        .expect("no reservation"));
    assert_eq!(traffic.lock().expect("traffic").preparations, 0);
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
    assert!(recording.signatures.lock().expect("signatures").is_empty());
}
