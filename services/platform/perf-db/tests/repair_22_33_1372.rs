use perfdb::aggregation::candle_builder::CandleOutput;
use perfdb::core::timeframe::Timeframe;
use perfdb::core::types::{CandleRecord, PriceTick};
use perfdb::{PerfDb, PerfDbConfig};

const SECOND: u64 = 1_000_000_000;
const WEEK: u64 = 604_800 * SECOND;

fn tick(timestamp_ns: u64, market_id: u16, price: f64) -> PriceTick {
    PriceTick {
        timestamp_ns,
        market_id,
        _pad: [0; 6],
        price,
    }
}

fn candle(timestamp_ns: u64, market_id: u16, tf: Timeframe, price: f64) -> CandleRecord {
    CandleRecord {
        timestamp_ns: tf.bucket_start_ns(timestamp_ns),
        open: price,
        high: price,
        low: price,
        close: price,
        volume: 0.0,
        market_id,
        timeframe: tf as u8,
        _pad: 0,
    }
}

fn assert_live_outputs(outputs: &[CandleOutput], expected: &[CandleRecord]) {
    assert_eq!(outputs.len(), expected.len());
    for (output, expected) in outputs.iter().zip(expected) {
        match output {
            CandleOutput::Live(actual) => assert_eq!(actual, expected),
            CandleOutput::Closed(_) => {
                panic!("tick in an active bucket must not finalize a candle")
            }
        }
    }
}

#[test]
fn candle_time_rewind_past_buckets_preserve_published_and_recovered_candles() {
    // Every timeframe has a boundary at a multiple of a week.
    for checkpoint_before_late_tick in [false, true] {
        let dir = tempfile::tempdir().unwrap();
        let cfg = PerfDbConfig::new(dir.path());
        let db = PerfDb::open(cfg.clone()).unwrap();
        db.ingest_tick(&tick(2 * WEEK - SECOND, 0, 100.0)).unwrap();
        let advanced = db.ingest_tick(&tick(2 * WEEK, 0, 200.0)).unwrap();
        assert_eq!(advanced.len(), 32);
        for (tf, pair) in Timeframe::ALL.into_iter().zip(advanced.chunks_exact(2)) {
            match (&pair[0], &pair[1]) {
                (CandleOutput::Closed(closed), CandleOutput::Live(live)) => {
                    assert_eq!(*closed, candle(2 * WEEK - SECOND, 0, tf, 100.0));
                    assert_eq!(*live, candle(2 * WEEK, 0, tf, 200.0));
                }
                _ => {
                    panic!("future bucket must close the old candle before publishing the new one")
                }
            }
        }
        if checkpoint_before_late_tick {
            db.sync_wal().unwrap();
        }

        assert!(db
            .ingest_tick(&tick(2 * WEEK - 1, 0, 1.0))
            .unwrap()
            .is_empty());
        for tf in Timeframe::ALL {
            assert_eq!(db.live_candle(0, tf), Some(candle(2 * WEEK, 0, tf, 200.0)));
            assert_eq!(
                db.candles(0, tf),
                vec![candle(2 * WEEK - SECOND, 0, tf, 100.0)]
            );
        }
        // Reopen with either all ticks in WAL or a checkpoint followed by WAL.
        drop(db);
        let db = PerfDb::open(cfg.clone()).unwrap();
        for tf in Timeframe::ALL {
            assert_eq!(db.live_candle(0, tf), Some(candle(2 * WEEK, 0, tf, 200.0)));
            assert_eq!(
                db.candles(0, tf),
                vec![candle(2 * WEEK - SECOND, 0, tf, 100.0)]
            );
        }
        assert!(db.ingest_tick(&tick(WEEK, 0, 2.0)).unwrap().is_empty());

        let advanced = db.ingest_tick(&tick(3 * WEEK, 0, 300.0)).unwrap();
        assert_eq!(advanced.len(), 32);
        db.shutdown().unwrap();
        drop(db);
        let db = PerfDb::open(cfg).unwrap();
        for tf in Timeframe::ALL {
            let stored = db.candles(0, tf);
            assert_eq!(
                stored,
                vec![
                    candle(2 * WEEK - SECOND, 0, tf, 100.0),
                    candle(2 * WEEK, 0, tf, 200.0),
                    candle(3 * WEEK, 0, tf, 300.0),
                ]
            );
            assert!(stored
                .windows(2)
                .all(|pair| pair[0].timestamp_ns < pair[1].timestamp_ns));
            assert_eq!(db.live_candle(0, tf), stored.last().copied());
        }
    }
}

#[test]
fn candle_time_rewind_active_bucket_keeps_arrival_order_after_reopen() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = PerfDbConfig::new(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    let base = 2 * WEEK;
    db.ingest_tick(&tick(base + 800_000_000, 0, 100.0)).unwrap();
    db.ingest_tick(&tick(base + 900_000_000, 0, 120.0)).unwrap();
    let late = db.ingest_tick(&tick(base + 200_000_000, 0, 90.0)).unwrap();
    let expected: Vec<_> = Timeframe::ALL
        .into_iter()
        .map(|tf| CandleRecord {
            high: 120.0,
            low: 90.0,
            close: 90.0,
            ..candle(base, 0, tf, 100.0)
        })
        .collect();
    assert_live_outputs(&late, &expected);
    for tf in Timeframe::ALL {
        assert_eq!(db.live_candle(0, tf), Some(expected[tf as usize]));
        assert!(db.candles(0, tf).is_empty());
    }
    db.shutdown().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    for tf in Timeframe::ALL {
        assert_eq!(db.live_candle(0, tf), Some(expected[tf as usize]));
        assert_eq!(db.candles(0, tf), vec![expected[tf as usize]]);
    }

    // Equal timestamps remain separate arrivals, and earlier arrivals in the
    // active bucket follow the same policy before and after recovery.
    let equal = db.ingest_tick(&tick(base + 200_000_000, 0, 110.0)).unwrap();
    let mut expected = expected;
    for record in &mut expected {
        record.close = 110.0;
    }
    assert_live_outputs(&equal, &expected);
    let late = db.ingest_tick(&tick(base + 100_000_000, 0, 95.0)).unwrap();
    for record in &mut expected {
        record.close = 95.0;
    }
    assert_live_outputs(&late, &expected);
    let advanced = db.ingest_tick(&tick(base + WEEK, 0, 130.0)).unwrap();
    assert_eq!(advanced.len(), 32);
    for tf in Timeframe::ALL {
        assert_eq!(db.candles(0, tf), vec![expected[tf as usize]]);
        assert_eq!(
            db.live_candle(0, tf),
            Some(candle(base + WEEK, 0, tf, 130.0))
        );
    }
}

#[test]
fn candle_time_rewind_mixed_timeframes_and_markets_are_independent() {
    let dir = tempfile::tempdir().unwrap();
    let db = PerfDb::open(PerfDbConfig::new(dir.path())).unwrap();
    db.ingest_tick(&tick(200_000_000, 0, 100.0)).unwrap();
    db.ingest_tick(&tick(SECOND + 200_000_000, 0, 200.0))
        .unwrap();
    let late = db.ingest_tick(&tick(800_000_000, 0, 50.0)).unwrap();
    let expected: Vec<_> = Timeframe::ALL
        .into_iter()
        .skip(1)
        .map(|tf| CandleRecord {
            high: 200.0,
            low: 50.0,
            close: 50.0,
            ..candle(0, 0, tf, 100.0)
        })
        .collect();
    assert_live_outputs(&late, &expected);
    assert_eq!(
        db.live_candle(0, Timeframe::Sec1),
        Some(candle(SECOND, 0, Timeframe::Sec1, 200.0))
    );
    assert_eq!(
        db.candles(0, Timeframe::Sec1),
        vec![candle(0, 0, Timeframe::Sec1, 100.0)]
    );
    for tf in Timeframe::ALL.into_iter().skip(1) {
        assert_eq!(db.live_candle(0, tf), Some(expected[tf as usize - 1]));
        assert!(db.candles(0, tf).is_empty());
    }

    // A different market may start earlier without being considered late.
    let other = db.ingest_tick(&tick(0, 1, 10.0)).unwrap();
    let other_expected: Vec<_> = Timeframe::ALL
        .into_iter()
        .map(|tf| candle(0, 1, tf, 10.0))
        .collect();
    assert_live_outputs(&other, &other_expected);
    db.ingest_tick(&tick(2 * SECOND, 0, 300.0)).unwrap();
    assert_eq!(
        db.candles(0, Timeframe::Sec1),
        vec![
            candle(0, 0, Timeframe::Sec1, 100.0),
            candle(SECOND, 0, Timeframe::Sec1, 200.0),
        ]
    );
    for tf in Timeframe::ALL {
        assert_eq!(db.live_candle(1, tf), Some(other_expected[tf as usize]));
        assert!(db.candles(1, tf).is_empty());
    }
}
