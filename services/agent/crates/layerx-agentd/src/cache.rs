//! Tenant-bounded cache whose values can only be constructed from verified evidence.

use std::collections::BTreeMap;

use layerx_proof::inclusion::InclusionEvidence;
use layerx_proof::merkle::{verify_path, Proof};
use layerx_proof::receipt::VerifiedReceipt;
use layerx_types::verify::VerificationLevel;

use crate::store::TenantId;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum EvidenceKind {
    Receipt,
    Inclusion,
    Checkpoint,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CacheValue {
    core_bytes: Box<[u8]>,
    level: VerificationLevel,
    evidence_kind: EvidenceKind,
    evidence_id: [u8; 32],
    observed_head_sequence: u64,
    observed_checkpoint: [u8; 32],
}

impl CacheValue {
    /// Compatibility constructor for an exact verified receipt.
    ///
    /// # Panics
    /// Panics on byte substitution. Use [`Self::try_from_receipt`] for untrusted input.
    #[must_use]
    pub fn from_receipt(
        core_bytes: Vec<u8>,
        evidence: &VerifiedReceipt,
        observed_head_sequence: u64,
        observed_checkpoint: [u8; 32],
    ) -> Self {
        Self::try_from_receipt(
            core_bytes,
            evidence,
            observed_head_sequence,
            observed_checkpoint,
        )
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "cached receipt must be the exact verified receipt"
            )
        })
    }

    /// Refuses bytes other than the complete canonical receipt that passed verification.
    /// # Errors
    /// Returns an error if cache limits are invalid or exceeded, or the supplied bytes do not
    /// match their verified evidence.
    #[allow(
        clippy::needless_pass_by_value,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn try_from_receipt(
        core_bytes: Vec<u8>,
        evidence: &VerifiedReceipt,
        observed_head_sequence: u64,
        observed_checkpoint: [u8; 32],
    ) -> Result<Self, CacheError> {
        if core_bytes != evidence.canonical_bytes() {
            return Err(CacheError::PayloadMismatch);
        }
        Ok(Self {
            core_bytes: evidence.canonical_bytes().into(),
            level: evidence.level(),
            evidence_kind: EvidenceKind::Receipt,
            evidence_id: evidence
                .evidence()
                .receipt_digest()
                .ok_or(CacheError::MissingEvidence)?,
            observed_head_sequence,
            observed_checkpoint,
        })
    }

    /// Compatibility constructor for a single-leaf authenticated tree.
    /// Multi-leaf trees must supply their path to [`Self::try_from_inclusion`].
    ///
    /// # Panics
    /// Panics unless these bytes alone reproduce the authenticated proof root.
    #[must_use]
    pub fn from_inclusion(
        core_bytes: Vec<u8>,
        evidence: &InclusionEvidence,
        observed_head_sequence: u64,
        observed_checkpoint: [u8; 32],
    ) -> Self {
        let proof = Proof::new(0, 1, Vec::new())
            .unwrap_or_else(|error| panic!("single-leaf path: {error:?}"));
        Self::try_from_inclusion(
            core_bytes,
            evidence,
            &proof,
            observed_head_sequence,
            observed_checkpoint,
        )
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "cached inclusion bytes must reproduce the authenticated root"
            )
        })
    }

    /// Checks the supplied leaf and path against the root carried by verified inclusion.
    /// # Errors
    /// Returns an error if cache limits are invalid or exceeded, or the supplied bytes do not
    /// match their verified evidence.
    pub fn try_from_inclusion(
        core_bytes: Vec<u8>,
        evidence: &InclusionEvidence,
        proof: &Proof,
        observed_head_sequence: u64,
        observed_checkpoint: [u8; 32],
    ) -> Result<Self, CacheError> {
        let root = evidence
            .evidence()
            .proof_root()
            .ok_or(CacheError::MissingEvidence)?;
        verify_path(&core_bytes, proof, &root).map_err(|_| CacheError::PayloadMismatch)?;
        Ok(Self {
            core_bytes: core_bytes.into_boxed_slice(),
            level: evidence.level(),
            evidence_kind: EvidenceKind::Inclusion,
            evidence_id: evidence
                .evidence()
                .header_digest()
                .ok_or(CacheError::MissingEvidence)?,
            observed_head_sequence,
            observed_checkpoint,
        })
    }

    #[must_use]
    pub fn core_bytes(&self) -> &[u8] {
        &self.core_bytes
    }

    #[must_use]
    pub const fn level(&self) -> VerificationLevel {
        self.level
    }

    #[must_use]
    pub const fn evidence_kind(&self) -> EvidenceKind {
        self.evidence_kind
    }

    #[must_use]
    pub const fn evidence_id(&self) -> [u8; 32] {
        self.evidence_id
    }

    #[must_use]
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }

    #[must_use]
    pub const fn observed_checkpoint(&self) -> [u8; 32] {
        self.observed_checkpoint
    }
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct CacheMetrics {
    pub entries: usize,
    pub bytes: usize,
    pub hits: u64,
    pub misses: u64,
    pub stale: u64,
    pub revalidations: u64,
    pub quota_refusals: u64,
}

/// Default global admission and key bounds for the compatibility constructor.
pub const MAXIMUM_TENANTS: usize = 128;
pub const MAXIMUM_KEY_BYTES: usize = 1024;

#[derive(Debug)]
pub struct EvidenceCache {
    maximum_entries_per_tenant: usize,
    maximum_bytes_per_tenant: usize,
    maximum_tenants: usize,
    maximum_key_bytes: usize,
    entries: BTreeMap<(TenantId, Box<[u8]>), CacheValue>,
    metrics: BTreeMap<TenantId, CacheMetrics>,
}

impl EvidenceCache {
    /// Creates an empty cache with per-tenant entry and byte ceilings.
    ///
    /// # Errors
    ///
    /// Refuses a zero entry ceiling or a zero byte ceiling.
    pub fn new(
        maximum_entries_per_tenant: usize,
        maximum_bytes_per_tenant: usize,
    ) -> Result<Self, CacheError> {
        Self::with_limits(
            maximum_entries_per_tenant,
            maximum_bytes_per_tenant,
            MAXIMUM_TENANTS,
            MAXIMUM_KEY_BYTES,
        )
    }

    /// Sets both per-tenant storage ceilings and global tenant/key bounds.
    /// # Errors
    /// Returns an error if cache limits are invalid or exceeded, or the supplied bytes do not
    /// match their verified evidence.
    pub fn with_limits(
        maximum_entries_per_tenant: usize,
        maximum_bytes_per_tenant: usize,
        maximum_tenants: usize,
        maximum_key_bytes: usize,
    ) -> Result<Self, CacheError> {
        if [
            maximum_entries_per_tenant,
            maximum_bytes_per_tenant,
            maximum_tenants,
            maximum_key_bytes,
        ]
        .contains(&0)
        {
            return Err(CacheError::InvalidLimits);
        }
        Ok(Self {
            maximum_entries_per_tenant,
            maximum_bytes_per_tenant,
            maximum_tenants,
            maximum_key_bytes,
            entries: BTreeMap::new(),
            metrics: BTreeMap::new(),
        })
    }

    /// Admits bounded metadata even for tenants that only perform cache misses.
    /// Inserts also use this admission; reads never admit an unknown tenant.
    /// # Errors
    /// Returns an error if cache limits are invalid or exceeded, or the supplied bytes do not
    /// match their verified evidence.
    #[allow(
        clippy::needless_pass_by_value,
        reason = "Preserve the public API signature used by daemon consumers."
    )]
    pub fn register_tenant(&mut self, tenant: TenantId) -> Result<(), CacheError> {
        if self.metrics.contains_key(&tenant) {
            return Ok(());
        }
        if self.metrics.len() >= self.maximum_tenants {
            return Err(CacheError::TenantLimit);
        }
        self.metrics.insert(tenant.clone(), CacheMetrics::default());
        Ok(())
    }

    #[must_use]
    pub fn tenant_count(&self) -> usize {
        self.metrics.len()
    }

    fn validate_key(&self, key: &[u8]) -> Result<(), CacheError> {
        if key.is_empty() {
            return Err(CacheError::MissingEvidence);
        }
        if key.len() > self.maximum_key_bytes {
            return Err(CacheError::KeyTooLong);
        }
        Ok(())
    }

    /// Inserts a value whose evidence has already been verified.
    ///
    /// # Errors
    ///
    /// Refuses empty/oversized keys, absent evidence, or exhausted tenant admission.
    /// Key and payload bytes both count toward the tenant's byte ceiling; entry
    /// and tenant counts bound map/metric overhead. Stored slices retain no spare
    /// vector capacity. Overflowing totals return `Arithmetic`.
    pub fn insert(
        &mut self,
        tenant: TenantId,
        key: Vec<u8>,
        value: CacheValue,
    ) -> Result<(), CacheError> {
        self.validate_key(&key)?;
        if value.evidence_id == [0; 32] {
            return Err(CacheError::MissingEvidence);
        }
        self.register_tenant(tenant.clone())?;
        let key = key.into_boxed_slice();
        let current = self.metrics[&tenant];
        let previous_bytes = self
            .entries
            .get(&(tenant.clone(), key.clone()))
            .map_or(0, |entry| key.len() + entry.core_bytes.len());
        let adding_entry = usize::from(!self.entries.contains_key(&(tenant.clone(), key.clone())));
        let projected_entries = current
            .entries
            .checked_add(adding_entry)
            .ok_or(CacheError::Arithmetic)?;
        let projected_bytes = current
            .bytes
            .checked_sub(previous_bytes)
            .and_then(|bytes| bytes.checked_add(key.len()))
            .and_then(|bytes| bytes.checked_add(value.core_bytes.len()))
            .ok_or(CacheError::Arithmetic)?;
        if projected_entries > self.maximum_entries_per_tenant
            || projected_bytes > self.maximum_bytes_per_tenant
        {
            let metrics = self
                .metrics
                .get_mut(&tenant)
                .ok_or(CacheError::UnknownTenant)?;
            metrics.quota_refusals = metrics.quota_refusals.saturating_add(1);
            return Err(CacheError::QuotaExceeded);
        }
        self.entries.insert((tenant.clone(), key), value);
        let metrics = self.metrics.entry(tenant).or_default();
        metrics.entries = projected_entries;
        metrics.bytes = projected_bytes;
        Ok(())
    }

    #[must_use]
    pub fn metrics(&self, tenant: &TenantId) -> CacheMetrics {
        self.metrics.get(tenant).copied().unwrap_or_default()
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CacheError {
    InvalidLimits,
    MissingEvidence,
    PayloadMismatch,
    KeyTooLong,
    UnknownTenant,
    TenantLimit,
    Missing,
    QuotaExceeded,
    InsufficientEvidence {
        requested: VerificationLevel,
        held: VerificationLevel,
    },
    CoreUnavailable {
        cached_level: VerificationLevel,
        stale_by_sequences: u64,
    },
    RevalidationMismatch,
    Arithmetic,
}

/// Returns a cached value only at its held proof level, revalidating any stale entry.
///
/// # Errors
///
/// Returns `Missing` for an unknown key, `InsufficientEvidence` when the held or refreshed level
/// is below the requested one, and `RevalidationMismatch` when a refresh does not observe the
/// current head and checkpoint; a stale entry propagates the refusal raised by `fetch_verified`.
#[allow(
    clippy::needless_pass_by_value,
    reason = "Preserve the public API signature used by daemon consumers."
)]
pub fn revalidate<F>(
    cache: &mut EvidenceCache,
    tenant: TenantId,
    key: &[u8],
    requested: VerificationLevel,
    current_head_sequence: u64,
    current_checkpoint: [u8; 32],
    mut fetch_verified: F,
) -> Result<CacheValue, CacheError>
where
    F: FnMut() -> Result<CacheValue, CacheError>,
{
    cache.validate_key(key)?;
    if !cache.metrics.contains_key(&tenant) {
        return Err(CacheError::UnknownTenant);
    }
    let cache_key = (tenant.clone(), Box::<[u8]>::from(key));
    let Some(cached) = cache.entries.get(&cache_key).cloned() else {
        let metrics = cache
            .metrics
            .get_mut(&tenant)
            .ok_or(CacheError::UnknownTenant)?;
        metrics.misses = metrics.misses.saturating_add(1);
        return Err(CacheError::Missing);
    };
    let stale = cached.observed_head_sequence != current_head_sequence
        || cached.observed_checkpoint != current_checkpoint;
    if !stale {
        if cached.level < requested {
            return Err(CacheError::InsufficientEvidence {
                requested,
                held: cached.level,
            });
        }
        let metrics = cache
            .metrics
            .get_mut(&tenant)
            .ok_or(CacheError::UnknownTenant)?;
        metrics.hits = metrics.hits.saturating_add(1);
        return Ok(cached);
    }
    let metrics = cache
        .metrics
        .get_mut(&tenant)
        .ok_or(CacheError::UnknownTenant)?;
    metrics.stale = metrics.stale.saturating_add(1);
    let refreshed = fetch_verified().map_err(|error| match error {
        CacheError::CoreUnavailable { .. } => CacheError::CoreUnavailable {
            cached_level: cached.level,
            stale_by_sequences: current_head_sequence.saturating_sub(cached.observed_head_sequence),
        },
        other => other,
    })?;
    if refreshed.observed_head_sequence != current_head_sequence
        || refreshed.observed_checkpoint != current_checkpoint
    {
        return Err(CacheError::RevalidationMismatch);
    }
    if refreshed.level < requested {
        return Err(CacheError::InsufficientEvidence {
            requested,
            held: refreshed.level,
        });
    }
    cache.insert(tenant.clone(), key.to_vec(), refreshed.clone())?;
    let metrics = cache
        .metrics
        .get_mut(&tenant)
        .ok_or(CacheError::UnknownTenant)?;
    metrics.revalidations = metrics.revalidations.saturating_add(1);
    Ok(refreshed)
}
