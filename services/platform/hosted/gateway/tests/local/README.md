# Local gateway verifier binaries

This standalone Cargo package (`layerx-gateway-local-qualification`) provides
two verifier binaries:

- `gateway-lifecycle-receipt-verify` ([`verifier.rs`](verifier.rs)) reads a
  receipt file, verifies its sequencer signature using
  `LAYERX_TEST_SEQUENCER_KEY_FILE`, and checks its binding to a locally signed
  activity in `LAYERX_LIFECYCLE_MANIFEST` on network 7332. It fetches batch
  evidence over authenticated TLS, verifies the batch-header signature and
  receipt inclusion proof, and checks authorized batch identity and state roots.
- `gateway-lifecycle-sdk-verify` ([`lifecycle_sdk.rs`](lifecycle_sdk.rs)) reads a
  private JSON request file and uses the `layerx-sdk` program lifecycle types
  and authenticated TLS transport to replay a signed deployment refusal. It
  checks the terminal policy refusal and recovers the exact expected receipt.

Each binary accepts its input file path as its sole positional argument.
