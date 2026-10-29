import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import { JsonRpcTransportError } from '../src/rpc.js';
import type { ChainAccount } from '../src/types.js';

async function endpoint_chain_account_decodes_the_paxeer_half_only(): Promise<void> {
  const address = '0x102132435465768798a9bacbdcedfe0f1e2d3c4b';
  const answer = {
    account: {
      evm_address: address,
      pax_address: 'pax1exampleaccount',
      layerx_did: `did:layerx:${'61'.repeat(32)}`,
      layerx_account: '457d8ff18e80b2c0671cb8dd37042120b6bfcfa2f2477389636a6d62e2e64de0',
      bound: true,
    } satisfies ChainAccount['account'],
    paxeer: {
      address,
      balance: '0x123456789abcdef',
      nonce: '0x2a',
      height: 16,
    },
  };
  const expected: ChainAccount = { ...answer, paxeer: { ...answer.paxeer, height: '16' } };
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
    const account: ChainAccount = await client.getChainAccount(address);
    expect(account).toEqual(expected);
    expect(account).not.toHaveProperty('layerx');
    expect(requests).toEqual([{
      method: 'POST',
      body: { jsonrpc: '2.0', id: 1, method: 'px_getAccount', params: [address] },
    }]);

    // Address casing does not change the EVM identity; retain the returned representation.
    const upperAddress = `0x${address.slice(2).toUpperCase()}`;
    result = { ...answer, paxeer: { ...answer.paxeer, address: upperAddress } };
    await expect(client.getChainAccount(address)).resolves.toEqual({
      ...expected, paxeer: { ...expected.paxeer, address: upperAddress },
    });

    result = { ...answer, layerx: { retired: true } };
    await expect(client.getChainAccount(address)).resolves.toEqual(expected);

    result = {
      ...answer,
      paxeer: {
        address,
        balance: '900719925474099312345678901234567890',
        nonce: '9007199254740993',
        height: '9007199254740994',
      },
    };
    await expect(client.getChainAccount(address)).resolves.toEqual(result);

    result = { ...answer, paxeer: { address, balance: '0', nonce: '0', height: '0' } };
    await expect(client.getChainAccount(address)).resolves.toEqual(result);

    const unbound = { ...answer.account, pax_address: null, layerx_did: null, layerx_account: null, bound: false };
    result = { ...answer, account: unbound };
    await expect(client.getChainAccount(address)).resolves.toEqual({ ...expected, account: unbound });

    result = { ...answer, paxeer: { ...answer.paxeer, address: `0x${'cd'.repeat(20)}` } };
    await expect(client.getChainAccount(address)).rejects.toThrow('malformed chain account paxeer address mismatch');

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
      { account: answer.account },
      { ...answer, paxeer: null },
      { ...answer, paxeer: [] },
      { ...answer, paxeer: { ...answer.paxeer, address: '0x1234' } },
    ]) {
      result = invalid;
      await expect(client.getChainAccount(address)).rejects.toThrow(JsonRpcTransportError);
    }
    for (const key of ['address', 'balance', 'nonce'] as const) {
      for (const invalid of [undefined, null, 42, '', {}, []]) {
        result = { ...answer, paxeer: { ...answer.paxeer, [key]: invalid } };
        await expect(client.getChainAccount(address)).rejects.toThrow(JsonRpcTransportError);
      }
    }
    for (const height of [0, 77, Number.MAX_SAFE_INTEGER, '0', '9007199254740994', '18446744073709551615']) {
      result = { ...answer, paxeer: { ...answer.paxeer, height } };
      await expect(client.getChainAccount(address)).resolves.toEqual({
        ...expected, paxeer: { ...expected.paxeer, height: String(height) },
      });
    }
    for (const height of [
      undefined, null, true, false, {}, [], -1, 1.5, Number.MAX_SAFE_INTEGER + 1, Number.MAX_SAFE_INTEGER + 2,
      '', '-1', '1.5', '1e3', '0x10', ' 1', '1 ', '1\n', '01', 'NaN',
    ]) {
      result = { ...answer, paxeer: { ...answer.paxeer, height } };
      await expect(client.getChainAccount(address)).rejects.toThrow('malformed chain account paxeer height');
    }
    expect(requests).toHaveLength(66);
    for (const [index, request] of requests.entries()) {
      expect(request).toEqual({
        method: 'POST',
        body: { jsonrpc: '2.0', id: index + 1, method: 'px_getAccount', params: [address] },
      });
    }
  } finally {
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
}

test(endpoint_chain_account_decodes_the_paxeer_half_only, endpoint_chain_account_decodes_the_paxeer_half_only);
