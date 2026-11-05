import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import postcss from "postcss";

const stylesheet = postcss.parse(
  readFileSync(
    process.env.PERPS_FOCUS_CSS ??
      new URL("../src/styles/globals.css", import.meta.url),
    "utf8",
  ),
);
const defaultFocusSelector =
  ':where(button, [role="button"], a, input, select, textarea, [tabindex]):focus-visible';

test("plain navigation links retain the default keyboard focus outline", () => {
  const declarations = new Map<string, string>();
  stylesheet.walkRules(defaultFocusSelector, (rule) => {
    rule.walkDecls((declaration) => {
      declarations.set(declaration.prop, declaration.value);
    });
  });

  assert.equal(
    declarations.get("outline"),
    "2px solid hsl(var(--ring, 220 13% 70%))",
  );
  assert.equal(declarations.get("outline-offset"), "2px");

  const genericFocusSelectors = new Set([
    "*",
    "a",
    ":focus",
    "*:focus",
    "a:focus",
    ":focus-visible",
    "*:focus-visible",
    "a:focus-visible",
    defaultFocusSelector,
  ]);
  stylesheet.walkRules((rule) => {
    if (
      !rule.selectors.some((selector) => genericFocusSelectors.has(selector))
    ) {
      return;
    }
    rule.walkDecls(/^(outline|outline-style|outline-width)$/, (declaration) => {
      assert.doesNotMatch(
        declaration.value,
        /^(?:none|hidden|0(?:px)?)(?:\s|$)/,
        `${rule.selector} must not suppress the keyboard outline`,
      );
    });
  });
});

test("mouse-only focus suppression does not match keyboard focus", () => {
  const outlines: string[] = [];
  stylesheet.walkRules("*:focus:not(:focus-visible)", (rule) => {
    rule.walkDecls("outline", (declaration) => {
      outlines.push(declaration.value);
    });
  });
  assert.deepEqual(outlines, ["none"]);
});
