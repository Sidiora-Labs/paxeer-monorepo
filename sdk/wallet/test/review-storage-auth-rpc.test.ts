import { createHmac } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http';
import { createRequire } from 'node:module';
import type { AddressInfo } from 'node:net';
import { gzipSync } from 'node:zlib';
import { act, createElement } from 'react';
import { expect, test } from 'vitest';
import { keccak256, parseTransaction, type Hex } from 'viem';
import {
  PaxeerWallet,
  type EvmApproval,
  type EvmReviewStorage,
  type EvmTransactionReview,
} from '../src/index.js';
import { useStandardAccount, useWallet } from '../src/react.js';
import { JsonRpcClient, JsonRpcTimeoutError } from '../src/rpc.js';
import { readSequencerReceipt, waitForSequencerReceipt } from '../src/receipt.js';

const requireWallet = createRequire(import.meta.url);
const recordedWallet = fixture('./fixtures/gateway/wallet-me.json').body;
const receiptVector = fixture('../../platform/conformance/fixtures/sequencer-receipt-v1.json')
  .cases[0];
const principalA = '5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a10';
const principalB = '5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a11';
const clients: PaxeerWallet[] = [];

function fixture(path: string): any {
  return JSON.parse(readFileSync(new URL(path, import.meta.url), 'utf8'));
}
function pause(ms = 0): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
function token(principal: string): string {
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({ sub: principal, exp: Math.floor(Date.now() / 1000) + 3600 })
  ).toString('base64url');
  const signingInput = `${header}.${payload}`;
  const signature = createHmac('sha256', 'loopback-jwt-signing-key')
    .update(signingInput)
    .digest('base64url');
  return `${signingInput}.${signature}`;
}
function requestPrincipal(req: IncomingMessage): string {
  const jwt = req.headers.authorization?.slice('Bearer '.length);
  return jwt ? JSON.parse(Buffer.from(jwt.split('.')[1]!, 'base64url').toString()).sub : '';
}
function json(res: ServerResponse, value: unknown): void {
  res.writeHead(200, { 'content-type': 'application/json' });
  res.end(JSON.stringify(value));
}
async function body(req: IncomingMessage): Promise<any> {
  let text = '';
  for await (const chunk of req) text += chunk;
  return text ? JSON.parse(text) : null;
}
function auth(req: IncomingMessage, res: ServerResponse): boolean {
  const url = new URL(req.url!, 'http://localhost');
  if (url.pathname.endsWith('/auth/v1/user')) {
    json(res, {
      id: requestPrincipal(req),
      aud: 'authenticated',
      role: 'authenticated',
      app_metadata: {},
      user_metadata: {},
      created_at: '1970-01-01T00:00:00.000Z',
    });
    return true;
  }
  if (url.pathname.endsWith('/auth/v1/logout')) {
    expect(req.method).toBe('POST');
    expect(url.searchParams.get('scope')).toBe('global');
    res.writeHead(204).end();
    return true;
  }
  return false;
}
async function serving(
  handle: (req: IncomingMessage, res: ServerResponse) => void | Promise<void>,
  work: (base: string) => Promise<void>
): Promise<void> {
  const errors: unknown[] = [];
  const server = createServer((req, res) => {
    void Promise.resolve()
      .then(() => handle(req, res))
      .catch((error) => {
        errors.push(error);
        res.writeHead(500).end();
      });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    await work(`http://127.0.0.1:${(server.address() as AddressInfo).port}`);
    expect(errors).toEqual([]);
  } finally {
    for (const client of clients.splice(0)) {
      await client.supabase.auth.stopAutoRefresh();
    }
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}
function wallet(base: string, storage?: EvmReviewStorage, apiUrl = base): PaxeerWallet {
  const client = new PaxeerWallet({
    apiUrl,
    supabaseUrl: base,
    supabaseAnonKey: 'loopback-public-anon-key',
    ...(storage ? { evmReview: { storage } } : {}),
  });
  clients.push(client);
  return client;
}
async function signIn(client: PaxeerWallet, principal = principalA): Promise<void> {
  const result = await client.supabase.auth.setSession({
    access_token: token(principal),
    refresh_token: 'loopback-refresh-token',
  });
  expect(result.error).toBeNull();
}
class ReviewStore implements EvmReviewStorage {
  readonly values = new Map<string, string>();
  async getItem(key: string): Promise<string | null> {
    const value = this.values.get(key) ?? null;
    await pause(1);
    return value;
  }
  async setItem(key: string, value: string): Promise<void> {
    await pause(1);
    this.values.set(key, value);
  }
}
const transaction = {
  type: 'eip1559',
  chainId: 125,
  nonce: 7,
  to: '0x3535353535353535353535353535353535353535',
  value: 1n,
  data: '0x',
  gas: 21000n,
  maxFeePerGas: 2n,
  maxPriorityFeePerGas: 1n,
} as const;

// A loopback gateway peer retains its issued artifacts while SDK clients stop/reopen.
function reviewGateway(environment: string) {
  const artifacts = new Map<string, EvmTransactionReview | EvmApproval>();
  const approvals: unknown[] = [];
  let changedChain = false;
  async function handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
    if (auth(req, res)) return;
    const request = await body(req);
    const principal = requestPrincipal(req);
    if (req.url?.endsWith('/v1/wallet/evm/review')) {
      const tx = parseTransaction(request.transaction as Hex);
      const id = `review-${tx.nonce}`;
      const artifact: EvmTransactionReview = {
        id,
        state: 'reviewed',
        to: tx.to!,
        value: (tx.value ?? 0n).toString(),
        data: tx.data ?? '0x',
        signing_hash: keccak256(request.transaction),
        transaction: request.transaction,
        principal,
        chain_id: tx.chainId!,
        method: '',
        destination: '',
        key_id: `${environment}:${principal}`,
        public_key: 'retained-public-key',
        epoch: 1,
        session_id: `${environment}:${id}`,
        expires_at: '4102444800',
        participants: ['one', 'two', 'three'],
      };
      artifacts.set(`${principal}:${id}`, artifact);
      json(res, artifact);
    } else if (req.url?.endsWith('/v1/wallet/evm/approve')) {
      approvals.push(request);
      const original = artifacts.get(`${principal}:${request.review_id}`)!;
      expect(request.signing_hash).toBe(original.signing_hash);
      const approved: EvmApproval = {
        ...original,
        state: 'approved',
        approval: {
          version: 1,
          principal,
          key_id: original.key_id,
          session_id: original.session_id,
          signing_hash: original.signing_hash,
          expires_at: original.expires_at,
        },
      };
      artifacts.set(`${principal}:${original.id}`, approved);
      json(res, changedChain ? { ...approved, chain_id: approved.chain_id + 1 } : approved);
    } else if (req.url?.includes('/v1/wallet/evm/approvals/')) {
      json(res, artifacts.get(`${principal}:${req.url.split('/').at(-1)}`));
    } else {
      throw new Error(`unexpected gateway request ${req.url}`);
    }
  }
  return {
    handle,
    artifacts,
    approvals,
    changeChain: () => {
      changedChain = true;
    },
  };
}

async function review_storage_environment_recovers_isolated_gateway_and_principal_artifacts(): Promise<void> {
  const store = new ReviewStore();
  const gatewayA = reviewGateway('gateway-a');
  const gatewayB = reviewGateway('gateway-b');
  await serving(gatewayA.handle, async (baseA) => {
    await serving(gatewayB.handle, async (baseB) => {
      const first = wallet(baseA, store);
      await signIn(first);
      const reviewA = await first.reviewEvmTransaction(transaction);
      const approvalA = await first.approveEvmTransaction(reviewA.id);
      await first.signOut();
      const other = wallet(baseB, store);
      await signIn(other);
      const reviewB = await other.reviewEvmTransaction({ ...transaction, chainId: 126 });
      await other.approveEvmTransaction(reviewB.id);
      await other.signOut();
      // Reopening with a path/trailing slash still identifies the same issuing origin.
      const reopenedA = wallet(baseA, store, `${baseA}/alias/`);
      await signIn(reopenedA);
      expect(await reopenedA.evmApprovalStatus(reviewA.id)).toEqual(approvalA);
      expect(await reopenedA.approveEvmTransaction(reviewA.id)).toEqual(approvalA);
      const reopenedB = wallet(baseB, store);
      await signIn(reopenedB);
      const recoveredB = await reopenedB.approveEvmTransaction(reviewB.id);
      expect(recoveredB.key_id).toBe(reviewB.key_id);
      expect(recoveredB.chain_id).toBe(126);
      expect(recoveredB.signing_hash).toBe(reviewB.signing_hash);
      expect(recoveredB.signing_hash).not.toBe(approvalA.signing_hash);
      await signIn(reopenedA, principalB);
      const before = gatewayA.approvals.length;
      await expect(reopenedA.approveEvmTransaction(reviewA.id)).rejects.toThrow(
        'original EVM review is required'
      );
      expect(gatewayA.approvals).toHaveLength(before);
      const userBReview = await reopenedA.reviewEvmTransaction(transaction);
      expect(userBReview.principal).toBe(principalB);
      const persisted = [...store.values.entries()];
      expect(persisted).toHaveLength(3);
      expect(persisted.map(([key]) => key)).toEqual(
        expect.arrayContaining([
          `paxeer:evm-review:v2:${encodeURIComponent(baseA)}:${principalA}`,
          `paxeer:evm-review:v2:${encodeURIComponent(baseB)}:${principalA}`,
          `paxeer:evm-review:v2:${encodeURIComponent(baseA)}:${principalB}`,
        ])
      );
      expect(persisted.map(([, raw]) => JSON.parse(raw)[0].chain_id).sort()).toEqual([
        125, 125, 126,
      ]);
      // Origin isolation must retain the original artifact's chain binding on recovery.
      gatewayB.changeChain();
      await expect(reopenedB.approveEvmTransaction(reviewB.id)).rejects.toThrow(
        'approval does not match the review'
      );
      expect(
        JSON.parse(
          store.values.get(`paxeer:evm-review:v2:${encodeURIComponent(baseB)}:${principalA}`)!
        )[0].chain_id
      ).toBe(126);
    });
  });
}

async function review_storage_environment_serializes_shared_clients_and_refuses_ambiguous_legacy(): Promise<void> {
  const peer = reviewGateway('same-origin');
  await serving(peer.handle, async (base) => {
    const store = new ReviewStore();
    const first = wallet(base, store);
    const second = wallet(base, store, `${base}/alias`);
    await Promise.all([signIn(first), signIn(second)]);
    const reviews = await Promise.all([
      first.reviewEvmTransaction(transaction),
      second.reviewEvmTransaction({ ...transaction, nonce: 8, chainId: 126 }),
    ]);
    expect(store.values.size).toBe(1);
    expect(
      JSON.parse([...store.values.values()][0]!)
        .map((entry: EvmTransactionReview) => entry.id)
        .sort()
    ).toEqual(reviews.map((review) => review.id).sort());
    await Promise.all([first.signOut(), second.signOut()]);
    const reopened = wallet(base, store);
    await signIn(reopened);
    for (const review of reviews) {
      const approval = await reopened.approveEvmTransaction(review.id);
      expect(approval.signing_hash).toBe(review.signing_hash);
      expect(approval.chain_id).toBe(review.chain_id);
    }
    const legacy = new ReviewStore();
    const legacyKey = `paxeer:evm-review:v1:${principalA}`;
    legacy.values.set(legacyKey, JSON.stringify(reviews));
    const old = wallet(base, legacy);
    await signIn(old);
    const before = peer.approvals.length;
    await expect(old.approveEvmTransaction(reviews[0]!.id)).rejects.toThrow(
      /ambiguous legacy EVM reviews.*origin unknown/
    );
    expect(peer.approvals).toHaveLength(before);
    expect(legacy.values.size).toBe(1);
    expect(JSON.parse(legacy.values.get(legacyKey)!)).toEqual(reviews);
    for (const invalid of ['broken-json', '{}']) {
      legacy.values.set(legacyKey, invalid);
      await expect(old.approveEvmTransaction(reviews[0]!.id)).rejects.toThrow(
        /ambiguous legacy EVM reviews.*origin unknown/
      );
      expect(legacy.values.get(legacyKey)).toBe(invalid);
    }
    const emptyLegacy = new ReviewStore();
    emptyLegacy.values.set(legacyKey, '[]');
    const empty = wallet(base, emptyLegacy);
    await signIn(empty);
    await expect(empty.reviewEvmTransaction(transaction)).resolves.toMatchObject({ chain_id: 125 });
  });
}

async function react_auth_refresh_tracks_stable_client_and_discards_replaced_reads(): Promise<void> {
  const { JSDOM } = requireWallet('jsdom');
  const dom = new JSDOM('<!doctype html><div id="root"></div>', { url: 'http://localhost/' });
  const globals = new Map<string, PropertyDescriptor | undefined>();
  for (const [key, value] of Object.entries({
    window: dom.window,
    document: dom.window.document,
    IS_REACT_ACT_ENVIRONMENT: true,
  })) {
    globals.set(key, Object.getOwnPropertyDescriptor(globalThis, key));
    Object.defineProperty(globalThis, key, { configurable: true, writable: true, value });
  }
  const { createRoot } = requireWallet('react-dom/client');
  const root = createRoot(dom.window.document.getElementById('root'));
  let heldPrincipal: string | null = null;
  const pending: { res: ServerResponse; principal: string }[] = [];
  const requests: string[] = [];
  const accountB = {
    ...recordedWallet,
    wallet: { ...recordedWallet.wallet, id: principalB, address: receiptVector.receiptJson.to },
  };
  const account = (principal: string) => (principal === principalA ? recordedWallet : accountB);
  let latest!: {
    active: ReturnType<typeof useWallet>;
    passive: ReturnType<typeof useStandardAccount>;
  };
  function Probe({ client }: { client: PaxeerWallet }) {
    latest = { active: useWallet(client), passive: useStandardAccount(client) };
    return createElement(
      'output',
      null,
      JSON.stringify({ active: latest.active.wallet, passive: latest.passive.data })
    );
  }
  async function settled(assertion: () => void): Promise<void> {
    await expect
      .poll(
        async () => {
          await act(async () => {
            await pause(5);
          });
          assertion();
          return true;
        },
        { timeout: 2000 }
      )
      .toBe(true);
  }
  function empty(): void {
    expect(latest.active.wallet).toBeNull();
    expect(latest.active.chain).toBeNull();
    expect(latest.passive.data).toBeNull();
    expect(latest.active.error).toBeNull();
    expect(latest.passive.error).toBeNull();
  }
  function release(status = 200, onlyPrincipal?: string): void {
    for (let index = pending.length - 1; index >= 0; index -= 1) {
      const { res, principal } = pending[index]!;
      if (onlyPrincipal && principal !== onlyPrincipal) continue;
      pending.splice(index, 1);
      if (status === 200) json(res, account(principal));
      else
        res
          .writeHead(status, { 'content-type': 'application/json' })
          .end(JSON.stringify({ error: 'old_read_failed' }));
    }
  }
  try {
    await serving(
      async (req, res) => {
        if (auth(req, res)) return;
        expect(req.url).toBe('/v1/wallet/me');
        const principal = requestPrincipal(req);
        requests.push(principal);
        if (heldPrincipal === principal) pending.push({ res, principal });
        else json(res, account(principal));
      },
      async (base) => {
        const client = wallet(base);
        await act(async () => {
          root.render(createElement(Probe, { client }));
        });
        await settled(() => {
          empty();
          expect(latest.active.loading).toBe(false);
          expect(latest.passive.loading).toBe(false);
        });
        expect(requests).toEqual([]);
        await act(async () => {
          await signIn(client);
        });
        await settled(() => {
          expect(latest.active.wallet).toEqual(recordedWallet.wallet);
          expect(latest.passive.data).toEqual(recordedWallet);
        });
        heldPrincipal = principalA;
        await act(async () => {
          latest.active.refresh();
          latest.passive.refresh();
        });
        await settled(() => {
          expect(pending).toHaveLength(2);
        });
        await act(async () => {
          await client.signOut();
        });
        empty();
        expect(latest.active.loading).toBe(false);
        expect(latest.passive.loading).toBe(false);
        await act(async () => {
          release();
          await pause(10);
        });
        empty();
        heldPrincipal = null;
        await act(async () => {
          await signIn(client);
        });
        await settled(() => {
          expect(latest.active.wallet).toEqual(recordedWallet.wallet);
          expect(latest.passive.data).toEqual(recordedWallet);
        });
        heldPrincipal = principalA;
        await act(async () => {
          latest.active.refresh();
          latest.passive.refresh();
        });
        await settled(() => {
          expect(pending).toHaveLength(2);
        });
        // Replace the account on the same client while both old reads are in flight.
        heldPrincipal = principalB;
        await act(async () => {
          await signIn(client, principalB);
        });
        await settled(() => {
          expect(pending).toHaveLength(4);
        });
        empty();
        expect(latest.active.loading).toBe(true);
        expect(latest.passive.loading).toBe(true);
        await act(async () => {
          release(200, principalB);
        });
        await settled(() => {
          expect(latest.active.wallet).toEqual(accountB.wallet);
          expect(latest.passive.data).toEqual(accountB);
        });
        await act(async () => {
          release(503);
          await pause(10);
        });
        expect(latest.active.wallet).toEqual(accountB.wallet);
        expect(latest.passive.data).toEqual(accountB);
        expect(latest.active.error).toBeNull();
        expect(latest.passive.error).toBeNull();
        await act(async () => {
          await client.signOut();
        });
        empty();
        expect(requests.filter((principal) => principal === principalB)).toHaveLength(2);
      }
    );
  } finally {
    await act(async () => {
      root.unmount();
    });
    dom.window.close();
    for (const [key, descriptor] of globals) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
}

async function rpc_has_no_request_bound_limits_headers_body_and_decoded_bytes(): Promise<void> {
  await serving(
    async (req, res) => {
      const request = await body(req);
      if (req.url === '/headers') return;
      if (req.url === '/body') {
        res.writeHead(200, { 'content-type': 'application/json' });
        res.write('{"jsonrpc":"2.0",');
        return;
      }
      if (req.url === '/length') {
        res.writeHead(200, { 'content-type': 'application/json', 'content-length': '4096' });
        res.flushHeaders();
        return;
      }
      if (req.url === '/chunked' || req.url === '/gzip') {
        const oversized = JSON.stringify({
          jsonrpc: '2.0',
          id: request.id,
          result: '界'.repeat(100),
        });
        if (req.url === '/gzip') {
          const compressed = gzipSync(oversized);
          expect(compressed.byteLength).toBeLessThan(128);
          res.writeHead(200, {
            'content-type': 'application/json',
            'content-encoding': 'gzip',
            'content-length': compressed.byteLength,
          });
          res.end(compressed);
        } else {
          res.writeHead(200, { 'content-type': 'application/json' });
          const bytes = Buffer.from(oversized);
          res.write(bytes.subarray(0, 80));
          res.end(bytes.subarray(80));
        }
        return;
      }
      const response = (entry: any) => ({ jsonrpc: '2.0', id: entry.id, result: '0x7d' });
      if (req.url === '/wrong-id') json(res, { ...response(request), id: request.id + 1 });
      else if (req.url === '/both')
        json(res, { ...response(request), error: { code: -1, message: 'invalid' } });
      else if (req.url === '/bad-error')
        json(res, { jsonrpc: '2.0', id: request.id, error: { code: 'bad', message: 'invalid' } });
      else if (req.url === '/not-json') res.writeHead(502).end('broken');
      else if (req.url === '/duplicate') json(res, [response(request[0]), response(request[0])]);
      else json(res, Array.isArray(request) ? request.map(response).reverse() : response(request));
    },
    async (base) => {
      for (const path of ['/headers', '/body']) {
        const rpc = new JsonRpcClient({ url: `${base}${path}`, timeoutMs: 80 });
        const started = Date.now();
        await expect(rpc.call('eth_chainId')).rejects.toBeInstanceOf(JsonRpcTimeoutError);
        expect(Date.now() - started).toBeLessThan(1000);
      }
      for (const path of ['/length', '/chunked', '/gzip']) {
        const rpc = new JsonRpcClient({
          url: `${base}${path}`,
          maxResponseBytes: 128,
          timeoutMs: 1000,
        });
        await expect(rpc.call('eth_chainId')).rejects.toMatchObject({
          name: 'JsonRpcTransportError',
          message: 'response exceeds maxResponseBytes',
        });
      }
      const rpc = new JsonRpcClient({ url: `${base}/ok`, timeoutMs: 1000, maxResponseBytes: 128 });
      expect(await rpc.call('eth_chainId')).toBe('0x7d');
      expect(
        await rpc.batch([
          { method: 'eth_chainId', params: [] },
          { method: 'eth_chainId', params: [] },
        ])
      ).toEqual([
        { ok: true, result: '0x7d' },
        { ok: true, result: '0x7d' },
      ]);
      for (const [path, message] of [
        ['/wrong-id', /request id/],
        ['/both', /neither or both/],
        ['/bad-error', /malformed JSON-RPC error/],
        ['/not-json', /not JSON/],
      ] as const) {
        await expect(
          new JsonRpcClient({ url: `${base}${path}` }).call('eth_chainId')
        ).rejects.toThrow(message);
      }
      await expect(
        new JsonRpcClient({ url: `${base}/duplicate` }).batch([
          { method: 'eth_chainId', params: [] },
          { method: 'eth_chainId', params: [] },
        ])
      ).rejects.toThrow('batch response repeats an id');
      expect(new JsonRpcClient({ url: base }).timeoutMs).toBe(30_000);
      expect(new JsonRpcClient({ url: base }).maxResponseBytes).toBe(4 * 1024 * 1024);
      for (const timeoutMs of [0, -1, Infinity, NaN, 1.5, 2147483648]) {
        expect(() => new JsonRpcClient({ url: base, timeoutMs })).toThrow(RangeError);
      }
      for (const maxResponseBytes of [0, -1, Infinity, NaN, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
        expect(() => new JsonRpcClient({ url: base, maxResponseBytes })).toThrow(RangeError);
      }
    }
  );
}

async function rpc_has_no_request_bound_cancels_each_receipt_transport_and_authenticates_success(): Promise<void> {
  const methods = ['pax_sequencerReceipt', 'px_getReceiptKeys', 'eth_getRawTransactionByHash'];
  for (const stalledMethod of methods) {
    const entered = deferred<void>();
    const closed = deferred<void>();
    const seen: string[] = [];
    await serving(
      async (req, res) => {
        const request = await body(req);
        seen.push(request.method);
        if (request.method === stalledMethod) {
          res.on('close', () => closed.resolve());
          res.writeHead(200, { 'content-type': 'application/json' });
          res.write('{');
          entered.resolve();
        } else {
          const result =
            request.method === 'pax_sequencerReceipt'
              ? receiptVector.receiptJson
              : { keys: [receiptVector.pinnedKey] };
          json(res, { jsonrpc: '2.0', id: request.id, result });
        }
      },
      async (base) => {
        const rpc = new JsonRpcClient({ url: base, timeoutMs: 2000 });
        const controller = new AbortController();
        const outcome = waitForSequencerReceipt(
          rpc.call.bind(rpc),
          receiptVector.receiptJson.txHash,
          { rung: 'committed', attempts: 3, intervalMs: 10, signal: controller.signal }
        );
        const rejection = expect(outcome).rejects.toMatchObject({ name: 'AbortError' });
        await entered.promise;
        controller.abort();
        await rejection;
        await closed.promise;
        expect(seen).toEqual(methods.slice(0, methods.indexOf(stalledMethod) + 1));
        await expect(
          readSequencerReceipt(rpc.call.bind(rpc), receiptVector.receiptJson.txHash, {
            signal: controller.signal,
          })
        ).rejects.toMatchObject({ name: 'AbortError' });
        expect(seen).toHaveLength(methods.indexOf(stalledMethod) + 1);
      }
    );
  }
  const seen: string[] = [];
  await serving(
    async (req, res) => {
      const request = await body(req);
      seen.push(request.method);
      const result =
        request.method === 'pax_sequencerReceipt'
          ? receiptVector.receiptJson
          : request.method === 'px_getReceiptKeys'
            ? { keys: [receiptVector.pinnedKey] }
            : receiptVector.rawTransaction;
      json(res, { jsonrpc: '2.0', id: request.id, result });
    },
    async (base) => {
      const rpc = new JsonRpcClient({ url: base });
      const receipt = await readSequencerReceipt(
        rpc.call.bind(rpc),
        receiptVector.receiptJson.txHash
      );
      expect(receipt).toMatchObject(receiptVector.receiptJson);
      expect(receipt?.authentication).toMatchObject({
        level: 'sequencer-signed',
        rawTransaction: receiptVector.rawTransaction,
      });
      expect(seen).toEqual(methods);
    }
  );
}

test(
  review_storage_environment_recovers_isolated_gateway_and_principal_artifacts.name,
  review_storage_environment_recovers_isolated_gateway_and_principal_artifacts
);
test(
  review_storage_environment_serializes_shared_clients_and_refuses_ambiguous_legacy.name,
  review_storage_environment_serializes_shared_clients_and_refuses_ambiguous_legacy
);
test(
  react_auth_refresh_tracks_stable_client_and_discards_replaced_reads.name,
  react_auth_refresh_tracks_stable_client_and_discards_replaced_reads
);
test(
  rpc_has_no_request_bound_limits_headers_body_and_decoded_bytes.name,
  rpc_has_no_request_bound_limits_headers_body_and_decoded_bytes
);
test(
  rpc_has_no_request_bound_cancels_each_receipt_transport_and_authenticates_success.name,
  rpc_has_no_request_bound_cancels_each_receipt_transport_and_authenticates_success
);
