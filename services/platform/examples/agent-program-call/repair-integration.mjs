import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, open, readFile, rm, stat, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { decodeRawEip1559, keccak256, recoverEip1559Sender, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { createSubmissionRelay, readSubmissionRecord, submissionResult, submitProgramCall } from "./submission.mjs";
import { verifyCalledReceipt } from "./verify-receipt.mjs";

const vectors = JSON.parse(await readFile(new URL(
  "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
export const fixture = vectors.cases.find((entry) => entry.id === "V1-layerxprograms-call");
assert.ok(fixture, "missing producer-generated program call fixture");
const raw = Buffer.from(fixture.rawTransaction.slice(2), "hex");
const decoded = decodeRawEip1559(raw);
export const programId = Buffer.from(decoded.data.slice(4, 36)).toString("hex");
export const expectedHash = `0x${Buffer.from(keccak256(raw)).toString("hex")}`;
assert.equal(expectedHash, fixture.receiptJson.txHash);
assert.equal(recoverEip1559Sender(raw).toLowerCase(), fixture.receiptJson.from);
assert.equal(decoded.chainId, 125n);
assert.equal(`0x${Buffer.from(decoded.to).toString("hex")}`, fixture.receiptJson.to);

export async function send(url, method = "eth_sendRawTransaction", params = [fixture.rawTransaction]) {
  const response = await fetch(url, {
    method: "POST", headers: { "content-type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", id: 1, method, params }),
    signal: AbortSignal.timeout(10000),
  });
  return { status: response.status, body: await response.json() };
}

export function assertUnknown(result, hash = expectedHash) {
  assert.equal(result.status, "unknown-fate");
  assert.equal(Object.hasOwn(result, "submitted"), false);
  assert.match(result.action, /do not resubmit/u);
  if (hash !== null) assert.equal(result.tx_hash, hash);
  else assert.equal(Object.hasOwn(result, "tx_hash"), false);
}

export async function withJournal(run) {
  const directory = await mkdtemp(join(tmpdir(), "call-fate-"));
  try {
    return await run(join(directory, "submission.json"), directory);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

export async function networkCase(mode) {
  return withJournal(async (path, directory) => {
    const store = join(directory, "accepted.json");
    await writeFile(store, "[]");
    let relay;
    let server;
    let port = 0;
    let rpcUrl;
    let calls = 0;
    let receiptsAvailable = false;
    let accepted;
    const acceptance = new Promise((accept) => { accepted = accept; });
    const serverErrors = [];
    const methods = [];
    async function listen() {
      server = createServer(async (request, response) => {
        try {
          let body = "";
          for await (const chunk of request) body += chunk;
          const message = JSON.parse(body);
          methods.push(message.method);
          response.setHeader("content-type", "application/json");
          if (message.method === "eth_chainId") {
            response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result: "0x7d" }));
            return;
          }
          if (message.method === "eth_getRawTransactionByHash") {
            assert.deepEqual(message.params, [expectedHash]);
            const stored = JSON.parse(await readFile(store, "utf8"));
            response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id, result: stored[0] ?? null }));
            return;
          }
          if (message.method === "pax_sequencerReceipt") {
            assert.deepEqual(message.params, [expectedHash]);
            response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id,
              result: receiptsAvailable ? fixture.receiptJson : { txHash: expectedHash, rung: "submitted" } }));
            return;
          }
          assert.equal(message.method, "eth_sendRawTransaction");
          assert.deepEqual(message.params, [fixture.rawTransaction]);
          calls += 1;
          // The real durable journal must precede the receiver's first observation.
          const record = await readSubmissionRecord(path, rpcUrl);
          assert.equal(record.state, "attempted");
          assert.equal(record.tx_hash, expectedHash);
          assert.equal((await stat(path)).mode & 0o777, 0o600);
          const received = Buffer.from(message.params[0].slice(2), "hex");
          assert.deepEqual(decodeRawEip1559(received), decoded);
          assert.equal(recoverEip1559Sender(received).toLowerCase(), fixture.receiptJson.from);
          const stored = JSON.parse(await readFile(store, "utf8"));
          stored.push(message.params[0]);
          const file = await open(store, "w");
          try { await file.writeFile(JSON.stringify(stored)); await file.sync(); } finally { await file.close(); }
          accepted();
          if (mode === "crash_ack") return;
          if (mode === "lost_ack") { response.destroy(); return; }
          if (mode === "malformed_ack") { response.end("{"); return; }
          if (mode === "rpc_rejection") {
            response.end(JSON.stringify({ jsonrpc: "2.0", id: message.id,
              error: { code: -32000, message: "already known" } }));
          } else {
            if (mode === "http_failure") response.statusCode = 503;
            response.end(JSON.stringify({ jsonrpc: "2.0", id: mode === "wrong_id" ? 2 : message.id,
              result: mode === "wrong_hash" ? `0x${"ff".repeat(32)}` : expectedHash }));
          }
        } catch (error) {
          serverErrors.push(error);
          accepted();
          response.destroy();
        }
      });
      server.listen(port, "127.0.0.1");
      await once(server, "listening");
      port = server.address().port;
    }
    async function closeServer() {
      server.closeAllConnections();
      await new Promise((accept, reject) => server.close((error) => error ? reject(error) : accept()));
    }
    try {
      await listen();
      rpcUrl = `http://127.0.0.1:${port}/rpc`;
      relay = await createSubmissionRelay(path, rpcUrl, programId, "absent");
      const preparation = await send(relay.rpcUrl, "eth_chainId", []);
      assert.equal(preparation.status, 200);
      assert.equal(preparation.body.result, "0x7d");
      assert.equal((await readSubmissionRecord(path, rpcUrl)).state, "preparing");
      const response = send(relay.rpcUrl).then((value) => value.status, () => "disconnected");
      let timer;
      try {
        await Promise.race([acceptance, new Promise((_, reject) => {
          timer = setTimeout(() => reject(new Error("acceptance timeout")), 10000);
        })]);
      } finally { clearTimeout(timer); }
      assert.deepEqual(serverErrors, []);
      if (mode === "crash_ack") {
        // Stop the component with its acknowledgement still outstanding; reopen only its journal.
        await relay.stop();
        assert.equal(await response, "disconnected");
      } else {
        assert.equal(await response, mode === "acknowledged" ? 200 : 502);
        const duplicate = await send(relay.rpcUrl);
        assert.equal(duplicate.status, 502);
        await relay.stop();
      }
      await closeServer();
      await listen();
      assert.deepEqual(JSON.parse(await readFile(store, "utf8")), [fixture.rawTransaction]);
      const record = await readSubmissionRecord(path, rpcUrl);
      assert.equal(record.state, mode === "acknowledged" ? "acknowledged" : "attempted");
      const recovered = submissionResult(record);
      assertUnknown(recovered);
      // Real submit entry point must return the journal without starting the CLI or using a key.
      const resumed = await submitProgramCall(path, rpcUrl, programId, "absent", join(directory, "missing-key.json"));
      assert.deepEqual(submissionResult(resumed), recovered);
      await assert.rejects(createSubmissionRelay(path, rpcUrl, programId, "absent"), { code: "EEXIST" });
      await assert.rejects(readSubmissionRecord(path, `http://localhost:${port}/rpc`), /invalid_submission_record/u);
      const key = fixture.pinnedKey;
      const keys = [{
        keyId: Buffer.from(key.keyId.slice(2), "hex"), publicKey: Buffer.from(key.publicKey.slice(2), "hex"),
        first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight),
      }];
      await assert.rejects(verifyCalledReceipt(rpcUrl, recovered.tx_hash, keys),
        { state: "pending", message: "sequencer_receipt_submitted" });
      assertUnknown(submissionResult(await readSubmissionRecord(path, rpcUrl)));
      receiptsAvailable = true;
      const proof = await verifyCalledReceipt(rpcUrl, recovered.tx_hash, keys);
      assert.equal(proof.verificationLevel, "receipted");
      assert.equal(proof.rawTransaction, fixture.rawTransaction);
      assert.equal(proof.signedReceipt, fixture.signedReceipt);
      const signedReceipt = Buffer.from(proof.signedReceipt.slice(2), "hex");
      // The proof fingerprints the whole signed blob; the producer records its signing digest.
      assert.equal(proof.receiptDigest, `0x${Buffer.from(keccak256(signedReceipt)).toString("hex")}`);
      const verified = await verifySequencerReceipt(signedReceipt, raw, keys);
      assert.equal(`0x${Buffer.from(verified.receiptDigest).toString("hex")}`, fixture.receiptDigest);
      // Reconciliation never releases the submission claim or authorizes another economic call.
      assert.deepEqual(await submitProgramCall(path, rpcUrl, programId, "absent", "missing-key"), record);
      assert.equal(calls, 1);
      assert.equal(methods.filter((method) => method === "eth_sendRawTransaction").length, 1);
      assert.deepEqual(serverErrors, []);
      assert.deepEqual(JSON.parse(await readFile(store, "utf8")), [fixture.rawTransaction]);
    } finally {
      if (relay) await relay.stop();
      if (server?.listening) await closeServer();
    }
  });
}
