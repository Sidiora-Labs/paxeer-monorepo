import assert from "node:assert/strict";
import test from "node:test";

import {
  EXPLORER_ANCHOR_PATH,
  EXPLORER_NAVIGATION,
  explorerLinkPath,
  type ExplorerLinkTarget,
} from "../src/explorer/links.ts";
import { classifyPerformanceRoute } from "../src/perf/budgets.ts";

test("the control plane admits its own explorer routes and none the explorer now renders", () => {
  for (const item of EXPLORER_NAVIGATION) {
    assert.equal(classifyPerformanceRoute(item.href), "explorer", item.href);
  }
  const linked: readonly ExplorerLinkTarget[] = [
    { kind: "receipt", receiptId: "c".repeat(64), transactionHash: `0x${"d".repeat(64)}` },
    { kind: "transaction", transactionHash: `0x${"d".repeat(64)}` },
    { kind: "address", address: `0x${"2".repeat(40)}` },
  ];
  for (const target of linked) {
    const path = explorerLinkPath(target);
    assert.equal(classifyPerformanceRoute(path), undefined, path);
  }
  for (const retired of [EXPLORER_ANCHOR_PATH, "/explorer/batches", "/explorer/checkpoints"]) {
    assert.equal(classifyPerformanceRoute(retired), undefined, retired);
    assert.ok(linked.every((target) => explorerLinkPath(target) !== retired), retired);
  }
});
