import type { GetServerSidePropsContext } from 'next';
import type { Server } from 'node:http';
import { createServer, IncomingMessage, ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { Socket } from 'node:net';

import type generateMetadataFunction from 'lib/metadata/generate';
import { averageGasPrice } from 'mocks/stats/line';
import type { getServerSideProps as statsServerSideProps } from 'pages/stats/[id]';
import { afterAll, beforeAll, expect, test, vi } from 'vitest';

import type fetchApiFunction from './fetchApi';

let server: Server;
let status = 200;
let body = JSON.stringify(averageGasPrice);
const requests: Array<URL> = [];
const mockedFetch = globalThis.fetch;
let fetchApi: typeof fetchApiFunction;
let getServerSideProps: typeof statsServerSideProps;
let generateMetadata: typeof generateMetadataFunction;

beforeAll(async() => {
  fetchMock.disableMocks();
  server = createServer((request, response) => {
    requests.push(new URL(request.url ?? '/', 'http://localhost'));
    response.writeHead(status, { 'content-type': 'application/json' });
    response.end(body);
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const origin = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;

  vi.stubEnv('NEXT_PUBLIC_STATS_API_HOST', origin);
  vi.stubEnv('NEXT_PUBLIC_STATS_API_BASE_PATH', '');
  vi.stubEnv('NEXT_PUBLIC_SEO_ENHANCED_DATA_ENABLED', 'true');
  vi.resetModules();
  ({ 'default': fetchApi } = await import('./fetchApi'));
  ({ getServerSideProps } = await import('pages/stats/[id]'));
  ({ 'default': generateMetadata } = await import('lib/metadata/generate'));
});

afterAll(async() => {
  try {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
  } finally {
    fetchMock.enableMocks();
    vi.unstubAllEnvs();
    vi.resetModules();
    expect(globalThis.fetch).toBe(mockedFetch);
  }
});

async function httpErrorsAsPayloads() {
  expect(vi.isMockFunction(globalThis.fetch)).toBe(false);
  const chartInfo = averageGasPrice.info;
  if (!chartInfo) {
    throw new Error('Stats fixture must contain chart metadata');
  }
  const req = Object.assign(new IncomingMessage(new Socket()), { cookies: {} });
  req.headers = { 'x-app-profile': 'private', 'user-agent': 'Twitterbot/1.0' };
  const ctx: GetServerSidePropsContext = {
    req,
    res: new ServerResponse(req),
    query: { id: 'averageGasPrice' },
    resolvedUrl: '/stats/averageGasPrice',
  };
  const route = { pathname: '/stats/[id]', query: ctx.query } as const;
  const params = { resource: 'stats:line', pathParams: { id: 'averageGasPrice' } } as const;

  for (const code of [ 400, 404, 500, 503 ]) {
    status = code;
    body = JSON.stringify({ message: code < 500 ? 'Not found' : 'Internal server error' });

    expect(await fetchApi(params)).toBeUndefined();
    const result = await getServerSideProps(ctx);
    expect(result).toHaveProperty('props');
    if (!('props' in result)) {
      throw new Error('Stats metadata caller did not return props');
    }
    const props = await result.props;
    expect(props.apiData).toBeNull();
    expect(generateMetadata(route, props.apiData)).toEqual(generateMetadata(route, null));
  }

  for (const code of [ 200, 201 ]) {
    status = code;
    body = JSON.stringify(averageGasPrice);

    expect(await fetchApi(params)).toEqual(averageGasPrice);
    const result = await getServerSideProps(ctx);
    expect(result).toHaveProperty('props');
    if (!('props' in result)) {
      throw new Error('Stats metadata caller did not return props');
    }
    const props = await result.props;
    expect(props.apiData).toEqual(chartInfo);
    expect(generateMetadata(route, props.apiData).title).toContain(chartInfo.title);
    expect(generateMetadata(route, props.apiData).title).not.toBe(generateMetadata(route, null).title);
  }

  expect(requests).toHaveLength(12);
  expect(requests.every((url) => url.pathname === '/api/v1/lines/averageGasPrice')).toBe(true);
  const metadataRequests = requests.filter((url) => url.searchParams.has('from'));
  expect(metadataRequests).toHaveLength(6);
  for (const url of metadataRequests) {
    expect(url.searchParams.get('from')).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(url.searchParams.get('to')).toBe(url.searchParams.get('from'));
  }
}

test(httpErrorsAsPayloads, httpErrorsAsPayloads);
