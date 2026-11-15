'use client';

import { useSyncExternalStore } from 'react';
import { AlertTriangle, RotateCw, X } from 'lucide-react';
import {
  clearBackgroundFailure,
  getBackgroundFailures,
  subscribeBackgroundFailures,
} from '@/platform/status/background-failures';

const EMPTY: ReturnType<typeof getBackgroundFailures> = [];

export function BackgroundStatus() {
  const failures = useSyncExternalStore(
    subscribeBackgroundFailures,
    getBackgroundFailures,
    () => EMPTY
  );
  const failure = failures[0];
  if (!failure) return null;

  return (
    <div
      role={failure.retryable ? 'status' : 'alert'}
      aria-live="polite"
      className="pax-background-status fixed left-3 right-3 top-[calc(var(--safe-area-top)+var(--space-16))] z-[70] mx-auto flex max-w-lg items-start gap-3 rounded-2xl bg-pax-warning-surface px-4 py-3 text-pax-warning shadow-menu"
    >
      {failure.retryable ? (
        <RotateCw aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0" />
      ) : (
        <AlertTriangle aria-hidden="true" className="mt-0.5 h-4 w-4 shrink-0" />
      )}
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold">Saved state needs attention</p>
        <p className="mt-0.5 text-xs text-pax-warning">{failure.message}</p>
      </div>
      <button
        type="button"
        aria-label="Dismiss status"
        onClick={() => clearBackgroundFailure(failure.id)}
        className="rounded-lg bg-pax-warning-surface p-1.5"
      >
        <X aria-hidden="true" className="h-4 w-4" />
      </button>
    </div>
  );
}
