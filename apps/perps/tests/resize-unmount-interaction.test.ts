import assert from "node:assert/strict";
import { test } from "node:test";
import * as React from "react";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "https://gateway.test";
process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = "https://paxscan.test";
process.env.NEXT_PUBLIC_PAXSCAN_API_URL = "https://paxscan.test/api";
process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = "wss://paxscan.test/socket";

// Task 22.33.83: resize-unmount-state is proved by the test below.
// No out-of-path wiring is required. The bounded gate order forbids a baseline run.
async function resize_unmount_state_restores_page_interaction() {
  const { JSDOM } = await import("../src/ui/atoms/test-support/dom");
  const dom = new JSDOM("<!doctype html><html><body></body></html>", {
    url: "https://perps.test",
    pretendToBeVisual: true,
  });
  const globals = {
    window: dom.window,
    document: dom.window.document,
    navigator: dom.window.navigator,
    HTMLElement: dom.window.HTMLElement,
    HTMLIFrameElement: dom.window.HTMLIFrameElement,
    Event: dom.window.Event,
    MouseEvent: dom.window.MouseEvent,
    TouchEvent: dom.window.TouchEvent,
    requestAnimationFrame: dom.window.requestAnimationFrame.bind(dom.window),
    cancelAnimationFrame: dom.window.cancelAnimationFrame.bind(dom.window),
    React,
    IS_REACT_ACT_ENVIRONMENT: true,
  };
  const previousGlobals = new Map(
    Object.keys(globals).map((key) => [
      key,
      Object.getOwnPropertyDescriptor(globalThis, key),
    ]),
  );
  Object.entries(globals).forEach(([key, value]) => {
    Object.defineProperty(globalThis, key, { configurable: true, value });
  });

  try {
    const { createRoot } = await import("react-dom/client");
    const { ResizeHandle } = await import("../src/ui/atoms/resize-handle");
    const frame = () =>
      new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

    for (const direction of ["horizontal", "vertical"] as const) {
      for (const styled of [false, true]) {
        const body = document.body;
        body.style.cssText = styled
          ? "cursor: crosshair !important; user-select: text !important; -webkit-user-select: text !important; color: red;"
          : "";
        const chart = document.createElement("iframe");
        const disabledChart = document.createElement("iframe");
        if (styled)
          chart.style.setProperty("pointer-events", "auto", "important");
        disabledChart.style.setProperty("pointer-events", "none", "important");
        body.append(chart, disabledChart);
        const properties: [CSSStyleDeclaration, string][] = [
          [body.style, "cursor"],
          [body.style, "user-select"],
          [body.style, "-webkit-user-select"],
          [chart.style, "pointer-events"],
          [disabledChart.style, "pointer-events"],
        ];
        const saved = properties.map(([style, property]) => [
          style.getPropertyValue(property),
          style.getPropertyPriority(property),
        ]);
        const assertRestored = () => {
          assert.deepEqual(
            properties.map(([style, property]) => [
              style.getPropertyValue(property),
              style.getPropertyPriority(property),
            ]),
            saved,
            "body selection/cursor and each iframe's prior value and priority are restored",
          );
          assert.equal(body.style.color, styled ? "red" : "");
        };

        const mount = async () => {
          const container = document.createElement("div");
          body.append(container);
          const root = createRoot(container);
          const targetRef = React.createRef<HTMLDivElement>();
          const committed: number[] = [];
          await React.act(async () => {
            root.render(
              React.createElement(
                "div",
                null,
                React.createElement("div", { ref: targetRef }),
                React.createElement(ResizeHandle, {
                  direction,
                  targetRef,
                  className: "test-resize-handle",
                  onResizeEnd: (size) => committed.push(size),
                }),
              ),
            );
          });
          const handle = container.querySelector<HTMLDivElement>(
            ".test-resize-handle",
          );
          const target = targetRef.current;
          assert.ok(handle);
          assert.ok(target);
          const mouse = (
            type: "mousedown" | "mousemove" | "mouseup",
            pos: number,
          ) => {
            (type === "mousedown" ? handle : window).dispatchEvent(
              new MouseEvent(type, {
                bubbles: true,
                cancelable: true,
                clientX: direction === "horizontal" ? pos : 0,
                clientY: direction === "vertical" ? pos : 0,
              }),
            );
          };
          const touch = (
            type: "touchstart" | "touchmove" | "touchend" | "touchcancel",
            pos: number,
          ) => {
            const point: Touch = {
              identifier: 1,
              target: handle,
              clientX: direction === "horizontal" ? pos : 0,
              clientY: direction === "vertical" ? pos : 0,
              screenX: direction === "horizontal" ? pos : 0,
              screenY: direction === "vertical" ? pos : 0,
              pageX: direction === "horizontal" ? pos : 0,
              pageY: direction === "vertical" ? pos : 0,
              radiusX: 1,
              radiusY: 1,
              rotationAngle: 0,
              force: 1,
            };
            (type === "touchstart" ? handle : window).dispatchEvent(
              new TouchEvent(type, {
                bubbles: true,
                touches:
                  type === "touchend" || type === "touchcancel" ? [] : [point],
                changedTouches: [point],
              }),
            );
          };
          return {
            handle,
            target,
            committed,
            mouse,
            touch,
            async unmount() {
              await React.act(async () => root.unmount());
              container.remove();
            },
          };
        };

        for (const input of ["mouse", "touch"] as const) {
          const fixture = await mount();
          try {
            if (input === "mouse") {
              fixture.mouse("mousedown", 100);
              fixture.mouse("mousedown", 100);
              fixture.mouse("mousemove", 140);
            } else {
              fixture.touch("touchstart", 100);
              fixture.touch("touchmove", 140);
            }
            assert.equal(body.style.userSelect, "none");
            assert.equal(body.style.webkitUserSelect, "none");
            assert.equal(
              body.style.cursor,
              direction === "horizontal" ? "col-resize" : "row-resize",
            );
            assert.equal(chart.style.pointerEvents, "none");
            assert.equal(
              fixture.handle.classList.contains("resize-active"),
              true,
            );
            const lateChart = document.createElement("iframe");
            lateChart.style.pointerEvents = "auto";
            body.append(lateChart);
            try {
              await fixture.unmount();
              assertRestored();
              assert.equal(
                lateChart.style.pointerEvents,
                "auto",
                "cleanup only restores captured frames",
              );
              assert.equal(
                fixture.handle.classList.contains("resize-active"),
                false,
              );
              await frame();
              fixture.mouse("mousemove", 190);
              fixture.mouse("mouseup", 190);
              fixture.touch("touchend", 190);
              assert.equal(fixture.target.style.width, "");
              assert.equal(fixture.target.style.height, "");
              assert.deepEqual(
                fixture.committed,
                [],
                "navigation cancels without committing to an unmounted owner",
              );
              assertRestored();
            } finally {
              lateChart.remove();
            }
          } finally {
            // React permits repeated unmount; this also releases the fixture on assertion failure.
            await fixture.unmount();
          }

          const reopened = await mount();
          try {
            for (const ending of [
              "mouseup",
              "touchend",
              "touchcancel",
            ] as const) {
              const count = reopened.committed.length;
              if (ending === "mouseup") {
                reopened.mouse("mousedown", 100);
                reopened.mouse("mousemove", 140);
                reopened.mouse(ending, 140);
              } else {
                reopened.touch("touchstart", 100);
                reopened.touch("touchmove", 140);
                reopened.touch(ending, 140);
              }
              assert.equal(
                reopened.target.style[
                  direction === "horizontal" ? "width" : "height"
                ],
                "40px",
                "release applies the pending movement",
              );
              assert.equal(reopened.committed.length, count + 1);
              reopened.mouse("mouseup", 140);
              reopened.touch("touchend", 140);
              assert.equal(
                reopened.committed.length,
                count + 1,
                "duplicate end does not commit twice",
              );
              assert.equal(
                reopened.handle.classList.contains("resize-active"),
                false,
              );
              assertRestored();
              await frame();
            }
          } finally {
            await reopened.unmount();
          }
          assertRestored();
        }
        chart.remove();
        disabledChart.remove();
      }
    }
  } finally {
    dom.window.close();
    previousGlobals.forEach((descriptor, key) => {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    });
  }
}

test(resize_unmount_state_restores_page_interaction);
