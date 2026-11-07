import { decompress } from 'brotli-compress/js';
import * as v from 'valibot';

import type { FlashblockItemApiMegaEth, FlashblockItemApiOptimism } from 'types/api/flashblocks';

const quantity = v.pipe(v.string(), v.regex(/^(?:0x[\da-f]+|\d+)$/i), v.check((value) => Number.isSafeInteger(Number(value))));
const index = v.pipe(v.number(), v.integer(), v.minValue(0), v.safeInteger());
const optimismItem = v.looseObject({
  payload_id: v.string(),
  index,
  base: v.optional(v.looseObject({
    parent_beacon_block_root: v.string(),
    parent_hash: v.string(),
    fee_recipient: v.string(),
    prev_randao: v.string(),
    block_number: quantity,
    gas_limit: quantity,
    timestamp: quantity,
    extra_data: v.string(),
    base_fee_per_gas: v.string(),
  })),
  diff: v.looseObject({
    state_root: v.string(),
    receipts_root: v.string(),
    logs_bloom: v.string(),
    gas_used: quantity,
    block_hash: v.string(),
    transactions: v.array(v.string()),
  }),
  metadata: v.looseObject({
    receipts: v.record(v.string(), v.unknown()),
    block_number: v.optional(index),
  }),
});
const megaEthItem = v.looseObject({
  block_number: quantity,
  block_timestamp: quantity,
  gas_used: quantity,
  index: quantity,
  mini_block_number: index,
  mini_block_timestamp: v.pipe(quantity, v.check((value) => Number(value) / 1000 <= 8.64e15)),
  receipts: v.array(v.unknown()),
  transactions: v.array(v.unknown()),
});

export async function parseSocketEventDataOptimism(event: MessageEvent): Promise<FlashblockItemApiOptimism> {
  let text: string;
  if (typeof event.data === 'string') {
    text = event.data;
  } else if (event.data instanceof Blob || event.data instanceof ArrayBuffer) {
    const bytes = new Uint8Array(event.data instanceof Blob ? await event.data.arrayBuffer() : event.data);
    const decoded = new TextDecoder().decode(bytes);
    text = decoded.trimStart().startsWith('{') ? decoded : new TextDecoder().decode(await decompress(bytes));
  } else {
    throw new TypeError('Unsupported flashblock frame');
  }

  return v.parse(optimismItem, JSON.parse(text));
}

export function parseSocketEventDataMegaEth(event: MessageEvent): FlashblockItemApiMegaEth | undefined {
  const message = v.parse(v.looseObject({ jsonrpc: v.literal('2.0') }), JSON.parse(event.data));
  if (message.method === 'eth_subscription') {
    return v.parse(v.looseObject({ params: v.looseObject({ result: megaEthItem, subscription: v.string() }) }), message).params.result;
  }

  v.parse(v.looseObject({ id: v.literal(1), result: v.pipe(v.string(), v.minLength(1)) }), message);
}
