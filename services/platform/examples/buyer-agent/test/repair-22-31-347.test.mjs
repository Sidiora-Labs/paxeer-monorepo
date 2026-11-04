import { readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

// Load the production middleware in-process: the declared gate builds only the
// SDK. Resolve its TypeScript sources without writing compiled artifacts.
const middleware = new URL("../../../middleware/", import.meta.url);
const sources = new Map([
  ["@sidiora/layerx-buyer-middleware", new URL("buyer/src/index.ts", middleware).href],
  ["@sidiora/layerx-seller-middleware", new URL("seller/src/index.ts", middleware).href],
]);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (sources.has(specifier)) {
      return { url: sources.get(specifier), shortCircuit: true };
    }
    if (context.parentURL?.startsWith(middleware.href)
      && specifier.startsWith("./") && specifier.endsWith(".js")) {
      return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(middleware.href) && url.endsWith(".ts")) {
      return {
        format: "module",
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }),
        shortCircuit: true,
      };
    }
    return nextLoad(url, context);
  },
});

await import("../repair.test.mjs");
