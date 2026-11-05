import assert from 'node:assert/strict';
import { globSync, readFileSync, statSync } from 'node:fs';
import { isAbsolute, relative, resolve, sep } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';

function packageScriptsReferenceExistingFiles() {
  const root = fileURLToPath(new URL('../', import.meta.url));
  const manifest = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));

  for (const name of ['test:phase2', 'test:phase4', 'build:phase4']) {
    assert.equal(Object.hasOwn(manifest.scripts, name), false, `${name} must remain retired`);
  }
  assert.equal(Object.hasOwn(manifest.kindleCompatibility, 'v2EvidenceManifest'), false);

  for (const [name, script] of Object.entries(manifest.scripts)) {
    const tokens = script.matchAll(/"([^"]*)"|'([^']*)'|([^\s&|;]+)/g);
    for (const token of tokens) {
      let argument = token[1] ?? token[2] ?? token[3];
      if (argument.startsWith('-')) {
        const equals = argument.indexOf('=');
        if (equals === -1) continue;
        argument = argument.slice(equals + 1);
      }
      if (isAbsolute(argument) || /^[a-z][a-z\d+.-]*:/i.test(argument)) continue;
      if (!argument.includes('/') && !/\.[a-z\d]+$/i.test(argument)) continue;

      const localPath = relative(root, resolve(root, argument));
      assert.ok(
        localPath !== '..' && !localPath.startsWith(`..${sep}`),
        `${name}: ${argument} must stay under the package`,
      );
      const files = globSync(argument, { cwd: root });
      assert.ok(files.length > 0, `${name}: ${argument} must reference an existing file`);
      for (const file of files) {
        assert.ok(statSync(resolve(root, file)).isFile(), `${name}: ${file} must be a file`);
      }
    }
  }
}

test(packageScriptsReferenceExistingFiles);
