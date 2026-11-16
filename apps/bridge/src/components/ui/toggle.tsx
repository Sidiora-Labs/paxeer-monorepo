import { cn } from '@/lib/utils';
import { Toggle as TogglePrimitive } from '@base-ui/react/toggle';
import { cva, type VariantProps } from 'class-variance-authority';

const toggleVariants = cva(
  'state-layer focus-ring inline-flex min-h-11 shrink-0 items-center justify-center gap-1.5 rounded-full text-body font-medium whitespace-nowrap text-muted-foreground transition-[background-color,color] duration-(--duration-fast) ease-(--ease-standard) outline-none select-none hover:text-foreground data-pressed:bg-secondary data-pressed:text-secondary-foreground data-pressed:before:opacity-0! data-disabled:pointer-events-none data-disabled:opacity-40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*=\'size-\'])]:size-4',
  {
    variants: {
      variant: {
        'default': 'bg-transparent',
        outline: 'border border-border bg-transparent data-pressed:border-transparent',
      },
      size: {
        sm: 'min-h-11 min-w-11 px-2.5',
        'default': 'min-h-11 min-w-11 px-3',
        lg: 'h-12 min-w-12 px-4',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  },
);

function Toggle({ className, variant, size, ...props }: TogglePrimitive.Props & VariantProps<typeof toggleVariants>) {
  return <TogglePrimitive data-slot="toggle" className={ cn(toggleVariants({ variant, size, className })) } { ...props }/>;
}

export { Toggle, toggleVariants };
