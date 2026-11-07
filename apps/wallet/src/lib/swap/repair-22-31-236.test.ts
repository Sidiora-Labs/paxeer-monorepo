import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { ethers } from 'ethers';
import { decodeFunctionData, encodeFunctionData, isHex, type Abi } from 'viem';
import { expect, test } from 'vitest';
import { NATIVE_PAX, SIDIORA_ADAPTER_ID, VAULT_ADAPTER_ID } from './constants';
import { executeSwap, prepareV4Multihop } from './executor';
import { CONTRACTS, TOKENS } from './sdk';
import type { SwapParams, SwapQuote } from './types';

const producerAbi: Abi = JSON.parse(
  readFileSync(
    resolve(__dirname, '../../../../launchpad/web/src/core/network/abis/MetaAGRouter.json'),
    'utf8'
  )
);
const owner = '0x0000000000000000000000000000000000000125';
const signer = new ethers.VoidSigner(owner);
const launchpad = {
  address: '0x0000000000000000000000000000000000000321',
  name: 'Launchpad',
  symbol: 'LAUNCH',
  decimals: 6,
  isLaunchpad: true,
};

function confirmation(selling: boolean): { quote: SwapQuote; params: SwapParams } {
  const quote: SwapQuote = {
    protocol: 'pecor-v4-multihop',
    protocolLabel: selling ? 'Sidiora → PECOR' : 'PECOR → Sidiora',
    tokenIn: selling ? { ...launchpad } : { ...TOKENS.USDC },
    tokenOut: selling ? { ...TOKENS.USDC } : { ...launchpad },
    poolAddress: '0x0000000000000000000000000000000000000456',
    amountIn: '12345678901234567890',
    amountOut: '9876543210987654321',
    intermediateAmount: '200000000',
    amountOutDisplay: '9876543210987.654321',
    fee: '0',
    feeBps: 0,
    priceImpact: 0,
    sufficient: true,
  };
  return {
    quote,
    params: {
      tokenIn: quote.tokenIn,
      tokenOut: quote.tokenOut,
      amountIn: quote.amountIn,
      slippageBps: 213,
      recipient: owner,
      deadline: Math.floor(Date.now() / 1000) + 617,
    },
  };
}

async function finding_4_atomic_bridge_calldata_preserves_the_confirmed_final_minimum() {
  expect(VAULT_ADAPTER_ID).toBe(ethers.id('PECORVault.v1'));
  expect(SIDIORA_ADAPTER_ID).toBe(ethers.id('SidioraAMM.v1'));
  for (const selling of [true, false]) {
    const { quote, params } = confirmation(selling);
    const prepared = prepareV4Multihop(signer, quote, params);
    const transaction = await prepared.method.populateTransaction(...prepared.args);
    expect(transaction.to).toBe(CONTRACTS.PECORRouter);
    if (!isHex(transaction.data)) throw new Error('Executor calldata is not hex');
    const consumed = decodeFunctionData({ abi: producerAbi, data: transaction.data });
    expect(consumed.functionName).toBe('swapMultiHop');
    const hops = [
      {
        tokenIn: quote.tokenIn.address,
        tokenOut: TOKENS.USDL.address,
        adapterId: selling ? SIDIORA_ADAPTER_ID : VAULT_ADAPTER_ID,
        minAmountOut: 0n,
      },
      {
        tokenIn: TOKENS.USDL.address,
        tokenOut: quote.tokenOut.address,
        adapterId: selling ? VAULT_ADAPTER_ID : SIDIORA_ADAPTER_ID,
        minAmountOut: 9666172840593617283n,
      },
    ];
    const expectedArgs = [
      hops,
      12345678901234567890n,
      9666172840593617283n,
      BigInt(params.deadline!),
    ];
    expect(consumed.args).toEqual(expectedArgs);
    expect(transaction.data).toBe(
      encodeFunctionData({ abi: producerAbi, functionName: 'swapMultiHop', args: expectedArgs })
    );
    // A moved second-leg quote would have authorized a much smaller output.
    const movedOutput = BigInt(quote.amountOut) / 2n;
    const movedMinimum = (movedOutput * BigInt(10000 - params.slippageBps)) / 10000n;
    expect(prepared.args[2]).toBeGreaterThan(movedOutput);
    expect(prepared.args[0][1].minAmountOut).toBeGreaterThan(movedMinimum);
  }
}

async function finding_4_confirmation_cannot_change_while_approval_is_pending() {
  for (const selling of [false, true]) {
    const { quote, params } = confirmation(selling);
    const prepared = prepareV4Multihop(signer, quote, params);
    const originalExpiry = BigInt(params.deadline!);
    const before = await prepared.method.populateTransaction(...prepared.args);
    quote.amountOut = '1';
    quote.amountIn = '1';
    quote.intermediateAmount = '1';
    quote.tokenIn.address = ethers.ZeroAddress;
    quote.tokenOut.address = ethers.ZeroAddress;
    params.slippageBps = 9999;
    params.deadline = Number(originalExpiry) + 86400;
    expect(Reflect.set(prepared.args, '2', 1n)).toBe(false);
    expect(Reflect.set(prepared.args[0][1], 'minAmountOut', 1n)).toBe(false);
    expect(Reflect.set(prepared.args, '3', originalExpiry + 86400n)).toBe(false);
    const after = await prepared.method.populateTransaction(...prepared.args);
    expect(after).toEqual(before);
    expect(prepared.args[2]).toBe(9666172840593617283n);
    expect(prepared.args[3]).toBe(originalExpiry);
  }
}

async function finding_4_expiry_is_absolute_and_default_is_captured_once() {
  const { quote, params } = confirmation(false);
  const absolute = prepareV4Multihop(signer, quote, params);
  expect(absolute.args[3]).toBe(BigInt(params.deadline!));
  delete params.deadline;
  const before = Math.floor(Date.now() / 1000);
  const prepared = prepareV4Multihop(signer, quote, params);
  const after = Math.floor(Date.now() / 1000);
  const expiry = prepared.args[3];
  expect(expiry).toBeGreaterThanOrEqual(BigInt(before + 300));
  expect(expiry).toBeLessThanOrEqual(BigInt(after + 300));
  params.deadline = after + 86400;
  const transaction = await prepared.method.populateTransaction(...prepared.args);
  if (!isHex(transaction.data)) throw new Error('Executor calldata is not hex');
  expect(decodeFunctionData({ abi: producerAbi, data: transaction.data }).args?.[3]).toBe(expiry);
}

async function finding_4_executor_rejects_expired_or_unprotected_confirmation_before_approval() {
  for (const selling of [false, true]) {
    for (const deadline of [0, -1, 300, Math.floor(Date.now() / 1000) - 1, NaN, Infinity, 1.5]) {
      const { quote, params } = confirmation(selling);
      params.deadline = deadline;
      await expect(executeSwap(signer, quote, params)).rejects.toThrow(
        'V4 multihop confirmation has expired or has an invalid expiry'
      );
    }
    for (const slippageBps of [-1, 10000, 10001, 0.5, NaN, Infinity]) {
      const { quote, params } = confirmation(selling);
      params.slippageBps = slippageBps;
      await expect(executeSwap(signer, quote, params)).rejects.toThrow(
        'Invalid V4 multihop slippage'
      );
    }
    const { quote, params } = confirmation(selling);
    quote.amountOut = '1';
    await expect(executeSwap(signer, quote, params)).rejects.toThrow(
      'V4 multihop requires positive input and minimum output'
    );
    params.slippageBps = 0;
    expect(prepareV4Multihop(signer, quote, params).args[2]).toBe(1n);
    quote.amountOut = '0';
    await expect(executeSwap(signer, quote, params)).rejects.toThrow(
      'V4 multihop requires positive input and minimum output'
    );
  }
}

async function finding_4_atomic_route_rejects_native_or_non_bridge_pairs() {
  for (const selling of [false, true]) {
    const { quote, params } = confirmation(selling);
    if (selling) quote.tokenOut = NATIVE_PAX;
    else quote.tokenIn = NATIVE_PAX;
    await expect(executeSwap(signer, quote, params)).rejects.toThrow('Invalid V4 multihop route');
  }
  for (const token of [TOKENS.USDL, launchpad, TOKENS.USDT]) {
    const { quote, params } = confirmation(false);
    quote.tokenIn = { ...token };
    if (token === TOKENS.USDT) quote.tokenOut = { ...TOKENS.USDC };
    await expect(executeSwap(signer, quote, params)).rejects.toThrow('Invalid V4 multihop route');
  }
}

test(
  finding_4_atomic_bridge_calldata_preserves_the_confirmed_final_minimum.name,
  finding_4_atomic_bridge_calldata_preserves_the_confirmed_final_minimum
);
test(
  finding_4_confirmation_cannot_change_while_approval_is_pending.name,
  finding_4_confirmation_cannot_change_while_approval_is_pending
);
test(
  finding_4_expiry_is_absolute_and_default_is_captured_once.name,
  finding_4_expiry_is_absolute_and_default_is_captured_once
);
test(
  finding_4_executor_rejects_expired_or_unprotected_confirmation_before_approval.name,
  finding_4_executor_rejects_expired_or_unprotected_confirmation_before_approval
);
test(
  finding_4_atomic_route_rejects_native_or_non_bridge_pairs.name,
  finding_4_atomic_route_rejects_native_or_non_bridge_pairs
);
