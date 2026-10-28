import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import ts from "typescript";
import {
  chartStateForPair,
  chartThemeTransition,
  resolveChartTheme,
} from "../src/widgets/trading/chartTheme";

const themes = ["dark", "light", "greenify", "hotline", "oled"] as const;
const source = ts.createSourceFile(
  "Chart.tsx",
  readFileSync(
    new URL("../src/widgets/trading/Chart.tsx", import.meta.url),
    "utf8",
  ),
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
);

function nodes<T extends ts.Node>(
  root: ts.Node,
  predicate: (node: ts.Node) => node is T,
): T[] {
  const result: T[] = [];
  const visit = (node: ts.Node) => {
    if (predicate(node)) result.push(node);
    ts.forEachChild(node, visit);
  };
  visit(root);
  return result;
}

function call(root: ts.Node, name: string) {
  const result = nodes(root, ts.isCallExpression).find(
    (node) => node.expression.getText(source).replace(/\s/g, "") === name,
  );
  assert.ok(result, `call to ${name}`);
  return result;
}

function effectWithCall(name: string) {
  const effect = nodes(source, ts.isCallExpression).find(
    (node) =>
      node.expression.getText(source) === "useEffect" &&
      nodes(node.arguments[0], ts.isCallExpression).some(
        (child) => child.expression.getText(source).replace(/\s/g, "") === name,
      ),
  );
  assert.ok(effect, `effect containing ${name}`);
  const dependencies = effect.arguments[1];
  assert.ok(dependencies && ts.isArrayLiteralExpression(dependencies));
  return {
    body: effect.arguments[0],
    dependencies: dependencies.elements.map((node) => node.getText(source)),
  };
}

test("chart theme plans select existing custom stylesheets and the typed base theme", () => {
  const backgrounds = ["#18191d", "#ffffff", "#0a0e18", "#0f0c2c", "#000000"];
  themes.forEach((name, index) => {
    const plan = resolveChartTheme(name);
    assert.deepEqual(plan, {
      name,
      baseTheme: name === "light" ? "light" : "dark",
      background: backgrounds[index],
      customCssUrl: `../${name}.css`,
    });
    assert.match(
      readFileSync(
        new URL(`../public/static/${name}.css`, import.meta.url),
        "utf8",
      ),
      /--tv-color-pane-background:/,
    );
  });
  assert.deepEqual(resolveChartTheme(undefined), resolveChartTheme("dark"));
  assert.deepEqual(
    resolveChartTheme("unrecognized"),
    resolveChartTheme("dark"),
  );
});

test("only changed custom styles rebuild; repeated themes and both built-in directions do not", () => {
  for (const previous of themes) {
    for (const next of themes) {
      const builtIn = ["dark", "light"];
      const expected =
        previous === next
          ? "none"
          : builtIn.includes(previous) && builtIn.includes(next)
            ? "runtime"
            : "rebuild";
      assert.equal(
        chartThemeTransition(previous, next),
        expected,
        `${previous} -> ${next}`,
      );
    }
  }
  assert.equal(chartThemeTransition(undefined, "dark"), "none");
  assert.equal(chartThemeTransition(undefined, "greenify"), "rebuild");
});

test("saved chart state remains opaque and is restored only for its captured market", () => {
  const state = Object.freeze({});
  const snapshot = Object.freeze({ selectedPair: "ETH/USD", state });
  assert.equal(chartStateForPair(snapshot, "ETH/USD"), state);
  assert.equal(chartStateForPair(snapshot, "BTC/USD"), undefined);
  assert.equal(chartStateForPair(null, "ETH/USD"), undefined);
});

test("source contract: a saved custom-theme transition reruns the constructor lifecycle and its cleanup", () => {
  const initialization = effectWithCall("loadTradingViewWidget");
  assert.deepEqual(initialization.dependencies, [
    "selectedPair",
    "chartRevision",
  ]);
  for (const reference of [
    "entryLineRef",
    "tpLineRef",
    "slLineRef",
    "widgetRef",
  ]) {
    call(
      initialization.body,
      `${reference}.current${reference === "widgetRef" ? "." : "?."}remove`,
    );
    assert.match(
      initialization.body.getText(source),
      new RegExp(`${reference}\\.current = null`),
    );
  }
  call(initialization.body, "setIsChartReady");

  const theme = effectWithCall("widget.save");
  assert.deepEqual(theme.dependencies, [
    "theme",
    "selectedPair",
    "chartRevision",
  ]);
  const save = call(theme.body, "widget.save");
  assert.equal(save.arguments[0].getText(source), "replace");
  assert.match(save.arguments[1].getText(source), /includeDrawings: true/);
  const replace = nodes(theme.body, ts.isVariableDeclaration).find(
    (node) => node.name.getText(source) === "replace",
  );
  assert.ok(replace?.initializer);
  assert.match(
    replace.initializer.getText(source),
    /cancelled \|\| widgetRef\.current !== widget/,
  );
  assert.match(
    replace.initializer.getText(source),
    /savedChartRef\.current\s*=\s*\{\s*selectedPair,\s*state,?\s*\}/,
  );
  call(replace.initializer, "setChartRevision");
  call(theme.body, "clearTimeout");
  assert.match(theme.body.getText(source), /cancelled = true/);
  assert.equal(
    nodes(theme.body, ts.isCallExpression).some((node) =>
      node.expression.getText(source).endsWith(".remove"),
    ),
    false,
  );

  const state = call(source, "chartStateForPair");
  assert.equal(
    state.arguments.map((node) => node.getText(source)).join(", "),
    "savedChartRef.current, selectedPair",
  );
  assert.ok(ts.isPropertyAssignment(state.parent));
  assert.equal(state.parent.name.getText(source), "saved_data");
  const constructor = nodes(source, ts.isNewExpression).find(
    (node) => node.expression.getText(source) === "window.TradingView.widget",
  );
  assert.ok(constructor?.arguments?.[0]);
  assert.match(constructor.arguments[0].getText(source), /\.\.\.themeOptions/);
  assert.match(
    constructor.arguments[0].getText(source),
    /load_last_chart: !themeOptions\.saved_data/,
  );
});

test("source contract: stale readiness is ignored and replacement order lines are regenerated", () => {
  const ready = call(source, "widget.onChartReady").arguments[0];
  assert.ok(ts.isArrowFunction(ready) && ts.isBlock(ready.body));
  assert.equal(
    ready.body.statements[0].getText(source),
    "if (widgetRef.current !== widget) return;",
  );
  const lines = effectWithCall("chart.createOrderLine");
  assert.ok(lines.dependencies.includes("chartRevision"));
  assert.ok(lines.dependencies.includes("isChartReady"));
  assert.match(lines.body.getText(source), /!isChartReadyRef\.current/);
  const theme = effectWithCall("widget.changeTheme");
  assert.match(
    theme.body.getText(source),
    /themeChangeRef\.current\s*=\s*themeChangeRef\.current\s*\.then/,
  );
  assert.equal(
    call(theme.body, "widget.changeTheme").arguments[0].getText(source),
    "nextTheme.baseTheme",
  );
});
