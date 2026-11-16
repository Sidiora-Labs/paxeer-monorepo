import { cn } from '@/lib/utils';
import { mergeProps } from '@base-ui/react/merge-props';
import { useRender } from '@base-ui/react/use-render';
import { cva, type VariantProps } from 'class-variance-authority';
import type * as React from 'react';

const badgeVariants = cva(
  'inline-flex w-fit min-w-0 max-w-full items-center justify-center gap-1 rounded-full px-2.5 py-0.5 text-label font-medium whitespace-normal text-center transition-colors duration-(--duration-fast) [&>svg]:pointer-events-none [&>svg]:size-3',
  {
    variants: {
      variant: {
        'default': 'bg-primary primary-fill text-primary-foreground',
        tonal: 'bg-container-high text-foreground',
        outline: 'border border-border text-foreground',
        destructive: 'bg-destructive-container text-destructive',
        success: 'bg-success-container text-success',
        warning: 'bg-warning-container text-warning',
        info: 'bg-info-container text-info',
      },
    },
    defaultVariants: { variant: 'default' },
  },
);

type BadgeProps = useRender.ComponentProps<'span'> & VariantProps<typeof badgeVariants>;

function Badge({ className, variant, render, ...props }: BadgeProps) {
  return useRender({
    defaultTagName: 'span',
    render,
    props: mergeProps<'span'>(
      { className: cn(badgeVariants({ variant }), className), 'data-slot': 'badge' } as React.ComponentProps<'span'>,
      props,
    ),
  });
}

export { Badge, badgeVariants };
