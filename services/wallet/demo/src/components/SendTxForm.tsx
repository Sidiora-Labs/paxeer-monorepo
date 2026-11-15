'use client';

import { useEffect, useRef, useState } from 'react';
import type { PaxeerWallet } from '@paxeer/wallet';
import { isAddress, nativeToWei } from '@/lib/format';
import { FormField } from './FormField';
import { WalletForm } from './WalletForm';

interface SendTxFormProps {
  paxeer: PaxeerWallet;
  explorerUrl?: string | null;
  onBack: () => void;
}

type Status =
  | { kind: 'idle' }
  | { kind: 'sending' }
  | { kind: 'sent'; hash: string }
  | { kind: 'error'; message: string };

export function SendTxForm({ paxeer, explorerUrl, onBack }: SendTxFormProps) {
  const [to, setTo] = useState('');
  const [amount, setAmount] = useState('');
  const [status, setStatus] = useState<Status>({ kind: 'idle' });
  const resultHeading = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    if (status.kind === 'sent') resultHeading.current?.focus();
  }, [status.kind]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      status.kind === 'sending' ||
      !event.currentTarget.reportValidity() ||
      !isAddress(to) ||
      !isPositiveAmount(amount)
    )
      return;
    setStatus({ kind: 'sending' });
    try {
      const value = nativeToWei(amount);
      const res = await paxeer.sendTransaction({ to: to as `0x${string}`, value });
      setStatus({ kind: 'sent', hash: res.tx_hash });
    } catch (err) {
      setStatus({
        kind: 'error',
        message: err instanceof Error ? err.message : 'transaction failed',
      });
    }
  }

  if (status.kind === 'sent') {
    const explorer = explorerUrl ? `${explorerUrl.replace(/\/$/, '')}/tx/${status.hash}` : null;
    return (
      <div className="outcome">
        <div className="outcome-icon success">
          <svg
            className="icon"
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            aria-hidden="true"
          >
            <path
              d="M5 12l5 5L20 7"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </div>
        <div className="stack">
          <h3 ref={resultHeading} tabIndex={-1}>
            Transaction submitted
          </h3>
          <p>Broadcast to HyperPaxeer. Confirmation in seconds.</p>
        </div>
        <code className="transaction-hash">{status.hash}</code>
        <div className="stack full-width">
          {explorer && (
            <a href={explorer} target="_blank" rel="noreferrer" className="button">
              View on explorer
            </a>
          )}
          <button type="button" onClick={onBack} className="button primary">
            Done
          </button>
        </div>
      </div>
    );
  }

  return (
    <WalletForm
      onSubmit={submit}
      className="form"
      aria-label="Send Transaction"
      aria-busy={status.kind === 'sending'}
    >
      <button type="button" onClick={onBack} className="button quiet">
        <svg
          className="icon small"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          aria-hidden="true"
        >
          <path
            d="M15 18l-6-6 6-6"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        Back
      </button>
      <FormField
        label="Send to"
        name="to"
        value={to}
        onValueChange={(value) => setTo(value.trim())}
        placeholder="0x…"
        dataInput
        pattern="0x[a-fA-F0-9]{40}"
        disabled={status.kind === 'sending'}
        errorMessage="Not a valid 0x address."
      />
      <FormField
        label="Amount (PAX)"
        name="amount"
        value={amount}
        onValueChange={setAmount}
        placeholder="0.0"
        dataInput
        numeric
        inputMode="decimal"
        validate={isPositiveAmount}
        disabled={status.kind === 'sending'}
        errorMessage="Enter an amount greater than zero with at most 18 decimal places."
      />
      {status.kind === 'error' && (
        <div role="alert" className="error notice">
          {status.message}
        </div>
      )}
      <button type="submit" disabled={status.kind === 'sending'} className="button primary">
        {status.kind === 'sending' ? 'Signing & broadcasting…' : 'Send Transaction'}
      </button>
    </WalletForm>
  );
}

function isPositiveAmount(value: string): boolean {
  return /^\d+(\.\d{1,18})?$/.test(value.trim()) && BigInt(nativeToWei(value)) > 0n;
}
