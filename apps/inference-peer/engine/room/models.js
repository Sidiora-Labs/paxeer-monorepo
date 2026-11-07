// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/models.js (MIT; see LICENSE-ENGINE).
// Model catalogue for the room: URLs, layer counts, memory needs, context length.
import { qwen4expMemory, QSA_RAW_RING } from "../engine/qwen4exp_mem.js";
import { qwen4expConfig, qwen4expShardBytes } from "../engine/gguf.js";
import { OFFLOAD_CKPT_SLOTS } from "./plan.js";

// GB each model needs at its default context (the picker's "needs N GB"); NEED_MIN_GB: what a model
// with a fallback context (CTX[model].fallback, or its ladder's last step: ctxFloor) needs there, the least a room can
// start it with (the 27B's and the MoE's are set at the end of this file, from roomBytes)
export const NEED_GB = { "qwen3-0.6b": 0.8, "qwen3-1.7b": 5.6, "qwen3-4b": 4.6, "qwen3.8-27b": 17.0, "qwen3.6-35b-moe": 22.5, "qwen3.5-122b-moe": 72.0,
  // Qwen3.8-Flash-Next: its 48 layers with their state at the default context, the embedding and the head, from the
  // planner's own costs (roomBytes at CTX def: set at the end of this file, flashLayerCost), in GiB (2^30, the unit the
  // pledges are in: 67.1 is 72.05e9 bytes, what the old fixed 72.0 decimal GB was within 0.2%); the 28.8 GB n-gram table
  // is not GPU memory (it stays on the disk of the device holding layer 1: SHAPE ngram, room/plan.js dealRoom)
  "qwen3.8-flash": 0, "smollm-135m": 0.6 };
export const NEED_MIN_GB = { "qwen3-1.7b": 4.0 };

// What the model host holds besides its layers, in bytes, from the file's own tensor sizes: the
// embedding stays in JS memory for row lookups, the output head goes to the GPU (the embedding again
// when the two are tied), and the Qwen 3.5/3.8 engines add the draft (MTP) block and, for the
// one-submit draft chain, a GPU copy of the embedding table (engine/qwen35.js). Qwen3.8-Flash-Next (qwen4exp) keeps
// its embedding in JS memory too; its `out` counts the head's HC mixer with output.weight, and its draft block (a
// separate MTP file) is not loaded in rooms yet (mtp 0). This is what the
// host's pledge pays for before its layers (room/plan.js layerCaps).
export function hostHeldBytes(kind, { embed = 0, out = 0, mtp = 0 } = {}) {
  const held = embed + (out || embed);
  return kind === "qwen35" ? held + mtp + (mtp ? embed : 0) : kind === "qwen4exp" ? held + mtp : held;
}
// K+V bytes per layer and position for a dense (Qwen3) room: the dense engine keeps its KV cache in
// f32 (engine/dense.js), one K and one V row of kvDim per position
export const denseKvBytesPerLayerPos = (kvDim) => 2 * kvDim * 4;

// ---- what expert offload parks (engine/expert_store.js ExpertStore.park) ----
// The loader converts each expert tensor before it is parked (engine/gguf.js convertEntry): Q4_0 stays Q4 (nibbles +
// an f16 scale per 32: 18/32 B per weight), Q8_0 and every other quant (Q4_1, Q5_0, Q5_K, Q6_K) become Q8 (int8 + an
// f16 scale per 32: 34/32 B per weight). So a Q4_1 tensor parks 34/32 B per weight, not the file's 20/32.
const Q4 = 2, Q41 = 3;   // engine/gguf.js GGML_Q4_0, GGML_Q4_1
const FLOAT_TYPES = [0, 1, 30];   // F32, F16, BF16: ExpertStore can't park these
// One expert's six parked slices (gate qs, gate scales, up qs, up scales, down qs, down scales: ExpertStore.SLICES),
// in bytes, for experts of rows x cols weights (gate / up: [ffn, hidden]; down: [hidden, ffn]) of these GGML types
export function expertSlices(ffn, hidden, types) {
  const n = ffn * hidden;
  return types.flatMap((t) => [t === Q4 ? n / 2 : n, n / 16]);
}
// A layer's parked slices from a GGUF header (G.tensors[name]: { shape: [nExp, rows, cols], ggmlType }), or null when
// it has no routed experts. -> { nExp, slices: [6] }
export function expertSlicesOf(G, i) {
  const ts = ["gate", "up", "down"].map((p) => G.tensors[`blk.${i}.ffn_${p}_exps.weight`]);
  if (ts.some((t) => !t)) return null;
  const slices = [];
  for (const t of ts) {
    if (FLOAT_TYPES.includes(t.ggmlType) || t.shape?.length !== 3) return null;
    const n = t.shape[1] * t.shape[2];
    slices.push(t.ggmlType === Q4 ? n / 2 : n, n / 16);
  }
  return { nExp: ts[0].shape[0], slices };
}
// The experts' offload profile (room/plan.js offloadNeed / offloadPlan) for layers 0..L-1 of a header, or null for a
// dense model: { E: one layer's routed expert bytes in the file (layers 0-3's average: the part of a layer's VRAM
// estimate that parking frees), nExp, slices: per layer, one expert's six parked slices (what ExpertStore parks) }
// ... and what a device that offloads needs past its layers and its expert cache (room/plan.js offloadSpare): kvPos
// (f16 K+V bytes per layer and position, averaged: a checkpoint's KV rows), state (a layer's recurrent state, averaged:
// DeltaNet S and conv window, which a checkpoint copies and speculation's rollback keeps once more) and work (the
// engine's working buffers: prefill frames, batched passes; offloadWork)
// Qwen3.8-Flash-Next (qwen4exp) has its own sizes: K / V in f32 on its 12 QSA layers (averaged per layer), the DeltaNet
// state from its own keys, no whole-layer region (flashOffload) and checkpoints of at most its 2048 positions.
export function expertsOf(G, L) {
  const per = [];
  for (let i = 0; i < L; i++) { const x = expertSlicesOf(G, i); if (!x) return null; per.push(x); }
  if (!per.length || per.some((x) => x.nExp !== per[0].nExp)) return null;
  const M = G.meta || {};
  const base = { E: expertBytesOf(G, 0, Math.min(4, L)) / Math.min(4, L), nExp: per[0].nExp, slices: per.map((x) => x.slices) };
  if (M["general.architecture"] === "qwen4exp") return { ...base, ...flashOffload(flashMeta(M)) };
  return { ...base, kvPos: kvBytesPerLayerPos(M, "f16"), state: stateBytesPerLayer(M), work: offloadWork(+M["qwen35.embedding_length"] || 0) };
}
// the fields flashOffload reads, from a qwen4exp header (per-layer arrays: the QSA flags as engine/gguf.js
// qwen4expConfig sets them, every interval-th layer when the file has no recurrent_layers key)
function flashMeta(M) {
  const k = (n) => M["qwen4exp." + n];
  const L = k("block_count") - (k("nextn_predict_layers") || 0), rec = k("attention.recurrent_layers"), iv = k("full_attention_interval") ?? 4;
  const qsa = Array.from({ length: L }, (_, i) => (Array.isArray(rec) ? !rec[i] : (i + 1) % iv === 0)).filter(Boolean).length;
  return { L, qsa, K: k("expert_used_count"), dim: k("embedding_length"), nKV: k("attention.head_count_kv"), headDim: k("attention.key_length"),
    ssm: { state: k("ssm.state_size"), nVH: k("ssm.time_step_rank"), nKH: k("ssm.group_count"), inner: k("ssm.inner_size"), convK: k("ssm.conv_kernel") } };
}
// Batched prompt passes (room/flash.js): the prompt frame width a node host sends (FlashRoomEngine.prefillFrame;
// POOLED_PREFILL_FRAME overrides it), measured on the GB10 pair (PR body): wider frames run the wide kernels better,
// narrower ones keep both devices busy (room/pipeline.js PREFILL_WINDOW frames in flight)
export const FLASH_PREFILL_FRAME = 256;
// The GPU buffers a Qwen4ExpBatch allocates (its scratch B, the readback stages; no QSA), as the loads logged them on the
// GB10 (qwen4expBatchBytes after create, wide kernels): 159 MiB at maxCols 256 on the host's layers (with the PLE
// layer's embedding / pk / pv buffers, about 60 KB a column more than a range without it: a worker's logged 142 MiB)
// and 615 MiB at 1024 on a solo device. Fitted on the ranges with the PLE layer, the largest: 622,600 B per column and
// 7 MiB fixed; the wantHidden stage (maxCols x hDim f32, made on the first frame) on top. -> bytes
export const FLASH_BATCH_BYTES_PER_COL = 622600, FLASH_BATCH_BYTES_FIXED = 7 * 2 ** 20;
export function flashBatchScratch(cols) { return FLASH_BATCH_BYTES_FIXED + cols * (FLASH_BATCH_BYTES_PER_COL + 10240 * 4); }
// What a dealer keeps back from each device's pledge in a Qwen3.8-Flash-Next room (room.js aiStart, roomnode.js
// dealPlan, the fit checks; room/pledge.js dealPledgeBytes): the batched passes' scratch at the room's prompt frame
// width, so a device dealt its whole pledge still has it. An offloading device that ends up without batched passes
// gives it back to its expert cache (room-node shard.js loadFlash). 0 for other models. -> bytes
export const flashDealReserve = (model) => (MODELS[model]?.kind === "qwen4exp" ? flashBatchScratch(FLASH_PREFILL_FRAME) : 0);
// Expert offload's extra sizes for Qwen3.8-Flash-Next (room/plan.js offloadSpare, expertSizes), from flashMeta's fields:
//   kvPos   K + V in f32 (the phase-1 engine), 2 x nKV x headDim x 4 per QSA layer and position, averaged over the layers
//   state   a DeltaNet layer's S and conv window, averaged (the QSA layers have none): speculation's rollback copy
//   work    the engine's working buffers (offloadWork by the hidden size). The batched passes' scratch is not in it:
//           the dealer keeps it back from every pledge (flashDealReserve), and an offloader that does not batch gets it
//           back for its cache
//   region  false: the deal sets no whole-layer region aside; the device takes one for batched prompt passes only from
//           its pledge's slack or out of a cache that keeps OFFLOAD_GOOD_SLOTS after it (room-node shard.js loadFlash),
//           else runs prompt frames one column at a time
//   byLayer the checkpoints' part comes from the room's per-layer costs (flashLayerCost: its offVram / offRam, the
//           shared flashCkptReserve), not from kvPos x ckptTokens, whenever the deal has them
//   ckptTokens  without per-layer costs (a deal given one number per layer): 8 slots of at most 2048 positions
//   K       experts a token picks (10 of 512): room/plan.js tabDiskCost
export const FLASH_CKPT_TOKENS = 8 * 2048;
export function flashOffload(f) {
  const s = f.ssm, dn = (f.L - f.qsa) / f.L;
  return { kvPos: 2 * f.nKV * f.headDim * 4 * f.qsa / f.L,
    state: (s.nVH * s.state * s.state * 4 + (2 * s.nKH * s.state + s.inner) * (s.convK - 1) * 4) * dn,
    work: offloadWork(f.dim), region: false, byLayer: true, ckptTokens: FLASH_CKPT_TOKENS, K: f.K || 10 };
}
// A Qwen3.5 / 3.6 layer's recurrent state, averaged over the layers (all but every full_attention_interval-th are
// DeltaNet): S (nVH x dState x dState f32) and the conv window (3 x convDim f32), as engine/qwen35.js allocates them
export function stateBytesPerLayer(meta) {
  const dState = +meta["qwen35.ssm.state_size"] || 0, nVH = +meta["qwen35.ssm.time_step_rank"] || 0;
  const convDim = 2 * dState * (+meta["qwen35.ssm.group_count"] || 0) + (+meta["qwen35.ssm.inner_size"] || 0);
  const k = +meta["qwen35.full_attention_interval"] || 1;
  return (nVH * dState * dState * 4 + 3 * convDim * 4) * (k - 1) / k;
}
// The engine's working buffers on a device that offloads, beyond its layers and the cache: the prefill frames' and
// batched passes' activations, allocated on the first long prompt. Measured with every buffer counted (Dawn, a worker
// shard, a 12k-token prefill, decode and verifies): 0.16 GiB on the 35B (hidden 2048), 0.30 GiB on the 122B (3072);
// D3D12 (the RTX 5070 PC) adds about 70% to that (64 KiB placement, Dawn's own heaps). 192 KiB per hidden unit
// covers both with that margin: 0.38 / 0.56 GiB.
export const offloadWork = (dim) => dim * 192 * 1024;
// SHAPE's park (runs of layers that park alike) as expertsOf's profile
const parkProfile = (s) => s.park && { E: s.exp, nExp: s.park.nExp, slices: s.park.runs.flatMap(([n, sl]) => Array(n).fill(sl)),
  ...(s.flash ? flashOffload(s.flash) : { kvPos: s.kvPos.f16, state: stateBytesPerLayer(s.meta), work: offloadWork(s.meta["qwen35.embedding_length"]) }) };

// the header fields stateBytesPerLayer and offloadWork read (SHAPE's MoE models: their GGUFs' values)
function ssmMeta(dim, nVH, nKH, inner) {
  return { "qwen35.embedding_length": dim, "qwen35.ssm.state_size": 128, "qwen35.ssm.time_step_rank": nVH, "qwen35.ssm.group_count": nKH,
    "qwen35.ssm.inner_size": inner, "qwen35.full_attention_interval": 4 };
}
// Qwen3.8-Flash-Next's 48 layers' weights in bytes (SHAPE layers), eight a row
const FLASH_LAYER_BYTES = [
  1527552896, 1546189696, 1527552896, 1523539968, 1527552896, 1519688576, 1465416576, 1461157888,
  1475124096, 1465416576, 1465416576, 1463001088, 1473280896, 1465416576, 1467259776, 1469267968,
  1473280896, 1467259776, 1465416576, 1461157888, 1475124096, 1465416576, 1465416576, 1463001088,
  1473280896, 1465416576, 1467259776, 1469267968, 1473280896, 1467259776, 1465416576, 1461157888,
  1475124096, 1465416576, 1465416576, 1463001088, 1473280896, 1465416576, 1467259776, 1469267968,
  1473280896, 1475124096, 1475124096, 1471111168, 1475124096, 1475124096, 1475124096, 1471111168];
// Each picker model's shape, from its GGUF header (the host reads the same numbers from the file at
// Start, so these only let the picker say before Start whether the room's pledges hold the model):
// L layers; layer = one layer's weights (the largest group's average for the hybrids); kvPos = K+V
// bytes per layer and position by KV format; embed / out / mtp: the host-only tensors; exp: one layer's routed
// experts in the file (the MoE models; layers 0-3's average, as `layer`); park: what ExpertStore parks per expert, by
// runs of layers (expertSlices; the 35B's layers 0-4 and the 122B's 0-5 have Q4_1 down experts, parked as Q8).
export const SHAPE = {
  "qwen3-1.7b": { kind: "gguf", L: 28, layer: 53494784, kvPos: { f16: denseKvBytesPerLayerPos(1024) }, embed: 330612736, out: 0, mtp: 0 },
  "qwen3.8-27b": { kind: "qwen35", L: 64, layer: 223970464, kvPos: { f16: 1024, q8: 576 }, embed: 715161600, out: 1042944000, mtp: 265197568 },
  "qwen3.6-35b-moe": { kind: "qwen35", L: 40, layer: 498197568, kvPos: { f16: 512, q8: 288 }, embed: 286064640, out: 417177600, mtp: 897955840, exp: 469762048,
    park: { nExp: 256, runs: [[5, expertSlices(512, 2048, [Q4, Q4, Q41])], [35, expertSlices(512, 2048, [Q4, Q4, Q4])]] }, meta: ssmMeta(2048, 32, 16, 4096) },
  "qwen3.5-122b-moe": { kind: "qwen35", L: 48, layer: 1485116672, kvPos: { f16: 512, q8: 288 }, embed: 429096960, out: 625766400, mtp: 2682195968, exp: 1409286144,
    park: { nExp: 256, runs: [[6, expertSlices(1024, 3072, [Q4, Q4, Q41])], [42, expertSlices(1024, 3072, [Q4, Q4, Q4])]] }, meta: ssmMeta(3072, 64, 16, 8192) },
  // Qwen3.8-Flash-Next (llama.cpp arch qwen4exp; roadmap/33), bartowski Q4_0 (three files), read from its headers:
  // layer: layers 0-3's average without QSA's indexer (qwen4expShardBytes over [0, 4) / 4), which goes with exp (also
  // layers 0-3's) wherever one size per layer is still read (#386: the file's whole average less exp left ~10 MB a layer
  // on the GPU for an offloaded layer, so the picker said Flash fit where Start refused it). The room deals Flash by
  // each layer's own cost (`layers` below, flashLayerCost: layer 0 is 1.528 GB, layer 1 with the PLE block 1.546 GB,
  // layers 0-5 carry Q4_1 down experts, plus each layer's own state at the room's context), not by this one size, and
  // the picker and Start compute those costs the same way (from SHAPE or from the header). kvPos: the 12 QSA
  // layers' K+V, 2 KV heads x 256, kept in f32 by the engine (engine/qwen4exp.js), so 4 KB per QSA layer and
  // position, 1 KB averaged over 4 (the room's KV format stays "f16": kvModeFor); out: output.weight and the head's
  // HC mixer (output_hc_*); exp / park: 512 experts of 2560 x 640, layers 0-5 with Q4_1 down experts (parked as Q8).
  // hDim: the 4-stream hyper-connection residual (4 x 2560) is what crosses devices (room/pipeline.js wireDim).
  // ngram: the n-gram (PLE) table, read row by row by the device holding its layer (engine/ngram.js); never on a GPU
  // and never part of a layer's bytes. pleState: that layer's extra state a checkpoint copies: the PLE conv history
  // (10 positions x hDim f32, engine/qwen4exp.js st.ring).
  "qwen3.8-flash": { kind: "qwen4exp", L: 48, layer: 1531208864, kvPos: { f16: 1024 }, embed: 357580800, out: 534620160, mtp: 0, exp: 1468006400,
    park: { nExp: 512, runs: [[6, expertSlices(640, 2560, [Q4, Q4, Q41])], [42, expertSlices(640, 2560, [Q4, Q4, Q4])]] }, meta: ssmMeta(2560, 48, 16, 6144),
    hDim: 10240, ngram: { layer: 1, bytes: 28800138240 }, pleState: 10 * 10240 * 4,
    // flashOffload's fields (expert offload: room/plan.js), from the header (flashMeta); iv (every iv-th layer is QSA)
    // and ple (the n-gram layer's ring) for flashCfgOf, which sizes the state without a header (flashRoomMem)
    flash: { L: 48, qsa: 12, K: 10, dim: 2560, nKV: 2, headDim: 256, iv: 4, ssm: { state: 128, nVH: 48, nKH: 16, inner: 6144, convK: 4 },
      ple: { layers: [1], ngram: 3, convK: 4 } },
    // layers: each layer's weights as the room loads them (qwen4expShardBytes of [i, i+1), no QSA indexer), from the
    // header (tests/unit/flash_mem_test.js checks them against tests/fixtures/q38flash): layers 0-5 carry Q4_1 down
    // experts (+50-80 MB), layer 1 the PLE block. indexer: what loading QSA adds to each QSA layer (its indexer);
    // layerExp: each layer's routed experts in the file, in runs (expertBytesOf: 6 x Q4_1-down layers, then 42): what
    // offloading that layer takes off the GPU (room/plan.js offloadAt), where `exp` is layers 0-3's average
    layers: FLASH_LAYER_BYTES, indexer: 3277824, layerExp: [[6, 1468006400], [42, 1415577600]] },
};
// The width of the hidden state that crosses devices, in floats: the model's hidden size, or for Qwen3.8-Flash-Next
// its 4-stream residual (SHAPE hDim). 0 when SHAPE does not say (the room reads it from the engine: wire.js wireDim).
export const wireWidth = (model) => SHAPE[model]?.hDim || +SHAPE[model]?.meta?.["qwen35.embedding_length"] || 0;
// Where a model's n-gram table must be: { layer, bytes } (the device holding that layer reads it from its own disk,
// room/plan.js dealRoom ngram), or null for every model without one
export const ngramOf = (model) => SHAPE[model]?.ngram || null;
// A split GGUF's headers (llama.cpp gguf-split: MODELS[key].shards, in order) as one: the first file's metadata and
// every file's tensors, each tensor with its file (shard: index; url: that file's URL when urls is given, for the
// loaders that fetch by URL). Offsets stay within their own file. -> the first header, merged
export function mergeSplitHeaders(headers, urls = null) {
  const G = headers[0], n = +G.meta?.["split.count"] || headers.length;
  if (n !== headers.length) throw new Error(`a split GGUF of ${n} files, ${headers.length} read`);
  for (let i = 1; i < headers.length; i++)
    for (const [name, t] of Object.entries(headers[i].tensors)) G.tensors[name] = { ...t, shard: i, ...(urls ? { url: urls[i] } : {}) };
  return G;
}
// A MoE GGUF's routed expert bytes over layers [lo, hi) (G: a parsed header): what a device that offloads parks per
// layer (room/plan.js offloadNeed); 0 for a dense model
export function expertBytesOf(G, lo, hi) {
  let n = 0;
  for (let i = lo; i < hi; i++) for (const p of ["gate", "up", "down"]) n += G.tensors[`blk.${i}.ffn_${p}_exps.weight`]?.byteLength || 0;
  return n;
}
// { L, layerBytes, hostBytes, expertBytes } for a room running `model` at `ctx` positions with `kv` format, or
// null for a model without a SHAPE (the picker then falls back to NEED_GB). expertBytes: 0 for a dense model;
// experts: the MoE models' offload profile (expertsOf), else null.
// Qwen3.8-Flash-Next also has layerCost: each layer's own bytes at ctx (flashLayerCost, in the engine's K / V format: kv
// "f32", or f16 for "f16" / anything else, the rooms' format: flashRoomLimits), which every deal and fit check takes in place
// of layerBytes (room/plan.js roomFit, dealRoom); layerBytes is then their average, so L x layerBytes is still the
// layers' total. Memoized per (model, ctx, kv): the picker asks on every room update (minFitPledge probes ~24 times).
const rbMemo = new Map();
export function roomBytes(model, ctx, kv = "f16") {
  const s = SHAPE[model]; if (!s) return null;
  const k = `${model}|${ctx}|${kv}`;
  if (rbMemo.has(k)) return rbMemo.get(k);
  let out;
  if (s.flash) {
    // (Flash: kv is the engine's K / V format, "f32" or f16 for anything else: FLASH_ROOM_KV, flashRoomLimits)
    const layerCost = flashLayerCost(s, ctx, { kv: kv === "f32" ? "f32" : "f16" });
    out = { L: s.L, layerBytes: layerCost.total / s.L, layerCost, hostBytes: hostHeldBytes(s.kind, s), expertBytes: s.exp || 0, experts: parkProfile(s) || null };
  } else out = { L: s.L, layerBytes: s.layer + ctx * (s.kvPos[kv] ?? s.kvPos.f16), hostBytes: hostHeldBytes(s.kind, s), expertBytes: s.exp || 0,
    experts: parkProfile(s) || null };
  // one object for every caller (the picker's probes, the deal, the cards): frozen, so no caller changes another's
  // (layerCost is a typed array, which can't be frozen: flashLayerCost's memo, read-only by contract too)
  if (out.experts) Object.freeze(out.experts);
  Object.freeze(out);
  if (rbMemo.size > 256) rbMemo.clear();
  rbMemo.set(k, out);
  return out;
}

// ---------------- Qwen3.8-Flash-Next's memory in a room: one model for every caller ----------------
// The planner (room/plan.js through roomBytes / flashLayerCost), the room page's deal (room.js aiStart), the room
// node's deal and context pick (packages/room-node/roomnode.js), its load-time check (shard.js flashShardBytes), the
// picker (room.js roomFitFor, minFitPledge, NEED_GB) and the expert-offload spare (room/plan.js offloadSpare) all size
// Flash's per-sequence state with engine/qwen4exp_mem.js qwen4expMemory, through flashRoomMem.
// The dense engine's longest context: QSA's selection equals dense attention below 2052 cells (engine/wgsl/qsa.js), and
// the rooms load without the indexer up to here (CTX "qwen3.8-flash")
export const FLASH_DENSE_CTX = 2048;
// Rooms load Flash with QSA (ai-load.flash) at every context, 2048 included, when every dealt device can: the planner
// counts QSA's indexer weights and state by default (a room with an older device loads dense: ~44 MB over, on the safe side)
export const FLASH_ROOM_QSA = true;
// Checkpoints at long context: one slot of a 32K room's state is ~0.8 GB per device, so a fixed slot count stops being
// a small thing. Every Flash device of a room past FLASH_DENSE_CTX keeps CKPT_RESERVE_SLOTS slots (one answer, one pinned
// prefix) of its range's state at the room's context in its deal (flashLayerCost); more come from slack at run time.
// At FLASH_DENSE_CTX and below the deal reserves nothing for them (as before), except on a device that offloads experts:
// OFFLOAD_CKPT_SLOTS slots of 2048 positions (room/plan.js offloadSpare, FLASH_CKPT_TOKENS), once, from these costs.
export const CKPT_RESERVE_SLOTS = 2;
// the checkpoint tokens a device keeps room for past FLASH_DENSE_CTX: the reserve's two slots of the room's context
export const CKPT_TOKEN_CAP = (ctx) => 2 * ctx;
// the slots a deal reserves in each layer's cost at ctx
export const ckptSlotsAt = (ctx) => (ctx > FLASH_DENSE_CTX ? CKPT_RESERVE_SLOTS : 0);

// qwen4expConfig's fields that qwen4expMemory reads, from SHAPE's flash entry (no header: the picker)
export function flashCfgOf(f, hDim = 4 * f.dim) {
  return { L: f.L, hDim, nKVHeads: f.nKV, headDim: f.headDim, qsa: Array.from({ length: f.L }, (_, i) => (i + 1) % (f.iv || 4) === 0),
    ssm: { convK: f.ssm.convK, stateSize: f.ssm.state, nKHeads: f.ssm.nKH, nVHeads: f.ssm.nVH, inner: f.ssm.inner },
    ple: f.ple ? { layers: f.ple.layers.slice(), ngram: f.ple.ngram, convK: f.ple.convK } : null };
}
// A Flash source as { cfg, weights (per layer, no indexer), indexer (per layer: what QSA's indexer adds) }, from a model
// key or its SHAPE entry (the picker), a GGUF header (G: { meta, tensors }; the room page and the room node read it
// at Start, the loaders at load), or a bare qwen4expConfig (no weights: flashRoomMem only)
const srcMemo = new WeakMap();
function flashSrc(src) {
  if (typeof src === "string") src = SHAPE[src];
  if (!src || typeof src !== "object") throw new Error("flashSrc: not a Qwen3.8-Flash-Next source");
  if (srcMemo.has(src)) return srcMemo.get(src);
  let out;
  if (src.flash) {
    const cfg = flashCfgOf(src.flash, src.hDim);
    out = { cfg, weights: src.layers || null, indexer: cfg.qsa.map((q) => (q ? src.indexer || 0 : 0)),
      experts: src.layerExp ? src.layerExp.flatMap(([n, b]) => Array(n).fill(b)) : null };
  } else if (src.meta && src.tensors) {
    const cfg = qwen4expConfig(src.meta), w = [], ix = [], ex = [];
    for (let i = 0; i < cfg.L; i++) {
      const o = { lo: i, hi: i + 1, hasEmbed: false, hasHead: false };
      w.push(qwen4expShardBytes(src, { ...o, qsa: false }));
      ix.push(qwen4expShardBytes(src, { ...o, qsa: true }) - w[i]);
      ex.push(expertBytesOf(src, i, i + 1));
    }
    out = { cfg, weights: w, indexer: ix, experts: ex };
  } else if (Array.isArray(src.qsa) && src.ssm) out = { cfg: src, weights: null, indexer: null };
  else throw new Error("flashSrc: not a Qwen3.8-Flash-Next source");
  srcMemo.set(src, out);
  return out;
}
// The state a Flash device keeps for one sequence at ctx: qwen4expMemory on that source, plus a checkpoint reserve.
// opts: kv ("f32": the engine's default), qsa (the indexer's weights and state; default FLASH_ROOM_QSA, as the rooms load),
// ring (rawK ring rows), range ([lo, hi): the device's layers; the whole model by default), ckptSlots (checkpoint slots
// of this state to reserve; 0 by default).
// -> qwen4expMemory's { ctx, kv, bytes, state, total, perToken, ... } plus reserve (ckptSlots x state) and need
// (total + reserve: what the device sets aside for it)
export function flashRoomMem(src, ctx, { kv = "f32", qsa = FLASH_ROOM_QSA, ring = QSA_RAW_RING, range = null, ckptSlots = 0 } = {}) {
  const m = qwen4expMemory(flashSrc(src).cfg, { ctx, kv, qsa, rawRing: ring, layerRange: range });
  const reserve = ckptSlots * m.state;
  return { ...m, reserve, need: m.total + reserve };
}
// The checkpoint reserve of a device holding `range` at ctx: `slots` copies of that range's state (flashRoomMem)
export function flashCkptReserve(src, range, ctx, slots = CKPT_RESERVE_SLOTS, opts = {}) {
  return slots * flashRoomMem(src, ctx, { ...opts, range }).state;
}
// Each layer's bytes in a Flash room at ctx: its weights (with the QSA indexer when qsa), its state at ctx (QSA layers
// their K / V, rawK and pooled keys; DeltaNet layers their fixed S and conv window; the n-gram
// layer its ring), and its share of the checkpoint reserve (ckptSlotsAt(ctx) copies of its state). So a range's bytes
// are the sum of its layers' plus the score scratch once (room/plan.js costOf cuts ranges on these), and a range of
// three DeltaNet layers costs what it does, not three averages. opts: as flashRoomMem (kv, qsa, ring), slots (the reserve: ckptSlotsAt(ctx) by default).
// -> Float64Array(L) with: total (the sum and once), once / onceAt (the QSA score scratch, one per device: its bytes, and
// the layers whose range keeps it), ctx, slots, weights / state / experts (per layer; experts: its routed
// experts in the file, what offloading it takes off the GPU: room/plan.js offloadAt), and what a device that offloads
// keeps past these per layer (room/plan.js offloadSpare, once): offVram (its checkpoints' VRAM: OFFLOAD_CKPT_SLOTS
// slots of 2048 positions at FLASH_DENSE_CTX and below; 0 past it, where the reserve is in the cost already) and offRam
// (one checkpoint's copy in RAM on its way to disk: FLASH_CKPT_TOKENS / 2 positions at FLASH_DENSE_CTX and below, the
// room's context past it: CKPT_TOKEN_CAP / 2)
// What a room page's model host keeps free past its layers while it loads Flash: Dawn's upload staging of its biggest
// layer and the engine's scratch, the terms a room node's load check counts on top of its deal (packages/room-node/
// shard.js flashShardBytes). A tab has no load-time check, so a tab host's deal sets them aside with the embedding and
// the head (room.js aiStart and roomFitFor; a room node or pooled host does not: its load check refuses instead)
// offloaded: the host offloads every layer it holds, so their experts are not staged (flashShardBytes offLayers): the
// biggest layer's other weights (room/plan.js dealWithHostMargin picks)
export const FLASH_SCRATCH = 64 * 2 ** 20;
export function flashTabHostMargin(src, offloaded = false) {
  const S = flashSrc(src), w = S.weights || [];
  return Math.max(0, ...w.map((b, i) => b - (offloaded ? S.experts?.[i] || 0 : 0))) + FLASH_SCRATCH;
}
const costMemo = new WeakMap();
export function flashLayerCost(src, ctx, { kv = "f32", qsa = FLASH_ROOM_QSA, ring = QSA_RAW_RING, slots = ckptSlotsAt(ctx) } = {}) {
  const S = flashSrc(src), key = `${ctx}|${kv}|${qsa}|${ring}|${slots}`;
  let per = costMemo.get(S);
  if (!per) costMemo.set(S, per = new Map());
  if (per.has(key)) return per.get(key);
  const { cfg } = S, L = cfg.L;
  if (!S.weights) throw new Error("flashLayerCost: the source has no per-layer weights");
  const out = new Float64Array(L), weights = new Float64Array(L), state = new Float64Array(L), offVram = new Float64Array(L), offRam = new Float64Array(L);
  const onceAt = new Uint8Array(L);
  let once = 0;
  const dense = ctx <= FLASH_DENSE_CTX;
  for (let i = 0; i < L; i++) {
    const r = [i, i + 1];
    const m = qwen4expMemory(cfg, { ctx, kv, qsa, rawRing: ring, layerRange: r });
    weights[i] = S.weights[i] + (qsa ? S.indexer?.[i] || 0 : 0);
    state[i] = m.state;
    // the QSA score scratch is one buffer per device, not per layer: `once`, counted once in a range that holds a QSA
    // layer (room/plan.js costOf), as the load check counts it (flashRoomMem over the range)
    out[i] = weights[i] + m.state + slots * m.state;
    if (m.bytes.scores > 0) { onceAt[i] = 1; once = Math.max(once, m.bytes.scores); }
    offVram[i] = dense ? OFFLOAD_CKPT_SLOTS * qwen4expMemory(cfg, { ctx: FLASH_DENSE_CTX, kv, qsa, rawRing: ring, layerRange: r }).state : 0;
    offRam[i] = qwen4expMemory(cfg, { ctx: dense ? FLASH_CKPT_TOKENS / 2 : CKPT_TOKEN_CAP(ctx) / 2, kv, qsa, rawRing: ring, layerRange: r }).state;
  }
  Object.assign(out, { total: out.reduce((a, b) => a + b, 0) + once, once, onceAt, ctx, slots, weights, state, offVram, offRam, ...(S.experts ? { experts: Float64Array.from(S.experts) } : {}) });
  per.set(key, out);
  return out;
}

// The whole weights file per picker model, in GB (the GGUF's size on Hugging Face). A room splits it:
// each device downloads about its share of the layers, so the picker can say what this device will fetch.
// Qwen3.8-Flash-Next: 71.8 of its 100.6 GB (FILES), without the 28.8 GB n-gram table, which only the device holding
// layer 1 fetches (SHAPE ngram, ngramOf), on top of its share: counting it here would put ~29 GB on every device.
export const FILE_GB = { "qwen3-1.7b": 1.83, "qwen3.8-27b": 16.06, "qwen3.6-35b-moe": 20.84, "qwen3.5-122b-moe": 72.52, "qwen3.8-flash": 71.8 };

// Each GGUF's exact size and SHA-256 (Hugging Face's x-linked-size / x-linked-etag), so `pooled pull`
// can check what it downloaded (cli/lib/cache.js)
export const FILES = {
  "qwen3-0.6b": { bytes: 639446688, sha256: "9465e63a22add5354d9bb4b99e90117043c7124007664907259bd16d043bb031" },
  "qwen3-1.7b": { bytes: 1834426016, sha256: "061b54daade076b5d3362dac252678d17da8c68f07560be70818cace6590cb1a" },
  "qwen3-4b": { bytes: 4280404704, sha256: "8c2f07f26af9747e41988551106f149b03eb9b5cb6df636027b6bf6278473300" },
  "qwen3.8-27b": { bytes: 16056478688, sha256: "ede16c7b36e578ca87a8c70e011e4b4633a32c831c0ce76d0f474582384e671d" },
  "qwen3.6-35b-moe": { bytes: 20836243072, sha256: "52312daa5b2190c1f5723d33c3315c01c55af4206f6c6e6eb63f3d8dd52bb85e" },
  // a split GGUF (llama.cpp gguf-split, two files): bytes is the whole model, shards each file's size and SHA-256
  "qwen3.5-122b-moe": { bytes: 72517482816, shards: [
    { bytes: 39917224704, sha256: "717ab3efe330cb8581e7233bb93ad0c965fdb351ce5ae61a6f7b1252ba6925d9" },
    { bytes: 32600258112, sha256: "28d9accacf99f2e87c6b2f4a199556067775af6ecccd76e31904f5bb0fd5867f" }] },
  // three files at the pinned revision (MODELS: resolve/928589f...), from the Hugging Face API (paths-info, lfs oid);
  // the first holds the 28.8 GB n-gram table. mtp: ggml-org's MTP file at its pinned revision (not loaded in rooms yet)
  "qwen3.8-flash": { bytes: 100599252672, shards: [
    { bytes: 39853467552, sha256: "d2dbb1e9e21fc2b214c78dfba33e2db7c6d36efe81e10bd329c3ca90314d9685" },
    { bytes: 39647284192, sha256: "d2b40751578e1e7a3725fa506410ba01dab544ca039d7da59ee9c8cfd10d4b5a" },
    { bytes: 21098500928, sha256: "1404069b7cd14cf79d1631b8cf5c5fdff1bb4edd45d82c443e93503569c6706f" }],
    mtp: { bytes: 2199652640, sha256: "d484e6e541a36b714976f72f7d1a86ae2cc450cff110b81e3656512784abd1c6" } },
};

// The models the room's picker offers, with the DISK_COPY models below. The others stay for tests and ?dev=1.
export const PICKER = ["qwen3-1.7b", "qwen3.8-27b", "qwen3.6-35b-moe"];

// Models that keep a copy on each device's disk and stream their experts from it, so they run on far less memory
// than their size. The picker's row and its two settings under Start (GPU memory, Disk) come from this entry and the
// room's own figures (room/diskpick.js), so another big model gets the same picker by adding one.
//   name: the row's name; params / arch: its chip ("125B · MoE · from disk"); active: parameters used per token (the
//   longer description: "125B (6B active)"; the model card's numbers, roadmap/33)
//   diskBytes: the most one tab stores (diskCopyBytes: every layer's experts and the n-gram table, room/flashdisk.js),
//   the same bytes room.js hostDiskBytes gives the host's disk question for a deal that puts all of them on its disk
//   ("92.4 GB")
export const diskCopyBytes = (key) => { const s = SHAPE[key]; return s ? s.exp * s.L + (s.ngram?.bytes || 0) : 0; };
export const DISK_COPY = {
  "qwen3.8-flash": { name: "Qwen3.8 Flash", params: "125B", active: "6B", arch: "MoE", diskBytes: diskCopyBytes("qwen3.8-flash") },
};

const FLASH_BASE = "https://huggingface.co/bartowski/Qwen3.8-Flash-Next-GGUF/resolve/928589fdb66c6ff07f22ac561e3fbce76553548f/Qwen3.8-Flash-Next-Q4_0/";
export const MODELS = {
  "qwen3-0.6b": { label: "Qwen3 0.6B · Q8", kind: "gguf",
    gguf: "https://huggingface.co/Qwen/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q8_0.gguf",
    cfg: "https://huggingface.co/Qwen/Qwen3-0.6B/resolve/main/config.json",
    tok: "https://huggingface.co/Qwen/Qwen3-0.6B/resolve/main/tokenizer.json" },
  "qwen3-1.7b": { label: "Qwen3 1.7B · Q8", kind: "gguf",
    gguf: "https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q8_0.gguf",
    cfg: "https://huggingface.co/Qwen/Qwen3-1.7B/resolve/main/config.json",
    tok: "https://huggingface.co/Qwen/Qwen3-1.7B/resolve/main/tokenizer.json" },
  "qwen3-4b": { label: "Qwen3 4B · Q8", kind: "gguf",
    gguf: "https://huggingface.co/Qwen/Qwen3-4B-GGUF/resolve/main/Qwen3-4B-Q8_0.gguf",
    cfg: "https://huggingface.co/Qwen/Qwen3-4B/resolve/main/config.json",
    tok: "https://huggingface.co/Qwen/Qwen3-4B/resolve/main/tokenizer.json" },
  "qwen3.8-27b": { label: "Qwen3.8 27B \u00b7 Q4", kind: "qwen35",
    gguf: "https://huggingface.co/unsloth/Qwen3.8-27B-GGUF/resolve/main/Qwen3.8-27B-Q4_0.gguf" },
  // mixture of experts: 256 experts, 8 active per token (~3B of 35B), so decode reads far less than the 27B
  "qwen3.6-35b-moe": { label: "Qwen3.6 35B MoE \u00b7 Q4", kind: "qwen35",
    gguf: "https://huggingface.co/bartowski/Qwen_Qwen3.6-35B-A3B-GGUF/resolve/main/Qwen_Qwen3.6-35B-A3B-Q4_0.gguf" },
  // 256 experts, 8 active (~10B of 122B), 48 layers, 16 query heads per kv head. Two files (shards: the loaders read
  // both, room/models.js mergeSplitHeaders). In the ?dev=1 list, not the picker: 72 GB of weights, a room of big
  // machines (or a PC that offloads its experts to RAM: room/plan.js), tests/test_moe.js MODEL=122b, `pooled pull`
  "qwen3.5-122b-moe": { label: "Qwen3.5 122B MoE \u00b7 Q4", kind: "qwen35",
    gguf: "https://huggingface.co/bartowski/Qwen_Qwen3.5-122B-A10B-GGUF/resolve/main/Qwen_Qwen3.5-122B-A10B-Q4_0/Qwen_Qwen3.5-122B-A10B-Q4_0-00001-of-00002.gguf",
    shards: ["00001", "00002"].map((n) => `https://huggingface.co/bartowski/Qwen_Qwen3.5-122B-A10B-GGUF/resolve/main/Qwen_Qwen3.5-122B-A10B-Q4_0/Qwen_Qwen3.5-122B-A10B-Q4_0-${n}-of-00002.gguf`) },
  // Qwen3.8-Flash-Next (roadmap/33): 512 experts, 10 active (~6B of 125B), a 4-stream residual and a 28.8 GB n-gram
  // table read from disk by the device holding layer 1. ?dev=1 only, like the 122B: its own engine (engine/qwen4exp.js,
  // PR #346; room/flash.js adapts it to the room), context <= 2048 until QSA (CTX). Pinned revisions: FILES' sizes and
  // hashes are these files'. mtp: the draft block's file (not loaded in rooms yet).
  "qwen3.8-flash": { label: "Qwen3.8 Flash Next \u00b7 Q4", kind: "qwen4exp",
    gguf: FLASH_BASE + "Qwen3.8-Flash-Next-Q4_0-00001-of-00003.gguf",
    shards: ["00001", "00002", "00003"].map((n) => `${FLASH_BASE}Qwen3.8-Flash-Next-Q4_0-${n}-of-00003.gguf`),
    mtp: "https://huggingface.co/ggml-org/Qwen3.8-Flash-Next-GGUF/resolve/052beeaca7bec4a303e59cc7bc630c4f3a1b845d/mtp-Qwen3.8-Flash-Next-Q4_0.gguf" },
  "smollm-135m": { label: "SmolLM 135M · bf16", kind: "safetensors",
    st: "https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct/resolve/main/model.safetensors",
    cfg: "https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct/resolve/main/config.json",
    tok: "https://huggingface.co/HuggingFaceTB/SmolLM2-135M-Instruct/resolve/main/tokenizer.json" },
};

// Context window per room, in tokens: prompt + answer. Each full-attention layer keeps K and V
// for this many positions (4 KB per position each for the 27B, so 16 MiB per attention layer at
// 2048); the kernels only use it as a stride. Generation stops before the cache would overflow.
export const MAX_SEQ = 2048;
// The 27B family keeps its KV cache in f16 with split-K flash attention (engine attnFlash), so its
// rooms get 8192 positions for the price of 4096 in f32: 32 MiB per attention layer, ~0.5 GB for
// the whole model, spread over the devices that hold the layers. The host reads the engine's
// maxSeq, so prompts, answer budgets and "context full" all follow it.
export const MAX_SEQ_LONG = 8192;
// Qwen3.8-Flash-Next's contexts in rooms. FLASH_CTX_PROVEN: the lengths a room has been checked at against llama.cpp
// (packages/room-node/test/flash_gold.mjs pair, f16 K / V, batched, 2026-10-13; docs/long-context-and-sessions.md has the
// runs). A length that is not proven is not offered and not opened. FLASH_CTX_OPTIONS: the picker's choices (a short
// list), the proven ones only. 131072 is not proven yet: memory and the 128 MiB binding hold it in f16, and a
// passphrase ~111K tokens back is found, but ctx128k and needle120k each differ from llama.cpp at one step by ~1.0 nat,
// past the golden rule's 0.5, in f16 and in f32 alike; add it here once a run passes.
export const FLASH_CTX_PROVEN = [2048, 8192, 16384, 32768, 65536];
export const FLASH_CTX_OPTIONS = [8192, 32768, 65536, 131072].filter((c) => FLASH_CTX_PROVEN.includes(c));
function FLASH_CTX_ENTRY() {
  const top = Math.max(...FLASH_CTX_PROVEN);
  return { def: top, max: top, ladder: FLASH_CTX_PROVEN.slice().sort((a, b) => b - a), options: FLASH_CTX_OPTIONS };
}
// Context per model, in tokens: the default a room opens with and the most ?ctx=N may ask for, and
// for some a fallback: the context a room opens with instead when its pledges can't hold the model
// at the default but can at the fallback (pickCtx), so a room short of memory still starts.
// These hybrids keep a KV cache only on their full-attention layers (1 in 4), and the DeltaNet
// layers hold a fixed-size state, so context is cheap in memory: f16 K+V is 64 KB per position
// for the 27B (16 attention layers, 4 KV heads x 256) and 20 KB for the 35B MoE (10 layers,
// 2 KV heads x 256). 32k on the MoE = 0.64 GB, 128k = 2.5 GB, spread over the devices holding
// those layers. One layer's K (or V) is one GPU buffer bound whole: 128 MiB for the MoE at 131072
// positions in f16, exactly the binding size every WebGPU device supports (so no device limits the
// MoE); the 27B needs 256 MiB at 131072 in f16 (128 MiB in int8), which the room only asks for
// when every device can bind it (ctxForBinding); its cap stays at 64K (128 MiB) until 128K is
// checked on it. The practical limit is prefill speed, not memory
// (docs/long-context-and-sessions.md: needle and speed at 32K..128K).
//
// A ladder (rooms long-context PR 5 for Flash; the 27B and the 35B MoE since this ladder's PR): the lengths a room may open
// the model at, longest first. A room opens at the longest step its devices hold (roomCtxPlan: pickCtx), down to the
// shortest; above `def` (the context the model's need, NEED_GB, is quoted at, and what rooms opened at before the ladder)
// only when the longer context costs no speed: the room holds it with every expert in GPU memory and on no more devices
// than at `def` (room/ctxpick.js roomCtxPlan fastAt; decode itself reads only the filled positions, so a longer window
// alone is no slower). `options`: the picker's context row (room/ctxpick.js ctxOptions). Only lengths checked correct
// on that model are on its ladder (docs/long-context-and-sessions.md):
//   27B  64K: needle found (tests/needle_ctx.js, 2026-09-30); 16K and 32K: needle found (attn decode v2, bench-log
//        2026-10-01); 8K: what its rooms ran before 16K (MAX_SEQ_LONG), and the same engine as at 16K / 32K (below).
//        128K is not checked (256 MiB per K/V buffer, the 128K prefill was stopped): not offered.
//   MoE  32K, 64K, 96K, 128K: needle found (tests/needle_ctx.js, 2026-09-30), spec = plain, no NaN; tests/test_moe.js
//        MATCHes llama.cpp with CTX=131072 (this PR); 8K and 16K: the engine at <= 32K positions is the 32K engine (one
//        256-position split length, engine/qwen35.js faSplit; decode splits follow the filled length), so it gives the
//        32K run's tokens on any prompt that fits.
export const CTX_LADDER_27B = [65536, 32768, 16384, 8192];
export const CTX_LADDER_MOE = [131072, 65536, 32768, 16384, 8192];
export const CTX = {
  "qwen3.8-27b": { def: 16384, max: 65536, ladder: CTX_LADDER_27B, options: [8192, 16384, 32768, 65536] },
  "qwen3.6-35b-moe": { def: 32768, max: 131072, ladder: CTX_LADDER_MOE, options: [8192, 32768, 65536, 131072] },
  // the 122B MoE: 12 attention layers, 2 KV heads x 256 (the 35B's K/V row): 24 KB per position in f16, 128 MiB per
  // layer's K at 131072. Checked at short contexts so far (phase 1): its default stays at the 35B's
  "qwen3.5-122b-moe": { def: 32768, max: 131072 },
  // Qwen3.8-Flash-Next with QSA (rooms long-context PR 5): the longest context proven correct in a room (FLASH_CTX_PROVEN,
  // goldens on the GB10) is both the default and the cap; the ladder is the proven lengths, longest first: a room opens at
  // the longest its devices hold (pickCtx), and the picker offers them (FLASH_CTX_OPTIONS). 12 QSA layers, 2 KV heads x
  // 256: in f16 (FLASH_ROOM_KV) one layer's K at 131072 is 128 MiB, WebGPU's default binding
  "qwen3.8-flash": FLASH_CTX_ENTRY(),
  // the dense engine keeps an f32 KV cache (~224 KB per position on the 1.7B: 1.8 GB at 8k, 3.6 GB
  // at 16k: 5.6 GB for the room in all). 16k fits an agent's prompt (OpenClaw's alone is ~12k), so
  // the room page, `pooled host` and the OpenClaw plugin all open it at 16k; a room short of that
  // (one 8 GB laptop lends 4 GB) opens it at 8k (4 GB), so one laptop still runs it alone.
  "qwen3-1.7b": { def: 16384, max: 16384, fallback: 8192 },
};
// WebGPU's default maxStorageBufferBindingSize: what a device binds when it reports nothing
export const WEBGPU_MIN_BIND = 128 * 2 ** 20;

// ---- Qwen3.8-Flash-Next with QSA in rooms (long context, step 1: QSA loaded, the room still at 2048 positions)
// Below 2052 cached cells QSA's selection is every cell, so a QSA engine gives a dense one's bits at 2048; loading it
// now lets checkpoints, memory and mixed rooms be checked before the context goes up.
// FLASH_QSA_RING: the raw indexer keys' ring rows (engine/qwen4exp_mem.js QSA_RAW_RING). FLASH_QSA_LAYER_BYTES: what QSA
// adds on each of the 12 QSA layers at 2048 positions: the indexer weights (3,277,824 B, from the header) and its
// state, the ring (1024 rows x 128 x f16) and the pooled keys (512 blocks x 128 x f16). The planner counts them per
// layer (flashLayerCost with qsa: FLASH_ROOM_QSA), as the load check does (shard.js flashShardBytes).
export const FLASH_QSA_RING = 1024;
export const FLASH_QSA_LAYER_BYTES = 3277824 + FLASH_QSA_RING * 256 + 512 * 256;
// What a build that loads Flash with QSA says in its hello (meta.flashLong, additive, no PROTOCOL bump): ctx, the longest
// context its loader takes (QSA's 411,000, or what one f32 K / V buffer of 2 KV heads x 256 fits in maxBindMB); kv, its
// K / V formats; batch, whether it runs batched prefill passes (1 by default: room/flash.js with a Qwen4ExpBatch; a node
// with POOLED_FLASH_BATCH=0 says 0; informational, ai-ready.batch is what the host acts on). A device without it is an
// older build: dense, 2048 positions.
// ctx16 (rooms long-context PR 5 / 9): the same for f16 K / V, which this build also loads (kv lists "f16"); an older
// build without it loads f32 only.
export function flashLongMeta(maxBindMB, { batch = 1 } = {}) {
  const bind = (+maxBindMB > 0 ? +maxBindMB : WEBGPU_MIN_BIND / 2 ** 20) * 2 ** 20;
  return { ctx: Math.min(411000, Math.floor(bind / (2 * 256 * 4))), ctx16: Math.min(411000, Math.floor(bind / (2 * 256 * 2))), kv: ["f32", "f16"], batch: batch ? 1 : 0 };
}
const flashLongOk = (m) => !!m?.flashLong && typeof m.flashLong === "object" && +m.flashLong.ctx >= 2048 && Array.isArray(m.flashLong.kv) && m.flashLong.kv.includes("f32");
// The host's Flash format for ai-load (ai-load.flash): QSA on, f32 K / V, the 1024-row ring, when every device it dealt
// layers to says it loads that (meta.flashLong); else null: no field, every device loads the dense 2048 engine as before.
// One format for the whole room, chosen by the host only.
export function flashRoomFormat(model, metas) {
  if (MODELS[model]?.kind !== "qwen4exp" || !metas.length || !metas.every(flashLongOk)) return null;
  return { qsa: 1, kv: flashKvOf(metas), ring: FLASH_QSA_RING };
}
// The rooms' K / V format for Flash: f16 (half f32's memory per token: 128K fits one 128 MiB binding) when every device
// loads it (meta.flashLong.kv), else f32 (a build from before f16 in rooms)
export const FLASH_ROOM_KV = "f16";
const flashKvOf = (metas) => (metas.every((m) => Array.isArray(m?.flashLong?.kv) && m.flashLong.kv.includes(FLASH_ROOM_KV)) ? FLASH_ROOM_KV : "f32");
// What a Flash room's devices (metas, host first; every device that may be dealt layers) allow, before any deal: kv, the
// K / V format they'd load (flashRoomFormat's), and cap, the longest context every one of them takes: its
// meta.flashLong ctx16 (f16) or ctx (f32), 2048 for a device without flashLong (an older build: dense, 2048).
// -> { kv, cap } (kv also for the planner: roomBytes(model, ctx, kv)); any other model: { kv: null, cap: Infinity }
export function flashRoomLimits(model, metas) {
  if (MODELS[model]?.kind !== "qwen4exp") return { kv: null, cap: Infinity };
  const ms = metas.filter(Boolean);
  if (!ms.length || !ms.every(flashLongOk)) return { kv: "f32", cap: FLASH_DENSE_CTX };
  const kv = flashKvOf(ms);
  return { kv, cap: Math.min(...ms.map((m) => +(kv === "f16" ? m.flashLong.ctx16 || m.flashLong.ctx : m.flashLong.ctx) || FLASH_DENSE_CTX)) };
}
// A worker's reading of ai-load.flash: null for none (the dense engine, at most 2048 positions), else
// { kv, ring } for a QSA engine; a format this build does not load throws (said back to the host as a load error).
export function flashLoadOpts(flash) {
  if (!flash?.qsa) return null;
  const kv = flash.kv ?? "f32", ring = flash.ring ?? FLASH_QSA_RING;
  if (kv !== "f32" && kv !== "f16") throw new Error(`the host asked for Qwen3.8-Flash-Next's K / V in ${String(kv).slice(0, 16)}; this device loads f32 or f16`);
  if (!(Number.isInteger(ring) && ring >= 64 && ring <= 65536 && (ring & (ring - 1)) === 0)) throw new Error(`bad QSA ring ${String(ring).slice(0, 16)}`);
  return { kv, ring };
}
// A worker's ai-ready, Flash only: ctx, the positions its engine holds, and fsig, its state signature (kv, qsa, qsaRing,
// lo, hi), so the host sees what was loaded. -> fields to spread into ai-ready ({} for any other model)
export function flashReadyFields(engine, model) {
  if (MODELS[model]?.kind !== "qwen4exp" || !engine) return {};
  let sig = null;
  try { sig = typeof engine.stateSignature === "function" ? engine.stateSignature() : null; } catch {}
  return { ctx: +engine.maxSeq || 0, ...(sig ? { fsig: sig } : {}) };
}
// The host, on a worker's ai-ready (d) in a Flash room of roomCtx positions with format flash (flashRoomFormat):
// why its engine can't serve the room, or null. A worker that sends no ctx (an older build) holds 2048.
export function readyCtxShort(model, roomCtx, d, flash = null) {
  if (MODELS[model]?.kind !== "qwen4exp" || !(roomCtx > 0)) return null;
  const has = +d?.ctx > 0 ? +d.ctx : 2048;
  if (has < roomCtx) return `its engine holds ${has} positions and the room runs ${roomCtx}`;
  const f = d?.fsig && typeof d.fsig === "object" ? d.fsig : null;
  if (!f) return null;
  if (flash && f.qsa && (f.kv !== flash.kv || f.qsaRing !== flash.ring))
    return `its engine keeps K / V in ${String(f.kv).slice(0, 8)} with a ${+f.qsaRing || 0}-row ring; the room asked for ${flash.kv}, ${flash.ring}`;
  // the other engine type: a dense engine on layers with a QSA layer in a QSA room, or a QSA engine in a dense room
  // (a range with no QSA layer reports qsa false in either)
  if (!!flash !== !!f.qsa && flashQsaIn(model, f.lo, f.hi))
    return `its engine runs ${f.qsa ? "QSA" : "dense attention"}; the room asked for ${flash ? "QSA" : "dense attention"}`;
  return null;
}
// Does layers [lo, hi) of a Flash model hold a QSA (full-attention) layer? Every interval-th layer is one
// (engine/gguf.js qwen4expConfig's rule, SHAPE's interval); a range it can't read counts as holding one.
export function flashQsaIn(model, lo, hi) {
  const s = SHAPE[model], every = +s?.meta?.["qwen35.full_attention_interval"] || 4;
  if (!Number.isInteger(lo) || !Number.isInteger(hi) || hi <= lo) return true;
  for (let i = lo; i < hi; i++) if ((i + 1) % every === 0) return true;
  return false;
}
// bytes of one attention layer's K (or V) cache buffer at ctx positions: kvDim x 1 (int8) or 2 (f16)
export const kvLayerBufBytes = (meta, ctx, kv = "f16") =>
  ctx * (meta["qwen35.attention.head_count_kv"] || 0) * (meta["qwen35.attention.key_length"] || 0) * (kv === "q8" ? 1 : 2);
// The longest context (<= ctx, a multiple of 256) whose per-layer K/V buffer fits bindBytes, the
// smallest binding limit among a room's devices (a device that reports none counts as WebGPU's 128 MiB).
export const ctxForBinding = (meta, ctx, kv = "f16", bindBytes = WEBGPU_MIN_BIND) => {
  const per = kvLayerBufBytes(meta, 1, kv);
  if (!per) return ctx;
  return Math.min(ctx, Math.floor(Math.max(bindBytes, WEBGPU_MIN_BIND) / per / 256) * 256);
};
// The longest context a room opens `model` at without an ask: the top of its ladder (27B 64K, MoE 128K, Flash 64K), else its
// default (the 1.7B: 16K). Rooms open there only when their memory holds it at full speed (roomCtxPlan).
export const ctxTop = (model) => CTX[model]?.ladder?.[0] ?? maxSeqFor(model);
// The shortest context a room opens `model` at (its fallback, or its ladder's last step), or null without one below def
export const ctxFloor = (model) => {
  const c = CTX[model], lo = c?.fallback ?? c?.ladder?.[c.ladder.length - 1];
  return lo > 0 && lo < c.def ? lo : null;
};
export const maxSeqFor = (model, ask = 0) => {
  const c = CTX[model];
  if (!c) return MODELS[model]?.kind === "qwen35" ? MAX_SEQ_LONG : MAX_SEQ;
  return ask > 0 ? Math.min(c.max, Math.max(2048, Math.round(ask / 256) * 256)) : c.def;
};
// The contexts a room tries for `model`, best first: `want` (the context it would open with), then the
// model's fallback when it has one below `want`, or (Qwen3.8-Flash-Next: CTX ladder) every ladder step below `want`.
// An explicit ask (?ctx=, --ctx, the picker's context choice) is tried alone.
export const ctxChoices = (model, want = maxSeqFor(model), ask = 0) => {
  const c = CTX[model], fb = c?.fallback;
  if (ask > 0) return [want];
  if (c?.ladder) return [want, ...c.ladder.filter((x) => x < want)];
  return !(fb > 0) || fb >= want ? [want] : [want, fb];
};
// The context a room opens with: the first of ctxChoices whose deal fits (fitsAt(ctx) -> bool), else
// the smallest of them (what a short room is short for: the least it could start with).
// -> { ctx, want, fits, fellBack } (fellBack: it fits, but only at a shorter context than `want`)
// cap: the longest context the room's devices take (flashRoomLimits): longer choices are skipped, and when every choice
// is longer the room is tried at the cap alone
// fastAt(ctx) -> bool (room/ctxpick.js roomCtxPlan, a room opened without a choice): a step longer than the model's def is
// taken only when fastAt says the room runs it at full speed there; want and fellBack are then measured against def (a
// room at 32K of the MoE's 128K ladder did not fall back: 32K is its default; one at 16K did)
export function pickCtx(model, { want = maxSeqFor(model), ask = 0, fitsAt, cap = Infinity, fastAt = null }) {
  let cs = ctxChoices(model, want, ask).filter((c) => c <= cap);
  if (!cs.length) cs = [Math.min(want, cap)];
  const def = CTX[model]?.def ?? want;
  const ok = (c) => !!fitsAt(c) && (!fastAt || c <= def || !!fastAt(c));
  const i = cs.findIndex(ok);
  const ctx = cs[i < 0 ? cs.length - 1 : i];
  const aim = fastAt && !(ask > 0) ? Math.min(want, def) : want;
  return { ctx, want: aim, fits: i >= 0, fellBack: i >= 0 && (fastAt ? ctx < aim : i > 0) };
}
// "16K", "8K": a context in words
export const ctxK = (n) => `${Math.round(n / 1024)}K`;
// What the room says where it shows the model when it fell back: "Qwen3 1.7B · 8K context: the
// room's memory is short for 16K"
export const ctxShortNote = (label, ctx, want) => `${label} · ${ctxK(ctx)} context: the room's memory is short for ${ctxK(want)}`;
// "needs 5.6 GB (4 GB at 8K)": a model's need at its default, and at its fallback when it has one
export const needText = (model, gb = NEED_GB[model]) => {
  const fb = ctxFloor(model), min = NEED_MIN_GB[model];
  return fb && min != null ? `${gb} GB (${min} GB at ${ctxK(fb)})` : `${gb} GB`;
};

// KV cache format per room: "f16" (the default) or "q8" (int8 values + one f32 scale per 32, engine
// kvQ8, ~56% of f16's memory; asked for with ?kv=q8). Only the qwen35 engine has the int8 kernels, so
// any other model, or any other ask, stays f16. The host decides and sends it with ai-load, so every
// device of a room keeps the same format and the layer deal counts the right bytes.
export const KV_MODES = ["f16", "q8"];
export const kvModeFor = (model, ask) => (ask === "q8" && MODELS[model]?.kind === "qwen35" ? "q8" : "f16");
// A device's format for an ai-load: the host's `kv` when it sent one (so a worker's own ?kv= cannot
// make it differ from the room), else this device's own ask (a host from before the field existed).
export const kvForLoad = (model, sent, ownAsk) => kvModeFor(model, sent ?? ownAsk);
// K+V bytes per position, averaged over a model's layers (for dealing layers by memory), f16 or int8
export const kvBytesPerLayerPos = (meta, kv = "f16") => {
  const kvDim = (meta["qwen35.attention.head_count_kv"] || 0) * (meta["qwen35.attention.key_length"] || 0);
  const perKV = kv === "q8" ? kvDim + kvDim / 32 * 4 : kvDim * 2;   // int8 values + f32 scales, or f16
  return perKV * 2 / (meta["qwen35.full_attention_interval"] || 1);
};
// The browser snapshots these loading options when it creates its loader. Null prefetch keeps
// the live device policy; engine presets are separate and read when an engine is constructed.
// ?wcache=0 disables converted weights; ?wcacheverify=1 verifies them; ?prefetch=N overrides lookahead.
// workers of a Flash tab's first save of its experts (?saveworkers=N overrides; 0: the main thread)
export const SAVE_WORKERS = 4;
export function modelLoadOptions(search) {
  const q = new URLSearchParams(search), prefetch = q.get("prefetch");
  // parload=0: a Flash tab's load parts one after the other, as before room/flashload.js; ngramworker=0: its n-gram
  // table copied on the main thread (after the experts'). The experts' first save (room/flashdisk.js expertTierCopy):
  // saveworkers=N copies in N workers (0: on the main thread, as before), savechunk / saveahead: experts a read and
  // reads in flight per worker, saveearly=0: the save waits for the GPU step even in workers; cdnresolve=0: every
  // range request goes through the model's URL (Hugging Face's 302) instead of the CDN URL it redirected to;
  // diskbatch=0: a tab whose layers read experts from its disk reads prompts one column at a time, as before;
  // layered=0: its batched prompt passes go pass after pass through every layer (each disk layer read once a pass)
  const int = (k, d, lo = 0, hi = 64) => { const v = parseInt(q.get(k) ?? "", 10); return Number.isFinite(v) ? Math.max(lo, Math.min(hi, v)) : d; };
  return { wcache: q.get("wcache") !== "0", wcacheVerify: q.get("wcacheverify") === "1",
    prefetch: prefetch === null ? null : Math.max(0, parseInt(prefetch, 10) || 0),
    parload: q.get("parload") !== "0", ngramWorker: q.get("ngramworker") !== "0",
    save: { workers: int("saveworkers", SAVE_WORKERS), chunk: int("savechunk", 8, 1, 64), ahead: int("saveahead", 3, 1, 16), early: q.get("saveearly") !== "0" },
    cdnResolve: q.get("cdnresolve") !== "0", diskBatch: q.get("diskbatch") !== "0", layered: q.get("layered") !== "0" };
}

export const MAX_NEW = 400;    // longest answer, tokens
export const MAX_NEW_THINKING = 1200;   // with thinking on, the think block comes out of the same budget
export const MIN_ROOM = 32;    // a prompt must leave at least this many tokens for the answer

// Flash's "needs N GB": the planner's own bytes at its default context (roomBytes: every layer's cost, the host's part),
// rounded up to a tenth, so the picker's number follows the memory model instead of a fixed figure
NEED_GB["qwen3.8-flash"] = (() => { const rb = roomBytes("qwen3.8-flash", CTX["qwen3.8-flash"].def); return Math.ceil((rb.L * rb.layerBytes + rb.hostBytes) / 2 ** 30 * 10 - 1e-9) / 10; })();
// What a model's whole deal needs at ctx, in GiB rounded up to a tenth (roomBytes: every layer with its KV cache there and
// what the host holds besides), or null for a model without a SHAPE: NEED_GB's own rule
export const needGBAt = (model, ctx, kv = "f16") => {
  const rb = roomBytes(model, ctx, kv);
  return rb ? Math.ceil((rb.L * rb.layerBytes + rb.hostBytes) / 2 ** 30 * 10 - 1e-9) / 10 : null;
};
// the 27B and the MoE at their ladder's last step (8K): the least a room opens them with ("17 GB (16.5 GB at 8K)"), as
// NEED_GB less the KV cache 8K saves against def (NEED_GB's label sits a little above the deal's bytes; so does this)
for (const k of ["qwen3.8-27b", "qwen3.6-35b-moe"]) NEED_MIN_GB[k] = +(NEED_GB[k] - (needGBAt(k, CTX[k].def) - needGBAt(k, ctxFloor(k)))).toFixed(1);
