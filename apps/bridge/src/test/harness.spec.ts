import { parseAmount } from '@/lib/bridge/amount';

import { expect, test } from 'vitest';

import { parseAmount as parseAmountFromSource } from '../lib/bridge/amount';

function resolvesTheSrcAlias() {
  expect(parseAmount).toBe(parseAmountFromSource);
  expect(parseAmount('1.25', 6)).toBe(BigInt('1250000'));
}

function defaultsToTheNodeEnvironment() {
  expect(process.release.name).toBe('node');
  expect(typeof window).toBe('undefined');
  expect(typeof document).toBe('undefined');
}

test(resolvesTheSrcAlias);
test(defaultsToTheNodeEnvironment);
