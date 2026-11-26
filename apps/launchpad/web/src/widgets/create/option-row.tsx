'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { DisclosureRow } from '@/ui/primitives';

export interface OptionRowProps {
  id: string;
  label: string;
  choice: string;
  error?: string | null;
  failed?: boolean;
  defaultOpen?: boolean;
  children: ReactNode;
}

export function OptionRow({ id, label, choice, error, failed = false, defaultOpen = false, children }: OptionRowProps) {
  const [open, setOpen] = useState(defaultOpen || failed || Boolean(error));
  useEffect(() => {
    if (error || failed) setOpen(true);
  }, [error, failed]);
  return (
    <DisclosureRow
      id={id}
      label={label}
      open={open}
      onToggle={setOpen}
      trailing={<span title={choice} className="max-w-1/2 truncate text-xs font-medium text-text-muted">{choice}</span>}
      className="border-b border-surface-3 [&>summary]:flex-nowrap [&>summary>span]:basis-auto"
    >
      <div className="flex min-w-0 flex-col gap-3">
        {error ? <p role="alert" className="text-xs text-danger">{error}</p> : null}
        {children}
      </div>
    </DisclosureRow>
  );
}
