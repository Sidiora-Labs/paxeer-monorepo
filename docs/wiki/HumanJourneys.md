# Human journeys

The browser contract is `services/human/schema/human-api/`, **major 1, minor
10** in `v1.kvx`. Journey vocabulary lives in `journeys.kvx`; passkeys and
sessions in `identity.kvx`; movement quote/commit in `movement.kvx`. Golden
files under `golden/` describe wire shapes, including retained historical
shapes. A historical vector does not make a retired operation active.

Human operates on the merged Paxeer network, chain ID **125** (`0x7d`). The
kernel is part of the network and proposes blocks; block commit is finality.
The Human HTTPS API described here is an application contract. Public node
JSON-RPC uses the root of `https://api-mainnet-beta.paxeer.network` or the
numbered `api1` through `api14.mainnet-beta.paxeer.network` hosts.

[Human custody](HumanCustody.md) explains signing, owner export and evidence.
[Hosted Human](HostedHuman.md) covers material assembly;
[Human owner registration](HumanOwnerRegistration.md) covers operator bootstrap;
[Hosted internal](HostedInternal.md) explains durable event delivery.

Every response envelope carries `{ ok, result?, error?, trace }`. Money amounts
are decimal strings of base units with an explicit currency, never JSON
numbers. Mutations that can move money or change protocol state require
`Idempotency-Key`; repeat the same request under the same key to recover the
original outcome. Live updates use `POST /v1/stream` and
`GET /v1/stream/{cursor}` (`v1.kvx`, `stream.kvx`).

---

## Journey vocabulary

Active kinds are `onboarding`, `move`, `agent-create`, `agent-fund`,
`agent-pause` and `agent-retire`. The enum also retains the retired kinds
`wallet-binding`, `deposit`, `withdraw` and `exit` for compatibility history.
`agent-fund` has no dedicated funding HTTP operation: use move quote/commit.
The pause operation returns an `Agent`, not a new `Journey`.

`JourneyState` is `getting-ready`, `sending`, `processing`, `done`,
`done-finalised`, `still-checking`, `refused` or `waiting-for-you`.
A `Journey` has `journey_id`, `kind`, `state`, `state_copy_key`, `stages[]`,
`evidence[]`, `started_at` and `updated_at`, with optional `refusal` and
`wallet_request`. Stages retain their own evidence and may expose transaction
hash and nonce when known.

| Read | Result |
| --- | --- |
| `GET /v1/journeys` | `JourneyPage` |
| `GET /v1/journeys/page/{cursor}` | Next `JourneyPage` |
| `GET /v1/journeys/{journey_id}` | `Journey` |
| `GET /v1/evidence/{evidence_id}` | Canonical receipt/proof material with its evidence class and verification |

`still-checking` means the outcome is unknown. Resolve it by lookup using the
retained idempotency identity; controls that could duplicate the effect remain
locked. A refusal states `refused_by`, a `copy_key`, whether `money_left`, and
an optional `change_path` (`journeys.kvx`, `movement.kvx`).

## Rungs and completion

| Achieved rung | Human verification |
| --- | --- |
| `submitted` | `unverified` |
| `receipted` | `receipt-verified` |
| `committed` | `checkpoint-finalised` |

These are achieved evidence levels, not optimistic progress labels. A receipt
can report execution before its block commits. A committed block is final on
this network. The retained `checkpoint-finalised` label does not describe a
second settlement system. `paxeer-finalised`, `settlement-anchored`, evidence
class `paxeer-finality` and `settlement_domain` are retired vocabulary.
See [Human custody](HumanCustody.md#verification-levels-on-one-chain).

---

## Sign-in

Sign-in is the session prerequisite, not a `JourneyKind` (`identity.kvx`).

| Step | Operation | Request | Response |
| --- | --- | --- | --- |
| Begin passkey | `POST /v1/passkeys/assertions` | optional `email` | `assertion_id`, `ceremony`, `expires_at` |
| Finish passkey | `POST /v1/passkeys/assertions/{assertion_id}` | `credential` | `PasskeyAssertion` |
| Open session | `POST /v1/sessions` | `assertion_id`, `device` metadata | `Session` |
| Refresh | `POST /v1/sessions/refresh` | empty | `Session` |
| Fee policy | `GET /v1/sessions/fee-policy` | empty | `NativeFeeAsset` |

The additive schema makes `device` optional, but the production service requires
it and refuses an opening request that omits it (`identity.kvx`).
Related: `GET /v1/sessions`, `DELETE /v1/sessions/{session_id}` and
`POST /v1/sessions/revoke-all`. Passkey ceremonies are opaque credential
records; the browser does not derive a protocol primary key from them.

## Onboarding

Kind: `onboarding`. Onboarding establishes the application identity, custody
key, protocol identity, initial funding and recovery registration. Local stage
completion and receipt-only acknowledgment do not activate protocol authority.
The native acceptance path requires a successful **committed** result bound to
the intended action, network and payload
(`services/human/crates/layerx-human-service/src/onboarding/native.rs`).

| Step | Operation | Request | Response |
| --- | --- | --- | --- |
| Create account | `POST /v1/accounts` | `email`, `display_name` | `account_id`, `onboarding` journey |
| Begin passkey registration | `POST /v1/passkeys/registrations` | `account_id` | `PasskeyRegistrationChallenge` |
| Finish registration | `POST /v1/passkeys/registrations/{registration_id}` | `credential` | `Passkey` |
| Status | `GET /v1/onboarding` | empty | `OnboardingJourney` |
| Resume | `POST /v1/onboarding/resume` | empty | The retained onboarding journey |

EVM binding uses gas station sponsorship for `bindLayerX` at `0x1004`, with
`LayerXBound` confirmation and sponsor/transaction facts when known
(`identity.kvx`, `src/server/onboarding_sponsor.rs` and
`src/onboarding/gas_station.rs` under the Human service). This is separate from
the retired wallet-binding HTTP journey below. Resumption preserves completed
stages and the original action identities.

The current production dispatcher finalizes the configured onboarding sponsor
from retained identity and recovery transactions. For an unfinished principal
other than that sponsor, `advance_native_onboarding` returns
`operation-retired`; session opening requires onboarding state `Complete`
(`src/server/production_onboarding.rs`, `src/server/production_components.rs`).
The schema operations above do not guarantee general account provisioning in
every deployment.

Copy keys include `onboarding.stage.creating-your-account`,
`onboarding.stage.adding-your-passkey`,
`onboarding.stage.setting-up-your-protocol-identity`,
`agent.create.stage.first-funding` and
`onboarding.stage.putting-recovery-in-place`
(`src/server/identity_dispatch.rs`). Identity activation and unfinished recovery
are separately reported; do not infer full onboarding completion from a local
key record.

## Move

Kind: `move`. The user chooses source, destination and money; the resolver
derives `fund`, `allocate`, `return` or `transfer`. These mechanisms operate on
the same network. Bank transfers use `0x1001` and exact EIP-1559 signing with
the pending sender nonce and disclosed gas/fee limits (`movement.kvx`).

| Step | Operation | Request | Response |
| --- | --- | --- | --- |
| Quote | `POST /v1/moves/quote` | `source`, `destination`, `money` | `quote_id`, description, mechanism, money, fee estimate/ceiling, arrival estimate, expiry; optional irreversibility and native-send-access disclosure |
| Commit | `POST /v1/moves` | `quote_id`, optional `native_access_id` | `Journey` |

Show the quote's description, fee ceiling, expiry and any irreversibility
before commitment. Estimates are replaced by evidence-backed actuals. An
expired quote refuses with `quote-expired`; obtain a new quote. Do not resubmit
a leg whose outcome is unknown.

When the quote supplies native-send access, use
`POST /v1/native/send/access/begin` to obtain the service-derived confirmation
digest and `POST /v1/native/send/access/confirm` with matching step-up to obtain
`native_access_id` (`v1.kvx`). Preserve the disclosed access limits, commitment,
nonce and fee budget; commit references that authorized access.

If a journey exposes `wallet_request`, pass its `to_sign_base64` unchanged to
the named `from_address` at the indicated stage. Its `chain_id`, pending
`nonce`, `max_fee_per_gas` and `max_priority_fee_per_gas` describe the exact
transaction. Do not create a custody deposit or withdrawal to implement a move.

## Retired custody routes

The following eleven operations are **retired**, including their status and
eligibility reads. They return **HTTP 410**, code `operation-retired`, copy key
`error.operation.retired`, retry `final`. This inventory identifies old clients;
it is not a sequence to initiate a journey.

| Historical family | Retired operations |
| --- | --- |
| Wallet binding | `POST /v1/wallet-binding/statement`; `POST /v1/wallet-binding`; `GET /v1/wallet-binding`; `POST /v1/wallet-binding/rebind/action`; `POST /v1/wallet-binding/rebind` |
| Deposit | `POST /v1/deposits`; `POST /v1/deposits/{journey_id}/confirm` |
| Withdrawal | `POST /v1/withdrawals`; `POST /v1/withdrawals/{journey_id}/claim` |
| Emergency exit | `GET /v1/exit/eligibility`; `POST /v1/exit` |

`identity.kvx` and `movement.kvx` mark these operations `retired = "true"`.
The HTTP adapter rejects them before backend execution
(`src/server/http.rs`, `src/server/backend.rs`). Retained kinds, wallet-sign
copy keys and settlement fields support historical decoding, not live custody
bridge instructions.

---

## Approvals

Approvals are not a `JourneyKind`. A held activity can put its journey in
`waiting-for-you`. `ApprovalState` is `pending`, `approved`, `rejected`,
`expired` or `defective`; facts must come from the held activity's disclosure
(`agents.kvx`).

| Step | Operation | Request | Response |
| --- | --- | --- | --- |
| List | `GET /v1/approvals` | empty | `ApprovalPage` |
| Detail | `GET /v1/approvals/{approval_id}` | empty | `ApprovalDetail`, including facts, budget and evidence |
| Step-up | `POST /v1/step-up`, then `POST /v1/step-up/{challenge_id}` | `confirms`, then `credential` | Challenge, then `StepUpEvidence` |
| Approve | `POST /v1/approvals/{approval_id}/approve` | `step_up_evidence` | `ApprovalDecision` |
| Reject | `POST /v1/approvals/{approval_id}/reject` | empty | `ApprovalDecision` |

## Managed agents

`POST /v1/agents` takes `name`, `purpose`, `monthly_limit` and optional
`native_fee_budget`, returning an `agent-create` journey. Setup, protection and
first funding retain their own progress and evidence (`agents.kvx`).

| Action | Operation | Request | Response |
| --- | --- | --- | --- |
| List / get | `GET /v1/agents`, `GET /v1/agents/{agent_id}` | empty | `AgentPage` / `Agent` |
| Pause / resume | `POST /v1/agents/{agent_id}/pause`, `POST /v1/agents/{agent_id}/resume` | empty | `Agent` |
| Limit | `POST /v1/agents/{agent_id}/limit` | `monthly_limit` | `Agent` |
| Reclaim | `POST /v1/agents/{agent_id}/reclaim` | `money` | Move journey |
| Rotate / recover | `POST /v1/agents/{agent_id}/rotate`, `POST /v1/agents/{agent_id}/recover` | empty | `KeyChallenge` |
| Owner rotation disclosure | `POST /v1/agents/{agent_id}/rotation/disclosure` | Delay, window and idempotency fields | `SecurityAction` |
| Owner rotation | `POST /v1/agents/{agent_id}/rotation` | Same fields and `step_up` | `KeyChallenge` |
| Archive | `POST /v1/agents/{agent_id}/archive` | `confirm_name` | `agent-retire` journey |

## Security and recovery

`identity.kvx` declares security action digests, passkey management, session
revocation, authenticator setup/disable, backup-code rotation and timed recovery
evidence disclosure. Use `POST /v1/security/actions` to obtain the `confirms`
value for the selected action and optional target, then the matching step-up
ceremony. Privileged operations require evidence for that exact operation.

Owner export adds `export-primary-key` to `SecurityActionKind` and exposes
`POST /v1/security/key-export/begin` and `POST /v1/security/key-export/finish`.
The API timed-secret ceremony and the recoverable encrypted provider handover
have different current implementation boundaries; read
[owner export and handover](HumanCustody.md#owner-export-and-handover) before
integrating them. Onboarding recovery remains part of the onboarding journey.

## Home, activity and support

| Surface | Operations |
| --- | --- |
| Home | `GET /v1/home`; `GET /v1/account/balance` |
| Activity | `POST /v1/activity/query`; `GET /v1/activity/{entry_id}` |
| Exports | `POST /v1/activity/exports/statement`; `POST /v1/activity/exports/evidence` |
| Support | `POST /v1/support/conversations`; `GET /v1/support/conversations` |

Home and balance retain their source verification, optional rung and freshness.
The summary cannot promote an individual fact's evidence. Journey and approval
transitions feed a durable observation outbox; webhook delivery and restart
behavior are described in [Hosted internal](HostedInternal.md).

[Human custody](HumanCustody.md) · [Hosted internal](HostedInternal.md) ·
[Home](Home.md)
