import assert from "node:assert/strict";
import test from "node:test";
import { PlatformSdkError } from "@sidiora/layerx-sdk";
import {
  MiddlewareError,
  SellerMiddleware,
  decodePaymentRequiredHeader,
  paymentCommitment,
} from "../dist/index.js";

function paymentCommitmentParsesRungNames() {
  for (const extra of [undefined, null, {}, { other: "terms" }]) {
    assert.equal(paymentCommitment(extra), "receipted");
  }
  for (const commitment of ["receipted", "committed"]) {
    assert.equal(paymentCommitment({ layerx: { commitment } }), commitment);
  }
}

function paymentCommitmentRefusesKernelLevels() {
  for (const commitment of ["executed", "batched", "finalised"]) {
    assert.throws(
      () => paymentCommitment({ layerx: { commitment } }),
      (error) => error instanceof PlatformSdkError && error.code === "verification-failure",
      `kernel commitment ${commitment} must be refused`,
    );
  }
  for (const layerx of [null, [], "committed", {}, { commitment: "unknown" }]) {
    assert.throws(
      () => paymentCommitment({ layerx }),
      (error) => error instanceof PlatformSdkError && error.code === "verification-failure",
    );
  }
}

function sellerRefusesLayerxNetworkIdentifiers() {
  const paymentRequired = {
    x402Version: 2,
    resource: { url: "https://seller.example/paid" },
    accepts: [{
      scheme: "exact",
      network: "eip155:125",
      amount: "25",
      asset: "11".repeat(32),
      payTo: "22".repeat(32),
      maxTimeoutSeconds: 120,
    }],
  };
  const sellerFor = (network) => new SellerMiddleware({
    protocolVersion: 3,
    paymentRequired: {
      ...paymentRequired,
      accepts: [{ ...paymentRequired.accepts[0], network }],
    },
  });

  for (const network of ["eip155:1", "eip155:125", "eip155:4294967295"]) {
    const decision = sellerFor(network).paymentRequired();
    assert.equal(decision.kind, "payment-required");
    assert.equal(decision.status, 402);
    assert.equal(decision.body.accepts[0].network, network);
    assert.equal(decodePaymentRequiredHeader(decision.headers["PAYMENT-REQUIRED"]).accepts[0].network, network);
  }
  for (const network of [
    "layerx:beta", "layerx:125", "layerx:", "layerx:eip155:125",
    "eip155:0", "eip155:4294967296", "eip155:99999999999999999999",
    "eip155:-1", "eip155:+125", "eip155:1.25", "eip155:1e2", "eip155:0x7d",
    "eip155:0125", "eip155:", "eip155:125:1", "eip155:125\n", "eip155: 125",
    "other:125",
  ]) {
    assert.throws(
      () => sellerFor(network),
      (error) => error instanceof MiddlewareError && error.code === "unsupported-payment",
      `network ${JSON.stringify(network)} must be refused`,
    );
  }
}

test(paymentCommitmentParsesRungNames);
test(paymentCommitmentRefusesKernelLevels);
test(sellerRefusesLayerxNetworkIdentifiers);
