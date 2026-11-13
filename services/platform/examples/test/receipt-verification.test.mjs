import "./support/source-loader.mjs";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { once } from "node:events";
import { mkdtemp, readFile, readdir, rm } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
const { verifyCalledReceipt } = await import("../agent-program-call/verify-receipt.mjs");
const { submissionResult } = await import("../agent-program-call/submission.mjs");
const { keccak256 } = await import("@sidiora/layerx-sdk");
const {
  ReceiptPayloadAuthority, SellerMiddleware, decodePaymentRequiredHeader, decodeSettlementHeader,
  encodePaymentPayloadHeader, importPortableNodeReceipt, verifyPaymentReceipt, verifyRpcPayment,
} = await import("@sidiora/layerx-seller-middleware");
const { SequencerReceiptClient } = await import("../support/runtime.mjs");
const { FileFulfillments } = await import("../support/merchant-app.mjs");
import { bytes, hex, nodeReceipt, portableReceipt, recordedNode } from "./support/node-receipts.mjs";

const execute = promisify(execFile);
await import("./runtime.test.mjs");
await import("./buyer-agent.test.mjs");
await import("./merchant-app.test.mjs");
await import("./merchant-config.test.mjs");
await import("../../middleware/merchant/src/settlement-webhook.test.mjs");

async function walkthroughVerifiesActualSequencerReceipt(t) {
  const vector = nodeReceipt("V1-layerxprograms-call");
  const node = await recordedNode(t, vector);
  const proof = await verifyCalledReceipt(node.rpcUrl, vector.txHash, [vector.key]);
  assert.deepEqual(proof, portableReceipt(vector));
  assert.deepEqual(node.requests.map(({ message }) => message.method), [
    "eth_getRawTransactionByHash", "pax_sequencerReceipt",
  ]);
  assert.equal(proof.receiptJson.status, "0x1");
  assert.equal(proof.receiptJson.to, "0x000000000000000000000000000000000000101d");
  assert.equal(proof.receiptJson.logs.length, 1);
  const output = await execute(process.execPath, [
    "--import", fileURLToPath(new URL("./support/source-loader.mjs", import.meta.url)),
    fileURLToPath(new URL("../agent-program-call/verify-receipt.mjs", import.meta.url)), vector.txHash,
  ], {
    timeout: 10000, maxBuffer: 1024 * 1024,
    env: { ...process.env, LAYERX_RPC_URL: node.rpcUrl, LAYERX_RECEIPT_KEYS_JSON: JSON.stringify([vector.pinnedKey]) },
  });
  assert.equal(output.stdout, "receipted\n");
  assert.equal(node.requests.length, 4);
}

async function walkthroughKeepsWrongPinsAndPendingReceiptsUnknown(t) {
  const vector = nodeReceipt("V1-layerxprograms-call");
  const node = await recordedNode(t, vector);
  const record = {
    program_id: vector.receiptJson.logs[0].topics[1].slice(2), interface_digest: "absent",
    state: "acknowledged", tx_hash: vector.txHash,
  };
  for (const key of [
    { ...vector.key, keyId: new Uint8Array(32).fill(1) },
    { ...vector.key, publicKey: new Uint8Array(32).fill(2) },
    { ...vector.key, first_height: 1003n },
    { ...vector.key, last_height: 1001n },
  ]) {
    await assert.rejects(verifyCalledReceipt(node.rpcUrl, vector.txHash, [key]), {
      name: "LayerXApplicationStateError", state: "refused", message: "sequencer_receipt_verification_failed",
    });
    const unknown = submissionResult(record);
    assert.equal(unknown.status, "unknown-fate");
    assert.equal(unknown.rung, "submitted");
    assert.equal(unknown.tx_hash, vector.txHash);
    assert.equal(Object.hasOwn(unknown, "outcome"), false);
    assert.equal(unknown.action, "Query receipt get with this transaction hash; do not resubmit the call");
  }
  node.result = { txHash: vector.txHash, rung: "submitted" };
  await assert.rejects(verifyCalledReceipt(node.rpcUrl, vector.txHash, [vector.key]), {
    name: "LayerXApplicationStateError", state: "pending", message: "sequencer_receipt_submitted",
  });
  assert.equal(submissionResult(record).status, "unknown-fate");
  assert.equal(node.requests.some(({ message }) => message.method === "eth_sendRawTransaction"), false);
}

test(walkthroughVerifiesActualSequencerReceipt);
test(walkthroughKeepsWrongPinsAndPendingReceiptsUnknown);

async function sellerVerifiesProducerReceiptDigest(t) {
  const vector = nodeReceipt("V1-layerxledger-pay-exact");
  const node = await recordedNode(t, vector);
  const proof = await new SequencerReceiptClient(node.rpcUrl, [vector.key]).resolve(vector.raw);
  const log = vector.receiptJson.logs[0];
  const requirements = {
    scheme: "exact", network: "eip155:125", amount: BigInt(`0x${log.data.slice(66)}`).toString(),
    asset: log.data.slice(0, 66), payTo: log.topics[3], maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer: log.topics[2].slice(2), purposeHash: log.topics[1].slice(2) } },
  };
  const verified = await verifyPaymentReceipt(proof, requirements, [vector.key]);
  assert.equal(verified.txHash, vector.txHash);
  assert.equal(hex(verified.receiptDigest), vector.receiptDigest);
  assert.equal(verified.rung, "receipted");
  assert.equal(verified.payment.amount.toString(), requirements.amount);
  const sender = vector.receiptJson.from.slice(2).padStart(64, "0");
  const rpcProof = await verifyRpcPayment(proof.receiptJson, proof.rawTransaction,
    sender, requirements, [vector.key]);
  assert.equal(rpcProof.kind, "verified");
  assert.equal(rpcProof.receiptDigest, proof.receiptDigest);
  assert.deepEqual(rpcProof.receiptJson, vector.receiptJson);
  assert.equal(rpcProof.verification.txHash, vector.txHash);
  assert.equal(hex(rpcProof.verification.receiptDigest), vector.receiptDigest);
  const refuses = { name: "MiddlewareError", code: "verification-failure" };
  assert.notEqual(sender, log.topics[2].slice(2));
  await assert.rejects(verifyRpcPayment(proof.receiptJson, proof.rawTransaction,
    log.topics[2].slice(2), requirements, [vector.key]), refuses);
  const legacyDigest = hex(keccak256(bytes(proof.signedReceipt)));
  assert.notEqual(legacyDigest, proof.receiptDigest);
  for (const receiptDigest of [legacyDigest, `0x${"00".repeat(32)}`]) {
    await assert.rejects(verifyPaymentReceipt({ ...proof, receiptDigest }, requirements, [vector.key]), refuses);
  }
  await assert.rejects(verifyPaymentReceipt(proof, requirements,
    [{ ...vector.key, publicKey: new Uint8Array(32).fill(1) }]), refuses);
  await assert.rejects(verifyPaymentReceipt(proof, { ...requirements, amount: "1" }, [vector.key]), refuses);
  const corrupted = bytes(proof.signedReceipt);
  corrupted[corrupted.length - 1] ^= 1;
  await assert.rejects(verifyPaymentReceipt({ ...proof, signedReceipt: hex(corrupted) }, requirements, [vector.key]), refuses);
  assert.deepEqual(node.requests.map(({ message }) => message.method), ["pax_sequencerReceipt"]);
}

test(sellerVerifiesProducerReceiptDigest);

async function sellerRetainsPortableNodeReceiptAcrossRestart(t) {
  const vector = nodeReceipt("V1-layerxledger-pay-exact");
  const node = await recordedNode(t, vector);
  const proof = importPortableNodeReceipt(await new SequencerReceiptClient(node.rpcUrl, [vector.key]).resolve(vector.raw));
  assert.deepEqual(proof, portableReceipt(vector));
  const { receiptJson, ...missingJson } = proof;
  assert.throws(() => importPortableNodeReceipt(missingJson), { name: "MiddlewareError", code: "invalid-payment-payload" });
  assert.throws(() => importPortableNodeReceipt({ ...proof, extra: true }), { name: "MiddlewareError", code: "invalid-payment-payload" });
  const log = receiptJson.logs[0];
  const requirements = {
    scheme: "exact", network: "eip155:125", amount: BigInt(`0x${log.data.slice(66)}`).toString(),
    asset: log.data.slice(0, 66), payTo: log.topics[3], maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", payer: log.topics[2].slice(2), purposeHash: log.topics[1].slice(2) } },
  };
  for (const corrupt of [
    (json) => { json.txHash = `0x${"00".repeat(32)}`; },
    (json) => { json.receipt = `0x${"00".repeat(32)}`; },
    (json) => { json.signingKey = `0x${"00".repeat(32)}`; },
    (json) => { json.height = "0x1"; },
    (json) => { json.status = "0x0"; },
    (json) => { json.nonce = "0x0"; },
    (json) => { json.gasUsed = "0x0"; },
    (json) => { json.feeAmount = "0x0"; },
    (json) => { json.paymentDenom = "another-token"; },
    (json) => { json.feeDenom = "another-token"; },
    (json) => { json.returnData = "0x01"; },
    (json) => { json.logs[0].data = `0x${"00".repeat(64)}`; },
    (json) => { json.logs[0].topics[1] = `0x${"00".repeat(32)}`; },
    (json) => { json.logs[0].logIndex = "0x1"; },
    (json) => { json.unexpected = true; },
  ]) {
    const altered = structuredClone(proof);
    corrupt(altered.receiptJson);
    await assert.rejects(verifyPaymentReceipt(altered, requirements, [vector.key]), {
      name: "MiddlewareError", code: "verification-failure",
    });
  }

  const directory = await mkdtemp(join(tmpdir(), "portable-seller-"));
  let server;
  let releases = 0;
  const failures = [];
  async function stop() {
    if (server === undefined || !server.listening) return;
    server.closeAllConnections();
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
  t.after(async () => {
    await stop();
    await rm(directory, { recursive: true, force: true });
    assert.deepEqual(failures, []);
  });
  async function start() {
    const seller = new SellerMiddleware({
      paymentRequired: { x402Version: 2, resource: { url: "https://seller.example/paid" }, accepts: [requirements] },
      receiptKeys: [vector.key], authority: new ReceiptPayloadAuthority([vector.key]),
      fulfillments: new FileFulfillments(directory),
    });
    server = createServer(async (request, response) => {
      try {
        assert.equal(request.method, "GET");
        assert.equal(request.url, "/paid");
        const decision = await seller.handle("portable-buyer", request.headers["payment-signature"], async () => {
          releases += 1;
          return "receipt-backed release";
        });
        response.writeHead(decision.status, { "content-type": "application/json", ...decision.headers })
          .end(JSON.stringify(decision.kind === "released" ? decision.resource : decision.body));
      } catch (error) {
        failures.push(error.message);
        response.writeHead(500).end(JSON.stringify({ error: error.message }));
      }
    });
    server.listen(0, "127.0.0.1");
    await once(server, "listening");
    return `http://127.0.0.1:${server.address().port}/paid`;
  }
  let url = await start();
  const unpaid = await fetch(url);
  assert.equal(unpaid.status, 402);
  const offer = decodePaymentRequiredHeader(unpaid.headers.get("payment-required"));
  assert.deepEqual(offer.accepts, [requirements]);
  await unpaid.arrayBuffer();
  const payment = encodePaymentPayloadHeader({
    x402Version: 2, resource: offer.resource, accepted: offer.accepts[0], payload: proof,
  });
  const paid = await fetch(url, { headers: { "payment-signature": payment } });
  assert.equal(paid.status, 200, await paid.clone().text());
  assert.equal(await paid.json(), "receipt-backed release");
  const settlement = decodeSettlementHeader(paid.headers.get("payment-response"));
  assert.equal(settlement.transaction, vector.txHash);
  assert.deepEqual(settlement.extensions.layerx, proof);
  const files = (await readdir(directory)).filter((file) => file.endsWith(".json"));
  assert.equal(files.length, 1);
  const stored = JSON.parse(await readFile(join(directory, files[0]), "utf8"));
  for (const [key, value] of Object.entries(proof)) assert.deepEqual(stored[key], value);
  assert.equal(stored.txHash, vector.txHash);
  await stop();
  url = await start();
  const replay = await fetch(url, { headers: { "payment-signature": payment } });
  assert.equal(replay.status, 200, await replay.clone().text());
  assert.equal(await replay.json(), "receipt-backed release");
  assert.deepEqual(decodeSettlementHeader(replay.headers.get("payment-response")), settlement);
  assert.equal(releases, 1);
  assert.equal(node.requests.length, 1);
  const reopened = new SellerMiddleware({
    paymentRequired: offer, receiptKeys: [vector.key],
    authority: new ReceiptPayloadAuthority([vector.key]), fulfillments: new FileFulfillments(directory),
  });
  const reused = await reopened.handle("another-buyer", payment, async () => {
    assert.fail("a different principal cannot release a fulfilled transaction");
  });
  assert.equal(reused.kind, "refused");
  assert.equal(reused.status, 409);
  assert.equal(reused.settlement.errorReason, "receipt_reuse");
  assert.deepEqual(await readdir(directory), files);
  assert.equal(node.requests.length, 1);
}

test(sellerRetainsPortableNodeReceiptAcrossRestart);
