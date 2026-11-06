import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { once } from "node:events";
import { createServer } from "node:http";
import { keccak_256 as keccak256 } from "@noble/hashes/sha3.js";

const sourceRuntimeUrl = new URL("./source-runtime.mjs", import.meta.url).href;
await import(sourceRuntimeUrl);

const { FileFulfillmentRepository } = await import("../lib/fulfillment.mjs");
const { GET } = await import("../app/paid/route.js");
const { layerx } = await import("../lib/layerx.js");
const {
  PAYMENT_REQUIRED_HEADER, PAYMENT_SIGNATURE_HEADER, PAYMENT_RESPONSE_HEADER,
  decodePaymentRequiredHeader, decodePaymentPayloadHeader, decodeSettlementHeader,
  encodePaymentPayloadHeader, verifyRpcPayment,
} = await import("@sidiora/layerx-seller-middleware");

const fixture = JSON.parse(await readFile(new URL("../../../../platform/conformance/fixtures/vector-v5-portable.json", import.meta.url), "utf8"));
const proof = fixture.bundles.find(value => value.verificationLevel === "receipted");
const hex = value => `0x${Buffer.from(value).toString("hex")}`;
const proposed = {
  ...proof,
  idempotencyKey: "ab".repeat(32), requestDigest: "cd".repeat(32),
  receiptDigest: hex(keccak256(Buffer.from(proof.signedReceipt.slice(2), "hex"))),
  txHash: hex(keccak256(Buffer.from(proof.rawTransaction.slice(2), "hex"))),
};

async function parseReceiptKeys() {
  const environment = await readFile(new URL("../../../../../services/platform/examples/samples/paid-route-next/.env", import.meta.url), "utf8");
  const declared = environment.split("\n").find(line => line.startsWith("LAYERX_RECEIPT_KEYS_JSON="));
  assert.ok(declared);
  const keys = JSON.parse(declared.slice(declared.indexOf("=") + 1));
  assert.ok(Array.isArray(keys));
  assert.ok(keys.length >= 1 && keys.length <= 32);
  const seen = new Set();
  for (const key of keys) {
    assert.deepEqual(Object.keys(key).sort(), ["firstHeight", "keyId", "lastHeight", "publicKey"]);
    for (const field of ["keyId", "publicKey"]) {
      assert.match(key[field], /^(?:0x)?[0-9a-fA-F]{64}$/u);
      assert.notEqual(BigInt(`0x${key[field].replace(/^0x/u, "")}`), 0n);
    }
    const keyId = key.keyId.replace(/^0x/u, "").toLowerCase();
    assert.ok(!seen.has(keyId));
    seen.add(keyId);
    for (const field of ["firstHeight", "lastHeight"]) {
      assert.match(key[field], /^(0|[1-9][0-9]*)$/u);
      assert.ok(BigInt(key[field]) <= 0xffff_ffff_ffff_ffffn);
    }
    assert.ok(BigInt(key.firstHeight) <= BigInt(key.lastHeight));
  }
}

test(parseReceiptKeys);

test("concurrent fulfillment and reopen release one durable resource", async () => {
  const root = await mkdtemp(join(tmpdir(), "next-fulfillment-"));
  try {
    const source = join(root, "resource.txt");
    await writeFile(source, "paid resource");
    let releases = 0;
    const release = async () => {
      releases += 1;
      return { contentType: "text/plain", body: await readFile(source, "utf8") };
    };
    const repository = new FileFulfillmentRepository(join(root, "state"));
    const results = await Promise.all(Array.from({ length: 16 }, () => repository.fulfill(proposed, release)));
    assert.equal(releases, 1);
    assert(results.every(value => value.receiptDigest === proposed.receiptDigest));
    assert(results.every(value => value.resource.body === "paid resource"));
    const reopened = new FileFulfillmentRepository(join(root, "state"));
    assert.equal((await reopened.fulfill(proposed, release)).resource.body, "paid resource");
    assert.equal(releases, 1);
    const database = new DatabaseSync(join(root, "state", "fulfillments.sqlite"));
    try {
      const stored = database.prepare("SELECT request_digest, receipt, resource FROM fulfillment WHERE id = ?").get(proposed.idempotencyKey);
      assert.equal(stored.receipt, proposed.receiptDigest);
      assert.equal(stored.request_digest, proposed.requestDigest);
      assert.equal(JSON.parse(stored.resource).body, "paid resource");
      assert.equal(database.prepare("SELECT COUNT(*) AS count FROM fulfillment_claim").get().count, 0);
    } finally {
      database.close();
    }
    await assert.rejects(reopened.fulfill({ ...proposed, requestDigest: "ef".repeat(32) }, release), { code: "fulfillment-conflict" });
    await assert.rejects(reopened.fulfill({ ...proposed, receiptDigest: `0x${"ef".repeat(32)}` }, release), { code: "fulfillment-conflict" });
    assert.equal(releases, 1);
  } finally {
    await rm(root, { recursive: true });
  }
});

test("legacy committed fulfillment migrates without another release", async () => {
  const root = await mkdtemp(join(tmpdir(), "next-fulfillment-legacy-"));
  try {
    const directory = join(root, "state");
    await mkdir(directory, { mode: 0o700 });
    const resource = { contentType: "text/plain", body: "previously released" };
    await writeFile(join(directory, `${proposed.idempotencyKey}.json`), JSON.stringify({
      requestDigest: proposed.requestDigest,
      receipt: proposed.receiptDigest,
      resource,
    }), { mode: 0o600 });
    const repository = new FileFulfillmentRepository(directory);
    await assert.rejects(repository.fulfill({ ...proposed, receiptDigest: `0x${"ef".repeat(32)}` }, async () => resource), { code: "fulfillment-conflict" });
    let releases = 0;
    const release = async () => { releases += 1; return resource; };
    assert.deepEqual((await repository.fulfill(proposed, release)).resource, resource);
    assert.deepEqual((await new FileFulfillmentRepository(directory).fulfill(proposed, release)).resource, resource);
    assert.equal(releases, 0);
  } finally {
    await rm(root, { recursive: true });
  }
});

test("failed resource read requires explicit reconciliation before a real retry", async () => {
  const root = await mkdtemp(join(tmpdir(), "next-fulfillment-retry-"));
  try {
    const source = join(root, "resource.txt");
    const directory = join(root, "state");
    const release = async () => ({ contentType: "text/plain", body: await readFile(source, "utf8") });
    await assert.rejects(new FileFulfillmentRepository(directory).fulfill(proposed, release), { code: "ENOENT" });
    const database = new DatabaseSync(join(directory, "fulfillments.sqlite"));
    try {
      assert.equal(database.prepare("SELECT receipt FROM fulfillment_claim WHERE id = ?").get(proposed.idempotencyKey).receipt, proposed.receiptDigest);
      assert.equal(database.prepare("SELECT COUNT(*) AS count FROM fulfillment").get().count, 0);
    } finally {
      database.close();
    }
    await assert.rejects(new FileFulfillmentRepository(directory).reconcile({ ...proposed, receiptDigest: `0x${"ef".repeat(32)}` }, release), { code: "fulfillment-conflict" });
    await writeFile(source, "available after retry");
    await assert.rejects(new FileFulfillmentRepository(directory).fulfill(proposed, release), { code: "fulfillment-outcome-unknown" });
    await new FileFulfillmentRepository(directory).reconcile(proposed, release);
    assert.equal((await new FileFulfillmentRepository(directory).fulfill(proposed, release)).resource.body, "available after retry");
  } finally {
    await rm(root, { recursive: true });
  }
});

test("a process crash after release retains its durable claim until actual reconciliation", async () => {
  const { spawn } = await import("node:child_process");
  const { once } = await import("node:events");
  const root = await mkdtemp(join(tmpdir(), "next-fulfillment-crash-"));
  try {
    const source = join(root, "released.json");
    const directory = join(root, "state");
    const input = join(root, "input.json");
    await writeFile(input, JSON.stringify(proposed));
    const module = new URL("../lib/fulfillment.mjs", import.meta.url).href;
    const child = spawn(process.execPath, ["--import", sourceRuntimeUrl, "--input-type=module", "-e", `
      import { readFile, open } from "node:fs/promises";
      import { FileFulfillmentRepository } from ${JSON.stringify(module)};
      const proposed = JSON.parse(await readFile(process.argv[1], "utf8"));
      await new FileFulfillmentRepository(process.argv[2]).fulfill(proposed, async () => {
        const file = await open(process.argv[3], "wx", 0o600);
        await file.writeFile(JSON.stringify({ contentType: "text/plain", body: "released before crash" }));
        await file.sync();
        await file.close();
        process.kill(process.pid, "SIGKILL");
      });
    `, input, directory, source], { stdio: ["ignore", "ignore", "inherit"] });
    const [code, signal] = await once(child, "exit");
    assert.equal(code, null);
    assert.equal(signal, "SIGKILL");
    const released = JSON.parse(await readFile(source, "utf8"));
    const repository = new FileFulfillmentRepository(directory);
    const secondRelease = async () => {
      await writeFile(source, "duplicate", { flag: "wx" });
      return released;
    };
    await assert.rejects(repository.fulfill(proposed, secondRelease), { code: "fulfillment-outcome-unknown" });
    assert.deepEqual(JSON.parse(await readFile(source, "utf8")), released);
    await assert.rejects(repository.reconcile({ ...proposed, requestDigest: "ef".repeat(32) }, async () => released), { code: "fulfillment-conflict" });
    assert.deepEqual((await repository.reconcile(proposed, async () => JSON.parse(await readFile(source, "utf8")))).resource, released);
    assert.deepEqual((await new FileFulfillmentRepository(directory).fulfill(proposed, secondRelease)).resource, released);
    assert.deepEqual(JSON.parse(await readFile(source, "utf8")), released);
  } finally {
    await rm(root, { recursive: true });
  }
});

async function finding023_mountedPaidRouteReplaysCurrentEvidence(t) {
  const vectors = JSON.parse(await readFile(new URL(
    "../../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
  ), "utf8"));
  assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const vector = vectors.cases.find(value => value.id === "V1-layerxledger-pay-exact");
  assert.ok(vector);
  const bytes = value => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
  const pin = {
    keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
    first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
  };
  const log = vector.receiptJson.logs[0];
  const requirements = {
    scheme: "exact", network: "eip155:125", amount: BigInt(`0x${log.data.slice(66)}`).toString(),
    asset: log.data.slice(0, 66), payTo: log.topics[3], maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", purposeHash: log.topics[1].slice(2) } },
  };
  const root = await mkdtemp(join(tmpdir(), "next-finding-023-"));
  const directory = join(root, "state");
  const environment = {
    LAYERX_PRINCIPAL: "merchant", LAYERX_PROTECTED_PATH: "/paid", LAYERX_PROTOCOL_VERSION: "3",
    LAYERX_RESOURCE_URL: "https://merchant.example/paid",
    LAYERX_X402_SCHEME: requirements.scheme, LAYERX_X402_NETWORK: requirements.network,
    LAYERX_PRICE: requirements.amount, LAYERX_ASSET: requirements.asset, LAYERX_PAY_TO: requirements.payTo,
    LAYERX_PAYMENT_TIMEOUT_SECONDS: "120", LAYERX_PAYMENT_PURPOSE_HASH: requirements.extra.layerx.purposeHash,
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([{
      ...vector.pinnedKey, firstHeight: pin.first_height.toString(), lastHeight: pin.last_height.toString(),
    }]),
    LAYERX_WEBHOOK_PATH: "/layerx/webhooks",
    LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ sequencer: Buffer.from(pin.publicKey).toString("base64") }),
    LAYERX_FULFILLMENT_DIR: directory,
  };
  const previous = Object.fromEntries(Object.keys(environment).map(key => [key, process.env[key]]));
  Object.assign(process.env, environment);
  layerx.destroy();
  let server;
  const requests = [];
  const errors = [];
  async function closeServer() {
    if (server === undefined) return;
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
    server = undefined;
  }
  t.after(async () => {
    await closeServer();
    layerx.destroy();
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    await rm(root, { recursive: true, force: true });
  });
  async function openServer() {
    server = createServer(async (request, response) => {
      try {
        requests.push({ method: request.method, path: request.url, payment: request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()] });
        const result = await GET(new Request(new URL(request.url, `http://${request.headers.host}`), {
          method: request.method, headers: request.headers,
        }));
        response.writeHead(result.status, Object.fromEntries(result.headers));
        response.end(Buffer.from(await result.arrayBuffer()));
      } catch (error) {
        errors.push(error);
        response.writeHead(500).end();
      }
    });
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    return `http://127.0.0.1:${server.address().port}/paid`;
  }
  function rows() {
    const database = new DatabaseSync(join(directory, "fulfillments.sqlite"));
    try {
      return {
        fulfillments: database.prepare("SELECT * FROM fulfillment").all(),
        claims: database.prepare("SELECT * FROM fulfillment_claim").all(),
      };
    } finally {
      database.close();
    }
  }

  let url = await openServer();
  const challenge = await fetch(url);
  assert.equal(challenge.status, 402);
  const offered = decodePaymentRequiredHeader(challenge.headers.get(PAYMENT_REQUIRED_HEADER));
  assert.deepEqual(await challenge.json(), offered);
  assert.deepEqual(offered, {
    x402Version: 2, resource: { url: environment.LAYERX_RESOURCE_URL }, accepts: [requirements],
  });
  // The real RPC proof producer verifies the recorded node receipt and emits the
  // current portable buyer evidence; no proposed repository record is supplied.
  const { kind, verification, ...evidence } = await verifyRpcPayment(
    vector.receiptJson, vector.rawTransaction, vector.receiptJson.from.slice(2).padStart(64, "0"),
    offered.accepts[0], [pin],
  );
  assert.equal(kind, "verified");
  assert.equal(verification.txHash, vector.receiptJson.txHash);
  assert.equal(evidence.format, "layerx-receipt-proof-v2");
  assert.equal(evidence.verificationLevel, "receipted");
  const payload = { x402Version: 2, accepted: offered.accepts[0], resource: offered.resource, payload: evidence };
  const payment = encodePaymentPayloadHeader(payload);
  assert.deepEqual(decodePaymentPayloadHeader(payment), payload);
  const paid = header => fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header } });
  const resource = await readFile(new URL("../resource.json", import.meta.url), "utf8");
  const responses = await Promise.all(Array.from({ length: 8 }, () => paid(payment)));
  assert.deepEqual(errors, []);
  let settlementHeader;
  for (const response of responses) {
    assert.equal(response.status, 200);
    assert.equal(response.headers.get("content-type"), "application/json");
    assert.equal(await response.text(), resource);
    const header = response.headers.get(PAYMENT_RESPONSE_HEADER);
    if (settlementHeader === undefined) settlementHeader = header;
    assert.equal(header, settlementHeader);
    const settlement = decodeSettlementHeader(header);
    assert.equal(settlement.success, true);
    assert.equal(settlement.transaction, vector.receiptJson.txHash);
    assert.equal(settlement.amount, requirements.amount);
    assert.equal(settlement.network, requirements.network);
    assert.deepEqual(settlement.extensions.layerx, evidence);
  }
  assert.deepEqual(requests[0], { method: "GET", path: "/paid", payment: undefined });
  assert.equal(requests.length, 9);
  for (const request of requests.slice(1)) assert.deepEqual(request, { method: "GET", path: "/paid", payment });
  const before = rows();
  assert.equal(before.fulfillments.length, 1);
  assert.deepEqual(before.claims, []);
  const row = before.fulfillments[0];
  const stored = await new FileFulfillmentRepository(directory).get(row.id);
  assert.deepEqual(stored, {
    ...evidence, idempotencyKey: row.id, requestDigest: row.request_digest,
    txHash: vector.receiptJson.txHash, resource: { contentType: "application/json", body: resource },
  });

  await closeServer();
  layerx.destroy();
  url = await openServer();
  const replay = await paid(payment);
  assert.equal(replay.status, 200);
  assert.equal(replay.headers.get("content-type"), "application/json");
  assert.equal(await replay.text(), resource);
  assert.equal(replay.headers.get(PAYMENT_RESPONSE_HEADER), settlementHeader);
  assert.deepEqual(await new FileFulfillmentRepository(directory).get(row.id), stored);
  assert.deepEqual(rows(), before);
  const conflict = await paid(encodePaymentPayloadHeader({
    ...payload, resource: { ...offered.resource, description: "different request" },
  }));
  assert.equal(conflict.status, 409);
  const refusal = decodeSettlementHeader(conflict.headers.get(PAYMENT_RESPONSE_HEADER));
  assert.equal(refusal.success, false);
  assert.equal(refusal.errorReason, "receipt_reuse");
  await conflict.arrayBuffer();
  const corrupted = bytes(evidence.signedReceipt);
  corrupted[corrupted.length - 1] ^= 1;
  const invalid = await paid(encodePaymentPayloadHeader({
    ...payload, payload: { ...evidence, signedReceipt: hex(corrupted) },
  }));
  assert.equal(invalid.status, 402);
  assert.deepEqual(await invalid.json(), { error: "verification-failure" });
  assert.deepEqual(rows(), before);
  assert.deepEqual(requests.slice(9), [
    { method: "GET", path: "/paid", payment },
    {
      method: "GET", path: "/paid", payment: encodePaymentPayloadHeader({
        ...payload, resource: { ...offered.resource, description: "different request" },
      }),
    },
    {
      method: "GET", path: "/paid", payment: encodePaymentPayloadHeader({
        ...payload, payload: { ...evidence, signedReceipt: hex(corrupted) },
      }),
    },
  ]);
  assert.deepEqual(errors, []);
}

test(finding023_mountedPaidRouteReplaysCurrentEvidence);
