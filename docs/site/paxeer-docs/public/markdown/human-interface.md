<!-- Source: https://docs.paxeer.app/human-interface/ -->

# Human interface

Manage your Paxeer X identity, balances, agents, approvals, and recovery through evidence-backed journeys.

## A human control plane for Paxeer X

The Human interface brings the LayerX execution system into an account experience: sign in with a passkey, add funds, create agents, authorize sensitive work, and follow every movement through receipts. Home combines your balance, managed agents, pending approvals, and recent activity.

Human is custodial by default. Account authentication and protocol authority have separate roles. This separation lets you manage agents without exposing protocol signing keys to the browser. On the merged network, money moves through `move` journeys; the former wallet-binding, deposit, withdraw, and exit journey kinds are retired and remain readable only as closed history.

| Identity | Purpose | Authority |
| --- | --- | --- |
| Application identity | Passkey authentication and device sessions | Opens the Human account experience |
| Protocol identity | LayerX DID and account operations | Custody service holds the primary signing key |

## Onboard and secure an account

1. Create an account with an email and display name.
2. Register a passkey using the browser credential ceremony.
3. Follow the protocol identity and recovery stages.
4. Open the account when its protocol identity has a verified LayerX receipt.

Onboarding progresses through creating the account, adding the passkey, setting up the protocol identity, and putting recovery in place. Resume continues the existing journey. A local progress indicator alone does not activate an account.

| Action | Human API |
| --- | --- |
| Create account | `POST /v1/accounts` |
| Register passkey | `POST /v1/passkeys/registrations`, then `POST /v1/passkeys/registrations/{registration_id}` |
| Read or resume onboarding | `GET /v1/onboarding`, `POST /v1/onboarding/resume` |
| Sign in | `POST /v1/passkeys/assertions`, finish the assertion, then `POST /v1/sessions` |
| Inspect or refresh sessions | `GET /v1/sessions`, `POST /v1/sessions/refresh` |

### Wallet binding is retired

The former wallet-binding flow (`POST /v1/wallet-binding/statement`, `POST` and `GET /v1/wallet-binding`, and rebind) is retired. Every binding type and operation is marked retired, and stored wallet-binding journeys read back only as closed history; they cannot resume signing or execution. There is no wallet to connect and no binding state to poll. An owner EVM address is bound to the DID during registration instead; see [identity](https://docs.paxeer.app/identity).

## Create and govern managed agents

An agent has a name, purpose, and monthly spending limit. Creation establishes the agent, its protection, and first funding. A separate native fee budget sets maximum fees per activity, total fees, period length, and maximum fees per period; a spending allowance and a fee allowance serve different purposes. On networks enforcing native fee authority version 2, the finite `native_fee_budget` must be supplied at creation: omitting it refuses creation, and changing or rotating a key never silently replenishes it.

```
POST /v1/agents
Idempotency-Key: 9f3c1e6a0b8d4f72a5c9e1d3b7a2c4e6

{
  "name": "Research Runner",
  "purpose": "Buys datasets and pays per-use research services",
  "monthly_limit": { "amount": "250000000", "currency": "LXP" },
  "native_fee_budget": { "...": "finite fee budget, required on fee-authority-v2 networks" }
}
```

The example follows the Human API wire format. Amounts are integer strings; present human-readable values using the asset's declared precision. Creation returns an `agent-create` journey.

| Control | Operation | Result |
| --- | --- | --- |
| Browse agents | `GET /v1/agents`; `GET /v1/agents/{agent_id}` | Agent details and status |
| Pause or resume | `POST /v1/agents/{agent_id}/pause` or `/resume` | Updated Agent |
| Change monthly limit | `POST /v1/agents/{agent_id}/limit` | Updated Agent |
| Fund | Move quote and commit | Receipt-backed movement journey |
| Reclaim funds | `POST /v1/agents/{agent_id}/reclaim` | Move journey using return semantics |
| Rotate or recover keys | `POST /v1/agents/{agent_id}/rotate` or `/recover` | Key challenge |
| Archive | `POST /v1/agents/{agent_id}/archive` with `confirm_name` | Agent retirement journey |

Owner key rotation adds a disclosure and step-up before starting the rotation window. Retirement stops work and closes the agent. Pause returns the current Agent directly; it does not create a separate journey response.

## Review approvals before money moves

Held activities appear as `waiting-for-you`. Open the approval to inspect the amount, counterparty, asset, fees, expiry, remaining budget after approval, and supporting evidence. A step-up ceremony confirms the disclosed action before approval. Rejecting resolves the request without approving its movement.

1. List pending requests with `GET /v1/approvals`.
2. Read `GET /v1/approvals/{approval_id}` and review its facts.
3. Begin step-up with `POST /v1/step-up` and finish it with `POST /v1/step-up/{challenge_id}` using the supplied confirmation digest.
4. Submit `step_up_evidence` to `POST /v1/approvals/{approval_id}/approve`, or use `/reject`.
5. Read the decision's `money_moved` and evidence instead of inferring success from the button press. At decision time `money_moved` is false; movement is established by the resulting journey and receipt.

Approval states are pending, approved, rejected, expired, and defective. Expiry and defective evidence prevent a request from being treated as an approved payment.

## Read progress and proof

Journeys expose stages, evidence references, timestamps, and refusals. States are getting-ready, sending, processing, done, done-finalised, still-checking, refused, waiting-for-you, closed, and awaiting-approval. Closed is how retired wallet-binding, deposit, withdraw, and exit history reads back; awaiting-approval marks a journey held for approval and is not receipt-backed completion. A refusal identifies who refused, explanatory copy, money left, and an optional path to change the request.

| Verification | Meaning |
| --- | --- |
| `unverified` | Submitted; no stronger authenticated execution evidence is attached |
| `receipt-verified` | Receipted; the LayerX execution receipt is verified |
| `checkpoint-finalised` | Committed; inclusion in a committed block is verified |

The former `paxeer-finalised` label is retired and defines no further rung. Use `GET /v1/journeys`, `GET /v1/journeys/{journey_id}`, and `GET /v1/evidence/{evidence_id}`. Evidence material includes its class, verification, content type, and base64 bytes. Live updates use `POST /v1/stream` and `GET /v1/stream/{cursor}`. These Human verification labels are distinct from RPC commitment names.

## Recovery, sessions, and support

Security settings manage passkeys, individual or all sessions, authenticator enrollment, backup-code rotation, and recovery evidence. Sensitive actions use a disclosure digest and step-up authorization. Revealed recovery evidence is a timed secret with a remask timestamp and copy policy. Protocol keys remain in custody; the single exception is the `export-primary-key` security action, the only route by which a DID primary key leaves the service.

Activity search uses `POST /v1/activity/query`; detail uses `GET /v1/activity/{entry_id}`. Support conversations accept deposit, withdrawal, agents, account, and report topics through `POST /v1/support/conversations`.

Continue with [funding, money movement, and withdrawals](https://docs.paxeer.app/funding) or explore [agent capabilities](https://docs.paxeer.app/agents).
