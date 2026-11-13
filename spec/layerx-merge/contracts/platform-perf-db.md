# Platform perf DB contract

The [performance database](../../../services/platform/perf-db) implements market-data records, queries, aggregation and storage. The tables below identify its CLI, environment names, file formats and public Rust declarations.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

## perf-db-cli-commands

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `struct Cli {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Balances {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `user: String,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Candles {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `last: usize,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `market: String,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `timeframe: String,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Checkpoint,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Daily {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `address: String,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `limit: usize,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `data_dir: PathBuf,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Events {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `market: Option<String>,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `trader: Option<String>,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Info,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `json: bool,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Leaderboard {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `metric: String,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Markets,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Orderbook {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `levels: usize,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Orders {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `active: bool,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `id: Option<String>,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `user: Option<String>,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Positions {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `open: bool,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Prices {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Snapshot,` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Stats {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `Ticks {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |
| `cli_command` | `User {` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |

## perf-db-env-vars

| Kind | Declaration | Source |
|---|---|---|
| `env_var` | `PERFDB_DATA_DIR` | [services/platform/perf-db/src/main.rs](../../../services/platform/perf-db/src/main.rs) |

## perf-db-file-formats

| Kind | Declaration | Source |
|---|---|---|
| `file_format` | `base_dir.join(format!("m{:04}_tf{:02}.candle", market_id, tf as u8))` | [services/platform/perf-db/src/storage/candle_store.rs](../../../services/platform/perf-db/src/storage/candle_store.rs) |
| `file_format` | `pub struct CandleRecord {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `file_format` | `pub struct CheckpointStore {` | [services/platform/perf-db/src/relational/checkpoint.rs](../../../services/platform/perf-db/src/relational/checkpoint.rs) |
| `file_format` | `const HEADER_SIZE: u64 = 8;` | [services/platform/perf-db/src/storage/mmap.rs](../../../services/platform/perf-db/src/storage/mmap.rs) |
| `file_format` | `pub struct PriceTick {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `file_format` | `pub struct StateSnapshot {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `file_format` | `base_dir.join(format!("market_{:04}.tick", market_id))` | [services/platform/perf-db/src/storage/tick_store.rs](../../../services/platform/perf-db/src/storage/tick_store.rs) |
| `file_format` | `pub enum WalEntry {` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |
| `file_format` | `const ENTRY_HEADER_SIZE: usize = 8;` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |

## perf-db-rust-pub-items

| Kind | Declaration | Source |
|---|---|---|
| `rust_pub_item` | `pub struct CandleBuilder {` | [services/platform/perf-db/src/aggregation/candle_builder.rs](../../../services/platform/perf-db/src/aggregation/candle_builder.rs) |
| `rust_pub_item` | `pub enum CandleOutput {` | [services/platform/perf-db/src/aggregation/candle_builder.rs](../../../services/platform/perf-db/src/aggregation/candle_builder.rs) |
| `rust_pub_item` | `pub struct Leaderboard {` | [services/platform/perf-db/src/aggregation/leaderboard.rs](../../../services/platform/perf-db/src/aggregation/leaderboard.rs) |
| `rust_pub_item` | `pub struct LeaderboardEntry {` | [services/platform/perf-db/src/aggregation/leaderboard.rs](../../../services/platform/perf-db/src/aggregation/leaderboard.rs) |
| `rust_pub_item` | `pub enum LeaderboardMetric {` | [services/platform/perf-db/src/aggregation/leaderboard.rs](../../../services/platform/perf-db/src/aggregation/leaderboard.rs) |
| `rust_pub_item` | `pub struct DailyStatsStore {` | [services/platform/perf-db/src/aggregation/stats.rs](../../../services/platform/perf-db/src/aggregation/stats.rs) |
| `rust_pub_item` | `pub struct TraderStatsStore {` | [services/platform/perf-db/src/aggregation/stats.rs](../../../services/platform/perf-db/src/aggregation/stats.rs) |
| `rust_pub_item` | `pub const fn is_valid_market(id: MarketId) -> bool {` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub const fn market_by_id(id: MarketId) -> Option<&'static MarketMeta> {` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub fn market_by_symbol(symbol: &str) -> Option<&'static MarketMeta> {` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub const MARKET_COUNT: usize = 18;` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub type MarketId = u16;` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub struct MarketMeta {` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub const MARKETS: [MarketMeta; MARKET_COUNT] = [` | [services/platform/perf-db/src/core/market.rs](../../../services/platform/perf-db/src/core/market.rs) |
| `rust_pub_item` | `pub enum Timeframe {` | [services/platform/perf-db/src/core/timeframe.rs](../../../services/platform/perf-db/src/core/timeframe.rs) |
| `rust_pub_item` | `pub struct Balance {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct DailyStats {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub enum Event {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub type FixedI128 = i128;` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct IndexedEvent {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct MarketInfo {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct Order {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub enum OrderStatus {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub enum OrderType {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub fn parse_address(s: &str) -> Result<[u8; 20], ParseIdentifierError> {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub fn parse_fixed(s: &str) -> Result<FixedI128, ParseFixedError> {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub fn parse_hash(s: &str) -> Result<[u8; 32], ParseIdentifierError> {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct Position {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub enum PositionStatus {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct ReferralInfo {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct Trade {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct TraderStats {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub enum TradeType {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct User {` | [services/platform/perf-db/src/core/types.rs](../../../services/platform/perf-db/src/core/types.rs) |
| `rust_pub_item` | `pub struct PerfDb {` | [services/platform/perf-db/src/engine.rs](../../../services/platform/perf-db/src/engine.rs) |
| `rust_pub_item` | `pub struct PerfDbConfig {` | [services/platform/perf-db/src/engine.rs](../../../services/platform/perf-db/src/engine.rs) |
| `rust_pub_item` | `pub enum PerfDbError {` | [services/platform/perf-db/src/error.rs](../../../services/platform/perf-db/src/error.rs) |
| `rust_pub_item` | `pub mod aggregation;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod core;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod engine;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub use engine::{PerfDb, PerfDbConfig};` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod error;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub use error::PerfDbError;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod query;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod relational;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod state;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub mod storage;` | [services/platform/perf-db/src/lib.rs](../../../services/platform/perf-db/src/lib.rs) |
| `rust_pub_item` | `pub fn find_candle_at(candles: &[CandleRecord], timestamp_ns: u64) -> Option<&CandleRecord> {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn lower_bound_candles(candles: &[CandleRecord], target_ns: u64) -> usize {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn lower_bound_ticks(ticks: &[PriceTick], target_ns: u64) -> usize {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn query_candle_range_inclusive(` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn query_candle_range(candles: &[CandleRecord], start_ns: u64, end_ns: u64) -> &[CandleRecord] {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn query_last_candles(candles: &[CandleRecord], count: usize) -> &[CandleRecord] {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn query_last_ticks(ticks: &[PriceTick], count: usize) -> &[PriceTick] {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn query_tick_range_inclusive(ticks: &[PriceTick], start_ns: u64, end_ns: u64) -> &[PriceTick] {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn query_tick_range(ticks: &[PriceTick], start_ns: u64, end_ns: u64) -> &[PriceTick] {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn upper_bound_candles(candles: &[CandleRecord], target_ns: u64) -> usize {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub fn upper_bound_ticks(ticks: &[PriceTick], target_ns: u64) -> usize {` | [services/platform/perf-db/src/query/time_range.rs](../../../services/platform/perf-db/src/query/time_range.rs) |
| `rust_pub_item` | `pub struct Alert {` | [services/platform/perf-db/src/relational/alerts.rs](../../../services/platform/perf-db/src/relational/alerts.rs) |
| `rust_pub_item` | `pub struct AlertStore {` | [services/platform/perf-db/src/relational/alerts.rs](../../../services/platform/perf-db/src/relational/alerts.rs) |
| `rust_pub_item` | `pub struct ApiKey {` | [services/platform/perf-db/src/relational/api_keys.rs](../../../services/platform/perf-db/src/relational/api_keys.rs) |
| `rust_pub_item` | `pub struct ApiKeyStore {` | [services/platform/perf-db/src/relational/api_keys.rs](../../../services/platform/perf-db/src/relational/api_keys.rs) |
| `rust_pub_item` | `pub struct Checkpoint {` | [services/platform/perf-db/src/relational/checkpoint.rs](../../../services/platform/perf-db/src/relational/checkpoint.rs) |
| `rust_pub_item` | `pub struct Competition {` | [services/platform/perf-db/src/relational/competitions.rs](../../../services/platform/perf-db/src/relational/competitions.rs) |
| `rust_pub_item` | `pub struct CompetitionEntry {` | [services/platform/perf-db/src/relational/competitions.rs](../../../services/platform/perf-db/src/relational/competitions.rs) |
| `rust_pub_item` | `pub struct CompetitionStore {` | [services/platform/perf-db/src/relational/competitions.rs](../../../services/platform/perf-db/src/relational/competitions.rs) |
| `rust_pub_item` | `pub struct ReferralEngine {` | [services/platform/perf-db/src/relational/referrals.rs](../../../services/platform/perf-db/src/relational/referrals.rs) |
| `rust_pub_item` | `pub struct ReferralTier {` | [services/platform/perf-db/src/relational/referrals.rs](../../../services/platform/perf-db/src/relational/referrals.rs) |
| `rust_pub_item` | `pub struct UserStore {` | [services/platform/perf-db/src/relational/users.rs](../../../services/platform/perf-db/src/relational/users.rs) |
| `rust_pub_item` | `pub struct BalanceCache {` | [services/platform/perf-db/src/state/balances.rs](../../../services/platform/perf-db/src/state/balances.rs) |
| `rust_pub_item` | `pub struct MarketSnapshot {` | [services/platform/perf-db/src/state/market_state.rs](../../../services/platform/perf-db/src/state/market_state.rs) |
| `rust_pub_item` | `pub struct MarketStateTable {` | [services/platform/perf-db/src/state/market_state.rs](../../../services/platform/perf-db/src/state/market_state.rs) |
| `rust_pub_item` | `pub struct DepthLevel {` | [services/platform/perf-db/src/state/orders.rs](../../../services/platform/perf-db/src/state/orders.rs) |
| `rust_pub_item` | `pub struct Orderbook {` | [services/platform/perf-db/src/state/orders.rs](../../../services/platform/perf-db/src/state/orders.rs) |
| `rust_pub_item` | `pub struct OrderTable {` | [services/platform/perf-db/src/state/orders.rs](../../../services/platform/perf-db/src/state/orders.rs) |
| `rust_pub_item` | `pub struct PositionTable {` | [services/platform/perf-db/src/state/positions.rs](../../../services/platform/perf-db/src/state/positions.rs) |
| `rust_pub_item` | `pub enum Channel {` | [services/platform/perf-db/src/state/pubsub.rs](../../../services/platform/perf-db/src/state/pubsub.rs) |
| `rust_pub_item` | `pub struct PubSubBus {` | [services/platform/perf-db/src/state/pubsub.rs](../../../services/platform/perf-db/src/state/pubsub.rs) |
| `rust_pub_item` | `pub enum PubSubMessage {` | [services/platform/perf-db/src/state/pubsub.rs](../../../services/platform/perf-db/src/state/pubsub.rs) |
| `rust_pub_item` | `pub struct CandleStore {` | [services/platform/perf-db/src/storage/candle_store.rs](../../../services/platform/perf-db/src/storage/candle_store.rs) |
| `rust_pub_item` | `pub struct EventStore {` | [services/platform/perf-db/src/storage/event_store.rs](../../../services/platform/perf-db/src/storage/event_store.rs) |
| `rust_pub_item` | `pub struct MmapFile {` | [services/platform/perf-db/src/storage/mmap.rs](../../../services/platform/perf-db/src/storage/mmap.rs) |
| `rust_pub_item` | `pub struct RecordIter<'a, T: bytemuck::Pod> {` | [services/platform/perf-db/src/storage/mmap.rs](../../../services/platform/perf-db/src/storage/mmap.rs) |
| `rust_pub_item` | `pub fn cleanup_snapshots(dir: &Path, keep_from_block: u64) -> Result<u64, PerfDbError> {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub struct DailyStatsEntry {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub fn list_snapshots(dir: &Path) -> Result<Vec<(u64, PathBuf)>, PerfDbError> {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub fn load_latest_snapshot(dir: &Path) -> Result<Option<StateSnapshot>, PerfDbError> {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub fn load_snapshot(dir: &Path, block: u64) -> Result<Option<StateSnapshot>, PerfDbError> {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub struct SnapshotCheckpoint {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub struct SnapshotMarketState {` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub fn write_snapshot(` | [services/platform/perf-db/src/storage/snapshot.rs](../../../services/platform/perf-db/src/storage/snapshot.rs) |
| `rust_pub_item` | `pub struct TickStore {` | [services/platform/perf-db/src/storage/tick_store.rs](../../../services/platform/perf-db/src/storage/tick_store.rs) |
| `rust_pub_item` | `pub fn replay(` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |
| `rust_pub_item` | `pub enum SyncPolicy {` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |
| `rust_pub_item` | `pub fn truncate_before(dir: impl AsRef<Path>, keep_from: u64) -> Result<u64, PerfDbError> {` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |
| `rust_pub_item` | `pub struct WalConfig {` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |
| `rust_pub_item` | `pub struct WalWriter {` | [services/platform/perf-db/src/storage/wal.rs](../../../services/platform/perf-db/src/storage/wal.rs) |
