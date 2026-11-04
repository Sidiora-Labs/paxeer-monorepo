import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { z } from "zod";
import {
  AgentEnvelopeTransport, AgentSessionCredential, keccak256, PlatformSdkError, SecretBytes,
} from "@sidiora/layerx-sdk";
import { DaemonReceiptResolver } from "../dist/daemon-providers.js";

// Task 22.31.420, finding 038: repair038DaemonReceiptRoundTripAndReopen proves
// request/response verification and durable replay; the other tests prove refusals.
// Baseline gate not run: the explicit gate order requires implementation first.
// Earlier qualification: all three tests failed with ERR_OSSL_PEM_NO_START_LINE.
// Wiring requests: none. The bounded gate ledger records the verified revision.
const vectors = JSON.parse(readFileSync(
  new URL("../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8",
)).cases;
const vector = vectors.find((entry) => entry.id === "V1-pax-transfer");
assert.ok(vector);
const context = Object.freeze({
  idempotencyKey: "ab".repeat(32), activityId: vector.receiptJson.txHash.slice(2),
});
const principal = Object.freeze({
  tenant: "receipt-repair", actor: "receipt-agent", sessionId: "11".repeat(32),
});
// The alias digest hashes the signed bytes; the vector's receiptDigest is the signing digest.
const receiptDigest = "8a3d6e826696185b6615798f59acac1bad74623ca977342db0bf10901f074e0c";
assert.equal(Buffer.from(keccak256(Buffer.from(vector.signedReceipt.slice(2), "hex"))).toString("hex"), receiptDigest);

// Public, test-only TLS identity for the in-process mutually authenticated listener.
const certificate = `-----BEGIN CERTIFICATE-----
MIIBzzCCAXWgAwIBAgIUJpAiiRCNrOWEDJiyUUx3emCgO2MwCgYIKoZIzj0EAwIw
IzEhMB8GA1UEAwwYcmVjZWlwdC1sb29wYmFjay1maXh0dXJlMCAXDTI2MTEwNDAy
MjEwM1oYDzIxMjYxMDExMDIyMTAzWjAjMSEwHwYDVQQDDBhyZWNlaXB0LWxvb3Bi
YWNrLWZpeHR1cmUwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQLP4YAwRW/ugYn
L8UNOR8Rbh5OtSOfo1HHPcrny6T/TsfRPKN6HUxl3im9oVRL50mzLb+tvgcwMaP6
/2YeBxC0o4GEMIGBMB0GA1UdDgQWBBQejSAMNyPgq7gZIN/dgkYR0PCAeDAfBgNV
HSMEGDAWgBQejSAMNyPgq7gZIN/dgkYR0PCAeDAPBgNVHREECDAGhwR/AAABMB0G
A1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAPBgNVHRMBAf8EBTADAQH/MAoG
CCqGSM49BAMCA0gAMEUCIQCWlxaYvhHLb87fPJcpTtDrUd1Sh3enA8xQz+wiP4WP
FAIgOGcI46OguOqVzS9f4gtV1mmQvQlJn+AIQakIWP4lCaM=
-----END CERTIFICATE-----
`;
const privateKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgpOf7pN7TX9Owe1R0
yrY/zeyy2452LirZucw5pAbS17yhRANCAAQLP4YAwRW/ugYnL8UNOR8Rbh5OtSOf
o1HHPcrny6T/TsfRPKN6HUxl3im9oVRL50mzLb+tvgcwMaP6/2YeBxC0
-----END PRIVATE KEY-----
`;

// ActivityLookupRequest in agent_rpc_dispatch.rs denies unknown fields and
// accepts only the three advertised rungs. This is the loopback peer's wire schema.
const activityLookupRequest = z.strictObject({
  tenant: z.string().optional(), agent: z.string().optional(),
  idempotency_key: z.string().regex(/^[0-9a-f]{64}$/u),
  expected_activity_id: z.string().regex(/^[0-9a-f]{64}$/u),
  requested_rung: z.enum(["submitted", "receipted", "committed"]),
});

function daemonReceipt(entry = vector) {
  return {
    raw_transaction: entry.rawTransaction.slice(2), signed_receipt: entry.signedReceipt.slice(2),
    receipt_key: {
      key_id: entry.pinnedKey.keyId.slice(2), public_key: entry.pinnedKey.publicKey.slice(2),
      first_height: String(entry.pinnedKey.firstHeight), last_height: String(entry.pinnedKey.lastHeight),
    },
    verification_level: "SequencerSigned",
  };
}

function expectedEvidence() {
  return {
    format: "layerx-receipt-proof-v2", verificationLevel: "receipted",
    rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
    receiptDigest: `0x${receiptDigest}`,
    receiptKey: {
      keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
      firstHeight: `0x${BigInt(vector.pinnedKey.firstHeight).toString(16)}`,
      lastHeight: `0x${BigInt(vector.pinnedKey.lastHeight).toString(16)}`,
    },
  };
}

function refuses(code, retry = "never") {
  return (error) => {
    assert.ok(error instanceof PlatformSdkError, error?.stack ?? String(error));
    assert.equal(error.code, code);
    assert.equal(error.retry, retry);
    return true;
  };
}

async function daemonFixture(t) {
  const directory = mkdtempSync(join(tmpdir(), "receipt-repair-"));
  const token = new SecretBytes(new Uint8Array(32).fill(0x22));
  const clientCertificate = new SecretBytes(Uint8Array.from(Buffer.from(certificate)));
  const clientKey = new SecretBytes(Uint8Array.from(Buffer.from(privateKey)));
  const resolvers = [];
  const session = new AgentSessionCredential(principal.tenant, principal.sessionId, token, 0n);
  const fixture = {
    requests: [], replies: [], receipt: daemonReceipt(),
    verification: { state: "achieved", rung: "receipted" }, found: true,
    storePath: join(directory, "receipts.sqlite"),
  };
  const server = createServer({
    cert: certificate, key: privateKey, ca: certificate,
    requestCert: true, rejectUnauthorized: true,
  }, async (request, response) => {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    fixture.requests.push(body);
    assert.equal(request.socket.authorized, true);
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/rpc");
    assert.equal(request.headers["content-type"], "application/json");
    assert.equal(body.version, 1);
    assert.equal(body.operation, "program.receipt");
    assert.equal(body.idempotency_key, null);
    assert.deepEqual(body.credential, session.encode());
    const decoded = activityLookupRequest.safeParse(body.request);
    let reply;
    if (!decoded.success) {
      response.statusCode = 400;
      reply = {
        request_id: body.request_id, class: "ProtocolIncompatibility",
        protocol_result_code: null, retriability: "Terminal",
        reason: Object.hasOwn(body.request, "requested_verification_level")
          ? "envelope.unknown_field" : "envelope.malformed",
      };
    } else {
      assert.equal(decoded.data.idempotency_key, context.idempotencyKey);
      reply = {
        request_id: body.request_id,
        value: fixture.found ? { found: true, receipt: fixture.receipt } : { found: false },
        verification_status: fixture.verification,
      };
    }
    fixture.replies.push(reply);
    response.setHeader("content-type", "application/json");
    response.end(JSON.stringify(reply));
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    for (const resolver of resolvers) resolver.close();
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
    token.destroy(); clientCertificate.destroy(); clientKey.destroy();
    rmSync(directory, { recursive: true, force: true });
  });
  fixture.transport = () => new AgentEnvelopeTransport({
    endpoint: `https://127.0.0.1:${server.address().port}`, surface: "daemon", session,
    clientCertificate, clientKey, trustedCa: certificate, timeoutMs: 2000,
  });
  fixture.open = () => {
    const resolver = new DaemonReceiptResolver({
      transport: fixture.transport(), protocolVersion: 2, principal, storePath: fixture.storePath,
    });
    resolvers.push(resolver);
    return resolver;
  };
  return fixture;
}

function storedRows(path) {
  const database = new DatabaseSync(path, { readOnly: true });
  try {
    const read = (table) => database.prepare(`SELECT * FROM ${table}`).all().map((row) => ({ ...row }));
    return {
      context: read("receipt_context"), bindings: read("receipt_bindings"), aliases: read("receipt_aliases"),
    };
  } finally { database.close(); }
}

function assertCurrentRequests(fixture) {
  assert.ok(fixture.requests.length > 0);
  for (const body of fixture.requests) {
    assert.deepEqual(body.request, {
      idempotency_key: context.idempotencyKey, expected_activity_id: context.activityId,
      requested_rung: "receipted",
    });
  }
}

async function repair038DaemonReceiptRoundTripAndReopen(t) {
  const fixture = await daemonFixture(t);
  const resolver = fixture.open();
  const evidence = await resolver.resolveFor("payment-receipt", context);
  assert.deepEqual(evidence, expectedEvidence());
  assert.deepEqual(fixture.replies[0].verification_status, { state: "achieved", rung: "receipted" });
  resolver.close();
  await assert.rejects(resolver.resolve("payment-receipt"), refuses("unavailable-capability", "safe"));
  assert.deepEqual(storedRows(fixture.storePath), {
    context: [{ singleton: 1, tenant: principal.tenant, actor: principal.actor,
      session_id: principal.sessionId, protocol_version: 2 }],
    bindings: [{ receipt_digest: receiptDigest, idempotency_key: context.idempotencyKey, activity_id: context.activityId }],
    aliases: [{ reference: receiptDigest, receipt_digest: receiptDigest },
      { reference: "payment-receipt", receipt_digest: receiptDigest }],
  });
  const reopened = fixture.open();
  assert.deepEqual(await reopened.resolve("payment-receipt"), evidence);
  assert.deepEqual(await reopened.resolve(receiptDigest), evidence);
  assert.deepEqual(await reopened.resolveActivity(context), evidence);
  await reopened.retainVerified({ ...context, receiptDigest, receiptRef: "retained-payment" });
  reopened.close();
  const replayed = fixture.open();
  assert.deepEqual(await replayed.resolve("retained-payment"), evidence);
  assert.equal(fixture.requests.length, 6);
  assertCurrentRequests(fixture);
}

async function repair038RefusesObsoleteSelector(t) {
  const fixture = await daemonFixture(t);
  const transport = fixture.transport();
  const base = { idempotency_key: context.idempotencyKey, expected_activity_id: context.activityId };
  for (const request of [
    { ...base, requested_verification_level: "sequencer-signed" },
    { ...base, requested_rung: "receipted", requested_verification_level: "sequencer-signed" },
    { ...base, requested_rung: "sequencer-signed" }, base,
  ]) {
    await assert.rejects(transport.call({ plane: "agent", operation: "program.receipt", request }),
      refuses("protocol-incompatibility"));
    assert.equal(fixture.replies.at(-1).class, "ProtocolIncompatibility");
  }
  assert.equal(fixture.requests.length, 4);
  assert.equal(fixture.replies[0].reason, "envelope.unknown_field");
  assert.equal(fixture.replies[1].reason, "envelope.unknown_field");
  assert.deepEqual(await fixture.open().resolveFor("current-selector", context), expectedEvidence());
}

async function repair038VerifiesBeforeRecordingAndOnReplay(t) {
  const fixture = await daemonFixture(t);
  const resolver = fixture.open();
  const empty = storedRows(fixture.storePath);
  for (const verification of [
    { state: "achieved", rung: "submitted" },
    { state: "pending", requested: "receipted", achieved: "submitted" },
    { state: "achieved", level: "SequencerSigned" },
  ]) {
    fixture.verification = verification;
    await assert.rejects(resolver.resolveFor("refused-payment", context), refuses("verification-failure"));
    assert.deepEqual(storedRows(fixture.storePath), empty);
  }
  fixture.verification = { state: "achieved", rung: "receipted" };
  const signature = Buffer.from(vector.signedReceipt.slice(2), "hex");
  signature[signature.length - 1] ^= 1;
  const invalidReceipt = { ...daemonReceipt(), signed_receipt: signature.toString("hex") };
  for (const receipt of [
    invalidReceipt,
    { ...daemonReceipt(), raw_transaction: "00" },
    { ...daemonReceipt(), receipt_key: { ...daemonReceipt().receipt_key, public_key: "99".repeat(32) } },
  ]) {
    fixture.receipt = receipt;
    await assert.rejects(resolver.resolveFor("refused-payment", context), refuses("verification-failure"));
    assert.deepEqual(storedRows(fixture.storePath), empty);
  }
  fixture.receipt = daemonReceipt();
  await assert.rejects(resolver.resolveFor("wrong-activity", { ...context, activityId: "99".repeat(32) }),
    refuses("verification-failure"));
  assert.deepEqual(storedRows(fixture.storePath), empty);
  fixture.found = false;
  await assert.rejects(resolver.resolveFor("missing-payment", context), refuses("unavailable-capability", "safe"));
  assert.deepEqual(storedRows(fixture.storePath), empty);
  fixture.found = true;
  assert.deepEqual(await resolver.resolveFor("verified-payment", context), expectedEvidence());
  resolver.close();
  const persisted = storedRows(fixture.storePath);
  const reopened = fixture.open();
  fixture.receipt = invalidReceipt;
  await assert.rejects(reopened.resolve("verified-payment"), refuses("verification-failure"));
  assert.deepEqual(storedRows(fixture.storePath), persisted);
  fixture.receipt = daemonReceipt();
  assert.deepEqual(await reopened.resolve("verified-payment"), expectedEvidence());
}

test(repair038DaemonReceiptRoundTripAndReopen);
test(repair038RefusesObsoleteSelector);
test(repair038VerifiesBeforeRecordingAndOnReplay);
