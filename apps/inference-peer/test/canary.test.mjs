import assert from "node:assert/strict";
import { createHash, createPublicKey, verify } from "node:crypto";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import test from "node:test";
import {
  canaryAnswer,
  CanaryTracker,
  encodeFixedPointLogits,
  validateCanaryPlan,
} from "../src/canary.mjs";
import {
  decodeSignalFrame,
  encodeSignalFrame,
} from "../src/signal.mjs";
import { PeerIdentity } from "../src/identity.mjs";
import { SignalClient } from "../src/transport.mjs";
import {
  fixedIdentity,
  manifest,
  RoomHost,
  roomId,
} from "./host-signal.mjs";

const fixture = JSON.parse(
  readFileSync(
    new URL("fixtures/host-canary-frames.json", import.meta.url),
  ),
);
const lapFixture = JSON.parse(
  readFileSync(
    new URL("fixtures/host-lap-frames.json", import.meta.url),
  ),
);
function deal() {
  return JSON.parse(
    lapFixture.session.find((item) => item.name === "deal").json,
  ).deal;
}
function integer(value, size) {
  const bytes = Buffer.alloc(size);
  if (size === 8) bytes.writeBigUInt64BE(BigInt(value));
  else bytes.writeUInt32BE(value);
  return bytes;
}
function domain(name, ...parts) {
  return createHash("sha256")
    .update(
      Buffer.concat([
        Buffer.from(name + "\0"),
        ...parts.map((part) => Buffer.from(part)),
      ]),
    )
    .digest();
}
function plan({ position = 0, lap = 0 } = {}) {
  const binding = deal();
  const prompt = [3, 5, 7];
  const promptDigest = domain(
    "PAXAI/canary-prompt/v1",
    binding.model_digest,
    integer(position, 8),
    ...prompt.map((token) => integer(token, 4)),
  );
  return {
    deal_id: binding.deal_id,
    lap_index: lap,
    generated_tokens: 256,
    position,
    prompt_tokens: prompt,
    peer_key: Array.from(Buffer.from(lapFixture.peer_key, "hex")),
    answer_context: Array.from(
      Buffer.concat([
        ...["chain", "program", "market"].map((key) =>
          Buffer.from(binding[key]),
        ),
        integer(binding.epoch, 8),
        integer(binding.config, 8),
        Buffer.from(binding.roster_digest),
        Buffer.from(binding.deal_id),
        integer(lap, 8),
        Buffer.alloc(32, 0x20), // Frozen worker ID; distinct from the peer public key.
        promptDigest,
      ]),
    ),
  };
}
async function tracker() {
  return new CanaryTracker({
    identity: await fixedIdentity(),
    deal: deal(),
  });
}
function decoded(bytes) {
  const view = new DataView(
    bytes.buffer,
    bytes.byteOffset,
    bytes.byteLength,
  );
  return Array.from({ length: 8 }, (_, i) =>
    view.getInt32(i * 4, false),
  );
}
function publicKey() {
  return createPublicKey({
    key: Buffer.concat([
      Buffer.from("302a300506032b6570032100", "hex"),
      Buffer.from(lapFixture.peer_key, "hex"),
    ]),
    format: "der",
    type: "spki",
  });
}

function fixed_point_logits_equal_program_fixture_bytes() {
  const input = Float32Array.from(fixture.program_codec.logits);
  const original = input.slice();
  const encoded = encodeFixedPointLogits(input);
  assert.equal(
    Buffer.from(encoded).toString("hex"),
    fixture.program_codec.encoded_hex,
  );
  assert.deepEqual(decoded(encoded), fixture.program_codec.fixed);
  assert.deepEqual(
    input,
    original,
    "selection must not reorder model output",
  );
  assert.deepEqual(encodeFixedPointLogits(Array.from(input)), encoded);
}

function q16_rounding_limits_ties_and_nonfinite_logits_are_checked() {
  const half = 1 / 131072;
  assert.deepEqual(
    decoded(
      encodeFixedPointLogits([
        half,
        -half,
        1.25,
        -1.25,
        0,
        -0,
        0.5,
        -0.5,
      ]),
    ),
    [81920, 32768, 1, 0, 0, -1, -32768, -81920],
  );
  assert.deepEqual(
    decoded(encodeFixedPointLogits(Array(12).fill(1))),
    Array(8).fill(65536),
  );
  const limits = decoded(
    encodeFixedPointLogits([
      32767 + 65535 / 65536,
      1,
      0,
      -1,
      -2,
      -3,
      -4,
      -32768,
    ]),
  );
  assert.equal(limits[0], 2147483647);
  assert.equal(limits[7], -2147483648);
  for (const value of [32768, -32768 - 1 / 65536]) {
    const values =
      value > 0 ? [value, ...Array(7).fill(0)] : Array(8).fill(value);
    assert.throws(() => encodeFixedPointLogits(values), /overflow/);
  }
  for (const value of [NaN, Infinity, -Infinity, "1", null])
    assert.throws(
      () => encodeFixedPointLogits([...Array(8).fill(0), value]),
      /finite logits/,
    );
  assert.throws(
    () => encodeFixedPointLogits(Array(7).fill(0)),
    /eight logits/,
  );
  assert.throws(
    () => encodeFixedPointLogits(new Array(8)),
    /finite logits/,
  );
  assert.throws(
    () => encodeFixedPointLogits(new Uint8Array(8)),
    /eight logits/,
  );
}

async function canary_answer_for_unknown_position_refuses() {
  const canaries = await tracker();
  assert.deepEqual(await canaryAnswer(plan(), canaries), {
    type: "refusal",
    reason: "unknown_position",
  });
  canaries.recordLogits({
    lapIndex: 0,
    position: 0,
    logits: fixture.program_codec.logits,
  });
  assert.deepEqual(
    await canaryAnswer(plan({ position: 1 }), canaries),
    {
      type: "refusal",
      reason: "unknown_position",
    },
  );
  assert.deepEqual(await canaryAnswer(plan({ lap: 1 }), canaries), {
    type: "refusal",
    reason: "unknown_position",
  });
}

async function canary_answer_signs_the_program_context_and_q16_digest() {
  const canaries = await tracker();
  const logits = Float64Array.from(fixture.program_codec.logits);
  canaries.recordLogits({ lapIndex: 0, position: 0, logits });
  logits.fill(0);
  const request = plan();
  const answer = await canaryAnswer(request, canaries);
  assert.deepEqual(Object.keys(answer), [
    "deal_id",
    "lap_index",
    "logits",
    "signature",
  ]);
  assert.deepEqual(answer.deal_id, request.deal_id);
  assert.equal(answer.lap_index, request.lap_index);
  assert.deepEqual(answer.logits, fixture.program_codec.fixed);
  const logitsDigest = domain(
    "PAXAI/canary-logits/q16/v1",
    Buffer.from(fixture.program_codec.encoded_hex, "hex"),
  );
  const digest = domain(
    "PAXAI/canary-answer/v1",
    request.answer_context,
    logitsDigest,
  );
  assert.ok(
    verify(null, digest, publicKey(), Buffer.from(answer.signature)),
  );
  assert.equal(
    verify(
      null,
      logitsDigest,
      publicKey(),
      Buffer.from(answer.signature),
    ),
    false,
  );
  assert.deepEqual(await canaryAnswer(request, canaries), answer);
  const expected = structuredClone(answer);
  answer.logits.fill(0);
  answer.signature.fill(0);
  answer.deal_id.fill(0);
  assert.deepEqual(await canaryAnswer(request, canaries), expected);
}

async function mismatched_deal_peer_prompt_and_context_refuse() {
  const canaries = await tracker();
  canaries.recordLogits({
    lapIndex: 0,
    position: 0,
    logits: fixture.program_codec.logits,
  });
  for (const [reason, change] of [
    [
      "deal_mismatch",
      (value) => {
        value.deal_id[0] ^= 1;
      },
    ],
    [
      "peer_mismatch",
      (value) => {
        value.peer_key[0] ^= 1;
      },
    ],
    [
      "answer_context_mismatch",
      (value) => {
        value.answer_context[0] ^= 1;
      },
    ],
    [
      "answer_context_mismatch",
      (value) => {
        value.answer_context[176] ^= 1;
      },
    ],
    [
      "prompt_mismatch",
      (value) => {
        value.prompt_tokens[0] ^= 1;
      },
    ],
    [
      "prompt_mismatch",
      (value) => {
        value.answer_context[216] ^= 1;
      },
    ],
  ]) {
    const request = plan();
    change(request);
    assert.deepEqual(await canaryAnswer(request, canaries), {
      type: "refusal",
      reason,
    });
  }
  assert.deepEqual(
    await canaryAnswer(
      {
        ...plan(),
        position: null,
        peer_key: null,
        prompt_tokens: [],
        answer_context: [],
      },
      canaries,
    ),
    {
      type: "refusal",
      reason: "no_canary_position",
    },
  );
  const pendingPlan = plan();
  const pending = canaryAnswer(pendingPlan, canaries);
  pendingPlan.answer_context.fill(0);
  assert.deepEqual((await pending).logits, fixture.program_codec.fixed);
  canaries.close();
  assert.deepEqual(await canaryAnswer(plan(), canaries), {
    type: "refusal",
    reason: "tracker_closed",
  });
  assert.throws(
    () =>
      canaries.recordLogits({
        lapIndex: 0,
        position: 1,
        logits: fixture.program_codec.logits,
      }),
    /closed/,
  );
}

async function invalid_plans_and_conflicting_position_records_refuse() {
  const canaries = await tracker();
  for (const change of [
    (value) => {
      value.position = 256;
    },
    (value) => {
      value.lap_index = -1;
    },
    (value) => {
      value.generated_tokens = 0;
    },
    (value) => {
      value.position = Number.MAX_SAFE_INTEGER + 1;
    },
    (value) => {
      value.lap_index = 1n << 64n;
    },
    (value) => {
      value.prompt_tokens = [-1];
    },
    (value) => {
      value.prompt_tokens = [4294967296];
    },
    (value) => {
      value.answer_context.pop();
    },
    (value) => {
      value.answer_context = "00".repeat(248);
    },
    (value) => {
      value.deal_id = "00".repeat(32);
    },
    (value) => {
      value.peer_key[0] = 256;
    },
    (value) => {
      value.future = true;
    },
    (value) => {
      delete value.peer_key;
    },
  ]) {
    const request = plan();
    change(request);
    assert.throws(() => validateCanaryPlan(request));
    const answer = await canaryAnswer(request, canaries);
    assert.equal(answer.type, "refusal");
    assert.match(answer.reason, /^malformed_plan:/);
  }
  canaries.recordLogits({
    lapIndex: 0,
    position: 0,
    logits: fixture.program_codec.logits,
  });
  canaries.recordLogits({
    lapIndex: 0n,
    position: 0n,
    logits: fixture.program_codec.logits,
  });
  assert.throws(
    () =>
      canaries.recordLogits({
        lapIndex: 0,
        position: 0,
        logits: Array(8).fill(0),
      }),
    /Conflicting/,
  );
  assert.throws(
    () =>
      canaries.recordLogits({
        lapIndex: -1,
        position: 0,
        logits: Array(8).fill(0),
      }),
    /u64/,
  );
  assert.deepEqual(
    (await canaryAnswer(plan(), canaries)).logits,
    fixture.program_codec.fixed,
  );
  assert.throws(() => canaryAnswer(plan(), {}), /CanaryTracker/);
}

async function host_http_payloads_cannot_be_sent_as_signalling_frames() {
  const canaries = await tracker();
  canaries.recordLogits({
    lapIndex: 0,
    position: 0,
    logits: fixture.program_codec.logits,
  });
  const request = plan();
  const answer = await canaryAnswer(request, canaries);
  assert.throws(
    () => decodeSignalFrame(JSON.stringify(request), "server"),
    /Unknown signalling type/,
  );
  assert.throws(
    () => encodeSignalFrame(answer, "client"),
    /Unknown signalling type/,
  );
  assert.throws(
    () => decodeSignalFrame(new Uint8Array(32)),
    /text frame/,
  );
}

function host_canary_socket_vectors_cover_both_directions() {
  for (const direction of ["server", "client"]) {
    const vectors = fixture.vectors.filter(
      (vector) => vector.direction === direction,
    );
    assert.ok(
      vectors.length > 0,
      `Missing producer-defined ${direction} canary signalling vector`,
    );
    for (const vector of vectors) {
      const frame = decodeSignalFrame(vector.json, direction);
      assert.equal(encodeSignalFrame(frame, direction), vector.json);
      assert.throws(
        () =>
          encodeSignalFrame(
            frame,
            direction === "server" ? "client" : "server",
          ),
        /Unknown signalling type/,
      );
      assert.throws(
        () => encodeSignalFrame({ ...frame, future: true }, direction),
        /Unknown member/,
      );
      const member = direction === "server" ? "plan" : "answer";
      for (const key of Object.keys(frame[member])) {
        const missing = structuredClone(frame);
        delete missing[member][key];
        assert.throws(() => encodeSignalFrame(missing, direction));
      }
      const unknown = structuredClone(frame);
      unknown[member].future = true;
      assert.throws(() => encodeSignalFrame(unknown, direction));
    }
  }
}

function canaryVector(direction) {
  const vector = fixture.vectors.find(
    (item) => item.direction === direction,
  );
  assert.ok(vector);
  return decodeSignalFrame(vector.json, direction);
}

// Decode the frozen binding from the host's produced answer context. This is
// local tracker configuration; the host sends the committed JSON unchanged.
function socketBinding() {
  const request = canaryVector("server").plan;
  const context = Buffer.from(request.answer_context);
  const binding = deal();
  for (const [key, offset] of [
    ["chain", 0],
    ["program", 32],
    ["market", 64],
    ["roster_digest", 112],
  ])
    binding[key] = Array.from(context.subarray(offset, offset + 32));
  binding.epoch = context.readBigUInt64BE(96);
  binding.config = context.readBigUInt64BE(104);
  binding.deal_id = request.deal_id.slice();
  binding.model_digest = Array(32).fill(1); // inference_canary_frames.rs::table
  binding.plan.layers[1].peer_key = request.peer_key.slice();
  return binding;
}

async function socketIdentity() {
  const expected = canaryVector("server").plan.peer_key;
  // Real Rust fixture delegates use the Ed25519 seeds [0x90 + n; 32].
  for (const seed of [0xa4, 0xa5]) {
    const privateKey = await crypto.subtle.importKey(
      "pkcs8",
      Buffer.concat([
        Buffer.from("302e020100300506032b657004220420", "hex"),
        Buffer.alloc(32, seed),
      ]),
      "Ed25519",
      true,
      ["sign"],
    );
    const jwk = await crypto.subtle.exportKey("jwk", privateKey);
    const key = Buffer.from(jwk.x, "base64url");
    if (key.equals(Buffer.from(expected)))
      return new PeerIdentity(privateKey, key.toString("hex"));
  }
  assert.fail(
    "The host canary vector must select a producer fixture delegate",
  );
}

async function socketSession(
  t,
  { identity, binding, tracking = true } = {},
) {
  identity ??= await socketIdentity();
  const canaries = tracking
    ? new CanaryTracker({ identity, deal: binding ?? socketBinding() })
    : undefined;
  const host = new RoomHost();
  const received = [],
    failures = [];
  let connection;
  const url = await host.listen(t, (client, frame) => {
    connection = client;
    if (frame.type === "hello") {
      const offer = lapFixture.session.find(
        (item) => item.name === "offer",
      );
      host.sendText(client.socket, offer.json);
    } else {
      assert.deepEqual(frame, canaryVector("client"));
      assert.equal(
        encodeSignalFrame(frame, "client"),
        fixture.vectors.find((item) => item.direction === "client")
          .json,
      );
      received.push(frame);
    }
  });
  const signal = new SignalClient(url, {
    roomId,
    identity,
    manifest,
    canaryTracker: canaries,
  });
  signal.addEventListener("error", ({ detail }) =>
    failures.push(detail),
  );
  t.after(() => signal.close());
  await signal.connect();
  t.after(() => assert.deepEqual(failures, []));
  function request() {
    // The stand-in never invents or mutates a canary frame.
    host.sendText(
      connection.socket,
      fixture.vectors.find((item) => item.direction === "server").json,
    );
  }
  async function close() {
    const closed = once(connection.socket, "close");
    signal.close();
    await closed;
  }
  return { host, signal, canaries, received, request, close };
}

async function canary_requests_are_answered_as_host_vector_text_frames(
  t,
) {
  const session = await socketSession(t);
  const request = canaryVector("server").plan;
  session.signal.recordCanaryLogits({
    lapIndex: request.lap_index,
    position: request.position,
    logits: fixture.program_codec.logits,
  });
  for (let i = 0; i < 2; i++) {
    const received = once(session.host, "frame");
    session.request();
    const [, frame] = await received;
    assert.deepEqual(frame, canaryVector("client"));
    const digest = domain(
      "PAXAI/canary-answer/v1",
      request.answer_context,
      domain(
        "PAXAI/canary-logits/q16/v1",
        Buffer.from(fixture.program_codec.encoded_hex, "hex"),
      ),
    );
    const key = createPublicKey({
      key: Buffer.concat([
        Buffer.from("302a300506032b6570032100", "hex"),
        Buffer.from(request.peer_key),
      ]),
      format: "der",
      type: "spki",
    });
    assert.ok(
      verify(null, digest, key, Buffer.from(frame.answer.signature)),
    );
  }
  assert.equal(session.received.length, 2);
  await session.close();
  assert.deepEqual(await canaryAnswer(request, session.canaries), {
    type: "refusal",
    reason: "tracker_closed",
  });
  assert.throws(
    () =>
      session.signal.recordCanaryLogits({
        lapIndex: 0,
        position: 0,
        logits: fixture.program_codec.logits,
      }),
    /No active/,
  );
}

async function unknown_socket_position_refuses_locally_and_can_be_answered_later(
  t,
) {
  const session = await socketSession(t);
  const refused = once(session.signal, "canary-refusal");
  session.request();
  const [{ detail }] = await refused;
  assert.deepEqual(detail, {
    type: "refusal",
    reason: "unknown_position",
  });
  assert.equal(session.signal.state, "offered");
  assert.equal(session.received.length, 0);
  const request = canaryVector("server").plan;
  session.signal.recordCanaryLogits({
    lapIndex: request.lap_index,
    position: request.position,
    logits: fixture.program_codec.logits,
  });
  const received = once(session.host, "frame");
  session.request();
  await received;
  await session.close();
  assert.equal(
    session.received.length,
    1,
    "refusal must not send a frame before the answer",
  );
}

async function refused_socket_bindings_send_no_answer(t) {
  const wrongBinding = socketBinding();
  wrongBinding.epoch++;
  const session = await socketSession(t, { binding: wrongBinding });
  const request = canaryVector("server").plan;
  session.signal.recordCanaryLogits({
    lapIndex: request.lap_index,
    position: request.position,
    logits: fixture.program_codec.logits,
  });
  const refused = once(session.signal, "canary-refusal");
  session.request();
  const [{ detail }] = await refused;
  assert.deepEqual(detail, {
    type: "refusal",
    reason: "answer_context_mismatch",
  });
  await session.close();
  assert.equal(session.received.length, 0);
}

async function foreign_peer_and_unavailable_tracker_refuse_on_the_socket(
  t,
) {
  for (const options of [
    { identity: await fixedIdentity(), binding: deal() },
    { tracking: false },
  ]) {
    const session = await socketSession(t, options);
    const refused = once(session.signal, "canary-refusal");
    session.request();
    const [{ detail }] = await refused;
    assert.deepEqual(detail, {
      type: "refusal",
      reason:
        options.tracking === false
          ? "canary_unavailable"
          : "peer_mismatch",
    });
    await session.close();
    assert.equal(session.received.length, 0);
  }
  const identity = await fixedIdentity();
  const foreign = new CanaryTracker({
    identity: await socketIdentity(),
    deal: socketBinding(),
  });
  assert.throws(
    () =>
      new SignalClient("ws://127.0.0.1/", {
        roomId,
        identity,
        manifest,
        canaryTracker: foreign,
      }),
    /signalling identity/,
  );
}

async function received_deals_replace_canary_records_and_socket_close_clears_them(
  t,
) {
  const identity = await fixedIdentity(),
    previous = await tracker();
  previous.recordLogits({
    lapIndex: 0,
    position: 0,
    logits: fixture.program_codec.logits,
  });
  const host = new RoomHost();
  let connection;
  function send(name) {
    const vector = lapFixture.session.find(
      (item) => item.name === name,
    );
    assert.equal(vector.direction, "server");
    host.sendText(connection.socket, vector.json);
  }
  const url = await host.listen(t, (client, frame) => {
    connection = client;
    if (frame.type === "hello") send("offer");
    else {
      assert.deepEqual(
        frame,
        JSON.parse(
          lapFixture.session.find((item) => item.name === "deal_ack")
            .json,
        ),
      );
      send("deal_live");
    }
  });
  const signal = new SignalClient(url, {
    roomId,
    identity,
    manifest,
    canaryTracker: previous,
  });
  t.after(() => signal.close());
  signal.addEventListener("frame", ({ detail }) => {
    if (detail.type === "deal")
      signal.acknowledgeDeal(
        JSON.parse(
          lapFixture.session.find((item) => item.name === "deal_ack")
            .json,
        ),
      );
  });
  await signal.connect();
  const live = new Promise((resolve) =>
    signal.addEventListener("frame", ({ detail }) => {
      if (detail.type === "deal_live") resolve();
    }),
  );
  send("deal");
  await live;
  assert.equal(signal.state, "ready");
  assert.deepEqual(await canaryAnswer(plan(), previous), {
    type: "refusal",
    reason: "tracker_closed",
  });
  signal.recordCanaryLogits({
    lapIndex: 0,
    position: 0,
    logits: fixture.program_codec.logits,
  });
  signal.close();
  assert.throws(
    () =>
      signal.recordCanaryLogits({
        lapIndex: 0,
        position: 1,
        logits: fixture.program_codec.logits,
      }),
    /No active/,
  );
}

function canary_codec_enforces_fixed_width_values_and_exact_frame_members() {
  const request = canaryVector("server"),
    answer = canaryVector("client");
  assert.deepEqual(
    new Set(fixture.vectors.map((item) => JSON.parse(item.json).type)),
    new Set(["canary_request", "canary_answer"]),
  );
  for (const value of [
    -2147483649,
    2147483648,
    NaN,
    Infinity,
    1.5,
    "1",
    1n,
  ]) {
    const invalid = structuredClone(answer);
    invalid.answer.logits[0] = value;
    assert.throws(() => encodeSignalFrame(invalid, "client"), /i32/);
  }
  const sparse = structuredClone(answer);
  sparse.answer.logits = new Array(8);
  assert.throws(() => encodeSignalFrame(sparse, "client"), /i32/);
  for (const count of [7, 9]) {
    const invalid = structuredClone(answer);
    invalid.answer.logits = Array(count).fill(0);
    assert.throws(() => encodeSignalFrame(invalid, "client"), /eight/);
  }
  for (const member of ["deal_id", "signature"]) {
    const invalid = structuredClone(answer);
    invalid.answer[member].pop();
    assert.throws(
      () => encodeSignalFrame(invalid, "client"),
      /byte array/,
    );
  }
  const max = (1n << 64n) - 1n;
  request.plan.lap_index = max;
  answer.answer.lap_index = max;
  assert.equal(
    decodeSignalFrame(encodeSignalFrame(request, "server")).plan
      .lap_index,
    max,
  );
  assert.equal(
    decodeSignalFrame(encodeSignalFrame(answer, "client"), "client")
      .answer.lap_index,
    max,
  );
}

for (const criterion of [
  fixed_point_logits_equal_program_fixture_bytes,
  q16_rounding_limits_ties_and_nonfinite_logits_are_checked,
  canary_answer_for_unknown_position_refuses,
  canary_answer_signs_the_program_context_and_q16_digest,
  mismatched_deal_peer_prompt_and_context_refuse,
  invalid_plans_and_conflicting_position_records_refuse,
  host_http_payloads_cannot_be_sent_as_signalling_frames,
  host_canary_socket_vectors_cover_both_directions,
  canary_requests_are_answered_as_host_vector_text_frames,
  unknown_socket_position_refuses_locally_and_can_be_answered_later,
  refused_socket_bindings_send_no_answer,
  foreign_peer_and_unavailable_tracker_refuse_on_the_socket,
  received_deals_replace_canary_records_and_socket_close_clears_them,
  canary_codec_enforces_fixed_width_values_and_exact_frame_members,
])
  test(criterion.name, { timeout: 15000 }, criterion);
