import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { AgentMiddleware, decodeDaemonSubmitResult, resolveDaemonReceipt } from "@sidiora/layerx-agent-middleware";
import { keccak256, ProductionClient, SecretBytes } from "@sidiora/layerx-sdk";
import { decodePaymentExactLog } from "@sidiora/layerx-seller-middleware";
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
const vector = vectors.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(vector);
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const hex = (value) => Buffer.from(value).toString("hex");
const daemonTranscript = JSON.parse(readFileSync(
  new URL("fixtures/daemon-submission-transcript.json", import.meta.url), "utf8",
));

async function portableReceipt(entry = vector) {
  const txHash = entry.receiptJson.txHash.slice(2);
  const submission = decodeDaemonSubmitResult({
    preparation_ref: "signed-payment", state: "Receipted",
    signed_receipt: entry.signedReceipt.slice(2), tx_hash: txHash,
  }, txHash, "signed-payment");
  const resolved = await resolveDaemonReceipt(submission, bytes(entry.rawTransaction), [{
    keyId: bytes(entry.pinnedKey.keyId), publicKey: bytes(entry.pinnedKey.publicKey),
    first_height: BigInt(entry.pinnedKey.firstHeight), last_height: BigInt(entry.pinnedKey.lastHeight),
  }]);
  assert.ok(resolved.portableReceipt);
  return resolved.portableReceipt;
}

function trackedSubmission(entry = vector, state = "Executed", rung = "receipted") {
  const txHash = entry.receiptJson.txHash.slice(2);
  return {
    tx_hash: txHash, submission_ref: txHash, state, rung,
    receipt_ref: createHash("sha256").update("LXP/v1/receipt\0").update(bytes(entry.signedReceipt)).digest("hex"),
    evidence: [], transitions: [],
  };
}

async function receiptFixture(t) {
  const fixture = {
    proof: await portableReceipt(), tracked: trackedSubmission(), requests: [],
    verification: { state: "achieved", rung: "receipted" },
  };
  const server = createServer(async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    fixture.requests.push({ path: request.url, method: request.method, body });
    assert.equal(request.method, "POST");
    assert.equal(request.headers["content-type"], "application/json");
    if (request.url === "/receipt") {
      assert.equal(request.headers.authorization, "Bearer receipt-fixture-token");
      assert.deepEqual(Object.keys(body), ["receipt_ref"]);
    } else {
      assert.equal(request.url, "/v1/agent/rpc");
      assert.equal(body.operation, "track");
      assert.deepEqual(Object.keys(body.request), ["submission_ref"]);
      assert.match(body.request.submission_ref, /^[0-9a-f]{64}$/u);
    }
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify(request.url === "/v1/agent/rpc" ? {
      request_id: body.request_id,
      value: fixture.tracked,
      verification_status: fixture.verification,
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
  const log = vector.receiptJson.logs[0];
  assert.ok(log);
  const payment = decodePaymentExactLog({ address: log.address, data: bytes(log.data), topics: log.topics.map(bytes) });
  const input = {
    receiptRef: "payment-receipt",
    asset: hex(payment.assetId),
    amount: payment.amount.toString(),
    recipient: hex(payment.to),
  };
  const expected = {
    ok: true,
    tool: "layerx_verify_receipt",
    result: {
      ...input,
      level: "sequencer-signed",
      receiptDigest: hex(keccak256(bytes(vector.signedReceipt))),
      resultCode: 0,
    },
  };
  assert.deepEqual(renderOutcome(await tools.execute("layerx_verify_receipt", input)), expected);
  assert.deepEqual(fixture.requests[0], {
    path: "/receipt", method: "POST", body: { receipt_ref: "payment-receipt" },
  });
  const trackInput = { submissionRef: vector.receiptJson.txHash.slice(2) };
  assert.deepEqual(await tools.execute("layerx_track", trackInput), {
    ok: true,
    tool: "layerx_track",
    result: {
      ...trackInput, state: "Executed",
      receiptDigest: hex(keccak256(bytes(vector.signedReceipt))), verificationLevel: "sequencer-signed",
    },
  });
  assert.equal(fixture.requests[1].body.operation, "track");
  assert.deepEqual(fixture.requests.at(-1), {
    path: "/receipt", method: "POST", body: { receipt_ref: fixture.tracked.receipt_ref },
  });
  fixture.tracked = trackedSubmission(vector, "Committed", "committed");
  fixture.verification = { state: "achieved", rung: "committed" };
  assert.deepEqual(await tools.execute("layerx_track", trackInput), {
    ok: true, tool: "layerx_track", result: {
      ...trackInput, state: "Committed",
      receiptDigest: hex(keccak256(bytes(vector.signedReceipt))), verificationLevel: "sequencer-signed",
    },
  });
  fixture.tracked = trackedSubmission();
  fixture.verification = { state: "achieved", rung: "receipted" };

  const refused = (tool) => ({ ok: false, tool, code: "verification-failure" });
  for (const [field, value] of [
    ["signedReceipt", changedByte(vector.signedReceipt)],
    ["rawTransaction", changedByte(vector.rawTransaction)],
  ]) {
    fixture.proof = { ...await portableReceipt(), [field]: value };
    assert.deepEqual(await tools.execute("layerx_verify_receipt", input), refused("layerx_verify_receipt"));
    assert.deepEqual(await tools.execute("layerx_track", trackInput), refused("layerx_track"));
  }
  fixture.proof = await portableReceipt();
  for (const changed of [
    { amount: "1000000000000000001" }, { asset: "55".repeat(32) }, { recipient: "66".repeat(32) },
  ]) {
    assert.deepEqual(await tools.execute("layerx_verify_receipt", { ...input, ...changed }), refused("layerx_verify_receipt"));
  }
  for (const pinnedKey of [
    { ...vector.pinnedKey, keyId: `0x${"77".repeat(32)}` },
    { ...vector.pinnedKey, publicKey: `0x${"88".repeat(32)}` },
    { ...vector.pinnedKey, firstHeight: Number(BigInt(vector.receiptJson.height)) + 1 },
    { ...vector.pinnedKey, firstHeight: 0, lastHeight: 999 },
  ]) {
    const untrusted = executor(t, fixture, [pinnedKey]);
    assert.deepEqual(await untrusted.execute("layerx_verify_receipt", input), refused("layerx_verify_receipt"));
    assert.deepEqual(await untrusted.execute("layerx_track", trackInput), refused("layerx_track"));
  }
  fixture.proof = { ...await portableReceipt(), signedReceipt: changedByte(vector.signedReceipt) };
  assert.deepEqual(await tools.execute("layerx_track", trackInput), refused("layerx_track"));
  const reverted = vectors.find((entry) => entry.id === "V2-layerxledger-revert");
  assert.ok(reverted);
  fixture.proof = await portableReceipt(reverted);
  assert.deepEqual(await tools.execute("layerx_verify_receipt", input), refused("layerx_verify_receipt"));
  fixture.tracked = trackedSubmission(reverted);
  assert.deepEqual(await tools.execute("layerx_track", { submissionRef: reverted.receiptJson.txHash.slice(2) }), refused("layerx_track"));
}

async function serviceClientReadsSignedReceiptFields(t) {
  const fixture = await receiptFixture(t);
  for (const verificationLevel of ["receipted", "committed"]) {
    fixture.proof = { ...await portableReceipt(), verificationLevel };
    const evidence = await fixture.receipts.resolve("payment-receipt");
    assert.deepEqual(evidence, fixture.proof);
    assert.equal(Object.hasOwn(evidence, "authorizedBatch"), false);
    assert.equal(Object.hasOwn(evidence, "canonicalReceipt"), false);
  }
  const proof = await portableReceipt();
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
    ...["-1", "01", "18446744073709551616", 1000, "0x00", "0x01", "0x10000000000000000", "0x", "0X1"].map((firstHeight) => ({
      ...proof, receiptKey: { ...proof.receiptKey, firstHeight },
    })),
    { ...proof, receiptKey: { ...proof.receiptKey, firstHeight: "0x7d0" } },
  ];
  for (const response of invalid) {
    fixture.proof = response;
    await assert.rejects(fixture.receipts.resolve("payment-receipt"), { code: "service-refused" });
  }
}

async function trackToolReadsRecordedDaemonSubmissions(t) {
  const fixture = await receiptFixture(t);
  const recordedKey = daemonTranscript.receipt_key;
  const entry = {
    rawTransaction: `0x${daemonTranscript.raw_transaction}`,
    signedReceipt: `0x${daemonTranscript.submit.signed_receipt}`,
    receiptJson: { txHash: `0x${daemonTranscript.submit.tx_hash}` },
    pinnedKey: {
      ...recordedKey,
      firstHeight: BigInt(recordedKey.firstHeight).toString(),
      lastHeight: BigInt(recordedKey.lastHeight).toString(),
    },
  };
  fixture.proof = await portableReceipt(entry);
  const tools = executor(t, fixture, [entry.pinnedKey]);
  const input = { submissionRef: daemonTranscript.submit.tx_hash };
  fixture.tracked = structuredClone(daemonTranscript.pending.submission);
  fixture.verification = { state: "achieved", rung: "submitted" };
  assert.deepEqual(await tools.execute("layerx_track", input), {
    ok: true, tool: "layerx_track", result: { ...input, state: "Unknown", verificationLevel: "Submitted" },
  });
  assert.equal(fixture.requests.length, 1);
  assert.deepEqual(fixture.requests[0].body.request, { submission_ref: input.submissionRef });
  fixture.tracked = structuredClone(daemonTranscript.committed.submission);
  fixture.verification = { state: "achieved", rung: "committed" };
  assert.deepEqual(await tools.execute("layerx_track", input), {
    ok: true, tool: "layerx_track", result: {
      ...input, state: "Committed", receiptDigest: hex(keccak256(bytes(entry.signedReceipt))),
      verificationLevel: "sequencer-signed",
    },
  });
  assert.equal(fixture.requests.length, 3);
  assert.deepEqual(fixture.requests.at(-1).body, { receipt_ref: fixture.tracked.receipt_ref });
}

async function trackToolRefusesMismatchedEvidenceAndMalformedSubmissions(t) {
  const fixture = await receiptFixture(t);
  const tools = executor(t, fixture);
  const input = { submissionRef: vector.receiptJson.txHash.slice(2) };
  const refused = { ok: false, tool: "layerx_track", code: "verification-failure" };
  const tracked = trackedSubmission();
  for (const changed of [
    { receiptDigest: `0x${"99".repeat(32)}` },
    await portableReceipt(vectors.find((entry) => entry.id === "V1-pax-transfer")),
  ]) {
    fixture.proof = { ...await portableReceipt(), ...changed };
    assert.deepEqual(await tools.execute("layerx_track", input), refused);
  }
  fixture.proof = await portableReceipt();
  for (const changed of [
    { ...tracked, tx_hash: "99".repeat(32) },
    { ...tracked, submission_ref: "99".repeat(32) },
    { ...tracked, receipt_ref: "99".repeat(32) },
    { ...tracked, receipt_ref: undefined },
    { ...tracked, state: "UnknownState" },
    { ...tracked, state: "Committed", rung: "receipted" },
    { ...tracked, rung: "submitted" },
    { ...tracked, rung: "sequencer-signed" },
    { ...tracked, verification_level: "Receipted" },
    { ...tracked, evidence: new Array(65).fill({ kind: "receipt", digest: "00".repeat(32) }) },
    { ...tracked, evidence: [{ kind: "receipt", digest: "00" }] },
    { ...tracked, transitions: new Array(65).fill({ from: "Submitted", to: "Unknown", cause: "retry", at: "1" }) },
    { ...tracked, transitions: [{ from: "Submitted", to: "Executed", cause: "retry", at: "1" }] },
    { ...tracked, transitions: [{ from: "Submitted", to: "Unknown", cause: "retry", at: "18446744073709551616" }] },
    { activity_id: tracked.tx_hash, submission: tracked, receipt: null },
  ]) {
    fixture.tracked = changed;
    assert.deepEqual(await tools.execute("layerx_track", input), refused);
  }
}

test(paymentToolVerifiesSignedSequencerReceipt);
test(serviceClientReadsSignedReceiptFields);
test(trackToolReadsRecordedDaemonSubmissions);
test(trackToolRefusesMismatchedEvidenceAndMalformedSubmissions);
