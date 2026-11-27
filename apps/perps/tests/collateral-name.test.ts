import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";
import { formatSymbol, tokenDisplayName } from "../packages/utils/src/symbol";

const root = fileURLToPath(new URL("../", import.meta.url));
const oldName = /\bUSDC\b/;
const displayAttributes = new Set([
  "unit",
  "suffix",
  "prefix",
  "label",
  "title",
  "placeholder",
]);
const sourceExceptions = [
  {
    file: "packages/ui-positions/src/components/liquidation/desktop/useLiquidationColumn.tsx",
    text: '"Price_(USDC)"',
    reason: "Table column lookup key.",
  },
];

function sourceFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const file = join(directory, entry.name);
    if (entry.isDirectory()) {
      return /^(?:__tests?__|tests?|node_modules)$/.test(entry.name)
        ? []
        : sourceFiles(file);
    }
    return /\.tsx?$/.test(entry.name) && !/\.(?:test|spec)\./.test(entry.name)
      ? [file]
      : [];
  });
}

function parse(file: string): ts.SourceFile {
  return ts.createSourceFile(
    file,
    readFileSync(file, "utf8"),
    ts.ScriptTarget.Latest,
    true,
  );
}

const sources = [
  ...readdirSync(join(root, "packages"), { withFileTypes: true })
    .filter(
      (entry) =>
        entry.isDirectory() &&
        existsSync(join(root, "packages", entry.name, "src")),
    )
    .flatMap((entry) => sourceFiles(join(root, "packages", entry.name, "src"))),
  ...sourceFiles(join(root, "apps/web/src")),
].map(parse);

function nodes(
  source: ts.SourceFile,
  predicate: (node: ts.Node) => boolean,
): ts.Node[] {
  const found: ts.Node[] = [];
  function visit(node: ts.Node) {
    if (predicate(node)) found.push(node);
    ts.forEachChild(node, visit);
  }
  visit(source);
  return found;
}

function location(source: ts.SourceFile, node: ts.Node): string {
  return `${relative(root, source.fileName)}:${source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1}: ${node.getText(source)}`;
}

function literalText(node: ts.Node): string | undefined {
  return ts.isStringLiteralLike(node) ||
    ts.isTemplateHead(node) ||
    ts.isTemplateMiddle(node) ||
    ts.isTemplateTail(node)
    ? node.text
    : undefined;
}

function noViewWritesTheOldNameAsAUnit() {
  const occurrences = sources.flatMap((source) =>
    nodes(source, (node) => {
      if (
        !ts.isJsxAttribute(node) ||
        !displayAttributes.has(node.name.getText(source)) ||
        !node.initializer
      )
        return false;
      const value = ts.isJsxExpression(node.initializer)
        ? node.initializer.expression
        : node.initializer;
      return value !== undefined && literalText(value)?.trim() === "USDC";
    }).map((node) => location(source, node)),
  );
  assert.deepEqual(occurrences, []);
}

test(noViewWritesTheOldNameAsAUnit);

function noViewTextHoldsTheOldName() {
  for (const exception of sourceExceptions) {
    assert.ok(
      readFileSync(join(root, exception.file), "utf8").includes(exception.text),
    );
    assert.ok(exception.reason.length > 0);
  }
  const occurrences = sources.flatMap((source) =>
    nodes(source, (node) => {
      if (ts.isJsxText(node)) return oldName.test(node.text);
      const value = literalText(node);
      if (value === undefined || !/\(\s*USDC\s*\)/.test(value)) return false;
      return !sourceExceptions.some(
        (exception) =>
          exception.file === relative(root, source.fileName) &&
          exception.text === node.getText(source),
      );
    }).map((node) => location(source, node)),
  );
  assert.deepEqual(occurrences, []);
}

test(noViewTextHoldsTheOldName);

function noLocaleValueHoldsTheOldName() {
  const modules = sourceFiles(
    join(root, "packages/i18n/src/locale/module"),
  ).map(parse);
  const occurrences = modules.flatMap((source) =>
    nodes(source, (node) => {
      const value = literalText(node);
      return (
        value !== undefined &&
        oldName.test(value) &&
        !(ts.isPropertyAssignment(node.parent) && node.parent.name === node)
      );
    }).map((node) => location(source, node)),
  );
  const trading = modules.find((source) =>
    source.fileName.endsWith("/trading.ts"),
  )!;
  assert.ok(
    nodes(
      trading,
      (node) =>
        ts.isPropertyAssignment(node) &&
        ts.isStringLiteral(node.name) &&
        node.name.text === "trading.faucet.getTestUSDC",
    ).length === 1,
  );
  for (const file of readdirSync(join(root, "packages/i18n/locales")).filter(
    (file) => file.endsWith(".json"),
  )) {
    const messages: Record<string, string> = JSON.parse(
      readFileSync(join(root, "packages/i18n/locales", file), "utf8"),
    );
    for (const [key, value] of Object.entries(messages)) {
      if (oldName.test(value)) occurrences.push(`${file}: ${key}: ${value}`);
    }
    if (file === "en.json") {
      assert.ok(Object.hasOwn(messages, "trading.faucet.getTestUSDC"));
      for (const source of modules) {
        for (const node of nodes(source, ts.isPropertyAssignment)) {
          if (
            ts.isPropertyAssignment(node) &&
            ts.isStringLiteralLike(node.name) &&
            ts.isStringLiteralLike(node.initializer)
          ) {
            assert.equal(
              messages[node.name.text],
              node.initializer.text,
              node.name.text,
            );
          }
        }
      }
    }
  }
  assert.deepEqual(occurrences, []);
}

test(noLocaleValueHoldsTheOldName);

function noViewPrintsTheQuoteNameBare() {
  function isQuote(node: ts.Node): boolean {
    return (
      (ts.isIdentifier(node) && node.text === "quote") ||
      (ts.isPropertyAccessExpression(node) && node.name.text === "quote")
    );
  }
  const occurrences = sources.flatMap((source) =>
    nodes(source, (node) => {
      if (ts.isTemplateSpan(node)) return isQuote(node.expression);
      if (
        !ts.isJsxExpression(node) ||
        !node.expression ||
        !isQuote(node.expression)
      )
        return false;
      return !(
        ts.isJsxAttribute(node.parent) &&
        node.parent.name.getText(source) === "quote"
      );
    }).map((node) => location(source, node)),
  );
  assert.deepEqual(occurrences, []);
}

test(noViewPrintsTheQuoteNameBare);

function theNameToShowForAKey() {
  assert.equal(tokenDisplayName("USDC"), "USDL");
  const keys = new Set([
    "",
    "USDL",
    "USDT",
    "ETH",
    "BTC",
    "usdc",
    "Usdc",
    "USDC.e",
    "USDCX",
    "PERP_ETH_USDC",
    " USDC",
    "USDC ",
  ]);
  for (const source of sources) {
    for (const node of nodes(source, ts.isStringLiteralLike)) {
      if (ts.isStringLiteralLike(node)) keys.add(node.text);
    }
  }
  for (const key of keys)
    assert.equal(tokenDisplayName(key), key === "USDC" ? "USDL" : key);
  assert.equal(tokenDisplayName(undefined), undefined);
  assert.equal(tokenDisplayName(null), null);
  assert.equal(formatSymbol("PERP_ETH_USDC", "base-quote"), "ETH-USDL");
  assert.equal(formatSymbol("PERP_ETH_USDT", "base-quote"), "ETH-USDT");
  for (const file of [
    "packages/hooks/src/native/NativeRuntime.tsx",
    "packages/hooks/src/next/useOrderEntry/useOrderEntry.ts",
  ]) {
    assert.match(readFileSync(join(root, file), "utf8"), /quote:\s*"USDC"/);
  }
  for (const packageName of ["trading", "trading-next"]) {
    assert.match(
      readFileSync(
        join(
          root,
          `packages/${packageName}/src/components/desktop/orderBook/index.desktop.tsx`,
        ),
        "utf8",
      ),
      /coinType !== quote/,
    );
  }
}

test(theNameToShowForAKey);
