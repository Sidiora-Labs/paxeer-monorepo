import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, readdir, rename, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { keccak256 } from "@sidiora/layerx-sdk";
import {
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
  verifyRpcPayment,
} from "@sidiora/layerx-seller-middleware";
import {
  DECLARED_KEYS,
  LayerXIntegrationError,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  SingleProcessWebhookDeliveryStore,
  mountLayerX,
  readDeclaredConfig,
} from "../dist/index.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const vectors = JSON.parse(await readFile(new URL(
  "../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const paymentVector = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(paymentVector);
assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
const publicKey = new Uint8Array(bytes(paymentVector.pinnedKey.publicKey));
const pin = {
  keyId: new Uint8Array(bytes(paymentVector.pinnedKey.keyId)), publicKey,
  first_height: BigInt(paymentVector.pinnedKey.firstHeight),
  last_height: BigInt(paymentVector.pinnedKey.lastHeight),
};
const paymentLog = paymentVector.receiptJson.logs[0];
const requirements = {
  scheme: "exact", network: "eip155:125",
  amount: BigInt(`0x${paymentLog.data.slice(66)}`).toString(),
  asset: paymentLog.data.slice(0, 66), payTo: paymentLog.topics[3], maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", purposeHash: paymentLog.topics[1].slice(2) } },
};

async function signedProof() {
  const expectedPayer = paymentVector.receiptJson.from.slice(2).padStart(64, "0");
  const { kind, verification, ...proof } = await verifyRpcPayment(
    paymentVector.receiptJson, paymentVector.rawTransaction, expectedPayer, requirements, [pin],
  );
  assert.equal(kind, "verified");
  assert.equal(verification.txHash, paymentVector.receiptJson.txHash);
  return proof;
}

function environment() {
  return {
    LAYERX_PRINCIPAL: "merchant",
    LAYERX_PROTECTED_PATH: "/paid",
    LAYERX_RESOURCE_URL: "https://merchant.example/paid",
    LAYERX_PROTOCOL_VERSION: "3",
    LAYERX_X402_SCHEME: "exact",
    LAYERX_X402_NETWORK: requirements.network,
    LAYERX_PRICE: requirements.amount,
    LAYERX_ASSET: requirements.asset,
    LAYERX_PAY_TO: requirements.payTo,
    LAYERX_PAYMENT_TIMEOUT_SECONDS: "120",
    LAYERX_PAYMENT_PURPOSE_HASH: requirements.extra.layerx.purposeHash,
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([{
      keyId: hex(pin.keyId), publicKey: hex(pin.publicKey), firstHeight: pin.first_height.toString(), lastHeight: pin.last_height.toString(),
    }]),
    LAYERX_WEBHOOK_PATH: "/webhook",
    LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ sequencer: Buffer.from(publicKey).toString("base64") }),
  };
}

class FileFulfillmentRepository {
  constructor(directory) {
    this.directory = directory;
  }

  path(idempotencyKey) {
    return join(this.directory, `${createHash("sha256").update(idempotencyKey).digest("hex")}.json`);
  }
  inFlight = new Map();

  async get(idempotencyKey) {
    try {
      return JSON.parse(await readFile(this.path(idempotencyKey), "utf8"));
    } catch (error) {
      if (error.code === "ENOENT") return undefined;
      throw error;
    }
  }

  async fulfill(proposed, release) {
    const existing = await this.get(proposed.idempotencyKey);
    if (existing !== undefined) {
      assert.equal(existing.requestDigest, proposed.requestDigest);
      return existing;
    }
    const pending = this.inFlight.get(proposed.idempotencyKey);
    if (pending !== undefined) {
      await pending;
      return this.fulfill(proposed, release);
    }
    const work = (async () => {
      const resource = await release();
      const stored = structuredClone({ ...proposed, resource });
      const path = this.path(proposed.idempotencyKey);
      await writeFile(`${path}.tmp`, JSON.stringify(stored), { flag: "wx" });
      await rename(`${path}.tmp`, path);
      return structuredClone(stored);
    })();
    this.inFlight.set(proposed.idempotencyKey, work);
    try {
      return await work;
    } finally {
      this.inFlight.delete(proposed.idempotencyKey);
    }
  }
}

function loadConfigReadsReceiptKeysJson() {
  const env = environment();
  assert.equal(Object.hasOwn(env, "LAYERX_AUTHORIZED_BATCH_JSON"), false);
  assert.equal(DECLARED_KEYS.includes("LAYERX_RECEIPT_KEYS_JSON"), true);
  assert.equal(DECLARED_KEYS.includes("LAYERX_AUTHORIZED_BATCH_JSON"), false);
  const config = readDeclaredConfig(env);
  assert.deepEqual(config.receiptKeys, [pin]);
  assert.deepEqual(config.requirements.extra, requirements.extra);
  assert.throws(() => readDeclaredConfig({ ...env, LAYERX_PAYMENT_PURPOSE_HASH: "00".repeat(32) }), {
    code: "invalid-declared-key", key: "LAYERX_PAYMENT_PURPOSE_HASH",
  });
  assert.equal(Object.hasOwn(config, "authorizedBatch"), false);
  const { LAYERX_RECEIPT_KEYS_JSON: omitted, ...missing } = env;
  assert.throws(() => readDeclaredConfig(missing), {
    code: "missing-declared-key", key: "LAYERX_RECEIPT_KEYS_JSON",
  });
  const entry = JSON.parse(omitted)[0];
  for (const value of ["{", "null", "{}", "[]", JSON.stringify([entry, entry]),
    JSON.stringify([{ ...entry, publicKey: "0x01" }]),
    JSON.stringify([{ ...entry, firstHeight: (pin.last_height + 1n).toString() }]),
    JSON.stringify([{ ...entry, firstHeight: "-1" }]),
    JSON.stringify([{ ...entry, firstHeight: 1000 }]),
    JSON.stringify([{ ...entry, lastHeight: "0x7cf" }]),
    JSON.stringify([{ ...entry, unexpected: "value" }]),
    JSON.stringify([{ keyId: entry.keyId, publicKey: entry.publicKey, firstHeight: "1" }]),
    JSON.stringify([{ ...entry, lastHeight: "18446744073709551616" }])]) {
    assert.throws(() => readDeclaredConfig({ ...env, LAYERX_RECEIPT_KEYS_JSON: value }),
      (error) => error instanceof LayerXIntegrationError && error.code === "invalid-declared-key");
  }
  const large = readDeclaredConfig({ ...env, LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([
    { ...entry, firstHeight: "9007199254740993", lastHeight: "18446744073709551615" },
  ]) });
  assert.equal(large.receiptKeys[0].first_height, 9007199254740993n);
  assert.equal(large.receiptKeys[0].last_height, 0xffff_ffff_ffff_ffffn);
}

async function nonExactConfigurationRequiresPurposeHash(context) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-next-config-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const options = configurationMountOptions(directory);
  for (const scheme of ["metered", "subscription"]) {
    const env = { ...environment(), LAYERX_X402_SCHEME: scheme };
    assert.deepEqual(readDeclaredConfig(env).requirements.extra, requirements.extra);
    delete env.LAYERX_PAYMENT_PURPOSE_HASH;
    for (const missing of [env, { ...env, LAYERX_PAYMENT_PURPOSE_HASH: "" }]) {
      for (const construct of [
        () => readDeclaredConfig(missing),
        () => mountLayerX({ ...options, environment: missing }),
      ]) {
        assert.throws(construct, (error) => {
          assert.ok(error instanceof LayerXIntegrationError);
          assert.equal(error.code, "missing-declared-key");
          assert.equal(error.key, "LAYERX_PAYMENT_PURPOSE_HASH");
          assert.equal(error.message, `missing-declared-key LAYERX_PAYMENT_PURPOSE_HASH for scheme ${scheme}`);
          return true;
        });
      }
    }
  }
}

async function exactConfigurationAcceptsMissingPurposeHash(context) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-next-config-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const options = configurationMountOptions(directory);
  const env = environment();
  delete env.LAYERX_PAYMENT_PURPOSE_HASH;
  for (const missing of [env, { ...env, LAYERX_PAYMENT_PURPOSE_HASH: "" }]) {
    const mounted = mountLayerX({ ...options, environment: missing });
    try {
      assert.equal(mounted.config.requirements.scheme, "exact");
      assert.equal(Object.hasOwn(mounted.config.requirements, "extra"), false);
    } finally {
      mounted.destroy();
    }
  }
}

function configurationMountOptions(directory) {
  const events = [];
  return {
    fulfillments: new FileFulfillmentRepository(directory),
    deliveries: new SingleProcessWebhookDeliveryStore(),
    resources: {
      async release() {
        return { contentType: "text/plain", body: "paid resource" };
      },
    },
    events: { async handle(event, deliveryId) { events.push({ event, deliveryId }); } },
  };
}

async function integrationSettlesSignedSequencerReceipt(context) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-next-receipt-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  let fulfillments = new FileFulfillmentRepository(directory);
  const released = [];
  const deliveries = new SingleProcessWebhookDeliveryStore();
  const events = [];
  const options = () => ({
    environment: environment(),
    fulfillments,
    deliveries,
    resources: {
      async release(request, idempotencyKey) {
        released.push({ url: request.url, idempotencyKey });
        return { contentType: "text/plain", body: "paid resource" };
      },
    },
    events: { async handle(event, deliveryId) { events.push({ event, deliveryId }); } },
  });
  let mounted = mountLayerX(options());
  const transportErrors = [];
  const server = createServer(async (request, response) => {
    try {
      const incoming = new Request(new URL(request.url, `http://${request.headers.host}`), {
        method: request.method, headers: request.headers,
      });
      const result = await mounted.resource.GET(incoming);
      response.writeHead(result.status, Object.fromEntries(result.headers));
      response.end(Buffer.from(await result.arrayBuffer()));
    } catch (error) {
      transportErrors.push(error);
      response.writeHead(500).end();
    }
  });
  context.after(async () => {
    const closed = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const url = `http://127.0.0.1:${server.address().port}/paid`;
  try {
    assert.deepEqual(mounted.config.receiptKeys, [pin]);
    const challenge = await fetch(url);
    assert.equal(challenge.status, 402);
    assert.equal(released.length, 0);
    const proof = await signedProof();
    const payment = encodePaymentPayloadHeader({
      x402Version: 2, accepted: mounted.config.requirements,
      resource: mounted.config.paymentRequired.resource, payload: proof,
    });
    const paidRequest = () => new Request(url, {
      headers: { [PAYMENT_SIGNATURE_HEADER]: payment },
    });
    const response = await fetch(paidRequest());
    assert.equal(response.status, 200);
    assert.equal(await response.text(), "paid resource");
    const settlement = decodeSettlementHeader(response.headers.get(PAYMENT_RESPONSE_HEADER));
    assert.equal(settlement.success, true);
    assert.equal(settlement.transaction, hex(keccak256(bytes(proof.rawTransaction))));
    assert.deepEqual(settlement.extensions.layerx, proof);
    assert.equal(settlement.amount, requirements.amount);
    assert.equal(settlement.network, requirements.network);
    assert.equal(released.length, 1);
    assert.equal((await readdir(directory)).length, 1);
    mounted.destroy();
    fulfillments = new FileFulfillmentRepository(directory);
    mounted = mountLayerX(options());
    const replay = await fetch(paidRequest());
    assert.equal(replay.status, 200);
    assert.equal(await replay.text(), "paid resource");
    assert.equal(released.length, 1);
    const unpinned = { ...proof, receiptKey: { ...proof.receiptKey, publicKey: hex(new Uint8Array(32).fill(7)) } };
    const refused = await fetch(new Request(url, {
      headers: { [PAYMENT_SIGNATURE_HEADER]: encodePaymentPayloadHeader({
        x402Version: 2, accepted: mounted.config.requirements, payload: unpinned,
      }) },
    }));
    assert.equal(refused.status, 402);
    assert.deepEqual(await refused.json(), { error: "verification-failure" });
    assert.equal(released.length, 1);
    assert.equal((await readdir(directory)).length, 1);
    assert.deepEqual(transportErrors, []);
  } finally {
    mounted.destroy();
  }
}

test(loadConfigReadsReceiptKeysJson);
test(nonExactConfigurationRequiresPurposeHash);
test(exactConfigurationAcceptsMissingPurposeHash);
test(integrationSettlesSignedSequencerReceipt);
