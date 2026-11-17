import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const wholeFile = fileURLToPath(new URL('../explorer.json', import.meta.url));
const whole = JSON.parse(await readFile(wholeFile, 'utf8'));
const names = ['home', 'blocks', 'transactions', 'addresses', 'tokens', 'statistics', 'chains', 'rollup', 'services'];
const seen = new Set();

async function normalise(value, base) {
  if (Array.isArray(value)) return Promise.all(value.map((entry) => normalise(entry, base)));
  if (value && typeof value === 'object') {
    return Object.fromEntries(await Promise.all(Object.entries(value).map(async ([key, entry]) => [
      key,
      key === 'file' ? (await readFile(resolve(base, entry))).toString('base64') : await normalise(entry, base),
    ])));
  }
  return value;
}

assert.deepEqual((await readdir(new URL('./parts/', import.meta.url))).filter((name) => name.endsWith('.json')).sort(),
  names.map((name) => `${name}.json`).sort(), 'Explorer parts');
for (const name of names) {
  const filename = fileURLToPath(new URL(`./parts/${name}.json`, import.meta.url));
  const part = JSON.parse(await readFile(filename, 'utf8'));
  const base = dirname(filename);
  for (const key of ['build', 'buildMs', 'outputs', 'start', 'address', 'readyPath', 'typeScale']) {
    assert.deepEqual(part[key], whole[key], `${name}: ${key}`);
  }
  assert.equal(resolve(base, part.directory), resolve(dirname(wholeFile), whole.directory), `${name}: directory`);
  assert.deepEqual(await normalise(part.answers, base), await normalise(whole.answers, dirname(wholeFile)), `${name}: answers`);
  assert.ok(part.views.length > 0, `${name}: views`);
  for (const view of part.views) {
    assert.ok(!seen.has(view.name), `${name}: duplicate view ${view.name}`);
    seen.add(view.name);
    const expected = whole.views.find((candidate) => candidate.name === view.name);
    assert.ok(expected, `${name}: unknown view ${view.name}`);
    assert.deepEqual(await normalise(view, base), await normalise(expected, dirname(wholeFile)), `${name}: view ${view.name}`);
  }
  console.log(`${name}: ${part.views.length} views`);
}
for (const view of whole.views) assert.ok(seen.has(view.name), `Missing view ${view.name}`);
assert.equal(seen.size, whole.views.length, 'Explorer view count');
console.log(`Explorer: ${seen.size} views in ${names.length} parts`);
