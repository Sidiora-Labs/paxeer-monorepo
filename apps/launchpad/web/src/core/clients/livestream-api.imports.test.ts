import { readFileSync, statSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';
import { expect, test } from 'vitest';

function livestreamClientImportsNoPackage() {
  const sourceRoot = fileURLToPath(new URL('../../', import.meta.url));
  const visited = new Set<string>();

  function visit(file: string) {
    if (visited.has(file)) return;
    visited.add(file);
    const source = ts.createSourceFile(file, readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true);

    function follow(specifier: string) {
      expect(specifier.startsWith('@/') || specifier.startsWith('./') || specifier.startsWith('../'), `${file} imports ${specifier}`).toBe(true);
      const target = specifier.startsWith('@/')
        ? resolve(sourceRoot, specifier.slice(2))
        : resolve(dirname(file), specifier);
      const dependency = [target, `${target}.ts`, `${target}.tsx`, `${target}.js`, resolve(target, 'index.ts'), resolve(target, 'index.tsx'), resolve(target, 'index.js')]
        .find(candidate => statSync(candidate, { throwIfNoEntry: false })?.isFile());
      expect(dependency, `${file} resolves ${specifier}`).toBeDefined();
      visit(dependency!);
    }

    function inspect(node: ts.Node) {
      if (ts.isImportDeclaration(node)) {
        if (node.importClause?.isTypeOnly) return;
        expect(ts.isStringLiteral(node.moduleSpecifier)).toBe(true);
        follow((node.moduleSpecifier as ts.StringLiteral).text);
      } else if (ts.isExportDeclaration(node) && node.moduleSpecifier && !node.isTypeOnly) {
        expect(ts.isStringLiteral(node.moduleSpecifier)).toBe(true);
        follow((node.moduleSpecifier as ts.StringLiteral).text);
      } else if (ts.isImportEqualsDeclaration(node) && !node.isTypeOnly && ts.isExternalModuleReference(node.moduleReference)) {
        const specifier = node.moduleReference.expression;
        expect(specifier && ts.isStringLiteral(specifier)).toBe(true);
        follow((specifier as ts.StringLiteral).text);
      } else if (ts.isCallExpression(node) && (node.expression.kind === ts.SyntaxKind.ImportKeyword || (ts.isIdentifier(node.expression) && node.expression.text === 'require'))) {
        const specifier = node.arguments[0];
        expect(specifier && ts.isStringLiteralLike(specifier), `${file} uses a literal module specifier`).toBe(true);
        follow((specifier as ts.StringLiteralLike).text);
      }
      ts.forEachChild(node, inspect);
    }

    inspect(source);
  }

  visit(resolve(sourceRoot, 'core/clients/livestream-api.ts'));
}

test(livestreamClientImportsNoPackage.name, livestreamClientImportsNoPackage);
