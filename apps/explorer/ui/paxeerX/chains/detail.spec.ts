import { RESOURCES } from 'lib/api/resources';
import { expect, test } from 'vitest';

import { addChainParams, parseActivityPage, parseSummary } from './detail';
import { parseChainsResponse } from './model';

function chain_detail_parses_activity_and_builds_wallet_params() {
  const recordJson = JSON.parse(`{
    "id":1,"name":"demo","operator":"0x1111111111111111111111111111111111111111",
    "status":"active","chain_id":125000001,"created_height":42,
    "bond":"100000000000000000000","gas_allowance":500000,"byte_allowance":65536,"fee_share_bps":100
  }`);
  const [ record ] = parseChainsResponse({ chains: [ recordJson ] });
  const summaryJson = JSON.parse('{"id":1,"tx_count":2,"gas_used":42005,"last_height":42}');
  const summary = parseSummary(summaryJson, record.id);
  expect(summary).toEqual(summaryJson);
  expect(parseSummary({ id: 1, tx_count: 0, gas_used: 0, last_height: 0 })).toEqual({
    id: 1, tx_count: 0, gas_used: 0, last_height: 0,
  });

  const transfersJson = JSON.parse(`{
    "version":1,"indexedHeight":42,"chainHeight":42,"next_cursor":"1",
    "items":[{
      "id":"1","height_or_seq":"42","chain":"pax:1","kind":"native","direction":"out",
      "account":"0x1111111111111111111111111111111111111111",
      "counterparty":"0x2222222222222222222222222222222222222222","asset":"PAX",
      "amount":"1000000000000000000","tx_id":"0x00000000000000000000000000000000000000000000000000000000000007d1",
      "tx_index":"1","log_index":"0","ordinal":"1","decoded":{},
      "final":false,"final_basis":"block_commit",
      "stability":{"level":"reversible","source":"block_commit","finalized_boundary":null},
      "settlement":{"level":"unverified","source":null,"reason":"no_verified_checkpoint_evidence"}
    }]
  }`);
  const eventsJson = JSON.parse(`{
    "version":1,"indexedHeight":42,"chainHeight":42,"next_cursor":null,
    "items":[{
      "id":"2","height_or_seq":"42","chain":"pax:1","source":"chain-transaction",
      "name":"transaction","contract":null,"account":"0x1111111111111111111111111111111111111111",
      "tx_id":"0x00000000000000000000000000000000000000000000000000000000000007d1",
      "ordinal":"0","decoded":{"chain_id":125000001}
    }]
  }`);
  const transfers = parseActivityPage(transfersJson, 'transfers', record.id);
  const events = parseActivityPage(eventsJson, 'events', record.id);
  expect(transfers).toEqual(transfersJson);
  expect(events).toEqual(eventsJson);
  expect(transfers.items[0].amount).toBe('1000000000000000000');
  expect(transfers.items[0].stability).toEqual({ level: 'reversible', source: 'block_commit', finalized_boundary: null });
  expect(events.items[0].decoded).toEqual({ chain_id: 125000001 });
  expect(parseActivityPage(transfersJson)).toEqual(transfersJson);
  expect(parseActivityPage(eventsJson)).toEqual(eventsJson);
  expect(parseActivityPage({ version: 1, items: [], next_cursor: null }, 'transfers', 1)).toEqual({
    version: 1, items: [], next_cursor: null,
  });

  const params = addChainParams(record, 'https://rpc.example', 'https://scan.example');
  expect(params).toEqual({
    chainId: '0x7735941',
    chainName: 'demo on Paxeer',
    rpcUrls: [ 'https://rpc.example/chain/1' ],
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    blockExplorerUrls: [ 'https://scan.example/chains/1' ],
  });
  expect(Number(params.chainId)).toBe(record.chain_id);
  expect(addChainParams(record, 'https://rpc.example/rpc///', 'https://scan.example/explorer///')).toEqual({
    ...params,
    rpcUrls: [ 'https://rpc.example/rpc/chain/1' ],
    blockExplorerUrls: [ 'https://scan.example/explorer/chains/1' ],
  });
  expect(RESOURCES.general.paxeer_x_chain).toEqual({ path: '/v1/chains/:id', pathParams: [ 'id' ] });
  expect(RESOURCES.general.paxeer_x_chain_summary.path).toBe('/v1/chains/:id/summary');
  expect(RESOURCES.general.paxeer_x_chain_transfers.path).toBe('/v1/chains/:id/transfers');
  expect(RESOURCES.general.paxeer_x_chain_events.path).toBe('/v1/chains/:id/events');

  for (const invalid of [ null, [], {}, { ...summary, id: 0 }, { ...summary, tx_count: '2' },
    { ...summary, gas_used: -1 }, { ...summary, last_height: 1.5 },
    { ...summary, tx_count: Number.MAX_SAFE_INTEGER + 1 } ]) {
    expect(() => parseSummary(invalid)).toThrow();
  }
  for (const field of Object.keys(summary)) {
    const incomplete = { ...summary };
    Reflect.deleteProperty(incomplete, field);
    expect(() => parseSummary(incomplete)).toThrow();
  }
  expect(() => parseSummary(summaryJson, 2)).toThrow();
  expect(() => parseActivityPage(eventsJson, 'events', 2)).toThrow();
  expect(() => parseActivityPage(transfersJson, 'transfers', 2)).toThrow();
  expect(() => parseActivityPage(transfersJson, 'events', 1)).toThrow();
  expect(() => parseActivityPage(eventsJson, 'transfers', 1)).toThrow();
  for (const invalid of [ null, [], {}, { ...events, version: 2 }, { ...events, items: null },
    { ...events, next_cursor: 2 }, { ...events, next_cursor: '' }, { ...events, items: [ {} ] } ]) {
    expect(() => parseActivityPage(invalid)).toThrow();
  }
  for (const changes of [ { id: '0' }, { height_or_seq: 42 }, { ordinal: '-1' }, { chain: 'paxeer' },
    { chain: 'pax:4294967296' }, { tx_id: '' }, { direction: 'both' }, { amount: 100 },
    { 'final': 'true' }, { stability: null }, { settlement: null } ]) {
    expect(() => parseActivityPage({ ...transfers, items: [ { ...transfers.items[0], ...changes } ] }, 'transfers', 1)).toThrow();
  }
  for (const changes of [ { name: '' }, { source: 1 }, { contract: 123 }, { account: false }, { decoded: undefined, chain: 'pax:0' } ]) {
    expect(() => parseActivityPage({ ...events, items: [ { ...events.items[0], ...changes } ] }, 'events', 1)).toThrow();
  }
  expect(() => parseActivityPage({ ...events, items: [ events.items[0], events.items[0] ] })).toThrow();
  expect(() => addChainParams({ ...record, chain_id: 125 }, 'https://rpc.example', 'https://scan.example')).toThrow();
  expect(() => addChainParams({ ...record, id: 0 }, 'https://rpc.example', 'https://scan.example')).toThrow();
  for (const base of [ '', 'wss://rpc.example', 'javascript:alert(1)', 'https://rpc.example?key=value',
    'https://rpc.example#hash', 'https://user:pass@rpc.example' ]) {
    expect(() => addChainParams(record, base, 'https://scan.example')).toThrow();
    expect(() => addChainParams(record, 'https://rpc.example', base)).toThrow();
  }
}

test(chain_detail_parses_activity_and_builds_wallet_params);
