# Paxeer X Network Explorer backend

This is the backend of the Paxeer X Network block explorer: a fork of the
Blockscout Elixir umbrella application, imported from `blockscout/blockscout`
at tag `v10.2.6` (see [upstream provenance](../UPSTREAM.md)). It indexes
Paxeer mainnet-beta over node EVM JSON-RPC, stores indexed data in PostgreSQL,
and serves the explorer frontend in [`apps/explorer`](../../../apps/explorer).

Paxeer is one BFT `paxd` network, with EVM chain id **125** (`0x7d`). Its kernel
is part of the network and proposes blocks; a receipt is final once its block
is committed. The explorer's indexed view can lag the node. Public node
JSON-RPC uses root URLs such as `https://api1.mainnet-beta.paxeer.network/`;
the [explorer overview](../README.md#network-and-interfaces) lists the node
names and links to the hosted gateway's `px_` contract. Sub-chains are tenant
domains in the database, launchable without new infrastructure.

## Umbrella applications

| Application | What it does |
| --- | --- |
| [`apps/ethereum_jsonrpc`](apps/ethereum_jsonrpc) | JSON-RPC client and per-node variants, including `paxeer_x` |
| [`apps/explorer`](apps/explorer) | Ecto schemas, database migrations and chain queries |
| [`apps/indexer`](apps/indexer) | Fetches blocks, transactions, receipts and tokens into the database; includes the native price-feed fetcher |
| [`apps/block_scout_web`](apps/block_scout_web) | Phoenix endpoint serving REST and, when enabled, GraphQL APIs |
| [`apps/nft_media_handler`](apps/nft_media_handler) | NFT media fetching, resizing and upload, off unless enabled |
| [`apps/utils`](apps/utils) | Helpers shared by the other applications |

The umbrella's `apps/explorer` is the Elixir database application; the
monorepo's `apps/explorer` is the separately built web frontend.

## Paxeer X additions

The fork uses `CHAIN_TYPE=paxeer_x` and
`ETHEREUM_JSONRPC_VARIANT=paxeer_x`. Its adapter,
[`EthereumJSONRPC.PaxeerX`](apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/paxeer_x.ex),
normalizes Cosmos-originated transaction type `0xffffffff` to the configured
explorer type (`PAXEER_X_COSMOS_TRANSACTION_TYPE`, default `127`). EVM balances
and values remain in 18-decimal units; they are not the six-decimal bank
amounts. `PAXEER_X_NATIVE_COIN_DECIMALS` defaults to `18`.

Paxeer schemas live under `apps/explorer/lib/explorer/chain/paxeer_x/`,
import runners under `apps/explorer/lib/explorer/chain/import/runner/paxeer_x/`,
and migrations under `apps/explorer/priv/repo/migrations/`.

The registered read APIs below are relative to `/api/v2` and use `GET`.
[`api_router.ex`](apps/block_scout_web/lib/block_scout_web/routers/api_router.ex)
and the linked controllers define their contracts. The first table describes
the current native account, transaction and feed APIs.

| Path | Purpose |
| --- | --- |
| `/paxeer-x/capabilities` | Last successful capability probe, plus native feed availability when present |
| `/addresses/:address_hash_param/unified` | Native identities, balances and cursor-paginated activity |
| `/transactions/:transaction_hash_param/status` | Observed transaction status and committed height |
| `/transactions/:transaction_hash_param/signature` | Verified receipt, receipt key and signature when evidence is available |
| `/paxeer-x/feed/markets` | Native price-feed markets and latest observations |
| `/paxeer-x/feed/markets/:market_id/updates` | Paginated observations for one market |
| `/paxeer-x/feed/updates` | Paginated native submissions, optionally filtered by feeder or market |
| `/paxeer-x/feed/updates/:hash` | One native submission |

Historical archive APIs are separate from current transaction status:

| Path | Purpose |
| --- | --- |
| `/paxeer-x/archive/anchors` | Frozen historical anchor checkpoints |
| `/paxeer-x/archive/receipts` | Frozen historical receipts |
| `/paxeer-x/archive/receipts/:id` | One frozen receipt by its full hash |

### Frontend integration

The web frontend's
[`paxeerX.ts`](../../../apps/explorer/lib/api/services/general/paxeerX.ts)
registers `paxeer_x_unified_account` and `paxeer_x_capabilities` against the
current routes above. Its `:hash` account parameter corresponds to the
router's `:address_hash_param`.
[`paxeerXFeed.ts`](../../../apps/explorer/lib/api/services/general/paxeerXFeed.ts)
registers `paxeer_x_feed_markets`, `paxeer_x_feed_update` and
`paxeer_x_feed_market_updates`; its market `:id` corresponds to `:market_id`.
Market lists return `items` and `chain_head`; market history returns `symbol`,
`items` and `next_page_params`. A submission detail returns one transaction
item with `method: "submitPrices"`, `transaction_types: ["price_update"]` and
the recorded observations under `price_update.observations`.

[`paxeerXLists.ts`](../../../apps/explorer/lib/api/services/general/paxeerXLists.ts)
registers the transaction status and signature resources alongside explicitly
archived anchor and receipt resources. Use `/transactions/:transaction_hash_param/status`
for current transaction observations. Unified accounts and native feeds do
not require archive records or the retired unprefixed anchor/receipt routes.

### Native account, status and capabilities

[`UnifiedAccountController`](apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/unified_account_controller.ex)
returns `identities` (`evm`, `pax`, `did`), `balances`, `activity`,
`next_page_params`, `page_cursor`, `first_page_cursor`, `page_number` and
`activity_total`. Follow the returned `next_page_params.cursor` unchanged;
cursors are bound to the account and snapshot and expire after one hour.
Invalid cursors return HTTP `422` with `message: "Invalid activity cursor"`
and a `reason`.

[`TransactionStatusController`](apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_status_controller.ex)
returns `rung`, `block_number` and `committed_height`. An indexed consensus
block at or below the indexed committed head gives `committed`. Other observed
states can be `submitted` or `receipted`; neither establishes finality. A hash
with neither an indexed transaction nor verified receipt evidence returns
HTTP `404`. The
[signature endpoint](apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller.ex)
returns HTTP `404` when its receipt evidence cannot be verified. These
observations do not add another settlement network.

[`CapabilitiesController`](apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/capabilities_controller.ex)
returns `addr`, `exchange`, `bridge`, `launchpad` and `checked_at`. Capability
probing is controlled by `PAXEER_X_CAPABILITIES_ENABLED` and
`PAXEER_X_CAPABILITIES_REFRESH_SECONDS` (default `30`). Before a successful
probe, the surface flags are false and `checked_at` is null; a failed refresh
retains the previous snapshot. `price_feed: true` is included only when the
fetcher is active or, with the indexer disabled, the database has a successfully
scanned feed block within 1000 blocks of the indexed head.

### Native feed and frozen archives

The [feed controller](apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/feed_controller.ex)
reads indexed native `MsgSubmitPrices` transactions from consensus blocks.
Enabling collection with `PAXEER_X_PRICE_FEED_ENABLED=true` requires
`PAXEER_X_COSMOS_REST_URLS`, `PAXEER_X_COMETBFT_RPC_URLS` (comma-separated
HTTP or HTTPS URL lists) and a non-negative `PAXEER_X_PRICE_FEED_START_BLOCK`.
The market-discovery precompile is configured by
`PAXEER_X_PRICE_FEED_PRECOMPILE`, default
`0x0000000000000000000000000000000000001020`.
`PAXEER_X_PRICE_FEED_RETENTION_DAYS` must be positive when set; unset means no
retention limit. The Compose environment example sets it to `30`.

The [archive controller](apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex)
serves frozen history, not current finality evidence. Lists return `items` and
`next_page_params`; each archive item has `frozen: true`. Use the returned
pagination parameters. An invalid or
unknown receipt ID returns HTTP `404`. The old unprefixed
`/api/v2/paxeer-x/anchors` and `/api/v2/paxeer-x/receipts` routes were removed
and return not-found; use the explicit `/archive/` paths above.

### Explorer accounts

User preferences, private tags, watchlists, API keys and custom ABIs use
`/api/account/v2`, with session authentication and CSRF protection. These
explorer user accounts are distinct from native chain identities. See the
[current account API blueprint](apps/block_scout_web/API%20blueprint.md) for
routes, payloads and errors. Account functionality requires `ACCOUNT_ENABLED`;
when disabled its API returns HTTP `404` with
`{"message":"Account functionality is disabled"}`.

Runtime settings are read in [`config/runtime.exs`](config/runtime.exs); the
[backend Compose preset](../deploy/env/backend.example.env) documents the
local deployment values. The public OpenAPI document is served at
`/api/v2/openapi` when the API is enabled.

## Building and testing

Use the Elixir and Erlang versions pinned in [`.tool-versions`](.tool-versions).
From this directory:

```sh
mix deps.get
mix compile
```

From the monorepo root,
`services/explorer/deploy/tools/mix-in-builder.sh` runs one mix command inside
the pinned builder image with a disposable PostgreSQL sidecar. For example:

```sh
services/explorer/deploy/tools/mix-in-builder.sh compile
services/explorer/deploy/tools/mix-in-builder.sh test apps/explorer/test/explorer/chain/paxeer_x
```

`scripts/explorer/gate-test.sh` runs the Paxeer X suites per umbrella
application, frontend checks and signature-provider tests. The
[explorer overview](../README.md) describes the builder, development gates,
lint scripts and local Docker Compose stack.

## Contributing

Upstream's contribution guide and code of conduct are kept in
[CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

GNU General Public License v3.0. See [LICENSE](LICENSE).
