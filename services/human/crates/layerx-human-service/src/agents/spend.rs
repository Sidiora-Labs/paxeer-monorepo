//! Principal-scoped projections of verified canonical budget and receipt state.

use std::fmt::{Display, Formatter};

use layerx_types::rung::Rung;

use crate::store::{PrincipalId, PrincipalScope, RowKey, StoreError, Table};
use layerx_agentd::budget::{
    self, LocalAccounting, ProtocolBudgetRecord, ProtocolBudgetState, SpendReceiptEvidence,
};
use layerx_agentd::protocol_evidence::EvidenceAuthority;
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};

/// Reserved copy-catalog key for the future canonical reconciliation result.
pub const RECONCILIATION_COPY_KEY: &str = "agent.spend.protocol-adopted";
/// Reserved reconciliation note for both shells once canonical state exists.
pub const RECONCILIATION_EXPLANATION: &str =
    "Recent verified activity and the network budget differed. The network figure is shown.";

/// The two native renderers consuming the same Human API agent record.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub enum AgentShell {
    Mobile,
    Desktop,
}

/// Immutable identity and protocol-account facts for one managed agent.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SpendProfile {
    pub principal: PrincipalId,
    pub agent_id: [u8; 32],
    pub agent_account: [u8; 32],
    pub budget_id: [u8; 32],
    pub asset: [u8; 32],
}

impl SpendProfile {
    fn validate(&self) -> Result<(), SpendError> {
        if self.agent_id == [0; 32]
            || self.agent_account == [0; 32]
            || self.budget_id == [0; 32]
            || self.asset == [0; 32]
        {
            return Err(SpendError::InvalidProfile);
        }
        Ok(())
    }
}

/// Direction of a visible receipt/protocol discrepancy.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ReconciliationDirection {
    ProtocolHigher,
    ProtocolLower,
}

/// Honest status of the local receipt calculation against protocol authority.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SpendReconciliationStatus {
    InSync,
    ProtocolAdopted {
        direction: ReconciliationDirection,
        difference: u128,
    },
}

/// Future output schema for current-period spend after canonical integration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AgentSpendView {
    agent_id: [u8; 32],
    period_start: u64,
    period_end: u64,
    /// Authoritative figure to be shown only after adopting verified protocol
    /// budget state and comparing it with receipt-only accounting.
    spent: u128,
    /// Independently recomputed total from verified current-period receipts.
    receipt_spent: u128,
    limit: u128,
    remaining: u128,
    receipt_count: usize,
    verification_level: Rung,
    observed_head_sequence: u64,
    reconciliation: SpendReconciliationStatus,
    reconciliation_copy_key: Option<&'static str>,
    reconciliation_explanation: Option<&'static str>,
    evidence_digests: Vec<[u8; 32]>,
}

impl AgentSpendView {
    #[must_use]
    pub const fn agent_id(&self) -> [u8; 32] {
        self.agent_id
    }

    #[must_use]
    pub const fn period_start(&self) -> u64 {
        self.period_start
    }

    #[must_use]
    pub const fn period_end(&self) -> u64 {
        self.period_end
    }

    #[must_use]
    pub const fn spent(&self) -> u128 {
        self.spent
    }

    #[must_use]
    pub const fn receipt_spent(&self) -> u128 {
        self.receipt_spent
    }

    #[must_use]
    pub const fn limit(&self) -> u128 {
        self.limit
    }

    #[must_use]
    pub const fn remaining(&self) -> u128 {
        self.remaining
    }

    #[must_use]
    pub const fn receipt_count(&self) -> usize {
        self.receipt_count
    }

    #[must_use]
    pub const fn verification_level(&self) -> Rung {
        self.verification_level
    }

    #[must_use]
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }

    #[must_use]
    pub const fn reconciliation(&self) -> SpendReconciliationStatus {
        self.reconciliation
    }

    #[must_use]
    pub const fn reconciliation_copy_key(&self) -> Option<&'static str> {
        self.reconciliation_copy_key
    }

    #[must_use]
    pub const fn reconciliation_explanation(&self) -> Option<&'static str> {
        self.reconciliation_explanation
    }

    #[must_use]
    pub fn evidence_digests(&self) -> &[[u8; 32]] {
        &self.evidence_digests
    }
}

/// Future shell-tagged projection of the Human API agent-spend contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ShellAgentSpend {
    shell: AgentShell,
    spend: AgentSpendView,
}

impl ShellAgentSpend {
    #[must_use]
    pub const fn shell(&self) -> AgentShell {
        self.shell
    }

    #[must_use]
    pub const fn spend(&self) -> &AgentSpendView {
        &self.spend
    }
}

/// Future native shell projections derived from one atomic snapshot.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AgentSpendSurfaces {
    mobile: ShellAgentSpend,
    desktop: ShellAgentSpend,
}

impl AgentSpendSurfaces {
    #[must_use]
    pub const fn mobile(&self) -> &ShellAgentSpend {
        &self.mobile
    }

    #[must_use]
    pub const fn desktop(&self) -> &ShellAgentSpend {
        &self.desktop
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
struct StoredSpend {
    version: u8,
    principal: String,
    agent_id: [u8; 32],
    account: [u8; 32],
    budget: [u8; 32],
    asset: [u8; 32],
    period_start: u64,
    period_end: u64,
    spent: u128,
    receipt_spent: u128,
    limit: u128,
    remaining: u128,
    receipt_count: usize,
    head: u64,
    rung: u8,
    evidence: Vec<[u8; 32]>,
}

/// Principal-scoped spend service which refuses reads until the daemon can
/// supply an opaque canonical protocol budget record and proof.
pub struct SpendReconciliation {
    profile: SpendProfile,
    observed: Option<StoredSpend>,
}

impl SpendReconciliation {
    /// Binds the service to one human principal and one managed-agent account.
    ///
    /// # Errors
    ///
    /// Rejects zero protocol identifiers.
    pub fn new(profile: SpendProfile) -> Result<Self, SpendError> {
        profile.validate()?;
        Ok(Self {
            profile,
            observed: None,
        })
    }

    /// Reopens the last verified projection from the owner's durable store.
    /// # Errors
    /// Returns an error if the requested data cannot be read or validated.
    pub fn open(scope: &PrincipalScope<'_>, profile: SpendProfile) -> Result<Self, SpendError> {
        let mut service = Self::new(profile)?;
        service.check_principal(scope.principal())?;
        if let Some(row) = scope.get(Table::Cache, &service.row()?) {
            let observed: StoredSpend =
                serde_json::from_slice(row.bytes()).map_err(|_| SpendError::EvidenceConflict)?;
            service.validate_observation(&observed)?;
            service.observed = Some(observed);
        }
        Ok(service)
    }

    /// Adopts canonical state only after the production budget and receipt verifiers succeed.
    /// Failed observations do not replace the retained verified evidence.
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    pub fn observe(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        protocol: &ProtocolBudgetState,
        receipts: &[SpendReceiptEvidence],
        verifier: &EvidenceAuthority,
        observed_at: u64,
    ) -> Result<AgentSpendView, SpendError> {
        self.check_principal(scope.principal())?;
        let verified_state = verifier
            .verify_state(&protocol.evidence)
            .map_err(|_| SpendError::UnverifiedEvidence)?;
        let record = ProtocolBudgetRecord::decode(verified_state.canonical_state())
            .map_err(|_| SpendError::EvidenceConflict)?;
        if record.budget_id != self.profile.budget_id
            || record.asset_id != self.profile.asset
            || record.source_account.unwrap_or(record.budget_account) != self.profile.agent_account
            || record.owner == [0; 32]
            || record.closed
            || record.revoked
            || observed_at < record.period_start
            || observed_at >= record.window_end_sequence()
            || observed_at >= record.expiry
        {
            return Err(SpendError::EvidenceConflict);
        }
        let mut total = 0_u128;
        let mut evidence = vec![Sha256::digest(verified_state.canonical_state()).into()];
        let mut rung = if verified_state
            .level()
            .compare(layerx_types::verify::VerificationLevel::COMMITTED)
            .is_ge()
        {
            Rung::Committed
        } else {
            Rung::Receipted
        };
        for entry in receipts {
            let receipt = verifier
                .verify_receipt(&entry.evidence)
                .map_err(|_| SpendError::UnverifiedEvidence)?;
            let decoded = layerx_wire::receipt::decode(receipt.canonical_receipt())
                .map_err(|_| SpendError::UnverifiedEvidence)?;
            let facts = decoded.protocol().ok_or(SpendError::UnverifiedEvidence)?;
            if receipt.activity_id() != entry.expected_activity_id
                || receipt.result_code() != 0
                || facts.asset() != self.profile.asset
                || facts.from() != self.profile.agent_account
                || facts.timestamp() < record.period_start
                || facts.timestamp() >= record.window_end_sequence()
                || receipt.global_sequence() > verified_state.observed_head_sequence()
            {
                return Err(SpendError::EvidenceConflict);
            }
            total = total
                .checked_add(receipt.amount())
                .ok_or(SpendError::EvidenceConflict)?;
            evidence.push(receipt.receipt_ref());
            if !receipt
                .level()
                .compare(layerx_types::verify::VerificationLevel::COMMITTED)
                .is_ge()
            {
                rung = Rung::Receipted;
            }
        }
        let mut accounting = LocalAccounting {
            consumed: total,
            window_start_sequence: record.period_start,
            last_receipt: None,
        };
        let reconciled = budget::reconcile(&mut accounting, protocol, receipts, verifier)
            .map_err(|_| SpendError::UnverifiedEvidence)?;
        if self.observed.as_ref().is_some_and(|previous| {
            reconciled.observed_head_sequence() < previous.head
                || record.period_start < previous.period_start
        }) {
            return Err(SpendError::EvidenceConflict);
        }
        let observed = StoredSpend {
            version: 1,
            principal: self.profile.principal.as_str().to_owned(),
            agent_id: self.profile.agent_id,
            account: self.profile.agent_account,
            budget: self.profile.budget_id,
            asset: self.profile.asset,
            period_start: record.period_start,
            period_end: record.window_end_sequence(),
            spent: accounting.consumed,
            receipt_spent: total,
            limit: record.per_period_limit,
            remaining: reconciled.remaining(),
            receipt_count: receipts.len(),
            head: reconciled.observed_head_sequence(),
            rung: rung.wire_rank(),
            evidence,
        };
        self.validate_observation(&observed)?;
        let encoded = serde_json::to_vec(&observed).map_err(|_| SpendError::EvidenceConflict)?;
        scope.put(Table::Cache, self.row()?, observed_at, encoded)?;
        self.observed = Some(observed);
        self.reconcile(scope.principal())
    }

    /// Projects the opaque proof issued by the production committed-chain budget verifier.
    /// # Errors
    /// Returns an error if validation or a required operation fails.
    pub fn observe_budget_proof(
        &mut self,
        scope: &mut PrincipalScope<'_>,
        proof: &layerx_agentd::budget::VerifiedBudgetProof,
        receipts: &[SpendReceiptEvidence],
        verifier: &EvidenceAuthority,
        observed_at: u64,
    ) -> Result<AgentSpendView, SpendError> {
        self.check_principal(scope.principal())?;
        let binding: serde_json::Value = serde_json::from_slice(proof.canonical_export_bytes())
            .map_err(|_| SpendError::EvidenceConflict)?;
        if binding["principal"].as_str() != Some(scope.principal().as_str())
            || binding["tenant"].as_str() != Some(scope.tenant().as_str())
        {
            return Err(SpendError::WrongPrincipal);
        }
        let protocol = ProtocolBudgetState::decode_get_budget(
            proof.budget_id(),
            proof.committed_height(),
            proof.get_budget_result(),
        )
        .map_err(|_| SpendError::EvidenceConflict)?;
        let record = protocol.record();
        if proof.budget_id() != self.profile.budget_id
            || proof.asset() != self.profile.asset
            || proof.source_account() != self.profile.agent_account
            || record.closed
            || record.revoked
            || observed_at < record.period_start
            || observed_at >= record.window_end_sequence()
            || observed_at >= record.expiry
            || !proof
                .verification()
                .compare(layerx_types::verify::VerificationLevel::COMMITTED)
                .is_ge()
            || proof.remaining() != record.remaining()
        {
            return Err(SpendError::EvidenceConflict);
        }
        let mut total = 0_u128;
        let mut evidence = vec![proof.digest()];
        let mut seen = std::collections::BTreeSet::new();
        let mut rung = Rung::Committed;
        for entry in receipts {
            let receipt = verifier
                .verify_receipt(&entry.evidence)
                .map_err(|_| SpendError::UnverifiedEvidence)?;
            let decoded = layerx_wire::receipt::decode(receipt.canonical_receipt())
                .map_err(|_| SpendError::UnverifiedEvidence)?;
            let facts = decoded.protocol().ok_or(SpendError::UnverifiedEvidence)?;
            if receipt.activity_id() != entry.expected_activity_id
                || receipt.result_code() != 0
                || facts.asset() != self.profile.asset
                || facts.from() != self.profile.agent_account
                || facts.timestamp() < record.period_start
                || facts.timestamp() >= record.window_end_sequence()
                || receipt.global_sequence() > proof.committed_height()
                || !seen.insert(receipt.activity_id())
            {
                return Err(SpendError::EvidenceConflict);
            }
            total = total
                .checked_add(receipt.amount())
                .ok_or(SpendError::EvidenceConflict)?;
            evidence.push(receipt.receipt_ref());
            if !receipt
                .level()
                .compare(layerx_types::verify::VerificationLevel::COMMITTED)
                .is_ge()
            {
                rung = Rung::Receipted;
            }
        }
        if self.observed.as_ref().is_some_and(|previous| {
            proof.observed_head_sequence() < previous.head
                || record.period_start < previous.period_start
        }) {
            return Err(SpendError::EvidenceConflict);
        }
        let observed = StoredSpend {
            version: 1,
            principal: self.profile.principal.as_str().to_owned(),
            agent_id: self.profile.agent_id,
            account: self.profile.agent_account,
            budget: self.profile.budget_id,
            asset: self.profile.asset,
            period_start: record.period_start,
            period_end: record.window_end_sequence(),
            spent: record.spent_this_period,
            receipt_spent: total,
            limit: record.per_period_limit,
            remaining: proof.remaining(),
            receipt_count: receipts.len(),
            head: proof.observed_head_sequence(),
            rung: rung.wire_rank(),
            evidence,
        };
        self.validate_observation(&observed)?;
        let bytes = serde_json::to_vec(&observed).map_err(|_| SpendError::EvidenceConflict)?;
        scope.put(Table::Cache, self.row()?, observed_at, bytes)?;
        self.observed = Some(observed);
        self.reconcile(scope.principal())
    }

    fn check_principal(&self, principal: &PrincipalId) -> Result<(), SpendError> {
        if principal == &self.profile.principal {
            Ok(())
        } else {
            Err(SpendError::WrongPrincipal)
        }
    }

    fn row(&self) -> Result<RowKey, StoreError> {
        let suffix: String = crate::hex::lower(&self.profile.agent_id);
        RowKey::new(format!("agent-spend-{suffix}"))
    }

    fn validate_observation(&self, value: &StoredSpend) -> Result<(), SpendError> {
        if value.version != 1
            || value.principal != self.profile.principal.as_str()
            || value.agent_id != self.profile.agent_id
            || value.account != self.profile.agent_account
            || value.budget != self.profile.budget_id
            || value.asset != self.profile.asset
            || value.head == 0
            || !(1..=2).contains(&value.rung)
            || value.period_start >= value.period_end
            || value.spent > value.limit
            || value.remaining != value.limit - value.spent
            || value.evidence.len() != value.receipt_count + 1
            || value.evidence.contains(&[0; 32])
        {
            return Err(SpendError::EvidenceConflict);
        }
        Ok(())
    }

    /// Returns one shell projection. The requesting principal must be the
    /// human owner bound when the service was constructed.
    ///
    /// # Errors
    ///
    /// Refuses cross-principal access and otherwise returns
    /// `ProtocolBudgetStateUnavailable` until the core defines the canonical
    /// budget state record, state key, and proof path.
    pub fn for_shell(
        &self,
        principal: &PrincipalId,
        shell: AgentShell,
    ) -> Result<ShellAgentSpend, SpendError> {
        Ok(ShellAgentSpend {
            shell,
            spend: self.reconcile(principal)?,
        })
    }

    /// Returns both shell projections only after canonical budget-state
    /// integration becomes available.
    ///
    /// # Errors
    ///
    /// Returns the same typed failures as [`Self::for_shell`].
    pub fn both_shells(&self, principal: &PrincipalId) -> Result<AgentSpendSurfaces, SpendError> {
        let spend = self.reconcile(principal)?;
        Ok(AgentSpendSurfaces {
            mobile: ShellAgentSpend {
                shell: AgentShell::Mobile,
                spend: spend.clone(),
            },
            desktop: ShellAgentSpend {
                shell: AgentShell::Desktop,
                spend,
            },
        })
    }

    fn reconcile(&self, principal: &PrincipalId) -> Result<AgentSpendView, SpendError> {
        self.check_principal(principal)?;
        let value = self
            .observed
            .as_ref()
            .ok_or(SpendError::ProtocolBudgetStateUnavailable)?;
        self.validate_observation(value)?;
        let reconciliation = match value.spent.cmp(&value.receipt_spent) {
            std::cmp::Ordering::Equal => SpendReconciliationStatus::InSync,
            std::cmp::Ordering::Greater => SpendReconciliationStatus::ProtocolAdopted {
                direction: ReconciliationDirection::ProtocolHigher,
                difference: value.spent - value.receipt_spent,
            },
            std::cmp::Ordering::Less => SpendReconciliationStatus::ProtocolAdopted {
                direction: ReconciliationDirection::ProtocolLower,
                difference: value.receipt_spent - value.spent,
            },
        };
        let differs = reconciliation != SpendReconciliationStatus::InSync;
        Ok(AgentSpendView {
            agent_id: value.agent_id,
            period_start: value.period_start,
            period_end: value.period_end,
            spent: value.spent,
            receipt_spent: value.receipt_spent,
            limit: value.limit,
            remaining: value.remaining,
            receipt_count: value.receipt_count,
            verification_level: if value.rung == 2 {
                Rung::Committed
            } else {
                Rung::Receipted
            },
            observed_head_sequence: value.head,
            reconciliation,
            reconciliation_copy_key: differs.then_some(RECONCILIATION_COPY_KEY),
            reconciliation_explanation: differs.then_some(RECONCILIATION_EXPLANATION),
            evidence_digests: value.evidence.clone(),
        })
    }
}

/// Typed spend read failure. No failure carries a partially trusted view.
#[derive(Debug)]
pub enum SpendError {
    Store(StoreError),
    UnverifiedEvidence,
    EvidenceConflict,
    InvalidProfile,
    WrongPrincipal,
    ProtocolBudgetStateUnavailable,
}

impl Display for SpendError {
    fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Store(error) => write!(formatter, "spend store failure: {error}"),
            Self::UnverifiedEvidence => formatter.write_str("spend evidence is unverified"),
            Self::EvidenceConflict => {
                formatter.write_str("spend evidence does not match the managed agent")
            }
            Self::InvalidProfile => formatter.write_str("managed-agent spend profile is invalid"),
            Self::WrongPrincipal => formatter.write_str("agent spend belongs to another principal"),
            Self::ProtocolBudgetStateUnavailable => formatter.write_str(
                "canonical protocol budget state is unavailable; managed-agent spend is blocked",
            ),
        }
    }
}

impl std::error::Error for SpendError {}

impl From<StoreError> for SpendError {
    fn from(error: StoreError) -> Self {
        Self::Store(error)
    }
}
