//! Existing-grant disclosure approvals through the authenticated, served web route.

use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::web_hold;
use web_hold::*;

use layerx_agentd::approval::ApprovalSubmissionQueue;

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
            option_assertion::required(
                fixture("gateway/buyer.json")["payerDid"].as_str(),
                "payer DID",
            ),
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
        result_assertion::required(
            option_assertion::required(self.served.as_mut(), "running server")
                .serve(&mut source, &mut output),
            "MCP line transport",
        );
        result_assertion::required(serde_json::from_slice(&output), "MCP response")
    }

    fn hold(&mut self) -> Prepared {
        let response = self.invoke();
        assert_eq!(response["result"]["isError"], true, "{response}");
        let ticket = option_assertion::required(
            result_assertion::required(self.registry.ticket(search_key()), "ticket lookup"),
            "metered hold",
        );
        assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
        assert_eq!(ticket.hold_id, search_key());
        assert!(
            response.to_string().contains(&hex(&ticket.hold_id)),
            "{response}"
        );
        assert_eq!(ticket.disclosure.amounts.values()[0].amount.0, 3114);
        assert_eq!(ticket.disclosure.asset.as_str(), "SID");
        let prepared = option_assertion::required(
            result_assertion::required(
                self.resources.preparation(&tenant(), search_key()),
                "retained preparation",
            ),
            "held draw",
        );
        assert_eq!(prepared.disclosure, ticket.disclosure);
        assert_eq!(
            prepared.disclosure.actor.as_str(),
            hex(&[0xab; 20]),
            "held disclosure actor must retain the sender address"
        );
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
        let grant_record = option_assertion::required(
            option_assertion::required(ledger["ledger"]["grants"].as_array(), "grants")
                .iter()
                .find(|grant| grant["name"] == "SID"),
            "SID grant",
        );
        let Payment::IssueGrant(grant) = result_assertion::required(
            Payment::decode(
                ModuleId::Asset,
                7,
                &unhex(option_assertion::required(
                    grant_record["canonical"].as_str(),
                    "grant bytes",
                )),
                b"",
            ),
            "real signed grant codec",
        ) else {
            panic!("issued grant")
        };
        let terms = GrantTerms {
            payer_did: option_assertion::required(
                fixture("gateway/buyer.json")["payerDid"].as_str(),
                "payer",
            )
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
            result_assertion::required(terms.prepare_draw(grant.id, 19).encode(), "canonical draw")
        );
        let commitment = unhex(option_assertion::required(
            prepared.disclosure.authority.as_str().strip_prefix("x402:"),
            "spend commitment",
        ));
        let mut hash = Sha256::new();
        hash.update(b"layerx-web-receiver-draw-v1");
        hash.update(commitment);
        hash.update(bytes);
        assert_eq!(
            prepared.signing_preimage.as_bytes(),
            hash.finalize().as_slice()
        );
        assert!(result_assertion::required(
            self.resources.limiter.has_reservation(search_key()),
            "budget reservation"
        ));
        self.assert_unpaid();
        prepared
    }

    fn assert_no_payer_transaction(&self) {
        let traffic = result_assertion::required(self.traffic.lock(), "traffic");
        assert_eq!(traffic.preparations, 0);
        assert_eq!(traffic.grants, 0);
        assert_eq!(traffic.submits, 0);
        assert!(traffic.submitted.is_empty());
    }

    fn assert_unpaid(&self) {
        self.assert_no_payer_transaction();
        assert!(
            result_assertion::required(self.recording.signatures.lock(), "payment signatures")
                .is_empty()
        );
    }

    fn approve(&self, prepared: &Prepared) {
        let queue = ApprovalSubmissionQueue::default();
        let decision = result_assertion::required(
            ApprovalService::new(
                &self.registry,
                &self.resources.limiter,
                &self.resources.expiry,
            )
            .approve(
                DecisionRequest {
                    tenant: &tenant(),
                    approval_id: search_key(),
                    idempotency_key: &result_assertion::required(
                        DecisionKey::new("approve-metered-draw"),
                        "decision key",
                    ),
                    approver: result_assertion::required(ApproverId::new("operator"), "approver"),
                    current_sequence: HEIGHT,
                    current_time: TimestampSeconds(HEIGHT),
                },
                prepared,
                &queue,
            ),
            "durable approval service",
        );
        assert_eq!(decision.outcome, ApprovalOutcome::Granted);
        assert_eq!(
            queue.prepared(option_assertion::required(
                decision.submission_ref,
                "released reference"
            )),
            Some(prepared.clone())
        );
        assert_eq!(
            option_assertion::required(
                result_assertion::required(self.registry.ticket(search_key()), "ticket"),
                "approved ticket"
            )
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
            result_assertion::required(
                registry.replay_tenant(&tenant(), &resources.limiter),
                "replay held draw and reservation"
            ),
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
        result_assertion::required(
            case.resources.preparation(&tenant(), search_key()),
            "recovered journal"
        ),
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
        option_assertion::required(
            result_assertion::required(case.registry.ticket(search_key()), "ticket"),
            "pending hold"
        )
        .state,
        ApprovalState::AwaitingApproval
    );
    assert!(result_assertion::required(
        case.resources.limiter.has_reservation(search_key()),
        "recovered reservation"
    ));
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
    let signatures =
        result_assertion::required(case.recording.signatures.lock(), "payment signatures");
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
    let decision = result_assertion::required(
        ApprovalService::new(
            &case.registry,
            &case.resources.limiter,
            &case.resources.expiry,
        )
        .reject(DecisionRequest {
            tenant: &tenant(),
            approval_id: search_key(),
            idempotency_key: &result_assertion::required(
                DecisionKey::new("reject-metered-draw"),
                "decision key",
            ),
            approver: result_assertion::required(ApproverId::new("operator"), "approver"),
            current_sequence: HEIGHT,
            current_time: TimestampSeconds(HEIGHT),
        }),
        "durable rejection service",
    );
    assert_eq!(decision.outcome, ApprovalOutcome::Rejected);
    assert!(!result_assertion::required(
        case.resources.limiter.has_reservation(search_key()),
        "released reservation"
    ));
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert_eq!(
        option_assertion::required(
            result_assertion::required(case.registry.ticket(search_key()), "ticket"),
            "rejected hold"
        )
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
        let mut ledger =
            result_assertion::required(case.recording.ledger.lock(), "ledger recording");
        let grants =
            option_assertion::required(ledger["ledger"]["grants"].as_array(), "recorded grants");
        let original = option_assertion::required(
            grants.iter().find(|grant| grant["name"] == "SID"),
            "SID grant",
        );
        let request_data = format!(
            "0x423fe4b2{}",
            option_assertion::required(original["grantId"].as_str(), "original grant ID")
        );
        let changed = option_assertion::required(
            grants.iter().find(|grant| grant["name"] == "lowLimit"),
            "producer's low-limit grant",
        );
        let Payment::IssueGrant(grant) = result_assertion::required(
            Payment::decode(
                ModuleId::Asset,
                7,
                &unhex(option_assertion::required(
                    changed["canonical"].as_str(),
                    "changed grant bytes",
                )),
                b"",
            ),
            "authentic changed grant",
        ) else {
            panic!("issued grant")
        };
        assert_eq!(grant.per_draw_maximum, 3113);
        assert!(grant.per_draw_maximum < prepared.disclosure.amounts.values()[0].amount.0);
        let changed_result = changed["result"].clone();
        let observation = option_assertion::required(
            option_assertion::required(
                ledger["endpoints"]["gateway"]["*"].as_array_mut(),
                "RPC observations",
            )
            .iter_mut()
            .find(|record| {
                record["method"] == "eth_call" && record["params"][0]["data"] == request_data
            }),
            "original grant observation",
        );
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
    result_assertion::required(case.traffic.lock(), "traffic").approval_unavailable = true;
    let response = case.invoke();
    assert_eq!(response["result"]["isError"], true, "{response}");
    assert!(
        response
            .to_string()
            .contains("the web payer is unavailable"),
        "{response}"
    );
    assert!(result_assertion::required(case.registry.hold_ids(), "hold IDs").is_empty());
    assert!(result_assertion::required(
        case.resources.preparation(&tenant(), search_key()),
        "journal"
    )
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
        "authenticated MCP server",
    );
    let buyer = fixture("gateway/buyer.json");
    let receiver = result_assertion::required(
        unhex(option_assertion::required(
            fixture("gateway/metered.json")["ledger"]["receiverAddress"].as_str(),
            "receiver",
        ))
        .try_into(),
        "receiver width",
    );
    let config = result_assertion::required(
        WebConfig::new(
            &recording.endpoint(),
            option_assertion::required(buyer["payerDid"].as_str(), "payer DID"),
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
        "served web route",
    );
    Session::new(Bound::Full(Box::new(server)), boundary)
}
