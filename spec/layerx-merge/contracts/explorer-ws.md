# Explorer ws contract

The explorer publishes indexed Paxeer mainnet-beta data through Phoenix channels. Paxeer is one BFT network: the kernel participates in the network and proposes blocks, the EVM chain id is `125` (`0x7d`), and a receipt is final once its block is committed. Explorer notifications report indexer progress and data; they do not establish consensus commitment. For receipt and block evidence, see the [interop committed proof contract](interop-committed-proof.md).

Channel registration and event publication are separate: a deployment publishes notifications only when the corresponding producer runs.

## channels

The registrations live in [UserSocket](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex) and [V2.UserSocket](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex). The two sockets use different topic prefixes and payload formats.

| Socket | Topic pattern | Channel module under `BlockScoutWeb` |
|---|---|---|
| `/socket` | `addresses_old:*` | `AddressChannel` |
| `/socket` | `blocks_old:*` | `BlockChannel` |
| `/socket` | `exchange_rate_old:*` | `ExchangeRateChannel` |
| `/socket` | `rewards_old:*` | `RewardChannel` |
| `/socket` | `transactions_old:*` | `TransactionChannel` |
| `/socket` | `tokens_old:*` | `TokenChannel` |
| `/socket` | `token_instances_old:*` | `TokenInstanceChannel` |
| `/socket/v2` | `addresses:*` | `V2.AddressChannel` |
| `/socket/v2` | `blocks:*` | `V2.BlockChannel` |
| `/socket/v2` | `exchange_rate:*` | `V2.ExchangeRateChannel` |
| `/socket/v2` | `rewards:*` | `V2.RewardChannel` |
| `/socket/v2` | `transactions:*` | `V2.TransactionChannel` |
| `/socket/v2` | `tokens:*` | `V2.TokenChannel` |
| `/socket/v2` | `token_instances:*` | `TokenInstanceChannel` |
| `/socket/v2` | `zkevm_batches:*` | `V2.PolygonZkevmConfirmedBatchChannel` |
| `/socket/v2` | `paxeer_x_feed:*` | `V2.PaxeerXFeedChannel` |

`arbitrum:*` and `optimism:*` register on `/socket/v2` only for the corresponding compile-time `:explorer, :chain_type`. The [local deployment](../../../services/explorer/deploy/docker-compose.local.yml) defaults `CHAIN_TYPE` to `paxeer_x`. The zkEVM channel is registered unconditionally, but its notifications require the corresponding indexer events; its presence does not describe an additional Paxeer network.

## graphql-subscription

The legacy `/socket` also uses `Absinthe.Phoenix.Socket` with [BlockScoutWeb.GraphQL.Schema](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/graphql/schema.ex). Its subscription field is:

```elixir
field :token_transfers, list_of(:token_transfer) do
  arg(:token_contract_address_hash, non_null(:address_hash))
  config(fn args, _info ->
    {:ok, topic: to_string(args.token_contract_address_hash)}
  end)
end
```

The argument is a token contract address. The subscription config converts it to a string topic, and [Notifier](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex) publishes transfers grouped by `token_contract_address_hash` to the same Absinthe topic. This is an Absinthe subscription, distinct from joining a Phoenix `tokens:` topic. Schema middleware checks the `:enabled` setting under `:block_scout_web, Api.GraphQL`; [runtime.exs](../../../services/explorer/backend/config/runtime.exs) sets it from `API_GRAPHQL_ENABLED`, defaulting to `true`. When disabled, the middleware returns `"GraphQL API is disabled."`.

## pushes

[BlockScoutWeb.Notifier](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex) produces the following V2 broadcasts. Braced payloads list their top-level fields; renderers supply the nested API objects.

| Topic | Event | Payload |
|---|---|---|
| `tokens:<token_contract_address_hash>` | `token_transfer` | `{token_transfer: count}` for that token in this notification batch |
| `tokens:<token_contract_address_hash>` | `total_supply` | `{total_supply: string}` |
| `addresses:<address_hash>` | `transaction`, `pending_transaction` | `{transactions: [...]}` rendered by `API.V2.TransactionView` |
| `addresses:<address_hash>` | `token_transfer` | `{token_transfers: [...]}` rendered by `API.V2.TransactionView` |
| `addresses:<address_hash>` | `balance` | `{balance, block_number, exchange_rate}` |
| `addresses:<address_hash>` | `coin_balance` | `{coin_balance}` rendered by `API.V2.AddressView` |
| `addresses:<address_hash>` | `current_coin_balance` | `{coin_balance, exchange_rate, block_number}` |
| `addresses:<address_hash>` | `token_balance` | `{block_number}` |
| `addresses:<address_hash>` | `updated_token_balances_<type>` | `{token_balances, overflow}`; type is lowercased with hyphens replaced by underscores, at most 50 balances per type |
| `addresses:<address_hash>` | `verification_result` | `{status: "success"}` or `{status: "error", errors}` |
| `addresses:<address_hash>` | `fetched_bytecode`, `changed_bytecode` | `{fetched_bytecode}` or `{}` respectively |
| `addresses:<address_hash>` | `smart_contract_was_verified`, `smart_contract_was_not_verified`, `eth_bytecode_db_lookup_started` | `{}` |
| `addresses:new_address` | `count` | `{count}` is broadcast, but the V2 address join validates the suffix as an address, so this broadcast topic is not a usable address subscription |
| `blocks:new_block`, `blocks:<miner_address>` | `new_block` | `{block, average_block_time}`; average block time is a string in milliseconds |
| `blocks:indexing`, `blocks:indexing_internal_transactions` | `index_status` | `{ratio: decimal_string, finished: boolean}` via `broadcast_indexed_ratio/2` |
| `transactions:new_transaction` | `transaction` | `{transaction: count}` |
| `transactions:new_pending_transaction` | `pending_transaction` | `{pending_transaction: count}` |
| `transactions:<transaction_hash>` | `raw_trace` | `{raw_trace}` rendered by `API.V2.TransactionView` |
| `rewards:<address_hash>` | `new_reward` | `{reward: 1}` when emission-funds rewards are enabled |
| `token_instances:<token_contract_address_hash>` | `fetched_token_instance_metadata` | `{token_id: string, fetched_metadata}` |
| `token_instances:<token_contract_address_hash>` | `not_fetched_token_instance_metadata` | `{token_id: string, reason}` |
| `paxeer_x_feed:new_update` | `new_update` | Native submission rendered by `API.V2.PaxeerX.FeedView.prepare_update/3` |
| `paxeer_x_feed:market:<market_id>` | `new_update` | Accepted observation from a successful submission, rendered by `FeedView.prepare_observation_update/1` |
| `zkevm_batches:new_zkevm_confirmed_batch` | `new_zkevm_confirmed_batch` | `{batch}` rendered by `API.V2.PolygonZkevmView` |

Native price updates also emit transaction counts and refreshed block notifications for updates within the notifier's 10-block live window. A repeated `new_block` can therefore update an already displayed block. Counts are notification-batch counts, not cumulative totals or proofs of finality.

For the conditional chain types, [Notifiers.Arbitrum](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifiers/arbitrum.ex) emits `new_arbitrum_batch` on `arbitrum:new_batch` and `new_messages_to_rollup_amount` on the same-named topic. [Notifiers.Optimism](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifiers/optimism.ex) emits `new_optimism_batch` on `optimism:new_batch` and `new_optimism_deposits` on `optimism:new_deposits`.

The legacy socket retains its own wire format. Its channel `handle_out` functions may turn notifier data into HTML, so notifier input maps must not be treated as legacy client payloads:

| Legacy topic | Client events and behavior |
|---|---|
| `tokens_old:<token_contract_address_hash>` | `token_transfer` becomes `{token_transfer_hash, token_transfer_html}`; notifier `token_total_supply` becomes client event `total_supply` with a formatted supply |
| `transactions_old:new_transaction`, `transactions_old:new_pending_transaction` | `transaction` or `pending_transaction` becomes `{transaction_hash, transaction_html}` |
| `transactions_old:<transaction_hash>` | `collated` and rendered `raw_trace` |
| `transactions_old:stats` | `update` with `{stats}`; a V2 stats join exists but this publisher targets the legacy topic |
| `blocks_old:new_block`, `blocks_old:<miner_address>` | `new_block` includes rendered block HTML and formatted duration |
| `rewards_old:<address_hash>` | `new_reward` becomes `{reward_html}` |
| `addresses_old:<address_hash>` | Balance, coin/token balance, transaction, internal-transaction, token-transfer, verification and bytecode events pass through `AddressChannel` rendering/interception |
| `addresses_old:new_address` | The notifier broadcasts `count`; legacy address join still applies address validation |
| `token_instances_old:<token_contract_address_hash>` | `TokenInstanceChannel` subscribes to the V2 `token_instances:` topic and forwards metadata events |
| `exchange_rate_old:new_rate` | Exchange-rate data goes through `ExchangeRateChannel`; V2 `exchange_rate:new_rate` broadcasts `{exchange_rate, available_supply, chart_data}` |

### Topic encoding and payload address casing

Topic names are exact strings. Use `0x` followed by 40 lowercase hex digits for an address suffix, matching [Explorer.Chain.Hash](../../../services/explorer/backend/apps/explorer/lib/explorer/chain/hash.ex) string conversion. Token transfer topics use the **token contract address**, including for an NFT instance page; a token id and a transaction hash are not topic suffixes for these subscriptions.

[V2.TokenChannel](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/token_channel.ex) still names its wildcard variable `_transaction_hash`, but accepts any suffix without validation. [TokenTransfer.tsx](../../../apps/explorer/ui/token/TokenTransfer/TokenTransfer.tsx) joins `tokens:${router.query.hash.toLowerCase()}` on a token page, and the notifier broadcasts by `token_contract_address_hash`. A successful join with an individual transaction hash can remain silent.

Payload address fields have a separate representation. [API.V2.Helper](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/api/v2/helper.ex) renders address `hash` values with `Explorer.Chain.Address.checksum`, so `from.hash` and `to.hash` can contain mixed-case hex. Compare decoded addresses or normalize both sides for identity comparisons, while preserving checksum casing for display.

The current [address token-transfer consumer](../../../apps/explorer/ui/address/useAddressTokenTransfersSocket.ts) lowercases its topic but compares `transfer.from.hash` and `transfer.to.hash` to the supplied address with case-sensitive equality in directional filters. Directional filters therefore require matching address casing. The [address transaction consumer](../../../apps/explorer/ui/txs/socket/useTxsSocketTypeAddress.tsx) already lowercases both sides of its directional comparisons.

## rust-phoenix-channel

The [actix-phoenix-channel library](../../../services/explorer/services/libs/actix-phoenix-channel/src/actix_handler.rs) exports `configure_channel_websocket_route<T: ChannelHandler>`, which registers `/socket/websocket` when a caller mounts it. It requires `vsn=2.0.0`; missing or unsupported versions return HTTP 400. Its example and test mount the route, but explorer service binaries do not call this configurator. This library route is separate from the Elixir endpoint's registrations.

## sockets

[BlockScoutWeb.Endpoint](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex) mounts `/socket` and `/socket/v2` when `:block_scout_web, :disable_api?` is false. Both specify a WebSocket timeout of `45_000` milliseconds. The legacy `UserSocket.connect/2` optionally assigns `locale`; supply `locale` for legacy HTML renderers that read `socket.assigns.locale`. The V2 socket accepts connection params without assigning a locale. Both return `nil` from `id/1`.

The frontend [getSocketUrl](../../../apps/explorer/lib/api/getSocketUrl.ts) combines `config.apis.general.socketEndpoint`, its optional `basePath`, and `/socket/v2`. These are explorer service URLs. Public EVM JSON-RPC uses the node root path; Phoenix channels use the explorer backend's own host and protocol.

## topics

Join a topic appropriate to the socket version. The V2 join implementations are in the [channels/v2 directory](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2); legacy and shared implementations are in [channels](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels).

| V2 topic | Suffix and join behavior |
|---|---|
| `addresses:<address_hash>` | Address validation and restricted-access check; success assigns `address_hash` |
| `blocks:new_block`, `blocks:indexing`, `blocks:indexing_internal_transactions` | Explicitly accepted common topics |
| `blocks:<miner_address>` | Address validation and restricted-access check |
| `exchange_rate:new_rate` | Exact topic |
| `rewards:<address_hash>` | Address validation and restricted-access check |
| `transactions:new_transaction`, `transactions:new_pending_transaction`, `transactions:stats` | Explicitly accepted common topics |
| `transactions:<transaction_hash>` | Wildcard accepted without hash validation |
| `tokens:<token_contract_address_hash>` | Wildcard accepted; use the lowercase token contract address |
| `token_instances:<token_contract_address_hash>` | Wildcard accepted by the shared `TokenInstanceChannel` |
| `paxeer_x_feed:new_update` | All native feed submissions |
| `paxeer_x_feed:market:<market_id>` | Entire suffix must parse as a nonnegative integer; otherwise `{reason: "invalid market id"}` |
| Other `paxeer_x_feed:` topics | Refused with `{reason: "unknown topic"}` |
| `zkevm_batches:new_zkevm_confirmed_batch` | Exact topic |
| `arbitrum:new_batch`, `arbitrum:new_messages_to_rollup_amount` | Exact topics, only when the Arbitrum channel is compiled |
| `optimism:new_batch`, `optimism:new_deposits` | Exact topics, only when the Optimism channel is compiled |

A normal V2 join returns `{:ok, %{}, socket}`; address-based refusals return `{:error, %{reason: reason}}` from [AccessHelper](../../../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/views/access_helper.ex), whose reasons are `"Invalid address hash"` and `"Restricted access"`. Legacy equivalents use the `_old` prefixes listed above. `token_instances_old:<address>` explicitly subscribes to the corresponding shared V2 topic; other legacy topics retain their legacy handlers.

[useSocketChannel](../../../apps/explorer/lib/socket/useSocketChannel.tsx) joins through `socket.channel(topic)`, shares channels by topic with a subscriber count, and leaves when the last subscriber unmounts or the channel is disabled. It reports socket closure, socket errors and join errors through callbacks. The token transfer view disables its channel during placeholder/error states and off the first pagination page. These live updates have no replay cursor in this contract; fetch indexed API data when reconstructing a view after disconnection.
