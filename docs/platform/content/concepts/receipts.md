# Receipts and verification

A receipt records protocol execution. A receipt is final once its block is committed. Paxeer is one BFT network.

For independent finality verification, authenticate the block header and CometBFT commit, then verify transaction or state inclusion against the authenticated root at the relevant height. See [Committed block evidence](https://docs.paxeer.app/checkpoints/).

The batch-based Rust sample and `layerx receipt verify` below are a local compatibility verifier. They check receipt bytes against caller-supplied batch facts without a network connection. Their success does not establish the merged network's committed rung or qualify a deployed service.

## What verification actually checks

| Check | What a failure means |
|---|---|
| Canonical encoding | The bytes are not a receipt this protocol version could have produced |
| Protocol invariants | The receipt describes a state change the transition function would not have made |
| Root chain | The receipt does not match the supplied compatibility batch roots |
| Signature | The signature does not match the supplied compatibility key |

Verification errors identify the check that refused the receipt.

## Verification levels

The current network reports achieved evidence rungs:

| Rung | What backs it |
|---|---|
| `submitted` | A known transaction without a signed execution receipt |
| `receipted` | A signed execution receipt awaiting block commitment |
| `committed` | The committed block covers the receipt; block commit is finality |

`checkpoint-finalised` and `done-finalised` are retained API names for the `committed` rung. `settlement-anchored` and `paxeer-finalised` are never reached on the merged network. A signed execution receipt alone is not final; the compatibility verifier's wire rank does not prove block commitment.

## Verifying one yourself

The Rust sample below checks a successful compatibility receipt against the five batch facts you supply. It returns the local verification level, amount, protocol result code and receipt digest, or the exact check that refused. Obtain the batch facts from an independently trusted source; internally consistent inputs alone do not authenticate chain evidence.

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

The CLI verifies compatibility receipt structure and signatures while preserving either a successful or refused protocol result. Inspect `result_code` as well as the verification result:

```sh
layerx receipt verify --receipt ./receipt.bin \
  --batch-id "$LAYERX_BATCH_ID" --asset "$LAYERX_BATCH_ASSET" \
  --previous-state-root "$LAYERX_PREVIOUS_STATE_ROOT" \
  --resulting-state-root "$LAYERX_RESULTING_STATE_ROOT" \
  --sequencer-public-key "$LAYERX_SEQUENCER_PUBLIC_KEY" --json
```

## Where the bytes come from

On the human plane, a journey carries evidence references. Fetch a `layerx-receipt` reference through `evidence.get` and inspect its `content_type` before decoding `bytes_base64`. Material with type `application/vnd.layerx.receipt-proof-v2+json` is a JSON proof containing the signed receipt and key metadata. Other receipt material can carry canonical receipt bytes directly. On the agent plane, `read.proof_bundle` and `export.offline` expose proof material. The [portable receipt guide](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/docs/interop/PORTABILITY.md) describes the `PortableReceiptV2` format and independently pinned height-ranged keys.

You can also check compatibility receipts from an untrusted mirror archive. That path returns freshness with the result instead of implying it; it does not replace committed block evidence for finality.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | The local compatibility check needs only receipt bytes and independently trusted batch facts; finality requires committed block evidence. |
| Honest verification levels | `agent-layer` | Every successful response carries its verification status, and a shortfall is reported rather than silently downgraded. |
| Done means verified | `service` | The human service renders `done` only against verified evidence, never against its own optimism. |
| Receipt-gated resource release | `service` | Seller middleware releases your resource only after the receipt covering the request verifies. |
