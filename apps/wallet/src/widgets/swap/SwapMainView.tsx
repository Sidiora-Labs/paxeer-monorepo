'use client';

/**
 * Main swap UI — pay/receive cards with token selectors, flip button,
 * percentage shortcuts, and the best-route summary card.
 *
 * Stateless. The orchestrator owns the form state and passes everything in.
 */

import { Loader2 } from 'lucide-react';
import { SvgIcon } from '@/components/ui/SvgIcon';
import type { SwapToken, SwapQuote } from '@/lib/swap';
import { TokenBadge } from './TokenBadge';
import { SwapErrorCard } from './SwapErrorCard';
import { RouteInfoCard } from './RouteInfoCard';

const PERCENTAGES = [25, 50, 75, 100] as const;

export interface SwapMainViewProps {
  fromToken: SwapToken;
  toToken: SwapToken;
  fromAmount: string;
  fromBalance: string | null;
  toBalance: string | null;
  fromBalanceRaw: bigint | null;
  slippageBps: number;
  bestQuote: SwapQuote | null;
  totalQuotes: number;
  quoting: boolean;
  quoteError: string;
  onChangeAmount: (amount: string) => void;
  onApplyPercentage: (pct: number) => void;
  onFlip: () => void;
  onSelectFrom: () => void;
  onSelectTo: () => void;
  onOpenSettings: () => void;
  onSubmit: () => void;
}

export function SwapMainView({
  fromToken,
  toToken,
  fromAmount,
  fromBalance,
  toBalance,
  fromBalanceRaw,
  slippageBps,
  bestQuote,
  totalQuotes,
  quoting,
  quoteError,
  onChangeAmount,
  onApplyPercentage,
  onFlip,
  onSelectFrom,
  onSelectTo,
  onOpenSettings,
  onSubmit,
}: SwapMainViewProps) {
  const submitDisabled = !bestQuote || quoting || !fromAmount;
  const submitLabel = quoting
    ? 'Finding best route...'
    : bestQuote
      ? 'Review Swap'
      : 'Enter an amount';

  const percentageDisabled = !fromBalanceRaw || fromBalanceRaw === BigInt(0);

  return (
    <div className="px-4 pt-4 pb-24">
      <div className="flex items-center justify-end mb-4">
        <button
          type="button"
          aria-label="Swap settings"
          onClick={onOpenSettings}
          className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] p-2 rounded-full bg-pax-control press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="settings" className="w-4 h-4" />
        </button>
      </div>

      <div className="relative">
        {/* From */}
        <div className="glass-card p-4 mb-1">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-pax-muted">You pay</span>
            <div className="flex items-center gap-2">
              {fromBalance !== null && (
                <span className="text-xs text-pax-muted">Balance: {fromBalance}</span>
              )}
              <span className="text-xs text-pax-muted/50">|</span>
              <span className="text-xs text-pax-muted">{slippageBps / 100}% slip</span>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <label htmlFor="swap-from-amount" className="sr-only">
              Amount to pay
            </label>
            <input
              id="swap-from-amount"
              type="text"
              inputMode="decimal"
              name="swap-from-amount"
              autoComplete="off"
              aria-label="Amount to pay"
              value={fromAmount}
              onChange={(e) => onChangeAmount(e.target.value)}
              placeholder="0.00"
              className="min-h-[var(--touch-target)] flex-1 bg-transparent text-2xl font-bold outline-none min-w-0 placeholder:text-pax-muted focus-visible:ring-2 focus-visible:ring-pax-accent"
            />
            <button
              type="button"
              aria-label={`Select ${fromToken.symbol}`}
              onClick={onSelectFrom}
              className="min-h-[var(--touch-target)] flex items-center gap-2 px-3 py-1.5 rounded-full bg-pax-overlay text-sm font-medium press-scale shrink-0 focus-visible:ring-2 focus-visible:ring-pax-accent"
            >
              <TokenBadge token={fromToken} size="sm" />
              {fromToken.symbol}
              <SvgIcon name="chevron-down" className="w-3.5 h-3.5" />
            </button>
          </div>
          {/* Percentage quick-select */}
          <div className="flex gap-2 mt-3">
            {PERCENTAGES.map((pct) => (
              <button
                type="button"
                key={pct}
                onClick={() => onApplyPercentage(pct)}
                disabled={percentageDisabled}
                className="min-h-[var(--touch-target)] flex-1 py-1.5 rounded-lg bg-pax-control text-xs font-medium text-pax-muted hover:bg-pax-overlay hover:text-pax-light transition-all press-scale focus-visible:ring-2 focus-visible:ring-pax-accent disabled:opacity-30 disabled:cursor-not-allowed"
              >
                {pct === 100 ? 'Max' : `${pct}%`}
              </button>
            ))}
          </div>
        </div>

        {/* Flip */}
        <div
          className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2 z-10"
          style={{ top: 'calc(50% + 2px)' }}
        >
          <button
            type="button"
            aria-label="Swap token direction"
            onClick={onFlip}
            className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] w-10 h-10 rounded-full bg-pax-card flex items-center justify-center press-scale hover:bg-pax-overlay transition-all focus-visible:ring-2 focus-visible:ring-pax-accent"
          >
            <SvgIcon name="swap" className="w-4 h-4" />
          </button>
        </div>

        {/* To */}
        <div className="glass-card p-4 mt-1">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-pax-muted">You receive</span>
            {toBalance !== null && (
              <span className="text-xs text-pax-muted">Balance: {toBalance}</span>
            )}
          </div>
          <div className="flex items-center gap-3">
            <div className="flex-1 text-2xl font-bold min-w-0">
              {quoting ? (
                <Loader2 className="w-5 h-5 animate-spin text-pax-muted" />
              ) : bestQuote ? (
                <span>{bestQuote.amountOutDisplay}</span>
              ) : (
                <span className="text-pax-muted">—</span>
              )}
            </div>
            <button
              type="button"
              aria-label={`Select ${toToken.symbol}`}
              onClick={onSelectTo}
              className="min-h-[var(--touch-target)] flex items-center gap-2 px-3 py-1.5 rounded-full bg-pax-overlay text-sm font-medium press-scale shrink-0 focus-visible:ring-2 focus-visible:ring-pax-accent"
            >
              <TokenBadge token={toToken} size="sm" />
              {toToken.symbol}
              <SvgIcon name="chevron-down" className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      {bestQuote && <RouteInfoCard bestQuote={bestQuote} totalRoutes={totalQuotes} />}

      {quoteError && !quoting && <SwapErrorCard error={quoteError} />}

      {!bestQuote && !quoteError && !quoting && (
        <div className="flex items-center gap-2 mt-4 px-1">
          <SvgIcon name="info" className="w-3.5 h-3.5" />
          <p className="text-xs text-pax-muted">
            Routes across PAX DEX, Stable Pool, V2 &amp; V3 AMMs, and Launchpad HLPMMv2
          </p>
        </div>
      )}

      <button
        type="button"
        disabled={submitDisabled}
        onClick={onSubmit}
        className="min-h-[var(--touch-target)] w-full mt-6 py-3 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale disabled:opacity-30 transition-all focus-visible:ring-2 focus-visible:ring-pax-accent"
      >
        {submitLabel}
      </button>
    </div>
  );
}
