import { useEffect, useState, useSyncExternalStore } from "react";
import {
  getNativeSnapshot,
  subscribeNativeSnapshot,
} from "@paxeer-network/paxeer-backend/runtime";
import {
  prepareNativeQuote,
  executeNativeOrder,
  nativeOrderKey,
  NATIVE_QUOTE_MAX_AGE_MS,
  planNativeOrder,
} from "@paxeer-network/paxeer-backend/nativeTrading";
import { formatUnits } from "viem";
import {
  nativeOrderCapacity,
  nativeOrderFunding,
  NativeInsufficientBalanceError,
} from "@paxeer-network/paxeer-backend/nativeOrderPlan";
import { highestPerpsFeeTier } from "@paxeer-network/paxeer-backend/core/perps/feeTier";
import { toNativeSymbol } from "@paxeer-network/paxeer-backend/nativeMetadata";
import { classifyError } from "@paxeer-network/paxeer-backend/core/errors/classify";
export function useNativeOrderEntry(
  order: any,
  selectedLeverage: number | undefined,
  reset: () => void,
  beforeDeposit?: () => Promise<void>,
) {
  const state = useSyncExternalStore(
    subscribeNativeSnapshot,
    getNativeSnapshot,
    getNativeSnapshot,
  );
  const leverage = state.leverage[order.symbol] ?? selectedLeverage ?? 2;
  const [quote, setQuote] = useState<Awaited<
    ReturnType<typeof prepareNativeQuote>
  > | null>(null);
  const [isMutating, setIsMutating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tick, setTick] = useState(0);
  const markPrice =
    state.markets.find(
      (row) => row.symbol === toNativeSymbol(order.symbol ?? ""),
    )?.price ?? Number.NaN;
  let key = "";
  try {
    key = nativeOrderKey(order, leverage);
  } catch {}
  useEffect(() => {
    const timer = setInterval(() => setTick((t) => t + 1), 10_000);
    return () => clearInterval(timer);
  }, []);
  useEffect(() => {
    let cancelled = false;
    setQuote(null);
    setError(null);
    if (
      !state.address ||
      !Number(order.order_quantity) ||
      !Number.isFinite(markPrice) ||
      order.reduce_only
    )
      return;
    const timer = setTimeout(() => {
      void prepareNativeQuote(order, markPrice, leverage)
        .then((value) => {
          if (!cancelled) setQuote(value);
        })
        .catch((reason) => {
          if (!cancelled) {
            const classified = classifyError(reason);
            setError(
              classified.code !== "UNKNOWN"
                ? classified.message
                : reason instanceof Error
                  ? reason.message
                  : "Quote unavailable.",
            );
          }
        });
    }, 300);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [key, markPrice, tick]);
  const current =
    quote?.key === key &&
    Date.now() - quote.timestamp <= NATIVE_QUOTE_MAX_AGE_MS
      ? quote
      : null;
  const validate = async () => {
    const plan = planNativeOrder(order, markPrice, leverage);
    if (!order.reduce_only) {
      const current = getNativeSnapshot();
      const funding = nativeOrderFunding(
        plan,
        current.freeCollateral,
        current.walletBalances.USDC,
        leverage,
        current.feeTier?.takerBps ?? highestPerpsFeeTier.takerBps,
      );
      if (funding.source === "insufficient")
        throw new NativeInsufficientBalanceError();
    }
    return order;
  };
  const submit = async (options?: { resetOnSuccess?: boolean }) => {
    await validate();
    if (
      error &&
      !order.reduce_only &&
      (order.order_type ?? "MARKET") === "MARKET"
    )
      throw new Error(error);
    if (
      !order.reduce_only &&
      (order.order_type ?? "MARKET") === "MARKET" &&
      !current
    )
      throw new Error("Wait for a fresh Paxeer quote before submitting.");
    if (order.tp_trigger_price || order.sl_trigger_price)
      throw new Error(
        "Attach TP/SL to the confirmed native position after opening it.",
      );
    setIsMutating(true);
    try {
      const result = await executeNativeOrder(
        {
          ...order,
          leverage,
          native_quote_id: current?.id,
        },
        { beforeDeposit },
      );
      if (options?.resetOnSuccess !== false) reset();
      return result;
    } finally {
      setIsMutating(false);
    }
  };
  const referencePrice =
    (order.order_type ?? order.type ?? "MARKET").toUpperCase() === "MARKET"
      ? markPrice
      : Number(order.order_price) || markPrice;
  const { freeCollateral, maxQty } = nativeOrderCapacity(
    state.freeCollateral,
    state.walletBalances.USDC,
    leverage,
    state.feeTier?.takerBps ?? highestPerpsFeeTier.takerBps,
    referencePrice,
  );
  const estLiqPrice = current
    ? Number(formatUnits(current.quote.liquidationPrice, 18))
    : null;
  const estSlippage = current
    ? Number(formatUnits(current.quote.priceImpact, 18))
    : null;
  return {
    submit,
    helper: {
      validate,
      validator: validate,
      getProjectedUSDCBorrow: () => null,
    },
    isMutating,
    freeCollateral,
    maxQty,
    maxQtys: { maxBuy: maxQty, maxSell: maxQty },
    estLiqPrice,
    estLiqPriceDistance:
      estLiqPrice && markPrice
        ? Math.abs(estLiqPrice - markPrice) / markPrice
        : null,
    estSlippage,
    estLeverage: leverage,
    symbolLeverage: leverage,
  };
}
