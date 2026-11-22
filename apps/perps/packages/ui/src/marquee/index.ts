import { createElement } from "react";
import { Marquee as MarqueeView, type MarqueeProps } from "./marquee";

const EMPTY_CAROUSEL_OPTIONS: MarqueeProps["carouselOptions"] = {};
const EMPTY_AUTO_SCROLL_OPTIONS: MarqueeProps["autoScrollOptions"] = {};

export function Marquee<T>(props: MarqueeProps<T>) {
  return createElement(MarqueeView<T>, {
    ...props,
    carouselOptions:
      props.carouselOptions === undefined
        ? EMPTY_CAROUSEL_OPTIONS
        : props.carouselOptions,
    autoScrollOptions:
      props.autoScrollOptions === undefined
        ? EMPTY_AUTO_SCROLL_OPTIONS
        : props.autoScrollOptions,
  });
}

export type { MarqueeProps } from "./marquee";
