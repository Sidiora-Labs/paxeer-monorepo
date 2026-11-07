//! Checkpointed relay journal. Every decision that must survive a restart is
//! one JSON line, written and fsynced before the action it records takes
//! effect outside the process: an item is observed before its scan cursor
//! moves, and a transaction is journaled with its exact signed bytes before it
//! is broadcast. On restart the relayer therefore rebroadcasts the same bytes
//! instead of signing a second transaction for the same bridge event.
//!
//! Items are keyed by `in:{chain}:{txHash}:{logIndex}` for Ethereum deposits
//! and `out:{chain}:{paxeerTxHash}:{nonce}` for Paxeer burns, Solana ones
//! included. A Solana release is journaled with its exact signed wire bytes
//! and the last block height its blockhash is valid for, and a burn whose
//! recipient handle has no recipient PDA yet is journaled as pending until the
//! PDA appears.

use std::collections::BTreeMap;
use std::fmt;
use std::fs::{self, File, OpenOptions};
use std::io::{BufRead as _, BufReader, Seek as _, SeekFrom, Write as _};
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};

use crate::abi::LogPosition;
use crate::attestation::{InboundAttestation, OutboundAttestation};
use crate::hex;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum JournalError {
    Io(String),
    Corrupt { line: usize },
    Conflict(String),
}

impl fmt::Display for JournalError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Io(error) => write!(formatter, "journal i/o: {error}"),
            Self::Corrupt { line } => write!(formatter, "journal line {line} is corrupt"),
            Self::Conflict(detail) => write!(formatter, "journal conflict: {detail}"),
        }
    }
}

impl std::error::Error for JournalError {}

fn io(error: &std::io::Error) -> JournalError {
    JournalError::Io(error.to_string())
}

/// The journal key of an inbound deposit: `(chain, txHash, logIndex)`.
#[must_use]
pub fn inbound_key(chain_id: u64, tx_hash: &[u8; 32], log_index: u64) -> String {
    format!("in:{chain_id}:{}:{log_index}", hex::prefixed(tx_hash))
}

/// The journal key of an outbound burn: `(chain, paxeerTxHash, nonce)`.
#[must_use]
pub fn outbound_key(chain_id: u64, paxeer_tx_hash: &[u8; 32], nonce: u64) -> String {
    format!("out:{chain_id}:{}:{nonce}", hex::prefixed(paxeer_tx_hash))
}

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Position {
    pub block_number: u64,
    #[serde(with = "hex::fixed_serde")]
    pub block_hash: [u8; 32],
}

impl From<LogPosition> for Position {
    fn from(value: LogPosition) -> Self {
        Self {
            block_number: value.block_number,
            block_hash: value.block_hash,
        }
    }
}

/// The attested content of an observed bridge event.
#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "direction", rename_all = "snake_case", deny_unknown_fields)]
pub enum Observation {
    Inbound {
        chain_id: u64,
        #[serde(with = "hex::fixed_serde")]
        vault: [u8; 20],
        #[serde(with = "hex::fixed_serde")]
        tx_hash: [u8; 32],
        log_index: u64,
        #[serde(with = "hex::fixed_serde")]
        recipient: [u8; 32],
        #[serde(with = "hex::fixed_serde")]
        asset: [u8; 20],
        #[serde(with = "hex::fixed_serde")]
        amount: [u8; 32],
        position: Position,
    },
    Outbound {
        chain_id: u64,
        #[serde(with = "hex::fixed_serde")]
        vault: [u8; 20],
        #[serde(with = "hex::fixed_serde")]
        paxeer_tx_hash: [u8; 32],
        paxeer_nonce: u64,
        #[serde(with = "hex::fixed_serde")]
        recipient: [u8; 20],
        #[serde(with = "hex::fixed_serde")]
        asset: [u8; 20],
        #[serde(with = "hex::fixed_serde")]
        amount: [u8; 32],
        position: Position,
    },
}

impl Observation {
    #[must_use]
    pub fn inbound(attestation: &InboundAttestation, position: Position) -> Self {
        Self::Inbound {
            chain_id: attestation.chain_id,
            vault: attestation.vault,
            tx_hash: attestation.tx_hash,
            log_index: attestation.log_index,
            recipient: attestation.recipient,
            asset: attestation.asset,
            amount: attestation.amount,
            position,
        }
    }

    #[must_use]
    pub fn outbound(attestation: &OutboundAttestation, position: Position) -> Self {
        Self::Outbound {
            chain_id: attestation.chain_id,
            vault: attestation.vault,
            paxeer_tx_hash: attestation.paxeer_tx_hash,
            paxeer_nonce: attestation.paxeer_nonce,
            recipient: attestation.recipient,
            asset: attestation.asset,
            amount: attestation.amount,
            position,
        }
    }

    /// The journal key this observation is stored under.
    #[must_use]
    pub fn key(&self) -> String {
        match self {
            Self::Inbound {
                chain_id,
                tx_hash,
                log_index,
                ..
            } => inbound_key(*chain_id, tx_hash, *log_index),
            Self::Outbound {
                chain_id,
                paxeer_tx_hash,
                paxeer_nonce,
                ..
            } => outbound_key(*chain_id, paxeer_tx_hash, *paxeer_nonce),
        }
    }

    #[must_use]
    pub const fn inbound_attestation(&self) -> Option<InboundAttestation> {
        match *self {
            Self::Inbound {
                chain_id,
                vault,
                tx_hash,
                log_index,
                recipient,
                asset,
                amount,
                ..
            } => Some(InboundAttestation {
                chain_id,
                vault,
                tx_hash,
                log_index,
                recipient,
                asset,
                amount,
            }),
            Self::Outbound { .. } => None,
        }
    }

    #[must_use]
    pub const fn outbound_attestation(&self) -> Option<OutboundAttestation> {
        match *self {
            Self::Outbound {
                chain_id,
                vault,
                paxeer_tx_hash,
                paxeer_nonce,
                recipient,
                asset,
                amount,
                ..
            } => Some(OutboundAttestation {
                chain_id,
                vault,
                paxeer_tx_hash,
                paxeer_nonce,
                recipient,
                asset,
                amount,
            }),
            Self::Inbound { .. } => None,
        }
    }
}

/// How an item left the relay.
#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "outcome", rename_all = "snake_case", deny_unknown_fields)]
pub enum Completion {
    /// This relayer's transaction executed successfully and is final.
    Included {
        #[serde(with = "hex::fixed_serde")]
        tx_hash: [u8; 32],
        block_number: u64,
    },
    /// The destination already consumed the event's nullifier, through this
    /// or another relayer's transaction.
    AlreadyBridged,
    /// This relayer's Solana release reached the configured commitment
    /// without error.
    Released {
        #[serde(with = "hex::fixed_serde")]
        signature: [u8; 64],
        slot: u64,
    },
}

/// One journal line.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "kind", rename_all = "snake_case", deny_unknown_fields)]
pub enum Entry {
    /// Every block below `next_block` of `stream` has been scanned.
    Cursor { stream: String, next_block: u64 },
    Observed {
        item: String,
        observation: Observation,
    },
    /// This relayer's attestor signature, `r || s || v`.
    Signed {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        signature: [u8; 65],
    },
    /// A signed transaction, journaled before its first broadcast.
    Submitted {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        submitter: [u8; 20],
        nonce: u64,
        #[serde(with = "hex::fixed_serde")]
        tx_hash: [u8; 32],
        #[serde(with = "hex::bytes_serde")]
        raw: Vec<u8>,
    },
    Receipted {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        tx_hash: [u8; 32],
        sequence: u64,
        height: u64,
    },
    /// The transaction executed and reverted without consuming the nullifier.
    Reverted {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        tx_hash: [u8; 32],
    },
    /// The transaction can no longer execute (refused, or its nonce was
    /// consumed by another transaction) and the nullifier is still open.
    Dropped {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        tx_hash: [u8; 32],
    },
    Completed {
        item: String,
        completion: Completion,
    },
    /// The event can never be bridged as observed; it is not signed.
    Refused { item: String, reason: String },
    /// The Solana recipient handle has no recipient PDA yet: nothing is
    /// signed or broadcast until it appears.
    RecipientPending { item: String },
    /// The 32-byte key the recipient PDA holds for the burn's handle.
    RecipientResolved {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        key: [u8; 32],
    },
    /// A signed Solana release, journaled before its first broadcast.
    ReleaseSubmitted {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        fee_payer: [u8; 32],
        #[serde(with = "hex::fixed_serde")]
        signature: [u8; 64],
        last_valid_block_height: u64,
        #[serde(with = "hex::bytes_serde")]
        raw: Vec<u8>,
    },
    /// The release's blockhash expired without inclusion; the same journaled
    /// attestation is resubmitted in a new transaction.
    ReleaseExpired {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        signature: [u8; 64],
    },
    /// The release executed and failed without consuming the nullifier.
    ReleaseFailed {
        item: String,
        #[serde(with = "hex::fixed_serde")]
        signature: [u8; 64],
    },
}

#[derive(Clone, Copy, Debug, Eq, PartialEq, Deserialize, Serialize)]
pub enum SubmissionStatus {
    Pending,
    Receipted {
        sequence: u64,
        height: u64,
    },
    Committed {
        sequence: u64,
        height: u64,
    },
    Reverted,
    Dropped,
    /// The transaction a completion names: it is final and awaits nothing.
    Landed,
}

#[derive(Clone, Debug, Eq, PartialEq, Deserialize, Serialize)]
pub struct Submission {
    pub submitter: [u8; 20],
    pub nonce: u64,
    pub tx_hash: [u8; 32],
    #[serde(with = "hex::bytes_serde")]
    pub raw: Vec<u8>,
    pub status: SubmissionStatus,
}

/// A signed Solana release transaction.
#[derive(Clone, Debug, Eq, PartialEq, Deserialize, Serialize)]
pub struct ReleaseSubmission {
    pub fee_payer: [u8; 32],
    #[serde(with = "hex::fixed_serde")]
    pub signature: [u8; 64],
    pub last_valid_block_height: u64,
    #[serde(with = "hex::bytes_serde")]
    pub raw: Vec<u8>,
    pub status: SubmissionStatus,
}

/// Where the lookup of a Solana recipient handle stands.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Deserialize, Serialize)]
pub enum Recipient {
    /// The recipient PDA does not exist yet.
    Pending,
    /// The 32-byte key the handle stands for.
    Resolved([u8; 32]),
}

#[derive(Clone, Debug, Eq, PartialEq, Deserialize, Serialize)]
pub struct Item {
    pub observation: Observation,
    #[serde(with = "optional_signature_serde")]
    pub signature: Option<[u8; 65]>,
    pub submissions: Vec<Submission>,
    pub completion: Option<Completion>,
    pub refusal: Option<String>,
    pub recipient: Option<Recipient>,
    pub releases: Vec<ReleaseSubmission>,
    /// Volatile serving policy, populated only on a relayer snapshot. Journal
    /// replay never authenticates a destination policy and leaves this absent.
    #[serde(skip)]
    pub attestation_policy: Option<crate::attestation_api::AuthenticatedPolicy>,
}

impl Item {
    const fn observed(observation: Observation) -> Self {
        Self {
            observation,
            signature: None,
            submissions: Vec::new(),
            completion: None,
            refusal: None,
            recipient: None,
            releases: Vec::new(),
            attestation_policy: None,
        }
    }

    /// Whether the relayer still has work to do for this item.
    #[must_use]
    pub const fn is_open(&self) -> bool {
        self.completion.is_none() && self.refusal.is_none()
    }

    /// The transaction still awaiting an outcome, if any.
    #[must_use]
    pub fn pending(&self) -> Option<&Submission> {
        self.submissions.iter().find(|submission| {
            matches!(
                submission.status,
                SubmissionStatus::Pending | SubmissionStatus::Receipted { .. }
            )
        })
    }

    /// The Solana release still awaiting an outcome, if any.
    #[must_use]
    pub fn pending_release(&self) -> Option<&ReleaseSubmission> {
        self.releases
            .iter()
            .find(|release| release.status == SubmissionStatus::Pending)
    }

    /// Every transaction this item has consumed, on any destination.
    #[must_use]
    pub fn transactions(&self) -> usize {
        self.submissions.len() + self.releases.len()
    }

    /// Marks the transaction `completion` names as landed. A completion that
    /// names a transaction this item never submitted is refused; one that
    /// names none marks nothing.
    fn land(&mut self, item: &str, completion: &Completion) -> Result<(), JournalError> {
        let status = match *completion {
            Completion::AlreadyBridged => return Ok(()),
            Completion::Included {
                tx_hash,
                block_number,
            } => {
                if self.pending().is_some_and(|submission| {
                    matches!(submission.status, SubmissionStatus::Receipted { .. })
                        && submission.tx_hash != tx_hash
                }) {
                    return Err(JournalError::Conflict(format!(
                        "{item} inclusion does not match its receipt"
                    )));
                }
                let submission = self
                    .submissions
                    .iter_mut()
                    .rev()
                    .find(|submission| submission.tx_hash == tx_hash)
                    .ok_or_else(|| {
                        JournalError::Conflict(format!(
                            "{item} never submitted {}",
                            hex::prefixed(&tx_hash)
                        ))
                    })?;
                if let SubmissionStatus::Receipted { sequence, height } = submission.status {
                    if block_number < height {
                        return Err(JournalError::Conflict(format!(
                            "{item} inclusion precedes its receipt height"
                        )));
                    }
                    submission.status = SubmissionStatus::Committed { sequence, height };
                    return Ok(());
                }
                &mut submission.status
            }
            Completion::Released { signature, .. } => self
                .releases
                .iter_mut()
                .rev()
                .find(|release| release.signature == signature)
                .map(|release| &mut release.status)
                .ok_or_else(|| {
                    JournalError::Conflict(format!(
                        "{item} never submitted release {}",
                        hex::prefixed(&signature)
                    ))
                })?,
        };
        *status = SubmissionStatus::Landed;
        Ok(())
    }
}

/// The state every journal line replays into.
#[derive(Clone, Debug, Default, Eq, PartialEq, Deserialize, Serialize)]
pub struct State {
    pub cursors: BTreeMap<String, u64>,
    pub items: BTreeMap<String, Item>,
}

impl State {
    fn item_mut(&mut self, item: &str) -> Result<&mut Item, JournalError> {
        self.items
            .get_mut(item)
            .ok_or_else(|| JournalError::Conflict(format!("{item} was never observed")))
    }

    fn apply(&mut self, entry: &Entry) -> Result<(), JournalError> {
        match entry {
            Entry::Cursor { stream, next_block } => {
                let current = self.cursors.entry(stream.clone()).or_insert(*next_block);
                if *next_block < *current {
                    return Err(JournalError::Conflict(format!(
                        "cursor {stream} moved back from {current} to {next_block}"
                    )));
                }
                *current = *next_block;
            }
            Entry::Observed { item, observation } => {
                if observation.key() != *item {
                    return Err(JournalError::Conflict(format!(
                        "{item} does not match its observation"
                    )));
                }
                if let Some(existing) = self.items.get(item) {
                    if existing.observation != *observation {
                        return Err(JournalError::Conflict(format!(
                            "{item} observed with different content"
                        )));
                    }
                } else {
                    self.items
                        .insert(item.clone(), Item::observed(*observation));
                }
            }
            Entry::Signed { item, signature } => {
                let record = self.item_mut(item)?;
                if record
                    .signature
                    .is_some_and(|existing| existing != *signature)
                {
                    return Err(JournalError::Conflict(format!("{item} signed twice")));
                }
                record.signature = Some(*signature);
            }
            Entry::Submitted {
                item,
                submitter,
                nonce,
                tx_hash,
                raw,
            } => {
                self.submit(item, *submitter, *nonce, *tx_hash, raw)?;
            }
            Entry::Receipted {
                item,
                tx_hash,
                sequence,
                height,
            } => {
                self.receipt(item, tx_hash, *sequence, *height)?;
            }
            Entry::Reverted { item, tx_hash } | Entry::Dropped { item, tx_hash } => {
                let status = if matches!(entry, Entry::Reverted { .. }) {
                    SubmissionStatus::Reverted
                } else {
                    SubmissionStatus::Dropped
                };
                let record = self.item_mut(item)?;
                let submission = record
                    .submissions
                    .iter_mut()
                    .find(|submission| {
                        submission.tx_hash == *tx_hash
                            && submission.status == SubmissionStatus::Pending
                    })
                    .ok_or_else(|| {
                        JournalError::Conflict(format!("{item} has no pending {tx_hash:02x?}"))
                    })?;
                submission.status = status;
            }
            Entry::Completed { item, completion } => {
                let record = self.item_mut(item)?;
                if record.completion.is_some() {
                    return Err(JournalError::Conflict(format!("{item} completed twice")));
                }
                record.land(item, completion)?;
                record.completion = Some(*completion);
            }
            Entry::Refused { item, reason } => {
                self.item_mut(item)?.refusal = Some(reason.clone());
            }
            Entry::RecipientPending { .. }
            | Entry::RecipientResolved { .. }
            | Entry::ReleaseSubmitted { .. }
            | Entry::ReleaseExpired { .. }
            | Entry::ReleaseFailed { .. } => self.apply_release(entry)?,
        }
        Ok(())
    }

    fn submit(
        &mut self,
        item: &str,
        submitter: [u8; 20],
        nonce: u64,
        tx_hash: [u8; 32],
        raw: &[u8],
    ) -> Result<(), JournalError> {
        let record = self.item_mut(item)?;
        if !record.is_open() {
            return Err(JournalError::Conflict(format!("{item} is closed")));
        }
        if record.pending().is_some() {
            return Err(JournalError::Conflict(format!(
                "{item} already has a pending transaction"
            )));
        }
        record.submissions.push(Submission {
            submitter,
            nonce,
            tx_hash,
            raw: raw.to_vec(),
            status: SubmissionStatus::Pending,
        });
        Ok(())
    }

    fn receipt(
        &mut self,
        item: &str,
        tx_hash: &[u8; 32],
        sequence: u64,
        height: u64,
    ) -> Result<(), JournalError> {
        let record = self.item_mut(item)?;
        if !record.is_open() || !matches!(record.observation, Observation::Inbound { .. }) {
            return Err(JournalError::Conflict(format!(
                "{item} is not an open inbound deposit"
            )));
        }
        if sequence == 0 || height == 0 {
            return Err(JournalError::Conflict(format!(
                "{item} receipt has a zero sequence or height"
            )));
        }
        let submission = record
            .submissions
            .iter_mut()
            .find(|submission| {
                submission.tx_hash == *tx_hash
                    && matches!(
                        submission.status,
                        SubmissionStatus::Pending | SubmissionStatus::Receipted { .. }
                    )
            })
            .ok_or_else(|| {
                JournalError::Conflict(format!("{item} has no pending {tx_hash:02x?}"))
            })?;
        let status = SubmissionStatus::Receipted { sequence, height };
        if submission.status != SubmissionStatus::Pending && submission.status != status {
            return Err(JournalError::Conflict(format!(
                "{item} receipt changed its sequence or height"
            )));
        }
        submission.status = status;
        Ok(())
    }

    /// Applies the entries only a Solana release writes.
    fn apply_release(&mut self, entry: &Entry) -> Result<(), JournalError> {
        match entry {
            Entry::RecipientPending { item } => {
                let record = self.outbound_mut(item)?;
                if record.recipient.is_some() {
                    return Err(JournalError::Conflict(format!(
                        "{item} recipient is already recorded"
                    )));
                }
                record.recipient = Some(Recipient::Pending);
            }
            Entry::RecipientResolved { item, key } => {
                let record = self.outbound_mut(item)?;
                if let Some(Recipient::Resolved(existing)) = record.recipient {
                    if existing != *key {
                        return Err(JournalError::Conflict(format!(
                            "{item} recipient resolved to two keys"
                        )));
                    }
                }
                record.recipient = Some(Recipient::Resolved(*key));
            }
            Entry::ReleaseSubmitted {
                item,
                fee_payer,
                signature,
                last_valid_block_height,
                raw,
            } => {
                let record = self.outbound_mut(item)?;
                if record.pending().is_some() || record.pending_release().is_some() {
                    return Err(JournalError::Conflict(format!(
                        "{item} already has a pending transaction"
                    )));
                }
                if !matches!(record.recipient, Some(Recipient::Resolved(_))) {
                    return Err(JournalError::Conflict(format!(
                        "{item} is released before its recipient is resolved"
                    )));
                }
                record.releases.push(ReleaseSubmission {
                    fee_payer: *fee_payer,
                    signature: *signature,
                    last_valid_block_height: *last_valid_block_height,
                    raw: raw.clone(),
                    status: SubmissionStatus::Pending,
                });
            }
            Entry::ReleaseExpired { item, signature }
            | Entry::ReleaseFailed { item, signature } => {
                let status = if matches!(entry, Entry::ReleaseFailed { .. }) {
                    SubmissionStatus::Reverted
                } else {
                    SubmissionStatus::Dropped
                };
                let record = self.item_mut(item)?;
                let release = record
                    .releases
                    .iter_mut()
                    .find(|release| {
                        release.signature == *signature
                            && release.status == SubmissionStatus::Pending
                    })
                    .ok_or_else(|| {
                        JournalError::Conflict(format!("{item} has no pending release"))
                    })?;
                release.status = status;
            }
            Entry::Cursor { .. }
            | Entry::Observed { .. }
            | Entry::Signed { .. }
            | Entry::Submitted { .. }
            | Entry::Receipted { .. }
            | Entry::Reverted { .. }
            | Entry::Dropped { .. }
            | Entry::Completed { .. }
            | Entry::Refused { .. } => {}
        }
        Ok(())
    }

    fn outbound_mut(&mut self, item: &str) -> Result<&mut Item, JournalError> {
        let record = self.item_mut(item)?;
        if !matches!(record.observation, Observation::Outbound { .. }) {
            return Err(JournalError::Conflict(format!(
                "{item} is not a burn to release"
            )));
        }
        Ok(record)
    }
}

mod optional_signature_serde {
    use serde::{Deserialize as _, Serialize as _};

    pub fn serialize<S: serde::Serializer>(
        value: &Option<[u8; 65]>,
        serializer: S,
    ) -> Result<S::Ok, S::Error> {
        value
            .as_ref()
            .map(|bytes| crate::hex::prefixed(bytes))
            .serialize(serializer)
    }

    pub fn deserialize<'de, D: serde::Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Option<[u8; 65]>, D::Error> {
        Option::<String>::deserialize(deserializer)?
            .map(|text| crate::hex::fixed(&text).map_err(serde::de::Error::custom))
            .transpose()
    }
}

/// Completed records are addressed directly by a digest of their journal key.
/// The contents retain the key's observation, signatures, nullifier outcome and
/// transaction history. They are immutable once removed from the active log.
#[derive(Clone, Debug)]
pub struct History {
    root: PathBuf,
}

impl History {
    fn item_path(&self, key: &str) -> PathBuf {
        self.root
            .join(hex::prefixed(&Sha256::digest(key.as_bytes())))
    }

    pub fn lookup(&self, key: &str) -> Result<Option<Item>, JournalError> {
        let bytes = match fs::read(self.item_path(key)) {
            Ok(bytes) => bytes,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
            Err(error) => return Err(io(&error)),
        };
        let item: Item =
            serde_json::from_slice(&bytes).map_err(|_| JournalError::Corrupt { line: 0 })?;
        if item.observation.key() != key || item.is_open() {
            return Err(JournalError::Corrupt { line: 0 });
        }
        Ok(Some(item))
    }

    fn archive(&self, key: &str, item: &Item) -> Result<(), JournalError> {
        if let Some(existing) = self.lookup(key)? {
            if existing != *item {
                return Err(JournalError::Conflict(format!("{key} history changed")));
            }
            return Ok(());
        }
        let path = self.item_path(key);
        let temporary = path.with_extension("tmp");
        let bytes =
            serde_json::to_vec(item).map_err(|error| JournalError::Io(error.to_string()))?;
        replace_synced(&temporary, &path, &bytes)
    }
}

fn sync_directory(path: &Path) -> Result<(), JournalError> {
    let parent = path
        .parent()
        .filter(|parent| !parent.as_os_str().is_empty())
        .unwrap_or_else(|| Path::new("."));
    File::open(parent)
        .and_then(|directory| directory.sync_all())
        .map_err(|error| io(&error))
}

fn replace_synced(temporary: &Path, path: &Path, bytes: &[u8]) -> Result<(), JournalError> {
    let mut file = OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .open(temporary)
        .map_err(|error| io(&error))?;
    file.write_all(bytes)
        .and_then(|()| file.sync_all())
        .map_err(|error| io(&error))?;
    fs::rename(temporary, path).map_err(|error| io(&error))?;
    sync_directory(path)
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct Checkpoint {
    checkpoint_version: u8,
    state: State,
}

/// The maximum number of new records between automatic checkpoints. Closed
/// records in memory and replay work are bounded by this interval, not uptime.
pub const CHECKPOINT_INTERVAL: usize = 256;

/// The active log contains a pending-state checkpoint followed by new entries.
/// Archival files reach durable storage before atomic checkpoint replacement;
/// until that replacement the old log remains the authority after a crash.
pub struct Journal {
    path: PathBuf,
    file: File,
    state: State,
    history: History,
    entries: usize,
    failed: bool,
}

impl Journal {
    pub fn open(path: &Path) -> Result<Self, JournalError> {
        let history = History {
            root: path.with_extension("history"),
        };
        let history_existed = history.root.is_dir();
        let mut file = OpenOptions::new()
            .read(true)
            .append(true)
            .create(true)
            .open(path)
            .map_err(|error| io(&error))?;
        let mut state = State::default();
        let mut reader = BufReader::new(&file);
        let mut valid_length = 0_u64;
        let mut line_number = 0_usize;
        let mut entries = 0;
        loop {
            let mut line = String::new();
            let read = reader.read_line(&mut line).map_err(|error| io(&error))?;
            if read == 0 || !line.ends_with('\n') {
                break;
            }
            line_number += 1;
            match serde_json::from_str::<Entry>(line.trim_end()) {
                Ok(entry) => {
                    state.apply(&entry)?;
                    entries += 1;
                }
                Err(_) if line_number == 1 => {
                    let checkpoint: Checkpoint = serde_json::from_str(line.trim_end())
                        .map_err(|_| JournalError::Corrupt { line: line_number })?;
                    if !history_existed
                        || checkpoint.checkpoint_version != 1
                        || checkpoint
                            .state
                            .items
                            .iter()
                            .any(|(key, item)| item.observation.key() != *key || !item.is_open())
                    {
                        return Err(JournalError::Corrupt { line: line_number });
                    }
                    state = checkpoint.state;
                }
                Err(_) => return Err(JournalError::Corrupt { line: line_number }),
            }
            valid_length +=
                u64::try_from(read).map_err(|_| JournalError::Corrupt { line: line_number })?;
        }
        drop(reader);
        if file.metadata().map_err(|error| io(&error))?.len() != valid_length {
            file.set_len(valid_length)
                .and_then(|()| file.sync_all())
                .map_err(|error| io(&error))?;
        }
        file.seek(SeekFrom::End(0)).map_err(|error| io(&error))?;
        sync_directory(path)?;
        fs::create_dir_all(&history.root).map_err(|error| io(&error))?;
        sync_directory(&history.root)?;
        let mut journal = Self {
            path: path.to_path_buf(),
            file,
            state,
            history,
            entries,
            failed: false,
        };
        if entries >= CHECKPOINT_INTERVAL {
            journal.checkpoint()?;
        }
        Ok(journal)
    }

    #[must_use]
    pub fn path(&self) -> &Path {
        &self.path
    }

    #[must_use]
    pub const fn state(&self) -> &State {
        &self.state
    }

    #[must_use]
    pub fn history(&self) -> History {
        self.history.clone()
    }

    pub fn lookup(&self, key: &str) -> Result<Option<Item>, JournalError> {
        match self.state.items.get(key) {
            Some(item) => Ok(Some(item.clone())),
            None => self.history.lookup(key),
        }
    }

    /// Archives closed records, then atomically replaces the log with exactly
    /// the remaining state. Uncommitted .tmp files are never used for recovery.
    pub fn checkpoint(&mut self) -> Result<(), JournalError> {
        if self.failed {
            return Err(JournalError::Io("journal requires reopen".to_owned()));
        }
        let result = self.write_checkpoint();
        if result.is_err() {
            self.failed = true;
        }
        result
    }

    fn write_checkpoint(&mut self) -> Result<(), JournalError> {
        for (key, item) in &self.state.items {
            if !item.is_open() {
                self.history.archive(key, item)?;
            }
        }
        let state = State {
            cursors: self.state.cursors.clone(),
            items: self
                .state
                .items
                .iter()
                .filter(|(_, item)| item.is_open())
                .map(|(key, item)| (key.clone(), item.clone()))
                .collect(),
        };
        let mut bytes = serde_json::to_vec(&Checkpoint {
            checkpoint_version: 1,
            state: state.clone(),
        })
        .map_err(|error| JournalError::Io(error.to_string()))?;
        bytes.push(b'\n');
        replace_synced(
            &self.path.with_extension("checkpoint.tmp"),
            &self.path,
            &bytes,
        )?;
        self.file = OpenOptions::new()
            .read(true)
            .append(true)
            .open(&self.path)
            .map_err(|error| io(&error))?;
        self.state = state;
        self.entries = 0;
        Ok(())
    }

    /// Validate a single affected record, fsync its entry, then publish its new
    /// state. No append clones unrelated pending records or historical items.
    pub fn append(&mut self, entry: &Entry) -> Result<(), JournalError> {
        if self.failed {
            return Err(JournalError::Io("journal requires reopen".to_owned()));
        }
        let mut next = State::default();
        match entry {
            Entry::Cursor { stream, .. } => {
                if let Some(cursor) = self.state.cursors.get(stream) {
                    next.cursors.insert(stream.clone(), *cursor);
                }
            }
            _ => {
                let key = entry
                    .item_key()
                    .ok_or_else(|| JournalError::Conflict("missing item".to_owned()))?;
                if let Some(item) = self.lookup(key)? {
                    if !self.state.items.contains_key(key) {
                        if matches!(entry, Entry::Observed { observation, .. } if *observation == item.observation)
                        {
                            return Ok(());
                        }
                        return Err(JournalError::Conflict(format!("{key} is archived")));
                    }
                    next.items.insert(key.to_owned(), item);
                }
            }
        }
        next.apply(entry)?;
        let mut line =
            serde_json::to_vec(entry).map_err(|error| JournalError::Io(error.to_string()))?;
        line.push(b'\n');
        if let Err(error) = self
            .file
            .write_all(&line)
            .and_then(|()| self.file.sync_data())
        {
            self.failed = true;
            return Err(io(&error));
        }
        self.state.cursors.extend(next.cursors);
        self.state.items.extend(next.items);
        self.entries += 1;
        if self.entries >= CHECKPOINT_INTERVAL {
            self.checkpoint()?;
        }
        Ok(())
    }
}

impl Entry {
    fn item_key(&self) -> Option<&str> {
        match self {
            Self::Cursor { .. } => None,
            Self::Observed { item, .. }
            | Self::Signed { item, .. }
            | Self::Submitted { item, .. }
            | Self::Receipted { item, .. }
            | Self::Reverted { item, .. }
            | Self::Dropped { item, .. }
            | Self::Completed { item, .. }
            | Self::Refused { item, .. }
            | Self::RecipientPending { item }
            | Self::RecipientResolved { item, .. }
            | Self::ReleaseSubmitted { item, .. }
            | Self::ReleaseExpired { item, .. }
            | Self::ReleaseFailed { item, .. } => Some(item),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::*;

    fn directory(name: &str) -> PathBuf {
        let path = std::env::temp_dir().join(format!(
            "layerx-bridge-relayer-journal-{}-{name}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&path);
        fs::create_dir_all(&path).unwrap_or_else(|error| panic!("temp dir: {error}"));
        path
    }

    fn observation() -> Observation {
        Observation::inbound(
            &InboundAttestation {
                chain_id: 1,
                vault: [0x11; 20],
                tx_hash: [0xaa; 32],
                log_index: 3,
                recipient: [0x55; 32],
                asset: [0x44; 20],
                amount: [0x01; 32],
            },
            Position {
                block_number: 100,
                block_hash: [0xbb; 32],
            },
        )
    }

    fn open(path: &Path) -> Journal {
        Journal::open(path).unwrap_or_else(|error| panic!("journal: {error}"))
    }

    fn append(journal: &mut Journal, entry: &Entry) {
        journal
            .append(entry)
            .unwrap_or_else(|error| panic!("append: {error}"));
    }

    fn refused(journal: &mut Journal, entry: &Entry) {
        assert!(matches!(
            journal.append(entry),
            Err(JournalError::Conflict(_))
        ));
    }

    #[test]
    fn a_reopened_journal_replays_to_the_same_state() {
        let path = directory("replay").join("journal.jsonl");
        let key = observation().key();
        assert_eq!(key, format!("in:1:0x{}:3", "aa".repeat(32)));
        let before = {
            let mut journal = open(&path);
            append(
                &mut journal,
                &Entry::Observed {
                    item: key.clone(),
                    observation: observation(),
                },
            );
            append(
                &mut journal,
                &Entry::Cursor {
                    stream: "in:1".to_owned(),
                    next_block: 109,
                },
            );
            append(
                &mut journal,
                &Entry::Signed {
                    item: key.clone(),
                    signature: [0x1b; 65],
                },
            );
            append(
                &mut journal,
                &Entry::Submitted {
                    item: key.clone(),
                    submitter: [0xb1; 20],
                    nonce: 5,
                    tx_hash: [0xcc; 32],
                    raw: vec![2, 3, 4],
                },
            );
            journal.state().clone()
        };
        let reopened = open(&path);
        assert_eq!(reopened.state(), &before);
        let item = &reopened.state().items[&key];
        assert_eq!(item.pending().map(|submission| submission.nonce), Some(5));
        assert_eq!(
            item.pending().map(|submission| submission.raw.clone()),
            Some(vec![2, 3, 4])
        );
        assert_eq!(reopened.state().cursors["in:1"], 109);
    }

    #[test]
    fn a_second_pending_transaction_and_a_backward_cursor_are_refused() {
        let path = directory("conflicts").join("journal.jsonl");
        let key = observation().key();
        let mut journal = open(&path);
        append(
            &mut journal,
            &Entry::Observed {
                item: key.clone(),
                observation: observation(),
            },
        );
        let submitted = Entry::Submitted {
            item: key.clone(),
            submitter: [0xb1; 20],
            nonce: 5,
            tx_hash: [0xcc; 32],
            raw: vec![1],
        };
        append(&mut journal, &submitted);
        assert!(matches!(
            journal.append(&submitted),
            Err(JournalError::Conflict(_))
        ));
        append(
            &mut journal,
            &Entry::Cursor {
                stream: "in:1".to_owned(),
                next_block: 10,
            },
        );
        assert!(matches!(
            journal.append(&Entry::Cursor {
                stream: "in:1".to_owned(),
                next_block: 9,
            }),
            Err(JournalError::Conflict(_))
        ));
        let mut different = observation();
        if let Observation::Inbound { amount, .. } = &mut different {
            *amount = [0x02; 32];
        }
        assert!(matches!(
            journal.append(&Entry::Observed {
                item: key,
                observation: different,
            }),
            Err(JournalError::Conflict(_))
        ));
        let lines = fs::read_to_string(&path)
            .unwrap_or_default()
            .lines()
            .count();
        assert_eq!(lines, 3, "refused entries are never written");
    }

    #[test]
    fn only_a_pending_submission_can_revert_and_replay_without_changing_its_identity() {
        use k256::ecdsa::SigningKey;
        use layerx_crypto::evm_transaction::Eip1559Call;
        use sha3::{Digest as _, Keccak256};

        let mut recipient = [0; 32];
        recipient[12..].fill(0x77);
        let deposit = InboundAttestation {
            chain_id: 1,
            vault: [0x11; 20],
            tx_hash: [0xaa; 32],
            log_index: 3,
            recipient,
            asset: [0x44; 20],
            amount: crate::attestation::uint256_from_u64(1_000_000_000_000_000_000),
        };
        let mut secret = [0; 32];
        secret[31] = 0xa1;
        let (signature, recovery) = SigningKey::from_slice(&secret)
            .unwrap()
            .sign_prehash_recoverable(&deposit.digest())
            .unwrap();
        let mut recoverable = [0; 65];
        recoverable[..64].copy_from_slice(&signature.to_bytes());
        recoverable[64] = recovery.to_byte();
        let signature = crate::attestation::to_attestor_signature(recoverable).unwrap();
        secret[31] = 0xb1;
        let raw = Eip1559Call {
            chain_id: 229,
            nonce: 5,
            max_priority_fee_per_gas: 1_000_000_000,
            max_fee_per_gas: 5_000_000_000,
            gas_limit: 1_000_000,
            to: crate::abi::LAYERX_BRIDGE_PRECOMPILE,
            data: crate::abi::encode_bridge_in(&deposit, &[signature]),
        }
        .sign(&secret)
        .unwrap();
        let tx_hash: [u8; 32] = Keccak256::digest(&raw).into();
        let fixture: serde_json::Value = serde_json::from_str(include_str!(
            "../tests/fixtures/inbound_sequencer_receipt.json"
        ))
        .unwrap();
        assert_eq!(hex::prefixed(&tx_hash), fixture["transactionHash"]);
        let observation = Observation::inbound(
            &deposit,
            Position {
                block_number: 100,
                block_hash: [0xbb; 32],
            },
        );
        let key = observation.key();
        let path = directory("receipted-reverted").join("journal.jsonl");
        let mut journal = open(&path);
        append(
            &mut journal,
            &Entry::Observed {
                item: key.clone(),
                observation,
            },
        );
        let submitted = Entry::Submitted {
            item: key.clone(),
            submitter: crate::attestation::ethereum_address(
                SigningKey::from_slice(&secret).unwrap().verifying_key(),
            ),
            nonce: 5,
            tx_hash,
            raw: raw.clone(),
        };
        append(&mut journal, &submitted);
        let pending_path = directory("pending-reverted").join("journal.jsonl");
        let mut pending_journal = open(&pending_path);
        append(
            &mut pending_journal,
            &Entry::Observed {
                item: key.clone(),
                observation,
            },
        );
        append(&mut pending_journal, &submitted);
        let pending_bytes = fs::read(&pending_path).unwrap();
        refused(
            &mut pending_journal,
            &Entry::Reverted {
                item: key.clone(),
                tx_hash: [0xee; 32],
            },
        );
        assert_eq!(fs::read(&pending_path).unwrap(), pending_bytes);
        append(
            &mut pending_journal,
            &Entry::Reverted {
                item: key.clone(),
                tx_hash,
            },
        );
        let item = &pending_journal.state().items[&key];
        assert_eq!(item.submissions.len(), 1);
        assert_eq!(item.submissions[0].tx_hash, tx_hash);
        assert_eq!(item.submissions[0].raw, raw);
        assert_eq!(item.submissions[0].status, SubmissionStatus::Reverted);
        assert!(item.pending().is_none());
        assert!(item.is_open());
        assert_eq!(open(&pending_path).state(), pending_journal.state());
        append(
            &mut journal,
            &Entry::Receipted {
                item: key.clone(),
                tx_hash,
                sequence: 5,
                height: 1002,
            },
        );
        assert_eq!(journal.state().items[&key].submissions.len(), 1);
        let pending = journal.state().items[&key].pending().unwrap();
        assert_eq!(pending.tx_hash, tx_hash);
        assert_eq!(pending.raw, raw);
        assert_eq!(
            pending.status,
            SubmissionStatus::Receipted {
                sequence: 5,
                height: 1002
            }
        );
        let before_refusal = fs::read(&path).unwrap();
        refused(
            &mut journal,
            &Entry::Dropped {
                item: key.clone(),
                tx_hash,
            },
        );
        refused(
            &mut journal,
            &Entry::Completed {
                item: key.clone(),
                completion: Completion::Included {
                    tx_hash,
                    block_number: 1001,
                },
            },
        );
        refused(
            &mut journal,
            &Entry::Reverted {
                item: key.clone(),
                tx_hash: [0xee; 32],
            },
        );
        refused(
            &mut journal,
            &Entry::Reverted {
                item: key.clone(),
                tx_hash,
            },
        );
        assert_eq!(fs::read(&path).unwrap(), before_refusal);
        let item = &journal.state().items[&key];
        assert_eq!(item.submissions.len(), 1);
        assert_eq!(item.submissions[0].tx_hash, tx_hash);
        assert_eq!(item.submissions[0].raw, raw);
        assert_eq!(
            item.submissions[0].status,
            SubmissionStatus::Receipted {
                sequence: 5,
                height: 1002
            }
        );
        assert_eq!(item.pending().unwrap().tx_hash, tx_hash);
        assert!(item.is_open());
        assert_eq!(open(&path).state(), journal.state());
        append(
            &mut journal,
            &Entry::Completed {
                item: key.clone(),
                completion: Completion::Included {
                    tx_hash,
                    block_number: 1003,
                },
            },
        );
        let completed = fs::read(&path).unwrap();
        refused(
            &mut journal,
            &Entry::Completed {
                item: key,
                completion: Completion::AlreadyBridged,
            },
        );
        assert_eq!(fs::read(&path).unwrap(), completed);
        assert_eq!(open(&path).state(), journal.state());
    }

    #[test]
    fn a_solana_release_replays_through_pending_recipient_expiry_and_release() {
        let path = directory("release").join("journal.jsonl");
        let burn = Observation::outbound(
            &OutboundAttestation {
                chain_id: 91_600_046_870_081,
                vault: [0x33; 20],
                paxeer_tx_hash: [0x6f; 32],
                paxeer_nonce: 11,
                recipient: [0xfb; 20],
                asset: [0x21; 20],
                amount: [0x01; 32],
            },
            Position {
                block_number: 28,
                block_hash: [0x1c; 32],
            },
        );
        let key = burn.key();
        let submitted = |signature: u8, height: u64| Entry::ReleaseSubmitted {
            item: key.clone(),
            fee_payer: [0xfe; 32],
            signature: [signature; 64],
            last_valid_block_height: height,
            raw: vec![1, signature],
        };
        let before = {
            let mut journal = open(&path);
            append(
                &mut journal,
                &Entry::Observed {
                    item: key.clone(),
                    observation: burn,
                },
            );
            refused(&mut journal, &submitted(0x51, 2000));
            append(&mut journal, &Entry::RecipientPending { item: key.clone() });
            refused(&mut journal, &Entry::RecipientPending { item: key.clone() });
            assert_eq!(
                journal.state().items[&key].recipient,
                Some(Recipient::Pending)
            );
            append(
                &mut journal,
                &Entry::RecipientResolved {
                    item: key.clone(),
                    key: [0x59; 32],
                },
            );
            refused(
                &mut journal,
                &Entry::RecipientResolved {
                    item: key.clone(),
                    key: [0x5a; 32],
                },
            );
            append(
                &mut journal,
                &Entry::Signed {
                    item: key.clone(),
                    signature: [0x1b; 65],
                },
            );
            append(&mut journal, &submitted(0x51, 2000));
            refused(&mut journal, &submitted(0x52, 2400));
            append(
                &mut journal,
                &Entry::ReleaseExpired {
                    item: key.clone(),
                    signature: [0x51; 64],
                },
            );
            append(&mut journal, &submitted(0x52, 2400));
            append(
                &mut journal,
                &Entry::Completed {
                    item: key.clone(),
                    completion: Completion::Released {
                        signature: [0x52; 64],
                        slot: 1300,
                    },
                },
            );
            journal.state().clone()
        };
        let reopened = open(&path);
        assert_eq!(reopened.state(), &before);
        let item = &reopened.state().items[&key];
        assert_eq!(item.recipient, Some(Recipient::Resolved([0x59; 32])));
        assert_eq!(item.transactions(), 2);
        assert_eq!(item.releases[0].status, SubmissionStatus::Dropped);
        assert_eq!(item.releases[1].raw, vec![1, 0x52]);
        assert_eq!(item.pending_release(), None);
        assert!(!item.is_open());
    }

    fn submitted(tx_hash: u8, nonce: u64) -> Entry {
        Entry::Submitted {
            item: observation().key(),
            submitter: [0xb1; 20],
            nonce,
            tx_hash: [tx_hash; 32],
            raw: vec![2, tx_hash],
        }
    }

    fn included(tx_hash: u8) -> Entry {
        Entry::Completed {
            item: observation().key(),
            completion: Completion::Included {
                tx_hash: [tx_hash; 32],
                block_number: 112,
            },
        }
    }

    fn observed_deposit(journal: &mut Journal) {
        append(
            journal,
            &Entry::Observed {
                item: observation().key(),
                observation: observation(),
            },
        );
    }

    #[test]
    fn an_included_completion_lands_its_submission_and_replays_to_the_same_state() {
        let path = directory("included").join("journal.jsonl");
        let key = observation().key();
        let before = {
            let mut journal = open(&path);
            observed_deposit(&mut journal);
            append(&mut journal, &submitted(0xcc, 5));
            append(
                &mut journal,
                &Entry::Dropped {
                    item: key.clone(),
                    tx_hash: [0xcc; 32],
                },
            );
            append(&mut journal, &submitted(0xcd, 6));
            assert_eq!(
                journal.state().items[&key]
                    .pending()
                    .map(|submission| submission.tx_hash),
                Some([0xcd; 32])
            );
            append(&mut journal, &included(0xcd));
            journal.state().clone()
        };
        let reopened = open(&path);
        assert_eq!(reopened.state(), &before);
        let item = &reopened.state().items[&key];
        assert_eq!(item.pending(), None);
        assert_eq!(item.pending_release(), None);
        assert_eq!(item.submissions[0].status, SubmissionStatus::Dropped);
        assert_eq!(item.submissions[1].status, SubmissionStatus::Landed);
        assert_eq!(
            item.completion,
            Some(Completion::Included {
                tx_hash: [0xcd; 32],
                block_number: 112,
            })
        );
        assert!(!item.is_open());
    }

    #[test]
    fn an_already_bridged_completion_marks_no_submission() {
        let path = directory("already-bridged").join("journal.jsonl");
        let key = observation().key();
        let mut journal = open(&path);
        observed_deposit(&mut journal);
        append(&mut journal, &submitted(0xcc, 5));
        append(
            &mut journal,
            &Entry::Completed {
                item: key.clone(),
                completion: Completion::AlreadyBridged,
            },
        );
        let item = &journal.state().items[&key];
        assert_eq!(item.submissions[0].status, SubmissionStatus::Pending);
        assert!(!item.is_open());
        assert_eq!(open(&path).state(), journal.state());
    }

    #[test]
    fn a_completion_naming_a_transaction_never_submitted_is_refused() {
        let path = directory("unknown-completion").join("journal.jsonl");
        let key = observation().key();
        let mut journal = open(&path);
        observed_deposit(&mut journal);
        refused(&mut journal, &included(0xcc));
        append(&mut journal, &submitted(0xcc, 5));
        let refusal = journal.append(&included(0xee));
        assert!(
            matches!(&refusal, Err(JournalError::Conflict(detail)) if detail.contains(&key)),
            "{refusal:?}"
        );
        refused(
            &mut journal,
            &Entry::Completed {
                item: key.clone(),
                completion: Completion::Released {
                    signature: [0xcc; 64],
                    slot: 1300,
                },
            },
        );
        let item = &journal.state().items[&key];
        assert!(item.is_open());
        assert_eq!(item.submissions[0].status, SubmissionStatus::Pending);
        let lines = fs::read_to_string(&path)
            .unwrap_or_default()
            .lines()
            .count();
        assert_eq!(lines, 2, "refused completions are never written");
        assert_eq!(open(&path).state(), journal.state());
    }

    #[test]
    fn a_release_completion_naming_a_release_never_submitted_is_refused() {
        let path = directory("unknown-release").join("journal.jsonl");
        let mut journal = open(&path);
        let burn = Observation::outbound(
            &OutboundAttestation {
                chain_id: 91_600_046_870_081,
                vault: [0x33; 20],
                paxeer_tx_hash: [0x6f; 32],
                paxeer_nonce: 11,
                recipient: [0xfb; 20],
                asset: [0x21; 20],
                amount: [0x01; 32],
            },
            Position {
                block_number: 28,
                block_hash: [0x1c; 32],
            },
        );
        let burn_key = burn.key();
        append(
            &mut journal,
            &Entry::Observed {
                item: burn_key.clone(),
                observation: burn,
            },
        );
        append(
            &mut journal,
            &Entry::RecipientResolved {
                item: burn_key.clone(),
                key: [0x59; 32],
            },
        );
        append(
            &mut journal,
            &Entry::ReleaseSubmitted {
                item: burn_key.clone(),
                fee_payer: [0xfe; 32],
                signature: [0x51; 64],
                last_valid_block_height: 2000,
                raw: vec![1, 0x51],
            },
        );
        let refusal = journal.append(&Entry::Completed {
            item: burn_key.clone(),
            completion: Completion::Released {
                signature: [0x52; 64],
                slot: 1300,
            },
        });
        assert!(
            matches!(&refusal, Err(JournalError::Conflict(detail)) if detail.contains(&burn_key)),
            "{refusal:?}"
        );
        let burn_item = &journal.state().items[&burn_key];
        assert!(burn_item.is_open());
        assert_eq!(
            burn_item.pending_release().map(|release| release.signature),
            Some([0x51; 64])
        );
        let lines = fs::read_to_string(&path)
            .unwrap_or_default()
            .lines()
            .count();
        assert_eq!(lines, 3, "refused completions are never written");
        assert_eq!(open(&path).state(), journal.state());
    }

    #[test]
    fn a_journal_written_before_landed_existed_replays_its_completion_as_landed() {
        let path = directory("earlier-format").join("journal.jsonl");
        let key = observation().key();
        let bytes = |byte: &str, length: usize| format!("0x{}", byte.repeat(length));
        let written = [
            format!(
                "{{\"kind\":\"observed\",\"item\":\"{key}\",\"observation\":{{\"direction\":\"inbound\",\"chain_id\":1,\"vault\":\"{}\",\"tx_hash\":\"{}\",\"log_index\":3,\"recipient\":\"{}\",\"asset\":\"{}\",\"amount\":\"{}\",\"position\":{{\"block_number\":100,\"block_hash\":\"{}\"}}}}}}",
                bytes("11", 20),
                bytes("aa", 32),
                bytes("55", 32),
                bytes("44", 20),
                bytes("01", 32),
                bytes("bb", 32),
            ),
            format!(
                "{{\"kind\":\"signed\",\"item\":\"{key}\",\"signature\":\"{}\"}}",
                bytes("1b", 65)
            ),
            format!(
                "{{\"kind\":\"submitted\",\"item\":\"{key}\",\"submitter\":\"{}\",\"nonce\":5,\"tx_hash\":\"{}\",\"raw\":\"0x02cc\"}}",
                bytes("b1", 20),
                bytes("cc", 32),
            ),
            format!(
                "{{\"kind\":\"completed\",\"item\":\"{key}\",\"completion\":{{\"outcome\":\"included\",\"tx_hash\":\"{}\",\"block_number\":112}}}}",
                bytes("cc", 32)
            ),
        ];
        let text = written.join("\n") + "\n";
        fs::write(&path, &text).unwrap_or_else(|error| panic!("write: {error}"));
        let replayed = open(&path);
        let item = &replayed.state().items[&key];
        assert_eq!(item.submissions.len(), 1);
        assert_eq!(item.submissions[0].status, SubmissionStatus::Landed);
        assert_eq!(item.pending(), None);
        assert!(!item.is_open());

        let rewritten = directory("earlier-format-rewritten").join("journal.jsonl");
        {
            let mut journal = open(&rewritten);
            observed_deposit(&mut journal);
            append(
                &mut journal,
                &Entry::Signed {
                    item: key.clone(),
                    signature: [0x1b; 65],
                },
            );
            append(&mut journal, &submitted(0xcc, 5));
            append(&mut journal, &included(0xcc));
            assert_eq!(journal.state(), replayed.state());
        }
        assert_eq!(
            fs::read_to_string(&rewritten).unwrap_or_default(),
            text,
            "the journal lines are written exactly as before"
        );
    }

    #[test]
    fn release_entries_are_refused_for_an_inbound_item() {
        let mut inbound = open(&directory("release-inbound").join("journal.jsonl"));
        append(
            &mut inbound,
            &Entry::Observed {
                item: observation().key(),
                observation: observation(),
            },
        );
        refused(
            &mut inbound,
            &Entry::RecipientPending {
                item: observation().key(),
            },
        );
    }

    #[test]
    fn a_torn_final_line_is_truncated_and_a_corrupt_middle_line_is_fatal() {
        let path = directory("torn").join("journal.jsonl");
        let key = observation().key();
        {
            let mut journal = open(&path);
            append(
                &mut journal,
                &Entry::Observed {
                    item: key.clone(),
                    observation: observation(),
                },
            );
        }
        let intact = fs::read(&path).unwrap_or_default();
        let mut torn = intact.clone();
        torn.extend_from_slice(b"{\"kind\":\"signed\",\"item\":\"in:1");
        fs::write(&path, &torn).unwrap_or_else(|error| panic!("write: {error}"));
        let mut journal = open(&path);
        assert_eq!(fs::read(&path).unwrap_or_default(), intact);
        append(
            &mut journal,
            &Entry::Refused {
                item: key,
                reason: "test".to_owned(),
            },
        );
        drop(journal);
        let mut corrupt = b"not json\n".to_vec();
        corrupt.extend_from_slice(&fs::read(&path).unwrap_or_default());
        fs::write(&path, &corrupt).unwrap_or_else(|error| panic!("write: {error}"));
        assert!(matches!(
            Journal::open(&path),
            Err(JournalError::Corrupt { line: 1 })
        ));
    }
}
