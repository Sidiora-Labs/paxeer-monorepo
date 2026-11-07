//! Durable tenant-owned projection of agents created through the Human plane.

use std::collections::BTreeSet;

use layerx_types::payload::ModuleId;
use layerx_types::verify::VerificationLevel;
use layerx_wire::receipt::decode as decode_receipt;
use sha2::{Digest, Sha256};

use crate::human::{HumanAgentLifecycleSeed, HumanOperationError, HumanResponse};
use crate::session::{InvalidationReason, RevocationEvent, SessionId, SessionRegistry};
use crate::store::{key, ObjectKind, StorageClass, Store, TenantId};

pub type SessionRestrictionRequest<'a> = (
    [u8; 32],
    [u8; 32],
    u64,
    [u8; 32],
    [u8; 32],
    u64,
    u64,
    &'a BTreeSet<String>,
    &'a BTreeSet<u16>,
);

pub type SessionRestrictionUpdate = (
    HumanResponse,
    crate::store::TenantKey,
    Vec<u8>,
    crate::store::TenantKey,
    Vec<u8>,
    crate::store::TenantKey,
    Vec<u8>,
);

#[path = "managed_agent_rotation.rs"]
pub(crate) mod rotation;

const PREFIX: &[u8] = b"managed-agent-v1:";
const VERSION: u8 = 4;
const LEGACY_VERSION: u8 = 3;
const MAX_AGENTS_PER_PAGE: u8 = 100;
const MAX_EVIDENCE: usize = 64;

#[derive(Clone, Copy, Debug, Eq, PartialEq, serde::Serialize, serde::Deserialize)]
pub struct FinalizationEvidence {
    pub action_key: [u8; 32],
    pub activity_id: [u8; 32],
    pub receipt_digest: [u8; 32],
    pub observed_sequence: u64,
    pub verification: u8,
    pub finalized_at: u64,
}
impl From<crate::human::HumanFinalizationEvidence> for FinalizationEvidence {
    fn from(value: crate::human::HumanFinalizationEvidence) -> Self {
        Self {
            action_key: value.action_key,
            activity_id: value.tx_hash,
            receipt_digest: value.receipt_hash,
            observed_sequence: value.observed_height,
            verification: value.rung,
            finalized_at: value.finalized_at,
        }
    }
}

#[derive(Clone, Eq, PartialEq)]
pub struct ManagedAgent {
    pub agent_id: String,
    pub name: String,
    pub purpose: String,
    pub state: u8,
    pub monthly_limit: u128,
    pub currency: String,
    pub period_start: u64,
    pub period_end: u64,
    pub spent: u128,
    pub created_at: u64,
    pub updated_at: u64,
    pub verified_evidence: Vec<[u8; 32]>,
    pub context: HumanAgentLifecycleSeed,
    pub agent_did: String,
    pub session_id: [u8; 32],
    pub session_token_id: [u8; 32],
    pub session_generation: u64,
    pub protocol_grant_id: [u8; 32],
    pub active_budget_id: [u8; 32],
}

impl std::fmt::Debug for ManagedAgent {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("ManagedAgent")
            .field("agent_id", &self.agent_id)
            .field("state", &self.state)
            .field("agent_did", &self.agent_did)
            .field("session_id", &self.session_id)
            .field("session_token_id", &"[redacted]")
            .field("session_generation", &self.session_generation)
            .finish_non_exhaustive()
    }
}

/// Durable replay result for one server-issued session-scope restriction.
pub struct SessionRestrictionReplay {
    pub session_id: [u8; 32],
    pub replacement_token: [u8; 32],
    pub generation: u64,
    pub response: HumanResponse,
}

/// Authority revocation whose finalized receipt and managed-agent observations were verified.
/// The inner event is intentionally constructible only in this module.
pub struct ValidatedAuthorityRevocation {
    event: RevocationEvent,
}

impl ValidatedAuthorityRevocation {
    pub(crate) const fn event(&self) -> &RevocationEvent {
        &self.event
    }
}

impl ManagedAgent {
    /// # Errors
    /// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
    pub fn from_creation(
        seed: &HumanAgentLifecycleSeed,
        installed_agent: &str,
        session_id: [u8; 32],
        session_token_id: [u8; 32],
        session_generation: u64,
    ) -> Result<Self, HumanOperationError> {
        if installed_agent.is_empty()
            || session_id == [0; 32]
            || session_token_id == [0; 32]
            || session_generation == 0
            || seed.agent_id.is_empty()
            || seed.name.is_empty()
            || seed.purpose.is_empty()
            || seed.currency.is_empty()
            || seed.monthly_limit == 0
            || seed.period_end <= seed.period_start
            || seed.updated_at < seed.created_at
            || seed.verified_evidence.is_empty()
            || seed.verified_evidence.len() > MAX_EVIDENCE
            || seed.verified_evidence.contains(&[0; 32])
        {
            return Err(HumanOperationError::Refused);
        }
        validate_context(seed)?;
        let active_budget_id = parse_agent_digest(&seed.agent_id)?;
        Ok(Self {
            agent_id: seed.agent_id.clone(),
            name: seed.name.clone(),
            purpose: seed.purpose.clone(),
            state: 1,
            monthly_limit: seed.monthly_limit,
            currency: seed.currency.clone(),
            period_start: seed.period_start,
            period_end: seed.period_end,
            spent: 0,
            created_at: seed.created_at,
            updated_at: seed.updated_at,
            verified_evidence: seed.verified_evidence.clone(),
            context: seed.clone(),
            agent_did: installed_agent.to_owned(),
            session_id,
            session_token_id,
            session_generation,
            protocol_grant_id: seed.protocol_grant_id,
            active_budget_id,
        })
    }

    fn validate(&self) -> Result<(), HumanOperationError> {
        if self.active_budget_id == [0; 32] {
            return Err(HumanOperationError::Refused);
        }
        if self.agent_id.is_empty()
            || self.agent_did.is_empty()
            || self.session_id == [0; 32]
            || self.session_token_id == [0; 32]
            || self.session_generation == 0
            || self.name.is_empty()
            || self.purpose.is_empty()
            || self.currency.is_empty()
            || self.state > 4
            || self.monthly_limit == 0
            || self.period_end <= self.period_start
            || self.updated_at < self.created_at
            || self.spent > self.monthly_limit
            || self.verified_evidence.is_empty()
            || self.verified_evidence.len() > MAX_EVIDENCE
            || self.verified_evidence.contains(&[0; 32])
            || self.context.agent_id != self.agent_id
        {
            return Err(HumanOperationError::Refused);
        }
        validate_context(&self.context)?;
        Ok(())
    }
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn publish_creation(
    store: &mut Store,
    tenant: &TenantId,
    agent: &ManagedAgent,
) -> Result<(), HumanOperationError> {
    agent.validate()?;
    let object_key = agent_key(tenant, &agent.agent_id)?;
    if let Some(existing) = store.get(&object_key) {
        if existing.class() != StorageClass::LocalOnly || decode(existing.bytes())? != *agent {
            return Err(HumanOperationError::Refused);
        }
        return Ok(());
    }
    store
        .put_local(object_key, encode(agent)?)
        .map_err(|_| HumanOperationError::Unavailable)
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn publish_creation_with_companion(
    store: &mut Store,
    tenant: &TenantId,
    agent: &ManagedAgent,
    companion: crate::store::TenantKey,
    bytes: Vec<u8>,
) -> Result<(), HumanOperationError> {
    agent.validate()?;
    let object_key = agent_key(tenant, &agent.agent_id)?;
    if let Some(existing) = store.get(&object_key) {
        if existing.class() != StorageClass::LocalOnly || decode(existing.bytes())? != *agent {
            return Err(HumanOperationError::Refused);
        }
        store
            .put_local(companion, bytes)
            .map_err(|_| HumanOperationError::Unavailable)?;
    } else {
        store
            .update_local_with_companion(object_key, encode(agent)?, companion, bytes)
            .map_err(|_| HumanOperationError::Unavailable)?;
    }
    Ok(())
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn validate_tenant(store: &Store, tenant: &TenantId) -> Result<(), HumanOperationError> {
    for object_id in store.list_object_ids(tenant, ObjectKind::Configuration) {
        if !object_id.starts_with(PREFIX) {
            continue;
        }
        let object_key = key(tenant.clone(), ObjectKind::Configuration, object_id)
            .map_err(|_| HumanOperationError::Refused)?;
        let value = store
            .get(&object_key)
            .ok_or(HumanOperationError::Unavailable)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(HumanOperationError::Refused);
        }
        let agent = decode(value.bytes())?;
        if agent_key(tenant, &agent.agent_id)? != object_key {
            return Err(HumanOperationError::Refused);
        }
    }
    Ok(())
}

/// One managed agent's protocol budget as durably recorded for a tenant.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BudgetOwner {
    pub agent_id: String,
    pub agent_did: String,
    pub active_budget_id: [u8; 32],
}

/// Lists every managed agent of `tenant` holding an active protocol budget,
/// ordered by agent identifier. Agents without an assigned budget are omitted.
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn budget_owners(
    store: &Store,
    tenant: &TenantId,
) -> Result<Vec<BudgetOwner>, HumanOperationError> {
    let mut owners = Vec::new();
    for object_id in store.list_object_ids(tenant, ObjectKind::Configuration) {
        if !object_id.starts_with(PREFIX) {
            continue;
        }
        let object_key = key(tenant.clone(), ObjectKind::Configuration, object_id)
            .map_err(|_| HumanOperationError::Refused)?;
        let value = store
            .get(&object_key)
            .ok_or(HumanOperationError::Unavailable)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(HumanOperationError::Refused);
        }
        let agent = decode(value.bytes())?;
        if agent_key(tenant, &agent.agent_id)? != object_key {
            return Err(HumanOperationError::Refused);
        }
        if agent.active_budget_id == [0; 32] {
            continue;
        }
        owners.push(BudgetOwner {
            agent_id: agent.agent_id,
            agent_did: agent.agent_did,
            active_budget_id: agent.active_budget_id,
        });
    }
    owners.sort_by(|left, right| left.agent_id.cmp(&right.agent_id));
    Ok(owners)
}

/// Returns the managed agent of `tenant` whose identifier digest is `agent_digest`.
///
/// The digest is the same value the human service uses as the agent's budget
/// identifier, so a budget-create payload naming it is bound to this agent.
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn budget_candidate(
    store: &Store,
    tenant: &TenantId,
    agent_digest: [u8; 32],
) -> Result<ManagedAgent, HumanOperationError> {
    if agent_digest == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    for object_id in store.list_object_ids(tenant, ObjectKind::Configuration) {
        if !object_id.starts_with(PREFIX) {
            continue;
        }
        let object_key = key(tenant.clone(), ObjectKind::Configuration, object_id)
            .map_err(|_| HumanOperationError::Refused)?;
        let value = store
            .get(&object_key)
            .ok_or(HumanOperationError::Unavailable)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(HumanOperationError::Refused);
        }
        let agent = decode(value.bytes())?;
        if agent_key(tenant, &agent.agent_id)? != object_key {
            return Err(HumanOperationError::Refused);
        }
        if parse_agent_digest(&agent.agent_id)? == agent_digest {
            return Ok(agent);
        }
    }
    Err(HumanOperationError::Refused)
}

/// Durably records `budget_id` as the active protocol budget of one managed agent
/// after it has been confirmed from proven core state.
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn assign_budget(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    budget_id: [u8; 32],
) -> Result<(), HumanOperationError> {
    if budget_id == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    let mut agent = load_agent(store, tenant, agent_id)?;
    if agent.state == 4 {
        return Err(HumanOperationError::Refused);
    }
    agent.active_budget_id = budget_id;
    agent.validate()?;
    store
        .put_local(agent_key(tenant, agent_id)?, encode(&agent)?)
        .map_err(|_| HumanOperationError::Unavailable)
}

/// Validates every durable managed-agent credential against the restored authoritative session
/// record. Legacy v3 managed records decode at the legacy session generation of one and are
/// refused if the corresponding restored session has ever advanced.
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn validate_session_coordinates(
    store: &Store,
    sessions: &SessionRegistry,
    tenant: &TenantId,
) -> Result<(), HumanOperationError> {
    for object_id in store.list_object_ids(tenant, ObjectKind::Configuration) {
        if !object_id.starts_with(PREFIX) {
            continue;
        }
        let object_key = key(tenant.clone(), ObjectKind::Configuration, object_id)
            .map_err(|_| HumanOperationError::Refused)?;
        let value = store
            .get(&object_key)
            .ok_or(HumanOperationError::Unavailable)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(HumanOperationError::Refused);
        }
        let agent = decode(value.bytes())?;
        let session = sessions
            .get(tenant, SessionId(agent.session_id))
            .ok_or(HumanOperationError::Refused)?;
        if session.request.token_id != agent.session_token_id
            || session.generation != agent.session_generation
            || session.request.agent.as_bytes() != agent.agent_did.as_bytes()
        {
            return Err(HumanOperationError::Refused);
        }
    }
    Ok(())
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn get(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
) -> Result<HumanResponse, HumanOperationError> {
    let object_key = agent_key(tenant, agent_id)?;
    let value = store.get(&object_key).ok_or(HumanOperationError::Refused)?;
    if value.class() != StorageClass::LocalOnly {
        return Err(HumanOperationError::Refused);
    }
    response_agent(&decode(value.bytes())?)
}

pub(crate) fn authenticates_native_owner(
    store: &Store,
    tenant: &TenantId,
    owner: &str,
    owner_account: &str,
    actor: &[u8],
) -> Result<bool, HumanOperationError> {
    let mut found = false;
    for object_id in store.list_object_ids(tenant, ObjectKind::Configuration) {
        if !object_id.starts_with(PREFIX) {
            continue;
        }
        let object_key = key(tenant.clone(), ObjectKind::Configuration, object_id)
            .map_err(|_| HumanOperationError::Refused)?;
        let value = store
            .get(&object_key)
            .ok_or(HumanOperationError::Unavailable)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(HumanOperationError::Refused);
        }
        let agent = decode(value.bytes())?;
        if agent_key(tenant, &agent.agent_id)? != object_key {
            return Err(HumanOperationError::Refused);
        }
        if agent.agent_did.as_bytes() != actor {
            continue;
        }
        if found || agent.context.actor != owner || agent.context.owner_account != owner_account {
            return Err(HumanOperationError::Refused);
        }
        let digest = *agent
            .context
            .creation_receipt_roots
            .first()
            .ok_or(HumanOperationError::Refused)?;
        if !agent.context.verified_evidence.contains(&digest) {
            return Err(HumanOperationError::Refused);
        }
        let served = evidence_export(store, tenant, &agent.agent_id, digest)?;
        let receipt =
            decode_receipt(&served.canonical_bytes).map_err(|_| HumanOperationError::Refused)?;
        let protocol = receipt.protocol().ok_or(HumanOperationError::Refused)?;
        if protocol.module_id() != ModuleId::Governance as u16
            || protocol.operation() != 0
            || protocol.result_code() != 0
            || served.metadata.result.code.raw() != 0
            || served.metadata.verification_level < VerificationLevel::SEQUENCER_SIGNED
        {
            return Err(HumanOperationError::Refused);
        }
        let action_key = served.metadata.idempotency_key;
        for object_kind in [ObjectKind::Outbox, ObjectKind::PreparedActivity] {
            let stored_key = key(tenant.clone(), object_kind, action_key.to_vec())
                .map_err(|_| HumanOperationError::Refused)?;
            let stored = store
                .get(&stored_key)
                .ok_or(HumanOperationError::Unavailable)?;
            if stored.class() != StorageClass::LocalOnly {
                return Err(HumanOperationError::Refused);
            }
        }
        let authority = layerx_agent_api::identity::strict_hex32(
            &agent.context.primary_authority,
            "primary_authority",
        )
        .map_err(|_| HumanOperationError::Refused)?;
        if authority[..12] != [0; 12] {
            return Err(HumanOperationError::Refused);
        }
        let sender = authority[12..]
            .try_into()
            .map_err(|_| HumanOperationError::Refused)?;
        let transaction =
            restore_owner_transaction(store, tenant, &served, sender, agent.context.network_id)?;
        let mut word = [0; 32];
        word[31] = 64;
        if transaction.to != layerx_client::account_binding::ADDR_PRECOMPILE
            || transaction.data.len() != 164
            || transaction.data[..4] != layerx_client::account_binding::SELECTOR_BIND_LAYERX
            || transaction.data[4..36] != agent.context.custody_public_key
            || transaction.data[36..68] != word
            || transaction.data[68..100] != word
            || format!("did:layerx:{}", hex(&agent.context.custody_public_key)).as_bytes() != actor
        {
            return Err(HumanOperationError::Refused);
        }
        found = true;
    }
    Ok(found)
}

/// Restores the exact owner-signed transaction bound to a served receipt.
///
/// # Errors
/// Refuses malformed type-2 transactions, invalid signatures, or mismatched
/// owner, network, outbox key, and receipt transaction hash.
pub fn restore_owner_transaction(
    store: &Store,
    tenant: &TenantId,
    served: &crate::receipt::ServedReceipt,
    owner: [u8; 20],
    network_id: u32,
) -> Result<layerx_crypto::evm_transaction::Eip1559Call, HumanOperationError> {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};

    let action_key = served.metadata.idempotency_key;
    let mut outbox = crate::outbox::Outbox::default();
    outbox
        .restore(store, tenant.clone(), action_key)
        .map_err(|_| HumanOperationError::Refused)?;
    let exact = outbox
        .exact_signed_bytes(action_key)
        .map_err(|_| HumanOperationError::Refused)?;
    let (transaction, signature) =
        decode_signed(exact).map_err(|_| HumanOperationError::Refused)?;
    let sender =
        recover_sender(&transaction, &signature).map_err(|_| HumanOperationError::Refused)?;
    let hash = transaction_hash(exact);
    if sender != owner
        || network_id == 0
        || transaction.chain_id != u64::from(network_id)
        || hash != action_key
        || hash != served.metadata.activity_id
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(transaction)
}

pub fn evidence_export(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    digest: [u8; 32],
) -> Result<crate::receipt::ServedReceipt, HumanOperationError> {
    if digest == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    let object_key = agent_key(tenant, agent_id)?;
    let value = store.get(&object_key).ok_or(HumanOperationError::Refused)?;
    if value.class() != StorageClass::LocalOnly {
        return Err(HumanOperationError::Refused);
    }
    let agent = decode(value.bytes())?;
    if agent.agent_id != agent_id {
        return Err(HumanOperationError::Refused);
    }
    if store
        .get(&finalized_evidence_key(tenant, agent_id, digest)?)
        .is_some()
    {
        let evidence = finalized_evidence(store, tenant, agent_id, digest)?;
        let decoded = decode_receipt(&evidence.canonical_receipt)
            .map_err(|_| HumanOperationError::Refused)?;
        let protocol = decoded.protocol().ok_or(HumanOperationError::Refused)?;
        return Ok(crate::receipt::ServedReceipt {
            canonical_bytes: evidence.canonical_receipt.clone(),
            metadata: crate::receipt::ReceiptMetadata {
                activity_id: evidence.metadata.transaction_hash,
                idempotency_key: evidence.metadata.transaction_hash,
                global_sequence: protocol.global_sequence(),
                verification_level: VerificationLevel::COMMITTED,
                result: crate::receipt::classify(layerx_types::result::ResultCode::from_raw(
                    protocol.result_code(),
                )),
            },
        });
    }
    if !agent.context.verified_evidence.contains(&digest)
        && !agent.verified_evidence.contains(&digest)
    {
        return Err(HumanOperationError::Refused);
    }
    let inventory = crate::receipt::evidence_inventory(store, tenant)
        .map_err(|_| HumanOperationError::Refused)?;
    let idempotency_keys = inventory
        .with_evidence
        .iter()
        .map(|record| record.idempotency_key)
        .chain(
            inventory
                .with_current_evidence
                .iter()
                .map(|record| record.idempotency_key),
        )
        .chain(
            inventory
                .without_evidence
                .iter()
                .map(|metadata| metadata.idempotency_key),
        );
    for idempotency_key in idempotency_keys {
        let served = crate::receipt::serve(
            store,
            tenant.clone(),
            crate::receipt::ReceiptLookupKey::Idempotency(idempotency_key),
        )
        .map_err(|_| HumanOperationError::Refused)?;
        let receipt_digest: [u8; 32] = Sha256::digest(&served.canonical_bytes).into();
        if receipt_digest != digest {
            continue;
        }
        let receipt =
            decode_receipt(&served.canonical_bytes).map_err(|_| HumanOperationError::Refused)?;
        let protocol = receipt.protocol().ok_or(HumanOperationError::Refused)?;
        if layerx_wire::receipt::encode(&receipt).map_err(|_| HumanOperationError::Refused)?
            != served.canonical_bytes
            || served.metadata.idempotency_key != idempotency_key
            || served.metadata.activity_id != protocol.activity_id()
            || served.metadata.global_sequence != protocol.global_sequence()
            || served.metadata.result
                != crate::receipt::classify(layerx_types::result::ResultCode::from_raw(
                    protocol.result_code(),
                ))
        {
            return Err(HumanOperationError::Refused);
        }
        return Ok(served);
    }
    Err(HumanOperationError::Unavailable)
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn context(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
) -> Result<HumanResponse, HumanOperationError> {
    let object = store
        .get(&agent_key(tenant, agent_id)?)
        .ok_or(HumanOperationError::Refused)?;
    if object.class() != StorageClass::LocalOnly {
        return Err(HumanOperationError::Refused);
    }
    let agent = decode(object.bytes())?;
    let mut out = Wire::new();
    encode_context(&mut out, &agent.context)?;
    out.text(&agent.agent_did)?;
    out.fixed(&agent.session_id);
    out.fixed(&agent.session_token_id);
    out.u64(agent.session_generation);
    out.fixed(&agent.protocol_grant_id);
    out.fixed(&agent.active_budget_id);
    out.u8(agent.state);
    out.u128(agent.monthly_limit);
    out.u128(agent.spent);
    out.u64(agent.updated_at);
    out.finish()
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn record_observation(
    store: &mut Store,
    tenant: &TenantId,
    class: u8,
    digest: [u8; 32],
    bytes: Vec<u8>,
) -> Result<(), HumanOperationError> {
    if digest == [0; 32] || bytes.is_empty() {
        return Err(HumanOperationError::Refused);
    }
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(class);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    if let Some(existing) = store.get(&record) {
        if existing.class() != StorageClass::LocalOnly || existing.bytes() != bytes {
            return Err(HumanOperationError::Refused);
        }
        return Ok(());
    }
    store
        .put_local(record, bytes)
        .map_err(|_| HumanOperationError::Unavailable)
}
// Persisted key-policy observations predate the current committed wire rank.
fn decode_key_policy_verification(rank: u8) -> Result<VerificationLevel, HumanOperationError> {
    match rank {
        4 | 5 => Ok(VerificationLevel::COMMITTED),
        rank => VerificationLevel::from_wire_rank(rank).map_err(|_| HumanOperationError::Refused),
    }
}

fn budget_observation(
    store: &Store,
    tenant: &TenantId,
    digest: [u8; 32],
    budget: [u8; 32],
    minimum_sequence: u64,
    require_zero: bool,
) -> Result<(), HumanOperationError> {
    if digest == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(1);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    let bytes = store
        .get(&record)
        .ok_or(HumanOperationError::Unavailable)?
        .bytes();
    let (stored_budget, state) = crate::human_runtime::CoreBudgetState::decode_snapshot(bytes)
        .map_err(|_| HumanOperationError::Refused)?;
    if stored_budget != budget
        || state.evidence_digest != digest
        || state.verification != VerificationLevel::COMMITTED.wire_rank()
        || state.receipt_digest == [0; 32]
        || state.checkpoint_digest == [0; 32]
        || state.asset == [0; 32]
    {
        return Err(HumanOperationError::Refused);
    }
    if state.observed_head_sequence < minimum_sequence
        || state.maximum_age_sequences == 0
        || state.age_sequences > state.maximum_age_sequences
        || (require_zero && state.remaining != 0)
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(())
}

fn key_policy_observation(
    store: &Store,
    tenant: &TenantId,
    digest: [u8; 32],
    did: &str,
    recovery: bool,
    (delay, ready_at, finalized_at): (u64, u64, u64),
) -> Result<(), HumanOperationError> {
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(2);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    let bytes = store
        .get(&record)
        .ok_or(HumanOperationError::Unavailable)?
        .bytes();
    let mut input = Input { bytes, offset: 0 };
    if input.text()? != did || input.u8()? != u8::from(recovery) {
        return Err(HumanOperationError::Refused);
    }
    let revision = input.u64()?;
    let required = input.u64()?;
    let maximum = input.u64()?;
    let effective = input.u64()?;
    let observed = input.u64()?;
    let verification = input.u8()?;
    let observed_digest = input.fixed()?;
    let checkpoint = input.fixed()?;
    let age = input.u64()?;
    let maximum_age = input.u64()?;
    if input.offset != bytes.len()
        || revision == 0
        || delay != required
        || delay > maximum
        || ready_at
            != finalized_at
                .checked_add(delay)
                .ok_or(HumanOperationError::Refused)?
        || effective == 0
        || observed < effective
        || decode_key_policy_verification(verification)? != VerificationLevel::COMMITTED
        || observed_digest != digest
        || checkpoint == [0; 32]
        || maximum_age == 0
        || age > maximum_age
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(())
}

fn require_session_observation(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    digest: [u8; 32],
    resume: bool,
) -> Result<(), HumanOperationError> {
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(3);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    let bytes = store
        .get(&record)
        .ok_or(HumanOperationError::Unavailable)?
        .bytes();
    let observation = SessionObservationV1::decode(bytes)?;
    if observation.agent_id != agent_id
        || observation.digest != digest
        || observation.open != resume
        || observation.action == [0; 32]
    {
        return Err(HumanOperationError::Refused);
    }
    let agent = load_agent(store, tenant, agent_id)?;
    if agent.agent_did != observation.did
        || agent.session_id != observation.session
        || agent.session_token_id != observation.token
        || agent.session_generation != observation.generation
    {
        return Err(HumanOperationError::Refused);
    }
    let mut sessions = SessionRegistry::default();
    sessions
        .restore_tenant(store, tenant)
        .map_err(|_| HumanOperationError::Refused)?;
    let restored = sessions
        .get(tenant, SessionId(observation.session))
        .ok_or(HumanOperationError::Refused)?;
    if restored.request.token_id != observation.token
        || restored.generation != observation.generation
        || restored.request.agent.as_bytes() != observation.did.as_bytes()
        || restored.open != observation.open
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(())
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn finalize_control(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    resume: bool,
    session_observation: [u8; 32],
    evidence: FinalizationEvidence,
) -> Result<HumanResponse, HumanOperationError> {
    require_session_observation(store, tenant, agent_id, session_observation, resume)?;
    let mut operation = vec![26, u8::from(resume)];
    operation.extend_from_slice(&session_observation);
    finalize_agent(
        store,
        tenant,
        agent_id,
        evidence,
        (ModuleId::Governance, 6, &operation),
        |agent| {
            if resume {
                if agent.state != 2 {
                    return Err(HumanOperationError::Refused);
                }
                agent.state = 1;
            } else {
                if agent.state != 1 {
                    return Err(HumanOperationError::Refused);
                }
                agent.state = 2;
            }
            Ok(())
        },
        response_agent,
    )
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn finalize_limit(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    monthly_limit: u128,
    currency: &str,
    replacement_budget_id: [u8; 32],
    evidence: FinalizationEvidence,
) -> Result<HumanResponse, HumanOperationError> {
    if monthly_limit == 0 || currency.is_empty() || replacement_budget_id == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    let mut body = Vec::new();
    body.push(27);
    body.extend_from_slice(&monthly_limit.to_be_bytes());
    body.extend_from_slice(currency.as_bytes());
    body.extend_from_slice(&replacement_budget_id);
    finalize_agent(
        store,
        tenant,
        agent_id,
        evidence,
        (ModuleId::Budget, 1, &body),
        |agent| {
            if agent.state == 4 || agent.currency != currency || agent.spent > monthly_limit {
                return Err(HumanOperationError::Refused);
            }
            agent.monthly_limit = monthly_limit;
            agent.active_budget_id = replacement_budget_id;
            Ok(())
        },
        response_agent,
    )
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn finalize_journey(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    (kind, amount, currency, delay, ready_at): (u8, u128, &str, u64, u64),
    pre_observation: [u8; 32],
    post_observation: [u8; 32],
    evidence: FinalizationEvidence,
) -> Result<HumanResponse, HumanOperationError> {
    if kind > 2
        || (kind == 0 && (amount == 0 || currency.is_empty() || delay != 0 || ready_at != 0))
        || (kind != 0
            && (amount != 0
                || !currency.is_empty()
                || delay == 0
                || ready_at <= evidence.finalized_at))
    {
        return Err(HumanOperationError::Refused);
    }
    if kind == 0 && pre_observation == post_observation {
        return Err(HumanOperationError::Refused);
    }
    if kind == 0 {
        let agent = load_agent(store, tenant, agent_id)?;
        budget_observation(
            store,
            tenant,
            pre_observation,
            agent.active_budget_id,
            0,
            false,
        )?;
        budget_observation(
            store,
            tenant,
            post_observation,
            agent.active_budget_id,
            evidence.observed_sequence,
            false,
        )?;
    } else {
        if pre_observation == [0; 32] || post_observation != [0; 32] {
            return Err(HumanOperationError::Refused);
        }
        let agent = load_agent(store, tenant, agent_id)?;
        key_policy_observation(
            store,
            tenant,
            pre_observation,
            &agent.agent_did,
            kind == 2,
            (delay, ready_at, evidence.finalized_at),
        )?;
    }
    let mut body = Vec::new();
    body.push(28);
    body.push(kind);
    body.extend_from_slice(&amount.to_be_bytes());
    body.extend_from_slice(currency.as_bytes());
    body.extend_from_slice(&delay.to_be_bytes());
    body.extend_from_slice(&ready_at.to_be_bytes());
    body.extend_from_slice(&pre_observation);
    body.extend_from_slice(&post_observation);
    finalize_agent(
        store,
        tenant,
        agent_id,
        evidence,
        (
            match kind {
                0 => ModuleId::Budget,
                1 | 2 => ModuleId::Governance,
                _ => return Err(HumanOperationError::Refused),
            },
            match kind {
                0 => 8,
                1 | 2 => 2,
                _ => return Err(HumanOperationError::Refused),
            },
            &body,
        ),
        |agent| {
            if agent.state == 4 || (kind == 0 && agent.currency != currency) {
                return Err(HumanOperationError::Refused);
            }
            Ok(())
        },
        |agent| {
            if kind == 0 {
                journey_response(agent, kind, evidence)
            } else {
                challenge_response(agent, kind, delay, ready_at, evidence)
            }
        },
    )
}

/// Verifies the finalized governance evidence that invalidates all sessions under an agent DID.
/// This performs no session mutation and returns an opaque value for the short ordered commit.
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn validate_authority_revocation(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    (kind, delay, ready_at): (u8, u64, u64),
    pre_observation: [u8; 32],
    post_observation: [u8; 32],
    evidence: FinalizationEvidence,
) -> Result<ValidatedAuthorityRevocation, HumanOperationError> {
    if !matches!(kind, 1 | 2)
        || delay == 0
        || ready_at <= evidence.finalized_at
        || pre_observation == [0; 32]
        || post_observation != [0; 32]
    {
        return Err(HumanOperationError::Refused);
    }
    let current = load_agent(store, tenant, agent_id)?;
    if current.state == 4 {
        return Err(HumanOperationError::Refused);
    }
    key_policy_observation(
        store,
        tenant,
        pre_observation,
        &current.agent_did,
        kind == 2,
        (delay, ready_at, evidence.finalized_at),
    )?;
    let mut body = Vec::new();
    body.push(28);
    body.push(kind);
    body.extend_from_slice(&0_u128.to_be_bytes());
    body.extend_from_slice(&delay.to_be_bytes());
    body.extend_from_slice(&ready_at.to_be_bytes());
    body.extend_from_slice(&pre_observation);
    body.extend_from_slice(&post_observation);
    let source = validate_finalization_source(
        store,
        tenant,
        agent_id,
        evidence,
        ModuleId::Governance,
        2,
        &body,
    )?;
    let agent = match source {
        FinalizationSource::Completed(_) => current.clone(),
        FinalizationSource::Pending { agent, .. } => *agent,
    };
    if agent.state == 4 || agent.agent_did != current.agent_did {
        return Err(HumanOperationError::Refused);
    }
    let did = layerx_types::ids::Did::new(agent.agent_did.as_bytes())
        .map_err(|_| HumanOperationError::Refused)?;
    Ok(ValidatedAuthorityRevocation {
        event: RevocationEvent {
            did,
            authority: None,
            reason: if kind == 1 {
                InvalidationReason::PrimaryKeyRotated
            } else {
                InvalidationReason::AccountRecovered
            },
            observed_sequence: evidence.observed_sequence,
        },
    })
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn finalize_archive(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    confirm_name: &str,
    (pre_observation, post_observation, session_observation): ([u8; 32], [u8; 32], [u8; 32]),
    evidence: FinalizationEvidence,
) -> Result<HumanResponse, HumanOperationError> {
    if pre_observation == post_observation {
        return Err(HumanOperationError::Refused);
    }
    let current = load_agent(store, tenant, agent_id)?;
    budget_observation(
        store,
        tenant,
        pre_observation,
        current.active_budget_id,
        0,
        false,
    )?;
    budget_observation(
        store,
        tenant,
        post_observation,
        current.active_budget_id,
        evidence.observed_sequence,
        true,
    )?;
    require_session_observation(store, tenant, agent_id, session_observation, false)?;
    let mut body = Vec::new();
    body.push(29);
    body.extend_from_slice(confirm_name.as_bytes());
    body.extend_from_slice(&pre_observation);
    body.extend_from_slice(&post_observation);
    body.extend_from_slice(&session_observation);
    finalize_agent(
        store,
        tenant,
        agent_id,
        evidence,
        (ModuleId::Governance, 6, &body),
        |agent| {
            if agent.state == 4 || agent.name != confirm_name {
                return Err(HumanOperationError::Refused);
            }
            agent.state = 4;
            Ok(())
        },
        |agent| journey_response(agent, 3, evidence),
    )
}
fn load_agent(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
) -> Result<ManagedAgent, HumanOperationError> {
    let value = store
        .get(&agent_key(tenant, agent_id)?)
        .ok_or(HumanOperationError::Refused)?;
    if value.class() != StorageClass::LocalOnly {
        return Err(HumanOperationError::Refused);
    }
    decode(value.bytes())
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn session_coordinates(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
) -> Result<(String, [u8; 32], [u8; 32], u64), HumanOperationError> {
    let agent = load_agent(store, tenant, agent_id)?;
    Ok((
        agent.agent_did,
        agent.session_id,
        agent.session_token_id,
        agent.session_generation,
    ))
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn protocol_grant(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
) -> Result<[u8; 32], HumanOperationError> {
    let agent = load_agent(store, tenant, agent_id)?;
    if agent.protocol_grant_id == [0; 32] {
        Err(HumanOperationError::Refused)
    } else {
        Ok(agent.protocol_grant_id)
    }
}
fn session_bytes(
    agent: &ManagedAgent,
    action: [u8; 32],
    open: bool,
) -> Result<(HumanResponse, [u8; 32]), HumanOperationError> {
    if action == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    let observation = SessionObservationV1 {
        agent_id: agent.agent_id.clone(),
        did: agent.agent_did.clone(),
        session: agent.session_id,
        token: agent.session_token_id,
        generation: agent.session_generation,
        open,
        action,
        digest: [0; 32],
    };
    observation.encode()
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn record_session_observation(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    action: [u8; 32],
    open: bool,
    create: bool,
) -> Result<HumanResponse, HumanOperationError> {
    let agent = load_agent(store, tenant, agent_id)?;
    let (response, digest) = session_bytes(&agent, action, open)?;
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(3);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    if let Some(existing) = store.get(&record) {
        if existing.class() != StorageClass::LocalOnly || existing.bytes() != response.bytes() {
            return Err(HumanOperationError::Refused);
        }
        return Ok(response);
    }
    if !create {
        return Err(HumanOperationError::Refused);
    }
    store
        .put_local(record, response.bytes().to_vec())
        .map_err(|_| HumanOperationError::Unavailable)?;
    Ok(response)
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn prepare_session_observation(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    action: [u8; 32],
    open: bool,
) -> Result<(HumanResponse, crate::store::TenantKey, Vec<u8>), HumanOperationError> {
    let agent = load_agent(store, tenant, agent_id)?;
    let (response, digest) = session_bytes(&agent, action, open)?;
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(3);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    if store.get(&record).is_some() {
        return Err(HumanOperationError::Refused);
    }
    let bytes = response.bytes().to_vec();
    Ok((response, record, bytes))
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn prepare_session_token_restriction(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    (
        session,
        previous_token,
        previous_generation,
        replacement_token,
        action,
        generation,
        current_sequence,
        scopes,
        permitted_activity_types,
    ): SessionRestrictionRequest<'_>,
) -> Result<SessionRestrictionUpdate, HumanOperationError> {
    if replacement_token == [0; 32] || replacement_token == previous_token || generation == 0 {
        return Err(HumanOperationError::Refused);
    }
    let mut agent = load_agent(store, tenant, agent_id)?;
    if agent.session_id != session
        || agent.session_token_id != previous_token
        || agent.session_generation != previous_generation
    {
        return Err(HumanOperationError::Refused);
    }
    agent.session_token_id = replacement_token;
    agent.session_generation = generation;
    let (response, digest) = session_bytes(&agent, action, true)?;
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(3);
    id.extend_from_slice(&digest);
    let companion = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    if store.get(&companion).is_some() {
        return Err(HumanOperationError::Refused);
    }
    let request_digest = session_restriction_digest(
        agent_id,
        action,
        current_sequence,
        scopes,
        permitted_activity_types,
    )?;
    let ledger_key = session_restriction_key(tenant, action)?;
    if store.get(&ledger_key).is_some() {
        return Err(HumanOperationError::Refused);
    }
    let mut ledger = Wire::new();
    ledger.fixed(b"LXRT01");
    ledger.fixed(&request_digest);
    ledger.fixed(&session);
    ledger.fixed(&replacement_token);
    ledger.u64(generation);
    ledger.u32(u32::try_from(response.bytes().len()).map_err(|_| HumanOperationError::Refused)?);
    ledger.fixed(response.bytes());
    Ok((
        response.clone(),
        agent_key(tenant, agent_id)?,
        encode(&agent)?,
        companion,
        response.bytes().to_vec(),
        ledger_key,
        ledger.0,
    ))
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn replay_session_token_restriction(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    action: [u8; 32],
    current_sequence: u64,
    scopes: &BTreeSet<String>,
    permitted_activity_types: &BTreeSet<u16>,
) -> Result<Option<SessionRestrictionReplay>, HumanOperationError> {
    let ledger_key = session_restriction_key(tenant, action)?;
    let Some(stored) = store.get(&ledger_key) else {
        return Ok(None);
    };
    if stored.class() != StorageClass::LocalOnly {
        return Err(HumanOperationError::Refused);
    }
    let expected = session_restriction_digest(
        agent_id,
        action,
        current_sequence,
        scopes,
        permitted_activity_types,
    )?;
    let mut input = Input {
        bytes: stored.bytes(),
        offset: 0,
    };
    if input.take::<6>()? != *b"LXRT01" || input.fixed()? != expected {
        return Err(HumanOperationError::Refused);
    }
    let session_id = input.fixed()?;
    let replacement_token = input.fixed()?;
    let generation = input.u64()?;
    let response_length =
        usize::try_from(input.u32()?).map_err(|_| HumanOperationError::Refused)?;
    let response = HumanResponse::new(input.bytes(response_length)?.to_vec())
        .map_err(|_| HumanOperationError::Refused)?;
    if input.offset != input.bytes.len()
        || session_id == [0; 32]
        || replacement_token == [0; 32]
        || generation == 0
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(Some(SessionRestrictionReplay {
        session_id,
        replacement_token,
        generation,
        response,
    }))
}

fn session_restriction_key(
    tenant: &TenantId,
    action: [u8; 32],
) -> Result<crate::store::TenantKey, HumanOperationError> {
    if action == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    let mut id = b"managed-agent-session-restrict-v1:".to_vec();
    id.extend_from_slice(&action);
    key(tenant.clone(), ObjectKind::Idempotency, id).map_err(|_| HumanOperationError::Refused)
}

fn session_restriction_digest(
    agent_id: &str,
    action: [u8; 32],
    current_sequence: u64,
    scopes: &BTreeSet<String>,
    permitted_activity_types: &BTreeSet<u16>,
) -> Result<[u8; 32], HumanOperationError> {
    if agent_id.is_empty()
        || action == [0; 32]
        || current_sequence == 0
        || scopes.is_empty()
        || scopes.len() > 64
        || permitted_activity_types.is_empty()
        || permitted_activity_types.len() > 256
    {
        return Err(HumanOperationError::Refused);
    }
    let mut digest = Sha256::new();
    digest.update(b"layerx-agentd/session-restrict/v1\0");
    digest.update(
        u32::try_from(agent_id.len())
            .map_err(|_| HumanOperationError::Refused)?
            .to_be_bytes(),
    );
    digest.update(agent_id.as_bytes());
    digest.update(action);
    digest.update(current_sequence.to_be_bytes());
    digest.update(
        u16::try_from(scopes.len())
            .map_err(|_| HumanOperationError::Refused)?
            .to_be_bytes(),
    );
    for scope in scopes {
        digest.update(
            u32::try_from(scope.len())
                .map_err(|_| HumanOperationError::Refused)?
                .to_be_bytes(),
        );
        digest.update(scope.as_bytes());
    }
    digest.update(
        u16::try_from(permitted_activity_types.len())
            .map_err(|_| HumanOperationError::Refused)?
            .to_be_bytes(),
    );
    for activity in permitted_activity_types {
        digest.update(activity.to_be_bytes());
    }
    Ok(digest.finalize().into())
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn bind_session(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    (session, token, generation, grant): ([u8; 32], [u8; 32], u64, [u8; 32]),
    action: [u8; 32],
) -> Result<HumanResponse, HumanOperationError> {
    if session == [0; 32] || token == [0; 32] || generation == 0 {
        return Err(HumanOperationError::Refused);
    }
    let mut agent = load_agent(store, tenant, agent_id)?;
    agent.session_id = session;
    agent.session_token_id = token;
    agent.session_generation = generation;
    agent.protocol_grant_id = grant;
    agent.context.protocol_grant_id = grant;
    let (response, digest) = session_bytes(&agent, action, true)?;
    let mut id = b"managed-agent-observation-v1:".to_vec();
    id.push(3);
    id.extend_from_slice(&digest);
    let record = key(tenant.clone(), ObjectKind::Configuration, id)
        .map_err(|_| HumanOperationError::Refused)?;
    if let Some(existing) = store.get(&record) {
        if existing.class() != StorageClass::LocalOnly || existing.bytes() != response.bytes() {
            return Err(HumanOperationError::Refused);
        }
        let current = load_agent(store, tenant, agent_id)?;
        if current.session_id != session
            || current.session_token_id != token
            || current.session_generation != generation
        {
            return Err(HumanOperationError::Refused);
        }
        return Ok(response);
    }
    store
        .update_local_with_companion(
            agent_key(tenant, agent_id)?,
            encode(&agent)?,
            record,
            response.bytes().to_vec(),
        )
        .map_err(|_| HumanOperationError::Unavailable)?;
    Ok(response)
}

fn finalize_agent<F, R>(
    store: &mut Store,
    tenant: &TenantId,
    agent_id: &str,
    evidence: FinalizationEvidence,
    (expected_module, expected_operation, operation): (ModuleId, u8, &[u8]),
    mutate: F,
    response: R,
) -> Result<HumanResponse, HumanOperationError>
where
    F: FnOnce(&mut ManagedAgent) -> Result<(), HumanOperationError>,
    R: FnOnce(&ManagedAgent) -> Result<HumanResponse, HumanOperationError>,
{
    let validation = validate_finalization_source(
        store,
        tenant,
        agent_id,
        evidence,
        expected_module,
        expected_operation,
        operation,
    )?;
    let (aggregate_key, action_key, request_digest, mut agent) = match validation {
        FinalizationSource::Completed(response) => return Ok(response),
        FinalizationSource::Pending {
            aggregate_key,
            action_key,
            request_digest,
            agent,
        } => (aggregate_key, action_key, request_digest, agent),
    };
    mutate(&mut agent)?;
    agent.updated_at = evidence.finalized_at;
    if agent.verified_evidence.len() == MAX_EVIDENCE {
        agent.verified_evidence.remove(0);
    }
    agent.verified_evidence.push(evidence.receipt_digest);
    agent.validate()?;
    let response = response(&agent)?;
    let mut action = Vec::with_capacity(32 + response.bytes().len());
    action.extend_from_slice(&request_digest);
    action.extend_from_slice(response.bytes());
    let index = FinalizedAction {
        agent_id: agent_id.to_owned(),
        evidence,
        request_digest,
    };
    let index_bytes = serde_json::to_vec(&index).map_err(|_| HumanOperationError::Unavailable)?;
    store
        .commit_local_records(vec![
            (aggregate_key, encode(&agent)?),
            (action_key, action),
            (
                finalized_evidence_key(tenant, agent_id, evidence.receipt_digest)?,
                index_bytes.clone(),
            ),
            (
                finalized_transaction_key(tenant, evidence.activity_id)?,
                index_bytes,
            ),
        ])
        .map_err(|_| HumanOperationError::Unavailable)?;
    Ok(response)
}

enum FinalizationSource {
    Completed(HumanResponse),
    Pending {
        aggregate_key: crate::store::TenantKey,
        action_key: crate::store::TenantKey,
        request_digest: [u8; 32],
        agent: Box<ManagedAgent>,
    },
}

fn validate_finalization_source(
    store: &Store,
    tenant: &TenantId,
    agent_id: &str,
    evidence: FinalizationEvidence,
    expected_module: ModuleId,
    expected_operation: u8,
    operation: &[u8],
) -> Result<FinalizationSource, HumanOperationError> {
    if !valid_finalization_shape(evidence) {
        return Err(HumanOperationError::Refused);
    }
    let aggregate_key = agent_key(tenant, agent_id)?;
    let action_key = action_record_key(tenant, evidence.action_key)?;
    let request_digest = finalization_digest(agent_id, operation, evidence)?;
    if let Some(saved) = store.get(&action_key) {
        if saved.class() != StorageClass::LocalOnly
            || saved.bytes().len() < 32
            || saved.bytes()[..32] != request_digest
        {
            return Err(HumanOperationError::Refused);
        }
        return Ok(FinalizationSource::Completed(
            HumanResponse::new(saved.bytes()[32..].to_vec())
                .map_err(|_| HumanOperationError::Refused)?,
        ));
    }
    if store
        .get(&finalized_transaction_key(tenant, evidence.activity_id)?)
        .is_some()
    {
        return Err(HumanOperationError::Refused);
    }
    let served =
        crate::receipt::serve_committed_evidence(store, tenant.clone(), evidence.activity_id)
            .map_err(|_| HumanOperationError::Refused)?;
    let receipt =
        decode_receipt(&served.canonical_receipt).map_err(|_| HumanOperationError::Refused)?;
    let protocol = receipt.protocol().ok_or(HumanOperationError::Refused)?;
    let digest: [u8; 32] = Sha256::digest(&served.canonical_receipt).into();
    if !(matches_finalized_operation(
        protocol.module_id(),
        protocol.operation(),
        expected_module,
        expected_operation,
    ) || (protocol.module_id() == 12 && protocol.operation() == 2))
        || served.metadata.transaction_hash != evidence.activity_id
        || served.metadata.block_height != evidence.observed_sequence
        || !served.metadata.status
        || served.metadata.rung != layerx_types::rung::Rung::Committed
        || served.metadata.rung.wire_rank() != evidence.verification
        || digest != evidence.receipt_digest
    {
        return Err(HumanOperationError::Refused);
    }
    let value = store
        .get(&aggregate_key)
        .ok_or(HumanOperationError::Refused)?;
    if value.class() != StorageClass::LocalOnly {
        return Err(HumanOperationError::Refused);
    }
    let agent = decode(value.bytes())?;
    if evidence.finalized_at < agent.updated_at {
        return Err(HumanOperationError::Refused);
    }
    validate_projected_effect(store, tenant, &agent, operation, evidence, &receipt)?;
    Ok(FinalizationSource::Pending {
        aggregate_key,
        action_key,
        request_digest,
        agent: Box::new(agent),
    })
}

#[derive(serde::Serialize, serde::Deserialize)]
struct FinalizedAction {
    agent_id: String,
    evidence: FinalizationEvidence,
    request_digest: [u8; 32],
}

fn finalized_evidence_key(
    tenant: &TenantId,
    agent: &str,
    digest: [u8; 32],
) -> Result<crate::store::TenantKey, HumanOperationError> {
    key(
        tenant.clone(),
        ObjectKind::Audit,
        [
            b"managed-agent-evidence-v1:".as_slice(),
            &cursor_for(agent),
            &digest,
        ]
        .concat(),
    )
    .map_err(|_| HumanOperationError::Refused)
}
fn finalized_transaction_key(
    tenant: &TenantId,
    transaction: [u8; 32],
) -> Result<crate::store::TenantKey, HumanOperationError> {
    key(
        tenant.clone(),
        ObjectKind::Audit,
        [b"managed-agent-effect-v1:".as_slice(), &transaction].concat(),
    )
    .map_err(|_| HumanOperationError::Refused)
}

/// Serves complete committed evidence even after it leaves the bounded agent view.
pub fn finalized_evidence(
    store: &Store,
    tenant: &TenantId,
    agent: &str,
    digest: [u8; 32],
) -> Result<crate::receipt::TransactionReceiptEvidence, HumanOperationError> {
    load_agent(store, tenant, agent)?;
    let record = store
        .get(&finalized_evidence_key(tenant, agent, digest)?)
        .ok_or(HumanOperationError::Refused)?;
    let index: FinalizedAction =
        serde_json::from_slice(record.bytes()).map_err(|_| HumanOperationError::Refused)?;
    if record.class() != StorageClass::LocalOnly
        || index.agent_id != agent
        || index.evidence.receipt_digest != digest
    {
        return Err(HumanOperationError::Refused);
    }
    let served =
        crate::receipt::serve_committed_evidence(store, tenant.clone(), index.evidence.activity_id)
            .map_err(|_| HumanOperationError::Refused)?;
    if Sha256::digest(&served.canonical_receipt).as_slice() != digest
        || served.metadata.block_height != index.evidence.observed_sequence
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(served)
}

fn abi_word(data: &[u8], index: usize) -> Result<[u8; 32], HumanOperationError> {
    data.get(4 + index * 32..4 + (index + 1) * 32)
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or(HumanOperationError::Refused)
}
fn abi_integer(data: &[u8], index: usize, width: usize) -> Result<u128, HumanOperationError> {
    let word = abi_word(data, index)?;
    if word[..32 - width].iter().any(|byte| *byte != 0) {
        return Err(HumanOperationError::Refused);
    }
    let mut value = [0; 16];
    value[16 - width..].copy_from_slice(&word[32 - width..]);
    Ok(u128::from_be_bytes(value))
}
fn abi_dynamic(data: &[u8], words: usize, index: usize) -> Result<Vec<u8>, HumanOperationError> {
    let offset = abi_integer(data, index, 8)? as usize;
    if offset != words * 32 {
        return Err(HumanOperationError::Refused);
    }
    let length = abi_integer(data, words, 8)? as usize;
    if length > 4096 {
        return Err(HumanOperationError::Refused);
    }
    let end = 4 + offset + 32 + length;
    let padded = (end - 4 + 31) & !31;
    let bytes = data
        .get(4 + offset + 32..end)
        .ok_or(HumanOperationError::Refused)?;
    if data.len() != 4 + padded || data[end..].iter().any(|byte| *byte != 0) {
        return Err(HumanOperationError::Refused);
    }
    Ok(bytes.to_vec())
}

fn require_committed_log(
    receipt: &layerx_wire::receipt::Receipt,
    address: [u8; 20],
    event: &str,
    id: [u8; 32],
    data: &[u8],
) -> Result<(), HumanOperationError> {
    use layerx_crypto::evm_transaction::transaction_hash;
    let protocol = receipt.protocol().ok_or(HumanOperationError::Refused)?;
    let summary = layerx_proof::sequencer_logs::decode_summary(receipt)
        .map_err(|_| HumanOperationError::Refused)?;
    if summary.log_count() as usize + 1 != protocol.effects().len() {
        return Err(HumanOperationError::Refused);
    }
    let mut expected = address.to_vec();
    expected.push(2);
    expected.extend_from_slice(&transaction_hash(event.as_bytes()));
    expected.extend_from_slice(&id);
    expected.extend_from_slice(&Sha256::digest(data));
    if !protocol
        .effects()
        .iter()
        .enumerate()
        .skip(1)
        .any(|(index, effect)| {
            effect.module_id() == protocol.module_id()
                && effect.ordinal() as usize == index
                && effect.event_type() == 0
                && effect.kind() == 3
                && !effect.monetary()
                && effect.transfer_set_root() == [0; 32]
                && effect.body() == expected
        })
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(())
}

fn validate_projected_effect(
    store: &Store,
    tenant: &TenantId,
    agent: &ManagedAgent,
    operation: &[u8],
    evidence: FinalizationEvidence,
    receipt: &layerx_wire::receipt::Receipt,
) -> Result<(), HumanOperationError> {
    use crate::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    let mut outbox = crate::outbox::Outbox::default();
    outbox
        .restore(store, tenant.clone(), evidence.activity_id)
        .map_err(|_| HumanOperationError::Refused)?;
    let raw = outbox
        .exact_signed_bytes(evidence.activity_id)
        .map_err(|_| HumanOperationError::Refused)?;
    let (call, signature) = decode_signed(raw).map_err(|_| HumanOperationError::Refused)?;
    let sender = recover_sender(&call, &signature).map_err(|_| HumanOperationError::Refused)?;
    let authority = layerx_agent_api::identity::strict_hex32(
        &agent.context.primary_authority,
        "primary_authority",
    )
    .map_err(|_| HumanOperationError::Refused)?;
    let protocol = receipt.protocol().ok_or(HumanOperationError::Refused)?;
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    if authority != from
        || call.chain_id != u64::from(agent.context.network_id)
        || protocol.from() != from
        || protocol.to() != to
        || agent.context.actor != format!("did:layerx:{}", hex(&agent.context.custody_public_key))
    {
        return Err(HumanOperationError::Refused);
    }
    match operation.first().copied() {
        Some(27) => {
            let amount = u128::from_be_bytes(
                operation
                    .get(1..17)
                    .ok_or(HumanOperationError::Refused)?
                    .try_into()
                    .map_err(|_| HumanOperationError::Refused)?,
            );
            let budget: [u8; 32] = operation
                .get(
                    operation
                        .len()
                        .checked_sub(32)
                        .ok_or(HumanOperationError::Refused)?..,
                )
                .ok_or(HumanOperationError::Refused)?
                .try_into()
                .map_err(|_| HumanOperationError::Refused)?;
            let registry = layerx_types::payload::ModuleRegistry::new(&[])
                .map_err(|_| HumanOperationError::Refused)?;
            let identity = crate::budget::budget_create_identity(raw, &registry)
                .map_err(|_| HumanOperationError::Refused)?;
            let account = layerx_types::account::AccountId::parse(&agent.context.budget_account)
                .map_err(|_| HumanOperationError::Refused)?;
            let budget_account = layerx_wire::hash::account_id_for_protocol(&account, 3)
                .map_err(|_| HumanOperationError::Refused)?;
            let expected_budget: [u8; 32] = Sha256::new()
                .chain_update(b"layerx-human/agent-limit-budget/v1")
                .chain_update(agent.active_budget_id)
                .chain_update(evidence.action_key)
                .finalize()
                .into();
            if identity.budget_id != budget
                || budget != expected_budget
                || identity.budget_account != budget_account
                || identity.asset != agent.context.budget_asset
                || identity.purpose != agent.context.purpose_hash
                || identity.per_period_limit != amount
                || identity.carry_cap != 0
                || identity.rollover != 1
                || identity.period_length_ms
                    != agent
                        .context
                        .budget_period_seconds
                        .checked_mul(1000)
                        .ok_or(HumanOperationError::Refused)?
                || operation.get(17..operation.len() - 32) != Some(agent.currency.as_bytes())
            {
                return Err(HumanOperationError::Refused);
            }
            let mut data = identity.asset.to_vec();
            data.extend_from_slice(&[0; 16]);
            data.extend_from_slice(&identity.initial_amount.to_be_bytes());
            require_committed_log(
                receipt,
                call.to,
                "BudgetCreated(bytes32,bytes32,uint128)",
                budget,
                &data,
            )
        }
        Some(26) if operation.get(1) == Some(&1) => {
            if call.to != PrecompileTarget::Auth.address()
                || call.data.get(..4)
                    != Some(
                        layerx_crypto::disclosure::call::ledger_selector(
                            "grantSession(bytes,uint64,bytes32)",
                        )
                        .as_slice(),
                    )
            {
                return Err(HumanOperationError::Refused);
            }
            let grant_bytes = abi_dynamic(&call.data, 3, 0)?;
            let grant = layerx_crypto::session::decode_session_key(&grant_bytes)
                .map_err(|_| HumanOperationError::Refused)?;
            if grant.grant_id != agent.protocol_grant_id
                || abi_word(&call.data, 2)? != evidence.action_key
                || grant.grantor != parse_agent_digest(&agent.agent_id)?
                || abi_integer(&call.data, 1, 8)? == 0
            {
                return Err(HumanOperationError::Refused);
            }
            let body = PrecompileCall {
                target: PrecompileTarget::Auth,
                selector: [0; 4],
                arguments: vec![AbiArgument::Bytes(grant_bytes)],
            }
            .encode()
            .map_err(|_| HumanOperationError::Refused)?;
            require_committed_log(
                receipt,
                call.to,
                "SessionGranted(bytes32,bytes)",
                grant.grant_id,
                &body[4..],
            )
        }
        Some(26) | Some(29) => {
            let reason = if operation[0] == 29 { 2 } else { 1 };
            if call.to != PrecompileTarget::Auth.address()
                || call.data.len() != 100
                || call.data[..4]
                    != layerx_crypto::disclosure::call::ledger_selector(
                        "revokeGrant(bytes32,uint8,uint64)",
                    )
                || abi_word(&call.data, 0)? != agent.protocol_grant_id
                || abi_integer(&call.data, 1, 1)? != reason
            {
                return Err(HumanOperationError::Refused);
            }
            let sequence = abi_integer(&call.data, 2, 8)? as u64;
            if sequence == 0 || sequence > evidence.observed_sequence {
                return Err(HumanOperationError::Refused);
            }
            let mut revoked = agent.protocol_grant_id.to_vec();
            revoked.push(reason as u8);
            revoked.extend_from_slice(&sequence.to_be_bytes());
            let body = PrecompileCall {
                target: PrecompileTarget::Auth,
                selector: [0; 4],
                arguments: vec![AbiArgument::Bytes(revoked)],
            }
            .encode()
            .map_err(|_| HumanOperationError::Refused)?;
            require_committed_log(
                receipt,
                call.to,
                "GrantRevoked(bytes32,bytes)",
                agent.protocol_grant_id,
                &body[4..],
            )
        }
        Some(28) if operation.get(1) == Some(&0) => {
            let amount = u128::from_be_bytes(
                operation
                    .get(2..18)
                    .ok_or(HumanOperationError::Refused)?
                    .try_into()
                    .map_err(|_| HumanOperationError::Refused)?,
            );
            if call.to != PrecompileTarget::Pay.address()
                || call.data.len() != 68
                || call.data[..4]
                    != layerx_crypto::disclosure::call::ledger_selector(
                        "budgetDefund(bytes32,uint128)",
                    )
                || abi_word(&call.data, 0)? != agent.active_budget_id
                || abi_integer(&call.data, 1, 16)? != amount
            {
                return Err(HumanOperationError::Refused);
            }
            let remaining_limit = agent
                .monthly_limit
                .checked_sub(amount)
                .ok_or(HumanOperationError::Refused)?;
            let data = [
                vec![0; 16],
                amount.to_be_bytes().to_vec(),
                vec![0; 16],
                remaining_limit.to_be_bytes().to_vec(),
            ]
            .concat();
            require_committed_log(
                receipt,
                call.to,
                "BudgetDefunded(bytes32,uint128,uint128)",
                agent.active_budget_id,
                &data,
            )
        }
        Some(28) => {
            // Bind the exact canonical delayed-key payload, including its challenge window.
            if call.to != PrecompileTarget::Auth.address()
                || call.data.get(..4)
                    != Some(
                        layerx_crypto::disclosure::call::ledger_selector(
                            "recordLifecycle(bytes,bytes32)",
                        )
                        .as_slice(),
                    )
                || abi_word(&call.data, 1)? != evidence.action_key
            {
                return Err(HumanOperationError::Refused);
            }
            let payload = abi_dynamic(&call.data, 2, 0)?;
            let did = layerx_types::ids::Did::new(agent.context.actor.as_bytes())
                .map_err(|_| HumanOperationError::Refused)?;
            let owner = layerx_wire::hash::did_id_for_protocol(&did, 3)
                .map_err(|_| HumanOperationError::Refused)?;
            let tail = operation
                .len()
                .checked_sub(80)
                .ok_or(HumanOperationError::Refused)?;
            let delay = u64::from_be_bytes(
                operation
                    .get(tail..tail + 8)
                    .ok_or(HumanOperationError::Refused)?
                    .try_into()
                    .map_err(|_| HumanOperationError::Refused)?,
            );
            let ready = u64::from_be_bytes(
                operation
                    .get(tail + 8..tail + 16)
                    .ok_or(HumanOperationError::Refused)?
                    .try_into()
                    .map_err(|_| HumanOperationError::Refused)?,
            );
            if payload.len() != 92
                || payload[..4] != [0x71, 2, 0, 4]
                || payload[4..36] != owner
                || payload[36..68] == [0; 32]
                || payload[68..76] != evidence.finalized_at.to_be_bytes()
                || payload[76..84] != ready.to_be_bytes()
                || ready.checked_sub(evidence.finalized_at) != Some(delay)
                || payload[84..92] != evidence.observed_sequence.to_be_bytes()
            {
                return Err(HumanOperationError::Refused);
            }
            Ok(())
        }
        _ => Err(HumanOperationError::Refused),
    }
}

fn valid_finalization_shape(evidence: FinalizationEvidence) -> bool {
    evidence.action_key != [0; 32]
        && evidence.activity_id != [0; 32]
        && evidence.receipt_digest != [0; 32]
        && evidence.observed_sequence != 0
        && evidence.verification == layerx_types::rung::Rung::Committed.wire_rank()
        && evidence.finalized_at != 0
}

fn matches_finalized_operation(
    module: u16,
    operation: u8,
    expected_module: ModuleId,
    expected_operation: u8,
) -> bool {
    module == expected_module as u16 && operation == expected_operation
}

fn action_record_key(
    tenant: &TenantId,
    action: [u8; 32],
) -> Result<crate::store::TenantKey, HumanOperationError> {
    let mut id = b"managed-agent-action-v1:".to_vec();
    id.extend_from_slice(&action);
    key(tenant.clone(), ObjectKind::Idempotency, id).map_err(|_| HumanOperationError::Refused)
}
fn finalization_digest(
    agent: &str,
    operation: &[u8],
    evidence: FinalizationEvidence,
) -> Result<[u8; 32], HumanOperationError> {
    let mut h = Sha256::new();
    h.update(b"layerx-agentd/managed-agent-finalize/v1\0");
    h.update(
        u32::try_from(agent.len())
            .map_err(|_| HumanOperationError::Refused)?
            .to_be_bytes(),
    );
    h.update(agent.as_bytes());
    h.update(
        u32::try_from(operation.len())
            .map_err(|_| HumanOperationError::Refused)?
            .to_be_bytes(),
    );
    h.update(operation);
    h.update(evidence.action_key);
    h.update(evidence.activity_id);
    h.update(evidence.receipt_digest);
    h.update(evidence.observed_sequence.to_be_bytes());
    h.update([evidence.verification]);
    h.update(evidence.finalized_at.to_be_bytes());
    Ok(h.finalize().into())
}
fn journey_response(
    agent: &ManagedAgent,
    kind: u8,
    evidence: FinalizationEvidence,
) -> Result<HumanResponse, HumanOperationError> {
    let label = match kind {
        0 => "reclaim",
        1 => "rotate",
        2 => "recover",
        3 => "archive",
        _ => return Err(HumanOperationError::Refused),
    };
    let mut out = Wire::new();
    out.text(&format!("jrn_{}", hex(&evidence.action_key)))?;
    out.text(label)?;
    out.u8(2);
    out.u8(1);
    out.text("receipt-finalized")?;
    out.text("agent.journey.receipt-finalized")?;
    out.u8(2);
    out.u8(1);
    out.text(&hex(&evidence.receipt_digest))?;
    out.text(label)?;
    out.u8(evidence.verification);
    out.text(&evidence.finalized_at.to_string())?;
    out.text(&agent.updated_at.to_string())?;
    out.u8(1);
    out.text(&hex(&evidence.receipt_digest))?;
    out.text(label)?;
    out.u8(evidence.verification);
    out.finish()
}
fn challenge_response(
    agent: &ManagedAgent,
    kind: u8,
    delay: u64,
    ready_at: u64,
    evidence: FinalizationEvidence,
) -> Result<HumanResponse, HumanOperationError> {
    let mut out = Wire::new();
    out.text(&agent.agent_id)?;
    out.u8(kind - 1);
    out.u64(delay);
    out.text(&ready_at.to_string())?;
    out.u8(1);
    out.text(&hex(&evidence.receipt_digest))?;
    out.text(if kind == 1 { "rotate" } else { "recover" })?;
    out.u8(evidence.verification);
    out.finish()
}

/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn list(
    store: &Store,
    tenant: &TenantId,
    cursor: Option<[u8; 32]>,
    limit: u8,
) -> Result<HumanResponse, HumanOperationError> {
    if limit == 0 || limit > MAX_AGENTS_PER_PAGE {
        return Err(HumanOperationError::Refused);
    }
    let mut agents = Vec::new();
    for object_id in store.list_object_ids(tenant, ObjectKind::Configuration) {
        if !object_id.starts_with(PREFIX) {
            continue;
        }
        let object_key = key(tenant.clone(), ObjectKind::Configuration, object_id)
            .map_err(|_| HumanOperationError::Refused)?;
        let value = store
            .get(&object_key)
            .ok_or(HumanOperationError::Unavailable)?;
        if value.class() != StorageClass::LocalOnly {
            return Err(HumanOperationError::Refused);
        }
        agents.push(decode(value.bytes())?);
    }
    let start = match cursor {
        None => 0,
        Some(cursor) => agents
            .iter()
            .position(|agent| cursor_for(&agent.agent_id) == cursor)
            .map(|index| index + 1)
            .ok_or(HumanOperationError::Refused)?,
    };
    let end = start.saturating_add(usize::from(limit)).min(agents.len());
    let page = &agents[start..end];
    let mut out = Wire::new();
    out.u8(u8::try_from(page.len()).map_err(|_| HumanOperationError::Refused)?);
    for agent in page {
        encode_response_agent(&mut out, agent)?;
    }
    if end < agents.len() {
        out.u8(1);
        out.fixed(&cursor_for(&agents[end - 1].agent_id));
    } else {
        out.u8(0);
    }
    out.finish()
}

fn agent_key(
    tenant: &TenantId,
    agent_id: &str,
) -> Result<crate::store::TenantKey, HumanOperationError> {
    if agent_id.is_empty() || agent_id.len() > 512 {
        return Err(HumanOperationError::Refused);
    }
    let mut object_id = Vec::with_capacity(PREFIX.len() + agent_id.len());
    object_id.extend_from_slice(PREFIX);
    object_id.extend_from_slice(agent_id.as_bytes());
    key(tenant.clone(), ObjectKind::Configuration, object_id)
        .map_err(|_| HumanOperationError::Refused)
}

fn cursor_for(agent_id: &str) -> [u8; 32] {
    Sha256::digest(
        [
            b"layerx-agentd/managed-agent-cursor/v1\0".as_slice(),
            agent_id.as_bytes(),
        ]
        .concat(),
    )
    .into()
}

fn response_agent(agent: &ManagedAgent) -> Result<HumanResponse, HumanOperationError> {
    let mut out = Wire::new();
    encode_response_agent(&mut out, agent)?;
    out.finish()
}
fn encode_response_agent(out: &mut Wire, agent: &ManagedAgent) -> Result<(), HumanOperationError> {
    agent.validate()?;
    out.text(&agent.agent_id)?;
    out.text(&agent.name)?;
    out.text(&agent.purpose)?;
    out.u8(agent.state);
    out.u128(agent.monthly_limit);
    out.text(&agent.currency)?;
    out.u8(0);
    out.text(&agent.period_start.to_string())?;
    out.text(&agent.period_end.to_string())?;
    out.u128(agent.spent);
    out.u128(agent.monthly_limit - agent.spent);
    out.u8(3);
    out.text(&agent.created_at.to_string())?;
    out.text(&agent.updated_at.to_string())?;
    out.u8(u8::try_from(agent.verified_evidence.len()).map_err(|_| HumanOperationError::Refused)?);
    for evidence in &agent.verified_evidence {
        out.text(&hex(evidence))?;
        out.text("agent-creation")?;
        out.u8(3);
    }
    Ok(())
}

fn encode(agent: &ManagedAgent) -> Result<Vec<u8>, HumanOperationError> {
    let mut out = Wire::new();
    out.u8(VERSION);
    out.text(&agent.agent_id)?;
    out.text(&agent.name)?;
    out.text(&agent.purpose)?;
    out.u8(agent.state);
    out.u128(agent.monthly_limit);
    out.text(&agent.currency)?;
    out.u64(agent.period_start);
    out.u64(agent.period_end);
    out.u128(agent.spent);
    out.u64(agent.created_at);
    out.u64(agent.updated_at);
    out.u8(u8::try_from(agent.verified_evidence.len()).map_err(|_| HumanOperationError::Refused)?);
    for value in &agent.verified_evidence {
        out.fixed(value);
    }
    encode_context(&mut out, &agent.context)?;
    out.text(&agent.agent_did)?;
    out.fixed(&agent.session_id);
    out.fixed(&agent.session_token_id);
    out.u64(agent.session_generation);
    out.fixed(&agent.protocol_grant_id);
    out.fixed(&agent.active_budget_id);
    Ok(out.0)
}
fn decode(bytes: &[u8]) -> Result<ManagedAgent, HumanOperationError> {
    let mut input = Input { bytes, offset: 0 };
    let version = input.u8()?;
    if version != VERSION && version != LEGACY_VERSION {
        return Err(HumanOperationError::Refused);
    }
    let agent = ManagedAgent {
        agent_id: input.text()?,
        name: input.text()?,
        purpose: input.text()?,
        state: input.u8()?,
        monthly_limit: input.u128()?,
        currency: input.text()?,
        period_start: input.u64()?,
        period_end: input.u64()?,
        spent: input.u128()?,
        created_at: input.u64()?,
        updated_at: input.u64()?,
        verified_evidence: {
            let count = usize::from(input.u8()?);
            if count == 0 || count > MAX_EVIDENCE {
                return Err(HumanOperationError::Refused);
            }
            let mut values = Vec::with_capacity(count);
            for _ in 0..count {
                values.push(input.fixed()?);
            }
            values
        },
        context: decode_context(&mut input)?,
        agent_did: input.text()?,
        session_id: input.fixed()?,
        session_token_id: input.fixed()?,
        session_generation: if version == VERSION { input.u64()? } else { 1 },
        protocol_grant_id: input.fixed()?,
        active_budget_id: input.fixed()?,
    };
    if input.offset != bytes.len() {
        return Err(HumanOperationError::Refused);
    }
    agent.validate()?;
    Ok(agent)
}

struct Wire(Vec<u8>);
impl Wire {
    fn new() -> Self {
        Self(Vec::new())
    }
    fn u8(&mut self, value: u8) {
        self.0.push(value);
    }
    fn u16(&mut self, value: u16) {
        self.0.extend_from_slice(&value.to_be_bytes());
    }
    fn u32(&mut self, value: u32) {
        self.0.extend_from_slice(&value.to_be_bytes());
    }
    fn u64(&mut self, value: u64) {
        self.0.extend_from_slice(&value.to_be_bytes());
    }
    fn u128(&mut self, value: u128) {
        self.0.extend_from_slice(&value.to_be_bytes());
    }
    fn fixed(&mut self, value: &[u8]) {
        self.0.extend_from_slice(value);
    }
    fn text(&mut self, value: &str) -> Result<(), HumanOperationError> {
        if value.is_empty() || value.len() > 4096 {
            return Err(HumanOperationError::Refused);
        }
        self.0.extend_from_slice(
            &u32::try_from(value.len())
                .map_err(|_| HumanOperationError::Refused)?
                .to_be_bytes(),
        );
        self.fixed(value.as_bytes());
        Ok(())
    }
    fn finish(self) -> Result<HumanResponse, HumanOperationError> {
        HumanResponse::new(self.0).map_err(|_| HumanOperationError::Refused)
    }
}
struct Input<'a> {
    bytes: &'a [u8],
    offset: usize,
}
impl Input<'_> {
    fn take<const N: usize>(&mut self) -> Result<[u8; N], HumanOperationError> {
        let end = self
            .offset
            .checked_add(N)
            .ok_or(HumanOperationError::Refused)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(HumanOperationError::Refused)?
            .try_into()
            .map_err(|_| HumanOperationError::Refused)?;
        self.offset = end;
        Ok(value)
    }
    fn u8(&mut self) -> Result<u8, HumanOperationError> {
        Ok(self.take::<1>()?[0])
    }
    fn u16(&mut self) -> Result<u16, HumanOperationError> {
        Ok(u16::from_be_bytes(self.take()?))
    }
    fn u32(&mut self) -> Result<u32, HumanOperationError> {
        Ok(u32::from_be_bytes(self.take()?))
    }
    fn u64(&mut self) -> Result<u64, HumanOperationError> {
        Ok(u64::from_be_bytes(self.take()?))
    }
    fn u128(&mut self) -> Result<u128, HumanOperationError> {
        Ok(u128::from_be_bytes(self.take()?))
    }
    fn fixed(&mut self) -> Result<[u8; 32], HumanOperationError> {
        self.take()
    }
    fn text(&mut self) -> Result<String, HumanOperationError> {
        let length = u32::from_be_bytes(self.take()?) as usize;
        if length == 0 || length > 4096 {
            return Err(HumanOperationError::Refused);
        }
        let end = self
            .offset
            .checked_add(length)
            .ok_or(HumanOperationError::Refused)?;
        let value = std::str::from_utf8(
            self.bytes
                .get(self.offset..end)
                .ok_or(HumanOperationError::Refused)?,
        )
        .map_err(|_| HumanOperationError::Refused)?
        .to_owned();
        self.offset = end;
        Ok(value)
    }
    fn bytes(&mut self, length: usize) -> Result<&[u8], HumanOperationError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(HumanOperationError::Refused)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(HumanOperationError::Refused)?;
        self.offset = end;
        Ok(value)
    }
}
fn hex(value: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(value.len() * 2);
    for byte in value {
        out.push(char::from(DIGITS[usize::from(byte >> 4)]));
        out.push(char::from(DIGITS[usize::from(byte & 15)]));
    }
    out
}
fn parse_agent_digest(value: &str) -> Result<[u8; 32], HumanOperationError> {
    let encoded = value
        .strip_prefix("agt_")
        .ok_or(HumanOperationError::Refused)?;
    if encoded.len() != 64 {
        return Err(HumanOperationError::Refused);
    }
    let mut out = [0u8; 32];
    for (index, pair) in encoded.as_bytes().chunks_exact(2).enumerate() {
        let digit = |b| match b {
            b'0'..=b'9' => Some(b - b'0'),
            b'a'..=b'f' => Some(b - b'a' + 10),
            _ => None,
        };
        out[index] = (digit(pair[0]).ok_or(HumanOperationError::Refused)? << 4)
            | digit(pair[1]).ok_or(HumanOperationError::Refused)?;
    }
    if out == [0; 32] {
        return Err(HumanOperationError::Refused);
    }
    Ok(out)
}
fn validate_context(c: &HumanAgentLifecycleSeed) -> Result<(), HumanOperationError> {
    if c.actor.is_empty()
        || c.primary_authority.is_empty()
        || c.custody_key.is_empty()
        || c.custody_public_key == [0; 32]
        || c.owner_account.is_empty()
        || c.budget_account.is_empty()
        || c.budget_asset == [0; 32]
        || c.purpose_hash == [0; 32]
        || c.recovery_root == [0; 32]
        || c.recovery_threshold == 0
        || c.capability_id == [0; 32]
        || c.activity_types.is_empty()
        || c.counterparties.is_empty()
        || c.assets.is_empty()
        || c.amount_ceiling == 0
        || c.rate_maximum_uses == 0
        || c.rate_window_sequences == 0
        || c.purposes.is_empty()
        || c.capability_expiry_sequence == 0
        || c.session_scopes.is_empty()
        || c.session_expiry_unix_seconds == 0
        || c.budget_period_seconds == 0
        || c.budget_expiry_seconds == 0
        || c.initial_funding == 0
        || c.network_id == 0
        || c.creation_receipt_roots.is_empty()
    {
        return Err(HumanOperationError::Refused);
    }
    Ok(())
}
fn encode_context(out: &mut Wire, c: &HumanAgentLifecycleSeed) -> Result<(), HumanOperationError> {
    out.text(&c.agent_id)?;
    out.text(&c.name)?;
    out.text(&c.purpose)?;
    out.text(&c.currency)?;
    out.u128(c.monthly_limit);
    out.u64(c.period_start);
    out.u64(c.period_end);
    out.u64(c.created_at);
    out.u64(c.updated_at);
    fixeds(out, &c.verified_evidence)?;
    out.text(&c.actor)?;
    out.text(&c.primary_authority)?;
    out.text(&c.custody_key)?;
    out.fixed(&c.custody_public_key);
    out.text(&c.owner_account)?;
    out.text(&c.budget_account)?;
    out.fixed(&c.budget_asset);
    out.fixed(&c.purpose_hash);
    out.fixed(&c.recovery_root);
    out.u16(c.recovery_threshold);
    out.fixed(&c.capability_id);
    u32s(out, &c.activity_types)?;
    fixeds(out, &c.counterparties)?;
    fixeds(out, &c.assets)?;
    out.u128(c.amount_ceiling);
    out.u64(c.rate_maximum_uses);
    out.u64(c.rate_window_sequences);
    texts(out, &c.purposes)?;
    out.u64(c.capability_expiry_sequence);
    texts(out, &c.session_scopes)?;
    out.u64(c.session_expiry_unix_seconds);
    out.fixed(&c.protocol_grant_id);
    out.u64(c.budget_period_seconds);
    out.u64(c.budget_expiry_seconds);
    out.u128(c.initial_funding);
    out.u32(c.network_id);
    fixeds(out, &c.creation_receipt_roots)
}
/// # Errors
/// Returns an error when the request is invalid, authority is refused, or required state is unavailable.
pub fn lifecycle_publish_digest(
    c: &HumanAgentLifecycleSeed,
) -> Result<[u8; 32], HumanOperationError> {
    let mut wire = Wire::new();
    encode_context(&mut wire, c)?;
    let mut digest = Sha256::new();
    digest.update(b"layerx-human-agent-lifecycle-publish/v1");
    digest.update(&wire.0);
    Ok(digest.finalize().into())
}
fn decode_context(i: &mut Input) -> Result<HumanAgentLifecycleSeed, HumanOperationError> {
    Ok(HumanAgentLifecycleSeed {
        agent_id: i.text()?,
        name: i.text()?,
        purpose: i.text()?,
        currency: i.text()?,
        monthly_limit: i.u128()?,
        period_start: i.u64()?,
        period_end: i.u64()?,
        created_at: i.u64()?,
        updated_at: i.u64()?,
        verified_evidence: read_fixeds(i, 64)?,
        actor: i.text()?,
        primary_authority: i.text()?,
        custody_key: i.text()?,
        custody_public_key: i.fixed()?,
        owner_account: i.text()?,
        budget_account: i.text()?,
        budget_asset: i.fixed()?,
        purpose_hash: i.fixed()?,
        recovery_root: i.fixed()?,
        recovery_threshold: i.u16()?,
        capability_id: i.fixed()?,
        activity_types: read_u32s(i, 256)?,
        counterparties: read_fixeds(i, 256)?,
        assets: read_fixeds(i, 256)?,
        amount_ceiling: i.u128()?,
        rate_maximum_uses: i.u64()?,
        rate_window_sequences: i.u64()?,
        purposes: read_texts(i, 64)?,
        capability_expiry_sequence: i.u64()?,
        session_scopes: read_texts(i, 64)?,
        session_expiry_unix_seconds: i.u64()?,
        protocol_grant_id: i.fixed()?,
        budget_period_seconds: i.u64()?,
        budget_expiry_seconds: i.u64()?,
        initial_funding: i.u128()?,
        network_id: i.u32()?,
        creation_receipt_roots: read_fixeds(i, 64)?,
    })
}
fn fixeds(out: &mut Wire, v: &[[u8; 32]]) -> Result<(), HumanOperationError> {
    out.u16(u16::try_from(v.len()).map_err(|_| HumanOperationError::Refused)?);
    for x in v {
        out.fixed(x);
    }
    Ok(())
}
fn u32s(out: &mut Wire, v: &[u32]) -> Result<(), HumanOperationError> {
    out.u16(u16::try_from(v.len()).map_err(|_| HumanOperationError::Refused)?);
    for x in v {
        out.u32(*x);
    }
    Ok(())
}
fn texts(out: &mut Wire, v: &[String]) -> Result<(), HumanOperationError> {
    out.u16(u16::try_from(v.len()).map_err(|_| HumanOperationError::Refused)?);
    for x in v {
        out.text(x)?;
    }
    Ok(())
}
fn read_fixeds(i: &mut Input, max: usize) -> Result<Vec<[u8; 32]>, HumanOperationError> {
    let n = usize::from(i.u16()?);
    if n == 0 || n > max {
        return Err(HumanOperationError::Refused);
    }
    let mut v = Vec::with_capacity(n);
    for _ in 0..n {
        v.push(i.fixed()?);
    }
    Ok(v)
}
fn read_u32s(i: &mut Input, max: usize) -> Result<Vec<u32>, HumanOperationError> {
    let n = usize::from(i.u16()?);
    if n == 0 || n > max {
        return Err(HumanOperationError::Refused);
    }
    let mut v = Vec::with_capacity(n);
    for _ in 0..n {
        v.push(i.u32()?);
    }
    Ok(v)
}
fn read_texts(i: &mut Input, max: usize) -> Result<Vec<String>, HumanOperationError> {
    let n = usize::from(i.u16()?);
    if n == 0 || n > max {
        return Err(HumanOperationError::Refused);
    }
    let mut v = Vec::with_capacity(n);
    for _ in 0..n {
        v.push(i.text()?);
    }
    Ok(v)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn evidence(verification: u8) -> FinalizationEvidence {
        FinalizationEvidence {
            action_key: [1; 32],
            activity_id: [1; 32],
            receipt_digest: [2; 32],
            observed_sequence: 1,
            verification,
            finalized_at: 1,
        }
    }

    #[test]
    fn finalization_shape_requires_checkpoint_rank() {
        use layerx_types::rung::Rung;
        assert!(!valid_finalization_shape(evidence(
            Rung::Submitted.wire_rank()
        )));
        assert!(!valid_finalization_shape(evidence(
            Rung::Receipted.wire_rank()
        )));
        assert!(valid_finalization_shape(evidence(
            Rung::Committed.wire_rank()
        )));
        for retired in [3, 4, 5] {
            assert!(!valid_finalization_shape(evidence(retired)));
        }
    }

    #[test]
    fn lifecycle_module_and_operation_must_both_match() {
        assert!(matches_finalized_operation(
            ModuleId::Budget as u16,
            1,
            ModuleId::Budget,
            1,
        ));
        assert!(!matches_finalized_operation(
            ModuleId::Governance as u16,
            1,
            ModuleId::Budget,
            1,
        ));
        assert!(!matches_finalized_operation(
            ModuleId::Budget as u16,
            7,
            ModuleId::Budget,
            1,
        ));
    }
}

/// The v1 digest domain versions the exact observation bytes, including generation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SessionObservationV1 {
    pub agent_id: String,
    pub did: String,
    pub session: [u8; 32],
    pub token: [u8; 32],
    pub generation: u64,
    pub open: bool,
    pub action: [u8; 32],
    pub digest: [u8; 32],
}
impl SessionObservationV1 {
    pub fn encode(&self) -> Result<(HumanResponse, [u8; 32]), HumanOperationError> {
        if self.session == [0; 32]
            || self.token == [0; 32]
            || self.generation == 0
            || self.action == [0; 32]
        {
            return Err(HumanOperationError::Refused);
        }
        let mut out = Wire::new();
        out.text(&self.agent_id)?;
        out.text(&self.did)?;
        out.fixed(&self.session);
        out.fixed(&self.token);
        out.u64(self.generation);
        out.u8(u8::from(self.open));
        out.fixed(&self.action);
        let digest: [u8; 32] = Sha256::digest(
            [
                b"layerx-agentd/session-observation/v1\0".as_slice(),
                out.0.as_slice(),
            ]
            .concat(),
        )
        .into();
        out.fixed(&digest);
        Ok((out.finish()?, digest))
    }
    pub fn decode(bytes: &[u8]) -> Result<Self, HumanOperationError> {
        let mut input = Input { bytes, offset: 0 };
        let value = Self {
            agent_id: input.text()?,
            did: input.text()?,
            session: input.fixed()?,
            token: input.fixed()?,
            generation: input.u64()?,
            open: match input.u8()? {
                0 => false,
                1 => true,
                _ => return Err(HumanOperationError::Refused),
            },
            action: input.fixed()?,
            digest: input.fixed()?,
        };
        let (encoded, digest) = value.encode()?;
        if input.offset != bytes.len() || value.digest != digest || encoded.bytes() != bytes {
            return Err(HumanOperationError::Refused);
        }
        Ok(value)
    }
}

/// Applies already authenticated Human lifecycle requests to the durable owner.
/// Transport authentication and cross-owner session invalidation remain with UnifiedAgentOwner.
pub fn apply_lifecycle_request(
    store: &mut Store,
    tenant: &TenantId,
    request: crate::human::HumanRequest,
) -> Result<HumanResponse, HumanOperationError> {
    use crate::human::{HumanAgentJourneyKind, HumanRequest};
    match request {
        HumanRequest::AgentControl {
            agent_id,
            resume,
            session_observation,
            evidence,
        } => finalize_control(
            store,
            tenant,
            &agent_id,
            resume,
            session_observation,
            evidence.into(),
        ),
        HumanRequest::AgentLimit {
            agent_id,
            monthly_limit,
            currency,
            replacement_budget_id,
            evidence,
        } => finalize_limit(
            store,
            tenant,
            &agent_id,
            monthly_limit,
            &currency,
            replacement_budget_id,
            evidence.into(),
        ),
        HumanRequest::AgentJourney {
            agent_id,
            kind,
            pre_observation,
            post_observation,
            evidence,
        } => {
            let (kind, amount, currency, delay, ready) = match kind {
                HumanAgentJourneyKind::Reclaim { amount, currency } => (0, amount, currency, 0, 0),
                HumanAgentJourneyKind::Rotate {
                    challenge_delay_seconds,
                    ready_at,
                } => (1, 0, String::new(), challenge_delay_seconds, ready_at),
                HumanAgentJourneyKind::Recover {
                    challenge_delay_seconds,
                    ready_at,
                } => (2, 0, String::new(), challenge_delay_seconds, ready_at),
                _ => return Err(HumanOperationError::Refused),
            };
            finalize_journey(
                store,
                tenant,
                &agent_id,
                (kind, amount, &currency, delay, ready),
                pre_observation,
                post_observation,
                evidence.into(),
            )
        }
        HumanRequest::AgentArchive {
            agent_id,
            confirm_name,
            pre_observation,
            post_observation,
            session_observation,
            evidence,
        } => finalize_archive(
            store,
            tenant,
            &agent_id,
            &confirm_name,
            (pre_observation, post_observation, session_observation),
            evidence.into(),
        ),
        _ => Err(HumanOperationError::Refused),
    }
}

pub fn record_budget_snapshot(
    store: &mut Store,
    tenant: &TenantId,
    budget: [u8; 32],
    state: &crate::human_runtime::CoreBudgetState,
) -> Result<HumanResponse, HumanOperationError> {
    let bytes = state
        .encode_snapshot(budget)
        .map_err(|_| HumanOperationError::Refused)?;
    let response = HumanResponse::new(bytes).map_err(|_| HumanOperationError::Refused)?;
    record_observation(
        store,
        tenant,
        1,
        state.evidence_digest,
        response.bytes().to_vec(),
    )?;
    Ok(response)
}
pub fn record_key_policy_snapshot(
    store: &mut Store,
    tenant: &TenantId,
    did: &str,
    recovery: bool,
    state: &crate::human_runtime::CoreKeyPolicy,
) -> Result<HumanResponse, HumanOperationError> {
    let mut out = Wire::new();
    out.text(did)?;
    out.u8(u8::from(recovery));
    out.u64(state.policy_revision);
    out.u64(state.required_delay_seconds);
    out.u64(state.maximum_delay_seconds);
    out.u64(state.effective_sequence);
    out.u64(state.observed_head_sequence);
    out.u8(state.verification);
    out.fixed(&state.evidence_digest);
    out.fixed(&state.checkpoint_digest);
    out.u64(state.age_sequences);
    out.u64(state.maximum_age_sequences);
    let response = out.finish()?;
    record_observation(
        store,
        tenant,
        2,
        state.evidence_digest,
        response.bytes().to_vec(),
    )?;
    Ok(response)
}

/// Advances both the restored session and managed binding atomically before emitting its observation.
pub fn suspend_session(
    store: &mut Store,
    sessions: &mut SessionRegistry,
    tenant: &TenantId,
    agent_id: &str,
    action: [u8; 32],
) -> Result<HumanResponse, HumanOperationError> {
    let mut agent = load_agent(store, tenant, agent_id)?;
    let current = sessions
        .get(tenant, SessionId(agent.session_id))
        .ok_or(HumanOperationError::Refused)?;
    if current.generation != agent.session_generation
        || current.request.token_id != agent.session_token_id
        || current.request.agent.as_bytes() != agent.agent_did.as_bytes()
    {
        return Err(HumanOperationError::Refused);
    }
    if !current.open {
        return record_session_observation(store, tenant, agent_id, action, false, false);
    }
    agent.session_generation = agent
        .session_generation
        .checked_add(1)
        .ok_or(HumanOperationError::Refused)?;
    let (response, digest) = session_bytes(&agent, action, false)?;
    let record = key(
        tenant.clone(),
        ObjectKind::Configuration,
        [b"managed-agent-observation-v1:".as_slice(), &[3], &digest].concat(),
    )
    .map_err(|_| HumanOperationError::Refused)?;
    crate::session::close_with_companions(
        store,
        sessions,
        tenant,
        SessionId(agent.session_id),
        vec![(agent_key(tenant, agent_id)?, encode(&agent)?)],
        vec![(record, response.bytes().to_vec())],
    )
    .map_err(|_| HumanOperationError::Unavailable)?;
    Ok(response)
}
