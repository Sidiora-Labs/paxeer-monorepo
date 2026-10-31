use std::time::Duration;

use layerx_agent_api::idempotency::{BodyDigest, IdempotentMutation, Key};
use layerx_agent_api::identity::{
    AgentDid, AuthorityRef, ClientId, ExplicitSet, PolicyVersion, SessionContext, SessionOpen,
    TenantId,
};
use layerx_agent_api::prepare::{CanonicalBytes, IdempotencyRef};
use layerx_agent_api::read::{AccountRef, BalanceValue, Freshness, RelativeTo, VerifiedRead};
use layerx_agent_api::track::{EvidenceRef, SubmissionRef, TrackedSubmission};
use layerx_agent_api::{Amount, ContractVersion, SubmissionState, TimestampSeconds};
use layerx_client::client::{ClientConfig, ReconnectPolicy};
use layerx_client::lni::handshake::HandshakeConfig;
use layerx_client::lni::schema::Version;
use layerx_client::lni::transport::Limits;
use layerx_sdk::approval::{
    ApprovalApproveRequest, ApprovalContractError, ApprovalDecisionOutcome, ApprovalEventKind,
    ApprovalGetRequest, ApprovalId, ApprovalListRequest, ApprovalRejectRequest, ApprovalState,
    DecisionKey, CONTRACT_INTRODUCED, ENFORCEMENT_NOTICE,
};
use layerx_sdk::{Client, Deployment, Operation, SdkError, SubmissionOutcome, GUARANTEES};
use layerx_types::result::ResultCode;
use layerx_types::rung::Rung;

fn direct_config() -> ClientConfig {
    ClientConfig {
        endpoint: "/tmp/layerx-node.sock".into(),
        handshake: HandshakeConfig {
            built_interface_version: Version::V1_0,
            expected_protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
            expected_network_id: 7,
        },
        limits: Limits {
            maximum_frame_bytes: 1_048_576,
            maximum_connections: 1,
            maximum_streams: 16,
            maximum_queued_bytes: 2_097_152,
            deadline: Duration::from_secs(5),
        },
        reconnect: ReconnectPolicy {
            maximum_attempts: 3,
            base_delay: Duration::from_millis(1),
            maximum_delay: Duration::from_millis(5),
            jitter_percent: 10,
        },
    }
}

fn context() -> SessionContext {
    SessionContext::new(
        TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}")),
        AgentDid::new("did:layerx:agent-a").unwrap_or_else(|error| panic!("agent: {error:?}")),
        AuthorityRef::new("authority-a").unwrap_or_else(|error| panic!("authority: {error:?}")),
        ExplicitSet::allow(vec![]),
        TimestampSeconds(100),
        ClientId::new("sdk").unwrap_or_else(|error| panic!("client: {error:?}")),
        PolicyVersion::new("policy-v1").unwrap_or_else(|error| panic!("policy: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("context: {error:?}"))
}

fn mutation<T>(operation: T) -> IdempotentMutation<T> {
    IdempotentMutation {
        request_id: layerx_agent_api::error::RequestId(7),
        key: Key::new([9; 32]).unwrap_or_else(|error| panic!("key: {error:?}")),
        body_digest: BodyDigest([8; 32]),
        operation,
    }
}

fn freshness() -> Freshness {
    Freshness {
        chain_head: 20,
        value_height: 17,
        relative_to: RelativeTo::Block {
            height: 19,
            hash: [0x19; 32],
        },
    }
}

#[test]
fn daemon_and_direct_node_shapes_publish_identical_guarantees_and_calls() {
    let daemon = Client::daemon(
        "/run/layerx/agentd.sock",
        ContractVersion { major: 2, minor: 2 },
    )
    .unwrap_or_else(|error| panic!("daemon: {error:?}"));
    let direct =
        Client::direct_node(direct_config()).unwrap_or_else(|error| panic!("direct: {error:?}"));
    assert_eq!(daemon.deployment(), Deployment::Daemon);
    assert_eq!(direct.deployment(), Deployment::DirectNode);
    assert_eq!(daemon.guarantees(), GUARANTEES);
    assert_eq!(daemon.guarantees(), direct.guarantees());

    let request = mutation(SessionOpen {
        context: context(),
        grant_id: [1; 32],
        registration_payload: Vec::new(),
        expiry_sequence: 100,
        purpose: [2; 32],
        fee_limit: Amount(1),
    });
    let daemon_call = daemon.session_open(request.clone());
    let direct_call = direct.session_open(request);
    assert_eq!(daemon_call.operation(), Operation::SessionOpen);
    assert_eq!(daemon_call.operation(), direct_call.operation());
    assert_eq!(daemon_call.request(), direct_call.request());
    assert_eq!(daemon_call.request().key.bytes(), [9; 32]);
    assert!(daemon_call.operation().mutating());
}

#[test]
fn operation_catalogue_covers_the_complete_contract_surface() {
    assert_eq!(Operation::ALL.len(), 53);
    let names: std::collections::BTreeSet<_> = Operation::ALL
        .iter()
        .map(|operation| operation.name())
        .collect();
    for required in [
        "agent.register",
        "approval.approve",
        "approval.get",
        "approval.list",
        "approval.reject",
        "availability.fetch",
        "budget.create",
        "budget.fund",
        "budget.list",
        "budget.reconciliation",
        "budget.revoke",
        "budget.state",
        "capability.attenuate",
        "capability.create",
        "capability.list",
        "capability.revoke",
        "export.offline",
        "mcp.invoke",
        "policy.dry_run",
        "prepare",
        "program.activity",
        "program.call",
        "program.deploy",
        "program.discover",
        "program.interface",
        "program.receipt",
        "program.simulate",
        "program.upgrade",
        "program.wind-down",
        "project",
        "read.account",
        "read.balance",
        "read.batch",
        "read.checkpoint",
        "read.history",
        "read.module_state",
        "read.proof_bundle",
        "session.close",
        "session.list",
        "session.open",
        "session.refresh",
        "sign",
        "submit",
        "subscription.acknowledge",
        "subscription.create",
        "subscription.delete",
        "subscription.health",
        "subscription.list",
        "subscription.pause",
        "subscription.resume",
        "tenant.readiness",
        "track",
        "wait",
    ] {
        assert!(names.contains(required), "missing SDK operation {required}");
    }
    assert!(Operation::ALL
        .iter()
        .filter(|operation| operation.mutating())
        .all(|operation| !matches!(operation, Operation::Track | Operation::Wait)));
    assert!(Operation::ProgramCall.mutating());
    assert!(Operation::ProgramDeploy.mutating());
    assert!(Operation::ProgramUpgrade.mutating());
    assert!(Operation::ProgramWindDown.mutating());
    assert!(!Operation::ProgramDiscover.mutating());
    assert!(!Operation::ProgramInterface.mutating());
    assert!(!Operation::ProgramSimulate.mutating());
    assert!(!Operation::ProgramReceipt.mutating());
    assert!(!Operation::ProgramActivity.mutating());
}

#[test]
fn approval_module_matches_contract_1_1_operations_events_and_outcomes() {
    assert_eq!(CONTRACT_INTRODUCED, (1, 1));
    assert!(ENFORCEMENT_NOTICE.contains("confers no protocol authority"));
    assert_eq!(
        ApprovalState::ALL
            .iter()
            .map(|value| value.name())
            .collect::<Vec<_>>(),
        ["Held", "Granted", "Rejected", "Expired", "Defective"]
    );
    assert_eq!(
        ApprovalDecisionOutcome::ALL
            .iter()
            .map(|value| value.name())
            .collect::<Vec<_>>(),
        [
            "Granted",
            "Rejected",
            "Expired",
            "Defective",
            "AlreadyDecided",
            "Conflict",
        ]
    );
    assert_eq!(
        ApprovalEventKind::ALL
            .iter()
            .map(|value| value.name())
            .collect::<Vec<_>>(),
        ["Created", "Granted", "Rejected", "Expired", "Defective"]
    );

    let tenant = TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}"));
    let approval_id = ApprovalId::new([7; 32]);
    let key = DecisionKey::new("decision-7").unwrap_or_else(|error| panic!("key: {error:?}"));
    let client = Client::daemon(
        "/run/layerx/agentd.sock",
        ContractVersion { major: 2, minor: 1 },
    )
    .unwrap_or_else(|error| panic!("daemon: {error:?}"));
    assert_eq!(
        client
            .approval_list(
                ApprovalListRequest::new(tenant.clone(), None, 50)
                    .unwrap_or_else(|error| panic!("list: {error:?}")),
            )
            .operation(),
        Operation::ApprovalList
    );
    assert_eq!(
        client
            .approval_get(ApprovalGetRequest {
                tenant: tenant.clone(),
                approval_id,
            })
            .operation(),
        Operation::ApprovalGet
    );
    assert_eq!(
        client
            .approval_approve(ApprovalApproveRequest {
                tenant: tenant.clone(),
                approval_id,
                idempotency_key: key.clone(),
            })
            .operation(),
        Operation::ApprovalApprove
    );
    assert_eq!(
        client
            .approval_reject(
                ApprovalRejectRequest::new(tenant, approval_id, key, "not expected")
                    .unwrap_or_else(|error| panic!("reject: {error:?}")),
            )
            .operation(),
        Operation::ApprovalReject
    );
    assert!(Operation::ApprovalApprove.mutating());
    assert!(Operation::ApprovalReject.mutating());
    assert_eq!(
        ApprovalListRequest::new(
            TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error:?}")),
            None,
            0,
        ),
        Err(ApprovalContractError::InvalidPageLimit)
    );
}

#[test]
fn reads_never_present_unverified_or_below_requested_values() {
    let balance = BalanceValue {
        account: AccountRef::new("account-a").unwrap_or_else(|error| panic!("account: {error:?}")),
        asset: layerx_agent_api::identity::Asset::new("LXP")
            .unwrap_or_else(|error| panic!("asset: {error:?}")),
        amount: Amount(9_007_199_254_740_993),
        height: 17,
        canonical_state: CanonicalBytes::new(b"core-state".to_vec())
            .unwrap_or_else(|error| panic!("state: {error:?}")),
    };
    let unverified = VerifiedRead::new(balance.clone(), Rung::Submitted, freshness());
    assert_eq!(
        Client::accept_verified_read(Rung::Receipted, unverified),
        Err(SdkError::UnverifiedRead)
    );
    let below = VerifiedRead::new(balance.clone(), Rung::Receipted, freshness());
    assert_eq!(
        Client::accept_verified_read(Rung::Committed, below),
        Err(SdkError::VerificationBelowRequested {
            requested: Rung::Committed,
            achieved: Rung::Receipted,
        })
    );
    let proven = VerifiedRead::new(balance, Rung::Committed, freshness());
    assert!(Client::accept_verified_read(Rung::Committed, proven).is_ok());
}

fn tracked(state: SubmissionState, evidence: Vec<EvidenceRef>, rung: Rung) -> TrackedSubmission {
    TrackedSubmission {
        submission_ref: SubmissionRef::new("submission-a")
            .unwrap_or_else(|error| panic!("submission: {error:?}")),
        state,
        evidence,
        rung,
        transitions: Vec::new(),
    }
}

#[test]
fn unknown_and_future_protocol_codes_remain_lossless() {
    let unknown = Client::submission_outcome(tracked(
        SubmissionState::Unknown,
        Vec::new(),
        Rung::Submitted,
    ));
    assert!(matches!(unknown, Ok(SubmissionOutcome::Unknown(_))));

    let future = ResultCode::from_raw(-77_777);
    let failed = Client::submission_outcome(tracked(
        SubmissionState::Failed { result: future },
        Vec::new(),
        Rung::Submitted,
    ));
    assert!(matches!(
        failed,
        Ok(SubmissionOutcome::Failed { result, .. }) if result.raw() == -77_777
    ));

    let executed_without_proof = Client::submission_outcome(tracked(
        SubmissionState::Executed {
            receipt_ref: layerx_agent_api::track::ReceiptRef::new("receipt-a")
                .unwrap_or_else(|error| panic!("receipt: {error:?}")),
        },
        Vec::new(),
        Rung::Submitted,
    ));
    assert_eq!(
        executed_without_proof,
        Err(SdkError::ExecutedWithoutEvidence)
    );

    let executed = |rung| {
        Client::submission_outcome(tracked(
            SubmissionState::Executed {
                receipt_ref: layerx_agent_api::track::ReceiptRef::new("receipt-a")
                    .unwrap_or_else(|error| panic!("receipt: {error:?}")),
            },
            vec![EvidenceRef {
                kind: "receipt".into(),
                digest: [0x22; 32],
            }],
            rung,
        ))
    };
    assert_eq!(
        executed(Rung::Submitted),
        Err(SdkError::ExecutedWithoutEvidence)
    );
    assert!(matches!(
        executed(Rung::Receipted),
        Ok(SubmissionOutcome::Executed(submission)) if submission.rung == Rung::Receipted
    ));
}

#[test]
fn signer_surface_contains_no_key_export() {
    let source = include_str!("../src/lib.rs");
    let signer_start = source
        .find("pub mod signer")
        .unwrap_or_else(|| panic!("signer module missing"));
    let signer_end = source[signer_start..]
        .find("pub enum Deployment")
        .map_or_else(
            || panic!("signer module end missing"),
            |offset| signer_start + offset,
        );
    let signer_source = &source[signer_start..signer_end];
    assert!(!signer_source.contains("private_key"));
    assert!(!signer_source.contains("secret"));
    assert!(signer_source.contains("trait ExternalSigner"));

    let key = IdempotencyRef::new("caller-owned-key")
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    assert_eq!(key.as_str(), "caller-owned-key");
}

#[test]
fn sdk_crate_has_no_settlement_domain_or_activity_codec() {
    use layerx_sdk::rpc::{wallet_account, RpcClient, RpcError};
    use layerx_sdk::wallet::Payment;
    use layerx_types::payload::ModuleId;

    for source in [
        include_str!("../src/lib.rs"),
        include_str!("../src/production.rs"),
        include_str!("../src/wallet.rs"),
    ] {
        for retired in [
            "SettlementDomain",
            "GuarantorKey",
            "layerx_proof::checkpoint",
            "layerx_wire::activity",
            "verify_checkpoint",
            "decode_signed",
        ] {
            assert!(!source.contains(retired), "SDK still uses {retired}");
        }
    }

    let native = wallet_account("did:layerx:alice", [0; 32], [0; 32])
        .unwrap_or_else(|error| panic!("native wallet account: {error:?}"));
    let token = wallet_account("did:layerx:alice", [0x11; 32], [0; 32])
        .unwrap_or_else(|error| panic!("token wallet account: {error:?}"));
    assert_eq!(
        native
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>(),
        "575498fd80da9b17115311af107ab11639acf474a69f944c9c9b1d0ea28ed205"
    );
    assert_eq!(
        token
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>(),
        "c25ada37deae26ed54923dab01b56e9dded08b4ca710b0fd0b1d172f44404324"
    );
    assert_ne!(native, token);
    let payment = Payment::OpenAccount { asset: [0x11; 32] };
    let payload = payment
        .encode(b"did:layerx:alice")
        .unwrap_or_else(|error| panic!("wallet payment encode: {error:?}"));
    let decoded = Payment::decode(ModuleId::Asset, 4, &payload, b"did:layerx:alice")
        .unwrap_or_else(|error| panic!("wallet payment decode: {error:?}"));
    assert!(matches!(decoded, Payment::OpenAccount { asset } if asset == [0x11; 32]));
    let rpc = RpcClient::connect("http://127.0.0.1:1", None)
        .unwrap_or_else(|error| panic!("wallet client: {error:?}"));
    assert!(matches!(
        rpc.wallet([0; 32]).balance("did::alice", [0x11; 32]),
        Err(RpcError::InvalidRequest)
    ));
}

#[test]
fn production_verifies_mirror_receipts_from_committed_sources() {
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_sdk::production::{
        verify_mirror_receipt_from_sources, MirrorReadPolicy, MirrorSdkError, MirrorSources,
        MirrorVerification,
    };
    use layerx_wire::receipt::Receipt;

    let _: fn(
        &MirrorSources,
        u64,
        &MirrorReadPolicy,
        usize,
        &[SequencerAuthorization],
    ) -> Result<MirrorVerification<Receipt>, MirrorSdkError> = verify_mirror_receipt_from_sources;

    let source = include_str!("../src/production.rs");
    for retired in [
        "SignedHeaderTrust",
        "from_source(",
        "verify_mirror_state_from_sources",
    ] {
        assert!(!source.contains(retired), "SDK still uses {retired}");
    }
}
