'use client';

import { useState, useEffect } from 'react';
import { ModalPanel } from './primitives';
import { Fuel, Clock, AlertTriangle, Zap, X } from 'lucide-react';

// ── Backdrop + Drawer shell ──────────────────────────────────────────────────
function ModalShell({
  open,
  onClose,
  children,
}: {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return open ? (
    <div className="fixed inset-0 z-50 flex items-end justify-center">
      <div className="absolute inset-0 bg-pax-scrim" onClick={onClose} />
      <ModalPanel
        onDismiss={onClose}
        aria-label="Transaction"
        className="relative w-full max-w-md bg-pax-card rounded-t-3xl p-5 safe-area-pb"
      >
        {children}
      </ModalPanel>
    </div>
  ) : null;
}

// ── Insufficient Gas Modal ───────────────────────────────────────────────────
interface InsufficientGasModalProps {
  open: boolean;
  onClose: () => void;
  onBuyPax?: () => void;
  requiredGas?: string;
  availableBalance?: string;
}

export function InsufficientGasModal({
  open,
  onClose,
  onBuyPax,
  requiredGas,
  availableBalance,
}: InsufficientGasModalProps) {
  return (
    <ModalShell open={open} onClose={onClose}>
      <div className="flex flex-col items-center text-center gap-4 pt-4">
        <div className="w-14 h-14 rounded-2xl bg-pax-warning-muted flex items-center justify-center">
          <Fuel className="w-7 h-7 text-pax-warning" />
        </div>

        <div className="space-y-1">
          <h3 className="text-base font-bold">Insufficient Gas</h3>
          <p className="text-sm text-pax-muted leading-relaxed">
            You need PAX to pay network fees. Add some to your wallet to continue.
          </p>
        </div>

        {(requiredGas || availableBalance) && (
          <div className="w-full glass-card  list-separated">
            {requiredGas && (
              <div className="flex flex-wrap gap-2 justify-between px-4 py-3">
                <span className="basis-32 flex-1 text-sm text-pax-muted">Estimated fee</span>
                <span className="basis-32 flex-1 text-sm font-medium">{requiredGas} PAX</span>
              </div>
            )}
            {availableBalance && (
              <div className="flex flex-wrap gap-2 justify-between px-4 py-3">
                <span className="basis-32 flex-1 text-sm text-pax-muted">Your balance</span>
                <span className="basis-32 flex-1 text-sm font-medium text-pax-error">
                  {availableBalance} PAX
                </span>
              </div>
            )}
          </div>
        )}

        <div className="w-full flex flex-col gap-2.5 mt-1">
          {onBuyPax && (
            <button
              onClick={() => {
                onBuyPax();
                onClose();
              }}
              className="w-full flex items-center justify-center gap-2 py-3.5 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale"
            >
              <Zap className="w-4 h-4" />
              Buy PAX
            </button>
          )}
          <button
            onClick={onClose}
            className="w-full py-3 rounded-2xl bg-pax-control text-sm font-medium text-pax-muted press-scale"
          >
            Cancel
          </button>
        </div>
      </div>
    </ModalShell>
  );
}

// ── Transaction Timeout Modal ────────────────────────────────────────────────
interface TxTimeoutModalProps {
  open: boolean;
  onClose: () => void;
  onSpeedUp?: () => void;
  onCancel?: () => void;
  txHash?: string;
  elapsedSeconds?: number;
}

export function TxTimeoutModal({
  open,
  onClose,
  onSpeedUp,
  onCancel,
  txHash,
  elapsedSeconds,
}: TxTimeoutModalProps) {
  const elapsed = elapsedSeconds ?? 0;
  const minutes = Math.floor(elapsed / 60);
  const seconds = elapsed % 60;
  const elapsedLabel = minutes > 0 ? `${minutes}m ${seconds}s` : `${seconds}s`;

  return (
    <ModalShell open={open} onClose={onClose}>
      <div className="flex flex-col items-center text-center gap-4 pt-4">
        <div className="w-14 h-14 rounded-2xl bg-pax-warning-muted flex items-center justify-center">
          <Clock className="w-7 h-7 text-pax-warning" />
        </div>

        <div className="space-y-1">
          <h3 className="text-base font-bold">Transaction Pending</h3>
          <p className="text-sm text-pax-muted leading-relaxed">
            Your transaction has been pending for {elapsedLabel}. The network may be congested.
          </p>
        </div>

        {txHash && (
          <div className="w-full px-4 py-3 rounded-xl bg-pax-control text-center">
            <p className="text-xs font-mono text-pax-muted break-all">{txHash}</p>
          </div>
        )}

        <div className="w-full flex flex-col gap-2.5 mt-1">
          {onSpeedUp && (
            <button
              onClick={() => {
                onSpeedUp();
                onClose();
              }}
              className="w-full flex items-center justify-center gap-2 py-3.5 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale"
            >
              <Zap className="w-4 h-4" />
              Speed Up (increase gas)
            </button>
          )}
          {onCancel && (
            <button
              onClick={() => {
                onCancel();
                onClose();
              }}
              className="w-full py-3.5 rounded-2xl bg-pax-error-muted text-pax-error text-sm font-semibold press-scale  "
            >
              Cancel Transaction
            </button>
          )}
          <button
            onClick={onClose}
            className="w-full py-3 rounded-2xl bg-pax-control text-sm font-medium text-pax-muted press-scale"
          >
            Wait longer
          </button>
        </div>
      </div>
    </ModalShell>
  );
}

// ── Rate Limit Modal ─────────────────────────────────────────────────────────
interface RateLimitModalProps {
  open: boolean;
  onClose: () => void;
  retryAfterSeconds?: number;
  onRetry?: () => void;
}

export function RateLimitModal({
  open,
  onClose,
  retryAfterSeconds = 30,
  onRetry,
}: RateLimitModalProps) {
  const [countdown, setCountdown] = useState(retryAfterSeconds);

  useEffect(() => {
    if (!open) return;
    setCountdown(retryAfterSeconds);
    const interval = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          clearInterval(interval);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(interval);
  }, [open, retryAfterSeconds]);

  return (
    <ModalShell open={open} onClose={onClose}>
      <div className="flex flex-col items-center text-center gap-4 pt-4">
        <div className="w-14 h-14 rounded-2xl bg-pax-warning-muted flex items-center justify-center">
          <AlertTriangle className="w-7 h-7 text-pax-warning" />
        </div>

        <div className="space-y-1">
          <h3 className="text-base font-bold">Rate Limit Reached</h3>
          <p className="text-sm text-pax-muted leading-relaxed">
            Too many requests in a short time. Please wait before trying again.
          </p>
        </div>

        {countdown > 0 ? (
          <div className="w-20 h-20 rounded-full   flex flex-col items-center justify-center">
            <span className="text-2xl font-mono font-bold text-pax-warning">{countdown}</span>
            <span className="text-xs text-pax-muted">sec</span>
          </div>
        ) : (
          <div className="w-14 h-14 rounded-2xl bg-pax-success-muted flex items-center justify-center">
            <Zap className="w-6 h-6 text-pax-success" />
          </div>
        )}

        <div className="w-full flex flex-col gap-2.5 mt-1">
          {countdown === 0 && onRetry && (
            <button
              onClick={() => {
                onRetry();
                onClose();
              }}
              className="w-full py-3.5 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale"
            >
              Try Again
            </button>
          )}
          <button
            onClick={onClose}
            className="w-full py-3 rounded-2xl bg-pax-control text-sm font-medium text-pax-muted press-scale"
          >
            Dismiss
          </button>
        </div>
      </div>
    </ModalShell>
  );
}

// ── Contract Revert Modal ────────────────────────────────────────────────────
interface ContractRevertModalProps {
  open: boolean;
  onClose: () => void;
  revertReason?: string;
  txHash?: string;
  onViewExplorer?: () => void;
}

export function ContractRevertModal({
  open,
  onClose,
  revertReason,
  txHash,
  onViewExplorer,
}: ContractRevertModalProps) {
  const shortHash = txHash ? `${txHash.slice(0, 10)}...${txHash.slice(-8)}` : '';

  return (
    <ModalShell open={open} onClose={onClose}>
      <div className="flex flex-col gap-4 pt-4">
        <div className="pax-transaction-heading flex items-center gap-3">
          <div className="w-12 h-12 rounded-2xl bg-pax-error-muted flex items-center justify-center shrink-0">
            <X className="w-6 h-6 text-pax-error" />
          </div>
          <div>
            <h3 className="text-base font-bold">Transaction Failed</h3>
            <p className="text-xs text-pax-muted mt-0.5">Contract rejected the transaction</p>
          </div>
        </div>

        {revertReason && (
          <div className="rounded-xl bg-pax-error-muted   px-4 py-3 space-y-1">
            <p className="text-xs font-semibold text-pax-error">Revert reason</p>
            <p className="text-xs text-pax-error font-mono leading-relaxed break-words">
              {revertReason}
            </p>
          </div>
        )}

        {txHash && (
          <div className="px-4 py-3 rounded-xl bg-pax-control flex items-center justify-between">
            <span className="text-xs text-pax-muted">Transaction</span>
            <span className="text-xs font-mono text-pax-light">{shortHash}</span>
          </div>
        )}

        <div className="flex flex-col gap-2.5">
          {onViewExplorer && (
            <button
              onClick={() => {
                onViewExplorer();
                onClose();
              }}
              className="w-full flex items-center justify-center gap-2 py-3.5 rounded-2xl bg-pax-control text-sm font-medium press-scale"
            >
              View on PaxScan
            </button>
          )}
          <button
            onClick={onClose}
            className="w-full py-3 rounded-2xl bg-pax-control text-sm font-medium text-pax-muted press-scale"
          >
            Close
          </button>
        </div>
      </div>
    </ModalShell>
  );
}
