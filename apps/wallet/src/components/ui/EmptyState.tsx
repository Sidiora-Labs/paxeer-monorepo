'use client';

import { motionEase } from '@/theme/motion';

import { motionDurations } from '@/theme/motion';

import { motion } from 'framer-motion';
import { SvgIcon } from '@/components/ui/SvgIcon';

type EmptyIcon =
  | 'inbox'
  | 'activity'
  | 'search'
  | 'contacts'
  | 'wallet'
  | 'swap'
  | 'bell'
  | 'chart';

const ICON_MAP: Record<EmptyIcon, { name: string }> = {
  inbox: { name: 'arrow-up-right' },
  activity: { name: 'activity' },
  search: { name: 'search' },
  contacts: { name: 'user' },
  wallet: { name: 'wallet' },
  swap: { name: 'refresh' },
  bell: { name: 'bell' },
  chart: { name: 'chart' },
};

interface EmptyStateProps {
  icon?: EmptyIcon;
  title: string;
  subtitle?: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  className?: string;
  compact?: boolean;
}

export function EmptyState({
  icon = 'inbox',
  title,
  subtitle,
  action,
  className = '',
  compact = false,
}: EmptyStateProps) {
  const ic = ICON_MAP[icon];

  return (
    <motion.div
      initial={{ opacity: 0, y: 'var(--space-2)' }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: motionDurations.quick, ease: motionEase }}
      className={`flex flex-col items-center justify-center text-center ${compact ? 'py-10' : 'py-16'} px-6 ${className}`}
    >
      <div
        className={`flex items-center justify-center rounded-2xl bg-pax-control mb-4 ${
          compact ? 'w-12 h-12' : 'w-16 h-16'
        }`}
      >
        <SvgIcon name={ic.name} className={compact ? 'w-5 h-5' : 'w-7 h-7'} />
      </div>

      <p className={`font-semibold text-pax-light ${compact ? 'text-sm' : 'text-base'}`}>{title}</p>

      {subtitle && (
        <p
          className={`text-pax-muted mt-1 max-w-xs leading-relaxed ${compact ? 'text-xs' : 'text-sm'}`}
        >
          {subtitle}
        </p>
      )}

      {action && (
        <button
          onClick={action.onClick}
          className="mt-5 px-5 py-2.5 rounded-xl bg-pax-accent text-pax-on-accent text-sm font-semibold press-scale transition-all hover:brightness-110"
        >
          {action.label}
        </button>
      )}
    </motion.div>
  );
}
