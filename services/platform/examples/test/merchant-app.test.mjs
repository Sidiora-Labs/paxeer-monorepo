import "./support/source-loader.mjs";
import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { keccak256 } from "@sidiora/layerx-sdk";
import {
  decodePaymentExactLog,
  decodePaymentRequiredHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
  importPortableNodeReceipt,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";

import { bytes, hex, nodeReceipt, portableReceipt } from "./support/node-receipts.mjs";
import { merchantReceipt } from "./support/merchant-receipt.mjs";
import { SequencerReceiptClient } from "../support/runtime.mjs";

const vector = nodeReceipt("V1-layerxledger-pay-exact");
const log = vector.receiptJson.logs[0];
const paymentFacts = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
const asset = paymentFacts.assetId;
const payee = paymentFacts.to;
const keyId = vector.key.keyId;
const publicKey = vector.key.publicKey;
const pin = vector.key;
const customer = {
  token: "merchant-customer-receipt-test-credential",
  tenant: "merchant-test",
  principal: "merchant-customer",
};

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}`;
}

async function close(server) {
  await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
}

async function startMerchant(t) {
  const root = await mkdtemp(join(tmpdir(), "layerx-merchant-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const webhookKey = generateKeyPairSync("ed25519");
  const webhookPublicKey = webhookKey.publicKey.export({ type: "spki", format: "der" }).subarray(-32);
  const transactions = new Map();
  const requests = [];
  const settlements = [];
  const authority = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      response.setHeader("content-type", "application/json");
      if (request.url === "/settle") {
        assert.equal(request.headers.authorization, "Bearer merchant-test-token");
        settlements.push(message);
        const transaction = message.payload.payload.transaction;
        assert.ok(transactions.has(transaction));
        response.end(JSON.stringify({ state: "settled", transaction }));
        return;
      }
      assert.equal(request.url, "/");
      assert.equal(request.method, "POST");
      assert.deepEqual(Object.keys(message).sort(), ["id", "jsonrpc", "method", "params"]);
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.id, 1);
      assert.equal(message.params.length, 1);
      requests.push(message);
      const proof = transactions.get(message.params[0]);
      assert.ok(proof);
      assert.ok(["eth_getRawTransactionByHash", "pax_sequencerReceipt"].includes(message.method));
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result:
        message.method === "eth_getRawTransactionByHash" ? proof.rawTransaction : {
          ...proof.receiptJson,
        } }));
    } catch (error) {
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  const authorityUrl = await listen(authority);
  t.after(() => close(authority));
  const reservation = createServer();
  const url = await listen(reservation);
  const config = JSON.parse(await readFile(new URL("../merchant-checkout/layerx.example.json", import.meta.url), "utf8"));
  config.environments.emulator = {
    ...config.environments.emulator,
    publicUrl: url,
    port: reservation.address().port,
    rpcUrl: authorityUrl,
    settlementUrl: `${authorityUrl}/settle`,
    receiptKeysEnvironment: "LAYERX_MERCHANT_TEST_RECEIPT_KEYS_JSON",
    customerCredentialsEnvironment: "LAYERX_MERCHANT_TEST_CUSTOMERS_JSON",
  };
  const configPath = join(root, "config.json");
  await writeFile(configPath, JSON.stringify(config));
  const env = {
    ...process.env,
    LAYERX_EXAMPLE_CONFIG: configPath,
    LAYERX_EXAMPLE_STATE_ROOT: root,
    LAYERX_EXAMPLE_ENDPOINT: "",
    LAYERX_EMULATOR_TOKEN: "merchant-test-token",
    LAYERX_EMULATOR_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ merchant: hex(webhookPublicKey) }),
    LAYERX_EMULATOR_ASSET: hex(asset),
    LAYERX_EMULATOR_MERCHANT: hex(payee),
    LAYERX_EMULATOR_MERCHANT_ACCOUNT: "agent:merchant:main",
    LAYERX_EMULATOR_CURRENCY: "USD",
    LAYERX_EMULATOR_PRICE: paymentFacts.amount.toString(),
    LAYERX_MERCHANT_TEST_CUSTOMERS_JSON: JSON.stringify([customer]),
    LAYERX_MERCHANT_TEST_RECEIPT_KEYS_JSON: JSON.stringify([{
      keyId: hex(keyId), publicKey: hex(publicKey), firstHeight: vector.pinnedKey.firstHeight, lastHeight: vector.pinnedKey.lastHeight,
    }]),
  };
  let child;
  async function stop() {
    if (child === undefined || child.exitCode !== null || child.signalCode !== null) return;
    const exited = once(child, "exit");
    child.kill("SIGTERM");
    await exited;
  }
  t.after(stop);
  async function start() {
    child = spawn(process.execPath, ["--import", fileURLToPath(new URL("./support/source-loader.mjs", import.meta.url)), fileURLToPath(new URL("../merchant-checkout/index.mjs", import.meta.url)),
      "--environment", "emulator"], { env, stdio: ["ignore", "pipe", "pipe"] });
    await new Promise((resolve, reject) => {
      let output = "";
      const timer = setTimeout(() => reject(new Error(`merchant_start_timeout: ${output}`)), 10000);
      const onExit = (code) => { clearTimeout(timer); reject(new Error(`merchant_exit_${code}: ${output}`)); };
      child.once("exit", onExit);
      child.once("error", (error) => { clearTimeout(timer); reject(error); });
      child.stderr.on("data", (chunk) => { output += chunk; });
      child.stdout.on("data", (chunk) => {
        output += chunk;
        if (output.includes('"listening":')) {
          clearTimeout(timer);
          child.removeListener("exit", onExit);
          resolve();
        }
      });
    });
  }
  await close(reservation);
  await start();
  async function checkout(key, payment) {
    return fetch(`${url}/checkout`, {
      method: "POST",
      headers: { "content-type": "application/json", authorization: `Bearer ${customer.token}`,
        ...(payment === undefined ? {} : { "payment-signature": payment }) },
      body: JSON.stringify({ principal: "merchant-customer", checkout_key: key, lines: [{ sku: "metered-report", quantity: 1 }] }),
    });
  }
  async function receiptFor(offer) {
    assert.equal(offer.accepts[0].amount, paymentFacts.amount.toString());
    assert.equal(offer.accepts[0].asset, hex(asset));
    assert.equal(offer.accepts[0].payTo, hex(payee));
    const recording = await merchantReceipt(offer);
    const recorded = portableReceipt(recording);
    const transaction = recording.txHash;
    transactions.set(transaction, recorded);
    const proof = importPortableNodeReceipt(await new SequencerReceiptClient(authorityUrl, [pin]).resolve(recording.raw));
    assert.deepEqual(proof, recorded);
    assert.notEqual(proof.receiptJson.logs[0].topics[1], vector.receiptJson.logs[0].topics[1]);
    await assert.rejects(verifyPaymentReceipt(proof, {
      ...offer.accepts[0], extra: { ...offer.accepts[0].extra, layerx: {
        ...offer.accepts[0].extra.layerx, purposeHash: vector.receiptJson.logs[0].topics[1].slice(2),
      } },
    }, [pin]), { name: "MiddlewareError", code: "verification-failure" });
    return { proof, transaction };
  }
  async function webhook(order, proof, transaction, id, corruptSignature = false) {
    const body = Buffer.from(JSON.stringify({
      order_id: order.orderId, request_digest: order.requestDigest,
      receipt_digest: proof.receiptDigest.slice(2), receipt_ref: hex(keccak256(bytes(proof.rawTransaction))),
      transaction, verification: "receipted",
    }));
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = sign(null, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]), webhookKey.privateKey);
    if (corruptSignature) signature[0] ^= 1;
    return fetch(`${url}/webhooks/settlement`, {
      method: "POST",
      headers: {
        "content-type": "application/json", "layerx-webhook-id": id, "layerx-webhook-timestamp": timestamp,
        "layerx-webhook-key-id": "merchant", "layerx-webhook-signature": `v1=${signature.toString("base64")}`,
      },
      body,
    });
  }
  const order = (id) => fetch(`${url}/orders/${id}`, { headers: { authorization: `Bearer ${customer.token}` } });
  return { root, url, checkout, order, receiptFor, webhook, requests, settlements, restart: async () => { await stop(); await start(); } };
}

async function merchantCheckoutOfferUsesReceiptedRung(t) {
  const app = await startMerchant(t);
  const response = await app.checkout("checkout-offer");
  assert.equal(response.status, 402);
  const offer = decodePaymentRequiredHeader(response.headers.get("payment-required"));
  assert.equal(offer.x402Version, 2);
  assert.equal(offer.accepts.length, 1);
  assert.equal(offer.accepts[0].network, "eip155:125");
  assert.equal(offer.accepts[0].extra.layerx.commitment, "receipted");
  assert.equal((await response.json()).order.state, "awaiting-payment");
  const { proof, transaction } = await app.receiptFor(offer);
  const imported = await verifyPaymentReceipt(proof, offer.accepts[0], [pin]);
  assert.equal(imported.txHash, transaction);
  const payment = encodePaymentPayloadHeader({
    x402Version: 2, resource: offer.resource, accepted: offer.accepts[0], payload: { transaction },
  });
  const paid = await app.checkout("checkout-offer", payment);
  assert.equal(paid.status, 200, JSON.stringify(await paid.clone().json()));
  const settlement = decodeSettlementHeader(paid.headers.get("payment-response"));
  assert.equal(settlement.transaction, transaction);
  assert.deepEqual(settlement.extensions.layerx, proof);
  const { order: paidOrder } = await paid.json();
  assert.equal(paidOrder.state, "paid-verified");
  assert.equal(paidOrder.receiptDigest, proof.receiptDigest.slice(2));
  assert.equal(paidOrder.transaction, transaction);
  const directory = join(app.root, "merchant-checkout", "state", "fulfillments");
  const files = await readdir(directory);
  assert.equal(files.length, 1);
  const stored = JSON.parse(await readFile(join(directory, files[0]), "utf8"));
  for (const [key, value] of Object.entries(proof)) assert.deepEqual(stored[key], value);
  assert.equal(stored.txHash, transaction);
  assert.equal(app.settlements.length, 1);
  assert.deepEqual(app.requests.map((request) => request.method), ["pax_sequencerReceipt", "eth_getRawTransactionByHash", "pax_sequencerReceipt"]);
  await app.restart();
  const replay = await app.checkout("checkout-offer", payment);
  assert.equal(replay.status, 200, JSON.stringify(await replay.clone().json()));
  assert.deepEqual(decodeSettlementHeader(replay.headers.get("payment-response")), settlement);
  assert.equal(app.settlements.length, 1);
  assert.equal(app.requests.length, 3);
}

async function merchantWebhookRefusesTransactionMismatch(t) {
  const app = await startMerchant(t);
  const checkout = await app.checkout("webhook-order");
  assert.equal(checkout.status, 402);
  const offer = decodePaymentRequiredHeader(checkout.headers.get("payment-required"));
  const { order } = await checkout.json();
  const { proof, transaction } = await app.receiptFor(offer);
  const verified = await verifyPaymentReceipt(proof, offer.accepts[0], [pin]);
  assert.equal(verified.txHash, transaction);
  const wrongTransaction = hex(Buffer.alloc(32, 0x99));
  assert.notEqual(wrongTransaction, transaction);
  const refused = await app.webhook(order, proof, wrongTransaction, "delivery-mismatch");
  assert.equal(refused.status, 400);
  assert.deepEqual(await refused.json(), { state: "refused", error: "invalid-webhook" });
  assert.deepEqual(await (await app.order(order.orderId)).json(), order);
  const stored = JSON.parse(await readFile(join(app.root, "merchant-checkout", "state", "orders", `${order.orderId}.json`), "utf8"));
  assert.equal(stored.state, "awaiting-payment");
  assert.equal(stored.transaction, undefined);
  assert.deepEqual(app.requests.map((request) => request.method), ["pax_sequencerReceipt", "eth_getRawTransactionByHash", "pax_sequencerReceipt"]);
  const paid = await app.webhook(order, proof, transaction, "delivery-matching");
  assert.equal(paid.status, 200, JSON.stringify(await paid.clone().json()));
  assert.deepEqual(await paid.json(), { state: "processed" });
  const paidOrder = await (await app.order(order.orderId)).json();
  assert.equal(paidOrder.state, "paid-verified");
  assert.equal(paidOrder.transaction, transaction);
}

test(merchantCheckoutOfferUsesReceiptedRung);
test(merchantWebhookRefusesTransactionMismatch);

async function merchantWebhookRefusesInvalidSignature(t) {
  const app = await startMerchant(t);
  const response = await app.checkout("webhook-signature");
  assert.equal(response.status, 402);
  const offer = decodePaymentRequiredHeader(response.headers.get("payment-required"));
  const { order } = await response.json();
  const { proof, transaction } = await app.receiptFor(offer);
  const rpcCount = app.requests.length;
  const refused = await app.webhook(order, proof, transaction, "delivery-bad-signature", true);
  assert.equal(refused.status, 400);
  assert.deepEqual(await refused.json(), { state: "refused", error: "invalid-webhook" });
  assert.deepEqual(await (await app.order(order.orderId)).json(), order);
  assert.equal(app.requests.length, rpcCount, "invalid webhook signature is refused before receipt transport");
  assert.equal(app.settlements.length, 0);
}

test(merchantWebhookRefusesInvalidSignature);
