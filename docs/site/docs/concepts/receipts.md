# Receipts

A receipt records an execution outcome computed by Paxeer X Network. There
is one BFT network, with EVM chain ID `125` (`0x7d`); the kernel is part of
the network and proposes blocks. A receipt becomes final when the block
covering it is committed.

## Transaction identity and outcome

Submit the signed transaction through `eth_sendRawTransaction` and retain
its exact bytes and transaction hash. `pax_sequencerReceipt` looks up that
hash. The signed protocol receipt carries the same hash as `activity_id`,
along with its global sequence, state roots, result code, effects, fee,
signed height and receipt-key identity. These are computed execution facts.

The RPC response includes `txHash`, `nonce` and `rung`. Once a receipt
exists, it also carries `receipt` (the canonical signed bytes), `keyId`,
`signingKey`, `seq`, `height`, `status`, logs, return data and execution
metadata. A committed response adds `blockNumber` and `blockHash`.
`rpc/sequencer_receipt.go` defines the response and checks that its decoded
view agrees with the signed bytes, including hashes of returned data.

A signed receipt can describe a failed transaction. Inspect the verified
result: protocol `result_code == 0` means success; a nonzero result remains
a failure even when its block is committed. The RPC view represents success
as `status: "0x1"` and failure as `status: "0x0"`.

## Public evidence names

| Rung | Evidence |
| --- | --- |
| `submitted` | The node holds the transaction, but has no execution receipt. The answer carries only `rung`, `txHash` and `nonce`. |
| `receipted` | Canonical signed execution evidence exists. Its signature and outcome can be verified, but commitment has not been established. |
| `committed` | A committed block's sequence batch covers the receipt. The answer identifies the block by height and hash. |

An unknown hash returns `null`. Neither that result, a transaction hash,
an admission acknowledgement nor HTTP 202 proves execution or failure.
Resolve uncertainty using the original transaction identity. Creating a new
payment while the first remains unknown can create a second economic effect.

Commitment is the network's finality boundary. The pre-merge progression
through separate checkpoints, guarantor certificates and settlement
references is retired.

## Independent verification

`layerx-proof::receipt::verify_sequencer_signature` verifies canonical
receipt bytes against independently pinned receipt keys. Each key entry
binds a 32-byte key ID, an Ed25519 public key and an inclusive signing-height
window. The receipt's signed key ID selects exactly one entry; its signed
height must fall within that window. A `signingKey` supplied beside a receipt
does not establish authority by itself.

The node implements the governed key-list method `px_getReceiptKeys`; the
hosted gateway relays those ranges. Obtain and authenticate the list through
the verifier's trust policy before using it. When consuming the full RPC
answer, verify its view bindings as well as the receipt signature, and check
committed block evidence before claiming finality.

For offline exchange, `layerx-portable` exports
`layerx-receipt-proof-v2` with `verificationLevel: "receipted"`, including
when the source answer was committed. Its verifier checks receipt
authenticity, pinned key windows, the digest and optional raw-transaction
binding without a network connection. The carried `inclusionHeight` alone
does not prove block inclusion or finality. See
[Portable receipt verifier](../interop/portable-receipts.md) for the exact
format, APIs and independent Python implementation, and
[Mirrors](../interop/mirrors.md) for committed archives.

## Agent-layer rule

Every claimed result must be backed by evidence the consumer verified.
Keep submitted, receipted and committed distinct, preserve rejected
outcomes, and present an unresolved submission as still checking.
[Ramps and fiat rails](../interop/ramps.md) additionally require verified
external settlement evidence for the off-network part of a payment.

[Interoperability](../interop/index.md) · [Public RPC endpoints](../reference/public-rpc.md)
