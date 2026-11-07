use std::fs::{self, OpenOptions};

use perfdb::core::timeframe::Timeframe;
use perfdb::core::types::*;
use perfdb::engine::PerfDbConfig;
use perfdb::relational::alerts::Alert;
use perfdb::relational::api_keys::ApiKey;
use perfdb::relational::competitions::Competition;
use perfdb::relational::referrals::ReferralTier;
use perfdb::storage::wal::{self, SyncPolicy, WalConfig, WalEntry};
use perfdb::PerfDb;

const SCALE: i128 = 1_000_000_000_000_000_000;

fn test_config(dir: &std::path::Path) -> PerfDbConfig {
    PerfDbConfig {
        data_dir: dir.to_path_buf(),
        wal: WalConfig {
            dir: dir.join("wal"),
            max_segment_size: 64 * 1024 * 1024,
            sync_policy: SyncPolicy::EveryWrite,
        },
        leaderboard_max: 100,
        pubsub_capacity: 64,
    }
}

fn make_tick(ts_ns: u64, market_id: u16, price: f64) -> PriceTick {
    PriceTick {
        timestamp_ns: ts_ns,
        market_id,
        _pad: [0; 6],
        price,
    }
}

fn addr(id: u8) -> [u8; 20] {
    let mut buf = [0u8; 20];
    buf[19] = id;
    buf
}

fn pid(id: u8) -> [u8; 32] {
    let mut buf = [0u8; 32];
    buf[31] = id;
    buf
}

fn oid(id: u8) -> [u8; 32] {
    let mut buf = [0u8; 32];
    buf[30] = 0xFF;
    buf[31] = id;
    buf
}

fn token(id: u8) -> [u8; 20] {
    let mut buf = [0u8; 20];
    buf[0] = 0xAA;
    buf[19] = id;
    buf
}

fn make_event(block: u64, event: Event) -> IndexedEvent {
    IndexedEvent {
        block_number: block,
        block_timestamp: block * 12,
        tx_hash: {
            let mut buf = [0u8; 32];
            buf[31] = block as u8;
            buf[30] = (block >> 8) as u8;
            buf
        },
        tx_index: 0,
        log_index: 0,
        event,
    }
}

pub fn all_event_variants_process_cleanly() {
    let dir = tempfile::tempdir().unwrap();
    let db = PerfDb::open(test_config(dir.path())).unwrap();

    let events = event_fixture();

    for event in &events {
        db.process_event(event).unwrap();
        assert_event_effect(&db, event);
    }
    assert_eq!(events.len(), 20);
    assert_eq!(db.events.total_count(), events.len() as u64);
    assert_eq!(db.checkpoint.last_block(), 20);
    db.sync_wal().unwrap();
    assert_wal_events(&db.config().wal.dir, &events);
}

pub fn snapshot_preserves_all_state() {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());

    {
        let db = PerfDb::open(config.clone()).unwrap();

        for i in 1..=5u8 {
            db.process_event(&make_event(
                i as u64 * 10,
                Event::CollateralDeposited {
                    user: addr(i),
                    token: token(1),
                    amount: i as i128 * 1_000_000,
                },
            ))
            .unwrap();

            db.process_event(&make_event(
                i as u64 * 10 + 1,
                Event::PositionOpened {
                    position_id: pid(i),
                    user: addr(i),
                    market_id: (i % 3) as u16,
                    is_long: i % 2 == 0,
                    size_usd: i as i128 * 100_000,
                    leverage: 10 * SCALE,
                    entry_price: 67_000,
                    collateral_token: token(1),
                    collateral_amount: i as i128 * 10_000,
                },
            ))
            .unwrap();
        }

        for i in [1u8, 3, 5] {
            db.process_event(&make_event(
                200 + i as u64,
                Event::PositionClosed {
                    position_id: pid(i),
                    user: addr(i),
                    market_id: (i % 3) as u16,
                    closed_size_usd: i as i128 * 100_000,
                    exit_price: 68_000,
                    realized_pnl: i as i128 * 1_000,
                    is_full_close: true,
                },
            ))
            .unwrap();
        }

        populate_relational_state(&db);
        assert_relational_state(&db);
        db.save_checkpoint(205, "0xblock205".to_string(), 2460);
        db.snapshot().unwrap();
    }

    {
        let db = PerfDb::open(config).unwrap();

        for i in 1..=5u8 {
            assert!(db.users.contains(&addr(i)), "user {} not found", i);
        }

        for i in 1..=5u8 {
            let pos = db.positions.get(&pid(i)).unwrap();
            if [1, 3, 5].contains(&i) {
                assert!(matches!(pos.status, PositionStatus::Closed));
            } else {
                assert!(matches!(pos.status, PositionStatus::Open));
            }
        }

        for i in 1..=5u8 {
            let bal = db.balances.get(&addr(i), &token(1)).unwrap();
            assert_eq!(bal.amount, i as i128 * 1_000_000);
        }

        assert_eq!(db.checkpoint.last_block(), 205);
        assert_relational_state(&db);
    }
}

pub fn truncated_wal_recovers_valid_entries() {
    tail_recovery(false);
}

pub fn corrupted_crc_stops_replay_at_corruption() {
    tail_recovery(true);
    interior_crc_corruption_is_rejected();
}

pub fn wal_replay_matches_live_position_state() {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());

    let live = {
        let db = PerfDb::open(config.clone()).unwrap();
        db.process_event(&make_event(
            1,
            Event::PositionOpened {
                position_id: pid(1),
                user: addr(1),
                market_id: 0,
                is_long: true,
                size_usd: 1_000_000,
                leverage: 10 * SCALE,
                entry_price: 67_000,
                collateral_token: token(1),
                collateral_amount: 100_000,
            },
        ))
        .unwrap();
        db.process_event(&make_event(
            2,
            Event::PositionModified {
                position_id: pid(1),
                new_size_usd: 1_500_000,
                new_collateral_usd: 150_000,
                new_collateral_amount: 150_000,
            },
        ))
        .unwrap();
        db.sync_wal().unwrap();
        (
            db.positions.get(&pid(1)).unwrap(),
            db.market_state.open_interest(0),
        )
    };

    let (live_pos, live_oi) = live;
    assert_eq!(live_pos.entry_price, 67_000);
    assert_eq!(live_pos.size_usd, 1_500_000);

    let db = PerfDb::open(config).unwrap();
    let pos = db.positions.get(&pid(1)).unwrap();
    assert_eq!(pos.entry_price, live_pos.entry_price);
    assert_eq!(pos.size_usd, live_pos.size_usd);
    assert_eq!(pos.collateral_usd, live_pos.collateral_usd);
    assert_eq!(pos.collateral_amount, live_pos.collateral_amount);
    assert_eq!(db.market_state.open_interest(0), live_oi);
    assert_eq!(live_oi, (1_500_000, 0));
}

fn assert_wal_events(dir: &std::path::Path, expected: &[IndexedEvent]) {
    let mut actual = Vec::new();
    wal::replay(dir, |entry| {
        if let WalEntry::IndexedEvent(event) = entry {
            actual.push(event);
        }
        Ok(())
    })
    .unwrap();
    assert_eq!(
        bincode::serialize(&actual).unwrap(),
        bincode::serialize(expected).unwrap()
    );
}

fn assert_unsupported_before_cursor(db: &PerfDb, event: &IndexedEvent) {
    db.sync_wal().unwrap();
    let checkpoint = db.checkpoint.last_block();
    let count = db.events.total_count();
    let mut before = Vec::new();
    wal::replay(&db.config().wal.dir, |entry| {
        before.push(entry);
        Ok(())
    })
    .unwrap();
    assert!(
        db.process_event(event).is_err(),
        "an invalid event must be refused explicitly"
    );
    assert_eq!(db.checkpoint.last_block(), checkpoint);
    assert_eq!(db.events.total_count(), count);
    db.sync_wal().unwrap();
    let mut after = Vec::new();
    wal::replay(&db.config().wal.dir, |entry| {
        after.push(entry);
        Ok(())
    })
    .unwrap();
    assert_eq!(
        bincode::serialize(&after).unwrap(),
        bincode::serialize(&before).unwrap()
    );
}

fn assert_event_effect(db: &PerfDb, indexed: &IndexedEvent) {
    match &indexed.event {
        Event::PositionOpened {
            position_id,
            user,
            market_id,
            is_long,
            size_usd,
            leverage,
            entry_price,
            collateral_token,
            collateral_amount,
        } => {
            let pos = db.positions.get(position_id).unwrap();
            assert_eq!(pos.user_address, *user);
            assert_eq!(pos.market_id, *market_id);
            assert_eq!(pos.is_long, *is_long);
            assert_eq!(pos.size_usd, *size_usd);
            assert_eq!(pos.collateral_usd, *size_usd / (*leverage / SCALE));
            assert_eq!(pos.collateral_amount, *collateral_amount);
            assert_eq!(pos.collateral_token, *collateral_token);
            assert_eq!(pos.entry_price, *entry_price);
            assert_eq!(pos.status, PositionStatus::Open);
            assert_eq!(pos.open_tx, indexed.tx_hash);
            assert_eq!(db.trader_stats.get(user).unwrap().open_positions, 1);
            assert_exposure(db);
        }
        Event::PositionModified {
            position_id,
            new_size_usd,
            new_collateral_usd,
            new_collateral_amount,
        } => {
            let pos = db.positions.get(position_id).unwrap();
            assert_eq!(pos.size_usd, *new_size_usd);
            assert_eq!(pos.collateral_usd, *new_collateral_usd);
            assert_eq!(pos.collateral_amount, *new_collateral_amount);
            assert_exposure(db);
        }
        Event::PositionClosed {
            position_id,
            user,
            exit_price,
            realized_pnl,
            is_full_close,
            ..
        } => {
            let pos = db.positions.get(position_id).unwrap();
            assert_eq!(pos.exit_price, Some(*exit_price));
            assert_eq!(pos.realized_pnl, Some(*realized_pnl));
            assert_eq!(
                pos.status,
                if *is_full_close {
                    PositionStatus::Closed
                } else {
                    PositionStatus::Open
                }
            );
            assert_eq!(db.trader_stats.get(user).unwrap().total_pnl, *realized_pnl);
            assert_eq!(
                db.trader_stats.get(user).unwrap().open_positions,
                if *is_full_close { 0 } else { 1 }
            );
            assert_exposure(db);
        }
        Event::OrderPlaced {
            order_id,
            user,
            market_id,
            is_long,
            trigger_price,
            size_usd,
            order_type,
        } => {
            let order = db.orders.get(order_id).unwrap();
            assert_eq!(order.status, OrderStatus::Active);
            assert_eq!(order.user_address, *user);
            assert_eq!(order.market_id, *market_id);
            assert_eq!(order.is_long, *is_long);
            assert_eq!(order.trigger_price, *trigger_price);
            assert_eq!(order.size_usd, *size_usd);
            assert_eq!(order.order_type, OrderType::from(*order_type));
        }
        Event::OrderExecuted {
            order_id,
            position_id,
            execution_price,
        } => {
            let order = db.orders.get(order_id).unwrap();
            assert_eq!(order.status, OrderStatus::Executed);
            assert_eq!(order.position_id, Some(*position_id));
            assert_eq!(order.execution_price, Some(*execution_price));
            assert_eq!(order.executed_at, Some(indexed.block_timestamp));
        }
        Event::OrderCancelled { order_id, .. } => assert_eq!(
            db.orders.get(order_id).unwrap().status,
            OrderStatus::Cancelled
        ),
        Event::Liquidation {
            position_id,
            user,
            liquidation_price,
            penalty,
            ..
        } => {
            let pos = db.positions.get(position_id).unwrap();
            assert_eq!(pos.status, PositionStatus::Liquidated);
            assert_eq!(pos.exit_price, Some(*liquidation_price));
            assert_eq!(pos.close_block, Some(indexed.block_number));
            let stats = db.trader_stats.get(user).unwrap();
            assert_eq!(stats.liquidation_count, 1);
            assert_eq!(stats.open_positions, 0);
            assert_eq!(stats.total_pnl, -*penalty);
            assert_exposure(db);
        }
        Event::ADLExecuted { position_id, .. } => {
            assert_eq!(db.positions.get(position_id).unwrap().size_usd, 50_000);
            assert_exposure(db);
        }
        Event::PriceUpdated {
            market_id,
            price,
            timestamp,
        } => {
            assert_eq!(db.latest_price(*market_id), *price as f64 / 1e18);
            assert_eq!(
                db.market_state.last_update_ns(*market_id),
                *timestamp * 1_000_000_000
            );
        }
        Event::FundingRateUpdated {
            market_id,
            new_rate_per_second,
            funding_rate_24h,
        } => {
            let state = db.market_state.snapshot(*market_id).unwrap();
            assert_eq!(state.funding_rate_per_second, *new_rate_per_second);
            assert_eq!(state.funding_rate_24h, *funding_rate_24h);
        }
        Event::MarketPaused { market_id } => {
            assert!(db.market_state.snapshot(*market_id).unwrap().paused)
        }
        Event::CollateralDeposited {
            user,
            token,
            amount,
        } => {
            let balance = db.balances.get(user, token).unwrap();
            assert_eq!(balance.amount, *amount);
            assert_eq!(balance.available, *amount);
            assert!(db.users.contains(user));
        }
        Event::CollateralWithdrawn {
            user,
            token,
            amount,
        } => {
            let balance = db.balances.get(user, token).unwrap();
            assert_eq!(balance.amount, 5_000_000 - *amount);
            assert_eq!(balance.available, balance.amount - balance.locked);
        }
        Event::VaultCreated { user, vault } => {
            assert_eq!(db.users.get(user).unwrap().vault_address, Some(*vault))
        }
        Event::FundingSettled {
            market_id,
            funding_rate,
            long_payment,
            short_payment,
        } => {
            let settlement = db.events.latest_funding_settlement(*market_id).unwrap();
            assert_eq!(settlement.block_number, indexed.block_number);
            assert_eq!(settlement.funding_rate_hourly, *funding_rate);
            assert_eq!(settlement.long_payment, *long_payment);
            assert_eq!(settlement.short_payment, *short_payment);
            // Settlement is hourly; only FundingRateUpdated owns the rate fields.
            assert_eq!(db.market_state.funding_rate(*market_id), 0);
        }
        Event::MarketCreated {
            market_id,
            name,
            symbol,
        } => {
            let meta = perfdb::core::market::market_by_id(*market_id).unwrap();
            assert_eq!(meta.name, name);
            assert_eq!(meta.symbol, symbol);
            assert!(db.market_state.is_enabled(*market_id));
            assert!(!db.market_state.is_paused(*market_id));
        }
        Event::VaultDeficit { token, deficit } => {
            let record = db.events.latest_vault_deficit(token).unwrap();
            assert_eq!(record.block_number, indexed.block_number);
            assert_eq!(record.deficit, *deficit);
        }
    }
}

fn assert_exposure(db: &PerfDb) {
    for market_id in 0..18 {
        let mut expected = (0, 0);
        for pos in db.positions.all() {
            if pos.market_id == market_id && pos.status == PositionStatus::Open {
                if pos.is_long {
                    expected.0 += pos.size_usd;
                } else {
                    expected.1 += pos.size_usd;
                }
            }
        }
        assert_eq!(
            db.market_state.open_interest(market_id),
            expected,
            "market {market_id} exposure must equal its open positions"
        );
    }
}

fn event_fixture() -> Vec<IndexedEvent> {
    vec![
        make_event(
            1,
            Event::PositionOpened {
                position_id: pid(1),
                user: addr(1),
                market_id: 0,
                is_long: true,
                size_usd: 100_000,
                leverage: 10 * SCALE,
                entry_price: 67_000,
                collateral_token: token(1),
                collateral_amount: 10_000,
            },
        ),
        make_event(
            2,
            Event::PositionModified {
                position_id: pid(1),
                new_size_usd: 200_000,
                new_collateral_usd: 20_000,
                new_collateral_amount: 20_000,
            },
        ),
        make_event(
            3,
            Event::PositionClosed {
                position_id: pid(1),
                user: addr(1),
                market_id: 0,
                closed_size_usd: 200_000,
                exit_price: 68_000,
                realized_pnl: 5_000,
                is_full_close: true,
            },
        ),
        make_event(
            4,
            Event::OrderPlaced {
                order_id: oid(1),
                user: addr(2),
                market_id: 0,
                order_type: 0,
                is_long: true,
                trigger_price: 66_000,
                size_usd: 100_000,
            },
        ),
        make_event(
            5,
            Event::OrderExecuted {
                order_id: oid(1),
                position_id: pid(5),
                execution_price: 66_100,
            },
        ),
        make_event(
            6,
            Event::OrderPlaced {
                order_id: oid(2),
                user: addr(3),
                market_id: 1,
                order_type: 2,
                is_long: false,
                trigger_price: 70_000,
                size_usd: 50_000,
            },
        ),
        make_event(
            7,
            Event::OrderCancelled {
                order_id: oid(2),
                user: addr(3),
            },
        ),
        make_event(
            8,
            Event::PositionOpened {
                position_id: pid(10),
                user: addr(4),
                market_id: 0,
                is_long: true,
                size_usd: 500_000,
                leverage: 50 * SCALE,
                entry_price: 67_000,
                collateral_token: token(1),
                collateral_amount: 10_000,
            },
        ),
        make_event(
            9,
            Event::Liquidation {
                position_id: pid(10),
                user: addr(4),
                market_id: 0,
                liquidation_price: 65_000,
                penalty: 5_000,
                keeper: addr(99),
            },
        ),
        make_event(
            10,
            Event::PositionOpened {
                position_id: pid(99),
                user: addr(7),
                market_id: 0,
                is_long: false,
                size_usd: 100_000,
                leverage: 10 * SCALE,
                entry_price: 67_000,
                collateral_token: token(1),
                collateral_amount: 10_000,
            },
        ),
        make_event(
            11,
            Event::ADLExecuted {
                position_id: pid(99),
                deleveraged_size_usd: 50_000,
            },
        ),
        make_event(
            12,
            Event::PriceUpdated {
                market_id: 0,
                price: 67_500_000_000_000_000_000_000,
                timestamp: 132,
            },
        ),
        make_event(
            13,
            Event::FundingSettled {
                market_id: 0,
                funding_rate: 100,
                long_payment: 5_000,
                short_payment: -5_000,
            },
        ),
        make_event(
            14,
            Event::FundingRateUpdated {
                market_id: 0,
                new_rate_per_second: 200,
                funding_rate_24h: 17_280_000,
            },
        ),
        make_event(
            15,
            Event::MarketCreated {
                market_id: 0,
                name: "Bitcoin".into(),
                symbol: "BTC".into(),
            },
        ),
        make_event(16, Event::MarketPaused { market_id: 0 }),
        make_event(
            17,
            Event::CollateralDeposited {
                user: addr(5),
                token: token(2),
                amount: 5_000_000,
            },
        ),
        make_event(
            18,
            Event::CollateralWithdrawn {
                user: addr(5),
                token: token(2),
                amount: 1_000_000,
            },
        ),
        make_event(
            19,
            Event::VaultCreated {
                user: addr(6),
                vault: addr(60),
            },
        ),
        make_event(
            20,
            Event::VaultDeficit {
                token: token(2),
                deficit: 500,
            },
        ),
    ]
}

pub fn event_variant(index: usize) {
    let events = event_fixture();
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    let prerequisites: &[usize] = match index {
        1 => &[0],
        2 => &[0, 1],
        4 => &[3],
        6 => &[5],
        8 => &[7],
        10 => &[9],
        17 => &[16],
        _ => &[],
    };
    let event = &events[index];
    let mut expected: Vec<IndexedEvent> =
        prerequisites.iter().map(|&i| events[i].clone()).collect();
    {
        let db = PerfDb::open(config.clone()).unwrap();
        for seed in &expected {
            db.process_event(seed).unwrap();
        }
        if index == 14 {
            db.market_state.set_paused(0, true);
            db.market_state.set_enabled(0, false);
        }
        db.process_event(event).unwrap();
        assert_event_effect(&db, event);
        expected.push(event.clone());
        db.sync_wal().unwrap();
        assert_wal_events(&config.wal.dir, &expected);
    }
    {
        let db = PerfDb::open(config.clone()).unwrap();
        assert_eq!(db.events.total_count(), expected.len() as u64);
        assert_eq!(db.checkpoint.last_block(), event.block_number);
        assert_event_effect(&db, event);
        db.snapshot().unwrap();
    }
    let db = PerfDb::open(config).unwrap();
    assert_eq!(db.events.total_count(), expected.len() as u64);
    assert_eq!(db.checkpoint.last_block(), event.block_number);
    assert_event_effect(&db, event);
}

fn populate_relational_state(db: &PerfDb) {
    let id = db.alerts.insert(Alert {
        id: 0,
        address: addr(1),
        alert_type: "price_above".into(),
        market_id: Some(0),
        threshold: Some("67000".into()),
        enabled: true,
        triggered_at: None,
        created_at: 11,
    });
    assert_eq!(id, 1);
    assert!(db.alerts.update_triggered(id, 12));
    assert!(db.alerts.set_enabled(id, false));
    db.api_keys.insert(ApiKey {
        id: "repair-key".into(),
        address: addr(1),
        key_hash: "fixture-hash".into(),
        label: Some("fixture".into()),
        rate_limit_rpm: 75,
        last_used_at: None,
        expires_at: Some(9999),
        created_at: 13,
    });
    assert!(db.api_keys.touch_last_used("repair-key", 14));
    db.referrals.lock().set_tiers(vec![
        ReferralTier {
            tier: "fixture-basic".into(),
            min_referrals: 0,
            min_volume: 0,
            reward_bps: 7,
            fee_discount_bps: 3,
        },
        ReferralTier {
            tier: "fixture-pro".into(),
            min_referrals: 2,
            min_volume: 100,
            reward_bps: 21,
            fee_discount_bps: 9,
        },
    ]);
    db.competitions.insert_competition(Competition {
        id: "repair-cup".into(),
        name: "Recovery cup".into(),
        description: Some("fixture".into()),
        start_at: 20,
        end_at: 200,
        metric: "pnl".into(),
        prize_pool: Some("500".into()),
        active: true,
        created_at: 15,
    });
    assert!(db.competitions.join("repair-cup", addr(1), 21));
    assert!(db.competitions.join("repair-cup", addr(2), 22));
    assert!(db.competitions.update_score("repair-cup", &addr(1), 75));
    assert!(db.competitions.update_score("repair-cup", &addr(2), 120));
    for (ts, price) in [(1_100_000_000, 100.0), (1_900_000_000, 80.0)] {
        db.ingest_tick(&make_tick(ts, 0, price)).unwrap();
    }
}

fn assert_relational_part(db: &PerfDb, part: u8) {
    match part {
        0 => {
            assert_eq!(db.alerts.count(), 1);
            let alert = db.alerts.get(1).unwrap();
            assert_eq!(alert.address, addr(1));
            assert_eq!(alert.alert_type, "price_above");
            assert_eq!(alert.market_id, Some(0));
            assert_eq!(alert.threshold.as_deref(), Some("67000"));
            assert!(!alert.enabled);
            assert_eq!(alert.triggered_at, Some(12));
            assert_eq!(alert.created_at, 11);
            assert_eq!(db.alerts.by_user(&addr(1)).len(), 1);
        }
        1 => {
            assert_eq!(db.api_keys.count(), 1);
            let key = db.api_keys.get("repair-key").unwrap();
            assert_eq!(key.address, addr(1));
            assert_eq!(key.key_hash, "fixture-hash");
            assert_eq!(key.label.as_deref(), Some("fixture"));
            assert_eq!(key.rate_limit_rpm, 75);
            assert_eq!(key.last_used_at, Some(14));
            assert_eq!(key.expires_at, Some(9999));
            assert_eq!(key.created_at, 13);
            assert_eq!(db.api_keys.get_by_hash("fixture-hash").unwrap().id, key.id);
            assert_eq!(db.api_keys.by_user(&addr(1)).len(), 1);
        }
        2 => {
            let referrals = db.referrals.lock();
            let tiers = referrals.tiers();
            assert_eq!(tiers.len(), 2);
            assert_eq!(
                (
                    tiers[0].tier.as_str(),
                    tiers[0].min_referrals,
                    tiers[0].min_volume,
                    tiers[0].reward_bps,
                    tiers[0].fee_discount_bps
                ),
                ("fixture-basic", 0, 0, 7, 3)
            );
            assert_eq!(
                (
                    tiers[1].tier.as_str(),
                    tiers[1].min_referrals,
                    tiers[1].min_volume,
                    tiers[1].reward_bps,
                    tiers[1].fee_discount_bps
                ),
                ("fixture-pro", 2, 100, 21, 9)
            );
            assert_eq!(referrals.compute_tier(2, 100).tier, "fixture-pro");
            assert_eq!(referrals.compute_tier(2, 99).tier, "fixture-basic");
        }
        3 => {
            assert_eq!(db.competitions.competition_count(), 1);
            let comp = db.competitions.get_competition("repair-cup").unwrap();
            assert_eq!(comp.name, "Recovery cup");
            assert_eq!(comp.description.as_deref(), Some("fixture"));
            assert_eq!((comp.start_at, comp.end_at, comp.created_at), (20, 200, 15));
            assert_eq!(comp.metric, "pnl");
            assert_eq!(comp.prize_pool.as_deref(), Some("500"));
            assert!(comp.active);
            let entries = db.competitions.get_entries("repair-cup", 10);
            assert_eq!(entries.len(), 2);
            assert_eq!(
                (entries[0].address, entries[0].score, entries[0].joined_at),
                (addr(2), 120, 22)
            );
            assert_eq!(
                (entries[1].address, entries[1].score, entries[1].joined_at),
                (addr(1), 75, 21)
            );
            assert_eq!(entries[0].competition_id, "repair-cup");
            assert_eq!(entries[0].rank, None);
            assert_eq!(
                db.competitions.user_competitions(&addr(1)),
                vec!["repair-cup".to_string()]
            );
            assert_eq!(
                db.competitions.user_competitions(&addr(2)),
                vec!["repair-cup".to_string()]
            );
        }
        4 => {
            assert_eq!(db.tick_count(0), 2);
            assert_eq!(db.latest_price(0), 80.0);
            for tf in Timeframe::ALL {
                let live = db.live_candle(0, tf).unwrap();
                assert_eq!(
                    (live.open, live.high, live.low, live.close, live.volume),
                    (100.0, 100.0, 80.0, 80.0, 0.0)
                );
                assert_eq!(live.market_id, 0);
                assert_eq!(live.timeframe, tf as u8);
            }
        }
        _ => unreachable!(),
    }
}

fn assert_relational_state(db: &PerfDb) {
    for part in 0..5 {
        assert_relational_part(db, part);
    }
}

pub fn relational_snapshot_roundtrip(part: u8) {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    {
        let db = PerfDb::open(config.clone()).unwrap();
        populate_relational_state(&db);
        assert_relational_part(&db, part);
        assert!(db.snapshot().unwrap().exists());
    }
    {
        let db = PerfDb::open(config.clone()).unwrap();
        assert_relational_part(&db, part);
        if part == 0 {
            let mut next = db.alerts.get(1).unwrap();
            next.id = 0;
            assert_eq!(db.alerts.insert(next), 2);
        }
        if part == 4 {
            db.ingest_tick(&make_tick(2_100_000_000, 0, 110.0)).unwrap();
            let closed = db.candles(0, Timeframe::Sec1);
            assert_eq!(closed.len(), 1);
            assert_eq!(
                (
                    closed[0].open,
                    closed[0].high,
                    closed[0].low,
                    closed[0].close
                ),
                (100.0, 100.0, 80.0, 80.0)
            );
        }
        db.sync_wal().unwrap();
    }
    if part == 4 {
        let db = PerfDb::open(config).unwrap();
        assert_eq!(db.tick_count(0), 3);
        let live = db.live_candle(0, Timeframe::Sec1).unwrap();
        assert_eq!((live.open, live.close), (110.0, 110.0));
        assert_eq!(db.candles(0, Timeframe::Sec1).len(), 1);
    }
}

fn recovery_events() -> Vec<IndexedEvent> {
    vec![
        make_event(
            1,
            Event::PositionOpened {
                position_id: pid(1),
                user: addr(1),
                market_id: 0,
                is_long: true,
                size_usd: 1_000_000,
                leverage: 10 * SCALE,
                entry_price: 67_000,
                collateral_token: token(1),
                collateral_amount: 100_000,
            },
        ),
        make_event(
            2,
            Event::CollateralDeposited {
                user: addr(2),
                token: token(1),
                amount: 3_000_000,
            },
        ),
        make_event(
            3,
            Event::PositionOpened {
                position_id: pid(2),
                user: addr(2),
                market_id: 1,
                is_long: false,
                size_usd: 500_000,
                leverage: 5 * SCALE,
                entry_price: 3_500,
                collateral_token: token(1),
                collateral_amount: 100_000,
            },
        ),
        make_event(
            4,
            Event::CollateralDeposited {
                user: addr(3),
                token: token(1),
                amount: 999_999,
            },
        ),
    ]
}

fn produce_recovery_wal(config: &PerfDbConfig, events: &[IndexedEvent]) -> std::path::PathBuf {
    {
        let db = PerfDb::open(config.clone()).unwrap();
        for (index, event) in events.iter().enumerate() {
            if index + 1 == events.len() {
                db.sync_wal().unwrap();
            }
            db.process_event(event).unwrap();
        }
        assert_wal_events(&config.wal.dir, events);
    }
    let mut segments: Vec<_> = fs::read_dir(&config.wal.dir)
        .unwrap()
        .map(|entry| entry.unwrap().path())
        .filter(|path| path.extension().is_some_and(|ext| ext == "bin"))
        .collect();
    segments.sort();
    assert_eq!(segments.len(), 1);
    segments.pop().unwrap()
}

fn next_frame(bytes: &[u8], offset: usize) -> usize {
    let len = u32::from_le_bytes(bytes[offset..offset + 4].try_into().unwrap()) as usize;
    let end = offset + 8 + len;
    assert!(end <= bytes.len());
    end
}

fn assert_recovery_prefix(db: &PerfDb) {
    let first = db.positions.get(&pid(1)).unwrap();
    assert_eq!(first.status, PositionStatus::Open);
    assert_eq!(
        (
            first.user_address,
            first.market_id,
            first.size_usd,
            first.entry_price
        ),
        (addr(1), 0, 1_000_000, 67_000)
    );
    let second = db.positions.get(&pid(2)).unwrap();
    assert_eq!(second.status, PositionStatus::Open);
    assert_eq!(
        (
            second.user_address,
            second.market_id,
            second.size_usd,
            second.entry_price
        ),
        (addr(2), 1, 500_000, 3_500)
    );
    assert_eq!(
        db.balances.get(&addr(2), &token(1)).unwrap().amount,
        3_000_000
    );
    assert!(
        db.balances.get(&addr(3), &token(1)).is_none(),
        "the damaged final event must never be applied"
    );
    assert_eq!(db.market_state.open_interest(0), (1_000_000, 0));
    assert_eq!(db.market_state.open_interest(1), (0, 500_000));
}

fn tail_recovery(corrupt_crc: bool) {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    let events = recovery_events();
    let segment = produce_recovery_wal(&config, &events);
    let bytes = fs::read(&segment).unwrap();
    let prefix_end = next_frame(&bytes, next_frame(&bytes, next_frame(&bytes, 0)));
    assert_eq!(next_frame(&bytes, prefix_end), bytes.len());
    if corrupt_crc {
        let mut damaged = bytes.clone();
        damaged[prefix_end + 4] ^= 0xFF;
        fs::write(&segment, &damaged).unwrap();
    } else {
        OpenOptions::new()
            .write(true)
            .open(&segment)
            .unwrap()
            .set_len((bytes.len() - 2) as u64)
            .unwrap();
    }
    OpenOptions::new()
        .write(true)
        .open(&segment)
        .unwrap()
        .sync_all()
        .unwrap();
    assert_wal_events(&config.wal.dir, &events[..3]);
    let fresh = make_event(
        5,
        Event::CollateralDeposited {
            user: addr(4),
            token: token(1),
            amount: 777_777,
        },
    );
    let expected = vec![
        events[0].clone(),
        events[1].clone(),
        events[2].clone(),
        fresh.clone(),
    ];
    {
        let db = PerfDb::open(config.clone()).unwrap();
        assert_recovery_prefix(&db);
        assert_eq!(db.events.total_count(), 3);
        assert_eq!(db.checkpoint.last_block(), 3);
        assert_eq!(fs::metadata(&segment).unwrap().len(), prefix_end as u64);
        db.process_event(&fresh).unwrap();
        assert_eq!(
            db.balances.get(&addr(4), &token(1)).unwrap().amount,
            777_777
        );
        db.sync_wal().unwrap();
        assert_wal_events(&config.wal.dir, &expected);
    }
    for _ in 0..2 {
        let db = PerfDb::open(config.clone()).unwrap();
        assert_recovery_prefix(&db);
        assert_eq!(
            db.balances.get(&addr(4), &token(1)).unwrap().amount,
            777_777
        );
        assert_eq!(db.events.total_count(), 4);
        assert_eq!(db.checkpoint.last_block(), 5);
        assert_wal_events(&config.wal.dir, &expected);
    }
}

pub fn interior_crc_corruption_is_rejected() {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    let segment = produce_recovery_wal(&config, &recovery_events());
    let mut bytes = fs::read(&segment).unwrap();
    let second = next_frame(&bytes, 0);
    bytes[second + 4] ^= 0xFF;
    fs::write(&segment, &bytes).unwrap();
    assert!(wal::replay(&config.wal.dir, |_| Ok(())).is_err());
    assert!(
        PerfDb::open(config).is_err(),
        "interior CRC corruption must not be repaired as a final append"
    );
    assert_eq!(fs::read(&segment).unwrap(), bytes);
}

pub fn lifecycle_recovery(is_long: bool, use_snapshot: bool, stage: usize) {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    let initial = make_event(
        1,
        Event::PositionOpened {
            position_id: pid(1),
            user: addr(1),
            market_id: 0,
            is_long,
            size_usd: 1_000_000,
            leverage: 10 * SCALE,
            entry_price: 67_000,
            collateral_token: token(1),
            collateral_amount: 100_000,
        },
    );
    {
        let db = PerfDb::open(config.clone()).unwrap();
        db.process_event(&initial).unwrap();
        assert_exposure(&db);
        db.sync_wal().unwrap();
    }
    let events = [
        make_event(
            2,
            Event::PositionModified {
                position_id: pid(1),
                new_size_usd: 1_500_000,
                new_collateral_usd: 150_000,
                new_collateral_amount: 150_000,
            },
        ),
        make_event(
            3,
            Event::PositionClosed {
                position_id: pid(1),
                user: addr(1),
                market_id: 0,
                closed_size_usd: 500_000,
                exit_price: 68_000,
                realized_pnl: 5_000,
                is_full_close: false,
            },
        ),
        make_event(
            4,
            Event::Liquidation {
                position_id: pid(1),
                user: addr(1),
                market_id: 0,
                liquidation_price: 65_000,
                penalty: 25_000,
                keeper: addr(99),
            },
        ),
    ];
    {
        let expected_size = [1_500_000, 1_000_000, 1_000_000][stage];
        let expected_oi = if stage == 2 {
            (0, 0)
        } else if is_long {
            (expected_size, 0)
        } else {
            (0, expected_size)
        };
        let live;
        {
            let db = PerfDb::open(config.clone()).unwrap();
            for event in &events[..=stage] {
                db.process_event(event).unwrap();
            }
            let pos = db.positions.get(&pid(1)).unwrap();
            assert_eq!(pos.size_usd, expected_size);
            assert_eq!(pos.collateral_usd, 150_000);
            assert_eq!(pos.collateral_amount, 150_000);
            assert_eq!(pos.entry_price, 67_000);
            assert_eq!(
                pos.status,
                if stage == 2 {
                    PositionStatus::Liquidated
                } else {
                    PositionStatus::Open
                }
            );
            let live_oi = db.market_state.open_interest(0);
            live = (
                bincode::serialize(&pos).unwrap(),
                bincode::serialize(&db.trader_stats.get(&addr(1)).unwrap()).unwrap(),
                live_oi,
            );
            db.sync_wal().unwrap();
            if use_snapshot {
                db.snapshot().unwrap();
            }
        }
        let db = PerfDb::open(config.clone()).unwrap();
        assert_eq!(
            bincode::serialize(&db.positions.get(&pid(1)).unwrap()).unwrap(),
            live.0
        );
        assert_eq!(
            bincode::serialize(&db.trader_stats.get(&addr(1)).unwrap()).unwrap(),
            live.1
        );
        assert_eq!(db.market_state.open_interest(0), live.2);
        assert_eq!(live.2, expected_oi);
        assert_eq!(db.market_state.open_interest(0), expected_oi);
        assert_exposure(&db);
        let stats = db.trader_stats.get(&addr(1)).unwrap();
        assert_eq!(stats.open_positions, if stage == 2 { 0 } else { 1 });
        assert_eq!(stats.total_trades, stage as u32);
        assert_eq!(stats.total_pnl, [0, 5_000, -20_000][stage]);
        assert_eq!(stats.liquidation_count, if stage == 2 { 1 } else { 0 });
    }
}

pub fn invalid_events_refused_before_cursor() {
    let dir = tempfile::tempdir().unwrap();
    let db = PerfDb::open(test_config(dir.path())).unwrap();
    for event in [
        Event::MarketCreated {
            market_id: 18,
            name: "Unsupported".into(),
            symbol: "UNSUPPORTED".into(),
        },
        Event::MarketCreated {
            market_id: 0,
            name: "Conflicting".into(),
            symbol: "OTHER".into(),
        },
        Event::VaultDeficit {
            token: token(1),
            deficit: -1,
        },
        Event::FundingSettled {
            market_id: 18,
            funding_rate: 1,
            long_payment: -1,
            short_payment: 1,
        },
    ] {
        assert_unsupported_before_cursor(&db, &make_event(1, event));
    }
    // Rejected input must leave the cursor and identity available for a real event.
    let valid = make_event(
        1,
        Event::VaultCreated {
            user: addr(1),
            vault: addr(2),
        },
    );
    db.process_event(&valid).unwrap();
    assert_event_effect(&db, &valid);
    db.sync_wal().unwrap();
    assert_wal_events(&db.config().wal.dir, &[valid]);
}

pub fn recorded_producer_roundtrip() {
    let records: std::collections::HashMap<String, Event> =
        serde_json::from_str(include_str!("../../src/fixtures/repair_1370_chain.json")).unwrap();
    let events: Vec<_> = [
        "position_opened",
        "position_modified",
        "funding_settled",
        "vault_deficit",
        "adl_executed",
    ]
    .iter()
    .enumerate()
    .map(|(i, name)| make_event(i as u64 + 1, records[*name].clone()))
    .collect();
    let (id, user) = match &events[0].event {
        Event::PositionOpened {
            position_id, user, ..
        } => (*position_id, *user),
        _ => unreachable!(),
    };
    let deficit_token = match &events[3].event {
        Event::VaultDeficit { token, .. } => *token,
        _ => unreachable!(),
    };
    let assert_state = |db: &PerfDb| {
        let position = db.positions.get(&id).unwrap();
        assert_eq!(position.size_usd, 917_760_000_000_000_000_000);
        assert_eq!(position.collateral_usd, 99 * SCALE);
        assert_eq!(position.collateral_amount, 99_000_000);
        assert_eq!(position.entry_price, 60_000 * SCALE);
        assert_eq!(position.status, PositionStatus::Open);
        assert_eq!(db.market_state.open_interest(0), (position.size_usd, 0));
        assert_exposure(db);
        let stats = db.trader_stats.get(&user).unwrap();
        assert_eq!(stats.total_pnl, 0);
        assert_eq!(stats.total_trades, 0);
        assert_eq!(stats.open_positions, 1);
        let funding = db.events.latest_funding_settlement(0).unwrap();
        assert_eq!(funding.funding_rate_hourly, 12_500_000_000_000);
        assert_eq!(funding.long_payment, -1_500_000_000_000_000_000);
        assert_eq!(funding.short_payment, 1_500_000_000_000_000_000);
        assert_eq!(funding.block_number, 3);
        assert_eq!(db.market_state.funding_rate(0), 0);
        let deficit = db.events.latest_vault_deficit(&deficit_token).unwrap();
        assert_eq!(deficit.deficit, 1_000_000_000);
        assert_eq!(deficit.block_number, 4);
        assert_eq!(db.events.total_count(), 5);
        assert_eq!(db.checkpoint.last_block(), 5);
    };
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    {
        let db = PerfDb::open(config.clone()).unwrap();
        for event in &events {
            db.process_event(event).unwrap();
            db.process_event(event).unwrap();
        }
        assert_state(&db);
        db.sync_wal().unwrap();
        assert_wal_events(&config.wal.dir, &events);
    }
    {
        let db = PerfDb::open(config.clone()).unwrap();
        assert_state(&db);
        db.snapshot().unwrap();
    }
    let db = PerfDb::open(config).unwrap();
    assert_state(&db);
}

pub fn snapshot_preserves_deleted_alert_allocator() {
    let dir = tempfile::tempdir().unwrap();
    let config = test_config(dir.path());
    {
        let db = PerfDb::open(config.clone()).unwrap();
        populate_relational_state(&db);
        let alert = db.alerts.get(1).unwrap();
        assert_eq!(db.alerts.insert(alert), 2);
        assert!(db.alerts.remove(2).is_some());
        db.snapshot().unwrap();
    }
    let db = PerfDb::open(config.clone()).unwrap();
    assert_relational_state(&db);
    assert_eq!(db.alerts.insert(db.alerts.get(1).unwrap()), 3);
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(config).unwrap();
    assert_eq!(db.alerts.count(), 2);
    assert_eq!(db.alerts.by_user(&addr(1)).len(), 2);
    assert!(db.alerts.get(2).is_none());
    assert!(db.alerts.get(3).is_some());
    assert_eq!(db.alerts.insert(db.alerts.get(1).unwrap()), 4);
}
