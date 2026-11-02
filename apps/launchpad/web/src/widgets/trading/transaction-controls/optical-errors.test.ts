import { encodeErrorResult, parseAbi } from 'viem';
import { expect, test } from 'vitest';
import { OpticalRejection, opticalRejection } from './providers';

function unexecutableQuoteMapsToOpticalRejection() {
  const data = encodeErrorResult({
    abi: parseAbi(['error UnexecutableQuote()']),
    errorName: 'UnexecutableQuote',
  });
  const error = new Error('Quote request failed', {
    cause: new Error('Contract call reverted', { cause: { data } }),
  });

  const rejection = opticalRejection(error);

  expect(rejection).toBeInstanceOf(OpticalRejection);
  expect(rejection).toMatchObject({
    reason: 'UnexecutableQuote',
    retryEligibility: 'request a new quote',
  });
}

test(unexecutableQuoteMapsToOpticalRejection);
