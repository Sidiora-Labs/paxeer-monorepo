import { cn } from '@/lib/utils';
import * as React from 'react';

function Kbd({ className, ...props }: React.ComponentProps<'kbd'>) {
  return (
    <kbd
      data-slot="kbd"
      className={ cn(
        'bg-container-high text-muted-foreground pointer-events-none inline-flex h-5 min-w-5 items-center justify-center gap-1 rounded-xs px-1.5 font-sans text-caption font-medium select-none shadow-subtle [&_svg:not([class*=\'size-\'])]:size-3',
        className,
      ) }
      { ...props }
    />
  );
}

function KbdGroup({ className, ...props }: React.ComponentProps<'kbd'>) {
  return <kbd data-slot="kbd-group" className={ cn('inline-flex items-center gap-1', className) } { ...props }/>;
}

export { Kbd, KbdGroup };
