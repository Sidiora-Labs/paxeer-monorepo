// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/qsa.js (MIT; see LICENSE-ENGINE).
// Host side of the QSA kernels (engine/wgsl/qsa.js, attn_dec.js's gather mode): shapes, buffers, the pass order
// and dispatch sizes. Standalone so the Flash path (engine/qwen35.js) can adopt it per QSA layer; tests/test_qsa.js
// and tests/bench_qsa.js drive it directly. docs/kernels.md "QSA" has the integration notes.
//
// One pass = nCols consecutive tokens at positions pos .. pos + nCols - 1 of one sequence (decode: 1; prefill
// chunk or verify pass: more). Per QSA layer, in order:
//   proj   (optional) qsa_proj: proj = [index_q_proj ; index_k_proj] . x (BF16), or the engine writes proj itself
//   prep   qsa_prep:  qi (normed, roped indexer q) and rawK[pos ..] (state)
//   pool   qsa_pool:  pooled[b] for the blocks the pass completes (state)
//   score  qsa_score / qsa_score_c4: skipped when no column sees more than 512 blocks
//   select qsa_select: the per-column KV row list
//   (the engine stores this pass's K / V rows into its KV cache here, as for dense attention)
//   attn   attn_dec_qsa + attn_dec_qsa_combine(_g): the pre-gate attention output, attn_dec's layout
// State per sequence and layer: rawK (maxSeq x 256 B) and pooled (maxSeq / 4 x 256 B), both f16. Everything else
// is per-pass scratch shared by all layers.

import { qsaWGSL, QSA_KPOOL, QSA_NTOP, QSA_MAXSEL, QSA_SEL_STRIDE, QSA_PROJ, QSA_IH, QSA_ID, QSA_MAX_SEQ } from "./wgsl/qsa.js";
import { attnDecWGSL, attnDecCombineGWSL, attnDecConfig, decSplits, DEC_CQ } from "./wgsl/attn_dec.js";

export { QSA_KPOOL, QSA_NTOP, QSA_MAXSEL, QSA_SEL_STRIDE, QSA_PROJ, QSA_MAX_SEQ };
export const QSA_SCORE_CW = 4;   // columns per qsa_score workgroup for multi-column passes

// attn_dec shape for the QSA attention: G = nH / nKV query heads per kv head, split into R workgroups of <= 8
export function qsaAttnConfig({ hd, nH, nKV, splits = 0 }) {
  const G = nH / nKV;
  let R = 1; while (G / R > 8 || G % R) R++;
  const c = attnDecConfig({ hd, G: G / R, nKV: nKV * R, R, splits });
  return c && { ...c, nKVa: nKV * R };
}

// blocks visible to the last column of a pass, and whether any column needs scoring (sees > 512 blocks)
export const qsaVisible = (pos, nCols) => Math.floor((pos + nCols) / QSA_KPOOL);
export const qsaNeedsScore = (pos, nCols) => qsaVisible(pos, nCols) > QSA_NTOP;
// the most KV rows any column of the pass attends to (the attention dispatch bound)
export const qsaMaxRows = (pos, nCols) => Math.min(pos + nCols, QSA_MAXSEL);

// rope theta scale as ggml computes it: powf(base, -2 / nRot) (ggml-cpu ops.cpp:6371)
export const qsaThetaScale = (nRot, base) => Math.fround(Math.pow(base, -2 / nRot));

// dispatch sizes of a pass: { kernel: [x, y, z] }; score is null when skipped
export function qsaDispatch({ pos, nCols, adCfg, nH, nKVa }) {
  const nvMax = qsaVisible(pos, nCols);
  const b0 = Math.floor(pos / QSA_KPOOL), b1 = Math.floor((pos + nCols - 1) / QSA_KPOOL);
  const cw = nCols === 1 ? 1 : QSA_SCORE_CW;
  return {
    proj: [Math.ceil(QSA_PROJ / 8), nCols, 1],
    prep: [QSA_IH + 1, nCols, 1],
    pool: [b1 - b0 + 1, 1, 1],
    score: qsaNeedsScore(pos, nCols) ? { entry: cw === 1 ? "qsa_score" : "qsa_score_c4", size: [Math.ceil(nvMax / 256), Math.ceil(nCols / cw), 1] } : null,
    select: [nCols, 1, 1],
    attn: [decSplits(qsaMaxRows(pos, nCols), adCfg.S), nCols, nKVa],
    combine: [nH * DEC_CQ, nCols, 1],
  };
}

// The QS uniform (engine/wgsl/qsa.js): column strides and the norm / rope constants
export function qsaUniform({ xStride, scoreStride, nRot = 64, eps = 1e-6, theta = 1e7 }) {
  const b = new ArrayBuffer(32), u = new Uint32Array(b), f = new Float32Array(b);
  u.set([xStride, QSA_PROJ, QSA_IH * QSA_ID, scoreStride, QSA_SEL_STRIDE, nRot]);
  f[6] = eps; f[7] = qsaThetaScale(nRot, theta);
  return new Uint8Array(b);
}

// buffer sizes (bytes). State: rawK, pooled (per sequence and QSA layer). Scratch: proj, qi, scores, sel (per pass;
// scores is the big one: maxCols x maxSeq / 4 floats). There is no column-group path: every kernel covers the
// pass's frame.nCols columns, so the caller keeps the pass width <= qsaMaxCols (the scores binding must fit).
export function qsaSizes({ maxSeq, maxCols }) {
  if (maxSeq > QSA_MAX_SEQ) throw new Error(`QSA: maxSeq ${maxSeq} > ${QSA_MAX_SEQ} (qsa_cs's 2 pi reduction is exact only below it)`);
  const maxBlocks = Math.ceil(maxSeq / QSA_KPOOL);
  return {
    maxBlocks,
    rawK: maxSeq * QSA_ID * 2, pooled: maxBlocks * QSA_ID * 2,
    proj: maxCols * QSA_PROJ * 4, qi: maxCols * QSA_IH * QSA_ID * 4,
    scores: maxCols * maxBlocks * 4, sel: maxCols * QSA_SEL_STRIDE * 4,
  };
}

// the widest pass whose scores scratch fits one storage binding (WebGPU's default maxStorageBufferBindingSize is
// 128 MiB: at 262,144 positions a 512-column pass is exactly 128 MiB, so any longer context needs the adapter's
// limit or fewer columns). Long wide chunks are also score-bound: see docs/kernels.md "QSA" timings.
export function qsaMaxCols({ maxSeq, maxBinding = 128 << 20 }) {
  return Math.max(1, Math.floor(maxBinding / (Math.ceil(maxSeq / QSA_KPOOL) * 4)));
}

// group 1 binding specs per entry point (binding -> "ro" | "rw" | "u"); group 0 is the engine's [cfg, frame]
export const QSA_SPECS = {
  qsa_proj: ["ro", "ro", "rw", "u"],
  qsa_prep: ["ro", "ro", "rw", "rw", "u"],
  qsa_pool: ["ro", "ro", "rw", "u"],
  qsa_score: ["ro", "ro", "rw", "u"],
  qsa_score_c4: ["ro", "ro", "rw", "u"],
  qsa_select: ["ro", "rw", "u"],
  attn_dec_qsa: ["ro", "ro", "ro", "rw", "rw", "u", "ro"],
  attn_dec_qsa_combine: ["ro", "ro", "rw", "u", null, null, "ro"],
  attn_dec_qsa_combine_g: ["ro", "ro", "rw", "ro", "u", null, "ro"],
};
const MODULE_OF = { attn_dec_qsa: "attn", attn_dec_qsa_combine: "attn", attn_dec_qsa_combine_g: "attnG" };

function layoutOf(device, spec) {
  const type = { ro: "read-only-storage", rw: "storage", u: "uniform" };
  return device.createBindGroupLayout({ entries: spec.map((t, i) => t && { binding: i, visibility: GPUShaderStage.COMPUTE, buffer: { type: type[t] } }).filter(Boolean) });
}

// Compile the modules with explicit layouts (group 0 = [cfg, frame] uniforms, as the engine's layout0).
// Returns { pipes, layouts, adCfg, nKVa }.
export async function qsaPipelines(device, { hd, nH, nKV, splits = 0, layout0 = null }) {
  const adCfg = qsaAttnConfig({ hd, nH, nKV, splits });
  if (!adCfg) throw new Error(`QSA attention: unsupported shape hd ${hd} nH ${nH} nKV ${nKV}`);
  const mods = {
    qsa: device.createShaderModule({ code: qsaWGSL() }),
    attn: device.createShaderModule({ code: attnDecWGSL({ ...adCfg, gather: QSA_SEL_STRIDE }) }),
    attnG: device.createShaderModule({ code: attnDecCombineGWSL({ ...adCfg, gather: QSA_SEL_STRIDE }) }),
  };
  for (const [name, m] of Object.entries(mods)) {
    const info = await m.getCompilationInfo?.();
    const errs = (info?.messages || []).filter((x) => x.type === "error");
    if (errs.length) throw new Error(`QSA ${name} WGSL: ` + errs.map((e) => `${e.lineNum}: ${e.message}`).join("; "));
  }
  const l0 = layout0 || layoutOf(device, ["u", "u"]);
  const layouts = {}, pipes = {};
  await Promise.all(Object.entries(QSA_SPECS).map(async ([e, spec]) => {
    layouts[e] = layoutOf(device, spec);
    pipes[e] = await device.createComputePipelineAsync({
      layout: device.createPipelineLayout({ bindGroupLayouts: [l0, layouts[e]] }),
      compute: { module: mods[MODULE_OF[e] || "qsa"], entryPoint: e } });
  }));
  return { pipes, layouts, layout0: l0, adCfg, nKVa: adCfg.nKVa };
}

// Bind groups for one layer's state + shared scratch. B: { cfg, frame, qs (QS uniform), fd (attn_dec FD uniform),
// x, w (BF16 [640][dim] as u32), proj, qNorm, kNorm, qi, rawK, pooled, scores, sel, q, kCache, vCache, faO, faML,
// out, qFull (gated combine only) }. An entry point whose buffers are not all given gets no group.
export function qsaBindGroups(device, K, B) {
  const g0 = device.createBindGroup({ layout: K.layout0, entries: [{ binding: 0, resource: { buffer: B.cfg } }, { binding: 1, resource: { buffer: B.frame } }] });
  const bufs = {
    qsa_proj: [B.x, B.w, B.proj, B.qs],
    qsa_prep: [B.proj, B.qNorm, B.qi, B.rawK, B.qs],
    qsa_pool: [B.rawK, B.kNorm, B.pooled, B.qs],
    qsa_score: [B.qi, B.pooled, B.scores, B.qs],
    qsa_score_c4: [B.qi, B.pooled, B.scores, B.qs],
    qsa_select: [B.scores, B.sel, B.qs],
    attn_dec_qsa: [B.q, B.kCache, B.vCache, B.faO, B.faML, B.fd, B.sel],
    attn_dec_qsa_combine: [B.faO, B.faML, B.out, B.fd, null, null, B.sel],
    attn_dec_qsa_combine_g: [B.faO, B.faML, B.out, B.qFull, B.fd, null, B.sel],
  };
  const G = {};
  for (const [e, list] of Object.entries(bufs)) {
    const spec = QSA_SPECS[e];
    if (!spec.every((t, i) => !t || list[i])) continue;
    G[e] = [g0, device.createBindGroup({ layout: K.layouts[e], entries: spec.map((t, i) => t && { binding: i, resource: { buffer: list[i] } }).filter(Boolean) })];
  }
  return G;
}

// Record one pass's QSA work into a compute pass. stages: which to run, in this order; "kv" is a callback slot
// for the engine's K / V store (between select and attn). gate: use attn_dec_qsa_combine_g (one column only).
export function encodeQsa(pass, pipes, G, d, { stages = ["proj", "prep", "pool", "score", "select", "attn"], kv = null, gate = false } = {}) {
  const run = (e, size) => { pass.setPipeline(pipes[e]); pass.setBindGroup(0, G[e][0]); pass.setBindGroup(1, G[e][1]); pass.dispatchWorkgroups(...size); };
  for (const s of stages) {
    if (s === "proj") run("qsa_proj", d.proj);
    else if (s === "prep") run("qsa_prep", d.prep);
    else if (s === "pool") run("qsa_pool", d.pool);
    else if (s === "score") { if (d.score) run(d.score.entry, d.score.size); }
    else if (s === "select") run("qsa_select", d.select);
    else if (s === "attn") {
      if (kv) kv(pass);
      run("attn_dec_qsa", d.attn);
      run(gate ? "attn_dec_qsa_combine_g" : "attn_dec_qsa_combine", gate ? [d.combine[0], 1, 1] : d.combine);
    }
  }
}
