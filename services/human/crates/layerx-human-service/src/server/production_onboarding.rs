use super::*;
use crate::onboarding::{NativeSponsor, OnboardingStatus};
use crate::store::{PrincipalId, PrincipalScope};

impl ProductionComponents {
    pub(super) fn advance_native_onboarding(
        &self,
        principal: &PrincipalId,
        trace: &str,
        observed_at: u64,
    ) -> Result<OnboardingStatus, ApiFailure> {
        let trace = TraceId::parse(trace).map_err(|_| ApiFailure::invalid_request(None))?;
        let mut store = self.store.lock().map_err(|_| ApiFailure::unavailable())?;
        let mut journey = self.load_native_onboarding(&mut store, principal, observed_at)?;
        if matches!(
            journey.status().state(),
            crate::onboarding::OnboardingState::Complete
                | crate::onboarding::OnboardingState::Refused
        ) {
            return Ok(journey.status());
        }
        if principal == &self.onboarding_sponsor_principal {
            return self.finalize_onboarding_sponsor(
                &mut store,
                principal,
                &mut journey,
                &trace,
                observed_at,
            );
        }
        Err(ApiFailure::operation_retired())
    }

    fn load_native_onboarding(
        &self,
        store: &mut PrincipalStore,
        principal: &PrincipalId,
        observed_at: u64,
    ) -> Result<OnboardingJourney, ApiFailure> {
        let mut scope = store
            .principal(principal)
            .map_err(|_| ApiFailure::unavailable())?;
        let mut journey = OnboardingJourney::load(&scope)
            .map_err(|_| ApiFailure::upstream_degraded())?
            .ok_or_else(ApiFailure::not_found)?;
        self.custody
            .resume_onboarding_local(&mut journey, &mut scope, observed_at)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        Ok(journey)
    }

    fn finalize_onboarding_sponsor(
        &self,
        store: &mut PrincipalStore,
        principal: &PrincipalId,
        journey: &mut OnboardingJourney,
        trace: &TraceId,
        observed_at: u64,
    ) -> Result<OnboardingStatus, ApiFailure> {
        let mut scope = store
            .principal(principal)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let mut agent = self.principal_agent(&scope)?;
        self.native_onboarding_sponsor(&scope, &mut agent)?;
        self.custody
            .describe_key(principal, &primary_key()?)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let registry = agent.registry().clone();
        for (name, stage) in [
            (
                "identity",
                crate::onboarding::ProtocolStage::DidRegistration,
            ),
            (
                "recovery",
                crate::onboarding::ProtocolStage::RecoveryRegistration,
            ),
        ] {
            let row = RowKey::new(format!("onboarding-sponsor-{name}"))
                .map_err(|_| ApiFailure::upstream_degraded())?;
            let signed = scope
                .get(Table::Journeys, &row)
                .ok_or_else(ApiFailure::upstream_degraded)?
                .bytes()
                .to_vec();
            let (call, authority) = verify_owner_activity(&signed)?;
            let sender = authority[12..]
                .try_into()
                .map_err(|_| ApiFailure::upstream_degraded())?;
            let disclosure =
                bind_call(&call, sender).map_err(|_| ApiFailure::upstream_degraded())?;
            let activity_id = transaction_hash(&signed);
            let material = match agent
                .evidence(disclosure.idempotency_key, activity_id)
                .map_err(agent_failure)?
            {
                crate::journeys::ReceiptLookup::Found(value) => value,
                crate::journeys::ReceiptLookup::Absent
                | crate::journeys::ReceiptLookup::Submitted { .. } => {
                    return Err(ApiFailure::upstream_degraded())
                }
            };
            if call.chain_id != u64::from(self.network_id) {
                return Err(ApiFailure::forbidden());
            }
            if disclosure.actor != self.agent_actor.as_str().as_bytes() {
                return Err(ApiFailure::upstream_degraded());
            }
            let evidence =
                ProductionAgentCreation::protocol_evidence(signed, authority, &registry, material)
                    .map_err(|_| ApiFailure::upstream_degraded())?;
            journey
                .accept_bootstrap_owner(&mut scope, stage, &evidence, &registry, trace, observed_at)
                .map_err(|_| ApiFailure::upstream_degraded())?;
            if journey.status().state() == crate::onboarding::OnboardingState::Refused {
                break;
            }
        }
        Ok(journey.status())
    }

    fn native_onboarding_sponsor(
        &self,
        scope: &PrincipalScope<'_>,
        agent: &mut AgentRuntime,
    ) -> Result<NativeSponsor, ApiFailure> {
        if scope.principal() != &self.onboarding_sponsor_principal {
            return Err(ApiFailure::forbidden());
        }
        let owner = owner::resolve_principal_owner(self, scope, agent)?;
        if owner.actor != self.agent_actor
            || owner.authority != self.agent_authority
            || owner.account.canonical() != self.agent_owner_account
        {
            return Err(ApiFailure::upstream_degraded());
        }
        if owner.recovery_policy()? != (self.agent_recovery_root, self.agent_recovery_threshold) {
            return Err(ApiFailure::upstream_degraded());
        }
        let key = self
            .custody
            .describe_key(scope.principal(), &primary_key()?)
            .map_err(|_| ApiFailure::upstream_degraded())?;
        let asset = agent.native_fee_policy().map_err(agent_failure)?.asset_id;
        Ok(NativeSponsor {
            principal: scope.principal().as_str().to_owned(),
            did: Did::new(owner.actor.as_str().as_bytes())
                .map_err(|_| ApiFailure::upstream_degraded())?,
            public_key: key.public_key,
            account: owner.account,
            asset,
            network_id: self.network_id,
            initial_funding: self.onboarding_initial_funding,
            timestamp_span: self.agent_timestamp_span_seconds,
        })
    }
}

fn primary_key() -> Result<KeyId, ApiFailure> {
    KeyId::new("human-primary").map_err(|_| ApiFailure::upstream_degraded())
}
