import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { checkDependencyPolicy, runWebPolicy } from "../policy/check.mjs";

// Load the production TypeScript sources without depending on generated dist files.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@sidiora/layerx-sdk/browser") {
      return nextResolve(new URL("../../../services/agent/sdk/typescript/src/browser.ts", import.meta.url).href, context);
    }
    if (specifier.startsWith(".") && context.parentURL && specifier.endsWith(".js")) {
      const source = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
      if (existsSync(source)) return nextResolve(source.href, context);
    }
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
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { exchange } = await import("../../../sdk/wallet/src/modules/exchange.ts");
const { PaxeerProvider } = await import("../../../sdk/wallet/src/provider.ts");
const { ModuleError } = await import("../../../sdk/wallet/src/modules/index.ts");

function readJson(path) {
  return JSON.parse(readFileSync(new URL(path, import.meta.url), "utf8"));
}

function policyInputs() {
  return {
    manifest: readJson("../package.json"),
    lockfile: readJson("../package-lock.json"),
    budget: readJson("../policy/runtime-dependencies.json"),
    localManifests: {
      "@layerx/ui": readJson("../packages/layerx-ui/package.json"),
      "@sidiora/layerx-sdk": readJson("../../../services/agent/sdk/typescript/package.json"),
      "@paxeer/wallet": readJson("../../../sdk/wallet/package.json"),
    },
  };
}

function rejectsChange(change, message) {
  const inputs = policyInputs();
  change(inputs);
  assert.throws(() => checkDependencyPolicy(inputs), message);
}

async function walletExchangeWalkthrough() {
  const owner = `0x${"11".repeat(20)}`;
  const token = `0x${"22".repeat(20)}`;
  const venue = "0x0000000000000000000000000000000000001015";
  const allowanceData = `0xdd62ed3e${owner.slice(2).padStart(64, "0")}${venue.slice(2).padStart(64, "0")}`;
  const approvalData = `0x095ea7b3${venue.slice(2).padStart(64, "0")}${"11".padStart(64, "0")}`;
  const replies = [`0x${"0".repeat(64)}`, `0x${"11".padStart(64, "0")}`, "0xzz"];
  const requests = [];
  const responses = [];
  const server = createServer(async (request, response) => {
    try {
      let body = "";
      for await (const chunk of request) body += chunk;
      const envelope = JSON.parse(body);
      requests.push({ method: request.method, path: request.url, contentType: request.headers["content-type"], body: envelope });
      const answer = { jsonrpc: "2.0", id: envelope.id, result: replies[requests.length - 1] };
      responses.push(answer);
      response.writeHead(200, { "content-type": "application/json" });
      response.end(JSON.stringify(answer));
    } catch (error) {
      response.writeHead(500);
      response.end(String(error));
    }
  });
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  try {
    const origin = `http://127.0.0.1:${server.address().port}`;
    const provider = new PaxeerProvider({ gatewayUrl: origin, rpcUrl: origin, token: () => null });
    const market = exchange(provider);
    assert.equal(market.address, venue);
    assert.deepEqual(await market.approval(owner, token, 17n), { to: token, data: approvalData, value: 0n });
    assert.equal(await market.approval(owner, token, 17n), undefined);
    await assert.rejects(market.approval(owner, token, 17n), (error) => {
      assert.ok(error instanceof ModuleError);
      assert.equal(error.code, "invalid_answer");
      assert.equal(error.field, "eth_call");
      return true;
    });
    assert.deepEqual(requests, [1, 2, 3].map((id) => ({
      method: "POST",
      path: "/",
      contentType: "application/json",
      body: { jsonrpc: "2.0", id, method: "eth_call", params: [{ to: token, data: allowanceData }, "latest"] },
    })));
    assert.deepEqual(responses, replies.map((result, index) => ({ jsonrpc: "2.0", id: index + 1, result })));
  } finally {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

async function webLocalLinkPolicy() {
  assert.match(await runWebPolicy(), /policies passed$/u);
  const inputs = policyInputs();
  for (const name of ["@layerx/ui", "@sidiora/layerx-sdk", "@paxeer/wallet"]) {
    const target = inputs.lockfile.packages[`node_modules/${name}`].resolved;
    rejectsChange(({ lockfile }) => { lockfile.packages[`node_modules/${name}`].resolved += "/other"; }, /must resolve/u);
    rejectsChange(({ manifest, lockfile, budget }) => {
      manifest.dependencies[name] = "file:../../other";
      budget.approved[name] = manifest.dependencies[name];
      lockfile.packages[""].dependencies[name] = manifest.dependencies[name];
    }, /manifest path changed/u);
    for (const field of ["name", "version", "license"]) {
      rejectsChange(({ lockfile }) => { lockfile.packages[target][field] = "unapproved"; }, /identity changed|version changed|license disposition changed/u);
      rejectsChange(({ localManifests }) => { localManifests[name][field] = "unapproved"; }, /identity changed|version changed|license disposition changed/u);
    }
    rejectsChange(({ lockfile }) => { delete lockfile.packages[`node_modules/${name}`]; }, /must remain/u);
    rejectsChange(({ lockfile }) => { delete lockfile.packages[target]; }, /metadata is missing/u);
    rejectsChange(({ lockfile }) => { lockfile.packages[target].link = true; }, /metadata is missing or linked/u);
    rejectsChange(({ lockfile }) => { lockfile.packages[target].dependencies = {}; }, /lockfile dependencies changed/u);
  }
  rejectsChange(({ lockfile }) => {
    lockfile.packages["node_modules/@unknown/package"] = { link: true, resolved: "packages/layerx-ui" };
  }, /unapproved local package link/u);
  rejectsChange(({ lockfile }) => {
    lockfile.packages["node_modules/nested/node_modules/@paxeer/wallet"] = { link: true, resolved: "../../sdk/wallet" };
  }, /unapproved local package link/u);
  rejectsChange(({ lockfile }) => {
    lockfile.packages["../unapproved"] = { name: "other", version: "1.0.0", license: "MIT" };
  }, /unapproved local package target/u);
  const nestedPackage = "packages/layerx-ui/node_modules/cac";
  assert.equal(inputs.lockfile.packages[nestedPackage].version, "6.7.14");
  assert.equal(inputs.lockfile.packages[nestedPackage].license, "MIT");
  for (const target of ["packages/layerx-ui", "../../sdk/wallet", "../../services/agent/sdk/typescript"]) {
    rejectsChange(({ lockfile }) => {
      lockfile.packages[`${target}/node_modules/unapproved-link`] = {
        link: true, resolved: "packages/layerx-ui",
      };
    }, /unapproved local package link/u);
  }
  for (const suffix of ["../escape", "cac/../../../escape", "@scope", "cac/other", "cac//node_modules/other"]) {
    rejectsChange(({ lockfile }) => {
      lockfile.packages[`packages/layerx-ui/node_modules/${suffix}`] = lockfile.packages[nestedPackage];
    }, /canonical installed package path/u);
  }
  rejectsChange(({ lockfile }) => {
    lockfile.packages[nestedPackage].license = "UNLICENSED";
  }, /non-allowlisted license/u);
  rejectsChange(({ lockfile }) => {
    delete lockfile.packages[nestedPackage].version;
  }, /not pinned to a version/u);
  rejectsChange(({ localManifests }) => { localManifests["@paxeer/wallet"].private = false; }, /requires a private package/u);
  assert.equal(inputs.localManifests["@paxeer/wallet"].license, "UNLICENSED");
  await walletExchangeWalkthrough();
}

test(webLocalLinkPolicy);

async function webDependencyBudget() {
  const inputs = policyInputs();
  checkDependencyPolicy(inputs);
  assert.equal(inputs.budget.maximumDirectRuntimeDependencies, 7);
  assert.equal(Object.keys(inputs.manifest.dependencies).length, 7);
  assert.equal(inputs.manifest.dependencies.next, "16.4.1");
  for (const name of ["@noble/curves", "@noble/hashes", "@supabase/supabase-js"]) {
    assert.equal(Object.hasOwn(inputs.manifest.dependencies, name), false);
    assert.equal(typeof inputs.localManifests["@paxeer/wallet"].dependencies[name], "string");
  }
  assert.equal(Object.hasOwn(inputs.manifest.dependencies, "viem"), false);
  assert.equal(inputs.manifest.devDependencies.viem, "2.56.9");
  assert.equal(typeof inputs.localManifests["@paxeer/wallet"].dependencies.viem, "string");
  rejectsChange(({ manifest }) => { manifest.dependencies.next = "16.3.5"; }, /without dependency-budget approval/u);
  rejectsChange(({ lockfile }) => { lockfile.packages[""].dependencies.next = "16.3.5"; }, /does not pin the declared runtime/u);
  rejectsChange(({ lockfile }) => { delete lockfile.packages[""].devDependencies.viem; }, /does not pin the declared development/u);
  rejectsChange(({ lockfile }) => { lockfile.lockfileVersion = 2; }, /lockfile version 3/u);
  rejectsChange(({ budget }) => { budget.maximumDirectRuntimeDependencies = 6; }, /budget was exceeded/u);
  for (const limit of [Infinity, NaN, 7.5, 0, -1, "7"]) {
    rejectsChange(({ budget }) => { budget.maximumDirectRuntimeDependencies = limit; }, /positive finite integer/u);
  }
  rejectsChange(({ manifest, lockfile, budget }) => {
    manifest.dependencies.unreviewed = "1.0.0";
    budget.approved.unreviewed = "1.0.0";
    lockfile.packages[""].dependencies.unreviewed = "1.0.0";
  }, /budget was exceeded/u);
  for (const license of ["UNLICENSED", "LicenseRef-Unknown", undefined]) {
    rejectsChange(({ lockfile }) => {
      lockfile.packages["node_modules/unapproved-license"] = { name: "@paxeer/wallet", version: "0.1.0", license };
    }, /non-allowlisted license/u);
  }
  rejectsChange(({ lockfile }) => {
    lockfile.packages["node_modules/unpinned"] = { license: "MIT" };
  }, /not pinned to a version/u);
  await walletExchangeWalkthrough();
}

test(webDependencyBudget);
