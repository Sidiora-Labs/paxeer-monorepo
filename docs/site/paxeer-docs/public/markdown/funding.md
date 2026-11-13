<!-- Source: https://docs.paxeer.app/funding/ -->

# Funding and money movement

Move balances, fund agents, and connect external payment rails on the merged network. Former custody-boundary deposit, withdraw, and exit journeys are retired.

## Understand the boundary

Current money movement on the merged network is the internal `move` journey: fund, allocate, return, or transfer between LayerX accounts, including the EVM intent path. The former custody-boundary journeys — deposit, withdraw, exit, and wallet-binding — are retired kinds. They are retained only for decoding stored history and cannot start new journeys. Retained history of those kinds reads as `closed`, cannot resume signing or execution, and does not assert a successful movement.

| Journey | Status | Authorization and evidence |
| --- | --- | --- |
| Move | Current | Quote, commit, execution receipt |
| Deposit | Retired — history only | Formerly wallet transaction and custody proof |
| Withdraw | Retired — history only | Formerly withdrawal authorization and claim signature |
| Exit | Retired — history only | Formerly eligibility, explicit confirmation, wallet claim |

The `settlement_domain` field is likewise retired; internal moves do not select a settlement domain. Money-moving mutations require an `Idempotency-Key`. Retain the key when retrying the same intended operation, and reconcile its journey before starting another movement.

## Retired custody-boundary journeys

`POST /v1/deposits` (and its `/confirm` step), `POST /v1/withdrawals` (and its `/claim` step), `POST /v1/exit`, and `GET /v1/exit/eligibility` remain in the Human API schema as retired operations. Do not build new deposit, withdrawal, or exit flows on them. The on-chain custody path they relied on is also retired: the custody keeper refuses deposits and the custody precompile reverts. New integrations fund and move balances with the move flow below, on accounts that already hold spendable balances on the merged network.

## Move money and fund an agent

Quote the source, destination, and amount first. The resolver selects `fund`, `allocate`, `return`, or `transfer` from the account relationship. Users choose what they intend to move; they do not choose a lower-level mechanism to bypass those relationships.

```
POST /v1/moves/quote

{
  "source": "acct_01j2gxq0aab1c2d3e4f5g6h7j8",
  "destination": "acct_01j2gxq0aak9m0n1p2q3r4s5t6",
  "money": { "amount": "500000", "currency": "LXP" }
}

POST /v1/moves
Idempotency-Key: <unique-key-for-this-move>

{ "quote_id": "<quote_id-from-response>" }
```

These request fields follow the Human API golden wire example. Amount is an integer string in the asset's precision, not a floating-point number. Obtain the quote ID from the response; the placeholder is not a real quote.

| Quote field | What to review |
| --- | --- |
| `money` and `mechanism` | The exact amount, currency, and resolved movement |
| `fee_estimate` and `fee_ceiling` | Expected charge and maximum disclosed charge |
| `arrival_estimate` and `expires_at` | Expected timing and quote validity |
| `irreversibility_copy_key` | Any irreversible-action disclosure |

Commit returns a journey. A refused movement reports the refusing authority, explanatory copy, remaining money, and optional change path. Agent funding uses this same quote-and-commit surface. Reclaim uses `POST /v1/agents/{agent_id}/reclaim` with `money` and returns a move with return semantics.

### Spending and fee budgets

Agent monthly limits govern spending. Native fee budgets independently constrain per-activity fees, aggregate fees, and period fees. Read `GET /v1/sessions/fee-policy` for the native fee asset, currency, and decimals. Present the principal movement and the execution fee separately so an agent's funding amount is not confused with its ability to pay fees.

## Verification levels

Human verification advances through three current rungs as evidence arrives: submitted (`unverified`), receipted (`receipt-verified`), and committed (`checkpoint-finalised`). `checkpoint-finalised` is the retained label for committed and is the current top rung. `paxeer-finalised` and `settlement-anchored` are retired decoding values; they do not define additional current rungs, so do not wait for them or present them as a stronger finality state.

## Card, bank, and real-time payment rails

The fiat interop adapter connects provider callbacks for card, bank, and real-time payments. Provider evidence distinguishes authorised, clearing, settled, reversed, and chargeback events. Credit and corrective movements require accepted provider evidence and a receipt-gated LayerX result.

| Rail | Provider callback route |
| --- | --- |
| Card | `POST /v1/http/fiat/card/callbacks` |
| Bank | `POST /v1/http/fiat/bank/callbacks` |
| Real-time payment | `POST /v1/http/fiat/rtp/callbacks` |

These are provider integration endpoints, not consumer deposit or card-entry endpoints. Callbacks carry token references only; PAN-shaped input is refused. A provider's authorisation is distinct from settlement, and reversal or chargeback is represented as its own verified event.

### Independent market-maker ramps

Market-maker ramps let ordinary LayerX principals offer external conversion through quote terms and receipt-gated orders. Their toolkit compiles payer grant draws and operator sends, verifies order receipts, and journals orders. External custody is explicitly labelled so users can distinguish a ramp operator's custody from the Human account custody boundary. This ramp role is separate from the fiat callback adapter.

## Reconcile the result

Read balances through `GET /v1/account/balance`, journey status through `GET /v1/journeys/{journey_id}`, and proof material through `GET /v1/evidence/{evidence_id}`. Journey states preserve still-checking, refused, and waiting-for-you alongside completion. The API response envelope is `{ ok, result?, error?, trace }`; retain trace and evidence references for support.

See [the Human interface](https://docs.paxeer.app/human-interface) for passkeys, wallet binding, approvals, and agent controls, and [interop](https://docs.paxeer.app/interop) for external system boundaries.
