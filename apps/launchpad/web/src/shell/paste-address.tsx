'use client';

import { useCallback, useId, useRef, useState, type FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { ClipboardPaste } from 'lucide-react';
import { toast } from 'sonner';
import { Button, IconButton } from '@/ui/primitives';
import { ModalDialog } from '@/core/app-foundation/modal-dialog';

const ADDRESS = /(?<![0-9a-fA-F])0x[0-9a-fA-F]{40}(?![0-9a-fA-F])/;

/** First 0x address found in free text (a bare address, an explorer URL or a coin page URL), lowercased. */
export function addressFromText(text: string): string | null {
  const match = ADDRESS.exec(text);
  return match ? match[0].toLowerCase() : null;
}

/** Coin page for a pool or token address. */
export function coinPath(address: string): string {
  const normalized = addressFromText(address);
  if (!normalized || normalized.length !== 42) throw new Error('Invalid address');
  return `/coin/${normalized}`;
}

export default function PasteAddress() {
  const router = useRouter();
  const [fallbackOpen, setFallbackOpen] = useState(false);
  const [manual, setManual] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [reading, setReading] = useState(false);
  const anchorRef = useRef<HTMLDivElement | null>(null);
  const inputId = useId();
  const close = useCallback(() => {
    setFallbackOpen(false);
    anchorRef.current?.querySelector('button')?.focus();
  }, []);

  const go = (address: string) => {
    router.push(coinPath(address));
    setFallbackOpen(false);
    setManual('');
    setError(null);
  };

  const paste = async () => {
    if (reading) return;
    setReading(true);
    try {
      const text = await navigator.clipboard.readText();
      const address = addressFromText(text);
      if (address) go(address);
      else toast('No coin address on your clipboard');
    } catch {
      setFallbackOpen(true);
    } finally {
      setReading(false);
    }
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const address = addressFromText(manual);
    if (!address) {
      setError('Enter a valid coin address (0x...)');
      return;
    }
    go(address);
  };

  return (
    <div ref={anchorRef} className="relative flex shrink-0">
      <IconButton
        label="Open a coin from a pasted address"
        icon={<ClipboardPaste />}
        variant="ghost"
        size="md"
        loading={reading}
        aria-haspopup="dialog"
        aria-expanded={fallbackOpen}
        onClick={() => {
          void paste();
        }}
      />
      <ModalDialog
        open={fallbackOpen}
        onClose={close}
        aria-label="Open a coin by address"
        className="w-72 max-w-[calc(100dvw-var(--sl-space-6))] rounded-md bg-surface-2 p-3 shadow-pop"
      >
        <form onSubmit={submit} className="flex w-full min-w-0 flex-col gap-2">
          <label htmlFor={inputId} className="text-xs font-bold text-text-muted">
            Paste the coin address here
          </label>
          <input
            id={inputId}
            value={manual}
            onChange={(event) => {
              setManual(event.target.value);
              setError(null);
            }}
            autoFocus
            data-autofocus
            autoComplete="off"
            spellCheck={false}
            inputMode="text"
            placeholder="0x…"
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? `${inputId}-error` : undefined}
            className="sl-field min-h-control-height-md py-2 w-full rounded-sm bg-surface-3 px-3 font-mono text-sm text-text placeholder:text-text-faint"
          />
          {error ? (
            <p id={`${inputId}-error`} role="alert" className="text-xs text-danger">
              {error}
            </p>
          ) : null}
          <div className="flex flex-wrap justify-end gap-2">
            <Button type="button" variant="ghost" size="sm" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" size="sm">
              Open coin
            </Button>
          </div>
        </form>
      </ModalDialog>
    </div>
  );
}
