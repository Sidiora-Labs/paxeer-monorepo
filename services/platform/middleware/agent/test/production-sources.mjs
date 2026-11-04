import { readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

const agent = new URL("../src/", import.meta.url).href;
const seller = new URL("../../seller/src/", import.meta.url).href;
const sdk = new URL("../../../../agent/sdk/typescript/src/", import.meta.url).href;
const integration = new URL("../../../../../sdk/integrations/agents/src/", import.meta.url).href;
const roots = [agent, seller, sdk, integration];
const packages = new Map([
  ["@sidiora/layerx-agent-middleware", `${agent}index.ts`],
  ["@sidiora/layerx-seller-middleware", `${seller}index.ts`],
  ["@sidiora/layerx-sdk", `${sdk}index.ts`],
  ["@sidiora/layerx-agent-integrations", `${integration}index.ts`],
]);
registerHooks({
  resolve(specifier, context, next) {
    if (packages.has(specifier)) return next(packages.get(specifier), context);
    if (context.parentURL?.startsWith(new URL("./", import.meta.url).href)
      && specifier === "../dist/index.js") return next(`${agent}index.ts`, context);
    if (roots.some((root) => context.parentURL?.startsWith(root))
      && specifier.startsWith(".") && specifier.endsWith(".js")) {
      return next(new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, context);
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (!roots.some((root) => url.startsWith(root)) || !url.endsWith(".ts")) return next(url, context);
    return { format: "module", shortCircuit: true,
      source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
  },
});
