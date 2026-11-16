import { cn } from '@/lib/utils';
import * as React from 'react';

type CardProps = React.ComponentProps<'div'> & {

  /** `outline` = hairline on surface, `tonal` = filled container, `elevated` = floating */
  variant?: 'outline' | 'tonal' | 'elevated';
};

function Card({ className, variant = 'outline', ...props }: CardProps) {
  return (
    <div
      data-slot="card"
      data-variant={ variant }
      className={ cn(
        'bg-card text-card-foreground @container/card flex min-w-0 flex-col gap-6 rounded-xl py-6',
        variant === 'outline' && 'border border-border',
        variant === 'tonal' && 'bg-container',
        variant === 'elevated' && 'shadow-float',
        className,
      ) }
      { ...props }
    />
  );
}

function CardHeader({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-header"
      className={ cn(
        '@container/card-header grid auto-rows-min grid-rows-[auto_auto] items-start gap-1.5 surface-inset-x has-data-[slot=card-action]:grid-cols-[1fr_auto] [.border-b]:pb-6',
        className,
      ) }
      { ...props }
    />
  );
}

function CardTitle({ className, as: Heading = 'h3', ...props }: React.ComponentProps<'h3'> & { as?: 'h2' | 'h3' }) {
  return (
    <Heading
      data-slot="card-title"
      className={ cn('text-title-sm leading-tight font-medium tracking-(--tracking-label)', className) }
      { ...props }
    />
  );
}

function CardDescription({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div data-slot="card-description" className={ cn('text-muted-foreground text-sm', className) } { ...props }/>
  );
}

function CardAction({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="card-action"
      className={ cn('col-start-2 row-span-2 row-start-1 self-start justify-self-end', className) }
      { ...props }
    />
  );
}

function CardContent({ className, ...props }: React.ComponentProps<'div'>) {
  return <div data-slot="card-content" className={ cn('surface-inset-x', className) } { ...props }/>;
}

function CardFooter({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div data-slot="card-footer" className={ cn('flex items-center surface-inset-x [.border-t]:pt-6', className) } { ...props }/>
  );
}

export { Card, CardHeader, CardFooter, CardTitle, CardAction, CardDescription, CardContent };
