import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import { test } from "node:test";
import { assertUnknown, expectedHash, fixture, networkCase, programId, send, withJournal } from "../repair-integration.mjs";
import { createSubmissionRelay, readSubmissionRecord, submissionResult, submitProgramCall } from "../submission.mjs";

async function call_submission_fate_recovers_after_acceptance_and_acknowledgement_loss() {
  await networkCase("lost_ack");
}

async function call_submission_fate_recovers_after_relay_stop_before_acknowledgement() {
  await networkCase("crash_ack");
}

async function call_submission_fate_acknowledgement_still_requires_receipt_reconciliation() {
  await networkCase("acknowledged");
}

async function call_submission_fate_rpc_errors_and_invalid_acknowledgements_remain_unknown() {
  for (const mode of ["wrong_hash", "rpc_rejection", "http_failure", "malformed_ack", "wrong_id"]) {
    await networkCase(mode);
  }
}

async function call_submission_fate_proves_pre_send_refusal() {
  await withJournal(async (path) => {
    const rpcUrl = "http://127.0.0.1:1/rpc";
    const relay = await createSubmissionRelay(path, rpcUrl, programId, "absent");
    try {
      // Invalid transactions and methods never reach the upstream transport.
      const invalid = await send(relay.rpcUrl, "eth_sendRawTransaction", ["0x0200"]);
      assert.equal(invalid.status, 502);
      const forbidden = await send(relay.rpcUrl, "eth_sendTransaction", [{}]);
      assert.equal(forbidden.status, 502);
      const record = await relay.finish();
      assert.equal(record.state, "refused");
      const result = submissionResult(record);
      assert.equal(result.status, "local-or-deterministic-refusal");
      assert.equal(result.submitted, false);
      assert.equal(Object.hasOwn(result, "tx_hash"), false);
      assert.deepEqual(await readSubmissionRecord(path, rpcUrl), record);
      assert.deepEqual(await submitProgramCall(path, rpcUrl, programId, "absent", "missing-key"), record);
    } finally { await relay.stop(); }
  });
}

async function call_submission_fate_corrupt_journal_cannot_authorize_resubmission() {
  await withJournal(async (path) => {
    const rpcUrl = "http://127.0.0.1:1/rpc";
    await writeFile(path, "{");
    await assert.rejects(readSubmissionRecord(path, rpcUrl), SyntaxError);
    await assert.rejects(submitProgramCall(path, rpcUrl, programId, "absent", "missing-key"), SyntaxError);
    await assert.rejects(createSubmissionRelay(path, rpcUrl, programId, "absent"), { code: "EEXIST" });
    assert.equal(await readFile(path, "utf8"), "{");
    for (const state of ["attempted", "acknowledged"]) {
      await writeFile(path, JSON.stringify({ version: 1, program_id: programId, interface_digest: "absent",
        rpc_url: rpcUrl, state, tx_hash: null }));
      await assert.rejects(readSubmissionRecord(path, rpcUrl), /invalid_submission_record/u);
      await assert.rejects(submitProgramCall(path, rpcUrl, programId, "absent", "missing-key"), /invalid_submission_record/u);
    }
  });
}

async function call_submission_fate_interrupted_preparation_cannot_authorize_resubmission() {
  await withJournal(async (path) => {
    const rpcUrl = "http://127.0.0.1:1/rpc";
    const relay = await createSubmissionRelay(path, rpcUrl, programId, "absent");
    await relay.stop();
    const record = await readSubmissionRecord(path, rpcUrl);
    assert.equal(record.state, "preparing");
    assertUnknown(submissionResult(record), null);
    assert.deepEqual(await submitProgramCall(path, rpcUrl, programId, "absent", "missing-key"), record);
    await assert.rejects(createSubmissionRelay(path, rpcUrl, programId, "absent"), { code: "EEXIST" });
    assert.deepEqual(await readSubmissionRecord(path, rpcUrl), record);
  });
}

async function call_submission_fate_concurrent_sends_forward_only_once() {
  // All contenders share the same real relay and exclusive durable submission claim.
  const { createServer } = await import("node:http");
  const { once } = await import("node:events");
  await withJournal(async (path) => {
    let calls = 0;
    const server = createServer(async (request, response) => {
      for await (const _ of request) { /* drain the real request body */ }
      calls += 1;
      response.end(JSON.stringify({ jsonrpc: "2.0", id: 1, result: expectedHash }));
    });
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    const rpcUrl = `http://127.0.0.1:${server.address().port}/rpc`;
    let relay;
    try {
      relay = await createSubmissionRelay(path, rpcUrl, programId, "absent");
      const results = await Promise.all([send(relay.rpcUrl), send(relay.rpcUrl)]);
      assert.deepEqual(results.map((result) => result.status).sort(), [200, 502]);
      assert.equal(calls, 1);
      const record = await relay.finish();
      assert.equal(record.tx_hash, fixture.receiptJson.txHash);
      assert.equal(record.state, "acknowledged");
      assertUnknown(submissionResult(await readSubmissionRecord(path, rpcUrl)));
    } finally {
      if (relay) await relay.stop();
      server.closeAllConnections();
      await new Promise((accept, reject) => server.close((error) => error ? reject(error) : accept()));
    }
  });
}

test(call_submission_fate_recovers_after_acceptance_and_acknowledgement_loss);
test(call_submission_fate_recovers_after_relay_stop_before_acknowledgement);
test(call_submission_fate_acknowledgement_still_requires_receipt_reconciliation);
test(call_submission_fate_rpc_errors_and_invalid_acknowledgements_remain_unknown);
test(call_submission_fate_proves_pre_send_refusal);
test(call_submission_fate_corrupt_journal_cannot_authorize_resubmission);
test(call_submission_fate_interrupted_preparation_cannot_authorize_resubmission);
test(call_submission_fate_concurrent_sends_forward_only_once);
