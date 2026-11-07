import { BaseError, ContractFunctionRevertedError, UserRejectedRequestError } from 'viem';

import { expect, test } from 'vitest';

import { layerxBridgeAbi } from './contracts';
import { describeWriteError } from './errors';

function precompileRevert(reason: string): ContractFunctionRevertedError {
  return new ContractFunctionRevertedError({
    abi: layerxBridgeAbi,
    functionName: 'creditDeposit',
    message: reason,
  });
}

function describeWriteErrorMapsThePausedCode() {
  const reason = 'LXP_BRIDGE_PAUSED: bridge is paused';
  const sentence = 'The bridge is paused on this chain. Try again after it reopens.';

  expect(describeWriteError(precompileRevert(reason))).toBe(sentence);
  expect(describeWriteError(new BaseError('Transaction failed', {
    cause: precompileRevert(`execution reverted: ${ reason }`),
  }))).toBe(sentence);
  expect(describeWriteError(new BaseError(
    `The contract reverted with the following reason:\n${ reason }`,
  ))).toBe(sentence);
}

function describeWriteErrorMapsTheNullifiedCode() {
  expect(describeWriteError(precompileRevert(
    'LXP_BRIDGE_NULLIFIED: deposit already credited',
  ))).toBe('This deposit has already been credited.');
}

function describeWriteErrorKeepsAnUnregisteredReason() {
  const reasons = [
    'Unexpected bridge failure',
    'LXP_BRIDGE_FUTURE_CODE: an unregistered refusal',
    'The bridge rejected this request',
  ];

  for (const reason of reasons) {
    expect(describeWriteError(precompileRevert(reason))).toBe(`${ reason }.`);
    expect(describeWriteError(precompileRevert(`execution reverted: ${ reason }  `)))
      .toBe(`${ reason }.`);
    expect(describeWriteError(precompileRevert(`${ reason }.`))).toBe(`${ reason }.`);
  }
}

function describeWriteErrorDetectsAWalletRejectionByType() {
  const rejection = new UserRejectedRequestError(new Error('Request cancelled'));
  const sentence = 'You rejected the request in your wallet.';

  expect(describeWriteError(rejection)).toBe(sentence);
  expect(describeWriteError(new BaseError('Transaction failed', {
    cause: new BaseError('Wallet request failed', { cause: rejection }),
  }))).toBe(sentence);

  for (const message of [ 'The server rejected the request', 'Access denied by the bridge' ]) {
    expect(describeWriteError(new Error(message))).toBe(message);
    expect(describeWriteError(new BaseError(message))).toBe(`${ message }.`);
    expect(describeWriteError(new BaseError('Transaction failed', {
      cause: new Error(message),
    }))).toBe('Transaction failed.');
  }
}

test(describeWriteErrorMapsThePausedCode);
test(describeWriteErrorMapsTheNullifiedCode);
test(describeWriteErrorKeepsAnUnregisteredReason);
test(describeWriteErrorDetectsAWalletRejectionByType);
