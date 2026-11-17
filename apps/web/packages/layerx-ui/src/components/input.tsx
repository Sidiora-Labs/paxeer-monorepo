"use client";

import * as React from "react";
import { Search, X } from "lucide-react";
import { cn } from "../lib/utils";

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  error?: boolean;
  /** Optional leading adornment (icon, flag, "+1" etc). */
  leading?: React.ReactNode;
  trailing?: React.ReactNode;
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, error, leading, trailing, ...props }, ref) => {
    return (
      <div
        className={cn(
          "flex h-12 items-center gap-2 rounded-md border bg-surface px-4 py-0 transition-colors",
          error
            ? "border-destructive focus-within:ring-2 focus-within:ring-destructive/25"
            : "border-border-strong focus-within:border-accent focus-within:ring-2 focus-within:ring-accent/20",
          props.disabled && "opacity-50",
          className,
        )}
      >
        {leading}
        <input
          ref={ref}
          className="min-h-11 w-full flex-1 min-w-0 bg-transparent text-body-sm text-foreground outline-none placeholder:text-faint-foreground"
          {...props}
          aria-invalid={error || props["aria-invalid"] || undefined}
        />
        {trailing}
      </div>
    );
  },
);
Input.displayName = "Input";

export { Input };

/* -------------------------------------------------------------------------- */

export interface SearchInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  onClear?: () => void;
}

/** Pill search field, as used in the home header and asset list. */
const SearchInput = React.forwardRef<HTMLInputElement, SearchInputProps>(
  ({ className, value, onClear, ...props }, ref) => {
    const hasValue = value !== undefined ? String(value).length > 0 : false;
    return (
      <div
        className={cn(
          "flex min-h-11 items-center gap-2.5 rounded-full bg-surface border border-border-strong px-4 py-0 transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-accent/20",
          className,
        )}
      >
        <Search
          className="size-icon-sm shrink-0 text-muted-foreground"
          aria-hidden
        />
        <input
          ref={ref}
          type="search"
          value={value}
          className="min-h-10 w-full min-w-0 bg-transparent text-body-sm text-foreground outline-none placeholder:text-faint-foreground [&::-webkit-search-cancel-button]:hidden"
          {...props}
        />
        {hasValue && onClear && (
          <button
            type="button"
            onClick={onClear}
            aria-label="Clear search"
            className="inline-flex size-4 shrink-0 items-center justify-center rounded-full text-faint-foreground hover:text-muted-foreground"
          >
            <X className="size-4" />
          </button>
        )}
      </div>
    );
  },
);
SearchInput.displayName = "SearchInput";

export { SearchInput };
