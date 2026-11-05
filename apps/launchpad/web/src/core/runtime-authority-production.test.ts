// @vitest-environment jsdom

import { readFile } from 'node:fs/promises';
import { createServer } from 'node:https';
import { join } from 'node:path';
import { getCACertificates, setDefaultCACertificates } from 'node:tls';
import type { ConnectedNetworkAuthorityResponse } from '../../../../../sdk/launchpad/packages/sdk-v2/src/types.gen';
import { afterEach, expect, test, vi } from 'vitest';
import { CORE_DEPLOYMENT } from './network/deployment';
import {
  loadRuntimeAuthority,
  parsePrivateFixtureAuthorityProfileV1,
  type PrivateFixtureAuthorityProfileV1,
} from './runtime-authority';

function privateFixtureProfile(): PrivateFixtureAuthorityProfileV1 {
  const digest = '1'.repeat(64);
  const direct = {
    kind: 'direct',
    address: '0x1111111111111111111111111111111111111111',
    runtimeHash: `0x${'a'.repeat(64)}`,
  } as const;
  const proxy = {
    ...direct,
    kind: 'erc1967',
    implementationAddress: '0x2222222222222222222222222222222222222222',
    implementationHash: `0x${'b'.repeat(64)}`,
  } as const;
  return {
    schemaVersion: 1,
    profile: 'PRIVATE_FIXTURE',
    chainId: 125,
    applicationOrigin: 'https://localhost:3050',
    sourceManifestSha256: digest,
    revision: {
      schemaVersion: 2,
      deploymentVersion: 'private-qualification-v1',
      manifestSha256: digest,
      capabilitySha256: digest,
      contractSupportSha256: digest,
      serialInterfaceSha256: digest,
      revisionSha256: digest,
    },
    networkSha256: digest,
    publicRPC: 'https://127.0.0.1:8545',
    explorerBase: 'https://localhost:4000',
    expectedBeaconOwner: direct.address,
    roles: {
      EventEmitter: direct,
      ProtocolConfig: direct,
      Router: proxy,
      Quoter: proxy,
      SidioraFactory: direct,
      PoolRegistry: proxy,
      FeeAccumulator: direct,
      SidioraNFT: direct,
      FeesRouter: direct,
      OpticalRegistry: direct,
      PoolBeacon: { ...proxy, kind: 'beacon' },
      USDL: direct,
    },
    sourceArtifacts: {
      chainSha256: digest,
      compilerArtifactsSha256: digest,
      reviewedManifestSha256: digest,
    },
  };
}

afterEach(() => vi.unstubAllEnvs());

function privateFixtureProfileRefusedInProduction() {
  vi.stubEnv('NODE_ENV', 'production');
  const profile = privateFixtureProfile();

  for (const input of [profile, JSON.stringify(profile), '', '{', null, false, 0]) {
    expect(() => parsePrivateFixtureAuthorityProfileV1(input)).toThrow(
      new Error('Private fixture authority is unavailable in production'),
    );
  }
  expect(() => parsePrivateFixtureAuthorityProfileV1(undefined)).toThrow(
    new Error('Network authority response is invalid'),
  );
}

function privateFixtureProfileParsedOutsideProduction() {
  const profile = privateFixtureProfile();

  for (const environment of ['development', 'test']) {
    vi.stubEnv('NODE_ENV', environment);
    expect(parsePrivateFixtureAuthorityProfileV1(profile)).toEqual(profile);
    expect(parsePrivateFixtureAuthorityProfileV1(JSON.stringify(profile))).toEqual(profile);

    for (const input of [
      { ...profile, schemaVersion: 2 },
      { ...profile, profile: 'PUBLIC' },
      { ...profile, chainId: 1 },
      { ...profile, revision: { ...profile.revision, schemaVersion: 1 } },
    ]) {
      expect(() => parsePrivateFixtureAuthorityProfileV1(input)).toThrow(
        new Error('Reviewed local network input is unavailable'),
      );
    }
    expect(() =>
      parsePrivateFixtureAuthorityProfileV1({
        ...profile,
        roles: { ...profile.roles, USDL: { ...profile.roles.USDL, kind: 'unsupported' } },
      }),
    ).toThrow(new Error('Reviewed local contract identity is unavailable'));
    expect(() =>
      parsePrivateFixtureAuthorityProfileV1({
        ...profile,
        roles: { ...profile.roles, Router: { ...profile.roles.Router, kind: 'beacon' } },
      }),
    ).toThrow(new Error('Reviewed local execution identity is unsupported'));
  }
}

test(privateFixtureProfileRefusedInProduction.name, privateFixtureProfileRefusedInProduction);
test(
  privateFixtureProfileParsedOutsideProduction.name,
  privateFixtureProfileParsedOutsideProduction,
);

declare const jsdom: { reconfigure(options: { url: string }): void };

async function withRecordedProductionAuthority(
  run: (state: {
    emitted: ConnectedNetworkAuthorityResponse;
    served: ConnectedNetworkAuthorityResponse;
    requests: string[];
  }) => Promise<void>,
) {
  const directory = join(__dirname, 'test-fixtures/authority-production');
  const [record, key, cert] = await Promise.all([
    readFile(join(directory, 'go-authority.json'), 'utf8'),
    readFile(join(directory, 'loopback-key.pem.txt')),
    readFile(join(directory, 'loopback-cert.pem.txt')),
  ]);
  const emitted = JSON.parse(record) as ConnectedNetworkAuthorityResponse;
  const state = { emitted, served: structuredClone(emitted), requests: [] as string[] };
  const server = createServer({ key, cert }, (request, response) => {
    state.requests.push(`${request.method} ${request.url}`);
    if (request.method !== 'GET' || request.url !== '/v2/network/authority') {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(200, {
      'Content-Type': 'application/json',
      'Cache-Control': 'private, no-store',
    });
    response.end(JSON.stringify(state.served));
  });
  const trusted = getCACertificates();
  const previousURL = window.location.href;
  try {
    setDefaultCACertificates([...trusted, cert.toString()]);
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('Authority listener unavailable');
    jsdom.reconfigure({ url: `https://127.0.0.1:${address.port}` });
    // Replay the attested observation at its recorded time; its bytes stay intact.
    vi.setSystemTime(Date.parse(emitted.attestation.observedAt));
    vi.stubEnv('NODE_ENV', 'production');
    await run(state);
  } finally {
    vi.useRealTimers();
    jsdom.reconfigure({ url: previousURL });
    setDefaultCACertificates(trusted);
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
}

async function authorityProductionCoverageAcceptsCurrentGoProducer() {
  await withRecordedProductionAuthority(async (state) => {
    expect(state.emitted.schemaVersion).toBe(2);
    expect(state.emitted.attestation.eligible).toBe(true);
    expect(state.emitted.serialExport.network.sourceManifestSha256).toBe(
      CORE_DEPLOYMENT.manifestSha256,
    );
    const authority = await loadRuntimeAuthority(AbortSignal.timeout(15_000));
    expect(authority.revision).toEqual(state.emitted.serialExport.revision);
    for (const [role, field] of [
      ['Router', 'router'],
      ['Quoter', 'quoter'],
      ['PoolRegistry', 'poolRegistry'],
    ] as const) {
      expect(CORE_DEPLOYMENT.roleIdentity[field].kind).toBe('surface');
      expect(
        authority.tradeDeployment.pins.find((pin) => pin.address === CORE_DEPLOYMENT[field]),
      ).toEqual({
        address: CORE_DEPLOYMENT[field],
        runtimeHash: state.emitted.attestation.identities[role].runtimeHash,
      });
    }
    expect(authority.network.usdlRuntimeHash).toBe(CORE_DEPLOYMENT.roleIdentity.usdl.runtimeHash);
    expect(await authority.network.committedHead(AbortSignal.timeout(15_000))).toEqual({
      number: BigInt(state.emitted.committedHead.number),
      hash: state.emitted.committedHead.hash,
    });
    expect(state.requests).toEqual(['GET /v2/network/authority', 'GET /v2/network/authority']);
  });
}

async function authorityProductionCoverageRefusesChangedRuntimeAndRevision() {
  await withRecordedProductionAuthority(async (state) => {
    const authority = await loadRuntimeAuthority(AbortSignal.timeout(15_000));
    state.served.attestation.identities.Router.runtimeHash = `0x${'1'.repeat(64)}`;
    await expect(loadRuntimeAuthority(AbortSignal.timeout(15_000))).rejects.toThrow(
      'Reviewed network runtime differs for Router',
    );
    await expect(authority.network.committedHead(AbortSignal.timeout(15_000))).rejects.toThrow(
      'Reviewed network runtime differs for Router',
    );
    state.served = structuredClone(state.emitted);
    state.served.serialExport.revision.revisionSha256 = '2'.repeat(64);
    await expect(loadRuntimeAuthority(AbortSignal.timeout(15_000))).rejects.toThrow(
      'Canonical deployment revision differs',
    );
    state.served = structuredClone(state.emitted);
    // A second internally valid producer revision must still invalidate a wallet
    // that pinned the first revision, even though its contract runtimes agree.
    const revision = state.served.serialExport.revision;
    revision.serialInterfaceSha256 = '3'.repeat(64);
    revision.revisionSha256 = '';
    const { createHash } = await import('node:crypto');
    revision.revisionSha256 = createHash('sha256')
      .update(
        JSON.stringify({
          schemaVersion: 2,
          deploymentVersion: revision.deploymentVersion,
          manifestSha256: revision.manifestSha256,
          capabilitySha256: revision.capabilitySha256,
          contractSupportSha256: revision.contractSupportSha256,
          serialInterfaceSha256: revision.serialInterfaceSha256,
          revisionSha256: '',
        }),
      )
      .digest('hex');
    await expect(authority.network.committedHead(AbortSignal.timeout(15_000))).rejects.toThrow(
      'Network revision changed; refresh before continuing',
    );
  });
}

test(
  authorityProductionCoverageAcceptsCurrentGoProducer.name,
  authorityProductionCoverageAcceptsCurrentGoProducer,
);
test(
  authorityProductionCoverageRefusesChangedRuntimeAndRevision.name,
  authorityProductionCoverageRefusesChangedRuntimeAndRevision,
);
