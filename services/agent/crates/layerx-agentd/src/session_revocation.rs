//! Receipt-backed layerxauth session grants and immediate local invalidation.

use layerx_agent_api::error::{ApiError, ErrorClass, ReasonCode, RequestId, Retriability};
use layerx_client::sequencer::{SequencerLog, TransactionReceipt};
use layerx_types::ids::Did;

use crate::session_keys::ProvisionedEvmSessionKey;

use crate::identity::ProtocolAuthority;
use crate::store::{ObjectKind, Store, TenantKey};

use super::{
    encode, next_generation, session_key, SessionCredential, SessionError, SessionRef,
    SessionRegistry, Token,
};

const LAYERXAUTH: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1c,
];
const GRANT_REVOKED: [u8; 32] = [
    0x56, 0xae, 0x84, 0xef, 0xf6, 0xbf, 0x7f, 0x5e, 0x20, 0x1e, 0x33, 0xf3, 0xe3, 0x7e, 0x1e, 0x3b,
    0xf3, 0xf3, 0x47, 0x0d, 0xac, 0xdb, 0x55, 0x80, 0x18, 0x42, 0x58, 0x9a, 0x3d, 0x15, 0x64, 0x19,
];
const SESSION_GRANTED: [u8; 32] = [
    0xcd, 0x2f, 0x11, 0x43, 0x3d, 0x26, 0x59, 0x6e, 0x4c, 0x5d, 0x21, 0x4c, 0x17, 0x7b, 0xd0, 0x3c,
    0x81, 0x7c, 0x9e, 0x19, 0xd9, 0x85, 0x0e, 0xe6, 0x1b, 0xda, 0x34, 0x42, 0xaa, 0x0e, 0x3b, 0x9a,
];

fn event_bytes<'a>(
    receipt: &'a TransactionReceipt,
    topic: [u8; 32],
    grant_id: [u8; 32],
    length: usize,
) -> Result<&'a [u8], SessionError> {
    if !receipt.status || grant_id == [0; 32] {
        return Err(SessionError::AuthorityMissing);
    }
    let mut logs = receipt.logs.iter().filter(|log| {
        log.address == LAYERXAUTH
            && log.topics.first() == Some(&topic)
            && log.topics.get(1) == Some(&grant_id)
    });
    let log = logs.next().ok_or(SessionError::AuthorityMissing)?;
    if logs.next().is_some() || log.topics.len() != 2 {
        return Err(SessionError::AuthorityMissing);
    }
    abi_event_bytes(log, length)
}

fn abi_event_bytes(log: &SequencerLog, length: usize) -> Result<&[u8], SessionError> {
    let padded = length.div_ceil(32) * 32;
    let data = &log.data;
    if data.len() != 64 + padded
        || data[..31] != [0; 31]
        || data[31] != 32
        || data[32..63] != [0; 31]
        || usize::from(data[63]) != length
        || data[64 + length..].iter().any(|byte| *byte != 0)
    {
        return Err(SessionError::AuthorityMissing);
    }
    Ok(&data[64..64 + length])
}

impl RevocationEvent {
    /// Decodes a successful revokeGrant receipt obtained from the trusted chain endpoint.
    /// The indexed grant and the canonical 41-byte revocation record must agree.
    ///
    /// # Errors
    /// Refuses reverted receipts, foreign emitters, ambiguous logs and malformed records.
    pub fn from_revoke_grant_receipt(
        did: Did,
        grant_id: [u8; 32],
        receipt: &TransactionReceipt,
    ) -> Result<Self, SessionError> {
        let record = event_bytes(receipt, GRANT_REVOKED, grant_id, 41)?;
        if record[..32] != grant_id || !(1..=5).contains(&record[32]) || record[33..] == [0; 8] {
            return Err(SessionError::AuthorityMissing);
        }
        Ok(Self {
            did,
            authority: Some(ProtocolAuthority::SessionKey(grant_id)),
            reason: InvalidationReason::GrantRevokedOnChain,
            observed_sequence: receipt.block_number,
        })
    }
}

fn grant_key(session: &SessionRef) -> Result<TenantKey, SessionError> {
    let mut id = b"layerxauth/session-grant/v1:".to_vec();
    id.extend(session.session_id.0);
    Ok(TenantKey::new(
        session.tenant.clone(),
        ObjectKind::Configuration,
        id,
    )?)
}

impl SessionRegistry {
    /// Retains the grantSession receipt for an existing session and its provisioned EVM key.
    ///
    /// # Errors
    /// Refuses receipts that do not bind this session's grant, principal and address.
    pub fn record_grant_session_receipt(
        &self,
        store: &mut Store,
        session: &SessionRef,
        key: &ProvisionedEvmSessionKey,
        receipt: &TransactionReceipt,
    ) -> Result<(), SessionError> {
        let record = self
            .get(&session.tenant, session.session_id)
            .ok_or(SessionError::NotFound)?;
        if !record.open {
            return Err(SessionError::Revoked);
        }
        let ProtocolAuthority::SessionKey(grant_id) = record.request.authority else {
            return Err(SessionError::AuthorityMissing);
        };
        let summary = event_bytes(receipt, SESSION_GRANTED, grant_id, 209)?;
        let principal = layerx_wire::hash::did_id_for_protocol(&record.request.agent, 3)
            .map_err(|_| SessionError::IdentityMismatch)?;
        if summary[..5] != *b"LXGS2"
            || summary[5..37] != grant_id
            || summary[37..69] != principal
            || summary[133..145] != [0; 12]
            || summary[145..165] != key.evm_address()
            || summary[165..173] != record.request.expiry_sequence.to_be_bytes()
        {
            return Err(SessionError::IdentityMismatch);
        }
        key.with_secret(|_| ()).map_err(|_| SessionError::Revoked)?;
        let evidence_key = grant_key(session)?;
        if let Some(existing) = store.get(&evidence_key) {
            if existing.bytes() != summary {
                return Err(SessionError::IdentityMismatch);
            }
            return Ok(());
        }
        store.put_local(evidence_key, summary.to_vec())?;
        Ok(())
    }

    /// Refreshes a bearer only after the session's grantSession receipt has been retained.
    /// HTTP adapters can forward the machine-readable refusal unchanged.
    ///
    /// # Errors
    /// Returns session.grant_missing without mutating state when grant evidence is absent.
    pub fn session_refresh(
        &mut self,
        store: &mut Store,
        credential: &SessionCredential,
        replacement_token: [u8; 32],
        current_height: u64,
        request_id: RequestId,
    ) -> Result<Token, ApiError> {
        let refusal = |reason| ApiError {
            class: ErrorClass::CapabilityRefusal,
            protocol_result_code: None,
            retriability: Retriability::Terminal,
            request_id,
            reason: ReasonCode::new(reason).unwrap_or_else(|_| unreachable!("static reason code")),
        };
        let session = SessionRef::new(credential.tenant().clone(), credential.session_id());
        let evidence_key = grant_key(&session).map_err(|_| refusal("session.grant_missing"))?;
        let summary = store
            .get(&evidence_key)
            .ok_or_else(|| refusal("session.grant_missing"))?
            .bytes();
        let token = self
            .authenticate(credential)
            .map_err(|_| refusal("session.revoked"))?;
        let record = self
            .get(token.tenant(), token.session_id())
            .ok_or_else(|| refusal("session.revoked"))?;
        let ProtocolAuthority::SessionKey(grant_id) = record.request.authority else {
            return Err(refusal("session.grant_missing"));
        };
        if summary.len() != 209 || summary[..5] != *b"LXGS2" || summary[5..37] != grant_id {
            return Err(refusal("session.grant_missing"));
        }
        if current_height >= record.request.expiry_sequence {
            return Err(refusal("session.expired"));
        }
        let scopes = record.request.scopes.clone();
        let activities = record.request.permitted_activity_types.clone();
        super::restrict_scope(
            store,
            self,
            credential.tenant(),
            credential.session_id(),
            replacement_token,
            scopes,
            activities,
        )
        .map_err(|error| {
            refusal(match error {
                SessionError::TokenReuse => "session.token_reuse",
                SessionError::Store(_) => "session.store_unavailable",
                _ => "session.refresh_refused",
            })
        })
    }
}

/// Receipt-derived grant revocation, or an existing identity rotation/recovery notification.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RevocationEvent {
    pub did: Did,
    pub authority: Option<ProtocolAuthority>,
    pub reason: InvalidationReason,
    pub observed_sequence: u64,
}

/// Exact reason reported to affected clients.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum InvalidationReason {
    GrantRevokedOnChain,
    PrimaryKeyRotated,
    AccountRecovered,
}

/// Local lifecycle state relevant to revocation handling.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum PreparationState {
    Prepared,
    Signed,
    Queued,
    Unknown,
    NonceConsumed,
    Executed,
    Failed,
}

/// One activity whose local handling may be changed by invalidation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PendingActivity {
    pub session: SessionRef,
    pub state: PreparationState,
    pub cancelled: bool,
    pub resolution_continues: bool,
}

/// Observable invalidation result.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct InvalidationReport {
    pub reason: InvalidationReason,
    pub observed_sequence: u64,
    pub invalidated_sessions: Vec<SessionRef>,
    pub invalidated_generations: Vec<(SessionRef, u64)>,
    pub cancelled_preparations: usize,
    pub unresolved_left_for_resolution: usize,
    pub executed_untouched: usize,
}

pub(crate) fn apply_revocation(
    store: &mut Store,
    registry: &mut SessionRegistry,
    activities: &mut [PendingActivity],
    event: &RevocationEvent,
) -> Result<InvalidationReport, SessionError> {
    apply_revocation_with_updates(store, registry, activities, event, Vec::new())
}

fn revoked_grant_id(authority: &ProtocolAuthority) -> Option<[u8; 32]> {
    match authority {
        ProtocolAuthority::SessionKey(id) | ProtocolAuthority::CapabilityGrant(id) => Some(*id),
        ProtocolAuthority::PrimaryKey(_) => None,
    }
}

pub(crate) fn apply_revocation_with_updates(
    store: &mut Store,
    registry: &mut SessionRegistry,
    activities: &mut [PendingActivity],
    event: &RevocationEvent,
    mut companion: Vec<(crate::store::TenantKey, Vec<u8>)>,
) -> Result<InvalidationReport, SessionError> {
    let grant_id = if event.reason == InvalidationReason::GrantRevokedOnChain {
        Some(
            event
                .authority
                .as_ref()
                .and_then(revoked_grant_id)
                .ok_or(SessionError::AuthorityMissing)?,
        )
    } else {
        None
    };
    let mut invalidated_sessions = Vec::new();
    let mut invalidated_generations = Vec::new();
    let candidates = registry
        .records()
        .iter()
        .filter(|(_, record)| {
            record.open
                && record.request.agent == event.did
                && match grant_id {
                    Some(id) => revoked_grant_id(&record.request.authority) == Some(id),
                    None => event
                        .authority
                        .as_ref()
                        .is_none_or(|authority| &record.request.authority == authority),
                }
        })
        .map(|(session, record)| {
            let mut invalidated = record.clone();
            invalidated.generation = next_generation(&invalidated)?;
            invalidated.open = false;
            Ok((session.clone(), invalidated))
        })
        .collect::<Result<Vec<_>, SessionError>>()?;
    let mut updates = candidates
        .iter()
        .map(|(_, record)| Ok((session_key(&record.request)?, encode(record)?)))
        .collect::<Result<Vec<_>, SessionError>>()?;
    updates.append(&mut companion);
    store.update_local_batch(updates)?;
    for (session, invalidated) in candidates {
        let invalidated_generation = invalidated
            .generation
            .checked_sub(1)
            .ok_or(SessionError::GenerationExhausted)?;
        registry.replace(&session, invalidated);
        invalidated_generations.push((session.clone(), invalidated_generation));
        invalidated_sessions.push(session);
    }

    let mut cancelled_preparations = 0_usize;
    let mut unresolved_left_for_resolution = 0_usize;
    let mut executed_untouched = 0_usize;
    for activity in activities {
        if !invalidated_sessions.contains(&activity.session) {
            continue;
        }
        match activity.state {
            PreparationState::Prepared | PreparationState::Signed => {
                if !activity.cancelled {
                    activity.cancelled = true;
                    cancelled_preparations += 1;
                }
            }
            PreparationState::Queued | PreparationState::Unknown => {
                if event.reason == InvalidationReason::GrantRevokedOnChain && !activity.cancelled {
                    activity.cancelled = true;
                    cancelled_preparations += 1;
                }
                activity.resolution_continues = true;
                unresolved_left_for_resolution += 1;
            }
            PreparationState::NonceConsumed => {
                activity.resolution_continues = true;
                unresolved_left_for_resolution += 1;
            }
            PreparationState::Executed | PreparationState::Failed => {
                executed_untouched += 1;
            }
        }
    }
    Ok(InvalidationReport {
        reason: event.reason,
        observed_sequence: event.observed_sequence,
        invalidated_sessions,
        invalidated_generations,
        cancelled_preparations,
        unresolved_left_for_resolution,
        executed_untouched,
    })
}
