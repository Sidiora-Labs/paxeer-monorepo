import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { LXP_RETRYABLE } from "../src/revert.js";

function retryable_set_matches_registry() {
  const table = readFileSync(
    new URL("../../../../../../spec/layerx-merge/contracts/address-table.md", import.meta.url),
    "utf8",
  );
  const start = table.indexOf("\n## (b) ");
  assert.ok(start >= 0, "registry section (b) must exist");
  const end = table.indexOf("\n## ", start + 1);
  assert.ok(end > start, "registry section (b) must have an end");
  const rows = table.slice(start, end).split("\n").filter((line) => line.startsWith("| LXP_"));
  assert.ok(rows.length > 0, "registry must contain codes");

  const registered = new Map<string, boolean>();
  const expected = new Set<string>();
  for (const row of rows) {
    const cells = row.split("|").map((cell) => cell.trim());
    assert.equal(cells.length, 7, row);
    assert.equal(cells[0], "", row);
    assert.equal(cells[6], "", row);
    const code = cells[1];
    const retryable = cells[5];
    assert.ok(code !== undefined, row);
    assert.match(code, /^LXP_[A-Z0-9]+(?:_[A-Z0-9]+)*$/u);
    assert.ok(!registered.has(code), `duplicate registry code: ${code}`);
    assert.ok(retryable === "yes" || retryable === "no", row);
    registered.set(code, retryable === "yes");
    if (retryable === "yes") expected.add(code);
  }
  assert.deepEqual(LXP_RETRYABLE, expected);

  for (const code of [
    "LXP_PRICEFEED_DELEGATECALL",
    "LXP_PRICEFEED_STATICCALL",
    "LXP_PRICEFEED_KEEPER_MISSING",
    "LXP_PRICEFEED_BATCH_TOO_LARGE",
    "LXP_AGENT_NOT_ACTIVE",
    "LXP_BANK_STATICCALL",
    "LXP_BANK_DELEGATECALL",
    "LXP_BANK_ZERO_VALUE",
  ]) {
    assert.equal(registered.get(code), false, code);
    assert.equal(LXP_RETRYABLE.has(code), false, code);
  }
}

test(retryable_set_matches_registry);
