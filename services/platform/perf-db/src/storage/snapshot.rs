use std::fs::{self, OpenOptions};
use std::io::Write;
use std::sync::atomic::{AtomicU64, Ordering};

use bincode::Options;
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use crate::core::types::{Balance, DailyStats, Order, Position, TraderStats, User};
use crate::error::PerfDbError;
use crate::relational::checkpoint::Checkpoint;
use crate::state::market_state::MarketSnapshot;
use crate::storage::wal::WalPosition;

/// Complete serializable snapshot of all in-memory state.
///
/// Used for fast recovery: load last snapshot, then replay WAL entries
/// written after the snapshot's checkpoint block.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateSnapshot {
    pub checkpoint: SnapshotCheckpoint,
    pub positions: Vec<Position>,
    pub orders: Vec<Order>,
    pub balances: Vec<Balance>,
    pub users: Vec<User>,
    pub trader_stats: Vec<TraderStats>,
    pub daily_stats: Vec<DailyStatsEntry>,
    pub market_states: Vec<SnapshotMarketState>,
    pub snapshot_timestamp: u64,
}

/// Checkpoint info embedded in the snapshot.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotCheckpoint {
    pub last_block: u64,
    pub last_block_hash: String,
    pub last_block_timestamp: u64,
}

impl From<Checkpoint> for SnapshotCheckpoint {
    fn from(cp: Checkpoint) -> Self {
        Self {
            last_block: cp.last_block,
            last_block_hash: cp.last_block_hash,
            last_block_timestamp: cp.last_block_timestamp,
        }
    }
}

/// Daily stats entry with the address included (the key is stripped
/// from DashMap on snapshot).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DailyStatsEntry {
    pub address: [u8; 20],
    pub stats: DailyStats,
}

/// Serializable version of MarketSnapshot (the original isn't Serialize).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotMarketState {
    pub market_id: u16,
    pub latest_price: f64,
    pub last_update_ns: u64,
    pub mark_price: Option<i128>,
    pub index_price: Option<i128>,
    pub funding_rate_per_second: i128,
    pub funding_rate_24h: i128,
    pub long_oi: i128,
    pub short_oi: i128,
    pub enabled: bool,
    pub paused: bool,
    pub max_leverage: Option<i128>,
    pub maintenance_margin_bps: Option<u32>,
    pub volume_24h: Option<i128>,
    pub trades_24h: Option<u32>,
    pub price_change_24h: Option<i128>,
    pub price_change_pct_24h: Option<i128>,
}

impl From<MarketSnapshot> for SnapshotMarketState {
    fn from(ms: MarketSnapshot) -> Self {
        Self {
            market_id: ms.market_id,
            latest_price: ms.latest_price,
            last_update_ns: ms.last_update_ns,
            mark_price: ms.mark_price,
            index_price: ms.index_price,
            funding_rate_per_second: ms.funding_rate_per_second,
            funding_rate_24h: ms.funding_rate_24h,
            long_oi: ms.long_oi,
            short_oi: ms.short_oi,
            enabled: ms.enabled,
            paused: ms.paused,
            max_leverage: ms.max_leverage,
            maintenance_margin_bps: ms.maintenance_margin_bps,
            volume_24h: ms.volume_24h,
            trades_24h: ms.trades_24h,
            price_change_24h: ms.price_change_24h,
            price_change_pct_24h: ms.price_change_pct_24h,
        }
    }
}

const SNAPSHOT_MAGIC: &[u8; 8] = b"PDBSNAP1";
static TEMP_SEQUENCE: AtomicU64 = AtomicU64::new(0);

/// An owned tier name is used on disk; the relational engine currently requires
/// static names, resolved explicitly by the caller when restoring the table.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotReferralTier {
    pub tier: String,
    pub min_referrals: u32,
    pub min_volume: i128,
    pub reward_bps: u32,
    pub fee_discount_bps: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelationalSnapshot {
    pub version: u32,
    pub sequence: u64,
    pub next_alert_id: u64,
    pub alerts: Vec<crate::relational::alerts::Alert>,
    pub api_keys: Vec<crate::relational::api_keys::ApiKey>,
    pub competitions: Vec<crate::relational::competitions::Competition>,
    pub competition_entries: Vec<crate::relational::competitions::CompetitionEntry>,
    pub referral_tiers: Vec<SnapshotReferralTier>,
}

/// Versioned extension without changing the legacy engine's struct literals or
/// silently treating an old snapshot as containing relational state.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompleteStateSnapshot {
    pub version: u32,
    pub state: StateSnapshot,
    pub relational: Option<RelationalSnapshot>,
}

/// Engine recovery is ordered by a WAL frame, never by block height. Keeping
/// the event ledger in this same image makes deduplication atomic with state.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SnapshotRecovery {
    pub position: WalPosition,
    pub events: Vec<crate::core::types::IndexedEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct EngineSnapshot {
    version: u32,
    complete: CompleteStateSnapshot,
    recovery: SnapshotRecovery,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub(crate) struct TickCheckpoint {
    pub version: u32,
    pub position: WalPosition,
    pub tick_counts: Vec<u64>,
    pub candle_counts: Vec<u64>,
    pub candle_tails: Vec<Option<crate::core::types::CandleRecord>>,
    pub builder: crate::aggregation::candle_builder::CandleBuilder,
}

pub(crate) fn load_tick_checkpoint(dir: &Path) -> Result<Option<TickCheckpoint>, PerfDbError> {
    let path = dir.join("tick_checkpoint.bin");
    let data = match fs::read(path) {
        Ok(data) => data,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(e) => return Err(super::wal::io_error("read tick checkpoint", e)),
    };
    let checkpoint: TickCheckpoint = decode_image(&data)?;
    if checkpoint.version != 1
        || checkpoint.tick_counts.len() != crate::core::market::MARKET_COUNT
        || checkpoint.candle_counts.len() != crate::core::market::MARKET_COUNT * 16
        || checkpoint.candle_tails.len() != crate::core::market::MARKET_COUNT * 16
    {
        return Err(PerfDbError::Corruption("invalid tick checkpoint".into()));
    }
    checkpoint.builder.validate_recovery_state()?;
    for (index, (&count, tail)) in checkpoint
        .candle_counts
        .iter()
        .zip(&checkpoint.candle_tails)
        .enumerate()
    {
        if (count == 0) != tail.is_none()
            || tail.is_some_and(|candle| {
                candle.market_id as usize != index / 16 || candle.timeframe as usize != index % 16
            })
        {
            return Err(PerfDbError::Corruption(
                "invalid committed candle tail".into(),
            ));
        }
    }
    Ok(Some(checkpoint))
}

pub(crate) fn write_tick_checkpoint(
    dir: &Path,
    checkpoint: &TickCheckpoint,
) -> Result<(), PerfDbError> {
    atomic_publish(&dir.join("tick_checkpoint.bin"), &encode_image(checkpoint)?)
}

pub fn write_engine_snapshot(
    dir: &Path,
    state: &StateSnapshot,
    recovery: SnapshotRecovery,
) -> Result<PathBuf, PerfDbError> {
    let image = EngineSnapshot {
        version: 2,
        complete: CompleteStateSnapshot {
            version: 1,
            state: state.clone(),
            relational: None,
        },
        recovery,
    };
    let path = dir.join(format!("snapshot_{:012}.bin", state.checkpoint.last_block));
    atomic_publish(&path, &encode_image(&image)?)?;
    Ok(path)
}

pub fn write_snapshot(dir: &Path, snapshot: &StateSnapshot) -> Result<PathBuf, PerfDbError> {
    write_complete_snapshot(
        dir,
        &CompleteStateSnapshot {
            version: 1,
            state: snapshot.clone(),
            relational: None,
        },
    )
}

pub fn write_complete_snapshot(
    dir: &Path,
    snapshot: &CompleteStateSnapshot,
) -> Result<PathBuf, PerfDbError> {
    if snapshot.version != 1 || snapshot.relational.as_ref().is_some_and(|r| r.version != 1) {
        return Err(PerfDbError::InvalidArgument(
            "unsupported snapshot version".into(),
        ));
    }
    let path = dir.join(format!(
        "snapshot_{:012}.bin",
        snapshot.state.checkpoint.last_block
    ));
    let data = encode_image(snapshot)?;
    atomic_publish(&path, &data)?;
    Ok(path)
}

pub(crate) fn encode_image<T: Serialize>(value: &T) -> Result<Vec<u8>, PerfDbError> {
    let payload = bincode::serialize(value).map_err(|e| PerfDbError::Serialization {
        context: "snapshot serialize".into(),
        detail: e.to_string(),
    })?;
    let mut data = Vec::with_capacity(20 + payload.len());
    data.extend_from_slice(SNAPSHOT_MAGIC);
    data.extend_from_slice(&(payload.len() as u64).to_le_bytes());
    data.extend_from_slice(&crc32fast::hash(&payload).to_le_bytes());
    data.extend_from_slice(&payload);
    Ok(data)
}

pub(crate) fn decode_image<T: serde::de::DeserializeOwned>(data: &[u8]) -> Result<T, PerfDbError> {
    if data.len() < 20
        || &data[..8] != SNAPSHOT_MAGIC
        || u64::from_le_bytes(data[8..16].try_into().unwrap()) != (data.len() - 20) as u64
        || u32::from_le_bytes(data[16..20].try_into().unwrap()) != crc32fast::hash(&data[20..])
    {
        return Err(PerfDbError::Corruption(
            "snapshot header, length or checksum mismatch".into(),
        ));
    }
    bincode::DefaultOptions::new()
        .with_fixint_encoding()
        .reject_trailing_bytes()
        .with_limit(data.len() as u64)
        .deserialize(&data[20..])
        .map_err(|e| PerfDbError::Corruption(format!("snapshot decode: {e}")))
}

pub(crate) fn atomic_publish(path: &Path, data: &[u8]) -> Result<(), PerfDbError> {
    use super::wal::{io_error, sync_directory};
    let dir = path
        .parent()
        .ok_or_else(|| PerfDbError::InvalidArgument("snapshot has no parent".into()))?;
    super::wal::create_storage_dir(dir)?;
    let (temp, mut file) = loop {
        let id = TEMP_SEQUENCE.fetch_add(1, Ordering::Relaxed);
        let temp = dir.join(format!(".snapshot-{}-{id}.tmp", std::process::id()));
        let mut options = OpenOptions::new();
        options.write(true).create_new(true);
        #[cfg(unix)]
        {
            use std::os::unix::fs::OpenOptionsExt;
            options.mode(0o600);
        }
        match options.open(&temp) {
            Ok(file) => break (temp, file),
            Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => continue,
            Err(e) => return Err(io_error("create temporary snapshot", e)),
        }
    };
    let result = (|| {
        file.write_all(data)
            .map_err(|e| io_error("write temporary snapshot", e))?;
        file.sync_all()
            .map_err(|e| io_error("sync temporary snapshot", e))?;
        fs::rename(&temp, path).map_err(|e| io_error("publish snapshot", e))?;
        sync_directory(dir)
    })();
    if result.is_err() && temp.exists() {
        fs::remove_file(&temp).map_err(|e| io_error("remove temporary snapshot", e))?;
    }
    result
}

fn read_snapshot_with_recovery(
    path: &Path,
    block: u64,
) -> Result<(CompleteStateSnapshot, Option<SnapshotRecovery>), PerfDbError> {
    let data = fs::read(path).map_err(|e| super::wal::io_error("read snapshot", e))?;
    let mut recovery = None;
    let image: CompleteStateSnapshot = if data.starts_with(SNAPSHOT_MAGIC) {
        if data.get(20..24) == Some(&2u32.to_le_bytes()[..]) {
            let engine: EngineSnapshot = decode_image(&data)?;
            recovery = Some(engine.recovery);
            engine.complete
        } else {
            decode_image(&data)?
        }
    } else {
        let state = bincode::DefaultOptions::new()
            .with_fixint_encoding()
            .reject_trailing_bytes()
            .with_limit(data.len() as u64)
            .deserialize(&data)
            .map_err(|e| PerfDbError::Corruption(format!("legacy snapshot decode: {e}")))?;
        CompleteStateSnapshot {
            version: 1,
            state,
            relational: None,
        }
    };
    if image.version != 1 || image.relational.as_ref().is_some_and(|r| r.version != 1) {
        return Err(PerfDbError::Serialization {
            context: "snapshot version".into(),
            detail: "unsupported snapshot version".into(),
        });
    }
    if image.state.checkpoint.last_block != block {
        return Err(PerfDbError::Corruption(
            "snapshot version or checkpoint does not match filename".into(),
        ));
    }
    Ok((image, recovery))
}

fn read_snapshot(path: &Path, block: u64) -> Result<CompleteStateSnapshot, PerfDbError> {
    read_snapshot_with_recovery(path, block).map(|(image, _)| image)
}

pub fn load_latest_complete_snapshot(
    dir: &Path,
) -> Result<Option<CompleteStateSnapshot>, PerfDbError> {
    load_latest_with_recovery(dir).map(|image| image.map(|(state, _)| state))
}

fn load_latest_with_recovery(
    dir: &Path,
) -> Result<Option<(CompleteStateSnapshot, Option<SnapshotRecovery>)>, PerfDbError> {
    let mut snapshots = list_snapshots(dir)?;
    snapshots.sort_by(|a, b| b.0.cmp(&a.0));
    let mut rejected_block = None;
    for (block, path) in snapshots {
        match read_snapshot_with_recovery(&path, block) {
            Ok(snapshot) => {
                if let Some(through) = rejected_block {
                    let wal_dir = dir
                        .parent()
                        .ok_or_else(|| PerfDbError::Corruption("missing snapshot parent".into()))?
                        .join("wal");
                    super::wal::verify_snapshot_coverage(&wal_dir, block, through)?;
                }
                return Ok(Some(snapshot));
            }
            Err(PerfDbError::Corruption(detail)) => {
                tracing::warn!(block, detail, "rejecting invalid recovery snapshot");
                rejected_block.get_or_insert(block);
            }
            Err(error) => return Err(error),
        }
    }
    if rejected_block.is_some() {
        return Err(PerfDbError::Corruption(
            "no usable recovery snapshot".into(),
        ));
    }
    Ok(None)
}

pub fn load_latest_engine_snapshot(
    dir: &Path,
) -> Result<Option<(StateSnapshot, SnapshotRecovery)>, PerfDbError> {
    match load_latest_with_recovery(dir)? {
        Some((image, _)) if image.relational.is_some() => Err(PerfDbError::InvalidArgument(
            "relational snapshot requires engine restoration through load_latest_complete_snapshot"
                .into(),
        )),
        Some((image, Some(recovery))) => Ok(Some((image.state, recovery))),
        Some((_, None)) => Err(PerfDbError::Corruption(
            "legacy snapshot has no exact WAL recovery cursor; explicit migration is required"
                .into(),
        )),
        None => Ok(None),
    }
}

pub fn load_latest_snapshot(dir: &Path) -> Result<Option<StateSnapshot>, PerfDbError> {
    match load_latest_complete_snapshot(dir)? {
        Some(image) if image.relational.is_some() => Err(PerfDbError::InvalidArgument(
            "relational snapshot requires engine restoration through load_latest_complete_snapshot"
                .into(),
        )),
        image => Ok(image.map(|image| image.state)),
    }
}

pub fn load_snapshot(dir: &Path, block: u64) -> Result<Option<StateSnapshot>, PerfDbError> {
    let path = dir.join(format!("snapshot_{block:012}.bin"));
    if !path.exists() {
        return Ok(None);
    }
    Ok(Some(read_snapshot(&path, block)?.state))
}

/// List all snapshot files in a directory as (block_number, path) pairs.
pub fn list_snapshots(dir: &Path) -> Result<Vec<(u64, PathBuf)>, PerfDbError> {
    if !dir.exists() {
        return Ok(Vec::new());
    }

    let mut results = Vec::new();

    for entry in fs::read_dir(dir).map_err(|e| PerfDbError::Io {
        context: format!("read snapshot dir: {}", dir.display()),
        source: e,
    })? {
        let entry = entry.map_err(|e| PerfDbError::Io {
            context: "read snapshot dir entry".into(),
            source: e,
        })?;

        let name = entry.file_name();
        let name = name.to_string_lossy();

        if let Some(num_str) = name
            .strip_prefix("snapshot_")
            .and_then(|s| s.strip_suffix(".bin"))
        {
            if let Ok(block) = num_str.parse::<u64>() {
                results.push((block, entry.path()));
            }
        }
    }

    Ok(results)
}

/// Remove all snapshots older than `keep_from_block`.
pub fn cleanup_snapshots(dir: &Path, keep_from_block: u64) -> Result<u64, PerfDbError> {
    let snapshots = list_snapshots(dir)?;
    let mut removed = 0u64;

    for (block, path) in snapshots {
        if block < keep_from_block {
            fs::remove_file(&path).map_err(|e| PerfDbError::Io {
                context: format!("remove snapshot: {}", path.display()),
                source: e,
            })?;
            removed += 1;
        }
    }

    if removed > 0 {
        tracing::info!(removed, keep_from_block, "old snapshots cleaned up");
    }

    Ok(removed)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_snapshot(block: u64) -> StateSnapshot {
        StateSnapshot {
            checkpoint: SnapshotCheckpoint {
                last_block: block,
                last_block_hash: format!("0x{:016x}", block),
                last_block_timestamp: block * 12,
            },
            positions: Vec::new(),
            orders: Vec::new(),
            balances: Vec::new(),
            users: Vec::new(),
            trader_stats: Vec::new(),
            daily_stats: Vec::new(),
            market_states: Vec::new(),
            snapshot_timestamp: block * 12,
        }
    }

    #[test]
    fn write_and_load_roundtrip() {
        let dir = tempfile::tempdir().unwrap();
        let snap_dir = dir.path().join("snapshots");

        let snap = make_snapshot(100);
        let path = write_snapshot(&snap_dir, &snap).unwrap();
        assert!(path.exists());

        let loaded = load_latest_snapshot(&snap_dir).unwrap().unwrap();
        assert_eq!(loaded.checkpoint.last_block, 100);
        assert_eq!(loaded.checkpoint.last_block_hash, "0x0000000000000064");
    }

    #[test]
    fn load_latest_picks_highest_block() {
        let dir = tempfile::tempdir().unwrap();
        let snap_dir = dir.path().join("snapshots");

        write_snapshot(&snap_dir, &make_snapshot(50)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(200)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(100)).unwrap();

        let loaded = load_latest_snapshot(&snap_dir).unwrap().unwrap();
        assert_eq!(loaded.checkpoint.last_block, 200);
    }

    #[test]
    fn load_specific_snapshot() {
        let dir = tempfile::tempdir().unwrap();
        let snap_dir = dir.path().join("snapshots");

        write_snapshot(&snap_dir, &make_snapshot(100)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(200)).unwrap();

        let loaded = load_snapshot(&snap_dir, 100).unwrap().unwrap();
        assert_eq!(loaded.checkpoint.last_block, 100);

        assert!(load_snapshot(&snap_dir, 999).unwrap().is_none());
    }

    #[test]
    fn load_from_empty_dir() {
        let dir = tempfile::tempdir().unwrap();
        assert!(load_latest_snapshot(dir.path()).unwrap().is_none());
    }

    #[test]
    fn load_from_nonexistent_dir() {
        let dir = Path::new("/tmp/perfdb_test_nonexistent_snapshot_dir_xyz");
        assert!(load_latest_snapshot(dir).unwrap().is_none());
    }

    #[test]
    fn list_snapshots_sorted() {
        let dir = tempfile::tempdir().unwrap();
        let snap_dir = dir.path().join("snapshots");

        write_snapshot(&snap_dir, &make_snapshot(300)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(100)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(200)).unwrap();

        let list = list_snapshots(&snap_dir).unwrap();
        assert_eq!(list.len(), 3);
    }

    #[test]
    fn cleanup_old_snapshots() {
        let dir = tempfile::tempdir().unwrap();
        let snap_dir = dir.path().join("snapshots");

        write_snapshot(&snap_dir, &make_snapshot(100)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(200)).unwrap();
        write_snapshot(&snap_dir, &make_snapshot(300)).unwrap();

        let removed = cleanup_snapshots(&snap_dir, 200).unwrap();
        assert_eq!(removed, 1);

        let remaining = list_snapshots(&snap_dir).unwrap();
        assert_eq!(remaining.len(), 2);
        assert!(remaining.iter().all(|(b, _)| *b >= 200));
    }

    #[test]
    fn snapshot_with_data() {
        let dir = tempfile::tempdir().unwrap();
        let snap_dir = dir.path().join("snapshots");

        let mut snap = make_snapshot(42);
        snap.positions.push(Position {
            position_id: [1u8; 32],
            user_address: [2u8; 20],
            market_id: 0,
            is_long: true,
            size_usd: 1_000_000_000_000_000_000_000,
            collateral_usd: 100_000_000_000_000_000_000,
            collateral_token: [3u8; 20],
            collateral_amount: 100_000_000,
            entry_price: 67_000_000_000_000_000_000_000,
            exit_price: None,
            realized_pnl: None,
            leverage: 10_000_000_000_000_000_000,
            status: crate::core::types::PositionStatus::Open,
            open_tx: [4u8; 32],
            close_tx: None,
            open_block: 40,
            close_block: None,
            opened_at: 480,
            closed_at: None,
        });
        snap.users.push(User {
            address: [2u8; 20],
            vault_address: None,
            referral_code: Some("ABC".to_string()),
            referred_by: None,
            tier: "silver".to_string(),
            fee_discount_bps: 5,
            created_at: 100,
            first_trade_at: Some(200),
            last_active_at: Some(480),
        });
        snap.market_states.push(SnapshotMarketState {
            market_id: 0,
            latest_price: 67_000.5,
            last_update_ns: 42_000_000_000,
            mark_price: Some(67_000_000_000_000_000_000_000),
            index_price: None,
            funding_rate_per_second: 500,
            funding_rate_24h: 43_200_000,
            long_oi: 10_000_000,
            short_oi: 8_000_000,
            enabled: true,
            paused: false,
            max_leverage: Some(100_000_000_000_000_000_000),
            maintenance_margin_bps: Some(50),
            volume_24h: None,
            trades_24h: None,
            price_change_24h: None,
            price_change_pct_24h: None,
        });

        write_snapshot(&snap_dir, &snap).unwrap();

        let loaded = load_latest_snapshot(&snap_dir).unwrap().unwrap();
        assert_eq!(loaded.positions.len(), 1);
        assert_eq!(loaded.positions[0].market_id, 0);
        assert_eq!(loaded.users.len(), 1);
        assert_eq!(loaded.users[0].referral_code, Some("ABC".to_string()));
        assert_eq!(loaded.market_states.len(), 1);
        assert_eq!(loaded.market_states[0].latest_price, 67_000.5);
    }
}
