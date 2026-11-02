import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { verifyRpcPayment, verifyPaymentReceipt, PaymentRpc } from "../../seller/dist/index.js";
import { pinnedReceiptKeys } from "./generate_evm_receipt.mjs";

const fixture = JSON.parse(readFileSync(new URL("evm-receipt.json", import.meta.url), "utf8"));
const keys = pinnedReceiptKeys(fixture);
const { receipt, offer, payer, txHash, rpcResult: result } = fixture;

test("RPC receipt response binds signed transaction hash and payer; submitted never succeeds", async () => {
  const verified = await verifyPaymentReceipt(receipt, offer, keys);
  assert.equal(verified.txHash, txHash);
  assert.equal(`0x${Buffer.from(verified.receipt.from).toString("hex")}`, payer);
  const verify = (body, expectedPayer = payer) => verifyRpcPayment(body, receipt.rawTransaction, expectedPayer, offer, keys);
  assert.deepEqual((await verify(result)).verification, verified);
  assert.equal((await verify({ txHash, rung: "submitted", nonce: "0x0" })).kind, "pending");
  for (const change of [
    { txHash: `0x${"ab".repeat(32)}` }, { receipt: "0x00" },
    { rung: "acknowledged" }, { rung: "batched" }, { rung: "committed" },
    { height: "0x8" }, { keyId: `0x${"ab".repeat(32)}` },
  ]) await assert.rejects(verify({ ...result, ...change }));
  await assert.rejects(verify(result, `0x${"ab".repeat(32)}`));
  await assert.rejects(verify({ txHash }));
});

test("RPC endpoint and submission bounds refuse before network access", async () => {
  for (const endpoint of ["http://example.com/rpc", "https://user:password@example.com/rpc", "https://example.com/rpc?token=1", "https://example.com/"]) assert.throws(() => new PaymentRpc(endpoint));
  const rpc = new PaymentRpc("http://127.0.0.1:1/rpc");
  for (const commitment of ["finalized", "acknowledged"]) await assert.rejects(rpc.send(receipt.rawTransaction, commitment));
  await assert.rejects(rpc.send("0x00", "receipted"));
});
