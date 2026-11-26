import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import React, { act } from 'react';
import type { Root } from 'react-dom/client';
import { afterAll, afterEach, beforeAll, expect, test } from 'vitest';
import type { Comment } from '@/widgets/community-interactions/interaction-client';

interface BrowserDom { window: Window & typeof globalThis }
type JsdomModule = { JSDOM: new (html: string, options: { url: string; pretendToBeVisual: boolean }) => BrowserDom };
const { JSDOM } = createRequire(import.meta.url)('jsdom') as JsdomModule;
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>', { url: 'http://localhost:3000', pretendToBeVisual: true });
const browserGlobals = globalThis as unknown as Record<string, unknown>;
for (const key of Object.getOwnPropertyNames(dom.window)) {
  if (key in globalThis || key.startsWith('_')) continue;
  const value = (dom.window as unknown as Record<string, unknown>)[key];
  browserGlobals[key] = typeof value === 'function' && !/^[A-Z]/.test(key) ? value.bind(dom.window) : value;
}
browserGlobals.window = dom.window;
browserGlobals.document = dom.window.document;
browserGlobals.IS_REACT_ACT_ENVIRONMENT = true;

const { createRoot } = await import('react-dom/client');
const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
const { RepliesTab } = await import('./replies-tab');
const { readWalletComments } = await import('@/widgets/social/wallet-comments');
const { shortAddress } = await import('@/utils/format');

interface Fixture {
  origin: string;
  missingOrigin: string;
  unavailableOrigin: string;
  wallet: string;
  emptyWallet: string;
  pool: string;
  otherPool: string;
  items: Comment[];
  nextCursor: string;
}
let fixture: Fixture;
let child: ChildProcess | undefined;
let directory: string;
let output = '';
let exited: Promise<number | null> | undefined;
const gatewayOrigin = process.env.NEXT_PUBLIC_GATEWAY_URL;
const views: { root: Root; container: HTMLDivElement; client: InstanceType<typeof QueryClient> }[] = [];

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), 'wallet-comments-'));
  const fixturePath = join(directory, 'comments.json');
  child = spawn('go', ['test', '-count=1', '-timeout=4m', '-run=^TestWalletCommentsProfileReads$', './internal/interactions/'], {
    cwd: fileURLToPath(new URL('../../../../../../services/launchpad/social/', import.meta.url)),
    env: { ...process.env, GOWORK: 'off', WALLET_COMMENTS_PROFILE_FIXTURE: fixturePath },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout?.on('data', chunk => { output += String(chunk); });
  child.stderr?.on('data', chunk => { output += String(chunk); });
  exited = new Promise((resolve, reject) => { child!.once('exit', resolve); child!.once('error', reject); });
  const deadline = Date.now() + 90_000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw new Error(`Comment service exited: ${output}`);
    try {
      fixture = JSON.parse(await readFile(fixturePath, 'utf8')) as Fixture;
      process.env.NEXT_PUBLIC_GATEWAY_URL = fixture.origin;
      return;
    } catch (error) {
      if (!(error instanceof Error) || !('code' in error) || error.code !== 'ENOENT') throw error;
    }
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  throw new Error(`Comment service did not become ready: ${output}`);
}, 120_000);

afterEach(() => {
  for (const view of views.splice(0)) {
    act(() => { view.root.unmount(); });
    view.client.clear();
    view.container.remove();
  }
  process.env.NEXT_PUBLIC_GATEWAY_URL = fixture.origin;
});

afterAll(async () => {
  try {
    if (directory) await writeFile(join(directory, 'closed'), '');
    if (child && exited) {
      const code = await exited;
      expect(code, output).toBe(0);
    }
  } finally {
    child?.kill();
    if (directory) await rm(directory, { recursive: true, force: true });
    if (gatewayOrigin === undefined) delete process.env.NEXT_PUBLIC_GATEWAY_URL;
    else process.env.NEXT_PUBLIC_GATEWAY_URL = gatewayOrigin;
    dom.window.close();
  }
}, 30_000);

function mount(wallet = fixture.wallet, isOwner = false) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  views.push({ root, container, client });
  act(() => { root.render(<QueryClientProvider client={client}><RepliesTab wallet={wallet} isOwner={isOwner} /></QueryClientProvider>); });
  return container;
}

async function waitFor(predicate: () => boolean) {
  const deadline = Date.now() + 8_000;
  while (Date.now() < deadline) {
    await act(async () => { await new Promise(resolve => setTimeout(resolve, 20)); });
    if (predicate()) return;
  }
  throw new Error(`Replies did not render: ${document.body.textContent}`);
}

async function repliesReadTheCommentViewOfTheService() {
  const page = await readWalletComments(fixture.wallet);
  expect(page.data.items.map(comment => comment.commentId)).toEqual(fixture.items.map(comment => comment.commentId));
  expect(page.nextCursor).toBe(fixture.nextCursor);
  const view = mount('0x' + fixture.wallet.slice(2).toUpperCase());
  await waitFor(() => view.querySelectorAll('li').length === 50);
  const rows = [...view.querySelectorAll('li')];
  expect(rows.map(row => row.querySelector('p')?.textContent)).toEqual(fixture.items.map(comment => comment.body));
  const reply = rows.find(row => row.textContent?.includes('Reply to another wallet'))!;
  expect(reply.querySelector('a')?.textContent).toBe(`Replied on ${shortAddress(fixture.pool)}`);
  expect(reply.querySelector('a')?.getAttribute('href')).toBe(`/coin/${fixture.pool}`);
  const posted = rows.find(row => row.textContent?.includes('Second pool comment'))!;
  expect(posted.querySelector('a')?.textContent).toBe(`Posted on ${shortAddress(fixture.otherPool)}`);
  expect(posted.textContent).toContain('1 likes');
  expect(view.textContent).not.toContain('Hidden comment');
  expect(view.textContent).not.toContain("Another wallet's comment");
  expect(view.textContent).not.toContain('ago');
  expect(view.querySelector('time')).toBeNull();
  expect(view.textContent).toContain('Showing the latest 50 replies.');
}
test(repliesReadTheCommentViewOfTheService.name, repliesReadTheCommentViewOfTheService);

async function repliesShowTheEmptyPageOfTheService() {
  const visitor = mount(fixture.emptyWallet);
  const owner = mount(fixture.emptyWallet, true);
  await waitFor(() => visitor.textContent?.includes('No replies yet') === true && owner.textContent?.includes('No replies yet') === true);
  expect(visitor.textContent).toContain("This wallet hasn't replied to any coins yet");
  expect(visitor.querySelector('a')).toBeNull();
  expect(owner.textContent).toContain('Replies you post on coins show up here');
  expect(owner.querySelector('a')?.getAttribute('href')).toBe('/explore');
  expect(visitor.textContent).not.toContain('Showing the latest');
}
test(repliesShowTheEmptyPageOfTheService.name, repliesShowTheEmptyPageOfTheService);

async function repliesKeepTheUnavailableStateForAnUnknownRoute() {
  process.env.NEXT_PUBLIC_GATEWAY_URL = fixture.missingOrigin;
  const view = mount();
  await waitFor(() => view.textContent?.includes('Check back in a bit') === true);
  expect(view.querySelector('ul')).toBeNull();
  expect(view.querySelector('button')?.textContent).toContain('Try again');
  expect(view.textContent).not.toContain('No replies yet');
}
test(repliesKeepTheUnavailableStateForAnUnknownRoute.name, repliesKeepTheUnavailableStateForAnUnknownRoute);

async function repliesKeepTheUnavailableStateWhenTheServiceIsUnavailable() {
  process.env.NEXT_PUBLIC_GATEWAY_URL = fixture.unavailableOrigin;
  const view = mount();
  await waitFor(() => view.textContent?.includes('Check back in a bit') === true);
  expect(view.querySelector('ul')).toBeNull();
  expect(view.querySelector('button')?.textContent).toContain('Try again');
  expect(view.textContent).not.toContain('No replies yet');
}
test(repliesKeepTheUnavailableStateWhenTheServiceIsUnavailable.name, repliesKeepTheUnavailableStateWhenTheServiceIsUnavailable);
