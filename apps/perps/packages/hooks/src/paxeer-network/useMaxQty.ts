import { useMemo } from "react";
import { MarginMode, OrderSide, OrderStatus } from "@paxeer-network/types";
import { calculateMaxQty } from "./calculateMaxQty";
import { useAccountInfo } from "./appStore";
import { useCollateral } from "./useCollateral";
import { useLeverageBySymbol } from "./useLeverageBySymbol";
import { useMarkPricesStream } from "./useMarkPricesStream";
import { useOrderStream } from "./useOrderStream/useOrderStream";
import { usePositions } from "./usePositionStream/usePosition.store";
import { useSymbolsInfo } from "./useSymbolsInfo";

/**
 * Options for useMaxQty hook
 */
export interface UseMaxQtyOptions {
  /**
   * Executes buy or sell orders which only reduce a current position.
   * If true, only allows orders that reduce current position
   * @default false
   */
  reduceOnly?: boolean;
  /**
   * Margin mode ("CROSS" or "ISOLATED")
   * @default MarginMode.CROSS
   */
  marginMode?: MarginMode;
  /**
   * Optional reference price for the **new order** when using isolated margin.
   *
   * If provided, this value will be used as `currentOrderReferencePrice`
   * in the isolated-margin max quantity formula instead of the mark price.
   * When omitted, the hook will fall back to `markPrice`.
   */
  currentOrderReferencePrice?: number;
}

/**
 * A hook that calculates the maximum tradeable quantity for a given symbol and side
 * @returns Maximum tradeable quantity
 * @example
 * ```tsx
 * // Get max buy quantity for BTC (backward compatible)
 * const maxBuyQty = useMaxQty("PERP_BTC_USDC", OrderSide.BUY);
 *
 * // Get max sell quantity with reduce only (backward compatible)
 * const maxSellQty = useMaxQty("PERP_BTC_USDC", OrderSide.SELL, true);
 *
 * // New object parameter style (recommended)
 * const maxQty = useMaxQty("PERP_BTC_USDC", OrderSide.BUY, {
 *   reduceOnly: false,
 *   marginMode: MarginMode.ISOLATED,
 * });
 *
 * // Only specify marginMode without reduceOnly
 * const maxQty = useMaxQty("PERP_BTC_USDC", OrderSide.BUY, {
 *   marginMode: MarginMode.ISOLATED,
 * });
 * ```
 */
export function useMaxQty(
  symbol: string,
  side: OrderSide,
  reduceOnly?: boolean,
  marginMode?: MarginMode,
): number;
export function useMaxQty(
  symbol: string,
  side: OrderSide,
  options?: UseMaxQtyOptions,
): number;
export function useMaxQty(
  /**
   * Trading pair symbol (e.g. "PERP_BTC_USDC")
   */
  symbol: string,
  /**
   * Order side ("BUY" or "SELL")
   */
  side: OrderSide,
  /**
   * Either reduceOnly boolean (backward compatible) or options object (recommended)
   */
  reduceOnlyOrOptions?: boolean | UseMaxQtyOptions,
  /**
   * Margin mode (only used when reduceOnlyOrOptions is boolean, for backward compatibility)
   */
  marginMode?: MarginMode,
): number {
  // Normalize parameters: handle both old (positional) and new (object) API
  const reduceOnly: boolean =
    typeof reduceOnlyOrOptions === "object" && reduceOnlyOrOptions !== null
      ? (reduceOnlyOrOptions.reduceOnly ?? false)
      : (reduceOnlyOrOptions ?? false);

  const finalMarginMode: MarginMode =
    typeof reduceOnlyOrOptions === "object" && reduceOnlyOrOptions !== null
      ? (reduceOnlyOrOptions.marginMode ?? MarginMode.CROSS)
      : (marginMode ?? MarginMode.CROSS);

  const currentOrderReferencePrice =
    typeof reduceOnlyOrOptions === "object" &&
    reduceOnlyOrOptions !== null &&
    typeof reduceOnlyOrOptions.currentOrderReferencePrice === "number" &&
    reduceOnlyOrOptions.currentOrderReferencePrice > 0
      ? reduceOnlyOrOptions.currentOrderReferencePrice
      : undefined;

  const positions = usePositions();

  const accountInfo = useAccountInfo();

  const symbolInfo = useSymbolsInfo();

  const { totalCollateral, freeCollateral } = useCollateral();

  const { data: markPrices } = useMarkPricesStream();

  const symbolLeverage = useLeverageBySymbol(symbol, finalMarginMode);

  // Per-order pending data for the isolated frozen simulation. `null` means
  // the stream has not loaded yet and the calculation falls back to the
  // aggregate pending quantities on the position.
  const [symbolOpenOrders] = useOrderStream({
    symbol,
    status: OrderStatus.INCOMPLETE,
    size: 100,
  });

  const maxQty = useMemo(() => {
    return calculateMaxQty({
      symbol,
      side,
      reduceOnly,
      finalMarginMode,
      currentOrderReferencePrice,
      positions,
      accountInfo: accountInfo ?? null,
      symbolInfo,
      totalCollateral,
      freeCollateral,
      markPrices,
      symbolLeverage,
      symbolOpenOrders,
    });
  }, [
    symbol,
    positions,
    reduceOnly,
    markPrices,
    accountInfo,
    symbolInfo,
    side,
    totalCollateral,
    freeCollateral,
    finalMarginMode,
    symbolLeverage,
    currentOrderReferencePrice,
    symbolOpenOrders,
  ]);

  return Math.max(maxQty, 0);
}
