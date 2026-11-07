import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { HOME_DESTINATIONS, homeActions } = await import("../src/journeys/home/model.ts");
const { copyEntry } = await import("../copy/runtime.ts");

function home_destinations_send_outward_moves_to_bridge() {
  assert.equal(HOME_DESTINATIONS.add, "/bridge");
  assert.equal(HOME_DESTINATIONS.withdraw, "/bridge");
  for (const route of Object.values(HOME_DESTINATIONS)) {
    for (const retired of ["/app/deposit", "/app/withdraw"]) {
      assert.equal(route.startsWith(retired), false, `${route} starts with retired route ${retired}`);
    }
  }
}

function home_actions_link_no_retired_route() {
  const actions = homeActions();
  assert.deepEqual(actions, [
    { id: "add", label: copyEntry("home.actions.add").message, icon: "add", route: "/bridge" },
    { id: "move", label: copyEntry("home.actions.move").message, icon: "move", route: "/app/move" },
    { id: "withdraw", label: copyEntry("home.actions.withdraw").message, icon: "withdraw", route: "/bridge" },
  ]);
  for (const { route } of actions) {
    for (const retired of ["/app/deposit", "/app/withdraw", "/app/journeys"]) {
      assert.equal(route.startsWith(retired), false, `${route} starts with retired route ${retired}`);
    }
  }
}

test(home_destinations_send_outward_moves_to_bridge);
test(home_actions_link_no_retired_route);
