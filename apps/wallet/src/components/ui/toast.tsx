'use client';

import { motion } from 'framer-motion';
import { X, CheckCircle2, AlertTriangle, Info } from 'lucide-react';
import { cn } from '@/lib/cn';
import type { ToasterToast } from './use-toast';
import { motionDurations, springMotion } from '@/theme/motion';

interface ToastItemProps {
  toast: ToasterToast;
  onClose: () => void;
}

const VARIANT_STYLES: Record<NonNullable<ToasterToast['variant']>, string> = {
  default: 'bg-pax-overlay',
  success: 'bg-pax-success-surface',
  destructive: 'bg-pax-error-surface',
};

const VARIANT_ICON = {
  default: Info,
  success: CheckCircle2,
  destructive: AlertTriangle,
} as const;

const VARIANT_ICON_CLASS: Record<NonNullable<ToasterToast['variant']>, string> = {
  default: 'text-pax-accent',
  success: 'text-pax-success',
  destructive: 'text-pax-error',
};

export function ToastItem({ toast, onClose }: ToastItemProps) {
  const variant = toast.variant ?? 'default';
  const Icon = VARIANT_ICON[variant];

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 'calc(-1 * var(--space-3))', scale: 0.96 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{
        opacity: 0,
        x: 'var(--space-6)',
        scale: 0.96,
        transition: { duration: motionDurations.fast },
      }}
      transition={springMotion}
      className={cn(
        'pax-toast pointer-events-auto flex items-start gap-2.5 rounded-2xl px-3.5 py-3 shadow-menu backdrop-blur-sm',
        VARIANT_STYLES[variant]
      )}
      role={variant === 'destructive' ? 'alert' : 'status'}
    >
      <Icon className={cn('mt-0.5 w-4 h-4 shrink-0', VARIANT_ICON_CLASS[variant])} />
      <div className="flex-1 min-w-0 space-y-0.5">
        {toast.title && (
          <div className="text-sm font-semibold leading-tight text-pax-light">{toast.title}</div>
        )}
        {toast.description && (
          <div className="text-xs leading-snug text-pax-muted break-words">{toast.description}</div>
        )}
        {toast.action && <div className="pt-1">{toast.action}</div>}
      </div>
      <button
        type="button"
        onClick={onClose}
        aria-label="Dismiss notification"
        className="p-1 -mr-1 rounded-md text-pax-muted hover:text-pax-light hover:bg-pax-control transition-colors press-scale"
      >
        <X className="w-3.5 h-3.5" />
      </button>
    </motion.div>
  );
}
