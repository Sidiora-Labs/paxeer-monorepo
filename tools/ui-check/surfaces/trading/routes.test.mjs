import test from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";

function tradingRoutesHaveViews() {
  const pages = readdirSync("apps/perps/apps/web/src/pages").filter(
    (name) => name.endsWith(".tsx") && !name.startsWith("_"),
  );
  const surface = JSON.parse(
    readFileSync(new URL("../trading.json", import.meta.url)),
  );
  const paths = new Set(
    surface.views.map((view) => new URL(view.path, surface.address).pathname),
  );
  const missing = pages.filter(
    (name) =>
      !paths.has(name === "[[...route]].tsx" ? "/" : `/${name.slice(0, -4)}`),
  );
  assert.deepEqual(missing, [], "Each router page has a rendered route");
  for (const route of [
    "/auth/callback",
    "/trade",
    "/trade/grid",
    "/markets",
    "/portfolio",
    ...[
      "positions",
      "orders",
      "assets",
      "feeTier",
      "apiKey",
      "history",
      "setting",
    ].map((part) => `/portfolio/${part}`),
  ]) {
    assert.ok(paths.has(route), `${route} has a rendered view`);
  }
}
test(tradingRoutesHaveViews);
