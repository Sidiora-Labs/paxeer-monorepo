# SDK terminal verification

SDK verification follows the signed sequencer receipt returned by
`pax_sequencerReceipt`. The merged node serves the `eth_` and `pax_` reads;
there is no separate relay/archive role (decision.831).

The reference evidence is the four committed JSON files under
`daemon/committed-fixture/vectors/`. Their V1–V4 names identify test vectors,
not receipt protocol versions. Verify the stored bytes and their provenance
without regenerating or re-signing them.

## Receipt progress and trust

`rpc/sequencer_receipt.go` defines three rungs:

| Rung | Evidence and meaning |
| --- | --- |
| `submitted` | The sequencer holds the transaction, but no signed receipt is available. |
| `receipted` | A signed execution receipt is available; its batch has not committed. |
| `committed` | The receipt is covered by the `MsgSequenceBatch` of a committed block. |

Execution success and commitment are separate checks. A receipted revert is
an authenticated execution failure. A valid receipt signature alone does not
prove block inclusion or consensus commitment.

Obtain receipt key pins and chain trust independently of the RPC response.
The fixtures carry `receiptKeys` or `authorization` for their test chain;
production callers must use their own trusted chain configuration. Never
accept the response's `signingKey` as its own authority.

## Shared sequencer vectors

All paths in this table are relative to `daemon/committed-fixture/vectors/`.
The corresponding tests live in `daemon/committed-fixture/`.

| File | Evidence | Reference test |
| --- | --- | --- |
| `vector-v1-receipted-positive.json` | Locally recorded EIP-1559 contract creation with two logs; `rawTransaction`, `signedReceipt`, `receiptJson`, `receiptKeys` and recording provenance. | `vector_v1_test.go`: `TestVectorV1ReceiptedPositiveVerifies` |
| `vector-v2-receipted-revert.json` | LayerXLedger revert with `rawTransaction`, signed `receiptJson`, key pins and provenance. The RPC status is zero, the signed result code is one, and the revert reason is `LXP_INSUFFICIENT_BALANCE: insufficient balance`. | `vector_v2_test.go`: `TestVectorV2ReceiptedRevertVerifies` |
| `vector-v3-receipt-refusals.json` | A positive baseline and six named mutations, each carrying its expected refusal. Its provenance identifies a source receipt re-signed under an ephemeral key for the refusal corpus. | `vector_v3_test.go`: `TestVectorV3EveryRefusalIsRefused` |
| `vector-v4-committed.json` | A recorded `payExact` receipt, an inclusion envelope with two refusals, and separate Comet settlement evidence. | `vector_v4_test.go`: `TestVectorV4CommittedVerifies` |

V1 is a local contract-creation recording. The stored V2 and V3 files derive
from `sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`; their
provenance records that distinction. The adapted platform conformance files
are described in `sdk/platform/conformance/README.md`. They have their own
schemas and case sets, so do not assume they are byte-for-byte copies of the
daemon vectors.

## Verify a receipted execution

The reference primitives are in `layerxproof/verify/receipt.go`; key pin
checks are in `modules/sequencer/keeper/`, and RPC view binding is implemented
by `SequencerReceiptResult.Verify` in `rpc/sequencer_receipt.go`.

1. Canonically decode the signed receipt and select its trusted key by key
   identifier and height. Require the receipt height to fall within the
   pin's inclusive range. The returned public key must agree with that pin.
2. Verify the receipt signature. `ReceiptSignature` checks canonical bytes,
   a supported protocol, a nonzero activity identifier, a present signature,
   and the Ed25519 signature over the receipt digest under the pinned key.
   This primitive does not select or authorize a key pin by itself.
3. Bind the receipt's activity identifier to `keccak256(rawTransaction)`.
   Check the transaction's chain, sender and nonce, and bind the global
   sequence to its predecessor with `SeqHash`. Verify the predecessor
   against trusted history when establishing continuity.
4. Verify the RPC view against the authenticated receipt, including status,
   return-data digest, log contents and ordering, gas and fee fields. In V1,
   check both constructor logs. In V2, require status zero and result code
   one, bind `SHA-256(returnData)` to `ActivityRoot`, and decode the ABI
   `Error(string)` reason.

V3 demonstrates why signature verification must be followed by these
bindings. Except for the noncanonical case, its mutated receipts still pass
`ReceiptSignature`; the complete verifier must refuse them:

| Mutation | Required refusal |
| --- | --- |
| `key-id-not-pinned` | The receipt key identifier has no trusted pin. |
| `height-outside-range` | The receipt height lies outside the pinned range. |
| `activity-id-not-raw-hash` | The activity identifier differs from the raw transaction hash. |
| `sequence-hash-mismatch` | The resulting sequence hash does not follow from its predecessor, sequence and activity identifier. |
| `tampered-log` | The JSON log differs from the signed receipt's log commitment. |
| `non-canonical-receipt-bytes` | The receipt encoding is noncanonical. |

## Verify committed evidence with V4

`vector-v4-committed.json` contains two evidence layers. Its `positive`
inclusion envelope is assembled and signed by the recorder under the genesis
receipt pin; the `settlement` section contains independently collected chain
evidence. Preserve that distinction when reporting what was verified.

For the inclusion envelope, use `BatchHeader` and `ReceiptInclusion` from
`layerxproof/verify/receipt.go` with the trusted `authorization`:

- Verify the canonical header, authorized batch range, sequencer identity
  and header signature.
- Verify the receipt's Merkle path to the header's receipt root and the
  receipt's signature. Require matching protocol versions and a global
  sequence inside the header's committed range.
- Refuse `flipped-proof-sibling` with a Merkle-root mismatch even though its
  header remains valid. Refuse `header-signature-by-unpinned-key` under the
  pinned authorization.

For chain settlement, follow the V4 reference test's additional checks:

1. Authenticate `settlement.receipt`, bind `raw_transaction` to its activity
   identifier and `txHash`, and require the receipt's block metadata to
   identify `execution_header`.
2. Verify `sequence_batch_proof` and `transaction_proof` against
   `execution_header.data_hash`. Decode the batch, verify its signature and
   sequence range, and require the transaction position to match the
   receipt. The proven Cosmos transaction must wrap the same signed EVM
   transaction as `raw_transaction`.
3. Verify the deterministic `exec_tx_result_bytes` and its
   `exec_tx_result_proof` against the successor `header.last_results_hash`.
   Require successful execution and matching gas usage.
4. Verify `execution_commit` for `execution_header`, and `commit` for the
   successor `header`, using the chain identity and trusted validator set.
   Bind the successor's height, previous block and last commit to the
   execution block. The validator set must match the header commitment,
   and commits require more than two thirds of voting power.
5. Decode the `payExact` call and bind its payment reference, source,
   destination, asset and amount to the authenticated `PaymentExact` log.
   Refuse `commit_at_most_two_thirds`; inclusion under a sequencer-signed
   envelope alone cannot establish this consensus result.

The fixture validator set is test evidence, not an independent production
trust anchor. Chain verification needs a trusted validator history or light
client checkpoint in addition to receipt key pins.

## Reference verification command

From the repository root, the daemon vector tests are:

```bash
go test -count=1 ./daemon/committed-fixture/ -run '^(TestVectorV1ReceiptedPositiveVerifies|TestVectorV2ReceiptedRevertVerifies|TestVectorV3EveryRefusalIsRefused|TestVectorV4CommittedVerifies)$'
```

These tests cover the stored receipt evidence, named refusals and V4
settlement proofs. For the platform SDK's adapted vectors and verifier
entry points, use `sdk/platform/conformance/README.md`.

[Home](../index.md)
