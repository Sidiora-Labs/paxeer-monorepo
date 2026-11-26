import { isAddress, zeroAddress, type Address } from "viem";
import * as calls from "@paxeer-network/paxeer-backend/core/perps/calls";
import { transaction, type TradingCall } from "./execution.js";
import { AgentKitError } from "./result.js";

export interface TradingCaps {
  ownerAddress: Address;
  maxTxValueWei: bigint;
  maxDailyValueWei: bigint;
  maxApproveWei: bigint;
  budgetWei: bigint;
  expiresInSeconds: number;
  rateLimitPerMin: number;
}

/** Owner route payloads for the trading workspace's contracts and functions. */
export async function buildTradingPreset(caps: TradingCaps) {
  if (
    !isAddress(caps.ownerAddress) ||
    caps.ownerAddress.toLowerCase() === zeroAddress ||
    [
      caps.maxTxValueWei,
      caps.maxDailyValueWei,
      caps.maxApproveWei,
      caps.budgetWei,
    ].some((value) => value < 0n || value >= 1n << 256n) ||
    caps.maxTxValueWei > caps.maxDailyValueWei ||
    caps.maxApproveWei === 0n ||
    caps.budgetWei === 0n ||
    !Number.isInteger(caps.expiresInSeconds) ||
    caps.expiresInSeconds <= 0 ||
    caps.expiresInSeconds > 31_536_000 ||
    !Number.isInteger(caps.rateLimitPerMin) ||
    caps.rateLimitPerMin <= 0 ||
    caps.rateLimitPerMin > 10_000
  ) {
    throw new AgentKitError(
      "INVALID_CAPS",
      "Owner address, ordered value caps, positive approval and budget caps, and bounded expiry and rate are required",
    );
  }
  const depositPlan = await calls.buildDepositPlan(
    1n,
    async () => 0n,
    new Set(),
  );
  const approval = depositPlan.find((call) => call.functionName === "approve");
  if (!approval)
    throw new AgentKitError(
      "INVALID_PLAN",
      "The USDL deposit plan must declare its approval",
    );
  const perpsCalls: TradingCall[] = [
    calls.buildDeposit(1n),
    calls.buildWithdraw(1n),
    calls.buildOpenPosition({
      marketId: 0n,
      isLong: true,
      collateralAmount: 1n,
      leverage: 10n ** 18n,
      quote: { sizeUsd: 10n ** 18n, entryPrice: 10n ** 18n },
    }),
    calls.buildCancelOrder(0n),
    calls.buildSetPositionTpSl({
      positionId: 0n,
      isLong: true,
      takeProfitPrice: 0n,
      stopLossPrice: 0n,
    }),
    calls.buildAddCollateral(0n, 1n),
    calls.buildRemoveCollateral(0n, 1n),
    calls.buildClosePosition({
      positionId: 0n,
      isLong: true,
      exitPrice: 10n ** 18n,
    }),
  ];
  const txs = [...perpsCalls, approval].map(transaction);
  const contracts = [...new Set(txs.map((tx) => tx.to.toLowerCase()))];
  const selectors = [...new Set(txs.map((tx) => tx.data.slice(0, 10)))];
  return {
    policy: {
      mode: "trade_only" as const,
      max_tx_value_wei: caps.maxTxValueWei.toString(),
      max_daily_value_wei: caps.maxDailyValueWei.toString(),
      max_approve_wei: caps.maxApproveWei.toString(),
      rate_limit_per_min: caps.rateLimitPerMin,
      allow_native_transfer: false,
      withdrawal_allowlist_only: true,
      daily_reset_utc_hour: 0,
    },
    rules: [
      ...contracts.map((value) => ({
        effect: "allow" as const,
        subject: "contract" as const,
        value,
      })),
      ...selectors.map((value) => ({
        effect: "allow" as const,
        subject: "selector" as const,
        value,
      })),
      {
        effect: "allow" as const,
        subject: "token" as const,
        value: approval.address.toLowerCase(),
        max_value_wei: caps.maxApproveWei.toString(),
      },
      {
        effect: "allow" as const,
        subject: "withdrawal" as const,
        value: caps.ownerAddress.toLowerCase(),
      },
    ],
    budget: {
      target_contract: perpsCalls[0].address,
      token: approval.address,
      cap_wei: caps.budgetWei.toString(),
      expires_in_seconds: caps.expiresInSeconds,
    },
    approvalSpenders: [approval.args[0]],
  };
}
