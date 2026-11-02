import * as v from 'valibot';
import type { AddEthereumChainParameter } from 'viem';

import type { ChainRecord } from './model';
import { parseChainsResponse, rpcUrl } from './model';

const unsignedInteger = v.pipe(v.number(), v.integer(), v.minValue(0), v.maxValue(Number.MAX_SAFE_INTEGER));
const virtualId = v.pipe(unsignedInteger, v.minValue(1), v.maxValue(0xFFFFFFFF));
const decimal = v.pipe(v.string(), v.regex(/^(0|[1-9]\d*)$/));
const rowId = v.pipe(v.string(), v.regex(/^[1-9]\d*$/));
const text = v.pipe(v.string(), v.minLength(1));
const summarySchema = v.object({ id: virtualId, tx_count: unsignedInteger, gas_used: unsignedInteger, last_height: unsignedInteger });
const activityFields = {
  id: rowId,
  chain: v.pipe(v.string(), v.regex(/^pax:[1-9]\d*$/)),
  height_or_seq: decimal,
  tx_id: text,
  ordinal: decimal,
  decoded: v.unknown(),
};
const transferSchema = v.looseObject({
  ...activityFields,
  kind: text,
  direction: v.picklist([ 'in', 'out' ]),
  account: text,
  counterparty: v.nullable(text),
  asset: text,
  amount: decimal,
  tx_index: decimal,
  log_index: decimal,
  'final': v.boolean(),
  final_basis: v.literal('block_commit'),
  stability: v.object({
    level: v.picklist([ 'depth_stable', 'reversible' ]),
    source: v.literal('block_commit'),
    finalized_boundary: v.nullable(decimal),
  }),
  settlement: v.record(v.string(), v.unknown()),
});
const eventSchema = v.looseObject({
  ...activityFields,
  source: text,
  name: text,
  contract: v.nullable(text),
  account: v.nullable(text),
});

export type ChainSummary = v.InferOutput<typeof summarySchema>;
export type ChainTransfer = v.InferOutput<typeof transferSchema>;
export type ChainEvent = v.InferOutput<typeof eventSchema>;
export type ActivityKind = 'transfers' | 'events';

export function parseSummary(json: unknown, expectedId?: number): ChainSummary {
  const summary = v.parse(summarySchema, json);
  if (expectedId !== undefined && summary.id !== expectedId) {
    throw new Error('Summary belongs to a different virtual chain');
  }
  return summary;
}

export function parseActivityPage(json: unknown, kind?: ActivityKind, expectedId?: number) {
  const schemas = { transfers: transferSchema, events: eventSchema };
  const itemSchema = kind ? schemas[kind] : v.union([ transferSchema, eventSchema ]);
  const page = v.parse(v.looseObject({
    version: v.literal(1),
    items: v.array(itemSchema),
    next_cursor: v.nullable(rowId),
  }), json);
  const ids = new Set<string>();
  for (const item of page.items) {
    const id = Number(item.chain.slice(4));
    v.parse(virtualId, id);
    if ((expectedId !== undefined && id !== expectedId) || ids.has(item.id)) {
      throw new Error('Invalid activity identity in virtual chain page');
    }
    ids.add(item.id);
  }
  return page;
}

function httpBase(base: string): string {
  const url = new URL(base);
  if (![ 'https:', 'http:' ].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error('A HTTP(S) base URL without credentials, query or fragment is required');
  }
  return base.replace(/\/+$/, '');
}

export function addChainParams(record: ChainRecord, rpcBase: string, explorerBase: string): AddEthereumChainParameter {
  const [ chain ] = parseChainsResponse({ chains: [ record ] });
  return {
    chainId: `0x${ (125000000 + chain.id).toString(16) }`,
    chainName: `${ chain.name } on Paxeer`,
    rpcUrls: [ rpcUrl(httpBase(rpcBase), chain.id) ],
    nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
    blockExplorerUrls: [ `${ httpBase(explorerBase) }/chains/${ chain.id }` ],
  };
}
