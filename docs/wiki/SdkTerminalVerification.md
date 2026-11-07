# SDK terminal verification

Paxeer is one BFT network. The kernel executes inside `paxd` and participates
in block proposal. A transaction receipt becomes final when its block commits;
there is no later settlement on a separate execution network.

Use JSON-RPC at the root of `https://api-mainnet-beta.paxeer.network` or the
numbered hosts `https://api1.mainnet-beta.paxeer.network` through
`https://api14.mainnet-beta.paxeer.network`. EVM chain ID is 125 (`0x7d`).
The public node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test`
and `txpool`. The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [the gateway OpenRPC contract](../../services/platform/hosted/gateway/openrpc.json).

The reference node receipt API is `pax_sequencerReceipt`. Its name refers to
the node's execution/proposal role. The stored V1–V4 files under
[`daemon/committed-fixture/vectors/`](../../daemon/committed-fixture/vectors/)
are test vectors, not receipt protocol versions. The directory name does not
require an additional runtime process.

## Receipt progress and trust

[`rpc/sequencer_receipt.go`](../../rpc/sequencer_receipt.go) defines three rungs:

| Rung | Evidence and meaning |
| --- | --- |
| `submitted` | The node holds the transaction, but no signed receipt is available. |
| `receipted` | A signed execution receipt is available before its batch commits. |
| `committed` | A committed block's `MsgSequenceBatch` covers the receipt. |

Execution success and block commitment are separate properties of a receipt.
A receipted revert is an authenticated execution failure. A signature or an
RPC response claiming `committed` alone does not independently prove consensus
commitment. V4 demonstrates how to verify that claim against chain evidence.

Obtain receipt-key authorization and chain trust independently of the receipt
response. Governed receipt pins bind `keyId`, `publicKey`, `firstHeight` and
`lastHeight`; both bounds are inclusive. The node's receipt-key API is named
`px_getReceiptKeys`, and the gateway relays it. Endpoint configuration controls
whether these APIs are served locally or forwarded; an unconfigured node can
return JSON-RPC error `-32000` with `served by sequencer`.
The `signingKey` beside a receipt is not its own trust anchor.

The fixtures' `receiptKeys`, `authorization` and validator sets authorize their
local test chain. Production verification needs authenticated receipt-key
history and validator history or a trusted light-client checkpoint. Preserve
the network and tenant bindings of any higher-level operation, including
[Human owner registration](HumanOwnerRegistration.md).

## Shared sequencer vectors

All file paths below are relative to
`daemon/committed-fixture/vectors/`; tests are in its parent directory.

| File | Stored evidence | Reference test |
| --- | --- | --- |
| `vector-v1-receipted-positive.json` | EIP-1559 contract creation with two constructor logs, raw transaction, signed receipt, RPC view, key pin and recording provenance. | `vector_v1_test.go`: `TestVectorV1ReceiptedPositiveVerifies` |
| `vector-v2-receipted-revert.json` | LayerXLedger insufficient-balance revert, raw transaction, signed RPC receipt, key pin and provenance. RPC status is zero, signed result code is one, and the reason is `LXP_INSUFFICIENT_BALANCE: insufficient balance`. | `vector_v2_test.go`: `TestVectorV2ReceiptedRevertVerifies` |
| `vector-v3-receipt-refusals.json` | Positive baseline and six named mutations with expected refusals. Provenance identifies the source receipt and its re-signing under an ephemeral test key. | `vector_v3_test.go`: `TestVectorV3EveryRefusalIsRefused` |
| `vector-v4-committed.json` | Recorded `payExact` receipt, a recorder-built signed inclusion envelope, two envelope refusals and BFT block evidence in the `settlement` field. | `vector_v4_test.go`: `TestVectorV4CommittedVerifies` |

V1's current recorder covers contract creation; it does not record the Programs
call and authenticated `ProgramCallOutcome` needed for broader Programs
conformance. Do not infer that coverage from this positive vector.

The stored V2 and V3 identify cases in
[`sequencer-receipt-v1.json`](../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json).
The current V2 producer in
[`vector_v2.go`](../../daemon/committed-fixture/vector_v2.go) selects its home's
primary key from the validated genesis pins, binds payer and payee DIDs,
registers an asset, opens their accounts and records an unfunded `payExact`
revert. It verifies the result, builds a canonical source case and derives
copies from that source. This is a producer pipeline, not a requirement to
reuse a stored transaction from a different recording.

For an intentional fresh recording, the existing recorder interface is:

```text
committed-fixture record --paxd PATH --out DIRECTORY --vector vector-v2-receipted-revert
```

V2 writes the canonical source and SDK group under `DIRECTORY/conformance/`
and the node vector under `DIRECTORY/`. Compare copies within the same
recording: transaction hash, signed bytes, pin, refusal code and provenance
must remain identical to that recording's canonical source. New recordings
have new transaction identities and must not be substituted for stored
fixtures without reviewing the complete derived set.

The stored-vector test verifies the checked-in V2 against its checked-in
source; it does not execute the recorder. The producer-path test is
`TestRepair_174_FixtureV2SourceContract` in `repair_174_test.go`, which drives
`recordV2RPC` through a real node/RPC path. Keep these evidence claims separate.
The [platform conformance guide](../../sdk/platform/conformance/README.md)
describes the SDK schemas and additional portable cases; the node V1 creation
and V3 re-signing are not byte-for-byte substitutes for all SDK cases.

## Verify a receipted execution

The signature primitive is `ReceiptSignature` in
[`layerxproof/verify/receipt.go`](../../layerxproof/verify/receipt.go).
Key-pin validation lives in `modules/sequencer/keeper/`;
`SequencerReceiptResult.Verify(pins)` in `rpc/sequencer_receipt.go` checks
RPC view binding. Passing no pins checks consistency without authenticating
the signer's authority.

1. Canonically decode the signed receipt and select an independently trusted
   pin by key identifier and height. Require the height inside its inclusive
   range and the returned public key to match the pin.
2. Verify the signature. `ReceiptSignature` checks canonical bytes, a
   supported protocol, a nonzero activity ID, a present signature and the
   strict Ed25519 signature over the receipt digest. It does not select or
   authorize a key pin.
3. Bind the activity ID to `keccak256(rawTransaction)`. Check the signed
   transaction's chain, sender and nonce. Compute `SeqHash` from its
   predecessor, global sequence and activity ID; authenticate the predecessor
   against trusted history when establishing continuity.
4. Bind the RPC status, return data, logs and their order, gas and fee fields
   to the signed receipt. In V1, check both constructor logs. In V2, require
   status zero and signed result code one, bind `SHA-256(returnData)` to
   `ActivityRoot`, and decode the ABI `Error(string)` reason.

V3 demonstrates why signatures must be followed by these bindings. Except
for the noncanonical case, its mutated receipts still pass `ReceiptSignature`;
the full verifier must refuse each case:

| Mutation | Required refusal |
| --- | --- |
| `key-id-not-pinned` | No trusted pin authorizes the key identifier. |
| `height-outside-range` | Receipt height is outside the pin's range. |
| `activity-id-not-raw-hash` | Activity ID differs from the raw transaction hash. |
| `sequence-hash-mismatch` | Sequence hash does not follow from predecessor, sequence and activity ID. |
| `tampered-log` | JSON log differs from the authenticated log commitment. |
| `non-canonical-receipt-bytes` | Receipt encoding is noncanonical. |

The V4 recording uses EIP-1559 type-2 transactions on EVM chain ID `125`.
Its `payExact` transaction is `0x0825b6d3abe886f04d91e13108cb78adfbe18f8a28204de0bae9901f910c64e2` and its execution block is
`0xa7c648246a5213d57d6a2f2aa7bdc8e0050249629e5e92a4b89f2bdb918e9108`. The daemon recording SHA-256 is
`5013cd232ee94ece8d25fca4c802ca60c892f3a5f637a70962d9909dc71c4e7b`. These are private fixture identities; production consumers need their own trust anchors.

## Verify committed evidence with V4

[`vector_v4_test.go`](../../daemon/committed-fixture/vector_v4_test.go) uses
`codec.DecodeBatchHeader`, `verify.ReceiptAtRoot` and `verify.Ed25519Digest`.
The Go proof package does not expose the removed `BatchHeader` and
`ReceiptInclusion` verifier functions. `codec.BatchHeader` is a data type;
SDK types with similar names are separate APIs.

The V4 `positive` envelope is assembled and signed by the recorder under its
receipt pin. It demonstrates receipt authenticity and inclusion under that
root. It is not itself a consensus certificate. Its `settlement` field holds
the recorded chain evidence for the same transaction on the same network.

For the inclusion envelope:

1. Decode the header with `codec.DecodeBatchHeader` and require canonical
   encoding, as the reference test does by re-encoding it. Independently
   check its network, protocol, sequencer identity and authorized inclusive
   batch-number range against trusted policy.
2. Require a 64-byte header signature. Verify it using
   `verify.Ed25519Digest(trustedPublicKey, signature, codec.BatchHeaderDigest(headerBytes))`.
3. Use `verify.ReceiptAtRoot(receiptBytes, proof, header.ReceiptMerkleRoot,
   trustedPublicKey)`. This checks the receipt signature and Merkle path;
   it does not authenticate the header or enforce its policy/range bindings.
4. Explicitly require the receipt and header protocols to agree and its
   global sequence to lie in the header's first/last sequence range.
   The key must be authorized for the receipt height as well as the batch.
5. Refuse `flipped-proof-sibling` with `codec.ErrMerkleRootMismatch` while
   retaining a valid header signature. Refuse
   `header-signature-by-unpinned-key` even if the signature verifies under
   the untrusted key carried by that refusal.

For BFT commitment, verify the `settlement` fields together:

1. Authenticate `receipt` under trusted height-bounded pins, bind
   `raw_transaction` to its activity ID and `txHash`, and require its block
   number/hash to identify `execution_header`.
2. Verify `sequence_batch_proof` and `transaction_proof` against that
   execution header's `data_hash`. Decode `MsgSequenceBatch`, verify its
   signature and height, and check the sequence range and transaction
   position. The proven Cosmos transaction must wrap the same signed EVM
   transaction as `raw_transaction`.
3. Re-encode `exec_tx_result` deterministically, compare
   `exec_tx_result_bytes`, and verify `exec_tx_result_proof` against the
   successor `header.last_results_hash`. Require success and matching gas.
   The successor carries the results commitment for the execution block;
   this proof layout does not introduce another settlement phase.
4. Verify `execution_commit` for `execution_header` and `commit` for the
   successor `header` under authenticated validator history and chain ID.
   Bind successor height, previous block and last-commit hash to the
   execution block and commit. Match the validator-set commitment and
   require more than two thirds of voting power. Refuse
   `commit_at_most_two_thirds`.
5. Decode the `payExact` calldata and bind payment reference, source,
   destination, asset and amount to the authenticated `PaymentExact` log.

The fixture validator set is test evidence. Matching its hash to a header
and verifying its signatures proves consistency with that set; production
callers must establish why that set is trusted at that height.

## Reference verification command

From the repository root, the stored node-vector tests are:

```bash
go test -count=1 ./daemon/committed-fixture/ -run '^(TestVectorV1ReceiptedPositiveVerifies|TestVectorV2ReceiptedRevertVerifies|TestVectorV3EveryRefusalIsRefused|TestVectorV4CommittedVerifies)$'
```

This command checks stored receipt evidence, named refusals and V4 block
proofs. It does not regenerate fixtures, exercise every producer path, prove
Programs-call coverage or establish live public-node readiness. For the SDK's
adapted vectors and verification entry points, see the
[platform conformance guide](../../sdk/platform/conformance/README.md).

[Human owner registration](HumanOwnerRegistration.md) · [Home](Home.md)
