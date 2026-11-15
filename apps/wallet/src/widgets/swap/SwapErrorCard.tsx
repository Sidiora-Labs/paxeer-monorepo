'use client';

/**
 * Inline error card with friendly copy mapped from a raw revert / exception.
 *
 * Also accepts a precomputed `pretty` object for cases where the caller
 * already mapped the error (e.g. tests or batched errors).
 */

import { AlertTriangle } from 'lucide-react';
import { prettifySwapError, type PrettyError } from './prettifyError';

export interface SwapErrorCardProps {
  error: string;
  pretty?: PrettyError;
}

export function SwapErrorCard({ error, pretty }: SwapErrorCardProps) {
  const { title, reason, tip } = pretty ?? prettifySwapError(error);
  return (
    <div className="mt-3 rounded-xl bg-pax-error-muted   p-3 space-y-1.5 animate-scale-in">
      <div className="flex items-center gap-2">
        <AlertTriangle className="w-3.5 h-3.5 text-pax-error shrink-0" />
        <span className="text-xs font-semibold text-pax-error">{title}</span>
      </div>
      <p className="text-xs text-pax-error leading-relaxed">{reason}</p>
      <p className="text-xs text-pax-muted leading-relaxed">Tip: {tip}</p>
    </div>
  );
}
