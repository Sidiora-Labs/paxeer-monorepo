'use client';

import { useCallback, useEffect, useState } from 'react';
import type { SwapQuote } from '@/lib/swap';
import { swapQuoteInputIdentity, useSwapQuotes, type UseSwapQuotesOptions } from './useSwapQuotes';

export interface SwapApproval {
  identity: string;
  quote: SwapQuote;
  fromAmount: string;
  slippageBps: number;
  recipient: string;
  expiresAt: number;
}

export function useSwapQuoteSelection(options: UseSwapQuotesOptions) {
  const identity = swapQuoteInputIdentity(options);
  const [selected, setSelected] = useState<SwapApproval | null>(null);
  const approval =
    selected?.identity === identity && selected.expiresAt > Date.now() ? selected : null;
  const quotes = useSwapQuotes({ ...options, paused: approval !== null });

  useEffect(() => {
    if (!selected) return;
    if (!approval) {
      setSelected(null);
      return;
    }
    const timer = setTimeout(() => setSelected(null), Math.max(0, selected.expiresAt - Date.now()));
    return () => clearTimeout(timer);
  }, [selected, approval]);

  const closeApproval = useCallback(() => setSelected(null), []);
  const openApproval = useCallback(() => {
    if (
      !quotes.bestQuote ||
      quotes.quoting ||
      !options.recipient ||
      quotes.quoteExpiresAt <= Date.now()
    )
      return;
    const quote = Object.freeze({
      ...quotes.bestQuote,
      tokenIn: Object.freeze({ ...quotes.bestQuote.tokenIn }),
      tokenOut: Object.freeze({ ...quotes.bestQuote.tokenOut }),
    });
    setSelected({
      identity,
      quote,
      fromAmount: options.fromAmount,
      slippageBps: options.slippageBps,
      recipient: options.recipient,
      expiresAt: quotes.quoteExpiresAt,
    });
  }, [identity, quotes, options.fromAmount, options.slippageBps, options.recipient]);

  return {
    quotes: approval
      ? {
          ...quotes,
          quotes: [approval.quote],
          bestQuote: approval.quote,
          quoting: false,
          quoteError: '',
          quoteExpiresAt: approval.expiresAt,
          quoteSecondsLeft: Math.max(0, Math.ceil((approval.expiresAt - Date.now()) / 1000)),
        }
      : quotes,
    approval,
    openApproval,
    closeApproval,
  };
}
