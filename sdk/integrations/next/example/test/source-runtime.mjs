import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import { fileURLToPath } from "node:url";
import ts from "typescript";

const root = new URL("../../../../../", import.meta.url);
const packages = new Map([
  ["@sidiora/layerx-sdk", "services/agent/sdk/typescript/src/index.ts"],
  ["@sidiora/layerx-seller-middleware", "services/platform/middleware/seller/src/index.ts"],
  ["@sidiora/layerx-buyer-middleware", "services/platform/middleware/buyer/src/index.ts"],
  ["@sidiora/layerx-next", "sdk/integrations/next/src/index.ts"],
]);
const sourceRoots = [...packages.values()].map(path => new URL("./", new URL(path, root)).href);

registerHooks({
  resolve(specifier, context, nextResolve) {
    const source = packages.get(specifier);
    if (source !== undefined) return nextResolve(new URL(source, root).href, context);
    if (specifier.startsWith(".") && specifier.endsWith(".js")
      && sourceRoots.some(directory => context.parentURL?.startsWith(directory))) {
      const candidate = new URL(specifier.replace(/\.js$/u, ".ts"), context.parentURL);
      if (existsSync(candidate)) return nextResolve(candidate.href, context);
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.endsWith(".ts") && sourceRoots.some(directory => url.startsWith(directory))) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        }).outputText,
      };
    }
    return nextLoad(url, context);
  },
});
