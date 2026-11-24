import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import nodeFetch from 'node-fetch';
import type { Server } from 'node:http';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';

import { afterAll, beforeAll, expect, test } from 'vitest';

import type * as MediaPolicy from '../../../../../../../pages/api/tokens/[hash]/instances/[id]/media-type';

let mediaPolicy: typeof MediaPolicy;

let metadataServer: Server;
let privateServer: Server;
let routeServer: Server;
let routeOrigin: string;
let privateOrigin: string;
let mediaUrl: string;
let redirectLocation = '';
let privateRequests = 0;
const metadataPaths: Array<string> = [];
const envNames = [ 'NEXT_PUBLIC_API_HOST', 'NEXT_PUBLIC_API_PORT', 'NEXT_PUBLIC_API_PROTOCOL', 'NEXT_PUBLIC_API_BASE_PATH' ] as const;
const previousEnv = envNames.map((name) => process.env[name]);

async function listen(server: Server) {
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  return `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;
}

beforeAll(async() => {
  privateServer = createServer((_request, response) => {
    privateRequests++;
    response.writeHead(302, { location: redirectLocation, 'content-type': 'image/png' });
    response.end();
  });
  privateOrigin = await listen(privateServer);
  metadataServer = createServer((request, response) => {
    metadataPaths.push(request.url ?? '');
    response.writeHead(200, { 'content-type': 'application/json' });
    response.end(JSON.stringify({ image_url: mediaUrl, animation_url: mediaUrl }));
  });
  const metadataOrigin = new URL(await listen(metadataServer));
  process.env.NEXT_PUBLIC_API_HOST = metadataOrigin.hostname;
  process.env.NEXT_PUBLIC_API_PORT = metadataOrigin.port;
  process.env.NEXT_PUBLIC_API_PROTOCOL = 'http';
  process.env.NEXT_PUBLIC_API_BASE_PATH = '';

  mediaPolicy = await import('../../../../../../../pages/api/tokens/[hash]/instances/[id]/media-type');
  const { 'default': handler } = mediaPolicy;
  routeServer = createServer((request, response) => {
    const url = new URL(request.url ?? '/', 'http://localhost');
    void apiResolver(request, response, {
      hash: '0x0000000000000000000000000000000000000001', id: '7', field: url.searchParams.get('field'),
    }, { 'default': handler }, {
      previewModeId: '', previewModeEncryptionKey: '', previewModeSigningKey: '', dev: false,
    }, false);
  });
  routeOrigin = await listen(routeServer);
});

afterAll(async() => {
  envNames.forEach((name, index) => {
    const value = previousEnv[index];
    if (value === undefined) {
      delete process.env[name];
    } else {
      process.env[name] = value;
    }
  });
  await Promise.all([ routeServer, metadataServer, privateServer ].filter(Boolean).map((server) => new Promise<void>((resolve, reject) => {
    server.closeAllConnections();
    server.close((error) => error ? reject(error) : resolve());
  })));
});

test('nft-media-outbound-trust rejects internal NFT metadata through the real API route', async() => {
  const before = privateRequests;
  const metadataBefore = metadataPaths.length;
  const port = new URL(privateOrigin).port;
  for (const field of [ 'image_url', 'animation_url' ]) {
    for (const host of [ 'localhost', '127.0.0.1', '2130706433', '0x7f000001', '[::1]', '[::ffff:127.0.0.1]' ]) {
      mediaUrl = `http://${ host }:${ port }/private-media`;
      const response = await nodeFetch(`${ routeOrigin }/?field=${ field }`);
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({});
      expect(privateRequests).toBe(before);
    }
  }
  expect(metadataPaths.slice(metadataBefore)).toEqual(Array(12).fill('/api/v2/tokens/0x0000000000000000000000000000000000000001/instances/7'));
});

test('nft-media-outbound-trust permits only public unicast addresses', () => {
  for (const address of [ '8.8.8.8', '1.1.1.1', '2606:4700:4700::1111', '2001:4860:4860::8888' ]) {
    expect(mediaPolicy.isPublicMediaAddress(address)).toBe(true);
  }
  for (const address of [
    '0.0.0.0', '10.2.3.4', '100.64.0.1', '127.0.0.1', '169.254.169.254', '172.16.0.1', '192.168.1.1',
    '192.0.0.1', '192.0.2.1', '192.88.99.1', '198.18.0.1', '198.51.100.1', '203.0.113.1', '224.0.0.1', '255.255.255.255',
    '::', '::1', '::ffff:127.0.0.1', '::ffff:8.8.8.8', 'fc00::1', 'fd00::1', 'fe80::1', 'ff02::1',
    '64:ff9b::a00:1', '2001::1', '2001:db8::1', '2002:7f00:1::', '3fff::1', 'invalid',
  ]) {
    expect(mediaPolicy.isPublicMediaAddress(address)).toBe(false);
  }
});

test('nft-media-outbound-trust revalidates real redirect headers and bounds the chain', async() => {
  const signal = new AbortController().signal;
  const source = new URL('https://8.8.8.8/nft/media');
  for (const location of [ `${ privateOrigin }/private`, `http://localhost:${ new URL(privateOrigin).port }/metadata`,
    'http://169.254.169.254/latest/meta-data/', 'http://[::ffff:127.0.0.1]/', 'file:///etc/passwd', 'https://user:pass@8.8.8.8/' ]) {
    redirectLocation = location;
    const response = await nodeFetch(privateOrigin, { method: 'HEAD', redirect: 'manual' });
    expect(response.status).toBe(302);
    expect(response.headers.get('location')).toBe(location);
    await expect(mediaPolicy.resolveMediaRedirect(source, response.headers.get('location'), 0, signal)).rejects.toThrow();
  }
  for (let redirects = 0; redirects < 3; redirects++) {
    redirectLocation = '../image.png';
    const response = await nodeFetch(privateOrigin, { method: 'HEAD', redirect: 'manual' });
    const destination = await mediaPolicy.resolveMediaRedirect(source, response.headers.get('location'), redirects, signal);
    expect(destination.url.href).toBe('https://8.8.8.8/image.png');
    expect(destination.address).toEqual({ address: '8.8.8.8', family: 4 });
  }
  await expect(mediaPolicy.resolveMediaRedirect(source, '/image.png', 3, signal)).rejects.toThrow('redirect limit');
  await expect(mediaPolicy.resolveMediaRedirect(source, null, 0, signal)).rejects.toThrow('missing destination');
});

test('nft-media-outbound-trust rejects unsupported schemes, credentials and oversized URLs before dispatch', async() => {
  const signal = new AbortController().signal;
  for (const input of [ 'file:///etc/passwd', 'ftp://8.8.8.8/image', 'data:image/png;base64,AAAA',
    'https://user:pass@8.8.8.8/image', `https://8.8.8.8/${ 'x'.repeat(8192) }` ]) {
    await expect(mediaPolicy.fetchOutboundMediaHeaders(input, signal)).rejects.toThrow();
  }
  expect(await mediaPolicy.resolveMediaDestination('https://8.8.8.8/image', signal)).toEqual({
    url: new URL('https://8.8.8.8/image'), address: { address: '8.8.8.8', family: 4 },
  });
  const controller = new AbortController();
  controller.abort(new Error('Media deadline reached'));
  await expect(mediaPolicy.fetchOutboundMediaHeaders('https://8.8.8.8/image', controller.signal)).rejects.toThrow('Media deadline reached');
});
