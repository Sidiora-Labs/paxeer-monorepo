import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createServer as createTlsServer } from "node:https";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.endsWith(".ts") && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        }).outputText,
      };
    }
    return nextLoad(url, context);
  },
});

const gateway = await import("../src/api/gateway.ts");
const { activityHistory, activitySide } = await import("../src/app/_markets/activity-history.ts");
const root = fileURLToPath(new URL("../../../", import.meta.url));
const walletScript = "scripts/wallet/check-live.sh";
const gasScript = "docker/gas-station/init.sh";
const recording = JSON.parse(readFileSync(new URL("./fixtures/repair-22-34-6-node-root.json", import.meta.url), "utf8"));

// The committed indexer/gateway producer recording used for node parity in
// rpc/px_node_root_history_test.go. Preserve amounts, positions and finality.
const history = JSON.parse(readFileSync(new URL("../../../rpc/testdata/px-history/gateway-recording.json", import.meta.url), "utf8"));
const address = history.account.evm_address;
const did = history.account.layerx_did;
const plainHistory = {
  ...history,
  account: address,
  items: history.items.map(({ side: _side, ...row }) => row),
};
const defaultNode = "https://api1.mainnet-beta.paxeer.network/";

function recorded(method) {
  const exchange = recording.exchanges.find((entry) => entry.request.method === method);
  assert.ok(exchange, `missing recorded ${method}`);
  return exchange.response;
}

async function fixture(context, replies, tls = false, path = "/") {
  const calls = [];
  const unexpected = [];
  const handler = async (request, response) => {
    try {
      assert.equal(request.method, "POST");
      assert.equal(request.url, path);
      assert.equal(request.headers["content-type"], "application/json");
      let body = "";
      for await (const chunk of request) body += chunk;
      const envelope = JSON.parse(body);
      assert.equal(envelope.jsonrpc, "2.0");
      assert.ok(Array.isArray(envelope.params));
      calls.push({ method: envelope.method, params: envelope.params, path: request.url });
      const reply = replies[envelope.method];
      assert.ok(reply, `unrecorded method ${envelope.method}`);
      response.setHeader("content-type", "application/json");
      response.end(JSON.stringify({ ...reply, id: envelope.id }));
    } catch (error) {
      unexpected.push(String(error));
      response.writeHead(500).end();
    }
  };
  const server = tls ? createTlsServer({
    cert: readFileSync(new URL("../e2e/fixtures/localhost-cert.pem", import.meta.url)),
    key: readFileSync(new URL("../e2e/fixtures/localhost-key.pem", import.meta.url)),
  }, handler) : createServer(handler);
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  context.after(async () => {
    await new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
    assert.deepEqual(unexpected, []);
  });
  return { url: `${tls ? "https" : "http"}://127.0.0.1:${server.address().port}${path}`, calls };
}

function environment(context, values) {
  const previous = Object.fromEntries(Object.keys(values).map((key) => [key, process.env[key]]));
  for (const [key, value] of Object.entries(values)) {
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
  context.after(() => {
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  });
}

function shell(args, values = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn("bash", args, {
      cwd: root,
      env: { ...process.env, ...values },
      stdio: ["ignore", "pipe", "pipe"],
      timeout: 10_000,
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => { stdout += chunk; });
    child.stderr.on("data", (chunk) => { stderr += chunk; });
    child.once("error", reject);
    child.once("close", (code, signal) => {
      if (signal) reject(new Error(`guarded shell terminated by ${signal}: ${stderr}`));
      else resolve({ code, stdout, stderr });
    });
  });
}

async function nodeReadsAndMarketActivityUseTheNodeRoot(context) {
  assert.equal(recorded("eth_chainId").result, "0x7d");
  assert.equal(gateway.DEFAULT_NODE_ENDPOINT, defaultNode);
  const node = await fixture(context, {
    px_resolveAccount: { jsonrpc: "2.0", result: history.account },
    px_getHistory: { jsonrpc: "2.0", result: plainHistory },
    px_getUnifiedHistory: { jsonrpc: "2.0", result: history },
    px_getCapabilities: recorded("px_getCapabilities"),
  });
  const hosted = await fixture(context, { eth_blockNumber: { jsonrpc: "2.0", result: "0xc" } }, false, "/rpc");
  environment(context, { LAYERX_NODE_ENDPOINT: node.url, LAYERX_GATEWAY_URL: hosted.url });

  assert.deepEqual(await gateway.pxResolveAccount(did), {
    evmAddress: address,
    layerxDid: did,
    layerxAccount: history.account.layerx_account,
    bound: true,
  });
  const plain = await gateway.paxeerHistory(` ${address.toUpperCase()} `);
  const unified = await gateway.unifiedHistory(did);
  assert.equal(plain.items[0].side, null);
  assert.equal(unified.items[0].side, "paxeer");
  for (const page of [plain, unified]) {
    assert.equal(page.items.length, 4);
    assert.equal(page.items[0].amount, 1_000_000_000_000_000_000n);
    assert.deepEqual(page.items.map((row) => row.id), ["3", "6", "9", "11"]);
    assert.deepEqual(page.items.map((row) => row.final), [false, false, false, false]);
    assert.equal(page.indexedHeight, 12);
    assert.equal(page.chainHeight, 12);
    assert.equal(page.nextCursor, null);
    assert.deepEqual(page.items.map((row) => [row.blockHeight, row.txIndex, row.logIndex]),
      [[10, 0, 0], [11, 0, 0], [12, 0, 0], [12, 1, 0]]);
  }
  assert.deepEqual(await activityHistory("paxeer", address, {}), plain);
  assert.deepEqual(await activityHistory("unified", did, {}), unified);
  assert.equal(activitySide(undefined), "unified");
  assert.equal(activitySide("paxeer"), "paxeer");
  await assert.rejects(gateway.pxGetCapabilities(), {
    name: "GatewayRpcError",
    code: recorded("px_getCapabilities").error.code,
    message: recorded("px_getCapabilities").error.message,
  });
  assert.deepEqual(hosted.calls, [], "served px reads, including refusals, must never call the gateway");
  assert.deepEqual(node.calls, [
    { method: "px_resolveAccount", params: [did], path: "/" },
    { method: "px_getHistory", params: [address, null, 25, null], path: "/" },
    { method: "px_getUnifiedHistory", params: [did, null, 25, null], path: "/" },
    { method: "px_getHistory", params: [address, null, 25, null], path: "/" },
    { method: "px_getUnifiedHistory", params: [did, null, 25, null], path: "/" },
    { method: "px_getCapabilities", params: [], path: "/" },
  ]);
  assert.equal(await gateway.ethBlockNumber(), 12n);
  assert.deepEqual(hosted.calls, [{ method: "eth_blockNumber", params: [], path: "/rpc" }]);
}

async function nodeConfigurationAndReadErrorsNeverFallBack(context) {
  const node = await fixture(context, {
    px_resolveAccount: recorded("px_resolveAccount"),
    px_getCapabilities: recorded("px_getCapabilities"),
  });
  const hosted = await fixture(context, {});
  environment(context, { LAYERX_NODE_ENDPOINT: node.url, LAYERX_GATEWAY_URL: hosted.url });
  const selector = recording.exchanges.find((entry) => entry.request.method === "px_resolveAccount").request.params[0];
  await assert.rejects(gateway.pxResolveAccount(selector), {
    name: "GatewayRpcError", code: -32601, message: recorded("px_resolveAccount").error.message,
  });
  for (const configured of ["", "bad-url", "http://api1.mainnet-beta.paxeer.network/", `${node.url}rpc`, `${node.url}evm`, `${node.url}?a=1`, `${node.url}#x`, node.url.replace("://", "://user:pass@")]) {
    process.env.LAYERX_NODE_ENDPOINT = configured;
    await assert.rejects(gateway.pxGetCapabilities(), gateway.GatewayUnavailableError);
  }
  assert.equal(node.calls.length, 1, "invalid node configuration must fail before transport");
  assert.deepEqual(hosted.calls, []);
  process.env.LAYERX_NODE_ENDPOINT = node.url;
  delete process.env.LAYERX_GATEWAY_URL;
  await assert.rejects(gateway.pxGetCapabilities(), gateway.GatewayRpcError);
  assert.equal(node.calls.length, 2, "node reads do not require a hosted URL");
  await assert.rejects(gateway.ethBlockNumber(), /LAYERX_GATEWAY_URL is not set/u);
}

async function walletProbesRecordedNodeAnswersAndRefusesRouteSuffixes(context) {
  const node = await fixture(context, Object.fromEntries(recording.exchanges.map((entry) => [entry.request.method, entry.response])));
  const hosted = await fixture(context, {});
  const values = { CHECK_LIVE_NODE_RPC_URL: node.url, CHECK_LIVE_HOSTED_RPC_URL: hosted.url, CHECK_LIVE_TIMEOUT: "2" };
  const result = await shell([walletScript, "endpoint"], values);
  assert.equal(result.code, 1, result.stderr);
  assert.match(result.stdout, /pass eth_chainId owner=node .* 0x7d/u);
  assert.match(result.stdout, /check-live: 3 check\(s\) failed/u);
  for (const method of ["px_resolveAccount", "px_getNetwork", "px_getAccount"]) {
    assert.match(result.stdout, new RegExp(`fail ${method} owner=node `, "u"));
  }
  assert.deepEqual(node.calls.map(({ method, path }) => [method, path]),
    ["eth_chainId", "px_resolveAccount", "px_getNetwork", "px_getAccount"].map((method) => [method, "/"]));
  assert.deepEqual(hosted.calls, []);
  for (const suffix of ["rpc", "evm", "?token=secret", "#fragment"]) {
    const refused = await shell([walletScript, "endpoint"], { ...values, CHECK_LIVE_NODE_RPC_URL: node.url + suffix });
    assert.equal(refused.code, 2);
    assert.match(refused.stderr, /node root without a route suffix/u);
  }
  assert.equal(node.calls.length, 4);
}

function gasEnvironment() {
  const source = readFileSync(new URL("../../../docker/gas-station/init.sh", import.meta.url), "utf8");
  const values = Object.fromEntries([...source.matchAll(/^need (GAS_STATION_[A-Z_]+) /gmu)].map((match) => [match[1], "1"]));
  Object.assign(values, {
    GAS_STATION_LISTEN: "[::]:8080",
    GAS_STATION_CHAIN_ID: "125",
    GAS_STATION_PAYMASTER: "0x0000000000000000000000000000000000000001",
    GAS_STATION_TOKEN: "0x0000000000000000000000000000000000000002",
    GAS_STATION_RELAYER_KEY_ENV: "REPAIR_SPONSOR_KEY",
    GAS_STATION_RATE_OWNER_KEY_ENV: "REPAIR_OWNER_KEY",
    REPAIR_SPONSOR_KEY: "11".repeat(32),
    REPAIR_OWNER_KEY: "22".repeat(32),
  });
  return values;
}

// A DEBUG guard stops the real init before any volume write, chown or process
// launch. The readiness call still uses real curl, TLS and recorded node data.
const gasGuard = `
set -T
trap 'case "$BASH_COMMAND" in mkdir*|mv*|chmod*|chown*|setpriv*|"env "*) echo unexpected-mutation >&2; exit 90;; esac' DEBUG
source "$1"
`;

async function gasBringUpProbesNodeRootAndFailsClosed(context) {
  const node = await fixture(context, { px_getNetwork: recorded("px_getNetwork") }, true);
  const result = await shell(["-c", gasGuard, "gas-guard", gasScript], {
    ...gasEnvironment(),
    GAS_STATION_ENDPOINTS: node.url,
    CURL_CA_BUNDLE: fileURLToPath(new URL("../e2e/fixtures/localhost-cert.pem", import.meta.url)),
  });
  assert.equal(result.code, 1, result.stderr);
  assert.match(result.stderr, /node-root px_getNetwork is unavailable or is not chain 125/u);
  assert.doesNotMatch(result.stderr, /unexpected-mutation/u);
  assert.deepEqual(node.calls, [{ method: "px_getNetwork", params: [], path: "/" }]);
  for (const suffix of ["rpc", "evm", "?token=secret", "#fragment"]) {
    const refused = await shell(["-c", gasGuard, "gas-guard", gasScript], {
      ...gasEnvironment(), GAS_STATION_ENDPOINTS: node.url + suffix,
    });
    assert.equal(refused.code, 2);
    assert.match(refused.stderr, /GAS_STATION_ENDPOINTS is unset or malformed/u);
  }
  assert.equal(node.calls.length, 1);
}

async function scriptsHaveValidSyntaxAndGuardedNodeDefaults() {
  for (const path of [walletScript, gasScript]) {
    const syntax = await shell(["-n", path]);
    assert.equal(syntax.code, 0, syntax.stderr);
  }
  const walletDefault = await shell(["-c", `
unset CHECK_LIVE_NODE_RPC_URL CHECK_LIVE_HOSTED_RPC_URL
set -T
trap 'case "$BASH_COMMAND" in "run node"*) printf "guarded-node=%s\\n" "$node_rpc_url"; exit 73;; esac' DEBUG
source "$1" endpoint
`, "wallet-default-guard", walletScript]);
  assert.equal(walletDefault.code, 73, walletDefault.stderr);
  assert.equal(walletDefault.stdout.trim(), `guarded-node=${defaultNode}`);

  const gasDefault = await shell(["-c", `
unset GAS_STATION_ENDPOINTS
set -T
trap 'case "$BASH_COMMAND" in answer=*) printf "guarded-node=%s\\n" "$url"; exit 73;; mkdir*|mv*|chmod*|chown*) exit 90;; esac' DEBUG
source "$1"
`, "gas-default-guard", gasScript], gasEnvironment());
  assert.equal(gasDefault.code, 73, gasDefault.stderr);
  assert.equal(gasDefault.stdout.trim(), `guarded-node=${defaultNode}`);
}

test(nodeReadsAndMarketActivityUseTheNodeRoot);
test(nodeConfigurationAndReadErrorsNeverFallBack);
test(walletProbesRecordedNodeAnswersAndRefusesRouteSuffixes);
test(gasBringUpProbesNodeRootAndFailsClosed);
test(scriptsHaveValidSyntaxAndGuardedNodeDefaults);
