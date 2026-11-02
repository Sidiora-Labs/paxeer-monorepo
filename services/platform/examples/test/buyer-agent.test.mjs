import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { generateKeyPairSync } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const entrypoint = fileURLToPath(new URL("../buyer-agent/index.mjs", import.meta.url));
const exampleConfig = new URL("../buyer-agent/layerx.example.json", import.meta.url);

async function buyerAgentRefusesLayerxNetworkConfig(t) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-buyer-agent-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  let requests = 0;
  let connections = 0;
  const observer = createServer((request, response) => {
    requests += 1;
    response.destroy();
  });
  observer.on("connection", () => { connections += 1; });
  observer.listen(0, "127.0.0.1");
  await once(observer, "listening");
  t.after(() => new Promise((resolve, reject) => {
    observer.closeAllConnections();
    observer.close((error) => error ? reject(error) : resolve());
  }));
  const endpoint = `http://127.0.0.1:${observer.address().port}`;
  const { publicKey } = generateKeyPairSync("ed25519");
  const receiptKeys = JSON.stringify([{
    keyId: `0x${"33".repeat(32)}`,
    publicKey: `0x${publicKey.export({ type: "spki", format: "der" }).subarray(-32).toString("hex")}`,
    firstHeight: "0",
    lastHeight: "18446744073709551615",
  }]);

  for (const environment of ["emulator", "beta"]) {
    const document = JSON.parse(await readFile(exampleConfig, "utf8"));
    const selected = document.environments[environment];
    selected.network = "layerx:beta";
    selected.humanUrl = endpoint;
    selected.rpcUrl = `${endpoint}/rpc`;
    selected.resourceUrl = `${endpoint}/paid`;
    const configPath = join(directory, `${environment}.json`);
    await writeFile(configPath, JSON.stringify(document));
    const env = {
      ...process.env,
      LAYERX_EXAMPLE_CONFIG: configPath,
      [selected.tokenEnvironment]: "buyer-agent-network-refusal-token",
      [selected.sourceEnvironment]: "agent:buyer-agent:main",
      [selected.idempotencyEnvironment]: "buyer-agent-network-refusal",
      [selected.receiptKeysEnvironment]: receiptKeys,
    };
    delete env.LAYERX_EXAMPLE_ENDPOINT;
    const child = spawn(process.execPath, [entrypoint, "--environment", environment], {
      env,
      stdio: ["ignore", "pipe", "pipe"],
      timeout: 10000,
    });
    const finished = once(child, "close");
    t.after(async () => {
      if (child.exitCode === null && child.signalCode === null) child.kill();
      await finished;
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    const [code, signal] = await finished;
    assert.equal(signal, null, stderr);
    assert.equal(code, 4, `${environment}: ${stderr}`);
    assert.equal(stderr, "", environment);
    const lines = stdout.trim().split("\n");
    assert.equal(lines.length, 1, stdout);
    assert.deepEqual(JSON.parse(lines[0]), {
      application: "buyer-agent",
      environment,
      state: "refused",
      detail: "unsupported-payment",
    });
    assert.equal(requests, 0, `${environment}: no HTTP requests before refusal`);
    assert.equal(connections, 0, `${environment}: no network connections before refusal`);
  }
}

async function buyerAgentExampleConfigHasNoReceiptAuthority() {
  const document = JSON.parse(await readFile(exampleConfig, "utf8"));
  assert.equal(document.version, 1);
  assert.equal(document.application, "buyer-agent");
  assert.deepEqual(Object.keys(document.environments).sort(), ["beta", "emulator"]);
  for (const [environment, config] of Object.entries(document.environments)) {
    assert.equal(config.network, "eip155:125", environment);
    assert.equal(
      config.rpcUrl,
      environment === "emulator"
        ? "http://127.0.0.1:9402/rpc"
        : "https://api-mainnet-beta.paxeer.network/rpc",
      environment,
    );
    assert.equal(
      config.receiptKeysEnvironment,
      environment === "emulator"
        ? "LAYERX_EMULATOR_RECEIPT_KEYS_JSON"
        : "LAYERX_TESTNET_RECEIPT_KEYS_JSON",
      environment,
    );
    assert.equal(Object.hasOwn(config, "receiptAuthorityUrl"), false, environment);
    assert.equal(Object.hasOwn(config, "protocolVersion"), false, environment);
  }
}

test(buyerAgentRefusesLayerxNetworkConfig);
test(buyerAgentExampleConfigHasNoReceiptAuthority);
