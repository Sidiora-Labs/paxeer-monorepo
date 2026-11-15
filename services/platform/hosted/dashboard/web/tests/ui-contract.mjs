import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const source = await readFile(
  new URL("../app/page.tsx", import.meta.url),
  "utf8",
);
for (const surface of [
  "API keys",
  "Requests",
  "Webhook endpoints",
  "Delivery log",
  "test payments",
  "verified receipts",
]) {
  assert.match(
    source,
    new RegExp(surface, "i"),
    `developer surface missing: ${surface}`,
  );
}
assert.match(
  source,
  /from "@layerx\/ui"/,
  "dashboard must consume the owner UI package",
);
assert.doesNotMatch(
  source,
  /x-layerx-principal/i,
  "dashboard must never trust an identity header",
);

const globalsPath = fileURLToPath(
  new URL("../app/globals.css", import.meta.url),
);
const globals = await readFile(globalsPath, "utf8");
const sourceMatch = globals.match(/@source\s+["']([^"']+)["']/);
assert.ok(sourceMatch, "dashboard styles must name the UI source");
const uiSourcePath = resolve(dirname(globalsPath), sourceMatch[1]);
assert.equal(
  (await stat(uiSourcePath)).isDirectory(),
  true,
  "dashboard UI source must resolve to a directory",
);
assert.equal(
  (await stat(resolve(uiSourcePath, "components/app-shell.tsx"))).isFile(),
  true,
  "dashboard UI source must include app-shell.tsx",
);

for (const control of [
  "Create a developer key in this browser and sign in",
  "Sign in with the key this browser holds",
  "Sign in with a session token",
  "Sign out",
  "Copy public key",
]) {
  assert.ok(
    source.includes(control),
    `dashboard session control is present: ${control}`,
  );
}
assert.match(
  source,
  /needsSession\s*\?\s*(?:\(\s*)?<SignInView/,
  "a refused read presents the sign-in view",
);
assert.match(
  source,
  /notificationControl=\{signOutControl\}/,
  "the shell presents sign-out on desktop and mobile",
);
assert.match(
  source,
  /sessionRequest\("\/session", \{\}, "DELETE"\)/,
  "sign-out removes the host session cookie",
);
assert.match(
  source,
  /does not support Ed25519/,
  "the token path explains browser key support",
);

assert.match(
  source,
  /role="tablist"\s+aria-label="Dashboard section"/,
  "dashboard sections expose a named tab group",
);
assert.match(
  source,
  /aria-selected=\{active === tab.value\}/,
  "tabs expose their selected section",
);
assert.match(
  source,
  /tabIndex=\{active === tab.value \? 0 : -1\}/,
  "only the selected tab is in sequential focus order",
);
for (const key of ["ArrowLeft", "ArrowRight", "Home", "End"]) {
  assert.ok(
    source.includes(`event.key === "${key}"`),
    `section tabs support ${key}`,
  );
}
assert.match(
  source,
  /role="tabpanel"/,
  "the selected section exposes a tab panel",
);
assert.match(
  source,
  /aria-labelledby=\{`tab-\$\{active\}`\}/,
  "the selected tab names its panel",
);
assert.match(
  source,
  /name="tenant"\s+autoComplete="username"/,
  "tenant sign-in supports identity autofill",
);
assert.match(
  source,
  /name="token"\s+type="password"\s+autoComplete="current-password"/,
  "token sign-in supports credential autofill",
);
assert.match(
  source,
  /onInvalid=\{\(\) => setTenantInvalid\(true\)\}/,
  "native tenant validation exposes an error after interaction",
);
assert.match(
  source,
  /onInvalid=\{\(\) => setTokenInvalid\(true\)\}/,
  "native token validation exposes an error after interaction",
);
assert.match(
  globals,
  /overflow-wrap: anywhere/,
  "long identifiers stay readable inside their card",
);
assert.match(
  globals,
  /repeat\(\s*auto-fit,\s*minmax\(min\(100%,\s*var\(--space-popover\)\),\s*1fr\)\s*\)/,
  "row layout responds to the card container",
);
assert.match(
  globals,
  /min-block-size: var\(--touch-target\)/,
  "controls use the shared touch target",
);
assert.doesNotMatch(
  globals,
  /#[0-9a-f]{3,8}\b|(?:rgb|hsl|oklch)\(/i,
  "dashboard colours come from shared roles",
);
