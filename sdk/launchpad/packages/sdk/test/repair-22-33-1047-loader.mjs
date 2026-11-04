import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { registerHooks } from 'node:module';
import { fileURLToPath } from 'node:url';
import ts from 'typescript';

const sourceRoot = new URL('../src/', import.meta.url);
const distributionRoot = new URL('../dist/', import.meta.url);
const declarations = [
  'validators-v1.gen.d.ts',
  'validators-v2.gen.d.ts',
  'validators-operations.gen.d.ts',
];
const before = declarations.map(name => readFileSync(new URL(name, sourceRoot)));

// Use the production validator emitter in this process; no installed SDK build
// or subprocess compiler is a prerequisite of the regression gate.
await import('../scripts/compile-validators.mjs');
declarations.forEach((name, index) => {
  assert.deepEqual(readFileSync(new URL(name, sourceRoot)), before[index], name);
});

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith('.') || specifier.startsWith('file:')) {
      const requested = new URL(specifier, context.parentURL);
      for (const root of [sourceRoot, distributionRoot]) {
        if (requested.href.startsWith(root.href) && requested.pathname.endsWith('.js')) {
          const source = new URL(requested.href.slice(root.href.length).replace(/\.js$/, '.ts'), sourceRoot);
          if (existsSync(source)) return { url: source.href, shortCircuit: true };
        }
      }
      if (requested.href.startsWith(sourceRoot.href) && /\/validators-[^/]+\.gen\.js$/.test(requested.pathname)) {
        return { url: new URL(requested.href.slice(sourceRoot.href.length), distributionRoot).href, shortCircuit: true };
      }
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith(sourceRoot.href) && url.endsWith('.ts')) {
      const source = ts.transpileModule(readFileSync(new URL(url), 'utf8'), {
        fileName: fileURLToPath(url),
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        reportDiagnostics: true,
      });
      assert.deepEqual(source.diagnostics, [], fileURLToPath(url));
      return { format: 'module', source: source.outputText, shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});
