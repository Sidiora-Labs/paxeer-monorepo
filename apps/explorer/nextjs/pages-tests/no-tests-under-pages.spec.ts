// @vitest-environment node

import { readdir } from 'node:fs/promises';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

import { expect, test } from 'vitest';

const explorerDirectory = fileURLToPath(new URL('../../', import.meta.url));
const pagesDirectory = join(explorerDirectory, 'pages');

async function findPageTestFiles(directory: string): Promise<Array<string>> {
  const entries = await readdir(directory, { withFileTypes: true });
  const matches = await Promise.all(entries.map(async(entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      return findPageTestFiles(path);
    }
    if (entry.name.includes('.spec.') || entry.name.includes('.test.') || entry.name.endsWith('.kvx')) {
      return [ relative(explorerDirectory, path) ];
    }
    return [];
  }));
  return matches.flat().sort();
}

test('noTestsUnderPages', async function noTestsUnderPages() {
  const offendingPaths = await findPageTestFiles(pagesDirectory);
  expect(offendingPaths).toEqual([]);
});
