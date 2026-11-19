import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const paths = ['/explorer/accounts', '/explorer/verified-contracts'];
const preparedPagesFile = '/tmp/explorer-address-pages.json';
const surfaceFile = new URL('./parts/addresses.json', import.meta.url);
const buildFile = new URL('../../../../apps/explorer/.next/BUILD_ID', import.meta.url);

export async function prepareAddressResponses() {
  const { readSurface } = await import('../../check.mjs');
  const surface = await readSurface(fileURLToPath(surfaceFile));
  const views = surface.views.filter((view) => paths.includes(view.path));
  assert.deepEqual(views.map((view) => view.path), paths);
  const pages = [];
  for (const view of views) {
    const response = await fetch(new URL(view.path, surface.address), { signal: AbortSignal.timeout(10000) });
    assert.equal(response.status, 200, `${view.name}: document status`);
    const body = await response.text();
    assert.ok(body.includes('id="__NEXT_DATA__"'), `${view.name}: application page data`);
    pages.push({ path: view.path, body, type: response.headers.get('content-type') });
  }
  const buildId = await readFile(buildFile, 'utf8');
  const source = await readFile(surfaceFile);
  const surfaceHash = createHash('sha256').update(source).digest('hex');
  await writeFile(preparedPagesFile, JSON.stringify({ buildId, surfaceHash, pages }));
}

export async function readAddressResponses() {
  const buildId = await readFile(buildFile, 'utf8');
  const prepared = JSON.parse(await readFile(preparedPagesFile, 'utf8'));
  assert.equal(prepared.buildId, buildId, 'Prepared address pages match the application build');
  const source = await readFile(surfaceFile);
  assert.equal(prepared.surfaceHash, createHash('sha256').update(source).digest('hex'), 'Prepared address pages match the surface');
  assert.deepEqual(prepared.pages.map((page) => page.path), paths);
  return new Map(prepared.pages.map((page) => {
    const body = Buffer.from(page.body);
    return [page.path, { body, compressed: gzipSync(body), type: page.type }];
  }));
}
