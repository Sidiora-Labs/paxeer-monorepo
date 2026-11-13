# Money and accounts

Paxeer mainnet-beta has native balances in the bank module and per-asset balances in `layerxledger`. Programs request bounded transfers through the kernel; guest code does not write balances directly. Both balance interfaces belong to the same chain.

## Amounts

Human payment amounts use decimal strings of base units with a currency code. Do not convert these strings to floating-point values.

```json
{ "amount": "1250000", "currency": "USD" }
```

SDKs retain exact amounts using integer or validated decimal-string types. Keep the asset's base-unit scale when formatting values.

## Principals and accounts

A principal identifies the actor, and an account holds balances under the applicable authority. The human API scopes profiles, activity and journeys to the authenticated principal; payment quotes accept source and destination identifiers. On the agent plane, `read.account` and `read.balance` address accounts directly.

## Movements

Human payment quotes use these movement labels:

| Mechanism | What it does |
|---|---|
| `fund` | Money enters a container from its owner |
| `allocate` | Money is set aside for a specific purpose |
| `return` | Allocated money goes back to where it came from |
| `transfer` | Money changes owner |

Funding on the network uses chain transfers. Transfers to external chains use the bridge module; an external-chain outcome must be verified under that chain's rules. Program wind-down exits release program-owned value within the network.

The current human `move.quote` operation prepares a bank transfer and reports `transfer`. Protocol budget and program operations have their own APIs. See [Paying for things](paying.md).

## What the protocol will refuse

- A transfer that would take a balance below zero.
- A program that attempts a debit without the required transfer authority.
- An EVM transaction whose sender nonce has already been consumed, or a kernel activity with an already-consumed sequence.
- A spend past a funded budget in the budget module.

The protocol enforces these checks independently of client-side validation.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Conserved supply | `protocol` | The kernel checks conservation for program transfer sets; native and asset-ledger balances retain their respective chain interfaces. |
| Atomic settlement | `protocol` | An activity applies completely or not at all. |
| Protocol budget ceilings | `protocol` | A budget is a protocol object. Spending past it is refused by the transition function, not by a policy layer you could bypass. |
| Done means verified | `service` | The human service reports a journey as done only against verified evidence at the requested level. |
