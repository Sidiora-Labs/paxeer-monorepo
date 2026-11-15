'use client';

import { motion, AnimatePresence } from 'framer-motion';
import { Loader2 } from 'lucide-react';
import { drawerSpringMotion } from '@/theme/motion';
import { ModalPanel } from '@/components/ui/primitives';
import { SvgIcon } from '@/components/ui/SvgIcon';

interface ConfirmDrawerProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  confirmLabel?: string;
  cancelLabel?: string;
  loading?: boolean;
  error?: string;
  children: React.ReactNode;
}

const itemVariants = {
  hidden: { y: 'var(--space-5)', opacity: 0 },
  visible: {
    y: 0,
    opacity: 1,
    transition: drawerSpringMotion,
  },
};

export function ConfirmDrawer({
  open,
  onClose,
  onConfirm,
  title,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  loading = false,
  error,
  children,
}: ConfirmDrawerProps) {
  return (
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-50 flex items-end justify-center">
          {/* Backdrop */}
          <div className="absolute inset-0 bg-pax-scrim" onClick={() => !loading && onClose()} />
          <ModalPanel
            onDismiss={() => !loading && onClose()}
            aria-label={title}
            className="relative w-full max-w-md bg-pax-card rounded-t-3xl p-5 safe-area-pb"
          >
            {/* Handle + header */}
            <motion.div variants={itemVariants} className="flex items-center justify-between mb-4">
              <div className="w-10 h-1 rounded-full bg-pax-control absolute top-2.5 left-1/2 -translate-x-1/2" />
              <h3 className="text-base font-bold">{title}</h3>
              <button
                aria-label="Close confirmation"
                onClick={() => !loading && onClose()}
                className="p-1.5 rounded-full bg-pax-control press-scale"
                disabled={loading}
              >
                <SvgIcon name="x" className="w-4 h-4" />
              </button>
            </motion.div>

            {/* Content */}
            <motion.div variants={itemVariants} className="space-y-3 mb-5">
              {children}
            </motion.div>

            {/* Error */}
            {error && (
              <motion.p variants={itemVariants} className="text-pax-error text-xs mb-3">
                {error}
              </motion.p>
            )}

            {/* Actions */}
            <motion.div variants={itemVariants} className="flex flex-col gap-2.5">
              <button
                onClick={onConfirm}
                disabled={loading}
                className="w-full flex items-center justify-center gap-2 py-3.5 rounded-2xl bg-pax-accent text-pax-on-accent font-semibold text-sm press-scale disabled:opacity-40 transition-all"
              >
                {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : confirmLabel}
              </button>
              <button
                onClick={() => !loading && onClose()}
                disabled={loading}
                className="w-full py-3 rounded-2xl bg-pax-control text-sm font-medium text-pax-muted press-scale disabled:opacity-30 transition-all"
              >
                {cancelLabel}
              </button>
            </motion.div>
          </ModalPanel>
        </div>
      )}
    </AnimatePresence>
  );
}

export { itemVariants as drawerItemVariants };
