# SDK terminal verification

Paxeer mainnet-beta is one BFT chain, with EVM chain ID `125` (`0x7d`). The
kernel is part of the network and proposes blocks; a receipt is final when
its block is committed. SDK verification binds a signed execution receipt to
the submitted transaction and then, for a committed result, verifies the
chain's inclusion proofs and validator commits.

The node API `pax_sequencerReceipt` and its result type are implemented in
`rpc/sequencer_receipt.go`. The word sequencer in API and code names refers
to the network's ordering and receipt machinery. The pre-merge separate
sequencer service and its settlement ladder are retired.

## Receipt progress and trust

| Rung | Evidence and meaning |
| --- | --- |
| `submitted` | A send record exists, but no signed receipt is available. |
| `receipted` | A signed execution receipt is available; its covering block has not yet been established as committed. |
| `committed` | The receipt is covered by the `MsgSequenceBatch` of a committed block. |

Execution success and commitment are separate checks. A receipt with status
zero authenticates an execution failure. A valid receipt signature alone
does not prove block inclusion or consensus commitment.

Obtain receipt key pins and chain trust independently of the response being
verified. `modules/sequencer/keeper/receipt_keys.go` defines key selection by
identifier and inclusive height range. Never accept a response's `signingKey`
as its own authority. Likewise, a validator set supplied beside a proof needs
a trusted chain identity and validator history or light-client checkpoint.
The fixture keys and validator sets are test trust anchors.

## Current verification entry points

| Implementation | Entry point and responsibility |
| --- | --- |
| `layerxproof/verify/receipt.go` | `ReceiptSignature` authenticates canonical receipt bytes under a supplied key. `ReceiptAtRoot` additionally checks a Merkle path under a supplied receipt root. The caller authenticates the key and root. |
| `rpc/sequencer_receipt.go` | `SequencerReceiptResult.Verify(pins...)` checks the RPC view against signed bytes and, when supplied, key pins. Callers must supply trusted pins; calling without them checks consistency and signature only. |
| `sdk/platform/go/verify.go` | `VerifySequencerReceipt(receiptJSON, rawTransaction, keys)` verifies a receipted JSON view, raw transaction hash, key range, sequence predecessor and execution fields. |
| `sdk/platform/go/committed.go` | `VerifyCommittedReceipt` adds execution-block and successor-block commits plus transaction, batch and execution-result inclusion. Its `ReceiptInclusion` is the SDK evidence type. |
| `sdk/platform/go/portable_receipt.go` | `VerifyPortableReceipt(data, keys)` verifies a `layerx-receipt-proof-v2` bundle using caller-supplied pins. `ExportPortableReceipt` constructs and verifies a bundle before export. |

The old `verify.BatchHeader` and `verify.ReceiptInclusion` functions are
removed. A signature over a recorder-produced batch envelope is not the
network's finality proof. Use the committed-chain verification path below.

The Go SDK's `VerifySequencerReceipt` accepts the receipted view rather than
the node's committed result envelope. Follow the adapter in
`sdk/platform/go/conformance_vectors_test.go` when preparing committed
inputs: preserve block metadata as separate evidence and verify it through
`VerifyCommittedReceipt`. Do not discard metadata and then label the
receipted verification result committed.

## Shared receipt vectors

All daemon paths below are relative to `daemon/committed-fixture/`. V1–V4
identify test vectors, not receipt protocol versions. Preserve their stored
bytes and recording provenance.

| File under `vectors/` | Evidence | Reference test |
| --- | --- | --- |
| `vector-v1-receipted-positive.json` | Locally recorded EIP-1559 contract creation with two logs, raw transaction, signed receipt, RPC view, key pins and recording provenance. | `vector_v1_test.go`: `TestVectorV1ReceiptedPositiveVerifies` |
| `vector-v2-receipted-revert.json` | LayerXLedger revert derived from the platform sequencer fixture; RPC status zero, signed result code one and `LXP_INSUFFICIENT_BALANCE: insufficient balance`. | `vector_v2_test.go`: `TestVectorV2ReceiptedRevertVerifies` |
| `vector-v3-receipt-refusals.json` | A baseline and six mutations; provenance records re-signing under an ephemeral key for the refusal corpus. | `vector_v3_test.go`: `TestVectorV3EveryRefusalIsRefused` |
| `vector-v4-committed.json` | Recorded `payExact` receipt, recorder-assembled inclusion envelope and independently collected chain evidence under `settlement`. | `vector_v4_test.go`: `TestVectorV4CommittedVerifies` |

V1 is one local creation recording. V2 and V3 derive from
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`. They do not
establish that every case was regenerated through the current producer.
The adapted SDK vectors have their own schemas and case sets; V1–V3 retain
all cases from the platform sequencer fixture. V4 adapts chain evidence to
the SDK's types, and V5 contains portable receipted and committed bundles.
The mappings and exact encodings are documented in
`sdk/platform/conformance/README.md`.

## Verify a receipted execution

1. Canonically decode the receipt and select a trusted key by key identifier
   and receipt height. Require the height to lie in the pin's inclusive
   range and the returned public key to agree with that pin.
2. Verify the signature. `ReceiptSignature` checks canonical bytes, supported
   protocol, nonzero activity identifier, a present signature and strict
   Ed25519 verification over the receipt digest. It does not authorize the
   supplied key by itself.
3. Bind the activity identifier to `keccak256(rawTransaction)`. Check the
   signed transaction's chain, sender and nonce. Bind the global sequence and
   activity identifier to the trusted predecessor sequence hash; the SDK's
   `ReceiptKeys.PreviousSequenceHash` supplies that predecessor.
4. Bind the JSON view to the signed fields: status, return-data digest, logs
   and their order, gas and fee fields. Check both constructor logs for the
   daemon V1 recording. For V2, require status zero and result code one,
   authenticate `SHA-256(returnData)` as `ActivityRoot` and decode the ABI
   `Error(string)` reason.

V3 demonstrates why signature verification must be followed by these
bindings. Except for the noncanonical case, the mutated receipts still pass
`ReceiptSignature`; full verification must refuse them:

| Mutation | Required refusal |
| --- | --- |
| `key-id-not-pinned` | The key identifier has no trusted pin. |
| `height-outside-range` | The receipt height is outside the pinned range. |
| `activity-id-not-raw-hash` | The activity identifier differs from the raw transaction hash. |
| `sequence-hash-mismatch` | The sequence hash does not follow from its predecessor, sequence and activity identifier. |
| `tampered-log` | The JSON log differs from the signed log commitment. |
| `non-canonical-receipt-bytes` | The receipt encoding is noncanonical. |

## Verify committed evidence with V4

The V4 `positive` inclusion envelope is assembled and signed by the recorder.
Its proof mutations exercise `ReceiptAtRoot` and `Ed25519Digest`: the flipped
sibling must fail Merkle verification, and a header signature under a key
outside the pinned authorization must fail. This envelope alone does not
establish commitment. The `settlement` section carries the chain evidence.

For committed verification, follow the chain checks in
`daemon/committed-fixture/vector_v4_test.go` and the SDK adapter:

1. Authenticate `settlement.receipt`. Bind `raw_transaction` to its activity
   identifier and `txHash`, and bind the receipt's block number and hash to
   `execution_header`.
2. Verify `sequence_batch_proof` and `transaction_proof` against the execution
   header's `data_hash`. Decode the `MsgSequenceBatch`, authenticate its
   signature and sequence range, and check the payment's position. The
   included Cosmos transaction must wrap the same signed EVM transaction as
   `raw_transaction`.
3. Verify the deterministic `exec_tx_result_bytes` and its proof against the
   successor `header.last_results_hash`. Bind result status and gas usage to
   the authenticated receipt. V4 is a successful `payExact` payment.
4. Verify `execution_commit` for `execution_header` and `commit` for the
   successor `header`. Check chain identity, heights, validator commitments,
   the successor's previous block and its last-commit hash. Authenticate
   validator transitions from trusted history. Each commit must carry more
   than two thirds of voting power; refuse `commit_at_most_two_thirds`.
5. Decode `payExact` and bind its payment reference, source, destination,
   asset and amount to the authenticated `PaymentExact` log.

The execution block commits the transaction; its successor authenticates
that block's execution-result root. Both are evidence from the same BFT
chain. See [Unified network](../overview/unified-network.md#receipts-and-commitment)
and [Getting started](../overview/getting-started.md#receipts-and-recovery).

## Reference verification commands

The daemon's stored-vector tests are selected from the repository root with:

```bash
go test -count=1 ./daemon/committed-fixture/ -run '^(TestVectorV1ReceiptedPositiveVerifies|TestVectorV2ReceiptedRevertVerifies|TestVectorV3EveryRefusalIsRefused|TestVectorV4CommittedVerifies)$'
```

The SDK's adapted vectors and portable bundles have their own focused gate:

```bash
go -C sdk/platform/go test -count=1 . -run '^(TestVectorFilesV1ToV3MatchSequencerReceiptFixture|TestVectorV4CommittedVerifiesAndRefuses|TestVectorV5PortableRoundTrips)$'
```

These are reference commands for SDK verification work. Their presence here
is not a claim that they ran as part of this documentation rewrite.

[One account](../concepts/one-account.md) ·
[Getting started](../overview/getting-started.md) ·
[Unified network](../overview/unified-network.md) ·
[Home](../index.md)
