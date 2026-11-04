/**
 * Pure helpers for the swap widget — no React, no API calls.
 *
 * - {@link paxscanToSwapToken} adapts paxscan token records to the SDK shape.
 * - {@link parseAmountToWei} validates and converts a decimal string to raw wei.
 * - {@link formatBalanceCompact} renders a balance with K/M abbreviations for
 *   header chips.
 * - {@link formatBalanceFull} renders a balance without abbreviations for
 *   percentage / max-fill calculations.
 * - {@link minReceived} computes the minimum-output amount given a quote and
 *   slippage tolerance.
 */

import type { PaxscanToken } from '@/lib/api';
import type { SwapToken, SwapQuote } from '@/lib/swap';

const KNOWN_STABLECOIN_SYMBOLS = new Set(['USDC', 'USDT', 'USDL', 'USID']);

// Ethers amount codecs support integer precision from 0 through 80.
export function supportedTokenDecimals(value: unknown): number | null {
  if (typeof value !== 'number' && (typeof value !== 'string' || !/^\d+$/.test(value))) {
    return null;
  }
  const decimals = Number(value);
  return Number.isInteger(decimals) && decimals >= 0 && decimals <= 80 ? decimals : null;
}

function requireTokenDecimals(decimals: number): void {
  if (supportedTokenDecimals(decimals) === null)
    throw new Error('Unknown or unsupported token precision');
}

export function paxscanToSwapToken(t: PaxscanToken): SwapToken | null {
  const decimals = supportedTokenDecimals(t.decimals);
  if (decimals === null) return null;
  const token: SwapToken = {
    symbol: t.symbol,
    name: t.name,
    address: t.address_hash.toLowerCase(),
    decimals,
    iconUrl: t.icon_url || undefined,
  };
  if (KNOWN_STABLECOIN_SYMBOLS.has(t.symbol)) {
    token.isStablecoin = true;
  }
  return token;
}

export function parseAmountToWei(amount: string, decimals: number): string | null {
  if (
    supportedTokenDecimals(decimals) === null ||
    !/^(?:\d+|\d*\.\d+)$/.test(amount) ||
    Number(amount) <= 0
  )
    return null;
  try {
    const parts = amount.split('.');
    const whole = parts[0] || '0';
    let frac = parts[1] || '';
    if (frac.length > decimals && /[1-9]/.test(frac.slice(decimals))) return null;
    frac = frac.slice(0, decimals);
    frac = frac.padEnd(decimals, '0');
    return BigInt(whole + frac).toString();
  } catch {
    return null;
  }
}

export function formatBalanceCompact(raw: bigint, decimals: number): string {
  requireTokenDecimals(decimals);
  const divisor = BigInt(10) ** BigInt(decimals);
  const whole = raw / divisor;
  const frac = (raw % divisor).toString().padStart(decimals, '0').slice(0, 4);
  const trimmed = frac.replace(/0+$/, '') || '0';
  if (whole >= BigInt(1_000_000)) return `${(Number(whole) / 1_000_000).toFixed(2)}M`;
  if (whole >= BigInt(1_000)) return `${(Number(whole) / 1_000).toFixed(2)}K`;
  return decimals === 0 ? `${whole}` : `${whole}.${trimmed}`;
}

export function formatBalanceFull(raw: bigint, decimals: number): string {
  requireTokenDecimals(decimals);
  const divisor = BigInt(10) ** BigInt(decimals);
  const whole = raw / divisor;
  const frac = (raw % divisor).toString().padStart(decimals, '0');
  const trimmed = frac.replace(/0+$/, '');
  return trimmed ? `${whole}.${trimmed}` : `${whole}`;
}

export function minReceived(quote: SwapQuote, slippageBps: number, decimals: number): string {
  requireTokenDecimals(decimals);
  const out = BigInt(quote.amountOut);
  const min = out - (out * BigInt(slippageBps)) / BigInt(10000);
  const divisor = BigInt(10) ** BigInt(decimals);
  const whole = min / divisor;
  const frac = (min % divisor).toString().padStart(decimals, '0').slice(0, 4);
  return decimals === 0 ? `${whole}` : `${whole}.${frac}`;
}
