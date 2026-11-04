import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { createHash, generateKeyPairSync } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { emulatorReceiptKeys, merchantCheckoutOutcome, scenarioReceiptKeys } from "../run-reference-apps.mjs";
import { programReceiptOutcome } from "../marketplace/receipt-outcome.mjs";

function referenceAppsCheckPassesWithoutRetiredSurfaces() {
  const env = Object.fromEntries(Object.entries(process.env).filter(([name]) => !name.startsWith("LAYERX_")));
  const result = spawnSync(process.execPath, [
    fileURLToPath(new URL("../run-reference-apps.mjs", import.meta.url)), "--check",
  ], {
    env,
    encoding: "utf8",
    timeout: 10000,
  });

  assert.ifError(result.error);
  assert.equal(result.signal, null);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stderr, "");
  const checked = JSON.parse(result.stdout);
  assert.deepEqual(checked.checked, ["buyer-agent", "paid-api", "merchant-shop", "marketplace"]);
  assert.deepEqual(checked.environments, ["emulator", "beta"]);
}

test(referenceAppsCheckPassesWithoutRetiredSurfaces);

function emulatorReceiptKeysReachScenarioVerifier(t) {
  const name = "LAYERX_EMULATOR_RECEIPT_KEYS_JSON";
  const previous = process.env[name];
  t.after(() => {
    if (previous === undefined) delete process.env[name];
    else process.env[name] = previous;
  });
  const { publicKey } = generateKeyPairSync("ed25519");
  const raw = publicKey.export({ format: "der", type: "spki" }).subarray(-32);
  const encoded = emulatorReceiptKeys(raw.toString("hex"));
  const expected = [{
    keyId: Uint8Array.from(createHash("sha256").update(raw).digest()),
    publicKey: Uint8Array.from(raw),
    first_height: 1n,
    last_height: 0xffff_ffff_ffff_ffffn,
  }];

  delete process.env[name];
  assert.deepEqual(scenarioReceiptKeys({ [name]: encoded }, name), expected);
  assert.equal(process.env[name], undefined);
  process.env[name] = "invalid inherited receipt keys";
  assert.deepEqual(scenarioReceiptKeys({ [name]: encoded }, name), expected);
  assert.equal(process.env[name], "invalid inherited receipt keys");
  assert.throws(() => scenarioReceiptKeys({ [name]: "[]" }, name), /invalid_receipt_keys/u);
  assert.equal(process.env[name], "invalid inherited receipt keys");
  assert.equal(emulatorReceiptKeys(`0x${raw.toString("hex")}`), encoded);
  assert.throws(() => emulatorReceiptKeys("0".repeat(63)), /invalid_emulator_receipt_public_key/u);
}

async function referenceOutcomesCarryVerifiedReceiptFields() {
  const document = JSON.parse(await readFile(new URL(
    "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json", import.meta.url,
  ), "utf8"));
  assert.equal(document.provenance.generator, "rpc/sequencer_receipt_vectors_test.go");
  const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
  for (const id of ["V1-layerxledger-pay-exact", "V2-layerxledger-revert"]) {
    const vector = document.cases.find((entry) => entry.id === id);
    assert.ok(vector);
    const verified = await verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [{
      keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
      first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight),
    }]);
    const outcome = programReceiptOutcome(vector.receiptJson, verified, vector.expected.rung);
    assert.equal(outcome.state, id === "V2-layerxledger-revert" ? "refused" : "completed");
    assert.equal(outcome.resultCode, verified.receipt.resultCode);
    assert.equal(outcome.resultCode, id === "V2-layerxledger-revert" ? 1 : 0);
    assert.equal(outcome.transactionHash, vector.receiptJson.txHash);
    assert.equal(outcome.receiptDigest, vector.receiptDigest.slice(2));
    assert.equal(outcome.rung, "receipted");
    assert.equal(Object.hasOwn(outcome, "status"), false);
    assert.deepEqual(outcome.output, vector.receiptJson);

    if (outcome.state === "completed") {
      const merchant = merchantCheckoutOutcome("emulator", "verified-order", outcome.receiptDigest, {
        txHash: outcome.transactionHash, verification: { ...verified, rung: outcome.rung },
      });
      assert.deepEqual(merchant, {
        application: "merchant-shop", environment: "emulator", state: "paid", orderId: "verified-order",
        receiptDigest: outcome.receiptDigest, rung: "receipted", transactionHash: vector.receiptJson.txHash,
        resultCode: verified.receipt.resultCode,
      });
      assert.equal(Object.hasOwn(merchant, "verification"), false);
      const committed = merchantCheckoutOutcome("beta", "verified-order", outcome.receiptDigest, {
        txHash: outcome.transactionHash, verification: { ...verified, rung: "committed" },
      });
      assert.equal(committed.environment, "beta");
      assert.equal(committed.rung, "committed");
      assert.equal(committed.transactionHash, outcome.transactionHash);
      assert.equal(Object.hasOwn(committed, "verification"), false);
    }
  }
}

test(emulatorReceiptKeysReachScenarioVerifier);
test(referenceOutcomesCarryVerifiedReceiptFields);
