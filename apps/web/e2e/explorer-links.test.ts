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

const { EXPLORER_LINK_KINDS, explorerLinkPath } = await import("../src/explorer/links.ts");

function explorer_link_kinds_are_receipt_transaction_address() {
  assert.deepEqual(EXPLORER_LINK_KINDS, ["receipt", "transaction", "address"]);
}

function explorer_link_path_refuses_anchor_kind() {
  assert.throws(
    // @ts-expect-error Retired kinds must also be refused at runtime.
    () => explorerLinkPath({ kind: "anchor" }),
    TypeError,
  );
}

test(explorer_link_kinds_are_receipt_transaction_address);
test(explorer_link_path_refuses_anchor_kind);
