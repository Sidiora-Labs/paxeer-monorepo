'use client';

import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { Settings2 } from 'lucide-react';
import { createPortal } from 'react-dom';
import { usePopoverPosition } from '@/ui/primitives/use-popover';
import { surfaceToken } from '@/core/app-foundation/surface-token';
import { Button, IconButton, SegmentedControl } from '@/ui/primitives';
import { safeFixed, shortAddress } from '@/utils/format';
import { parseAmount } from '@/widgets/trading/transaction-controls/domain';
import { cn } from '@/lib/utils';

export const SLIPPAGE_PRESETS = [50, 100, 300] as const;
export const MAX_SLIPPAGE_BPS = 5000;
export const HIGH_SLIPPAGE_BPS = 500;

type PresetValue = '50' | '100' | '300' | 'custom';

export interface TradeSettingsPopoverProps {
  slippageBps: number;
  onSlippageChange: (bps: number) => void;
  /** Execution deadline enforced by the reviewed router flow, in seconds after review. */
  deadlineSeconds?: number;
  /** Verified spender the approval targets. */
  spender?: string | null;
  size?: 'sm' | 'md';
  className?: string;
}

export function formatSlippage(bps: number): string {
  return `${safeFixed(bps / 100, bps % 100 === 0 ? 1 : 2)}%`;
}

export function TradeSettingsPopover({ slippageBps, onSlippageChange, deadlineSeconds, spender, size = 'sm', className }: TradeSettingsPopoverProps) {
  const [open, setOpen] = useState(false);
  const [custom, setCustom] = useState('');
  const [customError, setCustomError] = useState<string | null>(null);
  const root = useRef<HTMLDivElement>(null);
  const panel = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLElement | null>(null);
  const [offset, setOffset] = useState<number>();
  const position = usePopoverPosition(root, panel, open, 'end', offset);
  const positioned = position !== null;
  const titleId = useId();
  const customId = useId();
  const customErrorId = useId();

  const preset: PresetValue = SLIPPAGE_PRESETS.includes(slippageBps as (typeof SLIPPAGE_PRESETS)[number]) ? (String(slippageBps) as PresetValue) : 'custom';
  const highSlippage = slippageBps > HIGH_SLIPPAGE_BPS;

  useEffect(() => { setOffset(surfaceToken('--sl-space-2')); }, []);

  useEffect(() => {
    if (!open || !positioned) return;
    panel.current?.querySelector<HTMLElement>('[data-autofocus]')?.focus({ preventScroll: true });
    const onPointer = (event: PointerEvent) => {
      const bounds = panel.current?.getBoundingClientRect();
      const outside = bounds && (event.clientX < bounds.left || event.clientX > bounds.right || event.clientY < bounds.top || event.clientY > bounds.bottom);
      if (outside || (root.current && !root.current.contains(event.target as Node) && !panel.current?.contains(event.target as Node))) setOpen(false);
    };
    document.addEventListener('pointerdown', onPointer);
    return () => {
      document.removeEventListener('pointerdown', onPointer);
      if (document.activeElement === document.body || panel.current?.contains(document.activeElement)) opener.current?.focus();
    };
  }, [open, positioned]);

  useEffect(() => {
    if (!open) { setCustom(''); setCustomError(null); }
  }, [open]);

  const applyCustom = () => {
    const parsed = parseAmount(custom.trim(), 2);
    if (parsed.errorCode || parsed.parsedRaw === undefined || parsed.parsedRaw < 1n || parsed.parsedRaw > BigInt(MAX_SLIPPAGE_BPS)) {
      setCustomError('Enter a slippage between 0.01% and 50%');
      return;
    }
    setCustomError(null);
    onSlippageChange(Number(parsed.parsedRaw));
    setCustom('');
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDialogElement>) => {
    if (event.key === 'Escape') { event.preventDefault(); setOpen(false); return; }
  };

  return (
    <div ref={root} className={cn('relative', className)}>
      <IconButton
        label={`Trade settings, slippage ${formatSlippage(slippageBps)}`}
        icon={<Settings2 />}
        variant="raised"
        size={size}
        pressed={open}
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={event => { opener.current = event.currentTarget; setOpen(value => !value); }}
      />
      {open ? createPortal(
        <dialog
          ref={panel}
          onCancel={event => { event.preventDefault(); setOpen(false); }}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          onKeyDown={onKeyDown}
          style={{ top: position?.top ?? 0, left: position?.left ?? 0, visibility: position ? 'visible' : 'hidden' }}
          className="sl-popover fixed m-0 w-max border-0 text-text z-[var(--sl-z-dropdown)] max-w-[min(var(--sl-modal-width-sm),calc(100dvw-var(--sl-space-6)))] rounded-md sl-floating p-3"
        >
          <div className="flex items-center justify-between">
            <h3 id={titleId} className="text-sm font-bold text-text">Settings</h3>
            <span className="text-xs text-text-muted tabular-nums">{formatSlippage(slippageBps)}</span>
          </div>

          <div className="mt-3 space-y-2">
            <p className="text-xs text-text-muted">Max slippage</p>
            <SegmentedControl<PresetValue>
              label="Slippage preset"
              size="sm"
              shape="sm"
              fullWidth
              className="[&_button]:min-w-min [&_button]:whitespace-nowrap"
              value={preset}
              onChange={value => { if (value !== 'custom') onSlippageChange(Number(value)); }}
              options={[
                { value: '50', label: '0.5%' },
                { value: '100', label: '1%' },
                { value: '300', label: '3%' },
                { value: 'custom', label: preset === 'custom' ? formatSlippage(slippageBps) : 'Custom', disabled: preset !== 'custom' },
              ]}
            />
            <div className="flex items-center gap-2">
              <label htmlFor={customId} className="sr-only">Custom slippage percent</label>
              <input
                id={customId}
                data-autofocus
                type="text"
                inputMode="decimal"
                maxLength={80}
                value={custom}
                aria-invalid={customError ? true : undefined}
                aria-describedby={customError ? customErrorId : undefined}
                onChange={event => { setCustom(event.target.value); if (customError) setCustomError(null); }}
                onKeyDown={event => {
                  if (event.nativeEvent.isComposing || event.keyCode === 229) return;
                  if (event.key === 'Enter') { event.preventDefault(); applyCustom(); }
                }}
                placeholder="Custom %"
                className={cn('h-control-height-sm min-w-0 flex-1 rounded-sm border border-control-edge px-2 text-sm text-text tabular-nums placeholder:text-text-faint outline-none focus-visible:outline-2 focus-visible:outline-focus', customError ? 'bg-danger-soft' : 'bg-surface-2')}
              />
              <Button size="sm" variant="secondary" onClick={applyCustom} disabled={custom.trim() === ''}>Apply</Button>
            </div>
            {customError ? <p id={customErrorId} role="alert" className="text-xs text-danger">{customError}</p> : null}
            {highSlippage ? <p role="status" className="rounded-sm bg-warning-soft px-2 py-1.5 text-xs text-warning">High slippage. You&apos;ll be asked to confirm before each trade.</p> : null}
          </div>

          <dl className="mt-3 space-y-1.5 rounded-sm bg-surface-2 p-2 text-xs">
            {deadlineSeconds !== undefined ? (
              <div className="flex items-center justify-between gap-3">
                <dt className="text-text-muted">Trade expires</dt>
                <dd className="text-text tabular-nums">{deadlineSeconds}s after you confirm</dd>
              </div>
            ) : null}
            <div className="flex items-center justify-between gap-3">
              <dt className="text-text-muted">Network fee</dt>
              <dd className="text-text">Set in your wallet</dd>
            </div>
            {spender ? (
              <div className="flex items-center justify-between gap-3">
                <dt className="text-text-muted">Approved contract</dt>
                <dd className="font-mono-sl text-text" title={spender}>{shortAddress(spender)}</dd>
              </div>
            ) : null}
          </dl>
          <p className="mt-2 text-2xs text-text-faint">Saved on this device.</p>
        </dialog>,
        document.body
      ) : null}
    </div>
  );
}
