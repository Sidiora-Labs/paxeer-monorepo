'use client';

import { AnimatePresence, motion } from 'framer-motion';
import { Copy, Plus, ShieldCheck } from 'lucide-react';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import Image from 'next/image';
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
  const dialogRef = useRef<HTMLDivElement>(null);
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

  // Focus trap and Escape key handling
  useEffect(() => {
    if (!isOpen) return;
    const el = dialogRef.current;
    if (!el) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        handleClose();
        return;
      }
      if (e.key !== 'Tab') return;
      const focusable = el.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    // Focus the first focusable element
    requestAnimationFrame(() => {
      const focusable = el.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      focusable[0]?.focus();
    });

    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, handleClose]);

  return (
    <>
      <AnimatePresence>
        {isOpen && (
          <motion.div
            onClick={handleClose}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="bg-pax-scrim backdrop-blur-xs fixed inset-0 z-10 h-full w-full"
          />
        )}
      </AnimatePresence>
      <motion.div
        layout
        style={{ transformOrigin: '50% 50% 0px' }}
        className="font-sans relative z-20 w-full max-w-[430px] overflow-hidden rounded-3xl"
      >
        <motion.div
          style={{ pointerEvents: !isOpen ? 'all' : 'none' }}
          className="relative z-10 flex w-full items-center justify-center"
        >
          <motion.button
            onClick={handleOpen}
            whileTap={{ scale: disabled ? 1 : 0.9 }}
            transition={LOGO_SPRING}
            layoutId="send-confirm-btn"
            disabled={disabled}
            className="h-10 w-full max-w-[300px] transform-none rounded-full bg-pax-accent text-pax-on-accent disabled:opacity-40"
          >
            <motion.span>{triggerLabel}</motion.span>
          </motion.button>
        </motion.div>
        <AnimatePresence mode="popLayout">
          {isOpen && (
            <motion.div
              ref={dialogRef}
              layout
              role="dialog"
              aria-modal="true"
              aria-label="Confirm send transaction"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ type: 'spring', stiffness: 550 / SPEED, damping: 45, mass: 0.7 }}
              style={{ transformOrigin: '50% 50% 0px' }}
              className="bg-pax-bg relative flex flex-col justify-end rounded-3xl p-4"
            >
              <div className="flex w-full items-center justify-between">
                <div className="flex items-center justify-center gap-2 text-xl font-medium">
                  <div className="flex size-10 items-center justify-center rounded-full bg-pax-accent/10">
                    <ShieldCheck className="size-6 text-pax-accent" />
                  </div>
                  {p.confirmSend}
                </div>
                <button type="button" aria-label="Close" onClick={handleClose}>
                  <Plus className="size-6 rotate-45 text-pax-accent" />
                </button>
              </div>
              {token && amount && to ? (
                <div className="my-4 space-y-2 rounded-2xl bg-pax-control p-4">
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-pax-muted">Token</span>
                    <div className="flex items-center gap-2">
                      {token.iconUrl && (
                        <Image
                          src={token.iconUrl}
                          alt={token.symbol}
                          className="size-4 rounded-full"
                          width={16}
                          height={16}
                        />
                      )}
                      <span className="text-sm font-semibold">{token.symbol}</span>
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-pax-muted">Amount</span>
                    <span className="text-sm font-bold">
                      {amount} {token.symbol}
                    </span>
                  </div>
                  <div className="flex flex-col gap-1">
                    <span className="text-sm text-pax-muted">To</span>
                    <div className="flex items-center gap-2">
                      <code className="text-xs font-mono text-pax-light break-all">{to}</code>
                      <button
                        type="button"
                        onClick={handleCopyAddress}
                        aria-label="Copy address"
                        className="min-h-[var(--touch-target)] shrink-0 text-pax-muted hover:text-pax-light"
                      >
                        <Copy className="size-3.5" />
                      </button>
                      {copied && <span className="text-xs text-pax-accent">Copied</span>}
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-pax-muted">Network</span>
                    <span className="text-xs text-pax-light">
                      {networkName} ({chainId})
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-pax-muted">{p.maximumFee}</span>
                    <span className="text-xs text-pax-light">
                      {fee === 'unavailable' ? (
                        <span data-role="fee-unavailable">Unavailable</span>
                      ) : fee ? (
                        `${fee.amountPax} PAX`
                      ) : (
                        p.estimating
                      )}
                    </span>
                  </div>
                  {fee && fee !== 'unavailable' && (
                    <>
                      <div className="flex items-center justify-between">
                        <span className="text-sm text-pax-muted">{p.feeMode}</span>
                        <span className="text-xs capitalize text-pax-light">
                          {fee.mode} · {fee.source}
                        </span>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-sm text-pax-muted">{p.nonce}</span>
                        <span className="text-xs text-pax-light">{fee.nonce ?? p.automatic}</span>
                      </div>
                    </>
                  )}
                </div>
              ) : (
                <p className="text-pax-muted my-5">Confirm the transaction to proceed.</p>
              )}
              <div className="flex items-center justify-end gap-2">
                <button
                  type="button"
                  onClick={handleClose}
                  className="min-h-[var(--touch-target)] bg-pax-control h-10 w-full rounded-full text-sm"
                >
                  {t.common.cancel}
                </button>
                <motion.button
                  onClick={handleConfirm}
                  disabled={loading}
                  whileTap={{ scale: loading ? 1 : 0.9 }}
                  transition={LOGO_SPRING}
                  layoutId="send-confirm-btn"
                  className="h-10 w-full max-w-[300px] transform-none rounded-full bg-pax-accent text-pax-on-accent text-sm disabled:opacity-50"
                >
                  <motion.span>{loading ? t.tx.submitting : p.confirmSend}</motion.span>
                </motion.button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </motion.div>
    </>
  );
};

const SPEED = 1;

const LOGO_SPRING = {
  type: 'spring',
  stiffness: 350 / SPEED,
  damping: 35,
} as const;
