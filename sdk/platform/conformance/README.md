# SDK conformance suite

Shared receipt vectors for the Paxeer X Network SDKs. Receipt checks run through
the Go SDK's sequencer, committed and portable receipt verifiers.

| Fixture | Cases | Source |
| --- | --- | --- |
| [`vector-v1-receipted-positive.json`](fixtures/vector-v1-receipted-positive.json) | Four successful receipted transactions, preserving every V1 source case. | [`sequencer-receipt-v1.json`](fixtures/sequencer-receipt-v1.json), V1; daemon recording: [`daemon/committed-fixture/vectors/vector-v1-receipted-positive.json`](../../../daemon/committed-fixture/vectors/vector-v1-receipted-positive.json). |
| [`vector-v2-receipted-revert.json`](fixtures/vector-v2-receipted-revert.json) | The receipted LayerXLedger revert and authenticated `LXP_INSUFFICIENT_BALANCE` reason. | [`sequencer-receipt-v1.json`](fixtures/sequencer-receipt-v1.json), V2; daemon export: [`daemon/committed-fixture/vectors/vector-v2-receipted-revert.json`](../../../daemon/committed-fixture/vectors/vector-v2-receipted-revert.json). |
| [`vector-v3-receipt-refusals.json`](fixtures/vector-v3-receipt-refusals.json) | All six V3 refusals: key, height range, transaction hash, sequence hash, logs and canonical encoding. | [`sequencer-receipt-v1.json`](fixtures/sequencer-receipt-v1.json), V3; daemon export: [`daemon/committed-fixture/vectors/vector-v3-receipt-refusals.json`](../../../daemon/committed-fixture/vectors/vector-v3-receipt-refusals.json). |
| [`vector-v4-committed.json`](fixtures/vector-v4-committed.json) | Recorded payExact receipt, execution header, transaction/result/batch inclusion proofs, execution commit, successor header and commit, validators and insufficient-power successor commit. | [`daemon/committed-fixture/vectors/vector-v4-committed.json`](../../../daemon/committed-fixture/vectors/vector-v4-committed.json), `settlement`. |
| [`vector-v5-portable.json`](fixtures/vector-v5-portable.json) | One receipted and one committed `layerx-receipt-proof-v2` bundle, stored as compact wire documents for exact re-export comparison. | V1 `V1-pax-transfer` and V4 `settlement`; committed recording: [`daemon/committed-fixture/vectors/vector-v4-committed.json`](../../../daemon/committed-fixture/vectors/vector-v4-committed.json). |

V1–V3 copy the sequencer fixture cases without changing receipt bytes, pinned
keys, predecessor hashes or expectations. The daemon's V1 is a separate contract
creation recording; its V3 uses a re-signed receipt. Those exports are linked for
provenance and do not replace the cases required here.

V4 converts the daemon's encodings to the Go verifier's exported types. Byte
slices use base64 and fixed hashes use byte arrays, as in Go's JSON encoding.
The signed receipt is unchanged; `receiptJson` is its receipted view with the RPC
block metadata removed. V5 uses hexadecimal key height bounds and includes the
committed evidence fields only for the committed bundle. The committed pin is
limited to height 10, matching the daemon source authorization. Verification uses
the pins and predecessor from V1/V4, independently of the key claimed in the bundle.

V4 pairs `settlement.execution_header` with `settlement.execution_commit` and
`settlement.header` with `settlement.commit`. Its refusal replaces only the
successor commit with `settlement.commit_at_most_two_thirds`. Transaction and
batch proofs target the execution header's `data_hash`; the result proof targets
the successor header's `last_results_hash`.

The transaction inclusion proof covers the Cosmos block transaction from
`settlement.transaction_proof.data`. Both V4 and the committed V5 bundle retain
those bytes in `ReceiptInclusion.Transaction`, separately from `rawTransaction`,
which holds the signed EVM transaction. The committed verifier extracts and
binds the wrapped EVM transaction to the receipt's activity id.

The portable verifier binds `rawTransaction` to the EVM transaction wrapped in
`receiptInclusion.transaction`.

The focused Go gate is:

```bash
go -C sdk/platform/go test -count=1 . -run '^(TestVectorFilesV1ToV3MatchSequencerReceiptFixture|TestVectorV4CommittedVerifiesAndRefuses|TestVectorV5PortableRoundTrips)$'
```

The tests compare every V1–V3 case with its source, assert typed refusal checks,
verify committed evidence and insufficient power, and compare each published
portable document with both its re-export and a fresh export of its source.
The committed V5 case also checks that its evidence matches V4 and calls
`VerifyCommittedReceipt` directly with the execution header and its own commit.
