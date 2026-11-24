'use client';

import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ModalPanel } from '@/components/ui/primitives';
import { SvgIcon } from '@/components/ui/SvgIcon';
import Image from 'next/image';
import { openExternalUrl } from '@/lib/security/navigation';
import { isSwapReceiptResult, type SwapReceiptResult } from './swapReceiptResult';
import { motionDurations, motionEase } from '@/theme/motion';

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

interface SwapSuccessProps {
  fromAmount: string;
  fromSymbol: string;
  fromIconUrl?: string;
  toAmount: string;
  toSymbol: string;
  toIconUrl?: string;
  txHash: string;
  result?: SwapReceiptResult;
  explorerUrl?: string;
  onExplorerView?: () => void;
  onDone: () => void;
}

export function SwapSuccess({
  fromAmount: estimatedFromAmount,
  fromSymbol: estimatedFromSymbol,
  fromIconUrl: estimatedFromIconUrl,
  toAmount: estimatedToAmount,
  toSymbol: estimatedToSymbol,
  toIconUrl: estimatedToIconUrl,
  result,
  txHash,
  explorerUrl,
  onExplorerView,
  onDone,
}: SwapSuccessProps) {
  const verified =
    result && isSwapReceiptResult(result) && result.txHash.toLowerCase() === txHash.toLowerCase()
      ? result
      : undefined;
  const fromAmount = verified?.fromAmount ?? estimatedFromAmount;
  const fromSymbol = verified?.fromSymbol ?? estimatedFromSymbol;
  const fromIconUrl = verified ? verified.fromIconUrl : estimatedFromIconUrl;
  const toAmount = verified?.toAmount ?? estimatedToAmount;
  const toSymbol = verified?.toSymbol ?? estimatedToSymbol;
  const toIconUrl = verified ? verified.toIconUrl : estimatedToIconUrl;
  const [phase, setPhase] = useState<'processing' | 'completed'>('processing');

  useEffect(() => {
    setPhase('processing');
    if (!verified) return;
    const timer = setTimeout(() => setPhase('completed'), 1400);
    return () => clearTimeout(timer);
  }, [verified]);

  const shortHash = txHash.length > 16 ? `${txHash.slice(0, 10)}...${txHash.slice(-8)}` : txHash;

  return (
    <ModalPanel
      onDismiss={() => {
        if (phase === 'completed' || !verified) onDone();
      }}
      aria-label="Swap status"
      data-swap-quote={verified?.quoteId}
      data-swap-chain={verified?.chainId}
      className="pax-result-panel fixed inset-0 z-50 flex items-start justify-center overflow-y-auto bg-pax-bg backdrop-blur-sm px-6 py-6"
    >
      <motion.div
        initial={{ opacity: 0, y: 'var(--space-5)' }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: motionDurations.settle, ease: motionEase }}
        className="my-auto w-full max-w-sm flex flex-col items-center"
      >
        {/* Icon */}
        <div className="relative flex items-center justify-center size-[var(--size-success-mark)] mb-5">
          <motion.div
            animate={{ opacity: [0, 0.8, 0.6] }}
            className="absolute inset-0 rounded-full bg-pax-success-muted blur-2xl"
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
                  <SvgIcon name="refresh" className="w-8 h-8" />
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>

        {/* Title */}
        <AnimatePresence mode="wait">
          <motion.h2
            key={phase}
            initial={{ opacity: 0, y: 'var(--space-2-5)' }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 'calc(-1 * var(--space-2-5))' }}
            transition={{ duration: motionDurations.fade }}
            className="text-lg font-bold mb-1"
          >
            {verified && phase === 'completed' ? 'Swap Completed' : 'Verifying Swap Output'}
          </motion.h2>
        </AnimatePresence>

        <AnimatePresence mode="wait">
          <motion.p
            key={phase}
            initial={{ opacity: 0, y: 'var(--space-1)' }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 'calc(-1 * var(--space-1))' }}
            transition={{ duration: motionDurations.quick }}
            className="text-xs text-pax-success mb-5"
          >
            {phase === 'completed' ? shortHash : 'Processing...'}
          </motion.p>
        </AnimatePresence>

        {/* Swap card */}
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
            {/* You pay */}
            <div
              className={`glass-card p-3 transition-all duration-normal ${phase === 'completed' ? 'rounded-b-none ' : ''}`}
            >
              <span className="flex items-center gap-1 text-size-10 text-pax-muted mb-1">
                <SvgIcon name="arrow-up" className="w-3 h-3" />{' '}
                {verified ? 'You paid' : 'Quoted input'}
              </span>
              <div className="flex items-center gap-2">
                {fromIconUrl ? (
                  <Image
                    src={fromIconUrl}
                    alt={fromSymbol}
                    className="h-7 w-7 rounded-full bg-pax-control object-cover"
                    onError={(e) => {
                      (e.target as HTMLImageElement).src = '/wallet/default_icon.webp';
                    }}
                    width={28}
                    height={28}
                  />
                ) : (
                  <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-pax-control font-medium text-sm text-pax-accent">
                    {fromSymbol[0]}
                  </span>
                )}
                <div className="flex flex-col">
                  <motion.span animate={{ opacity: 1 }} className="text-sm font-medium">
                    {fromAmount} {fromSymbol}
                  </motion.span>
                </div>
              </div>
            </div>

            {/* You receive */}
            <div
              className={`glass-card p-3 transition-all duration-normal ${phase === 'completed' ? 'rounded-t-none ' : ''}`}
            >
              <span className="flex items-center gap-1 text-size-10 text-pax-muted mb-1">
                <SvgIcon name="arrow-down" className="w-3 h-3" />{' '}
                {verified ? 'You received' : 'Estimated output'}
              </span>
              <div className="flex items-center gap-2">
                {toIconUrl ? (
                  <Image
                    src={toIconUrl}
                    alt={toSymbol}
                    className="h-7 w-7 rounded-full bg-pax-control object-cover"
                    onError={(e) => {
                      (e.target as HTMLImageElement).src = '/wallet/default_icon.webp';
                    }}
                    width={28}
                    height={28}
                  />
                ) : (
                  <span className="inline-flex h-7 w-7 items-center justify-center rounded-lg bg-pax-control font-medium text-sm text-pax-accent">
                    {toSymbol[0]}
                  </span>
                )}
                <div className="flex flex-col">
                  <motion.span
                    animate={{ opacity: 1 }}
                    className="text-sm font-medium text-pax-success"
                  >
                    {toAmount} {toSymbol}
                  </motion.span>
                </div>
              </div>
            </div>
          </motion.div>
        </motion.div>

        {!verified && (
          <p role="status" className="text-xs text-pax-muted mt-3">
            Received amount awaits verified execution evidence.
          </p>
        )}

        {/* Actions */}
        <AnimatePresence>
          {(phase === 'completed' || !verified) && (
            <motion.div
              initial={{ opacity: 0, y: 'var(--space-2-5)' }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: motionDurations.quick, duration: motionDurations.fade }}
              className="flex flex-wrap justify-center gap-3 mt-6 w-full"
            >
              {(onExplorerView || explorerUrl) && (
                <button
                  onClick={() =>
                    onExplorerView
                      ? onExplorerView()
                      : explorerUrl
                        ? openExternalUrl(explorerUrl)
                        : undefined
                  }
                  className="flex flex-none items-center justify-center gap-1.5 px-4 py-2.5 rounded-xl bg-pax-control text-xs font-medium press-scale"
                >
                  <SvgIcon name="external-link" className="w-4 h-4" /> Explorer
                </button>
              )}
              <button
                onClick={onDone}
                className="flex-none px-6 py-2.5 rounded-xl bg-pax-accent text-pax-on-accent text-xs font-semibold press-scale"
              >
                {verified ? 'Done' : 'Close'}
              </button>
            </motion.div>
          )}
        </AnimatePresence>
      </motion.div>
    </ModalPanel>
  );
}
