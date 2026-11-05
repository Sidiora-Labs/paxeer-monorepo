//! Existing-grant disclosure approvals through the authenticated, served web route.

#[path = "support/web_hold.rs"]
mod web_hold;
use web_hold::*;

use layerx_agent_api::TimestampSeconds;
use layerx_crypto::payments::Payment;
use layerx_mcp::approval::ApprovalPolicy;
use layerx_mcp::boundary::{AgentSurface, ProgramReads};
use layerx_mcp::server::{Server, WebBoundary, WebRoute};
use layerx_mcp::stdio::{Bound, Session};
use layerx_mcp::tools::web::{GrantTerms, WebConfig};
use layerx_types::payload::ModuleId;

fn search_key() -> [u8; 32] {
    Sha256::digest(b"x-websearch-test/receive/metered-SID").into()
}

struct Case {
    served: Option<Session<WebBoundary<ProgramReads>>>,
    registry: Arc<ApprovalRegistry>,
    resources: WebApprovalResources,
    traffic: Arc<Mutex<Traffic>>,
    recording: Recording,
    enrolled: Enrolled,
}

impl Case {
    fn new() -> Self {
        let recording = Recording::start_search();
        let enrolled = Enrolled::new(
            &["write:web:search"],
            fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer DID"),
        );
        let resources = resources(&enrolled.directory.0.join("approvals"));
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&resources.store)));
        let traffic = Arc::new(Mutex::new(Traffic {
            standing_grant: true,
            ..Traffic::default()
        }));
        let served = Some(session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            resources.clone(),
            Arc::clone(&traffic),
        ));
        Self {
            served,
            registry,
            resources,
            traffic,
            recording,
            enrolled,
        }
    }

    fn invoke(&mut self) -> Value {
        let input = json!({"jsonrpc":"2.0", "id":1, "method":"tools/call", "params":{
            "name":"web.search", "arguments":{
                "query":"paxeer", "currency":"SID", "scheme":"metered", "idempotency_key":hex(&search_key())
            }
        }});
        let mut source = std::io::Cursor::new(format!("{input}\n"));
        let mut output = Vec::new();
        self.served
            .as_mut()
            .expect("running server")
            .serve(&mut source, &mut output)
            .expect("MCP line transport");
        serde_json::from_slice(&output).expect("MCP response")
    }

    fn hold(&mut self) -> Prepared {
        let response = self.invoke();
        assert_eq!(response["result"]["isError"], true, "{response}");
        let ticket = self
            .registry
            .ticket(search_key())
            .expect("ticket lookup")
            .expect("metered hold");
        assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
        assert_eq!(ticket.hold_id, search_key());
        assert!(
            response.to_string().contains(&hex(&ticket.hold_id)),
            "{response}"
        );
        assert_eq!(ticket.disclosure.amounts.values()[0].amount.0, 3114);
        assert_eq!(ticket.disclosure.asset.as_str(), "SID");
        let prepared = self
            .resources
            .preparation(&tenant(), search_key())
            .expect("retained preparation")
            .expect("held draw");
        assert_eq!(prepared.disclosure, ticket.disclosure);
        assert_eq!(prepared.disclosure.actor.as_str(), hex(&[0xab; 20]));
        assert_eq!(prepared.disclosure.fee_limit.0, 0);
        let bytes = prepared.unsigned_canonical_bytes.as_bytes();
        assert!(
            decode_unsigned(bytes).is_err(),
            "receiver draw must not be a payer transaction"
        );
        assert_eq!(
            prepared.disclosure.canonical_digest,
            <[u8; 32]>::from(Sha256::digest(bytes))
        );
        let ledger = fixture("gateway/metered.json");
        let grant_record = ledger["ledger"]["grants"]
            .as_array()
            .expect("grants")
            .iter()
            .find(|grant| grant["name"] == "SID")
            .expect("SID grant");
        let Payment::IssueGrant(grant) = Payment::decode(
            ModuleId::Asset,
            7,
            &unhex(grant_record["canonical"].as_str().expect("grant bytes")),
            b"",
        )
        .expect("real signed grant codec") else {
            panic!("issued grant")
        };
        let terms = GrantTerms {
            payer_did: fixture("gateway/buyer.json")["payerDid"]
                .as_str()
                .expect("payer")
                .to_owned(),
            payer_account: grant.from,
            recipient: grant.recipient,
            receiver_did_key: word(&ledger["ledger"]["receiverDidKey"]),
            asset: grant.asset,
            expires_at_ms: prepared.expiry.0 * 1000,
            amount: 3114,
            purpose_hash: grant.purpose_hash,
            idempotency_key: search_key(),
        };
        assert_eq!(
            bytes,
            terms
                .prepare_draw(grant.id, 19)
                .encode()
                .expect("canonical draw")
        );
        let commitment = unhex(
            prepared
                .disclosure
                .authority
                .as_str()
                .strip_prefix("x402:")
                .expect("spend commitment"),
        );
        let mut hash = Sha256::new();
        hash.update(b"layerx-web-receiver-draw-v1");
        hash.update(commitment);
        hash.update(bytes);
        assert_eq!(
            prepared.signing_preimage.as_bytes(),
            hash.finalize().as_slice()
        );
        assert!(self
            .resources
            .limiter
            .has_reservation(search_key())
            .expect("budget reservation"));
        self.assert_unpaid();
        prepared
    }

    fn assert_no_payer_transaction(&self) {
        let traffic = self.traffic.lock().expect("traffic");
        assert_eq!(traffic.preparations, 0);
        assert_eq!(traffic.grants, 0);
        assert_eq!(traffic.submits, 0);
        assert!(traffic.submitted.is_empty());
    }

    fn assert_unpaid(&self) {
        self.assert_no_payer_transaction();
        assert!(self
            .recording
            .signatures
            .lock()
            .expect("payment signatures")
            .is_empty());
    }

    fn approve(&self, prepared: &Prepared) {
        let queue = ApprovalSubmissionQueue::default();
        let decision = ApprovalService::new(
            &self.registry,
            &self.resources.limiter,
            &self.resources.expiry,
        )
        .approve(
            DecisionRequest {
                tenant: &tenant(),
                approval_id: search_key(),
                idempotency_key: &DecisionKey::new("approve-metered-draw").expect("decision key"),
                approver: ApproverId::new("operator").expect("approver"),
                current_sequence: HEIGHT,
                current_time: TimestampSeconds(HEIGHT),
            },
            prepared,
            &queue,
        )
        .expect("durable approval service");
        assert_eq!(decision.outcome, ApprovalOutcome::Granted);
        assert_eq!(
            queue.prepared(decision.submission_ref.expect("released reference")),
            Some(prepared.clone())
        );
        assert_eq!(
            self.registry
                .ticket(search_key())
                .expect("ticket")
                .expect("approved ticket")
                .state,
            ApprovalState::Approved
        );
    }

    fn restart(self) -> Self {
        let Self {
            served,
            registry,
            resources: original,
            traffic,
            recording,
            enrolled,
        } = self;
        drop(served);
        drop(registry);
        drop(original);
        let resources = resources(&enrolled.directory.0.join("approvals"));
        let registry = Arc::new(ApprovalRegistry::with_store(Arc::clone(&resources.store)));
        assert_eq!(
            registry
                .replay_tenant(&tenant(), &resources.limiter)
                .expect("replay held draw and reservation"),
            1
        );
        let served = Some(session(
            &enrolled,
            &recording,
            Arc::clone(&registry),
            resources.clone(),
            Arc::clone(&traffic),
        ));
        Self {
            served,
            registry,
            resources,
            traffic,
            recording,
            enrolled,
        }
    }
}

#[test]
fn above_threshold_standing_grant_search_is_held_on_its_disclosure() {
    let mut case = Case::new();
    case.hold();
}

#[test]
fn standing_grant_hold_survives_server_restart_and_is_pending() {
    let mut case = Case::new();
    let prepared = case.hold();
    case = case.restart();
    assert_eq!(
        case.resources
            .preparation(&tenant(), search_key())
            .expect("recovered journal"),
        Some(prepared)
    );
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert!(
        response.to_string().contains("awaiting approval"),
        "{response}"
    );
    assert!(
        response.to_string().contains(&hex(&search_key())),
        "{response}"
    );
    assert_eq!(
        case.registry
            .ticket(search_key())
            .expect("ticket")
            .expect("pending hold")
            .state,
        ApprovalState::AwaitingApproval
    );
    assert!(case
        .resources
        .limiter
        .has_reservation(search_key())
        .expect("recovered reservation"));
    case.assert_unpaid();
}

#[test]
fn approval_after_restart_settles_under_the_standing_grant_without_a_payer_transaction() {
    let mut case = Case::new();
    let prepared = case.hold();
    case = case.restart();
    case.approve(&prepared);
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], false, "{response}");
    assert_eq!(
        response["result"]["structuredContent"]["result"]["settlement"]["verificationLevel"],
        "sequencer-signed",
        "{response}"
    );
    case.assert_no_payer_transaction();
    let signatures = case
        .recording
        .signatures
        .lock()
        .expect("payment signatures");
    assert_eq!(signatures.len(), 1);
    assert_eq!(
        signatures[0]["payload"],
        fixture("client-exchange.json")["exchange"][1]["request"]["headers"]["PAYMENT-SIGNATURE"]
            ["payload"]
    );
}

#[test]
fn rejection_refuses_the_standing_grant_draw_without_payment() {
    let mut case = Case::new();
    case.hold();
    let decision = ApprovalService::new(
        &case.registry,
        &case.resources.limiter,
        &case.resources.expiry,
    )
    .reject(DecisionRequest {
        tenant: &tenant(),
        approval_id: search_key(),
        idempotency_key: &DecisionKey::new("reject-metered-draw").expect("decision key"),
        approver: ApproverId::new("operator").expect("approver"),
        current_sequence: HEIGHT,
        current_time: TimestampSeconds(HEIGHT),
    })
    .expect("durable rejection service");
    assert_eq!(decision.outcome, ApprovalOutcome::Rejected);
    assert!(!case
        .resources
        .limiter
        .has_reservation(search_key())
        .expect("released reservation"));
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert_eq!(
        case.registry
            .ticket(search_key())
            .expect("ticket")
            .expect("rejected hold")
            .state,
        ApprovalState::Rejected
    );
    assert!(response.to_string().contains("Rejected"), "{response}");
    case.assert_unpaid();
}

#[test]
fn a_grant_changed_below_the_draw_amount_is_refused_as_changed() {
    let mut case = Case::new();
    let prepared = case.hold();
    {
        let mut ledger = case.recording.ledger.lock().expect("ledger recording");
        let grants = ledger["ledger"]["grants"]
            .as_array()
            .expect("recorded grants");
        let original = grants
            .iter()
            .find(|grant| grant["name"] == "SID")
            .expect("SID grant");
        let request_data = format!(
            "0x423fe4b2{}",
            original["grantId"].as_str().expect("original grant ID")
        );
        let changed = grants
            .iter()
            .find(|grant| grant["name"] == "lowLimit")
            .expect("producer's low-limit grant");
        let Payment::IssueGrant(grant) = Payment::decode(
            ModuleId::Asset,
            7,
            &unhex(changed["canonical"].as_str().expect("changed grant bytes")),
            b"",
        )
        .expect("authentic changed grant") else {
            panic!("issued grant")
        };
        assert_eq!(grant.per_draw_maximum, 3113);
        assert!(grant.per_draw_maximum < prepared.disclosure.amounts.values()[0].amount.0);
        let changed_result = changed["result"].clone();
        let observation = ledger["endpoints"]["gateway"]["*"]
            .as_array_mut()
            .expect("RPC observations")
            .iter_mut()
            .find(|record| {
                record["method"] == "eth_call" && record["params"][0]["data"] == request_data
            })
            .expect("original grant observation");
        observation["result"] = changed_result;
    }
    case.approve(&prepared);
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert_eq!(
        response["result"]["structuredContent"]["refusal"],
        "the daemon response is unusable: the web spend differs from the disclosure under approval",
        "{response}"
    );
    case.assert_unpaid();
}

#[test]
fn standing_grant_payer_without_approval_resources_is_unavailable() {
    let mut case = Case::new();
    case.traffic.lock().expect("traffic").approval_unavailable = true;
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert!(
        response
            .to_string()
            .contains("the web payer is unavailable"),
        "{response}"
    );
    assert!(case.registry.hold_ids().expect("hold IDs").is_empty());
    assert!(case
        .resources
        .preparation(&tenant(), search_key())
        .expect("journal")
        .is_none());
    case.assert_unpaid();
}

fn session(
    enrolled: &Enrolled,
    recording: &Recording,
    registry: Arc<ApprovalRegistry>,
    resources: WebApprovalResources,
    traffic: Arc<Mutex<Traffic>>,
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
    .expect("receiver width");
    let config = WebConfig::new(
        &recording.endpoint(),
        buyer["payerDid"].as_str().expect("payer DID"),
        &format!("0x{}", hex(&[0xab; 20])),
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
        nonce: 7,
        sender: [0xab; 20],
        corrupt_hash: false,
        node: SequencerEndpoint::new(&recording.endpoint(), &[]).expect("real node client"),
        receipt: layerx_sdk::rpc::RpcClient::connect_node(&recording.endpoint(), None)
            .expect("real receipt client"),
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
        .expect("served web route");
    Session::new(Bound::Full(Box::new(server)), boundary)
}
