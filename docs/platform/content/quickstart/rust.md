# Rust quickstart

The Rust packages provide native node access through `layerx-client` and local receipt verification through `layerx-sdk` and `layerx-proof`.

`layerx-sdk` provides the agent client, typed errors, receipt and mirror verification, and human API call construction through `HumanApiCalls`. Human-payment transport is provided by the `layerx` CLI. The local verifier below takes compatibility receipt bytes and independently supplied batch facts.

## Make the payment

```sh
layerx payment test --from "$LAYERX_SOURCE" --to "$LAYERX_DESTINATION" \
  --currency "$LAYERX_CURRENCY" --amount "$LAYERX_AMOUNT" \
  --idempotency-key order-2f9c1b7e4a10 --json
```

That quotes and commits through the configured human-payment service under a key you chose and prints the journey. A journey carries evidence references. If you also have the native transaction hash, fetch its node receipt separately:

```sh
layerx receipt get "$TX_HASH" --json
```

Use the transaction hash for `TX_HASH`. `receipt get` fetches `pax_sequencerReceipt`; fetching that response does not verify execution or finality. The compatibility verifier below accepts a different receipt encoding.

## Verify it yourself

The sample is a local compatibility verifier for receipt bytes and caller-supplied batch facts, like `layerx receipt verify`. Its wire rank does not establish current network finality. A receipt is final once its block is committed. Verify the block header, CometBFT commit and transaction or state inclusion at the relevant height as described in [Committed block evidence](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/docs/site/docs/concepts/checkpoints.md#committed-block-evidence).

The native transaction rungs are `submitted`, `receipted` and `committed`. Human-service journey states, such as `done` and `done-finalised`, are separate fields. Use the evidence supporting a response to determine whether block commitment has been verified.

Use the local `layerx-sdk` and `layerx-proof` dependencies declared in the sample’s `Cargo.toml`.

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

Supply `LAYERX_RECEIPT_FILE`, `LAYERX_BATCH_ID`, `LAYERX_ASSET`,
`LAYERX_PREVIOUS_STATE_ROOT`, `LAYERX_RESULTING_STATE_ROOT` and
`LAYERX_SEQUENCER_PUBLIC_KEY`. The five batch facts are 32-byte hexadecimal
values obtained independently of the receipt supplier. The receipt file holds
raw compatibility receipt bytes or unprefixed ASCII hex of those bytes.

```sh
cd services/platform/examples/samples/verify-receipt-rust
cargo build --release
cargo run --release
```

The project depends on `layerx-proof` and `layerx-sdk` through local source
paths and declares its own Cargo workspace. Preserve or update those dependency
paths when copying it. Its local verification result does not replace committed
block evidence.

## Reading mirror evidence

`layerx_sdk::production::verify_mirror_receipt_from_sources` reads an archive through `MirrorSources` under a `MirrorReadPolicy` and checks the selected receipt against height-ranged `SequencerAuthorization` keys. Its result includes the source observation and freshness. Source reads use the configured mirror transport. Independently authenticate block commitment and inclusion before relying on a finality claim.

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Offline receipt verification | `protocol` | This program checks compatibility receipt bytes against trusted batch facts; finality requires committed block evidence. |
| Atomic settlement | `protocol` | The receipt describes a state change that either happened whole or not at all. |
| Conserved supply | `protocol` | The verifier checks the balance invariants for the supported receipt operation. |
| Honest verification levels | `agent-layer` | The level returned is the one the evidence justifies, never a requested one that was not achieved. |
