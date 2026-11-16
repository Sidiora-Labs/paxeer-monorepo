"use client";

import * as React from "react";
import * as SwitchPrimitive from "@radix-ui/react-switch";
import { cn } from "../lib/utils";

/** iOS-style switch used across settings sheets in the design set. */
const Switch = React.forwardRef<
  React.ElementRef<typeof SwitchPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>
>(({ className, ...props }, ref) => (
  <SwitchPrimitive.Root
    ref={ref}
    className={cn(
      "peer inline-flex h-switch w-switch shrink-0 cursor-pointer items-center rounded-full border border-transparent transition-colors outline-none",
      "bg-border-strong data-[state=checked]:bg-accent",
      "focus:ring-2 focus:ring-accent-strong focus-visible:ring-2 focus-visible:ring-accent-strong disabled:cursor-not-allowed disabled:opacity-50",
      className,
    )}
    {...props}
  >
    <SwitchPrimitive.Thumb
      className={cn(
        "pointer-events-none block size-switch-thumb rounded-full bg-surface shadow-sm ring-0 transition-transform",
        "translate-x-switch-inset data-[state=checked]:translate-x-switch",
      )}
    />
  </SwitchPrimitive.Root>
));
Switch.displayName = "Switch";

export { Switch };
