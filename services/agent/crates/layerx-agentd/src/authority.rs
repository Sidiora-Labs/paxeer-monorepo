//! Core-resolved authority checks performed at every write boundary.

use std::collections::{BTreeMap, BTreeSet};

use crate::identity::ProtocolAuthority;

/// Current protocol authority state returned by the node boundary.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthorityState {
    pub authority: ProtocolAuthority,
    pub generation: u64,
    pub valid_through_sequence: u64,
    pub revoked: bool,
    pub permitted_activity_types: BTreeSet<u16>,
    pub observed_sequence: u64,
}

/// Core boundary seam used for authority refreshes.
pub trait AuthorityResolver {
    /// Reads current core authority state, or `None` when core holds no such authority.
    ///
    /// # Errors
    ///
    /// Returns `BoundaryUnavailable` when the node boundary cannot be consulted.
    fn authority_state(
        &mut self,
        authority: &ProtocolAuthority,
    ) -> Result<Option<AuthorityState>, AuthorityError>;
}

/// Bounded cache keyed by protocol authority.
pub struct AuthorityCache {
    entries: BTreeMap<Vec<u8>, AuthorityState>,
    capacity: usize,
}

/// Default retention bound; callers may choose a smaller or larger explicit bound.
pub const DEFAULT_AUTHORITY_CACHE_CAPACITY: usize = 1_024;

impl Default for AuthorityCache {
    fn default() -> Self {
        Self::new(DEFAULT_AUTHORITY_CACHE_CAPACITY)
    }
}

impl AuthorityCache {
    /// Configures the maximum retained authorities. Zero disables retention.
    #[must_use]
    pub fn new(capacity: usize) -> Self {
        Self {
            entries: BTreeMap::new(),
            capacity,
        }
    }

    #[must_use]
    pub const fn capacity(&self) -> usize {
        self.capacity
    }

    #[must_use]
    pub fn len(&self) -> usize {
        self.entries.len()
    }

    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.entries.is_empty()
    }

    fn prune(&mut self, core_sequence: u64, maximum_cache_age: u64) {
        self.entries.retain(|_, state| {
            !state.revoked
                && state.valid_through_sequence >= core_sequence
                && state.observed_sequence <= core_sequence
                && core_sequence - state.observed_sequence <= maximum_cache_age
        });
    }

    fn insert(&mut self, key: Vec<u8>, state: AuthorityState) {
        if self.capacity == 0 || state.revoked {
            return;
        }
        if self.entries.len() == self.capacity && !self.entries.contains_key(&key) {
            // Oldest observation first; authority bytes break ties independently of access order.
            let oldest = self
                .entries
                .iter()
                .min_by_key(|(key, state)| (state.observed_sequence, *key))
                .map(|(key, _)| key.clone());
            if let Some(oldest) = oldest {
                self.entries.remove(&oldest);
            }
        }
        self.entries.insert(key, state);
    }
}

/// Successful check returned with the exact protocol authority used.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResolvedAuthority {
    pub authority: ProtocolAuthority,
    pub generation: u64,
    pub observed_sequence: u64,
}

/// Stable refusal reasons for audit and caller responses.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AuthorityError {
    BoundaryUnavailable,
    Missing,
    SequenceRegression { current: u64, observed: u64 },
    Stale { current: u64, observed: u64 },
    Expired,
    Revoked,
    Rotated,
    Scope,
}

/// Resolves an authority, consulting a cache only inside an explicit sequence bound.
///
/// # Errors
///
/// Returns `Missing` when the boundary holds no state for the authority, and propagates the
/// resolver's own refusal otherwise.
pub fn resolve(
    cache: &mut AuthorityCache,
    resolver: &mut dyn AuthorityResolver,
    authority: &ProtocolAuthority,
    core_sequence: u64,
    maximum_cache_age: u64,
) -> Result<AuthorityState, AuthorityError> {
    let key = authority_key(authority);
    if let Some(cached) = cache.entries.get(&key) {
        if cached.observed_sequence > core_sequence {
            let observed = cached.observed_sequence;
            cache.entries.remove(&key);
            cache.prune(core_sequence, maximum_cache_age);
            return Err(AuthorityError::SequenceRegression {
                current: core_sequence,
                observed,
            });
        }
    }
    cache.prune(core_sequence, maximum_cache_age);
    if let Some(cached) = cache.entries.get(&key) {
        return Ok(cached.clone());
    }
    let current = resolver
        .authority_state(authority)?
        .ok_or(AuthorityError::Missing)?;
    if current.observed_sequence > core_sequence {
        cache.entries.remove(&key);
        return Err(AuthorityError::SequenceRegression {
            current: core_sequence,
            observed: current.observed_sequence,
        });
    }
    if current.authority == *authority
        && current.valid_through_sequence >= core_sequence
        && core_sequence - current.observed_sequence <= maximum_cache_age
    {
        cache.insert(key, current.clone());
    }
    Ok(current)
}

/// Requires current, unrevoked, unrotated protocol scope for one write.
///
/// # Errors
///
/// Returns `Rotated` when the authority or generation differs from the opened one, `Revoked` for
/// a revoked authority, `Expired` past its valid-through sequence, and `Scope` when the activity
/// type is not permitted.
pub fn require_valid(
    expected: &ProtocolAuthority,
    opened_generation: u64,
    activity_type: u16,
    core_sequence: u64,
    state: &AuthorityState,
) -> Result<ResolvedAuthority, AuthorityError> {
    if state.observed_sequence != core_sequence {
        return Err(AuthorityError::Stale {
            current: core_sequence,
            observed: state.observed_sequence,
        });
    }
    if &state.authority != expected || state.generation != opened_generation {
        return Err(AuthorityError::Rotated);
    }
    if state.revoked {
        return Err(AuthorityError::Revoked);
    }
    if core_sequence > state.valid_through_sequence {
        return Err(AuthorityError::Expired);
    }
    if !state.permitted_activity_types.contains(&activity_type) {
        return Err(AuthorityError::Scope);
    }
    Ok(ResolvedAuthority {
        authority: state.authority.clone(),
        generation: state.generation,
        observed_sequence: state.observed_sequence,
    })
}

fn authority_key(authority: &ProtocolAuthority) -> Vec<u8> {
    let (tag, identifier) = match authority {
        ProtocolAuthority::PrimaryKey(identifier) => (1_u8, identifier),
        ProtocolAuthority::SessionKey(identifier) => (2_u8, identifier),
        ProtocolAuthority::CapabilityGrant(identifier) => (3_u8, identifier),
    };
    let mut key = Vec::with_capacity(33);
    key.push(tag);
    key.extend_from_slice(identifier);
    key
}
