// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/qwen4exp_mem.js (MIT; see LICENSE-ENGINE).
// Memory of Qwen3.8-Flash-Next's per-sequence state (engine/qwen4exp.js) by context length, for the planner
// (room/plan.js, later) and the long-context notes (docs/kernels.md "Flash long context"). Pure arithmetic on
// qwen4expConfig: no GPU, so the planner and the unit tests (tests/unit/q38flash_mem_test.js) can call it; the engine
// allocates exactly these sizes (Qwen4ExpEngine.stateBytes(), checked against this in tests/test_q38flash_ckpt.js).
//
// Per full-attention (QSA) layer:   K and V caches  2 x ctx x nKV x headDim x (4 f32 | 2 f16)
//                                    QSA rawK        (ring rows | ctx) x 128 x 2   (f16 raw indexer keys)
//                                    QSA pooled      ceil(ctx / 4) x 128 x 2       (f16 pooled keys, every block)
// Per DeltaNet layer (fixed):        S               nVHeads x 128 x 128 x 4
//                                    conv window     convDim x (convK - 1) x 4
// The PLE layer (fixed):             ring            ((K1 - 1) x ngram + 1) x hDim x 4
// Per-pass scratch that grows with the context: QSA's block scores, maxCols x ceil(ctx / 4) x 4 per batched pass
// object (and 1 column for decode); the rest of the scratch is fixed (MBs) and left out.
// "before" (main before the long-context work): kv "f32", rawRing 0 (rawK for every position).
import { QSA_KPOOL, QSA_ID, QSA_MAX_SEQ } from "./wgsl/qsa.js";

// rawK ring rows by default: passes of up to 1021 columns (256 KB per QSA layer)
export const QSA_RAW_RING = 1024;

const al16 = (n) => Math.max(16, Math.ceil(n / 16) * 16);   // the engine's buffer rounding

// cfg: qwen4expConfig. ctx: the context (the engine's maxSeq). layerRange: [lo, hi) of the trunk this device runs.
// kv: the K / V format; the default is the engine's ("f32"), so a planner sizing an engine built with defaults never
// under-counts; pass kv: "f16" for an engine created with it.
// qsa: the full-attention layers keep the indexer state (false: the dense path, ctx <= 2048).
// -> { ctx, kv, rawRing, bytes: { kv, rawK, pooled, deltaNet, ple, scores }, state (kv + rawK + pooled + deltaNet +
//    ple: what a checkpoint of a full context holds), total (state + scores), perToken (the bytes each position adds) }
export function qwen4expMemory(cfg, { ctx, kv = "f32", rawRing = QSA_RAW_RING, qsa = true, layerRange = null, maxCols = 64, batch = true } = {}) {
  if (!(Number.isInteger(ctx) && ctx > 0)) throw new Error(`qwen4expMemory: ctx ${ctx}`);
  if (kv !== "f32" && kv !== "f16") throw new Error(`qwen4expMemory: KV format ${kv}`);
  const [lo, hi] = layerRange || [0, cfg.L];
  let nQ = 0, nD = 0;
  for (let i = lo; i < hi; i++) { if (cfg.qsa[i]) nQ++; else nD++; }
  const kvB = kv === "f16" ? 2 : 4, ss = cfg.ssm;
  const convDim = 2 * ss.nKHeads * ss.stateSize + ss.inner;
  const blocks = Math.ceil(ctx / QSA_KPOOL);
  const bytes = {
    kv: nQ * 2 * al16(ctx * cfg.nKVHeads * cfg.headDim * kvB),
    rawK: qsa ? nQ * al16((rawRing || ctx) * QSA_ID * 2) : 0,
    pooled: qsa ? nQ * al16(blocks * QSA_ID * 2) : 0,
    deltaNet: nD * (al16(ss.nVHeads * ss.stateSize * ss.stateSize * 4) + al16(convDim * (ss.convK - 1) * 4)),
    ple: 0,
    scores: qsa && nQ ? al16(blocks * 4) + (batch ? al16(maxCols * blocks * 4) : 0) : 0,
  };
  const pleL = (cfg.ple?.layers || []).filter((l) => l >= lo && l < hi).length;
  if (pleL) bytes.ple = pleL * al16(((cfg.ple.convK - 1) * cfg.ple.ngram + 1) * cfg.hDim * 4);
  const state = bytes.kv + bytes.rawK + bytes.pooled + bytes.deltaNet + bytes.ple;
  const perToken = nQ * (2 * cfg.nKVHeads * cfg.headDim * kvB + (qsa ? (rawRing ? 0 : QSA_ID * 2) + QSA_ID * 2 / QSA_KPOOL : 0));
  return { ctx, kv, rawRing: qsa ? rawRing : 0, layers: { qsa: nQ, deltaNet: nD }, bytes, state, total: state + bytes.scores, perToken };
}

// The engine's hard ceiling on the context for these options: QSA_MAX_SEQ with QSA (qsa_cs's 2 pi reduction), 2048
// dense, and (maxBinding: the device's min(maxStorageBufferBindingSize, maxBufferSize)) the per-layer K / V buffer,
// which the engine binds whole: 128 MiB is 65536 positions in f32, 131072 in f16.
export function qwen4expCtxCap(cfg, { kv = "f32", qsa = true, maxBinding = Infinity } = {}) {
  if (kv !== "f32" && kv !== "f16") throw new Error(`qwen4expCtxCap: KV format ${kv}`);
  const row = cfg.nKVHeads * cfg.headDim * (kv === "f16" ? 2 : 4);
  return Math.min(qsa ? QSA_MAX_SEQ : 2048, Math.floor(maxBinding / row));
}

// The longest context whose state (and scores) fit in `budget` bytes, for a planner that knows a device's free memory,
// clamped to qwen4expCtxCap (a context the engine accepts). opts as qwen4expMemory, plus maxBinding.
export function qwen4expMaxCtx(cfg, budget, { maxBinding, ...opts } = {}) {
  let lo = 0, hi = qwen4expCtxCap(cfg, { kv: opts.kv, qsa: opts.qsa ?? true, maxBinding });
  if (hi <= 0) return 0;
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2);
    if (qwen4expMemory(cfg, { ...opts, ctx: mid }).total <= budget) lo = mid; else hi = mid - 1;
  }
  return lo;
}
