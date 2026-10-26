import { readFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it, test } from 'vitest';
import {
  PaxeerWallet,
  PaxeerWalletError,
  type SendTxResponse,
  type SequencerReceiptAnswer,
} from '../src/index.js';

type Seen = { method: string; url: string; authorization: string | undefined };

const seen: Seen[] = [];
let server: Server;
let apiUrl: string;
const recordedReceipts = JSON.parse(
  readFileSync(new URL('./fixtures/sequencer-receipt.json', import.meta.url), 'utf8'),
) as Record<'submitted' | 'receipted' | 'committed', SequencerReceiptAnswer>;
const recordedSend = JSON.parse(
  readFileSync(new URL('./fixtures/gateway/wallet-send.json', import.meta.url), 'utf8'),
) as { body: SendTxResponse };
const recordedWallet = JSON.parse(
  readFileSync(new URL('./fixtures/gateway/wallet-me.json', import.meta.url), 'utf8'),
) as { body: Awaited<ReturnType<PaxeerWallet['getWallet']>> };
const sent = { ...recordedSend.body, tx_hash: recordedReceipts.submitted.txHash };
const receiptRequests: { url: string; body: unknown; authorization: string | undefined }[] = [];
const user = {
  id: '5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a10',
  aud: 'authenticated',
  role: 'authenticated',
  email: 'wallet@example.invalid',
  app_metadata: {},
  user_metadata: {},
  created_at: '1970-01-01T00:00:00.000Z',
};

function receiptFor(scenario: string): SequencerReceiptAnswer | null {
  if (scenario === 'missing') return null;
  if (scenario.endsWith('foreign')) return { ...recordedReceipts.submitted, txHash: `0x${'ff'.repeat(32)}` };
  if (scenario === 'submitted' || scenario === 'inline') return recordedReceipts.submitted;
  return { ...recordedReceipts.committed, rung: scenario === 'receipted' ? 'receipted' : 'committed' };
}

const tiers = {
  tiers: [
    {
      tier_id: 'starter_25k',
      whitelist: [{ contract: '0x1111111111111111111111111111111111111111', selector: '0x095ea7b3' }],
    },
  ],
};

async function handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
  seen.push({ method: req.method ?? '', url: req.url ?? '', authorization: req.headers.authorization });
  if (req.method === 'GET' && req.url === '/auth/v1/user') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(user));
    return;
  }
  const receiptRoute = /^\/receipt\/([^/]+)\/(v1\/wallet\/(?:send|me)|rpc)$/.exec(req.url ?? '');
  if (receiptRoute) {
    const [, scenario, route] = receiptRoute;
    let text = '';
    for await (const chunk of req) text += chunk;
    const body = text ? JSON.parse(text) : undefined;
    receiptRequests.push({ url: req.url!, body, authorization: req.headers.authorization });
    res.writeHead(200, { 'content-type': 'application/json' });
    if (req.method === 'POST' && route === 'v1/wallet/send') {
      res.end(JSON.stringify(scenario!.startsWith('inline') ? { ...sent, receipt: receiptFor(scenario!) } : sent));
    } else if (req.method === 'GET' && route === 'v1/wallet/me') {
      res.end(JSON.stringify({
        ...recordedWallet.body,
        chain: { ...recordedWallet.body.chain, rpc_url: `${apiUrl}/receipt/${scenario}/rpc` },
      }));
    } else if (req.method === 'POST' && route === 'rpc') {
      res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result: receiptFor(scenario!) }));
    } else {
      res.end(JSON.stringify({ error: 'unexpected_request' }));
    }
    return;
  }
  if (req.method === 'GET' && req.url === '/v1/funded/tiers') {
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(tiers));
    return;
  }
  if (req.url === '/broken/v1/funded/tiers') {
    res.writeHead(502, { 'content-type': 'text/plain' });
    res.end('upstream unavailable');
    return;
  }
  res.writeHead(403, { 'content-type': 'application/json' });
  res.end(JSON.stringify({ error: 'CONTRACT_NOT_WHITELISTED', message: 'contract is not whitelisted' }));
}

beforeAll(async () => {
  server = createServer(handle);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  apiUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) => server.close((err) => (err ? reject(err) : resolve())));
});

function wallet(base: string): PaxeerWallet {
  return new PaxeerWallet({
    apiUrl: base,
    supabaseUrl: 'https://project.supabase.invalid',
    supabaseAnonKey: 'synthetic-publishable-key',
  });
}

async function receiptWallet(scenario: string): Promise<{ client: PaxeerWallet; token: string }> {
  const client = new PaxeerWallet({
    apiUrl: `${apiUrl}/receipt/${scenario}`,
    supabaseUrl: apiUrl,
    supabaseAnonKey: 'synthetic-publishable-key',
  });
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const payload = Buffer.from(JSON.stringify({
    sub: user.id, exp: Math.floor(Date.now() / 1000) + 3600,
  })).toString('base64url');
  const token = `${header}.${payload}.test-signature`;
  const { error } = await client.supabase.auth.setSession({ access_token: token, refresh_token: 'test-refresh-token' });
  expect(error).toBeNull();
  return { client, token };
}

async function paxeer_wallet_send_transaction_returns_the_sequencer_receipt() {
  for (const scenario of ['submitted', 'receipted', 'committed', 'inline', 'missing']) {
    const { client, token } = await receiptWallet(scenario);
    const before = receiptRequests.length;
    const result = await client.sendTransaction({ to: sent.address, value: 7n, chainId: sent.chain_id });
    const receipt = receiptFor(scenario);
    expect(result).toEqual(receipt ? { ...sent, receipt } : sent);
    const prefix = `/receipt/${scenario}`;
    expect(receiptRequests.slice(before)).toEqual([
      {
        url: `${prefix}/v1/wallet/send`,
        body: { tx: { to: sent.address, value: '7', chainId: sent.chain_id } },
        authorization: `Bearer ${token}`,
      },
      ...(scenario === 'inline' ? [] : [
        { url: `${prefix}/v1/wallet/me`, body: undefined, authorization: `Bearer ${token}` },
        {
          url: `${prefix}/rpc`,
          body: { jsonrpc: '2.0', id: 1, method: 'pax_sequencerReceipt', params: [sent.tx_hash] },
          authorization: undefined,
        },
      ]),
    ]);
  }
}

async function paxeer_wallet_send_transaction_refuses_a_receipt_for_another_hash() {
  for (const scenario of ['foreign', 'inline-foreign']) {
    const { client } = await receiptWallet(scenario);
    const before = receiptRequests.length;
    await expect(client.sendTransaction({ to: sent.address, value: '7' }))
      .rejects.toThrow(new TypeError('receipt does not match the sent transaction'));
    expect(receiptRequests.slice(before).map((request) => request.url)).toEqual(
      scenario === 'inline-foreign'
        ? [`/receipt/${scenario}/v1/wallet/send`]
        : [
          `/receipt/${scenario}/v1/wallet/send`,
          `/receipt/${scenario}/v1/wallet/me`,
          `/receipt/${scenario}/rpc`,
        ],
    );
  }
}

test(paxeer_wallet_send_transaction_returns_the_sequencer_receipt);
test(paxeer_wallet_send_transaction_refuses_a_receipt_for_another_hash);

describe('PaxeerWallet', () => {
  it('rejects a configuration missing any required field', () => {
    const base = { apiUrl: 'https://wallet.invalid', supabaseUrl: 'https://project.supabase.invalid', supabaseAnonKey: 'k' };
    expect(() => new PaxeerWallet({ ...base, apiUrl: '' })).toThrow('apiUrl required');
    expect(() => new PaxeerWallet({ ...base, supabaseUrl: '' })).toThrow('supabaseUrl required');
    expect(() => new PaxeerWallet({ ...base, supabaseAnonKey: '' })).toThrow('supabaseAnonKey required');
  });

  it('reads a public route without a bearer header and strips a trailing slash from the base', async () => {
    const result = await wallet(`${apiUrl}/`).listFundedTiers();
    expect(result).toEqual(tiers);
    const last = seen[seen.length - 1];
    expect(last).toEqual({ method: 'GET', url: '/v1/funded/tiers', authorization: undefined });
  });

  it('refuses an authenticated call without a session before any request is sent', async () => {
    const before = seen.length;
    const err = await wallet(apiUrl).signMessage('hello').catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PaxeerWalletError);
    expect(err).toMatchObject({ code: 'NO_SESSION', status: 401, message: 'not_authenticated' });
    expect(seen.length).toBe(before);
  });

  it('maps a structured error body to the error code, message, status and detail', async () => {
    const err = await wallet(`${apiUrl}/denied`).listFundedTiers().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PaxeerWalletError);
    expect(err).toMatchObject({
      code: 'CONTRACT_NOT_WHITELISTED',
      message: 'contract is not whitelisted',
      status: 403,
      detail: { error: 'CONTRACT_NOT_WHITELISTED', message: 'contract is not whitelisted' },
    });
  });

  it('maps a non-JSON error body to an HTTP status code', async () => {
    const err = await wallet(`${apiUrl}/broken`).listFundedTiers().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(PaxeerWalletError);
    expect(err).toMatchObject({ code: 'HTTP_502', message: 'request failed: 502', status: 502, detail: null });
  });
});
