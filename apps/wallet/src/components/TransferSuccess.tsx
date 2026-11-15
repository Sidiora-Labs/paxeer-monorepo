'use client';

import { useState, useEffect } from 'react';
import {
  JsonRpcClient,
  observeTransfer,
  recoverTransferObservation,
  statusLadder,
  submittedTransfer,
  type TransferIdentity,
  type TransferObservation,
  type TransferTruth,
} from '@paxeer/wallet';
import { StatusLadder } from '@/account/StatusLadder';
import { motion, AnimatePresence } from 'framer-motion';
import { ModalPanel } from '@/components/ui/primitives';
import { SvgIcon } from '@/components/ui/SvgIcon';
import { transferObservationsRepository } from '@/platform/storage/repositories';
import { motionDurations, motionEase } from '@/theme/motion';
import {
  resolveTransferOrigin,
  saveTransferObservation,
  transferStorageKey,
  type TransferOrigin,
} from './transferOrigin';

interface TransferSuccessProps {
  fromLabel: string;
  fromAmount: string;
  fromSymbol: string;
  toLabel: string;
  toAmount?: string;
  toSymbol?: string;
  transfer: TransferIdentity;
  explorerUrl?: string;
  submissionWarning?: string;
  onExplorerView?: () => void;
  onDone: (terminal: boolean) => void;
}

const draw = {
  hidden: { pathLength: 0, opacity: 0 },
  visible: (i: number) => ({
    pathLength: 1,
    opacity: 1,
    transition: {
      pathLength: {
        delay: i * motionDurations.normal,
        type: 'spring',
        duration: motionDurations.shimmer,
        bounce: 0.2,
        ease: motionEase,
      },
      opacity: { delay: i * motionDurations.normal, duration: motionDurations.quick },
    },
  }),
};

function AnimatedCheckmark() {
  return (
    <motion.svg animate="visible" className="h-20 w-20" initial="hidden" viewBox="0 0 100 100">
      <title>Success</title>
      <motion.circle
        custom={0}
        cx="50"
        cy="50"
        r="42"
        stroke="var(--color-status-success)"
        style={{
          strokeWidth: 2,
          strokeLinecap: 'round',
          fill: 'transparent',
        }}
        variants={draw as any}
      />
      <motion.path
        custom={1}
        d="M32 50L45 63L68 35"
        stroke="var(--color-status-success)"
        style={{
          strokeWidth: 2.5,
          strokeLinecap: 'round',
          strokeLinejoin: 'round',
          fill: 'transparent',
        }}
        variants={draw as any}
      />
    </motion.svg>
  );
}

const OBSERVATION_INTERVAL_MS = 2000;

const TRUTH_TITLES: Readonly<Record<TransferTruth, string>> = {
  submitted: 'Transfer Submitted',
  pending: 'Transfer Pending',
  unknown: 'Transfer Status Unknown',
  replaced: 'Transfer Replaced',
  reverted: 'Transfer Reverted',
  included: 'Transfer Included',
};

function recoverObservation(identity: TransferIdentity): TransferObservation {
  try {
    const raw = transferObservationsRepository.read()[transferStorageKey(identity)];
    return raw ? recoverTransferObservation(identity, raw) : submittedTransfer(identity);
  } catch {
    return submittedTransfer(identity);
  }
}

export function createTransferObservationClient(url: string): JsonRpcClient {
  return new JsonRpcClient({
    url,
    timeoutMs: 10_000,
    fetch: (input, init) => fetch(input, { ...init, cache: 'no-store' }),
  });
}

export function useTransferObservation(identity: TransferIdentity): {
  observation: TransferObservation;
  error: string;
  origin: TransferOrigin | null;
} {
  const [observation, setObservation] = useState<TransferObservation>(() =>
    submittedTransfer(identity)
  );
  const [error, setError] = useState('');
  const [origin, setOrigin] = useState<TransferOrigin | null>(null);

  useEffect(() => {
    let cancelled = false;
    let running = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let current = recoverObservation(identity);
    setObservation(current);
    setError('');
    let retainedOrigin: TransferOrigin;
    let persistenceError = '';
    setOrigin(null);
    try {
      retainedOrigin = resolveTransferOrigin(identity);
      setOrigin(retainedOrigin);
    } catch (caught) {
      setObservation({ ...current, truth: 'unknown', steps: [], blockNumber: null });
      setError(caught instanceof Error ? caught.message : 'The submitted network is unavailable.');
      return;
    }
    const persist = (next: TransferObservation) => {
      try {
        saveTransferObservation(next, retainedOrigin);
        persistenceError = '';
      } catch {
        persistenceError =
          'Observation is available, but this browser could not save its original network for reload.';
      }
    };
    // Retain the origin before the first request, including offline/pending submissions.
    persist(current);
    setError(persistenceError);
    const rpc = createTransferObservationClient(retainedOrigin.rpcUrl);
    const tick = async () => {
      if (cancelled || running) return;
      if (timer) clearTimeout(timer);
      running = true;
      try {
        const next = await observeTransfer((method, params) => rpc.call(method, params), current, {
          url: retainedOrigin.statusUrl,
        });
        if (cancelled) return;
        current = next;
        setObservation(next);
        persist(next);
        setError([next.warning, persistenceError].filter(Boolean).join(' '));
      } catch (e) {
        if (!cancelled) {
          current = {
            ...current,
            truth: 'unknown',
            steps: [],
            blockNumber: null,
            blockHash: undefined,
          };
          setObservation(current);
          setError(e instanceof Error ? e.message : 'The transaction status could not be read');
        }
      } finally {
        running = false;
        if (!cancelled) timer = setTimeout(tick, OBSERVATION_INTERVAL_MS);
      }
    };
    void tick();
    window.addEventListener('online', tick);
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      window.removeEventListener('online', tick);
    };
  }, [identity.hash, identity.chainId]);

  const matches =
    observation.identity.hash.toLowerCase() === identity.hash.toLowerCase() &&
    observation.identity.chainId === identity.chainId;
  return {
    observation: matches ? observation : submittedTransfer(identity),
    error: matches ? error : '',
    origin: matches && origin?.chainId === identity.chainId ? origin : null,
  };
}

export function TransferSuccess(props: TransferSuccessProps) {
  const { observation, error, origin } = useTransferObservation(props.transfer);
  const explorerUrl = origin
    ? `${origin.explorerUrl.replace(/\/+$/, '')}/tx/${props.transfer.hash}`
    : undefined;
  return (
    <TransferSuccessResult
      {...props}
      explorerUrl={explorerUrl}
      onExplorerView={undefined}
      observation={observation}
      error={error}
    />
  );
}

export function TransferSuccessResult({
  fromLabel,
  fromAmount,
  fromSymbol,
  toLabel,
  toAmount,
  toSymbol,
  transfer,
  explorerUrl,
  submissionWarning,
  onExplorerView,
  onDone,
  observation,
  error,
}: TransferSuccessProps & { observation: TransferObservation; error: string }) {
  const txHash = transfer.hash;
  const completed =
    observation.truth === 'included' && observation.steps.some((step) => step.rung === 'committed');
  const receipted =
    observation.truth === 'included' &&
    statusLadder.highest(observation.steps)?.rung === 'receipted';
  const phase: 'processing' | 'completed' = completed ? 'completed' : 'processing';

  const shortHash = txHash.length > 16 ? `${txHash.slice(0, 10)}...${txHash.slice(-8)}` : txHash;

  return (
    <ModalPanel
      onDismiss={() =>
        onDone(completed || observation.truth === 'reverted' || observation.truth === 'replaced')
      }
      aria-label="Transfer status"
      data-transfer-hash={transfer.hash}
      data-transfer-chain={transfer.chainId}
      className="pax-result-panel fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-pax-bg backdrop-blur-sm px-6 py-6"
    >
      <motion.div
        initial={{ opacity: 0, y: 'var(--space-5)' }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: motionDurations.settle, ease: motionEase }}
        className="my-auto w-full max-w-sm flex flex-col items-center"
      >
        {/* Animated icon */}
        <div className="relative flex items-center justify-center size-[var(--size-success-mark)] mb-5">
          <motion.div
            animate={{ opacity: [0, 0.8, 0.6] }}
            className={`absolute inset-0 rounded-full blur-2xl ${completed ? 'bg-pax-success-muted' : 'bg-pax-control'}`}
            initial={{ opacity: 0 }}
            transition={{
              duration: motionDurations.shimmer,
              times: [0, motionDurations.settle / motionDurations.shimmer, 1],
            }}
          />
          <AnimatePresence mode="wait">
            {phase === 'completed' ? (
              <motion.div
                key="check"
                initial={{ opacity: 0, scale: 0.5, rotate: -90 }}
                animate={{ opacity: 1, scale: 1, rotate: 0 }}
                transition={{ duration: motionDurations.settle, ease: motionEase }}
              >
                <AnimatedCheckmark />
              </motion.div>
            ) : (
              <motion.div
                key="spinner"
                exit={{ opacity: 0, scale: 0.5, rotate: 180 }}
                transition={{ duration: motionDurations.fade }}
                className="relative"
              >
                <motion.div
                  animate={{ rotate: 360 }}
                  className="absolute inset-0 rounded-full  "
                  style={{}}
                  transition={{
                    rotate: { duration: motionDurations.scan, repeat: Infinity, ease: 'linear' },
                  }}
                />
                <div className="rounded-full bg-pax-card p-4">
                  <span aria-hidden="true" className="text-2xl">
                    {observation.truth === 'reverted' || observation.truth === 'replaced'
                      ? '!'
                      : '…'}
                  </span>
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Title */}
        <AnimatePresence mode="wait">
          <motion.h2
            key={transfer.hash}
            data-truth={observation.truth}
            initial={{ opacity: 0, y: 'var(--space-2-5)' }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 'calc(-1 * var(--space-2-5))' }}
            transition={{ duration: motionDurations.fade }}
            className="text-lg font-bold mb-1"
          >
            {completed
              ? 'Transfer Completed'
              : receipted
                ? 'Receipted'
                : TRUTH_TITLES[observation.truth]}
          </motion.h2>
        </AnimatePresence>

        <AnimatePresence mode="wait">
          <motion.p
            key={phase}
            initial={{ opacity: 0, y: 'var(--space-1)' }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 'calc(-1 * var(--space-1))' }}
            transition={{ duration: motionDurations.quick }}
            className={`text-xs mb-5 ${observation.truth === 'reverted' ? 'text-pax-error' : 'text-pax-success'}`}
          >
            {shortHash}
          </motion.p>
        </AnimatePresence>

        <div className="w-full mb-4">
          <StatusLadder steps={observation.steps} />
          {observation.lastVerified && observation.steps.length === 0 && (
            <p data-retained-evidence className="mt-2 text-xs text-pax-muted">
              Saved evidence for block {observation.lastVerified.blockNumber}; current status
              requires revalidation.
            </p>
          )}
          {observation.replacementHash && (
            <p className="mt-2 text-xs break-all">Replacement: {observation.replacementHash}</p>
          )}
          {submissionWarning && (
            <p role="alert" className="mt-2 text-xs text-pax-muted">
              {submissionWarning}
            </p>
          )}
          {error && (
            <p role="alert" className="mt-2 text-xs text-pax-muted">
              {error}
            </p>
          )}
        </div>

        {/* Transfer card */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: motionDurations.normal, duration: motionDurations.settle }}
          className="w-full"
        >
          <motion.div
            animate={{ gap: phase === 'completed' ? 'var(--space-0)' : 'var(--space-2)' }}
            className="flex flex-col"
            transition={{ duration: motionDurations.settle, ease: motionEase }}
          >
            {/* From */}
            <div
              className={`glass-card p-3 transition-all duration-normal ${phase === 'completed' ? 'rounded-b-none ' : ''}`}
            >
              <span className="flex items-center gap-1 text-xs text-pax-muted mb-1">
                <SvgIcon name="arrow-up" className="w-3 h-3" /> From
              </span>
              <div className="flex items-center gap-2">
                <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-pax-control font-medium text-sm text-pax-accent">
                  {fromSymbol[0]}
                </span>
                <div className="flex flex-col">
                  <motion.span animate={{ opacity: 1 }} className="text-sm font-medium">
                    {fromAmount} {fromSymbol}
                  </motion.span>
                  <span className="text-xs text-pax-muted">{fromLabel}</span>
                </div>
              </div>
            </div>

            {/* To */}
            <div
              className={`glass-card p-3 transition-all duration-normal ${phase === 'completed' ? 'rounded-t-none ' : ''}`}
            >
              <span className="flex items-center gap-1 text-xs text-pax-muted mb-1">
                <SvgIcon name="arrow-down" className="w-3 h-3" /> To
              </span>
              <div className="flex items-center gap-2">
                <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-pax-control font-medium text-sm text-pax-accent">
                  {(toSymbol || fromSymbol)[0]}
                </span>
                <div className="flex flex-col">
                  <motion.span animate={{ opacity: 1 }} className="text-sm font-medium">
                    {toAmount || fromAmount} {toSymbol || fromSymbol}
                  </motion.span>
                  <span className="text-xs text-pax-muted">{toLabel}</span>
                </div>
              </div>
            </div>
          </motion.div>
        </motion.div>

        <AnimatePresence>
          {
            <motion.div
              initial={{ opacity: 0, y: 'var(--space-2-5)' }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: motionDurations.quick, duration: motionDurations.fade }}
              className="flex flex-wrap justify-center gap-3 mt-6 w-full"
            >
              {(onExplorerView || explorerUrl) && (
                <a
                  href={explorerUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={
                    onExplorerView
                      ? (event) => {
                          event.preventDefault();
                          onExplorerView();
                        }
                      : undefined
                  }
                  className="flex flex-none items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl bg-pax-control text-xs font-medium press-scale"
                >
                  <SvgIcon name="external-link" className="w-4 h-4" /> Explorer
                </a>
              )}
              <button
                onClick={() =>
                  onDone(
                    completed ||
                      observation.truth === 'reverted' ||
                      observation.truth === 'replaced'
                  )
                }
                className="flex-none px-6 py-2.5 rounded-xl bg-pax-accent text-pax-on-accent text-xs font-semibold press-scale"
              >
                {completed ? 'Done' : 'Close'}
              </button>
            </motion.div>
          }
        </AnimatePresence>
      </motion.div>
    </ModalPanel>
  );
}
