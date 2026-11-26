'use client';
import { NoticeBar, UnavailableState } from '@/ui/primitives';

export function MarketReadNotice({ failed, retained, error, title, onRetry }: { failed: boolean; retained: boolean; error: Error | null; title: string; onRetry(): void }) {
  if (!failed) return null;
  if (retained) return <NoticeBar tone="warning">Prices may be out of date</NoticeBar>;
  const status = error && 'status' in error && typeof error.status === 'number' ? error.status : null;
  return <UnavailableState variant="inline" title={title} operation="IF_CANDLES" status={status} error={error} onRetry={onRetry} />;
}
