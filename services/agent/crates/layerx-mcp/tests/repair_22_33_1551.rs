//! Task 22.33.1551 repair record.
//! web_hold_not_call: restart_hold_approval_and_exact_retry_through_served_route,
//! restart_refuses_changed_resource_offer_and_sender, changed_held_preparation_is_defective,
//! invalid_transaction_preparation_never_creates_a_hold.
//! Durable corruption: restart_refuses_changed_durable_transaction_and_sender.
//! Interrupted hold: restart_recovers_preparation_before_hold_publication.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd-host/src/payer.rs must
//! implement WebPayer::{approval_resources,prepare_transaction,submit_transaction} using
//! the owner's shared durable approval store/expiry/limits and exact EIP-1559 signing path.
//! Wiring request (domain 9): services/agent/crates/layerx-agentd-host/src/payer.rs must
//! obtain receiver-owned draw approval for above-threshold existing metered grants;
//! the payer's unsigned call cannot substitute for the sidecar's actual draw.
//! Exact-payment codec: restart_hold_approval_and_exact_retry_through_served_route
//! drives the web producer through EVM preparation, disclosure binding, durable hold,
//! reopen, approval and exact retry; pay_exact_codec_checks also proves strict ABI refusal.
//! Qualification: ed5358cce66b288c77c861bf58cf80d354663ecd;
//! cg spec done 22.33.1551 -f layerx-merge --root /work/paxeer-monorepo; exit 1;
//! /out/logs/verify-1.log: invalid_transaction_preparation_never_creates_a_hold passed;
//! restart_hold_approval_and_exact_retry_through_served_route,
//! restart_refuses_changed_resource_offer_and_sender, changed_held_preparation_is_defective,
//! restart_refuses_changed_durable_transaction_and_sender and
//! restart_recovers_preparation_before_hold_publication failed at the missing payExact codec.
//! Baseline: no earlier gate ledger or repair test existed; the explicit implementation-first,
//! wrapper-only gate order excludes the contradictory unchanged-tree verification run.
//! Continuation qualification: 36c6e1febc17409a21de554ff48341aadcc07607;
//! cg spec done 22.33.1551 -f layerx-merge --root /work/paxeer-monorepo; exit 1;
//! /out/logs/verify-1.log: three passed; the exact-retry recording omitted the JSON
//! Content-Type required by the real RPC client, and two substring checks did not
//! match the served ApprovalChanged refusal. Exact refusal equality is asserted below.
//! Qualification revision and private evidence are emitted by hf-build/hf-done.

#[path = "support/web_hold.rs"]
mod web_hold;
use web_hold::*;

use layerx_agentd::budget::{self, ReservationRequest};
use layerx_agentd::store::{ObjectKind, TenantKey};
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::boundary::{AgentSurface, ProgramReads, ToolBoundary};
use layerx_mcp::server::{Server, WebBoundary, WebRoute};
use layerx_mcp::stdio::{Bound, Session};
use layerx_mcp::tools::web::WebConfig;
use layerx_types::clock::Clock as _;
use layerx_types::payload::ModuleRegistry;

#[test]
fn restart_hold_approval_and_exact_retry_through_served_route() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer DID"),
    );
    let root = enrolled.directory.0.join("approvals");
    let original = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&original.store)));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        original.clone(),
        Arc::clone(&traffic),
        7,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    let response = invoke(&mut served);
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert!(
        response.to_string().contains(&hex(&key())),
        "served hold has the actual request identity: {response}"
    );
    let ticket = registry
        .ticket(key())
        .expect("ticket lookup")
        .expect("durable hold");
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    let prepared = original
        .preparation(&tenant(), key())
        .expect("retained preparation")
        .expect("prepared");
    let call = decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes())
        .expect("held bytes are unsigned EIP-1559");
    pay_exact_codec_checks(&call, &prepared);
    assert_eq!(prepared.disclosure.actor.as_str(), hex(&[0xab; 20]));
    assert_eq!(
        prepared.disclosure.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(call.encode_unsigned()))
    );
    assert_eq!(prepared.signing_preimage.as_bytes(), call.signing_hash());
    assert_eq!(prepared.disclosure, ticket.disclosure);
    assert_eq!(prepared.disclosure.fee_limit.0, 450_000);
    assert!(original
        .limiter
        .has_reservation(key())
        .expect("reservation"));
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
    assert!(recording.signatures.lock().expect("signatures").is_empty());
    registry
        .validate_registry(&tenant(), &ModuleRegistry::new(&[]).expect("call registry"))
        .expect("production registry validates sender-bound transaction");
    drop(served);
    drop(registry);
    drop(original);

    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        registry
            .replay_tenant(&tenant(), &recovered.limiter)
            .expect("reopen durable holds and budget reservations"),
        1
    );
    registry
        .validate_registry(&tenant(), &ModuleRegistry::new(&[]).expect("call registry"))
        .expect("recovered bytes and sender remain bound");
    assert_eq!(
        recovered
            .preparation(&tenant(), key())
            .expect("recovered preparation"),
        Some(prepared.clone())
    );
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        recovered.clone(),
        Arc::clone(&traffic),
        999,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    let pending = invoke(&mut served);
    assert_eq!(pending["result"]["isError"], true, "{pending}");
    assert!(
        pending.to_string().contains("awaiting approval"),
        "{pending}"
    );
    assert_eq!(
        traffic.lock().expect("traffic").preparations,
        1,
        "recovery cannot prepare a different nonce"
    );
    let excess = budget::reserve(
        &recovered.limiter,
        &ReservationRequest {
            id: [88; 32],
            amount: 1_000_000,
            expiry_sequence: 1000,
            current_sequence: HEIGHT,
            applicable_limits: recovered.limits.clone(),
        },
    );
    assert!(
        excess.is_err(),
        "the recovered payment reservation still counts against the ceiling"
    );
    assert_eq!(
        approve(&registry, &recovered, &prepared).outcome,
        ApprovalOutcome::Granted
    );
    for _ in 0..2 {
        let response = invoke(&mut served);
        assert_eq!(response["result"]["isError"], false, "{response}");
        assert_eq!(
            response["result"]["structuredContent"]["result"]["settlement"]["verificationLevel"],
            "sequencer-signed",
            "{response}"
        );
    }
    let traffic = traffic.lock().expect("traffic");
    assert_eq!(traffic.preparations, 1);
    assert_eq!(traffic.submitted, vec![prepared.clone(), prepared]);
    let signatures = recording.signatures.lock().expect("signatures");
    assert_eq!(signatures.len(), 2);
    assert_eq!(
        signatures[0], signatures[1],
        "retry preserves the complete payment signature envelope"
    );
}

#[test]
fn restart_refuses_changed_resource_offer_and_sender() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer"),
    );
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
        7,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    assert_eq!(invoke(&mut served)["result"]["isError"], true);
    let prepared = initial
        .preparation(&tenant(), key())
        .expect("preparation")
        .expect("held");
    drop(served);
    drop(registry);
    drop(initial);
    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        registry
            .replay_tenant(&tenant(), &recovered.limiter)
            .expect("replay"),
        1
    );
    assert_eq!(
        approve(&registry, &recovered, &prepared).outcome,
        ApprovalOutcome::Granted
    );
    let original_challenge = recording.challenge.lock().expect("challenge").clone();
    for mutation in ["resource", "offer", "sender"] {
        *recording.challenge.lock().expect("challenge") = original_challenge.clone();
        let sender = if mutation == "sender" {
            [0xac; 20]
        } else {
            [0xab; 20]
        };
        if mutation == "resource" {
            recording.challenge.lock().expect("challenge")["resource"]["description"] =
                json!("changed resource commitment");
        }
        if mutation == "offer" {
            for offer in recording.challenge.lock().expect("challenge")["accepts"]
                .as_array_mut()
                .expect("offers")
            {
                offer["amount"] = json!("1001");
            }
        }
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            recovered.clone(),
            Arc::clone(&traffic),
            999,
            sender,
            sender,
            false,
        );
        let response = invoke(&mut served);
        assert_eq!(
            response["result"]["isError"], true,
            "changed {mutation}: {response}"
        );
        assert_eq!(
            response["result"]["structuredContent"]["refusal"],
            "the daemon response is unusable: the web spend differs from the disclosure under approval",
            "changed {mutation}: {response}"
        );
    }
    assert_eq!(traffic.lock().expect("traffic").preparations, 1);
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
    assert!(recording.signatures.lock().expect("signatures").is_empty());
}

#[test]
fn changed_held_preparation_is_defective() {
    for mutation in ["bytes", "sender", "signing_hash"] {
        let recording = Recording::start();
        let enrolled = Enrolled::new(
            &["write:web:fetch"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer"),
        );
        let state = resources(&enrolled.directory.0.join("approvals"));
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&state.store)));
        let traffic = Arc::new(Mutex::new(Traffic::default()));
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            state.clone(),
            Arc::clone(&traffic),
            7,
            [0xab; 20],
            [0xab; 20],
            false,
        );
        assert_eq!(invoke(&mut served)["result"]["isError"], true);
        let mut prepared = state
            .preparation(&tenant(), key())
            .expect("retained preparation")
            .expect("prepared");
        match mutation {
            "bytes" => {
                let mut call = decode_unsigned(prepared.unsigned_canonical_bytes.as_bytes())
                    .expect("producer call");
                call.nonce += 1;
                prepared.unsigned_canonical_bytes =
                    layerx_agent_api::prepare::CanonicalBytes::new(call.encode_unsigned())
                        .expect("changed bytes");
            }
            "sender" => {
                prepared.disclosure.actor =
                    layerx_agent_api::identity::AgentDid::new(hex(&[0xac; 20]))
                        .expect("changed actor")
            }
            _ => {
                prepared.signing_preimage =
                    layerx_agent_api::prepare::SigningPreimage::new(vec![9; 32])
                        .expect("changed signing hash")
            }
        }
        assert_eq!(
            approve(&registry, &state, &prepared).outcome,
            ApprovalOutcome::Defective,
            "{mutation}"
        );
        assert!(!state
            .limiter
            .has_reservation(key())
            .expect("defective reservation released"));
        assert_eq!(invoke(&mut served)["result"]["isError"], true);
        assert!(traffic.lock().expect("traffic").submitted.is_empty());
        assert!(recording.signatures.lock().expect("signatures").is_empty());
    }
}

#[test]
fn invalid_transaction_preparation_never_creates_a_hold() {
    for (sender, corrupt_hash) in [([0xac; 20], false), ([0xab; 20], true)] {
        let recording = Recording::start();
        let enrolled = Enrolled::new(
            &["write:web:fetch"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer"),
        );
        let state = resources(&enrolled.directory.0.join("approvals"));
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&state.store)));
        let traffic = Arc::new(Mutex::new(Traffic::default()));
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            state.clone(),
            Arc::clone(&traffic),
            7,
            [0xab; 20],
            sender,
            corrupt_hash,
        );
        let response = invoke(&mut served);
        assert_eq!(response["result"]["isError"], true, "{response}");
        assert!(registry.hold_ids().expect("hold IDs").is_empty());
        assert!(state
            .preparation(&tenant(), key())
            .expect("journal")
            .is_none());
        assert!(!state.limiter.has_reservation(key()).expect("budget"));
        assert!(traffic.lock().expect("traffic").submitted.is_empty());
        assert!(recording.signatures.lock().expect("signatures").is_empty());
    }
}

#[test]
fn restart_refuses_changed_durable_transaction_and_sender() {
    for mutation in ["bytes", "sender", "signing_hash"] {
        let recording = Recording::start();
        let enrolled = Enrolled::new(
            &["write:web:fetch"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer"),
        );
        let root = enrolled.directory.0.join("approvals");
        let original = resources(&root);
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&original.store)));
        let traffic = Arc::new(Mutex::new(Traffic::default()));
        let mut served = session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            original.clone(),
            Arc::clone(&traffic),
            7,
            [0xab; 20],
            [0xab; 20],
            false,
        );
        assert_eq!(invoke(&mut served)["result"]["isError"], true);
        drop(served);
        drop(registry);
        drop(original);
        let mut store = Store::open(&root).expect("stopped component's durable store");
        let mut id = b"web-spend-v1:".to_vec();
        id.extend_from_slice(&key());
        let storage_key =
            TenantKey::new(tenant(), ObjectKind::PreparedActivity, id).expect("journal key");
        let mut record: Value =
            serde_json::from_slice(store.get(&storage_key).expect("producer's journal").bytes())
                .expect("journal codec");
        let byte = record[mutation][0].as_u64().expect("retained byte");
        record[mutation][0] = json!(byte ^ 1);
        store
            .put_local(
                storage_key,
                serde_json::to_vec(&record).expect("corrupted journal"),
            )
            .expect("persist changed state");
        drop(store);
        let recovered = resources(&root);
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
        assert_eq!(
            registry
                .replay_tenant(&tenant(), &recovered.limiter)
                .expect("real approval recovery"),
            1
        );
        let mut served = session(
            &enrolled,
            &recording,
            registry,
            recovered,
            Arc::clone(&traffic),
            999,
            [0xab; 20],
            [0xab; 20],
            false,
        );
        let response = invoke(&mut served);
        assert_eq!(
            response["result"]["isError"], true,
            "{mutation}: {response}"
        );
        assert_eq!(
            response["result"]["structuredContent"]["refusal"],
            "the daemon response is unusable: the web spend differs from the disclosure under approval",
            "{mutation}: {response}"
        );
        assert_eq!(traffic.lock().expect("traffic").preparations, 1);
        assert!(traffic.lock().expect("traffic").submitted.is_empty());
        assert!(recording.signatures.lock().expect("signatures").is_empty());
    }
}

#[test]
fn restart_recovers_preparation_before_hold_publication() {
    let recording = Recording::start();
    let enrolled = Enrolled::new(
        &["write:web:fetch"],
        fixture("gateway/buyer.json")["payerDid"]
            .as_str()
            .expect("payer"),
    );
    let root = enrolled.directory.0.join("approvals");
    let original = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&original.store)));
    let traffic = Arc::new(Mutex::new(Traffic::default()));
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        original.clone(),
        Arc::clone(&traffic),
        7,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    assert_eq!(invoke(&mut served)["result"]["isError"], true);
    let prepared = original
        .preparation(&tenant(), key())
        .expect("journal")
        .expect("prepared");
    drop(served);
    drop(registry);
    drop(original);
    let mut store = Store::open(&root).expect("stopped durable store");
    let mut id = b"approval-hold-v1:".to_vec();
    id.extend_from_slice(&key());
    let hold_key = TenantKey::new(tenant(), ObjectKind::PreparedActivity, id).expect("hold key");
    // The journal write precedes publication of the approval hold. Retain exactly the
    // producer's journal to reproduce a stop between those two durable writes.
    assert!(store
        .remove_local(&hold_key)
        .expect("interrupted publication"));
    drop(store);
    let recovered = resources(&root);
    let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&recovered.store)));
    assert_eq!(
        registry
            .replay_tenant(&tenant(), &recovered.limiter)
            .expect("interrupted hold recovery"),
        0
    );
    let mut served = session(
        &enrolled,
        &recording,
        Arc::clone(&registry),
        recovered.clone(),
        Arc::clone(&traffic),
        999,
        [0xab; 20],
        [0xab; 20],
        false,
    );
    assert_eq!(invoke(&mut served)["result"]["isError"], true);
    assert_eq!(
        registry
            .ticket(key())
            .expect("ticket")
            .expect("recovered hold")
            .disclosure,
        prepared.disclosure
    );
    assert!(recovered
        .limiter
        .has_reservation(key())
        .expect("recovered reservation"));
    assert_eq!(
        traffic.lock().expect("traffic").preparations,
        1,
        "the interrupted hold must retain the original transaction"
    );
    assert!(traffic.lock().expect("traffic").submitted.is_empty());
}

fn rpc<B: ToolBoundary>(session: &mut Session<B>, method: &str, params: Value) -> Value {
    let input = json!({"jsonrpc":"2.0", "id":1, "method":method, "params": params});
    let mut source = std::io::Cursor::new(format!("{input}\n"));
    let mut output = Vec::new();
    session
        .serve(&mut source, &mut output)
        .expect("MCP line transport");
    serde_json::from_slice(&output).expect("MCP response")
}

fn session(
    enrolled: &Enrolled,
    recording: &Recording,
    registry: Arc<ApprovalRegistry>,
    resources: WebApprovalResources,
    traffic: Arc<Mutex<Traffic>>,
    nonce: u64,
    configured_sender: [u8; 20],
    prepared_sender: [u8; 20],
    corrupt_hash: bool,
) -> Session<WebBoundary<ProgramReads>> {
    let server = Server::bind(
        enrolled.control.clone(),
        enrolled.credential.clone(),
        CapabilityId([9; 32]),
        HEIGHT,
        enrolled
            .directory
            .0
            .join(format!("audit-{}", NEXT.fetch_add(1, Ordering::Relaxed))),
    )
    .expect("authenticated MCP server");
    let buyer = fixture("gateway/buyer.json");
    let receiver = unhex(
        fixture("gateway/metered.json")["ledger"]["receiverAddress"]
            .as_str()
            .expect("receiver"),
    )
    .try_into()
    .expect("receiver address width");
    let config = WebConfig::new(
        &recording.endpoint(),
        buyer["payerDid"].as_str().expect("payer DID"),
        &format!("0x{}", hex(&configured_sender)),
        receiver,
        "eip155:125",
        word(&buyer["sequencerPublicKey"]),
        word(&buyer["sequencerId"]),
        buyer["firstBatchNumber"].as_u64().expect("first height"),
        buyer["lastBatchNumber"].as_u64().expect("last height"),
        Duration::from_secs(5),
        1,
    )
    .expect("web config");
    let payer = RecordedNodePayer {
        resources,
        traffic,
        nonce,
        sender: prepared_sender,
        corrupt_hash,
        node: SequencerEndpoint::new(&recording.endpoint(), &[]).expect("real node client"),
        receipt: layerx_sdk::rpc::RpcClient::connect_node(&recording.endpoint(), None)
            .expect("real receipt codec"),
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
    let surface = AgentSurface::new(
        &recording.address,
        "b".repeat(48),
        &"cc".repeat(32),
        Duration::from_secs(5),
    )
    .expect("daemon peer");
    let boundary = server
        .route_web(ProgramReads::new(surface), route)
        .expect("real served web route");
    Session::new(Bound::Full(Box::new(server)), boundary)
}
fn invoke(session: &mut Session<WebBoundary<ProgramReads>>) -> Value {
    rpc(
        session,
        "tools/call",
        json!({"name":"web.fetch","arguments":{"url":"https://paxeer.app/data.json","currency":"USDC","scheme":"exact","idempotency_key":hex(&key())}}),
    )
}

fn approve(
    registry: &ApprovalRegistry,
    resources: &WebApprovalResources,
    prepared: &Prepared,
) -> layerx_agentd::approval::ApprovalDecision {
    let queue = ApprovalSubmissionQueue::default();
    let result = ApprovalService::new(registry, &resources.limiter, &resources.expiry)
        .approve(
            DecisionRequest {
                tenant: &tenant(),
                approval_id: key(),
                idempotency_key: &DecisionKey::new("approve-web").expect("decision key"),
                approver: ApproverId::new("operator").expect("approver"),
                current_sequence: HEIGHT,
                current_time: layerx_agent_api::TimestampSeconds(
                    test_clock::TestClock::new(HEIGHT)
                        .sample(std::time::Duration::from_secs(1))
                        .expect("clock")
                        .unix_seconds(),
                ),
            },
            prepared,
            &queue,
        )
        .expect("durable approval service");
    if result.outcome == ApprovalOutcome::Granted {
        assert_eq!(
            queue.prepared(result.submission_ref.expect("released reference")),
            Some(prepared.clone()),
            "approval queues the byte-exact original preparation"
        );
    }
    result
}

fn pay_exact_codec_checks(call: &layerx_crypto::evm_transaction::Eip1559Call, held: &Prepared) {
    use layerx_crypto::disclosure::call::{bind_call, decode_pay_exact, reencode, validate_call};
    use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};

    let sender = [0xab; 20];
    let payment = decode_pay_exact(call).expect("decode real web producer's exact call");
    let disclosed = bind_call(call, sender).expect("bind exact call to sender");
    assert_eq!(
        reencode(&disclosed).expect("reencode"),
        call.encode_unsigned()
    );
    assert_eq!(
        disclosed.signing_digest().expect("signing digest"),
        call.signing_hash()
    );
    assert_eq!(disclosed.actor, sender);
    assert_eq!(disclosed.authority, sender);
    assert_eq!(disclosed.activity_type.ordinal(), 5);
    assert_eq!(disclosed.counterparties.len(), 2);
    assert_eq!(disclosed.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(disclosed.counterparties[0].account, payment.payer);
    assert_eq!(
        disclosed.counterparties[1].role,
        CounterpartyRole::Recipient
    );
    assert_eq!(disclosed.counterparties[1].account, payment.recipient);
    assert_eq!(disclosed.amounts.len(), 1);
    assert_eq!(disclosed.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosed.amounts[0].value, payment.amount);
    assert_eq!(payment.amount, held.disclosure.amounts.values()[0].amount.0);
    assert_eq!(disclosed.asset, payment.asset);
    assert_eq!(disclosed.expiry.payload_expires_at, payment.expires_at_ms);
    assert_eq!(payment.expires_at_ms, held.expiry.0 * 1000);
    assert_eq!(disclosed.idempotency_key, key());
    assert_eq!(payment.idempotency_key, key());
    assert_eq!(disclosed.fee_limit, held.disclosure.fee_limit.0);
    assert_eq!(
        disclosed.payload_sequence().expect("no payload sequence"),
        None
    );
    validate_call(call, &disclosed, sender).expect("consumer validates the original sender");
    assert!(validate_call(call, &disclosed, [0xac; 20]).is_err());

    for field in ["amount", "expiry", "asset", "counterparty", "key", "actor"] {
        let mut changed = disclosed.clone();
        match field {
            "amount" => changed.amounts[0].value += 1,
            "expiry" => changed.expiry.payload_expires_at += 1,
            "asset" => changed.asset[0] ^= 1,
            "counterparty" => changed.counterparties[0].account[0] ^= 1,
            "key" => changed.idempotency_key[0] ^= 1,
            _ => changed.actor[0] ^= 1,
        }
        assert!(reencode(&changed).is_err(), "altered {field} disclosure");
    }
    for mutation in [
        "amount_width",
        "expiry_width",
        "truncated",
        "trailing",
        "target",
        "selector",
        "zero_amount",
        "zero_key",
        "fee_overflow",
    ] {
        let mut changed = call.clone();
        match mutation {
            "amount_width" => changed.data[4 + 3 * 32] = 1,
            "expiry_width" => changed.data[4 + 4 * 32] = 1,
            "truncated" => {
                changed.data.pop();
            }
            "trailing" => changed.data.push(0),
            "target" => changed.to[19] ^= 1,
            "selector" => changed.data[0] ^= 1,
            "zero_amount" => changed.data[4 + 3 * 32..4 + 4 * 32].fill(0),
            "zero_key" => changed.data[4 + 5 * 32..].fill(0),
            _ => changed.max_fee_per_gas = u128::MAX,
        }
        assert!(bind_call(&changed, sender).is_err(), "malformed {mutation}");
    }
}
