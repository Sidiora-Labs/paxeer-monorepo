import { expect, test } from 'vitest';
import type { Locale } from './index';
import { formatHumanTokenAmount, formatRawTokenAmount, formatTokenAmount } from './formatters';
import './locale.test';

function finding23_explicit_raw_and_human_units() {
  expect(formatRawTokenAmount('1', 18, 'en')).toBe('0');
  expect(formatRawTokenAmount('1000000000000000000', 18, 'en', 'PAX')).toBe('1\u00a0PAX');
  expect(formatHumanTokenAmount('1', 18, 'en', 'PAX')).toBe('1\u00a0PAX');
  expect(formatHumanTokenAmount('1.0', 18, 'en', 'PAX')).toBe('1\u00a0PAX');
  expect(formatHumanTokenAmount('0001.000', 18, 'en')).toBe('1');
  expect(formatHumanTokenAmount('.5', 18, 'en')).toBe('0.5');
  expect(formatRawTokenAmount('000500000000000000000', 18, 'en')).toBe('0.5');
  expect(() => formatRawTokenAmount('1.0', 18, 'en')).toThrow(TypeError);
}

function finding23_legacy_provider_entry_uses_consistent_human_units() {
  expect(formatTokenAmount('1', 18, 'en')).toBe('1');
  expect(formatTokenAmount('1.0', 18, 'en')).toBe('1');
  expect(formatTokenAmount(1, 18, 'en')).toBe('1');
  expect(formatTokenAmount('1000000000000000000', 18, 'en')).toBe('1,000,000,000,000,000,000');
  expect(formatTokenAmount('9007199254740993.123456', 18, 'en')).toBe(
    '9,007,199,254,740,993.123456'
  );
  expect(formatTokenAmount(0.0000005, 18, 'en')).toBe('0.000001');
}

function finding22_exact_large_integer_and_fraction() {
  expect(formatRawTokenAmount('9007199254740993', 0, 'en')).toBe('9,007,199,254,740,993');
  expect(formatRawTokenAmount('9007199254740993123456', 6, 'en')).toBe(
    '9,007,199,254,740,993.123456'
  );
  expect(formatHumanTokenAmount('9007199254740993.123456', 18, 'en')).toBe(
    '9,007,199,254,740,993.123456'
  );
  expect(formatHumanTokenAmount('9007199254740993.9999995', 18, 'en')).toBe(
    '9,007,199,254,740,994'
  );
  expect(formatRawTokenAmount('9007199254740993999999500000000000', 18, 'en')).toBe(
    '9,007,199,254,740,994'
  );
}

function finding22_rounds_once_and_carries_across_the_whole_amount() {
  expect(formatRawTokenAmount('1999999400000000000', 18, 'en')).toBe('1.999999');
  expect(formatRawTokenAmount('1999999500000000000', 18, 'en')).toBe('2');
  expect(formatHumanTokenAmount('1.9999995', 18, 'en')).toBe('2');
  expect(formatRawTokenAmount('999999950000000000', 18, 'en')).toBe('1');
  expect(formatHumanTokenAmount('999.9999995', 18, 'en')).toBe('1,000');
  expect(formatHumanTokenAmount('-1.9999995', 18, 'en')).toBe('-2');
  expect(formatRawTokenAmount('-1999999500000000000', 18, 'en')).toBe('-2');
  expect(formatHumanTokenAmount('0.004999999999999999', 2, 'en')).toBe('0');
  expect(formatHumanTokenAmount('0.005', 2, 'en')).toBe('0.01');
  expect(formatHumanTokenAmount('9.995', 2, 'en')).toBe('10');
  expect(formatHumanTokenAmount('0.000000499999999999', 18, 'en')).toBe('0');
  expect(formatHumanTokenAmount('0.0000005', 18, 'en')).toBe('0.000001');
}

function finding22_arabic_digits_separators_and_signs() {
  expect(formatRawTokenAmount('1234500000000000000000', 18, 'ar', 'PAX')).toBe('١٬٢٣٤٫٥\u00a0PAX');
  expect(formatHumanTokenAmount('1234.5', 18, 'ar')).toBe('١٬٢٣٤٫٥');
  expect(formatRawTokenAmount('500000000000000000', 18, 'ar')).toBe('٠٫٥');
  expect(formatRawTokenAmount('999999950000000000', 18, 'ar')).toBe('١');
  expect(formatHumanTokenAmount('-0.5', 18, 'ar')).toBe('\u061c-٠٫٥');
  expect(formatHumanTokenAmount('9007199254740993.123456', 18, 'ar')).toBe(
    '٩٬٠٠٧٬١٩٩٬٢٥٤٬٧٤٠٬٩٩٣٫١٢٣٤٥٦'
  );
  expect(formatRawTokenAmount('0', 0, 'ar', 'PAX')).toBe('٠\u00a0PAX');
}

function finding24_zero_decimals_and_exact_locale_boundaries() {
  expect(formatRawTokenAmount('1234', 0, 'en', 'PAX')).toBe('1,234\u00a0PAX');
  expect(formatHumanTokenAmount('1234.49', 0, 'en')).toBe('1,234');
  expect(formatHumanTokenAmount('1234.5', 0, 'en')).toBe('1,235');
  expect(formatHumanTokenAmount('-1234.5', 0, 'en')).toBe('-1,235');
  expect(formatHumanTokenAmount('0', 0, 'en')).toBe('0');
  expect(formatRawTokenAmount('0', 18, 'en')).toBe('0');
  const examples: readonly [Locale, string][] = [
    ['en', '1,234,567.123457'],
    ['es', '1.234.567,123457'],
    ['fr', '1\u202f234\u202f567,123457'],
    ['de', '1.234.567,123457'],
    ['pt', '1.234.567,123457'],
    ['ar', '١٬٢٣٤٬٥٦٧٫١٢٣٤٥٧'],
    ['it', '1.234.567,123457'],
    ['hi', '12,34,567.123457'],
    ['ja', '1,234,567.123457'],
    ['ko', '1,234,567.123457'],
  ];
  for (const [locale, expected] of examples) {
    expect(formatHumanTokenAmount('1234567.1234567', 18, locale)).toBe(expected);
    expect(formatRawTokenAmount('1234567123456700000000000', 18, locale)).toBe(expected);
  }
}

function finding23_rejects_invalid_explicit_amounts_and_decimals() {
  for (const invalid of ['not-a-number', '1junk', 'Infinity', 'NaN', '', '1,000']) {
    expect(() => formatHumanTokenAmount(invalid, 18, 'en')).toThrow(TypeError);
    expect(() => formatRawTokenAmount(invalid, 18, 'en')).toThrow(TypeError);
  }
  for (const decimals of [-1, 0.5, NaN, Infinity]) {
    expect(() => formatHumanTokenAmount('1', decimals, 'en')).toThrow(RangeError);
    expect(() => formatRawTokenAmount('1', decimals, 'en')).toThrow(RangeError);
  }
}

test(finding23_explicit_raw_and_human_units.name, finding23_explicit_raw_and_human_units);
test(
  finding23_legacy_provider_entry_uses_consistent_human_units.name,
  finding23_legacy_provider_entry_uses_consistent_human_units
);
test(finding22_exact_large_integer_and_fraction.name, finding22_exact_large_integer_and_fraction);
test(
  finding22_rounds_once_and_carries_across_the_whole_amount.name,
  finding22_rounds_once_and_carries_across_the_whole_amount
);
test(
  finding22_arabic_digits_separators_and_signs.name,
  finding22_arabic_digits_separators_and_signs
);
test(
  finding24_zero_decimals_and_exact_locale_boundaries.name,
  finding24_zero_decimals_and_exact_locale_boundaries
);
test(
  finding23_rejects_invalid_explicit_amounts_and_decimals.name,
  finding23_rejects_invalid_explicit_amounts_and_decimals
);
