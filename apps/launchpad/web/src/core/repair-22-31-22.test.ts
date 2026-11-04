// @vitest-environment jsdom

import { execFile } from 'node:child_process';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { createServer, type Server } from 'node:https';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { getCACertificates, setDefaultCACertificates } from 'node:tls';
import { promisify } from 'node:util';
import { getAddress } from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { CORE_DEPLOYMENT } from './network/deployment';
import { loadRuntimeAuthority, type RuntimeAuthorityRoleV1 } from './runtime-authority';

const execute = promisify(execFile);
declare const jsdom: { reconfigure(options: { url: string }): void };
type Identity = {
  role: string;
  address: string;
  runtimeHash: string;
  implementationAddress?: string;
  implementationHash?: string;
  beaconAddress?: string;
  verified: boolean;
};
type AuthorityResponse = {
  schemaVersion: number;
  serialExport: { revision: { revisionSha256: string }; network: { sourceManifestSha256: string } };
  attestation: { eligible: boolean; identities: Record<RuntimeAuthorityRoleV1, Identity> };
  committedHead: { number: string; hash: string };
};

let emitted: AuthorityResponse;
let served: AuthorityResponse;
let server: Server | undefined;
let directory: string | undefined;
let trustedCertificates: string[] | undefined;
const requests: string[] = [];

beforeAll(async () => {
  const root = resolve(__dirname, '../../../../..');
  const produced = await execute('go', ['run', resolve(__dirname, 'repair-22-31-22/attest.go')], {
    cwd: join(root, 'sdk/launchpad/shared'),
    env: { ...process.env, GOWORK: 'off' },
    timeout: 240_000,
    maxBuffer: 1024 * 1024,
  });
  emitted = JSON.parse(produced.stdout) as AuthorityResponse;
  served = structuredClone(emitted);
  directory = await mkdtemp(join(tmpdir(), 'authority-22-31-22-'));
  const keyPath = join(directory, 'key.pem'), certPath = join(directory, 'cert.pem');
  await execute('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', keyPath, '-out', certPath, '-days', '1', '-subj', '/CN=localhost', '-addext', 'subjectAltName=DNS:localhost,IP:127.0.0.1']);
  const [key, cert] = await Promise.all([readFile(keyPath), readFile(certPath)]);
  trustedCertificates = getCACertificates();
  setDefaultCACertificates([...trustedCertificates, cert.toString()]);
  server = createServer({ key, cert }, (request, response) => {
    requests.push(`${request.method} ${request.url}`);
    if (request.method !== 'GET' || request.url !== '/v2/network/authority') {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'private, no-store' });
    response.end(JSON.stringify(served));
  });
  await new Promise<void>((resolve, reject) => {
    server!.once('error', reject);
    server!.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Authority HTTPS listener unavailable');
  jsdom.reconfigure({ url: `https://127.0.0.1:${address.port}` });
}, 300_000);

afterAll(async () => {
  if (server) {
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server!.close(error => error ? reject(error) : resolve()));
  }
  if (trustedCertificates) setDefaultCACertificates(trustedCertificates);
  if (directory) await rm(directory, { recursive: true, force: true });
});

async function surfaceAuthorityRoundTripsCurrentAttestation() {
  expect(emitted.schemaVersion).toBe(2);
  expect(emitted.attestation.eligible).toBe(true);
  expect(emitted.serialExport.network.sourceManifestSha256).toBe(CORE_DEPLOYMENT.manifestSha256);
  served = structuredClone(emitted);
  const authority = await loadRuntimeAuthority(AbortSignal.timeout(15_000));
  expect(authority.revision).toEqual(emitted.serialExport.revision);
  expect(authority.tradeDeployment.router).toBe(CORE_DEPLOYMENT.router);
  expect(authority.tradeDeployment.quoter).toBe(CORE_DEPLOYMENT.quoter);
  expect(authority.tradeDeployment.registry).toBe(CORE_DEPLOYMENT.poolRegistry);
  for (const [role, field] of [['Router', 'router'], ['Quoter', 'quoter'], ['PoolRegistry', 'poolRegistry']] as const) {
    const identity = emitted.attestation.identities[role];
    expect(identity.verified).toBe(true);
    expect(identity.runtimeHash).toBe(CORE_DEPLOYMENT.roleIdentity[field].runtimeHash);
    expect(identity).not.toHaveProperty('implementationAddress');
    expect(identity).not.toHaveProperty('implementationHash');
    expect(authority.tradeDeployment.pins.find(pin => pin.address === CORE_DEPLOYMENT[field])).toEqual({
      address: CORE_DEPLOYMENT[field], runtimeHash: identity.runtimeHash,
    });
  }
  expect(authority.tradeDeployment.pins.find(pin => pin.address === CORE_DEPLOYMENT.eventEmitter)).toEqual({
    address: CORE_DEPLOYMENT.eventEmitter,
    runtimeHash: CORE_DEPLOYMENT.roleIdentity.eventEmitter.runtimeHash,
    implementation: CORE_DEPLOYMENT.roleIdentity.eventEmitter.implementationAddress,
    implementationHash: CORE_DEPLOYMENT.roleIdentity.eventEmitter.implementationHash,
  });
  expect(authority.network.usdlRuntimeHash).toBe(CORE_DEPLOYMENT.roleIdentity.usdl.runtimeHash);
  expect(authority.network.network.addresses.USDL).toBe(emitted.attestation.identities.USDL.address);
  expect(await authority.network.committedHead(AbortSignal.timeout(15_000))).toEqual({
    number: BigInt(emitted.committedHead.number), hash: emitted.committedHead.hash,
  });
  expect(requests).toEqual(['GET /v2/network/authority', 'GET /v2/network/authority']);
}

async function surfaceAuthorityRejectsUnreviewedIdentities() {
  const incorrectHash = `0x${'1'.repeat(64)}`;
  const roles = Object.keys(emitted.attestation.identities) as RuntimeAuthorityRoleV1[];
  for (const role of roles.filter(role => ['EventEmitter', 'ProtocolConfig', 'Router', 'Quoter', 'SidioraFactory', 'PoolRegistry', 'FeeAccumulator', 'SidioraNFT', 'FeesRouter', 'OpticalRegistry', 'PoolBeacon', 'USDL'].includes(role))) {
    served = structuredClone(emitted);
    served.attestation.identities[role].runtimeHash = incorrectHash;
    await expect(loadRuntimeAuthority(AbortSignal.timeout(15_000))).rejects.toThrow(`Reviewed network runtime differs for ${role}`);
  }
  for (const role of ['EventEmitter', 'OpticalRegistry', 'PoolBeacon'] as const) {
    for (const mutate of [
      (identity: Identity) => { delete identity.implementationAddress; },
      (identity: Identity) => { delete identity.implementationHash; },
      (identity: Identity) => { identity.implementationAddress = getAddress(CORE_DEPLOYMENT.usdl); },
      (identity: Identity) => { identity.implementationHash = incorrectHash; },
    ]) {
      served = structuredClone(emitted);
      mutate(served.attestation.identities[role]);
      await expect(loadRuntimeAuthority(AbortSignal.timeout(15_000))).rejects.toThrow(`Reviewed network implementation differs for ${role}`);
    }
  }
  for (const role of ['Router', 'Quoter', 'ProtocolConfig', 'SidioraFactory', 'PoolRegistry', 'FeeAccumulator', 'SidioraNFT', 'FeesRouter', 'USDL'] as const) {
    for (const field of ['implementationAddress', 'implementationHash', 'beaconAddress'] as const) {
      served = structuredClone(emitted);
      served.attestation.identities[role][field] = field === 'implementationHash' ? incorrectHash : CORE_DEPLOYMENT.eventEmitter;
      await expect(loadRuntimeAuthority(AbortSignal.timeout(15_000))).rejects.toThrow(`Reviewed network identity differs for ${role}`);
    }
  }
}

test(surfaceAuthorityRoundTripsCurrentAttestation.name, surfaceAuthorityRoundTripsCurrentAttestation);
test(surfaceAuthorityRejectsUnreviewedIdentities.name, surfaceAuthorityRejectsUnreviewedIdentities);
