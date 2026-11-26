import { z } from "zod";
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
const id = z
  .string()
  .max(78)
  .regex(/^(0|[1-9]\d*)$/)
  .refine((value) => BigInt(value) <= MAX_UINT, "Identifier must fit uint256");
const decimal = z
  .string()
  .max(80)
  .regex(/^\d+(\.\d+)?$/)
  .describe("Exact decimal string; no exponent or units");
const amount = decimal.describe(
  "Positive USDL amount, at most 6 decimal places",
);
const price = decimal.describe(
  "Price in USD, at most 18 decimal places; use 0 to clear TP/SL",
);
const size = decimal.describe(
  "Positive notional in USD, at most 18 decimal places and a multiple of market lotSize",
);
const leverage = decimal.describe(
  "Leverage multiple, at least 1 and at most the chain market and margin tier caps",
);
const idempotencyKey = z
  .string()
  .min(8)
  .max(128)
  .regex(/^[A-Za-z0-9_.:-]+$/)
  .describe(
    "Stable unique intent key; reuse only to resume the same approval action",
  );
const positionArgs = {
  positionId: id.describe("Position ID belonging to this agent wallet"),
};
const protections = {
  takeProfit: price.default("0"),
  stopLoss: price.default("0"),
};
const orderArgs = {
  marketId: id,
  isLong: z.boolean(),
  leverage,
  sizeUsd: size,
  limitPrice: price,
  timeInForce: z.enum(["GTC", "IOC", "FOK", "POST_ONLY"]).default("GTC"),
  reduceOnly: z.boolean().default(false),
  clientOrderId: z
    .string()
    .regex(/^0x[0-9a-fA-F]{32}$/)
    .refine((value) => BigInt(value) !== 0n, "Client order ID must be nonzero"),
  ...protections,
};

export interface AgentTool {
  name: string;
  description: string;
  inputSchema: z.ZodObject;
  readOnly: boolean;
  execute(input: unknown): Promise<ToolResult>;
}

function tool<S extends z.ZodRawShape>(
  name: string,
  description: string,
  shape: S,
  readOnly: boolean,
  run: (input: z.output<z.ZodObject<S>>) => Promise<unknown>,
): AgentTool {
  const inputSchema = z.object(shape).strict();
  return {
    name,
    description,
    inputSchema,
    readOnly,
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
    tool(
      "markets",
      "List perpetual markets with chain configuration, mark/index prices, freshness and indexed funding rates. Amounts from the chain are raw integer units.",
      {},
      true,
      () => data.markets(),
    ),
    tool(
      "account",
      "Read this agent wallet's cross-margin account, USDL collateral, equity, margin requirements and withdrawable collateral.",
      {},
      true,
      async () => data.account(await gateway.walletAddress()),
    ),
    tool(
      "positions",
      "Read active positions belonging to this agent wallet, including chain position details and margin.",
      {},
      true,
      async () => data.positions(await gateway.walletAddress()),
    ),
    tool(
      "open_orders",
      "Read active orders belonging to this agent wallet directly from the perps precompile.",
      {},
      true,
      async () => data.openOrders(await gateway.walletAddress()),
    ),
    tool(
      "history",
      "Read paginated indexed trades and closed positions for this agent wallet. Use offset to retrieve the next page.",
      {
        limit: z.number().int().min(1).max(100).default(50),
        offset: z.number().int().min(0).max(2_147_483_647).default(0),
      },
      true,
      async (input) =>
        data.history(await gateway.walletAddress(), input.limit, input.offset),
    ),
    tool(
      "deposit",
      "Deposit USDL from the agent wallet into its trading account. Approves exactly the amount when needed; a returned action is pending until confirmed by action_status.",
      { amount, idempotencyKey },
      false,
      async (input) => {
        const plan = await calls.buildDepositPlan(
          units(input.amount, 6, "amount"),
          allowanceSource.allowance,
          allowanceSource.native,
        );
        return executePlan(gateway, plan, input.idempotencyKey);
      },
    ),
    tool(
      "withdraw",
      "Withdraw available USDL from the trading account back to the same agent wallet. Owner sweep moves wallet funds to the owner.",
      { amount },
      false,
      async (input) =>
        executePlan(gateway, [
          calls.buildWithdraw(units(input.amount, 6, "amount")),
        ]),
    ),
    tool(
      "open_market",
      "Open a market position using deposited USDL collateral and a fresh chain quote. Checks size and leverage against chain limits, then simulates and sends with the backend slippage bound.",
      {
        marketId: id,
        isLong: z.boolean(),
        collateralAmount: amount,
        leverage,
        ...protections,
      },
      false,
      async (input) => {
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
      },
    ),
    tool(
      "place_limit",
      "Place a limit order using deposited collateral. Supply a unique nonzero bytes16 clientOrderId, a size aligned to lotSize and a price aligned to tickSize.",
      orderArgs,
      false,
      (input) => resting(input),
    ),
    tool(
      "place_stop_limit",
      "Place a stop-limit order using deposited collateral. Both trigger and limit prices must align to the market tickSize.",
      { ...orderArgs, triggerPrice: price },
      false,
      (input) => resting(input, input.triggerPrice),
    ),
    tool(
      "cancel_order",
      "Cancel one active order owned by this agent wallet.",
      { orderId: id },
      false,
      async (input) => {
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
      },
    ),
    tool(
      "set_tpsl",
      "Set take profit and stop loss on an owned position; 0 clears a side. Prices must align to tickSize and straddle the current mark for the position side.",
      { ...positionArgs, takeProfit: price, stopLoss: price },
      false,
      async (input) => {
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
      },
    ),
    tool(
      "add_collateral",
      "Add USDL collateral to an owned position. Uses an exact allowance when needed; poll a returned action to confirmation.",
      { ...positionArgs, amount, idempotencyKey },
      false,
      async (input) => {
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
      },
    ),
    tool(
      "remove_collateral",
      "Remove USDL collateral from an owned position. Gateway simulation checks the remaining margin before sending.",
      { ...positionArgs, amount },
      false,
      async (input) => {
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
      },
    ),
    tool(
      "close_position",
      "Close the whole owned position using the current close quote and the backend slippage bound. The full-close sentinel follows the position size at execution.",
      positionArgs,
      false,
      async (input) => {
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
      },
    ),
    tool(
      "partial_close",
      "Close a fraction of an owned position. fractionBps is strictly between 0 and 10000; the resulting USD size must align to lotSize.",
      { ...positionArgs, fractionBps: z.number().int().min(1).max(9999) },
      false,
      async (input) => {
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
      },
    ),
    tool(
      "action_status",
      "Poll a durable approval-and-call action. terminal=false means pending; confirmed means success; a refusal keeps the gateway error code.",
      { actionId: z.string().regex(/^act_[0-9a-f]{24}$/) },
      true,
      (input) => gateway.action(input.actionId),
    ),
    tool(
      "transaction_receipt",
      "Read a submitted transaction receipt from the wallet gateway. A send response alone does not mean the transaction confirmed.",
      { hash: z.string().regex(/^0x[0-9a-fA-F]{64}$/) },
      true,
      (input) => gateway.receipt(input.hash as `0x${string}`),
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
