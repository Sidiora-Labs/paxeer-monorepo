import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { expect, test } from 'vitest';

function workspaceScriptFiltersResolve() {
  const launchpadRoot = resolve(__dirname, '../../..');
  const sdkPackagesRoot = resolve(launchpadRoot, '../../sdk/launchpad/packages');
  const packageDirectories = [
    resolve(launchpadRoot, 'web'),
    resolve(launchpadRoot, 'mobile'),
    ...readdirSync(sdkPackagesRoot, { withFileTypes: true })
      .filter(entry => entry.isDirectory())
      .map(entry => resolve(sdkPackagesRoot, entry.name)),
  ];
  const packageNames = new Set(
    packageDirectories.map(directory => {
      const manifest: { name: string } = JSON.parse(
        readFileSync(resolve(directory, 'package.json'), 'utf8'),
      );
      expect(manifest.name).toBeTypeOf('string');
      expect(manifest.name.length).toBeGreaterThan(0);
      return manifest.name;
    }),
  );
  const { scripts }: { scripts: Record<string, string> } = JSON.parse(
    readFileSync(resolve(launchpadRoot, 'package.json'), 'utf8'),
  );

  expect(scripts).not.toHaveProperty('dev:bot');
  expect(scripts).not.toHaveProperty('build:bot');
  let filterCount = 0;
  for (const [scriptName, command] of Object.entries(scripts)) {
    const filters = command.matchAll(/(?:^|\s)--filter(?:=|\s+)(('[^']*'|"[^"]*"|[^\s]+))/g);
    for (const match of filters) {
      const target = match[1].replace(/^(['"])(.*)\1$/, '$2');
      filterCount += 1;
      if (scriptName === 'build:packages' && target === '../../sdk/launchpad/packages/*') {
        expect(packageDirectories.length).toBeGreaterThan(2);
      } else {
        expect(
          packageNames.has(target),
          `${scriptName} filters unknown workspace package ${target}`,
        ).toBe(true);
      }
    }
  }
  expect(filterCount).toBeGreaterThan(0);
}

test(workspaceScriptFiltersResolve);
