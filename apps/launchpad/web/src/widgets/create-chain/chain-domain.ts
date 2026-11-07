import { decodeEventLog, parseAbi, type AddEthereumChainParameter, type Hex } from 'viem';
import { address, raw } from '../create/creation-domain';

export const CHAINS_PRECOMPILE = '0x000000000000000000000000000000000000101e';
export const VIRTUAL_CHAIN_ID_BASE = 125000000;
export const SIZE_TIERS = Object.freeze({
  starter: Object.freeze({ gas: 500000, bytes: 65536 }),
  growth: Object.freeze({ gas: 1500000, bytes: 262144 }),
  pro: Object.freeze({ gas: 4000000, bytes: 1048576 }),
});
export type SizeTier = keyof typeof SIZE_TIERS;

export function chainIdOf(id: number): number {
  if (!Number.isSafeInteger(id) || id < 1 || id > 0xffffffff) throw new Error('invalid_chain_id');
  return VIRTUAL_CHAIN_ID_BASE + id;
}

export function validateChainName(name: string): 'invalid_chain_name' | null {
  return typeof name === 'string' && name === name.trim() && /^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(name)
    ? null : 'invalid_chain_name';
}

function isSizeTier(tier: string): tier is SizeTier {
  return typeof tier === 'string' && Object.prototype.hasOwnProperty.call(SIZE_TIERS, tier);
}

export interface ChainDraft {
  draftId: string;
  version: number;
  creator: string;
  chainId: 125;
  name: string;
  tier: SizeTier;
  feeShareBps: number;
  updatedAt: number;
}

export function validateChainDraft(draft: ChainDraft, now: number): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!draft || typeof draft !== 'object') return { draft: 'invalid_draft' };
  if (typeof draft.draftId !== 'string' || !/^[A-Za-z0-9_-]{1,128}$/.test(draft.draftId)
    || !Number.isSafeInteger(draft.version) || draft.version < 1) errors.draftId = 'invalid_identity';
  try { address(draft.creator); } catch { errors.creator = 'invalid_creator'; }
  if (draft.chainId !== 125) errors.chainId = 'wrong_chain';
  const nameError = validateChainName(draft.name);
  if (nameError) errors.name = nameError;
  if (!isSizeTier(draft.tier)) errors.tier = 'invalid_tier';
  if (!Number.isInteger(draft.feeShareBps) || draft.feeShareBps < 0 || draft.feeShareBps > 5000) {
    errors.feeShareBps = 'invalid_fee_share';
  }
  if (!Number.isSafeInteger(now) || !Number.isSafeInteger(draft.updatedAt)
    || now < draft.updatedAt || now - draft.updatedAt > 30 * 86400000) errors.updatedAt = 'draft_expired';
  return errors;
}

/** The decoded return tuple of the chains precompile's params() view. */
export type ChainsParams = readonly [
  authority: string,
  minBond: bigint,
  creationFee: bigint,
  minGasAllowance: bigint,
  maxGasAllowance: bigint,
  totalGasAllowanceLeft: bigint,
  maxChains: number,
];

export interface ChainCost {
  bondRaw: string;
  feeRaw: string;
  totalValueRaw: string;
  gasEstimatePaxRaw: string;
}

export function chainCost(params: ChainsParams, tier: SizeTier, gasEstimatePaxRaw: string): Readonly<ChainCost> {
  if (!Array.isArray(params) || params.length !== 7) throw new Error('invalid_chain_params');
  const [authority, minBond, creationFee, minGas, maxGas, gasLeft, maxChains] = params;
  address(authority);
  for (const value of [minBond, creationFee, minGas, maxGas, gasLeft]) {
    if (typeof value !== 'bigint') throw new Error('invalid_chain_params');
    raw(value.toString());
  }
  if ([minGas, maxGas, gasLeft].some(value => value > (1n << 64n) - 1n)
    || minGas > maxGas || !Number.isInteger(maxChains) || maxChains < 0 || maxChains > 0xffffffff) {
    throw new Error('invalid_chain_params');
  }
  if (!isSizeTier(tier)) throw new Error('invalid_tier');
  const gas = BigInt(SIZE_TIERS[tier].gas);
  if (gas < minGas || gas > maxGas || gas > gasLeft) throw new Error('tier_unavailable');
  const totalValueRaw = (minBond + creationFee).toString();
  raw(totalValueRaw);
  // Gas is funded separately; createChain receives only the bond plus creation fee.
  raw(gasEstimatePaxRaw);
  return Object.freeze({ bondRaw: minBond.toString(), feeRaw: creationFee.toString(), totalValueRaw, gasEstimatePaxRaw });
}

const chainCreatedAbi = parseAbi([
  'event ChainCreated(uint32 indexed id,string name,address indexed operator,uint64 chainId)',
]);

export interface ChainRecord {
  id: number;
  name: string;
  operator: string;
  chainId: number;
}

export function decodeChainCreated(log: { address: string; topics: readonly Hex[]; data: Hex }): Readonly<ChainRecord> {
  if (address(log.address) !== CHAINS_PRECOMPILE || log.topics.length !== 3) throw new Error('invalid_chain_event');
  const { args } = decodeEventLog({
    abi: chainCreatedAbi,
    eventName: 'ChainCreated',
    topics: [...log.topics] as [Hex, ...Hex[]],
    data: log.data,
    strict: true,
  });
  const chainId = chainIdOf(args.id);
  if (validateChainName(args.name) || args.chainId !== BigInt(chainId)) throw new Error('invalid_chain_event');
  return Object.freeze({ id: args.id, name: args.name, operator: address(args.operator), chainId });
}

function networkUrl(base: string, path: string): string {
  const url = new URL(base);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error('invalid_network_url');
  }
  return `${url.href.replace(/\/+$/, '')}${path}`;
}

export function walletNetwork(record: Pick<ChainRecord, 'id' | 'name' | 'chainId'>, rpcBase: string, explorerBase: string) {
  const chainId = chainIdOf(record.id);
  if (record.chainId !== chainId || validateChainName(record.name)) throw new Error('invalid_chain_record');
  return {
    chainId: `0x${chainId.toString(16)}`,
    chainName: `${record.name} on Paxeer`,
    rpcUrls: [networkUrl(rpcBase, `/chain/${record.id}`)],
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    blockExplorerUrls: [networkUrl(explorerBase, `/chains/${record.id}`)],
  } satisfies AddEthereumChainParameter;
}
