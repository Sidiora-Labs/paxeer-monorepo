import { existsSync, readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

const root = new URL("../../../../../../", import.meta.url);
const entries = new Map([
  ...["merchant", "agent"].map((name) => [
    `@sidiora/layerx-${name}-middleware`, `services/platform/middleware/${name}/src/index.ts`,
  ]),
  ...["express", "next", "agents"].map((name) => [
    name === "agents" ? "@sidiora/layerx-agent-integrations" : `@sidiora/layerx-${name}`,
    `sdk/integrations/${name}/src/index.ts`,
  ]),
]);
const sourceRoots = [...entries.values()].map((path) => new URL("./", new URL(path, root)).href);

// The gate builds SDK/seller/buyer. Load only their unbuilt integration peers
// from production sources, including the peers' real relative dependencies.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (entries.has(specifier)) return nextResolve(new URL(entries.get(specifier), root).href, context);
    if (specifier.startsWith(".") && specifier.endsWith(".js")
      && sourceRoots.some((prefix) => context.parentURL?.startsWith(prefix))) {
      const source = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
      if (existsSync(source)) return nextResolve(source.href, context);
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.endsWith(".ts") && sourceRoots.some((prefix) => url.startsWith(prefix))) {
      return { format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
    }
    return nextLoad(url, context);
  },
});
