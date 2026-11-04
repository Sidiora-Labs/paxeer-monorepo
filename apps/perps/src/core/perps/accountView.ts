import { formatUnits, type Address } from "viem";

import type { AccountState } from "./calls";
import { COLLATERAL_TOKENS } from "./config";

/** The cross-margin account of getAccount(account) at 0x1015. */
export interface AccountView {
  /** USDL base units (6 decimals). */
  collateral: bigint;
  /** 18 decimals; negative when losses exceed collateral. */
  equityUsd: bigint;
  /** 18 decimals. */
  initialMarginUsd: bigint;
  /** 18 decimals. */
  maintenanceMarginUsd: bigint;
  /** USDL base units (6 decimals); the amount a withdraw-all requests. */
  withdrawable: bigint;
}

export function accountViewFromState(accountState: AccountState): AccountView {
  return {
    collateral: accountState.collateral,
    equityUsd: accountState.equityUsd,
    initialMarginUsd: accountState.initialMarginUsd,
    maintenanceMarginUsd: accountState.maintenanceMarginUsd,
    withdrawable: accountState.withdrawable,
  };
}

/** The collateral the account trades on: USDL, the single collateral token. */
export interface PickedCollateral {
  token: Address;
  symbol: string;
  decimals: number;
  /** USDL base units (6 decimals) of getAccount(account).collateral. */
  collateral: bigint;
}

export function pickCollateral(accountState: Pick<AccountState, "collateral">): PickedCollateral {
  const { address, symbol, decimals } = COLLATERAL_TOKENS.usdl;
  return { token: address, symbol, decimals, collateral: accountState.collateral };
}

/** Account-wide risk from the current getAccount snapshot, with no single liquidation price. */
export interface AccountRiskView {
  equityUsd: bigint;
  initialMarginUsd: bigint;
  maintenanceMarginUsd: bigint;
  headroomUsd: bigint;
  /** Maintenance / equity in basis points; null for non-positive equity. */
  maintenanceUsageBps: bigint | null;
  status: "above-maintenance" | "at-maintenance" | "below-maintenance";
  display: {
    label: "Cross-margin account";
    equity: string;
    initialMargin: string;
    maintenanceMargin: string;
    headroom: string;
    maintenanceUsage: string;
  };
}

export function accountRiskFromView(account: AccountView): AccountRiskView {
  const { equityUsd, initialMarginUsd, maintenanceMarginUsd } = account;
  const headroomUsd = equityUsd - maintenanceMarginUsd;
  const maintenanceUsageBps = equityUsd > 0n ? (maintenanceMarginUsd * 10_000n) / equityUsd : null;
  const usd = (value: bigint) => `${formatUnits(value, 18)} USD`;
  return {
    equityUsd,
    initialMarginUsd,
    maintenanceMarginUsd,
    headroomUsd,
    maintenanceUsageBps,
    status: headroomUsd < 0n
      ? "below-maintenance"
      : headroomUsd === 0n ? "at-maintenance" : "above-maintenance",
    display: {
      label: "Cross-margin account",
      equity: usd(equityUsd),
      initialMargin: usd(initialMarginUsd),
      maintenanceMargin: usd(maintenanceMarginUsd),
      headroom: usd(headroomUsd),
      maintenanceUsage: maintenanceUsageBps === null
        ? "Unavailable (non-positive equity)"
        : `${formatUnits(maintenanceUsageBps, 2)}%`,
    },
  };
}
