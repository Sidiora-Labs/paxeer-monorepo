//! Policy validation, immutable request snapshots and retained version history.

use crate::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::str;
use std::sync::{Arc, Mutex};

use super::{Decision, PolicySet, Rule, RuleConstraints, RuleEffect};

pub const MAX_POLICY_SOURCE_BYTES: usize = 1_048_576;
const MAX_POLICY_RULES: usize = 4_096;
const MAX_POLICY_LINE_BYTES: usize = 4_096;
const APPROVAL_SOURCE_V2_HEADER: &[u8] = b"schema=layerx.policy.source.v2\n";

/// Policy validation refusal taxonomy.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PolicyValidationError {
    EmptyVersion,
    EmptyRuleId,
    DuplicateRuleId(String),
    EmptyPurpose,
    InvalidSequenceWindow,
    ZeroStepLimit,
    StepLimitTooSmall,
    VersionAlreadyRetained(String),
    History(PolicyHistoryError),
}

/// Bounded source-loader failures.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PolicySourceError {
    TooLarge,
    InvalidUtf8,
    LineTooLarge,
    TooManyRules,
    InvalidDeclaration,
    InvalidInteger,
    InvalidEffect,
    Validation(PolicyValidationError),
}

/// Successful activation evidence.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Activation {
    pub previous_version: String,
    pub active_version: String,
    pub generation: u64,
}

/// Immutable policy selection captured when a request is received.
#[derive(Clone, Debug)]
pub struct PolicySnapshot {
    retained: Arc<RetainedPolicy>,
    generation: u64,
}

#[derive(Debug)]
struct RetainedPolicy {
    policy: PolicySet,
    source: Option<Arc<[u8]>>,
    generation: u64,
}

impl PolicySnapshot {
    #[must_use]
    pub fn policy(&self) -> &PolicySet {
        &self.retained.policy
    }

    #[must_use]
    pub fn version(&self) -> &str {
        &self.retained.policy.version
    }

    #[must_use]
    pub fn source(&self) -> Option<&[u8]> {
        self.retained.source.as_deref()
    }

    #[must_use]
    pub const fn generation(&self) -> u64 {
        self.generation
    }
}

/// Retrievable decision record retained without lossy projection.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PolicyAuditEntry {
    pub request_id: [u8; 32],
    pub decision: Decision,
    pub tenant: Option<TenantId>,
    pub generation: u64,
    pub source_digest: Option<[u8; 32]>,
}

/// Active policy plus all versions and decisions retained for reconstruction.
#[derive(Clone)]
pub struct PolicyRegistry {
    active_version: String,
    generation: u64,
    versions: BTreeMap<String, Arc<RetainedPolicy>>,
    audit: BTreeMap<[u8; 32], PolicyAuditEntry>,
    quotas: PolicyRetentionQuotas,
    durable: Option<DurableHistory>,
}

impl PolicyRegistry {
    /// Seeds the registry with a validated initial policy at generation 1.
    ///
    /// # Errors
    ///
    /// Propagates `validate_policy`: `EmptyVersion`, `ZeroStepLimit`, `StepLimitTooSmall`,
    /// `EmptyRuleId`, `DuplicateRuleId`, `EmptyPurpose` or `InvalidSequenceWindow`.
    pub fn new(initial: PolicySet) -> Result<Self, PolicyValidationError> {
        validate_policy(&initial)?;
        Ok(Self::initial(initial, None))
    }

    /// # Errors
    /// Returns an error if the protected policy source cannot be read, validated or activated.
    pub fn from_source(source: &[u8]) -> Result<Self, PolicySourceError> {
        let policy = load_policy_source(source)?;
        Ok(Self::initial(policy, Some(Arc::from(source))))
    }

    fn initial(initial: PolicySet, source: Option<Arc<[u8]>>) -> Self {
        let active_version = initial.version.clone();
        let versions = BTreeMap::from([(
            active_version.clone(),
            Arc::new(RetainedPolicy {
                policy: initial,
                source,
                generation: 1,
            }),
        )]);
        Self {
            active_version,
            generation: 1,
            versions,
            audit: BTreeMap::new(),
            quotas: PolicyRetentionQuotas::default(),
            durable: None,
        }
    }

    /// Captures the policy that applies before request processing begins.
    #[must_use]
    pub fn begin_request(&self) -> PolicySnapshot {
        let retained = self
            .versions
            .get(&self.active_version)
            .cloned()
            .unwrap_or_else(|| unreachable!("active policy is retained"));
        PolicySnapshot {
            retained,
            generation: self.generation,
        }
    }

    #[must_use]
    pub fn active_version(&self) -> &str {
        &self.active_version
    }

    #[must_use]
    pub fn retained(&self, version: &str) -> Option<&PolicySet> {
        self.versions.get(version).map(|retained| &retained.policy)
    }

    /// # Errors
    /// Returns an error if policy history is corrupt, unavailable or cannot retain the decision.
    pub fn record_decision(
        &mut self,
        request_id: [u8; 32],
        decision: Decision,
    ) -> Result<(), PolicyHistoryError> {
        let retained = self
            .versions
            .get(&decision.policy_version)
            .ok_or(PolicyHistoryError::UnknownVersion)?;
        let entry = PolicyAuditEntry {
            request_id,
            decision,
            tenant: self
                .durable
                .as_ref()
                .map(|history| history.key.tenant().clone()),
            generation: retained.generation,
            source_digest: retained.source.as_deref().map(source_digest),
        };
        if let Some(previous) = self.audit.get(&request_id) {
            return if previous == &entry {
                Ok(())
            } else {
                Err(PolicyHistoryError::DecisionConflict)
            };
        }
        if self.audit.len() >= self.quotas.decisions {
            return Err(PolicyHistoryError::DecisionQuota);
        }
        let mut next = self.clone();
        next.audit.insert(request_id, entry);
        next.persist()?;
        *self = next;
        Ok(())
    }

    #[must_use]
    pub fn audit_entry(&self, request_id: [u8; 32]) -> Option<&PolicyAuditEntry> {
        self.audit.get(&request_id)
    }
}

pub(crate) fn validate_policy(policy: &PolicySet) -> Result<(), PolicyValidationError> {
    if policy.version.is_empty() {
        return Err(PolicyValidationError::EmptyVersion);
    }
    if policy.evaluation_step_limit == 0 {
        return Err(PolicyValidationError::ZeroStepLimit);
    }
    let rule_count = u64::try_from(policy.rules.len()).unwrap_or(u64::MAX);
    if rule_count > policy.evaluation_step_limit {
        return Err(PolicyValidationError::StepLimitTooSmall);
    }
    let mut ids = BTreeSet::new();
    for rule in &policy.rules {
        if rule.id.is_empty() {
            return Err(PolicyValidationError::EmptyRuleId);
        }
        if !ids.insert(rule.id.clone()) {
            return Err(PolicyValidationError::DuplicateRuleId(rule.id.clone()));
        }
        if rule.constraints.purposes.iter().any(String::is_empty) {
            return Err(PolicyValidationError::EmptyPurpose);
        }
        if rule
            .constraints
            .sequence_window
            .is_some_and(|window| window.first > window.last)
        {
            return Err(PolicyValidationError::InvalidSequenceWindow);
        }
    }
    Ok(())
}

pub(crate) fn activate_policy(
    registry: &mut PolicyRegistry,
    policy: PolicySet,
) -> Result<Activation, PolicyValidationError> {
    activate_retained(registry, policy, None)
}

pub(crate) fn activate_policy_source(
    registry: &mut PolicyRegistry,
    source: &[u8],
) -> Result<Activation, PolicySourceError> {
    let policy = load_policy_source(source)?;
    activate_retained(registry, policy, Some(Arc::from(source)))
        .map_err(PolicySourceError::Validation)
}

fn activate_retained(
    registry: &mut PolicyRegistry,
    policy: PolicySet,
    source: Option<Arc<[u8]>>,
) -> Result<Activation, PolicyValidationError> {
    validate_policy(&policy)?;
    if registry.versions.contains_key(&policy.version) {
        return Err(PolicyValidationError::VersionAlreadyRetained(
            policy.version,
        ));
    }
    if registry.versions.len() >= registry.quotas.versions {
        return Err(PolicyValidationError::History(
            PolicyHistoryError::VersionQuota,
        ));
    }
    if registry.durable.is_some() && source.is_none() {
        return Err(PolicyValidationError::History(
            PolicyHistoryError::SourceRequired,
        ));
    }
    let previous_version = registry.active_version.clone();
    let active_version = policy.version.clone();
    let generation = registry
        .generation
        .checked_add(1)
        .ok_or(PolicyValidationError::History(
            PolicyHistoryError::GenerationOverflow,
        ))?;
    let mut next = registry.clone();
    next.versions.insert(
        active_version.clone(),
        Arc::new(RetainedPolicy {
            policy,
            source,
            generation,
        }),
    );
    next.active_version.clone_from(&active_version);
    next.generation = generation;
    next.persist().map_err(PolicyValidationError::History)?;
    *registry = next;
    Ok(Activation {
        previous_version,
        active_version,
        generation,
    })
}

pub const MAX_POLICY_HISTORY_BYTES: usize = 16 * 1024 * 1024;
pub const MAX_RETAINED_POLICY_VERSIONS: usize = 64;
pub const MAX_RETAINED_POLICY_DECISIONS: usize = 4_096;

#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PolicyRetentionQuotas {
    pub versions: usize,
    pub decisions: usize,
    pub bytes: usize,
}

impl Default for PolicyRetentionQuotas {
    fn default() -> Self {
        Self {
            versions: MAX_RETAINED_POLICY_VERSIONS,
            decisions: MAX_RETAINED_POLICY_DECISIONS,
            bytes: MAX_POLICY_HISTORY_BYTES,
        }
    }
}

impl PolicyRetentionQuotas {
    fn validate(self) -> Result<(), PolicyHistoryError> {
        if self.versions == 0
            || self.versions > MAX_RETAINED_POLICY_VERSIONS
            || self.decisions == 0
            || self.decisions > MAX_RETAINED_POLICY_DECISIONS
            || self.bytes == 0
            || self.bytes > MAX_POLICY_HISTORY_BYTES
        {
            return Err(PolicyHistoryError::InvalidQuotas);
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PolicyHistoryError {
    Unavailable,
    CorruptRecord,
    SourceRequired,
    Source(Box<PolicySourceError>),
    UnknownVersion,
    TenantMismatch,
    PolicyMismatch,
    DecisionConflict,
    StaleRegistry,
    VersionQuota,
    DecisionQuota,
    ByteQuota,
    InvalidQuotas,
    GenerationOverflow,
}

#[derive(Clone)]
struct DurableHistory {
    store: Arc<Mutex<Store>>,
    key: TenantKey,
    previous: Option<Vec<u8>>,
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct HistoryRecord {
    schema: u16,
    tenant: String,
    quotas: PolicyRetentionQuotas,
    active_version: String,
    generation: u64,
    versions: Vec<SourceRecord>,
    decisions: Vec<DecisionRecord>,
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct SourceRecord {
    source: Vec<u8>,
    digest: [u8; 32],
    generation: u64,
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct DecisionRecord {
    request_id: [u8; 32],
    generation: u64,
    source_digest: [u8; 32],
    explanation: Vec<u8>,
}

fn source_digest(source: &[u8]) -> [u8; 32] {
    Sha256::digest(source).into()
}

impl PolicyRegistry {
    /// # Errors
    /// Returns an error if policy history is corrupt, unavailable or cannot retain the decision.
    pub fn open(
        store: Arc<Mutex<Store>>,
        tenant: TenantId,
        source: &[u8],
        quotas: PolicyRetentionQuotas,
    ) -> Result<Self, PolicyHistoryError> {
        quotas.validate()?;
        let key = TenantKey::new(tenant, ObjectKind::Policy, b"policy-history-v1".to_vec())
            .map_err(|_| PolicyHistoryError::CorruptRecord)?;
        let previous = {
            let store = store.lock().map_err(|_| PolicyHistoryError::Unavailable)?;
            store
                .get(&key)
                .map(|value| {
                    if value.class() != StorageClass::LocalOnly {
                        return Err(PolicyHistoryError::CorruptRecord);
                    }
                    Ok(value.bytes().to_vec())
                })
                .transpose()?
        };
        let mut registry = if let Some(bytes) = &previous {
            Self::decode_history(bytes, key.tenant(), quotas)?
        } else {
            let mut registry = Self::from_source(source)
                .map_err(|error| PolicyHistoryError::Source(Box::new(error)))?;
            registry.quotas = quotas;
            registry
        };
        registry.durable = Some(DurableHistory {
            store,
            key,
            previous,
        });
        if registry.begin_request().source() != Some(source) {
            activate_policy_source(&mut registry, source)
                .map_err(|error| PolicyHistoryError::Source(Box::new(error)))?;
        } else if registry
            .durable
            .as_ref()
            .is_some_and(|history| history.previous.is_none())
        {
            registry.persist()?;
        }
        Ok(registry)
    }

    #[must_use]
    pub fn tenant(&self) -> Option<&TenantId> {
        self.durable.as_ref().map(|history| history.key.tenant())
    }

    #[must_use]
    pub fn retained_snapshot(&self, version: &str) -> Option<PolicySnapshot> {
        self.versions.get(version).map(|retained| PolicySnapshot {
            retained: Arc::clone(retained),
            generation: retained.generation,
        })
    }

    #[must_use]
    pub fn reconstruct_decision(
        &self,
        request_id: [u8; 32],
    ) -> Option<(&PolicyAuditEntry, PolicySnapshot)> {
        let entry = self.audit.get(&request_id)?;
        Some((
            entry,
            self.retained_snapshot(&entry.decision.policy_version)?,
        ))
    }

    fn encode_history(&self, tenant: &TenantId) -> Result<Vec<u8>, PolicyHistoryError> {
        let versions = self
            .versions
            .values()
            .map(|retained| {
                let source = retained
                    .source
                    .as_deref()
                    .ok_or(PolicyHistoryError::SourceRequired)?;
                Ok(SourceRecord {
                    source: source.to_vec(),
                    digest: source_digest(source),
                    generation: retained.generation,
                })
            })
            .collect::<Result<Vec<_>, PolicyHistoryError>>()?;
        let decisions = self
            .audit
            .values()
            .map(|entry| {
                Ok(DecisionRecord {
                    request_id: entry.request_id,
                    generation: entry.generation,
                    source_digest: entry
                        .source_digest
                        .ok_or(PolicyHistoryError::SourceRequired)?,
                    explanation: super::explain(&entry.decision, super::EvaluationMode::Live)
                        .machine_bytes(),
                })
            })
            .collect::<Result<Vec<_>, PolicyHistoryError>>()?;
        let bytes = serde_json::to_vec(&HistoryRecord {
            schema: 1,
            tenant: tenant.as_str().to_owned(),
            quotas: self.quotas,
            active_version: self.active_version.clone(),
            generation: self.generation,
            versions,
            decisions,
        })
        .map_err(|_| PolicyHistoryError::CorruptRecord)?;
        if bytes.len() > self.quotas.bytes {
            return Err(PolicyHistoryError::ByteQuota);
        }
        Ok(bytes)
    }

    fn persist(&mut self) -> Result<(), PolicyHistoryError> {
        let Some(history) = &self.durable else {
            return Ok(());
        };
        let bytes = self.encode_history(history.key.tenant())?;
        let mut store = history
            .store
            .lock()
            .map_err(|_| PolicyHistoryError::Unavailable)?;
        let current = store.get(&history.key);
        if current.is_some_and(|value| value.class() != StorageClass::LocalOnly)
            || current.map(crate::store::StoredValue::bytes) != history.previous.as_deref()
        {
            return Err(PolicyHistoryError::StaleRegistry);
        }
        store
            .put_local(history.key.clone(), bytes.clone())
            .map_err(|_| PolicyHistoryError::Unavailable)?;
        drop(store);
        if let Some(history) = &mut self.durable {
            history.previous = Some(bytes);
        }
        Ok(())
    }

    fn decode_history(
        bytes: &[u8],
        tenant: &TenantId,
        quotas: PolicyRetentionQuotas,
    ) -> Result<Self, PolicyHistoryError> {
        if bytes.len() > quotas.bytes {
            return Err(PolicyHistoryError::ByteQuota);
        }
        let record: HistoryRecord =
            serde_json::from_slice(bytes).map_err(|_| PolicyHistoryError::CorruptRecord)?;
        if record.schema != 1
            || record.tenant != tenant.as_str()
            || record.quotas != quotas
            || record.versions.is_empty()
            || record.versions.len() > quotas.versions
            || record.decisions.len() > quotas.decisions
            || record.generation
                != u64::try_from(record.versions.len())
                    .map_err(|_| PolicyHistoryError::CorruptRecord)?
        {
            return Err(PolicyHistoryError::CorruptRecord);
        }
        let mut versions = BTreeMap::new();
        let mut generations = BTreeSet::new();
        for version in record.versions {
            let policy = load_policy_source(&version.source)
                .map_err(|_| PolicyHistoryError::CorruptRecord)?;
            if source_digest(&version.source) != version.digest
                || version.generation == 0
                || version.generation > record.generation
                || !generations.insert(version.generation)
                || versions
                    .insert(
                        policy.version.clone(),
                        Arc::new(RetainedPolicy {
                            policy,
                            source: Some(Arc::from(version.source)),
                            generation: version.generation,
                        }),
                    )
                    .is_some()
            {
                return Err(PolicyHistoryError::CorruptRecord);
            }
        }
        if versions
            .get(&record.active_version)
            .is_none_or(|policy| policy.generation != record.generation)
        {
            return Err(PolicyHistoryError::CorruptRecord);
        }
        let mut audit = BTreeMap::new();
        for entry in record.decisions {
            let explanation = super::Explanation::from_machine_bytes(&entry.explanation)
                .map_err(|_| PolicyHistoryError::CorruptRecord)?;
            if explanation.mode != super::EvaluationMode::Live {
                return Err(PolicyHistoryError::CorruptRecord);
            }
            let retained = versions
                .get(&explanation.policy_version)
                .ok_or(PolicyHistoryError::CorruptRecord)?;
            if retained.generation != entry.generation
                || retained.source.as_deref().map(source_digest) != Some(entry.source_digest)
            {
                return Err(PolicyHistoryError::CorruptRecord);
            }
            let decision = Decision {
                outcome: explanation.outcome,
                policy_version: explanation.policy_version,
                matched_rules: explanation.matched_rules,
                deciding_rule: explanation.deciding_rule,
                reason: explanation.reason,
            };
            if audit
                .insert(
                    entry.request_id,
                    PolicyAuditEntry {
                        request_id: entry.request_id,
                        decision,
                        tenant: Some(tenant.clone()),
                        generation: entry.generation,
                        source_digest: Some(entry.source_digest),
                    },
                )
                .is_some()
            {
                return Err(PolicyHistoryError::CorruptRecord);
            }
        }
        let registry = Self {
            active_version: record.active_version,
            generation: record.generation,
            versions,
            audit,
            quotas,
            durable: None,
        };
        if registry.encode_history(tenant)? != bytes {
            return Err(PolicyHistoryError::CorruptRecord);
        }
        Ok(registry)
    }
}

/// Loads a bounded line-oriented policy source without ambient state.
///
/// # Errors
///
/// Returns `TooLarge`, `InvalidUtf8`, `LineTooLarge` or `TooManyRules` on the bounds,
/// `InvalidDeclaration` for an unknown, repeated or absent `version=`/`steps=` or a
/// comma-less `rule=`, `InvalidInteger`, `InvalidEffect`, or `Validation` on the set.
pub fn load_policy_source(source: &[u8]) -> Result<PolicySet, PolicySourceError> {
    if source.starts_with(APPROVAL_SOURCE_V2_HEADER) {
        return load_policy_source_v2(source);
    }
    if source.len() > MAX_POLICY_SOURCE_BYTES {
        return Err(PolicySourceError::TooLarge);
    }
    let text = str::from_utf8(source).map_err(|_| PolicySourceError::InvalidUtf8)?;
    let mut version = None;
    let mut evaluation_step_limit = None;
    let mut rules = Vec::new();
    for line in text.lines() {
        if line.len() > MAX_POLICY_LINE_BYTES {
            return Err(PolicySourceError::LineTooLarge);
        }
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if let Some(value) = line.strip_prefix("version=") {
            if version.replace(value.to_owned()).is_some() {
                return Err(PolicySourceError::InvalidDeclaration);
            }
        } else if let Some(value) = line.strip_prefix("steps=") {
            let parsed = value
                .parse::<u64>()
                .map_err(|_| PolicySourceError::InvalidInteger)?;
            if evaluation_step_limit.replace(parsed).is_some() {
                return Err(PolicySourceError::InvalidDeclaration);
            }
        } else if let Some(value) = line.strip_prefix("rule=") {
            if rules.len() >= MAX_POLICY_RULES {
                return Err(PolicySourceError::TooManyRules);
            }
            let (id, effect) = value
                .split_once(',')
                .ok_or(PolicySourceError::InvalidDeclaration)?;
            let effect = match effect {
                "permit" => RuleEffect::Permit,
                "deny" => RuleEffect::Deny,
                _ => return Err(PolicySourceError::InvalidEffect),
            };
            rules.push(Rule {
                id: id.to_owned(),
                effect,
                constraints: RuleConstraints::default(),
            });
        } else {
            return Err(PolicySourceError::InvalidDeclaration);
        }
    }
    let policy = PolicySet {
        version: version.ok_or(PolicySourceError::InvalidDeclaration)?,
        rules,
        evaluation_step_limit: evaluation_step_limit
            .ok_or(PolicySourceError::InvalidDeclaration)?,
    };
    validate_policy(&policy).map_err(PolicySourceError::Validation)?;
    Ok(policy)
}

fn load_policy_source_v2(source: &[u8]) -> Result<PolicySet, PolicySourceError> {
    if source.len() > MAX_POLICY_SOURCE_BYTES {
        return Err(PolicySourceError::TooLarge);
    }
    let source = source
        .strip_prefix(APPROVAL_SOURCE_V2_HEADER)
        .ok_or(PolicySourceError::InvalidDeclaration)?;
    let text = str::from_utf8(source).map_err(|_| PolicySourceError::InvalidUtf8)?;
    let mut version = None;
    let mut evaluation_step_limit = None;
    let mut rules = Vec::new();
    for line in text.lines() {
        if line.len() > MAX_POLICY_LINE_BYTES {
            return Err(PolicySourceError::LineTooLarge);
        }
        let line = line.trim();
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        if let Some(value) = line.strip_prefix("version=") {
            if version.replace(value.to_owned()).is_some() {
                return Err(PolicySourceError::InvalidDeclaration);
            }
        } else if let Some(value) = line.strip_prefix("steps=") {
            let parsed = value
                .parse::<u64>()
                .map_err(|_| PolicySourceError::InvalidInteger)?;
            if evaluation_step_limit.replace(parsed).is_some() {
                return Err(PolicySourceError::InvalidDeclaration);
            }
        } else if let Some(value) = line.strip_prefix("rule=") {
            if rules.len() >= MAX_POLICY_RULES {
                return Err(PolicySourceError::TooManyRules);
            }
            let mut fields = value.split(',');
            let id = fields.next().ok_or(PolicySourceError::InvalidDeclaration)?;
            let effect = match fields.next().ok_or(PolicySourceError::InvalidDeclaration)? {
                "permit" => RuleEffect::Permit,
                "deny" => RuleEffect::Deny,
                _ => return Err(PolicySourceError::InvalidEffect),
            };
            let required_approval =
                match fields.next().ok_or(PolicySourceError::InvalidDeclaration)? {
                    "required_approval" => true,
                    "no_approval" => false,
                    _ => return Err(PolicySourceError::InvalidDeclaration),
                };
            if fields.next().is_some() || (effect == RuleEffect::Deny && required_approval) {
                return Err(PolicySourceError::InvalidDeclaration);
            }
            rules.push(Rule {
                id: id.to_owned(),
                effect,
                constraints: RuleConstraints {
                    required_approval,
                    ..RuleConstraints::default()
                },
            });
        } else {
            return Err(PolicySourceError::InvalidDeclaration);
        }
    }
    let policy = PolicySet {
        version: version.ok_or(PolicySourceError::InvalidDeclaration)?,
        rules,
        evaluation_step_limit: evaluation_step_limit
            .ok_or(PolicySourceError::InvalidDeclaration)?,
    };
    validate_policy(&policy).map_err(PolicySourceError::Validation)?;
    Ok(policy)
}

#[cfg(test)]
mod approval_source_tests {
    use super::*;

    #[test]
    fn approval_source_v2_carries_required_approval_and_exact_source() {
        let source = b"schema=layerx.policy.source.v2\nversion=approval-v2\nsteps=4\nrule=review,permit,required_approval\nrule=ordinary,permit,no_approval\nrule=blocked,deny,no_approval\n";
        let registry = PolicyRegistry::from_source(source).expect("real configured policy");
        let snapshot = registry.begin_request();
        assert_eq!(snapshot.source(), Some(source.as_slice()));
        assert_eq!(snapshot.generation(), 1);
        assert_eq!(snapshot.version(), "approval-v2");
        let rules = &snapshot.policy().rules;
        assert_eq!(rules.len(), 3);
        assert_eq!(rules[0].effect, RuleEffect::Permit);
        assert!(rules[0].constraints.required_approval);
        assert!(!rules[1].constraints.required_approval);
        assert_eq!(rules[2].effect, RuleEffect::Deny);
        assert!(!rules[2].constraints.required_approval);
    }

    #[test]
    fn approval_source_v2_refuses_ambiguous_and_unsupported_declarations() {
        for rule in [
            "review,permit",
            "review,permit,true",
            "review,deny,required_approval",
            "review,permit,required_approval,extra",
            "review,permit,",
            ",permit,required_approval",
            "review,allow,required_approval",
        ] {
            let source =
                format!("schema=layerx.policy.source.v2\nversion=v2\nsteps=2\nrule={rule}\n");
            assert!(load_policy_source(source.as_bytes()).is_err(), "{rule}");
        }
        for source in [
            "schema=layerx.policy.source.v3\nversion=v2\nsteps=2\nrule=review,permit,required_approval\n",
            "schema=layerx.policy.source.v2\r\nversion=v2\nsteps=2\nrule=review,permit,required_approval\n",
            "version=v2\nschema=layerx.policy.source.v2\nsteps=2\nrule=review,permit,required_approval\n",
            "schema=layerx.policy.source.v2\nschema=layerx.policy.source.v2\nversion=v2\nsteps=2\n",
        ] {
            assert!(load_policy_source(source.as_bytes()).is_err());
        }
    }

    #[test]
    fn unprofiled_policy_source_keeps_two_field_grammar_and_refusals() {
        let policy =
            load_policy_source(b"version=v1\nsteps=2\nrule=ordinary,permit\nrule=blocked,deny\n")
                .expect("original configured source");
        assert!(policy
            .rules
            .iter()
            .all(|rule| !rule.constraints.required_approval));
        assert_eq!(
            load_policy_source(b"version=v1\nsteps=2\nrule=ordinary,permit,required_approval\n"),
            Err(PolicySourceError::InvalidEffect)
        );
        assert_eq!(
            load_policy_source(b"version=v1\nsteps=2\nrule=ordinary,permit,no_approval\n"),
            Err(PolicySourceError::InvalidEffect)
        );
        assert_eq!(
            load_policy_source(b"version=v1\nsteps=2\napproval=required\n"),
            Err(PolicySourceError::InvalidDeclaration)
        );
    }
}
