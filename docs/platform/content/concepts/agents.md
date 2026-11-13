# Agents and budgets

An agent is software authorized to prepare and submit payments within granted authority. Protocol budgets and daemon policy impose distinct limits.

## Budget ceilings

A budget is a protocol object in the budget module. You create it, you fund it, and spending past what is funded is refused by the transition function. For spending through that budget, the limit is enforced by the protocol independently of `layerx-agentd`. It does not limit separate authority the agent may hold over other funds.

Anything narrower than the funded budget - a per-call limit, a counterparty allow-list, a rate - is enforced by the agent layer or by your own code. Those controls bind callers that use the enforcing agent service.

## Capabilities

A capability is what `layerx-agentd` will let a holder do. You create one, attenuate it into something narrower, and revoke it when you are done.

| Operation | Effect |
|---|---|
| `capability.create` | Mints a capability for an agent |
| `capability.attenuate` | Produces a strictly narrower capability |
| `capability.revoke` | Ends it |
| `capability.list` | Shows what is outstanding |

Attenuation only ever narrows. There is no widening operation, so a capability handed down a chain cannot regain authority on the way.

## Approval holds

Some activity should not happen without a person. The approval module lets the agent layer hold an activity and surface it for a decision, carrying the held activity's structured disclosure, the digest of its canonical bytes, the reason it was held, and a deterministic expiry.

| Operation | Effect |
|---|---|
| `approval.list` | Pending holds |
| `approval.get` | One hold with its disclosure |
| `approval.approve` | Releases exactly the held activity |
| `approval.reject` | Ends it |

The digest matters: approving a hold approves the exact bytes that were disclosed, not a re-derived intent that might differ.

## The write path

Agent writes are always prepare, sign, submit, then track or wait. Preparing supplies the transaction facts, signing hash and disclosure. The signer produces a signed EIP-1559 transaction; submission checks those bytes against the preparation and hands them to the network. Tracking resolves the outcome. If a submission cannot be classified, the answer is `Unknown` - see [Retries and unknown outcomes](idempotency.md).

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Protocol budget ceilings | `protocol` | A spend through the budget cannot exceed its funded authority. |
| Programs never write balances | `protocol` | An agent calling a program cannot use it to exceed its own authority. |
| Capability attenuation | `agent-layer` | Binds callers that go through `layerx-agentd`. A principal reaching the protocol another way is bound by protocol budgets, not by this. |
| Approval holds | `agent-layer` | The hold exists while the daemon is in the request path. |
| Agent tenancy isolation | `agent-layer` | Sessions are scoped to an agent's tenancy. This is an agent-layer boundary, not a protocol one. |
| Unknown is a real outcome | `agent-layer` | An unclassifiable submission is reported as `Unknown`, never guessed. |
