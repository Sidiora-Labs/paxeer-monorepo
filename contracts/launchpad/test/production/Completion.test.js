"use strict";

const fs = require("node:fs");
const path = require("node:path");
const { expect } = require("chai");

function LaunchpadCompletionRetiredSurfacesAbsent() {
  const root = path.resolve(__dirname, "../../../..");
  const retiredPath = "modules/launchpad/keeper/history.go";
  const remainingRetiredSurfaces = [];

  if (fs.existsSync(path.join(root, retiredPath))) {
    const source = fs.readFileSync(path.join(root, retiredPath), "utf8");
    for (const symbol of ["HoldingCheckpointCount", "BeforeBalanceChange"]) {
      if (new RegExp(`\\b${symbol}\\s*\\(`).test(source)) {
        remainingRetiredSurfaces.push(`${retiredPath}::${symbol}`);
      }
    }
  }

  expect(remainingRetiredSurfaces, "retired domain-4 surfaces must be absent").to.deep.equal([]);

  for (const plannedPath of [
    "apps/launchpad/web/src/widgets/trading/transaction-controls/optical-errors.test.ts",
    "apps/launchpad/mobile/src/runtime/routes/activity.ts",
    "apps/launchpad/mobile/test/vitest/routes-activity-quote.test.ts",
  ]) {
    const absolutePath = path.join(root, plannedPath);
    expect(fs.existsSync(absolutePath), `required domain-4 path: ${plannedPath}`).to.equal(true);
    expect(fs.statSync(absolutePath).isFile(), `required domain-4 file: ${plannedPath}`).to.equal(
      true
    );
  }
}

it(LaunchpadCompletionRetiredSurfacesAbsent.name, LaunchpadCompletionRetiredSurfacesAbsent);
