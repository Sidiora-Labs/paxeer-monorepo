'use client';

import { RateLimitModal, TxTimeoutModal } from '@/components/ui/TransactionModals';
import { classifySwapError } from './prettifyError';
import type { UseSwapExecutionResult } from './useSwapExecution';

export function SwapRecoveryActions({ execution }: { execution: UseSwapExecutionResult }) {
  const errorKind = execution.execError ? classifySwapError(execution.execError) : null;
  const hasRecovery = !!execution.intent && execution.progress !== 'confirmed';
  return (
    <>
      {hasRecovery && (
        <div role="status" className="rounded-2xl bg-white/5 px-4 py-3 text-sm space-y-2">
          <p>
            {execution.completedLegs} of {execution.intent!.expectedLegs} swap legs confirmed.
          </p>
          <p>
            {execution.progress === 'partial'
              ? 'This route completed only partly. Keep the received tokens and reconcile the remaining leg before starting another swap.'
              : execution.progress === 'pending'
                ? 'The original transaction is awaiting reconciliation.'
                : 'This route stopped. Review its transactions before approving a new swap.'}
          </p>
          {execution.intent!.transactions.map((tx, index) => (
            <p key={index} className="break-all text-xs">
              {tx.kind === 'swap' ? `Swap leg ${tx.leg + 1}` : 'Approval'} · nonce {tx.nonce} ·{' '}
              {tx.hash ?? 'Broadcast result unknown'} ·{' '}
              {tx.receipt ? (tx.receipt.status === 1 ? 'confirmed' : 'reverted') : 'pending'}
            </p>
          ))}
          <button
            type="button"
            disabled={execution.executing}
            onClick={() => void execution.recover()}
            className="w-full rounded-full bg-pax-accent text-black font-semibold py-3 disabled:opacity-40"
          >
            Check transaction status
          </button>
          {(execution.progress === 'reverted' || execution.progress === 'failed') && (
            <button
              type="button"
              disabled={execution.executing}
              onClick={execution.reset}
              className="w-full py-2"
            >
              Review a new swap
            </button>
          )}
          {execution.execError && <p className="text-xs text-red-400">{execution.execError}</p>}
        </div>
      )}
      <TxTimeoutModal
        open={errorKind === 'timeout'}
        txHash={execution.txHash || undefined}
        onClose={() => {
          execution.clearError();
          void execution.recover();
        }}
      />
      <RateLimitModal
        open={errorKind === 'ratelimit'}
        onClose={execution.clearError}
        onRetry={() => void execution.recover()}
      />
    </>
  );
}
