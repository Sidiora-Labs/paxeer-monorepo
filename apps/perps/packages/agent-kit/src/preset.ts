import { isAddress, zeroAddress, type Address } from "viem";
import {
  buildPerpsAgentRules,
  PERPS_AGENT_APPROVAL,
} from "@paxeer-network/paxeer-backend/core/wallet/embedded/agentPreset";
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
    rules: buildPerpsAgentRules(
      caps.ownerAddress,
      caps.maxApproveWei.toString(),
    ).map((rule) => ({ ...rule, value: rule.value.toLowerCase() })),
    budget: {
      target_contract: PERPS_AGENT_APPROVAL.spender,
      token: PERPS_AGENT_APPROVAL.contract,
      cap_wei: caps.budgetWei.toString(),
      expires_in_seconds: caps.expiresInSeconds,
    },
    approvalSpenders: [PERPS_AGENT_APPROVAL.spender],
  };
}
