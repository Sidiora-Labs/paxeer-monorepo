import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { registerHooks } from "node:module";
import { join } from "node:path";
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

async function custody_journey_module_does_not_resolve() {
  for (const filename of ["handoff.ts", "model.ts", "time.ts", "evidence.ts", "copy.ts"]) {
    const source = new URL(`../src/journeys/custody/${filename}`, import.meta.url);
    await assert.rejects(
      () => import(source.href),
      { code: "ERR_MODULE_NOT_FOUND", url: source.href },
      `custody/${filename} must no longer resolve`,
    );
  }
}

function web_source_reads_no_human_settlement_variable() {
  const sourceRoot = new URL("../src/", import.meta.url);
  const variables = [
    "HUMAN_SETTLEMENT_CHECKPOINT_INTERVAL_SECONDS",
    "HUMAN_SETTLEMENT_PAXEER_BLOCK_SECONDS",
    "HUMAN_SETTLEMENT_REQUIRED_CONFIRMATIONS",
    "HUMAN_CHALLENGE_WINDOW_SECONDS",
    "HUMAN_DEPOSIT_DELAYED_AFTER_SECONDS",
  ];
  const files = readdirSync(sourceRoot, { recursive: true, withFileTypes: true })
    .filter((entry) => entry.isFile());
  assert.ok(files.length > 0, "The web source scan must inspect files");
  for (const entry of files) {
    const filename = join(entry.parentPath, entry.name);
    const source = readFileSync(filename, "utf8");
    for (const variable of variables) {
      assert.ok(!source.includes(variable), `${filename} must not read ${variable}`);
    }
  }
}

test(custody_journey_module_does_not_resolve);
test(web_source_reads_no_human_settlement_variable);
