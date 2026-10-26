# Explorer ws contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## channels

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ws-explorer-channel-socket-addresses-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:5</code> | <code>channel addresses_old:* -&gt; BlockScoutWeb.AddressChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-blocks-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:6</code> | <code>channel blocks_old:* -&gt; BlockScoutWeb.BlockChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-exchange-rate-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:7</code> | <code>channel exchange_rate_old:* -&gt; BlockScoutWeb.ExchangeRateChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-rewards-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:8</code> | <code>channel rewards_old:* -&gt; BlockScoutWeb.RewardChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-token-instances-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:11</code> | <code>channel token_instances_old:* -&gt; BlockScoutWeb.TokenInstanceChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-tokens-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:10</code> | <code>channel tokens_old:* -&gt; BlockScoutWeb.TokenChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-transactions-old</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex:9</code> | <code>channel transactions_old:* -&gt; BlockScoutWeb.TransactionChannel on /socket</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-addresses</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:8</code> | <code>channel addresses:* -&gt; BlockScoutWeb.V2.AddressChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-arbitrum</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:19</code> | <code>channel arbitrum:* -&gt; BlockScoutWeb.ArbitrumChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-blocks</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:9</code> | <code>channel blocks:* -&gt; BlockScoutWeb.V2.BlockChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-exchange-rate</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:10</code> | <code>channel exchange_rate:* -&gt; BlockScoutWeb.V2.ExchangeRateChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-optimism</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:20</code> | <code>channel optimism:* -&gt; BlockScoutWeb.OptimismChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-paxeer-x-feed</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:16</code> | <code>channel paxeer_x_feed:* -&gt; BlockScoutWeb.V2.PaxeerXFeedChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-rewards</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:11</code> | <code>channel rewards:* -&gt; BlockScoutWeb.V2.RewardChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-token-instances</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:14</code> | <code>channel token_instances:* -&gt; BlockScoutWeb.TokenInstanceChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-tokens</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:13</code> | <code>channel tokens:* -&gt; BlockScoutWeb.V2.TokenChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-transactions</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:12</code> | <code>channel transactions:* -&gt; BlockScoutWeb.V2.TransactionChannel on /socket/v2</code> |
| <code>interface.14.ws-explorer-channel-socket-v2-zkevm-batches</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex:15</code> | <code>channel zkevm_batches:* -&gt; BlockScoutWeb.V2.PolygonZkevmConfirmedBatchChannel on /socket/v2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ws-explorer-channel-socket-addresses-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-blocks-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-exchange-rate-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-rewards-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-token-instances-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-tokens-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-transactions-old</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-v2-arbitrum</code> | notes | registered only when CHAIN_TYPE is arbitrum; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.ws-explorer-channel-socket-v2-exchange-rate</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-v2-optimism</code> | notes | registered only when CHAIN_TYPE is optimism; deploy builds CHAIN_TYPE=paxeer_x (services/explorer/deploy/docker-compose.local.yml:171) |
| <code>interface.14.ws-explorer-channel-socket-v2-rewards</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-channel-socket-v2-zkevm-batches</code> | notes | registered for every chain type; only polygon_zkevm publishes to it |

## graphql-subscription

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ws-graphql-subscription-token-transfers</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex:120</code> | <code>subscription token_transfers(token_contract_address_hash: AddressHash!) over /socket (channels/user_socket.ex Absinthe.Phoenix.Socket)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ws-graphql-subscription-token-transfers</code> | notes | topic is the token contract address hash; published from notifier.ex; no in-repo client found |

## pushes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ws-explorer-push-addresses-hash-balance</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:640</code> | <code>push balance on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-changed-bytecode</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:400</code> | <code>push changed_bytecode on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-coin-balance</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:598</code> | <code>push coin_balance on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-current-coin-balance</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:602</code> | <code>push current_coin_balance on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:519</code> | <code>push event on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-event-l884</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:884</code> | <code>push event on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-fetched-bytecode</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:370</code> | <code>push fetched_bytecode on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-to-string-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:900</code> | <code>push to_string(event) on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-token-balance</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:616</code> | <code>push token_balance on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-verification-result</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:136</code> | <code>push verification_result on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-hash-verification-result-l177</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:177</code> | <code>push verification_result on addresses:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-new-address-count</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:102</code> | <code>push count on addresses:new_address</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-balance-update</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:631</code> | <code>push balance_update on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-changed-bytecode</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:399</code> | <code>push changed_bytecode on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-coin-balance</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:589</code> | <code>push coin_balance on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:814</code> | <code>push event on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event-l820</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:820</code> | <code>push event on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event-l854</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:854</code> | <code>push event on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event-l865</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:865</code> | <code>push event on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-fetched-bytecode</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:366</code> | <code>push fetched_bytecode on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-internal-transaction</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:744</code> | <code>push internal_transaction on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-internal-transaction-l750</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:750</code> | <code>push internal_transaction on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-to-string-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:899</code> | <code>push to_string(event) on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-token-balance</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:612</code> | <code>push token_balance on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-verification-result</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:128</code> | <code>push verification_result on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-hash-verification-result-l169</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:169</code> | <code>push verification_result on addresses_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-addresses-old-new-address-count</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:101</code> | <code>push count on addresses_old:new_address</code> |
| <code>interface.14.ws-explorer-push-arbitrum-new-batch-new-arbitrum-batch</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifiers/arbitrum.ex:15</code> | <code>push new_arbitrum_batch on arbitrum:new_batch</code> |
| <code>interface.14.ws-explorer-push-arbitrum-new-messages-to-rollup-amount-new-messages-to-rollup-amount</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifiers/arbitrum.ex:22</code> | <code>push new_messages_to_rollup_amount on arbitrum:new_messages_to_rollup_amount</code> |
| <code>interface.14.ws-explorer-push-blocks-hash-new-block</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:662</code> | <code>push new_block on blocks:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-blocks-new-block-new-block</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:661</code> | <code>push new_block on blocks:new_block</code> |
| <code>interface.14.ws-explorer-push-blocks-new-block-new-block-l720</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:720</code> | <code>push new_block on blocks:new_block</code> |
| <code>interface.14.ws-explorer-push-blocks-old-hash-new-block</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:654</code> | <code>push new_block on blocks_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-blocks-old-new-block-new-block</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:649</code> | <code>push new_block on blocks_old:new_block</code> |
| <code>interface.14.ws-explorer-push-default-channel-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:783</code> | <code>push event on default_channel</code> |
| <code>interface.14.ws-explorer-push-exchange-rate-new-rate-new-rate</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:241</code> | <code>push new_rate on exchange_rate:new_rate</code> |
| <code>interface.14.ws-explorer-push-exchange-rate-old-new-rate-new-rate</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:236</code> | <code>push new_rate on exchange_rate_old:new_rate</code> |
| <code>interface.14.ws-explorer-push-msg-index-status</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:548</code> | <code>push index_status on msg</code> |
| <code>interface.14.ws-explorer-push-optimism-new-batch-new-optimism-batch</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifiers/optimism.ex:14</code> | <code>push new_optimism_batch on optimism:new_batch</code> |
| <code>interface.14.ws-explorer-push-optimism-new-deposits-new-optimism-deposits</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifiers/optimism.ex:24</code> | <code>push new_optimism_deposits on optimism:new_deposits</code> |
| <code>interface.14.ws-explorer-push-paxeer-x-feed-market-hash-new-update</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:699</code> | <code>push new_update on paxeer_x_feed:market:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-paxeer-x-feed-new-update-new-update</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:692</code> | <code>push new_update on paxeer_x_feed:new_update</code> |
| <code>interface.14.ws-explorer-push-rewards-hash-new-reward</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:739</code> | <code>push new_reward on rewards:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-rewards-old-hash-new-reward</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:734</code> | <code>push new_reward on rewards_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-token-instances-hash-fetched-token-instance-metadata</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:379</code> | <code>push fetched_token_instance_metadata on token_instances:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-token-instances-hash-not-fetched-token-instance-metadata</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:390</code> | <code>push not_fetched_token_instance_metadata on token_instances:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-tokens-hash-token-transfer</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:829</code> | <code>push token_transfer on tokens:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-tokens-hash-total-supply</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:359</code> | <code>push total_supply on tokens:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-tokens-old-hash-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:859</code> | <code>push event on tokens_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-tokens-old-hash-token-total-supply</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:357</code> | <code>push token_total_supply on tokens_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-transaction-channel-event</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:810</code> | <code>push event on transaction_channel</code> |
| <code>interface.14.ws-explorer-push-transactions-hash-raw-trace</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:261</code> | <code>push raw_trace on transactions:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-transactions-new-transaction-transaction</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:707</code> | <code>push transaction on transactions:new_transaction</code> |
| <code>interface.14.ws-explorer-push-transactions-old-hash-collated</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:808</code> | <code>push collated on transactions_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-transactions-old-hash-raw-trace</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:253</code> | <code>push raw_trace on transactions_old:&lt;hash&gt;</code> |
| <code>interface.14.ws-explorer-push-transactions-old-stats-update</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:348</code> | <code>push update on transactions_old:stats</code> |
| <code>interface.14.ws-explorer-push-zkevm-batches-new-zkevm-confirmed-batch-new-zkevm-confirmed-batch</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex:209</code> | <code>push new_zkevm_confirmed_batch on zkevm_batches:new_zkevm_confirmed_batch</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ws-explorer-push-addresses-hash-event</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-hash-event-l884</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-hash-to-string-event</code> | notes | topic or event computed at runtime from to_string(event); no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-new-address-count</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-balance-update</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-changed-bytecode</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-coin-balance</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event-l820</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event-l854</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-event-l865</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-fetched-bytecode</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-internal-transaction</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-internal-transaction-l750</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-to-string-event</code> | notes | topic or event computed at runtime from to_string(event); no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-token-balance</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-verification-result</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-hash-verification-result-l169</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-addresses-old-new-address-count</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-arbitrum-new-batch-new-arbitrum-batch</code> | notes | compiled only for that CHAIN_TYPE |
| <code>interface.14.ws-explorer-push-arbitrum-new-messages-to-rollup-amount-new-messages-to-rollup-amount</code> | notes | compiled only for that CHAIN_TYPE |
| <code>interface.14.ws-explorer-push-blocks-old-hash-new-block</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-blocks-old-new-block-new-block</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-default-channel-event</code> | notes | topic or event computed at runtime from default_channel; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-exchange-rate-new-rate-new-rate</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-exchange-rate-old-new-rate-new-rate</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-msg-index-status</code> | notes | topic or event computed at runtime from msg |
| <code>interface.14.ws-explorer-push-optimism-new-batch-new-optimism-batch</code> | notes | compiled only for that CHAIN_TYPE; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-optimism-new-deposits-new-optimism-deposits</code> | notes | compiled only for that CHAIN_TYPE |
| <code>interface.14.ws-explorer-push-rewards-hash-new-reward</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-rewards-old-hash-new-reward</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-token-instances-hash-not-fetched-token-instance-metadata</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-tokens-old-hash-event</code> | notes | topic or event computed at runtime from event; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-tokens-old-hash-token-total-supply</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-transaction-channel-event</code> | notes | topic or event computed at runtime from transaction_channel; no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-transactions-old-hash-collated</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-transactions-old-hash-raw-trace</code> | notes | no in-repo client handler found by event name |
| <code>interface.14.ws-explorer-push-transactions-old-stats-update</code> | notes | no in-repo client handler found by event name |

## rust-phoenix-channel

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ws-phoenix-channel-lib-socket-websocket</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/services/libs/actix-phoenix-channel/src/actix_handler.rs:31</code> | <code>"/socket/websocket",</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ws-phoenix-channel-lib-socket-websocket</code> | notes | library route; no binary in services/explorer/services mounts it |

## sockets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ws-explorer-socket-socket</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex:22</code> | <code>socket /socket -&gt; BlockScoutWeb.UserSocket (Phoenix channels, websocket timeout 45000 ms)</code> |
| <code>interface.14.ws-explorer-socket-socket-v2</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex:23</code> | <code>socket /socket/v2 -&gt; BlockScoutWeb.V2.UserSocket (Phoenix channels, websocket timeout 45000 ms)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ws-explorer-socket-socket</code> | notes | legacy socket for *_old topics and Absinthe GraphQL subscriptions |
| <code>interface.14.ws-explorer-socket-socket-v2</code> | notes | v2 socket used by the explorer frontend |

## topics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.ws-explorer-join-socket-addresses-old-address-hash-string</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/address_channel.ex:36</code> | <code>join addresses_old:&lt;address_hash_string&gt; on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-blocks-old-miner-address</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/block_channel.ex:16</code> | <code>join blocks_old:&lt;miner_address&gt; on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-blocks-old-new-block</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/block_channel.ex:12</code> | <code>join blocks_old:new_block on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-exchange-rate-old-new-rate</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/exchange_rate_channel.ex:7</code> | <code>join exchange_rate_old:new_rate on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-rewards-old-address-hash-string</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/reward_channel.ex:13</code> | <code>join rewards_old:&lt;address_hash_string&gt; on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-token-instances-token-contract-address-hash</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/token_instance_channel.ex:7</code> | <code>join token_instances:&lt;token_contract_address_hash&gt; on /socket and /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-tokens-old-transaction-hash</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/token_channel.ex:20</code> | <code>join tokens_old:&lt;transaction_hash&gt; on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-transactions-old-new-pending-transaction</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/transaction_channel.ex:23</code> | <code>join transactions_old:new_pending_transaction on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-transactions-old-new-transaction</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/transaction_channel.ex:19</code> | <code>join transactions_old:new_transaction on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-transactions-old-stats</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/transaction_channel.ex:27</code> | <code>join transactions_old:stats on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-transactions-old-transaction-hash</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/transaction_channel.ex:31</code> | <code>join transactions_old:&lt;transaction_hash&gt; on /socket</code> |
| <code>interface.14.ws-explorer-join-socket-v2-addresses-address-hash-string</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/address_channel.ex:7</code> | <code>join addresses:&lt;address_hash_string&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-arbitrum-new-batch</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/arbitrum_channel.ex:7</code> | <code>join arbitrum:new_batch on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-arbitrum-new-messages-to-rollup-amount</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/arbitrum_channel.ex:11</code> | <code>join arbitrum:new_messages_to_rollup_amount on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-blocks-common</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/block_channel.ex:7</code> | <code>join blocks:&lt;common&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-blocks-miner-address</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/block_channel.ex:12</code> | <code>join blocks:&lt;miner_address&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-exchange-rate-new-rate</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/exchange_rate_channel.ex:7</code> | <code>join exchange_rate:new_rate on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-optimism-new-batch</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/optimism_channel.ex:7</code> | <code>join optimism:new_batch on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-optimism-new-deposits</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/optimism_channel.ex:11</code> | <code>join optimism:new_deposits on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-paxeer-x-feed-market-market-id</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/paxeer_x_feed_channel.ex:13</code> | <code>join paxeer_x_feed:market:&lt;market_id&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-paxeer-x-feed-new-update</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/paxeer_x_feed_channel.ex:9</code> | <code>join paxeer_x_feed:new_update on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-paxeer-x-feed-refused</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/paxeer_x_feed_channel.ex:20</code> | <code>join paxeer_x_feed:&lt;any other&gt; on /socket/v2 -&gt; {:error, %{reason: "unknown topic"}}</code> |
| <code>interface.14.ws-explorer-join-socket-v2-rewards-address-hash-string</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/reward_channel.ex:7</code> | <code>join rewards:&lt;address_hash_string&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-tokens-transaction-hash</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/token_channel.ex:7</code> | <code>join tokens:&lt;transaction_hash&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-transactions-new-pending-transaction</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/transaction_channel.ex:11</code> | <code>join transactions:new_pending_transaction on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-transactions-new-transaction</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/transaction_channel.ex:7</code> | <code>join transactions:new_transaction on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-transactions-stats</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/transaction_channel.ex:15</code> | <code>join transactions:stats on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-transactions-transaction-hash</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/transaction_channel.ex:19</code> | <code>join transactions:&lt;transaction_hash&gt; on /socket/v2</code> |
| <code>interface.14.ws-explorer-join-socket-v2-zkevm-batches-new-zkevm-confirmed-batch</code> | <code>ws_route</code> | <code>stable</code> | <code>services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/polygon_zkevm_confirmed_batch_channel.ex:7</code> | <code>join zkevm_batches:new_zkevm_confirmed_batch on /socket/v2</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.ws-explorer-join-socket-addresses-old-address-hash-string</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-blocks-old-miner-address</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-blocks-old-new-block</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-exchange-rate-old-new-rate</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-rewards-old-address-hash-string</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-tokens-old-transaction-hash</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-transactions-old-new-pending-transaction</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-transactions-old-new-transaction</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-transactions-old-stats</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-transactions-old-transaction-hash</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-v2-arbitrum-new-batch</code> | notes | channel registered only for CHAIN_TYPE arbitrum |
| <code>interface.14.ws-explorer-join-socket-v2-arbitrum-new-messages-to-rollup-amount</code> | notes | channel registered only for CHAIN_TYPE arbitrum |
| <code>interface.14.ws-explorer-join-socket-v2-exchange-rate-new-rate</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-v2-optimism-new-batch</code> | notes | channel registered only for CHAIN_TYPE optimism; no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-v2-optimism-new-deposits</code> | notes | channel registered only for CHAIN_TYPE optimism |
| <code>interface.14.ws-explorer-join-socket-v2-paxeer-x-feed-refused</code> | notes | refuses any paxeer_x_feed topic other than new_update and market:&lt;id&gt; |
| <code>interface.14.ws-explorer-join-socket-v2-rewards-address-hash-string</code> | notes | no in-repo client found |
| <code>interface.14.ws-explorer-join-socket-v2-transactions-stats</code> | notes | no in-repo client found |
