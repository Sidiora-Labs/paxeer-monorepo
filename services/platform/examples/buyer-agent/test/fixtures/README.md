`localhost-key.pem` is a public test-only private key for the committed loopback
certificate (valid 2025–2045). It must never be used by a deployment. The test
explicitly trusts this certificate without disabling TLS verification.

`human-transcript.json` records the Human API golden response shapes with the
documented replay adaptations. Receipt evidence is supplied by the production
`SequencerReceiptClient` from the node-produced `V1-layerxledger-pay-exact`
vector in `sdk/platform/conformance/fixtures/sequencer-receipt-v1.json`.
