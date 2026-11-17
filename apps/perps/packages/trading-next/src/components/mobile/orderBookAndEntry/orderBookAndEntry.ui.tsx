import React, { useLayoutEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { cn } from "@paxeer-network/ui";
import { OrderEntryWidget } from "@paxeer-network/ui-order-entry";
import type { OrderBookAndEntryState } from "./orderBookAndEntry.script";

import { OrderBookWidget } from "../../base/orderBook";

export const OrderBookAndEntry: React.FC<
  OrderBookAndEntryState & { className?: string }
> = (props) => {
  const [height, setHeight] = useState(0);
  const divRef = useRef(null);

  useLayoutEffect(() => {
    const div = divRef.current;
    if (!div) {
      return;
    }
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        flushSync(() => setHeight(entry.contentRect.height));
      }
    });
    resizeObserver.observe(div);
    return () => {
      resizeObserver.unobserve(div);
    };
  }, []);

  const orderBookColumn = (
    <div
      className="oui-min-w-0 oui-rounded-xl oui-bg-base-9"
      style={{ height: `${height + 16}px` }}
    >
      <React.Suspense fallback={null}>
        <OrderBookWidget
          symbol={props.symbol}
          height={height ? height - 44 : undefined}
        />
      </React.Suspense>
    </div>
  );
  const orderEntryColumn = (
    <div className="oui-min-w-0 oui-rounded-xl oui-bg-base-9 oui-p-2">
      <OrderEntryWidget symbol={props.symbol} containerRef={divRef} />
    </div>
  );

  return (
    <div
      className={cn(
        "oui-trading-orderBookAndEntry oui-mx-1 oui-grid oui-gap-1",
        props.layout === "right"
          ? "oui-grid-cols-[4fr,6fr]"
          : "oui-grid-cols-[6fr,4fr]",
        props.className,
      )}
    >
      {props.layout === "right" ? (
        <>
          {orderBookColumn}
          {orderEntryColumn}
        </>
      ) : (
        <>
          {orderEntryColumn}
          {orderBookColumn}
        </>
      )}
    </div>
  );
};
