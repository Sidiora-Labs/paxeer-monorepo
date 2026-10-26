# Platform perf db contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## perf-db-cli-commands

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.cli-perf-db-perfdb</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:19</code> | <code>struct Cli {</code> |
| <code>interface.8.cli-perf-db-perfdb-balances</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:105</code> | <code>Balances {</code> |
| <code>interface.8.cli-perf-db-perfdb-balances-user</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:107</code> | <code>user: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-candles</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:56</code> | <code>Candles {</code> |
| <code>interface.8.cli-perf-db-perfdb-candles-last</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:63</code> | <code>last: usize,</code> |
| <code>interface.8.cli-perf-db-perfdb-candles-market</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:58</code> | <code>market: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-candles-timeframe</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:60</code> | <code>timeframe: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-checkpoint</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:155</code> | <code>Checkpoint,</code> |
| <code>interface.8.cli-perf-db-perfdb-daily</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:123</code> | <code>Daily {</code> |
| <code>interface.8.cli-perf-db-perfdb-daily-address</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:125</code> | <code>address: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-daily-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:128</code> | <code>limit: usize,</code> |
| <code>interface.8.cli-perf-db-perfdb-data-dir</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:22</code> | <code>data_dir: PathBuf,</code> |
| <code>interface.8.cli-perf-db-perfdb-events</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:142</code> | <code>Events {</code> |
| <code>interface.8.cli-perf-db-perfdb-events-last</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:151</code> | <code>last: usize,</code> |
| <code>interface.8.cli-perf-db-perfdb-events-market</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:145</code> | <code>market: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-events-trader</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:148</code> | <code>trader: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-info</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:35</code> | <code>Info,</code> |
| <code>interface.8.cli-perf-db-perfdb-json</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:26</code> | <code>json: bool,</code> |
| <code>interface.8.cli-perf-db-perfdb-leaderboard</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:132</code> | <code>Leaderboard {</code> |
| <code>interface.8.cli-perf-db-perfdb-leaderboard-limit</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:138</code> | <code>limit: usize,</code> |
| <code>interface.8.cli-perf-db-perfdb-leaderboard-metric</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:135</code> | <code>metric: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-markets</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:38</code> | <code>Markets,</code> |
| <code>interface.8.cli-perf-db-perfdb-orderbook</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:96</code> | <code>Orderbook {</code> |
| <code>interface.8.cli-perf-db-perfdb-orderbook-levels</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:101</code> | <code>levels: usize,</code> |
| <code>interface.8.cli-perf-db-perfdb-orderbook-market</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:98</code> | <code>market: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-orders</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:83</code> | <code>Orders {</code> |
| <code>interface.8.cli-perf-db-perfdb-orders-active</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:92</code> | <code>active: bool,</code> |
| <code>interface.8.cli-perf-db-perfdb-orders-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:89</code> | <code>id: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-orders-user</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:86</code> | <code>user: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-positions</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:67</code> | <code>Positions {</code> |
| <code>interface.8.cli-perf-db-perfdb-positions-id</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:76</code> | <code>id: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-positions-market</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:73</code> | <code>market: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-positions-open</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:79</code> | <code>open: bool,</code> |
| <code>interface.8.cli-perf-db-perfdb-positions-user</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:70</code> | <code>user: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-prices</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:41</code> | <code>Prices {</code> |
| <code>interface.8.cli-perf-db-perfdb-prices-market</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:43</code> | <code>market: Option&lt;String&gt;,</code> |
| <code>interface.8.cli-perf-db-perfdb-snapshot</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:158</code> | <code>Snapshot,</code> |
| <code>interface.8.cli-perf-db-perfdb-stats</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:117</code> | <code>Stats {</code> |
| <code>interface.8.cli-perf-db-perfdb-stats-address</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:119</code> | <code>address: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-ticks</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:47</code> | <code>Ticks {</code> |
| <code>interface.8.cli-perf-db-perfdb-ticks-last</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:52</code> | <code>last: usize,</code> |
| <code>interface.8.cli-perf-db-perfdb-ticks-market</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:49</code> | <code>market: String,</code> |
| <code>interface.8.cli-perf-db-perfdb-user</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:111</code> | <code>User {</code> |
| <code>interface.8.cli-perf-db-perfdb-user-address</code> | <code>cli_command</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:113</code> | <code>address: String,</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.cli-perf-db-perfdb</code> | notes | 110 callers total |
| <code>interface.8.cli-perf-db-perfdb-balances</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-balances-user</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-candles</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-candles-last</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-candles-market</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-candles-timeframe</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-checkpoint</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-daily</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-daily-address</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-daily-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-data-dir</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-events</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-events-last</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-events-market</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-events-trader</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-info</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-json</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-leaderboard</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-leaderboard-limit</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-leaderboard-metric</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-markets</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orderbook</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orderbook-levels</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orderbook-market</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orders</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orders-active</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orders-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-orders-user</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-positions</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-positions-id</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-positions-market</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-positions-open</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-positions-user</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-prices</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-prices-market</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-snapshot</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-stats</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-stats-address</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-ticks</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-ticks-last</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-ticks-market</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-user</code> | notes | no in-repo invocation outside the defining file |
| <code>interface.8.cli-perf-db-perfdb-user-address</code> | notes | no in-repo invocation outside the defining file |

## perf-db-env-vars

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.env-perf-db-perfdb-data-dir</code> | <code>env_var</code> | <code>stable</code> | <code>services/platform/perf-db/src/main.rs:21</code> | <code>#[arg(long, env = "PERFDB_DATA_DIR", default_value = "./data")]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.env-perf-db-perfdb-data-dir</code> | notes | no in-repo setter or reader outside the defining file |

## perf-db-file-formats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.fmt-perf-db-candle-file-name</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/candle_store.rs:150</code> | <code>base_dir.join(format!("m{:04}_tf{:02}.candle", market_id, tf as u8))</code> |
| <code>interface.8.fmt-perf-db-candle-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:20</code> | <code>pub struct CandleRecord {</code> |
| <code>interface.8.fmt-perf-db-checkpoint</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/checkpoint.rs:12</code> | <code>pub struct CheckpointStore {</code> |
| <code>interface.8.fmt-perf-db-mmap-store-header</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/mmap.rs:37</code> | <code>const HEADER_SIZE: u64 = 8;</code> |
| <code>interface.8.fmt-perf-db-price-tick-record</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:7</code> | <code>pub struct PriceTick {</code> |
| <code>interface.8.fmt-perf-db-snapshot-file-name</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:112</code> | <code>let filename = format!("snapshot_{:012}.bin", snapshot.checkpoint.last_block);</code> |
| <code>interface.8.fmt-perf-db-state-snapshot</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:18</code> | <code>pub struct StateSnapshot {</code> |
| <code>interface.8.fmt-perf-db-tick-file-name</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/tick_store.rs:118</code> | <code>base_dir.join(format!("market_{:04}.tick", market_id))</code> |
| <code>interface.8.fmt-perf-db-wal-entry</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:17</code> | <code>pub enum WalEntry {</code> |
| <code>interface.8.fmt-perf-db-wal-entry-header</code> | <code>file_format</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:101</code> | <code>const ENTRY_HEADER_SIZE: usize = 4 + 4; // len + crc32</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.fmt-perf-db-candle-file-name</code> | notes | one candle file per market and timeframe |
| <code>interface.8.fmt-perf-db-candle-record</code> | notes | fixed-size repr(C) candle record; 40 callers total |
| <code>interface.8.fmt-perf-db-checkpoint</code> | notes | relational checkpoint record; 485 callers total |
| <code>interface.8.fmt-perf-db-mmap-store-header</code> | notes | append-only memory-mapped record file with an 8-byte length header; read and written only by the defining module |
| <code>interface.8.fmt-perf-db-price-tick-record</code> | notes | fixed-size repr(C) tick record; 56 callers total |
| <code>interface.8.fmt-perf-db-snapshot-file-name</code> | notes | snapshot file named by the last block |
| <code>interface.8.fmt-perf-db-state-snapshot</code> | notes | bincode state snapshot |
| <code>interface.8.fmt-perf-db-tick-file-name</code> | notes | one tick file per market |
| <code>interface.8.fmt-perf-db-wal-entry</code> | notes | bincode WalEntry records (IndexedEvent, PriceTick, CandleUpdate, BalanceUpdate, Checkpoint, BatchEnd) |
| <code>interface.8.fmt-perf-db-wal-entry-header</code> | notes | each WAL record is framed by a u32 length and a u32 CRC32; read and written only by the defining module |

## perf-db-rust-pub-items

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.rs-perf-db-aggregation-candle-builder-candlebuilder-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/candle_builder.rs:81</code> | <code>pub struct CandleBuilder {</code> |
| <code>interface.8.rs-perf-db-aggregation-candle-builder-candleoutput-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/candle_builder.rs:60</code> | <code>pub enum CandleOutput {</code> |
| <code>interface.8.rs-perf-db-aggregation-leaderboard-leaderboard-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/leaderboard.rs:32</code> | <code>pub struct Leaderboard {</code> |
| <code>interface.8.rs-perf-db-aggregation-leaderboard-leaderboardentry-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/leaderboard.rs:7</code> | <code>pub struct LeaderboardEntry {</code> |
| <code>interface.8.rs-perf-db-aggregation-leaderboard-leaderboardmetric-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/leaderboard.rs:18</code> | <code>pub enum LeaderboardMetric {</code> |
| <code>interface.8.rs-perf-db-aggregation-stats-dailystatsstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/stats.rs:154</code> | <code>pub struct DailyStatsStore {</code> |
| <code>interface.8.rs-perf-db-aggregation-stats-traderstatsstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/aggregation/stats.rs:11</code> | <code>pub struct TraderStatsStore {</code> |
| <code>interface.8.rs-perf-db-core-market-fn-2-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:52</code> | <code>pub const fn is_valid_market(id: MarketId) -&gt; bool {</code> |
| <code>interface.8.rs-perf-db-core-market-fn-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:38</code> | <code>pub const fn market_by_id(id: MarketId) -&gt; Option&lt;&amp;'static MarketMeta&gt; {</code> |
| <code>interface.8.rs-perf-db-core-market-market-by-symbol-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:47</code> | <code>pub fn market_by_symbol(symbol: &amp;str) -&gt; Option&lt;&amp;'static MarketMeta&gt; {</code> |
| <code>interface.8.rs-perf-db-core-market-market-count-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:5</code> | <code>pub const MARKET_COUNT: usize = 18;</code> |
| <code>interface.8.rs-perf-db-core-market-marketid-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:2</code> | <code>pub type MarketId = u16;</code> |
| <code>interface.8.rs-perf-db-core-market-marketmeta-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:9</code> | <code>pub struct MarketMeta {</code> |
| <code>interface.8.rs-perf-db-core-market-markets-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/market.rs:16</code> | <code>pub const MARKETS: [MarketMeta; MARKET_COUNT] = [</code> |
| <code>interface.8.rs-perf-db-core-timeframe-timeframe-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/timeframe.rs:8</code> | <code>pub enum Timeframe {</code> |
| <code>interface.8.rs-perf-db-core-types-balance-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:360</code> | <code>pub struct Balance {</code> |
| <code>interface.8.rs-perf-db-core-types-dailystats-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:332</code> | <code>pub struct DailyStats {</code> |
| <code>interface.8.rs-perf-db-core-types-event-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:184</code> | <code>pub enum Event {</code> |
| <code>interface.8.rs-perf-db-core-types-fixedi128-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:37</code> | <code>pub type FixedI128 = i128;</code> |
| <code>interface.8.rs-perf-db-core-types-indexedevent-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:287</code> | <code>pub struct IndexedEvent {</code> |
| <code>interface.8.rs-perf-db-core-types-marketinfo-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:341</code> | <code>pub struct MarketInfo {</code> |
| <code>interface.8.rs-perf-db-core-types-order-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:122</code> | <code>pub struct Order {</code> |
| <code>interface.8.rs-perf-db-core-types-orderstatus-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:104</code> | <code>pub enum OrderStatus {</code> |
| <code>interface.8.rs-perf-db-core-types-ordertype-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:83</code> | <code>pub enum OrderType {</code> |
| <code>interface.8.rs-perf-db-core-types-parse-address-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:383</code> | <code>pub fn parse_address(s: &amp;str) -&gt; [u8; 20] {</code> |
| <code>interface.8.rs-perf-db-core-types-parse-fixed-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:378</code> | <code>pub fn parse_fixed(s: &amp;str) -&gt; FixedI128 {</code> |
| <code>interface.8.rs-perf-db-core-types-parse-hash-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:394</code> | <code>pub fn parse_hash(s: &amp;str) -&gt; [u8; 32] {</code> |
| <code>interface.8.rs-perf-db-core-types-position-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:59</code> | <code>pub struct Position {</code> |
| <code>interface.8.rs-perf-db-core-types-positionstatus-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:41</code> | <code>pub enum PositionStatus {</code> |
| <code>interface.8.rs-perf-db-core-types-referralinfo-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:369</code> | <code>pub struct ReferralInfo {</code> |
| <code>interface.8.rs-perf-db-core-types-trade-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:167</code> | <code>pub struct Trade {</code> |
| <code>interface.8.rs-perf-db-core-types-traderstats-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:310</code> | <code>pub struct TraderStats {</code> |
| <code>interface.8.rs-perf-db-core-types-tradetype-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:145</code> | <code>pub enum TradeType {</code> |
| <code>interface.8.rs-perf-db-core-types-user-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/core/types.rs:297</code> | <code>pub struct User {</code> |
| <code>interface.8.rs-perf-db-engine-perfdb-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/engine.rs:80</code> | <code>pub struct PerfDb {</code> |
| <code>interface.8.rs-perf-db-engine-perfdbconfig-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/engine.rs:31</code> | <code>pub struct PerfDbConfig {</code> |
| <code>interface.8.rs-perf-db-error-perfdberror-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/error.rs:4</code> | <code>pub enum PerfDbError {</code> |
| <code>interface.8.rs-perf-db-perfdb-aggregation</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:1</code> | <code>pub mod aggregation;</code> |
| <code>interface.8.rs-perf-db-perfdb-core</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:2</code> | <code>pub mod core;</code> |
| <code>interface.8.rs-perf-db-perfdb-engine</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:3</code> | <code>pub mod engine;</code> |
| <code>interface.8.rs-perf-db-perfdb-engine-perfdb-perfdbconfig</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:10</code> | <code>pub use engine::{PerfDb, PerfDbConfig};</code> |
| <code>interface.8.rs-perf-db-perfdb-error</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:4</code> | <code>pub mod error;</code> |
| <code>interface.8.rs-perf-db-perfdb-error-perfdberror</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:11</code> | <code>pub use error::PerfDbError;</code> |
| <code>interface.8.rs-perf-db-perfdb-query</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:5</code> | <code>pub mod query;</code> |
| <code>interface.8.rs-perf-db-perfdb-relational</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:6</code> | <code>pub mod relational;</code> |
| <code>interface.8.rs-perf-db-perfdb-state</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:7</code> | <code>pub mod state;</code> |
| <code>interface.8.rs-perf-db-perfdb-storage</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/lib.rs:8</code> | <code>pub mod storage;</code> |
| <code>interface.8.rs-perf-db-query-time-range-find-candle-at-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:70</code> | <code>pub fn find_candle_at(candles: &amp;[CandleRecord], timestamp_ns: u64) -&gt; Option&lt;&amp;CandleRecord&gt; {</code> |
| <code>interface.8.rs-perf-db-query-time-range-lower-bound-candles-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:35</code> | <code>pub fn lower_bound_candles(candles: &amp;[CandleRecord], target_ns: u64) -&gt; usize {</code> |
| <code>interface.8.rs-perf-db-query-time-range-lower-bound-ticks-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:5</code> | <code>pub fn lower_bound_ticks(ticks: &amp;[PriceTick], target_ns: u64) -&gt; usize {</code> |
| <code>interface.8.rs-perf-db-query-time-range-query-candle-range-inclusive-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:52</code> | <code>pub fn query_candle_range_inclusive(</code> |
| <code>interface.8.rs-perf-db-query-time-range-query-candle-range-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:45</code> | <code>pub fn query_candle_range(candles: &amp;[CandleRecord], start_ns: u64, end_ns: u64) -&gt; &amp;[CandleRecord] {</code> |
| <code>interface.8.rs-perf-db-query-time-range-query-last-candles-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:63</code> | <code>pub fn query_last_candles(candles: &amp;[CandleRecord], count: usize) -&gt; &amp;[CandleRecord] {</code> |
| <code>interface.8.rs-perf-db-query-time-range-query-last-ticks-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:29</code> | <code>pub fn query_last_ticks(ticks: &amp;[PriceTick], count: usize) -&gt; &amp;[PriceTick] {</code> |
| <code>interface.8.rs-perf-db-query-time-range-query-tick-range-inclusive-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:22</code> | <code>pub fn query_tick_range_inclusive(ticks: &amp;[PriceTick], start_ns: u64, end_ns: u64) -&gt; &amp;[PriceTick] {</code> |
| <code>interface.8.rs-perf-db-query-time-range-query-tick-range-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:15</code> | <code>pub fn query_tick_range(ticks: &amp;[PriceTick], start_ns: u64, end_ns: u64) -&gt; &amp;[PriceTick] {</code> |
| <code>interface.8.rs-perf-db-query-time-range-upper-bound-candles-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:40</code> | <code>pub fn upper_bound_candles(candles: &amp;[CandleRecord], target_ns: u64) -&gt; usize {</code> |
| <code>interface.8.rs-perf-db-query-time-range-upper-bound-ticks-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/query/time_range.rs:10</code> | <code>pub fn upper_bound_ticks(ticks: &amp;[PriceTick], target_ns: u64) -&gt; usize {</code> |
| <code>interface.8.rs-perf-db-relational-alerts-alert-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/alerts.rs:7</code> | <code>pub struct Alert {</code> |
| <code>interface.8.rs-perf-db-relational-alerts-alertstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/alerts.rs:18</code> | <code>pub struct AlertStore {</code> |
| <code>interface.8.rs-perf-db-relational-api-keys-apikey-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/api_keys.rs:5</code> | <code>pub struct ApiKey {</code> |
| <code>interface.8.rs-perf-db-relational-api-keys-apikeystore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/api_keys.rs:16</code> | <code>pub struct ApiKeyStore {</code> |
| <code>interface.8.rs-perf-db-relational-checkpoint-checkpoint-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/checkpoint.rs:25</code> | <code>pub struct Checkpoint {</code> |
| <code>interface.8.rs-perf-db-relational-competitions-competition-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/competitions.rs:5</code> | <code>pub struct Competition {</code> |
| <code>interface.8.rs-perf-db-relational-competitions-competitionentry-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/competitions.rs:18</code> | <code>pub struct CompetitionEntry {</code> |
| <code>interface.8.rs-perf-db-relational-competitions-competitionstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/competitions.rs:26</code> | <code>pub struct CompetitionStore {</code> |
| <code>interface.8.rs-perf-db-relational-referrals-referralengine-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/referrals.rs:50</code> | <code>pub struct ReferralEngine {</code> |
| <code>interface.8.rs-perf-db-relational-referrals-referraltier-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/referrals.rs:5</code> | <code>pub struct ReferralTier {</code> |
| <code>interface.8.rs-perf-db-relational-users-userstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/relational/users.rs:12</code> | <code>pub struct UserStore {</code> |
| <code>interface.8.rs-perf-db-state-balances-balancecache-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/balances.rs:17</code> | <code>pub struct BalanceCache {</code> |
| <code>interface.8.rs-perf-db-state-market-state-marketsnapshot-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/market_state.rs:70</code> | <code>pub struct MarketSnapshot {</code> |
| <code>interface.8.rs-perf-db-state-market-state-marketstatetable-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/market_state.rs:16</code> | <code>pub struct MarketStateTable {</code> |
| <code>interface.8.rs-perf-db-state-orders-depthlevel-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/orders.rs:24</code> | <code>pub struct DepthLevel {</code> |
| <code>interface.8.rs-perf-db-state-orders-orderbook-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/orders.rs:15</code> | <code>pub struct Orderbook {</code> |
| <code>interface.8.rs-perf-db-state-orders-ordertable-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/orders.rs:121</code> | <code>pub struct OrderTable {</code> |
| <code>interface.8.rs-perf-db-state-positions-positiontable-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/positions.rs:17</code> | <code>pub struct PositionTable {</code> |
| <code>interface.8.rs-perf-db-state-pubsub-channel-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/pubsub.rs:11</code> | <code>pub enum Channel {</code> |
| <code>interface.8.rs-perf-db-state-pubsub-pubsubbus-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/pubsub.rs:84</code> | <code>pub struct PubSubBus {</code> |
| <code>interface.8.rs-perf-db-state-pubsub-pubsubmessage-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/state/pubsub.rs:50</code> | <code>pub enum PubSubMessage {</code> |
| <code>interface.8.rs-perf-db-storage-candle-store-candlestore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/candle_store.rs:30</code> | <code>pub struct CandleStore {</code> |
| <code>interface.8.rs-perf-db-storage-event-store-eventstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/event_store.rs:19</code> | <code>pub struct EventStore {</code> |
| <code>interface.8.rs-perf-db-storage-mmap-mmapfile-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/mmap.rs:27</code> | <code>pub struct MmapFile {</code> |
| <code>interface.8.rs-perf-db-storage-mmap-recorditer-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/mmap.rs:279</code> | <code>pub struct RecordIter&lt;'a, T: bytemuck::Pod&gt; {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-cleanup-snapshots-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:222</code> | <code>pub fn cleanup_snapshots(dir: &amp;Path, keep_from_block: u64) -&gt; Result&lt;u64, PerfDbError&gt; {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-dailystatsentry-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:51</code> | <code>pub struct DailyStatsEntry {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-list-snapshots-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:189</code> | <code>pub fn list_snapshots(dir: &amp;Path) -&gt; Result&lt;Vec&lt;(u64, PathBuf)&gt;, PerfDbError&gt; {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-load-latest-snapshot-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:136</code> | <code>pub fn load_latest_snapshot(dir: &amp;Path) -&gt; Result&lt;Option&lt;StateSnapshot&gt;, PerfDbError&gt; {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-load-snapshot-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:166</code> | <code>pub fn load_snapshot(dir: &amp;Path, block: u64) -&gt; Result&lt;Option&lt;StateSnapshot&gt;, PerfDbError&gt; {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-snapshotcheckpoint-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:32</code> | <code>pub struct SnapshotCheckpoint {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-snapshotmarketstate-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:58</code> | <code>pub struct SnapshotMarketState {</code> |
| <code>interface.8.rs-perf-db-storage-snapshot-write-snapshot-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/snapshot.rs:103</code> | <code>pub fn write_snapshot(</code> |
| <code>interface.8.rs-perf-db-storage-tick-store-tickstore-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/tick_store.rs:23</code> | <code>pub struct TickStore {</code> |
| <code>interface.8.rs-perf-db-storage-wal-replay-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:244</code> | <code>pub fn replay(</code> |
| <code>interface.8.rs-perf-db-storage-wal-syncpolicy-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:71</code> | <code>pub enum SyncPolicy {</code> |
| <code>interface.8.rs-perf-db-storage-wal-truncate-before-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:336</code> | <code>pub fn truncate_before(dir: impl AsRef&lt;Path&gt;, keep_from: u64) -&gt; Result&lt;u64, PerfDbError&gt; {</code> |
| <code>interface.8.rs-perf-db-storage-wal-walconfig-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:58</code> | <code>pub struct WalConfig {</code> |
| <code>interface.8.rs-perf-db-storage-wal-walwriter-j41</code> | <code>rust_pub_item</code> | <code>stable</code> | <code>services/platform/perf-db/src/storage/wal.rs:103</code> | <code>pub struct WalWriter {</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.rs-perf-db-aggregation-leaderboard-leaderboardmetric-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.rs-perf-db-core-types-marketinfo-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.rs-perf-db-core-types-referralinfo-j41</code> | notes | public item of its package; no in-repo caller outside the defining line found at HEAD |
| <code>interface.8.rs-perf-db-perfdb-aggregation</code> | notes | used only inside its package |
| <code>interface.8.rs-perf-db-perfdb-core</code> | notes | 37 callers total |
| <code>interface.8.rs-perf-db-perfdb-error</code> | notes | used only inside its package |
| <code>interface.8.rs-perf-db-perfdb-error-perfdberror</code> | notes | used only inside its package |
| <code>interface.8.rs-perf-db-perfdb-relational</code> | notes | used only inside its package |
