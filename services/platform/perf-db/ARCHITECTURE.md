# PerfDB Architecture

## Engine

`PerfDb` in `src/engine.rs` is an embedded library coordinating time-series,
state, relational and event stores. `src/main.rs` provides a command-line
inspector. Embedding applications supply ingestion and network APIs.

### Stores and access paths

| Workload | Component | Representation and persistence |
|---|---|---|
| Price ticks | `TickStore` | Per-market mmap files of `PriceTick`; WAL-backed ingestion |
| OHLCV candles | `CandleStore`, `CandleBuilder` | Per-market/per-timeframe mmap records; live buckets in memory |
| Latest prices, funding, open interest | `MarketStateTable` | Atomic price reads and locked market metadata |
| Positions | `PositionTable` | In-memory records and user/market indexes under a read/write lock |
| Orders and orderbooks | `OrderTable` | In-memory records, secondary indexes and sorted price levels under a read/write lock |
| Balances | `BalanceCache` | Sharded concurrent map |
| Users and indexer progress | `UserStore`, `CheckpointStore` | In-memory state restored by engine snapshots and event replay |
| Trader and daily statistics | `TraderStatsStore`, `DailyStatsStore` | In-memory aggregates included in engine snapshots |
| Leaderboards | `Leaderboard` | In-memory ranking rebuilt by `rebuild_leaderboard` |
| Referrals, competitions, alerts, API keys | `ReferralEngine`, `CompetitionStore`, `AlertStore`, `ApiKeyStore` | In-memory stores captured in engine snapshots; `storage/relational.rs` also exposes a separate durable relational coordinator |
| Events and funding settlements | `EventStore` | In-memory global log and market/trader indexes rebuilt from snapshot event ledger and WAL; funding settlement records kept by market |
| Streaming | `PubSubBus` | Typed `tokio::broadcast` messages on Prices, Positions, Orders, Trades, Liquidations, Funding and Balances channels |

Applications call `ingest_tick` or `process_event`. The engine appends a WAL
entry, applies it to the stores, and publishes typed messages. Event identity
and canonical block-slot checks reject conflicting redelivery; identical
redelivery is idempotent. Funding history is represented by indexed events and
funding state in `EventStore` and `MarketStateTable`.

### Record layouts

`src/core/types.rs` is the source of truth. `PriceTick` and `CandleRecord` are
`#[repr(C)]` and implement `bytemuck::Pod`; mmap payloads contain their native
memory representation, including explicit padding. Multi-byte fields use native
endianness. These records are distinct from the bincode WAL representation.

| Record | Field | Type | Offset | Bytes |
|---|---|---|---|---|
| PriceTick | timestamp_ns | u64 | 0 | 8 |
| PriceTick | market_id | u16 | 8 | 2 |
| PriceTick | _pad | [u8; 6] | 10 | 6 |
| PriceTick | price | f64 | 16 | 8 |
| CandleRecord | timestamp_ns | u64 | 0 | 8 |
| CandleRecord | open | f64 | 8 | 8 |
| CandleRecord | high | f64 | 16 | 8 |
| CandleRecord | low | f64 | 24 | 8 |
| CandleRecord | close | f64 | 32 | 8 |
| CandleRecord | volume | f32 | 40 | 4 |
| CandleRecord | market_id | u16 | 44 | 2 |
| CandleRecord | timeframe | u8 | 46 | 1 |
| CandleRecord | _pad | u8 | 47 | 1 |

`PriceTick` is 24 bytes, aligned to 8 bytes. `CandleRecord` is 48 bytes, aligned
to 8 bytes. Timestamps are nanoseconds; a candle timestamp is its bucket start.
`timeframe` is the enum index below. Price-feed candles have zero volume because
ticks contain only prices. `volume` is `f32` to keep candles at 48 bytes.

Each mmap file starts with an 8-byte little-endian logical payload length.
Records begin immediately after that header; allocated capacity can exceed the
logical length. New files initially allocate 4 MiB and grow as needed.
`TickStore` uses `ticks/market_0000.tick` for BTC (market 0) and
`ticks/market_0001.tick` for ETH (market 1). `CandleStore` uses
`candles/m0000_tf00.candle` for BTC/Sec1 and `candles/m0000_tf15.candle` for BTC/Week1.

`Position`, `Order`, `Trade`, `IndexedEvent`, users and statistics are heap
values serialized with serde/bincode for WAL entries or snapshots, rather than
fixed mmap record layouts. `FixedI128` is an `i128` with 18 decimal places.
`Position.status` is `PositionStatus` (Open, Closed, Liquidated), serialized by
serde; its Rust enum discriminants alone do not specify its bincode encoding.

### Timeframes (16 total)

`src/core/timeframe.rs` declares `Timeframe::ALL` in storage-index order.
Buckets align to Unix epoch multiples of their duration; weekly buckets start
on Thursday at 00:00 UTC.

| Index | Variant | Text | Seconds |
|---|---|---|---|
| 0 | Sec1 | 1s | 1 |
| 1 | Sec5 | 5s | 5 |
| 2 | Sec15 | 15s | 15 |
| 3 | Sec30 | 30s | 30 |
| 4 | Min1 | 1m | 60 |
| 5 | Min2 | 2m | 120 |
| 6 | Min3 | 3m | 180 |
| 7 | Min5 | 5m | 300 |
| 8 | Min15 | 15m | 900 |
| 9 | Min30 | 30m | 1800 |
| 10 | Hour1 | 1h | 3600 |
| 11 | Hour2 | 2h | 7200 |
| 12 | Hour4 | 4h | 14400 |
| 13 | Hour8 | 8h | 28800 |
| 14 | Day1 | 1D | 86400 |
| 15 | Week1 | 1W | 604800 |

`CandleBuilder` keeps one live bucket per market/timeframe (18 × 16 slots).
It emits `Live` updates for active buckets and `Closed` records when a later
bucket starts. The engine upserts closed candles; shutdown also upserts live
buckets. Earlier buckets cannot be changed by late ticks in the builder.
The tick store rejects timestamps older than that market's last stored tick;
equal timestamps preserve arrival order.

### Concurrency and memory

A writable `PerfDb::open` holds a filesystem writer lease for the database
lifetime. Mutations are serialized by the engine mutation mutex; tick storage,
candle storage, the candle builder and WAL writer have mutexes. Engine tick and
candle reads return copied vectors. Position and order tables use read/write
locks, and other state stores use sharded maps or locks. Latest price reads use
atomics. Pub/sub receivers can lag and lose older messages when their bounded
broadcast queue fills.

Tick and candle records are mmap-backed; state tables, event indexes, statistics,
live candles and broadcast queues occupy RAM. Memory use grows with retained
records. Archival, retention scheduling and background compaction require an
embedding application to arrange them.

### WAL, durability and recovery

`storage/wal.rs` implements `WalWriter` with a 64 KiB `BufWriter<File>`.
Segments are named `wal/wal_00000000.bin`, `wal/wal_00000001.bin`, and so on.
The default rotation threshold is 64 MiB; rotation syncs the old segment and
creates the next one.

Each WAL frame is `[len: u32 LE][crc32: u32 LE][payload: len bytes]`.
The payload is the complete bincode-serialized `WalEntry`, including its enum
discriminant. CRC32 covers the payload only; there is no separate type byte.
`WalEntry` variants are IndexedEvent, PriceTick, CandleUpdate, BalanceUpdate,
Checkpoint and BatchEnd. PriceTick WAL entries contain timestamp, market and
price, without the mmap record's padding.

`SyncPolicy::EveryWrite` flushes and calls `sync_data` after each append.
`SyncPolicy::Interval(Duration)` checks elapsed time on append and syncs when
due; the default interval is 10 ms. `SyncPolicy::None` leaves syncing to explicit
calls, rotation and the OS. An append acknowledged under Interval or None can be
lost on a crash before sync; there is no background timer in `WalWriter`.
Applications can call `sync_wal()` at batch boundaries. It syncs WAL and mmap
stores and atomically publishes the tick recovery checkpoint.

Writable startup loads the latest usable engine snapshot and recovery cursors,
opens and validates stores, rewinds uncommitted tick/candle suffixes, restores
in-memory tables and indexes, validates/repairs a damaged final WAL append, and
replays entries after the applicable cursors. Snapshot positions use segment and
byte offset, preserving order within a block. Corruption before a later valid
record or segment is rejected. Recovery rebuilds event indexes and candle state.

`snapshot()` persists the tick checkpoint and atomically publishes the engine
snapshot under `snapshots/`. `shutdown()` upserts live candles, flushes candle
storage, persists the tick checkpoint (including WAL sync), and writes a final
engine snapshot. The embedding application chooses when to invoke these methods.
Dropping the `PerfDb` releases the writer lease.

### Executable lifecycle

The `perfdb` binary parses one subcommand, opens the database, executes the
command, and exits. Query commands call `PerfDb::open_read_only`, which requires
an existing data directory, reads the committed tick prefix, and recovers
snapshot/WAL state privately in memory without repairing or creating files.
It can inspect a database while its writer is active. `snapshot` calls
`PerfDb::open` and needs the exclusive writer lease. The binary provides these
commands:

| Command | Purpose |
|---|---|
| info | Record counts and storage sizes |
| markets | Registered markets and status |
| prices | Latest prices, optionally for one market |
| ticks | Recent market ticks |
| candles | Recent candles for a market and timeframe |
| positions | Positions filtered by user, market, ID or open status |
| orders | Orders filtered by user, ID or active status |
| orderbook | Market price levels |
| balances | User balances |
| user | User profile and referrals |
| stats | Trader statistics |
| daily | Trader daily history |
| leaderboard | PnL or volume rankings |
| events | Events filtered by market or trader |
| checkpoint | Indexer progress |
| snapshot | Persist a state snapshot |

For example, from this crate directory after an application has created `./data`:

```bash
cargo run -- --data-dir ./data info
cargo run -- --data-dir ./data --json candles BTC 15m --last 10
cargo run -- --data-dir ./data snapshot
```

### Source map

| Source | Responsibility |
|---|---|
| `src/lib.rs`, `src/engine.rs` | Public `PerfDb` / `PerfDbConfig` API and engine coordination |
| `src/main.rs`, `src/formatting.rs` | CLI and fixed-point display |
| `src/core/types.rs`, `src/core/market.rs`, `src/core/timeframe.rs` | Types, 18-market registry and 16 timeframes |
| `src/storage/mmap.rs`, `src/storage/tick_store.rs`, `src/storage/candle_store.rs` | Mmap records and typed stores |
| `src/storage/wal.rs`, `src/storage/writer_lease.rs` | WAL codecs, recovery and writer ownership |
| `src/storage/snapshot.rs`, `src/storage/relational.rs` | Snapshot publication and durable relational coordinator |
| `src/storage/event_store.rs` | Global, market and trader event indexes |
| `src/state/positions.rs`, `src/state/orders.rs`, `src/state/market_state.rs`, `src/state/balances.rs`, `src/state/pubsub.rs` | Concurrent state tables and broadcast channels |
| `src/aggregation/candle_builder.rs`, `src/aggregation/stats.rs`, `src/aggregation/leaderboard.rs` | Candle, statistics and ranking computation |
| `src/query/time_range.rs` | Binary-search tick and candle ranges |
| `src/relational/users.rs`, `src/relational/referrals.rs`, `src/relational/competitions.rs`, `src/relational/alerts.rs`, `src/relational/api_keys.rs`, `src/relational/checkpoint.rs` | In-memory relational stores |
| `tests/integration.rs`, `tests/recovery.rs` | Engine lifecycle and recovery coverage |
| `benches/tick_ingestion.rs`, `benches/candle_query.rs`, `benches/position_lookup.rs`, `benches/price_read.rs` | Criterion benchmarks |

Embedding services supply ingestion, scheduling and network APIs around `PerfDb`.
Use `PerfDb::open(PerfDbConfig::new(path))` to open a writer; integration with
other services remains the responsibility of their owners.
