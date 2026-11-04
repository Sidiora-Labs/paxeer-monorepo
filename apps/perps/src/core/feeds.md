# Chart feeds and node configuration

Chart history uses the separate Chainflow UDF service. `NEXT_PUBLIC_MARKET_PROXY_HOST`
accepts a host (HTTPS), or a bare HTTP(S) origin for a separately configured
producer. The default is `https://data.feed.chainflowtrading.com`. Credentials,
paths, queries and fragments are refused. This setting does not change the perps
gateway or node RPC origin.

The validated gateway catalog loaded by `loadPerpsMarkets` replaces the bootstrap
chart symbol set through `registerVenueSymbols`. Chart search and UDF routing then
use that same catalog, including newly listed markets. Route symbols must contain
only ASCII letters and digits; `PERP_<symbol>_USDC` and `<symbol>/USD` chart names
are normalized before lookup. Catalog membership permits a request; the history
producer's `/symbols` metadata and `/history` responses determine availability.
A missing route, UDF refusal or mismatched symbol metadata reaches the chart's
error callback as an explicit unavailable feed. An empty history window still
uses the producer's `no_data` response.

The candle producer must provision `/api/<lowercase-symbol>/config`, `/symbols`
and `/history` when it supports a newly listed market. Catalog listing alone
does not promise a candle producer, or add a dedicated WebSocket upstream.

Node JSON-RPC defaults are the root URLs of `api1` through
`api14.mainnet-beta.paxeer.network`, followed by
`api-mainnet-beta.paxeer.network`, for chain 125. Configuration declares the set;
it makes no claim about live availability. `NEXT_PUBLIC_PAXEER_RPC_URLS` replaces
the set with a comma-separated operator list. `NEXT_PUBLIC_PAXEER_RPC_URL` puts
one operator URL ahead of the defaults. Duplicates, empty entries and legacy
hosts are filtered; a list containing only legacy hosts uses the existing
single-URL/default selection. No live probe is needed to verify this contract.
