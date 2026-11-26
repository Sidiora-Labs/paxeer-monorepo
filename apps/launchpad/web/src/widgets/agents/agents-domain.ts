import { erc20Abi, formatUnits, parseUnits, toFunctionSelector, toFunctionSignature, type Abi, type Address, type PublicClient } from 'viem';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS, ROUTER_ADDRESS, USDL_ADDRESS } from '@/core/network/contracts';
import RouterAbi from '@/core/network/abis/Router.json';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { PaxeerWalletError, type PaxeerWallet, type SigningSession, type AgentDetail, type AgentRuleInput, type OwnedAgent } from '@/core/wallet-sdk/embedded_sdk/embedded/sdk';

export type AgentChainReader = Pick<PublicClient, 'getBalance' | 'readContract' | 'getTransactionReceipt'>;
export type AgentPreset = 'read' | 'trade' | 'create';
export type AgentAsset = 'native' | 'quote';
export interface AgentBalances { native: bigint; quote: bigint; quoteDecimals: number }
export interface AgentRow { summary: OwnedAgent; detail: AgentDetail; balances: AgentBalances | null; balanceError: string | null }
export interface AgentInventory { rows: AgentRow[]; ownerBalances: AgentBalances | null; balanceError: string | null }

export const agentPresetMethods = {
  trade: ['buy(address,uint256,uint256,uint256)', 'sell(address,uint256,uint256,uint256)', 'swapTokenForToken(address,address,uint256,uint256,uint256)', 'approve(address,uint256)'],
  create: ['createMarket(string,string,uint8,address)', 'launchChain(string,string,uint8,address,string,uint64,uint64,uint16)'],
} as const;

export function agentCap(value: string, decimals: number): bigint {
  if (value.length > 80 || !/^(0|[1-9]\d*)(\.\d+)?$/.test(value) || (value.split('.')[1]?.length ?? 0) > decimals)
    throw new Error(`Enter a nonnegative amount with at most ${decimals} decimal places.`);
  const amount = parseUnits(value, decimals);
  if (amount > (1n << 256n) - 1n) throw new Error('Enter an amount within the token range.');
  return amount;
}
export function agentAmount(value: string, decimals: number, balance?: bigint): bigint {
  const amount = agentCap(value, decimals);
  if (amount === 0n) throw new Error('Enter an amount greater than zero.');
  if (balance !== undefined && amount > balance) throw new Error('The amount exceeds the available balance.');
  return amount;
}
export function agentAsset(asset: AgentAsset, balances: AgentBalances) {
  return asset === 'native'
    ? { symbol: CORE_DEPLOYMENT.nativeSymbol, decimals: CORE_DEPLOYMENT.nativeDecimals, token: null, balance: balances.native }
    : { symbol: 'USDL', decimals: balances.quoteDecimals, token: USDL_ADDRESS, balance: balances.quote };
}
export function agentBalanceText(balances: AgentBalances): string {
  return `${formatUnits(balances.native, 18)} PAX · ${formatUnits(balances.quote, balances.quoteDecimals)} USDL`;
}
export function agentError(error: unknown): string {
  if (error instanceof PaxeerWalletError) {
    if (['agent_lane_disabled', 'provisioning_unavailable'].includes(error.code)) return 'The agent gateway is not configured. Agent management is unavailable here.';
    if (error.code === 'SESSION_CHANGED' || error.code === 'NO_SESSION') return 'Your embedded wallet session changed. Sign in again.';
    if (error.status === 401) return 'Your sign-in has expired or the agent signature was refused. Sign in and try again.';
    if (error.status === 403) return 'The gateway refused this action. Check ownership and whether the agent is frozen.';
    if (error.status === 409) return 'The agent is already claimed or its wallet needs attention. Refresh its status.';
  }
  return 'Agent data could not be loaded or the action could not finish. Refresh the status before trying again.';
}
export async function readAgentBalances(reader: AgentChainReader, address: Address): Promise<AgentBalances> {
  const [native, quote, quoteDecimals] = await Promise.all([
    reader.getBalance({ address }),
    reader.readContract({ address: USDL_ADDRESS, abi: erc20Abi, functionName: 'balanceOf', args: [address] }),
    reader.readContract({ address: USDL_ADDRESS, abi: erc20Abi, functionName: 'decimals' }),
  ]);
  return { native, quote, quoteDecimals };
}
export async function loadAgentInventory(wallet: PaxeerWallet, expected: SigningSession, owner: Address, reader: AgentChainReader): Promise<AgentInventory> {
  const record = await wallet.getWallet(expected);
  if (record.wallet.chain_id !== 125 || record.chain.id !== 125 || record.wallet.address.toLowerCase() !== owner.toLowerCase())
    throw new Error('The embedded wallet does not match the connected account on Paxeer.');
  const { agents } = await wallet.listAgents(expected);
  const ownerBalance = await readAgentBalances(reader, owner).then(value => ({ value, error: null }), () => ({ value: null, error: 'Wallet balances could not be loaded.' }));
  const rows = await Promise.all(agents.map(async summary => {
    const detail = await wallet.getAgent(summary.did, expected);
    const balance = detail.wallet ? await readAgentBalances(reader, detail.wallet.address).then(value => ({ value, error: null }), () => ({ value: null, error: 'Agent balances could not be loaded.' })) : { value: null, error: null };
    return { summary, detail, balances: balance.value, balanceError: balance.error };
  }));
  expected.assertCurrent();
  return { rows, ownerBalances: ownerBalance.value, balanceError: ownerBalance.error };
}

export function agentPresetRules(preset: AgentPreset, owner: Address, quoteCap: string): AgentRuleInput[] {
  if (preset === 'read') return [];
  const targets = [ROUTER_ADDRESS, USDL_ADDRESS, ...(preset === 'create' ? [LAUNCHPAD_ADDRESS] : [])];
  const permitted = new Set<string>([...agentPresetMethods.trade, ...(preset === 'create' ? agentPresetMethods.create : [])].map(signature => toFunctionSelector(signature)));
  const abis: Abi[] = [RouterAbi as Abi, erc20Abi, CHAIN_LAUNCH_ABI];
  const denied = new Set<string>();
  for (const abi of abis) for (const item of abi) {
    if (item.type !== 'function' || item.stateMutability === 'view' || item.stateMutability === 'pure') continue;
    const selector = toFunctionSelector(toFunctionSignature(item));
    if (!permitted.has(selector)) denied.add(selector);
  }
  return [
    ...targets.map(value => ({ effect: 'allow' as const, subject: 'contract' as const, value })),
    ...[owner, ROUTER_ADDRESS, ...(preset === 'create' ? [LAUNCHPAD_ADDRESS] : [])].map(value => ({ effect: 'allow' as const, subject: 'withdrawal' as const, value })),
    { effect: 'allow', subject: 'token', value: USDL_ADDRESS, max_value_wei: quoteCap },
    ...Array.from(denied).map(value => ({ effect: 'deny' as const, subject: 'selector' as const, value })),
  ];
}

export async function saveAgentLimits(wallet: PaxeerWallet, expected: SigningSession, row: AgentRow, owner: Address,
  preset: AgentPreset, perTransaction: string, perDay: string, quoteApproval: string): Promise<void> {
  const tx = agentCap(perTransaction, 18);
  const day = agentCap(perDay, 18);
  if (tx > day) throw new Error('The daily PAX cap must be at least the transaction cap.');
  if (!row.balances) throw new Error('Load the balances before saving limits.');
  const quote = agentCap(quoteApproval, row.balances.quoteDecimals);
  const rules = agentPresetRules(preset, owner, quote.toString());
  const did = row.summary.did;
  await wallet.setAgentFrozen(did, true, expected);
  const current = await wallet.getAgent(did, expected);
  await wallet.updateAgentPolicy(did, { mode: 'read_only' }, expected);
  for (const rule of current.rules) {
    const result = await wallet.deleteAgentRule(did, rule.id, expected);
    if (!result.deleted) throw new Error('An agent rule changed while the limits were saving.');
  }
  for (const budget of current.budgets) {
    const result = await wallet.deleteAgentBudget(did, budget.id, expected);
    if (!result.deactivated) throw new Error('An agent budget changed while the limits were saving.');
  }
  for (const rule of rules) await wallet.addAgentRule(did, rule, expected);
  await wallet.addAgentBudget(did, { cap_wei: '0', expires_in_seconds: 86400 }, expected);
  await wallet.updateAgentPolicy(did, {
    mode: preset === 'read' ? 'read_only' : 'trade_only',
    max_tx_value_wei: tx.toString(), max_daily_value_wei: day.toString(),
    max_approve_wei: preset === 'read' ? '0' : quote.toString(),
    allow_native_transfer: false, withdrawal_allowlist_only: true, daily_reset_utc_hour: 0,
  }, expected);
}
