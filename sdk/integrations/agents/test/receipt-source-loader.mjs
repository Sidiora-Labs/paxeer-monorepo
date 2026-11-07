import { existsSync, readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

const root = new URL("../../../../", import.meta.url);
const packages = new Map([
  ["@sidiora/layerx-sdk", "services/agent/sdk/typescript/src/index.ts"],
  ["@sidiora/layerx-agent-middleware", "services/platform/middleware/agent/src/index.ts"],
  ["@sidiora/layerx-seller-middleware", "services/platform/middleware/seller/src/index.ts"],
]);

// Load the committed production sources, including the integration not built by
// the middleware gate, without a subprocess or generated integration tree.
registerHooks({
  resolve(specifier, context, next) {
    const source = packages.get(specifier);
    if (source !== undefined) return next(new URL(source, root).href, context);
    if (specifier.startsWith(".") && specifier.endsWith(".js") && context.parentURL?.startsWith(root.href)) {
      const candidate = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
      if (existsSync(candidate)) return next(candidate.href, context);
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.startsWith(root.href) && url.endsWith(".ts") && !url.includes("/node_modules/")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return next(url, context);
  },
});

