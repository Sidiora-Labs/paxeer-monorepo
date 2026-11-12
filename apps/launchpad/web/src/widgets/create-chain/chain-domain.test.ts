import { expect, test } from 'vitest';
import { decodeFunctionResult, encodeAbiParameters, encodeEventTopics, encodeFunctionResult, parseAbi, type Address, type Hex } from 'viem';
import * as c from './chain-domain';
import vectors from '../../../../../../tests/vectors/chains/abi-vectors.json';

function chain_domain_validates_names_tiers_cost_and_wallet_params() {
  expect(c.CHAINS_PRECOMPILE).toBe('0x000000000000000000000000000000000000101e');
  expect(c.VIRTUAL_CHAIN_ID_BASE).toBe(125000000);
  expect(c.SIZE_TIERS).toEqual({
    starter: { gas: 500000, bytes: 65536 },
    growth: { gas: 1500000, bytes: 262144 },
    pro: { gas: 4000000, bytes: 1048576 },
  });
  for (const name of ['ab', '-abc', 'abc-', 'ABC', 'a'.repeat(33), 'a b', 'abc\n', '', null, 123]) {
    // @ts-expect-error Runtime inputs can violate the declared name type.
    expect(c.validateChainName(name)).toBe('invalid_chain_name');
  }
  for (const name of ['demo', 'abc', 'a'.repeat(32), 'demo-123', '123']) expect(c.validateChainName(name)).toBe(null);
  for (const id of [0, -1, 1.5, NaN, '1', 0x100000000]) {
    // @ts-expect-error Exercise malformed input from outside TypeScript.
    expect(() => c.chainIdOf(id)).toThrow(/invalid_chain_id/);
  }
  expect(c.chainIdOf(1)).toBe(125000001);
  expect(c.chainIdOf(0xffffffff)).toBe(125000000 + 0xffffffff);

  const operator = '0x1111111111111111111111111111111111111111';
  const now = 1700000000000;
  const draft: c.ChainDraft = { draftId: 'chain-1', version: 2, creator: operator, chainId: 125, name: 'demo',
    tokenName: 'Demo Token', tokenSymbol: 'DMT', feeStrategy: 0, tier: 'pro', feeShareBps: 60, updatedAt: now };
  expect(c.validateChainDraft(draft, now)).toEqual({});
  expect(c.validateChainDraft(draft, now + 30 * 86400000)).toEqual({});
  for (const feeShareBps of [0, 5000]) expect(c.validateChainDraft({ ...draft, feeShareBps }, now)).toEqual({});
  for (const [field, value, error] of [
    ['creator', '0x0000000000000000000000000000000000000000', 'invalid_creator'],
    ['creator', 'not-an-address', 'invalid_creator'],
    ['chainId', 1, 'wrong_chain'],
    ['name', 'ABC', 'invalid_chain_name'],
    ['tokenName', '', 'invalid_text'],
    ['tokenName', 'a'.repeat(51), 'invalid_text'],
    ['tokenSymbol', '', 'invalid_text'],
    ['tokenSymbol', 'a'.repeat(11), 'invalid_text'],
    ['feeStrategy', 4, 'invalid_fee_strategy'],
    ['feeStrategy', -1, 'invalid_fee_strategy'],
    ['tier', 'enterprise', 'invalid_tier'],
    ['tier', 'toString', 'invalid_tier'],
    ['feeShareBps', -1, 'invalid_fee_share'],
    ['feeShareBps', 5001, 'invalid_fee_share'],
    ['feeShareBps', 1.5, 'invalid_fee_share'],
    ['feeShareBps', '60', 'invalid_fee_share'],
    ['draftId', '', 'invalid_identity'],
    ['version', 1, 'invalid_identity'],
    ['updatedAt', now + 1, 'draft_expired'],
    ['updatedAt', NaN, 'draft_expired'],
  ]) expect(c.validateChainDraft({ ...draft, [field]: value }, now)[field === 'version' ? 'draftId' : field]).toBe(error);
  expect(c.validateChainDraft(draft, now + 30 * 86400000 + 1).updatedAt).toBe('draft_expired');
  expect(c.validateChainDraft(draft, NaN).updatedAt).toBe('draft_expired');
  // @ts-expect-error A persisted draft can be null at runtime.
  expect(c.validateChainDraft(null, now)).toEqual({ draft: 'invalid_draft' });

  const abi = parseAbi([
    'function params() view returns (address authority,uint256 minBond,uint256 creationFee,uint64 minGasAllowance,uint64 maxGasAllowance,uint64 totalGasAllowanceLeft,uint32 maxChains)',
    'event ChainCreated(uint32 indexed id,string name,address indexed operator,uint64 chainId)',
  ]);
  const params = decodeFunctionResult({
    abi, functionName: 'params',
    data: encodeFunctionResult({ abi, functionName: 'params', result: [operator, 100n * 10n ** 18n, 10n * 10n ** 18n, 500000n, 4000000n, 1000000n, 100] }),
  });
  const gasEstimatePaxRaw = (60000n * 1000000000n).toString();
  expect(() => c.chainCost(params, draft.tier, gasEstimatePaxRaw)).toThrow(/tier_unavailable/);
  const starterDraft: c.ChainDraft = { ...draft, tier: 'starter' };
  expect(c.validateChainDraft(starterDraft, now)).toEqual({});
  const cost = c.chainCost(params, starterDraft.tier, gasEstimatePaxRaw);
  expect(cost).toEqual({ bondRaw: '100000000000000000000', feeRaw: '10000000000000000000', totalValueRaw: '110000000000000000000', gasEstimatePaxRaw });
  expect(Object.isFrozen(cost)).toBe(true);
  const maxLimited: [...c.ChainsParams] = [...params]; maxLimited[4] = 499999n;
  maxLimited[3] = 1n;
  expect(() => c.chainCost(maxLimited, 'starter', gasEstimatePaxRaw)).toThrow(/tier_unavailable/);
  const exhausted: [...c.ChainsParams] = [...params]; exhausted[5] = 499999n;
  expect(() => c.chainCost(exhausted, 'starter', gasEstimatePaxRaw)).toThrow(/tier_unavailable/);
  const enough: [...c.ChainsParams] = [...params]; enough[5] = 4000000n;
  for (const tier of ['starter', 'growth', 'pro'] as const) expect(c.chainCost(enough, tier, gasEstimatePaxRaw).totalValueRaw).toBe(cost.totalValueRaw);
  const exactAllowance: [...c.ChainsParams] = [...params]; exactAllowance[4] = exactAllowance[5] = 500000n;
  expect(c.chainCost(exactAllowance, 'starter', gasEstimatePaxRaw)).toEqual(cost);
  const minimumLimited: [...c.ChainsParams] = [...params]; minimumLimited[3] = 500001n;
  expect(() => c.chainCost(minimumLimited, 'starter', gasEstimatePaxRaw)).toThrow(/tier_unavailable/);
  for (const value of [null, [], params.slice(0, 6)]) {
    // @ts-expect-error Reject absent or incomplete ABI tuples at runtime.
    expect(() => c.chainCost(value, 'starter', gasEstimatePaxRaw)).toThrow(/invalid_chain_params/);
  }
  for (const [index, value] of [[1, '100'], [3, 1n << 64n], [4, 499999n], [5, 1n << 64n], [6, 1.5], [6, 0x100000000]] as const) {
    const malformed: unknown[] = [...params]; malformed[index] = value;
    // @ts-expect-error Deliberately malformed ABI fields must be rejected.
    expect(() => c.chainCost(malformed, 'starter', gasEstimatePaxRaw)).toThrow(/invalid_chain_params/);
  }
  const negativeFee: [...c.ChainsParams] = [...params]; negativeFee[2] = -1n;
  expect(() => c.chainCost(negativeFee, 'starter', gasEstimatePaxRaw)).toThrow(/invalid_raw_amount/);
  // @ts-expect-error Object prototype keys are not supported size tiers.
  expect(() => c.chainCost(params, 'toString', gasEstimatePaxRaw)).toThrow(/invalid_tier/);
  expect(() => c.chainCost(params, 'starter', '-1')).toThrow(/invalid_raw_amount/);
  const overflow: [...c.ChainsParams] = [...params]; overflow[1] = (1n << 256n) - 1n;
  expect(() => c.chainCost(overflow, 'starter', gasEstimatePaxRaw)).toThrow(/invalid_raw_amount/);

  const log: Parameters<typeof c.decodeChainCreated>[0] = {
    address: c.CHAINS_PRECOMPILE,
    topics: encodeEventTopics({ abi, eventName: 'ChainCreated', args: { id: 1, operator: starterDraft.creator as Address } }) as Hex[],
    data: encodeAbiParameters([{ type: 'string' }, { type: 'uint64' }], [starterDraft.name, 125000001n]),
  };
  expect(log.topics[0]).toBe(vectors.events.find(event => event.name === 'ChainCreated')!.topic0);
  expect(log.topics[1]).toBe(`0x${'1'.padStart(64, '0')}`);
  expect(log.topics[2]).toBe(`0x${operator.slice(2).padStart(64, '0')}`);
  const record = c.decodeChainCreated(log);
  expect(record).toEqual({ id: 1, name: 'demo', operator, chainId: 125000001 });
  expect(() => c.decodeChainCreated({ ...log, address: operator })).toThrow(/invalid_chain_event/);
  expect(() => c.decodeChainCreated({ ...log, topics: log.topics.slice(0, 2) })).toThrow(/invalid_chain_event/);
  expect(() => c.decodeChainCreated({ ...log, topics: [`0x${'0'.repeat(64)}`, ...log.topics.slice(1)] })).toThrow();
  expect(() => c.decodeChainCreated({ ...log, data: '0x' })).toThrow();
  expect(() => c.decodeChainCreated({ ...log, data: encodeAbiParameters([{ type: 'string' }, { type: 'uint64' }], ['demo', 125000002n]) })).toThrow(/invalid_chain_event/);
  expect(() => c.decodeChainCreated({ ...log, data: encodeAbiParameters([{ type: 'string' }, { type: 'uint64' }], ['ABC', 125000001n]) })).toThrow(/invalid_chain_event/);
  expect(() => c.decodeChainCreated({ ...log, topics: encodeEventTopics({ abi, eventName: 'ChainCreated', args: { id: 0, operator } }) as Hex[] })).toThrow(/invalid_chain_id/);

  const network = c.walletNetwork(record, 'https://rpc.example/evm/', 'https://explorer.example/');
  expect(network).toEqual({
    chainId: '0x7735941', chainName: 'demo on Paxeer',
    rpcUrls: ['https://rpc.example/evm/chain/1'],
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    blockExplorerUrls: ['https://explorer.example/chains/1'],
  });
  expect(() => c.walletNetwork({ ...record, chainId: 125 }, 'https://rpc.example', 'https://explorer.example')).toThrow(/invalid_chain_record/);
  for (const base of ['javascript:alert(1)', 'https://user:password@rpc.example', 'https://rpc.example?key=value']) {
    expect(() => c.walletNetwork(record, base, 'https://explorer.example')).toThrow(/invalid_network_url/);
  }
}

test(chain_domain_validates_names_tiers_cost_and_wallet_params.name, chain_domain_validates_names_tiers_cost_and_wallet_params);
