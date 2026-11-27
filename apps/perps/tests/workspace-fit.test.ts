import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { test } from "node:test";
import ts from "typescript";

async function sourceFile(path: string) {
  const file = new URL(`../packages/${path}`, import.meta.url);
  return ts.createSourceFile(
    file.pathname,
    await readFile(file, "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
}

function variable(root: ts.Node, name: string) {
  const matches: ts.VariableDeclaration[] = [];
  function visit(node: ts.Node) {
    if (
      ts.isVariableDeclaration(node) &&
      ts.isIdentifier(node.name) &&
      node.name.text === name
    ) {
      matches.push(node);
    }
    ts.forEachChild(node, visit);
  }
  visit(root);
  assert.equal(matches.length, 1, `One declaration exists for ${name}`);
  const declaration = matches[0];
  assert.ok(declaration.initializer, `${name} has an initializer`);
  return declaration.initializer;
}

function expression(node: ts.Expression): ts.Expression {
  while (ts.isParenthesizedExpression(node) || ts.isAsExpression(node)) {
    node = node.expression;
  }
  return node;
}

function identifier(node: ts.Node, name: string) {
  assert.ok(ts.isIdentifier(node));
  assert.equal(node.text, name);
}

function number(node: ts.Node) {
  assert.ok(ts.isNumericLiteral(node), "The value is a numeric literal");
  return Number(node.text);
}

async function theSideListOfMarketsFitsItsPanel() {
  const source = await sourceFile(
    "markets/src/components/sideMarkets/column.tsx",
  );
  const hook = variable(source, "useSideMarketsColumns");
  assert.ok(ts.isArrowFunction(hook));
  assert.ok(ts.isBlock(hook.body));
  const returns = hook.body.statements.filter(ts.isReturnStatement);
  assert.equal(returns.length, 1);
  assert.ok(returns[0].expression);
  const columns = expression(returns[0].expression);
  assert.ok(ts.isArrayLiteralExpression(columns));
  assert.equal(columns.elements.length, 3);
  const widths = columns.elements.map((column) => {
    assert.ok(ts.isObjectLiteralExpression(column));
    const properties = column.properties.filter(
      (property): property is ts.PropertyAssignment =>
        ts.isPropertyAssignment(property) &&
        ts.isIdentifier(property.name) &&
        property.name.text === "width",
    );
    assert.equal(
      properties.length,
      1,
      "Each column declares its width locally",
    );
    const width = properties[0];
    assert.equal(column.properties.at(-1), width, "The local width wins");
    const value = number(width.initializer);
    assert.ok(value > 0);
    return value;
  });
  assert.ok(
    widths.reduce((total, width) => total + width, 0) <= 272,
    "The three columns fit the 280px panel with 8px of scroll-area padding",
  );
}

async function theDesktopOrderBookCountsItsFixedRows() {
  const source = await sourceFile(
    "trading-next/src/components/base/orderBook/orderBook.script.tsx",
  );
  assert.equal(number(variable(source, "SPACE")), 104);
  assert.equal(number(variable(source, "DESKTOP_SPACE")), 110);
  assert.equal(
    number(variable(source, "BUY_SELL_RATIO_BAR_HEIGHT_MOBILE")),
    28,
  );
  assert.equal(
    number(variable(source, "BUY_SELL_RATIO_BAR_HEIGHT_DESKTOP")),
    38,
  );

  const hook = variable(source, "useOrderBookScript");
  assert.ok(ts.isArrowFunction(hook));
  const availableSpace = expression(variable(hook.body, "availableSpace"));
  assert.ok(ts.isBinaryExpression(availableSpace));
  assert.equal(availableSpace.operatorToken.kind, ts.SyntaxKind.PlusToken);
  const fixedRows = expression(availableSpace.left);
  assert.ok(ts.isConditionalExpression(fixedRows));
  identifier(fixedRows.condition, "isMobile");
  identifier(fixedRows.whenTrue, "SPACE");
  identifier(fixedRows.whenFalse, "DESKTOP_SPACE");

  const ratio = expression(availableSpace.right);
  assert.ok(ts.isConditionalExpression(ratio));
  identifier(ratio.condition, "showBuySellRatio");
  assert.equal(number(ratio.whenFalse), 0);
  const ratioHeight = expression(ratio.whenTrue);
  assert.ok(ts.isConditionalExpression(ratioHeight));
  identifier(ratioHeight.condition, "isMobile");
  identifier(ratioHeight.whenTrue, "BUY_SELL_RATIO_BAR_HEIGHT_MOBILE");
  identifier(ratioHeight.whenFalse, "BUY_SELL_RATIO_BAR_HEIGHT_DESKTOP");
}

test(theSideListOfMarketsFitsItsPanel);
test(theDesktopOrderBookCountsItsFixedRows);
