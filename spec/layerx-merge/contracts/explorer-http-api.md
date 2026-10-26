# Explorer http api contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## account-api-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-account-api-key-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:107</code> | <code>DELETE /account/api_key/:id -&gt; Account.ApiKeyController.resources</code> |
| <code>interface.14.http-get-account-api-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:107</code> | <code>GET /account/api_key -&gt; Account.ApiKeyController.resources</code> |
| <code>interface.14.http-patch-put-account-api-key-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:107</code> | <code>PATCH&#124;PUT /account/api_key/:id -&gt; Account.ApiKeyController.resources</code> |
| <code>interface.14.http-post-account-api-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:107</code> | <code>POST /account/api_key -&gt; Account.ApiKeyController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-account-api-key-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-get-account-api-key</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-patch-put-account-api-key-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-post-account-api-key</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-api-key-edit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-api-key-id-edit</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:107</code> | <code>GET /account/api_key/:id/edit -&gt; Account.ApiKeyController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-api-key-id-edit</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-api-key-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-api-key-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:107</code> | <code>GET /account/api_key/new -&gt; Account.ApiKeyController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-api-key-new</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-custom-abi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-account-custom-abi-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:112</code> | <code>DELETE /account/custom_abi/:id -&gt; Account.CustomABIController.resources</code> |
| <code>interface.14.http-get-account-custom-abi</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:112</code> | <code>GET /account/custom_abi -&gt; Account.CustomABIController.resources</code> |
| <code>interface.14.http-patch-put-account-custom-abi-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:112</code> | <code>PATCH&#124;PUT /account/custom_abi/:id -&gt; Account.CustomABIController.resources</code> |
| <code>interface.14.http-post-account-custom-abi</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:112</code> | <code>POST /account/custom_abi -&gt; Account.CustomABIController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-account-custom-abi-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-get-account-custom-abi</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-patch-put-account-custom-abi-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-post-account-custom-abi</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-custom-abi-edit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-custom-abi-id-edit</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:112</code> | <code>GET /account/custom_abi/:id/edit -&gt; Account.CustomABIController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-custom-abi-id-edit</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-custom-abi-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-custom-abi-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:112</code> | <code>GET /account/custom_abi/new -&gt; Account.CustomABIController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-custom-abi-new</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-tag-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-account-tag-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:86</code> | <code>DELETE /account/tag_address/:id -&gt; Account.TagAddressController.resources</code> |
| <code>interface.14.http-get-account-tag-address</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:86</code> | <code>GET /account/tag_address -&gt; Account.TagAddressController.resources</code> |
| <code>interface.14.http-post-account-tag-address</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:86</code> | <code>POST /account/tag_address -&gt; Account.TagAddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-account-tag-address-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-get-account-tag-address</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-post-account-tag-address</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-tag-address-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-tag-address-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:86</code> | <code>GET /account/tag_address/new -&gt; Account.TagAddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-tag-address-new</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-tag-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-account-tag-transaction-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:91</code> | <code>DELETE /account/tag_transaction/:id -&gt; Account.TagTransactionController.resources</code> |
| <code>interface.14.http-get-account-tag-transaction</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:91</code> | <code>GET /account/tag_transaction -&gt; Account.TagTransactionController.resources</code> |
| <code>interface.14.http-post-account-tag-transaction</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:91</code> | <code>POST /account/tag_transaction -&gt; Account.TagTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-account-tag-transaction-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-get-account-tag-transaction</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-post-account-tag-transaction</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-tag-transaction-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-tag-transaction-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:91</code> | <code>GET /account/tag_transaction/new -&gt; Account.TagTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-tag-transaction-new</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-watchlist

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-watchlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:96</code> | <code>GET /account/watchlist -&gt; Account.WatchlistController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-watchlist</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-watchlist-address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-account-watchlist-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:102</code> | <code>DELETE /account/watchlist_address/:id -&gt; Account.WatchlistAddressController.resources</code> |
| <code>interface.14.http-patch-put-account-watchlist-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:102</code> | <code>PATCH&#124;PUT /account/watchlist_address/:id -&gt; Account.WatchlistAddressController.resources</code> |
| <code>interface.14.http-post-account-watchlist-address</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:102</code> | <code>POST /account/watchlist_address -&gt; Account.WatchlistAddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-account-watchlist-address-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-patch-put-account-watchlist-address-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |
| <code>interface.14.http-post-account-watchlist-address</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-watchlist-address-edit

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-watchlist-address-id-edit</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:102</code> | <code>GET /account/watchlist_address/:id/edit -&gt; Account.WatchlistAddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-watchlist-address-id-edit</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## account-watchlist-address-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-account-watchlist-address-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:102</code> | <code>GET /account/watchlist_address/new -&gt; Account.WatchlistAddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-account-watchlist-address-new</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:35; no in-repo caller found by path search |

## accounts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-accounts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:115</code> | <code>GET /accounts -&gt; AddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-accounts</code> | notes | no in-repo caller found by path search |

## address

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:119</code> | <code>GET /address/:id -&gt; AddressController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-id</code> | notes | no in-repo caller found by path search |

## address-coin-balances

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-coin-balances</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:250</code> | <code>GET /address/:address_id/coin-balances -&gt; AddressCoinBalanceController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-coin-balances</code> | notes | no in-repo caller found by path search |

## address-coin-balances-by-day

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-coin-balances-by-day</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:257</code> | <code>GET /address/:address_id/coin-balances/by-day -&gt; AddressCoinBalanceByDayController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-coin-balances-by-day</code> | notes | no in-repo caller found by path search |

## address-contract-verifications-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-contract-verifications-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:150</code> | <code>GET /address/:address_id/contract_verifications/new -&gt; AddressContractVerificationController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-contract-verifications-new</code> | notes | no in-repo caller found by path search |

## address-contracts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:136</code> | <code>GET /address/:address_id/contracts -&gt; AddressContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-contracts</code> | notes | no in-repo caller found by path search |

## address-counters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:430</code> | <code>GET /address-counters -&gt; AddressController.address_counters</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-counters</code> | notes | no in-repo caller found by path search |

## address-internal-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-internal-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:122</code> | <code>GET /address/:address_id/internal-transactions -&gt; AddressInternalTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-internal-transactions</code> | notes | no in-repo caller found by path search |

## address-logs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-logs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:143</code> | <code>GET /address/:address_id/logs -&gt; AddressLogsController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-logs</code> | notes | no in-repo caller found by path search |

## address-read-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-read-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:192</code> | <code>GET /address/:address_id/read-contract -&gt; AddressReadContractController.resources</code> |
| <code>interface.14.http-get-address-address-id-read-contract-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:192</code> | <code>GET /address/:address_id/read-contract/:id -&gt; AddressReadContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-read-contract</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-address-address-id-read-contract-id</code> | notes | no in-repo caller found by path search |

## address-read-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-read-proxy</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:199</code> | <code>GET /address/:address_id/read-proxy -&gt; AddressReadProxyController.resources</code> |
| <code>interface.14.http-get-address-address-id-read-proxy-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:199</code> | <code>GET /address/:address_id/read-proxy/:id -&gt; AddressReadProxyController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-read-proxy</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-address-address-id-read-proxy-id</code> | notes | no in-repo caller found by path search |

## address-token-balances

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-token-balances</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:243</code> | <code>GET /address/:address_id/token-balances -&gt; AddressTokenBalanceController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-token-balances</code> | notes | no in-repo caller found by path search |

## address-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:220</code> | <code>GET /address/:address_id/token-transfers -&gt; AddressTokenTransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-token-transfers</code> | notes | no in-repo caller found by path search |

## address-tokens

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-tokens</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:234</code> | <code>GET /address/:address_id/tokens -&gt; AddressTokenController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-tokens</code> | notes | no in-repo caller found by path search |

## address-tokens-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-tokens-address-token-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:235</code> | <code>GET /address/:address_id/tokens/:address_token_id/token-transfers -&gt; AddressTokenTransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-tokens-address-token-id-token-transfers</code> | notes | no in-repo caller found by path search |

## address-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:120</code> | <code>GET /address/:address_id/transactions -&gt; AddressTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-transactions</code> | notes | no in-repo caller found by path search |

## address-validations

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-validations</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:129</code> | <code>GET /address/:address_id/validations -&gt; AddressValidationController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-validations</code> | notes | no in-repo caller found by path search |

## address-verify-via-flattened-code-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-flattened-code-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:157</code> | <code>GET /address/:address_id/verify-via-flattened-code/new -&gt; AddressContractVerificationViaFlattenedCodeController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-flattened-code-new</code> | notes | no in-repo caller found by path search |

## address-verify-via-metadata-json-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-metadata-json-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:164</code> | <code>GET /address/:address_id/verify-via-metadata-json/new -&gt; AddressContractVerificationViaJsonController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-metadata-json-new</code> | notes | no in-repo caller found by path search |

## address-verify-via-multi-part-files-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-multi-part-files-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:178</code> | <code>GET /address/:address_id/verify-via-multi-part-files/new -&gt; AddressContractVerificationViaMultiPartFilesController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-multi-part-files-new</code> | notes | no in-repo caller found by path search |

## address-verify-via-standard-json-input-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-standard-json-input-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:171</code> | <code>GET /address/:address_id/verify-via-standard-json-input/new -&gt; AddressContractVerificationViaStandardJsonInputController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-standard-json-input-new</code> | notes | no in-repo caller found by path search |

## address-verify-vyper-contract-new

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-vyper-contract-new</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:185</code> | <code>GET /address/:address_id/verify-vyper-contract/new -&gt; AddressContractVerificationVyperController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-vyper-contract-new</code> | notes | no in-repo caller found by path search |

## address-withdrawals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:227</code> | <code>GET /address/:address_id/withdrawals -&gt; AddressWithdrawalController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-withdrawals</code> | notes | no in-repo caller found by path search |

## address-write-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-write-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:206</code> | <code>GET /address/:address_id/write-contract -&gt; AddressWriteContractController.resources</code> |
| <code>interface.14.http-get-address-address-id-write-contract-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:206</code> | <code>GET /address/:address_id/write-contract/:id -&gt; AddressWriteContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-write-contract</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-address-address-id-write-contract-id</code> | notes | no in-repo caller found by path search |

## address-write-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-address-address-id-write-proxy</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:213</code> | <code>GET /address/:address_id/write-proxy -&gt; AddressWriteProxyController.resources</code> |
| <code>interface.14.http-get-address-address-id-write-proxy-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:213</code> | <code>GET /address/:address_id/write-proxy/:id -&gt; AddressWriteProxyController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-address-address-id-write-proxy</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-address-address-id-write-proxy-id</code> | notes | no in-repo caller found by path search |

## admin

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-admin</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex:54</code> | <code>GET /admin -&gt; DashboardController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-admin</code> | notes | no in-repo caller found by path search |

## admin-login

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-admin-login</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex:47</code> | <code>GET /admin/login -&gt; SessionController.new</code> |
| <code>interface.14.http-post-admin-login</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex:48</code> | <code>POST /admin/login -&gt; SessionController.create</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-admin-login</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-post-admin-login</code> | notes | no in-repo caller found by path search |

## admin-setup

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-admin-setup</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex:40</code> | <code>GET /admin/setup -&gt; SetupController.configure</code> |
| <code>interface.14.http-post-admin-setup</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex:41</code> | <code>POST /admin/setup -&gt; SetupController.configure_admin</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-admin-setup</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-post-admin-setup</code> | notes | no in-repo caller found by path search |

## admin-tasks-create-contract-methods

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-admin-tasks-create-contract-methods</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex:57</code> | <code>GET /admin/tasks/create_contract_methods -&gt; TaskController.create_contract_methods</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-admin-tasks-create-contract-methods</code> | notes | no in-repo caller found by path search |

## api-account-v2

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-api-account-v2-user-api-keys-api-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:144</code> | <code>DELETE /api/account/v2/user/api_keys/:api_key -&gt; UserController.delete_api_key</code> |
| <code>interface.14.http-delete-api-account-v2-user-custom-abis-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:149</code> | <code>DELETE /api/account/v2/user/custom_abis/:id -&gt; UserController.delete_custom_abi</code> |
| <code>interface.14.http-delete-api-account-v2-user-tags-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:156</code> | <code>DELETE /api/account/v2/user/tags/address/:id -&gt; UserController.delete_tag_address</code> |
| <code>interface.14.http-delete-api-account-v2-user-tags-transaction-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:162</code> | <code>DELETE /api/account/v2/user/tags/transaction/:id -&gt; UserController.delete_tag_transaction</code> |
| <code>interface.14.http-delete-api-account-v2-user-watchlist-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:139</code> | <code>DELETE /api/account/v2/user/watchlist/:id -&gt; UserController.delete_watchlist</code> |
| <code>interface.14.http-get-api-account-v2-authenticate</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:121</code> | <code>GET /api/account/v2/authenticate -&gt; AuthenticateController.authenticate_get</code> |
| <code>interface.14.http-get-api-account-v2-authenticate-via-dynamic</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:183</code> | <code>GET /api/account/v2/authenticate_via_dynamic -&gt; AuthenticateController.authenticate_via_dynamic</code> |
| <code>interface.14.http-get-api-account-v2-email-resend</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:131</code> | <code>GET /api/account/v2/email/resend -&gt; EmailController.resend_email</code> |
| <code>interface.14.http-get-api-account-v2-get-csrf</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:124</code> | <code>GET /api/account/v2/get_csrf -&gt; UserController.get_csrf</code> |
| <code>interface.14.http-get-api-account-v2-siwe-message</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:186</code> | <code>GET /api/account/v2/siwe_message -&gt; AuthenticateController.siwe_message</code> |
| <code>interface.14.http-get-api-account-v2-tags-address-address-hash</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:173</code> | <code>GET /api/account/v2/tags/address/:address_hash -&gt; TagsController.tags_address</code> |
| <code>interface.14.http-get-api-account-v2-tags-transaction-transaction-hash</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:175</code> | <code>GET /api/account/v2/tags/transaction/:transaction_hash -&gt; TagsController.tags_transaction</code> |
| <code>interface.14.http-get-api-account-v2-user-api-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:143</code> | <code>GET /api/account/v2/user/api_keys -&gt; UserController.api_keys</code> |
| <code>interface.14.http-get-api-account-v2-user-custom-abis</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:148</code> | <code>GET /api/account/v2/user/custom_abis -&gt; UserController.custom_abis</code> |
| <code>interface.14.http-get-api-account-v2-user-info</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:136</code> | <code>GET /api/account/v2/user/info -&gt; UserController.info</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-address</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:154</code> | <code>GET /api/account/v2/user/tags/address/ -&gt; UserController.tags_address</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:155</code> | <code>GET /api/account/v2/user/tags/address/:id -&gt; UserController.tags_address</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-transaction</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:160</code> | <code>GET /api/account/v2/user/tags/transaction/ -&gt; UserController.tags_transaction</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-transaction-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:161</code> | <code>GET /api/account/v2/user/tags/transaction/:id -&gt; UserController.tags_transaction</code> |
| <code>interface.14.http-get-api-account-v2-user-watchlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:138</code> | <code>GET /api/account/v2/user/watchlist -&gt; UserController.watchlist</code> |
| <code>interface.14.http-post-api-account-v2-address-link</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:127</code> | <code>POST /api/account/v2/address/link -&gt; AddressController.link_address</code> |
| <code>interface.14.http-post-api-account-v2-authenticate</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:122</code> | <code>POST /api/account/v2/authenticate -&gt; AuthenticateController.authenticate_post</code> |
| <code>interface.14.http-post-api-account-v2-authenticate-via-wallet</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:182</code> | <code>POST /api/account/v2/authenticate_via_wallet -&gt; AuthenticateController.authenticate_via_wallet</code> |
| <code>interface.14.http-post-api-account-v2-confirm-otp</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:185</code> | <code>POST /api/account/v2/confirm_otp -&gt; AuthenticateController.confirm_otp</code> |
| <code>interface.14.http-post-api-account-v2-email-link</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:132</code> | <code>POST /api/account/v2/email/link -&gt; EmailController.link_email</code> |
| <code>interface.14.http-post-api-account-v2-send-otp</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:184</code> | <code>POST /api/account/v2/send_otp -&gt; AuthenticateController.send_otp</code> |
| <code>interface.14.http-post-api-account-v2-user-api-keys</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:145</code> | <code>POST /api/account/v2/user/api_keys -&gt; UserController.create_api_key</code> |
| <code>interface.14.http-post-api-account-v2-user-custom-abis</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:150</code> | <code>POST /api/account/v2/user/custom_abis -&gt; UserController.create_custom_abi</code> |
| <code>interface.14.http-post-api-account-v2-user-tags-address</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:157</code> | <code>POST /api/account/v2/user/tags/address/ -&gt; UserController.create_tag_address</code> |
| <code>interface.14.http-post-api-account-v2-user-tags-transaction</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:163</code> | <code>POST /api/account/v2/user/tags/transaction/ -&gt; UserController.create_tag_transaction</code> |
| <code>interface.14.http-post-api-account-v2-user-watchlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:140</code> | <code>POST /api/account/v2/user/watchlist -&gt; UserController.create_watchlist</code> |
| <code>interface.14.http-put-api-account-v2-user-api-keys-api-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:146</code> | <code>PUT /api/account/v2/user/api_keys/:api_key -&gt; UserController.update_api_key</code> |
| <code>interface.14.http-put-api-account-v2-user-custom-abis-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:151</code> | <code>PUT /api/account/v2/user/custom_abis/:id -&gt; UserController.update_custom_abi</code> |
| <code>interface.14.http-put-api-account-v2-user-tags-address-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:158</code> | <code>PUT /api/account/v2/user/tags/address/:id -&gt; UserController.update_tag_address</code> |
| <code>interface.14.http-put-api-account-v2-user-tags-transaction-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:164</code> | <code>PUT /api/account/v2/user/tags/transaction/:id -&gt; UserController.update_tag_transaction</code> |
| <code>interface.14.http-put-api-account-v2-user-watchlist-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:141</code> | <code>PUT /api/account/v2/user/watchlist/:id -&gt; UserController.update_watchlist</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-api-account-v2-user-api-keys-api-key</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-delete-api-account-v2-user-custom-abis-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-delete-api-account-v2-user-tags-address-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-delete-api-account-v2-user-tags-transaction-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-delete-api-account-v2-user-watchlist-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-authenticate</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-authenticate-via-dynamic</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-email-resend</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-get-csrf</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-siwe-message</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-tags-address-address-hash</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-tags-transaction-transaction-hash</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116; no in-repo caller found by path search |
| <code>interface.14.http-get-api-account-v2-user-api-keys</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-user-custom-abis</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-user-info</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-user-tags-address</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-user-tags-address-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-user-tags-transaction</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116; 26 call sites found, first 25 listed |
| <code>interface.14.http-get-api-account-v2-user-tags-transaction-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-get-api-account-v2-user-watchlist</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116; 41 call sites found, first 25 listed |
| <code>interface.14.http-post-api-account-v2-address-link</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-authenticate</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-authenticate-via-wallet</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-confirm-otp</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-email-link</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-send-otp</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-user-api-keys</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-user-custom-abis</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-user-tags-address</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-post-api-account-v2-user-tags-transaction</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116; 26 call sites found, first 25 listed |
| <code>interface.14.http-post-api-account-v2-user-watchlist</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116; 41 call sites found, first 25 listed |
| <code>interface.14.http-put-api-account-v2-user-api-keys-api-key</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-put-api-account-v2-user-custom-abis-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-put-api-account-v2-user-tags-address-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-put-api-account-v2-user-tags-transaction-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |
| <code>interface.14.http-put-api-account-v2-user-watchlist-id</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:116 |

## api-docs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-docs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:97</code> | <code>GET /api-docs -&gt; APIDocsController.index</code> |
| <code>interface.14.http-get-api-docs-l100</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:100</code> | <code>GET /api-docs -&gt; PageNotFoundController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-docs</code> | notes | guarded by if @api_router_reading_enabled; no in-repo caller found by path search |
| <code>interface.14.http-get-api-docs-l100</code> | notes | guarded by not(if @api_router_reading_enabled); no in-repo caller found by path search |

## api-eth-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-post-api-eth-rpc</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:581</code> | <code>POST /api/eth-rpc -&gt; EthRPC.EthController.eth_request</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-post-api-eth-rpc</code> | notes | guarded by if @reading_enabled; 35 call sites found, first 25 listed |

## api-health

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-health</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex:9</code> | <code>GET /api/health -&gt; BlockScoutWeb.API.HealthController.health</code> |
| <code>interface.14.http-get-api-health-l564</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:564</code> | <code>GET /api/health -&gt; BlockScoutWeb.API.HealthController.health</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-health</code> | notes | plugged ahead of the router by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex:20 and services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_endpoint.ex:7 |

## api-health-liveness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-health-liveness</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex:10</code> | <code>GET /api/health/liveness -&gt; BlockScoutWeb.API.HealthController.liveness</code> |
| <code>interface.14.http-get-api-health-liveness-l565</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:565</code> | <code>GET /api/health/liveness -&gt; BlockScoutWeb.API.HealthController.liveness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-health-liveness</code> | notes | plugged ahead of the router by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex:20 and services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_endpoint.ex:7 |

## api-health-multichain-search-export

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-health-multichain-search-export</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:567</code> | <code>GET /api/health/multichain-search-export -&gt; BlockScoutWeb.API.HealthController.multichain_search_db_export</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-health-multichain-search-export</code> | notes | no in-repo caller found by path search |

## api-health-readiness

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-health-readiness</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex:11</code> | <code>GET /api/health/readiness -&gt; BlockScoutWeb.API.HealthController.readiness</code> |
| <code>interface.14.http-get-api-health-readiness-l566</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:566</code> | <code>GET /api/health/readiness -&gt; BlockScoutWeb.API.HealthController.readiness</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-health-readiness</code> | notes | plugged ahead of the router by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex:20 and services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_endpoint.ex:7; no in-repo caller found by path search |
| <code>interface.14.http-get-api-health-readiness-l566</code> | notes | no in-repo caller found by path search |

## api-v1-eth-rpc

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-post-api-v1-eth-rpc</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:543</code> | <code>POST /api/v1/eth-rpc -&gt; EthRPC.EthController.eth_request</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-post-api-v1-eth-rpc</code> | notes | guarded by if @reading_enabled; no in-repo caller found by path search |

## api-v1-gas-price-oracle

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v1-gas-price-oracle</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:539</code> | <code>GET /api/v1/gas-price-oracle -&gt; GasPriceOracleController.gas_price_oracle</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v1-gas-price-oracle</code> | notes | no in-repo caller found by path search |

## api-v1-graphql

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-forward-api-v1-graphql</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521</code> | <code>FORWARD /api/v1/graphql -&gt; Absinthe.Plug</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-forward-api-v1-graphql</code> | notes | guarded by if @graphql_enabled; plug forward; 59 call sites found, first 25 listed |

## api-v1-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v1-search</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:537</code> | <code>GET /api/v1/search -&gt; SearchController.search</code> |

## api-v1-supply

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v1-supply</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:542</code> | <code>GET /api/v1/supply -&gt; V1.SupplyController.supply</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v1-supply</code> | notes | guarded by if @reading_enabled; no in-repo caller found by path search |

## api-v1-verified-smart-contracts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-post-api-v1-verified-smart-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:547</code> | <code>POST /api/v1/verified_smart_contracts -&gt; V1.VerifiedSmartContractController.create</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-post-api-v1-verified-smart-contracts</code> | notes | guarded by if @writing_enabled; no in-repo caller found by path search |

## api-v2-addresses

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-addresses</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:241</code> | <code>GET /api/v2/addresses -&gt; V2.AddressController.addresses_list</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:242</code> | <code>GET /api/v2/addresses/:address_hash_param -&gt; V2.AddressController.address</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-beacon-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:269</code> | <code>GET /api/v2/addresses/:address_hash_param/beacon/deposits -&gt; V2.AddressController.beacon_deposits</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-blocks-validated</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:255</code> | <code>GET /api/v2/addresses/:address_hash_param/blocks-validated -&gt; V2.AddressController.blocks_validated</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-celo-election-rewards</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:264</code> | <code>GET /api/v2/addresses/:address_hash_param/celo/election-rewards -&gt; V2.AddressController.celo_election_rewards</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-celo-election-rewards-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:265</code> | <code>GET /api/v2/addresses/:address_hash_param/celo/election-rewards/csv -&gt; V2.CsvExportController.celo_election_rewards_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-coin-balance-history</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:256</code> | <code>GET /api/v2/addresses/:address_hash_param/coin-balance-history -&gt; V2.AddressController.coin_balance_history</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-coin-balance-history-by-day</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:257</code> | <code>GET /api/v2/addresses/:address_hash_param/coin-balance-history-by-day -&gt; V2.AddressController.coin_balance_history_by_day</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:244</code> | <code>GET /api/v2/addresses/:address_hash_param/counters -&gt; V2.AddressController.counters</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-internal-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:251</code> | <code>GET /api/v2/addresses/:address_hash_param/internal-transactions -&gt; V2.AddressController.internal_transactions</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-internal-transactions-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:252</code> | <code>GET /api/v2/addresses/:address_hash_param/internal-transactions/csv -&gt; V2.CsvExportController.internal_transactions_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-logs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:253</code> | <code>GET /api/v2/addresses/:address_hash_param/logs -&gt; V2.AddressController.logs</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-logs-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:254</code> | <code>GET /api/v2/addresses/:address_hash_param/logs/csv -&gt; V2.CsvExportController.logs_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-nft</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:259</code> | <code>GET /api/v2/addresses/:address_hash_param/nft -&gt; V2.AddressController.nft_list</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-nft-collections</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:260</code> | <code>GET /api/v2/addresses/:address_hash_param/nft/collections -&gt; V2.AddressController.nft_collections</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-tabs-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:243</code> | <code>GET /api/v2/addresses/:address_hash_param/tabs-counters -&gt; V2.AddressController.tabs_counters</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-token-balances</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:245</code> | <code>GET /api/v2/addresses/:address_hash_param/token-balances -&gt; V2.AddressController.token_balances</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:249</code> | <code>GET /api/v2/addresses/:address_hash_param/token-transfers -&gt; V2.AddressController.token_transfers</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-token-transfers-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:250</code> | <code>GET /api/v2/addresses/:address_hash_param/token-transfers/csv -&gt; V2.CsvExportController.token_transfers_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-tokens</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:246</code> | <code>GET /api/v2/addresses/:address_hash_param/tokens -&gt; V2.AddressController.tokens</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:247</code> | <code>GET /api/v2/addresses/:address_hash_param/transactions -&gt; V2.AddressController.transactions</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-transactions-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:248</code> | <code>GET /api/v2/addresses/:address_hash_param/transactions/csv -&gt; V2.CsvExportController.transactions_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-unified</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:261</code> | <code>GET /api/v2/addresses/:address_hash_param/unified -&gt; V2.PaxeerX.UnifiedAccountController.unified</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:258</code> | <code>GET /api/v2/addresses/:address_hash_param/withdrawals -&gt; V2.AddressController.withdrawals</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-addresses</code> | notes | 241 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param</code> | notes | 28 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-beacon-deposits</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-celo-election-rewards</code> | notes | guarded by if @chain_identity == {:optimism, :celo}: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-celo-election-rewards-csv</code> | notes | guarded by if @chain_identity == {:optimism, :celo}: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-advanced-filters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-advanced-filters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:511</code> | <code>GET /api/v2/advanced-filters -&gt; V2.AdvancedFilterController.list</code> |
| <code>interface.14.http-get-api-v2-advanced-filters-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:512</code> | <code>GET /api/v2/advanced-filters/csv -&gt; V2.AdvancedFilterController.list_csv</code> |
| <code>interface.14.http-get-api-v2-advanced-filters-methods</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:513</code> | <code>GET /api/v2/advanced-filters/methods -&gt; V2.AdvancedFilterController.list_methods</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-advanced-filters</code> | notes | 42 call sites found, first 25 listed |

## api-v2-arbitrum

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-arbitrum-batches</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:496</code> | <code>GET /api/v2/arbitrum/batches -&gt; V2.ArbitrumController.batches</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-batch-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:498</code> | <code>GET /api/v2/arbitrum/batches/:batch_number -&gt; V2.ArbitrumController.batch</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:497</code> | <code>GET /api/v2/arbitrum/batches/count -&gt; V2.ArbitrumController.batches_count</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-anytrust-data-hash</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:499</code> | <code>GET /api/v2/arbitrum/batches/da/anytrust/:data_hash -&gt; V2.ArbitrumController.batch_by_data_availability_info</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-celestia-height-transaction-commitment</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:502</code> | <code>GET /api/v2/arbitrum/batches/da/celestia/:height/:transaction_commitment -&gt; V2.ArbitrumController.batch_by_data_availability_info</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-eigenda-data-hash</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:500</code> | <code>GET /api/v2/arbitrum/batches/da/eigenda/:data_hash -&gt; V2.ArbitrumController.batch_by_data_availability_info</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-claim-message-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:494</code> | <code>GET /api/v2/arbitrum/messages/claim/:message_id -&gt; V2.ArbitrumController.claim_message</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-direction</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:492</code> | <code>GET /api/v2/arbitrum/messages/:direction -&gt; V2.ArbitrumController.messages</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-direction-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:493</code> | <code>GET /api/v2/arbitrum/messages/:direction/count -&gt; V2.ArbitrumController.messages_count</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-withdrawals-transaction-hash</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:495</code> | <code>GET /api/v2/arbitrum/messages/withdrawals/:transaction_hash -&gt; V2.ArbitrumController.withdrawals</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-arbitrum-batches</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-batches-batch-number</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-batches-count</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-anytrust-data-hash</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171); no in-repo caller found by path search |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-celestia-height-transaction-commitment</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-eigenda-data-hash</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171); no in-repo caller found by path search |
| <code>interface.14.http-get-api-v2-arbitrum-messages-claim-message-id</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-messages-direction</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-messages-direction-count</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-arbitrum-messages-withdrawals-transaction-hash</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-beacon

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-beacon-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:433</code> | <code>GET /api/v2/beacon/deposits -&gt; V2.Ethereum.DepositController.list</code> |
| <code>interface.14.http-get-api-v2-beacon-deposits-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:434</code> | <code>GET /api/v2/beacon/deposits/count -&gt; V2.Ethereum.DepositController.count</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-beacon-deposits</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-beacon-deposits-count</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-blobs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-blobs-blob-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:440</code> | <code>GET /api/v2/blobs/:blob_hash_param -&gt; V2.BlobController.blob</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-blobs-blob-hash-param</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-blocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-blocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:216</code> | <code>GET /api/v2/blocks -&gt; V2.BlockController.blocks</code> |
| <code>interface.14.http-get-api-v2-blocks-arbitrum-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:224</code> | <code>GET /api/v2/blocks/arbitrum-batch/:batch_number_param -&gt; V2.BlockController.arbitrum_batch</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:217</code> | <code>GET /api/v2/blocks/:block_hash_or_number_param -&gt; V2.BlockController.block</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-beacon-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:236</code> | <code>GET /api/v2/blocks/:block_hash_or_number_param/beacon/deposits -&gt; V2.BlockController.beacon_deposits</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-internal-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:219</code> | <code>GET /api/v2/blocks/:block_hash_or_number_param/internal-transactions -&gt; V2.BlockController.internal_transactions</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:218</code> | <code>GET /api/v2/blocks/:block_hash_or_number_param/transactions -&gt; V2.BlockController.transactions</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:220</code> | <code>GET /api/v2/blocks/:block_hash_or_number_param/withdrawals -&gt; V2.BlockController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-blocks-block-number-param-countdown</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:221</code> | <code>GET /api/v2/blocks/:block_number_param/countdown -&gt; V2.BlockController.block_countdown</code> |
| <code>interface.14.http-get-api-v2-blocks-optimism-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:228</code> | <code>GET /api/v2/blocks/optimism-batch/:batch_number_param -&gt; V2.BlockController.optimism_batch</code> |
| <code>interface.14.http-get-api-v2-blocks-scroll-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:232</code> | <code>GET /api/v2/blocks/scroll-batch/:batch_number_param -&gt; V2.BlockController.scroll_batch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-blocks</code> | notes | 79 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-blocks-arbitrum-batch-batch-number-param</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param</code> | notes | 30 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-beacon-deposits</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-blocks-optimism-batch-batch-number-param</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-blocks-scroll-batch-batch-number-param</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-celo

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-celo-epochs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:345</code> | <code>GET /api/v2/celo/epochs -&gt; V2.CeloController.epochs</code> |
| <code>interface.14.http-get-api-v2-celo-epochs-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:346</code> | <code>GET /api/v2/celo/epochs/:number -&gt; V2.CeloController.epoch</code> |
| <code>interface.14.http-get-api-v2-celo-epochs-number-election-rewards-type</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:347</code> | <code>GET /api/v2/celo/epochs/:number/election-rewards/:type -&gt; V2.CeloController.election_rewards</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-celo-epochs</code> | notes | guarded by if @chain_identity == {:optimism, :celo}: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-celo-epochs-number</code> | notes | guarded by if @chain_identity == {:optimism, :celo}: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-celo-epochs-number-election-rewards-type</code> | notes | guarded by if @chain_identity == {:optimism, :celo}: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-config

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-config-backend</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:143</code> | <code>GET /api/v2/config/backend -&gt; V2.ConfigController.backend</code> |
| <code>interface.14.http-get-api-v2-config-backend-version</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:144</code> | <code>GET /api/v2/config/backend-version -&gt; V2.ConfigController.backend_version</code> |
| <code>interface.14.http-get-api-v2-config-celo</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:152</code> | <code>GET /api/v2/config/celo -&gt; V2.ConfigController.celo</code> |
| <code>interface.14.http-get-api-v2-config-csv-export</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:145</code> | <code>GET /api/v2/config/csv-export -&gt; V2.ConfigController.csv_export</code> |
| <code>interface.14.http-get-api-v2-config-db-background-migrations</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:149</code> | <code>GET /api/v2/config/db-background-migrations -&gt; V2.ConfigController.db_background_migrations</code> |
| <code>interface.14.http-get-api-v2-config-indexer</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:146</code> | <code>GET /api/v2/config/indexer -&gt; V2.ConfigController.indexer</code> |
| <code>interface.14.http-get-api-v2-config-public-metrics</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:147</code> | <code>GET /api/v2/config/public-metrics -&gt; V2.ConfigController.public_metrics</code> |
| <code>interface.14.http-get-api-v2-config-smart-contracts-languages</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:148</code> | <code>GET /api/v2/config/smart-contracts/languages -&gt; V2.ConfigController.languages_list</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-config-celo</code> | notes | guarded by if @chain_identity == {:optimism, :celo}: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-import

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-api-v2-import-token-info</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:122</code> | <code>DELETE /api/v2/import/token-info -&gt; V2.ImportController.delete_token_info</code> |
| <code>interface.14.http-get-api-v2-import-smart-contracts-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:124</code> | <code>GET /api/v2/import/smart-contracts/:address_hash_param -&gt; V2.ImportController.try_to_search_contract</code> |
| <code>interface.14.http-post-api-v2-import-optimism-interop</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:128</code> | <code>POST /api/v2/import/optimism/interop/ -&gt; V2.OptimismController.interop_import</code> |
| <code>interface.14.http-post-api-v2-import-smart-contracts-address-hash-param-audit-reports</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:125</code> | <code>POST /api/v2/import/smart-contracts/:address_hash_param/audit-reports -&gt; V2.ImportController.import_audit_report</code> |
| <code>interface.14.http-post-api-v2-import-token-info</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:121</code> | <code>POST /api/v2/import/token-info -&gt; V2.ImportController.import_token_info</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-post-api-v2-import-optimism-interop</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-internal-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-internal-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:212</code> | <code>GET /api/v2/internal-transactions -&gt; V2.InternalTransactionController.internal_transactions</code> |

## api-v2-key

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-post-api-v2-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_key_v2_router.ex:28</code> | <code>POST /api/v2/key -&gt; V2.APIKeyController.get_key</code> |

## api-v2-main-page

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-main-page-arbitrum-batches-committed</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:306</code> | <code>GET /api/v2/main-page/arbitrum/batches/committed -&gt; V2.ArbitrumController.batches_committed</code> |
| <code>interface.14.http-get-api-v2-main-page-arbitrum-batches-latest-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:307</code> | <code>GET /api/v2/main-page/arbitrum/batches/latest-number -&gt; V2.ArbitrumController.batch_latest_number</code> |
| <code>interface.14.http-get-api-v2-main-page-arbitrum-messages-to-rollup</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:305</code> | <code>GET /api/v2/main-page/arbitrum/messages/to-rollup -&gt; V2.ArbitrumController.recent_messages_to_l2</code> |
| <code>interface.14.http-get-api-v2-main-page-blocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:285</code> | <code>GET /api/v2/main-page/blocks -&gt; V2.MainPageController.blocks</code> |
| <code>interface.14.http-get-api-v2-main-page-indexing-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:288</code> | <code>GET /api/v2/main-page/indexing-status -&gt; V2.MainPageController.indexing_status</code> |
| <code>interface.14.http-get-api-v2-main-page-optimism-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:291</code> | <code>GET /api/v2/main-page/optimism-deposits -&gt; V2.OptimismController.main_page_deposits</code> |
| <code>interface.14.http-get-api-v2-main-page-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:286</code> | <code>GET /api/v2/main-page/transactions -&gt; V2.MainPageController.transactions</code> |
| <code>interface.14.http-get-api-v2-main-page-transactions-watchlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:287</code> | <code>GET /api/v2/main-page/transactions/watchlist -&gt; V2.MainPageController.watchlist_transactions</code> |
| <code>interface.14.http-get-api-v2-main-page-zkevm-batches-confirmed</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:295</code> | <code>GET /api/v2/main-page/zkevm/batches/confirmed -&gt; V2.PolygonZkevmController.batches_confirmed</code> |
| <code>interface.14.http-get-api-v2-main-page-zkevm-batches-latest-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:296</code> | <code>GET /api/v2/main-page/zkevm/batches/latest-number -&gt; V2.PolygonZkevmController.batch_latest_number</code> |
| <code>interface.14.http-get-api-v2-main-page-zksync-batches-confirmed</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:300</code> | <code>GET /api/v2/main-page/zksync/batches/confirmed -&gt; V2.ZkSyncController.batches_confirmed</code> |
| <code>interface.14.http-get-api-v2-main-page-zksync-batches-latest-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:301</code> | <code>GET /api/v2/main-page/zksync/batches/latest-number -&gt; V2.ZkSyncController.batch_latest_number</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-main-page-arbitrum-batches-committed</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-arbitrum-batches-latest-number</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-arbitrum-messages-to-rollup</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-optimism-deposits</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-zkevm-batches-confirmed</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-zkevm-batches-latest-number</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-zksync-batches-confirmed</code> | notes | guarded by if @chain_type == :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-main-page-zksync-batches-latest-number</code> | notes | guarded by if @chain_type == :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-mud

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-mud-worlds</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:478</code> | <code>GET /api/v2/mud/worlds -&gt; V2.MudController.worlds</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:479</code> | <code>GET /api/v2/mud/worlds/count -&gt; V2.MudController.worlds_count</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-systems</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:481</code> | <code>GET /api/v2/mud/worlds/:world/systems -&gt; V2.MudController.world_systems</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-systems-system</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:482</code> | <code>GET /api/v2/mud/worlds/:world/systems/:system -&gt; V2.MudController.world_system</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:480</code> | <code>GET /api/v2/mud/worlds/:world/tables -&gt; V2.MudController.world_tables</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:483</code> | <code>GET /api/v2/mud/worlds/:world/tables/count -&gt; V2.MudController.world_tables_count</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:484</code> | <code>GET /api/v2/mud/worlds/:world/tables/:table_id/records -&gt; V2.MudController.world_table_records</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:485</code> | <code>GET /api/v2/mud/worlds/:world/tables/:table_id/records/count -&gt; V2.MudController.world_table_records_count</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records-record-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:486</code> | <code>GET /api/v2/mud/worlds/:world/tables/:table_id/records/:record_id -&gt; V2.MudController.world_table_record</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-mud-worlds</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-count</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-systems</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-systems-system</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-count</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records-count</code> | notes | guarded by if @mud_enabled_compile_time? |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records-record-id</code> | notes | guarded by if @mud_enabled_compile_time? |

## api-v2-openapi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-openapi</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:134</code> | <code>GET /api/v2/openapi -&gt; OpenApiSpex.Plug.RenderSpec.[]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-openapi</code> | notes | no in-repo caller found by path search |

## api-v2-optimism

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-optimism-batches</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:324</code> | <code>GET /api/v2/optimism/batches -&gt; V2.OptimismController.batches</code> |
| <code>interface.14.http-get-api-v2-optimism-batches-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:325</code> | <code>GET /api/v2/optimism/batches/count -&gt; V2.OptimismController.batches_count</code> |
| <code>interface.14.http-get-api-v2-optimism-batches-da-celestia-height-commitment</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:326</code> | <code>GET /api/v2/optimism/batches/da/celestia/:height/:commitment -&gt; V2.OptimismController.batch_by_celestia_blob</code> |
| <code>interface.14.http-get-api-v2-optimism-batches-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:327</code> | <code>GET /api/v2/optimism/batches/:number -&gt; V2.OptimismController.batch_by_number</code> |
| <code>interface.14.http-get-api-v2-optimism-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:330</code> | <code>GET /api/v2/optimism/deposits -&gt; V2.OptimismController.deposits</code> |
| <code>interface.14.http-get-api-v2-optimism-deposits-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:331</code> | <code>GET /api/v2/optimism/deposits/count -&gt; V2.OptimismController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-optimism-games</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:334</code> | <code>GET /api/v2/optimism/games -&gt; V2.OptimismController.games</code> |
| <code>interface.14.http-get-api-v2-optimism-games-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:335</code> | <code>GET /api/v2/optimism/games/count -&gt; V2.OptimismController.games_count</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-messages</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:336</code> | <code>GET /api/v2/optimism/interop/messages -&gt; V2.OptimismController.interop_messages</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-messages-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:337</code> | <code>GET /api/v2/optimism/interop/messages/count -&gt; V2.OptimismController.interop_messages_count</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-messages-unique-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:338</code> | <code>GET /api/v2/optimism/interop/messages/:unique_id -&gt; V2.OptimismController.interop_message</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-public-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:339</code> | <code>GET /api/v2/optimism/interop/public-key -&gt; V2.OptimismController.interop_public_key</code> |
| <code>interface.14.http-get-api-v2-optimism-output-roots</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:328</code> | <code>GET /api/v2/optimism/output-roots -&gt; V2.OptimismController.output_roots</code> |
| <code>interface.14.http-get-api-v2-optimism-output-roots-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:329</code> | <code>GET /api/v2/optimism/output-roots/count -&gt; V2.OptimismController.output_roots_count</code> |
| <code>interface.14.http-get-api-v2-optimism-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:332</code> | <code>GET /api/v2/optimism/withdrawals -&gt; V2.OptimismController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-optimism-withdrawals-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:333</code> | <code>GET /api/v2/optimism/withdrawals/count -&gt; V2.OptimismController.withdrawals_count</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-optimism-batches</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-batches-count</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-batches-da-celestia-height-commitment</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-batches-number</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-deposits</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-deposits-count</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-games</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-games-count</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-interop-messages</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-interop-messages-count</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-interop-messages-unique-id</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-interop-public-key</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-output-roots</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-output-roots-count</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-withdrawals</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-optimism-withdrawals-count</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-path</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:573</code> | <code>GET /api/v2/*path -&gt; BlockScoutWeb.API.V2.FallbackController.unknown_action</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-path</code> | notes | no in-repo caller found by path search |

## api-v2-paxeer-x

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-paxeer-x-anchors</code> | <code>http_route</code> | <code>retire</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:275</code> | <code>GET /api/v2/paxeer-x/anchors -&gt; V2.PaxeerX.AnchorController.anchors</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-archive-receipts-id-j54</code> | <code>http_route</code> | <code>new</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex</code> (planned) | <code>GET /api/v2/paxeer-x/archive/receipts/:id -&gt; V2.PaxeerX.ArchiveController.receipt</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-capabilities</code> | <code>http_route</code> | <code>change</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:274</code> | <code>GET /api/v2/paxeer-x/capabilities -&gt; V2.PaxeerX.CapabilitiesController.capabilities</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-markets</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:278</code> | <code>GET /api/v2/paxeer-x/feed/markets -&gt; V2.PaxeerX.FeedController.markets</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-markets-market-id-updates</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:279</code> | <code>GET /api/v2/paxeer-x/feed/markets/:market_id/updates -&gt; V2.PaxeerX.FeedController.market_updates</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-updates</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:280</code> | <code>GET /api/v2/paxeer-x/feed/updates -&gt; V2.PaxeerX.FeedController.updates</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-updates-hash</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:281</code> | <code>GET /api/v2/paxeer-x/feed/updates/:hash -&gt; V2.PaxeerX.FeedController.update</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-receipts</code> | <code>http_route</code> | <code>retire</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:276</code> | <code>GET /api/v2/paxeer-x/receipts -&gt; V2.PaxeerX.ReceiptController.receipts</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-receipts-id</code> | <code>http_route</code> | <code>retire</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:277</code> | <code>GET /api/v2/paxeer-x/receipts/:id -&gt; V2.PaxeerX.ReceiptController.receipt</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-paxeer-x-anchors</code> | notes | stale spec req.8071 and task 8.7.20 remove the anchors route; AnchorController still serves it at HEAD; the /api/v2/*path fallback at services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:573 answers unknown paths after removal |
| <code>interface.14.http-get-api-v2-paxeer-x-archive-receipts-id-j54</code> | notes | Frozen kernel receipt read; replaces GET /api/v2/paxeer-x/receipts/:id. |
| <code>interface.14.http-get-api-v2-paxeer-x-capabilities</code> | notes | stale spec task 8.5.38 drops the custody and anchor probes; services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/capabilities.ex:62-71 still probes 0x1013 and 0x1014 and reports custody and anchor |
| <code>interface.14.http-get-api-v2-paxeer-x-receipts</code> | notes | stale spec req.8072 and task 8.7.20 remove the kernel receipts route; ReceiptController still serves it at HEAD; 33 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-paxeer-x-receipts-id</code> | notes | stale spec req.8072 and task 8.7.20 remove the kernel receipt route; ReceiptController still serves it at HEAD |

Contract context <code>contract_context.14.paxeer-x-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | GET /api/v2/paxeer-x/archive/anchors, /archive/receipts and /archive/receipts/:id (planned) |
| storage | Reads lx_anchors and lx_receipts; no writes. |
| errors | 404 for an unknown receipt id and for any other /api/v2/paxeer-x path. |
| events | none |
| gas | none (off-chain service) |
| activation | Archive routes land in wave 3; live routes retire in wave 6. |
| invariants | Every archive item carries frozen true and no settlement rung. |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.241.1</code> | req | 14241.1 |
| <code>acceptance.14.241.1</code> | scenario | Given the Capabilities module after the change, when Capabilities.surfaces() is called, then it returns [:unified_account, :exchange, :bridge, :launchpad] |
| <code>acceptance.14.241.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/capabilities_retired_probe_test.exs::surfaces_omit_retired_precompiles (planned) |
| <code>acceptance.14.241.1</code> | qualification | UNRUN |
| <code>req.14241</code> | ac_1 | Capabilities.surfaces/0 returns exactly [:unified_account, :exchange, :bridge, :launchpad]; the 0x1013 and 0x1014 entries at services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/capabilities.ex:62-63 are gone. |
| <code>acceptance.14.241.2</code> | req | 14241.2 |
| <code>acceptance.14.241.2</code> | scenario | Given the Capabilities module after the change, when address(:custody), address(:anchor) and address(:exchange) are called, then the first two raise FunctionClauseError and the third returns 0x0000000000000000000000000000000000001015 |
| <code>acceptance.14.241.2</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/capabilities_retired_probe_test.exs::address_refuses_retired_surfaces (planned) |
| <code>acceptance.14.241.2</code> | qualification | UNRUN |
| <code>req.14241</code> | ac_2 | Capabilities.address/1 raises FunctionClauseError for :custody and :anchor and still returns 0x...1015 for :exchange. |
| <code>acceptance.14.242.1</code> | req | 14242.1 |
| <code>acceptance.14.242.1</code> | scenario | Given a running Capabilities snapshot, when GET /api/v2/paxeer-x/capabilities is requested, then the body has the keys addr, exchange, bridge, launchpad and checked_at and has neither custody nor anchor |
| <code>acceptance.14.242.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/capabilities_shape_test.exs::capabilities_response_has_no_retired_keys (planned) |
| <code>acceptance.14.242.1</code> | qualification | UNRUN |
| <code>req.14242</code> | ac_1 | GET /api/v2/paxeer-x/capabilities (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:274) answers 200 with no custody and no anchor key. |
| <code>acceptance.14.242.2</code> | req | 14242.2 |
| <code>acceptance.14.242.2</code> | scenario | Given the updated PaxeerXCapabilities schema, when the live response and a copy with custody: true are cast, then the first casts and the second fails on an unexpected property |
| <code>acceptance.14.242.2</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/capabilities_shape_test.exs::capabilities_response_casts_against_schema (planned) |
| <code>acceptance.14.242.2</code> | qualification | UNRUN |
| <code>req.14242</code> | ac_2 | The capabilities response casts against the PaxeerXCapabilities schema, and a body carrying custody is refused by it. |
| <code>acceptance.14.244.1</code> | req | 14244.1 |
| <code>acceptance.14.244.1</code> | scenario | Given committed_vectors.json, when SequencerReceipt.verify/4 runs each case with its eth_receipt, batch, commit and validators, then the results are {:committed, key, height}, {:error, :r7} and {:error, :r8} |
| <code>acceptance.14.244.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/committed_vectors_test.exs::committed_vectors_drive_verify (planned) |
| <code>acceptance.14.244.1</code> | qualification | UNRUN |
| <code>req.14244</code> | ac_1 | The three committed vectors drive SequencerReceipt.verify/4 (services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/sequencer_receipt.ex:170) to committed, r7 and r8 results. |
| <code>acceptance.14.245.1</code> | req | 14245.1 |
| <code>acceptance.14.245.1</code> | scenario | Given no indexed block and the committed-ok vector, when Status.of(nil, %Heights{committed_height: nil}, verification) is called, then rung is :committed, reason is :sequencer_receipt and committed_height is 120 |
| <code>acceptance.14.245.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/status_committed_test.exs::committed_verification_promotes_unindexed_hash (planned) |
| <code>acceptance.14.245.1</code> | qualification | UNRUN |
| <code>req.14245</code> | ac_1 | Status.of(nil, heights, {:committed, key, 120}) returns rung :committed with reason :sequencer_receipt; at HEAD it returns :receipted (services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/status.ex:36-41). |
| <code>acceptance.14.245.2</code> | req | 14245.2 |
| <code>acceptance.14.245.2</code> | scenario | Given no indexed block and a receipted verification, when Status.of/3 is called, then rung is :receipted and reason is :sequencer_receipt |
| <code>acceptance.14.245.2</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/status_committed_test.exs::receipted_verification_stays_receipted (planned) |
| <code>acceptance.14.245.2</code> | qualification | UNRUN |
| <code>req.14245</code> | ac_2 | Status.of(nil, heights, {:receipted, key}) still returns rung :receipted. |
| <code>acceptance.14.246.1</code> | req | 14246.1 |
| <code>acceptance.14.246.1</code> | scenario | Given a router answering pax_sequencerReceipt, px_getCommit and px_getValidators from the committed-ok vector and no indexed transaction, when the status route is requested, then it answers 200 with rung committed |
| <code>acceptance.14.246.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_status_committed_test.exs::status_answers_committed_for_router_verified_hash (planned) |
| <code>acceptance.14.246.1</code> | qualification | UNRUN |
| <code>req.14246</code> | ac_1 | GET /api/v2/transactions/:hash/status (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:192) answers rung committed for an unindexed hash whose router receipt passes R1-R8. |
| <code>acceptance.14.247.1</code> | req | 14247.1 |
| <code>acceptance.14.247.1</code> | scenario | Given the planned schema, when a body with a 64-byte signature and a body with a 63-byte signature are cast, then the first casts and the second fails |
| <code>acceptance.14.247.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/schemas/api/v2/paxeer_x/transaction_signature_test.exs::signature_schema_requires_64_byte_signature (planned) |
| <code>acceptance.14.247.1</code> | qualification | UNRUN |
| <code>req.14247</code> | ac_1 | PaxeerXTransactionSignature (planned) accepts a 64-byte signature and refuses a 63-byte one. |
| <code>acceptance.14.248.1</code> | req | 14248.1 |
| <code>acceptance.14.248.1</code> | scenario | Given a router answering valid-0 from receipt_vectors.json with its pinned key, when the signature route is requested, then it answers 200 and the body casts against PaxeerXTransactionSignature |
| <code>acceptance.14.248.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller_test.exs::signature_route_returns_verified_receipt (planned) |
| <code>acceptance.14.248.1</code> | qualification | UNRUN |
| <code>req.14248</code> | ac_1 | GET /api/v2/transactions/:hash/signature (planned) answers 200 with rung, receipt, receipt_key and a 64-byte signature for a hash whose receipt passes R1-R6. |
| <code>acceptance.14.248.2</code> | req | 14248.2 |
| <code>acceptance.14.248.2</code> | scenario | Given a router answering a bit-flipped receipt, when the signature route is requested, then it answers 404 |
| <code>acceptance.14.248.2</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller_test.exs::signature_route_404_when_unverified (planned) |
| <code>acceptance.14.248.2</code> | qualification | UNRUN |
| <code>req.14248</code> | ac_2 | The signature route answers 404 when verification stops at any of R1-R6. |
| <code>acceptance.14.249.1</code> | req | 14249.1 |
| <code>acceptance.14.249.1</code> | scenario | Given a router and node answering the committed-ok vector, when the signature route is requested, then rung is committed and committed_height equals the vector height |
| <code>acceptance.14.249.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_committed_test.exs::signature_route_reports_committed_rung (planned) |
| <code>acceptance.14.249.1</code> | qualification | UNRUN |
| <code>req.14249</code> | ac_1 | The signature route answers rung committed and the commit height when R7 and R8 pass. |
| <code>acceptance.14.250.1</code> | req | 14250.1 |
| <code>acceptance.14.250.1</code> | scenario | Given the planned archive schemas, when an anchor and a receipt body are cast with and without frozen, then only the bodies with frozen true cast |
| <code>acceptance.14.250.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/schemas/api/v2/paxeer_x/archive_schemas_test.exs::archive_schemas_require_frozen_marker (planned) |
| <code>acceptance.14.250.1</code> | qualification | UNRUN |
| <code>req.14250</code> | ac_1 | The planned archive schemas require frozen: true on every anchor and receipt item. |
| <code>acceptance.14.251.1</code> | req | 14251.1 |
| <code>acceptance.14.251.1</code> | scenario | Given two inserted lx_anchors rows, when the archive anchors route is requested, then it answers 200 with both rows by descending batch_number and frozen true |
| <code>acceptance.14.251.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/archive_controller_test.exs::archive_anchors_lists_frozen_rows (planned) |
| <code>acceptance.14.251.1</code> | qualification | UNRUN |
| <code>req.14251</code> | ac_1 | GET /api/v2/paxeer-x/archive/anchors (planned) lists the lx_anchors rows newest batch first with frozen true on each. |
| <code>acceptance.14.251.2</code> | req | 14251.2 |
| <code>acceptance.14.251.2</code> | scenario | Given two inserted lx_receipts rows, when the archive receipts route is requested, then it answers 200 with both rows, frozen true and no status key |
| <code>acceptance.14.251.2</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/archive_controller_test.exs::archive_receipts_lists_frozen_rows (planned) |
| <code>acceptance.14.251.2</code> | qualification | UNRUN |
| <code>req.14251</code> | ac_2 | GET /api/v2/paxeer-x/archive/receipts (planned) lists the lx_receipts rows with frozen true and no status field. |
| <code>acceptance.14.251.3</code> | req | 14251.3 |
| <code>acceptance.14.251.3</code> | scenario | Given no lx_receipts row for a 32-byte id, when the archive receipt route is requested with it, then it answers 404 |
| <code>acceptance.14.251.3</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/archive_controller_test.exs::archive_receipt_404_for_unknown_id (planned) |
| <code>acceptance.14.251.3</code> | qualification | UNRUN |
| <code>req.14251</code> | ac_3 | GET /api/v2/paxeer-x/archive/receipts/:id (planned) answers 404 for an id with no row. |
| <code>acceptance.14.252.1</code> | req | 14252.1 |
| <code>acceptance.14.252.1</code> | scenario | Given the router after the change, when /api/v2/paxeer-x/anchors, /receipts and /receipts/0x01 are requested, then each answers 404 |
| <code>acceptance.14.252.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/retired_routes_test.exs::retired_paxeer_x_routes_answer_404 (planned) |
| <code>acceptance.14.252.1</code> | qualification | UNRUN |
| <code>req.14252</code> | ac_1 | The routes at services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:275-277 answer 404 after retirement. |
| <code>acceptance.14.252.2</code> | req | 14252.2 |
| <code>acceptance.14.252.2</code> | scenario | Given the router after the change, when /api/v2/paxeer-x/archive/anchors is requested, then it answers 200 |
| <code>acceptance.14.252.2</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/retired_routes_test.exs::archive_routes_survive_retirement (planned) |
| <code>acceptance.14.252.2</code> | qualification | UNRUN |
| <code>req.14252</code> | ac_2 | The planned archive anchors route answers 200 after the live routes retire. |
| <code>acceptance.14.253.1</code> | req | 14253.1 |
| <code>acceptance.14.253.1</code> | scenario | Given the compiled block_scout_web app after the change, when Code.ensure_loaded?/1 is called on the four modules, then each returns false |
| <code>acceptance.14.253.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/paxeer_x_retired_modules_test.exs::live_kernel_controllers_are_gone (planned) |
| <code>acceptance.14.253.1</code> | qualification | UNRUN |
| <code>req.14253</code> | ac_1 | AnchorController (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/anchor_controller.ex:47), ReceiptController and their views no longer load. |
| <code>acceptance.14.254.1</code> | req | 14254.1 |
| <code>acceptance.14.254.1</code> | scenario | Given the generated OpenAPI document, when its paths are listed, then the three archive paths are present and the two live kernel list paths are absent |
| <code>acceptance.14.254.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/openapi_spec_test.exs::openapi_lists_archive_paths_not_live_kernel_paths (planned) |
| <code>acceptance.14.254.1</code> | qualification | UNRUN |
| <code>req.14254</code> | ac_1 | The OpenAPI document lists the archive paths and not /api/v2/paxeer-x/anchors or /receipts (services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/openapi_spec_test.exs:23). |
| <code>acceptance.14.263.1</code> | req | 14263.1 |
| <code>acceptance.14.263.1</code> | scenario | Given the compiled explorer app, when Code.ensure_loaded?(Explorer.Chain.PaxeerX.Finality.Anchor) is called, then it returns false |
| <code>acceptance.14.263.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/finality_heights_test.exs::finality_anchor_struct_is_gone (planned) |
| <code>acceptance.14.263.1</code> | qualification | UNRUN |
| <code>req.14263</code> | ac_1 | Explorer.Chain.PaxeerX.Finality.Anchor (services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/finality.ex:1-43) no longer loads. |
| <code>acceptance.14.263.2</code> | req | 14263.2 |
| <code>acceptance.14.263.2</code> | scenario | Given consensus blocks 7 and 9 and a non-consensus block 11, when Finality.heights/1 is called, then committed_height is 9 |
| <code>acceptance.14.263.2</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/finality_heights_test.exs::heights_reads_max_consensus_block (planned) |
| <code>acceptance.14.263.2</code> | qualification | UNRUN |
| <code>req.14263</code> | ac_2 | Finality.heights/1 (services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/finality.ex:62-71) reports the highest consensus block as committed_height. |
| <code>acceptance.14.265.1</code> | req | 14265.1 |
| <code>acceptance.14.265.1</code> | scenario | Given an address with one EVM balance and one transfer, when GET /api/v2/addresses/:hash/unified is requested and its body cast through PaxeerXUnifiedAccount, then the cast succeeds |
| <code>acceptance.14.265.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/schemas/api/v2/paxeer_x/unified_account_shape_test.exs::unified_answer_casts_against_schema (planned) |
| <code>acceptance.14.265.1</code> | qualification | UNRUN |
| <code>req.14265</code> | ac_1 | The answer of services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/unified_account_controller.ex:184-193 casts against the unified account schema with no kernel_account, parts or side. |
| <code>acceptance.14.266.1</code> | req | 14266.1 |
| <code>acceptance.14.266.1</code> | scenario | Given the compiled block_scout_web app, when Code.ensure_loaded?/1 is called on UnifiedAccountView, then it returns false |
| <code>acceptance.14.266.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/views/api/v2/paxeer_x/unified_account_view_gone_test.exs::unified_account_view_is_gone (planned) |
| <code>acceptance.14.266.1</code> | qualification | UNRUN |
| <code>req.14266</code> | ac_1 | services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/api/v2/paxeer_x/unified_account_view.ex:1 is deleted and the module no longer loads. |
| <code>acceptance.14.298.1</code> | req | 14298.1 |
| <code>acceptance.14.298.1</code> | scenario | Given the two schema modules, when their enum and required lists are read, then they equal [submitted, receipted, committed] and [rung, block_number, committed_height] |
| <code>acceptance.14.298.1</code> | test | services/explorer/backend/apps/block_scout_web/test/block_scout_web/controllers/api/v2/paxeer_x/transaction_status_schema_test.exs::status_schema_enum_is_three_rungs (planned) |
| <code>acceptance.14.298.1</code> | qualification | UNRUN |
| <code>req.14298</code> | ac_1 | The SettlementRung enum at services/explorer/backend/apps/block_scout_web/lib/block_scout_web/schemas/api/v2/paxeer_x/settlement_rung.ex:16 stays the three rungs and the status schema requires its three fields. |
| <code>capability.14.21.1</code> | title | Stop probing the retired custody and anchor precompiles in Capabilities |
| <code>capability.14.21.1</code> | reqs | 14241.1<br>14241.2 |
| <code>capability.14.21.2</code> | title | Drop custody and anchor from the capabilities response and its OpenAPI schema |
| <code>capability.14.21.2</code> | reqs | 14242.1<br>14242.2 |
| <code>capability.14.22.1</code> | title | Freeze committed-rung vectors for sequencer receipt R7 and R8 |
| <code>capability.14.22.1</code> | reqs | 14244.1 |
| <code>capability.14.22.2</code> | title | Promote a committed sequencer verification to the committed rung in Status.of |
| <code>capability.14.22.2</code> | reqs | 14245.1<br>14245.2 |
| <code>capability.14.22.3</code> | title | Pass R7 and R8 evidence to SequencerReceipt.verify from the status controller |
| <code>capability.14.22.3</code> | reqs | 14246.1 |
| <code>capability.14.22.4</code> | title | Add the schema for the sequencer signature answer |
| <code>capability.14.22.4</code> | reqs | 14247.1 |
| <code>capability.14.22.5</code> | title | Serve the verified sequencer signature at /transactions/:hash/signature |
| <code>capability.14.22.5</code> | reqs | 14248.1<br>14248.2 |
| <code>capability.14.22.6</code> | title | Report the committed rung on the signature route once R7 and R8 pass |
| <code>capability.14.22.6</code> | reqs | 14249.1 |
| <code>capability.14.23.1</code> | title | Add the archive anchor and receipt schemas with a frozen marker |
| <code>capability.14.23.1</code> | reqs | 14250.1 |
| <code>capability.14.23.2</code> | title | Serve frozen anchors and receipts under /api/v2/paxeer-x/archive |
| <code>capability.14.23.2</code> | reqs | 14251.1<br>14251.2<br>14251.3 |
| <code>capability.14.23.3</code> | title | Retire the live anchors and receipts routes and answer 404 under /paxeer-x |
| <code>capability.14.23.3</code> | reqs | 14252.1<br>14252.2 |
| <code>capability.14.23.4</code> | title | Delete the live anchor and receipt controllers, views and their tests |
| <code>capability.14.23.4</code> | reqs | 14253.1 |
| <code>capability.14.23.5</code> | title | Replace the kernel list schemas in the OpenAPI document with the archive schemas |
| <code>capability.14.23.5</code> | reqs | 14254.1 |
| <code>capability.14.26.1</code> | title | Delete the dead Finality.Anchor struct |
| <code>capability.14.26.1</code> | reqs | 14263.1<br>14263.2 |
| <code>capability.14.27.1</code> | title | Align the unified account OpenAPI schemas with the answer the controller sends |
| <code>capability.14.27.1</code> | reqs | 14265.1 |
| <code>capability.14.27.2</code> | title | Delete the uncalled UnifiedAccountView |
| <code>capability.14.27.2</code> | reqs | 14266.1 |
| <code>capability.14.36.1</code> | title | Describe the R1-R8 committed path in the transaction status schema |
| <code>capability.14.36.1</code> | reqs | 14298.1 |
| <code>resolution.17.947</code> | title | decision.1422 binds: Serve kernel anchors and receipts read-only under /api/v2/paxeer-x/archive |
| <code>resolution.17.947</code> | applies_to | decision.1422<br>task.14.23.2<br>task.14.23.3<br>task.14.23.6 |
| <code>resolution.17.947</code> | source | decision.1422 in fragments/J54/decision.kvx; tasks from affects |
| <code>resolution.17.951</code> | title | decision.1426 binds: A committed sequencer verification places an unindexed hash on the committed rung |
| <code>resolution.17.951</code> | applies_to | decision.1426<br>task.14.22.2<br>task.14.22.3<br>task.14.36.1 |
| <code>resolution.17.951</code> | source | decision.1426 in fragments/J54/decision.kvx; tasks from affects |
| <code>resolution.17.952</code> | title | decision.1427 binds: The unified account schemas follow the controller answer |
| <code>resolution.17.952</code> | applies_to | decision.1427<br>task.14.27.1<br>task.14.27.2<br>task.14.33.1 |
| <code>resolution.17.952</code> | source | decision.1427 in fragments/J54/decision.kvx; tasks from affects |

## api-v2-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-proxy-3rdparty-noves-fi-addresses-address-hash-param-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:396</code> | <code>GET /api/v2/proxy/3rdparty/noves-fi/addresses/:address_hash_param/transactions -&gt; V2.Proxy.NovesFiController.address_transactions</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-noves-fi-transaction-descriptions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:398</code> | <code>GET /api/v2/proxy/3rdparty/noves-fi/transaction-descriptions -&gt; V2.Proxy.NovesFiController.describe_transactions</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-noves-fi-transactions-transaction-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:394</code> | <code>GET /api/v2/proxy/3rdparty/noves-fi/transactions/:transaction_hash_param -&gt; V2.Proxy.NovesFiController.transaction</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-platform-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:391</code> | <code>GET /api/v2/proxy/3rdparty/:platform_id -&gt; V2.Proxy.UniversalProxyController.index</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-solidityscan-smart-contracts-address-hash-report</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:406</code> | <code>GET /api/v2/proxy/3rdparty/solidityscan/smart-contracts/:address_hash/report -&gt; V2.Proxy.SolidityScanController.solidityscan_report</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-xname-addresses-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:402</code> | <code>GET /api/v2/proxy/3rdparty/xname/addresses/:address_hash_param -&gt; V2.Proxy.XnameController.address</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-accounts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:420</code> | <code>GET /api/v2/proxy/account-abstraction/accounts -&gt; V2.Proxy.AccountAbstractionController.accounts</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-accounts-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:419</code> | <code>GET /api/v2/proxy/account-abstraction/accounts/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.account</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-bundlers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:414</code> | <code>GET /api/v2/proxy/account-abstraction/bundlers -&gt; V2.Proxy.AccountAbstractionController.bundlers</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-bundlers-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:413</code> | <code>GET /api/v2/proxy/account-abstraction/bundlers/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.bundler</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-bundles</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:421</code> | <code>GET /api/v2/proxy/account-abstraction/bundles -&gt; V2.Proxy.AccountAbstractionController.bundles</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-factories</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:416</code> | <code>GET /api/v2/proxy/account-abstraction/factories -&gt; V2.Proxy.AccountAbstractionController.factories</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-factories-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:415</code> | <code>GET /api/v2/proxy/account-abstraction/factories/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.factory</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-operations</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:422</code> | <code>GET /api/v2/proxy/account-abstraction/operations -&gt; V2.Proxy.AccountAbstractionController.operations</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-operations-operation-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:411</code> | <code>GET /api/v2/proxy/account-abstraction/operations/:operation_hash_param -&gt; V2.Proxy.AccountAbstractionController.operation</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-operations-operation-hash-param-summary</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:412</code> | <code>GET /api/v2/proxy/account-abstraction/operations/:operation_hash_param/summary -&gt; V2.Proxy.AccountAbstractionController.summary</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-paymasters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:418</code> | <code>GET /api/v2/proxy/account-abstraction/paymasters -&gt; V2.Proxy.AccountAbstractionController.paymasters</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-paymasters-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:417</code> | <code>GET /api/v2/proxy/account-abstraction/paymasters/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.paymaster</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:423</code> | <code>GET /api/v2/proxy/account-abstraction/status -&gt; V2.Proxy.AccountAbstractionController.status</code> |
| <code>interface.14.http-get-api-v2-proxy-metadata-addresses</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:427</code> | <code>GET /api/v2/proxy/metadata/addresses -&gt; V2.Proxy.MetadataController.addresses</code> |

## api-v2-scam-badge-addresses

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-delete-api-v2-scam-badge-addresses</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex:48</code> | <code>DELETE /api/v2/scam-badge-addresses -&gt; V2.AddressBadgeController.unassign_badge_from_address</code> |
| <code>interface.14.http-get-api-v2-scam-badge-addresses</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex:54</code> | <code>GET /api/v2/scam-badge-addresses -&gt; V2.AddressBadgeController.show_badge_addresses</code> |
| <code>interface.14.http-post-api-v2-scam-badge-addresses</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex:47</code> | <code>POST /api/v2/scam-badge-addresses -&gt; V2.AddressBadgeController.assign_badge_to_address</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-delete-api-v2-scam-badge-addresses</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-api-v2-scam-badge-addresses</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-post-api-v2-scam-badge-addresses</code> | notes | no in-repo caller found by path search |

## api-v2-scroll

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-scroll-batches</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:353</code> | <code>GET /api/v2/scroll/batches -&gt; V2.ScrollController.batches</code> |
| <code>interface.14.http-get-api-v2-scroll-batches-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:354</code> | <code>GET /api/v2/scroll/batches/count -&gt; V2.ScrollController.batches_count</code> |
| <code>interface.14.http-get-api-v2-scroll-batches-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:355</code> | <code>GET /api/v2/scroll/batches/:number -&gt; V2.ScrollController.batch</code> |
| <code>interface.14.http-get-api-v2-scroll-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:356</code> | <code>GET /api/v2/scroll/deposits -&gt; V2.ScrollController.deposits</code> |
| <code>interface.14.http-get-api-v2-scroll-deposits-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:357</code> | <code>GET /api/v2/scroll/deposits/count -&gt; V2.ScrollController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-scroll-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:358</code> | <code>GET /api/v2/scroll/withdrawals -&gt; V2.ScrollController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-scroll-withdrawals-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:359</code> | <code>GET /api/v2/scroll/withdrawals/count -&gt; V2.ScrollController.withdrawals_count</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-scroll-batches</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-scroll-batches-count</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-scroll-batches-number</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-scroll-deposits</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-scroll-deposits-count</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-scroll-withdrawals</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-scroll-withdrawals-count</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-search</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:137</code> | <code>GET /api/v2/search -&gt; V2.SearchController.search</code> |
| <code>interface.14.http-get-api-v2-search-check-redirect</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:138</code> | <code>GET /api/v2/search/check-redirect -&gt; V2.SearchController.check_redirect</code> |
| <code>interface.14.http-get-api-v2-search-quick</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:139</code> | <code>GET /api/v2/search/quick -&gt; V2.SearchController.quick_search</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-search</code> | notes | 79 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-search-quick</code> | notes | 29 call sites found, first 25 listed |

## api-v2-shibarium

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-shibarium-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:365</code> | <code>GET /api/v2/shibarium/deposits -&gt; V2.ShibariumController.deposits</code> |
| <code>interface.14.http-get-api-v2-shibarium-deposits-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:366</code> | <code>GET /api/v2/shibarium/deposits/count -&gt; V2.ShibariumController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-shibarium-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:367</code> | <code>GET /api/v2/shibarium/withdrawals -&gt; V2.ShibariumController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-shibarium-withdrawals-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:368</code> | <code>GET /api/v2/shibarium/withdrawals/count -&gt; V2.ShibariumController.withdrawals_count</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-shibarium-deposits</code> | notes | guarded by chain_scope :shibarium (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-shibarium-deposits-count</code> | notes | guarded by chain_scope :shibarium (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-shibarium-withdrawals</code> | notes | guarded by chain_scope :shibarium (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-shibarium-withdrawals-count</code> | notes | guarded by chain_scope :shibarium (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |

## api-v2-smart-contracts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-smart-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:51</code> | <code>GET /api/v2/smart-contracts -&gt; V2.SmartContractController.smart_contracts_list</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:53</code> | <code>GET /api/v2/smart-contracts/:address_hash_param -&gt; V2.SmartContractController.smart_contract</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-address-hash-param-audit-reports</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:54</code> | <code>GET /api/v2/smart-contracts/:address_hash_param/audit-reports -&gt; V2.SmartContractController.audit_reports_list</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:52</code> | <code>GET /api/v2/smart-contracts/counters -&gt; V2.SmartContractController.smart_contracts_counters</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-verification-config</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:56</code> | <code>GET /api/v2/smart-contracts/verification/config -&gt; V2.VerificationController.config</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-param-audit-reports</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:62</code> | <code>POST /api/v2/smart-contracts/:address_hash_param/audit-reports -&gt; V2.SmartContractController.audit_report_submission</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-flattened-code</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:71</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/flattened-code -&gt; V2.VerificationController.verification_via_flattened_code</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-multi-part</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:73</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/multi-part -&gt; V2.VerificationController.verification_via_multi_part</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-sourcify</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:72</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/sourcify -&gt; V2.VerificationController.verification_via_sourcify</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-standard-input</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:68</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/standard-input -&gt; V2.VerificationController.verification_via_standard_input</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-stylus-github-repository</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:80</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/stylus-github-repository -&gt; V2.VerificationController.verification_via_stylus_github_repository</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-code</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:74</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/vyper-code -&gt; V2.VerificationController.verification_via_vyper_code</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-multi-part</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:75</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/vyper-multi-part -&gt; V2.VerificationController.verification_via_vyper_multipart</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-standard-input</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex:76</code> | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/vyper-standard-input -&gt; V2.VerificationController.verification_via_vyper_standard_input</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-smart-contracts</code> | notes | 82 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-smart-contracts-address-hash-param</code> | notes | 26 call sites found, first 25 listed |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-flattened-code</code> | notes | guarded by if @chain_type !== :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-multi-part</code> | notes | guarded by if @chain_type !== :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-sourcify</code> | notes | guarded by if @chain_type !== :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-stylus-github-repository</code> | notes | guarded by if @chain_type === :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171); no in-repo caller found by path search |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-code</code> | notes | guarded by if @chain_type !== :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-multi-part</code> | notes | guarded by if @chain_type !== :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-standard-input</code> | notes | guarded by if @chain_type !== :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-stats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-stats</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:312</code> | <code>GET /api/v2/stats -&gt; V2.StatsController.stats</code> |
| <code>interface.14.http-get-api-v2-stats-charts-market</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:317</code> | <code>GET /api/v2/stats/charts/market -&gt; V2.StatsController.market_chart</code> |
| <code>interface.14.http-get-api-v2-stats-charts-secondary-coin-market</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:318</code> | <code>GET /api/v2/stats/charts/secondary-coin-market -&gt; V2.StatsController.secondary_coin_market_chart</code> |
| <code>interface.14.http-get-api-v2-stats-charts-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:316</code> | <code>GET /api/v2/stats/charts/transactions -&gt; V2.StatsController.transactions_chart</code> |
| <code>interface.14.http-get-api-v2-stats-hot-smart-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:313</code> | <code>GET /api/v2/stats/hot-smart-contracts -&gt; V2.StatsController.hot_smart_contracts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-stats</code> | notes | 35 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-stats-hot-smart-contracts</code> | notes | 28 call sites found, first 25 listed |

## api-v2-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:208</code> | <code>GET /api/v2/token-transfers -&gt; V2.TokenTransferController.token_transfers</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-token-transfers</code> | notes | 28 call sites found, first 25 listed |

## api-v2-tokens

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-tokens</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:67</code> | <code>GET /api/v2/tokens -&gt; V2.TokenController.tokens_list</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:68</code> | <code>GET /api/v2/tokens/:address_hash_param -&gt; V2.TokenController.token</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:69</code> | <code>GET /api/v2/tokens/:address_hash_param/counters -&gt; V2.TokenController.counters</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:71</code> | <code>GET /api/v2/tokens/:address_hash_param/holders -&gt; V2.TokenController.holders</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-holders-csv</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:72</code> | <code>GET /api/v2/tokens/:address_hash_param/holders/csv -&gt; V2.CsvExportController.export_token_holders</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:73</code> | <code>GET /api/v2/tokens/:address_hash_param/instances -&gt; V2.TokenController.instances</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:74</code> | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param -&gt; V2.TokenController.instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:76</code> | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param/holders -&gt; V2.TokenController.holders_by_instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:75</code> | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param/transfers -&gt; V2.TokenController.transfers_by_instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param-transfers-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:78</code> | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param/transfers-count -&gt; V2.TokenController.transfers_count_by_instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:70</code> | <code>GET /api/v2/tokens/:address_hash_param/transfers -&gt; V2.TokenController.transfers</code> |
| <code>interface.14.http-get-api-v2-tokens-bridged</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:64</code> | <code>GET /api/v2/tokens/bridged -&gt; V2.TokenController.bridged_tokens_list</code> |
| <code>interface.14.http-patch-api-v2-tokens-address-hash-param-instances-refetch-metadata</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:53</code> | <code>PATCH /api/v2/tokens/:address_hash_param/instances/refetch-metadata -&gt; V2.TokenController.trigger_nft_collection_metadata_refetch</code> |
| <code>interface.14.http-patch-api-v2-tokens-address-hash-param-instances-token-id-param-refetch-metadata</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex:51</code> | <code>PATCH /api/v2/tokens/:address_hash_param/instances/:token_id_param/refetch-metadata -&gt; V2.TokenController.refetch_metadata</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-tokens</code> | notes | 118 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param</code> | notes | 30 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-tokens-bridged</code> | notes | guarded by if @bridged_tokens_enabled |

## api-v2-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:157</code> | <code>GET /api/v2/transactions -&gt; V2.TransactionController.transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-arbitrum-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:170</code> | <code>GET /api/v2/transactions/arbitrum-batch/:batch_number_param -&gt; V2.TransactionController.arbitrum_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-execution-node-execution-node-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:182</code> | <code>GET /api/v2/transactions/execution-node/:execution_node_hash_param -&gt; V2.TransactionController.execution_node</code> |
| <code>interface.14.http-get-api-v2-transactions-optimism-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:174</code> | <code>GET /api/v2/transactions/optimism-batch/:batch_number_param -&gt; V2.TransactionController.optimism_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-scroll-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:178</code> | <code>GET /api/v2/transactions/scroll-batch/:batch_number_param -&gt; V2.TransactionController.scroll_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-stats</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:159</code> | <code>GET /api/v2/transactions/stats -&gt; V2.TransactionController.stats</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:185</code> | <code>GET /api/v2/transactions/:transaction_hash_param -&gt; V2.TransactionController.transaction</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-beacon-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:203</code> | <code>GET /api/v2/transactions/:transaction_hash_param/beacon/deposits -&gt; V2.TransactionController.beacon_deposits</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-blobs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:199</code> | <code>GET /api/v2/transactions/:transaction_hash_param/blobs -&gt; V2.TransactionController.blobs</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-external-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:195</code> | <code>GET /api/v2/transactions/:transaction_hash_param/external-transactions -&gt; V2.TransactionController.external_transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-internal-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:187</code> | <code>GET /api/v2/transactions/:transaction_hash_param/internal-transactions -&gt; V2.TransactionController.internal_transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-logs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:188</code> | <code>GET /api/v2/transactions/:transaction_hash_param/logs -&gt; V2.TransactionController.logs</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-raw-trace</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:189</code> | <code>GET /api/v2/transactions/:transaction_hash_param/raw-trace -&gt; V2.TransactionController.raw_trace</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-state-changes</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:190</code> | <code>GET /api/v2/transactions/:transaction_hash_param/state-changes -&gt; V2.TransactionController.state_changes</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-status</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:192</code> | <code>GET /api/v2/transactions/:transaction_hash_param/status -&gt; V2.PaxeerX.TransactionStatusController.status</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-summary</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:191</code> | <code>GET /api/v2/transactions/:transaction_hash_param/summary -&gt; V2.TransactionController.summary</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:186</code> | <code>GET /api/v2/transactions/:transaction_hash_param/token-transfers -&gt; V2.TransactionController.token_transfers</code> |
| <code>interface.14.http-get-api-v2-transactions-watchlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:158</code> | <code>GET /api/v2/transactions/watchlist -&gt; V2.TransactionController.watchlist_transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-zkevm-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:162</code> | <code>GET /api/v2/transactions/zkevm-batch/:batch_number_param -&gt; V2.TransactionController.polygon_zkevm_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-zksync-batch-batch-number-param</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:166</code> | <code>GET /api/v2/transactions/zksync-batch/:batch_number_param -&gt; V2.TransactionController.zksync_batch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-transactions</code> | notes | 153 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-transactions-arbitrum-batch-batch-number-param</code> | notes | guarded by if @chain_type == :arbitrum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-transactions-execution-node-execution-node-hash-param</code> | notes | guarded by if @chain_type == :suave: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-transactions-optimism-batch-batch-number-param</code> | notes | guarded by if @chain_type == :optimism: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-transactions-scroll-batch-batch-number-param</code> | notes | guarded by if @chain_type == :scroll: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param</code> | notes | 42 call sites found, first 25 listed |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-beacon-deposits</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171); no in-repo caller found by path search |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-blobs</code> | notes | guarded by if @chain_type == :ethereum: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-external-transactions</code> | notes | guarded by chain_scope :neon (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-transactions-zkevm-batch-batch-number-param</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-transactions-zksync-batch-batch-number-param</code> | notes | guarded by if @chain_type == :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-utils

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-utils-decode-calldata</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/utils_api_v2_router.ex:29</code> | <code>GET /api/v2/utils/decode-calldata -&gt; V2.UtilsController.decode_calldata</code> |
| <code>interface.14.http-post-api-v2-utils-decode-calldata</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/utils_api_v2_router.ex:30</code> | <code>POST /api/v2/utils/decode-calldata -&gt; V2.UtilsController.decode_calldata</code> |

## api-v2-validators

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-validators-blackfort</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:461</code> | <code>GET /api/v2/validators/blackfort -&gt; V2.ValidatorController.blackfort_validators_list</code> |
| <code>interface.14.http-get-api-v2-validators-blackfort-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:462</code> | <code>GET /api/v2/validators/blackfort/counters -&gt; V2.ValidatorController.blackfort_validators_counters</code> |
| <code>interface.14.http-get-api-v2-validators-stability</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:454</code> | <code>GET /api/v2/validators/stability -&gt; V2.ValidatorController.stability_validators_list</code> |
| <code>interface.14.http-get-api-v2-validators-stability-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:455</code> | <code>GET /api/v2/validators/stability/counters -&gt; V2.ValidatorController.stability_validators_counters</code> |
| <code>interface.14.http-get-api-v2-validators-zilliqa</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:447</code> | <code>GET /api/v2/validators/zilliqa -&gt; V2.ValidatorController.zilliqa_validators_list</code> |
| <code>interface.14.http-get-api-v2-validators-zilliqa-bls-public-key</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:448</code> | <code>GET /api/v2/validators/zilliqa/:bls_public_key -&gt; V2.ValidatorController.zilliqa_validator</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-validators-blackfort</code> | notes | guarded by chain_scope :blackfort (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-validators-blackfort-counters</code> | notes | guarded by chain_scope :blackfort (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-validators-stability</code> | notes | guarded by chain_scope :stability (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-validators-stability-counters</code> | notes | guarded by chain_scope :stability (runtime CheckChainType): answered only when the runtime CHAIN_TYPE matches (routers/chain_type_scope.ex) |
| <code>interface.14.http-get-api-v2-validators-zilliqa</code> | notes | guarded by if @chain_type == :zilliqa: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-validators-zilliqa-bls-public-key</code> | notes | guarded by if @chain_type == :zilliqa: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-withdrawals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:373</code> | <code>GET /api/v2/withdrawals -&gt; V2.WithdrawalController.withdrawals_list</code> |
| <code>interface.14.http-get-api-v2-withdrawals-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:374</code> | <code>GET /api/v2/withdrawals/counters -&gt; V2.WithdrawalController.withdrawals_counters</code> |

## api-v2-zkevm

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-zkevm-batches</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:379</code> | <code>GET /api/v2/zkevm/batches -&gt; V2.PolygonZkevmController.batches</code> |
| <code>interface.14.http-get-api-v2-zkevm-batches-batch-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:381</code> | <code>GET /api/v2/zkevm/batches/:batch_number -&gt; V2.PolygonZkevmController.batch</code> |
| <code>interface.14.http-get-api-v2-zkevm-batches-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:380</code> | <code>GET /api/v2/zkevm/batches/count -&gt; V2.PolygonZkevmController.batches_count</code> |
| <code>interface.14.http-get-api-v2-zkevm-deposits</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:382</code> | <code>GET /api/v2/zkevm/deposits -&gt; V2.PolygonZkevmController.deposits</code> |
| <code>interface.14.http-get-api-v2-zkevm-deposits-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:383</code> | <code>GET /api/v2/zkevm/deposits/count -&gt; V2.PolygonZkevmController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-zkevm-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:384</code> | <code>GET /api/v2/zkevm/withdrawals -&gt; V2.PolygonZkevmController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-zkevm-withdrawals-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:385</code> | <code>GET /api/v2/zkevm/withdrawals/count -&gt; V2.PolygonZkevmController.withdrawals_count</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-zkevm-batches</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zkevm-batches-batch-number</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zkevm-batches-count</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zkevm-deposits</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zkevm-deposits-count</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zkevm-withdrawals</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zkevm-withdrawals-count</code> | notes | guarded by if @chain_type == :polygon_zkevm: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## api-v2-zksync

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-zksync-batches</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:469</code> | <code>GET /api/v2/zksync/batches -&gt; V2.ZkSyncController.batches</code> |
| <code>interface.14.http-get-api-v2-zksync-batches-batch-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:471</code> | <code>GET /api/v2/zksync/batches/:batch_number -&gt; V2.ZkSyncController.batch</code> |
| <code>interface.14.http-get-api-v2-zksync-batches-count</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:470</code> | <code>GET /api/v2/zksync/batches/count -&gt; V2.ZkSyncController.batches_count</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-zksync-batches</code> | notes | guarded by if @chain_type == :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zksync-batches-batch-number</code> | notes | guarded by if @chain_type == :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.http-get-api-v2-zksync-batches-count</code> | notes | guarded by if @chain_type == :zksync: compiled only for that CHAIN_TYPE; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |

## auth

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-auth-provider</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:79</code> | <code>GET /auth/:provider -&gt; Account.AuthController.request</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-auth-provider</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:70; also reachable under /account and /api/account; no in-repo caller found by path search |

## auth-callback

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-auth-provider-callback</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:80</code> | <code>GET /auth/:provider/callback -&gt; Account.AuthController.callback</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-auth-provider-callback</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:70; also reachable under /account and /api/account; no in-repo caller found by path search |

## auth-logout

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-auth-logout</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:78</code> | <code>GET /auth/logout -&gt; Account.AuthController.logout</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-auth-logout</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:70; also reachable under /account and /api/account; no in-repo caller found by path search |

## auth-path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-any-auth-path</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:70</code> | <code>ANY /auth/*path -&gt; AccountRouter</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-any-auth-path</code> | notes | dispatches to the /auth scope of routers/account_router.ex; no in-repo caller found by path search |

## auth-profile

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-auth-profile</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex:77</code> | <code>GET /auth/profile -&gt; Account.AuthController.profile</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-auth-profile</code> | notes | mounted by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:70; also reachable under /account and /api/account; no in-repo caller found by path search |

## block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-block-hash-or-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:58</code> | <code>GET /block/:hash_or_number -&gt; BlockController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-block-hash-or-number</code> | notes | no in-repo caller found by path search |

## block-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-block-block-hash-or-number-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:59</code> | <code>GET /block/:block_hash_or_number/transactions -&gt; BlockTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-block-block-hash-or-number-transactions</code> | notes | no in-repo caller found by path search |

## block-withdrawals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-block-block-hash-or-number-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:60</code> | <code>GET /block/:block_hash_or_number/withdrawals -&gt; BlockWithdrawalController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-block-block-hash-or-number-withdrawals</code> | notes | no in-repo caller found by path search |

## blocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-blocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:63</code> | <code>GET /blocks -&gt; BlockController.resources</code> |
| <code>interface.14.http-get-blocks-hash-or-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:65</code> | <code>GET /blocks/:hash_or_number -&gt; BlockController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-blocks</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-blocks-hash-or-number</code> | notes | no in-repo caller found by path search |

## blocks-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-blocks-block-hash-or-number-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:69</code> | <code>GET /blocks/:block_hash_or_number/transactions -&gt; BlockTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-blocks-block-hash-or-number-transactions</code> | notes | no in-repo caller found by path search |

## blocks-withdrawals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-blocks-block-hash-or-number-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:70</code> | <code>GET /blocks/:block_hash_or_number/withdrawals -&gt; BlockWithdrawalController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-blocks-block-hash-or-number-withdrawals</code> | notes | no in-repo caller found by path search |

## chain-blocks

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-chain-blocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:444</code> | <code>GET /chain-blocks -&gt; ChainController.chain_blocks</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-chain-blocks</code> | notes | no in-repo caller found by path search |

## csv-export

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-csv-export</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:440</code> | <code>GET /csv-export -&gt; CsvExportController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-csv-export</code> | notes | no in-repo caller found by path search |

## eth-rpc-api-docs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-eth-rpc-api-docs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:98</code> | <code>GET /eth-rpc-api-docs -&gt; APIDocsController.eth_rpc</code> |
| <code>interface.14.http-get-eth-rpc-api-docs-l101</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:101</code> | <code>GET /eth-rpc-api-docs -&gt; PageNotFoundController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-eth-rpc-api-docs</code> | notes | guarded by if @api_router_reading_enabled; no in-repo caller found by path search |
| <code>interface.14.http-get-eth-rpc-api-docs-l101</code> | notes | guarded by not(if @api_router_reading_enabled); no in-repo caller found by path search |

## graphiql

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-forward-graphiql</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:81</code> | <code>FORWARD /graphiql -&gt; Absinthe.Plug.GraphiQL</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-forward-graphiql</code> | notes | guarded by if @graphql_enabled &amp;&amp; @api_router_reading_enabled; plug forward; no in-repo caller found by path search |

## graphql

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-graphql-query-address</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:71</code> | <code>POST /api/v1/graphql query { address }</code> |
| <code>interface.14.http-graphql-query-addresses</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:77</code> | <code>POST /api/v1/graphql query { addresses }</code> |
| <code>interface.14.http-graphql-query-block</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:84</code> | <code>POST /api/v1/graphql query { block }</code> |
| <code>interface.14.http-graphql-query-node</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:49</code> | <code>POST /api/v1/graphql query { node }</code> |
| <code>interface.14.http-graphql-query-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:90</code> | <code>POST /api/v1/graphql query { token_transfers }</code> |
| <code>interface.14.http-graphql-query-transaction</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:106</code> | <code>POST /api/v1/graphql query { transaction }</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-graphql-query-address</code> | notes | forwarded by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521 when API_GRAPHQL_ENABLED; no in-repo client query found outside block_scout_web tests |
| <code>interface.14.http-graphql-query-addresses</code> | notes | forwarded by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521 when API_GRAPHQL_ENABLED; no in-repo client query found outside block_scout_web tests |
| <code>interface.14.http-graphql-query-block</code> | notes | forwarded by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521 when API_GRAPHQL_ENABLED; no in-repo client query found outside block_scout_web tests |
| <code>interface.14.http-graphql-query-node</code> | notes | forwarded by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521 when API_GRAPHQL_ENABLED; no in-repo client query found outside block_scout_web tests |
| <code>interface.14.http-graphql-query-token-transfers</code> | notes | forwarded by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521 when API_GRAPHQL_ENABLED; no in-repo client query found outside block_scout_web tests |
| <code>interface.14.http-graphql-query-transaction</code> | notes | forwarded by services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:521 when API_GRAPHQL_ENABLED; no in-repo client query found outside block_scout_web tests |

## market-history-chart

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-market-history-chart</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:48</code> | <code>GET /market-history-chart -&gt; Chain.MarketHistoryChartController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-market-history-chart</code> | notes | no in-repo caller found by path search |

## path

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-path</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:450</code> | <code>GET /*path -&gt; PageNotFoundController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-path</code> | notes | no in-repo caller found by path search |

## pending-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-pending-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:77</code> | <code>GET /pending-transactions -&gt; PendingTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-pending-transactions</code> | notes | no in-repo caller found by path search |

## recent-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-recent-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:79</code> | <code>GET /recent-transactions -&gt; RecentTransactionsController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-recent-transactions</code> | notes | no in-repo caller found by path search |

## reorgs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-reorgs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:73</code> | <code>GET /reorgs -&gt; BlockController.reorg</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-reorgs</code> | notes | no in-repo caller found by path search |

## robots-txt

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-robots-txt</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:93</code> | <code>GET /robots.txt -&gt; RobotsController.robots</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-robots-txt</code> | notes | no in-repo caller found by path search |

## root

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:46</code> | <code>GET / -&gt; ChainController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get</code> | notes | no in-repo caller found by path search |

## rpc-module-account

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-account-balance</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:71</code> | <code>GET&#124;POST /api?module=account&amp;action=balance -&gt; RPC.AddressController.balance</code> |
| <code>interface.14.http-api-rpc-account-balancemulti</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:89</code> | <code>GET&#124;POST /api?module=account&amp;action=balancemulti -&gt; RPC.AddressController.balancemulti</code> |
| <code>interface.14.http-api-rpc-account-eth-get-balance</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:42</code> | <code>GET&#124;POST /api?module=account&amp;action=eth_get_balance -&gt; RPC.AddressController.eth_get_balance</code> |
| <code>interface.14.http-api-rpc-account-getminedblocks</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:324</code> | <code>GET&#124;POST /api?module=account&amp;action=getminedblocks -&gt; RPC.AddressController.getminedblocks</code> |
| <code>interface.14.http-api-rpc-account-listaccounts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:23</code> | <code>GET&#124;POST /api?module=account&amp;action=listaccounts -&gt; RPC.AddressController.listaccounts</code> |
| <code>interface.14.http-api-rpc-account-pendingtxlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:93</code> | <code>GET&#124;POST /api?module=account&amp;action=pendingtxlist -&gt; RPC.AddressController.pendingtxlist</code> |
| <code>interface.14.http-api-rpc-account-tokenbalance</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:291</code> | <code>GET&#124;POST /api?module=account&amp;action=tokenbalance -&gt; RPC.AddressController.tokenbalance</code> |
| <code>interface.14.http-api-rpc-account-tokenlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:306</code> | <code>GET&#124;POST /api?module=account&amp;action=tokenlist -&gt; RPC.AddressController.tokenlist</code> |
| <code>interface.14.http-api-rpc-account-tokennfttx</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:246</code> | <code>GET&#124;POST /api?module=account&amp;action=tokennfttx -&gt; RPC.AddressController.tokennfttx</code> |
| <code>interface.14.http-api-rpc-account-tokentx</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:242</code> | <code>GET&#124;POST /api?module=account&amp;action=tokentx -&gt; RPC.AddressController.tokentx</code> |
| <code>interface.14.http-api-rpc-account-txlist</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:118</code> | <code>GET&#124;POST /api?module=account&amp;action=txlist -&gt; RPC.AddressController.txlist</code> |
| <code>interface.14.http-api-rpc-account-txlistinternal</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex:144</code> | <code>GET&#124;POST /api?module=account&amp;action=txlistinternal -&gt; RPC.AddressController.txlistinternal</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-account-balance</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-balancemulti</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-eth-get-balance</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |
| <code>interface.14.http-api-rpc-account-getminedblocks</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-listaccounts</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-pendingtxlist</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-tokenbalance</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-tokenlist</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-tokennfttx</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-tokentx</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-account-txlist</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); 26 test call sites, first 25 listed |
| <code>interface.14.http-api-rpc-account-txlistinternal</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |

## rpc-module-block

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-block-eth-block-number</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex:177</code> | <code>GET&#124;POST /api?module=block&amp;action=eth_block_number -&gt; RPC.BlockController.eth_block_number</code> |
| <code>interface.14.http-api-rpc-block-getblockcountdown</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex:78</code> | <code>GET&#124;POST /api?module=block&amp;action=getblockcountdown -&gt; RPC.BlockController.getblockcountdown</code> |
| <code>interface.14.http-api-rpc-block-getblocknobytime</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex:135</code> | <code>GET&#124;POST /api?module=block&amp;action=getblocknobytime -&gt; RPC.BlockController.getblocknobytime</code> |
| <code>interface.14.http-api-rpc-block-getblockreward</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex:36</code> | <code>GET&#124;POST /api?module=block&amp;action=getblockreward -&gt; RPC.BlockController.getblockreward</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-block-eth-block-number</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |
| <code>interface.14.http-api-rpc-block-getblockcountdown</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-block-getblocknobytime</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-block-getblockreward</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |

## rpc-module-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-contract-checkproxyverification</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:291</code> | <code>GET&#124;POST /api?module=contract&amp;action=checkproxyverification -&gt; RPC.ContractController.checkproxyverification</code> |
| <code>interface.14.http-api-rpc-contract-checkverifystatus</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:240</code> | <code>GET&#124;POST /api?module=contract&amp;action=checkverifystatus -&gt; RPC.ContractController.checkverifystatus</code> |
| <code>interface.14.http-api-rpc-contract-getabi</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:512</code> | <code>GET&#124;POST /api?module=contract&amp;action=getabi -&gt; RPC.ContractController.getabi</code> |
| <code>interface.14.http-api-rpc-contract-getcontractcreation</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:45</code> | <code>GET&#124;POST /api?module=contract&amp;action=getcontractcreation -&gt; RPC.ContractController.getcontractcreation</code> |
| <code>interface.14.http-api-rpc-contract-getsourcecode</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:529</code> | <code>GET&#124;POST /api?module=contract&amp;action=getsourcecode -&gt; RPC.ContractController.getsourcecode</code> |
| <code>interface.14.http-api-rpc-contract-listcontracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:491</code> | <code>GET&#124;POST /api?module=contract&amp;action=listcontracts -&gt; RPC.ContractController.listcontracts</code> |
| <code>interface.14.http-api-rpc-contract-verify</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:81</code> | <code>GET&#124;POST /api?module=contract&amp;action=verify -&gt; RPC.ContractController.verify</code> |
| <code>interface.14.http-api-rpc-contract-verify-via-sourcify</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:133</code> | <code>GET&#124;POST /api?module=contract&amp;action=verify_via_sourcify -&gt; RPC.ContractController.verify_via_sourcify</code> |
| <code>interface.14.http-api-rpc-contract-verify-vyper-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:449</code> | <code>GET&#124;POST /api?module=contract&amp;action=verify_vyper_contract -&gt; RPC.ContractController.verify_vyper_contract</code> |
| <code>interface.14.http-api-rpc-contract-verifyproxycontract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:256</code> | <code>GET&#124;POST /api?module=contract&amp;action=verifyproxycontract -&gt; RPC.ContractController.verifyproxycontract</code> |
| <code>interface.14.http-api-rpc-contract-verifysourcecode</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex:196</code> | <code>GET&#124;POST /api?module=contract&amp;action=verifysourcecode -&gt; RPC.ContractController.verifysourcecode</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-contract-checkproxyverification</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-checkverifystatus</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |
| <code>interface.14.http-api-rpc-contract-getabi</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-getcontractcreation</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-getsourcecode</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-listcontracts</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-verify</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); write action, requires API_V1_WRITE_METHODS_DISABLED unset |
| <code>interface.14.http-api-rpc-contract-verify-via-sourcify</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-verify-vyper-contract</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |
| <code>interface.14.http-api-rpc-contract-verifyproxycontract</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-contract-verifysourcecode</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |

## rpc-module-epoch

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-epoch-getepoch</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/celo_controller.ex:9</code> | <code>GET&#124;POST /api?module=epoch&amp;action=getepoch -&gt; RPC.CeloController.getepoch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-epoch-getepoch</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); module mapped only when chain identity is {:optimism, :celo} (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:596); not served for CHAIN_TYPE=paxeer_x; no in-repo caller found |

## rpc-module-logs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-logs-getlogs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/logs_controller.ex:6</code> | <code>GET&#124;POST /api?module=logs&amp;action=getlogs -&gt; RPC.LogsController.getlogs</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-logs-getlogs</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |

## rpc-module-stats

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-stats-coinprice</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:85</code> | <code>GET&#124;POST /api?module=stats&amp;action=coinprice -&gt; RPC.StatsController.coinprice</code> |
| <code>interface.14.http-api-rpc-stats-coinsupply</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:61</code> | <code>GET&#124;POST /api?module=stats&amp;action=coinsupply -&gt; RPC.StatsController.coinsupply</code> |
| <code>interface.14.http-api-rpc-stats-ethprice</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:79</code> | <code>GET&#124;POST /api?module=stats&amp;action=ethprice -&gt; RPC.StatsController.ethprice</code> |
| <code>interface.14.http-api-rpc-stats-ethsupply</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:55</code> | <code>GET&#124;POST /api?module=stats&amp;action=ethsupply -&gt; RPC.StatsController.ethsupply</code> |
| <code>interface.14.http-api-rpc-stats-ethsupplyexchange</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:44</code> | <code>GET&#124;POST /api?module=stats&amp;action=ethsupplyexchange -&gt; RPC.StatsController.ethsupplyexchange</code> |
| <code>interface.14.http-api-rpc-stats-tokensupply</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:10</code> | <code>GET&#124;POST /api?module=stats&amp;action=tokensupply -&gt; RPC.StatsController.tokensupply</code> |
| <code>interface.14.http-api-rpc-stats-totalfees</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex:112</code> | <code>GET&#124;POST /api?module=stats&amp;action=totalfees -&gt; RPC.StatsController.totalfees</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-stats-coinprice</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-stats-coinsupply</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-stats-ethprice</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |
| <code>interface.14.http-api-rpc-stats-ethsupply</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-stats-ethsupplyexchange</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-stats-tokensupply</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-stats-totalfees</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |

## rpc-module-token

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-token-gettoken</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/token_controller.ex:8</code> | <code>GET&#124;POST /api?module=token&amp;action=gettoken -&gt; RPC.TokenController.gettoken</code> |
| <code>interface.14.http-api-rpc-token-gettokenholders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/token_controller.ex:25</code> | <code>GET&#124;POST /api?module=token&amp;action=gettokenholders -&gt; RPC.TokenController.gettokenholders</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-token-gettoken</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |
| <code>interface.14.http-api-rpc-token-gettokenholders</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583); no in-repo caller found |

## rpc-module-transaction

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-api-rpc-transaction-getstatus</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex:59</code> | <code>GET&#124;POST /api?module=transaction&amp;action=getstatus -&gt; RPC.TransactionController.getstatus</code> |
| <code>interface.14.http-api-rpc-transaction-gettxinfo</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex:11</code> | <code>GET&#124;POST /api?module=transaction&amp;action=gettxinfo -&gt; RPC.TransactionController.gettxinfo</code> |
| <code>interface.14.http-api-rpc-transaction-gettxreceiptstatus</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex:45</code> | <code>GET&#124;POST /api?module=transaction&amp;action=gettxreceiptstatus -&gt; RPC.TransactionController.gettxreceiptstatus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-api-rpc-transaction-getstatus</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-transaction-gettxinfo</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |
| <code>interface.14.http-api-rpc-transaction-gettxreceiptstatus</code> | notes | dispatched by RPC.RPCTranslator (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/rpc_translator.ex:73) under /api and /api/v1 when API_V1_READ_METHODS_DISABLED is not set (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:551, :583) |

## schema-graphql

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-schema-graphql</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:105</code> | <code>GET /schema.graphql -&gt; GraphQL.SchemaController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-schema-graphql</code> | notes | guarded by if @graphql_enabled; no in-repo caller found by path search |

## search

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-search</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:432</code> | <code>GET /search -&gt; ChainController.search</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-search</code> | notes | no in-repo caller found by path search |

## search-logs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-search-logs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:434</code> | <code>GET /search-logs -&gt; AddressLogsController.search_logs</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-search-logs</code> | notes | no in-repo caller found by path search |

## search-results

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-search-results</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:436</code> | <code>GET /search-results -&gt; SearchController.search_results</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-search-results</code> | notes | no in-repo caller found by path search |

## search-verified-contracts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-search-verified-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:438</code> | <code>GET /search-verified-contracts -&gt; VerifiedContractsController.search_verified_contracts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-search-verified-contracts</code> | notes | no in-repo caller found by path search |

## sent-emails

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-forward-sent-emails</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:32</code> | <code>FORWARD /sent_emails -&gt; Bamboo.SentEmailViewerPlug</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-forward-sent-emails</code> | notes | guarded by if Mix.env() == :dev; plug forward; no in-repo caller found by path search |

## sitemap-xml

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-sitemap-xml</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:94</code> | <code>GET /sitemap.xml -&gt; RobotsController.sitemap</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-sitemap-xml</code> | notes | no in-repo caller found by path search |

## smart-contracts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-smart-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:423</code> | <code>GET /smart-contracts -&gt; SmartContractController.resources</code> |
| <code>interface.14.http-get-smart-contracts-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:423</code> | <code>GET /smart-contracts/:id -&gt; SmartContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-smart-contracts</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-smart-contracts-id</code> | notes | no in-repo caller found by path search |

## token

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:265</code> | <code>GET /token/:id -&gt; Tokens.TokenController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-id</code> | notes | no in-repo caller found by path search |

## token-autocomplete

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-autocomplete</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:442</code> | <code>GET /token-autocomplete -&gt; ChainController.token_autocomplete</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-autocomplete</code> | notes | no in-repo caller found by path search |

## token-counters

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-counters</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:446</code> | <code>GET /token-counters -&gt; Tokens.TokenController.token_counters</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-counters</code> | notes | no in-repo caller found by path search |

## token-instance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:315</code> | <code>GET /token/:token_id/instance/:id -&gt; Tokens.InstanceController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-id</code> | notes | no in-repo caller found by path search |

## token-instance-metadata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-metadata</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:328</code> | <code>GET /token/:token_id/instance/:instance_id/metadata -&gt; Tokens.Instance.MetadataController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-metadata</code> | notes | no in-repo caller found by path search |

## token-instance-token-holders

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-token-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:335</code> | <code>GET /token/:token_id/instance/:instance_id/token-holders -&gt; Tokens.Instance.HolderController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-token-holders</code> | notes | no in-repo caller found by path search |

## token-instance-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:321</code> | <code>GET /token/:token_id/instance/:instance_id/token-transfers -&gt; Tokens.Instance.TransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-token-transfers</code> | notes | no in-repo caller found by path search |

## token-inventory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-inventory</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:308</code> | <code>GET /token/:token_id/inventory -&gt; Tokens.InventoryController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-inventory</code> | notes | no in-repo caller found by path search |

## token-read-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-read-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:273</code> | <code>GET /token/:token_id/read-contract -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-read-contract</code> | notes | no in-repo caller found by path search |

## token-read-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-read-proxy</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:287</code> | <code>GET /token/:token_id/read-proxy -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-read-proxy</code> | notes | no in-repo caller found by path search |

## token-token-holders

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-token-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:301</code> | <code>GET /token/:token_id/token-holders -&gt; Tokens.HolderController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-token-holders</code> | notes | no in-repo caller found by path search |

## token-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:266</code> | <code>GET /token/:token_id/token-transfers -&gt; Tokens.TransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-token-transfers</code> | notes | no in-repo caller found by path search |

## token-write-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-write-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:280</code> | <code>GET /token/:token_id/write-contract -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-write-contract</code> | notes | no in-repo caller found by path search |

## token-write-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-token-token-id-write-proxy</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:294</code> | <code>GET /token/:token_id/write-proxy -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-token-token-id-write-proxy</code> | notes | no in-repo caller found by path search |

## tokens

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:117</code> | <code>GET /tokens -&gt; TokensController.resources</code> |
| <code>interface.14.http-get-tokens-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:344</code> | <code>GET /tokens/:id -&gt; Tokens.TokenController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens</code> | notes | no in-repo caller found by path search |
| <code>interface.14.http-get-tokens-id</code> | notes | no in-repo caller found by path search |

## tokens-instance

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:394</code> | <code>GET /tokens/:token_id/instance/:id -&gt; Tokens.InstanceController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-id</code> | notes | no in-repo caller found by path search |

## tokens-instance-metadata

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-metadata</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:407</code> | <code>GET /tokens/:token_id/instance/:instance_id/metadata -&gt; Tokens.Instance.MetadataController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-metadata</code> | notes | no in-repo caller found by path search |

## tokens-instance-token-holders

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-token-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:414</code> | <code>GET /tokens/:token_id/instance/:instance_id/token-holders -&gt; Tokens.Instance.HolderController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-token-holders</code> | notes | no in-repo caller found by path search |

## tokens-instance-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:400</code> | <code>GET /tokens/:token_id/instance/:instance_id/token-transfers -&gt; Tokens.Instance.TransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-token-transfers</code> | notes | no in-repo caller found by path search |

## tokens-inventory

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-inventory</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:387</code> | <code>GET /tokens/:token_id/inventory -&gt; Tokens.InventoryController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-inventory</code> | notes | no in-repo caller found by path search |

## tokens-read-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-read-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:352</code> | <code>GET /tokens/:token_id/read-contract -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-read-contract</code> | notes | no in-repo caller found by path search |

## tokens-read-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-read-proxy</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:366</code> | <code>GET /tokens/:token_id/read-proxy -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-read-proxy</code> | notes | no in-repo caller found by path search |

## tokens-token-holders

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-token-holders</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:380</code> | <code>GET /tokens/:token_id/token-holders -&gt; Tokens.HolderController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-token-holders</code> | notes | no in-repo caller found by path search |

## tokens-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:345</code> | <code>GET /tokens/:token_id/token-transfers -&gt; Tokens.TransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-token-transfers</code> | notes | no in-repo caller found by path search |

## tokens-write-contract

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-write-contract</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:359</code> | <code>GET /tokens/:token_id/write-contract -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-write-contract</code> | notes | no in-repo caller found by path search |

## tokens-write-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-write-proxy</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:373</code> | <code>GET /tokens/:token_id/write-proxy -&gt; Tokens.ContractController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tokens-token-id-write-proxy</code> | notes | no in-repo caller found by path search |

## transaction-history-chart

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-transaction-history-chart</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:53</code> | <code>GET /transaction-history-chart -&gt; Chain.TransactionHistoryChartController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-transaction-history-chart</code> | notes | no in-repo caller found by path search |

## tx

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tx-id</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:87</code> | <code>GET /tx/:id -&gt; TransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tx-id</code> | notes | no in-repo caller found by path search |

## tx-internal-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-internal-transactions</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:88</code> | <code>GET /tx/:transaction_id/internal-transactions -&gt; TransactionInternalTransactionController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-internal-transactions</code> | notes | no in-repo caller found by path search |

## tx-logs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-logs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:102</code> | <code>GET /tx/:transaction_id/logs -&gt; TransactionLogController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-logs</code> | notes | no in-repo caller found by path search |

## tx-raw-trace

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-raw-trace</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:95</code> | <code>GET /tx/:transaction_id/raw-trace -&gt; TransactionRawTraceController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-raw-trace</code> | notes | no in-repo caller found by path search |

## tx-state

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-state</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:109</code> | <code>GET /tx/:transaction_id/state -&gt; TransactionStateController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-state</code> | notes | no in-repo caller found by path search |

## tx-token-transfers

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-token-transfers</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:104</code> | <code>GET /tx/:transaction_id/token-transfers -&gt; TransactionTokenTransferController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-token-transfers</code> | notes | no in-repo caller found by path search |

## txs

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-txs</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:85</code> | <code>GET /txs -&gt; TransactionController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-txs</code> | notes | no in-repo caller found by path search |

## uncles

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-uncles</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:75</code> | <code>GET /uncles -&gt; BlockController.uncle</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-uncles</code> | notes | no in-repo caller found by path search |

## v2-paxeer-x

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-paxeer-x-archive-anchors</code> | <code>http_route</code> | <code>new</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex</code> (planned) | <code>GET /api/v2/paxeer-x/archive/anchors -&gt; V2.PaxeerX.ArchiveController (planned route string)</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-archive-receipts</code> | <code>http_route</code> | <code>new</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex</code> (planned) | <code>GET /api/v2/paxeer-x/archive/receipts -&gt; V2.PaxeerX.ArchiveController (planned route string)</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-path-not-found</code> | <code>http_route</code> | <code>new</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex</code> (planned) | <code>GET /api/v2/paxeer-x/*path -&gt; 404</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-paxeer-x-archive-anchors</code> | notes | stale spec task 8.6.1: read-only pre-activation anchors with a frozen marker; the task names no exact route string |
| <code>interface.14.http-get-api-v2-paxeer-x-archive-receipts</code> | notes | stale spec task 8.6.1: read-only pre-activation kernel receipts with a frozen marker; the task names no exact route string |
| <code>interface.14.http-get-api-v2-paxeer-x-path-not-found</code> | notes | stale spec task 8.7.20: explicit 404 catch-all in the /paxeer-x scope, because the /api/v2/*path fallback (services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:573) answers 400 |

Contract context <code>contract_context.14.paxeer-x-archive</code>:

| Field | Frozen rule |
|---|---|
| interface | GET /api/v2/paxeer-x/archive/anchors, /archive/receipts and /archive/receipts/:id (planned) |
| storage | Reads lx_anchors and lx_receipts; no writes. |
| errors | 404 for an unknown receipt id and for any other /api/v2/paxeer-x path. |
| events | none |
| gas | none (off-chain service) |
| activation | Archive routes land in wave 3; live routes retire in wave 6. |
| invariants | Every archive item carries frozen true and no settlement rung. |
| pause | none |

## v2-transactions

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-signature</code> | <code>http_route</code> | <code>new</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller.ex</code> (planned) | <code>GET /api/v2/transactions/:transaction_hash_param/signature -&gt; V2.PaxeerX.TransactionSignatureController.signature</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-signature</code> | notes | stale spec task 8.5.37 (in_progress, not landed): route beside services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex:192 answering rung, receipt bytes, receiptKey and the 64-byte Ed25519 signature |

Contract context <code>contract_context.14.paxeer-x-signature</code>:

| Field | Frozen rule |
|---|---|
| interface | GET /api/v2/transactions/:transaction_hash_param/signature (planned) |
| storage | none; reads the router answers per request. |
| errors | 404 unless verification reaches receipted or committed. |
| events | none |
| gas | none (off-chain service) |
| activation | Wave 2 for R1-R6, wave 3 for the committed rung. |
| invariants | The signature is the 64-byte Ed25519 signature the pinned receipt key verifies. |
| pause | none |

## verified-contracts

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-verified-contracts</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:81</code> | <code>GET /verified-contracts -&gt; VerifiedContractsController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-verified-contracts</code> | notes | no in-repo caller found by path search |

## verify-smart-contract-contract-verifications

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-post-verify-smart-contract-contract-verifications</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex:112</code> | <code>POST /verify_smart_contract/contract_verifications -&gt; BlockScoutWeb.AddressContractVerificationController.create</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-post-verify-smart-contract-contract-verifications</code> | notes | no in-repo caller found by path search |

## visualize-sol2uml

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-visualize-sol2uml</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:448</code> | <code>GET /visualize/sol2uml -&gt; VisualizeSol2umlController.index</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-visualize-sol2uml</code> | notes | no in-repo caller found by path search |

## withdrawals

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.http-get-withdrawals</code> | <code>http_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex:83</code> | <code>GET /withdrawals -&gt; WithdrawalController.resources</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.http-get-withdrawals</code> | notes | no in-repo caller found by path search |
