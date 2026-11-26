import { encodeFunctionData, type Address, type PublicClient } from "viem";
import {
  buildOpenPosition,
  buildClosePosition,
  buildSetPositionTpSl,
  buildAddCollateral,
  buildRemoveCollateral,
  buildTopUpPosition,
  buildCancelOrder,
  buildCancelByCloid,
  buildDeposit,
  buildWithdraw,
} from "../../perps/calls";
import { COLLATERAL_TOKENS, erc20Abi } from "../../perps/config";
import { PERPS_SPENDER } from "../../network/approvals";
import {
  AgentOwnerClient,
  checkedAgentAmount,
  type AgentPolicy,
  type AgentRuleInput,
  type AgentBudgetInput,
} from "./sdk/agents";

const unit = 10n ** 18n;
const perpsWrites = [
  buildOpenPosition({
    marketId: 0n,
    isLong: true,
    collateralAmount: 1n,
    leverage: unit,
    quote: { entryPrice: unit, sizeUsd: unit },
  }),
  buildClosePosition({ positionId: 1n, isLong: true, exitPrice: unit }),
  buildSetPositionTpSl({
    positionId: 1n,
    isLong: true,
    takeProfitPrice: 0n,
    stopLossPrice: 0n,
  }),
  buildAddCollateral(1n, 1n),
  buildRemoveCollateral(1n, 1n),
  buildTopUpPosition(1n, 1n),
  buildCancelOrder(1n),
  buildCancelByCloid(0n, "0x00000000000000000000000000000001"),
  buildDeposit(1n),
  buildWithdraw(1n),
];
export const PERPS_AGENT_FUNCTIONS = perpsWrites.map((call) => ({
  contract: call.address,
  name: call.functionName,
  selector: encodeFunctionData(call).slice(0, 10),
}));
export const PERPS_AGENT_APPROVAL = {
  contract: COLLATERAL_TOKENS.usdl.address,
  spender: PERPS_SPENDER,
  selector: encodeFunctionData({
    abi: erc20Abi,
    functionName: "approve",
    args: [PERPS_SPENDER, 1n],
  }).slice(0, 10),
};
export interface PerpsAgentPreset {
  policy: AgentPolicy;
  rules: AgentRuleInput[];
  budgets: AgentBudgetInput[];
  sentences: string[];
}
export function buildPerpsAgentPreset(
  owner: Address,
  transactionCap: string,
  dailyCap: string,
  expiresInSeconds: number,
): PerpsAgentPreset {
  if (!/^0x[0-9a-fA-F]{40}$/.test(owner))
    throw new Error("An embedded wallet address is required");
  const max = (1n << 256n) - 1n;
  const tx = checkedAgentAmount(transactionCap, 18, max);
  const daily = checkedAgentAmount(dailyCap, 18, max);
  const approve = checkedAgentAmount(
    transactionCap,
    COLLATERAL_TOKENS.usdl.decimals,
    max,
  );
  const tokenDaily = checkedAgentAmount(
    dailyCap,
    COLLATERAL_TOKENS.usdl.decimals,
    max,
  );
  if (BigInt(tx) > BigInt(daily))
    throw new Error("The daily cap must be at least the per-transaction cap");
  if (
    !Number.isInteger(expiresInSeconds) ||
    expiresInSeconds <= 0 ||
    expiresInSeconds > 31_536_000
  )
    throw new Error("The budget must expire within one year");
  return {
    policy: {
      mode: "trade_only",
      max_tx_value_wei: tx,
      max_daily_value_wei: daily,
      max_approve_wei: approve,
      allow_native_transfer: false,
      withdrawal_allowlist_only: true,
      daily_reset_utc_hour: 0,
      rate_limit_per_min: 60,
    },
    rules: [
      ...Array.from(
        new Set(PERPS_AGENT_FUNCTIONS.map((call) => call.contract)),
      ).map(
        (value): AgentRuleInput => ({
          effect: "allow",
          subject: "contract",
          value,
        }),
      ),
      {
        effect: "allow",
        subject: "contract",
        value: PERPS_AGENT_APPROVAL.contract,
      },
      ...PERPS_AGENT_FUNCTIONS.map(
        (call): AgentRuleInput => ({
          effect: "allow",
          subject: "selector",
          value: call.selector,
          note: call.name,
        }),
      ),
      {
        effect: "allow",
        subject: "selector",
        value: PERPS_AGENT_APPROVAL.selector,
        note: "approve",
      },
      {
        effect: "allow",
        subject: "address",
        value: PERPS_AGENT_APPROVAL.spender,
        note: "USDL approval spender",
      },
      {
        effect: "allow",
        subject: "token",
        value: COLLATERAL_TOKENS.usdl.address,
        max_value_wei: approve,
      },
      { effect: "allow", subject: "withdrawal", value: owner },
    ],
    budgets: [
      {
        target_contract: PERPS_SPENDER,
        token: COLLATERAL_TOKENS.usdl.address,
        cap_wei: tokenDaily,
        expires_in_seconds: expiresInSeconds,
      },
    ],
    sentences: [
      "Use trade-only mode for the perpetuals contract and USDL approval functions listed below.",
      `Set the native transaction value cap to ${transactionCap} PAX and the native daily value cap to ${dailyCap} PAX, resetting at 00:00 UTC.`,
      `Set the USDL approval and token transfer cap to ${transactionCap} USDL. The approval spender is ${PERPS_AGENT_APPROVAL.spender}.`,
      `Create a ${dailyCap} USDL budget for the perpetuals contract, expiring in ${expiresInSeconds} seconds. Caps describe wallet value and token transfers; they do not describe position notional or trading losses.`,
      `Disable native transfers and set the withdrawal list to your embedded wallet only: ${owner}.`,
      "Replace the existing rules and active budgets. Keep the agent frozen after saving; unfreeze it separately when ready.",
    ],
  };
}
export async function savePerpsAgentPreset(
  client: AgentOwnerClient,
  did: string,
  preset: PerpsAgentPreset,
): Promise<void> {
  await client.freeze(did, true);
  const current = await client.detail(did);
  for (const rule of current.rules) await client.deleteRule(did, rule.id);
  for (const budget of current.budgets)
    await client.deleteBudget(did, budget.id);
  await client.policy(did, preset.policy);
  for (const rule of preset.rules) await client.addRule(did, rule);
  for (const budget of preset.budgets) await client.addBudget(did, budget);
}
export interface AgentWalletBalances {
  native: bigint;
  collateral: bigint;
}
export type ReadAgentBalances = (
  address: Address,
) => Promise<AgentWalletBalances>;
export async function readAgentWalletBalances(
  client: PublicClient,
  address: Address,
): Promise<AgentWalletBalances> {
  const [native, collateral] = await Promise.all([
    client.getBalance({ address }),
    client.readContract({
      address: COLLATERAL_TOKENS.usdl.address,
      abi: erc20Abi,
      functionName: "balanceOf",
      args: [address],
    }),
  ]);
  return { native, collateral };
}
