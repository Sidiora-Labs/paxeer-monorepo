import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const requiredClasses = [
  "bg-background",
  "bg-surface",
  "bg-surface-sunken",
  "border-border",
  "text-foreground",
  "text-muted-foreground",
  "rounded-md",
  "h-dvh",
  "shrink-0",
  "items-center",
  "justify-center",
  "font-semibold",
  "px-3",
  "transition-colors",
  "dashboard-shell",
  "dashboard-tabs",
  "dashboard-panel",
  "dashboard-card",
  "dashboard-stats",
  "dashboard-usage",
  "dashboard-webhooks",
  "dashboard-sign-in",
  "dashboard-field",
  "dashboard-badge",
];

async function styleSheets(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map(async (entry) => {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) return styleSheets(path);
      return entry.isFile() && entry.name.endsWith(".css") ? [path] : [];
    }),
  );
  return nested.flat();
}

function hasClassRule(sheet, className) {
  const escaped = className.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return new RegExp(`\\.${escaped}(?=[\\s{,.:>+~\\[]|$)`).test(sheet);
}

const staticDirectory = fileURLToPath(
  new URL("../.next/static/", import.meta.url),
);
const sheets = await styleSheets(staticDirectory);
assert.ok(
  sheets.length > 0,
  "dashboard build did not produce a stylesheet under .next/static",
);
const contents = await Promise.all(
  sheets.map((path) => readFile(path, "utf8")),
);
const combined = contents.join("\n");
const missing = requiredClasses.filter(
  (className) => !hasClassRule(combined, className),
);
assert.deepEqual(
  missing,
  [],
  `dashboard stylesheet is missing UI utility rules: ${missing.join(", ")}`,
);

assert.match(
  combined,
  /repeat\(\s*auto-fit,\s*minmax\(\s*min\(\s*100%,\s*var\(--space-popover\)\s*\),\s*1fr\s*\)\s*\)/,
  "dashboard rows fit their available space using the shared width scale",
);
assert.match(
  combined,
  /var\(--space-5\)/,
  "dashboard gaps use the shared spacing scale",
);
assert.match(
  combined,
  /var\(--font-size-body\)/,
  "dashboard body text uses the shared type scale",
);
assert.match(
  combined,
  /:user-invalid/,
  "dashboard field styling follows native interaction state",
);
assert.match(
  combined,
  /prefers-reduced-motion/,
  "dashboard styles serve the reduced-motion preference",
);
