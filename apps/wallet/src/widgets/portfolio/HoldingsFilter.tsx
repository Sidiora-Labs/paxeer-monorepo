'use client';

/**
 * Holdings section header + filter dropdown.
 *
 * The dropdown lets the user globally hide dust (< $1) and toggle per-token
 * visibility. State is owned by the parent via `useTokenFilters` so the
 * filter logic is shared with the grid that renders below.
 */

import { useState } from 'react';
import { SvgIcon } from '@/components/ui/SvgIcon';

interface Holding {
  contract_address?: string;
  symbol?: string | null;
}

export interface HoldingsFilterProps {
  allHoldings: Holding[];
  hideDust: boolean;
  hiddenTokens: Set<string>;
  onToggleHideDust: () => void;
  onToggleTokenVisibility: (address: string) => void;
}

export function HoldingsFilter({
  allHoldings,
  hideDust,
  hiddenTokens,
  onToggleHideDust,
  onToggleTokenVisibility,
}: HoldingsFilterProps) {
  const [open, setOpen] = useState(false);
  const hasActiveFilter = hideDust || hiddenTokens.size > 0;

  return (
    <div className="col-span-2 flex items-center justify-between px-1 pt-2">
      <h3 className="text-size-13 font-bold text-pax-muted uppercase tracking-[0.06em]">
        Holdings
      </h3>
      <div className="relative">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-size-11 font-medium press-scale transition-all ${
            hasActiveFilter ? 'bg-pax-accent/10 text-pax-accent' : 'bg-pax-control text-pax-muted'
          }`}
        >
          <SvgIcon name="sliders" className="w-3 h-3" />
          Filter
          {hasActiveFilter && <span className="w-1.5 h-1.5 rounded-full bg-pax-accent" />}
        </button>
        {open && (
          <>
            <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
            <div className="absolute right-0 top-full mt-1.5 w-56 max-w-[calc(100vw-2rem)] bg-pax-card   rounded-xl shadow-2xl z-50 p-3 space-y-3 animate-scale-in">
              <button
                type="button"
                onClick={onToggleHideDust}
                className="min-h-[var(--touch-target)] w-full flex flex-wrap gap-2 items-center justify-between px-2 py-2 rounded-lg hover:bg-pax-control transition-all"
              >
                <div className="flex items-center gap-2">
                  <SvgIcon name="filter" className="w-3.5 h-3.5" />
                  <span className="text-xs">Hide dust (&lt;$1)</span>
                </div>
                <div
                  className={`w-8 h-4.5 shrink-0 rounded-full transition-all flex items-center ${
                    hideDust ? 'bg-pax-accent justify-end' : 'bg-pax-overlay justify-start'
                  }`}
                >
                  <div className="w-3.5 h-3.5 rounded-full bg-pax-light mx-0.5 shadow-sm" />
                </div>
              </button>
              {allHoldings.length > 0 && (
                <div>
                  <p className="text-size-10 text-pax-muted uppercase tracking-wide mb-1.5 px-2">
                    Toggle tokens
                  </p>
                  <div className="max-h-40 overflow-y-auto space-y-0.5">
                    {allHoldings.map((h) => {
                      const addr = (h.contract_address || '').toLowerCase();
                      const isHid = hiddenTokens.has(addr);
                      return (
                        <button
                          type="button"
                          key={addr}
                          onClick={() => onToggleTokenVisibility(addr)}
                          className="w-full flex flex-wrap gap-2 items-center justify-between px-2 py-1.5 rounded-lg hover:bg-pax-control transition-all"
                        >
                          <span
                            className={`text-xs truncate ${isHid ? 'text-pax-muted line-through' : ''}`}
                          >
                            {h.symbol || '???'}
                          </span>
                          <div
                            className={`w-8 h-4.5 shrink-0 rounded-full transition-all flex items-center ${
                              !isHid ? 'bg-pax-accent justify-end' : 'bg-pax-overlay justify-start'
                            }`}
                          >
                            <div className="w-3.5 h-3.5 rounded-full bg-pax-light mx-0.5 shadow-sm" />
                          </div>
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
