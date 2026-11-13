// Regenerate the fixed vectors; no peer codec or signing implementation is used.
// Layouts: inference_market/laps.rs and programs/.../src/{laps,codec}.rs.
import {
  createHash,
  createPrivateKey,
  createPublicKey,
  sign,
} from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import {
  ScriptedModel,
  scriptedEmbedding,
  scriptedModelSource,
} from "../../tests/scripted_model.mjs";

const base = JSON.parse(
  readFileSync(new URL("host-signal-frames.json", import.meta.url)),
);
const fixed = (n) => Array(32).fill(n);
const hex = (bytes) => Buffer.from(bytes).toString("hex");
const sha = (...parts) =>
  createHash("sha256")
    .update(Buffer.concat(parts.map((part) => Buffer.from(part))))
    .digest();
const domain = (name, ...parts) => sha(Buffer.from(name + "\0"), ...parts);
function integer(value, length) {
  const out = Buffer.alloc(length);
  let n = BigInt(value);
  for (let i = length - 1; i >= 0; i--) {
    out[i] = Number(n & 255n);
    n >>= 8n;
  }
  return out;
}
function key(seed) {
  const pkcs8 = Buffer.concat([
    Buffer.from("302e020100300506032b657004220420", "hex"),
    seed,
  ]);
  const privateKey = createPrivateKey({
    key: pkcs8,
    type: "pkcs8",
    format: "der",
  });
  const publicKey = createPublicKey(privateKey)
    .export({ format: "der", type: "spki" })
    .subarray(-32);
  return { privateKey, publicKey, pkcs8 };
}
const peer = key(Buffer.from(base.key.seed_hex, "hex"));
const host = key(Buffer.alloc(32, 12));
const previous = key(Buffer.alloc(32, 40));
const source = await scriptedModelSource();
const deal = JSON.parse(
  base.vectors.find((v) => v.name === "deal").json,
).deal;
deal.model_digest = Array.from(source.digest);
deal.plan.layers = [
  {
    peer_key: Array.from(previous.publicKey),
    owner: fixed(11),
    bid_id: fixed(13),
    manifest_digest: fixed(15),
    first_layer: 0,
    end_layer: 2,
  },
  {
    peer_key: Array.from(peer.publicKey),
    owner: fixed(12),
    bid_id: fixed(14),
    manifest_digest: fixed(16),
    first_layer: 2,
    end_layer: 4,
  },
];
const binding = domain(
  "PAXAI/inference-deal/v1",
  ...[
    "chain",
    "program",
    "market",
    "room_id",
    "model_digest",
    "runtime_digest",
    "deal_id",
    "ask_id",
    "roster_digest",
  ].map((name) => deal[name]),
  ...["epoch", "config", "round"].map((name) => integer(deal[name], 8)),
  integer(deal.price, 16),
  deal.plan.embedding_host,
  deal.plan.head_host,
  integer(4, 2),
  integer(2, 2),
  ...deal.plan.layers.flatMap((split) => [
    split.peer_key,
    split.owner,
    split.bid_id,
    split.manifest_digest,
    integer(split.first_layer, 2),
    integer(split.end_layer, 2),
  ]),
);
function activation(values, position, layer) {
  const id = Buffer.from("lap-request");
  const floats = Buffer.alloc(values.length * 4);
  values.forEach((value, i) => floats.writeFloatBE(value, i * 4));
  return Buffer.concat([
    Buffer.from([1]),
    integer(id.length, 2),
    id,
    binding,
    integer(position, 4),
    integer(layer, 2),
    integer(values.length, 4),
    floats,
  ]);
}
const a = new ScriptedModel(source.model.layers.slice(0, 2));
const b = new ScriptedModel(source.model.layers.slice(2));
const ca = a.createCache("lap-request"),
  cb = b.createCache("lap-request");
const activations = [];
for (const [position, token] of [3, 5, 7, 11, 13, 17].entries()) {
  const input = await a.runActivation(scriptedEmbedding(token), position, ca);
  const output = await b.runActivation(input, position, cb);
  activations.push({
    input_hex: hex(activation(input, position, 2)),
    output_hex: hex(activation(output, position, 4)),
  });
}
function receiptBytes(receipt, signing = false) {
  return Buffer.concat([
    Buffer.from(receipt.deal_id),
    ...[
      "lap_index",
      "prompt_tokens",
      "generated_tokens",
      "cumulative_tokens",
    ].map((name) => integer(receipt[name], 8)),
    integer(receipt.layer_range_digests.length, 1),
    ...receipt.layer_range_digests.map((value) => Buffer.from(value)),
    signing ? Buffer.alloc(64) : Buffer.from(receipt.host_signature),
    integer(receipt.peer_countersignatures, 4),
  ]);
}
function signedReceipt(receipt, signer = host) {
  const copy = structuredClone(receipt);
  copy.host_signature = Array.from(
    sign(
      null,
      domain("PAXAI/lap-signing/v1", receiptBytes(copy, true)),
      signer.privateKey,
    ),
  );
  return copy;
}
function receipt(index, from, to, prompt, generated, cumulative) {
  return signedReceipt({
    deal_id: deal.deal_id,
    lap_index: index,
    prompt_tokens: prompt,
    generated_tokens: generated,
    cumulative_tokens: cumulative,
    layer_range_digests: ["input_hex", "output_hex"].map((name) =>
      Array.from(
        sha(
          ...activations
            .slice(from, to)
            .map((value) => Buffer.from(value[name], "hex")),
        ),
      ),
    ),
    host_signature: [],
    peer_countersignatures: 0,
  });
}
const first = receipt(0, 0, 4, 2, 2, 4),
  next = receipt(1, 4, 6, 0, 2, 6);
const frame = (receipt) => ({
  type: "lap_receipt",
  lap: {
    deal_digest: Array.from(binding),
    host_meter_key: Array.from(host.publicKey),
    deadline_ms: 4102444800000,
    receipt,
  },
});
const firstFrame = frame(first),
  nextFrame = frame(next);
function acknowledgement(frame) {
  const lap = frame.lap;
  const digest = domain(
    "PAXAI/lap-countersign/v1",
    lap.deal_digest,
    lap.host_meter_key,
    integer(lap.deadline_ms, 8),
    domain("PAXAI/lap-receipt/v1", receiptBytes(lap.receipt)),
  );
  return {
    type: "lap_countersign",
    deal_id: lap.receipt.deal_id,
    lap_index: lap.receipt.lap_index,
    digest: Array.from(digest),
    signature: Array.from(sign(null, digest, peer.privateKey)),
  };
}
const vectors = [];
function vector(name, direction, value) {
  vectors.push({ name, direction, json: JSON.stringify(value) });
}
vector("lap_receipt", "server", firstFrame);
vector("lap_countersign", "client", acknowledgement(firstFrame));
vector("lap_receipt_next", "server", nextFrame);
vector("lap_countersign_next", "client", acknowledgement(nextFrame));
for (const [name, patch] of [
  [
    "foreign_range",
    { layer_range_digests: [first.layer_range_digests[0], fixed(99)] },
  ],
  ["gap", { lap_index: 2 }],
  ["bad_tokens", { cumulative_tokens: 5 }],
  ["zero_generated", { prompt_tokens: 4, generated_tokens: 0 }],
  ["wrong_deal", { deal_id: fixed(99) }],
  ["nonempty_bitmap", { peer_countersignatures: 1 }],
])
  vector(
    "lap_receipt_" + name,
    "server",
    frame(signedReceipt({ ...first, ...patch })),
  );
const conflict = structuredClone(firstFrame);
conflict.lap.deadline_ms++;
vector("lap_receipt_conflict", "server", conflict);
const expired = structuredClone(firstFrame);
expired.lap.deadline_ms = 1;
vector("lap_receipt_expired", "server", expired);
const wrongBinding = structuredClone(firstFrame);
wrongBinding.lap.deal_digest = fixed(99);
vector("lap_receipt_wrong_binding", "server", wrongBinding);
const wrongKey = structuredClone(firstFrame);
wrongKey.lap.host_meter_key = Array.from(previous.publicKey);
wrongKey.lap.receipt = signedReceipt(first, previous);
vector("lap_receipt_wrong_key", "server", wrongKey);
const badSignature = structuredClone(firstFrame);
badSignature.lap.receipt.host_signature[0] ^= 1;
vector("lap_receipt_bad_signature", "server", badSignature);
const nextPrompt = frame(
  signedReceipt({ ...next, prompt_tokens: 1, generated_tokens: 1 }),
);
vector("lap_receipt_later_prompt", "server", nextPrompt);
const offer = {
  room_id: deal.room_id,
  model_digest: deal.model_digest,
  runtime_digest: deal.runtime_digest,
  layer_plan: deal.plan.layers.map(
    ({ peer_key, first_layer, end_layer }) => ({
      peer_key,
      first_layer,
      end_layer,
    }),
  ),
  price: deal.price,
};
const ackBytes = Buffer.concat([
  Buffer.from("PAXAI/inference-deal-ack/v1\0"),
  binding,
]);
const session = [
  {
    name: "offer",
    direction: "server",
    json: JSON.stringify({ type: "offer", offer }),
  },
  {
    name: "deal",
    direction: "server",
    json: JSON.stringify({ type: "deal", deal }),
  },
  {
    name: "deal_ack",
    direction: "client",
    json: JSON.stringify({
      type: "deal_ack",
      deal_id: deal.deal_id,
      digest: Array.from(binding),
      signature: Array.from(sign(null, ackBytes, peer.privateKey)),
    }),
  },
  {
    name: "deal_live",
    direction: "server",
    json: JSON.stringify({ type: "deal_live", deal_id: deal.deal_id }),
  },
];
// f11_lap_settlement.rs::batch(0, 1, 0), task 22.38.4.
const programReceipt = signedReceipt({
  deal_id: fixed(62),
  lap_index: 0,
  prompt_tokens: 10,
  generated_tokens: 22,
  cumulative_tokens: 32,
  layer_range_digests: [fixed(80), fixed(81)],
  host_signature: [],
  peer_countersignatures: 0,
});
writeFileSync(
  new URL("host-lap-frames.json", import.meta.url),
  JSON.stringify(
    {
      source:
        "services/agent/crates/layerx-agentd/src/inference_market/laps.rs::{LapServerFrame,LapAcknowledgement,LapFrame::countersign_digest}",
      codec_source:
        "programs/crates/layerx-programs-ai-market/src/laps.rs::LapReceipt",
      peer_key: base.key.peer_key_hex,
      host_meter_key: hex(host.publicKey),
      previous_peer_key: hex(previous.publicKey),
      range_digest:
        "SHA-256 of concatenated encoded output activation payloads in processing order; LapInput supplies the host digests",
      activations,
      session,
      vectors,
      program_codec: {
        source:
          "programs/crates/layerx-programs-ai-market/tests/f11_lap_settlement.rs::batch(0,1,0)",
        receipt: programReceipt,
        encoded_hex: hex(receiptBytes(programReceipt)),
        signing_digest_hex: hex(
          domain("PAXAI/lap-signing/v1", receiptBytes(programReceipt, true)),
        ),
        receipt_digest_hex: hex(
          domain("PAXAI/lap-receipt/v1", receiptBytes(programReceipt)),
        ),
      },
    },
    null,
    2,
  ) + "\n",
);
