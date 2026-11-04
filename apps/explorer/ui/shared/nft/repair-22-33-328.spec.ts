// @vitest-environment jsdom

import { ChakraProvider, defaultSystem } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, renderHook, waitFor } from '@testing-library/react';
import { MemoryBlockstore } from 'blockstore-core';
import { CID } from 'multiformats/cid';
import * as raw from 'multiformats/codecs/raw';
import { sha256 } from 'multiformats/hashes/sha2';
import { Blob as NodeBlob } from 'node:buffer';
import { readFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { URL as NodeURL } from 'node:url';
import React from 'react';
import { defaultFallbackInView } from 'react-intersection-observer';

import type { TokenInstance } from 'types/api/token';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import { base as tokenFixture } from 'mocks/tokens/tokenInstance';
import { afterAll, afterEach, beforeAll, expect, test, vi } from 'vitest';

import NftHtml from './NftHtml';
import NftImage from './NftImage';
import NftMedia from './NftMedia';
import NftMediaFullscreenModal from './NftMediaFullscreenModal';
import NftVideo, { loadNftVideoPoster } from './NftVideo';
import useLoadImageViaIpfs from './useLoadImageViaIpfs';
import useNftMediaInfo from './useNftMediaInfo';

const ipfsBoundary = vi.hoisted(() => ({
  read: null as ((url: string) => Promise<Uint8Array>) | null,
}));

// Keep the optional IPFS dependency at its import boundary; the NFT hook still
// fetches the recorded blocks, creates real blob URLs and owns their lifecycle.
vi.mock('@helia/verified-fetch', () => ({
  verifiedFetch: async(url: string, init?: RequestInit) => {
    if (!ipfsBoundary.read) {
      throw new Error('No gallery transcript is open');
    }
    const bytes = await ipfsBoundary.read(url);
    const range = new Headers(init?.headers).get('range');
    if (range) {
      const match = /^bytes=(\d+)-(\d*)$/.exec(range);
      const start = match ? Number(match[1]) : NaN;
      const end = match?.[2] ? Number(match[2]) : bytes.length - 1;
      if (!Number.isSafeInteger(start) || start < 0 || start >= bytes.length || end < start || end >= bytes.length) {
        return new Response(null, { status: 416, headers: { 'content-range': `bytes */${ bytes.length }` } });
      }
      return new Response(Uint8Array.from(bytes.slice(start, end + 1)), {
        status: 206,
        headers: { 'content-type': 'image/jpeg', 'content-range': `bytes ${ start }-${ end }/${ bytes.length }` },
      });
    }
    return new Response(Uint8Array.from(bytes), { status: 200, headers: { 'content-type': 'image/jpeg' } });
  },
}));

const h = React.createElement;
const nativeBlobDescriptor = Object.getOwnPropertyDescriptor(globalThis, 'Blob')!;
const nativeCreateObjectURL = URL.createObjectURL;
const nativeRevokeObjectURL = URL.revokeObjectURL;
let nativeFetch: typeof fetch;
let origin: string;
const requests: Array<string> = [];
const createdUrls: Array<string> = [];
const clients: Array<QueryClient> = [];
const heldReads: Array<() => void> = [];
let shell: string;
let pwSource: string;
let jpeg: Uint8Array;

function probePath(id: string, hash: string, field = 'animation_url') {
  return `/explorer/node-api/tokens/${ hash }/instances/${ id }/media-type?field=${ field }`;
}

function interceptedPath(name: string, data: TokenInstance) {
  const template = pwSource.match(new RegExp(`const ${ name } = \x60([^\x60]+)\x60`))?.[1];
  expect(template).toBeDefined();
  return template!.replace('${ TOKEN_HASH }', data.token.address_hash).replace('${ TOKEN_ID }', data.id);
}

const mediaTranscripts = new Map<string, { type?: 'html' | 'video' | 'image' }>();
const server = createServer((req, res) => {
  const path = req.url || '/';
  requests.push(path);
  if (path === '/explorer/nft-html-embed.html') {
    res.writeHead(200, { 'content-type': 'text/html' });
    res.end(shell);
  } else if (mediaTranscripts.has(path)) {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(mediaTranscripts.get(path)));
  } else if (path === '/poster.jpg') {
    res.writeHead(200, { 'content-type': 'image/jpeg' });
    res.end(jpeg);
  } else {
    res.writeHead(404);
    res.end();
  }
});

beforeAll(async() => {
  globalThis.fetchMock.disableMocks();
  nativeFetch = globalThis.fetch;
  [ shell, pwSource, jpeg ] = await Promise.all([
    readFile(`${ __dirname }/../../../public/nft-html-embed.html`, 'utf8'),
    readFile(`${ __dirname }/NftMedia.pw.tsx`, 'utf8'),
    readFile(`${ __dirname }/../../../playwright/mocks/image_md.jpg`).then((bytes) => Uint8Array.from(bytes)),
  ]);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  origin = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;
  // Resolve browser-relative requests into the real loopback fixture server.
  globalThis.fetch = (input, init) => nativeFetch(
    typeof input === 'string' ? new NodeURL(input, origin).href : input,
    init,
  );
  // Node's Response.blob() reads the global Blob constructor, which jsdom replaces.
  Object.defineProperty(globalThis, 'Blob', { configurable: true, writable: true, value: NodeBlob });
  // Use Node's real blob store: every URL is fetchable until actually revoked.
  URL.createObjectURL = (blob) => {
    if (!(blob instanceof NodeBlob)) {
      throw new TypeError('Expected a real response blob');
    }
    const url = NodeURL.createObjectURL(blob);
    createdUrls.push(url);
    return url;
  };
  URL.revokeObjectURL = NodeURL.revokeObjectURL;
  defaultFallbackInView(true);
});

afterEach(async() => {
  cleanup();
  heldReads.splice(0).forEach((release) => release());
  clients.splice(0).forEach((client) => client.clear());
  ipfsBoundary.read = null;
  createdUrls.splice(0).forEach((url) => NodeURL.revokeObjectURL(url));
  requests.length = 0;
  mediaTranscripts.clear();
});

afterAll(async() => {
  globalThis.fetch = nativeFetch;
  Object.defineProperty(globalThis, 'Blob', nativeBlobDescriptor);
  URL.createObjectURL = nativeCreateObjectURL;
  URL.revokeObjectURL = nativeRevokeObjectURL;
  defaultFallbackInView(undefined);
  await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

class GalleryBlockstore extends MemoryBlockstore {
  readonly gates = new Map<string, Promise<void>>();
  readonly started = new Set<string>();
  readonly completed = new Set<string>();

  hold(cid: CID) {
    let release!: () => void;
    this.gates.set(cid.toString(), new Promise<void>((resolve) => {
      release = resolve;
    }));
    heldReads.push(release);
    return release;
  }

  override async get(cid: CID) {
    const key = cid.toString();
    this.started.add(key);
    await this.gates.get(key);
    const bytes = await super.get(cid);
    this.completed.add(key);
    return bytes;
  }
}

async function gallery() {
  const store = new GalleryBlockstore();
  ipfsBoundary.read = (url) => store.get(CID.parse(new NodeURL(url).hostname));
  const { verifiedFetch: client } = await import('@helia/verified-fetch');
  const probeCid = CID.createV1(raw.code, await sha256.digest(jpeg));
  await store.put(probeCid, jpeg);
  const probe = await client(`ipfs://${ probeCid }?format=raw`);
  expect(probe.status, await probe.clone().text()).toBe(200);
  expect(new Uint8Array(await probe.arrayBuffer())).toEqual(jpeg);
  const invalidRange = await client(`ipfs://${ probeCid }?format=raw`, { headers: { range: `bytes=${ jpeg.length }-` } });
  expect(invalidRange.status).toBe(416);
  expect(invalidRange.headers.get('content-range')).toBe(`bytes */${ jpeg.length }`);
  const cids: Array<CID> = [];
  for (const suffix of [ 1, 2, 3, 4, 5 ]) {
    const bytes = new Uint8Array([ ...jpeg, suffix ]);
    const cid = CID.createV1(raw.code, await sha256.digest(bytes));
    await store.put(cid, bytes);
    cids.push(cid);
  }
  return { store, cids, sources: cids.map((cid) => `ipfs://${ cid }?format=raw`) };
}

function fixture() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
  clients.push(queryClient);
  const state = { chainId: '1' };
  function Wrapper({ children }: { children: React.ReactNode }) {
    return h(ChakraProvider, { value: defaultSystem, children:
      h(QueryClientProvider, { client: queryClient },
        h(MultichainContext.Provider, { value: { chain: { id: state.chainId, name: 'Gallery', slug: 'gallery', app_config: config } } },
          children,
        ),
      ),
    });
  }
  return { wrapper: Wrapper, queryClient, state };
}

function token(overrides: Partial<TokenInstance> = {}): TokenInstance {
  return { ...tokenFixture, id: '1', animation_url: null, image_url: null, metadata: null, thumbnails: null, ...overrides };
}

async function nft_embed_basepath() {
  let loaded = 0;
  const view = render(h(NftHtml, { src: 'data:text/html,<p>NFT</p>', transport: 'http', onLoad: () => {
    loaded += 1;
  } }), fixture());
  const iframe = view.container.querySelector('iframe')!;
  expect(new NodeURL(iframe.src).pathname).toBe('/explorer/nft-html-embed.html');
  expect(new NodeURL(iframe.src).origin).toBe(window.location.origin);
  expect(iframe.getAttribute('sandbox')).toBe('allow-scripts allow-same-origin');
  const response = await fetch(new NodeURL(iframe.src).pathname);
  expect(response.status).toBe(200);
  const frame = iframe.contentWindow!;
  const document = iframe.contentDocument!;
  document.open();
  document.write(await response.text());
  document.close();
  const nested = document.querySelector('iframe')!;
  expect(nested.getAttribute('sandbox')).toBe('allow-scripts');
  expect(nested.getAttribute('sandbox')).not.toContain('allow-same-origin');

  const incoming: Array<unknown> = [];
  const receiveRequest = (event: MessageEvent) => incoming.push(event.data);
  const outgoing: Array<unknown> = [];
  const receive = (event: MessageEvent) => outgoing.push(event.data);
  frame.addEventListener('message', receiveRequest);
  window.addEventListener('message', receive);
  try {
    fireEvent.load(iframe);
    await waitFor(() => expect(incoming).toContainEqual({ type: 'nft-html-embed', src: 'data:text/html,<p>NFT</p>' }));
    const request = incoming.find((data) => typeof data === 'object' && data !== null && 'type' in data);
    // Replay the production request with the browser metadata omitted by jsdom.
    frame.dispatchEvent(new MessageEvent('message', {
      origin: window.location.origin,
      source: window,
      data: request,
    }));
    expect(nested.src).toBe('data:text/html,<p>NFT</p>');
    fireEvent.load(nested);
    await waitFor(() => expect(outgoing).toContainEqual({ type: 'nft-html-embed-loaded', src: nested.src }));
    const payload = outgoing.find((data) => typeof data === 'object' && data !== null && 'type' in data);
    // jsdom postMessage omits source and origin; dispatch browser metadata explicitly.
    for (const [ source, origin, data ] of [
      [ window, window.location.origin, payload ],
      [ frame, 'https://untrusted.invalid', payload ],
      [ frame, window.location.origin, { type: 'nft-html-embed-loaded', src: 'wrong-source' } ],
    ] as const) {
      act(() => window.dispatchEvent(new MessageEvent('message', { source, origin, data })));
    }
    expect(loaded).toBe(0);
    act(() => window.dispatchEvent(new MessageEvent('message', { source: frame, origin: window.location.origin, data: payload })));
    expect(loaded).toBe(1);
  } finally {
    frame.removeEventListener('message', receiveRequest);
    window.removeEventListener('message', receive);
  }
}

test(nft_embed_basepath, nft_embed_basepath);

async function nft_image_async_race() {
  const { store, cids, sources } = await gallery();
  const releaseFirst = store.hold(cids[0]);
  const releaseSecond = store.hold(cids[1]);
  let loaded = 0;
  let errors = 0;
  const onLoad = () => {
    loaded += 1;
  };
  const onError = () => {
    errors += 1;
  };
  const view = render(h(NftImage, { src: sources[0], transport: 'ipfs', onLoad, onError }), fixture());
  await waitFor(() => expect(store.started.has(cids[0].toString())).toBe(true));
  view.rerender(h(NftImage, { src: sources[1], transport: 'ipfs', onLoad, onError }));
  await waitFor(() => expect(store.started.has(cids[1].toString())).toBe(true));
  await act(async() => {
    releaseSecond();
  });
  await waitFor(() => expect(loaded).toBe(1));
  const currentSrc = view.container.querySelector('img')!.src;
  const body = await (await nativeFetch(currentSrc)).arrayBuffer();
  expect(new Uint8Array(body).at(-1)).toBe(2);
  await act(async() => {
    releaseFirst();
  });
  await waitFor(() => expect(store.completed.has(cids[0].toString())).toBe(true));
  expect(view.container.querySelector('img')!.src).toBe(currentSrc);
  expect(loaded).toBe(1);
  expect(errors).toBe(0);
  expect(createdUrls).toHaveLength(1);

  const releaseThird = store.hold(cids[2]);
  view.rerender(h(NftImage, { src: sources[2], transport: 'ipfs', onLoad, onError }));
  expect(getComputedStyle(view.container.querySelector('img')!).opacity).toBe('0');
  await expect(nativeFetch(currentSrc)).rejects.toThrow();
  await act(async() => {
    releaseThird();
  });
  await waitFor(() => expect(loaded).toBe(2));
  expect(new Uint8Array(await (await nativeFetch(view.container.querySelector('img')!.src)).arrayBuffer()).at(-1)).toBe(3);
}

test(nft_image_async_race, nft_image_async_race);

async function nft_test_api_path() {
  const data = token({ animation_url: `${ origin }/animation` });
  const path = interceptedPath('MEDIA_TYPE_API_URL', data);
  mediaTranscripts.set(path, { type: 'html' });
  const view = render(h(NftMedia, { data }), fixture());
  await waitFor(() => expect(requests).toContain(path));
  await waitFor(() => expect(view.container.querySelector('iframe')).not.toBeNull());
  expect(path).toBe(probePath(data.id, data.token.address_hash));
  const responseAssertion = pwSource.indexOf('await mediaTypeResponse');
  expect(responseAssertion).toBeGreaterThan(0);
  expect(responseAssertion).toBeLessThan(pwSource.indexOf('await expect(component).toHaveScreenshot()', responseAssertion));
  view.unmount();

  const fallbackData = token({ animation_url: `${ origin }/unsupported`, image_url: `${ origin }/poster.jpg` });
  const fallbackPath = interceptedPath('ANIMATION_MEDIA_TYPE_API_URL', fallbackData);
  mediaTranscripts.set(fallbackPath, {});
  const fallback = render(h(NftMedia, { data: fallbackData }), fixture());
  await waitFor(() => expect(requests.filter((request) => request === fallbackPath)).toHaveLength(2));
  await waitFor(() => expect(fallback.container.querySelector('img')?.src).toBe(fallbackData.image_url));
  expect(fallbackPath).toBe(probePath(fallbackData.id, fallbackData.token.address_hash));
  const pagePreview = pwSource.slice(pwSource.indexOf('test.describe(\'page\''));
  expect(pagePreview.indexOf('await mediaTypeResponse')).toBeGreaterThan(0);
  expect(pagePreview.indexOf('await mediaTypeResponse')).toBeLessThan(pagePreview.indexOf('await expect(component).toHaveScreenshot()'));
}

test(nft_test_api_path, nft_test_api_path);

async function nft_media_replacement() {
  for (const change of [ 'contract', 'token', 'chain' ]) {
    const context = fixture();
    const failed = token();
    const view = render(h(NftMedia, { data: failed, fallback: h('span', null, 'Unavailable NFT') }), context);
    await waitFor(() => expect(view.getByText('Unavailable NFT')).toBeDefined());
    const valid = token({
      id: change === 'token' ? '2' : failed.id,
      token: { ...failed.token, address_hash: change === 'contract' ? '0x2222222222222222222222222222222222222222' : failed.token.address_hash },
      animation_url: `${ origin }/animated.mp4`,
    });
    if (change === 'chain') {
      context.state.chainId = '2';
    }
    view.rerender(h(NftMedia, { data: valid, fallback: h('span', null, 'Unavailable NFT') }));
    await waitFor(() => expect(view.container.querySelector('video')?.src).toBe(valid.animation_url));
    expect(view.queryByText('Unavailable NFT')).toBeNull();
    expect(view.container.querySelector('[data-loading]')).not.toBeNull();
    fireEvent.canPlayThrough(view.container.querySelector('video')!);
    await waitFor(() => expect(view.container.querySelector('[data-loading]')).toBeNull());
    view.unmount();
  }
}

test(nft_media_replacement, nft_media_replacement);

async function nft_fullscreen_failure() {
  const data = token({
    image_url: `${ origin }/original.jpg`,
    thumbnails: { original: `${ origin }/thumbnail.jpg` },
  });
  const props = { open: true, onOpenChange: () => undefined, data, field: 'image_url' as const };
  const view = render(h(NftMediaFullscreenModal, props), fixture());
  expect(view.getByRole('status').textContent).toContain('Loading NFT media');
  await waitFor(() => expect(document.querySelector('img')?.src).toBe(data.thumbnails!.original));
  fireEvent.error(document.querySelector('img')!);
  await waitFor(() => expect(document.querySelector('img')?.src).toBe(data.image_url));
  fireEvent.error(document.querySelector('img')!);
  await waitFor(() => expect(view.getByRole('alert').textContent).toContain('NFT media is unavailable'));
  expect(view.queryByRole('status')).toBeNull();
  expect(document.querySelector('img')).toBeNull();
  expect(view.getByRole('button', { name: /close/i })).toBeDefined();
  view.rerender(h(NftMediaFullscreenModal, { ...props, open: false }));
  view.rerender(h(NftMediaFullscreenModal, props));
  await waitFor(() => expect(document.querySelector('img')?.src).toBe(data.thumbnails!.original));
  expect(view.queryByRole('alert')).toBeNull();
  view.unmount();

  const empty = render(h(NftMediaFullscreenModal, { ...props, data: token() }), fixture());
  await waitFor(() => expect(empty.getByRole('alert').textContent).toContain('NFT media is unavailable'));
  expect(empty.queryByRole('status')).toBeNull();
}

test(nft_fullscreen_failure, nft_fullscreen_failure);

async function nft_http_poster() {
  const data = token({ image_url: `${ origin }/poster.jpg` });
  let loaded = 0;
  let failed = 0;
  const onLoad = () => {
    loaded += 1;
  };
  const onError = () => {
    failed += 1;
  };
  const view = render(h(NftVideo, {
    src: `${ origin }/video.mp4`, transport: 'http', instance: data, autoPlay: false, onLoad, onError,
  }), fixture());
  await waitFor(() => expect(view.container.querySelector('video')?.poster).toBe(data.image_url));
  const video = view.container.querySelector('video')!;
  expect(video.autoplay).toBe(false);
  const response = await nativeFetch(video.poster);
  expect(response.headers.get('content-type')).toBe('image/jpeg');
  expect(new Uint8Array(await response.arrayBuffer())).toEqual(jpeg);
  const poster = loadNftVideoPoster(video, data.image_url!, onLoad, onError);
  expect(poster.onload).toBe(onLoad);
  expect(poster.onerror).toBe(onError);
  fireEvent.load(poster);
  expect(loaded).toBe(1);
  expect(failed).toBe(0);
  fireEvent.error(poster);
  expect(failed).toBe(1);
}

test(nft_http_poster, nft_http_poster);

async function ipfs_blob_lifecycle() {
  const { sources, store, cids } = await gallery();
  const context = fixture();
  const view = render(h(NftImage, { src: sources[0], transport: 'ipfs' }), context);
  for (const source of sources.slice(0, 3)) {
    const previous = view.container.querySelector('img')?.src;
    view.rerender(h(NftImage, { src: source, transport: 'ipfs' }));
    await waitFor(() => expect(view.container.querySelector('img')?.src).toMatch(/^blob:/));
    const current = view.container.querySelector('img')!.src;
    expect((await nativeFetch(current)).status).toBe(200);
    if (previous?.startsWith('blob:') && previous !== current) {
      await expect(nativeFetch(previous)).rejects.toThrow();
    }
  }
  const current = view.container.querySelector('img')!.src;
  view.unmount();
  await expect(nativeFetch(current)).rejects.toThrow();

  const posterData = token({ image_url: `${ origin }/poster.jpg`, metadata: { image: sources[0] } });
  const video = render(h(NftVideo, {
    src: `${ origin }/video.mp4`, transport: 'http', instance: posterData, autoPlay: false,
  }), fixture());
  await waitFor(() => expect(video.container.querySelector('video')?.poster).toMatch(/^blob:/));
  const firstPoster = video.container.querySelector('video')!.poster;
  video.rerender(h(NftVideo, {
    src: `${ origin }/video.mp4`, transport: 'http', instance: { ...posterData, metadata: { image: sources[1] } }, autoPlay: false,
  }));
  await waitFor(() => {
    expect(video.container.querySelector('video')?.poster).toMatch(/^blob:/);
    expect(video.container.querySelector('video')?.poster).not.toBe(firstPoster);
  });
  const nextPoster = video.container.querySelector('video')!.poster;
  expect(nextPoster).not.toBe(firstPoster);
  await expect(nativeFetch(firstPoster)).rejects.toThrow();
  expect((await nativeFetch(nextPoster)).status).toBe(200);
  video.unmount();
  await expect(nativeFetch(nextPoster)).rejects.toThrow();

  const release = store.hold(cids[3]);
  const hook = renderHook(() => useLoadImageViaIpfs(), fixture());
  const pending = hook.result.current(sources[3]);
  const abandoned = expect(pending).rejects.toMatchObject({ name: 'AbortError' });
  await waitFor(() => expect(store.started.has(cids[3].toString())).toBe(true));
  const createdBefore = createdUrls.length;
  hook.unmount();
  release();
  await abandoned;
  expect(createdUrls).toHaveLength(createdBefore);

  const releasePoster = store.hold(cids[4]);
  const abandonedVideo = render(h(NftVideo, {
    src: `${ origin }/video.mp4`, transport: 'http', instance: { ...posterData, metadata: { image: sources[4] } }, autoPlay: false,
  }), fixture());
  await waitFor(() => expect(store.started.has(cids[4].toString())).toBe(true));
  const createdBeforePoster = createdUrls.length;
  abandonedVideo.unmount();
  await act(async() => {
    releasePoster();
  });
  await waitFor(() => expect(store.completed.has(cids[4].toString())).toBe(true));
  expect(createdUrls).toHaveLength(createdBeforePoster);
  for (const url of createdUrls) {
    await expect(nativeFetch(url)).rejects.toThrow();
  }
}

test(ipfs_blob_lifecycle, ipfs_blob_lifecycle);

async function nft_cache_identity() {
  const context = fixture();
  const shared = `${ origin }/original.jpg`;
  const first = token({ image_url: shared, thumbnails: { original: `${ origin }/first.jpg` } });
  const second = token({
    image_url: shared,
    token: { ...first.token, address_hash: '0x2222222222222222222222222222222222222222' },
    thumbnails: { original: `${ origin }/second.jpg` },
  });
  const hooks = renderHook(() => [
    useNftMediaInfo({ data: first, size: 'original', field: 'image_url', isEnabled: true }),
    useNftMediaInfo({ data: second, size: 'original', field: 'image_url', isEnabled: true }),
  ], context);
  await waitFor(() => expect(hooks.result.current.every((query) => query.isSuccess)).toBe(true));
  expect(hooks.result.current[0].data?.[0].src).toBe(first.thumbnails!.original);
  expect(hooks.result.current[1].data?.[0].src).toBe(second.thumbnails!.original);
  hooks.unmount();

  const props = { data: first, field: 'image_url' as 'image_url' | 'animation_url' };
  const revisions = renderHook(({ data, field }) => useNftMediaInfo({ data, field, size: 'original', isEnabled: true }), {
    ...context, initialProps: props,
  });
  const updated = { ...first, thumbnails: { original: `${ origin }/updated.jpg` }, metadata: { image: 'ipfs://updated' } };
  revisions.rerender({ data: updated, field: 'image_url' });
  await waitFor(() => expect(revisions.result.current.data?.map((media) => media.src)).toEqual([
    updated.thumbnails.original, 'ipfs://updated', shared,
  ]));
  const sharedFields = { ...updated, animation_url: shared, metadata: { ...updated.metadata, animation_url: 'ipfs://animation' } };
  revisions.rerender({ data: sharedFields, field: 'animation_url' });
  await waitFor(() => expect(revisions.result.current.data?.map((media) => media.src)).toEqual([
    updated.thumbnails.original, 'ipfs://animation', shared,
  ]));
  const beforeChain = context.queryClient.getQueryCache().getAll().length;
  context.state.chainId = '2';
  revisions.rerender({ data: sharedFields, field: 'animation_url' });
  await waitFor(() => expect(revisions.result.current.isSuccess).toBe(true));
  expect(context.queryClient.getQueryCache().getAll()).toHaveLength(beforeChain + 1);
  const changedMime = { ...sharedFields, media_url: shared, media_type: 'video/mp4' };
  revisions.rerender({ data: changedMime, field: 'animation_url' });
  await waitFor(() => expect(revisions.result.current.data).toEqual([
    { src: shared, mediaType: 'video', transport: 'http' },
  ]));
}

test(nft_cache_identity, nft_cache_identity);
