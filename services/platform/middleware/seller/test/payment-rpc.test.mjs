import assert from "node:assert/strict";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import { createServer } from "node:http";
import test from "node:test";
import { MiddlewareError, PaymentRpc, PaymentRpcError, rpcHex, verifyRpcPayment } from "../dist/index.js";

const vectors = JSON.parse(await readFile(new URL(
  "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
), "utf8"));
const payment = vectors.cases.find((entry) => entry.id === "V1-layerxledger-pay-exact");
assert.ok(payment);
const bytes = (hex) => Uint8Array.from(Buffer.from(hex.replace(/^0x/u, ""), "hex"));
const receiptKeys = [{
  keyId: bytes(payment.pinnedKey.keyId),
  publicKey: bytes(payment.pinnedKey.publicKey),
  first_height: BigInt(payment.pinnedKey.firstHeight),
  last_height: BigInt(payment.pinnedKey.lastHeight),
}];
const paymentLog = payment.receiptJson.logs[0];
const requirements = {
  scheme: "exact", network: "eip155:125", amount: BigInt(`0x${paymentLog.data.slice(66)}`).toString(),
  asset: paymentLog.data.slice(0, 66), payTo: paymentLog.topics[3], maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: "receipted", payer: paymentLog.topics[2].slice(2), purposeHash: paymentLog.topics[1].slice(2) } },
};

async function loopbackRpc(context, responses) {
  const requests = [];
  const remaining = [...responses];
  const server = createServer(async (request, response) => {
    try {
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push({ method: request.method, url: request.url, headers: request.headers, body });
      const answer = remaining.shift();
      if (answer === undefined) throw new Error("unexpected-rpc-request");
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify({ jsonrpc: "2.0", id: body.id, ...answer }));
    } catch (error) {
      response.writeHead(500);
      response.end(String(error));
    }
  });
  context.after(async () => {
    const closed = new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    server.closeAllConnections();
    await closed;
    assert.equal(remaining.length, 0, "every recorded response must be consumed");
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const rpc = new PaymentRpc(`http://127.0.0.1:${server.address().port}/rpc`, { "x-payment-client": "seller" });
  return { rpc, requests };
}

async function paymentRpcSendsRawTransaction(context) {
  const txHash = payment.receiptJson.txHash;
  const { rpc, requests } = await loopbackRpc(context, [
    { result: txHash },
    { result: txHash },
    { result: "0x1234" },
    { result: { txHash } },
    { error: { code: -32000, message: "transaction rejected", data: "invalid nonce" } },
  ]);

  assert.equal(await rpc.send(payment.rawTransaction), txHash);
  assert.equal(await rpc.send(payment.rawTransaction, "committed"), txHash);
  await assert.rejects(rpc.send(payment.rawTransaction), /invalid-rpc-transaction-hash/);
  await assert.rejects(rpc.send(payment.rawTransaction), /invalid-rpc-transaction-hash/);
  await assert.rejects(rpc.send(payment.rawTransaction), (error) =>
    error instanceof PaymentRpcError && error.code === -32000 && error.data === "invalid nonce");
  for (const raw of [payment.rawTransaction.slice(2), "0x02", `0x01${payment.rawTransaction.slice(4)}`]) {
    await assert.rejects(rpc.send(raw));
  }
  for (const [index, request] of requests.entries()) {
    assert.equal(request.method, "POST");
    assert.equal(request.url, "/rpc");
    assert.equal(request.headers["content-type"], "application/json");
    assert.equal(request.headers["x-payment-client"], "seller");
    assert.deepEqual(request.body, {
      jsonrpc: "2.0", id: index + 1, method: "eth_sendRawTransaction", params: [payment.rawTransaction],
    });
  }
  assert.equal(requests.length, 5, "invalid raw transactions must be refused before transport");
}

async function paymentRpcReadsSequencerReceipt(context) {
  const txHash = payment.receiptJson.txHash;
  const submitted = { rung: "submitted", txHash, nonce: payment.receiptJson.nonce };
  const committed = { ...payment.receiptJson, rung: "committed", blockNumber: "0x3e8", blockHash: `0x${"ab".repeat(32)}` };
  const landed = { transactionHash: txHash, blockNumber: committed.blockNumber, blockHash: committed.blockHash, status: "0x1" };
  const answers = [submitted, payment.receiptJson, committed, null];
  const { rpc, requests } = await loopbackRpc(context, [
    ...answers.map((result) => ({ result })),
    { result: null },
    { result: { transactionHash: txHash, blockNumber: null } },
    { result: landed },
    { result: { ...submitted, txHash: `0x${"00".repeat(32)}` } },
    { result: { ...submitted, rung: "completed" } },
  ]);

  for (const answer of answers) assert.deepEqual(await rpc.receipt(txHash), answer);
  assert.equal(await rpc.status(txHash), null);
  assert.equal(await rpc.status(txHash), null);
  assert.deepEqual(await rpc.status(txHash), landed);
  await assert.rejects(rpc.receipt(txHash), /invalid-rpc-receipt/);
  await assert.rejects(rpc.receipt(txHash), /invalid-rpc-receipt/);
  await assert.rejects(rpc.receipt(txHash.slice(2)), /invalid-rpc-transaction-hash/);
  await assert.rejects(rpc.status("0x1"), /invalid-rpc-hex/);
  for (const [index, request] of requests.entries()) {
    assert.deepEqual(request.body, {
      jsonrpc: "2.0", id: index + 1,
      method: index >= 4 && index <= 6 ? "eth_getTransactionReceipt" : "pax_sequencerReceipt",
      params: [txHash],
    });
  }
  assert.equal(requests.length, 9);
  assert.deepEqual(rpcHex("0x"), new Uint8Array());
  assert.deepEqual(rpcHex("0x0"), new Uint8Array([0]));
  assert.deepEqual(rpcHex("0x3e8"), new Uint8Array([3, 232]));
  assert.deepEqual(rpcHex("0x0001", 2), new Uint8Array([0, 1]));
  assert.deepEqual(rpcHex("aabb", 2), new Uint8Array([170, 187]));
  for (const value of ["", "0xzz", "0x-1", "0x1\n", "abc"]) assert.throws(() => rpcHex(value), /invalid-rpc-hex/);
  assert.throws(() => rpcHex("0x1", 1), /invalid-rpc-hex/);
}

async function verifyRpcPaymentReportsSubmittedAsPending(context) {
  const txHash = payment.receiptJson.txHash;
  const submitted = { rung: "submitted", txHash, nonce: payment.receiptJson.nonce };
  const { rpc, requests } = await loopbackRpc(context, [
    { result: submitted },
    { result: { ...submitted, txHash: vectors.cases[0].receiptJson.txHash } },
  ]);
  const payer = payment.receiptJson.from.slice(2).padStart(64, "0");
  const verify = (answer, raw = payment.rawTransaction, terms = requirements) =>
    verifyRpcPayment(answer, raw, payer, terms, receiptKeys);
  const refusesHash = (error) => error instanceof MiddlewareError && error.code === "verification-failure";

  assert.deepEqual(await verify(await rpc.receipt(txHash)), { kind: "pending" });
  assert.deepEqual(await verify(submitted, payment.rawTransaction, {
    ...requirements, extra: { layerx: { commitment: "committed" } },
  }), { kind: "pending" });
  await assert.rejects(verify(await rpc.call("pax_sequencerReceipt", [txHash])), refusesHash);
  await assert.rejects(verify(submitted, vectors.cases[0].rawTransaction), refusesHash);
  for (const rung of ["submitted", "receipted", "committed"]) {
    await assert.rejects(verify({ ...submitted, rung, txHash: `0x${"00".repeat(32)}` }), refusesHash);
  }
  await assert.rejects(verify({ activity_id: txHash, state: "pending" }), refusesHash);
  await assert.rejects(verify({ ...submitted, rung: "pending" }), (error) =>
    error instanceof MiddlewareError && error.code === "payment-refused");
  assert.equal(requests.length, 2);
  for (const [index, request] of requests.entries()) {
    assert.deepEqual(request.body, {
      jsonrpc: "2.0", id: index + 1, method: "pax_sequencerReceipt", params: [txHash],
    });
  }
}

async function verifyRpcPaymentAcceptsPinnedSequencerReceipt() {
  const payer = payment.receiptJson.from.slice(2).padStart(64, "0");
  const verify = (answer, keys = receiptKeys, terms = requirements, expectedPayer = payer) =>
    verifyRpcPayment(answer, payment.rawTransaction, expectedPayer, terms, keys);
  const verified = await verify(payment.receiptJson);
  assert.equal(verified.kind, "verified");
  assert.equal(verified.format, "layerx-receipt-proof-v2");
  assert.equal(verified.verificationLevel, "receipted");
  assert.equal(verified.rawTransaction, payment.rawTransaction);
  assert.equal(verified.signedReceipt, payment.signedReceipt);
  assert.equal(verified.verification.rung, "receipted");
  assert.equal(verified.verification.txHash, payment.receiptJson.txHash);
  assert.equal(verified.verification.payment.amount.toString(), requirements.amount);
  assert.equal(verified.receiptKey.publicKey, payment.pinnedKey.publicKey);
  const refuses = (error) => error instanceof MiddlewareError && error.code === "verification-failure";
  await assert.rejects(verify(payment.receiptJson, []), refuses);
  await assert.rejects(verify(payment.receiptJson, [{ ...receiptKeys[0], last_height: 1000n }]), refuses);
  await assert.rejects(verify({ ...payment.receiptJson, height: "0x3e8" }), refuses);
  await assert.rejects(verify({ ...payment.receiptJson, keyId: `0x${"00".repeat(32)}` }), refuses);
  await assert.rejects(verify(payment.receiptJson, receiptKeys, { ...requirements, amount: "1" }), refuses);
  await assert.rejects(verify(payment.receiptJson, receiptKeys, requirements, "00".repeat(32)), refuses);
  await assert.rejects(verify(payment.receiptJson, receiptKeys, {
    ...requirements, extra: { layerx: { ...requirements.extra.layerx, commitment: "committed" } },
  }), refuses);
}

test(paymentRpcSendsRawTransaction);
test(paymentRpcReadsSequencerReceipt);
test(verifyRpcPaymentReportsSubmittedAsPending);
test(verifyRpcPaymentAcceptsPinnedSequencerReceipt);
