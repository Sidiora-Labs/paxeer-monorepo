# Explorer frontend contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## api-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-fe-api-config-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/config.ts:3</code> | <code>/api/config (API route, any method)</code> |
| <code>interface.14.http-fe-api-csrf-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/csrf.ts:9</code> | <code>/api/csrf (API route, any method)</code> |
| <code>interface.14.http-fe-api-healthz-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/healthz.tsx:3</code> | <code>/api/healthz (API route, any method)</code> |
| <code>interface.14.http-fe-api-log-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/log.ts:5</code> | <code>/api/log (API route, any method)</code> |
| <code>interface.14.http-fe-api-metrics-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/metrics.ts:8</code> | <code>/api/metrics (API route, any method)</code> |
| <code>interface.14.http-fe-api-monitoring-invalid-api-schema-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/monitoring/invalid-api-schema.ts:11</code> | <code>/api/monitoring/invalid-api-schema (API route, any method)</code> |
| <code>interface.14.http-fe-api-proxy-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/proxy.ts:55</code> | <code>/api/proxy (API route, any method)</code> |
| <code>interface.14.http-fe-api-tokens-hash-instances-id-media-type-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api/tokens/[hash]/instances/[id]/media-type.ts:11</code> | <code>/api/tokens/{hash}/instances/{id}/media-type (API route, any method)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-fe-api-config-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-csrf-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-healthz-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-log-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-metrics-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-monitoring-invalid-api-schema-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-proxy-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-api-tokens-hash-instances-id-media-type-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |

## pages

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-fe-page-404-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/404.tsx:28</code> | <code>GET /404 (page)</code> |
| <code>interface.14.http-fe-page-account-api-key-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/account/api-key.tsx:17</code> | <code>GET /account/api-key (page)</code> |
| <code>interface.14.http-fe-page-account-custom-abi-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/account/custom-abi.tsx:17</code> | <code>GET /account/custom-abi (page)</code> |
| <code>interface.14.http-fe-page-account-merits-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/account/merits.tsx:17</code> | <code>GET /account/merits (page)</code> |
| <code>interface.14.http-fe-page-account-tag-address-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/account/tag-address.tsx:17</code> | <code>GET /account/tag-address (page)</code> |
| <code>interface.14.http-fe-page-account-verified-addresses-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/account/verified-addresses.tsx:17</code> | <code>GET /account/verified-addresses (page)</code> |
| <code>interface.14.http-fe-page-account-watchlist-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/account/watchlist.tsx:17</code> | <code>GET /account/watchlist (page)</code> |
| <code>interface.14.http-fe-page-accounts-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/accounts/index.tsx:25</code> | <code>GET /accounts (page)</code> |
| <code>interface.14.http-fe-page-accounts-label-slug-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/accounts/label/[slug].tsx:17</code> | <code>GET /accounts/label/{slug} (page)</code> |
| <code>interface.14.http-fe-page-address-hash-contract-verification-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/address/[hash]/contract-verification.tsx:17</code> | <code>GET /address/{hash}/contract-verification (page)</code> |
| <code>interface.14.http-fe-page-address-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/address/[hash]/index.tsx:26</code> | <code>GET /address/{hash} (page)</code> |
| <code>interface.14.http-fe-page-advanced-filter-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/advanced-filter.tsx:16</code> | <code>GET /advanced-filter (page)</code> |
| <code>interface.14.http-fe-page-api-docs-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/api-docs.tsx:17</code> | <code>GET /api-docs (page)</code> |
| <code>interface.14.http-fe-page-apps-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/apps/[id].tsx:41</code> | <code>GET /apps/{id} (page)</code> |
| <code>interface.14.http-fe-page-apps-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/apps/index.tsx:15</code> | <code>GET /apps (page)</code> |
| <code>interface.14.http-fe-page-auth-profile-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/auth/profile.tsx:16</code> | <code>GET /auth/profile (page)</code> |
| <code>interface.14.http-fe-page-batches-celestia-height-commitment-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/batches/celestia/[height]/[commitment].tsx:34</code> | <code>GET /batches/celestia/{height}/{commitment} (page)</code> |
| <code>interface.14.http-fe-page-batches-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/batches/index.tsx:38</code> | <code>GET /batches (page)</code> |
| <code>interface.14.http-fe-page-batches-number-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/batches/[number].tsx:40</code> | <code>GET /batches/{number} (page)</code> |
| <code>interface.14.http-fe-page-blobs-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/blobs/[hash].tsx:18</code> | <code>GET /blobs/{hash} (page)</code> |
| <code>interface.14.http-fe-page-block-countdown-height-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/block/countdown/[height].tsx:18</code> | <code>GET /block/countdown/{height} (page)</code> |
| <code>interface.14.http-fe-page-block-countdown-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/block/countdown/index.tsx:18</code> | <code>GET /block/countdown (page)</code> |
| <code>interface.14.http-fe-page-block-height-or-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/block/[height_or_hash].tsx:18</code> | <code>GET /block/{height_or_hash} (page)</code> |
| <code>interface.14.http-fe-page-blocks-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/blocks.tsx:25</code> | <code>GET /blocks (page)</code> |
| <code>interface.14.http-fe-page-cc-tx-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/cc/tx/[hash].tsx:18</code> | <code>GET /cc/tx/{hash} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-accounts-label-slug-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/accounts/label/[slug].tsx:21</code> | <code>GET /chain/{chain_slug}/accounts/label/{slug} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-advanced-filter-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/advanced-filter/index.tsx:19</code> | <code>GET /chain/{chain_slug}/advanced-filter (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-block-countdown-height-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/block/countdown/[height].tsx:22</code> | <code>GET /chain/{chain_slug}/block/countdown/{height} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-block-countdown-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/block/countdown/index.tsx:22</code> | <code>GET /chain/{chain_slug}/block/countdown (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-block-height-or-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/block/[height_or_hash].tsx:18</code> | <code>GET /chain/{chain_slug}/block/{height_or_hash} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-csv-export-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/csv-export/index.tsx:19</code> | <code>GET /chain/{chain_slug}/csv-export (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-op-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/op/[hash].tsx:22</code> | <code>GET /chain/{chain_slug}/op/{hash} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-token-hash-instance-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/token/[hash]/instance/[id].tsx:21</code> | <code>GET /chain/{chain_slug}/token/{hash}/instance/{id} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-token-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/token/[hash]/index.tsx:21</code> | <code>GET /chain/{chain_slug}/token/{hash} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-tx-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/tx/[hash]/index.tsx:18</code> | <code>GET /chain/{chain_slug}/tx/{hash} (page)</code> |
| <code>interface.14.http-fe-page-chain-chain-slug-visualize-sol2uml-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chain/[chain_slug]/visualize/sol2uml.tsx:19</code> | <code>GET /chain/{chain_slug}/visualize/sol2uml (page)</code> |
| <code>interface.14.http-fe-page-chakra-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/chakra.tsx:16</code> | <code>GET /chakra (page)</code> |
| <code>interface.14.http-fe-page-contract-verification-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/contract-verification.tsx:17</code> | <code>GET /contract-verification (page)</code> |
| <code>interface.14.http-fe-page-cross-chain-tx-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/cross-chain-tx/[id].tsx:18</code> | <code>GET /cross-chain-tx/{id} (page)</code> |
| <code>interface.14.http-fe-page-csv-export-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/csv-export.tsx:16</code> | <code>GET /csv-export (page)</code> |
| <code>interface.14.http-fe-page-deposits-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/deposits/index.tsx:47</code> | <code>GET /deposits (page)</code> |
| <code>interface.14.http-fe-page-dispute-games-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/dispute-games/index.tsx:17</code> | <code>GET /dispute-games (page)</code> |
| <code>interface.14.http-fe-page-ecosystems-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/ecosystems.tsx:17</code> | <code>GET /ecosystems (page)</code> |
| <code>interface.14.http-fe-page-epochs-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/epochs/index.tsx:17</code> | <code>GET /epochs (page)</code> |
| <code>interface.14.http-fe-page-epochs-number-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/epochs/[number].tsx:18</code> | <code>GET /epochs/{number} (page)</code> |
| <code>interface.14.http-fe-page-essential-dapps-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/essential-dapps/[id].tsx:16</code> | <code>GET /essential-dapps/{id} (page)</code> |
| <code>interface.14.http-fe-page-feed-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/feed/index.tsx:17</code> | <code>GET /feed (page)</code> |
| <code>interface.14.http-fe-page-feed-market-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/feed/market/[id].tsx:22</code> | <code>GET /feed/market/{id} (page)</code> |
| <code>interface.14.http-fe-page-feed-update-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/feed/update/[hash].tsx:22</code> | <code>GET /feed/update/{hash} (page)</code> |
| <code>interface.14.http-fe-page-gas-tracker-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/gas-tracker.tsx:17</code> | <code>GET /gas-tracker (page)</code> |
| <code>interface.14.http-fe-page-hot-contracts-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/hot-contracts.tsx:17</code> | <code>GET /hot-contracts (page)</code> |
| <code>interface.14.http-fe-page-internal-txs-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/internal-txs.tsx:25</code> | <code>GET /internal-txs (page)</code> |
| <code>interface.14.http-fe-page-interop-messages-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/interop-messages.tsx:17</code> | <code>GET /interop-messages (page)</code> |
| <code>interface.14.http-fe-page-login-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/login.tsx:16</code> | <code>GET /login (page)</code> |
| <code>interface.14.http-fe-page-mud-worlds-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/mud-worlds.tsx:17</code> | <code>GET /mud-worlds (page)</code> |
| <code>interface.14.http-fe-page-name-services-clusters-name-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/name-services/clusters/[name].tsx:18</code> | <code>GET /name-services/clusters/{name} (page)</code> |
| <code>interface.14.http-fe-page-name-services-domains-name-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/name-services/domains/[name].tsx:18</code> | <code>GET /name-services/domains/{name} (page)</code> |
| <code>interface.14.http-fe-page-name-services-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/name-services/index.tsx:17</code> | <code>GET /name-services (page)</code> |
| <code>interface.14.http-fe-page-op-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/op/[hash].tsx:18</code> | <code>GET /op/{hash} (page)</code> |
| <code>interface.14.http-fe-page-operation-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/operation/[id].tsx:18</code> | <code>GET /operation/{id} (page)</code> |
| <code>interface.14.http-fe-page-operations-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/operations.tsx:17</code> | <code>GET /operations (page)</code> |
| <code>interface.14.http-fe-page-ops-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/ops.tsx:25</code> | <code>GET /ops (page)</code> |
| <code>interface.14.http-fe-page-output-roots-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/output-roots/index.tsx:17</code> | <code>GET /output-roots (page)</code> |
| <code>interface.14.http-fe-page-paxeer-x-account-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/paxeer-x/account/[hash].tsx:21</code> | <code>GET /paxeer-x/account/{hash} (page)</code> |
| <code>interface.14.http-fe-page-paxeer-x-anchors-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/paxeer-x/anchors.tsx:17</code> | <code>GET /paxeer-x/anchors (page)</code> |
| <code>interface.14.http-fe-page-paxeer-x-receipts-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/paxeer-x/receipts/[id].tsx:21</code> | <code>GET /paxeer-x/receipts/{id} (page)</code> |
| <code>interface.14.http-fe-page-paxeer-x-receipts-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/paxeer-x/receipts.tsx:17</code> | <code>GET /paxeer-x/receipts (page)</code> |
| <code>interface.14.http-fe-page-pools-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/pools/[hash].tsx:18</code> | <code>GET /pools/{hash} (page)</code> |
| <code>interface.14.http-fe-page-pools-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/pools/index.tsx:17</code> | <code>GET /pools (page)</code> |
| <code>interface.14.http-fe-page-public-tags-submit-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/public-tags/submit.tsx:16</code> | <code>GET /public-tags/submit (page)</code> |
| <code>interface.14.http-fe-page-root-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/index.tsx:28</code> | <code>GET / (page)</code> |
| <code>interface.14.http-fe-page-search-results-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/search-results.tsx:36</code> | <code>GET /search-results (page)</code> |
| <code>interface.14.http-fe-page-sprite-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/sprite.tsx:16</code> | <code>GET /sprite (page)</code> |
| <code>interface.14.http-fe-page-stats-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/stats/[id].tsx:35</code> | <code>GET /stats/{id} (page)</code> |
| <code>interface.14.http-fe-page-stats-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/stats/index.tsx:18</code> | <code>GET /stats (page)</code> |
| <code>interface.14.http-fe-page-token-hash-instance-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/token/[hash]/instance/[id].tsx:25</code> | <code>GET /token/{hash}/instance/{id} (page)</code> |
| <code>interface.14.http-fe-page-token-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/token/[hash]/index.tsx:25</code> | <code>GET /token/{hash} (page)</code> |
| <code>interface.14.http-fe-page-token-transfers-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/token-transfers.tsx:25</code> | <code>GET /token-transfers (page)</code> |
| <code>interface.14.http-fe-page-tokens-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/tokens.tsx:25</code> | <code>GET /tokens (page)</code> |
| <code>interface.14.http-fe-page-tx-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/tx/[hash].tsx:20</code> | <code>GET /tx/{hash} (page)</code> |
| <code>interface.14.http-fe-page-txn-withdrawals-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/txn-withdrawals/index.tsx:26</code> | <code>GET /txn-withdrawals (page)</code> |
| <code>interface.14.http-fe-page-txs-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/txs/index.tsx:34</code> | <code>GET /txs (page)</code> |
| <code>interface.14.http-fe-page-txs-kettle-hash-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/txs/kettle/[hash].tsx:18</code> | <code>GET /txs/kettle/{hash} (page)</code> |
| <code>interface.14.http-fe-page-uptime-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/uptime.tsx:16</code> | <code>GET /uptime (page)</code> |
| <code>interface.14.http-fe-page-validators-id-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/validators/[id].tsx:28</code> | <code>GET /validators/{id} (page)</code> |
| <code>interface.14.http-fe-page-validators-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/validators/index.tsx:35</code> | <code>GET /validators (page)</code> |
| <code>interface.14.http-fe-page-verified-contracts-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/verified-contracts.tsx:25</code> | <code>GET /verified-contracts (page)</code> |
| <code>interface.14.http-fe-page-visualize-sol2uml-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/visualize/sol2uml.tsx:16</code> | <code>GET /visualize/sol2uml (page)</code> |
| <code>interface.14.http-fe-page-withdrawals-j75</code> | <code>http_route</code> | <code>stable</code> | <code>apps/explorer/pages/withdrawals/index.tsx:47</code> | <code>GET /withdrawals (page)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-fe-page-404-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-account-api-key-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-account-custom-abi-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-account-merits-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-account-tag-address-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-account-verified-addresses-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-account-watchlist-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-accounts-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-accounts-label-slug-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-address-hash-contract-verification-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-address-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-advanced-filter-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-api-docs-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-apps-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-apps-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-auth-profile-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-batches-celestia-height-commitment-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-batches-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-batches-number-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-blobs-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-block-countdown-height-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-block-countdown-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-block-height-or-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-blocks-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-cc-tx-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-accounts-label-slug-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-advanced-filter-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-block-countdown-height-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-block-countdown-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-block-height-or-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-csv-export-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-op-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-token-hash-instance-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-token-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-tx-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chain-chain-slug-visualize-sol2uml-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-chakra-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-contract-verification-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-cross-chain-tx-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-csv-export-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-deposits-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-dispute-games-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-ecosystems-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-epochs-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-epochs-number-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-essential-dapps-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-feed-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-feed-market-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-feed-update-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-gas-tracker-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-hot-contracts-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-internal-txs-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-interop-messages-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-login-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-mud-worlds-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-name-services-clusters-name-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-name-services-domains-name-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-name-services-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-op-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-operation-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-operations-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-ops-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-output-roots-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-paxeer-x-account-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-paxeer-x-anchors-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-paxeer-x-receipts-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-paxeer-x-receipts-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-pools-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-pools-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-public-tags-submit-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-root-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-search-results-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-sprite-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-stats-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-stats-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-token-hash-instance-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-token-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-token-transfers-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-tokens-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-tx-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-txn-withdrawals-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-txs-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-txs-kettle-hash-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-uptime-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-validators-id-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-validators-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-verified-contracts-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-visualize-sol2uml-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |
| <code>interface.14.http-fe-page-withdrawals-j75</code> | notes | Next.js pages-router file route of the explorer frontend; default export at the cited line; callers are browser navigation and in-app links, not enumerated |

## paxeer-x-pages

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.243.1</code> | req | 14243.1 |
| <code>acceptance.14.243.1</code> | scenario | Given the capabilities mocks, when their keys are listed, then none of them has custody or anchor |
| <code>acceptance.14.243.1</code> | test | apps/explorer/mocks/paxeerX/capabilities.spec.ts::capabilities_mocks_have_no_retired_keys (planned) |
| <code>acceptance.14.243.1</code> | qualification | UNRUN |
| <code>req.14243</code> | ac_1 | Every capabilities mock in apps/explorer/mocks/paxeerX/capabilities.ts:5 carries no custody and no anchor key. |
| <code>acceptance.14.243.2</code> | req | 14243.2 |
| <code>acceptance.14.243.2</code> | scenario | Given a capabilities answer without price_feed, when useNavItems renders, then no feed nav item is present |
| <code>acceptance.14.243.2</code> | test | apps/explorer/lib/hooks/useNavItems.spec.tsx::nav_hides_feed_without_price_feed_capability (planned) |
| <code>acceptance.14.243.2</code> | qualification | UNRUN |
| <code>req.14243</code> | ac_2 | useNavItems (apps/explorer/lib/hooks/useNavItems.tsx:35-38) omits the feed entry when the capabilities answer has no price_feed. |
| <code>acceptance.14.280.1</code> | req | 14280.1 |
| <code>acceptance.14.280.1</code> | scenario | Given the rungs module, when RUNG_ORDER is mapped to rung names, then it equals ['submitted', 'receipted', 'committed'] |
| <code>acceptance.14.280.1</code> | test | apps/explorer/ui/shared/statusLadder/rungs.spec.ts::rung_order_is_three_rungs (planned) |
| <code>acceptance.14.280.1</code> | qualification | UNRUN |
| <code>req.14280</code> | ac_1 | RUNG_ORDER (apps/explorer/ui/shared/statusLadder/rungs.ts:46) lists submitted, receipted and committed in that order. |
| <code>acceptance.14.281.1</code> | req | 14281.1 |
| <code>acceptance.14.281.1</code> | scenario | Given rung committed, when StatusLadderBadge renders, then its text is Committed |
| <code>acceptance.14.281.1</code> | test | apps/explorer/ui/shared/statusLadder/StatusLadderBadge.spec.tsx::badge_labels_committed_rung (planned) |
| <code>acceptance.14.281.1</code> | qualification | UNRUN |
| <code>req.14281</code> | ac_1 | StatusLadderBadge (apps/explorer/ui/shared/statusLadder/StatusLadderBadge.tsx:1) labels the committed rung Committed. |
| <code>acceptance.14.281.2</code> | req | 14281.2 |
| <code>acceptance.14.281.2</code> | scenario | Given a status answer {rung: 'receipted', block_number: null, committed_height: 10}, when TxStatusLadderBadge renders for the hash, then its text is Receipted |
| <code>acceptance.14.281.2</code> | test | apps/explorer/ui/shared/statusLadder/TxStatusLadderBadge.spec.tsx::tx_badge_renders_receipted_answer (planned) |
| <code>acceptance.14.281.2</code> | qualification | UNRUN |
| <code>req.14281</code> | ac_2 | TxStatusLadderBadge shows Receipted for a status answer with rung receipted. |
| <code>acceptance.14.282.1</code> | req | 14282.1 |
| <code>acceptance.14.282.1</code> | scenario | Given GENERAL_API_PAXEER_X_LISTS_RESOURCES, when the anchors, receipts and receipt paths are read, then each starts with /api/v2/paxeer-x/archive/ |
| <code>acceptance.14.282.1</code> | test | apps/explorer/lib/api/services/general/paxeerXLists.spec.ts::kernel_resources_use_archive_paths (planned) |
| <code>acceptance.14.282.1</code> | qualification | UNRUN |
| <code>req.14282</code> | ac_1 | The three kernel resources at apps/explorer/lib/api/services/general/paxeerXLists.ts:9-22 point at /api/v2/paxeer-x/archive paths. |
| <code>acceptance.14.283.1</code> | req | 14283.1 |
| <code>acceptance.14.283.1</code> | scenario | Given the archive anchors route answering one item, when PaxeerXAnchors renders, then the title reads Anchor checkpoints (archive) and data-archive-notice is present |
| <code>acceptance.14.283.1</code> | test | apps/explorer/ui/pages/PaxeerXAnchors.spec.tsx::anchors_page_shows_archive_notice (planned) |
| <code>acceptance.14.283.1</code> | qualification | UNRUN |
| <code>req.14283</code> | ac_1 | The anchors page (apps/explorer/ui/pages/PaxeerXAnchors.tsx:78) renders the archive title and the frozen notice. |
| <code>acceptance.14.284.1</code> | req | 14284.1 |
| <code>acceptance.14.284.1</code> | scenario | Given the archive receipts route answering one item, when PaxeerXReceipts renders, then the title reads Kernel receipts (archive) and no status ladder badge is rendered |
| <code>acceptance.14.284.1</code> | test | apps/explorer/ui/pages/PaxeerXReceipts.spec.tsx::receipts_page_shows_archive_rows_without_rung (planned) |
| <code>acceptance.14.284.1</code> | qualification | UNRUN |
| <code>req.14284</code> | ac_1 | The receipts page (apps/explorer/ui/pages/PaxeerXReceipts.tsx:78) shows the archive title and rows without a rung badge. |
| <code>acceptance.14.285.1</code> | req | 14285.1 |
| <code>acceptance.14.285.1</code> | scenario | Given the archive receipt route answering verification_status batch_included, when PaxeerXReceipt renders, then the field shows the batch-included label, data-archive-notice is present and data-settlement-status is absent |
| <code>acceptance.14.285.1</code> | test | apps/explorer/ui/pages/PaxeerXReceipt.spec.tsx::receipt_page_shows_frozen_verification_status (planned) |
| <code>acceptance.14.285.1</code> | qualification | UNRUN |
| <code>req.14285</code> | ac_1 | The receipt page (apps/explorer/ui/pages/PaxeerXReceipt.tsx:69) renders verification_status and the archive notice and no settlement status. |
| <code>acceptance.14.286.1</code> | req | 14286.1 |
| <code>acceptance.14.286.1</code> | scenario | Given the paxeerXLists feature enabled, when useNavItems renders, then one item reads Kernel archive with children routed to /paxeer-x/anchors and /paxeer-x/receipts |
| <code>acceptance.14.286.1</code> | test | apps/explorer/lib/hooks/useNavItemsArchive.spec.tsx::nav_groups_kernel_pages_under_archive (planned) |
| <code>acceptance.14.286.1</code> | qualification | UNRUN |
| <code>req.14286</code> | ac_1 | useNavItems (apps/explorer/lib/hooks/useNavItems.tsx:373) groups the anchors and receipts pages under Kernel archive. |
| <code>acceptance.14.287.1</code> | req | 14287.1 |
| <code>acceptance.14.287.1</code> | scenario | Given an archive anchor item with batch_number 1042, when PaxeerXAnchorsTableItem renders, then the batch number and the checkpoint id link are shown |
| <code>acceptance.14.287.1</code> | test | apps/explorer/ui/paxeerX/anchors/PaxeerXAnchorsTableItem.spec.tsx::anchors_table_item_renders_archive_row (planned) |
| <code>acceptance.14.287.1</code> | qualification | UNRUN |
| <code>req.14287</code> | ac_1 | PaxeerXAnchorsTableItem (apps/explorer/ui/paxeerX/anchors/PaxeerXAnchorsTableItem.tsx:1) renders an archive anchor row with its batch number and checkpoint id. |
| <code>acceptance.14.287.2</code> | req | 14287.2 |
| <code>acceptance.14.287.2</code> | scenario | Given an archive receipt item, when PaxeerXReceiptsTableItem renders, then the id and block number are shown and no status ladder badge is rendered |
| <code>acceptance.14.287.2</code> | test | apps/explorer/ui/paxeerX/receipts/PaxeerXReceiptsTableItem.spec.tsx::receipts_table_item_has_no_rung_badge (planned) |
| <code>acceptance.14.287.2</code> | qualification | UNRUN |
| <code>req.14287</code> | ac_2 | PaxeerXReceiptsTableItem (apps/explorer/ui/paxeerX/receipts/PaxeerXReceiptsTableItem.tsx:1) renders an archive receipt row with no status ladder badge. |
| <code>acceptance.14.288.1</code> | req | 14288.1 |
| <code>acceptance.14.288.1</code> | scenario | Given the account utils, when IDENTITY_ORDER is read, then it equals ['evm', 'pax', 'did'] |
| <code>acceptance.14.288.1</code> | test | apps/explorer/ui/paxeerX/account/utils.spec.ts::identity_order_has_three_kinds (planned) |
| <code>acceptance.14.288.1</code> | qualification | UNRUN |
| <code>req.14288</code> | ac_1 | IDENTITY_ORDER (apps/explorer/ui/paxeerX/account/utils.ts:5) lists evm, pax and did only. |
| <code>acceptance.14.289.1</code> | req | 14289.1 |
| <code>acceptance.14.289.1</code> | scenario | Given identities {evm, pax, did}, when IdentityList renders, then three rows render and none is labelled Kernel account |
| <code>acceptance.14.289.1</code> | test | apps/explorer/ui/paxeerX/account/IdentityList.spec.tsx::identity_list_renders_evm_pax_did (planned) |
| <code>acceptance.14.289.1</code> | qualification | UNRUN |
| <code>req.14289</code> | ac_1 | IdentityList (apps/explorer/ui/paxeerX/account/IdentityList.tsx:1) renders the evm, pax and did rows and no kernel account row. |
| <code>acceptance.14.290.1</code> | req | 14290.1 |
| <code>acceptance.14.290.1</code> | scenario | Given a balance {asset, total: '1200'}, when AssetListItem renders, then the total shows and no element carries data-parts-of |
| <code>acceptance.14.290.1</code> | test | apps/explorer/ui/paxeerX/account/AssetListItem.spec.tsx::asset_item_renders_total_without_parts (planned) |
| <code>acceptance.14.290.1</code> | qualification | UNRUN |
| <code>req.14290</code> | ac_1 | AssetListItem (apps/explorer/ui/paxeerX/account/AssetListItem.tsx:70) renders the total and no data-parts-of row. |
| <code>acceptance.14.291.1</code> | req | 14291.1 |
| <code>acceptance.14.291.1</code> | scenario | Given an activity item of a chain transfer, when ActivityListItem renders, then neither Chain nor Kernel appears as a side label |
| <code>acceptance.14.291.1</code> | test | apps/explorer/ui/paxeerX/account/ActivityListItem.spec.tsx::activity_item_renders_without_side (planned) |
| <code>acceptance.14.291.1</code> | qualification | UNRUN |
| <code>req.14291</code> | ac_1 | ActivityListItem (apps/explorer/ui/paxeerX/account/ActivityListItem.tsx:36) renders no Chain or Kernel side label. |
| <code>acceptance.14.292.1</code> | req | 14292.1 |
| <code>acceptance.14.292.1</code> | scenario | Given an address with a unified account, when AddressDetails renders, then an element with data-unified-account-link points at /paxeer-x/account/&lt;hash&gt; |
| <code>acceptance.14.292.1</code> | test | apps/explorer/ui/address/AddressDetails.spec.tsx::address_links_unified_account_page (planned) |
| <code>acceptance.14.292.1</code> | qualification | UNRUN |
| <code>req.14292</code> | ac_1 | AddressDetails (apps/explorer/ui/address/AddressDetails.tsx:269) links to /paxeer-x/account/[hash] through data-unified-account-link. |
| <code>acceptance.14.293.1</code> | req | 14293.1 |
| <code>acceptance.14.293.1</code> | scenario | Given the capabilities mock without custody and the unified account mock, when PaxeerXAccount renders, then no data-capability-notice="custody" element and no Custody or Anchoring row is present |
| <code>acceptance.14.293.1</code> | test | apps/explorer/ui/pages/PaxeerXAccount.spec.tsx::account_page_has_no_custody_notice (planned) |
| <code>acceptance.14.293.1</code> | qualification | UNRUN |
| <code>req.14293</code> | ac_1 | PaxeerXAccount (apps/explorer/ui/pages/PaxeerXAccount.tsx:249) renders no custody notice and no custody or anchoring capability row. |
| <code>acceptance.14.294.1</code> | req | 14294.1 |
| <code>acceptance.14.294.1</code> | scenario | Given GENERAL_API_PAXEER_X_LISTS_RESOURCES, when paxeer_x_tx_signature.path is read, then it equals /api/v2/transactions/:hash/signature |
| <code>acceptance.14.294.1</code> | test | apps/explorer/lib/api/services/general/paxeerXSignature.spec.ts::signature_resource_path_is_signature_route (planned) |
| <code>acceptance.14.294.1</code> | qualification | UNRUN |
| <code>req.14294</code> | ac_1 | The paxeer_x_tx_signature resource points at /api/v2/transactions/:hash/signature. |
| <code>acceptance.14.295.1</code> | req | 14295.1 |
| <code>acceptance.14.295.1</code> | scenario | Given the signature route answering a receipted body, when TxDetailsSequencerSignature renders, then the key id and the signature hex are shown |
| <code>acceptance.14.295.1</code> | test | apps/explorer/ui/tx/details/TxDetailsSequencerSignature.spec.tsx::signature_row_renders_key_and_signature (planned) |
| <code>acceptance.14.295.1</code> | qualification | UNRUN |
| <code>req.14295</code> | ac_1 | TxDetailsSequencerSignature renders the key id and signature for a verified answer. |
| <code>acceptance.14.295.2</code> | req | 14295.2 |
| <code>acceptance.14.295.2</code> | scenario | Given the signature route answering 404, when TxDetailsSequencerSignature renders, then it renders no element |
| <code>acceptance.14.295.2</code> | test | apps/explorer/ui/tx/details/TxDetailsSequencerSignature.spec.tsx::signature_row_hidden_on_404 (planned) |
| <code>acceptance.14.295.2</code> | qualification | UNRUN |
| <code>req.14295</code> | ac_2 | TxDetailsSequencerSignature renders nothing when the signature route answers 404. |
| <code>acceptance.14.296.1</code> | req | 14296.1 |
| <code>acceptance.14.296.1</code> | scenario | Given a transaction and a signature route answering a receipted body, when TxInfo renders, then the sequencer signature row is present |
| <code>acceptance.14.296.1</code> | test | apps/explorer/ui/tx/details/TxInfo.spec.tsx::tx_info_shows_sequencer_signature_row (planned) |
| <code>acceptance.14.296.1</code> | qualification | UNRUN |
| <code>req.14296</code> | ac_1 | TxInfo (apps/explorer/ui/tx/details/TxInfo.tsx:238) renders the sequencer signature row for a transaction whose signature route answers. |
| <code>capability.14.21.3</code> | title | Drop custody and anchor from the frontend capabilities type and mocks |
| <code>capability.14.21.3</code> | reqs | 14243.1<br>14243.2 |
| <code>capability.14.31.1</code> | title | Move the frontend status ladder to submitted, receipted and committed |
| <code>capability.14.31.1</code> | reqs | 14280.1 |
| <code>capability.14.31.2</code> | title | Render the three-rung badge for a transaction status answer |
| <code>capability.14.31.2</code> | reqs | 14281.1<br>14281.2 |
| <code>capability.14.32.1</code> | title | Point the anchors and receipts resources at the archive routes |
| <code>capability.14.32.1</code> | reqs | 14282.1 |
| <code>capability.14.32.2</code> | title | Show the anchors page as a frozen archive |
| <code>capability.14.32.2</code> | reqs | 14283.1 |
| <code>capability.14.32.3</code> | title | Show the kernel receipts page as a frozen archive without a settlement rung |
| <code>capability.14.32.3</code> | reqs | 14284.1 |
| <code>capability.14.32.4</code> | title | Show one archived kernel receipt with its frozen verification status |
| <code>capability.14.32.4</code> | reqs | 14285.1 |
| <code>capability.14.32.5</code> | title | Label the kernel pages Kernel archive in the navigation |
| <code>capability.14.32.5</code> | reqs | 14286.1 |
| <code>capability.14.32.6</code> | title | Move the kernel list item specs to the archive item shapes |
| <code>capability.14.32.6</code> | reqs | 14287.1<br>14287.2 |
| <code>capability.14.33.1</code> | title | Align the frontend unified account types and mocks with the backend answer |
| <code>capability.14.33.1</code> | reqs | 14288.1 |
| <code>capability.14.33.2</code> | title | Render three identities in IdentityList |
| <code>capability.14.33.2</code> | reqs | 14289.1 |
| <code>capability.14.33.3</code> | title | Render one total per asset in AssetListItem |
| <code>capability.14.33.3</code> | reqs | 14290.1 |
| <code>capability.14.33.4</code> | title | Render activity rows without a chain or kernel side |
| <code>capability.14.33.4</code> | reqs | 14291.1 |
| <code>capability.14.33.5</code> | title | Link an address to its unified account page without kernel wording |
| <code>capability.14.33.5</code> | reqs | 14292.1 |
| <code>capability.14.33.6</code> | title | Drop the custody and anchoring rows from the unified account page |
| <code>capability.14.33.6</code> | reqs | 14293.1 |
| <code>capability.14.34.1</code> | title | Add the sequencer signature type and API resource |
| <code>capability.14.34.1</code> | reqs | 14294.1 |
| <code>capability.14.34.2</code> | title | Show the verified sequencer signature on the transaction page |
| <code>capability.14.34.2</code> | reqs | 14295.1<br>14295.2 |
| <code>capability.14.34.3</code> | title | Place the sequencer signature row in the transaction details |
| <code>capability.14.34.3</code> | reqs | 14296.1 |
| <code>resolution.17.948</code> | title | decision.1423 binds: Keep the kernel frontend routes and repoint them at the archive |
| <code>resolution.17.948</code> | applies_to | decision.1423<br>task.14.32.1<br>task.14.32.2<br>task.14.32.3<br>task.14.32.4<br>task.14.32.5 |
| <code>resolution.17.948</code> | source | decision.1423 in fragments/J54/decision.kvx; tasks from affects |
