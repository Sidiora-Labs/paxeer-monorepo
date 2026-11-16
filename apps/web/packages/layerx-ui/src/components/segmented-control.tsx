"use client";

import * as React from "react";
import { cn } from "../lib/utils";

export interface SegmentedControlOption {
  value: string;
  label: React.ReactNode;
}

export interface SegmentedControlProps {
  options: SegmentedControlOption[];
  value: string;
  onValueChange: (value: string) => void;
  className?: string;
  size?: "sm" | "md";
  "aria-label"?: string;
}

/**
 * Gray track + white active thumb — the "Today / This week / This month"
 * recency switcher and "All / Fiat / Crypto" filter in the design set.
 */
export function SegmentedControl({
  options,
  value,
  onValueChange,
  className,
  size = "md",
  ...aria
}: SegmentedControlProps) {
  return (
    <div
      role="tablist"
      className={cn(
        "flex w-full items-center rounded-full bg-surface-sunken p-1",
        size === "sm" ? "min-h-9" : "min-h-11",
        className,
      )}
      {...aria}
    >
      {options.map((opt) => {
        const active = opt.value === value;
        return (
          <button
            key={opt.value}
            role="tab"
            aria-selected={active}
            type="button"
            onClick={() => onValueChange(opt.value)}
            className={cn(
              "flex min-h-11 min-w-0 flex-1 items-center justify-center rounded-full font-semibold whitespace-normal transition-colors outline-none focus:bg-accent-soft focus:text-accent-strong focus:ring-2 focus:ring-accent-strong focus-visible:bg-accent-soft focus-visible:text-accent-strong focus-visible:ring-2 focus-visible:ring-accent-strong",
              size === "sm" ? "px-3 py-1 text-label" : "px-4 py-2 text-sm",
              active
                ? "border border-border-strong bg-surface text-foreground shadow-sm"
                : "text-faint-foreground hover:text-muted-foreground",
            )}
          >
            {opt.label}
          </button>
        );
      })}
    </div>
  );
}
