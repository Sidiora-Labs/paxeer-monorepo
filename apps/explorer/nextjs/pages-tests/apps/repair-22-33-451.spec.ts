import type { GetServerSidePropsResult } from 'next';
import fetch from 'node-fetch';
import { once } from 'node:events';
import { createServer } from 'node:http';

import type { Props } from 'nextjs/getServerSideProps/handlers';

import { apps } from 'mocks/apps/apps';
import { afterAll, expect, test, vi } from 'vitest';

const assetPath = '/explorer/assets/configs/marketplace_config.json';
const catalogueRequests: Array<string | undefined> = [];

const server = createServer(async(req, res) => {
  try {
    if (req.url === assetPath) {
      catalogueRequests.push(req.headers.host);
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(apps));
      return;
    }

    const requestUrl = new URL(req.url || '/', origin);
    const id = requestUrl.pathname.split('/').pop() || '';
    const result = await getServerSideProps({
      req: Object.assign(req, { cookies: {} }),
      res,
      query: { id },
      params: { id },
      resolvedUrl: requestUrl.pathname,
    });
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(result));
  } catch (error) {
    res.writeHead(500, { 'content-type': 'text/plain' });
    res.end(String(error));
  }
});

afterAll(async() => {
  await new Promise<void>((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
  vi.unstubAllEnvs();
  fetchMock.enableMocks();
});

fetchMock.disableMocks();
server.listen(0, '127.0.0.1');
await once(server, 'listening');
const address = server.address();
if (!address || typeof address === 'string') {
  throw new Error('Expected a loopback TCP address');
}
const origin = `http://127.0.0.1:${ address.port }`;

vi.stubEnv('NEXT_PUBLIC_APP_PROTOCOL', 'http');
vi.stubEnv('NEXT_PUBLIC_APP_HOST', '127.0.0.1');
vi.stubEnv('NEXT_PUBLIC_APP_PORT', String(address.port));
vi.stubEnv('NEXT_PUBLIC_MARKETPLACE_ENABLED', 'true');
vi.stubEnv('NEXT_PUBLIC_MARKETPLACE_CONFIG_URL', JSON.stringify(apps));
vi.stubEnv('NEXT_PUBLIC_MARKETPLACE_SUBMIT_FORM', 'https://catalogue.example/submit');
vi.stubEnv('NEXT_PUBLIC_NETWORK_RPC_URL', 'https://rpc.example');
vi.stubEnv('NEXT_PUBLIC_OG_ENHANCED_DATA_ENABLED', 'true');

const appConfig = (await import('configs/app')).default;
const { getServerSideProps } = await import('../../../pages/apps/[id]');
const generateMetadata = (await import('lib/metadata/generate')).default;

test('marketplacePreviewLoadsGeneratedCatalogueFromTrustedOrigin', async function marketplacePreviewLoadsGeneratedCatalogueFromTrustedOrigin() {
  expect(appConfig.features.marketplace).toMatchObject({ isEnabled: true, configUrl: assetPath });
  expect(appConfig.app.origin).toBe(origin);
  const initialRequests = catalogueRequests.length;

  for (const app of apps) {
    const response = await fetch(`${ origin }/explorer/apps/${ app.id }`, {
      headers: {
        'user-agent': 'Twitterbot/1.0',
        host: 'untrusted.example',
        'x-forwarded-host': 'untrusted.example',
        'x-forwarded-proto': 'https',
      },
    });
    expect(response.status).toBe(200);
    const result = await response.json() as GetServerSidePropsResult<Props<'/apps/[id]'>>;
    expect(result).toHaveProperty('props');
    if (!('props' in result)) {
      throw new Error('Expected marketplace page props');
    }
    const props = await result.props;
    expect(props.apiData).toEqual({ app_name: app.title });
    const metadata = generateMetadata({ pathname: '/apps/[id]', query: { id: app.id } }, props.apiData);
    expect(metadata.opengraph.title).toContain(app.title);
  }

  expect(catalogueRequests.slice(initialRequests)).toEqual(apps.map(() => new URL(origin).host));
});

test('marketplacePreviewKeepsUnknownAppMetadataEmpty', async function marketplacePreviewKeepsUnknownAppMetadataEmpty() {
  const initialRequests = catalogueRequests.length;
  const response = await fetch(`${ origin }/explorer/apps/unknown-app`, { headers: { 'user-agent': 'facebookexternalhit/1.1' } });
  expect(response.status).toBe(200);
  const result = await response.json() as GetServerSidePropsResult<Props<'/apps/[id]'>>;
  expect(result).toHaveProperty('props');
  if (!('props' in result)) {
    throw new Error('Expected marketplace page props');
  }
  expect((await result.props).apiData).toBeNull();
  expect(catalogueRequests).toHaveLength(initialRequests + 1);
});

test('marketplacePreviewLeavesOtherBotsWithoutCatalogueRequests', async function marketplacePreviewLeavesOtherBotsWithoutCatalogueRequests() {
  const initialRequests = catalogueRequests.length;
  const response = await fetch(`${ origin }/explorer/apps/${ apps[0].id }`, { headers: { 'user-agent': 'Googlebot/2.1' } });
  expect(response.status).toBe(200);
  const result = await response.json() as GetServerSidePropsResult<Props<'/apps/[id]'>>;
  expect(result).toHaveProperty('props');
  if (!('props' in result)) {
    throw new Error('Expected marketplace page props');
  }
  expect((await result.props).apiData).toBeNull();
  expect(catalogueRequests).toHaveLength(initialRequests);
});
