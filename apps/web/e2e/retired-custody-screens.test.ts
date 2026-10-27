import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const loadedSources = new Set<string>();

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
    if (specifier === "server-only")
      return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation")
      return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      loadedSources.add(url);
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
    return loaded.format === "commonjs"
      ? { ...loaded, source: undefined }
      : loaded;
  },
});

async function custody_screens_do_not_resolve() {
  for (const filename of [
    "index.ts",
    "journey-screen.tsx",
    "timeline.tsx",
    "recovery.ts",
  ]) {
    const source = new URL(
      `../src/journeys/custody/${filename}`,
      import.meta.url,
    );
    await assert.rejects(
      () => import(source.href),
      { code: "ERR_MODULE_NOT_FOUND", url: source.href },
      `custody/${filename} must no longer resolve`,
    );
  }
}

async function wallet_api_imports_no_custody_module() {
  assert.equal(typeof window, "undefined");
  const wallet = await import("../src/api/wallet.ts");
  const exchange = await import("../src/app/exchange/exchange-client.tsx");
  const launchpad = await import("../src/app/launchpad/launchpad-client.tsx");
  const provider = await import("../src/api/provider.ts");

  assert.equal(typeof wallet.connectWalletAccount, "function");
  assert.equal(typeof exchange.ExchangeActions, "function");
  assert.equal(typeof launchpad.LaunchpadActions, "function");
  assert.equal(provider.windowWalletProvider(), undefined);
  assert.equal(await wallet.connectWalletAccount(), undefined);
  assert.ok(
    loadedSources.has(new URL("../src/api/provider.ts", import.meta.url).href),
  );

  const custodyDirectory = new URL("../src/journeys/custody/", import.meta.url)
    .href;
  assert.deepEqual(
    [...loadedSources].filter((url) => url.startsWith(custodyDirectory)),
    [],
    "wallet, exchange and launchpad must load without any custody module",
  );
}

test(custody_screens_do_not_resolve);
test(wallet_api_imports_no_custody_module);
