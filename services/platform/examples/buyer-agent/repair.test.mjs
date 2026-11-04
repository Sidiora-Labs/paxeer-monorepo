import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { keccak256, verifySequencerReceipt } from "@sidiora/layerx-sdk";
import { buyerHumanUrl, runBuyerAgent } from "./index.mjs";
import {
  LayerXApplicationStateError,
  loadApplicationConfig,
  receiptKeysFromEnvironment,
  SequencerReceiptClient,
} from "../support/runtime.mjs";

import { buyerPaymentFixture } from "./test/payment-fixture.mjs";

const bytes = (value) => Uint8Array.from(Buffer.from(value.slice(2), "hex"));
const hex = (value) => `0x${Buffer.from(value).toString("hex")}`;

async function buyerHumanEndpointConfiguration() {
  const document = JSON.parse(await readFile(new URL("./layerx.example.json", import.meta.url), "utf8"));
  const config = { name: "beta", ...document.environments.beta };
  assert.equal(config.humanUrl, "https://layerx-human.layerx-testnet.svc:9443");
  assert.equal(config.humanUrlEnvironment, "LAYERX_TESTNET_HUMAN_URL");
  assert.equal(config.rpcUrl, "https://api-mainnet-beta.paxeer.network");
  const saved = Object.fromEntries([config.humanUrlEnvironment, "LAYERX_EXAMPLE_ENDPOINT"]
    .map((name) => [name, process.env[name]]));
  const refused = (error) => error instanceof LayerXApplicationStateError && error.state === "refused";
  try {
    delete process.env[config.humanUrlEnvironment];
    delete process.env.LAYERX_EXAMPLE_ENDPOINT;
    assert.equal(buyerHumanUrl(config), config.humanUrl);
    for (const humanUrl of [config.rpcUrl, "https://api1.mainnet-beta.paxeer.network",
      "https://api14.mainnet-beta.paxeer.network", "https://api1.mainnet-beta.paxeer.network.",
      "http://127.0.0.1:9443", `${config.humanUrl}/rpc`]) {
      assert.throws(() => buyerHumanUrl({ ...config, humanUrl }), refused);
    }
    process.env.LAYERX_EXAMPLE_ENDPOINT = "https://api-mainnet-beta.paxeer.network";
    assert.throws(() => buyerHumanUrl(config), refused);
    process.env[config.humanUrlEnvironment] = config.humanUrl;
    assert.equal(buyerHumanUrl(config), config.humanUrl);
    process.env[config.humanUrlEnvironment] = config.rpcUrl;
    assert.throws(() => buyerHumanUrl(config), refused);
    process.env[config.humanUrlEnvironment] = "https://user:password@human.invalid";
    assert.throws(() => buyerHumanUrl(config), { message: "invalid_service_url" });
    assert.equal(buyerHumanUrl({ name: "emulator", ...document.environments.emulator }),
      document.environments.emulator.humanUrl);
  } finally {
    for (const [name, value] of Object.entries(saved)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  }
}

async function betaBuyerPaymentReachesHumanAndVerifiesNodeReceipt(context) {
  const directory = await mkdtemp(join(tmpdir(), "buyer-human-22-31-347-"));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const fixture = await buyerPaymentFixture(context, join(directory, "fulfillments"));
  const document = JSON.parse(await readFile(new URL("./layerx.example.json", import.meta.url), "utf8"));
  const beta = document.environments.beta;
  // Exercise the shared loader and the dedicated deployment override. The
  // generic endpoint deliberately points at the separate node JSON-RPC root.
  const configPath = join(directory, "layerx.example.json");
  await writeFile(configPath, JSON.stringify({ ...document, environments: {
    ...document.environments, beta: { ...beta, resourceUrl: fixture.resourceUrl },
  } }));
  const inputs = {
    LAYERX_EXAMPLE_CONFIG: configPath,
    LAYERX_EXAMPLE_ENDPOINT: fixture.rpcUrl,
    [beta.humanUrlEnvironment]: fixture.humanUrl,
    [beta.tokenEnvironment]: fixture.token,
    [beta.sourceEnvironment]: fixture.source,
    [beta.idempotencyEnvironment]: fixture.paymentKey,
    [beta.receiptKeysEnvironment]: JSON.stringify(fixture.pinInputs),
  };
  const saved = Object.fromEntries(Object.keys(inputs).map((name) => [name, process.env[name]]));
  const savedArguments = process.argv;
  let config;
  let result;
  try {
    Object.assign(process.env, inputs);
    process.argv = [process.execPath, new URL("./index.mjs", import.meta.url).pathname, "--environment", "beta"];
    config = await loadApplicationConfig(new URL("./index.mjs", import.meta.url).href, "buyer-agent");
    const humanOrigin = buyerHumanUrl(config);
    assert.equal(humanOrigin, fixture.humanUrl);
    assert.equal(new URL(config.rpcUrl).origin, fixture.rpcUrl);
    assert.notEqual(new URL(config.resourceUrl).origin, humanOrigin);
    assert.notEqual(new URL(config.rpcUrl).origin, humanOrigin);
    result = await runBuyerAgent(config);
    const pins = receiptKeysFromEnvironment(config.receiptKeysEnvironment);
    assert.equal(result.environment, "beta");
    assert.equal(result.state, "paid", JSON.stringify({ result, failures: fixture.failures.map(String) }));
    assert.equal(result.status, 200);
    assert.equal(result.rung, "receipted");
    assert.deepEqual(JSON.parse(result.body), fixture.body);
    const { exchanges, payments } = fixture;
    assert.deepEqual(fixture.failures, []);
    for (const path of ["/v1/moves/quote", "/v1/moves"]) {
      assert(exchanges.some((call) => call.origin === humanOrigin && call.path === path
        && call.method === "POST" && call.status >= 200 && call.status < 300
        && (path !== "/v1/moves" || call.idempotencyKey === fixture.paymentKey)), `missing real Human ${path}`);
    }
    assert(exchanges.some((call) => call.origin === humanOrigin && call.path.startsWith("/v1/evidence/")
      && call.method === "GET" && call.status === 200));
    assert(exchanges.some((call) => call.origin === new URL(config.resourceUrl).origin && call.status === 402));
    assert.equal(payments.length, 1, "seller must receive the real buyer-produced payment");
    const portable = payments[0].payload;
    const proof = await new SequencerReceiptClient(config.rpcUrl, pins).resolve(portable.rawTransaction);
    assert.equal(proof.signedReceipt, portable.signedReceipt);
    assert.equal(proof.signedReceipt, fixture.vector.signedReceipt);
    assert.equal(proof.rawTransaction, fixture.vector.rawTransaction);
    assert.equal(proof.receiptDigest, result.receiptDigest);
    assert.equal(result.transactionHash, hex(keccak256(bytes(portable.rawTransaction))));
    await verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), pins);
    const corrupted = bytes(proof.rawTransaction);
    corrupted[corrupted.length - 1] ^= 1;
    await assert.rejects(verifySequencerReceipt(bytes(proof.signedReceipt), corrupted, pins));
    await assert.rejects(verifySequencerReceipt(bytes(proof.signedReceipt), bytes(proof.rawTransaction), []));
    for (const call of exchanges) {
      assert.equal(call.authorization, call.origin === humanOrigin ? `Bearer ${fixture.token}` : null);
      if (call.origin === fixture.rpcUrl) assert.equal(call.path, "/");
    }
    assert(exchanges.filter((call) => call.origin === fixture.rpcUrl).length >= 2,
      "Human evidence and independent receipt verification must both read node RPC");
    assert.deepEqual(fixture.failures, []);
  } finally {
    process.argv = savedArguments;
    for (const [name, value] of Object.entries(saved)) {
      if (value === undefined) delete process.env[name];
      else process.env[name] = value;
    }
  }
}

test(buyerHumanEndpointConfiguration);
test(betaBuyerPaymentReachesHumanAndVerifiesNodeReceipt);
