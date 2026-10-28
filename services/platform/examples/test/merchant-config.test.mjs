import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function loadApplicationConfig(application) {
  const path = new URL(`../${application}/layerx.example.json`, import.meta.url);
  return JSON.parse(readFileSync(path, "utf8"));
}

function merchantExampleConfigsUseEip155AndReceiptKeys() {
  for (const application of ["merchant-checkout", "merchant-shop"]) {
    const document = loadApplicationConfig(application);
    assert.equal(document.version, 1, application);
    assert.equal(document.application, application);
    assert.deepEqual(Object.keys(document.environments).sort(), ["beta", "emulator"]);

    for (const [environment, config] of Object.entries(document.environments)) {
      const label = `${application}/${environment}`;
      assert.equal(config.network, "eip155:125", label);
      assert.equal(
        config.rpcUrl,
        environment === "emulator"
          ? "http://127.0.0.1:9402/rpc"
          : "https://api-mainnet-beta.paxeer.network/rpc",
        label,
      );
      assert.equal(
        config.receiptKeysEnvironment,
        environment === "emulator"
          ? "LAYERX_EMULATOR_RECEIPT_KEYS_JSON"
          : "LAYERX_TESTNET_RECEIPT_KEYS_JSON",
        label,
      );
      assert.equal(Object.hasOwn(config, "receiptAuthorityUrl"), false, label);
      assert.equal(Object.hasOwn(config, "protocolVersion"), false, label);
    }
  }
}

test(merchantExampleConfigsUseEip155AndReceiptKeys);
