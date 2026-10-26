# x402 v2 Conformance Vectors

The first-party x402 suite: every vector is a wire document of the x402 v2
message it names, together with whether the model must accept it.

## Files

- `payment-required.json` — `PaymentRequired` responses
- `payment-payload.json` — `PaymentPayload` requests
- `settlement-response.json` — `SettlementResponse` results
- `sequenced-settlement.json` — sequenced `PAYMENT-RESPONSE` bodies for
  receipted, submitted and committed-without-proof settlements

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

Every sequenced vector copies `extensions.layerx.receiptDigest` verbatim from
the `receiptDigest` of case `V1-layerxledger-pay-exact` in
`sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`. That fixture was
written by `rpc/sequencer_receipt_vectors_test.go` with
`go test -count=1 ./rpc/ -run TestSequencerReceiptVectors -update`. The
receipted and submitted cases also copy its `receiptJson.txHash` as
`transaction` and its `signedReceipt` as `extensions.layerx.signedReceipt`.
The committed-without-proof case has `success: true`, no `errorReason`, an
empty `transaction` and only `receiptDigest` in `extensions.layerx`.
`SettlementResponse::validate_wire` refuses that body with `EvidenceMissing`
on `/transaction`; it does not inspect the receipt extensions or establish
commitment.

## Conformance

`services/interop/crates/layerx-x402/tests/vectors.rs` reads these files and runs every
record through the production `PaymentRequired`, `PaymentPayload` and
`SettlementResponse` types, so the suite the deployment pins is the suite the
tests exercise. The vectors target the x402 v2 specification vendored at
`services/interop/specs/vendor/x402`.

`services/interop/crates/layerx-x402/tests/sequenced_vectors.rs` compares every
sequenced case with the recorded payExact receipt and asserts its declared
verdict through `SettlementResponse::validate_wire`. These checks exercise
wire validation; they do not authenticate the receipt or verify a chain proof.

`services/interop/deploy/gateway/render.py` derives the suite identifier, the vector
count and the SHA-256 the gateway configuration declares from these files.

## LayerX Profile

Vectors on a `layerx:*` network follow the LayerX profile the buyer and seller
middleware speak: `extra.layerx` carries the `commitment`, the
`agent:<did>:main` `account` the payer quotes against, and the `CurrencyCode`
that quote is priced in, and `payTo` is the account id derived from that
reference under `SHA256("LXP/v1/account-id\0" || name)` or
`SHA256("LX:ACCOUNT:v1" || u32be(len) || name)`. The suite therefore holds
refusals for an offer that omits the block, one whose `payTo` is another
account's identifier, and one whose currency is not a currency code, plus an
accepted offer outside `layerx:*` that carries no such block. The rule lives in
`PaymentRequirements::validate` (`services/interop/crates/layerx-x402/src/model.rs`), so
it holds for every message that embeds requirements.
