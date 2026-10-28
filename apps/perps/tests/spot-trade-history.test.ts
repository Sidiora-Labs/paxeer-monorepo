import { strict as assert } from "node:assert";
import { test } from "node:test";
import type { SpotTrade, SpotTradesResponse } from "../src/core/spot/api";
import {
  appendSpotTradeHistoryPage,
  mergeSpotTradeSnapshot,
  spotTradeHistory,
  spotTradeSnapshotForAddress,
} from "../src/core/spot/trade-history";

const ADDRESS = "0x00000000000000000000000000000000000000ab";
const OTHER_ADDRESS = "0x00000000000000000000000000000000000000cd";

function trade(block: string, logIndex = 0, user = ADDRESS): SpotTrade {
  return {
    id: `${block}:${logIndex}`,
    txHash: `0x${"1".repeat(64)}`,
    logIndex,
    blockNumber: block,
    blockTimestamp: "2026-10-22T00:00:00Z",
    user,
    tokenIn: `0x${"2".repeat(40)}`,
    tokenOut: `0x${"3".repeat(40)}`,
    amountIn: "1000000",
    amountOut: "2000000",
    executionPrice: "2000000000000000000",
    volumeUsd: "1000000",
    source: "router",
    contract: `0x${"4".repeat(40)}`,
    createdAt: "2026-10-22T00:00:00Z",
  };
}

function response(
  items: SpotTrade[],
  more: boolean,
  address = ADDRESS,
): SpotTradesResponse {
  const last = items.at(-1);
  assert.ok(!more || last);
  return {
    address,
    count: items.length,
    limit: 50,
    items,
    nextCursor:
      more && last
        ? { afterBlock: String(last.blockNumber), afterIndex: last.logIndex }
        : null,
  };
}

function cursorOf(page: SpotTradesResponse) {
  assert.ok(page.nextCursor);
  return page.nextCursor;
}

test("same-account refresh and reconnect install rows with their usable continuation", () => {
  const first = mergeSpotTradeSnapshot(
    response([trade("100", 9), trade("100", 8)], true),
  );
  let history = spotTradeHistory(first);
  history = appendSpotTradeHistoryPage(
    history,
    first,
    cursorOf(first),
    response([trade("100", 7)], false),
  );
  assert.equal(history.page?.nextCursor, null);

  for (const latest of [trade("101"), trade("102")]) {
    const fresh = mergeSpotTradeSnapshot(
      response([latest, trade("100", 9)], true),
    );
    history = spotTradeHistory(fresh);
    assert.equal(history.page, fresh);
    assert.deepEqual(history.page?.nextCursor, {
      afterBlock: "100",
      afterIndex: 9,
    });
    history = appendSpotTradeHistoryPage(
      history,
      fresh,
      cursorOf(fresh),
      response([trade("100", 8), trade("100", 7)], false),
    );
    assert.deepEqual(
      history.page?.items.map((item) => item.id),
      [latest.id, "100:9", "100:8", "100:7"],
    );
    assert.equal(history.page?.nextCursor, null);
  }
});

test("live updates retain the REST cursor, order exact block/index keys, and deduplicate", () => {
  const first = response(
    [trade("9007199254740993", 4), trade("9007199254740992", 9)],
    true,
  );
  const updated = { ...first.items[0], amountOut: "3000000" };
  const live = mergeSpotTradeSnapshot(first, [
    updated,
    trade("9007199254740993", 7),
    trade("9007199254740994", 0),
    updated,
  ]);
  assert.equal(live.nextCursor, first.nextCursor);
  assert.deepEqual(
    live.items.map((item) => item.id),
    [
      "9007199254740994:0",
      "9007199254740993:7",
      "9007199254740993:4",
      "9007199254740992:9",
    ],
  );
  assert.equal(live.items[2].amountOut, "3000000");
  const older = response(
    [
      first.items[1],
      trade("9007199254740992", 8),
      trade("9007199254740991", 10),
    ],
    false,
  );
  const history = appendSpotTradeHistoryPage(
    spotTradeHistory(live),
    live,
    cursorOf(live),
    older,
  );
  assert.equal(history.page?.items.length, 6);
  assert.deepEqual(
    history.page?.items.slice(-2).map((item) => item.id),
    ["9007199254740992:8", "9007199254740991:10"],
  );
  assert.equal(history.page?.nextCursor, null);
  assert.equal(first.items.length, 2);
});

test("live trades received during REST refresh are merged before exposing the paired snapshot", () => {
  const rest = response([trade("50", 9), trade("50", 8)], true);
  const newest = trade("51", 0);
  const lastUpdate = { ...newest, volumeUsd: "2000000" };
  const fresh = mergeSpotTradeSnapshot(rest, [
    lastUpdate,
    newest,
    trade("50", 10),
  ]);
  assert.deepEqual(
    fresh.items.map((item) => item.id),
    ["51:0", "50:10", "50:9", "50:8"],
  );
  assert.equal(fresh.items[0].volumeUsd, "2000000");
  assert.equal(fresh.nextCursor, rest.nextCursor);
  const history = appendSpotTradeHistoryPage(
    spotTradeHistory(fresh),
    fresh,
    cursorOf(fresh),
    response([trade("50", 7)], false),
  );
  assert.equal(history.page?.items.at(-1)?.id, "50:7");
});

test("trimming live snapshots moves the boundary to the last retained row without skipping fills", () => {
  for (const more of [true, false]) {
    const original = response(
      Array.from({ length: 200 }, (_, i) => trade("100", 200 - i)),
      more,
    );
    const live = mergeSpotTradeSnapshot(original, [trade("100", 201)]);
    assert.equal(live.items.length, 200);
    assert.equal(live.items.at(-1)?.id, "100:2");
    assert.deepEqual(live.nextCursor, { afterBlock: "100", afterIndex: 2 });
    const history = appendSpotTradeHistoryPage(
      spotTradeHistory(live),
      live,
      cursorOf(live),
      response([trade("100", 1), trade("100", 0), trade("99", 3)], false),
    );
    assert.deepEqual(
      history.page?.items.slice(-4).map((item) => item.id),
      ["100:2", "100:1", "100:0", "99:3"],
    );
    assert.equal(
      new Set(history.page?.items.map((item) => String(item.id))).size,
      203,
    );
    assert.equal(history.page?.nextCursor, null);
  }
});

test("out-of-order older live events cannot move the cursor past unseen fills", () => {
  const first = response([trade("100", 9), trade("100", 8)], true);
  const live = mergeSpotTradeSnapshot(first, [
    trade("99", 0),
    trade("100", 7),
    trade("100", 8),
  ]);
  assert.deepEqual(live.items, first.items);
  assert.equal(live.nextCursor, first.nextCursor);
  const history = appendSpotTradeHistoryPage(
    spotTradeHistory(live),
    live,
    cursorOf(live),
    response([trade("100", 7), trade("99", 0)], false),
  );
  assert.deepEqual(
    history.page?.items.map((item) => item.id),
    ["100:9", "100:8", "100:7", "99:0"],
  );
});

test("a page response racing a refreshed or live snapshot is rejected", () => {
  const first = response([trade("100", 9)], true);
  const oldCursor = cursorOf(first);
  const delayedPage = response([trade("100", 8)], false);
  for (const replacement of [
    mergeSpotTradeSnapshot(response([trade("101"), trade("100", 9)], true)),
    mergeSpotTradeSnapshot(first, [trade("102")]),
    mergeSpotTradeSnapshot(response([], false)),
  ]) {
    const current = spotTradeHistory(replacement);
    assert.equal(
      appendSpotTradeHistoryPage(current, first, oldCursor, delayedPage),
      current,
    );
    assert.equal(current.page, replacement);
  }
});

test("repeated stale page responses cannot rewind pagination or duplicate rows", () => {
  const first = response([trade("100", 9)], true);
  const older = response([trade("100", 8)], true);
  const history = appendSpotTradeHistoryPage(
    spotTradeHistory(first),
    first,
    cursorOf(first),
    older,
  );
  assert.equal(
    appendSpotTradeHistoryPage(history, first, cursorOf(first), older),
    history,
  );
  assert.equal(history.page?.nextCursor, older.nextCursor);
  const last = appendSpotTradeHistoryPage(
    history,
    first,
    cursorOf(older),
    response([], false),
  );
  assert.deepEqual(
    last.page?.items.map((item) => item.id),
    ["100:9", "100:8"],
  );
  assert.equal(last.page?.nextCursor, null);
});

test("account changes and disconnects never expose a prior account's rows or continuation", () => {
  const first = response([trade("100", 9)], true);
  assert.equal(
    spotTradeSnapshotForAddress(first, ADDRESS.toUpperCase()),
    first,
  );
  assert.equal(spotTradeSnapshotForAddress(first, OTHER_ADDRESS), null);
  assert.equal(spotTradeSnapshotForAddress(first, null), null);
  assert.equal(spotTradeSnapshotForAddress(null, ADDRESS), null);
  const current = spotTradeHistory(
    response([trade("10", 0, OTHER_ADDRESS)], false, OTHER_ADDRESS),
  );
  const delayedPage = response([trade("100", 8)], false);
  assert.equal(
    appendSpotTradeHistoryPage(current, first, cursorOf(first), delayedPage),
    current,
  );
  const oldHistory = spotTradeHistory(first);
  assert.equal(
    appendSpotTradeHistoryPage(
      oldHistory,
      first,
      cursorOf(first),
      response([], false, OTHER_ADDRESS),
    ),
    oldHistory,
  );
  const disconnected = spotTradeHistory(null);
  assert.equal(
    appendSpotTradeHistoryPage(
      disconnected,
      first,
      cursorOf(first),
      delayedPage,
    ),
    disconnected,
  );
});

test("empty and exhausted snapshots stay exhausted across a new live trade", () => {
  const empty = mergeSpotTradeSnapshot(response([], false));
  assert.deepEqual(spotTradeHistory(empty).page?.items, []);
  assert.equal(empty.nextCursor, null);
  const live = mergeSpotTradeSnapshot(empty, [trade("1")]);
  assert.deepEqual(
    live.items.map((item) => item.id),
    ["1:0"],
  );
  assert.equal(live.nextCursor, null);
  assert.equal(live.count, 1);
});
