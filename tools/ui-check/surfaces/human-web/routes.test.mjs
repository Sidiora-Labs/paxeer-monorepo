import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { relative, sep } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const application = fileURLToPath(
  new URL("../../../../apps/web/src/app/", import.meta.url),
);
const surfaceFile = new URL("../human-web.json", import.meta.url);

async function pageRoutes(directory = application) {
  const entries = await readdir(directory, { withFileTypes: true });
  const routes = await Promise.all(
    entries.map(async (entry) => {
      const path = `${directory}/${entry.name}`;
      if (entry.isDirectory()) return pageRoutes(path);
      if (!/^page\.(?:[jt]sx?|mdx)$/u.test(entry.name)) return [];
      const segments = relative(application, directory)
        .split(sep)
        .filter(
          (segment) =>
            segment && !segment.startsWith("(") && !segment.startsWith("@"),
        );
      return [`/${segments.join("/")}`];
    }),
  );
  return routes.flat().sort();
}

function routePattern(route) {
  const segments = route.split("/").slice(1);
  return new RegExp(
    `^${segments
      .map((segment) => {
        if (/^\[\[\.\.\..+\]\]$/u.test(segment)) return "(?:/[^/]+)*";
        if (/^\[\.\.\..+\]$/u.test(segment)) return "(?:/[^/]+)+";
        if (/^\[.+\]$/u.test(segment)) return "/[^/]+";
        return `/${segment.replace(/[.*+?^${}()|[\]\\]/gu, "\\$&")}`;
      })
      .join("")}/?$`,
    "u",
  );
}

async function humanWebEveryPageHasSurfaceView() {
  const [routes, surface] = await Promise.all([
    pageRoutes(),
    readFile(surfaceFile, "utf8").then(JSON.parse),
  ]);
  assert.ok(routes.length > 0, "The application has page routes");
  const paths = surface.views.map(
    (view) => new URL(view.path, surface.address).pathname,
  );
  const missing = routes.filter(
    (route) => !paths.some((path) => routePattern(route).test(path)),
  );
  assert.deepEqual(
    missing,
    [],
    `Page routes without a surface view:\n${missing.join("\n")}`,
  );
}

function humanWebParametersMatchOneCompleteSegment() {
  const pattern = routePattern("/app/agents/[agentId]");
  assert.ok(pattern.test("/app/agents/sample-agent"));
  assert.ok(pattern.test("/app/agents/another-agent"));
  assert.ok(!pattern.test("/app/agents"));
  assert.ok(!pattern.test("/app/agents/"));
  assert.ok(!pattern.test("/app/agents/sample-agent/settings"));
  assert.ok(!routePattern("/app").test("/app/activity"));
  assert.ok(routePattern("/").test("/"));
  assert.ok(routePattern("/docs/[...parts]").test("/docs/start/setup"));
  assert.ok(!routePattern("/docs/[...parts]").test("/docs"));
  assert.ok(routePattern("/docs/[[...parts]]").test("/docs"));
}

test(humanWebEveryPageHasSurfaceView);
test(humanWebParametersMatchOneCompleteSegment);
