import { encodeAbiCall, type PrecompileCall } from "./exchange.js";
import type { PxClient } from "./px.js";

export const INFERENCE_OPERATIONS = Object.freeze({
  openRoom: 0x0b01,
  postAsk: 0x0b02,
  postBid: 0x0b03,
  recordDeal: 0x0b04,
  settleLaps: 0x0b05,
  challengeLap: 0x0b06,
  resolveChallenge: 0x0b07,
  splitPayout: 0x0b08,
  lockBond: 0x0b09,
  slashBond: 0x0b0a,
  refundBond: 0x0b0b,
  closeRoom: 0x0b0c,
  roomView: 0x0b10,
  dealView: 0x0b11,
  peerView: 0x0b12,
  receiptBatchView: 0x0b13,
  bookView: 0x0b14,
} as const);

export const INFERENCE_LIMITS = Object.freeze({
  rooms: 16,
  asks: 64,
  bids: 128,
  dealPeers: 32,
  batchLaps: 64,
  manifestMinBytes: 386,
  manifestMaxBytes: 4096,
  bookSide: 64,
} as const);

const ZERO32 = "00".repeat(32);
const HEX32 = /^[0-9a-f]{64}$/u;
const U64_MAX = (1n << 64n) - 1n;
const U128_MAX = (1n << 128n) - 1n;

export class InferenceMarketError extends Error {
  public constructor(public readonly field: string) {
    super(`Invalid inference market ${field}`);
    this.name = "InferenceMarketError";
  }
}

function invalid(field: string): never {
  throw new InferenceMarketError(field);
}

export function inferenceId(value: unknown): string {
  if (typeof value !== "string" || !HEX32.test(value) || value === ZERO32)
    invalid("identifier");
  return value;
}

function integer(value: unknown, maximum: number, minimum = 0): number {
  if (
    typeof value !== "number" ||
    !Number.isSafeInteger(value) ||
    value < minimum ||
    value > maximum
  )
    invalid("integer");
  return value;
}

function unsigned(value: bigint, bytes: number, positive = false): bigint {
  if (
    typeof value !== "bigint" ||
    value < (positive ? 1n : 0n) ||
    value >= 1n << BigInt(bytes * 8)
  )
    invalid("quantity");
  return value;
}

class Writer {
  readonly #parts: Uint8Array[] = [];
  public put(value: Uint8Array): this {
    this.#parts.push(new Uint8Array(value));
    return this;
  }
  public number(value: bigint, bytes: number, positive = false): this {
    let rest = unsigned(value, bytes, positive);
    const out = new Uint8Array(bytes);
    for (let i = bytes - 1; i >= 0; i -= 1) {
      out[i] = Number(rest & 255n);
      rest >>= 8n;
    }
    return this.put(out);
  }
  public small(value: number, bytes: number, minimum = 0): this {
    return this.number(
      BigInt(integer(value, 2 ** (bytes * 8) - 1, minimum)),
      bytes,
    );
  }
  public id(value: string): this {
    return this.put(Buffer.from(inferenceId(value), "hex"));
  }
  public bytes(): Uint8Array {
    return new Uint8Array(Buffer.concat(this.#parts));
  }
}

class Reader {
  #offset = 0;
  public constructor(private readonly input: Uint8Array) {
    if (!(input instanceof Uint8Array)) invalid("view bytes");
  }
  public take(length: number): Uint8Array {
    if (this.#offset + length > this.input.length) invalid("view length");
    const out = new Uint8Array(
      this.input.subarray(this.#offset, this.#offset + length),
    );
    this.#offset += length;
    return out;
  }
  public number(bytes: number): bigint {
    let value = 0n;
    for (const byte of this.take(bytes)) value = (value << 8n) | BigInt(byte);
    return value;
  }
  public small(bytes: number): number {
    return Number(this.number(bytes));
  }
  public id(): string {
    return inferenceId(Buffer.from(this.take(32)).toString("hex"));
  }
  public zeros(length: number): void {
    if (this.take(length).some((byte) => byte !== 0)) invalid("padding");
  }
  public end(): void {
    if (this.#offset !== this.input.length) invalid("view length");
  }
}

export interface InferenceRoomProfile {
  readonly minimumBond: bigint;
  readonly canaryRatioBps: number;
  readonly lapSizeTokens: bigint;
  readonly challengeWindowBlocks: bigint;
  readonly slashFractionBps: number;
  readonly hostShareBps: number;
  readonly protocolShareBps: number;
  readonly settlementAccount: string;
}

function validateProfile(profile: InferenceRoomProfile): void {
  unsigned(profile.minimumBond, 16, true);
  unsigned(profile.lapSizeTokens, 8, true);
  unsigned(profile.challengeWindowBlocks, 8, true);
  integer(profile.canaryRatioBps, 10_000);
  integer(profile.slashFractionBps, 10_000);
  integer(profile.hostShareBps, 10_000);
  integer(profile.protocolShareBps, 10_000);
  if (profile.hostShareBps + profile.protocolShareBps > 10_000)
    invalid("payout shares");
  inferenceId(profile.settlementAccount);
}

function writeProfile(w: Writer, p: InferenceRoomProfile): void {
  validateProfile(p);
  w.number(p.minimumBond, 16)
    .small(p.canaryRatioBps, 2)
    .number(p.lapSizeTokens, 8)
    .number(p.challengeWindowBlocks, 8)
    .small(p.slashFractionBps, 2)
    .small(p.hostShareBps, 2)
    .small(p.protocolShareBps, 2)
    .id(p.settlementAccount);
}

function readProfile(r: Reader): InferenceRoomProfile {
  const profile: InferenceRoomProfile = Object.freeze({
    minimumBond: r.number(16),
    canaryRatioBps: r.small(2),
    lapSizeTokens: r.number(8),
    challengeWindowBlocks: r.number(8),
    slashFractionBps: r.small(2),
    hostShareBps: r.small(2),
    protocolShareBps: r.small(2),
    settlementAccount: r.id(),
  });
  validateProfile(profile);
  return profile;
}

export interface OpenRoomParams {
  readonly roomId: string;
  readonly modelDigest: string;
  readonly runtimeDigest: string;
  readonly profile: InferenceRoomProfile;
}

export function openRoom(params: OpenRoomParams): Uint8Array {
  const w = new Writer()
    .id(params.roomId)
    .id(params.modelDigest)
    .id(params.runtimeDigest);
  writeProfile(w, params.profile);
  return w.bytes();
}

export interface PostAskParams {
  readonly roomId: string;
  readonly askId: string;
  readonly modelDigest: string;
  readonly maximumPricePerThousandTokens: bigint;
  readonly streamId: bigint;
}

export function postAsk(params: PostAskParams): Uint8Array {
  return new Writer()
    .id(params.roomId)
    .id(params.askId)
    .id(params.modelDigest)
    .number(params.maximumPricePerThousandTokens, 16, true)
    .number(params.streamId, 16)
    .bytes();
}

export interface PostBidParams {
  readonly roomId: string;
  readonly bidId: string;
  readonly capabilityDigest: string;
  readonly minimumPricePerThousandTokens: bigint;
  readonly bondAmount: bigint;
  readonly layerCapacity: number;
}

export function postBid(params: PostBidParams): Uint8Array {
  return new Writer()
    .id(params.roomId)
    .id(params.bidId)
    .id(params.capabilityDigest)
    .number(params.minimumPricePerThousandTokens, 16, true)
    .number(params.bondAmount, 16, true)
    .small(params.layerCapacity, 2, 1)
    .bytes();
}

export interface InferenceLayerSplit {
  readonly bidId: string;
  readonly firstLayer: number;
  readonly endLayer: number;
}

export interface RecordDealParams {
  readonly roomId: string;
  readonly dealId: string;
  readonly askId: string;
  readonly agreedPricePerThousandTokens: bigint;
  readonly rosterDigest: string;
  readonly layers: readonly InferenceLayerSplit[];
}

function validateLayers(layers: readonly InferenceLayerSplit[]): void {
  if (!Array.isArray(layers)) invalid("layers");
  integer(layers.length, INFERENCE_LIMITS.dealPeers, 1);
  let end = 0;
  const seen = new Set<string>();
  for (const layer of layers) {
    inferenceId(layer.bidId);
    integer(layer.firstLayer, 65535);
    integer(layer.endLayer, 65535, 1);
    if (
      layer.firstLayer !== end ||
      layer.endLayer <= layer.firstLayer ||
      seen.has(layer.bidId)
    )
      invalid("layer partition");
    seen.add(layer.bidId);
    end = layer.endLayer;
  }
}

export function recordDeal(params: RecordDealParams): Uint8Array {
  validateLayers(params.layers);
  const w = new Writer()
    .id(params.roomId)
    .id(params.dealId)
    .id(params.askId)
    .number(params.agreedPricePerThousandTokens, 16, true)
    .id(params.rosterDigest)
    .small(params.layers.length, 1);
  for (const layer of params.layers)
    w.id(layer.bidId).small(layer.firstLayer, 2).small(layer.endLayer, 2);
  return w.bytes();
}

export interface InferenceReceiptBatch {
  readonly dealId: string;
  readonly firstLapIndex: bigint;
  readonly lastLapIndex: bigint;
  readonly cumulativeTokens: bigint;
  readonly batchDigest: string;
  readonly countersignQuorum: number;
}

function validateBatch(batch: InferenceReceiptBatch): void {
  inferenceId(batch.dealId);
  inferenceId(batch.batchDigest);
  unsigned(batch.firstLapIndex, 8);
  unsigned(batch.lastLapIndex, 8);
  unsigned(batch.cumulativeTokens, 8);
  integer(batch.countersignQuorum, 0xffff_ffff);
  if (
    batch.lastLapIndex < batch.firstLapIndex ||
    batch.lastLapIndex - batch.firstLapIndex >=
      BigInt(INFERENCE_LIMITS.batchLaps)
  )
    invalid("lap interval");
}

export interface SettleLapsParams {
  readonly batch: InferenceReceiptBatch;
  readonly artifactManifest: Uint8Array;
}

export function settleLaps(params: SettleLapsParams): Uint8Array {
  const b = params.batch;
  validateBatch(b);
  if (!(params.artifactManifest instanceof Uint8Array))
    invalid("artifact manifest");
  integer(
    params.artifactManifest.length,
    INFERENCE_LIMITS.manifestMaxBytes,
    INFERENCE_LIMITS.manifestMinBytes,
  );
  return new Writer()
    .id(b.dealId)
    .number(b.firstLapIndex, 8)
    .number(b.lastLapIndex, 8)
    .number(b.cumulativeTokens, 8)
    .id(b.batchDigest)
    .small(b.countersignQuorum, 4)
    .small(params.artifactManifest.length, 2)
    .put(params.artifactManifest)
    .bytes();
}

export interface ChallengeLapParams {
  readonly deal: string;
  readonly lap: bigint;
  readonly commitment: string;
  readonly deposit: bigint;
}

export function challengeLap(params: ChallengeLapParams): Uint8Array {
  return new Writer()
    .id(params.deal)
    .number(params.lap, 8)
    .id(params.commitment)
    .number(params.deposit, 16)
    .bytes();
}

export type CanaryLogits = readonly [
  number,
  number,
  number,
  number,
  number,
  number,
  number,
  number,
];
export interface ResolveChallengeParams {
  readonly deal: string;
  readonly lap: bigint;
  readonly commitment: string;
  readonly answer: CanaryLogits;
}

export function resolveChallenge(params: ResolveChallengeParams): Uint8Array {
  if (!Array.isArray(params.answer) || params.answer.length !== 8)
    invalid("canary logits");
  const w = new Writer()
    .id(params.deal)
    .number(params.lap, 8)
    .id(params.commitment);
  for (const value of params.answer) {
    integer(value, 0x7fff_ffff, -0x8000_0000);
    w.number(BigInt(value < 0 ? value + 0x1_0000_0000 : value), 4);
  }
  return w.bytes();
}

export interface SplitPayoutParams {
  readonly deal: string;
  readonly batch: string;
  readonly amount: bigint;
}

export function splitPayout(params: SplitPayoutParams): Uint8Array {
  return new Writer()
    .id(params.deal)
    .id(params.batch)
    .number(params.amount, 16, true)
    .bytes();
}

export interface LockBondParams {
  readonly room: string;
  readonly bid: string;
  readonly amount: bigint;
}

export function lockBond(params: LockBondParams): Uint8Array {
  return new Writer()
    .id(params.room)
    .id(params.bid)
    .number(params.amount, 16, true)
    .bytes();
}

export interface SlashBondParams {
  readonly deal: string;
  readonly bid: string;
  readonly lap: bigint;
  readonly evidence: string;
}

export function slashBond(params: SlashBondParams): Uint8Array {
  return new Writer()
    .id(params.deal)
    .id(params.bid)
    .number(params.lap, 8)
    .id(params.evidence)
    .bytes();
}

export interface RefundBondParams {
  readonly room: string;
  readonly bid: string;
}

export function refundBond(params: RefundBondParams): Uint8Array {
  return new Writer().id(params.room).id(params.bid).bytes();
}

export function closeRoom(roomId: string): Uint8Array {
  return new Writer().id(roomId).bytes();
}

export interface FundAskParams {
  readonly askRef: string;
  readonly settlementAccount: string;
  readonly rate: bigint;
  readonly rateUnit: number;
  readonly totalCap: bigint;
  readonly hostAuthorityKey: string;
}

export function fundAsk(params: FundAskParams): PrecompileCall {
  inferenceId(params.askRef);
  inferenceId(params.hostAuthorityKey);
  unsigned(params.rate, 16, true);
  unsigned(params.totalCap, 16, true);
  integer(params.rateUnit, 255, 1);
  if (
    typeof params.settlementAccount !== "string" ||
    !/^0x[0-9a-fA-F]{40}$/u.test(params.settlementAccount) ||
    /^0x0{40}$/u.test(params.settlementAccount)
  )
    invalid("settlement address");
  return Object.freeze({
    to: "0x000000000000000000000000000000000000101b",
    data: encodeAbiCall(
      "streamOpenForAsk",
      ["bytes32", "address", "uint256", "uint8", "uint256", "bytes32"],
      [
        `0x${params.askRef}`,
        params.settlementAccount,
        params.rate,
        BigInt(params.rateUnit),
        params.totalCap,
        `0x${params.hostAuthorityKey}`,
      ],
    ),
    value: 0n,
  });
}

export interface InferenceRoomView extends OpenRoomParams {
  readonly hostKey: string;
  readonly state: 0 | 1;
  readonly openedAtEpoch: bigint;
  readonly openAsks: readonly string[];
  readonly openBids: readonly string[];
}

function orderedIds(r: Reader, capacity: number): readonly string[] {
  const ids: string[] = [];
  let ended = false;
  for (let i = 0; i < capacity; i += 1) {
    const id = Buffer.from(r.take(32)).toString("hex");
    if (id === ZERO32) ended = true;
    else {
      if (ended || (ids.length > 0 && ids[ids.length - 1]! >= id))
        invalid("order identifiers");
      ids.push(id);
    }
  }
  return Object.freeze(ids);
}

export function decodeRoomView(bytes: Uint8Array): InferenceRoomView {
  const r = new Reader(bytes);
  const roomId = r.id(),
    hostKey = r.id(),
    modelDigest = r.id(),
    runtimeDigest = r.id();
  const profile = readProfile(r);
  const state = roomState(r.small(1));
  const openedAtEpoch = r.number(8);
  const openAsks = orderedIds(r, INFERENCE_LIMITS.asks);
  const openBids = orderedIds(r, INFERENCE_LIMITS.bids);
  r.end();
  if (state === 1 && (openAsks.length > 0 || openBids.length > 0))
    invalid("closed room orders");
  return Object.freeze({
    roomId,
    hostKey,
    modelDigest,
    runtimeDigest,
    profile,
    state,
    openedAtEpoch,
    openAsks,
    openBids,
  });
}

export interface InferenceDealView extends RecordDealParams {
  readonly state: 0 | 1;
  readonly settledTokens: bigint;
  readonly settledAmountOwed: bigint;
}

export function decodeDealView(bytes: Uint8Array): InferenceDealView {
  const r = new Reader(bytes);
  const roomId = r.id(),
    dealId = r.id(),
    askId = r.id();
  const agreedPricePerThousandTokens = unsigned(r.number(16), 16, true);
  const rosterDigest = r.id();
  const count = integer(r.small(1), INFERENCE_LIMITS.dealPeers, 1);
  const state = roomState(r.small(1));
  const layers: InferenceLayerSplit[] = [];
  for (let i = 0; i < count; i += 1)
    layers.push(
      Object.freeze({
        bidId: r.id(),
        firstLayer: r.small(2),
        endLayer: r.small(2),
      }),
    );
  r.zeros((INFERENCE_LIMITS.dealPeers - count) * 36);
  validateLayers(layers);
  const settledTokens = r.number(8),
    settledAmountOwed = r.number(16);
  r.end();
  return Object.freeze({
    roomId,
    dealId,
    askId,
    agreedPricePerThousandTokens,
    rosterDigest,
    layers: Object.freeze(layers),
    state,
    settledTokens,
    settledAmountOwed,
  });
}

export interface InferencePeerCapabilities {
  readonly acceleratorMemoryMib: number;
  readonly systemMemoryMib: number;
  readonly bandwidthClass: number;
  readonly regionCode: string;
  readonly attestationFamily: number;
  readonly runtimeKind: number;
}
export interface InferencePeerView {
  readonly peerKey: string;
  readonly worker: string;
  readonly delegate: string;
  readonly metadata: string;
  readonly generation: bigint;
  readonly keyVersion: bigint;
  readonly metadataRevision: bigint;
  readonly validFrom: bigint;
  readonly expiry: bigint;
  readonly state: 1 | 2 | 3 | 4 | 5 | 6;
  readonly enrolled: boolean;
  readonly capabilitiesStatus: 1 | 4;
  readonly capabilities: InferencePeerCapabilities | null;
  readonly bondStatus: 2;
  readonly faultCount: number;
}

function capabilities(
  value: InferencePeerCapabilities,
): InferencePeerCapabilities {
  integer(value.acceleratorMemoryMib, 0xffff_ffff);
  integer(value.systemMemoryMib, 0xffff_ffff);
  integer(value.bandwidthClass, 4);
  integer(value.attestationFamily, 255);
  integer(value.runtimeKind, 2);
  if (
    typeof value.regionCode !== "string" ||
    !/^[a-zA-Z]{2}$/u.test(value.regionCode)
  )
    invalid("region code");
  return Object.freeze(value);
}

export function decodePeerView(bytes: Uint8Array): InferencePeerView {
  const r = new Reader(bytes);
  const peerKey = r.id(),
    worker = r.id(),
    delegate = r.id(),
    metadata = r.id();
  const generation = unsigned(r.number(8), 8, true),
    keyVersion = unsigned(r.number(8), 8, true);
  const metadataRevision = unsigned(r.number(8), 8, true),
    validFrom = r.number(8),
    expiry = r.number(8);
  if (validFrom >= expiry) invalid("peer validity");
  const state = peerState(r.small(1));
  const enrolled = booleanByte(r.small(1));
  const capabilitiesStatus = capabilityStatus(r.small(1));
  let peerCapabilities: InferencePeerCapabilities | null = null;
  if (capabilitiesStatus === 1) {
    peerCapabilities = capabilities({
      acceleratorMemoryMib: r.small(4),
      systemMemoryMib: r.small(4),
      bandwidthClass: r.small(1),
      regionCode: Buffer.from(r.take(2)).toString("latin1"),
      attestationFamily: r.small(1),
      runtimeKind: r.small(1),
    });
  } else r.zeros(13);
  if (r.small(1) !== 2) invalid("bond availability");
  const faultCount = r.small(4);
  r.end();
  return Object.freeze({
    peerKey,
    worker,
    delegate,
    metadata,
    generation,
    keyVersion,
    metadataRevision,
    validFrom,
    expiry,
    state,
    enrolled,
    capabilitiesStatus,
    capabilities: peerCapabilities,
    bondStatus: 2,
    faultCount,
  });
}

export function decodeReceiptBatchView(
  bytes: Uint8Array,
): InferenceReceiptBatch {
  const r = new Reader(bytes);
  const batch: InferenceReceiptBatch = Object.freeze({
    dealId: r.id(),
    firstLapIndex: r.number(8),
    lastLapIndex: r.number(8),
    cumulativeTokens: r.number(8),
    batchDigest: r.id(),
    countersignQuorum: r.small(4),
  });
  r.end();
  validateBatch(batch);
  return batch;
}

export interface InferencePlacement {
  readonly roomId: string;
  readonly orderId: string;
  readonly pricePerThousandTokens: bigint;
  readonly streamId: bigint;
}
export interface InferenceBookView {
  readonly modelDigest: string;
  readonly asks: readonly InferencePlacement[];
  readonly bids: readonly InferencePlacement[];
  readonly crossings: readonly (readonly [number, number])[];
}

function readPlacements(
  r: Reader,
  count: number,
  ask: boolean,
): readonly InferencePlacement[] {
  const rows: InferencePlacement[] = [];
  for (let i = 0; i < count; i += 1) {
    const row = Object.freeze({
      roomId: r.id(),
      orderId: r.id(),
      pricePerThousandTokens: unsigned(r.number(16), 16, true),
      streamId: ask ? r.number(16) : 0n,
    });
    const previous = rows[i - 1];
    if (
      previous &&
      (previous.pricePerThousandTokens === row.pricePerThousandTokens
        ? previous.orderId >= row.orderId
        : ask
          ? previous.pricePerThousandTokens < row.pricePerThousandTokens
          : previous.pricePerThousandTokens > row.pricePerThousandTokens)
    )
      invalid("book ordering");
    rows.push(row);
  }
  return Object.freeze(rows);
}

export function decodeBookView(bytes: Uint8Array): InferenceBookView {
  const r = new Reader(bytes);
  if (Buffer.from(r.take(4)).toString("hex") !== "46313150" || r.small(2) !== 1)
    invalid("book version");
  const modelDigest = r.id();
  const askCount = integer(r.small(1), INFERENCE_LIMITS.bookSide);
  const bidCount = integer(r.small(1), INFERENCE_LIMITS.bookSide);
  const asks = readPlacements(r, askCount, true),
    bids = readPlacements(r, bidCount, false);
  const bitmap = r.take(512);
  r.end();
  const crossings: (readonly [number, number])[] = [];
  for (let a = 0; a < 64; a += 1) {
    for (let b = 0; b < 64; b += 1) {
      const ask = asks[a],
        bid = bids[b];
      const expected =
        ask !== undefined &&
        bid !== undefined &&
        ask.roomId === bid.roomId &&
        ask.pricePerThousandTokens >= bid.pricePerThousandTokens &&
        ask.streamId !== 0n;
      const bit = a * 64 + b;
      if (((bitmap[Math.floor(bit / 8)]! >> bit % 8) & 1) !== Number(expected))
        invalid("book crossing");
      if (expected) crossings.push(Object.freeze([a, b] as const));
    }
  }
  return Object.freeze({
    modelDigest,
    asks,
    bids,
    crossings: Object.freeze(crossings),
  });
}

function roomState(value: unknown): 0 | 1 {
  if (value !== 0 && value !== 1) invalid("room or deal state");
  return value;
}
function peerState(value: unknown): InferencePeerView["state"] {
  if (
    value !== 1 &&
    value !== 2 &&
    value !== 3 &&
    value !== 4 &&
    value !== 5 &&
    value !== 6
  )
    invalid("peer state");
  return value;
}
function capabilityStatus(value: unknown): 1 | 4 {
  if (value !== 1 && value !== 4) invalid("capability availability");
  return value;
}
function booleanByte(value: number): boolean {
  if (value !== 0 && value !== 1) invalid("boolean");
  return value === 1;
}
function jsonBoolean(value: unknown): boolean {
  if (typeof value !== "boolean") invalid("boolean");
  return value;
}
function object(value: unknown): Readonly<Record<string, unknown>> {
  if (typeof value !== "object" || value === null || Array.isArray(value))
    invalid("document");
  return value as Readonly<Record<string, unknown>>;
}
function quantity(value: unknown, maximum = U64_MAX, positive = false): bigint {
  if (
    typeof value !== "string" ||
    value.length > 39 ||
    !/^(0|[1-9][0-9]*)$/u.test(value)
  )
    invalid("decimal quantity");
  const result = BigInt(value);
  if (result > maximum || (positive && result === 0n))
    invalid("decimal quantity");
  return result;
}
function rows(value: unknown, maximum: number): readonly unknown[] {
  if (!Array.isArray(value) || value.length > maximum) invalid("rows");
  return value;
}
function jsonIds(value: unknown, maximum: number): readonly string[] {
  const ids = rows(value, maximum).map(inferenceId);
  for (let i = 1; i < ids.length; i += 1)
    if (ids[i - 1]! >= ids[i]!) invalid("order identifiers");
  return Object.freeze(ids);
}
function jsonProfile(value: unknown): InferenceRoomProfile {
  const p = object(value);
  const profile: InferenceRoomProfile = Object.freeze({
    minimumBond: quantity(p.minimum_bond, U128_MAX, true),
    canaryRatioBps: integer(p.canary_ratio_bps, 10_000),
    lapSizeTokens: quantity(p.lap_size_tokens, U64_MAX, true),
    challengeWindowBlocks: quantity(p.challenge_window_blocks, U64_MAX, true),
    slashFractionBps: integer(p.slash_fraction_bps, 10_000),
    hostShareBps: integer(p.host_share_bps, 10_000),
    protocolShareBps: integer(p.protocol_share_bps, 10_000),
    settlementAccount: inferenceId(p.settlement_account),
  });
  validateProfile(profile);
  return profile;
}

export interface InferenceAskStreamRecord {
  readonly payer: string;
  readonly streamAccount: string;
  readonly recipient: string;
  readonly assetId: string;
  readonly mode: number;
  readonly rate: bigint;
  readonly rateUnit: bigint;
  readonly startMs: bigint;
  readonly lastAccrualMs: bigint;
  readonly endMs: bigint;
  readonly totalCap: bigint;
  readonly accruedTotal: bigint;
  readonly settledTotal: bigint;
  readonly remainderCarry: bigint;
  readonly cumulativeMeter: bigint;
  readonly meterAuthorities: readonly string[];
  readonly underfunded: boolean;
  readonly paused: boolean;
  readonly closed: boolean;
}
export interface InferenceAskStream {
  readonly askId: string;
  readonly streamId: string;
  readonly stream: InferenceAskStreamRecord | null;
}
export interface InferenceRoom extends InferenceRoomView {
  readonly askStreams: readonly InferenceAskStream[];
}
export interface InferenceAttestationBinding {
  readonly family: number;
  readonly modelDigest: string;
  readonly runtimeDigest: string;
  readonly blockHeight: bigint;
}
export interface InferencePeer extends InferencePeerView {
  readonly attestationBinding: InferenceAttestationBinding;
}

function optionalDigest(value: unknown): string {
  if (typeof value !== "string" || !HEX32.test(value)) invalid("digest");
  return value;
}
function decodeStream(value: unknown): InferenceAskStreamRecord {
  const s = object(value);
  const meterAuthorities = Object.freeze(
    rows(s.meter_authorities, 8).map(inferenceId),
  );
  return Object.freeze({
    payer: inferenceId(s.payer),
    streamAccount: inferenceId(s.stream_account),
    recipient: inferenceId(s.recipient),
    assetId: inferenceId(s.asset_id),
    mode: integer(s.mode, 2, 1),
    rate: quantity(s.rate, U128_MAX, true),
    rateUnit: quantity(s.rate_unit, U64_MAX, true),
    startMs: quantity(s.start_ms),
    lastAccrualMs: quantity(s.last_accrual_ms),
    endMs: quantity(s.end_ms),
    totalCap: quantity(s.total_cap, U128_MAX),
    accruedTotal: quantity(s.accrued_total, U128_MAX),
    settledTotal: quantity(s.settled_total, U128_MAX),
    remainderCarry: quantity(s.remainder_carry, U128_MAX),
    cumulativeMeter: quantity(s.cumulative_meter),
    meterAuthorities,
    underfunded: jsonBoolean(s.underfunded),
    paused: jsonBoolean(s.paused),
    closed: jsonBoolean(s.closed),
  });
}

export function decodeInferenceRoom(value: unknown): InferenceRoom {
  const v = object(value);
  const state = roomState(v.state),
    openAsks = jsonIds(v.open_asks, INFERENCE_LIMITS.asks),
    openBids = jsonIds(v.open_bids, INFERENCE_LIMITS.bids);
  if (state === 1 && (openAsks.length > 0 || openBids.length > 0))
    invalid("closed room orders");
  const askStreams = rows(v.ask_streams, INFERENCE_LIMITS.asks).map(
    (value, index): InferenceAskStream => {
      const s = object(value);
      const askId = inferenceId(s.ask_id),
        streamId = optionalDigest(s.stream_id);
      if (
        askId !== openAsks[index] ||
        (streamId === ZERO32) !== (s.stream === null)
      )
        invalid("ask stream binding");
      return Object.freeze({
        askId,
        streamId,
        stream: s.stream === null ? null : decodeStream(s.stream),
      });
    },
  );
  if (askStreams.length !== openAsks.length) invalid("ask stream count");
  return Object.freeze({
    roomId: inferenceId(v.room_id),
    hostKey: inferenceId(v.host_key),
    modelDigest: inferenceId(v.model_digest),
    runtimeDigest: inferenceId(v.runtime_digest),
    profile: jsonProfile(v.profile),
    state,
    openedAtEpoch: quantity(v.opened_at_epoch),
    openAsks,
    openBids,
    askStreams: Object.freeze(askStreams),
  });
}

export function decodeInferenceRooms(value: unknown): readonly InferenceRoom[] {
  const result = rows(value, INFERENCE_LIMITS.rooms).map(decodeInferenceRoom);
  for (let i = 1; i < result.length; i += 1)
    if (result[i - 1]!.roomId >= result[i]!.roomId) invalid("room ordering");
  return Object.freeze(result);
}

export function decodeInferenceDeal(value: unknown): InferenceDealView {
  const v = object(value);
  const layers = Object.freeze(
    rows(v.layers, INFERENCE_LIMITS.dealPeers).map(
      (value): InferenceLayerSplit => {
        const l = object(value);
        return Object.freeze({
          bidId: inferenceId(l.bid_id),
          firstLayer: integer(l.first_layer, 65535),
          endLayer: integer(l.end_layer, 65535, 1),
        });
      },
    ),
  );
  validateLayers(layers);
  return Object.freeze({
    roomId: inferenceId(v.room_id),
    dealId: inferenceId(v.deal_id),
    askId: inferenceId(v.ask_id),
    agreedPricePerThousandTokens: quantity(
      v.agreed_price_per_thousand_tokens,
      U128_MAX,
      true,
    ),
    rosterDigest: inferenceId(v.roster_digest),
    layers,
    state: roomState(v.state),
    settledTokens: quantity(v.settled_tokens),
    settledAmountOwed: quantity(v.settled_amount_owed, U128_MAX),
  });
}

export function decodeInferencePeer(value: unknown): InferencePeer {
  const v = object(value),
    b = object(v.attestation_binding);
  const validFrom = quantity(v.valid_from),
    expiry = quantity(v.expiry);
  if (validFrom >= expiry) invalid("peer validity");
  const status = capabilityStatus(v.capabilities_status);
  let peerCapabilities: InferencePeerCapabilities | null = null;
  if (status === 4) {
    if (v.capabilities !== null) invalid("unavailable capabilities");
  } else {
    const c = object(v.capabilities);
    if (typeof c.region_code !== "string") invalid("region code");
    peerCapabilities = capabilities({
      acceleratorMemoryMib: integer(c.accelerator_memory_mib, 0xffff_ffff),
      systemMemoryMib: integer(c.system_memory_mib, 0xffff_ffff),
      bandwidthClass: integer(c.bandwidth_class, 4),
      regionCode: c.region_code,
      attestationFamily: integer(c.attestation_family, 255),
      runtimeKind: integer(c.runtime_kind, 2),
    });
  }
  if (v.bond_status !== 2) invalid("bond availability");
  return Object.freeze({
    peerKey: inferenceId(v.peer_key),
    worker: inferenceId(v.worker),
    delegate: inferenceId(v.delegate),
    metadata: inferenceId(v.metadata),
    generation: quantity(v.generation, U64_MAX, true),
    keyVersion: quantity(v.key_version, U64_MAX, true),
    metadataRevision: quantity(v.metadata_revision, U64_MAX, true),
    validFrom,
    expiry,
    state: peerState(v.state),
    enrolled: jsonBoolean(v.enrolled),
    capabilitiesStatus: status,
    capabilities: peerCapabilities,
    bondStatus: 2,
    faultCount: integer(v.fault_count, 0xffff_ffff),
    attestationBinding: Object.freeze({
      family: integer(b.family, 255),
      modelDigest: optionalDigest(b.model_digest),
      runtimeDigest: optionalDigest(b.runtime_digest),
      blockHeight: quantity(b.block_height),
    }),
  });
}

export function decodeInferenceReceipt(value: unknown): InferenceReceiptBatch {
  const v = object(value);
  const batch = Object.freeze({
    dealId: inferenceId(v.deal_id),
    firstLapIndex: quantity(v.first_lap_index),
    lastLapIndex: quantity(v.last_lap_index),
    cumulativeTokens: quantity(v.cumulative_tokens),
    batchDigest: inferenceId(v.batch_digest),
    countersignQuorum: integer(v.countersign_quorum, 0xffff_ffff),
  });
  validateBatch(batch);
  return batch;
}

export function decodeInferenceReceipts(
  value: unknown,
): readonly InferenceReceiptBatch[] {
  return Object.freeze(rows(value, 1).map(decodeInferenceReceipt));
}

export function getInferenceRooms(
  client: PxClient,
): Promise<readonly InferenceRoom[]> {
  return client.getInferenceRooms();
}
export function getInferenceRoom(
  client: PxClient,
  roomId: string,
): Promise<InferenceRoom> {
  return client.getInferenceRoom(roomId);
}
export function getInferenceDeal(
  client: PxClient,
  dealId: string,
): Promise<InferenceDealView> {
  return client.getInferenceDeal(dealId);
}
export function getInferencePeer(
  client: PxClient,
  peerKey: string,
): Promise<InferencePeer> {
  return client.getInferencePeer(peerKey);
}
export function getInferenceReceipts(
  client: PxClient,
  dealId: string,
): Promise<readonly InferenceReceiptBatch[]> {
  return client.getInferenceReceipts(dealId);
}
