<!-- Source: https://docs.paxeer.app/fees/ -->

# Fees and payment costs

Understand EVM gas, canonical kernel fee schedules, program occupancy, merchant prices, and fee ceilings on Paxeer.

## Identify the charge you are approving

| Cost | What it pays for | Where to inspect it |
| --- | --- | --- |
| EVM gas | EVM transaction execution, measured in gas and priced in the native gas token | Transaction fee fields, current EVM parameters, gas estimate, execution receipt |
| Native protocol fee | Canonical activity admission and execution under the committed kernel fee schedule | Fee estimate, signed `fee_limit`, canonical receipt |
| Programs CALL fee | Program execution under its versioned execution budget and fee schedule | Program-specific estimate, authorized budget, CALL outcome |
| Programs occupancy | Namespace bytes that persist across protocol batches | Occupancy receipt and batch settlement evidence |
| Service or merchant price | The resource, cart, subscription, or metered usage being purchased | Accepted quote, x402 requirement, order digest, payment receipt |
| Provider or conversion cost | Charges attached to the selected external payment mechanism or rail | The provider's terms and the accepted route quote |

A purchase amount and its network execution cost are separate terms. A merchant cart total is the sum of catalog prices and quantities; it does not define the protocol schedule. There is no universal merchant processing or ramp percentage to apply across every route. Review the specific quote and provider terms for the mechanism you select.

## EVM gas

EVM transactions use Ethereum-style gas accounting. Dynamic-fee transactions carry maximum fee and priority fee fields; execution receipts report gas used and effective gas price. Query the current network settings when preparing a transaction rather than assuming fixed base-fee bounds.

EVM transfers and contract calls use the node's EVM fee estimates. The kernel's canonical activity schedule is implemented in `src/protocol/lxp_fee.c`; it prices activity bytes and resource meters. Display each charge against the operation that incurs it.

## The native committed schedule

Native activity fees are deterministic functions of committed schedule bytes and execution meters. Parameters include `base_fee`, `per_activity_type_unit`, `per_encoded_byte`, `per_execution_unit`, `per_storage_unit`, `multiplier_basis_points`, Asset prices, and module prices.

| Schedule version | Pricing capabilities | Canonical encoding |
| --- | --- | --- |
| 1 | Base and resource coefficients | 86 bytes |
| 2 | Coefficients plus ten named Asset prices | 87 + 16 × 10 bytes |
| 3 | Eleven Asset prices, adding the withdrawal slot | Version 2 plus 8 bytes |
| 4 | Version 3 pricing plus seven named module prices | Version 3 head plus a module-count byte and 16 × 7 price bytes |

Schedule versions select an encoding and pricing contract; they are distinct from an individual estimate's observed state. The withdrawal price slot is encoded as an unsigned 64-bit value even though other price slots use unsigned 128-bit values. An available schedule slot does not by itself determine which activity encodings a particular submission interface accepts.

## How the fee is calculated

1. For a Programs CALL carrying an exact program fee, use the declared exact fee units under the applicable program fee schedule.
2. Otherwise begin with `base_fee`.
3. For an Asset activity under schedule version 2 or later, add its named flat Asset price. For other activity types, add `per_activity_type_unit × activity_type`.
4. Under version 4, add the named module price for escrow, budget, stream, service, perpetuals, governance, or bridge activities.
5. Add the encoded-byte, execution-unit, and storage-unit coefficients multiplied by their corresponding meters.
6. Multiply the total by `multiplier_basis_points`, divide by 10,000, and round upward.

```
ordinary_fee = ceil(
  (base_fee
   + activity_component
   + applicable_module_price
   + per_encoded_byte * canonical_encoded_bytes
   + per_execution_unit * execution_units
   + per_storage_unit * storage_units)
  * multiplier_basis_points / 10000
)
```

The exact Programs fee path is valid for Programs module 9, CALL ordinal 3, with a nonzero program fee schedule version. Unknown Asset ordinals fail as `LXP_ERR_UNKNOWN_ACTIVITY`; unsupported schedule versions fail as `LXP_ERR_VERSION_UNSUPPORTED`. Integer arithmetic and canonical encodings keep independent execution and replay calculations consistent.

## Asset and module prices

| Asset operation | Parameter | Ordinal |
| --- | --- | --- |
| Register | `fee.asset.register` | 1 |
| Pause / unpause | `fee.asset.pause` / `fee.asset.unpause` | 2 / 3 |
| Open account | `fee.asset.account_open` | 4 |
| Send / receive | `fee.asset.send` / `fee.asset.receive` | 5 / 6 |
| Issue / revoke grant | `fee.asset.grant_issue` / `fee.asset.grant_revoke` | 7 / 8 |
| Mint / burn | `fee.asset.mint` / `fee.asset.burn` | 10 / 11 |
| Withdrawal price slot | `fee.asset.withdraw` | 9; schedule version 3 or later |

Version 4 module parameters are `fee.module.escrow`, `fee.module.budget`, `fee.module.stream`, `fee.module.service`, `fee.module.perps`, `fee.module.governance`, and `fee.module.bridge`. Programs uses its own exact CALL pricing path rather than a slot in this seven-module table.

## Estimate before signing

Estimate EVM gas through `eth_estimateGas` at the node's root JSON-RPC endpoint and review the fee fields before signing. Check `services/platform/hosted/gateway/openrpc.json` for the hosted gateway's declared methods.

The hosted core estimate in `services/platform/hosted/core/src/public_reads.rs` decodes an unsigned EVM transaction for chain ID `125`. It computes `gas_limit × max_fee_per_gas` and returns `fee` as a decimal string, `parameter_version: 0`, an empty `canonical_schedule`, and `canonical_bytes`. This is the transaction fee ceiling derived from its supplied fields.

An estimate reserves no balance and proves no execution. Canonical kernel pricing requires the applicable schedule and actual encoded-byte, execution-unit, and storage-unit meters. Compare the charged fee in the resulting receipt with the authorized limit.

## Set and fund the fee ceiling

1. Prepare the exact canonical activity and estimate its fee.
2. Choose a maximum in integer base units and disclose that `fee_limit` with the other payment terms.
3. Ensure the actor's spendable fee balance covers the full limit, even when the estimate is lower.
4. Sign and submit the approved bytes, preserving their activity identifier and idempotency key.
5. Verify the final receipt, charged fee, result code, and requested commitment.

| Outcome | Sequence and fee behavior | Module effects |
| --- | --- | --- |
| Submission refused | No global sequence, no consumed account sequence, no fee | None |
| Insufficient spendable balance for the limit | `LXP_ERR_FEE_UNPAYABLE`; refused before sequence assignment | None |
| Admitted execution succeeds | Sequence consumed and computed fee charged within the limit | Committed |
| Admitted execution fails | Sequence consumed; fee charged within the limit; failure receipt emitted | Rolled back |
| Computed fee exceeds the signed limit | `LXP_ERR_FEE_LIMIT`; charge capped at the limit | Rolled back |

The kernel fee policy in `src/protocol/lxp_fee_policy.c` charges admitted execution within its signed limit and rolls back failed module effects. `lxp_fee_charge` records a conserved transfer with reason `LXP_REASON_PROTOCOL_FEE` to treasury account `system:fees`. Replay checks the charge, actor debit, treasury credit, schedule version, and resulting state.

## Programs occupancy and persistent storage

Program occupancy prices persistent namespace bytes in byte-batches. Its time unit is the protocol batch number. The program fee schedule and occupancy accounting are implemented in `src/modules/programs/fees.c`.

Program outcomes include occupancy byte-batches, fee units, asset identifier, evidence digest, and transfer root, as defined in `include/layerx/lxp_receipt.h`. Use these fields and the corresponding batch evidence to reconcile persistent storage charges.

## Merchant, metered, and subscription pricing

A merchant quote binds cart lines, integer totals, Asset, recipient, scheme, network, and timeout into a request digest. One cart uses one payment Asset and recipient. Catalog changes create new quote digests. In x402, accept the offered requirement without editing its amount or recipient, and distinguish that resource price from the network fee needed to settle it.

Metered and subscription grants authorize bounded future draws with explicit purpose, expiry, and billing windows. Each draw or renewal remains a separately authorized receive and settlement. Price a complete workflow by its actual activities and storage obligations, rather than assuming that an initial grant covers every future network charge.

## Governance and parameter changes

Governance controls `fee.base`, `fee.activity`, `fee.byte`, `fee.exec`, `fee.storage`, and `fee.multiplier_bps`. `fee.encoding` selects the schedule encoding and its named Asset and module prices. `src/modules/governance/lxp_fee_params.c` resolves these parameters and reads stored schedules from `fee.schedule` and, for version 4, `fee.module-prices`.

## Related guides

- [Payments and settlement](https://docs.paxeer.app/payments)

- [First payment walkthrough](https://docs.paxeer.app/payments-quickstart)

- [EVM execution and gas](https://docs.paxeer.app/evm)

- [Programs and occupancy](https://docs.paxeer.app/programs)

- [Public payment API](https://docs.paxeer.app/platform-api)

- [Governance](https://docs.paxeer.app/governance)
