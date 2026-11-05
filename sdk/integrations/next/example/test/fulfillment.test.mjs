import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import { keccak_256 as keccak256 } from "@noble/hashes/sha3.js";
import { FileFulfillmentRepository } from "../lib/fulfillment.mjs";

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
  const environment = await readFile(new URL("../../../../../services/platform/docs/samples/paid-route-next/.env", import.meta.url), "utf8");
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
    const child = spawn(process.execPath, ["--input-type=module", "-e", `
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
