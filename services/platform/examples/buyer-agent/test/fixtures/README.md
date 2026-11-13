# Buyer agent fixtures

[`human-transcript.json`](human-transcript.json) contains the Human API quote,
commit and evidence response shapes consumed by
[`payment-fixture.mjs`](../payment-fixture.mjs). The harness uses
`SequencerReceiptClient` with the node-produced `V1-layerxledger-pay-exact`
vector in
[`sequencer-receipt-v1.json`](../../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json).
