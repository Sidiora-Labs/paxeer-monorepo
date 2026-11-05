import assert from "node:assert/strict";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import express from "express";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import { decodeRawEip1559, keccak256 } from "@sidiora/layerx-sdk";
import {
  LAYERX_LEDGER_ADDRESS,
  decodePaymentRequiredHeader,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
  verifyRpcPayment,
} from "@sidiora/layerx-seller-middleware";
import {
  DECLARED_KEYS,
  LAYERX_RECEIPT_KEYS_JSON,
  LayerXIntegrationError,
  MiddlewareError,
  PAYMENT_REQUIRED_HEADER,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  SingleProcessWebhookDeliveryStore,
  mountLayerX,
  offerPurposeHash,
  readDeclaredConfig,
} from "../dist/index.js";

const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const sha256 = (value) => createHash("sha256").update(value).digest();
const word = (value) => Buffer.alloc(32, value);
const asset = word(0x11);
const payee = word(0x22);
const payer = word(0x33);
const resourceUrl = "https://merchant.example/paid";
const reference = sha256(`layerx-express-purpose:${resourceUrl}`);
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

function transaction(amount) {
  const selector = keccak256(new TextEncoder().encode("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")).slice(0, 4);
  const data = Buffer.concat([selector, payer, payee, asset, uint(amount, 32), uint(1n, 32), reference]);
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
  const rawTransaction = transaction(amount);
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

function environment(keys = [pin]) {
  return {
    LAYERX_PRINCIPAL: "merchant",
    LAYERX_PROTECTED_PATH: "/paid",
    LAYERX_RESOURCE_URL: resourceUrl,
    LAYERX_PROTOCOL_VERSION: "2",
    LAYERX_X402_SCHEME: "exact",
    LAYERX_X402_NETWORK: "eip155:125",
    LAYERX_PRICE: "25",
    LAYERX_ASSET: hex(asset),
    LAYERX_PAY_TO: hex(payee),
    LAYERX_PAYMENT_TIMEOUT_SECONDS: "120",
    [LAYERX_RECEIPT_KEYS_JSON]: JSON.stringify(keys.map((key) => ({
      keyId: hex(key.keyId),
      publicKey: hex(key.publicKey),
      firstHeight: key.first_height.toString(),
      lastHeight: key.last_height.toString(),
    }))),
    LAYERX_WEBHOOK_PATH: "/webhook",
    LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ sequencer: Buffer.from(publicKey).toString("base64") }),
  };
}

function invalidConfig(value) {
  assert.throws(() => readDeclaredConfig({ ...environment(), [LAYERX_RECEIPT_KEYS_JSON]: value }),
    (error) => error instanceof LayerXIntegrationError && error.code === "invalid-declared-key");
}

function loadConfigReadsReceiptKeysJson() {
  const env = environment();
  const config = readDeclaredConfig(env);
  assert.equal(offerPurposeHash(resourceUrl), reference.toString("hex"));
  assert.match(offerPurposeHash(resourceUrl), /^[0-9a-f]{64}$/u);
  assert.notEqual(offerPurposeHash(resourceUrl), "0".repeat(64));
  const otherUrl = `${resourceUrl}/other`;
  assert.equal(readDeclaredConfig({ ...env, LAYERX_RESOURCE_URL: otherUrl }).requirements.extra.layerx.purposeHash,
    sha256(`layerx-express-purpose:${otherUrl}`).toString("hex"));
  assert.deepEqual(config.receiptKeys, [pin]);
  assert.equal(Object.hasOwn(config, "authorizedBatch"), false);
  assert.ok(DECLARED_KEYS.includes(LAYERX_RECEIPT_KEYS_JSON));
  assert.equal(DECLARED_KEYS.includes("LAYERX_AUTHORIZED_BATCH_JSON"), false);
  const oldOnly = { ...env, LAYERX_AUTHORIZED_BATCH_JSON: "{}" };
  delete oldOnly[LAYERX_RECEIPT_KEYS_JSON];
  assert.throws(() => readDeclaredConfig(oldOnly),
    (error) => error instanceof LayerXIntegrationError && error.code === "missing-declared-key");
  assert.deepEqual(readDeclaredConfig({ ...env, LAYERX_AUTHORIZED_BATCH_JSON: "invalid" }).receiptKeys, [pin]);
  const rotated = { ...pin, keyId: new Uint8Array(32).fill(0x66), first_height: 1001n,
    last_height: 0xffff_ffff_ffff_ffffn };
  assert.deepEqual(readDeclaredConfig(environment([pin, rotated])).receiptKeys, [pin, rotated]);
  assert.equal(readDeclaredConfig(environment([{ ...pin, first_height: 0n }])).receiptKeys[0].first_height, 0n);
  for (const value of ["not json", "null", "{}", "[]", "[null]", "[1]", "[[]]"]) invalidConfig(value);
  const [encoded] = JSON.parse(env[LAYERX_RECEIPT_KEYS_JSON]);
  for (const changes of [{ keyId: "00" }, { publicKey: "zz".repeat(32) }, { firstHeight: 1 },
    { firstHeight: "-1" }, { firstHeight: "01" }, { firstHeight: "1001" },
    { lastHeight: "18446744073709551616" }, { lastHeight: "1.5" },
    { first_height: "1" }, { last_height: "1000" }, { extra: true }]) {
    invalidConfig(JSON.stringify([{ ...encoded, ...changes }]));
  }
  for (const field of Object.keys(encoded)) {
    const missing = { ...encoded };
    delete missing[field];
    invalidConfig(JSON.stringify([missing]));
  }
  invalidConfig(JSON.stringify(Array.from({ length: 33 }, () => encoded)));
}

class FileFulfillmentRepository {
  #directory;
  #pending = new Map();

  constructor(directory) {
    this.#directory = directory;
  }

  async get(key) {
    assert.match(key, /^[0-9a-f]{64}$/u);
    try {
      return JSON.parse(await readFile(join(this.#directory, `${key}.json`), "utf8"));
    } catch (error) {
      if (error.code === "ENOENT") return undefined;
      throw error;
    }
  }

  async fulfill(proposed, release) {
    const pending = this.#pending.get(proposed.idempotencyKey);
    if (pending !== undefined) await pending;
    const operation = (async () => {
      const stored = await this.get(proposed.idempotencyKey);
      if (stored !== undefined) {
        if (stored.requestDigest !== proposed.requestDigest) throw new MiddlewareError("fulfillment-conflict");
        return stored;
      }
      const record = { ...proposed, resource: await release() };
      await writeFile(join(this.#directory, `${proposed.idempotencyKey}.json`), JSON.stringify(record),
        { flag: "wx", mode: 0o600 });
      return record;
    })();
    this.#pending.set(proposed.idempotencyKey, operation);
    try {
      return await operation;
    } finally {
      if (this.#pending.get(proposed.idempotencyKey) === operation) this.#pending.delete(proposed.idempotencyKey);
    }
  }
}

async function integrationSettlesSignedSequencerReceipt() {
  const directory = await mkdtemp(join(tmpdir(), "layerx-express-receipt-"));
  const resource = { contentType: "application/json", body: JSON.stringify({ access: "paid-resource" }) };
  const releases = [];
  const events = [];
  const fulfillments = new FileFulfillmentRepository(directory);
  function startServer(env) {
    const app = express();
    const mount = mountLayerX(app, {
      environment: env,
      resources: {
        async release(request, idempotencyKey) {
          releases.push({ method: request.method, idempotencyKey });
          return resource;
        },
      },
      fulfillments: new FileFulfillmentRepository(directory),
      deliveries: new SingleProcessWebhookDeliveryStore(),
      events: {
        async handle(event, deliveryId) {
          events.push({ event, deliveryId });
        },
      },
    });
    return { mount, server: app.listen(0, "127.0.0.1") };
  }
  let { mount, server } = startServer(environment());
  async function closeServer() {
    mount.destroy();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
  try {
    await once(server, "listening");
    assert.deepEqual(mount.config.receiptKeys, [pin]);
    const url = `http://127.0.0.1:${server.address().port}/paid`;
    const challenge = await fetch(url);
    assert.equal(challenge.status, 402);
    const offer = decodePaymentRequiredHeader(challenge.headers.get(PAYMENT_REQUIRED_HEADER));
    assert.deepEqual(offer, mount.config.paymentRequired);
    assert.equal(offer.accepts[0].extra.layerx.commitment, "receipted");
    assert.equal(offer.accepts[0].extra.layerx.purposeHash, offerPurposeHash(environment().LAYERX_RESOURCE_URL));
    assert.equal(releases.length, 0);
    const proof = signedProof();
    const header = (evidence) => encodePaymentPayloadHeader({
      x402Version: 2, accepted: offer.accepts[0], resource: offer.resource, payload: evidence,
    });
    const badReceipt = bytes(proof.signedReceipt);
    badReceipt[badReceipt.length - 1] ^= 1;
    const invalidProofs = [
      { ...proof, signedReceipt: hex(badReceipt), receiptDigest: hex(keccak256(badReceipt)) },
      { ...proof, receiptKey: { ...proof.receiptKey, keyId: hex(word(0x66)) } },
      { ...proof, receiptKey: { ...proof.receiptKey, publicKey: hex(word(0x88)) } },
      { ...proof, receiptKey: { ...proof.receiptKey, firstHeight: "0x0" } },
      { ...proof, receiptKey: { ...proof.receiptKey, lastHeight: "0x3e9" } },
      signedProof({ height: 1001n }),
    ];
    for (const invalid of invalidProofs) {
      const response = await fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header(invalid) } });
      assert.equal(response.status, 402);
      assert.equal(response.headers.get(PAYMENT_RESPONSE_HEADER), null);
      assert.deepEqual(await response.json(), { error: "verification-failure" });
      assert.equal(releases.length, 0);
    }
    let firstSettlement;
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const response = await fetch(url, { headers: { [PAYMENT_SIGNATURE_HEADER]: header(proof) } });
      assert.equal(response.status, 200);
      assert.equal(await response.text(), resource.body);
      const settlement = decodeSettlementHeader(response.headers.get(PAYMENT_RESPONSE_HEADER));
      assert.equal(settlement.success, true);
      assert.equal(settlement.transaction, hex(keccak256(bytes(proof.rawTransaction))));
      assert.equal(settlement.amount, "25");
      assert.equal(settlement.network, "eip155:125");
      assert.deepEqual(settlement.extensions.layerx, proof);
      assert.equal(releases.length, 1);
      if (attempt === 0) firstSettlement = settlement;
    }
    const stored = await fulfillments.get(releases[0].idempotencyKey);
    assert.equal(stored.receiptDigest, proof.receiptDigest);
    assert.deepEqual(stored.resource, resource);

    const vectors = JSON.parse(await readFile(new URL(
      "../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
    ), "utf8"));
    assert.equal(vectors.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
    const payment = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
    assert.ok(payment);
    const recordedPin = {
      keyId: new Uint8Array(bytes(payment.pinnedKey.keyId)),
      publicKey: new Uint8Array(bytes(payment.pinnedKey.publicKey)),
      first_height: BigInt(payment.pinnedKey.firstHeight),
      last_height: BigInt(payment.pinnedKey.lastHeight),
    };
    const log = payment.receiptJson.logs[0];
    const recordedEnvironment = {
      ...environment([recordedPin]),
      LAYERX_ASSET: log.data.slice(0, 66),
      LAYERX_PAY_TO: log.topics[3],
      LAYERX_PRICE: BigInt(`0x${log.data.slice(66)}`).toString(),
    };
    const produced = await verifyRpcPayment(payment.receiptJson, payment.rawTransaction,
      payment.receiptJson.from.slice(2).padStart(64, "0"),
      { ...readDeclaredConfig(recordedEnvironment).requirements,
        extra: { layerx: { commitment: "receipted", purposeHash: log.topics[1].slice(2),
          payer: log.topics[2].slice(2) } } }, [recordedPin]);
    assert.equal(produced.kind, "verified");
    const portable = {
      format: produced.format,
      verificationLevel: produced.verificationLevel,
      rawTransaction: produced.rawTransaction,
      signedReceipt: produced.signedReceipt,
      receiptDigest: produced.receiptDigest,
      receiptKey: produced.receiptKey,
    };
    assert.equal(portable.signedReceipt, payment.signedReceipt);
    assert.equal(portable.receiptDigest, hex(keccak256(bytes(payment.signedReceipt))));
    for (let restart = 0; restart < 2; restart += 1) {
      await closeServer();
      ({ mount, server } = startServer(environment()));
      await once(server, "listening");
      const restartedUrl = `http://127.0.0.1:${server.address().port}/paid`;
      const requiredResponse = await fetch(restartedUrl);
      assert.equal(requiredResponse.status, 402);
      const required = decodePaymentRequiredHeader(requiredResponse.headers.get(PAYMENT_REQUIRED_HEADER));
      assert.deepEqual(required, offer);
      const response = await fetch(restartedUrl, { headers: {
        [PAYMENT_SIGNATURE_HEADER]: encodePaymentPayloadHeader({
          x402Version: 2, accepted: required.accepts[0], resource: required.resource, payload: proof,
        }),
      } });
      assert.equal(response.status, 200);
      assert.equal(await response.text(), resource.body);
      const settlement = decodeSettlementHeader(response.headers.get(PAYMENT_RESPONSE_HEADER));
      assert.equal(settlement.success, true);
      assert.equal(settlement.success, firstSettlement.success);
      assert.equal(settlement.transaction, firstSettlement.transaction);
      assert.equal(settlement.amount, "25");
      assert.equal(settlement.network, "eip155:125");
      assert.deepEqual(settlement.extensions.layerx, proof);
      assert.equal(releases.length, 1, "restart must recover the fulfilled receipt without releasing again");
      assert.deepEqual(settlement, firstSettlement);
      const recovered = await fulfillments.get(releases[0].idempotencyKey);
      assert.equal(recovered.receiptDigest, proof.receiptDigest);
      assert.deepEqual(recovered.resource, resource);
      assert.deepEqual(recovered, stored);
    }
  } finally {
    await closeServer();
    await rm(directory, { recursive: true, force: true });
  }
}

test(loadConfigReadsReceiptKeysJson);
test(integrationSettlesSignedSequencerReceipt);
