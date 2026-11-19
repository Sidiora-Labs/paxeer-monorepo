"use client";

import * as React from "react";
import { Slot, Slottable } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { Loader2 } from "lucide-react";
import { cn } from "../lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 min-w-0 max-w-full whitespace-normal text-center font-semibold transition-colors select-none outline-none focus-visible:ring-2 focus-visible:ring-accent-strong disabled:pointer-events-none disabled:opacity-40 [&_svg]:pointer-events-none [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        /** Signature solid black pill. */
        primary:
          "bg-primary text-primary-foreground hover:bg-primary-hover active:bg-primary-hover",
        /** White pill with a hairline border — pairs with primary in footers. */
        secondary:
          "bg-surface text-foreground border border-border-strong hover:bg-surface-sunken/60",
        /** Light gray filled pill. */
        soft: "border border-border-strong bg-surface-sunken text-foreground hover:bg-border/60",
        /** Blue pill — for accent CTAs. */
        accent: "bg-accent text-accent-foreground hover:bg-accent-strong",
        /** Solid red pill for irreversible actions. */
        destructive:
          "bg-destructive text-destructive-foreground hover:bg-destructive/90",
        /** Borderless. */
        ghost: "text-foreground hover:bg-surface-sunken",
        /** Text-only accent link. */
        link: "text-accent underline-offset-4 hover:underline h-auto px-0",
      },
      size: {
        sm: "min-h-9 px-4 text-sm rounded-full [&_svg]:size-4",
        md: "min-h-11 px-6 text-body-sm rounded-full [&_svg]:size-icon-sm",
        lg: "[--control-min-height:var(--space-button-lg)] min-h-button-lg px-7 text-base rounded-full [&_svg]:size-5",
        icon: "size-11 rounded-full [&_svg]:size-5",
      },
      fullWidth: {
        true: "w-full",
        false: "",
      },
    },
    defaultVariants: { variant: "primary", size: "md", fullWidth: false },
  },
);

export interface ButtonProps
  extends
    React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
  loading?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant,
      size,
      fullWidth,
      asChild = false,
      loading,
      children,
      disabled,
      type = "button",
      ...props
    },
    ref,
  ) => {
    const Comp = asChild ? Slot : "button";
    const isDisabled = Boolean(disabled || loading);
    const blockInteraction = (event: React.SyntheticEvent) => {
      event.preventDefault();
      event.stopPropagation();
    };
    const disabledSlotProps =
      asChild && isDisabled
        ? {
            disabled: true,
            "aria-disabled": true as const,
            tabIndex: -1,
            onClick: blockInteraction,
            onClickCapture: blockInteraction,
            onKeyDown: blockInteraction,
            onKeyDownCapture: blockInteraction,
          }
        : {};
    const child =
      asChild && isDisabled
        ? React.cloneElement(
            React.Children.only(children) as React.ReactElement<
              React.HTMLAttributes<HTMLElement>
            >,
            disabledSlotProps,
          )
        : children;
    return (
      <Comp
        ref={ref}
        disabled={isDisabled}
        type={type}
        aria-busy={loading || undefined}
        className={cn(buttonVariants({ variant, size, fullWidth, className }))}
        {...props}
        {...disabledSlotProps}
      >
        {loading && <Loader2 className="lx-spin" aria-hidden />}
        {asChild ? <Slottable>{child}</Slottable> : children}
      </Comp>
    );
  },
);
Button.displayName = "Button";

export { Button, buttonVariants };

/* -------------------------------------------------------------------------- */

const iconButtonVariants = cva(
  "inline-flex items-center justify-center rounded-full transition-colors outline-none focus-visible:ring-2 focus-visible:ring-accent-strong disabled:pointer-events-none disabled:opacity-40 [&_svg]:size-5",
  {
    variants: {
      variant: {
        /** White circle with hairline border — the design set's header buttons. */
        outline:
          "bg-surface border border-border-strong text-foreground hover:bg-surface-sunken/60",
        soft: "border border-border-strong bg-surface-sunken text-foreground hover:bg-border/60",
        ghost: "text-foreground hover:bg-surface-sunken",
        accent: "bg-accent text-accent-foreground hover:bg-accent-strong",
        primary: "bg-primary text-primary-foreground hover:bg-primary-hover",
      },
      size: {
        sm: "size-9 [&_svg]:size-4",
        md: "size-11",
        lg: "size-14 [&_svg]:size-6",
      },
    },
    defaultVariants: { variant: "outline", size: "md" },
  },
);

export interface IconButtonProps
  extends
    React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof iconButtonVariants> {}

const IconButton = React.forwardRef<HTMLButtonElement, IconButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button
      ref={ref}
      className={cn(iconButtonVariants({ variant, size, className }))}
      {...props}
    />
  ),
);
IconButton.displayName = "IconButton";

export { IconButton, iconButtonVariants };
