export const REPAIR_TASK_RECORD = Object.freeze({
  task: "22.33.612",
  findings: {
    "003": ["daemonProviderConsumesCurrentProducerResponse", "daemonProviderBindsCurrentReceiptEvidence",
      "daemonProviderVerifiesSignedSequencerReceipt", "daemonProviderRefusesObsoleteStatusFields",
      "daemonServiceProviderWiresCurrentReceiptEvidence"],
    "004": ["paymentToolVerifiesSignedSequencerReceipt", "paymentToolVerifiesCurrentProducerReceipt",
      "serviceClientReadsSignedReceiptFields", "serviceTransportRefusesObsoleteStatusFields",
      "trackingToolVerifiesCurrentProducerReceipt", "trackingToolRefusesUntrustedCurrentProducerReceipt",
      "trackingToolVerifiesExecutedReceiptAndRefusesSignedRevert"],
  },
  baseline: "Attempt 6 at 34715afd945e5c82272ac0ebf44a186821994734 reports 17 of 20 cases passing; paymentToolVerifiesSignedSequencerReceipt fails with verification-failure and the continuation ruling identifies three payment expectation failures. The unchanged tree was not rerun, following the explicit implementation-first gate order.",
  wiring: [
    "services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs — daemon domain: refresh MIDDLEWARE_SUBMISSION_TRANSCRIPT through receipt_value; its recorded authorised_batch.asset differs from the signed protocol fee asset and its receipt lacks current metadata.",
    "services/platform/middleware/agent/src/daemon.ts — middleware domain: decodeDaemonSubmission must accept current receipt_value metadata; this task feeds the recorded legacy observation to its unchanged portable proof producer and the full current receipt envelope to DaemonReceiptResolver.",
    "services/platform/middleware/seller/src/index.ts — middleware domain: reconcile the signing-digest evidence contract with the agent producer Keccak identifier; the agent tool validates the producer identifier and passes the authenticated signing digest at the seller boundary.",
  ],
  evidence: "The private hf-build/hf-done ledger supplies revision, command, exit code and log; no closure is claimed without a passing hf-done.",
});

await import("./daemon-providers-receipts.test.mjs");
await import("./tools-receipts.test.mjs");
