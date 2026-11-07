import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { promisify } from "node:util";
import { fileURLToPath } from "node:url";
import { verifyPaymentReceipt, MiddlewareError } from "../../seller/dist/index.js";
import { pinnedReceiptKeys } from "./generate_evm_receipt.mjs";

async function evmReceiptFixtureMatchesGenerator() {
  const directory = await mkdtemp(join(tmpdir(), "pay6-evm-receipt-"));
  try {
    const output = join(directory, "evm-receipt.json");
    await promisify(execFile)(process.execPath, [fileURLToPath(new URL("generate_evm_receipt.mjs", import.meta.url)), output]);
    assert.deepEqual(await readFile(output), await readFile(new URL("evm-receipt.json", import.meta.url)));
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
}

async function evmReceiptFixtureVerifiesUnderPinnedKey() {
  const fixture = JSON.parse(await readFile(new URL("evm-receipt.json", import.meta.url), "utf8"));
  const keys = pinnedReceiptKeys(fixture);
  const verified = await verifyPaymentReceipt(fixture.receipt, fixture.offer, keys);
  assert.equal(verified.txHash, fixture.txHash);
  assert.equal(verified.rung, "receipted");
  assert.equal(verified.height, BigInt(fixture.rpcResult.height));
  assert.equal(verified.payment.amount, BigInt(fixture.offer.amount));
  assert.equal(Buffer.from(verified.payment.assetId).toString("hex"), fixture.offer.asset);
  assert.equal(Buffer.from(verified.payment.to).toString("hex"), fixture.offer.payTo);
  const expired = { ...keys[0], last_height: verified.height - 1n };
  assert.ok(expired.last_height >= expired.first_height);
  const evidence = {
    ...fixture.receipt,
    receiptKey: { ...fixture.receipt.receiptKey, lastHeight: `0x${expired.last_height.toString(16)}` },
  };
  await assert.rejects(verifyPaymentReceipt(evidence, fixture.offer, [expired]),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure");
  await assert.rejects(verifyPaymentReceipt(fixture.receipt, fixture.offer, [expired]),
    (error) => error instanceof MiddlewareError && error.code === "verification-failure");
}

test(evmReceiptFixtureMatchesGenerator);
test(evmReceiptFixtureVerifiesUnderPinnedKey);
