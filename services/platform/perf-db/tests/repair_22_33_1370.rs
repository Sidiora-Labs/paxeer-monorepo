use std::collections::HashMap;
use std::path::Path;
use std::sync::{Arc, Barrier};

use perfdb::core::types::{Event, IndexedEvent, PositionStatus};
use perfdb::formatting::format_fixed;
use perfdb::storage::wal::{self, SyncPolicy, WalEntry};
use perfdb::{PerfDb, PerfDbConfig, PerfDbError};

const SCALE: i128 = 1_000_000_000_000_000_000;
const USER: [u8; 20] = [1; 20];
const TOKEN: [u8; 20] = [2; 20];

fn config(path: &Path) -> PerfDbConfig {
    let mut config = PerfDbConfig::new(path);
    config.wal.sync_policy = SyncPolicy::EveryWrite;
    config.wal.max_segment_size = 1024;
    config
}

fn indexed(number: u64, event: Event) -> IndexedEvent {
    let mut tx_hash = [0; 32];
    tx_hash[..8].copy_from_slice(&number.to_le_bytes());
    IndexedEvent {
        block_number: number,
        block_timestamp: number * 12,
        tx_hash,
        tx_index: 0,
        log_index: 0,
        event,
    }
}

// Normalized field recordings from precompiles/perps/abi/pack_test.go and
// TestEventFundingSettled/TestEventVaultDeficit. Native 18-decimal USD/rates,
// six-decimal token amounts, signs and big-endian ids are retained verbatim.
fn recorded(name: &str) -> Event {
    let mut transcript: HashMap<String, Event> =
        serde_json::from_str(include_str!("../src/fixtures/repair_1370_chain.json")).unwrap();
    transcript.remove(name).unwrap()
}

fn opening(id: u8, is_long: bool, size: i128, leverage: i128) -> Event {
    Event::PositionOpened {
        position_id: [id; 32],
        user: USER,
        market_id: 0,
        is_long,
        size_usd: size,
        leverage,
        entry_price: 60_000 * SCALE,
        collateral_token: TOKEN,
        collateral_amount: 100_000_000,
    }
}

fn wal_count(config: &PerfDbConfig) -> u64 {
    wal::replay(&config.wal.dir, |_| Ok(())).unwrap()
}

fn assert_refused(db: &PerfDb, event: &IndexedEvent) {
    let cursor = db.checkpoint.last_block();
    let history = bincode::serialize(&db.events.global(None, None)).unwrap();
    let count = wal_count(db.config());
    assert!(matches!(
        db.process_event(event),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert_eq!(db.checkpoint.last_block(), cursor);
    assert_eq!(
        bincode::serialize(&db.events.global(None, None)).unwrap(),
        history
    );
    assert_eq!(
        wal_count(db.config()),
        count,
        "refusal must precede WAL append"
    );
}

fn assert_exposure(db: &PerfDb) {
    let (long, short) = db
        .positions
        .all()
        .into_iter()
        .filter(|p| p.status == PositionStatus::Open)
        .fold((0i128, 0i128), |(long, short), p| {
            if p.is_long {
                (long + p.size_usd, short)
            } else {
                (long, short + p.size_usd)
            }
        });
    assert_eq!(db.market_state.open_interest(0), (long, short));
}

#[test]
fn event_application_order_concurrent_balances_snapshots_and_reopen() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = Arc::new(PerfDb::open(cfg.clone()).unwrap());
    let barrier = Arc::new(Barrier::new(9));
    std::thread::scope(|scope| {
        for writer in 0..8u64 {
            let db = Arc::clone(&db);
            let barrier = Arc::clone(&barrier);
            scope.spawn(move || {
                barrier.wait();
                for i in 0..32u64 {
                    let number = 1 + writer * 32 + i;
                    let event = if i % 2 == 0 {
                        Event::CollateralDeposited {
                            user: USER,
                            token: TOKEN,
                            amount: SCALE,
                        }
                    } else {
                        Event::CollateralWithdrawn {
                            user: USER,
                            token: TOKEN,
                            amount: 2 * SCALE,
                        }
                    };
                    db.process_event(&indexed(number, event)).unwrap();
                }
            });
        }
        let db = Arc::clone(&db);
        let barrier = Arc::clone(&barrier);
        scope.spawn(move || {
            barrier.wait();
            for _ in 0..8 {
                db.snapshot().unwrap();
                std::thread::yield_now();
            }
        });
    });
    db.sync_wal().unwrap();
    let mut expected = 0i128;
    let mut ordered = Vec::new();
    let count = wal::replay(&cfg.wal.dir, |entry| {
        if let WalEntry::IndexedEvent(event) = entry {
            match event.event {
                Event::CollateralDeposited { amount, .. } => expected += amount,
                Event::CollateralWithdrawn { amount, .. } => expected = (expected - amount).max(0),
                _ => panic!("unexpected event"),
            }
            ordered.push(event);
        }
        Ok(())
    })
    .unwrap();
    assert_eq!(count, 256);
    assert_eq!(db.events.total_count(), 256);
    assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, expected);
    assert_eq!(db.checkpoint.last_block(), 256);
    assert_eq!(
        bincode::serialize(&db.events.global(None, None)).unwrap(),
        bincode::serialize(&ordered).unwrap()
    );
    drop(db);
    let db = PerfDb::open(cfg.clone()).unwrap();
    assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, expected);
    assert_eq!(db.checkpoint.last_block(), 256);
    assert_eq!(
        bincode::serialize(&db.events.global(None, None)).unwrap(),
        bincode::serialize(&ordered).unwrap()
    );
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.balances.get(&USER, &TOKEN).unwrap().amount, expected);
    assert_eq!(db.events.total_count(), 256);
}

#[test]
fn position_collateral_units_recorded_event_and_checked_domain() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    let event = indexed(1, recorded("position_opened"));
    db.process_event(&event).unwrap();
    let id = match &event.event {
        Event::PositionOpened { position_id, .. } => *position_id,
        _ => unreachable!(),
    };
    assert_eq!(db.positions.get(&id).unwrap().collateral_usd, 99 * SCALE);
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.positions.get(&id).unwrap().collateral_usd, 99 * SCALE);

    // Includes cases whose numerator multiplication exceeds i128 although
    // the final fixed-point result fits, and a non-integral quotient.
    for (size, leverage, expected) in [
        (1_000 * SCALE, 10 * SCALE, 100 * SCALE),
        (1_001 * SCALE + 7, 3 * SCALE, 333_666_666_666_666_666_669),
        (i128::MAX, SCALE, i128::MAX),
        (i128::MAX - 1, i128::MAX, SCALE - 1),
        (1, 3 * SCALE, 0),
    ] {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(config(dir.path())).unwrap();
        db.process_event(&indexed(1, opening(1, true, size, leverage)))
            .unwrap();
        assert_eq!(db.positions.get(&[1; 32]).unwrap().collateral_usd, expected);
    }
    for (size, leverage) in [(SCALE, 0), (SCALE, -1), (-1, SCALE), (i128::MAX, 1)] {
        assert_refused(&db, &indexed(100, opening(9, true, size, leverage)));
        assert!(db.positions.get(&[9; 32]).is_none());
    }
}

#[test]
fn open_interest_lifecycle_modification_liquidation_and_recovery() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.process_event(&indexed(1, opening(1, true, 1_000 * SCALE, 10 * SCALE)))
        .unwrap();
    db.process_event(&indexed(2, opening(2, false, 500 * SCALE, 5 * SCALE)))
        .unwrap();
    for (number, id, size) in [(3, 1, 1_500), (4, 2, 700), (5, 1, 900), (6, 2, 200)] {
        let modified = indexed(
            number,
            Event::PositionModified {
                position_id: [id; 32],
                new_size_usd: size * SCALE,
                new_collateral_usd: 100 * SCALE,
                new_collateral_amount: 100_000_000,
            },
        );
        db.process_event(&modified).unwrap();
        db.process_event(&modified).unwrap();
        assert_exposure(&db);
    }
    assert_eq!(db.market_state.open_interest(0), (900 * SCALE, 200 * SCALE));
    db.snapshot().unwrap();
    let liquidation = indexed(
        7,
        Event::Liquidation {
            position_id: [1; 32],
            user: USER,
            market_id: 0,
            liquidation_price: 59_000 * SCALE,
            penalty: SCALE,
            keeper: [3; 20],
        },
    );
    db.process_event(&liquidation).unwrap();
    db.process_event(&liquidation).unwrap();
    assert_eq!(db.market_state.open_interest(0), (0, 200 * SCALE));
    assert_eq!(db.trader_stats.get(&USER).unwrap().liquidation_count, 1);
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg.clone()).unwrap();
    assert_exposure(&db);
    db.process_event(&liquidation).unwrap();
    assert_eq!(db.market_state.open_interest(0), (0, 200 * SCALE));
    let close = indexed(
        8,
        Event::PositionClosed {
            position_id: [2; 32],
            user: USER,
            market_id: 0,
            closed_size_usd: 200 * SCALE,
            exit_price: 60_000 * SCALE,
            realized_pnl: 0,
            is_full_close: true,
        },
    );
    db.process_event(&close).unwrap();
    // The chain may advertise the liquidation after a full-close log.
    db.process_event(&indexed(
        9,
        Event::Liquidation {
            position_id: [2; 32],
            user: USER,
            market_id: 0,
            liquidation_price: 60_000 * SCALE,
            penalty: 0,
            keeper: [3; 20],
        },
    ))
    .unwrap();
    assert_eq!(db.market_state.open_interest(0), (0, 0));
    assert_exposure(&db);
    assert_eq!(db.trader_stats.get(&USER).unwrap().open_positions, 0);
    db.snapshot().unwrap();
    drop(db);
    assert_exposure(&PerfDb::open(cfg).unwrap());
}

#[test]
fn open_interest_checked_refusals_precede_durable_cursor() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.process_event(&indexed(1, opening(1, true, i128::MAX - 10, SCALE)))
        .unwrap();
    db.process_event(&indexed(2, opening(2, true, 5, SCALE)))
        .unwrap();
    assert_refused(
        &db,
        &indexed(
            3,
            Event::PositionModified {
                position_id: [2; 32],
                new_size_usd: 11,
                new_collateral_usd: 1,
                new_collateral_amount: 1,
            },
        ),
    );
    assert_refused(&db, &indexed(3, opening(3, true, 6, SCALE)));
    assert_refused(
        &db,
        &indexed(
            3,
            Event::PositionModified {
                position_id: [2; 32],
                new_size_usd: -1,
                new_collateral_usd: 1,
                new_collateral_amount: 1,
            },
        ),
    );
    assert_refused(
        &db,
        &indexed(
            3,
            Event::ADLExecuted {
                position_id: [2; 32],
                deleveraged_size_usd: 6,
            },
        ),
    );
    assert_eq!(db.positions.get(&[2; 32]).unwrap().size_usd, 5);
    assert_eq!(db.market_state.open_interest(0), (i128::MAX - 5, 0));
    // A refused identity remains available; the database is not poisoned.
    db.process_event(&indexed(
        3,
        Event::PositionModified {
            position_id: [2; 32],
            new_size_usd: 10,
            new_collateral_usd: 1,
            new_collateral_amount: 1,
        },
    ))
    .unwrap();
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.market_state.open_interest(0), (i128::MAX, 0));
    assert_eq!(db.checkpoint.last_block(), 3);
    assert_eq!(db.events.total_count(), 3);
}

#[test]
fn ignored_chain_effects_recorded_settlement_deficit_adl_and_recovery() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    let opened = indexed(1, recorded("position_opened"));
    let id = match &opened.event {
        Event::PositionOpened { position_id, .. } => *position_id,
        _ => unreachable!(),
    };
    db.process_event(&opened).unwrap();
    db.process_event(&indexed(2, recorded("position_modified")))
        .unwrap();
    db.process_event(&indexed(
        3,
        Event::FundingRateUpdated {
            market_id: 0,
            new_rate_per_second: 3_472_222_222,
            funding_rate_24h: 300_000_000_000_000,
        },
    ))
    .unwrap();
    db.process_event(&indexed(4, recorded("funding_settled")))
        .unwrap();
    let deficit = indexed(5, recorded("vault_deficit"));
    let token = match &deficit.event {
        Event::VaultDeficit { token, .. } => *token,
        _ => unreachable!(),
    };
    db.process_event(&deficit).unwrap();
    let adl = indexed(6, recorded("adl_executed"));
    db.process_event(&adl).unwrap();
    db.process_event(&adl).unwrap();
    assert_eq!(
        db.positions.get(&id).unwrap().size_usd,
        917_760 * 1_000_000_000_000_000
    );
    assert_exposure(&db);
    let settlement = db.events.latest_funding_settlement(0).unwrap();
    assert_eq!(settlement.funding_rate_hourly, 12_500_000_000_000);
    assert_eq!(settlement.long_payment, -1_500_000_000_000_000_000);
    assert_eq!(settlement.short_payment, 1_500_000_000_000_000_000);
    assert_eq!(db.market_state.funding_rate(0), 3_472_222_222);
    assert_eq!(
        db.events.latest_vault_deficit(&token).unwrap().deficit,
        1_000_000_000
    );
    // First recover only from the real WAL; then from a snapshot plus suffix.
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg.clone()).unwrap();
    assert_eq!(db.events.latest_funding_settlement(0).unwrap(), settlement);
    assert_eq!(
        db.events.latest_vault_deficit(&token).unwrap().deficit,
        1_000_000_000
    );
    assert_exposure(&db);
    db.snapshot().unwrap();
    db.process_event(&indexed(
        7,
        Event::ADLExecuted {
            position_id: id,
            deleveraged_size_usd: 917_760 * 1_000_000_000_000_000,
        },
    ))
    .unwrap();
    assert_eq!(
        db.positions.get(&id).unwrap().status,
        PositionStatus::Closed
    );
    assert_eq!(
        db.trader_stats
            .get(&db.positions.get(&id).unwrap().user_address)
            .unwrap()
            .open_positions,
        0
    );
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_exposure(&db);
    assert_eq!(db.market_state.open_interest(0), (0, 0));
    assert_eq!(db.events.latest_funding_settlement(0).unwrap(), settlement);
    assert_eq!(
        db.events.latest_vault_deficit(&token).unwrap().deficit,
        1_000_000_000
    );
}

#[test]
fn ignored_chain_effects_price_market_and_unsupported_refusal() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.process_event(&indexed(1, Event::MarketPaused { market_id: 0 }))
        .unwrap();
    assert!(db.market_state.is_paused(0));
    db.process_event(&indexed(
        2,
        Event::MarketCreated {
            market_id: 0,
            name: "Bitcoin".into(),
            symbol: "BTC".into(),
        },
    ))
    .unwrap();
    assert!(db.market_state.is_enabled(0));
    assert!(!db.market_state.is_paused(0));
    db.process_event(&indexed(
        3,
        Event::PriceUpdated {
            market_id: 0,
            price: 60_100 * SCALE,
            timestamp: 3600,
        },
    ))
    .unwrap();
    assert_eq!(db.latest_price(0), 60_100.0);
    assert_eq!(db.market_state.last_update_ns(0), 3_600_000_000_000);
    for event in [
        Event::MarketCreated {
            market_id: 18,
            name: "New".into(),
            symbol: "NEW".into(),
        },
        Event::MarketCreated {
            market_id: 0,
            name: "Conflicting".into(),
            symbol: "OTHER".into(),
        },
        Event::PriceUpdated {
            market_id: 18,
            price: SCALE,
            timestamp: 3600,
        },
        Event::PriceUpdated {
            market_id: 0,
            price: SCALE,
            timestamp: u64::MAX,
        },
        Event::PositionModified {
            position_id: [9; 32],
            new_size_usd: SCALE,
            new_collateral_usd: SCALE,
            new_collateral_amount: 1,
        },
        Event::OrderExecuted {
            order_id: [9; 32],
            position_id: [9; 32],
            execution_price: SCALE,
        },
    ] {
        assert_refused(&db, &indexed(4, event));
    }
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.checkpoint.last_block(), 3);
    assert_eq!(db.latest_price(0), 60_100.0);
    assert!(db.market_state.is_enabled(0));
    assert!(!db.market_state.is_paused(0));
}

#[test]
fn event_history_unindexed_variants_bound_routes_global_and_replay() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    let open = indexed(1, recorded("position_opened"));
    let (id, user) = match &open.event {
        Event::PositionOpened {
            position_id, user, ..
        } => (*position_id, *user),
        _ => unreachable!(),
    };
    let mut order_id = [0; 32];
    order_id[31] = 5;
    let transcript = vec![
        open,
        indexed(2, recorded("position_modified")),
        indexed(
            3,
            Event::OrderPlaced {
                order_id,
                user,
                market_id: 0,
                order_type: 0,
                is_long: true,
                trigger_price: 60_000 * SCALE,
                size_usd: 600 * SCALE,
            },
        ),
        indexed(4, recorded("order_executed")),
        indexed(5, recorded("adl_executed")),
        indexed(6, recorded("vault_deficit")),
    ];
    for event in &transcript {
        db.process_event(event).unwrap();
    }
    assert_eq!(db.orders.get(&order_id).unwrap().position_id, Some(id));
    let assert_history = |db: &PerfDb| {
        assert_eq!(db.events.total_count(), 6);
        assert_eq!(
            bincode::serialize(&db.events.global(None, None)).unwrap(),
            bincode::serialize(&transcript).unwrap()
        );
        assert_eq!(
            db.events
                .by_market(0, None, None)
                .iter()
                .map(|e| e.block_number)
                .collect::<Vec<_>>(),
            vec![1, 2, 3, 4, 5]
        );
        assert_eq!(
            db.events
                .by_trader(&user, None, None)
                .iter()
                .map(|e| e.block_number)
                .collect::<Vec<_>>(),
            vec![1, 2, 3, 4, 5]
        );
        assert!(matches!(
            db.events.global(Some(6), Some(7))[0].event,
            Event::VaultDeficit { .. }
        ));
        assert_eq!(
            db.events
                .latest_global(2)
                .iter()
                .map(|e| e.block_number)
                .collect::<Vec<_>>(),
            vec![6, 5]
        );
    };
    assert_history(&db);
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg.clone()).unwrap();
    assert_history(&db);
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(cfg.clone()).unwrap();
    assert_history(&db);
    // Cancellation also derives its market from the order before terminal state.
    let cancel_order = [8; 32];
    db.process_event(&indexed(
        7,
        Event::OrderPlaced {
            order_id: cancel_order,
            user,
            market_id: 0,
            order_type: 0,
            is_long: false,
            trigger_price: SCALE,
            size_usd: SCALE,
        },
    ))
    .unwrap();
    db.process_event(&indexed(
        8,
        Event::OrderCancelled {
            order_id: cancel_order,
            user,
        },
    ))
    .unwrap();
    assert_eq!(db.events.latest_by_market(0, 1)[0].block_number, 8);
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.events.latest_by_market(0, 1)[0].block_number, 8);
    assert_eq!(db.events.latest_by_trader(&user, 1)[0].block_number, 8);
}

#[test]
fn cli_negative_fraction_exact_formatting_entire_domain() {
    for (value, expected) in [
        (0, "0"),
        (SCALE, "1"),
        (-SCALE, "-1"),
        (SCALE / 2, "0.5"),
        (-SCALE / 2, "-0.5"),
        (1, "0.000000000000000001"),
        (-1, "-0.000000000000000001"),
        (-SCALE - 120_000_000_000_000_000, "-1.12"),
        (i128::MIN, "-170141183460469231731.687303715884105728"),
        (i128::MAX, "170141183460469231731.687303715884105727"),
    ] {
        assert_eq!(format_fixed(value), expected);
    }
    // The CLI's actual formatter consumes a financial view recovered from
    // the engine, with a negative subunit realized PnL from its chain event.
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.process_event(&indexed(1, opening(1, true, 1_000 * SCALE, 10 * SCALE)))
        .unwrap();
    db.process_event(&indexed(
        2,
        Event::PositionClosed {
            position_id: [1; 32],
            user: USER,
            market_id: 0,
            closed_size_usd: 1_000 * SCALE,
            exit_price: 59_999 * SCALE,
            realized_pnl: -SCALE / 2,
            is_full_close: true,
        },
    ))
    .unwrap();
    assert_eq!(
        format_fixed(db.trader_stats.get(&USER).unwrap().total_pnl),
        "-0.5"
    );
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(
        format_fixed(db.trader_stats.get(&USER).unwrap().total_pnl),
        "-0.5"
    );
}
