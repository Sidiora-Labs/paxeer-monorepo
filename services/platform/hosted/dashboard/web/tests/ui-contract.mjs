import assert from "node:assert/strict";
import { readFile, stat } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { gatewayOriginConfigured, readGatewayTenant } from "../app/runtime-config.ts";

assert.equal(readGatewayTenant("tenant-01.example"), "tenant-01.example");
assert.equal(readGatewayTenant(undefined), undefined);
assert.equal(readGatewayTenant("Tenant"), undefined);
assert.equal(readGatewayTenant("a".repeat(96)), undefined);
assert.equal(gatewayOriginConfigured("https://gateway.example"), true);
assert.equal(gatewayOriginConfigured(""), false);
assert.equal(gatewayOriginConfigured(undefined), false);

const pageSource = await readFile(
  new URL("../app/page.tsx", import.meta.url),
  "utf8",
);
const source = await readFile(
  new URL("../app/dashboard-client.tsx", import.meta.url),
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
assert.doesNotMatch(
  pageSource,
  /gateway-tenant/,
  "the server page does not render a tenant field",
);
assert.doesNotMatch(
  source,
  /gateway-tenant|name="tenant"|Gateway tenant/,
  "the sign-in view does not collect a tenant",
);
assert.match(
  pageSource,
  /dynamic\s*=\s*"force-dynamic"/,
  "the page reads runtime settings for each request",
);
assert.match(
  pageSource,
  /DASHBOARD_GATEWAY_TENANT/,
  "the page receives the gateway tenant from its runtime setting",
);
assert.match(
  source,
  /registrationProof\(gatewayTenant\)[\s\S]*sessionProof\(gatewayTenant\)/,
  "key proofs use the runtime gateway tenant",
);
assert.match(
  source,
  /name="token"\s+type="password"\s+autoComplete="current-password"/,
  "token sign-in supports credential autofill",
);
assert.match(
  source,
  /onInvalid=\{\(event\) =>/,
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

const tokenSheet = await readFile(
  new URL("../../../../../../apps/web/packages/layerx-ui/src/styles/tokens.css", import.meta.url),
  "utf8",
);
function tokenPair(name) {
  const match = tokenSheet.match(new RegExp(`--${name}:\\s*light-dark\\((#[0-9a-f]+),\\s*(#[0-9a-f]+)\\)`));
  assert.ok(match, `token has light and dark values: ${name}`);
  return [match[1], match[2]];
}
function luminance(hex) {
  const channels = [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset + 1, offset + 3), 16) / 255);
  const linear = channels.map((channel) => (channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4));
  return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
}
function contrast(foreground, background) {
  const light = luminance(foreground);
  const dark = luminance(background);
  return (Math.max(light, dark) + 0.05) / (Math.min(light, dark) + 0.05);
}
const [primaryLight, primaryDark] = tokenPair("primary");
const [primaryForegroundLight, primaryForegroundDark] = tokenPair("primary-foreground");
for (const [foreground, background, scheme] of [
  [primaryForegroundLight, primaryLight, "light"],
  [primaryForegroundDark, primaryDark, "dark"],
]) {
  assert.ok(contrast(foreground, background) >= 4.5, `busy control contrast holds in ${scheme} scheme`);
}
assert.match(
  globals,
  /dashboard-busy-control\[aria-busy="true"\][\s\S]*background-color: var\(--primary\)[\s\S]*color: var\(--primary-foreground\)[\s\S]*opacity: 1/,
  "busy controls use opaque primary colours from the view class",
);
