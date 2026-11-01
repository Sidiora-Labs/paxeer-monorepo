use super::*;
use crate::agents::{AgentFailure, ProtocolEvidence};
use layerx_crypto::rotation::OwnerRotationState;
use serde::{Deserialize, Serialize};

#[derive(Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
enum Phase {
    Announce,
    Wait,
    Commit,
    Project,
    Revoke,
    Session,
    Done,
    Cancel,
    Cancelled,
    Refused,
}

#[derive(Serialize, Deserialize)]
struct CommittedOutcome {
    signed_activity: Vec<u8>,
    action_key: [u8; 32],
    activity_id: [u8; 32],
    receipt_digest: [u8; 32],
    observed_sequence: u64,
    verification: u8,
    finalized_at: u64,
}
impl CommittedOutcome {
    fn evidence(&self) -> super::super::agent_runtime::AgentFinalizationEvidence {
        super::super::agent_runtime::AgentFinalizationEvidence {
            action_key: self.action_key,
            activity_id: self.activity_id,
            receipt_digest: self.receipt_digest,
            observed_sequence: self.observed_sequence,
            verification: self.verification,
            finalized_at: self.finalized_at,
        }
    }
}

#[derive(Serialize, Deserialize)]
struct Rotation {
    version: u8,
    agent_id: String,
    did: String,
    network: u32,
    action: [u8; 32],
    old_key: [u8; 32],
    new_key: [u8; 32],
    old_custody: String,
    new_custody: String,
    delay: u64,
    window: u64,
    begin: u64,
    end: u64,
    effective_sequence: u64,
    started_at: u64,
    stage_at: u64,
    phase: Phase,
    announcement: [u8; 32],
    predecessor: [u8; 32],
    revoke_sequence: u64,
    announced: Option<CommittedOutcome>,
    committed: Option<CommittedOutcome>,
    terminal_evidence: Option<CommittedOutcome>,
    result_code: Option<i32>,
}
impl Rotation {
    fn load(
        scope: &crate::store::PrincipalScope<'_>,
        id: &crate::notify::JourneyId,
    ) -> Result<Self, ApiFailure> {
        let key = rotation_key(id)?;
        let row = scope
            .get(Table::Journeys, &key)
            .ok_or_else(ApiFailure::not_found)?;
        let value: Self =
            serde_json::from_slice(row.bytes()).map_err(|_| ApiFailure::upstream_degraded())?;
        if value.version != 1
            || value.action == [0; 32]
            || value.old_key == value.new_key
            || value.begin >= value.end
            || value.delay == 0
            || value.window == 0
            || id.as_str() != rotation_id(value.action)?.as_str()
        {
            return Err(ApiFailure::upstream_degraded());
        }
        Ok(value)
    }
    fn save(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        now: u64,
    ) -> Result<(), ApiFailure> {
        scope
            .put(
                Table::Journeys,
                rotation_key(&rotation_id(self.action)?)?,
                now,
                serde_json::to_vec(self).map_err(|_| ApiFailure::upstream_degraded())?,
            )
            .map_err(|_| ApiFailure::unavailable())
    }
    fn stage_key(&self, stage: u8) -> [u8; 32] {
        let mut hash = Sha256::new();
        hash.update(b"layerx-human/owner-rotation/v1\0");
        hash.update(self.action);
        hash.update([stage]);
        hash.finalize().into()
    }
    fn terminal(&self) -> bool {
        matches!(self.phase, Phase::Done | Phase::Cancelled | Phase::Refused)
    }
    fn challenge(&self) -> Result<serde_json::Value, ApiFailure> {
        let evidence = self
            .announced
            .as_ref()
            .ok_or_else(ApiFailure::upstream_degraded)?;
        if !(4..=5).contains(&evidence.verification) {
            return Err(ApiFailure::upstream_degraded());
        }
        let receipt = super::super::projection::managed_evidence(
            &super::super::agent_runtime::ManagedAgentEvidence {
                evidence_id: hex_bytes(&evidence.receipt_digest),
                class: "rotate".to_owned(),
                verification: evidence.verification,
            },
        )?;
        Ok(json!({
            "agent_id": self.agent_id,
            "kind": "rotate",
            "delay_copy_key": "agent.keys.rotate-delay",
            "delay_seconds": self.delay,
            "ready_at": super::super::projection::unix_milliseconds(self.begin)?,
            "evidence": [receipt],
        }))
    }
}
fn rotation_id(action: [u8; 32]) -> Result<crate::notify::JourneyId, ApiFailure> {
    crate::notify::JourneyId::new(format!("jrn_{}", hex_bytes(&action)))
        .map_err(|_| ApiFailure::upstream_degraded())
}
fn rotation_key(id: &crate::notify::JourneyId) -> Result<RowKey, ApiFailure> {
    RowKey::new(format!("owner-rotation-{}", id.as_str()))
        .map_err(|_| ApiFailure::upstream_degraded())
}
fn creation_failure(_: AgentFailure) -> ApiFailure {
    ApiFailure::upstream_degraded()
}
fn milliseconds(value: u64) -> Result<u64, ApiFailure> {
    value
        .checked_mul(1000)
        .ok_or_else(ApiFailure::upstream_degraded)
}
fn checked_identity(agent: &mut AgentRuntime, did: &str) -> Result<OwnerRotationState, ApiFailure> {
    let identity = agent.identity_resolve(did).map_err(agent_failure)?;
    if identity.frozen || identity.verification < 4 || identity.verification > 5 {
        return Err(ApiFailure::upstream_degraded());
    }
    let state = OwnerRotationState::decode(
        &identity.canonical_bytes,
        &Did::new(did.as_bytes()).map_err(|_| ApiFailure::upstream_degraded())?,
    )
    .map_err(|_| ApiFailure::upstream_degraded())?;
    if state.revocation_sequence != identity.revocation_sequence
        || state.observed_sequence > identity.head_sequence
        || !identity
            .authorities
            .contains(&(1, state.primary_public_key))
    {
        return Err(ApiFailure::upstream_degraded());
    }
    Ok(state)
}

impl ProductionComponents {
    pub(super) fn execute_agent_rotate(
        &self,
        request: &ScopedRequest<'_>,
        scope: &mut crate::store::PrincipalScope<'_>,
        _principal: &crate::store::PrincipalId,
    ) -> Result<BackendResponse, ApiFailure> {
        if request.operation.name == "agent.recover" {
            return Err(ApiFailure::upstream_degraded());
        }
        let action = action_key(required_idempotency(request)?);
        let id = rotation_id(action)?;
        let agent_id = path(request, "agent_id")?;
        if scope.get(Table::Journeys, &rotation_key(&id)?).is_none() {
            return Err(ApiFailure::operation_retired());
        }
        let record = Rotation::load(scope, &id)?;
        if record.agent_id != agent_id
            || (request.operation.name == "agent.rotation.start"
                && rotation_bounds(&request.body)? != (record.delay, record.window))
        {
            return Err(ApiFailure::invalid_request(None));
        }
        schedule_continuation(scope, "agent-rotation", &id, self.now()?)?;
        let trace =
            TraceId::parse(&request.trace).map_err(|_| ApiFailure::invalid_request(None))?;
        self.advance_owner_rotation(scope, &id, &trace, self.now()?)?;
        Ok(BackendResponse {
            result: Rotation::load(scope, &id)?.challenge()?,
            session: None,
        })
    }

    pub(super) fn advance_owner_rotation(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        id: &crate::notify::JourneyId,
        trace: &TraceId,
        observed_at: u64,
    ) -> Result<bool, ApiFailure> {
        let mut record = Rotation::load(scope, id)?;
        if record.terminal() {
            return Ok(true);
        }
        let mut agent = self.principal_agent(scope)?;
        match record.phase {
            Phase::Announce | Phase::Commit | Phase::Cancel => {
                return Err(ApiFailure::operation_retired());
            }
            Phase::Wait => {
                if milliseconds(observed_at)? < record.begin {
                    return Ok(false);
                }
                let state = checked_identity(&mut agent, &record.did)?;
                if state.primary_public_key != record.old_key
                    || state.pending_public_key != Some(record.new_key)
                    || state.begin != record.begin
                    || state.end != record.end
                    || state.effective_sequence != record.effective_sequence
                {
                    return Err(ApiFailure::upstream_degraded());
                }
                record.announcement = state.announcement;
                record.stage_at = observed_at;
                record.phase = if milliseconds(observed_at)? > record.end {
                    Phase::Cancel
                } else {
                    Phase::Commit
                };
            }
            Phase::Project => {
                let committed = record
                    .committed
                    .as_ref()
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                agent
                    .agent_owner_rotated(
                        &record.agent_id,
                        &record.new_custody,
                        &committed.signed_activity,
                        committed.evidence(),
                    )
                    .map_err(agent_failure)?;
                record.revoke_sequence = agent
                    .head()
                    .map_err(agent_failure)?
                    .chain_sequence
                    .checked_add(1)
                    .ok_or_else(ApiFailure::upstream_degraded)?;
                record.stage_at = observed_at;
                record.phase = Phase::Revoke;
            }
            Phase::Revoke => {
                self.revoke_rotation(scope, &mut agent, trace, &mut record, observed_at)?;
            }
            Phase::Session => {
                self.restore_rotation_session(scope, &mut agent, trace, &record)?;
                record.phase = Phase::Done;
            }
            Phase::Done | Phase::Cancelled | Phase::Refused => return Ok(true),
        }
        record.save(scope, observed_at)?;
        Ok(record.terminal())
    }
}

impl ProductionComponents {
    fn revoke_rotation(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        trace: &TraceId,
        record: &mut Rotation,
        observed_at: u64,
    ) -> Result<(), ApiFailure> {
        let intent = Intent::v1(IntentKind::SessionRevoke(
            SessionRevoke::new(
                AuthorityGrantId::new(record.predecessor),
                SessionRevocationReason::PrimaryKeyRotated,
                ProtocolSequence::from_u64(record.revoke_sequence),
            )
            .map_err(|_| ApiFailure::upstream_degraded())?,
        ));
        let outcome = self.submit_rotation(
            scope,
            agent,
            trace,
            record,
            RotationSubmission {
                intent,
                stage: 3,
                new_owner: true,
            },
        )?;
        accept_outcome(record, &outcome, 6, observed_at)?;
        if !record.terminal() {
            record.phase = Phase::Session;
            record.stage_at = observed_at;
        }
        Ok(())
    }
}

fn rotation_bounds(body: &serde_json::Value) -> Result<(u64, u64), ApiFailure> {
    let value = |name| {
        body.get(name)
            .and_then(serde_json::Value::as_u64)
            .filter(|value| (1..=u64::from(u32::MAX)).contains(value))
            .ok_or_else(|| ApiFailure::invalid_request(Some(name)))
    };
    Ok((value("delay_seconds")?, value("window_seconds")?))
}
fn rotation_did(record: &Rotation) -> Result<Did, ApiFailure> {
    Did::new(record.did.as_bytes()).map_err(|_| ApiFailure::upstream_degraded())
}
struct RotationSubmission {
    intent: Intent,
    stage: u8,
    new_owner: bool,
}
fn accept_outcome(
    record: &mut Rotation,
    outcome: &ProtocolEvidence,
    ordinal: u16,
    now: u64,
) -> Result<Option<CommittedOutcome>, ApiFailure> {
    let kind = layerx_types::payload::ActivityType::new(ModuleId::Governance, ordinal)
        .map_err(|_| ApiFailure::upstream_degraded())?;
    let verified = outcome.verify_outcome(kind).map_err(creation_failure)?;
    let protocol = verified
        .protocol()
        .ok_or_else(ApiFailure::upstream_degraded)?;
    if outcome.rung != layerx_types::rung::Rung::Committed
        || outcome.network_id != record.network
        || outcome.actor != record.did.as_bytes()
    {
        return Err(ApiFailure::upstream_degraded());
    }
    record.result_code = Some(protocol.result_code());
    let finalized = CommittedOutcome {
        signed_activity: outcome.signed_activity.clone(),
        action_key: outcome.action_key,
        activity_id: outcome.activity_id,
        receipt_digest: Sha256::digest(&outcome.receipt_bytes).into(),
        observed_sequence: protocol.global_sequence(),
        verification: outcome.rung.wire_rank(),
        finalized_at: now,
    };
    if protocol.result_code() != 0 {
        record.phase = Phase::Refused;
        record.terminal_evidence = Some(finalized);
        return Ok(None);
    }
    Ok(Some(finalized))
}

impl ProductionComponents {
    fn rotation_adapter<'a>(
        &'a self,
        agent: &'a mut AgentRuntime,
        trace: &'a TraceId,
        record: &Rotation,
        new_owner: bool,
    ) -> Result<ProductionAgentCreation<'a>, ApiFailure> {
        let public_key = if new_owner {
            record.new_key
        } else {
            record.old_key
        };
        ProductionAgentCreation::new(
            agent,
            &self.agent_contract,
            &self.custody,
            trace,
            AgentDid::new(record.did.clone()).map_err(|_| ApiFailure::upstream_degraded())?,
            AuthorityRef::new(hex_bytes(&public_key))
                .map_err(|_| ApiFailure::upstream_degraded())?,
            super::super::agent_creation::CreationBounds {
                timestamp_span: self.agent_timestamp_span_seconds,
                fee_limit: self.agent_fee_limit,
            },
        )
        .map_err(creation_failure)
    }

    fn submit_rotation(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        trace: &TraceId,
        record: &Rotation,
        submission: RotationSubmission,
    ) -> Result<ProtocolEvidence, ApiFailure> {
        let registry = agent.registry().clone();
        let key = if submission.new_owner {
            &record.new_custody
        } else {
            &record.old_custody
        };
        let mut adapter = self.rotation_adapter(agent, trace, record, submission.new_owner)?;
        adapter
            .submit_lifecycle_intent(
                scope,
                &registry,
                submission.intent,
                record.stage_key(submission.stage),
                KeyId::new(key.clone()).map_err(|_| ApiFailure::upstream_degraded())?,
                record.stage_at,
            )
            .map_err(creation_failure)
    }

    fn restore_rotation_session(
        &self,
        scope: &mut crate::store::PrincipalScope<'_>,
        agent: &mut AgentRuntime,
        trace: &TraceId,
        record: &Rotation,
    ) -> Result<(), ApiFailure> {
        let context = agent
            .agent_context(&record.agent_id)
            .map_err(agent_failure)?;
        if context.agent_did != record.did
            || context.seed.custody_public_key != record.new_key
            || context.seed.custody_key != record.new_custody
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let identity = checked_identity(agent, &record.did)?;
        let prior = agent
            .session_fee_state(record.predecessor)
            .map_err(agent_failure)?;
        validate_resumed_session(&prior, &record.did)?;
        let action_key = record.stage_key(4);
        let lifetime = context
            .seed
            .session_expiry_unix_seconds
            .checked_sub(context.seed.created_at)
            .filter(|value| *value > 0)
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let registry = agent.registry().clone();
        let mut adapter = self.rotation_adapter(agent, trace, record, true)?;
        let evidence = adapter
            .provision_session_scoped(
                scope,
                &registry,
                SessionProvision {
                    native_fee_budget: prior.grant.fee_budget,
                    replacement: prior.grant.fee_budget.map(|_| prior.clone()),
                    not_before: record.stage_at,
                    action_key,
                    did: rotation_did(record)?,
                    activity_types: context
                        .seed
                        .activity_types
                        .iter()
                        .map(|value| layerx_types::payload::ActivityType::from_u32(*value))
                        .collect::<Result<Vec<_>, _>>()
                        .map_err(|_| ApiFailure::upstream_degraded())?,
                    daemon_scopes: context.seed.session_scopes.clone(),
                    expires_at: record
                        .stage_at
                        .checked_add(lifetime)
                        .ok_or_else(ApiFailure::upstream_degraded)?,
                    primary_authority: record.new_key,
                    grantor: prior.grant.grantor,
                    custody_key: KeyId::new(record.new_custody.clone())
                        .map_err(|_| ApiFailure::upstream_degraded())?,
                    revocation_sequence: identity.revocation_sequence,
                },
            )
            .map_err(creation_failure)?;
        let (token_id, generation, finalization) = adapter
            .take_latest_session_credential()
            .map_err(creation_failure)?;
        if finalization.action_key != action_key
            || finalization.receipt_digest != evidence.receipt_digest
            || finalization.observed_sequence != evidence.observed_sequence
            || finalization.verification != evidence.rung.wire_rank()
        {
            return Err(ApiFailure::upstream_degraded());
        }
        let observation = agent
            .agent_session_bind(&record.agent_id, action_key, token_id, action_key)
            .map_err(agent_failure)?;
        if observation.generation != generation {
            return Err(ApiFailure::upstream_degraded());
        }
        let updated = agent
            .agent_context(&record.agent_id)
            .map_err(agent_failure)?;
        let successor = agent
            .session_fee_state(updated.protocol_grant_id)
            .map_err(agent_failure)?;
        let predecessor = agent
            .session_fee_state(record.predecessor)
            .map_err(agent_failure)?;
        if successor.grant.fee_budget != prior.grant.fee_budget
            || (prior.grant.fee_budget.is_some()
                && (predecessor.successor != successor.grant.grant_id
                    || successor.spent_total != prior.spent_total
                    || successor.spent_this_period != prior.spent_this_period
                    || successor.period_start != prior.period_start))
            || updated.spent != context.spent
            || updated.current_monthly_limit != context.current_monthly_limit
            || updated.active_budget_id != context.active_budget_id
            || updated.state != context.state
        {
            return Err(ApiFailure::upstream_degraded());
        }
        Ok(())
    }
}

impl ProductionComponents {
    pub(super) fn execute_rotation_disclosure(
        &self,
        request: &ScopedRequest<'_>,
        scope: &crate::store::PrincipalScope<'_>,
    ) -> Result<BackendResponse, ApiFailure> {
        let (delay, window) = rotation_bounds(&request.body)?;
        let idempotency_key = request
            .body
            .get("idempotency_key")
            .and_then(serde_json::Value::as_str)
            .filter(|value| {
                !value.is_empty()
                    && value.len() <= 128
                    && !value.bytes().any(|b| b.is_ascii_control())
            })
            .ok_or_else(|| ApiFailure::invalid_request(Some("idempotency_key")))?;
        let agent_id = path(request, "agent_id")?;
        self.principal_agent(scope)?
            .agent_context(agent_id)
            .map_err(agent_failure)?;
        let schema = ApiSchema::v1().map_err(|_| ApiFailure::upstream_degraded())?;
        let operation = schema
            .operation("agent.rotation.start")
            .ok_or_else(ApiFailure::upstream_degraded)?;
        let destination = format!("/v1/agents/{agent_id}/rotation");
        let digest = super::super::production_auth::step_up_digest(
            scope.principal(),
            scope.tenant(),
            operation,
            &destination,
            &request.path_parameters,
            &json!({"delay_seconds": delay, "window_seconds": window}),
            Some(idempotency_key),
        )
        .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(BackendResponse {
            result: json!({"confirms": format!("opd_{}", URL_SAFE_NO_PAD.encode(digest.bytes()))}),
            session: None,
        })
    }
}
