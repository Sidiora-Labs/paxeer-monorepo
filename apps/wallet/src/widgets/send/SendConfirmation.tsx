'use client';

import { Dialog, Button, ApprovalSummary } from '@/components/ui/primitives';
import { Copy } from 'lucide-react';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useWallet, WalletNotReadyError } from '@/wallet/WalletProvider';
import { useLocale } from '@/providers/LocaleProvider';

export interface SendConfirmationProps {
  triggerLabel?: string;
  token?: { symbol: string; iconUrl?: string | null };
  amount?: string;
  to?: string;
  networkName?: string;
  chainId?: number;
  loading?: boolean;
  onConfirm?: () => Promise<void> | void;
  onOpen?: () => boolean;
  disabled?: boolean;
  fee?:
    | {
        amountPax: string;
        mode: string;
        nonce: number | null;
        source: string;
      }
    | 'unavailable'
    | null;
}

export const SendConfirmation = ({
  triggerLabel = 'Send',
  token,
  amount,
  to,
  networkName = 'Paxeer Network',
  chainId: requestedChain,
  loading = false,
  onConfirm,
  onOpen,
  disabled = false,
  fee = null,
}: SendConfirmationProps) => {
  const { p, t } = useLocale();
  const wallet = useWallet();
  const chainId = wallet.chainId;
  const snapshotRef = useRef<{
    epoch: number;
    address: string;
    amount: string | undefined;
    to: string | undefined;
    chain: number;
  } | null>(null);
  const current = () => {
    const snapshot = snapshotRef.current;
    return (
      snapshot !== null &&
      wallet.isWriteCurrent(snapshot.epoch, snapshot.address, snapshot.chain) &&
      wallet.status === 'ready' &&
      snapshot.epoch === wallet.writeEpoch &&
      snapshot.address === wallet.address &&
      snapshot.chain === wallet.chainId &&
      snapshot.amount === amount &&
      snapshot.to === to &&
      (requestedChain === undefined || requestedChain === snapshot.chain)
    );
  };
  const [isOpen, setIsOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (isOpen && !current()) {
      snapshotRef.current = null;
      setIsOpen(false);
    }
  }, [
    isOpen,
    wallet.status,
    wallet.writeEpoch,
    wallet.address,
    wallet.chainId,
    amount,
    to,
    requestedChain,
  ]);

  const handleConfirm = async () => {
    if (!current()) {
      snapshotRef.current = null;
      setIsOpen(false);
      throw new WalletNotReadyError('confirm send');
    }
    if (onConfirm) {
      await onConfirm();
    }
    setIsOpen(false);
  };

  const handleOpen = useCallback(() => {
    if (
      disabled ||
      wallet.status !== 'ready' ||
      !wallet.address ||
      wallet.chainId === null ||
      (requestedChain !== undefined && requestedChain !== wallet.chainId)
    )
      return;
    if (onOpen && !onOpen()) return;
    snapshotRef.current = {
      epoch: wallet.writeEpoch,
      address: wallet.address,
      chain: wallet.chainId,
      amount,
      to,
    };
    previousFocusRef.current = document.activeElement as HTMLElement;
    setIsOpen(true);
  }, [
    disabled,
    onOpen,
    wallet.status,
    wallet.address,
    wallet.chainId,
    wallet.writeEpoch,
    amount,
    to,
    requestedChain,
  ]);

  const handleClose = useCallback(() => {
    setIsOpen(false);
    previousFocusRef.current?.focus();
  }, []);

  const handleCopyAddress = useCallback(async () => {
    if (!to) return;
    try {
      await navigator.clipboard.writeText(to);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard API may be unavailable
    }
  }, [to]);

  const rows = [
    { label: 'Token', value: token?.symbol ?? 'PAX' },
    { label: 'Amount', value: `${amount ?? '—'} ${token?.symbol ?? 'PAX'}`, emphasis: true },
    { label: 'To', value: to ?? '—' },
    { label: 'Network', value: `${networkName} (${chainId})` },
    {
      label: p.maximumFee,
      value: fee === 'unavailable' ? 'Unavailable' : fee ? `${fee.amountPax} PAX` : p.estimating,
    },
    {
      label: p.feeMode,
      value:
        fee === 'unavailable' ? 'Unavailable' : fee ? `${fee.mode} · ${fee.source}` : p.estimating,
    },
    {
      label: p.nonce,
      value:
        fee === 'unavailable'
          ? 'Unavailable'
          : fee
            ? String(fee.nonce ?? p.automatic)
            : p.estimating,
    },
  ];

  return (
    <>
      <Button className="w-full max-w-sm" onClick={handleOpen} disabled={disabled}>
        {triggerLabel}
      </Button>
      <Dialog open={isOpen} onClose={handleClose} title={p.confirmSend}>
        <div className="space-y-4">
          <ApprovalSummary rows={rows} />
          <Button variant="quiet" onClick={handleCopyAddress} aria-label="Copy address">
            <Copy className="h-4 w-4" aria-hidden="true" />
            {copied ? 'Copied' : 'Copy address'}
          </Button>
          <div className="grid grid-cols-2 gap-3">
            <Button variant="secondary" onClick={handleClose}>
              {t.common.cancel}
            </Button>
            <Button onClick={handleConfirm} disabled={loading}>
              {loading ? t.tx.submitting : p.confirmSend}
            </Button>
          </div>
        </div>
      </Dialog>
    </>
  );
};
