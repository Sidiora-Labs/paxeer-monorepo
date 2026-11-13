<!-- Source: https://docs.paxeer.app/funding/ -->

# Funding and money movement

Quote and commit money movements, fund managed agents, and inspect receipt evidence on Paxeer.

## Movement journeys

The Human API represents money movement as a `move` journey on one network. The resolver vocabulary is `fund`, `allocate`, `return`, and `transfer`. Each journey reports its own stages and evidence.

| Movement | Meaning | Authorization and evidence |
| --- | --- | --- |
| Fund | Fund a managed budget | Resolved route and verified receipts |
| Allocate | Move funds from a budget to an agent | Resolved route and verified receipts |
| Return | Return funds to the owner | Resolved route and verified receipts |
| Transfer | Send funds to a recipient | Quote, commit, execution receipt |

Money-moving mutations require an `Idempotency-Key`. Reuse the key when retrying the same intended action and reconcile the existing journey before requesting another movement.

## Production transfer path

`POST /v1/moves/quote` prepares a movement and `POST /v1/moves` commits that quote. In the production service, a quote builds an EIP-1559 bank transfer using the source owner's bound EVM account and pending nonce. The source must belong to the authenticated owner; the destination must resolve to a different, nonzero recipient.

## Move money and fund an agent

Quote the source, destination, and amount first. The resolver defines `fund`, `allocate`, `return`, and `transfer` according to the account relationship. The production bank quote returns `transfer` and preserves the selected source, recipient, amount, and fees for commitment.

```
POST /v1/moves/quote

{
  "source": "<authenticated-source-account>",
  "destination": "<recipient-account>",
  "money": { "amount": "500000", "currency": "<currency-from-balance>" }
}

POST /v1/moves
Idempotency-Key: <unique-key-for-this-move>

{ "quote_id": "<quote_id-from-response>" }
```

Substitute the authenticated source account, a resolvable recipient account, and the currency returned by the balance read. The production service accepts the owner's canonical source account or its returned account identifier; a destination may be a canonical account or EVM address. Amounts are positive integer strings in the asset's base units. Obtain `quote_id` from the quote response.

| Quote field | What to review |
| --- | --- |
| `money` and `mechanism` | The exact amount, currency, and resolved movement |
| `fee_estimate` and `fee_ceiling` | Expected charge and maximum disclosed charge |
| `arrival_estimate` and `expires_at` | Expected timing and quote validity |
| `irreversibility_copy_key` | Any irreversible-action disclosure |

Commit returns a journey. A refused movement reports the refusing authority, explanatory copy, remaining money, and optional change path. Funding a recipient account uses the quote-and-commit surface. Reclaim uses `POST /v1/agents/{agent_id}/reclaim` with `money` and returns a move with return semantics.

### Spending and fee budgets

Agent monthly limits carry an enforcement label: `protocol` or `app`. Native fee budgets separately constrain per-activity, total, and period fees. Read `GET /v1/sessions/fee-policy` for the configured fee asset, currency, and decimals, and present the principal amount separately from execution fees.

## Verification levels

Human evidence uses `unverified` for submitted, `receipt-verified` for receipted, and `checkpoint-finalised` for committed. Committed means the verified outcome is included in a committed block on this network. The journey's completion label alone is insufficient to establish that evidence.

## Card, bank, and real-time payment rails

The fiat interop adapter connects provider callbacks for card, bank, and real-time payments. Provider evidence distinguishes authorised, clearing, settled, reversed, and chargeback events. Credit and corrective movements require accepted provider evidence and a receipt-gated LayerX result.

| Rail | Provider callback route |
| --- | --- |
| Card | `POST /v1/http/fiat/card/callbacks` |
| Bank | `POST /v1/http/fiat/bank/callbacks` |
| Real-time payment | `POST /v1/http/fiat/rtp/callbacks` |

These are provider integration endpoints, not consumer deposit or card-entry endpoints. Callbacks carry token references only; PAN-shaped input is refused. A provider's authorisation is distinct from settlement, and reversal or chargeback is represented as its own verified event.

### Independent market-maker ramps

Market-maker ramps in `services/platform/ramps/toolkit` compile grant draws and operator sends, verify order receipts, and journal orders. Their quote terms identify the external custody role. Inspect those terms before authorizing a conversion.

## Reconcile the result

Read balances through `GET /v1/account/balance`, journey status through `GET /v1/journeys/{journey_id}`, and proof material through `GET /v1/evidence/{evidence_id}`. Journey states preserve still-checking, refused, and waiting-for-you alongside completion. The API response envelope is `{ ok, result?, error?, trace }`; retain trace and evidence references for support.

See [the Human interface](https://docs.paxeer.app/human-interface) for passkeys, ownership, approvals, and agent controls, and [interop](https://docs.paxeer.app/interop) for external system boundaries.
