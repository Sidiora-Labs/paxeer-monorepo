import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { keccak256, recoverEip1559Sender } from "@sidiora/layerx-sdk";
import {
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  decodePaymentPayloadHeader,
  decodePaymentRequiredHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
} from "@sidiora/layerx-seller-middleware";
import {
  ConformanceSequencer,
  buildSignedReceipt,
  buyerPayload,
  fixedBytes,
} from "../../../../../services/platform/middleware/conformance/src/receipts.ts";
import { FileFulfillmentRepository, createExampleApplication } from "../index.mjs";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");

async function repair_22_31_414_paid_route_reopens_and_replays_receipt(t) {
  const directory = await mkdtemp(join(tmpdir(), "express-repair-22-31-414-"));
  const resourceFile = join(directory, "resource.json");
  const fulfillmentDirectory = join(directory, "fulfillments");
  const resourceBody = await readFile(new URL("../resource.json", import.meta.url), "utf8");
  await writeFile(resourceFile, resourceBody);
  const sequencer = await ConformanceSequencer.generate();
  const key = sequencer.receiptKey();
  const asset = fixedBytes(0x11);
  const payTo = fixedBytes(0x22);
  const environment = {
    LAYERX_PRINCIPAL: "example-merchant",
    LAYERX_PROTECTED_PATH: "/paid",
    LAYERX_RESOURCE_URL: "https://merchant.example/paid",
    LAYERX_RESOURCE_FILE: resourceFile,
    LAYERX_FULFILLMENT_DIR: fulfillmentDirectory,
    LAYERX_PROTOCOL_VERSION: "2",
    LAYERX_X402_SCHEME: "exact",
    LAYERX_X402_NETWORK: "eip155:125",
    LAYERX_PRICE: "25",
    LAYERX_ASSET: hex(asset),
    LAYERX_PAY_TO: hex(payTo),
    LAYERX_PAYMENT_TIMEOUT_SECONDS: "120",
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([{
      keyId: hex(key.keyId),
      publicKey: hex(key.publicKey),
      firstHeight: key.firstHeight.toString(),
      lastHeight: key.lastHeight.toString(),
    }]),
    LAYERX_WEBHOOK_PATH: "/webhook",
    LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({
      sequencer: Buffer.from(key.publicKey).toString("base64"),
    }),
  };
  let running;
  async function close() {
    if (running === undefined) return;
    const { mount, server } = running;
    running = undefined;
    mount.destroy();
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
  }
  t.after(async () => {
    await close();
    await rm(directory, { recursive: true, force: true });
  });
  async function start() {
    const { app, mount } = await createExampleApplication(environment);
    const server = app.listen(0, "127.0.0.1");
    const requests = [];
    server.prependListener("request", (request) => {
      requests.push({ method: request.method, path: request.url,
        payment: request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()] });
    });
    running = { mount, server, requests };
    await once(server, "listening");
    return `http://127.0.0.1:${server.address().port}/paid`;
  }
  let url = await start();
  const challenge = await fetch(url);
  assert.equal(challenge.status, 402);
  const offer = decodePaymentRequiredHeader(challenge.headers.get(PAYMENT_REQUIRED_HEADER));
  assert.deepEqual(await challenge.json(), offer);
  assert.deepEqual(offer, running.mount.config.paymentRequired);
  assert.equal(offer.x402Version, 2);
  assert.equal(offer.accepts[0].network, "eip155:125");
  assert.equal(offer.accepts[0].extra.layerx.commitment, "receipted");
  const receipt = await buildSignedReceipt(sequencer, {
    asset, payTo, amount: 25n, from: fixedBytes(0x33),
    purposeHash: offer.accepts[0].extra.layerx.purposeHash,
    batchId: fixedBytes(0x44), previousStateRoot: fixedBytes(0x55),
    resultingStateRoot: fixedBytes(0x66),
  });
  const payload = buyerPayload({ requirements: offer.accepts[0], paymentRequired: offer },
    receipt, "repair-22-31-414");
  const header = encodePaymentPayloadHeader(payload);
  assert.deepEqual(decodePaymentPayloadHeader(header), payload);
  const repository = new FileFulfillmentRepository(fulfillmentDirectory);
  assert.equal(await repository.get("0".repeat(64)), undefined);

  const tampered = bytes(receipt.signedReceipt);
  tampered[tampered.length - 1] ^= 1;
  const invalid = encodePaymentPayloadHeader({ ...payload, payload: {
    ...payload.payload, signedReceipt: hex(tampered), receiptDigest: hex(keccak256(tampered)),
  } });
  const refused = await fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: invalid } });
  assert.equal(refused.status, 402);
  assert.deepEqual(await refused.json(), { error: "verification-failure" });
  assert.equal(await repository.get("0".repeat(64)), undefined);

  const paid = await fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header } });
  assert.equal(paid.status, 200);
  assert.match(paid.headers.get("content-type"), /^application\/json\b/u);
  assert.equal(await paid.text(), resourceBody);
  assert.deepEqual(running.requests.at(-1), { method: "GET", path: "/paid", payment: header });
  const settlementHeader = paid.headers.get(PAYMENT_RESPONSE_HEADER);
  const settlement = decodeSettlementHeader(settlementHeader);
  assert.deepEqual(settlement, {
    success: true,
    payer: recoverEip1559Sender(bytes(receipt.rawTransaction)).slice(2).toLowerCase().padStart(64, "0"),
    transaction: receipt.txHash,
    network: "eip155:125",
    amount: "25",
    extensions: { layerx: payload.payload },
  });
  const files = await readdir(fulfillmentDirectory);
  assert.equal(files.length, 1);
  assert.match(files[0], /^[0-9a-f]{64}\.json$/u);
  const idempotencyKey = files[0].slice(0, -5);
  const recordPath = join(fulfillmentDirectory, files[0]);
  const persisted = await readFile(recordPath, "utf8");
  const stored = JSON.parse(persisted);
  assert.match(stored.requestDigest, /^[0-9a-f]{64}$/u);
  assert.deepEqual(stored, {
    ...receipt, idempotencyKey, requestDigest: stored.requestDigest,
    resource: { contentType: "application/json", body: resourceBody },
  });
  assert.deepEqual(await repository.get(idempotencyKey), stored);
  assert.deepEqual(await repository.fulfill(stored, async () => {
    assert.fail("an existing fulfillment must not release again");
  }), stored);
  await assert.rejects(repository.fulfill({ ...stored, requestDigest: "different-request" }, async () => {
    assert.fail("a conflicting fulfillment must not release");
  }), (error) => error.code === "fulfillment-conflict");

  await close();
  await writeFile(resourceFile, JSON.stringify({ report: "replacement-resource" }));
  url = await start();
  assert.deepEqual(await new FileFulfillmentRepository(fulfillmentDirectory).get(idempotencyKey), stored);
  const replay = await fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header } });
  assert.equal(replay.status, 200);
  assert.equal(await replay.text(), resourceBody);
  assert.equal(replay.headers.get(PAYMENT_RESPONSE_HEADER), settlementHeader);
  assert.deepEqual(decodeSettlementHeader(replay.headers.get(PAYMENT_RESPONSE_HEADER)), settlement);
  assert.deepEqual(running.requests.at(-1), { method: "GET", path: "/paid", payment: header });
  const changed = encodePaymentPayloadHeader({ ...payload, resource: {
    ...payload.resource, description: "different request using the same receipt",
  } });
  const reused = await fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: changed } });
  assert.equal(reused.status, 409);
  assert.deepEqual(decodeSettlementHeader(reused.headers.get(PAYMENT_RESPONSE_HEADER)), {
    success: false, errorReason: "receipt_reuse", transaction: "", network: "eip155:125",
  });
  assert.equal(await reused.text(), "");
  assert.deepEqual(await readdir(fulfillmentDirectory), files);
  assert.equal(await readFile(recordPath, "utf8"), persisted);
}

test(repair_22_31_414_paid_route_reopens_and_replays_receipt);
