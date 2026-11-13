use super::*;
use crate::agents::ProtocolEvidence;
use layerx_types::account::AccountId;
use layerx_types::rung::Rung;

const FUNDING_ROW: &str = "onboarding-native-funding-receipt";

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct NativeSponsor {
    pub principal: String,
    pub did: Did,
    pub public_key: [u8; 32],
    pub account: AccountId,
    pub asset: [u8; 32],
    pub network_id: u32,
    pub initial_funding: u128,
    pub timestamp_span: u64,
}

impl OnboardingJourney {
    /// Prepares native setup using the sender's pending nonce from `ACCOUNT_SEQUENCE`.
    ///
    /// # Errors
    /// Refuses an unavailable nonce answer or an invalid preparation without
    /// retaining the caller's sequence as a fallback.
    pub fn prepare_native_agent_action(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        registry: &ModuleRegistry,
        agent: &AgentClient,
        runtime: &mut crate::server::agent_runtime::AgentRuntime,
        context: &AgentPreparationContext,
        now: u64,
    ) -> Result<Option<AgentPrepareAction>, OnboardingError> {
        let Some(stage) = self.next_action_stage()? else {
            return Ok(None);
        };
        if self.stage(stage).preparation.is_some() {
            return self.prepare_agent_action(scope, registry, agent, context, now);
        }
        let mut pending = context.clone();
        pending.account_sequence = runtime
            .account_sequence(&context.actor, &context.authority)
            .map_err(|_| OnboardingError::InvalidAgentContext)?;
        pending.pending_nonce = pending.account_sequence;
        self.prepare_agent_action(scope, registry, agent, &pending, now)
    }

    /// Accepts a native owner setup outcome after its committed rung is observed.
    ///
    /// # Errors
    /// Refuses uncommitted, conflicting, or unverifiable setup evidence.
    pub fn accept_bootstrap_owner(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        stage: ProtocolStage,
        evidence: &ProtocolEvidence,
        registry: &ModuleRegistry,
        trace: &TraceId,
        now: u64,
    ) -> Result<(), OnboardingError> {
        let (did, public_key, policy) = self.bootstrap_identity()?;
        let request = match stage {
            ProtocolStage::DidRegistration => layerx_intents::NativeOwnerBootstrap::Identity {
                did,
                primary_key: layerx_types::intent::PublicKey::new(public_key),
            },
            ProtocolStage::RecoveryRegistration => {
                layerx_intents::NativeOwnerBootstrap::RecoveryPolicy {
                    did,
                    root: policy.root(),
                    threshold: policy.threshold(),
                    minimum_delay: policy.challenge_delay_secs(),
                    maximum_delay: policy.challenge_delay_secs(),
                }
            }
        };
        let compiled = request.compile(registry)?;
        if evidence.rung != Rung::Committed
            || stage == ProtocolStage::RecoveryRegistration && !self.did_verified()
        {
            return Err(OnboardingError::EvidenceConflict);
        }
        let activity = evidence
            .bound_activity(compiled.activity_type())
            .map_err(|_| OnboardingError::EvidenceConflict)?;
        if activity.payload() != compiled.payload().as_bytes() {
            return Err(OnboardingError::EvidenceConflict);
        }
        let verified = evidence
            .verify_outcome(compiled.activity_type())
            .map_err(|_| OnboardingError::EvidenceConflict)?;
        let result = verified
            .protocol()
            .ok_or(OnboardingError::ReceiptShape)?
            .result_code();
        self.accept_native_stage(scope, (stage, result), evidence, trace, now)
    }

    pub(crate) fn bootstrap_identity(
        &self,
    ) -> Result<(Did, [u8; 32], RecoveryPolicy), OnboardingError> {
        Ok((
            self.did()?,
            self.record
                .public_key
                .ok_or(OnboardingError::CustodyKeyRequired)?,
            RecoveryPolicy::new(
                RecoveryRoot::new(self.record.recovery_root),
                ApprovalThreshold::new(self.record.recovery_threshold)
                    .map_err(|_| OnboardingError::EvidenceConflict)?,
                self.record.recovery_challenge_delay_secs,
            )?,
        ))
    }

    fn accept_native_stage(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        outcome: (ProtocolStage, i32),
        evidence: &ProtocolEvidence,
        trace: &TraceId,
        now: u64,
    ) -> Result<(), OnboardingError> {
        let (stage, result) = outcome;
        if evidence.action_key != self.action_key(stage) {
            return Err(OnboardingError::EvidenceConflict);
        }
        retain_native_outcome(
            scope,
            self.stage_mut(stage),
            receipt_row(stage)?,
            evidence,
            result,
            now,
        )?;
        self.persist(scope, now)?;
        if result == 0 {
            let mut audit = AuditChain::open(scope)?;
            self.ensure_activation_audit(scope, &mut audit, trace, stage, now)?;
        }
        Ok(())
    }
}

pub(super) fn funding_row() -> RowKey {
    RowKey::new(FUNDING_ROW).unwrap_or_else(|_| unreachable!("static onboarding funding row"))
}

fn retain_native_outcome(
    scope: &mut PrincipalScope<'_>,
    record: &mut ProtocolRecord,
    row: RowKey,
    evidence: &ProtocolEvidence,
    result: i32,
    now: u64,
) -> Result<(), OnboardingError> {
    if evidence.rung != Rung::Committed {
        return Err(OnboardingError::EvidenceConflict);
    }
    let digest: [u8; 32] = Sha256::digest(&evidence.receipt_bytes).into();
    if record.receipt_digest.is_some_and(|old| old != digest)
        || record
            .activity_id
            .is_some_and(|old| old != evidence.activity_id)
        || record.state == ProtocolState::Refused && record.refusal_code != Some(result)
        || record.receipt_digest.is_some() && record.refusal_code != (result != 0).then_some(result)
    {
        return Err(OnboardingError::EvidenceConflict);
    }
    put_exact(
        scope,
        RowKey::new(format!("{}-activity", row.as_str()))?,
        now,
        evidence.signed_activity.clone(),
    )?;
    put_exact(scope, row, now, evidence.receipt_bytes.clone())?;
    record.unavailable = None;
    record.submission_ref = Some(hex(&evidence.activity_id));
    record.receipt_digest = Some(digest);
    record.activity_id = Some(evidence.activity_id);
    record.evidence_rung = Some(evidence.rung.wire_rank());
    if result == 0 {
        record.state = ProtocolState::Verified;
        record.refusal_code = None;
    } else {
        record.state = ProtocolState::Refused;
        record.refusal_code = Some(result);
    }
    Ok(())
}
