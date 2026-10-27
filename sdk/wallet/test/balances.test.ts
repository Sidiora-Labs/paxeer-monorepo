import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import type { Balances } from '../src/types.js';

async function endpoint_bank_balances_decode_one_row_per_denom(): Promise<void> {
  const address = `0x${'11'.repeat(20)}`;
  const answer: Balances = {
    account: address,
    height: '9007199254740993',
    balances: [
      { denom: 'apax', amount: '900719925474099312345678901234567890' },
      { denom: 'uusdc', amount: '42' },
      { denom: 'ibc/ABCDEF', amount: '0' },
    ],
  };
  let result: unknown = answer;
  const requests: unknown[] = [];
  const server = createServer(async (req, res) => {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    requests.push({ method: req.method, body });
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ jsonrpc: '2.0', id: body.id, result }));
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const client = new EndpointClient({ url: `http://127.0.0.1:${(server.address() as AddressInfo).port}` });
    const balances: Balances = await client.getBankBalances(address);
    expect(balances).toEqual(answer);
    expect(balances.balances.map((row) => row.denom)).toEqual(['apax', 'uusdc', 'ibc/ABCDEF']);
    expect(requests).toEqual([{
      method: 'POST',
      body: { jsonrpc: '2.0', id: 1, method: 'px_getBalances', params: [address] },
    }]);

    result = { ...answer, balances: [] };
    await expect(client.getBankBalances(address)).resolves.toEqual({ ...answer, balances: [] });

    for (const amount of ['', '-1', '1.5', '1e3', '0x10', ' 1', '1\n', '01', 'NaN', 12, null]) {
      result = { ...answer, balances: [answer.balances[0], { denom: 'uusdc', amount }] };
      await expect(client.getBankBalances(address)).rejects.toThrow(TypeError);
    }
    for (const denom of ['', null, 12]) {
      result = { ...answer, balances: [{ denom, amount: '1' }] };
      await expect(client.getBankBalances(address)).rejects.toThrow(TypeError);
    }
    for (const invalid of [
      null,
      { ...answer, account: null },
      { ...answer, height: 77 },
      { ...answer, height: 'invalid' },
      { ...answer, balances: null },
      { ...answer, balances: [null] },
    ]) {
      result = invalid;
      await expect(client.getBankBalances(address)).rejects.toThrow(TypeError);
    }
    expect(requests).toHaveLength(22);
    for (const [index, request] of requests.entries()) {
      expect(request).toEqual({
        method: 'POST',
        body: { jsonrpc: '2.0', id: index + 1, method: 'px_getBalances', params: [address] },
      });
    }
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

test(endpoint_bank_balances_decode_one_row_per_denom, endpoint_bank_balances_decode_one_row_per_denom);
