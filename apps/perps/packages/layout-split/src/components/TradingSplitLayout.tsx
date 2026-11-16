/**
 * TradingSplitLayout - @uiw/react-split wrapper for trading layout.
 *
 * Ported from packages/trading-next/src/components/desktop/layout/splitLayout/.
 * Keeps all layout logic inside layout-split; trading-next only provides panels.
 */
import React, { forwardRef, HTMLAttributes, PropsWithChildren } from "react";
import Split, { SplitProps } from "@uiw/react-split";
import { cn } from "@paxeer-network/ui";

// ─── Split line bar ───────────────────────────────────────────────────────────

type SplitLineBarProps = Pick<SplitProps, "mode"> &
  HTMLAttributes<HTMLDivElement>;

/**
 * Styled drag handle for TradingSplitLayout.
 * Transparent by default; primary-light highlight on hover/active/focus.
 */
const SplitLineBar: React.FC<SplitLineBarProps> = (props) => {
  const { onMouseDown, mode = "horizontal", className, ...rest } = props;
  /** When the split container adds the "disable" class token, make handle non-interactive. */
  const tokens = className?.split(/\s+/).filter(Boolean) ?? [];
  const disable = tokens.includes("disable");
  const filteredCls = tokens.filter((t) => t !== "disable").join(" ");

  return (
    <div
      {...rest}
      className={cn(
        filteredCls || undefined,
        "!oui-transition-none",
        "!oui-bg-transparent !oui-shadow-none",
        "hover:!oui-bg-primary-light hover:!oui-shadow-sm hover:!oui-shadow-primary-light/80",
        "active:!oui-bg-primary-light active:!oui-shadow-sm active:!oui-shadow-primary-light/80",
        "focus:!oui-bg-primary-light focus:!oui-shadow-sm focus:!oui-shadow-primary-light/80",
        mode === "horizontal"
          ? "!oui-mx-1 !oui-w-0.5 !oui-min-w-0.5"
          : "!oui-my-1 !oui-h-0.5 !oui-min-h-0.5",
        disable && "oui-pointer-events-none",
      )}
    >
      <div
        onMouseDown={onMouseDown}
        className={cn(
          "!oui-transition-none",
          mode === "horizontal" ? "after:!oui-w-0.5" : "after:!oui-h-0.5",
          "after:!oui-bg-transparent after:!oui-shadow-transparent",
        )}
      />
    </div>
  );
};

// ─── TradingSplitLayout ───────────────────────────────────────────────────────

type TradingSplitLayoutProps = PropsWithChildren<SplitProps> & {
  /** Called with the pixel width/height string of the first panel on drag end. */
  onSizeChange?: (size: string) => void;
};

/**
 * Thin forwardRef wrapper around @uiw/react-split with a styled SplitLineBar.
 * API matches the internal splitLayout used in trading.ui.desktop.tsx.
 */
export const TradingSplitLayout = forwardRef<Split, TradingSplitLayoutProps>(
  (props, ref) => {
    const { onSizeChange, ...rest } = props;
    return (
      <Split
        ref={ref}
        {...rest}
        lineBar
        renderBar={(barProps) => (
          <SplitLineBar {...barProps} mode={props.mode} />
        )}
        onDragEnd={(_, width) => onSizeChange?.(`${width}`)}
      />
    );
  },
);

TradingSplitLayout.displayName = "TradingSplitLayout";
