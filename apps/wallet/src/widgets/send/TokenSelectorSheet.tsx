'use client';

/**
 * Bottom-sheet token picker.
 *
 * Shows a spinner while the underlying tokens query is pending, an empty
 * placeholder if there's nothing to send, otherwise a tappable list with the
 * current selection highlighted.
 */

import { SvgIcon } from '@/components/ui/SvgIcon';
import { Dialog } from '@/components/ui/primitives';
import { TokenIcon } from './TokenIcon';
import type { SendableToken } from './useSendableTokens';
import { Loader2 } from 'lucide-react';

export interface TokenSelectorSheetProps {
  open: boolean;
  onClose: () => void;
  tokens: SendableToken[];
  loading: boolean;
  selected: SendableToken | null;
  onSelect: (token: SendableToken) => void;
}

const isSameToken = (a: SendableToken | null, b: SendableToken): boolean =>
  a !== null && a.symbol === b.symbol && a.address === b.address;

export function TokenSelectorSheet({
  open,
  onClose,
  tokens,
  loading,
  selected,
  onSelect,
}: TokenSelectorSheetProps) {
  if (!open) return null;

  return (
    <Dialog open={open} onClose={onClose} title="Select Token" variant="sheet">
      <div className="space-y-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-base font-bold">Select Token</h3>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close token selector"
            className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] p-2 rounded-full bg-pax-control press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
          >
            <SvgIcon name="x" className="w-4 h-4" />
          </button>
        </div>
        <div className="space-y-1 max-h-72 overflow-y-auto">
          {loading ? (
            <div className="py-8 text-center">
              <Loader2
                aria-label="Loading tokens"
                className="mx-auto mb-2 h-5 w-5 animate-spin text-pax-accent"
              />
              <p className="text-xs text-pax-muted">Loading tokens...</p>
            </div>
          ) : tokens.length === 0 ? (
            <p className="text-xs text-pax-muted text-center py-8">No tokens found</p>
          ) : (
            tokens.map((t) => {
              const sel = isSameToken(selected, t);
              return (
                <button
                  type="button"
                  key={t.address || 'native'}
                  onClick={() => {
                    onSelect(t);
                    onClose();
                  }}
                  className={`min-h-[var(--touch-target)] w-full flex items-center gap-3 px-3 py-3 rounded-xl transition-all press-scale focus-visible:ring-2 focus-visible:ring-pax-accent ${
                    sel ? 'bg-pax-accent/10' : 'bg-pax-control hover:bg-pax-overlay'
                  }`}
                >
                  <TokenIcon token={t} />
                  <div className="flex-1 text-left min-w-0">
                    <p className="text-sm font-medium">{t.symbol}</p>
                    <p className="text-size-11 text-pax-muted truncate">{t.name}</p>
                  </div>
                  <div className="text-right shrink-0">
                    <p className="text-xs font-medium">{t.balance}</p>
                  </div>
                  {sel && <SvgIcon name="check" className="w-4 h-4" />}
                </button>
              );
            })
          )}
        </div>
      </div>
    </Dialog>
  );
}
