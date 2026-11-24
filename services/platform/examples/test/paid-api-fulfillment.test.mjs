import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { mkdir, mkdtemp, open, readFile, readdir, rm, stat, writeFile } from "node:fs/promises";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

const root = new URL("../../../../", import.meta.url);
const sources = new Map([
  ["@sidiora/layerx-sdk", new URL("services/agent/sdk/typescript/src/index.ts", root).href],
  ["@sidiora/layerx-seller-middleware", new URL("services/platform/middleware/seller/src/index.ts", root).href],
]);
const sourceRoots = [...sources.values()].map((url) => new URL("./", url).href);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (sources.has(specifier)) return { url: sources.get(specifier), shortCircuit: true };
    if (sourceRoots.some((url) => context.parentURL?.startsWith(url))
      && specifier.startsWith(".") && specifier.endsWith(".js")) {
      return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (sourceRoots.some((sourceRoot) => url.startsWith(sourceRoot)) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});

const { keccak256 } = await import("@sidiora/layerx-sdk");
const {
  PAYMENT_REQUIRED_HEADER, PAYMENT_RESPONSE_HEADER, PAYMENT_SIGNATURE_HEADER,
  ReceiptPayloadAuthority, SellerMiddleware, decodePaymentExactLog,
  decodePaymentPayloadHeader, decodePaymentRequiredHeader, decodeSettlementHeader,
  encodePaymentPayloadHeader,
} = await import("@sidiora/layerx-seller-middleware");
const { createPaidApiServer, FileFulfillmentRepository } = await import("../paid-api/index.mjs");
const vectors = JSON.parse(await readFile(new URL("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", root), "utf8"));
assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
const vector = vectors.cases.find(({ id }) => id === "V1-layerxledger-pay-exact");
assert.ok(vector);
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const log = vector.receiptJson.logs[0];
const paid = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
const pin = {
  keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
};
const proof = {
  format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
  rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
  receiptDigest: hex(keccak256(bytes(vector.signedReceipt))),
  receiptKey: {
    keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
    firstHeight: `0x${pin.first_height.toString(16)}`, lastHeight: `0x${pin.last_height.toString(16)}`,
  },
};
const offer = {
  x402Version: 2, resource: { url: "https://seller.example/paid", mimeType: "application/json" },
  accepts: [{ scheme: "exact", network: "eip155:125", amount: paid.amount.toString(),
    asset: hex(paid.assetId), payTo: hex(paid.to), maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer: hex(paid.from).slice(2),
      purposeHash: hex(paid.paymentReference).slice(2) } } }],
};
const principal = "public-paid-api";
const body = { weather: "sunny", temperature: 21, access: "receipt-verified" };

function seller(repository) {
  return new SellerMiddleware({ paymentRequired: offer, receiptKeys: [pin],
    authority: new ReceiptPayloadAuthority([pin]), fulfillments: repository });
}

function paymentHeader(description = "paid weather") {
  return encodePaymentPayloadHeader({ x402Version: 2, resource: { ...offer.resource, description },
    accepted: offer.accepts[0], payload: proof });
}

async function directoryFor(t) {
  const directory = await mkdtemp(join(tmpdir(), "paid-api-durable-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  return directory;
}

async function listen(t, middleware, resource = body) {
  const server = createPaidApiServer(middleware, resource);
  const requests = [];
  server.prependListener("request", (request) => {
    requests.push({ method: request.method, path: request.url,
      header: request.headers[PAYMENT_SIGNATURE_HEADER.toLowerCase()] });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  let closed = false;
  const close = async () => {
    if (closed) return;
    closed = true;
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  };
  t.after(close);
  return { url: `http://127.0.0.1:${server.address().port}/paid`, requests, close };
}

async function deliver(server, header = paymentHeader()) {
  return fetch(server.url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header }, signal: AbortSignal.timeout(5000) });
}

async function assertReleased(response, expected = body) {
  assert.equal(response.status, 200);
  assert.equal(response.headers.get("content-type"), "application/json");
  assert.deepEqual(await response.json(), expected);
  const settlement = decodeSettlementHeader(response.headers.get(PAYMENT_RESPONSE_HEADER));
  assert.equal(settlement.success, true);
  assert.equal(settlement.transaction, vector.receiptJson.txHash);
  assert.equal(settlement.network, offer.accepts[0].network);
  assert.equal(settlement.amount, offer.accepts[0].amount);
  assert.deepEqual(settlement.extensions.layerx, proof);
  return settlement;
}

async function paidApiConcurrentDeliveriesClaimBeforeRelease(t) {
  const directory = await directoryFor(t);
  const repository = new FileFulfillmentRepository(directory);
  const firstSeller = seller(repository);
  const secondSeller = seller(new FileFulfillmentRepository(join(directory, ".")));
  const started = Promise.withResolvers();
  const finish = Promise.withResolvers();
  let releases = 0;
  const release = async (key) => {
    releases += 1;
    const claim = JSON.parse(await readFile(join(directory, `${key}.claim`, "claim.json"), "utf8"));
    assert.equal(claim.idempotencyKey, key);
    assert.equal(claim.receiptDigest, proof.receiptDigest);
    assert.equal(claim.txHash, vector.receiptJson.txHash);
    assert.equal(Object.hasOwn(claim, "resource"), false);
    assert.match(claim.requestDigest, /^[0-9a-f]{64}$/u);
    await assert.rejects(readFile(join(directory, `${key}.json`)), { code: "ENOENT" });
    started.resolve(key);
    await finish.promise;
    return body;
  };
  const first = firstSeller.handle(principal, paymentHeader(), release);
  const key = await Promise.race([started.promise, first.then(() => { throw new Error("release did not execute"); })]);
  const second = secondSeller.handle(principal, paymentHeader(), release);
  const conflicting = secondSeller.handle(principal, paymentHeader("different request"), release);
  finish.resolve();
  const [a, b, conflict] = await Promise.all([first, second, conflicting]);
  assert.equal(a.kind, "released");
  assert.deepEqual(a, b);
  assert.equal(conflict.kind, "refused");
  assert.equal(conflict.status, 409);
  assert.equal(conflict.settlement.errorReason, "receipt_reuse");
  assert.equal(releases, 1);
  assert.deepEqual(await readdir(directory), [`${key}.json`]);
  assert.equal((await stat(join(directory, `${key}.json`))).mode & 0o777, 0o600);
  assert.deepEqual((await repository.get(key)).resource, body);
  const server = await listen(t, seller(new FileFulfillmentRepository(directory)), { incorrect: "second release" });
  await assertReleased(await deliver(server));
  assert.equal(releases, 1);
}

async function paidApiHttpConcurrentResponsesSurviveReopen(t) {
  const directory = await directoryFor(t);
  const server = await listen(t, seller(new FileFulfillmentRepository(directory)));
  const required = await fetch(server.url, { signal: AbortSignal.timeout(5000) });
  assert.equal(required.status, 402);
  assert.deepEqual(await required.json(), offer);
  assert.deepEqual(decodePaymentRequiredHeader(required.headers.get(PAYMENT_REQUIRED_HEADER)), offer);
  const header = paymentHeader();
  assert.deepEqual(decodePaymentPayloadHeader(header).payload, proof);
  const responses = await Promise.all(Array.from({ length: 12 }, () => deliver(server, header)));
  const settlements = await Promise.all(responses.map((response) => assertReleased(response)));
  for (const settlement of settlements) assert.deepEqual(settlement, settlements[0]);
  assert.deepEqual(server.requests, [{ method: "GET", path: "/paid", header: undefined },
    ...Array.from({ length: 12 }, () => ({ method: "GET", path: "/paid", header }))]);
  const [filename] = await readdir(directory);
  assert.match(filename, /^[0-9a-f]{64}\.json$/u);
  const persisted = JSON.parse(await readFile(join(directory, filename), "utf8"));
  assert.deepEqual(persisted.resource, body);
  await server.close();
  const reopened = await listen(t, seller(new FileFulfillmentRepository(directory)), { incorrect: "reopened release" });
  assert.deepEqual(await assertReleased(await deliver(reopened, header)), settlements[0]);
  assert.deepEqual(JSON.parse(await readFile(join(directory, filename), "utf8")), persisted);
}

async function paidApiInterruptedReleaseRequiresReconciliation(t) {
  const directory = await directoryFor(t);
  const repository = new FileFulfillmentRepository(directory);
  const middleware = seller(repository);
  const server = await listen(t, middleware);
  const resultFile = join(await directoryFor(t), "resource-result.json");
  let releases = 0;
  let key;
  await assert.rejects(middleware.handle(principal, paymentHeader(), async (idempotencyKey) => {
    releases += 1;
    key = idempotencyKey;
    const result = await open(resultFile, "wx", 0o600);
    try {
      await result.writeFile(JSON.stringify(body));
      await result.sync();
    } finally {
      await result.close();
    }
    throw new Error("interrupted after resource release");
  }), { message: "interrupted after resource release" });
  const claimPath = join(directory, `${key}.claim`, "claim.json");
  const claim = JSON.parse(await readFile(claimPath, "utf8"));
  assert.equal(claim.idempotencyKey, key);
  assert.equal(await repository.get(key), undefined);
  await server.close();
  const recoveredRepository = new FileFulfillmentRepository(directory);
  const recoveredSeller = seller(recoveredRepository);
  const reopened = await listen(t, recoveredSeller, { incorrect: "must never release again" });
  for (let attempt = 0; attempt < 2; attempt += 1) {
    const response = await deliver(reopened);
    assert.equal(response.status, 503);
    assert.equal(response.headers.get("retry-after"), "1");
    assert.equal(response.headers.get(PAYMENT_RESPONSE_HEADER), null);
    assert.deepEqual(await response.json(), { state: "unknown", error: "fulfillment-outcome-unknown" });
  }
  assert.deepEqual(JSON.parse(await readFile(claimPath, "utf8")), claim);
  await assert.rejects(recoveredRepository.reconcile(key, "wrong-digest", body), { code: "fulfillment-conflict" });
  const refused = await recoveredSeller.handle(principal, paymentHeader("changed request"), async () => {
    releases += 1;
    return body;
  });
  assert.equal(refused.status, 409);
  assert.equal(refused.settlement.errorReason, "receipt_reuse");
  const authoritativeResult = JSON.parse(await readFile(resultFile, "utf8"));
  const reconciled = await recoveredRepository.reconcile(key, claim.requestDigest, authoritativeResult);
  assert.deepEqual(reconciled.resource, body);
  await assertReleased(await deliver(reopened));
  const replayed = await recoveredSeller.handle(principal, paymentHeader(), async () => {
    releases += 1;
    return { incorrect: "repeat release" };
  });
  assert.equal(replayed.kind, "released");
  assert.deepEqual(replayed.resource, body);
  assert.equal(releases, 1);
  assert.deepEqual(await readdir(directory), [`${key}.json`]);
}

async function paidApiInterruptedPublicationRecoversCompleteRecord(t) {
  const directory = await directoryFor(t);
  const repository = new FileFulfillmentRepository(directory);
  const middleware = seller(repository);
  const server = await listen(t, middleware);
  const largeBody = { ...body, report: "weather-data".repeat(32768) };
  let releases = 0;
  let key;
  await assert.rejects(middleware.handle(principal, paymentHeader(), async (idempotencyKey) => {
    releases += 1;
    key = idempotencyKey;
    await mkdir(join(directory, `${key}.json`));
    return largeBody;
  }), { code: "EISDIR" });
  const readyPath = join(directory, `${key}.claim`, "ready.json");
  const ready = JSON.parse(await readFile(readyPath, "utf8"));
  assert.deepEqual(ready.resource, largeBody);
  assert.equal(ready.receiptDigest, proof.receiptDigest);
  assert.equal(ready.txHash, vector.receiptJson.txHash);
  await writeFile(join(directory, `${key}.claim`, "interrupted.tmp"), JSON.stringify(ready).slice(0, 100));
  await rm(join(directory, `${key}.json`), { recursive: true });
  await server.close();
  const reopenedRepository = new FileFulfillmentRepository(directory);
  const reopened = await listen(t, seller(reopenedRepository), { incorrect: "repeat release" });
  await assertReleased(await deliver(reopened), largeBody);
  assert.deepEqual(await reopenedRepository.get(key), ready);
  assert.deepEqual(JSON.parse(await readFile(join(directory, `${key}.json`), "utf8")), ready);
  assert.deepEqual(await readdir(directory), [`${key}.json`]);
  assert.equal(releases, 1);
}

async function paidApiIncompleteClaimNeverReleasesAgain(t) {
  const directory = await directoryFor(t);
  const repository = new FileFulfillmentRepository(directory);
  let key;
  let releases = 0;
  await assert.rejects(seller(repository).handle(principal, paymentHeader(), async (idempotencyKey) => {
    releases += 1;
    key = idempotencyKey;
    throw new Error("interrupted claim");
  }), { message: "interrupted claim" });
  const claimPath = join(directory, `${key}.claim`, "claim.json");
  const claim = await readFile(claimPath, "utf8");
  await writeFile(claimPath, claim.slice(0, 40));
  const reopened = await listen(t, seller(new FileFulfillmentRepository(directory)));
  const response = await deliver(reopened);
  assert.equal(response.status, 503);
  assert.equal(response.headers.get(PAYMENT_RESPONSE_HEADER), null);
  assert.deepEqual(await response.json(), { state: "unknown", error: "internal-fault" });
  assert.equal(releases, 1);
  assert.equal(await readFile(claimPath, "utf8"), claim.slice(0, 40));
  await assert.rejects(readFile(join(directory, `${key}.json`)), { code: "ENOENT" });
}

test(paidApiConcurrentDeliveriesClaimBeforeRelease);
test(paidApiHttpConcurrentResponsesSurviveReopen);
test(paidApiInterruptedReleaseRequiresReconciliation);
test(paidApiInterruptedPublicationRecoversCompleteRecord);
test(paidApiIncompleteClaimNeverReleasesAgain);
