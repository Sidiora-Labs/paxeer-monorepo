<!-- Source: https://docs.paxeer.app/agent-budgets/ -->

# Agent Budgets and Approvals

Fund bounded agent activity, delegate spending authority, review exact actions and reconcile verified results.

## A funded ceiling for autonomous work

Paxeer X budgets give an owner a bounded way to fund delegated work. A budget binds an account, asset, purpose, funding amount, period allowance and expiry. Delegates receive spending authority within that ceiling. The owner can change policy, remove delegates or close the budget as the assignment changes.

A budget is a funded ceiling over spending. Account balances, capability permissions, approval holds and budget allowances remain distinct controls. An agent must satisfy every applicable control before its action proceeds.

## Choose the enforcement boundary

| Control | Enforcement | Use |
| --- | --- | --- |
| `ProtocolBudget` | Native protocol validation. | Funded spending ceilings and delegated authority checked during execution. |
| `DaemonLimit` | The agent daemon. | Local automation limits for activity routed through that daemon. |
| Capability | Scoped agent authorization. | Activity, asset, counterparty, amount, rate, purpose and expiry restrictions. |
| Approval hold | The daemon preparation workflow. | Human review of a particular disclosed activity before release. |

Route all relevant traffic through the daemon when relying on a daemon limit or approval hold: bypassing that daemon bypasses those controls. Approval releases a prepared action and grants no additional protocol authority.

## Configure a budget

| Field | Meaning |
| --- | --- |
| `budget_id`, `budget_account` | 32-byte budget identity and account binding. |
| `asset_id`, `purpose_hash` | The funded asset and committed purpose reference. |
| `amount` | Initial funding amount. |
| `per_period_limit` | Maximum permitted spend in a budget period. |
| `period_length`, `period_start` | Period duration and initial period boundary in milliseconds of protocol batch time. |
| `rollover_policy`, `carry_cap` | No rollover, or capped carry-forward of allowance. |
| `expiry`, `revocation_sequence` | Expiry in milliseconds of protocol batch time, and the revocation counter. |
| `source_account`, `source_sequence` | Version 2 funding source and its replay-protected debit sequence. |

Amounts and limits use unsigned 128-bit values at the protocol boundary. Agent API amounts, sequences, limits and timestamps travel as decimal strings. Budget creation takes expiry in seconds; returned protocol records expose period_length_ms, period_start_ms, and expiry_ms in milliseconds. Native budgets support up to 16 delegates. The rollover constants are `LX_BUDGET_ROLLOVER_NONE = 1` and `LX_BUDGET_ROLLOVER_CAPPED = 2`.

### Example policy

Consider a procurement agent with 1,000 asset units funded, a 200-unit period ceiling, no rollover and a fixed expiry. A 60-unit purchase consumes both funding and current-period allowance. Adding funding increases available funds; it does not remove the period ceiling. A second delegate shares the budget ceiling rather than receiving an independent copy of it.

Choose asset-unit scaling before encoding values. With six decimal places, 60 units encodes as `60000000`. The example describes policy values; obtain the asset precision from its registered metadata.

## Native budget activities

Budget is module `3`. Its ten activity types occupy `0x0003xxxx`. Canonical payloads begin with a big-endian two-byte version prefix.

| Activity | Type | Purpose |
| --- | --- | --- |
| `LX_BUDGET_CREATE` | `0x00030001` | Create a funded budget and its period policy. |
| `LX_BUDGET_FUND` | `0x00030002` | Add funds, preserving policy limits. |
| `LX_BUDGET_AMEND` | `0x00030003` | Amend period limit, carry cap, expiry and rollover. |
| `LX_BUDGET_DELEGATE_ADD` | `0x00030004` | Add a permitted delegate. |
| `LX_BUDGET_DELEGATE_REMOVE` | `0x00030005` | Remove a delegate. |
| `LX_BUDGET_SPEND` | `0x00030006` | Transfer the specified amount to a recipient within the budget. |
| `LX_BUDGET_CLOSE` | `0x00030007` | Close using the required revocation sequence. |
| `LX_BUDGET_DEFUND` | `0x00030008` | Withdraw funding from a budget. Payload: 50 bytes. |
| `LX_BUDGET_REVOKE` | `0x00030009` | Revoke a budget. Payload: 42 bytes. |
| `LX_BUDGET_DELEGATE_SPEND` | `0x0003000a` | Spend as a delegate under a per-spend and total delegate capability. Payload: 114 bytes (version, budget, recipient, amount, and a 32-byte idempotency key). |

### Canonical spend payload

```
version:u16be = 1
budget_id:bytes32
recipient:bytes32
amount:u128be

Total: 82 bytes
```

Create supports version 1 at 211 bytes and version 2 at 251 bytes. Fund supports version 1 at 50 bytes and version 2 at 58 bytes. Version 2 create appends the source account and source sequence; version 2 fund appends the source sequence. Period maintenance uses the protocol batch timestamp to roll due periods.

## Agent API workflow

1. Use `budget.create` with the intended enforcement mode and establish the permitted agent capability. A `ProtocolBudget` create requires the signed `BudgetAuthorization`; that authorization is refused for a `DaemonLimit`.
2. Use `budget.fund` to add funds to a protocol budget. Funding requires the signed authorization carrying its preparation reference and signature; funding a `DaemonLimit` is refused, because a daemon limit holds no funds.
3. Inspect state with `budget.reconciliation` before scheduling new spending. The `budget.state` read returns the budget record, proven head and balance.
4. Prepare the action with actor, authority, account sequence, timestamp bound, idempotency key, fee limit, payload and payload hash.
5. Review the returned disclosure, then sign and submit the exact preparation.
6. Track the submission and reconcile its verified receipt with the budget and account state.
7. Use `budget.revoke` and capability revocation when the assignment ends. Protocol-budget revocation requires BudgetAuthorization for the exact prepared revoke activity.

Protocol spending uses the native budget activity and protocol validation. Daemon budget objects carry their enforcement mode explicitly. Keep receipt evidence attached to reconciled spending so local scheduling decisions follow committed results.

## Approve the exact action

An approval record contains `approval_id`, tenant, the held structured activity disclosure, canonical bytes digest, hold reason, creation time, expiry and state. Review actor, authority, counterparties, amounts, asset, fee limit, expiry and idempotency key alongside the canonical digest.

| Operation | Behavior |
| --- | --- |
| `approval.list` | Find approval records in the authorized scope. |
| `approval.get` | Inspect one held action and its disclosure. |
| `approval.approve` | Release the exact preparation bound to the reviewed digest. |
| `approval.reject` | Reject the held action. |

Decision operations carry an `idempotency_key`. Approval states are `Held`, `Granted`, `Rejected`, `Expired` and `Defective`; all except Held are terminal. A modified activity requires its own preparation and review. Approval does not override a missing delegate grant, expired capability or exhausted budget.

## Reconcile before retrying

Submission states include Prepared, Signed, Queued, Submitted, Unknown, Executed, Committed, Failed and Expired. `Unknown` is a pending state requiring reconciliation. Executed and Committed results carry a receipt reference. Use `track` or `wait` to inspect evidence and the rung actually reached (`submitted`, `receipted`, or `committed`) before making a new economic attempt.

### Common refusals

| Refusal | Response |
| --- | --- |
| `LXP_ERR_BUDGET_PERIOD_CAP`, `LXP_ERR_BUDGET_ALLOWANCE_EXCEEDED` | Inspect period policy and remaining allowance; amend only with owner authority. |
| `LXP_ERR_INSUFFICIENT_BUDGET_FUNDS` | Reconcile funds and fund the budget if appropriate. |
| `LXP_ERR_UNAUTHORIZED_DELEGATE`, `LXP_ERR_UNAUTHORIZED_DEBIT` | Check delegate, owner and funding-source bindings. |
| `LXP_ERR_BUDGET_REVOKED`, `LXP_ERR_EXPIRED` | Stop spending under that authority. |
| `LXP_ERR_STALE_REVOCATION`, `LXP_ERR_SEQUENCE_REUSED`, `LXP_ERR_SEQUENCE_GAP` | Refresh verified revocation and sequence state before preparing again. |

**Technical references:** `include/layerx/lx_budget.h`; `src/modules/budget/`; `services/agent/schema/agent-api/` identity, write and approval contracts.

See [identity and delegated credentials](https://docs.paxeer.app/identity) for owner authority, session revocation and recovery.
