import { cn } from '@/lib/utils';
import type * as React from 'react';

function Spinner({ className, ...props }: React.ComponentProps<'progress'>) {
  return <progress aria-label="Loading" data-slot="spinner" className={ cn('bridge-spinner inline-block size-4 shrink-0 rounded-full', className) } { ...props }/>;
}

export { Spinner };
