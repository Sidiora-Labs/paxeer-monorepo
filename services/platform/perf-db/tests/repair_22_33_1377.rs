//! Repair record 22.33.1377:
//! order-replacement-index: order_replacement_index_coordinates_and_status,
//! order_replacement_index_preserves_neighbors_and_lifecycle, order_replacement_index_concurrent.
//! position-replacement-index: position_replacement_index_coordinates_and_update,
//! position_replacement_index_concurrent_lookup.
//! Baseline verification was not run: the explicit gate order requires completed implementation
//! followed by hf-build and hf-done, and forbids running verify_cmd by hand.
//! No out-of-path wiring is required.

use std::sync::{Arc, Barrier};

use perfdb::core::types::{Order, OrderStatus, OrderType, Position, PositionStatus};
use perfdb::state::orders::{DepthLevel, OrderTable};
use perfdb::state::positions::PositionTable;

fn order() -> Order {
    Order {
        order_id: [7; 32],
        user_address: [1; 20],
        market_id: 0,
        is_long: true,
        order_type: OrderType::Limit,
        trigger_price: 100,
        limit_price: None,
        size_usd: 1_000,
        leverage: 10,
        collateral_token: [3; 20],
        collateral_amount: 100,
        status: OrderStatus::Active,
        execution_price: None,
        position_id: None,
        tx_hash: [4; 32],
        block_number: 10,
        created_at: 120,
        executed_at: None,
    }
}

fn position() -> Position {
    Position {
        position_id: [8; 32],
        user_address: [1; 20],
        market_id: 0,
        is_long: true,
        size_usd: 1_000,
        collateral_usd: 100,
        collateral_token: [3; 20],
        collateral_amount: 100,
        entry_price: 100,
        exit_price: None,
        realized_pnl: None,
        leverage: 10,
        status: PositionStatus::Open,
        open_tx: [4; 32],
        close_tx: None,
        open_block: 10,
        close_block: None,
        opened_at: 120,
        closed_at: None,
    }
}

fn assert_order(table: &OrderTable, expected: &Order) {
    let encoded = bincode::serialize(expected).unwrap();
    assert_eq!(
        bincode::serialize(&table.get(&expected.order_id).unwrap()).unwrap(),
        encoded
    );
    assert_eq!(table.count(), 1);
    assert_eq!(table.all().len(), 1);
    assert!(table.contains(&expected.order_id));
    let active = expected.status == OrderStatus::Active;
    assert_eq!(table.active_count(), usize::from(active));
    for user in [1, 2, 3] {
        let rows = table.by_user(&[user; 20]);
        if expected.user_address == [user; 20] {
            assert_eq!(rows.len(), 1);
            assert_eq!(bincode::serialize(&rows[0]).unwrap(), encoded);
        } else {
            assert!(rows.is_empty());
        }
        assert_eq!(
            table.active_by_user(&[user; 20]).len(),
            usize::from(active && expected.user_address == [user; 20])
        );
    }
    for market in 0..3 {
        let book = table.orderbook(market).unwrap();
        let level = vec![DepthLevel {
            price: expected.trigger_price,
            order_count: 1,
        }];
        if active && expected.market_id == market {
            if expected.is_long {
                assert_eq!(book.bids_depth(10), level);
                assert!(book.asks_depth(10).is_empty());
                assert_eq!(book.best_bid(), Some(expected.trigger_price));
                assert_eq!(book.best_ask(), None);
            } else {
                assert_eq!(book.asks_depth(10), level);
                assert!(book.bids_depth(10).is_empty());
                assert_eq!(book.best_ask(), Some(expected.trigger_price));
                assert_eq!(book.best_bid(), None);
            }
            assert_eq!(book.bid_count() + book.ask_count(), 1);
        } else {
            assert!(book.bids_depth(10).is_empty());
            assert!(book.asks_depth(10).is_empty());
            assert_eq!(book.bid_count() + book.ask_count(), 0);
            assert_eq!(book.best_bid(), None);
            assert_eq!(book.best_ask(), None);
        }
    }
}

#[test]
fn order_replacement_index_coordinates_and_status() {
    let table = OrderTable::new();
    let mut current = order();
    table.insert(current.clone());
    for step in 0..9 {
        match step {
            0 => {}
            1 => current.trigger_price = 110,
            2 => current.market_id = 1,
            3 => current.user_address = [2; 20],
            4 => current.is_long = false,
            5 => current.status = OrderStatus::Cancelled,
            6 => current.status = OrderStatus::Active,
            7 => {
                current.status = OrderStatus::Executed;
                current.execution_price = Some(115);
                current.position_id = Some([9; 32]);
                current.executed_at = Some(130);
            }
            8 => {
                current.user_address = [3; 20];
                current.market_id = 2;
                current.trigger_price = 120;
                current.is_long = true;
            }
            _ => unreachable!(),
        }
        for _ in 0..3 {
            table.insert(current.clone());
            assert_order(&table, &current);
        }
    }
    assert!(table.orderbook(u16::MAX).is_none());
    table.remove(&current.order_id).unwrap();
    assert_eq!(table.count(), 0);
    for user in [1, 2, 3] {
        assert!(table.by_user(&[user; 20]).is_empty());
    }
    for market in 0..3 {
        let book = table.orderbook(market).unwrap();
        assert_eq!(book.bid_count() + book.ask_count(), 0);
    }
}

#[test]
fn order_replacement_index_preserves_neighbors_and_lifecycle() {
    let table = OrderTable::new();
    let original = order();
    let mut neighbor = original.clone();
    neighbor.order_id = [9; 32];
    table.insert(original.clone());
    table.insert(neighbor.clone());
    let mut replacement = original.clone();
    replacement.user_address = [2; 20];
    replacement.market_id = 1;
    replacement.is_long = false;
    replacement.trigger_price = 200;
    table.insert(replacement.clone());
    assert_eq!(table.by_user(&original.user_address).len(), 1);
    assert_eq!(
        table.by_user(&original.user_address)[0].order_id,
        neighbor.order_id
    );
    assert_eq!(
        table.orderbook(0).unwrap().bids_depth(10),
        vec![DepthLevel {
            price: 100,
            order_count: 1
        }]
    );
    assert_eq!(
        table.cancel(&replacement.order_id).unwrap().status,
        OrderStatus::Cancelled
    );
    assert_eq!(table.orderbook(1).unwrap().ask_count(), 0);
    table.insert(replacement.clone());
    let executed = table
        .execute(&replacement.order_id, 205, [8; 32], 140)
        .unwrap();
    assert_eq!(executed.status, OrderStatus::Executed);
    assert_eq!(executed.execution_price, Some(205));
    assert_eq!(executed.position_id, Some([8; 32]));
    assert_eq!(executed.executed_at, Some(140));
    assert_eq!(table.orderbook(1).unwrap().ask_count(), 0);
    table.insert(replacement.clone());
    table.remove(&replacement.order_id).unwrap();
    assert!(table.by_user(&replacement.user_address).is_empty());
    assert_eq!(table.orderbook(1).unwrap().ask_count(), 0);
    assert_order(&table, &neighbor);
}

#[test]
fn order_replacement_index_concurrent() {
    let table = Arc::new(OrderTable::new());
    let barrier = Arc::new(Barrier::new(4));
    std::thread::scope(|scope| {
        for _ in 0..2 {
            scope.spawn(|| {
                barrier.wait();
                for round in 0..1_000 {
                    let mut value = order();
                    value.user_address = [1 + (round % 2) as u8; 20];
                    value.market_id = (round % 2) as u16;
                    value.is_long = round % 2 == 0;
                    value.trigger_price = 100 + round;
                    table.insert(value.clone());
                    table.insert(value);
                }
            });
        }
        for _ in 0..2 {
            scope.spawn(|| {
                barrier.wait();
                for _ in 0..1_000 {
                    for user in [1, 2] {
                        let rows = table.by_user(&[user; 20]);
                        assert!(rows.len() <= 1);
                        assert!(rows.iter().all(|row| row.user_address == [user; 20]));
                    }
                    for market in 0..2 {
                        let book = table.orderbook(market).unwrap();
                        assert!(book.bid_count() + book.ask_count() <= 1);
                    }
                }
            });
        }
    });
    let current = table.get(&order().order_id).unwrap();
    assert_order(&table, &current);
}

fn assert_position(table: &PositionTable, expected: &Position) {
    let encoded = bincode::serialize(expected).unwrap();
    assert_eq!(
        bincode::serialize(&table.get(&expected.position_id).unwrap()).unwrap(),
        encoded
    );
    assert_eq!(table.count(), 1);
    assert_eq!(table.all().len(), 1);
    assert!(table.contains(&expected.position_id));
    let open = expected.status == PositionStatus::Open;
    assert_eq!(table.open_count(), usize::from(open));
    for user in [1, 2, 3] {
        let rows = table.by_user(&[user; 20]);
        if expected.user_address == [user; 20] {
            assert_eq!(rows.len(), 1);
            assert_eq!(bincode::serialize(&rows[0]).unwrap(), encoded);
        } else {
            assert!(rows.is_empty());
        }
        assert_eq!(
            table.open_by_user(&[user; 20]).len(),
            usize::from(open && expected.user_address == [user; 20])
        );
    }
    for market in 0..3 {
        let rows = table.by_market(market);
        if expected.market_id == market {
            assert_eq!(rows.len(), 1);
            assert_eq!(bincode::serialize(&rows[0]).unwrap(), encoded);
        } else {
            assert!(rows.is_empty());
        }
        assert_eq!(
            table.open_by_market(market).len(),
            usize::from(open && expected.market_id == market)
        );
    }
}

#[test]
fn position_replacement_index_coordinates_and_update() {
    let table = PositionTable::new();
    let mut current = position();
    table.insert(current.clone());
    for step in 0..5 {
        match step {
            0 => {}
            1 => current.user_address = [2; 20],
            2 => current.market_id = 1,
            3 => {
                current.status = PositionStatus::Closed;
                current.exit_price = Some(110);
            }
            4 => {
                current.user_address = [3; 20];
                current.market_id = 2;
                current.status = PositionStatus::Liquidated;
            }
            _ => unreachable!(),
        }
        for _ in 0..3 {
            table.insert(current.clone());
            assert_position(&table, &current);
        }
    }
    assert!(table.update(&current.position_id, |row| {
        row.user_address = [1; 20];
        row.market_id = 0;
        row.status = PositionStatus::Open;
        row.size_usd = 2_000;
    }));
    current.user_address = [1; 20];
    current.market_id = 0;
    current.status = PositionStatus::Open;
    current.size_usd = 2_000;
    assert_position(&table, &current);
    let mut neighbor = position();
    neighbor.position_id = [9; 32];
    table.insert(neighbor.clone());
    table.remove(&current.position_id).unwrap();
    assert_position(&table, &neighbor);
    table.remove(&neighbor.position_id).unwrap();
    assert_eq!(table.count(), 0);
    for user in [1, 2, 3] {
        assert!(table.by_user(&[user; 20]).is_empty());
    }
    for market in 0..3 {
        assert!(table.by_market(market).is_empty());
    }
    assert!(!table.update(&current.position_id, |_| panic!(
        "missing row must not call update"
    )));
}

#[test]
fn position_replacement_index_concurrent_lookup() {
    let table = Arc::new(PositionTable::new());
    let barrier = Arc::new(Barrier::new(4));
    std::thread::scope(|scope| {
        for id in [8, 9] {
            let table = Arc::clone(&table);
            let barrier = Arc::clone(&barrier);
            scope.spawn(move || {
                barrier.wait();
                for round in 0..2_000 {
                    let mut value = position();
                    value.position_id = [id; 32];
                    value.market_id = (round % 2) as u16;
                    value.user_address = [1 + value.market_id as u8; 20];
                    table.insert(value.clone());
                    assert!(table.update(&value.position_id, |row| {
                        row.market_id = 1 - row.market_id;
                        row.user_address = [1 + row.market_id as u8; 20];
                        row.size_usd += 1;
                    }));
                    table.remove(&value.position_id).unwrap();
                }
            });
        }
        for _ in 0..2 {
            scope.spawn(|| {
                barrier.wait();
                for _ in 0..4_000 {
                    for user in [1, 2] {
                        let rows = table.by_user(&[user; 20]);
                        assert!(rows.len() <= 2);
                        assert!(rows.iter().all(|row| row.user_address == [user; 20]
                            && row.market_id == (user - 1) as u16));
                        let open = table.open_by_user(&[user; 20]);
                        assert!(open.iter().all(|row| row.user_address == [user; 20]
                            && row.status == PositionStatus::Open));
                    }
                    for market in 0..2 {
                        let rows = table.by_market(market);
                        assert!(rows.len() <= 2);
                        assert!(rows.iter().all(|row| row.market_id == market
                            && row.user_address == [1 + market as u8; 20]));
                    }
                }
            });
        }
    });
    assert_eq!(table.count(), 0);
    assert!(table.all().is_empty());
    for user in [1, 2] {
        assert!(table.by_user(&[user; 20]).is_empty());
    }
    for market in 0..2 {
        assert!(table.by_market(market).is_empty());
    }
}
