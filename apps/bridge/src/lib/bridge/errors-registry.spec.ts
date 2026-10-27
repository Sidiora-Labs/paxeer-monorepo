import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import ts from 'typescript';

import { expect, test } from 'vitest';

function everyPrecompileRevertCodeHasASentence() {
  const precompile = readFileSync(resolve(__dirname, '../../../../../precompiles/layerxbridge/layerxbridge.go'), 'utf8');
  const registry = precompile.match(/^var revertCodes = \[\]struct \{[\s\S]*?^\}\{([\s\S]*?)^\}/m);

  expect(registry).not.toBeNull();
  const revertCodes = Array.from(registry![1].matchAll(/"(LXP_BRIDGE_[A-Z_]+)"/g), (match) => match[1]);
  expect(revertCodes.length).toBeGreaterThan(0);

  const errors = ts.createSourceFile(
    'errors.ts',
    readFileSync(resolve(__dirname, 'errors.ts'), 'utf8'),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  const declaration = errors.statements
    .filter(ts.isVariableStatement)
    .flatMap((statement) => statement.declarationList.declarations)
    .find((variable) => ts.isIdentifier(variable.name) && variable.name.text === 'PRECOMPILE_CODES');

  if (!declaration?.initializer || !ts.isObjectLiteralExpression(declaration.initializer)) {
    throw new Error('PRECOMPILE_CODES must declare the client refusal-code table.');
  }

  const PRECOMPILE_CODES = Object.fromEntries(declaration.initializer.properties.map((property) => {
    if (!ts.isPropertyAssignment(property) ||
        !(ts.isIdentifier(property.name) || ts.isStringLiteral(property.name)) ||
        !ts.isStringLiteral(property.initializer)) {
      throw new Error('Every PRECOMPILE_CODES entry must map a literal code to a sentence.');
    }
    return [ property.name.text, property.initializer.text ];
  }));

  expect(Object.keys(PRECOMPILE_CODES).map((code) => `LXP_BRIDGE_${ code }`).sort()).toEqual(revertCodes.sort());
  for (const sentence of Object.values(PRECOMPILE_CODES)) {
    expect(sentence).toMatch(/\S.*\.$/);
  }
}

test(everyPrecompileRevertCodeHasASentence.name, everyPrecompileRevertCodeHasASentence);
