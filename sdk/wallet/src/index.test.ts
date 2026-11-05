import { Buffer } from 'node:buffer';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { PaxeerWallet } from './index.js';
import type { SendTxResponse, SequencerReceiptAnswer } from './types.js';

const recordedSend = JSON.parse(
  readFileSync(new URL('../test/fixtures/gateway/wallet-send.json', import.meta.url), 'utf8')
) as { body: SendTxResponse };
const recordedReceipts = JSON.parse(
  readFileSync(new URL('../test/fixtures/sequencer-receipt.json', import.meta.url), 'utf8')
) as Record<'submitted' | 'receipted' | 'committed', SequencerReceiptAnswer>;
const sent: SendTxResponse = {
  ...recordedSend.body,
  tx_hash: recordedReceipts.submitted.txHash,
  receipt: recordedReceipts.submitted,
};
const requests: { method: string | undefined; url: string | undefined; body: unknown }[] = [];
const user = {
  id: '5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a10',
  aud: 'authenticated',
  role: 'authenticated',
  email: 'wallet@example.invalid',
  app_metadata: {},
  user_metadata: {},
  created_at: '1970-01-01T00:00:00.000Z',
};
let client: PaxeerWallet;
const server = createServer(async (req, res) => {
  let body = '';
  for await (const chunk of req) body += chunk;
  requests.push({ method: req.method, url: req.url, body: body ? JSON.parse(body) : undefined });
  res.setHeader('content-type', 'application/json');
  if (req.method === 'GET' && req.url === '/auth/v1/user') {
    res.end(JSON.stringify(user));
  } else if (req.method === 'POST' && req.url === '/v1/wallet/send') {
    res.end(JSON.stringify(sent));
  } else {
    res.writeHead(404);
    res.end(JSON.stringify({ error: 'unexpected_request' }));
  }
});

beforeAll(async () => {
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const apiUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  client = new PaxeerWallet({
    apiUrl,
    supabaseUrl: apiUrl,
    supabaseAnonKey: 'synthetic-publishable-key',
  });
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const payload = Buffer.from(
    JSON.stringify({ sub: user.id, exp: Math.floor(Date.now() / 1000) + 3600 })
  ).toString('base64url');
  const { error } = await client.supabase.auth.setSession({
    access_token: `${header}.${payload}.test-signature`,
    refresh_token: 'test-refresh-token',
  });
  expect(error).toBeNull();
});

afterAll(async () => {
  client?.supabase.auth.stopAutoRefresh();
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve()))
  );
});

async function send_transaction_accepts_a_request_without_chain_id() {
  const before = requests.length;
  await expect(client.sendTransaction({ to: sent.address, value: 7n })).resolves.toEqual(sent);
  expect(requests.slice(before)).toEqual([
    {
      method: 'POST',
      url: '/v1/wallet/send',
      body: { tx: { to: sent.address, value: '7' } },
    },
  ]);
}

async function send_transaction_refuses_a_response_for_a_different_chain() {
  const before = requests.length;
  const chainId = sent.chain_id + 1;
  await expect(client.sendTransaction({ to: sent.address, value: 7n, chainId })).rejects.toThrow(
    new TypeError('send response chain differs from the requested chain')
  );
  expect(requests.slice(before)).toEqual([
    {
      method: 'POST',
      url: '/v1/wallet/send',
      body: { tx: { to: sent.address, value: '7', chainId } },
    },
  ]);
}

test(send_transaction_accepts_a_request_without_chain_id);
test(send_transaction_refuses_a_response_for_a_different_chain);
