import "./support/source-loader.mjs";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { SequencerReceiptClient, receiptKeysFromEnvironment } from "../support/runtime.mjs";
import { nodeReceipt, portableReceipt, recordedNode } from "./support/node-receipts.mjs";

function loadApplicationConfig(application) {
  const path = new URL(`../${application}/layerx.example.json`, import.meta.url);
  return JSON.parse(readFileSync(path, "utf8"));
}

function merchantExampleConfigsUseEip155AndReceiptKeys() {
  const emulatorRpcUrls = {
    "merchant-checkout": "http://127.0.0.1:9402/rpc",
    "merchant-shop": "http://127.0.0.1:9402",
    marketplace: "http://127.0.0.1:8545",
  };
  for (const application of ["merchant-checkout", "merchant-shop", "marketplace"]) {
    const document = loadApplicationConfig(application);
    assert.equal(document.version, 1, application);
    assert.equal(document.application, application);
    assert.deepEqual(Object.keys(document.environments).sort(), ["beta", "emulator"]);

    for (const [environment, config] of Object.entries(document.environments)) {
      const label = `${application}/${environment}`;
      assert.equal(config.network, "eip155:125", label);
      if (environment === "beta") assert.equal(new URL(config.rpcUrl).pathname, "/", label);
      assert.equal(
        config.rpcUrl,
        environment === "emulator"
          ? emulatorRpcUrls[application]
          : "https://api-mainnet-beta.paxeer.network",
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

async function merchantConfigTransportsNodeReceiptWithRequiredPins(t) {
  const vector = nodeReceipt("V1-layerxledger-pay-exact");
  const node = await recordedNode(t, vector);
  for (const application of ["merchant-checkout", "merchant-shop", "marketplace"]) {
    const config = loadApplicationConfig(application).environments.beta;
    assert.equal(new URL(config.rpcUrl).pathname, "/");
    const binding = config.receiptKeysEnvironment;
    const previous = process.env[binding];
    try {
      delete process.env[binding];
      assert.throws(() => receiptKeysFromEnvironment(binding), /missing_/u);
      process.env[binding] = "[]";
      assert.throws(() => receiptKeysFromEnvironment(binding), /invalid_receipt_keys/u);
      process.env[binding] = JSON.stringify([vector.pinnedKey]);
      const pins = receiptKeysFromEnvironment(binding);
      const proof = await new SequencerReceiptClient(node.rpcUrl, pins).resolve(vector.raw);
      assert.deepEqual(proof, portableReceipt(vector));
    } finally {
      if (previous === undefined) delete process.env[binding]; else process.env[binding] = previous;
    }
  }
  assert.equal(node.requests.length, 3);
  assert.equal(node.requests.every(({ path }) => path === "/"), true);
}

test(merchantConfigTransportsNodeReceiptWithRequiredPins);
