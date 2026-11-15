import { cn } from '@/lib/utils';
import { Button as ButtonPrimitive } from '@base-ui/react/button';
import { useRender } from '@base-ui/react/use-render';
import { cva, type VariantProps } from 'class-variance-authority';
import * as React from 'react';

const buttonVariants = cva(
  'state-layer focus-ring inline-flex min-h-11 min-w-11 max-w-full items-center justify-center gap-2 rounded-full font-medium whitespace-normal select-none transition-[background-color,color,box-shadow,transform] duration-(--duration-fast) ease-(--ease-standard) active:not-disabled:scale-(--press-scale) disabled:pointer-events-none data-disabled:pointer-events-none disabled:opacity-40 data-disabled:opacity-40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*=\'size-\'])]:size-4',
  {
    variants: {
      variant: {
        'default':
          'bg-primary primary-fill text-primary-foreground [--state-hover:var(--state-primary-hover)] [--state-press:var(--state-primary-press)] [--state-focus:var(--state-primary-focus)]',
        tonal: 'bg-secondary text-secondary-foreground',
        outline: 'border border-border bg-transparent text-foreground',
        ghost: 'bg-transparent text-foreground',
        'protected':
          'bg-background/70 text-foreground shadow-subtle backdrop-blur-xl',
        destructive:
          'bg-destructive text-destructive-foreground [--state-hover:var(--state-primary-hover)] [--state-press:var(--state-primary-press)]',
        link: 'h-auto! rounded-xs px-0! text-muted-foreground underline-offset-(--underline-offset) before:hidden hover:text-foreground hover:underline',
      },
      size: {
        xs: 'min-h-11 py-2 px-3 text-label [&_svg:not([class*=\'size-\'])]:size-3.5',
        sm: 'min-h-11 py-2 px-3.5 text-body-sm',
        'default': 'min-h-11 py-2 px-5 text-body',
        lg: 'min-h-12 px-7 text-title-sm',
        icon: 'size-11',
        'icon-xs': 'size-11 [&_svg:not([class*=\'size-\'])]:size-3.5',
        'icon-sm': 'size-11',
        'icon-lg': 'size-12 [&_svg:not([class*=\'size-\'])]:size-5',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
);

type ButtonProps = ButtonPrimitive.Props & VariantProps<typeof buttonVariants>;

/** `render={<a />}` / `render={<Link />}` → not a native button, so Base UI skips button-only semantics. */
function isNativeButton(render: ButtonProps['render']) {
  if (!render || typeof render === 'function') return true;
  return React.isValidElement(render) && render.type === 'button';
}

function Button({ className, variant, size, render, nativeButton, ...props }: ButtonProps) {
  const link = useRender({
    defaultTagName: 'a', render,
    state: { disabled: props.disabled ?? false },
    props: { ...props, 'data-slot': 'button', 'data-variant': variant ?? 'default', className: cn(buttonVariants({ variant, size, className })) },
  });
  if (nativeButton === false || !isNativeButton(render)) return link;
  return (
    <ButtonPrimitive
      data-slot="button"
      render={ render }
      nativeButton={ nativeButton ?? isNativeButton(render) }
      data-variant={ variant ?? 'default' }
      className={ cn(buttonVariants({ variant, size, className })) }
      { ...props }
    />
  );
}

export { Button, buttonVariants, type ButtonProps };
