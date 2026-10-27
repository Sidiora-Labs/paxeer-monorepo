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
        ContractVersion { major: 1, minor: 2 },
    )
    .unwrap_or_else(|error| panic!("daemon: {error:?}"));
    let direct =
        Client::direct_node(direct_config()).unwrap_or_else(|error| panic!("direct: {error:?}"));
    assert_eq!(daemon.deployment(), Deployment::Daemon);
    assert_eq!(direct.deployment(), Deployment::DirectNode);
    assert_eq!(daemon.guarantees(), GUARANTEES);
    assert_eq!(daemon.guarantees(), direct.guarantees());

    let request = mutation(SessionOpen(context()));
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
    assert_eq!(Operation::ALL.len(), 50);
    let names: std::collections::BTreeSet<_> = Operation::ALL
        .iter()
        .map(|operation| operation.name())
        .collect();
    for required in [
        "agent.register",
        "session.open",
        "capability.create",
        "budget.create",
        "project",
        "prepare",
        "sign",
        "submit",
        "track",
        "read.balance",
        "read.proof_bundle",
        "availability.fetch",
        "faucet.claim",
        "subscription.create",
        "approval.list",
        "approval.get",
        "approval.approve",
        "approval.reject",
        "program.discover",
        "program.interface",
        "program.simulate",
        "program.call",
        "program.deploy",
        "program.upgrade",
        "program.wind-down",
        "program.receipt",
        "program.activity",
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
        ContractVersion { major: 1, minor: 1 },
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
fn signer_surface_contains_no_key_export_and_offline_verifier_is_real() {
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

    let keys = [];
    assert_eq!(
        layerx_sdk::production::verify_receipt(
            &[],
            layerx_sdk::production::ReceiptKeys::HeightRanges(&keys),
            [1; 32],
            [2; 32],
            [3; 32],
        )
        .map_err(|failure| failure.check),
        Err(layerx_sdk::ReceiptFailureCode::Decode)
    );

    let key = IdempotencyRef::new("caller-owned-key")
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    assert_eq!(key.as_str(), "caller-owned-key");
}

#[test]
fn sdk_crate_has_no_settlement_domain_or_activity_codec() {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_proof::inclusion::SequencerAuthorization;
    use layerx_sdk::production::{verify_receipt, ReceiptKeys};
    use layerx_sdk::rpc::{wallet_account, RpcClient, RpcError};
    use layerx_sdk::wallet::Payment;
    use layerx_sdk::ReceiptFailureCode;
    use layerx_types::payload::ModuleId;
    use layerx_wire::hash::receipt_digest;
    use layerx_wire::receipt::{decode, encode_unsigned};

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

    let fixture: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/receipt-positive-v2.json"
    ))
    .unwrap_or_else(|error| panic!("receipt fixture: {error}"));
    let hex = fixture["canonical_receipt_hex"]
        .as_str()
        .unwrap_or_else(|| panic!("canonical receipt missing"));
    assert!(hex.len().is_multiple_of(2));
    let mut canonical: Vec<u8> = (0..hex.len() / 2)
        .map(|index| {
            u8::from_str_radix(&hex[index * 2..index * 2 + 2], 16)
                .unwrap_or_else(|error| panic!("receipt hex: {error}"))
        })
        .collect();
    let receipt = decode(&canonical).unwrap_or_else(|error| panic!("receipt decode: {error:?}"));
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt missing"));
    let height = 42_u64;
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&height.to_be_bytes());
    let offsets: Vec<_> = canonical
        .windows(32)
        .enumerate()
        .filter_map(|(offset, bytes)| (bytes == protocol.batch_id()).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1);
    canonical[offsets[0]..offsets[0] + 32].copy_from_slice(&batch_id);
    let receipt =
        decode(&canonical).unwrap_or_else(|error| panic!("height receipt decode: {error:?}"));
    let unsigned =
        encode_unsigned(&receipt).unwrap_or_else(|error| panic!("receipt preimage: {error:?}"));
    let digest =
        receipt_digest(&unsigned).unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    let signer = SigningKey::from_bytes(&[0x33; 32]);
    let signature_offset = canonical.len() - 64;
    canonical[signature_offset..].copy_from_slice(&signer.sign(&digest).to_bytes());
    let keys = [SequencerAuthorization::new(
        protocol.context_hash(),
        signer.verifying_key().to_bytes(),
        height,
        height,
    )];
    let verify = |bytes: &[u8], keys: &[SequencerAuthorization]| {
        verify_receipt(
            bytes,
            ReceiptKeys::HeightRanges(keys),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
        )
    };
    let verified = verify(&canonical, &keys)
        .unwrap_or_else(|error| panic!("public receipt verification: {error:?}"));
    assert_eq!(verified.canonical_bytes(), canonical);
    assert_eq!(verified.rung(), Rung::Receipted);
    assert_eq!(verified.evidence().receipt_digest(), Some(digest));
    assert_eq!(
        verify(&canonical, &[]).map_err(|failure| failure.check),
        Err(ReceiptFailureCode::SequencerSignature)
    );
    let expired = [SequencerAuthorization::new(
        protocol.context_hash(),
        signer.verifying_key().to_bytes(),
        height + 1,
        height + 1,
    )];
    assert_eq!(
        verify(&canonical, &expired).map_err(|failure| failure.check),
        Err(ReceiptFailureCode::SequencerSignature)
    );
    canonical[signature_offset] ^= 1;
    assert_eq!(
        verify(&canonical, &keys).map_err(|failure| failure.check),
        Err(ReceiptFailureCode::SequencerSignature)
    );

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
