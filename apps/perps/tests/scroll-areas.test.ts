import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import postcss, { type Root, type Rule } from "postcss";

async function stylesheet() {
  const file = new URL("../apps/web/src/styles/globals.css", import.meta.url);
  return postcss.parse(await readFile(file, "utf8"), { from: file.pathname });
}

function rulesFor(root: Root, selector: string) {
  const rules: Rule[] = [];
  root.walkRules((rule) => {
    if (rule.selectors.includes(selector)) rules.push(rule);
  });
  assert.ok(rules.length > 0, `Rules exist for ${selector}`);
  return rules;
}

function declarations(rule: Rule) {
  const values: Record<string, string> = {};
  rule.walkDecls((declaration) => {
    values[declaration.prop] = declaration.value;
  });
  return values;
}

async function everyScrollAreaDrawsTheThinBar() {
  const root = await stylesheet();
  const expected = {
    "::-webkit-scrollbar": { width: "6px", height: "6px" },
    "::-webkit-scrollbar-track": {
      "background-color": "transparent",
      "border-radius": "4px",
      "padding-top": "20px",
    },
    "::-webkit-scrollbar-thumb": {
      "border-radius": "3px",
      "border-style": "dashed",
      "border-color": "transparent",
      "border-width": "1px",
      "background-color": "rgb(var(--oui-color-base-7))",
      "background-clip": "padding-box",
    },
    "::-webkit-scrollbar-thumb:hover": {
      background: "rgb(var(--oui-color-base-5))",
      "border-radius": "3px",
    },
    "::-webkit-scrollbar-corner": { "background-color": "transparent" },
  };
  for (const selector of ["*", ":where(html)"]) {
    for (const [pseudo, values] of Object.entries(expected)) {
      const rules = rulesFor(root, selector + pseudo);
      assert.equal(rules.length, 1);
      assert.equal(rules[0].parent, root);
      assert.deepEqual(declarations(rules[0]), values);
    }
  }
}

async function theStandardPropertiesStandBehindTheCondition() {
  const root = await stylesheet();
  const conditions = root.nodes.filter(
    (node) =>
      node.type === "atrule" &&
      node.name === "supports" &&
      node.params === "not selector(::-webkit-scrollbar)",
  );
  assert.equal(conditions.length, 1);
  const condition = conditions[0];
  for (const selector of ["*", ":where(html)"]) {
    const rules = rulesFor(root, selector).filter((rule) =>
      rule.nodes.some(
        (node) => node.type === "decl" && node.prop.startsWith("scrollbar-"),
      ),
    );
    assert.equal(rules.length, 1);
    assert.equal(rules[0].parent, condition);
    assert.deepEqual(declarations(rules[0]), {
      "scrollbar-width": "thin",
      "scrollbar-color": "rgb(var(--oui-color-base-7)) transparent",
    });
  }
  root.walkDecls((declaration) => {
    if (
      declaration.prop === "scrollbar-color" ||
      (declaration.prop === "scrollbar-width" && declaration.value !== "none")
    ) {
      assert.equal(declaration.parent?.parent, condition);
    }
  });
  for (const selector of [".hide-scrollbar", ".oui-hide-scrollbar"]) {
    const hidden = rulesFor(root, selector);
    assert.equal(hidden.length, 1);
    assert.equal(hidden[0].parent, condition);
    assert.deepEqual(declarations(hidden[0]), { "scrollbar-width": "none" });
  }
}

async function rowsAndControlsKeepTheirScrollAndDrawNoBar() {
  const root = await stylesheet();
  const parts = [
    [".oui-tabs-list"],
    [".oui-scaffold-topNavbar > div > div > div:last-child"],
    [".oui-portfolio-trader-card", ".oui-portfolio-trader-card > div"],
    [".oui-datepicker-trigger > span"],
    [".oui-orderBook button > span"],
  ];
  for (const part of parts) {
    for (const selector of part) {
      const rules = rulesFor(root, selector);
      assert.ok(
        rules.some((rule) => declarations(rule).overflow === "auto"),
        `${selector} keeps scrolling`,
      );
      assert.ok(
        rules.some((rule) => declarations(rule)["scrollbar-width"] === "none"),
        `${selector} hides the standard bar`,
      );
      for (const rule of rules) {
        rule.walkDecls(/^overflow(?:-[xy])?$/, (declaration) => {
          assert.equal(declaration.value, "auto");
        });
      }
      const bars = rulesFor(root, `${selector}::-webkit-scrollbar`);
      assert.equal(bars.length, 1);
      assert.equal(bars[0].parent, root);
      assert.equal(declarations(bars[0]).display, "none");
    }
  }
  const dateRules = rulesFor(root, ".oui-datepicker-trigger > span");
  assert.ok(
    dateRules.some((rule) =>
      rule.nodes.some(
        (node) =>
          node.type === "decl" &&
          node.prop === "overflow" &&
          node.value === "auto" &&
          node.important,
      ),
    ),
  );
  for (const selector of [
    ".oui-scaffold-container",
    ".oui-trading-dataList-container",
    ".oui-layout-grid-item",
    ".oui-sheet-content",
  ]) {
    const rules = rulesFor(root, selector);
    assert.equal(rules.length, 1);
    assert.equal(
      declarations(rules[0])[
        selector === ".oui-sheet-content" ? "overflow-y" : "overflow"
      ],
      "auto",
    );
    assert.equal(declarations(rules[0])["scrollbar-width"], undefined);
  }
}

test(everyScrollAreaDrawsTheThinBar);
test(theStandardPropertiesStandBehindTheCondition);
test(rowsAndControlsKeepTheirScrollAndDrawNoBar);
