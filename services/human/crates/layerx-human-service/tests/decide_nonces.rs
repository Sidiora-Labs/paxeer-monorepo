#[path = "support/repair_22_33_44_auth.rs"]
mod auth_fixture;
use auth_fixture::{auth_config, register_session, required, SoftwareAuthenticator};

#[path = "support/repair_22_33_44_approval.rs"]
mod durable_approval;

use layerx_human_test_support as support;

use std::fs;
use std::time::Duration;

use layerx_agent_api::prepare::Prepared;
use layerx_agent_api::track::{SubmissionRef, SubmissionState, TrackedSubmission};
use layerx_agent_api::TimestampSeconds;
use layerx_agentd::approval::{
    ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey, DecisionRequest,
};
use layerx_agentd::policy::approval::{ApprovalState, ApproverId};
use layerx_agentd::prepare::evm::{
    prepare_bound, EvmPreparationState, PrecompileCall, PreparationDefaults, PrepareError,
    PrepareRequest, Prepared as EvmPrepared,
};
use layerx_agentd::store::TenantId;
use layerx_human_service::approvals::{
    AgentApprovalRecord, AgentApprovalState, AgentDecision, AgentDecisionBoundary,
    AgentDecisionResolution, AgentDecisionStatus, ApprovalBoundary, ApprovalBoundaryError,
    DecisionError, Decisions, VerifiedBudgetAfter,
};
use layerx_human_service::audit::{ApprovalOutcome as AuditOutcome, AuditChain, AuditEvent};
use layerx_human_service::auth::{OperationDigest, Passkeys};
use layerx_human_service::trace::TraceId;
use layerx_intents::{compile, Intent, IntentKind, LxpSend};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount as IntentAmount;
use layerx_types::clock::Clock as _;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, NetworkId, ProtocolVersion, PublicKey, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds as IntentTimestampSeconds,
};
use layerx_types::payload::{
    ActivityType as PayloadActivityType, ModuleId, ModuleRegistration, ModuleRegistry,
};
use layerx_types::rung::Rung;

fn tenant() -> TenantId {
    TenantId::new("tenant-decisions").expect("tenant")
}

fn ledger_payload() -> Vec<u8> {
    let signer = layerx_crypto::local::LocalSigner::new([0x51; 32]);
    let public_key = layerx_crypto::signer::Signer::public_key(&signer);
    let from = AccountId::parse("agent:did:layerx:decision-agent:main").expect("source account");
    let to = AccountId::parse("agent:did:layerx:recipient:main").expect("destination account");
    let protocol = layerx_intents::canonical::PROTOCOL_VERSION;
    let debit = layerx_crypto::send::SendDebit {
        from: layerx_intents::canonical::account_id_for_protocol(&from, protocol)
            .expect("canonical source account"),
        to: layerx_intents::canonical::account_id_for_protocol(&to, protocol)
            .expect("canonical destination account"),
        asset: [0x33; 32],
        amount: 1,
        source_sequence: 7,
        idempotency_key: [4; 32],
        expires_at: durable_approval::activity_expiry(),
        context_hash: [0x55; 32],
        conditions: Vec::new(),
        authorization_kind: SendAuthorizationKind::Owner as u8,
        network_id: 77,
        protocol_version: protocol,
    };
    let send = LxpSend::new(
        from,
        to,
        AssetId::new(debit.asset),
        IntentAmount::from_u128(debit.amount),
        Sequence::from_u64(debit.source_sequence),
        IdempotencyKey::new(debit.idempotency_key),
        IntentTimestampSeconds::from_u64(debit.expires_at),
        ContextHash::new(debit.context_hash),
        support::sign_send(
            &signer,
            &debit,
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new(public_key),
                AuthorizationSignature::new([0x77; 64]),
            ),
        ),
        NetworkId::new(debit.network_id).expect("network"),
        ProtocolVersion::new(protocol).expect("protocol"),
    )
    .expect("send intent");
    let activity = PayloadActivityType::new(ModuleId::Asset, 5).expect("send activity");
    let registration =
        ModuleRegistration::new(ModuleId::Asset, &[activity]).expect("asset registration");
    let registry = ModuleRegistry::new(&[registration]).expect("module registry");
    let compiled =
        compile(&Intent::v1(IntentKind::LxpSend(send)), &registry).expect("compiled send intent");
    compiled.payload().as_bytes().to_vec()
}

fn pending_nonce(prepared_nonce: u64, pending: u64) -> Result<EvmPrepared, PrepareError> {
    prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: pending,
            base_fee_per_gas: 10,
            head_height: 88,
        },
        PreparationDefaults {
            max_priority_fee_per_gas: 2,
            max_fee_per_gas: 100,
            maximum_payload_bytes: 1024,
        },
        PrepareRequest {
            from: [0x11; 20],
            call: PrecompileCall::ledger_transfer(ledger_payload()),
            expected_nonce: Some(prepared_nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1024,
        },
        100_000,
    )
}

fn preparation(evm: &EvmPrepared) -> Prepared {
    durable_approval::preparation(evm)
}

struct AgentdDecisions<'a> {
    clock: std::sync::Arc<layerx_client::runtime_clock::RuntimeClock>,
    service: ApprovalService<'a>,
    queue: &'a ApprovalSubmissionQueue,
    held: Prepared,
    evm: EvmPrepared,
    stale: Option<PrepareError>,
}

impl AgentdDecisions<'_> {
    fn decision_time(&self) -> Result<TimestampSeconds, ApprovalBoundaryError> {
        let reading = self
            .clock
            .sample(Duration::from_secs(1))
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        if reading.generation == [0; 16] {
            return Err(ApprovalBoundaryError::Unavailable);
        }
        Ok(TimestampSeconds(reading.unix_seconds()))
    }

    fn normalize(
        decision: layerx_agentd::approval::ApprovalDecision,
    ) -> Result<AgentDecision, ApprovalBoundaryError> {
        let status = match decision.outcome {
            ApprovalOutcome::Granted => AgentDecisionStatus::Approved {
                submission_ref: decision.submission_ref,
            },
            ApprovalOutcome::Rejected => AgentDecisionStatus::Rejected,
            ApprovalOutcome::Expired => AgentDecisionStatus::Expired,
            ApprovalOutcome::Defective => AgentDecisionStatus::Defective,
            ApprovalOutcome::AlreadyDecided | ApprovalOutcome::Conflict => {
                return Err(ApprovalBoundaryError::VerificationFailed)
            }
        };
        Ok(AgentDecision {
            status,
            resolution: AgentDecisionResolution::Applied,
        })
    }
}

impl ApprovalBoundary for AgentdDecisions<'_> {
    fn approval(
        &mut self,
        approval_id: [u8; 32],
        at_sequence: u64,
    ) -> Result<AgentApprovalRecord, ApprovalBoundaryError> {
        let record = self
            .service
            .get(&tenant(), approval_id, at_sequence, self.decision_time()?)
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        let state = match record.state {
            ApprovalState::AwaitingApproval => AgentApprovalState::AwaitingApproval,
            ApprovalState::Approved => AgentApprovalState::Approved {
                submission_ref: record
                    .submission_ref
                    .ok_or(ApprovalBoundaryError::Corrupt)?,
            },
            ApprovalState::Rejected => AgentApprovalState::Rejected,
            ApprovalState::Expired => AgentApprovalState::Expired,
            ApprovalState::Defective => AgentApprovalState::Defective,
        };
        Ok(AgentApprovalRecord {
            approval_id,
            held_activity: record.held_activity,
            canonical_bytes_digest: record.canonical_bytes_digest,
            hold_reason_code: record.hold_reason.code.to_owned(),
            hold_reason: record.hold_reason.message.to_owned(),
            created_at_sequence: record.created_at_sequence,
            expires_at_sequence: record.expires_at_sequence,
            state,
        })
    }

    fn verified_budget_after(
        &mut self,
        hold: &AgentApprovalRecord,
        at_sequence: u64,
    ) -> Result<VerifiedBudgetAfter, ApprovalBoundaryError> {
        self.service
            .get(
                &tenant(),
                hold.approval_id,
                at_sequence,
                self.decision_time()?,
            )
            .map_err(|_| ApprovalBoundaryError::NotFound)?;
        Ok(VerifiedBudgetAfter {
            remaining: 975,
            rung: Rung::Committed,
            evidence_digest: [9; 32],
            observed_at_sequence: at_sequence,
        })
    }

    fn track_released(
        &mut self,
        submission_ref: [u8; 32],
    ) -> Result<TrackedSubmission, ApprovalBoundaryError> {
        let released = self
            .queue
            .prepared(submission_ref)
            .ok_or(ApprovalBoundaryError::NotFound)?;
        if released != self.held {
            return Err(ApprovalBoundaryError::VerificationFailed);
        }
        let reference: String = submission_ref
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();
        Ok(TrackedSubmission {
            submission_ref: SubmissionRef::new(reference)
                .map_err(|_| ApprovalBoundaryError::Corrupt)?,
            state: SubmissionState::Queued,
            evidence: Vec::new(),
            rung: Rung::Submitted,
            transitions: Vec::new(),
        })
    }
}

impl AgentDecisionBoundary for AgentdDecisions<'_> {
    fn approve(
        &mut self,
        approval_id: [u8; 32],
        held_digest: [u8; 32],
        idempotency_key: &str,
        pending: u64,
    ) -> Result<AgentDecision, ApprovalBoundaryError> {
        if held_digest != self.held.disclosure.canonical_digest {
            return Err(ApprovalBoundaryError::VerificationFailed);
        }
        let current = pending_nonce(self.evm.call.nonce, pending).map_err(|error| {
            self.stale = Some(error);
            ApprovalBoundaryError::VerificationFailed
        })?;
        let decision = self
            .service
            .approve(
                DecisionRequest {
                    tenant: &tenant(),
                    approval_id,
                    idempotency_key: &DecisionKey::new(idempotency_key)
                        .map_err(|_| ApprovalBoundaryError::Corrupt)?,
                    approver: ApproverId::new("human:mara")
                        .map_err(|_| ApprovalBoundaryError::Corrupt)?,
                    current_sequence: pending,
                    current_time: self.decision_time()?,
                },
                &preparation(&current),
                self.queue,
            )
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        Self::normalize(decision)
    }

    fn reject(
        &mut self,
        approval_id: [u8; 32],
        held_digest: [u8; 32],
        idempotency_key: &str,
        current_sequence: u64,
    ) -> Result<AgentDecision, ApprovalBoundaryError> {
        if held_digest != self.held.disclosure.canonical_digest {
            return Err(ApprovalBoundaryError::VerificationFailed);
        }
        let decision = self
            .service
            .reject(DecisionRequest {
                tenant: &tenant(),
                approval_id,
                idempotency_key: &DecisionKey::new(idempotency_key)
                    .map_err(|_| ApprovalBoundaryError::Corrupt)?,
                approver: ApproverId::new("human:mara")
                    .map_err(|_| ApprovalBoundaryError::Corrupt)?,
                current_sequence,
                current_time: self.decision_time()?,
            })
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        Self::normalize(decision)
    }
}

fn decide_at(pending: u64) {
    let directory = support::directory(&format!("decide-nonce-{pending}"));
    let queue = ApprovalSubmissionQueue::default();
    let evm = pending_nonce(7, 7).expect("prepared EVM call");
    assert_eq!(evm.call.nonce, 7);
    assert_eq!(evm.observed_head_height, 88);
    let held = preparation(&evm);
    let clock = durable_approval::clock();
    let (registry, limiter, expiry) = durable_approval::register_and_reopen(
        &directory.join("registry"),
        &tenant(),
        &evm,
        1,
        1,
        190,
        clock.as_ref(),
    );
    let mut boundary = AgentdDecisions {
        clock: clock.clone(),
        service: ApprovalService::new(&registry, &limiter, &expiry),
        queue: &queue,
        held: held.clone(),
        evm: evm.clone(),
        stale: None,
    };
    let map = support::tenancy(&[("mara", "tenant-decisions")]);
    let (mut store, _) = support::install_and_open(
        &directory.join("human"),
        &map,
        support::retention_uniform(10_000),
    );
    let mut scope = store
        .principal(&support::principal("mara"))
        .expect("principal scope");
    let passkeys = required(Passkeys::new(auth_config()), "passkeys");
    let mut authenticator = SoftwareAuthenticator::new();
    let session = register_session(&passkeys, &mut scope, &mut authenticator, 100);
    let challenge = required(
        passkeys.begin_step_up(
            &mut scope,
            session.access_token().expose(),
            session.csrf_token().expose(),
            OperationDigest::new(held.disclosure.canonical_digest),
            110,
        ),
        "begin step-up",
    );
    let assertion = authenticator.assert(&challenge.ceremony);
    let evidence = required(
        passkeys.finish_step_up(&mut scope, &challenge.challenge_id, &assertion, 111),
        "finish step-up",
    );
    let mut audit = AuditChain::open(&scope).expect("audit");
    let result = Decisions::approve(
        &mut scope,
        &passkeys,
        session.access_token().expose(),
        session.csrf_token().expose(),
        &evidence,
        &mut boundary,
        [1; 32],
        "approve-pending-nonce",
        pending,
        112,
        &mut audit,
        &TraceId::mint([1; 16]),
    );
    if pending == 7 {
        let approved = result.expect("approval at the pending nonce");
        let AgentDecisionStatus::Approved {
            submission_ref: Some(reference),
        } = approved.status()
        else {
            panic!("hold must release, got {:?}", approved.status());
        };
        assert_eq!(approved.resolution(), AgentDecisionResolution::Applied);
        assert_eq!(
            approved.tracking().map(|value| &value.state),
            Some(&SubmissionState::Queued)
        );
        assert_eq!(queue.len(), 1);
        let released = queue.prepared(reference).expect("released preparation");
        assert_eq!(released, held);
        let transaction = layerx_crypto::evm_transaction::decode_unsigned(
            released.unsigned_canonical_bytes.as_bytes(),
        )
        .expect("released EVM call");
        assert_eq!(transaction.nonce, pending);
        assert_eq!(transaction.chain_id, evm.call.chain_id);
        assert_eq!(transaction.data, evm.call.data);
        assert_eq!(released.signing_preimage.as_bytes(), evm.signing_hash);
        assert_eq!(boundary.stale, None);
        assert_eq!(
            boundary
                .service
                .get(
                    &tenant(),
                    [1; 32],
                    pending,
                    boundary.decision_time().expect("decision time"),
                )
                .expect("approved hold")
                .state,
            ApprovalState::Approved
        );
        let entries = audit.entries(&scope).expect("audit entries");
        assert_eq!(entries.len(), 1);
        assert!(
            matches!(entries[0].event(), AuditEvent::ApprovalDecision { hold_digest, outcome: AuditOutcome::Approved, .. } if *hold_digest == held.disclosure.canonical_digest)
        );
    } else {
        assert!(matches!(
            result,
            Err(DecisionError::Agent(
                ApprovalBoundaryError::VerificationFailed
            ))
        ));
        assert_eq!(
            boundary.stale,
            Some(PrepareError::StaleNonce {
                expected: 7,
                pending
            })
        );
        assert!(queue.is_empty());
        assert_eq!(
            boundary
                .service
                .get(
                    &tenant(),
                    [1; 32],
                    pending,
                    boundary.decision_time().expect("decision time"),
                )
                .expect("unreleased hold")
                .state,
            ApprovalState::AwaitingApproval
        );
        assert!(audit.entries(&scope).expect("audit entries").is_empty());
    }
    drop(scope);
    drop(store);
    fs::remove_dir_all(directory).expect("cleanup");
}

#[test]
fn approval_releases_a_hold_prepared_at_the_pending_nonce() {
    decide_at(7);
}

#[test]
fn approval_of_a_stale_nonce_hold_is_refused() {
    decide_at(8);
}
