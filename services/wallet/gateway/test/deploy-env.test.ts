import { readFile } from 'node:fs/promises';
import ts from 'typescript';
import { expect, test } from 'vitest';

async function railwayExampleNamesOnlyEnvSchemaKeys() {
  const source = ts.createSourceFile(
    'env.ts',
    await readFile(new URL('../src/env.ts', import.meta.url), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS
  );
  const declaration = source.statements
    .filter(ts.isVariableStatement)
    .flatMap((statement) => statement.declarationList.declarations)
    .find((item) => ts.isIdentifier(item.name) && item.name.text === 'Env');
  expect(declaration?.initializer).toBeDefined();

  let shape: ts.ObjectLiteralExpression | undefined;
  function visit(node: ts.Node) {
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      ts.isIdentifier(node.expression.expression) &&
      node.expression.expression.text === 'z' &&
      node.expression.name.text === 'object' &&
      node.arguments[0] &&
      ts.isObjectLiteralExpression(node.arguments[0])
    ) {
      shape = node.arguments[0];
    }
    ts.forEachChild(node, visit);
  }
  visit(declaration!.initializer!);
  expect(shape).toBeDefined();
  const schemaKeys = new Set(
    shape!.properties.map((property) => {
      expect(ts.isPropertyAssignment(property)).toBe(true);
      const name = property.name!;
      expect(ts.isIdentifier(name) || ts.isStringLiteral(name)).toBe(true);
      return (name as ts.Identifier | ts.StringLiteral).text;
    })
  );
  expect(schemaKeys.size).toBeGreaterThan(0);

  const example = await readFile(
    new URL('../../deploy/railway.env.example', import.meta.url),
    'utf8'
  );
  const exampleKeys = [...example.matchAll(/^[\t ]*([A-Za-z_][A-Za-z0-9_]*)=/gm)].map(
    (match) => match[1]
  );
  expect(exampleKeys.length).toBeGreaterThan(0);
  expect(exampleKeys.filter((key) => !schemaKeys.has(key))).toEqual([]);
}

async function envCatalogOmitsSponsorKey() {
  const catalog = await readFile(new URL('../../deploy/env', import.meta.url), 'utf8');
  expect(catalog).not.toMatch(/\bSPONSOR_PRIVATE_KEY(?:_FILE)?\b/);
}

test(railwayExampleNamesOnlyEnvSchemaKeys);
test(envCatalogOmitsSponsorKey);
