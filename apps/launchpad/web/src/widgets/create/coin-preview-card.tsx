'use client';

import './create-text.css';
import { displayCoinText } from '@/utils/coin-text';
import { cn } from '@/lib/utils';
import { CoinCard } from '@/ui/primitives';
import { feeStrategyLabel } from './strategy-picker';
import type { FeeStrategy } from './use-create-prefill';

export interface CoinPreviewCardProps {
  creator?: string;
  name: string;
  symbol: string;
  description: string;
  image: string | null;
  strategy: FeeStrategy;
  socialLinks: readonly string[];
  className?: string;
}

function hostLabel(link: string): string {
  try {
    return new URL(link).hostname.replace(/^www\./, '');
  } catch {
    return link;
  }
}

export function CoinPreviewCard({ creator = 'Your wallet', name, symbol, description, image, strategy, socialLinks, className }: CoinPreviewCardProps) {
  const empty = !name.trim() && !symbol.trim() && !image;
  return (
    <section data-create-text aria-labelledby="create-preview-title" className={cn('flex flex-col gap-3', className)}>
      <h2 id="create-preview-title" className="text-md font-semibold text-text">Preview</h2>
      {empty ? (
        <div data-create-preview-frame className="flex flex-col items-center justify-center gap-2 rounded-lg bg-surface-1 p-4 text-center">
          <p className="text-sm text-text-muted">A preview of how your coin will look</p>
        </div>
      ) : (
        <div data-create-preview-frame className="pointer-events-none flex flex-col gap-3 rounded-lg bg-surface-1" aria-label="Coin card preview">
          <CoinCard
            image={image}
            name={displayCoinText(name, null) || 'Untitled coin'}
            symbol={displayCoinText(symbol, null) || 'TICKER'}
            mcapRaw={null}
            percentChange={null}
            createdAt={null}
            href="/create"
            creator={creator}
            description={description.trim()}
            className="[&_.line-clamp-2]:line-clamp-none [&_.line-clamp-2]:[overflow-wrap:anywhere]"
            surface="canvas"
            footerStats={[
              { label: 'Rewards to', value: feeStrategyLabel(strategy) },
              { label: 'Paired with', value: 'USDL' },
              { label: 'Rules', value: 'Standard' },
            ]}
          />
          {socialLinks.length > 0 ? (
            <ul className="flex flex-wrap gap-1.5">
              {socialLinks.map(link => (
                <li key={link} className="inline-flex min-h-badge-height-sm items-center rounded-pill bg-surface-2 px-2 text-xs text-text-muted">{hostLabel(link)}</li>
              ))}
            </ul>
          ) : null}
          <p className="text-2xs text-text-faint">Market cap shows up after the first trade</p>
        </div>
      )}
    </section>
  );
}
