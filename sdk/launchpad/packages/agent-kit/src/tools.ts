import { z } from 'zod';
import { LaunchpadAgentKit } from './kit.js';
import { AgentKitError, toolResult } from './errors.js';

const address = z
  .string()
  .regex(/^0x[0-9a-fA-F]{40}$/)
  .describe('EVM address, including 0x.');
const amount = z
  .string()
  .regex(/^(0|[1-9][0-9]{0,77})$/)
  .describe('Integer amount in raw base units; USDL has 6 decimals.');
const pool = address.describe('Pool address returned by list_coins or search_coins.');
const limit = z.number().int().min(1).max(100).optional();
const cursor = z.string().min(1).max(2048).optional();
const page = { limit, cursor };
const idempotencyKey = z
  .string()
  .regex(/^[A-Za-z0-9_.:-]{8,128}$/)
  .describe(
    'Unique operation ID. Keep it when polling an accepted action; do not resubmit a transaction after an uncertain response.',
  );
const trade = z
  .object({
    pool,
    quoteId: z
      .string()
      .uuid()
      .describe(
        'Unexpired quote ID from this running kit for this side, pool, amount and wallet. Used once.',
      ),
    amountInRaw: amount,
    maxSlippageBps: z
      .number()
      .int()
      .min(1)
      .max(5000)
      .describe(
        'Maximum slippage in basis points. 100 = 1%. Minimum output is floor(quote output × (10000 − bps) / 10000).',
      ),
    idempotencyKey,
    acceptHighSlippage: z
      .boolean()
      .optional()
      .describe('Explicit consent for this trade when slippage exceeds 500 bps.'),
    acceptHighImpact: z
      .boolean()
      .optional()
      .describe('Explicit consent for this trade when the quote impact exceeds 1000 bps.'),
  })
  .strict();

export const TOOL_SCHEMAS = {
  list_coins: z
    .object({ sort: z.enum(['new', 'volume', 'trending']).default('new'), ...page })
    .strict(),
  search_coins: z.object({ q: z.string().min(1).max(100), limit }).strict(),
  coin: z.object({ pool }).strict(),
  trades: z.object({ pool, side: z.enum(['buy', 'sell']).optional(), ...page }).strict(),
  candles: z
    .object({
      pool,
      interval: z.enum(['1m', '5m', '15m', '1h', '4h', '1d', '1w']),
      from: z.iso.datetime({ precision: 3 }),
      to: z.iso.datetime({ precision: 3 }),
      ...page,
    })
    .strict(),
  holders: z.object({ pool, ...page }).strict(),
  balances: z.object({ tokens: z.array(address).max(100).default([]) }).strict(),
  positions: z.object(page).strict(),
  quote: z.object({ pool, side: z.enum(['buy', 'sell']), amountInRaw: amount }).strict(),
  buy: trade,
  sell: trade,
  launch: z
    .object({
      name: z.string().min(1).max(64),
      symbol: z.string().min(1).max(16),
      feeStrategy: z.number().int().min(0).max(3),
      idempotencyKey,
    })
    .strict(),
  action: z.object({ actionId: z.string().regex(/^act_[0-9a-f]{24}$/) }).strict(),
  preset: z
    .object({
      ownerWallet: address,
      maxTxValueWei: amount,
      maxDailyValueWei: amount,
      maxApproveWei: amount,
      budgetCapWei: amount,
      expiresInSeconds: z.number().int().min(1).max(31536000),
      sellTokens: z.array(address).max(100).optional(),
      rateLimitPerMinute: z.number().int().min(1).max(10000).optional(),
    })
    .strict(),
  agent_identity: z.object({}).strict(),
  provision_agent: z
    .object({
      stage: z.enum(['wallet', 'identity']).optional(),
      bindSignature: z
        .string()
        .regex(/^(0x)?[0-9a-fA-F]{128}$/)
        .optional(),
    })
    .strict(),
  claim_agent: z.object({ ownerUserId: z.string().min(1).max(256) }).strict(),
};

export type ToolName = keyof typeof TOOL_SCHEMAS;

export const TOOL_DESCRIPTIONS: Record<ToolName, string> = {
  list_coins:
    'List public launchpad pools by age, volume or trend. Follow nextCursor for more; amounts and quality remain exact.',
  search_coins:
    'Search public launchpad coins by name or symbol. Use the returned pool address with coin or quote.',
  coin: 'Read a pool, token, price and market valuation. Progress is null with status unavailable. Preserve quality and watermark when interpreting prices.',
  trades: 'Read executed buys and sells for one pool with side filter and cursor pagination.',
  candles:
    'Read OHLC candles for one pool and UTC time range. Use ISO timestamps with milliseconds; preserve partial coverage.',
  holders: 'Read token holders for one pool, balances, ownership shares and coverage.',
  balances:
    'Read the agent wallet native PAX balance and requested ERC-20 balances through the gateway. Missing balances remain null.',
  positions:
    'Read the public portfolio for the wallet authenticated by the agent gateway. Includes basis and pricing quality; never reads the owner portfolio.',
  quote:
    'Simulate quoteExactInput through the gateway. Returns a quote valid for 30 seconds for the pool, side, amount and agent wallet. Foreign home chains are refused before request signing.',
  buy: 'Buy tokens using USDL with a fresh quote and explicit maximum slippage. Simulates the call, or the exact approval followed by a gateway action that simulates the call after approval. The recipient is the agent wallet. Poll action when submission is action.',
  sell: 'Sell tokens for USDL using a fresh sell quote and explicit maximum slippage. Simulates the call, or approval followed by a gateway action. The recipient is the agent wallet. Poll action when submission is action.',
  launch:
    'Create a coin with createMarket and the standard optical (zero address). Fee strategy is 0–3; the protocol fee is one USDL plus PAX gas. Simulates the call or approval before allowance-and-call. No initial buy; quote the created pool separately.',
  action:
    'Poll a durable gateway action by ID. Continue until terminal; an accepted or pending action does not mean the transaction confirmed. Refusals retain the gateway error code.',
  preset:
    'Return owner policy, contract/selector rules and an expiring budget for the supplied caps. The owner applies these through owner routes. Native transfers are off and the owner wallet is the withdrawal entry.',
  agent_identity:
    'Read the authenticated agent identity, wallet, gateway chain and effective policy. Does not expose the signing seed.',
  provision_agent:
    'Request provisioning of the agent wallet. Signs a requested binding only after checking its domain, chain and wallet address, then continues provisioning. Pending provisioning is not an active wallet.',
  claim_agent:
    'Create a short-lived claim signature for the given owner user ID. The signed-in owner submits it to the owner claim route. Does not claim ownership by itself.',
};

export const TOOL_NAMES = Object.freeze(Object.keys(TOOL_SCHEMAS) as ToolName[]);

function defined<T extends object>(value: T): { [K in keyof T]: Exclude<T[K], undefined> } {
  return Object.fromEntries(Object.entries(value).filter(([, item]) => item !== undefined)) as {
    [K in keyof T]: Exclude<T[K], undefined>;
  };
}

export async function invokeTool(kit: LaunchpadAgentKit, name: ToolName, input: unknown) {
  return toolResult(async () => {
    if (!Object.hasOwn(TOOL_SCHEMAS, name))
      throw new AgentKitError('unknown_tool', 'Unknown launchpad tool.');
    const parsed = TOOL_SCHEMAS[name].safeParse(input);
    if (!parsed.success)
      throw new AgentKitError(
        'invalid_parameters',
        'Tool parameters do not match the schema.',
        400,
        parsed.error.issues,
      );
    switch (name) {
      case 'list_coins':
        return kit.listCoins(defined(TOOL_SCHEMAS.list_coins.parse(input)));
      case 'search_coins':
        return kit.searchCoins(defined(TOOL_SCHEMAS.search_coins.parse(input)));
      case 'coin':
        return kit.coin(TOOL_SCHEMAS.coin.parse(input).pool);
      case 'trades': {
        const { pool, ...query } = TOOL_SCHEMAS.trades.parse(input);
        return kit.trades(pool, defined(query));
      }
      case 'candles': {
        const { pool, ...query } = TOOL_SCHEMAS.candles.parse(input);
        return kit.candles(pool, defined(query));
      }
      case 'holders': {
        const { pool, ...query } = TOOL_SCHEMAS.holders.parse(input);
        return kit.holders(pool, defined(query));
      }
      case 'balances':
        return kit.balances(TOOL_SCHEMAS.balances.parse(input).tokens);
      case 'positions':
        return kit.positions(defined(TOOL_SCHEMAS.positions.parse(input)));
      case 'quote':
        return kit.quote(TOOL_SCHEMAS.quote.parse(input));
      case 'buy':
        return kit.buy(defined(TOOL_SCHEMAS.buy.parse(input)));
      case 'sell':
        return kit.sell(defined(TOOL_SCHEMAS.sell.parse(input)));
      case 'launch':
        return kit.launch(TOOL_SCHEMAS.launch.parse(input));
      case 'action':
        return kit.gateway.action(TOOL_SCHEMAS.action.parse(input).actionId);
      case 'preset':
        return kit.preset(defined(TOOL_SCHEMAS.preset.parse(input)));
      case 'agent_identity':
        return kit.gateway.identity();
      case 'provision_agent': {
        const { bindSignature, stage } = TOOL_SCHEMAS.provision_agent.parse(input);
        return kit.gateway.provision(bindSignature, stage);
      }
      case 'claim_agent':
        return kit.gateway.claim(TOOL_SCHEMAS.claim_agent.parse(input).ownerUserId);
    }
  });
}
