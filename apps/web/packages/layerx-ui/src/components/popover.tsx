"use client";

import * as React from "react";
import * as PopoverPrimitive from "@radix-ui/react-popover";
import { cn } from "../lib/utils";

const Popover = PopoverPrimitive.Root;
const PopoverTrigger = PopoverPrimitive.Trigger;

/** Anchored popover surface — desktop filter chips, bell menu, etc. */
const PopoverContent = React.forwardRef<
  React.ElementRef<typeof PopoverPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof PopoverPrimitive.Content>
>(({ className, align = "start", sideOffset = 8, ...props }, ref) => (
  <PopoverContentFrame
    ref={ref}
    align={align}
    sideOffset={sideOffset}
    className={className}
    {...props}
  />
));
PopoverContent.displayName = "PopoverContent";

const PopoverContentFrame = React.forwardRef<
  React.ElementRef<typeof PopoverPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof PopoverPrimitive.Content>
>(({ className, ...props }, ref) => {
  React.useEffect(() => {
    const disableGuards = () => {
      document
        .querySelectorAll<HTMLElement>("[data-radix-focus-guard]")
        .forEach((guard) => {
          guard.tabIndex = -1;
        });
    };
    disableGuards();
    const timer = window.setTimeout(disableGuards, 0);
    const observer = new MutationObserver(disableGuards);
    observer.observe(document.body, { childList: true });
    return () => {
      window.clearTimeout(timer);
      observer.disconnect();
    };
  }, []);

  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Content
        ref={ref}
        className={cn(
          "z-50 w-auto min-w-popover rounded-lg border border-border bg-surface p-2 shadow-overlay outline-none",
          "data-[state=open]:animate-fade-in data-[state=closed]:animate-fade-out",
          className,
        )}
        {...props}
      />
    </PopoverPrimitive.Portal>
  );
});
PopoverContentFrame.displayName = "PopoverContentFrame";

export { Popover, PopoverTrigger, PopoverContent };
