import assert from 'node:assert/strict';
import { mkdtemp, readlink, rm, symlink, writeFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';

import { cacheTraceReadlinks } from './trace-readlinks.mjs';

const require = createRequire(new URL('../../../../apps/explorer/package.json', import.meta.url));
const tracer = require('next/dist/compiled/@vercel/nft');

async function explorerBuildTracePreservesFileInventory() {
  const directory = await mkdtemp(join(tmpdir(), 'explorer-trace-'));
  try {
    await writeFile(join(directory, 'entry.js'), 'require("./linked.js"); require("./other.js");\n');
    await writeFile(join(directory, 'value.js'), 'module.exports = 7;\n');
    await writeFile(join(directory, 'other.js'), 'module.exports = require("./value.js");\n');
    await symlink('value.js', join(directory, 'linked.js'));
    const originalReads = new Map();
    const cachedReads = new Map();
    const reader = (reads) => async(path) => {
      reads.set(path, (reads.get(path) ?? 0) + 1);
      try {
        return await readlink(path);
      } catch (error) {
        if (['EINVAL', 'ENOENT', 'UNKNOWN'].includes(error.code)) return null;
        throw error;
      }
    };
    const files = [join(directory, 'entry.js')];
    const cachedTracer = cacheTraceReadlinks(tracer);
    assert.deepEqual(Reflect.ownKeys(cachedTracer), Reflect.ownKeys(tracer));
    assert.equal(cachedTracer.Job, tracer.Job);
    const original = await tracer.nodeFileTrace(files, { base: directory, readlink: reader(originalReads) });
    const cached = await cachedTracer.nodeFileTrace(files, { base: directory, readlink: reader(cachedReads) });
    const reasons = (trace) => [...trace.reasons].map(([path, reason]) => [path, {
      ...reason,
      type: [...reason.type].sort(),
      parents: [...reason.parents].sort(),
    }]).sort(([left], [right]) => left.localeCompare(right));

    assert.deepEqual([...cached.fileList].sort(), ['entry.js', 'linked.js', 'other.js', 'value.js']);
    assert.deepEqual([...cached.fileList].sort(), [...original.fileList].sort());
    assert.deepEqual([...cached.esmFileList].sort(), [...original.esmFileList].sort());
    assert.deepEqual(reasons(cached), reasons(original));
    assert.equal(cached.warnings.size, 0);
    assert.equal(original.warnings.size, 0);
    assert.ok([...originalReads.values()].some((count) => count > 1));
    assert.ok(cachedReads.size > 0);
    assert.ok([...cachedReads.values()].every((count) => count === 1));
  } finally {
    await rm(directory, { recursive: true });
  }
}

test(explorerBuildTracePreservesFileInventory);
