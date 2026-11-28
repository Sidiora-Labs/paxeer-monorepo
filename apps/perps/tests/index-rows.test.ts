import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
  indexTimeMs,
  quantityAtPrice,
  triggerIdFields,
  orderListVariables,
} from "../packages/paxeer-backend/src/core/perps/indexRows";

const root = fileURLToPath(new URL("../", import.meta.url));

function aTimeTextOfTheIndexGivesItsMilliseconds() {
  for (const [text, expected] of [
    ["2026-11-28T04:52:35Z", 1795841555000],
    ["2026-11-28T04:52:35.12Z", 1795841555120],
    ["2026-11-28T04:52:35.123456Z", 1795841555123],
    ["2026-11-28T04:52:35.123456789Z", 1795841555123],
    ["2026-11-28T05:52:35+01:00", 1795841555000],
  ] as const)
    assert.equal(indexTimeMs(text), expected, text);
}

test(aTimeTextOfTheIndexGivesItsMilliseconds);

function aTextOfSecondsAndANumberCountAsSeconds() {
  for (const [value, expected] of [
    ["1", 1000],
    ["1795841555", 1795841555000],
    [1, 1000],
    [1.5, 1500],
    [1n, 1000],
  ] as const)
    assert.equal(indexTimeMs(value), expected, String(value));
}

test(aTextOfSecondsAndANumberCountAsSeconds);

function aValueThatIsNoTimeGivesNoNumber() {
  for (const value of [
    null,
    undefined,
    "",
    "abc",
    "2026-11-28",
    "2026-11-28T04:52:35",
    "2026-13-45T00:00:00Z",
    NaN,
    Infinity,
    true,
    {},
    "2026-02-29T00:00:00Z",
    "2026-04-31T00:00:00Z",
    "2026-11-28T24:00:00Z",
    "2026-11-28T04:60:00Z",
    "2026-11-28T04:52:60Z",
    "2026-11-28T04:52:35+24:00",
    "2026-11-28T04:52:35+01:60",
    " 1 ",
    "1.5",
  ])
    assert.equal(indexTimeMs(value), undefined, String(value));
}

test(aValueThatIsNoTimeGivesNoNumber);

function theQuantityOfAnOrderIsRoundedToEightPlaces() {
  for (const [size, price, expected] of [
    [999960720000000000000n, 644720000000000000n, 1551],
    [999925158000000000000n, 59202200000000000000000n, 0.01689],
    [1000000000000000000n, 3000000000000000000n, 0.33333333],
    [2000000000000000000n, 3000000000000000000n, 0.66666667],
    [0n, 3000000000000000000n, 0],
    [999960720000000000000n - 644720000000000000000n, 644720000000000000n, 551],
    [1n, 200000000n, 0.00000001],
    [1n, 200000001n, 0],
    [1000000000000000000n, 0n, undefined],
    [1000000000000000000n, -1n, undefined],
    [-1n, 3000000000000000000n, undefined],
  ] as const)
    assert.equal(quantityAtPrice(size, price), expected, `${size}/${price}`);
}

test(theQuantityOfAnOrderIsRoundedToEightPlaces);

function aPlainOrderCarriesNoTriggerId() {
  for (const type of ["LIMIT", "MARKET"])
    assert.deepEqual(triggerIdFields(type, "7"), {}, type);
  for (const type of [
    "STOP_LIMIT",
    "STOP_MARKET",
    "TAKE_PROFIT",
    "STOP_LOSS",
    "OTHER",
  ])
    assert.deepEqual(triggerIdFields(type, "7"), { algo_order_id: "7" }, type);
}

test(aPlainOrderCarriesNoTriggerId);

function theListOfRestingOrdersAsksTheIndexForItsState() {
  assert.deepEqual(orderListVariables("abc", "INCOMPLETE"), {
    userAddress: "abc",
    status: "active",
    limit: 500,
  });
  for (const status of ["ALL", "NEW", "FILLED", "CANCELLED", "", null])
    assert.deepEqual(orderListVariables("abc", status), {
      userAddress: "abc",
      limit: 500,
    });
}

test(theListOfRestingOrdersAsksTheIndexForItsState);

function theAdapterAndTheQueryUseTheFourFunctions() {
  const adapter = readFileSync(
    join(root, "packages/paxeer-backend/src/nativeClient.ts"),
    "utf8",
  );
  const subgraph = readFileSync(
    join(root, "packages/paxeer-backend/src/hooks/perps/useSubgraph.ts"),
    "utf8",
  );
  for (const name of [
    "indexTimeMs",
    "quantityAtPrice",
    "triggerIdFields",
    "orderListVariables",
  ])
    assert.match(adapter, new RegExp(`\\b${name}\\s*\\(`));
  for (const text of [
    "Number(position.openedAt)",
    "Number(row.placedAt)",
    "Number(row.resolvedAt",
    "Number(row.blockTimestamp)",
    "algo_order_id: row.orderId",
  ])
    assert.equal(adapter.includes(text), false, text);
  const queryStart = subgraph.indexOf("export const ORDERS_QUERY");
  assert.ok(queryStart >= 0);
  const queryEnd = subgraph.indexOf("export const", queryStart + 1);
  assert.ok(queryEnd > queryStart);
  const ordersQuery = subgraph.slice(queryStart, queryEnd);
  assert.match(ordersQuery, /\bremainingSize\b/);
  assert.ok((ordersQuery.match(/\blimit\b/g) ?? []).length >= 2);
}

test(theAdapterAndTheQueryUseTheFourFunctions);
