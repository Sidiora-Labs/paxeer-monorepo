"use client";

import * as React from "react";
import { cn } from "../lib/utils";

export interface QuickAction {
  id: string;
  label: string;
  icon: React.ReactNode;
}

/**
 * Circle icon + label grid ("Send / Receive / Swap / Pay bills").
 * Circles use a hairline border on white, per the design set.
 */
export function QuickActions({
  actions,
  onAction,
  className,
}: {
  actions: QuickAction[];
  onAction?: (id: string) => void;
  className?: string;
}) {
  const columns = ["grid-cols-1", "grid-cols-2", "grid-cols-3", "grid-cols-4", "grid-cols-5"][
    Math.min(Math.max(actions.length, 1), 5) - 1
  ];
  return (
    <div
      className={cn("grid gap-2", columns, className)}
    >
      {actions.map((a) => (
        <button
          key={a.id}
          type="button"
          onClick={() => onAction?.(a.id)}
          className="group flex flex-col items-center gap-2 outline-none"
        >
          <span className="inline-flex size-12 items-center justify-center rounded-full border border-border bg-surface text-foreground transition-colors group-hover:bg-surface-sunken group-focus-visible:ring-2 group-focus-visible:ring-accent-strong [&_svg]:size-5">
            {a.icon}
          </span>
          <span className="text-label font-medium text-foreground-secondary">
            {a.label}
          </span>
        </button>
      ))}
    </div>
  );
}
