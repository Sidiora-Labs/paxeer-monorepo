import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { existsSync, readFileSync, realpathSync, statSync } from 'node:fs';
import { basename, isAbsolute, relative, resolve, sep, win32 } from 'node:path';
import { fileURLToPath } from 'node:url';
import * as exportedAbis from '../src/abis.ts';

const authoritativeSources = {
  factoryAbi: 'contracts/launchpad/contracts/core/SidioraFactory.sol',
  routerAbi: 'contracts/launchpad/contracts/periphery/Router.sol',
  quoterAbi: 'contracts/launchpad/contracts/periphery/Quoter.sol',
  protocolConfigAbi: 'contracts/launchpad/contracts/protocol/ProtocolConfig.sol',
  erc20Abi: 'contracts/launchpad/contracts/core/SidioraERC20.sol',
};

export function containedProvenanceFile(repositoryRoot, path, label) {
  assert.equal(typeof path, 'string', `${label}: must be a path`);
  assert.ok(path.length > 0, `${label}: must not be empty`);
  assert.ok(!isAbsolute(path), `${label}: must be relative`);
  assert.ok(!win32.isAbsolute(path), `${label}: must be relative on Windows`);
  const filePath = resolve(repositoryRoot, path);
  const lexicalPath = relative(repositoryRoot, filePath);
  assert.ok(
    lexicalPath !== '..' && !lexicalPath.startsWith(`..${sep}`) && !isAbsolute(lexicalPath),
    `${label}: must resolve inside the repository`,
  );
  assert.ok(existsSync(filePath), `${label}: must exist`);
  const repositoryPath = relative(repositoryRoot, realpathSync(filePath));
  assert.ok(
    repositoryPath !== '..' && !repositoryPath.startsWith(`..${sep}`) && !isAbsolute(repositoryPath),
    `${label}: must resolve inside the repository`,
  );
  assert.ok(statSync(filePath).isFile(), `${label}: must be a file`);
  return filePath;
}

export function assertProvenanceDigest(bytes, digest, label) {
  assert.match(digest, /^[a-f0-9]{64}$/, `${label}: SHA-256 must be lowercase hex`);
  assert.equal(createHash('sha256').update(bytes).digest('hex'), digest, `${label}: SHA-256 must match`);
}

export function assertProvenanceLicense(source, license, label) {
  assert.equal(typeof license, 'string', `${label}: license must be a marker`);
  assert.match(license, /^\/\/ SPDX-License-Identifier: \S+(?: \S+)*$/, `${label}: license must be an SPDX marker`);
  const markers = source.split(/\r?\n/).map(line => line.trim()).filter(line => line.startsWith('// SPDX-License-Identifier:'));
  assert.deepEqual(markers, [license], `${label}: source license marker must match`);
}

export function assertExportedArtifactAbi(entry, artifact) {
  assert.ok(Object.hasOwn(exportedAbis, entry.export), `${entry.export}: must name an exported ABI`);
  assert.ok(Array.isArray(artifact), `${entry.export}: artifact must be an ABI array`);
  assert.deepEqual(exportedAbis[entry.export], artifact, `${entry.export}: exported ABI must match authoritative artifact`);
}

function provenanceArtifactsResolveInRepository() {
  const repositoryRoot = realpathSync(fileURLToPath(new URL('../../../../../', import.meta.url)));
  const provenancePath = containedProvenanceFile(repositoryRoot, 'sdk/launchpad/packages/sidiora-launchpad-v3/provenance.json', 'provenance');
  const provenance = JSON.parse(readFileSync(provenancePath, 'utf8'));
  assert.ok(Array.isArray(provenance.abi_sources));
  assert.ok(provenance.abi_sources.length > 0);
  assert.deepEqual(
    provenance.abi_sources.map(entry => entry.export).sort(),
    Object.keys(exportedAbis).sort(),
    'provenance must cover every exported ABI exactly once',
  );

  for (const entry of provenance.abi_sources) {
    const sourceIdentity = authoritativeSources[entry.export];
    assert.equal(entry.source, sourceIdentity, `${entry.export}: source must match authoritative contract identity`);
    assert.equal(
      entry.artifact,
      `precompiles/launchpad/abi/${basename(sourceIdentity, '.sol')}.json`,
      `${entry.export}: artifact must match authoritative ABI identity`,
    );
    const artifactPath = containedProvenanceFile(repositoryRoot, entry.artifact, `${entry.export}: artifact`);
    const artifact = readFileSync(artifactPath);
    assertProvenanceDigest(artifact, entry.artifact_sha256, `${entry.export}: artifact`);
    const sourcePath = containedProvenanceFile(repositoryRoot, entry.source, `${entry.export}: source`);
    const source = readFileSync(sourcePath);
    assertProvenanceDigest(source, entry.source_sha256, `${entry.export}: source`);
    assertProvenanceLicense(source.toString('utf8'), entry.license, entry.export);
    const parsedArtifact = JSON.parse(artifact.toString('utf8'));
    assertExportedArtifactAbi(entry, parsedArtifact);
  }
}

test(provenanceArtifactsResolveInRepository);
