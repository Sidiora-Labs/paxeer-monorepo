import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import { JsonRpcTransportError } from '../src/rpc.js';
import type { ChainAccount } from '../src/types.js';

async function endpoint_chain_account_decodes_the_paxeer_half_only(): Promise<void> {
  const address = `0x${'ab'.repeat(20)}`;
  const answer: ChainAccount = {
    account: 'pax1account',
    paxeer: {
      address,
      balance: '900719925474099312345678901234567890',
      nonce: '9007199254740993',
      height: '9007199254740994',
    },
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
    const account: ChainAccount = await client.getChainAccount(address);
    expect(account).toEqual(answer);
    expect(account).not.toHaveProperty('layerx');
    expect(requests).toEqual([{
      method: 'POST',
      body: { jsonrpc: '2.0', id: 1, method: 'px_getAccount', params: [address] },
    }]);

    // Address casing does not change the EVM identity; retain the returned representation.
    result = { ...answer, paxeer: { ...answer.paxeer, address: `0x${'AB'.repeat(20)}` } };
    await expect(client.getChainAccount(address)).resolves.toEqual(result);

    result = { ...answer, layerx: { retired: true } };
    await expect(client.getChainAccount(address)).resolves.toEqual(answer);

    result = { ...answer, paxeer: { address, balance: '0', nonce: '0', height: '0' } };
    await expect(client.getChainAccount(address)).resolves.toEqual(result);

    result = { ...answer, paxeer: { ...answer.paxeer, address: `0x${'cd'.repeat(20)}` } };
    await expect(client.getChainAccount(address)).rejects.toThrow('malformed chain account paxeer address mismatch');

    for (const invalid of [
      null,
      [],
      { ...answer, account: '' },
      { ...answer, account: null },
      { ...answer, account: { evm_address: address } },
      { account: answer.account },
      { ...answer, paxeer: null },
      { ...answer, paxeer: [] },
      { ...answer, paxeer: { ...answer.paxeer, address: '0x1234' } },
    ]) {
      result = invalid;
      await expect(client.getChainAccount(address)).rejects.toThrow(JsonRpcTransportError);
    }
    for (const key of ['address', 'balance', 'nonce', 'height'] as const) {
      for (const invalid of [undefined, null, 42, '', {}, []]) {
        result = { ...answer, paxeer: { ...answer.paxeer, [key]: invalid } };
        await expect(client.getChainAccount(address)).rejects.toThrow(JsonRpcTransportError);
      }
    }
    expect(requests).toHaveLength(38);
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
