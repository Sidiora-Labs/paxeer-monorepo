import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import {
  isMainThread,
  parentPort,
  Worker,
  workerData,
} from "node:worker_threads";
import { parseUnits } from "viem";
import {
  getNativeSnapshot,
  updateNativeSnapshot,
} from "../packages/paxeer-backend/src/runtime";

async function nativeBinding() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??=
    "https://api1.mainnet-beta.paxeer.network";
  const client = await import("../packages/paxeer-backend/src/nativeClient");
  const metadata =
    await import("../packages/paxeer-backend/src/nativeMetadata");
  return { ...client, ...metadata };
}

function closeTo(actual: unknown, expected: number) {
  assert.equal(typeof actual, "number");
  assert.ok(
    Math.abs((actual as number) - expected) <= 0.000000001,
    `${actual} differs from ${expected}`,
  );
}

async function nativePositions() {
  const binding = await nativeBinding();
  updateNativeSnapshot({ marketCatalog: [], maintenanceMarginUsd: null });
  const [first, second] = binding.nativeSymbols();
  assert.ok(first && second);
  const rawPosition = (
    marketId: number,
    isLong: boolean,
    size: string,
    entry: string,
    collateral: string,
    leverage: string,
  ) => ({
    marketId,
    isLong,
    positionId: String(marketId + 1),
    sizeUsd: parseUnits(size, 18).toString(),
    entryPrice: parseUnits(entry, 18).toString(),
    collateralUsd: parseUnits(collateral, 18).toString(),
    leverage: parseUnits(leverage, 18).toString(),
    openedAt: "1",
  });
  const positionA = rawPosition(first.marketId, false, "80", "4", "40", "2");
  const positionB = rawPosition(second.marketId, true, "200", "400", "50", "4");
  const marketA = {
    symbol: binding.toNativeSymbol(first.symbol),
    price: 5,
    index_price: 6,
  };
  const marketB = { symbol: binding.toNativeSymbol(second.symbol), price: 600 };
  updateNativeSnapshot({
    positions: [positionA, positionB],
    markets: [marketA, marketB],
  });
  return { ...binding, positionA, positionB, marketA, marketB };
}

async function runCaseInWorker(
  caseFunction: (context?: TestContext) => Promise<void>,
  context?: TestContext,
) {
  assert.ok(context);
  const worker = new Worker(new URL(import.meta.url), {
    execArgv: ["--import", "tsx"],
    workerData: caseFunction.name,
  });
  context.after(async () => {
    await worker.terminate();
  });
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    await new Promise<void>((resolve, reject) => {
      timer = setTimeout(
        () => reject(new Error("Position assertions timed out")),
        30000,
      );
      worker.once("error", reject);
      worker.once("exit", (code) =>
        reject(
          new Error(`Position assertions exited before completion: ${code}`),
        ),
      );
      worker.once("message", (message) => {
        try {
          assert.equal(message, "assertions-passed");
          resolve();
        } catch (error) {
          reject(error);
        }
      });
    });
  } finally {
    clearTimeout(timer);
  }
}

async function aRowCarriesNotionalAndResultFromEntryAndMark(
  context?: TestContext,
) {
  if (isMainThread)
    return runCaseInWorker(
      aRowCarriesNotionalAndResultFromEntryAndMark,
      context,
    );
  const { nativePositionRows, positionA } = await nativePositions();
  const [a, b] = nativePositionRows();
  closeTo(a.position_qty, -20);
  closeTo(a.average_open_price, 4);
  closeTo(a.notional, 100);
  closeTo(a.unrealized_pnl, -20);
  closeTo(a.unsettled_pnl, -20);
  closeTo(a.unrealized_pnl_ROI, -0.5);
  closeTo(a.unrealized_pnl_index, -40);
  closeTo(a.unrealized_pnl_ROI_index, -1);
  closeTo(b.position_qty, 0.5);
  closeTo(b.notional, 300);
  closeTo(b.unrealized_pnl, 100);
  closeTo(b.unrealized_pnl_ROI, 2);
  closeTo(b.unrealized_pnl_index, 0);
  closeTo(b.unrealized_pnl_ROI_index, 0);
  for (const row of [a, b]) {
    assert.equal(row.mmr, undefined);
    assert.equal(row.est_liq_price, undefined);
  }
  updateNativeSnapshot({ positions: [{ ...positionA, collateralUsd: "0" }] });
  closeTo(nativePositionRows()[0].unrealized_pnl_ROI, -0.5);
  updateNativeSnapshot({
    positions: [{ ...positionA, collateralUsd: "0", leverage: "0" }],
  });
  closeTo(nativePositionRows()[0].unrealized_pnl_ROI, 0);
}

async function aRowWithoutAMarkPriceCarriesNoFigure(context?: TestContext) {
  if (isMainThread)
    return runCaseInWorker(aRowWithoutAMarkPriceCarriesNoFigure, context);
  const { nativePositionRows, positionA, marketA } = await nativePositions();
  for (const maintenanceMarginUsd of [null, 0.5]) {
    for (const markets of [
      [],
      ...[0, -1, NaN, Infinity, "5"].map((price) => [{ ...marketA, price }]),
    ]) {
      updateNativeSnapshot({
        positions: [positionA],
        markets,
        maintenanceMarginUsd,
      });
      const [row] = nativePositionRows();
      assert.equal(row.notional, undefined);
      assert.equal(row.unrealized_pnl, undefined);
      assert.equal(row.unrealized_pnl_ROI, undefined);
      assert.equal(row.mm, null);
      closeTo(row.position_qty, -20);
      closeTo(row.average_open_price, 4);
    }
  }
}

async function theMaintenanceMarginOfTheAccountIsSplitByNotional(
  context?: TestContext,
) {
  if (isMainThread)
    return runCaseInWorker(
      theMaintenanceMarginOfTheAccountIsSplitByNotional,
      context,
    );
  const { nativePositionRows, marketA } = await nativePositions();
  const { sumPositionMaintenanceMargin } =
    await import("../packages/hooks/src/paxeer-network/useMarginRatio");
  updateNativeSnapshot({ maintenanceMarginUsd: 0.5 });
  let rows = nativePositionRows();
  closeTo(rows[0].mm, 0.125);
  closeTo(rows[1].mm, 0.375);
  closeTo(sumPositionMaintenanceMargin(rows), 0.5);
  updateNativeSnapshot({ maintenanceMarginUsd: null });
  rows = nativePositionRows();
  assert.equal(rows[0].mm, null);
  assert.equal(rows[1].mm, null);
  assert.equal(sumPositionMaintenanceMargin(rows), null);
  updateNativeSnapshot({ maintenanceMarginUsd: 0 });
  rows = nativePositionRows();
  closeTo(rows[0].mm, 0);
  closeTo(rows[1].mm, 0);
  closeTo(sumPositionMaintenanceMargin(rows), 0);
  updateNativeSnapshot({ maintenanceMarginUsd: 0.5, markets: [marketA] });
  rows = nativePositionRows();
  closeTo(rows[0].mm, 0.5);
  assert.equal(rows[1].mm, null);
  closeTo(sumPositionMaintenanceMargin(rows), 0.5);
  updateNativeSnapshot({ markets: [] });
  assert.ok(nativePositionRows().every((row) => row.mm === null));
}

async function theTotalsOfAListComeFromItsRows(context?: TestContext) {
  if (isMainThread)
    return runCaseInWorker(theTotalsOfAListComeFromItsRows, context);
  const { nativePositionRows, nativePositionTotals, marketA } =
    await nativePositions();
  const rows = nativePositionRows();
  const totals = nativePositionTotals(rows, 1080);
  closeTo(totals.notional, 400);
  closeTo(totals.unrealPnL, 80);
  closeTo(totals.total_unreal_pnl, 80);
  closeTo(totals.total_unreal_pnl_index, -40);
  closeTo(totals.unrealPnlROI, 0.08);
  closeTo(totals.unrealPnlROI_index, -40 / 1120);
  for (const totalValue of [null, 0, 80, NaN, Infinity])
    closeTo(nativePositionTotals(rows, totalValue).unrealPnlROI, 0);
  closeTo(nativePositionTotals(rows, -40).unrealPnlROI_index, 0);
  assert.deepEqual(nativePositionTotals([], 1080), {
    notional: 0,
    unrealPnL: 0,
    total_unreal_pnl: 0,
    total_unreal_pnl_index: 0,
    unrealPnlROI: 0,
    unrealPnlROI_index: 0,
  });
  const onlyA = nativePositionTotals([rows[0]], 1080);
  closeTo(onlyA.notional, 100);
  closeTo(onlyA.unrealPnL, -20);
  closeTo(onlyA.total_unreal_pnl, -20);
  closeTo(onlyA.total_unreal_pnl_index, -40);
  closeTo(onlyA.unrealPnlROI, -20 / 1100);
  closeTo(onlyA.unrealPnlROI_index, -40 / 1120);
  updateNativeSnapshot({ markets: [marketA] });
  assert.deepEqual(nativePositionTotals(nativePositionRows(), 1080), onlyA);
  assert.deepEqual(
    nativePositionTotals(
      [
        rows[0],
        {},
        {
          notional: NaN,
          unrealized_pnl: Infinity,
          unrealized_pnl_index: -Infinity,
        },
        { notional: "300", unrealized_pnl: null, unrealized_pnl_index: "100" },
      ],
      1080,
    ),
    onlyA,
  );
}

async function anotherAddressForgetsTheMaintenanceMargin(
  context?: TestContext,
) {
  if (isMainThread)
    return runCaseInWorker(anotherAddressForgetsTheMaintenanceMargin, context);
  assert.equal(getNativeSnapshot().maintenanceMarginUsd, null);
  updateNativeSnapshot({
    address: "0x0000000000000000000000000000000000000001",
    maintenanceMarginUsd: 0.5,
  });
  closeTo(getNativeSnapshot().maintenanceMarginUsd, 0.5);
  updateNativeSnapshot({
    address: "0x0000000000000000000000000000000000000001",
  });
  closeTo(getNativeSnapshot().maintenanceMarginUsd, 0.5);
  updateNativeSnapshot({
    address: "0x0000000000000000000000000000000000000002",
  });
  assert.equal(getNativeSnapshot().maintenanceMarginUsd, null);
}

async function theRateOfARowComesFromTheLeverageOfThePosition(
  context?: TestContext,
) {
  if (isMainThread)
    return runCaseInWorker(
      theRateOfARowComesFromTheLeverageOfThePosition,
      context,
    );
  const { nativePositionRows, positionA } = await nativePositions();
  updateNativeSnapshot({
    positions: [
      {
        ...positionA,
        collateralUsd: parseUnits("4000", 18).toString(),
        leverage: parseUnits("1", 18).toString(),
      },
    ],
  });
  let row = nativePositionRows()[0];
  closeTo(row.unrealized_pnl, -20);
  closeTo(row.unrealized_pnl_ROI, -0.25);
  closeTo(row.imr, 1);
  updateNativeSnapshot({
    positions: [
      {
        ...positionA,
        collateralUsd: parseUnits("4000", 18).toString(),
        leverage: "0",
      },
    ],
  });
  row = nativePositionRows()[0];
  closeTo(row.unrealized_pnl_ROI, -0.005);
  closeTo(row.imr, 50);
  updateNativeSnapshot({
    positions: [{ ...positionA, collateralUsd: "0", leverage: "0" }],
  });
  row = nativePositionRows()[0];
  closeTo(row.unrealized_pnl_ROI, 0);
  assert.equal(row.imr, undefined);
}

async function aRowCarriesTheLiquidationPriceAndTheFundingOfTheVenue(
  context?: TestContext,
) {
  if (isMainThread)
    return runCaseInWorker(
      aRowCarriesTheLiquidationPriceAndTheFundingOfTheVenue,
      context,
    );
  const { nativePositionRows, positionA } = await nativePositions();
  const position = {
    ...positionA,
    liquidationPrice: parseUnits("3", 18).toString(),
    fundingOwedUsd: parseUnits("-0.25", 18).toString(),
  };
  updateNativeSnapshot({ positions: [position] });
  let row = nativePositionRows()[0];
  closeTo(row.est_liq_price, 3);
  closeTo(row.accrued_funding_fee, 0.25);
  updateNativeSnapshot({ positions: [{ ...position, fundingOwedUsd: "0" }] });
  row = nativePositionRows()[0];
  assert.ok(Object.is(row.accrued_funding_fee, 0));
  updateNativeSnapshot({
    positions: [
      { ...position, fundingOwedUsd: parseUnits("0.5", 18).toString() },
    ],
  });
  closeTo(nativePositionRows()[0].accrued_funding_fee, -0.5);
  updateNativeSnapshot({ positions: [positionA] });
  row = nativePositionRows()[0];
  assert.equal(row.est_liq_price, undefined);
  assert.equal(row.accrued_funding_fee, undefined);
}

const cases = {
  aRowCarriesNotionalAndResultFromEntryAndMark,
  aRowWithoutAMarkPriceCarriesNoFigure,
  theMaintenanceMarginOfTheAccountIsSplitByNotional,
  theTotalsOfAListComeFromItsRows,
  anotherAddressForgetsTheMaintenanceMargin,
  theRateOfARowComesFromTheLeverageOfThePosition,
  aRowCarriesTheLiquidationPriceAndTheFundingOfTheVenue,
};

if (isMainThread) {
  test(aRowCarriesNotionalAndResultFromEntryAndMark);
  test(aRowWithoutAMarkPriceCarriesNoFigure);
  test(theMaintenanceMarginOfTheAccountIsSplitByNotional);
  test(theTotalsOfAListComeFromItsRows);
  test(anotherAddressForgetsTheMaintenanceMargin);
  test(theRateOfARowComesFromTheLeverageOfThePosition);
  test(aRowCarriesTheLiquidationPriceAndTheFundingOfTheVenue);
} else {
  void (async () => {
    const original = getNativeSnapshot();
    try {
      process.env.NEXT_PUBLIC_PAXEER_RPC_URLS =
        "https://api1.mainnet-beta.paxeer.network";
      process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL =
        "https://api1.mainnet-beta.paxeer.network";
      await cases[workerData as keyof typeof cases]();
    } finally {
      updateNativeSnapshot(original);
    }
    assert.ok(parentPort);
    parentPort.postMessage("assertions-passed");
  })();
}
