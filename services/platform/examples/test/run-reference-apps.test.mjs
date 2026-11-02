import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";

function referenceAppsCheckPassesWithoutRetiredSurfaces() {
  const env = Object.fromEntries(Object.entries(process.env).filter(([name]) => !name.startsWith("LAYERX_")));
  const result = spawnSync(process.execPath, [
    fileURLToPath(new URL("../run-reference-apps.mjs", import.meta.url)), "--check",
  ], {
    env,
    encoding: "utf8",
    timeout: 10000,
  });

  assert.ifError(result.error);
  assert.equal(result.signal, null);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stderr, "");
  const checked = JSON.parse(result.stdout);
  assert.deepEqual(checked.checked, ["buyer-agent", "paid-api", "merchant-shop", "marketplace"]);
  assert.deepEqual(checked.environments, ["emulator", "beta"]);
}

test(referenceAppsCheckPassesWithoutRetiredSurfaces);
