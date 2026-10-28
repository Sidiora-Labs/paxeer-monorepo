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
        const source = candidate.href.endsWith(".js")
          ? new URL(candidate.href.slice(0, -3) + suffix)
          : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module",
        shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: {
            target: ts.ScriptTarget.ES2022,
            module: ts.ModuleKind.ESNext,
            jsx: ts.JsxEmit.ReactJSX,
          },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { EXPLORER_NAVIGATION } = await import("../src/explorer/links.ts");
const { CURRENT_ROUTES, humanStateMatrix } = await import("../src/states/matrix.ts");
const { ROUTE_SCRIPT_BUDGETS, classifyPerformanceRoute } = await import(
  "../src/perf/budgets.ts"
);

function explorer_navigation_has_no_verify_entry() {
  assert.deepEqual(EXPLORER_NAVIGATION, [
    { href: "/explorer", copyKey: "explorer.navigation.overview" },
  ]);
}

function state_matrix_has_no_verify_route() {
  const matrix = humanStateMatrix();
  matrix.assertComplete();
  const currentRoutes = new Set<string>(CURRENT_ROUTES);
  const entries = matrix.enumerate();

  assert.equal(currentRoutes.has("/explorer/verify"), false);
  assert.equal(entries.some((entry) => entry.route === "/explorer/verify"), false);
  assert.equal(entries.some((entry) => entry.id === "route.explorer.verify"), false);
  assert.equal(Object.hasOwn(ROUTE_SCRIPT_BUDGETS, "/explorer/verify"), false);
  for (const pathname of ["/explorer/verify", "/explorer/verify/", "/explorer/verify/receipt"]) {
    assert.equal(classifyPerformanceRoute(pathname), undefined, `${pathname} has no performance classification`);
  }

  for (const route of [
    "/explorer",
    "/explorer/receipts/[receiptId]",
    "/explorer/accounts/[accountId]",
  ]) {
    assert.equal(currentRoutes.has(route), true, `${route} remains a current route`);
    assert.equal(entries.some((entry) => entry.kind === "route" && entry.route === route), true);
    assert.equal(Object.hasOwn(ROUTE_SCRIPT_BUDGETS, route), true, `${route} retains its script budget`);
    assert.equal(classifyPerformanceRoute(route), "explorer", `${route} retains its performance classification`);
  }
}

test(explorer_navigation_has_no_verify_entry);
test(state_matrix_has_no_verify_route);
