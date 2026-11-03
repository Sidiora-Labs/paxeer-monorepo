import assert from "node:assert/strict";
import test from "node:test";
import { runServiceScenarios } from "../service.mjs";
import { Suite } from "../dist/support.js";

let scenarios;
function serviceScenarios() {
  scenarios ??= (async () => {
    const suite = new Suite();
    const written = await runServiceScenarios(suite);
    return { suite, ...written };
  })();
  return scenarios;
}

async function runServiceScenariosPassesAgainstExampleApps() {
  const { suite } = await serviceScenarios();
  const results = suite.results();
  assert.equal(results.length, 13, "all paid API, merchant and agent checks must execute");
  assert.equal(new Set(results.map((result) => result.name)).size, results.length);
  assert.deepEqual(results.filter((result) => !result.ok), [], "every service scenario must pass");
}

async function serviceConfigCarriesReceiptKeysEnvironment() {
  const { config, receiptKeysEnvironment, receiptKey } = await serviceScenarios();
  assert.equal(config.application, "paid-api");
  const selected = config.environments.emulator;
  assert.equal(selected.network, "eip155:125");
  assert.equal(selected.receiptKeysEnvironment, "LAYERX_RECEIPT_KEYS_JSON");
  assert.equal(Object.hasOwn(selected, "authorizedBatchEnvironment"), false);
  assert.equal(Object.hasOwn(selected, "protocolVersion"), false);
  const keys = JSON.parse(receiptKeysEnvironment);
  assert.deepEqual(keys, [{
    keyId: Buffer.from(receiptKey.keyId).toString("hex"),
    publicKey: Buffer.from(receiptKey.publicKey).toString("hex"),
    firstHeight: receiptKey.firstHeight.toString(),
    lastHeight: receiptKey.lastHeight.toString(),
  }]);
}

test(runServiceScenariosPassesAgainstExampleApps);
test(serviceConfigCarriesReceiptKeysEnvironment);
