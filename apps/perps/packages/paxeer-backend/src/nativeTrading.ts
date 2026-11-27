import { formatUnits, parseUnits, numberToHex } from "viem";
import {
  PerpsPrecompileABI,
  TimeInForce,
} from "./core/network/abi/PerpsPrecompile";
import Decimal from "decimal.js";
import { PERPS_MARKETS, COLLATERAL_TOKENS } from "./core/perps/config";
import { PRECOMPILES } from "./core/network/precompiles";
import { paxeerChain } from "./core/chain";
import { getConnectedAddress } from "./core/viem/clients";
import { usePosition } from "./hooks/perps/usePosition";
import { ensureAllowance } from "./hooks/spot/useSpotApprovals";
import { usePerpsOrders } from "./hooks/perps/usePerpsOrders";
import { useQuoter } from "./hooks/perps/useQuoter";
import { refreshPositions } from "./hooks/perps/usePortfolioStats";
import { getNativeSnapshot, updateNativeSnapshot } from "./runtime";
import { toNativeSymbol } from "./nativeMetadata";
import {
  planNativeOrder,
  nativeOrderFunding,
  NativeInsufficientBalanceError,
  type NativeOrder,
} from "./nativeOrderPlan";
import type { PerpsCall } from "./core/perps/calls";
import { highestPerpsFeeTier } from "./core/perps/feeTier";
import { writeContract, waitForTx } from "./hooks/perps/useContract";
import { nativeDeposit } from "./nativeClient";
export { planNativeOrder, type NativeOrder } from "./nativeOrderPlan";
export async function executeNativeOrder(
  order: NativeOrder,
  options?: { beforeDeposit?: () => Promise<void> },
) {
  const state = getNativeSnapshot();
  const address = getConnectedAddress();
  if (
    !address ||
    !state.address ||
    address.toLowerCase() !== state.address.toLowerCase()
  )
    throw new Error("Connect your Paxeer wallet before trading.");
  if (state.chainId !== paxeerChain.id)
    throw new Error("Switch to Paxeer Mainnet before trading.");
  const nativeSymbol = toNativeSymbol(order.symbol);
  const live = state.markets.find(
    (row) => row.symbol.toUpperCase() === nativeSymbol,
  );
  const leverage = order.leverage ?? state.leverage[order.symbol] ?? 2;
  if (order.reduce_only) {
    if (
      !["BUY", "SELL"].includes(order.side?.toUpperCase()) ||
      order.order_type_ext ||
      order.visible_quantity !== undefined
    )
      throw new Error("Unsupported close execution instructions.");
    if ((order.order_type ?? order.type ?? "MARKET").toUpperCase() !== "MARKET")
      throw new Error("Use native position TP/SL for a conditional close.");
    const isLong = order.side.toUpperCase() === "SELL";
    const candidates = state.positions.filter(
      (position) =>
        PERPS_MARKETS.find(
          (market) => market.marketId === Number(position.marketId),
        )?.symbol === nativeSymbol &&
        Boolean(position.isLong) === isLong &&
        (!order.native_position_id ||
          String(position.positionId) === String(order.native_position_id)),
    );
    if (candidates.length !== 1)
      throw new Error(
        "Choose the native position to close from the Positions table.",
      );
    const position = candidates[0];
    const quantity = new Decimal(order.order_quantity ?? order.quantity ?? 0);
    const size = new Decimal(position.sizeUsd.toString()).div("1e18");
    const entry = new Decimal(position.entryPrice.toString()).div("1e18");
    const available = size.div(entry);
    if (!quantity.isFinite() || quantity.lte(0))
      throw new Error("Enter a positive close quantity.");
    const whole = Boolean(order.native_close_all) || quantity.eq(available);
    if (quantity.gt(available.plus("0.00000001")))
      throw new Error("Close quantity exceeds this position.");
    await useQuoter().quoteClosePosition(BigInt(position.positionId));
    const receipt = whole
      ? await usePosition().closePosition(BigInt(position.positionId))
      : await usePosition().partialClose(
          BigInt(position.positionId),
          parseUnits(quantity.mul(entry).toFixed(18), 18),
        );
    refreshPositions();
    if (typeof window !== "undefined")
      window.dispatchEvent(new Event("paxeer-network:transaction"));
    return {
      success: true,
      data: { transaction_hash: receipt.transactionHash },
      timestamp: Date.now(),
    };
  }
  const planned = planNativeOrder(order, live?.price ?? Number.NaN, leverage);
  const fundingForAccount = () => {
    const current = getNativeSnapshot();
    if (
      getConnectedAddress()?.toLowerCase() !== address.toLowerCase() ||
      current.address?.toLowerCase() !== address.toLowerCase() ||
      current.chainId !== paxeerChain.id
    )
      throw new Error("Wallet or network changed while preparing the order.");
    const funding = nativeOrderFunding(
      planned,
      current.freeCollateral,
      current.walletBalances.USDC,
      leverage,
      current.feeTier?.takerBps ?? highestPerpsFeeTier.takerBps,
    );
    if (funding.source === "insufficient")
      throw new NativeInsufficientBalanceError();
    return funding;
  };
  let funding = fundingForAccount();
  const bound =
    planned.type === "MARKET" ? takeNativeQuote(order, leverage) : null;
  const reviewedDeposit = funding.source === "split";
  if (reviewedDeposit) {
    await options?.beforeDeposit?.();
    funding = fundingForAccount();
  }
  if (funding.source === "split") {
    await nativeDeposit({
      token: "USDC",
      amount: formatUnits(
        funding.depositAmount,
        COLLATERAL_TOKENS.usdc.decimals,
      ),
      decimals: COLLATERAL_TOKENS.usdc.decimals,
    });
    const current = getNativeSnapshot();
    if (
      getConnectedAddress()?.toLowerCase() !== address.toLowerCase() ||
      current.address?.toLowerCase() !== address.toLowerCase() ||
      current.chainId !== paxeerChain.id
    )
      throw new Error("Wallet or network changed after the deposit.");
  }
  let receipt;
  if (funding.source === "wallet") {
    await ensureAllowance({
      token: planned.collateralToken,
      owner: address,
      spender: PRECOMPILES.perps,
      amount: funding.requiredAmount,
    });
    const tail = [
      planned.sizeUsd,
      planned.leverage,
      planned.collateralToken,
      funding.requiredAmount,
    ] as const;
    const call =
      planned.type === "MARKET"
        ? {
            functionName: "openPosition" as const,
            args: [
              planned.marketId,
              planned.collateralToken,
              funding.requiredAmount,
              planned.leverage,
              planned.isLong,
            ] as const,
          }
        : planned.type === "LIMIT"
          ? {
              functionName: "placeLimitOrder" as const,
              args: [
                planned.marketId,
                planned.isLong,
                planned.limitPrice,
                ...tail,
              ] as const,
            }
          : {
              functionName: "placeStopLimitOrder" as const,
              args: [
                planned.marketId,
                planned.isLong,
                planned.triggerPrice,
                planned.limitPrice,
                ...tail,
              ] as const,
            };
    const contractCall = {
      address: PRECOMPILES.perps,
      abi: PerpsPrecompileABI,
      ...call,
    } satisfies
      | PerpsCall<"openPosition">
      | PerpsCall<"placeLimitOrder">
      | PerpsCall<"placeStopLimitOrder">;
    receipt = await waitForTx(await writeContract(contractCall));
  } else if (bound) {
    const quote = reviewedDeposit
      ? await useQuoter().quoteOpenPosition({
          ...planned,
          account: address,
          collateralAmount: funding.requiredAmount,
        })
      : bound.quote;
    receipt = await usePosition().openPosition({
      marketId: planned.marketId,
      collateralAmount: planned.collateralAmount,
      leverage: planned.leverage,
      isLong: planned.isLong,
      quote: { entryPrice: quote.entryPrice, sizeUsd: planned.sizeUsd },
    });
  } else {
    const input = {
      marketId: planned.marketId,
      isLong: planned.isLong,
      limitPrice: planned.limitPrice,
      sizeUsd: planned.sizeUsd,
      leverage: planned.leverage,
      collateralAmount: planned.collateralAmount,
      timeInForce: TimeInForce.GTC,
      reduceOnly: false,
      cloid: numberToHex(
        (BigInt(Date.now()) << 32n) | BigInt(++orderSequence),
        { size: 16 },
      ),
    };
    receipt =
      planned.type === "LIMIT"
        ? await usePerpsOrders().placeLimitOrder(input)
        : await usePerpsOrders().placeStopLimitOrder({
            ...input,
            triggerPrice: planned.triggerPrice,
          });
  }
  refreshPositions();
  if (typeof window !== "undefined")
    window.dispatchEvent(new Event("paxeer-network:transaction"));
  return {
    success: true,
    data: { transaction_hash: receipt.transactionHash },
    timestamp: Date.now(),
  };
}
export async function cancelNativeOrder(orderId: string) {
  if (!getConnectedAddress())
    throw new Error("Connect your wallet before cancelling an order.");
  const receipt = await usePerpsOrders().cancelOrder(BigInt(orderId));
  if (typeof window !== "undefined")
    window.dispatchEvent(new Event("paxeer-network:transaction"));
  return {
    success: true,
    data: { transaction_hash: receipt.transactionHash },
    timestamp: Date.now(),
  };
}

let orderSequence = 0;

export const NATIVE_QUOTE_MAX_AGE_MS = 15_000;
type BoundNativeQuote = {
  plan: ReturnType<typeof planNativeOrder>;
  quote: import("./hooks/perps/useQuoter").OpenPositionQuote;
  timestamp: number;
  key: string;
  id: string;
};
const quotes = new Map<string, BoundNativeQuote>();
let quoteSequence = 0;
export function nativeOrderKey(order: NativeOrder, leverage: number) {
  const state = getNativeSnapshot();
  return JSON.stringify([
    state.address?.toLowerCase(),
    state.chainId,
    order.symbol,
    order.side?.toUpperCase(),
    order.order_type ?? order.type ?? "MARKET",
    new Decimal(order.order_quantity ?? order.quantity ?? 0).toString(),
    String(order.order_price ?? ""),
    String(order.trigger_price ?? ""),
    leverage,
    Boolean(order.reduce_only),
    order.collateral_token ?? "USDC",
  ]);
}
export async function prepareNativeQuote(
  order: NativeOrder,
  markPrice: number,
  leverage: number,
) {
  const key = nativeOrderKey(order, leverage);
  const plan = planNativeOrder(order, markPrice, leverage);
  const state = getNativeSnapshot();
  const funding = nativeOrderFunding(
    plan,
    state.freeCollateral,
    state.walletBalances.USDC,
    leverage,
    state.feeTier?.takerBps ?? highestPerpsFeeTier.takerBps,
  );
  const quote = await useQuoter().quoteOpenPosition({
    ...plan,
    account: state.address ?? undefined,
    collateralAmount: funding.requiredAmount,
  });
  if (nativeOrderKey(order, leverage) !== key)
    throw new Error("Wallet or network changed while requesting the quote.");
  const result = {
    plan,
    quote,
    timestamp: Date.now(),
    key,
    id: `native-quote-${Date.now()}-${++quoteSequence}`,
  };
  quotes.set(result.id, result);
  // Bound cache, never persist a trade quote between browser sessions.
  for (const [id, value] of quotes)
    if (Date.now() - value.timestamp > NATIVE_QUOTE_MAX_AGE_MS)
      quotes.delete(id);
  return result;
}
export function takeNativeQuote(
  order: NativeOrder,
  leverage: number,
  now = Date.now(),
) {
  const key = nativeOrderKey(order, leverage);
  const bound = order.native_quote_id
    ? quotes.get(order.native_quote_id)
    : undefined;
  if (
    !bound ||
    bound.key !== key ||
    now - bound.timestamp > NATIVE_QUOTE_MAX_AGE_MS
  )
    throw new Error(
      "Wait for a fresh quote matching this order before submitting.",
    );
  quotes.delete(bound.id);
  return bound;
}
