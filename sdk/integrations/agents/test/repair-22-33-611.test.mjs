import assert from "node:assert/strict";
import { fork } from "node:child_process";
import { createHash, generateKeyPairSync, sign } from "node:crypto";
import { once } from "node:events";
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { secp256k1 } from "@noble/curves/secp256k1.js";
import {
  AgentEnvelopeTransport, AgentSessionCredential, LayerXKeyCredential, SecretBytes,
  eip1559SigningHash, encodeNativePrepareRequest, encodeUnsignedEip1559, keccak256,
} from "@sidiora/layerx-sdk";
import { decodeDaemonPrepared, signedTransactionBytes } from "@sidiora/layerx-agent-middleware";
import { DaemonPreparationBudget } from "../dist/daemon-budget.js";
import { DaemonReceiptResolver } from "../dist/daemon-providers.js";
import { createAgentIntegration } from "../dist/integration.js";
import { FileVerifiedEventQueue, LayerXMcpServer } from "../dist/mcp.js";
import { LayerXRemoteSigner, LayerXServiceEndpoint } from "../dist/services.js";
import { AgentWebhookGateway, SingleProcessWebhookDeliveryStore } from "../dist/webhooks.js";

export const REPAIR_TASK_RECORD = Object.freeze({
  task: "22.33.611",
  findings: {
    "001": ["repair001RestartsBeforeAndAfterRemoteCalls", "repair001RetainsExactRequestIdentity"],
    "002": ["repair002KilledQueueOwnerReopensPersistedEvents", "repair002ConcurrentQueueOwnersPreserveEvents", "repair002SignedWebhookToQueueToMcpDrain"],
    "040": ["repair040CapacityOneOverlappingDeliveries", "repair040CompletedClaimsSurviveCapacityPressure"],
  },
  baseline: "The unchanged continued tree at f4c6920b93041131509993cbdb5c86f328d4b4e6 failed repair001RestartsBeforeAndAfterRemoteCalls and repair001RetainsExactRequestIdentity in /out/logs/verify-1.log (exit 1); the five queue/webhook tests passed.",
  recovery: "Preparation and signing reconcile by replaying the exact retained request with the owner's original idempotency identity; signed submissions retain their signature as well as their submit key.",
  retention: "Completed process-local webhook claims are retained for the store's lifetime; full stores refuse new IDs with retryable HTTP 503 while existing IDs remain queryable.",
  wiring: [
    "services/platform/middleware/agent/src/daemon.ts — agent middleware domain: native preparation/signing adapter wiring remains outside this task; this proof uses the recorded EVM owner response consumed by decodeDaemonPrepared (decision 2520).",
    "services/agent/sdk/typescript/src/production.ts — SDK domain: SecretBytes uses Buffer.slice views and clears a Buffer-backed secret on first use; the loopback fixture follows production integration configuration by passing TextEncoder Uint8Arrays.",
  ],
  qualification: "The budget strictly decodes the EVM owner response and binds the unsigned transaction digest and preparation reference to the retained purpose; native middleware wiring is not a prerequisite for the loopback recovery proof.",
  evidence: "The bounded hf-build/hf-done ledger records the exact verified revision, command, exit and private log.",
});

const hex = (value) => Buffer.from(value).toString("hex");
const sha256 = (value) => createHash("sha256").update(value).digest("hex");
const principal = { tenant: "budget-recovery", actor: "agent:repair", sessionId: "11".repeat(32) };
const signingSecret = new Uint8Array(32).fill(1);
const signerPublicKey = hex(secp256k1.getPublicKey(signingSecret));

function canonical(value) {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonical(value[key])}`).join(",")}}`;
  }
  return JSON.stringify(value);
}

function nativeCanonical() {
  return encodeUnsignedEip1559({
    chainId: 125n, nonce: 1n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n, gasLimit: 100000n,
    to: Buffer.alloc(20, 0x10), data: Buffer.from("deadbeef", "hex"), value: 0n, accessList: [],
  });
}

function paymentRequest() {
  return {
    tenant: principal.tenant, networkId: "125", signerPublicKey, authorityHex: "01",
    submitIdempotencyKey: "33".repeat(32), approvalCurrentSequence: "1",
    asset: "44".repeat(32), amount: "1", recipient: "55".repeat(32),
    preparation: encodeNativePrepareRequest({
      variant: "native_v1", activity: { version: "1", module: "9", ordinal: "1" },
      actor: principal.actor, authority: "authority", account_sequence: "1", not_before: "0",
      not_after: "1000", idempotency_key: "22".repeat(32), fee_limit: "1", payload: "01",
      payload_hash: sha256(Buffer.concat([Buffer.from("LXP/v1/payload-hash\0"), Buffer.of(1)])),
      capability_id: "77".repeat(32), purpose: {
        purpose: {
          version: "1", tenant: principal.tenant, agent_did: principal.actor, session_id: principal.sessionId,
          generation: "1", expires_at_ms: "1000000", capability_id: "77".repeat(32),
          preparation_id: sha256(nativeCanonical()), canonical_digest: sha256(nativeCanonical()), commitment: "88".repeat(32),
        },
        owner_public_key: "99".repeat(32), signature: "aa".repeat(64),
      },
    }),
  };
}

function preparedResponse(request) {
  const p = request.preparation;
  const tx = {
    chainId: 125n, nonce: 1n, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n, gasLimit: 100000n,
    to: Buffer.alloc(20, 0x10), data: Buffer.from("deadbeef", "hex"), value: 0n, accessList: [],
  };
  return {
    preparation_ref: p.purpose.purpose.preparation_id,
    chain_id: "125", nonce: "1", max_priority_fee_per_gas: "1", max_fee_per_gas: "2", gas_limit: "100000",
    to: hex(tx.to), data: hex(tx.data), signing_hash: hex(eip1559SigningHash(tx)),
    activity_type: String(BigInt(p.activity.module) * 65536n + BigInt(p.activity.ordinal)),
    ...Object.fromEntries(["actor", "authority", "account_sequence", "not_before", "not_after", "fee_limit",
      "payload", "payload_hash", "idempotency_key"].map((field) => [field, p[field]])),
  };
}

function retainedRecord(path) {
  const database = new DatabaseSync(path, { readOnly: true });
  try {
    const rows = database.prepare("SELECT record FROM admissions").all();
    assert.equal(rows.length, 1);
    return JSON.parse(rows[0].record);
  } finally { database.close(); }
}

async function budgetFixture(t, fault) {
  const directory = mkdtempSync(join(tmpdir(), "repair-budget-"));
  const request = paymentRequest();
  const response = preparedResponse(request);
  const digest = sha256(canonical(request));
  const sessionToken = new SecretBytes(new Uint8Array(32).fill(0x12));
  const gatewaySecret = new SecretBytes(new TextEncoder().encode(`lxp_live_${"13".repeat(32)}`));
  const serviceToken = new SecretBytes(new TextEncoder().encode("repair-signing-service"));
  const session = new AgentSessionCredential(principal.tenant, principal.sessionId, sessionToken, 1n);
  const credential = new LayerXKeyCredential("budget-repair", gatewaySecret);
  const fixture = {
    request, digest, fault, faultUsed: false, errors: [], calls: [],
    preparations: new Map(), signatures: new Map(), submissions: new Map(),
    storePath: join(directory, "budget.sqlite"), current: undefined,
  };
  const components = [];
  const server = createServer(async (incoming, outgoing) => {
    try {
      const chunks = [];
      for await (const chunk of incoming) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      const operation = incoming.url === "/sign" ? "sign" : body.operation;
      fixture.calls.push({ operation, body });
      assert.equal(incoming.method, "POST");
      if (operation === "sign") {
        assert.equal(incoming.headers.authorization, "Bearer repair-signing-service");
        assert.equal(incoming.headers["idempotency-key"], response.preparation_ref);
      } else {
        assert.equal(incoming.url, "/v1/agent/rpc");
        assert.deepEqual(body.credential, session.encode());
        assert.equal(incoming.headers.authorization, `LayerX-Key budget-repair:lxp_live_${"13".repeat(32)}`);
      }
      const interrupt = !fixture.faultUsed && fixture.fault?.operation === operation;
      if (interrupt && fixture.fault.when === "before") {
        fixture.faultUsed = true;
        fixture.current.close();
        incoming.socket.destroy();
        return;
      }
      let value;
      if (operation === "prepare") {
        assert.deepEqual(body.request, request.preparation);
        assert.equal(body.idempotency_key, request.preparation.idempotency_key);
        const previous = fixture.preparations.get(body.idempotency_key);
        if (previous) assert.deepEqual(previous.request, body.request);
        else fixture.preparations.set(body.idempotency_key, { request: body.request, result: response });
        value = fixture.preparations.get(body.idempotency_key).result;
      } else if (operation === "approval.get") {
        assert.deepEqual(body.request, { variant: "native_v1", approval_id: response.preparation_ref });
        value = { version: "1", approval_id: response.preparation_ref, held_digest: "ab".repeat(32),
          activity: request.preparation.activity, state: "NotRequired", submission_ref: null };
      } else if (operation === "sign") {
        const key = incoming.headers["idempotency-key"];
        const previous = fixture.signatures.get(key);
        if (previous) assert.deepEqual(previous.request, body);
        else {
          const prepared = decodeDaemonPrepared(response, request, 2);
          assert.deepEqual(body, {
            preparation_ref: prepared.preparation_ref, signing_hash: prepared.signing_hash,
            disclosure: prepared.disclosure, expiry: prepared.expiry,
          });
          const recovered = secp256k1.sign(Buffer.from(body.signing_hash, "hex"), signingSecret,
            { prehash: false, format: "recovered" });
          fixture.signatures.set(key, { request: body,
            result: { signature: hex(Buffer.concat([recovered.subarray(1), recovered.subarray(0, 1)])) } });
        }
        value = fixture.signatures.get(key).result;
      } else if (operation === "submit") {
        assert.equal(body.idempotency_key, request.submitIdempotencyKey);
        const previous = fixture.submissions.get(body.idempotency_key);
        if (previous) assert.deepEqual(previous.request, body.request);
        else {
          assert.equal(body.request.signature, fixture.signatures.get(response.preparation_ref).result.signature);
          assert.equal(body.request.approval_release_ref, null);
          const raw = signedTransactionBytes(decodeDaemonPrepared(response, request, 2), body.request.signature, signerPublicKey);
          const activityId = hex(keccak256(raw));
          fixture.submissions.set(body.idempotency_key, { request: body.request, raw,
            result: { activity_id: activityId, submission: {
              tx_hash: activityId, submission_ref: activityId, state: "Submitted", rung: "submitted",
              evidence: [], transitions: [],
            }, receipt: null } });
        }
        value = fixture.submissions.get(body.idempotency_key).result;
      } else if (operation === "track") {
        value = fixture.submissions.get(request.submitIdempotencyKey).result;
        assert.deepEqual(body.request, { submission_ref: value.activity_id });
      } else {
        throw new Error(`unexpected operation ${operation}`);
      }
      if (interrupt && fixture.fault.when === "after") {
        fixture.faultUsed = true;
        fixture.current.close();
      }
      outgoing.setHeader("content-type", "application/json");
      outgoing.end(JSON.stringify(operation === "sign" ? value : {
        request_id: body.request_id, value, verification_status: { state: "achieved", rung: "submitted" },
      }));
    } catch (error) {
      fixture.errors.push(error);
      outgoing.statusCode = 500;
      outgoing.end("fixture assertion failed");
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const endpoint = `http://127.0.0.1:${server.address().port}`;
  fixture.endpoint = endpoint;
  fixture.open = () => {
    const transport = new AgentEnvelopeTransport({ endpoint, session, gatewayCredential: credential, timeoutMs: 1000 });
    const budget = new DaemonPreparationBudget({ transport, principal, storePath: fixture.storePath });
    const receipts = new DaemonReceiptResolver({ transport, principal, protocolVersion: 2,
      storePath: join(directory, "receipts.sqlite") });
    const signer = new LayerXRemoteSigner(new LayerXServiceEndpoint({
      url: `${endpoint}/sign`, token: serviceToken, timeoutMs: 1000,
    }));
    components.push(budget, receipts);
    fixture.current = budget;
    return { budget, services: { signer, receipts, receiptKeys: [] } };
  };
  t.after(async () => {
    for (const component of components) component.close();
    await new Promise((resolve) => { server.close(resolve); server.closeAllConnections(); });
    sessionToken.destroy(); gatewaySecret.destroy(); serviceToken.destroy();
    rmSync(directory, { recursive: true, force: true });
    assert.deepEqual(fixture.errors, []);
  });
  return fixture;
}

async function repair001RestartsBeforeAndAfterRemoteCalls(t) {
  for (const operation of ["prepare", "approval.get", "sign", "submit", "track"]) {
    for (const when of ["before", "after"]) {
      await t.test(`${operation} ${when}`, async (t) => {
        const fixture = await budgetFixture(t, { operation, when });
        let { budget, services } = fixture.open();
        const interrupted = await budget.spendPrepared(fixture.request, fixture.digest, services);
        assert.ok(["admission-unknown", "unknown", "pending"].includes(interrupted.state));
        assert.equal(fixture.faultUsed, true, "the chosen remote boundary was reached");
        const stopped = retainedRecord(fixture.storePath);
        assert.equal(stopped.stage, operation === "prepare" ? "preparing"
          : operation === "approval.get" ? "prepared" : operation === "sign" ? "signing" : "signed");
        assert.deepEqual(stopped.request, fixture.request);
        assert.equal(stopped.requestDigest, fixture.digest);
        ({ budget, services } = fixture.open());
        const recovered = await budget.spendPrepared(fixture.request, fixture.digest, services);
        assert.equal(recovered.state, "pending");
        assert.equal(recovered.admissionObserved, true);
        assert.equal(fixture.preparations.size, 1);
        assert.equal(fixture.signatures.size, 1);
        assert.equal(fixture.submissions.size, 1);
        const retained = retainedRecord(fixture.storePath);
        assert.equal(retained.stage, "signed");
        assert.equal(retained.submissionRef, recovered.submission.submission_ref);
        assert.equal(retained.activityId, recovered.submission.tx_hash);
        assert.equal(retained.signature, fixture.signatures.values().next().value.result.signature);
        assert.deepEqual(retained.request, fixture.request);
        const submitted = structuredClone([...fixture.submissions.values()].map(({ request, result }) => ({ request, result })));
        budget.close();
        ({ budget, services } = fixture.open());
        assert.equal((await budget.spendPrepared(fixture.request, fixture.digest, services)).state, "pending");
        assert.deepEqual([...fixture.submissions.values()].map(({ request, result }) => ({ request, result })), submitted);
        assert.equal(fixture.calls.filter((call) => call.operation === "prepare").length,
          operation === "prepare" ? 2 : 1);
        assert.equal(fixture.calls.filter((call) => call.operation === "sign").length,
          operation === "sign" ? 2 : 1);
      });
    }
  }
}

async function repair001RetainsExactRequestIdentity(t) {
  const fixture = await budgetFixture(t, { operation: "sign", when: "after" });
  let { budget, services } = fixture.open();
  assert.equal((await budget.spendPrepared(fixture.request, fixture.digest, services)).state, "unknown");
  assert.equal(fixture.faultUsed, true, "the signing owner accepted the original request before restart");
  assert.equal(retainedRecord(fixture.storePath).stage, "signing");
  assert.equal(fixture.signatures.size, 1);
  ({ budget, services } = fixture.open());
  const before = fixture.calls.length;
  for (const changed of [
    { ...fixture.request, amount: "2" },
    { ...fixture.request, submitIdempotencyKey: "ef".repeat(32) },
    { ...fixture.request, approvalReleaseRef: "ee".repeat(32) },
    { ...fixture.request, preparation: { ...fixture.request.preparation, idempotency_key: "ed".repeat(32) } },
  ]) {
    await assert.rejects(budget.spendPrepared(changed, fixture.digest, services), { code: "idempotency-conflict" });
  }
  assert.equal(fixture.calls.length, before);
  const reordered = Object.fromEntries(Object.entries(fixture.request).reverse());
  assert.equal((await budget.spendPrepared(reordered, fixture.digest, services)).state, "pending");
  assert.equal(fixture.submissions.size, 1);
}

async function queueOwner(path) {
  const queue = new FileVerifiedEventQueue(path, 16);
  const event = { get interrupted() {
    process.send({ lock: "held" });
    Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0);
    throw new Error("owner must be killed while holding its production queue lock");
  } };
  await queue.push("interrupted", event);
}

async function repair002KilledQueueOwnerReopensPersistedEvents(t) {
  const directory = mkdtempSync(join(tmpdir(), "repair-queue-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const path = join(directory, "events.json");
  const first = new FileVerifiedEventQueue(path, 16);
  const event = { kind: "payment", amount: "1" };
  await first.push("retained", event);
  const persisted = readFileSync(path, "utf8");
  const child = fork(fileURLToPath(import.meta.url), ["--queue-owner", path], {
    execArgv: [], stdio: ["ignore", "ignore", "pipe", "ipc"],
  });
  const exited = once(child, "exit");
  t.after(async () => {
    if (child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
    await exited;
  });
  let childErrors = "";
  child.stderr.on("data", (chunk) => { childErrors += chunk.toString(); });
  const ready = await Promise.race([
    once(child, "message", { signal: AbortSignal.timeout(5000) }).then(([message]) => message),
    exited.then(() => { throw new Error(`queue owner exited before locking: ${childErrors}`); }),
  ]);
  assert.deepEqual(ready, { lock: "held" });
  const reopened = new FileVerifiedEventQueue(path, 16);
  let counted = false;
  const waiting = reopened.count().then((count) => { counted = true; return count; });
  await new Promise((resolve) => setTimeout(resolve, 75));
  assert.equal(counted, false, "a live owner still excludes concurrent reads");
  assert.equal(child.kill("SIGKILL"), true);
  assert.deepEqual(await exited, [null, "SIGKILL"]);
  assert.equal(await waiting, 1);
  assert.equal(readFileSync(path, "utf8"), persisted);
  assert.equal(existsSync(`${path}.coordination.sqlite3`), true);
  // A sentinel left by the previous implementation must not require manual cleanup.
  writeFileSync(`${path}.lock`, "", { mode: 0o600 });
  await reopened.push("next", { kind: "settled" });
  await reopened.push("retained", event);
  const final = new FileVerifiedEventQueue(path, 16);
  assert.deepEqual(await final.drain(16), [
    { deliveryId: "retained", event }, { deliveryId: "next", event: { kind: "settled" } },
  ]);
  assert.equal(await final.count(), 0);
}

async function repair002ConcurrentQueueOwnersPreserveEvents(t) {
  const directory = mkdtempSync(join(tmpdir(), "repair-queue-concurrent-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const path = join(directory, "events.json");
  const queues = Array.from({ length: 8 }, () => new FileVerifiedEventQueue(path, 16));
  await Promise.all(queues.map((queue, index) => queue.push(`event-${index}`, { index })));
  assert.equal(await queues[0].count(), 8);
  const drained = (await Promise.all(queues.map((queue) => queue.drain(1)))).flat();
  assert.equal(new Set(drained.map((entry) => entry.deliveryId)).size, 8);
  for (const entry of drained) assert.equal(entry.deliveryId, `event-${entry.event.index}`);
  assert.equal(await new FileVerifiedEventQueue(path, 16).count(), 0);
}

async function webhookFixture() {
  const pair = generateKeyPairSync("ed25519");
  const publicKey = new Uint8Array(pair.publicKey.export({ type: "spki", format: "der" }).subarray(-32));
  const now = 1_800_000_000_000;
  const deliveries = new SingleProcessWebhookDeliveryStore(() => now, 1);
  const gateway = new AgentWebhookGateway({ deliveries, now: () => now,
    webhook: { publicKeys: { repair: publicKey }, maximumAgeMs: 300000, leaseMs: 60000 } });
  return { gateway, deliveries, now, publicKey, delivery(id, event = { kind: "payment", amount: "1" }) {
    const body = Buffer.from(JSON.stringify(event));
    const timestamp = String(now / 1000);
    return { body, headers: {
      "layerx-webhook-id": id, "layerx-webhook-timestamp": timestamp, "layerx-webhook-key-id": "repair",
      "layerx-webhook-signature": `v1=${sign(null, Buffer.concat([Buffer.from(`${id}.${timestamp}.`), body]), pair.privateKey).toString("base64")}`,
    } };
  } };
}

async function repair040CapacityOneOverlappingDeliveries() {
  const fixture = await webhookFixture();
  const first = fixture.delivery("first");
  const second = fixture.delivery("second");
  let entered;
  const started = new Promise((resolve) => { entered = resolve; });
  let release;
  const held = new Promise((resolve) => { release = resolve; });
  const calls = [];
  const handler = { async handle(event, id) {
    calls.push({ event, id });
    if (id === "first" && calls.length === 1) {
      entered();
      await held;
    }
  } };
  const pending = fixture.gateway.respond(first.body, first.headers, handler);
  try {
    await started;
    const refused = await fixture.gateway.respond(second.body, second.headers, handler);
    assert.equal(refused.status, 503);
    assert.equal(refused.headers["retry-after"], "1");
    assert.deepEqual(JSON.parse(refused.body), { error: "service-refused" });
    const replay = await fixture.gateway.respond(first.body, first.headers, handler);
    assert.equal(replay.status, 409);
    assert.deepEqual(JSON.parse(replay.body), { outcome: "processing" });
    assert.deepEqual(calls, [{ id: "first", event: { kind: "payment", amount: "1" } }]);
  } finally { release(); }
  assert.deepEqual(JSON.parse((await pending).body), { outcome: "processed" });
  assert.deepEqual(JSON.parse((await fixture.gateway.respond(first.body, first.headers, handler)).body), { outcome: "duplicate" });
  assert.equal(calls.length, 1);
}

async function repair040CompletedClaimsSurviveCapacityPressure() {
  const fixture = await webhookFixture();
  const first = fixture.delivery("completed");
  let handled = 0;
  const handler = { async handle() { handled += 1; } };
  assert.equal(await fixture.gateway.consume(first.body, first.headers, handler), "processed");
  const second = fixture.delivery("new");
  assert.equal((await fixture.gateway.respond(second.body, second.headers, handler)).status, 503);
  assert.equal(await fixture.gateway.consume(first.body, first.headers, handler), "duplicate");
  const changed = fixture.delivery("completed", { kind: "payment", amount: "2" });
  assert.equal((await fixture.gateway.respond(changed.body, changed.headers, handler)).status, 409);
  assert.equal(handled, 1);
  let clock = 100;
  const expiring = new SingleProcessWebhookDeliveryStore(() => clock, 1);
  const digest = "ab".repeat(32);
  assert.equal(await expiring.claim({ deliveryId: "expired", payloadDigest: digest, leaseUntilMs: 101 }), "claimed");
  clock = 101;
  assert.equal(await expiring.claim({ deliveryId: "replacement", payloadDigest: digest, leaseUntilMs: 102 }), "claimed");
  await expiring.release("replacement", digest);
  assert.equal(await expiring.claim({ deliveryId: "released", payloadDigest: digest, leaseUntilMs: 102 }), "claimed");
}

async function repair002SignedWebhookToQueueToMcpDrain(t) {
  const directory = mkdtempSync(join(tmpdir(), "repair-mcp-webhook-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const fixture = await webhookFixture();
  const path = join(directory, "events.json");
  const owner = await budgetFixture(t);
  const { budget, services } = owner.open();
  const vectors = JSON.parse(readFileSync(
    new URL("../../../platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url), "utf8",
  ));
  const integration = createAgentIntegration({
    environment: {
      LAYERX_AGENT_RPC_URL: owner.endpoint, LAYERX_AGENT_SURFACE: "gateway",
      LAYERX_SESSION_ID: principal.sessionId, LAYERX_SESSION_TOKEN: "12".repeat(32),
      LAYERX_SESSION_GENERATION: "1", LAYERX_GATEWAY_KEY_ID: "budget-repair",
      LAYERX_GATEWAY_KEY: `lxp_live_${"13".repeat(32)}`, LAYERX_PROTOCOL_VERSION: "2",
      LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([vectors.cases[0].pinnedKey]),
      LAYERX_TENANT: principal.tenant, LAYERX_ACTOR: principal.actor,
      LAYERX_AUTHORITY: "authority", LAYERX_FEE_LIMIT: "1",
      LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ repair: hex(fixture.publicKey) }),
    },
    preparationBudgets: budget, signer: services.signer, receipts: services.receipts,
    deliveries: fixture.deliveries, now: () => fixture.now,
  });
  t.after(() => integration.destroy());
  const server = new LayerXMcpServer({ integration, eventQueue: new FileVerifiedEventQueue(path) });
  const delivery = fixture.delivery("verified-queue");
  assert.deepEqual(JSON.parse((await server.deliver(delivery.body, delivery.headers)).body), { outcome: "processed" });
  const reopened = new LayerXMcpServer({
    integration, eventQueue: new FileVerifiedEventQueue(path),
  });
  assert.deepEqual(await reopened.drainVerifiedEvents({}), {
    tool: "layerx_events", result: { events: [{ deliveryId: "verified-queue", event: { kind: "payment", amount: "1" } }], remaining: 0 },
  });
}

if (process.argv[2] === "--queue-owner") {
  await queueOwner(process.argv[3]);
} else {
  test(repair001RestartsBeforeAndAfterRemoteCalls);
  test(repair001RetainsExactRequestIdentity);
  test(repair002KilledQueueOwnerReopensPersistedEvents);
  test(repair002ConcurrentQueueOwnersPreserveEvents);
  test(repair040CapacityOneOverlappingDeliveries);
  test(repair040CompletedClaimsSurviveCapacityPressure);
  test(repair002SignedWebhookToQueueToMcpDrain);
}
