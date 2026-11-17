'use client';

import { ArrowDownUp, ChevronDown, Equal, Loader2, X } from 'lucide-react';
import NumberFlow from '@number-flow/react';
import React, { useMemo, useState } from 'react';
import Image from 'next/image';
import { ModalPanel } from '@/components/ui/primitives';
import { useWalletState } from '@/providers/WalletProvider';
import { usePaxPriceQuery } from '@/lib/queries';
import { type SwapToken } from '@/lib/swap';
import { useHeldTokens } from './useHeldTokens';
import { useOutputTokenCatalogue } from './useOutputTokens';
import { useSwapBalances } from './useSwapBalances';
import type { UseSwapQuotesResult } from './useSwapQuotes';
import { formatBalanceFull, parseAmountToWei } from './util';
import { formatUsd } from '@/lib/format';

function TokenIcon({ token }: { token: SwapToken }) {
  return (
    <div className="relative size-8 rounded-full bg-pax-control flex items-center justify-center overflow-hidden shrink-0">
      <Image
        src={token.iconUrl || '/wallet/default_icon.webp'}
        alt={token.symbol}
        className="w-full h-full object-cover rounded-full"
        fill
        sizes="32px"
      />
    </div>
  );
}

function TokenSelectorSheet({
  title,
  tokens,
  selected,
  onSelect,
  onClose,
  enableSearch = false,
  loading = false,
  error,
}: {
  title: string;
  tokens: SwapToken[];
  selected: SwapToken;
  onSelect: (t: SwapToken) => void;
  onClose: () => void;
  enableSearch?: boolean;
  loading?: boolean;
  error?: Error | null;
}) {
  const [query, setQuery] = useState('');
  const list = useMemo(() => {
    const f = !query.trim()
      ? tokens
      : tokens.filter(
          (t) =>
            t.symbol.toLowerCase().includes(query.toLowerCase()) ||
            t.name.toLowerCase().includes(query.toLowerCase())
        );
    return f.filter((t) => !(t.symbol === selected.symbol && t.address === selected.address));
  }, [tokens, query, selected]);
  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center">
      <div className="absolute inset-0 bg-pax-scrim backdrop-blur-sm" onClick={onClose} />
      <ModalPanel
        onDismiss={onClose}
        aria-label={title}
        className="pax-token-sheet relative w-full max-w-[430px] bg-pax-surface rounded-t-3xl flex flex-col"
        style={{ maxHeight: 'var(--size-modal-max-height)' }}
      >
        <div className="flex items-center justify-between gap-3 px-5 pt-5 pb-3  ">
          <span className="text-sm font-semibold">{title}</span>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close token selector"
            className="min-h-[var(--touch-target)] p-1.5 rounded-full bg-pax-control hover:bg-pax-overlay"
          >
            <X className="size-4 opacity-60" />
          </button>
        </div>
        {enableSearch && (
          <div className="px-4 pt-3">
            <input
              id="pax-swap-token-search"
              type="text"
              name="token-search"
              autoComplete="off"
              aria-label="Search tokens"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search..."
              autoFocus
              className="w-full bg-pax-control rounded-xl px-3 py-2.5 text-sm outline-none placeholder:opacity-40"
            />
          </div>
        )}
        <div className="min-h-0 flex-1 overflow-y-auto p-3 space-y-1">
          {loading && (
            <p role="status" className="py-8 text-center text-pax-muted">
              Loading tokens…
            </p>
          )}
          {error && (
            <p role="alert" className="py-8 text-center text-pax-error">
              {error.message}
            </p>
          )}
          {!loading && !error && list.length === 0 && (
            <p className="text-center text-sm text-pax-muted py-8">No tokens found</p>
          )}
          {!loading &&
            !error &&
            list.map((t) => (
              <button
                type="button"
                key={t.address || 'native'}
                onClick={() => onSelect(t)}
                className="w-full flex items-center gap-3 px-3 py-3 rounded-xl bg-pax-card hover:bg-pax-control transition-colors"
              >
                <TokenIcon token={t} />
                <div className="flex-1 text-left min-w-0">
                  <p className="text-sm font-semibold">{t.symbol}</p>
                  <p className="text-xs text-pax-muted truncate">{t.name}</p>
                </div>
                {t.isStablecoin && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-pax-info-muted text-pax-info">
                    Stable
                  </span>
                )}
                {t.isNative && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-pax-accent/10 text-pax-accent">
                    Native
                  </span>
                )}
                {t.isLaunchpad && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-pax-accent/10 text-pax-accent">
                    Launchpad
                  </span>
                )}
              </button>
            ))}
        </div>
      </ModalPanel>
    </div>
  );
}

export interface PaxSwapModalProps {
  quotes: UseSwapQuotesResult;
  fromToken: SwapToken;
  toToken: SwapToken;
  fromAmount: string;
  slippageBps: number;
  onFromTokenChange: (t: SwapToken) => void;
  onToTokenChange: (t: SwapToken) => void;
  onFromAmountChange: (v: string) => void;
  onFlip: () => void;
}

export const PaxSwapModal = (props: PaxSwapModalProps) => {
  const { activeAccount } = useWalletState();
  const recipient = activeAccount?.address;

  const fromToken = props.fromToken;
  const toToken = props.toToken;
  const inputValue = props.fromAmount;
  const setFromToken = props.onFromTokenChange;
  const setToToken = props.onToTokenChange;
  const setInputValue = props.onFromAmountChange;
  const value = parseFloat(inputValue || '0') || 0;

  const [selectorTarget, setSelectorTarget] = useState<'from' | 'to' | null>(null);
  const heldTokens = useHeldTokens(recipient);
  const outputCatalogue = useOutputTokenCatalogue();
  const outputTokens = outputCatalogue.data ?? [];
  const balances = useSwapBalances(recipient, fromToken, toToken);
  const { data: paxPriceData } = usePaxPriceQuery();
  const quotes = props.quotes;
  const fromBalanceNum = parseFloat(balances.fromBalance || '0') || 0;
  const isInsufficient = (() => {
    if (value <= 0 || balances.fromBalanceRaw === null || balances.fromBalanceRaw === undefined)
      return false;
    const inputWei = parseAmountToWei(inputValue, fromToken.decimals);
    if (!inputWei) return false;
    return BigInt(inputWei) > balances.fromBalanceRaw;
  })();
  const usdValue = useMemo(() => {
    if (!value) return 0;
    if (fromToken.isNative && paxPriceData?.latest) return value * paxPriceData.latest;
    if (toToken.isStablecoin && quotes.bestQuote)
      return parseFloat(quotes.bestQuote.amountOutDisplay.replace(/,/g, '')) || 0;
    return 0;
  }, [value, fromToken.isNative, paxPriceData, toToken.isStablecoin, quotes.bestQuote]);
  const showUsdValue = usdValue > 0 && !isInsufficient && !quotes.quoting;

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value;
    if (!/^[0-9]*\.?[0-9]*$/.test(v) && v !== '') return;
    setInputValue(v);
  };
  const handleUseMax = () => {
    if (!balances.fromBalanceRaw) return;
    setInputValue(formatBalanceFull(balances.fromBalanceRaw, fromToken.decimals));
  };
  const handleClear = () => {
    setInputValue('');
  };
  const handleFlip = props.onFlip;
  const receiveDisplay = quotes.quoting
    ? null
    : quotes.bestQuote
      ? quotes.bestQuote.amountOutDisplay.replace(/,/g, '')
      : null;
  const errorMessage = isInsufficient
    ? `Not Enough ${fromToken.symbol}`
    : quotes.quoteError && !quotes.quoting && value > 0
      ? 'No Route Found'
      : null;

  return (
    <>
      {selectorTarget && (
        <TokenSelectorSheet
          title={selectorTarget === 'from' ? 'You Pay' : 'You Receive'}
          tokens={selectorTarget === 'from' ? heldTokens : outputTokens}
          selected={selectorTarget === 'from' ? fromToken : toToken}
          enableSearch={selectorTarget === 'to'}
          loading={selectorTarget === 'to' && outputCatalogue.isPending}
          error={selectorTarget === 'to' ? outputCatalogue.error : null}
          onSelect={(token) => {
            if (selectorTarget === 'from') setFromToken(token);
            else setToToken(token);
            setSelectorTarget(null);
            handleClear();
          }}
          onClose={() => setSelectorTarget(null)}
        />
      )}
      <div className="font-sans w-full text-pax-light">
        <div className="flex flex-col gap-3">
          <div className="relative flex w-full flex-col gap-5 rounded-3xl bg-pax-card p-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <button
                type="button"
                aria-label="Select pay token"
                onClick={() => setSelectorTarget('from')}
                className="flex max-w-full items-center gap-2.5 rounded-xl p-2"
              >
                <TokenIcon token={fromToken} />
                <div className="min-w-0 text-left break-words">
                  <div className="flex items-center gap-1">
                    <h2 className="text-base font-semibold">{fromToken.symbol}</h2>
                    <ChevronDown className="size-3.5 shrink-0" />
                  </div>
                  <p className="text-xs text-pax-muted">
                    {balances.loading ? '…' : (balances.fromBalance ?? '—')}
                  </p>
                </div>
              </button>
              <button
                type="button"
                onClick={handleUseMax}
                disabled={fromBalanceNum === 0}
                className="rounded-full bg-pax-control px-3 py-1 text-sm font-semibold disabled:opacity-30"
              >
                {value > 0 && value >= fromBalanceNum && fromBalanceNum > 0
                  ? 'Using Max'
                  : 'Use Max'}
              </button>
            </div>
            <div className="mb-6 flex w-full flex-col items-center gap-4">
              <input
                id="pax-swap-amount"
                type="text"
                name="swap-amount"
                inputMode="decimal"
                autoComplete="off"
                aria-label="Amount to pay"
                className="w-full rounded-xl bg-transparent px-3 py-2 text-center text-display font-semibold tracking-tight text-pax-light"
                placeholder="0"
                value={inputValue}
                onChange={handleInputChange}
              />
              <div
                aria-live="polite"
                className="flex min-h-14 w-full items-center justify-center gap-2 text-center"
              >
                {errorMessage ? (
                  <p role="alert" className="font-semibold text-pax-error">
                    {errorMessage}
                  </p>
                ) : quotes.quoting ? (
                  <Loader2
                    aria-label="Loading quote"
                    className="size-5 animate-spin text-pax-muted"
                  />
                ) : showUsdValue ? (
                  <p className="flex flex-wrap items-center justify-center gap-2 font-semibold">
                    <Equal className="size-5" />
                    {formatUsd(usdValue)}
                    <ArrowDownUp className="size-5" />
                  </p>
                ) : value > 0 ? (
                  <p className="break-words text-sm text-pax-muted">
                    {inputValue} {fromToken.symbol}
                  </p>
                ) : null}
              </div>
            </div>
            <button
              type="button"
              onClick={handleFlip}
              className="absolute -bottom-6 self-center rounded-full bg-pax-surface p-1.5"
              aria-label="Flip tokens"
            >
              <ChevronDown className="size-5" />
            </button>
          </div>
          <div className="flex w-full flex-wrap items-center justify-between gap-3 rounded-3xl bg-pax-card p-3">
            <button
              type="button"
              aria-label="Select receive token"
              onClick={() => setSelectorTarget('to')}
              className="flex max-w-full items-center gap-2.5 rounded-xl p-2"
            >
              <TokenIcon token={toToken} />
              <div className="min-w-0 text-left break-words">
                <div className="flex items-center gap-1">
                  <h2 className="text-base font-semibold">{toToken.symbol}</h2>
                  <ChevronDown className="size-3.5 shrink-0" />
                </div>
                <p className="text-sm text-pax-muted">Receive {toToken.name}</p>
              </div>
            </button>
            <p className="min-h-8 max-w-full break-words pr-2 text-lg font-semibold">
              {quotes.quoting ? (
                <Loader2
                  aria-label="Loading received amount"
                  className="size-5 animate-spin text-pax-muted"
                />
              ) : receiveDisplay ? (
                <NumberFlow
                  animated={false}
                  value={parseFloat(receiveDisplay)}
                  format={{ maximumFractionDigits: 8 }}
                />
              ) : (
                <span className="text-pax-muted">—</span>
              )}
            </p>
          </div>
          {inputValue && (
            <button
              type="button"
              onClick={handleClear}
              className="w-full rounded-full bg-pax-control py-2 text-sm text-pax-muted"
            >
              Clear
            </button>
          )}
        </div>
      </div>
    </>
  );
};
