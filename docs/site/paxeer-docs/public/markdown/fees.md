<!-- Source: https://docs.paxeer.app/fees/ -->

# Fees and payment costs

Understand Paxeer X EVM gas, kernel execution fees, program occupancy, merchant pricing, spending ceilings and fee estimation.

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

Use the EVM RPC and wallet fee controls for an EVM transfer or contract call. Native LayerX activity fee estimates apply to canonical activities rather than arbitrary EVM transactions. A workflow spanning both execution planes should expose the costs and evidence of its individual legs.

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

There is no public `lx_estimateFee` JSON-RPC method in this checkout. The hosted gateway refuses every `lx_*` method name with JSON-RPC `-32601` (`Method not found`). Do not present an `lx_estimateFee` request, or response fields such as `observed_head_sequence`, `state_root`, or `verification: "authenticated_committed_snapshot"`, as a public estimation contract.

The hosted core fee estimate that does exist prices an EVM transaction: it computes `gas_limit × max_fee_per_gas` and returns only `fee` (decimal string), `parameter_version: 0`, an empty `canonical_schedule`, and `canonical_bytes`. For native activities, price the exact canonical bytes against the committed schedule described above and set the signed `fee_limit` from that calculation.

An estimate reserves no balance, locks no schedule, and proves no execution. When the schedule needs execution or storage meters that estimation cannot establish, the estimator refuses instead of treating the missing meters as zero. LNI `fee_estimate` and `session_fee_state` expose the committed schedule projection with the same non-reservation semantics.

## Set and fund the fee ceiling

1. Prepare the exact canonical activity and estimate its fee.
2. Choose a maximum in integer base units and disclose that `fee_limit` with the other payment terms.
3. Ensure the actor's spendable fee balance covers the full limit, even when the estimate is lower.
4. Sign and submit the approved bytes, preserving their activity identifier and idempotency key.
5. Verify the final receipt, charged fee, result code, and requested commitment.

| Outcome | Sequence and fee behavior | Module effects |
| --- | --- | --- |
| Admission refused | No global sequence, no consumed account sequence, no fee | None |
| Insufficient spendable balance for the limit | `LXP_ERR_FEE_UNPAYABLE`; refused before sequence assignment | None |
| Admitted execution succeeds | Sequence consumed and computed fee charged within the limit | Committed |
| Admitted execution fails | Sequence consumed; fee charged within the limit; failure receipt emitted | Rolled back |
| Computed fee exceeds the signed limit | `LXP_ERR_FEE_LIMIT`; charge capped at the limit | Rolled back |

Protocol fee charges settle as `402LXP` transfer legs with reason `LXP_REASON_PROTOCOL_FEE` to treasury account `system:fees`. Replay verifies the recorded charge, actor debit, treasury credit, schedule version, and resulting state. A failed payment can therefore have a fee-bearing receipt even when its requested transfer did not apply.

## Programs occupancy and persistent storage

Occupancy charges for namespace bytes held across protocol batches. Its time unit is the batch number, not elapsed wall-clock time. The CALL flow prices its execution budget and carves authorized occupancy out of the signed fee limit. Due maintenance transfers settle as ordinary `402LXP` legs.

Occupancy is a separate batch settlement. A CALL that writes persistent storage is not fully settled until occupancy settlement for its containing batch completes. Program outcomes carry occupancy byte-batches, fee units, Asset identifier, evidence digest, and transfer root. Preserve the outcome and batch evidence when reconciling persistent storage costs; simulation alone does not perform batch occupancy settlement.

## Merchant, metered, and subscription pricing

A merchant quote binds cart lines, integer totals, Asset, recipient, scheme, network, and timeout into a request digest. One cart uses one payment Asset and recipient. Catalog changes create new quote digests. In x402, accept the offered requirement without editing its amount or recipient, and distinguish that resource price from the network fee needed to settle it.

Metered and subscription grants authorize bounded future draws with explicit purpose, expiry, and billing windows. Each draw or renewal remains a separately authorized receive and settlement. Price a complete workflow by its actual activities and storage obligations, rather than assuming that an initial grant covers every future network charge.

## Governance and parameter changes

Governance controls `fee.base`, `fee.activity`, `fee.byte`, `fee.exec`, `fee.storage`, `fee.multiplier_bps`, and `fee.encoding`, together with the named Asset and module prices. Committed schedule bytes are stored under `fee.schedule`; version 4 also uses `fee.module-prices`. Read the active committed schedule when estimating instead of copying fixed prices into application code.

## Related guides

- [Payments and settlement](https://docs.paxeer.app/payments)

- [First payment walkthrough](https://docs.paxeer.app/payments-quickstart)

- [EVM execution and gas](https://docs.paxeer.app/evm)

- [Programs and occupancy](https://docs.paxeer.app/programs)

- [Public payment API](https://docs.paxeer.app/platform-api)

- [Governance](https://docs.paxeer.app/governance)
