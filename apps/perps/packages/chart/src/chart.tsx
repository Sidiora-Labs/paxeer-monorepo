import {
  PropsWithChildren,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { ChartRenderer } from "./chartRenderer";
import { DeafultMargin } from "./constants/config";
import { ChartProps, Size } from "./constants/types";
import { ChartContext, ChartContextState } from "./hooks/chartContext";
import { ChartScale, useChartState } from "./hooks/useChartState";

const Chart = <T,>(props: PropsWithChildren<ChartProps<T>>) => {
  const { margin = DeafultMargin, ...rest } = props;
  const wrapperRef = useRef<HTMLDivElement | null>(null);
  const [size, setSize] = useState<Size | undefined>(
    props.width && props.height
      ? { width: props.width, height: props.height }
      : undefined,
  );

  const [scale, setScale] = useState<ChartScale | undefined>();

  // set width and height
  useLayoutEffect(() => {
    const element = wrapperRef.current;
    if (!element || !!size) {
      return;
    }
    const updateSize = () => {
      const { width, height } = element.getBoundingClientRect();
      if (width > 0 && height > 0) {
        setSize({ width, height });
      }
    };
    if (typeof ResizeObserver === "undefined") {
      updateSize();
      return;
    }
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        if (entry.contentRect) {
          setSize({
            width: entry.contentRect.width,
            height: entry.contentRect.height,
          });
        }
      }
    });

    resizeObserver.observe(wrapperRef.current);

    return () => {
      resizeObserver.disconnect();
    };
  }, [size]);

  const memoizedValue = useMemo<ChartContextState>(() => {
    return {
      margin,
      size: size!,
      scale: scale!,
      data: props.data,
      registerScale: setScale,
    };
  }, [margin, size, scale, props.data, setScale]);

  return (
    <div ref={wrapperRef} className="oui-size-full">
      <ChartContext.Provider value={memoizedValue}>
        <ChartRenderer {...rest} />
      </ChartContext.Provider>
    </div>
  );
};

if (process.env.NODE_ENV !== "production") {
  Chart.displayName = "Chart";
}

export { Chart };
