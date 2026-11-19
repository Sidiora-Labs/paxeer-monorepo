import { useLayoutEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";

export const useOrderBookAndTradesScript = (symbol: string) => {
  const [containerSize, setContainerSize] = useState<
    | {
        width: number;
        height: number;
      }
    | undefined
  >(undefined);

  const [tab, setTab] = useState<"orderBook" | "lastTrades">("orderBook");

  const containerRef = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        flushSync(() => setContainerSize({ width, height }));
      }
    });

    const targetDiv = containerRef.current;

    if (targetDiv) {
      const { width, height } = targetDiv.getBoundingClientRect();
      setContainerSize({ width, height });
      resizeObserver.observe(targetDiv);
    }

    return () => {
      if (targetDiv) {
        resizeObserver.unobserve(targetDiv);
      }
    };
  }, []);

  return {
    symbol,
    containerSize,
    containerRef: containerRef,
    tab,
    setTab,
  };
};

export type OrderBookAndTradesState = ReturnType<
  typeof useOrderBookAndTradesScript
>;
