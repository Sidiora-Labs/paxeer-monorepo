# PerfDB

Embedded Rust database engine for trading applications.

## Contents

- [Overview](#overview)
- [Performance](#performance)
- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Usage](#usage)
- [Supported Timeframes](#supported-timeframes)
- [Durability and Recovery](#durability-and-recovery)
- [Benchmarks](#benchmarks)
- [Project Status](#project-status)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)
- [Related](#related)

## Overview

PerfDB is an embedded Rust database library for trading applications. It
coordinates mmap-backed price ticks and candles, in-memory state tables,
relational stores, event indexes and typed broadcast channels. Engine mutations
are serialized; position and order reads use read/write locks, balance reads
use a sharded map, and latest-price reads use atomics.

See [the architecture](ARCHITECTURE.md) for storage layouts and concurrency.

## Performance

The benchmark suite measures latest-price reads, candle queries, position
lookups and tick ingestion. Run it on the target hardware to assess those
workloads; see [Benchmarks](#benchmarks) for the available commands.

## Architecture

`PerfDb` coordinates these components:

| Workload | Components |
| --- | --- |
| Time series | `TickStore`, `CandleStore`, `CandleBuilder` |
| Market and account state | `MarketStateTable`, `PositionTable`, `OrderTable`, `BalanceCache` |
| Relational state | `UserStore`, `ReferralEngine`, `CheckpointStore`, `AlertStore`, `ApiKeyStore`, `CompetitionStore` |
| Statistics | `TraderStatsStore`, `DailyStatsStore`, `Leaderboard` |
| Events and streaming | `EventStore`, `PubSubBus` |
| Persistence | WAL segments, engine snapshots and mmap files |

## Quick Start

```rust
use perfdb::core::market::market_by_symbol;
use perfdb::core::types::PriceTick;
use perfdb::{PerfDb, PerfDbConfig};

pub fn ingest_btc(data_dir: impl AsRef<std::path::Path>) -> Result<(), perfdb::PerfDbError> {
    let config = PerfDbConfig::new(data_dir);
    let db = PerfDb::open(config)?;
    let btc = market_by_symbol("BTC").expect("BTC is a registered market");

    db.ingest_tick(&PriceTick {
        timestamp_ns: 1_699_900_000_000_000_000,
        market_id: btc.id, // BTC = 0; ETH = 1
        _pad: [0; 6],
        price: 43_250.50,
    })?;

    assert_eq!(db.latest_price(btc.id), 43_250.50);
    assert_eq!(db.ticks(btc.id).len(), 1);

    db.shutdown()?;
    Ok(())
}

fn main() -> Result<(), perfdb::PerfDbError> {
    ingest_btc("./data")
}
```

The example creates `./data`; start with an empty directory for its single-tick assertion.

## Installation

PerfDB source lives at
`services/platform/perf-db` in the monorepo. Use it as a git dependency
(Cargo finds the `perfdb` package in that directory):

```toml
[dependencies]
perfdb = { git = "https://github.com/Sidiora-Labs/paxeer-monorepo" }
```

For a local monorepo checkout, use a path dependency, adjusting the path
relative to your application's `Cargo.toml`:

```toml
[dependencies]
perfdb = { path = "../paxeer-monorepo/services/platform/perf-db" }
```

### From source

```bash
git clone https://github.com/Sidiora-Labs/paxeer-monorepo
cd paxeer-monorepo/services/platform/perf-db
cargo build --release
```

### System requirements

- Rust 1.85 or newer (stable), with edition 2024 support
- Unix writer locking via `flock`; opening a writer on a non-Unix platform returns an error
- SSD recommended for WAL and mmap performance

## Usage

### Opening a database

```rust
use perfdb::{PerfDb, PerfDbConfig};

let config = PerfDbConfig::new("/data/perfdb");
let db = PerfDb::open(config)?;
```

`PerfDb::open` loads the latest snapshot (if any), replays WAL entries
written after that snapshot's checkpoint, and opens the mmap-backed tick
and candle stores. See [Durability and Recovery](#durability-and-recovery).

### Time-series

```rust
db.ingest_tick(&tick)?;

let candles = db.candles(market_id, Timeframe::Min15);
let live = db.live_candle(market_id, Timeframe::Min15);
let price = db.latest_price(market_id);
```

### Real-time state

```rust
let position = db.positions.get(&position_id);
let user_positions = db.positions.by_user(&address);
let orderbook = db.orders.orderbook(market_id);
let balance = db.balances.get(&address, &token);
```

### Events

```rust
use perfdb::core::types::IndexedEvent;

db.process_event(&indexed_event)?;
let by_market = db.events.by_market(market_id, None, None);
let by_trader = db.events.by_trader(&address, None, None);
```

`process_event` writes to the WAL, appends to the event log, dispatches
the event to update positions/orders/balances/market state, and advances
the checkpoint.

### Pub/sub

```rust
use perfdb::state::pubsub::Channel;

let mut rx = db.pubsub.subscribe(Channel::Prices);
while let Ok(msg) = rx.recv().await {
    // handle PubSubMessage
}
```

### Leaderboards and stats

```rust
db.rebuild_leaderboard(now_ts);
let pnl_board = db.pnl_leaderboard(100);
let volume_board = db.volume_leaderboard(100);
```

### Shutdown

```rust
db.shutdown()?; // flushes candle builder, syncs WAL, writes final snapshot
```

## Supported Timeframes

PerfDB supports 16 candle timeframes, from sub-minute to weekly:

| Category | Variants |
|---|---|
| Sub-minute | `Sec1`, `Sec5`, `Sec15`, `Sec30` |
| Minute | `Min1`, `Min2`, `Min3`, `Min5`, `Min15`, `Min30` |
| Hourly | `Hour1`, `Hour2`, `Hour4`, `Hour8` |
| Daily+ | `Day1`, `Week1` |

```rust
use perfdb::core::timeframe::Timeframe;

let candles = db.candles(market_id, Timeframe::Min15);
```

## Durability and Recovery

`ingest_tick` and `process_event` append to the WAL before applying updates.
The default `SyncPolicy::Interval` checks for a sync on append at a 10 ms
interval; it has no background timer. Acknowledged writes can be lost on a crash
before sync. `SyncPolicy::EveryWrite` syncs each append; `SyncPolicy::None`
relies on explicit syncing and rotation. Direct mutations of public state
stores do not automatically append engine WAL entries.

`PerfDb::open` acquires an exclusive writer lease, restores the latest usable
snapshot and recovery cursors, validates stores, rewinds uncommitted time-series
suffixes, and replays WAL entries. It repairs a damaged final append; corruption
before later valid records is refused. `PerfDb::open_read_only` inspects an
existing database without acquiring a writer lease or modifying its files.

Call `sync_wal()` to sync the WAL and mmap stores and persist the tick checkpoint.
`snapshot()` publishes an engine snapshot; `shutdown()` also upserts live candles,
flushes stores and syncs the WAL. The embedding application schedules these
operations. Recovery cases are in [`tests/recovery.rs`](tests/recovery.rs).

## Benchmarks

```bash
cargo bench                        # full suite
cargo bench --bench tick_ingestion
cargo bench --bench candle_query
cargo bench --bench position_lookup
cargo bench --bench price_read
```

Criterion writes reports under the Cargo target directory, normally
`target/criterion/`.

## Project Status

PerfDB is pre-release. The on-disk WAL and snapshot formats, and the public
API, may change without a deprecation period before 1.0. It is being
developed as the storage backend for the Sidiora perpetual futures
platform's off-chain infrastructure; see `ARCHITECTURE.md` for the full
design rationale and the systems it replaces.

## Contributing

Contributions are welcome. See [`CONTRIBUTING.md`](../../../CONTRIBUTING.md) for
setup instructions, code standards, and the PR process.

```bash
git clone https://github.com/Sidiora-Labs/paxeer-monorepo
cd paxeer-monorepo/services/platform/perf-db
rustup component add clippy rustfmt
cargo test
cargo clippy --all-targets --all-features -- -D warnings
cargo fmt
```

## Security

See [`SECURITY.md`](../../../SECURITY.md) for the vulnerability disclosure process.

## License

Licensed under Apache-2.0 (Apache License, Version 2.0). See the repository
[LICENSE](../../../LICENSE) or <http://www.apache.org/licenses/LICENSE-2.0>.

### Acknowledgments

- [`memmap2`](https://crates.io/crates/memmap2) — memory-mapped file I/O
- [`tokio`](https://tokio.rs/) — async runtime and broadcast channels
- [`dashmap`](https://crates.io/crates/dashmap) / [`parking_lot`](https://crates.io/crates/parking_lot) — concurrent data structures
- [`criterion`](https://crates.io/crates/criterion) — benchmarking framework

## Related

- [Paxeer Network](https://paxeer.app) — BFT network with EVM chain ID 125. A receipt is final when its block is committed.

---

<p align="center">
  <em>Four databases. One library. Zero network hops.</em>
</p>

<p align="center">
  Built by <strong>Sidiora Labs</strong>
</p>

<p align="center">
  <sub>SPDX-License-Identifier: Apache-2.0</sub>
</p>
