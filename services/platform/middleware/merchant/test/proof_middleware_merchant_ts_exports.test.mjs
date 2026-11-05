// Proves: conformance/receipts.ts buildSignedReceipt + scenarios.ts buildMerchantSettlementDelivery -> merchant/dist/index.js MerchantSettlementWebhooks -> examples/support/merchant-app.mjs createMerchantApplicationServer + conformance/scenarios.ts runMerchantScenarios.
// Covers SettlementWebhookEvent, MerchantReceiptEvidence, MerchantReceiptResolver,
// MerchantSettlementWebhooksConfig and MerchantSettlementWebhooks through real
// signed receipts, JSON-RPC receipt resolution, signed HTTP deliveries and disk orders.
// Repairs merchant/src/index.ts requiredField: malformed webhook text must report
// invalid-webhook, rather than the unrelated invalid-cart error.
// Only SDK, seller and merchant dist entries are imported; the declared gates build
// them in dependency order. Node loads the conformance consumer's TypeScript source.
import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createMerchantApplicationServer } from "../../../examples/support/merchant-app.mjs";

const conformanceSource = new URL("../../conformance/src/", import.meta.url).href;
// Resolve the consumer's emitted .js specifiers to its real .ts source. This
// changes only file resolution; all signing, decoding and verification runs intact.
const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL?.startsWith(conformanceSource)
      && (specifier === "./receipts.js" || specifier === "./support.js")) {
      return nextResolve(new URL(specifier.replace(/\.js$/u, ".ts"), context.parentURL).href, context);
    }
    return nextResolve(specifier, context);
  },
});
const { ConformanceSequencer, buildSignedReceipt, fixedBytes, toHex } =
  await import("../../conformance/src/receipts.ts");
const { buildMerchantSettlementDelivery, runMerchantScenarios } =
  await import("../../conformance/src/scenarios.ts");
hooks.deregister();

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}`;
}

async function close(server) {
  if (server?.listening) await new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeIdleConnections();
  });
}

async function proofMiddlewareMerchantTsExports(context) {
  const directory = await mkdtemp(join(tmpdir(), "merchant-ts-exports-"));
  const sequencer = await ConformanceSequencer.generate();
  const foreignSequencer = await ConformanceSequencer.generate();
  const key = sequencer.receiptKey();
  const asset = fixedBytes(0xbb);
  const payTo = fixedBytes(0xaa);
  const requests = [];
  const receipts = new Map();
  const rpc = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    requests.push(body);
    const receipt = receipts.get(body.params?.[0]);
    let result;
    if (body.method === "eth_getRawTransactionByHash") result = receipt?.rawTransaction ?? null;
    else if (body.method === "pax_sequencerReceipt" && receipt !== undefined) {
      result = { txHash: receipt.txHash, rung: "receipted", receipt: receipt.signedReceipt };
    } else {
      response.writeHead(400).end();
      return;
    }
    response.writeHead(200, { "content-type": "application/json" })
      .end(JSON.stringify({ jsonrpc: "2.0", id: body.id, result }));
  });
  let application;
  const bindings = {
    MERCHANT_PROOF_TOKEN: "merchant-proof-local-token",
    MERCHANT_PROOF_PRICE: "250000",
    MERCHANT_PROOF_ASSET: toHex(asset),
    MERCHANT_PROOF_PAY_TO: toHex(payTo),
    MERCHANT_PROOF_ACCOUNT: "agent:merchant-proof:main",
    MERCHANT_PROOF_CURRENCY: "UHPX",
    MERCHANT_PROOF_KEYS: JSON.stringify([{
      keyId: toHex(key.keyId), publicKey: toHex(key.publicKey),
      firstHeight: key.firstHeight.toString(), lastHeight: key.lastHeight.toString(),
    }]),
    MERCHANT_PROOF_WEBHOOK_KEYS: JSON.stringify({ "seq-merchant": toHex(sequencer.publicKey) }),
  };
  const previous = Object.fromEntries(Object.keys(bindings).map((name) => [name, process.env[name]]));
  delete process.env.LAYERX_EXAMPLE_STATE_ROOT;
  Object.assign(process.env, bindings);
  context.after(async () => {
    await close(application);
    await close(rpc);
    for (const [name, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
    await rm(directory, { recursive: true, force: true });
  });
  const rpcUrl = await listen(rpc);
  const config = {
    directory, stateDirectory: "state", publicUrl: "https://merchant.example",
    rpcUrl, settlementUrl: `${rpcUrl}/settle`, scheme: "exact",
    tokenEnvironment: "MERCHANT_PROOF_TOKEN", priceEnvironment: "MERCHANT_PROOF_PRICE",
    assetEnvironment: "MERCHANT_PROOF_ASSET", payToEnvironment: "MERCHANT_PROOF_PAY_TO",
    accountEnvironment: "MERCHANT_PROOF_ACCOUNT", currencyEnvironment: "MERCHANT_PROOF_CURRENCY",
    receiptKeysEnvironment: "MERCHANT_PROOF_KEYS", webhookKeysEnvironment: "MERCHANT_PROOF_WEBHOOK_KEYS",
  };
  application = createMerchantApplicationServer(config);
  let baseUrl = await listen(application);
  const checkout = await fetch(`${baseUrl}/checkout`, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ principal: "acct:merchant-proof", checkout_key: "proof-order",
      lines: [{ sku: "metered-report", quantity: 1 }] }),
  });
  const opened = await checkout.json();
  assert.equal(checkout.status, 402, JSON.stringify(opened));
  assert.equal(opened.state, "payment-required");
  assert.equal(opened.order.state, "awaiting-payment");
  const receipt = await buildSignedReceipt(sequencer, {
    asset, payTo, amount: 250_000n, from: fixedBytes(0xcc), batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2), resultingStateRoot: fixedBytes(0xd3),
    purposeHash: opened.order.quote.paymentRequired.accepts[0].extra.layerx.purposeHash,
  });
  receipts.set(receipt.txHash, receipt);
  const orderPath = join(directory, "state/orders/proof-order.json");
  assert.deepEqual(JSON.parse(await readFile(orderPath, "utf8")), opened.order);

  async function reopen() {
    await close(application);
    application = createMerchantApplicationServer(config);
    baseUrl = await listen(application);
  }
  async function observedOrder() {
    const response = await fetch(`${baseUrl}/orders/proof-order`);
    assert.equal(response.status, 200);
    return response.json();
  }
  async function deliver(delivery) {
    const response = await fetch(`${baseUrl}/webhooks/settlement`, {
      method: "POST", body: delivery.body,
      headers: Object.fromEntries(Object.entries(delivery.headers).map(([name, value]) =>
        [`layerx-webhook-${name === "keyId" ? "key-id" : name}`, value])),
    });
    return { status: response.status, body: await response.json() };
  }
  await reopen();
  assert.deepEqual(await observedOrder(), opened.order);
  const wrongSigner = await buildMerchantSettlementDelivery(foreignSequencer, opened.order, receipt, "proof-wrong-signer");
  assert.deepEqual(await deliver(wrongSigner), { status: 400, body: { state: "refused", error: "invalid-webhook" } });
  assert.equal(requests.length, 0, "wrong signatures must fail before receipt resolution");

  assert.deepEqual(await observedOrder(), opened.order, "a foreign signer must not pay the order");
  const first = await buildMerchantSettlementDelivery(sequencer, opened.order, receipt, "proof-first");
  assert.deepEqual(await deliver(first), { status: 200, body: { state: "processed" } });
  const paid = await observedOrder();
  assert.equal(paid.state, "paid-verified");
  assert.equal(paid.receiptDigest, receipt.receiptDigest.slice(2));
  assert.equal(paid.transaction, receipt.txHash);
  assert.deepEqual(JSON.parse(await readFile(orderPath, "utf8")), paid);
  assert.equal(requests.length, 2);

  await reopen();
  assert.deepEqual(await observedOrder(), paid);
  assert.deepEqual(await deliver(first), { status: 200, body: { state: "duplicate" } });
  assert.equal(requests.length, 2, "the persisted delivery ledger must recognize exact replays");
  const second = await buildMerchantSettlementDelivery(sequencer, opened.order, receipt, "proof-second");
  assert.deepEqual(await deliver(second), { status: 200, body: { state: "processed" } });
  assert.deepEqual(await observedOrder(), paid);
  assert.deepEqual(requests.map((request) => request.method), [
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
  ], "a fresh delivery must cross the real receipt transport again");
  for (const request of requests) {
    assert.equal(request.jsonrpc, "2.0");
    assert.deepEqual(request.params, [receipt.txHash]);
  }

  const malformed = await buildMerchantSettlementDelivery(sequencer, opened.order, receipt, "proof-malformed");
  const event = JSON.parse(new TextDecoder().decode(malformed.body));
  event.receipt_ref = "";
  malformed.body = new TextEncoder().encode(JSON.stringify(event));
  const prefix = Buffer.from(`${malformed.headers.id}.${malformed.headers.timestamp}.`);
  malformed.headers.signature = `v1=${Buffer.from(await sequencer.sign(Buffer.concat([prefix, malformed.body]))).toString("base64")}`;
  assert.deepEqual(await deliver(malformed), { status: 400, body: { state: "refused", error: "invalid-webhook" } });
  assert.equal(requests.length, 4, "malformed events must be refused at the merchant decoder");

  // Corrupt the real producer's signed result in transit; the application's
  // SequencerReceiptClient must refuse it, even after two successful deliveries.
  receipts.set(receipt.txHash, { ...receipt, signedReceipt: "0x00" });
  const brokenReceipt = await buildMerchantSettlementDelivery(sequencer, opened.order, receipt, "proof-broken-receipt");
  assert.deepEqual(await deliver(brokenReceipt), {
    status: 400, body: { state: "refused", error: "sequencer_receipt_verification_failed" },
  });
  assert.equal(requests.length, 6, "receipt evidence must be resolved afresh, without a cached success");
  assert.deepEqual(await observedOrder(), paid);
  assert.deepEqual(JSON.parse(await readFile(orderPath, "utf8")), paid);
  receipts.set(receipt.txHash, receipt);
  assert.deepEqual(await deliver(brokenReceipt), { status: 200, body: { state: "processed" } });
  assert.equal(requests.length, 8, "a refused delivery must be retryable after evidence recovery");
  await reopen();
  assert.deepEqual(await observedOrder(), paid);
}

async function proofConformanceMerchantConsumer() {
  const suite = await runMerchantScenarios();
  const results = suite.results();
  assert.deepEqual(results.map((result) => result.name), [
    "merchant: a signed settlement webhook verifies the receipt and pays exactly once",
    "merchant: a signed claim with mismatched receipt evidence cannot pay an order",
  ]);
  for (const result of results) assert.equal(result.ok, true, `${result.name}: ${result.detail ?? ""}`);
}

test(proofMiddlewareMerchantTsExports);
test(proofConformanceMerchantConsumer);
