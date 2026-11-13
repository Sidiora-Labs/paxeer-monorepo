# Ramps and fiat rails

Third-party on-ramps and off-ramps are independent market makers operating
ordinary accounts on Paxeer X Network. They receive no special protocol
authority. The network's native ledger and committed receipts establish the
on-network payment outcome; provider evidence establishes the external
funding or payout outcome.

Every customer surface must display:

> External custody: this independent market maker controls the off-platform funds and payout.

The market maker controls external funds; the customer's network payment
uses an ordinary account on the same network, with EVM chain ID `125`.

## Market-maker toolkit

`services/platform/ramps/toolkit` provides `layerx-ramp-toolkit` and
`ramp-verify-receipt`; `services/platform/ramps/reference` provides
`layerx-reference-ramp`. Both belong to the `services/platform` Cargo
workspace.

`RampOrder::bind` combines an authenticated customer, an operator-owned quote
and operator configuration. Its digest binds the accounts, direction,
asset, exact amounts, rate, fees, expiry, provider tokens and payment
authorisation. Customer JSON cannot select operator credentials, signers,
compliance decisions or an arbitrary customer account.

The maintained payment paths compile signed EVM transactions:

| Direction | Network leg |
| --- | --- |
| On-ramp | Operator-funded `payExact` to the native ledger at `0x000000000000000000000000000000000000101a`; the reference binds the order digest. |
| Off-ramp | Customer-authorised `budgetSpend(bytes32,bytes32,uint128)` at `0x000000000000000000000000000000000000101b`, paying the configured operator from the customer's budget authority. |

`compile_operator_send` and `compile_customer_payment` implement these
paths in `toolkit/src/lib.rs`. The compatibility name
`compile_payer_grant_draw` delegates to the customer budget path. Historical
activity-envelope send/receive and ledger-draw instructions are retired.

The client submits exact signed bytes through `eth_sendRawTransaction`,
retains their hash and resolves them through `pax_sequencerReceipt`.
It authenticates the receipt under pinned key windows, execution metadata,
transaction and payment logs against the bound order. Committed answers
also require matching block inclusion. A signed receipt at `receipted`
remains pending; the engine requires authenticated `committed` evidence
before advancing to `LayerxVerified`, marking the order done or permitting
the off-ramp payout.

## Provider settlement and status

The public order vocabulary is `pending`, `unknown`, `refused`,
`manual_review`, `reversed` and `done`. An on-ramp requires verified source
settlement before the operator pays on-network. An off-ramp requires the
committed network payment and settled external payout before `done`.
A later provider reversal removes `done` immediately.

`services/interop/crates/layerx-fiat` supplies adapter ID `fiat` and rails
`Card`, `Bank` and `RealTimePayment`. Card data terminates at the certified
provider; the adapter accepts opaque token references. Its evidence classes
`Authorised`, `Clearing`, `Settled`, `Reversed` and `Chargeback` remain
distinct. A card authorisation or clearing notice does not establish settled
funds, and a network receipt cannot prove the external payout.

## Crash recovery

The append-only journal persists planned submissions, exact signed
transactions, transaction hashes, receipt observations, provider identities
and callback sequences. Restart replays that state. Unknown outcomes retain
their original identity; recovery reconciles them before deciding whether
the exact transaction may be redelivered. It never creates a replacement
payment merely because a response was lost.

Callback handling is implemented in `toolkit/src/engine.rs` and
`toolkit/src/journal.rs`. The engine verifies the signed callback against
the order. `apply_provider_callback` rejects conflicting identities or
sequences and treats identical redelivery as a no-op. `Journal::append`
validates the complete projected transition before its durable write, then
applies the staged order and callback-index changes.

## Reference service and deployment

The configured TLS listener mounts `POST /v1/orders`,
`GET /v1/orders/{order_digest}`, `POST /v1/provider-callbacks`, `/livez`
and `/readyz`. Operator worker routes include `POST /internal/v1/work`,
`POST /internal/v1/rebalances` and
`GET /internal/v1/rebalances/{idempotency_key}`. These belong to the
reference ramp deployment, not the node JSON-RPC. Customer identity comes
from token introspection; inventory rebalance configuration is operator-owned.

Deployments supply actual provider, compliance and signer coordinates and
qualify them against the corresponding services. See
`services/platform/ramps/config.example.json` and
`services/platform/ramps/OPERATIONS.md`. Repository code does not establish
that a provider sandbox or production payout service is available.

[Receipts](../concepts/receipts.md) · [Portable verification](portable-receipts.md) · [Interoperability](index.md)
