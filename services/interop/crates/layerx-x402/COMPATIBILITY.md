# x402 compatibility matrix

The adapter declares x402 v2 (`2.0.0`) pinned to revision
`7d5363a6d51750dc246041f2b0ed5819dd46a0d7` in [src/lib.rs](src/lib.rs).
Buyer, seller and facilitator messages are carried across HTTP, MCP and A2A.
HTTP uses `PAYMENT-REQUIRED`, `PAYMENT-SIGNATURE` and `PAYMENT-RESPONSE` headers
for payment signals; facilitator messages use JSON bodies on all transports.

| Transport | Buyer | Seller | Facilitator | Local matrix source |
| --- | --- | --- | --- | --- |
| HTTP | yes | yes | yes | [tests/transports.rs](tests/transports.rs) |
| MCP | yes | yes | yes | [tests/transports.rs](tests/transports.rs) |
| A2A | yes | yes | yes | [tests/transports.rs](tests/transports.rs) |

Here, “yes” means the transport binding and role-message codec exist. The local
matrix exercises encode/decode parity, validation and settlement identity. It
does not establish upstream reference-implementation conformance or successful
settlement with an independently operated facilitator.

The HTTP row covers header and JSON-body codecs; the MCP and A2A rows cover
JSON message bindings. These local cases do not start HTTP, MCP or A2A services
or qualify their deployed transport sessions. The role documents are committed
first-party fixtures under `services/interop/specs/conformance/transport-*`.

Paxeer mainnet-beta is one BFT network, EVM chain ID 125 (`eip155:125` in x402).
The kernel is part of the network and proposes blocks; block commit establishes
finality. Receipt signing and block commit are stages of that network. See the
[portable verification guide](../../../../docs/interop/PORTABILITY.md) and
[receipt format](../layerx-portable/FORMAT.md) for the limits of signature proofs.

## Facilitator settlement

The facilitator carries `/verify`, `/settle` and `/supported` messages as JSON
bodies. `/verify` validates a request without executing a payment. `/settle`
reports success after the adapter and gateway verify the signed receipt and
match authenticated payment logs to the expected terms. Pending responses have
`success: false`, `errorReason: "settlement_pending"` and verification level
`submitted`. Success preserves the supplied evidence level, `receipted` or
`committed`. For committed evidence, `Seller::settled` also checks the commit
proof against the supplied validator set and chain ID. See
[the seller](src/seller.rs) and [facilitator](src/facilitator.rs).

The payment authority is the Paxeer ledger precompile at
`0x000000000000000000000000000000000000101a` on chain 125. Gateway translation
identity is transport independent and separates settlement steps. The
facilitator calls its payment plane on retries, including duplicates; the
payment authority must preserve the economic idempotency key across restart.
A translation record or receipt signature alone is not a balance observation.

## Development coverage

[tests/transports.rs](tests/transports.rs) covers role-message round trips,
settlement identity, pending and error retries, duplicate delivery and receipt
matching. It uses local signed receipts and a retained in-memory `GatewayCore`
for fault-injected settlement cases. Those cases exercise local retry behavior;
they do not establish process restart recovery or independently observed funded
settlement. [tests/vectors.rs](tests/vectors.rs) covers message-model vectors.

The [Interop x402 transport matrix workflow](../../../../.github/workflows/interop-x402.yml)
runs `make interop-test-x402` on matching pull requests and main-branch changes,
and supports manual dispatch. See the [workspace index](../../README.md) for
the wider interop boundary.
