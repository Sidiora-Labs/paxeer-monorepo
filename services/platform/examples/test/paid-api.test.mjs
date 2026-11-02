import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { generateKeyPairSync } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createInterface } from "node:readline";
import test from "node:test";
import { fileURLToPath } from "node:url";

const entrypoint = fileURLToPath(new URL("../paid-api/index.mjs", import.meta.url));

async function startPaidApi(t, environment, withReceiptKeys) {
  const directory = await mkdtemp(join(tmpdir(), "layerx-paid-api-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const config = JSON.parse(await readFile(new URL("../paid-api/layerx.example.json", import.meta.url), "utf8"));
  const selected = config.environments[environment];
  const reservation = createServer();
  reservation.listen(0, "127.0.0.1");
  await once(reservation, "listening");
  const port = reservation.address().port;
  await new Promise((resolve, reject) => reservation.close((error) => error ? reject(error) : resolve()));
  selected.port = port;
  selected.resourceUrl = `http://127.0.0.1:${port}/paid`;
  selected.resourceFile = join(directory, "resource.json");
  await writeFile(selected.resourceFile, JSON.stringify({ temperature: 21 }));
  const configPath = join(directory, "layerx.example.json");
  await writeFile(configPath, JSON.stringify(config));
  const env = {
    ...process.env,
    LAYERX_EXAMPLE_CONFIG: configPath,
    LAYERX_EXAMPLE_STATE_ROOT: directory,
    [selected.assetEnvironment]: "11".repeat(32),
    [selected.payToEnvironment]: "22".repeat(32),
    [selected.accountEnvironment]: "agent:paid-api:main",
    [selected.currencyEnvironment]: "USD",
    [selected.priceEnvironment]: "10",
  };
  delete env.LAYERX_EXAMPLE_ENDPOINT;
  delete env.LAYERX_RECEIPT_KEYS_JSON;
  if (withReceiptKeys) {
    const { publicKey } = generateKeyPairSync("ed25519");
    env.LAYERX_RECEIPT_KEYS_JSON = JSON.stringify([{
      keyId: `0x${"33".repeat(32)}`,
      publicKey: `0x${publicKey.export({ type: "spki", format: "der" }).subarray(-32).toString("hex")}`,
      firstHeight: "0",
      lastHeight: "18446744073709551615",
    }]);
  }
  const child = spawn(process.execPath, [entrypoint, "--environment", environment], {
    env,
    stdio: ["ignore", "pipe", "pipe"],
    timeout: 10000,
  });
  const finished = once(child, "close");
  const lines = createInterface({ input: child.stdout });
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => { stdout += chunk; });
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  t.after(async () => {
    if (child.exitCode === null && child.signalCode === null) child.kill();
    await finished;
    lines.close();
  });
  return { child, finished, lines, port, output: () => ({ stdout, stderr }) };
}

async function paidApiAdvertisesEip155ReceiptedOffer(t) {
  for (const environment of ["emulator", "beta"]) {
    const app = await startPaidApi(t, environment, true);
    const [line] = await Promise.race([
      once(app.lines, "line"),
      app.finished.then(() => { throw new Error(`paid API exited before listening: ${app.output().stderr}`); }),
    ]);
    const listening = JSON.parse(line);
    assert.equal(listening.application, "paid-api");
    assert.equal(listening.environment, environment);
    assert.equal(listening.listening, app.port);
    const response = await fetch(`http://127.0.0.1:${app.port}/paid`, { signal: AbortSignal.timeout(5000) });
    assert.equal(response.status, 402);
    const offer = await response.json();
    assert.equal(offer.x402Version, 2);
    assert.equal(offer.accepts.length, 1);
    assert.equal(offer.accepts[0].network, "eip155:125");
    assert.equal(offer.accepts[0].extra.layerx.commitment, "receipted");
    const header = response.headers.get("PAYMENT-REQUIRED");
    assert.ok(header);
    assert.deepEqual(JSON.parse(Buffer.from(header, "base64").toString("utf8")), offer);
    app.child.kill();
    await app.finished;
  }
}

async function paidApiRefusesStartWithoutReceiptKeys(t) {
  for (const environment of ["emulator", "beta"]) {
    const app = await startPaidApi(t, environment, false);
    const [code, signal] = await app.finished;
    assert.equal(signal, null, app.output().stderr);
    assert.ok(Number.isInteger(code) && code !== 0, `expected non-zero exit, got ${code}`);
    assert.match(app.output().stderr, /missing_layerx_receipt_keys_json/iu);
    assert.doesNotMatch(app.output().stdout, /"listening"/u);
  }
}

test(paidApiAdvertisesEip155ReceiptedOffer);
test(paidApiRefusesStartWithoutReceiptKeys);
