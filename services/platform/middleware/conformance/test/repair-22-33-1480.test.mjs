import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import test from "node:test";

// The declared gate needs no dist build. Execute the committed production sources
// in-process, including their enums and parameter properties, with Node's loader.
const repository = new URL("../../../../../", import.meta.url);
const packages = new Map([
  ["@sidiora/layerx-sdk", "services/agent/sdk/typescript/src/"],
  ["@sidiora/layerx-agent-integrations", "sdk/integrations/agents/src/"],
  ...["agent", "buyer", "merchant", "seller"].map((name) => [
    `@sidiora/layerx-${name}-middleware`, `services/platform/middleware/${name}/src/`,
  ]),
].map(([name, path]) => [name, new URL(path, repository)]));
const conformance = new URL("../src/", import.meta.url);
const roots = [...packages.values(), conformance].map((url) => url.href);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (packages.has(specifier)) {
      return { url: new URL("index.ts", packages.get(specifier)).href, shortCircuit: true };
    }
    if (context.parentURL === new URL("../service.mjs", import.meta.url).href && specifier === "../seller/dist/rpc.js") {
      return { url: new URL("rpc.ts", packages.get("@sidiora/layerx-seller-middleware")).href, shortCircuit: true };
    }
    if (context.parentURL === new URL("../service.mjs", import.meta.url).href
      && ["./dist/receipts.js", "./dist/support.js"].includes(specifier)) {
      return { url: new URL(specifier.replace("./dist/", "").replace(/\.js$/u, ".ts"), conformance).href, shortCircuit: true };
    }
    if (roots.some((root) => context.parentURL?.startsWith(root))
      && specifier.startsWith(".") && specifier.endsWith(".js")) {
      const source = new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL);
      if (existsSync(source)) return { url: source.href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (roots.some((root) => url.startsWith(root)) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});

const { runMerchantServiceScenarios, runAgentServiceScenarios, servicePayee, SERVICE_PAYEE_ACCOUNT }
  = await import("../service.mjs");
const { ConformanceSequencer, buildSignedReceipt, fixedBytes, toHex } = await import("../src/receipts.ts");
const { FixedReceiptKeys, Suite } = await import("../src/support.ts");
const { HostedGatewayRpc, PaymentRpc } = await import("../../seller/src/rpc.ts");

async function conformance_node_router_fixture_uses_root_and_separate_hosted_identity() {
  const sequencer = await ConformanceSequencer.generate();
  const facts = {
    amount: 250_000n, asset: fixedBytes(0xbb), payTo: await servicePayee(SERVICE_PAYEE_ACCOUNT),
    from: fixedBytes(0xcc), batchId: fixedBytes(0xd1),
    previousStateRoot: fixedBytes(0xd2), resultingStateRoot: fixedBytes(0xd3),
  };
  const receipt = await buildSignedReceipt(sequencer, facts);
  const receiptKeys = await new FixedReceiptKeys([sequencer.receiptKey()]).resolve();
  const suite = new Suite();
  const observed = await runMerchantServiceScenarios(suite, sequencer, facts, receipt, receiptKeys);
  assert.equal(suite.results().length, 3, "checkout, verified payment/restart and tampering must all execute");
  assert.deepEqual(suite.results().filter((result) => !result.ok), []);
  assert.equal(new URL(observed.nodeUrl).pathname, "/");
  assert.equal(new URL(observed.hostedGatewayUrl).pathname, "/");
  assert.notEqual(new URL(observed.nodeUrl).origin, new URL(observed.hostedGatewayUrl).origin);
  assert.equal(observed.hosted.length, 1);
  const identity = observed.hosted[0];
  assert.equal(identity.path, "/");
  assert.equal(identity.authorization, "Bearer identity-conformance-token");
  assert.deepEqual(identity.request.params, [toHex(facts.payTo)]);
  assert.equal(identity.request.method, "px_resolveAccount");
  assert.deepEqual(identity.response.body, { jsonrpc: "2.0", id: identity.request.id, result: {
    evm_address: null, pax_address: null, layerx_did: null, layerx_account: toHex(facts.payTo), bound: true,
  } });
  assert.ok(observed.node.length >= 3, "both raw transaction and signed receipt must travel over the node transport");
  assert.ok(observed.node.some((call) => call.request.method === "eth_getRawTransactionByHash"));
  assert.ok(observed.node.some((call) => call.request.method === "pax_sequencerReceipt"));
  for (const call of observed.node) {
    assert.equal(call.path, "/");
    assert.equal(call.authorization, undefined, "hosted credentials must not reach the node");
    assert.equal(call.request.jsonrpc, "2.0");
    assert.equal(call.response.status, 200);
    assert.equal(call.response.body.jsonrpc, "2.0");
    assert.equal(call.response.body.id, call.request.id);
    assert.equal(call.request.params.length, 1);
    assert.match(call.request.params[0], /^0x[0-9a-f]{64}$/u);
    assert.ok(["eth_getRawTransactionByHash", "pax_sequencerReceipt"].includes(call.request.method));
  }
  assert.equal(observed.settlements.length, 1, "restart replay must not settle or debit twice");
  const settlement = observed.settlements[0];
  assert.equal(settlement.path, "/settle");
  assert.equal(settlement.authorization, "Bearer merchant-conformance-token");
  assert.equal(settlement.request.requirements.amount, facts.amount.toString());
  assert.equal(settlement.request.requirements.asset, toHex(facts.asset));
  assert.equal(settlement.request.requirements.payTo, toHex(facts.payTo));
  assert.equal(observed.restarted, true);
  assert.equal(observed.payments.length, 1);
  const payment = observed.payments[0];
  assert.deepEqual(payment.storedAfter, payment.storedBefore);
  assert.ok(Object.keys(payment.storedBefore).some((path) => path.startsWith("orders/")));
  assert.ok(Object.keys(payment.storedBefore).some((path) => path.startsWith("fulfillments/")));
  assert.equal(payment.first.state, "paid");
  assert.equal(payment.replay.state, "paid");
  assert.equal(payment.first.order.state, "paid-verified");
  assert.equal(payment.first.order.tenant, "conformance");
  assert.equal(payment.first.order.principal, "merchant-conformance");
  assert.equal(payment.replay.order.tenant, payment.first.order.tenant);
  assert.equal(payment.replay.order.principal, payment.first.order.principal);
  assert.equal(payment.replay.order.receiptDigest, payment.receiptDigest.slice(2));
  assert.equal(settlement.request.payload.payload.signedReceipt, payment.signedReceipt);
  assert.deepEqual(settlement.response, { state: "settled", transaction: payment.txHash });
  const paidReceiptCalls = observed.node.filter((call) => call.request.method === "pax_sequencerReceipt"
    && call.request.params[0] === payment.txHash);
  assert.ok(paidReceiptCalls.length > 0);
  for (const call of paidReceiptCalls) assert.equal(call.response.body.result.receipt, payment.signedReceipt);
  assert.throws(() => new PaymentRpc(`${new URL(observed.nodeUrl).origin}/rpc`), /invalid-rpc-endpoint/u);
  assert.throws(() => new HostedGatewayRpc(`${new URL(observed.hostedGatewayUrl).origin}/rpc`), /invalid-rpc-endpoint/u);
  assert.throws(() => new PaymentRpc(observed.nodeUrl).call("px_resolveAccount", [SERVICE_PAYEE_ACCOUNT]), /invalid-node-method/u);
  assert.throws(() => new HostedGatewayRpc(observed.hostedGatewayUrl).call("pax_sequencerReceipt", [receipt.txHash]), /invalid-gateway-method/u);
}

async function conformance_daemon_fixture_tracks_and_recovers_real_serializer_output() {
  const transcript = JSON.parse(readFileSync(new URL("../fixtures/daemon-submission.json", import.meta.url), "utf8"));
  const dispatch = readFileSync(new URL("services/agent/crates/layerx-agentd/src/agent_rpc_dispatch.rs", repository), "utf8");
  const recorded = dispatch.match(/const MIDDLEWARE_SUBMISSION_TRANSCRIPT: &str = r#"([\s\S]*?)"#;/u);
  assert.ok(recorded, "the Rust dispatch producer must retain its shared serializer transcript");
  assert.deepEqual(transcript, JSON.parse(recorded[1]), "peer messages must equal the producer's exact committed output");
  const suite = new Suite();
  const observed = await runAgentServiceScenarios(suite);
  assert.equal(suite.results().length, 6, "verified spend, approval, rate limit, unknown outcome, recovery and terminal refusal must execute");
  assert.deepEqual(suite.results().filter((result) => !result.ok), []);
  assert.deepEqual(observed.errors, []);
  const submit = observed.calls.filter((call) => call.request.operation === "submit" && call.response);
  assert.equal(submit.length, 1);
  assert.deepEqual(submit[0].response.value, transcript.submit);
  assert.equal(submit[0].request.request.preparation_ref, transcript.submit.preparation_ref);
  assert.equal(submit[0].response.request_id, submit[0].request.request_id);
  assert.deepEqual(submit[0].response.verification_status, { state: "achieved", rung: "submitted" });
  const tracks = observed.calls.filter((call) => call.request.operation === "track");
  assert.equal(tracks.length, 3, "unresolved, committed and failed observations must reach the real consumer");
  assert.deepEqual(tracks[0].response.value, transcript.pending);
  assert.deepEqual(tracks[1].response.value, transcript.committed);
  for (const call of tracks) {
    assert.deepEqual(call.request.request, { submission_ref: transcript.submit.tx_hash });
    assert.equal(call.request.idempotency_key, null);
    assert.equal(call.response.request_id, call.request.request_id);
    assert.equal(call.response.value.activity_id, transcript.submit.tx_hash);
    assert.equal(call.response.value.submission.tx_hash, transcript.submit.tx_hash);
    assert.equal(call.response.value.submission.submission_ref, transcript.submit.tx_hash);
    assert.deepEqual(call.response.verification_status, { state: "achieved", rung: call.response.value.submission.rung });
  }
  assert.equal(observed.restarted, true);
  assert.equal(observed.results.length, 1);
  const recovery = observed.results[0];
  assert.equal(recovery.lost.kind, "unknown");
  assert.equal(recovery.pending.kind, "unknown");
  assert.equal(recovery.pending.submission.state, "Unknown");
  assert.equal(recovery.pending.submission.rung, "submitted");
  assert.equal(recovery.pending.reservation.state, "reserved");
  assert.equal(recovery.resumed.kind, "verified");
  assert.equal(recovery.resumed.submission.state, "Committed");
  assert.equal(recovery.resumed.submission.rung, "committed");
  assert.deepEqual(recovery.resumed.submission.observation, transcript.committed);
  assert.equal(recovery.resumed.submission.verification_level, "Committed");
  assert.equal(recovery.resumed.submission.receiptEvidence.verificationLevel, "receipted");
  assert.equal(recovery.resumed.submission.receiptEvidence.signedReceipt, `0x${transcript.submit.signed_receipt}`);
  assert.equal(recovery.resumed.submission.receiptEvidence.rawTransaction, `0x${transcript.raw_transaction}`);
  assert.equal(recovery.resumed.verification.txHash, `0x${transcript.submit.tx_hash}`);
  assert.equal(recovery.resumed.verification.rung, "receipted", "observation state must not replace a cryptographic commitment proof");
  assert.equal(recovery.resumed.reservation.state, "committed");
  assert.deepEqual(recovery.replay.reservation, recovery.resumed.reservation);
  const key = Object.keys(recovery.before.records).find((key) => recovery.before.records[key].reservationId === recovery.pending.reservation.reservationId);
  assert.ok(key);
  const before = recovery.before.records[key];
  const after = recovery.after.records[key];
  assert.equal(before.state, "reserved");
  assert.equal(after.state, "committed");
  for (const field of ["reservationId", "requestDigest", "amount", "asset"]) assert.equal(after[field], before[field]);
  assert.equal(after.receiptDigest, Buffer.from(recovery.resumed.verification.receiptDigest).toString("hex"));
  const commits = observed.services.filter((call) => call.path === "/budget" && call.request.action === "commit"
    && call.request.transition.reservation_id === after.reservationId);
  assert.equal(commits.length, 1, "reopening and replay must not commit the reservation twice");
  assert.equal(observed.calls.filter((call) => call.disconnected).length, 2, "uncertain submissions must never be retried during recovery");
  assert.ok(observed.services.some((call) => call.path === "/receipt" && call.request.receipt_ref === after.receiptDigest));
}

test(conformance_node_router_fixture_uses_root_and_separate_hosted_identity);
test(conformance_daemon_fixture_tracks_and_recovers_real_serializer_output);
