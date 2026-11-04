import { readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";

const sdkRoot = new URL("../../../agent/sdk/typescript/src/", import.meta.url);
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "@sidiora/layerx-sdk") {
      return { url: new URL("index.ts", sdkRoot).href, shortCircuit: true };
    }
    if (context.parentURL?.startsWith(sdkRoot.href)
      && specifier.startsWith(".") && specifier.endsWith(".js")) {
      return { url: new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(sdkRoot.href) && url.endsWith(".ts")) {
      return {
        format: "module", shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }),
      };
    }
    return nextLoad(url, context);
  },
});

async function repair22331408() {
  await import("../marketplace/test/deploy-payload.test.mjs");
  await import("../marketplace/test/receipt-outcome.test.mjs");
}

await repair22331408();
