<!-- Source: https://docs.paxeer.app/human-interface/ -->

# Human interface

Manage your Paxeer identity, balances, agents, approvals, and recovery through receipt-backed journeys.

## An account interface for Paxeer

The Human interface presents the Paxeer network through accounts and journeys: authenticate with a passkey, move funds, manage agents, review approvals, and inspect receipts. Home combines balances, managed agents, pending approvals, and recent activity.

Self-custody is the default posture; custodial holding requires explicit opt-in. Account authentication and protocol authority have separate roles. The key-export ceremony transfers the primary key once and stops subsequent service signing for that identity. Current money movements use `move` journeys on the same network.

| Identity | Purpose | Authority |
| --- | --- | --- |
| Application identity | Passkey authentication and device sessions | Opens the Human account experience |
| Protocol identity | DID and account operations | Self-custody or explicitly selected custodial authority |

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

### Protocol ownership

Onboarding binds protocol identity through the address precompile and verifies the resulting receipt. Passkeys authorize application access; protocol authority controls signed operations. See [identity](https://docs.paxeer.app/identity) for these roles.

## Create and govern managed agents

Agent creation asks for a name, purpose, and monthly spending limit. Each limit identifies whether enforcement is `protocol` or `app`. A separate native fee budget sets maximum fees per activity, total fees, period length, and maximum fees per period. Networks enforcing fee-authority version 2 require a finite `native_fee_budget`; changing or rotating a key does not replenish it.

```
POST /v1/agents
Idempotency-Key: 9f3c1e6a0b8d4f72a5c9e1d3b7a2c4e6

{
  "name": "Research Runner",
  "purpose": "Buys datasets and pays per-use research services",
  "monthly_limit": { "amount": "250000000", "currency": "LXP" },
  "native_fee_budget": {
    "asset_id": "<asset_id-from-fee-policy>",
    "maximum_per_activity": "1000000000000000",
    "maximum_total": "100000000000000000",
    "period_length_ms": "86400000",
    "maximum_per_period": "10000000000000000"
  }
}
```

This example follows `services/human/schema/human-api/agents.kvx`. Substitute `asset_id` from `GET /v1/sessions/fee-policy`. Amounts are integer strings in the declared asset units. The positive per-activity limit cannot exceed the total; a nonzero period requires a period limit at least as large as the per-activity limit. Creation returns an `agent-create` journey.

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

Journeys expose stages, evidence references, timestamps, and refusals. States include getting-ready, sending, processing, done, done-finalised, still-checking, refused, waiting-for-you, closed, and awaiting-approval. A journey held for approval has no completed movement. A refusal identifies its authority, explanatory copy, whether money left, and an optional path to change the request.

| Verification | Meaning |
| --- | --- |
| `unverified` | Submitted; no stronger authenticated execution evidence is attached |
| `receipt-verified` | Receipted; the execution receipt is verified |
| `checkpoint-finalised` | Committed; inclusion in a committed block is verified |

Read `GET /v1/journeys`, `GET /v1/journeys/{journey_id}`, and `GET /v1/evidence/{evidence_id}`. Evidence material includes its class, verification, content type, base64 bytes, and optional progress rung and inclusion height. Verify the actual material and its trusted key bindings. Live updates use `POST /v1/stream` and `GET /v1/stream/{cursor}`.

## Recovery, sessions, and support

Security settings manage passkeys, sessions, authenticators, backup codes, and recovery evidence. Sensitive actions require a server disclosure digest and matching step-up evidence. Recovery evidence has a remask time and copy policy. Primary-key export uses `POST /v1/security/key-export/begin` and `POST /v1/security/key-export/finish`; it returns the key once and leaves the service unable to sign for that identity.

Activity search uses `POST /v1/activity/query`; detail uses `GET /v1/activity/{entry_id}`. Support conversations accept deposit, withdrawal, agents, account, and report topics through `POST /v1/support/conversations`.

Continue with [funding and money movement](https://docs.paxeer.app/funding) or explore [agent capabilities](https://docs.paxeer.app/agents).
