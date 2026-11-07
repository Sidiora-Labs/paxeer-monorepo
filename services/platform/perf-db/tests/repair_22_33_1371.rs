use std::fs::{self, OpenOptions};
use std::io::{Seek, SeekFrom, Write};
use std::path::Path;

use perfdb::core::timeframe::Timeframe;
use perfdb::core::types::{CandleRecord, PriceTick};
use perfdb::query::time_range::{query_tick_range, query_tick_range_inclusive};
use perfdb::storage::candle_store::CandleStore;
use perfdb::storage::mmap::MmapFile;
use perfdb::storage::tick_store::TickStore;
use perfdb::storage::wal::{self, SyncPolicy, WalConfig, WalEntry, WalWriter};
use perfdb::{PerfDb, PerfDbConfig, PerfDbError};

#[derive(Clone, Copy, bytemuck::Pod, bytemuck::Zeroable)]
#[repr(C, align(16))]
struct AlignedRecord([u8; 16]);

fn tick(timestamp_ns: u64, market_id: u16) -> PriceTick {
    PriceTick {
        timestamp_ns,
        market_id,
        _pad: [0; 6],
        price: timestamp_ns as f64 + 100.0,
    }
}

fn overwrite_header(path: &Path, len: u64) {
    let mut file = OpenOptions::new().write(true).open(path).unwrap();
    file.seek(SeekFrom::Start(0)).unwrap();
    file.write_all(&len.to_le_bytes()).unwrap();
    file.sync_all().unwrap();
}

fn assert_corruption<T>(result: Result<T, PerfDbError>) {
    assert!(
        matches!(result, Err(PerfDbError::Corruption(_))),
        "expected typed corruption refusal"
    );
}

#[test]
fn mmap_header_validation_short_existing_files_are_refused_without_mutation() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("short.tick");
    for len in 0..8 {
        let original = vec![0xab; len];
        fs::write(&path, &original).unwrap();
        for create in [false, true] {
            assert_corruption(MmapFile::open(&path, create));
            assert_eq!(fs::read(&path).unwrap(), original);
        }
    }
    let missing = dir.path().join("missing.tick");
    assert!(matches!(
        MmapFile::open(&missing, false),
        Err(PerfDbError::Io { .. })
    ));
    assert!(!missing.exists());
    let empty = dir.path().join("header-only.tick");
    fs::write(&empty, 0u64.to_le_bytes()).unwrap();
    let mut file = MmapFile::open_records::<PriceTick>(&empty, false).unwrap();
    file.append_record(&tick(10, 0)).unwrap();
    file.flush().unwrap();
    drop(file);
    assert_eq!(
        MmapFile::open_records::<PriceTick>(&empty, false)
            .unwrap()
            .as_slice::<PriceTick>()
            .unwrap(),
        &[tick(10, 0)]
    );
}

#[test]
fn mmap_header_validation_capacity_and_partial_records_refuse_reopen() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("ticks");
    let mut store = TickStore::open(&path).unwrap();
    store.append(&tick(10, 0)).unwrap();
    store.flush().unwrap();
    drop(store);
    let file = path.join("market_0000.tick");
    let original = fs::read(&file).unwrap();
    let capacity = original.len() as u64 - 8;
    for len in [
        capacity + 1,
        u64::MAX,
        1,
        size_of::<PriceTick>() as u64 - 1,
        size_of::<PriceTick>() as u64 + 1,
    ] {
        overwrite_header(&file, len);
        let damaged = fs::read(&file).unwrap();
        assert_corruption(TickStore::open(&path));
        assert_eq!(
            fs::read(&file).unwrap(),
            damaged,
            "opening must not reconstruct unverified data"
        );
    }
    fs::write(&file, original).unwrap();
    assert_eq!(TickStore::open(&path).unwrap().ticks(0), &[tick(10, 0)]);
}

#[test]
fn mmap_header_validation_typed_access_and_overflow_are_checked() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("records");
    let mut file = MmapFile::open(&path, true).unwrap();
    file.append_record(&tick(10, 0)).unwrap();
    assert!(file.read_at(u64::MAX, 2).is_none());
    assert!(file.read_at(1, u64::MAX).is_none());
    assert!(file.write_at(u64::MAX, &[1, 2]).is_err());
    assert!(file.read_record::<PriceTick>(1).is_none());
    assert!(file.read_record::<()>(0).is_none());
    assert!(matches!(
        file.as_slice::<()>(),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert!(file.iter_records::<()>().is_err());
    assert!(file.record_count::<()>().is_err());
    // The eight-byte header cannot align a 16-byte-aligned record.
    let aligned_path = dir.path().join("aligned");
    let mut aligned = MmapFile::open(&aligned_path, true).unwrap();
    aligned.append(&[0; 16]).unwrap();
    assert_corruption(aligned.as_slice::<AlignedRecord>());
    assert!(aligned.read_record::<AlignedRecord>(0).is_none());
    let before = file.len();
    assert!(file.append_record(&AlignedRecord([0; 16])).is_err());
    assert_eq!(file.len(), before);
    file.append(&[1]).unwrap();
    assert_corruption(file.as_slice::<PriceTick>());
    assert_corruption(file.record_count::<PriceTick>());
    assert_corruption(file.iter_records::<PriceTick>());
    assert!(file.append_record(&tick(20, 0)).is_err());
    file.flush().unwrap();
    drop(file);
    assert_corruption(MmapFile::open_records::<PriceTick>(&path, false));
}

#[test]
fn mmap_header_validation_candle_store_checks_typed_layout() {
    let dir = tempfile::tempdir().unwrap();
    let mut store = CandleStore::open(dir.path()).unwrap();
    let candle = CandleRecord {
        timestamp_ns: 10,
        open: 1.0,
        high: 3.0,
        low: 1.0,
        close: 2.0,
        volume: 5.0,
        market_id: 0,
        timeframe: Timeframe::Sec1 as u8,
        _pad: 0,
    };
    store.append(&candle).unwrap();
    store.flush().unwrap();
    drop(store);
    let path = dir
        .path()
        .join(format!("m0000_tf{:02}.candle", Timeframe::Sec1 as u8));
    overwrite_header(&path, size_of::<CandleRecord>() as u64 + 1);
    assert_corruption(CandleStore::open(dir.path()));
    overwrite_header(&path, size_of::<CandleRecord>() as u64);
    assert_eq!(
        CandleStore::open(dir.path())
            .unwrap()
            .candles(0, Timeframe::Sec1),
        &[candle]
    );
}

#[test]
fn mmap_header_validation_engine_reopen_refuses_unverified_reconstruction() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = PerfDbConfig::new(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.ingest_tick(&tick(10, 0)).unwrap();
    db.shutdown().unwrap();
    drop(db);
    let path = dir.path().join("ticks/market_0000.tick");
    let original = fs::read(&path).unwrap();
    overwrite_header(&path, u64::MAX);
    let damaged = fs::read(&path).unwrap();
    assert_corruption(PerfDb::open(cfg.clone()));
    assert_corruption(PerfDb::open_read_only(cfg.clone()));
    assert_eq!(fs::read(&path).unwrap(), damaged);
    fs::write(&path, original).unwrap();
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.ticks(0), vec![tick(10, 0)]);
    db.shutdown().unwrap();
}

fn wal_config(path: &Path) -> WalConfig {
    WalConfig {
        dir: path.to_path_buf(),
        max_segment_size: 1024 * 1024,
        sync_policy: SyncPolicy::EveryWrite,
    }
}

#[test]
fn tick_chronological_admission_precedes_wal_and_survives_reopen() {
    let dir = tempfile::tempdir().unwrap();
    let ticks = dir.path().join("ticks");
    let logs = dir.path().join("wal");
    let mut store = TickStore::open(&ticks).unwrap();
    let mut writer = WalWriter::open(wal_config(&logs)).unwrap();
    let accepted = [tick(10, 0), tick(20, 0), tick(20, 0), tick(5, 1)];
    for value in &accepted {
        store.append_with_wal(value, &mut writer).unwrap();
    }
    let position = writer.position();
    assert!(matches!(
        store.append_with_wal(&tick(19, 0), &mut writer),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert!(matches!(
        store.append_with_wal(&tick(30, u16::MAX), &mut writer),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert_eq!(writer.position(), position);
    assert_eq!(store.ticks(0), &accepted[..3]);
    assert_eq!(query_tick_range(store.ticks(0), 10, 20), &accepted[..1]);
    assert_eq!(
        query_tick_range_inclusive(store.ticks(0), 20, 20),
        &accepted[1..3]
    );
    store.flush().unwrap();
    drop(writer);
    drop(store);
    let mut store = TickStore::open(&ticks).unwrap();
    let mut writer = WalWriter::open(wal_config(&logs)).unwrap();
    let position = writer.position();
    assert!(matches!(
        store.append_with_wal(&tick(19, 0), &mut writer),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert!(matches!(
        store.append(&tick(4, 1)),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert_eq!(writer.position(), position);
    store.append_with_wal(&tick(30, 0), &mut writer).unwrap();
    store.flush().unwrap();
    drop(writer);
    drop(store);
    let mut recovered = TickStore::open(dir.path().join("replayed")).unwrap();
    assert_eq!(
        wal::replay(&logs, |entry| {
            match entry {
                WalEntry::PriceTick {
                    timestamp_ns,
                    market_id,
                    price,
                } => recovered.append(&PriceTick {
                    timestamp_ns,
                    market_id,
                    price,
                    _pad: [0; 6],
                })?,
                other => panic!("unexpected entry: {other:?}"),
            }
            Ok(())
        })
        .unwrap(),
        5
    );
    recovered.flush().unwrap();
    drop(recovered);
    let recovered = TickStore::open(dir.path().join("replayed")).unwrap();
    let original = TickStore::open(&ticks).unwrap();
    assert_eq!(recovered.ticks(0), original.ticks(0));
    assert_eq!(recovered.ticks(1), original.ticks(1));
    assert_eq!(original.latest(0), Some(&tick(30, 0)));
}

#[test]
fn tick_chronological_admission_reopen_rejects_unsorted_history() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("market_0000.tick");
    let mut file = MmapFile::open(&path, true).unwrap();
    file.append_record(&tick(20, 0)).unwrap();
    file.append_record(&tick(10, 0)).unwrap();
    file.flush().unwrap();
    drop(file);
    let original = fs::read(&path).unwrap();
    assert_corruption(TickStore::open(dir.path()));
    assert_eq!(fs::read(&path).unwrap(), original);
}

#[test]
fn tick_chronological_admission_engine_sorted_path_recovers_duplicates() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = PerfDbConfig::new(dir.path());
    let accepted = [tick(10, 0), tick(20, 0), tick(20, 0)];
    let db = PerfDb::open(cfg.clone()).unwrap();
    for value in &accepted {
        db.ingest_tick(value).unwrap();
    }
    db.shutdown().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.ticks(0), accepted);
    assert_eq!(
        query_tick_range_inclusive(&db.ticks(0), 20, 20),
        &accepted[1..]
    );
    db.shutdown().unwrap();
}
