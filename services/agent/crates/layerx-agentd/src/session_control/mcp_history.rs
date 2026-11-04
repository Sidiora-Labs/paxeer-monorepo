use super::{McpInvocationPhase, McpInvocationRecord};
use crate::session::SessionRef;
use crate::store::{ObjectKind, StorageClass, StoreError, StoredValue, TenantId, TenantKey};
use std::collections::{BTreeMap, BTreeSet};

/// Only this many settled records per tenant participate in generation transitions.
/// Older evidence stays in the durable archive and the exact replay/lookup indexes.
/// Begun and effect-started records are pinned independently of this limit.
pub const MCP_RETAINED_TERMINAL_LIMIT: usize = 64;

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct McpHistoryStats {
    pub retained_terminal: usize,
    pub archived_terminal: usize,
    pub unresolved: usize,
    pub last_transition_candidates: usize,
}

#[derive(Debug)]
pub(crate) struct IndexedInvocation {
    session: SessionRef,
    generation: u64,
    idempotency: [u8; 32],
    tool: Option<String>,
    terminal: bool,
    order: u64,
    native_outbox: Option<TenantKey>,
}

#[derive(Debug, Default)]
pub(crate) struct McpInvocationIndex {
    records: BTreeMap<TenantKey, IndexedInvocation>,
    by_session: BTreeMap<(SessionRef, u64), BTreeSet<TenantKey>>,
    by_idempotency: BTreeMap<(TenantId, [u8; 32]), BTreeSet<TenantKey>>,
    by_tool: BTreeMap<(TenantId, [u8; 32], String), BTreeSet<TenantKey>>,
    by_outbox: BTreeMap<TenantKey, BTreeSet<TenantKey>>,
    recent_terminal: BTreeMap<TenantId, BTreeSet<(u64, TenantKey)>>,
    counts: BTreeMap<TenantId, (usize, usize)>,
}

fn remove_from<K: Ord>(map: &mut BTreeMap<K, BTreeSet<TenantKey>>, group: K, key: &TenantKey) {
    if let Some(keys) = map.get_mut(&group) {
        keys.remove(key);
        if keys.is_empty() {
            map.remove(&group);
        }
    }
}

impl McpInvocationIndex {
    pub(crate) fn is_invocation(key: &TenantKey) -> bool {
        key.kind() == ObjectKind::Idempotency && key.object_id().starts_with(b"mcp-invocation-v1/")
    }

    pub(crate) fn decode_entry(
        key: &TenantKey,
        value: Option<&StoredValue>,
    ) -> Result<Option<IndexedInvocation>, StoreError> {
        let Some(value) = value else { return Ok(None) };
        let corrupt = || StoreError::Corrupt("invalid indexed MCP invocation");
        if value.class() != StorageClass::LocalOnly {
            return Err(corrupt());
        }
        let record = McpInvocationRecord::decode(value.bytes()).map_err(|_| corrupt())?;
        let expected_id = match &record.tool_name {
            Some(tool) => {
                use sha2::{Digest, Sha256};
                let mut input = (tool.len() as u64).to_be_bytes().to_vec();
                input.extend_from_slice(tool.as_bytes());
                input.extend_from_slice(&record.idempotency_key);
                [b"mcp-invocation-v1/".as_slice(), &Sha256::digest(input)].concat()
            }
            None => [b"mcp-invocation-v1/".as_slice(), &record.idempotency_key].concat(),
        };
        if record.tenant != key.tenant().as_str() || expected_id != key.object_id() {
            return Err(corrupt());
        }
        let native_outbox = if record.phase == McpInvocationPhase::EffectStarted {
            record
                .native_preparation
                .as_ref()
                .map(|native| {
                    TenantKey::new(
                        key.tenant().clone(),
                        ObjectKind::Outbox,
                        native.idempotency_key.to_vec(),
                    )
                })
                .transpose()?
        } else {
            None
        };
        Ok(Some(IndexedInvocation {
            session: SessionRef::new(
                key.tenant().clone(),
                crate::session::SessionId(record.session),
            ),
            generation: record.generation,
            idempotency: record.idempotency_key,
            tool: record.tool_name,
            terminal: record.phase == McpInvocationPhase::Settled,
            order: record.events.last().ok_or_else(corrupt)?.order,
            native_outbox,
        }))
    }

    pub(crate) fn replace(&mut self, key: TenantKey, record: Option<IndexedInvocation>) {
        if let Some(old) = self.records.remove(&key) {
            remove_from(&mut self.by_session, (old.session, old.generation), &key);
            remove_from(
                &mut self.by_idempotency,
                (key.tenant().clone(), old.idempotency),
                &key,
            );
            if let Some(tool) = old.tool {
                remove_from(
                    &mut self.by_tool,
                    (key.tenant().clone(), old.idempotency, tool),
                    &key,
                );
            }
            if let Some(outbox) = old.native_outbox {
                remove_from(&mut self.by_outbox, outbox, &key);
            }
            if let Some(recent) = self.recent_terminal.get_mut(key.tenant()) {
                recent.remove(&(old.order, key.clone()));
                if recent.is_empty() {
                    self.recent_terminal.remove(key.tenant());
                }
            }
            if let Some(counts) = self.counts.get_mut(key.tenant()) {
                counts.0 -= 1;
                counts.1 -= usize::from(!old.terminal);
                if counts.0 == 0 {
                    self.counts.remove(key.tenant());
                }
            }
        }
        let Some(record) = record else { return };
        let counts = self.counts.entry(key.tenant().clone()).or_default();
        counts.0 += 1;
        counts.1 += usize::from(!record.terminal);
        self.by_session
            .entry((record.session.clone(), record.generation))
            .or_default()
            .insert(key.clone());
        self.by_idempotency
            .entry((key.tenant().clone(), record.idempotency))
            .or_default()
            .insert(key.clone());
        if let Some(tool) = &record.tool {
            self.by_tool
                .entry((key.tenant().clone(), record.idempotency, tool.clone()))
                .or_default()
                .insert(key.clone());
        }
        if let Some(outbox) = &record.native_outbox {
            self.by_outbox
                .entry(outbox.clone())
                .or_default()
                .insert(key.clone());
        }
        let terminal = record.terminal;
        let order = record.order;
        self.records.insert(key.clone(), record);
        if terminal {
            let recent = self
                .recent_terminal
                .entry(key.tenant().clone())
                .or_default();
            recent.insert((order, key));
            if recent.len() > MCP_RETAINED_TERMINAL_LIMIT {
                if let Some((_, evicted)) = recent.pop_first() {
                    if let Some(record) = self.records.get(&evicted) {
                        remove_from(
                            &mut self.by_session,
                            (record.session.clone(), record.generation),
                            &evicted,
                        );
                    }
                }
            }
        }
    }

    pub(crate) fn lookup(
        &self,
        tenant: &TenantId,
        idempotency: [u8; 32],
        tool: Option<&str>,
    ) -> Vec<TenantKey> {
        let keys = match tool {
            Some(tool) => self
                .by_tool
                .get(&(tenant.clone(), idempotency, tool.to_owned())),
            None => self.by_idempotency.get(&(tenant.clone(), idempotency)),
        };
        // Two entries already establish ambiguity; never traverse all colliding tools.
        keys.into_iter()
            .flat_map(|keys| keys.iter().take(2).cloned())
            .collect()
    }

    pub(crate) fn needs_previous(
        &self,
        changed: &BTreeSet<TenantKey>,
        session_transition: bool,
    ) -> bool {
        changed.iter().any(|key| {
            if key.kind() == ObjectKind::Outbox && self.by_outbox.contains_key(key) {
                return true;
            }
            if session_transition && key.kind() == ObjectKind::Session {
                if let Ok(id) = <[u8; 32]>::try_from(key.object_id()) {
                    let session =
                        SessionRef::new(key.tenant().clone(), crate::session::SessionId(id));
                    return self
                        .by_session
                        .range((session.clone(), 0)..=(session, u64::MAX))
                        .next()
                        .is_some();
                }
            }
            false
        })
    }

    pub(crate) fn transition_candidates(
        &self,
        changed: &BTreeSet<TenantKey>,
        session_transition: bool,
        previous: &crate::store::Store,
        current: &crate::store::Store,
    ) -> Result<BTreeSet<TenantKey>, StoreError> {
        let mut candidates = BTreeSet::new();
        for key in changed {
            if session_transition && key.kind() == ObjectKind::Session {
                if let Ok(id) = <[u8; 32]>::try_from(key.object_id()) {
                    let session =
                        SessionRef::new(key.tenant().clone(), crate::session::SessionId(id));
                    if previous.get(key).is_none() {
                        continue;
                    }
                    let record =
                        crate::session::load_record(previous, key.tenant(), session.session_id)
                            .map_err(|_| StoreError::Corrupt("invalid MCP session transition"))?;
                    if let Some(keys) = self.by_session.get(&(session, record.generation)) {
                        candidates.extend(keys.iter().cloned());
                    }
                }
            }
            if key.kind() == ObjectKind::Outbox {
                if let Some(keys) = self.by_outbox.get(key) {
                    candidates.extend(keys.iter().cloned());
                }
            }
        }
        candidates.retain(|key| current.get(key).is_some());
        Ok(candidates)
    }

    pub(crate) fn stats(&self, tenant: &TenantId, candidates: usize) -> McpHistoryStats {
        let (total, unresolved) = self.counts.get(tenant).copied().unwrap_or_default();
        let retained_terminal = self.recent_terminal.get(tenant).map_or(0, BTreeSet::len);
        McpHistoryStats {
            retained_terminal,
            archived_terminal: total - unresolved - retained_terminal,
            unresolved,
            last_transition_candidates: candidates,
        }
    }
}
