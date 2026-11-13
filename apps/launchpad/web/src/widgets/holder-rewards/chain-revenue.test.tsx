import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, test } from 'vitest';
import { decodeFunctionData, encodeFunctionData } from 'viem';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';
import { chainRevenueWriteRequest } from './chain-revenue';

describe('chain revenue', () => {
  test('encodes one epoch claim with its registry number', function chainRevenueSingleClaimArguments() {
    const request = chainRevenueWriteRequest(7, 'claim', [3n]);
    expect(request.address).toBe(LAUNCHPAD_ADDRESS);
    const decoded = decodeFunctionData({ abi: CHAIN_LAUNCH_ABI, data: encodeFunctionData(request) });
    expect(decoded.functionName).toBe('claimChainRevenue');
    expect(decoded.args).toEqual([7, 3n]);
  });

  test('encodes all open epochs in one batch', function chainRevenueBatchClaimArguments() {
    const request = chainRevenueWriteRequest(7, 'batch', [1n, 3n, 4n]);
    const decoded = decodeFunctionData({ abi: CHAIN_LAUNCH_ABI, data: encodeFunctionData(request) });
    expect(decoded.functionName).toBe('claimChainRevenueBatch');
    expect(decoded.args).toEqual([7, [1n, 3n, 4n]]);
  });

  test('opens the next epoch with the chain number', function chainRevenueOpenArguments() {
    const request = chainRevenueWriteRequest(7, 'open');
    const decoded = decodeFunctionData({ abi: CHAIN_LAUNCH_ABI, data: encodeFunctionData(request) });
    expect(decoded.functionName).toBe('openChainRevenueEpoch');
    expect(decoded.args).toEqual([7]);
  });

  test('matches the published launchpad function names', function chainRevenueAbiNames() {
    const published = JSON.parse(readFileSync(resolve(process.cwd(), '../../../precompiles/launchpad/abi.json'), 'utf8')) as Array<{ type: string; name: string }>;
    const names = new Set(published.filter(item => item.type === 'function').map(item => item.name));
    for (const name of ['chainRevenueEpoch', 'chainRevenueClaimable', 'claimChainRevenue', 'claimChainRevenueBatch', 'openChainRevenueEpoch']) {
      expect(names.has(name)).toBe(true);
      expect(CHAIN_LAUNCH_ABI.some(item => item.type === 'function' && item.name === name)).toBe(true);
    }
  });
});
