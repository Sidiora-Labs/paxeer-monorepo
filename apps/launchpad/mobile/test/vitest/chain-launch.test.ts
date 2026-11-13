import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { decodeFunctionData } from 'viem';
import {
  validateChainDetailResponse,
  validateChainRevenueResponse,
} from '../../../../../sdk/launchpad/packages/sdk/src/validate.ts';
import type {
  ChainDetailResponse,
  ChainRevenueResponse,
} from '../../../../../sdk/launchpad/packages/sdk/src/chains.ts';
import {
  chainRoute,
  chainLaunchAbi,
  chainRevenueClaimRequest,
  launchpadAddress,
} from '../../src/runtime/chain-launch.ts';
import { decodeTradeRequest } from '../../src/runtime/trade-recovery.ts';

function chain_launch_home_routes() {
  const rootRPC = 'https://api1.mainnet-beta.paxeer.network';
  expect(chainRoute(0n, rootRPC)).toEqual({
    chainId: 125,
    registryNumber: null,
    rpcURL: rootRPC,
  });
  expect(chainRoute(125000007n, rootRPC)).toEqual({
    chainId: 125000007,
    registryNumber: '7',
    rpcURL: `${rootRPC}/chain/7`,
  });
  expect(() => chainRoute(125000000n, rootRPC)).toThrow();
  expect(() => chainRoute(4419967296n, rootRPC)).toThrow();
  const root = decodeTradeRequest({
    chainId: '125',
    from: '0x0000000000000000000000000000000000000001',
    to: launchpadAddress,
    data: '0x',
    valueRaw: '0',
    gasLimitRaw: null,
    maxFeePerGasRaw: null,
    maxPriorityFeePerGasRaw: null,
  });
  const home = decodeTradeRequest({ ...root, chainId: '125000007' });
  expect(root.chainId).toBe('125');
  expect(home.chainId).toBe('125000007');
  expect(() => decodeTradeRequest({ ...root, chainId: '126' })).toThrow();
}
test(chain_launch_home_routes);

function chain_launch_claim_call() {
  const holder = '0x0000000000000000000000000000000000000001';
  const request = chainRevenueClaimRequest('7', ['2', '4'], holder);
  expect(request.chainId).toBe('125');
  expect(request.from).toBe(holder);
  expect(request.to).toBe(launchpadAddress);
  expect(request.valueRaw).toBe('0');
  const decoded = decodeFunctionData({
    abi: chainLaunchAbi,
    data: request.data as `0x${string}`,
  });
  expect(decoded.functionName).toBe('claimChainRevenueBatch');
  expect(decoded.args).toEqual([7, [2n, 4n]]);
  expect(() => chainRevenueClaimRequest('7', ['2', '2'], holder)).toThrow();
}
test(chain_launch_claim_call);

function chain_launch_abi_names() {
  const source = JSON.parse(
    readFileSync(
      new URL('../../../../../precompiles/launchpad/abi.json', import.meta.url),
      'utf8',
    ),
  ) as Array<{ type: string; name?: string; inputs?: Array<{ type: string }> }>;
  const names = new Map(
    source
      .filter((item) => item.type === 'function')
      .map((item) => [item.name, item.inputs?.map((input) => input.type)]),
  );
  for (const name of [
    'homeChainOf',
    'chainRevenueEpoch',
    'chainRevenueClaimable',
    'claimChainRevenueBatch',
  ])
    expect(names.has(name)).toBe(true);
  expect(names.get('homeChainOf')).toEqual(['address']);
  expect(names.get('chainRevenueClaimable')).toEqual([
    'uint32',
    'uint64',
    'address',
  ]);
  expect(names.get('claimChainRevenueBatch')).toEqual(['uint32', 'uint64[]']);
}
test(chain_launch_abi_names);

function chain_launch_api_contract() {
  const envelope = {
    requestId: 'chain-read',
    revision: '4',
    watermark: null,
    quality: 'complete',
    nextCursor: null,
    generatedAt: '2026-11-11T00:00:00.000Z',
  } as const;
  const chain = {
    chain: '7',
    chainId: '125000007',
    pool: '0x0000000000000000000000000000000000000002',
    token: '0x0000000000000000000000000000000000000003',
    nftId: '5',
    creator: '0x0000000000000000000000000000000000000001',
    blockNumber: '88',
    status: 'active',
  } as const;
  const detail: ChainDetailResponse = validateChainDetailResponse({
    ...envelope,
    data: { chain },
  });
  const revenue: ChainRevenueResponse = validateChainRevenueResponse({
    ...envelope,
    data: {
      chain: '7',
      epochs: [
        {
          epoch: '2',
          epochAmount: '900',
          snapshotBlock: '90',
          eligibleSupply: '1000',
        },
      ],
      holder: chain.creator,
      claims: [],
    },
  });
  expect(detail.data.chain.chainId).toBe('125000007');
  expect(revenue.data.epochs[0].epochAmount).toBe('900');
  expect(() =>
    validateChainDetailResponse({
      ...envelope,
      data: { chain: { ...chain, chainId: 125000007 } },
    }),
  ).toThrow();
}
test(chain_launch_api_contract);
