import { EventEmitter } from 'node:events';
import { createServer, type Server } from 'node:http';
import { QueryClient } from '@tanstack/react-query';
import { expect, test, vi } from 'vitest';
import { decodeFunctionData, encodeFunctionResult, erc20Abi, getAddress, keccak256, type Address, type EIP1193Provider, type Hex } from 'viem';
import { readAssetBalance, usdlAsset, type AssetRef } from './assets';
import { approvedWalletAssets, type WalletNetworkAuthorityV2 } from './backend';
import { getPublicClient, PaxeerWallet } from './client';
import { beginWalletMeasurement, finishWalletMeasurement, type WalletMeasurementRaw } from './measurement';
import { clearWalletSessionQueries } from './runtime';
import { StaleWalletSessionError, WalletSessionController } from './session';

const A = getAddress('0x0000000000000000000000000000000000000001');
const B = getAddress('0x0000000000000000000000000000000000000002');
const TOKEN = getAddress('0x0000000000000000000000000000000000000125');
const measurementIdentity = { runId: 'repair_22_33_907', sourceSha256: 'a'.repeat(64), deploymentManifestHash: 'b'.repeat(64) };
function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>(done => { resolve = done; });
  return { promise, resolve };
}

// A committed in-memory chain store served over actual HTTP JSON-RPC. Each
// block owns its code, precision and balances; calls must name a stored block.
class WalletChainStore {
  chainId = 125;
  readonly blocks = new Map<string, { code: Hex; decimals: number; balance: bigint }>([
    ['0xa', { code: '0x60006000f3', decimals: 6, balance: 1000000n }],
    ['0xb', { code: '0x60006000f3', decimals: 6, balance: 2000000n }],
  ]);
  readonly requests: { method: string; params: unknown[] }[] = [];
  readonly errors: unknown[] = [];
  decimalsGate?: { arrived: ReturnType<typeof deferred>; release: ReturnType<typeof deferred> };
  private observers = new Set<() => void>();
  server?: Server;
  origin = '';
  async open() {
    this.server = createServer(async (request, response) => {
      try {
        let body = '';
        for await (const chunk of request) body += chunk;
        const input = JSON.parse(body) as { id: number; method: string; params: unknown[] };
        this.requests.push({ method: input.method, params: input.params });
        for (const observer of this.observers) observer();
        const result = await this.read(input.method, input.params);
        response.setHeader('Content-Type', 'application/json');
        response.end(JSON.stringify({ jsonrpc: '2.0', id: input.id, result }));
      } catch (error) {
        this.errors.push(error);
        response.statusCode = 500;
        response.end(String(error));
      }
    });
    await new Promise<void>(resolve => this.server!.listen(0, '127.0.0.1', resolve));
    const address = this.server.address();
    if (!address || typeof address === 'string') throw new Error('Loopback TCP address required');
    this.origin = `http://127.0.0.1:${address.port}`;
  }
  async close() {
    this.decimalsGate?.release.resolve();
    if (this.server) {
      this.server.closeAllConnections();
      await new Promise<void>((resolve, reject) => this.server!.close(error => error ? reject(error) : resolve()));
    }
  }
  async waitForCodeReads(count: number) {
    const enough = () => this.requests.filter(request => request.method === 'eth_getCode').length >= count;
    if (enough()) return;
    await new Promise<void>(resolve => {
      const observe = () => { if (enough()) { this.observers.delete(observe); resolve(); } };
      this.observers.add(observe);
    });
  }
  async read(method: string, params: unknown[]) {
    if (method === 'eth_chainId') return `0x${this.chainId.toString(16)}`;
    const block = this.blocks.get(String(params[1]));
    if (!block) throw new Error(`Uncommitted block: ${params[1]}`);
    if (method === 'eth_getCode') {
      expect(String(params[0]).toLowerCase()).toBe(TOKEN.toLowerCase());
      return block.code;
    }
    if (method !== 'eth_call') throw new Error(`Unadmitted RPC method: ${method}`);
    const call = params[0] as { to: Address; data: Hex };
    expect(call.to.toLowerCase()).toBe(TOKEN.toLowerCase());
    const decoded = decodeFunctionData({ abi: erc20Abi, data: call.data });
    if (decoded.functionName === 'decimals') {
      const decimals = block.decimals;
      const gate = this.decimalsGate;
      if (gate) { gate.arrived.resolve(); await gate.release.promise; }
      return encodeFunctionResult({ abi: erc20Abi, functionName: 'decimals', result: decimals });
    }
    if (decoded.functionName === 'balanceOf') {
      expect(decoded.args[0]).toBe(A);
      return encodeFunctionResult({ abi: erc20Abi, functionName: 'balanceOf', result: block.balance });
    }
    throw new Error(`Unadmitted contract method: ${decoded.functionName}`);
  }
  calls(name: 'decimals' | 'balanceOf') {
    return this.requests.filter(request => request.method === 'eth_call' && decodeFunctionData({ abi: erc20Abi, data: (request.params[0] as { data: Hex }).data }).functionName === name);
  }
  authority(manifestHash = 'b'.repeat(64)): WalletNetworkAuthorityV2 {
    return {
      network: { schemaVersion: 1, deploymentVersion: 'reviewed-1', manifestHash, sourceManifestSha256: 'a'.repeat(64), chainId: 125, publicRPC: 'https://rpc.example.test', explorerBase: 'https://explorer.example.test', native: { symbol: 'PAX', decimals: 18 }, settlement: { symbol: 'USDL', decimals: 6 }, addresses: { USDL: TOKEN }, creationFeeRaw: '0' },
      usdlRuntimeHash: keccak256(this.blocks.get('0xa')!.code),
      committedHead: async signal => { signal.throwIfAborted(); return { number: 10n, hash: `0x${'01'.repeat(32)}` }; },
    };
  }
}
function metadataStarts(raw: WalletMeasurementRaw) { return raw.samples.filter(sample => sample.kind === 'asset.metadata' && sample.phase === 'start'); }

async function metadata_measurement_budget() {
  const store = new WalletChainStore();
  await store.open();
  beginWalletMeasurement(measurementIdentity);
  let raw: WalletMeasurementRaw;
  try {
    const client = getPublicClient(store.origin);
    const asset = approvedWalletAssets(store.authority())[1]!;
    const gate = { arrived: deferred(), release: deferred() };
    store.decimalsGate = gate;
    const first = readAssetBalance(client, asset, A, 10n);
    await gate.arrived.promise;
    const second = readAssetBalance(getPublicClient(store.origin), approvedWalletAssets(store.authority())[1]!, A, 10n);
    await store.waitForCodeReads(2);
    gate.release.resolve();
    expect(await Promise.all([first, second])).toEqual([1000000n, 1000000n]);
    expect(await readAssetBalance(client, asset, A, 11n)).toBe(2000000n);
    expect(store.calls('decimals')).toHaveLength(1);
    expect(store.calls('balanceOf')).toHaveLength(3);
    expect(store.calls('balanceOf').map(request => request.params[1])).toEqual(['0xa', '0xa', '0xb']);
    expect(store.requests.filter(request => request.method === 'eth_chainId')).toHaveLength(3);
    expect(store.requests.filter(request => request.method === 'eth_getCode')).toHaveLength(3);
    expect(store.errors).toEqual([]);
  } finally { raw = finishWalletMeasurement(); await store.close(); }
  expect(raw.dropped).toBe(0);
  expect(metadataStarts(raw)).toHaveLength(1);
  expect(raw.samples.filter(sample => sample.kind === 'asset.metadata' && sample.phase === 'end').map(sample => sample.outcome)).toEqual(['resolved']);
}
test(metadata_measurement_budget.name, metadata_measurement_budget);

async function metadata_identity_invalidation() {
  const store = new WalletChainStore();
  await store.open();
  try {
    const client = getPublicClient(store.origin);
    const approved = approvedWalletAssets(store.authority())[1]!;
    expect(await readAssetBalance(client, approved, A, 10n)).toBe(1000000n);
    const redeployed = approvedWalletAssets(store.authority('c'.repeat(64)))[1]!;
    expect(await readAssetBalance(client, redeployed, A, 10n)).toBe(1000000n);
    expect(store.calls('decimals')).toHaveLength(2);
    expect(await readAssetBalance(client, redeployed, A, 10n)).toBe(1000000n);
    expect(store.calls('decimals')).toHaveLength(2);
    const block = store.blocks.get('0xb')!;
    block.code = '0x60016000f3';
    block.decimals = 18;
    await expect(readAssetBalance(client, redeployed, A, 11n)).rejects.toThrow('runtime differs');
    expect(store.calls('balanceOf')).toHaveLength(3);
    const reapproved = usdlAsset(TOKEN, keccak256(block.code));
    await expect(readAssetBalance(client, reapproved, A, 11n)).rejects.toThrow('decimals mismatch');
    expect(store.calls('decimals')).toHaveLength(3);
    block.decimals = 6;
    expect(await readAssetBalance(client, reapproved, A, 11n)).toBe(2000000n);
    expect(store.calls('decimals')).toHaveLength(4);
    store.chainId = 1;
    await expect(readAssetBalance(client, reapproved, A, 11n)).rejects.toThrow('chain 125');
    store.chainId = 125;
    expect(await readAssetBalance(client, reapproved, A, 11n)).toBe(2000000n);
    expect(store.calls('decimals')).toHaveLength(5);
    block.code = '0x';
    await expect(readAssetBalance(client, reapproved, A, 11n)).rejects.toThrow('code unavailable');
    block.code = '0x60016000f3';
    expect(await readAssetBalance(client, reapproved, A, 11n)).toBe(2000000n);
    expect(store.calls('decimals')).toHaveLength(6);
    const unattested: AssetRef = usdlAsset(TOKEN);
    expect(await readAssetBalance(client, unattested, A, 11n)).toBe(2000000n);
    block.code = '0x60026000f3';
    block.decimals = 18;
    await expect(readAssetBalance(client, unattested, A, 11n)).rejects.toThrow('decimals mismatch');
    expect(store.errors).toEqual([]);
  } finally { await store.close(); }
}
test(metadata_identity_invalidation.name, metadata_identity_invalidation);

// This provider owns a real in-memory authorization store and emits events
// when its account/chain change. PaxeerWallet consumes it via viem's transport.
class StoredWalletProvider extends EventEmitter {
  account = A;
  chainId = 125;
  authorize(account: Address) { this.account = account; this.emit('accountsChanged', [account]); }
  request = (async ({ method, params }: { method: string; params?: readonly unknown[] }) => {
    if (method === 'eth_accounts') return [this.account];
    if (method === 'eth_chainId') return `0x${this.chainId.toString(16)}`;
    if (method === 'wallet_switchEthereumChain') {
      this.chainId = Number((params![0] as { chainId: string }).chainId);
      this.emit('chainChanged', `0x${this.chainId.toString(16)}`);
      return null;
    }
    throw new Error(`Unadmitted wallet request: ${method}`);
  }) as EIP1193Provider['request'];
}
function cacheKey(account: Address) { return ['readContract', { functionName: 'allowance', args: [account, TOKEN] }] as const; }

async function session_burst_budget() {
  vi.useFakeTimers();
  const queries = new QueryClient();
  let cleanups = 0, refreshes = 0;
  const sessions = new WalletSessionController((previous, invalidated) => { cleanups++; clearWalletSessionQueries(queries, previous, invalidated); }, () => { refreshes++; });
  const provider = new StoredWalletProvider();
  const wallet = new PaxeerWallet(provider);
  sessions.bind(provider);
  sessions.select('stored', 'injected', A, 125, ['stored']);
  await vi.advanceTimersByTimeAsync(50);
  cleanups = 0; refreshes = 0;
  queries.setQueryData(cacheKey(A), 'A-private');
  queries.setQueryData(cacheKey(B), 'B-private');
  queries.setQueryData(['token-stats', TOKEN], 'public-market');
  const original = sessions.capture();
  let release!: (value: string) => void;
  const delayed = sessions.run(() => new Promise<string>(resolve => { release = resolve; }));
  const delayedAssertion = expect(delayed).rejects.toBeInstanceOf(StaleWalletSessionError);
  beginWalletMeasurement(measurementIdentity);
  let raw: WalletMeasurementRaw;
  try {
    provider.authorize(B);
    expect(sessions.sessionEpoch).toBe(original.sessionEpoch + 1);
    expect(original.signal.aborted).toBe(true);
    expect(sessions.cacheReady).toBe(false);
    expect(() => sessions.assertCurrent(original)).toThrow(StaleWalletSessionError);
    const intermediate = sessions.capture();
    await expect(sessions.run(async () => queries.getQueryData(cacheKey(B)))).rejects.toBeInstanceOf(StaleWalletSessionError);
    await vi.advanceTimersByTimeAsync(20);
    const switched = provider.request({ method: 'wallet_switchEthereumChain', params: [{ chainId: '0x7d' }] });
    expect(sessions.sessionEpoch).toBe(original.sessionEpoch + 2);
    expect(intermediate.signal.aborted).toBe(true);
    expect(sessions.cacheReady).toBe(false);
    await switched;
    expect(await wallet.getAccounts()).toEqual([B]);
    await wallet.assertChain();
    release('stale-A-result');
    await delayedAssertion;
    await vi.advanceTimersByTimeAsync(49);
    expect(cleanups).toBe(0); expect(refreshes).toBe(0);
    await expect(sessions.run(async () => queries.getQueryData(cacheKey(A)))).rejects.toBeInstanceOf(StaleWalletSessionError);
    const beforeReady = sessions.current;
    await vi.advanceTimersByTimeAsync(1);
    expect(cleanups).toBe(1); expect(refreshes).toBe(1);
    expect(sessions.cacheReady).toBe(true);
    expect(sessions.current).not.toBe(beforeReady);
    expect(sessions.current?.account).toBe(B);
    expect(sessions.sessionEpoch).toBe(original.sessionEpoch + 2);
    expect(queries.getQueryData(cacheKey(A))).toBeUndefined();
    expect(queries.getQueryData(cacheKey(B))).toBeUndefined();
    expect(queries.getQueryData(['token-stats', TOKEN])).toBe('public-market');
    expect(await sessions.run(async () => queries.getQueryData(cacheKey(B)))).toBeUndefined();
    await vi.advanceTimersByTimeAsync(100);
    expect(cleanups).toBe(1); expect(refreshes).toBe(1);
  } finally { raw = finishWalletMeasurement(); sessions.dispose(); queries.clear(); vi.useRealTimers(); }
  expect(raw.dropped).toBe(0);
  expect(raw.samples.filter(sample => sample.kind === 'wallet.event')).toHaveLength(2);
  expect(raw.samples.filter(sample => sample.kind === 'wallet.cache_clear')).toHaveLength(1);
  expect(raw.samples.filter(sample => sample.kind === 'wallet.refresh_batch')).toHaveLength(1);
  const events = raw.samples.filter(sample => sample.kind === 'wallet.event');
  const clear = raw.samples.find(sample => sample.kind === 'wallet.cache_clear')!;
  expect(clear.monotonicMs).toBeGreaterThanOrEqual(events[1]!.monotonicMs + 50);
}
test(session_burst_budget.name, session_burst_budget);

async function session_disconnect_burst() {
  vi.useFakeTimers();
  let cleanups = 0, refreshes = 0;
  const sessions = new WalletSessionController(() => { cleanups++; }, () => { refreshes++; });
  const provider = new StoredWalletProvider();
  sessions.bind(provider);
  sessions.select('stored', 'injected', A, 125, ['stored']);
  await vi.advanceTimersByTimeAsync(50);
  cleanups = 0; refreshes = 0;
  try {
    const epoch = sessions.sessionEpoch;
    provider.authorize(B);
    provider.emit('accountsChanged', []);
    expect(sessions.sessionEpoch).toBe(epoch + 2);
    expect(sessions.current).toBeNull();
    expect(sessions.cacheReady).toBe(false);
    await vi.advanceTimersByTimeAsync(50);
    expect(cleanups).toBe(1); expect(refreshes).toBe(0);
    expect(sessions.cacheReady).toBe(true);
    expect(() => sessions.capture()).toThrow('Connect');
    provider.emit('chainChanged', 'invalid');
    sessions.dispose();
    expect(cleanups).toBe(2);
    await vi.advanceTimersByTimeAsync(100);
    expect(cleanups).toBe(2); expect(refreshes).toBe(0);
    expect(provider.listenerCount('accountsChanged')).toBe(0);
    expect(provider.listenerCount('chainChanged')).toBe(0);
  } finally { vi.useRealTimers(); }
}
test(session_disconnect_burst.name, session_disconnect_burst);

async function session_cleanup_failure() {
  vi.useFakeTimers();
  let failCleanup = false, refreshes = 0;
  const sessions = new WalletSessionController(() => {
    if (failCleanup) throw new Error('Private cache cleanup failed');
  }, () => { refreshes++; });
  const provider = new StoredWalletProvider();
  sessions.bind(provider);
  sessions.select('stored', 'injected', A, 125, ['stored']);
  await vi.advanceTimersByTimeAsync(50);
  refreshes = 0;
  try {
    failCleanup = true;
    provider.authorize(B);
    await expect(vi.advanceTimersByTimeAsync(50)).rejects.toThrow('Private cache cleanup failed');
    expect(sessions.cacheReady).toBe(false);
    await expect(sessions.run(async () => 'private-data')).rejects.toBeInstanceOf(StaleWalletSessionError);
    expect(refreshes).toBe(0);
  } finally { failCleanup = false; sessions.dispose(); vi.useRealTimers(); }
}
test(session_cleanup_failure.name, session_cleanup_failure);
