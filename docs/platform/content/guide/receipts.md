# Receipts

A receipt records execution, while a `200`, webhook or dashboard row reports an application outcome. A receipt is final once its block is committed. The separate execution, checkpoint and settlement architecture is retired.

`layerx receipt verify` and the `layerx-proof` sample are a local compatibility verifier for receipt bytes and caller-supplied batch facts. They do not establish block commitment. For finality, authenticate the block header and CometBFT commit and verify transaction or state inclusion against the authenticated root at the relevant height. See [Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/abc81fd2cead6b2b9d5612c6e09a6ab56f13c1ed/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

## What verification needs

The local compatibility check takes the canonical receipt bytes and the batch facts they should be checked against.

| Batch fact | What it is |
|---|---|
| `batch_id` | Which batch this receipt claims membership in |
| `asset` | The asset the batch settles |
| `previous_state_root` | The state the batch started from |
| `resulting_state_root` | The state it produced |
| `sequencer_public_key` | The independently trusted key for the compatibility signature check |

Verification is a pure function of those inputs. No node, no daemon, no database, no clock and no network connection is involved. This is what makes a receipt portable: you can hand it, and the batch facts, to a counterparty who has never spoken to us, and they can check it for themselves.

## Where the batch facts must come from

Use an independently trusted source for compatibility batch facts. If the party supplying the receipt also supplies all its trust inputs, a successful check proves only internal consistency. An archived batch header or mirror publication does not establish current network finality; use committed block evidence for that claim.

## Verifying in code

```rust sample=verify-receipt-rust
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::production::verify_receipt;

fn settlement(receipt: &[u8], batch: &BatchFacts) -> Result<Settlement, String> {
    let authorised = AuthorizedBatch::new(batch.batch_id, batch.asset, batch.previous_state_root, batch.resulting_state_root, batch.sequencer_public_key);
    let verified = verify_receipt(receipt, &authorised).map_err(|failure| format!("receipt refused at {:?}", failure.check))?;
    let facts = verified.receipt().protocol().ok_or("receipt carries no protocol facts")?;
    Ok(Settlement { level: verified.level().wire_rank(), amount: facts.amount(), result_code: facts.result_code(), digest: verified.evidence().receipt_digest().ok_or("verifier produced no digest")? })
}
```

## Verifying from the command line

```
layerx receipt get <receipt-id> --json
layerx receipt verify \
  --receipt ./receipt.hex \
  --batch-id <hex> \
  --asset <hex> \
  --previous-state-root <hex> \
  --resulting-state-root <hex> \
  --sequencer-public-key <hex>
```

`receipt get` fetches receipt material; on the merged network its argument is a transaction hash for `pax_sequencerReceipt`. Fetching is not verification. `receipt verify` never touches the network: every fact it checks against is an argument you supplied. The `--receipt` file may hold the canonical bytes raw, or the same bytes as ASCII hex - the CLI accepts either, so piping a hex field out of a JSON response into a file works without a conversion step. The retained command example above uses compatibility inputs.

On success it prints the verification level as its wire rank, the receipt digest, the activity id, the batch id, the protocol result code and the canonical byte length. On failure it names the exact check that failed - not a generic error.

## Verification levels

The current network reports achieved evidence rungs:

| Rung | Means |
|---|---|
| `submitted` | A known transaction without a signed execution receipt |
| `receipted` | A signed execution receipt awaiting block commitment |
| `committed` | The committed block covers the receipt; block commit is finality |

`checkpoint-finalised` and `done-finalised` are retained API names for the `committed` rung. `settlement-anchored` and `paxeer-finalised` are never reached on the merged network. Check the achieved rung explicitly: a signed execution receipt and a local compatibility wire rank do not establish block commitment.

## `still-checking` is an answer

If a receipt cannot presently be verified, that is reported as itself. It is not an error and it is not a failure, and treating it as either is how double-payments happen. The rule across every LayerX surface is the same: an unknown outcome stays unknown until something resolves it.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | Compatibility verification is a pure function of receipt bytes and batch facts; finality requires committed block evidence. |
| Atomic settlement | `protocol` | A verified receipt describes a payment that applied whole. |
| Conserved supply | `protocol` | The amounts in a verified receipt are the amounts that moved. |
| Replay refusal | `protocol` | A receipt cannot be presented twice to settle twice. |
| Honest verification levels | `agent-layer` | Levels are reported exactly; `still-checking` is first class. |
