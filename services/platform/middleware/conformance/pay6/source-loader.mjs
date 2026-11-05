import { existsSync, readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { fileURLToPath } from "node:url";

const root = new URL("../../../../../", import.meta.url);
const packages = {
  "@sidiora/layerx-sdk": "services/agent/sdk/typescript/src/index.ts",
  "@sidiora/layerx-seller-middleware": "services/platform/middleware/seller/src/index.ts",
  "@sidiora/layerx-buyer-middleware": "services/platform/middleware/buyer/src/index.ts",
  "@sidiora/layerx-agent-middleware": "services/platform/middleware/agent/src/index.ts",
};

registerHooks({
  resolve(specifier, context, next) {
    if (packages[specifier]) return { url: new URL(packages[specifier], root).href, shortCircuit: true };
    if (specifier.startsWith(".") && context.parentURL?.startsWith(root.href)) {
      const target = new URL(specifier, context.parentURL);
      const source = new URL(target.href.replace(/\/dist\//u, "/src/").replace(/\.js$/u, ".ts"));
      if (source.href !== target.href && existsSync(fileURLToPath(source))) {
        return { url: source.href, shortCircuit: true };
      }
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (url.startsWith(root.href) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return next(url, context);
  },
});
