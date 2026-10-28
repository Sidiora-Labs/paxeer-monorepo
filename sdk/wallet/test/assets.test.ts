import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import { JsonRpcTransportError } from '../src/rpc.js';
import type { AssetPage } from '../src/types.js';

async function endpoint_bank_assets_decode_the_bank_asset_page(): Promise<void> {
  const answer: AssetPage = {
    height: '9007199254740993',
    assets: [
      { asset_id: 'apax', denom: 'apax', kind: 'native', name: 'Paxeer', symbol: 'PAX', decimals: 18, supply: '900719925474099312345678901234567890' },
      { asset_id: 'uusdc', denom: 'uusdc', kind: 'bank', name: 'USD Coin', symbol: 'USDC', decimals: 6, supply: '42' },
      { asset_id: 'ibc/ABCDEF', denom: 'ibc/ABCDEF', kind: 'ibc', name: 'IBC Asset', symbol: 'IBC', decimals: 0, supply: '0' },
    ],
    joined_limit: 128,
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
    const page: AssetPage = await client.listBankAssets();
    expect(page).toEqual(answer);
    expect(requests).toEqual([{
      method: 'POST',
      body: { jsonrpc: '2.0', id: 1, method: 'px_listAssets', params: [] },
    }]);

    result = { height: '0', assets: [], joined_limit: 1 };
    await expect(client.listBankAssets()).resolves.toEqual(result);

    for (const decimals of [0, 36]) {
      result = { ...answer, assets: [{ ...answer.assets[0], decimals }] };
      await expect(client.listBankAssets()).resolves.toEqual(result);
    }
    for (const decimals of [-1, 37, 1.5, '18', null, undefined]) {
      result = { ...answer, assets: [answer.assets[0], { ...answer.assets[1], decimals }] };
      await expect(client.listBankAssets()).rejects.toThrow('malformed bank asset decimals');
    }
    for (const key of ['asset_id', 'denom', 'kind', 'name', 'symbol'] as const) {
      for (const invalid of [undefined, null, 42, {}, []]) {
        result = { ...answer, assets: [{ ...answer.assets[0], [key]: invalid }] };
        await expect(client.listBankAssets()).rejects.toThrow(`malformed bank asset ${key}`);
      }
    }
    for (const invalid of ['', '-1', '1.5', '1e3', '0x10', ' 1', '1\n', '01', 12, null, undefined]) {
      result = { ...answer, height: invalid };
      await expect(client.listBankAssets()).rejects.toThrow('malformed bank asset page height');
      result = { ...answer, assets: [{ ...answer.assets[0], supply: invalid }] };
      await expect(client.listBankAssets()).rejects.toThrow('malformed bank asset supply');
    }
    for (const joined_limit of [0, -1, 1.5, '128', null, undefined]) {
      result = { ...answer, joined_limit };
      await expect(client.listBankAssets()).rejects.toThrow('malformed joined_limit');
    }
    for (const invalid of [
      null,
      [],
      {},
      { ...answer, assets: null },
      { ...answer, assets: {} },
      { ...answer, assets: [null] },
      { ...answer, assets: [[]] },
      { ...answer, assets: [{}] },
      { assets: [], joined_limit: answer.joined_limit },
      { assets: [{ asset_id: 'legacy', layerx: {}, paxeer: null }], joined_limit: answer.joined_limit },
    ]) {
      result = invalid;
      await expect(client.listBankAssets()).rejects.toThrow(JsonRpcTransportError);
    }
    expect(requests).toHaveLength(73);
    for (const [index, request] of requests.entries()) {
      expect(request).toEqual({
        method: 'POST',
        body: { jsonrpc: '2.0', id: index + 1, method: 'px_listAssets', params: [] },
      });
    }
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

test(endpoint_bank_assets_decode_the_bank_asset_page, endpoint_bank_assets_decode_the_bank_asset_page);
