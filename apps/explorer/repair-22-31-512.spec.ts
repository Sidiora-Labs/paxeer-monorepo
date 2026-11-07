import { readdirSync, readFileSync } from 'node:fs';
import { join, relative } from 'node:path';

import ts from 'typescript';

import { expect, test } from 'vitest';

const explorerRoot = __dirname;
const generatedDirectories = new Set([
  'node_modules', 'node_modules_linux', '.next', '.git', '.cache', 'coverage', 'dist', 'out', 'build',
]);
const serverOnlyModules = new Set([ 'nextjs/utils/fetchProxy.ts', 'instrumentation.node.ts' ]);
// These tests run in Vitest's Node process (including its jsdom environment), outside the Next.js route tree.
const nodeTestModules = new Set([
  'nextjs/pages-tests/apps/repair-22-33-451.spec.ts',
  'nextjs/pages-tests/api/repair-22-33-448.spec.ts',
  'nextjs/pages-tests/api/tokens/[hash]/instances/[id]/repair-22-33-449.spec.ts',
  'nextjs/utils/fetchProxy.spec.ts',
  'nextjs/csp/policies/repair-22-31-412.spec.ts',
  'nextjs/csp/policies/repair-22-33-542.spec.ts',
  'nextjs/middlewares/repair-22-33-540.spec.ts',
  'lib/socket/repair-22-33-292.spec.ts',
]);

function explorerSources(directory: string): Array<string> {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      return generatedDirectories.has(entry.name) ? [] : explorerSources(path);
    }
    return entry.isFile() && /\.[cm]?[jt]sx?$/.test(entry.name) ? [ path ] : [];
  });
}

const sources = explorerSources(explorerRoot);

test('nodeFetchImportsStayServerOnly', function nodeFetchImportsStayServerOnly() {
  const importers: Array<string> = [];
  const leaks: Array<string> = [];
  for (const path of sources) {
    const modulePath = relative(explorerRoot, path).split('\\').join('/');
    const source = ts.createSourceFile(path, readFileSync(path, 'utf8'), ts.ScriptTarget.Latest, true);
    for (const statement of source.statements) {
      if (!(ts.isImportDeclaration(statement) || ts.isExportDeclaration(statement)) ||
        !statement.moduleSpecifier || !ts.isStringLiteral(statement.moduleSpecifier) ||
        !/^node-fetch(?:\/|$)/.test(statement.moduleSpecifier.text)) {
        continue;
      }
      importers.push(modulePath);
      if (!modulePath.startsWith('pages/api/') && !serverOnlyModules.has(modulePath) && !nodeTestModules.has(modulePath)) {
        leaks.push(`${ modulePath }:${ source.getLineAndCharacterOfPosition(statement.getStart()).line + 1 }`);
      }
    }
  }
  expect(importers).toContain('nextjs/utils/fetchProxy.ts');
  expect(leaks).toEqual([]);
});

test('testModulesStayOutOfNextPages', function testModulesStayOutOfNextPages() {
  const routes = sources.map(path => relative(explorerRoot, path).split('\\').join('/')).filter(path => path.startsWith('pages/'));
  expect(routes).toContain('pages/apps/[id].tsx');
  expect(routes).toContain('pages/api/proxy.ts');
  expect(routes.filter(path => /\.(?:spec|test|pw)\.[cm]?[jt]sx?$/.test(path))).toEqual([]);
  for (const path of nodeTestModules) {
    expect(path.startsWith('pages/')).toBe(false);
    expect(sources).toContain(join(explorerRoot, path));
  }
});
