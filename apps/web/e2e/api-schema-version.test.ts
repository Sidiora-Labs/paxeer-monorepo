import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const api = await import("../src/api/generated/index.ts");
const { conformance } = await import("../src/api/generated/conformance.ts");

function generated_client_reports_schema_minor_10() {
  assert.deepEqual(api.schemaVersion, { major: 1, minor: 10 });
  for (const level of ["paxeer-finalised", "settlement-anchored"]) {
    assert.equal(api.decodeVerificationLevel(level, "stored verification"), level);
  }
  const binding = { state: "bound", address: `0x${"11".repeat(20)}` };
  assert.equal(api.decodeBindingState(binding.state, "stored binding state"), binding.state);
  assert.deepEqual(api.decodeWalletBinding(binding, "stored binding"), binding);
}

function generated_client_has_no_binding_operations() {
  const client = api.createHumanApiClient();
  assert.equal(typeof client.accountBalance, "function");
  for (const method of [
    "bindingSubmit",
    "bindingStatus",
    "bindingStatement",
    "bindingRebind",
    "bindingRebindAction",
    "depositStart",
    "depositConfirm",
    "withdrawStart",
    "withdrawClaim",
    "exitStart",
    "exitEligibility",
  ]) {
    assert.equal(method in client, false, `${method} must not be exposed by the client`);
  }
  for (const table of [api.operationNames, Object.keys(api.operations), Object.keys(conformance)]) {
    for (const name of table) {
      assert.doesNotMatch(name, /^(binding|deposit|withdraw|exit)\./u);
    }
  }
}

test(generated_client_reports_schema_minor_10);
test(generated_client_has_no_binding_operations);
