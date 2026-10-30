import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { AgentMiddleware } from "@sidiora/layerx-agent-middleware";
import { keccak256, ProductionClient, SecretBytes } from "@sidiora/layerx-sdk";
import { authenticatedAgentTransport, readDeclaredConfig } from "../dist/config.js";
import {
  LayerXAgentTransport,
  LayerXBudgetLedger,
  LayerXReceiptResolver,
  LayerXRemoteSigner,
  LayerXServiceEndpoint,
} from "../dist/services.js";
import { AgentToolExecutor, renderOutcome } from "../dist/tools.js";

const vectors = JSON.parse(readFileSync(
  new URL("../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8",
)).cases;
const vector = vectors.find((entry) => entry.id === "V1-pax-transfer");
assert.ok(vector);
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const hex = (value) => Buffer.from(value).toString("hex");

function portableReceipt(entry = vector) {
  return {
    format: "layerx-receipt-proof-v2",
    verificationLevel: "receipted",
    rawTransaction: entry.rawTransaction,
    signedReceipt: entry.signedReceipt,
    receiptDigest: `0x${hex(keccak256(bytes(entry.signedReceipt)))}`,
    receiptKey: {
      keyId: entry.pinnedKey.keyId,
      publicKey: entry.pinnedKey.publicKey,
      firstHeight: String(entry.pinnedKey.firstHeight),
      lastHeight: String(entry.pinnedKey.lastHeight),
    },
  };
}

function observation(entry = vector) {
  const proof = portableReceipt(entry);
  return {
    tx_hash: entry.receiptJson.txHash.slice(2),
    submission: {
      submission_ref: "signed-payment",
      state: "Executed",
      verification_level: "Receipted",
      receipt_ref: "payment-receipt",
      evidence: [],
      transitions: [],
    },
    receipt: {
      signed_receipt: proof.signedReceipt.slice(2),
      raw_transaction: proof.rawTransaction.slice(2),
      verification_level: "Receipted",
      receipt_key: {
        key_id: proof.receiptKey.keyId.slice(2),
        public_key: proof.receiptKey.publicKey.slice(2),
        first_height: proof.receiptKey.firstHeight,
        last_height: proof.receiptKey.lastHeight,
      },
    },
  };
}

async function receiptFixture(t) {
  const fixture = { proof: portableReceipt(), tracked: observation(), requests: [] };
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    fixture.requests.push({ path: request.url, method: request.method, body });
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify(request.url === "/v1/agent/rpc" ? {
      request_id: body.request_id,
      value: fixture.tracked,
      verification_status: { state: "achieved", level: "SequencerSigned" },
    } : fixture.proof));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(() => new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeAllConnections();
  }));
  const url = `http://127.0.0.1:${server.address().port}`;
  const token = new SecretBytes(new TextEncoder().encode("receipt-fixture-token"));
  t.after(() => token.destroy());
  fixture.endpoint = new LayerXServiceEndpoint({ url: `${url}/receipt`, token, timeoutMs: 1000 });
  fixture.receipts = new LayerXReceiptResolver(fixture.endpoint);
  fixture.environment = {
    LAYERX_AGENT_RPC_URL: url,
    LAYERX_AGENT_SURFACE: "gateway",
    LAYERX_SESSION_ID: "11".repeat(32),
    LAYERX_SESSION_TOKEN: "22".repeat(32),
    LAYERX_SESSION_GENERATION: "0",
    LAYERX_GATEWAY_KEY_ID: "receipt-test",
    LAYERX_GATEWAY_KEY: `lxp_live_${"33".repeat(32)}`,
    LAYERX_PROTOCOL_VERSION: "2",
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([vector.pinnedKey]),
    LAYERX_TENANT: "receipt-test",
    LAYERX_ACTOR: "receipt-test-actor",
    LAYERX_AUTHORITY: "receipt-test-authority",
    LAYERX_FEE_LIMIT: "0",
    LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ receipt: "44".repeat(32) }),
  };
  return fixture;
}

function executor(t, fixture, receiptKeys = [vector.pinnedKey]) {
  const environment = { ...fixture.environment, LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(receiptKeys) };
  const config = readDeclaredConfig(environment);
  const authenticated = authenticatedAgentTransport(environment, config);
  t.after(() => authenticated.destroy());
  const client = new ProductionClient(new LayerXAgentTransport(authenticated.transport));
  const middleware = new AgentMiddleware({
    client,
    receiptKeys: config.receiptKeys,
    budgets: new LayerXBudgetLedger(fixture.endpoint),
    signer: new LayerXRemoteSigner(fixture.endpoint),
    receipts: fixture.receipts,
  });
  return new AgentToolExecutor({ middleware, client, receipts: fixture.receipts, config });
}

function changedByte(value) {
  const changed = bytes(value);
  changed[changed.length - 1] ^= 1;
  return `0x${hex(changed)}`;
}

async function paymentToolVerifiesSignedSequencerReceipt(t) {
  const fixture = await receiptFixture(t);
  const tools = executor(t, fixture);
  const input = {
    receiptRef: "payment-receipt",
    asset: "0e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73",
    amount: "1000000000000000000",
    recipient: `000000000000000000000000${vector.receiptJson.to.slice(2)}`,
  };
  const expected = {
    ok: true,
    tool: "layerx_verify_receipt",
    result: {
      ...input,
      level: "sequencer-signed",
      receiptDigest: vector.receiptDigest.slice(2),
      resultCode: 0,
    },
  };
  assert.deepEqual(renderOutcome(await tools.execute("layerx_verify_receipt", input)), expected);
  assert.deepEqual(fixture.requests[0], {
    path: "/receipt", method: "POST", body: { receipt_ref: "payment-receipt" },
  });
  assert.deepEqual(await tools.execute("layerx_track", { submissionRef: "signed-payment" }), {
    ok: true,
    tool: "layerx_track",
    result: {
      submissionRef: "signed-payment", state: "Executed",
      receiptDigest: vector.receiptDigest.slice(2), verificationLevel: "sequencer-signed",
    },
  });
  assert.equal(fixture.requests.at(-1).body.operation, "track");

  const refused = (tool) => ({ ok: false, tool, code: "verification-failure" });
  for (const [field, value] of [
    ["signedReceipt", changedByte(vector.signedReceipt)],
    ["rawTransaction", changedByte(vector.rawTransaction)],
  ]) {
    fixture.proof = { ...portableReceipt(), [field]: value };
    assert.deepEqual(await tools.execute("layerx_verify_receipt", input), refused("layerx_verify_receipt"));
  }
  fixture.proof = portableReceipt();
  for (const changed of [
    { amount: "1000000000000000001" }, { asset: "55".repeat(32) }, { recipient: "66".repeat(32) },
  ]) {
    assert.deepEqual(await tools.execute("layerx_verify_receipt", { ...input, ...changed }), refused("layerx_verify_receipt"));
  }
  for (const pinnedKey of [
    { ...vector.pinnedKey, keyId: `0x${"77".repeat(32)}` },
    { ...vector.pinnedKey, publicKey: `0x${"88".repeat(32)}` },
    { ...vector.pinnedKey, firstHeight: 1001 },
    { ...vector.pinnedKey, firstHeight: 0, lastHeight: 999 },
  ]) {
    const untrusted = executor(t, fixture, [pinnedKey]);
    assert.deepEqual(await untrusted.execute("layerx_verify_receipt", input), refused("layerx_verify_receipt"));
    assert.deepEqual(await untrusted.execute("layerx_track", { submissionRef: "signed-payment" }), refused("layerx_track"));
  }
  fixture.tracked.receipt.signed_receipt = changedByte(vector.signedReceipt).slice(2);
  assert.deepEqual(await tools.execute("layerx_track", { submissionRef: "signed-payment" }), refused("layerx_track"));
  const reverted = vectors.find((entry) => entry.id === "V2-layerxledger-revert");
  assert.ok(reverted);
  fixture.proof = portableReceipt(reverted);
  assert.deepEqual(await tools.execute("layerx_verify_receipt", input), refused("layerx_verify_receipt"));
  fixture.tracked = observation(reverted);
  assert.deepEqual(await tools.execute("layerx_track", { submissionRef: "signed-payment" }), refused("layerx_track"));
}

async function serviceClientReadsSignedReceiptFields(t) {
  const fixture = await receiptFixture(t);
  for (const verificationLevel of ["receipted", "committed"]) {
    fixture.proof = { ...portableReceipt(), verificationLevel };
    const evidence = await fixture.receipts.resolve("payment-receipt");
    assert.deepEqual(evidence, fixture.proof);
    assert.equal(Object.hasOwn(evidence, "authorizedBatch"), false);
    assert.equal(Object.hasOwn(evidence, "canonicalReceipt"), false);
  }
  const proof = portableReceipt();
  const invalid = [
    { authorized_batch: {}, canonical_receipt: vector.signedReceipt },
    ...["authorized_batch", "authorizedBatch", "canonical_receipt", "canonicalReceipt"].map((field) => ({ ...proof, [field]: {} })),
    { ...proof, format: "layerx-receipt-proof-v1" },
    { ...proof, verificationLevel: "SettlementAnchored" },
    ...["rawTransaction", "signedReceipt", "receiptDigest"].flatMap((field) => [
      { ...proof, [field]: undefined }, { ...proof, [field]: "0x" }, { ...proof, [field]: "abcd" },
    ]),
    { ...proof, receiptDigest: "0x00" },
    { ...proof, receiptKey: undefined },
    ...["keyId", "publicKey"].map((field) => ({ ...proof, receiptKey: { ...proof.receiptKey, [field]: "0x00" } })),
    ...["-1", "01", "18446744073709551616", 1000].map((firstHeight) => ({
      ...proof, receiptKey: { ...proof.receiptKey, firstHeight },
    })),
    { ...proof, receiptKey: { ...proof.receiptKey, firstHeight: "2000" } },
  ];
  for (const response of invalid) {
    fixture.proof = response;
    await assert.rejects(fixture.receipts.resolve("payment-receipt"), { code: "service-refused" });
  }
}

test(paymentToolVerifiesSignedSequencerReceipt);
test(serviceClientReadsSignedReceiptFields);
