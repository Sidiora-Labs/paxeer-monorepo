'use client';

import { motionEase } from '@/theme/motion';

import { useState, useEffect } from 'react';
import { motion, AnimatePresence, useReducedMotion } from 'framer-motion';
import { motionDurations } from '@/theme/motion';

// ── Splash ────────────────────────────────────────────────────────────────────

const SPLASH_MS = motionDurations.splash * 1000;
const EXIT_MS = motionDurations.exit * 1000;

interface SplashScreenProps {
  children: React.ReactNode;
}

export function SplashScreen({ children }: SplashScreenProps) {
  const reducedMotion = useReducedMotion();
  const [phase, setPhase] = useState<'visible' | 'fading' | 'gone'>('visible');

  useEffect(() => {
    if (reducedMotion) {
      setPhase('gone');
      return;
    }
    const showTimer = setTimeout(() => setPhase('fading'), SPLASH_MS);
    const goneTimer = setTimeout(() => setPhase('gone'), SPLASH_MS + EXIT_MS);
    return () => {
      clearTimeout(showTimer);
      clearTimeout(goneTimer);
    };
  }, [reducedMotion]);

  return (
    <>
      <AnimatePresence>
        {phase !== 'gone' && (
          <motion.div
            key="splash"
            initial={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: EXIT_MS / 1000, ease: motionEase }}
            aria-hidden="true"
            className="fixed inset-0 z-[9999] overflow-hidden pointer-events-none"
          >
            {/* Full-bleed brand image */}
            <motion.img
              src="/wallet/PAXPORT.png"
              alt="Paxport"
              className="absolute inset-0 w-full h-full object-cover"
              initial={{ opacity: 0, scale: 1.04 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ duration: motionDurations.intro, ease: motionEase }}
            />

            {/* Progress bar */}
            <motion.div
              className="absolute bottom-14 left-1/2 -translate-x-1/2 h-px rounded-full overflow-hidden"
              style={{ width: 'var(--space-20)', background: 'var(--color-border-subtle)' }}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: motionDurations.exit, duration: motionDurations.fade }}
            >
              <motion.div
                className="h-full rounded-full"
                style={{ background: 'var(--color-action-primary)' }}
                initial={{ width: '0%' }}
                animate={{ width: '100%' }}
                transition={{
                  delay: motionDurations.progress,
                  duration: motionDurations.splash - motionDurations.progress,
                  ease: 'linear',
                }}
              />
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
      {children}
    </>
  );
}
