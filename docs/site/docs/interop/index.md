# Interoperability

Adapters translate external protocols into verified payment meaning and
network transactions. Paxeer X Network is one BFT network with EVM chain
ID `125` (`0x7d`); its kernel participates in block proposal and execution.
Native ledger operations update balances, and receipts become final when
their blocks are committed. An edge adapter has no independent balance,
minting or settlement authority.

The native ledger module is `modules/layerxledger`, exposed to EVM callers
at `0x000000000000000000000000000000000000101a`. Interop payment paths bind
verified execution logs from that address to the authorised payer, payee,
asset, amount and reference. A mandate signature authorises payment; a
matching execution receipt establishes the network outcome. These are
distinct evidence claims.

## Surfaces in this tree

| Surface | Tree | Page |
| --- | --- | --- |
| x402 v2 | `services/interop/crates/layerx-x402` | [x402](x402.md) |
| AP2 | `services/interop/crates/layerx-ap2` | [AP2](ap2.md) |
| UCP | `services/interop/crates/layerx-ucp` | [UCP](ucp.md) |
| Visa TAP | `services/interop/crates/layerx-visa-tap` | [Visa TAP](visa-tap.md) |
| Portable receipts | `services/interop/crates/layerx-portable` | [Portable receipts](portable-receipts.md) |
| Fiat rails | `services/interop/crates/layerx-fiat` | [Ramps and fiat](ramps.md) |
| Market-maker ramps | `services/platform/ramps` | [Ramps and fiat](ramps.md) |
| Ethereum / Solana mirrors | `services/interop/crates/layerx-mirror` | [Mirrors](mirrors.md) |
| ETH / Solana migration | `services/interop/crates/layerx-migrate` | See `services/interop/crates/layerx-migrate/OPERATIONS.md` |

`services/interop/crates/layerx-interop-service` mounts the adapter HTTP
handlers. Its route contract is
`services/interop/crates/layerx-interop-gateway/src/server.rs`. A2A and MCP
are payment transports in that contract; there is no standalone `layerx-a2a`
crate. The CLI installer lives in `tools/platform/cli`.

## Connecting to the network

Use the public node JSON-RPC at the root of the selected node's HTTPS origin.
Confirm `eth_chainId` returns `0x7d`.
See [Public RPC endpoints](../reference/public-rpc.md) for node selection.
The interop service's HTTP routes, such as the [AP2](ap2.md) routes, belong
to a separately configured edge application; they are not node JSON-RPC
paths. Sub-chains are tenant domains in the database and require no new
chain infrastructure.

## Honesty rule

Report only what verified evidence supports: mandate verification,
submitted transaction, signed receipt, committed network outcome or
external settlement. A [portable receipt](portable-receipts.md) proves
receipt authenticity at the receipted level. A [mirror](mirrors.md)
archives committed block data with source provenance and freshness.
Neither establishes an external provider's payout or prevents a chargeback.

External custody and payout remain the [ramp operator's](ramps.md)
responsibility. Keep pending, unknown, refusal and reversal states visible;
an unavailable dependency cannot be translated into success.

[Receipt concepts](../concepts/receipts.md)
