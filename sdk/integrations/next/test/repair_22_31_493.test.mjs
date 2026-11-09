import assert from "node:assert/strict";
import { createServer } from "node:http";
import test from "node:test";
import { decodePaymentRequiredHeader, decodeSettlementHeader } from "@sidiora/layerx-seller-middleware";
import * as next from "../dist/index.js";
import {
  MemoryFulfillments, assertDigestTamperingRefused, buyerEvidence, hex, listen, pin, requirements, resource, vector,
} from "./receipt-digest-support.mjs";

function mount(context) {
  const fulfillments = new MemoryFulfillments();
  const releases = [];
  const mounted = next.mountLayerX({
    environment: {
      LAYERX_PRINCIPAL: "merchant", LAYERX_PROTECTED_PATH: "/paid",
      LAYERX_RESOURCE_URL: "https://merchant.example/paid", LAYERX_PROTOCOL_VERSION: "3",
      LAYERX_X402_SCHEME: "exact", LAYERX_X402_NETWORK: requirements.network,
      LAYERX_PRICE: requirements.amount, LAYERX_ASSET: requirements.asset, LAYERX_PAY_TO: requirements.payTo,
      LAYERX_PAYMENT_TIMEOUT_SECONDS: "120", LAYERX_PAYMENT_PURPOSE_HASH: requirements.extra.layerx.purposeHash,
      LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([{
        keyId: hex(pin.keyId), publicKey: hex(pin.publicKey),
        firstHeight: pin.first_height.toString(), lastHeight: pin.last_height.toString(),
      }]),
      LAYERX_WEBHOOK_PATH: "/webhook",
      LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ sequencer: Buffer.from(pin.publicKey).toString("base64") }),
    },
    fulfillments, deliveries: new next.SingleProcessWebhookDeliveryStore(),
    resources: { async release(request, key) { releases.push(key); return resource; } },
    events: { async handle() { throw new Error("unexpected webhook"); } },
  });
  context.after(() => mounted.destroy());
  assert.ok(mounted.seller instanceof next.SellerMiddleware);
  return { mounted, fulfillments, releases };
}

async function nextReleasesRpcProducedReceiptSignatureDigest(context) {
  const { mounted, fulfillments, releases } = mount(context);
  const errors = [];
  const server = createServer(async (request, response) => {
    try {
      const result = await mounted.resource.GET(new Request(`http://${request.headers.host}${request.url}`, {
        method: request.method, headers: request.headers,
      }));
      response.writeHead(result.status, Object.fromEntries(result.headers));
      response.end(Buffer.from(await result.arrayBuffer()));
    } catch (error) {
      errors.push(error);
      response.writeHead(500).end();
    }
  });
  const url = await listen(context, server);
  const challenge = await fetch(url);
  assert.equal(challenge.status, 402);
  assert.equal(releases.length, 0);
  const required = decodePaymentRequiredHeader(challenge.headers.get(next.PAYMENT_REQUIRED_HEADER));
  await challenge.body.cancel();
  const { header, proof, verification } = await buyerEvidence(required);
  for (let replay = 0; replay < 2; replay += 1) {
    const response = await fetch(url, { headers: { [next.PAYMENT_SIGNATURE_HEADER]: header } });
    assert.equal(response.status, 200);
    assert.equal(await response.text(), resource.body);
    const settlement = decodeSettlementHeader(response.headers.get(next.PAYMENT_RESPONSE_HEADER));
    assert.equal(settlement.success, true);
    assert.equal(settlement.transaction, vector.receiptJson.txHash);
    assert.equal(settlement.extensions.layerx.receiptDigest, hex(verification.receiptDigest));
    assert.deepEqual(settlement.extensions.layerx, proof);
    assert.equal(releases.length, 1);
  }
  assert.equal(fulfillments.records.size, 1);
  assert.deepEqual(errors, []);
}

async function nextRefusesTamperedReleasedSettlementDigest(context) {
  const { mounted } = mount(context);
  await assertDigestTamperingRefused(next, mounted.seller, mounted.config.paymentRequired);
}

test(nextReleasesRpcProducedReceiptSignatureDigest);
test(nextRefusesTamperedReleasedSettlementDigest);
