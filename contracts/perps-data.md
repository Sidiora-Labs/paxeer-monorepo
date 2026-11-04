# Perps data service contract

The services under [services/platform/perps-data](../services/platform/perps-data)
index and serve the native [market venues](markets.md) on Paxeer mainnet-beta,
chain ID 125 (`0x7d`). They read the same BFT chain that executes transactions;
a receipt is final when its block commits. The kernel belongs to that network
and proposes blocks. An indexer cursor or a socket acknowledgment reports data
service progress, not a second finality stage.

The [perps client](perps-client.md) uses the data gateway for REST, GraphQL and
subscriptions, and node JSON-RPC for chain reads and transaction submission.
Public JSON-RPC is HTTPS at the root of `api1.mainnet-beta.paxeer.network`
through `api14.mainnet-beta.paxeer.network`, and
`api-mainnet-beta.paxeer.network`.
`node.hyperpaxeer.com` is the hpx library mirror, not an RPC endpoint.
Sub-chains are tenant domains in the database;
`PERPS_CHAIN_ID` identifies this service's chain, not a new consensus network.

## CLI

The [cmd directory](../services/platform/perps-data/cmd) contains these entry
points. Long-running commands load their named service configuration and handle
termination signals.

| Binary | Responsibility |
| --- | --- |
| `perps-indexer` | Migrations, chain following, facts/projections, outbox publication and market discovery |
| `perps-gateway` | REST, GraphQL, topic sockets and spot account sockets |
| `perps-explorer` | Indexed chain history API |
| `perps-portal` | HTML market/account portal reading the gateway |
| `perps-replay` | Bounded historical replay with optional dry run |
| `perps-loadtest` | Gateway socket load profile and report |

`perps-replay --from <height> [--to <height>] [--dry-run]` requires `--from`.
Omitted `--to` resolves to the stored cursor. Read the
[replay implementation](../services/platform/perps-data/internal/replay/replay.go)
before operating on a populated database.

## Configuration

[config.go](../services/platform/perps-data/internal/config/config.go) reads only
the variables a named service uses, except `PERPS_CHAIN_ID`, which is required
for all six services. Configuration is loaded at startup; it has no chain-ID,
port or database default.

| Variable | Consumers | Contract |
| --- | --- | --- |
| `PERPS_CHAIN_ID` | All | Nonzero decimal uint64; use `125` for mainnet-beta |
| `PERPS_RPC_URLS` | Indexer, gateway, replay | Nonempty comma-separated HTTP(S) node URLs |
| `PERPS_COMET_URLS` | Indexer, replay | Nonempty comma-separated HTTP(S) Comet URLs; configure the actual Comet listener separately from EVM JSON-RPC |
| `DATABASE_URL` | Indexer, gateway, explorer, replay | Required Postgres connection string |
| `NATS_URL` | Indexer, gateway | Required broker connection string |
| `PORT` | Indexer, gateway, explorer, portal | Integer from 1 to 65535 |
| `PERPS_START_HEIGHT` | Indexer | Explicit first indexed height, at least 1; mainnet uses the native-market activation height |
| `PERPS_AUTH_SECRET` | Gateway | At least 32 bytes; server-side HS256 signing secret |
| `PERPS_EXPLORER_TX_BASE` | Explorer, portal | Optional HTTP(S) transaction-link base |
| `PERPS_GATEWAY_URL` | Portal, loadtest | Required HTTP(S) or WS(S) gateway URL |
| `PERPS_TRUSTED_PROXY_CIDRS` | Gateway, explorer | Valid proxy CIDRs for forwarded client identity |

`Config.String`, `GoString` and structured logging redact the database URL,
NATS URL and auth secret. The broker's separate
[nats.conf](../services/platform/perps-data/deploy/nats/nats.conf) consumes
`NATS_ROUTES`, `NATS_CLUSTER_PASSWORD`, `NATS_INDEXER_PASSWORD`,
`NATS_GATEWAY_PASSWORD` and `NATS_MAX_FILE_STORE`.
The `PERPS_DATA_TEST_*` variables in `e2e_test.go` are test inputs, not service
configuration.

## Indexer

[Indexer](../services/platform/perps-data/internal/indexer/indexer.go) owns a
Postgres writer lease and resumes from the stored cursor. The
[source follower](../services/platform/perps-data/internal/source/follower.go)
reads blocks and receipts, orders their facts and handles branch reconciliation.
Each block's facts, projections, cursor and delivery outbox are written
transactionally; publication drains the durable outbox.

The writer lease lasts 30 seconds and renews every 10 seconds. Loss of the
lease returns `ErrLeaseLost`. On an empty cursor the configured start height
sets the first block. Restart recovery uses persisted state rather than an
in-memory head counter.

## Decode

[The registry](../services/platform/perps-data/internal/decode/registry.go)
selects a decoder by emitter and event topic. The native sources include perps
`0x1015`, pricefeed `0x1020`, the five spot rails `0x1021`–`0x1025` and discovered
share pointers. ABI sources are linked in [markets](markets.md#addresses-and-abi-sources).

### Native share decoding

[spot.go](../services/platform/perps-data/internal/decode/spot.go) loads the
deployed `Share()` ABI, with `Transfer` and `Approval`, rather than the historical
Solidity ownership ABI. A share emitter is a dynamic pointer, not `0x1021`–`0x1025`.
Resolution checks `ShareTokenCreated` earlier in the batch, the registry's
pointer cache, persisted `perps.spot_token_registry` rows, then
`getRegisteredTokens()` and `shareTokens(token)` on `0x1025`. The last lookup
uses latest chain state. A restarted decoder can recover the association from
the persisted row or chain reads; an unresolved pointer produces `share_lookup`.

The creation row carries `token` and `shareToken`; transfer rows carry the
pointer, underlying token, `from`, `to` and exact decimal-string `value`.
Approval rows carry the pointer, underlying token, owner account, spender and
exact decimal-string `value`. Zero-source and zero-destination transfers
represent keeper mint and burn. Filter by the resolved emitter as well as
topic: another ERC20's `Transfer` is not a vault share transfer. The native
[share lifecycle and refusals](markets.md#spot-share-token) are the producer
contract consumed here.

Receipt kinds are `tx`, `perps_endblock`, `spot_endblock`, `activation` and
`block_event`. Synthetic receipt hashes are
`keccak256(tag || uint64-big-endian(height))`, using `perps-endblock`,
`spot-endblock` and `v6120-activation` tags. They preserve events emitted outside
ordinary EVM transactions.

Unknown or undecodable venue logs are retained in `perps.raw_logs` rather than
dropped. Failure classes include `unknown_topic`, `abi_decode`, `order_lookup`,
`share_lookup`, `account_lookup` and `projection_lookup`. Decoder admission
bounds include 64 KiB fields, 4096 logs and 16 MiB per raw batch.

## Storage

[Embedded migrations](../services/platform/perps-data/migrations) define schema
`perps`: chain blocks and transactions, immutable venue facts, current positions
and orders, account and market projections, raw logs, cursors, writer leases,
auth records and delivery state. The indexer runs migrations before following.

Fact ordering uses block, transaction index, log index and sub-index. Store
256-bit amounts as exact decimal integers; the
[bigint helpers](../services/platform/perps-data/internal/bigint/bigint.go)
validate their canonical representation. Reconciliation removes affected facts
and rebuilds projections through the follower; it does not erase the market
registry or fee configuration.

## Delivery

[Envelope](../services/platform/perps-data/internal/delivery/envelope.go) carries
`event_id`, `event_type`, `schema_version`, `chain_id`, `source_order`,
`emitted_at`, `trace_id`, `payload_hash` and `payload`. Source-order components
`height`, `transaction_index` and `log_index` are decimal strings. Payload hashes
use SHA-256; the accepted schema version is 1. Subject admission is server-owned;
arbitrary public subjects are not accepted.

The [outbox/inbox store](../services/platform/perps-data/internal/delivery/store.go)
and [publisher](../services/platform/perps-data/internal/publish/publisher.go)
preserve immutable event identity and broker ordering across retries. Consumers
must deduplicate; broker delivery is not a claim of exactly-once application.
The transport limit is 1 MiB including mandatory headers; `MaxEnvelopeBytes`
reserves their space before accepting a body.

[PERPS_V1](../services/platform/perps-data/internal/stream/layout.go) uses file
storage, three replicas, 72-hour retention, a two-minute deduplication window
and discard-new behavior. Subject families are:

| Family | Subject |
| --- | --- |
| Market | `perps.<chain>.<market>.<kind>` |
| Account | `perps.<chain>.user.<lowercase-address>.<kind>` |
| Chain | `perps.<chain>.chain.<kind>` |
| Spot | `perps.<chain>.spot.<lowercase-token>.<kind>` |

Kinds are enumerated in `stream/layout.go`; clients cannot invent additional
kinds. This internal envelope protocol is distinct from public socket frames.

## Gateway

[Server registration](../services/platform/perps-data/internal/gateway/server.go)
mounts REST, GraphQL, sockets, auth, probes and metrics. It does not mount a node
JSON-RPC proxy. Configure chain transports independently.

REST and GraphQL have a 600-request per-minute client budget and a global
10,000-request in-flight cap. Forwarded identity is trusted only from configured
proxies. Long-lived socket routes bypass those request counters and use their
own limits. GraphQL request bodies are bounded to 1 MiB.

## REST

All paths below are relative to the data gateway origin.

| Method/path | Result |
| --- | --- |
| `GET /api/markets` | `{ts, markets}` |
| `GET /api/markets/{symbol}` | `{ts, market}` |
| `GET /api/markets/health` | Market availability/health |
| `GET /api/totals` | `{ts, totals}` |
| `GET /api/{symbol}/price` | Symbol price |
| `GET /api/{symbol}/stats` or `/perp_stats` | Symbol statistics |
| `GET /api/{symbol}/trades` or `/perp_trades` | Trade history; default limit 100, maximum 1000 |
| `GET /api/{symbol}/health` | Symbol health |
| `GET /api/orderbook/{symbol}` | `{success, data: {asks, bids}}`; depth defaults to 20 and is clamped to 1–100 |
| `GET /api/open-interest`, `/api/ticker/{symbol}`, `/api/tickers` | Venue OI and ticker snapshots |
| `GET /v1/users/{address}/{resource}` | Spot holdings, positions, orders, trades, trade stats, closed legs, PnL or identity |
| `GET /v1/leaderboards/{kind}` | Spot leaderboard |
| `GET /v2/user/{address}/daily-pnl`, `/v2/leaderboard` | Public perps account analytics |

Venue book/OI/ticker routes also have `/api/markets`-prefixed aliases. Spot
resource suffixes are `holdings`, `positions`, `orders`, `trades`, `trades/stats`,
`closed-legs`, `pnl/summary`, `pnl` and `identity`. Exact query parameters and
response types live in the [REST handlers](../services/platform/perps-data/internal/rest).
Unknown symbols produce 404. Unavailable venue snapshots produce 503; node
failures on spot account reads can produce 502. Preserve unavailable/null data
rather than replacing it with a successful zero.

Chart candles stay on the configured Chainflow feed; the gateway's symbol
routes do not serve TradingView history.

## GraphQL

`POST /graphql` accepts queries; `POST /api/subgraph` is an alias. Subscriptions
use `graphql-transport-ws` at `GET /graphql`, also available at `/graphql-ws`.
The [SDL and resolver setup](../services/platform/perps-data/internal/graphql/schema.go)
are the field/type contract, with maximum depth 10 and parallelism 20.

Queries cover markets, positions, orders, trades, funding, liquidation, fees,
account ledger, spot data, protocol statistics and `indexerStatus`.
Subscriptions include `indexerStatusUpdated`, `latestPricesUpdated`,
`priceUpdated`, `positionChanged`, `orderChanged`, `tradeCreated`,
`liquidationCreated` and `tradeExecuted`. `BigInt`, `BigDecimal` and `DateTime`
are explicit scalars; raw `BigInt` fields such as margin-tier bounds are
canonical decimal strings. Use each field's declared units and nullability.

## WebSocket

The public topic protocol is available at `/ws`, the root socket and
`/ws/markets`, `/ws/explorer`, `/ws/{symbol}`. It is read-only and unauthenticated.
[protocol.go](../services/platform/perps-data/internal/ws/protocol.go) defines
these client frames:

```json
{"event":"subscribe","topic":"BTC@perp_orderbook","limit":20}
{"event":"unsubscribe","topic":"BTC@perp_orderbook"}
{"event":"ping"}
```

Each connection begins with `welcome`; controls include `subscribed`,
`unsubscribed`, `pong` and `error`. Controls carry millisecond `ts`, and applicable
snapshot controls carry `snapshotId`. Data frames carry `topic`, `ts`,
`snapshotId`, `seq`, `data`, and `snapshot:true` on a snapshot. Use the snapshot
as a fresh baseline; do not continue deltas across a changed snapshot identity
or a sequence gap.

[Topic providers](../services/platform/perps-data/internal/feeds/topics.go) serve
`markets@all`, `markets@totals`, `chain@blocks`, `chain@transactions`, and
registered-symbol `@perp_orderbook`, `@perp_trade`, `@perp_stats` and `@market`
topics. Spot `@orderbook`, `@trade` and `@stats` are served for PAX and SID.
Defaults include 100 trade, 20 block and 50 transaction snapshot items, 100 ms
book updates, five-second book checksums and 30-second stats refreshes.

A socket permits 200 subscriptions and 50 subscribe operations per second,
with 4096-byte client frames and 30-second server pings. Error messages are
`invalid_json`, `missing_topic`, `unknown_event`, `unknown_topic`,
`subscribe_rate_limit` and `too_many_subscriptions`; these leave the socket open.
Load shedding closes with code 1013 and `retry-after=<seconds>` (5–30 seconds).
Reconnect and obtain fresh snapshots after closure or loss of continuity.

`/v1/ws` is a separate
[spot account protocol](../services/platform/perps-data/internal/ws/spotuser.go),
with streams `trade`, `position`, `pnl`, `order` and `closed-leg`, keyed by
address. Its 50-subscription limit and `{stream,address,data}` items are not the
shared topic frame shape.

## Auth

[Wallet sign-in](../services/platform/perps-data/internal/auth/auth.go) is:

1. `POST /v2/auth/challenge` with `{address}` returns `{nonce,message}`.
2. Sign the returned message using EIP-191, then `POST /v2/auth/verify` with
   `{address,signature,message}`. Successful verification consumes the challenge
   and returns `{token,expires_in}`.
3. Send `Authorization: Bearer <token>` to protected account operations.
   `POST /v2/auth/refresh` refreshes within the original sign-in lifetime.

Challenges live five minutes. HS256 tokens live one hour; refresh chains last
at most 24 hours. Middleware derives the acting address from the token.
Protected routes are `POST /v2/alerts`, `DELETE /v2/alerts/{id}`,
`POST /v2/referrals/generate`, `POST /v2/referrals/apply` and
`POST /v2/competitions/{id}/join`. Public market/history reads do not require
wallet sign-in.

## Markets

[Discovery](../services/platform/perps-data/internal/markets/markets.go) reads
native market configuration into `perps.markets`. The gateway and GraphQL
schema consume that registry. A new symbol needs authoritative market data;
a client label alone does not create a tradable market.

## Records

[records.go](../services/platform/perps-data/internal/records/records.go) defines
the REST/socket market records. Display prices and notionals are converted
from 18-decimal integers; opening spot amounts use six decimals. Market IDs
are strings in `MarketRecord`, and timestamps use epoch milliseconds where
specified. Funding fields `last_funding_rate_hourly` and `perp_funding_rate`
are exact decimal strings of the last settled hourly rate; `funding_8h` is eight
times that rate. These fields are null before the first settlement. Other
missing-source values retain their declared nullability.

## PnL

[PnL math](../services/platform/perps-data/internal/pnl/pnl.go) uses exact
`math/big` arithmetic on decimal strings. Spot swaps fold average cost;
perpetual realized PnL comes from `PositionClosed` and `TradeSettled`
components. A missing price for nonempty spot inventory is unavailable;
an explicit zero mark is a different state.

## Explorer

The [explorer server](../services/platform/perps-data/internal/explorer/server.go)
serves `/api/chain/blocks`, `/api/chain/transactions`, `/api/chain/health`,
`/api/explorer/health`, `/api/chain/accounts/{address}/history`,
`/api/chain/markets/{symbol}/history` and `/api/chain/tx/{hash}`. These are indexed
history routes on the explorer service; gateway registration is separate.

## Portal

The [portal](../services/platform/perps-data/internal/portal/server.go) renders
`/`, `/markets/{symbol}`, `/accounts/{address}`, `/leaderboard` and `/stats` from
the configured gateway. It does not execute trades.

## Replay

[Replay](../services/platform/perps-data/internal/replay/replay.go) reprocesses a
bounded source range into the existing store. Use `--dry-run` to inspect
per-table counts without writes. Replay remains an operator tool and is not a
public gateway endpoint.

## Loadtest

The [load tool](../services/platform/perps-data/loadtest/load.go) drives the
configured gateway and reports socket behavior. Its results measure the data
service, not consensus finality.

## Operations

[Health handlers](../services/platform/perps-data/internal/httpx/health.go)
serve `/healthz/live` and `/healthz/ready`; readiness returns 503 when a dependency
fails, with a two-second evaluation timeout. HTTP services expose `/metrics`.
Indexer readiness bounds head lag to 30 blocks and oldest open outbox age to
30 seconds. Gateway readiness checks stream lag against ten seconds and gates
new sockets while unready. Preserve Postgres and JetStream state across
restarts. See the [deployment files](../services/platform/perps-data/deploy)
for process wiring and the [client recovery contract](perps-client.md#hooks)
for handling interrupted feeds.
