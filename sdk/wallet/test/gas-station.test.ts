import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';

import { PaxeerProvider } from '../src/provider.js';
import { ModuleError } from '../src/modules/index.js';
import {
  GasStationError,
  gasStation,
  type SponsoredBatch,
  type SponsoredSubmissionEvidence,
} from '../src/modules/gas-station.js';
import type { SequencerReceiptAnswer } from '../src/types.js';

const account = '0x1111111111111111111111111111111111111111';
const sponsor = '0x2222222222222222222222222222222222222222';
const paymaster = '0x3333333333333333333333333333333333333333';
const accessToken = 'gas-budget-test-token';

async function quoteExchange(status: number, answer: unknown) {
  const requests: { method: string; path: string; authorization: string | undefined; body: unknown }[] = [];
  const rpcMethods: string[] = [];
  const server = createServer(async (request, response) => {
    let text = '';
    for await (const chunk of request) text += chunk;
    const body: unknown = text ? JSON.parse(text) : null;
    response.setHeader('content-type', 'application/json');
    if (request.url === '/v1/wallet/me' && request.method === 'GET') {
      response.end(JSON.stringify({ wallet: { address: account, chain_id: 125 }, chain: { id: 125 } }));
      return;
    }
    if (request.url === '/rpc' && request.method === 'POST') {
      const rpc = body as { id: number; method: string };
      rpcMethods.push(rpc.method);
      if (rpc.method === 'eth_gasPrice') {
        response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result: '0x7b' }));
        return;
      }
    }
    requests.push({
      method: request.method ?? '',
      path: request.url ?? '',
      authorization: request.headers.authorization,
      body,
    });
    if (request.method === 'POST' && request.url === '/v1/wallet/sponsored/quote' &&
      request.headers.authorization === `Bearer ${accessToken}`) {
      response.writeHead(status);
      response.end(JSON.stringify(answer));
      return;
    }
    response.writeHead(404);
    response.end(JSON.stringify({ error: 'unknown_route' }));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const gatewayUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const provider = new PaxeerProvider({ gatewayUrl, rpcUrl: `${gatewayUrl}/rpc`, token: () => accessToken });
  const station = gasStation(provider, {
    chainId: 125n,
    sponsor,
    paymaster,
    gatewayUrl: `${gatewayUrl}/`,
    accessToken: () => accessToken,
  });
  return {
    provider,
    station,
    requests,
    rpcMethods,
    close: () => new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

async function gas_station_quote_posts_the_budget_to_the_sponsored_quote_route() {
  const exchange = await quoteExchange(200, { gasCost: '21000000', dailyRemaining: '5' });
  try {
    await exchange.provider.request({ method: 'eth_requestAccounts' });
    const quote = await exchange.station.quote({ account, gasLimit: 21_000n, rate: '1000000000000000000' });

    expect(exchange.requests).toEqual([{
      method: 'POST',
      path: '/v1/wallet/sponsored/quote',
      authorization: `Bearer ${accessToken}`,
      body: { account, gasLimit: '21000', gasPrice: '123', rate: '1000000000000000000' },
    }]);
    expect(exchange.rpcMethods).toEqual(['eth_gasPrice']);
    expect(quote).toEqual({
      gasLimit: 21_000n,
      gasPrice: 123n,
      gasCost: 21_000_000n,
      dailyRemaining: 5n,
      rate: '1000000000000000000',
      tokenAmount: 21_000_000n,
      token: exchange.station.config.token,
      symbol: 'SID',
      decimals: exchange.station.config.decimals,
    });
    expect(quote.gasCost).not.toBe(quote.gasLimit * quote.gasPrice);
  } finally {
    await exchange.close();
  }
}

async function gas_station_quote_reports_an_exhausted_daily_budget() {
  const exchange = await quoteExchange(429, { error: 'budget_exhausted' });
  try {
    await exchange.provider.request({ method: 'eth_requestAccounts' });
    const quote = exchange.station.quote({ account, gasLimit: 21_000n, gasPrice: 1_000n, rate: '3114000' });

    await expect(quote).rejects.toBeInstanceOf(ModuleError);
    await expect(quote).rejects.toBeInstanceOf(GasStationError);
    await expect(quote).rejects.toMatchObject({
      code: 'budget_exhausted',
      refusal: { code: 'budget_exhausted', field: 'dailyRemaining' },
    });
    expect(exchange.requests).toEqual([{
      method: 'POST',
      path: '/v1/wallet/sponsored/quote',
      authorization: `Bearer ${accessToken}`,
      body: { account, gasLimit: '21000', gasPrice: '1000', rate: '3114000' },
    }]);
    expect(exchange.rpcMethods).toEqual([]);
  } finally {
    await exchange.close();
  }
}

async function gas_station_status_evidence_carries_the_matching_sequencer_receipt() {
  const recorded = JSON.parse(
    readFileSync(new URL('./fixtures/sequencer-receipt.json', import.meta.url), 'utf8'),
  ) as Record<'submitted' | 'receipted' | 'committed', SequencerReceiptAnswer>;
  const txHash = recorded.committed.txHash;
  const relayerSignature = `0x${'11'.repeat(65)}`;
  const batch: SponsoredBatch = {
    chainId: 125n,
    account,
    nonce: 0n,
    calls: [{ to: paymaster, value: 0n, data: '0x' }],
    quote: {
      sponsor,
      token: '0x4444444444444444444444444444444444444444',
      maxTokenAmount: 10n,
      tokenAmount: 1n,
      deadline: 2_000_000_000n,
      quoteNonce: 7n,
      gasCost: 21_000n,
      decimals: 6,
    },
  };
  const pendingKey = `paxeer:sponsored:pending:v1:125:${account}`;
  const stored = new Map<string, string>();
  const requests: { path: string; authorization: string | undefined; body: unknown }[] = [];
  let status: SponsoredSubmissionEvidence['status'] = 'confirmed';
  let receipt: SequencerReceiptAnswer | null = recorded.committed;
  let terminalHash: typeof txHash | null = txHash;
  let report: Omit<SponsoredSubmissionEvidence, 'receipt'>;
  const server = createServer(async (request, response) => {
    let text = '';
    for await (const chunk of request) text += chunk;
    const body: unknown = text ? JSON.parse(text) : null;
    response.setHeader('content-type', 'application/json');
    if (request.method === 'GET' && request.url === '/v1/wallet/me') {
      response.end(JSON.stringify({ wallet: { address: account, chain_id: 125 }, chain: { id: 125 } }));
      return;
    }
    requests.push({ path: request.url ?? '', authorization: request.headers.authorization, body });
    if (request.method === 'POST' && request.url === '/v1/wallet/sponsored/status' &&
      request.headers.authorization === `Bearer ${accessToken}`) {
      response.end(JSON.stringify(report));
      return;
    }
    if (request.method === 'POST' && request.url === '/rpc') {
      const rpc = body as { id: number; method: string; params: unknown[] };
      if (rpc.method === 'pax_sequencerReceipt' && rpc.params.length === 1 && rpc.params[0] === txHash) {
        response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, result: receipt }));
        return;
      }
      response.end(JSON.stringify({ jsonrpc: '2.0', id: rpc.id, error: { code: -32602, message: 'unexpected receipt request' } }));
      return;
    }
    response.writeHead(404);
    response.end(JSON.stringify({ error: 'unknown_route' }));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const gatewayUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  const provider = new PaxeerProvider({ gatewayUrl, rpcUrl: `${gatewayUrl}/rpc`, token: () => accessToken });
  const station = gasStation(provider, {
    chainId: 125n,
    sponsor,
    paymaster,
    gatewayUrl,
    accessToken: () => accessToken,
    submissionStore: {
      getItem: key => stored.get(key) ?? null,
      setItem: (key, value) => { stored.set(key, value); },
    },
  });
  const receipted: SequencerReceiptAnswer = { ...recorded.committed, rung: 'receipted' };
  const reverted: SequencerReceiptAnswer = { ...recorded.committed, status: '0x0', vmError: 'execution reverted' };
  const scenarios: {
    status: SponsoredSubmissionEvidence['status'];
    receipt: SequencerReceiptAnswer | null;
    error?: 'receipt' | 'tx_hash' | 'hash';
    nullHash?: boolean;
  }[] = [
    { status: 'confirmed', receipt: recorded.committed },
    { status: 'confirmed', receipt: receipted },
    { status: 'reverted', receipt: reverted },
    { status: 'reverted', receipt: { ...reverted, rung: 'receipted' } },
    { status: 'confirmed', receipt: reverted, error: 'receipt' },
    { status: 'reverted', receipt: recorded.committed, error: 'receipt' },
    { status: 'confirmed', receipt: null, error: 'receipt' },
    { status: 'reverted', receipt: null, error: 'receipt' },
    { status: 'confirmed', receipt: recorded.submitted, error: 'receipt' },
    { status: 'confirmed', receipt: { ...recorded.committed, status: undefined }, error: 'receipt' },
    { status: 'confirmed', receipt: { ...recorded.committed, status: '0x2' }, error: 'receipt' },
    { status: 'confirmed', receipt: { ...recorded.committed, txHash: `0x${'ff'.repeat(32)}` }, error: 'hash' },
    { status: 'confirmed', receipt: recorded.committed, error: 'tx_hash', nullHash: true },
    { status: 'pending', receipt: recorded.committed },
    { status: 'cancelled', receipt: recorded.committed },
  ];
  try {
    await provider.request({ method: 'eth_requestAccounts' });
    for (const scenario of scenarios) {
      status = scenario.status;
      receipt = scenario.receipt;
      terminalHash = scenario.nullHash ? null : txHash;
      const terminal = status === 'confirmed' || status === 'reverted';
      report = {
        tx_hash: terminalHash,
        status,
        station: status === 'pending' ? null : {
          state: 'completed',
          deadline: '2000000000',
          submission: { sponsorNonce: '0', transactionHash: txHash },
          replacement: null,
          completion: {
            outcome: status === 'confirmed' ? 'included' : status,
            transactionHash: terminalHash,
            blockNumber: '1000',
            sidCollected: '1',
            paxSpent: '21000',
          },
        },
      };
      stored.set(pendingKey, 'retained-submission');
      requests.length = 0;
      const result = station.status(batch, relayerSignature);
      if (scenario.error === 'hash') {
        await expect(result).rejects.toThrow('sequencer receipt txHash differs from the asked hash');
      } else if (scenario.error) {
        await expect(result).rejects.toBeInstanceOf(ModuleError);
        await expect(result).rejects.toMatchObject({ code: 'invalid_answer', field: scenario.error });
      } else {
        const evidence = await result;
        expect(evidence).toEqual({ ...report, receipt: terminal ? receipt : null });
        expect(Object.isFrozen(evidence)).toBe(true);
      }
      expect(stored.get(pendingKey)).toBe(scenario.error || status === 'pending' ? 'retained-submission' : 'null');
      expect(requests[0]).toEqual({
        path: '/v1/wallet/sponsored/status',
        authorization: `Bearer ${accessToken}`,
        body: { account, sponsor, quoteNonce: '7', relayerSignature },
      });
      if (terminal && !scenario.nullHash) {
        expect(requests).toHaveLength(2);
        expect(requests[1]).toMatchObject({
          path: '/rpc',
          body: { jsonrpc: '2.0', method: 'pax_sequencerReceipt', params: [txHash] },
        });
      } else {
        expect(requests).toHaveLength(1);
      }
    }
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close(error => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  }
}

test(gas_station_quote_posts_the_budget_to_the_sponsored_quote_route);
test(gas_station_quote_reports_an_exhausted_daily_budget);
test(gas_station_status_evidence_carries_the_matching_sequencer_receipt);
