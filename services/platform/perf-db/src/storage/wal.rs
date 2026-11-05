use std::fs::{self, File, OpenOptions};
use std::io::{BufWriter, Read, Seek, SeekFrom, Write};
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};

use crc32fast::Hasher as Crc32Hasher;
use serde::{Deserialize, Serialize, de::DeserializeOwned};

use crate::core::types::IndexedEvent;
use crate::error::PerfDbError;

/// Every mutation in PerfDB is first written to the WAL as a `WalEntry`.
///
/// This is the complete vocabulary of state changes, covering all 4 replaced
/// databases.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum WalEntry {
    /// Chain event from the indexer (covers all 17 DecodedEvent variants).
    IndexedEvent(IndexedEvent),

    /// Raw price tick from the price feed.
    PriceTick {
        timestamp_ns: u64,
        market_id: u16,
        price: f64,
    },

    /// Pre-computed candle update.
    CandleUpdate {
        timestamp_ns: u64,
        market_id: u16,
        timeframe: u8,
        open: f64,
        high: f64,
        low: f64,
        close: f64,
        volume: f32,
    },

    /// User balance change.
    BalanceUpdate {
        user: [u8; 20],
        token: [u8; 20],
        amount: i128,
        locked: i128,
        available: i128,
    },

    /// Indexer checkpoint (last processed block).
    Checkpoint { block_number: u64 },

    /// Marks the end of a batch (used for batch fsync).
    BatchEnd,
}

/// WAL behavior configuration.
#[derive(Debug, Clone)]
pub struct WalConfig {
    /// Directory where WAL segment files are stored.
    pub dir: PathBuf,

    /// Maximum segment size before rotation (bytes).
    /// Default: 64 MB.
    pub max_segment_size: u64,

    /// Fsync policy.
    pub sync_policy: SyncPolicy,
}

#[derive(Debug, Clone, Copy)]
pub enum SyncPolicy {
    /// Fsync after every write. Safest. ~10μs per write.
    EveryWrite,

    /// Fsync every N milliseconds. Risk losing last batch on crash.
    Interval(Duration),

    /// Never fsync (OS decides). Fastest. Data loss on crash.
    None,
}

impl Default for WalConfig {
    fn default() -> Self {
        Self {
            dir: PathBuf::from("wal"),
            max_segment_size: 64 * 1024 * 1024,
            sync_policy: SyncPolicy::Interval(Duration::from_millis(10)),
        }
    }
}

/// On-disk entry format: `[len: u32 LE][crc32: u32 LE][payload: len bytes]`.
/// The payload is the complete bincode value (including its enum discriminant);
/// the checksum covers exactly the payload. There is no separate type byte.
const ENTRY_HEADER_SIZE: usize = 8;
const MAX_ENTRY_SIZE: usize = 128 * 1024 * 1024;

/// Exclusive end of an applied frame. Unlike a block height, this also orders
/// multiple events from the same block and survives segment rotation/pruning.
#[derive(Debug, Default, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Serialize, Deserialize)]
pub struct WalPosition {
    pub segment: u64,
    pub offset: u64,
}

pub struct WalWriter {
    config: WalConfig,
    current_segment: u64,
    writer: BufWriter<File>,
    segment_bytes: u64,
    last_sync: Instant,
}

impl WalWriter {
    /// Open or create a WAL in the given directory.
    pub fn open(config: WalConfig) -> Result<Self, PerfDbError> {
        Self::open_for::<WalEntry>(config)
    }

    fn open_for<T: DeserializeOwned>(config: WalConfig) -> Result<Self, PerfDbError> {
        create_storage_dir(&config.dir)?;

        // Validate every segment before changing the final one or enabling writes.
        let (_, damaged_tail) = scan_wal::<T>(&config.dir, |_| Ok(()))?;
        if let Some((segment, valid_offset)) = damaged_tail {
            let path = segment_path(&config.dir, segment);
            let file = OpenOptions::new()
                .write(true)
                .open(&path)
                .map_err(|e| io_error("open damaged WAL tail", e))?;
            file.set_len(valid_offset)
                .map_err(|e| io_error("truncate damaged WAL tail", e))?;
            file.sync_all()
                .map_err(|e| io_error("sync repaired WAL tail", e))?;
        }
        let current_segment = find_latest_segment(&config.dir)?;
        let seg_path = segment_path(&config.dir, current_segment);

        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&seg_path)
            .map_err(|e| PerfDbError::Io {
                context: format!("open WAL segment: {}", seg_path.display()),
                source: e,
            })?;

        file.sync_all()
            .map_err(|e| io_error("sync WAL segment", e))?;
        sync_directory(&config.dir)?;

        let segment_bytes = file
            .metadata()
            .map_err(|e| PerfDbError::Io {
                context: "WAL segment metadata".into(),
                source: e,
            })?
            .len();

        Ok(Self {
            config,
            current_segment,
            writer: BufWriter::with_capacity(64 * 1024, file),
            segment_bytes,
            last_sync: Instant::now(),
        })
    }

    /// Append a WAL entry.
    pub fn append(&mut self, entry: &WalEntry) -> Result<(), PerfDbError> {
        let payload = bincode::serialize(entry).map_err(|e| PerfDbError::Serialization {
            context: "WAL entry serialize".into(),
            detail: e.to_string(),
        })?;

        self.append_payload(&payload)
    }

    fn append_payload(&mut self, payload: &[u8]) -> Result<(), PerfDbError> {
        if payload.len() > MAX_ENTRY_SIZE {
            return Err(PerfDbError::InvalidArgument(
                "WAL entry exceeds maximum size".into(),
            ));
        }
        let len = payload.len() as u32;

        let mut hasher = Crc32Hasher::new();
        hasher.update(&payload);
        let crc = hasher.finalize();

        self.writer
            .write_all(&len.to_le_bytes())
            .map_err(|e| PerfDbError::Io {
                context: "WAL write len".into(),
                source: e,
            })?;
        self.writer
            .write_all(&crc.to_le_bytes())
            .map_err(|e| PerfDbError::Io {
                context: "WAL write crc".into(),
                source: e,
            })?;
        self.writer
            .write_all(&payload)
            .map_err(|e| PerfDbError::Io {
                context: "WAL write payload".into(),
                source: e,
            })?;

        self.segment_bytes += ENTRY_HEADER_SIZE as u64 + payload.len() as u64;

        match self.config.sync_policy {
            SyncPolicy::EveryWrite => {
                self.sync()?;
            }
            SyncPolicy::Interval(d) => {
                if self.last_sync.elapsed() >= d {
                    self.sync()?;
                }
            }
            SyncPolicy::None => {}
        }

        if self.segment_bytes >= self.config.max_segment_size {
            self.rotate()?;
        }

        Ok(())
    }

    /// Force flush + fsync.
    pub fn sync(&mut self) -> Result<(), PerfDbError> {
        self.writer.flush().map_err(|e| PerfDbError::Io {
            context: "WAL flush".into(),
            source: e,
        })?;
        self.writer
            .get_ref()
            .sync_data()
            .map_err(|e| PerfDbError::Io {
                context: "WAL fsync".into(),
                source: e,
            })?;
        self.last_sync = Instant::now();
        Ok(())
    }

    /// Current segment number.
    pub fn current_segment(&self) -> u64 {
        self.current_segment
    }

    /// Total bytes written to current segment.
    pub fn segment_bytes(&self) -> u64 {
        self.segment_bytes
    }

    pub fn position(&self) -> WalPosition {
        WalPosition {
            segment: self.current_segment,
            offset: self.segment_bytes,
        }
    }

    fn rotate(&mut self) -> Result<(), PerfDbError> {
        self.sync()?;

        self.current_segment += 1;
        let seg_path = segment_path(&self.config.dir, self.current_segment);

        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&seg_path)
            .map_err(|e| PerfDbError::Io {
                context: format!("open new WAL segment: {}", seg_path.display()),
                source: e,
            })?;

        file.sync_all()
            .map_err(|e| io_error("sync new WAL segment", e))?;
        sync_directory(&self.config.dir)?;
        self.writer = BufWriter::with_capacity(64 * 1024, file);
        self.segment_bytes = 0;

        tracing::info!(segment = self.current_segment, "WAL segment rotated");

        Ok(())
    }
}

/// Replay the valid prefix. Only a damaged final append may be ignored;
/// corruption preceding another valid record or a later segment is an error.
pub fn replay(
    dir: impl AsRef<Path>,
    callback: impl FnMut(WalEntry) -> Result<(), PerfDbError>,
) -> Result<u64, PerfDbError> {
    scan_wal(dir.as_ref(), callback).map(|(count, _)| count)
}

pub fn replay_positioned(
    dir: impl AsRef<Path>,
    callback: impl FnMut(WalEntry, WalPosition) -> Result<(), PerfDbError>,
) -> Result<u64, PerfDbError> {
    scan_wal_positioned(dir.as_ref(), callback).map(|(count, _)| count)
}

fn decode_record<T: DeserializeOwned>(payload: &[u8]) -> Result<T, bincode::Error> {
    use bincode::Options;
    bincode::DefaultOptions::new()
        .with_fixint_encoding()
        .reject_trailing_bytes()
        .with_limit(MAX_ENTRY_SIZE as u64)
        .deserialize(payload)
}

fn scan_wal<T: DeserializeOwned>(
    dir: &Path,
    mut callback: impl FnMut(T) -> Result<(), PerfDbError>,
) -> Result<(u64, Option<(u64, u64)>), PerfDbError> {
    scan_wal_positioned(dir, |entry, _| callback(entry))
}

fn scan_wal_positioned<T: DeserializeOwned>(
    dir: &Path,
    mut callback: impl FnMut(T, WalPosition) -> Result<(), PerfDbError>,
) -> Result<(u64, Option<(u64, u64)>), PerfDbError> {
    let mut segments = list_segments(dir)?;
    segments.sort_unstable();
    if segments
        .windows(2)
        .any(|w| w[0].checked_add(1) != Some(w[1]))
    {
        return Err(PerfDbError::Corruption(
            "missing interior WAL segment".into(),
        ));
    }
    let mut count = 0;
    for (index, &segment) in segments.iter().enumerate() {
        let path = segment_path(dir, segment);
        let mut file = File::open(&path).map_err(|e| io_error("read WAL segment", e))?;
        let file_len = file
            .metadata()
            .map_err(|e| io_error("WAL metadata", e))?
            .len();
        let mut offset = 0;
        while offset < file_len {
            let mut header = [0u8; ENTRY_HEADER_SIZE];
            let mut frame_end = None;
            let record = if file_len - offset < ENTRY_HEADER_SIZE as u64 {
                None
            } else {
                file.read_exact(&mut header)
                    .map_err(|e| io_error("read WAL header", e))?;
                let len = u32::from_le_bytes(header[..4].try_into().unwrap()) as usize;
                let crc = u32::from_le_bytes(header[4..].try_into().unwrap());
                if len > MAX_ENTRY_SIZE || len as u64 > file_len - offset - ENTRY_HEADER_SIZE as u64
                {
                    None
                } else {
                    frame_end = Some(offset + ENTRY_HEADER_SIZE as u64 + len as u64);
                    let mut payload = vec![0; len];
                    file.read_exact(&mut payload)
                        .map_err(|e| io_error("read WAL payload", e))?;
                    if crc32fast::hash(&payload) == crc {
                        decode_record::<T>(&payload).ok().map(|entry| (entry, len))
                    } else {
                        None
                    }
                }
            };
            if let Some((entry, len)) = record {
                offset += ENTRY_HEADER_SIZE as u64 + len as u64;
                callback(entry, WalPosition { segment, offset })?;
                count += 1;
                continue;
            }
            if index + 1 != segments.len()
                || frame_end.is_some_and(|end| end < file_len)
                || valid_record_after::<T>(&mut file, offset + 1)?
            {
                return Err(PerfDbError::Corruption(format!(
                    "interior WAL corruption at segment {segment}, offset {offset}"
                )));
            }
            return Ok((count, Some((segment, offset))));
        }
    }
    Ok((count, None))
}

fn valid_record_after<T: DeserializeOwned>(
    file: &mut File,
    offset: u64,
) -> Result<bool, PerfDbError> {
    file.seek(SeekFrom::Start(offset))
        .map_err(|e| io_error("seek damaged WAL", e))?;
    let mut tail = Vec::new();
    file.read_to_end(&mut tail)
        .map_err(|e| io_error("read damaged WAL", e))?;
    for start in 0..tail.len().saturating_sub(ENTRY_HEADER_SIZE - 1) {
        let header = &tail[start..start + ENTRY_HEADER_SIZE];
        let len = u32::from_le_bytes(header[..4].try_into().unwrap()) as usize;
        if len == 0 || len > MAX_ENTRY_SIZE || len > tail.len() - start - ENTRY_HEADER_SIZE {
            continue;
        }
        let payload = &tail[start + ENTRY_HEADER_SIZE..start + ENTRY_HEADER_SIZE + len];
        let crc = u32::from_le_bytes(header[4..].try_into().unwrap());
        if crc32fast::hash(payload) == crc && decode_record::<T>(payload).is_ok() {
            return Ok(true);
        }
    }
    Ok(false)
}

pub(crate) fn io_error(context: &str, source: std::io::Error) -> PerfDbError {
    PerfDbError::Io {
        context: context.into(),
        source,
    }
}

pub(crate) fn create_storage_dir(dir: &Path) -> Result<(), PerfDbError> {
    let mut missing = Vec::new();
    let mut cursor = dir;
    while !cursor.exists() && !cursor.as_os_str().is_empty() {
        missing.push(cursor.to_path_buf());
        match cursor.parent() {
            Some(parent) => cursor = parent,
            None => break,
        }
    }
    fs::create_dir_all(dir).map_err(|e| io_error("create storage directory", e))?;
    for path in missing.iter().rev() {
        sync_directory(path)?;
        let parent = path
            .parent()
            .filter(|p| !p.as_os_str().is_empty())
            .unwrap_or_else(|| Path::new("."));
        sync_directory(parent)?;
    }
    Ok(())
}

pub(crate) fn sync_directory(dir: &Path) -> Result<(), PerfDbError> {
    File::open(dir)
        .and_then(|file| file.sync_all())
        .map_err(|e| io_error("sync storage directory", e))
}

/// Versioned, separately namespaced relational WAL. The legacy enum and its
/// discriminants remain readable by existing engine callers.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RelationalWalEntry {
    pub version: u32,
    pub sequence: u64,
    pub mutation: RelationalMutation,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum RelationalMutation {
    PutAlert(crate::relational::alerts::Alert),
    DeleteAlert(u64),
    PutApiKey(crate::relational::api_keys::ApiKey),
    DeleteApiKey(String),
    PutCompetition(crate::relational::competitions::Competition),
    PutCompetitionEntry(crate::relational::competitions::CompetitionEntry),
    SetReferralTiers(Vec<super::snapshot::SnapshotReferralTier>),
}

pub struct RelationalWalWriter(WalWriter);

impl RelationalWalWriter {
    pub fn open(mut config: WalConfig) -> Result<Self, PerfDbError> {
        config.sync_policy = SyncPolicy::EveryWrite;
        Ok(Self(WalWriter::open_for::<RelationalWalEntry>(config)?))
    }

    pub fn append(&mut self, entry: &RelationalWalEntry) -> Result<(), PerfDbError> {
        if entry.version != 1 {
            return Err(PerfDbError::InvalidArgument(
                "unsupported relational WAL version".into(),
            ));
        }
        let payload = bincode::serialize(entry).map_err(|e| PerfDbError::Serialization {
            context: "relational WAL serialize".into(),
            detail: e.to_string(),
        })?;
        self.0.append_payload(&payload)
    }

    pub fn sync(&mut self) -> Result<(), PerfDbError> {
        self.0.sync()
    }
}

pub fn replay_relational(
    dir: &Path,
    mut callback: impl FnMut(RelationalWalEntry) -> Result<(), PerfDbError>,
) -> Result<u64, PerfDbError> {
    scan_wal(dir, |entry: RelationalWalEntry| {
        if entry.version != 1 {
            return Err(PerfDbError::Corruption(
                "unsupported relational WAL version".into(),
            ));
        }
        callback(entry)
    })
    .map(|(count, _)| count)
}

/// A fallback snapshot may be used only with a validated, retained WAL span.
/// Without a precise snapshot cursor, require the original segment zero and a
/// durable checkpoint at or beyond the damaged snapshot's advertised height.
pub fn verify_snapshot_coverage(
    dir: &Path,
    from_block: u64,
    through_block: u64,
) -> Result<(), PerfDbError> {
    let mut segments = list_segments(dir)?;
    segments.sort_unstable();
    if segments.first() != Some(&0) {
        return Err(PerfDbError::Corruption(
            "snapshot fallback lacks retained WAL origin".into(),
        ));
    }
    let mut checkpoint = 0;
    let (_, tail) = scan_wal(dir, |entry: WalEntry| {
        if let WalEntry::Checkpoint { block_number } = entry {
            if block_number < checkpoint {
                return Err(PerfDbError::Corruption("WAL checkpoints regress".into()));
            }
            checkpoint = block_number;
        }
        Ok(())
    })?;
    if tail.is_some() || checkpoint < through_block || from_block > through_block {
        return Err(PerfDbError::Corruption(
            "snapshot fallback lacks verified WAL coverage".into(),
        ));
    }
    Ok(())
}

/// Delete all segments before (not including) the given segment number.
/// Used after a successful checkpoint/snapshot to reclaim disk space.
pub fn truncate_before(dir: impl AsRef<Path>, keep_from: u64) -> Result<u64, PerfDbError> {
    let dir = dir.as_ref();
    let segments = list_segments(dir)?;
    let mut removed = 0u64;

    for seg_id in segments {
        if seg_id < keep_from {
            let path = segment_path(dir, seg_id);
            fs::remove_file(&path).map_err(|e| PerfDbError::Io {
                context: format!("remove WAL segment: {}", path.display()),
                source: e,
            })?;
            removed += 1;
        }
    }

    if removed > 0 {
        sync_directory(dir)?;
        tracing::info!(removed, keep_from, "WAL segments truncated");
    }

    Ok(removed)
}

fn segment_path(dir: &Path, id: u64) -> PathBuf {
    dir.join(format!("wal_{:08}.bin", id))
}

pub(crate) fn list_segments(dir: &Path) -> Result<Vec<u64>, PerfDbError> {
    if !dir.exists() {
        return Ok(Vec::new());
    }

    let mut ids = Vec::new();

    for entry in fs::read_dir(dir).map_err(|e| PerfDbError::Io {
        context: format!("read WAL dir: {}", dir.display()),
        source: e,
    })? {
        let entry = entry.map_err(|e| PerfDbError::Io {
            context: "read WAL dir entry".into(),
            source: e,
        })?;

        let name = entry.file_name();
        let name = name.to_string_lossy();

        if let Some(num_str) = name
            .strip_prefix("wal_")
            .and_then(|s| s.strip_suffix(".bin"))
        {
            if let Ok(id) = num_str.parse::<u64>() {
                ids.push(id);
            }
        }
    }

    Ok(ids)
}

fn find_latest_segment(dir: &Path) -> Result<u64, PerfDbError> {
    let segments = list_segments(dir)?;
    Ok(segments.into_iter().max().unwrap_or(0))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_config(dir: &Path) -> WalConfig {
        WalConfig {
            dir: dir.to_path_buf(),
            max_segment_size: 1024, // small for testing rotation
            sync_policy: SyncPolicy::EveryWrite,
        }
    }

    #[test]
    fn write_and_replay() {
        let dir = tempfile::tempdir().unwrap();
        let config = test_config(dir.path());

        {
            let mut wal = WalWriter::open(config.clone()).unwrap();

            wal.append(&WalEntry::PriceTick {
                timestamp_ns: 1_000_000,
                market_id: 0,
                price: 67_000.0,
            })
            .unwrap();

            wal.append(&WalEntry::Checkpoint { block_number: 42 })
                .unwrap();

            wal.sync().unwrap();
        }

        let mut entries = Vec::new();
        let count = replay(dir.path(), |e| {
            entries.push(e);
            Ok(())
        })
        .unwrap();

        assert_eq!(count, 2);

        match &entries[0] {
            WalEntry::PriceTick {
                market_id, price, ..
            } => {
                assert_eq!(*market_id, 0);
                assert_eq!(*price, 67_000.0);
            }
            other => panic!("unexpected entry: {other:?}"),
        }

        match &entries[1] {
            WalEntry::Checkpoint { block_number } => {
                assert_eq!(*block_number, 42);
            }
            other => panic!("unexpected entry: {other:?}"),
        }
    }

    #[test]
    fn segment_rotation() {
        let dir = tempfile::tempdir().unwrap();
        let config = WalConfig {
            dir: dir.path().to_path_buf(),
            max_segment_size: 200, // very small to force rotation
            sync_policy: SyncPolicy::EveryWrite,
        };

        let mut wal = WalWriter::open(config).unwrap();
        assert_eq!(wal.current_segment(), 0);

        for i in 0..50 {
            wal.append(&WalEntry::PriceTick {
                timestamp_ns: i * 1_000_000,
                market_id: 0,
                price: 60_000.0 + i as f64,
            })
            .unwrap();
        }

        assert!(
            wal.current_segment() > 0,
            "expected rotation, got segment 0"
        );

        let mut count = 0u64;
        replay(dir.path(), |_| {
            count += 1;
            Ok(())
        })
        .unwrap();
        assert_eq!(count, 50);
    }

    #[test]
    fn truncation() {
        let dir = tempfile::tempdir().unwrap();
        let config = WalConfig {
            dir: dir.path().to_path_buf(),
            max_segment_size: 100,
            sync_policy: SyncPolicy::EveryWrite,
        };

        let mut wal = WalWriter::open(config).unwrap();

        for i in 0..100 {
            wal.append(&WalEntry::PriceTick {
                timestamp_ns: i,
                market_id: 0,
                price: 1.0,
            })
            .unwrap();
        }
        wal.sync().unwrap();

        let current = wal.current_segment();
        assert!(current > 2);

        let removed = truncate_before(dir.path(), current).unwrap();
        assert!(removed > 0);

        let remaining = list_segments(dir.path()).unwrap();
        for seg_id in &remaining {
            assert!(*seg_id >= current);
        }
    }

    #[test]
    fn corrupted_entry_stops_replay() {
        let dir = tempfile::tempdir().unwrap();
        let seg_path = segment_path(dir.path(), 0);

        {
            let config = test_config(dir.path());
            let mut wal = WalWriter::open(config).unwrap();
            wal.append(&WalEntry::Checkpoint { block_number: 1 })
                .unwrap();
            wal.sync().unwrap();
        }

        {
            let mut f = OpenOptions::new().append(true).open(&seg_path).unwrap();
            f.write_all(&[0xFF; 50]).unwrap();
        }

        let mut count = 0u64;
        replay(dir.path(), |_| {
            count += 1;
            Ok(())
        })
        .unwrap();
        assert_eq!(count, 1);
    }
}
