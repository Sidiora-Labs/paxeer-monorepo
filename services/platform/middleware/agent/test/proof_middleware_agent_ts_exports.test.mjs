import "./production-sources.mjs";
/*
Proves: agent/src/daemon.ts decodeDaemonPrepared/decodeDaemonSubmission/signedActivityId and
agent/src/index.ts AgentMiddleware/AgentGrantMiddleware/verifyCommittedAgentPayment ->
@sidiora/layerx-agent-middleware exports -> committedReceiptConsumer and agents/src/
integration.ts, service-providers.ts, services.ts, daemon-providers.ts, tools.ts.

Repairs: daemon.ts and daemon-providers.ts emitted decimal receipt-key heights that
seller verification refuses; services.ts refused the canonical hexadecimal heights.
DaemonReceiptResolver expected a legacy status.level instead of the transport's rung.
AgentToolExecutor required an Ed25519-sized key for a secp256k1 spend; AgentMiddleware
accepted a committed replay on the wrong network. Integration now validates provider
methods, and loadAgentServiceProviders preserves a provider's destroy receiver.
AgentToolExecutor confused the SDK's signing digest with the portable receipt ID
and read legacy balance fields instead of verifying the signed PaymentExact event.

The signed input and observation are the committed daemon dispatch transcript.
The HTTP adapter replays its producer-owned submit and observation bytes;
no middleware, decoder, signature verifier, endpoint client or envelope transport is
replaced. SQLite bindings are closed and reopened before consumption.
*/
import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { secp256k1 } from "@noble/curves/secp256k1.js";
const {
  AgentGrantMiddleware, AgentMiddleware, decodeDaemonPrepared, decodeDaemonSubmission,
  decodeDaemonSubmitResult, resolveDaemonReceipt, signedActivityId, verifyAgentPayment,
} = await import("@sidiora/layerx-agent-middleware");
import { daemonFixture } from "./repair-support.mjs";
import {
  decodeRawEip1559, eip1559SigningHash, keccak256, SecretBytes,
} from "@sidiora/layerx-sdk";
import { ReceiptPayloadAuthority } from "@sidiora/layerx-seller-middleware";
import { committedReceiptConsumer } from "./committed-receipt.test.mjs";

// Execute the actual integration sources: the declared gate builds only the SDK,
// seller and agent dist entries. Transforming TS here avoids importing stale dist.
const integrationSources = new URL("../../../../../sdk/integrations/agents/src/", import.meta.url);
const sourceRoot = integrationSources.href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL?.startsWith(sourceRoot) && specifier.startsWith(".") && specifier.endsWith(".js")) {
      return nextResolve(new URL(specifier.slice(0, -3) + ".ts", context.parentURL).href, context);
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(sourceRoot) && url.endsWith(".ts")) {
      return { format: "module", source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }), shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});
const { createAgentIntegration } = await import(new URL("integration.ts", integrationSources));
const { loadAgentServiceProviders } = await import(new URL("service-providers.ts", integrationSources));
const { DaemonReceiptResolver } = await import(new URL("daemon-providers.ts", integrationSources));
const { authenticatedAgentTransport } = await import(new URL("config.ts", integrationSources));
const {
  LayerXServiceEndpoint, LayerXReceiptResolver, LayerXBudgetLedger, LayerXRemoteSigner,
} = await import(new URL("services.ts", integrationSources));
const { describeSpend, SPEND_TOOL } = await import(new URL("tools.ts", integrationSources));

const hex = (bytes) => Buffer.from(bytes).toString("hex");
const bytes = (value) => Buffer.from(value.slice(2), "hex");

import { createAgentServices } from "./example-services.mjs";
export { createAgentServices };

function preparationFor(input) {
  const tx = decodeRawEip1559(bytes(input.evidence.rawTransaction));
  const prepared = decodeDaemonPrepared({
    ...input.request.preparation,
    preparation_ref: "receipt-export-proof",
    chain_id: tx.chainId.toString(), nonce: tx.nonce.toString(),
    max_priority_fee_per_gas: tx.maxPriorityFeePerGas.toString(),
    max_fee_per_gas: tx.maxFeePerGas.toString(), gas_limit: tx.gasLimit.toString(),
    to: hex(tx.to), data: hex(tx.data), signing_hash: hex(eip1559SigningHash(tx)),
  }, input.request, 2);
  return { prepared, signature: tx.r.toString(16).padStart(64, "0") + tx.s.toString(16).padStart(64, "0") + tx.yParity.toString(16).padStart(2, "0") };
}

function daemonReceipt(evidence) {
  return {
    signed_receipt: evidence.signedReceipt.slice(2), raw_transaction: evidence.rawTransaction.slice(2),
    verification_level: "Receipted",
    receipt_key: {
      key_id: evidence.receiptKey.keyId.slice(2), public_key: evidence.receiptKey.publicKey.slice(2),
      first_height: BigInt(evidence.receiptKey.firstHeight).toString(),
      last_height: BigInt(evidence.receiptKey.lastHeight).toString(),
    },
  };
}

async function producer(input) {
  const { transcript, raw } = input;
  const wire = transcript.committed;
  const submission = await resolveDaemonReceipt(decodeDaemonSubmission(wire, wire.activity_id,
    wire.submission.submission_ref), raw, input.receiptKeys);
  const evidence = submission.portableReceipt;
  // Consensus verification is a separate boundary: a daemon's committed label is
  // decoded here, while the receipt consumer authenticates the receipted rung.
  const receipted = (await resolveDaemonReceipt(decodeDaemonSubmitResult(transcript.submit,
    transcript.submit.tx_hash, transcript.submit.preparation_ref), raw, input.receiptKeys)).portableReceipt;
  const result = await committedReceiptConsumer(receipted, input.reservation, input.request, input.receiptKeys);
  assert.equal(evidence.receiptDigest, receipted.receiptDigest);
  assert.equal(evidence.verificationLevel, "receipted");
  return { wire, submission, evidence: receipted, result };
}

export async function crossing(t) {
  const directory = mkdtempSync(join(tmpdir(), "middleware-exports-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const fixture = daemonFixture();
  const evidence = (await resolveDaemonReceipt(decodeDaemonSubmitResult(fixture.transcript.submit,
    fixture.transcript.submit.tx_hash, fixture.prepared.preparation_ref), fixture.raw, fixture.receiptKeys)).portableReceipt;
  const input = { ...fixture, request: fixture.input, evidence,
    reservation: { state: "committed", reservationId: fixture.transcript.submit.tx_hash,
      requestDigest: "77".repeat(32), amount: fixture.input.amount, asset: fixture.input.asset,
      receiptDigest: evidence.receiptDigest.slice(2) } };
  const calls = [];
  let corrupt = false;
  let tracked = input.transcript.committed;
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      calls.push({ path: request.url, body });
      const emitted = await producer(input);
      let value;
      if (request.url === "/receipt") {
        assert.equal(request.headers.authorization, "Bearer proof-local-services");
        assert.equal(body.receipt_ref, input.reservation.receiptDigest);
        value = structuredClone(emitted.evidence);
        if (corrupt) value.receiptKey.firstHeight = "01";
      } else if (request.url === "/budget") {
        assert.equal(body.action, "reserve", "a committed replay performs no budget transition");
        assert.equal(body.request.amount, input.request.amount);
        assert.equal(body.request.asset, input.request.asset);
        // Bind the real verified reservation to the caller's canonical spend digest.
        // The adapter has no ledger or execution implementation of its own.
        const committed = emitted.result.reservation;
        value = { kind: "reserved", reservation: {
          state: committed.state, reservation_id: committed.reservationId,
          request_digest: body.request.request_digest, amount: committed.amount,
          asset: committed.asset, receipt_digest: committed.receiptDigest,
        } };
      } else if (request.url === "/sign") {
        assert.equal(body.signing_hash, preparationFor(input).prepared.signing_hash);
        const recovered = secp256k1.sign(Buffer.from(body.signing_hash, "hex"), new Uint8Array(32).fill(1), { prehash: false, format: "recovered" });
        value = { signature: hex(recovered.subarray(1)) + recovered[0].toString(16).padStart(2, "0") };
      } else {
        assert.equal(request.url, "/v1/agent/rpc");
        assert.equal(request.headers.authorization, `LayerX-Key export-proof:lxp_live_${"33".repeat(32)}`);
        if (body.operation === "track") {
          assert.equal(body.request.submission_ref, emitted.submission.submission_ref);
          value = structuredClone(tracked);
          if (corrupt) delete value.receipt.canonical_bytes;
        } else {
          assert.equal(body.operation, "program.receipt");
          assert.equal(body.request.expected_activity_id, emitted.submission.tx_hash);
          assert.equal(body.request.idempotency_key, input.request.preparation.idempotency_key);
          value = { found: true, receipt: { ...daemonReceipt(emitted.evidence), verification_level: "SequencerSigned" } };
          if (corrupt) value.receipt.receipt_key.first_height = "01";
        }
        value = { request_id: body.request_id, value, verification_status: { state: "achieved", rung: value.submission?.rung ?? "receipted" } };
      }
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify(value));
    } catch (error) {
      response.writeHead(500, { "content-type": "application/json" });
      response.end(JSON.stringify({ code: "internal-fault", retry: "never", reason: String(error) }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(() => new Promise((resolve, reject) => {
    server.close((error) => error ? reject(error) : resolve());
    server.closeAllConnections();
  }));
  const url = `http://127.0.0.1:${server.address().port}`;
  const environment = {
    LAYERX_AGENT_RPC_URL: url, LAYERX_AGENT_SURFACE: "gateway",
    LAYERX_AGENT_SERVICES_MODULE: fileURLToPath(import.meta.url),
    LAYERX_SESSION_ID: "11".repeat(32), LAYERX_SESSION_TOKEN: "22".repeat(32),
    LAYERX_SESSION_GENERATION: "0", LAYERX_GATEWAY_KEY_ID: "export-proof",
    LAYERX_GATEWAY_KEY: `lxp_live_${"33".repeat(32)}`, LAYERX_PROTOCOL_VERSION: "2",
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(input.receiptKeys.map((key) => ({
      keyId: `0x${hex(key.keyId)}`, publicKey: `0x${hex(key.publicKey)}`,
      firstHeight: key.first_height.toString(), lastHeight: key.last_height.toString(),
    }))),
    LAYERX_TENANT: input.request.tenant, LAYERX_ACTOR: input.request.preparation.actor,
    LAYERX_AUTHORITY: input.request.preparation.authority, LAYERX_FEE_LIMIT: "1",
    LAYERX_WEBHOOK_PUBLIC_KEYS_JSON: JSON.stringify({ proof: "44".repeat(32) }),
    LAYERX_WEBHOOK_DELIVERY_STORE_PATH: join(directory, "deliveries.sqlite"),
  };
  return { input, directory, calls, environment, corrupt: () => { corrupt = true; },
    pending: () => { tracked = input.transcript.pending; },
    committed: () => { tracked = input.transcript.committed; } };
}

async function middlewareAgentExportsReachRealConsumers(t) {
  const path = await crossing(t);
  const providers = await loadAgentServiceProviders(path.environment);
  t.after(() => providers.destroy());
  const integration = createAgentIntegration({ environment: path.environment, ...providers });
  t.after(() => integration.destroy());
  assert.ok(integration.middleware instanceof AgentMiddleware);
  assert.ok(providers.receipts instanceof LayerXReceiptResolver);
  const { request, evidence, reservation, receiptKeys } = path.input;
  const input = { ...request };
  delete input.tenant;
  delete input.commitment;
  assert.match(request.signerPublicKey, new RegExp(SPEND_TOOL.inputSchema.properties.signerPublicKey.pattern));
  const { prepared, signature } = preparationFor(path.input);
  const canonical = await producer(path.input);
  assert.equal(canonical.submission.tx_hash, path.input.transcript.submit.tx_hash);
  assert.equal(canonical.submission.state, "Committed");
  assert.deepEqual(canonical.wire, path.input.transcript.committed);
  const signed = await providers.signer.sign(prepared);
  assert.equal(signed, signature);
  assert.equal(signedActivityId(prepared, signed, request.signerPublicKey), hex(keccak256(bytes(evidence.rawTransaction))));
  assert.throws(() => signedActivityId(prepared, signed, hex(secp256k1.getPublicKey(new Uint8Array(32).fill(2)))), { code: "verification-failure" });
  const expected = describeSpend((await producer(path.input)).result);
  for (let repeat = 1; repeat <= 2; repeat += 1) {
    const result = await integration.tools.execute("layerx_spend", input);
    assert.deepEqual(result, { ok: true, tool: "layerx_spend", result: expected });
    assert.equal(path.calls.filter((call) => call.path === "/budget").length, repeat);
    assert.equal(path.calls.filter((call) => call.path === "/receipt").length, repeat);
  }
  path.pending();
  const pending = await integration.tools.execute("layerx_track", { submissionRef: path.input.transcript.submit.tx_hash });
  assert.deepEqual(pending, { ok: true, tool: "layerx_track", result: {
    submissionRef: path.input.transcript.submit.tx_hash, state: "Unknown", verificationLevel: "Submitted",
  } });
  path.committed();
  const authenticated = authenticatedAgentTransport(path.environment, {
    agentRpcUrl: path.environment.LAYERX_AGENT_RPC_URL, tenant: request.tenant, requestTimeoutMs: 1000,
  });
  t.after(() => authenticated.destroy());
  for (let repeat = 0; repeat < 2; repeat += 1) {
    const result = await integration.tools.execute("layerx_track", { submissionRef: path.input.transcript.submit.tx_hash });
    assert.equal(result.ok, true);
    assert.equal(result.result.state, "Committed");
    assert.equal(result.result.verificationLevel, "sequencer-signed");
    const transported = await authenticated.transport.call({ plane: "agent", operation: "track",
      request: { submission_ref: path.input.transcript.submit.tx_hash } });
    assert.deepEqual(transported.value, path.input.transcript.committed);
    const decoded = await resolveDaemonReceipt(decodeDaemonSubmission(transported.value), path.input.raw, receiptKeys);
    assert.equal(decoded.portableReceipt.receiptDigest.slice(2), reservation.receiptDigest);
    assert.deepEqual(decoded.observation, path.input.transcript.committed);
  }
  assert.equal(path.calls.filter((call) => call.body.operation === "track").length, 5);
  const verifyInput = { receiptRef: reservation.receiptDigest, amount: request.amount, asset: request.asset, recipient: request.recipient };
  for (let repeat = 0; repeat < 2; repeat += 1) {
    const result = await integration.tools.execute("layerx_verify_receipt", verifyInput);
    assert.equal(result.ok, true);
    assert.equal(result.result.receiptDigest, reservation.receiptDigest);
    assert.equal(result.result.amount, request.amount);
  }
  assert.equal(path.calls.filter((call) => call.path === "/receipt").length, 6);
  for (const changed of [{ amount: "26" }, { asset: "00".repeat(32) }, { recipient: "00".repeat(32) }]) {
    assert.deepEqual(await integration.tools.execute("layerx_verify_receipt", { ...verifyInput, ...changed }), {
      ok: false, tool: "layerx_verify_receipt", code: "verification-failure",
    });
  }
  const wrongNetwork = await integration.tools.execute("layerx_spend", { ...input, networkId: "1" });
  assert.equal(wrongNetwork.result.kind, "unknown");
  const count = path.calls.length;
  assert.deepEqual(await integration.tools.execute("layerx_spend", { ...input, signerPublicKey: "11".repeat(32) }), {
    ok: false, tool: "layerx_spend", code: "invalid-tool-input",
  });
  assert.equal(path.calls.length, count);
  await assert.rejects(verifyAgentPayment(evidence, { ...request, recipient: "00".repeat(32) }, receiptKeys));
  path.corrupt();
  await assert.rejects(providers.receipts.resolve(reservation.receiptDigest), { code: "service-refused" });
  assert.deepEqual(await integration.tools.execute("layerx_track", { submissionRef: path.input.transcript.submit.tx_hash }), {
    ok: false, tool: "layerx_track", code: "decode-failure",
  });
  assert.deepEqual(await integration.tools.execute("layerx_verify_receipt", verifyInput), {
    ok: false, tool: "layerx_verify_receipt", code: "service-refused",
  });
  assert.throws(() => createAgentIntegration({ environment: path.environment, budgets: providers.budgets, signer: {}, receipts: providers.receipts }), { code: "missing-declared-key" });
  assert.throws(() => createAgentIntegration({ environment: path.environment, budgets: providers.budgets, signer: providers.signer, receipts: {} }), { code: "missing-declared-key" });
  const authority = new ReceiptPayloadAuthority(receiptKeys);
  const grant = new AgentGrantMiddleware({
    tenant: request.tenant, receiptKeys, budgets: providers.budgets, receipts: providers.receipts,
    draws: {
      execute(draw) {
        return authority.settle({
          principal: draw.principal, requestDigest: draw.requestDigest, idempotencyKey: draw.idempotencyKey,
          requirements: draw.requirements,
          payload: { x402Version: 2, accepted: draw.requirements, payload: evidence },
        });
      },
    },
  });
  // An exact payment must never enter the grant execution path.
  await assert.rejects(grant.execute({ requirements: { scheme: "exact" } }), { code: "invalid-request" });
}

async function middlewareAgentReceiptBindingSurvivesReopen(t) {
  const path = await crossing(t);
  const authenticated = authenticatedAgentTransport(path.environment, {
    agentRpcUrl: path.environment.LAYERX_AGENT_RPC_URL, tenant: path.input.request.tenant, requestTimeoutMs: 1000,
  });
  t.after(() => authenticated.destroy());
  const options = {
    transport: authenticated.transport, protocolVersion: 2,
    principal: { tenant: path.input.request.tenant, actor: path.input.request.preparation.actor, sessionId: path.environment.LAYERX_SESSION_ID },
    storePath: join(path.directory, "receipts.sqlite"),
  };
  const context = { idempotencyKey: path.input.request.preparation.idempotency_key, activityId: hex(keccak256(bytes(path.input.evidence.rawTransaction))) };
  let resolver = new DaemonReceiptResolver(options);
  t.after(() => resolver.close());
  const alias = "retained-receipt";
  await resolver.retainVerified({ ...context, receiptDigest: path.input.reservation.receiptDigest, receiptRef: alias });
  resolver.close();
  resolver = new DaemonReceiptResolver(options);
  for (let repeat = 1; repeat <= 2; repeat += 1) {
    const evidence = await resolver.resolve(alias);
    assert.deepEqual(evidence, path.input.evidence);
    const observed = await committedReceiptConsumer(evidence, path.input.reservation, path.input.request, path.input.receiptKeys);
    assert.equal(observed.kind, "verified");
    assert.equal(path.calls.length, repeat + 1, "each resolution fetches fresh authenticated evidence after reopen");
  }
  await assert.rejects(resolver.resolve("unknown-receipt"), { code: "unavailable-capability" });
  assert.equal(path.calls.length, 3);
  path.corrupt();
  await assert.rejects(resolver.resolve(alias), { code: "decode-failure" });
  assert.equal(path.calls.length, 4, "a retained binding must not hide malformed producer evidence");
  resolver.close();
  assert.throws(() => new DaemonReceiptResolver({ ...options, principal: { ...options.principal, actor: "different-actor" } }), { code: "internal-fault" });
}

test(middlewareAgentExportsReachRealConsumers);
test(middlewareAgentReceiptBindingSurvivesReopen);
