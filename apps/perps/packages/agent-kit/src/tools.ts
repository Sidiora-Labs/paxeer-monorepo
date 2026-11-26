import type { z } from "zod";
import { AGENT_TOOLS, orderArgs, type AgentToolDefinition } from "./catalog.js";
import { parseUnits } from "viem";
import * as calls from "@paxeer-network/paxeer-backend/core/perps/calls";
import { COLLATERAL_TOKENS } from "@paxeer-network/paxeer-backend/core/perps/config";
import { deriveImpliedCollateral } from "@paxeer-network/paxeer-backend/core/perps/amounts";
import { maxLeverageForNotional } from "@paxeer-network/paxeer-backend/core/perps/marketConfig";
import { partialCloseSize } from "@paxeer-network/paxeer-backend/core/perps/positionActions";
import {
  planClose,
  planPartialClose,
  planCollateral,
  planOpen,
  planOrder,
} from "@paxeer-network/paxeer-backend/core/perps/positionWrites";
import { PERPS_SPENDER } from "@paxeer-network/paxeer-backend/core/network/approvals";
import { TimeInForce } from "@paxeer-network/paxeer-backend/core/network/abi/PerpsPrecompile";
import { PerpsDataClient } from "./data.js";
import { AgentGatewayClient } from "./gateway.js";
import { executePlan } from "./execution.js";
import { AgentKitError, errorResult, type ToolResult } from "./result.js";

const E18 = 10n ** 18n;
const MAX_UINT = (1n << 256n) - 1n;

export interface AgentTool {
  name: string;
  description: string;
  inputSchema: z.ZodObject;
  readOnly: boolean;
  execute(input: unknown): Promise<ToolResult>;
}

function tool<S extends z.ZodRawShape>(
  definition: AgentToolDefinition<S>,
  run: (input: z.output<z.ZodObject<S>>) => Promise<unknown>,
): AgentTool {
  const { inputSchema } = definition;
  return {
    ...definition,
    async execute(input) {
      try {
        return { ok: true, data: await run(inputSchema.parse(input)) };
      } catch (error) {
        return errorResult(error);
      }
    },
  };
}

function units(
  value: string,
  decimals: number,
  field: string,
  allowZero = false,
): bigint {
  if ((value.split(".")[1]?.length ?? 0) > decimals) {
    throw new AgentKitError(
      "PRECISION_BOUND",
      `${field} exceeds ${decimals} decimal places`,
      { field, decimals },
    );
  }
  const result = parseUnits(value, decimals);
  if (result > MAX_UINT || (allowZero ? result < 0n : result <= 0n)) {
    throw new AgentKitError(
      "AMOUNT_BOUND",
      `${field} must be ${allowZero ? "nonnegative" : "positive"} and fit uint256`,
      { field },
    );
  }
  return result;
}

export function createAgentTools(
  gateway: AgentGatewayClient,
  data: PerpsDataClient,
) {
  const allowanceSource = {
    allowance: () =>
      gateway.allowance(COLLATERAL_TOKENS.usdl.address, PERPS_SPENDER),
    native: new Set<`0x${string}`>(),
  };

  async function checkMarket(
    marketId: bigint,
    input: {
      sizeUsd?: bigint;
      leverage?: bigint;
      prices?: readonly bigint[];
      opening?: boolean;
    },
  ) {
    const snapshot = await data.market(marketId);
    const { config, tiers } = snapshot;
    if (
      config.marketId !== marketId ||
      config.tickSize <= 0n ||
      config.lotSize <= 0n ||
      config.maxLeverage < 1n ||
      tiers.some((tier) => tier.maxLeverage < 1n)
    ) {
      throw new AgentKitError(
        "INVALID_MARKET_CONFIG",
        "The chain market configuration is invalid",
      );
    }
    if (input.opening && !config.enabled)
      throw new AgentKitError(
        "MARKET_DISABLED",
        "The market enabled bound is false",
      );
    if (input.sizeUsd !== undefined) {
      if (input.sizeUsd <= 0n || input.sizeUsd % config.lotSize !== 0n) {
        throw new AgentKitError(
          "SIZE_LOT_BOUND",
          `sizeUsd must be positive and a multiple of lotSize ${config.lotSize}`,
          {
            lotSize: config.lotSize.toString(),
            sizeUsd: input.sizeUsd.toString(),
          },
        );
      }
      if (
        input.opening &&
        config.oiCapUsd > 0n &&
        input.sizeUsd > config.oiCapUsd
      ) {
        throw new AgentKitError(
          "SIZE_CAP_BOUND",
          `sizeUsd exceeds oiCapUsd ${config.oiCapUsd}`,
          { oiCapUsd: config.oiCapUsd.toString() },
        );
      }
    }
    if (input.leverage !== undefined) {
      const tierCap =
        input.sizeUsd === undefined
          ? config.maxLeverage
          : maxLeverageForNotional(input.sizeUsd, tiers);
      const cap = tierCap < config.maxLeverage ? tierCap : config.maxLeverage;
      if (input.leverage < E18 || input.leverage > cap * E18) {
        throw new AgentKitError(
          "LEVERAGE_BOUND",
          `leverage must be between 1 and ${cap}`,
          { minimum: "1", maximum: cap.toString() },
        );
      }
    }
    for (const value of input.prices ?? []) {
      if (value !== 0n && (value < 0n || value % config.tickSize !== 0n)) {
        throw new AgentKitError(
          "PRICE_TICK_BOUND",
          `price must be a multiple of tickSize ${config.tickSize}`,
          { tickSize: config.tickSize.toString(), price: value.toString() },
        );
      }
    }
    return snapshot;
  }

  async function ownedPosition(positionId: bigint) {
    const position = await data.position(positionId);
    if (
      !position.active ||
      position.user.toLowerCase() !==
        (await gateway.walletAddress()).toLowerCase()
    ) {
      throw new AgentKitError(
        "POSITION_NOT_OWNED",
        "An active position belonging to the agent wallet is required",
      );
    }
    return position;
  }

  async function resting(
    input: z.output<z.ZodObject<typeof orderArgs>>,
    trigger?: string,
  ) {
    const marketId = BigInt(input.marketId);
    const sizeUsd = units(input.sizeUsd, 18, "sizeUsd");
    const lev = units(input.leverage, 18, "leverage");
    const limitPrice = units(input.limitPrice, 18, "limitPrice");
    const triggerPrice =
      trigger === undefined ? undefined : units(trigger, 18, "triggerPrice");
    const tp = units(input.takeProfit, 18, "takeProfit", true);
    const sl = units(input.stopLoss, 18, "stopLoss", true);
    await checkMarket(marketId, {
      sizeUsd,
      leverage: lev,
      prices: [limitPrice, triggerPrice ?? 0n, tp, sl],
      opening: true,
    });
    const common = {
      marketId,
      isLong: input.isLong,
      sizeUsd,
      leverage: lev,
      limitPrice,
      collateralAmount: deriveImpliedCollateral(
        sizeUsd,
        lev,
        COLLATERAL_TOKENS.usdl.decimals,
      ),
      timeInForce: TimeInForce[input.timeInForce],
      reduceOnly: input.reduceOnly,
      cloid: input.clientOrderId as `0x${string}`,
      takeProfitPrice: tp,
      stopLossPrice: sl,
    };
    const call =
      triggerPrice === undefined
        ? calls.buildPlaceLimitOrder(common)
        : calls.buildPlaceStopLimitOrder({ ...common, triggerPrice });
    return executePlan(gateway, await planOrder([call]));
  }

  const tools = [
    tool(AGENT_TOOLS.markets, () => data.markets()),
    tool(AGENT_TOOLS.account, async () =>
      data.account(await gateway.walletAddress()),
    ),
    tool(AGENT_TOOLS.positions, async () =>
      data.positions(await gateway.walletAddress()),
    ),
    tool(AGENT_TOOLS.open_orders, async () =>
      data.openOrders(await gateway.walletAddress()),
    ),
    tool(AGENT_TOOLS.history, async (input) =>
      data.history(await gateway.walletAddress(), input.limit, input.offset),
    ),
    tool(AGENT_TOOLS.deposit, async (input) => {
      const plan = await calls.buildDepositPlan(
        units(input.amount, 6, "amount"),
        allowanceSource.allowance,
        allowanceSource.native,
      );
      return executePlan(gateway, plan, input.idempotencyKey);
    }),
    tool(AGENT_TOOLS.withdraw, async (input) =>
      executePlan(gateway, [
        calls.buildWithdraw(units(input.amount, 6, "amount")),
      ]),
    ),
    tool(AGENT_TOOLS.open_market, async (input) => {
      const marketId = BigInt(input.marketId);
      const lev = units(input.leverage, 18, "leverage");
      const collateralAmount = units(
        input.collateralAmount,
        6,
        "collateralAmount",
      );
      const tp = units(input.takeProfit, 18, "takeProfit", true);
      const sl = units(input.stopLoss, 18, "stopLoss", true);
      await checkMarket(marketId, {
        leverage: lev,
        prices: [tp, sl],
        opening: true,
      });
      const quote = await data.quoteOpen(
        marketId,
        collateralAmount,
        lev,
        input.isLong,
      );
      if (
        quote.leverage !== lev ||
        quote.collateralUsd !== collateralAmount * 10n ** 12n
      ) {
        throw new AgentKitError(
          "QUOTE_INPUT_MISMATCH",
          "The chain quote does not match collateral and leverage",
        );
      }
      await checkMarket(marketId, {
        sizeUsd: quote.sizeUsd,
        leverage: lev,
        opening: true,
      });
      return executePlan(
        gateway,
        await planOpen({
          marketId,
          isLong: input.isLong,
          collateralAmount,
          leverage: lev,
          quote,
          takeProfitPrice: tp,
          stopLossPrice: sl,
        }),
      );
    }),
    tool(AGENT_TOOLS.place_limit, (input) => resting(input)),
    tool(AGENT_TOOLS.place_stop_limit, (input) =>
      resting(input, input.triggerPrice),
    ),
    tool(AGENT_TOOLS.cancel_order, async (input) => {
      const orderId = BigInt(input.orderId);
      const order = await data.order(orderId);
      if (
        !order.active ||
        order.user.toLowerCase() !==
          (await gateway.walletAddress()).toLowerCase()
      ) {
        throw new AgentKitError(
          "ORDER_NOT_OWNED",
          "An active order belonging to the agent wallet is required",
        );
      }
      return executePlan(gateway, [calls.buildCancelOrder(orderId)]);
    }),
    tool(AGENT_TOOLS.set_tpsl, async (input) => {
      const positionId = BigInt(input.positionId);
      const position = await ownedPosition(positionId);
      const tp = units(input.takeProfit, 18, "takeProfit", true);
      const sl = units(input.stopLoss, 18, "stopLoss", true);
      await checkMarket(position.marketId, { prices: [tp, sl] });
      const mark = await data.mark(position.marketId);
      if (!mark.fresh || mark.markPrice <= 0n)
        throw new AgentKitError(
          "MARK_PRICE_STALE",
          "A fresh positive mark price is required for TP/SL",
        );
      if (
        (tp !== 0n &&
          (position.isLong ? tp <= mark.markPrice : tp >= mark.markPrice)) ||
        (sl !== 0n &&
          (position.isLong ? sl >= mark.markPrice : sl <= mark.markPrice))
      ) {
        throw new AgentKitError(
          "TPSL_PRICE_BOUND",
          "Take profit and stop loss must straddle the mark price for the position side",
        );
      }
      return executePlan(gateway, [
        calls.buildSetPositionTpSl({
          positionId,
          isLong: position.isLong,
          takeProfitPrice: tp,
          stopLossPrice: sl,
        }),
      ]);
    }),
    tool(AGENT_TOOLS.add_collateral, async (input) => {
      const positionId = BigInt(input.positionId);
      await ownedPosition(positionId);
      return executePlan(
        gateway,
        await planCollateral(
          {
            action: "add",
            positionId,
            amount: units(input.amount, 6, "amount"),
          },
          allowanceSource,
        ),
        input.idempotencyKey,
      );
    }),
    tool(AGENT_TOOLS.remove_collateral, async (input) => {
      const positionId = BigInt(input.positionId);
      await ownedPosition(positionId);
      return executePlan(
        gateway,
        await planCollateral(
          {
            action: "remove",
            positionId,
            amount: units(input.amount, 6, "amount"),
          },
          allowanceSource,
        ),
      );
    }),
    tool(AGENT_TOOLS.close_position, async (input) => {
      const positionId = BigInt(input.positionId);
      const position = await ownedPosition(positionId);
      const quote = await data.quoteClose(positionId);
      return executePlan(
        gateway,
        planClose({
          positionId,
          isLong: position.isLong,
          exitPrice: quote.exitPrice,
        }),
      );
    }),
    tool(AGENT_TOOLS.partial_close, async (input) => {
      const positionId = BigInt(input.positionId);
      const position = await ownedPosition(positionId);
      const closeSizeUsd = partialCloseSize(
        position.sizeUsd,
        input.fractionBps,
      );
      await checkMarket(position.marketId, { sizeUsd: closeSizeUsd });
      const [exitPrice] = await data.quotePartialClose(
        positionId,
        closeSizeUsd,
      );
      return executePlan(
        gateway,
        planPartialClose({
          positionId,
          isLong: position.isLong,
          closeSizeUsd,
          exitPrice,
        }),
      );
    }),
    tool(AGENT_TOOLS.action_status, (input) => gateway.action(input.actionId)),
    tool(AGENT_TOOLS.transaction_receipt, (input) =>
      gateway.receipt(input.hash as `0x${string}`),
    ),
  ];
  return {
    tools,
    async callTool(name: string, input: unknown): Promise<ToolResult> {
      const entry = tools.find((candidate) => candidate.name === name);
      return entry
        ? entry.execute(input)
        : errorResult(
            new AgentKitError(
              "UNKNOWN_TOOL",
              "The requested trading tool is not registered",
            ),
          );
    },
  };
}
