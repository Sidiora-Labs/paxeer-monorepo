import "./support/source-loader.mjs";
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
    selected.humanUrl = environment === "beta" ? "https://human.example" : endpoint;
    selected.rpcUrl = endpoint;
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
    delete env.LAYERX_TESTNET_HUMAN_URL;
    const child = spawn(process.execPath, ["--disable-warning=ExperimentalWarning", "--import", fileURLToPath(new URL("./support/source-loader.mjs", import.meta.url)), entrypoint, "--environment", environment], {
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
        : "https://api-mainnet-beta.paxeer.network",
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

async function buyerAgentUsesHostedHumanPaymentRoute(t) {
  const { runBuyerAgent, buyerHumanUrl } = await import("../buyer-agent/index.mjs");
  const { SequencerReceiptClient } = await import("../support/runtime.mjs");
  const { nodeReceipt, recordedNode, bytes, hex } = await import("./support/node-receipts.mjs");
  const { decodePaymentExactLog, ReceiptPayloadAuthority, SellerMiddleware } = await import("@sidiora/layerx-seller-middleware");
  const { FileFulfillments } = await import("../support/merchant-app.mjs");
  const vector = nodeReceipt("V1-layerxledger-pay-exact");
  const node = await recordedNode(t, vector);
  const proof = await new SequencerReceiptClient(node.rpcUrl, [vector.key]).resolve(vector.raw);
  const log = vector.receiptJson.logs[0];
  const paid = decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
  const requirements = {
    scheme: "exact", network: "eip155:125", amount: paid.amount.toString(),
    asset: hex(paid.assetId), payTo: hex(paid.to), maxTimeoutSeconds: 120,
    extra: { layerx: { commitment: "receipted", account: "agent:did:layerx:payee:main", currency: "LXP",
      payer: hex(paid.from).slice(2), purposeHash: hex(paid.paymentReference).slice(2) } },
  };
  const golden = async (operation) => JSON.parse(await readFile(new URL(
    `../../../human/schema/human-api/golden/${operation}.response.json`, import.meta.url,
  ), "utf8")).body;
  const quote = await golden("move.quote");
  quote.result.money = { amount: requirements.amount, currency: "LXP" };
  // The recorded quote is time-relative; serving it today must not make it expired.
  quote.result.expires_at = new Date(Date.now() + 60000).toISOString();
  const evidence = await golden("evidence.get");
  Object.assign(evidence.result, { verification: "receipted", content_type: "application/vnd.layerx.receipt-proof-v2+json",
    bytes_base64: Buffer.from(JSON.stringify(proof)).toString("base64") });
  const reference = { evidence_id: evidence.result.evidence_id, class: "layerx-receipt", verification: "receipted" };
  const journey = await golden("move.commit");
  journey.result.evidence = [reference];
  journey.result.stages = [{ ...journey.result.stages[0], evidence: [reference] }];
  const humanRequests = [];
  const failures = [];
  const human = createServer(async (request, response) => {
    try {
      assert.equal(request.headers.authorization, "Bearer buyer-route-token");
      const chunks = [];
      for await (const chunk of request) chunks.push(chunk);
      const body = chunks.length === 0 ? undefined : JSON.parse(Buffer.concat(chunks).toString("utf8"));
      humanRequests.push({ method: request.method, path: request.url, body, key: request.headers["idempotency-key"] });
      let reply;
      switch (request.url) {
        case "/v1/moves/quote":
          assert.equal(request.method, "POST");
          assert.deepEqual(body, { source: "acct:buyer-route", destination: requirements.extra.layerx.account,
            money: { amount: requirements.amount, currency: "LXP" } });
          assert.equal(request.headers["idempotency-key"], undefined);
          reply = quote;
          break;
        case "/v1/moves":
          assert.equal(request.method, "POST");
          assert.deepEqual(body, { quote_id: quote.result.quote_id });
          assert.equal(request.headers["idempotency-key"], "buyer-route-payment");
          reply = journey;
          break;
        case `/v1/evidence/${evidence.result.evidence_id}`:
          assert.equal(request.method, "GET");
          assert.equal(body, undefined);
          reply = evidence;
          break;
        default: assert.fail(`unexpected human route ${request.url}`);
      }
      response.writeHead(200, { "content-type": "application/json" }).end(JSON.stringify(reply));
    } catch (error) {
      failures.push(error.message);
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  human.listen(0, "127.0.0.1");
  await once(human, "listening");
  t.after(() => new Promise((resolve) => { human.closeAllConnections(); human.close(resolve); }));
  const directory = await mkdtemp(join(tmpdir(), "buyer-route-"));
  t.after(() => rm(directory, { recursive: true, force: true }));
  let seller;
  let resourceRequests = 0;
  let releases = 0;
  const resource = createServer(async (request, response) => {
    try {
      resourceRequests += 1;
      assert.equal(request.method, "GET");
      assert.equal(request.url, "/paid");
      const decision = await seller.handle("buyer-route", request.headers["payment-signature"], async () => {
        releases += 1;
        return "receipt-backed report";
      });
      response.writeHead(decision.status, { "content-type": "application/json", ...decision.headers })
        .end(decision.kind === "released" ? decision.resource : JSON.stringify(decision.body));
    } catch (error) {
      failures.push(error.message);
      response.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  resource.listen(0, "127.0.0.1");
  await once(resource, "listening");
  t.after(() => new Promise((resolve) => { resource.closeAllConnections(); resource.close(resolve); }));
  const resourceUrl = `http://127.0.0.1:${resource.address().port}/paid`;
  seller = new SellerMiddleware({ paymentRequired: { x402Version: 2, resource: { url: resourceUrl }, accepts: [requirements] },
    receiptKeys: [vector.key], authority: new ReceiptPayloadAuthority([vector.key]),
    fulfillments: new FileFulfillments(join(directory, "fulfillments")) });
  const environment = {
    BUYER_ROUTE_TOKEN: "buyer-route-token", BUYER_ROUTE_SOURCE: "acct:buyer-route", BUYER_ROUTE_KEY: "buyer-route-payment",
    BUYER_ROUTE_PINS: JSON.stringify([{ ...vector.pinnedKey }]),
  };
  const previous = Object.fromEntries(Object.keys(environment).map((name) => [name, process.env[name]]));
  Object.assign(process.env, environment);
  t.after(() => {
    for (const [name, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[name]; else process.env[name] = value;
    }
  });
  const beta = JSON.parse(await readFile(exampleConfig, "utf8")).environments.beta;
  assert.equal(buyerHumanUrl({ ...beta, name: "beta", humanUrlEnvironment: undefined }), new URL(beta.humanUrl).origin);
  assert.notEqual(new URL(beta.humanUrl).origin, new URL(beta.rpcUrl).origin);
  const result = await runBuyerAgent({ name: "emulator", network: "eip155:125", scheme: "exact",
    humanUrl: `http://127.0.0.1:${human.address().port}`, rpcUrl: node.rpcUrl, resourceUrl,
    tokenEnvironment: "BUYER_ROUTE_TOKEN", sourceEnvironment: "BUYER_ROUTE_SOURCE",
    idempotencyEnvironment: "BUYER_ROUTE_KEY", receiptKeysEnvironment: "BUYER_ROUTE_PINS" });
  assert.deepEqual(failures, []);
  assert.deepEqual(humanRequests.map(({ method, path }) => ({ method, path })), [
    { method: "POST", path: "/v1/moves/quote" }, { method: "POST", path: "/v1/moves" },
    { method: "GET", path: `/v1/evidence/${evidence.result.evidence_id}` },
  ]);
  assert.equal(result.state, "paid", JSON.stringify(result));
  assert.equal(result.transactionHash, vector.txHash);
  assert.equal(result.receiptDigest, vector.receiptDigest);
  assert.equal(result.rung, "receipted");
  assert.equal(result.body, "receipt-backed report");
  assert.equal(releases, 1);
  assert.equal(resourceRequests, 2);
}

test(buyerAgentUsesHostedHumanPaymentRoute);
