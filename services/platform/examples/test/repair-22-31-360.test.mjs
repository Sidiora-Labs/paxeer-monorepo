import assert from "node:assert/strict";
import { fork } from "node:child_process";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { mkdtemp, readFile, readdir, rm, stat, writeFile } from "node:fs/promises";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
let secp256k1;
let decodeRawEip1559;
let keccak256;
let LAYERX_LEDGER_ADDRESS;
let decodePaymentRequiredHeader;
let decodeSettlementHeader;
let encodePaymentPayloadHeader;
let createMerchantApplicationServer;
let FileFulfillments;
let exactTopic;

async function loadProduction() {
  const middlewareRoot = new URL("../../middleware/", import.meta.url);
  const sourceRoots = ["merchant", "seller"].map((name) => new URL(name + "/src/", middlewareRoot).href);
  const entries = new Map(["merchant", "seller"].map((name) => [
    "@sidiora/layerx-" + name + "-middleware",
    new URL(name + "/src/index.ts", middlewareRoot).href,
  ]));
  registerHooks({
    resolve(specifier, context, nextResolve) {
      if (entries.has(specifier)) return { url: entries.get(specifier), shortCircuit: true };
      if (sourceRoots.some((root) => context.parentURL?.startsWith(root))
        && specifier.startsWith("./") && specifier.endsWith(".js")) {
        return { url: new URL(specifier.slice(0, -3) + ".ts", context.parentURL).href, shortCircuit: true };
      }
      return nextResolve(specifier, context);
    },
    load(url, context, nextLoad) {
      if (sourceRoots.some((root) => url.startsWith(root)) && url.endsWith(".ts")) {
        return {
          format: "module", shortCircuit: true,
          source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }),
        };
      }
      return nextLoad(url, context);
    },
  });
  ({ secp256k1 } = await import("@noble/curves/secp256k1.js"));
  ({ decodeRawEip1559, keccak256 } = await import("../../../agent/sdk/typescript/dist/src/index.js"));
  ({ LAYERX_LEDGER_ADDRESS, decodePaymentRequiredHeader, decodeSettlementHeader, encodePaymentPayloadHeader }
    = await import("@sidiora/layerx-seller-middleware"));
  ({ createMerchantApplicationServer, FileFulfillments } = await import("../support/merchant-app.mjs"));
  exactTopic = keccak256(new TextEncoder().encode("PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"));
}

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
  const root = await mkdtemp(join(tmpdir(), "layerx-repair-22-31-360-"));
  const savedEnvironment = { ...process.env };
  const transactions = new Map();
  const settlements = [];
  const rpcRequests = [];
  const releases = [];
  const authority = createServer(async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.headers["content-type"], "application/json");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const message = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      response.setHeader("content-type", "application/json");
      if (request.url === "/settle") {
        assert.equal(request.headers.authorization, "Bearer repair-merchant-token");
        assert.deepEqual(Object.keys(message).sort(), ["idempotencyKey", "payload", "principal", "requestDigest", "requirements"]);
        assert.equal(message.principal, "merchant-customer");
        assert.match(message.idempotencyKey, /^[a-f0-9]{64}$/u);
        assert.match(message.requestDigest, /^[a-f0-9]{64}$/u);
        settlements.push(message);
        const transaction = message.payload.payload.transaction;
        assert.ok(transactions.has(transaction));
        response.end(JSON.stringify({ state: "settled", transaction }));
        return;
      }
      assert.equal(request.url, "/rpc");
      assert.deepEqual(Object.keys(message).sort(), ["id", "jsonrpc", "method", "params"]);
      assert.equal(message.jsonrpc, "2.0");
      assert.equal(message.id, 1);
      assert.equal(message.params.length, 1);
      const proof = transactions.get(message.params[0]);
      assert.ok(proof);
      assert.ok(["eth_getRawTransactionByHash", "pax_sequencerReceipt"].includes(message.method));
      rpcRequests.push(message);
      response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result:
        message.method === "eth_getRawTransactionByHash" ? proof.rawTransaction : {
          txHash: message.params[0], rung: "receipted", receipt: proof.signedReceipt,
        } }));
    } catch (error) {
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  let server;
  let url;
  t.after(async () => {
    if (server?.listening) await close(server);
    if (authority.listening) await close(authority);
    for (const name of Object.keys(process.env)) {
      if (!(name in savedEnvironment)) delete process.env[name];
    }
    Object.assign(process.env, savedEnvironment);
    await rm(root, { recursive: true, force: true });
  });
  const authorityUrl = await listen(authority);
  Object.assign(process.env, {
    LAYERX_EXAMPLE_STATE_ROOT: root,
    REPAIR_360_TOKEN: "repair-merchant-token",
    REPAIR_360_KEYS: JSON.stringify([{
      keyId: hex(keyId), publicKey: hex(publicKey), firstHeight: "1", lastHeight: "1000",
    }]),
    REPAIR_360_WEBHOOK_KEYS: JSON.stringify({ merchant: hex(publicKey) }),
    REPAIR_360_ASSET: hex(asset),
    REPAIR_360_PAYEE: hex(payee),
    REPAIR_360_ACCOUNT: "agent:merchant:main",
    REPAIR_360_CURRENCY: "USD",
    REPAIR_360_PRICE: "25",
  });
  const config = {
    application: "merchant-checkout", directory: root, stateDirectory: "state", publicUrl: "http://127.0.0.1",
    scheme: "exact", rpcUrl: authorityUrl + "/rpc", settlementUrl: authorityUrl + "/settle",
    tokenEnvironment: "REPAIR_360_TOKEN", receiptKeysEnvironment: "REPAIR_360_KEYS",
    webhookKeysEnvironment: "REPAIR_360_WEBHOOK_KEYS", assetEnvironment: "REPAIR_360_ASSET",
    payToEnvironment: "REPAIR_360_PAYEE", accountEnvironment: "REPAIR_360_ACCOUNT",
    currencyEnvironment: "REPAIR_360_CURRENCY", priceEnvironment: "REPAIR_360_PRICE",
  };
  async function start() {
    server = createMerchantApplicationServer(config);
    server.on("request", (request) => {
      if (request.headers["idempotency-key"] !== undefined) {
        releases.push({ method: request.method, path: request.url, key: request.headers["idempotency-key"] });
      }
    });
    url = await listen(server);
  }
  await start();
  async function checkout(payment) {
    return fetch(url + "/checkout", {
      method: "POST",
      headers: { "content-type": "application/json", ...(payment === undefined ? {} : { "payment-signature": payment }) },
      body: JSON.stringify({ principal: "merchant-customer", checkout_key: "recovery-order",
        lines: [{ sku: "metered-report", quantity: 1 }] }),
    });
  }
  const offered = await checkout();
  assert.equal(offered.status, 402);
  const offer = decodePaymentRequiredHeader(offered.headers.get("payment-required"));
  assert.equal(offer.x402Version, 2);
  assert.equal(offer.accepts[0].network, "eip155:125");
  assert.equal(offer.accepts[0].extra.layerx.commitment, "receipted");
  assert.equal((await offered.json()).order.state, "awaiting-payment");
  const proof = signedProof({ paymentReference: bytes("0x" + offer.accepts[0].extra.layerx.purposeHash) });
  const transaction = hex(keccak256(bytes(proof.rawTransaction)));
  transactions.set(transaction, proof);
  const payment = encodePaymentPayloadHeader({
    x402Version: 2, resource: offer.resource, accepted: offer.accepts[0], payload: { transaction },
  });
  const paid = await checkout(payment);
  assert.equal(paid.status, 200, JSON.stringify(await paid.clone().json()));
  const settlement = decodeSettlementHeader(paid.headers.get("payment-response"));
  assert.equal(settlement.transaction, transaction);
  assert.deepEqual(settlement.extensions.layerx, proof);
  const { state, order } = await paid.json();
  assert.equal(state, "paid");
  assert.equal(order.state, "paid-verified");
  assert.equal(order.transaction, transaction);
  assert.equal(order.receiptDigest, proof.receiptDigest.slice(2));
  assert.equal(settlements.length, 1);
  assert.deepEqual(rpcRequests.map((request) => request.method), ["eth_getRawTransactionByHash", "pax_sequencerReceipt"]);
  const fulfillmentDirectory = join(root, "merchant-checkout", "state", "fulfillments");
  const files = await readdir(fulfillmentDirectory);
  assert.equal(files.length, 1);
  const produced = JSON.parse(await readFile(join(fulfillmentDirectory, files[0]), "utf8"));
  for (const [key, value] of Object.entries(proof)) assert.deepEqual(produced[key], value);
  const { resource, ...proposed } = produced;
  assert.equal(resource.orderId, order.orderId);
  assert.equal(proposed.txHash, transaction);
  const proposalPath = join(root, "proposal.json");
  await writeFile(proposalPath, JSON.stringify(proposed));
  const directory = join(root, "recovering-fulfillments");
  return {
    root, directory, proposalPath, proposed, order, releases, checkout, settlement, payment,
    releaseUrl: () => url + "/orders/" + order.orderId,
    restart: async () => { await close(server); await start(); },
    settlements, rpcRequests,
  };
}

async function runFulfillmentWorker(message) {
  const proposed = JSON.parse(await readFile(message.proposalPath, "utf8"));
  const fulfillments = new FileFulfillments(message.directory);
  try {
    const record = await fulfillments.fulfill(proposed, async (idempotencyKey) => {
      const path = join(message.directory, proposed.idempotencyKey + ".json");
      const pending = JSON.parse(await readFile(path + ".pending", "utf8"));
      assert.deepEqual(pending, { state: "pending", fulfillment: proposed });
      assert.equal(idempotencyKey, proposed.idempotencyKey);
      process.send({ event: "before-release", idempotencyKey });
      if (message.boundary === "before-release") await once(process, "message");
      const response = await fetch(message.releaseUrl, {
        headers: { "idempotency-key": idempotencyKey },
      });
      assert.equal(response.status, 200);
      assert.equal(response.headers.get("content-type"), "application/json");
      const resource = await response.json();
      assert.equal(resource.state, "paid-verified");
      assert.equal(resource.transaction, proposed.txHash);
      assert.equal(resource.receiptDigest, proposed.receiptDigest.slice(2));
      process.send({ event: "after-release", idempotencyKey, resource });
      if (message.boundary === "after-release") await once(process, "message");
      return resource;
    });
    process.send({ event: "completed", record });
  } catch (error) {
    process.send({ event: "failed", error: error.code ?? error.message, stack: error.stack });
    process.exitCode = 1;
  }
  process.disconnect();
}

function startWorker(t, app, boundary, proposalPath = app.proposalPath) {
  const child = fork(fileURLToPath(import.meta.url), ["--fulfillment-worker"], {
    stdio: ["ignore", "pipe", "pipe", "ipc"], execArgv: [],
  });
  let stderr = "";
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  const messages = [];
  const listeners = new Set();
  child.on("message", (message) => {
    messages.push(message);
    for (const notify of listeners) notify();
  });
  let exited = false;
  const exit = new Promise((resolve) => child.once("exit", (code, signal) => {
    exited = true;
    resolve({ code, signal });
    for (const notify of listeners) notify();
  }));
  async function stop() {
    if (!exited) child.kill("SIGKILL");
    return exit;
  }
  t.after(stop);
  function wait(event) {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => finish(new Error("worker_timeout_" + event + ": " + stderr)), 10000);
      function finish(error, value) {
        clearTimeout(timer);
        listeners.delete(notify);
        if (error) reject(error);
        else resolve(value);
      }
      function notify() {
        const message = messages.find((entry) => entry.event === event);
        const failure = messages.find((entry) => entry.event === "failed");
        if (message !== undefined) finish(undefined, message);
        else if (failure !== undefined) finish(new Error(failure.stack));
        else if (exited) finish(new Error("worker_exited_before_" + event + ": " + stderr));
      }
      listeners.add(notify);
      notify();
    });
  }
  child.send({ directory: app.directory, proposalPath, releaseUrl: app.releaseUrl(), boundary });
  return { child, wait, stop, exit, messages };
}

async function assertPending(app) {
  const path = join(app.directory, app.proposed.idempotencyKey + ".json");
  assert.deepEqual(JSON.parse(await readFile(path + ".pending", "utf8")), {
    state: "pending", fulfillment: app.proposed,
  });
  assert.equal(await new FileFulfillments(app.directory).get(app.proposed.idempotencyKey), undefined);
  await assert.rejects(readFile(path), { code: "ENOENT" });
}

async function assertCompleted(t, app, expected) {
  const repository = new FileFulfillments(app.directory);
  const stored = await repository.get(app.proposed.idempotencyKey);
  assert.deepEqual(stored, { ...app.proposed, resource: expected });
  const path = join(app.directory, app.proposed.idempotencyKey + ".json");
  const inode = (await stat(path)).ino;
  const releases = app.releases.length;
  const replay = startWorker(t, app);
  assert.deepEqual((await replay.wait("completed")).record, stored);
  assert.deepEqual(await replay.exit, { code: 0, signal: null });
  assert.equal(app.releases.length, releases);
  assert.equal((await stat(path)).ino, inode);
  assert.deepEqual(await readdir(app.directory), [app.proposed.idempotencyKey + ".json"]);
  for (const release of app.releases) {
    assert.deepEqual(release, { method: "GET", path: "/orders/" + app.order.orderId, key: app.proposed.idempotencyKey });
  }
}

async function fulfillmentReleaseBeforeClaimConcurrentProcesses(t) {
  const app = await startMerchant(t);
  const workers = Array.from({ length: 4 }, () => startWorker(t, app, "before-release"));
  await Promise.all(workers.map((worker) => worker.wait("before-release")));
  await assertPending(app);
  assert.equal(app.releases.length, 0);
  for (const worker of workers) worker.child.send({ continue: true });
  const results = await Promise.all(workers.map((worker) => worker.wait("completed")));
  for (const worker of workers) assert.deepEqual(await worker.exit, { code: 0, signal: null });
  for (const result of results) assert.deepEqual(result.record, { ...app.proposed, resource: app.order });
  assert.equal(app.releases.length, 4);
  await assertCompleted(t, app, app.order);
  await app.restart();
  const replay = await app.checkout(app.payment);
  assert.equal(replay.status, 200, JSON.stringify(await replay.clone().json()));
  assert.deepEqual(decodeSettlementHeader(replay.headers.get("payment-response")), app.settlement);
  assert.equal((await replay.json()).order.state, "paid-verified");
}

async function fulfillmentReleaseBeforeClaimRecoversBeforeRelease(t) {
  const app = await startMerchant(t);
  const interrupted = startWorker(t, app, "before-release");
  await interrupted.wait("before-release");
  await assertPending(app);
  assert.deepEqual(await interrupted.stop(), { code: null, signal: "SIGKILL" });
  assert.equal(app.releases.length, 0);
  await app.restart();
  const recovered = startWorker(t, app);
  assert.deepEqual((await recovered.wait("completed")).record.resource, app.order);
  assert.deepEqual(await recovered.exit, { code: 0, signal: null });
  assert.equal(app.releases.length, 1);
  await assertCompleted(t, app, app.order);
}

async function fulfillmentReleaseBeforeClaimRecoversAfterRelease(t) {
  const app = await startMerchant(t);
  const interrupted = startWorker(t, app, "after-release");
  const released = await interrupted.wait("after-release");
  assert.deepEqual(released.resource, app.order);
  await assertPending(app);
  assert.deepEqual(await interrupted.stop(), { code: null, signal: "SIGKILL" });
  assert.equal(app.releases.length, 1);
  await app.restart();
  const recovered = startWorker(t, app);
  const result = await recovered.wait("completed");
  assert.deepEqual(result.record.resource, released.resource);
  assert.deepEqual(await recovered.exit, { code: 0, signal: null });
  assert.equal(app.releases.length, 2);
  await assertCompleted(t, app, released.resource);
}

async function fulfillmentReleaseBeforeClaimRejectsConflictingPending(t) {
  const app = await startMerchant(t);
  const interrupted = startWorker(t, app, "before-release");
  await interrupted.wait("before-release");
  await assertPending(app);
  for (const field of ["requestDigest", "receiptDigest", "txHash"]) {
    const proposalPath = join(app.root, "conflicting-" + field + ".json");
    const changed = { ...app.proposed, [field]: (app.proposed[field].startsWith("0x") ? "0x" : "") + "ab".repeat(32) };
    assert.notEqual(changed[field], app.proposed[field]);
    await writeFile(proposalPath, JSON.stringify(changed));
    const conflicting = startWorker(t, app, undefined, proposalPath);
    assert.equal((await conflicting.wait("failed")).error, "fulfillment-conflict");
    assert.deepEqual(await conflicting.exit, { code: 1, signal: null });
    assert.equal(app.releases.length, 0);
    await assertPending(app);
  }
  assert.deepEqual(await interrupted.stop(), { code: null, signal: "SIGKILL" });
  const recovered = startWorker(t, app);
  await recovered.wait("completed");
  assert.deepEqual(await recovered.exit, { code: 0, signal: null });
  await assertCompleted(t, app, app.order);
}

if (process.argv[2] === "--fulfillment-worker") {
  const message = once(process, "message");
  try {
    await loadProduction();
    await runFulfillmentWorker((await message)[0]);
  } catch (error) {
    process.send({ event: "failed", error: error.code ?? error.message, stack: error.stack });
    process.exitCode = 1;
    process.disconnect();
  }
} else {
  test(async function fulfillmentReleaseBeforeClaim(t) {
    await loadProduction();
    await t.test(fulfillmentReleaseBeforeClaimConcurrentProcesses);
    await t.test(fulfillmentReleaseBeforeClaimRecoversBeforeRelease);
    await t.test(fulfillmentReleaseBeforeClaimRecoversAfterRelease);
    await t.test(fulfillmentReleaseBeforeClaimRejectsConflictingPending);
  });
}
