import assert from "node:assert/strict";
import { test } from "node:test";
import * as sdk from "../src/index.js";

const verifyReceipt: unknown = Reflect.get(sdk, "verifyReceipt");
const verifyReceiptOutcome: unknown = Reflect.get(sdk, "verifyReceiptOutcome");
const verifyProgramReceipt: unknown = Reflect.get(sdk, "verifyProgramReceipt");

function kernel_receipt_paths_are_gone(): void {
  assert.equal(verifyReceipt, undefined, "verifyReceipt must be retired");
  assert.equal(verifyReceiptOutcome, undefined, "verifyReceiptOutcome must be retired");
  assert.equal(verifyProgramReceipt, undefined, "verifyProgramReceipt must be retired");
  for (const name of [
    "verifyProgramLifecycleReceipt",
    "verifyNativeProgramReceiptOutcomeV5",
    "decodeProgramReceiptOutcome",
    "decodeProtocolReceipt",
    "verifyProgramReceiptV5",
    "programsModuleVersionForProtocol",
    "resolveLifecycleResponse",
    "verifyLifecycleRecovery",
  ]) {
    assert.equal(Reflect.get(sdk, name), undefined, `${name} must be retired`);
  }
  assert.equal(typeof sdk.verifySequencerReceipt, "function");
  assert.equal(typeof sdk.verifyCommittedReceipt, "function");
}

test(kernel_receipt_paths_are_gone);
