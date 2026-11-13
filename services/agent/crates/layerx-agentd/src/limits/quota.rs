use std::collections::{BTreeMap, BTreeSet};

use crate::limits::admission::Priority;
use crate::store::{ObjectKind, Store, StoreError, TenantId, TenantKey};

const SHED_PREFIX: &[u8] = b"quota-shed:";
const INDEX_ID: &[u8] = b"quota-shedding-index-v2";
const SHED_MAGIC: &[u8; 4] = b"LXQS";

/// One durable object write measured against its tenant resource quota.
pub struct ResourceWrite {
    pub resource: Resource,
    pub object_id: Vec<u8>,
    pub bytes: Vec<u8>,
}

/// Durable resource categories with independent per-tenant quotas.
#[derive(Clone, Copy, Debug, Eq, Ord, PartialEq, PartialOrd)]
pub enum Resource {
    Subscription,
    PreparedActivity,
    OutboxEntry,
    AuditRetention,
    StoredReceipt,
}

impl Resource {
    pub const ALL: [Self; 5] = [
        Self::Subscription,
        Self::PreparedActivity,
        Self::OutboxEntry,
        Self::AuditRetention,
        Self::StoredReceipt,
    ];

    const fn object_kind(self) -> ObjectKind {
        match self {
            Self::Subscription => ObjectKind::Subscription,
            Self::PreparedActivity => ObjectKind::PreparedActivity,
            Self::OutboxEntry => ObjectKind::Outbox,
            Self::AuditRetention => ObjectKind::Audit,
            Self::StoredReceipt => ObjectKind::Receipt,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TenantQuota {
    pub tenant: TenantId,
    pub limits: BTreeMap<Resource, usize>,
}

impl TenantQuota {
    /// Builds one tenant quota that must price every durable resource category.
    ///
    /// # Errors
    ///
    /// Returns `InvalidConfiguration` unless all five `Resource` variants are present with a
    /// non-zero limit.
    pub fn new(
        tenant: TenantId,
        limits: impl IntoIterator<Item = (Resource, usize)>,
    ) -> Result<Self, QuotaError> {
        let limits = limits.into_iter().collect::<BTreeMap<_, _>>();
        if limits.len() != Resource::ALL.len()
            || Resource::ALL
                .iter()
                .any(|resource| limits.get(resource).is_none_or(|limit| *limit == 0))
        {
            return Err(QuotaError::InvalidConfiguration);
        }
        Ok(Self { tenant, limits })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SheddingPolicy {
    pub window_ms: u64,
    pub maximum_requests: u64,
    pub maximum_retries: u64,
    pub maximum_identical_operations: u64,
    pub shed_for_ms: u64,
}

impl SheddingPolicy {
    fn validate(self) -> Result<Self, QuotaError> {
        if self.window_ms == 0
            || self.maximum_requests == 0
            || self.maximum_retries == 0
            || self.maximum_identical_operations == 0
            || self.shed_for_ms == 0
        {
            Err(QuotaError::InvalidConfiguration)
        } else {
            Ok(self)
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ClientActivity {
    pub tenant: TenantId,
    pub client_id: String,
    pub operation_digest: [u8; 32],
    pub retry: bool,
    pub observed_at_ms: u64,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum SheddingReason {
    RetryStorm = 1,
    HotLoop = 2,
    PathologicalClient = 3,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SheddingDecision {
    pub tenant: TenantId,
    pub client_id: String,
    pub reason: SheddingReason,
    pub observed_at_ms: u64,
    pub shed_until_ms: u64,
    pub requests_in_window: u64,
    pub retries_in_window: u64,
    pub identical_operations: u64,
    pub durable_record: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResourceUtilization {
    pub resource: Resource,
    pub used: usize,
    pub limit: usize,
    pub remaining: usize,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct QuotaHealth {
    pub tenant: TenantId,
    pub resources: Vec<ResourceUtilization>,
    pub actively_shed_clients: Vec<String>,
}

#[derive(Debug)]
pub enum QuotaError {
    InvalidConfiguration,
    InvalidClient,
    UnconfiguredTenant(TenantId),
    Exhausted {
        tenant: TenantId,
        resource: Resource,
        used: usize,
        limit: usize,
    },
    ClientShed {
        tenant: TenantId,
        client_id: String,
        reason: SheddingReason,
        retry_after_ms: u64,
    },
    TimeRegressed,
    Arithmetic,
    CorruptDecision,
    ObservationCapacity {
        tenant: TenantId,
        limit: usize,
    },
    Store(StoreError),
}

impl From<StoreError> for QuotaError {
    fn from(value: StoreError) -> Self {
        Self::Store(value)
    }
}

#[derive(Clone, Debug)]
struct ClientWindow {
    start_ms: u64,
    last_observed_ms: u64,
    requests: u64,
    retries: u64,
    last_operation: [u8; 32],
    identical_operations: u64,
}

/// Latest decisions are retained until this interval after the shed ends.
/// Superseded decisions are compacted immediately; active windows are never evicted for churn.
#[derive(Clone, Copy, Debug)]
pub struct ObservationBounds {
    pub maximum_clients_per_tenant: usize,
    pub audit_retention_ms: u64,
}

impl Default for ObservationBounds {
    fn default() -> Self {
        Self {
            maximum_clients_per_tenant: 1024,
            audit_retention_ms: 60_000,
        }
    }
}

#[derive(Default, serde::Serialize, serde::Deserialize)]
struct DecisionIndex {
    watermark_ms: u64,
    clients: BTreeMap<String, u64>,
}

/// Per-tenant durable-resource quotas and client-isolated load shedding.
#[derive(Debug)]
pub struct Quota {
    tenants: BTreeMap<TenantId, TenantQuota>,
    shedding: SheddingPolicy,
    bounds: ObservationBounds,
    windows: BTreeMap<(TenantId, String), ClientWindow>,
}

impl Quota {
    /// Builds the quota enforcer from one quota per tenant plus a shedding policy.
    ///
    /// # Errors
    ///
    /// Returns `InvalidConfiguration` for a repeated tenant, an empty tenant set, or a
    /// shedding policy whose window, request, retry, identical-operation or shed duration
    /// bound is zero.
    pub fn new(
        tenants: impl IntoIterator<Item = TenantQuota>,
        shedding: SheddingPolicy,
    ) -> Result<Self, QuotaError> {
        let mut configured = BTreeMap::new();
        for quota in tenants {
            if configured.insert(quota.tenant.clone(), quota).is_some() {
                return Err(QuotaError::InvalidConfiguration);
            }
        }
        if configured.is_empty() {
            return Err(QuotaError::InvalidConfiguration);
        }
        Ok(Self {
            tenants: configured,
            shedding: shedding.validate()?,
            bounds: ObservationBounds::default(),
            windows: BTreeMap::new(),
        })
    }

    /// # Errors
    /// Returns an error if quota configuration or a retained decision is invalid, corrupt or
    /// unavailable.
    pub fn with_observation_bounds(
        mut self,
        bounds: ObservationBounds,
    ) -> Result<Self, QuotaError> {
        if bounds.maximum_clients_per_tenant == 0 || bounds.audit_retention_ms == 0 {
            return Err(QuotaError::InvalidConfiguration);
        }
        self.bounds = bounds;
        Ok(self)
    }

    /// Migrate the old timestamped layout before attaching an existing store to requests.
    /// # Errors
    /// Returns an error if quota configuration or a retained decision is invalid, corrupt or
    /// unavailable.
    pub fn restore(&mut self, store: &mut Store, observed_at_ms: u64) -> Result<(), QuotaError> {
        for tenant in self.tenants.keys().cloned().collect::<Vec<_>>() {
            self.prepare_index(store, &tenant, observed_at_ms)?;
            let mut legacy = Vec::new();
            for id in store.list_object_ids(&tenant, ObjectKind::Configuration) {
                if !id.starts_with(SHED_PREFIX) {
                    continue;
                }
                let key = TenantKey::new(tenant.clone(), ObjectKind::Configuration, id.clone())?;
                let decision = decode_decision(
                    tenant.clone(),
                    store.get(&key).ok_or(QuotaError::CorruptDecision)?.bytes(),
                )?;
                if id != decision_object_id(&decision.client_id, 0) {
                    legacy.push(key);
                }
            }
            store.update_local_batch_removing(Vec::new(), legacy)?;
        }
        self.windows.retain(|_, window| {
            observed_at_ms.saturating_sub(window.last_observed_ms) < self.shedding.window_ms
        });
        Ok(())
    }

    #[must_use]
    pub fn active_windows(&self, tenant: &TenantId) -> usize {
        self.windows
            .keys()
            .filter(|(owner, _)| owner == tenant)
            .count()
    }

    /// # Errors
    /// Returns an error if quota configuration or a retained decision is invalid, corrupt or
    /// unavailable.
    #[allow(
        clippy::unused_self,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn retained_decisions(
        &self,
        store: &Store,
        tenant: &TenantId,
    ) -> Result<usize, QuotaError> {
        Ok(load_index(store, tenant)?.clients.len())
    }

    fn prepare_index(
        &self,
        store: &mut Store,
        tenant: &TenantId,
        now: u64,
    ) -> Result<DecisionIndex, QuotaError> {
        let index_key = index_key(tenant)?;
        if store.get(&index_key).is_none() {
            let mut latest: BTreeMap<String, SheddingDecision> = BTreeMap::new();
            let mut legacy = Vec::new();
            for id in store.list_object_ids(tenant, ObjectKind::Configuration) {
                if !id.starts_with(SHED_PREFIX) {
                    continue;
                }
                let key = TenantKey::new(tenant.clone(), ObjectKind::Configuration, id)?;
                let decision = decode_decision(
                    tenant.clone(),
                    store.get(&key).ok_or(QuotaError::CorruptDecision)?.bytes(),
                )?;
                legacy.push(key);
                if latest
                    .get(&decision.client_id)
                    .is_none_or(|old| old.observed_at_ms < decision.observed_at_ms)
                {
                    latest.insert(decision.client_id.clone(), decision);
                }
            }
            let mut index = DecisionIndex::default();
            let mut records = Vec::new();
            for decision in latest.values() {
                if now < decision.observed_at_ms {
                    return Err(QuotaError::TimeRegressed);
                }
                let retain_until = decision
                    .shed_until_ms
                    .checked_add(self.bounds.audit_retention_ms)
                    .ok_or(QuotaError::Arithmetic)?;
                if now >= retain_until {
                    continue;
                }
                if index.clients.len() >= self.bounds.maximum_clients_per_tenant {
                    return Err(self.capacity(tenant));
                }
                index
                    .clients
                    .insert(decision.client_id.clone(), retain_until);
                records.push((
                    decision_key(tenant, &decision.client_id)?,
                    encode_decision(decision)?,
                ));
            }
            records.push((index_key.clone(), encode_index(&index)?));
            store.commit_local_records(records)?;
            // Recovery may be interrupted here: the index and latest decisions are already durable.
            let retained = index
                .clients
                .keys()
                .map(|client| decision_key(tenant, client))
                .collect::<Result<BTreeSet<_>, _>>()?;
            let removals = legacy
                .into_iter()
                .filter(|key| !retained.contains(key))
                .collect();
            store.update_local_batch_removing(Vec::new(), removals)?;
        }
        let mut index = load_index(store, tenant)?;
        if now < index.watermark_ms {
            return Err(QuotaError::TimeRegressed);
        }
        if index.clients.len() > self.bounds.maximum_clients_per_tenant {
            return Err(self.capacity(tenant));
        }
        let expired = index
            .clients
            .iter()
            .filter(|(_, expiry)| now >= **expiry)
            .map(|(client, _)| client.clone())
            .collect::<Vec<_>>();
        let mut removals = Vec::new();
        for client in expired {
            index.clients.remove(&client);
            removals.push(decision_key(tenant, &client)?);
        }
        index.watermark_ms = now;
        store.update_local_batch_removing(vec![(index_key, encode_index(&index)?)], removals)?;
        Ok(index)
    }

    fn capacity(&self, tenant: &TenantId) -> QuotaError {
        QuotaError::ObservationCapacity {
            tenant: tenant.clone(),
            limit: self.bounds.maximum_clients_per_tenant,
        }
    }

    /// Creates one real durable resource only after checking its tenant quota.
    ///
    /// # Errors
    ///
    /// Returns `UnconfiguredTenant`, `InvalidClient` or `ClientShed` from admission, then
    /// `Exhausted` when a new object would pass the resource limit, or `Store` for a rejected
    /// object id or a failed durable write.
    pub fn create_resource(
        &self,
        store: &mut Store,
        tenant: &TenantId,
        client_id: &str,
        write: ResourceWrite,
        observed_at_ms: u64,
    ) -> Result<(), QuotaError> {
        let ResourceWrite {
            resource,
            object_id,
            bytes,
        } = write;
        self.admit_work(
            store,
            tenant,
            client_id,
            Priority::InteractiveRead,
            observed_at_ms,
        )?;
        let configuration = self.configuration(tenant)?;
        let key = TenantKey::new(tenant.clone(), resource.object_kind(), object_id)?;
        let used = store.list_object_ids(tenant, resource.object_kind()).len();
        let limit = configuration.limits[&resource];
        if store.get(&key).is_none() && used >= limit {
            return Err(QuotaError::Exhausted {
                tenant: tenant.clone(),
                resource,
                used,
                limit,
            });
        }
        match resource {
            Resource::StoredReceipt => store.put_core_cache(key, bytes)?,
            _ => store.put_local(key, bytes)?,
        }
        Ok(())
    }

    /// Preserves submission and receipt-resolution capacity while shedding
    /// only non-critical work belonging to an actively shed client.
    ///
    /// # Errors
    ///
    /// Returns `UnconfiguredTenant` for an unknown tenant, `InvalidClient` for an empty,
    /// oversized or NUL-bearing client id, `ClientShed` with `retry_after_ms` while a durable
    /// shed is still active, or `CorruptDecision`/`TimeRegressed` from decoding that record.
    pub fn admit_work(
        &self,
        store: &Store,
        tenant: &TenantId,
        client_id: &str,
        priority: Priority,
        observed_at_ms: u64,
    ) -> Result<(), QuotaError> {
        self.configuration(tenant)?;
        validate_client(client_id)?;
        if matches!(priority, Priority::Submission | Priority::ReceiptResolution) {
            return Ok(());
        }
        if let Some(decision) = active_decision(store, tenant, client_id, observed_at_ms)? {
            return Err(QuotaError::ClientShed {
                tenant: tenant.clone(),
                client_id: client_id.to_owned(),
                reason: decision.reason,
                retry_after_ms: decision.shed_until_ms - observed_at_ms,
            });
        }
        Ok(())
    }

    /// Updates exact per-client observations and records a newly detected shed durably.
    ///
    /// # Errors
    ///
    /// Returns `UnconfiguredTenant` or `InvalidClient` for a rejected caller, `TimeRegressed`
    /// when the observation predates the window or an existing decision, `Arithmetic` on
    /// counter or shed-deadline overflow, and `Store` if a new decision cannot be persisted.
    pub fn observe_activity(
        &mut self,
        store: &mut Store,
        activity: ClientActivity,
    ) -> Result<Option<SheddingDecision>, QuotaError> {
        self.configuration(&activity.tenant)?;
        validate_client(&activity.client_id)?;
        let mut index = self.prepare_index(store, &activity.tenant, activity.observed_at_ms)?;
        if let Some(active) = active_decision(
            store,
            &activity.tenant,
            &activity.client_id,
            activity.observed_at_ms,
        )? {
            return Ok(Some(active));
        }

        self.windows.retain(|(tenant, _), window| {
            *tenant != activity.tenant
                || activity
                    .observed_at_ms
                    .saturating_sub(window.last_observed_ms)
                    < self.shedding.window_ms
        });
        let key = (activity.tenant.clone(), activity.client_id.clone());
        crate::reject_if(
            !self.windows.contains_key(&key)
                && self.active_windows(&activity.tenant) >= self.bounds.maximum_clients_per_tenant,
            || self.capacity(&activity.tenant),
        )?;
        let window = self.windows.entry(key).or_insert(ClientWindow {
            start_ms: activity.observed_at_ms,
            last_observed_ms: activity.observed_at_ms,
            requests: 0,
            retries: 0,
            last_operation: activity.operation_digest,
            identical_operations: 0,
        });
        observe_client_window(window, &activity, self.shedding.window_ms)?;

        let reason = if window.retries > self.shedding.maximum_retries {
            Some(SheddingReason::RetryStorm)
        } else if window.identical_operations > self.shedding.maximum_identical_operations {
            Some(SheddingReason::HotLoop)
        } else if window.requests > self.shedding.maximum_requests {
            Some(SheddingReason::PathologicalClient)
        } else {
            None
        };
        let Some(reason) = reason else {
            return Ok(None);
        };
        let shed_until_ms = activity
            .observed_at_ms
            .checked_add(self.shedding.shed_for_ms)
            .ok_or(QuotaError::Arithmetic)?;
        let durable_record = decision_object_id(&activity.client_id, activity.observed_at_ms);
        let decision = SheddingDecision {
            tenant: activity.tenant.clone(),
            client_id: activity.client_id,
            reason,
            observed_at_ms: activity.observed_at_ms,
            shed_until_ms,
            requests_in_window: window.requests,
            retries_in_window: window.retries,
            identical_operations: window.identical_operations,
            durable_record: durable_record.clone(),
        };
        crate::reject_if(
            !index.clients.contains_key(&decision.client_id)
                && index.clients.len() >= self.bounds.maximum_clients_per_tenant,
            || self.capacity(&activity.tenant),
        )?;
        let retention = decision
            .shed_until_ms
            .checked_add(self.bounds.audit_retention_ms)
            .ok_or(QuotaError::Arithmetic)?;
        index.clients.insert(decision.client_id.clone(), retention);
        store.commit_local_records(vec![
            (
                decision_key(&activity.tenant, &decision.client_id)?,
                encode_decision(&decision)?,
            ),
            (index_key(&activity.tenant)?, encode_index(&index)?),
        ])?;
        Ok(Some(decision))
    }

    /// Reports per-resource utilization and the clients currently being shed.
    ///
    /// # Errors
    ///
    /// Returns `UnconfiguredTenant` for an unknown tenant, `Store` when a shed object id is
    /// rejected, or `CorruptDecision` when a listed shed record is missing or malformed.
    pub fn health(
        &self,
        store: &Store,
        tenant: &TenantId,
        observed_at_ms: u64,
    ) -> Result<QuotaHealth, QuotaError> {
        let configuration = self.configuration(tenant)?;
        let resources = Resource::ALL
            .into_iter()
            .map(|resource| {
                let used = store.list_object_ids(tenant, resource.object_kind()).len();
                let limit = configuration.limits[&resource];
                ResourceUtilization {
                    resource,
                    used,
                    limit,
                    remaining: limit.saturating_sub(used),
                }
            })
            .collect();
        let mut clients = BTreeSet::new();
        let index = load_index(store, tenant)?;
        if observed_at_ms < index.watermark_ms {
            return Err(QuotaError::TimeRegressed);
        }
        for client in index.clients.keys() {
            if active_decision(store, tenant, client, observed_at_ms)?.is_some() {
                clients.insert(client.clone());
            }
        }
        Ok(QuotaHealth {
            tenant: tenant.clone(),
            resources,
            actively_shed_clients: clients.into_iter().collect(),
        })
    }

    fn configuration(&self, tenant: &TenantId) -> Result<&TenantQuota, QuotaError> {
        self.tenants
            .get(tenant)
            .ok_or_else(|| QuotaError::UnconfiguredTenant(tenant.clone()))
    }
}
fn observe_client_window(
    window: &mut ClientWindow,
    activity: &ClientActivity,
    window_ms: u64,
) -> Result<(), QuotaError> {
    crate::reject_if(activity.observed_at_ms < window.last_observed_ms, || {
        QuotaError::TimeRegressed
    })?;
    if activity.observed_at_ms.saturating_sub(window.start_ms) >= window_ms {
        *window = ClientWindow {
            start_ms: activity.observed_at_ms,
            last_observed_ms: activity.observed_at_ms,
            requests: 0,
            retries: 0,
            last_operation: activity.operation_digest,
            identical_operations: 0,
        };
    }
    window.last_observed_ms = activity.observed_at_ms;
    window.requests = window
        .requests
        .checked_add(1)
        .ok_or(QuotaError::Arithmetic)?;
    if activity.retry {
        window.retries = window
            .retries
            .checked_add(1)
            .ok_or(QuotaError::Arithmetic)?;
    }
    if window.last_operation == activity.operation_digest {
        window.identical_operations = window
            .identical_operations
            .checked_add(1)
            .ok_or(QuotaError::Arithmetic)?;
    } else {
        window.last_operation = activity.operation_digest;
        window.identical_operations = 1;
    }

    Ok(())
}

fn validate_client(client_id: &str) -> Result<(), QuotaError> {
    if client_id.is_empty() || client_id.len() > 255 || client_id.as_bytes().contains(&0) {
        Err(QuotaError::InvalidClient)
    } else {
        Ok(())
    }
}

fn active_decision(
    store: &Store,
    tenant: &TenantId,
    client_id: &str,
    observed_at_ms: u64,
) -> Result<Option<SheddingDecision>, QuotaError> {
    if observed_at_ms < index_watermark(store, tenant)? {
        return Err(QuotaError::TimeRegressed);
    }
    let key = decision_key(tenant, client_id)?;
    let Some(stored) = store.get(&key) else {
        if load_index(store, tenant)?.clients.contains_key(client_id) {
            return Err(QuotaError::CorruptDecision);
        }
        return Ok(None);
    };
    let decision = decode_decision(tenant.clone(), stored.bytes())?;
    if decision.client_id != client_id {
        return Err(QuotaError::CorruptDecision);
    }
    if observed_at_ms < decision.observed_at_ms {
        return Err(QuotaError::TimeRegressed);
    }
    Ok((observed_at_ms < decision.shed_until_ms).then_some(decision))
}

fn decision_object_id(client_id: &str, _observed_at_ms: u64) -> Vec<u8> {
    let mut object_id = SHED_PREFIX.to_vec();
    object_id.extend_from_slice(client_id.as_bytes());
    object_id.push(0);
    object_id
}

fn encode_decision(decision: &SheddingDecision) -> Result<Vec<u8>, QuotaError> {
    let client = decision.client_id.as_bytes();
    let length = u16::try_from(client.len()).map_err(|_| QuotaError::InvalidClient)?;
    let mut bytes = Vec::with_capacity(55 + client.len());
    bytes.extend_from_slice(SHED_MAGIC);
    bytes.extend_from_slice(&length.to_be_bytes());
    bytes.extend_from_slice(client);
    bytes.push(decision.reason as u8);
    for value in [
        decision.observed_at_ms,
        decision.shed_until_ms,
        decision.requests_in_window,
        decision.retries_in_window,
        decision.identical_operations,
    ] {
        bytes.extend_from_slice(&value.to_be_bytes());
    }
    Ok(bytes)
}

fn decode_decision(tenant: TenantId, bytes: &[u8]) -> Result<SheddingDecision, QuotaError> {
    if bytes.len() < 47 || &bytes[..4] != SHED_MAGIC {
        return Err(QuotaError::CorruptDecision);
    }
    let client_length = usize::from(u16::from_be_bytes([bytes[4], bytes[5]]));
    let expected = 47_usize
        .checked_add(client_length)
        .ok_or(QuotaError::Arithmetic)?;
    if bytes.len() != expected {
        return Err(QuotaError::CorruptDecision);
    }
    let client_id = std::str::from_utf8(&bytes[6..6 + client_length])
        .map_err(|_| QuotaError::CorruptDecision)?
        .to_owned();
    validate_client(&client_id)?;
    let reason = match bytes[6 + client_length] {
        1 => SheddingReason::RetryStorm,
        2 => SheddingReason::HotLoop,
        3 => SheddingReason::PathologicalClient,
        _ => return Err(QuotaError::CorruptDecision),
    };
    let mut offset = 7 + client_length;
    let mut next = || -> Result<u64, QuotaError> {
        let end = offset + 8;
        let value = u64::from_be_bytes(
            bytes[offset..end]
                .try_into()
                .map_err(|_| QuotaError::CorruptDecision)?,
        );
        offset = end;
        Ok(value)
    };
    let observed_at_ms = next()?;
    Ok(SheddingDecision {
        durable_record: decision_object_id(&client_id, observed_at_ms),
        tenant,
        client_id,
        reason,
        observed_at_ms,
        shed_until_ms: next()?,
        requests_in_window: next()?,
        retries_in_window: next()?,
        identical_operations: next()?,
    })
}

fn index_key(tenant: &TenantId) -> Result<TenantKey, QuotaError> {
    Ok(TenantKey::new(
        tenant.clone(),
        ObjectKind::Configuration,
        INDEX_ID.to_vec(),
    )?)
}

fn decision_key(tenant: &TenantId, client: &str) -> Result<TenantKey, QuotaError> {
    Ok(TenantKey::new(
        tenant.clone(),
        ObjectKind::Configuration,
        decision_object_id(client, 0),
    )?)
}

fn load_index(store: &Store, tenant: &TenantId) -> Result<DecisionIndex, QuotaError> {
    match store.get(&index_key(tenant)?) {
        Some(value) => {
            let bytes = value.bytes();
            let watermark = index_watermark(store, tenant)?;
            let index: DecisionIndex =
                serde_json::from_slice(bytes.get(8..).ok_or(QuotaError::CorruptDecision)?)
                    .map_err(|_| QuotaError::CorruptDecision)?;
            if index.watermark_ms != watermark {
                return Err(QuotaError::CorruptDecision);
            }
            Ok(index)
        }
        None => Ok(DecisionIndex::default()),
    }
}

fn encode_index(index: &DecisionIndex) -> Result<Vec<u8>, QuotaError> {
    let mut bytes = index.watermark_ms.to_be_bytes().to_vec();
    bytes.extend(serde_json::to_vec(index).map_err(|_| QuotaError::CorruptDecision)?);
    Ok(bytes)
}

fn index_watermark(store: &Store, tenant: &TenantId) -> Result<u64, QuotaError> {
    match store.get(&index_key(tenant)?) {
        Some(value) => Ok(u64::from_be_bytes(
            value
                .bytes()
                .get(..8)
                .ok_or(QuotaError::CorruptDecision)?
                .try_into()
                .map_err(|_| QuotaError::CorruptDecision)?,
        )),
        None => Ok(0),
    }
}
