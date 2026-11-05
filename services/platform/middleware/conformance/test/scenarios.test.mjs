import assert from "node:assert/strict";
import test from "node:test";
import { runScenarios } from "../dist/scenarios.js";

const suite = runScenarios();

async function runScenariosPassesEveryCheck() {
  const results = (await suite).results();
  assert.ok(results.length > 0, "runScenarios must execute conformance checks");
  for (const result of results) {
    assert.equal(result.ok, true, `${result.name}: ${result.detail ?? "check failed"}`);
  }
}

async function runScenariosChecksCommittedRungIsNotDowngraded() {
  const results = (await suite).results();
  const checks = results.filter((result) => result.name === "x402: a committed requirement refuses receipted-only evidence");
  assert.equal(checks.length, 1, "runScenarios must check that committed requirements refuse receipted-only evidence");
  assert.equal(checks[0].ok, true, checks[0].detail);
}

test(runScenariosPassesEveryCheck);
test(runScenariosChecksCommittedRungIsNotDowngraded);
