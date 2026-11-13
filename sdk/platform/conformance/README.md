# SDK conformance suite

Shared receipt vectors for the Paxeer X Network SDKs. Receipt checks run through
the Go SDK's sequencer, committed and portable receipt verifiers.

The JVM catalogue follows the retained transaction entries for `program.activity`
and `program.receipt` in [`operations.json`](operations.json), group `O32`: both use
`eth_getTransactionByHash`, with an empty destination and no contract calls.
`GeneratedCatalogueProgramsTest.programOperationsNameLayerxprogramsCalls` checks
the generated transports against that record;
`programReceiptTracesRetainedTransactionThroughHttp` exercises receipt lookup
through the real JVM HTTP transport using the retained native Programs receipt.

| Fixture | Cases | Source |
| --- | --- | --- |
| [`vector-v1-receipted-positive.json`](fixtures/vector-v1-receipted-positive.json) | Five successful receipted transactions, including native Programs. | [`sequencer-receipt-v1.json`](fixtures/sequencer-receipt-v1.json), V1. |
| [`vector-v2-receipted-revert.json`](fixtures/vector-v2-receipted-revert.json) | The receipted LayerXLedger revert with authenticated `LXP_INSUFFICIENT_BALANCE` reason, and `V2-pax-transfer-insufficient-funds`, a funded PAX sender refused before execution. | [`sequencer-receipt-v1.json`](fixtures/sequencer-receipt-v1.json), V2. |
| [`vector-v3-receipt-refusals.json`](fixtures/vector-v3-receipt-refusals.json) | All six V3 refusals: key, height range, transaction hash, sequence hash, logs and canonical encoding. | [`sequencer-receipt-v1.json`](fixtures/sequencer-receipt-v1.json), V3. |
| [`vector-v4-committed.json`](fixtures/vector-v4-committed.json) | Recorded payExact receipt, execution header, transaction/result/batch inclusion proofs, execution commit, successor header and commit, validators and insufficient-power successor commit. | [`daemon/committed-fixture/vectors/vector-v4-committed.json`](../../../daemon/committed-fixture/vectors/vector-v4-committed.json), `settlement`. |
| [`vector-v5-portable.json`](fixtures/vector-v5-portable.json) | One receipted and one committed `layerx-receipt-proof-v2` bundle, stored as compact wire documents for exact re-export comparison. | V1 `V1-pax-transfer` and V4 `settlement`; committed recording: [`daemon/committed-fixture/vectors/vector-v4-committed.json`](../../../daemon/committed-fixture/vectors/vector-v4-committed.json). |
| [`vector-v6-sponsored-bind.json`](fixtures/vector-v6-sponsored-bind.json) | Sponsor-paid type-4 `executeSponsored` with a user-signed delegation and executed `bindLayerX`; independent type-2 transfer refusal. Includes raw transactions, signed receipts, genesis pins, measured fees and delegation code readback. | [`daemon/committed-fixture/vectors/vector-v6-sponsored-bind.json`](../../../daemon/committed-fixture/vectors/vector-v6-sponsored-bind.json), recorded on the private chain with EVM chain id 125. |
| [`vector-v7-named-payee.json`](fixtures/vector-v7-named-payee.json) | Committed payExact into the named payee asset sub-account, payer/payee DIDs, main and asset account names and native identifiers, asset, amount and reference; V4 inclusion, Comet evidence and refusals. | [`daemon/committed-fixture/vectors/vector-v7-named-payee.json`](../../../daemon/committed-fixture/vectors/vector-v7-named-payee.json), recorded through the V4 production ledger path on local EVM chain id 125. |

The V1–V3 files preserve the receipt bytes, pinned keys, predecessor hashes and
expectations from [sequencer-receipt-v1.json](fixtures/sequencer-receipt-v1.json).
The V2 PAX-transfer refusal has failed status, empty return data and logs, and
`vmError` equal to `insufficient balance for transfer`. The sequencer layout's
result code `1` denotes failure. The retained answer is at the `receipted` rung.

V4 converts the daemon's encodings to the Go verifier's exported types. Byte
slices use base64 and fixed hashes use byte arrays, as in Go's JSON encoding.
The signed receipt is unchanged; `receiptJson` is its receipted view with the RPC
block metadata removed. V5 uses hexadecimal key height bounds and includes the
committed evidence fields only for the committed bundle. The committed pin is
limited to height 10, matching the daemon source authorization. Verification uses
the pins and predecessor from V1/V4, independently of the key claimed in the bundle.

The V4 recording uses EIP-1559 type-2 transactions on EVM chain ID `125`.
Fixture keys and consensus checkpoints identify the fixture chain. Applications
must obtain their production trust anchors independently.

Run `python3 sdk/platform/conformance/fixtures/refresh_receipt_vectors.py` from the
repository root to copy the retained sequencer producer's V1–V3 cases byte for
byte, then refresh V4, V5 and the Go SDK's fixed external checkpoint at
`sdk/platform/go/testdata/repair_m1384/checkpoint.json`. The checkpoint is captured
from the producer's `execution_commit`, with its source digest recorded separately
from the portable bundle. Verification reads the fixed checkpoint without deriving
trust from the document being verified.

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

Run the Go vector tests from the repository root:

```bash
go -C sdk/platform/go test -count=1 . -run '^(TestVectorFilesV1ToV3MatchSequencerReceiptFixture|TestVectorV4CommittedVerifiesAndRefuses|TestVectorV5PortableRoundTrips)$'
```

The tests compare every V1–V3 case with its source, assert typed refusal checks,
verify committed evidence and insufficient power, and compare each published
portable document with both its re-export and a fresh export of its source.
The committed V5 case also checks that its evidence matches V4 and calls
`VerifyCommittedReceipt` directly with the execution header and its own commit.

V6 retains the recorder's RPC encodings. `positive` and `refusal` each carry
`rawTransaction`, `receiptJson`, `signedReceipt`, `receiptDigest`,
`sponsorPaidFee`, before/after native balances and `delegationDesignator`.
The transfer refusal is a valid successful receipt, but has type 2, no
authorization list, no sponsored call and no `Sponsored` log. Existing
delegation code alone does not prove sponsored execution. `pinnedKey` comes
from the recording genesis, independently of the receipt's `signingKey`.
`delegateDeployment` and `delegateCode` identify the real wallet delegate;
`bindCalldata`, `didPublicKey` and `boundDid` retain the executed DID binding.

The V6 test exercises sponsored binding with an in-process node and checks the
retained fixture. Run it from the repository root:

```bash
go test -count=1 ./daemon/committed-fixture/ -run '^TestVectorV6SponsoredBindVerifies$' && cmp daemon/committed-fixture/vectors/vector-v6-sponsored-bind.json sdk/platform/conformance/fixtures/vector-v6-sponsored-bind.json
```

V7 retains the recorder's encodings byte for byte. Its `parties` object carries
`payer_did`, `payee_did`, `payee_main_account` and `payee_asset_account` (each
account has `name` and `identifier`), `asset`, decimal-string `amount` and
`reference`. DIDs use `did:layerx:<hex>`; the asset account name ends with the
64 lowercase asset hex digits without `0x`. Both account identifiers use the
native `LX:ACCOUNT:v1` domain and four-byte big-endian name length. The payment
settles into the asset account. The main account name identifies the payee in
an offer's `extra.layerx.account`. These fields are consumed for the
buyer human-evidence rung. Buyer human-evidence verification binds the
named payee to the payment's asset account; the committed payment checks below
verify the payer, payee, asset, amount and reference.

To record V7 from the repository root, set `PAXD_BIN` to the path of your built
`paxd` executable and run the recorder:

```bash
go run ./daemon/committed-fixture record --paxd "$PAXD_BIN" --out daemon/committed-fixture/vectors --vector vector-v7-named-payee
cp daemon/committed-fixture/vectors/vector-v7-named-payee.json sdk/platform/conformance/fixtures/vector-v7-named-payee.json
```

The V7 test verifies committed artifacts from the retained fixture. It runs every
V4 committed evidence and refusal check, then verifies
that calldata and authenticated `PaymentExact` logs bind the named payee asset
account, payer asset account, asset, amount and reference:

```bash
go test -count=1 ./daemon/committed-fixture/ -run '^TestVectorV7NamedPayeeVerifies$' && cmp daemon/committed-fixture/vectors/vector-v7-named-payee.json sdk/platform/conformance/fixtures/vector-v7-named-payee.json
```
