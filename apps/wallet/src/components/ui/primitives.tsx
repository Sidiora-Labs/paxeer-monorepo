'use client';

import {
  type ButtonHTMLAttributes,
  type InputHTMLAttributes,
  type HTMLAttributes,
  useLayoutEffect,
  type ReactNode,
  useEffect,
  useId,
  useRef,
  useState,
} from 'react';
import { createPortal } from 'react-dom';
import { AlertTriangle, ChevronDown, LoaderCircle, X } from 'lucide-react';
import { cn } from '@/lib/cn';

export function Button({
  variant = 'primary',
  loading = false,
  className,
  type = 'button',
  children,
  disabled,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'secondary' | 'danger' | 'quiet';
  loading?: boolean;
}) {
  const styles = {
    primary: 'bg-pax-accent text-pax-on-accent',
    secondary: 'bg-pax-control text-pax-light',
    danger: 'bg-pax-error-surface text-pax-error',
    quiet: 'bg-transparent text-pax-light hover:bg-pax-control',
  };
  return (
    <button
      type={type}
      className={cn(
        'inline-flex min-h-[var(--touch-target)] items-center justify-center gap-2 rounded-xl px-4 py-2 text-sm font-semibold transition-[background-color,transform,opacity] focus-visible:ring-2 focus-visible:ring-pax-accent focus-visible:ring-offset-2 focus-visible:ring-offset-pax-bg disabled:cursor-not-allowed disabled:opacity-40 active:scale-[var(--scale-press)]',
        styles[variant],
        className
      )}
      aria-busy={loading || undefined}
      disabled={loading || disabled}
      {...props}
    >
      {loading && <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />}
      {children}
    </button>
  );
}

export function TextField({
  label,
  error,
  hint,
  className,
  id: suppliedId,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  error?: string;
  hint?: string;
}) {
  const generatedId = useId();
  const id = suppliedId ?? generatedId;
  const descriptionId = `${id}-description`;
  return (
    <label htmlFor={id} className="block space-y-2">
      <span className="block text-sm font-semibold text-pax-light">{label}</span>
      <input
        id={id}
        aria-invalid={Boolean(error)}
        aria-describedby={error || hint ? descriptionId : undefined}
        className={cn(
          'min-h-[var(--touch-target)] w-full rounded-xl bg-pax-control px-3 py-2 text-base text-pax-light outline-none placeholder:text-pax-mid focus:bg-pax-overlay focus-visible:ring-2 focus-visible:ring-pax-accent',
          className
        )}
        {...props}
      />
      {(error || hint) && (
        <span
          id={descriptionId}
          role={error ? 'alert' : undefined}
          className={cn('block text-xs', error ? 'text-pax-error' : 'text-pax-muted')}
        >
          {error ?? hint}
        </span>
      )}
    </label>
  );
}

export function AmountText({
  value,
  symbol,
  label,
  className,
}: {
  value: string;
  symbol?: string;
  label: string;
  className?: string;
}) {
  return (
    <span
      aria-label={label}
      className={cn('tabular-nums break-words font-semibold', className)}
      title={`${value}${symbol ? ` ${symbol}` : ''}`}
    >
      {value}
      {symbol && <span className="ml-1 text-pax-muted">{symbol}</span>}
    </span>
  );
}

export function SegmentedControl<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly { value: T; label: string }[];
  onChange: (value: T) => void;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className="flex flex-wrap gap-1 rounded-xl bg-pax-surface p-1"
    >
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          aria-checked={value === option.value}
          tabIndex={value === option.value ? 0 : -1}
          onKeyDown={(event) => {
            const index = options.findIndex((entry) => entry.value === option.value);
            const next =
              event.key === 'Home'
                ? 0
                : event.key === 'End'
                  ? options.length - 1
                  : ['ArrowRight', 'ArrowDown'].includes(event.key)
                    ? (index + 1) % options.length
                    : ['ArrowLeft', 'ArrowUp'].includes(event.key)
                      ? (index - 1 + options.length) % options.length
                      : null;
            if (next === null) return;
            event.preventDefault();
            onChange(options[next].value);
            const controls =
              event.currentTarget.parentElement?.querySelectorAll<HTMLButtonElement>(
                '[role="radio"]'
              );
            controls?.[next].focus();
          }}
          onClick={() => onChange(option.value)}
          className={cn(
            'min-h-[var(--touch-target)] flex-1 basis-20 rounded-lg px-3 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-pax-accent',
            value === option.value ? 'bg-pax-overlay text-pax-light' : 'text-pax-muted'
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export function Switch({
  checked,
  onCheckedChange,
  label,
  description,
}: {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: string;
  description?: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={() => onCheckedChange(!checked)}
      className="flex min-h-[var(--touch-target)] w-full items-center justify-between gap-4 text-left focus-visible:ring-2 focus-visible:ring-pax-accent"
    >
      <span>
        <span className="block text-sm font-medium">{label}</span>
        {description && <span className="mt-0.5 block text-xs text-pax-muted">{description}</span>}
      </span>
      <span
        aria-hidden="true"
        className={cn(
          'flex h-7 w-12 shrink-0 items-center rounded-full p-1 transition-colors',
          checked ? 'bg-pax-accent' : 'bg-pax-control'
        )}
      >
        <span
          className={cn(
            'h-5 w-5 rounded-full bg-[var(--color-switch-thumb)] transition-transform',
            checked && 'translate-x-5'
          )}
        />
      </span>
    </button>
  );
}

function focusable(container: HTMLElement): HTMLElement[] {
  return [
    ...container.querySelectorAll<HTMLElement>(
      'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])'
    ),
  ];
}

export function ModalPanel({
  children,
  onDismiss,
  className,
  ...props
}: HTMLAttributes<HTMLDialogElement> & {
  onDismiss: () => void;
  children: ReactNode;
}) {
  const panel = useRef<HTMLDialogElement>(null);
  const dismiss = useRef(onDismiss);
  dismiss.current = onDismiss;

  useLayoutEffect(() => {
    const canScroll = (target: EventTarget | null, delta: number) => {
      let element = target instanceof Element ? target : null;
      const dialog = element?.closest('.pax-modal-panel[open]');
      if (!dialog) return false;
      while (element) {
        const style = getComputedStyle(element);
        if (/auto|scroll/.test(style.overflowY) && element.scrollHeight > element.clientHeight) {
          if (delta < 0 && element.scrollTop > 0) return true;
          if (delta > 0 && element.scrollTop + element.clientHeight < element.scrollHeight)
            return true;
        }
        if (element === dialog) break;
        element = element.parentElement;
      }
      return false;
    };
    const wheel = (event: WheelEvent) => {
      if (!canScroll(event.target, event.deltaY)) event.preventDefault();
    };
    let touchY = 0;
    const touchStart = (event: TouchEvent) => {
      touchY = event.touches[0]?.clientY ?? 0;
    };
    const touchMove = (event: TouchEvent) => {
      const next = event.touches[0]?.clientY ?? touchY;
      if (!canScroll(event.target, touchY - next)) event.preventDefault();
      touchY = next;
    };
    const keydown = (event: KeyboardEvent) => {
      if (
        event.target instanceof Element &&
        (event.target.matches('input, textarea, select, [contenteditable=true]') ||
          (event.key === ' ' &&
            event.target.closest(
              'button, a, summary, [role=button], [role=radio], [role=checkbox], [role=switch]'
            )))
      )
        return;
      const delta = ['ArrowDown', 'PageDown', 'End', ' '].includes(event.key)
        ? 1
        : ['ArrowUp', 'PageUp', 'Home'].includes(event.key)
          ? -1
          : 0;
      if (delta && !canScroll(event.target, event.shiftKey ? -delta : delta))
        event.preventDefault();
    };
    document.addEventListener('wheel', wheel, { passive: false });
    document.addEventListener('touchstart', touchStart, { passive: true });
    document.addEventListener('touchmove', touchMove, { passive: false });
    document.addEventListener('keydown', keydown);
    return () => {
      document.removeEventListener('wheel', wheel);
      document.removeEventListener('touchstart', touchStart);
      document.removeEventListener('touchmove', touchMove);
      document.removeEventListener('keydown', keydown);
    };
  }, []);

  useLayoutEffect(() => {
    const dialog = panel.current;
    if (!dialog) return;
    if (typeof dialog.showModal === 'function') {
      const close = () => {
        if (!dialog.open) dismiss.current();
      };
      dialog.addEventListener('close', close);
      dialog.showModal();
      return () => {
        dialog.removeEventListener('close', close);
        dialog.close();
      };
    }

    const previous = document.activeElement as HTMLElement | null;
    const siblings: Array<[HTMLElement, boolean]> = [];
    let ancestor: HTMLElement = dialog;
    while (ancestor.parentElement) {
      for (const child of ancestor.parentElement.children) {
        if (
          child !== ancestor &&
          child instanceof HTMLElement &&
          !child.matches('[data-modal-backdrop]')
        ) {
          siblings.push([child, child.inert]);
          child.inert = true;
        }
      }
      ancestor = ancestor.parentElement;
    }
    dialog.setAttribute('open', '');
    dialog.setAttribute('role', 'dialog');
    dialog.setAttribute('aria-modal', 'true');
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') dismiss.current();
      if (event.key !== 'Tab') return;
      const controls = focusable(dialog);
      const first = controls[0];
      const last = controls[controls.length - 1];
      if (!first) {
        event.preventDefault();
        dialog.focus();
      } else if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    dialog.addEventListener('keydown', keydown);
    (focusable(dialog)[0] ?? dialog).focus();
    return () => {
      dialog.removeEventListener('keydown', keydown);
      siblings.forEach(([element, inert]) => {
        element.inert = inert;
      });
      previous?.focus();
    };
  }, []);

  return (
    <dialog
      {...props}
      ref={panel}
      tabIndex={-1}
      className={cn('pax-modal-panel text-pax-light', className)}
      onCancel={(event) => {
        event.preventDefault();
        dismiss.current();
      }}
      onClick={(event) => {
        if (event.target !== event.currentTarget) return;
        const rect = event.currentTarget.getBoundingClientRect();
        if (
          event.clientX < rect.left ||
          event.clientX > rect.right ||
          event.clientY < rect.top ||
          event.clientY > rect.bottom
        )
          dismiss.current();
      }}
    >
      {children}
    </dialog>
  );
}

export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  variant = 'dialog',
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  variant?: 'dialog' | 'sheet';
}) {
  const titleId = useId();
  const descriptionId = useId();
  const [portal, setPortal] = useState<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const node = document.createElement('div');
    node.dataset.paxportModalRoot = 'true';
    document.body.appendChild(node);
    setPortal(node);
    return () => {
      node.remove();
      setPortal(null);
    };
  }, [open]);

  if (!open || !portal) return null;
  return createPortal(
    <div className="fixed inset-0 z-[100] flex items-end justify-center bg-pax-scrim p-0 sm:items-center sm:p-4">
      <ModalPanel
        onDismiss={onClose}
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        tabIndex={-1}
        className={cn(
          'w-full overflow-y-auto rounded-t-3xl bg-pax-overlay p-5 shadow-sheet',
          variant === 'sheet'
            ? 'rounded-t-3xl sm:max-w-lg sm:rounded-3xl'
            : 'max-w-lg rounded-t-3xl sm:rounded-3xl'
        )}
      >
        <div className="pax-dialog-heading mb-4 flex items-start justify-between gap-4">
          <div>
            <h2 id={titleId} className="text-lg font-semibold">
              {title}
            </h2>
            {description && (
              <p id={descriptionId} className="mt-1 text-sm text-pax-muted">
                {description}
              </p>
            )}
          </div>
          <Button
            variant="quiet"
            className="shrink-0 p-2"
            aria-label="Close dialog"
            onClick={onClose}
          >
            <X aria-hidden="true" className="h-5 w-5" />
          </Button>
        </div>
        {children}
      </ModalPanel>
    </div>,
    portal
  );
}

export const Sheet = Dialog;

export function Menu({ label, children }: { label: string; children: ReactNode }) {
  const id = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [native, setNative] = useState(false);

  useEffect(() => {
    setNative('showPopover' in HTMLElement.prototype);
  }, []);
  useEffect(() => {
    const panel = menu.current;
    if (!panel || !native) return;
    panel.setAttribute('popover', 'auto');
    const toggle = () => setOpen(panel.matches(':popover-open'));
    panel.addEventListener('toggle', toggle);
    return () => {
      panel.removeEventListener('toggle', toggle);
      panel.removeAttribute('popover');
    };
  }, [native]);
  useEffect(() => {
    const panel = menu.current;
    const button = trigger.current;
    if (!panel || !button) return;
    if (native) {
      if (open && !panel.matches(':popover-open')) panel.showPopover();
      else if (!open && panel.matches(':popover-open')) panel.hidePopover();
    }
    if (!open) return;
    const gap =
      parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--space-2')) *
      parseFloat(getComputedStyle(document.documentElement).fontSize);
    const rect = button.getBoundingClientRect();
    const bounds = panel.getBoundingClientRect();
    panel.style.top = `${Math.max(gap, Math.min(rect.bottom + gap, window.innerHeight - bounds.height - gap))}px`;
    panel.style.left = `${Math.max(gap, Math.min(rect.right - bounds.width, window.innerWidth - bounds.width - gap))}px`;
    const controls = focusable(panel);
    controls.forEach((control) => {
      if (!control.hasAttribute('role')) control.setAttribute('role', 'menuitem');
      control.tabIndex = -1;
    });
    controls[0]?.focus();
    const outside = (event: PointerEvent) => {
      if (!panel.contains(event.target as Node) && !button.contains(event.target as Node))
        setOpen(false);
    };
    if (!native) document.addEventListener('pointerdown', outside);
    return () => document.removeEventListener('pointerdown', outside);
  }, [open, native]);

  const close = () => {
    setOpen(false);
    trigger.current?.focus();
  };
  return (
    <div className="relative">
      <button
        ref={trigger}
        type="button"
        aria-haspopup="menu"
        aria-controls={id}
        aria-expanded={open}
        className="inline-flex items-center gap-2 rounded-xl bg-pax-control px-4 py-2 text-sm font-semibold"
        onClick={() => setOpen(!open)}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown') {
            event.preventDefault();
            setOpen(true);
          }
        }}
      >
        {label}
        <ChevronDown aria-hidden="true" className="h-4 w-4" />
      </button>
      <div
        ref={menu}
        id={id}
        role="menu"
        aria-label={`${label} options`}
        hidden={!native && !open}
        className="pax-menu fixed z-50 m-0 min-w-48 max-w-full rounded-xl bg-pax-overlay p-1 text-pax-light shadow-menu"
        onClick={(event) => {
          if ((event.target as HTMLElement).closest('button, a')) close();
        }}
        onKeyDown={(event) => {
          const controls = focusable(event.currentTarget);
          const index = controls.indexOf(document.activeElement as HTMLElement);
          const next =
            event.key === 'Home'
              ? 0
              : event.key === 'End'
                ? controls.length - 1
                : event.key === 'ArrowDown'
                  ? (index + 1) % controls.length
                  : event.key === 'ArrowUp'
                    ? (index - 1 + controls.length) % controls.length
                    : null;
          if (next !== null) {
            event.preventDefault();
            controls[next]?.focus();
          } else if (event.key === 'Escape') {
            event.preventDefault();
            close();
          } else if (event.key === 'Tab') close();
        }}
      >
        {children}
      </div>
    </div>
  );
}

export function ToastRegion({ children }: { children: ReactNode }) {
  return (
    <div
      aria-live="polite"
      aria-relevant="additions"
      className="pointer-events-none fixed inset-x-3 top-[calc(var(--safe-area-top)+var(--space-3))] z-[110] mx-auto max-w-md space-y-2"
    >
      {children}
    </div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return (
    <span aria-hidden="true" className={cn('shimmer block rounded-lg bg-pax-control', className)} />
  );
}

export function ErrorState({
  title,
  message,
  onRetry,
}: {
  title: string;
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div
      role="alert"
      aria-live="assertive"
      className="rounded-2xl bg-pax-error-surface p-4 text-pax-error"
    >
      <AlertTriangle aria-hidden="true" className="mb-3 h-5 w-5" />
      <h2 className="font-semibold">{title}</h2>
      <p className="mt-1 text-sm text-pax-error">{message}</p>
      {onRetry && (
        <Button variant="danger" className="mt-4" onClick={onRetry}>
          Retry
        </Button>
      )}
    </div>
  );
}

export function ApprovalSummary({
  rows,
}: {
  rows: readonly { label: string; value: ReactNode; emphasis?: boolean }[];
}) {
  return (
    <dl className="space-y-2 rounded-2xl bg-pax-surface p-4">
      {rows.map((row) => (
        <div
          key={row.label}
          className="flex flex-wrap items-start justify-between gap-2 rounded-xl bg-pax-card px-3 py-2.5"
        >
          <dt className="basis-32 flex-1 text-sm text-pax-muted">{row.label}</dt>
          <dd
            className={cn(
              'max-w-full flex-1 basis-32 break-words text-right text-sm',
              row.emphasis && 'font-semibold text-pax-light'
            )}
          >
            {row.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}
