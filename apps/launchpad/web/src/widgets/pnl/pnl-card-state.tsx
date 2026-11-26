'use client';

import { Clock, TriangleAlert } from 'lucide-react';
import { Button, EmptyState } from '@/ui/primitives';

type PnlCardStateProps = { state: 'expired' } | { state: 'failed'; cardId: string };

export function PnlCardState(props: PnlCardStateProps) {
  const expired = props.state === 'expired';
  return (
    <main className="mx-auto w-full min-w-0 max-w-content-narrow-max-width p-6 text-text">
      <EmptyState
        headingLevel={1}
        icon={expired ? Clock : TriangleAlert}
        title={expired ? 'This card is gone' : "Can't load this card right now"}
        body={expired ? 'It was removed or has expired' : 'Try again in a moment'}
        action={props.state === 'expired' ? (
          <Button href="/profile" variant="secondary">Go to your profile</Button>
        ) : (
          <Button href={`/pnl/${encodeURIComponent(props.cardId)}`} variant="secondary">Try again</Button>
        )}
        className="rounded-lg bg-surface-1 p-4 [&>div]:w-full [&>div]:min-w-0 [&>div]:max-w-full [&>div]:flex-wrap [&_h1]:break-words [&_h1]:leading-[calc(var(--sl-leading-md)/var(--sl-text-md))] [&_p]:break-words [&_p]:leading-[calc(var(--sl-leading-sm)/var(--sl-text-sm))]"
      />
      {expired ? null : <div className="mt-3 flex min-w-0 flex-wrap"><Button href="/profile" variant="ghost">Go to your profile</Button></div>}
    </main>
  );
}
