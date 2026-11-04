//! Repair record 22.33.1383 (audit 22.30.75, acceptance 22998.1).
//! perf-architecture-drift: perf_architecture_storage_layouts,
//! perf_architecture_all_timeframes, perf_architecture_wal_and_inspection,
//! perf_architecture_cli_and_modules.
//! perf-readme-drift: perf_readme_toolchain_and_quick_start.
//! Baseline verification was not run: the explicit ordered gate instructions require
//! complete implementation followed by hf-build/hf-done and forbid a manual verify run.
//! No tests are claimed to have failed before the change. No out-of-path wiring is required.
//! Verified revision, command, exit and private log are recorded by the hf-done ledger.

use std::fs::File;
use std::io::Read;
use std::mem::{align_of, offset_of, size_of, size_of_val};
use std::path::Path;
use std::time::Duration;

use perfdb::core::market::market_by_symbol;
use perfdb::core::timeframe::Timeframe;
use perfdb::core::types::{CandleRecord, PriceTick};
use perfdb::storage::candle_store::CandleStore;
use perfdb::storage::tick_store::TickStore;
use perfdb::storage::wal::{self, SyncPolicy, WalConfig, WalEntry, WalWriter};
use perfdb::{PerfDb, PerfDbConfig};

const ARCHITECTURE: &str = include_str!("../ARCHITECTURE.md");
const README: &str = include_str!("../README.md");

fn tick(timestamp_ns: u64, price: f64) -> PriceTick {
    PriceTick {
        timestamp_ns,
        market_id: 0,
        _pad: [0; 6],
        price,
    }
}

fn payload(path: &Path, record_size: usize) -> Vec<u8> {
    let mut file = File::open(path).unwrap();
    let mut header = [0; 8];
    file.read_exact(&mut header).unwrap();
    assert_eq!(u64::from_le_bytes(header), record_size as u64);
    let mut bytes = vec![0; record_size];
    file.read_exact(&mut bytes).unwrap();
    bytes
}

#[test]
fn perf_architecture_storage_layouts() {
    assert_eq!((size_of::<PriceTick>(), align_of::<PriceTick>()), (24, 8));
    assert_eq!(
        (size_of::<CandleRecord>(), align_of::<CandleRecord>()),
        (48, 8)
    );
    let tick = PriceTick {
        market_id: 1,
        ..tick(123_456_789, 43_250.50)
    };
    let candle = CandleRecord {
        timestamp_ns: 0,
        open: 100.0,
        high: 120.0,
        low: 95.0,
        close: 110.0,
        volume: 123.5,
        market_id: 1,
        timeframe: Timeframe::Week1 as u8,
        _pad: 0,
    };
    macro_rules! documented_field {
        ($record:ty, $value:ident, $field:ident) => {
            assert!(
                ARCHITECTURE.contains(&format!(
                    "| {} | {} | {} | {} | {} |",
                    stringify!($record),
                    stringify!($field),
                    std::any::type_name_of_val(&$value.$field),
                    offset_of!($record, $field),
                    size_of_val(&$value.$field),
                )),
                "undocumented layout: {}.{}",
                stringify!($record),
                stringify!($field)
            );
        };
    }
    documented_field!(PriceTick, tick, timestamp_ns);
    documented_field!(PriceTick, tick, market_id);
    documented_field!(PriceTick, tick, _pad);
    documented_field!(PriceTick, tick, price);
    documented_field!(CandleRecord, candle, timestamp_ns);
    documented_field!(CandleRecord, candle, open);
    documented_field!(CandleRecord, candle, high);
    documented_field!(CandleRecord, candle, low);
    documented_field!(CandleRecord, candle, close);
    documented_field!(CandleRecord, candle, volume);
    documented_field!(CandleRecord, candle, market_id);
    documented_field!(CandleRecord, candle, timeframe);
    documented_field!(CandleRecord, candle, _pad);

    let dir = tempfile::tempdir().unwrap();
    let ticks_dir = dir.path().join("ticks");
    let candles_dir = dir.path().join("candles");
    let mut ticks = TickStore::open(&ticks_dir).unwrap();
    let mut candles = CandleStore::open(&candles_dir).unwrap();
    ticks.append(&tick).unwrap();
    candles.append(&candle).unwrap();
    ticks.flush().unwrap();
    candles.flush().unwrap();

    let stored_tick = payload(&ticks_dir.join("market_0001.tick"), 24);
    assert_eq!(&stored_tick[0..8], tick.timestamp_ns.to_ne_bytes());
    assert_eq!(&stored_tick[8..10], tick.market_id.to_ne_bytes());
    assert_eq!(&stored_tick[10..16], tick._pad);
    assert_eq!(&stored_tick[16..24], tick.price.to_ne_bytes());
    let stored_candle = payload(&candles_dir.join("m0001_tf15.candle"), 48);
    assert_eq!(&stored_candle[..8], candle.timestamp_ns.to_ne_bytes());
    for (index, value) in [candle.open, candle.high, candle.low, candle.close]
        .iter()
        .enumerate()
    {
        assert_eq!(
            &stored_candle[8 + index * 8..16 + index * 8],
            value.to_ne_bytes()
        );
    }
    assert_eq!(&stored_candle[40..44], candle.volume.to_ne_bytes());
    assert_eq!(&stored_candle[44..46], candle.market_id.to_ne_bytes());
    assert_eq!(&stored_candle[46..48], [candle.timeframe, candle._pad]);
    drop((ticks, candles));
    let ticks = TickStore::open(&ticks_dir).unwrap();
    let candles = CandleStore::open(&candles_dir).unwrap();
    assert_eq!(ticks.ticks(1), &[tick]);
    assert_eq!(candles.candles(1, Timeframe::Week1), &[candle]);
}

#[test]
fn perf_architecture_all_timeframes() {
    assert_eq!(Timeframe::ALL.len(), 16);
    let dir = tempfile::tempdir().unwrap();
    let config = PerfDbConfig::new(dir.path());
    let db = PerfDb::open(config.clone()).unwrap();
    let first = tick(0, 100.0);
    let second = tick(Timeframe::Week1.duration_secs() * 1_000_000_000, 120.0);
    db.ingest_tick(&first).unwrap();
    db.ingest_tick(&second).unwrap();
    for tf in Timeframe::ALL {
        assert!(ARCHITECTURE.contains(&format!(
            "| {} | {:?} | {} | {} |",
            tf as u8,
            tf,
            tf.as_str(),
            tf.duration_secs()
        )));
        assert_eq!(Timeframe::from_str(tf.as_str()), Some(tf));
        let closed = db.candles(0, tf);
        assert_eq!(closed.len(), 1);
        assert_eq!(closed[0].timeframe, tf as u8);
        assert_eq!(
            (closed[0].timestamp_ns, closed[0].open, closed[0].close),
            (0, 100.0, 100.0)
        );
        assert_eq!(closed[0].volume, 0.0);
        let live = db.live_candle(0, tf).unwrap();
        assert_eq!(live.timestamp_ns, tf.bucket_start_ns(second.timestamp_ns));
        assert_eq!((live.open, live.close), (120.0, 120.0));
    }
    db.shutdown().unwrap();
    let expected: Vec<_> = Timeframe::ALL.iter().map(|&tf| db.candles(0, tf)).collect();
    drop(db);
    let reopened = PerfDb::open(config).unwrap();
    assert_eq!(reopened.ticks(0), vec![first, second]);
    for (tf, candles) in Timeframe::ALL.into_iter().zip(expected) {
        assert_eq!(candles.len(), 2);
        assert_eq!(reopened.candles(0, tf), candles);
        assert_eq!(reopened.live_candle(0, tf).unwrap().close, 120.0);
    }
    reopened.shutdown().unwrap();
}

#[test]
fn perf_architecture_wal_and_inspection() {
    let defaults = WalConfig::default();
    assert_eq!(defaults.max_segment_size, 64 * 1024 * 1024);
    assert!(
        matches!(defaults.sync_policy, SyncPolicy::Interval(d) if d == Duration::from_millis(10))
    );
    for claim in [
        "BufWriter<File>",
        "64 KiB",
        "64 MiB",
        "10 ms",
        "[len: u32 LE][crc32: u32 LE][payload: len bytes]",
        "SyncPolicy::EveryWrite",
        "SyncPolicy::Interval(Duration)",
        "SyncPolicy::None",
        "lost on a crash before sync",
        "open_read_only",
        "exclusive writer lease",
    ] {
        assert!(
            ARCHITECTURE.contains(claim),
            "missing WAL/lifecycle contract: {claim}"
        );
    }
    let dir = tempfile::tempdir().unwrap();
    let mut config = PerfDbConfig::new(dir.path());
    config.wal.sync_policy = SyncPolicy::EveryWrite;
    let db = PerfDb::open(config.clone()).unwrap();
    let tick = tick(1_000_000_000, 42.25);
    db.ingest_tick(&tick).unwrap();
    db.sync_wal().unwrap();
    let segment = std::fs::read(config.wal.dir.join("wal_00000000.bin")).unwrap();
    let len = u32::from_le_bytes(segment[0..4].try_into().unwrap()) as usize;
    let crc = u32::from_le_bytes(segment[4..8].try_into().unwrap());
    assert_eq!(segment.len(), 8 + len);
    assert_eq!(crc, crc32fast::hash(&segment[8..]));
    let entry = WalEntry::PriceTick {
        timestamp_ns: tick.timestamp_ns,
        market_id: tick.market_id,
        price: tick.price,
    };
    assert_eq!(&segment[8..], bincode::serialize(&entry).unwrap());
    let mut replayed = Vec::new();
    assert_eq!(
        wal::replay(&config.wal.dir, |entry| {
            replayed.push(bincode::serialize(&entry).unwrap());
            Ok(())
        })
        .unwrap(),
        1
    );
    assert_eq!(replayed, vec![segment[8..].to_vec()]);

    let inspection = PerfDb::open_read_only(config.clone()).unwrap();
    assert_eq!(inspection.ticks(0), vec![tick]);
    assert_eq!(inspection.latest_price(0), tick.price);
    assert!(inspection.ingest_tick(&tick).is_err());
    assert!(inspection.snapshot().is_err());
    assert!(inspection.shutdown().is_err());
    assert_eq!(
        std::fs::read(config.wal.dir.join("wal_00000000.bin")).unwrap(),
        segment
    );
    assert!(PerfDb::open(config.clone()).is_err());
    drop(inspection);
    drop(db);
    let reopened = PerfDb::open(config).unwrap();
    assert_eq!(reopened.ticks(0), vec![tick]);
    assert_eq!(reopened.latest_price(0), tick.price);
    reopened.shutdown().unwrap();

    let rotation_dir = tempfile::tempdir().unwrap();
    let mut writer = WalWriter::open(WalConfig {
        dir: rotation_dir.path().to_path_buf(),
        max_segment_size: segment.len() as u64,
        sync_policy: SyncPolicy::None,
    })
    .unwrap();
    writer.append(&entry).unwrap();
    assert_eq!(writer.current_segment(), 1);
    writer.append(&entry).unwrap();
    assert_eq!(writer.current_segment(), 2);
    drop(writer);
    let mut count = 0;
    assert_eq!(
        wal::replay(rotation_dir.path(), |_| {
            count += 1;
            Ok(())
        })
        .unwrap(),
        2
    );
    assert_eq!(count, 2);
}

#[test]
fn perf_architecture_cli_and_modules() {
    let cli = include_str!("../src/main.rs");
    for (variant, command) in [
        ("Info", "info"),
        ("Markets", "markets"),
        ("Prices", "prices"),
        ("Ticks", "ticks"),
        ("Candles", "candles"),
        ("Positions", "positions"),
        ("Orders", "orders"),
        ("Orderbook", "orderbook"),
        ("Balances", "balances"),
        ("User", "user"),
        ("Stats", "stats"),
        ("Daily", "daily"),
        ("Leaderboard", "leaderboard"),
        ("Events", "events"),
        ("Checkpoint", "checkpoint"),
        ("Snapshot", "snapshot"),
    ] {
        assert!(cli.contains(&format!("Command::{variant}")));
        assert!(ARCHITECTURE.contains(&format!("| {command} |")));
    }
    assert!(cli.contains("PerfDb::open_read_only(config)"));
    assert!(cli.contains("matches!(&cli.command, Command::Snapshot)"));
    assert!(ARCHITECTURE.contains("command-line inspector"));
    for obsolete in [
        "FundingStore",
        "TimeSeriesEngine",
        "StateEngine",
        "RelationalEngine",
        "EventLogEngine",
        "standalone server",
        "position_query.rs",
        "trade_query.rs",
        "event_query.rs",
    ] {
        assert!(
            !ARCHITECTURE.contains(obsolete),
            "obsolete component: {obsolete}"
        );
    }
    for token in ARCHITECTURE.split('`') {
        if token.starts_with("src/") && token.ends_with(".rs") {
            assert!(
                Path::new(env!("CARGO_MANIFEST_DIR")).join(token).is_file(),
                "missing source: {token}"
            );
        }
    }
    let dir = tempfile::tempdir().unwrap();
    let absent = PerfDbConfig::new(dir.path().join("absent"));
    assert!(PerfDb::open_read_only(absent.clone()).is_err());
    assert!(!absent.data_dir.exists());
    let db = PerfDb::open(absent.clone()).unwrap();
    db.ingest_tick(&tick(0, 55.0)).unwrap();
    db.shutdown().unwrap();
    drop(db);
    let inspector = PerfDb::open_read_only(absent).unwrap();
    assert_eq!(inspector.latest_price(0), 55.0);
    assert_eq!(inspector.tick_count(0), 1);
}

fn compact(source: &str) -> String {
    source.chars().filter(|c| !c.is_whitespace()).collect()
}

#[test]
fn perf_readme_toolchain_and_quick_start() {
    let manifest = include_str!("../Cargo.toml");
    let rust_version = manifest
        .lines()
        .find_map(|line| line.strip_prefix("rust-version = "))
        .unwrap()
        .trim_matches('"');
    let edition = manifest
        .lines()
        .find_map(|line| line.strip_prefix("edition = "))
        .unwrap()
        .trim_matches('"');
    assert!(README.contains(&format!("Rust-{rust_version}%2B")));
    assert!(README.contains(&format!(
        "Rust {rust_version} or newer (stable), with edition {edition} support"
    )));
    assert!(!README.contains("1.75"));
    let quick_start = README
        .split("## Quick Start\n")
        .nth(1)
        .unwrap()
        .split("```rust\n")
        .nth(1)
        .unwrap()
        .split("\n```")
        .next()
        .unwrap();
    // This entire fenced example is compiled below and called against the real engine.
    assert!(compact(include_str!("repair_22_33_1383.rs")).contains(&compact(quick_start)));
    assert_eq!(market_by_symbol("BTC").unwrap().id, 0);
    assert_eq!(market_by_symbol("ETH").unwrap().id, 1);
    let dir = tempfile::tempdir().unwrap();
    readme_example::ingest_btc(dir.path()).unwrap();
    let db = PerfDb::open(PerfDbConfig::new(dir.path())).unwrap();
    assert_eq!(
        db.ticks(0),
        vec![tick(1_699_900_000_000_000_000, 43_250.50)]
    );
    assert!(db.ticks(1).is_empty());
    assert_eq!(db.latest_price(0), 43_250.50);
    db.shutdown().unwrap();
    // Type-check the example's executable entry point without creating its ./data directory.
    readme_example::verify_entrypoint();
}

mod readme_example {
    use perfdb::core::market::market_by_symbol;
    use perfdb::core::types::PriceTick;
    use perfdb::{PerfDb, PerfDbConfig};

    pub fn ingest_btc(data_dir: impl AsRef<std::path::Path>) -> Result<(), perfdb::PerfDbError> {
        let config = PerfDbConfig::new(data_dir);
        let db = PerfDb::open(config)?;
        let btc = market_by_symbol("BTC").expect("BTC is a registered market");

        db.ingest_tick(&PriceTick {
            timestamp_ns: 1_699_900_000_000_000_000,
            market_id: btc.id, // BTC = 0; ETH = 1
            _pad: [0; 6],
            price: 43_250.50,
        })?;

        assert_eq!(db.latest_price(btc.id), 43_250.50);
        assert_eq!(db.ticks(btc.id).len(), 1);

        db.shutdown()?;
        Ok(())
    }

    fn main() -> Result<(), perfdb::PerfDbError> {
        ingest_btc("./data")
    }

    pub(super) fn verify_entrypoint() {
        let _: fn() -> Result<(), perfdb::PerfDbError> = main;
    }
}
