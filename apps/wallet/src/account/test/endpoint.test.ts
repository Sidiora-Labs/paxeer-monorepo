import { readFileSync } from 'node:fs';
import path from 'node:path';
import { expect, test } from 'vitest';
import { OWNER, SCENARIOS, recordings, startAccountEndpoint } from './endpoint';

async function account_test_endpoint_answers_the_merged_reads() {
  const endpoint = await startAccountEndpoint();
  try {
    const vectors: typeof import('../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json') =
      JSON.parse(
        readFileSync(
          path.resolve(
            __dirname,
            '../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'
          ),
          'utf8'
        )
      );
    const vector = vectors.cases.find((entry) => entry.id === 'V1-erc20-approve');
    if (!vector) throw new Error('missing ERC-20 approval conformance vector');
    const calls = [
      { jsonrpc: '2.0', id: 'account', method: 'px_getAccount', params: [OWNER.toUpperCase()] },
      { jsonrpc: '2.0', id: 'balances', method: 'px_getBalances', params: [OWNER] },
      { jsonrpc: '2.0', id: 'assets', method: 'px_listAssets', params: [] },
      {
        jsonrpc: '2.0',
        id: 'nonce',
        method: 'eth_getTransactionCount',
        params: [OWNER, 'pending'],
      },
      {
        jsonrpc: '2.0',
        id: 'receipt',
        method: 'pax_sequencerReceipt',
        params: [vector.receiptJson.txHash],
      },
    ];
    const expected = [
      {
        account: OWNER,
        paxeer: { address: OWNER, balance: '0xde0b6b3a7640000', nonce: '0x3', height: '1000' },
      },
      { account: OWNER, height: '1000', balances: [{ denom: 'ulxp', amount: '125000' }] },
      {
        height: '1000',
        assets: [
          {
            asset_id: 'aa'.repeat(32),
            denom: 'ulxp',
            kind: 'bank',
            name: 'LayerX Points',
            symbol: 'LXP',
            decimals: 6,
            supply: '1000000000000',
          },
          {
            asset_id: 'bb'.repeat(32),
            denom: 'usid',
            kind: 'bank',
            name: 'Sidiora',
            symbol: 'SID',
            decimals: 6,
            supply: '1000000000000',
          },
        ],
        joined_limit: 16,
      },
      '0x4',
      vector.receiptJson,
    ];
    for (const [index, call] of calls.entries()) {
      const response = await fetch(endpoint.config('merged').endpointUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(call),
      });
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual({
        jsonrpc: '2.0',
        id: call.id,
        result: expected[index],
      });
    }
    expect(vector.receiptJson.rung).toBe('receipted');
    expect(vector.receiptJson.receipt).toBe(vector.signedReceipt);
    expect(vector.receiptJson.logs).toHaveLength(1);
    expect(endpoint.methods('merged')).toEqual(calls.map((call) => call.method));
    expect(endpoint.unmatched).toEqual([]);

    const batch = await fetch(endpoint.config('merged').endpointUrl, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(calls),
    });
    expect(batch.status).toBe(200);
    expect(await batch.json()).toEqual(
      calls.map((call, index) => ({ jsonrpc: '2.0', id: call.id, result: expected[index] }))
    );

    for (const scenario of SCENARIOS) {
      const legacy = recordings(scenario).filter(
        (entry) => scenario !== 'merged' || entry.request.method.startsWith('lx_')
      );
      const response = await fetch(endpoint.config(scenario).endpointUrl, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify(legacy.map((entry, id) => ({ jsonrpc: '2.0', id, ...entry.request }))),
      });
      expect(response.status).toBe(200);
      expect(await response.json()).toEqual(
        legacy.map((entry, id) => ({ jsonrpc: '2.0', id, ...entry.response }))
      );
    }
    expect(endpoint.unmatched).toEqual([]);
    expect(endpoint.refusals).toEqual([]);

    const invalid = [
      { jsonrpc: '2.0', id: 1, method: 'eth_getTransactionCount', params: [OWNER, 'latest'] },
      { jsonrpc: '2.0', id: 2, method: 'eth_getTransactionCount', params: [OWNER] },
      { jsonrpc: '2.0', id: 3, method: 'pax_sequencerReceipt', params: [`0x${'ff'.repeat(32)}`] },
      { jsonrpc: '2.0', id: 4, method: 'px_getBalances', params: [`0x${'22'.repeat(20)}`] },
      { jsonrpc: '2.0', id: 5, method: 'px_listAssets', params: [OWNER] },
    ];
    const refused = await fetch(endpoint.config('merged').endpointUrl, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(invalid),
    });
    expect(refused.status).toBe(200);
    expect(await refused.json()).toEqual(
      invalid.map((call) => ({
        jsonrpc: '2.0',
        id: call.id,
        error: { code: -32601, message: 'Method not found' },
      }))
    );
    expect(endpoint.unmatched).toEqual(
      invalid.map((call) => `merged ${call.method} ${JSON.stringify(call.params)}`)
    );
  } finally {
    await endpoint.close();
  }
}

test(account_test_endpoint_answers_the_merged_reads);
