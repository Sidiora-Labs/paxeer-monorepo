'use client';

import { KeyRound } from 'lucide-react';
import { cn } from '@/lib/utils';
import { FormField } from '@/ui/primitives';
import { InfoTip } from './info-tip';
import type { CreationDraftV2 } from './creation-storage';
import { OpticalPresetPicker, type OpticalRegistryState } from './optical-preset-picker';
import { QuotePairIndicator } from './quote-pair-indicator';
import { SocialLinksDisclosure } from './social-links-disclosure';
import { StrategyPicker, feeStrategyLabel } from './strategy-picker';
import { OptionRow } from '@/ui/primitives';

export const NAME_MAX_BYTES = 50;
export const SYMBOL_MAX_BYTES = 10;
export const DESCRIPTION_MAX_CHARS = 4096;

const encoder = new TextEncoder();
export const byteLength = (value: string) => encoder.encode(value).length;

const fieldCopy: Record<string, string> = {
  name: `Name must be 1 to ${NAME_MAX_BYTES} characters`,
  symbol: `Ticker must be 1 to ${SYMBOL_MAX_BYTES} characters`,
  description: `Keep the description under ${DESCRIPTION_MAX_CHARS.toLocaleString()} characters`,
  socialLinks: 'Add up to 5 links, each starting with https://',
};

export function fieldMessage(field: string, code: string | undefined): string | null {
  if (!code) return null;
  if (code === 'unsafe_text' && (field === 'name' || field === 'symbol')) return `${field === 'name' ? 'Name' : 'Ticker'} must not contain control or text direction characters`;
  return code.includes(' ') ? code : fieldCopy[field] ?? code;
}

export interface CreateFormFieldsProps {
  draft: CreationDraftV2;
  errors: Record<string, string>;
  locked: boolean;
  onEdit: <K extends keyof CreationDraftV2>(key: K, value: CreationDraftV2[K]) => void;
  registeredOpticals: readonly string[];
  registry: OpticalRegistryState;
  opticalReason: (presetId: string) => string | null;
}

/** Counts appear after typing to keep the empty fields quiet. */
export function InlineCounter({ id, count, max }: { id: string; count: number; max: number }) {
  if (count === 0) return null;
  const tone = count >= max ? 'text-danger' : count >= max * 0.9 ? 'text-warning' : 'text-text-faint';
  return (
    <span id={id} data-numeric aria-live={count >= max * 0.9 ? 'polite' : 'off'} className={cn('text-xs tabular-nums', tone)}>
      {count}/{max}
    </span>
  );
}

export function CreateFormFields({ draft, errors, locked, onEdit, registeredOpticals, registry, opticalReason }: CreateFormFieldsProps) {
  const descriptionCount = [...draft.description].length;
  return (
    <section aria-labelledby="create-details-title" className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <h2 id="create-details-title" className="text-base font-semibold text-text">Coin details</h2>
        <p className="text-xs text-text-faint">Choose carefully, the name and ticker can&apos;t be changed once the coin is created</p>
      </div>
      <div data-create-form-card className="flex flex-col gap-4 rounded-lg bg-surface-1 p-4">
        <div className="grid gap-4 sm:grid-cols-2 sm:gap-3">
          <FormField
            id="create-name"
            label="Coin name"
            error={fieldMessage('name', errors.name)}
            trailing={<InlineCounter id="create-name-count" count={byteLength(draft.name)} max={NAME_MAX_BYTES} />}
            inputProps={{ value: draft.name, disabled: locked, maxLength: NAME_MAX_BYTES, autoComplete: 'off', placeholder: 'Name your coin', onChange: event => onEdit('name', event.target.value) }}
          />
          <FormField
            id="create-symbol"
            label="Ticker"
            error={fieldMessage('symbol', errors.symbol)}
            trailing={<InlineCounter id="create-symbol-count" count={byteLength(draft.symbol)} max={SYMBOL_MAX_BYTES} />}
            inputProps={{ value: draft.symbol, disabled: locked, maxLength: SYMBOL_MAX_BYTES, autoComplete: 'off', spellCheck: false, placeholder: 'Add a coin ticker (e.g. PAXE)', onChange: event => onEdit('symbol', event.target.value) }}
          />
        </div>
        <FormField
          id="create-description"
          label="Description"
          optional
          multiline
          error={fieldMessage('description', errors.description)}
          trailing={<InlineCounter id="create-description-count" count={descriptionCount} max={DESCRIPTION_MAX_CHARS} />}
          textareaProps={{ value: draft.description, disabled: locked, rows: 4, placeholder: 'Write a short description', onChange: event => onEdit('description', event.target.value) }}
        />
        <SocialLinksDisclosure
          links={draft.socialLinks}
          onChange={links => onEdit('socialLinks', links)}
          disabled={locked}
          error={fieldMessage('socialLinks', errors.socialLinks)}
        />
        <OptionRow id="create-pair" label="Pair" choice="USDL">
          <QuotePairIndicator />
        </OptionRow>
        <CreatorRewardsOptions id="create-rewards" value={draft.feeStrategy} onChange={value => onEdit('feeStrategy', value)} disabled={locked} error={errors.feeStrategy} />
        <OptionRow id="create-rules" label="Trading rules" choice={draft.opticalSelection === 'standard' ? 'Standard' : draft.opticalSelection} error={errors.opticalSelection} failed={registry.state === 'error'}>
          <OpticalPresetPicker
            value={draft.opticalSelection}
            onChange={value => onEdit('opticalSelection', value)}
            registered={registeredOpticals}
            registry={registry}
            reasonFor={opticalReason}
            disabled={locked}
          />
        </OptionRow>
      </div>
    </section>
  );
}

export const FEE_RIGHTS_DETAIL = 'When you create a coin, you get a creator rewards pass (an NFT) in your wallet. Whoever holds it can claim the coin\'s trading fees, change where they go, or pass it on to someone else.';

/** Explains the rewards pass without adding another panel. */
export function FeeRightsNote() {
  return (
    <div className="flex flex-wrap items-start gap-3">
      <span aria-hidden="true" className="flex size-8 shrink-0 items-center justify-center rounded-pill bg-surface-3 text-text-muted">
        <KeyRound className="size-icon-md" />
      </span>
      <div className="min-w-0 flex-1 basis-40">
        <p className="flex flex-wrap items-center gap-1 text-sm font-semibold text-text">
          Creator rewards
          <InfoTip label="About creator rewards" content={FEE_RIGHTS_DETAIL} />
        </p>
        <p className="text-xs text-text-muted">Earn trading fees from your coin. You can change where they go from the coin page later.</p>
      </div>
    </div>
  );
}

export function CreatorRewardsOptions({ id, value, onChange, disabled, error }: {
  id: string;
  value: CreationDraftV2['feeStrategy'];
  onChange: (value: CreationDraftV2['feeStrategy']) => void;
  disabled?: boolean;
  error?: string;
}) {
  return (
    <OptionRow id={id} label="Creator rewards" choice={feeStrategyLabel(value)} error={error}>
      <FeeRightsNote />
      <StrategyPicker value={value} onChange={onChange} disabled={disabled} />
    </OptionRow>
  );
}
