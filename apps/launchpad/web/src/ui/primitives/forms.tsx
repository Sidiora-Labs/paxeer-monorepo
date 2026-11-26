'use client';

import { useEffect, useId, useState, type HTMLAttributes, type ReactNode } from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '@/lib/utils';
import { ProgressBar } from './data';

export function ProgressLine({ label, value, className }: {
  label: string;
  value: number | null;
  className?: string;
}) {
  return (
    <div className={cn('flex min-w-0 flex-col gap-1', className)}>
      <div className="flex items-center justify-between gap-2 text-xs text-text-muted">
        <span>{label}</span>
        <span data-numeric className="tabular-nums">{value === null ? '—' : `${Math.round(Math.max(0, Math.min(100, value)))}%`}</span>
      </div>
      <ProgressBar label={label} value={value} tone="accent" size={6} className="w-full" />
    </div>
  );
}

export interface OptionRowProps {
  label: string;
  choice: ReactNode;
  children: ReactNode;
  error?: string | null;
  disabled?: boolean;
  className?: string;
}

export function OptionRow({ label, choice, children, error, disabled, className }: OptionRowProps) {
  const [open, setOpen] = useState(Boolean(error));
  const id = useId();
  useEffect(() => { if (error) setOpen(true); }, [error]);
  return (
    <details open={open} onToggle={event => setOpen(event.currentTarget.open)} className={cn('sl-hairline min-w-0', className)}>
      <summary
        id={`${id}-label`}
        aria-controls={`${id}-panel`}
        aria-expanded={open}
        aria-disabled={disabled || undefined}
        onClick={event => { event.preventDefault(); if (!disabled) setOpen(!open); }}
        className="flex min-h-12 cursor-pointer list-none items-center gap-3 py-2 text-sm font-semibold text-text [&::-webkit-details-marker]:hidden"
      >
        <span className="min-w-0 flex-1 truncate" title={label}>{label}</span>
        <span className="min-w-0 max-w-full truncate font-medium text-text-muted">{choice}</span>
        <ChevronDown aria-hidden="true" className={cn('size-icon-md shrink-0 text-text-muted transition-transform duration-base', open && 'rotate-180')} />
      </summary>
      <div id={`${id}-panel`} role="region" aria-labelledby={`${id}-label`} hidden={!open} className="pb-4">
        {children}
        {error ? <p role="alert" className="text-xs text-danger">{error}</p> : null}
      </div>
    </details>
  );
}

export interface PanelProps extends HTMLAttributes<HTMLElement> {
  nested?: boolean;
  flush?: boolean;
}

export function Panel({ nested = false, flush = false, className, children, ...props }: PanelProps) {
  return (
    <section {...props} className={cn(
      'min-w-0 p-4 shadow-none',
      nested ? 'rounded-sm bg-surface-2' : 'rounded-md bg-surface-1',
      flush && 'sl-hairline rounded-none bg-canvas',
      className
    )}>{children}</section>
  );
}
