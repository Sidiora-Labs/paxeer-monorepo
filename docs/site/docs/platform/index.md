# Developer platform

The developer platform provides hosted identity, account and history reads,
payment tooling, program discovery and interoperability services for Paxeer X
Network. The network is one BFT chain run by `paxd`, with EVM chain ID `125`
(`0x7d`); its kernel participates in block proposal and execution. Receipts
become final when their block is committed. Sub-chains are tenant domains in
the database and can be launched without new chain infrastructure.

## Connect to the network

Use root-path node JSON-RPC at
`https://api-mainnet-beta.paxeer.network` or
`https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. Check `eth_chainId` before
signing, keep multiple nodes configured and compare their block heights.
The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`
and `txpool`; see [Public RPC endpoints](../reference/public-rpc.md).

The former unified router, `/rpc` and `/rpc/ws` public endpoints are retired.
The separate LayerX interface and sequencer service deployment model are retired.
`node.hyperpaxeer.com` is only the hpx library mirror, never an RPC endpoint.

Hosted gateway methods are a distinct service contract for this same network.
Obtain the gateway origin from its operator and post JSON-RPC to its root.
The 17 declared methods in
`services/platform/hosted/gateway/openrpc.json` cover account resolution,
balances, indexed history, registration, funding claims, consensus evidence,
capabilities and the node receipt relay. They are not automatically available
on every public node. The gateway refuses all retired `lx_` calls with
`-32601`. See [Hosted gateway](hosted-gateway.md) for methods, credentials,
configuration and readiness.

## Hosted surfaces

| Surface | Guide |
| --- | --- |
| Hosted gateway, root JSON-RPC and protected REST reads | [Hosted gateway](hosted-gateway.md) |
| Public node HTTP and WebSocket access | [Public RPC endpoints](../reference/public-rpc.md) |
| Receipt states and payment evidence | [Commitment levels](../protocol/commitment-levels.md) |
| Private funding service | [Faucet](faucet.md) |
| Program discovery | [Registry](registry.md) |
| Event delivery | [Webhooks](webhooks.md) |
| Hosted identity and sessions | [Identity](identity.md) |
| Human interface | [Hosted Human](../human/hosted.md) |
| Developer tooling | [CLI](cli.md) |

Hosted service boundaries extend the network's product APIs. They do not
introduce another execution chain or a second finality decision. Service
availability depends on operator configuration; a route catalogue or healthy
process is not proof that a complete feature journey works.

## Funding and payments

Fund an EVM account with a native on-chain transfer on chain `125`, then
confirm its balance with node-root `eth_getBalance`. Asset-specific balances
can be read through the hosted gateway's `px_getBalances`. Funding is not a
custody-credit deposit into an independent execution domain.

Where enabled by an operator, `px_requestFunds` claims an authenticated
principal's starter credit through the hosted faucet. It requires a Bearer
identity session, a DID and its authorised signer public key; pending claims
return an explicit error and must be retried with the same identity. This is
not an unauthenticated public faucet. See [Getting started](../overview/getting-started.md)
for initial account setup.

Submit signed EVM transactions through node-root `eth_sendRawTransaction`.
Keep the original signed transaction, nonce and hash through timeouts. A
returned transaction hash acknowledges submission; read and verify receipt
evidence before treating a payment as successful. Payment middleware accepts
`receipted` or `committed` requirements; only committed-block evidence proves
network finality. See [Commitment levels](../protocol/commitment-levels.md)
for verification and recovery.

## Developer benchmark

The platform specification in `spec/layerx-platform/spec.kvx` sets a target
of integrating payments in fewer than ten lines and completing a verified
test payment within five minutes from a clean environment. Those are
qualification targets; this page does not record a passing measurement.
Use the current node and hosted contracts above when evaluating an
integration, with independently configured verification trust.

[Home](../index.md)
