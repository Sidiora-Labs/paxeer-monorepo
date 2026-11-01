import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import test from "node:test";
import { fileURLToPath } from "node:url";

function agentSpendStartsWithoutProtocolVersion() {
  const env = Object.fromEntries(Object.entries(process.env).filter(([name]) => !name.startsWith("LAYERX_")));
  const result = spawnSync(process.execPath, [fileURLToPath(new URL("../agent-spend/index.mjs", import.meta.url))], {
    env,
    encoding: "utf8",
    timeout: 10000,
  });

  assert.ifError(result.error);
  assert.equal(result.signal, null);
  assert.equal(result.status, 1, result.stderr);
  assert.equal(result.stdout, "");
  assert.match(result.stderr, /Error: missing_layerx_agent_rpc_url\b/u);
  assert.doesNotMatch(result.stderr, /missing_layerx_protocol_version/u);
}

test(agentSpendStartsWithoutProtocolVersion);
