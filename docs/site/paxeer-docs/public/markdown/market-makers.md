<!-- Source: https://docs.paxeer.app/market-makers/ -->

# Market Makers

Provide exchange liquidity and operate independent asset ramps with bound quotes, authorized payments, durable reconciliation, and receipt-gated settlement.

## Two liquidity workflows

Paxeer X supports market makers as ordinary LayerX principals. On exchange books, a maker posts resting bids and asks and manages the inventory committed to those orders. An independent ramp operator quotes conversions between LayerX assets and an external payment or payout system. Exchange orders and ramp orders have distinct execution and custody models.

| Workflow | Inventory | Execution evidence |
| --- | --- | --- |
| Spot quoting | Base or quote units escrowed for resting orders | Matching result and asset settlement receipt |
| On-ramp | Operator LayerX assets and external funds | External settlement followed by verified LayerX delivery |
| Off-ramp | Customer-authorized LayerX payment and operator external payout inventory | Verified LayerX payment followed by external payout settlement |

## Quote exchange liquidity

Create separate order identifiers for each quote, size bids against quote-asset inventory and asks against base-asset inventory, and submit limit GTC orders to the [spot book](https://docs.paxeer.app/spot-exchange). Matching uses best price and time priority; fills execute at the resting order's price. Update exposure from verified fills and residual order state.

Before repricing, cancel the old order and reconcile the result. Maintain separate records for free inventory, inventory reserved by resting quotes, and completed trades. Market halts stop new placement; an agent's quoting policy should inspect market state and stop replacement submissions while a market is halted.

## Independent ramp custody

The ramp toolkit binds a customer, operator, quote, and authorization into one order. The operator controls off-platform funds and payout. Show the toolkit's custody disclosure wherever the customer reviews or tracks a ramp:

External custody: this independent market maker controls the off-platform funds and payout.

The LayerX receipt proves the LayerX payment leg. External payment settlement comes from the provider's evidence and remains a separate obligation. A complete ramp order reconciles both.

## Bind the quote

| Terms | Purpose |
| --- | --- |
| `quote_id`, direction, expiry | Identify the offer, on-ramp or off-ramp flow, and validity window |
| LayerX asset and amount | Bind the exact on-platform inventory movement |
| External currency and minor-unit amount | Bind the external payment amount without floating-point ambiguity |
| Rate numerator and denominator, fee, maximum slippage | Make the conversion terms explicit |
| Context, provider token, payout token | Bind the intended payment context and provider operation |

`RampOrder::bind` validates the quote and customer/operator identities, matches the requested quote identifier, and computes an order digest. Customers and operators use the `AgentMain` account namespace. On-ramp orders use operator authorization; off-ramp orders require a payer grant. The digest commits to the quote, principals, operator signing-key handle, context, and authorization reference.

```
RampOrder::bind(request, quote, customer, operator, now)
  -> order_digest

On-ramp:  external provider settles -> operator sends LayerX asset -> verify receipt
Off-ramp: customer payer grant -> verify LayerX payment -> external provider settles
```

This is the toolkit workflow, not an HTTP endpoint. Integrate through the toolkit's typed clients and your provider's actual contract.

## Execute an on-ramp

1. Authenticate the customer and obtain a valid quote with exact LayerX and external amounts.
2. Bind and journal the order, then complete the compliance decision and external collection workflow.
3. Reconcile external provider settlement before preparing the LayerX delivery.
4. Use `compile_operator_send(order)`, which takes only the bound order and returns a `LedgerCall` to `LEDGER_ADDRESS` carrying `encode_pay_exact(..., order_digest)`. It refuses orders that are not operator-funded on-ramps without a payer grant.
5. Retain the canonical activity before submission. Resolve pending or unknown submissions by activity identifier.
6. Verify the receipt against the authorized batch and bound order; mark the workflow done after verified delivery.

## Execute an off-ramp

1. Bind the customer's quote and payer-grant identifier into the order.
2. Use `compile_customer_payment(order)` to construct the authorized customer-to-operator payment: a budget spend to `PAY_ADDRESS`. `compile_payer_grant_draw(order, _receiver_sequence)` is a compatibility shim that delegates to it and never uses the retired ledger draw selector.
3. Verify the LayerX payment receipt before initiating the provider payout.
4. Reconcile the provider operation or authenticated callback and retain its evidence digest.
5. Complete the order after the external payout settles. Surface refusal, reversal, or manual review as distinct outcomes.

## Verify payment evidence

`verify_order_receipt` checks canonical receipt evidence against the authorized batch and the expected asset-module operation. It binds the activity identifier, sender, receiver, asset, amount, and context to the order. On success it returns a `VerifiedLayerxLeg` with `tx_hash`, `receipt_digest`, `height`, and `rung`, derived from the sequencer signature and log binding.

A provider callback is verified against the bound order and provider public key before it changes the journal. Callback identifiers and provider sequence numbers support replay handling. Keep the order digest, activity identifier, receipt digest, and provider evidence digest together in the customer's operational record.

## Durability and recovery

The journal uses the domain `LXP/market-maker-ramp/journal/v2`; `LXP/market-maker-ramp/journal/v1` is retained only as the legacy domain for migration. Worker leases coordinate order progression. The engine records the canonical planned activity before broadcasting, so recovery can resolve its result without inventing a fresh payment. Unknown submission status remains unknown until reconciled.

| Customer status | Operational meaning |
| --- | --- |
| Pending | A payment or payout is progressing |
| Unknown | Submission outcome requires lookup and reconciliation |
| Refused | A leg refuses; retain the refusal code |
| Manual review | The provider requires an operator decision |
| Reversed | The provider reports a reversal |
| Done | The direction-specific settlement gates are satisfied |

## Rebalance Paxeer inventory

`InventoryRebalancer` journals custody transfers under an idempotency key and reconciles the retained operation identifier and transaction hash. Reusing a key with different asset or amount terms returns a conflict. A finality tracker polls the Paxeer transaction and records block identity and confirmations; a broadcast hash alone does not release inventory for a completed strategy cycle.

## Integration map

The toolkit lives at `services/platform/ramps/toolkit` and exposes `clients`, `engine`, `evm`, `journal`, and `migration` modules. Use `QuoteTerms` and `RampOrder` for order binding, the compilation helpers for authorized payment construction, `verify_order_receipt` for payment verification, and `RampPresentation` for customer-facing status.

Continue with [Spot Exchange](https://docs.paxeer.app/spot-exchange), [EVM Precompiles](https://docs.paxeer.app/precompiles), and [LayerX Modules](https://docs.paxeer.app/modules).

**Protocol references:** `services/platform/ramps/toolkit/src/lib.rs`, `services/platform/ramps/toolkit/src/engine.rs`, `services/platform/ramps/toolkit/src/journal.rs`, and `include/layerx/lx_spot.h`.
