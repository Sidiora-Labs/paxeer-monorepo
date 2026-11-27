'use client';

import { useEffect, useId, useState, type HTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { ProgressBar } from './data';
import { DisclosureRow } from './states';

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
  id?: string;
  failed?: boolean;
  defaultOpen?: boolean;
  label: string;
  choice: ReactNode;
  children: ReactNode;
  error?: string | null;
  disabled?: boolean;
  className?: string;
}

export function OptionRow({ id: providedId, label, choice, children, error, failed = false, defaultOpen = false, disabled, className }: OptionRowProps) {
  const [open, setOpen] = useState(defaultOpen || failed || Boolean(error));
  const generatedId = useId();
  const id = providedId ?? generatedId;
  useEffect(() => { if (error || failed) setOpen(true); }, [error, failed]);
  return (
    <DisclosureRow
      id={id}
      label={label}
      open={open}
      onToggle={setOpen}
      disabled={disabled}
      trailing={<span title={typeof choice === 'string' ? choice : undefined} className="max-w-1/2 truncate text-xs font-medium text-text-muted">{choice}</span>}
      className={cn('sl-hairline min-w-0 [&>summary]:flex-nowrap [&>summary>span]:basis-auto', className)}
    >
      <div className="flex min-w-0 flex-col gap-3">
        {error ? <p role="alert" className="text-xs text-danger">{error}</p> : null}
        {children}
      </div>
    </DisclosureRow>
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
