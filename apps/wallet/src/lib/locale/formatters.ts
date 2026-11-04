import type { Locale } from './index';

function localeTag(locale: Locale): string {
  const map: Record<Locale, string> = {
    en: 'en-US',
    es: 'es-ES',
    fr: 'fr-FR',
    de: 'de-DE',
    pt: 'pt-BR',
    ar: 'ar-SA',
    it: 'it-IT',
    hi: 'hi-IN',
    ja: 'ja-JP',
    ko: 'ko-KR',
  };
  return map[locale] ?? 'en-US';
}

export function formatNumber(
  value: number,
  locale: Locale,
  options?: Intl.NumberFormatOptions
): string {
  return new Intl.NumberFormat(localeTag(locale), options).format(value);
}

export function formatCurrency(value: number, locale: Locale, currency: string = 'USD'): string {
  return new Intl.NumberFormat(localeTag(locale), {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(value);
}

interface ExactTokenAmount {
  coefficient: bigint;
  scale: number;
  negative: boolean;
}

function decimalTokenAmount(value: string): ExactTokenAmount | null {
  const match = /^([+-]?)(?:(\d+)(?:\.(\d*))?|\.(\d+))(?:[eE]([+-]?\d+))?$/.exec(value.trim());
  if (!match) return null;
  const fraction = match[3] ?? match[4] ?? '';
  const exponent = Number(match[5] ?? '0');
  const scale = fraction.length - exponent;
  if (!Number.isSafeInteger(scale)) throw new RangeError('Invalid decimal scale');
  return {
    coefficient: BigInt(`${match[2] ?? '0'}${fraction}`),
    scale,
    negative: match[1] === '-',
  };
}

function tokenPrecision(decimals: number): number {
  if (!Number.isSafeInteger(decimals) || decimals < 0) {
    throw new RangeError('Token decimals must be a non-negative safe integer');
  }
  return Math.min(decimals, 6);
}

function formatExactTokenAmount(
  amount: ExactTokenAmount,
  decimals: number,
  locale: Locale,
  symbol?: string
): string {
  const precision = tokenPrecision(decimals);
  const shift = amount.scale - precision;
  const divisor = 10n ** BigInt(Math.abs(shift));
  const rounded =
    shift > 0 ? (amount.coefficient + divisor / 2n) / divisor : amount.coefficient * divisor;
  const unit = 10n ** BigInt(precision);
  const whole = rounded / unit;
  const fraction =
    precision === 0 ? '' : (rounded % unit).toString().padStart(precision, '0').replace(/0+$/, '');
  const formatter = new Intl.NumberFormat(localeTag(locale), {
    minimumFractionDigits: fraction.length,
    maximumFractionDigits: fraction.length,
  });
  const digits = new Intl.NumberFormat(localeTag(locale), { useGrouping: false });
  const localizedFraction = fraction.replace(/\d/g, (digit) => digits.format(Number(digit)));
  const signedWhole = amount.negative ? (whole === 0n ? -0 : -whole) : whole;
  const formatted = formatter
    .formatToParts(signedWhole)
    .map((part) => (part.type === 'fraction' ? localizedFraction : part.value))
    .join('');
  return symbol ? `${formatted}\u00A0${symbol}` : formatted;
}

export function formatRawTokenAmount(
  rawUnits: string,
  decimals: number,
  locale: Locale,
  symbol?: string
): string {
  tokenPrecision(decimals);
  const trimmed = rawUnits.trim();
  if (!/^[+-]?\d+$/.test(trimmed)) throw new TypeError('Raw token units must be an integer string');
  return formatExactTokenAmount(
    {
      coefficient: BigInt(trimmed.replace(/^[+-]/, '')),
      scale: decimals,
      negative: trimmed.startsWith('-'),
    },
    decimals,
    locale,
    symbol
  );
}

export function formatHumanTokenAmount(
  humanAmount: string,
  decimals: number,
  locale: Locale,
  symbol?: string
): string {
  const amount = decimalTokenAmount(humanAmount);
  if (!amount) throw new TypeError('Human token amounts must be decimal strings');
  return formatExactTokenAmount(amount, decimals, locale, symbol);
}

/** @deprecated Use formatHumanTokenAmount with an exact decimal string, or formatRawTokenAmount. */
export function formatTokenAmount(
  value: string | number,
  decimals: number,
  locale: Locale,
  symbol?: string
): string {
  const amount = decimalTokenAmount(String(value));
  if (!amount) return symbol ? `0 ${symbol}` : '0';
  return formatExactTokenAmount(amount, decimals, locale, symbol);
}

export function formatCompactNumber(value: number, locale: Locale): string {
  return new Intl.NumberFormat(localeTag(locale), {
    notation: 'compact',
    compactDisplay: 'short',
    maximumFractionDigits: 2,
  }).format(value);
}

export function formatDate(
  value: number | Date,
  locale: Locale,
  options?: Intl.DateTimeFormatOptions
): string {
  const date = typeof value === 'number' ? new Date(value) : value;
  return new Intl.DateTimeFormat(localeTag(locale), {
    dateStyle: 'medium',
    ...options,
  }).format(date);
}

export function formatTime(value: number | Date, locale: Locale): string {
  const date = typeof value === 'number' ? new Date(value) : value;
  return new Intl.DateTimeFormat(localeTag(locale), {
    timeStyle: 'short',
  }).format(date);
}

export function formatRelativeTime(
  value: number,
  locale: Locale,
  unit: Intl.RelativeTimeFormatUnit = 'second'
): string {
  return new Intl.RelativeTimeFormat(localeTag(locale), {
    numeric: 'auto',
  }).format(value, unit);
}

export function formatPercent(value: number, locale: Locale, fractionDigits: number = 2): string {
  return new Intl.NumberFormat(localeTag(locale), {
    style: 'percent',
    minimumFractionDigits: 0,
    maximumFractionDigits: fractionDigits,
  }).format(value / 100);
}

export function plural(
  count: number,
  locale: Locale,
  forms: { zero?: string; one: string; two?: string; few?: string; many?: string; other: string }
): string {
  const pr = new Intl.PluralRules(localeTag(locale));
  const rule = pr.select(count);
  return forms[rule] ?? forms.other;
}
