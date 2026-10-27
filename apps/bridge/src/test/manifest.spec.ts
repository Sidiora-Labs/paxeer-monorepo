import { existsSync, readdirSync, readFileSync } from 'node:fs';
import { isBuiltin } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from 'vitest';

import manifest from '../../package.json';

const appRoot = fileURLToPath(new URL('../../', import.meta.url));

function sourceFiles(directory: string): Array<string> {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      return sourceFiles(filename);
    }
    return /\.(?:[cm]?[jt]sx?|css)$/.test(entry.name) ? [ filename ] : [];
  });
}

function importedPackages(): Set<string> {
  const packages = new Set<string>();
  const filenames = [
    ...sourceFiles(path.join(appRoot, 'src')),
    path.join(appRoot, 'next.config.ts'),
    path.join(appRoot, 'vitest.config.ts'),
  ];

  function addSpecifier(specifier: string): void {
    if (specifier.startsWith('.') || specifier.startsWith('/') || specifier.startsWith('@/') ||
      specifier.startsWith('#') || isBuiltin(specifier) || /^[a-z][a-z\d+.-]*:/i.test(specifier)) {
      return;
    }
    const segments = specifier.split('/');
    packages.add(specifier.startsWith('@') ? segments.slice(0, 2).join('/') : segments[0]);
  }

  for (const filename of filenames) {
    const source = readFileSync(filename, 'utf8');
    const specifiers = /\b(?:import|export)\s+(?:[^'";]*?\bfrom\s*)?['"]([^'"\r\n]+)['"]|\b(?:import|require)\s*\(\s*['"]([^'"\r\n]+)['"]|@(?:import|plugin)\s+(?:url\(\s*)?['"]([^'"\r\n]+)['"]|\/\*[\s\S]*?\*\/|\/\/[^\r\n]*|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|`(?:\\.|[^`\\])*`/g;
    for (const match of source.matchAll(specifiers)) {
      const specifier = match[1] ?? match[2] ?? match[3];
      if (specifier !== undefined) {
        addSpecifier(specifier);
      }
    }
    if (filename === path.join(appRoot, 'next.config.ts')) {
      for (const loaders of source.matchAll(/\bloaders\s*:\s*\[([^\]]*)\]/g)) {
        for (const loader of loaders[1].matchAll(/['"]([^'"]+)['"]/g)) {
          addSpecifier(loader[1]);
        }
      }
    }
  }
  return packages;
}

function everyImportedPackageIsDeclared(): void {
  const declared = new Map(Object.entries({ ...manifest.dependencies, ...manifest.devDependencies }));
  const imported = importedPackages();
  expect(imported.size).toBeGreaterThan(0);
  expect([ ...imported ].filter((name) => !declared.has(name)).sort()).toEqual([]);
  for (const name of [ 'motion', 'sonner', 'tailwindcss', '@tailwindcss/turbopack', '@noble/curves' ]) {
    expect(declared.has(name), name).toBe(true);
    expect(declared.get(name), name).toMatch(/^\d+\.\d+\.\d+$/);
  }
}

function noDeclaredRuntimeDependencyIsUnused(): void {
  const imported = importedPackages();
  expect(Object.keys(manifest.dependencies).filter((name) => !imported.has(name)).sort()).toEqual([]);
  for (const name of [ 'next', 'react', 'react-dom', 'viem', 'wagmi', '@tanstack/react-query' ]) {
    expect(Object.hasOwn(manifest.dependencies, name), name).toBe(true);
  }
}

function declaresTestAndTypecheckScripts(): void {
  expect(manifest.scripts.test).toBe('vitest run');
  expect(manifest.scripts.typecheck).toBe('tsc --noEmit -p tsconfig.json');
}

function declaresNoTelemetryPackages(): void {
  for (const [ section, declarations ] of Object.entries(manifest)) {
    if ([ 'dependencies', 'devDependencies', 'optionalDependencies', 'peerDependencies' ].includes(section)) {
      expect(Object.keys(declarations).filter((name) => name.startsWith('@opentelemetry/'))).toEqual([]);
    }
  }
  expect(existsSync(path.join(appRoot, 'instrumentation.node.ts'))).toBe(false);
  expect(existsSync(path.join(appRoot, 'tsconfig.jest.json'))).toBe(false);
}

test(everyImportedPackageIsDeclared);
test(noDeclaredRuntimeDependencyIsUnused);
test(declaresTestAndTypecheckScripts);
test(declaresNoTelemetryPackages);
