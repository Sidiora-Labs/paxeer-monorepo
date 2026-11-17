import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import ts from "typescript";
import { WORKSPACE_THEME_CSS_VARS } from "../apps/web/src/theme/workspaceTheme.ts";

function readSource(path: string) {
  return ts.createSourceFile(
    path,
    readFileSync(new URL(`../apps/web/src/${path}`, import.meta.url), "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
}

function descendants(node: ts.Node): ts.Node[] {
  const nodes: ts.Node[] = [];
  ts.forEachChild(node, (child) => {
    nodes.push(child, ...descendants(child));
  });
  return nodes;
}

function applicationShellLeavesThemeAndNavigationToTheWorkspace() {
  const source = readSource("pages/_app.tsx");
  const nodes = descendants(source);
  const elements = nodes.filter(
    (node) => ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node),
  );
  assert.ok(elements.length > 0);
  for (const element of elements) {
    assert.notEqual(element.tagName.getText(source), "nav");
    for (const attribute of element.attributes.properties) {
      assert.ok(ts.isJsxAttribute(attribute), "shell attributes are explicit");
      assert.notEqual(attribute.name.getText(source), "style");
    }
  }
  const imports = nodes.filter(ts.isImportDeclaration);
  for (const declaration of imports) {
    assert.doesNotMatch(declaration.moduleSpecifier.getText(source), /theme/i);
    assert.doesNotMatch(
      declaration.importClause?.getText(source) ?? "",
      /CSS_VARS/,
    );
  }
}

test(applicationShellLeavesThemeAndNavigationToTheWorkspace);

function scaffoldFooterContainsTheLegalLinksBesideTheWorkspaceName() {
  const source = readSource("native/PaxeerNetwork.tsx");
  const scaffold = descendants(source).find(
    (node): node is ts.VariableDeclaration =>
      ts.isVariableDeclaration(node) &&
      node.name.getText(source) === "scaffold",
  );
  assert.ok(
    scaffold?.initializer && ts.isObjectLiteralExpression(scaffold.initializer),
  );
  const footer = scaffold.initializer.properties.find(
    (node): node is ts.PropertyAssignment =>
      ts.isPropertyAssignment(node) &&
      node.name.getText(source) === "footerProps",
  );
  assert.ok(footer && ts.isObjectLiteralExpression(footer.initializer));
  const trailing = footer.initializer.properties.find(
    (node): node is ts.PropertyAssignment =>
      ts.isPropertyAssignment(node) && node.name.getText(source) === "trailing",
  );
  assert.ok(trailing);
  assert.match(trailing.initializer.getText(source), /Paxeer Xtrades/);
  const links = descendants(trailing.initializer).filter(
    (node): node is ts.JsxElement =>
      ts.isJsxElement(node) &&
      node.openingElement.tagName.getText(source) === "Link",
  );
  assert.equal(links.length, 3);
  for (const [index, [href, label]] of [
    ["/privacy", "Privacy Policy"],
    ["/terms", "Terms of Service"],
    ["/cookies", "Cookie Settings"],
  ].entries()) {
    const link = links[index];
    const address = link.openingElement.attributes.properties.find(
      (node): node is ts.JsxAttribute =>
        ts.isJsxAttribute(node) && node.name.getText(source) === "href",
    );
    assert.ok(address?.initializer && ts.isStringLiteral(address.initializer));
    assert.equal(address.initializer.text, href);
    assert.equal(
      link.children
        .map((child) => child.getText(source))
        .join("")
        .trim(),
      label,
    );
  }
  const consumers = descendants(source).filter(
    (node): node is ts.JsxOpeningElement =>
      ts.isJsxOpeningElement(node) &&
      ["Scaffold", "PortfolioLayoutWidget"].includes(
        node.tagName.getText(source),
      ),
  );
  assert.equal(consumers.length, 3);
  for (const consumer of consumers) {
    assert.ok(
      consumer.attributes.properties.some(
        (node) =>
          ts.isJsxSpreadAttribute(node) &&
          node.expression.getText(source) === "scaffold",
      ),
    );
  }
}

test(scaffoldFooterContainsTheLegalLinksBesideTheWorkspaceName);

function legalPagesAndConsentShareTheDeclaredWorkspaceTheme() {
  const legal = readSource("legal/LegalLayout.tsx");
  const native = readSource("native/PaxeerNetwork.tsx");
  for (const source of [legal, native]) {
    const declaration = descendants(source)
      .filter(ts.isImportDeclaration)
      .find(
        (node) =>
          ts.isStringLiteral(node.moduleSpecifier) &&
          node.moduleSpecifier.text === "../theme/workspaceTheme",
      );
    assert.ok(declaration);
    assert.equal(
      declaration.importClause?.namedBindings?.getText(source),
      "{ WORKSPACE_THEME_CSS_VARS }",
    );
  }
  assert.match(native.text, /cssVars:\s*WORKSPACE_THEME_CSS_VARS/);
  const themedLayout = descendants(legal).find(
    (node): node is ts.JsxElement =>
      ts.isJsxElement(node) &&
      node.openingElement.attributes.properties.some(
        (attribute) =>
          ts.isJsxAttribute(attribute) &&
          attribute.name.getText(legal) === "style" &&
          attribute.initializer?.getText(legal) ===
            "{WORKSPACE_THEME_CSS_VARS as CSSProperties}",
      ),
  );
  assert.ok(themedLayout);
  for (const source of [themedLayout, native]) {
    assert.equal(
      descendants(source).filter(
        (node) =>
          ts.isJsxSelfClosingElement(node) &&
          node.tagName.getText() === "CookieBanner",
      ).length,
      1,
    );
  }
  for (const key of [
    "--oui-color-primary",
    "--oui-color-primary-light",
    "--oui-color-primary-darken",
  ] as const) {
    assert.equal(WORKSPACE_THEME_CSS_VARS[key], "255 255 255");
  }
  assert.equal(
    WORKSPACE_THEME_CSS_VARS["--oui-color-primary-contrast"],
    "0 0 0",
  );
  assert.equal(
    WORKSPACE_THEME_CSS_VARS["--oui-color-base-foreground"],
    "255 255 255",
  );
}

test(legalPagesAndConsentShareTheDeclaredWorkspaceTheme);
