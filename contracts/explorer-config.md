# Explorer backend configuration contract

The explorer backend is the Blockscout fork under
[services/explorer/backend](../services/explorer/backend/). It indexes the
single Paxeer mainnet-beta BFT network, EVM chain ID **125 (`0x7d`)**. The
kernel is part of the network and proposes blocks; a receipt becomes final
when its block is committed. Explorer indexing can lag consensus.
Sub-chains are tenant domains in the database, not separate node deployments.

The configuration owners are
[config/runtime.exs](../services/explorer/backend/config/runtime.exs),
[config/config_helper.exs](../services/explorer/backend/config/config_helper.exs),
and the environment-specific
[production](../services/explorer/backend/config/runtime/prod.exs) and
[development](../services/explorer/backend/config/runtime/dev.exs) files.
Settings below distinguish runtime defaults from the deployment
[example environment](../services/explorer/deploy/env/backend.example.env).
Do not copy example policy choices as claims about every deployment.

## Network and node transport

| Variable | Contract |
| --- | --- |
| `CHAIN_TYPE` | `paxeer_x` selects the Paxeer feature set through `ConfigHelper.chain_identity` |
| `CHAIN_ID` | set to `125` for mainnet-beta |
| `ETHEREUM_JSONRPC_VARIANT` | `paxeer_x` selects [EthereumJSONRPC.PaxeerX](../services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/paxeer_x.ex) |
| `ETHEREUM_JSONRPC_TRANSPORT` | `http` (default) or `ipc`; other values raise at startup |
| `ETHEREUM_JSONRPC_HTTP_URL`, `ETHEREUM_JSONRPC_HTTP_URLS` | HTTP(S) node endpoint or comma-separated pool |
| `ETHEREUM_JSONRPC_TRACE_URL`, `ETHEREUM_JSONRPC_TRACE_URLS` | HTTP(S) trace endpoints with the required retained state |
| `ETHEREUM_JSONRPC_ETH_CALL_URL`, `ETHEREUM_JSONRPC_ETH_CALL_URLS` | HTTP(S) endpoints for contract reads |
| `ETHEREUM_JSONRPC_WS_URL` | WS(S) notification endpoint; it does not replace the HTTP request transport |
| `PAXEER_X_COSMOS_TRANSACTION_TYPE` | synthetic transaction type, default 127 |
| `PAXEER_X_NATIVE_COIN_DECIMALS` | default 18 |
| `NETWORK`, `SUBNETWORK`, `COIN`, `COIN_NAME` | display labels; set Paxeer network labels and PAX coin labels explicitly |

Use the [public node root URLs](evm-rpc.md#http-endpoint), for example
`https://api1.mainnet-beta.paxeer.network/`, for JSON-RPC.
The numbered hosts are `api1` through `api14`; the unnumbered host is
`api-mainnet-beta.paxeer.network`.
`node.hyperpaxeer.com` is only the library mirror, not an RPC endpoint.
The pre-merge router and its /rpc, /rpc/ws and /rpc/schema paths are retired.
There is no separate LayerX interface or separate sequencer service to supply.

HTTP request URLs must use `http` or `https`; WebSocket URLs must use `ws` or
`wss`. `ConfigHelper.parse_urls_list` rejects WebSocket schemes in HTTP pools
and supports the corresponding fallback pools. A nonempty plural URL list
takes precedence over its singular variable. If neither is supplied, trace
and call pools fall back to HTTP; fallback pools also resolve through the
helper. `IPC_PATH` is used with IPC transport. Do not put a WSS endpoint in an
HTTP URL variable.
The Paxeer adapter refetches notified blocks by hash and delegates tracing to
the Geth variant; its implementation is the authority on transaction shapes.

## Database and web service

Production config reads `DATABASE_URL` for `Explorer.Repo`; replica, account
and auxiliary repos use
[Explorer.Repo.ConfigHelper](../services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex).
`POOL_SIZE` and `POOL_SIZE_API` default to 50 in production runtime config;
the local deployment example chooses 40 and 20. `ECTO_USE_SSL` selects SSL
through that helper. Tenant data must retain its domain scope; changing the
display `SUBNETWORK` label is not tenant provisioning.

`PORT`, `BLOCKSCOUT_HOST`, `BLOCKSCOUT_PROTOCOL`, `SECRET_KEY_BASE`, and
`CHECK_ORIGIN` configure Phoenix. Supply a real signing secret through the
deployment environment. `API_V2_ENABLED` defaults to `true` in runtime config.
The local stack sets `DISABLE_WEBAPP=true` to serve the separate
[Next.js frontend](explorer-frontend-config.md), with API v2 retained.
Account, API authentication and rate-limit settings have their own readers;
enabling a public feature in the browser does not authorize backend access.

## Request capacity and indexer

| Variable | Runtime default / behavior |
| --- | --- |
| `ETHEREUM_JSONRPC_THROTTLE_MAX_INFLIGHT` | 32 requests per endpoint |
| `ETHEREUM_JSONRPC_THROTTLE_RPS` | 200 |
| `ETHEREUM_JSONRPC_THROTTLE_TRACE_MAX_INFLIGHT` | 4 |
| `ETHEREUM_JSONRPC_THROTTLE_ARCHIVE_MAX_INFLIGHT` | 8 |
| `ETHEREUM_JSONRPC_THROTTLE_COALESCE_ENABLED` | `true` |
| `ETHEREUM_JSONRPC_WAIT_PER_TIMEOUT` | `20s` request-coordinator backoff interval |
| `ETHEREUM_JSONRPC_DEBUG_TRACE_TRANSACTION_TIMEOUT` | `5s` |
| `ETHEREUM_JSONRPC_ARCHIVE_BALANCES_WINDOW` | 200 blocks |
| `ETHEREUM_JSONRPC_RECEIPTS_BY_BLOCK` | `false` |
| `ETHEREUM_JSONRPC_MAX_RECEIPTS_BY_BLOCK` | 1,000 |

`DISABLE_INDEXER`, `DISABLE_REALTIME_INDEXER`, `DISABLE_CATCHUP_INDEXER`, and
the individual `INDEXER_DISABLE_*` switches control which workers run.
The local deployment example disables internal transaction, block reward,
archival token balance and withdrawal fetchers. It chooses catchup batches of
100 with concurrency 20, receipt batches of 250 with concurrency 10, and coin
balance batches of 500 with concurrency 4. Those are example workload choices;
the node's retained state and query limits still apply. Trace or archive URL
configuration does not restore state the node has pruned.

## Paxeer capabilities

`PAXEER_X_CAPABILITIES_ENABLED` defaults to false through the boolean helper;
`PAXEER_X_CAPABILITIES_REFRESH_SECONDS` defaults to 30. The discovery process
in [capabilities.ex](../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/capabilities.ex)
probes one head height, then checks bridge and exchange views, launchpad code,
and the addr precompile's unified-account response. It publishes absent flags
until a successful probe and retains a prior snapshot on a failed refresh.
The [API controller](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/capabilities_controller.ex)
exposes the capability contract to the browser. Address constants and ABI
files alone do not establish availability; see
[activation and retirement](address-inventory.md#activation).

Legacy `PAXEER_X_ROUTER_URL` is still read into the compatibility receipt
client's `router_url` configuration in `runtime.exs`; that pre-merge router
reference is retired and is not a merged-network deployment requirement.
Likewise, registry references to removed kernel receipt import workers are
historical references, not extra workers to start. The source index below
marks absent declaration files rather than supplying a replacement service.

## Native price-feed indexing

Native price submissions use Cosmos REST and CometBFT block results in
addition to EVM reads. These are node protocol transports, not another
execution or settlement network.

| Variable | Runtime contract |
| --- | --- |
| `PAXEER_X_PRICE_FEED_ENABLED` | default false; controls the price update supervisor and maintenance |
| `PAXEER_X_COSMOS_REST_URLS` | required nonempty comma list of HTTP(S) base URLs when enabled |
| `PAXEER_X_COMETBFT_RPC_URLS` | required nonempty comma list of HTTP(S) base URLs when enabled |
| `PAXEER_X_PRICE_FEED_START_BLOCK` | required nonnegative integer when enabled, within available node history |
| `PAXEER_X_PRICE_FEED_CONCURRENCY` | default 4 |
| `PAXEER_X_PRICE_FEED_MARKETS_REFRESH_SECONDS` | default 300 |
| `PAXEER_X_PRICE_FEED_PRECOMPILE` | default `0x0000000000000000000000000000000000001020`, normalized to lowercase |
| `PAXEER_X_PRICE_FEED_RETENTION_DAYS` | if set, must be positive; unset retains history; deployment example chooses 30 days |

The runtime raises on missing required URLs or start block when the feed is
enabled, and on an invalid configured retention period. The frontend feature
switch does not turn these indexers on.

## Optional services and configuration families

`MICROSERVICE_*` settings enable independent explorer helpers, such as the
smart-contract verifier, signature provider and metadata service.
`MICROSERVICE_METADATA_ENABLED=true` requires a valid HTTP(S)
`MICROSERVICE_METADATA_URL` base URL without the appended API suffix.
The metadata request timeout defaults and deployment choices belong to the
runtime reader, not to the EVM node. Keep credentials on the backend;
`NEXT_PUBLIC_*` values are visible to browser clients.

Other Blockscout configuration families remain available for their respective
features and chain variants. An `env-*` or `cfg-*` index target below links to
the owning reader; it does not mean that every optional upstream integration
is enabled for Paxeer. Boolean parsing in `ConfigHelper` compares the
lowercased string with `true`; integer, duration, URL and JSON settings use
their dedicated parsers.

For bridge assets use [deployment records](bridge-deploy.md#records) and
[vector identities](bridge-vectors.md), rather than inventing an explorer-only
address. The browser-facing variables are documented in
[explorer-frontend-config.md](explorer-frontend-config.md).

## Registry source index

These link targets preserve interface-registry references. Each row points
to the current source owner for its declaration, ABI, message, configuration
family or helper. It does not assert that a method is registered, a feature
is enabled, or a contract is deployed. Rows marked retired identify source
references absent from this tree and must not be used as deployment targets.

| Link targets | Source owner |
| --- | --- |
| <a id="cfg-bcrypt-elixir"></a>`cfg-bcrypt-elixir` | [services/explorer/backend/apps/explorer/config/test.exs](../services/explorer/backend/apps/explorer/config/test.exs) |
| <a id="cfg-ethereum-jsonrpc"></a>`cfg-ethereum-jsonrpc` | [services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs](../services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="cfg-ex-aws"></a>`cfg-ex-aws`, <a id="cfg-libcluster"></a>`cfg-libcluster`, <a id="cfg-nft-media-handler"></a>`cfg-nft-media-handler`, <a id="cfg-ueberauth"></a>`cfg-ueberauth` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="cfg-ex-cldr"></a>`cfg-ex-cldr`, <a id="cfg-oauth2"></a>`cfg-oauth2`, <a id="cfg-prometheus"></a>`cfg-prometheus`, <a id="cfg-spandex-phoenix"></a>`cfg-spandex-phoenix` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs) |
| <a id="cfg-explorer"></a>`cfg-explorer` | [services/explorer/backend/apps/explorer/config/config.exs](../services/explorer/backend/apps/explorer/config/config.exs); [services/explorer/backend/apps/explorer/config/dev/anvil.exs](../services/explorer/backend/apps/explorer/config/dev/anvil.exs); [services/explorer/backend/apps/explorer/config/runtime/test.exs](../services/explorer/backend/apps/explorer/config/runtime/test.exs); [services/explorer/backend/apps/explorer/config/test.exs](../services/explorer/backend/apps/explorer/config/test.exs); [services/explorer/backend/apps/explorer/config/test/besu.exs](../services/explorer/backend/apps/explorer/config/test/besu.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs); [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="cfg-indexer"></a>`cfg-indexer` | [services/explorer/backend/apps/indexer/config/config.exs](../services/explorer/backend/apps/indexer/config/config.exs); [services/explorer/backend/apps/indexer/config/dev/anvil.exs](../services/explorer/backend/apps/indexer/config/dev/anvil.exs); [services/explorer/backend/apps/indexer/config/dev/rsk.exs](../services/explorer/backend/apps/indexer/config/dev/rsk.exs); [services/explorer/backend/apps/indexer/config/runtime/test.exs](../services/explorer/backend/apps/indexer/config/runtime/test.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="cfg-logger"></a>`cfg-logger` | [services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs](../services/explorer/backend/apps/ethereum_jsonrpc/config/config.exs); [services/explorer/backend/apps/explorer/config/config.exs](../services/explorer/backend/apps/explorer/config/config.exs); [services/explorer/backend/apps/explorer/config/dev.exs](../services/explorer/backend/apps/explorer/config/dev.exs); [services/explorer/backend/apps/indexer/config/config.exs](../services/explorer/backend/apps/indexer/config/config.exs); [services/explorer/backend/apps/indexer/config/dev.exs](../services/explorer/backend/apps/indexer/config/dev.exs); [services/explorer/backend/config/config.exs](../services/explorer/backend/config/config.exs); [services/explorer/backend/config/dev.exs](../services/explorer/backend/config/dev.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="cfg-os-mon"></a>`cfg-os-mon` | [services/explorer/backend/apps/indexer/config/config.exs](../services/explorer/backend/apps/indexer/config/config.exs) |
| <a id="cfg-phoenix"></a>`cfg-phoenix` | [services/explorer/backend/apps/block_scout_web/config/dev.exs](../services/explorer/backend/apps/block_scout_web/config/dev.exs); [services/explorer/backend/config/config.exs](../services/explorer/backend/config/config.exs) |
| <a id="cfg-spandex-ecto"></a>`cfg-spandex-ecto` | [services/explorer/backend/apps/explorer/config/config.exs](../services/explorer/backend/apps/explorer/config/config.exs) |
| <a id="cfg-tesla"></a>`cfg-tesla` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs); [services/explorer/backend/config/config.exs](../services/explorer/backend/config/config.exs) |
| <a id="cfg-wallaby"></a>`cfg-wallaby` | [services/explorer/backend/apps/block_scout_web/config/test.exs](../services/explorer/backend/apps/block_scout_web/config/test.exs) |
| <a id="cfg-web"></a>`cfg-web` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs); [services/explorer/backend/apps/block_scout_web/config/test.exs](../services/explorer/backend/apps/block_scout_web/config/test.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-account"></a>`env-account` | [services/explorer/backend/apps/explorer/lib/explorer/application.ex](../services/explorer/backend/apps/explorer/lib/explorer/application.ex); [services/explorer/backend/apps/explorer/lib/explorer/vault.ex](../services/explorer/backend/apps/explorer/lib/explorer/vault.ex); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs); [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-addresses"></a>`env-addresses` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-admin"></a>`env-admin` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs) |
| <a id="env-amb"></a>`env-amb` | [services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex](../services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex) |
| <a id="env-amplitude"></a>`env-amplitude` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs) |
| <a id="env-api"></a>`env-api` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-application"></a>`env-application` | [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs) |
| <a id="env-apps"></a>`env-apps` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-beryx"></a>`env-beryx` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-blackfort"></a>`env-blackfort` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-block"></a>`env-block` | [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-blockscout"></a>`env-blockscout` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-bridged"></a>`env-bridged` | [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-cache"></a>`env-cache` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-celo"></a>`env-celo` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-certfile"></a>`env-certfile` | [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-chain"></a>`env-chain` | [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-check"></a>`env-check` | [services/explorer/backend/config/runtime/prod.exs](../services/explorer/backend/config/runtime/prod.exs) |
| <a id="env-checksum"></a>`env-checksum` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-coin"></a>`env-coin` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-contract"></a>`env-contract` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-csv"></a>`env-csv` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-custom"></a>`env-custom` | [services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex](../services/explorer/backend/apps/explorer/lib/explorer/tags/address_tag_cataloger.ex); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-database"></a>`env-database` | [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-datadog"></a>`env-datadog` | [services/explorer/backend/apps/explorer/lib/explorer/application.ex](../services/explorer/backend/apps/explorer/lib/explorer/application.ex); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-decode"></a>`env-decode` | [services/explorer/backend/apps/explorer/config/config.exs](../services/explorer/backend/apps/explorer/config/config.exs) |
| <a id="env-disable"></a>`env-disable` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs); [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-display"></a>`env-display` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-ecto"></a>`env-ecto` | [services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex](../services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex) |
| <a id="env-eip"></a>`env-eip` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-emission"></a>`env-emission` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-eth"></a>`env-eth` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-ethereum"></a>`env-ethereum` | [services/explorer/backend/apps/explorer/config/dev/anvil.exs](../services/explorer/backend/apps/explorer/config/dev/anvil.exs); [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-exchange"></a>`env-exchange` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-external"></a>`env-external` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-fetch"></a>`env-fetch` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-filecoin"></a>`env-filecoin` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-filfox"></a>`env-filfox` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-first"></a>`env-first` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-footer"></a>`env-footer` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-gas"></a>`env-gas` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-hackney"></a>`env-hackney` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-health"></a>`env-health` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-hide"></a>`env-hide` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-indexer"></a>`env-indexer` | [services/explorer/backend/apps/indexer/config/dev/anvil.exs](../services/explorer/backend/apps/indexer/config/dev/anvil.exs); Retired reference: `services/explorer/backend/apps/indexer/lib/indexer/fetcher/paxeer_x_kernel_receipts.ex` is absent; [services/explorer/backend/config/config_helper.exs](../services/explorer/backend/config/config_helper.exs); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-ipc"></a>`env-ipc` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-ipfs"></a>`env-ipfs` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-json"></a>`env-json` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-k8s"></a>`env-k8s` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-keyfile"></a>`env-keyfile` | [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-keys"></a>`env-keys` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-last"></a>`env-last` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-logo"></a>`env-logo` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-market"></a>`env-market` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-metadata"></a>`env-metadata` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-microservice"></a>`env-microservice` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-migration"></a>`env-migration` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-min"></a>`env-min` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-missing"></a>`env-missing` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-mixpanel"></a>`env-mixpanel` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs) |
| <a id="env-mud"></a>`env-mud` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs); [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-network"></a>`env-network` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-new"></a>`env-new` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-nft"></a>`env-nft` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-noves"></a>`env-noves` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-paxeer"></a>`env-paxeer` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-permanent"></a>`env-permanent` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-pool"></a>`env-pool` | [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-port"></a>`env-port` | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex) |
| <a id="env-precompiled"></a>`env-precompiled` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-public"></a>`env-public` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-rate"></a>`env-rate` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-re"></a>`env-re` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-release"></a>`env-release` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-replica"></a>`env-replica` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-rewards"></a>`env-rewards` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-rootstock"></a>`env-rootstock` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-scroll"></a>`env-scroll` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-secret"></a>`env-secret` | [services/explorer/backend/config/runtime/dev.exs](../services/explorer/backend/config/runtime/dev.exs) |
| <a id="env-session"></a>`env-session` | [services/explorer/backend/apps/block_scout_web/config/config.exs](../services/explorer/backend/apps/block_scout_web/config/config.exs) |
| <a id="env-show"></a>`env-show` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-shrink"></a>`env-shrink` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-solidityscan"></a>`env-solidityscan` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-sourcify"></a>`env-sourcify` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-spandex"></a>`env-spandex` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-suave"></a>`env-suave` | [services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex](../services/explorer/backend/apps/explorer/lib/explorer/repo/config_helper.ex); [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-subnetwork"></a>`env-subnetwork` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-supply"></a>`env-supply` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-supported"></a>`env-supported` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-tenderly"></a>`env-tenderly` | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/transaction_view.ex](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/transaction_view.ex) |
| <a id="env-test"></a>`env-test` | [services/explorer/backend/apps/explorer/config/test.exs](../services/explorer/backend/apps/explorer/config/test.exs) |
| <a id="env-testnet"></a>`env-testnet` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-tests"></a>`env-tests` | [tests/explorer/paxeer-x-data-boundary.py](../tests/explorer/paxeer-x-data-boundary.py) |
| <a id="env-token"></a>`env-token` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-trace"></a>`env-trace` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-txs"></a>`env-txs` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-uncles"></a>`env-uncles` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-universal"></a>`env-universal` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-validators"></a>`env-validators` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-webapp"></a>`env-webapp` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-weth"></a>`env-weth` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-whitelisted"></a>`env-whitelisted` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-withdrawals"></a>`env-withdrawals` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
| <a id="env-xname"></a>`env-xname` | [services/explorer/backend/config/runtime.exs](../services/explorer/backend/config/runtime.exs) |
