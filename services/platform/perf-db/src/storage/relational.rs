use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};

use crate::error::PerfDbError;
use crate::relational::alerts::{Alert, AlertStore};
use crate::relational::api_keys::{ApiKey, ApiKeyStore};
use crate::relational::competitions::{Competition, CompetitionEntry, CompetitionStore};
use crate::relational::referrals::{ReferralEngine, ReferralTier};

use super::snapshot::{self, RelationalSnapshot, SnapshotReferralTier};
use super::wal::{self, RelationalMutation, RelationalWalEntry, RelationalWalWriter, WalConfig};

pub struct RelationalStores {
    pub alerts: AlertStore,
    pub api_keys: ApiKeyStore,
    pub competitions: CompetitionStore,
    pub referrals: ReferralEngine,
}

impl From<&ReferralTier> for SnapshotReferralTier {
    fn from(tier: &ReferralTier) -> Self {
        Self {
            tier: tier.tier.clone(),
            min_referrals: tier.min_referrals,
            min_volume: tier.min_volume,
            reward_bps: tier.reward_bps,
            fee_discount_bps: tier.fee_discount_bps,
        }
    }
}

impl RelationalSnapshot {
    pub fn capture(sequence: u64, next_alert_id: u64, stores: &RelationalStores) -> Self {
        Self::capture_stores(
            sequence,
            next_alert_id,
            &stores.alerts,
            &stores.api_keys,
            &stores.competitions,
            &stores.referrals,
        )
    }

    pub(crate) fn capture_stores(
        sequence: u64,
        next_alert_id: u64,
        alerts: &AlertStore,
        api_keys: &ApiKeyStore,
        competitions: &CompetitionStore,
        referrals: &ReferralEngine,
    ) -> Self {
        Self {
            version: 1,
            sequence,
            next_alert_id,
            alerts: alerts.all(),
            api_keys: api_keys.all(),
            competitions: competitions.all_competitions(),
            competition_entries: competitions.all_entries(),
            referral_tiers: referrals
                .tiers()
                .iter()
                .map(SnapshotReferralTier::from)
                .collect(),
        }
    }

    pub fn restore(&self, tier_names: &[&'static str]) -> Result<RelationalStores, PerfDbError> {
        self.restore_with_tiers(resolve_tiers(&self.referral_tiers, tier_names)?)
    }

    pub(crate) fn restore_owned(&self) -> Result<RelationalStores, PerfDbError> {
        if self.referral_tiers.is_empty() {
            return Err(PerfDbError::Corruption("empty referral tier table".into()));
        }
        let mut names = HashSet::new();
        let mut tiers = Vec::new();
        for tier in &self.referral_tiers {
            if !names.insert(&tier.tier) {
                return Err(PerfDbError::Corruption(
                    "duplicate referral tier name".into(),
                ));
            }
            tiers.push(ReferralTier {
                tier: tier.tier.clone(),
                min_referrals: tier.min_referrals,
                min_volume: tier.min_volume,
                reward_bps: tier.reward_bps,
                fee_discount_bps: tier.fee_discount_bps,
            });
        }
        self.restore_with_tiers(tiers)
    }

    fn restore_with_tiers(
        &self,
        tiers: Vec<ReferralTier>,
    ) -> Result<RelationalStores, PerfDbError> {
        if self.version != 1 {
            return Err(PerfDbError::Corruption(
                "unsupported relational snapshot version".into(),
            ));
        }
        let stores = RelationalStores {
            alerts: AlertStore::new(),
            api_keys: ApiKeyStore::new(),
            competitions: CompetitionStore::new(),
            referrals: ReferralEngine::with_tiers(tiers),
        };
        let mut ids = HashSet::new();
        for alert in &self.alerts {
            if alert.id == 0 || alert.id >= self.next_alert_id || !ids.insert(alert.id) {
                return Err(PerfDbError::Corruption(
                    "invalid alert identity or allocator in snapshot".into(),
                ));
            }
            stores.alerts.set_next_id(alert.id);
            stores.alerts.insert(alert.clone());
        }
        if self.next_alert_id == 0 {
            return Err(PerfDbError::Corruption(
                "invalid next alert identity".into(),
            ));
        }
        stores.alerts.set_next_id(self.next_alert_id);
        let mut keys = HashSet::new();
        let mut hashes = HashSet::new();
        for key in &self.api_keys {
            if !keys.insert(&key.id) || !hashes.insert(&key.key_hash) {
                return Err(PerfDbError::Corruption(
                    "duplicate API key identity or hash".into(),
                ));
            }
            stores.api_keys.insert(key.clone());
        }
        let mut competitions = HashSet::new();
        for competition in &self.competitions {
            if !competitions.insert(&competition.id) {
                return Err(PerfDbError::Corruption(
                    "duplicate competition identity".into(),
                ));
            }
            stores.competitions.insert_competition(competition.clone());
        }
        let mut entries = HashSet::new();
        for entry in &self.competition_entries {
            if entry.rank.is_some()
                || !entries.insert((&entry.competition_id, entry.address))
                || !stores
                    .competitions
                    .join(&entry.competition_id, entry.address, entry.joined_at)
            {
                return Err(PerfDbError::Corruption(
                    "competition entry cannot be restored by the relational interface".into(),
                ));
            }
            stores
                .competitions
                .update_score(&entry.competition_id, &entry.address, entry.score);
        }
        Ok(stores)
    }
}

fn resolve_tiers(
    tiers: &[SnapshotReferralTier],
    names: &[&'static str],
) -> Result<Vec<ReferralTier>, PerfDbError> {
    if tiers.is_empty() {
        return Err(PerfDbError::Corruption("empty referral tier table".into()));
    }
    let defaults = ReferralEngine::new();
    let mut seen = HashSet::new();
    tiers
        .iter()
        .map(|tier| {
            let registered = names.iter().any(|name| *name == tier.tier)
                || defaults
                    .tiers()
                    .iter()
                    .any(|default| default.tier == tier.tier);
            if !registered {
                return Err(PerfDbError::InvalidArgument(format!(
                    "referral tier name requires registration: {}",
                    tier.tier
                )));
            }
            if !seen.insert(tier.tier.as_str()) {
                return Err(PerfDbError::Corruption(
                    "duplicate referral tier name".into(),
                ));
            }
            Ok(ReferralTier {
                tier: tier.tier.clone(),
                min_referrals: tier.min_referrals,
                min_volume: tier.min_volume,
                reward_bps: tier.reward_bps,
                fee_discount_bps: tier.fee_discount_bps,
            })
        })
        .collect()
}

/// Single writer boundary: validate, durably append, then update the real stores.
/// Its WAL lives in its own namespace until the engine owner wires this boundary.
pub struct DurableRelationalStore {
    dir: PathBuf,
    wal: RelationalWalWriter,
    stores: RelationalStores,
    sequence: u64,
    next_alert_id: u64,
    tier_names: Vec<&'static str>,
}

impl DurableRelationalStore {
    pub fn open(dir: &Path, tier_names: &[&'static str]) -> Result<Self, PerfDbError> {
        wal::create_storage_dir(dir)?;
        let wal_dir = dir.join("wal");
        let writer = RelationalWalWriter::open(WalConfig {
            dir: wal_dir.clone(),
            ..WalConfig::default()
        })?;
        let mut entries = Vec::new();
        let mut last_sequence = 0u64;
        wal::replay_relational(&wal_dir, |entry| {
            if last_sequence.checked_add(1) != Some(entry.sequence) {
                return Err(PerfDbError::Corruption(
                    "relational WAL sequence gap".into(),
                ));
            }
            last_sequence = entry.sequence;
            entries.push(entry);
            Ok(())
        })?;
        let snap_dir = dir.join("snapshots");
        let mut snapshots = Vec::new();
        if snap_dir.exists() {
            for entry in fs::read_dir(&snap_dir)
                .map_err(|e| wal::io_error("read relational snapshots", e))?
            {
                let entry =
                    entry.map_err(|e| wal::io_error("read relational snapshot entry", e))?;
                let name = entry.file_name();
                if let Some(sequence) = name
                    .to_str()
                    .and_then(|s| s.strip_prefix("relational_"))
                    .and_then(|s| s.strip_suffix(".bin"))
                    .and_then(|s| s.parse::<u64>().ok())
                {
                    snapshots.push((sequence, entry.path()));
                }
            }
        }
        snapshots.sort_by(|a, b| b.0.cmp(&a.0));
        let mut recovered = None;
        for (sequence, path) in snapshots {
            if sequence > last_sequence {
                return Err(PerfDbError::Corruption(
                    "relational snapshot lacks retained WAL coverage".into(),
                ));
            }
            let bytes = fs::read(path).map_err(|e| wal::io_error("read relational snapshot", e))?;
            match snapshot::decode_image::<RelationalSnapshot>(&bytes) {
                Ok(image) => {
                    if image.sequence != sequence || image.version != 1 {
                        return Err(PerfDbError::Corruption(
                            "relational snapshot version or sequence mismatch".into(),
                        ));
                    }
                    let stores = image.restore(tier_names)?;
                    recovered = Some((image, stores));
                    break;
                }
                Err(PerfDbError::Corruption(detail)) => tracing::warn!(
                    sequence,
                    detail,
                    "invalid relational snapshot; retained WAL required"
                ),
                Err(error) => return Err(error),
            }
        }
        let (sequence, next_alert_id, stores) = if let Some((image, stores)) = recovered {
            (image.sequence, image.next_alert_id, stores)
        } else {
            (
                0,
                1,
                RelationalStores {
                    alerts: AlertStore::new(),
                    api_keys: ApiKeyStore::new(),
                    competitions: CompetitionStore::new(),
                    referrals: ReferralEngine::new(),
                },
            )
        };
        let mut storage = Self {
            dir: dir.into(),
            wal: writer,
            stores,
            sequence,
            next_alert_id,
            tier_names: tier_names.to_vec(),
        };
        for entry in entries {
            if entry.sequence > storage.sequence {
                storage.validate(&entry.mutation)?;
                storage.apply(entry.mutation)?;
                storage.sequence = entry.sequence;
            }
        }
        Ok(storage)
    }

    fn validate(&self, mutation: &RelationalMutation) -> Result<(), PerfDbError> {
        match mutation {
            RelationalMutation::PutAlert(alert) if alert.id == 0 || alert.id == u64::MAX => {
                return Err(PerfDbError::InvalidArgument(
                    "alert identity exhausted or zero".into(),
                ));
            }
            RelationalMutation::PutApiKey(key) => {
                if self
                    .stores
                    .api_keys
                    .get_by_hash(&key.key_hash)
                    .is_some_and(|old| old.id != key.id)
                {
                    return Err(PerfDbError::InvalidArgument(
                        "API key hash already assigned".into(),
                    ));
                }
            }
            RelationalMutation::PutCompetitionEntry(entry) => {
                if self
                    .stores
                    .competitions
                    .get_competition(&entry.competition_id)
                    .is_none()
                    || entry.rank.is_some()
                {
                    return Err(PerfDbError::InvalidArgument(
                        "competition entry requires an existing competition and an unranked entry"
                            .into(),
                    ));
                }
                if self
                    .stores
                    .competitions
                    .get_entries(&entry.competition_id, usize::MAX)
                    .iter()
                    .any(|old| old.address == entry.address && old.joined_at != entry.joined_at)
                {
                    return Err(PerfDbError::InvalidArgument(
                        "competition join timestamp cannot change".into(),
                    ));
                }
            }
            RelationalMutation::SetReferralTiers(tiers) => {
                resolve_tiers(tiers, &self.tier_names)?;
            }
            _ => {}
        }
        Ok(())
    }

    fn apply(&mut self, mutation: RelationalMutation) -> Result<(), PerfDbError> {
        match mutation {
            RelationalMutation::PutAlert(alert) => {
                self.next_alert_id = self.next_alert_id.max(alert.id + 1);
                self.stores.alerts.remove(alert.id);
                self.stores.alerts.set_next_id(alert.id);
                self.stores.alerts.insert(alert);
                self.stores.alerts.set_next_id(self.next_alert_id);
            }
            RelationalMutation::DeleteAlert(id) => {
                self.stores.alerts.remove(id);
            }
            RelationalMutation::PutApiKey(key) => {
                self.stores.api_keys.remove(&key.id);
                self.stores.api_keys.insert(key);
            }
            RelationalMutation::DeleteApiKey(id) => {
                self.stores.api_keys.remove(&id);
            }
            RelationalMutation::PutCompetition(competition) => {
                self.stores.competitions.insert_competition(competition)
            }
            RelationalMutation::PutCompetitionEntry(entry) => {
                if !self
                    .stores
                    .competitions
                    .get_entries(&entry.competition_id, usize::MAX)
                    .iter()
                    .any(|old| old.address == entry.address)
                {
                    self.stores.competitions.join(
                        &entry.competition_id,
                        entry.address,
                        entry.joined_at,
                    );
                }
                self.stores.competitions.update_score(
                    &entry.competition_id,
                    &entry.address,
                    entry.score,
                );
            }
            RelationalMutation::SetReferralTiers(tiers) => self
                .stores
                .referrals
                .set_tiers(resolve_tiers(&tiers, &self.tier_names)?),
        }
        Ok(())
    }

    pub fn mutate(&mut self, mutation: RelationalMutation) -> Result<(), PerfDbError> {
        self.validate(&mutation)?;
        let sequence = self
            .sequence
            .checked_add(1)
            .ok_or_else(|| PerfDbError::InvalidArgument("relational sequence exhausted".into()))?;
        self.wal.append(&RelationalWalEntry {
            version: 1,
            sequence,
            mutation: mutation.clone(),
        })?;
        self.apply(mutation)?;
        self.sequence = sequence;
        Ok(())
    }

    pub fn insert_alert(&mut self, mut alert: Alert) -> Result<u64, PerfDbError> {
        alert.id = self.next_alert_id;
        let id = alert.id;
        self.mutate(RelationalMutation::PutAlert(alert))?;
        Ok(id)
    }

    pub fn alert(&self, id: u64) -> Option<Alert> {
        self.stores.alerts.get(id)
    }
    pub fn alerts_by_user(&self, user: &[u8; 20]) -> Vec<Alert> {
        self.stores.alerts.by_user(user)
    }
    pub fn api_key(&self, id: &str) -> Option<ApiKey> {
        self.stores.api_keys.get(id)
    }
    pub fn api_key_by_hash(&self, hash: &str) -> Option<ApiKey> {
        self.stores.api_keys.get_by_hash(hash)
    }
    pub fn api_keys_by_user(&self, user: &[u8; 20]) -> Vec<ApiKey> {
        self.stores.api_keys.by_user(user)
    }
    pub fn competition(&self, id: &str) -> Option<Competition> {
        self.stores.competitions.get_competition(id)
    }
    pub fn competition_entries(&self, id: &str) -> Vec<CompetitionEntry> {
        self.stores.competitions.get_entries(id, usize::MAX)
    }
    pub fn user_competitions(&self, user: &[u8; 20]) -> Vec<String> {
        self.stores.competitions.user_competitions(user)
    }
    pub fn referral_tier(&self, count: u32, volume: i128) -> &ReferralTier {
        self.stores.referrals.compute_tier(count, volume)
    }
    pub fn snapshot(&self) -> RelationalSnapshot {
        RelationalSnapshot::capture(self.sequence, self.next_alert_id, &self.stores)
    }

    pub fn shutdown(&mut self) -> Result<PathBuf, PerfDbError> {
        self.wal.sync()?;
        let path = self
            .dir
            .join("snapshots")
            .join(format!("relational_{:020}.bin", self.sequence));
        snapshot::atomic_publish(&path, &snapshot::encode_image(&self.snapshot())?)?;
        Ok(path)
    }
}
