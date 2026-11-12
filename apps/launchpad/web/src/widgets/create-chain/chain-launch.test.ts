import { readFileSync } from 'node:fs';
import { expect, test } from 'vitest';
import { encodeFunctionData } from 'viem';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';
import { chainLaunchRequest } from './chain-adapters';
import { chainCost, validateChainDraft, validateChainName, type ChainDraft, type ChainsParams } from './chain-domain';

const producer = JSON.parse(readFileSync(new URL('../../../../../../precompiles/launchpad/abi.json', import.meta.url), 'utf8')) as Array<{
  type: string; name?: string; inputs?: Array<{ name: string; type: string; indexed?: boolean }>;
  outputs?: Array<{ name: string; type: string }>; stateMutability?: string;
}>;

function chain_launch_uses_producer_function_and_event_names() {
  for (const name of ['launchChain', 'launchedChain', 'ChainLaunched'] as const) {
    const local = CHAIN_LAUNCH_ABI.find(item => item.name === name)!;
    const published = producer.find(item => item.name === name);
    expect(published).toBeDefined();
    expect(local.type).toBe(published!.type);
    expect(local.inputs.map(input => ({ name: 'name' in input ? input.name : undefined, type: input.type, indexed: 'indexed' in input && input.indexed === true })))
      .toEqual(published!.inputs!.map(input => ({ name: input.name, type: input.type, indexed: input.indexed === true })));
    if (local.type === 'function') {
      expect(local.outputs.map(output => ({ name: 'name' in output ? output.name : undefined, type: output.type })))
        .toEqual(published!.outputs!.map(output => ({ name: output.name, type: output.type })));
      expect(local.stateMutability).toBe(published!.stateMutability);
    }
  }
}

function chain_launch_sends_eight_reviewed_arguments_and_chain_value() {
  const creator = '0x1111111111111111111111111111111111111111';
  const draft: ChainDraft = {
    draftId: 'launch-1', version: 2, creator, chainId: 125, name: 'my-chain',
    tokenName: 'My Token', tokenSymbol: 'MYT', feeStrategy: 2,
    tier: 'growth', feeShareBps: 150, updatedAt: 1700000000000,
  };
  const params: ChainsParams = [creator, 23n * 10n ** 18n, 2n * 10n ** 18n, 500000n, 4000000n, 10000000n, 100];
  expect(validateChainDraft(draft, draft.updatedAt)).toEqual({});
  const cost = chainCost(params, draft.tier, '60000000000000');
  const request = chainLaunchRequest(draft, creator, BigInt(cost.totalValueRaw));
  expect(request.address).toBe(LAUNCHPAD_ADDRESS);
  expect(request.functionName).toBe('launchChain');
  expect(request.args).toEqual(['My Token', 'MYT', 2, '0x0000000000000000000000000000000000000000',
    'my-chain', 1500000n, 262144n, 150]);
  expect(request.value).toBe(params[1] + params[2]);
  expect(encodeFunctionData(request).slice(0, 10)).toBe(encodeFunctionData({
    abi: CHAIN_LAUNCH_ABI, functionName: 'launchChain', args: request.args,
  }).slice(0, 10));
  expect(validateChainName('Taken Name')).toBe('invalid_chain_name');
  expect(validateChainDraft({ ...draft, chainId: 1 as 125 }, draft.updatedAt).chainId).toBe('wrong_chain');
}

test(chain_launch_uses_producer_function_and_event_names.name, chain_launch_uses_producer_function_and_event_names);
test(chain_launch_sends_eight_reviewed_arguments_and_chain_value.name, chain_launch_sends_eight_reviewed_arguments_and_chain_value);
