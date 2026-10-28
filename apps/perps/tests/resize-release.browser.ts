import { createElement, createRef } from "react";
import { createRoot } from "react-dom/client";
import { flushSync } from "react-dom";
import { ResizeHandle } from "../src/ui/atoms/resize-handle";

let checks = 0;
const nextFrame = () =>
  new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

function assert(condition: unknown, message: string): asserts condition {
  if (!condition) throw new Error(message);
  checks += 1;
}

function mount(
  direction: "horizontal" | "vertical",
  invert = false,
  cssProp?: "width" | "height" | "flexBasis",
) {
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  const targetRef = createRef<HTMLDivElement>();
  const committed: number[] = [];
  flushSync(() => {
    root.render(
      createElement(
        "div",
        { style: { display: "flex", flexDirection: "column" } },
        createElement("div", {
          ref: targetRef,
          style: {
            width: "200px",
            height: "200px",
            flexGrow: 0,
            flexShrink: 0,
            ...(cssProp === "flexBasis" ? { flexBasis: "200px" } : {}),
          },
        }),
        createElement(ResizeHandle, {
          direction,
          targetRef,
          cssProp,
          min: 120,
          max: 320,
          invert,
          className: "test-resize-handle",
          onResizeEnd: (size) => committed.push(size),
        }),
      ),
    );
  });
  const target = targetRef.current!;
  const handle = container.querySelector<HTMLDivElement>(
    ".test-resize-handle",
  )!;
  const size = () => {
    const rect = target.getBoundingClientRect();
    return direction === "horizontal" ? rect.width : rect.height;
  };
  const mouse = (type: "mousedown" | "mousemove" | "mouseup", position: number) => {
    (type === "mousedown" ? handle : window).dispatchEvent(
      new MouseEvent(type, {
        bubbles: true,
        cancelable: true,
        clientX: direction === "horizontal" ? position : 0,
        clientY: direction === "vertical" ? position : 0,
      }),
    );
  };
  const touch = (
    type: "touchstart" | "touchmove" | "touchend" | "touchcancel",
    position: number,
    changedOnly = false,
  ) => {
    const point = new Touch({
      identifier: 1,
      target: handle,
      clientX: direction === "horizontal" ? position : 0,
      clientY: direction === "vertical" ? position : 0,
    });
    const ending = type === "touchend" || type === "touchcancel";
    (type === "touchstart" ? handle : window).dispatchEvent(
      new TouchEvent(type, {
        bubbles: true,
        touches: ending || changedOnly ? [] : [point],
        changedTouches: [point],
      }),
    );
  };
  const clean = () => {
    assert(!handle.classList.contains("resize-active"), "active class removed");
    assert(document.body.style.cursor === "", "cursor restored");
    assert(document.body.style.userSelect === "", "selection restored");
    assert(document.body.style.webkitUserSelect === "", "WebKit selection restored");
    assert(
      document.querySelector("iframe")!.style.pointerEvents === "",
      "iframe pointer events restored",
    );
  };
  return {
    committed,
    target,
    handle,
    size,
    mouse,
    touch,
    clean,
    unmount() {
      root.unmount();
      container.remove();
    },
  };
}

async function run() {
  const iframe = document.createElement("iframe");
  document.body.append(iframe);
  try {
    for (const direction of ["horizontal", "vertical"] as const) {
      const fixture = mount(direction);
      try {
        fixture.mouse("mousedown", 100);
        fixture.mouse("mousemove", 120);
        fixture.mouse("mousemove", 175);
        assert(fixture.size() === 200, "mouse moves remain frame-throttled");
        assert(fixture.committed.join() === "", "drag does not commit early");
        assert(fixture.handle.classList.contains("resize-active"), "drag is active");
        assert(iframe.style.pointerEvents === "none", "iframe is blocked during drag");
        fixture.mouse("mouseup", 175);
        assert(fixture.size() === 275, "release synchronously applies latest pending move");
        assert(fixture.committed.join() === "275", "release commits latest dimensions once");
        fixture.clean();
        fixture.mouse("mouseup", 175);
        fixture.mouse("mousemove", 300);
        await nextFrame();
        assert(fixture.size() === 275, "cancelled frame and stray move cannot resize");
        assert(fixture.committed.join() === "275", "duplicate release does not commit");

        fixture.mouse("mousedown", 100);
        fixture.mouse("mousemove", 200);
        fixture.mouse("mouseup", 200);
        assert(fixture.size() === 320, "repeated drag clamps to maximum");
        assert(fixture.committed.join() === "275,320", "repeated drag starts at current size");
        fixture.clean();

        fixture.mouse("mousedown", 100);
        fixture.mouse("mousemove", -500);
        fixture.mouse("mouseup", -500);
        assert(fixture.size() === 120, "pending release clamps to minimum");
        assert(fixture.committed.join() === "275,320,120", "minimum is committed once");
        fixture.clean();

        fixture.mouse("mousedown", 50);
        fixture.mouse("mousemove", 100);
        assert(fixture.size() === 120, "new drag is still throttled");
        await nextFrame();
        assert(fixture.size() === 170, "animation frame applies ongoing drag");
        assert(fixture.committed.length === 3, "animation frame does not commit");
        fixture.mouse("mousemove", 120);
        fixture.mouse("mouseup", 120);
        assert(fixture.size() === 190, "release flushes movement after an applied frame");
        assert(fixture.committed.join() === "275,320,120,190", "final move after frame is committed");
        fixture.clean();

        fixture.mouse("mousedown", 50);
        fixture.mouse("mouseup", 50);
        assert(fixture.size() === 190, "release without movement preserves size");
        assert(fixture.committed.join() === "275,320,120,190,190", "stationary release commits once");
        fixture.clean();
      } finally {
        fixture.unmount();
      }
    }

    for (const cssProp of ["height", "flexBasis"] as const) {
      const fixture = mount("vertical", true, cssProp);
      try {
        fixture.touch("touchstart", 100);
        fixture.touch("touchmove", 60);
        fixture.touch("touchmove", 25, true);
        assert(fixture.size() === 200, "touch moves remain frame-throttled");
        fixture.touch("touchend", 25);
        assert(fixture.size() === 275, "touch release applies inverted pending movement");
        assert(fixture.target.style[cssProp] === "275px", "configured CSS property is updated");
        assert(fixture.committed.join() === "275", "touch release commits latest dimensions");
        fixture.clean();
        fixture.touch("touchend", 25);
        await nextFrame();
        assert(fixture.committed.join() === "275", "duplicate touch release does not commit");

        fixture.touch("touchstart", 100);
        fixture.touch("touchmove", 400);
        fixture.touch("touchcancel", 400);
        assert(fixture.size() === 120, "touch cancellation flushes the bounded position");
        assert(fixture.committed.join() === "275,120", "touch cancellation commits once");
        fixture.clean();
      } finally {
        fixture.unmount();
      }
    }
  } finally {
    iframe.remove();
  }
  return checks;
}

void run().then(
  (count) => {
    const result = document.createElement("pre");
    result.id = "resize-release-result";
    result.textContent = `PASS ${count}`;
    document.body.append(result);
  },
  (error: unknown) => {
    const result = document.createElement("pre");
    result.id = "resize-release-result";
    result.textContent = `FAIL ${String(error)}`;
    document.body.append(result);
  },
);
