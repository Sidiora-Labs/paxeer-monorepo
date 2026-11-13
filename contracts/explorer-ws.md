# Explorer WebSocket contract

The explorer uses Phoenix channels to publish indexed data from the single Paxeer BFT network (EVM chain ID **125**, `0x7d`). Its kernel participates in block proposal. A transaction is final when its block is committed; a socket message announcing a pending item or refreshing an index does not establish commitment. See [HTTP snapshots and transaction status](explorer-http-api.md) and [metrics](explorer-metrics.md).

## Sockets

[Endpoint](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/endpoint.ex) declares `/socket` for the legacy UI and `/socket/v2` for the current API/UI in its API-enabled branch; a build with `disable_api?` does not mount these sockets. Both set a WebSocket timeout of 45,000 ms. Use a Phoenix client with the explorer origin; it supplies the `/websocket` transport suffix and Phoenix protocol parameters. This is not node `eth_subscribe` or hosted gateway streaming.

Both socket modules accept connections without authenticating a user and return no socket ID (`nil`). Individual channels can validate addresses and restricted access. A successful connection does not authorize account mutations or receipt claims. Legacy socket connections can carry `locale` for server-rendered output.

## Channels

[V2.UserSocket](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/v2/user_socket.ex) maps these patterns:

| Pattern | Handler |
| --- | --- |
| `addresses:*` | `BlockScoutWeb.V2.AddressChannel` |
| `blocks:*` | `BlockScoutWeb.V2.BlockChannel` |
| `exchange_rate:*` | `BlockScoutWeb.V2.ExchangeRateChannel` |
| `rewards:*` | `BlockScoutWeb.V2.RewardChannel` |
| `transactions:*` | `BlockScoutWeb.V2.TransactionChannel` |
| `tokens:*` | `BlockScoutWeb.V2.TokenChannel` |
| `token_instances:*` | `BlockScoutWeb.TokenInstanceChannel` |
| `zkevm_batches:*` | `BlockScoutWeb.V2.PolygonZkevmConfirmedBatchChannel` |
| `paxeer_x_feed:*` | `BlockScoutWeb.V2.PaxeerXFeedChannel` |

`arbitrum:*` and `optimism:*` are registered only for their respective compile-time chain types. These shared-code channels do not add settlement layers to Paxeer.

[UserSocket](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/channels/user_socket.ex) registers the legacy patterns `addresses_old:*`, `blocks_old:*`, `exchange_rate_old:*`, `rewards_old:*`, `transactions_old:*`, `tokens_old:*`, and `token_instances_old:*`. Legacy handlers can transform notifications into HTML; V2 payloads must not be inferred from those templates.

## Topics

A wildcard registration selects a handler; the handler's `join` clauses determine accepted topics. Successful V2 joins generally reply with an empty object, not a state snapshot.

| Topic on `/socket/v2` | Join behavior |
| --- | --- |
| `blocks:new_block` | Block notifications |
| `blocks:indexing`, `blocks:indexing_internal_transactions` | Indexing progress |
| `blocks:<miner_address>` | Validates address and restricted access |
| `addresses:<address_hash>` | Validates address/restricted access and assigns the address |
| `rewards:<address_hash>` | Validates address and restricted access |
| `exchange_rate:new_rate` | Accepts the rate topic |
| `transactions:new_transaction` | New transaction count notifications |
| `transactions:new_pending_transaction` | Pending transaction count notifications |
| `transactions:stats` | Accepted by TransactionChannel |
| `transactions:<transaction_hash>` | Accepted by TransactionChannel; the suffix is not hash-validated there |
| `tokens:<suffix>` | Accepted by TokenChannel without suffix validation |
| `token_instances:<suffix>` | Accepted by TokenInstanceChannel without suffix validation |
| `paxeer_x_feed:new_update` | Native price submission notifications |
| `paxeer_x_feed:market:<id>` | Requires a nonnegative integer with no trailing characters |

The feed channel replies with `{"reason":"invalid market id"}` for a malformed market ID and `{"reason":"unknown topic"}` for other unmatched topics. Address-based channels return an error object with the validation reason. Topic acceptance alone does not prove that any publisher produces events for that suffix.

## Pushes

[Notifier](../services/explorer/backend/apps/block_scout_web/lib/block_scout_web/notifier.ex) and the individual channels define the actual event names and envelopes. The main V2 notifications are:

| Topic | Event | Payload meaning |
| --- | --- | --- |
| `blocks:new_block` and miner-specific block topics | `new_block` | `{block, average_block_time}`; average time is a millisecond string |
| Block indexing topics | `index_status` | Indexing progress, including `finished` and `ratio` |
| `transactions:new_transaction` | `transaction` | `{transaction: count}`; not a transaction object |
| `transactions:new_pending_transaction` | `pending_transaction` | `{pending_transaction: count}` |
| `transactions:<hash>` | `raw_trace` | `{raw_trace: [...]}` after an on-demand trace fetch |
| `addresses:<hash>` | `transaction`, `pending_transaction` | Transaction arrays under `transactions` |
| `addresses:<hash>` | `token_transfer` | Transfers under `token_transfers` |
| `addresses:<hash>` | `balance`, `current_coin_balance`, `coin_balance`, `token_balance` | Balance value/history or invalidation data specific to the event |
| `addresses:<hash>` | `changed_bytecode`, `fetched_bytecode`, `verification_result` | Contract information/verification updates |
| `tokens:<hash>` | `token_transfer`, `total_supply` | Transfer count or supply update |
| `token_instances:<hash>` | `fetched_token_instance_metadata`, `not_fetched_token_instance_metadata` | Metadata fetch result |
| `exchange_rate:new_rate` | `new_rate` | `exchange_rate`, `available_supply`, and `chart_data` |
| `rewards:<hash>` | `new_reward` | Block reward notification |
| `paxeer_x_feed:new_update` | `new_update` | One full submission rendered by `FeedView.prepare_update` |
| `paxeer_x_feed:market:<id>` | `new_update` | One accepted market observation rendered by `FeedView.prepare_observation_update` |

Feed publication iterates submissions oldest first; market pushes require a successful submission (`status == 1`) and an accepted observation. The overall feed includes failed submissions too. Feed items use the same rendering as [HTTP reads](explorer-http-api.md#api-v2-paxeer-x), including the native submission type 127. Do not parse decimal-string prices or amounts through JavaScript floating-point numbers.

<a id="raw-trace-limitation"></a>

### Raw trace reads

Read `GET /api/v2/transactions/:hash/raw-trace` after indexing for the trace array.
The V2 on-demand trace notification wraps the rendered array under `raw_trace`;
the HTTP action returns the array directly. An empty or missing push does not
establish that a transaction has no traces.

### Transaction commitment

V2.TransactionChannel accepts per-hash joins but does not implement a per-hash commitment publisher. The legacy channel intercepts `collated`; the frontend also names a `collated` status event, but those declarations do not prove a V2 producer. Read `/api/v2/transactions/:hash/status` to determine `submitted`, `receipted`, or `committed`. A transaction count, block notification, or raw-trace event cannot substitute for that status evidence.

## Connection lifecycle and recovery

These channel joins carry no durable replay cursor or missed-event log. After disconnect/restart, join the desired topics again and fetch HTTP snapshots to reconcile the indexed state. Treat counts as invalidations and deduplicate items by their resource identities; repeated block/feed announcements can update an existing item.

The frontend [SocketProvider](../apps/explorer/lib/socket/context.tsx) owns the Phoenix connection and disconnects on unmount. [useSocketChannel](../apps/explorer/lib/socket/useSocketChannel.tsx) shares channels by topic with subscriber counts; the last subscriber leaves and removes the registry entry. It installs close/error callbacks when provided. [useSocketMessage](../apps/explorer/lib/socket/useSocketMessage.tsx) removes its event handler on cleanup.

Consumers should refetch HTTP snapshots when channels rejoin; the shared socket
hooks manage subscriptions, while each component owns its data reconciliation.
[useFeedSocket](../apps/explorer/ui/paxeerX/feed/useFeedSocket.ts) marks the feed
down on close/error and uses a five-second HTTP fallback while unjoined;
overview/history buffers are bounded at 200/50. Refresh transaction status after
new blocks or recovery.

## GraphQL subscription

The legacy `/socket` uses `Absinthe.Phoenix.Socket` with `BlockScoutWeb.GraphQL.Schema`. Its `token_transfers(token_contract_address_hash: AddressHash!)` subscription belongs to GraphQL, not a V2 channel topic. Use the GraphQL schema and feature configuration in [the HTTP contract](explorer-http-api.md#api-v1-graphql).

## Rust Phoenix channel

The separate [actix-phoenix-channel library](../services/explorer/services/libs/actix-phoenix-channel/src/lib.rs) exports `configure_channel_websocket_route`, channel handlers, connections, and broadcasters. Its helper registers `/socket/websocket` and handles Phoenix joins, leaves, and heartbeats. A consuming Rust service must mount the helper and authorize its own topics; the library does not mount the Elixir `/socket/v2` contract or provide replay persistence.
