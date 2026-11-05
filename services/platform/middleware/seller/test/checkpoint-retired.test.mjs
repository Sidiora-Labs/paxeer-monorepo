import assert from "node:assert/strict";
import test from "node:test";
import * as seller from "../dist/index.js";

function packageEntryHasNoCheckpointEvidenceExports() {
  assert.equal(Object.hasOwn(seller, "RpcCheckpointAuthority"), false);
  assert.equal(Object.hasOwn(seller, "rpcCheckpointEvidence"), false);
  assert.equal(Object.hasOwn(seller, "rpcBatchEvidence"), false);
  assert.equal(typeof seller.verifyPaymentReceipt, "function");
}

test(packageEntryHasNoCheckpointEvidenceExports);
