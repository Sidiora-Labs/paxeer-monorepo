import { cn } from '@/lib/utils';
import { cva, type VariantProps } from 'class-variance-authority';
import * as React from 'react';

const alertVariants = cva(
  'relative grid w-full grid-cols-[0_minmax(0,1fr)] items-start gap-y-1 rounded-lg px-4 py-3.5 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_minmax(0,1fr)] has-[>svg]:gap-x-3 [&>svg]:size-4 [&>svg]:translate-y-0.5 [&>svg]:text-current',
  {
    variants: {
      variant: {
        'default': 'bg-container text-foreground',
        outline: 'border border-border text-foreground',
        destructive: 'bg-destructive-container text-destructive *:data-[slot=alert-description]:text-destructive',
        success: 'bg-success-container text-success *:data-[slot=alert-description]:text-success',
        warning: 'bg-warning-container text-warning *:data-[slot=alert-description]:text-warning',
        info: 'bg-info-container text-info *:data-[slot=alert-description]:text-info',
      },
    },
    defaultVariants: { variant: 'default' },
  },
);

function Alert({ className, variant, ...props }: React.ComponentProps<'div'> & VariantProps<typeof alertVariants>) {
  return <div data-slot="alert" role="alert" className={ cn(alertVariants({ variant }), className) } { ...props }/>;
}

function AlertTitle({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div data-slot="alert-title" className={ cn('col-start-2 min-h-4 min-w-0 break-words font-medium tracking-(--tracking-label)', className) } { ...props }/>
  );
}

function AlertDescription({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="alert-description"
      className={ cn('text-muted-foreground col-start-2 min-w-0 break-words grid justify-items-start gap-1 text-sm [&_p]:leading-relaxed', className) }
      { ...props }
    />
  );
}

export { Alert, AlertTitle, AlertDescription };
