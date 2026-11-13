# Explorer HTTP API contract

The explorer serves HTTP routes through the [Phoenix routers](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers). The tables include routes selected by chain type and feature configuration; registration and availability depend on those conditions and the configured pipelines.

## account-api-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-account-api-key-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /account/api_key/:id -&gt; Account.ApiKeyController.delete</code> |
| <code>interface.14.http-get-account-api-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/api_key -&gt; Account.ApiKeyController.index</code> |
| <code>interface.14.http-patch-put-account-api-key-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PATCH&#124;PUT /account/api_key/:id -&gt; Account.ApiKeyController.update</code> |
| <code>interface.14.http-post-account-api-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /account/api_key -&gt; Account.ApiKeyController.create</code> |

## account-api-key-edit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-api-key-id-edit</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/api_key/:id/edit -&gt; Account.ApiKeyController.edit</code> |

## account-api-key-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-api-key-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/api_key/new -&gt; Account.ApiKeyController.new</code> |

## account-custom-abi

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-account-custom-abi-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /account/custom_abi/:id -&gt; Account.CustomABIController.delete</code> |
| <code>interface.14.http-get-account-custom-abi</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/custom_abi -&gt; Account.CustomABIController.index</code> |
| <code>interface.14.http-patch-put-account-custom-abi-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PATCH&#124;PUT /account/custom_abi/:id -&gt; Account.CustomABIController.update</code> |
| <code>interface.14.http-post-account-custom-abi</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /account/custom_abi -&gt; Account.CustomABIController.create</code> |

## account-custom-abi-edit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-custom-abi-id-edit</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/custom_abi/:id/edit -&gt; Account.CustomABIController.edit</code> |

## account-custom-abi-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-custom-abi-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/custom_abi/new -&gt; Account.CustomABIController.new</code> |

## account-tag-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-account-tag-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /account/tag_address/:id -&gt; Account.TagAddressController.delete</code> |
| <code>interface.14.http-get-account-tag-address</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/tag_address -&gt; Account.TagAddressController.index</code> |
| <code>interface.14.http-post-account-tag-address</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /account/tag_address -&gt; Account.TagAddressController.create</code> |

## account-tag-address-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-tag-address-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/tag_address/new -&gt; Account.TagAddressController.new</code> |

## account-tag-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-account-tag-transaction-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /account/tag_transaction/:id -&gt; Account.TagTransactionController.delete</code> |
| <code>interface.14.http-get-account-tag-transaction</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/tag_transaction -&gt; Account.TagTransactionController.index</code> |
| <code>interface.14.http-post-account-tag-transaction</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /account/tag_transaction -&gt; Account.TagTransactionController.create</code> |

## account-tag-transaction-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-tag-transaction-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/tag_transaction/new -&gt; Account.TagTransactionController.new</code> |

## account-watchlist

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-watchlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/watchlist -&gt; Account.WatchlistController.show</code> |

## account-watchlist-address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-account-watchlist-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /account/watchlist_address/:id -&gt; Account.WatchlistAddressController.delete</code> |
| <code>interface.14.http-patch-put-account-watchlist-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PATCH&#124;PUT /account/watchlist_address/:id -&gt; Account.WatchlistAddressController.update</code> |
| <code>interface.14.http-post-account-watchlist-address</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /account/watchlist_address -&gt; Account.WatchlistAddressController.create</code> |

## account-watchlist-address-edit

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-watchlist-address-id-edit</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/watchlist_address/:id/edit -&gt; Account.WatchlistAddressController.edit</code> |

## account-watchlist-address-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-account-watchlist-address-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /account/watchlist_address/new -&gt; Account.WatchlistAddressController.new</code> |

## accounts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-accounts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /accounts -&gt; AddressController.index</code> |

## address

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:id -&gt; AddressController.show</code> |

## address-coin-balances

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-coin-balances</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/coin-balances -&gt; AddressCoinBalanceController.index</code> |

## address-coin-balances-by-day

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-coin-balances-by-day</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/coin-balances/by-day -&gt; AddressCoinBalanceByDayController.index</code> |

## address-contract-verifications-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-contract-verifications-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/contract_verifications/new -&gt; AddressContractVerificationController.new</code> |

## address-contracts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/contracts -&gt; AddressContractController.index</code> |

## address-counters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address-counters -&gt; AddressController.address_counters</code> |

## address-internal-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-internal-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/internal-transactions -&gt; AddressInternalTransactionController.index</code> |

## address-logs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-logs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/logs -&gt; AddressLogsController.index</code> |

## address-read-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-read-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/read-contract -&gt; AddressReadContractController.index</code> |

## address-read-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-read-proxy</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/read-proxy -&gt; AddressReadProxyController.index</code> |

## address-token-balances

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-token-balances</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/token-balances -&gt; AddressTokenBalanceController.index</code> |

## address-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/token-transfers -&gt; AddressTokenTransferController.index</code> |

## address-tokens

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-tokens</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/tokens -&gt; AddressTokenController.index</code> |

## address-tokens-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-tokens-address-token-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/tokens/:address_token_id/token-transfers -&gt; AddressTokenTransferController.index</code> |

## address-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/transactions -&gt; AddressTransactionController.index</code> |

## address-validations

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-validations</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/validations -&gt; AddressValidationController.index</code> |

## address-verify-via-flattened-code-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-flattened-code-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/verify-via-flattened-code/new -&gt; AddressContractVerificationViaFlattenedCodeController.new</code> |

## address-verify-via-metadata-json-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-metadata-json-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/verify-via-metadata-json/new -&gt; AddressContractVerificationViaJsonController.new</code> |

## address-verify-via-multi-part-files-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-multi-part-files-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/verify-via-multi-part-files/new -&gt; AddressContractVerificationViaMultiPartFilesController.new</code> |

## address-verify-via-standard-json-input-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-via-standard-json-input-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/verify-via-standard-json-input/new -&gt; AddressContractVerificationViaStandardJsonInputController.new</code> |

## address-verify-vyper-contract-new

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-verify-vyper-contract-new</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/verify-vyper-contract/new -&gt; AddressContractVerificationVyperController.new</code> |

## address-withdrawals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/withdrawals -&gt; AddressWithdrawalController.index</code> |

## address-write-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-write-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/write-contract -&gt; AddressWriteContractController.index</code> |

## address-write-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-address-address-id-write-proxy</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /address/:address_id/write-proxy -&gt; AddressWriteProxyController.index</code> |

## admin

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-admin</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex) | <code>GET /admin -&gt; DashboardController.index</code> |

## admin-login

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-admin-login</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex) | <code>GET /admin/login -&gt; SessionController.new</code> |
| <code>interface.14.http-post-admin-login</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex) | <code>POST /admin/login -&gt; SessionController.create</code> |

## admin-setup

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-admin-setup</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex) | <code>GET /admin/setup -&gt; SetupController.configure</code> |
| <code>interface.14.http-post-admin-setup</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex) | <code>POST /admin/setup -&gt; SetupController.configure_admin</code> |

## admin-tasks-create-contract-methods

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-admin-tasks-create-contract-methods</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/admin_router.ex) | <code>GET /admin/tasks/create_contract_methods -&gt; TaskController.create_contract_methods</code> |

## api-account-v2

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-api-account-v2-user-api-keys-api-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /api/account/v2/user/api_keys/:api_key -&gt; UserController.delete_api_key</code> |
| <code>interface.14.http-delete-api-account-v2-user-custom-abis-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /api/account/v2/user/custom_abis/:id -&gt; UserController.delete_custom_abi</code> |
| <code>interface.14.http-delete-api-account-v2-user-tags-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /api/account/v2/user/tags/address/:id -&gt; UserController.delete_tag_address</code> |
| <code>interface.14.http-delete-api-account-v2-user-tags-transaction-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /api/account/v2/user/tags/transaction/:id -&gt; UserController.delete_tag_transaction</code> |
| <code>interface.14.http-delete-api-account-v2-user-watchlist-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>DELETE /api/account/v2/user/watchlist/:id -&gt; UserController.delete_watchlist</code> |
| <code>interface.14.http-get-api-account-v2-authenticate</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/authenticate -&gt; AuthenticateController.authenticate_get</code> |
| <code>interface.14.http-get-api-account-v2-authenticate-via-dynamic</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/authenticate_via_dynamic -&gt; AuthenticateController.authenticate_via_dynamic</code> |
| <code>interface.14.http-get-api-account-v2-email-resend</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/email/resend -&gt; EmailController.resend_email</code> |
| <code>interface.14.http-get-api-account-v2-get-csrf</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/get_csrf -&gt; UserController.get_csrf</code> |
| <code>interface.14.http-get-api-account-v2-siwe-message</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/siwe_message -&gt; AuthenticateController.siwe_message</code> |
| <code>interface.14.http-get-api-account-v2-tags-address-address-hash</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/tags/address/:address_hash -&gt; TagsController.tags_address</code> |
| <code>interface.14.http-get-api-account-v2-tags-transaction-transaction-hash</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/tags/transaction/:transaction_hash -&gt; TagsController.tags_transaction</code> |
| <code>interface.14.http-get-api-account-v2-user-api-keys</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/api_keys -&gt; UserController.api_keys</code> |
| <code>interface.14.http-get-api-account-v2-user-custom-abis</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/custom_abis -&gt; UserController.custom_abis</code> |
| <code>interface.14.http-get-api-account-v2-user-info</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/info -&gt; UserController.info</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-address</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/tags/address/ -&gt; UserController.tags_address</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/tags/address/:id -&gt; UserController.tags_address</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-transaction</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/tags/transaction/ -&gt; UserController.tags_transaction</code> |
| <code>interface.14.http-get-api-account-v2-user-tags-transaction-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/tags/transaction/:id -&gt; UserController.tags_transaction</code> |
| <code>interface.14.http-get-api-account-v2-user-watchlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /api/account/v2/user/watchlist -&gt; UserController.watchlist</code> |
| <code>interface.14.http-post-api-account-v2-address-link</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/address/link -&gt; AddressController.link_address</code> |
| <code>interface.14.http-post-api-account-v2-authenticate</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/authenticate -&gt; AuthenticateController.authenticate_post</code> |
| <code>interface.14.http-post-api-account-v2-authenticate-via-wallet</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/authenticate_via_wallet -&gt; AuthenticateController.authenticate_via_wallet</code> |
| <code>interface.14.http-post-api-account-v2-confirm-otp</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/confirm_otp -&gt; AuthenticateController.confirm_otp</code> |
| <code>interface.14.http-post-api-account-v2-email-link</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/email/link -&gt; EmailController.link_email</code> |
| <code>interface.14.http-post-api-account-v2-send-otp</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/send_otp -&gt; AuthenticateController.send_otp</code> |
| <code>interface.14.http-post-api-account-v2-user-api-keys</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/user/api_keys -&gt; UserController.create_api_key</code> |
| <code>interface.14.http-post-api-account-v2-user-custom-abis</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/user/custom_abis -&gt; UserController.create_custom_abi</code> |
| <code>interface.14.http-post-api-account-v2-user-tags-address</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/user/tags/address/ -&gt; UserController.create_tag_address</code> |
| <code>interface.14.http-post-api-account-v2-user-tags-transaction</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/user/tags/transaction/ -&gt; UserController.create_tag_transaction</code> |
| <code>interface.14.http-post-api-account-v2-user-watchlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>POST /api/account/v2/user/watchlist -&gt; UserController.create_watchlist</code> |
| <code>interface.14.http-put-api-account-v2-user-api-keys-api-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PUT /api/account/v2/user/api_keys/:api_key -&gt; UserController.update_api_key</code> |
| <code>interface.14.http-put-api-account-v2-user-custom-abis-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PUT /api/account/v2/user/custom_abis/:id -&gt; UserController.update_custom_abi</code> |
| <code>interface.14.http-put-api-account-v2-user-tags-address-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PUT /api/account/v2/user/tags/address/:id -&gt; UserController.update_tag_address</code> |
| <code>interface.14.http-put-api-account-v2-user-tags-transaction-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PUT /api/account/v2/user/tags/transaction/:id -&gt; UserController.update_tag_transaction</code> |
| <code>interface.14.http-put-api-account-v2-user-watchlist-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>PUT /api/account/v2/user/watchlist/:id -&gt; UserController.update_watchlist</code> |

## api-docs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-docs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /api-docs -&gt; APIDocsController.index</code> |
| <code>interface.14.http-get-api-docs-l100</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /api-docs -&gt; PageNotFoundController.index</code> |

## api-eth-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-post-api-eth-rpc</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>POST /api/eth-rpc -&gt; EthRPC.EthController.eth_request</code> |

## api-health

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-health</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex) | <code>GET /api/health -&gt; BlockScoutWeb.API.HealthController.health</code> |
| <code>interface.14.http-get-api-health-l564</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/health -&gt; BlockScoutWeb.API.HealthController.health</code> |

## api-health-liveness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-health-liveness</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex) | <code>GET /api/health/liveness -&gt; BlockScoutWeb.API.HealthController.liveness</code> |
| <code>interface.14.http-get-api-health-liveness-l565</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/health/liveness -&gt; BlockScoutWeb.API.HealthController.liveness</code> |

## api-health-multichain-search-export

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-health-multichain-search-export</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/health/multichain-search-export -&gt; BlockScoutWeb.API.HealthController.multichain_search_db_export</code> |

## api-health-readiness

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-health-readiness</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/health_router.ex) | <code>GET /api/health/readiness -&gt; BlockScoutWeb.API.HealthController.readiness</code> |
| <code>interface.14.http-get-api-health-readiness-l566</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/health/readiness -&gt; BlockScoutWeb.API.HealthController.readiness</code> |

## api-v1-eth-rpc

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-post-api-v1-eth-rpc</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>POST /api/v1/eth-rpc -&gt; EthRPC.EthController.eth_request</code> |

## api-v1-gas-price-oracle

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v1-gas-price-oracle</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v1/gas-price-oracle -&gt; GasPriceOracleController.gas_price_oracle</code> |

## api-v1-graphql

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-forward-api-v1-graphql</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>FORWARD /api/v1/graphql -&gt; Absinthe.Plug</code> |

## api-v1-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v1-search</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v1/search -&gt; SearchController.search</code> |

## api-v1-supply

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v1-supply</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v1/supply -&gt; V1.SupplyController.supply</code> |

## api-v1-verified-smart-contracts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-post-api-v1-verified-smart-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>POST /api/v1/verified_smart_contracts -&gt; V1.VerifiedSmartContractController.create</code> |

## api-v2-addresses

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-addresses</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses -&gt; V2.AddressController.addresses_list</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param -&gt; V2.AddressController.address</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-beacon-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/beacon/deposits -&gt; V2.AddressController.beacon_deposits</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-blocks-validated</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/blocks-validated -&gt; V2.AddressController.blocks_validated</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-celo-election-rewards</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/celo/election-rewards -&gt; V2.AddressController.celo_election_rewards</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-celo-election-rewards-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/celo/election-rewards/csv -&gt; V2.CsvExportController.celo_election_rewards_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-coin-balance-history</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/coin-balance-history -&gt; V2.AddressController.coin_balance_history</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-coin-balance-history-by-day</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/coin-balance-history-by-day -&gt; V2.AddressController.coin_balance_history_by_day</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/counters -&gt; V2.AddressController.counters</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-internal-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/internal-transactions -&gt; V2.AddressController.internal_transactions</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-internal-transactions-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/internal-transactions/csv -&gt; V2.CsvExportController.internal_transactions_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-logs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/logs -&gt; V2.AddressController.logs</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-logs-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/logs/csv -&gt; V2.CsvExportController.logs_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-nft</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/nft -&gt; V2.AddressController.nft_list</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-nft-collections</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/nft/collections -&gt; V2.AddressController.nft_collections</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-tabs-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/tabs-counters -&gt; V2.AddressController.tabs_counters</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-token-balances</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/token-balances -&gt; V2.AddressController.token_balances</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/token-transfers -&gt; V2.AddressController.token_transfers</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-token-transfers-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/token-transfers/csv -&gt; V2.CsvExportController.token_transfers_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-tokens</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/tokens -&gt; V2.AddressController.tokens</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/transactions -&gt; V2.AddressController.transactions</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-transactions-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/transactions/csv -&gt; V2.CsvExportController.transactions_csv</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-unified</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/unified -&gt; V2.PaxeerX.UnifiedAccountController.unified</code> |
| <code>interface.14.http-get-api-v2-addresses-address-hash-param-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/addresses/:address_hash_param/withdrawals -&gt; V2.AddressController.withdrawals</code> |

## api-v2-advanced-filters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-advanced-filters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/advanced-filters -&gt; V2.AdvancedFilterController.list</code> |
| <code>interface.14.http-get-api-v2-advanced-filters-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/advanced-filters/csv -&gt; V2.AdvancedFilterController.list_csv</code> |
| <code>interface.14.http-get-api-v2-advanced-filters-methods</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/advanced-filters/methods -&gt; V2.AdvancedFilterController.list_methods</code> |

## api-v2-arbitrum

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-arbitrum-batches</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/batches -&gt; V2.ArbitrumController.batches</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-batch-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/batches/:batch_number -&gt; V2.ArbitrumController.batch</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/batches/count -&gt; V2.ArbitrumController.batches_count</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-anytrust-data-hash</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/batches/da/anytrust/:data_hash -&gt; V2.ArbitrumController.batch_by_data_availability_info</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-celestia-height-transaction-commitment</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/batches/da/celestia/:height/:transaction_commitment -&gt; V2.ArbitrumController.batch_by_data_availability_info</code> |
| <code>interface.14.http-get-api-v2-arbitrum-batches-da-eigenda-data-hash</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/batches/da/eigenda/:data_hash -&gt; V2.ArbitrumController.batch_by_data_availability_info</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-claim-message-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/messages/claim/:message_id -&gt; V2.ArbitrumController.claim_message</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-direction</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/messages/:direction -&gt; V2.ArbitrumController.messages</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-direction-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/messages/:direction/count -&gt; V2.ArbitrumController.messages_count</code> |
| <code>interface.14.http-get-api-v2-arbitrum-messages-withdrawals-transaction-hash</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/arbitrum/messages/withdrawals/:transaction_hash -&gt; V2.ArbitrumController.withdrawals</code> |

## api-v2-beacon

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-beacon-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/beacon/deposits -&gt; V2.Ethereum.DepositController.list</code> |
| <code>interface.14.http-get-api-v2-beacon-deposits-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/beacon/deposits/count -&gt; V2.Ethereum.DepositController.count</code> |

## api-v2-blobs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-blobs-blob-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blobs/:blob_hash_param -&gt; V2.BlobController.blob</code> |

## api-v2-blocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-blocks</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks -&gt; V2.BlockController.blocks</code> |
| <code>interface.14.http-get-api-v2-blocks-arbitrum-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/arbitrum-batch/:batch_number_param -&gt; V2.BlockController.arbitrum_batch</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/:block_hash_or_number_param -&gt; V2.BlockController.block</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-beacon-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/:block_hash_or_number_param/beacon/deposits -&gt; V2.BlockController.beacon_deposits</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-internal-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/:block_hash_or_number_param/internal-transactions -&gt; V2.BlockController.internal_transactions</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/:block_hash_or_number_param/transactions -&gt; V2.BlockController.transactions</code> |
| <code>interface.14.http-get-api-v2-blocks-block-hash-or-number-param-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/:block_hash_or_number_param/withdrawals -&gt; V2.BlockController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-blocks-block-number-param-countdown</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/:block_number_param/countdown -&gt; V2.BlockController.block_countdown</code> |
| <code>interface.14.http-get-api-v2-blocks-optimism-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/optimism-batch/:batch_number_param -&gt; V2.BlockController.optimism_batch</code> |
| <code>interface.14.http-get-api-v2-blocks-scroll-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/blocks/scroll-batch/:batch_number_param -&gt; V2.BlockController.scroll_batch</code> |

## api-v2-celo

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-celo-epochs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/celo/epochs -&gt; V2.CeloController.epochs</code> |
| <code>interface.14.http-get-api-v2-celo-epochs-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/celo/epochs/:number -&gt; V2.CeloController.epoch</code> |
| <code>interface.14.http-get-api-v2-celo-epochs-number-election-rewards-type</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/celo/epochs/:number/election-rewards/:type -&gt; V2.CeloController.election_rewards</code> |

## api-v2-config

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-config-backend</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/backend -&gt; V2.ConfigController.backend</code> |
| <code>interface.14.http-get-api-v2-config-backend-version</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/backend-version -&gt; V2.ConfigController.backend_version</code> |
| <code>interface.14.http-get-api-v2-config-celo</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/celo -&gt; V2.ConfigController.celo</code> |
| <code>interface.14.http-get-api-v2-config-csv-export</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/csv-export -&gt; V2.ConfigController.csv_export</code> |
| <code>interface.14.http-get-api-v2-config-db-background-migrations</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/db-background-migrations -&gt; V2.ConfigController.db_background_migrations</code> |
| <code>interface.14.http-get-api-v2-config-indexer</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/indexer -&gt; V2.ConfigController.indexer</code> |
| <code>interface.14.http-get-api-v2-config-public-metrics</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/public-metrics -&gt; V2.ConfigController.public_metrics</code> |
| <code>interface.14.http-get-api-v2-config-smart-contracts-languages</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/config/smart-contracts/languages -&gt; V2.ConfigController.languages_list</code> |

## api-v2-import

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-api-v2-import-token-info</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>DELETE /api/v2/import/token-info -&gt; V2.ImportController.delete_token_info</code> |
| <code>interface.14.http-get-api-v2-import-smart-contracts-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/import/smart-contracts/:address_hash_param -&gt; V2.ImportController.try_to_search_contract</code> |
| <code>interface.14.http-post-api-v2-import-optimism-interop</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>POST /api/v2/import/optimism/interop/ -&gt; V2.OptimismController.interop_import</code> |
| <code>interface.14.http-post-api-v2-import-smart-contracts-address-hash-param-audit-reports</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>POST /api/v2/import/smart-contracts/:address_hash_param/audit-reports -&gt; V2.ImportController.import_audit_report</code> |
| <code>interface.14.http-post-api-v2-import-token-info</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>POST /api/v2/import/token-info -&gt; V2.ImportController.import_token_info</code> |

## api-v2-internal-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-internal-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/internal-transactions -&gt; V2.InternalTransactionController.internal_transactions</code> |

## api-v2-key

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-post-api-v2-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_key_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_key_v2_router.ex) | <code>POST /api/v2/key -&gt; V2.APIKeyController.get_key</code> |

## api-v2-main-page

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-main-page-arbitrum-batches-committed</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/arbitrum/batches/committed -&gt; V2.ArbitrumController.batches_committed</code> |
| <code>interface.14.http-get-api-v2-main-page-arbitrum-batches-latest-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/arbitrum/batches/latest-number -&gt; V2.ArbitrumController.batch_latest_number</code> |
| <code>interface.14.http-get-api-v2-main-page-arbitrum-messages-to-rollup</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/arbitrum/messages/to-rollup -&gt; V2.ArbitrumController.recent_messages_to_l2</code> |
| <code>interface.14.http-get-api-v2-main-page-blocks</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/blocks -&gt; V2.MainPageController.blocks</code> |
| <code>interface.14.http-get-api-v2-main-page-indexing-status</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/indexing-status -&gt; V2.MainPageController.indexing_status</code> |
| <code>interface.14.http-get-api-v2-main-page-optimism-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/optimism-deposits -&gt; V2.OptimismController.main_page_deposits</code> |
| <code>interface.14.http-get-api-v2-main-page-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/transactions -&gt; V2.MainPageController.transactions</code> |
| <code>interface.14.http-get-api-v2-main-page-transactions-watchlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/transactions/watchlist -&gt; V2.MainPageController.watchlist_transactions</code> |
| <code>interface.14.http-get-api-v2-main-page-zkevm-batches-confirmed</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/zkevm/batches/confirmed -&gt; V2.PolygonZkevmController.batches_confirmed</code> |
| <code>interface.14.http-get-api-v2-main-page-zkevm-batches-latest-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/zkevm/batches/latest-number -&gt; V2.PolygonZkevmController.batch_latest_number</code> |
| <code>interface.14.http-get-api-v2-main-page-zksync-batches-confirmed</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/zksync/batches/confirmed -&gt; V2.ZkSyncController.batches_confirmed</code> |
| <code>interface.14.http-get-api-v2-main-page-zksync-batches-latest-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/main-page/zksync/batches/latest-number -&gt; V2.ZkSyncController.batch_latest_number</code> |

## api-v2-mud

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-mud-worlds</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds -&gt; V2.MudController.worlds</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/count -&gt; V2.MudController.worlds_count</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-systems</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/systems -&gt; V2.MudController.world_systems</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-systems-system</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/systems/:system -&gt; V2.MudController.world_system</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/tables -&gt; V2.MudController.world_tables</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/tables/count -&gt; V2.MudController.world_tables_count</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/tables/:table_id/records -&gt; V2.MudController.world_table_records</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/tables/:table_id/records/count -&gt; V2.MudController.world_table_records_count</code> |
| <code>interface.14.http-get-api-v2-mud-worlds-world-tables-table-id-records-record-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/mud/worlds/:world/tables/:table_id/records/:record_id -&gt; V2.MudController.world_table_record</code> |

## api-v2-openapi

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-openapi</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/openapi -&gt; OpenApiSpex.Plug.RenderSpec</code> |

## api-v2-optimism

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-optimism-batches</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/batches -&gt; V2.OptimismController.batches</code> |
| <code>interface.14.http-get-api-v2-optimism-batches-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/batches/count -&gt; V2.OptimismController.batches_count</code> |
| <code>interface.14.http-get-api-v2-optimism-batches-da-celestia-height-commitment</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/batches/da/celestia/:height/:commitment -&gt; V2.OptimismController.batch_by_celestia_blob</code> |
| <code>interface.14.http-get-api-v2-optimism-batches-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/batches/:number -&gt; V2.OptimismController.batch_by_number</code> |
| <code>interface.14.http-get-api-v2-optimism-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/deposits -&gt; V2.OptimismController.deposits</code> |
| <code>interface.14.http-get-api-v2-optimism-deposits-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/deposits/count -&gt; V2.OptimismController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-optimism-games</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/games -&gt; V2.OptimismController.games</code> |
| <code>interface.14.http-get-api-v2-optimism-games-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/games/count -&gt; V2.OptimismController.games_count</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-messages</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/interop/messages -&gt; V2.OptimismController.interop_messages</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-messages-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/interop/messages/count -&gt; V2.OptimismController.interop_messages_count</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-messages-unique-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/interop/messages/:unique_id -&gt; V2.OptimismController.interop_message</code> |
| <code>interface.14.http-get-api-v2-optimism-interop-public-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/interop/public-key -&gt; V2.OptimismController.interop_public_key</code> |
| <code>interface.14.http-get-api-v2-optimism-output-roots</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/output-roots -&gt; V2.OptimismController.output_roots</code> |
| <code>interface.14.http-get-api-v2-optimism-output-roots-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/output-roots/count -&gt; V2.OptimismController.output_roots_count</code> |
| <code>interface.14.http-get-api-v2-optimism-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/withdrawals -&gt; V2.OptimismController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-optimism-withdrawals-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/optimism/withdrawals/count -&gt; V2.OptimismController.withdrawals_count</code> |

## api-v2-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-path</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/*path -&gt; BlockScoutWeb.API.V2.FallbackController.call(conn, :unknown_action)</code> |

## api-v2-paxeer-x

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-paxeer-x-archive-receipts-id-j54</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex) | <code>GET /api/v2/paxeer-x/archive/receipts/:id -&gt; V2.PaxeerX.ArchiveController.receipt</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-capabilities</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/paxeer-x/capabilities -&gt; V2.PaxeerX.CapabilitiesController.capabilities</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-markets</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/paxeer-x/feed/markets -&gt; V2.PaxeerX.FeedController.markets</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-markets-market-id-updates</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/paxeer-x/feed/markets/:market_id/updates -&gt; V2.PaxeerX.FeedController.market_updates</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-updates</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/paxeer-x/feed/updates -&gt; V2.PaxeerX.FeedController.updates</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-feed-updates-hash</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/paxeer-x/feed/updates/:hash -&gt; V2.PaxeerX.FeedController.update</code> |

| Field | Rule |
|---|---|
| interface | GET /api/v2/paxeer-x/archive/anchors, /archive/receipts and /archive/receipts/:id |
| storage | Reads lx_anchors and lx_receipts; no writes. |
| errors | 404 for an unknown receipt id and for any other /api/v2/paxeer-x path. |
| events | none |
| gas | none (off-chain service) |
| invariants | Every archive item carries frozen true and no settlement rung. |
| pause | none |

## api-v2-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-proxy-3rdparty-noves-fi-addresses-address-hash-param-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/3rdparty/noves-fi/addresses/:address_hash_param/transactions -&gt; V2.Proxy.NovesFiController.address_transactions</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-noves-fi-transaction-descriptions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/3rdparty/noves-fi/transaction-descriptions -&gt; V2.Proxy.NovesFiController.describe_transactions</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-noves-fi-transactions-transaction-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/3rdparty/noves-fi/transactions/:transaction_hash_param -&gt; V2.Proxy.NovesFiController.transaction</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-platform-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/3rdparty/:platform_id -&gt; V2.Proxy.UniversalProxyController.index</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-solidityscan-smart-contracts-address-hash-report</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/3rdparty/solidityscan/smart-contracts/:address_hash/report -&gt; V2.Proxy.SolidityScanController.solidityscan_report</code> |
| <code>interface.14.http-get-api-v2-proxy-3rdparty-xname-addresses-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/3rdparty/xname/addresses/:address_hash_param -&gt; V2.Proxy.XnameController.address</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-accounts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/accounts -&gt; V2.Proxy.AccountAbstractionController.accounts</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-accounts-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/accounts/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.account</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-bundlers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/bundlers -&gt; V2.Proxy.AccountAbstractionController.bundlers</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-bundlers-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/bundlers/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.bundler</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-bundles</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/bundles -&gt; V2.Proxy.AccountAbstractionController.bundles</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-factories</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/factories -&gt; V2.Proxy.AccountAbstractionController.factories</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-factories-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/factories/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.factory</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-operations</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/operations -&gt; V2.Proxy.AccountAbstractionController.operations</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-operations-operation-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/operations/:operation_hash_param -&gt; V2.Proxy.AccountAbstractionController.operation</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-operations-operation-hash-param-summary</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/operations/:operation_hash_param/summary -&gt; V2.Proxy.AccountAbstractionController.summary</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-paymasters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/paymasters -&gt; V2.Proxy.AccountAbstractionController.paymasters</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-paymasters-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/paymasters/:address_hash_param -&gt; V2.Proxy.AccountAbstractionController.paymaster</code> |
| <code>interface.14.http-get-api-v2-proxy-account-abstraction-status</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/account-abstraction/status -&gt; V2.Proxy.AccountAbstractionController.status</code> |
| <code>interface.14.http-get-api-v2-proxy-metadata-addresses</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/proxy/metadata/addresses -&gt; V2.Proxy.MetadataController.addresses</code> |

## api-v2-scam-badge-addresses

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-delete-api-v2-scam-badge-addresses</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex) | <code>DELETE /api/v2/scam-badge-addresses -&gt; V2.AddressBadgeController.unassign_badge_from_address</code> |
| <code>interface.14.http-get-api-v2-scam-badge-addresses</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex) | <code>GET /api/v2/scam-badge-addresses -&gt; V2.AddressBadgeController.show_badge_addresses</code> |
| <code>interface.14.http-post-api-v2-scam-badge-addresses</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/address_badges_v2_router.ex) | <code>POST /api/v2/scam-badge-addresses -&gt; V2.AddressBadgeController.assign_badge_to_address</code> |

## api-v2-scroll

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-scroll-batches</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/batches -&gt; V2.ScrollController.batches</code> |
| <code>interface.14.http-get-api-v2-scroll-batches-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/batches/count -&gt; V2.ScrollController.batches_count</code> |
| <code>interface.14.http-get-api-v2-scroll-batches-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/batches/:number -&gt; V2.ScrollController.batch</code> |
| <code>interface.14.http-get-api-v2-scroll-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/deposits -&gt; V2.ScrollController.deposits</code> |
| <code>interface.14.http-get-api-v2-scroll-deposits-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/deposits/count -&gt; V2.ScrollController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-scroll-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/withdrawals -&gt; V2.ScrollController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-scroll-withdrawals-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/scroll/withdrawals/count -&gt; V2.ScrollController.withdrawals_count</code> |

## api-v2-search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-search</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/search -&gt; V2.SearchController.search</code> |
| <code>interface.14.http-get-api-v2-search-check-redirect</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/search/check-redirect -&gt; V2.SearchController.check_redirect</code> |
| <code>interface.14.http-get-api-v2-search-quick</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/search/quick -&gt; V2.SearchController.quick_search</code> |

## api-v2-shibarium

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-shibarium-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/shibarium/deposits -&gt; V2.ShibariumController.deposits</code> |
| <code>interface.14.http-get-api-v2-shibarium-deposits-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/shibarium/deposits/count -&gt; V2.ShibariumController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-shibarium-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/shibarium/withdrawals -&gt; V2.ShibariumController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-shibarium-withdrawals-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/shibarium/withdrawals/count -&gt; V2.ShibariumController.withdrawals_count</code> |

## api-v2-smart-contracts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-smart-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>GET /api/v2/smart-contracts -&gt; V2.SmartContractController.smart_contracts_list</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>GET /api/v2/smart-contracts/:address_hash_param -&gt; V2.SmartContractController.smart_contract</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-address-hash-param-audit-reports</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>GET /api/v2/smart-contracts/:address_hash_param/audit-reports -&gt; V2.SmartContractController.audit_reports_list</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>GET /api/v2/smart-contracts/counters -&gt; V2.SmartContractController.smart_contracts_counters</code> |
| <code>interface.14.http-get-api-v2-smart-contracts-verification-config</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>GET /api/v2/smart-contracts/verification/config -&gt; V2.VerificationController.config</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-param-audit-reports</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash_param/audit-reports -&gt; V2.SmartContractController.audit_report_submission</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-flattened-code</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/flattened-code -&gt; V2.VerificationController.verification_via_flattened_code</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-multi-part</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/multi-part -&gt; V2.VerificationController.verification_via_multi_part</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-sourcify</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/sourcify -&gt; V2.VerificationController.verification_via_sourcify</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-standard-input</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/standard-input -&gt; V2.VerificationController.verification_via_standard_input</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-stylus-github-repository</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/stylus-github-repository -&gt; V2.VerificationController.verification_via_stylus_github_repository</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-code</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/vyper-code -&gt; V2.VerificationController.verification_via_vyper_code</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-multi-part</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/vyper-multi-part -&gt; V2.VerificationController.verification_via_vyper_multipart</code> |
| <code>interface.14.http-post-api-v2-smart-contracts-address-hash-verification-via-vyper-standard-input</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/smart_contracts_api_v2_router.ex) | <code>POST /api/v2/smart-contracts/:address_hash/verification/via/vyper-standard-input -&gt; V2.VerificationController.verification_via_vyper_standard_input</code> |

## api-v2-stats

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-stats</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/stats -&gt; V2.StatsController.stats</code> |
| <code>interface.14.http-get-api-v2-stats-charts-market</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/stats/charts/market -&gt; V2.StatsController.market_chart</code> |
| <code>interface.14.http-get-api-v2-stats-charts-secondary-coin-market</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/stats/charts/secondary-coin-market -&gt; V2.StatsController.secondary_coin_market_chart</code> |
| <code>interface.14.http-get-api-v2-stats-charts-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/stats/charts/transactions -&gt; V2.StatsController.transactions_chart</code> |
| <code>interface.14.http-get-api-v2-stats-hot-smart-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/stats/hot-smart-contracts -&gt; V2.StatsController.hot_smart_contracts</code> |

## api-v2-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/token-transfers -&gt; V2.TokenTransferController.token_transfers</code> |

## api-v2-tokens

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-tokens</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens -&gt; V2.TokenController.tokens_list</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param -&gt; V2.TokenController.token</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/counters -&gt; V2.TokenController.counters</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-holders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/holders -&gt; V2.TokenController.holders</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-holders-csv</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/holders/csv -&gt; V2.CsvExportController.export_token_holders</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/instances -&gt; V2.TokenController.instances</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param -&gt; V2.TokenController.instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param-holders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param/holders -&gt; V2.TokenController.holders_by_instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param/transfers -&gt; V2.TokenController.transfers_by_instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-instances-token-id-param-transfers-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/instances/:token_id_param/transfers-count -&gt; V2.TokenController.transfers_count_by_instance</code> |
| <code>interface.14.http-get-api-v2-tokens-address-hash-param-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/:address_hash_param/transfers -&gt; V2.TokenController.transfers</code> |
| <code>interface.14.http-get-api-v2-tokens-bridged</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>GET /api/v2/tokens/bridged -&gt; V2.TokenController.bridged_tokens_list</code> |
| <code>interface.14.http-patch-api-v2-tokens-address-hash-param-instances-refetch-metadata</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>PATCH /api/v2/tokens/:address_hash_param/instances/refetch-metadata -&gt; V2.TokenController.trigger_nft_collection_metadata_refetch</code> |
| <code>interface.14.http-patch-api-v2-tokens-address-hash-param-instances-token-id-param-refetch-metadata</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/tokens_api_v2_router.ex) | <code>PATCH /api/v2/tokens/:address_hash_param/instances/:token_id_param/refetch-metadata -&gt; V2.TokenController.refetch_metadata</code> |

## api-v2-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions -&gt; V2.TransactionController.transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-arbitrum-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/arbitrum-batch/:batch_number_param -&gt; V2.TransactionController.arbitrum_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-execution-node-execution-node-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/execution-node/:execution_node_hash_param -&gt; V2.TransactionController.execution_node</code> |
| <code>interface.14.http-get-api-v2-transactions-optimism-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/optimism-batch/:batch_number_param -&gt; V2.TransactionController.optimism_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-scroll-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/scroll-batch/:batch_number_param -&gt; V2.TransactionController.scroll_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-stats</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/stats -&gt; V2.TransactionController.stats</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param -&gt; V2.TransactionController.transaction</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-beacon-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/beacon/deposits -&gt; V2.TransactionController.beacon_deposits</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-blobs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/blobs -&gt; V2.TransactionController.blobs</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-external-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/external-transactions -&gt; V2.TransactionController.external_transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-internal-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/internal-transactions -&gt; V2.TransactionController.internal_transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-logs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/logs -&gt; V2.TransactionController.logs</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-raw-trace</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/raw-trace -&gt; V2.TransactionController.raw_trace</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-state-changes</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/state-changes -&gt; V2.TransactionController.state_changes</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-status</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/status -&gt; V2.PaxeerX.TransactionStatusController.status</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-summary</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/summary -&gt; V2.TransactionController.summary</code> |
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/token-transfers -&gt; V2.TransactionController.token_transfers</code> |
| <code>interface.14.http-get-api-v2-transactions-watchlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/watchlist -&gt; V2.TransactionController.watchlist_transactions</code> |
| <code>interface.14.http-get-api-v2-transactions-zkevm-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/zkevm-batch/:batch_number_param -&gt; V2.TransactionController.polygon_zkevm_batch</code> |
| <code>interface.14.http-get-api-v2-transactions-zksync-batch-batch-number-param</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/transactions/zksync-batch/:batch_number_param -&gt; V2.TransactionController.zksync_batch</code> |

## api-v2-utils

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-utils-decode-calldata</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/utils_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/utils_api_v2_router.ex) | <code>GET /api/v2/utils/decode-calldata -&gt; V2.UtilsController.decode_calldata</code> |
| <code>interface.14.http-post-api-v2-utils-decode-calldata</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/utils_api_v2_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/utils_api_v2_router.ex) | <code>POST /api/v2/utils/decode-calldata -&gt; V2.UtilsController.decode_calldata</code> |

## api-v2-validators

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-validators-blackfort</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/validators/blackfort -&gt; V2.ValidatorController.blackfort_validators_list</code> |
| <code>interface.14.http-get-api-v2-validators-blackfort-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/validators/blackfort/counters -&gt; V2.ValidatorController.blackfort_validators_counters</code> |
| <code>interface.14.http-get-api-v2-validators-stability</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/validators/stability -&gt; V2.ValidatorController.stability_validators_list</code> |
| <code>interface.14.http-get-api-v2-validators-stability-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/validators/stability/counters -&gt; V2.ValidatorController.stability_validators_counters</code> |
| <code>interface.14.http-get-api-v2-validators-zilliqa</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/validators/zilliqa -&gt; V2.ValidatorController.zilliqa_validators_list</code> |
| <code>interface.14.http-get-api-v2-validators-zilliqa-bls-public-key</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/validators/zilliqa/:bls_public_key -&gt; V2.ValidatorController.zilliqa_validator</code> |

## api-v2-withdrawals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/withdrawals -&gt; V2.WithdrawalController.withdrawals_list</code> |
| <code>interface.14.http-get-api-v2-withdrawals-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/withdrawals/counters -&gt; V2.WithdrawalController.withdrawals_counters</code> |

## api-v2-zkevm

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-zkevm-batches</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/batches -&gt; V2.PolygonZkevmController.batches</code> |
| <code>interface.14.http-get-api-v2-zkevm-batches-batch-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/batches/:batch_number -&gt; V2.PolygonZkevmController.batch</code> |
| <code>interface.14.http-get-api-v2-zkevm-batches-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/batches/count -&gt; V2.PolygonZkevmController.batches_count</code> |
| <code>interface.14.http-get-api-v2-zkevm-deposits</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/deposits -&gt; V2.PolygonZkevmController.deposits</code> |
| <code>interface.14.http-get-api-v2-zkevm-deposits-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/deposits/count -&gt; V2.PolygonZkevmController.deposits_count</code> |
| <code>interface.14.http-get-api-v2-zkevm-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/withdrawals -&gt; V2.PolygonZkevmController.withdrawals</code> |
| <code>interface.14.http-get-api-v2-zkevm-withdrawals-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zkevm/withdrawals/count -&gt; V2.PolygonZkevmController.withdrawals_count</code> |

## api-v2-zksync

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-zksync-batches</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zksync/batches -&gt; V2.ZkSyncController.batches</code> |
| <code>interface.14.http-get-api-v2-zksync-batches-batch-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zksync/batches/:batch_number -&gt; V2.ZkSyncController.batch</code> |
| <code>interface.14.http-get-api-v2-zksync-batches-count</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/zksync/batches/count -&gt; V2.ZkSyncController.batches_count</code> |

## auth

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-auth-provider</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /auth/:provider -&gt; Account.AuthController.request</code> |

## auth-callback

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-auth-provider-callback</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /auth/:provider/callback -&gt; Account.AuthController.callback</code> |

## auth-logout

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-auth-logout</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /auth/logout -&gt; Account.AuthController.logout</code> |

## auth-path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-any-auth-path</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>ANY /auth/*path -&gt; AccountRouter</code> |

## auth-profile

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-auth-profile</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/account_router.ex) | <code>GET /auth/profile -&gt; Account.AuthController.profile</code> |

## block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-block-hash-or-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /block/:hash_or_number -&gt; BlockController.show</code> |

## block-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-block-block-hash-or-number-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /block/:block_hash_or_number/transactions -&gt; BlockTransactionController.index</code> |

## block-withdrawals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-block-block-hash-or-number-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /block/:block_hash_or_number/withdrawals -&gt; BlockWithdrawalController.index</code> |

## blocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-blocks</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /blocks -&gt; BlockController.index</code> |
| <code>interface.14.http-get-blocks-hash-or-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /blocks/:hash_or_number -&gt; BlockController.show</code> |

## blocks-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-blocks-block-hash-or-number-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /blocks/:block_hash_or_number/transactions -&gt; BlockTransactionController.index</code> |

## blocks-withdrawals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-blocks-block-hash-or-number-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /blocks/:block_hash_or_number/withdrawals -&gt; BlockWithdrawalController.index</code> |

## chain-blocks

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-chain-blocks</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /chain-blocks -&gt; ChainController.chain_blocks</code> |

## csv-export

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-csv-export</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /csv-export -&gt; CsvExportController.index</code> |

## eth-rpc-api-docs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-eth-rpc-api-docs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /eth-rpc-api-docs -&gt; APIDocsController.eth_rpc</code> |
| <code>interface.14.http-get-eth-rpc-api-docs-l101</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /eth-rpc-api-docs -&gt; PageNotFoundController.index</code> |

## graphiql

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-forward-graphiql</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>FORWARD /graphiql -&gt; Absinthe.Plug.GraphiQL</code> |

## graphql

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-graphql-query-address</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex) | <code>POST /api/v1/graphql query { address }</code> |
| <code>interface.14.http-graphql-query-addresses</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex) | <code>POST /api/v1/graphql query { addresses }</code> |
| <code>interface.14.http-graphql-query-block</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex) | <code>POST /api/v1/graphql query { block }</code> |
| <code>interface.14.http-graphql-query-node</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex) | <code>POST /api/v1/graphql query { node }</code> |
| <code>interface.14.http-graphql-query-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex) | <code>POST /api/v1/graphql query { token_transfers }</code> |
| <code>interface.14.http-graphql-query-transaction</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex) | <code>POST /api/v1/graphql query { transaction }</code> |

## market-history-chart

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-market-history-chart</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /market-history-chart -&gt; Chain.MarketHistoryChartController.show</code> |

## path

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-path</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /*path -&gt; PageNotFoundController.index</code> |

## pending-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-pending-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /pending-transactions -&gt; PendingTransactionController.index</code> |

## recent-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-recent-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /recent-transactions -&gt; RecentTransactionsController.index</code> |

## reorgs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-reorgs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /reorgs -&gt; BlockController.reorg</code> |

## robots-txt

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-robots-txt</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /robots.txt -&gt; RobotsController.robots</code> |

## root

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET / -&gt; ChainController.show</code> |

## rpc-module-account

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-account-balance</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=balance -&gt; RPC.AddressController.balance</code> |
| <code>interface.14.http-api-rpc-account-balancemulti</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=balancemulti -&gt; RPC.AddressController.balancemulti</code> |
| <code>interface.14.http-api-rpc-account-eth-get-balance</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=eth_get_balance -&gt; RPC.AddressController.eth_get_balance</code> |
| <code>interface.14.http-api-rpc-account-getminedblocks</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=getminedblocks -&gt; RPC.AddressController.getminedblocks</code> |
| <code>interface.14.http-api-rpc-account-listaccounts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=listaccounts -&gt; RPC.AddressController.listaccounts</code> |
| <code>interface.14.http-api-rpc-account-pendingtxlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=pendingtxlist -&gt; RPC.AddressController.pendingtxlist</code> |
| <code>interface.14.http-api-rpc-account-tokenbalance</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=tokenbalance -&gt; RPC.AddressController.tokenbalance</code> |
| <code>interface.14.http-api-rpc-account-tokenlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=tokenlist -&gt; RPC.AddressController.tokenlist</code> |
| <code>interface.14.http-api-rpc-account-tokennfttx</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=tokennfttx -&gt; RPC.AddressController.tokennfttx</code> |
| <code>interface.14.http-api-rpc-account-tokentx</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=tokentx -&gt; RPC.AddressController.tokentx</code> |
| <code>interface.14.http-api-rpc-account-txlist</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=txlist -&gt; RPC.AddressController.txlist</code> |
| <code>interface.14.http-api-rpc-account-txlistinternal</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/address_controller.ex) | <code>GET&#124;POST /api?module=account&amp;action=txlistinternal -&gt; RPC.AddressController.txlistinternal</code> |

## rpc-module-block

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-block-eth-block-number</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex) | <code>GET&#124;POST /api?module=block&amp;action=eth_block_number -&gt; RPC.BlockController.eth_block_number</code> |
| <code>interface.14.http-api-rpc-block-getblockcountdown</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex) | <code>GET&#124;POST /api?module=block&amp;action=getblockcountdown -&gt; RPC.BlockController.getblockcountdown</code> |
| <code>interface.14.http-api-rpc-block-getblocknobytime</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex) | <code>GET&#124;POST /api?module=block&amp;action=getblocknobytime -&gt; RPC.BlockController.getblocknobytime</code> |
| <code>interface.14.http-api-rpc-block-getblockreward</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/block_controller.ex) | <code>GET&#124;POST /api?module=block&amp;action=getblockreward -&gt; RPC.BlockController.getblockreward</code> |

## rpc-module-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-contract-checkproxyverification</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=checkproxyverification -&gt; RPC.ContractController.checkproxyverification</code> |
| <code>interface.14.http-api-rpc-contract-checkverifystatus</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=checkverifystatus -&gt; RPC.ContractController.checkverifystatus</code> |
| <code>interface.14.http-api-rpc-contract-getabi</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=getabi -&gt; RPC.ContractController.getabi</code> |
| <code>interface.14.http-api-rpc-contract-getcontractcreation</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=getcontractcreation -&gt; RPC.ContractController.getcontractcreation</code> |
| <code>interface.14.http-api-rpc-contract-getsourcecode</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=getsourcecode -&gt; RPC.ContractController.getsourcecode</code> |
| <code>interface.14.http-api-rpc-contract-listcontracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=listcontracts -&gt; RPC.ContractController.listcontracts</code> |
| <code>interface.14.http-api-rpc-contract-verify</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=verify -&gt; RPC.ContractController.verify</code> |
| <code>interface.14.http-api-rpc-contract-verify-via-sourcify</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=verify_via_sourcify -&gt; RPC.ContractController.verify_via_sourcify</code> |
| <code>interface.14.http-api-rpc-contract-verify-vyper-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=verify_vyper_contract -&gt; RPC.ContractController.verify_vyper_contract</code> |
| <code>interface.14.http-api-rpc-contract-verifyproxycontract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=verifyproxycontract -&gt; RPC.ContractController.verifyproxycontract</code> |
| <code>interface.14.http-api-rpc-contract-verifysourcecode</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/contract_controller.ex) | <code>GET&#124;POST /api?module=contract&amp;action=verifysourcecode -&gt; RPC.ContractController.verifysourcecode</code> |

## rpc-module-epoch

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-epoch-getepoch</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/celo_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/celo_controller.ex) | <code>GET&#124;POST /api?module=epoch&amp;action=getepoch -&gt; RPC.CeloController.getepoch</code> |

## rpc-module-logs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-logs-getlogs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/logs_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/logs_controller.ex) | <code>GET&#124;POST /api?module=logs&amp;action=getlogs -&gt; RPC.LogsController.getlogs</code> |

## rpc-module-stats

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-stats-coinprice</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=coinprice -&gt; RPC.StatsController.coinprice</code> |
| <code>interface.14.http-api-rpc-stats-coinsupply</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=coinsupply -&gt; RPC.StatsController.coinsupply</code> |
| <code>interface.14.http-api-rpc-stats-ethprice</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=ethprice -&gt; RPC.StatsController.ethprice</code> |
| <code>interface.14.http-api-rpc-stats-ethsupply</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=ethsupply -&gt; RPC.StatsController.ethsupply</code> |
| <code>interface.14.http-api-rpc-stats-ethsupplyexchange</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=ethsupplyexchange -&gt; RPC.StatsController.ethsupplyexchange</code> |
| <code>interface.14.http-api-rpc-stats-tokensupply</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=tokensupply -&gt; RPC.StatsController.tokensupply</code> |
| <code>interface.14.http-api-rpc-stats-totalfees</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/stats_controller.ex) | <code>GET&#124;POST /api?module=stats&amp;action=totalfees -&gt; RPC.StatsController.totalfees</code> |

## rpc-module-token

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-token-gettoken</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/token_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/token_controller.ex) | <code>GET&#124;POST /api?module=token&amp;action=gettoken -&gt; RPC.TokenController.gettoken</code> |
| <code>interface.14.http-api-rpc-token-gettokenholders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/token_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/token_controller.ex) | <code>GET&#124;POST /api?module=token&amp;action=gettokenholders -&gt; RPC.TokenController.gettokenholders</code> |

## rpc-module-transaction

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-api-rpc-transaction-getstatus</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex) | <code>GET&#124;POST /api?module=transaction&amp;action=getstatus -&gt; RPC.TransactionController.getstatus</code> |
| <code>interface.14.http-api-rpc-transaction-gettxinfo</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex) | <code>GET&#124;POST /api?module=transaction&amp;action=gettxinfo -&gt; RPC.TransactionController.gettxinfo</code> |
| <code>interface.14.http-api-rpc-transaction-gettxreceiptstatus</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/rpc/transaction_controller.ex) | <code>GET&#124;POST /api?module=transaction&amp;action=gettxreceiptstatus -&gt; RPC.TransactionController.gettxreceiptstatus</code> |

## schema-graphql

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-schema-graphql</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /schema.graphql -&gt; GraphQL.SchemaController.index</code> |

## search

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-search</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /search -&gt; ChainController.search</code> |

## search-logs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-search-logs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /search-logs -&gt; AddressLogsController.search_logs</code> |

## search-results

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-search-results</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /search-results -&gt; SearchController.search_results</code> |

## search-verified-contracts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-search-verified-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /search-verified-contracts -&gt; VerifiedContractsController.search_verified_contracts</code> |

## sent-emails

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-forward-sent-emails</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>FORWARD /sent_emails -&gt; Bamboo.SentEmailViewerPlug</code> |

## sitemap-xml

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-sitemap-xml</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>GET /sitemap.xml -&gt; RobotsController.sitemap</code> |

## smart-contracts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-smart-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /smart-contracts -&gt; SmartContractController.index</code> |
| <code>interface.14.http-get-smart-contracts-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /smart-contracts/:id -&gt; SmartContractController.show</code> |

## token

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:id -&gt; Tokens.TokenController.show</code> |

## token-autocomplete

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-autocomplete</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token-autocomplete -&gt; ChainController.token_autocomplete</code> |

## token-counters

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-counters</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token-counters -&gt; Tokens.TokenController.token_counters</code> |

## token-instance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/instance/:id -&gt; Tokens.InstanceController.show</code> |

## token-instance-metadata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-metadata</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/instance/:instance_id/metadata -&gt; Tokens.Instance.MetadataController.index</code> |

## token-instance-token-holders

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-token-holders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/instance/:instance_id/token-holders -&gt; Tokens.Instance.HolderController.index</code> |

## token-instance-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-instance-instance-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/instance/:instance_id/token-transfers -&gt; Tokens.Instance.TransferController.index</code> |

## token-inventory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-inventory</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/inventory -&gt; Tokens.InventoryController.index</code> |

## token-read-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-read-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/read-contract -&gt; Tokens.ContractController.index</code> |

## token-read-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-read-proxy</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/read-proxy -&gt; Tokens.ContractController.index</code> |

## token-token-holders

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-token-holders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/token-holders -&gt; Tokens.HolderController.index</code> |

## token-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/token-transfers -&gt; Tokens.TransferController.index</code> |

## token-write-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-write-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/write-contract -&gt; Tokens.ContractController.index</code> |

## token-write-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-token-token-id-write-proxy</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /token/:token_id/write-proxy -&gt; Tokens.ContractController.index</code> |

## tokens

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens -&gt; TokensController.index</code> |
| <code>interface.14.http-get-tokens-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:id -&gt; Tokens.TokenController.show</code> |

## tokens-instance

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/instance/:id -&gt; Tokens.InstanceController.show</code> |

## tokens-instance-metadata

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-metadata</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/instance/:instance_id/metadata -&gt; Tokens.Instance.MetadataController.index</code> |

## tokens-instance-token-holders

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-token-holders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/instance/:instance_id/token-holders -&gt; Tokens.Instance.HolderController.index</code> |

## tokens-instance-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-instance-instance-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/instance/:instance_id/token-transfers -&gt; Tokens.Instance.TransferController.index</code> |

## tokens-inventory

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-inventory</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/inventory -&gt; Tokens.InventoryController.index</code> |

## tokens-read-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-read-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/read-contract -&gt; Tokens.ContractController.index</code> |

## tokens-read-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-read-proxy</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/read-proxy -&gt; Tokens.ContractController.index</code> |

## tokens-token-holders

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-token-holders</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/token-holders -&gt; Tokens.HolderController.index</code> |

## tokens-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/token-transfers -&gt; Tokens.TransferController.index</code> |

## tokens-write-contract

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-write-contract</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/write-contract -&gt; Tokens.ContractController.index</code> |

## tokens-write-proxy

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tokens-token-id-write-proxy</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tokens/:token_id/write-proxy -&gt; Tokens.ContractController.index</code> |

## transaction-history-chart

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-transaction-history-chart</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /transaction-history-chart -&gt; Chain.TransactionHistoryChartController.show</code> |

## tx

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tx-id</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tx/:id -&gt; TransactionController.show</code> |

## tx-internal-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-internal-transactions</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tx/:transaction_id/internal-transactions -&gt; TransactionInternalTransactionController.index</code> |

## tx-logs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-logs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tx/:transaction_id/logs -&gt; TransactionLogController.index</code> |

## tx-raw-trace

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-raw-trace</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tx/:transaction_id/raw-trace -&gt; TransactionRawTraceController.index</code> |

## tx-state

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-state</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tx/:transaction_id/state -&gt; TransactionStateController.index</code> |

## tx-token-transfers

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-tx-transaction-id-token-transfers</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /tx/:transaction_id/token-transfers -&gt; TransactionTokenTransferController.index</code> |

## txs

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-txs</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /txs -&gt; TransactionController.index</code> |

## uncles

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-uncles</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /uncles -&gt; BlockController.uncle</code> |

## v2-paxeer-x

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-paxeer-x-archive-anchors</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex) | <code>GET /api/v2/paxeer-x/archive/anchors -&gt; V2.PaxeerX.ArchiveController.anchors</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-archive-receipts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/archive_controller.ex) | <code>GET /api/v2/paxeer-x/archive/receipts -&gt; V2.PaxeerX.ArchiveController.receipts</code> |
| <code>interface.14.http-get-api-v2-paxeer-x-path-not-found</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/api_router.ex) | <code>GET /api/v2/paxeer-x/*path -&gt; 404</code> |

| Field | Rule |
|---|---|
| interface | GET /api/v2/paxeer-x/archive/anchors, /archive/receipts and /archive/receipts/:id |
| storage | Reads lx_anchors and lx_receipts; no writes. |
| errors | 404 for an unknown receipt id and for any other /api/v2/paxeer-x path. |
| events | none |
| gas | none (off-chain service) |
| invariants | Every archive item carries frozen true and no settlement rung. |
| pause | none |

## v2-transactions

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-api-v2-transactions-transaction-hash-param-signature</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/api/v2/paxeer_x/transaction_signature_controller.ex) | <code>GET /api/v2/transactions/:transaction_hash_param/signature -&gt; V2.PaxeerX.TransactionSignatureController.signature</code> |

| Field | Rule |
|---|---|
| interface | GET /api/v2/transactions/:transaction_hash_param/signature |
| storage | none; reads node receipt answers per request. |
| errors | 404 unless verification reaches receipted or committed. |
| events | none |
| gas | none (off-chain service) |
| invariants | The signature is the 64-byte Ed25519 signature the pinned receipt key verifies. |
| pause | none |

## verified-contracts

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-verified-contracts</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /verified-contracts -&gt; VerifiedContractsController.index</code> |

## verify-smart-contract-contract-verifications

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-post-verify-smart-contract-contract-verifications</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/router.ex) | <code>POST /verify_smart_contract/contract_verifications -&gt; BlockScoutWeb.AddressContractVerificationController.create</code> |

## visualize-sol2uml

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-visualize-sol2uml</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /visualize/sol2uml -&gt; VisualizeSol2umlController.index</code> |

## withdrawals

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.http-get-withdrawals</code> | <code>http_route</code> | [services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/routers/web_router.ex) | <code>GET /withdrawals -&gt; WithdrawalController.index</code> |
