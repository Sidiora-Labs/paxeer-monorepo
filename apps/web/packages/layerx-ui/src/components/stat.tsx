import * as React from "react";
import { cn } from "../lib/utils";

/** Big number over a muted label — "18 / Total referrals". */
export function Stat({
  value,
  label,
  className,
  align = "center",
}: {
  value: React.ReactNode;
  label: React.ReactNode;
  className?: string;
  align?: "left" | "center";
}) {
  return (
    <div
      className={cn(
        "flex flex-col gap-1",
        align === "center" ? "items-center text-center" : "items-start",
        className,
      )}
    >
      <span className="max-w-full text-xl font-bold tabular-nums text-foreground">
        {value}
      </span>
      <span className="text-sm text-muted-foreground">{label}</span>
    </div>
  );
}

/** Paired statistics adapt their columns to the available width. */
export function StatPair({
  left,
  right,
  className,
}: {
  left: { value: React.ReactNode; label: React.ReactNode };
  right: { value: React.ReactNode; label: React.ReactNode };
  className?: string;
}) {
  return (
    <div
      className={cn(
        "grid grid-cols-[repeat(auto-fit,minmax(min(100%,var(--space-sidebar)),1fr))] gap-6",
        className,
      )}
    >
      <Stat value={left.value} label={left.label} />
      <Stat value={right.value} label={right.label} />
    </div>
  );
}
