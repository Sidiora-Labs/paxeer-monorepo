'use client';

/**
 * Full-page token picker rendered when the swap widget enters
 * `selectFrom` / `selectTo` view.
 *
 * Local filter on the in-memory tokens list, plus an optional debounced
 * paxscan search for the output side that returns extra results from the
 * tokens index.
 */

import { useEffect, useMemo, useRef, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { searchTokens } from '@/lib/api';
import type { SwapToken } from '@/lib/swap';
import { TokenBadge } from './TokenBadge';
import { paxscanToSwapToken } from './util';
import Image from 'next/image';

export interface TokenSelectorViewProps {
  title: string;
  tokens: SwapToken[];
  selected: SwapToken;
  exclude: SwapToken;
  onSelect: (t: SwapToken) => void;
  onClose: () => void;
  /** Enable paxscan-backed search for the output side. */
  enableSearch?: boolean;
}

export function TokenSelectorView({
  title,
  tokens,
  selected,
  exclude,
  onSelect,
  onClose,
  enableSearch = false,
}: TokenSelectorViewProps) {
  const [query, setQuery] = useState('');
  const [searchResults, setSearchResults] = useState<SwapToken[]>([]);
  const [searching, setSearching] = useState(false);
  const searchRef = useRef<ReturnType<typeof setTimeout>>();

  // Debounced paxscan search.
  useEffect(() => {
    if (!enableSearch || !query.trim()) {
      setSearchResults([]);
      return;
    }
    if (searchRef.current) clearTimeout(searchRef.current);
    setSearching(true);
    searchRef.current = setTimeout(async () => {
      try {
        const results = await searchTokens(query);
        setSearchResults(
          results.map(paxscanToSwapToken).filter((token): token is SwapToken => token !== null)
        );
      } catch {
        setSearchResults([]);
      } finally {
        setSearching(false);
      }
    }, 400);
    return () => {
      if (searchRef.current) clearTimeout(searchRef.current);
    };
  }, [query, enableSearch]);

  // Local filter applied to the in-memory list.
  const filteredTokens = useMemo(() => {
    if (!query.trim()) return tokens;
    const q = query.trim().toLowerCase();
    return tokens.filter(
      (t) =>
        t.symbol.toLowerCase().includes(q) ||
        t.name.toLowerCase().includes(q) ||
        t.address.toLowerCase().includes(q)
    );
  }, [tokens, query]);

  // Merge local filter results with paxscan search results, deduped by address.
  const displayTokens = useMemo(() => {
    if (!query.trim()) return tokens;
    const seen = new Set(filteredTokens.map((t) => t.address.toLowerCase() || 'native'));
    const merged = [...filteredTokens];
    for (const t of searchResults) {
      const key = t.address.toLowerCase();
      if (!seen.has(key)) {
        seen.add(key);
        merged.push(t);
      }
    }
    return merged;
  }, [tokens, filteredTokens, searchResults, query]);

  const isSelected = (t: SwapToken) =>
    t.symbol === selected.symbol && t.address === selected.address;
  const isExcluded = (t: SwapToken) => t.symbol === exclude.symbol && t.address === exclude.address;

  return (
    <div className="px-4 pt-4 pb-24">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-bold">{title}</h2>
        <button
          type="button"
          onClick={onClose}
          aria-label={`Close ${title}`}
          className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] p-2 rounded-full bg-pax-control press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="x" className="w-4 h-4" />
        </button>
      </div>

      <div className="relative mb-4">
        <label htmlFor="swap-token-search" className="sr-only">
          {enableSearch ? 'Search tokens' : 'Filter tokens'}
        </label>
        <Image
          src="/wallet/ui_icons/search.svg"
          alt=""
          className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4"
          width={16}
          height={16}
        />
        <input
          id="swap-token-search"
          type="text"
          name="token-search"
          autoComplete="off"
          aria-label={enableSearch ? 'Search tokens' : 'Filter tokens'}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={enableSearch ? 'Search by name or paste address...' : 'Filter tokens...'}
          className="w-full pl-10 pr-4 py-3 rounded-xl bg-pax-control   text-sm outline-none  transition-colors placeholder:text-pax-muted"
        />
        {query && (
          <button
            type="button"
            aria-label="Clear token search"
            onClick={() => setQuery('')}
            className="absolute right-2 top-1/2 min-h-[var(--touch-target)] min-w-[var(--touch-target)] -translate-y-1/2 p-2"
          >
            <SvgIcon name="x" className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {searching && (
        <div className="flex items-center gap-2 px-1 mb-3">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-pax-muted" />
          <span className="text-xs text-pax-muted">Searching tokens...</span>
        </div>
      )}

      <div className="space-y-1.5">
        {displayTokens.length === 0 && !searching && query.trim() && (
          <div className="py-8 text-center">
            <p className="text-xs text-pax-muted">No tokens found for &ldquo;{query}&rdquo;</p>
          </div>
        )}
        {displayTokens
          .filter((t) => !isExcluded(t))
          .map((t) => {
            const sel = isSelected(t);
            return (
              <button
                type="button"
                key={t.address || 'native'}
                onClick={() => onSelect(t)}
                className={`min-h-[var(--touch-target)] w-full flex items-center gap-3 px-3 py-3 rounded-xl transition-all press-scale focus-visible:ring-2 focus-visible:ring-pax-accent ${
                  sel ? 'bg-pax-accent/10' : 'bg-pax-control hover:bg-pax-overlay'
                }`}
              >
                <TokenBadge token={t} />
                <div className="flex-1 text-left min-w-0">
                  <p className="text-sm font-medium">{t.symbol}</p>
                  <p className="text-size-11 text-pax-muted truncate">{t.name}</p>
                </div>
                {t.isStablecoin && (
                  <span className="text-size-10 px-1.5 py-0.5 rounded bg-pax-info-muted text-pax-info">
                    Stable
                  </span>
                )}
                {t.isNative && (
                  <span className="text-size-10 px-1.5 py-0.5 rounded bg-pax-accent/10 text-pax-accent">
                    Native
                  </span>
                )}
                {t.isLaunchpad && (
                  <span className="text-size-10 px-1.5 py-0.5 rounded bg-pax-accent/10 text-pax-accent">
                    Launchpad
                  </span>
                )}
                {sel && <SvgIcon name="check" className="w-4 h-4" />}
              </button>
            );
          })}
      </div>
    </div>
  );
}
