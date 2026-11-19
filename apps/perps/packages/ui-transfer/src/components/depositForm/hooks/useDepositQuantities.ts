import { useMemo } from "react";
import { paxeerChain } from "@paxeer-network/paxeer-backend/core/chain";
import { API } from "@paxeer-network/types";
import { Decimal, formatWithPrecision } from "@paxeer-network/utils";
import { useMaxDepositAmount } from "./useMaxDepositAmount";

export type GetIndexPrice = (symbol: string) => number | undefined;

export type UseDepositFormQuantitiesParams = {
  chainId?: number;
  sourceToken?: API.TokenInfo;
  targetToken?: API.TokenInfo;
  balance?: string;
  quantity: string;
  needSwap: boolean;
  swapQuantity?: string;
  swapPrice?: number;
  getIndexPrice: GetIndexPrice;
};

export const useDepositFormQuantities = ({
  chainId,
  sourceToken,
  targetToken,
  balance,
  quantity,
  needSwap,
  swapQuantity,
  swapPrice,
  getIndexPrice,
}: UseDepositFormQuantitiesParams) => {
  const maxQuantity = useMemo(
    () =>
      new Decimal(balance || 0)
        .todp(sourceToken?.precision ?? 2, Decimal.ROUND_DOWN)
        .toString(),
    [balance, sourceToken?.precision],
  );

  const maxDepositAmount = useMaxDepositAmount(sourceToken, balance);

  const targetQuantity = useMemo(() => {
    if (needSwap) {
      return swapQuantity
        ? new Decimal(swapQuantity)
            ?.todp(targetToken?.precision ?? 6, Decimal.ROUND_DOWN)
            .toString()
        : undefined;
    }
    return quantity;
  }, [needSwap, swapQuantity, quantity, targetToken?.precision]);

  const swapPriceInUSD = useMemo(() => {
    if (swapPrice && targetToken?.symbol) {
      const indexPrice = getIndexPrice(targetToken.symbol);
      if (!indexPrice) return undefined;
      const num = new Decimal(swapPrice).mul(indexPrice);
      return formatWithPrecision(num, 2);
    }
  }, [swapPrice, targetToken?.symbol, getIndexPrice]);

  const quantityNotional = useMemo(() => {
    if (chainId === paxeerChain.id) return Number(quantity || 0);
    const indexPrice = needSwap
      ? swapPrice
      : getIndexPrice(sourceToken?.symbol!);

    if (quantity && indexPrice) {
      return new Decimal(quantity).mul(indexPrice).toNumber();
    }
    return 0;
  }, [
    chainId,
    quantity,
    sourceToken?.symbol,
    swapPrice,
    needSwap,
    getIndexPrice,
  ]);

  const indexPrice = useMemo(() => {
    return getIndexPrice(sourceToken?.symbol!);
    // update by quantity changes (revalidate when quantity changes)
  }, [quantity, sourceToken?.symbol]);

  const swapIndexPrice = useMemo(() => {
    return getIndexPrice(sourceToken?.symbol!);
    // update by swapPrice changes (revalidate when swapPrice changes)
  }, [swapPrice, sourceToken?.symbol]);

  return {
    maxQuantity,
    maxDepositAmount,
    targetQuantity,
    swapPriceInUSD,
    quantityNotional,
    indexPrice,
    swapIndexPrice,
  };
};
