'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { getSwapQuotes, getBestQuote, type SwapToken, type SwapQuote } from '@/lib/swap';
import { parseAmountToWei, supportedTokenDecimals } from './util';

const QUOTE_TTL_MS = 30_000;

export interface UseSwapQuotesResult {
  quotes: SwapQuote[];
  bestQuote: SwapQuote | null;
  quoting: boolean;
  quoteError: string;
  quoteSecondsLeft: number;
  quoteExpiresAt: number;
  clear: () => void;
  refresh: () => void;
}

export interface UseSwapQuotesOptions {
  fromToken: SwapToken;
  toToken: SwapToken;
  fromAmount: string;
  slippageBps: number;
  recipient: string | undefined;
  debounceMs?: number;
  paused?: boolean;
}

export function swapQuoteInputIdentity(options: UseSwapQuotesOptions): string {
  return JSON.stringify([
    options.fromToken,
    options.toToken,
    options.fromAmount,
    options.slippageBps,
    options.recipient,
  ]);
}

interface QuoteState {
  identity: string;
  quotes: SwapQuote[];
  bestQuote: SwapQuote | null;
  quoting: boolean;
  quoteError: string;
  quoteExpiresAt: number;
}

function emptyQuoteState(identity: string): QuoteState {
  return {
    identity,
    quotes: [],
    bestQuote: null,
    quoting: false,
    quoteError: '',
    quoteExpiresAt: 0,
  };
}

export function useSwapQuotes(options: UseSwapQuotesOptions): UseSwapQuotesResult {
  const {
    fromToken,
    toToken,
    fromAmount,
    slippageBps,
    recipient,
    debounceMs = 600,
    paused = false,
  } = options;
  const identity = swapQuoteInputIdentity(options);
  const currentInput = useRef({ identity, paused });
  currentInput.current = { identity, paused };
  const [state, setState] = useState<QuoteState>(() => emptyQuoteState(identity));
  const [now, setNow] = useState(Date.now);
  const debounceRef = useRef<ReturnType<typeof setTimeout>>();
  const requestSeqRef = useRef(0);
  const expiryRef = useRef(0);
  const fetchQuotesRef = useRef<(() => void) | null>(null);

  const clear = useCallback(() => {
    ++requestSeqRef.current;
    if (debounceRef.current) clearTimeout(debounceRef.current);
    expiryRef.current = 0;
    setState(emptyQuoteState(currentInput.current.identity));
  }, []);

  const doFetch = useCallback(
    async (seq: number) => {
      const isCurrent = () =>
        seq === requestSeqRef.current &&
        identity === currentInput.current.identity &&
        !currentInput.current.paused;
      if (!isCurrent()) return;
      const wei = parseAmountToWei(fromAmount, fromToken.decimals);
      if (!wei || !recipient || supportedTokenDecimals(toToken.decimals) === null) {
        if (isCurrent()) setState(emptyQuoteState(identity));
        return;
      }
      try {
        const quotes = await getSwapQuotes({
          tokenIn: fromToken,
          tokenOut: toToken,
          amountIn: wei,
          slippageBps,
          recipient,
        });
        if (!isCurrent()) return;
        const quoteExpiresAt = quotes.length ? Date.now() + QUOTE_TTL_MS : 0;
        expiryRef.current = quoteExpiresAt;
        setNow(Date.now());
        setState({
          identity,
          quotes,
          bestQuote: getBestQuote(quotes),
          quoting: false,
          quoteError: quotes.length ? '' : 'No routes found for this pair',
          quoteExpiresAt,
        });
      } catch (error) {
        if (!isCurrent()) return;
        expiryRef.current = 0;
        setState({
          ...emptyQuoteState(identity),
          quoteError: error instanceof Error ? error.message : 'Quote failed',
        });
      }
    },
    [identity, fromAmount, fromToken, toToken, slippageBps, recipient]
  );

  useEffect(() => {
    const start = (delay: number) => {
      const seq = ++requestSeqRef.current;
      if (debounceRef.current) clearTimeout(debounceRef.current);
      expiryRef.current = 0;
      setState(emptyQuoteState(identity));
      if (
        paused ||
        !recipient ||
        !parseAmountToWei(fromAmount, fromToken.decimals) ||
        supportedTokenDecimals(toToken.decimals) === null
      )
        return;
      setState({ ...emptyQuoteState(identity), quoting: true });
      if (delay === 0) void doFetch(seq);
      else debounceRef.current = setTimeout(() => void doFetch(seq), delay);
    };
    fetchQuotesRef.current = () => start(0);
    start(debounceMs);
    return () => {
      ++requestSeqRef.current;
      if (debounceRef.current) clearTimeout(debounceRef.current);
      expiryRef.current = 0;
      fetchQuotesRef.current = null;
    };
  }, [
    identity,
    paused,
    debounceMs,
    doFetch,
    recipient,
    fromAmount,
    fromToken.decimals,
    toToken.decimals,
  ]);

  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now());
      if (expiryRef.current && Date.now() >= expiryRef.current) {
        expiryRef.current = 0;
        fetchQuotesRef.current?.();
      }
    }, 1_000);
    return () => clearInterval(timer);
  }, []);

  const refresh = useCallback(() => fetchQuotesRef.current?.(), []);
  const visible = state.identity === identity && !paused ? state : emptyQuoteState(identity);
  const quoteSecondsLeft = Math.max(0, Math.ceil((visible.quoteExpiresAt - now) / 1000));
  return { ...visible, quoteSecondsLeft, clear, refresh };
}
