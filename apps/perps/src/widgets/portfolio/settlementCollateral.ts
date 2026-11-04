import { useQueries } from "@tanstack/react-query";
import { z } from "zod";
import { formatUnits } from "viem";
import { gqlQuery, type GraphQLEndpoint } from "@/core/graphql/client";
import { COLLATERAL_TOKENS, INDEXER_GRAPHQL } from "@/core/perps/config";
import { parseWei } from "@/ui/utils/format";

export const SETTLEMENT_COLLATERAL_QUERY = /* GraphQL */ `
  query PortfolioSettlementCollateral($positionId: String!) {
    position(positionId: $positionId) {
      positionId
      collateralToken
    }
    collateralTokens {
      tokenAddress
      decimals
    }
  }
`;

const address = z.string().regex(/^0x[0-9a-fA-F]{40}$/);
const collateralResult = z.object({
  position: z
    .object({ positionId: z.string(), collateralToken: address.nullable() })
    .nullable(),
  collateralTokens: z.array(
    z.object({
      tokenAddress: address,
      decimals: z.number().int().min(0).max(255),
    }),
  ),
});

export interface SettlementCollateral {
  tokenAddress: string;
  decimals: number;
  symbol: string;
}

export async function fetchSettlementCollateral(
  positionId: string,
  endpoint: GraphQLEndpoint = INDEXER_GRAPHQL,
): Promise<SettlementCollateral> {
  const result = collateralResult.parse(
    await gqlQuery<unknown>(endpoint, SETTLEMENT_COLLATERAL_QUERY, {
      positionId,
    }),
  );
  if (
    result.position?.positionId !== positionId ||
    !result.position.collateralToken
  ) {
    throw new Error(
      `Settlement collateral unavailable for position ${positionId}`,
    );
  }
  const tokenAddress = result.position.collateralToken;
  const token = result.collateralTokens.find(
    (entry) => entry.tokenAddress.toLowerCase() === tokenAddress.toLowerCase(),
  );
  if (!token)
    throw new Error(
      `Settlement token decimals unavailable for ${tokenAddress}`,
    );
  const configured = Object.values(COLLATERAL_TOKENS).find(
    (entry) => entry.address.toLowerCase() === tokenAddress.toLowerCase(),
  );
  return {
    tokenAddress,
    decimals: token.decimals,
    symbol: configured?.symbol ?? tokenAddress,
  };
}

export function useSettlementCollateral(positionIds: readonly string[]) {
  return useQueries({
    queries: positionIds.map((positionId) => ({
      queryKey: ["perps", "settlementCollateral", positionId],
      queryFn: () => fetchSettlementCollateral(positionId),
      staleTime: Infinity,
    })),
  });
}

export function settlementPayout(
  raw: string,
  collateral: SettlementCollateral,
): string {
  return `${formatUnits(parseWei(raw), collateral.decimals)} ${collateral.symbol}`;
}
