'use client';

/**
 * Slippage-tolerance picker rendered when the swap widget enters `settings`
 * view. Quick presets at 0.25% / 0.5% / 1% / 2%, plus a custom-bps field.
 *
 * `slippageBps` is in basis points (1bp = 0.01%). Anything above 300 bps
 * (3%) gets a warning banner.
 */

import { AlertTriangle } from 'lucide-react';
import { SvgIcon } from '@/components/ui/SvgIcon';

const PRESETS_BPS = [25, 50, 100, 200] as const;
const HIGH_SLIP_THRESHOLD_BPS = 300;

export interface SwapSettingsViewProps {
  slippageBps: number;
  customSlippage: string;
  onChangeSlippageBps: (bps: number) => void;
  onChangeCustomSlippage: (raw: string) => void;
  onClose: () => void;
}

export function SwapSettingsView({
  slippageBps,
  customSlippage,
  onChangeSlippageBps,
  onChangeCustomSlippage,
  onClose,
}: SwapSettingsViewProps) {
  return (
    <div className="px-4 pt-4 pb-24">
      <div className="flex items-center justify-between mb-6">
        <h2 className="text-lg font-bold">Swap Settings</h2>
        <button
          type="button"
          onClick={onClose}
          aria-label="Close swap settings"
          className="min-h-[var(--touch-target)] min-w-[var(--touch-target)] p-2 rounded-full bg-pax-control press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          <SvgIcon name="x" className="w-4 h-4" />
        </button>
      </div>

      <div className="glass-card p-4 space-y-4">
        <p className="text-sm font-medium">Slippage Tolerance</p>
        <div className="flex gap-2">
          {PRESETS_BPS.map((bps) => (
            <button
              type="button"
              key={bps}
              onClick={() => {
                onChangeSlippageBps(bps);
                onChangeCustomSlippage('');
              }}
              className={`min-h-[var(--touch-target)] flex-1 py-2 rounded-xl text-xs font-medium transition-all press-scale focus-visible:ring-2 focus-visible:ring-pax-accent ${
                slippageBps === bps && !customSlippage
                  ? 'bg-pax-accent/15 text-pax-accent'
                  : 'bg-pax-control text-pax-muted'
              }`}
            >
              {bps / 100}%
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2">
          <label htmlFor="swap-custom-slippage" className="sr-only">
            Custom slippage percentage
          </label>
          <input
            id="swap-custom-slippage"
            type="text"
            inputMode="decimal"
            name="custom-slippage"
            autoComplete="off"
            aria-label="Custom slippage percentage"
            value={customSlippage}
            onChange={(e) => {
              onChangeCustomSlippage(e.target.value);
              const v = parseFloat(e.target.value);
              if (!isNaN(v) && v > 0 && v < 50) onChangeSlippageBps(Math.round(v * 100));
            }}
            placeholder="Custom %"
            className="flex-1 px-3 py-2.5 rounded-xl bg-pax-control   text-sm outline-none  placeholder:text-pax-muted"
          />
        </div>
        {slippageBps > HIGH_SLIP_THRESHOLD_BPS && (
          <div className="flex items-start gap-2">
            <AlertTriangle className="w-3.5 h-3.5 text-pax-warning mt-0.5 shrink-0" />
            <p className="text-xs text-pax-warning/80">
              High slippage may result in unfavorable rates
            </p>
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={onClose}
        className="min-h-[var(--touch-target)] w-full mt-6 py-3 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
      >
        Done
      </button>
    </div>
  );
}
