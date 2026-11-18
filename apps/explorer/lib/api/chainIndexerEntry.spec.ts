import { afterEach, beforeEach, expect, test, vi } from 'vitest';

beforeEach(() => {
  vi.resetModules();
  vi.stubEnv('NEXT_PUBLIC_APP_HOST', 'localhost');
  vi.stubEnv('NEXT_PUBLIC_APP_PROTOCOL', 'http');
  vi.stubEnv('NEXT_PUBLIC_APP_ENV', 'testing');
  vi.stubEnv('NEXT_PUBLIC_USE_NEXT_JS_PROXY', 'false');
  vi.stubEnv('NEXT_PUBLIC_API_HOST', 'localhost');
  vi.stubEnv('NEXT_PUBLIC_API_PROTOCOL', 'http');
  vi.stubEnv('NEXT_PUBLIC_PAXEER_X_ENABLED', 'true');
  vi.stubEnv('NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST', 'https://example.com');
  vi.stubEnv('NEXT_PUBLIC_PAXEER_X_INDEXER_API_BASE_PATH', '/network/indexer/');
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

test('chainIndexerEntryBuildsApplicationChainUrls', async function chainIndexerEntryBuildsApplicationChainUrls() {
  const { 'default': config } = await import('configs/app');
  const { 'default': buildUrl } = await import('./buildUrl');
  const { 'default': getResourceParams } = await import('./getResourceParams');

  expect(config.apis.paxeerXIndexer).toEqual({
    endpoint: 'https://example.com',
    basePath: '/network/indexer',
  });
  expect(Object.isFrozen(config.apis.paxeerXIndexer)).toBe(true);
  for (const namespace of [ 'general', 'paxeerXIndexer' ] as const) {
    expect(getResourceParams(`${ namespace }:paxeer_x_chains`).api).toBe(config.apis.paxeerXIndexer);
    expect(buildUrl(`${ namespace }:paxeer_x_chains`))
      .toBe('https://example.com/network/indexer/v1/chains');
    expect(buildUrl(`${ namespace }:paxeer_x_chain`, { id: '7' }))
      .toBe('https://example.com/network/indexer/v1/chains/7');
  }
});

test('chainIndexerEntryAllowsAnOptionalBasePath', async function chainIndexerEntryAllowsAnOptionalBasePath() {
  vi.stubEnv('NEXT_PUBLIC_PAXEER_X_INDEXER_API_BASE_PATH', undefined);
  const { 'default': config } = await import('configs/app');
  const { 'default': buildUrl } = await import('./buildUrl');

  expect(config.apis.paxeerXIndexer).toEqual({ endpoint: 'https://example.com', basePath: '' });
  expect(buildUrl('general:paxeer_x_chains')).toBe('https://example.com/v1/chains');
  expect(buildUrl('general:paxeer_x_chain', { id: '7' })).toBe('https://example.com/v1/chains/7');
});

test('chainIndexerEntryPreservesChainConfigurationPrecedence', async function chainIndexerEntryPreservesChainConfigurationPrecedence() {
  const { 'default': config } = await import('configs/app');
  const { 'default': buildUrl } = await import('./buildUrl');
  const { 'default': getResourceParams } = await import('./getResourceParams');
  const chain = {
    id: '125',
    name: 'Root network',
    slug: 'root',
    app_config: {
      ...config,
      apis: {
        ...config.apis,
        paxeerXIndexer: { endpoint: 'https://example.com', basePath: '/chain/indexer' },
      },
    },
  };

  expect(getResourceParams('general:paxeer_x_chains', chain).api).toBe(chain.app_config.apis.paxeerXIndexer);
  expect(buildUrl('general:paxeer_x_chain', { id: '7' }, undefined, true, chain))
    .toBe('https://example.com/chain/indexer/v1/chains/7');
});

test('chainIndexerEntryRefusesReadsWithoutAHost', async function chainIndexerEntryRefusesReadsWithoutAHost() {
  vi.stubEnv('NEXT_PUBLIC_PAXEER_X_INDEXER_API_HOST', undefined);
  const { 'default': config } = await import('configs/app');
  const { 'default': paxeerX } = await import('configs/app/features/paxeerX');
  const { 'default': buildUrl } = await import('./buildUrl');
  const { 'default': getResourceParams } = await import('./getResourceParams');
  const { PAXEER_X_CHAIN_RESOURCES } = await import('./resources');

  expect(paxeerX.isEnabled).toBe(true);
  expect(config.apis.paxeerXIndexer).toBeUndefined();
  expect(config.apis.general).toBeDefined();
  for (const name of Object.keys(PAXEER_X_CHAIN_RESOURCES) as Array<keyof typeof PAXEER_X_CHAIN_RESOURCES>) {
    expect(() => getResourceParams(`general:${ name }`)).toThrow('API config for paxeerXIndexer not found');
    expect(() => buildUrl(`general:${ name }`, { id: '7' })).toThrow('API config for paxeerXIndexer not found');
  }
});
