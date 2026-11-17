'use client';

import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type ChangeEvent,
  type InputHTMLAttributes,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
  type TextareaHTMLAttributes,
} from 'react';
import {
  ChevronDown,
  ChevronsUpDown,
  ChevronUp,
  CircleAlert,
  LoaderCircle,
  Lock,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { formatCount } from '@/utils/format';

/* ---------------------------------------------------- SegmentedControl */

export interface SegmentOption<T extends string> {
  value: T;
  label: ReactNode;
  count?: number;
  icon?: ReactNode;
  disabled?: boolean;
  /** Accessible name; set it when `icon` only or when `label` is a short form of a longer name. */
  ariaLabel?: string;
  /** Hover text; defaults to `ariaLabel` so a shortened label still shows its full name. */
  title?: string;
}

export interface SegmentedControlProps<T extends string> {
  options: SegmentOption<T>[];
  value: T;
  onChange: (value: T) => void;
  label: string;
  size?: 'sm' | 'md';
  shape?: 'pill' | 'sm';
  /** Trade tone colours the selected segment accent (first option) or danger (second). */
  tone?: 'neutral' | 'trade';
  fullWidth?: boolean;
  /** Tighter segment padding (6 px a side) for narrow containers (dashboard column headers). */
  compact?: boolean;
  className?: string;
}

export function SegmentedControl<T extends string>({
  options,
  value,
  onChange,
  label,
  size = 'sm',
  shape = 'pill',
  tone = 'neutral',
  fullWidth = false,
  compact = false,
  className,
}: SegmentedControlProps<T>) {
  const refs = useRef<Array<HTMLButtonElement | null>>([]);

  function move(from: number, delta: number) {
    const enabled = options.map((o, i) => (o.disabled ? -1 : i)).filter((i) => i >= 0);
    if (enabled.length === 0) return;
    const pos = enabled.indexOf(from);
    const next = enabled[(pos + delta + enabled.length) % enabled.length];
    onChange(options[next].value);
    refs.current[next]?.focus();
  }

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
      event.preventDefault();
      move(index, 1);
    } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
      event.preventDefault();
      move(index, -1);
    } else if (event.key === 'Home') {
      event.preventDefault();
      move(-1, 1);
    } else if (event.key === 'End') {
      event.preventDefault();
      const enabled = options.map((o, i) => (o.disabled ? -1 : i)).filter((i) => i >= 0);
      const next = enabled[enabled.length - 1];
      if (next === undefined) return;
      onChange(options[next].value);
      refs.current[next]?.focus();
    }
  }

  const radius = shape === 'pill' ? 'rounded-pill' : 'rounded-sm';
  return (
    <div
      role="radiogroup"
      aria-label={label}
      className={cn(
        'inline-flex min-w-0 max-w-full flex-wrap bg-surface-2 p-0.5',
        radius,
        fullWidth && 'flex w-full',
        className
      )}
    >
      {options.map((option, index) => {
        const selected = option.value === value;
        const tradeTone =
          tone === 'trade'
            ? index === 0
              ? 'bg-accent text-on-accent'
              : 'bg-danger text-on-danger'
            : 'border-[var(--sl-control-edge)] bg-surface-active text-text';
        return (
          <button
            key={option.value}
            ref={(el) => {
              refs.current[index] = el;
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={option.ariaLabel}
            title={option.title ?? option.ariaLabel}
            tabIndex={selected ? 0 : -1}
            disabled={option.disabled}
            onClick={() => onChange(option.value)}
            onKeyDown={(event) => onKeyDown(event, index)}
            className={cn(
              'inline-flex items-center justify-center gap-1.5 border border-transparent py-2 text-sm font-bold transition-colors duration-fast',
              compact ? 'whitespace-nowrap px-1.5' : 'px-3',
              radius,
              size === 'sm' ? 'min-h-control-height-sm' : 'min-h-control-height-md',
              fullWidth && 'flex-1',
              selected
                ? `${tradeTone} underline underline-offset-[var(--sl-focus-offset)]`
                : 'text-text-muted hover:bg-surface-hover hover:text-text',
              option.disabled && 'cursor-not-allowed text-text-disabled hover:bg-transparent'
            )}
          >
            {option.icon ? (
              <span aria-hidden="true" className="inline-flex [&>svg]:size-icon-sm">
                {option.icon}
              </span>
            ) : null}
            {option.label}
            {option.count !== undefined ? (
              <span
                className={cn('text-xs tabular-nums', selected ? 'opacity-80' : 'text-text-faint')}
                data-numeric
              >
                {formatCount(option.count)}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

/* ------------------------------------------------------------ TabStrip */

export interface TabItem<T extends string> {
  id: T;
  label: ReactNode;
  count?: number;
  disabled?: boolean;
  /** Auth gated: shows a lock, the caller opens sign-in in onChange. */
  locked?: boolean;
}

export interface TabStripProps<T extends string> {
  tabs: TabItem<T>[];
  active: T;
  onChange: (id: T) => void;
  label: string;
  variant?: 'underline' | 'filled';
  /** Manual activation: arrow keys move focus, Enter/Space activate. Default automatic. */
  manual?: boolean;
  trailing?: ReactNode;
  /** Shared id prefix so tabpanels can point at tabs: `${idPrefix}-tab-${id}` / `${idPrefix}-panel-${id}`. */
  idPrefix?: string;
  className?: string;
}

export function tabIds(idPrefix: string, id: string) {
  return { tab: `${idPrefix}-tab-${id}`, panel: `${idPrefix}-panel-${id}` };
}

export function TabStrip<T extends string>({
  tabs,
  active,
  onChange,
  label,
  variant = 'underline',
  manual = false,
  trailing,
  idPrefix,
  className,
}: TabStripProps<T>) {
  const generated = useId();
  const prefix = idPrefix ?? `tabs${generated}`;
  const refs = useRef<Array<HTMLButtonElement | null>>([]);

  function focusIndex(index: number, activate: boolean) {
    const enabled = tabs.map((t, i) => (t.disabled ? -1 : i)).filter((i) => i >= 0);
    if (enabled.length === 0) return;
    const target = enabled.includes(index) ? index : enabled[0];
    refs.current[target]?.focus();
    refs.current[target]?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' });
    if (activate) onChange(tabs[target].id);
  }

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const enabled = tabs.map((t, i) => (t.disabled ? -1 : i)).filter((i) => i >= 0);
    const pos = enabled.indexOf(index);
    let next: number | null = null;
    if (event.key === 'ArrowRight') next = enabled[(pos + 1) % enabled.length];
    else if (event.key === 'ArrowLeft') next = enabled[(pos - 1 + enabled.length) % enabled.length];
    else if (event.key === 'Home') next = enabled[0];
    else if (event.key === 'End') next = enabled[enabled.length - 1];
    else if (manual && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault();
      onChange(tabs[index].id);
      return;
    }
    if (next !== null) {
      event.preventDefault();
      focusIndex(next, !manual);
    }
  }

  return (
    <div className={cn('flex flex-wrap items-center gap-2', className)}>
      <div
        role="tablist"
        aria-label={label}
        className={cn(
          'flex min-w-0 flex-1 basis-40 items-end overflow-x-auto p-1',
          variant === 'filled' && 'gap-1'
        )}
      >
        {tabs.map((tab, index) => {
          const selected = tab.id === active;
          const ids = tabIds(prefix, tab.id);
          return (
            <button
              key={tab.id}
              ref={(el) => {
                refs.current[index] = el;
              }}
              id={ids.tab}
              type="button"
              role="tab"
              aria-selected={selected}
              aria-controls={ids.panel}
              aria-disabled={tab.disabled || undefined}
              tabIndex={selected ? 0 : -1}
              disabled={tab.disabled}
              onClick={() => onChange(tab.id)}
              onKeyDown={(event) => onKeyDown(event, index)}
              className={cn(
                'relative inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap px-3 font-bold transition-colors duration-fast',
                variant === 'underline'
                  ? 'min-h-11 py-2 text-base'
                  : 'min-h-control-height-md border border-control-edge py-2 rounded-sm text-sm',
                selected
                  ? variant === 'underline'
                    ? 'text-text'
                    : 'bg-surface-3 text-text'
                  : 'text-text-muted hover:text-text',
                variant === 'filled' && !selected && 'hover:bg-surface-hover',
                tab.disabled && 'cursor-not-allowed text-text-disabled hover:text-text-disabled'
              )}
            >
              {tab.locked ? <Lock aria-hidden="true" className="size-icon-sm" /> : null}
              <span>{tab.label}</span>
              {tab.count !== undefined ? (
                <span
                  data-numeric
                  className="inline-flex h-5 items-center rounded-pill bg-surface-3 px-1.5 text-xs tabular-nums text-text-muted"
                >
                  {formatCount(tab.count)}
                </span>
              ) : null}
              {tab.locked ? <span className="sr-only">, sign in required</span> : null}
              {selected ? (
                <span
                  aria-hidden="true"
                  className="absolute inset-x-3 bottom-0 h-0.5 rounded-pill bg-accent"
                />
              ) : null}
            </button>
          );
        })}
      </div>
      {trailing ? <div className="max-w-full shrink-0">{trailing}</div> : null}
    </div>
  );
}

export function UnderlineTabs<T extends string>(props: Omit<TabStripProps<T>, 'variant'>) {
  return <TabStrip {...props} variant="underline" />;
}

/* ---------------------------------------------------------- SelectPill */

export interface SelectOption<T extends string> {
  value: T;
  label: ReactNode;
  /** Plain text used when the option is selected and shown in the trigger. */
  text?: string;
  disabled?: boolean;
  icon?: ReactNode;
}

export interface SelectPillProps<T extends string> {
  options: SelectOption<T>[];
  value: T | null;
  onChange: (value: T) => void;
  /** Accessible name of the control, also shown as the "label:" prefix when `showLabel`. */
  label: string;
  showLabel?: boolean;
  placeholder?: string;
  size?: 'sm' | 'md';
  disabled?: boolean;
  loading?: boolean;
  error?: string | null;
  /**
   * Value-only trigger for narrow rows (dashboard column headers): tighter padding and no chevron,
   * so the pill is just the selected value. The menu, keyboard handling and accessible name are unchanged,
   * and the title spells out `label: value`.
   */
  compact?: boolean;
  className?: string;
}

export function SelectPill<T extends string>({
  options,
  value,
  onChange,
  label,
  showLabel = false,
  placeholder = 'Select',
  size = 'sm',
  disabled = false,
  loading = false,
  error = null,
  compact = false,
  className,
}: SelectPillProps<T>) {
  const id = useId();
  const errorId = `${id}-error`;
  const selected = options.find((option) => option.value === value);
  return (
    <div className={cn('inline-flex min-w-0 max-w-full flex-col gap-1', className)}>
      <div
        className={cn(
          'sl-field relative inline-flex min-h-control-height-sm min-w-0 flex-wrap items-center gap-1.5 rounded-pill px-3 py-1',
          compact && 'px-2 py-0',
          error ? 'bg-danger-soft' : 'bg-surface-2'
        )}
      >
        {showLabel ? (
          <label htmlFor={id} className="text-sm text-text-faint">
            {label}:
          </label>
        ) : null}
        <select
          id={id}
          name={label}
          value={value ?? ''}
          aria-label={showLabel ? undefined : label}
          title={
            compact
              ? `${label}: ${selected?.text ?? (typeof selected?.label === 'string' ? selected.label : placeholder)}`
              : undefined
          }
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? errorId : undefined}
          aria-busy={loading || undefined}
          disabled={disabled || loading}
          onChange={(event) => {
            const option = options.find((item) => item.value === event.target.value);
            if (option && !option.disabled) onChange(option.value);
          }}
          className={cn(
            'sl-native-select min-w-[min(var(--sl-select-min-width),max(100%,var(--sl-touch-target-min)))] max-w-full flex-1 cursor-pointer rounded-sm bg-transparent py-1 text-md font-bold text-text disabled:cursor-not-allowed disabled:text-text-disabled',
            compact && 'min-w-0 flex-none appearance-none whitespace-nowrap px-0 py-0 text-sm [field-sizing:content] [&::picker-icon]:hidden',
            size === 'md' && 'min-h-control-height-md'
          )}
        >
          <option value="" disabled>
            {placeholder}
          </option>
          {options.length === 0 ? <option disabled>No matches</option> : null}
          {options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {option.text ?? option.label}
            </option>
          ))}
        </select>
        {loading ? (
          <LoaderCircle
            aria-hidden="true"
            className="size-icon-sm shrink-0 animate-spin text-text-faint"
          />
        ) : null}
      </div>
      {error ? (
        <span
          id={errorId}
          role="alert"
          className="inline-flex items-center gap-1 text-xs text-danger"
        >
          <CircleAlert aria-hidden="true" className="size-icon-sm shrink-0" />
          {error}
        </span>
      ) : null}
    </div>
  );
}

/* -------------------------------------------------------------- Toggle */

export interface ToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** Accessible name (or use `labelledBy`). */
  label?: string;
  labelledBy?: string;
  describedBy?: string;
  size?: 'sm' | 'md';
  disabled?: boolean;
  loading?: boolean;
  className?: string;
}

export function Toggle({
  checked,
  onChange,
  label,
  labelledBy,
  describedBy,
  size = 'md',
  disabled = false,
  loading = false,
  className,
}: ToggleProps) {
  return (
    <span
      className={cn(
        'relative inline-flex min-h-touch-target-min min-w-touch-target-min shrink-0 items-center justify-center',
        className
      )}
    >
      <input
        type="checkbox"
        role="switch"
        checked={checked}
        aria-label={label}
        aria-labelledby={labelledBy}
        aria-describedby={describedBy}
        aria-busy={loading || undefined}
        disabled={disabled || loading}
        onChange={(event) => onChange(event.target.checked)}
        className={cn(
          'peer absolute inset-0 m-auto h-touch-target-min w-full cursor-pointer appearance-none rounded-pill disabled:cursor-not-allowed',
          size === 'md' ? 'min-w-12' : 'min-w-11'
        )}
      />
      <span
        aria-hidden="true"
        className={cn(
          'sl-field pointer-events-none inline-flex items-center rounded-pill p-0.5 transition-colors duration-fast',
          size === 'md' ? 'h-6 w-10' : 'h-5 w-8',
          disabled ? 'bg-surface-3' : checked ? 'bg-accent' : 'bg-surface-active'
        )}
      >
        <span
          className={cn(
            'inline-flex items-center justify-center rounded-pill transition-transform duration-fast ease-sl',
            size === 'md' ? 'size-5' : 'size-4',
            disabled ? 'bg-text-disabled' : checked ? 'bg-on-accent' : 'bg-text-muted',
            checked && (size === 'md' ? 'translate-x-4' : 'translate-x-3')
          )}
        >
          {loading ? <LoaderCircle className="size-3 animate-spin text-text" /> : null}
        </span>
      </span>
    </span>
  );
}

/* -------------------------------------------------------------- Slider */

export interface SliderProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  label: string;
  /** Spoken unit for aria-valuetext, e.g. "%". */
  unit?: string;
  ticks?: number[];
  disabled?: boolean;
  outOfRange?: boolean;
  showValue?: boolean;
  className?: string;
}

export function Slider({
  value,
  onChange,
  min = 0,
  max = 100,
  step = 1,
  label,
  unit = '',
  ticks,
  disabled = false,
  outOfRange = false,
  showValue = true,
  className,
}: SliderProps) {
  const pct = max === min ? 0 : Math.max(0, Math.min(100, ((value - min) / (max - min)) * 100));
  return (
    <div className={cn('flex items-center gap-3', className)}>
      <div className="relative flex min-h-touch-target-min min-w-0 flex-1 items-center">
        <span
          aria-hidden="true"
          className={cn(
            'absolute inset-x-0 h-1 rounded-pill',
            disabled ? 'bg-surface-3' : 'bg-track'
          )}
        />
        <span
          aria-hidden="true"
          style={{ width: `${pct}%` }}
          className={cn(
            'absolute left-0 h-1 rounded-pill',
            disabled ? 'bg-surface-3' : outOfRange ? 'bg-warning' : 'bg-track-fill'
          )}
        />
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          disabled={disabled}
          aria-label={label}
          aria-valuetext={`${value}${unit}`}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange(Number(event.target.value))}
          onKeyDown={(event) => {
            if (event.shiftKey && (event.key === 'ArrowRight' || event.key === 'ArrowUp')) {
              event.preventDefault();
              onChange(Math.min(max, value + step * 10));
            }
            if (event.shiftKey && (event.key === 'ArrowLeft' || event.key === 'ArrowDown')) {
              event.preventDefault();
              onChange(Math.max(min, value - step * 10));
            }
          }}
          className={cn(
            'relative h-touch-target-min w-full cursor-pointer appearance-none bg-transparent',
            '[&::-webkit-slider-runnable-track]:bg-transparent [&::-moz-range-track]:bg-transparent',
            '[&::-webkit-slider-thumb]:size-4 [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-pill [&::-webkit-slider-thumb]:transition-transform',
            '[&::-moz-range-thumb]:size-4 [&::-moz-range-thumb]:rounded-pill [&::-moz-range-thumb]:border-0',
            'focus-visible:[&::-webkit-slider-thumb]:outline-2 focus-visible:[&::-webkit-slider-thumb]:outline-focus focus-visible:[&::-webkit-slider-thumb]:outline-offset-2',
            disabled
              ? 'cursor-not-allowed [&::-webkit-slider-thumb]:bg-text-disabled [&::-moz-range-thumb]:bg-text-disabled'
              : '[&::-webkit-slider-thumb]:bg-accent hover:[&::-webkit-slider-thumb]:scale-110 active:[&::-webkit-slider-thumb]:bg-accent-active [&::-moz-range-thumb]:bg-accent'
          )}
        />
        {ticks && ticks.length > 0 ? (
          <div aria-hidden="true" className="pointer-events-none absolute inset-x-0 top-full h-1.5">
            {ticks.map((tick) => (
              <span
                key={tick}
                style={{ left: `${max === min ? 0 : ((tick - min) / (max - min)) * 100}%` }}
                className="absolute h-1.5 w-0.5 -translate-x-1/2 rounded-pill bg-surface-active"
              />
            ))}
          </div>
        ) : null}
      </div>
      {showValue ? (
        <span
          data-numeric
          className={cn(
            'w-10 text-right text-xs tabular-nums',
            outOfRange ? 'text-warning' : 'text-text-muted'
          )}
        >
          {value}
          {unit}
        </span>
      ) : null}
    </div>
  );
}

/* ----------------------------------------------------------- FormField */

type InputProps = Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'id' | 'className' | 'aria-describedby' | 'aria-invalid'
>;
type TextareaProps = Omit<
  TextareaHTMLAttributes<HTMLTextAreaElement>,
  'id' | 'className' | 'aria-describedby' | 'aria-invalid'
>;

export interface FormFieldProps {
  id: string;
  label: string;
  optional?: boolean;
  helper?: ReactNode;
  error?: string | null;
  multiline?: boolean;
  size?: 'md' | 'lg';
  /** Surface the field sits on; the control takes the next step. */
  surface?: 'canvas' | 'surface-1' | 'surface-2';
  leading?: ReactNode;
  trailing?: ReactNode;
  counter?: { count: number; max: number };
  loading?: boolean;
  mono?: boolean;
  uppercase?: boolean;
  inputProps?: InputProps;
  textareaProps?: TextareaProps;
  className?: string;
}

const controlSurface = {
  canvas: 'bg-surface-1',
  'surface-1': 'bg-surface-2',
  'surface-2': 'bg-surface-3',
} as const;

export function FormField({
  id,
  label,
  optional = false,
  helper,
  error = null,
  multiline = false,
  size = 'md',
  surface = 'surface-1',
  leading,
  trailing,
  counter,
  loading = false,
  mono = false,
  uppercase = false,
  inputProps,
  textareaProps,
  className,
}: FormFieldProps) {
  const helperId = `${id}-helper`;
  const errorId = `${id}-error`;
  const counterId = `${id}-counter`;
  const describedBy =
    [error ? errorId : helper ? helperId : null, counter ? counterId : null]
      .filter(Boolean)
      .join(' ') || undefined;
  const disabled = Boolean(inputProps?.disabled ?? textareaProps?.disabled);
  const readOnly = Boolean(inputProps?.readOnly ?? textareaProps?.readOnly);
  const counterTone = counter
    ? counter.count >= counter.max
      ? 'text-danger'
      : counter.count >= counter.max * 0.9
        ? 'text-warning'
        : 'text-text-faint'
    : '';
  const shell = cn(
    'sl-field flex min-w-0 items-center gap-2 rounded-sm px-3 transition-colors duration-fast',
    error
      ? 'bg-danger-soft'
      : readOnly
        ? 'bg-transparent'
        : disabled
          ? 'bg-surface-2'
          : controlSurface[surface],
    'focus-within:outline-2 focus-within:outline-focus',
    multiline
      ? 'items-start py-2'
      : size === 'lg'
        ? 'min-h-control-height-lg py-2'
        : 'min-h-control-height-md py-2'
  );
  const control = cn(
    'min-w-0 flex-1 bg-transparent text-md text-text outline-none placeholder:text-text-faint',
    'disabled:text-text-disabled read-only:text-text-muted',
    mono && 'font-[family-name:var(--sl-font-mono)]',
    uppercase && 'uppercase',
    multiline && 'min-h-24 resize-y leading-[var(--sl-leading-base)]'
  );

  return (
    <div className={cn('flex min-w-0 flex-col gap-1.5', className)}>
      <label htmlFor={id} className="text-sm font-bold text-text">
        {label}
        {optional ? <span className="ml-1 inline-block font-semibold text-text-faint">(Optional)</span> : null}
      </label>
      <div className={shell}>
        {leading ? (
          <span
            aria-hidden="true"
            className="inline-flex shrink-0 text-text-faint [&>svg]:size-icon-md"
          >
            {leading}
          </span>
        ) : null}
        {multiline ? (
          <textarea
            id={id}
            name={id}
            aria-describedby={describedBy}
            aria-invalid={error ? true : undefined}
            className={control}
            {...textareaProps}
          />
        ) : (
          <input
            id={id}
            name={id}
            aria-describedby={describedBy}
            aria-invalid={error ? true : undefined}
            className={control}
            {...inputProps}
          />
        )}
        {loading ? (
          <LoaderCircle
            aria-hidden="true"
            className="size-icon-md shrink-0 animate-spin text-text-faint"
          />
        ) : null}
        {trailing ? <span className="inline-flex shrink-0 items-center">{trailing}</span> : null}
      </div>
      <div className="flex items-start justify-between gap-3">
        {error ? (
          <p
            id={errorId}
            role="alert"
            className="inline-flex items-center gap-1 text-xs text-danger"
          >
            <CircleAlert aria-hidden="true" className="size-icon-sm shrink-0" />
            {error}
          </p>
        ) : helper ? (
          <p id={helperId} className="text-xs text-text-faint">
            {helper}
          </p>
        ) : (
          <span />
        )}
        {counter ? (
          <span
            id={counterId}
            data-numeric
            aria-live={counter.count >= counter.max * 0.9 ? 'polite' : 'off'}
            className={cn('shrink-0 text-xs tabular-nums', counterTone)}
          >
            {counter.count}/{counter.max}
          </span>
        ) : null}
      </div>
    </div>
  );
}

/* ---------------------------------------------------------- SortHeader */

export type SortDirection = 'asc' | 'desc' | null;

export interface SortHeaderProps {
  label: ReactNode;
  /** Current direction for this column; null when another column (or none) is active. */
  sort?: SortDirection;
  /** Omit to render a static header. */
  onSort?: () => void;
  align?: 'left' | 'right' | 'center';
  width?: number | string;
  className?: string;
}

export function SortHeader({
  label,
  sort = null,
  onSort,
  align = 'left',
  width,
  className,
}: SortHeaderProps) {
  const alignClass =
    align === 'right'
      ? 'justify-end text-right'
      : align === 'center'
        ? 'justify-center text-center'
        : 'justify-start text-left';
  const ariaSort = sort === 'asc' ? 'ascending' : sort === 'desc' ? 'descending' : undefined;
  const content = (
    <span className={cn('inline-flex items-center gap-1', alignClass)}>
      <span>{label}</span>
      {onSort ? (
        sort === 'asc' ? (
          <ChevronUp aria-hidden="true" className="size-3 text-accent" />
        ) : sort === 'desc' ? (
          <ChevronDown aria-hidden="true" className="size-3 text-accent" />
        ) : (
          <ChevronsUpDown aria-hidden="true" className="size-3 opacity-60" />
        )
      ) : null}
    </span>
  );
  return (
    <th
      scope="col"
      aria-sort={ariaSort}
      style={width !== undefined ? { width } : undefined}
      className={cn(
        'min-h-table-header-height px-3 py-2 text-2xs font-bold uppercase tracking-[var(--sl-tracking-label)]',
        sort ? 'text-text' : 'text-text-faint',
        align === 'right' ? 'text-right' : align === 'center' ? 'text-center' : 'text-left',
        className
      )}
    >
      {onSort ? (
        <button
          type="button"
          onClick={onSort}
          className={cn(
            'flex h-full w-full scroll-mx-3 items-center rounded-xs uppercase tracking-[var(--sl-tracking-label)] transition-colors hover:text-text',
            alignClass
          )}
        >
          {content}
          <span className="sr-only">
            {sort === 'asc'
              ? ', sorted ascending'
              : sort === 'desc'
                ? ', sorted descending'
                : ', sortable'}
          </span>
        </button>
      ) : (
        <div className={cn('flex h-full items-center', alignClass)}>{content}</div>
      )}
    </th>
  );
}

/* -------------------------------------------------------- ResizeHandle */

export interface ResizeHandleProps {
  value: number;
  min: number;
  max: number;
  onChange: (value: number) => void;
  /** Double-click / Enter resets to this value. */
  defaultValue?: number;
  step?: number;
  label?: string;
  className?: string;
  /** 'horizontal' (default) is a row divider dragged up/down; 'vertical' is a column divider dragged left/right. */
  orientation?: 'horizontal' | 'vertical';
  /** The resized element sits after the handle (below / right of it): dragging toward it, or ArrowUp / ArrowLeft, grows the value. */
  inverted?: boolean;
}

export function ResizeHandle({
  value,
  min,
  max,
  onChange,
  defaultValue,
  step = 16,
  label = 'Resize chart',
  className,
  orientation = 'horizontal',
  inverted = false,
}: ResizeHandleProps) {
  const [dragging, setDragging] = useState(false);
  const start = useRef<{ at: number; value: number } | null>(null);
  const clamp = useCallback((v: number) => Math.max(min, Math.min(max, Math.round(v))), [min, max]);
  const vertical = orientation === 'vertical';
  const direction = inverted ? -1 : 1;
  const coordinate = (event: ReactPointerEvent<HTMLDivElement>) =>
    vertical ? event.clientX : event.clientY;

  function onPointerDown(event: ReactPointerEvent<HTMLDivElement>) {
    start.current = { at: coordinate(event), value };
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
  }
  function onPointerMove(event: ReactPointerEvent<HTMLDivElement>) {
    if (!start.current) return;
    onChange(clamp(start.current.value + direction * (coordinate(event) - start.current.at)));
  }
  function onPointerUp(event: ReactPointerEvent<HTMLDivElement>) {
    start.current = null;
    setDragging(false);
    if (event.currentTarget.hasPointerCapture(event.pointerId))
      event.currentTarget.releasePointerCapture(event.pointerId);
  }
  function onKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') {
      event.preventDefault();
      onChange(clamp(value - direction * step));
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowRight') {
      event.preventDefault();
      onChange(clamp(value + direction * step));
    } else if (event.key === 'Home') {
      event.preventDefault();
      onChange(min);
    } else if (event.key === 'End') {
      event.preventDefault();
      onChange(max);
    } else if (event.key === 'Enter' && defaultValue !== undefined) {
      event.preventDefault();
      onChange(clamp(defaultValue));
    }
  }

  return (
    <div
      role="separator"
      aria-orientation={orientation}
      aria-label={label}
      aria-valuenow={value}
      aria-valuemin={min}
      aria-valuemax={max}
      tabIndex={0}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onDoubleClick={() => defaultValue !== undefined && onChange(clamp(defaultValue))}
      onKeyDown={onKeyDown}
      data-dragging={dragging ? '' : undefined}
      className={cn(
        'group flex touch-none select-none items-center justify-center focus-visible:bg-surface-active focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-focus',
        vertical
          ? 'h-full min-w-touch-target-min cursor-col-resize'
          : 'min-h-touch-target-min w-full cursor-row-resize',
        className
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          'rounded-pill transition-colors duration-fast group-focus-visible:bg-accent',
          vertical ? 'h-8 w-1' : 'h-1 w-8',
          dragging ? 'bg-accent' : 'bg-surface-3 group-hover:bg-surface-hover'
        )}
      />
    </div>
  );
}
