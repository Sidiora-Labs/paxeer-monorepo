//! Task 22.33.1373 repair record: query-reversed-range is proven by
//! query_reversed_range_ticks, query_reversed_range_ticks_inclusive,
//! query_reversed_range_candles and query_reversed_range_candles_inclusive.
//! Baseline verification was not run: the task's ordered qualification rules
//! require implementation first and verification only through hf-done.
//! Wiring requests: none; the repair preserves all four public signatures.

use std::fmt::Debug;

use perfdb::aggregation::candle_builder::{CandleBuilder, CandleOutput};
use perfdb::core::timeframe::Timeframe;
use perfdb::core::types::PriceTick;
use perfdb::query::time_range::{
    query_candle_range, query_candle_range_inclusive, query_tick_range, query_tick_range_inclusive,
};
use perfdb::storage::candle_store::CandleStore;
use perfdb::storage::tick_store::TickStore;

const SECOND: u64 = 1_000_000_000;

fn stored_records() -> (tempfile::TempDir, TickStore, CandleStore) {
    let dir = tempfile::tempdir().unwrap();
    let mut ticks = TickStore::open(dir.path().join("ticks")).unwrap();
    let mut candles = CandleStore::open(dir.path().join("candles")).unwrap();
    let mut builder = CandleBuilder::new();
    for (index, timestamp_ns) in [0, SECOND, 2 * SECOND, 2 * SECOND, 3 * SECOND]
        .into_iter()
        .enumerate()
    {
        let tick = PriceTick {
            timestamp_ns,
            market_id: 0,
            _pad: [0; 6],
            price: 60_000.0 + index as f64,
        };
        ticks.append(&tick).unwrap();
        for output in builder.on_tick(&tick) {
            if let CandleOutput::Closed(candle) = output {
                candles.append(&candle).unwrap();
            }
        }
    }
    for candle in builder.flush_all() {
        candles.append(&candle).unwrap();
    }
    ticks
        .append(&PriceTick {
            timestamp_ns: u64::MAX,
            market_id: 0,
            _pad: [0; 6],
            price: 60_100.0,
        })
        .unwrap();
    assert_eq!(ticks.tick_count(0), 6);
    assert_eq!(candles.candle_count(0, Timeframe::Sec1), 4);
    (dir, ticks, candles)
}

fn assert_ranges<T: Copy + Debug + PartialEq>(
    records: &[T],
    timestamp: impl Fn(&T) -> u64,
    query: fn(&[T], u64, u64) -> &[T],
    inclusive: bool,
) {
    // These bounds used to yield lo > hi and panic on a populated mmap slice.
    assert!(query(records, 3 * SECOND, SECOND).is_empty());
    let bounds = [
        0,
        1,
        SECOND - 1,
        SECOND,
        SECOND + 1,
        2 * SECOND,
        2 * SECOND + 1,
        3 * SECOND,
        3 * SECOND + 1,
        u64::MAX - 1,
        u64::MAX,
    ];
    for start in bounds {
        for end in bounds {
            let expected: Vec<T> = records
                .iter()
                .filter(|record| {
                    let ts = timestamp(record);
                    ts >= start && if inclusive { ts <= end } else { ts < end }
                })
                .copied()
                .collect();
            let actual = query(records, start, end);
            assert_eq!(actual, expected.as_slice(), "bounds {start}..{end}");
            if !actual.is_empty() {
                let first = records.iter().position(|r| timestamp(r) >= start).unwrap();
                assert_eq!(actual.as_ptr(), records[first..].as_ptr());
            }
        }
    }
}

#[test]
fn query_reversed_range_ticks() {
    let (_dir, ticks, _candles) = stored_records();
    for records in [ticks.ticks(0), ticks.ticks(1), &ticks.ticks(0)[2..3]] {
        assert_ranges(records, |t| t.timestamp_ns, query_tick_range, false);
    }
}

#[test]
fn query_reversed_range_ticks_inclusive() {
    let (_dir, ticks, _candles) = stored_records();
    for records in [ticks.ticks(0), ticks.ticks(1), &ticks.ticks(0)[2..3]] {
        assert_ranges(
            records,
            |t| t.timestamp_ns,
            query_tick_range_inclusive,
            true,
        );
    }
}

#[test]
fn query_reversed_range_candles() {
    let (_dir, _ticks, candles) = stored_records();
    let stored = candles.candles(0, Timeframe::Sec1);
    for records in [stored, candles.candles(1, Timeframe::Sec1), &stored[2..3]] {
        assert_ranges(records, |c| c.timestamp_ns, query_candle_range, false);
    }
}

#[test]
fn query_reversed_range_candles_inclusive() {
    let (_dir, _ticks, candles) = stored_records();
    let stored = candles.candles(0, Timeframe::Sec1);
    for records in [stored, candles.candles(1, Timeframe::Sec1), &stored[2..3]] {
        assert_ranges(
            records,
            |c| c.timestamp_ns,
            query_candle_range_inclusive,
            true,
        );
    }
}
