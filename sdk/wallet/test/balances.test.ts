import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import type { Balances, BankBalance } from '../src/types.js';

async function endpoint_bank_balances_decode_one_row_per_denom(): Promise<void> {
  const address = '0x102132435465768798a9bacbdcedfe0f1e2d3c4b';
  const vectors = JSON.parse(readFileSync(new URL(
    '../../../services/platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json', import.meta.url,
  ), 'utf8')) as { cases: { name: string; out: { response: BankBalance[] } }[] };
  const bank = vectors.cases.find((entry) => entry.name === 'bank.all_balances');
  expect(bank).toBeDefined();
  const answer = {
    account: {
      evm_address: address,
      pax_address: 'pax1exampleaccount',
      layerx_did: `did:layerx:${'61'.repeat(32)}`,
      layerx_account: '457d8ff18e80b2c0671cb8dd37042120b6bfcfa2f2477389636a6d62e2e64de0',
      bound: true,
    } satisfies Balances['account'],
    height: 16,
    balances: bank!.out.response,
  };
  const expected: Balances = { ...answer, height: '16' };
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
    expect(balances).toEqual(expected);
    expect(balances.balances.map((row) => row.denom)).toEqual(['ulxp', `ibc/${'ab'.repeat(32)}`]);
    expect(requests).toEqual([{
      method: 'POST',
      body: { jsonrpc: '2.0', id: 1, method: 'px_getBalances', params: [address] },
    }]);

    result = { ...answer, balances: [] };
    await expect(client.getBankBalances(address)).resolves.toEqual({ ...expected, balances: [] });

    result = {
      ...answer,
      balances: [
        { denom: 'apax', amount: '900719925474099312345678901234567890' },
        { denom: 'uusdc', amount: '42' },
        { denom: 'ibc/ABCDEF', amount: '0' },
      ],
    };
    await expect(client.getBankBalances(address)).resolves.toEqual({
      ...expected,
      balances: [
        { denom: 'apax', amount: '900719925474099312345678901234567890' },
        { denom: 'uusdc', amount: '42' },
        { denom: 'ibc/ABCDEF', amount: '0' },
      ],
    });

    const unbound = { ...answer.account, pax_address: null, layerx_did: null, layerx_account: null, bound: false };
    result = { ...answer, account: unbound };
    await expect(client.getBankBalances(address)).resolves.toEqual({ ...expected, account: unbound });

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
      [],
      { ...answer, account: '' },
      { ...answer, account: address },
      { ...answer, account: null },
      { ...answer, account: { evm_address: address } },
      { ...answer, account: { ...answer.account, bound: 'true' } },
      { ...answer, account: { ...answer.account, evm_address: '0x1234' } },
      { ...answer, account: { ...answer.account, pax_address: 42 } },
      { ...answer, account: { ...answer.account, layerx_did: 42 } },
      { ...answer, account: { ...answer.account, layerx_account: 42 } },
      { ...answer, height: 'invalid' },
      { ...answer, balances: null },
      { ...answer, balances: [null] },
    ]) {
      result = invalid;
      await expect(client.getBankBalances(address)).rejects.toThrow(TypeError);
    }
    for (const height of [0, 77, Number.MAX_SAFE_INTEGER, '0', '9007199254740993', '18446744073709551615']) {
      result = { ...answer, height };
      await expect(client.getBankBalances(address)).resolves.toEqual({ ...expected, height: String(height) });
    }
    for (const height of [
      undefined, null, true, false, {}, [], -1, 1.5, Number.MAX_SAFE_INTEGER + 1, Number.MAX_SAFE_INTEGER + 2,
      '', '-1', '1.5', '1e3', '0x10', ' 1', '1 ', '1\n', '01', 'NaN',
    ]) {
      result = { ...answer, height };
      await expect(client.getBankBalances(address)).rejects.toThrow('malformed bank balances height');
    }
    expect(requests).toHaveLength(58);
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
