'use client';

import { cn } from '@/lib/utils';
import { MEDIA_RULES } from './media-validation';

export function MediaRuleLegend({ className }: { className?: string }) {
  return (
    <p className={cn('text-xs text-text-faint', className)}>
      {MEDIA_RULES.format} · {MEDIA_RULES.size} · {MEDIA_RULES.video} · {MEDIA_RULES.shape} · {MEDIA_RULES.resolution}
    </p>
  );
}
