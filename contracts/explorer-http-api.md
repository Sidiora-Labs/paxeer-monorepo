# Explorer HTTP API

The explorer indexes and presents Paxeer mainnet-beta, one BFT network with EVM chain ID **125** (`0x7d`). The kernel is part of the network and proposes blocks; execution is final when its block is committed. Sub-chains are tenant domains in the database. An explorer index, cached response, or pending submission is not an independent source of finality.

Public node JSON-RPC is served at the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, and `api-mainnet-beta.paxeer.network`. The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test`, and `txpool`. Hosted `px_` methods are declared separately in the [gateway OpenRPC contract](../services/platform/hosted/gateway/openrpc.json). `node.hyperpaxeer.com` is only the build library mirror, not an RPC endpoint.

This page describes the explorer's Phoenix HTTP application. Related contracts: [WebSocket topics and recovery](explorer-ws.md), [metrics](explorer-metrics.md), [Rust service configuration](explorer-services-config.md), [kernel archive formats](kernel.md), and [platform CLI](platform-cli.md).

## API mount and request handling

[BlockScoutWeb.Router](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) mounts [ApiRouter](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) at `/api`. Paths below include that prefix. Requests pass through the rate-limit plug. Limits and API-key handling are deployment configuration, not a promise of unlimited anonymous access.

V2 JSON routes pass `CheckApiV2`, fetch the session, and apply forgery protection. If V2 is disabled the response is HTTP 404 with `{"message":"API V2 is disabled"}`. Import routes use a separate pipeline without session/forgery protection; their controllers apply their own authorization checks. Query strings are limited to 5,000 bytes in these pipelines. Do not infer identical body limits or authentication from the shared prefix.

## API V2 OpenAPI

`GET /api/v2/openapi` renders `BlockScoutWeb.Specs.Public`. Controller operation declarations and response views are the source for individual fields. Many lists return `{"items":[...],"next_page_params":{...}}`; a null `next_page_params` ends pagination. Send the returned cursor parameters on the next request, retaining the original filters. Cursor keys differ by resource; do not substitute a numeric page counter.

## API V2 transactions

`GET /api/v2/transactions` lists transactions. Other collection reads are `/watchlist` and `/stats`. Reads under `/api/v2/transactions/:transaction_hash_param` include the detail itself, `/token-transfers`, `/internal-transactions`, `/logs`, `/raw-trace`, `/state-changes`, `/summary`, `/status`, and `/signature`.

For an indexed transaction with successfully fetched traces, the HTTP raw-trace action renders the trace array directly, using the renderer's `raw_traces` assign. It does not return a `raw_trace` wrapper. The action's pending-transaction branch instead passes `internal_transactions: []`, which does not match that renderer clause; do not rely on that branch to produce an empty array. Fetch failures return HTTP 500. The [socket contract](explorer-ws.md#pushes) describes the current push mismatch and why clients need the HTTP read after indexing.

### Transaction status and signature

[TransactionStatusController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_status_controller.ex) returns three fields:

```json
{"rung":"committed","block_number":123,"committed_height":123}
```

`rung` is `submitted`, `receipted`, or `committed`; the two heights can be null. An indexed consensus transaction is committed when its block number is covered by the committed height. Missing block/head evidence leaves it submitted. A verified signed receipt can establish receipted status, while verified landing evidence can establish commitment before indexing catches up. A signed pre-commit receipt alone is not final execution. See [Status.of](../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/status.ex).

For an unknown hash, the handler attempts receipt verification through its configured provider; without verified evidence it returns 404. The optional provider configuration still uses the internal key `router_url` under `Explorer.Chain.PaxeerX.SequencerReceipt`; that name is a legacy implementation detail, not a deployed routing tier. The handler calls `pax_sequencerReceipt` and uses receipt keys; landing verification additionally requests the transaction receipt, consensus commit/validators, and `px_getSequenceBatch`. A public node's root URL alone does not guarantee all these provider methods.

[TransactionSignatureController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller.ex) serves `/signature` only after receipt verification. It adds `committed_height` when verified as committed. Unconfigured, unavailable, malformed, or unverified receipt material follows the not-found path; this endpoint does not manufacture a signature.

## API V2 blocks

`GET /api/v2/blocks` lists blocks; `GET /api/v2/blocks/:block_hash_or_number_param` reads one. Subresources include `/transactions`, `/internal-transactions`, and `/withdrawals`. `GET /api/v2/blocks/:block_number_param/countdown` is a separate estimate, subject to average-block-time availability. Indexing progress is not a consensus commit certificate.

## API V2 addresses

`GET /api/v2/addresses` lists addresses. Under `/api/v2/addresses/:address_hash_param`, reads include the detail, `/tabs-counters`, `/counters`, `/token-balances`, `/tokens`, `/transactions`, `/token-transfers`, `/internal-transactions`, `/logs`, `/blocks-validated`, `/coin-balance-history`, `/coin-balance-history-by-day`, `/withdrawals`, `/nft`, `/nft/collections`, and `/unified`. Selected histories also expose `/csv` exports. The `/unified` action is the native account projection, distinct from explorer user accounts.

## API V2 Paxeer X

These are native-network and historical-data reads under `/api/v2/paxeer-x`:

| GET suffix | Meaning |
| --- | --- |
| `/capabilities` | Latest capability probes and price-feed availability |
| `/archive/anchors` | Frozen anchor checkpoints, paginated by `batch_number` |
| `/archive/receipts` | Frozen public receipts, paginated by `id` |
| `/archive/receipts/:id` | One public archived receipt by full hash |
| `/feed/markets` | Markets with latest observations and `chain_head` |
| `/feed/markets/:market_id/updates` | One market's accepted observation history |
| `/feed/updates` | Native price submissions, newest first |
| `/feed/updates/:hash` | One native submission with observations |

[ArchiveController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex) reads frozen history; these routes do not describe another live settlement network. Unknown Paxeer X paths return not found.

[CapabilitiesController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/capabilities_controller.ex) publishes `addr`, `exchange`, `bridge`, `launchpad`, and `checked_at`. Unanswered probes read false. `price_feed` is present and true only when the local indexer enables the fetcher, or, with indexing disabled, the database has a recently scanned price-feed block within 1,000 blocks of the current height. Absence of a flag must not be interpreted as availability.

[FeedController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/feed_controller.ex) reads native `MsgSubmitPrices` submissions from consensus blocks. These are not EVM transactions. `/feed/updates` accepts optional `feeder` and nonnegative `market_id`, with `block_number`, `index`, and `items_count` cursors. [FeedView](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/api/v2/paxeer_x/feed_view.ex) renders submissions as transaction-list items with `type: 127`, `method: "submitPrices"`, `transaction_types: ["price_update"]`, null recipient, zero value/fee, and observations under `price_update`. Price integers are strings; preserve their precision.

## API V2 main page

`GET /api/v2/main-page/blocks`, `/transactions`, `/transactions/watchlist`, and `/indexing-status` provide initial dashboard snapshots. Live updates use the [socket topics](explorer-ws.md#topics); count notifications require a refetch rather than insertion as complete transaction objects.

## API V2 search

`GET /api/v2/search`, `/api/v2/search/check-redirect`, and `/api/v2/search/quick` serve search and redirect hints. Search results describe indexed resources, not receipt verification.

## API V2 config

Under `/api/v2/config`, GET routes are `/backend`, `/backend-version`, `/csv-export`, `/indexer`, `/public-metrics`, `/smart-contracts/languages`, and `/db-background-migrations`. The public-metrics configuration read is separate from the [Prometheus exporter](explorer-metrics.md#exporters).

## API V2 stats

`GET /api/v2/stats`, `/stats/hot-smart-contracts`, and `/stats/charts/{transactions,market,secondary-coin-market}` read explorer statistics. They can lag chain state.

## API V2 tokens

[TokensApiV2Router](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) owns `/api/v2/tokens`, including token detail, holders, transfers, counters, and NFT instances. `/api/v2/token-transfers` and `/api/v2/internal-transactions` are independent collections. Token-specific paging and validation are defined by their controllers.

## API V2 smart contracts

[SmartContractsApiV2Router](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) owns `/api/v2/smart-contracts`, including verification requests and compiler metadata. Source verification proves a bytecode/source relationship; it does not establish chain commitment. Rust verification-service settings are in [service configuration](explorer-services-config.md#cfg-verifier).

## API V2 import

`/api/v2/import/token-info` supports POST and DELETE. `GET /api/v2/import/smart-contracts/:address_hash_param` searches contract information; POST to its `/audit-reports` subresource imports a report. These use the import pipeline and controller authorization, not the ordinary browser session pipeline.

## API V2 proxy

`/api/v2/proxy` contains configured third-party, metadata, and account-abstraction integrations. [ApiRouter](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) declares their exact paths. An integration's route existing in source does not prove its upstream is configured.

## Chain-specific routes

The shared explorer code also contains Optimism, Arbitrum, Scroll, Polygon zkEVM, zkSync, Ethereum beacon/blob, Celo, and other chain-specific routes. They are guarded by compile-time chain type/identity or `chain_scope` conditions. They are not additional Paxeer execution or settlement layers. MUD routes require their compile-time and runtime feature switches. Inspect the deployed chain configuration before using any such route.

## RPC-style query API

The Etherscan-compatible query API accepts `module` and `action` at `/api` and `/api/v1` when V1 reads are enabled. For example: `GET /api?module=account&action=balance&address=<address>`. This database API differs from node JSON-RPC. [RPCTranslator](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex) validates the module/action, path, and write-access configuration.

### RPC module account

`listaccounts`, `balance`, `balancemulti`, `pendingtxlist`, `txlist`, `txlistinternal`, `tokentx`, `tokennfttx`, `tokenbalance`, `tokenlist`, and `getminedblocks` are declared by AddressController. Required identifiers and paging/filter parameters vary by action.

### RPC module block

`getblockreward`, `getblockcountdown`, and `getblocknobytime` are block-query actions. Countdown depends on average-block-time calculation.

### RPC module contract

Read actions include `getabi`, `getsourcecode`, `getcontractcreation`, and `listcontracts`. Verification actions include `verify`, `verify_via_sourcify`, `verifysourcecode`, `checkverifystatus`, `verifyproxycontract`, `checkproxyverification`, and `verify_vyper_contract`; actual availability follows the translator and writing configuration.

### RPC module logs

`getlogs` validates address/topic and block-range parameters before querying indexed logs.

### RPC module stats

`tokensupply`, `ethsupplyexchange`, `ethsupply`, `coinsupply`, `ethprice`, `coinprice`, and `totalfees` retain compatibility names; `eth` in an action name does not change Paxeer's chain identity.

### RPC module token

`gettoken` and `gettokenholders` read indexed token data.

### RPC module transaction

`gettxinfo`, `gettxreceiptstatus`, and `getstatus` read indexed transaction results. A successful execution result and a verified commit are different facts.

### RPC module epoch

The epoch module is conditionally registered for the Optimism/Celo identity, not the Paxeer mainnet-beta configuration.

## API V1 GraphQL

`/api/v1/graphql` forwards to Absinthe when GraphQL is enabled; complexity and token limits apply. The optional `/graphiql` browser interface additionally requires V1 reads. GraphQL subscriptions use the legacy explorer [socket](explorer-ws.md#graphql-subscription).

## Account API V2

[AccountRouter](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) mounts under `/api/account`. Its `/v2` routes include authentication, CSRF acquisition, user information, watchlists, API keys, tags, custom ABIs, and address/email linking. Their private OpenAPI schema is separate from the public schema. Session, account-feature, and forgery checks depend on the declared pipeline; API keys for rate limiting are not a replacement for the user's session.

## Auth path

The top-level `/auth/*path` delegates to AccountRouter. The account router declares provider requests/callbacks, `/auth/profile`, and `/auth/logout`; use its route declarations for the mounted authentication flow. Browser account management is under `/api/account`, not a native on-chain identity API.

## API health

`GET /api/health` checks indexed block/cache health and returns HTTP 200 or 500 with `healthy` and metadata. Standalone media-worker mode returns a simple healthy response. `/api/health/multichain-search-export` reports migration progress separately.

### API health liveness

`GET /api/health/liveness` returns 200 `OK` for process liveness.

### API health readiness

`GET /api/health/readiness` calls the database block-status helper outside standalone media-worker mode, then returns 200 `OK`. It is not a guarantee that indexing has reached the chain head. Implementation: [HealthController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/health_controller.ex).

## Errors

[FallbackController](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/fallback_controller.ex) renders V2 errors as message objects, not hosted-gateway JSON-RPC errors:

| HTTP | Representative message |
| --- | --- |
| 400 | `Unknown API v2 action` |
| 401 | `Unauthorized`, `Wrong API key` |
| 403 | `Restricted access`, `API endpoint is disabled`, `API key not configured on the server` |
| 404 | `Not found`, `Address not found`, `Block lost consensus` |
| 422 | `Invalid parameter(s)`, `Invalid hash`, `Invalid address hash`, `Invalid number` |
| 501 | `Service is disabled` |

OpenAPI casting can produce structured validation errors. Some compatibility actions return a message with HTTP 200, so inspect the response body. The query translator returns 400 for missing module/action or unknown module/action, 404 for an invalid V1 request path, and 500 for controller dispatch errors. Endpoint-specific validation and rate limiting can return additional responses; consult the owning controller and plug rather than treating this table as exhaustive.
