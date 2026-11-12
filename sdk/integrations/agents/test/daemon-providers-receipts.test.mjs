import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { once } from "node:events";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

export { sdk, middleware, services, config, currentPaymentReceipt, currentPortableReceipt,
  currentStatus, transcript } from "./receipt-producer-fixture.mjs";
import { sdk, daemon, services, currentPortableReceipt, currentStatus, receiptedStatus,
  receiptContext, transcript } from "./receipt-producer-fixture.mjs";
import { fileURLToPath } from "node:url";
const { loadAgentServiceProviders } = await import("../src/service-providers.ts");

// Replay the recorded signed bytes with receipt_value metadata and success_body
// envelope fields; only the request correlation ID varies during transport.
export async function currentDaemonFixture(t) {
  const directory = mkdtempSync(join(tmpdir(), "receipt-audit-"));
  const token = new sdk.SecretBytes(new Uint8Array(32).fill(0x22));
  const key = new sdk.SecretBytes(new TextEncoder().encode(`lxp_live_${"33".repeat(32)}`));
  const credential = new sdk.LayerXKeyCredential("receipt-test", key);
  const session = new sdk.AgentSessionCredential("receipt-test", "11".repeat(32), token, 0n);
  const fixture = { requests: [], replies: [],
    evidenceRequests: [], proof: await currentPortableReceipt(),
    receipt: structuredClone(transcript.receipt_envelope.value.receipt), status: currentStatus };
  const resolvers = [];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      if (request.url === "/receipt") {
        assert.equal(request.method, "POST");
        assert.equal(request.headers.authorization, `Bearer lxp_live_${"33".repeat(32)}`);
        assert.deepEqual(body, { receipt_ref: receiptContext.activityId });
        fixture.evidenceRequests.push(body);
        response.writeHead(200, { "content-type": "application/json" });
        response.end(JSON.stringify(fixture.proof));
        return;
      }
      fixture.requests.push(body);
      assert.equal(request.method, "POST");
      assert.equal(request.url, "/v1/agent/rpc");
      assert.equal(request.headers.authorization, `LayerX-Key receipt-test:lxp_live_${"33".repeat(32)}`);
      assert.deepEqual(body.credential, session.encode());
      assert.equal(body.version, 1);
      assert.equal(body.operation, "program.receipt");
      assert.equal(body.idempotency_key, null);
      assert.deepEqual(body.request, { idempotency_key: receiptContext.idempotencyKey,
        expected_activity_id: receiptContext.activityId, requested_rung: "receipted" });
      const reply = { ...structuredClone(transcript.receipt_envelope), request_id: body.request_id,
        value: { found: true, receipt: fixture.receipt }, verification_status: fixture.status };
      fixture.replies.push(reply);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify(reply));
    } catch (error) {
      response.writeHead(500, { "content-type": "application/json" });
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  t.after(async () => {
    for (const resolver of resolvers) resolver.close();
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
    token.destroy(); key.destroy();
    rmSync(directory, { recursive: true, force: true });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  fixture.transport = new sdk.AgentEnvelopeTransport({
    endpoint: `http://127.0.0.1:${server.address().port}`, surface: "gateway", session,
    gatewayCredential: credential, timeoutMs: 2000,
  });
  fixture.directory = directory;
  fixture.url = `http://127.0.0.1:${server.address().port}`;
  fixture.evidence = new services.LayerXReceiptResolver(new services.LayerXServiceEndpoint({
    url: `http://127.0.0.1:${server.address().port}/receipt`, token: key, timeoutMs: 2000,
  }));
  fixture.open = () => {
    const resolver = new daemon.DaemonReceiptResolver({ transport: fixture.transport, protocolVersion: 2,
      principal: { tenant: "receipt-test", actor: "receipt-agent", sessionId: "11".repeat(32) },
      receiptEvidence: fixture.evidence,
      storePath: join(directory, "receipts.sqlite") });
    resolvers.push(resolver);
    return resolver;
  };
  fixture.read = () => fixture.transport.call({ plane: "agent", operation: "program.receipt", request: {
    idempotency_key: receiptContext.idempotencyKey, expected_activity_id: receiptContext.activityId,
    requested_rung: "receipted",
  } });
  return fixture;
}

const { PlatformSdkError, Rung } = sdk;
const { verifySequencerReceipt } = daemon;

const vectors = JSON.parse(readFileSync(
  new URL("../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8",
)).cases;
const vector = vectors[0];
const bytes = (value) => Buffer.from(value.slice(2), "hex");
const context = (entry) => ({ idempotencyKey: "11".repeat(32), activityId: entry.receiptJson.txHash.slice(2) });
const status = (rung = Rung.Receipted) => ({ state: receiptedStatus.state, rung });
const height = (value) => `0x${BigInt(value).toString(16)}`;

function daemonReceipt(entry = vector, level = "SequencerSigned") {
  return {
    raw_transaction: entry.rawTransaction.slice(2),
    signed_receipt: entry.signedReceipt.slice(2),
    receipt_key: {
      key_id: entry.pinnedKey.keyId.slice(2), public_key: entry.pinnedKey.publicKey.slice(2),
      first_height: String(entry.pinnedKey.firstHeight), last_height: String(entry.pinnedKey.lastHeight),
    },
    verification_level: level,
  };
}

function refuses(code) {
  return (error) => {
    assert.ok(error instanceof PlatformSdkError);
    assert.equal(error.code, code);
    assert.equal(error.retry, "never");
    return true;
  };
}

async function daemonProviderConsumesCurrentProducerResponse(t) {
  const fixture = await currentDaemonFixture(t);
  const expected = await currentPortableReceipt();
  const envelope = await fixture.read();
  assert.equal(envelope.verification_status.rung, Rung.Committed);
  assert.deepEqual(envelope.value.receipt, fixture.receipt);
  const resolver = fixture.open();
  const evidence = await resolver.resolveFor("payment-receipt", receiptContext);
  assert.deepEqual(evidence, expected);
  assert.equal(evidence.receiptKey.firstHeight, "0x1");
  assert.equal(evidence.receiptKey.lastHeight, "0x3e8");
  resolver.close();
  assert.deepEqual(await fixture.open().resolve("payment-receipt"), evidence);
  assert.equal(fixture.requests.length, 3);
  assert.deepEqual(fixture.evidenceRequests, Array(2).fill({ receipt_ref: receiptContext.activityId }));
}

async function daemonServiceProviderWiresCurrentReceiptEvidence(t) {
  const fixture = await currentDaemonFixture(t);
  const providers = await loadAgentServiceProviders({
    LAYERX_AGENT_SERVICES_MODULE: fileURLToPath(new URL("./receipt-producer-fixture.mjs", import.meta.url)),
    LAYERX_AGENT_RPC_URL: fixture.url, LAYERX_AGENT_SURFACE: "gateway",
    LAYERX_TENANT: "receipt-test", LAYERX_ACTOR: "receipt-agent",
    LAYERX_SESSION_ID: "11".repeat(32), LAYERX_SESSION_TOKEN: "22".repeat(32),
    LAYERX_SESSION_GENERATION: "0", LAYERX_GATEWAY_KEY_ID: "receipt-test",
    LAYERX_GATEWAY_KEY: `lxp_live_${"33".repeat(32)}`, LAYERX_PROTOCOL_VERSION: "2",
    LAYERX_RECEIPT_FIXTURE_URL: `${fixture.url}/receipt`,
    LAYERX_RECEIPT_STORE_PATH: join(fixture.directory, "services.sqlite"),
  });
  try {
    assert.ok(providers.receipts instanceof daemon.DaemonReceiptResolver);
    assert.deepEqual(await providers.receipts.resolveFor("wired-receipt", receiptContext), await currentPortableReceipt());
    assert.deepEqual(fixture.evidenceRequests, [{ receipt_ref: receiptContext.activityId }]);
  } finally { await providers.destroy(); }
}

async function daemonProviderBindsCurrentReceiptEvidence(t) {
  const fixture = await currentDaemonFixture(t);
  const resolver = fixture.open();
  const proof = structuredClone(fixture.proof);
  for (const changed of [
    { ...proof, rawTransaction: `0x${"99".repeat(32)}` },
    { ...proof, signedReceipt: `0x${"99".repeat(32)}` },
    { ...proof, receiptDigest: `0x${"99".repeat(32)}` },
    { ...proof, receiptKey: { ...proof.receiptKey, keyId: `0x${"99".repeat(32)}` } },
  ]) {
    fixture.proof = changed;
    await assert.rejects(resolver.resolveFor("invalid-proof", receiptContext), refuses("verification-failure"));
    await assert.rejects(resolver.resolve("invalid-proof"), { code: "unavailable-capability" });
  }
  fixture.proof = proof;
  const batch = fixture.receipt.authorised_batch;
  fixture.receipt.authorised_batch = { ...batch, asset: "99".repeat(32) };
  await assert.rejects(resolver.resolveFor("invalid-batch", receiptContext), refuses("verification-failure"));
  fixture.receipt.authorised_batch = batch;
  const receipt = structuredClone(fixture.receipt);
  for (const changed of [
    { receipt_hash: "99".repeat(32) }, { block_height: "8" },
    { inclusion_height: "8" }, { receipt_key_id: "99".repeat(32) },
    { public_key: "99".repeat(32) }, { first_height: "2" },
    { last_height: "999" }, { signed_receipt: "99".repeat(32) },
    { verification_level: "5" },
  ]) {
    fixture.receipt = { ...receipt, ...changed };
    await assert.rejects(resolver.resolveFor("invalid-metadata", receiptContext), refuses("verification-failure"));
  }
  fixture.receipt = receipt;
  fixture.status = { state: "achieved", rung: Rung.Receipted };
  await assert.rejects(resolver.resolveFor("invalid-rung", receiptContext), refuses("verification-failure"));
}

async function daemonProviderVerifiesSignedSequencerReceipt() {
  const successful = vectors.filter((entry) => entry.id.startsWith("V1-"));
  assert.ok(successful.length > 0);
  for (const entry of successful) {
    const receiptDigest = entry.receiptDigest.slice(2);
    const verified = await verifySequencerReceipt(daemonReceipt(entry), status(), context(entry), receiptDigest);
    assert.equal(verified.receiptDigest, receiptDigest);
    assert.deepEqual(verified.evidence, {
      format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
      rawTransaction: entry.rawTransaction, signedReceipt: entry.signedReceipt,
      receiptDigest: `0x${receiptDigest}`,
      receiptKey: { keyId: entry.pinnedKey.keyId, publicKey: entry.pinnedKey.publicKey,
        firstHeight: height(entry.pinnedKey.firstHeight), lastHeight: height(entry.pinnedKey.lastHeight) },
    });
    assert.equal(Object.hasOwn(verified.evidence, "authorizedBatch"), false);
    assert.equal(Object.hasOwn(verified.evidence, "canonicalReceipt"), false);
  }

  const receipt = daemonReceipt();
  const changedSignature = Buffer.from(receipt.signed_receipt, "hex");
  changedSignature[changedSignature.length - 1] ^= 1;
  const changedTransaction = Buffer.from(receipt.raw_transaction, "hex");
  changedTransaction[changedTransaction.length - 1] ^= 1;
  for (const changed of [
    { ...receipt, signed_receipt: changedSignature.toString("hex") },
    { ...receipt, raw_transaction: changedTransaction.toString("hex") },
    { ...receipt, receipt_key: { ...receipt.receipt_key, public_key: "99".repeat(32) } },
    { ...receipt, receipt_key: { ...receipt.receipt_key, key_id: "99".repeat(32) } },
    { ...receipt, receipt_key: { ...receipt.receipt_key, first_height: "1001" } },
    { ...receipt, receipt_key: { ...receipt.receipt_key, first_height: "0", last_height: "999" } },
  ]) {
    await assert.rejects(verifySequencerReceipt(changed, status(), context(vector)), refuses("verification-failure"));
  }
  await assert.rejects(verifySequencerReceipt(receipt, status(), {
    ...context(vector), activityId: "99".repeat(32),
  }), refuses("verification-failure"));
  await assert.rejects(verifySequencerReceipt(receipt, status(), context(vector), "99".repeat(32)),
    refuses("verification-failure"));
  const reverted = vectors.find((entry) => entry.id === "V2-layerxledger-revert");
  assert.ok(reverted);
  await assert.rejects(verifySequencerReceipt(daemonReceipt(reverted), status(), context(reverted)),
    refuses("verification-failure"));

  for (const changed of [
    { canonical_bytes: receipt.signed_receipt, authorised_batch: {}, verification_level: "SequencerSigned" },
    { ...receipt, authorised_batch: {} },
    { ...receipt, signed_receipt: "0x" + receipt.signed_receipt },
    { ...receipt, raw_transaction: "" },
    { ...receipt, receipt_key: { ...receipt.receipt_key, public_key: "00" } },
    ...["-1", "01", "18446744073709551616"].map((first_height) => ({
      ...receipt, receipt_key: { ...receipt.receipt_key, first_height },
    })),
    { ...receipt, receipt_key: { ...receipt.receipt_key, first_height: "2000" } },
  ]) {
    await assert.rejects(verifySequencerReceipt(changed, status(), context(vector)), refuses("decode-failure"));
  }
}

async function daemonFinalityStagesExcludeSettlementAnchored() {
  for (const level of ["SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised"]) {
    const verified = await verifySequencerReceipt(daemonReceipt(vector, level), status(), context(vector));
    assert.equal(verified.evidence.verificationLevel, "receipted");
    assert.equal(verified.receiptDigest, vector.receiptDigest.slice(2));
  }
  for (const level of ["SettlementAnchored", "Unverified", "Unknown"]) {
    await assert.rejects(verifySequencerReceipt(daemonReceipt(vector, level), status(), context(vector)),
      refuses("verification-failure"));
  }
  for (const verificationStatus of [
    status(Rung.Submitted), status(Rung.Committed),
    { state: "pending", requested: Rung.Receipted, achieved: Rung.Submitted },
    status("SettlementAnchored"), status("BatchIncluded"), status("submitted"), status("committed"),
    { state: "pending", requested: "receipted", achieved: "submitted" },
  ]) {
    await assert.rejects(verifySequencerReceipt(daemonReceipt(), verificationStatus, context(vector)),
      refuses("verification-failure"));
  }
  for (const verificationStatus of [
    { state: "achieved", level: "SequencerSigned" },
    { state: "achieved", rung: "receipted", level: "SequencerSigned" },
  ]) {
    await assert.rejects(verifySequencerReceipt(daemonReceipt(), verificationStatus, context(vector)),
      refuses("decode-failure"));
  }
}

async function daemonProviderRefusesObsoleteStatusFields() {
  for (const verificationStatus of [
    { state: "achieved", level: "SequencerSigned" },
    { state: "achieved", level: "BatchIncluded" },
    { state: "achieved", level: "SettlementAnchored" },
    { state: "achieved", rung: Rung.Receipted, level: "SequencerSigned" },
  ]) {
    await assert.rejects(verifySequencerReceipt(daemonReceipt(), verificationStatus, context(vector)),
      refuses("decode-failure"));
  }
  await assert.rejects(verifySequencerReceipt(daemonReceipt(),
    { state: "pending", level: "SequencerSigned" }, context(vector)), refuses("verification-failure"));
}

test(daemonProviderConsumesCurrentProducerResponse);
test(daemonServiceProviderWiresCurrentReceiptEvidence);
test(daemonProviderBindsCurrentReceiptEvidence);
test(daemonProviderVerifiesSignedSequencerReceipt);
test(daemonFinalityStagesExcludeSettlementAnchored);
test(daemonProviderRefusesObsoleteStatusFields);
