'use client';

import { useEffect } from 'react';
import { TriangleAlert } from 'lucide-react';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Unhandled error:', error);
  }, [error]);

  return (
    <div className="flex min-h-[var(--sl-page-min-height)] flex-col items-center justify-center gap-6 p-8" role="alert">
      <div
        className="flex h-20 w-20 items-center justify-center rounded-lg bg-danger-soft text-danger"
      >
        <TriangleAlert className="size-9" aria-hidden="true" />
      </div>

      <div
        className="space-y-2 text-center"
      >
        <h1 className="text-xl font-bold text-text">Something went wrong</h1>
        <p className="max-w-sm text-sm text-text-muted">
          Your funds are safe. Try again or go home
        </p>
      </div>

      <div
        className="flex flex-wrap justify-center gap-3"
      >
        <button
          type="button"
          onClick={reset}
          className="min-h-control-height-md rounded-sm bg-accent px-4 text-base font-bold text-on-accent transition-colors hover:bg-accent-hover active:bg-accent-active"
        >
          Try again
        </button>
        <button
          type="button"
          onClick={() => { window.location.href = '/'; }}
          className="min-h-control-height-md rounded-sm bg-surface-2 px-4 text-base font-bold text-text shadow-[inset_0_0_0_var(--sl-edge-width)_var(--sl-control-edge)] transition-colors hover:bg-surface-hover active:bg-surface-active"
        >
          Go home
        </button>
      </div>
    </div>
  );
}
