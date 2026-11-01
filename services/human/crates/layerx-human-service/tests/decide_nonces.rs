use layerx_human_test_support as support;

use std::fmt::Display;
use std::fs;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, Disclosure, IdempotencyRef, PreparationRef, Prepared, SigningPreimage,
};
use layerx_agent_api::track::{SubmissionRef, SubmissionState, TrackedSubmission};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::{
    ApprovalExpiry, ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey,
    DecisionRequest,
};
use layerx_agentd::budget::BudgetLimiter;
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    hold, ApprovalContext, ApprovalRegistry, ApprovalState, ApproverId,
};
use layerx_agentd::prepare::evm::{
    prepare_bound, EvmPreparationState, PrecompileCall, PreparationDefaults, PrepareError,
    PrepareRequest, Prepared as EvmPrepared,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_human_service::approvals::{
    AgentApprovalRecord, AgentApprovalState, AgentDecision, AgentDecisionBoundary,
    AgentDecisionResolution, AgentDecisionStatus, ApprovalBoundary, ApprovalBoundaryError,
    DecisionError, Decisions, VerifiedBudgetAfter,
};
use layerx_human_service::audit::{ApprovalOutcome as AuditOutcome, AuditChain, AuditEvent};
use layerx_human_service::auth::{
    AccountIdentity, AuthConfig, Device, OperationDigest, Passkeys, RateLimit, SessionGrant,
};
use layerx_human_service::custody::EvmTransaction;
use layerx_human_service::store::PrincipalScope;
use layerx_human_service::trace::TraceId;
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const RP_ID: &str = "paxportwallet.com";
const ORIGIN: &str = "https://paxportwallet.com";
const FLAG_UP: u8 = 1;
const FLAG_UV: u8 = 1 << 2;
const FLAG_AT: u8 = 1 << 6;

fn required<T, E: Display>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error}"))
}

fn auth_config() -> AuthConfig {
    AuthConfig {
        rp_id: RP_ID.to_owned(),
        rp_name: "LayerX".to_owned(),
        origin: ORIGIN.to_owned(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 30,
        session_ttl_secs: 60,
        refresh_ttl_secs: 300,
        step_up_ttl_secs: 10,
        rate_limit: RateLimit {
            attempts: 100,
            window_secs: 60,
        },
    }
}

fn decode_ceremony(value: &str) -> Value {
    let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
    required(serde_json::from_slice(&bytes), "parse ceremony")
}

fn required_text<'a>(value: &'a Value, pointer: &str) -> &'a str {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
}

fn encode_response(value: &Value) -> String {
    URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
}

struct SoftwareAuthenticator {
    signing_key: SigningKey,
    credential_id: Vec<u8>,
    counter: u32,
    user_handle: Option<String>,
}

impl SoftwareAuthenticator {
    fn new() -> Self {
        let mut seed = [0_u8; 32];
        required(getrandom::fill(&mut seed), "authenticator entropy");
        let mut credential_id = vec![0_u8; 32];
        required(
            getrandom::fill(&mut credential_id),
            "credential identifier entropy",
        );
        Self {
            signing_key: SigningKey::from_bytes(&seed),
            credential_id,
            counter: 0,
            user_handle: None,
        }
    }

    fn register(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.user_handle = Some(required_text(&options, "/user/id").to_owned());
        let client_data = client_data("webauthn.create", challenge);
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "transports": ["internal"],
            "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
        }))
    }

    fn assert(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge);
        let client_hash = Sha256::digest(&client_data);
        let mut signed = Vec::with_capacity(authenticator_data.len() + client_hash.len());
        signed.extend_from_slice(&authenticator_data);
        signed.extend_from_slice(&client_hash);
        let signature = self.signing_key.sign(&signed).to_bytes();
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "authenticatorData": URL_SAFE_NO_PAD.encode(authenticator_data),
            "signature": URL_SAFE_NO_PAD.encode(signature),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
            "userHandle": self.user_handle,
        }))
    }

    fn attestation_object(&self) -> Vec<u8> {
        let map = CborValue::Map(vec![
            (
                CborValue::Text("fmt".to_owned()),
                CborValue::Text("none".to_owned()),
            ),
            (
                CborValue::Text("attStmt".to_owned()),
                CborValue::Map(Vec::new()),
            ),
            (
                CborValue::Text("authData".to_owned()),
                CborValue::Bytes(self.authenticator_data(0, true)),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode attestation",
        );
        bytes
    }

    fn authenticator_data(&self, counter: u32, attested: bool) -> Vec<u8> {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&Sha256::digest(RP_ID.as_bytes()));
        bytes.push(FLAG_UP | FLAG_UV | if attested { FLAG_AT } else { 0 });
        bytes.extend_from_slice(&counter.to_be_bytes());
        if attested {
            bytes.extend_from_slice(&[0_u8; 16]);
            let credential_length = u16::try_from(self.credential_id.len())
                .unwrap_or_else(|_| panic!("credential identifier too long"));
            bytes.extend_from_slice(&credential_length.to_be_bytes());
            bytes.extend_from_slice(&self.credential_id);
            bytes.extend_from_slice(&self.cose_public_key());
        }
        bytes
    }

    fn cose_public_key(&self) -> Vec<u8> {
        let public_key = self.signing_key.verifying_key().to_bytes().to_vec();
        let map = CborValue::Map(vec![
            (CborValue::Integer(1.into()), CborValue::Integer(1.into())),
            (
                CborValue::Integer(3.into()),
                CborValue::Integer((-8).into()),
            ),
            (
                CborValue::Integer((-1).into()),
                CborValue::Integer(6.into()),
            ),
            (
                CborValue::Integer((-2).into()),
                CborValue::Bytes(public_key),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode public key",
        );
        bytes
    }
}

fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": ORIGIN,
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

fn register_session(
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    authenticator: &mut SoftwareAuthenticator,
    now: u64,
) -> SessionGrant {
    let identity = required(
        AccountIdentity::new("mara@example.com", "Mara"),
        "account identity",
    );
    let registration = required(
        passkeys.begin_registration(scope, &identity, "Phone passkey", now),
        "begin registration",
    );
    let response = authenticator.register(&registration.ceremony);
    required(
        passkeys.finish_registration(scope, &registration.registration_id, &response, now + 1),
        "finish registration",
    );
    open_session(
        passkeys,
        scope,
        authenticator,
        "dev_aabbccddeeff00112233445566778891",
        now + 2,
    )
}

fn open_session(
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    authenticator: &mut SoftwareAuthenticator,
    device_id: &str,
    now: u64,
) -> SessionGrant {
    let challenge = required(passkeys.begin_assertion(scope, now), "begin assertion");
    let response = authenticator.assert(&challenge.ceremony);
    required(
        passkeys.finish_assertion(scope, &challenge.assertion_id, &response, now + 1),
        "finish assertion",
    );
    let device = required(Device::new(device_id, "Phone", "mobile"), "device");
    required(
        passkeys.open_session(scope, &challenge.assertion_id, device, now + 2),
        "open session",
    )
}

fn tenant() -> TenantId {
    TenantId::new("tenant-decisions").expect("tenant")
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
            call: PrecompileCall::ledger_transfer(vec![0x33; 32]),
            expected_nonce: Some(prepared_nonce),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1024,
        },
        100_000,
    )
}

fn preparation(evm: &EvmPrepared) -> Prepared {
    let call = &evm.call;
    let transaction = EvmTransaction {
        chain_id: call.chain_id,
        nonce: call.nonce,
        max_priority_fee_per_gas: u64::try_from(call.max_priority_fee_per_gas)
            .expect("priority fee"),
        max_fee_per_gas: u64::try_from(call.max_fee_per_gas).expect("fee cap"),
        gas_limit: call.gas_limit,
        to: call.to,
        value: [0; 32],
        calldata: call.data.clone(),
    };
    let bytes = serde_json::to_vec(&transaction).expect("canonical transaction");
    Prepared {
        preparation_ref: PreparationRef::new("pending-nonce-hold").expect("preparation reference"),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&bytes).into(),
            activity_type: ActivityType(5),
            actor: AgentDid::new("did:layerx:decision-agent").expect("actor"),
            authority: AuthorityRef::new("session-key").expect("authority"),
            counterparties: ExplicitSet::deny_all(),
            amounts: ExplicitSet::deny_all(),
            asset: Asset::new("LXP").expect("asset"),
            fee_limit: Amount(2),
            expiry: TimestampSeconds(200),
            idempotency_key: IdempotencyRef::new("nonce-hold").expect("activity key"),
        },
        unsigned_canonical_bytes: CanonicalBytes::new(bytes).expect("transaction bytes"),
        signing_preimage: SigningPreimage::new(evm.signing_hash.to_vec())
            .expect("EVM signing hash"),
        expiry: TimestampSeconds(200),
    }
}

struct AgentdDecisions<'a> {
    service: ApprovalService<'a>,
    queue: &'a ApprovalSubmissionQueue,
    held: Prepared,
    evm: EvmPrepared,
    stale: Option<PrepareError>,
}

impl AgentdDecisions<'_> {
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
            .get(&tenant(), approval_id, at_sequence)
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
            .get(&tenant(), hold.approval_id, at_sequence)
            .map_err(|_| ApprovalBoundaryError::NotFound)?;
        Err(ApprovalBoundaryError::VerificationFailed)
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
            })
            .map_err(|_| ApprovalBoundaryError::Unavailable)?;
        Self::normalize(decision)
    }
}

fn decide_at(pending: u64) {
    let directory = support::directory(&format!("decide-nonce-{pending}"));
    let registry = ApprovalRegistry::default();
    let limiter = BudgetLimiter::new(Vec::new()).expect("limiter");
    let expiry = ApprovalExpiry::open(&directory.join("agent")).expect("approval expiry");
    let queue = ApprovalSubmissionQueue::default();
    let evm = pending_nonce(7, 7).expect("prepared EVM call");
    assert_eq!(evm.call.nonce, 7);
    assert_eq!(evm.observed_head_height, 88);
    let held = preparation(&evm);
    hold(
        &registry,
        ApprovalContext {
            tenant: tenant(),
            agent: Did::new(b"did:layerx:decision-agent").expect("agent"),
            session: SessionId([2; 32]),
            capability: CapabilityId([3; 32]),
            policy_version: "policy-decisions-v1".to_owned(),
            request_id: [1; 32],
        },
        held.clone(),
        1,
        190,
    )
    .expect("hold");
    let mut boundary = AgentdDecisions {
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
        let transaction: EvmTransaction =
            serde_json::from_slice(released.unsigned_canonical_bytes.as_bytes())
                .expect("released EVM call");
        assert_eq!(transaction.nonce, pending);
        assert_eq!(transaction.chain_id, evm.call.chain_id);
        assert_eq!(transaction.calldata, evm.call.data);
        assert_eq!(released.signing_preimage.as_bytes(), evm.signing_hash);
        assert_eq!(boundary.stale, None);
        assert_eq!(
            boundary
                .service
                .get(&tenant(), [1; 32], pending)
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
                .get(&tenant(), [1; 32], pending)
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
