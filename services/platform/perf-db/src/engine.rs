use std::collections::HashMap;
use std::path::{Path, PathBuf};

use parking_lot::Mutex;

use crate::aggregation::candle_builder::{CandleBuilder, CandleOutput};
use crate::aggregation::leaderboard::{Leaderboard, LeaderboardEntry};
use crate::aggregation::stats::{DailyStatsStore, TraderStatsStore};
use crate::core::market::MarketId;
use crate::core::timeframe::Timeframe;
use crate::core::types::*;
use crate::error::PerfDbError;
use crate::relational::alerts::AlertStore;
use crate::relational::api_keys::ApiKeyStore;
use crate::relational::checkpoint::CheckpointStore;
use crate::relational::competitions::CompetitionStore;
use crate::relational::referrals::ReferralEngine;
use crate::relational::users::UserStore;
use crate::state::balances::BalanceCache;
use crate::state::market_state::MarketStateTable;
use crate::state::orders::OrderTable;
use crate::state::positions::PositionTable;
use crate::state::pubsub::{Channel, PubSubBus, PubSubMessage};
use crate::storage::candle_store::CandleStore;
use crate::storage::event_store::EventStore;
use crate::storage::snapshot::{
    self, DailyStatsEntry, SnapshotMarketState, SnapshotRecovery, StateSnapshot, TickCheckpoint,
};
use crate::storage::tick_store::TickStore;
use crate::storage::wal::{WalConfig, WalEntry, WalPosition, WalWriter};
use crate::storage::writer_lease::WriterLease;

#[derive(Default)]
struct MutationState {
    position: WalPosition,
    events: Vec<IndexedEvent>,
    identities: HashMap<([u8; 32], u32), usize>,
    slots: HashMap<(u64, u32, u32), [u8; 32]>,
    poisoned: bool,
}

impl MutationState {
    fn is_duplicate(&self, event: &IndexedEvent) -> Result<bool, PerfDbError> {
        let key = (event.tx_hash, event.log_index);
        if let Some(&index) = self.identities.get(&key) {
            let encode = |value: &IndexedEvent| {
                bincode::serialize(value).map_err(|e| PerfDbError::Serialization {
                    context: "event identity".into(),
                    detail: e.to_string(),
                })
            };
            if encode(&self.events[index])? != encode(event)? {
                return Err(PerfDbError::Corruption(
                    "conflicting event redelivery or block binding".into(),
                ));
            }
            return Ok(true);
        }
        if self
            .slots
            .contains_key(&(event.block_number, event.tx_index, event.log_index))
        {
            return Err(PerfDbError::Corruption(
                "event canonical block slot already occupied".into(),
            ));
        }
        Ok(false)
    }

    fn record(&mut self, event: IndexedEvent) {
        self.identities
            .insert((event.tx_hash, event.log_index), self.events.len());
        self.slots.insert(
            (event.block_number, event.tx_index, event.log_index),
            event.tx_hash,
        );
        self.events.push(event);
    }
}

/// Configuration for the PerfDB engine.
#[derive(Debug, Clone)]
pub struct PerfDbConfig {
    /// Root directory for all data files.
    pub data_dir: PathBuf,

    /// WAL configuration.
    pub wal: WalConfig,

    /// Maximum leaderboard entries per board.
    pub leaderboard_max: usize,

    /// Pub/sub channel capacity (per channel).
    pub pubsub_capacity: usize,
}

impl PerfDbConfig {
    /// Create a config pointing at the given data directory with defaults.
    pub fn new(data_dir: impl AsRef<Path>) -> Self {
        let data_dir = data_dir.as_ref().to_path_buf();
        Self {
            wal: WalConfig {
                dir: data_dir.join("wal"),
                ..WalConfig::default()
            },
            data_dir,
            leaderboard_max: 500,
            pubsub_capacity: 4096,
        }
    }

    fn ticks_dir(&self) -> PathBuf {
        self.data_dir.join("ticks")
    }

    fn candles_dir(&self) -> PathBuf {
        self.data_dir.join("candles")
    }

    pub fn snapshots_dir(&self) -> PathBuf {
        self.data_dir.join("snapshots")
    }
}

/// Top-level PerfDB engine. Coordinates all sub-engines.
///
/// Thread safety:
/// - Mutable time-series stores behind `Mutex` (single writer path)
/// - All state/relational stores are inherently concurrent (`DashMap`, `RwLock`)
/// - WAL writer behind `Mutex` (single writer)
/// - All reads are `&self`
pub struct PerfDb {
    config: PerfDbConfig,

    tick_store: Mutex<TickStore>,
    candle_store: Mutex<CandleStore>,
    candle_builder: Mutex<CandleBuilder>,

    pub market_state: MarketStateTable,
    pub positions: PositionTable,
    pub orders: OrderTable,
    pub balances: BalanceCache,
    pub pubsub: PubSubBus,

    pub users: UserStore,
    pub referrals: Mutex<ReferralEngine>,
    pub checkpoint: CheckpointStore,

    pub events: EventStore,

    pub trader_stats: TraderStatsStore,
    pub daily_stats: DailyStatsStore,
    pub leaderboard: Leaderboard,

    pub alerts: AlertStore,
    pub api_keys: ApiKeyStore,
    pub competitions: CompetitionStore,

    wal: Mutex<Option<WalWriter>>,
    mutation: Mutex<MutationState>,
    tick_position: Mutex<WalPosition>,
    read_only: bool,
    // Last field: release only after the writable stores/WAL have been dropped.
    _writer_lease: Option<WriterLease>,
}

impl PerfDb {
    /// Open or create a PerfDB instance at the configured data directory.
    ///
    /// Recovery order:
    /// 1. Load latest snapshot (if any)
    /// 2. Restore in-memory state from snapshot
    /// 3. Open WAL, replay entries after snapshot checkpoint
    /// 4. Open mmap stores (ticks, candles)
    pub fn open(config: PerfDbConfig) -> Result<Self, PerfDbError> {
        Self::open_mode(config, false)
    }

    /// Inspect the committed tick prefix and snapshot/WAL through read-only
    /// file handles. Recovery happens in private memory, including while a
    /// writer is active; it never repairs files or creates storage directories.
    pub fn open_read_only(config: PerfDbConfig) -> Result<Self, PerfDbError> {
        Self::open_mode(config, true)
    }

    fn open_mode(config: PerfDbConfig, read_only: bool) -> Result<Self, PerfDbError> {
        let writer_lease = if read_only {
            if !config.data_dir.is_dir() {
                return Err(PerfDbError::InvalidArgument(
                    "inspection requires an existing database".into(),
                ));
            }
            None
        } else {
            std::fs::create_dir_all(&config.data_dir).map_err(|e| PerfDbError::Io {
                context: format!("create data_dir: {}", config.data_dir.display()),
                source: e,
            })?;
            Some(WriterLease::acquire(&config.data_dir)?)
        };

        let snap = snapshot::load_latest_engine_snapshot(&config.snapshots_dir())?;
        let snapshot_position = snap
            .as_ref()
            .map(|(_, recovery)| recovery.position)
            .unwrap_or_default();
        let mut mutation = MutationState {
            position: snapshot_position,
            ..MutationState::default()
        };
        if let Some((_, recovery)) = &snap {
            for event in &recovery.events {
                if mutation.is_duplicate(event)? {
                    return Err(PerfDbError::Corruption(
                        "duplicate in snapshot event ledger".into(),
                    ));
                }
                mutation.record(event.clone());
            }
        }

        let tick_checkpoint = snapshot::load_tick_checkpoint(&config.data_dir)?;
        let tick_position = tick_checkpoint
            .as_ref()
            .map(|cp| cp.position)
            .unwrap_or_default();
        let tick_counts = tick_checkpoint
            .as_ref()
            .map(|cp| cp.tick_counts.clone())
            .unwrap_or_else(|| vec![0; crate::core::market::MARKET_COUNT]);
        let candle_counts = tick_checkpoint
            .as_ref()
            .map(|cp| cp.candle_counts.clone())
            .unwrap_or_else(|| vec![0; crate::core::market::MARKET_COUNT * 16]);
        let candle_tails = tick_checkpoint
            .as_ref()
            .map(|cp| cp.candle_tails.clone())
            .unwrap_or_else(|| vec![None; crate::core::market::MARKET_COUNT * 16]);

        let segments = crate::storage::wal::list_segments(&config.wal.dir)?;
        if let Some(first) = segments.iter().min() {
            if *first > snapshot_position.segment || *first > tick_position.segment {
                return Err(PerfDbError::Corruption(
                    "retained WAL does not cover recovery cursors".into(),
                ));
            }
        } else if snapshot_position != WalPosition::default()
            || tick_position != WalPosition::default()
        {
            return Err(PerfDbError::Corruption(
                "recovery cursors have no retained WAL".into(),
            ));
        }

        let (tick_store, candle_store, candle_builder) = if read_only {
            let ticks = TickStore::inspect(&config.ticks_dir(), &tick_counts)?;
            let mut candles = CandleStore::inspection();
            let mut builder = CandleBuilder::new();
            for mid in 0..crate::core::market::MARKET_COUNT as MarketId {
                for tick in ticks.ticks(mid) {
                    for output in builder.on_tick(tick) {
                        if let CandleOutput::Closed(candle) = output {
                            candles.upsert(&candle)?;
                        }
                    }
                }
                for tf in Timeframe::ALL {
                    let expected = candle_counts[mid as usize * 16 + tf as usize];
                    if let Some(tail) = candle_tails[mid as usize * 16 + tf as usize] {
                        candles.upsert(&tail)?;
                    }
                    if candles.candle_count(mid, tf) != expected {
                        return Err(PerfDbError::Corruption(
                            "candle prefix disagrees with tick checkpoint".into(),
                        ));
                    }
                }
            }
            (ticks, candles, builder)
        } else {
            let mut ticks = TickStore::open(config.ticks_dir())?;
            let mut candles = CandleStore::open(config.candles_dir())?;
            // Discard only suffixes beyond the last atomically published tick
            // cursor. Their WAL entries will be reapplied, even after another
            // interrupted recovery. Equal ticks remain separate occurrences.
            ticks.rewind(&tick_counts)?;
            candles.rewind(&candle_counts)?;
            let builder = tick_checkpoint.map(|cp| cp.builder).unwrap_or_default();
            // An uncommitted upsert may overwrite the previously stored live
            // bucket. Restore its exact bytes, which may differ from the
            // builder if further live ticks arrived after the last shutdown.
            for mid in 0..crate::core::market::MARKET_COUNT as MarketId {
                for tf in Timeframe::ALL {
                    if let Some(tail) = candle_tails[mid as usize * 16 + tf as usize] {
                        if !candles
                            .latest(mid, tf)
                            .is_some_and(|last| last.timestamp_ns == tail.timestamp_ns)
                        {
                            return Err(PerfDbError::Corruption(
                                "committed candle tail has moved".into(),
                            ));
                        }
                        candles.upsert(&tail)?;
                    }
                }
            }
            (ticks, candles, builder)
        };

        let market_state = MarketStateTable::new();
        let positions = PositionTable::new();
        let orders = OrderTable::new();
        let balances = BalanceCache::new();
        let pubsub = PubSubBus::with_capacity(config.pubsub_capacity);

        let users = UserStore::new();
        let referrals = ReferralEngine::new();
        let checkpoint = CheckpointStore::empty();

        let events = EventStore::new();
        let trader_stats = TraderStatsStore::new();
        let daily_stats = DailyStatsStore::new();
        let leaderboard = Leaderboard::new(config.leaderboard_max);
        let alerts = AlertStore::new();
        let api_keys = ApiKeyStore::new();
        let competitions = CompetitionStore::new();

        if let Some((snap, _)) = snap {
            restore_snapshot(
                &snap,
                &market_state,
                &positions,
                &orders,
                &balances,
                &users,
                &checkpoint,
                &trader_stats,
                &daily_stats,
            );
            tracing::info!(
                block = snap.checkpoint.last_block,
                positions = snap.positions.len(),
                users = snap.users.len(),
                "state restored from snapshot"
            );
        }

        let wal_writer = if read_only {
            None
        } else {
            Some(WalWriter::open(config.wal.clone())?)
        };
        if let Some(writer) = &wal_writer {
            if snapshot_position > writer.position() || tick_position > writer.position() {
                return Err(PerfDbError::Corruption(
                    "recovery cursor exceeds retained WAL".into(),
                ));
            }
        }

        let db = Self {
            config,
            tick_store: Mutex::new(tick_store),
            candle_store: Mutex::new(candle_store),
            candle_builder: Mutex::new(candle_builder),
            market_state,
            positions,
            orders,
            balances,
            pubsub,
            users,
            referrals: Mutex::new(referrals),
            checkpoint,
            events,
            trader_stats,
            daily_stats,
            leaderboard,
            alerts,
            api_keys,
            competitions,
            wal: Mutex::new(wal_writer),
            mutation: Mutex::new(mutation),
            tick_position: Mutex::new(tick_position),
            read_only,
            _writer_lease: writer_lease,
        };

        // Snapshot event history and identity share the same durable boundary.
        for event in &db.mutation.lock().events {
            let (market, trader) = extract_event_keys(&event.event);
            db.events.append(event.clone(), market, trader);
        }

        let wal_dir = db.config.wal.dir.clone();
        let replay_count = crate::storage::wal::replay_positioned(&wal_dir, |entry, position| {
            db.replay_wal_entry(&entry, position, snapshot_position)
        })?;

        if !read_only {
            let position = db.mutation.lock().position;
            db.persist_tick_checkpoint(position)?;
        }

        if replay_count > 0 {
            tracing::info!(
                replay_count,
                from_segment = snapshot_position.segment,
                "WAL entries replayed"
            );
        }

        Ok(db)
    }

    /// Ingest a raw price tick from the price feed.
    ///
    /// 1. Write to WAL
    /// 2. Append to TickStore (mmap)
    /// 3. Update MarketState (atomic)
    /// 4. Run CandleBuilder → emit Closed/Live candles
    /// 5. Persist Closed candles to CandleStore
    /// 6. Publish price update to PubSub
    ///
    /// Returns candle outputs for the caller to act on (e.g. stream live candles).
    pub fn ingest_tick(&self, tick: &PriceTick) -> Result<Vec<CandleOutput>, PerfDbError> {
        let mut mutation = self.mutation.lock();
        self.ensure_writable(&mutation)?;
        if tick.market_id as usize >= crate::core::market::MARKET_COUNT {
            return Err(PerfDbError::InvalidArgument(
                "market_id out of range".into(),
            ));
        }
        let result = (|| {
            let position = self.append_wal(&WalEntry::PriceTick {
                timestamp_ns: tick.timestamp_ns,
                market_id: tick.market_id,
                price: tick.price,
            })?;
            let outputs = self.apply_tick(tick)?;
            *self.tick_position.lock() = position;
            mutation.position = position;

            self.pubsub.publish(
                Channel::Prices,
                PubSubMessage::PriceUpdate {
                    market_id: tick.market_id,
                    price: tick.price,
                    timestamp_ns: tick.timestamp_ns,
                },
            );

            Ok(outputs)
        })();
        if result.is_err() {
            mutation.poisoned = true;
        }
        result
    }

    fn ensure_writable(&self, mutation: &MutationState) -> Result<(), PerfDbError> {
        if self.read_only {
            return Err(PerfDbError::InvalidArgument(
                "database inspection is read-only".into(),
            ));
        }
        if mutation.poisoned {
            return Err(PerfDbError::Corruption(
                "mutation failed; reopen database before writing or snapshotting".into(),
            ));
        }
        Ok(())
    }

    fn append_wal(&self, entry: &WalEntry) -> Result<WalPosition, PerfDbError> {
        let mut wal = self.wal.lock();
        let writer = wal.as_mut().ok_or_else(|| {
            PerfDbError::InvalidArgument("database inspection is read-only".into())
        })?;
        writer.append(entry)?;
        Ok(writer.position())
    }

    fn apply_tick(&self, tick: &PriceTick) -> Result<Vec<CandleOutput>, PerfDbError> {
        self.tick_store.lock().append(tick)?;
        let outputs = self.candle_builder.lock().on_tick(tick);
        let mut candles = self.candle_store.lock();
        for output in &outputs {
            if let CandleOutput::Closed(candle) = output {
                candles.upsert(candle)?;
            }
        }
        self.market_state
            .set_price(tick.market_id, tick.price, tick.timestamp_ns);
        Ok(outputs)
    }

    // Called only while mutation is excluded (or before open returns). Publish
    // a cursor only after WAL and both derived stores are durable. A crash
    // before publication leaves a suffix that recovery rewinds and replays.
    fn persist_tick_checkpoint(&self, position: WalPosition) -> Result<(), PerfDbError> {
        self.wal
            .lock()
            .as_mut()
            .ok_or_else(|| PerfDbError::InvalidArgument("database inspection is read-only".into()))?
            .sync()?;
        let ticks = self.tick_store.lock();
        let candles = self.candle_store.lock();
        ticks.flush()?;
        candles.flush()?;
        snapshot::write_tick_checkpoint(
            &self.config.data_dir,
            &TickCheckpoint {
                version: 1,
                position,
                tick_counts: ticks.counts(),
                candle_counts: candles.counts(),
                candle_tails: (0..crate::core::market::MARKET_COUNT as MarketId)
                    .flat_map(|mid| {
                        Timeframe::ALL
                            .into_iter()
                            .map(|tf| candles.latest(mid, tf).copied())
                            .collect::<Vec<_>>()
                    })
                    .collect(),
                builder: self.candle_builder.lock().clone(),
            },
        )?;
        *self.tick_position.lock() = position;
        Ok(())
    }

    /// Process a decoded chain event from the indexer.
    ///
    /// Updates all relevant stores based on the event variant.
    /// Writes to WAL, updates state, publishes to PubSub.
    pub fn process_event(&self, indexed: &IndexedEvent) -> Result<(), PerfDbError> {
        let mut mutation = self.mutation.lock();
        self.ensure_writable(&mutation)?;
        if mutation.is_duplicate(indexed)? {
            return Ok(());
        }
        let position = match self.append_wal(&WalEntry::IndexedEvent(indexed.clone())) {
            Ok(position) => position,
            Err(error) => {
                mutation.poisoned = true;
                return Err(error);
            }
        };

        self.apply_event(indexed);
        mutation.record(indexed.clone());
        mutation.position = position;
        Ok(())
    }

    /// Apply an indexed event to every store. Shared by live processing and
    /// WAL replay so recovery reconstructs exactly the live state.
    fn apply_event(&self, indexed: &IndexedEvent) {
        let (market_id, trader) = extract_event_keys(&indexed.event);

        self.events.append(indexed.clone(), market_id, trader);

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
                let pos = Position {
                    position_id: *position_id,
                    user_address: *user,
                    market_id: *market_id,
                    is_long: *is_long,
                    size_usd: *size_usd,
                    collateral_usd: *size_usd / (*leverage).max(1),
                    collateral_token: *collateral_token,
                    collateral_amount: *collateral_amount,
                    entry_price: *entry_price,
                    exit_price: None,
                    realized_pnl: None,
                    leverage: *leverage,
                    status: PositionStatus::Open,
                    open_tx: indexed.tx_hash,
                    close_tx: None,
                    open_block: indexed.block_number,
                    close_block: None,
                    opened_at: indexed.block_timestamp,
                    closed_at: None,
                };
                self.positions.insert(pos.clone());
                self.users.ensure(*user, indexed.block_timestamp);
                self.trader_stats.ensure(*user);
                self.trader_stats.increment_open_positions(user);

                if *is_long {
                    self.market_state.add_long_oi(*market_id, *size_usd);
                } else {
                    self.market_state.add_short_oi(*market_id, *size_usd);
                }

                self.pubsub.publish(
                    Channel::Positions,
                    PubSubMessage::PositionUpdate(Box::new(pos)),
                );
            }

            Event::PositionClosed {
                position_id,
                user,
                market_id,
                closed_size_usd,
                exit_price,
                realized_pnl,
                is_full_close,
            } => {
                let is_win = *realized_pnl > 0;
                self.positions.update(position_id, |p| {
                    p.exit_price = Some(*exit_price);
                    p.realized_pnl = Some(*realized_pnl);
                    p.close_tx = Some(indexed.tx_hash);
                    p.close_block = Some(indexed.block_number);
                    p.closed_at = Some(indexed.block_timestamp);
                    if *is_full_close {
                        p.status = PositionStatus::Closed;
                    } else {
                        p.size_usd -= *closed_size_usd;
                    }
                });

                self.trader_stats.ensure(*user);
                self.trader_stats.increment_trade(
                    user,
                    *closed_size_usd,
                    *realized_pnl,
                    0,
                    is_win,
                    false,
                    indexed.block_timestamp,
                );
                if *is_full_close {
                    self.trader_stats.decrement_open_positions(user);
                }

                let date = timestamp_to_date(indexed.block_timestamp);
                self.daily_stats
                    .accumulate_trade(*user, date, *closed_size_usd, *realized_pnl, 0);

                if let Some(pos) = self.positions.get(position_id) {
                    if pos.is_long {
                        self.market_state.add_long_oi(*market_id, -closed_size_usd);
                    } else {
                        self.market_state.add_short_oi(*market_id, -closed_size_usd);
                    }

                    self.pubsub.publish(
                        Channel::Positions,
                        PubSubMessage::PositionUpdate(Box::new(pos)),
                    );
                }
            }

            Event::PositionModified {
                position_id,
                new_size_usd,
                new_collateral_usd,
                new_collateral_amount,
            } => {
                self.positions.update(position_id, |p| {
                    p.size_usd = *new_size_usd;
                    p.collateral_usd = *new_collateral_usd;
                    p.collateral_amount = *new_collateral_amount;
                });

                if let Some(pos) = self.positions.get(position_id) {
                    self.pubsub.publish(
                        Channel::Positions,
                        PubSubMessage::PositionUpdate(Box::new(pos)),
                    );
                }
            }

            Event::OrderPlaced {
                order_id,
                user,
                market_id,
                order_type,
                is_long,
                trigger_price,
                size_usd,
            } => {
                let order = Order {
                    order_id: *order_id,
                    user_address: *user,
                    market_id: *market_id,
                    is_long: *is_long,
                    order_type: OrderType::from(*order_type),
                    trigger_price: *trigger_price,
                    limit_price: None,
                    size_usd: *size_usd,
                    leverage: 0,
                    collateral_token: [0u8; 20],
                    collateral_amount: 0,
                    status: OrderStatus::Active,
                    execution_price: None,
                    position_id: None,
                    tx_hash: indexed.tx_hash,
                    block_number: indexed.block_number,
                    created_at: indexed.block_timestamp,
                    executed_at: None,
                };
                self.orders.insert(order.clone());
                self.users.ensure(*user, indexed.block_timestamp);

                self.pubsub
                    .publish(Channel::Orders, PubSubMessage::OrderUpdate(Box::new(order)));
            }

            Event::OrderExecuted {
                order_id,
                position_id,
                execution_price,
            } => {
                if let Some(order) = self.orders.execute(
                    order_id,
                    *execution_price,
                    *position_id,
                    indexed.block_timestamp,
                ) {
                    self.pubsub
                        .publish(Channel::Orders, PubSubMessage::OrderUpdate(Box::new(order)));
                }
            }

            Event::OrderCancelled { order_id, .. } => {
                if let Some(order) = self.orders.cancel(order_id) {
                    self.pubsub
                        .publish(Channel::Orders, PubSubMessage::OrderUpdate(Box::new(order)));
                }
            }

            Event::Liquidation {
                position_id,
                user,
                market_id,
                liquidation_price,
                penalty,
                keeper: _,
            } => {
                self.positions.update(position_id, |p| {
                    p.status = PositionStatus::Liquidated;
                    p.exit_price = Some(*liquidation_price);
                    p.close_tx = Some(indexed.tx_hash);
                    p.close_block = Some(indexed.block_number);
                    p.closed_at = Some(indexed.block_timestamp);
                });

                self.trader_stats.ensure(*user);
                let neg_penalty = -(*penalty);
                self.trader_stats.increment_trade(
                    user,
                    0,
                    neg_penalty,
                    0,
                    false,
                    true,
                    indexed.block_timestamp,
                );
                self.trader_stats.decrement_open_positions(user);

                self.pubsub.publish(
                    Channel::Liquidations,
                    PubSubMessage::LiquidationEvent {
                        position_id: *position_id,
                        user: *user,
                        market_id: *market_id,
                        liquidation_price: *liquidation_price,
                        penalty: *penalty,
                    },
                );
            }

            Event::FundingRateUpdated {
                market_id,
                new_rate_per_second,
                funding_rate_24h,
            } => {
                self.market_state.set_funding_rate(
                    *market_id,
                    *new_rate_per_second,
                    *funding_rate_24h,
                );

                self.pubsub.publish(
                    Channel::Funding,
                    PubSubMessage::FundingUpdate {
                        market_id: *market_id,
                        rate_per_second: *new_rate_per_second,
                        rate_24h: *funding_rate_24h,
                    },
                );
            }

            Event::CollateralDeposited {
                user,
                token,
                amount,
            } => {
                self.users.ensure(*user, indexed.block_timestamp);
                let existing = self.balances.get(user, token);
                let new_amount = existing.as_ref().map_or(*amount, |b| b.amount + *amount);
                let new_locked = existing.as_ref().map_or(0, |b| b.locked);
                self.balances.set_raw(
                    *user,
                    *token,
                    new_amount,
                    new_locked,
                    new_amount - new_locked,
                );

                if let Some(bal) = self.balances.get(user, token) {
                    self.pubsub.publish(
                        Channel::Balances,
                        PubSubMessage::BalanceUpdate(Box::new(bal)),
                    );
                }
            }

            Event::CollateralWithdrawn {
                user,
                token,
                amount,
            } => {
                let existing = self.balances.get(user, token);
                let new_amount = existing.as_ref().map_or(0, |b| (b.amount - *amount).max(0));
                let new_locked = existing.as_ref().map_or(0, |b| b.locked);
                self.balances.set_raw(
                    *user,
                    *token,
                    new_amount,
                    new_locked,
                    (new_amount - new_locked).max(0),
                );

                if let Some(bal) = self.balances.get(user, token) {
                    self.pubsub.publish(
                        Channel::Balances,
                        PubSubMessage::BalanceUpdate(Box::new(bal)),
                    );
                }
            }

            Event::VaultCreated { user, vault } => {
                self.users.ensure(*user, indexed.block_timestamp);
                self.users.set_vault(user, *vault);
            }

            Event::MarketPaused { market_id } => {
                self.market_state.set_paused(*market_id, true);
            }

            Event::PriceUpdated { .. }
            | Event::FundingSettled { .. }
            | Event::MarketCreated { .. }
            | Event::ADLExecuted { .. }
            | Event::VaultDeficit { .. } => {}
        }

        self.checkpoint
            .advance_block(self.checkpoint.last_block().max(indexed.block_number));
    }

    /// Replay a single WAL entry into state (used during recovery).
    fn replay_wal_entry(
        &self,
        entry: &WalEntry,
        position: WalPosition,
        snapshot_position: WalPosition,
    ) -> Result<(), PerfDbError> {
        let mut mutation = self.mutation.lock();
        match entry {
            WalEntry::PriceTick {
                market_id,
                price,
                timestamp_ns,
            } => {
                if position > *self.tick_position.lock() {
                    self.apply_tick(&PriceTick {
                        market_id: *market_id,
                        price: *price,
                        timestamp_ns: *timestamp_ns,
                        _pad: [0; 6],
                    })?;
                    *self.tick_position.lock() = position;
                } else if position > snapshot_position {
                    self.market_state
                        .set_price(*market_id, *price, *timestamp_ns);
                }
            }

            WalEntry::Checkpoint { block_number } => {
                if position > snapshot_position {
                    self.checkpoint
                        .advance_block(self.checkpoint.last_block().max(*block_number));
                }
            }

            WalEntry::BalanceUpdate {
                user,
                token,
                amount,
                locked,
                available,
            } => {
                if position > snapshot_position {
                    self.balances
                        .set_raw(*user, *token, *amount, *locked, *available);
                }
            }

            WalEntry::IndexedEvent(indexed) => {
                let duplicate = mutation.is_duplicate(indexed)?;
                if position > snapshot_position && !duplicate {
                    self.apply_event(indexed);
                    mutation.record(indexed.clone());
                } else if position <= snapshot_position && !duplicate {
                    return Err(PerfDbError::Corruption(
                        "snapshot event ledger does not cover its WAL cursor".into(),
                    ));
                }
            }

            WalEntry::CandleUpdate { .. } | WalEntry::BatchEnd => {}
        }
        mutation.position = mutation.position.max(position);
        Ok(())
    }

    /// Update the checkpoint to a specific block (called at batch boundaries).
    pub fn save_checkpoint(&self, block: u64, hash: String, timestamp: u64) {
        let _mutation = self.mutation.lock();
        self.checkpoint.save(block, hash, timestamp);
    }

    /// Zero-copy tick slice for a market. Sorted by timestamp.
    pub fn ticks(&self, market_id: MarketId) -> Vec<PriceTick> {
        self.tick_store.lock().ticks(market_id).to_vec()
    }

    /// Tick count for a market.
    pub fn tick_count(&self, market_id: MarketId) -> u64 {
        self.tick_store.lock().tick_count(market_id)
    }

    /// Latest tick for a market.
    pub fn latest_tick(&self, market_id: MarketId) -> Option<PriceTick> {
        self.tick_store.lock().latest(market_id).copied()
    }

    /// Stored candles for a (market, timeframe) pair. Sorted by timestamp.
    pub fn candles(&self, market_id: MarketId, tf: Timeframe) -> Vec<CandleRecord> {
        self.candle_store.lock().candles(market_id, tf).to_vec()
    }

    /// Candle count for a (market, timeframe) pair.
    pub fn candle_count(&self, market_id: MarketId, tf: Timeframe) -> u64 {
        self.candle_store.lock().candle_count(market_id, tf)
    }

    /// Current live (in-progress) candle for a (market, timeframe).
    pub fn live_candle(&self, market_id: MarketId, tf: Timeframe) -> Option<CandleRecord> {
        self.candle_builder.lock().current(market_id, tf)
    }

    /// Latest stored price for a market (lock-free, < 10ns).
    pub fn latest_price(&self, market_id: MarketId) -> f64 {
        self.market_state.latest_price(market_id)
    }

    /// All latest prices (non-zero).
    pub fn all_prices(&self) -> Vec<(MarketId, f64)> {
        self.market_state.all_prices()
    }

    /// Rebuild leaderboards from current stats. Call periodically.
    pub fn rebuild_leaderboard(&self, now: u64) {
        let stats = self.trader_stats.all();
        self.leaderboard.rebuild(&stats, now);
    }

    /// PnL leaderboard.
    pub fn pnl_leaderboard(&self, limit: usize) -> Vec<LeaderboardEntry> {
        self.leaderboard.pnl_board(limit)
    }

    /// Volume leaderboard.
    pub fn volume_leaderboard(&self, limit: usize) -> Vec<LeaderboardEntry> {
        self.leaderboard.volume_board(limit)
    }

    /// Create a state snapshot and write it to disk.
    pub fn snapshot(&self) -> Result<PathBuf, PerfDbError> {
        let mut mutation = self.mutation.lock();
        self.ensure_writable(&mutation)?;
        let result = (|| {
            self.persist_tick_checkpoint(mutation.position)?;
            self.snapshot_locked(&mutation)
        })();
        if result.is_err() {
            mutation.poisoned = true;
        }
        result
    }

    fn snapshot_locked(&self, mutation: &MutationState) -> Result<PathBuf, PerfDbError> {
        let cp = self.checkpoint.snapshot();

        let daily_entries: Vec<DailyStatsEntry> = {
            let all_trader_stats = self.trader_stats.all();
            let mut entries = Vec::new();
            for ts in &all_trader_stats {
                let history = self.daily_stats.trader_daily_history(&ts.address, 1000);
                for ds in history {
                    entries.push(DailyStatsEntry {
                        address: ts.address,
                        stats: ds,
                    });
                }
            }
            entries
        };

        let market_states: Vec<SnapshotMarketState> = (0..crate::core::market::MARKET_COUNT)
            .filter_map(|i| {
                self.market_state
                    .snapshot(i as MarketId)
                    .map(SnapshotMarketState::from)
            })
            .collect();

        let snap = StateSnapshot {
            checkpoint: cp.into(),
            positions: self.positions.all(),
            orders: self.orders.all(),
            balances: self.balances.all(),
            users: self.users.all(),
            trader_stats: self.trader_stats.all(),
            daily_stats: daily_entries,
            market_states,
            snapshot_timestamp: self.checkpoint.last_block_timestamp(),
        };

        snapshot::write_engine_snapshot(
            &self.config.snapshots_dir(),
            &snap,
            SnapshotRecovery {
                position: mutation.position,
                events: mutation.events.clone(),
            },
        )
    }

    /// Graceful shutdown: flush all stores, sync WAL, create final snapshot.
    pub fn shutdown(&self) -> Result<(), PerfDbError> {
        let mut mutation = self.mutation.lock();
        self.ensure_writable(&mutation)?;
        let result = (|| {
            {
                let builder = self.candle_builder.lock();
                let mut cs = self.candle_store.lock();
                for candle in builder.flush_all() {
                    cs.upsert(&candle)?;
                }
                cs.flush()?;
            }

            self.persist_tick_checkpoint(mutation.position)?;
            self.snapshot_locked(&mutation)?;

            tracing::info!("PerfDB shutdown complete");
            Ok(())
        })();
        if result.is_err() {
            mutation.poisoned = true;
        }
        result
    }

    /// Sync the WAL to disk (call periodically or at batch boundaries).
    pub fn sync_wal(&self) -> Result<(), PerfDbError> {
        let mut mutation = self.mutation.lock();
        self.ensure_writable(&mutation)?;
        let result = self.persist_tick_checkpoint(mutation.position);
        if result.is_err() {
            mutation.poisoned = true;
        }
        result
    }

    /// Access config.
    pub fn config(&self) -> &PerfDbConfig {
        &self.config
    }
}

/// Extract routing keys (market_id, trader_address) from an event for EventStore.
fn extract_event_keys(event: &Event) -> (Option<u16>, Option<[u8; 20]>) {
    match event {
        Event::PositionOpened {
            market_id, user, ..
        } => (Some(*market_id), Some(*user)),
        Event::PositionClosed {
            market_id, user, ..
        } => (Some(*market_id), Some(*user)),
        Event::PositionModified { .. } => (None, None),
        Event::OrderPlaced {
            market_id, user, ..
        } => (Some(*market_id), Some(*user)),
        Event::OrderExecuted { .. } => (None, None),
        Event::OrderCancelled { user, .. } => (None, Some(*user)),
        Event::Liquidation {
            market_id, user, ..
        } => (Some(*market_id), Some(*user)),
        Event::ADLExecuted { .. } => (None, None),
        Event::PriceUpdated { market_id, .. } => (Some(*market_id), None),
        Event::FundingSettled { market_id, .. } => (Some(*market_id), None),
        Event::FundingRateUpdated { market_id, .. } => (Some(*market_id), None),
        Event::MarketCreated { market_id, .. } => (Some(*market_id), None),
        Event::MarketPaused { market_id } => (Some(*market_id), None),
        Event::CollateralDeposited { user, .. } => (None, Some(*user)),
        Event::CollateralWithdrawn { user, .. } => (None, Some(*user)),
        Event::VaultCreated { user, .. } => (None, Some(*user)),
        Event::VaultDeficit { .. } => (None, None),
    }
}

/// Restore in-memory state from a snapshot.
fn restore_snapshot(
    snap: &StateSnapshot,
    market_state: &MarketStateTable,
    positions: &PositionTable,
    orders: &OrderTable,
    balances: &BalanceCache,
    users: &UserStore,
    checkpoint: &CheckpointStore,
    trader_stats: &TraderStatsStore,
    daily_stats: &DailyStatsStore,
) {
    checkpoint.save(
        snap.checkpoint.last_block,
        snap.checkpoint.last_block_hash.clone(),
        snap.checkpoint.last_block_timestamp,
    );

    for pos in &snap.positions {
        positions.insert(pos.clone());
    }

    for order in &snap.orders {
        orders.insert(order.clone());
    }

    for bal in &snap.balances {
        balances.set(bal.clone());
    }

    for user in &snap.users {
        users.upsert(user.clone());
    }

    for ts in &snap.trader_stats {
        trader_stats.ensure(ts.address);
        trader_stats.update(&ts.address, |s| *s = ts.clone());
    }

    for entry in &snap.daily_stats {
        daily_stats.set_trader_daily(entry.address, entry.stats.date, entry.stats.clone());
    }

    for ms in &snap.market_states {
        market_state.set_price(ms.market_id, ms.latest_price, ms.last_update_ns);
        market_state.set_funding_rate(
            ms.market_id,
            ms.funding_rate_per_second,
            ms.funding_rate_24h,
        );
        market_state.set_open_interest(ms.market_id, ms.long_oi, ms.short_oi);
        market_state.set_oracle_prices(ms.market_id, ms.mark_price, ms.index_price);
        market_state.set_config(ms.market_id, ms.max_leverage, ms.maintenance_margin_bps);
        market_state.set_enabled(ms.market_id, ms.enabled);
        market_state.set_paused(ms.market_id, ms.paused);
        market_state.set_24h_stats(
            ms.market_id,
            ms.volume_24h,
            ms.trades_24h,
            ms.price_change_24h,
            ms.price_change_pct_24h,
        );
    }
}

/// Convert a unix timestamp (seconds) to YYYYMMDD u32.
fn timestamp_to_date(ts: u64) -> u32 {
    // civil_from_days algorithm: https://howardhinnant.github.io/date_algorithms.html
    let days = ts / 86400;
    let z = days + 719468;
    let era = z / 146097;
    let doe = z - era * 146097;
    let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let d = doy - (153 * mp + 2) / 5 + 1;
    let m = if mp < 10 { mp + 3 } else { mp - 9 };
    let y = if m <= 2 { y + 1 } else { y };
    (y as u32) * 10000 + (m as u32) * 100 + (d as u32)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_config(dir: &Path) -> PerfDbConfig {
        PerfDbConfig {
            data_dir: dir.to_path_buf(),
            wal: WalConfig {
                dir: dir.join("wal"),
                max_segment_size: 64 * 1024 * 1024,
                sync_policy: crate::storage::wal::SyncPolicy::None,
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

    fn make_indexed_event(block: u64, event: Event) -> IndexedEvent {
        IndexedEvent {
            block_number: block,
            block_timestamp: block * 12,
            tx_hash: {
                let mut buf = [0u8; 32];
                buf[31] = block as u8;
                buf
            },
            tx_index: 0,
            log_index: 0,
            event,
        }
    }

    #[test]
    fn open_and_shutdown() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();
        db.shutdown().unwrap();
    }

    #[test]
    fn open_creates_directories() {
        let dir = tempfile::tempdir().unwrap();
        let data_dir = dir.path().join("perfdb_data");
        let _ = PerfDb::open(PerfDbConfig::new(&data_dir)).unwrap();

        assert!(data_dir.join("ticks").exists());
        assert!(data_dir.join("candles").exists());
        assert!(data_dir.join("wal").exists());
    }

    #[test]
    fn ingest_tick_stores_and_updates_price() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        let tick = make_tick(1_000_000_000, 0, 67_000.0);
        let outputs = db.ingest_tick(&tick).unwrap();

        assert_eq!(db.tick_count(0), 1);
        assert_eq!(db.latest_price(0), 67_000.0);
        assert!(!outputs.is_empty());
    }

    #[test]
    fn ingest_multiple_ticks() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        for i in 0..100u64 {
            db.ingest_tick(&make_tick(i * 1_000_000_000, 0, 67_000.0 + i as f64))
                .unwrap();
        }

        assert_eq!(db.tick_count(0), 100);
        assert_eq!(db.latest_price(0), 67_099.0);
        assert!(db.live_candle(0, Timeframe::Min1).is_some());
    }

    #[test]
    fn ingest_tick_generates_closed_candles() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.ingest_tick(&make_tick(500_000_000, 0, 67_000.0))
            .unwrap();
        let outputs = db
            .ingest_tick(&make_tick(1_500_000_000, 0, 67_100.0))
            .unwrap();

        let closed: Vec<_> = outputs
            .iter()
            .filter(|o| matches!(o, CandleOutput::Closed(_)))
            .collect();
        assert!(!closed.is_empty());
        assert!(db.candle_count(0, Timeframe::Sec1) >= 1);
    }

    #[test]
    fn ingest_tick_multi_market() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.ingest_tick(&make_tick(1_000, 0, 67_000.0)).unwrap();
        db.ingest_tick(&make_tick(1_000, 1, 3_500.0)).unwrap();

        assert_eq!(db.latest_price(0), 67_000.0);
        assert_eq!(db.latest_price(1), 3_500.0);
        assert_eq!(db.tick_count(0), 1);
        assert_eq!(db.tick_count(1), 1);
    }

    #[test]
    fn process_position_opened() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        let event = make_indexed_event(
            100,
            Event::PositionOpened {
                position_id: pid(1),
                user: addr(10),
                market_id: 0,
                is_long: true,
                size_usd: 1_000_000,
                leverage: 10,
                entry_price: 67_000,
                collateral_token: [0u8; 20],
                collateral_amount: 100_000,
            },
        );

        db.process_event(&event).unwrap();

        assert!(db.positions.get(&pid(1)).is_some());
        assert!(db.users.contains(&addr(10)));
        assert_eq!(db.trader_stats.get(&addr(10)).unwrap().open_positions, 1);
        assert_eq!(db.checkpoint.last_block(), 100);
        assert_eq!(db.events.total_count(), 1);
    }

    #[test]
    fn process_position_closed() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.process_event(&make_indexed_event(
            100,
            Event::PositionOpened {
                position_id: pid(1),
                user: addr(10),
                market_id: 0,
                is_long: true,
                size_usd: 1_000_000,
                leverage: 10,
                entry_price: 67_000,
                collateral_token: [0u8; 20],
                collateral_amount: 100_000,
            },
        ))
        .unwrap();

        db.process_event(&make_indexed_event(
            200,
            Event::PositionClosed {
                position_id: pid(1),
                user: addr(10),
                market_id: 0,
                closed_size_usd: 1_000_000,
                exit_price: 68_000,
                realized_pnl: 15_000,
                is_full_close: true,
            },
        ))
        .unwrap();

        let pos = db.positions.get(&pid(1)).unwrap();
        assert!(matches!(pos.status, PositionStatus::Closed));
        assert_eq!(pos.exit_price, Some(68_000));

        let stats = db.trader_stats.get(&addr(10)).unwrap();
        assert_eq!(stats.total_trades, 1);
        assert_eq!(stats.total_pnl, 15_000);
        assert_eq!(stats.win_count, 1);
        assert_eq!(stats.open_positions, 0);
    }

    #[test]
    fn process_order_lifecycle() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.process_event(&make_indexed_event(
            100,
            Event::OrderPlaced {
                order_id: oid(1),
                user: addr(10),
                market_id: 0,
                order_type: 0,
                is_long: true,
                trigger_price: 66_000,
                size_usd: 500_000,
            },
        ))
        .unwrap();

        assert!(db.orders.get(&oid(1)).is_some());
        assert!(matches!(
            db.orders.get(&oid(1)).unwrap().status,
            OrderStatus::Active
        ));

        db.process_event(&make_indexed_event(
            200,
            Event::OrderExecuted {
                order_id: oid(1),
                position_id: pid(5),
                execution_price: 66_050,
            },
        ))
        .unwrap();

        let order = db.orders.get(&oid(1)).unwrap();
        assert!(matches!(order.status, OrderStatus::Executed));
        assert_eq!(order.execution_price, Some(66_050));
    }

    #[test]
    fn process_order_cancelled() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.process_event(&make_indexed_event(
            100,
            Event::OrderPlaced {
                order_id: oid(2),
                user: addr(10),
                market_id: 0,
                order_type: 0,
                is_long: false,
                trigger_price: 68_000,
                size_usd: 300_000,
            },
        ))
        .unwrap();

        db.process_event(&make_indexed_event(
            150,
            Event::OrderCancelled {
                order_id: oid(2),
                user: addr(10),
            },
        ))
        .unwrap();

        assert!(matches!(
            db.orders.get(&oid(2)).unwrap().status,
            OrderStatus::Cancelled
        ));
    }

    #[test]
    fn process_liquidation() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.process_event(&make_indexed_event(
            100,
            Event::PositionOpened {
                position_id: pid(1),
                user: addr(10),
                market_id: 0,
                is_long: true,
                size_usd: 1_000_000,
                leverage: 50,
                entry_price: 67_000,
                collateral_token: [0u8; 20],
                collateral_amount: 20_000,
            },
        ))
        .unwrap();

        db.process_event(&make_indexed_event(
            300,
            Event::Liquidation {
                position_id: pid(1),
                user: addr(10),
                market_id: 0,
                liquidation_price: 65_000,
                penalty: 5_000,
                keeper: addr(99),
            },
        ))
        .unwrap();

        let pos = db.positions.get(&pid(1)).unwrap();
        assert!(matches!(pos.status, PositionStatus::Liquidated));

        let stats = db.trader_stats.get(&addr(10)).unwrap();
        assert_eq!(stats.liquidation_count, 1);
        assert_eq!(stats.open_positions, 0);
    }

    #[test]
    fn process_collateral_events() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        let token = {
            let mut t = [0u8; 20];
            t[0] = 0xAA;
            t
        };

        db.process_event(&make_indexed_event(
            100,
            Event::CollateralDeposited {
                user: addr(10),
                token,
                amount: 1_000_000,
            },
        ))
        .unwrap();

        let bal = db.balances.get(&addr(10), &token).unwrap();
        assert_eq!(bal.amount, 1_000_000);
        assert_eq!(bal.available, 1_000_000);

        db.process_event(&make_indexed_event(
            200,
            Event::CollateralWithdrawn {
                user: addr(10),
                token,
                amount: 300_000,
            },
        ))
        .unwrap();

        let bal = db.balances.get(&addr(10), &token).unwrap();
        assert_eq!(bal.amount, 700_000);
    }

    #[test]
    fn process_vault_created() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.process_event(&make_indexed_event(
            100,
            Event::VaultCreated {
                user: addr(10),
                vault: addr(50),
            },
        ))
        .unwrap();

        let user = db.users.get(&addr(10)).unwrap();
        assert_eq!(user.vault_address, Some(addr(50)));
    }

    #[test]
    fn process_funding_rate_update() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.process_event(&make_indexed_event(
            100,
            Event::FundingRateUpdated {
                market_id: 0,
                new_rate_per_second: 500,
                funding_rate_24h: 43_200_000,
            },
        ))
        .unwrap();

        assert_eq!(db.market_state.funding_rate(0), 500);
        assert_eq!(db.market_state.funding_rate_24h(0), 43_200_000);
    }

    #[test]
    fn leaderboard_rebuild() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.trader_stats.ensure(addr(1));
        db.trader_stats
            .increment_trade(&addr(1), 10_000, 5_000, 100, true, false, 100);
        db.trader_stats.ensure(addr(2));
        db.trader_stats
            .increment_trade(&addr(2), 20_000, -1_000, 200, false, false, 200);

        db.rebuild_leaderboard(1000);

        let pnl = db.pnl_leaderboard(10);
        assert_eq!(pnl.len(), 2);
        assert_eq!(pnl[0].address, addr(1));

        let vol = db.volume_leaderboard(10);
        assert_eq!(vol[0].address, addr(2));
    }

    #[test]
    fn snapshot_and_recover() {
        let dir = tempfile::tempdir().unwrap();

        {
            let db = PerfDb::open(test_config(dir.path())).unwrap();

            db.ingest_tick(&make_tick(1_000_000_000, 0, 67_000.0))
                .unwrap();
            db.ingest_tick(&make_tick(2_000_000_000, 0, 67_100.0))
                .unwrap();

            db.process_event(&make_indexed_event(
                100,
                Event::PositionOpened {
                    position_id: pid(1),
                    user: addr(10),
                    market_id: 0,
                    is_long: true,
                    size_usd: 1_000_000,
                    leverage: 10,
                    entry_price: 67_000,
                    collateral_token: [0u8; 20],
                    collateral_amount: 100_000,
                },
            ))
            .unwrap();

            db.save_checkpoint(100, "0xblock100".to_string(), 1200);
            db.snapshot().unwrap();
        }

        {
            let db = PerfDb::open(test_config(dir.path())).unwrap();

            assert!(db.positions.get(&pid(1)).is_some());
            assert!(db.users.contains(&addr(10)));
            assert_eq!(db.checkpoint.last_block(), 100);
            assert_eq!(db.tick_count(0), 2);
        }
    }

    #[test]
    fn timestamp_to_date_conversion() {
        assert_eq!(timestamp_to_date(1731628800), 20241115);
        assert_eq!(timestamp_to_date(0), 19700101);
        assert_eq!(timestamp_to_date(946684800), 20000101);
    }

    #[test]
    fn save_checkpoint_updates() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.save_checkpoint(42, "0xhash".to_string(), 504);
        assert_eq!(db.checkpoint.last_block(), 42);
        assert_eq!(db.checkpoint.last_block_hash(), "0xhash");
    }

    #[test]
    fn all_prices_after_ticks() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();

        db.ingest_tick(&make_tick(1_000, 0, 67_000.0)).unwrap();
        db.ingest_tick(&make_tick(1_000, 1, 3_500.0)).unwrap();

        let prices = db.all_prices();
        assert_eq!(prices.len(), 2);
    }

    #[test]
    fn sync_wal_succeeds() {
        let dir = tempfile::tempdir().unwrap();
        let db = PerfDb::open(test_config(dir.path())).unwrap();
        db.ingest_tick(&make_tick(1_000, 0, 67_000.0)).unwrap();
        db.sync_wal().unwrap();
    }
}
