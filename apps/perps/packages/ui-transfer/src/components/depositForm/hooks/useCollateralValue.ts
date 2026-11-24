import { useMemo } from "react";
import { useComputedLTV } from "@paxeer-network/hooks";
import { account as accountPerp } from "@paxeer-network/perp";
import { paxeerChain } from "@paxeer-network/paxeer-backend/core/chain";
import { Decimal } from "@paxeer-network/utils";
import { API } from "@paxeer-network/types";

const { collateralRatio, collateralContribution } = accountPerp;

export const useCollateralValue = (params: {
  chainId?: number;
  sourceToken?: API.TokenInfo;
  targetToken?: API.TokenInfo;
  quantity: string;
  indexPrice?: number;
}) => {
  const { sourceToken, targetToken, indexPrice } = params;

  const quantity = Number(params.quantity);
  const isPaxeer = params.chainId === paxeerChain.id;

  const memoizedCollateralRatio = useMemo(() => {
    if (isPaxeer) return new Decimal(1);
    return collateralRatio({
      baseWeight: targetToken?.base_weight ?? 0,
      discountFactor: targetToken?.discount_factor ?? 0,
      collateralQty: quantity,
      collateralCap: sourceToken?.user_max_qty ?? quantity,
      indexPrice: indexPrice ?? 0,
    });
  }, [isPaxeer, targetToken, quantity, sourceToken?.user_max_qty, indexPrice]);

  const collateralContributionQuantity = useMemo(() => {
    if (isPaxeer) return quantity;
    return collateralContribution({
      collateralQty: quantity,
      collateralCap: sourceToken?.user_max_qty ?? quantity,
      collateralRatio: memoizedCollateralRatio.toNumber(),
      indexPrice: indexPrice ?? 0,
    });
  }, [
    isPaxeer,
    quantity,
    sourceToken?.user_max_qty,
    memoizedCollateralRatio,
    indexPrice,
  ]);

  const currentLTV = useComputedLTV();

  const nextLTV = useComputedLTV({
    input: quantity,
    token: sourceToken?.symbol,
  });

  return {
    collateralRatio: memoizedCollateralRatio.toNumber(),
    collateralContributionQuantity,
    currentLTV: isPaxeer ? 0 : currentLTV,
    nextLTV: isPaxeer ? 0 : nextLTV,
  };
};
