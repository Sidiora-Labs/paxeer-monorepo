# Commitment levels

Paxeer mainnet-beta is one BFT chain run by `paxd`, with EVM chain ID `125`
(`0x7d`). Its kernel is part of the network and proposes blocks. A receipt is
final once its block is committed. Execution acknowledgements and signed
receipts received before commitment carry less evidence than a committed
block; they do not introduce a separate settlement chain.

Use node-root JSON-RPC for transaction submission and receipt recovery.
[Public RPC endpoints](../reference/public-rpc.md) lists the public nodes;
[Hosted gateway](../platform/hosted-gateway.md) describes hosted consensus
and receipt-key reads. The [Developer platform](../platform/index.md)
connects these APIs to funding and payments.

## Evidence at each level

The node method `pax_sequencerReceipt` takes one `0x`-prefixed 32-byte
transaction hash and reports these exact `rung` values:

| Rung | Evidence | Meaning |
| --- | --- | --- |
| `submitted` | `txHash` and `nonce` | The node holds the transaction; there is no signed execution receipt yet |
| `receipted` | Signed canonical receipt, signing key and decoded execution outputs | The transaction has execution evidence; its covering sequence batch is not committed |
| `committed` | The receipt plus `blockNumber` and `blockHash` of the covering committed `MsgSequenceBatch` | The receipt is covered by the chain's committed block |

An unknown transaction returns `null`. Quantities in this node response are
`0x`-prefixed hexadecimal strings. A node without a local receipt source or
configured forwarding can return `-32000` for this method. The hosted gateway
relays the node response unchanged; it cannot manufacture a stronger rung.
The implementation is `rpc/sequencer_receipt.go`.

`eth_sendRawTransaction` returning a hash proves submission only.
`eth_getTransactionReceipt` returns committed EVM receipts; it does not expose
a merely submitted or receipted transaction as a committed receipt. Check
execution status as well as inclusion: a reverted transaction may be committed
without making a successful payment.

The former `executed`, `batched` and `finalised` submission contract is retired.
`lx_sendActivity` and the old `lx_` proof/status methods are retired; the gateway returns `-32601` for them.
The separate sequencer service and guarantor finality model are retired.

## Verifying committed evidence

Authenticate the receipt signing key against an independently trusted key
list and its inclusive height range. A `signingKey` field returned with a
receipt is not sufficient trust by itself. Verify the canonical receipt
signature, the transaction identity and the decoded execution outputs; do
not accept a status string as evidence.

Hosted evidence methods are declared in
`services/platform/hosted/gateway/openrpc.json`:

| Method | Input and output |
| --- | --- |
| `px_getReceiptKeys` | Empty positional array; key IDs/public keys as `0x`-prefixed 32-byte hex, height ranges as JSON integers |
| `px_getCommit` | Positive decimal height string; signed consensus header and commit |
| `px_getValidators` | Positive decimal height string; complete validator set and decimal-string voting powers |
| `px_getSequenceBatch` | Positive decimal height string; transaction zero and Merkle proof under the block header's data hash |

A batch proof connects that transaction to the block. Receipt verification
must also bind the receipt's transaction ID, digest and sequence to the batch,
and verify the signed commit against a trusted validator set for that network
and height. An arbitrary newer block, a matching height alone or validators
supplied only by the same untrusted response cannot establish that trust.
The payment SDK's `verifyCommittedReceipt` performs committed-receipt checks;
applications must provide the independently anchored validator information.

Unavailable consensus reads return `-32003` with
`data.code: "consensus_unavailable"`. Missing gateway receipt keys return
`-32003` with `receipt_keys_unavailable`. A height without a sequence batch
returns `-32602` with `no_sequence_batch`; invalid batch evidence is refused
with `-32003` and `invalid_consensus_response`. Do not replace missing
proofs with fabricated evidence or a weaker success.

## Payment and x402 requirements

Payment middleware uses the exact commitment names `receipted` and
`committed`, defined in
`services/platform/middleware/seller/src/commitment.ts`.
An x402 offer selects one in `extra.layerx.commitment`:

```json
{"extra":{"layerx":{"commitment":"committed"}}}
```

When the LayerX extra is absent, the middleware defaults to `receipted`.
When `extra.layerx` is present, its commitment must be one of those two
values. Old names, alternate spellings and uppercase variants are refused
as verification failures. Payer, purpose and other payment terms must also
match the exact accepted offer; commitment evidence alone does not prove the
recipient, asset or amount.

A `receipted` requirement still requires a verified signed receipt and
payment facts. A `committed` requirement additionally needs a
`PaymentCommitmentResolver` providing batch inclusion, block header, commit
and validators, with `resolveValidators(network, height)` supplying an
independently trusted chain identity and validator hash. The middleware
checks that the proof's transaction hash, receipt digest and sequence match
the verified receipt before verifying commitment.

The buyer and seller verify the requested level before accepting the
payment or releasing a resource. A `verificationLevel` string, portable
receipt reference or HTTP success does not stand alone. Missing or
inconsistent committed evidence is a verification failure; it never becomes
a `receipted` success for an offer requiring `committed`.

## Pending and recovery behavior

Keep the exact signed transaction, nonce and transaction hash before
broadcast. After a timeout or unknown result, query that original hash with
`pax_sequencerReceipt` and `eth_getTransactionReceipt`. `null` can mean the
queried node has no record yet. A `submitted` rung is pending execution;
`receipted` is still pending when the application requires `committed`.
Use bounded polling and compare node heads when an endpoint trails.

Do not sign a replacement payment solely to escape an uncertain response.
Preserve the original offer, idempotency binding and verification evidence
through restart and retry. A receipt mismatch, invalid signature or failed
proof is a refusal, not a reason to reduce the commitment requirement.

[Hosted gateway](../platform/hosted-gateway.md) ·
[Developer platform](../platform/index.md) · [Home](../index.md)
