// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/engine/qwen35.js (MIT; see LICENSE-ENGINE).
// Qwen35Engine: the hybrid Gated-DeltaNet + attention engine (Qwen 3.5/3.6/3.8),
// with batched prefill/verify paths and multi-token-prediction speculation.
// See docs/architecture.md. Layer-shardable like DenseEngine. Golden
// reference: tests/reference/ref_q38.mjs (validated line by line against
// llama.cpp eval-callback dumps).
import { WGSL } from "./wgsl/base.js";
import { compilePipeline } from "./compile.js";
import { moduleSet } from "./wgsl/prune.js";
import { gemmWGSL, GEMM_S, GEMM_TILE } from "./wgsl/gemm.js";
import { gemmSgmWGSL, pickSgmConfig, sgmPlan, SGM_FEATURES, SGM_FEATURES_OPT, SGM_SYNTAX, SGM_DEFAULT } from "./wgsl/gemm_sgm.js";
import { gemmWideWGSL, wideTileConfig, gemmDp4aWGSL, dp4aTileConfig, probeDp4a, SILU_MUL_W_WGSL } from "./wgsl/gemm_wide.js";
import { coopWGSL, probeUnpack } from "./wgsl/coop.js";
import { layerFuseWGSL } from "./wgsl/layer_fuse.js";
import { WGSL2 } from "./wgsl/qwen35.js";
import { moeWGSL, moeFusedWGSL, moeKernelConfig, moeFusedLayout, normRouterKernel } from "./wgsl/moe.js";
import { attnTileWGSL, attnTileConfig } from "./wgsl/attn_tile.js";
import { attnDecWGSL, attnDecCombineGWSL, attnDecConfig, decSplits, DEC_CQ } from "./wgsl/attn_dec.js";
import { moeGroupWGSL, moeGroupSizes, dnGroupRows, tiledGroupWGSL, tileRows, dp4aGroupWGSL } from "./wgsl/moe_group.js";
import { f16ToF32, f32ToF16 } from "./gguf.js";
import { TOPK_MAX, topkK, readCands } from "./topk.js";
import { mapRead, KICK_DEFAULT } from "./map_kick.js";
import { REMAP_WGSL } from "./expert_store.js";

// the kernels each layerFuse flag dispatches ("pass" has none of its own)
const LF_PIPES = {
  dn: ["dn_delta_gnp"],
  conv: ["matvec_coop_cv", "matvec_q8_coop_cv", "matvec_q4_coop_cv"],
  kv: ["attn_glue_kv"],
  comb: ["attn_combine_g"],
  norm: ["matvec_coop_n", "matvec_q8_coop_n", "matvec_q4_coop_n"],
};

// Every Qwen35Engine method that reads or writes the sequence state, other than forwardTokenIds: each first drops
// a decode-ahead step forwardTokenIds may have queued (_settleAhead)
const AHEAD_SETTLE = ["reset", "exportState", "exportSlot", "importState", "saveSlot", "loadSlot", "dropSlot", "stateSignature", "_readParts",
  "forwardToken", "prefillTokens", "prefillToken", "prefillHidden", "embedRun", "runHidden", "headFromHidden", "headFromHiddenIds",
  "embedRunBatch", "runHiddenBatch", "headBatch", "headBatchIds", "verifyN", "specStep", "specStepDrafts", "mtpRun", "headAhead",
  "keepAhead", "dropAhead", "setHidden", "ffnOnly", "_readback", "moeGroupStats"];
// The methods that run layers or hand the sequence state out: after a disk read failed part way through a step
// (_failed, see _diskWait) they refuse until reset() (or importState / loadSlot: the whole sequence state replaced)
const FAILED_REFUSE = ["forwardToken", "forwardTokenIds", "prefillTokens", "prefillToken", "prefillHidden", "embedRun", "runHidden",
  "embedRunBatch", "runHiddenBatch", "verifyN", "specStep", "specStepDrafts", "mtpRun", "headAhead", "ffnOnly", "exportState", "exportSlot", "saveSlot"];

// Prefill GEMM operand precision (docs/research/prefill-f16-subgroup.md):
//   "f32"      default: the row-stationary f32 GEMM (engine/wgsl/gemm.js), the pinned numerics
//   "f16"      same kernels, weights and activations rounded to f16 on the way in (ALU, any device)
//   "sgmatrix" tensor cores via Chrome's chromium-experimental-subgroup-matrix (engine/wgsl/gemm_sgm.js),
//              f16 operands, f32 accumulation; feature-probed and self-tested, else the f32 GEMM
// Only full-width prefill passes use a GEMM, so decode and verify never change. Both opt-in modes
// change prefill logits (off by default). Deno tests: PREFILL_MATH=f16|sgmatrix (needs --allow-env).
export const PREFILL_MATH = ["f32", "f16", "sgmatrix"];
function envPrefillMath() {
  try {
    const D = globalThis.Deno;
    if (!D?.env || D.permissions?.querySync?.({ name: "env", variable: "PREFILL_MATH" })?.state !== "granted") return undefined;
    return D.env.get("PREFILL_MATH") || undefined;
  } catch { return undefined; }
}
// Features to request on the device for a prefillMath mode (only those the adapter has).
export function prefillMathFeatures(adapter, mode = envPrefillMath()) {
  if (mode !== "sgmatrix" || !adapter?.features) return [];
  return [...SGM_FEATURES, ...SGM_FEATURES_OPT].filter((f) => adapter.features.has(f));
}

// MoE prefill ubatch (tokens) for the default expert-grouped + wide prefill (moeGroupPrefill, prefillUbatch). 512 since
// the dp4a expert kernels (each expert's rows are read once per chunk of its pairs, so a wider ubatch reads them for
// more tokens): GB10 Deno 2048 / 8192 tokens ~800 / 730 tok/s at 256, ~780-860 / 790 at 512, ~835 / 815 at 1024
// (docs/bench-log.md 2026-10-02 "MoE prefill"). Bits do not depend on it (every kernel's per-column arithmetic is the
// same at any chunk width).
export const MOE_PREFILL_UBATCH = 512;
// dense (27B) default wide prefill ubatch: +24-29% prefill on the GB10 with the f32 wide GEMM (72-76 -> 91-99 tok/s),
// logits relDiff vs the 16-column path 4e-5..1.4e-4 (under the dense prefill tolerance 2e-3)
export const DENSE_PREFILL_UBATCH = 256;
// wide prefill: layers per command buffer. A wide chunk encodes a compute pass and a copy per NC-column
// sub-batch per layer, so one command buffer for the whole model grows with ubatch x layers. On Apple M5
// (Metal, Deno / wgpu) a whole-model buffer at ubatch 256 lost the device (27B and MoE); 192 columns, or 28
// of the MoE's 40 layers at 256, passed. Submitting every 8 layers fixes it; the results are bit-identical
// (the GEMM has no split-K, so chunking and submit boundaries do not change the arithmetic).
export const WIDE_SUBMIT_LAYERS = 8;
// prefillDp4a default (undefined / "auto"), per model kind, on the devices dp4aAutoDevice() accepts (see _init).
// Dense: on (~1.8x the f32 wide GEMM's prefill on the 27B, GB10: 187 vs 103 tok/s at 2048 tokens). It failed #302's
// gate (max |logprob - llama.cpp| over llama.cpp's top 20, dominated by tail tokens at -13..-17: 0.95 vs 0.26 nats at
// 2100 tokens) but passed the pre-registered probability-weighted evaluation that replaced it (tests/eval_dp4a.js,
// docs/bench-log.md 2026-10-02, PR #309): 27B and Qwen3.5-2B, 256 teacher-forced tokens on 15 prompts up to 8K tokens,
// KL(llama.cpp || ours) over its top 20 within 2x of the f32 path's (27B 0.00046 vs 0.00045 nats), top-1 agreement
// with llama.cpp no lower (98.94% vs 98.88%), and 40 scored downstream items identical (33/33, 2B 27/27).
// MoE: on, together with the dp4a expert kernels (moeGroupDp4a, which follows DP4A_DEFAULT.moe): passed the same
// pre-registered evaluation on the 35B-A3B (tests/eval_dp4a.js MODEL=moe, docs/bench-log.md 2026-10-02 "MoE prefill"):
// KL(llama.cpp || ours) 0.00270 vs the f32 path's 0.00255 nats, top-1 98.10% vs 98.21%, downstream 36 / 36 both.
export const DP4A_DEFAULT = Object.freeze({ dense: true, moe: true });
// Devices where "auto" turns dp4a on: measured faster there (docs/bench-log.md, 2026-09-30). NVIDIA by the adapter's
// vendor (Chrome); Deno reports no vendor, so there: any OS but macOS (the GB10 is where it was measured). Never Apple:
// Metal has no native int8 dot product. Other vendors stay opt-in until measured.
// the dp4a prefill's kernels: optional at create (one that does not compile turns prefillDp4a off)
const DP4A_PIPES = ["gemm_d_q4", "gemm_d_q4_acc", "gemm_d_q8", "gemm_d_q8_acc", "quant_q8_w"];
export function dp4aAutoDevice(info) {
  const v = (info?.vendor || "").toLowerCase();
  if (v) return v.includes("nvidia");
  return typeof Deno !== "undefined" && Deno.build?.os !== "darwin";
}

export class Qwen35Engine {
  // Option defaults applied under every create() call's own options (test runners set these from the
  // environment for A/B runs, e.g. tests/load_model.js ATTN_PREFILL_TILE=1).
  static defaults = {};
  static async create(opts) {
    const e = new Qwen35Engine();
    await e._init({ ...Qwen35Engine.defaults, ...opts });
    // a decode-ahead step (forwardTokenIds) still queued is dropped before anything else touches the state
    for (const m of AHEAD_SETTLE) if (typeof e[m] === "function") { const f = e[m]; e[m] = function (...a) { this._settleAhead(); return f.apply(this, a); }; }
    for (const m of FAILED_REFUSE) if (typeof e[m] === "function") {
      const f = e[m], async = Qwen35Engine.prototype[m]?.constructor?.name === "AsyncFunction";
      e[m] = function (...a) {
        if (!this._failed) return f.apply(this, a);
        const err = new Error(`qwen35: an earlier step failed part way (${this._failed.message}); reset() first`);
        if (async) return Promise.reject(err);
        throw err;
      };
    }
    return e;
  }

  // ---- session state (prefix cache, session switching, SSD cache) ----
  // Everything this device holds about the tokens written so far: per layer the KV rows [0, pos)
  // (full attention) or the recurrent state and conv window (DeltaNet), the draft block's KV, and
  // this.x, the trunk hidden the next draft starts from. Restoring it and feeding the next token
  // is exactly the same as never having left (tests/e2e/state_synth.mjs). A device only holds its
  // own layers, so in a room every device saves and restores its own part under the same key.
  _stateParts(pos = this.pos) {
    const kvRow = this.dims.kvDim * (this.kvQ8 ? 1 : this.flash ? 2 : 4), scRow = this.dims.kvDim / 32 * 4;
    const parts = [];
    for (const L of this.mtpLayer ? [...this.layers, this.mtpLayer] : this.layers) {
      if (L.isFull) {
        parts.push({ buf: L.kCache, bytes: pos * kvRow }, { buf: L.vCache, bytes: pos * kvRow });
        if (this.kvQ8) parts.push({ buf: L.kScale, bytes: pos * scRow }, { buf: L.vScale, bytes: pos * scRow });
      }
      else parts.push({ buf: L.S, bytes: L.S.size }, { buf: L.convState, bytes: L.convState.size });
    }
    parts.push({ buf: this.x, bytes: this.dims.dim * 4 });
    return parts;
  }
  // What a saved state must match to be loaded here.
  stateSignature() {
    // pm only when prefill ran with f16 operands, so f32-made states keep their old signature
    return { v: 1, lo: this.lo, hi: this.hi, mtp: !!this.mtpLayer, flash: !!this.flash, kvQ8: !!this.kvQ8, dims: [this.dims.dim, this.dims.kvDim, this.dims.nVH, this.dims.convDim],
      // wide prefill sums projections in another order: its states are not interchangeable with the default's
      ...(this.ubatch && this.prefillWide !== false ? { ub: this.ubatch } : {}),
      ...(this.ubatch && this.prefillWide !== false && this.dp4aCfg && this.prefillDp4a !== false ? { dp4a: 1 } : {}),
      ...(this.prefillMath && this.prefillMath !== "f32" && this._pmAvail?.[this.prefillMath] ? { pm: this.prefillMath } : {}) };
  }
  // Read the state back to the CPU: { sig, pos, parts: [ArrayBuffer] }. One part at a time through
  // one staging buffer, so a long context never needs one giant mapping.
  async exportState() { return this._readParts(this._stateParts(), this.pos); }
  // A GPU slot read back the same way (for spilling a session to disk without switching to it).
  // A slot dropped while it is read back keeps its buffers until the read ends (dropSlot), so the
  // copy never reads a destroyed buffer (what it read is still that slot's state; `dropped` says it went).
  async exportSlot(name) {
    const sl = this.slots?.get(name);
    if (!sl) throw new Error("no saved slot " + name);
    sl.reading = (sl.reading || 0) + 1;
    try { return { ...(await this._readParts(this._stateParts(sl.pos).map((p, i) => ({ buf: sl.bufs[i], bytes: p.bytes })), sl.pos)), dropped: !!sl.dropped }; }
    finally { if (!--sl.reading && sl.dropped) for (const b of sl.bufs) b.destroy(); }
  }
  async _readParts(parts, pos) {
    const most = Math.max(4, ...parts.map((p) => p.bytes));
    const stage = this.device.createBuffer({ size: most, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST });
    const out = [];
    try {
      for (const p of parts) {
        if (!p.bytes) { out.push(new ArrayBuffer(0)); continue; }
        const enc = this.device.createCommandEncoder();
        enc.copyBufferToBuffer(p.buf, 0, stage, 0, p.bytes);
        this.device.queue.submit([enc.finish()]);
        await stage.mapAsync(GPUMapMode.READ, 0, p.bytes);
        out.push(stage.getMappedRange(0, p.bytes).slice(0));
        stage.unmap();
      }
    } finally { stage.destroy(); }
    return { sig: this.stateSignature(), pos, parts: out };
  }
  // Load a state from exportState (same model, same layers, same KV format).
  importState(st) {
    const sig = JSON.stringify(this.stateSignature());
    if (JSON.stringify(st.sig) !== sig) throw new Error("saved state is for a different model, layer range or KV format");
    const parts = this._stateParts(st.pos);
    if (parts.length !== st.parts.length || parts.some((p, i) => p.bytes !== st.parts[i].byteLength)) throw new Error("saved state has the wrong shape");
    parts.forEach((p, i) => { if (p.bytes) this.device.queue.writeBuffer(p.buf, 0, st.parts[i]); });
    this.pos = st.pos;
    this._pre = null;
    this._failed = null;   // the whole sequence state is replaced: a step that failed part way left nothing behind
  }
  // GPU-side checkpoints: copies on the GPU (no readback), for switching between sessions or
  // rewinding an agent to an earlier turn. Costs GPU memory: the DeltaNet states (~3 MB per
  // layer on the 27B) plus the KV rows so far.
  saveSlot(name) {
    this.dropSlot(name);
    const parts = this._stateParts();
    const enc = this.device.createCommandEncoder();
    const bufs = parts.map((p) => {
      const b = this.device.createBuffer({ size: Math.max(4, p.bytes), usage: GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST });
      if (p.bytes) enc.copyBufferToBuffer(p.buf, 0, b, 0, p.bytes);
      return b;
    });
    this.device.queue.submit([enc.finish()]);
    (this.slots = this.slots || new Map()).set(name, { pos: this.pos, bufs });
  }
  loadSlot(name) {
    const sl = this.slots?.get(name);
    if (!sl) throw new Error("no saved slot " + name);
    const parts = this._stateParts(sl.pos);
    const enc = this.device.createCommandEncoder();
    parts.forEach((p, i) => { if (p.bytes) enc.copyBufferToBuffer(sl.bufs[i], 0, p.buf, 0, p.bytes); });
    this.device.queue.submit([enc.finish()]);
    this.pos = sl.pos;
    this._pre = null;
    this._failed = null;   // (as importState)
  }
  dropSlot(name) {
    const sl = this.slots?.get(name);
    if (!sl) return;
    this.slots.delete(name);
    if (sl.reading) sl.dropped = true;   // exportSlot destroys them when its read ends
    else for (const b of sl.bufs) b.destroy();
  }
  dropAllSlots() { for (const k of [...(this.slots?.keys() || [])]) this.dropSlot(k); }

  // moeTrace (engine option, default off; tests/trace_moe.js): every MoE layer routes one-token passes into
  // its own selection buffers instead of the shared ones, so after a decode step readMoeTrace() returns which
  // experts each layer chose and their weights. Same kernels and arithmetic: only the buffers differ.
  _moeTraceBufs(R) {
    if (!this.moeTraceOn) return this.moeB;
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST;
    R.trSel = this.device.createBuffer({ size: 32 * 4, usage: S }); R.trSelw = this.device.createBuffer({ size: 32 * 4, usage: S });
    return { ...this.moeB, sel: R.trSel, selw: R.trSelw };
  }
  // -> [{ sel: Uint32Array(K), w: Float32Array(K) }] per MoE layer of this engine (trunk order), for the last one-token pass
  async readMoeTrace() {
    const ls = this.layers.filter((R) => R.trSel);
    if (!ls.length) throw new Error("readMoeTrace: create the engine with moeTrace: true");
    const dev = this.device, K = this.moe.K;
    this._trStage ||= dev.createBuffer({ size: ls.length * 256, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST });
    const enc = dev.createCommandEncoder();
    ls.forEach((R, i) => { enc.copyBufferToBuffer(R.trSel, 0, this._trStage, i * 256, 128); enc.copyBufferToBuffer(R.trSelw, 0, this._trStage, i * 256 + 128, 128); });
    dev.queue.submit([enc.finish()]);
    await this._trStage.mapAsync(GPUMapMode.READ);
    const ab = this._trStage.getMappedRange().slice(0);
    this._trStage.unmap();
    return ls.map((_, i) => ({ sel: new Uint32Array(ab, i * 256, K).slice(), w: new Float32Array(ab, i * 256 + 128, K).slice() }));
  }

  // Fresh context: forget the conversation so far. Recurrent (DeltaNet) states and
  // conv windows are zeroed; the KV caches are simply overwritten from position 0.
  reset() {
    const enc = this.device.createCommandEncoder();
    for (const R of this.layers) {
      if (R.convState) enc.clearBuffer(R.convState);
      if (R.S) enc.clearBuffer(R.S);
    }
    this.device.queue.submit([enc.finish()]);
    this.pos = 0;
    this._pre = null;
    this._failed = null;   // (a disk read that failed part way: _diskWait)
  }

  // opts: { device, meta (gguf meta), weights, layerRange, hasEmbed, hasHead, maxSeq }
  async _init({ device, meta, weights, layerRange, hasEmbed = true, hasHead = true, maxSeq = 512, vocab: vocabOpt, matvecVariant = "coop", coopWG = 256, coopRows = 4, batchCols = 4, coopRowsB = coopRows, gemm = true, draftVocab = 0, replayRollback = true, gemm8 = true, softmaxWG = true, draftChain = true, specFuse = true, attnGlue = true, dnFuse = true, attnMC = true, attnFlash = true, kvQ8 = false, attnTile = true, attnPrefillTile, attnPrefillSplits = 32, attnPrefillTK = 0, attnTileKvh = true, attnTilePf = true, attnTileNr = true, wideDn = true, wideRouter = true, mtpWide = true, attnDecode = "v2", attnDecodeSplits = 0, fuseProj = true, moeFuse = true, moeNormRouter = true, dnNormBA = true, moeDnRows = 1, moeFusedLayout: moeFusedLayoutOpt, moeKernel, draftVocabAuto = true, moeGroupPrefill, moeGroupUC = 8, moeGroupTiled = true, moeGroupDp4a, prefillUbatch, prefillTile, prefillDp4a, prefillDp4aTile, prefillMath, prefillSgm, adapterInfo, headRows = 0, gpuSample = true, argmaxWide = true, hostFuse = true, layerFuse, moeTrace = false }) {
    // GPU sampling (see headFromHiddenIds): argmax / top-k on the GPU, k (idx, value) pairs back
    // instead of the logits, when the sampler carries .gpu (room/sampling.js pickSampler). On by default
    // (GPU suites, the MoE/27B checks and split == solo rooms pass with it on); false: logits path.
    this.gpuSample = !!gpuSample;
    // argmaxWide: the draft argmax as the two-stage multi-workgroup kernel (topk_a/b, k = 1) instead
    // of the single-workgroup one. Same tie rule, same result; false keeps the old kernel (A/B).
    this.argmaxWide = !!argmaxWide;
    this.replay = replayRollback !== false;
    // longest draft run one verify can take: with replay rollback the limit is the replay buffers
    // (max(batchCols, 8) columns), so prompt-lookup drafts can run to 15 tokens when code is being copied
    this.maxDrafts = this.replay ? Math.min(15, Math.max(batchCols, 8) - 1) : 7;
    this.device = device;
    this.mvVariant = matvecVariant;
    this.coopWG = coopWG; this.coopRows = coopRows;
    this.NC = batchCols; this.coopRowsB = coopRowsB;   // batched (prefill/verify) column count, rows per WG
    const M = meta;
    const dim = M["qwen35.embedding_length"];
    const nH = M["qwen35.attention.head_count"];
    const nKV = M["qwen35.attention.head_count_kv"];
    const hd = M["qwen35.attention.key_length"];
    const nRot = M["qwen35.rope.dimension_count"];
    const ropeTheta = M["qwen35.rope.freq_base"];
    const eps = M["qwen35.attention.layer_norm_rms_epsilon"];
    // Mixture of experts (Qwen3.5 / 3.6 MoE): K of nExp routed experts per token plus a shared
    // expert. The dense FFN buffers (g, u, cfg.inter) serve the shared expert.
    const nExp = M["qwen35.expert_count"] || 0;
    this.moe = nExp > 0 ? { nExp, K: M["qwen35.expert_used_count"], inter: M["qwen35.expert_feed_forward_length"],
      shInter: M["qwen35.expert_shared_feed_forward_length"] || 0, norm: M["qwen35.expert_weights_norm"] === false ? 0 : 1 } : null;
    if (this.moe && (nExp > 1024 || !(this.moe.K > 0) || this.moe.K > 16)) throw new Error(`unsupported MoE shape: ${nExp} experts, top-${this.moe.K}`);
    this.moeTraceOn = !!this.moe && moeTrace === true;
    const inter = this.moe ? (this.moe.shInter || this.moe.inter) : M["qwen35.feed_forward_length"];
    // Fused MoE FFN (engine/wgsl/moe.js moeFusedWGSL): router GEMV with the shared-expert gate as row
    // nExp, moe_route, gate/up over K + 1 slots (the shared expert is slot K), down + combine + residual
    // in one launch: 5 dispatches per MoE layer instead of 9. Needs a shared expert, an F32 router and
    // shared gate, Q4_0 / Q8_0 shared weights we can repack, on every MoE layer; otherwise (or with
    // moeFuse: false) the unfused kernels run. moeDnRows: output rows per moe_dnc workgroup (1, 2, 4).
    this.moeFuse = false;
    if (this.moe && moeFuse !== false) {
      const { nExp: nE, inter: ei } = this.moe;
      const q = (e) => !!e && (e.kind === "q4" || e.kind === "q8");
      const COPY = GPUBufferUsage.COPY_SRC;
      // packGU needs 4-byte aligned parts (CPU copies: an odd Q4/Q8 block count falls back to unfused)
      const packable = (e) => q(e) && (e.gpu ? !!(e.gpu.qs?.usage & COPY) && !!(e.gpu.sc?.usage & COPY)
        : !!(e.qs && e.scales) && e.qs.byteLength % 4 === 0 && e.scales.byteLength % 4 === 0);
      const f32 = (e, n) => !!e && e.kind === "f32" && !!e.data && e.data.length === n;
      const moeLs = [...weights.layers, ...(weights.mtp && hasHead ? [weights.mtp.layer] : [])].filter((L) => L.moe);
      const ok = (L) => f32(L.router, nE * M["qwen35.embedding_length"]) && f32(L.shRouter, M["qwen35.embedding_length"])
        && q(L.expGate) && q(L.expDown) && L.shGate && L.shUp && L.shGate.kind === L.shUp.kind && packable(L.shGate) && packable(L.shUp) && q(L.shDown);
      // moe_dnc keeps (K + 1) * R * 64 f32 partial sums in workgroup memory
      const wgMemOK = (this.moe.K + 1) * moeDnRows * 64 * 4 <= device.limits.maxComputeWorkgroupStorageSize;
      if (moeLs.length && moeLs.every(ok) && inter % 32 === 0 && [1, 2, 4].includes(moeDnRows) && wgMemOK) {
        this.moeFuse = true;
        this.moe.KS = this.moe.K + 1; this.moe.sDim = inter; this.moe.hs = Math.max(ei, inter); this.moe.R = moeDnRows;
        this.moe.guPairs = [...new Set(moeLs.map((L) => L.expGate.kind + "_" + L.shGate.kind))];
        this.moe.dnPairs = [...new Set(moeLs.map((L) => L.expDown.kind + "_" + L.shDown.kind))];
        // fused kernel layout (engine/wgsl/moe.js moeFusedLayout): "legacy" | "wide" | { gu: {...}, dn: {...} }. Default
        // (undefined / "auto"): the wide layout on Apple GPUs, where the legacy one is barrier bound (M5 Max, Chrome:
        // docs/bench-log.md), the legacy kernels everywhere else (their bits unchanged). Either keeps batched == one-token.
        // (Deno: device.adapterInfo panics in wgpu-core once the adapter is dropped, and its adapter info is empty anyway)
        const info = adapterInfo ?? (typeof Deno === "undefined" ? device.adapterInfo : null), autoL = moeFusedLayoutOpt === undefined || moeFusedLayoutOpt === "auto";
        const lay = moeFusedLayout(autoL ? (/apple/i.test(info?.vendor || "") ? "wide" : "legacy") : moeFusedLayoutOpt, this.moe.K);
        const wideMem = !lay || Math.max(2 * lay.gu.R * lay.gu.WG, this.moe.KS * lay.dn.R * lay.dn.WG) * 4 <= device.limits.maxComputeWorkgroupStorageSize;
        if (!wideMem) console.warn("moeFusedLayout: reduction scratch over the device's workgroup memory; legacy fused kernels");
        this.moe.layout = wideMem ? lay : null;
        // output rows per moe_gus / moe_dnc workgroup (grid x = ceil(rows / these))
        this.moe.gusRows = this.moe.layout ? this.moe.layout.gu.rows : 4; this.moe.dncRows = this.moe.layout ? this.moe.layout.dn.rows : moeDnRows;
        // moeNormRouter (default on): the post-attention RMSNorm and the router GEMV in one launch (moe_nrt, engine/wgsl/moe.js
        // normRouterKernel), the router in BF16 when every layer's router is exactly BF16 (bartowski's files: yes). false:
        // rmsnorm + the f32 router GEMV, as before.
        const bfExact = (d) => { const u = new Uint32Array(d.buffer, d.byteOffset, d.length); for (let i = 0; i < u.length; i++) if (u[i] & 0xffff) return false; return true; };
        this.moe.nrt = moeNormRouter !== false && dim % 4 === 0 ? { ROWS: 4, bf16: moeLs.every((L) => bfExact(L.router.data) && bfExact(L.shRouter.data)) } : null;
      } else console.warn(`MoE: fused FFN off (unfused kernels): ${!moeLs.length ? "no MoE layers" : ![1, 2, 4].includes(moeDnRows) ? `moeDnRows ${moeDnRows} not 1, 2 or 4`
        : !wgMemOK ? "moe_dnc workgroup memory over the device limit" : "a layer's router / shared-expert tensors are not fusable"}`);
    }
    // expert GEMV layout (engine/wgsl/moe.js): moeKernel = undefined (MOE_LEGACY) | "default" (the GB10-tuned MOE_DEFAULT) | "legacy" | { gu: {...}, dn: {...} }.
    // Every (column, slot) runs the same code in every pass width, so any setting keeps batched == one-token.
    this.moeK = this.moe ? moeKernelConfig(moeKernel, { dim, inter: this.moe.inter }) : null;
    // (only the unfused moe_gu / moe_dn kernels use it: with moeFuse the fused moe_gus / moe_dnc run instead)
    const dState = M["qwen35.ssm.state_size"];
    const nKH = M["qwen35.ssm.group_count"];
    const nVH = M["qwen35.ssm.time_step_rank"];
    const dInner = M["qwen35.ssm.inner_size"];
    if (dState !== 128) throw new Error(`dn_delta_mc keeps a 128-row state column in registers; this model has dState ${dState}`);
    const keyDim = dState * nKH;
    const convDim = keyDim * 2 + dInner;
    // workers parse the header without the vocab; the embedding's row count says the same thing
    const vocab = M["tokenizer.ggml.tokens"]?.length ?? vocabOpt ?? M["qwen35.vocab_size"] ?? 248320;
    const qDim = nH * hd, kvDim = nKV * hd;
    // attention workgroups per kv head: the flash / attn_dec kernels take at most 8 query heads per workgroup,
    // so a kv head with more (Qwen3.5-122B: 16 per kv head) gets R workgroups of nH / nKV / R heads each;
    // nKVa = nKV * R is their grid size. R = 1 on every model with <= 8 (the 27B, the 35B MoE): unchanged.
    const attnR = Math.ceil(nH / nKV / 8), nKVa = nKV * attnR;
    this.dims = { dim, nH, nKV, hd, nRot, inter, dState, nKH, nVH, dInner, keyDim, convDim, vocab, qDim, kvDim, attnR, nKVa };
    this.maxSeq = maxSeq;
    // workgroup softmax (bit-identical, see engine/wgsl/base.js); its staging array holds 2048 positions
    this.softmaxWG = softmaxWG !== false && maxSeq <= 2048;
    // long context: f16 KV cache + split-K flash attention (engine/wgsl/qwen35.js attn_flash), used
    // by every pass so decode, verify and prefill agree bit for bit. attnFlash: false keeps the f32
    // cache and the scores / softmax / out kernels (A/B, and the pre-flash numerics).
    this.flash = attnFlash !== false && hd <= 256 && nH % nKV === 0 && (nH / nKV) % attnR === 0 && nH / nKVa * hd <= 2048;
    this.faSplit = Math.max(256, Math.ceil(maxSeq / 128 / 64) * 64);   // <= 128 splits per head
    this.faSplits = Math.ceil(maxSeq / this.faSplit);
    // kvQ8: int8 K/V with one scale per 32 values (~56% of f16's memory: 36 KB per token for the
    // whole 27B), for 32K+ contexts. Off by default: it changes the numerics (tests/e2e/flash_synth.mjs --q8).
    this.kvQ8 = this.flash && kvQ8 === true && hd % 32 === 0;
    // Each attention layer keeps K and V in one buffer each, bound whole, so maxSeq x kvDim x (1 | 2 | 4
    // bytes) must fit the device's binding limit. Every WebGPU device binds 128 MiB: the MoE's 131072
    // positions in f16 (kvDim 512) exactly, the 27B's 65536 in f16 or 131072 in int8 (kvDim 1024).
    // Beyond that it needs a device created with the adapter's higher limit (room.js, tests/load_model.js);
    // the room clamps its context to what every device can bind (room/models.js ctxForBinding).
    // Failing here beats a lost device or invalid bind groups at the first attention layer.
    this.kvBufBytes = maxSeq * kvDim * (this.kvQ8 ? 1 : this.flash ? 2 : 4);
    if ([...(weights.layers || []), weights.mtp?.layer].some((L) => L?.isFull)) {
      const lim = Math.min(device.limits.maxStorageBufferBindingSize, device.limits.maxBufferSize);
      if (this.kvBufBytes > lim)
        throw new Error(`context ${maxSeq} needs a ${(this.kvBufBytes / 2 ** 20).toFixed(0)} MiB KV buffer per attention layer; this GPU device binds at most ${(lim / 2 ** 20).toFixed(0)} MiB (ask for a shorter context, or int8 KV)`);
    }
    this.ksPipe = this.kvQ8 ? "kv_store_q8" : "kv_store";
    this.faPipe = this.kvQ8 ? "attn_flash_q8" : "attn_flash";
    // two columns per workgroup in batched passes (attn_flash_t2): K/V read once per pair, same bits
    this.attnTileOn = this.flash && !this.kvQ8 && attnTile !== false && 2 * (nH / nKV) * hd <= 3072 && nH / nKV <= 8;
    this.attnTile = this.attnTileOn;
    // attnPrefillTile: tiled causal flash attention for full-width prefill passes (engine/wgsl/attn_tile.js):
    // one workgroup per (split, kv head, group of up to 64 query rows) instead of per column (pair).
    // It changes the prefill summation order (tolerance, not bits) and only runs on full-width passes
    // that are not a speculative verify (_snapNow null: prefillTokens, runHiddenBatch/embedRunBatch
    // without snapshots); decode and verify keep attn_flash at every batchCols, so spec == plain.
    // Default ("auto" / undefined): on for every model. Dense: 27B relDiff vs attn_flash 9.6e-5 at 16k, 2.2x
    // prefill at 16k. MoE (on since 2026-09-27, docs/bench-log.md): it lands 4e-3..1.6e-2 from attn_flash
    // because expert-routing near-ties amplify any summation-order change, the same size as the MoE's own
    // batched-prefill vs token-by-token gap (2e-3..2.3e-2 at 700+ tokens in Deno); argmax, greedy text, spec == plain
    // and split rooms are unchanged (tests/test_moe_split.js, test_prefill_opts.js). false forces attn_flash.
    // The kernel is compiled whenever it is not explicitly false, so engine.attnPrefillTile can be flipped
    // at runtime for A/B runs.
    const aptOn = attnPrefillTile !== false;
    this.attnPTCfg = this.flash && !this.kvQ8 && attnPrefillTile !== false
      ? attnTileConfig({ hd, G: nH / nKV, faSplit: this.faSplit, faSplits: this.faSplits,
        wgMem: device.limits.maxComputeWorkgroupStorageSize, target: attnPrefillSplits, tk: attnPrefillTK,
        // the K / V tile as f16 pairs (kvh), the next tile prefetched into registers (pf), named registers (nr): the same
        // arithmetic, so the same bits; GB10 one 16-column pass at 8K / 16K: 1630 / 2769 -> 1505 / 2468 µs
        // (tests/bench/attn_tile_bench.js). attnTileKvh / attnTilePf / attnTileNr: false for A/B.
        kvh: attnTileKvh, pf: attnTilePf, nr: attnTileNr }) : null;
    // attnDecode: "v2" (default) = split-K decode attention with coalesced loads and per-position split
    // lengths (engine/wgsl/attn_dec.js) for decode and verify passes; faster than attn_flash at every context
    // length measured (docs/research/long-context-2026-09.md). Other numerics than attn_flash (same greedy
    // tokens, llama.cpp goldens still match); spec == plain holds because a column's splits depend only on
    // its position. "v1" = attn_flash + attn_combine (A/B, ?attndecode=v1). "compile" builds the v2 kernels
    // but starts on v1 (engine.attnDecode = "v2" at runtime). kvQ8, head sizes other than 256 or more than
    // 8 query heads per kv head stay on v1.
    this.adCfg = this.flash && (attnDecode === "v2" || attnDecode === "compile")
      ? attnDecConfig({ hd, G: nH / nKVa, nKV: nKVa, R: attnR, splits: attnDecodeSplits, kvQ8: this.kvQ8 }) : null;
    this.attnDecode = "v1";
    this._adWant = attnDecode;
    // fused attention glue (qsplit + q/k head_norm + rope in one dispatch, bit-identical); its
    // staging array holds one 256-wide head. engine.attnGlue = false restores the five dispatches.
    this.attnGlueOn = attnGlue !== false && hd <= 256;
    this.attnGlue = this.attnGlueOn;
    // dn_delta + dn_gatenorm in one dispatch for decode (bit-identical); engine.dnFuse = false for A/B
    this.dnFuse = dnFuse !== false;
    // Decode layer fusion (engine/wgsl/layer_fuse.js, bit-identical, one-token passes only):
    //   dn:   dn_pre folded into dn_delta_gn (needs dnFuse)    -1 dispatch per DeltaNet layer
    //   conv: dn_conv in the [qkv | z] GEMV's epilogue         -1 dispatch per DeltaNet layer
    //   comb: sigmoid_mul in attn_combine, or in attn_dec_combine under attnDecode v2 (attn_dec_combine_g,
    //         engine/wgsl/attn_dec.js, optional: its compile failure only turns comb off for v2); the gate is
    //         read from q_full; f16 flash KV                                            -1 per attention layer
    //   kv:   kv_store in attn_glue (needs comb: the glue no longer copies the gate)  -1 per attention layer
    //   norm: an attention layer's input rmsnorm folded into its [k | v] GEMV, run before q (coop.js NRM)
    //         -1 per attention layer. (The MoE router / DeltaNet beta-alpha norm folds are perf/decode-moe-bandwidth's.)
    //   pass: forwardToken's layers and head in one compute pass (91 -> 1 passes per MoE token)
    // layerFuse: "auto" / undefined (default) | true (all) | false | "dn,conv,kv,comb,norm,pass" | { dn: false, ... };
    // engine.layerFuse.<k> at runtime for A/B. Every flag keeps today's bits (per-flag logit hashes, spec == plain,
    // test_q38_bits, test_moe on the GB10 and the M5 Max). Default (auto): on where it was measured faster and
    // bit-checked: NVIDIA (GB10 Chrome MoE +3.6%, 27B +1.3%) and Apple (M5 Max Chrome +0.9%), and under Deno, whose
    // adapter info is empty (the GB10 and the M5 Max are the Deno hosts; GB10 Deno MoE 31.96 -> 31.0 ms/token).
    // Other vendors keep the unfused kernels until measured: a driver that contracts the fused kernels' arithmetic
    // differently from the batched ones would break spec == plain there. docs/bench-log.md 2026-09-29 / 09-30.
    // A flag's kernels are compiled only when it is on at create (LF_PIPES below), and a fused kernel this GPU's
    // compiler rejects turns layerFuse off (the unfused path runs) instead of failing the load: CLI 0.3.4 failed
    // every Windows (FXC) join of the MoE / 27B on matvec_coop_n. A flag that was off at create stays a no-op if
    // switched on at runtime (its kernels were never built): create with layerFuse: true to A/B them.
    {
      let lf = layerFuse;
      if (lf === undefined || lf === "auto") {
        const info = adapterInfo ?? (typeof Deno === "undefined" ? device.adapterInfo : null), v = `${info?.vendor || ""} ${info?.architecture || ""}`;
        lf = !info || !v.trim() || /nvidia|apple/i.test(v);
      }
      const want = (k) => lf === true ? true : lf === false ? false
        : typeof lf === "string" ? lf.split(",").map((x) => x.trim()).includes(k) : lf[k] !== false;
      this.layerFuse = { dn: want("dn"), conv: want("conv") && matvecVariant === "coop", kv: want("kv"), comb: want("comb"), norm: want("norm") && matvecVariant === "coop", pass: want("pass") };
    }
    // Merged projection GEMVs (docs/research/kernels-next-2026-09.md D5): at load, the DeltaNet
    // [qkv | z] and [beta | alpha] weights and the attention [k | v] weights are row-concatenated
    // into one matrix each (and the MoE router with the shared-expert gate), so one GEMV launch
    // replaces two. Every row keeps its kernel, its reduction order and its full/tail branch, so
    // the output is bit-identical (see _fuseW). The per-tensor ops still exist over the same
    // memory: engine.fuseProj = false switches back at runtime for A/B; fuseProj: false at create
    // keeps the old separate buffers entirely. Only for the coop GEMV ladder.
    this.fuseProjOn = fuseProj !== false && matvecVariant === "coop";
    this.fuseProj = this.fuseProjOn;
    // dnNormBA (MoE models, default on; the 27B keeps its bits): a DeltaNet layer's input RMSNorm and its merged F32
    // beta / alpha GEMV in one launch (dn_nba: engine/wgsl/moe.js normRouterKernel, as moe_nrt), which also writes xn
    // for the [qkv | z] GEMV that follows. Decode and the batched passes both run it (with fuseProj on), so spec == plain.
    this.dnNba = dnNormBA !== false && !!this.moe && this.fuseProjOn;
    // batched attention for verify / prefill passes (one dispatch per stage for all columns instead
    // of three per column; bit-identical, needs the workgroup softmax). engine.attnMC = false for A/B.
    this.attnMCOn = attnMC !== false && this.softmaxWG;
    this.attnMC = this.attnMCOn;
    const [lo, hi] = layerRange;
    this.lo = lo; this.hi = hi;
    this.hasEmbed = hasEmbed; this.hasHead = hasHead;
    this.pos = 0;

    // ---- prefill GEMM plan (docs/research/prefill-gemm-v2.md) ----
    // Only at the full batch width, only for Q4 weights, only for shapes with a
    // pinned split-K factor. Narrower passes (decode, speculative verify, the
    // prompt tail) stay on the GEMV ladder, so the speculative stream is
    // structurally identical to plain decoding.
    this._gemmShapes = new Map();
    if (batchCols >= 16 && gemm !== false) {
      for (const [dOut, dIn] of [[convDim, dim], [dInner, dim], [dim, dInner], [inter, dim], [dim, inter],
                                 [nH * hd * 2, dim], [kvDim, dim], [dim, qDim]]) {
        const S = GEMM_S[`${dOut}x${dIn}`];
        if (S && dOut >= 256 && dIn % 64 === 0 && ((dIn / 32) / 2) % S === 0) this._gemmShapes.set(`${dOut}x${dIn}`, S);
      }
    }
    // Q8_0 tensors with a pinned shape get the Q8 GEMM (engine/wgsl/gemm.js kernel8): in the real
    // 27B these are ffn_down, ssm_out and attn_output, ~40% of a DeltaNet layer's bytes
    this._gemm8Pairs = [];
    if (this._gemmShapes.size && gemm8 !== false) {
      const q8keys = new Set();
      const note = (e, dOut, dIn) => { if (e && e.kind === "q8") q8keys.add(`${dOut}x${dIn}`); };
      for (const L of [...weights.layers, ...(weights.mtp ? [weights.mtp.layer] : [])]) {
        note(L.ffnDown, dim, inter); note(L.wOut, dim, dInner); note(L.wo, dim, qDim);
      }
      for (const k of q8keys) {
        const S = this._gemmShapes.get(k), dIn = +k.split("x")[1];
        if (S && ((dIn / 32) / 2) % S === 0) this._gemm8Pairs.push([dIn, S]);
      }
    }
    this._gemm8Set = new Set(this._gemm8Pairs.map(([dIn, S]) => `${dIn}:${S}`));
    this.gemmOn = this._gemmShapes.size > 0;
    // A verify pass exactly NC columns wide would go through the prefill GEMM (_encodeLayerBatch
    // and _dop pick it by width), whose sums differ from the GEMV twins that plain decoding matches
    // (prefill tolerance, not bit-identical). A 15-draft lookup run at NC=16 was such a pass, so
    // cap the run one short of that: every verify stays on the GEMV ladder and spec == plain holds
    // by construction. Drafts only: no kernel changes, a copy run just verifies 14 at a time.
    // Only when the longest run is exactly one short of NC (NC = 8, 16): at NC = 4 the runs are
    // up to 7 and a cap of 2 would gut lookup, so NC = 4 keeps its old limit.
    if (this.gemmOn && this.maxDrafts === batchCols - 1) this.maxDrafts = batchCols - 2;
    this._gemmDIns = [...new Set([...this._gemmShapes.keys()].map((k) => +k.split("x")[1]))];
    this._gemmSplits = [...new Set(this._gemmShapes.values())];
    this._gemmPairs = [...new Set([...this._gemmShapes].map(([k, S]) => `${k.split("x")[1]}:${S}`))].map((x) => x.split(":").map(Number));
    this.gemm = this.gemmOn;   // runtime kill switch: engine.gemm = false reproduces the GEMV path
    // Expert-grouped MoE prefill (engine/wgsl/moe_group.js, _prefillGrouped; candidate D of
    // docs/research/prefill-profile-2026-09.md). moeGroupPrefill = U > 0: prompt prefill runs in
    // layer-major ubatches of U tokens (a multiple of batchCols, at least two passes), the last one cut to the
    // remaining whole passes (if at least two); under two passes' worth goes the ordinary way. Each layer's
    // attention / DeltaNet part runs as the usual batchCols-wide passes; the MoE FFN of all U tokens is
    // sorted by expert and each chosen expert's rows are streamed once per chunk of up to moeGroupUC
    // (column, slot) pairs instead of once per pair. moeGroupTiled (the default when moeGroupPrefill is set)
    // runs real per-expert tiles: experts 3x faster, MoE prefill +23..38% on the GB10, but its logits drift
    // from the per-pass path (1.7e-2 at 700 tokens, argmax/greedy equal; the MoE prefill tolerance is 2e-2,
    // see attnPrefillTile). Default (undefined): 256 on a MoE engine whose kernels allow it, silently off
    // otherwise; an explicit value warns when it cannot be used. moeGroupTiled: false keeps the per-pair arithmetic (bit-identical
    // to the per-pass path, but slower than it at every UC; kept only as a reference).
    // engine.moeGroup = false at runtime restores the per-pass path.
    // Decode, speculative verify and the prompt tail (under 2 * batchCols tokens) never use it.
    this.moeGrpU = 0; this.moeGrpUC = 0;
    {
      const auto = moeGroupPrefill === undefined || moeGroupPrefill === "auto";
      const gU = auto ? (this.moe ? MOE_PREFILL_UBATCH : 0) : Math.floor(+moeGroupPrefill) || 0, UC = Math.floor(+moeGroupUC) || 8, R = dnGroupRows(UC);
      const why = gU <= 0 ? null : !this.moeFuse ? "needs the fused MoE FFN (moeFuse)" : !this.flash ? "needs flash attention"
        : gU % batchCols || gU < 2 * batchCols ? `ubatch ${gU} is not a multiple of batchCols ${batchCols} (at least 2 passes)`
        : ![1, 2, 4, 8, 16].includes(UC) ? `moeGroupUC ${UC} is not 1, 2, 4, 8 or 16`
        : moeGroupTiled && UC > 8 ? `moeGroupUC ${UC}: the tiled kernels take at most 8`
        : !moeGroupTiled && this.moe.layout ? "moeGroupTiled: false mirrors the legacy fused kernels (moeFusedLayout is not legacy)"
        : !moeGroupTiled && UC * R * 64 * 4 > Math.min(16384, device.limits.maxComputeWorkgroupStorageSize) - 16 ? `moeGroupUC ${UC} x ${R} down rows exceed workgroup memory`
        : gU * (this.moe.K + 1) * dim * 4 > device.limits.maxStorageBufferBindingSize ? `ubatch ${gU} expert outputs exceed the storage-binding limit`
        : gU > device.limits.maxComputeWorkgroupsPerDimension
          || moeGroupSizes({ U: gU, K: this.moe.K, nExp: this.moe.nExp, UC }).maxChunks > device.limits.maxComputeWorkgroupsPerDimension
          ? `ubatch ${gU} exceeds the dispatch limit (an over-limit indirect dispatch would silently do nothing)` : "";
      if (why && !auto) console.warn(`moeGroupPrefill ${gU} off: ${why}`);
      else if (why === "") { this.moeGrpU = gU; this.moeGrpUC = UC; this.moeGrpTiled = !!moeGroupTiled; }
    }
    this.moeGroup = this.moeGrpU > 0;   // runtime kill switch
    // wide prefill structure switches (also runtime engine properties, for A/B; none of them changes a value of the
    // trunk): the DeltaNet middle once per chunk, the grouped chunk's router once per chunk, the draft-cache fill
    // with wide GEMMs (see _encodeLayerWide / _mtpFillWide)
    this.wideDn = wideDn !== false; this.wideRouter = wideRouter !== false; this.mtpWide = mtpWide !== false;

    // ---- wide prefill (docs/research/prefill-profile-2026-09.md candidate B) ----
    // prefillUbatch = U > 0: prefillTokens runs chunks of up to U prompt tokens (multiples of the tile
    // width BN) through _encodeLayerWide: every Q4_0 / Q8_0 projection is ONE tiled GEMM over the
    // whole chunk (engine/wgsl/gemm_wide.js); the DeltaNet recurrence, attention and MoE experts run
    // on the existing batchCols-wide kernels, sub-batch by sub-batch. Prefill-tolerance numerics (a
    // different summation order); decode and verify never use it. Default (undefined): 256 on a MoE engine
    // (MOE_PREFILL_UBATCH; the MoE prefill tolerance, see attnPrefillTile) and on a dense engine that holds the
    // embedding (DENSE_PREFILL_UBATCH); its 64x64 tile fits the 16 KB default workgroup memory. A MoE shard
    // without the embedding (a room's later devices) uses it through prefillHidden.
    // An automatic setting that cannot be used here is silently off; an explicit one warns.
    // engine.prefillWide = false switches it off at runtime. prefillTile: { BM, BN, TM, TN, KB } overrides the tile.
    this.ubatch = 0; this.wideCfg = null;
    const ubAuto = prefillUbatch === undefined || prefillUbatch === "auto";
    const UB = ubAuto ? (this.moe ? MOE_PREFILL_UBATCH : hasEmbed && lo === 0 ? DENSE_PREFILL_UBATCH : 0) : Math.floor(+prefillUbatch || 0);
    if (UB > 0) {
      let why = null, cfg = null;
      try { cfg = wideTileConfig(prefillTile, device.limits.maxComputeWorkgroupStorageSize); } catch (e) { why = e.message; }
      const qk = (e) => !!e && (e.kind === "q4" || e.kind === "q8");
      const projOK = (L) => L.isFull ? [L.wq, L.wk, L.wv, L.wo].every(qk) : [L.wqkv, L.wz, L.wOut].every(qk);
      const ffnOK = (L) => !!L.moe || [L.ffnGate, L.ffnUp, L.ffnDown].every(qk);
      if (why) { /* tile config error */ }
      else if (UB > device.limits.maxComputeWorkgroupsPerDimension) why = `it exceeds the dispatch limit ${device.limits.maxComputeWorkgroupsPerDimension}`
      else if (UB % cfg.BN || cfg.BN % batchCols) why = `it must be a multiple of the tile width ${cfg.BN}, which batchCols (${batchCols}) must divide`;
      else if (!this.flash) why = "it needs the flash-attention path";
      else if ([dim, dInner, qDim, ...(weights.layers.some((L) => !L.moe) ? [inter] : [])].some((d) => d % cfg.KS)) why = `a projection width is not a multiple of ${cfg.KS}`;
      else if (!weights.layers.every((L) => projOK(L) && ffnOK(L))) why = "a projection is not Q4_0 / Q8_0";
      if (why) { if (!ubAuto) console.warn(`prefillUbatch ${UB}: wide prefill off (${why})`); }
      else { this.ubatch = UB; this.wideCfg = cfg; }
    }
    this.prefillWide = this.ubatch > 0;

    // ---- dp4a wide prefill GEMM (engine/wgsl/gemm_wide.js gemmDp4aWGSL) ----
    // prefillDp4a: the wide chunk's projections as int8 dot products (dot4I8Packed) on activations
    // quantized per 32 values (Q8_1-style, llama.cpp's MMQ numerics) instead of the f32 tiled GEMM.
    // Prefill only (decode and verify never use it). Needs the wide path and the
    // packed_4x8_integer_dot_product WGSL feature; skipped on Apple GPUs, where Metal emulates dp4a with
    // scalar multiply-adds. engine.prefillDp4a = false switches it off at runtime (A/B).
    this.dp4aCfg = null;
    const dpAuto = prefillDp4a === undefined || prefillDp4a === "auto";
    const dpInfo = adapterInfo ?? (typeof Deno === "undefined" ? device.adapterInfo : null);
    if (this.ubatch && (dpAuto ? DP4A_DEFAULT[this.moe ? "moe" : "dense"] && dp4aAutoDevice(dpInfo) : !!prefillDp4a)) {
      let why = null, cfg = null;
      const info = dpInfo;
      try { cfg = dp4aTileConfig(prefillDp4aTile, device.limits.maxComputeWorkgroupStorageSize); } catch (e) { why = e.message; }
      if (why) { /* tile config error */ }
      else if (/apple/i.test(info?.vendor || "")) why = "Apple GPUs emulate dp4a";
      else if (!(await probeDp4a(device))) why = "no packed_4x8_integer_dot_product on this device";
      else if (this.wideCfg.BN % cfg.BN) why = `its tile width ${cfg.BN} must divide the wide tile width ${this.wideCfg.BN}`;
      else if ([dim, dInner, qDim, ...(weights.layers.some((L) => !L.moe) ? [inter] : [])].some((d) => d % cfg.KS)) why = `a projection width is not a multiple of ${cfg.KS}`;
      if (why) { if (!dpAuto) console.warn(`prefillDp4a: off (${why})`); }
      else this.dp4aCfg = cfg;
    }
    this.prefillDp4a = !!this.dp4aCfg;

    // moeGroupDp4a: the expert-grouped prefill's expert kernels as int8 dot products (engine/wgsl/moe_group.js
    // dp4aGroupKernel: the chunk's activations quantized per 32 values, llama.cpp's MMQ numerics) instead of the f32
    // tiled kernels. Needs the tiled grouped prefill (moeGroupTiled) and dot4I8Packed; default (undefined / "auto"):
    // DP4A_DEFAULT.moe on the devices dp4aAutoDevice() accepts. engine.moeGroupDp4a = false switches it off at runtime.
    this.moeDp4a = false;
    {
      const auto = moeGroupDp4a === undefined || moeGroupDp4a === "auto";
      if (this.moeGrpU && this.moeGrpTiled && (auto ? DP4A_DEFAULT.moe && dp4aAutoDevice(dpInfo) : !!moeGroupDp4a)) {
        const why = /apple/i.test(dpInfo?.vendor || "") ? "Apple GPUs emulate dp4a" : !(await probeDp4a(device)) ? "no packed_4x8_integer_dot_product on this device" : "";
        if (why) { if (!auto) console.warn(`moeGroupDp4a: off (${why})`); } else this.moeDp4a = true;
      } else if (!auto && moeGroupDp4a) console.warn("moeGroupDp4a: off (needs the tiled expert-grouped prefill)");
    }
    this.moeGroupDp4a = this.moeDp4a;

    // prefill GEMM operand precision (see PREFILL_MATH above). _pmAvail: the modes built for this
    // engine; engine.prefillMath can be switched at runtime among them (A/B), anything else runs f32.
    let pm = prefillMath ?? envPrefillMath() ?? "f32";
    if (!PREFILL_MATH.includes(pm)) { console.warn(`prefillMath "${pm}" unknown (${PREFILL_MATH.join(" | ")}): using f32`); pm = "f32"; }
    this.prefillMathReq = pm;
    this._pmAvail = { f32: true, f16: false, sgmatrix: false };
    this._pmR16 = this.gemmOn && pm === "f16";
    this.sgmWhy = pm !== "f32" && !this.gemmOn ? "no prefill GEMM on this engine (batchCols < 16, gemm: false, or no GEMM shapes)" : "";

    // ---- pipelines with explicit layouts ----
    const unpack = await probeUnpack(device);
    // one module per entry point, pruned to what it uses (engine/wgsl/prune.js): Dawn reprocesses the
    // whole ~650 KB module for every pipeline otherwise (~1.3 s each under Windows' FXC)
    const mod = moduleSet(device, WGSL + coopWGSL(coopWG, coopRows, 64, batchCols, coopRowsB, unpack, true, true)
      + (this.moe ? moeWGSL(this.moeK) : "")
      + (this.dnNba ? normRouterKernel({ ROWS: 4, bf16: false, name: "dn_nba", P: "nba", struct: !(this.moeFuse && this.moe.nrt) }) : "")
      + (this.moeFuse ? moeFusedWGSL({ K: this.moe.K, R: this.moe.R, layout: this.moe.layout, nrt: this.moe.nrt, gu: this.moe.guPairs.map((p) => p.split("_")), dn: this.moe.dnPairs.map((p) => p.split("_")) }) : "")
      + (this.moeGrpU && this.moeGrpTiled ? tiledGroupWGSL({ K: this.moe.K, UC: this.moeGrpUC, U: this.moeGrpU, nExp: this.moe.nExp,
        gu: this.moe.guPairs.map((p) => p.split("_")), dn: this.moe.dnPairs.map((p) => p.split("_")) }) : "")
      + (this.moeDp4a ? dp4aGroupWGSL({ K: this.moe.K, UC: this.moeGrpUC, U: this.moeGrpU, nExp: this.moe.nExp, sort: false,
        gu: this.moe.guPairs.map((p) => p.split("_")), dn: this.moe.dnPairs.map((p) => p.split("_")) }) : "")
      + (this.moeGrpU && !this.moeGrpTiled ? moeGroupWGSL({ K: this.moe.K, R: dnGroupRows(this.moeGrpUC), UC: this.moeGrpUC, U: this.moeGrpU, nExp: this.moe.nExp,
        gu: this.moe.guPairs.map((p) => p.split("_")), dn: this.moe.dnPairs.map((p) => p.split("_")) }) : "")
      + (this.gemmOn ? gemmWGSL({ N: batchCols, pairs: this._gemmPairs, pairs8: this._gemm8Pairs, UNPACK: unpack, R16: this._pmR16 }) : "")
      + (this.ubatch ? gemmWideWGSL(this.wideCfg, { UNPACK: unpack }) : "")
      + (this.dp4aCfg ? gemmDp4aWGSL(this.dp4aCfg) : "") + SILU_MUL_W_WGSL + WGSL2 + layerFuseWGSL());
    const C = GPUShaderStage.COMPUTE;
    const layout0 = device.createBindGroupLayout({
      entries: [
        { binding: 0, visibility: C, buffer: { type: "uniform" } },
        { binding: 1, visibility: C, buffer: { type: "uniform" } },
      ],
    });
    const G1 = {
      matvec: ["ro", "ro", "rw", "u"], matvec_q8: ["ro", "ro", "ro", "rw", "u"],
      matvec_q4: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop: ["ro", "ro", "rw", "u"], matvec_q8_coop: ["ro", "ro", "ro", "rw", "u"],
      matvec_q4_coop: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop_acc: ["ro", "ro", "rw", "u"], matvec_q8_coop_acc: ["ro", "ro", "ro", "rw", "u"], matvec_q4_coop_acc: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop_b_acc: ["ro", "ro", "rw", "u"], matvec_q8_coop_b_acc: ["ro", "ro", "ro", "rw", "u"], matvec_q4_coop_b_acc: ["ro", "ro", "ro", "rw", "u"],
      matvec_coop_b: ["ro", "ro", "rw", "u"], matvec_q8_coop_b: ["ro", "ro", "ro", "rw", "u"],
      matvec_q4_coop_b: ["ro", "ro", "ro", "rw", "u"],
      matvec_gu: ["ro", "ro", "ro", "rw", "u"], matvec_gu_b: ["ro", "ro", "ro", "rw", "u"],
      matvec_q8_gu: ["ro", "ro", "ro", "ro", "ro", "rw", "u"], matvec_q8_gu_b: ["ro", "ro", "ro", "ro", "ro", "rw", "u"],
      matvec_q4_gu: ["ro", "ro", "ro", "ro", "ro", "rw", "u"], matvec_q4_gu_b: ["ro", "ro", "ro", "ro", "ro", "rw", "u"],
      rmsnorm: ["ro", "ro", "rw", "u"], head_norm: ["rw", "ro", "u"],
      attn_scores: ["ro", "ro", "rw"], attn_softmax: ["rw"], attn_softmax_wg: ["rw"],
      attn_out: ["ro", "ro", "rw"], silu_mul: ["rw", "ro"], add_res: ["rw", "ro"],
      dn_conv: ["ro", "ro", "rw", "rw", "u"],
      dn_gates: ["ro", "ro", "ro", "ro", "rw", "rw", "u"],
      dn_l2: ["rw", "u", "u"],
      dn_delta: ["ro", "ro", "ro", "ro", "ro", "rw", "rw", "u"],
      dn_gatenorm: ["ro", "ro", "ro", "rw", "u"],
      qsplit: ["ro", "rw", "rw", "u"],
      rope_part: ["rw", "u", "u"],
      sigmoid_mul: ["rw", "ro", "u"],
      rmsnorm_mc: ["ro", "ro", "rw", "u"], add_res_mc: ["rw", "ro", "u"],
      dn_gates_mc: ["ro", "ro", "ro", "ro", "rw", "rw", "u"], dn_conv_mc: ["ro", "ro", "rw", "rw", "u", "rw"],
      dn_pre: ["ro", "ro", "ro", "ro", "rw", "rw", "rw", "u"], dn_pre_mc: ["ro", "ro", "ro", "ro", "rw", "rw", "rw", "u", "u"],
      dn_l2_mc: ["rw", "u", "u"], dn_delta_mc: ["ro", "ro", "ro", "rw", "rw", "u", "u", "rw"], dn_conv_restore: ["ro", "rw", "u"],
      dn_gatenorm_mc: ["ro", "ro", "ro", "rw", "u", "u"], qsplit_mc: ["ro", "rw", "rw", "u", "u"],
      head_norm_mc: ["rw", "ro", "u"], rope_part_mc: ["rw", "u", "u"], sigmoid_mul_mc: ["rw", "ro", "u"],
      attn_glue: ["ro", "rw", "rw", "rw", "ro", "ro", "u", "u"],
      dn_delta_gn: ["ro", "ro", "ro", "rw", "ro", "ro", "rw", "u"],
      dn_delta_gnp: ["ro", "ro", "ro", "ro", "rw", "ro", "ro", "rw", "u"],
      attn_glue_kv: ["ro", "rw", "ro", "ro", "ro", "ro", "rw", "rw", "u"], attn_combine_g: ["ro", "ro", "rw", "ro", "u"],
      matvec_coop_cv: ["ro", "ro", "rw", "u", "ro", "rw", "rw", "u"],
      matvec_q8_coop_cv: ["ro", "ro", "ro", "rw", "u", "ro", "rw", "rw", "u"], matvec_q4_coop_cv: ["ro", "ro", "ro", "rw", "u", "ro", "rw", "rw", "u"],
      matvec_coop_n: ["ro", "ro", "rw", "u", "ro", "rw"], matvec_q8_coop_n: ["ro", "ro", "ro", "rw", "u", "ro", "rw"], matvec_q4_coop_n: ["ro", "ro", "ro", "rw", "u", "ro", "rw"],
      kv_store: ["ro", "ro", "rw", "rw", "u"], attn_flash: ["ro", "ro", "ro", "rw", "rw", "u"], attn_combine: ["ro", "ro", "rw", "u"],
      kv_store_q8: ["ro", "ro", "rw", "rw", "rw", "rw", "u"], attn_flash_q8: ["ro", "ro", "ro", "ro", "ro", "rw", "rw", "u"], attn_flash_t2: ["ro", "ro", "ro", "rw", "rw", "u"],
      attn_scores_mc: ["ro", "ro", "rw", "u"], attn_softmax_wg_mc: ["rw"], attn_out_mc: ["ro", "ro", "rw", "u"],
      argmax: ["ro", "rw", "u"], emb_gather: ["ro", "ro", "ro", "rw", "u"],
      topk_a: ["ro", "rw", "u"], topk_b: ["ro", "rw", "u"],
    };
    // the LM head's kernels only where the head is: a device without it never runs them, and a
    // compiler that rejects one of them (FXC once refused topk_b) then cannot fail that device
    if (!hasHead) for (const n of ["argmax", "emb_gather", "topk_a", "topk_b"]) delete G1[n];
    // layerFuse's kernels: only the flags on at create, and optional (see the layerFuse note above)
    const lfOf = new Map();
    for (const [k, names] of Object.entries(LF_PIPES)) for (const n of names) { if (this.layerFuse[k]) lfOf.set(n, k); else delete G1[n]; }
    const lfFails = [], dpFails = [], mdpFails = [];
    if (this.moe) Object.assign(G1, {
      moe_router: ["ro", "rw", "rw", "u"], moe_combine: ["rw", "ro", "ro", "ro", "ro", "u"],
      moe_gu_q4: ["ro", "ro", "ro", "ro", "ro", "rw", "ro", "u"], moe_gu_q8: ["ro", "ro", "ro", "ro", "ro", "rw", "ro", "u"],
      moe_dn_q4: ["ro", "ro", "ro", "rw", "ro", "u"], moe_dn_q8: ["ro", "ro", "ro", "rw", "ro", "u"],
    });
    if (this.dnNba) G1.dn_nba = ["ro", "ro", "ro", "rw", "rw", "u"];
    if (this.moeFuse) {
      G1.moe_route = ["ro", "rw", "rw", "u"];
      if (this.moe.nrt) G1.moe_nrt = ["ro", "ro", "ro", "rw", "rw", "u"];
      for (const p of this.moe.guPairs) G1["moe_gus_" + p] = ["ro", "ro", "ro", "ro", "ro", "rw", "ro", "ro", "u"];
      for (const p of this.moe.dnPairs) G1["moe_dnc_" + p] = ["ro", "ro", "ro", "rw", "ro", "ro", "ro", "ro", "u"];
    }
    if (this.moeGrpU) {
      G1.moe_gsort = ["ro", "rw", "rw", "u"];
      G1.moe_combw = ["rw", "ro", "ro", "u"];
      for (const p of this.moe.guPairs) G1["moe_gusg_" + p] = ["ro", "ro", "ro", "ro", "ro", "rw", "ro", "ro", "u"];
      for (const p of this.moe.dnPairs) G1["moe_dng_" + p] = ["ro", "ro", "ro", "rw", "ro", "ro", "ro", "u"];
    }
    const mdpPipes = this.moeDp4a ? ["moe_qx", ...this.moe.guPairs.map((p) => "moe_gusq_" + p), ...this.moe.dnPairs.map((p) => "moe_dnq_" + p)] : [];
    if (this.moeDp4a) {
      G1.moe_qx = ["ro", "rw", "u"];
      for (const p of this.moe.guPairs) G1["moe_gusq_" + p] = ["ro", "ro", "ro", "ro", "ro", "rw", "ro", "ro", "u"];
      for (const p of this.moe.dnPairs) G1["moe_dnq_" + p] = ["ro", "ro", "ro", "rw", "ro", "ro", "ro", "u"];
    }
    // narrower twins: a verify or tail pass with w live columns pays for w, not batchCols
    for (const W of [8, 4]) if (batchCols > W) Object.assign(G1, {
      [`matvec_coop_b${W}`]: G1.matvec_coop_b, [`matvec_q8_coop_b${W}`]: G1.matvec_q8_coop_b, [`matvec_q4_coop_b${W}`]: G1.matvec_q4_coop_b,
      [`matvec_coop_b${W}_acc`]: G1.matvec_coop_b, [`matvec_q8_coop_b${W}_acc`]: G1.matvec_q8_coop_b, [`matvec_q4_coop_b${W}_acc`]: G1.matvec_q4_coop_b,
      [`matvec_gu_b${W}`]: G1.matvec_gu_b, [`matvec_q8_gu_b${W}`]: G1.matvec_q8_gu_b, [`matvec_q4_gu_b${W}`]: G1.matvec_q4_gu_b,
    });
    // the prefill GEMM and its split-K reduce / transpose all reuse the
    // matvec_q4_coop_b layout (qs, sc, x, y, shape) verbatim
    if (this.gemmOn) {
      for (const [dIn, S] of this._gemmPairs) G1[`gemm_q4_${dIn}_s${S}`] = G1.matvec_q4_coop_b;
      for (const [dIn, S] of this._gemm8Pairs) G1[`gemm_q8_${dIn}_s${S}`] = G1.matvec_q4_coop_b;
      for (const S of this._gemmSplits) { G1[`gemm_red_s${S}`] = G1.matvec_q4_coop_b; G1[`gemm_red_s${S}_acc`] = G1.matvec_q4_coop_b; }
      G1.gemm_xpose = G1.matvec_q4_coop_b;
      if (this._pmR16) {
        for (const [dIn, S] of this._gemmPairs) G1[`gemm_q4_${dIn}_s${S}_r16`] = G1.matvec_q4_coop_b;
        for (const [dIn, S] of this._gemm8Pairs) G1[`gemm_q8_${dIn}_s${S}_r16`] = G1.matvec_q4_coop_b;
        G1.gemm_xpose_r16 = G1.matvec_q4_coop_b;
      }
    }
    // wide prefill GEMMs: the same (qs, sc, x, y, shape) layout
    if (this.ubatch) {
      for (const p of ["gemm_w_q4", "gemm_w_q4_acc", "gemm_w_q8", "gemm_w_q8_acc"]) G1[p] = G1.matvec_q4_coop_b;
    }
    G1.silu_mul_w = ["rw", "ro", "u"];   // SiLU(g) * u over many columns: the wide chunk and the batched pass (_encodeLayerBatch)
    if (this.dp4aCfg) {
      for (const p of DP4A_PIPES.slice(0, 4)) G1[p] = ["ro", "ro", "ro", "ro", "rw", "u"];
      G1.quant_q8_w = ["ro", "rw", "rw", "u"];
    }
    const bufType = { u: "uniform", ro: "read-only-storage", rw: "storage" };
    this.pipes = {};
    // compile every pipeline in parallel (async): overlaps shader compilation
    // with the rest of setup instead of serializing 20+ compiles
    await Promise.all(Object.entries(G1).map(async ([name, spec]) => {
      const layout1 = device.createBindGroupLayout({
        entries: spec.map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })),
      });
      const desc = { layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module: mod(name), entryPoint: name } };
      if (DP4A_PIPES.includes(name)) {   // optional too: a compiler that rejects them leaves the f32 wide GEMM
        try { this.pipes[name] = await compilePipeline(device, desc); } catch (e) { dpFails.push(e); }
        return;
      }
      if (mdpPipes.includes(name)) {   // optional: a compiler that rejects them leaves the f32 tiled expert kernels
        try { this.pipes[name] = await compilePipeline(device, desc); } catch (e) { mdpFails.push(e); }
        return;
      }
      if (!lfOf.has(name)) { this.pipes[name] = await compilePipeline(device, desc); return; }
      try { this.pipes[name] = await compilePipeline(device, desc); } catch (e) { lfFails.push(e); }
    }));
    if (dpFails.length) {
      for (const n of DP4A_PIPES) delete this.pipes[n];
      this.dp4aCfg = null; this.prefillDp4a = false;
      console.warn(`prefillDp4a off: ${dpFails.map((e) => e.message).join("; ")}`);
    }
    if (mdpFails.length) {
      for (const n of mdpPipes) delete this.pipes[n];
      this.moeDp4a = false; this.moeGroupDp4a = false;
      console.warn(`moeGroupDp4a off: ${mdpFails.map((e) => e.message).join("; ")}`);
    }
    if (lfFails.length) {   // the unfused kernels compiled (or the load would have failed above): run those
      for (const n of lfOf.keys()) delete this.pipes[n];
      for (const k of Object.keys(this.layerFuse)) this.layerFuse[k] = false;
      this.layerFuseError = lfFails.map((e) => e.message).join("; ");
      console.warn(`layerFuse off: ${this.layerFuseError}`);
    }
    this._pmAvail.f16 = this._pmR16;
    if (pm === "sgmatrix" && this.gemmOn) {
      const spec = G1.matvec_q4_coop_b;
      const layout1 = device.createBindGroupLayout({ entries: spec.map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
      this._pmAvail.sgmatrix = await this._initSgm(device, device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), layout0, adapterInfo,
        { ...SGM_DEFAULT, ...(prefillSgm || {}) });
    }
    this.prefillMath = this._pmAvail[pm] ? pm : "f32";
    if (pm !== "f32" && this.prefillMath !== pm) console.warn(`prefillMath ${pm} unavailable${this.sgmWhy ? ": " + this.sgmWhy : ""}; prefill uses the f32 GEMM`);

    if (this.attnPTCfg) await this._initAttnTile(device, layout0, bufType);
    if (this.adCfg) await this._initAttnDec(device, layout0, bufType);
    this.attnDecode = this._adWant === "v2" && this.pipes.attn_dec ? "v2" : "v1";
    this.attnPrefillTile = !!this.pipes.attn_flash_tile && aptOn;

    // LM head GEMVs (full and draft) with their own rows per workgroup (off by default: no gain on the
    // 27B head, 5.78 vs 5.79 ms on the GB10): rows per workgroup never enter
    // a row's arithmetic, so the logits are bit-identical to the coopRows kernel (tests/bench_wide.js)
    this.headRows = hasHead && matvecVariant === "coop" && headRows > 0 && headRows !== coopRows ? headRows : 0;
    if (this.headRows) {
      const modH = moduleSet(device, WGSL + coopWGSL(coopWG, this.headRows, 64, batchCols, coopRowsB, unpack));
      for (const name of ["matvec_q8_coop", "matvec_q4_coop", "matvec_coop"]) {
        const layout1 = device.createBindGroupLayout({ entries: G1[name].map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
        this.pipes[name + "_h"] = await compilePipeline(device, { layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module: modH(name), entryPoint: name } });
      }
    }
    // ---- uniforms ----
    const cfgData = new ArrayBuffer(48);
    const cu = new Uint32Array(cfgData), cf = new Float32Array(cfgData);
    cu.set([dim, kvDim, nH, nKV, hd, inter, vocab, maxSeq], 0);
    cf[8] = eps; cf[9] = ropeTheta; cu[10] = qDim;
    this.cfgBuf = this._buf(cfgData, GPUBufferUsage.UNIFORM);
    this.frameBuf = device.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
    const dnData = new ArrayBuffer(48);
    const du = new Uint32Array(dnData), df = new Float32Array(dnData);
    du.set([convDim, dState, nKH, nVH, keyDim, nRot, hd, dInner], 0);
    df[8] = ropeTheta; df[9] = eps;
    this.dnBuf = this._buf(dnData, GPUBufferUsage.UNIFORM);
    this._shapes = {};
    this.uNH = this._buf(new Uint32Array([nH]), GPUBufferUsage.UNIFORM);
    this.uNKV = this._buf(new Uint32Array([nKV]), GPUBufferUsage.UNIFORM);
    this.uNKH = this._buf(new Uint32Array([nKH]), GPUBufferUsage.UNIFORM);
    this.uDim = this._buf(new Uint32Array([dim]), GPUBufferUsage.UNIFORM);
    this.uQDim = this._buf(new Uint32Array([qDim]), GPUBufferUsage.UNIFORM);
    this.uHd = this._buf(new Uint32Array([hd]), GPUBufferUsage.UNIFORM);

    // ---- working buffers ----
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC;
    this.x = device.createBuffer({ size: dim * 4, usage: S });
    this.xn = device.createBuffer({ size: dim * 4, usage: S });
    this.tmpDim = device.createBuffer({ size: dim * 4, usage: S });
    this.g = device.createBuffer({ size: inter * 4, usage: S });
    this.u = device.createBuffer({ size: inter * 4, usage: S });
    // delta-net
    // fuseProj: the outputs of a merged GEMV share one buffer; each tensor is a 256-byte aligned
    // view of it (segment starts padded to 64 rows), bound exactly like the old buffer
    const segBufs = (sizes) => {   // sizes: [rows, view bytes] per segment -> views of one buffer
      const offs = Qwen35Engine._segOffs(sizes.map(([r]) => r));
      const buf = device.createBuffer({ size: Math.max(...sizes.map(([, b], i) => offs[i] * 4 + b)), usage: S });
      return sizes.map(([, b], i) => Qwen35Engine._view(buf, offs[i] * 4, b));
    };
    if (this.fuseProjOn) {
      [this.qkv, this.z] = segBufs([[convDim, convDim * 4], [dInner, dInner * 4]]);
      [this.betaRaw, this.alpha] = segBufs([[nVH, 64 * 4], [nVH, 64 * 4]]);
    } else {
      this.qkv = device.createBuffer({ size: convDim * 4, usage: S });
      this.z = device.createBuffer({ size: dInner * 4, usage: S });
      this.alpha = device.createBuffer({ size: 64 * 4, usage: S });
      this.betaRaw = device.createBuffer({ size: 64 * 4, usage: S });
    }
    this.convOut = device.createBuffer({ size: convDim * 4, usage: S });
    this.beta = device.createBuffer({ size: 64 * 4, usage: S });
    this.decay = device.createBuffer({ size: 64 * 4, usage: S });
    this.dOut = device.createBuffer({ size: dInner * 4, usage: S });
    this.gated = device.createBuffer({ size: dInner * 4, usage: S });
    // full attention
    this.qFull = device.createBuffer({ size: nH * hd * 2 * 4, usage: S });
    this.q = device.createBuffer({ size: qDim * 4, usage: S });
    this.gAttn = device.createBuffer({ size: qDim * 4, usage: S });
    // k and v share a buffer under fuseProj; q stays apart: attn_glue reads qFull read-only while
    // it rewrites k, and one dispatch cannot bind a buffer both read-only and writable
    if (this.fuseProjOn) [this.k, this.v] = segBufs([[kvDim, kvDim * 4], [kvDim, kvDim * 4]]);
    else {
      this.k = device.createBuffer({ size: kvDim * 4, usage: S });
      this.v = device.createBuffer({ size: kvDim * 4, usage: S });
    }
    this.attnOut = device.createBuffer({ size: qDim * 4, usage: S });
    this.scores = device.createBuffer({ size: (this.flash ? 1 : nH * maxSeq) * 4, usage: S });
    if (this.flash) {   // split partials for every batch column (column 0 doubles as the decode's)
      const NCf = Math.max(1, batchCols);
      const slots = Math.max(this.faSplits, this.adCfg ? this.adCfg.S : 0);   // attn_dec may use more slots
      this.faO = device.createBuffer({ size: NCf * nH * slots * hd * 4, usage: S });
      this.faML = device.createBuffer({ size: NCf * nH * slots * 2 * 4, usage: S });
      if (this.adCfg) this.adU1 = this._buf(new Uint32Array([0, 0, this.adCfg.S, this.adCfg.S]), GPUBufferUsage.UNIFORM);
      this.faU1 = this._buf(new Uint32Array([0, 0, this.faSplit, this.faSplits]), GPUBufferUsage.UNIFORM);
    }
    this.stageX = device.createBuffer({ size: dim * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });

    // ---- weights + per-layer resources ----
    const up = (e2) => {
      if (!e2) return null;
      if (e2.gpu) return e2.gpu;          // already streamed onto the GPU during download
      let r;
      if (e2.kind === "q8" || e2.kind === "q4")
        r = { kind: e2.kind, qs: this._buf(e2.qs, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC), sc: this._buf(e2.scales, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
      else r = { kind: "f32", buf: this._buf(e2.data, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
      e2.qs = e2.scales = e2.data = null; // release CPU copy once it lives on the GPU
      return r;
    };
    const coop = this.mvVariant === "coop";
    // acc: y[row] += W x (coop only) -> the residual add needs no dispatch
    const mv = (w, x, y, dOut, dIn, acc = false) => {
      const base = w.kind === "q8" ? "matvec_q8" : w.kind === "q4" ? "matvec_q4" : "matvec";
      acc = acc && coop;
      const pipe = coop ? base + "_coop" + (acc ? "_acc" : "") : base;
      const bufs = w.kind === "f32" ? [w.buf, x, y, this._shape(dOut, dIn)] : [w.qs, w.sc, x, y, this._shape(dOut, dIn)];
      return { pipe, acc, wgs: coop ? Math.ceil(dOut / this.coopRows) : Math.ceil(dOut / 64), bg: this._bg(this.pipes[pipe], 1, bufs) };
    };
    this._mv = mv;
    // layerFuse.norm: a GEMV over rmsnorm(x) with the norm computed in-kernel from the residual x
    // (coop.js NRM); workgroup 0 also writes xn for the layer's other GEMVs
    const mvN = (w, nw, y, dOut) => {
      if (!coop || !w || !nw || nw.kind !== "f32") return null;
      const pipe = (w.kind === "q8" ? "matvec_q8" : w.kind === "q4" ? "matvec_q4" : "matvec") + "_coop_n";
      if (!this.pipes[pipe]) return null;   // layerFuse.norm off at create, or its kernel did not compile
      const bufs = w.kind === "f32" ? [w.buf, this.x, y, this._shape(dOut, dim)] : [w.qs, w.sc, this.x, y, this._shape(dOut, dim)];
      return { pipe, wgs: Math.ceil(dOut / this.coopRows), bg: this._bg(this.pipes[pipe], 1, [...bufs, nw.buf, this.xn]) };
    };
    const guOp = (wg2, wu2, x, y, dOut, dIn, xB, yB) => {
      if (!coop || !wg2 || !wu2 || wg2.kind !== wu2.kind) return null;
      const base = wg2.kind === "q8" ? "matvec_q8_gu" : wg2.kind === "q4" ? "matvec_q4_gu" : "matvec_gu";
      const pipe = xB ? base + "_b" : base;
      const shp = xB ? this._shapeB(dOut, dIn, xB.stride / 16, yB.stride / 4) : this._shapeB(dOut, dIn, 0, 0);
      const bufs = wg2.kind === "f32" ? [wg2.buf, wu2.buf, x, y, shp] : [wg2.qs, wg2.sc, wu2.qs, wu2.sc, x, y, shp];
      const op = { pipe, wgs: Math.ceil(dOut / (xB ? this._rowsFor(this.NC) : this.coopRows)), bg: this._bg(this.pipes[pipe], 1, bufs) };
      if (xB) for (const W of [8, 4]) if (this.NC > W) {
        op[`pipe${W}`] = `${base}_b${W}`;
        op[`bg${W}`] = this._bg(this.pipes[op[`pipe${W}`]], 1, bufs);
        op[`wgs${W}`] = Math.ceil(dOut / this._rowsFor(W));
      }
      return op;
    };
    this._guOp = guOp;
    const bgNorm = (x, w, y) => this._bg(this.pipes.rmsnorm, 1, [x, w.buf, y, this.uDim]);

    // ---- expert offload (engine/expert_store.js): layers whose routed experts the loader parked in an ExpertStore ----
    // (qwen35Weights(..., { experts })). Their expert tensors become the store's region (the bind groups every
    // resident path builds) and its per-layer slot pool (one-token and batched passes, after a cut at moe_route).
    this.experts = null;
    {
      const offL = weights.layers.filter((L) => L.moe && L.expGate?.offload);
      if (offL.length) {
        if (!this.moeFuse) throw new Error("expert offload needs the fused MoE FFN (moeFuse)");
        if (!offL.every((L) => L.expUp?.offload && L.expDown?.offload && L.expGate.store === offL[0].expGate.store)) throw new Error("expert offload: a layer's experts are only partly parked");
        const st = offL[0].expGate.store;
        st.build({ K: this.moe.K });
        await st.checkAlloc();
        this.experts = st;
        const KS = this.moe.KS;
        this.offSel = device.createBuffer({ size: KS * 4, usage: S });   // one token's slot ids (moe_gus / moe_dnc on the pool)
        this.offStage = device.createBuffer({ size: Math.max(1, batchCols) * KS * 4, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST });
        this._cut = null;
      }
    }
    this.layers = [];
    if (this.moe) {   // one token's routing and expert activations
      const { nExp, K, inter: ei } = this.moe;
      // fuseProj (unfused MoE only): router logits and the shared-expert gate come out of one GEMV (one
      // buffer). The fused MoE FFN has its own [nExp + 1]-row router, whose row nExp is the shared gate.
      const [logits, sg] = this.fuseProjOn && !this.moeFuse ? segBufs([[nExp, nExp * 4], [1, 16]])
        : [device.createBuffer({ size: (nExp + 1) * 4, usage: S }), device.createBuffer({ size: 16, usage: S })];
      this.moeB = { logits, sel: device.createBuffer({ size: 32 * 4, usage: S }),
        selw: device.createBuffer({ size: 32 * 4, usage: S }), h: device.createBuffer({ size: K * ei * 4, usage: S }),
        y: device.createBuffer({ size: K * dim * 4, usage: S }), sh: device.createBuffer({ size: dim * 4, usage: S }), sg };
      if (this.moeFuse) this.moeB.hF = device.createBuffer({ size: this.moe.KS * this.moe.hs * 4, usage: S });
    }
    // fused MoE: the shared expert's gate and up (qs and f16 scales) packed into one buffer, so the
    // gate/up kernel stays within 8 storage bindings: [gate qs | up qs | gate scales | up scales]
    const packGU = (g, u) => {
      const parts = [g.gpu ? g.gpu.qs : g.qs, u.gpu ? u.gpu.qs : u.qs, g.gpu ? g.gpu.sc : g.scales, u.gpu ? u.gpu.sc : u.scales];
      const sizes = parts.map((b) => b.byteLength !== undefined ? b.byteLength : b.size);
      if (sizes.some((n) => n % 4)) throw new Error("packGU: unaligned shared-expert tensor");
      const offs = [0]; for (const n of sizes) offs.push(offs[offs.length - 1] + n);
      // the wide fused kernels read it as vec4<u32>: whole 16 B elements, and the up qs start on one
      if (this.moe.layout && offs[1] % 16) throw new Error("packGU: shared up qs not 16-byte aligned");
      const buf = device.createBuffer({ size: Math.ceil(offs[4] / 16) * 16, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST });
      let enc = null;
      parts.forEach((b, i) => {
        if (b.byteLength !== undefined) device.queue.writeBuffer(buf, offs[i], b.buffer, b.byteOffset, b.byteLength);
        else (enc ||= device.createCommandEncoder()).copyBufferToBuffer(b, 0, buf, offs[i], sizes[i]);
      });
      if (enc) device.queue.submit([enc.finish()]);
      // release the originals: CPU copies, or (streamed room path) the GPU buffers once the copy has run
      for (const e of [g, u]) if (e.gpu) { e.gpu.qs.destroy(); e.gpu.sc.destroy(); e.gpu = null; }
      g.qs = g.scales = u.qs = u.scales = null;
      return { buf, oUq: offs[1] / 4, oGs: offs[2] / 4, oUs: offs[3] / 4 };
    };
    const moeKind = (e, what) => {
      if (e.kind !== "q4" && e.kind !== "q8") throw new Error(`MoE ${what} weights must be Q4_0 or Q8_0 (got ${e.kind})`);
      return e.kind;
    };
    // fuseProj: row-concatenate same-format weights (see _fuseW); R.<name> become views into the
    // merged matrix, and the merged op is built only if the decode kernel's row grouping keeps
    // every original row in the same full/tail branch
    // engine.fuseStats: how many groups were merged / left apart (a quick check for validators)
    this.fuseStats = { merged: 0, apart: 0, decodeOps: 0 };
    const fuse = (parts, dIn) => {
      if (!this.fuseProjOn) return null;
      const m = this._fuseW(parts.map(([src, w, rows]) => ({ src, w, rows })), dIn);
      this.fuseStats[m ? "merged" : "apart"]++;
      const decodeOK = !!m && Qwen35Engine._rowsKeep(parts.map((p) => p[2]), this.coopRows);
      if (decodeOK) this.fuseStats.decodeOps++;
      return m && { ...m, decodeOK };
    };
    const buildLayer = (L) => {
      const R = { isFull: L.isFull };
      R.attnNorm = up(L.attnNorm); R.postNorm = up(L.postNorm);
      R.bgNorm1 = bgNorm(this.x, R.attnNorm, this.xn);
      R.bgNorm2 = bgNorm(this.x, R.postNorm, this.xn);
      if (L.moe && this.moeFuse) {
        const { nExp, inter: ei, norm, sDim, hs } = this.moe, MB = this._moeTraceBufs(R);
        R.moe = true; R.fused = true; R.shared = true;
        // router and shared-expert gate as one F32 [nExp + 1][dim] matrix: one GEMV, logit nExp is the shared gate
        const rp = new Float32Array((nExp + 1) * dim);
        rp.set(L.router.data); rp.set(L.shRouter.data, nExp * dim);
        L.router.data = L.shRouter.data = null;
        const nrt = this.moe.nrt;
        if (nrt && nrt.bf16) {   // moe_nrt's BF16 router: two values per word (the f32 values' high halves, exact)
          const u = new Uint32Array(rp.buffer), pk = new Uint32Array(u.length / 2);
          for (let i = 0; i < pk.length; i++) pk[i] = (u[2 * i] >>> 16) | (u[2 * i + 1] & 0xffff0000);
          R.router = { kind: "bf16", buf: this._buf(pk, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
        } else R.router = { kind: "f32", buf: this._buf(rp, GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC) };
        // offloaded (L.expGate.offload): the experts bind the store's whole-layer region on every resident code path
        // (filled per use: _cutResolve, ExpertStore.loadLayer), plus the layer's slot pool for the cut passes (bgGusP / bgDncP)
        const off = L.expGate.offload ? L.expGate.layer : -1;
        if (off >= 0) {
          R.off = true; R.offLayer = off;
          [R.expGate, R.expUp, R.expDown] = ["gate", "up", "down"].map((p) => this.experts.regionEntry(off, p));
          [R.poolGate, R.poolUp, R.poolDown] = ["gate", "up", "down"].map((p) => this.experts.poolEntry(off, p));
        } else { R.expGate = up(L.expGate); R.expUp = up(L.expUp); R.expDown = up(L.expDown); }
        const lim = device.limits.maxStorageBufferBindingSize;
        for (const [w, n] of [[R.expGate, "ffn_gate_exps"], [R.expUp, "ffn_up_exps"], [R.expDown, "ffn_down_exps"], ...(R.off ? [[R.poolGate, "pool gate"], [R.poolUp, "pool up"], [R.poolDown, "pool down"]] : [])])
          if (w.qs.size > lim) throw new Error(`${n} is ${(w.qs.size / 2 ** 20).toFixed(0)} MB, over this device's ${(lim / 2 ** 20).toFixed(0)} MB storage-binding limit (a Q4_0 file needs less)`);
        if (R.expUp.kind !== R.expGate.kind) throw new Error("MoE gate and up experts must share a format");
        R.shPack = packGU(L.shGate, L.shUp);
        R.ffnDown = up(L.shDown);
        R.gusPipe = `moe_gus_${R.expGate.kind}_${L.shGate.kind}`; R.dncPipe = `moe_dnc_${R.expDown.kind}_${R.ffnDown.kind}`;
        R.gusgPipe = R.gusPipe.replace("moe_gus_", "moe_gusg_"); R.dngPipe = R.dncPipe.replace("moe_dnc_", "moe_dng_");
        R.gusqPipe = R.gusPipe.replace("moe_gus_", "moe_gusq_"); R.dnqPipe = R.dncPipe.replace("moe_dnc_", "moe_dnq_");
        R.moeU = (xs, dOut, dIn, ys, pk) => [dOut, dIn, sDim, nExp, xs, ys, norm, 1, pk ? pk.oUq : 0, pk ? pk.oGs : 0, pk ? pk.oUs : 0, 0];
        const U = (a) => this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM);
        if (nrt) R.bgNrt = this._bg(this.pipes.moe_nrt, 1, [this.x, R.postNorm.buf, R.router.buf, this.xn, MB.logits, U([dim, nExp + 1, dim, nExp + 1, dim, 0, 0, 0])]);
        else R.mvRouter = mv(R.router, this.xn, MB.logits, nExp + 1, dim);
        R.bgRoute = this._bg(this.pipes.moe_route, 1, [MB.logits, MB.sel, MB.selw, U(R.moeU(nExp + 1, 0, 0, 0))]);
        R.bgGus = this._bg(this.pipes[R.gusPipe], 1, [R.expGate.qs, R.expGate.sc, R.expUp.qs, R.expUp.sc, this.xn, MB.hF, MB.sel, R.shPack.buf,
          U(R.moeU(dim, ei, dim, hs, R.shPack))]);
        R.bgDnc = this._bg(this.pipes[R.dncPipe], 1, [R.expDown.qs, R.expDown.sc, MB.hF, this.x, MB.sel, MB.selw, R.ffnDown.qs, R.ffnDown.sc,
          U(R.moeU(dim, dim, ei, hs))]);
        if (R.off) {   // the same kernels on the slot pool, the slot ids (offSel) in place of the expert ids
          R.selBuf = MB.sel;
          R.bgGusP = this._bg(this.pipes[R.gusPipe], 1, [R.poolGate.qs, R.poolGate.sc, R.poolUp.qs, R.poolUp.sc, this.xn, MB.hF, this.offSel, R.shPack.buf,
            U(R.moeU(dim, ei, dim, hs, R.shPack))]);
          R.bgDncP = this._bg(this.pipes[R.dncPipe], 1, [R.poolDown.qs, R.poolDown.sc, MB.hF, this.x, this.offSel, MB.selw, R.ffnDown.qs, R.ffnDown.sc,
            U(R.moeU(dim, dim, ei, hs))]);
        }
      } else if (L.moe) {
        const { nExp, K, inter: ei, norm } = this.moe, MB = this._moeTraceBufs(R);
        R.moe = true;
        R.router = up(L.router); R.expGate = up(L.expGate); R.expUp = up(L.expUp); R.expDown = up(L.expDown);
        const lim = device.limits.maxStorageBufferBindingSize;
        for (const [w, n] of [[R.expGate, "ffn_gate_exps"], [R.expUp, "ffn_up_exps"], [R.expDown, "ffn_down_exps"]])
          if (w.qs.size > lim) throw new Error(`${n} is ${(w.qs.size / 2 ** 20).toFixed(0)} MB, over this device's ${(lim / 2 ** 20).toFixed(0)} MB storage-binding limit (a Q4_0 file needs less)`);
        const gk = moeKind(R.expGate, "gate/up");
        if (moeKind(R.expUp, "gate/up") !== gk) throw new Error("MoE gate and up experts must share a format");
        R.guPipe = "moe_gu_" + gk; R.dnPipe = "moe_dn_" + moeKind(R.expDown, "down");
        const U = (a) => this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM);
        R.mvRouter = mv(R.router, this.xn, MB.logits, nExp, dim);
        R.bgRouter = this._bg(this.pipes.moe_router, 1, [MB.logits, MB.sel, MB.selw, U([0, 0, K, nExp, nExp, 0, norm, 0])]);
        R.bgGu = this._bg(this.pipes[R.guPipe], 1, [R.expGate.qs, R.expGate.sc, R.expUp.qs, R.expUp.sc, this.xn, MB.h, MB.sel, U([ei, dim, K, nExp, dim, ei, 0, 0])]);
        R.bgDn = this._bg(this.pipes[R.dnPipe], 1, [R.expDown.qs, R.expDown.sc, MB.h, MB.y, MB.sel, U([dim, ei, K, nExp, ei, dim, 0, 0])]);
        R.shared = !!L.shGate;
        if (R.shared) {
          R.ffnGate = up(L.shGate); R.ffnUp = up(L.shUp); R.ffnDown = up(L.shDown); R.shRouter = up(L.shRouter);
          R.mvGate = mv(R.ffnGate, this.xn, this.g, inter, dim);
          R.mvUp = mv(R.ffnUp, this.xn, this.u, inter, dim);
          R.gu = guOp(R.ffnGate, R.ffnUp, this.xn, this.g, inter, dim);
          R.mvShDown = mv(R.ffnDown, this.g, MB.sh, dim, inter);
          const f = fuse([[L.router, R.router, nExp], [L.shRouter, R.shRouter, 1]], dim);
          if (f) {
            [R.router, R.shRouter] = f.parts;
            R.fRS = f;
            R.mvRouter = mv(R.router, this.xn, MB.logits, nExp, dim);
            if (f.decodeOK) R.mvRS = mv(f.w, this.xn, MB.logits.buffer, f.rows, dim);
          }
          R.mvShRouter = mv(R.shRouter, this.xn, MB.sg, 1, dim);
        }
        R.bgMoeCombine = this._bg(this.pipes.moe_combine, 1, [this.x, MB.y, MB.selw, MB.sh, MB.sg, U([dim, 0, K, dim, dim, dim, R.shared ? 1 : 0, 1])]);
      } else {
        R.ffnGate = up(L.ffnGate); R.ffnUp = up(L.ffnUp); R.ffnDown = up(L.ffnDown);
        R.mvGate = mv(R.ffnGate, this.xn, this.g, inter, dim);
        R.mvUp = mv(R.ffnUp, this.xn, this.u, inter, dim);
        R.gu = guOp(R.ffnGate, R.ffnUp, this.xn, this.g, inter, dim);
        R.mvDown = coop ? mv(R.ffnDown, this.g, this.x, dim, inter, true) : mv(R.ffnDown, this.g, this.tmpDim, dim, inter);
      }
      if (L.isFull) {
        R.wq = up(L.wq); R.wk = up(L.wk); R.wv = up(L.wv); R.wo = up(L.wo);
        R.qNorm = up(L.qNorm); R.kNorm = up(L.kNorm);
        const kvBytes = this.kvQ8 ? 1 : this.flash ? 2 : 4;   // int8 / f16 pairs / f32
        R.kCache = device.createBuffer({ size: maxSeq * kvDim * kvBytes, usage: S });
        R.vCache = device.createBuffer({ size: maxSeq * kvDim * kvBytes, usage: S });
        if (this.kvQ8) {
          R.kScale = device.createBuffer({ size: maxSeq * kvDim / 32 * 4, usage: S });
          R.vScale = device.createBuffer({ size: maxSeq * kvDim / 32 * 4, usage: S });
        }
        const f = fuse([[L.wk, R.wk, kvDim], [L.wv, R.wv, kvDim]], dim);
        if (f) {
          [R.wk, R.wv] = f.parts;
          R.fKV = f;
          if (f.decodeOK) { R.mvKV = mv(f.w, this.xn, this.k.buffer, f.rows, dim); R.mvKVn = mvN(f.w, R.attnNorm, this.k.buffer, f.rows); }
        }
        R.mvQ = mv(R.wq, this.xn, this.qFull, nH * hd * 2, dim);
        R.mvK = mv(R.wk, this.xn, this.k, kvDim, dim);
        R.mvV = mv(R.wv, this.xn, this.v, kvDim, dim);
        R.mvO = coop ? mv(R.wo, this.attnOut, this.x, dim, qDim, true) : mv(R.wo, this.attnOut, this.tmpDim, dim, qDim);
        R.bgQsplit = this._bg(this.pipes.qsplit, 1, [this.qFull, this.q, this.gAttn, this.dnBuf]);
        R.bgQNorm = this._bg(this.pipes.head_norm, 1, [this.q, R.qNorm.buf, this.uNH]);
        R.bgKNorm = this._bg(this.pipes.head_norm, 1, [this.k, R.kNorm.buf, this.uNKV]);
        R.bgRopeQ = this._bg(this.pipes.rope_part, 1, [this.q, this.uNH, this.dnBuf]);
        R.bgRopeK = this._bg(this.pipes.rope_part, 1, [this.k, this.uNKV, this.dnBuf]);
        this._uZero4 = this._uZero4 || this._buf(new Uint32Array(4), GPUBufferUsage.UNIFORM);
        if (this.flash) {
          this._uZero4 = this._uZero4 || this._buf(new Uint32Array(4), GPUBufferUsage.UNIFORM);
          const sc = this.kvQ8 ? [R.kScale, R.vScale] : [];
          R.bgKvStore = this._bg(this.pipes[this.ksPipe], 1, [this.k, this.v, R.kCache, R.vCache, ...sc, this._uZero4]);
          R.bgFlash = this._bg(this.pipes[this.faPipe], 1, [this.q, R.kCache, R.vCache, ...sc, this.faO, this.faML, this.faU1]);
          R.bgCombine = this._bg(this.pipes.attn_combine, 1, [this.faO, this.faML, this.attnOut, this.faU1]);
          if (!this.kvQ8) {   // layerFuse.comb / .kv (when their kernels were built)
            if (this.pipes.attn_combine_g) R.bgCombineG = this._bg(this.pipes.attn_combine_g, 1, [this.faO, this.faML, this.attnOut, this.qFull, this.faU1]);
            if (this.attnGlueOn && this.pipes.attn_glue_kv) R.bgGlueKv = this._bg(this.pipes.attn_glue_kv, 1, [this.qFull, this.q, this.k, this.v, R.qNorm.buf, R.kNorm.buf, R.kCache, R.vCache, this.dnBuf]);
          }
          if (this.pipes.attn_dec) {
            R.bgDec = this._bg(this.pipes.attn_dec, 1, [this.q, R.kCache, R.vCache, this.faO, this.faML, this.adU1]);
            R.bgDecC = this._bg(this.pipes.attn_dec_combine, 1, [this.faO, this.faML, this.attnOut, this.adU1]);
            if (this.pipes.attn_dec_combine_g) R.bgDecCG = this._bg(this.pipes.attn_dec_combine_g, 1, [this.faO, this.faML, this.attnOut, this.qFull, this.adU1]);
          }
        }
        R.bgGlue = this._bg(this.pipes.attn_glue, 1, [this.qFull, this.q, this.gAttn, this.k, R.qNorm.buf, R.kNorm.buf, this._uZero4, this.dnBuf]);
        R.bgScores = this._bg(this.pipes.attn_scores, 1, [this.q, R.kCache, this.scores]);
        R.bgSoftmax = this._bg(this.pipes.attn_softmax, 1, [this.scores]);
        R.bgAttnOut = this._bg(this.pipes.attn_out, 1, [this.scores, R.vCache, this.attnOut]);
        R.bgSigMul = this._bg(this.pipes.sigmoid_mul, 1, [this.attnOut, this.gAttn, this.uQDim]);
      } else {
        R.wqkv = up(L.wqkv); R.wz = up(L.wz);
        R.wBeta = up(L.wBeta); R.wAlpha = up(L.wAlpha);
        R.wOut = up(L.wOut);
        R.dtBias = this._buf(L.dtBias.data, GPUBufferUsage.STORAGE);
        R.ssmA = this._buf(L.ssmA.data, GPUBufferUsage.STORAGE);
        R.convW = this._buf(L.conv.data, GPUBufferUsage.STORAGE);
        R.ssmNorm = this._buf(L.ssmNorm.data, GPUBufferUsage.STORAGE);
        R.convState = device.createBuffer({ size: convDim * 3 * 4, usage: S });
        R.S = device.createBuffer({ size: nVH * dState * dState * 4, usage: S });
        const fqz = fuse([[L.wqkv, R.wqkv, convDim], [L.wz, R.wz, dInner]], dim);
        if (fqz) {
          [R.wqkv, R.wz] = fqz.parts;
          R.fQZ = fqz;
          if (fqz.decodeOK) R.mvQZ = mv(fqz.w, this.xn, this.qkv.buffer, fqz.rows, dim);
        }
        const fba = fuse([[L.wBeta, R.wBeta, nVH], [L.wAlpha, R.wAlpha, nVH]], dim);
        if (fba) {
          [R.wBeta, R.wAlpha] = fba.parts;
          R.fBA = fba;
          if (fba.decodeOK) R.mvBA = mv(fba.w, this.xn, this.betaRaw.buffer, fba.rows, dim);
          // dnNormBA: input norm + beta / alpha in one launch (writes xn too); ls = output column stride
          if (this.dnNba && fba.w.kind === "f32") R.bgNba = this._bg(this.pipes.dn_nba, 1, [this.x, R.attnNorm.buf, fba.w.buf, this.xn, this.betaRaw.buffer,
            this._buf(new Uint32Array([dim, fba.rows, dim, fba.rows, dim, 0, 0, 0]), GPUBufferUsage.UNIFORM)]);
        }
        R.mvQKV = mv(R.wqkv, this.xn, this.qkv, convDim, dim);
        R.mvZ = mv(R.wz, this.xn, this.z, dInner, dim);
        R.mvBeta = mv(R.wBeta, this.xn, this.betaRaw, nVH, dim);
        R.mvAlpha = mv(R.wAlpha, this.xn, this.alpha, nVH, dim);
        R.mvOut = coop ? mv(R.wOut, this.gated, this.x, dim, dInner, true) : mv(R.wOut, this.gated, this.tmpDim, dim, dInner);
        R.bgConv = this._bg(this.pipes.dn_conv, 1, [this.qkv, R.convW, R.convState, this.convOut, this.dnBuf]);
        R.bgGates = this._bg(this.pipes.dn_gates, 1, [this.alpha, this.betaRaw, R.dtBias, R.ssmA, this.beta, this.decay, this.dnBuf]);
        R.bgPre = this._bg(this.pipes.dn_pre, 1, [this.alpha, this.betaRaw, R.dtBias, R.ssmA, this.beta, this.decay, this.convOut, this.dnBuf]);
        R.bgL2Q = this._bg2(this.pipes.dn_l2, [
          { buffer: this.convOut, offset: 0, size: keyDim * 4 },
          { buffer: this.uNKH }, { buffer: this.dnBuf }]);
        R.bgL2K = this._bg2(this.pipes.dn_l2, [
          { buffer: this.convOut, offset: keyDim * 4, size: keyDim * 4 },
          { buffer: this.uNKH }, { buffer: this.dnBuf }]);
        R.bgDelta = this._bg2(this.pipes.dn_delta, [
          { buffer: this.convOut, offset: 0, size: keyDim * 4 },
          { buffer: this.convOut, offset: keyDim * 4, size: keyDim * 4 },
          { buffer: this.convOut, offset: keyDim * 2 * 4, size: dInner * 4 },
          { buffer: this.beta }, { buffer: this.decay },
          { buffer: R.S }, { buffer: this.dOut }, { buffer: this.dnBuf }]);
        R.bgGateNorm = this._bg(this.pipes.dn_gatenorm, 1, [this.dOut, this.z, R.ssmNorm, this.gated, this.dnBuf]);
        R.bgDeltaGn = this._bg(this.pipes.dn_delta_gn, 1, [this.convOut, this.beta, this.decay, R.S, this.z, R.ssmNorm, this.gated, this.dnBuf]);
        // layerFuse.dn: [dt bias | A] in one buffer, so dn_delta_gnp fits in 8 storage bindings
        if (this.pipes.dn_delta_gnp && dState === 128 && L.dtBias.data && L.ssmA.data && L.dtBias.data.length === nVH && L.ssmA.data.length === nVH) {
          const dtA = new Float32Array(2 * nVH); dtA.set(L.dtBias.data, 0); dtA.set(L.ssmA.data, nVH);
          R.dtA = this._buf(dtA, GPUBufferUsage.STORAGE);
          R.bgDeltaGnp = this._bg(this.pipes.dn_delta_gnp, 1, [this.convOut, this.betaRaw, this.alpha, R.dtA, R.S, this.z, R.ssmNorm, this.gated, this.dnBuf]);
        }
        // layerFuse.conv: the qkv GEMV (merged [qkv | z] when fuseProj) with dn_conv in its epilogue
        if (coop) {
          const cvOp = (w, y, dOut) => {
            const pipe = (w.kind === "q8" ? "matvec_q8" : w.kind === "q4" ? "matvec_q4" : "matvec") + "_coop_cv";
            if (!this.pipes[pipe]) return null;   // layerFuse.conv off at create, or its kernel did not compile
            const bufs = w.kind === "f32" ? [w.buf, this.xn, y, this._shape(dOut, dim)] : [w.qs, w.sc, this.xn, y, this._shape(dOut, dim)];
            return { pipe, wgs: Math.ceil(dOut / this.coopRows), bg: this._bg(this.pipes[pipe], 1, [...bufs, R.convW, R.convState, this.convOut, this.dnBuf]) };
          };
          if (R.mvQZ) R.mvQZcv = cvOp(fqz.w, this.qkv.buffer, fqz.rows);
          R.mvQKVcv = cvOp(R.wqkv, this.qkv, convDim);
        }
      }
      return R;
    };
    for (const L of weights.layers) this.layers.push(buildLayer(L));

    if (hasEmbed || hasHead) this.cpuEmbed = weights.embed;
    if (hasHead) {
      this.finalNorm = up(weights.finalNorm);
      if (weights.head) this.headEntry = up(weights.head);
      else { // tied: upload embed for the head but keep CPU copy for row lookups
        const e = weights.embed;
        this.headEntry = { kind: e.kind, qs: this._buf(e.qs, GPUBufferUsage.STORAGE), sc: this._buf(e.scales, GPUBufferUsage.STORAGE) };
      }
      this.logits = device.createBuffer({ size: vocab * 4, usage: S });
      this.stageLogits = device.createBuffer({ size: vocab * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
      // on-GPU argmax of the logits (draft chain): 8-byte readback instead of 1 MB
      this.argBuf = device.createBuffer({ size: 16, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC });
      this.stageArg = device.createBuffer({ size: 16, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
      this.bgArgmax = this._bg(this.pipes.argmax, 1, [this.logits, this.argBuf, this._buf(new Uint32Array([vocab, 0, 0, 0]), GPUBufferUsage.UNIFORM)]);
      this.bgFinalNorm = bgNorm(this.x, this.finalNorm, this.xn);
      const headMv = (n) => {
        const op = mv(this.headEntry, this.xn, this.logits, n, dim);
        if (!this.headRows) return op;
        const bufs = this.headEntry.kind === "f32" ? [this.headEntry.buf, this.xn, this.logits, this._shape(n, dim)] : [this.headEntry.qs, this.headEntry.sc, this.xn, this.logits, this._shape(n, dim)];
        return { pipe: op.pipe + "_h", acc: false, wgs: Math.ceil(n / this.headRows), bg: this._bg(this.pipes[op.pipe + "_h"], 1, bufs) };
      };
      this.headOp = headMv(vocab);
      // GPU sampling buffers (topk_a/topk_b): the stage-a partials (NC columns x nw workgroups x
      // (2k + 2) u32 at k <= 64), the per-column results (NC x (2k + 2) u32) and their staging buffers
      {
        const R = 2 * TOPK_MAX + 2, nc = Math.max(1, this.NC), nw = Math.ceil(vocab / 4096);
        const SU = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC;
        this.tkPart = device.createBuffer({ size: nc * nw * R * 4, usage: SU });
        this.topBuf = device.createBuffer({ size: nc * R * 4, usage: SU });
        this.stageTop = device.createBuffer({ size: R * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
        // + 8 x 16 B tail for the fused speculative step's drafts (as stageLogitsN)
        this.stageTopN = device.createBuffer({ size: nc * R * 4 + 128, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
        this._tkOps = new Map();
      }
      // Draft head over the first `draftVocab` rows only (BPE ids roughly follow frequency, so the
      // prefix holds the common tokens): the head is the biggest matrix a draft reads (0.7 GB on
      // the 27B), and drafts only need to be good guesses. The verify pass still uses the full
      // head, so the output is unchanged; a rare token just can't be drafted. Off by default.
      const dv = draftVocab > 0 && draftVocab < vocab ? Math.ceil(draftVocab / 64) * 64 : 0;
      if (dv && dv < vocab) {
        this.draftVocab = dv;
        this.headOpDraft = headMv(dv);
        this.bgArgmaxDraft = this._bg(this.pipes.argmax, 1, [this.logits, this.argBuf, this._buf(new Uint32Array([dv, 0, 0, 0]), GPUBufferUsage.UNIFORM)]);
        // The small head can only draft ids < dv. On English prose and code 1-2.5% of tokens lie
        // at or above 65536 (benchmarks/draftvocab_coverage.js), but on Chinese it is ~84% and on
        // Spanish ~18%, where the small head would draft almost nothing right. So watch the
        // share of prompt and output tokens >= dv (EMA over ~32 tokens) and draft with the full
        // head while it is high. draftVocabAuto = false keeps the small head always on.
        this.draftVocabAuto = draftVocabAuto !== false;
        this._dvMiss = 0; this._dvSmall = true;
        // Chat-template control tokens (<|im_start|>, <think>, ...) sit at the top of the vocab but
        // say nothing about the language, and a short chat prompt is ~20% of them, which pushed
        // every English chat onto the full head. Leave non-normal tokens (GGUF token_type 3/4/5:
        // control, user-defined, unused) out of the average.
        const tt = M["tokenizer.ggml.token_type"];
        if (tt?.length > dv) {
          this._dvSkip = new Uint8Array(tt.length - dv);
          for (let i = dv; i < tt.length; i++) if (tt[i] >= 3 && tt[i] <= 5) this._dvSkip[i - dv] = 1;
        }
      }
    }

    this.bgCommonFor = {};
    for (const [k2, p] of Object.entries(this.pipes))
      this.bgCommonFor[k2] = this._bg(p, 0, [this.cfgBuf, this.frameBuf]);
    this.bgSilu = this._bg(this.pipes.silu_mul, 1, [this.g, this.u]);
    this.bgAddTmp = this._bg(this.pipes.add_res, 1, [this.x, this.tmpDim]);

    // ---- multi-token prediction (draft) head ----
    if (weights.mtp && hasHead) {
      const W = weights.mtp;
      this.mtpLayer = buildLayer(W.layer);
      this.mtp = {
        ehProj: up(W.ehProj), enorm: up(W.enorm), hnorm: up(W.hnorm), headNorm: up(W.sharedHeadNorm),
        emb: device.createBuffer({ size: dim * 4, usage: S }),
        ehIn: device.createBuffer({ size: 2 * dim * 4, usage: S }),   // [enorm(e) | hnorm(h)]
        stats: { drafts: 0, accepted: 0 },
      };
      const M2 = this.mtp;
      M2.bgENorm = this._bg2res(this.pipes.rmsnorm, [{ buffer: M2.emb }, { buffer: M2.enorm.buf },
        { buffer: M2.ehIn, offset: 0, size: dim * 4 }, { buffer: this.uDim }]);
      M2.bgHNormX = this._bg2res(this.pipes.rmsnorm, [{ buffer: this.x }, { buffer: M2.hnorm.buf },
        { buffer: M2.ehIn, offset: dim * 4, size: dim * 4 }, { buffer: this.uDim }]);
      M2.proj = mv(M2.ehProj, M2.ehIn, this.x, dim, 2 * dim);           // eh_proj -> MTP residual (in x)
      M2.bgHeadNorm = bgNorm(this.x, M2.headNorm, this.xn);               // shared_head_norm -> xn
    }
    // One-submit draft chain (roadmap/26-host-side-overhead.md; on by default, draftChain: false or ?draftchain=0 turns
    // it off; it keeps the embedding table on the GPU too, ~715 MB for the 27B, ~290 MB for the
    // 35B-A3B, or only its first draftVocab rows): the K draft steps of a speculative step go into
    // one command buffer; each step gathers the previous argmax's embedding on the GPU (bit-exact
    // with _embedRowF32) and has its own frame uniform, and one readback returns all K drafts.
    // With specFuse the same gather also fills the verify columns, so drafts + verify + head are
    // one submit. Skipped (per-submit drafts) when the table does not fit one storage binding.
    const ce = this.cpuEmbed;
    const egRows = Math.min(vocab, this.draftVocab || vocab), egPer = ce && ce.kind === "q4" ? 16 : 32;
    const egBytes = egRows * (dim / 32) * egPer, lim = device.limits || {};
    const egFits = egBytes <= (lim.maxStorageBufferBindingSize ?? Infinity) && egBytes <= (lim.maxBufferSize ?? Infinity);
    let egQs = null, egSc = null;
    if (draftChain && this.mtp && ce && (ce.kind === "q4" || ce.kind === "q8") && ce.qs && ce.scales && egFits) {
      // the table is an extra allocation on top of the model: if the device cannot hold it, fall back
      // to per-submit drafts instead of failing the load
      const nb = dim / 32;
      device.pushErrorScope("out-of-memory");
      try {
        egQs = this._buf(ce.qs.subarray(0, egRows * nb * egPer), GPUBufferUsage.STORAGE);
        egSc = this._buf(ce.scales.subarray(0, Math.ceil(egRows * nb / 2)), GPUBufferUsage.STORAGE);
      } catch { /* RangeError from mappedAtCreation: out of memory */ }
      const oom = await device.popErrorScope();
      if (oom || !egQs || !egSc) {
        egQs?.destroy(); egSc?.destroy(); egQs = egSc = null;
        console.warn(`draft chain off: no GPU memory for the ${(egBytes / 2 ** 20).toFixed(0)} MB embedding table`);
      }
    }
    if (egQs) {
      const rows = egRows, qs = egQs, sc = egSc;
      const egU = this._buf(new Uint32Array([dim, ce.kind === "q4" ? 0 : 1, rows, 0]), GPUBufferUsage.UNIFORM);
      this._eg = { qs, sc, u: egU, rows };
      this.bgEmbGather = this._bg(this.pipes.emb_gather, 1, [qs, sc, this.argBuf, this.mtp.emb, egU]);
      this.stepFrames = Array.from({ length: 8 }, () => device.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST }));
      this.bgCommonStep = this.stepFrames.map((f) => {
        const m = {};
        for (const [k2, p] of Object.entries(this.pipes)) m[k2] = this._bg(p, 0, [this.cfgBuf, f]);
        return m;
      });
      this.stageArgK = device.createBuffer({ size: 8 * 16, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
      this.draftChain = true;
    }
    // One-submit speculative verify (docs/research/kernels-next-2026-09.md D1): the batched trunk
    // and the batched LM head of a solo verify go into one command buffer (after the draft chain,
    // when it is on) with one readback of the logits (and the drafts); the trunk hiddens stay on
    // the GPU. Same kernels, same inputs, same order: bit-identical. engine.specFuse = false
    // restores the separate submits (trunk readback, head write-back) for A/B.
    this.specFuse = specFuse !== false;
    // A chain host's share of each lap in one submit (headAhead, _hostTrunkFused); hostFuse: false
    // keeps the separate submits for A/B (same bits)
    this.hostFuse = hostFuse !== false;
  }

  // ---- tensor-core prefill GEMM (prefillMath "sgmatrix", engine/wgsl/gemm_sgm.js) ----
  // Builds its own shader module (it needs `enable` directives), compiles one pipeline per GEMM
  // shape with the f32 GEMM's bind group layout, then checks one Q4 (and one Q8) kernel against a
  // CPU reference on random data. Any failure (feature missing, no usable MMA shape, compile
  // error, wrong numbers) leaves the f32 GEMM in place and records why in this.sgmWhy.
  async _initSgm(device, layout, layout0, adapterInfo, tune) {
    const no = (why) => { this.sgmWhy = why; return false; };
    const missing = SGM_FEATURES.filter((f) => !device.features?.has(f));
    if (missing.length) return no(`device lacks ${missing.join(", ")} (request them: prefillMathFeatures(adapter, "sgmatrix"); Chrome only, --enable-unsafe-webgpu)`);
    const info = adapterInfo ?? device.adapterInfo;
    const { TM, KB, PAD } = tune;
    const { cfg, why } = pickSgmConfig(info, { N: this.NC, TM, KB });
    if (!cfg) return no(why);
    if (why) console.warn("prefillMath sgmatrix: " + why);
    const smem = 2 * (TM * (32 * KB + PAD) + 32 * KB * this.NC);
    if (smem > device.limits.maxComputeWorkgroupStorageSize) return no(`TM ${TM} KB ${KB} needs ${smem} B of workgroup memory, device limit ${device.limits.maxComputeWorkgroupStorageSize}`);
    const ok = (dIn, S, q8) => { try { sgmPlan({ q8, dIn, S, N: this.NC, TM, KB, PAD, ...cfg }); return true; } catch { return false; } };
    const pairs = this._gemmPairs.filter(([dIn, S]) => ok(dIn, S, false)), pairs8 = this._gemm8Pairs.filter(([dIn, S]) => ok(dIn, S, true));
    if (!pairs.length && !pairs8.length) return no("no GEMM shape fits the MMA tiling");
    const names = [...pairs.map(([dIn, S]) => `gemm_sgm_q4_${dIn}_s${S}`), ...pairs8.map(([dIn, S]) => `gemm_sgm_q8_${dIn}_s${S}`)];
    const pipes = {};
    try {
      let mod = null, err = "";
      for (const syntax of SGM_SYNTAX) {   // current builtin spelling first, then the first Chrome releases'
        device.pushErrorScope("validation");
        const m = device.createShaderModule({ code: gemmSgmWGSL({ N: this.NC, TM, KB, PAD, cfg, pairs, pairs8, syntax }) });
        const errs = (await m.getCompilationInfo()).messages.filter((x) => x.type === "error");
        const scope = await device.popErrorScope();
        if (!errs.length && !scope) { mod = m; this.sgmSyntax = syntax; break; }
        err ||= `shader (${syntax}): ${errs[0] ? `L${errs[0].lineNum}: ${errs[0].message}` : scope.message}`.slice(0, 400);
      }
      if (!mod) return no(err);
      device.pushErrorScope("validation");
      await Promise.all(names.map(async (n) => { pipes[n] = await compilePipeline(device, { layout, compute: { module: mod, entryPoint: n } }); }));
      const pe = await device.popErrorScope();
      if (pe) return no("pipeline: " + pe.message.slice(0, 300));
    } catch (e) { return no("pipeline: " + String(e?.message || e).slice(0, 300)); }
    // self-test: dOut = 2 tiles, every split, against float64 with f16-rounded operands
    for (const [dIn, S, q8] of [pairs[0] && [...pairs[0], false], pairs8[0] && [...pairs8[0], true]].filter(Boolean)) {
      const rel = await this._sgmSelfTest(device, pipes[`gemm_sgm_${q8 ? "q8" : "q4"}_${dIn}_s${S}`], layout0, dIn, S, q8, TM);
      if (!(rel < 2e-3)) return no(`self-test ${q8 ? "q8" : "q4"} dIn ${dIn} S ${S}: rel L2 ${rel} vs CPU (want < 2e-3)`);
    }
    Object.assign(this.pipes, pipes);
    this._sgmPipes = new Set(names);
    this.sgmCfg = { ...cfg, TM, KB, PAD, syntax: this.sgmSyntax };
    return true;
  }
  async _sgmSelfTest(device, pipe, layout0, dIn, S, q8, TM) {
    const N = this.NC, dOut = 2 * TM, nb = dIn / 32, U = GPUBufferUsage;
    let seed = 12345; const rnd = () => ((seed = (seed * 1103515245 + 12345) >>> 0) / 4294967296);
    const qs = Uint32Array.from({ length: dOut * nb * (q8 ? 8 : 4) }, () => (rnd() * 4294967296) >>> 0);
    const sch = Uint16Array.from({ length: dOut * nb }, () => f32ToF16((0.005 + rnd() * 0.03) * (rnd() < 0.5 ? -1 : 1)));
    const xT = Float32Array.from({ length: dIn * N }, () => rnd() * 2 - 1);
    const h = Math.f16round ?? ((v) => f16ToF32(f32ToF16(v)));
    const mk = (data, usage) => { const b = device.createBuffer({ size: Math.max(16, Math.ceil(data.byteLength / 16) * 16), usage: usage | U.COPY_DST }); device.queue.writeBuffer(b, 0, data); return b; };
    const bQ = mk(qs, U.STORAGE), bS = mk(new Uint32Array(sch.buffer), U.STORAGE), bX = mk(xT, U.STORAGE);
    const bY = device.createBuffer({ size: S * N * dOut * 4, usage: U.STORAGE | U.COPY_SRC });
    const bSh = mk(new Uint32Array([dOut, dIn, 0, dOut]), U.UNIFORM), bC = mk(new Uint32Array(12), U.UNIFORM), bF = mk(new Uint32Array(4), U.UNIFORM);
    const st = device.createBuffer({ size: S * N * dOut * 4, usage: U.COPY_DST | U.MAP_READ });
    const enc = device.createCommandEncoder(), pass = enc.beginComputePass();
    pass.setPipeline(pipe);
    pass.setBindGroup(0, device.createBindGroup({ layout: layout0, entries: [bC, bF].map((b, i) => ({ binding: i, resource: { buffer: b } })) }));
    pass.setBindGroup(1, device.createBindGroup({ layout: pipe.getBindGroupLayout(1), entries: [bQ, bS, bX, bY, bSh].map((b, i) => ({ binding: i, resource: { buffer: b } })) }));
    pass.dispatchWorkgroups(Math.ceil(dOut / TM) * S);
    pass.end(); enc.copyBufferToBuffer(bY, 0, st, 0, st.size);
    device.queue.submit([enc.finish()]);
    await st.mapAsync(GPUMapMode.READ);
    const p = new Float32Array(st.getMappedRange().slice(0)); st.unmap();
    for (const b of [bQ, bS, bX, bY, bSh, bC, bF, st]) b.destroy();
    const w = (r, k) => {   // dequantized weight in f32, then rounded to f16 (what the kernel stages)
      const b = Math.floor(k / 32), kk = k % 32, sc = f16ToF32(sch[r * nb + b]);
      let q;
      if (!q8) { const wd = qs[(r * nb + b) * 4 + ((kk % 16) >> 2)]; q = ((wd >>> (8 * (kk % 4) + (kk >= 16 ? 4 : 0))) & 15) - 8; }
      else { const wd = qs[(r * nb + b) * 8 + (kk >> 2)]; q = (wd << (24 - 8 * (kk % 4))) >> 24; }
      return h(Math.fround(q * sc));
    };
    let num = 0, den = 0;
    for (let r = 0; r < dOut; r++) {
      const wr = Array.from({ length: dIn }, (_, k) => w(r, k));
      for (let c = 0; c < N; c++) {
        let ref = 0; for (let k = 0; k < dIn; k++) ref += wr[k] * h(xT[k * N + c]);
        let got = 0; for (let s = 0; s < S; s++) got += p[(s * N + c) * dOut + r];
        num += (got - ref) ** 2; den += ref * ref;
      }
    }
    return Math.sqrt(num / Math.max(den, 1e-30));
  }
  _shape(dOut, dIn) {
    const key = dOut + "," + dIn;
    if (!this._shapes[key])
      this._shapes[key] = this._buf(new Uint32Array([dOut, dIn, 0, 0]), GPUBufferUsage.UNIFORM);
    return this._shapes[key];
  }
  _shapeB(dOut, dIn, xs4, ys) {
    const key = "b" + dOut + "," + dIn + "," + xs4 + "," + ys;
    if (!this._shapes[key])
      this._shapes[key] = this._buf(new Uint32Array([dOut, dIn, xs4, ys]), GPUBufferUsage.UNIFORM);
    return this._shapes[key];
  }
  _buf(data, usage) {
    const src = ArrayBuffer.isView(data) ? data : new Uint8Array(data);
    const size = Math.ceil(src.byteLength / 4) * 4;
    const buf = this.device.createBuffer({ size, usage, mappedAtCreation: true });
    new Uint8Array(buf.getMappedRange()).set(new Uint8Array(src.buffer, src.byteOffset, src.byteLength));
    buf.unmap();
    return buf;
  }
  _bg(pipe, group, buffers) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(group),
      entries: buffers.map((b, i) => ({ binding: i, resource: Qwen35Engine._res(b) })),
    });
  }
  // ---- fuseProj: merged projection GEMVs ----
  // A view is a 256-byte aligned range of a buffer that binds like a buffer of its own.
  static _view(buffer, offset, size) { return { __view: true, buffer, offset, size }; }
  static _res(b) { return b && b.__view ? { buffer: b.buffer, offset: b.offset, size: b.size } : { buffer: b }; }
  // Row offsets of concatenated segments: every segment but the first starts on a multiple of 64
  // rows, so each segment's weights (Q4/Q8 rows are dIn/2 or dIn bytes plus dIn/16 bytes of f16
  // scales; f32 rows dIn*4) and its f32 outputs start 256-byte aligned for dIn % 64 == 0.
  static _segOffs(rows) {
    const offs = [];
    let o = 0;
    for (const r of rows) { offs.push(o); o = Math.ceil((o + r) / 64) * 64; }
    return offs;
  }
  // true if a GEMV with R rows per workgroup puts every row of every segment in the same position
  // of the same kind of workgroup (all-rows-live "full" branch vs the guarded tail branch) as it
  // had in its own GEMV: segments start on multiples of R, and only the last one may end in a
  // partial group (which stays the matrix's last group). The row's arithmetic never depends on R.
  static _rowsKeep(rows, R) {
    const offs = Qwen35Engine._segOffs(rows);
    return offs.every((o) => o % R === 0) && rows.slice(0, -1).every((r) => r % R === 0);
  }
  // Row-concatenate GPU weight entries of one format and input width into one matrix (segment
  // starts from _segOffs, the padding rows stay zero). parts: [{ src: the loader's weight entry,
  // w: its GPU entry, rows }]. Returns { w: merged entry, parts: per-part view entries, rows, offs }
  // or null when the formats differ or a source cannot be copied; the caller then keeps the
  // separate tensors. The old buffers are released and each loader entry's .gpu now points at
  // its view, so a second engine built from the same weights still finds them.
  _fuseW(parts, dIn) {
    const kind = parts[0].w?.kind;
    if (!["q4", "q8", "f32"].includes(kind) || !parts.every((p) => p.w && p.w.kind === kind) || dIn % 64) return null;
    const rb = kind === "q4" ? [dIn / 2, dIn / 16] : kind === "q8" ? [dIn, dIn / 16] : [dIn * 4];   // bytes per row
    const srcs = (w) => kind === "f32" ? [w.buf] : [w.qs, w.sc];
    const whole = (b) => b.__view ? b : { buffer: b, offset: 0, size: b.size };
    for (const p of parts) {
      const ss = srcs(p.w);
      if (ss.some((b, j) => { const r = whole(b); return !r.buffer || r.size < p.rows * rb[j] || r.offset % 4 || !(r.buffer.usage & GPUBufferUsage.COPY_SRC); })) return null;
    }
    const offs = Qwen35Engine._segOffs(parts.map((p) => p.rows));
    const rows = offs[offs.length - 1] + parts[parts.length - 1].rows;
    const lim = this.device.limits;
    if (rb.some((x) => rows * x > lim.maxStorageBufferBindingSize || rows * x > lim.maxBufferSize)) return null;
    const U = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST;
    const dst = rb.map((x) => this.device.createBuffer({ size: Math.ceil(rows * x / 4) * 4, usage: U }));
    const enc = this.device.createCommandEncoder();
    parts.forEach((p, i) => srcs(p.w).forEach((b, j) => {
      const r = whole(b);
      enc.copyBufferToBuffer(r.buffer, r.offset, dst[j], offs[i] * rb[j], p.rows * rb[j]);
    }));
    this.device.queue.submit([enc.finish()]);
    const entry = (bs) => kind === "f32" ? { kind, buf: bs[0] } : { kind, qs: bs[0], sc: bs[1] };
    const views = parts.map((p, i) => entry(rb.map((x, j) => Qwen35Engine._view(dst[j], offs[i] * x, p.rows * x))));
    parts.forEach((p, i) => {
      for (const b of srcs(p.w)) if (!b.__view) b.destroy();   // freed once the copy above has run
      if (p.src && typeof p.src === "object") p.src.gpu = views[i];
    });
    return { w: entry(dst), parts: views, rows, offs, lens: parts.map((p) => p.rows) };
  }
  _bg2(pipe, resources) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(1),
      entries: resources.map((r, i) => ({ binding: i, resource: r })),
    });
  }
  _d(pass, name, bg, threads, wg = 64) {
    if (this.skip && this.skip.has(name)) return;   // profiling aid (bench_breakdown)
    pass.setPipeline(this.pipes[name]);
    pass.setBindGroup(0, (this._common || this.bgCommonFor)[name]);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(Math.ceil(threads / wg));
  }
  _dop(pass, op, nCols = 0) {
    if (this.skip && this.skip.has(op.pipe)) return;
    if (op.r16 && this.prefillMath === "f16") op = op.r16;   // GEMM activation transposes (prefillMath f16)
    // full-width prefill passes go through the row-stationary GEMM; anything
    // narrower (decode, speculative verify, prompt tail) uses the GEMV ladder
    if (this._gemmAt(op, nCols)) {
      const g = op.gemm, z = (this._gz ^= 1), pm = this.prefillMath;
      const v = (pm === "sgmatrix" && g.sgm) || (pm === "f16" && g.r16) || g;   // same grid, partials and reduce
      this._d3(pass, v.pipe, v.bg[z], v.wgs ?? g.wgs);
      this._d3(pass, g.red, g.redBg[z], g.redWgs);
      return;
    }
    const w = this.b4 === false ? this.NC : (nCols > 0 ? nCols : this.NC);
    const W = w <= 4 && op.pipe4 ? 4 : w <= 8 && op.pipe8 ? 8 : 0;
    this._d3(pass, W ? op[`pipe${W}`] : op.pipe, W ? op[`bg${W}`] : op.bg, W ? (op[`wgs${W}`] ?? op.wgs) : op.wgs);
  }
  // true if _dop would run this op on the prefill GEMM at this width (mirrors the test in _dop)
  _gemmAt(op, nCols) { return !!(op && op.gemm && nCols === this.NC && this.gemm !== false && !(op.gemm.q8 && this.gemm8 === false)); }
  // rows per workgroup for a W-column batched kernel; mirrors rowsFor() in coop.js
  _rowsFor(W) { return Math.max(1, Math.min(8, Math.round(this.coopRowsB * this.NC / W))); }
  _d3(pass, pipe, bg, wgs) {
    pass.setPipeline(this.pipes[pipe]);
    pass.setBindGroup(0, (this._common || this.bgCommonFor)[pipe]);
    pass.setBindGroup(1, bg);
    if (wgs > 32768) pass.dispatchWorkgroups(32768, Math.ceil(wgs / 32768)); else pass.dispatchWorkgroups(wgs);   // > 65535 per dimension is silently dropped
  }
  _dInd(pass, name, bg, buf, off) {   // indirect launch (workgroup counts written by an earlier dispatch)
    if (this.skip && this.skip.has(name)) return;
    pass.setPipeline(this.pipes[name]);
    pass.setBindGroup(0, (this._common || this.bgCommonFor)[name]);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroupsIndirect(buf, off);
  }
  _dxyz(pass, name, bg, x, y, z) {
    if (this.skip && this.skip.has(name)) return;   // profiling aid (bench_breakdown)
    pass.setPipeline(this.pipes[name]);
    pass.setBindGroup(0, (this._common || this.bgCommonFor)[name]);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(x, y, z);
  }
  _setFrame(pos, seqLen) {
    this.device.queue.writeBuffer(this.frameBuf, 0, new Uint32Array([pos, seqLen]));
  }

  _encodeLayer(enc, i) { this._encodeLayerR(enc, this.layers[i], this.pos); }
  _encodeLayerR(enc, L, pos) {
    const D = this.dims;
    const seqLen = pos + 1;
    const LF = this.layerFuse;
    if (L.isFull) {
      const v2 = this.flash && this.attnDecode === "v2" && !!L.bgDec;   // split-K decode attention (attn_dec + attn_dec_combine)
      const lfC = LF.comb && this.flash && !this.kvQ8 && !!(v2 ? L.bgDecCG : L.bgCombineG);   // sigmoid_mul in attn_combine / attn_dec_combine
      const lfA = lfC && LF.kv && this.attnGlue && !!L.bgGlueKv;         // kv_store in attn_glue (its gate copy is skipped: needs lfC)
      {
        const p = this._pp || enc.beginComputePass();
        if (LF.norm && this.fuseProj && L.mvKVn) { this._dop(p, L.mvKVn); this._dop(p, L.mvQ); }   // norm in the [k | v] GEMV, then q
        else {
        this._d(p, "rmsnorm", L.bgNorm1, 256, 256);
        this._dop(p, L.mvQ);
        if (this.fuseProj && L.mvKV) this._dop(p, L.mvKV);
        else { this._dop(p, L.mvK); this._dop(p, L.mvV); }
        }
        if (lfA) this._d(p, "attn_glue_kv", L.bgGlueKv, (D.nH + D.nKV) * 64);
        else if (this.attnGlue) this._d(p, "attn_glue", L.bgGlue, (D.nH + D.nKV) * 64);
        else {
          this._d(p, "qsplit", L.bgQsplit, D.nH * D.hd);
          this._d(p, "head_norm", L.bgQNorm, D.nH, 32);
          this._d(p, "head_norm", L.bgKNorm, D.nKV, 32);
          this._d(p, "rope_part", L.bgRopeQ, D.nH * D.nRot / 2);
          this._d(p, "rope_part", L.bgRopeK, D.nKV * D.nRot / 2);
        }
        if (p !== this._pp) p.end();
      }
      if (!this.flash) {
        const k = Qwen35Engine._res(this.k), v = Qwen35Engine._res(this.v);
        enc.copyBufferToBuffer(k.buffer, k.offset || 0, L.kCache, pos * D.kvDim * 4, D.kvDim * 4);
        enc.copyBufferToBuffer(v.buffer, v.offset || 0, L.vCache, pos * D.kvDim * 4, D.kvDim * 4);
      }
      {
        const p = this._pp || enc.beginComputePass();
        if (lfC) {
          if (!lfA) this._dxyz(p, this.ksPipe, L.bgKvStore, Math.ceil(D.kvDim / 2 / 64), 1, 1);
          if (v2) {
            this._dxyz(p, "attn_dec", L.bgDec, decSplits(seqLen, this.adCfg.S), 1, D.nKVa);
            this._dxyz(p, "attn_dec_combine_g", L.bgDecCG, D.nH * DEC_CQ, 1, 1);
          } else {
            this._dxyz(p, this.faPipe, L.bgFlash, Math.ceil(seqLen / this.faSplit), 1, D.nKVa);
            this._dxyz(p, "attn_combine_g", L.bgCombineG, D.nH, 1, 1);
          }
        } else if (this.flash) {
          this._dxyz(p, this.ksPipe, L.bgKvStore, Math.ceil(D.kvDim / (this.kvQ8 ? 32 : 2) / 64), 1, 1);
          if (this.attnDecode === "v2" && L.bgDec) {
            this._dxyz(p, "attn_dec", L.bgDec, decSplits(seqLen, this.adCfg.S), 1, D.nKVa);
            this._dxyz(p, "attn_dec_combine", L.bgDecC, D.nH * DEC_CQ, 1, 1);
          } else {
            this._dxyz(p, this.faPipe, L.bgFlash, Math.ceil(seqLen / this.faSplit), 1, D.nKVa);
            this._dxyz(p, "attn_combine", L.bgCombine, D.nH, 1, 1);
          }
        } else {
          this._d(p, "attn_scores", L.bgScores, D.nH * seqLen);
          if (this.softmaxWG) this._d(p, "attn_softmax_wg", L.bgSoftmax, D.nH * 256, 256);
          else this._d(p, "attn_softmax", L.bgSoftmax, D.nH, 1);
          this._d(p, "attn_out", L.bgAttnOut, D.qDim);
        }
        if (!lfC) this._d(p, "sigmoid_mul", L.bgSigMul, D.qDim);
        this._dop(p, L.mvO);
        if (!L.mvO.acc) this._d(p, "add_res", this.bgAddTmp, D.dim);
        if (p !== this._pp) p.end();
      }
    } else {
      const p = this._pp || enc.beginComputePass();
      const nba = this.fuseProj && L.bgNba;   // input norm + beta / alpha, then [qkv | z] on its xn
      if (nba) this._dxyz(p, "dn_nba", L.bgNba, Math.ceil(L.fBA.rows / 4), 1, 1);
      else this._d(p, "rmsnorm", L.bgNorm1, 256, 256);
      const cv = LF.conv && !!L.mvQKVcv && !(this.fuseProj && L.mvQZ && !L.mvQZcv);   // conv in the qkv GEMV's epilogue
      if (this.fuseProj && L.mvQZ) this._dop(p, cv ? L.mvQZcv : L.mvQZ);
      else { this._dop(p, cv ? L.mvQKVcv : L.mvQKV); this._dop(p, L.mvZ); }
      if (!nba) {
        if (this.fuseProj && L.mvBA) this._dop(p, L.mvBA);
        else { this._dop(p, L.mvBeta); this._dop(p, L.mvAlpha); }
      }
      if (!cv) this._d(p, "dn_conv", L.bgConv, D.convDim);
      if (LF.dn && this.dnFuse && L.bgDeltaGnp) this._d(p, "dn_delta_gnp", L.bgDeltaGnp, D.nVH * 128, 128);   // pre + delta + gated norm
      else {
      this._d(p, "dn_pre", L.bgPre, 128, 128);      // gates + L2(q,k) fused
      if (this.dnFuse) this._d(p, "dn_delta_gn", L.bgDeltaGn, D.nVH * 128, 128);
      else {
        this._d(p, "dn_delta", L.bgDelta, D.nVH * 128, 128);
        this._d(p, "dn_gatenorm", L.bgGateNorm, D.nVH * 128, 128);
      }
      }
      this._dop(p, L.mvOut);
      if (!L.mvOut.acc) this._d(p, "add_res", this.bgAddTmp, D.dim);
      if (p !== this._pp) p.end();
    }
    this._encodeFFN(enc, L);
  }

  // The FFN half of a layer (post-attention norm, then the dense or MoE FFN, residual added).
  _encodeFFN(enc, L) {
    const D = this.dims;
    const p = this._pp || enc.beginComputePass();
    if (!(L.fused && this.moe.nrt)) this._d(p, "rmsnorm", L.bgNorm2, 256, 256);
    if (L.fused) {   // (norm +) router (+ shared gate) GEMV, route, gate/up over K + 1 slots, down + combine
      const { KS, hs, gusRows, dncRows } = this.moe;
      if (this.moe.nrt) this._dxyz(p, "moe_nrt", L.bgNrt, Math.ceil((this.moe.nExp + 1) / this.moe.nrt.ROWS), 1, 1);
      else this._dop(p, L.mvRouter);
      this._dxyz(p, "moe_route", L.bgRoute, 1, 1, 1);
      if (L.off) {   // offloaded: the experts run after _cutResolve
        const m = this._ffnMode;
        if (m === "spec") {   // cutSpec: slot ids from the GPU table (moe_remap), no cut; checked at the next cut
          p.setPipeline(this._spec.pipe); p.setBindGroup(0, L.bgRemap); p.dispatchWorkgroups(1);
        }
        if (m === "spec" || m === "redo") {   // (redo: the slot ids the CPU wrote, after a rollback to this layer)
          this._dxyz(p, L.gusPipe, L.bgGusP, Math.ceil(hs / gusRows), KS, 1);
          this._dxyz(p, L.dncPipe, L.bgDncP, Math.ceil(D.dim / dncRows), 1, 1);
          if (p !== this._pp) p.end();
          return;
        }
        this._cutAt(enc, p, L, 1, L.selBuf, null);
        return;
      }
      this._dxyz(p, L.gusPipe, L.bgGus, Math.ceil(hs / gusRows), KS, 1);
      this._dxyz(p, L.dncPipe, L.bgDnc, Math.ceil(D.dim / dncRows), 1, 1);
      if (p !== this._pp) p.end();
      return;
    }
    if (L.moe) {
      const { K, inter: ei } = this.moe;
      const rs = this.fuseProj && L.mvRS;   // router + shared-expert gate in one GEMV
      this._dop(p, rs || L.mvRouter);
      this._dxyz(p, "moe_router", L.bgRouter, 1, 1, 1);
      this._dxyz(p, L.guPipe, L.bgGu, Math.ceil(ei / this.moeK.gu.rows), K, 1);
      this._dxyz(p, L.dnPipe, L.bgDn, Math.ceil(D.dim / this.moeK.dn.rows), K, 1);
      if (L.shared) {
        if (L.gu) this._dop(p, L.gu);
        else { this._dop(p, L.mvGate); this._dop(p, L.mvUp); this._d(p, "silu_mul", this.bgSilu, D.inter); }
        this._dop(p, L.mvShDown);
        if (!rs) this._dop(p, L.mvShRouter);
      }
      this._dxyz(p, "moe_combine", L.bgMoeCombine, Math.ceil(D.dim / 64), 1, 1);
      if (p !== this._pp) p.end();
      return;
    }
    if (L.gu) this._dop(p, L.gu);
    else {
      this._dop(p, L.mvGate);
      this._dop(p, L.mvUp);
      this._d(p, "silu_mul", this.bgSilu, D.inter);
    }
    this._dop(p, L.mvDown);
    if (!L.mvDown.acc) this._d(p, "add_res", this.bgAddTmp, D.dim);
    if (p !== this._pp) p.end();
  }
  // ---- expert offload: the cut at an offloaded layer's moe_route (see engine/expert_store.js) ----
  // _cutAt ends the pass, copies the route's selection (nCols x KS ids) to the staging buffer and records the cut;
  // the encoding caller then runs _cutResolve before anything else: it submits what was encoded, reads the ids back,
  // lets the store plan the copies (misses into the pool, or the experts into the region when they outnumber the
  // pool), writes the slot ids and encodes the layer's expert kernels into a new encoder, which it returns.
  // M: the batched pass's bind groups (null: the one-token pass). A shared compute pass (layerFuse.pass _pp, the
  // verify pass _ppB) is ended here and reopened in the new encoder.
  _cutAt(enc, p, L, nCols, selBuf, M) {
    if (this._cut) throw new Error("expert offload: a cut was not resolved");
    const pp = p === this._pp, ppB = p === this._ppB;
    p.end();
    if (pp) this._pp = null;
    if (ppB) this._ppB = null;
    enc.copyBufferToBuffer(selBuf, 0, this.offStage, 0, nCols * this.moe.KS * 4);
    this._cut = { L, nCols, M, pp, ppB };
  }
  // The gate (offGate): from a cut's submit until the call that made it has submitted the rest, the GPU has run only
  // part of that call's work, and its scratch (this.x / xn, the batch columns, the frame uniforms) is mid-use. Work
  // the room queues meanwhile without awaiting a trunk call (the draft fill: setHidden + mtpRun, _mtpFillBatch on
  // B.x, room/pipeline.js fillDrafts) would land between the parts and overwrite it: an offloaded chain host answered
  // garbage. Such work waits for offGate() to be null. A resident engine never opens it (no cuts): unchanged.
  _gateOpen() {
    this._offWaits = (this._offWaits || 0) + 1;
    if (!this._offGateP) { let res; this._offGateP = { p: new Promise((r) => (res = r)), res }; }
  }
  _gateDone() {
    this._offWaits--;
    // closes once the caller has gone on past its last cut: its encoding between two cuts is synchronous, so by the
    // next macrotask it is either waiting on another cut (offWaits > 0) or has submitted everything
    setTimeout(() => { if (!this._offWaits && !this._cut && !this._cutPend && this._offGateP) { const g = this._offGateP; this._offGateP = null; g.res(); } }, 0);
  }
  // null when nothing of an offloaded call is in flight, else a promise that resolves when that may have changed (check again)
  offGate() { return this._offGateP ? this._offGateP.p : null; }
  // cutProf (tests/bench/cut_prof.js): { n, enc, submit, wait, plan, last } ms totals per phase of the cuts: enc = the
  // CPU encoding between cuts (a layer's attention etc.), submit = finish + submit, wait = awaiting the map of the
  // selection, plan = reading it, plan(), the slot ids and the copies / expert kernels' encoding
  //
  // pipe (cutPipe, opt-in: no gain measured beyond noise; the one-token decode of _encodeForwardOff): the cut submits, then encodes the layer's expert kernels
  // on the slot pool at once and returns without waiting; the caller encodes the next layer while the GPU runs this
  // part and the selection comes back. The next cut (or _cutFlush) lands this one first (_cutLand: the map, plan(),
  // the misses' copies in a command buffer of their own and the slot ids), then submits its part, whose first kernels
  // are those experts. The GPU gets the same commands in the same order (copies, slot ids, experts, next layer); only
  // the CPU's encoding moves off the critical path, into the time the GPU and the readback take anyway. It needs the
  // pool path to be certain before the selection is known: nCols * K <= P (always for one token: P >= K).
  // map kick (engine/map_kick.js) on an offloaded readback: mapKick true / false, else on in Chrome only. Never on a
  // resident engine's readbacks (a review measured resident decode in Chrome 2-6 tok/s slower with it).
  _kick() { return !!this.experts && (this.mapKick ?? KICK_DEFAULT); }
  // A disk layer's read (engine/expert_store.js prepare / loadExpertsAsync / loadLayerAsync), awaited with the gate
  // open. One that fails leaves the step half run: the layers before it wrote their DeltaNet state, conv windows and
  // K/V rows, but pos did not move, so running the token again would apply them twice. The engine is marked failed:
  // every call that runs layers refuses until reset() (FAILED_REFUSE; as engine/qwen4exp.js's failed step), and the
  // offloaded call's state (a pending cut, the FFN mode, mapped staging buffers) is cleared as _offAbort does.
  async _diskWait(fn) {
    this._gateOpen();
    try { return await fn(); }
    catch (e) { this._failed = e; await this._offAbort(); throw e; }
    finally { this._gateDone(); }
  }
  async _cutResolve(enc, pipe = false) {
    const c = this._cut, q = this.device.queue, KS = this.moe.KS, bytes = c.nCols * KS * 4, st = this.offStage, P = this.cutProf;
    this._cut = null;
    const t0 = P ? performance.now() : 0;
    if (P && P.last) P.enc += t0 - P.last;
    if (this._cutPend) await this._cutLand();   // the previous cut's copies and slot ids before this part
    const t1 = P ? performance.now() : 0;
    q.submit([enc.finish()]);
    this._gateOpen();
    const mapped = mapRead(this.device, st, 0, bytes, this._kick());
    const { hs, gusRows, dncRows } = this.moe, L = c.L;
    if (pipe && !c.M && c.nCols * this.moe.K <= this.experts.P) {
      this._cutPend = { c, mapped, bytes };
      const e2 = this.device.createCommandEncoder(), p = e2.beginComputePass();
      if (c.pp) this._pp = p;
      if (c.ppB) this._ppB = p;
      this._dxyz(p, L.gusPipe, L.bgGusP, Math.ceil(hs / gusRows), KS, 1);
      this._dxyz(p, L.dncPipe, L.bgDncP, Math.ceil(this.dims.dim / dncRows), 1, 1);
      if (!c.pp && !c.ppB) p.end();
      if (P) { const t2 = performance.now(); P.n++; P.submit += t2 - t1; P.last = t2; }
      return e2;
    }
    const t2 = P ? performance.now() : 0;
    try { await mapped; } finally { this._gateDone(); }
    const t3 = P ? performance.now() : 0;
    const sel = new Uint32Array(st.getMappedRange(0, bytes)).slice();
    st.unmap();
    // a disk layer (engine/expert_store.js): read the experts plan() will put, before it plans (the cut's encoder is
    // submitted, so plan()'s writeBuffers land after it and before e2)
    if (this.experts.isDisk(c.L.offLayer)) await this._diskWait(() => this.experts.prepare(c.L.offLayer, sel, c.nCols, KS));
    const e2 = this.device.createCommandEncoder();
    const r = this.experts.plan(e2, c.L.offLayer, sel, c.nCols, KS);
    if (r.pool) q.writeBuffer(c.M ? this.B.offSel : this.offSel, 0, r.remap);   // lands before e2's kernels (queue order)
    const p = e2.beginComputePass();
    if (c.pp) this._pp = p;
    if (c.ppB) this._ppB = p;
    if (c.M) {
      this._dMC(p, L.gusPipe, r.pool ? c.M.gusP : c.M.gus, Math.ceil(hs / gusRows) * 64, 64, c.nCols * KS);
      this._dMC(p, L.dncPipe, r.pool ? c.M.dncP : c.M.dnc, Math.ceil(this.dims.dim / dncRows) * 64, 64, c.nCols);
    } else {
      this._dxyz(p, L.gusPipe, r.pool ? L.bgGusP : L.bgGus, Math.ceil(hs / gusRows), KS, 1);
      this._dxyz(p, L.dncPipe, r.pool ? L.bgDncP : L.bgDnc, Math.ceil(this.dims.dim / dncRows), 1, 1);
    }
    if (!c.pp && !c.ppB) p.end();
    if (P) {
      const t4 = performance.now();
      P.n++; P.submit += t2 - t1; P.wait += t3 - t2; P.plan += t4 - t3; P.last = t4;
    }
    return e2;
  }
  // a pipelined cut's second half: its selection, plan(), the misses' copies (submitted now, ahead of the part that
  // runs its experts, which the caller submits next) and its slot ids (queue.writeBuffer: also ahead of that part)
  async _cutLand() {
    const { c, mapped, bytes } = this._cutPend, st = this.offStage, q = this.device.queue, P = this.cutProf;
    const t0 = P ? performance.now() : 0;
    try { await mapped; } finally { this._cutPend = null; this._gateDone(); }
    const t1 = P ? performance.now() : 0;
    const sel = new Uint32Array(st.getMappedRange(0, bytes)).slice();
    st.unmap();
    // a disk layer: its misses read before plan() (nothing unsubmitted holds a T1 slot here: plan()'s copies go in e)
    if (this.experts.isDisk(c.L.offLayer)) await this._diskWait(() => this.experts.prepare(c.L.offLayer, sel, c.nCols, this.moe.KS));
    const e = this.device.createCommandEncoder(), b0 = this.experts.stats.bytes;
    const r = this.experts.plan(e, c.L.offLayer, sel, c.nCols, this.moe.KS);
    if (!r.pool) throw new Error("expert offload: a pipelined cut left the slot pool");   // (nCols * K <= P: cannot)
    if (this.experts.stats.bytes !== b0) q.submit([e.finish()]);
    q.writeBuffer(this.offSel, 0, r.remap);
    if (P) { P.wait += t1 - t0; P.plan += performance.now() - t1; }
  }
  // the end of a pipelined pass: its last cut landed before the caller submits the rest
  async _cutFlush(enc) {
    if (this._cutPend) {
      const P = this.cutProf, t0 = P ? performance.now() : 0;
      if (P && P.last) P.enc += t0 - P.last;
      await this._cutLand();
      if (P) P.last = 0;
    }
    return enc;
  }
  // A prefill frame of W tokens on an offloaded layer (grouped prefill, after the routing of all W): its experts into
  // the store's region. A frame whose picks can cover the layer (W * K >= nExp; 512-token frames touch nearly every
  // expert) copies the whole layer with no readback; a shorter one reads its routing back and copies only the
  // distinct experts it chose. -> the encoder to continue in
  async _regionLoad(enc, L, selBuf, W) {
    const { K, KS, nExp } = this.moe, st = this.experts;
    if (W * K >= nExp) {
      if (!st.isDisk(L.offLayer)) { st.loadLayer(enc, L.offLayer); return enc; }
      return this._regionLoadDisk(enc, L.offLayer);
    }
    const bytes = W * KS * 4, q = this.device.queue;
    if (!this._offStageG || this._offStageG.size < bytes) { this._offStageG?.destroy(); this._offStageG = this.device.createBuffer({ size: bytes, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST }); }
    const sg = this._offStageG;
    enc.copyBufferToBuffer(selBuf, 0, sg, 0, bytes);
    q.submit([enc.finish()]);
    this._gateOpen();
    try { await mapRead(this.device, sg, 0, bytes, this._kick()); } finally { this._gateDone(); }
    const sel = new Uint32Array(sg.getMappedRange(0, bytes)).slice();
    sg.unmap();
    if (st.isDisk(L.offLayer)) {   // read after the submit above: the writeBuffers land before the next encoder
      await this._diskWait(() => st.loadExpertsAsync(L.offLayer, sel, W, KS));
      return this.device.createCommandEncoder();
    }
    const e2 = this.device.createCommandEncoder();
    st.loadExperts(e2, L.offLayer, sel, W, KS);
    return e2;
  }
  // A disk layer's whole expert set into the region: submit what was encoded (it may still read the region), then
  // read the layer and write it in queue order. -> a fresh encoder. A no-op when the region already holds it.
  async _regionLoadDisk(enc, layer) {
    const st = this.experts;
    if (st.region.layer === layer) return enc;
    this.device.queue.submit([enc.finish()]);
    await this._diskWait(() => st.loadLayerAsync(layer));
    return this.device.createCommandEncoder();
  }
  // Debug / test hook: run only layer i's FFN block on a given residual x (one token).
  async ffnOnly(i, xIn) {
    this.device.queue.writeBuffer(this.x, 0, xIn);
    let enc = this.device.createCommandEncoder();
    this._encodeFFN(enc, i < this.layers.length ? this.layers[i] : this.mtpLayer);
    if (this._cut) enc = await this._cutResolve(enc);
    this.device.queue.submit([enc.finish()]);
    return this._readback(this.x, this.stageX, this.dims.dim);
  }

  _embedRowF32(id) {
    const { dim } = this.dims;
    const e = this.cpuEmbed;
    if (e.kind === "f32") return e.data.subarray(id * dim, (id + 1) * dim);
    const nb = dim / 32;
    const out = new Float32Array(dim);
    const rowB = id * nb;
    for (let b = 0; b < nb; b++) {
      const si = rowB + b;
      const s = f16ToF32((e.scales[si >> 1] >>> ((si & 1) * 16)) & 0xFFFF);
      if (e.kind === "q4") {
        const qBase = (rowB + b) * 16;
        for (let j = 0; j < 16; j++) {
          const q = e.qs[qBase + j];
          out[b * 32 + j] = s * ((q & 0xF) - 8);
          out[b * 32 + j + 16] = s * ((q >> 4) - 8);
        }
      } else {
        const qBase = (rowB + b) * 32;
        for (let i = 0; i < 32; i++) {
          const q = e.qs[qBase + i];
          out[b * 32 + i] = s * (q > 127 ? q - 256 : q);
        }
      }
    }
    return out;
  }

  async _readback(srcBuf, stageBuf, n) {
    const enc = this.device.createCommandEncoder();
    enc.copyBufferToBuffer(srcBuf, 0, stageBuf, 0, n * 4);
    this.device.queue.submit([enc.finish()]);
    await stageBuf.mapAsync(GPUMapMode.READ);
    const out = Float32Array.from(new Float32Array(stageBuf.getMappedRange(), 0, n));
    stageBuf.unmap();
    return out;
  }

  // ---- batched prefill (NC prompt tokens per pass; the room uses 16) ----
  _bg2g0(pipe, resources) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(0),
      entries: resources.map((r, i) => ({ binding: i, resource: r })),
    });
  }
  // attn_flash_tile + attn_combine_tile from their own module (engine/wgsl/attn_tile.js), so a compile
  // problem there only turns the option off (with a warning) instead of failing the engine.
  async _initAttnTile(device, layout0, bufType) {
    const C = GPUShaderStage.COMPUTE;
    const specs = { attn_flash_tile: ["ro", "ro", "ro", "rw", "rw", "u"], attn_combine_tile: ["ro", "ro", "rw", "u"] };
    const pipes = {};
    let fail = null;
    device.pushErrorScope("validation");
    try {
      const module = device.createShaderModule({ code: attnTileWGSL(this.attnPTCfg) });
      for (const [name, spec] of Object.entries(specs)) {
        const layout1 = device.createBindGroupLayout({ entries: spec.map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
        pipes[name] = await compilePipeline(device, {
          layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module, entryPoint: name } });
      }
    } catch (e) { fail = e; }
    const err = await device.popErrorScope();   // always popped: the scope must not leak into later calls
    fail ||= err;
    if (fail) {
      console.warn("attnPrefillTile disabled:", String(fail.message || fail).slice(0, 300));
      this.attnPTCfg = null;
    } else Object.assign(this.pipes, pipes);
  }
  // attn_dec + attn_dec_combine (engine/wgsl/attn_dec.js) from their own module, like the prefill tile:
  // a compile problem only leaves attnDecode on "v1" (with a warning)
  async _initAttnDec(device, layout0, bufType) {
    const C = GPUShaderStage.COMPUTE;
    const specs = { attn_dec: ["ro", "ro", "ro", "rw", "rw", "u"], attn_dec_combine: ["ro", "ro", "rw", "u"] };
    const pipes = {};
    let fail = null;
    device.pushErrorScope("validation");
    try {
      const module = device.createShaderModule({ code: attnDecWGSL(this.adCfg) });
      for (const [name, spec] of Object.entries(specs)) {
        const layout1 = device.createBindGroupLayout({ entries: spec.map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
        pipes[name] = await device.createComputePipelineAsync({
          layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module, entryPoint: name } });
      }
    } catch (e) { fail = e; }
    const err = await device.popErrorScope();
    fail ||= err;
    if (fail) {
      console.warn("attnDecode v2 disabled:", String(fail.message || fail).slice(0, 300));
      this.adCfg = null;
    } else Object.assign(this.pipes, pipes);
    // layerFuse.comb under v2: attn_dec_combine_g (attn_dec_combine + sigmoid_mul) in a module of its own and
    // optional like the other layerFuse kernels: a compile failure (FXC, ...) only leaves the separate sigmoid_mul
    if (!fail && this.layerFuse.comb && !this.kvQ8) {
      device.pushErrorScope("validation");
      let gf = null;
      try {
        const module = device.createShaderModule({ code: attnDecCombineGWSL(this.adCfg) });
        const layout1 = device.createBindGroupLayout({ entries: ["ro", "ro", "rw", "ro", "u"].map((t, i) => ({ binding: i, visibility: C, buffer: { type: bufType[t] } })) });
        this.pipes.attn_dec_combine_g = await compilePipeline(device, {
          layout: device.createPipelineLayout({ bindGroupLayouts: [layout0, layout1] }), compute: { module, entryPoint: "attn_dec_combine_g" } });
      } catch (e) { gf = e; }
      gf ||= await device.popErrorScope();
      if (gf) {
        delete this.pipes.attn_dec_combine_g;
        console.warn("layerFuse.comb under attnDecode v2 off:", String(gf.message || gf).slice(0, 300));
      }
    }
  }
  _bg2res(pipe, resources) {
    return this.device.createBindGroup({
      layout: pipe.getBindGroupLayout(1),
      entries: resources.map((r, i) => ({ binding: i, resource: r })),
    });
  }
  _dMC(pass, name, bg, threads, wg, ny, nz = 1) {
    if (this.skip && this.skip.has(name)) return;   // profiling aid (bench_breakdown)
    pass.setPipeline(this.pipes[name]);
    pass.setBindGroup(0, (this._g0 || this._mcCommon || this.bgCommonB[0])[name]);   // _g0: a grouped-prefill sub-pass's frame; _mcCommon: a wide-prefill sub-batch's
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(Math.ceil(threads / wg), ny, nz);
  }
  // silu_mul_w over nCols columns of the batched g / u (it reads no group-0 binding)
  _dSiluMC(pass, bg, nCols) {
    if (this.skip && this.skip.has("silu_mul_w")) return;
    pass.setPipeline(this.pipes.silu_mul_w);
    pass.setBindGroup(0, this.bgCommonFor.silu_mul_w);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(Math.ceil(this.dims.inter / 64), nCols);
  }
  _dCol(pass, name, col, bg, threads, wg = 64) {
    if (this.skip && this.skip.has(name)) return;
    pass.setPipeline(this.pipes[name]);
    pass.setBindGroup(0, this.bgCommonB[col][name] || this.bgCommonFor[name]);
    pass.setBindGroup(1, bg);
    pass.dispatchWorkgroups(Math.ceil(threads / wg));
  }

  _initBatch() {
    const D = this.dims;
    const dev = this.device;
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC;
    const NC = this.NC, cix = Array.from({ length: NC }, (_, c) => c);
    const al = (n) => Math.ceil(n * 4 / 256) * 256;
    const mkB = (n) => ({ buf: dev.createBuffer({ size: NC * al(n), usage: S }), stride: al(n), n });
    // fuseProj: a merged GEMV writes the column-major [seg0 | seg1] rows of one buffer; each
    // segment is { buf, stride, n, off } (off: its 256-byte aligned start inside every column)
    const segB = (rows) => {
      const offs = Qwen35Engine._segOffs(rows), m = mkB(offs[offs.length - 1] + rows[rows.length - 1]);
      return rows.map((n, i) => ({ buf: m.buf, stride: m.stride, n, off: offs[i] * 4 }));
    };
    const fz = this.fuseProjOn;
    const [qkv, z] = fz ? segB([D.convDim, D.dInner]) : [mkB(D.convDim), mkB(D.dInner)];
    const [betaRaw, alpha] = fz ? segB([D.nVH, D.nVH]) : [mkB(D.nVH), mkB(D.nVH)];
    const [k, v] = fz ? segB([D.kvDim, D.kvDim]) : [mkB(D.kvDim), mkB(D.kvDim)];
    // dn_gates_mc / dn_pre_mc index alpha, betaRaw, beta and decay with ONE column stride (s0), so
    // beta and decay must share the merged [betaRaw | alpha] buffer's stride
    const mkS = (n, stride) => ({ buf: dev.createBuffer({ size: NC * stride, usage: S }), stride, n });
    const [beta, decay] = fz ? [mkS(D.nVH, alpha.stride), mkS(D.nVH, alpha.stride)] : [mkB(D.nVH), mkB(D.nVH)];
    const B = this.B = {
      x: mkB(D.dim), xn: mkB(D.dim), tmpDim: mkB(D.dim), g: mkB(D.inter), u: mkB(D.inter),
      qkv, convOut: mkB(D.convDim), z,
      alpha, betaRaw, beta, decay,
      dOut: mkB(D.dInner), gated: mkB(D.dInner),
      qFull: mkB(D.nH * D.hd * 2), q: mkB(D.qDim), gAttn: mkB(D.qDim),
      k, v, attnOut: mkB(D.qDim),
    };
    if (!(beta.stride === alpha.stride && decay.stride === alpha.stride && betaRaw.stride === alpha.stride))
      throw new Error("batched DeltaNet gates: alpha, betaRaw, beta and decay must share one column stride");
    this.stageXB = dev.createBuffer({ size: NC * D.dim * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    if (this.gemmOn) {
      // column-major copies of the three activation tensors the GEMM reads, and
      // two partials buffers so consecutive GEMMs in one pass do not alias
      const T = (n) => dev.createBuffer({ size: n * NC * 4, usage: S });
      B.xnT = T(D.dim); B.gT = T(D.inter); B.aoT = T(Math.max(D.qDim, D.dInner));
      const maxPart = Math.max(...[...this._gemmShapes].map(([k, sp]) => sp * NC * +k.split("x")[0]));
      this.gemmP = [0, 1].map(() => dev.createBuffer({ size: maxPart * 4, usage: S }));
      this._gz = 0;
      const xp = (src, dst, dIn) => {
        const bufs = [src.buf, src.buf, src.buf, dst, this._shapeB(0, dIn, src.stride / 16, 0)], wgs = Math.ceil(dIn * NC / 64);
        const op = { pipe: "gemm_xpose", wgs, bg: this._bg(this.pipes.gemm_xpose, 1, bufs) };
        // prefillMath "f16": the transposed activations are stored rounded to f16 (only GEMMs read them)
        if (this._pmR16) op.r16 = { pipe: "gemm_xpose_r16", wgs, bg: this._bg(this.pipes.gemm_xpose_r16, 1, bufs) };
        return op;
      };
      this.xposeXn = xp(B.xn, B.xnT, D.dim);
      this.xposeG = xp(B.g, B.gT, D.inter);
      this.xposeGated = xp(B.gated, B.aoT, D.dInner);
      this.xposeAttnOut = xp(B.attnOut, B.aoT, D.qDim);
    }
    // Rollback for the recurrent layers (speculative decoding). Snapshots: the state after every
    // verify column, 7 slots (~3.1 MB each per layer on the 27B: ~1 GB for a whole model).
    // Replay (default): the pre-verify state plus each column's delta-rule inputs, which
    // dn_delta_mc writes into L.rp itself during the verify (no copies); a rejection re-runs
    // dn_delta_mc from there over the accepted columns: same kernel, same inputs, same order, so
    // the state is bit-identical, in ~1/7 of the memory. The conv state snapshots are small and
    // stay either way. L.rp: [S_pre | maxCols columns of conv | of beta | of decay].
    const maxCols = this._rpCols = Math.max(NC, 8);
    if (this.replay && B.decay.stride !== B.beta.stride) throw new Error("replay: beta and decay must share a column stride");
    this._dummy = this._dummy || dev.createBuffer({ size: 256, usage: S });
    for (const L of this.layers) if (!L.isFull && !L.conv_shadow) {
      L.conv_shadow = dev.createBuffer({ size: Math.max(7, this.maxDrafts) * L.convState.size, usage: S });
      if (this.replay) {
        L.rp = L.S_shadow = dev.createBuffer({ size: L.S.size + maxCols * (B.convOut.stride + 2 * B.beta.stride), usage: S });
      } else L.S_shadow = dev.createBuffer({ size: 7 * L.S.size, usage: S });
    }
    // + 8 x 16 B tail: the fused speculative step (_verifyFused) reads its drafts in the same map
    this.stageLogitsN = dev.createBuffer({ size: NC * D.vocab * 4 + 128, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    const slice = (b, c) => ({ buffer: b.buf, offset: c * b.stride + (b.off || 0), size: b.n * 4 });
    const part = (b, c, off, size) => ({ buffer: b.buf, offset: c * b.stride + (b.off || 0) + off, size });
    // a batched tensor bound from its first column on (a fuseProj segment starts at its offset)
    const yv = (b) => b.off ? Qwen35Engine._view(b.buf, b.off, b.buf.size - b.off) : b.buf;
    this.frameBufsB = cix.map(() => dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST }));
    const colPipes = this._colPipes = ["rmsnorm", "head_norm", "attn_scores", "attn_softmax", "attn_softmax_wg", "attn_out",
      "silu_mul", "add_res", "rope_part", "qsplit", "sigmoid_mul",
      "dn_gates", "dn_conv", "dn_l2", "dn_delta", "dn_gatenorm",
      "rmsnorm_mc", "add_res_mc", "dn_gates_mc", "dn_conv_mc", "dn_l2_mc", "dn_pre_mc", "dn_delta_mc", "dn_conv_restore", "dn_gatenorm_mc",
      "qsplit_mc", "head_norm_mc", "rope_part_mc", "sigmoid_mul_mc", "attn_glue",
      "attn_scores_mc", "attn_softmax_wg_mc", "attn_out_mc", "kv_store", "attn_flash", "attn_combine", "kv_store_q8", "attn_flash_q8", "attn_flash_t2",
      ...(this.pipes.attn_flash_tile ? ["attn_flash_tile", "attn_combine_tile"] : []),
      ...(this.pipes.attn_dec ? ["attn_dec", "attn_dec_combine"] : []),
      ...(this.dnNba ? ["dn_nba"] : []),
      ...(this.moe ? ["moe_router", "moe_combine", "moe_gu_q4", "moe_gu_q8", "moe_dn_q4", "moe_dn_q8"] : []),
      ...(this.moeFuse ? ["moe_route", ...(this.moe.nrt ? ["moe_nrt"] : []), ...this.moe.guPairs.map((p) => "moe_gus_" + p), ...this.moe.dnPairs.map((p) => "moe_dnc_" + p)] : [])];
    this.bgCommonB = cix.map((c) => {
      const m = {};
      for (const name of colPipes)
        m[name] = this._bg2g0(this.pipes[name], [{ buffer: this.cfgBuf }, { buffer: this.frameBufsB[c] }]);
      return m;
    });
    const mvB = (w, xB, yB, dOut, dIn, acc = false, xT = null) => {
      const base = w.kind === "q8" ? "matvec_q8" : w.kind === "q4" ? "matvec_q4" : "matvec";
      const pipe = base + "_coop_b" + (acc ? "_acc" : "");
      const shp = this._shapeB(dOut, dIn, xB.stride / 16, yB.stride / 4);
      const bufs = w.kind === "f32" ? [w.buf, yv(xB), yv(yB), shp] : [w.qs, w.sc, yv(xB), yv(yB), shp];
      const op = { pipe, acc, wgs: Math.ceil(dOut / this._rowsFor(this.NC)), bg: this._bg(this.pipes[pipe], 1, bufs) };
      for (const W of [8, 4]) if (this.NC > W) {
        op[`pipe${W}`] = `${base}_coop_b${W}${acc ? "_acc" : ""}`;
        op[`bg${W}`] = this._bg(this.pipes[op[`pipe${W}`]], 1, bufs);
        op[`wgs${W}`] = Math.ceil(dOut / this._rowsFor(W));   // narrower twins carry more rows per workgroup
      }
      const S2 = this._gemmShapes.get(`${dOut}x${dIn}`);
      if (S2 && xT && (w.kind === "q4" || (w.kind === "q8" && this._gemm8Set.has(`${dIn}:${S2}`)))) {
        const gp = `gemm_${w.kind}_${dIn}_s${S2}`, rp = `gemm_red_s${S2}${acc ? "_acc" : ""}`;
        op.gemm = {
          q8: w.kind === "q8",
          pipe: gp, wgs: Math.ceil(dOut / GEMM_TILE) * S2,
          bg: [0, 1].map((z) => this._bg(this.pipes[gp], 1, [w.qs, w.sc, xT, this.gemmP[z], shp])),
          red: rp, redBg: [0, 1].map((z) => this._bg(this.pipes[rp], 1, [this.gemmP[z], w.sc, w.sc, yv(yB), shp])),
          redWgs: Math.ceil(this.NC * dOut / 64),
        };
        // opt-in operand precisions (prefillMath): same buffers, grid and split-K partials, same reduce
        const sgp = `gemm_sgm_${w.kind}_${dIn}_s${S2}`;
        if (this._sgmPipes?.has(sgp) && dOut % this.sgmCfg.TM === 0)
          op.gemm.sgm = { pipe: sgp, wgs: dOut / this.sgmCfg.TM * S2, bg: [0, 1].map((z) => this._bg(this.pipes[sgp], 1, [w.qs, w.sc, xT, this.gemmP[z], shp])) };
        if (this._pmR16)
          op.gemm.r16 = { pipe: gp + "_r16", bg: [0, 1].map((z) => this._bg(this.pipes[gp + "_r16"], 1, [w.qs, w.sc, xT, this.gemmP[z], shp])) };
      }
      return op;
    };
    if (this.hasHead) {
      B.logits = mkB(D.vocab);
      this.headB = mvB(this.headEntry, B.xn, B.logits, D.vocab, D.dim);
      this.bgFinalNormB = cix.map((c) => this._bg2res(this.pipes.rmsnorm,
        [slice(B.x, c), { buffer: this.finalNorm.buf }, { buffer: this.xn }, { buffer: this.uDim }]));
      if (this.mtp) this.mtp.bgHNormB = cix.map((c) => this._bg2res(this.pipes.rmsnorm,
        [slice(B.x, c), { buffer: this.mtp.hnorm.buf }, { buffer: this.mtp.ehIn, offset: D.dim * 4, size: D.dim * 4 }, { buffer: this.uDim }]));
    }
    // per-column score rows for the batched attention (NC x nH x maxSeq; 3 MB at 16 x 24 x 2048)
    if (this.attnMCOn && !this.flash && !this.scoresMC) this.scoresMC = dev.createBuffer({ size: NC * this.dims.nH * this.maxSeq * 4, usage: GPUBufferUsage.STORAGE });
    this._mcU = this._mcU || {};
    if (this.moe && !B.mLogits) {   // routing + expert activations for every column
      const { nExp, K, inter: ei } = this.moe;
      const [mLogits, mSg] = this.fuseProjOn && !this.moeFuse ? segB([nExp, 1]) : [mkB(nExp + 1), mkB(1)];
      Object.assign(B, { mLogits, mSh: mkB(D.dim), mSg,
        mSel: dev.createBuffer({ size: NC * (K + 1) * 4, usage: S }), mSelw: dev.createBuffer({ size: NC * (K + 1) * 4, usage: S }),
        mH: dev.createBuffer({ size: NC * K * ei * 4, usage: S }), mY: dev.createBuffer({ size: NC * K * D.dim * 4, usage: S }) });
      if (this.moeFuse) B.mHF = dev.createBuffer({ size: NC * this.moe.KS * this.moe.hs * 4, usage: S });
    }
    // expert-grouped prefill (_prefillGrouped): the ubatch's residual / normed input / routing (copied out of
    // the pass buffers after each sub-pass), its expert activations and outputs, the chunk list, the indirect
    // launch sizes, and one frame uniform (+ group-0 bind groups) per sub-pass
    if (this.moeGrpU && !this.gB) {
      const U = this.moeGrpU, { K, KS, hs, nExp } = this.moe, UC = this.moeGrpUC, R = dnGroupRows(UC), sz = moeGroupSizes({ U, K, nExp, UC });
      const nb = (n) => dev.createBuffer({ size: n, usage: S });
      const gB = this.gB = { U, nSub: U / NC, sz,
        XW: nb(U * B.x.stride), XNW: nb(U * B.xn.stride), sel: nb(U * KS * 4), selw: nb(U * KS * 4),
        H: nb(U * KS * hs * 4), Y: nb(U * KS * D.dim * 4), grp: nb(sz.words * 4),
        ind: dev.createBuffer({ size: 32, usage: S | GPUBufferUsage.INDIRECT }),
        frames: Array.from({ length: U / NC }, () => dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST })) };
      gB.common = gB.frames.map((f) => Object.fromEntries(colPipes.map((name) => [name, this._bg2g0(this.pipes[name], [{ buffer: this.cfgBuf }, { buffer: f }])])));
      const u32 = (a) => ({ buffer: this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM) });
      // the sort's uniform { pairs, nExp, gate/up x groups, down x groups }: pairs is rewritten per ubatch (width W <= U)
      gB.sortU = dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });
      gB.sortArgs = this.moeGrpTiled ? [nExp, Math.ceil(hs / tileRows("gu")), Math.ceil(D.dim / tileRows("dn"))] : [nExp, Math.ceil(hs / 4), Math.ceil(D.dim / R)]; gB.sortW = 0;
      gB.bgSort = this._bg2res(this.pipes.moe_gsort, [{ buffer: gB.sel }, { buffer: gB.grp }, { buffer: gB.ind }, { buffer: gB.sortU }]);
      gB.bgComb = this._bg2res(this.pipes.moe_combw, [{ buffer: gB.XW }, { buffer: gB.Y }, { buffer: gB.selw },
        u32([D.dim, 0, 0, nExp, B.x.stride / 4, 0, 0, 0, 0, 0, 0, 0])]);
      if (this.moeDp4a) {   // the quantized expert inputs: packed i8 blocks, then their f32 scales (moe_qx)
        gB.XQ = nb(U * D.dim * 9 / 8); gB.HQ = nb(U * KS * hs * 9 / 8);
        gB.bgQx = this._bg2res(this.pipes.moe_qx, [{ buffer: gB.XNW }, { buffer: gB.XQ }, u32([D.dim, B.xn.stride / 16, U * D.dim / 4, 0])]);
        // h is compact (pair stride hs): quantized as one flat column of U * KS * hs values, so every workgroup is full
        gB.bgQh = this._bg2res(this.pipes.moe_qx, [{ buffer: gB.H }, { buffer: gB.HQ }, u32([U * KS * hs, 0, U * KS * hs / 4, 0])]);
      }
    }
    if (this.flash) this.faUB = this._buf(new Uint32Array([B.q.stride / 4, B.attnOut.stride / 4, this.faSplit, this.faSplits]), GPUBufferUsage.UNIFORM);
    if (this.adCfg) this.adUB = this._buf(new Uint32Array([B.q.stride / 4, B.attnOut.stride / 4, this.adCfg.S, this.adCfg.S]), GPUBufferUsage.UNIFORM);
    const mcU = (n, s0 = 0, s1 = 0, s2 = 0) => {
      const k = n + "," + s0 + "," + s1 + "," + s2;
      return this._mcU[k] || (this._mcU[k] = { buffer: this._buf(new Uint32Array([n, s0, s1, s2]), GPUBufferUsage.UNIFORM) });
    };
    const st = (b) => b.stride / 4;   // column stride in floats
    const whole = (b) => (b.off ? { buffer: b.buf, offset: b.off } : { buffer: b.buf });
    const dn = { buffer: this.dnBuf };
    if (this.hasHead) this.bgFinalNormMC = this._bg2res(this.pipes.rmsnorm_mc,
      [whole(B.x), { buffer: this.finalNorm.buf }, whole(B.xn), mcU(D.dim, st(B.x), st(B.xn))]);
    // batched draft-block pre-pass: per column, eh_proj [enorm(embed(next token)) | hnorm(trunk hidden)]
    if (this.mtp) {
      const M2 = this.mtp;
      B.mEmb = mkB(D.dim); B.ehIn = mkB(2 * D.dim);
      M2.bgENormMC = this._bg2res(this.pipes.rmsnorm_mc, [whole(B.mEmb), { buffer: M2.enorm.buf },
        { buffer: B.ehIn.buf, offset: 0, size: B.ehIn.buf.size }, mcU(D.dim, st(B.mEmb), st(B.ehIn))]);
      M2.bgHNormMC = this._bg2res(this.pipes.rmsnorm_mc, [whole(B.x), { buffer: M2.hnorm.buf },
        { buffer: B.ehIn.buf, offset: D.dim * 4, size: B.ehIn.buf.size - D.dim * 4 }, mcU(D.dim, st(B.x), st(B.ehIn))]);
      M2.projB = mvB(M2.ehProj, B.ehIn, B.x, D.dim, 2 * D.dim);
    }
    // the draft (MTP) block is a full-attention layer too: it gets batched bind groups at index
    // this.layers.length, so prefill can fill the draft cache for a whole chunk in one pass
    // a merged batched op must keep the row grouping of every kernel width it can run at
    const batchWidths = [this.NC, ...[8, 4].filter((W) => this.NC > W)];
    const batchOK = (f) => batchWidths.every((W) => Qwen35Engine._rowsKeep(f.lens, this._rowsFor(W)));
    this.layerB = (this.mtpLayer ? [...this.layers, this.mtpLayer] : this.layers).map((L) => {
      const bgNormC = (w, c) => this._bg2res(this.pipes.rmsnorm,
        [slice(B.x, c), { buffer: w.buf }, slice(B.xn, c), { buffer: this.uDim }]);
      const mc = {
        norm1: this._bg2res(this.pipes.rmsnorm_mc, [whole(B.x), { buffer: L.attnNorm.buf }, whole(B.xn), mcU(D.dim, st(B.x), st(B.xn))]),
        norm2: this._bg2res(this.pipes.rmsnorm_mc, [whole(B.x), { buffer: L.postNorm.buf }, whole(B.xn), mcU(D.dim, st(B.x), st(B.xn))]),
        addTmp: this._bg2res(this.pipes.add_res_mc, [whole(B.x), whole(B.tmpDim), mcU(D.dim, st(B.x), st(B.tmpDim))]),
      };
      if (L.isFull) Object.assign(mc, {
        qsplit: this._bg2res(this.pipes.qsplit_mc, [whole(B.qFull), whole(B.q), whole(B.gAttn), mcU(0, st(B.qFull), st(B.q), st(B.gAttn)), dn]),
        qNorm: this._bg2res(this.pipes.head_norm_mc, [whole(B.q), { buffer: L.qNorm.buf }, mcU(D.nH, st(B.q))]),
        kNorm: this._bg2res(this.pipes.head_norm_mc, [whole(B.k), { buffer: L.kNorm.buf }, mcU(D.nKV, st(B.k))]),
        ropeQ: this._bg2res(this.pipes.rope_part_mc, [whole(B.q), mcU(D.nH, st(B.q)), dn]),
        ropeK: this._bg2res(this.pipes.rope_part_mc, [whole(B.k), mcU(D.nKV, st(B.k)), dn]),
        kvStore: this.flash && this._bg2res(this.pipes[this.ksPipe], [whole(B.k), whole(B.v), { buffer: L.kCache }, { buffer: L.vCache },
          ...(this.kvQ8 ? [{ buffer: L.kScale }, { buffer: L.vScale }] : []), mcU(0, st(B.k), st(B.v))]),
        flash: this.flash && this._bg2res(this.pipes[this.faPipe], [whole(B.q), { buffer: L.kCache }, { buffer: L.vCache },
          ...(this.kvQ8 ? [{ buffer: L.kScale }, { buffer: L.vScale }] : []), { buffer: this.faO }, { buffer: this.faML },
          { buffer: this.faUB }]),
        flashT2: this.attnTileOn && this._bg2res(this.pipes.attn_flash_t2, [whole(B.q), { buffer: L.kCache }, { buffer: L.vCache }, { buffer: this.faO }, { buffer: this.faML }, { buffer: this.faUB }]),
        combine: this.flash && this._bg2res(this.pipes.attn_combine, [{ buffer: this.faO }, { buffer: this.faML }, whole(B.attnOut), { buffer: this.faUB }]),
        flashTile: !!this.pipes.attn_flash_tile && this._bg2res(this.pipes.attn_flash_tile, [whole(B.q), { buffer: L.kCache }, { buffer: L.vCache }, { buffer: this.faO }, { buffer: this.faML }, { buffer: this.faUB }]),
        dec: !!this.pipes.attn_dec && this._bg2res(this.pipes.attn_dec, [whole(B.q), { buffer: L.kCache }, { buffer: L.vCache }, { buffer: this.faO }, { buffer: this.faML }, { buffer: this.adUB }]),
        decC: !!this.pipes.attn_dec && this._bg2res(this.pipes.attn_dec_combine, [{ buffer: this.faO }, { buffer: this.faML }, whole(B.attnOut), { buffer: this.adUB }]),
        combineTile: !!this.pipes.attn_combine_tile && this._bg2res(this.pipes.attn_combine_tile, [{ buffer: this.faO }, { buffer: this.faML }, whole(B.attnOut), { buffer: this.faUB }]),
        scoresMC: this.scoresMC && this._bg2res(this.pipes.attn_scores_mc, [whole(B.q), { buffer: L.kCache }, { buffer: this.scoresMC }, mcU(0, st(B.q))]),
        softmaxMC: this.scoresMC && this._bg2res(this.pipes.attn_softmax_wg_mc, [{ buffer: this.scoresMC }]),
        outMC: this.scoresMC && this._bg2res(this.pipes.attn_out_mc, [{ buffer: this.scoresMC }, { buffer: L.vCache }, whole(B.attnOut), mcU(0, st(B.attnOut))]),
        glue: this._bg2res(this.pipes.attn_glue, [whole(B.qFull), whole(B.q), whole(B.gAttn), whole(B.k),
          { buffer: L.qNorm.buf }, { buffer: L.kNorm.buf }, mcU(st(B.k), st(B.qFull), st(B.q), st(B.gAttn)), dn]),
        sigMul: this._bg2res(this.pipes.sigmoid_mul_mc, [whole(B.attnOut), whole(B.gAttn), mcU(D.qDim, st(B.attnOut), st(B.gAttn))]),
      });
      else Object.assign(mc, {
        gates: this._bg2res(this.pipes.dn_gates_mc, [whole(B.alpha), whole(B.betaRaw), { buffer: L.dtBias }, { buffer: L.ssmA },
          whole(B.beta), whole(B.decay), mcU(D.nVH, st(B.alpha))]),
        conv: this._bg2res(this.pipes.dn_conv_mc, [whole(B.qkv), { buffer: L.convW }, { buffer: L.convState }, whole(B.convOut),
          mcU(D.convDim, st(B.qkv), st(B.convOut)), { buffer: L.conv_shadow }]),
        l2: this._bg2res(this.pipes.dn_l2_mc, [whole(B.convOut), mcU(D.nKH, st(B.convOut), D.keyDim), dn]),
        pre: this._bg2res(this.pipes.dn_pre_mc, [whole(B.alpha), whole(B.betaRaw), { buffer: L.dtBias }, { buffer: L.ssmA },
          whole(B.beta), whole(B.decay), whole(B.convOut), mcU(0, st(B.alpha), st(B.convOut)), dn]),
        delta: this._bg2res(this.pipes.dn_delta_mc, [whole(B.convOut), whole(B.beta), whole(B.decay), { buffer: L.S }, whole(B.dOut),
          mcU(L.rp ? this._rpCols : 0, st(B.convOut), st(B.beta), st(B.dOut)), dn, { buffer: L.S_shadow }]),
        // replay: the same kernel over the saved inputs of the accepted columns, from S_pre (all in L.rp)
        replay: L.rp ? this._bg2res(this.pipes.dn_delta_mc, [{ buffer: L.rp }, { buffer: L.rp }, { buffer: L.rp }, { buffer: L.S }, whole(B.dOut),
          mcU(this._rpCols, st(B.convOut), st(B.beta), st(B.dOut)), dn, { buffer: this._dummy }]) : null,
        convRestore: this._bg2res(this.pipes.dn_conv_restore, [{ buffer: L.conv_shadow }, { buffer: L.convState }, mcU(L.convState.size / 4)]),
        gatenorm: this._bg2res(this.pipes.dn_gatenorm_mc, [whole(B.dOut), whole(B.z), { buffer: L.ssmNorm }, whole(B.gated),
          mcU(0, st(B.dOut), st(B.z), st(B.gated)), dn]),
      });
      const dense = !L.moe || (L.shared && !L.fused);   // the (unfused) shared expert is an ordinary gated FFN
      const R = {
        mc,
        gateUp: dense ? [mvB(L.ffnGate, B.xn, B.g, D.inter, D.dim, false, this.gemmOn ? B.xnT : null), mvB(L.ffnUp, B.xn, B.u, D.inter, D.dim, false, this.gemmOn ? B.xnT : null)] : [],
        gu: dense ? this._guOp(L.ffnGate, L.ffnUp, B.xn.buf, B.g.buf, D.inter, D.dim, B.xn, B.g) : null,
        down: !L.moe ? mvB(L.ffnDown, B.g, B.x, D.dim, D.inter, true, this.gemmOn ? B.gT : null) : null,
        cols: cix.map((c) => ({
          norm1: bgNormC(L.attnNorm, c),
          norm2: bgNormC(L.postNorm, c),
          addTmp: this._bg2res(this.pipes.add_res, [slice(B.x, c), slice(B.tmpDim, c)]),
        })),
        // SiLU(g) * u for all nCols columns in one launch (silu_mul_w: silu_mul's expression, same bits)
        silu: dense ? this._bg2res(this.pipes.silu_mul_w, [whole(B.g), whole(B.u), mcU(D.inter, st(B.g), st(B.u))]) : null,
      };
      if (L.fused) {
        const { nExp, inter: ei, hs } = this.moe;
        const U = (a) => ({ buffer: this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM) });
        if (this.moe.nrt) mc.nrt = this._bg2res(this.pipes.moe_nrt, [whole(B.x), { buffer: L.postNorm.buf }, { buffer: L.router.buf }, whole(B.xn), whole(B.mLogits),
          U([D.dim, nExp + 1, st(B.x), st(B.mLogits), st(B.xn), 0, 0, 0])]);
        else R.router = mvB(L.router, B.xn, B.mLogits, nExp + 1, D.dim);
        mc.route = this._bg2res(this.pipes.moe_route, [whole(B.mLogits), { buffer: B.mSel }, { buffer: B.mSelw }, U(L.moeU(st(B.mLogits), 0, 0, 0))]);
        mc.gus = this._bg2res(this.pipes[L.gusPipe], [{ buffer: L.expGate.qs }, { buffer: L.expGate.sc }, { buffer: L.expUp.qs }, { buffer: L.expUp.sc },
          whole(B.xn), { buffer: B.mHF }, { buffer: B.mSel }, { buffer: L.shPack.buf }, U(L.moeU(st(B.xn), ei, D.dim, hs, L.shPack))]);
        mc.dnc = this._bg2res(this.pipes[L.dncPipe], [{ buffer: L.expDown.qs }, { buffer: L.expDown.sc }, { buffer: B.mHF }, whole(B.x),
          { buffer: B.mSel }, { buffer: B.mSelw }, { buffer: L.ffnDown.qs }, { buffer: L.ffnDown.sc }, U(L.moeU(st(B.x), D.dim, ei, hs))]);
        if (L.off) {   // offloaded: the same launches on the layer's slot pool, slot ids from B.offSel (_cutResolve)
          B.offSel ||= dev.createBuffer({ size: NC * this.moe.KS * 4, usage: S });
          mc.gusP = this._bg2res(this.pipes[L.gusPipe], [{ buffer: L.poolGate.qs }, { buffer: L.poolGate.sc }, { buffer: L.poolUp.qs }, { buffer: L.poolUp.sc },
            whole(B.xn), { buffer: B.mHF }, { buffer: B.offSel }, { buffer: L.shPack.buf }, U(L.moeU(st(B.xn), ei, D.dim, hs, L.shPack))]);
          mc.dncP = this._bg2res(this.pipes[L.dncPipe], [{ buffer: L.poolDown.qs }, { buffer: L.poolDown.sc }, { buffer: B.mHF }, whole(B.x),
            { buffer: B.offSel }, { buffer: B.mSelw }, { buffer: L.ffnDown.qs }, { buffer: L.ffnDown.sc }, U(L.moeU(st(B.x), D.dim, ei, hs))]);
        }
        if (this.gB) {   // grouped prefill: the same weights and uniforms, over the ubatch buffers and the chunk list
          const gB = this.gB;
          mc.gusG = this._bg2res(this.pipes[L.gusgPipe], [{ buffer: L.expGate.qs }, { buffer: L.expGate.sc }, { buffer: L.expUp.qs }, { buffer: L.expUp.sc },
            { buffer: gB.XNW }, { buffer: gB.H }, { buffer: gB.grp }, { buffer: L.shPack.buf }, U(L.moeU(st(B.xn), ei, D.dim, hs, L.shPack))]);
          mc.dngG = this._bg2res(this.pipes[L.dngPipe], [{ buffer: L.expDown.qs }, { buffer: L.expDown.sc }, { buffer: gB.H }, { buffer: gB.Y },
            { buffer: gB.grp }, { buffer: L.ffnDown.qs }, { buffer: L.ffnDown.sc }, U(L.moeU(st(B.x), D.dim, ei, hs))]);
          if (this.moeDp4a) {
            mc.gusQ = this._bg2res(this.pipes[L.gusqPipe], [{ buffer: L.expGate.qs }, { buffer: L.expGate.sc }, { buffer: L.expUp.qs }, { buffer: L.expUp.sc },
              { buffer: gB.XQ }, { buffer: gB.H }, { buffer: gB.grp }, { buffer: L.shPack.buf }, U(L.moeU(st(B.xn), ei, D.dim, hs, L.shPack))]);
            mc.dnQ = this._bg2res(this.pipes[L.dnqPipe], [{ buffer: L.expDown.qs }, { buffer: L.expDown.sc }, { buffer: gB.HQ }, { buffer: gB.Y },
              { buffer: gB.grp }, { buffer: L.ffnDown.qs }, { buffer: L.ffnDown.sc }, U(L.moeU(st(B.x), D.dim, ei, hs))]);
          }
        }
      } else if (L.moe) {
        const { nExp, K, inter: ei, norm } = this.moe;
        const U = (a) => ({ buffer: this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM) });
        R.router = mvB(L.router, B.xn, B.mLogits, nExp, D.dim);
        mc.router = this._bg2res(this.pipes.moe_router, [whole(B.mLogits), { buffer: B.mSel }, { buffer: B.mSelw }, U([0, 0, K, nExp, st(B.mLogits), 0, norm, 0])]);
        mc.gu = this._bg2res(this.pipes[L.guPipe], [{ buffer: L.expGate.qs }, { buffer: L.expGate.sc }, { buffer: L.expUp.qs }, { buffer: L.expUp.sc },
          whole(B.xn), { buffer: B.mH }, { buffer: B.mSel }, U([ei, D.dim, K, nExp, st(B.xn), ei, 0, 0])]);
        mc.dn = this._bg2res(this.pipes[L.dnPipe], [{ buffer: L.expDown.qs }, { buffer: L.expDown.sc }, { buffer: B.mH }, { buffer: B.mY }, { buffer: B.mSel },
          U([D.dim, ei, K, nExp, ei, D.dim, 0, 0])]);
        if (L.shared) {
          R.shDown = mvB(L.ffnDown, B.g, B.mSh, D.dim, D.inter, false, this.gemmOn ? B.gT : null);
          R.shRouter = mvB(L.shRouter, B.xn, B.mSg, 1, D.dim);
          if (L.fRS && batchOK(L.fRS)) R.rs = mvB(L.fRS.w, B.xn, { buf: B.mLogits.buf, stride: B.mLogits.stride }, L.fRS.rows, D.dim);
        }
        mc.moeCombine = this._bg2res(this.pipes.moe_combine, [whole(B.x), { buffer: B.mY }, { buffer: B.mSelw }, whole(B.mSh), whole(B.mSg),
          U([D.dim, 0, K, st(B.mSh), st(B.x), D.dim, L.shared ? 1 : 0, st(B.mSg)])]);
      }
      if (L.isFull) {
        R.qkvOps = [mvB(L.wq, B.xn, B.qFull, D.nH * D.hd * 2, D.dim, false, this.gemmOn ? B.xnT : null),
          mvB(L.wk, B.xn, B.k, D.kvDim, D.dim, false, this.gemmOn ? B.xnT : null), mvB(L.wv, B.xn, B.v, D.kvDim, D.dim, false, this.gemmOn ? B.xnT : null)];
        if (L.fKV && batchOK(L.fKV)) R.kv = mvB(L.fKV.w, B.xn, { buf: B.k.buf, stride: B.k.stride }, L.fKV.rows, D.dim);
        R.o = mvB(L.wo, B.attnOut, B.x, D.dim, D.qDim, true, this.gemmOn ? B.aoT : null);
        for (let c = 0; c < NC; c++) Object.assign(R.cols[c], {
          qsplit: this._bg2res(this.pipes.qsplit, [slice(B.qFull, c), slice(B.q, c), slice(B.gAttn, c), { buffer: this.dnBuf }]),
          qNorm: this._bg2res(this.pipes.head_norm, [slice(B.q, c), { buffer: L.qNorm.buf }, { buffer: this.uNH }]),
          kNorm: this._bg2res(this.pipes.head_norm, [slice(B.k, c), { buffer: L.kNorm.buf }, { buffer: this.uNKV }]),
          ropeQ: this._bg2res(this.pipes.rope_part, [slice(B.q, c), { buffer: this.uNH }, { buffer: this.dnBuf }]),
          ropeK: this._bg2res(this.pipes.rope_part, [slice(B.k, c), { buffer: this.uNKV }, { buffer: this.dnBuf }]),
          scores: this._bg2res(this.pipes.attn_scores, [slice(B.q, c), { buffer: L.kCache }, { buffer: this.scores }]),
          softmax: this._bg2res(this.pipes.attn_softmax, [{ buffer: this.scores }]),
          attnOut: this._bg2res(this.pipes.attn_out, [{ buffer: this.scores }, { buffer: L.vCache }, slice(B.attnOut, c)]),
          sigMul: this._bg2res(this.pipes.sigmoid_mul, [slice(B.attnOut, c), slice(B.gAttn, c), { buffer: this.uQDim }]),
        });
      } else {
        R.dnOps = [mvB(L.wqkv, B.xn, B.qkv, D.convDim, D.dim, false, this.gemmOn ? B.xnT : null), mvB(L.wz, B.xn, B.z, D.dInner, D.dim, false, this.gemmOn ? B.xnT : null),
          mvB(L.wBeta, B.xn, B.betaRaw, D.nVH, D.dim), mvB(L.wAlpha, B.xn, B.alpha, D.nVH, D.dim)];
        if (L.fQZ && batchOK(L.fQZ)) R.qz = mvB(L.fQZ.w, B.xn, { buf: B.qkv.buf, stride: B.qkv.stride }, L.fQZ.rows, D.dim);
        if (L.fBA && batchOK(L.fBA)) R.ba = mvB(L.fBA.w, B.xn, { buf: B.betaRaw.buf, stride: B.betaRaw.stride }, L.fBA.rows, D.dim);
        if (L.bgNba) R.nba = this._bg2res(this.pipes.dn_nba, [whole(B.x), { buffer: L.attnNorm.buf }, { buffer: L.fBA.w.buf }, whole(B.xn), { buffer: B.betaRaw.buf },
          { buffer: this._buf(new Uint32Array([D.dim, L.fBA.rows, st(B.x), B.betaRaw.stride / 4, st(B.xn), 0, 0, 0]), GPUBufferUsage.UNIFORM) }]);
        R.out = mvB(L.wOut, B.gated, B.x, D.dim, D.dInner, true, this.gemmOn ? B.aoT : null);
        for (let c = 0; c < NC; c++) Object.assign(R.cols[c], {
          gates: this._bg2res(this.pipes.dn_gates, [slice(B.alpha, c), slice(B.betaRaw, c),
            { buffer: L.dtBias }, { buffer: L.ssmA }, slice(B.beta, c), slice(B.decay, c), { buffer: this.dnBuf }]),
          conv: this._bg2res(this.pipes.dn_conv, [slice(B.qkv, c), { buffer: L.convW },
            { buffer: L.convState }, slice(B.convOut, c), { buffer: this.dnBuf }]),
          l2q: this._bg2res(this.pipes.dn_l2, [part(B.convOut, c, 0, D.keyDim * 4),
            { buffer: this.uNKH }, { buffer: this.dnBuf }]),
          l2k: this._bg2res(this.pipes.dn_l2, [part(B.convOut, c, D.keyDim * 4, D.keyDim * 4),
            { buffer: this.uNKH }, { buffer: this.dnBuf }]),
          delta: this._bg2res(this.pipes.dn_delta, [part(B.convOut, c, 0, D.keyDim * 4),
            part(B.convOut, c, D.keyDim * 4, D.keyDim * 4),
            part(B.convOut, c, D.keyDim * 2 * 4, D.dInner * 4),
            slice(B.beta, c), slice(B.decay, c), { buffer: L.S }, slice(B.dOut, c), { buffer: this.dnBuf }]),
          gatenorm: this._bg2res(this.pipes.dn_gatenorm, [slice(B.dOut, c), slice(B.z, c),
            { buffer: L.ssmNorm }, slice(B.gated, c), { buffer: this.dnBuf }]),
        });
      }
      return R;
    });
  }

  // nCols < NC: the batched matvecs pick their narrower twins (b8/b4) and
  // dispatch over the live columns only; every per-column op runs as ONE
  // multi-column dispatch over the live columns. snapshotDN (set through frame.snap) makes
  // the recurrent kernels save their state after each non-final column so a
  // rejected speculative suffix can be rolled back.
  // routeOnly (grouped prefill, fused MoE layers): stop after moe_route; _prefillGrouped runs the experts
  _encodeLayerBatch(enc, i, basePos, nCols = this.NC, snapshotDN = false, routeOnly = false) {
    const D = this.dims, L = i < this.layers.length ? this.layers[i] : this.mtpLayer, LB = this.layerB[i], B = this.B, M = LB.mc;
    const G = this.gemmOn && this.gemm !== false && nCols === this.NC;   // full-width pass: GEMM needs transposed activations
    if (L.isFull) {
      {
        const p = this._ppB || enc.beginComputePass();
        this._dMC(p, "rmsnorm_mc", M.norm1, 256, 256, nCols);
        if (G) this._dop(p, this.xposeXn);
        const [oq, ok, ov] = LB.qkvOps;
        this._dop(p, oq, nCols);
        if (this.fuseProj && LB.kv && !this._gemmAt(ok, nCols) && !this._gemmAt(ov, nCols)) this._dop(p, LB.kv, nCols);
        else { this._dop(p, ok, nCols); this._dop(p, ov, nCols); }
        this._encAttnGlue(p, M, nCols);
        if (p !== this._ppB) p.end();
      }
      if (!this.flash) for (let c = 0; c < nCols; c++) {
        enc.copyBufferToBuffer(B.k.buf, c * B.k.stride + (B.k.off || 0), L.kCache, (basePos + c) * D.kvDim * 4, D.kvDim * 4);
        enc.copyBufferToBuffer(B.v.buf, c * B.v.stride + (B.v.off || 0), L.vCache, (basePos + c) * D.kvDim * 4, D.kvDim * 4);
      }
      {
        const p = this._ppB || enc.beginComputePass();
        this._encAttnCore(p, LB, M, basePos, nCols);
        if (G) this._dop(p, this.xposeAttnOut);
        this._dop(p, LB.o, nCols);
        if (!LB.o.acc) this._dMC(p, "add_res_mc", M.addTmp, D.dim, 64, nCols);
        if (p !== this._ppB) p.end();
      }
    } else {
      // a verify pass with replay: dn_delta_mc records what replay needs into L.rp (frame.snap)
      const p = this._ppB || enc.beginComputePass();
      const nba = this.fuseProj && LB.nba;   // input norm + beta / alpha (as the one-token path), then [qkv | z]
      if (nba) this._dMC(p, "dn_nba", LB.nba, Math.ceil(this.layers[i].fBA.rows / 4) * 64, 64, nCols);
      else this._dMC(p, "rmsnorm_mc", M.norm1, 256, 256, nCols);
      if (G) this._dop(p, this.xposeXn);
      const [oqkv, oz, obeta, oalpha] = LB.dnOps;
      // full-width prefill keeps qkv and z on the GEMM, which has no merged form
      if (this.fuseProj && LB.qz && !this._gemmAt(oqkv, nCols) && !this._gemmAt(oz, nCols)) this._dop(p, LB.qz, nCols);
      else { this._dop(p, oqkv, nCols); this._dop(p, oz, nCols); }
      this._encDnMid(p, LB, M, nCols, !!nba);
      if (G) this._dop(p, this.xposeGated);
      this._dop(p, LB.out, nCols);
      if (!LB.out.acc) this._dMC(p, "add_res_mc", M.addTmp, D.dim, 64, nCols);
      if (p !== this._ppB) p.end();
    }
    {
      const p = this._ppB || enc.beginComputePass();
      if (!(L.fused && this.moe.nrt)) this._dMC(p, "rmsnorm_mc", M.norm2, 256, 256, nCols);   // (moe_nrt: the norm is in the router launch)
      if (L.moe) {
        const cut = !!L.off && !routeOnly;   // offloaded: stop after moe_route, the experts run after _cutResolve
        this._encMoeFfn(p, L, LB, M, nCols, G, routeOnly || cut);
        if (cut) { this._cutAt(enc, p, L, nCols, B.mSel, M); return; }
        if (p !== this._ppB) p.end();
        return;
      }
      if (G) this._dop(p, this.xposeXn);
      if (LB.gu && !G) this._dop(p, LB.gu, nCols);
      else {   // the GEMM has no fused gate/up: run them separately, then SiLU
        for (const op of LB.gateUp) this._dop(p, op, nCols);
        this._dSiluMC(p, LB.silu, nCols);
      }
      if (G) this._dop(p, this.xposeG);
      this._dop(p, LB.down, nCols);
      if (!LB.down.acc) this._dMC(p, "add_res_mc", M.addTmp, D.dim, 64, nCols);
      if (p !== this._ppB) p.end();
    }
  }

  // ---- pieces of a batched layer pass, shared by _encodeLayerBatch and the wide prefill ----
  // (same dispatches in the same order as before they were split out)
  _encAttnGlue(p, M, nCols) {
    const D = this.dims;
    if (this.attnGlue) this._dMC(p, "attn_glue", M.glue, (D.nH + D.nKV) * 64, 64, nCols);
    else {
      this._dMC(p, "qsplit_mc", M.qsplit, D.nH * D.hd, 64, nCols);
      this._dMC(p, "head_norm_mc", M.qNorm, D.nH, 32, nCols);
      this._dMC(p, "head_norm_mc", M.kNorm, D.nKV, 32, nCols);
      this._dMC(p, "rope_part_mc", M.ropeQ, D.nH * D.nRot / 2, 64, nCols);
      this._dMC(p, "rope_part_mc", M.ropeK, D.nKV * D.nRot / 2, 64, nCols);
    }
  }
  // KV store (flash) + attention + sigmoid gate: B.q / B.k / B.v / B.gAttn -> B.attnOut
  _encAttnCore(p, LB, M, basePos, nCols) {
    const D = this.dims;
    if (this.flash) {
      this._dMC(p, this.ksPipe, M.kvStore, D.kvDim / (this.kvQ8 ? 32 : 2), 64, nCols);
      if (this.attnPrefillTile && M.flashTile && nCols === this.NC && nCols > 1 && !this._snapNow) {
        // full-width prefill pass (never a verify): tiled kernel over every split slot (slots past this
        // pass's split count exit at once; the kernel derives the split length from frame), then its combine
        this._dMC(p, "attn_flash_tile", M.flashTile, this.faSplits * 256, 256, D.nKV, Math.ceil(nCols / this.attnPTCfg.CW));
        this._dMC(p, "attn_combine_tile", M.combineTile, D.nH * 256, 256, nCols);
      } else if (this.attnDecode === "v2" && M.dec) {
        // decode / verify: each column's splits from its own position (engine/wgsl/attn_dec.js)
        let ns = 1;
        for (let c = 0; c < nCols; c++) ns = Math.max(ns, decSplits(basePos + c + 1, this.adCfg.S));
        this._dMC(p, "attn_dec", M.dec, ns * 256, 256, nCols, D.nKVa);
        this._dMC(p, "attn_dec_combine", M.decC, D.nH * DEC_CQ * 256, 256, nCols);
      } else {
        if (this.attnTile && M.flashT2 && nCols > 1) this._dMC(p, "attn_flash_t2", M.flashT2, Math.ceil((basePos + nCols) / this.faSplit) * 256, 256, Math.ceil(nCols / 2), D.nKV);
        else this._dMC(p, this.faPipe, M.flash, Math.ceil((basePos + nCols) / this.faSplit) * 256, 256, nCols, D.nKVa);
        this._dMC(p, "attn_combine", M.combine, D.nH * 256, 256, nCols);
      }
    } else if (this.attnMC && M.scoresMC) {
      this._dMC(p, "attn_scores_mc", M.scoresMC, basePos + nCols, 64, nCols, D.nH);
      this._dMC(p, "attn_softmax_wg_mc", M.softmaxMC, D.nH * 256, 256, nCols);
      this._dMC(p, "attn_out_mc", M.outMC, D.qDim, 64, nCols);
    } else for (let c = 0; c < nCols; c++) {   // shared score scratch: columns in turn
      const C = LB.cols[c];
      this._dCol(p, "attn_scores", c, C.scores, D.nH * (basePos + c + 1));
      if (this.softmaxWG) this._dCol(p, "attn_softmax_wg", c, C.softmax, D.nH * 256, 256);
      else this._dCol(p, "attn_softmax", c, C.softmax, D.nH, 1);
      this._dCol(p, "attn_out", c, C.attnOut, D.qDim);
    }
    this._dMC(p, "sigmoid_mul_mc", M.sigMul, D.qDim, 64, nCols);
  }
  // DeltaNet beta / alpha projections, conv, gates + L2, recurrence, gated norm: B.xn, B.qkv, B.z -> B.gated
  // baDone: dn_nba already wrote beta / alpha
  _encDnBA(p, LB, nCols) {   // the DeltaNet beta / alpha projections: B.xn -> B.betaRaw / B.alpha
    const [, , obeta, oalpha] = LB.dnOps;
    if (this.fuseProj && LB.ba && !this._gemmAt(obeta, nCols) && !this._gemmAt(oalpha, nCols)) this._dop(p, LB.ba, nCols);
    else { this._dop(p, obeta, nCols); this._dop(p, oalpha, nCols); }
  }
  _encDnMid(p, LB, M, nCols, baDone = false) {
    const D = this.dims;
    if (!baDone) this._encDnBA(p, LB, nCols);
    this._dMC(p, "dn_conv_mc", M.conv, D.convDim, 64, 1);           // loops over columns
    this._dMC(p, "dn_pre_mc", M.pre, 128, 128, nCols);              // gates + L2(q,k) fused, one WG per column
    this._dMC(p, "dn_delta_mc", M.delta, D.nVH * 128, 128, 1);     // loops over columns
    this._dMC(p, "dn_gatenorm_mc", M.gatenorm, D.nVH * 128, 128, nCols);
  }
  // MoE FFN after the post-attention norm (B.xn): experts + shared expert, residual into B.x
  // routeOnly (grouped prefill, fused MoE layers): stop after moe_route; _prefillGrouped runs the experts
  _encMoeFfn(p, L, LB, M, nCols, G, routeOnly = false) {
    const D = this.dims;
    if (L.fused) {   // same four launches as the one-token path, one workgroup row per column
      const { KS, hs, gusRows, dncRows } = this.moe;
      if (this.moe.nrt) this._dMC(p, "moe_nrt", M.nrt, Math.ceil((this.moe.nExp + 1) / this.moe.nrt.ROWS) * 64, 64, nCols);   // norm + router
      else this._dop(p, LB.router, nCols);
      this._dMC(p, "moe_route", M.route, nCols * 256, 256, 1);
      if (routeOnly) return;
      this._dMC(p, L.gusPipe, M.gus, Math.ceil(hs / gusRows) * 64, 64, nCols * KS);
      this._dMC(p, L.dncPipe, M.dnc, Math.ceil(D.dim / dncRows) * 64, 64, nCols);
      return;
    }
    const { K, inter: ei } = this.moe;
    const rs = this.fuseProj && LB.rs && !this._gemmAt(LB.router, nCols) && !this._gemmAt(LB.shRouter, nCols) ? LB.rs : null;
    this._dop(p, rs || LB.router, nCols);   // router (+ shared-expert gate when merged)
    this._dMC(p, "moe_router", M.router, nCols * 256, 256, 1);
    this._dMC(p, L.guPipe, M.gu, Math.ceil(ei / this.moeK.gu.rows), 1, nCols * K);   // same grid per (column, slot) as the one-token path
    this._dMC(p, L.dnPipe, M.dn, Math.ceil(D.dim / this.moeK.dn.rows), 1, nCols * K);
    if (L.shared) {
      if (G) this._dop(p, this.xposeXn);
      if (LB.gu && !G) this._dop(p, LB.gu, nCols);
      else {
        for (const op of LB.gateUp) this._dop(p, op, nCols);
        this._dSiluMC(p, LB.silu, nCols);
      }
      if (G) this._dop(p, this.xposeG);
      this._dop(p, LB.shDown, nCols);
      if (!rs) this._dop(p, LB.shRouter, nCols);
    }
    this._dMC(p, "moe_combine", M.moeCombine, D.dim, 64, nCols);
  }

  // ---- wide prefill (prefillUbatch; see _init) ----
  // Every batched tensor B.* gets a twin U columns wide with the same column stride and segment
  // layout, so column block j of a twin is one contiguous range: a sub-batch moves between the wide
  // GEMMs and the batchCols-wide kernels with one copyBufferToBuffer per tensor. Sub-batch j runs
  // the unchanged batched kernels with its own frame (positions basePos + j * NC ...).
  _initWide() {
    if (!this.B) this._initBatch();
    const D = this.dims, dev = this.device, B = this.B, NC = this.NC, U = this.ubatch, cfg = this.wideCfg;
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC;
    const twin = this._wTwin = new Map();   // set first: a failed init frees what it made
    const W = (b) => {
      if (!twin.has(b.buf)) {
        if (b.buf.size !== NC * b.stride) throw new Error("wide prefill: a batched buffer is not NC columns of one stride");
        const lim = Math.min(dev.limits.maxStorageBufferBindingSize, dev.limits.maxBufferSize);
        if (U * b.stride > lim) throw new Error(`wide prefill: ubatch ${U} x ${b.stride} B columns exceeds the ${lim} B buffer limit`);
        twin.set(b.buf, dev.createBuffer({ size: U * b.stride, usage: S }));
      }
      return { buf: twin.get(b.buf), stride: b.stride, n: b.n, off: b.off || 0 };
    };
    const dense = this.layers.some((L) => !L.moe), full = this.layers.some((L) => L.isFull), dn = this.layers.some((L) => !L.isFull);
    const Wt = this.Wt = { x: W(B.x), xn: W(B.xn) };
    if (dn) Object.assign(Wt, { qkv: W(B.qkv), z: W(B.z), gated: W(B.gated) });
    if (full) Object.assign(Wt, { qFull: W(B.qFull), k: W(B.k), v: W(B.v), attnOut: W(B.attnOut) });
    if (dense) Object.assign(Wt, { g: W(B.g), u: W(B.u) });
    const st = (b) => b.stride / 4;
    const whole = (b) => (b.off ? { buffer: b.buf, offset: b.off } : { buffer: b.buf });
    const view = (b) => (b.off ? Qwen35Engine._view(b.buf, b.off, b.buf.size - b.off) : b.buf);
    const mcU = (n, s0 = 0, s1 = 0, s2 = 0) => {
      const k = n + "," + s0 + "," + s1 + "," + s2;
      return this._mcU[k] || (this._mcU[k] = { buffer: this._buf(new Uint32Array([n, s0, s1, s2]), GPUBufferUsage.UNIFORM) });
    };
    // y (+)= w x over the chunk: one launch, grid (dOut / BM, width / BN)
    let wop = (w, x, y, dOut, dIn, acc = false) => {
      const pipe = `gemm_w_${w.kind}${acc ? "_acc" : ""}`;
      if (!this.pipes[pipe]) throw new Error(`wide prefill: no GEMM for ${w.kind} weights`);
      return { pipe, gx: Math.ceil(dOut / cfg.BM), bg: this._bg(this.pipes[pipe], 1, [w.qs, w.sc, view(x), view(y), this._shapeB(dOut, dIn, x.stride / 16, y.stride / 4)]) };
    };
    // dp4a: each GEMM input is quantized once per chunk into one shared scratch (Xq, Xd: compact, nb = dIn / 32
    // blocks per column), consumed by the GEMMs that follow it in the same pass. wop() then returns the dp4a
    // GEMM as op.d next to the f32 one, and qop() the quantize dispatch; engine.prefillDp4a picks per chunk.
    const dc = this.dp4aCfg;
    if (dc) {
      const maxIn = Math.max(D.dim, D.qDim, D.dInner || 0, dense ? D.inter : 0);
      this._wDq = { q: dev.createBuffer({ size: U * maxIn, usage: S }), d: dev.createBuffer({ size: U * maxIn / 8, usage: S }) };
    }
    const qop = (x, dIn) => dc && { dIn, bg: this._bg2res(this.pipes.quant_q8_w, [Qwen35Engine._res(view(x)),
      { buffer: this._wDq.q }, { buffer: this._wDq.d }, mcU(dIn, x.stride / 16)]) };
    const wopF = wop;
    wop = (w, x, y, dOut, dIn, acc = false) => {
      const op = wopF(w, x, y, dOut, dIn, acc);
      if (dc) {
        const pipe = `gemm_d_${w.kind}${acc ? "_acc" : ""}`;
        op.d = { pipe, gx: Math.ceil(dOut / dc.BM), bn: dc.BN, bg: this._bg(this.pipes[pipe], 1, [w.qs, w.sc, this._wDq.q, this._wDq.d, view(y), this._shapeB(dOut, dIn, 0, y.stride / 4)]) };
      }
      return op;
    };
    const uniq = (...bs) => bs.filter((b, i) => bs.findIndex((c) => c.buf === b.buf) === i);   // fuseProj segments share a buffer
    // one frame per sub-batch: [pos, seqLen, nCols, snap], written by prefillTokens before each chunk
    this.frameW = Array.from({ length: U / NC }, () => dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST }));
    this.frameWD = dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST });   // [basePos, basePos + 1, w, 0]
    this.bgCommonWD = {};
    for (const name of this._colPipes) if (this.pipes[name]) this.bgCommonWD[name] = this._bg2g0(this.pipes[name], [{ buffer: this.cfgBuf }, { buffer: this.frameWD }]);
    this.bgCommonW = this.frameW.map((f) => {
      const m = {};
      for (const name of this._colPipes) if (this.pipes[name]) m[name] = this._bg2g0(this.pipes[name], [{ buffer: this.cfgBuf }, { buffer: f }]);
      return m;
    });
    this.layerW = this.layers.map((L) => {
      const R = {
        norm1: this._bg2res(this.pipes.rmsnorm_mc, [whole(Wt.x), { buffer: L.attnNorm.buf }, whole(Wt.xn), mcU(D.dim, st(Wt.x), st(Wt.xn))]),
        norm2: this._bg2res(this.pipes.rmsnorm_mc, [whole(Wt.x), { buffer: L.postNorm.buf }, whole(Wt.xn), mcU(D.dim, st(Wt.x), st(Wt.xn))]),
        qXn: qop(Wt.xn, D.dim),
      };
      if (L.isFull) {
        R.proj = [wop(L.wq, Wt.xn, Wt.qFull, D.nH * D.hd * 2, D.dim)];
        // merged [k | v] rows (fuseProj) write the merged output layout: one launch
        if (L.fKV) R.proj.push(wop(L.fKV.w, Wt.xn, { buf: Wt.k.buf, stride: Wt.k.stride }, L.fKV.rows, D.dim));
        else R.proj.push(wop(L.wk, Wt.xn, Wt.k, D.kvDim, D.dim), wop(L.wv, Wt.xn, Wt.v, D.kvDim, D.dim));
        R.out = wop(L.wo, Wt.attnOut, Wt.x, D.dim, D.qDim, true); R.qOut = qop(Wt.attnOut, D.qDim);
        R.toNarrow = uniq(B.qFull, B.k, B.v);
        R.toWide = [B.attnOut];
      } else {
        R.proj = L.fQZ ? [wop(L.fQZ.w, Wt.xn, { buf: Wt.qkv.buf, stride: Wt.qkv.stride }, L.fQZ.rows, D.dim)]
          : [wop(L.wqkv, Wt.xn, Wt.qkv, D.convDim, D.dim), wop(L.wz, Wt.xn, Wt.z, D.dInner, D.dim)];
        R.out = wop(L.wOut, Wt.gated, Wt.x, D.dim, D.dInner, true); R.qOut = qop(Wt.gated, D.dInner);
        R.toNarrow = uniq(B.xn, B.qkv, B.z);   // xn: the beta / alpha GEMV stays batched
        R.toWide = [B.gated];
        // wide DeltaNet middle (engine.wideDn, default on): the beta / alpha GEMV per sub-batch into a wide twin, then
        // conv, gates + L2, the recurrence and the gated norm ONE launch each over the whole chunk (the same
        // per-column kernels, which loop over frame.nCols columns or take one workgroup row per column: same bits)
        const tw = { ba: W(B.betaRaw), beta: W(B.beta), decay: W(B.decay), conv: W(B.convOut), dOut: W(B.dOut) };
        const dn = { buffer: this.dnBuf };
        R.dnW = {
          conv: this._bg2res(this.pipes.dn_conv_mc, [whole(Wt.qkv), { buffer: L.convW }, { buffer: L.convState }, whole(tw.conv),
            mcU(D.convDim, st(B.qkv), st(B.convOut)), { buffer: L.conv_shadow }]),
          pre: this._bg2res(this.pipes.dn_pre_mc, [whole(W(B.alpha)), whole(tw.ba), { buffer: L.dtBias }, { buffer: L.ssmA },
            whole(tw.beta), whole(tw.decay), whole(tw.conv), mcU(0, st(B.alpha), st(B.convOut)), dn]),
          delta: this._bg2res(this.pipes.dn_delta_mc, [whole(tw.conv), whole(tw.beta), whole(tw.decay), { buffer: L.S }, whole(tw.dOut),
            mcU(0, st(B.convOut), st(B.beta), st(B.dOut)), dn, { buffer: L.S_shadow || this._dummy }]),
          gatenorm: this._bg2res(this.pipes.dn_gatenorm_mc, [whole(tw.dOut), whole(Wt.z), { buffer: L.ssmNorm }, whole(Wt.gated),
            mcU(0, st(B.dOut), st(B.z), st(B.gated)), dn]),
        };
      }
      if (L.moe && L.fused && this.moe.nrt && this.gB) {   // the expert-grouped chunk's router over all its columns at once (_wideGrp)
        const gB = this.gB, ls = B.mLogits.stride / 4, u32 = (a) => ({ buffer: this._buf(new Uint32Array(a), GPUBufferUsage.UNIFORM) });
        if (!gB.LG) gB.LG = dev.createBuffer({ size: U * B.mLogits.stride, usage: S });
        if (!gB.XNS) gB.XNS = dev.createBuffer({ size: U * B.xn.stride, usage: S });   // moe_nrt's own xn: not used (the experts take norm2's)
        R.nrtW = this._bg2res(this.pipes.moe_nrt, [whole(Wt.x), { buffer: L.postNorm.buf }, { buffer: L.router.buf }, { buffer: gB.XNS }, { buffer: gB.LG },
          u32([D.dim, this.moe.nExp + 1, st(Wt.x), ls, st(B.xn), 0, 0, 0])]);
        R.routeW = this._bg2res(this.pipes.moe_route, [{ buffer: gB.LG }, { buffer: gB.sel }, { buffer: gB.selw }, u32(L.moeU(ls, 0, 0, 0))]);
        R.combW = this._bg2res(this.pipes.moe_combw, [whole(Wt.x), { buffer: gB.Y }, { buffer: gB.selw },
          u32([D.dim, 0, 0, this.moe.nExp, st(Wt.x), 0, 0, 0, 0, 0, 0, 0])]);
      }
      if (!L.moe) {
        R.gate = wop(L.ffnGate, Wt.xn, Wt.g, D.inter, D.dim);
        R.up = wop(L.ffnUp, Wt.xn, Wt.u, D.inter, D.dim);
        R.silu = this._bg2res(this.pipes.silu_mul_w, [whole(Wt.g), whole(Wt.u), mcU(D.inter, st(Wt.g), st(Wt.u))]);
        R.down = wop(L.ffnDown, Wt.g, Wt.x, D.dim, D.inter, true); R.qG = qop(Wt.g, D.inter);
      }
      return R;
    });
  }
  // The draft (MTP) block's share of a wide chunk (_mtpFillWide): only its KV cache rows matter during prefill (the
  // draft block's attention output and FFN feed the draft head, which prefill never runs), so the wide fill is
  // enorm / hnorm + eh_proj + the block's input norm + k / v projections as wide GEMMs, then per sub-batch the
  // attention glue (k norm, rope) and kv_store at positions + 1. null when the block cannot take the wide path.
  _initMtpWide() {
    const M2 = this.mtp, L = this.mtpLayer, D = this.dims, dev = this.device, B = this.B, NC = this.NC, U = this.ubatch, Wt = this.Wt, cfg = this.wideCfg;
    const qk = (e) => !!e && (e.kind === "q4" || e.kind === "q8");
    if (!M2 || !L || !L.isFull || !B.mEmb || !qk(M2.ehProj) || !(L.fKV ? qk(L.fKV.w) : qk(L.wk) && qk(L.wv)) || (2 * D.dim) % cfg.KS) return null;
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_DST | GPUBufferUsage.COPY_SRC;
    const lim = Math.min(dev.limits.maxStorageBufferBindingSize, dev.limits.maxBufferSize);
    if (U * B.ehIn.stride > lim) return null;
    const emb = dev.createBuffer({ size: U * B.mEmb.stride, usage: S }), ehIn = dev.createBuffer({ size: U * B.ehIn.stride, usage: S });
    this._wTwin.set(Symbol("mtpEmb"), emb); this._wTwin.set(Symbol("mtpEhIn"), ehIn);   // freed with the twins
    const st = (b) => b.stride / 4;
    const mcU = (n, s0 = 0, s1 = 0, s2 = 0) => {
      const k = n + "," + s0 + "," + s1 + "," + s2;
      return this._mcU[k] || (this._mcU[k] = { buffer: this._buf(new Uint32Array([n, s0, s1, s2]), GPUBufferUsage.UNIFORM) });
    };
    const view = (b) => (b.off ? Qwen35Engine._view(b.buf, b.off, b.buf.size - b.off) : b.buf);
    const op = (w, x, y, dOut, dIn) => {   // as _initWide's wop (f32, plus the dp4a twin when dp4a is built)
      const pipe = `gemm_w_${w.kind}`, o = { pipe, gx: Math.ceil(dOut / cfg.BM), bg: this._bg(this.pipes[pipe], 1, [w.qs, w.sc, view(x), view(y), this._shapeB(dOut, dIn, x.stride / 16, y.stride / 4)]) };
      if (this.dp4aCfg) { const dc = this.dp4aCfg, pd = `gemm_d_${w.kind}`;
        o.d = { pipe: pd, gx: Math.ceil(dOut / dc.BM), bn: dc.BN, bg: this._bg(this.pipes[pd], 1, [w.qs, w.sc, this._wDq.q, this._wDq.d, view(y), this._shapeB(dOut, dIn, 0, y.stride / 4)]) }; }
      return o;
    };
    const qop = (x, dIn) => this.dp4aCfg && { dIn, bg: this._bg2res(this.pipes.quant_q8_w, [Qwen35Engine._res(view(x)),
      { buffer: this._wDq.q }, { buffer: this._wDq.d }, mcU(dIn, x.stride / 16)]) };
    const E = { buf: ehIn, stride: B.ehIn.stride };
    const frames = Array.from({ length: U / NC }, () => dev.createBuffer({ size: 16, usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST }));
    return {
      emb, frames,
      common: frames.map((f) => { const m = {}; for (const name of this._colPipes) if (this.pipes[name]) m[name] = this._bg2g0(this.pipes[name], [{ buffer: this.cfgBuf }, { buffer: f }]); return m; }),
      enorm: this._bg2res(this.pipes.rmsnorm_mc, [{ buffer: emb }, { buffer: M2.enorm.buf }, { buffer: ehIn, offset: 0, size: ehIn.size }, mcU(D.dim, st(B.mEmb), st(B.ehIn))]),
      hnorm: this._bg2res(this.pipes.rmsnorm_mc, [{ buffer: Wt.x.buf }, { buffer: M2.hnorm.buf }, { buffer: ehIn, offset: D.dim * 4, size: ehIn.size - D.dim * 4 }, mcU(D.dim, st(Wt.x), st(B.ehIn))]),
      qIn: qop(E, 2 * D.dim), proj: op(M2.ehProj, E, Wt.x, D.dim, 2 * D.dim),
      norm1: this._bg2res(this.pipes.rmsnorm_mc, [{ buffer: Wt.x.buf }, { buffer: L.attnNorm.buf }, { buffer: Wt.xn.buf }, mcU(D.dim, st(Wt.x), st(Wt.xn))]),
      qXn: qop(Wt.xn, D.dim),
      kv: L.fKV ? [op(L.fKV.w, Wt.xn, { buf: Wt.k.buf, stride: Wt.k.stride }, L.fKV.rows, D.dim)] : [op(L.wk, Wt.xn, Wt.k, D.kvDim, D.dim), op(L.wv, Wt.xn, Wt.v, D.kvDim, D.dim)],
      toNarrow: [B.k, B.v].filter((b, i, a) => a.findIndex((c) => c.buf === b.buf) === i),
    };
  }
  // Fill the draft cache for wide chunk ids[i .. i + w) at positions basePos + 1 ... from the trunk's final hiddens in
  // Wt.x (clobbers Wt.x / Wt.xn). False (nothing done) when the wide fill does not apply: then the per-sub-batch fill runs.
  _mtpFillWide(ids, i, basePos, w) {
    if (this.mtpWide === false || ids.length - i - 1 < w) return false;   // the prompt's last column has no next token: the old fill
    if (this._mtpW === undefined) { try { this._mtpW = this._initMtpWide(); } catch (e) { console.warn("wide draft fill off:", String(e.message || e).slice(0, 200)); this._mtpW = null; } }
    const MW = this._mtpW; if (!MW) return false;
    this._pre = null;   // as _mtpFillBatch
    const q = this.device.queue, NC = this.NC, D = this.dims, B = this.B, Mm = this.layerB[this.layers.length].mc;
    for (let j = 0; j < w / NC; j++) q.writeBuffer(MW.frames[j], 0, new Uint32Array([basePos + j * NC + 1, basePos + j * NC + 2, NC, 0]));
    for (let c = 0; c < w; c++) q.writeBuffer(MW.emb, c * B.mEmb.stride, this._embedRowF32(ids[i + c + 1]));
    const enc = this.device.createCommandEncoder();
    {
      const p = enc.beginComputePass();
      this._dMC(p, "rmsnorm_mc", MW.enorm, 256, 256, w);
      this._dMC(p, "rmsnorm_mc", MW.hnorm, 256, 256, w);
      this._dQ(p, MW.qIn, w);
      this._dW(p, MW.proj, w);
      this._dMC(p, "rmsnorm_mc", MW.norm1, 256, 256, w);
      this._dQ(p, MW.qXn, w);
      for (const o of MW.kv) this._dW(p, o, w);
      p.end();
    }
    try {
      for (let j = 0; j < w / NC; j++) {
        for (const b of MW.toNarrow) this._wCopy(enc, b, j, false);
        this._mcCommon = MW.common[j];
        const p = enc.beginComputePass();
        this._encAttnGlue(p, Mm, NC);
        this._dMC(p, this.ksPipe, Mm.kvStore, D.kvDim / (this.kvQ8 ? 32 : 2), 64, NC);
        p.end();
        this._mcCommon = null;
      }
    } finally { this._mcCommon = null; }
    q.submit([enc.finish()]);
    return true;
  }
  // sub-batch j of the chunk between a wide twin and its batchCols-wide buffer (whole columns, all segments)
  _wCopy(enc, b, j, toWide) {
    const tw = this._wTwin.get(b.buf), n = this.NC * b.stride;
    if (toWide) enc.copyBufferToBuffer(b.buf, 0, tw, j * n, n);
    else enc.copyBufferToBuffer(tw, j * n, b.buf, 0, n);
  }
  _dW(p, op, w) {
    if (op.d && this._dp4aOn) op = op.d;
    if (this.skip && this.skip.has(op.pipe)) return;
    p.setPipeline(this.pipes[op.pipe]);
    p.setBindGroup(0, this.bgCommonFor[op.pipe]);
    p.setBindGroup(1, op.bg);
    p.dispatchWorkgroups(op.gx, w / (op.bn || this.wideCfg.BN));
  }
  // dp4a: quantize a GEMM input (qop from _initWide) for the dp4a GEMMs that follow in the same pass
  _dQ(p, q, w) {
    if (!q || !this._dp4aOn || (this.skip && this.skip.has("quant_q8_w"))) return;
    p.setPipeline(this.pipes.quant_q8_w);
    p.setBindGroup(0, this.bgCommonFor.quant_q8_w);
    p.setBindGroup(1, q.bg);
    p.dispatchWorkgroups(Math.ceil(q.dIn / 32 / 32), w);
  }
  // One layer over a wide chunk of w columns (w a multiple of BN, <= ubatch) at positions basePos...
  // Wt.x holds the chunk's residual stream. Projections: wide GEMMs; the rest: the batched kernels
  // per NC-column sub-batch in order, so the recurrence and the causal KV writes see the columns in
  // sequence exactly as consecutive NC passes would.
  _encodeLayerWide(enc, i, basePos, w) {
    const D = this.dims, L = this.layers[i], LB = this.layerB[i], LW = this.layerW[i], B = this.B, M = LB.mc, NC = this.NC, J = w / NC;
    const G = this.gemmOn && this.gemm !== false;   // what a full-width NC pass would use (unfused MoE shared expert)
    try {
      {
        const p = enc.beginComputePass();
        this._dMC(p, "rmsnorm_mc", LW.norm1, 256, 256, w);
        this._dQ(p, LW.qXn, w);
        for (const op of LW.proj) this._dW(p, op, w);
        p.end();
      }
      if (!L.isFull && LW.dnW && this.wideDn !== false) {
        const B2 = this.B;
        for (let j = 0; j < J; j++) {   // beta / alpha: the batched GEMV per sub-batch, into the wide twin
          this._wCopy(enc, B2.xn, j, false);
          this._mcCommon = this.bgCommonW[j];
          const p = enc.beginComputePass();
          this._encDnBA(p, LB, NC);
          p.end();
          this._mcCommon = null;
          this._wCopy(enc, B2.betaRaw, j, true);
          if (B2.alpha.buf !== B2.betaRaw.buf) this._wCopy(enc, B2.alpha, j, true);
        }
        this._mcCommon = this.bgCommonWD;
        const p = enc.beginComputePass();
        this._dMC(p, "dn_conv_mc", LW.dnW.conv, D.convDim, 64, 1);
        this._dMC(p, "dn_pre_mc", LW.dnW.pre, 128, 128, w);
        this._dMC(p, "dn_delta_mc", LW.dnW.delta, D.nVH * 128, 128, 1);
        this._dMC(p, "dn_gatenorm_mc", LW.dnW.gatenorm, D.nVH * 128, 128, w);
        p.end();
        this._mcCommon = null;
      } else for (let j = 0; j < J; j++) {
        for (const b of LW.toNarrow) this._wCopy(enc, b, j, false);
        this._mcCommon = this.bgCommonW[j];
        const p = enc.beginComputePass();
        if (L.isFull) { this._encAttnGlue(p, M, NC); this._encAttnCore(p, LB, M, basePos + j * NC, NC); }
        else this._encDnMid(p, LB, M, NC);
        p.end();
        this._mcCommon = null;
        for (const b of LW.toWide) this._wCopy(enc, b, j, true);
      }
      {
        const p = enc.beginComputePass();
        this._dQ(p, LW.qOut, w);
        this._dW(p, LW.out, w);
        this._dMC(p, "rmsnorm_mc", LW.norm2, 256, 256, w);
        if (!L.moe) {
          this._dQ(p, LW.qXn, w);
          this._dW(p, LW.gate, w);
          this._dW(p, LW.up, w);
          if (!(this.skip && this.skip.has("silu_mul_w"))) {
            p.setPipeline(this.pipes.silu_mul_w); p.setBindGroup(0, this.bgCommonFor.silu_mul_w); p.setBindGroup(1, LW.silu);
            p.dispatchWorkgroups(Math.ceil(D.inter / 64), w);
          }
          this._dQ(p, LW.qG, w);
          this._dW(p, LW.down, w);
        }
        p.end();
      }
      if (L.moe && this._wideGrp(L, w)) {
        // experts expert-grouped over the whole chunk (moeGroupPrefill on too): route each sub-batch, then one
        // sort + grouped gate/up + grouped down + combine over all w tokens, as _prefillGrouped does per ubatch
        const gB = this.gB, ss = NC * this.moe.KS * 4, tx = this._wTwin.get(B.x.buf), txn = this._wTwin.get(B.xn.buf);
        if (LW.nrtW && this.wideRouter !== false) {   // moe_nrt + moe_route over all w columns: the same per-column kernels, 2 launches
          const p = enc.beginComputePass();
          this._dxyz(p, "moe_nrt", LW.nrtW, Math.ceil((this.moe.nExp + 1) / this.moe.nrt.ROWS), w, 1);
          this._dxyz(p, "moe_route", LW.routeW, w, 1, 1);
          p.end();
        } else for (let j = 0; j < J; j++) {
          this._wCopy(enc, B.xn, j, false);
          if (this.moe.nrt) this._wCopy(enc, B.x, j, false);   // moe_nrt norms x itself
          this._mcCommon = this.bgCommonW[j];
          const p = enc.beginComputePass();
          this._encMoeFfn(p, L, LB, M, NC, G, true);
          p.end();
          this._mcCommon = null;
          enc.copyBufferToBuffer(B.mSel, 0, gB.sel, j * ss, ss);
          enc.copyBufferToBuffer(B.mSelw, 0, gB.selw, j * ss, ss);
        }
        enc.copyBufferToBuffer(txn, 0, gB.XNW, 0, w * B.xn.stride);
        if (!LW.combW) enc.copyBufferToBuffer(tx, 0, gB.XW, 0, w * B.x.stride);
        if (L.off) this.experts.loadLayer(enc, L.offLayer);   // offloaded: the layer's whole expert set into the region
        const p = enc.beginComputePass();
        this._dxyz(p, "moe_gsort", gB.bgSort, 1, 1, 1);
        this._encGroupExperts(p, L, M, w, true);
        this._dxyz(p, "moe_combw", LW.combW || gB.bgComb, Math.ceil(D.dim / 64), w, 1);   // combW: straight into the chunk's residual
        p.end();
        if (!LW.combW) enc.copyBufferToBuffer(gB.XW, 0, tx, 0, w * B.x.stride);
      } else if (L.moe) for (let j = 0; j < J; j++) {   // experts: the batched MoE kernels, sub-batch by sub-batch
        if (L.off && j === 0) this.experts.loadLayer(enc, L.offLayer);   // (offloaded: on the whole-layer region)
        this._wCopy(enc, B.xn, j, false); this._wCopy(enc, B.x, j, false);
        this._mcCommon = this.bgCommonW[j];
        const p = enc.beginComputePass();
        this._encMoeFfn(p, L, LB, M, NC, G);
        p.end();
        this._mcCommon = null;
        this._wCopy(enc, B.x, j, true);
      }
    } finally { this._mcCommon = null; }
  }
  // Wide chunk: ids[i .. i + w) at this.pos. Leaves this.x = the last column's hidden, fills the
  // draft cache sub-batch by sub-batch exactly like the NC loop in prefillTokens.
  // wide chunk of w tokens whose fused MoE layer can take the expert-grouped kernels (moeGroupPrefill also on)
  _wideGrp(L, w) {
    return !!(L.fused && this.gB && this.moeGroup !== false && w <= this.gB.U && w % this.NC === 0 && w >= 2 * this.NC
      && this._wTwin.get(this.B.x.buf) && this._wTwin.get(this.B.xn.buf));
  }
  // Returns false (and switches wide prefill off, freeing what it built) when its buffers cannot be set
  // up on this device, so prefillTokens falls back to the batchCols-wide passes instead of failing.
  // xs (prefillHidden, a chain shard): the chunk's input hiddens (w * dim floats) instead of ids' embeddings;
  // fill = false: no draft-cache fill (a chain host fills it from the hiddens the chain returns)
  async _prefillWide(ids, i, w, { xs = null, fill = true } = {}) {
    if (!this.layerW) {
      try { this._initWide(); }
      catch (e) {
        console.warn("wide prefill off:", String(e.message || e).slice(0, 300));
        for (const b of [...(this._wTwin?.values() || []), this._wDq?.q, this._wDq?.d]) b?.destroy();
        this._wTwin = null; this._wDq = null; this.Wt = null; this.layerW = null; this.ubatch = 0; this.prefillWide = false; this.dp4aCfg = null; this.prefillDp4a = false;
        return false;
      }
    }
    const q = this.device.queue, NC = this.NC, basePos = this.pos, Wx = this.Wt.x;
    this._dp4aOn = !!(this.dp4aCfg && this.prefillDp4a !== false);
    for (let j = 0; j < w / NC; j++) q.writeBuffer(this.frameW[j], 0, new Uint32Array([basePos + j * NC, basePos + j * NC + 1, NC, 0]));
    q.writeBuffer(this.frameWD, 0, new Uint32Array([basePos, basePos + 1, w, 0]));
    const dim = this.dims.dim;
    for (let c = 0; c < w; c++) q.writeBuffer(Wx.buf, c * Wx.stride, xs ? xs.subarray(c * dim, (c + 1) * dim) : this._embedRowF32(ids[i + c]));
    const gB = this.gB;
    if (gB && this.layers.some((L) => this._wideGrp(L, w)) && gB.sortW !== w) {   // the grouped sort's pair count
      q.writeBuffer(gB.sortU, 0, new Uint32Array([w * this.moe.KS, ...gB.sortArgs])); gB.sortW = w;
    }
    let enc = this.device.createCommandEncoder();
    for (let l = 0; l < this.layers.length; l++) {
      if (l && l % WIDE_SUBMIT_LAYERS === 0) { q.submit([enc.finish()]); enc = this.device.createCommandEncoder(); }
      // a disk layer's experts into the region first (_encodeLayerWide's loadLayer is then a no-op): the read is async,
      // and a writeBuffer inside this encoder would land before the earlier layers' kernels in it ran
      const Lw = this.layers[l];
      if (Lw.off && this.experts.isDisk(Lw.offLayer)) enc = await this._regionLoadDisk(enc, Lw.offLayer);
      this._encodeLayerWide(enc, l, basePos, w);
    }
    enc.copyBufferToBuffer(Wx.buf, (w - 1) * Wx.stride, this.x, 0, this.dims.dim * 4);
    q.submit([enc.finish()]);
    if (fill && this.mtp && this.mtpFill !== false && !this._mtpFillWide(ids, i, basePos, w)) for (let j = 0; j < w / NC; j++) {
      const e = this.device.createCommandEncoder();   // the sub-batch's final hiddens into B.x, as after an NC pass
      this._wCopy(e, this.B.x, j, false);
      q.submit([e.finish()]);
      const i0 = i + j * NC, b0 = basePos + j * NC;
      if (this.mtpBatchFill !== false) this._mtpFillBatch(ids, i0, b0, NC);
      else for (let c = 0; c < NC; c++) if (i0 + c + 1 < ids.length) await this.mtpRun(c, ids[i0 + c + 1], b0 + c + 1, false);
    }
    this.pos += w;
    await q.onSubmittedWorkDone();
  }

  async _runBatchAndRead(basePos, n = this.NC) {
    const { dim } = this.dims;
    let enc = this.device.createCommandEncoder();
    for (let l = 0; l < this.layers.length; l++) { this._encodeLayerBatch(enc, l, basePos, n); if (this._cut) enc = await this._cutResolve(enc); }
    for (let c = 0; c < n; c++) enc.copyBufferToBuffer(this.B.x.buf, c * this.B.x.stride, this.stageXB, c * dim * 4, dim * 4);
    this.device.queue.submit([enc.finish()]);
    await this.stageXB.mapAsync(GPUMapMode.READ, 0, n * dim * 4);
    const out = Float32Array.from(new Float32Array(this.stageXB.getMappedRange(0, n * dim * 4), 0, n * dim));
    this.stageXB.unmap();
    this.pos = basePos + n;
    return out;
  }
  // ids.length columns (1..NC). snapshot: save recurrent state after every
  // non-final column so restoreDN(k) can undo a rejected speculative suffix.
  _snapWord(snapshot, n) {   // frame.snap: slot base + 1 | total << 8 | replay (bit 31); records the verify chunk for the encoder
    if (!snapshot) { this._snapNow = null; return 0; }
    const base = typeof snapshot === "object" ? snapshot.base : 0, total = typeof snapshot === "object" ? snapshot.total : n;
    this._snapNow = { base, total };
    return (((total << 8) | (base + 1)) | (this.replay ? 0x80000000 : 0)) >>> 0;
  }
  async embedRunBatch(ids, basePos, snapshot = false) {
    if (!this.B) this._initBatch();
    const n = ids.length;
    this.pos = basePos;
    const sp = this._snapWord(snapshot, n);
    for (let c = 0; c < n; c++) {
      this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1, n, sp]));
      this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, this._embedRowF32(ids[c]));
    }
    return this._runBatchAndRead(basePos, n);
  }
  async runHiddenBatch(xs, basePos, snapshot = false) {
    if (!this.B) this._initBatch();
    const { dim } = this.dims;
    const n = xs.length / dim;
    this.pos = basePos;
    const sp = this._snapWord(snapshot, n);
    for (let c = 0; c < n; c++) {
      this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1, n, sp]));
      this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, xs.subarray(c * dim, (c + 1) * dim));
    }
    return this._runBatchAndRead(basePos, n);
  }
  // (a no-op before the first batched pass: nothing was verified, so there is nothing to roll back)
  restoreDN(k) { if (this.B) this._restoreDN(k); }
  setHidden(h) { this._pre = null; this.device.queue.writeBuffer(this.x, 0, h); }   // final trunk hidden (chain host) for the draft head

  // final norm + LM head for n hidden states (n*dim floats, or null to use the
  // columns already in B.x) -> array of n logits vectors. Leaves the hiddens
  // in B.x so the draft head can read them by column.
  async headBatch(hs, n = hs ? hs.length / this.dims.dim : this.NC) {
    if (!this.B) this._initBatch();
    const { dim, vocab } = this.dims;
    if (hs) for (let c = 0; c < n; c++) this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, hs.subarray(c * dim, (c + 1) * dim));
    this.device.queue.writeBuffer(this.frameBufsB[0], 0, new Uint32Array([this.pos, this.pos + 1, n, 0]));
    const enc = this.device.createCommandEncoder();
    const p = enc.beginComputePass();
    this._dMC(p, "rmsnorm_mc", this.bgFinalNormMC, 256, 256, n);
    this._dop(p, this.headB, n);
    p.end();
    for (let c = 0; c < n; c++) enc.copyBufferToBuffer(this.B.logits.buf, c * this.B.logits.stride, this.stageLogitsN, c * vocab * 4, vocab * 4);
    this.device.queue.submit([enc.finish()]);
    await this.stageLogitsN.mapAsync(GPUMapMode.READ, 0, n * vocab * 4);
    const all = new Float32Array(this.stageLogitsN.getMappedRange(0, n * vocab * 4)).slice();
    this.stageLogitsN.unmap();
    const out = [];
    for (let c = 0; c < n; c++) out.push(all.subarray(c * vocab, (c + 1) * vocab));
    return out;
  }

  // ---- GPU sampling ----
  // The sampler's GPU descriptor ({ kind: "greedy" } | { kind: "topk", k, temp }) when this engine
  // samples on the GPU, else null (the caller gets full logits). A sampler that carries .gpu must
  // accept both a logits vector and a candidates object { ids, vals, bad } (room/sampling.js).
  gpuDescFor(sample) { return this.gpuSample && this.hasHead && sample && sample.gpu ? sample.gpu : null; }
  // bind groups and grid of a top-k over the columns of `src` (n logits each, column stride in
  // floats) into `out` (2k + 2 u32 per column), through the shared stage-a partials (tkPart)
  _tkOp(key, src, n, stride, k, out) {
    const id = key + ":" + n + ":" + k;
    let op = this._tkOps.get(id);
    if (!op) {
      const nw = Math.ceil(n / 4096);
      const uA = this._buf(new Uint32Array([n, stride, k, nw]), GPUBufferUsage.UNIFORM);
      const uB = this._buf(new Uint32Array([nw, k, 0, 0]), GPUBufferUsage.UNIFORM);
      op = { k, nw, R: 2 * k + 2,
        bgA: this._bg(this.pipes.topk_a, 1, [src, this.tkPart, uA]),
        bgB: this._bg(this.pipes.topk_b, 1, [this.tkPart, out, uB]) };
      this._tkOps.set(id, op);
    }
    return op;
  }
  _dTopk(pass, op, cols = 1) {
    this._dxyz(pass, "topk_a", op.bgA, op.nw, cols, 1);
    this._dxyz(pass, "topk_b", op.bgB, 1, cols, 1);
  }
  // the draft head's argmax into argBuf ([idx, bits, ...], idx read by emb_gather and the host):
  // two-stage multi-workgroup (argmaxWide) or the old single-workgroup kernel
  _dArgmax(p, small) {
    if (this.argmaxWide) this._dTopk(p, this._tkOp(small ? "argD" : "arg", this.logits, small ? this.draftVocab : this.dims.vocab, 0, 1, this.argBuf));
    else this._d(p, "argmax", small ? this.bgArgmaxDraft : this.bgArgmax, 256, 256);
  }
  // final norm + LM head + top-k of this.x (one column) into topBuf, copied to stageTop
  _encodeHeadIds(enc, desc) {
    const op = this._tkOp("one", this.logits, this.dims.vocab, 0, topkK(desc), this.topBuf);
    const p = enc.beginComputePass();
    this._d(p, "rmsnorm", this.bgFinalNorm, 256, 256);
    this._dop(p, this.headOp);
    this._dTopk(p, op, 1);
    p.end();
    enc.copyBufferToBuffer(this.topBuf, 0, this.stageTop, 0, op.R * 4);
    return op;
  }
  async _readTop(op) {
    await this.stageTop.mapAsync(GPUMapMode.READ, 0, op.R * 4);
    const c = readCands(new Uint32Array(this.stageTop.getMappedRange(0, op.R * 4)), 0, op.k);
    this.stageTop.unmap();
    return c;
  }
  // headFromHidden with the sampling on the GPU: 16 B (greedy) or 8k + 8 B (top-k) back instead of
  // vocab * 4. -> { ids, vals, bad }: ids/vals the top pairs, value descending (index ascending on
  // ties); bad the column's count of non-finite logits (NaN / Inf), so callers keep their NaN check.
  async headFromHiddenIds(xIn, desc = { kind: "greedy" }) {
    this._pre = null;
    this.device.queue.writeBuffer(this.x, 0, xIn);
    const enc = this.device.createCommandEncoder();
    const op = this._encodeHeadIds(enc, desc);
    this.device.queue.submit([enc.finish()]);
    return await this._readTop(op);
  }
  // forwardToken with the sampling on the GPU (see headFromHiddenIds); encode-ahead as forwardToken.
  // Decode-ahead (greedy, from the second call in a row on): token N + 1 is submitted right behind token N, its
  // embedding gathered on the GPU from N's top-1 (emb_gather, bit-exact with _embedRowF32), so the GPU never waits
  // for the readback, the JS and the next submit between tokens (~1.8 ms a MoE token in Chrome, ~12 ms in Deno).
  // The step saves the recurrent state (DeltaNet S and conv window) and this.x first, in the same command buffer.
  // The next call keeps it when its token is that top-1; any other token, and any other engine call
  // (_settleAhead), puts the state back (the KV row it wrote is rewritten before anything reads it).
  // Same kernels, same inputs: the same bits. engine.decodeAhead = false turns it off.
  async forwardTokenIds(tokenId, desc = { kind: "greedy" }) {
    this._pre = null;
    const A = this._ahead, key = this._fwdKey(desc);
    this._ahead = null;
    let job;
    if (A && A.pos === this.pos && A.tok === tokenId && A.key === key) job = A.job;   // already queued
    else {
      if (A) this._dropAhead(A);
      this._setFrame(this.pos, this.pos + 1);
      this.device.queue.writeBuffer(this.x, 0, this._embedRowF32(tokenId));
      const pre = this._fwdPre;
      this._fwdPre = null;
      job = this.experts ? await this._encodeForwardOff(this.pos, desc) : pre && pre.pos === this.pos && pre.key === key ? pre : this._encodeForward(this.pos, desc);
      this.device.queue.submit([job.cb]);
    }
    const bytes = job.op.R * 4;
    const mapped = mapRead(this.device, job.stage, 0, bytes, this._kick());   // (resident: no kick, see map_kick.js)
    let next = null;
    if (this._fwdRun > 0 && this._canDecodeAhead(desc)) {
      this._setFrame(this.pos + 1, this.pos + 2);
      next = this._encodeForward(this.pos + 1, desc, true);
      this.device.queue.submit([next.cb]);
    } else if (this.encodeAhead !== false && !this.experts && this.pos + 1 < this.maxSeq) this._fwdPre = this._encodeForward(this.pos + 1, desc);
    this._fwdRun = (this._fwdRun || 0) + 1;
    await mapped;
    const c = readCands(new Uint32Array(job.stage.getMappedRange(0, bytes)), 0, job.op.k);
    job.stage.unmap();
    this.pos++;
    // the queued step gathered the top-1's embedding: kept only if that id is in the gather table
    if (next) this._ahead = { pos: this.pos, tok: !c.bad && c.ids[0] < this._eg.rows ? c.ids[0] : -1, job: next, key };
    return c;
  }
  _canDecodeAhead(desc) {
    return this.decodeAhead !== false && !this.experts && desc && desc.kind === "greedy" && topkK(desc) === 1 && !!this._eg && this.hasEmbed && this.hasHead
      && this.flash && !!this.pipes.emb_gather && this.pos + 2 < this.maxSeq;
  }
  // buffers a step run ahead saves the recurrent state and this.x into (shared with headAhead: never both at once)
  _aheadBufs() {
    if (this._egX) return;
    const dev = this.device, S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST, E = this._eg, dim = this.dims.dim;
    for (const L of this.layers) if (!L.isFull) {
      L.S_ahead = dev.createBuffer({ size: L.S.size, usage: S });
      L.conv_ahead = dev.createBuffer({ size: L.convState.size, usage: S });
    }
    this.xAhead = dev.createBuffer({ size: dim * 4, usage: S });
    this._egX = this._bg2(this.pipes.emb_gather, [{ buffer: E.qs }, { buffer: E.sc }, { buffer: this.topBuf, offset: 0, size: 16 },
      { buffer: this.x, offset: 0, size: dim * 4 }, { buffer: E.u }]);
  }
  _saveAhead(enc, x = false) {
    for (const L of this.layers) if (!L.isFull) {
      enc.copyBufferToBuffer(L.S, 0, L.S_ahead, 0, L.S.size);
      enc.copyBufferToBuffer(L.convState, 0, L.conv_ahead, 0, L.convState.size);
    }
    if (x) enc.copyBufferToBuffer(this.x, 0, this.xAhead, 0, this.dims.dim * 4);
  }
  _restoreAhead(x = false) {
    const enc = this.device.createCommandEncoder();
    for (const L of this.layers) if (!L.isFull) {
      enc.copyBufferToBuffer(L.S_ahead, 0, L.S, 0, L.S.size);
      enc.copyBufferToBuffer(L.conv_ahead, 0, L.convState, 0, L.convState.size);
    }
    if (x) enc.copyBufferToBuffer(this.xAhead, 0, this.x, 0, this.dims.dim * 4);
    this.device.queue.submit([enc.finish()]);
  }
  // a queued decode-ahead step is not wanted: the state (and this.x) as they were before it
  _dropAhead(A) { this._restoreAhead(true); this._fwdRun = 0; }
  // every engine entry point but forwardTokenIds runs this first (see the wrappers in create)
  _settleAhead() {
    this._fwdRun = 0;
    const A = this._ahead;
    if (!A) return;
    this._ahead = null;
    this._dropAhead(A);
  }
  // headBatch with the sampling on the GPU: an array of n candidates objects
  async headBatchIds(hs, n = hs ? hs.length / this.dims.dim : this.NC, desc = { kind: "greedy" }) {
    if (!this.B) this._initBatch();
    const { dim, vocab } = this.dims;
    if (hs) for (let c = 0; c < n; c++) this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, hs.subarray(c * dim, (c + 1) * dim));
    this.device.queue.writeBuffer(this.frameBufsB[0], 0, new Uint32Array([this.pos, this.pos + 1, n, 0]));
    const op = this._tkOpB(desc);
    const enc = this.device.createCommandEncoder();
    const p = enc.beginComputePass();
    this._dMC(p, "rmsnorm_mc", this.bgFinalNormMC, 256, 256, n);
    this._dop(p, this.headB, n);
    this._dTopk(p, op, n);
    p.end();
    enc.copyBufferToBuffer(this.topBuf, 0, this.stageTopN, 0, n * op.R * 4);
    this.device.queue.submit([enc.finish()]);
    await this.stageTopN.mapAsync(GPUMapMode.READ, 0, n * op.R * 4);
    const u = new Uint32Array(this.stageTopN.getMappedRange(0, n * op.R * 4));
    const out = [];
    for (let c = 0; c < n; c++) out.push(readCands(u, c * op.R, op.k));
    this.stageTopN.unmap();
    return out;
  }
  _tkOpB(desc) { return this._tkOp("B", this.B.logits.buf, this.dims.vocab, this.B.logits.stride / 4, topkK(desc), this.topBuf); }

  // draft with the reduced-vocabulary head? (see draftVocabAuto in _init)
  _smallHead() { return !!this.headOpDraft && (!this.draftVocabAuto || this._dvSmall); }
  _noteDV(ids) {
    if (!this.headOpDraft || !this.draftVocabAuto) return;
    const dv = this.draftVocab;
    let m = this._dvMiss;
    const skip = this._dvSkip;
    for (const t of ids) { if (skip && t >= dv && skip[t - dv]) continue; m += ((t >= dv ? 1 : 0) - m) / 32; }
    this._dvMiss = m;
    if (this._dvSmall && m > 0.05) this._dvSmall = false;        // hysteresis: off above 5%,
    else if (!this._dvSmall && m < 0.025) this._dvSmall = true;  // back on below 2.5%
  }

  // ---- speculative decoding with the MTP head ----
  // Run the draft block for the token `tNext` (at position `pos`) given the
  // trunk hidden of the previous position: srcCol === null reads this.x,
  // otherwise batch column srcCol. Appends to the MTP layer's own KV cache.
  // wantLogits -> returns draft logits (argmax = drafted token).
  async mtpRun(srcCol, tNext, pos, wantLogits) {
    for (let g = this.offGate(); g; g = this.offGate()) await g;   // (expert offload: not inside another call's cut window)
    this._pre = null; return this._mtpRun(srcCol, tNext, pos, wantLogits);
  }
  async _mtpRun(srcCol, tNext, pos, wantLogits) {
    const M2 = this.mtp, { dim, vocab } = this.dims;
    this.device.queue.writeBuffer(M2.emb, 0, this._embedRowF32(tNext));
    this._setFrame(pos, pos + 1);
    const enc = this.device.createCommandEncoder();
    {
      const p = enc.beginComputePass();
      this._d(p, "rmsnorm", M2.bgENorm, 256, 256);
      this._d(p, "rmsnorm", srcCol === null ? M2.bgHNormX : M2.bgHNormB[srcCol], 256, 256);
      this._dop(p, M2.proj);
      p.end();
    }
    this._encodeLayerR(enc, this.mtpLayer, pos);
    if (wantLogits) {
      const p = enc.beginComputePass();
      this._d(p, "rmsnorm", M2.bgHeadNorm, 256, 256);
      const small = wantLogits === "argmax" && this._smallHead();
      this._dop(p, small ? this.headOpDraft : this.headOp);
      if (wantLogits === "argmax") this._dArgmax(p, small);
      p.end();
    }
    if (wantLogits === "argmax") enc.copyBufferToBuffer(this.argBuf, 0, this.stageArg, 0, 16);
    this.device.queue.submit([enc.finish()]);
    if (!wantLogits) return null;
    if (wantLogits === "argmax") {
      await this.stageArg.mapAsync(GPUMapMode.READ);
      const id = new Uint32Array(this.stageArg.getMappedRange())[0];
      this.stageArg.unmap();
      return id;
    }
    return await this._readback(this.logits, this.stageLogits, vocab);
  }

  // verify tokens[k] at positions pos+k (any n, run in chunks of NC): trunk (local batched
  // pass with DeltaNet snapshots, or a caller-supplied runTrunk for a device
  // chain) then one batched head pass -> logits per column.
  // desc (a GPU sampling descriptor, see gpuDescFor): candidates per column instead of logits
  async verifyN(tokens, pos, runTrunk = null, desc = null) {
    const n = tokens.length, { dim } = this.dims;
    let hs;
    if (runTrunk) hs = await runTrunk(tokens, pos);
    else if (n <= this.NC) hs = await this.embedRunBatch(tokens, pos, true);
    else {
      hs = new Float32Array(n * dim);
      for (let c0 = 0; c0 < n; c0 += this.NC) {
        const m = Math.min(this.NC, n - c0);
        hs.set(await this.embedRunBatch(tokens.slice(c0, c0 + m), pos + c0, { base: c0, total: n }), c0 * dim);
      }
    }
    const lgs = [];
    for (let c0 = 0; c0 < n; c0 += this.NC)
      lgs.push(...await (desc ? this.headBatchIds(hs.subarray(c0 * dim, Math.min(n, c0 + this.NC) * dim), Math.min(this.NC, n - c0), desc)
        : this.headBatch(hs.subarray(c0 * dim, Math.min(n, c0 + this.NC) * dim), Math.min(this.NC, n - c0))));
    return { lgs, hs };
  }
  _restoreDN(k) {   // recurrent state as it was after verify column k
    // one pass for every layer: replay columns 0..k from S_pre (snap bit 30), conv state <- slot k
    const enc = this.device.createCommandEncoder(), q = this.device.queue, D = this.dims;
    q.writeBuffer(this.frameBufsB[0], 0, new Uint32Array([0, 1, k + 1, this.replay ? 0x40000000 : 0]));
    if (!this.replay) for (const L of this.layers) if (!L.isFull) enc.copyBufferToBuffer(L.S_shadow, k * L.S.size, L.S, 0, L.S.size);
    const p = enc.beginComputePass();
    for (let i = 0; i < this.layers.length; i++) {
      if (this.layers[i].isFull) continue;
      const M = this.layerB[i].mc;
      if (this.replay) this._dMC(p, "dn_delta_mc", M.replay, D.nVH * 128, 128, 1);
      this._dMC(p, "dn_conv_restore", M.convRestore, this.layers[i].convState.size / 4, 64, 1);
    }
    p.end();
    q.submit([enc.finish()]);
    q.writeBuffer(this.frameBufsB[0], 0, new Uint32Array([0, 1, k + 1, 0]));   // no later pass may see bit 30
  }
  _adoptHidden(col) {   // batch column -> this.x (the hidden the next draft reads)
    const enc = this.device.createCommandEncoder();
    enc.copyBufferToBuffer(this.B.x.buf, col * this.B.x.stride, this.x, 0, this.dims.dim * 4);
    this.device.queue.submit([enc.finish()]);
  }

  // the K drafts of specStep in one submit (see draftChain in _init); same kernels, same inputs
  async _draftChain(tNext, pos, K) {
    this.device.queue.writeBuffer(this.mtp.emb, 0, this._embedRowF32(tNext));
    const enc = this.device.createCommandEncoder();
    this._encodeDraftChain(enc, pos, K, this.stageArgK, 0, false);
    this.device.queue.submit([enc.finish()]);
    await this.stageArgK.mapAsync(GPUMapMode.READ, 0, K * 16);
    const a = new Uint32Array(this.stageArgK.getMappedRange(0, K * 16));
    const drafts = Array.from({ length: K }, (_, k) => a[k * 4]);
    this.stageArgK.unmap();
    return drafts;
  }
  // Encode the K draft steps (M2.emb must already hold tNext's embedding). Draft k's argmax goes
  // to stage at stageOff + 16k. toB: also gather draft k's embedding into verify column k + 1 of
  // B.x (the fused step's trunk input), from the same argBuf value the host reads back.
  _encodeDraftChain(enc, pos, K, stage, stageOff, toB) {
    const M2 = this.mtp, small = !!this.headOpDraft;
    for (let k = 0; k < K; k++) this.device.queue.writeBuffer(this.stepFrames[k], 0, new Uint32Array([pos + k, pos + k + 1]));
    try {
      for (let k = 0; k < K; k++) {
        this._common = this.bgCommonStep[k];
        const p = enc.beginComputePass();
        if (k > 0) this._d(p, "emb_gather", this.bgEmbGather, this.dims.dim);
        this._d(p, "rmsnorm", M2.bgENorm, 256, 256);
        this._d(p, "rmsnorm", M2.bgHNormX, 256, 256);
        this._dop(p, M2.proj);
        p.end();
        this._encodeLayerR(enc, this.mtpLayer, pos + k);
        const p2 = enc.beginComputePass();
        this._d(p2, "rmsnorm", M2.bgHeadNorm, 256, 256);
        this._dop(p2, small ? this.headOpDraft : this.headOp);
        this._dArgmax(p2, small);
        p2.end();
        enc.copyBufferToBuffer(this.argBuf, 0, stage, stageOff + k * 16, 16);
        if (toB) {
          const p3 = enc.beginComputePass();
          this._d(p3, "emb_gather", this._egB(k + 1), this.dims.dim);
          p3.end();
        }
      }
    } finally { this._common = null; }
  }
  _egB(c) {   // emb_gather bind group writing batch column c of B.x
    this._egBG = this._egBG || [];
    if (!this._egBG[c]) {
      const E = this._eg, X = this.B.x;
      this._egBG[c] = this._bg2(this.pipes.emb_gather, [{ buffer: E.qs }, { buffer: E.sc }, { buffer: this.argBuf },
        { buffer: X.buf, offset: c * X.stride, size: this.dims.dim * 4 }, { buffer: E.u }]);
    }
    return this._egBG[c];
  }

  // One-submit speculative verify (specFuse). tokens: [tNext, ...drafts], or chainK > 0 to draft
  // tNext's K continuations in the same command buffer (then tokens = [tNext] and the drafts come
  // back with the logits). Encodes exactly what the separate path runs, in the same order:
  //   [draft chain] -> embedRunBatch's trunk pass (snapshots on) -> headBatch's norm + LM head
  // and differs only in where the data waits: the verify columns' embeddings for the drafts are
  // gathered on the GPU (emb_gather, bit-exact with _embedRowF32), the trunk hiddens stay in B.x
  // (the separate path reads them to the CPU and writes the same bytes back for the head), and one
  // mapAsync returns the logits plus the drafts. Returns { lgs, drafts }; B.x keeps the hiddens.
  // desc (GPU sampling): the head's top-k runs in the same pass and the map shrinks from
  // n * vocab * 4 + 128 B to n * (8k + 8) + 128 B; lgs are then candidates objects.
  async _verifyFusedIds(tokens, pos, chainK = 0, desc = { kind: "greedy" }) { return this._verifyFused(tokens, pos, chainK, desc); }
  async _verifyFused(tokens, pos, chainK = 0, desc = null) {
    const { dim, vocab } = this.dims, n = tokens.length + chainK, q = this.device.queue;
    if (chainK) q.writeBuffer(this.mtp.emb, 0, this._embedRowF32(tokens[0]));
    this.pos = pos;
    const sp = this._snapWord(true, n);
    for (let c = 0; c < n; c++) q.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([pos + c, pos + c + 1, n, sp]));
    for (let c = 0; c < tokens.length; c++) q.writeBuffer(this.B.x.buf, c * this.B.x.stride, this._embedRowF32(tokens[c]));
    let enc = this.device.createCommandEncoder();
    const op = desc ? this._tkOpB(desc) : null, stage = op ? this.stageTopN : this.stageLogitsN;
    const tail = op ? n * op.R * 4 : n * vocab * 4;   // drafts go right after the n logits rows (or candidate rows)
    if (chainK) this._encodeDraftChain(enc, pos, chainK, stage, tail, true);
    // verifyPass: the trunk's layers and the head in one compute pass instead of 3 per layer (as layerFuse.pass does
    // for one token): the same dispatches in the same order, each its own usage scope, so the same bits
    this._ppB = this.verifyPass !== false && this.flash ? enc.beginComputePass() : null;
    for (let l = 0; l < this.layers.length; l++) { this._encodeLayerBatch(enc, l, pos, n); if (this._cut) enc = await this._cutResolve(enc); }
    const p = this._ppB || enc.beginComputePass();
    this._ppB = null;
    this._dMC(p, "rmsnorm_mc", this.bgFinalNormMC, 256, 256, n);
    this._dop(p, this.headB, n);
    if (op) this._dTopk(p, op, n);
    p.end();
    if (op) enc.copyBufferToBuffer(this.topBuf, 0, stage, 0, tail);
    else for (let c = 0; c < n; c++) enc.copyBufferToBuffer(this.B.logits.buf, c * this.B.logits.stride, stage, c * vocab * 4, vocab * 4);
    q.submit([enc.finish()]);
    const bytes = tail + chainK * 16;
    await stage.mapAsync(GPUMapMode.READ, 0, bytes);
    const m = stage.getMappedRange(0, bytes);
    const lgs = [];
    if (op) { const u = new Uint32Array(m, 0, tail / 4); for (let c = 0; c < n; c++) lgs.push(readCands(u, c * op.R, op.k)); }
    else { const all = new Float32Array(m, 0, n * vocab).slice(); for (let c = 0; c < n; c++) lgs.push(all.subarray(c * vocab, (c + 1) * vocab)); }
    const ids = new Uint32Array(m, tail, chainK * 4);
    const drafts = Array.from({ length: chainK }, (_, k) => ids[k * 4]);
    stage.unmap();
    this.pos = pos + n;
    return { lgs, drafts };
  }
  // ---- a chain host's share of a lap in one submit (hostFuse) ----
  // A room's host holds the embedding, the first layers and the head, so every lap it ran up to three
  // GPU round trips of its own (submit, wait, read back) with the CPU idle in between. These put
  // them into one command buffer with one readback. Same kernels, same inputs, same order as the
  // separate calls, so the same bits; only where the data waits changes. Both need the GPU
  // embedding table of the draft chain (emb_gather, bit-exact with _embedRowF32).
  _canHostFuse(n) {
    if (this.hostFuse === false || !this.draftChain || !this._eg || !this.hasHead || !this.hasEmbed || n > this.NC) return false;
    if (!this.B) this._initBatch();
    return true;
  }
  // Speculative step, chain host: the K drafts of tNext (the draft chain), their embeddings gathered
  // into verify columns 1..K and this device's layers over [tNext, ...drafts] with the verify
  // snapshots, as _draftChain then embedRunBatch(tokens, pos, true) run them -> { drafts, hs }.
  async _hostTrunkFused(tNext, pos, K) {
    const { dim } = this.dims, n = K + 1, q = this.device.queue;
    q.writeBuffer(this.mtp.emb, 0, this._embedRowF32(tNext));
    this.pos = pos;
    const sp = this._snapWord(true, n);
    for (let c = 0; c < n; c++) q.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([pos + c, pos + c + 1, n, sp]));
    q.writeBuffer(this.B.x.buf, 0, this._embedRowF32(tNext));
    const stage = this.stageHF ||= this.device.createBuffer({ size: this.NC * dim * 4 + 128, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    const tail = n * dim * 4;   // the drafts go right after the hiddens
    let enc = this.device.createCommandEncoder();
    this._encodeDraftChain(enc, pos, K, stage, tail, true);
    for (let l = 0; l < this.layers.length; l++) { this._encodeLayerBatch(enc, l, pos, n); if (this._cut) enc = await this._cutResolve(enc); }
    for (let c = 0; c < n; c++) enc.copyBufferToBuffer(this.B.x.buf, c * this.B.x.stride, stage, c * dim * 4, dim * 4);
    q.submit([enc.finish()]);
    const bytes = tail + K * 16;
    await stage.mapAsync(GPUMapMode.READ, 0, bytes);
    const m = stage.getMappedRange(0, bytes);
    const hs = Float32Array.from(new Float32Array(m, 0, n * dim));
    const ids = new Uint32Array(m, tail, K * 4);
    const drafts = Array.from({ length: K }, (_, k) => ids[k * 4]);
    stage.unmap();
    this.pos = pos + n;
    return { drafts, hs };
  }
  // Plain decoding, chain host, greedy: the final norm + LM head + top-1 of the hidden the chain
  // returned for pos - 1 (headFromHiddenIds), that id's embedding gathered into x, and this device's
  // layers at pos (embedRun) -> { cands, h }: h is embedRun(cands.ids[0], pos). The layers run before
  // the caller knows it keeps the token (it may be a stop token or past the answer cap), so the
  // recurrent state is saved first in the same buffer: keepAhead() keeps the step, dropAhead() puts
  // the state back (the KV rows at pos are rewritten before anything reads them). The gather table
  // may hold only the first draftVocab rows: a rarer pick returns h = null with the state already
  // put back (the caller runs embedRun).
  canHeadAhead() { return this.hostFuse !== false && !!this._eg && this.hasHead && this.hasEmbed && !!this.bgFinalNorm; }
  async headAhead(xIn, pos, desc = { kind: "greedy" }) {
    this._pre = null;
    const { dim, vocab } = this.dims, dev = this.device;
    if (!this.stageAhead) {
      this._aheadBufs();
      this.stageAhead = dev.createBuffer({ size: 16 + dim * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    }
    const op = this._tkOp("one", this.logits, vocab, 0, 1, this.topBuf);   // [idx, bits, bad, 0]
    this.pos = pos;
    this._setFrame(pos, pos + 1);
    dev.queue.writeBuffer(this.x, 0, xIn);
    let enc = dev.createCommandEncoder();
    {
      const p = enc.beginComputePass();
      this._d(p, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dop(p, this.headOp);
      this._dTopk(p, op, 1);
      p.end();
    }
    {
      const p = enc.beginComputePass();
      this._d(p, "emb_gather", this._egX, dim);
      p.end();
    }
    this._saveAhead(enc);
    for (let i = 0; i < this.layers.length; i++) { this._encodeLayer(enc, i); if (this._cut) enc = await this._cutResolve(enc); }
    enc.copyBufferToBuffer(this.topBuf, 0, this.stageAhead, 0, 16);
    enc.copyBufferToBuffer(this.x, 0, this.stageAhead, 16, dim * 4);
    dev.queue.submit([enc.finish()]);
    this._aheadAt = pos;
    await this.stageAhead.mapAsync(GPUMapMode.READ, 0, 16 + dim * 4);
    const m = this.stageAhead.getMappedRange(0, 16 + dim * 4);
    const cands = readCands(new Uint32Array(m, 0, 4), 0, op.k);
    const h = Float32Array.from(new Float32Array(m, 16, dim));
    this.stageAhead.unmap();
    if (!(cands.ids[0] < this._eg.rows)) { this.dropAhead(); return { cands, h: null }; }
    return { cands, h };
  }
  keepAhead() { this._aheadAt = null; }
  dropAhead() {
    if (this._aheadAt == null) return;
    this._aheadAt = null;
    this._restoreAhead();
  }
  _canFuse(n, runTrunk) {   // solo verify that fits one batch pass
    if (this.specFuse === false || runTrunk || n > this.NC || !this.hasHead || !this.hasEmbed) return false;
    if (!this.B) this._initBatch();
    return !!this.bgFinalNormMC;
  }

  // One speculative step with K chained drafts (K up to maxDrafts). Precondition:
  // this.x holds the trunk hidden of the previous position and `tNext` is the
  // already-sampled token for this.pos. Returns 1..K+1 new tokens.
  // runTrunk(tokens, pos) -> hidden states for all columns (chain mode);
  // onReject(k) tells the other devices to roll their recurrent state back
  // to what it was after column k.
  // preTrunk: runTrunk takes a third argument { hs, t0 }: this device's layers already ran (drafts
  // and layers in one submit, _hostTrunkFused) and hs holds their hiddens, to send on as they are.
  async specStep(tNext, sample, K = 3, { runTrunk = null, onReject = null, preTrunk = false } = {}) {
    const pos = this.pos, M2 = this.mtp;
    K = Math.max(1, Math.min(7, K));
    // GPU sampling: lgs are candidates objects ({ ids, vals, bad }) that `sample` reads directly
    const desc = this.gpuDescFor(sample);
    // the chain gathers draft embeddings from a table of the first draftVocab rows, so it drafts with
    // the small head; while the draftvocab fallback wants the full head (e.g. a Chinese chat), the
    // drafts go through _mtpRun instead
    const chain = this.draftChain && this.chainOn !== false && (this._eg.rows >= this.dims.vocab || this._smallHead());
    let drafts = [], lgs, hs = null;
    // the previous step may already have run the draft block for (tNext, pos) (see _mtpRefill)
    const pre = chain ? this._takePre(-1, -1) : this._takePre(tNext, pos);
    if (chain && this._canFuse(K + 1, runTrunk)) ({ lgs, drafts } = await this._verifyFused([tNext], pos, K, desc));
    else if (chain && runTrunk && preTrunk && this._canHostFuse(K + 1)) {
      // chain host: drafts + its layers in one submit; runTrunk gets the hiddens and sends them on
      const t0 = performance.now();
      const r = await this._hostTrunkFused(tNext, pos, K);
      drafts = r.drafts;
      ({ lgs, hs } = await this.verifyN([tNext, ...drafts], pos, (tokens, p) => runTrunk(tokens, p, { hs: r.hs, t0 }), desc));
    } else {
      const d0 = pre ? await this._preDraft0(pre) : null;
      if (d0 !== null) drafts.push(d0);   // this.x now holds the draft block's output for column 0
      if (chain) drafts = await this._draftChain(tNext, pos, K);
      else for (let k = drafts.length; k < K; k++) {
        // after the first call this.x holds the MTP block's own output hidden,
        // which is what chained drafting feeds back in
        drafts.push(await this._mtpRun(null, k === 0 ? tNext : drafts[k - 1], pos + k, "argmax"));
      }
      if (this._canFuse(K + 1, runTrunk)) ({ lgs } = await this._verifyFused([tNext, ...drafts], pos, 0, desc));
      else ({ lgs, hs } = await this.verifyN([tNext, ...drafts], pos, runTrunk, desc));
    }
    const out = [];
    let a = 0;   // accepted drafts
    for (let k = 0; k <= K; k++) {
      const t = sample(lgs[k]);
      out.push(t);
      if (k < K && t === drafts[k]) a++; else break;
    }
    M2.stats.drafts += K; M2.stats.accepted += a;
    this._noteDV(out);
    if (a < K) { this._restoreDN(a); if (onReject) await onReject(a); }
    // re-fill the draft cache for the accepted positions with exact trunk hiddens (and run the
    // next step's first draft in the same pass); hs = null: the hiddens are still in B.x (fused verify)
    // (the one-submit chain recomputes that column itself, so it gets the refill alone)
    await this._refillDrafts(out, hs, pos, a, chain ? "none" : "head");
    this.pos = pos + a + 1;
    return out;
  }

  // specStep with drafts from the caller (prompt lookup: tokens that followed the same n-gram
  // earlier in the context) instead of the draft head. Same verify, same acceptance rule, same
  // rollback, so the output is exactly plain decoding's. The draft head's cache still gets its
  // rows: tNext's before the verify (this.x holds the trunk hidden of the previous position, as in
  // specStep), the accepted ones after it from the exact trunk hiddens.
  async specStepDrafts(tNext, sample, drafts, { runTrunk = null, onReject = null } = {}) {
    const pos = this.pos, M2 = this.mtp, { dim } = this.dims;
    const K = Math.max(1, Math.min(this.maxDrafts || 7, drafts.length));
    drafts = drafts.slice(0, K);
    const desc = this.gpuDescFor(sample);   // GPU sampling (see specStep)
    // tNext's draft-cache row: already written if the previous step ran its draft block column
    if (M2 && !this._takePre(tNext, pos)) await this._mtpRun(null, tNext, pos, false);
    let lgs, hs = null;
    if (this._canFuse(K + 1, runTrunk)) ({ lgs } = await this._verifyFused([tNext, ...drafts], pos, 0, desc));
    else ({ lgs, hs } = await this.verifyN([tNext, ...drafts], pos, runTrunk, desc));
    const out = [];
    let a = 0;
    for (let k = 0; k <= K; k++) {
      const t = sample(lgs[k]);
      out.push(t);
      if (k < K && t === drafts[k]) a++; else break;
    }
    this.lookupStats = this.lookupStats || { drafts: 0, accepted: 0 };
    this.lookupStats.drafts += K; this.lookupStats.accepted += a;
    this._noteDV(out);
    if (a < K) { this._restoreDN(a); if (onReject) await onReject(a); }
    // a lookup step is often followed by another one, so pre-run the next draft block column
    // (its cache row, which every next step needs) but not the head (only specStep needs that)
    if (M2) await this._refillDrafts(out, hs, pos, a, "row");
    else if (hs) this.setHidden(hs.subarray(a * dim, (a + 1) * dim));
    else this._adoptHidden(a);
    this.pos = pos + a + 1;
    return out;
  }

  // ---- post-verify draft-cache refill ----
  // After a verify that accepted `a` drafts, the draft block's cache rows pos+1 .. pos+a have to be
  // rewritten from the exact trunk hiddens hs[0 .. a-1] (drafting wrote them from the block's own
  // guesses), and the next step starts by running the draft block on (hs[a], out[a]) for row
  // pos+a+1. Column c of that work pairs hs[c] with out[c] and writes row pos+c+1, exactly what
  // _mtpFillBatch does for a prompt chunk, so all of it is one batched pass of a+1 columns instead
  // of a+1 single-column submits. With `head`, the same submit also runs the draft head on the last
  // column and starts the readback of the next step's first draft, which specStep picks up if it is
  // called next with (out[a], pos+a+1).
  //   mtpBatchRefill = false: the old one-submit-per-row loop (A/B).
  //   mtpPreDraft = false: no extra column; the next step runs its first draft itself.
  // Drafts only: the verify pass decides every output token, whatever these rows hold.
  // next: "head" (pre-run the next step's first draft), "row" (its draft-block column only), "none".
  // hs = null (fused verify): the trunk hiddens are still in B.x columns 0 .. a (the refill's column c
  // is verify column c, so the refill reads them in place, the same bytes the CPU copy would hold).
  async _refillDrafts(out, hs, pos, a, next) {
    const { dim } = this.dims;
    const trunkX = hs ? hs.subarray(a * dim, (a + 1) * dim) : null;
    if (this.mtpBatchRefill === false) {
      for (let j = 1; j <= a; j++) {
        if (hs) { this.device.queue.writeBuffer(this.x, 0, hs.subarray((j - 1) * dim, j * dim)); await this._mtpRun(null, out[j - 1], pos + j, false); }
        else await this._mtpRun(j - 1, out[j - 1], pos + j, false);   // hnorm reads B.x column j - 1
      }
      if (hs) this.device.queue.writeBuffer(this.x, 0, trunkX);
      else this._adoptHidden(a);
      return;
    }
    const pre = next !== "none" && this.mtpPreDraft !== false && pos + a + 1 < this.maxSeq, head = next === "head";
    const m = a + (pre ? 1 : 0);
    if (m > 0) {
      if (!this.B) this._initBatch();
      if (pre && head && this._preBusy) await this._preBusy;   // the staging buffer's last map
      const id = this._mtpRefill(out, hs, pos, m, pre && head, a);
      if (pre) this._pre = { pos: pos + a + 1, tok: out[a], id };
    } else if (!hs) this._adoptHidden(a);
    // the trunk hidden the next step starts from; queued after the refill submit, which reads B.x
    // (its own copy of hs) and leaves the draft block's output for the last column in mtp.xNext
    // (hs = null: _mtpRefill saved B.x column a first and restores it to this.x at its end)
    if (hs) this.device.queue.writeBuffer(this.x, 0, trunkX);
  }
  // Encode + submit the refill: columns c = 0..m-1 pair hs[c] with toks[c] and write draft-cache
  // row pos+c+1, in chunks of at most NC columns. Returns the promise of the head's argmax on the
  // last column when `head`, else null.
  // hs = null: the hiddens are B.x columns 0 .. m - 1 already (m <= NC); column `a` (the trunk hidden
  // the next step starts from) is saved first and copied to this.x after everything else.
  _mtpRefill(toks, hs, pos, m, head, a = m - 1) {
    const D = this.dims, M2 = this.mtp, B = this.B, dev = this.device;
    if (!M2.xNext) M2.xNext = dev.createBuffer({ size: D.dim * 4, usage: GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST });
    if (!hs && m > this.NC) throw new Error("_mtpRefill: in-place refill wider than one batch pass");
    const enc = dev.createCommandEncoder();
    if (!hs) {
      if (!this._xSave) this._xSave = dev.createBuffer({ size: D.dim * 4, usage: GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST });
      enc.copyBufferToBuffer(B.x.buf, a * B.x.stride, this._xSave, 0, D.dim * 4);
    }
    let last = 0;
    for (let c0 = 0; c0 < m; c0 += this.NC) {
      const w = Math.min(this.NC, m - c0);
      // all writes for this chunk land before this submit, and the chunks share B's columns, so
      // a second chunk needs its own submit after the first
      for (let j = 0; j < w; j++) {
        const c = c0 + j;
        if (hs) dev.queue.writeBuffer(B.x.buf, j * B.x.stride, hs.subarray(c * D.dim, (c + 1) * D.dim));
        dev.queue.writeBuffer(B.mEmb.buf, j * B.mEmb.stride, this._embedRowF32(toks[c]));
        dev.queue.writeBuffer(this.frameBufsB[j], 0, new Uint32Array([pos + c + 1, pos + c + 2, w, 0]));
      }
      const e = c0 + w < m ? dev.createCommandEncoder() : enc;
      {
        const p = e.beginComputePass();
        this._dMC(p, "rmsnorm_mc", M2.bgENormMC, 256, 256, w);
        this._dMC(p, "rmsnorm_mc", M2.bgHNormMC, 256, 256, w);
        this._dop(p, M2.projB, w);
        p.end();
      }
      this._encodeLayerBatch(e, this.layers.length, pos + c0 + 1, w);
      if (e !== enc) dev.queue.submit([e.finish()]);
      last = w - 1;
    }
    enc.copyBufferToBuffer(B.x.buf, last * B.x.stride, M2.xNext, 0, D.dim * 4);
    if (!head) {
      if (!hs) enc.copyBufferToBuffer(this._xSave, 0, this.x, 0, D.dim * 4);
      dev.queue.submit([enc.finish()]);
      return null;
    }
    // the draft head on the last column, as _mtpRun runs it (this.x -> shared_head_norm -> head)
    enc.copyBufferToBuffer(B.x.buf, last * B.x.stride, this.x, 0, D.dim * 4);
    const small = this._smallHead();
    {
      const p = enc.beginComputePass();
      this._d(p, "rmsnorm", M2.bgHeadNorm, 256, 256);
      this._dop(p, small ? this.headOpDraft : this.headOp);
      this._dArgmax(p, small);
      p.end();
    }
    if (!this.stagePre) this.stagePre = dev.createBuffer({ size: 16, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    enc.copyBufferToBuffer(this.argBuf, 0, this.stagePre, 0, 16);
    if (!hs) enc.copyBufferToBuffer(this._xSave, 0, this.x, 0, D.dim * 4);
    dev.queue.submit([enc.finish()]);
    const st = this.stagePre;
    const id = st.mapAsync(GPUMapMode.READ).then(() => { const v = new Uint32Array(st.getMappedRange())[0]; st.unmap(); return v; }, () => null);
    this._preBusy = id;
    return id;
  }
  // The pending pre-run column if it is the one (tok, pos) needs; clears it either way.
  _takePre(tok, pos) {
    const p = this._pre;
    this._pre = null;
    return p && p.pos === pos && p.tok === tok && this.mtp?.xNext ? p : null;
  }
  // First draft of a step whose draft-block column was pre-run: the pending argmax, or the head
  // run now on mtp.xNext. Leaves this.x = the block's output (what the second draft reads), the
  // same state _mtpRun(null, tNext, pos, "argmax") leaves. null: fall back to the normal path.
  async _preDraft0(pre) {
    const M2 = this.mtp, dim = this.dims.dim;
    let id = null;
    if (pre.id) {
      id = await pre.id;
      if (id === null) return null;
      const enc = this.device.createCommandEncoder();
      enc.copyBufferToBuffer(M2.xNext, 0, this.x, 0, dim * 4);
      this.device.queue.submit([enc.finish()]);
      return id;
    }
    const small = this._smallHead();
    const enc = this.device.createCommandEncoder();
    enc.copyBufferToBuffer(M2.xNext, 0, this.x, 0, dim * 4);
    {
      const p = enc.beginComputePass();
      this._d(p, "rmsnorm", M2.bgHeadNorm, 256, 256);
      this._dop(p, small ? this.headOpDraft : this.headOp);
      this._dArgmax(p, small);
      p.end();
    }
    enc.copyBufferToBuffer(this.argBuf, 0, this.stageArg, 0, 16);
    this.device.queue.submit([enc.finish()]);
    await this.stageArg.mapAsync(GPUMapMode.READ);
    id = new Uint32Array(this.stageArg.getMappedRange())[0];
    this.stageArg.unmap();
    return id;
  }

  // Fill the draft block's cache for the columns of a chunk that just went through the trunk
  // (their final hiddens are in B.x): column c pairs the trunk hidden at basePos + c with token
  // ids[i0 + c + 1] and writes the draft cache row at basePos + c + 1. One batched pre-pass and
  // one batched layer pass for the whole chunk, instead of one submit per column (mtpRun).
  // Drafts only: the output never depends on it. mtpBatchFill = false restores the per-column path.
  _mtpFillBatch(ids, i0, basePos, n) {
    this._pre = null;
    const m = Math.min(n, ids.length - i0 - 1);   // columns whose next token is in this prompt
    if (m <= 0) return;
    const D = this.dims, M2 = this.mtp, B = this.B;
    for (let c = 0; c < m; c++) {
      this.device.queue.writeBuffer(B.mEmb.buf, c * B.mEmb.stride, this._embedRowF32(ids[i0 + c + 1]));
      this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c + 1, basePos + c + 2, m, 0]));
    }
    const enc = this.device.createCommandEncoder();
    {
      const p = enc.beginComputePass();
      this._dMC(p, "rmsnorm_mc", M2.bgENormMC, 256, 256, m);
      this._dMC(p, "rmsnorm_mc", M2.bgHNormMC, 256, 256, m);
      this._dop(p, M2.projB, m);
      p.end();
    }
    this._encodeLayerBatch(enc, this.layers.length, basePos + 1, m);
    this.device.queue.submit([enc.finish()]);
  }

  // One layer-major ubatch of W prompt tokens ids[i0 .. i0 + W - 1] at this.pos (W a multiple of NC, at most
  // moeGrpU; moeGroupPrefill, see engine/wgsl/moe_group.js). Per layer: the W / NC sub-passes run the layer
  // exactly as a prefill pass does (_encodeLayerBatch, each with its own frame uniform), stopping after
  // moe_route on fused MoE layers; their residual, normed input and routing are copied into the ubatch
  // buffers; then one sort + grouped gate/up + grouped down + combine covers the MoE FFN of all W tokens.
  // The draft (MTP) cache is filled per sub-pass afterwards, from the final hiddens, as prefillTokens does
  // after each pass.
  // xs / fill: as _prefillWide (prefillHidden)
  async _prefillGrouped(ids, i0, W = this.gB.U, { xs: hx = null, fill = true } = {}) {
    const gB = this.gB, NC = this.NC, B = this.B, D = this.dims, q = this.device.queue, basePos = this.pos, nSub = W / NC;
    if (W % NC || W > gB.U || W < NC) throw new Error(`_prefillGrouped: width ${W} (NC ${NC}, ubatch ${gB.U})`);
    const xs = NC * B.x.stride, xns = NC * B.xn.stride, ss = NC * this.moe.KS * 4;
    if (gB.sortW !== W) { q.writeBuffer(gB.sortU, 0, new Uint32Array([W * this.moe.KS, ...gB.sortArgs])); gB.sortW = W; }
    for (let s = 0; s < nSub; s++) q.writeBuffer(gB.frames[s], 0, new Uint32Array([basePos + s * NC, basePos + s * NC + 1, NC, 0]));
    for (let c = 0; c < W; c++) q.writeBuffer(gB.XW, c * B.x.stride, hx ? hx.subarray(c * D.dim, (c + 1) * D.dim) : this._embedRowF32(ids[i0 + c]));
    let enc = this.device.createCommandEncoder();
    try {
      for (let l = 0; l < this.layers.length; l++) {
        const L = this.layers[l], grp = !!L.fused;
        for (let s = 0; s < nSub; s++) {
          enc.copyBufferToBuffer(gB.XW, s * xs, B.x.buf, 0, xs);
          this._g0 = gB.common[s];
          this._encodeLayerBatch(enc, l, basePos + s * NC, NC, false, grp);
          this._g0 = null;
          enc.copyBufferToBuffer(B.x.buf, 0, gB.XW, s * xs, xs);
          if (grp) {
            enc.copyBufferToBuffer(B.xn.buf, 0, gB.XNW, s * xns, xns);
            enc.copyBufferToBuffer(B.mSel, 0, gB.sel, s * ss, ss);
            enc.copyBufferToBuffer(B.mSelw, 0, gB.selw, s * ss, ss);
          }
        }
        if (grp) {
          if (L.off) enc = await this._regionLoad(enc, L, gB.sel, W);   // offloaded: the frame's experts into the region
          const M = this.layerB[l].mc, p = enc.beginComputePass();
          this._dxyz(p, "moe_gsort", gB.bgSort, 1, 1, 1);
          this._encGroupExperts(p, L, M, W);
          this._dxyz(p, "moe_combw", gB.bgComb, Math.ceil(D.dim / 64), W, 1);
          p.end();
        }
      }
    } finally { this._g0 = null; }
    enc.copyBufferToBuffer(gB.XW, (W - 1) * B.x.stride, this.x, 0, D.dim * 4);
    if (this._grpDone) await this._grpDone;   // at most two ubatches in flight
    q.submit([enc.finish()]);
    this._grpDone = q.onSubmittedWorkDone();
    if (fill && this.mtp && this.mtpFill !== false) for (let s = 0; s < nSub; s++) {
      const e2 = this.device.createCommandEncoder();
      e2.copyBufferToBuffer(gB.XW, s * xs, B.x.buf, 0, xs);
      q.submit([e2.finish()]);
      const i = i0 + s * NC, pos = basePos + s * NC;
      if (this.mtpBatchFill !== false) this._mtpFillBatch(ids, i, pos, NC);
      else for (let c = 0; c < NC; c++) if (i + c + 1 < ids.length) await this.mtpRun(c, ids[i + c + 1], pos + c + 1, false);
    }
    this.pos += W;
  }

  // the grouped expert launches of one MoE layer over w ubatch columns (after moe_gsort, before moe_combw): the f32
  // tiled kernels, or with moeGroupDp4a the input quantization + dp4a gate/up, h quantization + dp4a down. dp4a only
  // in wide chunks (wide = true), where prefillDp4a's projections run too: a prompt under one wide tile (the grouped
  // ubatches of _prefillGrouped, 32-63 tokens at batchCols 16) keeps the f32 kernels, as its projections do.
  _encGroupExperts(p, L, M, w, wide = false) {
    const gB = this.gB;
    if (wide && this.moeDp4a && this.moeGroupDp4a !== false) {
      const { KS, hs } = this.moe;
      this._dxyz(p, "moe_qx", gB.bgQx, Math.ceil(this.dims.dim / 1024), w, 1);
      this._dInd(p, L.gusqPipe, M.gusQ, gB.ind, 0);
      this._dxyz(p, "moe_qx", gB.bgQh, Math.ceil(w * KS * hs / 1024), 1, 1);
      this._dInd(p, L.dnqPipe, M.dnQ, gB.ind, 12);
      return;
    }
    this._dInd(p, L.gusgPipe, M.gusG, gB.ind, 0);
    this._dInd(p, L.dngPipe, M.dngG, gB.ind, 12);
  }

  // What the last grouped MoE layer saw: { pairs, uniqueRouted (distinct routed experts), chunks }. For logs and
  // validation; it reads the indirect-args buffer back, so call it between prefills, not in a hot loop.
  async moeGroupStats() {
    if (!this.gB) return null;
    const st = this.device.createBuffer({ size: 32, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
    const enc = this.device.createCommandEncoder(); enc.copyBufferToBuffer(this.gB.ind, 0, st, 0, 32); this.device.queue.submit([enc.finish()]);
    await st.mapAsync(GPUMapMode.READ); const a = new Uint32Array(st.getMappedRange().slice(0)); st.unmap(); st.destroy();
    return { pairs: a[7], uniqueRouted: a[6], chunks: a[1] };
  }

  async prefillTokens(ids) {
    if (!this.B) this._initBatch();
    this._pre = null;
    this._noteDV(ids);   // the prompt's language decides the draft head from the first step
    this._snapNow = null;   // not a verify: nothing to keep for replay
    let i = 0, sinceSync = 0;
    const NC = this.NC;
    // wide prefill (default on a whole-model engine): chunks of up to ubatch tokens, multiples of the GEMM tile width
    if (this.ubatch && this.prefillWide !== false) {
      const BN = this.wideCfg.BN;
      while (ids.length - i >= BN) {
        const w = Math.min(this.ubatch, Math.floor((ids.length - i) / BN) * BN);
        if (await this._prefillWide(ids, i, w) === false) break;
        i += w;
      }
    }
    // expert-grouped ubatches (MoE, opt-in; inside the wide chunks too when both are on): full ones of U tokens,
    // then one of the remaining whole passes (at least two); the rest (under 2 * NC tokens) goes through the passes below
    if (this.gB && this.moeGroup !== false) {
      while (ids.length - i >= 2 * NC) {
        const W = Math.min(this.gB.U, Math.floor((ids.length - i) / NC) * NC);
        await this._prefillGrouped(ids, i, W); i += W;
      }
      this._grpDone = null;
    }
    while (ids.length - i >= NC) {
      const basePos = this.pos;
      for (let c = 0; c < NC; c++) {
        this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1, NC, 0]));
        this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, this._embedRowF32(ids[i + c]));
      }
      let enc = this.device.createCommandEncoder();
      for (let l = 0; l < this.layers.length; l++) { this._encodeLayerBatch(enc, l, basePos, NC); if (this._cut) enc = await this._cutResolve(enc); }
      enc.copyBufferToBuffer(this.B.x.buf, (NC - 1) * this.B.x.stride, this.x, 0, this.dims.dim * 4);
      this.device.queue.submit([enc.finish()]);
      if (this.mtp && this.mtpFill !== false) {
        if (this.mtpBatchFill !== false) this._mtpFillBatch(ids, i, basePos, NC);
        else for (let c = 0; c < NC; c++) if (i + c + 1 < ids.length) await this.mtpRun(c, ids[i + c + 1], basePos + c + 1, false);
      }
      this.pos += NC;
      i += NC;
      if (++sinceSync >= 4) { await this.device.queue.onSubmittedWorkDone(); sinceSync = 0; }
    }
    // step the tail down through the narrower twins (16 -> 8 -> 4) before
    // falling back to single tokens: at NC=16 a 15-token remainder would
    // otherwise cost 15 full single-token passes (~1.7 s on a 27B).
    for (const W of [8, 4].filter((w) => w < NC)) {
      while (ids.length - i >= W) {
        const basePos = this.pos;
        for (let c = 0; c < W; c++) {
          this.device.queue.writeBuffer(this.frameBufsB[c], 0, new Uint32Array([basePos + c, basePos + c + 1, W, 0]));
          this.device.queue.writeBuffer(this.B.x.buf, c * this.B.x.stride, this._embedRowF32(ids[i + c]));
        }
        let enc = this.device.createCommandEncoder();
        for (let l = 0; l < this.layers.length; l++) { this._encodeLayerBatch(enc, l, basePos, W); if (this._cut) enc = await this._cutResolve(enc); }
        enc.copyBufferToBuffer(this.B.x.buf, (W - 1) * this.B.x.stride, this.x, 0, this.dims.dim * 4);
        this.device.queue.submit([enc.finish()]);
        if (this.mtp && this.mtpFill !== false) {
          if (this.mtpBatchFill !== false) this._mtpFillBatch(ids, i, basePos, W);
          else for (let c = 0; c < W; c++) if (i + c + 1 < ids.length) await this.mtpRun(c, ids[i + c + 1], basePos + c + 1, false);
        }
        this.pos += W; i += W;
        await this.device.queue.onSubmittedWorkDone();
      }
    }
    for (; i < ids.length; i++) {
      await this.prefillToken(ids[i]);
      // the draft cache entry for the next token sits at the next position (this.pos after the
      // prefillToken above), which is i + 1 only when the prompt started at position 0
      if (this.mtp && this.mtpFill !== false && i + 1 < ids.length) await this.mtpRun(null, ids[i + 1], this.pos, false);
      if (i % 8 === 7) await this.device.queue.onSubmittedWorkDone();
    }
    await this.device.queue.onSubmittedWorkDone();
  }

  // A room chain's prompt frames (room-node's batched prefill): n columns at basePos through this shard's
  // layers with the kernels prefillTokens uses (wide GEMM chunks, expert-grouped MoE ubatches), every
  // column's output hidden read back. src: token ids (the shard with the embedding) or n * dim input
  // hiddens (a later shard). The draft cache is not filled: the chain host fills it from the hiddens the
  // chain returns (roomnode fillDrafts). Columns past the last whole chunk run as batchCols passes
  // (runHiddenBatch / embedRunBatch). Prefill tolerance, as prefillTokens. -> Float32Array(n * dim)
  // the frame width prefillHidden is made for (a chain host's prompt frames); 0: no prefill kernels on this shard
  prefillFrame() { return this.ubatch && this.prefillWide !== false ? this.ubatch : this.moeGrpU && this.moeGroup !== false ? this.moeGrpU : 0; }
  async prefillHidden(src, basePos) {
    if (!this.B) this._initBatch();
    this._pre = null; this._snapNow = null;
    const dim = this.dims.dim, NC = this.NC, ids = src instanceof Float32Array ? null : src;
    const n = ids ? ids.length : src.length / dim, out = new Float32Array(n * dim);
    const part = (i, m) => (ids ? null : src.subarray(i * dim, (i + m) * dim));
    const read = async (buf, stride, i, m) => {   // columns 0 .. m - 1 of buf -> out[i ..]
      if (!this._stageW || this._stageW.size < m * dim * 4) {
        this._stageW?.destroy();
        this._stageW = this.device.createBuffer({ size: m * dim * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ });
      }
      const st = this._stageW, enc = this.device.createCommandEncoder();
      for (let c = 0; c < m; c++) enc.copyBufferToBuffer(buf, c * stride, st, c * dim * 4, dim * 4);
      this.device.queue.submit([enc.finish()]);
      await st.mapAsync(GPUMapMode.READ, 0, m * dim * 4);
      out.set(new Float32Array(st.getMappedRange(0, m * dim * 4)), i * dim);
      st.unmap();
    };
    let i = 0;
    this.pos = basePos;
    if (this.ubatch && this.prefillWide !== false) {
      const BN = this.wideCfg.BN;
      while (n - i >= BN) {
        const w = Math.min(this.ubatch, Math.floor((n - i) / BN) * BN);
        if (await this._prefillWide(ids, i, w, { xs: part(i, w), fill: false }) === false) break;
        await read(this.Wt.x.buf, this.Wt.x.stride, i, w); i += w;
      }
    }
    if (this.gB && this.moeGroup !== false) {
      while (n - i >= 2 * NC) {
        const W = Math.min(this.gB.U, Math.floor((n - i) / NC) * NC);
        await this._prefillGrouped(ids, i, W, { xs: part(i, W), fill: false });
        await read(this.gB.XW, this.B.x.stride, i, W); i += W;
      }
      this._grpDone = null;
    }
    while (i < n) {
      const m = Math.min(NC, n - i);
      out.set(ids ? await this.embedRunBatch(ids.slice(i, i + m), this.pos) : await this.runHiddenBatch(part(i, m), this.pos), i * dim);
      i += m;
    }
    this.pos = basePos + n;
    return out;
  }

  // prefill fast path: layers only, no head, no readback
  async prefillToken(tokenId) {
    this._pre = null;
    this._setFrame(this.pos, this.pos + 1);
    this.device.queue.writeBuffer(this.x, 0, this._embedRowF32(tokenId));
    let enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) { this._encodeLayer(enc, i); if (this._cut) enc = await this._cutResolve(enc); }
    this.device.queue.submit([enc.finish()]);
    this.pos++;
    // fire-and-forget; callers batch backpressure via onSubmittedWorkDone()
  }

  async embedRun(tokenId, pos) {
    this._pre = null;
    const { dim } = this.dims;
    this.pos = pos;
    this._setFrame(pos, pos + 1);
    this.device.queue.writeBuffer(this.x, 0, this._embedRowF32(tokenId));
    let enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) { this._encodeLayer(enc, i); if (this._cut) enc = await this._cutResolve(enc); }
    this.device.queue.submit([enc.finish()]);
    return await this._readback(this.x, this.stageX, dim);
  }

  async runHidden(xIn, pos) {
    this._pre = null;
    const { dim } = this.dims;
    this.pos = pos;
    this._setFrame(pos, pos + 1);
    this.device.queue.writeBuffer(this.x, 0, xIn);
    let enc = this.device.createCommandEncoder();
    for (let i = 0; i < this.layers.length; i++) { this._encodeLayer(enc, i); if (this._cut) enc = await this._cutResolve(enc); }
    this.device.queue.submit([enc.finish()]);
    const h = await this._readback(this.x, this.stageX, dim);
    // the caches now hold [0, pos]: a checkpoint saved before the next frame (sv) must keep this
    // row, as it does after a batch (_runBatchAndRead leaves pos at the batch's end)
    this.pos = pos + 1;
    return h;
  }

  async headFromHidden(xIn) {
    this._pre = null;
    const { vocab } = this.dims;
    this.device.queue.writeBuffer(this.x, 0, xIn);
    const enc = this.device.createCommandEncoder();
    {
      const p = enc.beginComputePass();
      this._d(p, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dop(p, this.headOp);
      p.end();
    }
    this.device.queue.submit([enc.finish()]);
    return await this._readback(this.logits, this.stageLogits, vocab);
  }

  // Whole token in one encoder + one submit; no hidden-state readback between the last layer and
  // the head (that round trip cost a full pipeline drain), and the logits copy rides in the same
  // command buffer. Encode-ahead: while the GPU runs token N and we wait for its logits, the command
  // buffer for position N + 1 is recorded as well. A token's commands depend only on the position and
  // the runtime switches (the embedding and the frame uniform are queue writes made at call time), so
  // the next call submits at once instead of paying the CPU encode (~900 dispatches) on the critical
  // path. Same commands, same bits. engine.encodeAhead = false for A/B.
  // desc: forwardTokenIds' GPU sampling descriptor (the head's top-k in the same buffer), null: logits
  _fwdKey(desc = null) { return [this.attnGlue, this.attnDecode, this.fuseProj, this.dnFuse, this.layerFuse.dn, this.layerFuse.conv, this.layerFuse.kv, this.layerFuse.comb, this.layerFuse.norm, this.layerFuse.pass, this.softmaxWG, this.b4, this.skip ? 1 : 0, this._common ? 1 : 0, desc ? topkK(desc) : 0].join(); }
  _encodeForward(pos, desc = null, ahead = false) {
    const enc = this._fwdBegin(ahead);
    for (let i = 0; i < this.layers.length; i++) this._encodeLayerR(enc, this.layers[i], pos);
    return this._fwdEnd(enc, pos, desc);
  }
  // expert offload: the same token, its command buffer split at every offloaded layer (_cutResolve submits the parts
  // up to each cut); -> the job of the last part, as _encodeForward's (cb not yet submitted)
  async _encodeForwardOff(pos, desc = null) {
    if (this.cutSpec) return this._encodeForwardSpec(pos, desc);
    if (this.cutProf) this.cutProf.last = performance.now();
    let enc = this._fwdBegin(false);
    const pipe = this.cutPipe === true;
    try {
      for (let i = 0; i < this.layers.length; i++) { this._encodeLayerR(enc, this.layers[i], pos); if (this._cut) enc = await this._cutResolve(enc, pipe); }
      enc = await this._cutFlush(enc);
    } catch (e) { await this._offAbort(); throw e; }
    return this._fwdEnd(enc, pos, desc);
  }
  // an offloaded token that threw mid-way: no cut, pending plan or FFN mode left behind, the gate count back, the
  // staging buffers unmapped (so the next call can map them)
  async _offAbort() {
    const pd = this._cutPend;
    this._cutPend = null; this._cut = null; this._ffnMode = null; this._pp = null; this._ppB = null;
    if (pd) { try { await pd.mapped; } catch { /* the token failed anyway */ } this._gateDone(); }
    for (const b of [this.offStage, this._spec?.stage]) if (b && b.mapState === "mapped") b.unmap();
  }
  // ---- cutSpec: speculative decode over offloaded layers (one token; engine flag, off by default) ----
  // A layer that has rarely missed of late (specMiss, a per-layer moving average of "had a miss this token", below
  // cutSpecTheta) does not cut: moe_remap (engine/expert_store.js REMAP_WGSL) maps its route's ids to slots through
  // the store's GPU table and its experts run at once, in the same command buffer as the layers around it. Its ids
  // and a miss flag go to a log the next cut reads back with its own selection (one map for the whole window). If
  // every speculated layer of the window hit, the CPU replays plan() for each of them in order (stamps only: the
  // LRU state a cut per layer would leave) and handles the cut as before. If one missed, its experts read a slot that
  // held another expert: the token rolls back to that layer's FFN. Its misses are copied into the pool, the recurrent
  // state (DeltaNet S, conv window) of every later layer comes back from the copy taken at the token's start, x from
  // moe_remap's copy of the layer's FFN input; the FFN then runs again from its norm (the same router and route, the
  // same selection) on the CPU's slot ids, and encoding goes on from the next layer. KV rows the window wrote are
  // written again before anything reads them. So every expert kernel that counts read the right expert: the same bits.
  // The last offloaded layer always cuts (closes the last window). cutSpecMax: most layers speculated in a row.
  _specInit() {
    if (this._spec) return this._spec;
    const dev = this.device, st = this.experts, { K, KS } = this.moe, dim = this.dims.dim;
    st.gpuTables();
    const offs = this.layers.filter((L) => L.off), nOff = offs.length;
    const S = GPUBufferUsage.STORAGE | GPUBufferUsage.COPY_SRC | GPUBufferUsage.COPY_DST;
    const log = dev.createBuffer({ size: (nOff * KS + nOff) * 4, usage: S });
    const stage = dev.createBuffer({ size: (nOff * KS + nOff + KS) * 4, usage: GPUBufferUsage.MAP_READ | GPUBufferUsage.COPY_DST });
    const xs = dev.createBuffer({ size: nOff * dim * 4, usage: S });
    const pipe = dev.createComputePipeline({ layout: "auto", compute: { module: dev.createShaderModule({ code: REMAP_WGSL }), entryPoint: "moe_remap" } });
    offs.forEach((L, j) => {
      L.specJ = j; L.specMiss = 1;   // pessimistic start: a layer cuts until it has hit for a few tokens
      const u = this._buf(new Uint32Array([K, KS, j, dim, nOff, 0, 0, 0]), GPUBufferUsage.UNIFORM);
      L.bgRemap = dev.createBindGroup({ layout: pipe.getBindGroupLayout(0),
        entries: [L.selBuf, st.L.get(L.offLayer).tab, this.offSel, log, this.x, xs, u].map((b, i) => ({ binding: i, resource: { buffer: b } })) });
    });
    for (const L of this.layers) if (!L.isFull && L.S) {
      L.S_spec = dev.createBuffer({ size: L.S.size, usage: S });
      L.conv_spec = dev.createBuffer({ size: L.convState.size, usage: S });
    }
    return (this._spec = { pipe, log, stage, xs, nOff });
  }
  async _encodeForwardSpec(pos, desc = null) {
    const sp = this._specInit(), q = this.device.queue, nL = this.layers.length, P = this.cutProf;
    if (P) P.last = performance.now();
    const th = this.cutSpecTheta ?? 0.25, mx = this.cutSpecMax ?? 8;
    let lastOff = -1, run = 0, first = -1;
    for (let i = 0; i < nL; i++) if (this.layers[i].off) lastOff = i;
    const mode = this.layers.map((L, i) => {
      if (!L.off) return null;
      if (i !== lastOff && L.specMiss < th && run < mx) { run++; if (first < 0) first = i; return "spec"; }
      run = 0; return "cut";
    });
    if (first >= 0) {   // the recurrent state of every layer after the first speculated one, for a rollback
      const e = this.device.createCommandEncoder();
      for (let i = first + 1; i < nL; i++) {
        const L = this.layers[i];
        if (L.S_spec) { e.copyBufferToBuffer(L.S, 0, L.S_spec, 0, L.S.size); e.copyBufferToBuffer(L.convState, 0, L.conv_spec, 0, L.convState.size); }
      }
      q.submit([e.finish()]);
    }
    const missed = new Uint8Array(nL);
    let enc = this._fwdBegin(false), win = [], i = 0;
    try {
      while (i < nL) {
        this._ffnMode = mode[i] === "spec" ? "spec" : null;
        this._encodeLayerR(enc, this.layers[i], pos);
        this._ffnMode = null;
        if (mode[i] === "spec") { win.push(i); i++; continue; }
        if (!this._cut) { i++; continue; }
        const r = await this._specCut(enc, win, i, missed);
        enc = r.enc; i = r.next; win = [];
      }
    } catch (e) { await this._offAbort(); throw e; }
    for (let k = 0; k < nL; k++) { const L = this.layers[k]; if (L.off) L.specMiss = 0.75 * L.specMiss + 0.25 * missed[k]; }
    return this._fwdEnd(enc, pos, desc);
  }
  // the cut at layer ci after a window of speculated layers (win): one readback of the window's log and ci's selection
  async _specCut(enc, win, ci, missed) {
    const c = this._cut, sp = this._spec, st = this.experts, q = this.device.queue, { KS } = this.moe, nOff = sp.nOff, dim = this.dims.dim, P = this.cutProf;
    this._cut = null;
    if (c.ppB || c.M || c.nCols !== 1) throw new Error("expert offload: a speculative window is one-token decode only");   // (reopen handles _pp only)
    const t0 = P ? performance.now() : 0;
    if (P && P.last) P.enc += t0 - P.last;
    const tail = nOff * KS + nOff, bytes = (tail + KS) * 4;
    if (win.length) enc.copyBufferToBuffer(sp.log, 0, sp.stage, 0, tail * 4);
    enc.copyBufferToBuffer(c.L.selBuf, 0, sp.stage, tail * 4, KS * 4);
    q.submit([enc.finish()]);
    const t1 = P ? performance.now() : 0;
    this._gateOpen();
    try { await mapRead(this.device, sp.stage, 0, bytes, this._kick()); } finally { this._gateDone(); }
    const t2 = P ? performance.now() : 0;
    const a = new Uint32Array(sp.stage.getMappedRange(0, bytes)).slice();
    sp.stage.unmap();
    const e2 = this.device.createCommandEncoder();
    const reopen = () => { if (c.pp) this._pp = e2.beginComputePass(); };
    // a disk layer reads its misses before plan(). Only the first read may flush T1 (evict and remap slots): after
    // it, e2 may already hold copies from T1 slots, which must stay unmapped until e2 is submitted.
    let flushed = false;
    const prep = async (layer, sel) => {
      if (!st.isDisk(layer)) return;
      const flush = !flushed; flushed = true;
      await this._diskWait(() => st.prepare(layer, sel, 1, KS, { flush }));
    };
    const { hs, gusRows, dncRows } = this.moe;
    let next = ci + 1;
    for (const m of win) {
      const L = this.layers[m], j = L.specJ, sel = a.subarray(j * KS, (j + 1) * KS);
      if (a[nOff * KS + j] === 0) {   // held: plan() only stamps its experts
        const b0 = st.stats.bytes;
        await prep(L.offLayer, sel);   // (reads nothing: every expert is in the pool)
        st.plan(e2, L.offLayer, sel, 1, KS);
        if (st.stats.bytes !== b0) throw new Error(`expert offload: layer ${L.offLayer} hit on the GPU, missed on the CPU`);
        st.stats.specs++; missed[m] = 0;
        continue;
      }
      // missed: roll back to layer m's FFN (see the note at _specInit)
      st.stats.specFails++; missed[m] = 1;
      await prep(L.offLayer, sel);
      const r = st.plan(e2, L.offLayer, sel, 1, KS);
      q.writeBuffer(this.offSel, 0, r.remap);
      for (let k = m + 1; k <= ci; k++) {
        const Lk = this.layers[k];
        if (Lk.S_spec) { e2.copyBufferToBuffer(Lk.S_spec, 0, Lk.S, 0, Lk.S.size); e2.copyBufferToBuffer(Lk.conv_spec, 0, Lk.convState, 0, Lk.convState.size); }
      }
      e2.copyBufferToBuffer(sp.xs, j * dim * 4, this.x, 0, dim * 4);
      reopen();
      this._ffnMode = "redo";
      this._encodeFFN(e2, L);
      this._ffnMode = null;
      next = m + 1;
      break;
    }
    if (next === ci + 1) {   // the window held: the cut as _cutResolve handles it
      const b0 = st.stats.misses;
      await prep(c.L.offLayer, a.subarray(tail, tail + KS));
      const r = st.plan(e2, c.L.offLayer, a.subarray(tail, tail + KS), 1, KS);
      missed[ci] = st.stats.misses !== b0 ? 1 : 0;
      q.writeBuffer(this.offSel, 0, r.remap);
      const p = e2.beginComputePass();
      if (c.pp) this._pp = p;
      this._dxyz(p, c.L.gusPipe, c.L.bgGusP, Math.ceil(hs / gusRows), KS, 1);
      this._dxyz(p, c.L.dncPipe, c.L.bgDncP, Math.ceil(dim / dncRows), 1, 1);
      if (!c.pp) p.end();
    }
    if (P) { const t3 = performance.now(); P.n++; P.submit += t1 - t0; P.wait += t2 - t1; P.plan += t3 - t2; P.last = t3; }
    return { enc: e2, next };
  }
  _fwdBegin(ahead) {
    const enc = this.device.createCommandEncoder();
    if (ahead) { this._aheadBufs(); this._saveAhead(enc, true); }   // decode-ahead: save the state, then gather x from the top-1
    // layerFuse.pass: the whole token (every layer and the head) in one compute pass instead of 2-3
    // per layer; dispatches stay in the same order (each dispatch is its own usage scope)
    const one = this.layerFuse.pass && this.flash;
    this._pp = one ? enc.beginComputePass() : null;
    if (ahead) { const p = this._pp || enc.beginComputePass(); this._d(p, "emb_gather", this._egX, this.dims.dim); if (p !== this._pp) p.end(); }
    return enc;
  }
  _fwdEnd(enc, pos, desc) {
    const { vocab } = this.dims;
    if (desc) {
      const op = this._tkOp("one", this.logits, vocab, 0, topkK(desc), this.topBuf);
      const p = this._pp || enc.beginComputePass();
      this._pp = null;
      this._d(p, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dop(p, this.headOp);
      this._dTopk(p, op, 1);
      p.end();
      this._stageTI = (this._stageTI || 0) ^ 1;   // alternate staging buffers, as the logits path
      const stage = this._stageTI ? (this.stageTop2 ||= this.device.createBuffer({ size: this.stageTop.size, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ })) : this.stageTop;
      enc.copyBufferToBuffer(this.topBuf, 0, stage, 0, op.R * 4);
      return { pos, key: this._fwdKey(desc), cb: enc.finish(), stage, op };
    }
    {
      const p = this._pp || enc.beginComputePass();
      this._pp = null;
      this._d(p, "rmsnorm", this.bgFinalNorm, 256, 256);
      this._dop(p, this.headOp);
      p.end();
    }
    this._stageI = (this._stageI || 0) ^ 1;   // alternate staging buffers: one can still be mapped
    const stage = this._stageI ? (this.stageLogits2 ||= this.device.createBuffer({ size: vocab * 4, usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ })) : this.stageLogits;
    enc.copyBufferToBuffer(this.logits, 0, stage, 0, vocab * 4);
    return { pos, key: this._fwdKey(), cb: enc.finish(), stage };
  }
  async forwardToken(tokenId) {
    this._pre = null;
    const { vocab } = this.dims;
    this._setFrame(this.pos, this.pos + 1);
    this.device.queue.writeBuffer(this.x, 0, this._embedRowF32(tokenId));
    const pre = this._fwdPre;
    this._fwdPre = null;
    // (expert offload: no encode-ahead, a token's commands depend on its routing)
    const job = this.experts ? await this._encodeForwardOff(this.pos) : pre && pre.pos === this.pos && pre.key === this._fwdKey() ? pre : this._encodeForward(this.pos);
    this.device.queue.submit([job.cb]);
    const mapped = mapRead(this.device, job.stage, 0, undefined, this._kick());
    if (this.encodeAhead !== false && !this.experts && this.pos + 1 < this.maxSeq) this._fwdPre = this._encodeForward(this.pos + 1);
    await mapped;
    const logits = Float32Array.from(new Float32Array(job.stage.getMappedRange(), 0, vocab));
    job.stage.unmap();
    this.pos++;
    return logits;
  }
}
