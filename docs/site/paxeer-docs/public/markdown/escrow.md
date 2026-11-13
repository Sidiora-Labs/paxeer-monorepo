<!-- Source: https://docs.paxeer.app/escrow/ -->

# Escrow and Conditional Payments

Reserve funds, capture milestone payments, refund unused balances, and resolve disputes through native Paxeer X escrow.

## Payment Guarantees for Agent Commerce

Escrow separates a payment commitment from its final distribution. A buyer or agent funds a dedicated escrow account, a named beneficiary captures funds under the agreement, and a named arbiter resolves disputes. Marketplaces, service purchases, procurement, and milestone contracts use the same protocol lifecycle.

The escrow module stores the agreement terms and lifecycle. Every movement of value uses the shared `402LXP` transfer mechanism, including funding, beneficiary payments, refunds, and dispute splits. The account namespace is `agent:<did>:escrow:<id>`.

## Define an Escrow

| Field | Purpose |
| --- | --- |
| Escrow ID and account | Identify the hold and its isolated funds. |
| Owner | Funds the hold and receives unused funds. |
| Beneficiary | Receives captured payments. |
| Arbiter | Authorizes the distribution of disputed funds. |
| Asset and amount | Specify the asset and positive funding amount in integer base units. |
| Expiry | Absolute batch timestamp deadline in milliseconds; zero means no expiry. |
| Dispute window | Absolute final timestamp for opening a dispute; zero disables dispute opening. |
| Terms hash and agreement reference | Bind the hold to the agreed conditions and reference record. |

The terms hash commits to the agreement. Applications verify delivery, evidence, or milestones against that agreement before requesting an authorized capture; hashing terms does not itself evaluate a real-world condition.

## Lifecycle and Actions

| Action | Result | State |
| --- | --- | --- |
| Open | Move the owner's funds into escrow. | `OPEN` |
| Partial capture | Pay a positive amount strictly below the remaining balance. | `PARTIALLY_CAPTURED` |
| Capture | Pay the entire remaining balance to the beneficiary. | `CAPTURED` |
| Release | Return the remaining balance to the owner. | `RELEASED` |
| Timeout | Return an expired active hold's remaining balance to the owner. | `TIMED_OUT` |
| Open dispute | Freeze ordinary capture and refund paths. | `DISPUTED` |
| Resolve dispute | Distribute the remaining funds according to the arbiter's split. | `RESOLVED` |

**Capture pays the beneficiary. Release refunds the owner.** Keep these meanings visible in application buttons and transaction previews.

### Authority and Deadlines

The beneficiary or an owner principal acting through a delegated capability authorizes capture. The owner authorizes release. Either owner or beneficiary can open a dispute while the hold is active and the dispute deadline has not passed. Only the designated arbiter resolves the dispute.

Captures fail at or after a nonzero expiry, even before a timeout sweep changes the recorded state. Batch maintenance processes due escrow deadlines using the batch timestamp, with a sweep capacity of 24 (`LX_ESCROW_SWEEP_CAPACITY`). Disputed holds do not follow the ordinary timeout refund path; they require resolution.

The dispute deadline is inclusive: a dispute opens when the batch timestamp equals `dispute_window`, and the window is closed only when the timestamp is greater than it. A zero dispute window disables dispute opening.

## Milestone Payment Workflow

1. Agree on the asset, owner, beneficiary, arbiter, deadlines, and delivery conditions.
2. Open and fund the escrow for 1,000 base units.
3. After the first milestone, partially capture 300 units. The beneficiary receives 300 and 700 remain locked.
4. On completion, capture the remaining 700. Alternatively, release the unused 700 to the owner.
5. If a dispute is opened before the deadline, submit evidence to the arbiter and resolve the remaining balance.

### Dispute Split Example

A resolution specifies the beneficiary share in basis points from 0 to 10,000. With 700 units remaining and a 6,000 basis-point award, the beneficiary receives 420 and the owner receives 280. Earlier captures remain paid. The beneficiary amount rounds down to an integer base unit; the owner receives the remainder, preserving the complete escrow balance.

```
beneficiary = floor(remaining_balance × beneficiary_basis_points / 10000)
owner       = remaining_balance - beneficiary
```

## Protocol Reference

Escrow is module `2`. Activity payloads use fixed-width IDs and integer amounts; they are protocol activities rather than EVM contract method names.

| Activity | Type | Payload bytes |
| --- | --- | --- |
| `LX_ESCROW_OPEN` | `0x00020001` | 288 |
| `LX_ESCROW_CAPTURE` | `0x00020002` | 80 |
| `LX_ESCROW_PARTIAL_CAPTURE` | `0x00020003` | 80 |
| `LX_ESCROW_RELEASE` | `0x00020004` | 64 |
| `LX_ESCROW_TIMEOUT` | `0x00020005` | 64 |
| `LX_ESCROW_DISPUTE_OPEN` | `0x00020006` | 32 |
| `LX_ESCROW_DISPUTE_RESOLVE` | `0x00020007` | 68 |

### Retries and Failure Handling

Capture, release, timeout, and resolution carry a nonzero idempotency key. Preserve that key when retrying the same operation. Keys bind to the escrow and operation context; reusing one for a different payment is rejected. Track the returned receipt and final state before advancing a business workflow.

Handle expired holds, disputed holds, unauthorized capture, and capture amounts above the remaining balance as distinct failures. A partial capture cannot equal the entire balance: use full capture instead. Out-of-range dispute splits are rejected.

## Related Guides

- [Continuous and metered payments](https://docs.paxeer.app/streams)

- [Protocol modules](https://docs.paxeer.app/modules)

- [Build with the SDK](https://docs.paxeer.app/sdk)
