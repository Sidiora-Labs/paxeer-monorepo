import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { mkdtemp, readdir, readFile, rm } from "node:fs/promises";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";
import {
  DurableFulfillmentRepository,
  MiddlewareError,
  PAYMENT_RESPONSE_HEADER,
  PAYMENT_SIGNATURE_HEADER,
  ReceiptPayloadAuthority,
  SellerMiddleware,
  decodePaymentExactLog,
  decodeSettlementHeader,
  encodePaymentPayloadHeader,
} from "../dist/index.js";

// Exercise the real route before the declared gate builds the Next package.
const nextSource = new URL("../../../../../sdk/integrations/next/src/index.ts", import.meta.url).href;
const buyerSource = new URL("../../buyer/src/index.ts", import.meta.url).href;
const scanSource = new URL("../../../../../sdk/integrations/next/src/scan.ts", import.meta.url).href;
const sources = new Set([nextSource, buyerSource, scanSource]);
const hooks = registerHooks({
  resolve(specifier, context, next) {
    if (context.parentURL === nextSource) {
      if (specifier === "@sidiora/layerx-buyer-middleware") {
        return { url: buyerSource, shortCircuit: true };
      }
      if (specifier === "./scan.js") return { url: scanSource, shortCircuit: true };
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (!sources.has(url)) return next(url, context);
    return {
      format: "module", shortCircuit: true,
      source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }),
    };
  },
});
let mountLayerX;
let SingleProcessWebhookDeliveryStore;
try {
  ({ mountLayerX, SingleProcessWebhookDeliveryStore } = await import(nextSource));
} finally {
  hooks.deregister();
}

const vectors = JSON.parse(readFileSync(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const vector = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(vector);
const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;
const log = vector.receiptJson.logs[0];
const payment = decodePaymentExactLog({
  address: log.address, data: bytes(log.data), topics: log.topics.map(bytes),
});
const pin = {
  keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
};
const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [pin]);
const proof = {
  format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
  rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
  receiptDigest: hex(verified.receiptDigest),
  receiptKey: {
    keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
    firstHeight: `0x${pin.first_height.toString(16)}`, lastHeight: `0x${pin.last_height.toString(16)}`,
  },
};
const environment = {
  LAYERX_PRINCIPAL: "merchant", LAYERX_PROTECTED_PATH: "/paid",
  LAYERX_RESOURCE_URL: "https://merchant.example/paid", LAYERX_PROTOCOL_VERSION: "3",
  LAYERX_X402_SCHEME: "exact", LAYERX_X402_NETWORK: "eip155:125",
  LAYERX_PRICE: payment.amount.toString(), LAYERX_ASSET: hex(payment.assetId),
  LAYERX_PAY_TO: hex(payment.to), LAYERX_PAYMENT_TIMEOUT_SECONDS: "120",
  LAYERX_PAYMENT_PURPOSE_HASH: hex(payment.paymentReference).slice(2),
  LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([{
    keyId: hex(pin.keyId), publicKey: hex(pin.publicKey),
    firstHeight: pin.first_height.toString(), lastHeight: pin.last_height.toString(),
  }]),
  LAYERX_WEBHOOK_PATH: "/webhook",
  LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ sequencer: Buffer.from(pin.publicKey).toString("base64") }),
};

class CountingReceiptPayloadAuthority extends ReceiptPayloadAuthority {
  settlements = 0;

  async settle(request) {
    this.settlements += 1;
    return super.settle(request);
  }
}

async function repositorySnapshot(directory) {
  const files = {};
  for (const entry of (await readdir(directory, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name))) {
    files[entry.name] = entry.isDirectory()
      ? await repositorySnapshot(join(directory, entry.name))
      : await readFile(join(directory, entry.name), "utf8");
  }
  return files;
}

async function differingRequestIsVerifiedBeforeReceiptReuse(t) {
  const directory = await mkdtemp(join(tmpdir(), "seller-22-31-530-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const releases = [];
  const events = [];
  const resources = {
    async release(request, idempotencyKey) {
      releases.push({ url: request.url, idempotencyKey });
      return { contentType: "text/plain", body: "verified paid resource" };
    },
  };
  let authority;
  let repository;
  function mount() {
    authority = new CountingReceiptPayloadAuthority([pin]);
    repository = new DurableFulfillmentRepository(directory);
    return mountLayerX({
      environment, authority, fulfillments: repository, resources,
      deliveries: new SingleProcessWebhookDeliveryStore(),
      events: { async handle(event, deliveryId) { events.push({ event, deliveryId }); } },
    });
  }
  let mounted = mount();
  t.after(() => mounted.destroy());
  assert.ok(mounted.seller instanceof SellerMiddleware);
  assert.ok(authority instanceof ReceiptPayloadAuthority);
  function header(description, evidence = proof) {
    return encodePaymentPayloadHeader({
      x402Version: 2, accepted: mounted.config.requirements, payload: evidence,
      resource: { ...mounted.config.paymentRequired.resource, description },
    });
  }
  const original = header("first request");
  const request = (paymentHeader) => new Request(environment.LAYERX_RESOURCE_URL, {
    headers: { [PAYMENT_SIGNATURE_HEADER]: paymentHeader },
  });
  const handle = (paymentHeader) => mounted.seller.handle("merchant", paymentHeader,
    (key) => resources.release(request(paymentHeader), key));
  let first;
  let snapshot;
  let record;

  await t.test(async function settlesOnceAndReleases() {
    first = await handle(original);
    assert.equal(first.kind, "released");
    assert.equal(first.status, 200);
    assert.equal(first.resource.body, "verified paid resource");
    assert.equal(first.verification.txHash, vector.receiptJson.txHash);
    assert.equal(releases.length, 1);
    assert.equal(authority.settlements, 1);
    record = await repository.get(releases[0].idempotencyKey);
    assert.equal(record.receiptDigest, proof.receiptDigest);
    assert.deepEqual(record.resource, first.resource);
    snapshot = await repositorySnapshot(directory);
    assert.equal(Object.keys(snapshot).length, 1);
    assert.deepEqual(Object.keys(snapshot[record.idempotencyKey]).sort(), ["claim.json", "fulfilled.json"]);
  });

  await t.test(async function identicalRequestReplaysStoredRecordAfterRestart() {
    mounted.destroy();
    mounted = mount();
    const replayed = await handle(original);
    assert.equal(replayed.kind, "released");
    assert.equal(replayed.status, 200);
    assert.deepEqual(replayed.resource, first.resource);
    assert.deepEqual(replayed.settlement, first.settlement);
    assert.deepEqual(replayed.headers, first.headers);
    assert.equal(authority.settlements, 0);
    assert.equal(releases.length, 1);
    assert.deepEqual(await repository.get(record.idempotencyKey), record);
    assert.deepEqual(await repositorySnapshot(directory), snapshot);
  });

  await t.test(async function changedReceiptKeyFailsVerificationBeforeReuse() {
    const changedKey = bytes(proof.receiptKey.publicKey);
    changedKey[0] ^= 1;
    const tampered = { ...proof, receiptKey: { ...proof.receiptKey, publicKey: hex(changedKey) } };
    assert.equal(tampered.rawTransaction, proof.rawTransaction);
    const differing = header("unverified differing request", tampered);
    assert.notEqual(differing, original);
    await assert.rejects(handle(differing), (error) => {
      assert.ok(error instanceof MiddlewareError);
      assert.equal(error.code, "verification-failure");
      return true;
    });
    const refused = await mounted.resource.GET(request(differing));
    assert.equal(refused.status, 402);
    const body = await refused.json();
    assert.equal(body.error, "verification-failure");
    assert.notEqual(body.error, "receipt_reuse");
    assert.equal(authority.settlements, 2);
    assert.equal(releases.length, 1);
    assert.deepEqual(await repository.get(record.idempotencyKey), record);
    assert.deepEqual(await repositorySnapshot(directory), snapshot);
  });

  await t.test(async function validDifferingRequestIsReceiptReuse() {
    const differing = header("verified differing request");
    assert.notEqual(differing, original);
    const refused = await handle(differing);
    assert.equal(refused.kind, "refused");
    assert.equal(refused.status, 409);
    assert.equal(refused.settlement.success, false);
    assert.equal(refused.settlement.errorReason, "receipt_reuse");
    assert.equal(refused.settlement.transaction, "");
    assert.deepEqual(decodeSettlementHeader(refused.headers[PAYMENT_RESPONSE_HEADER]), refused.settlement);
    assert.equal(authority.settlements, 3);
    assert.equal(releases.length, 1);
    assert.deepEqual(await repository.get(record.idempotencyKey), record);
    assert.deepEqual(await repositorySnapshot(directory), snapshot);
    assert.deepEqual(events, []);
  });
}

test(differingRequestIsVerifiedBeforeReceiptReuse);
