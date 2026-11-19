import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const wholeFile = fileURLToPath(new URL('../explorer.json', import.meta.url));
const whole = JSON.parse(await readFile(wholeFile, 'utf8'));
const names = ['home', 'blocks', 'transactions', 'addresses', 'tokens', 'statistics', 'chains', 'rollup', 'services'];
const seen = new Set();
const crossChainViews = new Set(['/cc/tx/[hash]', '/cross-chain-tx/[id]']);
const membership = new Map();

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
  const partNames = new Set();
  for (const view of part.views) {
    assert.ok(!partNames.has(view.name), `${name}: duplicate view ${view.name}`);
    partNames.add(view.name);
    const owners = membership.get(view.name) ?? [];
    owners.push(name);
    membership.set(view.name, owners);
    seen.add(view.name);
    const expected = whole.views.find((candidate) => candidate.name === view.name);
    assert.ok(expected, `${name}: unknown view ${view.name}`);
    const documentRefusal = expected.expects?.find((entry) =>
      entry.method === 'GET' && entry.status === 404 && entry.pattern === new URL(expected.path, whole.address).href);
    const expectedInPart = name === 'chains' && documentRefusal && !expected.expects.some((entry) => entry.method === 'HEAD') ? {
      ...expected,
      expects: [...expected.expects, {
        method: 'HEAD',
        pattern: `${whole.address}/explorer/_next/data/*${expected.path.slice('/explorer'.length)}.json`,
        status: 404,
      }],
    } : expected;
    assert.deepEqual(await normalise(view, base), await normalise(expectedInPart, dirname(wholeFile)), `${name}: view ${view.name}`);
  }
  if (name === 'chains') {
    assert.deepEqual([...partNames].sort(), whole.views
      .filter((view) => view.name.startsWith('/chain/') || view.name === '/chains' || view.name.startsWith('/chains/') || crossChainViews.has(view.name))
      .map((view) => view.name).sort(), 'Chain and cross-chain views');
  }
  if (name === 'rollup') {
    const measuring = JSON.parse(await readFile(new URL('./measure.json', import.meta.url), 'utf8'));
    const dimensions = JSON.parse(await readFile(new URL('./rollup-measure.json', import.meta.url), 'utf8'));
    const paths = new Set(part.views.map((view) => view.path));
    assert.deepEqual(dimensions, {
      ...measuring,
      name: 'Paxscan explorer rollup dimensions',
      views: measuring.views.filter((view) => paths.has(view.path)),
    }, 'Rollup dimension coverage');
    assert.equal(dimensions.views.length, part.views.length, 'Rollup measured views');
  }
  console.log(`${name}: ${part.views.length} views`);
}
for (const view of whole.views) {
  assert.ok(seen.has(view.name), `Missing view ${view.name}`);
  const owners = membership.get(view.name).sort();
  if (crossChainViews.has(view.name)) {
    assert.deepEqual(owners, ['chains', 'transactions'], `Cross-chain coverage: ${view.name}`);
  } else {
    assert.equal(owners.length, 1, `Exclusive view: ${view.name}`);
  }
}
assert.equal(seen.size, whole.views.length, 'Explorer view count');
console.log(`Explorer: ${seen.size} views in ${names.length} parts`);
