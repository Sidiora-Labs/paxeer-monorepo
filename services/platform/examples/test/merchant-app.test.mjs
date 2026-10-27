import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256 } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  decodePaymentRequiredHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
  verifyPaymentReceipt,
} from "@sidiora/layerx-seller-middleware";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const asset = word(0x11);
const payee = word(0x22);
const payer = word(0x33);
const reference = word(0x44);
const keyId = word(0x55);
const pair = generateKeyPairSync("ed25519");
const publicKey = new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
const pin = { keyId: new Uint8Array(keyId), publicKey, first_height: 1n, last_height: 1000n };
const exactTopic = keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"));

function uint(value, width) {
  assert.ok(value >= 0n && value < 1n << BigInt(width * 8));
  return Buffer.from(value.toString(16).padStart(width * 2, "0"), "hex");
}

function bounded(value) {
  return Buffer.concat([uint(BigInt(value.length), 4), value]);
}

function integer(value) {
  if (value === 0n) return Buffer.alloc(0);
  const digits = value.toString(16);
  return Buffer.from(digits.padStart(digits.length + digits.length % 2, "0"), "hex");
}

function rlp(value) {
  const list = Array.isArray(value);
  const payload = list ? Buffer.concat(value.map(rlp)) : value;
  if (!list && payload.length === 1 && payload[0] < 128) return payload;
  const prefix = list ? 192 : 128;
  if (payload.length < 56) return Buffer.concat([Buffer.of(prefix + payload.length), payload]);
  const length = integer(BigInt(payload.length));
  return Buffer.concat([Buffer.of(prefix + 55 + length.length), length, payload]);
}

function transaction(amount, paymentReference) {
  const selector = keccak256(new TextEncoder().encode("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")).slice(0, 4);
  const data = Buffer.concat([selector, payer, payee, asset, uint(amount, 32), uint(1n, 32), paymentReference]);
  const fields = [integer(125n), integer(0n), integer(1n), integer(2n), integer(200000n),
    bytes(LAYERX_LEDGER_ADDRESS), integer(0n), data, []];
  const unsigned = Buffer.concat([Buffer.of(2), rlp(fields)]);
  const signature = secp256k1.sign(keccak256(unsigned), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
  const parsed = secp256k1.Signature.fromBytes(signature, "recovered");
  const raw = Buffer.concat([Buffer.of(2), rlp([...fields, integer(BigInt(signature[0])), integer(parsed.r), integer(parsed.s)])]);
  const decoded = decodeRawEip1559(raw);
  assert.equal(decoded.chainId, 125n);
  assert.deepEqual(Buffer.from(decoded.data), data);
  return raw;
}

function effect(ordinal, kind, body) {
  return Buffer.concat([uint(12n, 2), uint(BigInt(ordinal), 2), uint(0n, 2), Buffer.of(kind, 0), bounded(Buffer.alloc(32)), bounded(body)]);
}

function signedProof({ amount = 25n, height = 7n, signingPin = pin, paymentReference = reference,
  paymentTo = payee, logAddress = LAYERX_LEDGER_ADDRESS, count = 1, resultCode = 0n } = {}) {
  const rawTransaction = transaction(amount, paymentReference);
  const txHash = keccak256(rawTransaction);
  const logData = Buffer.concat([asset, uint(amount, 32)]);
  const logBody = Buffer.concat([bytes(logAddress), Buffer.of(4), exactTopic, paymentReference, payer, paymentTo, sha256(logData)]);
  const summary = Buffer.concat([
    uint(50000n, 8), uint(50000n, 8), uint(2n, 8), uint(0n, 4), Buffer.alloc(40), sha256(Buffer.alloc(0)),
    uint(0n, 4), uint(BigInt(count), 4), Buffer.of(4), Buffer.from("uhpx"),
  ]);
  const effects = [effect(0, 1, summary), ...Array.from({ length: count }, (_, i) => effect(i + 1, 3, logBody))];
  const from = Buffer.from("0000000000000000000000001a642f0e3c3af545e7acbd38b07251b3990914f1", "hex");
  const to = Buffer.concat([Buffer.alloc(12), bytes(LAYERX_LEDGER_ADDRESS)]);
  const prefix = Buffer.concat([
    uint(2n, 2), uint(0x5201n, 2), uint(2n, 2), bounded(txHash), uint(1n, 8),
    bounded(Buffer.alloc(32)), bounded(sha256(Buffer.concat([uint(1n, 8), txHash]))), bounded(sha256(Buffer.alloc(0))),
    uint(resultCode, 4), uint(BigInt(effects.length), 4), ...effects,
    uint(100000n, 16), bounded(uint(height, 32)), uint(12n, 2), uint(1n, 4), uint(0n, 4), Buffer.of(2),
    bounded(sha256(Buffer.from("uhpx"))), uint(0n, 16), bounded(from), uint(1000000n, 16), uint(900000n, 16),
    uint(0n, 8), bounded(to), uint(0n, 16), uint(0n, 16), bounded(Buffer.alloc(32)), bounded(Buffer.alloc(32)),
    bounded(keyId), uint(1n, 8),
  ]);
  const unsignedReceipt = Buffer.concat([prefix, Buffer.of(0)]);
  const signingDigest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), unsignedReceipt]));
  const signedReceipt = Buffer.concat([prefix, Buffer.of(1), bounded(sign(null, signingDigest, pair.privateKey))]);
  return {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted", rawTransaction: hex(rawTransaction),
    signedReceipt: hex(signedReceipt), receiptDigest: hex(keccak256(signedReceipt)),
    receiptKey: { keyId: hex(signingPin.keyId), publicKey: hex(signingPin.publicKey),
      firstHeight: `0x${signingPin.first_height.toString(16)}`, lastHeight: `0x${signingPin.last_height.toString(16)}` },
  };
}

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
      assert.equal(request.url, "/rpc");
      requests.push(message);
      const proof = transactions.get(message.params[0]);
      assert.ok(proof);
      assert.ok(["eth_getRawTransactionByHash", "pax_sequencerReceipt"].includes(message.method));
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result:
        message.method === "eth_getRawTransactionByHash" ? proof.rawTransaction : {
          txHash: message.params[0], rung: "receipted", receipt: proof.signedReceipt,
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
    rpcUrl: `${authorityUrl}/rpc`,
    settlementUrl: `${authorityUrl}/settle`,
    receiptKeysEnvironment: "LAYERX_MERCHANT_TEST_RECEIPT_KEYS_JSON",
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
    LAYERX_EMULATOR_PRICE: "25",
    LAYERX_MERCHANT_TEST_RECEIPT_KEYS_JSON: JSON.stringify([{
      keyId: hex(keyId), publicKey: hex(publicKey), firstHeight: "1", lastHeight: "1000",
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
    child = spawn(process.execPath, [fileURLToPath(new URL("../merchant-checkout/index.mjs", import.meta.url)),
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
      headers: { "content-type": "application/json", ...(payment === undefined ? {} : { "payment-signature": payment }) },
      body: JSON.stringify({ principal: "merchant-customer", checkout_key: key, lines: [{ sku: "metered-report", quantity: 1 }] }),
    });
  }
  function receiptFor(offer) {
    const proof = signedProof({ paymentReference: bytes(`0x${offer.accepts[0].extra.layerx.purposeHash}`) });
    const transaction = hex(keccak256(bytes(proof.rawTransaction)));
    transactions.set(transaction, proof);
    return { proof, transaction };
  }
  async function webhook(order, proof, transaction, id) {
    const body = Buffer.from(JSON.stringify({
      order_id: order.orderId, request_digest: order.requestDigest,
      receipt_digest: proof.receiptDigest.slice(2), receipt_ref: hex(keccak256(bytes(proof.rawTransaction))),
      transaction, verification: "receipted",
    }));
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = sign(null, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]), webhookKey.privateKey);
    return fetch(`${url}/webhooks/settlement`, {
      method: "POST",
      headers: {
        "content-type": "application/json", "layerx-webhook-id": id, "layerx-webhook-timestamp": timestamp,
        "layerx-webhook-key-id": "merchant", "layerx-webhook-signature": `v1=${signature.toString("base64")}`,
      },
      body,
    });
  }
  return { root, url, checkout, receiptFor, webhook, requests, settlements, restart: async () => { await stop(); await start(); } };
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
  const { proof, transaction } = app.receiptFor(offer);
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
  assert.deepEqual(app.requests.map((request) => request.method), ["eth_getRawTransactionByHash", "pax_sequencerReceipt"]);
  await app.restart();
  const replay = await app.checkout("checkout-offer", payment);
  assert.equal(replay.status, 200, JSON.stringify(await replay.clone().json()));
  assert.deepEqual(decodeSettlementHeader(replay.headers.get("payment-response")), settlement);
  assert.equal(app.settlements.length, 1);
  assert.equal(app.requests.length, 2);
}

async function merchantWebhookRefusesTransactionMismatch(t) {
  const app = await startMerchant(t);
  const checkout = await app.checkout("webhook-order");
  assert.equal(checkout.status, 402);
  const offer = decodePaymentRequiredHeader(checkout.headers.get("payment-required"));
  const { order } = await checkout.json();
  const { proof, transaction } = app.receiptFor(offer);
  const verified = await verifyPaymentReceipt(proof, offer.accepts[0], [pin]);
  assert.equal(verified.txHash, transaction);
  const wrongTransaction = hex(Buffer.alloc(32, 0x99));
  assert.notEqual(wrongTransaction, transaction);
  const refused = await app.webhook(order, proof, wrongTransaction, "delivery-mismatch");
  assert.equal(refused.status, 400);
  assert.deepEqual(await refused.json(), { state: "refused", error: "invalid-webhook" });
  assert.deepEqual(await (await fetch(`${app.url}/orders/${order.orderId}`)).json(), order);
  const stored = JSON.parse(await readFile(join(app.root, "merchant-checkout", "state", "orders", `${order.orderId}.json`), "utf8"));
  assert.equal(stored.state, "awaiting-payment");
  assert.equal(stored.transaction, undefined);
  assert.deepEqual(app.requests.map((request) => request.method), ["eth_getRawTransactionByHash", "pax_sequencerReceipt"]);
  const paid = await app.webhook(order, proof, transaction, "delivery-matching");
  assert.equal(paid.status, 200, JSON.stringify(await paid.clone().json()));
  assert.deepEqual(await paid.json(), { state: "processed" });
  const paidOrder = await (await fetch(`${app.url}/orders/${order.orderId}`)).json();
  assert.equal(paidOrder.state, "paid-verified");
  assert.equal(paidOrder.transaction, transaction);
}

test(merchantCheckoutOfferUsesReceiptedRung);
test(merchantWebhookRefusesTransactionMismatch);
