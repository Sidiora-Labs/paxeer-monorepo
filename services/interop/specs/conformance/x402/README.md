# x402 v2 Conformance Vectors

The first-party x402 suite: every vector is a wire document of the x402 v2
message it names, together with whether the model must accept it. Paxeer
mainnet-beta is one BFT network with its kernel integrated into `paxd`, EVM
chain id 125 and CAIP-2 identifier `eip155:125`. A receipt is final once its
block is committed; wire acceptance alone does not prove that event.

## Files

- `payment-required.json` — `PaymentRequired` responses
- `payment-payload.json` — `PaymentPayload` requests
- `settlement-response.json` — `SettlementResponse` results
- `sequenced-settlement.json` — sequenced `PAYMENT-RESPONSE` bodies for
  receipted, submitted, committed-without-proof and committed settlements

## Vector Format

Each file holds a JSON array of records:

- `name`: the case the vector covers
- `valid`: whether `parse` and `validate` must both succeed
- `document`: the wire document, exactly as it travels

`sequenced-settlement.json` also declares `expected.verdict` (`accept`,
`pending` or `refuse`) and `expected.field`, a JSON Pointer to the field that
decides the verdict: `/success`, `/errorReason` or `/transaction`. Pending
responses are valid wire documents with `success: false` and
`errorReason: "settlement_pending"`.

The `receipted_settlement`, `submitted_settlement` and
`committed_without_proof` vectors carry `extensions.layerx.receiptDigest`
verbatim from the `receiptDigest` of case `V1-layerxledger-pay-exact` in
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`. The producer is
`rpc/sequencer_receipt_vectors_test.go::TestSequencerReceiptVectors`. The
receipted and submitted cases also copy its `receiptJson.txHash` as
`transaction` and its `signedReceipt` as `extensions.layerx.signedReceipt`.
The committed-without-proof case has `success: true`, no `errorReason`, an
empty `transaction` and only `receiptDigest` in `extensions.layerx`.
`SettlementResponse::validate_wire` refuses that body with `EvidenceMissing`
on `/transaction`; it does not inspect the receipt extensions or establish
commitment.

`committed_settlement` copies the `committed-payExact` case in
`services/interop/specs/conformance/committed/vector-v4-committed.json`, produced
by `daemon/committed-fixture/vector_v4.go::recordV4`. It uses that case's
`receiptJson.txHash`, signed receipt, payer account and amount (3 atomic units).
Its LayerX extension carries the canonical receipt digest,
`verificationLevel: "committed"`, `blockNumber: "0xa"` and the recorded
`blockHash`. The expected verdict is `accept` on `/success`.

## Conformance

`services/interop/crates/layerx-x402/tests/vectors.rs` reads the three
message-model files and runs each record through the production
`PaymentRequired`, `PaymentPayload` and `SettlementResponse` types. The vectors target the x402 v2 specification vendored at
`services/interop/specs/vendor/x402`.

`services/interop/crates/layerx-x402/tests/sequenced_vectors.rs` compares every
sequenced case with its source payExact receipt and asserts its declared
verdict through `SettlementResponse::validate_wire`. These checks exercise
wire validation. `x402_committed_vector_settles_as_committed` additionally
authenticates the committed vector's original receipt and logs and verifies its
recorded consensus and batch inclusion proof. It then binds the recorded payment
to the test facilitator request's identity and account profile and signs that
receipt under a test authority, preserving the activity, sequence chain and
consensus proof. The production facilitator and gateway settle that evidence;
the test requires the committed level and both block fields to match the vector.

[The gateway renderer](../../../deploy/gateway/README.md#derived-by-renderpy-never-a-deployment-input)
derives the suite identifier, vector count and SHA-256 from these files.
`vectors.rs` reads the three message-model files; `sequenced_vectors.rs` reads
`sequenced-settlement.json`. Rendering a pin does not execute either harness.
The [websearch sidecar](../../../crates/x-websearch/README.md) consumes the
exact-payment profile and authenticated node receipts.

## LayerX Profile

`PaymentRequirements::is_layerx_network` selects this profile only for
`eip155:125` (`LAYERX_NETWORK` in
[`model.rs`](../../../crates/layerx-x402/src/model.rs)).

`extra.layerx` supplies the quote's account and currency terms:

- `account` is `agent:<did>:main`, or `agent:<did>:asset:<asset>` where the
  asset suffix is the offer's lowercase 64-digit asset id without `0x`.
- `currency` is a `CurrencyCode` represented by 1–16 uppercase ASCII letters
  or digits.
- `payTo` is the 32-byte account id derived from that account reference by
  `SHA256("LXP/v1/account-id\0" || name)` or
  `SHA256("LX:ACCOUNT:v1" || u32be(len) || name)`. `name` is the account's
  UTF-8 bytes and `len` is their byte length. The validator accepts either
  derivation for compatibility with the two account-id protocols.

`PaymentRequirements::validate` requires those terms on `eip155:125`, checks
that a per-asset account names the offered asset, and binds `payTo` to the
account. The vectors cover a missing profile, another account's `payTo`, an
invalid currency and an offer on another network without this profile. On a
non-Paxeer network the validator rejects a supplied LayerX profile.

Some vectors carry an `extra.layerx.commitment` extension field.
It is not a profile requirement or a finality selector:
`LayerXTerms` contains only `account` and `currency`, and the profile parser
reads only those fields. Settlement evidence is evaluated separately. The
`extensions.layerx.verificationLevel` value in a settlement response describes
the evidence level; authenticating a receipt and proving its block's commitment
require the verification paths described above, not an offer field or a
successful wire parse.
