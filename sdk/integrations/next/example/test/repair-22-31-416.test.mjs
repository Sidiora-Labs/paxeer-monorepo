import assert from "node:assert/strict";
import { once } from "node:events";
import { chmod, mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import {
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  verifyRpcPayment,
} from "../../dist/index.js";
import {
  decodePaymentRequiredHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
} from "@sidiora/layerx-seller-middleware";
import { GET } from "../app/paid/route.js";
import { FileFulfillmentRepository } from "../lib/fulfillment.mjs";
import { layerx } from "../lib/layerx.js";

const vectors = JSON.parse(await readFile(new URL(
  "../../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const vector = vectors.cases.find(entry => entry.id === "V1-layerxledger-pay-exact");
assert.ok(vector);
assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
const bytes = value => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const log = vector.receiptJson.logs[0];
const requirements = {
  scheme: "exact", network: "eip155:125",
  amount: BigInt(`0x${log.data.slice(66)}`).toString(),
  asset: log.data.slice(0, 66), payTo: log.topics[3], maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", purposeHash: log.topics[1].slice(2) } },
};
const pin = {
  keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
};

async function paymentProof() {
  const { kind, verification, ...proof } = await verifyRpcPayment(
    vector.receiptJson, vector.rawTransaction, vector.receiptJson.from.slice(2).padStart(64, "0"),
    requirements, [pin],
  );
  assert.equal(kind, "verified");
  assert.equal(verification.txHash, vector.receiptJson.txHash);
  return proof;
}

async function proposal() {
  return {
    ...await paymentProof(), idempotencyKey: "ab".repeat(32), requestDigest: "cd".repeat(32),
    txHash: vector.receiptJson.txHash,
  };
}

function databaseRows(directory) {
  const database = new DatabaseSync(join(directory, "fulfillments.sqlite"));
  try {
    return {
      records: database.prepare("SELECT * FROM fulfillment").all(),
      claims: database.prepare("SELECT * FROM fulfillment_claim").all(),
    };
  } finally {
    database.close();
  }
}

async function finding022_nextPaidHandlerReplaysAfterReopen(t) {
  const root = await mkdtemp(join(tmpdir(), "next-repair-416-http-"));
  const directory = join(root, "state");
  const environment = {
    LAYERX_PRINCIPAL: "merchant", LAYERX_PROTECTED_PATH: "/paid",
    LAYERX_RESOURCE_URL: "https://merchant.example/paid", LAYERX_PROTOCOL_VERSION: "3",
    LAYERX_X402_SCHEME: "exact", LAYERX_X402_NETWORK: requirements.network,
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
  const requests = [];
  const errors = [];
  let server;
  async function closeServer() {
    if (server === undefined) return;
    const closed = new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
    server = undefined;
  }
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
  t.after(async () => {
    await closeServer();
    layerx.destroy();
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    await rm(root, { recursive: true, force: true });
  });

  let url = await openServer();
  const challenge = await fetch(url);
  assert.equal(challenge.status, 402);
  const offered = decodePaymentRequiredHeader(challenge.headers.get(PAYMENT_REQUIRED_HEADER));
  assert.deepEqual(await challenge.json(), offered);
  assert.deepEqual(offered.accepts, [requirements]);
  const proof = await paymentProof();
  const payload = { x402Version: 2, accepted: offered.accepts[0], resource: offered.resource, payload: proof };
  const payment = encodePaymentPayloadHeader(payload);
  const paid = async header => fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header } });
  const response = await paid(payment);
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("content-type"), "application/json");
  const body = await readFile(new URL("../resource.json", import.meta.url), "utf8");
  assert.equal(await response.text(), body);
  const settlementHeader = response.headers.get(PAYMENT_RESPONSE_HEADER);
  const settlement = decodeSettlementHeader(settlementHeader);
  assert.equal(settlement.success, true);
  assert.equal(settlement.transaction, vector.receiptJson.txHash);
  assert.equal(settlement.amount, requirements.amount);
  assert.equal(settlement.network, requirements.network);
  assert.deepEqual(settlement.extensions.layerx, proof);
  assert.deepEqual(requests[1], { method: "GET", path: "/paid", payment });
  const before = databaseRows(directory);
  assert.equal(before.records.length, 1);
  assert.deepEqual(before.claims, []);
  const row = before.records[0];
  const repository = new FileFulfillmentRepository(directory);
  const stored = await repository.get(row.id);
  assert.deepEqual(stored, {
    ...proof, idempotencyKey: row.id, requestDigest: row.request_digest,
    txHash: vector.receiptJson.txHash, resource: { contentType: "application/json", body },
  });
  stored.receiptKey.publicKey = "0x00";
  assert.deepEqual((await repository.get(row.id)).receiptKey, proof.receiptKey);

  await closeServer();
  layerx.destroy();
  url = await openServer();
  const replay = await paid(payment);
  assert.equal(replay.status, 200);
  assert.equal(await replay.text(), body);
  assert.equal(replay.headers.get(PAYMENT_RESPONSE_HEADER), settlementHeader);
  const conflict = await paid(encodePaymentPayloadHeader({
    ...payload, resource: { ...offered.resource, description: "changed request" },
  }));
  assert.equal(conflict.status, 409);
  const refused = decodeSettlementHeader(conflict.headers.get(PAYMENT_RESPONSE_HEADER));
  assert.equal(refused.success, false);
  assert.equal(refused.errorReason, "receipt_reuse");
  const tampered = bytes(proof.signedReceipt);
  tampered[tampered.length - 1] ^= 1;
  const invalid = await paid(encodePaymentPayloadHeader({
    ...payload, payload: { ...proof, signedReceipt: `0x${Buffer.from(tampered).toString("hex")}` },
  }));
  assert.equal(invalid.status, 402);
  assert.deepEqual(await invalid.json(), { error: "verification-failure" });
  assert.deepEqual(databaseRows(directory), before);
  assert.deepEqual(errors, []);
}

async function finding022_lookupPreservesConcurrentFulfillmentAndConflicts(t) {
  const root = await mkdtemp(join(tmpdir(), "next-repair-416-lookup-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, "state");
  const proposed = await proposal();
  let releases = 0;
  const release = async () => {
    releases += 1;
    return { contentType: "application/json", body: await readFile(new URL("../resource.json", import.meta.url), "utf8") };
  };
  const repository = new FileFulfillmentRepository(directory);
  assert.equal(await repository.get(proposed.idempotencyKey), undefined);
  const results = await Promise.all(Array.from({ length: 8 }, () =>
    new FileFulfillmentRepository(directory).fulfill(proposed, release)));
  assert.equal(releases, 1);
  for (const result of results) assert.deepEqual(result, results[0]);
  const reopened = new FileFulfillmentRepository(directory);
  assert.deepEqual(await reopened.get(proposed.idempotencyKey), results[0]);
  for (const changed of [
    { requestDigest: "ef".repeat(32) }, { receiptDigest: `0x${"ef".repeat(32)}` },
    { txHash: `0x${"ef".repeat(32)}` },
  ]) {
    await assert.rejects(reopened.fulfill({ ...proposed, ...changed }, release), { code: "fulfillment-conflict" });
  }
  assert.equal(releases, 1);
  await assert.rejects(reopened.get("../invalid"), { code: "fulfillment-conflict" });
  const database = new DatabaseSync(join(directory, "fulfillments.sqlite"));
  try {
    database.prepare("UPDATE fulfillment SET request_digest = ? WHERE id = ?").run("ef".repeat(32), proposed.idempotencyKey);
  } finally {
    database.close();
  }
  await assert.rejects(reopened.get(proposed.idempotencyKey), { code: "fulfillment-conflict" });
}

async function finding022_legacyRecordsBackfillVerifiedEvidenceWithoutRelease(t) {
  const root = await mkdtemp(join(tmpdir(), "next-repair-416-legacy-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const proposed = await proposal();
  const original = await new FileFulfillmentRepository(join(root, "producer")).fulfill(proposed, async () => ({
    contentType: "application/json", body: await readFile(new URL("../resource.json", import.meta.url), "utf8"),
  }));
  for (const format of ["sqlite", "json"]) {
    const directory = join(root, format);
    await mkdir(directory, { mode: 0o700 });
    if (format === "sqlite") {
      const path = join(directory, "fulfillments.sqlite");
      const database = new DatabaseSync(path);
      try {
        database.exec("CREATE TABLE fulfillment (id TEXT PRIMARY KEY, request_digest TEXT NOT NULL, receipt TEXT NOT NULL, resource TEXT NOT NULL)");
        database.prepare("INSERT INTO fulfillment VALUES (?, ?, ?, ?)").run(
          proposed.idempotencyKey, proposed.requestDigest, proposed.receiptDigest, JSON.stringify(original.resource),
        );
      } finally {
        database.close();
      }
      await chmod(path, 0o600);
    } else {
      await writeFile(join(directory, `${proposed.idempotencyKey}.json`), JSON.stringify({
        requestDigest: proposed.requestDigest, receipt: proposed.receiptDigest, resource: original.resource,
      }), { mode: 0o600 });
    }
    const repository = new FileFulfillmentRepository(directory);
    assert.equal(await repository.get(proposed.idempotencyKey), undefined);
    let releases = 0;
    const release = async () => {
      releases += 1;
      return { contentType: "application/json", body: await readFile(new URL("../resource.json", import.meta.url), "utf8") };
    };
    await assert.rejects(repository.fulfill({ ...proposed, requestDigest: "ef".repeat(32) }, release), { code: "fulfillment-conflict" });
    await assert.rejects(repository.fulfill({ ...proposed, receiptDigest: `0x${"ef".repeat(32)}` }, release), { code: "fulfillment-conflict" });
    assert.deepEqual(await repository.fulfill(proposed, release), original);
    assert.deepEqual(await new FileFulfillmentRepository(directory).get(proposed.idempotencyKey), original);
    assert.equal(releases, 0);
  }
}

async function finding022_lookupDoesNotReleaseUnresolvedClaims(t) {
  const root = await mkdtemp(join(tmpdir(), "next-repair-416-claim-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const directory = join(root, "state");
  const source = join(root, "resource.json");
  const proposed = await proposal();
  let releases = 0;
  const release = async () => {
    releases += 1;
    return { contentType: "application/json", body: await readFile(source, "utf8") };
  };
  await assert.rejects(new FileFulfillmentRepository(directory).fulfill(proposed, release), { code: "ENOENT" });
  const repository = new FileFulfillmentRepository(directory);
  assert.equal(await repository.get(proposed.idempotencyKey), undefined);
  assert.equal(databaseRows(directory).claims.length, 1);
  const body = await readFile(new URL("../resource.json", import.meta.url), "utf8");
  await writeFile(source, body);
  await assert.rejects(repository.fulfill(proposed, release), { code: "fulfillment-outcome-unknown" });
  await assert.rejects(repository.reconcile({ ...proposed, requestDigest: "ef".repeat(32) }, release), { code: "fulfillment-conflict" });
  assert.equal(releases, 1);
  const reconciled = await repository.reconcile(proposed, release);
  assert.equal(releases, 2);
  assert.deepEqual(reconciled, { ...proposed, resource: { contentType: "application/json", body } });
  await rm(source);
  const reopened = new FileFulfillmentRepository(directory);
  assert.deepEqual(await reopened.get(proposed.idempotencyKey), reconciled);
  assert.deepEqual(await reopened.fulfill(proposed, release), reconciled);
  assert.equal(releases, 2);
  assert.deepEqual(databaseRows(directory).claims, []);
}

test(finding022_nextPaidHandlerReplaysAfterReopen);
test(finding022_lookupPreservesConcurrentFulfillmentAndConflicts);
test(finding022_legacyRecordsBackfillVerifiedEvidenceWithoutRelease);
test(finding022_lookupDoesNotReleaseUnresolvedClaims);
