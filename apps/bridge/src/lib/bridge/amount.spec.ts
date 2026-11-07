import { expect, test } from 'vitest';

import { parseAmount } from './amount';

function parseAmountRefusesACommaDecimalSeparator() {
  for (const input of [ '1,5', '12,34', '1234,567', '1,,234', ',123', '123,', '1,234,56', '1.2,345', '1,234.5,6' ]) {
    expect(parseAmount(input, 18), input).toBeUndefined();
  }
}

function parseAmountAcceptsGroupedThousands() {
  expect(parseAmount('1,234.5', 6)).toBe(BigInt('1234500000'));
  expect(parseAmount('12,345,678.9', 6)).toBe(BigInt('12345678900000'));
  expect(parseAmount(' 123,456 ', 0)).toBe(BigInt('123456'));
  expect(parseAmount('1234.5', 6)).toBe(BigInt('1234500000'));
  expect(parseAmount('1,234.567', 2)).toBeUndefined();
}

test(parseAmountRefusesACommaDecimalSeparator);
test(parseAmountAcceptsGroupedThousands);
