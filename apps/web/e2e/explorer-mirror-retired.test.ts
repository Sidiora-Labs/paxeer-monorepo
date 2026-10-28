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
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
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

async function explorer_mirror_modules_do_not_resolve() {
  for (const path of [
    "../src/explorer/mirror-server.ts",
    "../src/explorer/mirror-admission.ts",
    "../src/app/api/explorer/verify/route.ts",
  ]) {
    const source = new URL(path, import.meta.url);
    await assert.rejects(
      () => import(source.href),
      { code: "ERR_MODULE_NOT_FOUND", url: source.href },
      `${path} must no longer resolve`,
    );
  }
}

async function explorer_client_exports_no_verify_upstream() {
  const client = await import("../src/explorer/client.ts");
  assert.equal("verifyEvidenceUpstream" in client, false);
}

test(explorer_mirror_modules_do_not_resolve);
test(explorer_client_exports_no_verify_upstream);
