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
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
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

const { verificationLabel } = await import("../src/journeys/approvals/model.ts");
const { copyEntry } = await import("../copy/runtime.ts");

function approval_verification_label_for_rungs() {
  for (const [level, copyKey] of [
    ["unverified", "verification.unverified"],
    ["receipt-verified", "verification.receipt_verified"],
    ["checkpoint-finalised", "verification.checkpoint_finalised"],
  ] as const) {
    assert.equal(verificationLabel(level), copyEntry(copyKey).message);
  }
}

function approval_verification_label_for_retired_level() {
  for (const level of ["paxeer-finalised", "settlement-anchored"]) {
    assert.equal(verificationLabel(level), copyEntry("verification.unverified").message);
  }
}

test(approval_verification_label_for_rungs);
test(approval_verification_label_for_retired_level);
