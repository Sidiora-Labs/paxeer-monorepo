//! Finality-bound delivery of AI market events.
//!
//! The producer side admits one captured market outbox row only after a
//! verified block commit from the pinned validator set binds the rank-4 row
//! to its exact block height, hash, app hash and store proof on the configured
//! chain, maps it onto the existing program event family,
//! and keeps a durable per-market outbox whose event identifiers, bodies and
//! subject sequences survive restart. The receiver side verifies the published
//! signature scheme and applies each event once, in subject order, durably.

use std::collections::BTreeMap;
use std::fs::{self, File, OpenOptions};
use std::io::{ErrorKind, Write};
use std::path::{Path, PathBuf};

use crate::chain::{ChainPolicy, CommitCertificate, ProgramStateProof};
use ed25519_dalek::{Signer, SigningKey};
use layerx_indexer::ai_market::{BlockCommitProof, FinalityAuthority, VerifiedCommit};
use layerx_programs_ai_market::codec::state_digest;
use layerx_programs_ai_market::queries::{SnapshotBinding, BINDING_MAX_BYTES, FINALIZED_RANK};
use layerx_programs_ai_market::{Digest32, Presence};
use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use subtle::ConstantTimeEq;
use zeroize::Zeroizing;

use crate::encoding::{digest, hex_encode};
use crate::error::WebhookError;
use crate::events::{
    EndpointId, EventDraft, EventId, EventKind, Principal, ProtocolEvent, ProtocolFact, SubjectId,
    Verification,
};
use crate::http::MAXIMUM_REQUEST_BYTES;
use crate::scheme::{self, Presentation};
use crate::trusted::IngressRole;

/// Domain of the stable market event identifier.
pub const VIEW_EVENT_DOMAIN: &[u8] = b"PAXAI/view-event/v1";
/// Prefix of every market event identifier.
pub const EVENT_ID_PREFIX: &str = "0x";
/// Prefix of every per-market ordering subject.
pub const SUBJECT_PREFIX: &str = "paxai_market_";
/// The only capture decoder version this adapter admits.
pub const DECODER_VERSION: u16 = 1;
/// Weakest verification rank that permits finalized publication.
pub const MINIMUM_PUBLICATION_RANK: u8 = FINALIZED_RANK;
/// Largest exact serialized market state a snapshot binds.
pub const MAXIMUM_STATE_BYTES: usize = 262_144;
/// Largest store proof admitted per row.
pub const MAXIMUM_EVIDENCE_BYTES: usize = 1_048_576;
/// Largest validator signature set admitted per row.
pub const MAXIMUM_VALIDATORS: usize = 256;
/// Largest registered mutation operation name carried in an event topic.
pub const MAXIMUM_ACTION_BYTES: usize = 55;
/// Largest resume page.
pub const MAXIMUM_PAGE_ROWS: usize = 32;
/// Unpublished rows a market stream holds before capture stops.
pub const MAXIMUM_UNPUBLISHED_ROWS: usize = 4_096;

/// Durable per-tenant market limit, including revoked streams (their replay floors survive).
pub const MAXIMUM_MARKETS: usize = 128;
/// Maximum provisional identities retained per market until finality or rollback.
pub const MAXIMUM_PROVISIONAL: usize = 256;
/// Maximum retained admitted identities per market; published history must be pruned.
pub const MAXIMUM_ADMITTED: usize = 4_096;
/// Tenant-wide identity/row limit, independent of the number of markets.
pub const MAXIMUM_RETAINED_IDENTITIES: usize = 8_192;
/// Public market facts version. Stored v1 events remain byte-identical on replay;
/// v1's checkpoint_id was a store proof digest, never a checkpoint identity.
pub const MARKET_FACTS_SCHEMA: &str = "paxai.webhook-facts.v2";

const MAXIMUM_ALERTS: usize = 256;
const MAXIMUM_APPLIED: usize = 4_096;
const MAXIMUM_SUBJECTS: usize = 1_024;
const MAXIMUM_STORE_BYTES: u64 = 64 * 1024 * 1024;
const CURSOR_PREFIX: &str = "paxaiwc1_";
const CURSOR_DOMAIN: &[u8] = b"PAXAI/webhook-cursor/v1";
const CURSOR_BODY_BYTES: usize = 2 + 16 + 32;
const STATE_FILE: &str = "state.json";
const STAGED_FILE: &str = "state.json.staged";
const LOCK_FILE: &str = "state.lock";

/// Returns the stable event identifier of one market effect.
///
/// # Errors
/// Returns [`WebhookError::InvalidRequest`] if the identifier violates the
/// webhook token rule.
pub fn market_event_id(
    market_id: &[u8; 32],
    source_activity_id: &[u8; 32],
    effect_ordinal: u16,
    ai_event_tag: u16,
) -> Result<EventId, WebhookError> {
    let mut hasher = Sha256::new();
    hasher.update(VIEW_EVENT_DOMAIN);
    hasher.update([0]);
    hasher.update(market_id);
    hasher.update(source_activity_id);
    hasher.update(effect_ordinal.to_be_bytes());
    hasher.update(ai_event_tag.to_be_bytes());
    let value: [u8; 32] = hasher.finalize().into();
    EventId::new(format!("{EVENT_ID_PREFIX}{}", hex_encode(&value)))
}

/// Returns the ordering subject of one market stream.
///
/// # Errors
/// Returns [`WebhookError::InvalidRequest`] if the subject violates the token rule.
pub fn market_subject(market_id: &[u8; 32]) -> Result<SubjectId, WebhookError> {
    SubjectId::new(format!("{SUBJECT_PREFIX}{}", hex_encode(market_id)))
}

/// Upstream manifest material that rides with a capture but is never public.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct UpstreamManifest {
    pub display_name: Option<String>,
    pub result_locator: Option<String>,
}

/// One durable capture outbox row written with its canonical snapshot binding.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct CapturedEvent {
    pub binding: SnapshotBinding,
    pub snapshot_id: [u8; 32],
    pub event_id: String,
    pub decoder_version: u16,
    pub action: String,
    pub ai_event_tag: u16,
    pub source_activity_id: [u8; 32],
    pub effect_ordinal: u16,
    pub receipt_digest: [u8; 32],
    pub manifest: UpstreamManifest,
}

/// The exact state bytes a capture row binds and, once the binding claims
/// finalized rank, the verified block commit for its root.
#[derive(Clone, Copy, Debug)]
pub struct Evidence<'a> {
    pub state: &'a [u8],
    pub proof: Option<BlockCommitProof<'a>>,
}

/// The deployment chain domain plus the pinned Comet chain id and validator set.
#[derive(Clone, Debug)]
pub struct FinalityPolicy {
    pub chain_domain: [u8; 32],
    pub authority: FinalityAuthority,
}

/// Proof that the caller presented a verified internal producer leaf.
#[derive(Debug)]
pub struct ProducerGrant(());

impl ProducerGrant {
    /// The ingress caller must obtain `role` from its verified TLS peer leaf.
    pub fn from_authenticated_role(role: IngressRole) -> Result<Self, MarketEventError> {
        if role != IngressRole::Producer {
            return Err(MarketEventError::ProducerRoleRequired);
        }
        Ok(Self(()))
    }
    /// Accepts only a client leaf holding the producer role.
    ///
    /// # Errors
    /// Returns [`MarketEventError::ProducerRoleRequired`] for any other leaf.
    pub fn from_certificate(leaf: &[u8]) -> Result<Self, MarketEventError> {
        match IngressRole::from_certificate(leaf) {
            Ok(IngressRole::Producer) => Ok(Self(())),
            Ok(IngressRole::Operator) | Err(_) => Err(MarketEventError::ProducerRoleRequired),
        }
    }
}

/// Why a market stream stopped finalized publication.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum QuarantineCause {
    DecoderVersion,
    ChainDomain,
    StateDigest,
    SnapshotId,
    EventId,
    CommitChain,
    FinalityConflict,
    EventConflict,
    FinalityBoundary,
}

impl QuarantineCause {
    /// Returns the operator error category.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::DecoderVersion => "unsupported_decoder_version",
            Self::ChainDomain => "chain_domain_mismatch",
            Self::StateDigest => "state_digest_mismatch",
            Self::SnapshotId => "snapshot_id_mismatch",
            Self::EventId => "event_id_mismatch",
            Self::CommitChain => "commit_chain_mismatch",
            Self::FinalityConflict => "finality_conflict",
            Self::EventConflict => "event_conflict",
            Self::FinalityBoundary => "finality_boundary_crossed",
        }
    }
}

/// An operator alert carrying identifiers and an error category only.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
pub struct OperatorAlert {
    pub market_id: String,
    pub event_id: Option<String>,
    pub snapshot_id: Option<String>,
    pub category: String,
}

/// Exact refusals of the market event surface.
#[derive(Debug)]
pub enum MarketEventError {
    /// A row, cursor or argument violated a declared bound or shape.
    InvalidRequest,
    /// Capture precedes the durable source replay floor; resume requires resync.
    HistoryExpired,
    /// The caller does not hold the internal producer role.
    ProducerRoleRequired,
    /// The block commit did not verify against the pinned validator set or
    /// proves another block, height, app hash or store proof; nothing was admitted.
    CommitRejected,
    /// This call quarantined the market stream.
    Quarantined(QuarantineCause),
    /// The market stream is quarantined and refuses finalized publication.
    StreamQuarantined,
    /// Unpublished rows reached their bound; capture stops and alerts.
    CaptureStopped,
    /// The tenant holds no current authorization for the market stream.
    Revoked,
    /// The webhook surface refused the event or its durable store failed.
    Webhook(WebhookError),
}

impl From<WebhookError> for MarketEventError {
    fn from(value: WebhookError) -> Self {
        Self::Webhook(value)
    }
}

impl From<std::io::Error> for MarketEventError {
    fn from(value: std::io::Error) -> Self {
        Self::Webhook(WebhookError::Io(value))
    }
}

trait Persisted {
    fn persists(&self) -> bool;
}

impl Persisted for MarketEventError {
    fn persists(&self) -> bool {
        matches!(self, Self::Quarantined(_) | Self::CaptureStopped)
    }
}

impl Persisted for WebhookError {
    fn persists(&self) -> bool {
        false
    }
}

struct Store {
    directory: PathBuf,
}

impl Store {
    fn open(directory: &Path) -> Result<Self, WebhookError> {
        fs::create_dir_all(directory)?;
        Ok(Self {
            directory: directory.to_path_buf(),
        })
    }

    fn lock(&self) -> Result<File, WebhookError> {
        Ok(OpenOptions::new()
            .create(true)
            .truncate(false)
            .write(true)
            .open(self.directory.join(LOCK_FILE))?)
    }

    fn load<T: DeserializeOwned + Default>(&self) -> Result<T, WebhookError> {
        let path = self.directory.join(STATE_FILE);
        match fs::metadata(&path) {
            Ok(metadata) if metadata.len() > MAXIMUM_STORE_BYTES => Err(WebhookError::CorruptStore),
            Ok(_) => {
                serde_json::from_slice(&fs::read(&path)?).map_err(|_| WebhookError::CorruptStore)
            }
            Err(error) if error.kind() == ErrorKind::NotFound => Ok(T::default()),
            Err(error) => Err(WebhookError::Io(error)),
        }
    }

    fn read<T: DeserializeOwned + Default>(&self) -> Result<T, WebhookError> {
        let lock = self.lock()?;
        lock.lock_shared()?;
        self.load()
    }

    fn transact<T, R, E>(&self, apply: impl FnOnce(&mut T) -> Result<R, E>) -> Result<R, E>
    where
        T: Serialize + DeserializeOwned + Default,
        E: From<WebhookError> + Persisted,
    {
        let lock = self.lock()?;
        lock.lock().map_err(WebhookError::Io)?;
        let mut state: T = self.load()?;
        let outcome = apply(&mut state);
        let durable = match &outcome {
            Ok(_) => true,
            Err(error) => error.persists(),
        };
        if durable {
            self.commit(&state)?;
        }
        outcome
    }

    fn commit<T: Serialize>(&self, state: &T) -> Result<(), WebhookError> {
        let bytes = serde_json::to_vec(state).map_err(|_| WebhookError::CorruptStore)?;
        if u64::try_from(bytes.len()).unwrap_or(u64::MAX) > MAXIMUM_STORE_BYTES {
            return Err(WebhookError::Unavailable);
        }
        let staged = self.directory.join(STAGED_FILE);
        let mut file = File::create(&staged)?;
        file.write_all(&bytes)?;
        file.sync_all()?;
        fs::rename(&staged, self.directory.join(STATE_FILE))?;
        File::open(&self.directory)?.sync_all()?;
        Ok(())
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
struct OutboxRow {
    event: ProtocolEvent,
    observed_sequence: u64,
    published: bool,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
struct Admitted {
    sequence: u64,
    #[serde(default)]
    observed_sequence: Option<u64>,
    identity: [u8; 32],
}

#[derive(Clone, Debug, Serialize, Deserialize)]
struct Provisional {
    identity: [u8; 32],
    observed_sequence: u64,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct MarketStream {
    granted: bool,
    quarantine: Option<QuarantineCause>,
    last_sequence: u64,
    pruned_through: u64,
    finality_boundary: u64,
    #[serde(default)]
    replay_through: Option<u64>,
    #[serde(default)]
    pruned_observed_through: Option<u64>,
    latest_snapshot: Option<[u8; 32]>,
    rows: BTreeMap<u64, OutboxRow>,
    admitted: BTreeMap<String, Admitted>,
    provisional: BTreeMap<String, Provisional>,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct OutboxState {
    streams: BTreeMap<String, MarketStream>,
    alerts: Vec<OperatorAlert>,
}

impl OutboxState {
    fn stop(
        &mut self,
        market: &str,
        event_id: Option<&str>,
        snapshot_id: Option<&[u8; 32]>,
        stop: Stop,
    ) -> MarketEventError {
        let (category, error) = match stop {
            Stop::Quarantine(cause) => {
                if let Some(stream) = self.streams.get_mut(market) {
                    stream.quarantine = Some(cause);
                }
                (cause.as_str(), MarketEventError::Quarantined(cause))
            }
            Stop::Capacity => ("outbox_capacity", MarketEventError::CaptureStopped),
            Stop::Refuse(error) => return error,
        };
        if self.alerts.len() >= MAXIMUM_ALERTS {
            self.alerts.remove(0);
        }
        self.alerts.push(OperatorAlert {
            market_id: market.to_owned(),
            event_id: event_id.map(str::to_owned),
            snapshot_id: snapshot_id.map(|value| hex_encode(value)),
            category: category.to_owned(),
        });
        error
    }
}

enum Stop {
    Quarantine(QuarantineCause),
    Capacity,
    Refuse(MarketEventError),
}

/// The result of admitting one captured row.
#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub enum Admission {
    /// Matching finality evidence is not yet available; publication waits.
    Delayed,
    /// The row holds its durable subject sequence and immutable event.
    Finalized {
        event: ProtocolEvent,
        duplicate: bool,
    },
}

/// One resume answer for a webhook endpoint.
#[derive(Clone, Debug, Eq, PartialEq, Serialize)]
pub enum ResumePage {
    /// Events strictly after the cursor position, in subject order.
    Events {
        events: Vec<ProtocolEvent>,
        next_cursor: String,
        has_more: bool,
    },
    /// The cursor is no longer resumable; the receiver must resynchronize
    /// from the named finalized snapshot through the named sequence.
    Resync {
        snapshot_id: Option<[u8; 32]>,
        through_sequence: u64,
        next_cursor: String,
    },
}

/// Webhook resume cursor keys with bounded rotation overlap.
#[derive(Clone)]
pub struct CursorKeys {
    current: u8,
    keys: BTreeMap<u8, Zeroizing<[u8; 32]>>,
}

impl CursorKeys {
    /// Issues cursors under `current` and still accepts the retained keys.
    ///
    /// # Errors
    /// Returns [`MarketEventError::InvalidRequest`] when a key identifier repeats.
    pub fn new(
        current: (u8, [u8; 32]),
        retained: &[(u8, [u8; 32])],
    ) -> Result<Self, MarketEventError> {
        let mut keys = BTreeMap::new();
        for (id, key) in std::iter::once(&current).chain(retained) {
            if keys.insert(*id, Zeroizing::new(*key)).is_some() {
                return Err(MarketEventError::InvalidRequest);
            }
        }
        Ok(Self {
            current: current.0,
            keys,
        })
    }

    fn encode(
        &self,
        principal: &Principal,
        endpoint: &EndpointId,
        market: &str,
        position: u64,
    ) -> String {
        self.keys
            .get(&self.current)
            .map_or_else(String::new, |key| {
                cursor_text(self.current, key, principal, endpoint, market, position)
            })
    }

    fn decode(
        &self,
        principal: &Principal,
        endpoint: &EndpointId,
        market: &str,
        cursor: &str,
    ) -> Result<Option<u64>, MarketEventError> {
        let body = cursor
            .strip_prefix(CURSOR_PREFIX)
            .filter(|body| body.len() == CURSOR_BODY_BYTES && body.is_ascii())
            .ok_or(WebhookError::InvalidCursor)?;
        let key_id = u8::from_str_radix(&body[..2], 16).map_err(|_| WebhookError::InvalidCursor)?;
        let position =
            u64::from_str_radix(&body[2..18], 16).map_err(|_| WebhookError::InvalidCursor)?;
        let Some(key) = self.keys.get(&key_id) else {
            return Ok(None);
        };
        let expected = cursor_text(key_id, key, principal, endpoint, market, position);
        if expected.as_bytes().ct_eq(cursor.as_bytes()).unwrap_u8() != 1 {
            return Err(WebhookError::InvalidCursor.into());
        }
        Ok(Some(position))
    }
}

fn cursor_text(
    key_id: u8,
    key: &[u8; 32],
    principal: &Principal,
    endpoint: &EndpointId,
    market: &str,
    position: u64,
) -> String {
    let tag = cursor_tag(key, principal, endpoint, market, position);
    format!(
        "{CURSOR_PREFIX}{key_id:02x}{position:016x}{}",
        hex_encode(&tag[..16])
    )
}

fn cursor_tag(
    key: &[u8; 32],
    principal: &Principal,
    endpoint: &EndpointId,
    market: &str,
    position: u64,
) -> [u8; 32] {
    let mut inner_pad = Zeroizing::new([0x36_u8; 64]);
    let mut outer_pad = Zeroizing::new([0x5c_u8; 64]);
    for (index, byte) in key.iter().enumerate() {
        inner_pad[index] ^= byte;
        outer_pad[index] ^= byte;
    }
    let mut inner = Sha256::new();
    inner.update(*inner_pad);
    inner.update(CURSOR_DOMAIN);
    inner.update([0]);
    inner.update(principal.as_str().as_bytes());
    inner.update([0]);
    inner.update(endpoint.as_str().as_bytes());
    inner.update([0]);
    inner.update(market.as_bytes());
    inner.update(position.to_be_bytes());
    let mut outer = Sha256::new();
    outer.update(*outer_pad);
    outer.update(inner.finalize());
    outer.finalize().into()
}

/// The durable per-tenant outbox of finalized AI market events.
pub struct MarketOutbox {
    store: Store,
    principal: Principal,
    policy: CapturePolicy,
    cursor_keys: CursorKeys,
}

impl MarketOutbox {
    /// Opens the tenant outbox stored under `directory`.
    ///
    /// # Errors
    /// Returns [`WebhookError::Io`] when the directory cannot be created.
    pub fn open(
        directory: &Path,
        principal: Principal,
        policy: FinalityPolicy,
        cursor_keys: CursorKeys,
    ) -> Result<Self, MarketEventError> {
        Self::open_portable(
            directory,
            principal,
            CapturePolicy {
                chain_domain: policy.chain_domain,
                consensus: ChainPolicy::from_native(&policy.authority)?,
            },
            cursor_keys,
        )
    }

    pub fn open_portable(
        directory: &Path,
        principal: Principal,
        policy: CapturePolicy,
        cursor_keys: CursorKeys,
    ) -> Result<Self, MarketEventError> {
        policy.consensus.validate()?;
        Ok(Self {
            store: Store::open(directory)?,
            principal,
            policy,
            cursor_keys,
        })
    }

    pub fn admit_document(
        &self,
        producer: &ProducerGrant,
        document: &CaptureDocument,
    ) -> Result<Admission, MarketEventError> {
        let row = document.row()?;
        well_formed_document(&row, &document.state, &document.store_proof)?;
        let verified = document
            .commit
            .verify(&self.policy.consensus, &document.store_proof)
            .map_err(|_| MarketEventError::CommitRejected)?;
        let evidence = Evidence {
            state: &document.state,
            proof: None,
        };
        self.admit_examined(
            producer,
            &row,
            examine(
                &self.policy,
                &row,
                &evidence,
                Some((&verified, &document.store_proof)),
            ),
        )
    }

    fn admit_examined(
        &self,
        _producer: &ProducerGrant,
        row: &CapturedEvent,
        examined: Result<Examined, Stop>,
    ) -> Result<Admission, MarketEventError> {
        let market = hex_encode(row.binding.market.as_bytes());
        self.store.transact(|state: &mut OutboxState| {
            if !state.streams.contains_key(&market) {
                return Err(MarketEventError::Revoked);
            }
            let retained: usize = state
                .streams
                .values()
                .map(|s| s.admitted.len() + s.provisional.len())
                .sum();
            let stream = state
                .streams
                .get_mut(&market)
                .ok_or(MarketEventError::Revoked)?;
            if retained >= MAXIMUM_RETAINED_IDENTITIES
                && !stream.admitted.contains_key(&row.event_id)
                && !stream.provisional.contains_key(&row.event_id)
            {
                return Err(state.stop(
                    &market,
                    Some(&row.event_id),
                    Some(&row.snapshot_id),
                    Stop::Capacity,
                ));
            }
            admit_into(stream, &self.principal, row, examined).map_err(|stop| {
                state.stop(&market, Some(&row.event_id), Some(&row.snapshot_id), stop)
            })
        })
    }

    /// Hands durable rows to hosted delivery before acknowledging them. A crash
    /// between the two commits replays the same event into hosted deduplication.
    pub fn drain(
        &self,
        service: &crate::HostedService,
        now: u64,
    ) -> Result<usize, MarketEventError> {
        self.store.transact(|state: &mut OutboxState| {
            let mut count = 0;
            for stream in state
                .streams
                .values_mut()
                .filter(|s| s.granted && s.quarantine.is_none())
            {
                for row in stream.rows.values_mut().filter(|row| !row.published) {
                    service.publish(
                        &crate::trusted::TrustedEvent::from_market(row.event.clone())?,
                        now,
                    )?;
                    row.published = true;
                    count += 1;
                }
            }
            Ok(count)
        })
    }

    /// Records the tenant's current authorization for one market stream.
    ///
    /// # Errors
    /// Returns store failures.
    pub fn grant(&self, market_id: &[u8; 32]) -> Result<(), MarketEventError> {
        self.set_grant(market_id, true)
    }

    /// Revokes the tenant's authorization; later admission, delivery and
    /// resume for the market refuse.
    ///
    /// # Errors
    /// Returns store failures.
    pub fn revoke(&self, market_id: &[u8; 32]) -> Result<(), MarketEventError> {
        self.set_grant(market_id, false)
    }

    fn set_grant(&self, market_id: &[u8; 32], granted: bool) -> Result<(), MarketEventError> {
        let market = hex_encode(market_id);
        self.store.transact(|state: &mut OutboxState| {
            if !state.streams.contains_key(&market) && state.streams.len() >= MAXIMUM_MARKETS {
                return Err(state.stop(&market, None, None, Stop::Capacity));
            }
            if !granted && !state.streams.contains_key(&market) {
                return Err(MarketEventError::Revoked);
            }
            state.streams.entry(market).or_default().granted = granted;
            Ok::<(), MarketEventError>(())
        })
    }

    /// Admits one captured row. Rows below rank 4 wait; rows whose evidence
    /// does not bind the configured chain, exact state, snapshot, identifier,
    /// decoder and committed root quarantine the market stream.
    ///
    /// # Errors
    /// Returns [`MarketEventError::InvalidRequest`] for an unbounded or
    /// malformed row, [`MarketEventError::Revoked`] without a current grant,
    /// [`MarketEventError::Quarantined`] for the quarantining cause,
    /// [`MarketEventError::StreamQuarantined`] once quarantined,
    /// [`MarketEventError::CommitRejected`] for a block commit that does
    /// not verify or proves another block, height, app hash or store proof,
    /// [`MarketEventError::CaptureStopped`] when unpublished rows are full, and
    /// store failures.
    pub fn admit(
        &self,
        _producer: &ProducerGrant,
        row: &CapturedEvent,
        evidence: &Evidence<'_>,
    ) -> Result<Admission, MarketEventError> {
        well_formed(row, evidence)?;
        let verified = evidence
            .proof
            .map(|proof| {
                CommitCertificate::from_proof(&proof)?
                    .verify(&self.policy.consensus, proof.store_proof)
            })
            .transpose()
            .map_err(|_| MarketEventError::CommitRejected)?;
        self.admit_examined(
            _producer,
            row,
            examine(
                &self.policy,
                row,
                evidence,
                verified.as_ref().zip(evidence.proof.map(|p| p.store_proof)),
            ),
        )
    }

    /// Removes provisional rows observed after `observed_through`. A rollback
    /// below the verified finality boundary is refused and quarantines.
    ///
    /// # Errors
    /// Returns [`MarketEventError::Quarantined`] with
    /// [`QuarantineCause::FinalityBoundary`], [`MarketEventError::StreamQuarantined`],
    /// and store failures.
    pub fn rollback(
        &self,
        market_id: &[u8; 32],
        observed_through: u64,
    ) -> Result<usize, MarketEventError> {
        let market = hex_encode(market_id);
        self.store.transact(|state: &mut OutboxState| {
            let stream = state
                .streams
                .get_mut(&market)
                .ok_or(MarketEventError::Revoked)?;
            if stream.quarantine.is_some() {
                return Err(MarketEventError::StreamQuarantined);
            }
            if observed_through < stream.finality_boundary {
                return Err(state.stop(
                    &market,
                    None,
                    None,
                    Stop::Quarantine(QuarantineCause::FinalityBoundary),
                ));
            }
            let before = stream.provisional.len();
            stream
                .provisional
                .retain(|_, row| row.observed_sequence <= observed_through);
            Ok(before.saturating_sub(stream.provisional.len()))
        })
    }

    /// Returns unpublished finalized events in subject order.
    ///
    /// # Errors
    /// Returns [`MarketEventError::Revoked`] without a current grant and store failures.
    pub fn pending(&self, market_id: &[u8; 32]) -> Result<Vec<ProtocolEvent>, MarketEventError> {
        let state: OutboxState = self.store.read()?;
        let stream = granted(&state, &hex_encode(market_id))?;
        Ok(stream
            .rows
            .values()
            .filter(|row| !row.published)
            .map(|row| row.event.clone())
            .collect())
    }

    /// Returns only durably admitted, granted and unpublished events for trusted
    /// delivery adapters; callers cannot promote an arbitrary event this way.
    pub fn pending_trusted(
        &self,
        market_id: &[u8; 32],
    ) -> Result<Vec<crate::trusted::TrustedEvent>, MarketEventError> {
        let state: OutboxState = self.store.read()?;
        let stream = granted(&state, &hex_encode(market_id))?;
        if stream.quarantine.is_some() {
            return Err(MarketEventError::StreamQuarantined);
        }
        stream
            .rows
            .values()
            .filter(|row| !row.published)
            .map(|row| {
                crate::trusted::TrustedEvent::from_market(row.event.clone())
                    .map_err(MarketEventError::from)
            })
            .collect()
    }

    /// Records that `event` reached an accepting endpoint.
    ///
    /// # Errors
    /// Returns [`WebhookError::UnknownDelivery`] for an event this outbox did not
    /// admit and store failures.
    pub fn acknowledge(
        &self,
        market_id: &[u8; 32],
        event: &EventId,
    ) -> Result<(), MarketEventError> {
        let market = hex_encode(market_id);
        self.store.transact(|state: &mut OutboxState| {
            let stream = state
                .streams
                .get_mut(&market)
                .ok_or(WebhookError::UnknownDelivery)?;
            let sequence = stream
                .admitted
                .get(event.as_str())
                .map(|admitted| admitted.sequence)
                .ok_or(WebhookError::UnknownDelivery)?;
            if let Some(row) = stream.rows.get_mut(&sequence) {
                row.published = true;
            }
            Ok(())
        })
    }

    /// Releases published rows through `sequence`; unpublished rows are never
    /// released to meet retention.
    ///
    /// # Errors
    /// Returns [`MarketEventError::InvalidRequest`] when an unpublished row
    /// would be released and store failures.
    pub fn prune(&self, market_id: &[u8; 32], sequence: u64) -> Result<usize, MarketEventError> {
        let market = hex_encode(market_id);
        self.store.transact(|state: &mut OutboxState| {
            let stream = state
                .streams
                .get_mut(&market)
                .ok_or(MarketEventError::InvalidRequest)?;
            if sequence > stream.last_sequence
                || stream
                    .rows
                    .range(..=sequence)
                    .any(|(_, row)| !row.published)
            {
                return Err(MarketEventError::InvalidRequest);
            }
            let before = stream.rows.len();
            // Only a published contiguous subject prefix proves a replay horizon.
            // Keep the horizon strictly before every retained finalized row,
            // including unpublished rows admitted out of source order.
            let removed_through = stream
                .rows
                .range(..=sequence)
                .map(|(_, row)| row.observed_sequence)
                .chain(stream.pruned_observed_through)
                .chain((sequence == stream.last_sequence).then_some(stream.finality_boundary))
                .max();
            stream.pruned_observed_through = removed_through;
            let retained_floor = stream
                .rows
                .range(sequence.saturating_add(1)..)
                .map(|(_, row)| row.observed_sequence)
                .min();
            let proven = removed_through.and_then(|through| match retained_floor {
                Some(0) => None,
                Some(floor) => Some(through.min(floor - 1)),
                None => Some(through),
            });
            if let Some(proven) = proven {
                let through = stream.replay_through.map_or(proven, |old| old.max(proven));
                stream.replay_through = Some(through);
                stream
                    .provisional
                    .retain(|_, row| row.observed_sequence > through);
                stream.admitted.retain(|_, row| {
                    row.sequence > sequence
                        || row
                            .observed_sequence
                            .map_or(through < stream.finality_boundary, |observed| {
                                observed > through
                            })
                });
            }
            stream.rows.retain(|position, _| *position > sequence);
            stream.pruned_through = stream.pruned_through.max(sequence);
            Ok(before.saturating_sub(stream.rows.len()))
        })
    }

    /// Resumes one endpoint after an authenticated cursor. The caller passes
    /// the principal its current session authenticated; the cursor grants no
    /// access of its own. An expired cursor answers an explicit resync.
    ///
    /// # Errors
    /// Returns [`MarketEventError::InvalidRequest`] for an unbounded page,
    /// [`MarketEventError::Revoked`] for another principal or a revoked grant,
    /// [`WebhookError::InvalidCursor`] for a forged or foreign cursor, and
    /// store failures.
    pub fn resume(
        &self,
        authorized: &Principal,
        endpoint: &EndpointId,
        market_id: &[u8; 32],
        cursor: Option<&str>,
        limit: usize,
    ) -> Result<ResumePage, MarketEventError> {
        if limit == 0 || limit > MAXIMUM_PAGE_ROWS || cursor.is_some_and(|value| value.len() > 64) {
            return Err(MarketEventError::InvalidRequest);
        }
        if authorized != &self.principal {
            return Err(MarketEventError::Revoked);
        }
        let market = hex_encode(market_id);
        let state: OutboxState = self.store.read()?;
        let stream = granted(&state, &market)?;
        let position = match cursor {
            None => Some(stream.pruned_through),
            Some(value) => self
                .cursor_keys
                .decode(authorized, endpoint, &market, value)?,
        };
        let Some(position) = position.filter(|position| *position >= stream.pruned_through) else {
            return Ok(ResumePage::Resync {
                snapshot_id: stream.latest_snapshot,
                through_sequence: stream.last_sequence,
                next_cursor: self.cursor_keys.encode(
                    authorized,
                    endpoint,
                    &market,
                    stream.last_sequence,
                ),
            });
        };
        if position > stream.last_sequence {
            return Err(WebhookError::InvalidCursor.into());
        }
        let mut selected = stream
            .rows
            .range(position.saturating_add(1)..)
            .take(limit.saturating_add(1));
        let events: Vec<(u64, ProtocolEvent)> = selected
            .by_ref()
            .take(limit)
            .map(|(sequence, row)| (*sequence, row.event.clone()))
            .collect();
        let has_more = selected.next().is_some();
        let reached = events.last().map_or(position, |(sequence, _)| *sequence);
        Ok(ResumePage::Events {
            events: events.into_iter().map(|(_, event)| event).collect(),
            next_cursor: self
                .cursor_keys
                .encode(authorized, endpoint, &market, reached),
            has_more,
        })
    }

    /// Returns the operator alerts recorded by this outbox.
    ///
    /// # Errors
    /// Returns store failures.
    pub fn alerts(&self) -> Result<Vec<OperatorAlert>, MarketEventError> {
        let state: OutboxState = self.store.read()?;
        Ok(state.alerts)
    }

    /// Returns the quarantine cause of a market stream, if any.
    ///
    /// # Errors
    /// Returns store failures.
    pub fn quarantine(
        &self,
        market_id: &[u8; 32],
    ) -> Result<Option<QuarantineCause>, MarketEventError> {
        let state: OutboxState = self.store.read()?;
        Ok(state
            .streams
            .get(&hex_encode(market_id))
            .and_then(|stream| stream.quarantine))
    }
}

fn granted<'a>(state: &'a OutboxState, market: &str) -> Result<&'a MarketStream, MarketEventError> {
    state
        .streams
        .get(market)
        .filter(|stream| stream.granted)
        .ok_or(MarketEventError::Revoked)
}

fn nonzero(value: &[u8; 32]) -> bool {
    value.iter().any(|byte| *byte != 0)
}

fn well_formed(row: &CapturedEvent, evidence: &Evidence<'_>) -> Result<(), MarketEventError> {
    let mut canonical = [0_u8; BINDING_MAX_BYTES];
    let finalized = row.binding.rank >= MINIMUM_PUBLICATION_RANK;
    let shaped = row.binding.encode(&mut canonical).is_ok()
        && evidence.state.len() <= MAXIMUM_STATE_BYTES
        && evidence.proof.map_or(!finalized, |proof| {
            !proof.store_proof.is_empty()
                && proof.store_proof.len() <= MAXIMUM_EVIDENCE_BYTES
                && proof.commit.signatures.len() <= MAXIMUM_VALIDATORS
        })
        && nonzero(&row.source_activity_id)
        && nonzero(&row.receipt_digest)
        && !row.action.is_empty()
        && row.action.len() <= MAXIMUM_ACTION_BYTES
        && row
            .action
            .bytes()
            .all(|byte| byte.is_ascii_uppercase() || byte.is_ascii_digit() || byte == b'_');
    if !shaped {
        return Err(MarketEventError::InvalidRequest);
    }
    EventId::new(row.event_id.as_str())?;
    Ok(())
}

struct Examined {
    identity: [u8; 32],
    finalized: bool,
}

fn examine(
    policy: &CapturePolicy,
    row: &CapturedEvent,
    evidence: &Evidence<'_>,
    verified: Option<(&VerifiedCommit, &[u8])>,
) -> Result<Examined, Stop> {
    let binding = &row.binding;
    if row.decoder_version != DECODER_VERSION {
        return Err(Stop::Quarantine(QuarantineCause::DecoderVersion));
    }
    if binding.chain.bytes() != policy.chain_domain {
        return Err(Stop::Quarantine(QuarantineCause::ChainDomain));
    }
    if state_digest(evidence.state).ok() != Some(binding.state_digest) {
        return Err(Stop::Quarantine(QuarantineCause::StateDigest));
    }
    if binding.snapshot_id().ok().map(Digest32::bytes) != Some(row.snapshot_id) {
        return Err(Stop::Quarantine(QuarantineCause::SnapshotId));
    }
    let expected = market_event_id(
        binding.market.as_bytes(),
        &row.source_activity_id,
        row.effect_ordinal,
        row.ai_event_tag,
    )
    .map_err(|_| Stop::Quarantine(QuarantineCause::EventId))?;
    if expected.as_str() != row.event_id {
        return Err(Stop::Quarantine(QuarantineCause::EventId));
    }
    let identity = identity(row);
    match verified {
        Some((proof, membership)) if binding.rank >= MINIMUM_PUBLICATION_RANK => {
            certify(policy, binding, proof, membership, evidence.state)?;
            Ok(Examined {
                identity,
                finalized: true,
            })
        }
        _ => Ok(Examined {
            identity,
            finalized: false,
        }),
    }
}

fn certify(
    policy: &CapturePolicy,
    binding: &SnapshotBinding,
    verified: &VerifiedCommit,
    membership: &[u8],
    state: &[u8],
) -> Result<(), Stop> {
    // Match the same SnapshotBindingV2 commit fields that indexer promotion builds
    // through VerifiedCommit::finality_evidence before admitting the outbox row.
    let rejected = || Stop::Refuse(MarketEventError::CommitRejected);
    binding.finalized_head().map_err(|_| rejected())?;
    if verified.chain_id != policy.consensus.chain_id {
        return Err(Stop::Quarantine(QuarantineCause::CommitChain));
    }
    if verified.height == binding.execution_height
        && verified.app_hash != binding.native_state_root.bytes()
    {
        return Err(Stop::Quarantine(QuarantineCause::FinalityConflict));
    }
    if verified.height != binding.execution_height
        || binding.execution_height != binding.block_height
        || verified.block_hash != binding.batch_id.bytes()
        || binding.batch_id != binding.block_hash
        || verified.app_hash != binding.native_state_root.bytes()
        || binding.native_state_root != binding.app_hash
        || verified.store_proof_digest != binding.store_proof_digest.bytes()
    {
        return Err(rejected());
    }
    let header =
        layerx_programs_ai_market::queries::read_header(state, binding.chain, binding.program)
            .map_err(|_| rejected())?;
    if header.market != binding.market
        || header.revision != binding.revision
        || header.digest != binding.state_digest
        || header.config != binding.config
        || header.epoch != binding.epoch
        || header.roster != binding.roster
        || layerx_programs_ai_market::queries::policy_digest(state).map_err(|_| rejected())?
            != binding.policy
    {
        return Err(rejected());
    }
    let membership: ProgramStateProof =
        serde_json::from_slice(membership).map_err(|_| rejected())?;
    membership
        .verify(verified.app_hash, binding.program.as_bytes(), state)
        .map_err(|_| rejected())?;
    Ok(())
}

fn identity(row: &CapturedEvent) -> [u8; 32] {
    let binding = &row.binding;
    let mut hasher = Sha256::new();
    hasher.update(row.snapshot_id);
    hasher.update(row.event_id.as_bytes());
    hasher.update([0]);
    hasher.update(row.action.as_bytes());
    hasher.update([0]);
    hasher.update(row.ai_event_tag.to_be_bytes());
    hasher.update(row.decoder_version.to_be_bytes());
    hasher.update(row.source_activity_id);
    hasher.update(row.effect_ordinal.to_be_bytes());
    hasher.update(row.receipt_digest);
    hasher.update(binding.store_proof_digest.bytes());
    hasher.finalize().into()
}

fn admit_into(
    stream: &mut MarketStream,
    principal: &Principal,
    row: &CapturedEvent,
    examined: Result<Examined, Stop>,
) -> Result<Admission, Stop> {
    if !stream.granted {
        return Err(Stop::Refuse(MarketEventError::Revoked));
    }
    if stream.quarantine.is_some() {
        return Err(Stop::Refuse(MarketEventError::StreamQuarantined));
    }
    let examined = examined?;
    if let Some(admitted) = stream.admitted.get(&row.event_id) {
        if admitted.identity != examined.identity {
            return Err(Stop::Quarantine(QuarantineCause::EventConflict));
        }
        let event = match stream.rows.get(&admitted.sequence) {
            Some(stored) => stored.event.clone(),
            None if admitted.observed_sequence.is_none() => {
                return Err(Stop::Refuse(MarketEventError::HistoryExpired))
            }
            None => market_event(principal, row, admitted.sequence)
                .map_err(|error| Stop::Refuse(error.into()))?,
        };
        return Ok(Admission::Finalized {
            event,
            duplicate: true,
        });
    }
    if stream
        .replay_through
        .is_some_and(|through| row.binding.observed_sequence <= through)
    {
        return Err(Stop::Refuse(MarketEventError::HistoryExpired));
    }
    if stream
        .provisional
        .get(&row.event_id)
        .is_some_and(|provisional| provisional.identity != examined.identity)
    {
        return Err(Stop::Quarantine(QuarantineCause::EventConflict));
    }
    if !examined.finalized {
        if !stream.provisional.contains_key(&row.event_id)
            && stream.provisional.len() >= MAXIMUM_PROVISIONAL
        {
            return Err(Stop::Capacity);
        }
        stream.provisional.insert(
            row.event_id.clone(),
            Provisional {
                identity: examined.identity,
                observed_sequence: row.binding.observed_sequence,
            },
        );
        return Ok(Admission::Delayed);
    }
    if stream.admitted.len() >= MAXIMUM_ADMITTED || stream.rows.len() >= MAXIMUM_UNPUBLISHED_ROWS {
        return Err(Stop::Capacity);
    }
    let sequence = stream
        .last_sequence
        .checked_add(1)
        .ok_or(Stop::Refuse(MarketEventError::InvalidRequest))?;
    let event =
        market_event(principal, row, sequence).map_err(|error| Stop::Refuse(error.into()))?;
    stream.provisional.remove(&row.event_id);
    stream.last_sequence = sequence;
    stream.admitted.insert(
        row.event_id.clone(),
        Admitted {
            sequence,
            observed_sequence: Some(row.binding.observed_sequence),
            identity: examined.identity,
        },
    );
    stream.rows.insert(
        sequence,
        OutboxRow {
            event: event.clone(),
            observed_sequence: row.binding.observed_sequence,
            published: false,
        },
    );
    if row.binding.observed_sequence >= stream.finality_boundary {
        stream.finality_boundary = row.binding.observed_sequence;
        stream.latest_snapshot = Some(row.snapshot_id);
    }
    Ok(Admission::Finalized {
        event,
        duplicate: false,
    })
}

fn market_event(
    principal: &Principal,
    row: &CapturedEvent,
    sequence: u64,
) -> Result<ProtocolEvent, WebhookError> {
    let binding = &row.binding;
    let label = Verification::from_wire_rank(binding.rank);
    let receipt = hex_encode(&row.receipt_digest);
    let epoch = match binding.epoch {
        Presence::Present(epoch) => epoch.to_string(),
        Presence::Absent => "absent".to_owned(),
    };
    let values = [
        ("facts_schema", MARKET_FACTS_SCHEMA.to_owned()),
        ("ai_action", row.action.clone()),
        ("ai_event_tag", row.ai_event_tag.to_string()),
        ("market_id", hex_encode(binding.market.as_bytes())),
        ("snapshot_id", hex_encode(&row.snapshot_id)),
        ("epoch", epoch),
        ("config_version", binding.config.get().to_string()),
        ("market_state_revision", binding.revision.to_string()),
        ("source_activity_id", hex_encode(&row.source_activity_id)),
        ("effect_ordinal", row.effect_ordinal.to_string()),
        ("chain_domain", hex_encode(binding.chain.as_bytes())),
        ("program_id", hex_encode(binding.program.as_bytes())),
        ("observed_sequence", binding.observed_sequence.to_string()),
        ("execution_height", binding.execution_height.to_string()),
        ("batch_id", hex_encode(binding.batch_id.as_bytes())),
        (
            "native_state_root",
            hex_encode(binding.native_state_root.as_bytes()),
        ),
        (
            "store_proof_digest",
            hex_encode(binding.store_proof_digest.as_bytes()),
        ),
        ("block_height", binding.block_height.to_string()),
        ("block_hash", hex_encode(binding.block_hash.as_bytes())),
        ("app_hash", hex_encode(binding.app_hash.as_bytes())),
        ("achieved_rank", binding.rank.to_string()),
    ];
    let facts = values
        .into_iter()
        .map(|(name, value)| ProtocolFact::verified(name, value, label, receipt.as_str()))
        .collect::<Result<Vec<_>, _>>()?;
    ProtocolEvent::new(EventDraft {
        id: EventId::new(row.event_id.as_str())?,
        kind: EventKind::Program,
        principal: principal.clone(),
        subject: market_subject(binding.market.as_bytes())?,
        subject_sequence: sequence,
        occurred_at: binding.publication_time_ms / 1_000,
        facts,
    })
}

#[derive(Serialize, Deserialize)]
struct Envelope {
    scheme: String,
    endpoint_id: String,
    receiver_obligation: String,
    event: ProtocolEvent,
}

/// One signed delivery under the published webhook scheme.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SignedDelivery {
    pub id: String,
    pub timestamp: String,
    pub key_id: String,
    pub signature: String,
    pub body: Vec<u8>,
}

impl SignedDelivery {
    /// Signs the exact delivery body of `event` for one endpoint. Every retry
    /// of one event carries the same identifier over a byte-identical body.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an invalid key identifier
    /// and [`WebhookError::CorruptStore`] when the body cannot be encoded.
    pub fn sign(
        event: &ProtocolEvent,
        endpoint: &EndpointId,
        key_id: &str,
        key: &SigningKey,
        timestamp: u64,
    ) -> Result<Self, WebhookError> {
        if !scheme::valid_key_id(key_id) {
            return Err(WebhookError::InvalidRequest);
        }
        let body = serde_json::to_vec(&Envelope {
            scheme: scheme::SCHEME_VERSION.to_owned(),
            endpoint_id: endpoint.as_str().to_owned(),
            receiver_obligation: scheme::RECEIVER_OBLIGATION.to_owned(),
            event: event.clone(),
        })
        .map_err(|_| WebhookError::CorruptStore)?;
        let signature = key.sign(&scheme::canonical_message(
            event.id().as_str(),
            timestamp,
            &body,
        ));
        Ok(Self {
            id: event.id().as_str().to_owned(),
            timestamp: timestamp.to_string(),
            key_id: key_id.to_owned(),
            signature: scheme::signature_header(&signature.to_bytes()),
            body,
        })
    }

    /// Presents this delivery to a receiver clock.
    #[must_use]
    pub fn presentation(&self, now: u64) -> Presentation<'_> {
        Presentation {
            id: &self.id,
            timestamp: &self.timestamp,
            key_id: &self.key_id,
            signature: &self.signature,
            payload: &self.body,
            now,
            tolerance_seconds: scheme::DEFAULT_TOLERANCE_SECONDS,
        }
    }
}

/// The receiver's answer to one verified delivery.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Consumed {
    /// The local effect was applied and recorded with the sequence.
    Applied { sequence: u64, result: String },
    /// The event was already applied; the saved result is returned.
    Duplicate { sequence: u64, result: String },
    /// A sequence gap suspends ordered application until the missing event
    /// arrives or the receiver resynchronizes from a finalized snapshot.
    Suspended { expected: u64, received: u64 },
}

#[derive(Debug, Serialize, Deserialize)]
struct Applied {
    sequence: u64,
    body_digest: [u8; 32],
    result: String,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct SubjectLedger {
    last_sequence: u64,
    applied: BTreeMap<String, Applied>,
    #[serde(default)]
    effects: BTreeMap<String, String>,
}

#[derive(Debug, Default, Serialize, Deserialize)]
struct ConsumerState {
    subjects: BTreeMap<String, SubjectLedger>,
}

/// A durable receiver that applies each finalized market event exactly once.
pub struct MarketConsumer {
    store: Store,
    keys: BTreeMap<String, [u8; 32]>,
}

impl MarketConsumer {
    /// Opens the receiver ledger under `directory` accepting `keys`.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] without keys and store failures.
    pub fn open(directory: &Path, keys: BTreeMap<String, [u8; 32]>) -> Result<Self, WebhookError> {
        if keys.is_empty() {
            return Err(WebhookError::InvalidRequest);
        }
        Ok(Self {
            store: Store::open(directory)?,
            keys,
        })
    }

    /// Verifies a delivery and commits the transaction's business state, result
    /// and sequence together. The closure prepares durable state only; it can
    /// be replayed after an aborted transaction and must not perform external I/O.
    ///
    /// # Errors
    /// Returns [`WebhookError::InvalidRequest`] for an oversized body,
    /// [`WebhookError::StaleTimestamp`] or [`WebhookError::SignatureRejected`]
    /// from the published scheme, [`WebhookError::VerificationRequired`] for
    /// an event below block commitment, [`WebhookError::EventConflict`] for
    /// a changed body under an applied identifier,
    /// [`WebhookError::OrderViolation`] for a decreasing sequence,
    /// [`WebhookError::ReplayCapacity`] when the ledger is full, and store failures.
    pub fn receive(
        &self,
        presentation: &Presentation<'_>,
        effect: impl FnOnce(&mut ConsumerTransaction<'_>) -> String,
    ) -> Result<Consumed, WebhookError> {
        if presentation.payload.len() > MAXIMUM_REQUEST_BYTES {
            return Err(WebhookError::InvalidRequest);
        }
        let verified = scheme::verify(presentation, &self.keys)?;
        let envelope: Envelope = serde_json::from_slice(presentation.payload)
            .map_err(|_| WebhookError::SignatureRejected)?;
        let event = envelope.event;
        if envelope.scheme != scheme::SCHEME_VERSION || event.id().as_str() != verified.id {
            return Err(WebhookError::SignatureRejected);
        }
        event.validate()?;
        if event.kind() != EventKind::Program
            || !event.verification().at_least(Verification::BlockCommitted)
        {
            return Err(WebhookError::VerificationRequired);
        }
        let body_digest = digest(presentation.payload);
        self.store.transact(|state: &mut ConsumerState| {
            if !state.subjects.contains_key(event.subject().as_str())
                && state.subjects.len() >= MAXIMUM_SUBJECTS
            {
                return Err(WebhookError::ReplayCapacity);
            }
            let ledger = state
                .subjects
                .entry(event.subject().as_str().to_owned())
                .or_default();
            apply_once(ledger, &event, body_digest, effect)
        })
    }

    /// Resynchronizes one subject from a finalized snapshot that covers every
    /// event through `through_sequence`.
    ///
    /// # Errors
    /// Returns [`WebhookError::OrderViolation`] for a resync behind the
    /// applied sequence and store failures.
    pub fn resynchronize(
        &self,
        subject: &SubjectId,
        through_sequence: u64,
    ) -> Result<(), WebhookError> {
        self.store.transact(|state: &mut ConsumerState| {
            let ledger = state
                .subjects
                .entry(subject.as_str().to_owned())
                .or_default();
            if through_sequence < ledger.last_sequence {
                return Err(WebhookError::OrderViolation);
            }
            ledger.last_sequence = through_sequence;
            Ok(())
        })
    }

    pub fn effect(&self, subject: &SubjectId, key: &str) -> Result<Option<String>, WebhookError> {
        let state: ConsumerState = self.store.read()?;
        Ok(state
            .subjects
            .get(subject.as_str())
            .and_then(|s| s.effects.get(key))
            .cloned())
    }

    /// Returns the last applied sequence of `subject`.
    ///
    /// # Errors
    /// Returns store failures.
    pub fn last_sequence(&self, subject: &SubjectId) -> Result<u64, WebhookError> {
        let state: ConsumerState = self.store.read()?;
        Ok(state
            .subjects
            .get(subject.as_str())
            .map_or(0, |ledger| ledger.last_sequence))
    }

    /// Returns how many effects `subject` recorded.
    ///
    /// # Errors
    /// Returns store failures.
    pub fn applied(&self, subject: &SubjectId) -> Result<usize, WebhookError> {
        let state: ConsumerState = self.store.read()?;
        Ok(state
            .subjects
            .get(subject.as_str())
            .map_or(0, |ledger| ledger.applied.len()))
    }
}

fn apply_once(
    ledger: &mut SubjectLedger,
    event: &ProtocolEvent,
    body_digest: [u8; 32],
    effect: impl FnOnce(&mut ConsumerTransaction<'_>) -> String,
) -> Result<Consumed, WebhookError> {
    if let Some(applied) = ledger.applied.get(event.id().as_str()) {
        if applied.body_digest != body_digest {
            return Err(WebhookError::EventConflict);
        }
        return Ok(Consumed::Duplicate {
            sequence: applied.sequence,
            result: applied.result.clone(),
        });
    }
    let sequence = event.subject_sequence();
    if sequence <= ledger.last_sequence {
        return Err(WebhookError::OrderViolation);
    }
    let expected = ledger.last_sequence.saturating_add(1);
    if sequence != expected {
        return Ok(Consumed::Suspended {
            expected,
            received: sequence,
        });
    }
    let result = effect(&mut ConsumerTransaction {
        event,
        effects: &mut ledger.effects,
    });
    if ledger.applied.len() >= MAXIMUM_APPLIED {
        let oldest = ledger
            .applied
            .iter()
            .min_by_key(|(_, applied)| applied.sequence)
            .map(|(id, _)| id.clone());
        if let Some(oldest) = oldest {
            ledger.applied.remove(&oldest);
        }
    }
    ledger.applied.insert(
        event.id().as_str().to_owned(),
        Applied {
            sequence,
            body_digest,
            result: result.clone(),
        },
    );
    ledger.last_sequence = sequence;
    Ok(Consumed::Applied { sequence, result })
}

/// Business state owned by the same durable transaction as the event ledger.
/// Only writes through this value share the receiver's atomic commit.
pub struct ConsumerTransaction<'a> {
    event: &'a ProtocolEvent,
    effects: &'a mut BTreeMap<String, String>,
}
impl std::ops::Deref for ConsumerTransaction<'_> {
    type Target = ProtocolEvent;
    fn deref(&self) -> &Self::Target {
        self.event
    }
}
impl ConsumerTransaction<'_> {
    pub fn get(&self, key: &str) -> Option<&str> {
        self.effects.get(key).map(String::as_str)
    }
    pub fn set(&mut self, key: &str, value: String) -> Result<(), WebhookError> {
        if key.is_empty()
            || key.len() > 128
            || value.len() > 4096
            || (!self.effects.contains_key(key) && self.effects.len() >= 4096)
        {
            return Err(WebhookError::InvalidRequest);
        }
        self.effects.insert(key.to_owned(), value);
        Ok(())
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CapturePolicy {
    pub chain_domain: [u8; 32],
    pub consensus: ChainPolicy,
}

/// Producer codec for a real captured row, exact state, membership and commit.
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CaptureDocument {
    pub binding: Vec<u8>,
    pub snapshot_id: [u8; 32],
    pub event_id: String,
    pub decoder_version: u16,
    pub action: String,
    pub ai_event_tag: u16,
    pub source_activity_id: [u8; 32],
    pub effect_ordinal: u16,
    pub receipt_digest: [u8; 32],
    pub state: Vec<u8>,
    pub store_proof: Vec<u8>,
    pub commit: CommitCertificate,
}
impl CaptureDocument {
    pub fn from_capture(
        row: &CapturedEvent,
        evidence: &Evidence<'_>,
    ) -> Result<Self, MarketEventError> {
        well_formed(row, evidence)?;
        let proof = evidence.proof.ok_or(MarketEventError::InvalidRequest)?;
        let mut binding = vec![0; BINDING_MAX_BYTES];
        let n = row
            .binding
            .encode(&mut binding)
            .map_err(|_| MarketEventError::InvalidRequest)?;
        binding.truncate(n);
        Ok(Self {
            binding,
            snapshot_id: row.snapshot_id,
            event_id: row.event_id.clone(),
            decoder_version: row.decoder_version,
            action: row.action.clone(),
            ai_event_tag: row.ai_event_tag,
            source_activity_id: row.source_activity_id,
            effect_ordinal: row.effect_ordinal,
            receipt_digest: row.receipt_digest,
            state: evidence.state.to_vec(),
            store_proof: proof.store_proof.to_vec(),
            commit: CommitCertificate::from_proof(&proof)?,
        })
    }
    fn row(&self) -> Result<CapturedEvent, MarketEventError> {
        if self.binding.len() > BINDING_MAX_BYTES {
            return Err(MarketEventError::InvalidRequest);
        }
        Ok(CapturedEvent {
            binding: SnapshotBinding::decode(&self.binding)
                .map_err(|_| MarketEventError::InvalidRequest)?,
            snapshot_id: self.snapshot_id,
            event_id: self.event_id.clone(),
            decoder_version: self.decoder_version,
            action: self.action.clone(),
            ai_event_tag: self.ai_event_tag,
            source_activity_id: self.source_activity_id,
            effect_ordinal: self.effect_ordinal,
            receipt_digest: self.receipt_digest,
            manifest: UpstreamManifest::default(),
        })
    }
}
fn well_formed_document(
    row: &CapturedEvent,
    state: &[u8],
    proof: &[u8],
) -> Result<(), MarketEventError> {
    let mut canonical = [0; BINDING_MAX_BYTES];
    if row.binding.encode(&mut canonical).is_err()
        || state.len() > MAXIMUM_STATE_BYTES
        || proof.is_empty()
        || proof.len() > MAXIMUM_EVIDENCE_BYTES
        || !nonzero(&row.source_activity_id)
        || !nonzero(&row.receipt_digest)
        || row.action.is_empty()
        || row.action.len() > MAXIMUM_ACTION_BYTES
        || !row
            .action
            .bytes()
            .all(|b| b.is_ascii_uppercase() || b.is_ascii_digit() || b == b'_')
    {
        return Err(MarketEventError::InvalidRequest);
    }
    EventId::new(&row.event_id)?;
    Ok(())
}

#[derive(Default, Serialize, Deserialize)]
struct TenantRegistry {
    principals: BTreeMap<String, Principal>,
}

/// Shared producer/control-plane bridge. Each tenant retains its own durable outbox.
pub struct MarketHub {
    root: PathBuf,
    policy: CapturePolicy,
    cursor_keys: CursorKeys,
}
impl MarketHub {
    pub fn new(
        root: &Path,
        policy: CapturePolicy,
        cursor_keys: CursorKeys,
    ) -> Result<Self, MarketEventError> {
        policy.consensus.validate()?;
        Store::open(root)?;
        Ok(Self {
            root: root.to_path_buf(),
            policy,
            cursor_keys,
        })
    }
    pub fn from_environment() -> Result<Option<Self>, String> {
        let Some(root) = std::env::var_os("LAYERX_WEBHOOKS_MARKET_OUTBOX_DIR") else {
            return Ok(None);
        };
        let policy_path = std::env::var("LAYERX_WEBHOOKS_MARKET_POLICY_FILE")
            .map_err(|_| "LAYERX_WEBHOOKS_MARKET_POLICY_FILE is required")?;
        let policy: CapturePolicy =
            serde_json::from_slice(&fs::read(policy_path).map_err(|e| e.to_string())?)
                .map_err(|e| e.to_string())?;
        let key_path = std::env::var("LAYERX_WEBHOOKS_CURSOR_KEY_FILE")
            .map_err(|_| "LAYERX_WEBHOOKS_CURSOR_KEY_FILE is required")?;
        let key = Zeroizing::new(fs::read_to_string(key_path).map_err(|e| e.to_string())?);
        let keys = CursorKeys::new(
            (
                1,
                crate::encoding::fixed_hex(key.trim()).map_err(|e| e.to_string())?,
            ),
            &[],
        )
        .map_err(|e| format!("market cursor keys: {e:?}"))?;
        Self::new(Path::new(&root), policy, keys)
            .map(Some)
            .map_err(|e| format!("market outbox: {e:?}"))
    }
    fn tenant(&self, principal: &Principal) -> Result<MarketOutbox, MarketEventError> {
        let id = hex_encode(&digest(principal.as_str().as_bytes()));
        MarketOutbox::open_portable(
            &self.root.join(id),
            principal.clone(),
            self.policy.clone(),
            self.cursor_keys.clone(),
        )
    }
    pub fn grant(&self, principal: &Principal, market: &[u8; 32]) -> Result<(), MarketEventError> {
        Store::open(&self.root)?.transact(|registry: &mut TenantRegistry| {
            let id = hex_encode(&digest(principal.as_str().as_bytes()));
            if !registry.principals.contains_key(&id) && registry.principals.len() >= 4096 {
                return Err(MarketEventError::CaptureStopped);
            }
            registry.principals.insert(id, principal.clone());
            Ok(())
        })?;
        self.tenant(principal)?.grant(market)
    }
    pub fn revoke(&self, principal: &Principal, market: &[u8; 32]) -> Result<(), MarketEventError> {
        self.tenant(principal)?.revoke(market)
    }
    pub fn capture(
        &self,
        principal: &Principal,
        producer: &ProducerGrant,
        document: &CaptureDocument,
    ) -> Result<Admission, MarketEventError> {
        self.tenant(principal)?.admit_document(producer, document)
    }
    pub fn resume(
        &self,
        principal: &Principal,
        endpoint: &EndpointId,
        market: &[u8; 32],
        cursor: Option<&str>,
        limit: usize,
    ) -> Result<ResumePage, MarketEventError> {
        self.tenant(principal)?
            .resume(principal, endpoint, market, cursor, limit)
    }
    pub fn drain(
        &self,
        service: &crate::HostedService,
        now: u64,
    ) -> Result<usize, MarketEventError> {
        let report = self.drain_report(service, now)?;
        if report.failure_count > 0 {
            return Err(WebhookError::Unavailable.into());
        }
        Ok(report.released)
    }

    /// Drains each tenant independently and retains failures in delivery readiness.
    pub fn drain_report(
        &self,
        service: &crate::HostedService,
        now: u64,
    ) -> Result<crate::hosted::HostedRetentionReport, MarketEventError> {
        use crate::hosted::{HostedRetentionReport, WorkerStage};
        let registry: TenantRegistry = match Store::open(&self.root).and_then(|store| store.read())
        {
            Ok(registry) => {
                service.clear_worker_failure("all", WorkerStage::MarketDrain);
                registry
            }
            Err(error) => {
                service.record_worker_failure("all", None, WorkerStage::MarketDrain, &error);
                return Err(error.into());
            }
        };
        let mut report = HostedRetentionReport::default();
        for (id, principal) in registry.principals {
            match self
                .tenant(&principal)
                .and_then(|tenant| tenant.drain(service, now))
            {
                Ok(count) => {
                    report.released += count;
                    service.clear_worker_failure(&id, WorkerStage::MarketDrain);
                }
                Err(error) => {
                    let error = match error {
                        MarketEventError::Webhook(e) => e,
                        _ => WebhookError::Unavailable,
                    };
                    service.record_worker_failure(
                        &id,
                        Some(principal.clone()),
                        WorkerStage::MarketDrain,
                        &error,
                    );
                    report.failure_count += 1;
                    crate::hosted::push_failure(
                        &mut report.failures,
                        id,
                        Some(principal),
                        WorkerStage::MarketDrain,
                        &error,
                    );
                }
            }
        }
        Ok(report)
    }
}
