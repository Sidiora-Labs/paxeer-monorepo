# Tendermint RPC

The route table is in [`routes.go`](./routes.go); the request and response types are in [`../../../rpc/coretypes`](../../../rpc/coretypes/).

## Pagination

`validators`, `tx_search`, `block_search` and `unconfirmed_txs` use `page` and
`per_page`. The default page is 1 and the default page size is 30. An omitted
or nonpositive `per_page` selects the default. Values above 100 are capped at
100 unless the RPC `unsafe` option is enabled. These rules are implemented in
[`env.go`](env.go); other collection routes use their own limits.

## Subscribing to events

The user can subscribe to events emitted by the engine by sending the JSON-RPC
method `subscribe` over the internal `/websocket` endpoint, when
`experimental-disable-websocket` is false. An error is returned if the maximum number of clients
(`max-subscription-clients`) is reached, if the client has too many
subscriptions (`max-subscriptions-per-client`), or if the query is longer than
512 bytes. Setting up a subscription has a 5-second timeout; this does not limit
the subscription's lifetime. Each subscription has a buffer of
100 events to accommodate short bursts of events or some slowness in clients.
If the subscription is terminated by the publisher, the client receives the
error "subscription terminated by publisher". The user can unsubscribe using
either `unsubscribe` or `unsubscribe_all` over the same WebSocket connection.
