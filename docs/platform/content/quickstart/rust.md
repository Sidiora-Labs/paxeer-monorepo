# Rust quickstart

Rust is where LayerX Network is implemented, so the Rust surface is aimed at the two things Rust callers actually want: talking to the protocol directly through `layerx-client`, and verifying evidence with no service in the path.

> `layerx-sdk` ships the agent-plane client, the typed error taxonomy, the receipt and mirror verifiers, and typed human-plane call construction through `HumanApiCalls`. It does not yet ship an HTTP client for the human plane. Until it does, make the payment with the `layerx` CLI - which is a real Rust client over the same endpoint - and do the part that matters in your own process: verify the receipt.

## Make the payment

```text
layerx payment test --from "$LAYERX_SOURCE" --to "$LAYERX_DESTINATION" \
  --currency "$LAYERX_CURRENCY" --amount "$LAYERX_AMOUNT" \
  --idempotency-key order-2f9c1b7e4a10 --json
```

That quotes and commits under a key you chose and prints the journey. Fetch the receipt material it references:

```text
layerx receipt get "$EVIDENCE_ID" --json
```

The retained command example uses the compatibility evidence identifier. On the merged network, `receipt get` takes a transaction hash and fetches `pax_sequencerReceipt`; fetching that response does not verify its execution or finality.

## Verify it yourself

The sample is a local compatibility verifier for receipt bytes and caller-supplied batch facts, like `layerx receipt verify`. Its wire rank does not establish current network finality. A receipt is final once its block is committed. Verify the block header, CometBFT commit and transaction or state inclusion at the relevant height as described in [Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/abc81fd2cead6b2b9d5612c6e09a6ab56f13c1ed/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

The network rungs are `submitted`, `receipted` and `committed`. `checkpoint-finalised` and `done-finalised` are retained API names for the `committed` rung. `settlement-anchored` and `paxeer-finalised` are never reached on the merged network.

```text
cargo add layerx-sdk layerx-proof
```

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

`AuthorizedBatch::new` takes the five 32-byte facts that identify the batch you are checking against: the batch identifier, the asset, the previous and resulting state roots, and the sequencer public key. `verify_receipt` returns the exact check that failed - canonical encoding, invariant, root chain or signature - rather than a generic error, and on success gives you the verification level, the protocol facts and the receipt digest.

Get those five compatibility facts from an independently trusted source. If the receipt supplier also supplies all the trust inputs, the check establishes only internal consistency. Neither those facts nor the sample's result replace committed block evidence.

## Run the whole sample

```text
cd services/platform/examples/samples/verify-receipt-rust
cargo build --release
LAYERX_RECEIPT_FILE=./receipt.hex \
LAYERX_BATCH_ID=$(cat ./batch/batch-id.hex) \
LAYERX_BATCH_ASSET=$(cat ./batch/asset.hex) \
LAYERX_PREVIOUS_STATE_ROOT=$(cat ./batch/previous-state-root.hex) \
LAYERX_RESULTING_STATE_ROOT=$(cat ./batch/resulting-state-root.hex) \
LAYERX_SEQUENCER_PUBLIC_KEY=$(cat ./batch/sequencer-public-key.hex) \
cargo run --release
```

The batch facts are read from files here for a reason: they must come from somewhere you trust independently of whoever handed you the receipt. The receipt file itself may be raw canonical bytes or the hex text the CLI prints; the sample accepts either. It has no dependency beyond `layerx-proof` and `layerx-sdk`, and it is its own cargo workspace, so it does not join the platform workspace when you copy it.

## Verifying without any LayerX component

`verify_mirror_receipt` checks a compatibility receipt out of an untrusted mirror archive: it validates the archive, checks the batch header against trust you configure, proves inclusion, and returns freshness alongside the result instead of implying it. That local path needs no node, gateway or hosted service; it does not establish the merged network's committed rung.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | This program checks compatibility receipt bytes against trusted batch facts; finality requires committed block evidence. |
| Atomic settlement | `protocol` | The receipt describes a state change that either happened whole or not at all. |
| Conserved supply | `protocol` | A receipt describing a balance change outside a 402LXP transfer would not verify. |
| Honest verification levels | `agent-layer` | The level returned is the one the evidence justifies, never a requested one that was not achieved. |
