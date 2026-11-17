import { cn } from '@/lib/utils';
import { Input as InputPrimitive } from '@base-ui/react/input';

function Input({ className, type, ...props }: InputPrimitive.Props) {
  return (
    <InputPrimitive
      type={ type }
      data-slot="input"
      className={ cn(
        'flex min-h-10 w-full min-w-0 rounded-md border border-input bg-transparent px-3.5 py-2 text-sm transition-[border-color,box-shadow,background-color] duration-(--duration-fast) ease-(--ease-standard) outline-none',
        'placeholder:text-muted-foreground selection:bg-primary selection:text-primary-foreground',
        'hover:border-foreground/30 focus-visible:border-foreground field-focus focus-ring',
        'file:text-foreground file:inline-flex file:min-h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium',
        'disabled:pointer-events-none disabled:cursor-not-allowed disabled:bg-container disabled:opacity-60',
        'aria-invalid:border-destructive aria-invalid:field-invalid data-invalid:border-destructive',
        className,
      ) }
      { ...props }
    />
  );
}

export { Input };
