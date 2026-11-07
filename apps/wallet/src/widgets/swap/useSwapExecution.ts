'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ethers } from 'ethers';
import { useWallet, useWalletActions } from '@/providers/WalletProvider';
import type { SwapToken, SwapQuote, SwapParams } from '@/lib/swap';
import {
  validateEvmAddress,
  validateSlippageBps,
  validateSpendableBalance,
  validateSwapQuote,
} from '@/lib/txValidation';
import {
  JournaledSwapExecution,
  SwapJournal,
  swapProgress,
  type SwapIntent,
  type SwapProgress,
} from './swapJournal';

export interface UseSwapExecutionResult {
  executing: boolean;
  txHash: string;
  execError: string;
  intent: SwapIntent | null;
  progress: SwapProgress | null;
  completedLegs: number;
  execute: (
    bestQuote: SwapQuote,
    fromToken: SwapToken,
    toToken: SwapToken,
    slippageBps: number,
    recipient: string,
    balanceRaw?: bigint | null
  ) => Promise<void>;
  recover: () => Promise<void>;
  reset: () => void;
  clearError: () => void;
}

export interface SwapExecutionContext {
  account: string | null;
  chainId: number | null;
  getSigner: () => Promise<ethers.Signer>;
  isCurrent: () => boolean;
}

/** Shared by the wallet hook and consumers that already own a signer session. */
export function useJournaledSwapExecution(context: SwapExecutionContext): UseSwapExecutionResult {
  const { account, chainId, getSigner, isCurrent } = context;
  const scope = account && chainId ? `${chainId}:${account.toLowerCase()}` : '';
  const [view, setView] = useState<{
    scope: string;
    intent: SwapIntent | null;
    error: string;
    busy: boolean;
  }>({ scope: '', intent: null, error: '', busy: false });
  const session = useRef<{
    scope: string;
    execution: JournaledSwapExecution;
    busy: boolean;
  } | null>(null);
  const currentContext = useRef(context);
  currentContext.current = context;

  useEffect(() => {
    setView({ scope, intent: null, error: '', busy: false });
    if (!account || !chainId) return;
    let execution: JournaledSwapExecution;
    try {
      const journal = new SwapJournal(window.localStorage, account, chainId);
      const current = journal.current();
      execution = new JournaledSwapExecution(
        journal,
        (intent) => {
          setView((previous) =>
            previous.scope === scope
              ? {
                  ...previous,
                  intent,
                  error: intent && swapProgress(intent) === 'confirmed' ? '' : previous.error,
                }
              : previous
          );
        },
        () =>
          currentContext.current.account?.toLowerCase() === account.toLowerCase() &&
          currentContext.current.chainId === chainId &&
          isCurrent()
      );
      const owner = { scope, execution, busy: false };
      session.current = owner;
      setView({ scope, intent: current, error: current?.error ?? '', busy: !!current });
      if (current) {
        owner.busy = true;
        void getSigner()
          .then((signer) => execution.recover(signer))
          .catch((error: unknown) => {
            if (session.current === owner)
              setView((previous) => ({
                ...previous,
                error: error instanceof Error ? error.message : 'Swap recovery failed',
              }));
          })
          .finally(() => {
            owner.busy = false;
            if (session.current === owner) setView((previous) => ({ ...previous, busy: false }));
          });
      }
    } catch (error) {
      setView({
        scope,
        intent: null,
        busy: false,
        error: error instanceof Error ? error.message : 'Swap journal unavailable',
      });
    }
    return () => {
      execution?.close();
      session.current = null;
    };
  }, [account, chainId, scope, getSigner, isCurrent]);

  const run = useCallback(
    async (action: (execution: JournaledSwapExecution, signer: ethers.Signer) => Promise<void>) => {
      const owner = session.current;
      if (!owner || owner.scope !== scope || owner.busy) return;
      owner.busy = true;
      setView((previous) => ({ ...previous, busy: true, error: '' }));
      try {
        await action(owner.execution, await getSigner());
      } catch (error) {
        if (session.current === owner)
          setView((previous) => ({
            ...previous,
            error: error instanceof Error ? error.message : 'Swap failed',
          }));
      } finally {
        owner.busy = false;
        if (session.current === owner) setView((previous) => ({ ...previous, busy: false }));
      }
    },
    [getSigner, scope]
  );

  const execute = useCallback(
    async (
      bestQuote: SwapQuote,
      fromToken: SwapToken,
      toToken: SwapToken,
      slippageBps: number,
      recipient: string,
      balanceRaw?: bigint | null
    ) =>
      run(async (execution, signer) => {
        if (execution.journal.current()) {
          await execution.recover(signer);
          return;
        }
        validateEvmAddress(recipient);
        validateSlippageBps(slippageBps);
        validateSwapQuote(bestQuote);
        validateSpendableBalance(BigInt(bestQuote.amountIn), balanceRaw);
        const params: SwapParams = {
          tokenIn: fromToken,
          tokenOut: toToken,
          amountIn: bestQuote.amountIn,
          slippageBps,
          recipient,
        };
        await execution.execute(signer, bestQuote, params);
      }),
    [run]
  );

  const recover = useCallback(() => run((execution, signer) => execution.recover(signer)), [run]);
  const clearError = useCallback(() => setView((previous) => ({ ...previous, error: '' })), []);
  const reset = useCallback(() => {
    const owner = session.current;
    if (!owner || owner.scope !== scope || owner.busy) return;
    try {
      owner.execution.journal.acknowledge();
      setView({ scope, intent: null, error: '', busy: false });
    } catch (error) {
      setView((previous) => ({
        ...previous,
        error: error instanceof Error ? error.message : 'Swap needs recovery',
      }));
    }
  }, [scope]);

  const visible = view.scope === scope ? view : { intent: null, error: '', busy: false };
  const intent = visible.intent;
  const swaps = intent?.transactions.filter((tx) => tx.kind === 'swap') ?? [];
  const txHash =
    swaps.findLast((tx) => tx.hash)?.hash ??
    intent?.transactions.findLast((tx) => tx.hash)?.hash ??
    '';
  return {
    executing: visible.busy,
    txHash,
    execError: visible.error,
    intent,
    progress: intent ? swapProgress(intent) : null,
    completedLegs: swaps.filter((tx) => tx.receipt?.status === 1).length,
    execute,
    recover,
    reset,
    clearError,
  };
}

export function useSwapExecution(): UseSwapExecutionResult {
  const { getSigner } = useWalletActions();
  const { address, chainId, writeEpoch, isWriteCurrent } = useWallet();
  const isCurrent = useCallback(
    () => !!address && !!chainId && isWriteCurrent(writeEpoch, address, chainId),
    [address, chainId, writeEpoch, isWriteCurrent]
  );
  return useJournaledSwapExecution({ account: address, chainId, getSigner, isCurrent });
}
