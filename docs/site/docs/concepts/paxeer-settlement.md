# Paxeer settlement

Paxeer mainnet-beta is one BFT network, running `paxd` with EVM chain id `125`
(`0x7d`). The kernel is part of that network and proposes blocks. Native ledger,
bank and program execution use the same chain's committed state. A successful
receipt is final once its block is committed; no second checkpoint registration
or external attestation is required for an ordinary payment.

Sub-chains are tenant domains in the database. Creating a tenant does not create
another chain or a new settlement authority. See [Assets and tokens](assets.md)
for native denominations, ledger mirrors and pointer views.

## Receipt and commit

`rpc/sequencer_receipt.go` implements `pax_sequencerReceipt` for a transaction
hash. The method name refers to the node's receipt machinery, not another
network. Its states distinguish acceptance from finality:

| Rung | Meaning | Settlement action |
| --- | --- | --- |
| No result | No receipt is available for this transaction | Keep pending; do not infer failure or resubmit a payment blindly |
| `submitted` | Accepted transaction, without signed execution evidence | Keep pending |
| `receipted` | Signed execution receipt available | Verify it, but wait for block commit |
| `committed` | A committed sequence batch covers the receipt | Check the execution result and matching chain receipt |

The node validates the canonical signed bytes, the governed receipt key and its
height range, and the fields derived from the receipt. A committed result adds
`blockNumber` and `blockHash`. A failed execution can also be committed: finality
alone does not mean the recipient was paid. `eth_getTransactionReceipt` must
show success and the expected value transfer or payment event.

## Public node and hosted surfaces

Send JSON-RPC to the root path of
`https://api1.mainnet-beta.paxeer.network/` or another public node. The public
node names are `api1` through `api14.mainnet-beta.paxeer.network` and
`api-mainnet-beta.paxeer.network`; check `eth_chainId` returns `0x7d`.
`node.hyperpaxeer.com` is only the native-library mirror, not an RPC endpoint.
There is no router or separate LayerX interface in the merged network.

Public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and
`txpool`. Hosted `px_` reads are described by
`services/platform/hosted/gateway/openrpc.json`; do not assume every public node
serves that hosted contract. Source availability also does not certify that a
particular deployment has enabled every precompile or hosted service.

For ordinary transfers, submit a signed chain transaction and follow its
receipt. Bank and ledger entry points are described under
[Assets and tokens](assets.md#transaction-entry-points). The internal
[Agent boundary](../platform/agent-boundary.md) reads chain receipts; it does not
supply an independent finality authority.

## Merchant verification

The hosted gateway's authenticated `POST /v1/settle` route is a verification
operation, not a second transfer. The implementation is `settle` in
`services/platform/hosted/gateway/src/main.rs`, with binding checks in
`src/settlement.rs`.

1. Parse the bounded x402 version-2 request. Bind the canonical receipt digest
   and the idempotency key to the principal and request digest. The request's
   `verificationLevel` is `sequencer-signed`; this is a signature label, not
   a claim that signature alone settles the payment.
2. Verify the receipt against the configured key id, public key and valid
   height range. Bind the signed execution to the claimed transaction.
3. Query `pax_sequencerReceipt` for the same transaction. Missing, submitted or
   receipted evidence remains pending. For a committed result, compare the
   canonical receipt bytes, nonce, sequence, signed execution fields and block
   identity.
4. Query `eth_getTransactionReceipt` and match transaction hash, block hash and
   number, status, gas fields and logs. Missing chain evidence remains pending;
   conflicting evidence is refused.
5. Return settled only for a successful verified execution. The merchant must
   also bind the payment to the expected asset, amount, payee and request before
   releasing the resource.

Pending results use HTTP `202` with result state `pending`. Verification
refusals can use HTTP `200` with result state `refused`; an HTTP success status
alone is not a paid order. Representative reasons are
`receipt_verification_failed`, `settlement_receipt_mismatch`,
`settlement_inclusion_mismatch` and `settlement_result_refused`. A transport
failure returns `503` `settlement_inclusion_unavailable` rather than fabricated
settlement evidence.

The merchant middleware in `services/platform/middleware/merchant/src/index.ts`
persists its checkout/request bindings and verifies receipts before recording
`paid-verified`. Preserve those bindings across retries and restarts. Proof
consumers that need independent commit authentication should follow
[Settlement evidence](../operators/settlement-evidence.md), rather than treating
a hosted JSON field as a Merkle proof.

## Historical custody and checkpoint evidence

The pre-merge execution/settlement split, custody checkpoint withdrawals,
guarantor attestations and forced-exit publication lifecycle are retired as the
ordinary network settlement model. They are not current deposit, withdrawal or
emergency-recovery instructions. An archived balance witness or recipient
signature must not be submitted as a substitute for a current chain transaction.

[Settlement evidence](../operators/settlement-evidence.md#historical-publication-formats)
retains the archive boundary and native witness encoding. Actual external-asset
bridge operations must follow the deployed bridge's current contract; the old
checkpoint workflow does not establish that such a bridge is available.

[Home](../index.md)
