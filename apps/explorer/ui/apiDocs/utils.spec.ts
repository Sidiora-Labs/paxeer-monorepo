// @vitest-environment jsdom

import { getFeaturePayload } from 'configs/app/features/types';

import config from 'configs/app';
import { expect, test, vi } from 'vitest';

import { CORE_API_SPEC_PUBLIC_URL, getCoreApiSpecBackendUrl, getCoreApiSpecCandidates, REST_API_SECTIONS } from './utils';

vi.hoisted(() => {
  window.__envs = {
    ...window.__envs,
    NEXT_PUBLIC_API_SPEC_URL: 'https://example.com/openapi.yaml',
  };
});

test('uses the application base path for the shipped spec', function coreApiSpecUsesApplicationBasePath() {
  expect(CORE_API_SPEC_PUBLIC_URL).toBe(`${ config.app.basePath }/openapi.yaml`);
  expect(CORE_API_SPEC_PUBLIC_URL.startsWith(config.app.basePath)).toBe(true);
  expect(CORE_API_SPEC_PUBLIC_URL.endsWith('/openapi.yaml')).toBe(true);
});

test('keeps the configured spec before the shipped spec and backend', function coreApiSpecPreservesCandidateOrder() {
  const configuredUrl = getFeaturePayload(config.features.apiDocs)?.coreApiSpecUrl;
  const backendUrl = getCoreApiSpecBackendUrl();

  expect(configuredUrl).toBe('https://example.com/openapi.yaml');
  expect(backendUrl).toBeDefined();
  expect(getCoreApiSpecCandidates()).toEqual([ configuredUrl, CORE_API_SPEC_PUBLIC_URL, backendUrl ]);
});

test('names the first section core api and keeps its identifier', function coreApiSectionKeepsIdentifier() {
  expect(REST_API_SECTIONS[0]).toMatchObject({
    id: 'blockscout-core-api',
    title: 'Core API',
    swagger: { url: CORE_API_SPEC_PUBLIC_URL },
  });
});
