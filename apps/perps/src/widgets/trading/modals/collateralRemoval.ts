import { queryOptions } from "@tanstack/react-query";
import { BaseError, ContractFunctionRevertedError, type Address, type PublicClient } from "viem";
import {
  buildGetAccount,
  buildGetMarginTiers,
  buildGetPositionDetails,
  buildRemoveCollateral,
} from "@/core/perps/calls";
import { COLLATERAL_TOKENS } from "@/core/perps/config";
import { maintenanceMargin } from "@/core/perps/marketConfig";
import { getPublicClient } from "@/core/viem/clients";

const USD_PER_COLLATERAL_UNIT = 10n ** BigInt(18 - COLLATERAL_TOKENS.usdl.decimals);

export interface CollateralRemovalRisk {
  positionId: bigint;
  account: Address;
  blockNumber: bigint;
  equityUsd: bigint;
  maintenanceMarginUsd: bigint;
  marketMaintenanceUsd: bigint;
  maxRemoval: bigint;
}

export async function readCollateralRemovalRisk(
  positionId: bigint,
  account: Address,
  client: PublicClient = getPublicClient(),
): Promise<CollateralRemovalRisk> {
  const blockNumber = await client.getBlockNumber({ cacheTime: 0 });
  const [position, state] = await Promise.all([
    client.readContract({ ...buildGetPositionDetails(positionId), blockNumber }),
    client.readContract({ ...buildGetAccount(account), blockNumber }),
  ]);
  if (
    position.positionId !== positionId ||
    position.user.toLowerCase() !== account.toLowerCase() ||
    position.sizeUsdAtMark <= 0n
  ) {
    throw new Error("Position is unavailable for this account.");
  }
  const tiers = await client.readContract({
    ...buildGetMarginTiers(position.marketId),
    blockNumber,
  });
  if (
    tiers.length === 0 || tiers[0].lowerBoundUsd !== 0n ||
    tiers.some((tier, index) =>
      tier.maintenanceBps > 10_000n ||
      (index > 0 && tier.lowerBoundUsd <= tiers[index - 1].lowerBoundUsd),
    )
  ) {
    throw new Error("Market maintenance tiers are unavailable.");
  }
  const marketMaintenanceUsd = maintenanceMargin(position.sizeUsdAtMark, tiers);
  if (marketMaintenanceUsd < 0n) throw new Error("Market maintenance is invalid.");
  const maintenanceMarginUsd = state.maintenanceMarginUsd > marketMaintenanceUsd
    ? state.maintenanceMarginUsd : marketMaintenanceUsd;
  const headroom = state.equityUsd - maintenanceMarginUsd;
  const equityLimit = headroom > 0n ? (headroom - 1n) / USD_PER_COLLATERAL_UNIT : 0n;
  // Withdrawable includes account-wide initial margin, order reserves and the
  // governed floor. Stored collateral is pre-settlement and excludes profits.
  let maxRemoval = equityLimit < state.withdrawable ? equityLimit : state.withdrawable;
  async function admits(amount: bigint) {
    try {
      // eth_call executes settlement and its per-position native-unit rounding
      // without writing state. Pin it to the same block as every risk read.
      await client.simulateContract({
        ...buildRemoveCollateral(positionId, amount), account, blockNumber,
      });
      return true;
    } catch (error) {
      const revert = error instanceof BaseError
        ? error.walk((cause) => cause instanceof ContractFunctionRevertedError)
        : undefined;
      if (revert instanceof ContractFunctionRevertedError &&
          revert.data?.errorName === "PerpsInsufficientMargin") return false;
      // Transport failures, unavailable prices and all other reverts disable
      // removal rather than masquerading as a smaller available balance.
      throw error;
    }
  }
  if (maxRemoval > 0n && !await admits(maxRemoval)) {
    let lower = 0n;
    let upper = maxRemoval;
    while (upper - lower > 1n) {
      const middle = (lower + upper) / 2n;
      if (await admits(middle)) lower = middle;
      else upper = middle;
    }
    maxRemoval = lower;
  }
  return {
    positionId, account, blockNumber, equityUsd: state.equityUsd,
    maintenanceMarginUsd, marketMaintenanceUsd, maxRemoval,
  };
}

export function collateralRemovalQueryOptions(positionId: bigint, account: Address) {
  return queryOptions({
    queryKey: ["perps", "collateralRemoval", positionId.toString(), account],
    queryFn: () => readCollateralRemovalRisk(positionId, account),
    retry: false,
    staleTime: 0,
    refetchInterval: 15_000,
  });
}

export function canRemoveCollateral(risk: CollateralRemovalRisk | undefined, amount: bigint) {
  return risk !== undefined && amount > 0n && amount <= risk.maxRemoval;
}

export function remainingRemovalEquity(risk: CollateralRemovalRisk, amount: bigint) {
  return risk.equityUsd - amount * USD_PER_COLLATERAL_UNIT;
}

export async function prepareCollateralRemoval(
  positionId: bigint,
  amount: bigint,
  account: Address,
  client: PublicClient = getPublicClient(),
) {
  const risk = await readCollateralRemovalRisk(positionId, account, client);
  if (!canRemoveCollateral(risk, amount)) {
    throw new Error("Removal exceeds current account margin or withdrawable collateral.");
  }
  return buildRemoveCollateral(positionId, amount);
}
