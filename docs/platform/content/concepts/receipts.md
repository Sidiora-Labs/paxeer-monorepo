# Receipts and verification

A receipt records protocol execution. A receipt is final once its block is committed. Paxeer is one BFT network; the separate execution, checkpoint and settlement architecture is retired.

For independent finality verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the authenticated root at the relevant height. See [Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/abc81fd2cead6b2b9d5612c6e09a6ab56f13c1ed/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

The batch-based Rust sample and `layerx receipt verify` below are a local compatibility verifier. They check receipt bytes against caller-supplied batch facts without a network connection. Their success does not establish the merged network's committed rung or qualify a deployed service.

## What verification actually checks

| Check | What a failure means |
|---|---|
| Canonical encoding | The bytes are not a receipt this protocol version could have produced |
| Protocol invariants | The receipt describes a state change the transition function would not have made |
| Root chain | The receipt does not match the supplied compatibility batch roots |
| Signature | The signature does not match the supplied compatibility key |

The verifier returns the exact check that failed. It never returns a generic "invalid".

## Verification levels

The current network reports achieved evidence rungs:

| Rung | What backs it |
|---|---|
| `submitted` | A known transaction without a signed execution receipt |
| `receipted` | A signed execution receipt awaiting block commitment |
| `committed` | The committed block covers the receipt; block commit is finality |

`checkpoint-finalised` and `done-finalised` are retained API names for the `committed` rung. `settlement-anchored` and `paxeer-finalised` are never reached on the merged network. A signed execution receipt alone is not final; the compatibility verifier's wire rank does not prove block commitment.

## Verifying one yourself

The Rust sample below checks the compatibility receipt bytes against the five batch facts you supply. It returns the local verification level, amount, protocol result code and receipt digest, or the exact check that refused. Obtain the batch facts from an independently trusted source; internally consistent inputs alone do not authenticate chain evidence.

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

The same local compatibility check from a shell, with the CLI:

```text
layerx receipt verify --receipt ./receipt.bin \
  --batch-id "$LAYERX_BATCH_ID" --asset "$LAYERX_BATCH_ASSET" \
  --previous-state-root "$LAYERX_PREVIOUS_STATE_ROOT" \
  --resulting-state-root "$LAYERX_RESULTING_STATE_ROOT" \
  --sequencer-public-key "$LAYERX_SEQUENCER_PUBLIC_KEY" --json
```

## Where the bytes come from

On the human plane, a journey carries evidence references. A reference whose class is `layerx-receipt` can be fetched as evidence material - the canonical bytes exactly as the protocol produced them, plus its content type and verification level. On the agent plane, `read.proof_bundle` and `export.offline` give you the same material for offline use.

You can also check compatibility receipts from an untrusted mirror archive. That path returns freshness with the result instead of implying it; it does not replace committed block evidence for finality.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | The local compatibility check needs only receipt bytes and independently trusted batch facts; finality requires committed block evidence. |
| Honest verification levels | `agent-layer` | Every successful response carries its verification status, and a shortfall is reported rather than silently downgraded. |
| Done means verified | `service` | The human service renders `done` only against verified evidence, never against its own optimism. |
| Receipt-gated resource release | `service` | Seller middleware releases your resource only after the receipt covering the request verifies. |
