import "./production-sources.mjs";
import assert from "node:assert/strict";
import { once } from "node:events";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { Worker } from "node:worker_threads";
import test from "node:test";
import { BudgetStore, daemonFixture, loopback, temporaryDirectory } from "./repair-support.mjs";

export const REPAIR_TASK_RECORD = Object.freeze({
  task: "22.33.1476",
  base: "9e535b4932ba8111a729a5cef10e0ca2370ca435",
  findings: {
    "agent-draw-fixture": ["agentGrantSettlesDrawWithPaymentDrawnLog", "agentGrantRefusesDrawAboveReservation", "agentDrawProducerRefusalSurvivesReopen"],
    "daemon-decoder-fixture": ["decodeDaemonSubmissionAcceptsOnlyRungLevels", "daemonSerializerPendingTrackingSurvivesReopen", "realAgentSpendExampleSubmitsProducerBytesAndReopens"],
    "agent-export-daemon-fixture": ["middlewareAgentExportsReachRealConsumers", "middlewareAgentReceiptBindingSurvivesReopen", "realAgentSpendExampleSubmitsProducerBytesAndReopens"],
  },
  baseline: "Not run: the explicit implementation-first order and hf-done-only verification rule prohibit an unchanged-tree baseline; no pre-change failing test is claimed.",
  evidence: "The hf-build/hf-done private ledger supplies the exact verified candidate revision, command, exit code and log path; this record claims no unrun gate.",
  provenance: "agent_rpc_dispatch.rs::MIDDLEWARE_SUBMISSION_TRANSCRIPT is checked against the real Rust serializers by middleware_submit_transcript_is_produced_by_dispatch and middleware_observation_transcripts_are_produced_by_dispatch. Legacy signed grant cases prove codec binding only; they do not prove a live ledger debit.",
  wiring: [
    "modules/layerxledger/keeper/{grants.go,draw.go,views.go} and precompiles/layerxledger/proof_layerxledger_0x101a_test.go — ledger domain: current grant issuance, lookup and draw refuse LXP_NO_PAYER_GRANT; provide supported grant execution and a producer-recorded PaymentDrawn success/refusal/reopen transcript before successful live draw closure can be claimed.",
    "sdk/integrations/agents/src/tools.ts — agent integration domain: resolve canonical observation receipts with retained raw transaction bytes and configured pins before reporting an authenticated receipt digest for Executed or Committed tracking.",
    "daemon/committed-fixture and sdk/platform/conformance/fixtures — node/conformance domain: provide consensus evidence for the daemon EIP-1559 receipt; the committed label alone is deliberately insufficient to commit the middleware budget.",
  ],
  gotchas: [
    "The bounded Node gate installs dependencies but does not build agent dist; production sources are loaded with Node's TypeScript transform without modifying package resolution outside this test process.",
    "A Committed daemon state and lowercase committed rung decode correctly but cannot replace independent consensus evidence.",
  ],
});

await import("./agent-grant.test.mjs");
await import("./daemon-evm.test.mjs");
await import("./proof_middleware_agent_ts_exports.test.mjs");

function budgetWire(reservation) {
  return { state: reservation.state, reservation_id: reservation.reservationId,
    request_digest: reservation.requestDigest, amount: reservation.amount, asset: reservation.asset,
    ...(reservation.receiptDigest === undefined ? {} : { receipt_digest: reservation.receiptDigest }) };
}

async function runRealExample(environment) {
  const worker = new Worker(new URL("./example-runner.mjs", import.meta.url), {
    env: environment, argv: ["spend"], stdout: true, stderr: true,
  });
  const stdout = [];
  const stderr = [];
  worker.stdout.on("data", (chunk) => stdout.push(chunk));
  worker.stderr.on("data", (chunk) => stderr.push(chunk));
  const [code] = await once(worker, "exit");
  assert.equal(code, 0, Buffer.concat(stderr).toString("utf8"));
  return JSON.parse(Buffer.concat(stdout).toString("utf8"));
}

async function realAgentSpendExampleSubmitsProducerBytesAndReopens(t) {
  const { transcript, input, prepared, receiptKeys, signer } = daemonFixture();
  const directory = temporaryDirectory(t);
  const storePath = join(directory, "example-budget.sqlite");
  let budgets = new BudgetStore(storePath);
  t.after(() => budgets.close());
  const calls = [];
  const signed = await signer.sign({ signing_hash: prepared.signing_hash });
  const endpoint = await loopback(t, async (body, request) => {
    calls.push({ path: request.url, body });
    if (request.url === "/budget") {
      assert.equal(request.headers.authorization, "Bearer proof-local-services");
      if (body.action === "reserve") {
        assert.equal(body.request.tenant, input.tenant);
        assert.equal(body.request.idempotency_key, input.preparation.idempotency_key);
        assert.equal(body.request.amount, input.amount);
        assert.equal(body.request.asset, input.asset);
        assert.match(body.request.request_digest, /^[0-9a-f]{64}$/u);
        const result = await budgets.reserve({ tenant: body.request.tenant,
          idempotencyKey: body.request.idempotency_key, requestDigest: body.request.request_digest,
          amount: body.request.amount, asset: body.request.asset });
        assert.equal(result.kind, "reserved");
        return { kind: result.kind, reservation: budgetWire(result.reservation) };
      }
      assert.equal(body.action, "commit");
      const transition = body.transition;
      assert.equal(transition.receipt_digest, transcript.committed.submission.receipt_ref);
      return budgetWire(await budgets.commit({ reservationId: transition.reservation_id,
        requestDigest: transition.request_digest, amount: transition.amount, asset: transition.asset,
        receiptDigest: transition.receipt_digest }));
    }
    if (request.url === "/sign") {
      assert.equal(request.headers.authorization, "Bearer proof-local-services");
      assert.equal(body.preparation_ref, prepared.preparation_ref);
      assert.equal(body.signing_hash, prepared.signing_hash);
      assert.equal(body.disclosure.canonical_digest, prepared.signing_hash);
      return { signature: signed };
    }
    if (request.url === "/receipt") {
      assert.equal(body.receipt_ref, transcript.committed.submission.receipt_ref);
      const { decodeDaemonSubmitResult, resolveDaemonReceipt } = await import("@sidiora/layerx-agent-middleware");
      return (await resolveDaemonReceipt(decodeDaemonSubmitResult(transcript.submit,
        transcript.submit.tx_hash, prepared.preparation_ref), Buffer.from(transcript.raw_transaction, "hex"), receiptKeys)).portableReceipt;
    }
    assert.equal(request.url, "/v1/agent/rpc");
    assert.equal(request.headers.authorization, `LayerX-Key example-proof:lxp_live_${"33".repeat(32)}`);
    let value;
    if (body.operation === "prepare") {
      assert.deepEqual(body.request, input.preparation);
      value = prepared;
    } else {
      assert.equal(body.operation, "submit");
      assert.deepEqual(body.request, { preparation_ref: prepared.preparation_ref, signature: signed,
        signer_public_key: input.signerPublicKey, approval_release_ref: null });
      value = transcript.submit;
      assert.deepEqual(Object.keys(value).sort(), ["preparation_ref", "signed_receipt", "state", "tx_hash"]);
    }
    return { request_id: body.request_id, value, verification_status: { state: "achieved", rung: "receipted" } };
  });
  const environment = {
    LAYERX_AGENT_RPC_URL: endpoint, LAYERX_AGENT_SURFACE: "gateway", LAYERX_TENANT: input.tenant,
    LAYERX_AGENT_SERVICES_MODULE: fileURLToPath(new URL("./example-services.mjs", import.meta.url)),
    LAYERX_SESSION_ID: "11".repeat(32), LAYERX_SESSION_TOKEN: "22".repeat(32), LAYERX_SESSION_GENERATION: "0",
    LAYERX_GATEWAY_KEY_ID: "example-proof", LAYERX_GATEWAY_KEY: `lxp_live_${"33".repeat(32)}`,
    LAYERX_PROTOCOL_VERSION: "2", LAYERX_SPEND_REQUEST_JSON: JSON.stringify(input),
    LAYERX_RECEIPT_KEYS_JSON: JSON.stringify(receiptKeys.map((key) => ({
      keyId: Buffer.from(key.keyId).toString("hex"), publicKey: Buffer.from(key.publicKey).toString("hex"),
      first_height: key.first_height.toString(), last_height: key.last_height.toString(),
    }))),
  };
  const first = await runRealExample(environment);
  assert.deepEqual(first, { kind: "verified", transactionHash: `0x${transcript.submit.tx_hash}`,
    receiptDigest: transcript.committed.submission.receipt_ref, rung: "receipted", reservationState: "committed" });
  assert.equal(budgets.available, 75000n);
  assert.equal(budgets.transitions.length, 1);
  budgets.close();
  budgets = new BudgetStore(storePath);
  assert.deepEqual(await runRealExample(environment), first);
  assert.equal(budgets.available, 75000n);
  assert.equal(budgets.transitions.length, 1);
  assert.equal(calls.filter((call) => call.body.operation === "prepare").length, 1);
  assert.equal(calls.filter((call) => call.body.operation === "submit").length, 1);
  assert.equal(calls.filter((call) => call.path === "/sign").length, 1);
  assert.equal(calls.filter((call) => call.path === "/receipt").length, 1);
  assert.equal(calls.filter((call) => call.body.action === "reserve").length, 2);
  assert.equal(calls.filter((call) => call.body.action === "commit").length, 1);
}

test(realAgentSpendExampleSubmitsProducerBytesAndReopens);
