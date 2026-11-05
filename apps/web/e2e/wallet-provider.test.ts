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

const { windowWalletProvider } = await import("../src/api/provider.ts");

function window_wallet_provider_reads_window_ethereum() {
  const previous = Object.getOwnPropertyDescriptor(globalThis, "window");
  const ethereum = { request: async () => undefined };
  Object.defineProperty(globalThis, "window", { configurable: true, value: { ethereum } });
  try {
    assert.equal(windowWalletProvider(), ethereum);
  } finally {
    if (previous === undefined) Reflect.deleteProperty(globalThis, "window");
    else Object.defineProperty(globalThis, "window", previous);
  }
}

test(window_wallet_provider_reads_window_ethereum);

function window_wallet_provider_is_undefined_without_request() {
  const previous = Object.getOwnPropertyDescriptor(globalThis, "window");
  try {
    Reflect.deleteProperty(globalThis, "window");
    assert.equal(windowWalletProvider(), undefined);
    for (const value of [
      {},
      { ethereum: undefined },
      { ethereum: null },
      { ethereum: {} },
      { ethereum: { request: undefined } },
      { ethereum: { request: null } },
      { ethereum: { request: "request" } },
      { ethereum: { request: 1 } },
    ]) {
      Object.defineProperty(globalThis, "window", { configurable: true, value });
      assert.equal(windowWalletProvider(), undefined);
    }
  } finally {
    if (previous === undefined) Reflect.deleteProperty(globalThis, "window");
    else Object.defineProperty(globalThis, "window", previous);
  }
}

test(window_wallet_provider_is_undefined_without_request);
