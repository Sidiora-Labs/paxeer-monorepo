import { account, positions as positionsPerp } from "@paxeer-network/perp";
import { type API, MarginMode, OrderSide } from "@paxeer-network/types";
import { resolveIsolatedPendingOrders } from "../utils/order/isolatedPendingOrders";
import type { SymbolsInfo } from "./useSymbolsInfo";

export interface MaxQuantityState {
  symbol: string;
  side: OrderSide;
  reduceOnly: boolean;
  finalMarginMode: MarginMode;
  currentOrderReferencePrice?: number;
  positions: API.Position[] | null;
  accountInfo:
    | (Pick<API.AccountInfo, "imr_factor" | "futures_taker_fee_rate"> & {
        max_notional?: Record<string, number>;
        account_group?: string;
        margin_mode?: "CROSS" | "ISOLATED";
      })
    | null;
  symbolInfo: SymbolsInfo;
  totalCollateral: number;
  freeCollateral: number;
  markPrices: Record<string, number> | null;
  symbolLeverage?: number;
  symbolOpenOrders: Array<API.Order | API.AlgoOrder> | null | undefined;
}

export function calculateMaxQty({
  symbol,
  side,
  reduceOnly,
  finalMarginMode,
  currentOrderReferencePrice,
  positions,
  accountInfo,
  symbolInfo,
  totalCollateral,
  freeCollateral,
  markPrices,
  symbolLeverage,
  symbolOpenOrders,
}: MaxQuantityState): number {
  // Early return for invalid symbol
  if (!symbol) return 0;

  // Early return if required data is not available
  if (!markPrices || !markPrices[symbol] || !accountInfo || !positions) {
    return 0;
  }

  const positionsArray = (positions === null ? [] : positions).filter(
    (position) =>
      (position.margin_mode ??
        (accountInfo.account_group === "PAXEER"
          ? accountInfo.margin_mode
          : undefined)) === finalMarginMode,
  );
  const positionQty = account.getQtyFromPositions(positionsArray, symbol);

  /**
   * Reduce-only mode handling:
   * For long positions (positionQty > 0):
   * - Buy orders return 0 (cannot increase long position)
   * - Sell orders return absolute position quantity (can close long position)
   * For short positions (positionQty < 0):
   * - Buy orders return absolute position quantity (can close short position)
   * - Sell orders return 0 (cannot increase short position)
   */
  if (reduceOnly) {
    if (positionQty === 0) return 0;
    // For long position: BUY returns 0, SELL returns positionQty
    // For short position: BUY returns |positionQty|, SELL returns 0
    return side === OrderSide.BUY
      ? positionQty < 0
        ? Math.abs(positionQty)
        : 0
      : positionQty > 0
        ? positionQty
        : 0;
  }

  const getSymbolInfo = symbolInfo[symbol];

  // Find current symbol position and filter other positions in a single pass
  let currentSymbolPosition: API.Position | undefined;
  const otherPositions: API.Position[] = [];

  for (const position of positionsArray) {
    if (position.symbol === symbol) {
      currentSymbolPosition = position;
    } else {
      otherPositions.push(position);
    }
  }

  // Extract common parameters used by both maxQty and maxQtyForIsolatedMargin
  const markPrice = markPrices[symbol];

  const baseIMR = getSymbolInfo("base_imr") ?? 0;
  const IMR_Factor = accountInfo.imr_factor[symbol] ?? 0;
  const leverage = symbolLeverage || currentSymbolPosition?.leverage || 1;

  // current symbol buy/sell order quantities
  const buyOrdersQty = currentSymbolPosition?.pending_long_qty ?? 0;
  const sellOrdersQty = currentSymbolPosition?.pending_short_qty ?? 0;

  if (finalMarginMode === MarginMode.ISOLATED) {
    const availableBalance = freeCollateral;

    // Per-order data when the order stream is trusted; otherwise
    // approximate the aggregate pending quantities at mark price. Either
    // way the flip binary search runs the risk-engine close/open
    // allocation, never the legacy full-notional formula.
    const isolatedPendingOrders = resolveIsolatedPendingOrders(
      symbolOpenOrders,
      {
        symbol,
        fallbackPrice: markPrice,
        pendingLongQty: buyOrdersQty,
        pendingShortQty: sellOrdersQty,
      },
    ) ?? [
      ...(buyOrdersQty > 0
        ? [
            {
              side: OrderSide.BUY,
              referencePrice: markPrice,
              quantity: buyOrdersQty,
            },
          ]
        : []),
      ...(sellOrdersQty > 0
        ? [
            {
              side: OrderSide.SELL,
              referencePrice: markPrice,
              quantity: sellOrdersQty,
            },
          ]
        : []),
    ];

    // Build pending orders arrays for the simplified same-direction
    // formula (per-order prices when the stream is trusted).
    const pendingLongOrders = isolatedPendingOrders
      .filter((order) => order.side === OrderSide.BUY)
      .map(({ referencePrice, quantity }) => ({
        referencePrice,
        quantity,
      }));

    const pendingSellOrders = isolatedPendingOrders
      .filter((order) => order.side === OrderSide.SELL)
      .map(({ referencePrice, quantity }) => ({
        referencePrice,
        quantity,
      }));

    // Legacy aggregate-frozen inputs, ignored by the allocation-based
    // binary search; kept only for the maxQtyForIsolatedMargin API shape.
    const isoOrderFrozenLong = 0;
    const isoOrderFrozenShort = 0;

    // Get or calculate symbolMaxNotional
    // Priority: accountInfo.max_notional[symbol] > calculated maxPositionNotional
    const symbolMaxNotional =
      accountInfo.max_notional?.[symbol] ??
      positionsPerp.maxPositionNotional({
        leverage,
        IMRFactor: IMR_Factor,
      });

    const effectiveOrderReferencePrice =
      currentOrderReferencePrice ?? markPrice;

    return account.maxQtyForIsolatedMargin({
      ...(accountInfo.account_group === "PAXEER"
        ? { isoTakerFeeBuffer: 2 * accountInfo.futures_taker_fee_rate * 0.0001 }
        : {}),
      symbol,
      orderSide: side,
      currentOrderReferencePrice: effectiveOrderReferencePrice,
      availableBalance,
      leverage,
      baseIMR,
      IMR_Factor,
      markPrice,
      positionQty,
      pendingLongOrders,
      pendingSellOrders,
      isoOrderFrozenLong,
      isoOrderFrozenShort,
      isolatedPendingOrders,
      symbolMaxNotional,
    });
  }

  // Only calculate otherIMs for CROSS margin mode (not needed for ISOLATED)
  const otherIMs = account.otherIMs({
    positions: otherPositions,
    symbolInfo,
    markPrices,
    IMR_Factors: accountInfo.imr_factor,
  });

  return Math.max(
    0,
    account.maxQty(side, {
      markPrice,
      symbol,
      baseMaxQty: getSymbolInfo("base_max"),
      totalCollateral,
      maxLeverage: leverage,
      takerFeeRate: accountInfo.futures_taker_fee_rate,
      baseIMR,
      otherIMs,
      positionQty,
      buyOrdersQty,
      sellOrdersQty,
      IMR_Factor,
    }),
  );
}
