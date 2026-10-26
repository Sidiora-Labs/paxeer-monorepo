import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, realpathSync, statSync } from 'node:fs';
import { isAbsolute, relative, resolve, sep, win32 } from 'node:path';
import { fileURLToPath } from 'node:url';

function provenanceArtifactsResolveInRepository() {
  const repositoryRoot = realpathSync(fileURLToPath(new URL('../../../../../', import.meta.url)));
  const provenance = JSON.parse(readFileSync(new URL('../provenance.json', import.meta.url), 'utf8'));
  assert.ok(Array.isArray(provenance.abi_sources));
  assert.ok(provenance.abi_sources.length > 0);

  for (const entry of provenance.abi_sources) {
    assert.equal(typeof entry.artifact, 'string', `${entry.export}: artifact must be a path`);
    assert.ok(entry.artifact.length > 0, `${entry.export}: artifact must not be empty`);
    assert.ok(!isAbsolute(entry.artifact), `${entry.export}: artifact must be relative`);
    assert.ok(!win32.isAbsolute(entry.artifact), `${entry.export}: artifact must be relative on Windows`);

    const artifactPath = resolve(repositoryRoot, entry.artifact);
    assert.ok(existsSync(artifactPath), `${entry.export}: artifact must exist`);
    const repositoryPath = relative(repositoryRoot, realpathSync(artifactPath));
    assert.ok(
      repositoryPath !== '..' && !repositoryPath.startsWith(`..${sep}`) && !isAbsolute(repositoryPath),
      `${entry.export}: artifact must resolve inside the repository`,
    );
    assert.ok(statSync(artifactPath).isFile(), `${entry.export}: artifact must be a file`);
    const sha256 = createHash('sha256').update(readFileSync(artifactPath)).digest('hex');
    assert.equal(sha256, entry.artifact_sha256, `${entry.export}: artifact SHA-256 must match`);
  }
}

test(provenanceArtifactsResolveInRepository);
