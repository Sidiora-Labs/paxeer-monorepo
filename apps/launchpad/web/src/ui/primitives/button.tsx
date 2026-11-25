'use client';

import type { ButtonHTMLAttributes, ReactNode } from 'react';
import Link from 'next/link';
import { LoaderCircle } from 'lucide-react';
import { cn } from '@/lib/utils';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'danger-text' | 'inverse' | 'link';
export type ControlSize = 'sm' | 'md' | 'lg';

const sizeClass: Record<ControlSize, string> = {
  sm: 'min-h-control-height-sm px-3 py-1 text-sm gap-1.5',
  md: 'min-h-control-height-md px-4 py-1 text-base gap-2',
  lg: 'min-h-control-height-lg px-5 py-1 text-base gap-2',
};

const edgePaddingClass: Record<ControlSize, string> = {
  sm: 'px-[calc(var(--sl-space-3)-var(--sl-edge-width))]',
  md: 'px-[calc(var(--sl-space-4)-var(--sl-edge-width))]',
  lg: 'px-[calc(var(--sl-space)*5-var(--sl-edge-width))]',
};

const variantClass: Record<ButtonVariant, string> = {
  inverse:
    'bg-surface-inverse text-canvas hover:bg-text-muted active:bg-text-faint disabled:text-text-disabled aria-disabled:text-text-disabled',
  primary:
    'bg-accent text-on-accent hover:bg-accent-hover active:bg-accent-active disabled:bg-accent-soft-strong disabled:text-text-disabled aria-disabled:bg-accent-soft-strong aria-disabled:text-text-disabled',
  secondary:
    'border border-control-edge bg-surface-2 text-text hover:bg-surface-hover active:bg-surface-active disabled:text-text-disabled aria-disabled:text-text-disabled',
  ghost:
    'border border-transparent bg-transparent text-text-muted hover:border-control-edge hover:bg-surface-hover hover:text-text active:bg-surface-active disabled:text-text-disabled aria-disabled:text-text-disabled',
  danger:
    'bg-danger text-on-danger hover:bg-danger-strong active:bg-danger disabled:text-text-disabled aria-disabled:text-text-disabled',
  'danger-text':
    'bg-surface-2 text-danger hover:bg-surface-hover active:bg-surface-active disabled:text-text-disabled aria-disabled:text-text-disabled',
  link: 'bg-transparent px-0 text-accent hover:underline disabled:text-text-disabled aria-disabled:text-text-disabled h-auto',
};

export interface ButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children'> {
  variant?: ButtonVariant;
  size?: ControlSize;
  fullWidth?: boolean;
  pill?: boolean;
  loading?: boolean;
  icon?: ReactNode;
  href?: string;
  children: ReactNode;
}

export function buttonClass(opts: {
  variant?: ButtonVariant;
  size?: ControlSize;
  fullWidth?: boolean;
  pill?: boolean;
  className?: string;
}) {
  const { variant = 'secondary', size = 'md', fullWidth, pill, className } = opts;
  return cn(
    'relative inline-flex max-w-full select-none items-center justify-center text-center font-bold transition-[background-color] duration-fast',
    variant === 'link' ? 'flex-wrap' : 'min-w-0 whitespace-nowrap',
    'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus',
    'disabled:cursor-not-allowed aria-disabled:cursor-not-allowed aria-busy:cursor-progress',
    pill ? 'rounded-pill' : 'rounded-sm',
    sizeClass[size],
    variantClass[variant],
    (variant === 'secondary' || variant === 'ghost') &&
      [edgePaddingClass[size], 'py-[calc(var(--sl-space-1)-var(--sl-edge-width))]'],
    fullWidth && 'w-full',
    className
  );
}

export function Button({
  variant = 'secondary',
  size = 'md',
  fullWidth,
  pill,
  loading = false,
  icon,
  href,
  className,
  children,
  disabled,
  type = 'button',
  ...rest
}: ButtonProps) {
  const classes = buttonClass({ variant, size, fullWidth, pill, className });
  const leading =
    loading && icon ? (
      <LoaderCircle aria-hidden="true" className="size-icon-md animate-spin" />
    ) : icon ? (
      <span
        aria-hidden="true"
        className="inline-flex size-icon-md shrink-0 items-center justify-center [&>svg]:size-full"
      >
        {icon}
      </span>
    ) : null;

  if (href && !disabled && !loading) {
    return (
      <Link prefetch={false} href={href} className={classes} aria-disabled={disabled || undefined}>
        {leading}
        <span className={variant === 'link' ? undefined : 'min-w-0 max-w-full truncate'}>{children}</span>
      </Link>
    );
  }

  return (
    <button
      type={type}
      className={classes}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {leading}
      {loading && !icon ? (
        <LoaderCircle aria-hidden="true" className="absolute size-icon-md animate-spin" />
      ) : null}
      <span className={cn(variant !== 'link' && 'min-w-0 max-w-full truncate', loading && !icon && 'opacity-0')}>{children}</span>
    </button>
  );
}

export type IconButtonVariant = 'ghost' | 'raised' | 'round' | 'on-media';

export interface IconButtonProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'children' | 'aria-label'> {
  label: string;
  icon: ReactNode;
  variant?: IconButtonVariant;
  size?: ControlSize;
  pressed?: boolean;
  loading?: boolean;
}

const iconSizeClass: Record<ControlSize, string> = {
  sm: 'size-control-height-sm [&_svg]:size-icon-sm',
  md: 'size-control-height-md [&_svg]:size-icon-md',
  lg: 'size-control-height-lg [&_svg]:size-icon-lg',
};

const iconVariantClass: Record<IconButtonVariant, string> = {
  ghost:
    'border border-transparent bg-transparent text-text-muted hover:border-control-edge hover:bg-surface-hover hover:text-text active:bg-surface-active rounded-sm',
  raised:
    'border border-control-edge bg-surface-2 text-text hover:bg-surface-hover active:bg-surface-active rounded-sm',
  round:
    'border border-control-edge bg-surface-2 text-text hover:bg-surface-hover active:bg-surface-active rounded-pill',
  'on-media': 'bg-media-overlay text-on-media hover:bg-scrim active:bg-scrim rounded-sm',
};

export function IconButton({
  label,
  icon,
  variant = 'ghost',
  size = 'md',
  pressed,
  loading = false,
  className,
  disabled,
  type = 'button',
  ...rest
}: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      aria-busy={loading || undefined}
      disabled={disabled || loading}
      className={cn(
        'inline-flex shrink-0 items-center justify-center transition-colors duration-fast',
        'disabled:cursor-not-allowed disabled:text-text-disabled aria-disabled:text-text-disabled',
        iconSizeClass[size],
        iconVariantClass[variant],
        pressed && 'bg-accent-soft text-accent hover:bg-accent-soft-strong',
        className
      )}
      {...rest}
    >
      {loading ? (
        <LoaderCircle aria-hidden="true" className="animate-spin" />
      ) : (
        <span aria-hidden="true" className="inline-flex items-center justify-center">
          {icon}
        </span>
      )}
    </button>
  );
}
