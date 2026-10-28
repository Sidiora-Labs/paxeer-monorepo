import ts from 'typescript';
import { expect, test } from 'vitest';

function unauthorized_reasons_exclude_custody_not_supported(): void {
  const errorsPath = decodeURIComponent(new URL('../src/errors.ts', import.meta.url).pathname);
  const program = ts.createProgram([errorsPath], {
    noEmit: true,
    strict: true,
    target: ts.ScriptTarget.ES2022,
    types: [],
  });
  const source = program.getSourceFile(errorsPath);
  if (!source) throw new Error('errors.ts was not loaded');
  const declaration = source.statements.find(
    (statement): statement is ts.TypeAliasDeclaration =>
      ts.isTypeAliasDeclaration(statement) && statement.name.text === 'UnauthorizedReason',
  );
  if (!declaration) throw new Error('UnauthorizedReason was not declared');
  const reasonType = program.getTypeChecker().getTypeFromTypeNode(declaration.type);
  if (!reasonType.isUnion()) throw new Error('UnauthorizedReason must be a union');
  const reasons = reasonType.types.map((member) => {
    if (!member.isStringLiteral())
      throw new Error('UnauthorizedReason must contain string literals');
    return member.value;
  });

  expect(reasons).not.toContain('custody_not_supported');
  expect(reasons).toEqual([
    'not_connected',
    'no_token',
    'session_changed',
    'account_mismatch',
    'missing_construction',
    'unknown_construction',
    'digest_mismatch',
  ]);
}

test(unauthorized_reasons_exclude_custody_not_supported);
