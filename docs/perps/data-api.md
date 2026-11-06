# Paxeer Frontend Integration Guide

REST and WebSocket reference for the perps data gateway in
[`services/platform/perps-data`](../../services/platform/perps-data). The frontend
sets `NEXT_PUBLIC_PERPS_GATEWAY_URL` to its bare HTTP(S) origin, without a path,
credentials, query or fragment. [`gatewayUrls`](../../apps/perps/src/core/network/gateway.ts)
derives `/api`, `/ws`, and `/graphql` from that origin. Symbol, market and chain
topics share this gateway; separate per-asset ports are not required.

Paxeer mainnet-beta is one BFT network with EVM chain ID **125 (`0x7d`)**. Its
kernel participates in block proposal and receipts are final when their block
commits. Public node JSON-RPC uses the root of the `api1` through `api14` hosts
under `mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`, for
example `https://api1.mainnet-beta.paxeer.network`. Those node endpoints are
separate from the application data gateway described here. Tenant sub-chains
are database domains, not additional networks to deploy for this UI.

See [deployment and environment](../../apps/perps/deploy/README.md),
[settings behavior](../../apps/perps/src/app/modals/SettingsModal.md), and the
[component integration guide](../../apps/perps/src/components/paxeer/README.md).

## Table of contents

1. [WebSocket protocol (shared by all services)](#websocket-protocol-shared-by-all-services)
2. [Symbol Service](#symbol-service)
3. [Markets Service](#markets-service)
4. [Chain Explorer Service](#chain-explorer-service)

The section names group data families in the gateway. HTTP bodies below use
JSON; `ts` and trade times are epoch milliseconds, and ISO timestamps are UTC.
Examples illustrate payloads rather than current market values.

## WebSocket protocol (shared by all services)

Connect to the gateway's `/ws` using `ws:` or `wss:` to match its HTTP scheme.
The shared topic protocol is public, read-only and unauthenticated, with no
subprotocol. `/` is also accepted. Convenience paths automatically subscribe:

| Path | Initial topics |
| --- | --- |
| `/ws/markets` | `markets@all`, `markets@totals` |
| `/ws/explorer` | `chain@blocks`, `chain@transactions` |
| `/ws/<symbol>` | Lowercase symbol path, e.g. `/ws/btc`: its perpetual book, trades and stats; PAX and SID also get their spot topics |
| `/ws` or `/` | None; send subscribe frames |

`/graphql` uses the separate GraphQL transport; `/v1/ws` uses the spot user
protocol. Do not send the shared topic frames to those sockets.

### Client → Server frames

Send each object as an individual frame:

```json
{ "event": "subscribe", "topic": "BTC@perp_stats" }
```

```json
{ "event": "unsubscribe", "topic": "BTC@perp_stats" }
```

```json
{ "event": "ping" }
```

The codec accepts an optional unsigned `limit` on subscribe, but the current
providers use fixed snapshot depths. The gateway has no kline topic.

### Server → Client frames

Control frames have `event` and millisecond `ts`. `welcome` and `subscribed`
also carry `snapshotId`; subscription acknowledgements name the `topic`.
`unsubscribed` names the topic, and `pong` answers an application ping.

```json
{
  "event": "error",
  "message": "unknown_topic",
  "topic": "BTC@kline_1m",
  "ts": 1747526400000
}
```

Error messages are `invalid_json`, `missing_topic`, `unknown_event`,
`unknown_topic`, `subscribe_rate_limit` and `too_many_subscriptions`. These
protocol errors leave the socket open.

Every topic frame uses this envelope:

```json
{
  "topic": "BTC@perp_stats",
  "ts": 1747526400000,
  "snapshotId": "opaque-process-id",
  "seq": 12,
  "snapshot": true,
  "data": {
    "symbol": "BTC",
    "volume_24h": 12345678.9,
    "open_interest": 98765432.1,
    "last_funding_rate_hourly": "0.0000125",
    "last_funding_rate": 0.0000125,
    "mark_price": 77182.4,
    "index_price": 77173,
    "basis_bps": 1.218,
    "long_short_ratio": 1.03,
    "liq_volume_24h": 0
  }
}
```

`snapshot` is omitted on live frames. Payload shapes depend on the topic.

### Stream consistency contract

- `snapshotId` identifies the gateway process. On a changed ID, discard the
  previous sequence baseline and acquire new snapshots.
- `seq` is dense and increasing per topic, not global and not a block height.
- A `subscribed` acknowledgement is followed by a snapshot carrying the topic's
  latest sequence. Apply that snapshot, then accept newer live frames.
- The server can send another `snapshot: true` after recovery; replace the
  corresponding local state with it.
- On a sequence gap, reconnect or resubscribe for a new socket snapshot. REST
  market responses have no `seq` or `snapshotId`, so they cannot establish a
  WebSocket sequence baseline.

### Connection lifetime

Default caps are 200 subscriptions and 50 subscribe operations per second per
socket. The server sends protocol pings every 30 seconds and closes sockets
that miss two ping intervals. WebSocket libraries normally answer these pings
automatically. A server under load or unable to build a snapshot may close with
code `1013` and reason `retry-after=<seconds>` (5–30 seconds); respect that delay.

Source: [`internal/ws`](../../services/platform/perps-data/internal/ws).

## Symbol Service

Symbol REST routes live under `/api/<symbol>/` on the gateway origin. Symbol
matching is case-insensitive. All registered perps markets have perpetual
topics; spot topics are provided for **PAX and SID**. A BTC spot trade request
can return an empty list; it is not an external BTC spot tape.

### REST — Market data

| Method and path | Body |
| --- | --- |
| `GET /api/btc/price` | `symbol`, nullable `price`, `timestamp`, `open`, `high`, `low` |
| `GET /api/btc/stats` | Symbol stats below |
| `GET /api/btc/perp_stats` | Stats plus nullable `mark_price`, `index_price`, `basis_bps`, `long_short_ratio`, and numeric `liq_volume_24h` |
| `GET /api/btc/trades?limit=100` | `{ "symbol": "BTC", "count": 0, "trades": [] }` for an asset without registered spot trades |
| `GET /api/btc/perp_trades?limit=100` | `{ "symbol": "BTC", "count": 1, "trades": [ ... ] }` |
| `GET /api/btc/health` | `{ "status": "ok", "backfill": "complete", "lastFill": null, "symbol": "BTC" }` |

`price` is the latest index price; open/high/low come from fills in the last
24 hours. They can be `null` independently. Health reports `backfill: "pending"`
until the head cursor exists; `lastFill` is an ISO timestamp or `null`.

Symbol stats:

```json
{
  "symbol": "BTC",
  "volume_24h": 12345678.9,
  "open_interest": 98765432.1,
  "last_funding_rate_hourly": "0.0000125",
  "last_funding_rate": 0.0000125
}
```

`volume_24h` is USD fill notional over the rolling 24-hour window;
`open_interest` sums long and short USD open interest. These are JSON numbers,
including zero when no fills or open interest have been recorded.
`last_funding_rate_hourly` is the exact signed decimal **fraction per
hour** from the latest `FundingSettled` row, scaled from the chain's 18-decimal
integer. `"0.0000125"` means `0.00125%` per hour. It is `null` before the first
settlement. `last_funding_rate` is a nullable JSON-number compatibility copy of
the same settled hourly rate. Neither stats response carries
`est_funding_rate`, a predicted `funding_rate`, or `next_funding_ms`.

#### Perpetual stats wire example

`GET /api/btc/perp_stats` and the `data` body of `BTC@perp_stats` use
[`records.PerpStats`](../../services/platform/perps-data/internal/records/records.go),
which embeds `records.Stats`. This example uses the settled-rate and notional
values of the gateway's symbol-route fixture:

```json
{
  "symbol": "BTC",
  "volume_24h": 3500,
  "open_interest": 1000000,
  "last_funding_rate_hourly": "0.0000125",
  "last_funding_rate": 0.0000125,
  "mark_price": 77182.4,
  "index_price": 77173,
  "basis_bps": 1.218,
  "long_short_ratio": 1.5,
  "liq_volume_24h": 500
}
```

#### Perpetual stats before settlement

Prices, basis and long/short ratio are nullable independently. Missing funding
is `null`, never a zero rate. An empty registered market can send:

```json
{
  "symbol": "ETH",
  "volume_24h": 0,
  "open_interest": 0,
  "last_funding_rate_hourly": null,
  "last_funding_rate": null,
  "mark_price": null,
  "index_price": null,
  "basis_bps": null,
  "long_short_ratio": null,
  "liq_volume_24h": 0
}
```

Trade lists are newest first. `limit` defaults to 100 and is clamped to 1–1000;
malformed or repeated limits return HTTP 400 with `{ "error": "invalid limit" }`.
The optional trailing symbol on `/api/btc/trades/BTC` or
`/api/btc/perp_trades/BTC` must match the prefix.

Spot trade entries have `id`, `side` (`BUY`/`SELL`), numeric `price`, `size`
(base token units), and millisecond `trade_time`. Perpetual trade entries use
`contracts` (base-equivalent size, which can be fractional), USD `notional`,
and boolean `liquidation` in place of `size`. The ID is built from block and
log index, not a wall-clock trade ID.

Unknown symbols return HTTP 404 with `{ "error": "Unknown symbol: BTC" }`
(using the requested uppercase symbol); unsupported symbol routes return
`{ "error": "Not found" }`. Database failures return HTTP 500 with
`{ "error": "internal error" }`.

### REST — TradingView UDF datafeed

Candles remain on the external Chainflow feed; the perps data gateway does not
serve symbol `config`, `history`, `symbols`, `search`, `time` or `quotes` routes.
The Next.js frontend forwards chart requests through its own
`/api/candle/<endpoint>` route, implemented in
[`src/pages/api/candle/[...path].ts`](../../apps/perps/src/pages/api/candle/%5B...path%5D.ts).
It accepts GET/HEAD for `config`, `symbols`, `search`, `history`, `marks`,
`timescale_marks`, `time` and `quotes`, with allowlisted query parameters.

A typical history request is
`/api/candle/history?symbol=BTC&resolution=1&from=1747522800&to=1747526400`.
UDF times are seconds; candles use the upstream UDF arrays (`t`, `o`, `h`, `l`,
`c`, `v`). The frontend's `MARKET_PROXY_HOST` and required bare
`MARKET_PROXY_ORIGIN` configure this proxy; see [deployment](../../apps/perps/deploy/README.md).
Missing origin returns HTTP 503, and upstream timeout/unreachability produces
504/502. There is no gateway `<SYMBOL>@kline_<interval>` subscription.

### WebSocket topics

| Topic | Subscribe snapshot | Live payload |
| --- | --- | --- |
| `<SYMBOL>@perp_orderbook` | Up to 100 levels per side | Up to 20 levels per side, at most 10 frames/second |
| `<SYMBOL>@perp_trade` | `{ "symbol": "BTC", "trades": [ ... ] }`, latest 100 newest first | One perpetual trade |
| `<SYMBOL>@perp_stats` | Same fields as symbol REST `perp_stats` | Full stats every 30 seconds |
| `PAX@orderbook`, `SID@orderbook` | Up to 100 spot levels per side | Up to 20 levels per side |
| `PAX@trade`, `SID@trade` | Latest 100 spot trades in a `trades` wrapper | One spot trade |
| `PAX@stats`, `SID@stats` | Same shape as REST `stats` | Full stats every 30 seconds |

Book payloads contain `symbol`, `asks` ascending and `bids` descending. Each
level is `[price, size]`. Perpetual sizes are fractional contracts computed as
remaining USD notional divided by the limit price; spot sizes are base token
units. Perpetual books also contain nullable `mark_price`, `index_price` and
`basis_bps`. Live frames replace the displayed top levels and periodically
include a CRC32 `checksum` for the top 20 levels. Do not interpret them as
individual order inserts/deletes or assume integer sizes. The venue REST book
(`/api/orderbook/BTC`) instead uses `{price, quantity}` objects with USD
notional quantities; its payload is not interchangeable with the socket book.

The current symbol adapter
[`useSidPaxMarketStream`](../../apps/perps/src/hooks/perps/useSidPaxMarketStream.ts) converts the
numeric `last_funding_rate` compatibility field to a nullable decimal string
with `fundingRateFromJsonNumber`; an already plain decimal string also passes
unchanged. The wire's `last_funding_rate_hourly` remains the authoritative exact
on-chain hourly rate; the compatibility number can lose decimal precision.

| Gateway wire field | Adapter field | Meaning |
| --- | --- | --- |
| `volume_24h` | `volume_24h_usd` and `volume_24h` | Rolling USD fill notional |
| `open_interest` | `oi_usd` and `open_interest` | Long plus short USD open interest |
| `last_funding_rate` | `last_funding_rate` | Nullable plain decimal string converted from the compatibility number |
| No `funding_rate` in the on-chain body | `funding_rate: null` | No predicted rate supplied |

`volume_24h_usd` and `oi_usd` are client aliases, not additional fields promised
by `/perp_stats`. Contract quantities are not implied by these USD values.
Optional external reference estimates use `funding_rate` in the adapter and
are separate from the last on-chain settlement; their period is defined by
the reference source, not by the on-chain hourly rate. `est_funding_rate` is
not a field of the current gateway contract. Absent predicted funding remains `null`;
[`fundingRowFigure`](../../apps/perps/src/core/perps/fundingRate.ts) then displays the venue's
last settled hourly rate. A connected symbol socket does not imply an estimate
is available. Multiplying the settled hourly rate by 24 is a projection of the
last settlement, not an estimate of the next funding payment.

## Markets Service

### REST

| Method and path | Response |
| --- | --- |
| `GET /api/markets` | `{ "ts": 1747526400000, "markets": [ ... ] }`, market ID order |
| `GET /api/markets/btc` | `{ "ts": 1747526400000, "market": { ... } }`, case-insensitive symbol |
| `GET /api/totals` | `{ "ts": 1747526400000, "totals": { ... } }` |
| `GET /api/markets/health` | `{ "ok": true, "service": "perps-gateway", "symbols": 18, "uptimeSec": 12345 }` (count varies with registrations) |

A market record has this shape:

```json
{
  "market_id": "0",
  "symbol": "BTC",
  "symbol_full": "Bitcoin",
  "price": 77173,
  "change_24h": 0.0123,
  "volume_24h": 1234567.8,
  "open_interest": 987654.3,
  "funding_8h": "0.0001",
  "last_funding_rate_hourly": "0.0000125",
  "volume_7d": 9876543.2,
  "volume_30d": 45678901.2,
  "trades_count": 123456,
  "last_updated": 1747526399000,
  "perp_mark_price": 77182.4,
  "perp_index_price": 77173,
  "perp_basis_bps": 1.218,
  "perp_funding_rate": "0.0000125",
  "perp_next_funding_ms": 1747530000000,
  "perp_long_short_ratio": 1.03,
  "perp_liq_volume_24h": 0,
  "perp_oi_contracts": 12.8,
  "perp_oi_usd": 987654.3,
  "perp_volume_24h_contracts": 16,
  "perp_volume_24h_usd": 1234567.8
}
```

`market_id` is a decimal **string**. Prices/notionals are USD, `change_24h` is a
fraction (0.0123 means 1.23%), and `perp_basis_bps` is in basis points.
`price`, `change_24h`, `last_updated`, mark/index/basis, long/short ratio,
`perp_oi_contracts` and `perp_oi_usd` are nullable in the record type.
Volumes, trade count and top-level open interest are numeric; empty aggregate
windows yield zero. A zero short OI leaves the long/short ratio `null`.
`last_updated` reflects a mark block in REST and the rolling-statistics update
block in market topic records; it is not a per-field freshness guarantee.

### Funding contract

| Field | Wire type | Meaning |
| --- | --- | --- |
| `last_funding_rate_hourly` | decimal string or `null` | Latest settled hourly fraction |
| `perp_funding_rate` | decimal string or `null` | Same settled hourly fraction |
| `funding_8h` | decimal string or `null` | Exactly eight times that settled hourly fraction |
| `perp_next_funding_ms` | integer milliseconds | Next whole UTC hour at response construction |

The three rates come from the same latest `FundingSettled` database row and
are all `null` before one exists. `"0"` is an observed zero rate; `null` is
unavailable. The eight-hour value is a multiplication, not a prediction or a
claim that settlement runs every eight hours. The next-hour timestamp does
not guarantee a future settlement or price. Market records have no estimated
funding field.

[`useMarketsStream`](../../apps/perps/src/hooks/perps/useMarketsStream.ts) parses these strings
as signed 18-decimal `bigint` values; numbers and exponent notation are not
accepted as exact funding inputs. A malformed field becomes `null` with a
rate-limited warning for that symbol/field. Preserve nulls when rendering;
do not coerce unavailable funding to zero. The 24-hour figure is the last
settled hourly rate multiplied by 24, explicitly labeled as a projection.

Totals contain numeric `total_trades`, `volume_24h`, `volume_7d`, `volume_30d`
and `total_oi`, derived from rolling statistics. Unknown market symbols return
HTTP 404 with `{ "error": "Unknown symbol: NOSUCH" }`; database failures return
HTTP 500 with `{ "error": "internal error" }`.

### WebSocket topics

- `markets@all`: `data` is the full market-record array.
- `markets@totals`: `data` is the totals object.
- `<SYMBOL>@market`: `data` is one market record.

These topics send subscribe snapshots and full replacements every five
seconds while watched. Funding types and units match REST. The frame envelope
supplies the sequence; REST wrappers (`ts`, `markets`, `market`, `totals`) are
not part of the topic's `data`.

Sources: [`records.go`](../../services/platform/perps-data/internal/records/records.go),
[`rest/markets.go`](../../services/platform/perps-data/internal/rest/markets.go),
and [`feeds/topics.go`](../../services/platform/perps-data/internal/feeds/topics.go).

## Chain Explorer Service

The gateway serves chain data as `chain@blocks` and `chain@transactions` topics
on the shared socket. `/ws/explorer` subscribes to both. It does not register
the former standalone `/blocks`, `/transactions` or explorer `/health` REST
routes; use the socket snapshots for this protocol's recent history.

`chain@blocks` sends the latest 20 blocks, newest first, as the snapshot array;
a live frame carries one block:

```json
{
  "block_number": 1234567,
  "time": "2026-05-18T00:00:00.123Z",
  "proposer": "AB12CD34",
  "hash": "0x0123",
  "transactions": 142
}
```

`chain@transactions` snapshots the latest 50 indexed action rows, newest first;
a live frame carries one action:

```json
{
  "hash": "0x0123",
  "action": "Place Order",
  "block": 1234567,
  "time": "2026-05-18T00:00:00.123Z",
  "user": "pax1...",
  "symbol": "BTC",
  "side": "buy",
  "price": 77175.2,
  "size": 0.42
}
```

`user` uses the `pax` Bech32 prefix. `symbol`, `side`, `price` and `size` may be
`null` for actions without those attributes. One transaction may produce more
than one action row. Prices and amount sizes are scaled from their emitter's
units; `Oracle Updated` and `Sealed Batch Executed` sizes are counts. This
stream describes committed network activity, not a second finality stage.
