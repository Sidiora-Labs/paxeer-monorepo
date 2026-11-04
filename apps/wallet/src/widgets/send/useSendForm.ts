'use client';

/**
 * Send-form state machine.
 *
 * Owns the recipient address, amount, error/loading flags, the confirm-drawer
 * open state, and the resulting tx hash. Wires to {@link useWalletActions} for
 * the actual transfer and persists a pending optimistic event so the
 * portfolio screen can subtract immediately on the next mount.
 */

import { useCallback, useEffect, useState } from 'react';
import { submittedTransfer, type TransferIdentity } from '@paxeer/wallet';
import { useActiveNetwork } from '@/components/NetworkSwitcher';
import { useWallet, useWalletActions, useWalletState } from '@/providers/WalletProvider';
import { storeScopedPendingSend } from '@/widgets/portfolio/pendingSends';
import { saveRecentRecipient } from '@/lib/recentRecipients';
import {
  putTransferRecord,
  submittedTransfersRepository,
  type JsonValue,
} from '@/platform/storage/repositories';
import {
  validateEvmAddress,
  validateSpendableBalance,
  validateTokenAmount,
} from '@/lib/txValidation';
import { readTokenPrecision, type SendableToken } from './useSendableTokens';

export interface SubmittedTransfer extends TransferIdentity {
  sender: string;
  symbol: string;
  decimals: number;
  recipient: string;
  amount: string;
}

export interface UseSendFormResult {
  to: string;
  amount: string;
  loading: boolean;
  error: string;
  txHash: string;
  transfer: SubmittedTransfer | null;
  clearTransfer: () => void;
  confirmOpen: boolean;

  setTo: (value: string) => void;
  setAmount: (value: string) => void;
  applyPercentage: (pct: number, token: SendableToken | null) => void;

  openConfirm: (token?: SendableToken | null) => void;
  closeConfirm: () => void;
  validateBeforeConfirm: (token: SendableToken | null) => boolean;
  submit: (token: SendableToken | null) => Promise<void>;
}

const computePortion = (balanceRaw: string, pct: number, decimals: number): string | null => {
  try {
    const raw = BigInt(balanceRaw);
    const portion = (raw * BigInt(pct)) / BigInt(100);
    const divisor = BigInt(10) ** BigInt(decimals);
    const whole = portion / divisor;
    const frac = (portion % divisor).toString().padStart(decimals, '0');
    const trimmed = frac.replace(/0+$/, '');
    return trimmed ? `${whole}.${trimmed}` : `${whole}`;
  } catch {
    return null;
  }
};

function submittedTransferKey(chainId: number, sender: string): string {
  return `${chainId}:${sender.toLowerCase()}`;
}

function readSubmittedTransfer(chainId: number, sender: string): SubmittedTransfer | null {
  try {
    const raw = submittedTransfersRepository.read()[submittedTransferKey(chainId, sender)];
    if (!raw) return null;
    const value = raw as Partial<SubmittedTransfer>;
    if (
      typeof value.hash !== 'string' ||
      typeof value.chainId !== 'number' ||
      typeof value.sender !== 'string' ||
      value.sender.toLowerCase() !== sender.toLowerCase() ||
      typeof value.symbol !== 'string' ||
      !value.symbol ||
      typeof value.decimals !== 'number' ||
      !Number.isInteger(value.decimals) ||
      value.decimals < 0 ||
      value.decimals > 36 ||
      typeof value.recipient !== 'string' ||
      typeof value.amount !== 'string' ||
      !value.intent ||
      value.intent.sender?.toLowerCase() !== sender.toLowerCase() ||
      value.intent.recipient?.toLowerCase() !== value.recipient.toLowerCase()
    )
      return null;
    const identity = submittedTransfer({
      hash: value.hash,
      chainId: value.chainId,
      intent: value.intent,
    }).identity;
    if (identity.chainId !== chainId) return null;
    validateEvmAddress(value.recipient);
    if (validateTokenAmount(value.amount, value.decimals).toString() !== value.intent.amountRaw)
      return null;
    return {
      ...identity,
      sender: value.sender,
      symbol: value.symbol,
      decimals: value.decimals,
      recipient: value.recipient,
      amount: value.amount,
    };
  } catch {
    return null;
  }
}

export function useSendForm(): UseSendFormResult {
  const { send } = useWalletActions();
  const { activeAccount } = useWalletState();
  const sender = activeAccount?.address;
  const { chainId, writeEpoch, isWriteCurrent } = useWallet();
  const activeNetwork = useActiveNetwork();

  const [to, setTo] = useState('');
  const [amount, setAmount] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [transfer, setTransfer] = useState<SubmittedTransfer | null>(null);
  const activeTransfer =
    transfer?.sender.toLowerCase() === sender?.toLowerCase() &&
    transfer?.chainId === chainId &&
    chainId === activeNetwork.chainId
      ? transfer
      : null;
  const txHash = activeTransfer?.hash ?? '';

  useEffect(() => {
    const recovered =
      sender && chainId !== null && chainId === activeNetwork.chainId
        ? readSubmittedTransfer(chainId, sender)
        : null;
    setTransfer(recovered);
    setTo(recovered?.recipient ?? '');
    setAmount(recovered?.amount ?? '');
    setError('');
    setConfirmOpen(false);
  }, [sender, chainId, activeNetwork.chainId, writeEpoch]);

  const clearTransfer = useCallback(() => {
    try {
      if (sender && chainId !== null) {
        const key = submittedTransferKey(chainId, sender);
        submittedTransfersRepository.update((records) =>
          Object.fromEntries(Object.entries(records).filter(([entry]) => entry !== key))
        );
      }
    } catch {
      // storage unavailable: the in-memory identity is still cleared
    }
    setTransfer(null);
  }, [sender, chainId]);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const applyPercentage = useCallback((pct: number, token: SendableToken | null) => {
    if (
      !token ||
      readTokenPrecision(token.decimals) === null ||
      !token.balanceRaw ||
      token.balanceRaw === '0'
    )
      return;
    const result = computePortion(token.balanceRaw, pct, token.decimals);
    if (result !== null) {
      setAmount(result);
    } else if (pct === 100 && token.balance) {
      setAmount(token.balance);
    }
  }, []);

  const validateBeforeConfirm = useCallback(
    (token: SendableToken | null): boolean => {
      try {
        if (!token) throw new Error('Select a token to send.');
        if (readTokenPrecision(token.decimals) === null)
          throw new Error('Token precision is unavailable.');
        if (
          !sender ||
          chainId === null ||
          chainId !== activeNetwork.chainId ||
          !isWriteCurrent(writeEpoch, sender, chainId)
        ) {
          throw new Error(
            'Switch the wallet and selected network to the same chain before sending.'
          );
        }
        validateEvmAddress(to);
        const amountRaw = validateTokenAmount(amount, token.decimals);
        validateSpendableBalance(amountRaw, token.balanceRaw);
        setError('');
        return true;
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Invalid transaction');
        return false;
      }
    },
    [to, amount, sender, chainId, activeNetwork.chainId, isWriteCurrent, writeEpoch]
  );

  const openConfirm = useCallback(
    (token?: SendableToken | null) => {
      if (token !== undefined && !validateBeforeConfirm(token)) return;
      setError('');
      setConfirmOpen(true);
    },
    [validateBeforeConfirm]
  );

  const closeConfirm = useCallback(() => {
    setConfirmOpen(false);
    setError('');
  }, []);

  const submit = useCallback(
    async (token: SendableToken | null) => {
      if (!token || !sender || chainId === null || !validateBeforeConfirm(token)) return;
      setLoading(true);
      setError('');
      try {
        const recipient = validateEvmAddress(to);
        const amountRaw = validateTokenAmount(amount, token.decimals);
        validateSpendableBalance(amountRaw, token.balanceRaw);
        const hash = await send({
          to: recipient,
          value: amount,
          tokenAddress: token.address,
          decimals: token.decimals,
        });
        const identity: SubmittedTransfer = {
          ...submittedTransfer({
            hash,
            chainId,
            intent: {
              sender,
              recipient,
              amountRaw: amountRaw.toString(),
              tokenAddress: token.address,
            },
          }).identity,
          sender,
          recipient,
          amount,
          symbol: token.symbol,
          decimals: token.decimals,
        };
        try {
          const stored = JSON.parse(JSON.stringify(identity)) as JsonValue;
          submittedTransfersRepository.update((records) =>
            putTransferRecord(records, submittedTransferKey(identity.chainId, sender), stored)
          );
        } catch {
          setError('Transfer submitted. This browser could not save its identity for reload.');
        }
        setTransfer(identity);
        saveRecentRecipient(recipient);
        storeScopedPendingSend({
          sender,
          chainId,
          tokenAddress: token.address,
          symbol: token.symbol,
          amount,
          decimals: token.decimals,
          recipient,
          txHash: hash,
          timestamp: Date.now(),
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Transaction failed');
      } finally {
        setLoading(false);
      }
    },
    [to, amount, send, sender, chainId, validateBeforeConfirm]
  );

  return {
    to,
    amount,
    loading,
    error,
    txHash,
    transfer: activeTransfer,
    clearTransfer,
    confirmOpen,
    setTo,
    setAmount,
    applyPercentage,
    openConfirm,
    closeConfirm,
    validateBeforeConfirm,
    submit,
  };
}
