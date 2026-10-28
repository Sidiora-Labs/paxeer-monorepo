// @vitest-environment jsdom

import * as React from "react";
import { renderToString } from "react-dom/server";
import { afterEach, expect, test } from "vitest";

import { ThemeProvider, useTheme } from "./theme-provider";

function ThemeValue() {
  const { theme, resolvedTheme, accent } = useTheme();
  return <output>{`${theme}:${resolvedTheme}:${accent}`}</output>;
}

afterEach(() => {
  window.localStorage.clear();
  document.documentElement.className = "";
});

test.each([
  ["light", "ink"],
  ["dark", "dx-gradient"],
  ["system", "dx-gradient"],
  ["invalid-theme", "invalid-accent"],
])(
  "initial browser-environment markup matches SSR with stored %s / %s",
  (theme, accent) => {
    window.localStorage.setItem("dx-theme", theme);
    window.localStorage.setItem("dx-accent", accent);

    expect(
      renderToString(
        <ThemeProvider>
          <ThemeValue />
        </ThemeProvider>,
      ),
    ).toBe("<output>system:light:ink</output>");
    expect(window.localStorage.getItem("dx-theme")).toBe(theme);
    expect(window.localStorage.getItem("dx-accent")).toBe(accent);
  },
);

test("initial rendering retains the pre-paint theme classes", () => {
  window.localStorage.setItem("dx-theme", "dark");
  window.localStorage.setItem("dx-accent", "dx-gradient");
  document.documentElement.className = "dark accent-dx-gradient";

  expect(
    renderToString(
      <ThemeProvider>
        <ThemeValue />
      </ThemeProvider>,
    ),
  ).toBe("<output>system:light:ink</output>");
  expect(document.documentElement.className).toBe("dark accent-dx-gradient");
});
