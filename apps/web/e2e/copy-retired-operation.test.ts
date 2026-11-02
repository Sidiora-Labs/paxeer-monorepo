import assert from "node:assert/strict";
import test from "node:test";

import { BANNED_VOCABULARY, copyEntries } from "../copy/catalog.ts";
import { runtimeMessages } from "../copy/messages.generated.ts";
import { copyEntry } from "../copy/runtime.ts";

function copy_has_operation_retired_key() {
  const entry = copyEntry("error.operation.retired");
  assert.equal(entry.key, "error.operation.retired");
  assert.ok(entry.message.trim().length > 0);
  assert.equal(entry.message, "This action is no longer available.");
  assert.doesNotMatch(entry.message, /[{}]/u);
  for (const banned of BANNED_VOCABULARY) {
    assert.doesNotMatch(entry.message, new RegExp(`\b${banned}\b`, "iu"));
  }
}

function copy_runtime_matches_catalog_after_retired_key() {
  const runtimeKeys = runtimeMessages.map(([key]) => key);
  const catalogKeys = copyEntries.map(({ key }) => key);
  assert.ok(runtimeKeys.includes("error.operation.retired"));
  assert.ok(catalogKeys.includes("error.operation.retired"));
  assert.equal(new Set(runtimeKeys).size, runtimeKeys.length);
  assert.equal(new Set(catalogKeys).size, catalogKeys.length);
  assert.deepEqual(runtimeKeys.toSorted(), catalogKeys.toSorted());
}

test(copy_has_operation_retired_key);
test(copy_runtime_matches_catalog_after_retired_key);
