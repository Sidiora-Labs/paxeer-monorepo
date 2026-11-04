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
[portable verification guide](../../PORTABILITY.md) and
[receipt format](../layerx-portable/FORMAT.md) for the limits of signature proofs.

## Facilitator conformance results

The facilitator carries `/verify`, `/settle` and `/supported` messages as JSON
bodies. `/verify` is a read-only translation and does not execute a payment.
`/settle` reports success only after the adapter and gateway verify the signed
receipt and match authenticated payment logs to the expected terms. Pending
responses have `success: false`, `errorReason: "settlement_pending"` and LayerX
verification level `submitted`. A successful response preserves the evidence
level, `receipted` or `committed`; signature validity alone does not prove commit.
When committed evidence is supplied, `Seller::settled` additionally requires a
matching commit proof checked against the supplied validator set and chain ID.
The [seller](src/seller.rs) and [facilitator](src/facilitator.rs) implement these
checks.

The following table describes assertions in the local suites, not a report of a
new test run or live economic settlement:

| Facilitator behaviour under test | Evidence in `tests/transports.rs` unless stated otherwise |
| --- | --- |
| Role messages round-trip on HTTP, MCP and A2A | `every_role_round_trips_on_every_transport` |
| Success records a receipt-verified gateway translation | `confirmed_settlement_records_exactly_one_receipt_verified_effect` |
| Identity excludes transport and distinguishes settlement steps | `settlement_identity_is_transport_independent_and_step_separated` |
| Cross-transport delivery reuses the local translation identity | `transport_independent_identity_settles_once_across_http_mcp_and_a2a` |
| Repeated delivery preserves the recorded receipt digest | `duplicate_delivery_of_the_same_settlement_does_not_double_charge` |
| Pending/error retries eventually confirm in the same in-memory gateway | `settlement_recovers_after_a_crash_without_a_second_economic_effect` |
| A different receipt for an already settled identity is refused | `a_swapped_receipt_for_a_settled_identity_is_refused` |
| Payment-required, payload and settlement wire vectors | [tests/vectors.rs](tests/vectors.rs) |

The fault-injection cases construct `FaultInjectingPlane` and local signed test
receipts and retain the same `GatewayCore` across retries. In particular, the
case named `settlement_recovers_after_a_crash_without_a_second_economic_effect`
returns a scripted transient error; it does not terminate a process, reopen
persistent state or observe a funded ledger across restart. Its duplicate
assertions compare gateway translation status and receipt digest, not an
independently measured chain balance. The facilitator calls the plane again on
retries, including duplicates, so durable economic deduplication also depends
on the deployed payment authority.

These suites establish the bounded local behaviour above. They do not prove
durable crash recovery or exactly-once economic effect across a production
restart. Such a claim needs evidence through the real producer, transport,
persisted state and payment authority, with restart/recovery and independently
observed balances. Upstream vector conformance and third-party interoperability
likewise require their own recorded qualification evidence.

## Pending qualification

| Claim | Status | Evidence required before promotion |
| --- | --- | --- |
| Durable recovery across service restart | pending | Stop the real service, reopen its persisted state, retry the same payment through its transport and verify the recovered result. |
| Exactly-once economic effect | pending | Observe authoritative native ledger balances and payment records before submission and after duplicate delivery and service restart. |
| Upstream reference conformance | pending | Run the pinned upstream reference suite and record its revision and results. |
| Independently operated facilitator interoperability | pending | Record a real buyer, seller and independent facilitator exchange. |

Promote durable recovery and economic-effect claims only with real service
restart and native ledger evidence from the same payment path. The native
payment authority is the Paxeer ledger at `0x101a` on chain 125; a local gateway
translation, receipt digest or valid signature alone is not a balance
observation. Until that evidence is recorded, these claims remain pending even
when the local transport matrix passes.

The [Interop x402 transport matrix workflow](../../../../.github/workflows/interop-x402.yml)
runs `make interop-test-x402` on matching pull requests and main-branch changes,
and supports manual dispatch. See the [workspace index](../../README.md) for
the wider interop boundary.
