import assert from "node:assert/strict";
import test, { type TestContext } from "node:test";
import { parseUnits } from "viem";
import {
  account,
  positions as positionFormulas,
} from "../packages/perp/src/index";
import { MarginMode, OrderSide } from "../packages/types/src/order";
import Decimal from "../packages/utils/src/decimal";
import { calculateMaxQty } from "../packages/hooks/src/paxeer-network/calculateMaxQty";
import { createGetter } from "../packages/hooks/src/utils/createGetter";
import { feeTierView } from "../packages/paxeer-backend/src/core/perps/feeTier";
import {
  getNativeSnapshot,
  updateNativeSnapshot,
} from "../packages/paxeer-backend/src/runtime";

type NativeClient =
  typeof import("../packages/paxeer-backend/src/nativeClient");
type NativeMetadata =
  typeof import("../packages/paxeer-backend/src/nativeMetadata");
async function nativeBinding() {
  process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL ??= "https://gateway.test";
  const client: NativeClient =
    await import("../packages/paxeer-backend/src/nativeClient");
  const metadata: NativeMetadata =
    await import("../packages/paxeer-backend/src/nativeMetadata");
  return { ...client, ...metadata };
}

async function nativeAccountFieldsUseFormulaUnits(context: TestContext) {
  const original = getNativeSnapshot();
  context.after(() => updateNativeSnapshot(original));
  const { nativeAccountInfo, nativeSymbols, nativeRequest } =
    await nativeBinding();
  updateNativeSnapshot({
    address: "0x0000000000000000000000000000000000000001",
    marginMode: "CROSS",
    feeTier: feeTierView([0, 450n, 150n, 0n]),
  });
  const info = nativeAccountInfo();
  assert.equal(info.futures_taker_fee_rate, 4.5);
  assert.equal(info.futures_maker_fee_rate, 1.5);
  assert.equal(info.taker_fee_rate, 0);
  assert.equal(info.maker_fee_rate, 0);
  assert.deepEqual((await nativeRequest("/v1/client/info")).data, info);
  for (const symbol of nativeSymbols()) {
    assert.equal(info.imr_factor[symbol.symbol], 0);
    assert.equal(symbol.imr_factor, 0);
    assert.ok(new Decimal(symbol.base_imr).isPositive());
    assert.ok(Number.isFinite(new Decimal(symbol.base_max).toNumber()));
    assert.ok(
      Number.isFinite(new Decimal(info.imr_factor[symbol.symbol]).toNumber()),
    );
  }
  updateNativeSnapshot({ feeTier: feeTierView([4, 280n, 0n, 0n]) });
  assert.equal(nativeAccountInfo().futures_maker_fee_rate, 0);
  updateNativeSnapshot({ feeTier: feeTierView([4, 280n, -10n, 0n]) });
  assert.equal(nativeAccountInfo().futures_maker_fee_rate, -0.1);
  updateNativeSnapshot({ feeTier: undefined });
  assert.equal(nativeAccountInfo().futures_taker_fee_rate, 4.5);
  assert.equal(nativeAccountInfo().futures_maker_fee_rate, 1.5);
  updateNativeSnapshot({ feeTier: feeTierView([0, 450n, 150n, 0n]) });
  updateNativeSnapshot({
    address: "0x0000000000000000000000000000000000000002",
  });
  assert.equal(nativeAccountInfo().futures_taker_fee_rate, 4.5);
}

async function nativeCollateralProducesBoundedBuyAndSellQuantities(
  context: TestContext,
) {
  const original = getNativeSnapshot();
  context.after(() => updateNativeSnapshot(original));
  const { nativeAccountInfo, nativeSymbols, nativePositionRows } =
    await nativeBinding();
  for (const feeTier of [undefined, feeTierView([0, 450n, 150n, 0n])]) {
    updateNativeSnapshot({
      positions: [],
      marginMode: "CROSS",
      feeTier,
      totalVaultUsd: 1000,
      freeCollateral: 1000,
    });
    const info = nativeAccountInfo();
    const symbols = nativeSymbols();
    const symbolInfo = createGetter(
      Object.fromEntries(symbols.map((row) => [row.symbol, row])),
    );
    for (const row of symbols) {
      const price = 3000;
      const leverage = 10;
      const inputs: account.MaxQtyInputs = {
        symbol: row.symbol,
        baseMaxQty: row.base_max,
        totalCollateral: getNativeSnapshot().totalVaultUsd!,
        otherIMs: account.otherIMs({
          positions: nativePositionRows(),
          symbolInfo,
          markPrices: { [row.symbol]: price },
          IMR_Factors: info.imr_factor,
        }),
        maxLeverage: leverage,
        baseIMR: row.base_imr,
        markPrice: price,
        positionQty: 0,
        buyOrdersQty: 0,
        sellOrdersQty: 0,
        IMR_Factor: info.imr_factor[row.symbol],
        takerFeeRate: info.futures_taker_fee_rate,
      };
      for (const quantity of [
        account.maxQtyByLong(inputs),
        account.maxQtyByShort(inputs),
      ]) {
        assert.ok(Number.isFinite(quantity) && quantity > 0);
        assert.ok(quantity <= (inputs.totalCollateral * leverage) / price);
      }
      for (const mode of [MarginMode.CROSS, MarginMode.ISOLATED]) {
        for (const side of [OrderSide.BUY, OrderSide.SELL]) {
          const state = {
            symbol: row.symbol,
            side,
            reduceOnly: false,
            finalMarginMode: mode,
            accountInfo: info,
            symbolInfo,
            positions: nativePositionRows(),
            totalCollateral: 1000,
            freeCollateral: 1000,
            markPrices: { [row.symbol]: price },
            symbolLeverage: leverage,
            symbolOpenOrders: [],
          };
          const quantity = calculateMaxQty(state);
          assert.ok(Number.isFinite(quantity) && quantity > 0);
          assert.ok(quantity <= (1000 * leverage) / price);
          assert.equal(
            calculateMaxQty({
              ...state,
              totalCollateral: 0,
              freeCollateral: 0,
            }),
            0,
          );
        }
      }
      assert.equal(account.maxQtyByLong({ ...inputs, totalCollateral: 0 }), 0);
      assert.equal(account.maxQtyByShort({ ...inputs, totalCollateral: 0 }), 0);
    }
  }
}

async function nativeOpenPositionsAndPendingOrdersKeepQuantitiesFinite(
  context: TestContext,
) {
  const original = getNativeSnapshot();
  context.after(() => updateNativeSnapshot(original));
  const { nativeAccountInfo, nativeSymbols, nativePositionRows } =
    await nativeBinding();
  const rawPosition = (marketId: number, isLong: boolean) => ({
    marketId,
    isLong,
    positionId: String(marketId + 1),
    sizeUsd: parseUnits("300", 18).toString(),
    entryPrice: parseUnits("3000", 18).toString(),
    collateralUsd: parseUnits("30", 18).toString(),
    leverage: parseUnits("10", 18).toString(),
    openedAt: "1",
  });
  const symbols = nativeSymbols();
  const current = symbols[0];
  const other = symbols[1];
  const symbolInfo = createGetter(
    Object.fromEntries(symbols.map((row) => [row.symbol, row])),
  );
  const markPrices = { [current.symbol]: 3000, [other.symbol]: 3000 };
  for (const feeTier of [undefined, feeTierView([0, 450n, 150n, 0n])]) {
    for (const mode of [MarginMode.CROSS, MarginMode.ISOLATED]) {
      for (const isLong of [true, false]) {
        updateNativeSnapshot({
          marginMode: mode,
          feeTier,
          positions: [
            rawPosition(current.marketId, isLong),
            rawPosition(other.marketId, true),
          ],
          markets: [current, other].map((row) => ({
            symbol: row.display_symbol_name,
            price: 3000,
          })),
        });
        const info = nativeAccountInfo();
        const positions = nativePositionRows();
        const otherIMs = account.otherIMs({
          positions: positions.slice(1),
          markPrices,
          symbolInfo,
          IMR_Factors: info.imr_factor,
        });
        assert.equal(otherIMs, 30);
        const inputs: account.MaxQtyInputs = {
          symbol: current.symbol,
          baseMaxQty: current.base_max,
          totalCollateral: 1000,
          otherIMs,
          maxLeverage: 10,
          baseIMR: current.base_imr,
          markPrice: 3000,
          positionQty: positions[0].position_qty,
          buyOrdersQty: 0.01,
          sellOrdersQty: 0.02,
          IMR_Factor: info.imr_factor[current.symbol],
          takerFeeRate: info.futures_taker_fee_rate,
        };
        for (const quantity of [
          account.maxQtyByLong(inputs),
          account.maxQtyByShort(inputs),
        ]) {
          assert.ok(Number.isFinite(quantity) && quantity > 0);
        }
        assert.ok(
          account.maxQtyByLong(inputs) <
            account.maxQtyByLong({ ...inputs, buyOrdersQty: 0 }),
        );
        assert.ok(
          account.maxQtyByShort(inputs) <
            account.maxQtyByShort({ ...inputs, sellOrdersQty: 0 }),
        );
        for (const side of [OrderSide.BUY, OrderSide.SELL]) {
          const state = {
            symbol: current.symbol,
            side,
            reduceOnly: false,
            finalMarginMode: mode,
            positions,
            accountInfo: info,
            symbolInfo,
            totalCollateral: 1000,
            freeCollateral: 900,
            markPrices,
            symbolLeverage: 10,
            symbolOpenOrders: undefined,
          };
          assert.ok(
            Number.isFinite(calculateMaxQty(state)) &&
              calculateMaxQty(state) > 0,
          );
          assert.equal(
            calculateMaxQty({ ...state, reduceOnly: true }),
            (isLong ? side === OrderSide.SELL : side === OrderSide.BUY)
              ? 0.1
              : 0,
          );
          const isolated = account.maxQtyForIsolatedMargin({
            symbol: current.symbol,
            orderSide: side,
            currentOrderReferencePrice: 3000,
            availableBalance: 900,
            leverage: 10,
            baseIMR: current.base_imr,
            IMR_Factor: info.imr_factor[current.symbol],
            markPrice: 3000,
            positionQty: positions[0].position_qty,
            pendingLongOrders: [{ referencePrice: 3000, quantity: 0.01 }],
            pendingSellOrders: [{ referencePrice: 3000, quantity: 0.02 }],
            isoOrderFrozenLong: 0,
            isoOrderFrozenShort: 0,
            isolatedPendingOrders: [
              { side: OrderSide.BUY, referencePrice: 3000, quantity: 0.01 },
              { side: OrderSide.SELL, referencePrice: 3000, quantity: 0.02 },
            ],
            symbolMaxNotional: positionFormulas.maxPositionNotional({
              leverage: 10,
              IMRFactor: info.imr_factor[current.symbol],
            }),
            isoTakerFeeBuffer: 2 * info.futures_taker_fee_rate * 0.0001,
          });
          assert.ok(Number.isFinite(isolated) && isolated > 0);
        }
      }
    }
  }
}

test(nativeAccountFieldsUseFormulaUnits);
test(nativeCollateralProducesBoundedBuyAndSellQuantities);
test(nativeOpenPositionsAndPendingOrdersKeepQuantitiesFinite);
