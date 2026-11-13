import { expect, test } from 'vitest';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import { randomUUID } from 'node:crypto';
import { privateKeyToAccount } from 'viem/accounts';
import {
  encodeAbiParameters,
  encodeFunctionData,
  keccak256,
  parseTransaction,
  recoverTransactionAddress,
  toHex,
  type Hex,
} from 'viem';
import {
  ForegroundLifecycle,
  executeExternalWallet,
  type WalletExecutionProvider,
} from '../../src/runtime/lifecycle.ts';
import { PendingOperations } from '../../src/runtime/deep-links.ts';
import {
  SessionController,
  type CredentialStore,
} from '../../src/runtime/session.ts';
import { SnapshotCache } from '../../src/runtime/offline-cache.ts';
import {
  MobileServices,
  type PublicConfiguration,
} from '../../src/runtime/service-client.ts';
import {
  prepareTradeIdentity,
  type TradeIdentity,
} from '../../src/runtime/trade-recovery.ts';
import { nativeRouterAbi } from '../../src/runtime/quote-transaction.ts';
import {
  decodeContract,
  decodeV2Contract,
} from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import type { Quote } from '../../../../../sdk/launchpad/packages/sdk/src/types.gen.ts';

const owner = privateKeyToAccount(`0x${'11'.repeat(32)}`);
const other = privateKeyToAccount(`0x${'22'.repeat(32)}`);
const router = '0x3333333333333333333333333333333333333333';
const pool = '0x4444444444444444444444444444444444444444';
const origin = 'https://kindle.test';
const blockHash = `0x${'55'.repeat(32)}` as Hex;
const signal = () => new AbortController().signal;
type Key = Parameters<CredentialStore['get']>[0];
class InstallationStore implements CredentialStore {
  closed = false;
  constructor(readonly values = new Map<Key, string>()) {}
  check() {
    if (this.closed) throw new Error('Installation closed');
  }
  async get(key: Key) {
    this.check();
    return this.values.get(key) ?? null;
  }
  async set(key: Key, value: string) {
    this.check();
    this.values.set(key, value);
  }
  async remove(key: Key) {
    this.check();
    this.values.delete(key);
  }
  async clear() {
    this.check();
    this.values.clear();
  }
  close() {
    this.closed = true;
  }
  reopen() {
    return new InstallationStore(this.values);
  }
}
function envelope(data: unknown) {
  return {
    requestId: randomUUID(),
    generatedAt: new Date().toISOString(),
    nextCursor: null,
    quality: 'complete',
    revision: '1',
    watermark: null,
    data,
  };
}
function reply(
  response: ServerResponse,
  body: unknown,
  headers: Record<string, string> = {},
) {
  response.writeHead(200, { 'Content-Type': 'application/json', ...headers });
  response.end(JSON.stringify(body));
}
async function bodyOf(request: IncomingMessage) {
  let text = '';
  for await (const chunk of request) text += String(chunk);
  return JSON.parse(text);
}
class WalletChainFixture {
  url = '';
  errors: unknown[] = [];
  submissions: Hex[] = [];
  transaction: Record<string, unknown> | undefined;
  receipt: Record<string, unknown> | undefined;
  loseResponse = true;
  afterAcceptance: (() => void) | undefined;
  mined = true;
  reverted = false;
  chain = 125;
  canonicalHash = blockHash;
  mutateTransaction: ((tx: Record<string, unknown>) => void) | undefined;
  mutateReceipt: ((receipt: Record<string, unknown>) => void) | undefined;
  afterTransaction: (() => void) | undefined;
  identity = {
    accountId: '11111111-1111-4111-8111-111111111111',
    sessionId: '22222222-2222-4222-8222-222222222222',
    wallet: owner.address,
    authEpoch: 1,
    proofAt: new Date().toISOString(),
    accessExpiresAt: new Date(Date.now() + 3600_000).toISOString(),
  };
  readonly server = createServer((request, response) => {
    void this.handle(request, response).catch((error) => {
      this.errors.push(error);
      response.writeHead(500);
      response.end(String(error));
    });
  });
  async open() {
    await new Promise<void>((resolve) =>
      this.server.listen(0, '127.0.0.1', resolve),
    );
    const address = this.server.address();
    if (!address || typeof address === 'string')
      throw new Error('Loopback listener required');
    this.url = `http://127.0.0.1:${address.port}`;
    return this;
  }
  async close() {
    await new Promise<void>((resolve, reject) => {
      this.server.close((error) => (error ? reject(error) : resolve()));
      this.server.closeAllConnections();
    });
    expect(this.errors).toEqual([]);
  }
  configuration(): PublicConfiguration {
    return {
      apiOrigin: origin,
      identityOrigin: origin,
      mediaOrigin: origin,
      canonicalOrigin: origin,
      identityURI: origin,
      streamOrigin: 'wss://kindle.test',
      approvedStreamOrigins: ['wss://kindle.test'],
      production: true,
      manifestDigest: 'a'.repeat(64),
      manifestURL: `${this.url}/manifest`,
      optionalFeatures: { chat: false, live: false },
      approvedPlaybackOrigins: [],
      walletConnect: {
        projectId: 'b'.repeat(32),
        walletLink: `${origin}/wallet`,
        rpcURL: this.url,
        iconURL: `${origin}/icon.png`,
      },
    };
  }
  services() {
    return new MobileServices(
      this.configuration(),
      undefined,
      async (url, init) => {
        const target = new URL(url);
        expect(target.origin).toBe(origin);
        return fetch(`${this.url}${target.pathname}${target.search}`, init);
      },
    );
  }
  async handle(request: IncomingMessage, response: ServerResponse) {
    if (request.url === '/manifest')
      return reply(response, {
        schemaVersion: 1,
        core: { chainId: 125, manifestSha256: 'a'.repeat(64) },
      });
    if (request.url === '/v2/auth/session')
      return reply(
        response,
        { data: this.identity },
        { 'X-CSRF-Token': 'c'.repeat(32) },
      );
    if (request.url === '/v2/auth/sessions?limit=50')
      return reply(
        response,
        decodeV2Contract(
          'SessionsResponse',
          envelope({
            items: [
              decodeV2Contract('Session', {
                sessionId: this.identity.sessionId,
                wallet: this.identity.wallet,
                deviceLabel: 'Fixture installation',
                scopes: ['user'],
                accessExpiresAt: this.identity.accessExpiresAt,
                refreshExpiresAt: new Date(
                  Date.now() + 86400_000,
                ).toISOString(),
                revokedAt: null,
              }),
            ],
          }),
        ),
      );
    if (request.url === '/wallet/activity')
      return reply(response, {
        transactionHash: this.transaction?.hash,
        account: this.transaction?.from,
        nonce: this.transaction?.nonce,
      });
    if (request.url === '/wallet/submit') {
      const trade: TradeIdentity = await bodyOf(request);
      const tx = trade.request;
      expect(tx.from.toLowerCase()).toBe(owner.address.toLowerCase());
      expect(trade.nonce).toBe(7);
      const raw = await owner.signTransaction({
        chainId: 125,
        type: 'eip1559',
        nonce: trade.nonce,
        to: tx.to as Hex,
        data: tx.data as Hex,
        value: BigInt(tx.valueRaw),
        gas: 250000n,
        maxFeePerGas: 2n,
        maxPriorityFeePerGas: 1n,
      });
      const signed = parseTransaction(raw);
      const from = await recoverTransactionAddress({
        serializedTransaction: raw,
      });
      expect(from.toLowerCase()).toBe(owner.address.toLowerCase());
      expect(signed.chainId).toBe(125);
      this.submissions.push(raw);
      const hash = keccak256(raw);
      this.transaction = {
        hash,
        from,
        to: signed.to,
        input: signed.data,
        value: toHex(signed.value ?? 0n),
        nonce: toHex(signed.nonce ?? 0),
        chainId: toHex(signed.chainId ?? 0),
        type: '0x2',
        gas: '0x3d090',
        gasPrice: '0x2',
        maxFeePerGas: '0x2',
        maxPriorityFeePerGas: '0x1',
        accessList: [],
        r: signed.r,
        s: signed.s,
        v: toHex(signed.v ?? 0n),
        yParity: toHex(signed.yParity ?? 0),
        blockHash,
        blockNumber: '0xa',
        transactionIndex: '0x0',
      };
      this.receipt = {
        transactionHash: hash,
        from,
        to: signed.to,
        blockHash,
        blockNumber: '0xa',
        transactionIndex: '0x0',
        status: this.reverted ? '0x0' : '0x1',
        gasUsed: '0x100',
        cumulativeGasUsed: '0x100',
        effectiveGasPrice: '0x2',
        type: '0x2',
        logs: [],
        logsBloom: `0x${'00'.repeat(256)}`,
        contractAddress: null,
      };
      this.afterAcceptance?.();
      if (this.loseResponse) {
        response.destroy();
        return;
      }
      return reply(response, { transactionHash: hash });
    }
    expect(request.url).toBe('/');
    const rpc = (await bodyOf(request)) as {
      id: number;
      method: string;
      params: unknown[];
    };
    let result: unknown;
    switch (rpc.method) {
      case 'eth_chainId':
        result = toHex(this.chain);
        break;
      case 'eth_getTransactionCount':
        expect(rpc.params).toEqual([owner.address, 'pending']);
        result = '0x7';
        break;
      case 'eth_call':
        result = encodeAbiParameters(
          [{ type: 'uint256' }, { type: 'uint256' }, { type: 'uint256' }],
          [200n, 1n, 1n],
        );
        break;
      case 'eth_getTransactionByHash': {
        expect(rpc.params).toEqual([this.transaction?.hash]);
        const tx = { ...this.transaction };
        this.mutateTransaction?.(tx);
        this.afterTransaction?.();
        result = tx;
        break;
      }
      case 'eth_getTransactionReceipt': {
        expect(rpc.params).toEqual([this.transaction?.hash]);
        const receipt = { ...this.receipt };
        this.mutateReceipt?.(receipt);
        result = this.mined ? receipt : null;
        break;
      }
      case 'eth_getBlockByNumber':
        result = {
          number: '0xa',
          hash: this.canonicalHash,
          timestamp: toHex(Math.floor(Date.now() / 1000)),
          transactions: this.transaction ? [this.transaction.hash] : [],
          gasLimit: '0x1000000',
          gasUsed: '0x100',
          difficulty: '0x0',
          extraData: '0x',
          parentHash: `0x${'00'.repeat(32)}`,
          size: '0x100',
          uncles: [],
        };
        break;
      default:
        throw new Error(`Undeclared fixture RPC: ${rpc.method}`);
    }
    reply(response, { jsonrpc: '2.0', id: rpc.id, result });
  }
}
class ExternalWalletPeer implements WalletExecutionProvider {
  readonly supportedChain = 125 as const;
  readonly supportsTrade = true;
  connectedAccount = owner.address;
  constructor(readonly fixture: WalletChainFixture) {}
  async connect() {
    return { address: this.connectedAccount, proofKind: 'eoa_eip191' as const };
  }
  async signMessage(message: string) {
    return owner.signMessage({ message });
  }
  async prepareTrade(quote: Quote, abort: AbortSignal) {
    const request = decodeContract('TransactionRequest', {
      chainId: '125',
      from: this.connectedAccount,
      to: router,
      valueRaw: '0',
      gasLimitRaw: '250000',
      maxFeePerGasRaw: '2',
      maxPriorityFeePerGasRaw: '1',
      data: encodeFunctionData({
        abi: nativeRouterAbi,
        functionName: 'buy',
        args: [
          quote.poolAddress as Hex,
          BigInt(quote.inputRaw),
          BigInt(quote.minOutputRaw),
          BigInt(Math.floor(Date.parse(quote.validUntil) / 1000)),
        ],
      }),
    });
    return prepareTradeIdentity(
      this.fixture.url,
      request,
      quote.quoteId,
      abort,
    );
  }
  async submitTrade(identity: TradeIdentity, abort: AbortSignal) {
    const response = await fetch(`${this.fixture.url}/wallet/submit`, {
      method: 'POST',
      signal: abort,
      body: JSON.stringify(identity),
    });
    return (await response.json()) as { transactionHash: string };
  }
  async execute(input: { quote: Quote }, abort: AbortSignal) {
    return this.submitTrade(await this.prepareTrade(input.quote, abort), abort);
  }
}
async function openApplication(
  fixture: WalletChainFixture,
  storage = new InstallationStore(),
) {
  const services = fixture.services(),
    session = new SessionController(storage, new SnapshotCache());
  await session.restore(services);
  const pending = new PendingOperations(storage, origin);
  await pending.restore();
  const lifecycle = new ForegroundLifecycle(
    services,
    session,
    pending,
    () => {},
  );
  await lifecycle.foreground();
  return {
    services,
    session,
    pending,
    lifecycle,
    storage,
    wallet: new ExternalWalletPeer(fixture),
  };
}
type Application = Awaited<ReturnType<typeof openApplication>>;
function stopApplication(app: Application) {
  app.lifecycle.background();
  app.session.cancel();
  app.storage.close();
}
function operation() {
  return {
    requestId: randomUUID(),
    chainId: 125 as const,
    startedAt: Date.now(),
    returnRoute: '/portfolio' as const,
    state: 'awaiting_wallet' as const,
  };
}
async function loseAcceptedTrade(
  app: Application,
  fixture: WalletChainFixture,
) {
  await app.lifecycle.requestQuote({
    poolAddress: pool,
    side: 'buy',
    inputRaw: '100',
    slippageBps: 50,
  });
  expect(app.lifecycle.canExecute).toBe(true);
  await expect(
    executeExternalWallet(
      app.wallet,
      app.lifecycle,
      app.pending,
      operation(),
      signal(),
    ),
  ).rejects.toThrow();
  expect(fixture.submissions).toHaveLength(1);
  expect(app.pending.current?.state).toBe('unknown');
  expect(app.pending.current?.transactionHash).toBeUndefined();
  expect(app.pending.trade?.nonce).toBe(7);
  const activity = (await (
    await fetch(`${fixture.url}/wallet/activity`)
  ).json()) as { transactionHash: string };
  return activity.transactionHash;
}
async function mobileTradeUnknownRecovery() {
  const fixture = await new WalletChainFixture().open();
  let app = await openApplication(fixture);
  try {
    const hash = await loseAcceptedTrade(app, fixture),
      storage = app.storage;
    stopApplication(app);
    await expect(storage.get('pending')).rejects.toThrow('closed');
    app = await openApplication(fixture, storage.reopen());
    expect(app.lifecycle.tradeRecovery).toMatchObject({
      status: 'awaiting_hash',
      account: owner.address,
      chainId: 125,
      nonce: 7,
    });
    expect(app.lifecycle.state.receiptReconciled).toBe(false);
    const callback = `${origin}/wallet/return?requestId=${encodeURIComponent(app.pending.current!.requestId)}`;
    await app.pending.receive(callback);
    expect(app.pending.current?.transactionHash).toBeUndefined();
    expect(app.pending.trade?.nonce).toBe(7);
    await expect(app.pending.receive(callback)).rejects.toThrow('DUPLICATE');
    await app.lifecycle.requestQuote({
      poolAddress: pool,
      side: 'buy',
      inputRaw: '100',
      slippageBps: 50,
    });
    await expect(
      executeExternalWallet(
        app.wallet,
        app.lifecycle,
        app.pending,
        operation(),
        signal(),
      ),
    ).rejects.toThrow('unavailable');
    expect(await app.lifecycle.recoverTrade(hash)).toBe(true);
    expect(app.lifecycle.tradeRecoveryResult).toBe('confirmed');
    expect(app.pending.current).toBeUndefined();
    expect(await app.storage.get('pending')).toBeNull();
    expect(app.lifecycle.canExecute).toBe(false);
    stopApplication(app);
    app = await openApplication(fixture, storage.reopen());
    expect(app.lifecycle.state.receiptReconciled).toBe(true);
    expect(app.lifecycle.tradeRecovery).toBeUndefined();
    expect(fixture.submissions).toHaveLength(1);
  } finally {
    stopApplication(app);
    await fixture.close();
  }
}
async function mobileTradeRecoveryIdentityAndCanonicalReceipt() {
  const fixture = await new WalletChainFixture().open();
  const app = await openApplication(fixture);
  try {
    const hash = await loseAcceptedTrade(app, fixture);
    await expect(
      app.lifecycle.recoverTrade('untrusted-callback'),
    ).rejects.toThrow('hash');
    for (const [field, value] of [
      ['from', other.address],
      ['to', other.address],
      ['nonce', '0x8'],
      ['chainId', '0x1'],
      ['input', '0x'],
      ['value', '0x1'],
      ['hash', `0x${'66'.repeat(32)}`],
    ] as const) {
      fixture.mutateTransaction = (tx) => {
        tx[field] = value;
      };
      await expect(app.lifecycle.recoverTrade(hash)).rejects.toThrow(
        'mismatch',
      );
      expect(app.pending.current?.state).toBe('unknown');
      expect(await app.storage.get('pending')).not.toBeNull();
    }
    fixture.mutateTransaction = undefined;
    fixture.chain = 1;
    await expect(app.lifecycle.recoverTrade(hash)).rejects.toThrow('chain');
    fixture.chain = 125;
    fixture.canonicalHash = `0x${'77'.repeat(32)}`;
    await expect(app.lifecycle.recoverTrade(hash)).rejects.toThrow('canonical');
    fixture.canonicalHash = blockHash;
    for (const [field, value] of [
      ['from', other.address],
      ['to', other.address],
      ['transactionHash', `0x${'66'.repeat(32)}`],
      ['blockNumber', '0xb'],
      ['transactionIndex', '0x1'],
      ['status', '0x2'],
    ] as const) {
      fixture.mutateReceipt = (receipt) => {
        receipt[field] = value;
      };
      await expect(app.lifecycle.recoverTrade(hash)).rejects.toThrow(
        'canonical',
      );
      expect(app.lifecycle.state.receiptReconciled).toBe(false);
    }
    fixture.mutateReceipt = undefined;
    expect(await app.lifecycle.recoverTrade(hash)).toBe(true);
    expect(fixture.submissions).toHaveLength(1);
  } finally {
    stopApplication(app);
    await fixture.close();
  }
}
async function mobileTradePendingRecoveryAcrossRestart() {
  const fixture = await new WalletChainFixture().open();
  fixture.mined = false;
  let app = await openApplication(fixture);
  try {
    const hash = await loseAcceptedTrade(app, fixture);
    expect(await app.lifecycle.recoverTrade(hash)).toBe(false);
    expect(app.pending.current).toMatchObject({
      state: 'submitted',
      transactionHash: hash,
    });
    const storage = app.storage;
    stopApplication(app);
    app = await openApplication(fixture, storage.reopen());
    expect(app.lifecycle.tradeRecovery?.status).toBe('pending');
    expect(app.lifecycle.state.receiptReconciled).toBe(false);
    fixture.mined = true;
    await app.lifecycle.foreground();
    expect(app.pending.current).toBeUndefined();
    expect(app.lifecycle.tradeRecoveryResult).toBe('confirmed');
    expect(fixture.submissions).toHaveLength(1);
  } finally {
    stopApplication(app);
    await fixture.close();
  }
}
async function mobileTradeProcessDeathBeforeWalletResponse() {
  const fixture = await new WalletChainFixture().open();
  let app = await openApplication(fixture);
  try {
    await app.lifecycle.requestQuote({
      poolAddress: pool,
      side: 'buy',
      inputRaw: '100',
      slippageBps: 50,
    });
    // The wallet accepts a real signed transaction, but the installation dies before
    // either the result or an unknown-state catch can be persisted.
    const storage = app.storage;
    fixture.afterAcceptance = () => stopApplication(app);
    await expect(
      executeExternalWallet(
        app.wallet,
        app.lifecycle,
        app.pending,
        operation(),
        signal(),
      ),
    ).rejects.toThrow('closed');
    fixture.afterAcceptance = undefined;
    const journal = JSON.parse(storage.values.get('pending')!);
    expect(journal.pending.operation.state).toBe('awaiting_wallet');
    expect(journal.pending.operation.transactionHash).toBeUndefined();
    expect(journal.trade.nonce).toBe(7);
    const activity = (await (
      await fetch(`${fixture.url}/wallet/activity`)
    ).json()) as { transactionHash: string };
    app = await openApplication(fixture, storage.reopen());
    expect(app.pending.current?.state).toBe('unknown');
    expect(await app.lifecycle.recoverTrade(activity.transactionHash)).toBe(
      true,
    );
    expect(fixture.submissions).toHaveLength(1);
  } finally {
    stopApplication(app);
    await fixture.close();
  }
}
async function mobileTradeRecoveryAuthorityAndReversion() {
  const fixture = await new WalletChainFixture().open();
  fixture.reverted = true;
  let app = await openApplication(fixture);
  try {
    const hash = await loseAcceptedTrade(app, fixture);
    const original = app.session.current!;
    app.session.current = {
      ...original,
      identity: { ...original.identity, wallet: other.address },
    };
    await expect(app.lifecycle.recoverTrade(hash)).rejects.toThrow('authority');
    app.session.current = original;
    fixture.afterTransaction = () => app.lifecycle.background();
    await expect(app.lifecycle.recoverTrade(hash, signal())).rejects.toThrow(
      'authority',
    );
    expect(app.pending.current?.state).toBe('unknown');
    fixture.afterTransaction = undefined;
    const storage = app.storage;
    stopApplication(app);
    app = await openApplication(fixture, storage.reopen());
    expect(await app.lifecycle.recoverTrade(hash)).toBe(true);
    expect(app.lifecycle.tradeRecoveryResult).toBe('reverted');
    expect(app.pending.current).toBeUndefined();
    expect(fixture.submissions).toHaveLength(1);
  } finally {
    stopApplication(app);
    await fixture.close();
  }
}

async function mobileTradeKnownHashRecovery() {
  const fixture = await new WalletChainFixture().open();
  fixture.loseResponse = false;
  let app = await openApplication(fixture);
  try {
    await app.lifecycle.requestQuote({
      poolAddress: pool,
      side: 'buy',
      inputRaw: '100',
      slippageBps: 50,
    });
    const result = await executeExternalWallet(
      app.wallet,
      app.lifecycle,
      app.pending,
      operation(),
      signal(),
    );
    expect(app.pending.current).toMatchObject({
      state: 'submitted',
      transactionHash: result.transactionHash,
    });
    const storage = app.storage;
    stopApplication(app);
    app = await openApplication(fixture, storage.reopen());
    expect(app.lifecycle.tradeRecoveryResult).toBe('confirmed');
    expect(app.lifecycle.state.receiptReconciled).toBe(true);
    expect(app.pending.current).toBeUndefined();
    expect(fixture.submissions).toHaveLength(1);
  } finally {
    stopApplication(app);
    await fixture.close();
  }
}

test(mobileTradeUnknownRecovery.name, mobileTradeUnknownRecovery);
test(
  mobileTradeRecoveryIdentityAndCanonicalReceipt.name,
  mobileTradeRecoveryIdentityAndCanonicalReceipt,
);
test(
  mobileTradePendingRecoveryAcrossRestart.name,
  mobileTradePendingRecoveryAcrossRestart,
);
test(
  mobileTradeProcessDeathBeforeWalletResponse.name,
  mobileTradeProcessDeathBeforeWalletResponse,
);
test(
  mobileTradeRecoveryAuthorityAndReversion.name,
  mobileTradeRecoveryAuthorityAndReversion,
);

test(mobileTradeKnownHashRecovery.name, mobileTradeKnownHashRecovery);
