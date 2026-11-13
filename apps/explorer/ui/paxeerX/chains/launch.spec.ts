import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { BaseError, ContractFunctionRevertedError, decodeFunctionResult, encodeErrorResult, encodeFunctionResult } from 'viem';
import type { Abi } from 'viem';

import { expect, test } from 'vitest';

import { chainLaunchAbi, isAbsentLaunch, mapChainLaunch, mapRevenueEpoch } from './launch';
import { parseChainsResponse } from './model';

function chain_launch_decodes_producer_reads_and_formats_revenue() {
  const producerAbi = JSON.parse(readFileSync(resolve(__dirname, '../../../../../precompiles/launchpad/abi.json'), 'utf8')) as Abi;
  for (const name of [ 'launchedChain', 'chainRevenueEpoch' ]) {
    const producer = producerAbi.find((entry) => entry.type === 'function' && entry.name === name);
    const consumer = chainLaunchAbi.find((entry) => entry.type === 'function' && entry.name === name);
    expect(consumer).toBeDefined();
    expect(producer).toBeDefined();
    if (producer?.type !== 'function' || consumer?.type !== 'function') throw new Error('Missing chain launch read');
    expect(consumer.inputs.map(({ name: field, type }) => ({ name: field, type }))).toEqual(
      producer.inputs.map(({ name: field, type }) => ({ name: field, type })),
    );
    expect(consumer.outputs.map(({ name: field, type }) => ({ name: field, type }))).toEqual(
      producer.outputs.map(({ name: field, type }) => ({ name: field, type })),
    );
  }

  const [ chain ] = parseChainsResponse({ chains: [ {
    id: 7, name: 'launch-demo', operator: '0x1111111111111111111111111111111111111111',
    status: 'active', chain_id: 125000007, created_height: 42, bond: '1000000000000000000',
    gas_allowance: 500000, byte_allowance: 65536, fee_share_bps: 100,
  } ] });
  const pool = '0x2222222222222222222222222222222222222222';
  const token = '0x3333333333333333333333333333333333333333';
  const creator = '0x4444444444444444444444444444444444444444';
  const launchData = encodeFunctionResult({
    abi: producerAbi,
    functionName: 'launchedChain',
    result: [ 7, BigInt(125000007), pool, token, BigInt(19), creator, BigInt(42) ],
  });
  const launch = decodeFunctionResult({ abi: chainLaunchAbi, functionName: 'launchedChain', data: launchData });
  const epochData = encodeFunctionResult({
    abi: producerAbi,
    functionName: 'chainRevenueEpoch',
    result: [ 7, BigInt(3), BigInt('1234567890123456789'), BigInt(88), BigInt(1000), BigInt(0), BigInt(90) ],
  });
  const epoch = decodeFunctionResult({ abi: chainLaunchAbi, functionName: 'chainRevenueEpoch', data: epochData });
  const revenue = mapRevenueEpoch(chain, BigInt(3), epoch);
  expect(revenue).toEqual({ number: BigInt(3), amountPax: '1.234567890123456789', snapshotBlock: BigInt(88) });
  expect(mapChainLaunch(chain, launch, 'LAUNCH', revenue)).toEqual({
    token, tokenSymbol: 'LAUNCH', pool, feeRightId: BigInt(19), homeChainId: BigInt(125000007),
    launchBlock: BigInt(42), lastEpoch: revenue,
  });
  expect(mapChainLaunch(chain, launch, 'LAUNCH', null).lastEpoch).toBeNull();
  expect(() => mapRevenueEpoch(chain, BigInt(2), epoch)).toThrow('Revenue epoch belongs to a different virtual chain');
  expect(() => mapChainLaunch({ ...chain, chain_id: 125000008 }, launch, 'LAUNCH', null)).toThrow(
    'Launch belongs to a different virtual chain',
  );

  const absent = new ContractFunctionRevertedError({
    abi: chainLaunchAbi,
    data: encodeErrorResult({ abi: chainLaunchAbi, errorName: 'NotLaunchedChain' }),
    functionName: 'launchedChain',
  });
  expect(isAbsentLaunch(new BaseError('No launch record', { cause: absent }))).toBe(true);
  expect(isAbsentLaunch(new Error('RPC unavailable'))).toBe(false);
}

test(chain_launch_decodes_producer_reads_and_formats_revenue);
