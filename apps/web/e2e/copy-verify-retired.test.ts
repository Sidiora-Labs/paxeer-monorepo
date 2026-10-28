import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
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

const copyEntry = (await import("../copy/runtime.ts")).copyEntry;
const { copyEntries } = await import("../copy/catalog.ts");

function retired_verification_copy_keys_are_gone() {
  const retiredKeys = [
    "explorer.verification.unverified",
    "explorer.verification.sequencer_signed",
    "explorer.verification.batch_included",
    "explorer.verification.state_proven",
    "explorer.verification.checkpoint_finalised",
    "explorer.verification.settlement_anchored",
    "explorer.verify.title",
    "explorer.verify.body",
    "explorer.verify.form.title",
    "explorer.verify.kind.label",
    "explorer.verify.kind.receipt",
    "explorer.verify.kind.activity",
    "explorer.verify.kind.state",
    "explorer.verify.evidence.label",
    "explorer.verify.evidence.placeholder",
    "explorer.verify.evidence.required",
    "explorer.verify.action",
    "explorer.verify.refused",
    "explorer.verify.divergent",
    "explorer.verify.overloaded",
    "explorer.verify.overloaded.unknown_wait",
    "explorer.verify.retry",
    "explorer.verify.result.table",
    "explorer.verify.result.kind",
    "explorer.verify.result.header",
    "explorer.verify.result.root",
    "explorer.verify.result.source",
    "explorer.verify.result.target",
    "explorer.verify.result.freshness",
    "verification.paxeer_finalised",
    "activity.verification.paxeer_finalised",
  ];
  for (const key of retiredKeys) {
    assert.equal(copyEntries.some((entry) => entry.key === key), false, key);
    assert.throws(() => copyEntry(key), new Error(`Unknown copy key: ${key}`));
  }
  assert.deepEqual(
    copyEntries.filter(({ key }) => key.startsWith("explorer.verify.") || key.startsWith("explorer.verification."))
      .map(({ key }) => key),
    ["explorer.verify.unavailable"],
  );
  execFileSync(process.execPath, [fileURLToPath(new URL("../copy/generate-runtime.ts", import.meta.url)), "--check"], {
    stdio: "pipe",
  });
}

function explorer_verify_unavailable_key_is_kept() {
  assert.deepEqual(copyEntry("explorer.verify.unavailable"), {
    key: "explorer.verify.unavailable",
    message: "Verification is temporarily unavailable.",
  });
  assert.deepEqual(copyEntry("verification.receipt_verified"), {
    key: "verification.receipt_verified",
    message: "Verified against its receipt",
  });
}

test(retired_verification_copy_keys_are_gone);
test(explorer_verify_unavailable_key_is_kept);
