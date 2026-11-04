# Human journeys

A journey is the Human service's durable record of an operation: what the user
approved, its ordered stages, its progress and the evidence for the outcome.
All current money movements execute on Paxeer mainnet-beta, EVM chain ID `125`.
The kernel is part of the network and proposes blocks; successful outcomes
become final when their block commits.

The active engine kinds in
`services/human/crates/layerx-human-service/src/journeys/engine.rs` are
`onboarding`, `move`, `agent-create`, `agent-fund`, `agent-pause` and
`agent-retire`. Admission still depends on the operation's configured
providers, authorization and readiness. A kind in the engine is not a promise
that every deployment enables it.

## Money vocabulary

The user chooses who receives how much through **Move money**. The service
derives fund, allocate, return or transfer from the accounts and their
relationship. The user does not choose a transfer mechanism or settlement
domain.

The current bank-transfer path builds calls to the bank precompile at
`0x0000000000000000000000000000000000001001`. The resolver carries the exact
EIP-1559 transaction, chain ID, sender's pending nonce, gas limit and fee bounds.
Managed-agent budget and grant operations require the corresponding native
authority. Ownership is explained in [Identity and authority](../concepts/identity.md).

## Quote, review and commit

`POST /v1/moves/quote` returns the proposed movement, fee estimate, fee ceiling,
arrival estimate and expiry. The review states any irreversibility before the
user approves. Estimates remain estimates until receipt-backed actuals exist.
`POST /v1/moves` commits the quoted operation using its `quote_id` and an
`Idempotency-Key`. An expired quote is refused and needs a fresh review.

The Human API requires authenticated, principal-scoped authorization for these
operations. A wallet or managed signer approves the exact prepared transaction;
the browser cannot authorize a changed destination, amount or nonce by reusing
the old approval. Bank plans retain their per-leg transaction identities and
consecutive pending nonces, so recovery refers to the original execution.

## Done means committed success

The receipt lifecycle is `Submitted`, `Receipted`, then `Committed`.
`Submitted` records acceptance; `Receipted` supplies execution evidence;
`Committed` establishes inclusion in a committed block. A signed execution
receipt by itself is insufficient for completion. A committed refusal or
reverted transaction is a failure, not a successful money movement.

For a native journey, the service verifies the outcome against its action,
transaction, nonce and expected operation. Every required leg must have
successful committed evidence before the journey completes. There is no
additional settlement chain to wait for. Some retained API labels, including
`done-finalised` and `checkpoint-finalised`, describe that committed evidence;
they do not introduce another finality system.

When an outcome is unknown, the journey remains `still-checking` and controls
that could duplicate its effect stay locked. Recovery looks up the existing
transaction and receipts under the original journey/action identity. A timeout
does not authorize a fresh economic submission or a replacement idempotency
key. Durable state allows the same journey to resume after a service restart.

## Stating an intent once

`POST /v1/intents/plan` plans a movement without moving money. It returns
ordered legs, total fees, a `plan_digest` and signing requirements. The service
reads balances, wallet binding, allowances and fees; callers cannot supply
headroom as authority. Missing observations produce a typed refusal.

The digest binds the intent and every leg. `POST /v1/intents/submit` re-plans
against current observations and refuses a digest mismatch before creating a
journey. Submissions use `Idempotency-Key`; repeating the same request returns
the original journey. The retained `account_sequence` field carries the
sender's EVM nonce read with the pending tag. It is not a caller-chosen
protocol counter.

Some schema names retain pre-merge domain vocabulary for compatibility. Current
bank execution is one ordered transaction plan on chain `125`; those names do
not describe separate execution and custody networks. The implementations are
`src/journeys/resolver.rs`, `move_money.rs` and `intent_submit.rs` in the Human
service crate.

## Reading progress and evidence

The retained Human API supports:

| Operation | Purpose |
| --- | --- |
| `GET /v1/journeys` | List the principal's journeys |
| `GET /v1/journeys/page/{cursor}` | Continue the list |
| `GET /v1/journeys/{journey_id}` | Read stages, state, refusal and evidence references |
| `GET /v1/evidence/{evidence_id}` | Retrieve canonical material for export and verification |

An approval hold stays `waiting-for-you` until the authorized decision or its
expiry. A refusal names the limiting policy, budget, capability or protocol
check and states whether money left the account. Activity, notifications and
support should use this same evidence rather than inventing a success state.

## Historical journeys

Standalone `wallet-binding`, `deposit`, `withdraw` and `exit` kinds are retired.
Their persisted records remain readable as closed history; the engine does
not resume them or admit new work through their old initiation operations.
Old custody-boundary and cross-domain instructions are historical only.
Wallet setup is described in [Identity and authority](../concepts/identity.md).

Taking money back from a managed agent requires a permitted budget return,
agent-authorized transfer or explicit payer grant. Holding managed signing
keys does not permit an arbitrary sweep of an agent's funds.

## Related pages

- [Web application](web-app.md)
- [Owner registration](owner-registration.md)
- [Hosted Human](hosted.md)
- [Public node and hosted gateway JSON-RPC](../platform/gateway-rpc.md)
- [Approvals in the agent contract](../agents/agentd.md)
