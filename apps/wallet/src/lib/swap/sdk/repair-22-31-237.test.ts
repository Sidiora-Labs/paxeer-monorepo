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
import { pecorOrdersAbi as producerOrdersAbi } from '../../../../../perps/src/core/network/spot_abi/PECOROrders';
import { pecorStopOrdersAbi as producerStopsAbi } from '../../../../../perps/src/core/network/spot_abi/PECORStopOrders';
import { CONTRACTS, MAX_UINT256, TOKENS } from './addresses';
import { PECOR_ORDERS_ABI, PECOR_STOP_ORDERS_ABI, SIDIORA_ROUTER_ABI } from './abis';
import { toLimitOrder, toStopLimitOrder, toStopLossOrder } from './order-records';
import { prepareTokenToTokenSwap, swapTokenToToken } from './sidiora';
import type { SidioraMultihopConfirmation } from './types';

const producerRouterJson = readFileSync(
  resolve(__dirname, '../../../../../../precompiles/launchpad/abi/Router.json'),
  'utf8'
);
const producerRouterAbi: Abi = JSON.parse(producerRouterJson);
const producerRouter = new ethers.Interface(producerRouterJson);
const walletRouter = new ethers.Interface(SIDIORA_ROUTER_ABI);
const walletOrders = new ethers.Interface(PECOR_ORDERS_ABI);
const walletStops = new ethers.Interface(PECOR_STOP_ORDERS_ABI);
const owner = '0x0000000000000000000000000000000000000125';
const signer = new ethers.VoidSigner(owner);
const amountIn = 12345678901234567890n;

function confirmation(): SidioraMultihopConfirmation {
  return {
    minimumAmountOut: 9666172840593617283n,
    deadline: BigInt(Math.floor(Date.now() / 1000) + 617),
  };
}

async function finding_6_sdk_prepares_the_generation_2_role_with_the_confirmed_bound() {
  const confirmed = confirmation();
  const prepared = prepareTokenToTokenSwap(
    signer,
    TOKENS.SID.address,
    TOKENS.USID.address,
    amountIn,
    confirmed
  );
  const tx = await prepared.method.populateTransaction(...prepared.args);
  expect(tx.to).toBe(CONTRACTS.Router);
  expect(tx.data.slice(0, 10)).toBe('0x8ef5184b');
  expect(walletRouter.getFunction('swapTokenToToken')).toBeNull();
  expect(prepared.method.fragment.format('full')).toBe(
    producerRouter.getFunction('swapTokenForToken')!.format('full')
  );
  if (!isHex(tx.data)) throw new Error('SDK calldata is not hex');
  const produced = decodeFunctionData({ abi: producerRouterAbi, data: tx.data });
  expect(produced.functionName).toBe('swapTokenForToken');
  expect(produced.args).toEqual([
    TOKENS.SID.address,
    TOKENS.USID.address,
    amountIn,
    confirmed.minimumAmountOut,
    confirmed.deadline,
  ]);
  expect(tx.data).toBe(
    encodeFunctionData({
      abi: producerRouterAbi,
      functionName: 'swapTokenForToken',
      args: prepared.args,
    })
  );
}

async function finding_6_approval_cannot_replace_the_confirmed_minimum_or_expiry() {
  const confirmed = confirmation();
  const prepared = prepareTokenToTokenSwap(
    signer,
    TOKENS.SID.address,
    TOKENS.USID.address,
    amountIn,
    confirmed
  );
  const before = await prepared.method.populateTransaction(...prepared.args);
  const originalExpiry = confirmed.deadline;
  confirmed.minimumAmountOut = 1n;
  confirmed.deadline += 86400n;
  expect(Reflect.set(prepared.args, '3', 1n)).toBe(false);
  expect(Reflect.set(prepared.args, '4', confirmed.deadline)).toBe(false);
  expect(await prepared.method.populateTransaction(...prepared.args)).toEqual(before);
  expect(prepared.args[3]).toBe(9666172840593617283n);
  expect(prepared.args[4]).toBe(originalExpiry);
  expect(prepared.args[3]).toBeGreaterThan(9666172840593617282n);
}

function finding_6_generation_2_return_codec_preserves_both_amounts() {
  for (const amounts of [
    [0n, 0n],
    [9876543210987654321n, 1234567890123456789n],
    [MAX_UINT256, MAX_UINT256 - 1n],
  ]) {
    const produced = encodeFunctionResult({
      abi: producerRouterAbi,
      functionName: 'swapTokenForToken',
      result: amounts,
    });
    const received = walletRouter.decodeFunctionResult('swapTokenForToken', produced);
    expect(received.amountOut).toBe(amounts[0]);
    expect(received.intermediateUsdl).toBe(amounts[1]);
    const returned = walletRouter.encodeFunctionResult('swapTokenForToken', received);
    if (!isHex(returned)) throw new Error('SDK result is not hex');
    expect(
      decodeFunctionResult({
        abi: producerRouterAbi,
        functionName: 'swapTokenForToken',
        data: returned,
      })
    ).toEqual(amounts);
    expect(() =>
      walletRouter.decodeFunctionResult('swapTokenForToken', produced.slice(0, 66))
    ).toThrow();
  }
}

async function finding_6_sdk_rejects_unprotected_or_expired_confirmations_before_approval() {
  for (const minimumAmountOut of [0n, -1n, MAX_UINT256 + 1n]) {
    await expect(
      swapTokenToToken(TOKENS.SID.address, TOKENS.USID.address, amountIn, signer, 150, 300, {
        ...confirmation(),
        minimumAmountOut,
      })
    ).rejects.toThrow('Sidiora swap requires positive uint256 input and minimum output');
  }
  for (const input of [0n, -1n, MAX_UINT256 + 1n]) {
    await expect(
      swapTokenToToken(
        TOKENS.SID.address,
        TOKENS.USID.address,
        input,
        signer,
        150,
        300,
        confirmation()
      )
    ).rejects.toThrow('Sidiora swap requires positive uint256 input and minimum output');
  }
  for (const deadline of [0n, -1n, BigInt(Math.floor(Date.now() / 1000)) - 1n, MAX_UINT256 + 1n]) {
    await expect(
      swapTokenToToken(TOKENS.SID.address, TOKENS.USID.address, amountIn, signer, 150, 300, {
        ...confirmation(),
        deadline,
      })
    ).rejects.toThrow('Sidiora swap confirmation has expired or has an invalid expiry');
  }
  for (const slippage of [-1, 10000, 10001, 0.5, NaN, Infinity]) {
    await expect(
      swapTokenToToken(
        TOKENS.SID.address,
        TOKENS.USID.address,
        amountIn,
        signer,
        slippage,
        300,
        confirmation()
      )
    ).rejects.toThrow('Invalid Sidiora swap slippage');
  }
  for (const seconds of [0, -1, 0.5, NaN, Infinity, Number.MAX_SAFE_INTEGER + 1]) {
    await expect(
      swapTokenToToken(
        TOKENS.SID.address,
        TOKENS.USID.address,
        amountIn,
        signer,
        150,
        seconds,
        confirmation()
      )
    ).rejects.toThrow('Invalid Sidiora swap deadline');
  }
}

function orderFields() {
  return {
    id: 9007199254740993n,
    user: owner,
    stablecoin: TOKENS.USDL.address,
    token: TOKENS.SID.address,
    amount: MAX_UINT256 - 1n,
    createdAt: 1700000001n,
    expiresAt: 1700000618n,
  } as const;
}

function finding_52_limit_orders_round_trip_every_enum_value_as_numbers() {
  for (const status of [0, 1, 2, 3, 4]) {
    for (const orderType of [0, 1, 2, 3, 4]) {
      const emitted = {
        ...orderFields(),
        targetPrice: 123456789012345678901234567890n,
        orderType,
        status,
      };
      const bytes = encodeFunctionResult({
        abi: producerOrdersAbi,
        functionName: 'getLimitOrder',
        result: emitted,
      });
      const decoded = walletOrders.decodeFunctionResult('getLimitOrder', bytes)[0];
      expect(typeof decoded.orderType).toBe('bigint');
      expect(typeof decoded.status).toBe('bigint');
      const order = toLimitOrder(decoded);
      expect(order).toEqual(emitted);
      expect(typeof order.orderType).toBe('number');
      expect(typeof order.status).toBe('number');
      expect(Object.getPrototypeOf(order)).toBe(Object.prototype);
      expect(walletOrders.encodeFunctionResult('getLimitOrder', [order])).toBe(bytes);
      expect(
        decodeFunctionResult({
          abi: producerOrdersAbi,
          functionName: 'getLimitOrder',
          data: bytes,
        })
      ).toEqual(order);
    }
  }
}

function finding_52_stop_orders_round_trip_every_enum_value_as_numbers() {
  for (const status of [0, 1, 2, 3, 4]) {
    const emittedLoss = { ...orderFields(), triggerPrice: MAX_UINT256, status };
    const lossBytes = encodeFunctionResult({
      abi: producerStopsAbi,
      functionName: 'getStopLossOrder',
      result: emittedLoss,
    });
    const decodedLoss = walletStops.decodeFunctionResult('getStopLossOrder', lossBytes)[0];
    expect(typeof decodedLoss.status).toBe('bigint');
    const loss = toStopLossOrder(decodedLoss);
    expect(loss).toEqual(emittedLoss);
    expect(typeof loss.status).toBe('number');
    expect(Object.getPrototypeOf(loss)).toBe(Object.prototype);
    expect(walletStops.encodeFunctionResult('getStopLossOrder', [loss])).toBe(lossBytes);
    expect(
      decodeFunctionResult({
        abi: producerStopsAbi,
        functionName: 'getStopLossOrder',
        data: lossBytes,
      })
    ).toEqual(loss);

    for (const orderType of [0, 1, 2, 3, 4]) {
      const emittedLimit = {
        ...orderFields(),
        stopPrice: MAX_UINT256 - 2n,
        limitPrice: MAX_UINT256 - 3n,
        orderType,
        status,
      };
      const limitBytes = encodeFunctionResult({
        abi: producerStopsAbi,
        functionName: 'getStopLimitOrder',
        result: emittedLimit,
      });
      const decodedLimit = walletStops.decodeFunctionResult('getStopLimitOrder', limitBytes)[0];
      expect(typeof decodedLimit.orderType).toBe('bigint');
      expect(typeof decodedLimit.status).toBe('bigint');
      const limit = toStopLimitOrder(decodedLimit);
      expect(limit).toEqual(emittedLimit);
      expect(typeof limit.orderType).toBe('number');
      expect(typeof limit.status).toBe('number');
      expect(Object.getPrototypeOf(limit)).toBe(Object.prototype);
      expect(walletStops.encodeFunctionResult('getStopLimitOrder', [limit])).toBe(limitBytes);
      expect(
        decodeFunctionResult({
          abi: producerStopsAbi,
          functionName: 'getStopLimitOrder',
          data: limitBytes,
        })
      ).toEqual(limit);
    }
  }
}

function finding_52_unknown_enum_values_are_rejected_instead_of_published() {
  for (const value of [5, 255]) {
    for (const field of ['orderType', 'status']) {
      const emittedLimit = {
        ...orderFields(),
        targetPrice: 1n,
        orderType: 0,
        status: 0,
        [field]: value,
      };
      const limitBytes = encodeFunctionResult({
        abi: producerOrdersAbi,
        functionName: 'getLimitOrder',
        result: emittedLimit,
      });
      expect(() =>
        toLimitOrder(walletOrders.decodeFunctionResult('getLimitOrder', limitBytes)[0])
      ).toThrow(`Invalid order ${field}: ${value}`);
      const emittedStop = {
        ...orderFields(),
        stopPrice: 1n,
        limitPrice: 1n,
        orderType: 0,
        status: 0,
        [field]: value,
      };
      const stopBytes = encodeFunctionResult({
        abi: producerStopsAbi,
        functionName: 'getStopLimitOrder',
        result: emittedStop,
      });
      expect(() =>
        toStopLimitOrder(walletStops.decodeFunctionResult('getStopLimitOrder', stopBytes)[0])
      ).toThrow(`Invalid order ${field}: ${value}`);
    }
    const lossBytes = encodeFunctionResult({
      abi: producerStopsAbi,
      functionName: 'getStopLossOrder',
      result: { ...orderFields(), triggerPrice: 1n, status: value },
    });
    expect(() =>
      toStopLossOrder(walletStops.decodeFunctionResult('getStopLossOrder', lossBytes)[0])
    ).toThrow(`Invalid order status: ${value}`);
  }
}

test(
  finding_6_sdk_prepares_the_generation_2_role_with_the_confirmed_bound.name,
  finding_6_sdk_prepares_the_generation_2_role_with_the_confirmed_bound
);
test(
  finding_6_approval_cannot_replace_the_confirmed_minimum_or_expiry.name,
  finding_6_approval_cannot_replace_the_confirmed_minimum_or_expiry
);
test(
  finding_6_generation_2_return_codec_preserves_both_amounts.name,
  finding_6_generation_2_return_codec_preserves_both_amounts
);
test(
  finding_6_sdk_rejects_unprotected_or_expired_confirmations_before_approval.name,
  finding_6_sdk_rejects_unprotected_or_expired_confirmations_before_approval
);
test(
  finding_52_limit_orders_round_trip_every_enum_value_as_numbers.name,
  finding_52_limit_orders_round_trip_every_enum_value_as_numbers
);
test(
  finding_52_stop_orders_round_trip_every_enum_value_as_numbers.name,
  finding_52_stop_orders_round_trip_every_enum_value_as_numbers
);
test(
  finding_52_unknown_enum_values_are_rejected_instead_of_published.name,
  finding_52_unknown_enum_values_are_rejected_instead_of_published
);
