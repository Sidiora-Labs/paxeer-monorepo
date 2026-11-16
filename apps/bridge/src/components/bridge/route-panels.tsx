'use client';

import { ChainList, type ChainOption } from '@/components/trading/chain-selector';
import { ChainIcon, TokenIcon } from '@/components/trading/token-icon';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { InputGroup, InputGroupAddon, InputGroupInput, InputGroupText } from '@/components/ui/input-group';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Skeleton } from '@/components/ui/skeleton';
import { formatAmountDisplay } from '@/lib/bridge/amount';
import { cn } from '@/lib/utils';
import { ArrowUpDownIcon } from 'lucide-react';
import type { ReactNode } from 'react';

export type RouteOption = {

  /** "paxeer" or a foreign chain key. */
  key: string;
  name: string;
  hint?: string;
  disabled?: boolean;
};

function RouteTrigger({
  option,
  label,
  onClick,
}: {
  option: RouteOption;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={ onClick }
      aria-haspopup="dialog"
      aria-label={ `${ label }: ${ option.name }. Change` }
      className="focus-ring grid min-h-11 w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-2 gap-y-1 rounded-lg px-2 py-1.5 text-left transition-colors duration-(--duration-fast) hover:bg-container-high"
    >
      <ChainIcon name={ option.name } size="md" className="route-chain-icon"/>
      <span className="min-w-0 text-sm font-medium">{ option.name }</span>
      <span aria-hidden="true" className="ml-auto shrink-0 text-xs text-muted-foreground">
        ▾
      </span>
      { option.hint ? <span className="col-span-3 min-w-0 text-caption text-muted-foreground">{ option.hint }</span> : null }
    </button>
  );
}

export function RoutePanel({
  label,
  option,
  options,
  onSelect,
  pickerTitle,
  children,
}: {
  label: string;
  option: RouteOption;
  options: Array<RouteOption>;
  onSelect: (key: string) => void;
  pickerTitle: string;
  children: ReactNode;
}) {
  return (
    <section aria-label={ label } className="@container/route rounded-2xl border border-border bg-container surface-inset">
      <div className="mb-2 flex items-center justify-between px-1">
        <span className="text-label font-medium tracking-wide text-muted-foreground uppercase">{ label }</span>
      </div>
      <RouteTriggerWithPicker
        label={ label }
        option={ option }
        options={ options }
        onSelect={ onSelect }
        pickerTitle={ pickerTitle }
      />
      <div className="mt-3">{ children }</div>
    </section>
  );
}

function RouteTriggerWithPicker({
  label,
  option,
  options,
  onSelect,
  pickerTitle,
}: {
  label: string;
  option: RouteOption;
  options: Array<RouteOption>;
  onSelect: (key: string) => void;
  pickerTitle: string;
}) {
  const chainOptions: Array<ChainOption> = options.map((o) => ({ id: o.key, name: o.name, disabled: o.disabled }));
  return (
    <DialogTriggerRoute
      label={ label }
      option={ option }
      pickerTitle={ pickerTitle }
      dialog={ (
        <ChainList
          chains={ chainOptions }
          value={ option.key }
          variant="wide"
          onSelect={ (chain) => onSelect(chain.id) }
        />
      ) }
    />
  );
}

import { useEffect, useState } from 'react';

function DialogTriggerRoute({
  label,
  option,
  pickerTitle,
  dialog,
}: {
  label: string;
  option: RouteOption;
  pickerTitle: string;
  dialog: ReactNode;
}) {
  const [ open, setOpen ] = useState(false);
  useEffect(() => {
    const pageElements = [ ...document.querySelectorAll<HTMLElement>('header, main, footer, .skip-link') ];
    pageElements.forEach((element) => {
      if (open) element.inert = true;
      else element.inert = false;
    });
    return () => pageElements.forEach((element) => {
      element.inert = false;
    });
  }, [ open ]);
  return (
    <>
      <RouteTrigger option={ option } label={ label } onClick={ () => setOpen(true) }/>
      <Dialog open={ open } onOpenChange={ (o) => setOpen(o) }>
        <DialogContent className="gap-4 surface-inset sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="text-title-sm">{ pickerTitle }</DialogTitle>
            <DialogDescription>Every route runs through the attested PaxeerX bridge.</DialogDescription>
          </DialogHeader>
          <div
            onClick={ (event) => {
              if ((event.target as HTMLElement).closest('button')) setOpen(false);
            } }
          >
            { dialog }
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

export function SwapDirectionButton({ onClick, disabled }: { onClick: () => void; disabled?: boolean }) {
  return (
    <div className="relative z-10 -my-3 flex justify-center">
      <Button
        variant="protected"
        size="icon-sm"
        onClick={ onClick }
        disabled={ disabled }
        aria-label="Swap bridge direction"
        className="border border-border bg-card shadow-float"
      >
        <ArrowUpDownIcon/>
      </Button>
    </div>
  );
}

export function AmountInput({
  id,
  value,
  onChange,
  symbol,
  balance,
  decimals,
  balanceLoading,
  invalid,
  onBlur,
  disabled,
  onMax,
}: {
  id: string;
  value: string;
  onChange: (next: string) => void;
  symbol: string;
  balance?: bigint;
  decimals: number;
  balanceLoading?: boolean;
  invalid?: boolean;
  onBlur?: () => void;
  disabled?: boolean;
  onMax?: () => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={ id } className="text-label font-medium">Amount</label>
      <InputGroup className={ cn('min-h-14 flex-wrap rounded-xl bg-background', invalid && 'border-destructive') }>
        <InputGroupInput
          id={ id }
          name={ id }
          value={ value }
          onChange={ (e) => onChange(e.target.value) }
          inputMode="decimal"
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="none"
          spellCheck={ false }
          placeholder="0.0"
          aria-invalid={ Boolean(invalid) }
          aria-describedby={ invalid ? 'bridge-amount-error' : undefined }
          onBlur={ onBlur }
          disabled={ disabled }
          className="font-mono text-title tabular-nums"
        />
        <InputGroupAddon align="inline-end" className="ml-auto min-w-0 max-w-full flex-wrap">
          <span className="flex items-center gap-2 rounded-full bg-container-high py-1 pr-3 pl-1.5">
            <TokenIcon symbol={ symbol } size="sm"/>
            <InputGroupText className="font-medium text-foreground">{ symbol }</InputGroupText>
          </span>
          { onMax ? (
            <Button variant="tonal" size="xs" onClick={ onMax } disabled={ disabled || balance === undefined }>
              Max
            </Button>
          ) : null }
        </InputGroupAddon>
      </InputGroup>
      <div className="flex flex-wrap items-center justify-between gap-2 px-1 text-caption text-muted-foreground">
        <span>Available</span>
        { balanceLoading ? (
          <Skeleton className="h-3.5 w-20"/>
        ) : balance !== undefined ? (
          <span className="font-mono tabular-nums">
            { formatAmountDisplay(balance, decimals) } { symbol }
          </span>
        ) : (
          <span>—</span>
        ) }
      </div>
    </div>
  );
}

export function ReceivePreview({
  symbol,
  amountText,
  denom,
}: {
  symbol: string;
  amountText?: string;
  denom?: string;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="flex min-h-14 flex-wrap items-center gap-3 rounded-xl border border-dashed border-border bg-background/60 px-3.5">
        <span className="min-w-0 flex-[1_1_8ch] break-all font-mono text-title tabular-nums text-muted-foreground">
          { amountText || '0.0' }
        </span>
        <span className="flex shrink-0 items-center gap-2 rounded-full bg-container-high py-1 pr-3 pl-1.5 text-sm font-medium">
          <TokenIcon symbol={ symbol } size="sm"/>
          { symbol }
        </span>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2 px-1 text-caption text-muted-foreground">
        <span>You receive 1:1 — the bridge takes no fee.</span>
        { denom ? (
          <Badge variant="tonal" className="max-w-44 truncate font-mono text-caption" title={ denom }>
            { denom }
          </Badge>
        ) : null }
      </div>
    </div>
  );
}

export function AssetPicker({
  symbols,
  value,
  onChange,
  label,
}: {
  symbols: Array<string>;
  value: string;
  onChange: (symbol: string) => void;
  label: string;
}) {
  if (symbols.length <= 1) return null;
  return (
    <Popover>
      <PopoverTrigger
        render={ (
          <Button variant="tonal" size="sm" aria-label={ `${ label }: ${ value }` }>
            <TokenIcon symbol={ value } size="xs"/> { value } ▾
          </Button>
        ) }
      />
      <PopoverContent align="start" className="w-48 p-1.5">
        <div aria-label={ label } className="flex flex-col">
          { symbols.map((symbol) => (
            <Button
              key={ symbol }
              variant="ghost"
              size="sm"
              aria-pressed={ symbol === value }
              className="justify-start"
              onClick={ () => onChange(symbol) }
            >
              <TokenIcon symbol={ symbol } size="xs"/> { symbol }
              { symbol === value ? <span aria-hidden="true" className="ml-auto size-1.5 rounded-full bg-primary"/> : null }
            </Button>
          )) }
        </div>
      </PopoverContent>
    </Popover>
  );
}
