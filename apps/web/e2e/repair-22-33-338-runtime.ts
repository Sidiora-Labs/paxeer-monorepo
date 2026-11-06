import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import { fileURLToPath } from "node:url";
import ts from "typescript";

// Node's server-side proof executes the repository's sources, including JSX.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") return nextResolve("next/dist/compiled/server-only/empty.js", context);
    if (["next/navigation", "next/link", "next/server"].includes(specifier)) return nextResolve(`${specifier}.js`, context);
    if (specifier === "@paxeer/wallet" || specifier === "@paxeer/wallet/provider") {
      return nextResolve(new URL(`../../../sdk/wallet/src/${specifier.endsWith("/provider") ? "provider" : "index"}.ts`, import.meta.url).href, context);
    }
    if (specifier.startsWith("@layerx/ui/")) {
      const path = specifier === "@layerx/ui/cn" ? "lib/utils" : specifier.slice("@layerx/ui/".length);
      for (const suffix of [".ts", ".tsx"]) {
        const source = new URL(`../packages/layerx-ui/src/${path}${suffix}`, import.meta.url);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = new URL(candidate.href.endsWith(".js") ? candidate.href.slice(0, -3) + suffix : candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return { format: "module", shortCircuit: true, source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
        fileName: fileURLToPath(url),
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
      }).outputText };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});
