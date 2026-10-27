import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';

import { PaxeerProvider } from '../src/provider.js';
import { ModuleError } from '../src/modules/index.js';
import { GasStationError, gasStation } from '../src/modules/gas-station.js';

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

test(gas_station_quote_posts_the_budget_to_the_sponsored_quote_route);
test(gas_station_quote_reports_an_exhausted_daily_budget);
