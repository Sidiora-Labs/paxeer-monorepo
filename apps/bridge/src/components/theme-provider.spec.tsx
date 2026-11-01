import * as React from "react";
import { renderToString } from "react-dom/server";
import { expect, test } from "vitest";

import { ThemeProvider, useTheme } from "./theme-provider";

function ThemeValue() {
  const { theme, resolvedTheme, accent } = useTheme();
  return <output>{`${theme}:${resolvedTheme}:${accent}`}</output>;
}

test("server rendering does not require browser globals or storage", () => {
  expect(typeof window).toBe("undefined");
  expect(typeof document).toBe("undefined");
  expect(
    renderToString(
      <ThemeProvider>
        <ThemeValue />
      </ThemeProvider>,
    ),
  ).toBe("<output>system:light:ink</output>");
});

test("server rendering preserves errors from the rendered application", () => {
  const failure = new Error("application render failure");
  function BrokenChild(): React.ReactNode {
    throw failure;
  }
  expect(() =>
    renderToString(
      <ThemeProvider>
        <BrokenChild />
      </ThemeProvider>,
    ),
  ).toThrow(failure);
});
