import assert from "node:assert/strict";
import test from "node:test";
import * as seller from "@sidiora/layerx-seller-middleware";

function packageEntryHasNoKernelBatchEvidenceExports() {
  for (const name of ["rpcBatchEvidence", "rpcCheckpointEvidence", "RpcCheckpointAuthority", "MERKLE_LEAF_DOMAIN"]) {
    assert.equal(Object.hasOwn(seller, name), false, `${name} must remain retired`);
  }
}

function packageEntryKeepsLedgerAndReceiptExports() {
  for (const name of ["verifyPaymentReceipt", "verifyRpcPayment", "decodePaymentExactLog"]) {
    assert.equal(typeof seller[name], "function", `${name} must remain available`);
  }
}

test(packageEntryHasNoKernelBatchEvidenceExports);
test(packageEntryKeepsLedgerAndReceiptExports);
