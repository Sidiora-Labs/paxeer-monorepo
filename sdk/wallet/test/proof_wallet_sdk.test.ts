/**
 * Proves: GasStationModule.quote/status, PaxeerProvider.request and PaxeerWallet.sendTransaction -> wallet public exports and loopback HTTP/JSON-RPC -> useSurface.surfaceReceipt, wallet.sidioraQuotePresentation/walletSendFailure, KernelPlan.journeyLadderSteps and walletSigningProvider.
 * Repairs unbound send response identities in types.ts/index.ts/provider.ts,
 * cross-chain/sponsor status reads in modules/gas-station.ts, mutable ladder
 * rungs in ladder.ts, and malformed web quote consent identities in wallet.ts.
 * CompletionAsset and the native activity confirmation contract were retired:
 * the bank asset API and provider confirmation are their current consumers.
 * The application harness signs and verifies constructions and raw EVM
 * transactions; every network crossing uses the real SDK transport. Its
 * journal and the SDK's submission store are closed and reopened from disk.
 * Workspace source is loaded with the application's aliases, without doubles;
 * only the agent SDK uses dist, built first by the declared verification.
 */
import assert from 'node:assert/strict';
import { once } from 'node:events';
import {
  mkdtempSync,
  openSync,
  closeSync,
  readFileSync,
  writeFileSync,
  fsyncSync,
  rmSync,
} from 'node:fs';
import { createServer, type Server } from 'node:http';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from 'vitest';
import { keccak256, toHex, parseTransaction, recoverTransactionAddress, type Hex } from 'viem';
import { privateKeyToAccount } from 'viem/accounts';
import {
  gasQuoteDigest,
  sponsoredBatchCall,
  assembleEip7702Authorization,
  type GasResult,
  type SponsoredBatch,
} from '@sidiora/layerx-sdk/browser';
import type * as WalletSdk from '../src/index.js';
import type { GasSubmissionStore, SequencerReceiptAnswer, SendTxResponse } from '../src/index.js';

type Json = Record<string, any>;
function unwrap<T>(result: GasResult<T>): T {
  assert(result.ok, wire(result));
  return result.value;
}
function wire(value: unknown): string {
  return JSON.stringify(value, (_key, item: unknown) =>
    typeof item === 'bigint' ? item.toString() : item
  );
}

/** Actual file handles are discarded on close; reopening reparses the bytes. */
class SubmissionStore implements GasSubmissionStore {
  private fd: number | null;
  private entries: Record<string, string>;
  constructor(path: string) {
    this.fd = openSync(path, 'a+', 0o600);
    const saved = readFileSync(this.fd, 'utf8');
    const last = saved.trim().split('\n').at(-1);
    this.entries = last ? JSON.parse(last) : {};
  }
  getItem(key: string): string | null {
    assert(this.fd !== null, 'submission store must be open');
    return this.entries[key] ?? null;
  }
  setItem(key: string, value: string): void {
    assert(this.fd !== null, 'submission store must be open');
    this.entries[key] = value;
    // append-only snapshots make writes durable without keeping an open cache on restart
    writeFileSync(this.fd, wire(this.entries) + '\n');
    fsyncSync(this.fd);
  }
  close(): void {
    if (this.fd !== null) closeSync(this.fd);
    this.fd = null;
    this.entries = {};
  }
}

class Journal {
  private fd: number | null;
  readonly entries: Json[];
  constructor(path: string) {
    this.fd = openSync(path, 'a+', 0o600);
    this.entries = readFileSync(this.fd, 'utf8')
      .split('\n')
      .filter(Boolean)
      .map((line) => JSON.parse(line));
  }
  put(entry: Json): void {
    assert(this.fd !== null, 'producer journal must be open');
    writeFileSync(this.fd, wire(entry) + '\n');
    fsyncSync(this.fd);
    this.entries.push(JSON.parse(wire(entry)));
  }
  close(): void {
    if (this.fd !== null) closeSync(this.fd);
    this.fd = null;
  }
}

async function proof_wallet_sdk_producer_transport_consumer() {
  const root = resolve(dirname(fileURLToPath(import.meta.url)), '../../..');
  const temporary = mkdtempSync(join(tmpdir(), 'wallet-sdk-proof-'));
  const require = createRequire(import.meta.url);
  const vite = require(
    require.resolve('vite', { paths: [dirname(require.resolve('vitest/package.json'))] })
  );
  const loader = await vite.createServer({
    root,
    configFile: false,
    envFile: false,
    cacheDir: join(temporary, 'vite'),
    server: { middlewareMode: true, watch: null },
    resolve: {
      alias: {
        '@paxeer/wallet': join(root, 'sdk/wallet/src/index.ts'),
        '@': join(root, 'apps/wallet/src'),
        // Next binds this marker to its server entry when loading server actions.
        'server-only': require.resolve('next/dist/compiled/server-only/empty.js'),
      },
    },
    esbuild: { jsx: 'automatic' },
  });
  let server: Server | undefined;
  let journal = new Journal(join(temporary, 'producer.jsonl'));
  let store: SubmissionStore | undefined;
  let wallet: InstanceType<typeof WalletSdk.PaxeerWallet> | undefined;
  const producerErrors: unknown[] = [];
  try {
    const sdk: typeof WalletSdk = await loader.ssrLoadModule('/sdk/wallet/src/index.ts');
    const surface: typeof import('../../../apps/wallet/src/surfaces/useSurface.ts') =
      await loader.ssrLoadModule('/apps/wallet/src/surfaces/useSurface.ts');
    const web: typeof import('../../../apps/web/src/api/wallet.ts') = await loader.ssrLoadModule(
      '/apps/web/src/api/wallet.ts'
    );
    const kernel: typeof import('../../../apps/wallet/src/account/KernelPlan.tsx') =
      await loader.ssrLoadModule('/apps/wallet/src/account/KernelPlan.tsx');
    const production: typeof import('../../../services/wallet/gateway/test/e2e/wallet-signing-adapters.production.ts') =
      await loader.ssrLoadModule(
        '/services/wallet/gateway/test/e2e/wallet-signing-adapters.production.ts'
      );
    const owner = privateKeyToAccount(keccak256(toHex('wallet proof owner')));
    const sponsor = privateKeyToAccount(keccak256(toHex('wallet proof sponsor')));
    const outsider = privateKeyToAccount(keccak256(toHex('wallet proof wrong signer')));
    const account = owner.address.toLowerCase() as Hex;
    const paymaster = '0x4444444444444444444444444444444444444444';
    const blockHash = keccak256(toHex('wallet proof block'));
    const user = {
      id: '5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a10',
      aud: 'authenticated',
      role: 'authenticated',
      email: 'proof@example.invalid',
      app_metadata: {},
      user_metadata: {},
      created_at: '1970-01-01T00:00:00Z',
    };
    const token = [
      Buffer.from(wire({ alg: 'HS256', typ: 'JWT' })).toString('base64url'),
      Buffer.from(
        wire({ sub: user.id, aud: 'authenticated', exp: Math.floor(Date.now() / 1000) + 3600 })
      ).toString('base64url'),
      Buffer.alloc(32).toString('base64url'),
    ].join('.');
    const requests: { path: string; body: Json; authorization?: string }[] = [];
    let wrongQuoteSigner = false;
    let wrongSendChain = false;
    let wrongSendSigner = false;
    let base = '';
    const config = {
      chainId: 125n,
      sponsor: sponsor.address.toLowerCase(),
      token: sdk.SIDIORA_TOKEN,
      decimals: 6,
      paymaster,
    };
    const readReceipt = (hash: Hex): SequencerReceiptAnswer | null => {
      const saved = journal.entries.find((entry) => entry.hash === hash);
      if (!saved) return null;
      const transaction = parseTransaction(saved.raw);
      return sdk.decodeSequencerReceipt({
        rung: 'committed',
        txHash: hash,
        nonce: toHex(transaction.nonce ?? 0),
        blockNumber: '0x1',
        blockHash,
        status: '0x1',
        logs: [],
        from: saved.account,
        to: transaction.to,
      });
    };
    const publish = async (tx: Json, signer = owner): Promise<SendTxResponse> => {
      const raw = await signer.signTransaction({
        type: 'eip1559',
        chainId: tx.chainId ?? 125,
        nonce: tx.nonce ?? 0,
        to: tx.to,
        value: BigInt(tx.value ?? 0),
        data: tx.data ?? '0x',
        gas: BigInt(tx.gas ?? 250_000),
        maxFeePerGas: BigInt(tx.maxFeePerGas ?? 1_000_000_000),
        maxPriorityFeePerGas: BigInt(tx.maxPriorityFeePerGas ?? 1),
      });
      const hash = keccak256(raw);
      assert.equal(
        (await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase(),
        signer.address.toLowerCase()
      );
      if (!journal.entries.some((entry) => entry.hash === hash))
        journal.put({ kind: 'transaction', hash, raw, account: signer.address.toLowerCase() });
      return {
        tx_hash: hash,
        address: wrongSendSigner ? outsider.address : signer.address,
        chain_id: wrongSendChain ? 1 : 125,
      };
    };
    const stationReport = (saved: Json) => ({
      tx_hash: saved.hash,
      status: 'pending',
      receipt: null,
      station: {
        state: 'pending',
        deadline: saved.deadline,
        submission: { sponsorNonce: '0', transactionHash: saved.hash },
        replacement: null,
        completion: null,
      },
    });
    const start = async () => {
      server = createServer(async (incoming, outgoing) => {
        const reply = (status: number, body: unknown) => {
          outgoing.writeHead(status, { 'content-type': 'application/json' });
          outgoing.end(wire(body));
        };
        try {
          let text = '';
          for await (const chunk of incoming) text += chunk;
          const body: Json = text ? JSON.parse(text) : {};
          const path = incoming.url!;
          requests.push({ path, body, authorization: incoming.headers.authorization });
          if (path === '/auth/v1/user') return reply(200, user);
          if (path === '/rpc') {
            assert.equal(body.jsonrpc, '2.0');
            let result: unknown;
            switch (body.method) {
              case 'eth_gasPrice':
                result = toHex(1_000_000_000n);
                break;
              case 'eth_getCode':
                result = `0xef0100${paymaster.slice(2)}`;
                break;
              case 'eth_getTransactionCount':
                result = '0x0';
                break;
              case 'eth_call':
                assert.deepEqual(body.params, [
                  { to: account, data: keccak256(toHex('nonce()')).slice(0, 10) },
                  'pending',
                ]);
                result = `0x${'0'.repeat(64)}`;
                break;
              case 'pax_sequencerReceipt':
                result = readReceipt(body.params[0]);
                break;
              case 'px_listAssets':
                result = {
                  height: '1',
                  joined_limit: 64,
                  assets: [
                    {
                      asset_id: sdk.SIDIORA_TOKEN.toLowerCase(),
                      denom: 'usid',
                      kind: 'erc20',
                      name: 'Sidiora',
                      symbol: 'SID',
                      decimals: 6,
                      supply: '1000000',
                    },
                  ],
                };
                break;
              default:
                return reply(200, {
                  jsonrpc: '2.0',
                  id: body.id,
                  error: { code: -32601, message: `unsupported ${body.method}` },
                });
            }
            return reply(200, { jsonrpc: '2.0', id: body.id, result });
          }
          if (path === '/gas-station/quote') {
            const quote = {
              sponsor: config.sponsor,
              token: config.token,
              decimals: 6,
              maxTokenAmount: BigInt(body.maxTokenAmount),
              tokenAmount: 66n,
              deadline: BigInt(Math.floor(Date.now() / 1000) + 600),
              quoteNonce: BigInt(journal.entries.filter((entry) => entry.kind === 'quote').length),
              gasCost: BigInt(body.gasCost),
            };
            const signature = await (wrongQuoteSigner ? outsider : sponsor).sign({
              hash: unwrap(gasQuoteDigest(125n, account, quote)) as Hex,
            });
            journal.put({ kind: 'quote', quote, signature, account });
            return reply(200, { quote, relayerSignature: signature, feeDenom: 'usid' });
          }
          if (incoming.headers.authorization !== `Bearer ${token}`)
            return reply(401, { error: 'not_authenticated' });
          if (path === '/v1/wallet/me')
            return reply(200, {
              wallet: {
                id: user.id,
                address: account,
                chain_id: 125,
                created_at: user.created_at,
                last_used_at: null,
              },
              chain: { id: 125, rpc_url: `${base}/rpc`, explorer_url: null },
            });
          if (path === '/v1/wallet/sponsored/quote') {
            assert.equal(body.account, account);
            const gasCost = BigInt(body.gasLimit) * BigInt(body.gasPrice);
            return reply(200, { gasCost, dailyRemaining: 10n ** 18n - gasCost });
          }
          if (path === '/v1/wallet/sign-digest') {
            const construction = sdk.toWireConstruction(body.construction);
            const digest = sdk.constructionDigest(construction);
            return reply(200, { address: account, signature: await owner.sign({ hash: digest }) });
          }
          if (path === '/v1/wallet/send') return reply(200, await publish(body.tx));
          if (path === '/v1/wallet/sponsored/status') {
            const saved = journal.entries.find(
              (entry) => entry.kind === 'sponsored' && entry.quoteNonce === body.quoteNonce
            );
            if (!saved) return reply(404, { error: 'unknown_quote' });
            if (
              saved.signature !== body.relayerSignature ||
              saved.account !== body.account ||
              saved.sponsor !== body.sponsor
            )
              return reply(403, { error: 'quote_identity_conflict' });
            return reply(200, stationReport(saved));
          }
          if (path === '/v1/wallet/sponsored/submit') {
            const fields = sdk.wireSponsoredBatch(body.construction);
            const batch: SponsoredBatch = {
              chainId: BigInt(fields.chainId),
              account: fields.account,
              nonce: BigInt(fields.nonce),
              calls: fields.calls.map((call) => ({ ...call, value: BigInt(call.value) })),
              quote: {
                ...fields.quote,
                decimals: 6,
                ...Object.fromEntries(
                  Object.entries(fields.quote)
                    .filter(([key]) => !['sponsor', 'token'].includes(key))
                    .map(([key, value]) => [key, BigInt(value)])
                ),
              } as SponsoredBatch['quote'],
            };
            const call = unwrap(
              sponsoredBatchCall(config, batch, body.account_signature, body.relayer_signature)
            );
            assert.equal(call.data, body.data);
            const authorization = body.authorization;
            const authSignature = `${authorization.r}${authorization.s.slice(2)}${(authorization.yParity + 27).toString(16)}`;
            unwrap(
              assembleEip7702Authorization(
                config,
                account,
                BigInt(authorization.nonce),
                authSignature
              )
            );
            const existing = journal.entries.find(
              (entry) => entry.kind === 'sponsored' && entry.quoteNonce === fields.quote.quoteNonce
            );
            if (existing) {
              assert.equal(existing.construction, wire(fields));
              return reply(200, stationReport(existing));
            }
            const sent = await publish({ to: account, data: call.data, value: '0' }, sponsor);
            const saved = {
              kind: 'sponsored',
              hash: sent.tx_hash,
              account,
              sponsor: config.sponsor,
              signature: body.relayer_signature,
              quoteNonce: fields.quote.quoteNonce,
              deadline: fields.quote.deadline,
              construction: wire(fields),
            };
            journal.put(saved);
            return reply(202, stationReport(saved));
          }
          return reply(404, { error: 'unknown_route' });
        } catch (error) {
          producerErrors.push(error);
          reply(500, { error: String(error) });
        }
      });
      server.listen(0, '127.0.0.1');
      await once(server, 'listening');
      const address = server.address();
      assert(address && typeof address === 'object');
      base = `http://127.0.0.1:${address.port}`;
    };
    const stop = async () => {
      if (server) {
        await new Promise<void>((done, reject) =>
          server!.close((error) => (error ? reject(error) : done()))
        );
        server = undefined;
      }
    };
    const makeProvider = (approval?: WalletSdk.PaxeerProviderConfig['confirm']) =>
      production.walletSigningProvider({
        gatewayUrl: base,
        rpcUrl: `${base}/rpc`,
        chainId: 125,
        token: () => token,
        confirm: approval ?? confirm,
      });
    const approvals: WalletSdk.ConfirmRequest[] = [];
    async function confirm(review: WalletSdk.ConfirmRequest): Promise<boolean> {
      assert(Object.isFrozen(review.params));
      approvals.push(review);
      return true;
    }
    await start();
    let provider = makeProvider();
    expect(await provider.request({ method: 'eth_requestAccounts' })).toEqual([account]);
    store = new SubmissionStore(join(temporary, 'submission.jsonl'));
    const stationOptions = () => ({
      chainId: 125n,
      sponsor: config.sponsor,
      paymaster,
      gatewayUrl: base,
      quoteUrl: `${base}/gas-station/quote`,
      accessToken: () => token,
      submissionStore: store!,
    });
    let station = sdk.gasStation(provider, stationOptions());
    const budget = await station.quote({ account, gasLimit: 21_000n, rate: '3114000' });
    expect(budget).toMatchObject({
      gasCost: 21_000_000_000_000n,
      tokenAmount: 66n,
      decimals: 6,
      symbol: 'SID',
    });
    expect(budget.dailyRemaining).toBe(10n ** 18n - budget.gasCost);
    const calls = [sdk.feeToken(provider).setFeeDenom('usid')];
    const request = {
      account,
      did: `did:layerx:${'ab'.repeat(32)}`,
      nonce: await station.batchNonce(account),
      calls,
      maxTokenAmount: 132n,
      gasCost: budget.gasCost,
    };
    const quoted = await station.requestQuote(request);
    const batch: SponsoredBatch = {
      chainId: 125n,
      account,
      nonce: request.nonce,
      calls,
      quote: quoted.quote,
    };
    const webQuote = { batch, relayerSignature: quoted.relayerSignature, paymaster };
    const presentation = web.sidioraQuotePresentation(webQuote);
    assert(presentation, 'apps/web/src/api/wallet.ts must present the SDK quote');
    expect(presentation.amount).toBe('0.000066');
    expect(presentation.identity).toContain(station.digest(batch));
    expect(
      web.sidioraQuotePresentation({ ...webQuote, relayerSignature: 'malformed' })
    ).toBeUndefined();
    expect(web.sidioraQuotePresentation({ ...webQuote, paymaster: 'malformed' })).toBeUndefined();
    wrongQuoteSigner = true;
    await expect(station.requestQuote(request)).rejects.toMatchObject({
      code: 'invalid_signature',
    });
    wrongQuoteSigner = false;
    const priorSubmissions = journal.entries.filter((entry) => entry.kind === 'sponsored').length;
    await expect(
      station.submitFirstUse(batch, quoted.relayerSignature, { confirm: () => false })
    ).rejects.toMatchObject({ code: 'refused', field: 'consent' });
    expect(journal.entries.filter((entry) => entry.kind === 'sponsored')).toHaveLength(
      priorSubmissions
    );
    const hash = await station.submitFirstUse(batch, quoted.relayerSignature, {
      confirm: (consent) => {
        expect(consent.batchDigest).toBe(station.digest(batch));
        return true;
      },
    });
    expect((await station.status(batch, quoted.relayerSignature)).tx_hash).toBe(hash);
    const signaturesBeforeRestart = approvals.length;
    const pending = await station.pending(account);
    expect(pending?.batch).toEqual(batch);
    store.close();
    provider.disconnect();
    await stop();
    journal.close();
    journal = new Journal(join(temporary, 'producer.jsonl'));
    await start();
    store = new SubmissionStore(join(temporary, 'submission.jsonl'));
    provider = makeProvider();
    await provider.request({ method: 'eth_requestAccounts' });
    station = sdk.gasStation(provider, stationOptions());
    const recovered = await station.pending(account);
    expect(recovered).toEqual(pending);
    expect((await station.resume(recovered!.batch, recovered!.relayerSignature)).tx_hash).toBe(
      hash
    );
    expect(await station.submitFirstUse(batch, quoted.relayerSignature)).toBe(hash);
    expect(approvals).toHaveLength(signaturesBeforeRestart);
    expect(journal.entries.filter((entry) => entry.kind === 'sponsored')).toHaveLength(
      priorSubmissions + 1
    );
    const beforeRefusal = requests.length;
    await expect(
      station.status({ ...batch, chainId: 1n }, quoted.relayerSignature)
    ).rejects.toMatchObject({ code: 'chain_mismatch' });
    await expect(
      station.status(
        { ...batch, quote: { ...batch.quote, sponsor: outsider.address } },
        quoted.relayerSignature
      )
    ).rejects.toMatchObject({ code: 'sponsor_mismatch' });
    expect(requests).toHaveLength(beforeRefusal);
    await expect(
      station.status(
        { ...batch, quote: { ...batch.quote, quoteNonce: 999n } },
        quoted.relayerSignature
      )
    ).rejects.toMatchObject({ code: 'refused' });
    const tx = { from: account, to: paymaster, value: '0x1', nonce: '0x1', chainId: '0x7d' };
    const directHash = (await provider.request({
      method: 'eth_sendTransaction',
      params: [tx],
    })) as Hex;
    expect(directHash).toMatch(/^0x[0-9a-f]{64}$/);
    const receipt = await sdk.readSequencerReceipt(
      (method, params) => provider.request({ method, params }),
      directHash
    );
    assert(receipt);
    const shown = surface.surfaceReceipt(receipt, sdk.bridge(provider));
    expect(shown).toEqual({ hash: directHash, rung: 'committed', status: 'confirmed', events: [] });
    expect(() => surface.receiptStatus({ status: 'invalid' })).toThrow('invalid status');
    expect(kernel.journeyLadderSteps({ state: 'done-finalised' })).toEqual([
      { rung: 'committed', source: 'journey', state: 'done-finalised' },
    ]);
    expect(() => kernel.journeyLadderSteps({ state: 'retired' as WalletSdk.JourneyState })).toThrow(
      RangeError
    );
    expect(sdk.statusLadder.highest(sdk.statusLadder.fromSequencerReceipt(receipt))?.rung).toBe(
      'committed'
    );
    expect(Object.isFrozen(sdk.statusLadder.rungs)).toBe(true);
    expect(Reflect.set(sdk.statusLadder.rungs, 0, 'retired')).toBe(false);
    const rejected = production.walletSigningProvider({
      gatewayUrl: base,
      rpcUrl: `${base}/rpc`,
      token: () => token,
      confirm: async () => false,
    });
    await rejected.request({ method: 'eth_requestAccounts' });
    const writesBefore = requests.filter((item) => item.path === '/v1/wallet/send').length;
    const refusal = await rejected
      .request({ method: 'eth_sendTransaction', params: [tx] })
      .catch((error) => error);
    expect(web.walletSendFailure(refusal)).toEqual({ outcome: 'cancelled' });
    expect(requests.filter((item) => item.path === '/v1/wallet/send')).toHaveLength(writesBefore);
    wrongSendSigner = true;
    await expect(provider.request({ method: 'eth_sendTransaction', params: [tx] })).rejects.toThrow(
      'signer'
    );
    wrongSendSigner = false;
    wrongSendChain = true;
    await expect(provider.request({ method: 'eth_sendTransaction', params: [tx] })).rejects.toThrow(
      'chain'
    );
    wrongSendChain = false;
    wallet = new sdk.PaxeerWallet({
      apiUrl: base,
      supabaseUrl: base,
      supabaseAnonKey: 'proof-public-key',
    });
    const session = await wallet.supabase.auth.setSession({
      access_token: token,
      refresh_token: 'proof-refresh',
    });
    expect(session.error).toBeNull();
    const sent: SendTxResponse = await wallet.sendTransaction({
      to: paymaster,
      value: 2n,
      nonce: 2,
      chainId: 125,
    });
    expect(sent.receipt?.txHash).toBe(sent.tx_hash);
    expect(surface.receiptStatus(sent.receipt)).toBe('confirmed');
    wrongSendChain = true;
    await expect(wallet.sendTransaction({ to: paymaster, chainId: 125 })).rejects.toThrow('chain');
    wrongSendChain = false;
    await CompletionAsset(sdk, `${base}/rpc`);
    // sdk/wallet/src/wallet.ts WalletInterface uses the current EVM provider;
    // retired custody methods must continue to be refused before transport.
    const beforeRetired = requests.length;
    await expect(
      provider.request({ method: 'paxeer_prepareCustody', params: [] })
    ).rejects.toMatchObject({ code: 4200 });
    expect(requests).toHaveLength(beforeRetired);
    expect(producerErrors).toEqual([]);
    expect(
      requests.some((item) => item.path === '/rpc' && item.body.method === 'pax_sequencerReceipt')
    ).toBe(true);
    expect(
      requests
        .filter((item) => item.path.startsWith('/v1/wallet/'))
        .every((item) => item.authorization === `Bearer ${token}`)
    ).toBe(true);
  } finally {
    if (wallet) await wallet.supabase.auth.stopAutoRefresh();
    store?.close();
    if (server)
      await new Promise<void>((done, reject) =>
        server!.close((error) => (error ? reject(error) : done()))
      );
    journal.close();
    await loader.close();
    rmSync(temporary, { recursive: true, force: true });
  }
}

async function CompletionAsset(sdk: typeof WalletSdk, url: string) {
  // sdk/wallet/src/endpoint.ts EndpointClient.listBankAssets is the replacement
  // for the removed joined asset contract; the wire carries bank metadata.
  const page = await new sdk.EndpointClient({ url }).listBankAssets();
  expect(page.assets).toHaveLength(1);
  expect(page.assets[0]).toMatchObject({ symbol: 'SID', denom: 'usid', decimals: 6 });
  expect('CompletionAsset' in sdk).toBe(false);
}

test(proof_wallet_sdk_producer_transport_consumer, 60_000);
