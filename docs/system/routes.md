# Network and hosted service routes

Paxeer mainnet-beta is one BFT network with EVM chain ID `125` (`0x7d`).
The kernel runs as part of the network and proposes blocks. A receipt is final
when its block is committed. Hosted services provide application APIs and views
of that network; they do not form a second execution or settlement network.
Sub-chains are tenant domains in the database, launchable without new infrastructure.

This page describes the checked-in route registrations. A registered application
route still needs its configured upstream, authentication and readiness checks
before it can serve requests. A backend route and a route admitted by the hosted
gateway are identified separately below.

## Public node JSON-RPC

The public node endpoints serve JSON-RPC at the root path over HTTPS and
WebSocket. Select a node URL from the [public RPC reference](../site/docs/reference/public-rpc.md).

The node registers `debug`, `echo`, `eth`, `net`, `pax`, `pax2`, `px`,
`txpool` and `web3`. The `test` namespace is opt-in on non-live chains.
Transport and legacy-method settings are described in [EVM JSON-RPC](rpc.md).
Check `eth_chainId` for `0x7d` and compare `eth_blockNumber` across configured nodes.
Node availability does not imply that every method in an upstream Geth namespace
is supported. See [Public RPC endpoints](../site/docs/reference/public-rpc.md)
for the individual URLs and client examples. The root HTTP/WebSocket proxy mapping
is in [the node installation template](../../tools/bringup/install-archive.sh).

The old router and its `/rpc`, `/rpc/ws`, `/rpc/schema` and `/rpc/evm/ws` public topology are retired.
There is no separate LayerX interface, sequencer service or independent guarantor finality.
The `layerxd` binary is retired.
The hpx library mirror distributes build libraries; it does not serve chain RPC.

## Sources and discovery

The authoritative dispatch code for hosted requests is
[gateway main.rs](../../services/platform/hosted/gateway/src/main.rs),
[routes.rs](../../services/platform/hosted/gateway/src/routes.rs),
[explorer_proxy.rs](../../services/platform/hosted/gateway/src/explorer_proxy.rs)
and [ui_proxy.rs](../../services/platform/hosted/gateway/src/ui_proxy.rs).
Hosted JSON-RPC method contracts are in
[openrpc.json](../../services/platform/hosted/gateway/openrpc.json), with dispatch
in [rpc.rs](../../services/platform/hosted/gateway/src/rpc.rs).
These hosted `px_` methods are not promises of methods on every public node.

The [route catalogue](../../tools/paxeer-x/route-catalogue.json), schema
`paxeer-x.routes.v1`, is compiled into the gateway. It supplies generic proxy
metadata, but also retains historical service descriptions, paths and node
records. Its total row count is not a count of reachable merged-network APIs.
[gen-system-docs.sh](../../tools/docs/gen-system-docs.sh) renders catalogue
metadata. Use the dispatch sources above to determine whether a route is
admitted, which authentication it requires and how unavailable backends refuse
requests.

| Hosted discovery request | Authentication | Response |
| --- | --- | --- |
| `GET /v1/routes` | Gateway API key | Catalogue plus configured readiness |
| `GET /v1/routes/{service}` | Gateway API key | Product service metadata and routes; 503 if unhealthy, 403 for private services, 404 for unknown services |
| JSON-RPC `px_getRouteCatalogue` | Gateway API key | Hosted catalogue contract; see OpenRPC for parameters |

The protected `LAYERX_GATEWAY_ROUTE_BINDINGS_FILE` configures generic service
bindings and allowed origins. The gateway checks readiness, network identity and
wire version before forwarding. A missing or unhealthy generic binding returns
503 `route_unavailable`. Discovery metadata alone does not enable an upstream.

## Services

The names below identify owners and catalogue groups. They do not describe
independently deployed consensus components. Application routes use the
configured hosted origin; the public node origin above is not a universal
origin for all application paths.

| Owner or catalogue group | Current role and route boundary |
| --- | --- |
| kernel, paxeer-boundaries | Network execution and node state; use root node RPC. Historical group names do not define a settlement hop. |
| explorer | Explorer frontend and Blockscout backend through explicit hosted allowlists; native account views have their own binding. |
| wallet-ui, wallet-gateway | Wallet pages and authenticated wallet/agent APIs; distinguish service registrations from hosted catalogue admission. |
| human | Schema-defined account, session, agent, activity and security APIs; retired operations return 410. |
| human-web | Optional evidence page/API profile retained in UI proxy code; not a node RPC or finality authority. |
| agentd, mcp-a2a | Agent RPC and configured MCP tools; service and owner authentication still apply. |
| gas, interop, ramp, search-web, mirror | Application adapters with route-specific authorization and upstream bindings. |
| indexer | Hosted history projections and reads; network commitment determines finality. |
| bridge, markets | Contract/state reads through node RPC and their owning application APIs. |
| hpx | Library/artifact distribution and registry, not chain RPC. |
| archive | Historical catalogue group; the relay archive package and beta deployment are retired. Explorer archive views remain separately mounted. |
| program-registry, programs-runtime | Historical REST groups; external Programs REST ingress is refused by the hosted gateway. |
| webhooks-dashboard | Authenticated dashboard and webhook lifecycle/delivery APIs. |
| replica-authority, core-boundary | Historical/private catalogue groups, not a separate public finality or execution boundary. |
| ci, human-kms, identity, internal-services, private-profile, redis, wallet-attestors, xweb-attestors | Private implementation/deployment groups; service discovery does not make them public APIs. |

## Routes by service

Templates use `{name}`, `:name` or `[name]` as documented by the owning handler.
They are placeholders, not literal URL characters or permission to traverse an
arbitrary path. Each table states whether it describes hosted admission, an
upstream registration or a retired compatibility surface.

### explorer

Hosted backend requests have prefix `/explorer/backend`. The prefix is stripped
before forwarding to the configured Blockscout backend, preserving the admitted
query string. The following table is the explicit `ROUTES` registration in
`explorer_proxy.rs`; prepend `/explorer/backend` to each upstream path.

All requests require the configured public Host. Supplied Origin and Referer must
match the public explorer origin. Unsafe requests require Origin or Referer;
rows marked session/CSRF also require a nonempty `x-csrf-token` for non-GET
requests. Backend account authentication and authorization still apply.
Only the admitted explorer cookies are forwarded and response cookies are scoped
to `/explorer` with Secure. These routes do not require a generic gateway API key.

| Method | Upstream path | Policy |
| --- | --- | --- |
| GET | `/api/account/auth/logout` | Session/CSRF |
| POST | `/api/account/v2/address/link` | Session/CSRF |
| GET | `/api/account/v2/authenticate` | Session/CSRF |
| POST | `/api/account/v2/authenticate` | Session/CSRF |
| GET | `/api/account/v2/authenticate_via_dynamic` | Explorer origin/header checks |
| POST | `/api/account/v2/authenticate_via_wallet` | Explorer origin/header checks |
| POST | `/api/account/v2/confirm_otp` | Explorer origin/header checks |
| POST | `/api/account/v2/email/link` | Session/CSRF |
| GET | `/api/account/v2/email/resend` | Session/CSRF |
| GET | `/api/account/v2/get_csrf` | Session/CSRF |
| POST | `/api/account/v2/send_otp` | Explorer origin/header checks |
| GET | `/api/account/v2/siwe_message` | Explorer origin/header checks |
| GET | `/api/account/v2/user/api_keys` | Session/CSRF |
| POST | `/api/account/v2/user/api_keys` | Session/CSRF |
| DELETE | `/api/account/v2/user/api_keys/{api_key}` | Session/CSRF |
| PUT | `/api/account/v2/user/api_keys/{api_key}` | Session/CSRF |
| GET | `/api/account/v2/user/custom_abis` | Session/CSRF |
| POST | `/api/account/v2/user/custom_abis` | Session/CSRF |
| DELETE | `/api/account/v2/user/custom_abis/{id}` | Session/CSRF |
| PUT | `/api/account/v2/user/custom_abis/{id}` | Session/CSRF |
| GET | `/api/account/v2/user/info` | Session/CSRF |
| GET | `/api/account/v2/user/tags/address` | Session/CSRF |
| POST | `/api/account/v2/user/tags/address` | Session/CSRF |
| DELETE | `/api/account/v2/user/tags/address/{id}` | Session/CSRF |
| GET | `/api/account/v2/user/tags/address/{id}` | Session/CSRF |
| PUT | `/api/account/v2/user/tags/address/{id}` | Session/CSRF |
| GET | `/api/account/v2/user/tags/transaction` | Session/CSRF |
| POST | `/api/account/v2/user/tags/transaction` | Session/CSRF |
| DELETE | `/api/account/v2/user/tags/transaction/{id}` | Session/CSRF |
| GET | `/api/account/v2/user/tags/transaction/{id}` | Session/CSRF |
| PUT | `/api/account/v2/user/tags/transaction/{id}` | Session/CSRF |
| GET | `/api/account/v2/user/watchlist` | Session/CSRF |
| POST | `/api/account/v2/user/watchlist` | Session/CSRF |
| DELETE | `/api/account/v2/user/watchlist/{id}` | Session/CSRF |
| PUT | `/api/account/v2/user/watchlist/{id}` | Session/CSRF |
| GET | `/api/v1/search` | Explorer origin/header checks |
| GET | `/api/v2/addresses` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/blocks-validated` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/coin-balance-history` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/coin-balance-history-by-day` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/counters` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/internal-transactions` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/internal-transactions/csv` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/logs` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/logs/csv` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/nft` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/nft/collections` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/tabs-counters` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/token-balances` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/token-transfers` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/token-transfers/csv` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/tokens` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/transactions` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/transactions/csv` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/unified` | Explorer origin/header checks |
| GET | `/api/v2/addresses/{address_hash_param}/withdrawals` | Explorer origin/header checks |
| GET | `/api/v2/blocks` | Explorer origin/header checks |
| GET | `/api/v2/blocks/{block_hash_or_number_param}` | Explorer origin/header checks |
| GET | `/api/v2/blocks/{block_hash_or_number_param}/internal-transactions` | Explorer origin/header checks |
| GET | `/api/v2/blocks/{block_hash_or_number_param}/transactions` | Explorer origin/header checks |
| GET | `/api/v2/blocks/{block_hash_or_number_param}/withdrawals` | Explorer origin/header checks |
| GET | `/api/v2/blocks/{block_number_param}/countdown` | Explorer origin/header checks |
| GET | `/api/v2/config/backend` | Explorer origin/header checks |
| GET | `/api/v2/config/backend-version` | Explorer origin/header checks |
| GET | `/api/v2/config/csv-export` | Explorer origin/header checks |
| GET | `/api/v2/config/db-background-migrations` | Explorer origin/header checks |
| GET | `/api/v2/config/indexer` | Explorer origin/header checks |
| GET | `/api/v2/config/public-metrics` | Explorer origin/header checks |
| GET | `/api/v2/config/smart-contracts/languages` | Explorer origin/header checks |
| GET | `/api/v2/internal-transactions` | Explorer origin/header checks |
| GET | `/api/v2/main-page/blocks` | Explorer origin/header checks |
| GET | `/api/v2/main-page/indexing-status` | Explorer origin/header checks |
| GET | `/api/v2/main-page/transactions` | Explorer origin/header checks |
| GET | `/api/v2/main-page/transactions/watchlist` | Explorer origin/header checks |
| GET | `/api/v2/paxeer-x/archive/anchors` | Explorer origin/header checks |
| GET | `/api/v2/paxeer-x/archive/receipts` | Explorer origin/header checks |
| GET | `/api/v2/paxeer-x/archive/receipts/{id}` | Explorer origin/header checks |
| GET | `/api/v2/paxeer-x/capabilities` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/accounts` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/accounts/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/bundlers` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/bundlers/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/bundles` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/factories` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/factories/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/operations` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/operations/{operation_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/operations/{operation_hash_param}/summary` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/paymasters` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/paymasters/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/proxy/account-abstraction/status` | Explorer origin/header checks |
| GET | `/api/v2/search` | Explorer origin/header checks |
| GET | `/api/v2/search/check-redirect` | Explorer origin/header checks |
| GET | `/api/v2/search/quick` | Explorer origin/header checks |
| GET | `/api/v2/smart-contracts/` | Explorer origin/header checks |
| GET | `/api/v2/smart-contracts/counters` | Explorer origin/header checks |
| GET | `/api/v2/smart-contracts/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/smart-contracts/{address_hash_param}/audit-reports` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash_param}/audit-reports` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/flattened-code` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/multi-part` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/sourcify` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/standard-input` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/vyper-code` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/vyper-multi-part` | Explorer origin/header checks |
| POST | `/api/v2/smart-contracts/{address_hash}/verification/via/vyper-standard-input` | Explorer origin/header checks |
| GET | `/api/v2/stats` | Explorer origin/header checks |
| GET | `/api/v2/stats/charts/market` | Explorer origin/header checks |
| GET | `/api/v2/stats/charts/secondary-coin-market` | Explorer origin/header checks |
| GET | `/api/v2/stats/charts/transactions` | Explorer origin/header checks |
| GET | `/api/v2/stats/hot-smart-contracts` | Explorer origin/header checks |
| GET | `/api/v2/token-transfers` | Explorer origin/header checks |
| GET | `/api/v2/tokens/` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/counters` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/holders` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/holders/csv` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/instances` | Explorer origin/header checks |
| PATCH | `/api/v2/tokens/{address_hash_param}/instances/refetch-metadata` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/instances/{token_id_param}` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/instances/{token_id_param}/holders` | Explorer origin/header checks |
| PATCH | `/api/v2/tokens/{address_hash_param}/instances/{token_id_param}/refetch-metadata` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/instances/{token_id_param}/transfers` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/instances/{token_id_param}/transfers-count` | Explorer origin/header checks |
| GET | `/api/v2/tokens/{address_hash_param}/transfers` | Explorer origin/header checks |
| GET | `/api/v2/transactions` | Explorer origin/header checks |
| GET | `/api/v2/transactions/stats` | Explorer origin/header checks |
| GET | `/api/v2/transactions/watchlist` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/external-transactions` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/internal-transactions` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/logs` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/raw-trace` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/signature` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/state-changes` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/status` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/summary` | Explorer origin/header checks |
| GET | `/api/v2/transactions/{transaction_hash_param}/token-transfers` | Explorer origin/header checks |
| GET | `/api/v2/withdrawals` | Explorer origin/header checks |
| GET | `/api/v2/withdrawals/counters` | Explorer origin/header checks |
| GET | `/socket/v2/websocket` | Explorer origin/header checks |

The socket entry uses WebSocket upgrade handling, not an ordinary GET response.
`/explorer/backend/socket/v2/websocket` carries the explorer socket protocol; it
is not the public node subscription endpoint. Use root node WebSocket for
`eth_subscribe`.

The retained archive routes are explorer database views under
`/explorer/backend/api/v2/paxeer-x/archive/`. The transaction signature and status
routes are under `/explorer/backend/api/v2/transactions/{transaction_hash_param}/`.
Their backend declarations are in
[api_router.ex](../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex).
Archive names and historical anchor fields do not create another finality layer.
The hosted capabilities response removes the `custody` and `anchor` keys.

Explorer target bounds are 4096 bytes overall, 2048 for the path and 2048 for the
query. Paths reject percent escapes, backslashes, duplicate slashes and traversal;
query percent escapes must be valid. A malformed target returns 400
`explorer_target_refused`. Unknown routes return 404 and a wrong method on a
known route returns 405, both with `explorer_route_not_admitted`. Missing backend
configuration returns 503 `explorer_not_configured`; a failed upstream request
returns 502 `explorer_upstream_unavailable`.

The frontend uses GET/HEAD page and asset allowlists in `ui_proxy.rs`. Common
page patterns include:

| Hosted page | Purpose |
| --- | --- |
| `/explorer`, `/explorer/` | Explorer home |
| `/explorer/blocks`, `/explorer/block/[height_or_hash]` | Blocks |
| `/explorer/txs`, `/explorer/tx/[hash]` | Transactions |
| `/explorer/accounts`, `/explorer/address/[hash]` | Accounts and address views |
| `/explorer/tokens`, `/explorer/token/[hash]` | Tokens |
| `/explorer/advanced-filter` | Advanced filter page; its backend APIs are listed separately below |
| `/explorer/account/api-key`, `/explorer/account/watchlist`, `/explorer/account/custom-abi` | Account pages with upstream account authorization |
| `/explorer/paxeer-x/account/[hash]`, `/explorer/paxeer-x/receipts`, `/explorer/paxeer-x/receipts/[id]`, `/explorer/paxeer-x/anchors` | Retained native/history page registrations; names do not establish a separate network |
| `/explorer/api/healthz`, `/explorer/node-api/healthz` | Frontend health API registrations, distinct from gateway readiness |

Next data and static resources use constrained paths such as
`/explorer/_next/data/{build_id}/{page}.json` and `/explorer/_next/static/{asset}`.
Public icons and configuration assets use explicit lists/prefix policies in
`ui_proxy.rs`; these examples do not admit arbitrary files. Frontend account
pages, backend account routes and the explorer socket have different handlers.

#### Price feed API admission

The explorer backend proxy admits these three read-only feed routes through its
existing target, origin, header and upstream checks. Market ids contain only
ASCII decimal digits (at most 256 characters); update hashes are `0x` followed by
64 hexadecimal digits. Other feed paths and methods remain refused. Query
parameters, including the `block_number` and `index` history cursor, are forwarded
unchanged to the backend.

| Method | Admitted hosted path | Backend action |
| --- | --- | --- |
| GET | `/explorer/backend/api/v2/paxeer-x/feed/markets` | `FeedController.markets` |
| GET | `/explorer/backend/api/v2/paxeer-x/feed/markets/{market_id}/updates` | `FeedController.market_updates` |
| GET | `/explorer/backend/api/v2/paxeer-x/feed/updates/{hash}` | `FeedController.update` |

The overview carries `FeedView` markets and chain head; history carries items and
`next_page_params`; detail carries the stored type-127 submission, observation
acceptance flags and exact decimal price strings.

#### Backend-only explorer APIs

The backend declares the following GET routes. The hosted explorer proxy does
not admit them.

| Method | Backend path | Backend action |
| --- | --- | --- |
| GET | `/api/v2/advanced-filters` | `AdvancedFilterController.list` |
| GET | `/api/v2/advanced-filters/csv` | `AdvancedFilterController.list_csv` |
| GET | `/api/v2/advanced-filters/methods` | `AdvancedFilterController.list_methods` |
| GET | `/api/v2/paxeer-x/feed/updates` | `FeedController.updates` |

The feed page patterns `/explorer/feed`,
`/explorer/feed/market/[id]` and `/explorer/feed/update/[hash]` are also absent from
the hosted frontend page allowlist. The existing `/explorer/advanced-filter`
page registration does not admit its backend API by implication.

The [backend admission inventory](../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/hosted_route_admission.json)
is separate from the Rust frontend and backend allowlists. The hosted frontend
serves only its admitted pages and constrained Next data routes.

### wallet-gateway

The wallet service registers its handlers in
[index.ts](../../services/wallet/gateway/src/index.ts). These are upstream
registrations. The hosted generic proxy additionally requires a matching
catalogue entry and healthy binding; registration in Fastify does not add an
entry to the hosted allowlist.

Current wallet transaction and approval routes include:

| Method | Upstream path | Guard | Handler source |
| --- | --- | --- | --- |
| POST | `/v1/wallet/provision` | requireAuth | [wallet.ts](../../services/wallet/gateway/src/routes/wallet.ts) |
| GET | `/v1/wallet/me` | requireAuth | [wallet.ts](../../services/wallet/gateway/src/routes/wallet.ts) |
| POST | `/v1/wallet/sign` | requireAuth | [sign.ts](../../services/wallet/gateway/src/routes/sign.ts) |
| POST | `/v1/wallet/send` | requireAuth | [sign.ts](../../services/wallet/gateway/src/routes/sign.ts) |
| POST | `/v1/wallet/sign-digest` | requireAuth | [sign.ts](../../services/wallet/gateway/src/routes/sign.ts) |
| POST | `/v1/wallet/sign-message` | requireAuth | [sign.ts](../../services/wallet/gateway/src/routes/sign.ts) |
| POST | `/v1/wallet/sign-typed-data` | requireAuth | [sign.ts](../../services/wallet/gateway/src/routes/sign.ts) |
| POST | `/v1/wallet/evm/review` | requireAuth | [evmApprovals.ts](../../services/wallet/gateway/src/routes/evmApprovals.ts) |
| POST | `/v1/wallet/evm/approve` | requireAuth | [evmApprovals.ts](../../services/wallet/gateway/src/routes/evmApprovals.ts) |
| POST | `/v1/wallet/evm/sign` | requireAuth | [evmApprovals.ts](../../services/wallet/gateway/src/routes/evmApprovals.ts) |
| GET | `/v1/wallet/evm/approvals/:id` | requireAuth | [evmApprovals.ts](../../services/wallet/gateway/src/routes/evmApprovals.ts) |
| POST | `/v1/wallet/sponsored/quote` | requireAuth | [wallet.ts](../../services/wallet/gateway/src/routes/wallet.ts) |
| POST | `/v1/wallet/sponsored/submit` | requireAuth | [wallet.ts](../../services/wallet/gateway/src/routes/wallet.ts) |
| POST | `/v1/wallet/sponsored/status` | requireAuth | [wallet.ts](../../services/wallet/gateway/src/routes/wallet.ts) |
| GET | `/healthz` | Public service health | [index.ts](../../services/wallet/gateway/src/index.ts) |
| GET | `/readyz` | Service readiness | [readiness.ts](../../services/wallet/gateway/src/routes/readiness.ts) |
| GET | `/v1/admin/readiness` | Bearer `GATEWAY_ADMIN_TOKEN` | [readiness.ts](../../services/wallet/gateway/src/routes/readiness.ts) |

The hosted catalogue includes the EVM approval paths and
`/v1/wallet/sponsored/quote`, together with sponsorship submit and status.
Requests still require a healthy service binding and upstream authorization.

Agent reads use `requireAgent`; value-moving/signing requests use
`requireSignedAgentRequest`. The current
[agent action handler](../../services/wallet/gateway/src/routes/agentActions.ts)
registers `POST /v1/agent/actions/allowance-and-call` and
`GET /v1/agent/actions/:action_id`. The old LayerX deposit action is removed from
that handler. Agent authentication challenge/verify/refresh, agent reads and
precompile helpers are registered by the route modules imported in `index.ts`.
They operate on the same network; they do not establish a second execution lane.

The old `/v1/wallet/lx/review`, `/v1/wallet/lx/approve`,
`/v1/wallet/lx/sign` and `/v1/wallet/lx/approvals/:id` names still appear in
[sign.ts](../../services/wallet/gateway/src/routes/sign.ts) and the hosted
catalogue as legacy activity-approval compatibility handlers. They are not the
EVM approval flow above or evidence of a separate network interface. Do not
assume every old wallet path returns 410: status depends on the owning handler.

### wallet-ui

Wallet pages are GET/HEAD routes under `/wallet`, forwarded by the UI proxy to
the protected wallet origin. Examples are `/wallet/`, `/wallet/offline/`,
`/wallet/privacy/` and `/wallet/surfaces/bridge/`. Static icons, splash screens,
service-worker and manifest files are constrained by the explicit UI lists and
asset policies. Hundreds of image GET/HEAD entries in the old catalogue are
static files, not distinct transaction APIs.

`LAYERX_GATEWAY_UI_BINDINGS_FILE` provides the protected wallet origin and the
optional Human evidence profile. UI proxying does not bypass upstream session
or operation checks.

### human

The Human HTTP server selects operations from the checked-in
[Human API schema](../../services/human/schema/human-api/README.md).
Its [HTTP handler](../../services/human/crates/layerx-human-service/src/server/http.rs)
checks `operation.retired()` before authorization and dispatch. Retired schema
operations return HTTP 410 with code `operation-retired`, copy key
`error.operation.retired` and retry classification `final`.

The following are the retired operation routes in that schema. Catalogue aliases
under `/human` may forward to the same upstream operations; an alias does not
restore a retired operation.

| Method | Upstream path | Status |
| --- | --- | --- |
| POST | `/v1/wallet-binding/statement` | Retired; 410 `operation-retired` |
| POST | `/v1/wallet-binding` | Retired; 410 `operation-retired` |
| GET | `/v1/wallet-binding` | Retired; 410 `operation-retired` |
| POST | `/v1/wallet-binding/rebind/action` | Retired; 410 `operation-retired` |
| POST | `/v1/wallet-binding/rebind` | Retired; 410 `operation-retired` |
| POST | `/v1/deposits` | Retired; 410 `operation-retired` |
| POST | `/v1/deposits/{journey_id}/confirm` | Retired; 410 `operation-retired` |
| POST | `/v1/withdrawals` | Retired; 410 `operation-retired` |
| POST | `/v1/withdrawals/{journey_id}/claim` | Retired; 410 `operation-retired` |
| POST | `/v1/exit` | Retired; 410 `operation-retired` |
| GET | `/v1/exit/eligibility` | Retired; 410 `operation-retired` |

Account/session bootstrap, profile, security, notifications, support and agent
operations must be read from their schema sections, including their authorization
class and idempotency requirement. For example, retained intent and movement
route names do not promise that retired deposit, withdrawal or custody
operations are available. The schema and component implementation determine
the accepted operation and response, not the old catalogue purpose string.

### human-web

The optional profile in `ui_proxy.rs` retains GET/HEAD `/explorer/verify`,
POST `/api/explorer/verify`, and constrained GET/HEAD assets under
`/human-ui/_next/static/`. It requires a configured Human web origin and actual
page health checks. These registrations describe compatibility evidence UI,
not an independent checkpoint/finality service or a required public node page.

### archive

The standalone relay archive package and beta deployment are retired; the old
`/v1/history/*`, `/v1/sync/*` and `/v1/peers` catalogue rows are historical metadata,
not a supported archive deployment contract. The beta image/cluster definitions
no longer deploy that service. Retained explorer archive routes are listed in
[explorer](#explorer) and are owned by the explorer backend.

### kernel

The kernel is part of the network. The hosted gateway retains a small
compatibility production parser in
[lib.rs](../../services/platform/hosted/gateway/src/lib.rs):

| Method | Hosted compatibility path | Constraints |
| --- | --- | --- |
| POST | `/v1/settle` | Exact path; component backend, gateway API key and route scope required |
| GET | `/v1/state` | Exact path; same backend and authorization requirements |
| GET | `/v1/receipts/{activity_id}` | Identifier is exactly 64 hexadecimal characters; same backend and authorization requirements |

These code registrations do not imply a separate settlement network or a live
component backend. Missing backend authority must return unavailable rather
than fabricated state. Public reads also register GET
`/v1/accounts/{id}/balance` and `/v1/dids/{did}/accounts` in
[public_reads.rs](../../services/platform/hosted/gateway/src/public_reads.rs),
with selector validation and bounded per-client/global read limits.

Hosted JSON-RPC is POST `/` with `application/json`. It supports the methods
in OpenRPC, including `px_register`, account/balance/asset reads,
`px_getHistory`, `px_getUnifiedHistory`, `px_getCommit`, `px_getValidators`,
`px_getReceiptKeys`, `px_getSequenceBatch`, `px_requestFunds`,
`px_listHolderRecords`, `px_getNetwork`, `px_getCapabilities` and
`px_getRouteCatalogue`. Method-specific validation, authorization and backend
availability apply. The dispatcher refuses `lx_` methods with JSON-RPC -32601
`Method not found`; old `lx_subscribe`/`lx_unsubscribe` entries are retired.

### paxeer-boundaries

This is a historical catalogue label. Use the root node HTTP/WebSocket endpoints
in [Public node JSON-RPC](#public-node-json-rpc) for chain access. Some hosted
source branches still recognize retired prefixed RPC paths; their presence is
not a supported public node endpoint contract. Do not copy the old `web3_*`
catalogue row as the current node namespace list.

### program-registry

External `/v1/programs` and every descendant path are retired REST ingress.
The gateway returns 404 `not_found` before generic catalogue dispatch, including
registry reads, interface/source lookups and source publication. Historical
registry metadata does not make these routes live.

### programs-runtime

The same retired `/v1/programs` prefix refusal covers call, simulate, read,
deploy, upgrade, wind-down, activity and idempotency receipt routes. External
`/v1/activities` and its descendants are also refused with 404 `not_found`.
Use the merged network's transaction interfaces and committed chain evidence;
there is no separate Programs executor service implied by these old groups.

### bridge

The catalogue's `eth_call` row is a JSON-RPC method, not a REST bridge path.
Bridge contract reads use the node's root endpoint and the deployed contract
ABI/address for the relevant network. The wallet bridge UI is an application
surface; its existence does not make the retired deposit action available.

### markets

Likewise, the duplicated `eth_call` entries describe contract reads, not two
market HTTP routes or a settlement boundary. Resolve current contract and
precompile interfaces from their owning implementation before constructing a call.

### indexer

`px_getHistory` and `px_getUnifiedHistory` are hosted JSON-RPC methods with the
OpenRPC parameter and response contracts. Indexer/history views do not create
independent finality; commitment of the network block remains authoritative.

### agentd

POST `/v1/agent/rpc` is handled specially by `routes.rs`: it requires a gateway
API key, a configured agentd binding with matching network identity, and an
application/json request within the 1 MiB body limit. Agent method/session
requirements remain the owning service's responsibility.

### mcp-a2a

The gateway handles `/mcp` using configured MCP bindings. Tool selection and
owner/principal authorization apply; old `tools/call` names in the catalogue are
tool operations, not independent HTTP paths. See the binding and MCP handling
in `routes.rs` rather than treating every metadata row as a mounted API.

### gas

The hosted catalogue admits POST and OPTIONS for `/gas-station/quote`,
`/gas-station/submit`, `/gas-station/status` and `/gas-station/retry`, forwarding
to the gas upstream's corresponding unprefixed path. The older POST `/quote`
and `/submit` entries are also generic proxy declarations. Preflight origins
and signed sponsorship requests are validated; a quote is not a transaction
receipt. Wallet `/v1/wallet/sponsored/quote` is a separate upstream registration.

### hpx

The generic catalogue declares GET `/hpx/api/peers`, `/hpx/api/peers.txt`,
`/hpx/api/nodes`, `/hpx/api/statesync`, `/hpx/api/myip` and POST
`/hpx/api/register`. These require gateway API-key authorization, the hpx ingress
policy and healthy service binding. Their upstream paths strip `/hpx`.
They are registry/distribution endpoints, not node JSON-RPC.

### interop

Generic proxy declarations cover `/v1/adapters`, protocol-specific paths under
`/v1/http/`, `/v1/mcp/`, `/v1/a2a/`, `/v1/operations/{operation}` and
`/v2/migration/`. Each declaration selects an exact method and owning adapter;
prefixes are not unrestricted mounts. Most operations require a gateway API key;
the adapters listing is public. Migration additionally requires customer
authorization and refuses client-supplied internal identity headers.

### mirror

GET `/mirror/status` is a generic proxy declaration requiring a gateway API key
and healthy mirror binding. It reports application mirror status, not network
finality.

### ramp

POST `/v1/orders`, POST `/v1/provider-callbacks` and GET `/v1/orders/{digest}`
are upstream ramp contracts. The hosted catalogue also declares aliases under
`/v1/ramp/`, including GET `/v1/ramp/readyz` mapped to upstream `/readyz`.
Provider/customer authority is checked by the owning ramp service.

### search-web

GET `/search`, `/fetch` and `/content/{digest}` are declared generic proxy paths
for the search-web upstream. The service enforces x402 payment requirements;
a hosted binding does not make a paid request public or free.

### webhooks-dashboard

The generic catalogue includes dashboard reads under `/v1/dashboard/`, webhook
endpoint/delivery/dead-letter management under `/v1/webhooks/`, and the public
GET `/v1/webhooks/scheme`. Principal sessions protect the other declared
operations. Creation, key rotation, suspension/resumption and replay use their
exact declared methods and identifiers; discovery alone does not authorize them.

## Authentication, transport and readiness

Generic proxy rows carry authentication labels consumed by `routes.rs` and the
upstream service. Gateway API-key routes authenticate at the gateway. Human,
wallet, interop, ramp and dashboard requests retain upstream authorization;
public read, signed operation and session-guarded routes have different policies.
Never send trusted internal principal/API-key headers from a client: the gateway
refuses supplied `x-layerx-principal` and `x-layerx-api-key` headers.

The generic HTTP client uses a 3-second connection timeout and 8-second I/O
timeouts. Catalogue proxy rows declare zero retries. These are per-operation
settings, not an end-to-end deadline guarantee for every transport. UI streaming
and explorer sockets have specialized handling in `http.rs`, `ui_proxy.rs` and
`explorer_proxy.rs`. Readiness checks and backend refusals must be preserved;
repeated submission is not a safe response to an ambiguous timeout.

Hosted GET `/livez`, `/readyz`, `/readyz/core`, `/v1/status` and `/metrics` are
gateway operational registrations. They do not replace the node's root
`eth_chainId` check, wallet readiness or explorer backend health. Configure
`LAYERX_GATEWAY_PUBLIC_HOST`, `LAYERX_GATEWAY_EXPLORER_FRONTEND_URL` and
`LAYERX_GATEWAY_EXPLORER_BACKEND_URL` for the explorer proxy; upstream origins
are HTTPS origins, not credentials or node RPC aliases.

## Denied surfaces

The catalogue's denied surfaces remain:

- `event-publication`
- `operator-admin`
- `private-kms`
- `signer-shares`

Private service discovery returns 403 `private_service`. A public metadata row,
frontend asset or historical catalogue entry does not admit any of these private
surfaces or bypass the route, origin, identity and scope checks above.
