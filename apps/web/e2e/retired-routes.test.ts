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

const { CURRENT_ROUTES, humanStateMatrix } = await import("../src/states/matrix.ts");
const { ROUTE_SCRIPT_BUDGETS, classifyPerformanceRoute } = await import(
  "../src/perf/budgets.ts"
);

function state_matrix_has_no_custody_routes() {
  const retiredRoutes = [
    "/app/deposit",
    "/app/withdraw",
    "/app/move/withdraw",
    "/app/journeys/[journeyId]",
    "/app/journeys/[journeyId]/claim",
    "/app/settings/exit",
  ] as const;
  const matrix = humanStateMatrix();
  matrix.assertComplete();
  const entries = matrix.enumerate();
  const currentRoutes = new Set<string>(CURRENT_ROUTES);

  for (const route of retiredRoutes) {
    assert.equal(currentRoutes.has(route), false, `${route} is not a current route`);
    assert.equal(
      entries.some((entry) => entry.route === route),
      false,
      `${route} has no state-matrix entry`,
    );
    assert.equal(Object.hasOwn(ROUTE_SCRIPT_BUDGETS, route), false, `${route} has no script budget`);
    assert.equal(
      existsSync(new URL(`../src/app${route}/page.tsx`, import.meta.url)),
      false,
      `${route} has no route module`,
    );
    const concreteRoute = route.replace("[journeyId]", "journey-123");
    for (const pathname of [route, `${route}/`, concreteRoute, `${concreteRoute}/`]) {
      assert.equal(classifyPerformanceRoute(pathname), undefined, `${pathname} has no performance classification`);
    }
  }

  for (const route of [
    "/app",
    "/app/move",
    "/app/activity",
    "/app/settings",
    "/app/settings/wallet",
  ]) {
    assert.equal(classifyPerformanceRoute(route), "app", `${route} retains its performance classification`);
    assert.equal(Object.hasOwn(ROUTE_SCRIPT_BUDGETS, route), true, `${route} retains its script budget`);
  }
  assert.equal(classifyPerformanceRoute("/"), "root");
  assert.equal(classifyPerformanceRoute("/explorer"), "explorer");
}

test(state_matrix_has_no_custody_routes);
