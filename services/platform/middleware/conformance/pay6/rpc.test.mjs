import assert from "node:assert/strict";
import { test } from "node:test";
import { verifyRpcPayment, verifyPaymentReceipt, PaymentRpc, PaymentRpcError } from "../../seller/dist/index.js";
import { HostedGatewayRpc } from "../../seller/dist/rpc.js";
import { keys, offer, payer, proof as receipt, txHash, result } from "./node-fixtures.mjs";
import { loopback } from "./peers.mjs";

async function rpcReceiptBindingAndPending() {
  const verified = await verifyPaymentReceipt(receipt, offer, keys);
  assert.equal(verified.txHash, txHash);
  assert.equal(`0x${Buffer.from(verified.receipt.from).toString("hex")}`, payer);
  const verify = (body, expectedPayer = payer, requirements = offer) => verifyRpcPayment(body, receipt.rawTransaction, expectedPayer, requirements, keys);
  assert.deepEqual((await verify(result)).verification, verified);
  assert.equal((await verify({ txHash, rung: "submitted", nonce: "0x0" })).kind, "pending");
  for (const change of [
    { txHash: `0x${"ab".repeat(32)}` }, { receipt: "0x00" },
    { rung: "acknowledged" }, { rung: "batched" }, { rung: "committed" },
    { rung: "committed", blockHash: `0x${"12".repeat(32)}` },
    { height: "0x8" }, { keyId: `0x${"ab".repeat(32)}` },
  ]) {
    await assert.rejects(verify({ ...result, ...change }));
    const requirements = change.rung === "committed" ? { ...offer,
      extra: { layerx: { ...offer.extra.layerx, commitment: "committed" } } } : offer;
    await assert.rejects(verify({ ...result, ...change }, payer, requirements));
  }
  await assert.rejects(verify(result, `0x${"ab".repeat(32)}`));
  await assert.rejects(verify({ txHash }));
}

async function rpcRootAndHostedOrigins(context) {
  const node = await loopback(context, (body, request) => {
    assert.equal(request.url, "/");
    assert.equal(request.headers.authorization, "Bearer node-transcript");
    if (body.method === "eth_sendRawTransaction") {
      assert.deepEqual(body.params, [receipt.rawTransaction]);
      return { result: txHash };
    }
    assert.equal(body.method, "pax_sequencerReceipt");
    assert.deepEqual(body.params, [txHash]);
    return { result };
  });
  const gateway = await loopback(context, (body, request) => {
    assert.equal(request.url, "/rpc");
    assert.equal(request.headers.authorization, "Bearer gateway-transcript");
    assert.equal(body.method, "px_getReceiptKeys");
    assert.deepEqual(body.params, []);
    return { result: { keys: [{ ...receipt.receiptKey, firstHeight: Number(keys[0].first_height), lastHeight: Number(keys[0].last_height) }] } };
  });
  const rpc = new PaymentRpc(node.endpoint, { authorization: "Bearer node-transcript" });
  const hosted = new HostedGatewayRpc(`${gateway.endpoint}rpc`, { authorization: "Bearer gateway-transcript" });
  assert.equal(await rpc.send(receipt.rawTransaction), txHash);
  assert.equal((await verifyRpcPayment(await rpc.receipt(txHash), receipt.rawTransaction, payer, offer, keys)).kind, "verified");
  assert.equal((await hosted.call("px_getReceiptKeys", [])).keys[0].keyId, receipt.receiptKey.keyId);
  assert.notEqual(node.endpoint, gateway.endpoint);
  assert.deepEqual(node.requests.map((request) => request.url), ["/", "/"]);
  assert.deepEqual(node.requests.map((request) => request.body.id), [1, 2]);
  assert.deepEqual(gateway.requests.map((request) => request.body.id), [1]);
  assert.throws(() => rpc.call("px_getReceiptKeys", []), /invalid-node-method/u);
  assert.throws(() => hosted.call("eth_sendRawTransaction", []), /invalid-gateway-method/u);
}

async function rpcEndpointAndSubmissionBounds() {
  assert.ok(new PaymentRpc("https://example.com/"));
  assert.ok(new PaymentRpc("http://[::1]:1/"));
  for (const endpoint of ["http://example.com/", "https://user:password@example.com/", "https://example.com/?token=1",
    "https://example.com/#token", "https://example.com/rpc"]) assert.throws(() => new PaymentRpc(endpoint));
  const rpc = new PaymentRpc("http://127.0.0.1:1/");
  for (const commitment of ["submitted", "finalized", "acknowledged"]) await assert.rejects(rpc.send(receipt.rawTransaction, commitment));
  for (const raw of ["0x00", "0x02", receipt.rawTransaction.slice(2), `${receipt.rawTransaction}f`, `0x${"00".repeat(524_289)}`]) {
    await assert.rejects(rpc.send(raw));
  }
}

async function rpcRejectsRedirectsOversizeAndMalformedResponses(context) {
  const destination = await loopback(context, () => { assert.fail("redirect must never reach another origin"); });
  const redirect = await loopback(context, () => ({ status: 307, headers: { location: destination.endpoint }, text: "" }));
  await assert.rejects(new PaymentRpc(redirect.endpoint).receipt(txHash));
  assert.equal(destination.requests.length, 0);
  const oversized = await loopback(context, () => ({ text: "x".repeat(8_388_609) }));
  await assert.rejects(new PaymentRpc(oversized.endpoint).receipt(txHash), /rpc-body-too-large/u);
  for (const body of [
    { jsonrpc: "2.0", id: 999, result }, { jsonrpc: "1.0", id: 1, result },
    { jsonrpc: "2.0", id: 1, result, error: { code: -1, message: "both" } },
    { jsonrpc: "2.0", id: 1, result: { ...result, txHash: `0x${"ff".repeat(32)}` } },
  ]) {
    const malformed = await loopback(context, () => ({ text: JSON.stringify(body) }));
    await assert.rejects(new PaymentRpc(malformed.endpoint).receipt(txHash));
  }
  const wrongHash = await loopback(context, () => ({ result: "0x1234" }));
  await assert.rejects(new PaymentRpc(wrongHash.endpoint).send(receipt.rawTransaction), /invalid-rpc-transaction-hash/u);
  const invalidNonce = await loopback(context, () => ({ error: { code: -32000, message: "invalid nonce", data: { nonce: "0x00" } } }));
  await assert.rejects(new PaymentRpc(invalidNonce.endpoint).send(receipt.rawTransaction), (error) =>
    error instanceof PaymentRpcError && error.code === -32000 && error.data.nonce === "0x00");
}

test(rpcReceiptBindingAndPending);
test(rpcRootAndHostedOrigins);
test(rpcEndpointAndSubmissionBounds);
test(rpcRejectsRedirectsOversizeAndMalformedResponses);
