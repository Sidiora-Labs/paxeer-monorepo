import { randomUUID } from 'node:crypto';
import { readFile, unlink, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';

import { checkSurface } from '../../check.mjs';

const { values } = parseArgs({ options: { out: { type: 'string' }, view: { type: 'string' } } });
if (!values.out) throw new Error('An output directory is required');

const part = JSON.parse(await readFile(new URL('./parts/chains.json', import.meta.url), 'utf8'));
const measure = JSON.parse(await readFile(new URL('./measure.json', import.meta.url), 'utf8'));
const surface = {
  ...measure,
  name: `${part.name} geometry`,
  views: part.views.map((view) => measure.views.find((measured) => measured.path === view.path) ?? view),
};
const filename = new URL(`./.chains-geometry-${randomUUID()}.json`, import.meta.url);

await writeFile(filename, `${JSON.stringify(surface, null, 2)}\n`, { flag: 'wx' });
try {
  const result = await checkSurface({ surface: fileURLToPath(filename), out: values.out, view: values.view });
  process.exitCode = result.passed ? 0 : 1;
} finally {
  await unlink(filename);
}
