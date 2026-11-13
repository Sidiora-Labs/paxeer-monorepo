import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { sdk, middleware, services, config, currentPaymentReceipt, currentPortableReceipt,
  currentStatus, transcript, paymentFacts } from "./receipt-producer-fixture.mjs";
const { AgentMiddleware, decodeDaemonSubmitResult, resolveDaemonReceipt } = middleware;
const { ProductionClient, SecretBytes } = sdk;
const { authenticatedAgentTransport, readDeclaredConfig } = config;
const { LayerXAgentTransport, LayerXBudgetLedger, LayerXReceiptResolver,
  LayerXRemoteSigner, LayerXServiceEndpoint } = services;
const { AgentToolExecutor, renderOutcome } = await import("../src/tools.ts");
const producedEvidence = await currentPaymentReceipt();
const trackedEvidence = await currentPortableReceipt();
const paymentPin = {
  ...producedEvidence.receiptKey,
  firstHeight: BigInt(producedEvidence.receiptKey.firstHeight).toString(),
  lastHeight: BigInt(producedEvidence.receiptKey.lastHeight).toString(),
};
const trackingPin = {
  ...transcript.receipt_key,
  firstHeight: BigInt(transcript.receipt_key.firstHeight).toString(),
  lastHeight: BigInt(transcript.receipt_key.lastHeight).toString(),
};

const vectors = JSON.parse(readFileSync(
  new URL("../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8",
)).cases;
const vector = vectors.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(vector);
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const hex = (value) => Buffer.from(value).toString("hex");
const daemonTranscript = transcript;

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
    receipt_ref: entry.receiptDigest.slice(2),
    evidence: [], transitions: [],
  };
}

async function receiptMetadata(entry) {
  const signedReceipt = bytes(entry.signedReceipt);
  const key = entry.pinnedKey;
  const verified = await sdk.verifySequencerReceipt(signedReceipt, bytes(entry.rawTransaction), [{
    keyId: bytes(key.keyId), publicKey: bytes(key.publicKey),
    first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight),
  }]);
  return {
    tx_hash: entry.receiptJson.txHash.slice(2),
    canonical_bytes: entry.signedReceipt.slice(2),
    authorised_batch: {
      batch_id: hex(verified.receipt.batchId), asset: hex(verified.receipt.asset),
      previous_state_root: hex(verified.receipt.previousStateRoot),
      resulting_state_root: hex(verified.receipt.resultingStateRoot),
      sequencer_public_key: key.publicKey.slice(2),
    },
    rung: "receipted", verification_level: "1",
    receipt_hash: createHash("sha256").update(signedReceipt).digest("hex"),
    block_height: verified.height.toString(), signed_receipt: entry.signedReceipt.slice(2),
    receipt_key_id: key.keyId.slice(2), public_key: key.publicKey.slice(2),
    first_height: BigInt(key.firstHeight).toString(), last_height: BigInt(key.lastHeight).toString(),
    inclusion_height: verified.height.toString(),
  };
}

async function receiptFixture(t) {
  const fixture = {
    proof: await portableReceipt(), tracked: trackedSubmission(), requests: [], replies: [],
    activityId: vector.receiptJson.txHash.slice(2), receipt: await receiptMetadata(vector),
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
    const reply = request.url === "/v1/agent/rpc" ? {
      request_id: body.request_id,
      value: {
        activity_id: fixture.activityId, submission: fixture.tracked,
        receipt: fixture.tracked.state === "Executed" || fixture.tracked.state === "Committed"
          ? { ...fixture.receipt, rung: fixture.tracked.rung } : null,
      },
      verification_status: fixture.verification,
    } : fixture.proof;
    fixture.replies.push(reply);
    response.end(JSON.stringify(reply));
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
  const input = { receiptRef: "payment-receipt", ...paymentFacts(fixture.proof) };
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
  const trackInput = { submissionRef: vector.receiptJson.txHash.slice(2) };
  assert.deepEqual(await tools.execute("layerx_track", trackInput), {
    ok: true,
    tool: "layerx_track",
    result: {
      ...trackInput, state: "Executed",
      receiptDigest: vector.receiptDigest.slice(2), verificationLevel: "sequencer-signed",
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
      receiptDigest: vector.receiptDigest.slice(2), verificationLevel: "sequencer-signed",
    },
  });
  fixture.tracked = trackedSubmission();
  fixture.verification = { state: "achieved", rung: "receipted" };

  const refused = (tool) => ({ ok: false, tool, code: "verification-failure" });
  for (const [field, value] of [
    ["signedReceipt", changedByte(vector.signedReceipt)],
    ["rawTransaction", changedByte(vector.rawTransaction)],
    ["receiptDigest", `0x${"99".repeat(32)}`],
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
  fixture.activityId = reverted.receiptJson.txHash.slice(2);
  fixture.receipt = await receiptMetadata(reverted);
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
    ...["1", "1000", "-1", "01", "18446744073709551616", 1000, "0x00", "0x01", "0x10000000000000000", "0x", "0X1"].map((firstHeight) => ({
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
    receiptDigest: `0x${daemonTranscript.committed.submission.receipt_ref}`,
    pinnedKey: {
      ...recordedKey,
      firstHeight: BigInt(recordedKey.firstHeight).toString(),
      lastHeight: BigInt(recordedKey.lastHeight).toString(),
    },
  };
  fixture.proof = await portableReceipt(entry);
  fixture.activityId = daemonTranscript.committed.activity_id;
  fixture.receipt = structuredClone(daemonTranscript.committed.receipt);
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
      ...input, state: "Committed", receiptDigest: entry.receiptDigest.slice(2),
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
  const malformed = { ok: false, tool: "layerx_track", code: "decode-failure" };
  const tracked = trackedSubmission();
  for (const changed of [
    { receiptDigest: `0x${"99".repeat(32)}` },
    await portableReceipt(vectors.find((entry) => entry.id === "V1-pax-transfer")),
  ]) {
    fixture.proof = { ...await portableReceipt(), ...changed };
    assert.deepEqual(await tools.execute("layerx_track", input), refused);
  }
  fixture.proof = await portableReceipt();
  for (const [index, changed] of [
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
  ].entries()) {
    fixture.tracked = changed;
    assert.deepEqual(await tools.execute("layerx_track", input), index === 2 ? refused : malformed);
  }
}

async function currentTrackingFixture(t) {
  const fixture = await receiptFixture(t);
  fixture.tracked = structuredClone(transcript.committed.submission);
  fixture.activityId = transcript.committed.activity_id;
  fixture.receipt = structuredClone(transcript.committed.receipt);
  fixture.proof = structuredClone(trackedEvidence);
  fixture.verification = currentStatus;
  return fixture;
}

async function paymentToolVerifiesCurrentProducerReceipt(t) {
  const fixture = await receiptFixture(t);
  fixture.proof = structuredClone(producedEvidence);
  const input = { receiptRef: "payment-receipt", ...paymentFacts(producedEvidence) };
  assert.deepEqual(await executor(t, fixture, [paymentPin]).execute("layerx_verify_receipt", input), {
    ok: true, tool: "layerx_verify_receipt", result: {
      ...input, level: "sequencer-signed", receiptDigest: producedEvidence.receiptDigest.slice(2), resultCode: 0,
    },
  });
  assert.deepEqual(fixture.requests, [{
    path: "/receipt", method: "POST", body: { receipt_ref: "payment-receipt" },
  }]);
  assert.deepEqual(fixture.replies, [producedEvidence]);
}

async function trackingToolVerifiesCurrentProducerReceipt(t) {
  const fixture = await currentTrackingFixture(t);
  const tools = executor(t, fixture, [trackingPin]);
  assert.deepEqual(await tools.execute("layerx_track", { submissionRef: transcript.committed.submission.submission_ref }), {
    ok: true,
    tool: "layerx_track",
    result: {
      submissionRef: transcript.committed.submission.submission_ref, state: "Committed",
      receiptDigest: trackedEvidence.receiptDigest.slice(2), verificationLevel: "sequencer-signed",
    },
  });
  assert.equal(fixture.requests[0].body.operation, "track");
  assert.deepEqual(fixture.requests[0].body.request, { submission_ref: transcript.committed.submission.submission_ref });
  assert.deepEqual(fixture.requests[1], {
    path: "/receipt", method: "POST", body: { receipt_ref: transcript.committed.submission.receipt_ref },
  });
}

async function trackingToolRefusesUntrustedCurrentProducerReceipt(t) {
  const fixture = await currentTrackingFixture(t);
  const refused = { ok: false, tool: "layerx_track", code: "verification-failure" };
  for (const pinnedKey of [
    { ...trackingPin, keyId: `0x${"77".repeat(32)}` },
    { ...trackingPin, publicKey: `0x${"88".repeat(32)}` },
    { ...trackingPin, firstHeight: 8, lastHeight: "1000" },
    { ...trackingPin, firstHeight: 0, lastHeight: 6 },
  ]) {
    await t.test(`refuses untrusted pin ${JSON.stringify(pinnedKey)}`, async () => {
      const untrusted = executor(t, fixture, [pinnedKey]);
      assert.deepEqual(await untrusted.execute("layerx_track", {
        submissionRef: transcript.committed.submission.submission_ref,
      }), refused);
    });
  }
  const tools = executor(t, fixture, [trackingPin]);
  fixture.proof.signedReceipt = changedByte(trackedEvidence.signedReceipt);
  await t.test("refuses a changed sequencer signature", async () => {
    assert.deepEqual(await tools.execute("layerx_track", {
      submissionRef: transcript.committed.submission.submission_ref,
    }), refused);
  });
  const reverted = vectors.find((entry) => entry.id === "V2-layerxledger-revert");
  assert.ok(reverted);
  fixture.proof = await portableReceipt(reverted);
  fixture.tracked = trackedSubmission(reverted);
  fixture.activityId = reverted.receiptJson.txHash.slice(2);
  fixture.receipt = await receiptMetadata(reverted);
  await t.test("refuses a reverted receipt", async () => {
    assert.deepEqual(await tools.execute("layerx_track", {
      submissionRef: reverted.receiptJson.txHash.slice(2),
    }), refused);
  });
}

async function trackingToolVerifiesExecutedReceiptAndRefusesSignedRevert(t) {
  const fixture = await currentTrackingFixture(t);
  fixture.tracked.state = "Executed";
  fixture.tracked.rung = "receipted";
  fixture.proof.verificationLevel = "receipted";
  fixture.verification = { state: "achieved", rung: "receipted" };
  assert.deepEqual(await executor(t, fixture, [trackingPin]).execute("layerx_track", {
    submissionRef: fixture.tracked.submission_ref,
  }), {
    ok: true, tool: "layerx_track", result: {
      submissionRef: fixture.tracked.submission_ref, state: "Executed",
      receiptDigest: trackedEvidence.receiptDigest.slice(2), verificationLevel: "sequencer-signed",
    },
  });
  const reverted = vectors.find((entry) => entry.id === "V2-layerxledger-revert");
  assert.ok(reverted);
  fixture.proof = await portableReceipt(reverted);
  const proof = fixture.proof;
  const pin = { ...reverted.pinnedKey,
    firstHeight: String(reverted.pinnedKey.firstHeight), lastHeight: String(reverted.pinnedKey.lastHeight) };
  const verified = await sdk.verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), [{
    keyId: bytes(pin.keyId), publicKey: bytes(pin.publicKey),
    first_height: BigInt(pin.firstHeight), last_height: BigInt(pin.lastHeight),
  }]);
  assert.notEqual(verified.receipt.resultCode, 0);
  const activityId = reverted.receiptJson.txHash.slice(2);
  fixture.tracked.tx_hash = activityId;
  fixture.tracked.submission_ref = activityId;
  fixture.tracked.receipt_ref = trackedSubmission(reverted).receipt_ref;
  fixture.activityId = activityId;
  fixture.receipt = await receiptMetadata(reverted);
  assert.deepEqual(await executor(t, fixture, [pin]).execute("layerx_track", { submissionRef: activityId }), {
    ok: false, tool: "layerx_track", code: "verification-failure",
  });
}

async function serviceTransportRefusesObsoleteStatusFields(t) {
  const fixture = await receiptFixture(t);
  const configured = readDeclaredConfig(fixture.environment);
  const authenticated = authenticatedAgentTransport(fixture.environment, configured);
  t.after(() => authenticated.destroy());
  for (const status of [
    { state: "achieved", level: "SequencerSigned" },
    { state: "achieved", rung: "committed", level: "SequencerSigned" },
  ]) {
    fixture.verification = status;
    await assert.rejects(authenticated.transport.call({ plane: "agent", operation: "track",
      request: { submission_ref: transcript.committed.submission.submission_ref } }),
    { code: "verification-failure", retry: "never" });
  }
  assert.equal(fixture.requests.length, 2);
  for (const request of fixture.requests) {
    assert.equal(request.path, "/v1/agent/rpc");
    assert.equal(request.method, "POST");
    assert.equal(request.body.operation, "track");
    assert.deepEqual(request.body.request, { submission_ref: transcript.committed.submission.submission_ref });
  }
}

test(serviceTransportRefusesObsoleteStatusFields);
test(paymentToolVerifiesCurrentProducerReceipt);
test(trackingToolVerifiesCurrentProducerReceipt);
test(trackingToolRefusesUntrustedCurrentProducerReceipt);
test(trackingToolVerifiesExecutedReceiptAndRefusesSignedRevert);
test(paymentToolVerifiesSignedSequencerReceipt);
test(serviceClientReadsSignedReceiptFields);
test(trackToolReadsRecordedDaemonSubmissions);
test(trackToolRefusesMismatchedEvidenceAndMalformedSubmissions);
