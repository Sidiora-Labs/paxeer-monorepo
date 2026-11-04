#![allow(non_snake_case)]

use std::collections::BTreeMap;
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Barrier};

use perfdb::aggregation::candle_builder::{CandleBuilder, CandleOutput};
use perfdb::core::timeframe::Timeframe;
use perfdb::core::types::{CandleRecord, Event, IndexedEvent, PriceTick};
use perfdb::storage::candle_store::CandleStore;
use perfdb::storage::snapshot;
use perfdb::storage::tick_store::TickStore;
use perfdb::storage::wal::{self, SyncPolicy, WalEntry, WalWriter};
use perfdb::{PerfDb, PerfDbConfig};

const USER: [u8; 20] = [1; 20];
const TOKEN: [u8; 20] = [2; 20];

fn config(dir: &Path) -> PerfDbConfig {
    let mut config = PerfDbConfig::new(dir);
    config.wal.sync_policy = SyncPolicy::EveryWrite;
    config.wal.max_segment_size = 256;
    config
}

fn tick(timestamp_ns: u64, market_id: u16, price: f64) -> PriceTick {
    PriceTick {
        timestamp_ns,
        market_id,
        price,
        _pad: [0; 6],
    }
}

fn indexed(block_number: u64, log_index: u32, event: Event) -> IndexedEvent {
    let mut tx_hash = [0; 32];
    tx_hash[..8].copy_from_slice(&block_number.to_le_bytes());
    IndexedEvent {
        block_number,
        block_timestamp: block_number * 12,
        tx_hash,
        tx_index: 0,
        log_index,
        event,
    }
}

fn deposit(block: u64, log: u32, amount: i128) -> IndexedEvent {
    indexed(
        block,
        log,
        Event::CollateralDeposited {
            user: USER,
            token: TOKEN,
            amount,
        },
    )
}

fn position(block: u64, log: u32) -> IndexedEvent {
    indexed(
        block,
        log,
        Event::PositionOpened {
            position_id: [3; 32],
            user: USER,
            market_id: 0,
            is_long: true,
            size_usd: 1_000,
            leverage: 10,
            entry_price: 100,
            collateral_token: TOKEN,
            collateral_amount: 100,
        },
    )
}

fn wal_tick(tick: &PriceTick) -> WalEntry {
    WalEntry::PriceTick {
        timestamp_ns: tick.timestamp_ns,
        market_id: tick.market_id,
        price: tick.price,
    }
}

#[derive(Debug, PartialEq)]
struct History {
    ticks: Vec<Vec<PriceTick>>,
    candles: Vec<Vec<CandleRecord>>,
    live: Vec<Option<CandleRecord>>,
    prices: Vec<f64>,
}

fn history(db: &PerfDb) -> History {
    History {
        ticks: (0..2).map(|mid| db.ticks(mid)).collect(),
        candles: (0..2)
            .flat_map(|mid| {
                Timeframe::ALL
                    .into_iter()
                    .map(move |tf| db.candles(mid, tf))
            })
            .collect(),
        live: (0..2)
            .flat_map(|mid| {
                Timeframe::ALL
                    .into_iter()
                    .map(move |tf| db.live_candle(mid, tf))
            })
            .collect(),
        prices: (0..2).map(|mid| db.latest_price(mid)).collect(),
    }
}

fn fingerprint(dir: &Path) -> BTreeMap<PathBuf, (u64, u32)> {
    fn visit(root: &Path, dir: &Path, out: &mut BTreeMap<PathBuf, (u64, u32)>) {
        for entry in fs::read_dir(dir).unwrap() {
            let path = entry.unwrap().path();
            if path.is_dir() {
                visit(root, &path, out);
                continue;
            }
            let mut file = File::open(&path).unwrap();
            let mut crc = crc32fast::Hasher::new();
            let mut bytes = [0; 8192];
            loop {
                let n = file.read(&mut bytes).unwrap();
                if n == 0 {
                    break;
                }
                crc.update(&bytes[..n]);
            }
            out.insert(
                path.strip_prefix(root).unwrap().to_path_buf(),
                (file.metadata().unwrap().len(), crc.finalize()),
            );
        }
    }
    let mut out = BTreeMap::new();
    visit(dir, dir, &mut out);
    out
}

#[test]
fn TestRepair_140_tick_replay_history_crash_boundaries() {
    let feed = [
        tick(1_100_000_000, 0, 100.0),
        tick(1_100_000_000, 0, 100.0), // Equal records are distinct WAL occurrences.
        tick(1_900_000_000, 0, 80.0),
        tick(1_950_000_000, 1, 30.0),
        tick(2_100_000_000, 0, 120.0),
        tick(61_100_000_000, 0, 115.0),
    ];
    let reference_dir = tempfile::tempdir().unwrap();
    let reference = PerfDb::open(config(reference_dir.path())).unwrap();
    for value in &feed {
        reference.ingest_tick(value).unwrap();
    }
    let expected = history(&reference);
    assert_eq!(expected.ticks[0].len(), 5);
    assert_eq!(
        reference.live_candle(0, Timeframe::Min1).unwrap().open,
        115.0
    );

    // Durable WAL before mmap append; mmap before aggregation; aggregation
    // before closed-candle persistence; both stores before cursor publication.
    for boundary in 0..4 {
        let dir = tempfile::tempdir().unwrap();
        let cfg = config(dir.path());
        {
            let db = PerfDb::open(cfg.clone()).unwrap();
            for value in &feed[..2] {
                db.ingest_tick(value).unwrap();
            }
            db.sync_wal().unwrap();
        }
        {
            let mut wal = WalWriter::open(cfg.wal.clone()).unwrap();
            let mut ticks = TickStore::open(dir.path().join("ticks")).unwrap();
            let mut candles = CandleStore::open(dir.path().join("candles")).unwrap();
            let mut builder = CandleBuilder::new();
            for value in &feed[..2] {
                builder.on_tick(value);
            }
            for value in &feed[2..] {
                wal.append(&wal_tick(value)).unwrap();
                if boundary >= 1 {
                    ticks.append(value).unwrap();
                }
                if boundary >= 2 {
                    for output in builder.on_tick(value) {
                        if boundary >= 3 {
                            if let CandleOutput::Closed(candle) = output {
                                candles.upsert(&candle).unwrap();
                            }
                        }
                    }
                }
            }
            ticks.flush().unwrap();
            candles.flush().unwrap();
        }
        for reopen in 0..2 {
            let recovered = PerfDb::open(cfg.clone()).unwrap();
            assert_eq!(
                history(&recovered),
                expected,
                "boundary {boundary}, reopen {reopen}"
            );
        }
    }
}

#[test]
fn TestRepair_140_tick_live_bucket_and_shutdown_cursor() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let expected;
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        db.ingest_tick(&tick(1_100_000_000, 0, 100.0)).unwrap();
        db.ingest_tick(&tick(1_900_000_000, 0, 80.0)).unwrap();
        db.sync_wal().unwrap();
        expected = history(&db);
    }
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        assert_eq!(history(&db), expected);
        assert_eq!(db.live_candle(0, Timeframe::Sec1).unwrap().high, 100.0);
        db.shutdown().unwrap();
    }
    // A shutdown stored a live candle; a later crash closes/overwrites it
    // before the next cursor is published. Recovery must not duplicate it.
    let after_crash;
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        db.ingest_tick(&tick(1_950_000_000, 0, 70.0)).unwrap();
        db.ingest_tick(&tick(2_100_000_000, 0, 110.0)).unwrap();
        after_crash = history(&db);
    }
    for _ in 0..2 {
        let db = PerfDb::open(cfg.clone()).unwrap();
        assert_eq!(history(&db), after_crash);
        let closed = db.candles(0, Timeframe::Sec1);
        assert_eq!(closed.len(), 1);
        assert_eq!(
            (
                closed[0].open,
                closed[0].high,
                closed[0].low,
                closed[0].close
            ),
            (100.0, 100.0, 70.0, 70.0)
        );
        assert_eq!(db.live_candle(0, Timeframe::Sec1).unwrap().open, 110.0);
        assert_eq!(db.tick_count(0), 4);
    }
}

#[test]
fn TestRepair_140_snapshot_same_block_exact_event_boundary() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let before = deposit(42, 0, 10);
    let after = deposit(42, 1, 20);
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        db.process_event(&before).unwrap();
        db.ingest_tick(&tick(1_100_000_000, 0, 100.0)).unwrap();
        db.snapshot().unwrap();
        let (_, boundary) = snapshot::load_latest_engine_snapshot(&cfg.snapshots_dir())
            .unwrap()
            .unwrap();
        assert_eq!(boundary.events.len(), 1);
        assert!(boundary.position.offset > 0 || boundary.position.segment > 0);
        db.process_event(&after).unwrap();
        // A late event below the checkpoint must also follow its WAL cursor.
        db.process_event(&deposit(41, 2, 5)).unwrap();
        db.ingest_tick(&tick(1_900_000_000, 0, 120.0)).unwrap();
        db.sync_wal().unwrap();
    }
    for _ in 0..2 {
        let db = PerfDb::open(cfg.clone()).unwrap();
        assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 35);
        assert_eq!(db.events.total_count(), 3);
        assert_eq!(db.checkpoint.last_block(), 42);
        assert_eq!(db.latest_price(0), 120.0);
        assert_eq!(db.tick_count(0), 2);
        let live = db.live_candle(0, Timeframe::Sec1).unwrap();
        assert_eq!((live.open, live.close), (100.0, 120.0));
        db.process_event(&before).unwrap();
        db.process_event(&after).unwrap();
        assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 35);
    }
}

#[test]
fn TestRepair_140_snapshot_concurrent_mutation_boundary() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = Arc::new(PerfDb::open(cfg.clone()).unwrap());
    let rounds = 24u32;
    let barrier = Arc::new(Barrier::new(2));
    std::thread::scope(|scope| {
        let writer = Arc::clone(&db);
        let writer_barrier = Arc::clone(&barrier);
        let producer = scope.spawn(move || {
            let mut results = Vec::new();
            for log in 0..rounds {
                writer_barrier.wait();
                results.push(writer.process_event(&deposit(77, log, 1)).and_then(|_| {
                    writer
                        .ingest_tick(&tick(1_000_000_000 + log as u64, 0, 100.0 + log as f64))
                        .map(|_| ())
                }));
                writer_barrier.wait();
            }
            results
        });
        let mut samples = Vec::new();
        for _ in 0..rounds {
            barrier.wait();
            samples.push(
                db.snapshot()
                    .and_then(|_| snapshot::load_latest_engine_snapshot(&cfg.snapshots_dir())),
            );
            barrier.wait();
        }
        for result in producer.join().unwrap() {
            result.unwrap();
        }
        for sample in samples {
            let (state, boundary) = sample.unwrap().unwrap();
            let balance = state
                .balances
                .iter()
                .find(|balance| balance.user_address == USER)
                .map_or(0, |balance| balance.amount);
            assert_eq!(balance, boundary.events.len() as i128);
        }
    });
    db.process_event(&deposit(77, rounds, 1)).unwrap();
    db.sync_wal().unwrap();
    drop(db);
    let recovered = PerfDb::open(cfg).unwrap();
    assert_eq!(
        recovered.balances.get(&USER, &TOKEN).unwrap().amount,
        (rounds + 1) as i128
    );
    assert_eq!(recovered.events.total_count(), (rounds + 1) as u64);
    assert_eq!(recovered.tick_count(0), rounds as u64);
    assert_eq!(
        recovered.live_candle(0, Timeframe::Sec1).unwrap().close,
        100.0 + (rounds - 1) as f64
    );
}

#[test]
fn TestRepair_140_event_redelivery_dedup_before_and_after_reopen() {
    for use_snapshot in [false, true] {
        let dir = tempfile::tempdir().unwrap();
        let cfg = config(dir.path());
        let deposit = deposit(0, 0, 100);
        let position = position(0, 1);
        for reopen in 0..3 {
            let db = PerfDb::open(cfg.clone()).unwrap();
            for _ in 0..3 {
                db.process_event(&deposit).unwrap();
                db.process_event(&position).unwrap();
            }
            assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 100);
            assert_eq!(db.positions.count(), 1);
            assert_eq!(db.trader_stats.get(&USER).unwrap().open_positions, 1);
            assert_eq!(db.market_state.open_interest(0), (1_000, 0));
            assert_eq!(db.events.total_count(), 2);
            db.sync_wal().unwrap();
            assert_eq!(wal::replay(&cfg.wal.dir, |_| Ok(())).unwrap(), 2);
            if use_snapshot && reopen == 0 {
                db.snapshot().unwrap();
            }
        }
    }
}

#[test]
fn TestRepair_140_event_conflicting_redelivery_refused_before_wal() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let accepted = deposit(42, 0, 100);
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        db.process_event(&accepted).unwrap();
        db.snapshot().unwrap();
    }
    let db = PerfDb::open(cfg.clone()).unwrap();
    let before = fingerprint(dir.path());
    let mut changed_block = accepted.clone();
    changed_block.block_number += 1;
    let mut changed_tx_index = accepted.clone();
    changed_tx_index.tx_index += 1;
    let mut changed_payload = accepted.clone();
    changed_payload.event = Event::CollateralDeposited {
        user: USER,
        token: TOKEN,
        amount: 200,
    };
    let mut changed_slot = accepted.clone();
    changed_slot.tx_hash[31] = 1;
    for conflicting in [
        changed_block,
        changed_tx_index,
        changed_payload,
        changed_slot,
    ] {
        assert!(db.process_event(&conflicting).is_err());
        assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 100);
        assert_eq!(db.events.total_count(), 1);
    }
    assert_eq!(fingerprint(dir.path()), before);
    assert_eq!(wal::replay(&cfg.wal.dir, |_| Ok(())).unwrap(), 1);
}

#[test]
fn TestRepair_140_event_wal_redelivery_is_idempotent() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    {
        let mut producer = WalWriter::open(cfg.wal.clone()).unwrap();
        for _ in 0..3 {
            producer
                .append(&WalEntry::IndexedEvent(deposit(10, 0, 100)))
                .unwrap();
            producer
                .append(&WalEntry::IndexedEvent(position(10, 1)))
                .unwrap();
        }
    }
    for _ in 0..2 {
        let db = PerfDb::open(cfg.clone()).unwrap();
        assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 100);
        assert_eq!(db.trader_stats.get(&USER).unwrap().open_positions, 1);
        assert_eq!(db.market_state.open_interest(0), (1_000, 0));
        assert_eq!(db.events.total_count(), 2);
        db.snapshot().unwrap();
    }
}

#[test]
fn TestRepair_140_snapshot_cursor_and_dedup_survive_wal_pruning() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let expected;
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        for log in 0..8 {
            db.process_event(&deposit(42, log, 10)).unwrap();
            db.ingest_tick(&tick(1_100_000_000 + log as u64, 0, 100.0 + log as f64))
                .unwrap();
        }
        db.snapshot().unwrap();
        expected = history(&db);
    }
    let (_, boundary) = snapshot::load_latest_engine_snapshot(&cfg.snapshots_dir())
        .unwrap()
        .unwrap();
    assert!(boundary.position.segment > 0);
    assert!(wal::truncate_before(&cfg.wal.dir, boundary.position.segment).unwrap() > 0);
    {
        let db = PerfDb::open(cfg.clone()).unwrap();
        assert_eq!(history(&db), expected);
        db.process_event(&deposit(42, 0, 10)).unwrap();
        assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 80);
        db.process_event(&deposit(42, 8, 10)).unwrap();
    }
    let db = PerfDb::open(cfg.clone()).unwrap();
    assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, 90);
    assert_eq!(db.events.total_count(), 9);
    assert_eq!(history(&PerfDb::open_read_only(cfg).unwrap()), expected);
}

#[test]
fn TestRepair_140_database_writer_lease_and_live_inspection() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.ingest_tick(&tick(1_100_000_000, 0, 100.0)).unwrap();
    db.process_event(&deposit(42, 0, 100)).unwrap();
    db.sync_wal().unwrap();
    let before = fingerprint(dir.path());
    let competing = cfg.clone();
    assert!(
        std::thread::spawn(move || PerfDb::open(competing).is_err())
            .join()
            .unwrap()
    );
    assert_eq!(fingerprint(dir.path()), before);
    let inspection = PerfDb::open_read_only(cfg.clone()).unwrap();
    assert_eq!(history(&inspection), history(&db));
    assert_eq!(inspection.balances.get(&USER, &TOKEN).unwrap().amount, 100);
    assert!(inspection.ingest_tick(&tick(9, 0, 9.0)).is_err());
    assert!(inspection.process_event(&deposit(42, 1, 9)).is_err());
    assert!(inspection.snapshot().is_err());
    assert!(inspection.sync_wal().is_err());
    assert!(inspection.shutdown().is_err());
    assert_eq!(fingerprint(dir.path()), before);
    db.ingest_tick(&tick(1_900_000_000, 0, 120.0)).unwrap();
    db.sync_wal().unwrap();
    assert_eq!(inspection.tick_count(0), 1);
    assert_eq!(
        PerfDb::open_read_only(cfg.clone()).unwrap().tick_count(0),
        2
    );
    db.shutdown().unwrap();
    assert!(
        PerfDb::open(cfg.clone()).is_err(),
        "lease lasts through drop, including after shutdown"
    );
    drop(db);
    let reopened = PerfDb::open(cfg).unwrap();
    assert_eq!(reopened.tick_count(0), 2);
    assert_eq!(reopened.latest_price(0), 120.0);
}

#[cfg(unix)]
#[test]
fn TestRepair_140_database_writer_lease_directory_alias() {
    let dir = tempfile::tempdir().unwrap();
    let data = dir.path().join("data");
    let db = PerfDb::open(config(&data)).unwrap();
    let alias = dir.path().join("alias");
    std::os::unix::fs::symlink(&data, &alias).unwrap();
    assert!(PerfDb::open(config(&alias)).is_err());
    drop(db);
    assert!(PerfDb::open(config(&alias)).is_ok());
}

#[test]
fn TestRepair_140_inspection_ignores_uncommitted_mmap_suffix() {
    let dir = tempfile::tempdir().unwrap();
    let mut cfg = config(dir.path());
    cfg.wal.sync_policy = SyncPolicy::None;
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.ingest_tick(&tick(1_100_000_000, 0, 100.0)).unwrap();
    db.sync_wal().unwrap();
    db.ingest_tick(&tick(1_900_000_000, 0, 120.0)).unwrap();
    let before = fingerprint(dir.path());
    let inspection = PerfDb::open_read_only(cfg).unwrap();
    assert_eq!(inspection.tick_count(0), 1);
    assert_eq!(inspection.latest_price(0), 100.0);
    assert_eq!(
        inspection.live_candle(0, Timeframe::Sec1).unwrap().close,
        100.0
    );
    assert_eq!(fingerprint(dir.path()), before);
    db.sync_wal().unwrap();
}

#[test]
fn TestRepair_140_inspection_does_not_repair_wal_or_create_database() {
    let dir = tempfile::tempdir().unwrap();
    let missing = dir.path().join("missing");
    assert!(PerfDb::open_read_only(config(&missing)).is_err());
    assert!(!missing.exists());
    let cfg = config(dir.path());
    let segment;
    {
        let mut producer = WalWriter::open(cfg.wal.clone()).unwrap();
        producer
            .append(&wal_tick(&tick(1_100_000_000, 0, 100.0)))
            .unwrap();
        producer.sync().unwrap();
        segment = producer.current_segment();
    }
    let path = cfg.wal.dir.join(format!("wal_{segment:08}.bin"));
    let mut file = OpenOptions::new().append(true).open(&path).unwrap();
    file.write_all(&[1, 2, 3]).unwrap();
    file.sync_all().unwrap();
    let before = fingerprint(dir.path());
    let db = PerfDb::open_read_only(cfg).unwrap();
    assert_eq!(db.tick_count(0), 1);
    assert_eq!(db.latest_price(0), 100.0);
    assert_eq!(fingerprint(dir.path()), before);
    assert!(!dir.path().join(".writer.lock").exists());
}
