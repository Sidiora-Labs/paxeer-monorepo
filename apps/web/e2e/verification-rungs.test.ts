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
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { VERIFICATION_PRESENTATIONS, verificationRung, verificationWord } = await import("../src/kit/model.ts");
const { copyEntry } = await import("../copy/runtime.ts");

function verification_rung_keeps_three_rungs() {
  for (const [level, copyKey] of [
    ["unverified", "verification.unverified"],
    ["receipt-verified", "verification.receipt_verified"],
    ["checkpoint-finalised", "verification.checkpoint_finalised"],
  ] as const) {
    assert.equal(verificationRung(level), level);
    assert.equal(VERIFICATION_PRESENTATIONS[level].copyKey, copyKey);
    assert.equal(verificationWord(level), copyEntry(copyKey).message);
  }
}

function verification_rung_fails_retired_levels_closed() {
  assert.deepEqual(Object.keys(VERIFICATION_PRESENTATIONS), [
    "unverified", "receipt-verified", "checkpoint-finalised",
  ]);
  for (const level of [
    "paxeer-finalised", "settlement-anchored", "unknown-level", "",
    "__proto__", "constructor", "toString", "RECEIPT-VERIFIED",
  ]) {
    assert.equal(verificationRung(level), "unverified");
    assert.equal(verificationWord(level), copyEntry("verification.unverified").message);
  }
}

test(verification_rung_keeps_three_rungs);
test(verification_rung_fails_retired_levels_closed);
