import { createServer } from 'node:http';
import { expect, test } from 'vitest';
import { env } from '../src/env.js';
import { GasStationClient, StationAdapterError } from '../src/sponsor/station.js';

async function stationClientRefusesUnconfiguredUrl(): Promise<void> {
  const configured = env.WALLET_GAS_STATION_URL;
  delete env.WALLET_GAS_STATION_URL;
  try {
    const client = new GasStationClient();
    for (const method of ['quote', 'submit', 'status'] as const) {
      const result = client[method]({ account: `0x${'11'.repeat(20)}` });
      await expect(result).rejects.toBeInstanceOf(StationAdapterError);
      await expect(result).rejects.toMatchObject({
        status: 503,
        code: 'gas_station_unconfigured',
      });
    }
  } finally {
    if (configured === undefined) delete env.WALLET_GAS_STATION_URL;
    else env.WALLET_GAS_STATION_URL = configured;
  }
}

async function stationClientRelaysStatusBody(): Promise<void> {
  const body = {
    account: `0x${'11'.repeat(20)}`,
    sponsor: `0x${'22'.repeat(20)}`,
    quoteNonce: '7',
    relayerSignature: `0x${'33'.repeat(65)}`,
  };
  const answer = { status: 'pending' };
  const received: {
    method: string | undefined;
    path: string | undefined;
    contentType: string | undefined;
    body: unknown;
  }[] = [];
  const server = createServer((req, res) => {
    const chunks: Buffer[] = [];
    req.on('data', (chunk: Buffer) => chunks.push(chunk));
    req.on('end', () => {
      received.push({
        method: req.method,
        path: req.url,
        contentType: req.headers['content-type'],
        body: JSON.parse(Buffer.concat(chunks).toString('utf8')),
      });
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(answer));
    });
  });
  try {
    await new Promise<void>((resolve, reject) => {
      server.once('error', reject);
      server.listen(0, '127.0.0.1', resolve);
    });
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('station did not bind');
    const client = new GasStationClient(`http://127.0.0.1:${address.port}`);
    await expect(client.status(body)).resolves.toStrictEqual(answer);
    expect(received).toStrictEqual([
      { method: 'POST', path: '/status', contentType: 'application/json', body },
    ]);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}

test(stationClientRefusesUnconfiguredUrl, stationClientRefusesUnconfiguredUrl);
test(stationClientRelaysStatusBody, stationClientRelaysStatusBody);
