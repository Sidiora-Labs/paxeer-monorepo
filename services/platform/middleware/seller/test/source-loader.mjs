import { existsSync, readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

// Run production sources without a compiler subprocess or prebuilt seller bundle.
const root = new URL("../../../../../", import.meta.url);
const packages = new Map([
  ["@sidiora/layerx-sdk", "services/agent/sdk/typescript/src/index.ts"],
  ["@sidiora/layerx-seller-middleware", "services/platform/middleware/seller/src/index.ts"],
  ["@sidiora/layerx-buyer-middleware", "services/platform/middleware/buyer/src/index.ts"],
  ["@sidiora/layerx-merchant-middleware", "services/platform/middleware/merchant/src/index.ts"],
]);
const seller = new URL("../", import.meta.url);
registerHooks({
  resolve(specifier, context, next) {
    if (packages.has(specifier)) return { url: new URL(packages.get(specifier), root).href, shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL) {
      const requested = new URL(specifier, context.parentURL);
      if (requested.href.startsWith(new URL("dist/", seller).href) && requested.pathname.endsWith(".js")) {
        return { url: requested.href.replace("/seller/dist/", "/seller/src/").replace(/\.js$/u, ".ts"), shortCircuit: true };
      }
      if (context.parentURL.endsWith(".ts") && requested.pathname.endsWith(".js")) {
        const source = new URL(requested.href.replace(/\.js$/u, ".ts"));
        if (existsSync(source)) return { url: source.href, shortCircuit: true };
      }
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (!url.startsWith(root.href) || !url.endsWith(".ts")) return next(url, context);
    return { format: "module", shortCircuit: true,
      source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
  },
});
