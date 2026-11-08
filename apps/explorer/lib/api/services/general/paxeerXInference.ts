import * as v from 'valibot';

import type { ApiResource } from '../../types';

const digest = v.pipe(v.string(), v.regex(/^[0-9a-f]{64}$/));
const unsigned = (bits: number) => v.pipe(
  v.string(), v.regex(/^(0|[1-9]\d*)$/), v.maxLength(39),
  v.check((value) => BigInt(value) < (BigInt(1) << BigInt(bits))),
);
const integer = (max: number) => v.pipe(v.number(), v.integer(), v.minValue(0), v.maxValue(max));
const state = v.union([ v.literal(0), v.literal(1) ]);

const streamSchema = v.object({
  payer: digest,
  stream_account: digest,
  recipient: digest,
  asset_id: digest,
  mode: integer(255),
  rate: unsigned(128),
  rate_unit: unsigned(64),
  start_ms: unsigned(64),
  last_accrual_ms: unsigned(64),
  end_ms: unsigned(64),
  total_cap: unsigned(128),
  accrued_total: unsigned(128),
  settled_total: unsigned(128),
  remainder_carry: unsigned(128),
  cumulative_meter: unsigned(64),
  meter_authorities: v.array(digest),
  underfunded: v.boolean(),
  paused: v.boolean(),
  closed: v.boolean(),
});

const roomSchema = v.object({
  room_id: digest,
  host_key: digest,
  model_digest: digest,
  runtime_digest: digest,
  profile: v.pipe(v.object({
    minimum_bond: unsigned(128),
    canary_ratio_bps: integer(10000),
    lap_size_tokens: unsigned(64),
    challenge_window_blocks: unsigned(64),
    slash_fraction_bps: integer(10000),
    host_share_bps: integer(10000),
    protocol_share_bps: integer(10000),
    settlement_account: digest,
  }), v.check((profile) => profile.host_share_bps + profile.protocol_share_bps <= 10000)),
  state,
  opened_at_epoch: unsigned(64),
  open_asks: v.pipe(v.array(digest), v.maxLength(64)),
  open_bids: v.pipe(v.array(digest), v.maxLength(128)),
  ask_streams: v.array(v.object({ ask_id: digest, stream_id: digest, stream: v.nullable(streamSchema) })),
});

const dealSchema = v.object({
  room_id: digest,
  deal_id: digest,
  ask_id: digest,
  agreed_price_per_thousand_tokens: unsigned(128),
  roster_digest: digest,
  state,
  layers: v.pipe(v.array(v.object({ bid_id: digest, first_layer: integer(65535), end_layer: integer(65535) })),
    v.minLength(1), v.maxLength(32), v.check((layers) => layers.every((layer, index) =>
      layer.first_layer === (index === 0 ? 0 : layers[index - 1].end_layer) && layer.end_layer > layer.first_layer) &&
      new Set(layers.map((layer) => layer.bid_id)).size === layers.length)),
  settled_tokens: unsigned(64),
  settled_amount_owed: unsigned(128),
});

const receiptSchema = v.pipe(v.object({
  deal_id: digest,
  first_lap_index: unsigned(64),
  last_lap_index: unsigned(64),
  cumulative_tokens: unsigned(64),
  batch_digest: digest,
  countersign_quorum: integer(4294967295),
}), v.check((receipt) => BigInt(receipt.last_lap_index) >= BigInt(receipt.first_lap_index) &&
  BigInt(receipt.last_lap_index) - BigInt(receipt.first_lap_index) < BigInt(64)));

const peerSchema = v.object({
  peer_key: digest,
  worker: digest,
  delegate: digest,
  metadata: digest,
  generation: unsigned(64),
  key_version: unsigned(64),
  metadata_revision: unsigned(64),
  valid_from: unsigned(64),
  expiry: unsigned(64),
  state: v.pipe(integer(6), v.minValue(1)),
  enrolled: v.boolean(),
  capabilities_status: v.union([ v.literal(1), v.literal(4) ]),
  capabilities: v.nullable(v.object({
    accelerator_memory_mib: integer(4294967295),
    system_memory_mib: integer(4294967295),
    bandwidth_class: integer(4),
    region_code: v.pipe(v.string(), v.regex(/^[a-z]{2}$/i)),
    attestation_family: integer(255),
    runtime_kind: integer(2),
  })),
  bond_status: v.literal(2),
  fault_count: integer(4294967295),
  attestation_binding: v.object({
    family: integer(255), model_digest: digest, runtime_digest: digest, block_height: unsigned(64),
  }),
});

export const INFERENCE_SCHEMAS = {
  px_getInferenceRooms: v.array(roomSchema),
  px_getInferenceRoom: roomSchema,
  px_getInferenceDeal: dealSchema,
  px_getInferencePeer: peerSchema,
  px_getInferenceReceipts: v.array(receiptSchema),
};

export type InferenceMethod = keyof typeof INFERENCE_SCHEMAS;
export type InferenceResults = { [M in InferenceMethod]: v.InferOutput<typeof INFERENCE_SCHEMAS[M]> };
export type InferenceRoom = InferenceResults['px_getInferenceRoom'];
export type InferenceDeal = InferenceResults['px_getInferenceDeal'];
export type InferenceReceipt = InferenceResults['px_getInferenceReceipts'][number];

export const GENERAL_API_PAXEER_X_INFERENCE_RESOURCES = {
  paxeer_x_inference_rooms: { path: '/api/inference', rpcMethod: 'px_getInferenceRooms' },
  paxeer_x_inference_room: { path: '/api/inference', rpcMethod: 'px_getInferenceRoom' },
  paxeer_x_inference_deal: { path: '/api/inference', rpcMethod: 'px_getInferenceDeal' },
  paxeer_x_inference_peer: { path: '/api/inference', rpcMethod: 'px_getInferencePeer' },
  paxeer_x_inference_receipts: { path: '/api/inference', rpcMethod: 'px_getInferenceReceipts' },
} as const satisfies Record<string, ApiResource & { rpcMethod: InferenceMethod }>;

export type GeneralApiPaxeerXInferenceResourceName = `general:${ keyof typeof GENERAL_API_PAXEER_X_INFERENCE_RESOURCES }`;
export type GeneralApiPaxeerXInferenceResourcePayload<R extends GeneralApiPaxeerXInferenceResourceName> =
  R extends `general:${ infer K extends keyof typeof GENERAL_API_PAXEER_X_INFERENCE_RESOURCES }` ?
    InferenceResults[typeof GENERAL_API_PAXEER_X_INFERENCE_RESOURCES[K]['rpcMethod']] : never;

export function isInferenceMethod(value: unknown): value is InferenceMethod {
  return typeof value === 'string' && Object.hasOwn(INFERENCE_SCHEMAS, value);
}

export function isInferenceId(value: unknown): value is string {
  return v.is(digest, value) && value !== '0'.repeat(64);
}

export function parseInferenceResult<M extends InferenceMethod>(method: M, value: unknown): InferenceResults[M] {
  const result = v.safeParse(INFERENCE_SCHEMAS[method], value);
  if (!result.success) throw new Error('Invalid inference response');
  return result.output as InferenceResults[M];
}

export function inferenceQueryKey(method: InferenceMethod, id?: string) {
  return [ 'paxeer-x-inference', method, id ?? null ] as const;
}

export async function paxeerXInference<M extends InferenceMethod>(method: M, id?: string, signal?: AbortSignal): Promise<InferenceResults[M]> {
  const resource = Object.values(GENERAL_API_PAXEER_X_INFERENCE_RESOURCES).find((entry) => entry.rpcMethod === method);
  if (!resource || (method !== 'px_getInferenceRooms' && !isInferenceId(id)) || (method === 'px_getInferenceRooms' && id !== undefined)) {
    throw new Error('Invalid inference request');
  }
  const response = await fetch(`/explorer${ resource.path }`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'same-origin',
    signal,
    body: JSON.stringify({ method, params: id === undefined ? [] : [ id ] }),
  });
  if (!response.ok) throw new Error('Inference data is unavailable. Please try again later.');
  const payload: unknown = await response.json();
  if (typeof payload !== 'object' || payload === null || !('result' in payload)) throw new Error('Invalid inference response');
  return parseInferenceResult(method, payload.result);
}
