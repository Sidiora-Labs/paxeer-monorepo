<!--
order: 1
-->

# Concepts

The `transfer` module implements ICS20 fungible token transfers through IBC channels.
On Paxeer, it runs in the same network as the EVM and kernel: Paxeer mainnet-beta
uses EVM chain id `125` (`0x7d`), and its kernel participates in block proposal.
A receipt is final when its block is committed. IBC transfers to a counterparty
chain still require packet and acknowledgement proofs verified by IBC clients.

## Acknowledgements

ICS20 uses the acknowledgement envelope described in
[ICS 04](https://github.com/cosmos/ibc/tree/master/spec/core/ics-004-channel-and-packet-semantics#acknowledgement-envelope).
The [application callbacks](../ibc_module.go) return a synchronous acknowledgement:

- Success sets `Response` to `Acknowledgement_Result{Result: []byte{1}}`, encoded
  as JSON `{"result":"AQ=="}`.
- A packet JSON decoding failure returns
  `{"error":"cannot unmarshal ICS-20 transfer packet data"}`.
- A receive-keeper failure uses [NewErrorAcknowledgement](../types/ack.go) to
  return `{"error":"ABCI code: <code>: error handling packet on destination chain: see events for details"}`.
  The ABCI code is included instead of a potentially nondeterministic error log.

An error acknowledgement causes a refund when relayed to the sending chain.
A proven timeout also refunds the sender; a successful acknowledgement does not.
See [Events](05_events.md) for the callback attributes and
[core packet concepts](../../../core/spec/01_concepts.md#sending-receiving-acknowledging-packets)
for the commitment lifecycle.

## Denomination Trace

A denomination trace consists of `path` and `base_denom`. The path contains
port/channel pairs, newest first. The base denomination can itself contain
slashes. [ParseDenomTrace](../types/trace.go) recognizes leading pairs with
canonical channel identifiers such as `channel-0`; names such as `channelToA`
are not suitable trace examples for this implementation.

For a non-empty path, the bank denomination is `ibc/{HASH}`, where `HASH` is the
uppercase hexadecimal SHA-256 hash of `path + "/" + base_denom`. For example,
`transfer/channel-0/uatom` becomes `ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2`.
A trace with an empty path uses its base denomination directly. The full trace is stored as described in
[State](02_state.md); packet data carries the full denomination path rather
than the `ibc/` hash.

The source/sink role is relative to a transfer's channel, not a permanent label
for a chain:

- If the outgoing full denomination does not start with the sending port/channel
  prefix, the sender escrows the coins. The receiver prefixes its own destination
  port/channel pair, records the trace and mints vouchers.
- If it does start with that prefix, the sender burns the vouchers. The receiver
  removes the packet's source port/channel prefix and unescrows the corresponding
  coins from its destination-channel escrow account.

These operations are implemented in [the transfer keeper](../keeper/relay.go).
See [State Transitions](03_state_transitions.md) and
[ADR 001: Coin Source Tracing](../../../../docs/architecture/adr-001-coin-source-tracing.md)
for the tracing model.

### UX suggestions for clients

Wallets, exchanges and explorers should resolve the stored trace and channel
metadata rather than infer the origin from a ticker or a channel number.
Identifiers are local to each chain; the two ends of a channel need not share
an identifier.

#### Direct connection

For a voucher on chain B with trace `transfer/channel-0/uatom`, use B's IBC query
service to find the counterparty chain A. Where a Cosmos gRPC gateway is enabled,
the proto-defined HTTP requests are:

1. `GET /ibc/apps/transfer/v1/denom_traces/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2`
   returns `{"denom_trace":{"path":"transfer/channel-0","base_denom":"uatom"}}`.
2. `GET /ibc/core/channel/v1/channels/channel-0/ports/transfer`
   returns a `channel` object plus `proof` and `proof_height`. Read
   `channel.counterparty.port_id` and `channel.counterparty.channel_id` to learn
   A's channel end, and `channel.connection_hops` for B's connection identifiers.
3. `GET /ibc/core/channel/v1/channels/channel-0/ports/transfer/client_state`
   returns `identified_client_state`, `proof` and `proof_height`. The nested
   `identified_client_state.client_id` identifies B's client of A;
   `identified_client_state.client_state` is a protobuf `Any`. For a Tendermint
   client, its decoded `chain_id` identifies A.
4. To inspect A's view of B, query A's service with the counterparty port/channel
   from step 2. Do not use A's local channel identifier against B's service.

The routes and envelopes are declared in the
[transfer query proto](../../../../proto/ibc/applications/transfer/v1/query.proto)
and [channel query proto](../../../../proto/ibc/core/channel/v1/query.proto).
The denom-trace query returns gRPC `InvalidArgument` for an invalid hash and
`NotFound` when the trace is absent. Channel and client queries can also fail
when their referenced state is missing; the client-state response is not a flat
object containing a client ID and chain ID.

These are Cosmos query-service routes, available on a configured gRPC gateway.
Their declaration does not establish their availability on public Ethereum
JSON-RPC hosts. Public Paxeer JSON-RPC uses the HTTPS root path, for example
`https://api1.mainnet-beta.paxeer.network`, with EVM chain id `125`.

The denom-trace response above is checked against a packet-produced voucher,
the registered query service and the generated HTTP gateway in
`TestRepairM_1206_QueryShape` in [the repair tests](../repair_m1206_test.go).

### Multiple hops

Here A, B and C are external IBC chains with separate clients, channels and
bank stores. Tenant namespaces inside Paxeer are not additional IBC chains
and do not add denomination-trace hops.

For transfers `A -> B -> C`, suppose B receives on `transfer/channel-0` and C
receives on `transfer/channel-7`. C stores the trace
`transfer/channel-7/transfer/channel-0/uatom`. The outer pair belongs to C and
identifies its channel to B; after following that hop, the next pair must be
interpreted on B. A direct C-to-A connection, if one exists, does not identify
the B-to-A channel used by this voucher.

To reconstruct the path, repeat the [direct-connection queries](#direct-connection)
on each relevant chain, interpreting one local pair at a time. An indexer or
relayer service may cache this mapping for clients. Treat such a service's
unverified mappings as third-party information; verify state proofs with the
appropriate light clients when trustless attribution is required. This module
does not expose a global network-topology query or automatically forward a
packet over all the hops in a trace.

## Locked Funds

A frozen or expired client can prevent packet, acknowledgement and timeout
proofs from being processed, leaving funds unavailable until the required
client and channel operations can proceed. The recovery mechanism is a
`ClientUpdateProposal` naming a non-active subject client and an active
substitute with a strictly greater latest height. The client implementation
checks compatibility and copies the required state; governance does not simply
supply a replacement header. See
[Client Misbehaviour](../../../core/spec/01_concepts.md#client-misbehaviour).
Recovery is conditional on these checks and does not reopen closed channels.

To recover the original denomination, send vouchers back through the exact
channel route in reverse order. Sending them to the same chain through another
channel adds a different trace rather than releasing the original escrow. If
an intermediate channel closes, normal reverse-route redemption through that
channel is unavailable. Closing a channel does not itself return all outstanding
vouchers; timeout refunds require proof that the individual packet was not
received.

## Security Considerations

Only the transfer module should mint denominations in the `ibc/` namespace.
Other modules must not be allowed to forge vouchers backed by transfer escrow.
The module's [send and receive parameters](07_params.md) apply to all
transfer channels; they do not replace packet proof verification or channel
capability checks.
