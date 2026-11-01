import { afterEach, expect, test, vi } from 'vitest';
import { parsePrivateFixtureAuthorityProfileV1, type PrivateFixtureAuthorityProfileV1 } from './runtime-authority';

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
    expect(() => parsePrivateFixtureAuthorityProfileV1({
      ...profile,
      roles: { ...profile.roles, USDL: { ...profile.roles.USDL, kind: 'unsupported' } },
    })).toThrow(new Error('Reviewed local contract identity is unavailable'));
    expect(() => parsePrivateFixtureAuthorityProfileV1({
      ...profile,
      roles: { ...profile.roles, Router: { ...profile.roles.Router, kind: 'beacon' } },
    })).toThrow(new Error('Reviewed local execution identity is unsupported'));
  }
}

test(privateFixtureProfileRefusedInProduction);
test(privateFixtureProfileParsedOutsideProduction);
