import { z } from "zod";

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
export const orderArgs = {
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

export interface AgentToolDefinition<S extends z.ZodRawShape> {
  name: string;
  description: string;
  inputSchema: z.ZodObject<S>;
  readOnly: boolean;
}

function defineTool<S extends z.ZodRawShape>(
  name: string,
  description: string,
  shape: S,
  readOnly: boolean,
): AgentToolDefinition<S> {
  return { name, description, inputSchema: z.object(shape).strict(), readOnly };
}

export const AGENT_TOOLS = {
  markets: defineTool(
    "markets",
    "List perpetual markets with chain configuration, mark/index prices, freshness and indexed funding rates. Amounts from the chain are raw integer units.",
    {},
    true,
  ),
  account: defineTool(
    "account",
    "Read this agent wallet's cross-margin account, USDL collateral, equity, margin requirements and withdrawable collateral.",
    {},
    true,
  ),
  positions: defineTool(
    "positions",
    "Read active positions belonging to this agent wallet, including chain position details and margin.",
    {},
    true,
  ),
  open_orders: defineTool(
    "open_orders",
    "Read active orders belonging to this agent wallet directly from the perps precompile.",
    {},
    true,
  ),
  history: defineTool(
    "history",
    "Read paginated indexed trades and closed positions for this agent wallet. Use offset to retrieve the next page.",
    {
      limit: z.number().int().min(1).max(100).default(50),
      offset: z.number().int().min(0).max(2_147_483_647).default(0),
    },
    true,
  ),
  deposit: defineTool(
    "deposit",
    "Deposit USDL from the agent wallet into its trading account. Approves exactly the amount when needed; a returned action is pending until confirmed by action_status.",
    { amount, idempotencyKey },
    false,
  ),
  withdraw: defineTool(
    "withdraw",
    "Withdraw available USDL from the trading account back to the same agent wallet. Owner sweep moves wallet funds to the owner.",
    { amount },
    false,
  ),
  open_market: defineTool(
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
  ),
  place_limit: defineTool(
    "place_limit",
    "Place a limit order using deposited collateral. Supply a unique nonzero bytes16 clientOrderId, a size aligned to lotSize and a price aligned to tickSize.",
    orderArgs,
    false,
  ),
  place_stop_limit: defineTool(
    "place_stop_limit",
    "Place a stop-limit order using deposited collateral. Both trigger and limit prices must align to the market tickSize.",
    { ...orderArgs, triggerPrice: price },
    false,
  ),
  cancel_order: defineTool(
    "cancel_order",
    "Cancel one active order owned by this agent wallet.",
    { orderId: id },
    false,
  ),
  set_tpsl: defineTool(
    "set_tpsl",
    "Set take profit and stop loss on an owned position; 0 clears a side. Prices must align to tickSize and straddle the current mark for the position side.",
    { ...positionArgs, takeProfit: price, stopLoss: price },
    false,
  ),
  add_collateral: defineTool(
    "add_collateral",
    "Add USDL collateral to an owned position. Uses an exact allowance when needed; poll a returned action to confirmation.",
    { ...positionArgs, amount, idempotencyKey },
    false,
  ),
  remove_collateral: defineTool(
    "remove_collateral",
    "Remove USDL collateral from an owned position. Gateway simulation checks the remaining margin before sending.",
    { ...positionArgs, amount },
    false,
  ),
  close_position: defineTool(
    "close_position",
    "Close the whole owned position using the current close quote and the backend slippage bound. The full-close sentinel follows the position size at execution.",
    positionArgs,
    false,
  ),
  partial_close: defineTool(
    "partial_close",
    "Close a fraction of an owned position. fractionBps is strictly between 0 and 10000; the resulting USD size must align to lotSize.",
    { ...positionArgs, fractionBps: z.number().int().min(1).max(9999) },
    false,
  ),
  action_status: defineTool(
    "action_status",
    "Poll a durable approval-and-call action. terminal=false means pending; confirmed means success; a refusal keeps the gateway error code.",
    { actionId: z.string().regex(/^act_[0-9a-f]{24}$/) },
    true,
  ),
  transaction_receipt: defineTool(
    "transaction_receipt",
    "Read a submitted transaction receipt from the wallet gateway. A send response alone does not mean the transaction confirmed.",
    { hash: z.string().regex(/^0x[0-9a-fA-F]{64}$/) },
    true,
  ),
};
