import assert from "node:assert/strict";
import type { PublicClient } from "viem";
import type { OrderFormState } from "../../src/core/perps/orderPlan";

export async function configuredLimitPlan(
  publicClient: Pick<PublicClient, "readContract">,
  state: OrderFormState,
) {
  const calls = await import("../../src/core/perps/calls");
  const { decodeMarketConfig } = await import("../../src/core/perps/marketConfig");
  const { orderLimitsFromConfig } = await import("../../src/widgets/trading/PerpsOrderForm");
  const { buildOrderPlan } = await import("../../src/core/perps/orderPlan");
  const [rawConfig, tiers] = await Promise.all([
    publicClient.readContract(calls.buildGetMarketConfig(state.marketId)),
    publicClient.readContract(calls.buildGetMarginTiers(state.marketId)),
  ]);
  const limits = orderLimitsFromConfig({ config: decodeMarketConfig(rawConfig), tiers },
    (await import("viem")).parseUnits(state.sizeUsd, 18));
  assert.ok(limits, "the market must be enabled");
  return buildOrderPlan(state, null, limits);
}
