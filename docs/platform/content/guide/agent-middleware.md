# Agent middleware

`@sidiora/layerx-agent-middleware` is what an autonomous agent spends through. It wraps the agent plane's prepare/sign/submit/track loop in a budget ledger and refuses to call a payment successful without a verified receipt.

## Spend outcomes

`spend(request)` returns a typed result. Configure `client`, `receiptKeys`, `signer` and `receipts`, with exactly one of `budgets` or `preparationBudgets`. The direct-budget path returns:

| Result | Meaning | Budget state |
|---|---|---|
| `verified` | Receipt verified and matched against amount, asset and recipient | `committed` |
| `approval-hold` | The daemon refused on policy and a matching held approval exists | `held` |
| `pending` | Still settling after the poll budget | `reserved` |
| `unknown` | The outcome is genuinely not known | `reserved` - deliberately not released |
| `refused` | Failed, expired, or refused by the daemon | `released` |
| `budget-refused` | The reservation was refused before anything was prepared | none taken |

## The order of operations

Reserve first. The budget is taken before `prepare` is called, so a spend that never reaches the network cannot exceed the ceiling. `reserve` is keyed on tenant, idempotency key and request digest, so a retry of the same spend re-reserves the same reservation rather than a second one; a same-key different-digest reservation is a `conflict`, which raises `idempotency-conflict`.

Then prepare, sign, submit. The signer only ever sees a `PreparedActivity` - the EVM chain identifier, nonce, fees, destination, calldata, signing hash, sender and disclosure. The middleware never constructs canonical bytes itself and never holds a key.

Then track, with a bounded backoff of up to `maximumTrackPolls` polls (default 20, allowed range 0–1000). The wait function is configurable. Signed EIP-1559 transaction bytes are checked against the prepared facts and bound to their transaction hash. `resume(input, rawTransaction, submissionRef)` tracks that same transaction.

The preparation-budget path returns `kind: "owner-budget"` with a state of `admission-unknown`, `wallet-consent`, `approval`, `pending`, `unknown`, `owner-rejected`, `owner-expired` or `settled`. Its result carries the preparation identifier, whether admission was observed, and available approval, submission and verification material.

## `unknown` is a real answer

If the SDK raises with `retry === "unknown-outcome"`, the middleware returns `unknown` and leaves the reservation standing. It does not release the budget, because releasing it would let a second spend go out while the first may yet settle. It does not commit it either, because nothing has been verified. Resolution belongs to whoever can look the submission up later.

`retry === "safe"` also yields `unknown` rather than an automatic retry: the middleware will not decide on your behalf that a payment is safe to reissue.

## Verification is a match, not a lookup

Receipt verification checks the signed receipt against pinned `receiptKeys` and binds it to the raw transaction. The ledger's `PaymentExact` event must match the requested amount, asset and recipient. A `committed` result additionally requires the configured commitment resolver.

A cryptographically valid receipt for a different payment raises `verification-failure`. Only after the payment matches is the direct budget committed; the commit result is checked for state and request digest.

## Approval holds

A `policy-refusal` or `budget-refusal` on submit triggers a lookup of the tenant's held approvals, matching on the disclosure digest of the prepared activity. A match returns an `ApprovalHold` with `enforcement: "daemon_enforced"` - the hold is real and enforced outside your process; the middleware only surfaces it.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Protocol budget ceilings | `protocol` | The budget module refuses over-ceiling spends at the protocol boundary. |
| Offline receipt verification | `protocol` | Signed receipts are checked against pinned, height-ranged receipt keys. |
| Replay refusal | `protocol` | The same signed transaction cannot settle twice. |
| Capability attenuation | `agent-layer` | Signed transaction facts must match the prepared activity. |
| Approval holds | `agent-layer` | A held approval is enforced by the daemon; the middleware reports it. |
| Honest verification levels | `agent-layer` | `pending` and `unknown` are returned as themselves, never as success or failure. |
| Unknown is a real outcome | `agent-layer` | An unresolved spend keeps its reservation instead of freeing budget. |
| Agent tenancy isolation | `agent-layer` | Reservations and approvals are scoped to the tenant on the request. |
