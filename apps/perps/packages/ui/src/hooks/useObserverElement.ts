import { useLayoutEffect } from "react";
import { flushSync } from "react-dom";

/**
 * observe the element and call the callback when the element is changed
 * */
export function useObserverElement<T extends HTMLElement>(
  element: T | null,
  callback: (entry: ResizeObserverEntry) => void,
) {
  useLayoutEffect(() => {
    if (!element) {
      return;
    }

    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        flushSync(() => callback(entry));
      }
    });

    resizeObserver.observe(element);

    return () => {
      resizeObserver.unobserve(element);
    };
  }, [element]);
}
