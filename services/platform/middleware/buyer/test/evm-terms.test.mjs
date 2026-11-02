import assert from "node:assert/strict";
import test from "node:test";
import { ProductionClient, STATE_COMMITMENT_PROTOCOL_VERSION, SecretBytes } from "@sidiora/layerx-sdk";
import { MiddlewareError, encodePaymentRequiredHeader } from "@sidiora/layerx-seller-middleware";
import { ReceiptAuthorityClient } from "../../../examples/support/runtime.mjs";
import { BuyerMiddleware, LayerXPaymentHttpTransport } from "../dist/index.js";

function buyerConfig(network = "eip155:125") {
  return {
    client: new ProductionClient(new LayerXPaymentHttpTransport({
      baseUrl: "http://127.0.0.1:1/",
      bearerToken: new SecretBytes(new TextEncoder().encode("unused-buyer-test-token")),
    })),
    source: "acct:buyer-evm-terms-test",
    protocolVersion: STATE_COMMITMENT_PROTOCOL_VERSION,
    supported: [{ scheme: "exact", network }],
    authorizedBatches: new ReceiptAuthorityClient("http://127.0.0.1:1/"),
  };
}

function buyerAcceptsReceiptedOfferWithoutResolver() {
  const config = buyerConfig();
  assert.equal(config.commitments, undefined);
  const buyer = new BuyerMiddleware(config);
  const required = {
    x402Version: 2,
    resource: { url: "https://seller.example/paid" },
    accepts: [{
      scheme: "exact",
      network: "eip155:125",
      amount: "25",
      asset: "11".repeat(32),
      payTo: "22".repeat(32),
      maxTimeoutSeconds: 120,
      extra: { layerx: { commitment: "receipted" } },
    }],
  };
  const parsed = buyer.parseOffer(encodePaymentRequiredHeader(required));
  assert.deepEqual(parsed.required, required);
  assert.deepEqual(parsed.accepted, required.accepts[0]);

  const committed = {
    ...required,
    accepts: [{ ...required.accepts[0], extra: { layerx: { commitment: "committed" } } }],
  };
  assert.throws(
    () => buyer.parseOffer(encodePaymentRequiredHeader(committed)),
    (error) => error instanceof MiddlewareError && error.code === "unsupported-payment",
  );
}

function buyerRefusesLayerxSupportedKind() {
  for (const network of ["eip155:1", "eip155:125", "eip155:4294967295"]) {
    const buyer = new BuyerMiddleware(buyerConfig(network));
    assert.equal(buyer.protocolVersion, STATE_COMMITMENT_PROTOCOL_VERSION);
  }
  for (const network of [
    "layerx:beta", "layerx:125", "layerx:", "layerx:eip155:125",
    "eip155:0", "eip155:4294967296", "eip155:99999999999999999999",
    "eip155:-1", "eip155:+125", "eip155:1.25", "eip155:1e2", "eip155:0x7d",
    "eip155:0125", "eip155:", "eip155:125:1", "eip155:125\n", "eip155: 125",
    "other:125", "",
  ]) {
    assert.throws(
      () => new BuyerMiddleware(buyerConfig(network)),
      (error) => error instanceof MiddlewareError && error.code === "unsupported-payment",
      `network ${JSON.stringify(network)} must be refused`,
    );
  }
}

test(buyerAcceptsReceiptedOfferWithoutResolver);
test(buyerRefusesLayerxSupportedKind);
