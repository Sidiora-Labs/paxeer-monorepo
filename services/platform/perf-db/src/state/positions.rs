use std::collections::{HashMap, HashSet};

use parking_lot::RwLock;

use crate::core::types::{Position, PositionStatus};

/// In-memory position table with secondary indexes.
///
/// Replaces DragonflyDB keys:
/// - `position:{id}` → `get()`
/// - `user:positions:{addr}` → `by_user()`
/// - `market:positions:{mid}` → `by_market()`
///
/// Concurrency: primary records and secondary indexes are published under one lock.
pub struct PositionTable {
    state: RwLock<PositionState>,
}

#[derive(Default)]
struct PositionState {
    positions: HashMap<[u8; 32], Position>,
    by_user: HashMap<[u8; 20], HashSet<[u8; 32]>>,
    by_market: HashMap<u16, HashSet<[u8; 32]>>,
}

impl PositionTable {
    pub fn new() -> Self {
        Self {
            state: RwLock::new(PositionState::default()),
        }
    }

    /// Insert or replace a position. Updates all indexes atomically.
    pub fn insert(&self, position: Position) {
        self.state.write().insert(position.position_id, position);
    }

    /// Get a position by ID. O(1).
    pub fn get(&self, position_id: &[u8; 32]) -> Option<Position> {
        self.state.read().positions.get(position_id).cloned()
    }

    /// Update a position in place. Returns `true` if the position exists.
    ///
    /// The callback runs under the table write lock. Keep it fast and do not
    /// call back into the table. Coordinate changes update indexes atomically.
    pub fn update(&self, position_id: &[u8; 32], f: impl FnOnce(&mut Position)) -> bool {
        let mut state = self.state.write();
        let Some(mut position) = state.positions.get(position_id).cloned() else {
            return false;
        };
        f(&mut position);
        state.insert(*position_id, position);
        true
    }

    /// Remove a position and clean up all indexes.
    pub fn remove(&self, position_id: &[u8; 32]) -> Option<Position> {
        let mut state = self.state.write();
        let position = state.positions.remove(position_id)?;
        state.remove_indexes(position_id, &position);
        Some(position)
    }

    /// All positions for a user (any status).
    pub fn by_user(&self, user_address: &[u8; 20]) -> Vec<Position> {
        let state = self.state.read();
        state.collect_from_index(&state.by_user, user_address)
    }

    /// All positions for a market (any status).
    pub fn by_market(&self, market_id: u16) -> Vec<Position> {
        let state = self.state.read();
        state.collect_from_index(&state.by_market, &market_id)
    }

    /// Open positions for a user.
    pub fn open_by_user(&self, user_address: &[u8; 20]) -> Vec<Position> {
        self.by_user(user_address)
            .into_iter()
            .filter(|p| matches!(p.status, PositionStatus::Open))
            .collect()
    }

    /// Open positions for a market.
    pub fn open_by_market(&self, market_id: u16) -> Vec<Position> {
        self.by_market(market_id)
            .into_iter()
            .filter(|p| matches!(p.status, PositionStatus::Open))
            .collect()
    }

    /// Total number of positions (all statuses).
    pub fn count(&self) -> usize {
        self.state.read().positions.len()
    }

    /// Number of open positions.
    pub fn open_count(&self) -> usize {
        self.state
            .read()
            .positions
            .values()
            .filter(|position| matches!(position.status, PositionStatus::Open))
            .count()
    }

    /// All positions (for iteration, diagnostics, snapshots).
    pub fn all(&self) -> Vec<Position> {
        self.state.read().positions.values().cloned().collect()
    }

    /// Check if a position exists.
    pub fn contains(&self, position_id: &[u8; 32]) -> bool {
        self.state.read().positions.contains_key(position_id)
    }
}

impl PositionState {
    fn insert(&mut self, position_id: [u8; 32], position: Position) {
        if let Some(previous) = self.positions.remove(&position_id) {
            self.remove_indexes(&position_id, &previous);
        }
        self.by_user
            .entry(position.user_address)
            .or_default()
            .insert(position_id);
        self.by_market
            .entry(position.market_id)
            .or_default()
            .insert(position_id);
        self.positions.insert(position_id, position);
    }

    fn remove_indexes(&mut self, position_id: &[u8; 32], position: &Position) {
        if let Some(ids) = self.by_user.get_mut(&position.user_address) {
            ids.remove(position_id);
            if ids.is_empty() {
                self.by_user.remove(&position.user_address);
            }
        }
        if let Some(ids) = self.by_market.get_mut(&position.market_id) {
            ids.remove(position_id);
            if ids.is_empty() {
                self.by_market.remove(&position.market_id);
            }
        }
    }

    fn collect_from_index<K: Eq + std::hash::Hash>(
        &self,
        index: &HashMap<K, HashSet<[u8; 32]>>,
        key: &K,
    ) -> Vec<Position> {
        match index.get(key) {
            Some(ids) => ids
                .iter()
                .filter_map(|pid| self.positions.get(pid).cloned())
                .collect(),
            None => Vec::new(),
        }
    }
}

impl Default for PositionTable {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_position(id: u8, user: u8, market: u16, status: PositionStatus) -> Position {
        Position {
            position_id: {
                let mut buf = [0u8; 32];
                buf[31] = id;
                buf
            },
            user_address: {
                let mut buf = [0u8; 20];
                buf[19] = user;
                buf
            },
            market_id: market,
            is_long: true,
            size_usd: 1_000_000_000_000_000_000_000,
            collateral_usd: 100_000_000_000_000_000_000,
            collateral_token: [0u8; 20],
            collateral_amount: 100_000_000,
            entry_price: 67_000_000_000_000_000_000_000,
            exit_price: None,
            realized_pnl: None,
            leverage: 10_000_000_000_000_000_000,
            status,
            open_tx: [0u8; 32],
            close_tx: None,
            open_block: 1000,
            close_block: None,
            opened_at: 1700000000,
            closed_at: None,
        }
    }

    fn pid(id: u8) -> [u8; 32] {
        let mut buf = [0u8; 32];
        buf[31] = id;
        buf
    }

    fn user(id: u8) -> [u8; 20] {
        let mut buf = [0u8; 20];
        buf[19] = id;
        buf
    }

    #[test]
    fn insert_and_get() {
        let table = PositionTable::new();
        let pos = make_position(1, 10, 0, PositionStatus::Open);
        table.insert(pos.clone());

        let got = table.get(&pid(1)).unwrap();
        assert_eq!(got.position_id, pid(1));
        assert_eq!(got.market_id, 0);
        assert!(got.is_long);
    }

    #[test]
    fn get_nonexistent() {
        let table = PositionTable::new();
        assert!(table.get(&pid(99)).is_none());
    }

    #[test]
    fn update_position() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));

        let updated = table.update(&pid(1), |p| {
            p.status = PositionStatus::Closed;
            p.exit_price = Some(68_000_000_000_000_000_000_000);
            p.closed_at = Some(1700001000);
        });
        assert!(updated);

        let got = table.get(&pid(1)).unwrap();
        assert!(matches!(got.status, PositionStatus::Closed));
        assert_eq!(got.exit_price, Some(68_000_000_000_000_000_000_000));
    }

    #[test]
    fn update_nonexistent() {
        let table = PositionTable::new();
        let updated = table.update(&pid(99), |_| {});
        assert!(!updated);
    }

    #[test]
    fn remove_position() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));

        let removed = table.remove(&pid(1)).unwrap();
        assert_eq!(removed.position_id, pid(1));

        assert!(table.get(&pid(1)).is_none());
        assert!(table.by_user(&user(10)).is_empty());
        assert!(table.by_market(0).is_empty());
    }

    #[test]
    fn remove_nonexistent() {
        let table = PositionTable::new();
        assert!(table.remove(&pid(99)).is_none());
    }

    #[test]
    fn by_user_index() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.insert(make_position(2, 10, 1, PositionStatus::Open));
        table.insert(make_position(3, 20, 0, PositionStatus::Open));

        let user10 = table.by_user(&user(10));
        assert_eq!(user10.len(), 2);

        let user20 = table.by_user(&user(20));
        assert_eq!(user20.len(), 1);

        let user99 = table.by_user(&user(99));
        assert!(user99.is_empty());
    }

    #[test]
    fn by_market_index() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.insert(make_position(2, 20, 0, PositionStatus::Open));
        table.insert(make_position(3, 10, 1, PositionStatus::Open));

        let mkt0 = table.by_market(0);
        assert_eq!(mkt0.len(), 2);

        let mkt1 = table.by_market(1);
        assert_eq!(mkt1.len(), 1);
    }

    #[test]
    fn open_by_user_filters_status() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.insert(make_position(2, 10, 1, PositionStatus::Closed));
        table.insert(make_position(3, 10, 2, PositionStatus::Liquidated));

        let open = table.open_by_user(&user(10));
        assert_eq!(open.len(), 1);
        assert_eq!(open[0].position_id, pid(1));
    }

    #[test]
    fn open_by_market_filters_status() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.insert(make_position(2, 20, 0, PositionStatus::Closed));

        let open = table.open_by_market(0);
        assert_eq!(open.len(), 1);
        assert_eq!(open[0].position_id, pid(1));
    }

    #[test]
    fn count_and_open_count() {
        let table = PositionTable::new();
        assert_eq!(table.count(), 0);
        assert_eq!(table.open_count(), 0);

        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.insert(make_position(2, 20, 0, PositionStatus::Closed));
        table.insert(make_position(3, 30, 1, PositionStatus::Open));

        assert_eq!(table.count(), 3);
        assert_eq!(table.open_count(), 2);
    }

    #[test]
    fn insert_replaces_existing() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));

        let mut updated = make_position(1, 10, 0, PositionStatus::Closed);
        updated.exit_price = Some(68_000);
        table.insert(updated);

        assert_eq!(table.count(), 1);
        let got = table.get(&pid(1)).unwrap();
        assert!(matches!(got.status, PositionStatus::Closed));
    }

    #[test]
    fn all_positions() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.insert(make_position(2, 20, 1, PositionStatus::Open));

        let all = table.all();
        assert_eq!(all.len(), 2);
    }

    #[test]
    fn contains() {
        let table = PositionTable::new();
        assert!(!table.contains(&pid(1)));

        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        assert!(table.contains(&pid(1)));
    }

    #[test]
    fn remove_cleans_indexes_fully() {
        let table = PositionTable::new();
        table.insert(make_position(1, 10, 0, PositionStatus::Open));
        table.remove(&pid(1));

        assert!(table.by_user(&user(10)).is_empty());
        assert!(table.by_market(0).is_empty());
        assert_eq!(table.count(), 0);
    }
}
