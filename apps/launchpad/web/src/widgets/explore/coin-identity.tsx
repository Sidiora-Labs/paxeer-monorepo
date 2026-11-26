'use client';

import type { ReactNode } from 'react';
import { AgeChip, Avatar, StageBadge, type Stage } from '@/ui/primitives';

export interface CoinIdentityProps {
  image: string | null;
  name: string;
  symbol: string;
  createdAt?: number | null;
  stage?: Stage | null;
  children?: ReactNode;
}

export function CoinIdentity({ image, name, symbol, createdAt, stage, children }: CoinIdentityProps) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-3">
      <Avatar src={image} name={name} size="md" shape="square" decorative
        badge={stage ? <StageBadge stage={stage} size={14} decorative surface={1} /> : undefined} />
      <div className="min-w-0 flex-1">
        {children ?? <span className="block truncate text-sm font-semibold text-text" title={name}>{name}</span>}
        <span className="flex min-w-0 items-center gap-2 text-xs text-text-muted">
          <span className="truncate" title={symbol}>${symbol}</span>
          {createdAt !== undefined ? <AgeChip iso={createdAt} className="shrink-0" /> : null}
        </span>
      </div>
    </div>
  );
}
