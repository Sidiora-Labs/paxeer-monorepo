<!-- Source: https://docs.paxeer.app/proofs/ -->

# Receipts & Proofs

Verify Paxeer outcomes independently with signed receipts, native state witnesses, custody proofs, and mirror archives.

## Choose the evidence for your decision

Paxeer executes transactions inside the node and returns signed execution receipts. A merchant verifies payment facts before releasing a resource; an auditor preserves the signed outcome; an account owner verifies state against an authenticated root. Finality requires the covering block to commit.

| Evidence | Establishes | Use |
| --- | --- | --- |
| Portable receipt (v2) | Canonical outcome authenticated by a pinned receipt key | Offline receipt validation and outcome accounting |
| Native state witness | A canonical account or module fact is included in a composite state root | Balance and module-state verification |
| Custody state proof | Custody state at verified heights under a pinned chain identity | Paxeer custody verification |
| Mirror archive | A receipt belongs to an archive matching an independently observed mirror commitment | Independent receipt verification from Ethereum or Solana mirrors |

## Portable receipt format

The portable format is `layerx-receipt-proof-v2`, implemented by `PortableReceiptV2` in `layerx-portable`. Verification runs offline against a receipt-key list pinned independently by the caller. The portable object establishes receipt authentication; it does not by itself prove block commitment.

| Member | Value |
| --- | --- |
| `format` | `layerx-receipt-proof-v2` |
| `verificationLevel` | `receipted` |
| `signedReceipt` | Exact signed canonical receipt bytes; lowercase `0x` hex |
| `receiptDigest` | 32-byte receipt digest; lowercase `0x` hex |
| `receiptKey` | Object with `keyId` and `publicKey` (32 bytes each, `0x` hex) and `firstHeight` / `lastHeight` (`0x` quantities): the inclusive height window in which that key signs |
| `inclusionHeight` | `0x` quantity; must not precede the signed receipt height |
| `rawTransaction` | Optional; raw transaction bytes as `0x` hex, bound to the receipt activity ID |

Byte strings are lowercase `0x` hex and heights are `0x` quantities. Unknown JSON members are refused. The JSON input limit is 1,500,000 bytes; the signed receipt must contain between 1 and 1,048,576 bytes.

### Establish the trust root first

The trust root is your pinned `ReceiptKeys` list of `ReceiptKeyEntry` values (key ID, Ed25519 public key, first and last signing height). The `receiptKey` carried in the JSON names the entry that signed; it is trusted only after it equals the entry selected from your own pinned list. Copying the key out of the incoming JSON does not authorize it.

```
// Rust: json_bytes and receipt_keys (your pinned list) are supplied by your application.
let verified = layerx_portable::PortableReceiptV2::verify_json(
    &json_bytes,
    &receipt_keys,
)?;
```

`from_json` parses the bounded representation without establishing trust; `verify_json` parses and verifies it against your pinned keys. Export uses `PortableReceiptV2::export(signed_receipt, receipt_key, inclusion_height)`, or `export_with_transaction` to also bind raw transaction bytes. Export verifies the signed receipt before producing its JSON representation. `from_sequencer_receipt` exports the signed receipt of a `receipted` or `committed` RPC answer and refuses `submitted` answers.

### What verification checks

1. Parse the bounded JSON and decode every field in its canonical `0x` form.
2. Verify the node receipt key’s Ed25519 signature on the signed receipt against the pinned key list.
3. Select the pinned entry by the receipt key ID and require the carried `receiptKey` to equal it exactly.
4. Require `inclusionHeight` to be at or after the signed receipt height.
5. Recompute the receipt digest and require it to equal `receiptDigest`.
6. When `rawTransaction` is present, require its Keccak-256 hash to equal the receipt activity ID.

```
receipt_digest domain: "LXP/v1/receipt\0"
```

A cryptographically verified rejected outcome remains rejected. The portable proof authenticates the signed receipt. Its inclusion height is checked against the signed height, but is not an inclusion proof. Verify the covering block and its receipt coverage separately when commitment is required.

**Source:** `services/interop/crates/layerx-portable/src/receipt.rs` — `PortableReceiptV2`, `verify_json`, `export`.

## Native state witnesses

Native witnesses prove a canonical key/value leaf through its module subtree into the composite state root. Account leaves with module ID 0 and a 33-byte key beginning with byte 4 include a separate positional account-registry path, followed by the module-zero `account-tree` binding and the module wrapper.

Lengths, indices, leaf counts, path depths and trailing bytes are checked. Odd-width Merkle levels duplicate the final node and require that exact sibling. The native leaf hash places both lengths before the key and value:

```
SHA256("LXP/v1/state-leaf\0" || key_len:u32be || value_len:u32be || key || value)
SHA256("LXP/v1/state-node\0" || left[32] || right[32])
```

**Source:** `services/agent/crates/layerx-proof/src/state_witness.rs`.

## Custody state proofs

The Go `custodyproof` package verifies custody state bundles under the format version `paxeer-custody-state-v3`. `Verify(request, now)` checks a decoded request against an `Expected` pin set by the caller: genesis hash, Comet chain ID, EVM chain ID, custody identity, module hash, asset ID and confirmation count. It verifies the light-block history from genesis and the state membership proofs at the requested heights before returning a result. `Decode` parses the bounded JSON input.

Pin the expected values from your own configuration. Values copied out of the incoming bundle do not authorize themselves.

**Source:** `custodyproof/verify.go`, `custodyproof/decode.go`.

## Archive and mirror verification

The Ethereum and Solana mirror tooling reads archives under configured source identities and read policies. `layerx-mirror-verify` checks the archive against the observed source commitment, requires a committed block coordinate, and verifies selected receipts against pinned receipt-key windows. It reports source provenance and freshness. The implementation is in `services/interop/crates/layerx-mirror/src/verify.rs` and its `src/bin/layerx-mirror-verify.rs` entry point.

For long-term evidence, retain signed receipts, your pinned receipt-key list, authenticated block evidence, state proofs and mirror publication references. Preserve exact bytes rather than reconstructed display JSON. See [Data Availability](https://docs.paxeer.app/data-availability) for availability verification and [Finality](https://docs.paxeer.app/finality) for block commitment.

## Verification failures

Handle typed refusals explicitly. Portable-receipt failures are distinct: JSON bounds or shape, receipt bounds, unsupported format or verification level, invalid hex, length or quantity, receipt verification failure, receipt-key mismatch, missing or mismatched digest, raw-transaction mismatch, a not-yet-receipted transaction, and an inclusion height preceding the signed height. Preserve the original activity ID for recovery and never upgrade a weaker evidence level to a stronger success.
