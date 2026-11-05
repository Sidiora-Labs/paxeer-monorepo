import assert from "node:assert/strict";
import { createServer } from "node:http";
import test from "node:test";
import express from "express";
import { decodePaymentRequiredHeader, decodeSettlementHeader } from "@sidiora/layerx-seller-middleware";
import * as integration from "../dist/index.js";
import {
  MemoryFulfillments, assertDigestTamperingRefused, buyerEvidence, hex, listen, pin, requirements, resource, vector,
} from "../../next/test/receipt-digest-support.mjs";

function mount() {
  const fulfillments = new MemoryFulfillments();
  const releases = [];
  const required = { x402Version: 2, resource: { url: "https://merchant.example/paid" }, accepts: [requirements] };
  const seller = new integration.SellerMiddleware({
    paymentRequired: required, receiptKeys: [pin],
    authority: new integration.ReceiptPayloadAuthority([pin]), fulfillments,
  });
  const app = express();
  app.get("/paid", integration.layerXPaymentGate({
    seller, principal: "merchant", requirements,
    resources: { async release(request, key) { releases.push(key); return resource; } },
  }));
  const errors = [];
  app.use((error, request, response, next) => {
    errors.push(error);
    response.status(500).end();
  });
  return { app, seller, required, fulfillments, releases, errors };
}

async function expressReleasesRpcProducedReceiptSignatureDigest(context) {
  const { app, fulfillments, releases, errors } = mount();
  const url = await listen(context, createServer(app));
  const challenge = await fetch(url);
  assert.equal(challenge.status, 402);
  assert.equal(releases.length, 0);
  const required = decodePaymentRequiredHeader(challenge.headers.get(integration.PAYMENT_REQUIRED_HEADER));
  await challenge.body.cancel();
  const { header, proof, verification } = await buyerEvidence(required);
  for (let replay = 0; replay < 2; replay += 1) {
    const response = await fetch(url, { headers: { [integration.PAYMENT_SIGNATURE_HEADER]: header } });
    assert.equal(response.status, 200);
    assert.equal(await response.text(), resource.body);
    const settlement = decodeSettlementHeader(response.headers.get(integration.PAYMENT_RESPONSE_HEADER));
    assert.equal(settlement.success, true);
    assert.equal(settlement.transaction, vector.receiptJson.txHash);
    assert.equal(settlement.extensions.layerx.receiptDigest, hex(verification.receiptDigest));
    assert.deepEqual(settlement.extensions.layerx, proof);
    assert.equal(releases.length, 1);
  }
  assert.equal(fulfillments.records.size, 1);
  assert.deepEqual(errors, []);
}

async function expressRefusesTamperedReleasedSettlementDigest() {
  const { seller, required } = mount();
  await assertDigestTamperingRefused(integration, seller, required);
}

test(expressReleasesRpcProducedReceiptSignatureDigest);
test(expressRefusesTamperedReleasedSettlementDigest);
