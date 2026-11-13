# Receipts

A receipt records execution on Paxeer X Network, while a `200`, webhook or dashboard row reports an application outcome. A receipt is final once its block is committed. Check the execution result separately: a committed transaction can have reverted.

`layerx receipt verify` and the Rust sample below provide local compatibility verification for receipt bytes and caller-supplied batch facts. They do not establish block commitment. For finality, authenticate the block header and CometBFT commit and verify transaction or state inclusion against the authenticated root at the relevant height. See [Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

## What verification needs

The local compatibility check takes the canonical receipt bytes and the batch facts they should be checked against.

| Batch fact | What it is |
|---|---|
| `batch_id` | Which batch this receipt claims membership in |
| `asset` | The asset the batch settles |
| `previous_state_root` | The state the batch started from |
| `resulting_state_root` | The state it produced |
| `sequencer_public_key` | The independently trusted key for the compatibility signature check |

The compatibility check uses only those inputs. It requires no running service, database, clock or network connection. It checks encoding, supported protocol facts, balance invariants, roots and the signature against the supplied facts.

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

```text
layerx receipt get <tx-hash> --json
layerx receipt verify \
  --receipt ./receipt.hex \
  --batch-id <hex> \
  --asset <hex> \
  --previous-state-root <hex> \
  --resulting-state-root <hex> \
  --sequencer-public-key <hex> --json
```

`receipt get` fetches receipt material; on the merged network its argument is a transaction hash for `pax_sequencerReceipt`. Fetching is not verification. `receipt verify` never touches the network: every fact it checks against is an argument you supplied. The `--receipt` file holds raw compatibility receipt bytes or unprefixed ASCII hex of the same bytes. The native `pax_sequencerReceipt` response uses a different encoding; use the native SDK verifier for that receipt. The command above uses compatibility inputs.

The result contains the verification level as its wire rank, the receipt digest, the activity id, the batch id, the protocol result code and the canonical byte length. `--json` wraps these fields in `data`. A verification failure names the check that refused the receipt. The CLI uses `verify_outcome`, so a verified receipt with a nonzero `result_code` still produces a successful command result. The Rust sample uses `verify_receipt`, which refuses a nonzero protocol result.

## Verification levels

The current network reports achieved evidence rungs:

| Rung | Means |
|---|---|
| `submitted` | A known transaction without a signed execution receipt |
| `receipted` | A signed execution receipt awaiting block commitment |
| `committed` | The committed block covers the receipt; block commit is finality |

These are the native transaction rungs. Human-service journey states, such as `done` and `done-finalised`, are separate fields. Check the achieved rung and its supporting evidence explicitly: a signed execution receipt and a local compatibility wire rank do not establish block commitment.

## `still-checking` is an answer

The human-payment service can report a journey as `still-checking` while its outcome remains unresolved. Retain the journey identifier and reconcile its evidence before starting another payment. The local receipt verifier instead returns a verification failure when its checks do not pass.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | Compatibility verification is a pure function of receipt bytes and batch facts; finality requires committed block evidence. |
| Atomic settlement | `protocol` | A successful payment applies as one state transition; inspect the receipt's result code. |
| Conserved supply | `protocol` | Receipt verification checks the balance invariants for the supported operation. |
| Replay refusal | `protocol` | Execution checks activity sequences; the offline verifier does not track receipt consumption. |
| Honest verification levels | `agent-layer` | Signed execution evidence and committed block evidence carry distinct rungs. |
