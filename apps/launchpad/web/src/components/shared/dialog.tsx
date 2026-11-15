'use client';

import * as React from 'react';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';
import { cn } from '@/lib/utils';

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
}

const DialogContext = React.createContext<{
  open: boolean;
  close: () => void;
  titleId: string;
} | null>(null);

export function Dialog({ open, onOpenChange, children }: DialogProps) {
  const titleId = React.useId();
  return (
    <DialogContext.Provider value={{ open, close: () => onOpenChange(false), titleId }}>
      {children}
    </DialogContext.Provider>
  );
}

export function DialogContent({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  const context = React.useContext(DialogContext);
  if (!context) throw new Error('DialogContent requires Dialog');
  return (
    <ModalDialog
      open={context.open}
      onClose={() => {
        context.close();
        window.dispatchEvent(new CustomEvent('dialog-close'));
      }}
      aria-labelledby={context.titleId}
      aria-label={props['aria-label'] ?? 'Dialog'}
      className="max-w-modal-width-md flex-col rounded-lg shadow-modal"
    >
      <div className={cn('relative p-6', className)} {...props}>
        {children}
      </div>
    </ModalDialog>
  );
}

export function DialogHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn('flex flex-col space-y-1.5 text-center sm:text-left', className)}
      {...props}
    />
  );
}

export function DialogTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  const context = React.useContext(DialogContext);
  return (
    <h2
      id={context?.titleId}
      className={cn(
        'text-lg font-semibold leading-[var(--sl-leading-none)] tracking-tight',
        className
      )}
      {...props}
    />
  );
}
