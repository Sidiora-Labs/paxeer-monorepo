import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { copyEntry } from "../copy/runtime.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
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

const { verificationLabel } = await import("../src/explorer/components.tsx");

function explorer_label_uses_kit_rung_words() {
  assert.equal(verificationLabel("receipt-verified"), copyEntry("verification.receipt_verified").message);
  assert.equal(verificationLabel("checkpoint-finalised"), copyEntry("verification.checkpoint_finalised").message);
  assert.equal(verificationLabel("unverified"), copyEntry("verification.unverified").message);
}

function explorer_label_for_retired_level_is_unverified() {
  assert.equal(verificationLabel("settlement-anchored"), copyEntry("verification.unverified").message);
  assert.equal(verificationLabel("state-proven"), copyEntry("verification.unverified").message);
}

test(explorer_label_uses_kit_rung_words);
test(explorer_label_for_retired_level_is_unverified);
