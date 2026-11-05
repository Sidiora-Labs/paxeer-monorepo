import { runScenarios } from "./dist/scenarios.js";
import { runServiceScenarios } from "./service.mjs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

// The middleware conformance runner. It exercises the buyer and seller
// middleware — in process and against the running paid-api service — and
// enforces the non-authority rule: no code path may present success without
// backing evidence. Any failing check exits non-zero so CI fails hard.
export function conformanceReport(suite) {
  const results = suite.results();
  if (!Array.isArray(results) || results.length === 0) throw new Error("invalid_conformance_results");
  const names = new Set();
  for (const result of results) {
    if (result === null || typeof result !== "object"
      || typeof result.name !== "string" || result.name.trim().length === 0
      || names.has(result.name) || typeof result.ok !== "boolean"
      || (result.detail !== undefined && typeof result.detail !== "string")) {
      throw new Error("invalid_conformance_result");
    }
    names.add(result.name);
  }
  const failures = results.filter((result) => !result.ok).length;
  return { results, failures, exitCode: failures === 0 ? 0 : 1 };
}

export async function runConformance() {
  const suite = await runScenarios();
  const serviceEvidence = await runServiceScenarios(suite);
  return { ...conformanceReport(suite), serviceEvidence };
}

async function main() {
  const { results, failures, exitCode } = await runConformance();
  for (const result of results) {
    if (result.ok) {
      process.stdout.write(`ok    ${result.name}\n`);
    } else {
      process.stdout.write(`FAIL  ${result.name}\n      ${result.detail ?? "no detail"}\n`);
    }
  }
  process.stdout.write(`\n${results.length - failures}/${results.length} conformance checks passed\n`);
  return exitCode;
}

if (process.argv[1] !== undefined && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main()
    .then((code) => {
      process.exitCode = code;
    })
    .catch((error) => {
      process.stderr.write(`conformance runner crashed: ${error instanceof Error ? (error.stack ?? error.message) : String(error)}\n`);
      process.exitCode = 1;
    });
}
