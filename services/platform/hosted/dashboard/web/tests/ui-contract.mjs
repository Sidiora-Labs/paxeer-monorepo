import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const source = await readFile(new URL("../app/page.tsx", import.meta.url), "utf8");
for (const surface of ["API keys", "Requests", "Webhook endpoints", "Delivery log", "test payments", "verified receipts"]) {
  assert.match(source, new RegExp(surface, "i"), `developer surface missing: ${surface}`);
}
assert.match(source, /from "@layerx\/ui"/, "dashboard must consume the owner UI package");
assert.doesNotMatch(source, /x-layerx-principal/i, "dashboard must never trust an identity header");

const globalsPath = fileURLToPath(new URL("../app/globals.css", import.meta.url));
const globals = await readFile(globalsPath, "utf8");
const sourceMatch = globals.match(/@source\s+["']([^"']+)["']/);
assert.ok(sourceMatch, "dashboard styles must name the UI source");
const uiSourcePath = resolve(dirname(globalsPath), sourceMatch[1]);
assert.equal((await stat(uiSourcePath)).isDirectory(), true, "dashboard UI source must resolve to a directory");
assert.equal((await stat(resolve(uiSourcePath, "components/app-shell.tsx"))).isFile(), true, "dashboard UI source must include app-shell.tsx");
