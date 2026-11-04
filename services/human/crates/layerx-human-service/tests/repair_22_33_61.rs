use std::fmt::{Debug, Display};
use std::fs;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::track::{EvidenceRef, SubmissionRef, SubmissionState, TrackedSubmission};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::{
    ApprovalDecision, ApprovalEventKind, ApprovalEvents, ApprovalExpiry, ApprovalLifecycle,
    ApprovalOutcome, ApprovalService, ApprovalSubmissionQueue, DecisionKey, DecisionRequest,
};
use layerx_agentd::audit::{Coverage, Log};
use layerx_agentd::budget::{
    BudgetLimiter, CoreTimestampMs, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::events::EventIngestor;
use layerx_agentd::policy::approval::{
    reserve_and_hold_at, ApprovalContext, ApprovalRegistry, ApprovalSequence, ApprovalState,
    ApproverId,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store as AgentStore, TenantId};
use layerx_crypto::disclosure::call::{bind_call, ledger_selector};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_human_service::approvals::{
    AgentApprovalRecord, AgentApprovalState, AgentDecision, AgentDecisionBoundary,
    AgentDecisionResolution, AgentDecisionStatus, ApprovalActivityClass, ApprovalBoundary,
    ApprovalBoundaryError, ApprovalEvent, ApprovalNotification, ApprovalPresentation,
    BudgetVerification, DecisionError, Decisions, DisclosureRenderError, DisclosureRenderer, Inbox,
    InboxError, InboxState, RenderedCounterparty, VerifiedBudgetAfter,
};
use layerx_human_service::audit::AuditChain;
use layerx_human_service::auth::{
    AccountIdentity, AuthConfig, Device, OperationDigest, Passkeys, RateLimit, SessionGrant,
};
use layerx_human_service::notify::Dispatcher;
use layerx_human_service::store::{PrincipalScope, Table};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support as support;
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

const SEQUENCE: u64 = 10;
const HOLD_EXPIRY: u64 = 50;
const NOW: u64 = 100;
const LIMIT: LimitId = LimitId([1; 16]);

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("approval repair path: {error:?}"))
}

fn hex(bytes: impl AsRef<[u8]>) -> String {
    bytes
        .as_ref()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

fn tenant() -> TenantId {
    checked(TenantId::new("tenant-submitted-budget"))
}

fn prepared() -> (Prepared, layerx_crypto::disclosure::Disclosure) {
    let mut recipient = [0; 32];
    recipient[12..].fill(5);
    let mut amount = [0; 32];
    amount[31] = 25;
    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 10,
        to: [0x15; 20],
        data: [
            ledger_selector("transfer(address,uint256)").to_vec(),
            recipient.to_vec(),
            amount.to_vec(),
        ]
        .concat(),
    };
    let signed = checked(call.sign(&[0x68; 32]));
    let (call, signature) = checked(decode_signed(&signed));
    let sender = checked(recover_sender(&call, &signature));
    let disclosure = checked(bind_call(&call, sender));
    let bytes = call.encode_unsigned();
    let recipient = checked(AgentDid::new(hex(recipient)));
    let value = Prepared {
        preparation_ref: checked(PreparationRef::new("submitted-budget-preparation")),
        unsigned_canonical_bytes: checked(CanonicalBytes::new(bytes.clone())),
        signing_preimage: checked(SigningPreimage::new(call.signing_hash().to_vec())),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&bytes).into(),
            activity_type: ActivityType(5),
            actor: checked(AgentDid::new(hex(&disclosure.actor))),
            authority: checked(AuthorityRef::new("session-key")),
            counterparties: ExplicitSet::allow(vec![recipient.clone()]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty: recipient,
                amount: Amount(disclosure.amounts[0].value),
            }]),
            asset: checked(Asset::new(hex(disclosure.asset))),
            fee_limit: Amount(disclosure.fee_limit),
            expiry: TimestampSeconds(200),
            idempotency_key: checked(IdempotencyRef::new(hex([1; 32]))),
        },
        expiry: TimestampSeconds(200),
    };
    (value, disclosure)
}

fn hex_to_address(value: &str) -> Result<[u8; 20], std::num::ParseIntError> {
    let mut address = [0; 20];
    for (index, byte) in address.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&value[index * 2..index * 2 + 2], 16)?;
    }
    Ok(address)
}

fn context(prepared: &Prepared) -> ApprovalContext {
    ApprovalContext {
        tenant: tenant(),
        agent: checked(Did::new(prepared.disclosure.actor.as_str().as_bytes())),
        actor_address: checked(hex_to_address(prepared.disclosure.actor.as_str())),
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "submitted-budget-policy".to_owned(),
        request_id: [1; 32],
    }
}

// In-process peer: production preparation, approval registry, budget limiter and event codec.
// Its rung is the recorded read strength; no approval consumer is substituted.
struct AgentPeer<'a> {
    registry: &'a ApprovalRegistry,
    limiter: &'a BudgetLimiter,
    service: ApprovalService<'a>,
    queue: &'a ApprovalSubmissionQueue,
    prepared: &'a Prepared,
    rung: Rung,
    observed_sequence: u64,
    evidence_present: bool,
}

impl ApprovalBoundary for AgentPeer<'_> {
    fn approval(
        &mut self,
        id: [u8; 32],
        _: u64,
    ) -> Result<AgentApprovalRecord, ApprovalBoundaryError> {
        let ticket = self
            .registry
            .ticket(id)
            .map_err(|_| ApprovalBoundaryError::Unavailable)?
            .ok_or(ApprovalBoundaryError::NotFound)?;
        let state = match ticket.state {
            ApprovalState::AwaitingApproval => AgentApprovalState::AwaitingApproval,
            ApprovalState::Rejected => AgentApprovalState::Rejected,
            ApprovalState::Expired => AgentApprovalState::Expired,
            ApprovalState::Defective => AgentApprovalState::Defective,
            ApprovalState::Approved => AgentApprovalState::Approved {
                submission_ref: self
                    .registry
                    .audit_entry(id)
                    .map_err(|_| ApprovalBoundaryError::Unavailable)?
                    .and_then(|audit| audit.resulting_activity_id)
                    .ok_or(ApprovalBoundaryError::Corrupt)?,
            },
        };
        Ok(AgentApprovalRecord {
            approval_id: ticket.hold_id,
            held_activity: ticket.disclosure,
            canonical_bytes_digest: ticket.disclosure_digest,
            hold_reason_code: "policy_approval_required".to_owned(),
            hold_reason: "Policy requires human approval before submission".to_owned(),
            created_at_sequence: SEQUENCE,
            expires_at_sequence: ticket.expires_at_sequence,
            state,
        })
    }

    fn verified_budget_after(
        &mut self,
        hold: &AgentApprovalRecord,
        sequence: u64,
    ) -> Result<VerifiedBudgetAfter, ApprovalBoundaryError> {
        let amount =
            hold.held_activity.amounts.values()[0].amount.0 + hold.held_activity.fee_limit.0;
        let remaining = self
            .limiter
            .remaining_after_reservation(
                hold.approval_id,
                amount,
                sequence,
                CoreTimestampMs(NOW * 1_000),
            )
            .map_err(|_| ApprovalBoundaryError::VerificationFailed)?;
        let mut evidence = Sha256::new();
        evidence.update(hold.approval_id);
        evidence.update(hold.canonical_bytes_digest);
        evidence.update(remaining.to_be_bytes());
        evidence.update(sequence.to_be_bytes());
        evidence.update([self.rung.wire_rank()]);
        Ok(VerifiedBudgetAfter {
            remaining,
            rung: self.rung,
            evidence_digest: if self.evidence_present {
                evidence.finalize().into()
            } else {
                [0; 32]
            },
            observed_at_sequence: self.observed_sequence,
        })
    }

    fn track_released(
        &mut self,
        reference: [u8; 32],
    ) -> Result<TrackedSubmission, ApprovalBoundaryError> {
        self.queue
            .prepared(reference)
            .ok_or(ApprovalBoundaryError::NotFound)?;
        Ok(TrackedSubmission {
            submission_ref: SubmissionRef::new(hex(reference))
                .map_err(|_| ApprovalBoundaryError::Corrupt)?,
            state: SubmissionState::Queued,
            evidence: vec![EvidenceRef {
                kind: "approval-release".to_owned(),
                digest: reference,
            }],
            rung: Rung::Submitted,
            transitions: Vec::new(),
        })
    }
}

fn normalize(decision: ApprovalDecision) -> Result<AgentDecision, ApprovalBoundaryError> {
    let status = match decision.winning_outcome.unwrap_or(decision.outcome) {
        ApprovalOutcome::Granted => AgentDecisionStatus::Approved {
            submission_ref: decision.submission_ref,
        },
        ApprovalOutcome::Rejected => AgentDecisionStatus::Rejected,
        ApprovalOutcome::Expired => AgentDecisionStatus::Expired,
        ApprovalOutcome::Defective => AgentDecisionStatus::Defective,
        ApprovalOutcome::AlreadyDecided | ApprovalOutcome::Conflict => {
            return Err(ApprovalBoundaryError::Corrupt)
        }
    };
    Ok(AgentDecision {
        status,
        resolution: if decision.outcome == ApprovalOutcome::AlreadyDecided {
            AgentDecisionResolution::AlreadyDecided
        } else {
            AgentDecisionResolution::Applied
        },
    })
}

impl AgentDecisionBoundary for AgentPeer<'_> {
    fn approve(
        &mut self,
        id: [u8; 32],
        digest: [u8; 32],
        key: &str,
        sequence: u64,
    ) -> Result<AgentDecision, ApprovalBoundaryError> {
        if self.prepared.disclosure.canonical_digest != digest {
            return Err(ApprovalBoundaryError::VerificationFailed);
        }
        let key = DecisionKey::new(key).map_err(|_| ApprovalBoundaryError::Corrupt)?;
        normalize(
            self.service
                .approve(
                    DecisionRequest {
                        tenant: &tenant(),
                        approval_id: id,
                        idempotency_key: &key,
                        approver: ApproverId::new("human:mara")
                            .map_err(|_| ApprovalBoundaryError::Corrupt)?,
                        current_sequence: sequence,
                    },
                    self.prepared,
                    self.queue,
                )
                .map_err(|_| ApprovalBoundaryError::Unavailable)?,
        )
    }

    fn reject(
        &mut self,
        id: [u8; 32],
        digest: [u8; 32],
        key: &str,
        sequence: u64,
    ) -> Result<AgentDecision, ApprovalBoundaryError> {
        if self.prepared.disclosure.canonical_digest != digest {
            return Err(ApprovalBoundaryError::VerificationFailed);
        }
        let key = DecisionKey::new(key).map_err(|_| ApprovalBoundaryError::Corrupt)?;
        normalize(
            self.service
                .reject(DecisionRequest {
                    tenant: &tenant(),
                    approval_id: id,
                    idempotency_key: &key,
                    approver: ApproverId::new("human:mara")
                        .map_err(|_| ApprovalBoundaryError::Corrupt)?,
                    current_sequence: sequence,
                })
                .map_err(|_| ApprovalBoundaryError::Unavailable)?,
        )
    }
}

fn produce_event(root: &std::path::Path, prepared: &Prepared) -> ApprovalEvent {
    let context = context(prepared);
    let mut ingestor = checked(EventIngestor::open(
        checked(AgentStore::open(root)),
        tenant(),
        16,
        SEQUENCE,
    ));
    let mut audit = checked(Log::open(root, &tenant()));
    let emission = checked(ApprovalEvents::emit(
        &mut ingestor,
        &mut audit,
        &mut Coverage::default(),
        &ApprovalLifecycle {
            tenant: context.tenant,
            agent: context.agent,
            session: context.session,
            capability: context.capability.0,
            policy_version: context.policy_version,
            approval_id: context.request_id,
            canonical_digest: prepared.disclosure.canonical_digest,
            activity_type: prepared.disclosure.activity_type.0,
            asset: prepared.disclosure.asset.as_str().to_owned(),
            kind: ApprovalEventKind::Created,
            principal: None,
            observed_at_ms: NOW * 1_000,
        },
    ));
    let event = checked(ApprovalEvent::decode_agent_stream(
        &emission.canonical_event_bytes,
        NOW,
    ));
    assert_eq!(event.sequence, SEQUENCE);
    assert_eq!(event.approval_id, [1; 32]);
    assert_eq!(
        event.disclosure_digest,
        prepared.disclosure.canonical_digest
    );
    event
}

fn exercise_path(rung: Rung) {
    let root = support::directory("repair-22-33-61");
    checked(fs::create_dir_all(&root));
    let registry = ApprovalRegistry::default();
    let limiter = checked(BudgetLimiter::new(vec![LimitConfig {
        id: LIMIT,
        name: "approval-budget".to_owned(),
        scope: LimitScope::Tenant([2; 32]),
        ceiling: 1_000,
        consumed: 0,
    }]));
    let expiry = checked(ApprovalExpiry::open(root.join("expiry")));
    let queue = ApprovalSubmissionQueue::default();
    let (prepared, disclosure) = prepared();
    let ticket = checked(reserve_and_hold_at(
        &registry,
        &limiter,
        context(&prepared),
        prepared.clone(),
        &ReservationRequest {
            id: [1; 32],
            amount: 55,
            expiry_sequence: HOLD_EXPIRY,
            current_sequence: SEQUENCE,
            applicable_limits: vec![LIMIT],
        },
        ApprovalSequence(HOLD_EXPIRY),
        TimestampSeconds(NOW),
    ));
    assert_eq!(ticket.disclosure, prepared.disclosure);
    assert_eq!(checked(limiter.held_reservations()), 1);
    let event = produce_event(&root.join("stream"), &prepared);
    let mut peer = AgentPeer {
        registry: &registry,
        limiter: &limiter,
        service: ApprovalService::new(&registry, &limiter, &expiry),
        queue: &queue,
        prepared: &prepared,
        rung,
        observed_sequence: SEQUENCE,
        evidence_present: true,
    };
    let map = support::tenancy(&[("mara", "tenant-submitted-budget")]);
    let (mut store, _) = support::install_and_open(
        &root.join("human"),
        &map,
        support::retention_uniform(10_000),
    );
    let mut scope = checked(store.principal(&support::principal("mara")));
    let mut audit = checked(AuditChain::open(&scope));
    let mut inbox = checked(Inbox::new(SEQUENCE, 3));
    checked(inbox.consume(
        &[event],
        &mut peer,
        &mut scope,
        &mut audit,
        &TraceId::mint([1; 16]),
    ));
    let snapshot = checked(inbox.snapshot(SEQUENCE));
    assert_eq!(snapshot.awaiting_count(), 1);
    assert_eq!(snapshot.items().len(), 1);
    assert_eq!(snapshot.observed_sequence(), SEQUENCE);
    let item = snapshot.items()[0];
    assert_eq!(item.state(), &InboxState::AwaitingApproval);
    assert_eq!(item.amount(), 25);
    assert_eq!(
        item.counterparty(),
        prepared.disclosure.counterparties.values()[0].as_str()
    );
    assert_eq!(item.asset(), hex(disclosure.asset));
    assert_eq!(item.fee_limit(), 30);
    assert_eq!(item.activity_expiry(), 200);
    assert_eq!(item.hold_expiry(), HOLD_EXPIRY);
    assert_eq!(
        item.disclosure_digest(),
        prepared.disclosure.canonical_digest
    );
    assert_eq!(item.budget_after().remaining, 945);
    assert_eq!(item.budget_after().rung, rung);
    assert_eq!(item.budget_after().observed_at_sequence, SEQUENCE);
    let verification = if rung == Rung::Committed {
        BudgetVerification::Verified
    } else {
        BudgetVerification::Unverified
    };
    assert_eq!(item.budget_verification(), verification);
    assert_eq!(item.can_approve(), rung == Rung::Committed);
    assert_eq!(
        verification.label(),
        if rung == Rung::Committed {
            "Verified budget"
        } else {
            "Unverified budget"
        }
    );
    let budget = item.budget_after();
    let digest = prepared.disclosure.canonical_digest;
    assert_eq!(
        <[u8; 32]>::from(Sha256::digest(checked(disclosure.reencode()))),
        digest,
        "renderer and inbox must bind the same producer canonical bytes",
    );
    let presentation = checked(DisclosureRenderer::render(&disclosure, digest, budget));
    assert_eq!(presentation.can_approve(), rung == Rung::Committed);
    let ApprovalPresentation::Rendered(rendered) = presentation else {
        panic!("known prepared transfer must remain viewable");
    };
    assert_eq!(rendered.class, ApprovalActivityClass::MoveMoney);
    assert_eq!(rendered.budget_after, budget);
    assert_eq!(rendered.budget_after.verification(), verification);
    assert_eq!(rendered.disclosure_digest, digest);
    assert_eq!(rendered.facts.amount, Some(25));
    assert_eq!(
        rendered.facts.counterparty,
        RenderedCounterparty::Account(disclosure.counterparties[0].account,)
    );
    assert_eq!(rendered.facts.asset, Some(disclosure.asset));
    assert_eq!(rendered.facts.fee_limit, 30);
    assert_eq!(rendered.facts.expires_at, u64::MAX);
    assert_eq!(rendered.copy_key, "approval.activity.move_money");
    assert_eq!(
        rendered.plain_copy,
        format!(
            "Move 25 {} to {}. Fees can be up to 30.",
            hex(disclosure.asset),
            hex(disclosure.counterparties[0].account),
        )
    );
    let mut changed_digest = digest;
    changed_digest[0] ^= 1;
    assert_eq!(
        DisclosureRenderer::render(&disclosure, changed_digest, budget),
        Err(DisclosureRenderError::DigestMismatch)
    );
    for field in 0..6 {
        let mut changed = disclosure.clone();
        match field {
            0 => changed.amounts[0].value += 1,
            1 => changed.counterparties[0].account[0] ^= 1,
            2 => changed.asset[0] ^= 1,
            3 => changed.fee_limit += 1,
            4 => changed.expiry.payload_expires_at -= 1,
            _ => changed.actor[0] ^= 1,
        }
        assert!(
            DisclosureRenderer::render(&changed, digest, budget).is_err(),
            "changed disclosure field {field} must not render at {rung:?}"
        );
    }
    let mut no_evidence = budget;
    no_evidence.evidence_digest = [0; 32];
    assert_eq!(
        DisclosureRenderer::render(&disclosure, digest, no_evidence),
        Err(DisclosureRenderError::UnverifiedBudget)
    );
    let deliveries = checked(Dispatcher::deliveries(&scope));
    assert!(!deliveries.is_empty());
    for delivery in &deliveries {
        let notification = checked(ApprovalNotification::for_delivery(&scope, delivery));
        assert_eq!(notification.amount(), 25);
        assert_eq!(notification.asset(), item.asset());
        assert_eq!(notification.disclosure_digest(), event.disclosure_digest);
    }
    assert_eq!(inbox.next_sequence(), SEQUENCE + 1);
    assert!(matches!(
        inbox.consume(
            &[event],
            &mut peer,
            &mut scope,
            &mut audit,
            &TraceId::mint([2; 16])
        ),
        Err(InboxError::EventOrder {
            expected: 11,
            actual: 10
        })
    ));
    let mut bad_event = event;
    bad_event.disclosure_digest[0] ^= 1;
    let mut unchanged = checked(Inbox::new(SEQUENCE, 3));
    assert!(matches!(
        unchanged.consume(
            &[bad_event],
            &mut peer,
            &mut scope,
            &mut audit,
            &TraceId::mint([3; 16])
        ),
        Err(InboxError::DisclosureChanged)
    ));
    assert_eq!(unchanged.next_sequence(), SEQUENCE);
    assert_eq!(checked(Dispatcher::deliveries(&scope)), deliveries);
    for (observed, present) in [(SEQUENCE + 1, true), (SEQUENCE, false)] {
        peer.observed_sequence = observed;
        peer.evidence_present = present;
        let mut refused = checked(Inbox::new(SEQUENCE, 3));
        assert!(matches!(
            refused.consume(
                &[event],
                &mut peer,
                &mut scope,
                &mut audit,
                &TraceId::mint([6; 16])
            ),
            Err(InboxError::UnverifiedBudget)
        ));
        assert_eq!(refused.next_sequence(), SEQUENCE);
        assert_eq!(checked(Dispatcher::deliveries(&scope)), deliveries);
    }
    peer.observed_sequence = SEQUENCE;
    peer.evidence_present = true;

    let passkeys = required(Passkeys::new(auth_config()), "passkeys");
    let mut authenticator = SoftwareAuthenticator::new();
    let session = register_session(&passkeys, &mut scope, &mut authenticator, NOW);
    let challenge = required(
        passkeys.begin_step_up(
            &mut scope,
            session.access_token().expose(),
            session.csrf_token().expose(),
            OperationDigest::new(prepared.disclosure.canonical_digest),
            110,
        ),
        "begin step-up",
    );
    let assertion = authenticator.assert(&challenge.ceremony);
    let evidence = required(
        passkeys.finish_step_up(&mut scope, &challenge.challenge_id, &assertion, 111),
        "finish step-up",
    );
    if rung != Rung::Committed {
        let before = scope.keys(Table::Journeys);
        assert!(matches!(
            Decisions::approve(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                &mut peer,
                [1; 32],
                "submitted-must-not-release",
                SEQUENCE,
                112,
                &mut audit,
                &TraceId::mint([4; 16]),
            ),
            Err(DecisionError::UnverifiedBudget)
        ));
        assert_eq!(scope.keys(Table::Journeys), before);
    }
    peer.rung = Rung::Committed;
    for (observed, present) in [(SEQUENCE + 1, true), (SEQUENCE, false)] {
        peer.observed_sequence = observed;
        peer.evidence_present = present;
        assert!(matches!(
            Decisions::approve(
                &mut scope,
                &passkeys,
                session.access_token().expose(),
                session.csrf_token().expose(),
                &evidence,
                &mut peer,
                [1; 32],
                "committed-must-have-bound-evidence",
                SEQUENCE,
                112,
                &mut audit,
                &TraceId::mint([7; 16]),
            ),
            Err(DecisionError::UnverifiedBudget)
        ));
    }
    peer.observed_sequence = SEQUENCE;
    peer.evidence_present = true;
    // Even a committed budget supplies no passkey/session authority of its own.
    assert!(matches!(
        Decisions::approve(
            &mut scope,
            &passkeys,
            session.access_token().expose(),
            "wrong-csrf",
            &evidence,
            &mut peer,
            [1; 32],
            "committed-still-needs-auth",
            SEQUENCE,
            112,
            &mut audit,
            &TraceId::mint([5; 16]),
        ),
        Err(DecisionError::Auth(_))
    ));
    assert_eq!(
        checked(registry.ticket([1; 32]))
            .expect("held ticket")
            .state,
        ApprovalState::AwaitingApproval
    );
    assert!(queue.is_empty());
    assert_eq!(checked(limiter.consumed(LIMIT)), 0);
    assert_eq!(checked(limiter.held_reservations()), 1);
    drop(scope);
    drop(store);
    drop(peer);
    drop(expiry);
    checked(fs::remove_dir_all(root));
}

#[test]
fn inbox_submitted_budget_refused_real_producer_transport_consumer() {
    exercise_path(Rung::Submitted);
}

#[test]
fn renderer_submitted_budget_refused_real_producer_transport_consumer() {
    for rung in [Rung::Submitted, Rung::Receipted] {
        exercise_path(rung);
    }
}

#[test]
fn committed_budget_keeps_exact_disclosure_presentation_and_authentication() {
    exercise_path(Rung::Committed);
}
