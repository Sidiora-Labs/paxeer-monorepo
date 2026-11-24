import * as React from "react";
import { Loader2 } from "lucide-react";
import { cn } from "../lib/utils";

export function Spinner({ className }: { className?: string }) {
  return (
    <span role="status" aria-label="Loading" className="inline-flex">
      <Loader2 className={cn("size-5 lx-spin text-muted-foreground", className)} aria-hidden />
    </span>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn("lx-pulse rounded-md bg-surface-sunken", className)} />;
}

/** List-row shaped skeleton for loading lists. */
export function SkeletonRow() {
  return (
    <div className="flex items-center gap-3 py-3.5">
      <Skeleton className="size-11 rounded-full" />
      <div className="flex flex-1 flex-col gap-2">
        <Skeleton className="h-3.5 w-1/3" />
        <Skeleton className="h-3 w-1/4" />
      </div>
      <Skeleton className="h-3.5 w-16" />
    </div>
  );
}
