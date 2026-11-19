'use client';

import { useEffect, useState } from 'react';
import type { PaxeerWallet } from '@paxeer/wallet';
import { paxeerWallet } from '@/lib/paxeer';
import { WalletModal } from './WalletModal';
import { unifiedOrigin } from './FundedAccountPanel';

export function ConnectButton() {
  const [client, setClient] = useState<PaxeerWallet | null>(null);
  const [origin, setOrigin] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    try {
      setOrigin(unifiedOrigin());
      setClient(paxeerWallet());
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Wallet configuration unavailable');
    }
  }, []);

  return (
    <div className="connect-control">
      <button
        type="button"
        aria-haspopup="dialog"
        aria-expanded={open}
        data-wallet-connect
        onClick={() => setOpen(true)}
        disabled={!client}
        className="button primary"
      >
        {error ? 'Wallet unavailable' : client ? 'Connect wallet' : 'Loading…'}
      </button>
      {error && (
        <p role="status" className="small-copy error">
          {error}
        </p>
      )}
      {client && <WalletModal open={open} onOpenChange={setOpen} paxeer={client} origin={origin} />}
    </div>
  );
}
