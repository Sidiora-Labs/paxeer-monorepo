'use client';

import {
  cloneElement,
  isValidElement,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type HTMLAttributes,
  type KeyboardEvent,
  type ReactElement,
  type ReactNode,
} from 'react';
import { createPortal } from 'react-dom';
import Link from 'next/link';
import { Check, ChevronDown, ChevronRight, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { IconButton } from './button';
import { surfaceToken } from '@/core/app-foundation/surface-token';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';
import { useDismiss, useMounted, usePopoverPosition } from './use-popover';

/* -------------------------------------------------------- DropdownMenu */

export interface MenuItem {
  id: string;
  label: ReactNode;
  onSelect?: () => void;
  href?: string;
  icon?: ReactNode;
  destructive?: boolean;
  disabled?: boolean;
  /** Defined => rendered as menuitemradio with a check when true. */
  selected?: boolean;
  /** Optional group label rendered above this item. */
  group?: string;
  /** Marks an item that opens a nested surface (chevron). */
  submenu?: boolean;
}

export interface DropdownMenuProps {
  items: MenuItem[];
  /** Accessible name of the trigger, e.g. "More actions for hdegroot". */
  label: string;
  /** Trigger body. Defaults to a caret-only trigger when omitted. */
  trigger?: ReactNode;
  /** Shows the trigger label visually (title caret style). */
  showLabel?: boolean;
  triggerClassName?: string;
  align?: 'start' | 'end';
  size?: 'sm' | 'md';
  menuLabel?: string;
  className?: string;
}

export function DropdownMenu({
  items,
  label,
  trigger,
  showLabel = false,
  triggerClassName,
  align = 'start',
  size = 'md',
  menuLabel,
  className,
}: DropdownMenuProps) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const itemRefs = useRef<Array<HTMLElement | null>>([]);
  const mounted = useMounted();
  const menuId = useId();
  const position = usePopoverPosition(triggerRef, menuRef, open, align);
  const close = useCallback(() => {
    setOpen(false);
  }, []);
  useDismiss(open, close, [triggerRef, menuRef]);

  const enabledIndexes = items.map((item, i) => (item.disabled ? -1 : i)).filter((i) => i >= 0);
  const firstEnabledRef = useRef(-1);
  firstEnabledRef.current = enabledIndexes[0] ?? -1;

  useEffect(() => {
    if (!open) {
      setActiveIndex(-1);
      return;
    }
    const first = firstEnabledRef.current;
    setActiveIndex(first);
    const frame = requestAnimationFrame(() => itemRefs.current[first]?.focus());
    return () => cancelAnimationFrame(frame);
  }, [open]);

  function focusItem(index: number) {
    setActiveIndex(index);
    itemRefs.current[index]?.focus();
  }

  function closeAndRestore() {
    setOpen(false);
    triggerRef.current?.focus();
  }

  function select(item: MenuItem) {
    if (item.disabled) return;
    item.onSelect?.();
    closeAndRestore();
  }

  function onMenuKey(event: KeyboardEvent<HTMLDivElement>) {
    if (enabledIndexes.length === 0) return;
    const pos = enabledIndexes.indexOf(activeIndex);
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      focusItem(enabledIndexes[(pos + 1) % enabledIndexes.length]);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      focusItem(enabledIndexes[(pos - 1 + enabledIndexes.length) % enabledIndexes.length]);
    } else if (event.key === 'Home') {
      event.preventDefault();
      focusItem(enabledIndexes[0]);
    } else if (event.key === 'End') {
      event.preventDefault();
      focusItem(enabledIndexes[enabledIndexes.length - 1]);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      closeAndRestore();
    } else if (event.key === 'Tab') {
      setOpen(false);
    } else if (event.key.length === 1 && /\S/.test(event.key)) {
      const needle = event.key.toLowerCase();
      const text = (item: MenuItem) =>
        (typeof item.label === 'string' ? item.label : '').toLowerCase();
      const after = enabledIndexes.find(
        (i) => i > activeIndex && text(items[i]).startsWith(needle)
      );
      const wrap = after ?? enabledIndexes.find((i) => text(items[i]).startsWith(needle));
      if (wrap !== undefined) focusItem(wrap);
    }
  }

  return (
    <div className={cn('inline-flex', className)}>
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        aria-label={showLabel ? undefined : label}
        title={showLabel ? undefined : label}
        onClick={() => setOpen((v) => !v)}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            setOpen(true);
          }
        }}
        className={cn(
          'inline-flex items-center justify-center gap-1.5 rounded-sm font-bold text-text-muted transition-colors duration-fast hover:bg-surface-hover hover:text-text',
          open && 'bg-surface-3 text-text',
          trigger
            ? size === 'sm'
              ? 'min-h-control-height-sm px-2 py-2 text-sm'
              : 'min-h-control-height-md px-3 py-2 text-base'
            : size === 'sm'
              ? 'size-control-height-sm'
              : 'size-control-height-md',
          triggerClassName
        )}
      >
        {showLabel ? <span>{label}</span> : null}
        {trigger ?? (
          <ChevronDown
            aria-hidden="true"
            className={cn('size-icon-md transition-transform duration-fast', open && 'rotate-180')}
          />
        )}
        {showLabel && trigger ? (
          <ChevronDown
            aria-hidden="true"
            className={cn('size-icon-sm transition-transform duration-fast', open && 'rotate-180')}
          />
        ) : null}
      </button>
      {mounted && open && position
        ? createPortal(
            <div
              ref={menuRef}
              id={menuId}
              role="menu"
              aria-label={menuLabel ?? label}
              onKeyDown={onMenuKey}
              style={{ top: position.top, left: position.left }}
              className="sl-popover fixed z-[var(--sl-z-dropdown)] min-w-[min(var(--sl-menu-min-width),calc(100dvw-var(--sl-space-6)))] max-w-[calc(100dvw-var(--sl-space-6))] max-h-[var(--sl-dialog-max-height)] overflow-y-auto rounded-md bg-surface-3 p-1 shadow-pop"
            >
              {items.map((item, index) => {
                const role = item.selected !== undefined ? 'menuitemradio' : 'menuitem';
                const common: HTMLAttributes<HTMLElement> & {
                  ref: (el: HTMLElement | null) => void;
                  tabIndex: number;
                } = {
                  ref: (el) => {
                    itemRefs.current[index] = el;
                  },
                  role,
                  tabIndex: index === activeIndex ? 0 : -1,
                  'aria-disabled': item.disabled || undefined,
                  'aria-checked': item.selected !== undefined ? item.selected : undefined,
                  onMouseEnter: () => !item.disabled && setActiveIndex(index),
                  className: cn(
                    'flex min-h-touch-target-min w-full py-2 items-center gap-2 rounded-sm px-3 text-left text-base transition-colors duration-fast',
                    index === activeIndex && !item.disabled && 'bg-surface-hover',
                    item.selected && 'bg-surface-active',
                    item.destructive ? 'text-danger' : 'text-text',
                    item.disabled && 'cursor-not-allowed text-text-disabled'
                  ),
                };
                const body = (
                  <>
                    {item.icon ? (
                      <span
                        aria-hidden="true"
                        className="inline-flex shrink-0 [&>svg]:size-icon-md"
                      >
                        {item.icon}
                      </span>
                    ) : null}
                    <span className="flex-1">{item.label}</span>
                    {item.selected ? (
                      <Check aria-hidden="true" className="size-icon-md text-accent" />
                    ) : null}
                    {item.submenu ? (
                      <ChevronRight aria-hidden="true" className="size-icon-md text-text-faint" />
                    ) : null}
                  </>
                );
                return (
                  <div key={item.id}>
                    {item.group && (index === 0 || items[index - 1].group !== item.group) ? (
                      <div
                        role="presentation"
                        className="px-3 pb-1 pt-2 text-2xs font-bold uppercase tracking-[var(--sl-tracking-label)] text-text-faint"
                      >
                        {item.group}
                      </div>
                    ) : null}
                    {item.href && !item.disabled ? (
                      <Link prefetch={false} href={item.href} {...common} onClick={() => closeAndRestore()}>
                        {body}
                      </Link>
                    ) : (
                      <button
                        type="button"
                        {...common}
                        disabled={item.disabled}
                        onClick={() => select(item)}
                      >
                        {body}
                      </button>
                    )}
                  </div>
                );
              })}
            </div>,
            triggerRef.current?.closest('dialog[open]') ?? document.body
          )
        : null}
    </div>
  );
}

/* ---------------------------------------------------------- ModalFrame */

export interface ModalFrameProps {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  subtitle?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
  /** Scrim click closes; confirm dialogs set false (Escape still closes). */
  closeOnScrim?: boolean;
  footer?: ReactNode;
  children: ReactNode;
  closeLabel?: string;
  /** Hides the visible title (it stays as the accessible name). */
  hideTitle?: boolean;
  className?: string;
}

const modalWidth = {
  sm: 'max-w-modal-width-sm',
  md: 'max-w-modal-width-md',
  lg: 'max-w-modal-width-lg',
} as const;

export function ModalFrame({
  open,
  onClose,
  title,
  subtitle,
  size = 'md',
  closeOnScrim = true,
  footer,
  children,
  closeLabel = 'Close dialog',
  hideTitle = false,
  className,
}: ModalFrameProps) {
  const titleId = useId();
  const subtitleId = useId();
  return (
    <ModalDialog
      open={open}
      onClose={onClose}
      closeOnScrim={closeOnScrim}
      aria-labelledby={titleId}
      aria-describedby={subtitle ? subtitleId : undefined}
      className={cn('flex-col rounded-lg bg-surface-2 shadow-modal', modalWidth[size], className)}
    >
      <div className="@container/modal-header flex shrink-0 flex-wrap items-start gap-3 px-4 pt-4">
        <div className="order-last min-w-0 flex-1 basis-full @xs/modal-header:order-none @xs/modal-header:basis-40">
          <h2
            id={titleId}
            className={cn('text-xl font-extrabold text-text', hideTitle && 'sr-only')}
          >
            {title}
          </h2>
          {subtitle ? (
            <p id={subtitleId} className="mt-1 text-sm text-text-muted">
              {subtitle}
            </p>
          ) : null}
        </div>
        <IconButton
          label={closeLabel}
          icon={<X />}
          size="sm"
          onClick={onClose}
          className="ml-auto -mr-2 -mt-1"
        />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">{children}</div>
      {footer ? (
        <div className="flex flex-wrap items-center justify-end gap-2 px-6 pb-6">{footer}</div>
      ) : null}
    </ModalDialog>
  );
}

/* ------------------------------------------------------------- Tooltip */

export interface TooltipProps {
  content: ReactNode;
  children: ReactElement<HTMLAttributes<HTMLElement>>;
  delay?: number;
  align?: 'start' | 'center' | 'end';
  className?: string;
}

export function Tooltip({ content, children, delay, align = 'center', className }: TooltipProps) {
  const [visible, setVisible] = useState(false);
  const anchorRef = useRef<HTMLSpanElement | null>(null);
  const tipRef = useRef<HTMLDivElement | null>(null);
  const timer = useRef<number | null>(null);
  const hovered = useRef(false);
  const focused = useRef(false);
  const mounted = useMounted();
  const id = useId();
  const position = usePopoverPosition(anchorRef, tipRef, visible, align, undefined, 'hint');

  const show = useCallback(
    (immediate: boolean) => {
      if (timer.current) window.clearTimeout(timer.current);
      if (immediate) setVisible(true);
      else
        timer.current = window.setTimeout(
          () => setVisible(true),
          delay ?? surfaceToken('--sl-duration-tooltip')
        );
    },
    [delay]
  );
  const hide = useCallback(() => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = null;
    setVisible(false);
  }, []);

  const deferHide = useCallback(() => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(hide, delay ?? surfaceToken('--sl-duration-tooltip'));
  }, [hide, delay]);

  useEffect(() => {
    if (!visible) return;
    function onKey(event: globalThis.KeyboardEvent) {
      if (event.key === 'Escape') hide();
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [visible, hide]);

  useEffect(
    () => () => {
      if (timer.current) window.clearTimeout(timer.current);
    },
    []
  );

  useDismiss(visible, hide, [anchorRef, tipRef]);
  const triggerName = children.props['aria-label'] ?? children.props.children;
  const repeatsName = typeof content === 'string' && content === triggerName;
  const child = isValidElement(children)
    ? cloneElement(children, {
        'aria-describedby': children.props['aria-describedby'] ?? (repeatsName ? undefined : id),
      })
    : children;

  return (
    <span
      ref={anchorRef}
      className={cn('inline-flex', className)}
      onMouseEnter={() => {
        hovered.current = true;
        show(false);
      }}
      onMouseLeave={() => {
        hovered.current = false;
        if (!focused.current) deferHide();
      }}
      onFocus={() => {
        focused.current = true;
        show(true);
      }}
      onBlur={() => {
        focused.current = false;
        if (!hovered.current) hide();
      }}
    >
      {child}
      {mounted
        ? createPortal(
            <div
              ref={tipRef}
              id={id}
              role="tooltip"
              hidden={!visible}
              onMouseEnter={() => {
                hovered.current = true;
                show(true);
              }}
              onMouseLeave={() => {
                hovered.current = false;
                if (!focused.current) deferHide();
              }}
              style={position ? { top: position.top, left: position.left } : undefined}
              className="sl-popover fixed z-[var(--sl-z-tooltip)] max-w-[min(var(--sl-notice-max-width),calc(100dvw-var(--sl-space-6)))] rounded-sm bg-surface-3 px-2 py-1 text-xs text-text shadow-pop"
            >
              {content}
            </div>,
            anchorRef.current?.closest('dialog[open]') ?? document.body
          )
        : null}
    </span>
  );
}
