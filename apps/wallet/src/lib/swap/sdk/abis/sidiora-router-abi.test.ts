import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { ethers } from 'ethers';
import {
  decodeFunctionData,
  decodeFunctionResult,
  encodeFunctionData,
  encodeFunctionResult,
  isHex,
  type Abi,
} from 'viem';
import { expect, test } from 'vitest';
import { prepareSidioraMultihop } from '../../executor';
import type { SwapParams, SwapQuote } from '../../types';
import { CONTRACTS, TOKENS } from '../addresses';
import { SIDIORA_ROUTER_ABI } from './Application';

const producerAbiJson = readFileSync(
  resolve(__dirname, '../../../../../../../precompiles/launchpad/abi/Router.json'),
  'utf8'
);
const producerAbi: Abi = JSON.parse(producerAbiJson);
const producerInterface = new ethers.Interface(producerAbiJson);
const walletInterface = new ethers.Interface(SIDIORA_ROUTER_ABI);
const owner = '0x0000000000000000000000000000000000000125';
const quote: SwapQuote = {
  protocol: 'sidiora-multihop',
  protocolLabel: 'Sidiora',
  tokenIn: { ...TOKENS.SID, isLaunchpad: true },
  tokenOut: { ...TOKENS.USID, isLaunchpad: true },
  amountIn: '12345678901234567890',
  amountOut: '9876543210987654321',
  amountOutDisplay: '9876543210987.654321',
  fee: '0',
  feeBps: 0,
  priceImpact: 0,
  sufficient: true,
};
const params: SwapParams = {
  tokenIn: quote.tokenIn,
  tokenOut: quote.tokenOut,
  amountIn: quote.amountIn,
  slippageBps: 213,
  recipient: owner,
};

function finding_1_router_roles_match_the_served_generation_2_abi() {
  for (const name of ['buy', 'sell', 'swapTokenForToken']) {
    const produced = producerInterface.getFunction(name);
    const consumed = walletInterface.getFunction(name);
    expect(produced).not.toBeNull();
    expect(consumed?.format('full')).toBe(produced?.format('full'));
  }
  expect(walletInterface.getFunction('swapTokenToToken')).toBeNull();
  expect(walletInterface.getFunction('swapTokenForToken')?.selector).toBe('0x8ef5184b');
}

async function finding_1_executor_encodes_the_generation_2_multihop_call() {
  for (const deadline of [undefined, 617]) {
    const before = Math.floor(Date.now() / 1000);
    const prepared = prepareSidioraMultihop(new ethers.VoidSigner(owner), quote, {
      ...params,
      deadline,
    });
    const transaction = await prepared.method.populateTransaction(...prepared.args);
    const after = Math.floor(Date.now() / 1000);
    expect(transaction.to).toBe(CONTRACTS.Router);
    expect(transaction.data.slice(0, 10)).toBe('0x8ef5184b');

    if (!isHex(transaction.data)) throw new Error('Executor calldata is not hex');
    const producedCall = decodeFunctionData({ abi: producerAbi, data: transaction.data });
    expect(producedCall.functionName).toBe('swapTokenForToken');
    const expectedArguments = [
      quote.tokenIn.address,
      quote.tokenOut.address,
      12345678901234567890n,
      9666172840593617283n,
      prepared.args[4],
    ] as const;
    expect(producedCall.args).toEqual(expectedArguments);
    expect(prepared.args[4]).toBeGreaterThanOrEqual(BigInt(before + (deadline ?? 300)));
    expect(prepared.args[4]).toBeLessThanOrEqual(BigInt(after + (deadline ?? 300)));
    expect(transaction.data).toBe(
      encodeFunctionData({
        abi: producerAbi,
        functionName: 'swapTokenForToken',
        args: expectedArguments,
      })
    );
    const walletCall = prepared.router.interface.parseTransaction({ data: transaction.data });
    expect(walletCall?.name).toBe('swapTokenForToken');
    expect(walletCall?.args.toArray()).toEqual(expectedArguments);
    expect(walletCall?.value).toBe(0n);
  }
}

function finding_1_wallet_decodes_both_generation_2_return_amounts() {
  const prepared = prepareSidioraMultihop(new ethers.VoidSigner(owner), quote, params);
  for (const [amountOut, intermediateUsdl] of [
    [0n, 0n],
    [9876543210987654321n, 1234567890123456789n],
    [ethers.MaxUint256, ethers.MaxUint256 - 1n],
  ]) {
    const produced = encodeFunctionResult({
      abi: producerAbi,
      functionName: 'swapTokenForToken',
      result: [amountOut, intermediateUsdl],
    });
    const consumed = prepared.router.interface.decodeFunctionResult(
      prepared.method.fragment,
      produced
    );
    expect(consumed.toArray()).toEqual([amountOut, intermediateUsdl]);
    expect(consumed.amountOut).toBe(amountOut);
    expect(consumed.intermediateUsdl).toBe(intermediateUsdl);
    const returned = prepared.router.interface.encodeFunctionResult(
      prepared.method.fragment,
      consumed
    );
    if (!isHex(returned)) throw new Error('Wallet return encoding is not hex');
    expect(
      decodeFunctionResult({
        abi: producerAbi,
        functionName: 'swapTokenForToken',
        data: returned,
      })
    ).toEqual([amountOut, intermediateUsdl]);
    expect(() =>
      prepared.router.interface.decodeFunctionResult(
        prepared.method.fragment,
        produced.slice(0, 66)
      )
    ).toThrow();
  }
}

test(finding_1_router_roles_match_the_served_generation_2_abi);
test(finding_1_executor_encodes_the_generation_2_multihop_call);
test(finding_1_wallet_decodes_both_generation_2_return_amounts);
