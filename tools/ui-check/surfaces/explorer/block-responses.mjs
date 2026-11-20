import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { gzipSync } from 'node:zlib';

export async function readBlockResponses() {
  const buildId = await readFile(new URL('../../../../apps/explorer/.next/BUILD_ID', import.meta.url), 'utf8');
  const prepared = JSON.parse(await readFile('/tmp/explorer-block-pages.json', 'utf8'));
  assert.equal(prepared.buildId, buildId, 'Prepared block pages match the application build');
  const source = await readFile(new URL('./parts/blocks.json', import.meta.url));
  assert.equal(prepared.surfaceHash, createHash('sha256').update(source).digest('hex'), 'Prepared block pages match the surface');
  const surface = JSON.parse(source.toString());
  assert.deepEqual(prepared.pages.map((page) => page.path), surface.views.map((view) => view.path));
  return new Map(prepared.pages.map((page) => {
    const body = Buffer.from(page.body);
    return [page.path, { body, compressed: gzipSync(body), type: page.type }];
  }));
}
