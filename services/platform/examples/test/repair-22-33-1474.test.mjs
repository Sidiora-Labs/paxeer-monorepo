import assert from "node:assert/strict";
import { fork } from "node:child_process";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { channel } from "node:diagnostics_channel";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { mkdtemp, readFile, readdir, rm, stat } from "node:fs/promises";
import { createServer } from "node:http";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { setTimeout as delay } from "node:timers/promises";
import { fileURLToPath } from "node:url";
import test from "node:test";

const workerOptions = process.argv[2] === "--application-worker" ? once(process, "message") : undefined;
const root = new URL("../../../../", import.meta.url);
const sources = new Map([
  ["@sidiora/layerx-sdk", new URL("services/agent/sdk/typescript/src/index.ts", root).href],
  ["@sidiora/layerx-seller-middleware", new URL("services/platform/middleware/seller/src/index.ts", root).href],
  ["@sidiora/layerx-merchant-middleware", new URL("services/platform/middleware/merchant/src/index.ts", root).href],
]);
const sourceRoots = [...sources.values()].map((url) => new URL("./", url).href);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (sources.has(specifier)) return { url: sources.get(specifier), shortCircuit: true };
    if (sourceRoots.some((url) => context.parentURL?.startsWith(url))
      && /^\.\.?\//u.test(specifier) && specifier.endsWith(".js")) {
      return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (sourceRoots.some((base) => url.startsWith(base)) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});

const { secp256k1 } = await import("@noble/curves/secp256k1.js");
const { decodeRawEip1559, encodeSequencerReceipt, keccak256 } = await import("@sidiora/layerx-sdk");
const { LAYERX_LEDGER_ADDRESS, decodePaymentRequiredHeader, decodeSettlementHeader, encodePaymentPayloadHeader }
  = await import("@sidiora/layerx-seller-middleware");
const { createMerchantApplicationServer, FileOrders, scopedCheckoutKey }
  = await import("../support/merchant-app.mjs");
const { LayerXApplicationStateError, SequencerReceiptClient } = await import("../support/runtime.mjs");
const { FileCompareAndSet } = await import("../support/file-state.mjs");
const exactTopic = keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"));

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

function transaction(amount, paymentReference, nonce = 0n) {
  const selector = keccak256(new TextEncoder().encode("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")).slice(0, 4);
  const data = Buffer.concat([selector, payer, payee, asset, uint(amount, 32), uint(1n, 32), paymentReference]);
  const fields = [integer(125n), integer(nonce), integer(1n), integer(2n), integer(200000n),
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
  paymentTo = payee, logAddress = LAYERX_LEDGER_ADDRESS, count = 1, resultCode = 0n, nonce = 0n } = {}) {
  const rawTransaction = transaction(amount, paymentReference, nonce);
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
  const zero = Buffer.alloc(32);
  const receipt = {
    protocolVersion: 2, activityId: txHash, globalSequence: 1n,
    previousStateRoot: zero, resultingStateRoot: sha256(Buffer.concat([uint(1n, 8), txHash])),
    activityRoot: sha256(Buffer.alloc(0)), resultCode: Number(resultCode),
    effects: effects.map((body, ordinal) => ({ moduleId: 12, ordinal, eventType: 0,
      kind: ordinal === 0 ? 1 : 3, monetary: false, transferSetRoot: zero,
      body: ordinal === 0 ? summary : logBody })),
    feeCharged: 100000n, batchId: uint(height, 32), moduleId: 12, moduleVersion: 1,
    parameterVersion: 0, operation: 2, asset: sha256(Buffer.from("uhpx")), amount: 0n,
    from, fromBalanceBefore: 1000000n, fromBalanceAfter: 900000n, fromSequence: 0n,
    to, toBalanceBefore: 0n, toBalanceAfter: 0n, transferSetRoot: zero,
    authorizationHash: zero, contextHash: keyId, timestamp: 1n, sequencerSignature: new Uint8Array(64),
  };
  const unsignedReceipt = encodeSequencerReceipt(receipt, false);
  const signingDigest = sha256(Buffer.concat([Buffer.from("LXP/v1/receipt\0"), unsignedReceipt]));
  receipt.sequencerSignature = sign(null, signingDigest, pair.privateKey);
  const signedReceipt = encodeSequencerReceipt(receipt);
  return {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted", rawTransaction: hex(rawTransaction),
    signedReceipt: hex(signedReceipt), receiptDigest: hex(keccak256(signedReceipt)),
    receiptKey: { keyId: hex(signingPin.keyId), publicKey: hex(signingPin.publicKey),
      firstHeight: `0x${signingPin.first_height.toString(16)}`, lastHeight: `0x${signingPin.last_height.toString(16)}` },
  };
}

const customers = [
  { token: "customer-alice-tenant-one-token-1474", tenant: "one", principal: "alice" },
  { token: "customer-bobby-tenant-one-token-1474", tenant: "one", principal: "bobby" },
  { token: "customer-alice-tenant-two-token-1474", tenant: "two", principal: "alice" },
];

async function listen(server) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  return `http://127.0.0.1:${server.address().port}`;
}

async function close(server) {
  if (!server.listening) return;
  await new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
}

function deferred() {
  let resolve;
  const promise = new Promise((accept) => { resolve = accept; });
  return { promise, resolve };
}

async function within(promise, message) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error(message)), 8000);
    })]);
  } finally {
    clearTimeout(timer);
  }
}

function worker(t, options) {
  const child = fork(fileURLToPath(import.meta.url), ["--application-worker"], {
    stdio: ["ignore", "pipe", "pipe", "ipc"], execArgv: [],
  });
  let stderr = "";
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  const messages = [];
  const listeners = new Set();
  let exited = false;
  const exit = new Promise((resolve) => child.once("exit", (code, signal) => {
    exited = true;
    resolve({ code, signal });
    for (const notify of listeners) notify();
  }));
  child.on("message", (message) => {
    messages.push(message);
    for (const notify of listeners) notify();
  });
  async function stop() {
    if (!exited) child.kill("SIGKILL");
    return exit;
  }
  t.after(stop);
  async function wait(event) {
    return within(new Promise((resolve, reject) => {
      function notify() {
        const value = messages.find((message) => message.event === event);
        const failure = messages.find((message) => message.event === "failed");
        if (value !== undefined || failure !== undefined || exited) {
          listeners.delete(notify);
          if (value !== undefined) resolve(value);
          else reject(new Error(failure?.stack ?? `worker exited before ${event}: ${stderr}`));
        }
      }
      listeners.add(notify);
      notify();
    }), `worker timeout ${event}: ${stderr}`);
  }
  child.send(options);
  return { child, wait, stop, exit, messages };
}

async function fixture(t, overrides = {}) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-repair-1474-"));
  const transactions = new Map();
  const requests = [];
  const failures = [];
  const actions = {};
  const workers = [];
  const ledger = new Map();
  let server;
  t.after(async () => {
    await Promise.all(workers.map((app) => app.stop()));
    if (server) await close(server);
    await rm(directory, { recursive: true, force: true });
    assert.deepEqual(failures, [], "loopback producer assertions");
  });
  server = createServer(async (request, response) => {
    const closed = deferred();
    response.once("close", closed.resolve);
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.headers["content-type"], "application/json");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      const target = request.url === "/settle" ? "settlement" : body.method;
      const record = { path: request.url, headers: request.headers, body, target, closed: closed.promise };
      requests.push(record);
      response.setHeader("content-type", "application/json");
      if (target === "settlement") {
        assert.equal(request.url, "/settle");
        assert.equal(request.headers.authorization, "Bearer loopback-settlement-token");
        assert.deepEqual(Object.keys(body).sort(), ["idempotencyKey", "payload", "principal", "requestDigest", "requirements"]);
        const identity = JSON.parse(body.principal);
        assert.ok(customers.some((customer) => customer.tenant === identity[0] && customer.principal === identity[1]));
        assert.match(body.idempotencyKey, /^[0-9a-f]{64}$/u);
        assert.match(body.requestDigest, /^[0-9a-f]{64}$/u);
        assert.equal(body.requirements.network, "eip155:125");
        const previous = ledger.get(body.idempotencyKey);
        if (previous !== undefined) assert.deepEqual(body, previous);
        else ledger.set(body.idempotencyKey, body);
      } else {
        assert.equal(request.url, "/");
        assert.equal(request.headers.authorization, undefined);
        assert.deepEqual(Object.keys(body).sort(), ["id", "jsonrpc", "method", "params"]);
        assert.equal(body.jsonrpc, "2.0");
        assert.equal(body.id, 1);
        assert.equal(body.params.length, 1);
        assert.ok(["eth_getRawTransactionByHash", "pax_sequencerReceipt"].includes(target));
        assert.match(body.params[0], /^0x[0-9a-f]{64}$/u);
      }
      const action = actions[target];
      if (typeof action === "function") {
        await action(response, record);
        return;
      }
      if (action === "stall-headers") return;
      if (action === "stall-body") {
        response.write('{"result":');
        return;
      }
      if (action === "oversize-stream") {
        response.write('"' + "x".repeat(16 * 1024));
        return;
      }
      if (action === "oversize-header") {
        response.writeHead(200, { "content-length": 16 * 1024 });
        response.flushHeaders();
        return;
      }
      if (action === "redirect") {
        response.writeHead(307, { location: "/credential-sink" }).end("{}");
        return;
      }
      if (action === "malformed") { response.end("{broken"); return; }
      if (action === "unknown") { response.writeHead(503).end('{"state":"unknown"}'); return; }
      if (target === "settlement") {
        if (action === "refused") response.writeHead(402).end('{"state":"refused","reason":"declined"}');
        else if (action === "pending") response.writeHead(202).end('{"state":"pending"}');
        else {
          const transaction = body.payload.payload.transaction;
          assert.ok(transactions.has(transaction));
          response.end(JSON.stringify({ state: "settled", transaction }));
        }
      } else {
        const proof = transactions.get(body.params[0]);
        assert.ok(proof);
        const result = target === "eth_getRawTransactionByHash" ? proof.rawTransaction
          : { txHash: body.params[0], rung: action === "pending" ? "submitted" : "receipted", receipt: proof.signedReceipt };
        response.end(JSON.stringify(action === "refused"
          ? { jsonrpc: "2.0", id: body.id, error: { code: -32000, message: "refused" } }
          : { jsonrpc: "2.0", id: body.id, result: action === "missing" ? null : result }));
      }
    } catch (error) {
      failures.push(error.stack);
      response.destroy();
    }
  });
  const peer = await listen(server);
  const environment = {
    LAYERX_EXAMPLE_STATE_ROOT: directory,
    REPAIR_1474_TOKEN: "loopback-settlement-token",
    REPAIR_1474_CUSTOMERS: JSON.stringify(customers),
    REPAIR_1474_KEYS: JSON.stringify([{ keyId: hex(keyId), publicKey: hex(publicKey), firstHeight: "1", lastHeight: "1000" }]),
    REPAIR_1474_WEBHOOK_KEYS: JSON.stringify({ merchant: hex(publicKey) }),
    REPAIR_1474_ASSET: hex(asset), REPAIR_1474_PAYEE: hex(payee),
    REPAIR_1474_ACCOUNT: "agent:merchant:main", REPAIR_1474_CURRENCY: "USD", REPAIR_1474_PRICE: "25",
  };
  const config = {
    application: "merchant-checkout", directory, stateDirectory: "state", publicUrl: "http://127.0.0.1",
    scheme: "exact", rpcUrl: peer + "/", settlementUrl: peer + "/settle",
    tokenEnvironment: "REPAIR_1474_TOKEN", customerCredentialsEnvironment: "REPAIR_1474_CUSTOMERS",
    receiptKeysEnvironment: "REPAIR_1474_KEYS", webhookKeysEnvironment: "REPAIR_1474_WEBHOOK_KEYS",
    assetEnvironment: "REPAIR_1474_ASSET", payToEnvironment: "REPAIR_1474_PAYEE",
    accountEnvironment: "REPAIR_1474_ACCOUNT", currencyEnvironment: "REPAIR_1474_CURRENCY",
    priceEnvironment: "REPAIR_1474_PRICE", transportLimits: { timeoutMs: 250, maxResponseBytes: 8192 },
    ...overrides,
  };
  async function start(boundary) {
    const process = worker(t, { config, environment, boundary });
    workers.push(process);
    const { url } = await process.wait("ready");
    async function checkout(key, payment, customer = customers[0], fields = {}) {
      return fetch(url + "/checkout", {
        method: "POST", headers: { "content-type": "application/json",
          ...(customer ? { authorization: `Bearer ${customer.token}` } : {}),
          ...(payment ? { "payment-signature": payment } : {}) },
        body: JSON.stringify({ checkout_key: key, lines: [{ sku: "metered-report", quantity: 1 }], ...fields }),
      });
    }
    async function get(orderId, customer = customers[0]) {
      return fetch(url + "/orders/" + orderId, { headers: customer ? { authorization: `Bearer ${customer.token}` } : {} });
    }
    async function webhook(delivery) {
      return fetch(url + "/webhooks/settlement", { method: "POST", headers: delivery.headers, body: delivery.body });
    }
    return { ...process, url, checkout, get, webhook };
  }
  function paymentFor(offer, options = {}) {
    const proof = signedProof({ paymentReference: bytes("0x" + offer.accepts[0].extra.layerx.purposeHash), ...options });
    const transaction = hex(keccak256(bytes(proof.rawTransaction)));
    transactions.set(transaction, proof);
    const payment = encodePaymentPayloadHeader({ x402Version: 2, resource: offer.resource,
      accepted: offer.accepts[0], payload: { transaction } });
    return { proof, transaction, payment };
  }
  function deliveryFor(order, proof, id) {
    const transaction = hex(keccak256(bytes(proof.rawTransaction)));
    const body = Buffer.from(JSON.stringify({ order_id: order.orderId, request_digest: order.requestDigest,
      receipt_digest: proof.receiptDigest.slice(2), receipt_ref: transaction, transaction, verification: "receipted" }));
    const timestamp = Math.floor(Date.now() / 1000).toString();
    const signature = sign(null, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]), pair.privateKey);
    return { body, headers: { "content-type": "application/json", "layerx-webhook-id": id,
      "layerx-webhook-timestamp": timestamp, "layerx-webhook-key-id": "merchant",
      "layerx-webhook-signature": "v1=" + signature.toString("base64") } };
  }
  return { directory, peer, start, config, environment, actions, requests, ledger, transactions, paymentFor, deliveryFor,
    orderDirectory: join(directory, "merchant-checkout", "state", "orders"),
    deliveryDirectory: join(directory, "merchant-checkout", "state", "webhook-deliveries") };
}

async function offerFor(app, key, customer = customers[0], fields) {
  const response = await app.checkout(key, undefined, customer, fields);
  const body = await response.json();
  assert.equal(response.status, 402, JSON.stringify(body));
  assert.equal(body.state, "payment-required");
  assert.equal(body.order.state, "awaiting-payment");
  assert.equal(body.order.tenant, customer.tenant);
  assert.equal(body.order.principal, customer.principal);
  const offer = decodePaymentRequiredHeader(response.headers.get("payment-required"));
  assert.equal(offer.x402Version, 2);
  assert.equal(offer.accepts[0].network, "eip155:125");
  assert.equal(offer.accepts[0].extra.layerx.commitment, "receipted");
  assert.equal(new URL(offer.resource.url).pathname, "/orders/" + body.order.orderId);
  assert.deepEqual(offer, body.order.quote.paymentRequired);
  return { offer, order: body.order };
}

async function assertPaid(response, proof) {
  const body = await response.json();
  assert.equal(response.status, 200, JSON.stringify(body));
  assert.equal(body.state, "paid");
  assert.equal(body.order.state, "paid-verified");
  assert.equal(body.order.receiptDigest, proof.receiptDigest.slice(2));
  const settlement = decodeSettlementHeader(response.headers.get("payment-response"));
  assert.equal(settlement.transaction, body.order.transaction);
  assert.deepEqual(settlement.extensions.layerx, proof);
  return body.order;
}

async function merchantOrderReadAuth(t) {
  const service = await fixture(t);
  let app = await service.start();
  const { offer, order } = await offerFor(app, "private-order");
  const { proof, payment } = service.paymentFor(offer);
  const paid = await assertPaid(await app.checkout("private-order", payment), proof);
  assert.deepEqual(await (await app.get(order.orderId)).json(), paid);
  for (const customer of [null, { token: "invalid-token-that-is-at-least-32-chars" }]) {
    const response = await app.get(order.orderId, customer);
    assert.equal(response.status, 401);
    assert.equal(response.headers.get("www-authenticate"), "Bearer");
    assert.deepEqual(await response.json(), { state: "refused", error: "authentication_required" });
  }
  for (const customer of customers.slice(1)) {
    const forbidden = await app.get(order.orderId, customer);
    const missing = await app.get("unrelated-order", customer);
    assert.equal(forbidden.status, 404);
    assert.equal(missing.status, 404);
    assert.deepEqual(await forbidden.json(), await missing.json());
  }
  await app.stop();
  app = await service.start();
  assert.deepEqual(await (await app.get(order.orderId)).json(), paid);
  for (const customer of customers.slice(1)) assert.equal((await app.get(order.orderId, customer)).status, 404);
}

async function merchantCheckoutPrincipalAuth(t) {
  const service = await fixture(t);
  let app = await service.start();
  const denied = await app.checkout("shared-key", undefined, null, { principal: "alice", tenant: "one" });
  assert.equal(denied.status, 401);
  await denied.json();
  await assert.rejects(readdir(service.orderDirectory), { code: "ENOENT" });
  for (const fields of [{ principal: "bobby" }, { tenant: "two" }, { principal: null }, { tenant: {} }]) {
    const response = await app.checkout("shared-key", undefined, customers[0], fields);
    assert.equal(response.status, 403);
    assert.deepEqual(await response.json(), { state: "refused", error: "identity_mismatch" });
  }
  await assert.rejects(readdir(service.orderDirectory), { code: "ENOENT" });
  const offers = [];
  for (const customer of customers) offers.push(await offerFor(app, "shared-key", customer, {
    principal: customer.principal, tenant: customer.tenant,
  }));
  assert.equal(new Set(offers.map(({ order }) => order.orderId)).size, 3);
  const owned = service.paymentFor(offers[0].offer);
  const paid = await assertPaid(await app.checkout("shared-key", owned.payment), owned.proof);
  const settled = service.requests.find((request) => request.target === "settlement");
  assert.equal(settled.body.principal, JSON.stringify(["one", "alice"]));
  assert.equal(settled.body.payload.resource.url, offers[0].offer.resource.url);
  for (let index = 1; index < customers.length; index += 1) {
    const replay = await app.checkout("shared-key", owned.payment, customers[index]);
    assert.equal(replay.status, 400);
    assert.equal((await replay.json()).state, "refused");
    assert.equal((await (await app.get(offers[index].order.orderId, customers[index])).json()).state, "awaiting-payment");
  }
  assert.equal(service.requests.filter((request) => request.target === "settlement").length, 1);
  await app.stop();
  app = await service.start();
  assert.deepEqual(await (await app.get(paid.orderId)).json(), paid);
  assert.deepEqual((await offerFor(app, "shared-key", customers[1])).order, offers[1].order);
}

async function merchantTransportUnbounded(t) {
  const service = await fixture(t);
  let app = await service.start();
  const targets = ["settlement", "eth_getRawTransactionByHash", "pax_sequencerReceipt"];
  for (const target of targets) {
    for (const action of ["stall-headers", "stall-body", "oversize-stream", "oversize-header", "redirect", "malformed"]) {
      const key = `${target}-${action}`;
      const { offer, order } = await offerFor(app, key);
      const { payment, proof } = service.paymentFor(offer);
      service.actions[target] = action;
      const before = service.requests.length;
      const started = Date.now();
      const failed = await app.checkout(key, payment);
      const body = await failed.json();
      assert.equal(failed.status, 503, JSON.stringify(body));
      assert.equal(failed.headers.get("retry-after"), "1");
      assert.equal(body.state, "unknown");
      assert.ok(Date.now() - started < 2000, "bounded checkout deadline");
      const request = service.requests.slice(before).find((entry) => entry.target === target);
      assert.ok(request);
      await within(request.closed, "producer connection was not cancelled");
      assert.equal(service.requests.some((entry) => entry.path === "/credential-sink"), false);
      assert.deepEqual(await (await app.get(order.orderId)).json(), order);
      delete service.actions[target];
      await app.stop();
      app = await service.start();
      assert.deepEqual(await (await app.get(order.orderId)).json(), order);
      const recovered = await assertPaid(await app.checkout(key, payment), proof);
      assert.equal(recovered.orderId, order.orderId);
      const attempts = service.requests.filter((entry) => entry.target === "settlement"
        && entry.body.payload.payload.transaction === recovered.transaction);
      assert.equal(attempts.length, 2);
      assert.deepEqual(attempts[0].body, attempts[1].body, "unknown outcome retries preserve the authority idempotency key");
    }
  }
  const offered = await offerFor(app, "typed-states");
  const { payment, proof } = service.paymentFor(offered.offer);
  service.actions.settlement = "pending";
  const pending = await app.checkout("typed-states", payment);
  assert.equal(pending.status, 202);
  assert.equal((await pending.json()).state, "pending");
  assert.deepEqual(await (await app.get(offered.order.orderId)).json(), offered.order);
  service.actions.settlement = "unknown";
  const unknown = await app.checkout("typed-states", payment);
  assert.equal(unknown.status, 503);
  assert.equal((await unknown.json()).state, "unknown");
  delete service.actions.settlement;
  await assertPaid(await app.checkout("typed-states", payment), proof);
  const refusedOffer = await offerFor(app, "authority-refused");
  service.actions.settlement = "refused";
  const refused = await app.checkout("authority-refused", service.paymentFor(refusedOffer.offer).payment);
  assert.equal(refused.status, 402);
  assert.equal((await refused.json()).order.state, "refused");
}

function hasState(state, detail) {
  return (error) => {
    assert.ok(error instanceof LayerXApplicationStateError);
    assert.equal(error.state, state);
    if (detail !== undefined) assert.equal(error.message, detail);
    return true;
  };
}

async function receiptTransportUnbounded(t) {
  const service = await fixture(t);
  const proof = signedProof();
  const raw = bytes(proof.rawTransaction);
  const txHash = hex(keccak256(raw));
  service.transactions.set(txHash, proof);
  const client = new SequencerReceiptClient(service.peer + "/", [pin], { timeoutMs: 250, maxResponseBytes: 8192 });
  assert.deepEqual(await client.resolve(raw), proof);
  for (const action of ["stall-headers", "stall-body", "oversize-stream", "oversize-header", "redirect", "malformed"]) {
    service.actions.pax_sequencerReceipt = action;
    const started = Date.now();
    await assert.rejects(client.resolve(raw), hasState("unknown", "sequencer_receipt_unreachable"));
    assert.ok(Date.now() - started < 2000);
    const request = service.requests.at(-1);
    assert.deepEqual(request.body, { jsonrpc: "2.0", id: 1, method: "pax_sequencerReceipt", params: [txHash] });
    await within(request.closed, "receipt producer connection was not cancelled");
  }
  service.actions.pax_sequencerReceipt = "pending";
  await assert.rejects(client.resolve(raw), hasState("pending", "sequencer_receipt_submitted"));
  service.actions.pax_sequencerReceipt = "refused";
  await assert.rejects(client.resolve(raw), hasState("refused", "sequencer_receipt_refused"));
  delete service.actions.pax_sequencerReceipt;
  assert.deepEqual(await new SequencerReceiptClient(service.peer + "/", [pin]).resolve(raw), proof);
  assert.equal(service.requests.some((request) => request.path === "/credential-sink"), false);
}

async function orderFileCrashWindow(t) {
  const service = await fixture(t);
  for (const stage of ["created", "written", "synced", "published", "durable"]) {
    const key = "crash-" + stage;
    const orderId = scopedCheckoutKey(customers[0], key);
    const path = join(service.orderDirectory, orderId + ".json");
    const app = await service.start({ stage, path });
    const request = app.checkout(key).then((response) => response.json(), (error) => error);
    const boundary = await app.wait("boundary");
    assert.equal(boundary.stage, stage);
    assert.equal(boundary.path, path);
    assert.equal((await stat(boundary.temporary)).mode & 0o777, 0o600);
    if (stage === "created") assert.equal((await stat(boundary.temporary)).size, 0);
    else assert.equal(JSON.parse(await readFile(boundary.temporary, "utf8")).state, "awaiting-payment");
    if (stage === "published" || stage === "durable") {
      assert.equal(JSON.parse(await readFile(path, "utf8")).orderId, orderId);
    } else await assert.rejects(readFile(path), { code: "ENOENT" });
    assert.deepEqual(await app.stop(), { code: null, signal: "SIGKILL" });
    assert.ok(await request instanceof Error);
    const reopened = await service.start();
    const offered = await offerFor(reopened, key);
    assert.equal(offered.order.orderId, orderId);
    assert.deepEqual(await (await reopened.get(orderId)).json(), offered.order);
    assert.deepEqual(JSON.parse(await readFile(path, "utf8")), offered.order);
    const original = await stat(path);
    assert.deepEqual((await offerFor(reopened, key)).order, offered.order);
    assert.equal((await stat(path)).ino, original.ino, "creation must never replace the winning inode");
    const conflicting = await reopened.checkout(key, undefined, customers[0], { lines: [{ sku: "metered-report", quantity: 2 }] });
    assert.equal(conflicting.status, 400);
    assert.equal((await conflicting.json()).error, "order-conflict");
    assert.deepEqual(await (await reopened.get(orderId)).json(), offered.order);
    await reopened.stop();
  }
}

async function orderCrossProcessCreation(t) {
  const service = await fixture(t);
  const key = "concurrent-open";
  const orderId = scopedCheckoutKey(customers[0], key);
  const boundary = { stage: "synced", path: join(service.orderDirectory, orderId + ".json") };
  const apps = await Promise.all([service.start(boundary), service.start(boundary)]);
  const responses = apps.map((app, index) => app.checkout(key, undefined, customers[0], {
    lines: [{ sku: "metered-report", quantity: index + 1 }],
  }));
  await Promise.all(apps.map((app) => app.wait("boundary")));
  for (const app of apps) app.child.kill("SIGCONT");
  const results = await Promise.all(responses);
  assert.deepEqual(results.map((response) => response.status).sort(), [400, 402]);
  const winner = await results.find((response) => response.status === 402).json();
  assert.equal(winner.order.state, "awaiting-payment");
  assert.equal((await results.find((response) => response.status === 400).json()).error, "order-conflict");
  await Promise.all(apps.map((app) => app.stop()));
  const reopened = await service.start();
  assert.deepEqual(await (await reopened.get(orderId)).json(), winner.order);
}

async function raceTerminals(t, secondKind) {
  const service = await fixture(t);
  const setup = await service.start();
  const key = "terminal-" + secondKind;
  const { offer, order } = await offerFor(setup, key);
  await setup.stop();
  const boundary = { stage: "synced", path: join(service.orderDirectory, order.orderId + ".json.1") };
  const apps = await Promise.all([service.start(boundary), service.start(boundary)]);
  const first = service.paymentFor(offer, { nonce: 1n, height: 7n });
  const second = service.paymentFor(offer, { nonce: 2n, height: 8n });
  if (secondKind === "refused") service.actions.settlement = (response, request) => {
    if (request.body.payload.payload.transaction === second.transaction) response.writeHead(402).end('{"state":"refused"}');
    else response.end(JSON.stringify({ state: "settled", transaction: first.transaction }));
  };
  const responses = [apps[0].checkout(key, first.payment), apps[1].checkout(key, second.payment)];
  await Promise.all(apps.map((app) => app.wait("boundary")));
  for (const app of apps) app.child.kill("SIGCONT");
  const results = await Promise.all(responses);
  assert.equal(results.filter((response) => response.status === 400).length, 1);
  assert.equal((await results.find((response) => response.status === 400).json()).error, "order-conflict");
  const winning = results.find((response) => response.status !== 400);
  let winner;
  if (winning.status === 200) {
    const proof = results[0].status === 200 ? first.proof : second.proof;
    winner = await assertPaid(winning, proof);
  } else {
    assert.equal(secondKind, "refused");
    assert.equal(winning.status, 402);
    winner = (await winning.json()).order;
    assert.equal(winner.state, "refused");
    assert.equal(winner.receiptDigest, undefined);
  }
  await Promise.all(apps.map((app) => app.stop()));
  const reopened = await service.start();
  assert.deepEqual(await (await reopened.get(order.orderId)).json(), winner);
  const orders = new FileOrders(service.orderDirectory);
  if (winner.state === "paid-verified") {
    await assert.rejects(orders.markRefused(order.orderId, order.requestDigest), { code: "order-conflict" });
    const loser = winner.transaction === first.transaction ? second : first;
    await assert.rejects(orders.markPaid(order.orderId, order.requestDigest, loser.proof.receiptDigest.slice(2), loser.transaction),
      { code: "order-conflict" });
  } else {
    await assert.rejects(orders.markPaid(order.orderId, order.requestDigest, first.proof.receiptDigest.slice(2), first.transaction),
      { code: "order-conflict" });
  }
  assert.deepEqual(await orders.get(order.orderId), winner);
}

async function orderCrossProcessPaidRefused(t) { await raceTerminals(t, "refused"); }
async function orderCrossProcessWinningReceipt(t) { await raceTerminals(t, "paid"); }

async function orderTerminalCrashRecovery(t) {
  const service = await fixture(t);
  const setup = await service.start();
  const { offer, order } = await offerFor(setup, "terminal-crash");
  await setup.stop();
  const { payment, proof } = service.paymentFor(offer);
  const interrupted = await service.start({ stage: "published", path: join(service.orderDirectory, order.orderId + ".json.1") });
  const pending = interrupted.checkout("terminal-crash", payment).then((response) => response.json(), (error) => error);
  await interrupted.wait("boundary");
  await interrupted.stop();
  assert.ok(await pending instanceof Error);
  const reopened = await service.start();
  const stored = await (await reopened.get(order.orderId)).json();
  assert.equal(stored.state, "paid-verified");
  assert.equal(stored.receiptDigest, proof.receiptDigest.slice(2));
  assert.deepEqual(await assertPaid(await reopened.checkout("terminal-crash", payment), proof), stored);
}

async function webhookFencingScenario(t, staleOutcome, sameProcess = false) {
  const service = await fixture(t, { webhookLeaseMs: 1000, transportLimits: { timeoutMs: 8000, maxResponseBytes: 8192 } });
  const apps = await Promise.all([service.start(), service.start()]);
  const { offer } = await offerFor(apps[0], "webhook-lease");
  const { proof, payment } = service.paymentFor(offer);
  const order = await assertPaid(await apps[0].checkout("webhook-lease", payment), proof);
  const id = "delivery-fenced-" + staleOutcome;
  const delivery = service.deliveryFor(order, proof, id);
  const held = [];
  let arrival = deferred();
  service.actions.eth_getRawTransactionByHash = (response, request) => {
    held.push({ response, request });
    arrival.resolve();
  };
  async function waitHeld(count) {
    while (held.length < count) {
      const current = arrival;
      await within(current.promise, "webhook owner did not reach the node transport");
      if (current === arrival) arrival = deferred();
    }
  }
  async function result(promise) {
    try {
      const response = await promise;
      return { status: response.status, body: await response.json() };
    } catch (error) { return error; }
  }
  const pending = apps.map((app, index) => result(app.webhook(delivery)).then((value) => ({ index, value })));
  const processing = await Promise.race(pending);
  assert.deepEqual(processing.value, { status: 202, body: { state: "processing" } });
  const ownerIndex = 1 - processing.index;
  const owner = apps[ownerIndex];
  await waitHeld(1);
  assert.equal(held.length, 1, "only one process may consume a claimed delivery");
  const storage = new FileCompareAndSet(service.deliveryDirectory);
  const original = await storage.read(id);
  assert.equal(original.value.state, "processing");
  assert.match(original.value.token, /^[0-9a-f-]{36}$/u);
  if (staleOutcome === "restart") {
    await owner.stop();
    assert.ok((await pending[ownerIndex]).value instanceof Error);
    await within(held[0].request.closed, "interrupted owner did not close the RPC request");
  }
  await delay(Math.max(1, original.value.leaseUntilMs - Date.now() + 30));
  const recovering = staleOutcome === "restart" ? apps[processing.index]
    : sameProcess ? owner : apps[processing.index];
  const resumed = result(recovering.webhook(delivery));
  await waitHeld(2);
  const replacement = await storage.read(id);
  assert.equal(replacement.version, original.version + 1);
  assert.equal(replacement.value.state, "processing");
  assert.notEqual(replacement.value.token, original.value.token);
  if (staleOutcome !== "restart") {
    held[0].response.end(JSON.stringify({ jsonrpc: "2.0", id: 1,
      result: staleOutcome === "release" ? null : proof.rawTransaction }));
    const stale = (await pending[ownerIndex]).value;
    assert.deepEqual(stale, { status: 400, body: { state: "refused", error: "webhook-replay" } });
    const fenced = await storage.read(id);
    assert.deepEqual(fenced, replacement, "stale complete/release must not change the new owner's lease");
  }
  held[1].response.end(JSON.stringify({ jsonrpc: "2.0", id: 1, result: proof.rawTransaction }));
  assert.deepEqual(await resumed, { status: 200, body: { state: "processed" } });
  const completed = await storage.read(id);
  assert.equal(completed.value.state, "completed");
  assert.equal(completed.value.token, replacement.value.token);
  assert.deepEqual(await (await recovering.get(order.orderId)).json(), order);
  await Promise.all(apps.map((app) => app.stop()));
  const reopened = await service.start();
  const before = service.requests.length;
  assert.deepEqual(await result(reopened.webhook(delivery)), { status: 200, body: { state: "duplicate" } });
  assert.equal(service.requests.length, before, "completed replay must not resolve or apply the receipt again");
  assert.deepEqual(await storage.read(id), completed);
  const conflict = service.deliveryFor({ ...order, requestDigest: "ab".repeat(32) }, proof, id);
  assert.deepEqual(await result(reopened.webhook(conflict)), {
    status: 400, body: { state: "refused", error: "webhook-replay" },
  });
  assert.deepEqual(await storage.read(id), completed);
  const invalid = { ...delivery, body: Buffer.from(delivery.body.toString().replace("receipted", "committed")) };
  assert.deepEqual(await result(reopened.webhook(invalid)), {
    status: 400, body: { state: "refused", error: "invalid-webhook" },
  });
}

async function webhookCrossProcessClaim(t) { await webhookFencingScenario(t, "complete"); }
async function webhookCrossProcessReleaseFencing(t) { await webhookFencingScenario(t, "release"); }
async function webhookSameProcessLeaseFencing(t) { await webhookFencingScenario(t, "complete", true); }
async function webhookCrossProcessRestart(t) { await webhookFencingScenario(t, "restart"); }

async function webhookOwnedReleaseRecovery(t) {
  const service = await fixture(t);
  let app = await service.start();
  const { offer, order } = await offerFor(app, "webhook-release");
  const { proof } = service.paymentFor(offer);
  const delivery = service.deliveryFor(order, proof, "delivery-release");
  service.actions.eth_getRawTransactionByHash = "missing";
  const pending = await app.webhook(delivery);
  assert.equal(pending.status, 202);
  assert.deepEqual(await pending.json(), { state: "pending", error: "transaction_unavailable" });
  const storage = new FileCompareAndSet(service.deliveryDirectory);
  const released = await storage.read("delivery-release");
  assert.equal(released.value.state, "released");
  assert.deepEqual(await (await app.get(order.orderId)).json(), order);
  await app.stop();
  delete service.actions.eth_getRawTransactionByHash;
  app = await service.start();
  const processed = await app.webhook(delivery);
  assert.equal(processed.status, 200);
  assert.deepEqual(await processed.json(), { state: "processed" });
  const paid = await (await app.get(order.orderId)).json();
  assert.equal(paid.state, "paid-verified");
  assert.equal(paid.receiptDigest, proof.receiptDigest.slice(2));
  const completed = await storage.read("delivery-release");
  assert.equal(completed.value.state, "completed");
  assert.notEqual(completed.value.token, released.value.token);
}

async function applicationWorker() {
  const [options] = await workerOptions;
  Object.assign(process.env, options.environment);
  if (options.boundary) {
    let paused = false;
    channel("layerx.examples.state.publication").subscribe((event) => {
      if (!paused && event.stage === options.boundary.stage && event.path === options.boundary.path) {
        paused = true;
        process.send({ event: "boundary", ...event });
        process.kill(process.pid, "SIGSTOP");
      }
    });
  }
  const server = createMerchantApplicationServer(options.config);
  const url = await listen(server);
  process.send({ event: "ready", url });
  process.once("disconnect", () => { void close(server); });
}

if (process.argv[2] === "--application-worker") {
  try { await applicationWorker(); }
  catch (error) {
    process.send({ event: "failed", stack: error.stack });
    process.exitCode = 1;
    process.disconnect();
  }
} else {
  test(async function repair22331474(t) {
    await t.test(merchantOrderReadAuth);
    await t.test(merchantCheckoutPrincipalAuth);
    await t.test(merchantTransportUnbounded);
    await t.test(receiptTransportUnbounded);
    await t.test(orderFileCrashWindow);
    await t.test(orderCrossProcessCreation);
    await t.test(orderCrossProcessPaidRefused);
    await t.test(orderCrossProcessWinningReceipt);
    await t.test(orderTerminalCrashRecovery);
    await t.test(webhookCrossProcessClaim);
    await t.test(webhookCrossProcessReleaseFencing);
    await t.test(webhookSameProcessLeaseFencing);
    await t.test(webhookCrossProcessRestart);
    await t.test(webhookOwnedReleaseRecovery);
  });
}
