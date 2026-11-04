import { readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

// Exercise committed sources without requiring generated dist/ files or a build toolchain.
const packages = new Map([
  ["@sidiora/layerx-sdk", new URL("../../../../agent/sdk/typescript/src/", import.meta.url)],
  ...["buyer", "seller", "merchant"].map((name) => [
    `@sidiora/layerx-${name}-middleware`, new URL(`../../../middleware/${name}/src/`, import.meta.url),
  ]),
]);
const roots = [...packages.values()].map((url) => url.href);

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (packages.has(specifier)) {
      return { url: new URL("index.ts", packages.get(specifier)).href, shortCircuit: true };
    }
    if (roots.some((root) => context.parentURL?.startsWith(root))
      && (specifier.startsWith("./") || specifier.startsWith("../")) && specifier.endsWith(".js")) {
      return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (roots.some((root) => url.startsWith(root)) && url.endsWith(".ts")) {
      return {
        format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }),
      };
    }
    return nextLoad(url, context);
  },
});
