'use client';

/**
 * Send-form input fields: token selector, recipient, amount, percentage.
 *
 * Stateless — all values flow in/out via props so the orchestrator owns the
 * form state machine.
 */

import { ScanLine } from 'lucide-react';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { TokenIcon } from './TokenIcon';
import type { SendableToken } from './useSendableTokens';
import type { Contact } from '@/lib/contacts';

export interface SendFormProps {
  selectedToken: SendableToken | null;
  to: string;
  amount: string;
  resolvedContact?: Contact;
  hasContacts: boolean;
  onOpenTokenSelector: () => void;
  onOpenContactPicker: () => void;
  onOpenScanner: () => void;
  onChangeTo: (value: string) => void;
  onChangeAmount: (value: string) => void;
  onApplyPercentage: (pct: number) => void;
}

const PERCENTAGES = [25, 50, 75, 100] as const;

export function SendForm({
  selectedToken,
  to,
  amount,
  resolvedContact,
  hasContacts,
  onOpenTokenSelector,
  onOpenContactPicker,
  onOpenScanner,
  onChangeTo,
  onChangeAmount,
  onApplyPercentage,
}: SendFormProps) {
  return (
    <div className="flex-1 flex flex-col gap-4">
      {/* Token Selector */}
      <div>
        <label className="text-xs text-pax-muted mb-1.5 block">Token</label>
        <button
          type="button"
          onClick={onOpenTokenSelector}
          aria-haspopup="dialog"
          className="min-h-[var(--touch-target)] w-full flex items-center gap-3 px-4 py-3 rounded-xl bg-pax-control press-scale transition-colors hover:bg-pax-overlay focus-visible:ring-2 focus-visible:ring-pax-accent"
        >
          {selectedToken ? (
            <>
              <TokenIcon token={selectedToken} />
              <div className="flex-1 text-left min-w-0">
                <p className="text-sm font-medium">{selectedToken.symbol}</p>
                <p className="text-size-11 text-pax-muted truncate">{selectedToken.name}</p>
              </div>
              <div className="text-right mr-1">
                <p className="text-xs text-pax-muted">{selectedToken.balance}</p>
              </div>
              <SvgIcon name="chevron-down" className="w-4 h-4" />
            </>
          ) : (
            <span className="text-sm text-pax-muted">Select token...</span>
          )}
        </button>
      </div>

      {/* Recipient */}
      <div>
        <div className="flex flex-wrap gap-2 items-center justify-between mb-1.5">
          <label htmlFor="send-recipient" className="text-xs text-pax-muted">
            Recipient Address
          </label>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={onOpenScanner}
              className="min-h-[var(--touch-target)] flex items-center gap-1 text-size-11 text-pax-accent press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
            >
              <ScanLine className="w-3 h-3" />
              Scan QR
            </button>
            {hasContacts && (
              <button
                type="button"
                onClick={onOpenContactPicker}
                className="min-h-[var(--touch-target)] flex items-center gap-1 text-size-11 text-pax-accent press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
              >
                <SvgIcon name="user" className="w-3 h-3" />
                Contacts
              </button>
            )}
          </div>
        </div>
        <input
          id="send-recipient"
          type="text"
          name="recipient"
          autoComplete="off"
          aria-label="Recipient address"
          value={to}
          onChange={(e) => onChangeTo(e.target.value)}
          placeholder="0x..."
          className="w-full px-4 py-3 rounded-xl bg-pax-control   text-sm outline-none  transition-colors placeholder:text-pax-muted"
        />
        {resolvedContact && (
          <p className="text-size-11 text-pax-accent mt-1 px-1">
            Sending to: {resolvedContact.name}
          </p>
        )}
      </div>

      {/* Amount */}
      <div>
        <label htmlFor="send-amount" className="text-xs text-pax-muted mb-1.5 block">
          Amount
        </label>
        <div className="pax-filled-field relative rounded-xl">
          <input
            id="send-amount"
            type="text"
            name="amount"
            inputMode="decimal"
            autoComplete="off"
            aria-label="Amount"
            value={amount}
            onChange={(e) => onChangeAmount(e.target.value)}
            placeholder="0.00"
            className="w-full px-4 py-3 pr-16 rounded-xl bg-pax-control   text-sm outline-none  transition-colors placeholder:text-pax-muted"
          />
          <span className="absolute right-4 top-1/2 -translate-y-1/2 text-xs text-pax-muted font-medium">
            {selectedToken?.symbol || 'PAX'}
          </span>
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 mt-2">
          {PERCENTAGES.map((pct) => (
            <button
              type="button"
              key={pct}
              onClick={() => onApplyPercentage(pct)}
              className="min-h-[var(--touch-target)] flex-1 py-1.5 rounded-lg bg-pax-control text-size-11 font-medium text-pax-muted hover:bg-pax-overlay hover:text-pax-light transition-all press-scale focus-visible:ring-2 focus-visible:ring-pax-accent"
            >
              {pct === 100 ? 'MAX' : `${pct}%`}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
