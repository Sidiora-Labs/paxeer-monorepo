<!-- Source: https://docs.paxeer.app/receipts/ -->

# Receipts & Proofs

Verify what the protocol executed, what it charged, and whether the covering block has committed.

A receipt is evidence computed by the protocol. Clients submit signed activities; they do not choose receipt fields, balance outcomes, state roots, or result codes. Applications use verified receipts to reconcile payments, release paid resources, display completed work, and recover uncertain submissions.

## Activity receipt anatomy

| Evidence | Native fields | What it establishes |
| --- | --- | --- |
| Identity and order | `activity_id`, `global_sequence`, `batch_id` | The activity and its position in ordered execution. |
| State transition | `previous_state_root`, `resulting_state_root`, `activity_root` | The executed state transition and activity commitments. |
| Outcome | `result_code`, `effects`, `fee_charged` | Success or refusal, computed effects, and execution charge. |
| Rules | `protocol_version`, `module_id`, `module_version`, `parameter_version` | The execution rules needed for independent interpretation and replay. |
| Authorization | `authorization_hash`, `context_hash`, `sequencer_signature` | The authorization and context commitments authenticated by the receipt signing key. |

## Payment evidence

A successful payment operation provides protocol-computed before and after balances, asset and amount, sender and recipient account identifiers, sequence facts, transfer-set root, authorization and context hashes, chained state roots, batch identifier, timestamp, and signature. Client-supplied balances cannot establish payment.

A failed payment produces a failure result with its result code and unchanged payment balances. It does not produce a signed successful payment receipt. The enclosing admitted activity can still incur protocol fee and sequence bookkeeping. Distinguish the payment’s economic outcome from the activity’s execution record.

### Programs produce receipt-native outcomes

Program calls bind terminal success, failure, or resource exhaustion to the activity receipt. Outcome evidence includes runtime and ABI versions, fee and metering schedules, CPU fuel, memory and storage counters, output size, occupancy charges and evidence, call-graph commitments, terminal payload commitments, and transfer roots. A program’s local response or application log does not substitute for the protocol receipt.

## Finality rungs

Node evidence is `pax_sequencerReceipt`. `px_getReceiptKeys` lists the node's pinned receipt keys. A receipt is final once its block is committed. There is no later checkpoint or settlement step.

| Rung | Evidence |
| --- | --- |
| `submitted` | A known transaction without a signed execution receipt. |
| `receipted` | A signed execution receipt awaiting block commitment. |
| `committed` | A receipt covered by the committed block’s sequence batch. This is final. |

An activity identifier or submission acknowledgement establishes neither execution nor finality. Authenticate the signed receipt against an independently pinned receipt-key list, match the intended operation, and require covering-block commitment before treating the result as final.

The agent types crate accepts verification wire ranks 0, 1, 2, 3 and 6: `unverified`, `sequencer_signed`, `batch_included`, `state_proven` and `committed`. Receipt signature, batch inclusion and native state-witness checks do not authenticate a CometBFT commit. A parsed label alone does not establish verified evidence.

## Independent verification

| Surface | Use |
| --- | --- |
| `layerx-proof` | Verify signed receipt bytes. |
| `layerx receipt verify` | Perform a local check against caller-supplied batch facts. |
| `layerx-portable` | Export and verify a `layerx-receipt-proof-v2` JSON object against independently pinned `ReceiptKeyEntry` values. |

Portable verification runs offline. Its trust anchor is the caller’s independently pinned receipt-key list, including each key’s allowed signing heights. Merely packaging a receipt with a claimed signer does not make that signer authoritative. The portable proof authenticates a receipt but does not establish block commitment.

For block-level verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the corresponding authenticated root. State proofs must name the relevant height and key. An unrelated newer head does not prove an earlier receipt's effects.

### Verification workflow

1. Decode canonical receipt bytes and authenticate the signature against the pinned receipt key valid at the signed height.
2. Match the activity identifier and require the expected success result.
3. For payment, compare asset, amount, source, destination, authorization, and context with the intended purchase or transfer.
4. Verify the covering block’s receipt coverage and any state proof against the corresponding authenticated root.
5. Require the `committed` rung from `pax_sequencerReceipt` when finality is required.
6. Persist the verified receipt identifier and evidence so retries and reconciliation refer to the original economic result.

## Unknown outcomes and honest UI states

If submission fate cannot be determined, retain the signed transaction and its hash, then query the original receipt before considering a rebroadcast. Display the operation as still checking until verified evidence supports completion. An agent claims a result only after independently verifying node-produced evidence; a local success string is insufficient.

## Native receipt interfaces

The C interface in `include/layerx/lxp_receipt.h` provides `lxp_receipt_encode`, `lxp_receipt_decode`, `lxp_receipt_sign`, `lxp_receipt_verify`, and `lxp_receipt_digest`. Program evidence is bound with `lxp_receipt_bind_program_outcome` and `lxp_receipt_bind_program_artifacts`. Supply validation is exposed through `lxp_receipt_validate_supply`.

## Continue reading

- [Activity submission and recovery](https://docs.paxeer.app/activities)

- [Payments](https://docs.paxeer.app/payments)

- [Sequencing](https://docs.paxeer.app/sequencing)

- [Finality](https://docs.paxeer.app/finality)

- [Program execution evidence](https://docs.paxeer.app/programs)

- [CLI verification](https://docs.paxeer.app/cli)
