import { expect, test } from 'vitest';

import { parseChainsResponse, rpcUrl, sortChains, statusLabel } from './model';

function chains_model_parses_sorts_and_labels() {
  const json = JSON.parse(`{"chains":[{"id":1,"name":"demo","operator":"0x1111111111111111111111111111111111111111","status":"active","chain_id":125000001,"created_height":42,"bond":"100000000000000000000","gas_allowance":500000,"byte_allowance":65536,"fee_share_bps":100}]}`);
  const parsed = parseChainsResponse(json);
  expect({ chains: parsed }).toEqual(json);
  expect(parsed[0].bond).toBe('100000000000000000000');
  expect(parsed[0].gas_allowance).toBe(500000);
  expect(parsed[0].byte_allowance).toBe(65536);
  expect(parsed[0].created_height).toBe(42);

  const first = parsed[0];
  const input = [
    { ...first, id: 4, chain_id: 125000004, name: 'four', status: 'active' },
    { ...first, id: 3, chain_id: 125000003, name: 'three', status: 'retired' },
    { ...first, id: 2, chain_id: 125000002, name: 'two', status: 'governance_paused' },
    { ...first, status: 'paused' },
    { ...first, id: 5, chain_id: 125000005, name: 'five', status: 'active' },
  ];
  const chains = parseChainsResponse({ chains: input });
  expect(chains.map(({ id }) => id)).toEqual([ 1, 2, 3, 4, 5 ]);
  expect(input.map(({ id }) => id)).toEqual([ 4, 3, 2, 1, 5 ]);
  const rows = sortChains(chains);
  expect(rows.map(({ id }) => id)).toEqual([ 4, 5, 1, 2, 3 ]);
  expect(rows.map(({ status }) => statusLabel(status))).toEqual([
    'Active', 'Active', 'Paused', 'Paused by governance', 'Retired',
  ]);
  expect(chains.map(({ id }) => id)).toEqual([ 1, 2, 3, 4, 5 ]);
  expect(sortChains([])).toEqual([]);
  expect(parseChainsResponse({ chains: [] })).toEqual([]);

  for (const invalid of [ null, [], {}, { chains: null }, { chains: [ null ] }, { chains: [ {} ] } ]) {
    expect(() => parseChainsResponse(invalid)).toThrow();
  }
  for (const invalid of [
    { id: 0 }, { id: 1.5 }, { id: 0x100000000 }, { name: '' }, { name: '-demo' },
    { operator: '0x123' }, { status: 'unknown' }, { chain_id: 125 }, { created_height: -1 },
    { bond: 100 }, { bond: '-1' }, { gas_allowance: '500000' }, { gas_allowance: Number.MAX_SAFE_INTEGER + 1 },
    { byte_allowance: 1.5 }, { fee_share_bps: 10001 },
  ]) {
    expect(() => parseChainsResponse({ chains: [ { ...first, ...invalid } ] })).toThrow();
  }
  for (const field of Object.keys(first)) {
    const incomplete = { ...first };
    Reflect.deleteProperty(incomplete, field);
    expect(() => parseChainsResponse({ chains: [ incomplete ] })).toThrow();
  }
  expect(() => parseChainsResponse({ chains: [ first, first ] })).toThrow();
  expect(() => parseChainsResponse({ chains: [ first, { ...first, id: 2, chain_id: 125000002 } ] })).toThrow();

  expect(rpcUrl('https://rpc.example', 1)).toBe('https://rpc.example/chain/1');
  expect(rpcUrl('https://rpc.example/rpc///', 4)).toBe('https://rpc.example/rpc/chain/4');
  expect(rpcUrl('wss://rpc.example/', 2)).toBe('wss://rpc.example/chain/2');
  for (const id of [ 0, -1, 1.5, NaN, 0x100000000 ]) {
    expect(() => rpcUrl('https://rpc.example', id)).toThrow();
  }
  for (const base of [ '', 'javascript:alert(1)', 'https://rpc.example?key=value', 'https://rpc.example#hash' ]) {
    expect(() => rpcUrl(base, 1)).toThrow();
  }
}

test(chains_model_parses_sorts_and_labels);
