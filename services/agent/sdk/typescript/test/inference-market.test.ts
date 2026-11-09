import assert from "node:assert/strict";
import { test } from "node:test";
import { once } from "node:events";
import * as http from "node:http";
import * as inference from "../src/inference_market.js";
import * as sdk from "../src/index.js";
import { PxClient } from "../src/index.js";
import {
  PAXAI_OPERATIONS,
  encodeRequestEnvelope,
  decodeRequestEnvelope,
  type RequestEnvelope,
} from "../src/ai_market.js";

function bytes(hex: string): Uint8Array {
  assert.match(hex, /^(?:[0-9a-f]{2})+$/u);
  return new Uint8Array(Buffer.from(hex, "hex"));
}
function hex(value: Uint8Array): string {
  return Buffer.from(value).toString("hex");
}
function id(byte: string): string {
  return byte.repeat(32);
}

const ROOM_WIRE =
  "01010101010101010101010101010101010101010101010101010101010101010202020202020202020202020202020202020202020202020202020202020202" +
  "03030303030303030303030303030303030303030303030303030303030303030404040404040404040404040404040404040404040404040404040404040404" +
  "0000000000000000000000000000006400640000000000000020000000000000000a01f403e80064050505050505050505050505050505050505050505050505" +
  "05050505050505050000000000000000060707070707070707070707070707070707070707070707070707070707070707" +
  "00".repeat(2016) +
  "0808080808080808080808080808080808080808080808080808080808080808" +
  "00".repeat(4064);

const DEAL_WIRE =
  "01010101010101010101010101010101010101010101010101010101010101010909090909090909090909090909090909090909090909090909090909090909" +
  "0707070707070707070707070707070707070707070707070707070707070707000000000000000000000000000009c40a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a" +
  "0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a02010808080808080808080808080808080808080808080808080808080808080808000000100b0b0b0b0b0b0b0b0b0b" +
  "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b00100020" +
  "00".repeat(1080) +
  "002000000000000101000000000000000000000000000007";

const PEER_WIRE =
  "0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d" +
  "0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f0f" +
  "00000000000000010000000000000002000000000000000300000000000000040000000000000064020104000000000000000000000000000200000009";

const RECEIPT_WIRE =
  "09090909090909090909090909090909090909090909090909090909090909090000000000000004000000000000000700200000000000011010101010101010" +
  "10101010101010101010101010101010101010101010101000000002";

const profile: inference.InferenceRoomProfile = Object.freeze({
  minimumBond: 100n,
  canaryRatioBps: 100,
  lapSizeTokens: 32n,
  challengeWindowBlocks: 10n,
  slashFractionBps: 500,
  hostShareBps: 1000,
  protocolShareBps: 100,
  settlementAccount: id("05"),
});
const room: inference.InferenceRoomView = Object.freeze({
  roomId: id("01"),
  hostKey: id("02"),
  modelDigest: id("03"),
  runtimeDigest: id("04"),
  profile,
  state: 0,
  openedAtEpoch: 6n,
  openAsks: [id("07")],
  openBids: [id("08")],
});
const deal: inference.InferenceDealView = Object.freeze({
  roomId: id("01"),
  dealId: id("09"),
  askId: id("07"),
  agreedPricePerThousandTokens: 2500n,
  rosterDigest: id("0a"),
  layers: [
    { bidId: id("08"), firstLayer: 0, endLayer: 16 },
    { bidId: id("0b"), firstLayer: 16, endLayer: 32 },
  ],
  state: 1,
  settledTokens: 9007199254740993n,
  settledAmountOwed: 1329227995784915872903807060280344583n,
});
const peer: inference.InferencePeerView = Object.freeze({
  peerKey: id("0c"),
  worker: id("0d"),
  delegate: id("0e"),
  metadata: id("0f"),
  generation: 1n,
  keyVersion: 2n,
  metadataRevision: 3n,
  validFrom: 4n,
  expiry: 100n,
  state: 2,
  enrolled: true,
  capabilitiesStatus: 4,
  capabilities: null,
  bondStatus: 2,
  faultCount: 9,
});
const receipt: inference.InferenceReceiptBatch = Object.freeze({
  dealId: id("09"),
  firstLapIndex: 4n,
  lastLapIndex: 7n,
  cumulativeTokens: 9007199254740993n,
  batchDigest: id("10"),
  countersignQuorum: 2,
});
const roomJson = {
  room_id: id("01"),
  host_key: id("02"),
  model_digest: id("03"),
  runtime_digest: id("04"),
  profile: {
    minimum_bond: "100",
    canary_ratio_bps: 100,
    lap_size_tokens: "32",
    challenge_window_blocks: "10",
    slash_fraction_bps: 500,
    host_share_bps: 1000,
    protocol_share_bps: 100,
    settlement_account: id("05"),
  },
  state: 0,
  opened_at_epoch: "6",
  open_asks: [id("07")],
  open_bids: [id("08")],
  ask_streams: [{ ask_id: id("07"), stream_id: id("00"), stream: null }],
};
const dealJson = {
  room_id: id("01"),
  deal_id: id("09"),
  ask_id: id("07"),
  agreed_price_per_thousand_tokens: "2500",
  roster_digest: id("0a"),
  layers: [
    { bid_id: id("08"), first_layer: 0, end_layer: 16 },
    { bid_id: id("0b"), first_layer: 16, end_layer: 32 },
  ],
  state: 1,
  settled_tokens: "9007199254740993",
  settled_amount_owed: "1329227995784915872903807060280344583",
};
const peerJson = {
  peer_key: id("0c"),
  worker: id("0d"),
  delegate: id("0e"),
  metadata: id("0f"),
  generation: "1",
  key_version: "2",
  metadata_revision: "3",
  valid_from: "4",
  expiry: "100",
  state: 2,
  enrolled: true,
  capabilities_status: 4,
  capabilities: null,
  bond_status: 2,
  fault_count: 9,
  attestation_binding: {
    family: 0,
    model_digest: id("00"),
    runtime_digest: id("00"),
    block_height: "0",
  },
};
const receiptJson = {
  deal_id: id("09"),
  first_lap_index: "4",
  last_lap_index: "7",
  cumulative_tokens: "9007199254740993",
  batch_digest: id("10"),
  countersign_quorum: 2,
};

function inferenceMarketOpenRoomFixture(): void {
  const wire = bytes(ROOM_WIRE);
  const expected = Buffer.concat([
    wire.subarray(0, 32),
    wire.subarray(64, 200),
  ]);
  assert.deepEqual(Buffer.from(inference.openRoom(room)), expected);
  assert.equal(expected.length, 168);
}
test(inferenceMarketOpenRoomFixture);

function inferenceMarketPostAskFixture(): void {
  const expected =
    id("3c") +
    id("3d") +
    id("32") +
    "00000000000000000000000000000014" +
    "0000000000000000000000000000007b";
  assert.equal(
    hex(
      inference.postAsk({
        roomId: id("3c"),
        askId: id("3d"),
        modelDigest: id("32"),
        maximumPricePerThousandTokens: 20n,
        streamId: 123n,
      }),
    ),
    expected,
  );
  assert.equal(
    inference.postAsk({
      roomId: id("3c"),
      askId: id("3d"),
      modelDigest: id("32"),
      maximumPricePerThousandTokens: 20n,
      streamId: 0n,
    }).length,
    128,
  );
}
test(inferenceMarketPostAskFixture);

function inferenceMarketPostBidFixture(): void {
  const expected =
    id("3c") +
    id("46") +
    id("14") +
    "00000000000000000000000000000005" +
    "00000000000000000000000000000064" +
    "0010";
  assert.equal(
    hex(
      inference.postBid({
        roomId: id("3c"),
        bidId: id("46"),
        capabilityDigest: id("14"),
        minimumPricePerThousandTokens: 5n,
        bondAmount: 100n,
        layerCapacity: 16,
      }),
    ),
    expected,
  );
}
test(inferenceMarketPostBidFixture);

function inferenceMarketRecordDealFixture(): void {
  const wire = bytes(DEAL_WIRE);
  const expected = Buffer.concat([
    wire.subarray(0, 145),
    wire.subarray(146, 218),
  ]);
  assert.deepEqual(Buffer.from(inference.recordDeal(deal)), expected);
  assert.equal(expected.length, 217);
}
test(inferenceMarketRecordDealFixture);

function inferenceMarketChallengeLapFixture(): void {
  assert.equal(
    hex(
      inference.challengeLap({
        deal: id("09"),
        lap: 4n,
        commitment: id("10"),
        deposit: 7n,
      }),
    ),
    id("09") +
      "0000000000000004" +
      id("10") +
      "00000000000000000000000000000007",
  );
}
test(inferenceMarketChallengeLapFixture);

function inferenceMarketResolveChallengeFixture(): void {
  const answer: inference.CanaryLogits = [
    524288, 262144, 65536, 0, -65536, -131072, -262144, -524288,
  ];
  assert.equal(
    hex(
      inference.resolveChallenge({
        deal: id("09"),
        lap: 4n,
        commitment: id("10"),
        answer,
      }),
    ),
    id("09") +
      "0000000000000004" +
      id("10") +
      "00080000000400000001000000000000ffff0000fffe0000fffc0000fff80000",
  );
}
test(inferenceMarketResolveChallengeFixture);

function inferenceMarketCloseRoomFixture(): void {
  assert.equal(hex(inference.closeRoom(id("3c"))), id("3c"));
}
test(inferenceMarketCloseRoomFixture);

function inferenceMarketRequiredEncoderSurface(): void {
  for (const name of [
    "openRoom",
    "postAsk",
    "postBid",
    "recordDeal",
    "settleLaps",
    "challengeLap",
    "resolveChallenge",
    "splitPayout",
    "lockBond",
    "slashBond",
    "refundBond",
    "closeRoom",
  ])
    assert.equal(
      typeof Reflect.get(inference, name),
      "function",
      `${name} requires its authoritative program payload codec and fixtures`,
    );
  const split = inference.splitPayout({
    deal: id("3e"),
    batch: id("63"),
    amount: 400n,
  });
  assert.equal(
    hex(split),
    id("3e") + id("63") + "00000000000000000000000000000190",
    "f11_payout_split.rs exceeding_settled_amount_and_mismatched_batch_refuse_without_credits",
  );
  const lock = inference.lockBond({
    room: id("3c"),
    bid: id("48"),
    amount: 100n,
  });
  assert.equal(
    hex(lock),
    id("3c") + id("48") + "00000000000000000000000000000064",
    "f11_bonds.rs lock_then_bid_passes_and_lock_replay_does_not_deposit_twice",
  );
  const slash = inference.slashBond({
    deal: id("3e"),
    bid: id("46"),
    lap: 0n,
    evidence: id("63"),
  });
  assert.equal(
    hex(slash),
    id("3e") + id("46") + "0000000000000000" + id("63"),
    "f11_bonds.rs double_slash_for_one_lap_refuses_even_with_another_request_or_fault_kind",
  );
  const refund = inference.refundBond({ room: id("3c"), bid: id("46") });
  assert.equal(
    hex(refund),
    id("3c") + id("46"),
    "f11_bonds.rs refund_after_window_returns_remainder_once_and_preserves_reward_claims",
  );
  for (const [name, payload, length] of [
    ["splitPayout", split, 80],
    ["lockBond", lock, 80],
    ["slashBond", slash, 104],
    ["refundBond", refund, 64],
  ] as const) {
    assert.equal(sdk[name], inference[name]);
    assert.equal(payload.length, length);
    const request = envelope(inference.INFERENCE_OPERATIONS[name], payload);
    assert.deepEqual(
      decodeRequestEnvelope(encodeRequestEnvelope(request)).envelope,
      request,
    );
  }
}
test(inferenceMarketRequiredEncoderSurface);

function inferenceMarketPayoutAndBondEncoderBounds(): void {
  const split: inference.SplitPayoutParams = {
    deal: id("3e"),
    batch: id("63"),
    amount: 400n,
  };
  const lock: inference.LockBondParams = {
    room: id("3c"),
    bid: id("48"),
    amount: 100n,
  };
  const slash: inference.SlashBondParams = {
    deal: id("3e"),
    bid: id("46"),
    lap: 0n,
    evidence: id("63"),
  };
  const refund: inference.RefundBondParams = {
    room: id("3c"),
    bid: id("46"),
  };
  for (const amount of [0n, -1n, 1n << 128n]) {
    assert.throws(
      () => inference.splitPayout({ ...split, amount }),
      inference.InferenceMarketError,
    );
    assert.throws(
      () => inference.lockBond({ ...lock, amount }),
      inference.InferenceMarketError,
    );
  }
  for (const lap of [-1n, 1n << 64n])
    assert.throws(
      () => inference.slashBond({ ...slash, lap }),
      inference.InferenceMarketError,
    );
  for (const invalid of [id("00"), id("AB"), "0x" + id("01"), "01"]) {
    for (const field of ["deal", "batch"] as const)
      assert.throws(
        () => inference.splitPayout({ ...split, [field]: invalid }),
        inference.InferenceMarketError,
      );
    for (const field of ["room", "bid"] as const) {
      assert.throws(
        () => inference.lockBond({ ...lock, [field]: invalid }),
        inference.InferenceMarketError,
      );
      assert.throws(
        () => inference.refundBond({ ...refund, [field]: invalid }),
        inference.InferenceMarketError,
      );
    }
    for (const field of ["deal", "bid", "evidence"] as const)
      assert.throws(
        () => inference.slashBond({ ...slash, [field]: invalid }),
        inference.InferenceMarketError,
      );
  }
  assert.equal(
    hex(inference.splitPayout({ ...split, amount: (1n << 128n) - 1n })),
    id("3e") + id("63") + "ff".repeat(16),
  );
  assert.equal(
    hex(inference.lockBond({ ...lock, amount: (1n << 128n) - 1n })),
    id("3c") + id("48") + "ff".repeat(16),
  );
  assert.equal(
    hex(inference.slashBond({ ...slash, lap: (1n << 64n) - 1n })),
    id("3e") + id("46") + "ff".repeat(8) + id("63"),
  );
}
test(inferenceMarketPayoutAndBondEncoderBounds);

function inferenceMarketRecordedViewDecoders(): void {
  assert.deepEqual(inference.decodeRoomView(bytes(ROOM_WIRE)), room);
  assert.deepEqual(inference.decodeDealView(bytes(DEAL_WIRE)), deal);
  assert.deepEqual(inference.decodePeerView(bytes(PEER_WIRE)), peer);
  assert.deepEqual(
    inference.decodeReceiptBatchView(bytes(RECEIPT_WIRE)),
    receipt,
  );
  const joinedRoom = inference.decodeInferenceRoom(roomJson);
  assert.deepEqual(joinedRoom, {
    ...room,
    askStreams: [{ askId: id("07"), streamId: id("00"), stream: null }],
  });
  assert.deepEqual(inference.decodeInferenceRooms([roomJson]), [joinedRoom]);
  assert.deepEqual(inference.decodeInferenceDeal(dealJson), deal);
  assert.deepEqual(inference.decodeInferencePeer(peerJson), {
    ...peer,
    attestationBinding: {
      family: 0,
      modelDigest: id("00"),
      runtimeDigest: id("00"),
      blockHeight: 0n,
    },
  });
  assert.deepEqual(inference.decodeInferenceReceipt(receiptJson), receipt);
  assert.deepEqual(inference.decodeInferenceReceipts([receiptJson]), [receipt]);
}
test(inferenceMarketRecordedViewDecoders);

function inferenceMarketRejectsMalformedViews(): void {
  for (const [wire, decode] of [
    [ROOM_WIRE, inference.decodeRoomView],
    [DEAL_WIRE, inference.decodeDealView],
    [PEER_WIRE, inference.decodePeerView],
    [RECEIPT_WIRE, inference.decodeReceiptBatchView],
  ] as const) {
    const raw = bytes(wire),
      before = raw.slice();
    assert.throws(
      () => decode(raw.subarray(0, raw.length - 1)),
      inference.InferenceMarketError,
    );
    assert.throws(
      () => decode(Buffer.concat([raw, Buffer.from([0])])),
      inference.InferenceMarketError,
    );
    assert.deepEqual(raw, before);
  }
  for (const [wire, offset, value, decode] of [
    [ROOM_WIRE, 200, 2, inference.decodeRoomView],
    [ROOM_WIRE, 200, 1, inference.decodeRoomView],
    [ROOM_WIRE, 273, 1, inference.decodeRoomView],
    [DEAL_WIRE, 145, 2, inference.decodeDealView],
    [DEAL_WIRE, 218, 1, inference.decodeDealView],
    [PEER_WIRE, 170, 2, inference.decodePeerView],
    [PEER_WIRE, 171, 1, inference.decodePeerView],
    [PEER_WIRE, 184, 1, inference.decodePeerView],
    [RECEIPT_WIRE, 47, 3, inference.decodeReceiptBatchView],
  ] as const) {
    const raw = bytes(wire);
    raw[offset] = value;
    assert.throws(() => decode(raw), inference.InferenceMarketError);
  }
  assert.throws(
    () =>
      inference.decodeInferenceDeal({
        ...dealJson,
        settled_tokens: 9007199254740992,
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.decodeInferenceDeal({
        ...dealJson,
        settled_tokens: "18446744073709551616",
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.decodeInferenceReceipt({
        ...receiptJson,
        cumulative_tokens: "01",
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.decodeInferenceRoom({ ...roomJson, ask_streams: [] }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.decodeInferencePeer({ ...peerJson, capabilities: {} }),
    inference.InferenceMarketError,
  );
}
test(inferenceMarketRejectsMalformedViews);

function inferenceMarketEncoderRefusals(): void {
  assert.throws(
    () =>
      inference.openRoom({
        ...room,
        profile: { ...profile, hostShareBps: 9901 },
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.openRoom({ ...room, roomId: id("00") }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.postAsk({
        roomId: id("3c"),
        askId: id("3d"),
        modelDigest: id("32"),
        maximumPricePerThousandTokens: 0n,
        streamId: 1n,
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.postBid({
        roomId: id("3c"),
        bidId: id("46"),
        capabilityDigest: id("14"),
        minimumPricePerThousandTokens: 5n,
        bondAmount: 100n,
        layerCapacity: 0,
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.recordDeal({ ...deal, layers: [] }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.recordDeal({
        ...deal,
        layers: [{ bidId: id("08"), firstLayer: 1, endLayer: 16 }],
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.recordDeal({
        ...deal,
        layers: [
          { bidId: id("08"), firstLayer: 0, endLayer: 16 },
          { bidId: id("08"), firstLayer: 16, endLayer: 32 },
        ],
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.settleLaps({
        batch: receipt,
        artifactManifest: new Uint8Array(385),
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.settleLaps({
        batch: receipt,
        artifactManifest: new Uint8Array(4097),
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.settleLaps({
        batch: { ...receipt, lastLapIndex: 68n },
        artifactManifest: new Uint8Array(386),
      }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.challengeLap({
        deal: id("09"),
        lap: -1n,
        commitment: id("10"),
        deposit: 7n,
      }),
    inference.InferenceMarketError,
  );
}
test(inferenceMarketEncoderRefusals);

function envelope(selector: number, payload: Uint8Array): RequestEnvelope {
  const spec = PAXAI_OPERATIONS.get(selector);
  assert.ok(spec);
  const read = spec.boundary === "program-read";
  return {
    selector,
    chain: id("01"),
    program: id("02"),
    market: id("03"),
    actor: id("04"),
    epoch: read ? 0n : 6n,
    config: read ? 0n : 1n,
    roster: read ? null : id("05"),
    sequence: spec.sequence === "role" ? 1n : 0n,
    expiry: 1024n,
    request: id("06"),
    payload,
    delegated: false,
  };
}

function inferenceMarketF11EnvelopeIntegration(): void {
  for (const [name, selector] of Object.entries(
    inference.INFERENCE_OPERATIONS,
  )) {
    const spec = PAXAI_OPERATIONS.get(selector);
    assert.ok(spec, `${name} missing from signing registry`);
    assert.equal(spec.feature, 11);
    assert.equal(spec.delegation, "native-only");
  }
  const open = envelope(
    inference.INFERENCE_OPERATIONS.openRoom,
    inference.openRoom(room),
  );
  assert.deepEqual(
    decodeRequestEnvelope(encodeRequestEnvelope(open)).envelope,
    open,
  );
  const ask = envelope(
    inference.INFERENCE_OPERATIONS.postAsk,
    inference.postAsk({
      roomId: room.roomId,
      askId: id("07"),
      modelDigest: room.modelDigest,
      maximumPricePerThousandTokens: 20n,
      streamId: 123n,
    }),
  );
  assert.equal(ask.sequence, 0n);
  assert.deepEqual(
    decodeRequestEnvelope(encodeRequestEnvelope(ask)).envelope,
    ask,
  );
  for (const selector of [0x0b10, 0x0b11, 0x0b12, 0x0b13, 0x0b14]) {
    const payload =
      selector === 0x0b13 ? bytes(id("09") + id("10")) : bytes(id("01"));
    const read = envelope(selector, payload);
    assert.deepEqual(
      decodeRequestEnvelope(encodeRequestEnvelope(read)).envelope,
      read,
    );
    assert.throws(() => encodeRequestEnvelope({ ...read, config: 1n }));
    assert.throws(() => encodeRequestEnvelope({ ...read, roster: id("05") }));
  }
  assert.throws(() => encodeRequestEnvelope({ ...open, sequence: 0n }));
  assert.throws(() => encodeRequestEnvelope({ ...open, delegated: true }));
  assert.throws(() => encodeRequestEnvelope({ ...open, roster: null }));
}
test(inferenceMarketF11EnvelopeIntegration);

async function inferenceMarketPxRootMethods(): Promise<void> {
  const transcript = [
    { method: "px_getInferenceRooms", params: [] },
    { method: "px_getInferenceRoom", params: [id("01")] },
    { method: "px_getInferenceDeal", params: [id("09")] },
    { method: "px_getInferencePeer", params: [id("0c")] },
    { method: "px_getInferenceReceipts", params: [id("09")] },
  ];
  const requests: unknown[] = [];
  const server = http.createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      const input = JSON.parse(Buffer.concat(chunks).toString("utf8"));
      requests.push(input);
      response.destroy();
    });
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert.ok(address && typeof address === "object");
  const client = new PxClient({
    endpoint: `http://127.0.0.1:${address.port}/`,
  });
  try {
    await assert.rejects(inference.getInferenceRooms(client), {
      code: "ECONNRESET",
    });
    await assert.rejects(inference.getInferenceRoom(client, id("01")), {
      code: "ECONNRESET",
    });
    await assert.rejects(inference.getInferenceDeal(client, id("09")), {
      code: "ECONNRESET",
    });
    await assert.rejects(inference.getInferencePeer(client, id("0c")), {
      code: "ECONNRESET",
    });
    await assert.rejects(inference.getInferenceReceipts(client, id("09")), {
      code: "ECONNRESET",
    });
    assert.deepEqual(
      requests,
      transcript.map((entry, i) => ({
        jsonrpc: "2.0",
        id: String(i + 1),
        method: entry.method,
        params: entry.params,
      })),
    );
    for (const invalid of [id("00"), "0x" + id("01"), id("AB"), " " + id("01")])
      await assert.rejects(
        client.getInferenceRoom(invalid),
        inference.InferenceMarketError,
      );
    assert.equal(requests.length, 5);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
}
test(inferenceMarketPxRootMethods);

function inferenceMarketFundAskAbi(): void {
  const params: inference.FundAskParams = {
    askRef: id("a7"),
    settlementAccount: "0x" + "11".repeat(20),
    rate: 3n,
    rateUnit: 2,
    totalCap: 100n,
    hostAuthorityKey: id("22"),
  };
  const call = inference.fundAsk(params);
  assert.deepEqual(call, {
    to: "0x000000000000000000000000000000000000101b",
    value: 0n,
    data:
      "0x382e3492" +
      id("a7") +
      "00".repeat(12) +
      "11".repeat(20) +
      "0000000000000000000000000000000000000000000000000000000000000003" +
      "0000000000000000000000000000000000000000000000000000000000000002" +
      "0000000000000000000000000000000000000000000000000000000000000064" +
      id("22"),
  });
  assert.throws(
    () => inference.fundAsk({ ...params, rateUnit: 0 }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.fundAsk({ ...params, rateUnit: 256 }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.fundAsk({ ...params, rate: 1n << 128n }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.fundAsk({ ...params, hostAuthorityKey: id("00") }),
    inference.InferenceMarketError,
  );
  assert.throws(
    () =>
      inference.fundAsk({
        ...params,
        settlementAccount: "0x" + "00".repeat(20),
      }),
    inference.InferenceMarketError,
  );
}
test(inferenceMarketFundAskAbi);

function inferenceMarketBookView(): void {
  const empty = bytes("463131500001" + id("32") + "0000" + "00".repeat(512));
  assert.deepEqual(inference.decodeBookView(empty), {
    modelDigest: id("32"),
    asks: [],
    bids: [],
    crossings: [],
  });
  const book = bytes(
    "463131500001" +
      id("32") +
      "0101" +
      id("3c") +
      id("3d") +
      "00000000000000000000000000000014" +
      "0000000000000000000000000000007b" +
      id("3c") +
      id("46") +
      "00000000000000000000000000000005" +
      "01" +
      "00".repeat(511),
  );
  assert.deepEqual(inference.decodeBookView(book), {
    modelDigest: id("32"),
    asks: [
      {
        roomId: id("3c"),
        orderId: id("3d"),
        pricePerThousandTokens: 20n,
        streamId: 123n,
      },
    ],
    bids: [
      {
        roomId: id("3c"),
        orderId: id("46"),
        pricePerThousandTokens: 5n,
        streamId: 0n,
      },
    ],
    crossings: [[0, 0]],
  });
  const wrongCrossing = book.slice();
  wrongCrossing[216] = 0;
  assert.throws(
    () => inference.decodeBookView(wrongCrossing),
    inference.InferenceMarketError,
  );
  const unusedCrossing = book.slice();
  unusedCrossing[217] = 1;
  assert.throws(
    () => inference.decodeBookView(unusedCrossing),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.decodeBookView(book.subarray(0, book.length - 1)),
    inference.InferenceMarketError,
  );
  assert.throws(
    () => inference.decodeBookView(Buffer.concat([book, Buffer.from([0])])),
    inference.InferenceMarketError,
  );
}
test(inferenceMarketBookView);
