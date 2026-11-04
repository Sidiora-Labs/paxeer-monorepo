# Chainflow Data Gateway Python API

Operation reference for the synchronous client. `AsyncChainflowDataGateway`
exposes the same resources: await REST calls and use `async with` for WebSocket
connections. See [the skill guide](./SKILL.md) for a complete connection example,
[the README](./README.md) for client options, and the [Rust SDK](../rust/README.md)
for the corresponding asynchronous Rust client.

The default gateway is `https://data.feed.chainflowtrading.com`. This SDK serves
market data and explorer observations. For Paxeer node JSON-RPC, use the public
node root path, such as `https://api1.mainnet-beta.paxeer.network/`, with chain ID
125 (`0x7d`); see [public RPC](../../../docs/site/docs/reference/public-rpc.md).
Paxeer is one BFT network whose kernel proposes blocks; committed blocks make
receipts final. Sub-chains are tenant domains in the database.

## Contents

- [`Gateway`](#gateway)
  - [Check gateway liveness](#check-gateway-liveness)
  - [Mint a scoped WebSocket token](#mint-a-scoped-websocket-token)
- [`PriceFeeds`](#pricefeeds)
  - [Get the current feed price](#get-the-current-feed-price)
  - [Get feed health](#get-feed-health)
  - [Get spot-style market statistics](#get-spot-style-market-statistics)
  - [Get perpetual-market statistics](#get-perpetual-market-statistics)
  - [Get trade-tape storage status](#get-trade-tape-storage-status)
  - [Get recent spot trades](#get-recent-spot-trades)
  - [Get recent perpetual trades](#get-recent-perpetual-trades)
- [`TradingView`](#tradingview)
  - [Get UDF feed capabilities](#get-udf-feed-capabilities)
  - [Get server Unix time](#get-server-unix-time)
  - [Resolve UDF symbol metadata](#resolve-udf-symbol-metadata)
  - [Search within one feed](#search-within-one-feed)
  - [Get OHLCV history](#get-ohlcv-history)
  - [Get UDF quotes](#get-udf-quotes)
- [`Markets`](#markets)
  - [Get aggregator health](#get-aggregator-health)
  - [List display-scaled market summaries](#list-display-scaled-market-summaries)
  - [Get one display-scaled market summary](#get-one-display-scaled-market-summary)
  - [Get display-scaled aggregate totals](#get-display-scaled-aggregate-totals)
  - [List fresh unscaled tickers](#list-fresh-unscaled-tickers)
  - [Get one fresh unscaled ticker](#get-one-fresh-unscaled-ticker)
  - [Get estimated long/short perpetual open interest](#get-estimated-longshort-perpetual-open-interest)
- [`Explorer`](#explorer)
  - [Get EVM explorer health](#get-evm-explorer-health)
  - [Get recent EVM blocks](#get-recent-evm-blocks)
  - [Get recent explorer activity](#get-recent-explorer-activity)
- [`CoinGecko`](#coingecko)
  - [List local CoinGecko-compatible IDs](#list-local-coingecko-compatible-ids)
  - [Get CoinGecko-compatible market data](#get-coingecko-compatible-market-data)
  - [Get CoinGecko-compatible coin detail](#get-coingecko-compatible-coin-detail)
  - [Get compact USD quotes](#get-compact-usd-quotes)
- [`CoinMarketCap`](#coinmarketcap)
  - [Get CMC-compatible latest quotes](#get-cmc-compatible-latest-quotes)
  - [Get CMC-compatible latest listings](#get-cmc-compatible-latest-listings)
  - [Get CMC-compatible latest quotes](#get-cmc-compatible-latest-quotes-1)
  - [Get CMC-compatible latest listings](#get-cmc-compatible-latest-listings-1)
- [`WebSockets`](#websockets)
  - [Open {market} WebSocket](#open-market-websocket)
  - [Open markets WebSocket](#open-markets-websocket)
  - [Open explorer WebSocket](#open-explorer-websocket)
  - [Open explorer WebSocket alias](#open-explorer-websocket-alias)

## Setup

Install `chainflow-data-gateway` for REST, or
`python3 -m pip install 'chainflow-data-gateway[websockets]'` for stream support.
The examples below include token minting, so configure both distinct API keys:

```python
import os

from chainflow_data_gateway import ChainflowDataGateway

client = ChainflowDataGateway(
    rest_api_key=os.environ["REST_API_KEY"],
    ws_mint_key=os.environ["WS_MINT_KEY"],
)
```

The constructor also reads `REST_API_KEY`, `WS_MINT_KEY` and `WS_TOKEN` when
options are omitted; `rest_api_key` is required at construction. REST-only callers
can omit `ws_mint_key`. REST data requests send the REST key in `X-API-Key`;
`POST /auth/ws-token` selects only the mint key, and stream upgrades select only
the scoped token in the `token` query parameter. Keep API keys server-side.
Set `base_url` or `SIDIORALABS_BASE_URL` to change the HTTP gateway;
`websocket_base_url` overrides the stream base, otherwise the scheme changes
from HTTPS to WSS (HTTP to WS) on the same host.

Close the client when finished, or use a `with ChainflowDataGateway(...)` block.
HTTP 4xx and 5xx responses raise `APIStatusError` subclasses; inspect
`status_code`, `body` and `response.headers` without logging credentials.
Application errors inside HTTP 200 responses must be checked separately.

## `Gateway`

Gateway health and WebSocket credential issuance.

### Check gateway liveness

`GET /health`.

Returns text from the gateway liveness check. This operation declares no authentication; it checks Nginx only and does not probe the auth helper, database or market services.

| Direction | Type |
| --- | --- |
| Response | `str` |

```python
gateway = client.gateway.list_health()
```

### Mint a scoped WebSocket token

`POST /auth/ws-token`.

The setup client supplies `ws_mint_key` from `WS_MINT_KEY` for this request.
The JSON body contains only `symbol`: a lowercase feed name, `markets` or
`explorer`. The returned `WsToken` exposes `token`, `symbol`, `expires_at` and
`expires_in` (300 seconds); the wire names of the last two are `expiresAt` and
`expiresIn`. Keep the token in memory only and never log it. Credentials are
checked at upgrade; token expiry alone does not close an established socket.
Minting does not update `client.ws_token`: assign it explicitly before connecting.

| Direction | Type |
| --- | --- |
| Request | [`GatewayCreateWebSocketTokenParams`](./src/chainflow_data_gateway/types/gateway_create_web_socket_token_params.py) |
| Response | [`WsToken`](./src/chainflow_data_gateway/types/ws_token.py) |

```python
credential = client.gateway.create_web_socket_token(symbol="btc")
client.ws_token = credential.token
```

## `PriceFeeds`

Per-symbol USD prices, statistics and tapes.

### Get the current feed price

`GET /api/{market}/price`.

HTTP 200 may contain `{"error": "No price data available"}` during cold start. The price timestamp is the candle-bucket start, not the observation time of a trade.

| Direction | Type |
| --- | --- |
| Response | [`PriceFeedListResponse`](./src/chainflow_data_gateway/types/price_feed_list_response.py) |

```python
price_feed = client.price_feeds.list(
    market="btc",
)
```

### Get feed health

`GET /api/{market}/health`.

| Direction | Type |
| --- | --- |
| Response | [`FeedHealth`](./src/chainflow_data_gateway/types/feed_health.py) |

```python
price_feed = client.price_feeds.list_health(
    market="btc",
)
```

### Get spot-style market statistics

`GET /api/{market}/stats`.

| Direction | Type |
| --- | --- |
| Response | [`Stats`](./src/chainflow_data_gateway/types/stats.py) |

```python
price_feed = client.price_feeds.list_spot_style_stats(
    market="btc",
)
```

### Get perpetual-market statistics

`GET /api/{market}/perp_stats`.

| Direction | Type |
| --- | --- |
| Response | [`PerpStats`](./src/chainflow_data_gateway/types/perp_stats.py) |

```python
price_feed = client.price_feeds.list_perpetual_stats(
    market="btc",
)
```

### Get trade-tape storage status

`GET /api/{market}/trades`.

This path reports tape storage status; use `/api/{market}/trades/{symbol}` for trade records.

| Direction | Type |
| --- | --- |
| Response | [`TradeTapeStatus`](./src/chainflow_data_gateway/types/trade_tape_status.py) |

```python
price_feed = client.price_feeds.list_trade_tape_status(
    market="btc",
)
```

### Get recent spot trades

`GET /api/{market}/trades/{symbol}`.

| Direction | Type |
| --- | --- |
| Request | [`PriceFeedRetrieveRecentSpotTradesParams`](./src/chainflow_data_gateway/types/price_feed_retrieve_recent_spot_trades_params.py) |
| Response | [`SpotTrades`](./src/chainflow_data_gateway/types/spot_trades.py) |

```python
price_feed = client.price_feeds.retrieve_recent_spot_trades(
    market="btc",
    symbol="BTC",
    limit=100,
)
```

### Get recent perpetual trades

`GET /api/{market}/perp_trades/{symbol}`.

| Direction | Type |
| --- | --- |
| Request | [`PriceFeedRetrieveRecentPerpetualTradesParams`](./src/chainflow_data_gateway/types/price_feed_retrieve_recent_perpetual_trades_params.py) |
| Response | [`PerpTrades`](./src/chainflow_data_gateway/types/perp_trades.py) |

```python
price_feed = client.price_feeds.retrieve_recent_perpetual_trades(
    market="btc",
    symbol="BTC",
    limit=100,
)
```

## `TradingView`

TradingView UDF configuration, metadata, quotes and candles.

### Get UDF feed capabilities

`GET /api/{market}/config`.

| Direction | Type |
| --- | --- |
| Response | [`UdfConfig`](./src/chainflow_data_gateway/types/udf_config.py) |

```python
trading_view = client.trading_view.list_datafeed_config(
    market="btc",
)
```

### Get server Unix time

`GET /api/{market}/time`.

Unix seconds encoded as text. The client returns `str`; convert with `int(...)` if needed. The feed serves `text/html`, not JSON.

| Direction | Type |
| --- | --- |
| Response | `str` |

```python
trading_view = client.trading_view.list_feed_time(
    market="btc",
)
```

### Resolve UDF symbol metadata

`GET /api/{market}/symbols`.

Unknown or mismatched symbol returns an application error with HTTP 200.

| Direction | Type |
| --- | --- |
| Request | [`TradingViewResolveSymbolParams`](./src/chainflow_data_gateway/types/trading_view_resolve_symbol_params.py) |
| Response | [`TradingViewResolveSymbolResponse`](./src/chainflow_data_gateway/types/trading_view_resolve_symbol_response.py) |

```python
trading_view = client.trading_view.resolve_symbol(
    market="btc",
    symbol="BTC",
)
```

### Search within one feed

`GET /api/{market}/search`.

This is not cross-market search. The endpoint ignores UDF exchange/type filters.

| Direction | Type |
| --- | --- |
| Request | [`TradingViewSearchParams`](./src/chainflow_data_gateway/types/trading_view_search_params.py) |
| Response | [`TradingViewSearchResponse`](./src/chainflow_data_gateway/types/trading_view_search_response.py) |

```python
trading_view = client.trading_view.search(
    market="btc",
    query="",
    limit=30,
)
```

### Get OHLCV history

`GET /api/{market}/history`.

Ascending, inclusive range. `countback` limits rows from the start of that range. Request subsequent windows starting at the last returned timestamp plus one interval; do not assume backward-fill semantics. Open candles can change. UDF unknown-symbol/resolution errors use HTTP 200; malformed parameters use HTTP 400.

| Direction | Type |
| --- | --- |
| Request | [`TradingViewListCandleHistoryParams`](./src/chainflow_data_gateway/types/trading_view_list_candle_history_params.py) |
| Response | [`TradingViewListCandleHistoryResponse`](./src/chainflow_data_gateway/types/trading_view_list_candle_history_response.py) |

```python
trading_view = client.trading_view.list_candle_history(
    market="btc",
    symbol="BTC",
    resolution="1",
    from_=1735689600,
    to=1735693200,
    countback=1000,
)
```

### Get UDF quotes

`GET /api/{market}/quotes`.

Omitting `symbols` returns an empty `d` array. Bid and ask both equal candle close; these are not orderbook top-of-book prices.

| Direction | Type |
| --- | --- |
| Request | [`TradingViewListQuotesParams`](./src/chainflow_data_gateway/types/trading_view_list_quotes_params.py) |
| Response | [`TradingViewListQuotesResponse`](./src/chainflow_data_gateway/types/trading_view_list_quotes_response.py) |

```python
trading_view = client.trading_view.list_quotes(
    market="btc",
)
```

## `Markets`

Cross-market summaries and ticker adapters. Summary and totals responses use
display-scaled values; ticker responses retain raw numeric values. Do not mix
the two representations when calculating prices or totals.

### Get aggregator health

`GET /api/markets/health`.

| Direction | Type |
| --- | --- |
| Response | [`MarketsHealth`](./src/chainflow_data_gateway/types/markets_health.py) |

```python
market = client.markets.list_health()
```

### List display-scaled market summaries

`GET /api/markets/markets`.

| Direction | Type |
| --- | --- |
| Response | [`MarketListResponse`](./src/chainflow_data_gateway/types/market_list_response.py) |

```python
market = client.markets.list()
```

### Get one display-scaled market summary

`GET /api/markets/markets/{symbol}`.

| Direction | Type |
| --- | --- |
| Response | [`MarketRetrieveResponse`](./src/chainflow_data_gateway/types/market_retrieve_response.py) |

```python
market = client.markets.retrieve(
    symbol="BTC",
)
```

### Get display-scaled aggregate totals

`GET /api/markets/totals`.

| Direction | Type |
| --- | --- |
| Response | [`MarketListTotalsResponse`](./src/chainflow_data_gateway/types/market_list_totals_response.py) |

```python
market = client.markets.list_totals()
```

### List fresh unscaled tickers

`GET /api/markets/tickers`.

Omits markets without a positive price ingested during the last 30 seconds; data may be empty or contain fewer than 18 markets.

| Direction | Type |
| --- | --- |
| Response | [`MarketListTickersResponse`](./src/chainflow_data_gateway/types/market_list_tickers_response.py) |

```python
market = client.markets.list_tickers()
```

### Get one fresh unscaled ticker

`GET /api/markets/ticker/{symbol}`.

Unknown, missing or stale prices return HTTP 503, not 404.

| Direction | Type |
| --- | --- |
| Response | [`MarketRetrieveTickerResponse`](./src/chainflow_data_gateway/types/market_retrieve_ticker_response.py) |

```python
market = client.markets.retrieve_ticker(
    symbol="BTC",
)
```

### Get estimated long/short perpetual open interest

`GET /api/markets/open-interest`.

| Direction | Type |
| --- | --- |
| Response | [`MarketListOpenInterestResponse`](./src/chainflow_data_gateway/types/market_list_open_interest_response.py) |

```python
market = client.markets.list_open_interest()
```

## `Explorer`

EVM block and activity observations from the gateway.

### Get EVM explorer health

`GET /api/explorer/health`.

| Direction | Type |
| --- | --- |
| Response | [`ExplorerHealth`](./src/chainflow_data_gateway/types/explorer_health.py) |

```python
explorer = client.explorer.list_health()
```

### Get recent EVM blocks

`GET /api/explorer/blocks`.

In-memory snapshot; may be empty on startup or immediately after reorg/reset. Not an archival chain API.

| Direction | Type |
| --- | --- |
| Request | [`ExplorerListBlocksParams`](./src/chainflow_data_gateway/types/explorer_list_blocks_params.py) |
| Response | [`ExplorerListBlocksResponse`](./src/chainflow_data_gateway/types/explorer_list_blocks_response.py) |

```python
explorer = client.explorer.list_blocks(
    limit=20,
)
```

### Get recent explorer activity

`GET /api/explorer/transactions`.

Explorer activity.

| Direction | Type |
| --- | --- |
| Request | [`ExplorerListTransactionsParams`](./src/chainflow_data_gateway/types/explorer_list_transactions_params.py) |
| Response | [`ExplorerListTransactionsResponse`](./src/chainflow_data_gateway/types/explorer_list_transactions_response.py) |

```python
explorer = client.explorer.list_transactions(
    limit=50,
)
```

## `CoinGecko`

USD-only CoinGecko-compatible local-feed formats.

### List local CoinGecko-compatible IDs

`GET /api/v3/coins/list`.

Unsupported or duplicate query parameters are rejected with HTTP 400. This is a local compatibility adapter, not the official provider.

| Direction | Type |
| --- | --- |
| Request | [`CoinGeckoListParams`](./src/chainflow_data_gateway/types/coin_gecko_list_params.py) |
| Response | [`CoinGeckoListResponse`](./src/chainflow_data_gateway/types/coin_gecko_list_response.py) |

```python
coin_gecko = client.coin_gecko.list(
    include_platform=False,
)
```

### Get CoinGecko-compatible market data

`GET /api/v3/coins/markets`.

Unsupported or duplicate query parameters are rejected with HTTP 400. This is a local compatibility adapter, not the official provider.

| Direction | Type |
| --- | --- |
| Request | [`CoinGeckoListMarketsParams`](./src/chainflow_data_gateway/types/coin_gecko_list_markets_params.py) |
| Response | [`CoinGeckoListMarketsResponse`](./src/chainflow_data_gateway/types/coin_gecko_list_markets_response.py) |

```python
coin_gecko = client.coin_gecko.list_markets(
    vs_currency="usd",
    page=1,
    per_page=100,
    order="market_cap_desc",
    sparkline=False,
    precision="full",
    locale="en",
)
```

### Get CoinGecko-compatible coin detail

`GET /api/v3/coins/{id}`.

Unsupported or duplicate query parameters are rejected with HTTP 400. This is a local compatibility adapter, not the official provider.

| Direction | Type |
| --- | --- |
| Request | [`CoinGeckoRetrieveParams`](./src/chainflow_data_gateway/types/coin_gecko_retrieve_params.py) |
| Response | [`CoinDetail`](./src/chainflow_data_gateway/types/coin_detail.py) |

```python
coin_gecko = client.coin_gecko.retrieve(
    id="bitcoin",
    localization=False,
    tickers=False,
    community_data=False,
    developer_data=False,
    sparkline=False,
    market_data=True,
)
```

### Get compact USD quotes

`GET /api/v3/simple/price`.

Response keys are local IDs unless `symbols` was supplied, in which case keys are lowercase symbols. Omission of both selectors means all markets. Unsupported or duplicate query parameters are rejected with 400. This is a local compatibility adapter, not the official provider.

| Direction | Type |
| --- | --- |
| Request | [`CoinGeckoListSimplePricesParams`](./src/chainflow_data_gateway/types/coin_gecko_list_simple_prices_params.py) |
| Response | [`CoinGeckoListSimplePricesResponse`](./src/chainflow_data_gateway/types/coin_gecko_list_simple_prices_response.py) |

```python
coin_gecko = client.coin_gecko.list_simple_prices(
    vs_currencies="usd",
    include_market_cap=False,
    include_24hr_vol=False,
    include_24hr_change=False,
    include_last_updated_at=False,
)
```

## `CoinMarketCap`

USD-only CMC-compatible local-feed formats.

### Get CMC-compatible latest quotes

`GET /v1/cryptocurrency/quotes/latest`.

Only USD; unknown or duplicate parameters are rejected. Quotes are keyed by
uppercase symbol, while listings use an array of assets. Missing selected prices
fail the whole request. Unsupported metadata is null.

| Direction | Type |
| --- | --- |
| Request | [`CoinMarketCapListQuotesParams`](./src/chainflow_data_gateway/types/coin_market_cap_list_quotes_params.py) |
| Response | [`CoinMarketCapListQuotesResponse`](./src/chainflow_data_gateway/types/coin_market_cap_list_quotes_response.py) |

```python
coin_market_cap = client.coin_market_cap.list_quotes(
    convert="USD",
)
```

### Get CMC-compatible latest listings

`GET /v1/cryptocurrency/listings/latest`.

Only USD; unknown or duplicate parameters are rejected. Quotes are keyed by
uppercase symbol, while listings use an array of assets. Missing selected prices
fail the whole request. Unsupported metadata is null.

| Direction | Type |
| --- | --- |
| Request | [`CoinMarketCapListListingsParams`](./src/chainflow_data_gateway/types/coin_market_cap_list_listings_params.py) |
| Response | [`CoinMarketCapListListingsResponse`](./src/chainflow_data_gateway/types/coin_market_cap_list_listings_response.py) |

```python
coin_market_cap = client.coin_market_cap.list_listings(
    convert="USD",
    start=1,
    limit=100,
    sort="market_cap",
    sort_dir="desc",
)
```

### Get CMC-compatible latest quotes

`GET /api/v1/cryptocurrency/quotes/latest`.

Alias of the same path without `/api`. Only USD; unknown or duplicate parameters
are rejected. Quotes are keyed by uppercase symbol, while listings use an array of assets. Missing selected prices
fail the whole request. Unsupported metadata is null.

| Direction | Type |
| --- | --- |
| Request | [`CoinMarketCapListQuotesAPIAliasParams`](./src/chainflow_data_gateway/types/coin_market_cap_list_quotes_api_alias_params.py) |
| Response | [`CoinMarketCapListQuotesAPIAliasResponse`](./src/chainflow_data_gateway/types/coin_market_cap_list_quotes_api_alias_response.py) |

```python
coin_market_cap = client.coin_market_cap.list_quotes_api_alias(
    convert="USD",
)
```

### Get CMC-compatible latest listings

`GET /api/v1/cryptocurrency/listings/latest`.

Alias of the same path without `/api`. Only USD; unknown or duplicate parameters
are rejected. Quotes are keyed by uppercase symbol, while listings use an array of assets. Missing selected prices
fail the whole request. Unsupported metadata is null.

| Direction | Type |
| --- | --- |
| Request | [`CoinMarketCapListListingsAPIAliasParams`](./src/chainflow_data_gateway/types/coin_market_cap_list_listings_api_alias_params.py) |
| Response | [`CoinMarketCapListListingsAPIAliasResponse`](./src/chainflow_data_gateway/types/coin_market_cap_list_listings_api_alias_response.py) |

```python
coin_market_cap = client.coin_market_cap.list_listings_api_alias(
    convert="USD",
    start=1,
    limit=100,
    sort="market_cap",
    sort_dir="desc",
)
```

## `WebSockets`

These methods implement stream connections through the optional `websockets`
package. They upgrade to a persistent socket; success has no HTTP JSON response
body. `recv()` parses a JSON frame into a Python value. Each example below uses
the setup client with both keys and explicitly installs a token for its stream.
Minting and renewal are caller-managed. Use separate clients for simultaneous
scopes, and mint a fresh token before an upgrade after expiry. Do not log tokens
or connection URLs; WebSocket debug logging includes the token-bearing URL.

### Open {market} WebSocket

`GET /ws/{market}` (WebSocket upgrade). One connection addresses one lowercase
feed name. Mint for that same name, such as `btc`; the corresponding application
symbol is `BTC`. There are no cross-symbol subscriptions on a feed connection.

```python
client.ws_token = client.gateway.create_web_socket_token(symbol="btc").token
with client.web_sockets.connect_price_feed(market="btc") as connection:
    message = connection.recv()
    print(message)
```

### Open markets WebSocket

`GET /ws/markets` (WebSocket upgrade). Mint for `markets`. This aggregate stream
sends initial snapshots and full replacements; its default broadcast cadence is
5 seconds.

```python
client.ws_token = client.gateway.create_web_socket_token(symbol="markets").token
with client.web_sockets.connect_markets() as connection:
    message = connection.recv()
    print(message)
```

### Open explorer WebSocket

`GET /ws/explorer` (WebSocket upgrade). Mint for `explorer`. The stream carries
EVM blocks and a separate activity stream.

```python
client.ws_token = client.gateway.create_web_socket_token(symbol="explorer").token
with client.web_sockets.connect_explorer() as connection:
    message = connection.recv()
    print(message)
```

### Open explorer WebSocket alias

`GET /explorer` (WebSocket upgrade). This aliases `/ws/explorer` and uses the
same `explorer` token scope.

```python
client.ws_token = client.gateway.create_web_socket_token(symbol="explorer").token
with client.web_sockets.connect_explorer_alias() as connection:
    message = connection.recv()
    print(message)
```
