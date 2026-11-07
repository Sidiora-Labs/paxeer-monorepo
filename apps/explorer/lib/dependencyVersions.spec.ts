import { existsSync, readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import * as v from 'valibot';

import { expect, test } from 'vitest';

test('explorerDependencyVersionsMatchPins', function explorerDependencyVersionsMatchPins() {
  const explorerRoot = resolve(__dirname, '..');
  const manifestPath = resolve(explorerRoot, 'package.json');
  const explorerRequire = createRequire(manifestPath);
  const versions = v.record(v.string(), v.string());
  const manifest = v.parse(
    v.object({ dependencies: versions, resolutions: versions }),
    JSON.parse(readFileSync(manifestPath, 'utf8')),
  );
  const lockfile = readFileSync(resolve(explorerRoot, 'yarn.lock'), 'utf8');
  const floors = v.parse(
    v.object({ manifests: v.record(v.string(), versions) }),
    JSON.parse(readFileSync(resolve(explorerRoot, '../../tools/deps/floors/explorer-web.json'), 'utf8')),
  );
  const pins = { 'js-cookie': '3.0.7', phoenix: '1.6.17' };
  const lockEntries = Array.from(lockfile.matchAll(/^([^\s#][^\r\n]*):\r?\n((?:[ \t][^\r\n]*(?:\r?\n|$))*)/gm));

  for (const [ name, pin ] of Object.entries(pins)) {
    expect(manifest.dependencies[name]).toBe(pin);
    expect(manifest.resolutions[name]).toBe(pin);
    expect(floors.manifests['apps/explorer/package.json'][name]).toBe(pin);

    let installedRoot = dirname(explorerRequire.resolve(name));
    while (!existsSync(resolve(installedRoot, 'package.json'))) {
      const parent = dirname(installedRoot);
      expect(parent).not.toBe(installedRoot);
      installedRoot = parent;
    }
    const installed = v.parse(
      v.object({ name: v.string(), version: v.string() }),
      JSON.parse(readFileSync(resolve(installedRoot, 'package.json'), 'utf8')),
    );
    expect(installed.name).toBe(name);
    expect(installed.version).toBe(pin);

    const selectors: Array<string> = [];
    for (const [ , header, body ] of lockEntries) {
      const entrySelectors = Array.from(header.matchAll(/"([^"]+)"|([^,\s]+)/g), (match) => match[1] ?? match[2]);
      const packageSelectors = entrySelectors.filter((selector) => selector.startsWith(`${ name }@`));
      for (const selector of packageSelectors) {
        expect(body.match(/^ {2}version "([^"]+)"\r?$/m)?.[1]).toBe(pin);
        selectors.push(selector);
      }
    }
    expect(selectors).toContain(`${ name }@${ pin }`);
  }
});
