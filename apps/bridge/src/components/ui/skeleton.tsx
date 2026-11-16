import { cn } from '@/lib/utils';
import * as React from 'react';

function Skeleton({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="skeleton"
      className={ cn(
        'relative overflow-hidden rounded-md bg-container-high',
        'after:absolute after:inset-0 after:-translate-x-full after:animate-[dx-shimmer_var(--duration-loading)_var(--ease-standard)_infinite] after:bg-linear-to-r after:from-transparent after:via-background/60 after:to-transparent',
        className,
      ) }
      { ...props }
    >
    </div>
  );
}

export { Skeleton };
