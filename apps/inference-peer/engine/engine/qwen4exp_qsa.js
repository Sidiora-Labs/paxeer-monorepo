// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/qwen4exp_qsa.js (MIT; see LICENSE-ENGINE).
// QSA (Qwen Sparse Attention) wired into the Qwen3.8-Flash-Next path: the indexer kernels of PR #344
// (engine/wgsl/qsa.js, host notes in engine/qsa.js and docs/kernels.md "QSA") on the qwen4exp engine's buffers.
// Roadmap 33 PR plan item 9. Used by the decode engine (engine/qwen4exp.js, one column) and the batched passes
// (engine/qwen4exp_batch.js, nCols columns), which share each layer's indexer state.
//
// Per QSA layer and pass, on the layer's attention input x (the HC mixer's output):
//   proj  qsa_proj:  [index_q_proj ; index_k_proj] . x (BF16)            always
//   prep  qsa_prep:  qi (normed, roped indexer query); rawK[pos ..]        always (the state must be complete
//   pool  qsa_pool:  pooled keys of the blocks this pass completes        before the context passes 2051)
//   then, only when some column of the pass has more than 2051 cells (pos + nCols > 2051):
//   score qsa_score(_c4), when some column sees more than 512 blocks
//   select qsa_select: each column's KV row list (all rows 0 .. p while it has <= 2051)
//   and the gathered attention (engine/wgsl/qwen4exp_batch.js attnDecMcWGSL({ gather })) over the engine's KV.
// While every column has <= 2051 cells QSA selects them all, so the engine keeps its dense attention there
// (bit-equal to the gathered kernel over the full list: tests/test_q38flash_batch.js).
//
// The attention itself is the engine's (its f32 or f16 KV, one workgroup per head), not attn_dec_qsa from #344,
// which reads the Qwen3.5 engine's f16 KV through its split-K layout (same row lists).
// rawK is a ring on this path (qsaFlashPipelines / qsaLayerState rawRing, QSA_RAW_RING rows by default): pooling
// reads only the rows of the blocks a pass completes, so the ring needs pass columns + 3 rows, not the context.
import { qsaWGSL, qsaRingOk, QSA_KPOOL, QSA_NTOP, QSA_MAXSEL, QSA_SEL_STRIDE, QSA_PROJ, QSA_IH, QSA_ID, QSA_MAX_SEQ } from "./wgsl/qsa.js";
import { QSA_SPECS, qsaUniform, QSA_SCORE_CW } from "./qsa.js";
import { QSA_RAW_RING } from "./qwen4exp_mem.js";

export { QSA_MAXSEL, QSA_SEL_STRIDE, QSA_KPOOL, QSA_ID };
export { QSA_RAW_RING };
// the widest pass a ring of R rows serves (the rows of the blocks it completes: up to nCols + 3)
export const qsaRingCols = (R) => (R ? R - (QSA_KPOOL - 1) : Infinity);
const U = GPUBufferUsage;
const ENTRIES = ["qsa_proj", "qsa_prep", "qsa_pool", "qsa_score", "qsa_score_c4", "qsa_select"];

// The QSA kernels have the indexer's shape built in (engine/wgsl/qsa.js constants): refuse a model whose metadata
// differs, rather than run them on the wrong shape. cfg: qwen4expConfig.
export function qsaCheckConfig(cfg) {
  const ix = cfg.indexer || {}, want = { nHeads: QSA_IH, headDim: QSA_ID, topK: QSA_NTOP * QSA_KPOOL, block: QSA_KPOOL };
  const bad = Object.entries(want).filter(([k, v]) => ix[k] !== v);
  if (bad.length) throw new Error("qwen4exp QSA: the kernels are built for indexer " + JSON.stringify(want) + ", the model has " + JSON.stringify(ix));
  // the indexer q and pooled k are roped with the model's n_rot (llama.cpp qwen4exp.cpp:785, 801), in rotary pairs
  if (!(cfg.ropeDims > 0 && cfg.ropeDims % 2 === 0 && cfg.ropeDims <= QSA_ID)) throw new Error("qwen4exp QSA: rope dims " + cfg.ropeDims + " for indexer heads of " + QSA_ID);
}

// true when a pass of nCols columns from pos needs the sparse path (some column has more than 2051 cells)
export const qsaSparse = (pos, nCols) => pos + nCols > QSA_MAXSEL;

// pipelines (explicit layouts: group 0 = [cfg, frame] uniforms, group 1 = the kernel's buffers)
export async function qsaFlashPipelines(device, { rawRing = QSA_RAW_RING } = {}) {
  const module = device.createShaderModule({ code: qsaWGSL({ rawRing }), label: "qsa" });
  const info = await module.getCompilationInfo?.();
  const errs = (info?.messages || []).filter((m) => m.type === "error");
  if (errs.length) throw new Error("QSA WGSL: " + errs.map((e) => `${e.lineNum}: ${e.message}`).join("; "));
  const type = { ro: "read-only-storage", rw: "storage", u: "uniform" };
  const lay = (spec) => device.createBindGroupLayout({ entries: spec.map((t, i) => t && { binding: i, visibility: GPUShaderStage.COMPUTE, buffer: { type: type[t] } }).filter(Boolean) });
  const layout0 = lay(["u", "u"]), layouts = {}, pipes = {};
  await Promise.all(ENTRIES.map(async (e) => {
    layouts[e] = lay(QSA_SPECS[e]);
    pipes[e] = await device.createComputePipelineAsync({ label: e, layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layouts[e]] }),
      compute: { module, entryPoint: e } });
  }));
  return { pipes, layouts, layout0 };
}

// The engine-wide uniforms: cfg (the Qwen3.5 Config layout qsa.js reads; only dim is used) and QS (strides, eps,
// rope). maxSeq sets the score stride.
export function qsaShared(device, { dim, maxSeq, eps, theta, nRot }) {
  if (maxSeq > QSA_MAX_SEQ) throw new Error(`qwen4exp QSA: maxSeq ${maxSeq} > ${QSA_MAX_SEQ} (qsa_cs's 2 pi reduction is exact only below it)`);
  const maxBlocks = Math.ceil(maxSeq / QSA_KPOOL);
  const cfgU = new ArrayBuffer(48);
  new Uint32Array(cfgU).set([dim, 0, 0, 0, 0, 0, 0, maxSeq]);
  new Float32Array(cfgU, 32, 1)[0] = eps;
  const ub = (bytes) => { const b = device.createBuffer({ size: Math.ceil(bytes.byteLength / 16) * 16, usage: U.UNIFORM | U.COPY_DST }); device.queue.writeBuffer(b, 0, bytes); return b; };
  return { maxBlocks, cfg: ub(new Uint8Array(cfgU)), qs: ub(qsaUniform({ xStride: dim, scoreStride: maxBlocks, nRot, eps, theta })) };
}

// Per-pass scratch for up to maxCols columns, and the pass frame (pos, seqLen, nCols, 0).
export function qsaScratch(device, { maxCols, maxBlocks }) {
  const sb = (n) => device.createBuffer({ size: Math.max(16, Math.ceil(n / 16) * 16), usage: U.STORAGE | U.COPY_SRC | U.COPY_DST });
  return { maxCols, proj: sb(maxCols * QSA_PROJ * 4), qi: sb(maxCols * QSA_IH * QSA_ID * 4), scores: sb(maxCols * maxBlocks * 4),
    sel: sb(maxCols * QSA_SEL_STRIDE * 4), frame: device.createBuffer({ size: 16, usage: U.UNIFORM | U.COPY_DST }) };
}
export const qsaSetFrame = (device, S, pos, nCols) => device.queue.writeBuffer(S.frame, 0, new Uint32Array([pos, pos + 1, nCols, 0]));

// A QSA layer's state (rawK, pooled: f16, per sequence) and its indexer weights on the GPU. rawRing: rawK's rows
// (a ring, as the pipelines were built with), 0 for every position. L.indexer: { q, k,
// qNorm, kNorm } as qwen4expWeights loads them (BF16 projections dequantized to f32 on the CPU: exact, so packing
// them back to BF16 loses nothing).
export function qsaLayerState(device, L, { maxSeq, dim, rawRing = QSA_RAW_RING }) {
  if (!qsaRingOk(rawRing)) throw new Error(`qwen4exp QSA: rawK ring of ${rawRing} rows`);
  const ix = L.indexer;
  if (!ix || !ix.q?.data || !ix.k?.data || !ix.qNorm?.data || !ix.kNorm?.data) throw new Error("qwen4exp QSA: layer without indexer weights (load with qsa: true)");
  if (ix.q.data.length !== QSA_IH * QSA_ID * dim || ix.k.data.length !== QSA_ID * dim) throw new Error("qwen4exp QSA: indexer shapes");
  const sb = (n) => device.createBuffer({ size: Math.max(16, Math.ceil(n / 16) * 16), usage: U.STORAGE | U.COPY_SRC | U.COPY_DST });
  const up = (a) => { const b = sb(a.byteLength); device.queue.writeBuffer(b, 0, a); return b; };
  const w = new Uint32Array(QSA_PROJ * dim / 2), qb = new Uint32Array(ix.q.data.buffer, ix.q.data.byteOffset, ix.q.data.length),
    kb = new Uint32Array(ix.k.data.buffer, ix.k.data.byteOffset, ix.k.data.length), nq = qb.length / 2;
  for (let i = 0; i < w.length; i++) {
    const src = i < nq ? qb : kb, j = 2 * (i < nq ? i : i - nq);
    w[i] = (src[j] >>> 16) | (src[j + 1] & 0xffff0000);
  }
  return { w: up(w), qNorm: up(ix.qNorm.data), kNorm: up(ix.kNorm.data),
    rawK: sb((rawRing || maxSeq) * QSA_ID * 2), pooled: sb(Math.ceil(maxSeq / QSA_KPOOL) * QSA_ID * 2), rawRing };
}

// Bind groups of one layer for one scratch set; x: the layer's attention input (column stride dim).
export function qsaBind(device, K, sh, S, st, x) {
  const g0 = device.createBindGroup({ layout: K.layout0, entries: [{ binding: 0, resource: { buffer: sh.cfg } }, { binding: 1, resource: { buffer: S.frame } }] });
  const bufs = {
    qsa_proj: [x, st.w, S.proj, sh.qs],
    qsa_prep: [S.proj, st.qNorm, S.qi, st.rawK, sh.qs],
    qsa_pool: [st.rawK, st.kNorm, st.pooled, sh.qs],
    qsa_score: [S.qi, st.pooled, S.scores, sh.qs],
    qsa_score_c4: [S.qi, st.pooled, S.scores, sh.qs],
    qsa_select: [S.scores, S.sel, sh.qs],
  };
  const G = {};
  for (const e of ENTRIES) G[e] = [g0, device.createBindGroup({ layout: K.layouts[e], entries: bufs[e].map((b, i) => ({ binding: i, resource: { buffer: b } })) })];
  return G;
}

// The engine ops (engine/qwen4exp.js op format, with dyn(pos, nCols) -> workgroups | null) of the indexer state
// update (proj, prep, pool), and of the sparse selection (score, select), which run only when the pass is sparse.
export function qsaStateOps(K, G) {
  const op = (e, dyn) => ({ pipe: K.pipes[e], bg: G[e], dyn, label: e });
  return [
    op("qsa_proj", (pos, n) => [Math.ceil(QSA_PROJ / 8), n, 1]),
    op("qsa_prep", (pos, n) => [QSA_IH + 1, n, 1]),
    op("qsa_pool", (pos, n) => [Math.floor((pos + n - 1) / QSA_KPOOL) - Math.floor(pos / QSA_KPOOL) + 1, 1, 1]),
  ];
}
// sparse(pos, nCols): the engine's choice (qsaSparse, or always for the QSA == dense check)
export function qsaSelectOps(K, G, sparse = qsaSparse) {
  const nv = (pos, n) => Math.floor((pos + n) / QSA_KPOOL);
  return [
    { pipe: K.pipes.qsa_score, bg: G.qsa_score, label: "qsa_score",
      dyn: (pos, n) => (sparse(pos, n) && n === 1 && nv(pos, n) > QSA_NTOP ? [Math.ceil(nv(pos, n) / 256), 1, 1] : null) },
    { pipe: K.pipes.qsa_score_c4, bg: G.qsa_score_c4, label: "qsa_score_c4",
      dyn: (pos, n) => (sparse(pos, n) && n > 1 && nv(pos, n) > QSA_NTOP ? [Math.ceil(nv(pos, n) / 256), Math.ceil(n / QSA_SCORE_CW), 1] : null) },
    { pipe: K.pipes.qsa_select, bg: G.qsa_select, label: "qsa_select", dyn: (pos, n) => (sparse(pos, n) ? [n, 1, 1] : null) },
  ];
}
