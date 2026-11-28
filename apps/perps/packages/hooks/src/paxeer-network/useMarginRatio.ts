import { useMemo } from "react";
import { account } from "@paxeer-network/perp";
import { AccountStatusEnum, EMPTY_LIST } from "@paxeer-network/types";
import { zero } from "@paxeer-network/utils";
import { useAccount } from "../useAccount";
import { useCollateral } from "./useCollateral";
import { useMarkPricesStream } from "./useMarkPricesStream";
import { usePositionStore } from "./usePositionStream/usePosition.store";

/**
 * The return type of useMarginRatio hook
 */
export type MarginRatioReturn = {
  /**
   * Current leverage of the account, null if trading is not enabled
   */
  currentLeverage: number | null;
  /**
   * Current margin ratio of the account
   */
  marginRatio: number;
  /**
   * Maintenance margin ratio (MMR) of the account, null if user has no positions
   */
  mmr: number | null;
  /**
   * Maintenance margin of the account
   */
  maintenanceMargin: number | null;
};

export function sumPositionMaintenanceMargin(
  rows?: readonly { mm?: unknown }[] | null,
): number | null {
  if (!rows) {
    return null;
  }

  let sum = zero;
  let hasValue = false;
  for (const row of rows) {
    if (typeof row.mm === "number" && Number.isFinite(row.mm)) {
      sum = sum.add(row.mm);
      hasValue = true;
    }
  }

  return hasValue ? sum.toNumber() : null;
}

/**
 * Hook to calculate and monitor account's margin ratio, leverage, and maintenance margin ratio (MMR)
 * @example
 * ```typescript
 * const { marginRatio, currentLeverage, mmr } = useMarginRatio();
 * ```
 */
export const useMarginRatio = (): MarginRatioReturn => {
  // const [{ rows, aggregated }] = usePositionStream();

  const positions = usePositionStore((state) => state.positions.all);

  const { rows, notional } = positions;

  const { state } = useAccount();

  const { data: markPrices } = useMarkPricesStream();
  //
  // const markPrices = useMarkPrices();
  //

  const { totalCollateral } = useCollateral();

  /**
   * Calculate the total margin ratio based on collateral, mark prices and positions
   */
  const marginRatio = useMemo(() => {
    if (!rows || !markPrices || !totalCollateral || rows.length === 0) {
      return 0;
    }

    return account.totalMarginRatio({
      totalCollateral: totalCollateral,
      markPrices: markPrices,
      positions: rows ?? EMPTY_LIST,
    });
  }, [rows, markPrices, totalCollateral]);

  /**
   * Calculate current leverage based on margin ratio
   * Returns null if trading is not enabled
   */
  const currentLeverage = useMemo(() => {
    if (
      state.status >= AccountStatusEnum.EnableTrading ||
      state.status === AccountStatusEnum.EnableTradingWithoutConnected
    ) {
      return account.currentLeverage(marginRatio);
    }

    return null;
  }, [marginRatio, state.status]);

  /**
   * Calculate maintenance margin ratio (MMR) based on positions
   * Returns null if user has no positions
   */
  const mmr = useMemo<number | null>(() => {
    if (!rows || rows.length <= 0 || notional == null) {
      return null;
    }
    const positionsMM = sumPositionMaintenanceMargin(rows);
    if (positionsMM === null) {
      return null;
    }

    return account.MMR({
      positionsMMR: positionsMM,
      positionsNotional: notional,
    });
  }, [rows, notional]);

  /**
   * Calculate Maintenance Margin based on positions
   * Returns null if user has no positions
   */
  const maintenanceMargin = useMemo<number | null>(() => {
    if (!rows || rows.length <= 0 || notional == null) {
      return null;
    }
    return sumPositionMaintenanceMargin(rows);
  }, [rows, notional]);

  return { marginRatio, currentLeverage, mmr, maintenanceMargin };
};
