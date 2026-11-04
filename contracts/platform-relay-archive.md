# Relay and archive migration contract

The standalone relay/archive service is **retired**. Its former `services/platform/relay_archive` tree is absent. This page preserves contract anchors for migration; it does not describe a service to install. Paxeer is one BFT network: the kernel participates in `paxd` block proposal, and a receipt is final when its block commits. Full and archive `paxd` nodes provide chain history. Tenant domains do not require a relay process or another chain.

See [the retirement inventory](../spec/layerx-merge/programme/data/retired.kvx), component `replica`, for the deletion boundary. Use [hosted history and indexing](platform-hosted.md#indexer-http-routes) for application projections and [the gateway](platform-gateway.md#gateway-jsonrpc-methods) for hosted history methods. An indexer's completeness watermark and an archive node's retained state are different guarantees.

## Relay archive config keys

The old relay/archive configuration schema and `config.example.json` are retired. No keys from that file configure the merged network. Configure the node's own retention and state services through its maintained node configuration; configure a hosted indexer independently. Copying the old replica, forwarder or archive settings does not enable either facility.

## Relay archive CLI commands

The old `install.sh` and `runtime.py` commands are retired. There is no supported relay/archive install, forward, replay or state-archive command at the deleted path. Use the native `paxd` lifecycle and operator procedures. The [hosted node wrapper](../services/platform/hosted/node/native_runtime.py) prepares and supervises a native node and checks committed health; it is not the removed relay/archive runtime.

## Relay archive env vars

The deleted installer and runtime's environment variables are retired. They are not aliases for [current hosted configuration](platform-hosted.md#env-vars). Do not carry old credentials or service origins into node or indexer configuration by renaming them mechanically.

## Relay archive error codes

The deleted runtime, forwarder and state archive's error vocabulary is retired. New integrations handle the actual node JSON-RPC, gRPC or hosted API errors at the selected boundary. Missing history, an unavailable node and an incomplete indexer projection must remain distinguishable from a successful empty query.

## Relay archive file formats

The old relay store and state-archive formats are retired, including the files owned by `store.py` and `state_archive.py`. They are not native node snapshots or a supported import format. Preserve any historical material under the operator's retention policy; this page specifies no automatic conversion into trusted chain state. Native history must be read and verified through the maintained node interfaces.

## Relay archive HTTP routes

The former runtime, forwarding and state-archive HTTP endpoints are retired. Public Ethereum JSON-RPC uses the HTTPS root of `api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`, or `api-mainnet-beta.paxeer.network`, with chain ID 125 (`0x7d`). The retired `/rpc` family is not a public network route. Hosted indexer HTTP reads are documented [separately](platform-hosted.md#indexer-http-routes); their deployment address is configuration, not a public node alias.

<a id="relay-archive-jsonrpc-methods"></a>

## Relay archive JSON-RPC methods

The old relay/archive method registrations are retired. The node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`; hosted `px_` methods belong to the [gateway contract](platform-gateway.md). For protobuf query and transaction services, use [the protobuf API contract](proto-api.md). `node.hyperpaxeer.com` is the library mirror, not an RPC endpoint or history service.
